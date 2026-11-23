package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// postImportBody sends a JSON body to path on router and returns the recorded answer.
func postImportBody(t *testing.T, router *gin.Engine, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// jsonPreviewSessionID uploads a Charon export with one host and returns the session id of its preview.
func jsonPreviewSessionID(t *testing.T, router *gin.Engine, domain, forwardHost string) string {
	t.Helper()
	export := CharonExport{
		Version: "1.0.0",
		ProxyHosts: []CharonProxyHost{{
			UUID:          "uuid-" + domain,
			Name:          domain,
			DomainNames:   domain,
			ForwardScheme: "http",
			ForwardHost:   forwardHost,
			ForwardPort:   8080,
			Enabled:       true,
		}},
	}
	content, err := json.Marshal(export)
	require.NoError(t, err)
	w := postImportBody(t, router, "/api/v1/import/json/upload", map[string]string{"content": string(content)})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var body struct {
		Session struct {
			ID string `json:"id"`
		} `json:"session"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.NotEmpty(t, body.Session.ID)
	return body.Session.ID
}

// The JSON import commit reports a refused forward target with the same text the host form shows.
func TestJSONImportCommitReportsARefusedForwardTargetWithTheHostFormText(t *testing.T) {
	db := setupJSONTestDB(t)
	router := gin.New()
	NewJSONImportHandler(db).RegisterRoutes(router.Group("/api/v1"))
	sessionID := jsonPreviewSessionID(t, router, "json-forward.example.com", "orthrus:edge-agent")

	w := postImportBody(t, router, "/api/v1/import/json/commit", map[string]any{"session_uuid": sessionID, "resolutions": map[string]string{}})

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var body struct {
		Created int      `json:"created"`
		Errors  []string `json:"errors"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Zero(t, body.Created)
	require.Len(t, body.Errors, 1)
	assert.Equal(t, "json-forward.example.com: "+forwardTargetRefusedText, body.Errors[0])
}

// The NPM import commit reports a refused forward target with the same text the host form shows.
func TestNPMImportCommitReportsARefusedForwardTargetWithTheHostFormText(t *testing.T) {
	db := setupNPMTestDB(t)
	router := gin.New()
	NewNPMImportHandler(db).RegisterRoutes(router.Group("/api/v1"))
	export := NPMExport{ProxyHosts: []NPMProxyHost{{
		ID:          1,
		DomainNames: []string{"npm-forward.example.com"},
		ForwardHost: "orthrus:edge-agent",
		ForwardPort: 8080,
		Enabled:     true,
	}}}
	content, err := json.Marshal(export)
	require.NoError(t, err)
	upload := postImportBody(t, router, "/api/v1/import/npm/upload", map[string]string{"content": string(content)})
	require.Equal(t, http.StatusOK, upload.Code, upload.Body.String())
	var preview struct {
		Session struct {
			ID string `json:"id"`
		} `json:"session"`
	}
	require.NoError(t, json.Unmarshal(upload.Body.Bytes(), &preview))

	w := postImportBody(t, router, "/api/v1/import/npm/commit", map[string]any{"session_uuid": preview.Session.ID, "resolutions": map[string]string{}})

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var body struct {
		Errors []string `json:"errors"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	require.Len(t, body.Errors, 1)
	assert.Equal(t, "npm-forward.example.com: "+forwardTargetRefusedText, body.Errors[0])
}

// The import session store holds four previews. A fifth preview evicts the oldest, whose commit then answers 404,
// while the newer previews still commit.
func TestJSONImportFifthPreviewEvictsTheFirst(t *testing.T) {
	db := setupJSONTestDB(t)
	router := gin.New()
	NewJSONImportHandler(db).RegisterRoutes(router.Group("/api/v1"))

	var sessionIDs []string
	for i := 0; i < maxImportSessions+1; i++ {
		domain := fmt.Sprintf("evict-%d.example.com", i)
		sessionIDs = append(sessionIDs, jsonPreviewSessionID(t, router, domain, "10.0.1."+fmt.Sprint(i+1)))
	}

	evicted := postImportBody(t, router, "/api/v1/import/json/commit", map[string]any{"session_uuid": sessionIDs[0], "resolutions": map[string]string{}})
	assert.Equal(t, http.StatusNotFound, evicted.Code, evicted.Body.String())
	assert.True(t, strings.Contains(evicted.Body.String(), "session not found or expired"))

	newest := postImportBody(t, router, "/api/v1/import/json/commit", map[string]any{"session_uuid": sessionIDs[maxImportSessions], "resolutions": map[string]string{}})
	assert.Equal(t, http.StatusOK, newest.Code, newest.Body.String())
}
