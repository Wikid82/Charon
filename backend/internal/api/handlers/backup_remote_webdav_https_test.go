package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

// postWebDAVTarget sends a create request for a WebDAV target with the given address and flag.
func postWebDAVTarget(t *testing.T, router http.Handler, name, address string, allowInsecure bool) *httptest.ResponseRecorder {
	t.Helper()
	dav := map[string]any{"url": address}
	if allowInsecure {
		dav["allow_insecure_http"] = true
	}
	body, err := json.Marshal(map[string]any{
		"name":   name,
		"type":   "webdav",
		"config": map[string]any{"webdav": dav},
		"secrets": map[string]any{
			"password": "pw",
		},
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/backups/remote-targets", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	return resp
}

func TestBackupRemoteHandler_Create_WebDAVHTTPWithoutFlag_Returns400WithCode(t *testing.T) {
	router, db := setupBackupRemoteHandlerTest(t, testEncryptionService(t))

	resp := postWebDAVTarget(t, router, "Plain", "http://203.0.113.10/dav/", false)
	require.Equal(t, http.StatusBadRequest, resp.Code, resp.Body.String())

	var body map[string]string
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
	assert.Equal(t, "Use an https:// address, or turn on \"Allow unencrypted HTTP\" for this destination.", body["error"])
	assert.Equal(t, "webdav_https_required", body["error_code"])

	var count int64
	require.NoError(t, db.Model(&models.RemoteStorageTarget{}).Count(&count).Error)
	assert.Zero(t, count)
}

func TestBackupRemoteHandler_Create_WebDAVOtherScheme_Returns400WithMessage(t *testing.T) {
	router, _ := setupBackupRemoteHandlerTest(t, testEncryptionService(t))

	resp := postWebDAVTarget(t, router, "Ftp", "ftp://203.0.113.10/dav/", true)
	require.Equal(t, http.StatusBadRequest, resp.Code, resp.Body.String())

	var body map[string]string
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
	assert.Equal(t, "WebDAV addresses must start with https:// or http://.", body["error"])
	assert.NotContains(t, body, "error_code")
}

func TestBackupRemoteHandler_Create_WebDAVHTTPSAndFlaggedHTTPCreated(t *testing.T) {
	router, db := setupBackupRemoteHandlerTest(t, testEncryptionService(t))

	secure := postWebDAVTarget(t, router, "Secure", "https://203.0.113.10/dav/", false)
	require.Equal(t, http.StatusCreated, secure.Code, secure.Body.String())

	flagged := postWebDAVTarget(t, router, "Flagged", "http://203.0.113.11/dav/", true)
	require.Equal(t, http.StatusCreated, flagged.Code, flagged.Body.String())

	var stored models.RemoteStorageTarget
	require.NoError(t, db.Where("name = ?", "Flagged").First(&stored).Error)
	assert.Contains(t, stored.ConfigJSON, `"allow_insecure_http":true`)
}

// A flagged http target can be saved again with the same address and flag.
func TestBackupRemoteHandler_Update_FlaggedHTTPKeepsWorking(t *testing.T) {
	router, db := setupBackupRemoteHandlerTest(t, testEncryptionService(t))

	created := postWebDAVTarget(t, router, "Flagged", "http://203.0.113.11/dav/", true)
	require.Equal(t, http.StatusCreated, created.Code, created.Body.String())
	var stored models.RemoteStorageTarget
	require.NoError(t, db.Where("name = ?", "Flagged").First(&stored).Error)

	body, err := json.Marshal(map[string]any{
		"name":   "Flagged",
		"config": map[string]any{"webdav": map[string]any{"url": "http://203.0.113.11/dav/", "allow_insecure_http": true}},
	})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPut, "/api/v1/backups/remote-targets/"+stored.UUID, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp := httptest.NewRecorder()
	router.ServeHTTP(resp, req)
	require.Equal(t, http.StatusOK, resp.Code, resp.Body.String())
}
