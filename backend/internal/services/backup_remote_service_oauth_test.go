package services

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

// rewriteHostRoundTripper redirects every outgoing request to targetBaseURL
// regardless of the request's original host. Used so CompleteOAuth's tests
// can exercise the real remotestorage.DropboxOAuthConfig/
// GoogleDriveOAuthConfig token-exchange path (hardcoded to the real vendor
// TokenURL in production) against a local httptest.Server instead — never a
// live network call (spec §6 Commit 3 test requirements). oauth2.Config.Exchange
// honors an *http.Client stashed in ctx via oauth2.HTTPClient, which is the
// supported extension point for exactly this kind of test substitution.
type rewriteHostRoundTripper struct {
	targetBaseURL string
}

func (rt *rewriteHostRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	target, err := url.Parse(rt.targetBaseURL)
	if err != nil {
		return nil, err
	}
	req = req.Clone(req.Context())
	req.URL.Scheme = target.Scheme
	req.URL.Host = target.Host
	req.Host = target.Host
	return http.DefaultTransport.RoundTrip(req)
}

func oauthExchangeContext(tokenServerURL string) context.Context {
	client := &http.Client{Transport: &rewriteHostRoundTripper{targetBaseURL: tokenServerURL}}
	return context.WithValue(context.Background(), oauth2.HTTPClient, client)
}

// newFakeTokenExchangeServer issues a fixed access/refresh token pair for
// any Exchange() call it receives, mimicking a successful OAuth2 token
// endpoint response.
func newFakeTokenExchangeServer(t *testing.T, accessToken, refreshToken string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  accessToken,
			"refresh_token": refreshToken,
			"token_type":    "Bearer",
			"expires_in":    3600,
		})
	}))
	t.Cleanup(server.Close)
	return server
}

func newFakeTokenExchangeErrorServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{"error": "invalid_grant"})
	}))
	t.Cleanup(server.Close)
	return server
}

// --- StartOAuth ---

func TestStartOAuth_PublicURLNotConfigured(t *testing.T) {
	db := newRemoteServiceTestDB(t)
	enc := newRemoteServiceCoverageEncryption(t)
	svc := NewBackupRemoteService(db, enc, t.TempDir())

	target, err := svc.Create("Dropbox", "dropbox", true,
		RemoteTargetConfig{Dropbox: &DropboxConfig{AppKey: "app-key"}},
		RemoteTargetSecrets{OAuthClientSecret: "app-secret"})
	require.NoError(t, err)

	_, err = svc.StartOAuth(target.UUID)
	require.ErrorIs(t, err, ErrPublicURLNotConfigured)
}

func TestStartOAuth_TargetNotFound(t *testing.T) {
	db := newRemoteServiceTestDB(t)
	svc := NewBackupRemoteService(db, newRemoteServiceCoverageEncryption(t), t.TempDir())

	_, err := svc.StartOAuth("does-not-exist")
	require.Error(t, err)
}

func TestStartOAuth_UnsupportedType(t *testing.T) {
	db := newRemoteServiceTestDB(t)
	enc := newRemoteServiceCoverageEncryption(t)
	svc := NewBackupRemoteService(db, enc, t.TempDir())
	require.NoError(t, db.Create(&models.Setting{Key: "app.public_url", Value: "https://charon.example.com", Type: "string", Category: "general"}).Error)

	target, err := svc.Create("NAS", "sftp", true, RemoteTargetConfig{Host: "203.0.113.5"}, RemoteTargetSecrets{Password: "x"})
	require.NoError(t, err)

	_, err = svc.StartOAuth(target.UUID)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "oauth is only supported")
}

