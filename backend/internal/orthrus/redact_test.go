package orthrus

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/time/rate"
)

// inspectBody is a container inspect document with two environment entries
// and a field that must survive redaction unchanged.
const inspectBody = `{"Id":"abc","Name":"/web","Config":{"Image":"nginx:1.27","Env":["PATH=/usr/bin","DB_PASSWORD=hunter2","BARE"]},"State":{"Running":true}}`

func decodeObject(t *testing.T, raw []byte) map[string]any {
	t.Helper()
	var m map[string]any
	require.NoError(t, json.Unmarshal(raw, &m))
	return m
}

func TestRedactContainerEnv_RedactsValuesOnly(t *testing.T) {
	out, err := redactContainerEnv([]byte(inspectBody))
	require.NoError(t, err)

	got := decodeObject(t, out)
	want := decodeObject(t, []byte(inspectBody))
	config := want["Config"].(map[string]any)
	config["Env"] = []any{"PATH=[redacted]", "DB_PASSWORD=[redacted]", "BARE"}
	assert.Equal(t, want, got)
	assert.NotContains(t, string(out), "hunter2")
}

func TestRedactContainerEnv_UnchangedWithoutEnv(t *testing.T) {
	cases := map[string]string{
		"no Config":        `{"Id":"abc"}`,
		"null Config":      `{"Id":"abc","Config":null}`,
		"no Env":           `{"Id":"abc","Config":{"Image":"x"}}`,
		"null Env":         `{"Id":"abc","Config":{"Env":null}}`,
		"empty Env":        `{"Id":"abc","Config":{"Env":[]}}`,
		"no Env in config": `{"Config":{}}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			out, err := redactContainerEnv([]byte(body))
			require.NoError(t, err)
			assert.Equal(t, body, string(out))
		})
	}
}

func TestRedactContainerEnv_Errors(t *testing.T) {
	cases := map[string]string{
		"not JSON":          `{"Config":`,
		"top level array":   `[]`,
		"Config not object": `{"Config":5}`,
		"Env not strings":   `{"Config":{"Env":[1,2]}}`,
		"Env not array":     `{"Config":{"Env":"PATH=/"}}`,
	}
	for name, body := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := redactContainerEnv([]byte(body))
			require.Error(t, err)
		})
	}
}

// trackingBody records whether anything read it, so tests can prove the
// proxy never buffered an unflagged response.
type trackingBody struct {
	io.Reader
	reads int
}

func (b *trackingBody) Read(p []byte) (int, error) {
	b.reads++
	return b.Reader.Read(p)
}

func (b *trackingBody) Close() error { return nil }

func flaggedRequest(t *testing.T) *http.Request {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, "/containers/abc/json", http.NoBody)
	return req.WithContext(context.WithValue(req.Context(), redactEnvKey{}, true))
}

func responseFor(req *http.Request, status int, header http.Header, body io.Reader) *http.Response {
	if header == nil {
		header = http.Header{}
	}
	return &http.Response{
		StatusCode:    status,
		Header:        header,
		Body:          io.NopCloser(body),
		ContentLength: -1,
		Request:       req,
	}
}

func TestRedactInspectEnv_FlaggedResponseIsRedacted(t *testing.T) {
	req := flaggedRequest(t)
	header := http.Header{}
	header.Set("Content-Type", "application/json")
	header.Set("Transfer-Encoding", "chunked")
	resp := responseFor(req, http.StatusOK, header, strings.NewReader(inspectBody))

	require.NoError(t, redactInspectEnv(resp))

	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	assert.NotContains(t, string(body), "hunter2")
	assert.Contains(t, string(body), "DB_PASSWORD=[redacted]")
	assert.Equal(t, int64(len(body)), resp.ContentLength)
	assert.Equal(t, strconv.Itoa(len(body)), resp.Header.Get("Content-Length"))
	assert.Empty(t, resp.Header.Get("Transfer-Encoding"))
	assert.Equal(t, "application/json", resp.Header.Get("Content-Type"))
}

func TestRedactInspectEnv_UnflaggedResponseIsNotTouched(t *testing.T) {
	cases := map[string]*http.Request{
		"no flag":    httptest.NewRequest(http.MethodGet, "/containers/json", http.NoBody),
		"no request": nil,
	}
	for name, req := range cases {
		t.Run(name, func(t *testing.T) {
			body := &trackingBody{Reader: strings.NewReader(inspectBody)}
			resp := responseFor(req, http.StatusOK, nil, body)
			resp.Body = body

			require.NoError(t, redactInspectEnv(resp))
			assert.Zero(t, body.reads, "unflagged response must not be read")
		})
	}
}

func TestRedactInspectEnv_NonOKIsNotTouched(t *testing.T) {
	req := flaggedRequest(t)
	body := &trackingBody{Reader: strings.NewReader(`{"message":"no such container"}`)}
	resp := responseFor(req, http.StatusNotFound, nil, body)
	resp.Body = body

	require.NoError(t, redactInspectEnv(resp))
	assert.Zero(t, body.reads)
}

func TestRedactInspectEnv_FailsClosed(t *testing.T) {
	tests := []struct {
		name   string
		header http.Header
		body   []byte
	}{
		{
			name:   "content encoding",
			header: http.Header{"Content-Encoding": []string{"gzip"}},
			body:   []byte(inspectBody),
		},
		{
			name: "oversized body",
			body: bytes.Repeat([]byte(" "), maxInspectBodyBytes+1),
		},
		{
			name: "invalid JSON",
			body: []byte(`{"Config":`),
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			resp := responseFor(flaggedRequest(t), http.StatusOK, tc.header, bytes.NewReader(tc.body))
			defer func() { _ = resp.Body.Close() }()
			err := redactInspectEnv(resp)
			require.Error(t, err)
			assert.NotContains(t, err.Error(), "hunter2", "error must never carry body content")
		})
	}
}

func TestRedactInspectEnv_BodyReadErrorFailsClosed(t *testing.T) {
	resp := responseFor(flaggedRequest(t), http.StatusOK, nil, io.MultiReader(
		strings.NewReader(`{"Config":`), errReader{},
	))
	defer func() { _ = resp.Body.Close() }()
	require.Error(t, redactInspectEnv(resp))
}

type errReader struct{}

func (errReader) Read([]byte) (int, error) { return 0, io.ErrUnexpectedEOF }

// captureNext records what the downstream handler saw.
type captureNext struct {
	flagged        bool
	acceptEncoding string
}

func (c *captureNext) ServeHTTP(_ http.ResponseWriter, r *http.Request) {
	c.flagged = isEnvRedacted(r.Context())
	c.acceptEncoding = r.Header.Get("Accept-Encoding")
}

func TestMuzzle_FlagsContainerInspectOnly(t *testing.T) {
	tests := []struct {
		name        string
		writeEnable bool
		method      string
		path        string
		wantFlag    bool
	}{
		{name: "read-only inspect", method: http.MethodGet, path: "/containers/abc/json", wantFlag: true},
		{name: "versioned inspect", method: http.MethodGet, path: "/v1.44/containers/abc/json", wantFlag: true},
		{name: "dot segment inspect", method: http.MethodGet, path: "/containers/abc/./json", wantFlag: true},
		{name: "write-enabled inspect", writeEnable: true, method: http.MethodGet, path: "/containers/abc/json", wantFlag: true},
		{name: "container list", method: http.MethodGet, path: "/containers/json"},
		{name: "image inspect", method: http.MethodGet, path: "/images/nginx/json"},
		{name: "HEAD inspect", writeEnable: true, method: http.MethodHead, path: "/containers/abc/json"},
		{name: "write-enabled POST", writeEnable: true, method: http.MethodPost, path: "/containers/abc/json"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			next := &captureNext{}
			m := NewMuzzle(next, tc.writeEnable, rate.NewLimiter(rate.Inf, 1), nil, "uuid")

			req := httptest.NewRequest(tc.method, tc.path, http.NoBody)
			req.Header.Set("Accept-Encoding", "gzip")
			rec := httptest.NewRecorder()
			m.ServeHTTP(rec, req)

			assert.Equal(t, tc.wantFlag, next.flagged)
			if tc.wantFlag {
				assert.Empty(t, next.acceptEncoding, "compressed inspect bodies must not be requested")
			} else {
				assert.Equal(t, "gzip", next.acceptEncoding)
			}
			// The caller's request keeps its headers.
			assert.Equal(t, "gzip", req.Header.Get("Accept-Encoding"))
		})
	}
}

func TestIsContainerInspect(t *testing.T) {
	assert.True(t, isContainerInspect("/containers/abc/json"))
	assert.False(t, isContainerInspect("/containers/json"))
	assert.False(t, isContainerInspect("/containers/a/b/json"))
	assert.False(t, isContainerInspect("/images/abc/json"))
}

func TestIsEnvRedacted(t *testing.T) {
	assert.False(t, isEnvRedacted(context.Background()))
	assert.True(t, isEnvRedacted(context.WithValue(context.Background(), redactEnvKey{}, true)))
}
