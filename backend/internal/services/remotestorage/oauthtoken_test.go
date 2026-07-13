package remotestorage

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/oauth2"
)

// spyTokenSaver is a minimal TokenSaver fake that records every call, used
// to prove NewClient's persisting layer only saves when a refresh actually
// changes the access token (spec §3.5).
type spyTokenSaver struct {
	mu    sync.Mutex
	calls int
	saved struct {
		accessToken, refreshToken string
		expiresAt                 time.Time
	}
	err error
}

func (s *spyTokenSaver) SaveToken(_ context.Context, accessToken, refreshToken string, expiresAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.calls++
	s.saved.accessToken = accessToken
	s.saved.refreshToken = refreshToken
	s.saved.expiresAt = expiresAt
	return s.err
}

func (s *spyTokenSaver) callCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

// newFakeOAuthTokenServer returns an httptest.Server that always issues a
// fresh access token in response to a refresh_token grant, so
// oauth2.Config's TokenSource is forced to actually perform (and this test
// to observe) a refresh round trip.
func newFakeOAuthTokenServer(t *testing.T, accessToken string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"access_token":  accessToken,
			"refresh_token": "new-refresh-token",
			"token_type":    "Bearer",
			"expires_in":    3600,
		})
	}))
	t.Cleanup(server.Close)
	return server
}

// newFakeOAuthErrorServer returns an httptest.Server that always responds
// with the RFC 6749 "invalid_grant" error, simulating a revoked refresh
// token (spec R6).
func newFakeOAuthErrorServer(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]any{
			"error":             "invalid_grant",
			"error_description": "refresh token revoked",
		})
	}))
	t.Cleanup(server.Close)
	return server
}

func TestOAuthNewClient_PersistsRefreshedToken(t *testing.T) {
	tokenServer := newFakeOAuthTokenServer(t, "fresh-access-token")

	var authHeader string
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(apiServer.Close)

	conf := oauth2.Config{
		ClientID:     "client-id",
		ClientSecret: "client-secret",
		Endpoint:     oauth2.Endpoint{TokenURL: tokenServer.URL},
	}
	expired := &oauth2.Token{
		AccessToken:  "stale-access-token",
		RefreshToken: "stale-refresh-token",
		Expiry:       time.Now().Add(-time.Hour), // force a refresh on first Token()
	}

	saver := &spyTokenSaver{}
	client := NewClient(context.Background(), conf, expired, saver)

	req, err := http.NewRequest(http.MethodGet, apiServer.URL, http.NoBody)
	require.NoError(t, err)
	resp, err := client.Do(req)
	require.NoError(t, err)
	_ = resp.Body.Close()

	assert.Equal(t, "Bearer fresh-access-token", authHeader)
	assert.Equal(t, 1, saver.callCount())
	assert.Equal(t, "fresh-access-token", saver.saved.accessToken)
	assert.Equal(t, "new-refresh-token", saver.saved.refreshToken)

	// A second request with the still-valid refreshed token must not save
	// again (the access token hasn't changed since the last observation).
	req2, err := http.NewRequest(http.MethodGet, apiServer.URL, http.NoBody)
	require.NoError(t, err)
	resp2, err := client.Do(req2)
	require.NoError(t, err)
	_ = resp2.Body.Close()
	assert.Equal(t, 1, saver.callCount(), "an unchanged access token must not trigger a second SaveToken call")
}

func TestOAuthNewClient_NilSaver_DoesNotPanic(t *testing.T) {
	tokenServer := newFakeOAuthTokenServer(t, "fresh-access-token")
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(apiServer.Close)

	conf := oauth2.Config{Endpoint: oauth2.Endpoint{TokenURL: tokenServer.URL}}
	expired := &oauth2.Token{AccessToken: "stale", RefreshToken: "stale-refresh", Expiry: time.Now().Add(-time.Hour)}

	client := NewClient(context.Background(), conf, expired, nil)

	req, err := http.NewRequest(http.MethodGet, apiServer.URL, http.NoBody)
	require.NoError(t, err)

	require.NotPanics(t, func() {
		resp, doErr := client.Do(req)
		require.NoError(t, doErr)
		_ = resp.Body.Close()
	})
}

func TestOAuthNewClient_RefreshFailsWithInvalidGrant_TranslatesToErrOAuthRevoked(t *testing.T) {
	tokenServer := newFakeOAuthErrorServer(t)
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(apiServer.Close)

	conf := oauth2.Config{Endpoint: oauth2.Endpoint{TokenURL: tokenServer.URL}}
	expired := &oauth2.Token{AccessToken: "stale", RefreshToken: "revoked-refresh-token", Expiry: time.Now().Add(-time.Hour)}

	saver := &spyTokenSaver{}
	client := NewClient(context.Background(), conf, expired, saver)

	req, err := http.NewRequest(http.MethodGet, apiServer.URL, http.NoBody)
	require.NoError(t, err)

	_, err = client.Do(req) //nolint:bodyclose // Do returns a nil response on transport error, nothing to close
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrOAuthRevoked)
	assert.Equal(t, 0, saver.callCount(), "a failed refresh must never call SaveToken")
}

func TestOAuthNewClient_SaveTokenError_PropagatesAsTransportError(t *testing.T) {
	tokenServer := newFakeOAuthTokenServer(t, "fresh-access-token")
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(apiServer.Close)

	conf := oauth2.Config{Endpoint: oauth2.Endpoint{TokenURL: tokenServer.URL}}
	expired := &oauth2.Token{AccessToken: "stale", RefreshToken: "stale-refresh", Expiry: time.Now().Add(-time.Hour)}

	saver := &spyTokenSaver{err: assertAnOAuthError}
	client := NewClient(context.Background(), conf, expired, saver)

	req, err := http.NewRequest(http.MethodGet, apiServer.URL, http.NoBody)
	require.NoError(t, err)

	_, err = client.Do(req) //nolint:bodyclose // Do returns a nil response on transport error, nothing to close
	require.Error(t, err)
	assert.Contains(t, err.Error(), "persist refreshed token")
}

var assertAnOAuthError = &oauthTestBoomError{}

type oauthTestBoomError struct{}

func (*oauthTestBoomError) Error() string { return "boom: simulated save failure" }