func TestStartOAuth_Dropbox_Success(t *testing.T) {
	db := newRemoteServiceTestDB(t)
	enc := newRemoteServiceCoverageEncryption(t)
	svc := NewBackupRemoteService(db, enc, t.TempDir())
	require.NoError(t, db.Create(&models.Setting{Key: "app.public_url", Value: "https://charon.example.com", Type: "string", Category: "general"}).Error)

	target, err := svc.Create("Dropbox", "dropbox", true,
		RemoteTargetConfig{Dropbox: &DropboxConfig{AppKey: "app-key"}},
		RemoteTargetSecrets{OAuthClientSecret: "app-secret"})
	require.NoError(t, err)
	assert.Equal(t, "not_connected", target.OAuthStatus, "R2: created in a pending state")

	authorizeURL, err := svc.StartOAuth(target.UUID)
	require.NoError(t, err)
	assert.Contains(t, authorizeURL, "dropbox.com/oauth2/authorize")
	assert.Contains(t, authorizeURL, "client_id=app-key")
	assert.Contains(t, authorizeURL, "redirect_uri=")
	assert.Contains(t, authorizeURL, url.QueryEscape("https://charon.example.com/api/v1/backups/remote-targets/oauth/dropbox/callback"))
	assert.Contains(t, authorizeURL, "state=")
	assert.Contains(t, authorizeURL, "token_access_type=offline")
}

func TestStartOAuth_GoogleDrive_Success(t *testing.T) {
	db := newRemoteServiceTestDB(t)
	enc := newRemoteServiceCoverageEncryption(t)
	svc := NewBackupRemoteService(db, enc, t.TempDir())
	require.NoError(t, db.Create(&models.Setting{Key: "app.public_url", Value: "https://charon.example.com", Type: "string", Category: "general"}).Error)

	target, err := svc.Create("Google Drive", "google_drive", true,
		RemoteTargetConfig{GoogleDrive: &GoogleDriveConfig{ClientID: "client-id"}},
		RemoteTargetSecrets{OAuthClientSecret: "client-secret"})
	require.NoError(t, err)

	authorizeURL, err := svc.StartOAuth(target.UUID)
	require.NoError(t, err)
	assert.Contains(t, authorizeURL, "accounts.google.com/o/oauth2/v2/auth")
	assert.Contains(t, authorizeURL, "client_id=client-id")
	assert.Contains(t, authorizeURL, url.QueryEscape("https://charon.example.com/api/v1/backups/remote-targets/oauth/google_drive/callback"))
	assert.Contains(t, authorizeURL, "access_type=offline")
	assert.Contains(t, authorizeURL, "prompt=consent")
}

// --- CompleteOAuth ---

func TestCompleteOAuth_Success_Dropbox_PersistsTokensAndConnectedStatus(t *testing.T) {
	db := newRemoteServiceTestDB(t)
	enc := newRemoteServiceCoverageEncryption(t)
	svc := NewBackupRemoteService(db, enc, t.TempDir())

	target, err := svc.Create("Dropbox", "dropbox", true,
		RemoteTargetConfig{Dropbox: &DropboxConfig{AppKey: "app-key"}},
		RemoteTargetSecrets{OAuthClientSecret: "app-secret"})
	require.NoError(t, err)

	tokenServer := newFakeTokenExchangeServer(t, "fresh-access-token", "fresh-refresh-token")
	ctx := oauthExchangeContext(tokenServer.URL)

	err = svc.CompleteOAuth(ctx, target.UUID, "dropbox", "auth-code", "https://charon.example.com")
	require.NoError(t, err)

	reloaded, err := svc.Get(target.UUID)
	require.NoError(t, err)
	assert.Equal(t, "connected", reloaded.OAuthStatus)
	require.NotNil(t, reloaded.OAuthConnectedAt)

	secrets, err := svc.decryptSecrets(reloaded)
	require.NoError(t, err)
	assert.Equal(t, "fresh-access-token", secrets.OAuthAccessToken)
	assert.Equal(t, "fresh-refresh-token", secrets.OAuthRefreshToken)
	assert.NotEmpty(t, secrets.OAuthExpiresAt)
	// The App secret entered at Create time must survive the token-exchange
	// re-encrypt (only the OAuth* fields are ever overwritten).
	assert.Equal(t, "app-secret", secrets.OAuthClientSecret)
}

