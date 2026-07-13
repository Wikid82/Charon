package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/crypto"
	"github.com/Wikid82/charon/backend/internal/models"
)

func newOAuthHandlerTestEncryption(t *testing.T) *crypto.EncryptionService {
	t.Helper()
	enc, err := crypto.NewEncryptionService("MDEyMzQ1Njc4OWFiY2RlZjAxMjM0NTY3ODlhYmNkZWY=")
	require.NoError(t, err)
	return enc
}

func setPublicURL(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Create(&models.Setting{Key: "app.public_url", Value: "https://charon.example.com", Type: "string", Category: "general"}).Error)
}

func createOAuthTarget(t *testing.T, router httpHandlerHelper, targetType string, config, secrets map[string]any) string {
	t.Helper()
	body, err := json.Marshal(map[string]any{
		"name":    "Test Target",
		"type":    targetType,
		"enabled": true,
		"config":  config,
		"secrets": secrets,
	})
	require.NoError(t, err)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/backups/remote-targets", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())

	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	uuid, _ := resp["uuid"].(string)
	require.NotEmpty(t, uuid)
	return uuid
}

// httpHandlerHelper is the minimal surface createOAuthTarget needs — both
// *gin.Engine (returned by setupBackupRemoteHandlerTest) satisfy
// http.Handler.
type httpHandlerHelper interface {
	ServeHTTP(w http.ResponseWriter, r *http.Request)
}

func TestOAuthStart_PublicURLNotConfigured_Returns400(t *testing.T) {
	router, _ := setupBackupRemoteHandlerTest(t, newOAuthHandlerTestEncryption(t))

	uuid := createOAuthTarget(t, router, "dropbox",
		map[string]any{"dropbox": map[string]any{"app_key": "app-key"}},
		map[string]any{"oauth_client_secret": "app-secret"})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/backups/remote-targets/"+uuid+"/oauth/start", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "public_url_not_configured", resp["error_code"])
}

func TestOAuthStart_Success_ReturnsAuthorizeURL(t *testing.T) {
	router, db := setupBackupRemoteHandlerTest(t, newOAuthHandlerTestEncryption(t))
	setPublicURL(t, db)

	uuid := createOAuthTarget(t, router, "dropbox",
		map[string]any{"dropbox": map[string]any{"app_key": "app-key"}},
		map[string]any{"oauth_client_secret": "app-secret"})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/backups/remote-targets/"+uuid+"/oauth/start", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Contains(t, resp["authorize_url"], "dropbox.com/oauth2/authorize")
}

func TestOAuthCallback_InvalidState_Returns400(t *testing.T) {
	router, db := setupBackupRemoteHandlerTest(t, newOAuthHandlerTestEncryption(t))
	setPublicURL(t, db)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/backups/remote-targets/oauth/dropbox/callback?state=never-issued&code=abc", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusBadRequest, rec.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "invalid_oauth_state", resp["error_code"])
}

func TestOAuthCallback_ProviderDenied_RedirectsWithErrorMessage(t *testing.T) {
	router, db := setupBackupRemoteHandlerTest(t, newOAuthHandlerTestEncryption(t))
	setPublicURL(t, db)

	uuid := createOAuthTarget(t, router, "dropbox",
		map[string]any{"dropbox": map[string]any{"app_key": "app-key"}},
		map[string]any{"oauth_client_secret": "app-secret"})

	startReq := httptest.NewRequest(http.MethodPost, "/api/v1/backups/remote-targets/"+uuid+"/oauth/start", nil)
	startRec := httptest.NewRecorder()
	router.ServeHTTP(startRec, startReq)
	require.Equal(t, http.StatusOK, startRec.Code)
	var startResp map[string]any
	require.NoError(t, json.Unmarshal(startRec.Body.Bytes(), &startResp))
	authorizeURL, _ := startResp["authorize_url"].(string)
	parsed, err := url.Parse(authorizeURL)
	require.NoError(t, err)
	state := parsed.Query().Get("state")
	require.NotEmpty(t, state)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/backups/remote-targets/oauth/dropbox/callback?state="+url.QueryEscape(state)+"&error=access_denied", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusFound, rec.Code)
	location := rec.Header().Get("Location")
	assert.True(t, strings.HasPrefix(location, "https://charon.example.com/backups?"))
	assert.Contains(t, location, "oauth_result=error")
	assert.Contains(t, location, "message=authorization_denied")
}

