package handlers

import (
	"bytes"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func writeConfigFile(t *testing.T, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "config.yaml")
	require.NoError(t, os.WriteFile(p, []byte(content), 0o600))
	return p
}

func TestValidateYAMLFile(t *testing.T) {
	tests := []struct {
		name    string
		content string
		wantErr string // empty means valid
	}{
		{"empty file", "", "config.yaml is empty"},
		{"comment only", "# nothing here\n", "config.yaml is empty"},
		{"list root", "- api\n- common\n", "config.yaml must be a YAML mapping"},
		{"scalar root", "api\n", "config.yaml must be a YAML mapping"},
		{"duplicate key", "api:\n  a: 1\napi:\n  b: 2\n", "config.yaml is not valid YAML"},
		{"non-string key", "1: x\napi: {}\n", "config.yaml is not valid YAML"},
		{"broken yaml containing api text", "api:\n  server: [unclosed\n", "config.yaml is not valid YAML"},
		{"unknown keys only", "foo: bar\nserver: {}\n", "config.yaml has no CrowdSec configuration sections"},
		{"disable flag only", "disable_agent: true\n", "config.yaml has no CrowdSec configuration sections"},
		{"minimal api", "api:\n  server:\n    listen_uri: 0.0.0.0:8080\n", ""},
		{"common only", "common:\n  log_level: info\n", ""},
		{"first document only", "api: {}\n---\n- list\n", ""},
		{"first document invalid", "foo: bar\n---\napi: {}\n", "config.yaml has no CrowdSec configuration sections"},
		{"later document unparsed", "api: {}\n---\nbad: [unclosed\n", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateYAMLFile(writeConfigFile(t, tt.content))
			if tt.wantErr == "" {
				require.NoError(t, err)
				return
			}
			require.EqualError(t, err, tt.wantErr)
		})
	}
}

func TestValidateYAMLFileRejectsOversize(t *testing.T) {
	content := "api: {}\n# " + strings.Repeat("x", maxCrowdsecFileBytes) + "\n"
	require.EqualError(t, validateYAMLFile(writeConfigFile(t, content)), "config.yaml is too large")
}

func TestValidateYAMLFileMissingFile(t *testing.T) {
	err := validateYAMLFile(filepath.Join(t.TempDir(), "missing.yaml"))
	require.ErrorContains(t, err, "failed to read file")
}

func TestValidateYAMLFileDefaultConfigFixture(t *testing.T) {
	// testdata/crowdsec_default_config.yaml is the stock config.yaml shipped in the Charon image
	// (/etc/crowdsec.dist/config.yaml, captured from the E2E container).
	require.NoError(t, validateYAMLFile(filepath.Join("testdata", "crowdsec_default_config.yaml")))
}

func TestValidateYAMLFileImportableFixture(t *testing.T) {
	require.NoError(t, validateYAMLFile(writeConfigFile(t, importableConfig)))
}

func TestValidateYAMLFileErrorsHideParserDetail(t *testing.T) {
	err := validateYAMLFile(writeConfigFile(t, "api:\n  server: [unclosed\n"))
	require.Error(t, err)
	require.NotContains(t, err.Error(), "line")
	require.NotContains(t, err.Error(), "yaml:")
}

func TestParseYAMLDocumentsEmptyStream(t *testing.T) {
	_, err := parseYAMLDocuments(strings.NewReader(""), false)
	require.ErrorIs(t, err, io.EOF)
}

func importRequestNamed(t *testing.T, filename string, data []byte) *http.Request {
	t.Helper()
	buf := &bytes.Buffer{}
	mw := multipart.NewWriter(buf)
	fw, err := mw.CreateFormFile("file", filename)
	require.NoError(t, err)
	_, err = fw.Write(data)
	require.NoError(t, err)
	require.NoError(t, mw.Close())
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/import", buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	return req
}

func TestImportConfigRejectsNonTarGzUploads(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"cfg.zip", "cfg.ZIP", "cfg.tgz", "cfg.txt", "cfg"} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			_, r := newImportRouter(t, dir)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, importRequestNamed(t, name, []byte("not an archive")))
			require.Equal(t, http.StatusUnprocessableEntity, w.Code, w.Body.String())
			require.JSONEq(t, `{"error":"validation failed: only .tar.gz archives are supported"}`, w.Body.String())
		})
	}
}

func TestImportConfigConfigYAMLValidationMessages(t *testing.T) {
	t.Parallel()
	cases := map[string]struct{ content, want string }{
		"empty":     {"# only a comment\n", "config validation failed: config.yaml is empty"},
		"list root": {"- api\n", "config validation failed: config.yaml must be a YAML mapping"},
		"no keys":   {"foo: bar\n", "config validation failed: config.yaml has no CrowdSec configuration sections"},
		"invalid":   {"api: [unclosed\n", "config validation failed: config.yaml is not valid YAML"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := t.TempDir()
			_, r := newImportRouter(t, dir)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, importRequest(t, map[string]string{"config.yaml": tc.content}))
			require.Equal(t, http.StatusUnprocessableEntity, w.Code, w.Body.String())
			require.JSONEq(t, `{"error":"`+tc.want+`"}`, w.Body.String())
		})
	}
}

// validateWithin runs validateYAMLFile and fails the test if it does not return within the bound.
func validateWithin(t *testing.T, content string) error {
	const bound = 10 * time.Second
	t.Helper()
	path := writeConfigFile(t, content)
	done := make(chan error, 1)
	go func() { done <- validateYAMLFile(path) }()
	select {
	case err := <-done:
		return err
	case <-time.After(bound):
		t.Fatalf("validateYAMLFile did not return within %s", bound)
		return nil
	}
}

// aliasBombDocument builds a small document whose fully expanded form is fanout^levels nodes.
func aliasBombDocument(levels, fanout int) string {
	var b strings.Builder
	b.WriteString("api:\n  server: {}\n")
	b.WriteString("l0: &l0 [" + strings.TrimSuffix(strings.Repeat("x,", fanout), ",") + "]\n")
	for i := 1; i <= levels; i++ {
		refs := make([]string, fanout)
		for j := range refs {
			refs[j] = fmt.Sprintf("*l%d", i-1)
		}
		fmt.Fprintf(&b, "l%d: &l%d [%s]\n", i, i, strings.Join(refs, ", "))
	}
	return b.String()
}

func TestValidateYAMLFileRejectsAliasExpansionBomb(t *testing.T) {
	doc := aliasBombDocument(10, 9)
	require.Less(t, len(doc), 1024, "the bomb must stay tiny on disk")

	start := time.Now()
	err := validateWithin(t, doc)
	require.EqualError(t, err, "config.yaml is not valid YAML")
	require.Less(t, time.Since(start), 5*time.Second)
}

func TestValidateYAMLFileAcceptsModerateAliasing(t *testing.T) {
	// Two levels of 9-way reuse (81 expanded nodes) is ordinary anchor reuse and must still be accepted.
	require.NoError(t, validateWithin(t, aliasBombDocument(2, 9)))
}

func TestValidateYAMLFileNesting(t *testing.T) {
	nested := func(depth int) string {
		return "api:\n  server: " + strings.Repeat("[", depth) + strings.Repeat("]", depth) + "\n"
	}

	t.Run("moderate nesting is accepted", func(t *testing.T) {
		require.NoError(t, validateWithin(t, nested(20)))
	})

	t.Run("very deep nesting returns promptly without panicking", func(t *testing.T) {
		err := validateWithin(t, nested(5000))
		if err != nil {
			require.EqualError(t, err, "config.yaml is not valid YAML")
		}
	})
}