func TestCompleteOAuth_Success_GoogleDrive_PersistsTokensAndConnectedStatus(t *testing.T) {
	db := newRemoteServiceTestDB(t)
	enc := newRemoteServiceCoverageEncryption(t)
	svc := NewBackupRemoteService(db, enc, t.TempDir())

	target, err := svc.Create("Google Drive", "google_drive", true,
		RemoteTargetConfig{GoogleDrive: &GoogleDriveConfig{ClientID: "client-id"}},
		RemoteTargetSecrets{OAuthClientSecret: "client-secret"})
	require.NoError(t, err)

	tokenServer := newFakeTokenExchangeServer(t, "drive-access-token", "drive-refresh-token")
	ctx := oauthExchangeContext(tokenServer.URL)

	err = svc.CompleteOAuth(ctx, target.UUID, "google_drive", "auth-code", "https://charon.example.com")
	require.NoError(t, err)

	reloaded, err := svc.Get(target.UUID)
	require.NoError(t, err)
	assert.Equal(t, "connected", reloaded.OAuthStatus)

	secrets, err := svc.decryptSecrets(reloaded)
	require.NoError(t, err)
	assert.Equal(t, "drive-access-token", secrets.OAuthAccessToken)
	assert.Equal(t, "drive-refresh-token", secrets.OAuthRefreshToken)
}

func TestCompleteOAuth_ProviderMismatch(t *testing.T) {
	db := newRemoteServiceTestDB(t)
	enc := newRemoteServiceCoverageEncryption(t)
	svc := NewBackupRemoteService(db, enc, t.TempDir())

	target, err := svc.Create("Dropbox", "dropbox", true,
		RemoteTargetConfig{Dropbox: &DropboxConfig{AppKey: "app-key"}},
		RemoteTargetSecrets{OAuthClientSecret: "app-secret"})
	require.NoError(t, err)

	err = svc.CompleteOAuth(context.Background(), target.UUID, "google_drive", "auth-code", "https://charon.example.com")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "does not match")
}

func TestCompleteOAuth_TargetNotFound(t *testing.T) {
	db := newRemoteServiceTestDB(t)
	svc := NewBackupRemoteService(db, newRemoteServiceCoverageEncryption(t), t.TempDir())

	err := svc.CompleteOAuth(context.Background(), "does-not-exist", "dropbox", "code", "https://charon.example.com")
	require.Error(t, err)
}

func TestCompleteOAuth_EncryptionKeyMissing(t *testing.T) {
	db := newRemoteServiceTestDB(t)
	enc := newRemoteServiceCoverageEncryption(t)
	svc := NewBackupRemoteService(db, enc, t.TempDir())

	target, err := svc.Create("Dropbox", "dropbox", true,
		RemoteTargetConfig{Dropbox: &DropboxConfig{AppKey: "app-key"}},
		RemoteTargetSecrets{OAuthClientSecret: "app-secret"})
	require.NoError(t, err)

	svc.encryption = nil
	err = svc.CompleteOAuth(context.Background(), target.UUID, "dropbox", "code", "https://charon.example.com")
	require.ErrorIs(t, err, ErrEncryptionKeyMissing)
}

func TestCompleteOAuth_ExchangeFailure_PropagatesError(t *testing.T) {
	db := newRemoteServiceTestDB(t)
	enc := newRemoteServiceCoverageEncryption(t)
	svc := NewBackupRemoteService(db, enc, t.TempDir())

	target, err := svc.Create("Dropbox", "dropbox", true,
		RemoteTargetConfig{Dropbox: &DropboxConfig{AppKey: "app-key"}},
		RemoteTargetSecrets{OAuthClientSecret: "app-secret"})
	require.NoError(t, err)

	tokenServer := newFakeTokenExchangeErrorServer(t)
	ctx := oauthExchangeContext(tokenServer.URL)

	err = svc.CompleteOAuth(ctx, target.UUID, "dropbox", "bad-code", "https://charon.example.com")
	require.Error(t, err)

	reloaded, getErr := svc.Get(target.UUID)
	require.NoError(t, getErr)
	assert.Equal(t, "not_connected", reloaded.OAuthStatus, "a failed exchange must never transition the target to connected")
}