func TestOAuthCallback_State_IsSingleUse(t *testing.T) {
	router, db := setupBackupRemoteHandlerTest(t, newOAuthHandlerTestEncryption(t))
	setPublicURL(t, db)

	uuid := createOAuthTarget(t, router, "dropbox",
		map[string]any{"dropbox": map[string]any{"app_key": "app-key"}},
		map[string]any{"oauth_client_secret": "app-secret"})

	startReq := httptest.NewRequest(http.MethodPost, "/api/v1/backups/remote-targets/"+uuid+"/oauth/start", nil)
	startRec := httptest.NewRecorder()
	router.ServeHTTP(startRec, startReq)
	var startResp map[string]any
	require.NoError(t, json.Unmarshal(startRec.Body.Bytes(), &startResp))
	authorizeURL, _ := startResp["authorize_url"].(string)
	parsed, err := url.Parse(authorizeURL)
	require.NoError(t, err)
	state := parsed.Query().Get("state")

	// First use: provider-denied path, doesn't require a real token
	// exchange, but does consume the state.
	req1 := httptest.NewRequest(http.MethodGet, "/api/v1/backups/remote-targets/oauth/dropbox/callback?state="+url.QueryEscape(state)+"&error=access_denied", nil)
	rec1 := httptest.NewRecorder()
	router.ServeHTTP(rec1, req1)
	require.Equal(t, http.StatusFound, rec1.Code)

	// Replay: the same state must now be rejected outright.
	req2 := httptest.NewRequest(http.MethodGet, "/api/v1/backups/remote-targets/oauth/dropbox/callback?state="+url.QueryEscape(state)+"&code=abc", nil)
	rec2 := httptest.NewRecorder()
	router.ServeHTTP(rec2, req2)
	require.Equal(t, http.StatusBadRequest, rec2.Code)
	var resp2 map[string]any
	require.NoError(t, json.Unmarshal(rec2.Body.Bytes(), &resp2))
	assert.Equal(t, "invalid_oauth_state", resp2["error_code"])
}

func TestOAuthDisconnect_Success(t *testing.T) {
	router, db := setupBackupRemoteHandlerTest(t, newOAuthHandlerTestEncryption(t))
	setPublicURL(t, db)

	uuid := createOAuthTarget(t, router, "dropbox",
		map[string]any{"dropbox": map[string]any{"app_key": "app-key"}},
		map[string]any{"oauth_client_secret": "app-secret"})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/backups/remote-targets/"+uuid+"/oauth/disconnect", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var resp map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	assert.Equal(t, "not_connected", resp["oauth_status"])
}

func TestOAuthDisconnect_UnsupportedType_ReturnsError(t *testing.T) {
	router, _ := setupBackupRemoteHandlerTest(t, newOAuthHandlerTestEncryption(t))

	uuid := createOAuthTarget(t, router, "sftp",
		map[string]any{"host": "203.0.113.5"},
		map[string]any{"password": "x"})

	req := httptest.NewRequest(http.MethodPost, "/api/v1/backups/remote-targets/"+uuid+"/oauth/disconnect", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)

	assert.NotEqual(t, http.StatusOK, rec.Code)
}

func TestListRemoteTargets_IncludesOAuthStatusField(t *testing.T) {
	router, db := setupBackupRemoteHandlerTest(t, newOAuthHandlerTestEncryption(t))
	setPublicURL(t, db)

	createOAuthTarget(t, router, "google_drive",
		map[string]any{"google_drive": map[string]any{"client_id": "client-id"}},
		map[string]any{"oauth_client_secret": "client-secret"})

	req := httptest.NewRequest(http.MethodGet, "/api/v1/backups/remote-targets", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)

	var resp []map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &resp))
	require.Len(t, resp, 1)
	assert.Equal(t, "not_connected", resp[0]["oauth_status"])
	assert.NotContains(t, resp[0], "oauth_client_secret")
	assert.NotContains(t, resp[0], "oauth_access_token")
}
