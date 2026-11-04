package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"

	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"net/url"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/crowdsec"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

// recordLAPIAddressValidator records every address a CrowdSec reader asks to validate and rejects it, so the
// handler takes its fallback path without any network call.
func recordLAPIAddressValidator(seen *[]string) func(string) (*url.URL, error) {
	return func(raw string) (*url.URL, error) {
		*seen = append(*seen, raw)
		return nil, errors.New("recorded in test")
	}
}

// TestCrowdSecReaders_UseLocalEngineAddress checks that every reader validates the local engine address, even
// when the stored settings row carries a different address.
func TestCrowdSecReaders_UseLocalEngineAddress(t *testing.T) {
	db := setupCrowdDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}, &models.SecurityDecision{}, &models.Setting{}))
	require.NoError(t, db.Create(&models.SecurityConfig{
		Name: models.DefaultSecurityConfigName, UUID: "default", CrowdSecAPIURL: "http://other.example:9999",
	}).Error)

	h := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", t.TempDir())
	h.Security = services.NewSecurityService(db)
	t.Cleanup(func() { h.Security.Close() })

	var seen []string
	h.validateLAPIURL = recordLAPIAddressValidator(&seen)

	t.Run("decisions", func(t *testing.T) {
		seen = nil
		h.GetLAPIDecisions(lapiTestContext(t))
		assert.Equal(t, []string{crowdsec.LocalLAPIURL}, seen)
	})

	t.Run("health", func(t *testing.T) {
		seen = nil
		h.CheckLAPIHealth(lapiTestContext(t))
		assert.Equal(t, []string{crowdsec.LocalLAPIURL}, seen)
	})

	t.Run("dashboard active decisions", func(t *testing.T) {
		seen = nil
		assert.Equal(t, int64(-1), h.fetchActiveDecisionCount(context.Background()))
		assert.Equal(t, []string{crowdsec.LocalLAPIURL}, seen)
	})
}

// lapiTestContext returns a gin test context with a plain GET request.
func lapiTestContext(t *testing.T) *gin.Context {
	t.Helper()
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	return c
}

// redirectLocalLAPI returns a validator that accepts only the local engine address and maps it to target, so a
// test can point the readers at a mock LAPI server.
func redirectLocalLAPI(target string) func(string) (*url.URL, error) {
	return func(raw string) (*url.URL, error) {
		if raw != crowdsec.LocalLAPIURL {
			return nil, fmt.Errorf("unexpected engine address %q", raw)
		}
		return url.Parse(target)
	}
}