// --- DisconnectOAuth ---

func TestDisconnectOAuth_ClearsTokensAndStatus(t *testing.T) {
	db := newRemoteServiceTestDB(t)
	enc := newRemoteServiceCoverageEncryption(t)
	svc := NewBackupRemoteService(db, enc, t.TempDir())

	target, err := svc.Create("Dropbox", "dropbox", true,
		RemoteTargetConfig{Dropbox: &DropboxConfig{AppKey: "app-key"}},
		RemoteTargetSecrets{OAuthClientSecret: "app-secret"})
	require.NoError(t, err)

	tokenServer := newFakeTokenExchangeServer(t, "access", "refresh")
	require.NoError(t, svc.CompleteOAuth(oauthExchangeContext(tokenServer.URL), target.UUID, "dropbox", "code", "https://charon.example.com"))

	disconnected, err := svc.DisconnectOAuth(target.UUID)
	require.NoError(t, err)
	assert.Equal(t, "not_connected", disconnected.OAuthStatus)
	assert.Nil(t, disconnected.OAuthConnectedAt)

	secrets, err := svc.decryptSecrets(disconnected)
	require.NoError(t, err)
	assert.Empty(t, secrets.OAuthAccessToken)
	assert.Empty(t, secrets.OAuthRefreshToken)
	assert.Empty(t, secrets.OAuthExpiresAt)
	assert.Equal(t, "app-secret", secrets.OAuthClientSecret, "the App secret itself is not an OAuth token and must survive disconnect")
}

func TestDisconnectOAuth_UnsupportedType(t *testing.T) {
	db := newRemoteServiceTestDB(t)
	enc := newRemoteServiceCoverageEncryption(t)
	svc := NewBackupRemoteService(db, enc, t.TempDir())

	target, err := svc.Create("NAS", "sftp", true, RemoteTargetConfig{Host: "203.0.113.5"}, RemoteTargetSecrets{Password: "x"})
	require.NoError(t, err)

	_, err = svc.DisconnectOAuth(target.UUID)
	require.Error(t, err)
}

func TestDisconnectOAuth_NeverConnected_NoOp(t *testing.T) {
	db := newRemoteServiceTestDB(t)
	enc := newRemoteServiceCoverageEncryption(t)
	svc := NewBackupRemoteService(db, enc, t.TempDir())

	target, err := svc.Create("Dropbox", "dropbox", true,
		RemoteTargetConfig{Dropbox: &DropboxConfig{AppKey: "app-key"}},
		RemoteTargetSecrets{OAuthClientSecret: "app-secret"})
	require.NoError(t, err)

	disconnected, err := svc.DisconnectOAuth(target.UUID)
	require.NoError(t, err)
	assert.Equal(t, "not_connected", disconnected.OAuthStatus)
}

func TestDisconnectOAuth_TargetNotFound(t *testing.T) {
	db := newRemoteServiceTestDB(t)
	svc := NewBackupRemoteService(db, newRemoteServiceCoverageEncryption(t), t.TempDir())

	_, err := svc.DisconnectOAuth("does-not-exist")
	require.Error(t, err)
}

// --- ConsumeOAuthState / ConfiguredPublicURL passthroughs ---

