package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/crowdsec"
	"github.com/Wikid82/charon/backend/internal/crowdsec/bouncerkey"
	"github.com/Wikid82/charon/backend/internal/models"
)

// redirectLocalKeyChecks makes the handler's key checks answer from baseURL, for tests only. The production check
// always addresses the local engine; this only sends the request to a stub server.
func redirectLocalKeyChecks(h *CrowdsecHandler, baseURL string) {
	prev := h.keyCheck
	if prev == nil {
		prev = bouncerkey.Check
	}
	h.keyCheck = func(ctx context.Context, doer bouncerkey.Doer, key string, opts bouncerkey.CheckOptions) bouncerkey.Verdict {
		opts.BaseURL = baseURL
		return prev(ctx, doer, key, opts)
	}
}

// newStubLAPI serves every request with status, as a stand-in for the engine's LAPI.
func newStubLAPI(t *testing.T, status int) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestTestKeyAgainstLAPI_ChecksOnlyTheLocalEngine(t *testing.T) {
	const external = "http://10.255.255.1:9"
	cases := map[string]func(t *testing.T){
		"stored address": func(t *testing.T) {
			db := OpenTestDB(t)
			require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}))
			require.NoError(t, db.Create(&models.SecurityConfig{Name: models.DefaultSecurityConfigName, CrowdSecAPIURL: external}).Error)
		},
		"environment variable": func(t *testing.T) {
			t.Setenv("CHARON_SECURITY_CROWDSEC_API_URL", external)
		},
	}
	for name, setup := range cases {
		t.Run(name, func(t *testing.T) {
			setup(t)
			db := OpenTestDB(t)
			require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}))
			h := &CrowdsecHandler{Security: createTestSecurityService(t, db), keyCheckWait: time.Second}

			var gotBase string
			h.keyCheck = func(_ context.Context, _ bouncerkey.Doer, _ string, opts bouncerkey.CheckOptions) bouncerkey.Verdict {
				gotBase = opts.BaseURL
				return bouncerkey.VerdictAccepted
			}

			assert.Equal(t, bouncerkey.VerdictAccepted, h.testKeyAgainstLAPI(context.Background(), "a-key"))
			assert.Equal(t, crowdsec.LocalLAPIURL, gotBase)
		})
	}
}

// A redirected check still reports the stub's verdict, so the seam changes only the destination.
func TestTestKeyAgainstLAPI_RedirectKeepsTheVerdict(t *testing.T) {
	stub := newStubLAPI(t, http.StatusForbidden)
	h := &CrowdsecHandler{keyCheckWait: time.Second}
	redirectLocalKeyChecks(h, stub.URL)

	assert.Equal(t, bouncerkey.VerdictRejected, h.testKeyAgainstLAPI(context.Background(), "a-key"))
}