func TestConsumeOAuthState_DelegatesToStore(t *testing.T) {
	db := newRemoteServiceTestDB(t)
	svc := NewBackupRemoteService(db, newRemoteServiceCoverageEncryption(t), t.TempDir())

	state, err := svc.states.Issue("target-uuid", "dropbox")
	require.NoError(t, err)

	targetUUID, provider, ok := svc.ConsumeOAuthState(state)
	require.True(t, ok)
	assert.Equal(t, "target-uuid", targetUUID)
	assert.Equal(t, "dropbox", provider)
}

func TestConfiguredPublicURL_DelegatesToUtils(t *testing.T) {
	db := newRemoteServiceTestDB(t)
	svc := NewBackupRemoteService(db, newRemoteServiceCoverageEncryption(t), t.TempDir())

	_, ok := svc.ConfiguredPublicURL()
	assert.False(t, ok)

	require.NoError(t, db.Create(&models.Setting{Key: "app.public_url", Value: "https://charon.example.com", Type: "string", Category: "general"}).Error)
	baseURL, ok := svc.ConfiguredPublicURL()
	require.True(t, ok)
	assert.Equal(t, "https://charon.example.com", baseURL)
}

// --- remoteTargetTokenSaver (the new Commit-3 TokenSaver wiring) ---

// TestRemoteTargetTokenSaver_SaveToken_PersistsRefreshedTokenToDB is the
// required (§6 Commit 3) token-refresh-persists test at the
// BackupRemoteService integration level: a refreshed token handed to
// remoteTargetTokenSaver.SaveToken must actually land in the target's
// encrypted secrets in the database, not just in memory.
func TestRemoteTargetTokenSaver_SaveToken_PersistsRefreshedTokenToDB(t *testing.T) {
	db := newRemoteServiceTestDB(t)
	enc := newRemoteServiceCoverageEncryption(t)
	svc := NewBackupRemoteService(db, enc, t.TempDir())

	target, err := svc.Create("Dropbox", "dropbox", true,
		RemoteTargetConfig{Dropbox: &DropboxConfig{AppKey: "app-key"}},
		RemoteTargetSecrets{OAuthClientSecret: "app-secret", OAuthAccessToken: "stale-access", OAuthRefreshToken: "stale-refresh", OAuthExpiresAt: time.Now().Add(-time.Hour).UTC().Format(time.RFC3339)})
	require.NoError(t, err)

	saver := &remoteTargetTokenSaver{svc: svc, target: target}
	expiresAt := time.Now().Add(time.Hour).UTC()
	require.NoError(t, saver.SaveToken(context.Background(), "refreshed-access", "refreshed-refresh", expiresAt))

	// Reload from the DB via a *fresh* target row (not the in-memory
	// `target` pointer) to prove the write actually reached the database.
	reloaded, err := svc.Get(target.UUID)
	require.NoError(t, err)
	secrets, err := svc.decryptSecrets(reloaded)
	require.NoError(t, err)
	assert.Equal(t, "refreshed-access", secrets.OAuthAccessToken)
	assert.Equal(t, "refreshed-refresh", secrets.OAuthRefreshToken)
	assert.Equal(t, expiresAt.Format(time.RFC3339), secrets.OAuthExpiresAt)
}

func TestRemoteTargetTokenSaver_SaveToken_EncryptionKeyMissing(t *testing.T) {
	db := newRemoteServiceTestDB(t)
	enc := newRemoteServiceCoverageEncryption(t)
	svc := NewBackupRemoteService(db, enc, t.TempDir())

	target, err := svc.Create("Dropbox", "dropbox", true,
		RemoteTargetConfig{Dropbox: &DropboxConfig{AppKey: "app-key"}},
		RemoteTargetSecrets{OAuthClientSecret: "app-secret"})
	require.NoError(t, err)

	svc.encryption = nil
	saver := &remoteTargetTokenSaver{svc: svc, target: target}
	err = saver.SaveToken(context.Background(), "access", "refresh", time.Now())
	require.ErrorIs(t, err, ErrEncryptionKeyMissing)
}
