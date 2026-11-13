package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// decodeReloadErrorBody parses a reload error body into its error and error_code fields.
func decodeReloadErrorBody(t *testing.T, raw string) (message, code string) {
	t.Helper()
	var body map[string]any
	require.NoError(t, json.Unmarshal([]byte(raw), &body))
	message, _ = body["error"].(string)
	code, _ = body["error_code"].(string)
	return message, code
}

func TestSecurityHandler_ReloadGeoIP_LoadErrorHidesPathAndCarriesCode(t *testing.T) {
	missingDir := t.TempDir()
	missingPath := filepath.Join(missingDir, "missing", "GeoLite2-Country.mmdb")

	h := NewSecurityHandler(config.SecurityConfig{}, openGeoIPReasonTestDB(t), nil)
	h.SetGeoIPService(services.NewUnloadedGeoIPService(missingPath))

	w := httptest.NewRecorder()
	reloadGeoIPRouter(h).ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/security/geoip/reload", http.NoBody))

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	message, code := decodeReloadErrorBody(t, w.Body.String())
	assert.Equal(t, "Failed to reload the GeoIP database", message)
	assert.Equal(t, "geoip_reload_failed", code)
	assert.NotContains(t, w.Body.String(), missingDir, "the body must not expose a filesystem path")
	assert.NotContains(t, strings.ToLower(w.Body.String()), "no such file", "the body must not carry OS error text")
}

func TestSecurityHandler_ReloadGeoIP_ServiceMissingCarriesCode(t *testing.T) {
	h := NewSecurityHandler(config.SecurityConfig{}, nil, nil)

	w := httptest.NewRecorder()
	reloadGeoIPRouter(h).ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/security/geoip/reload", http.NoBody))

	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
	_, code := decodeReloadErrorBody(t, w.Body.String())
	assert.Equal(t, "geoip_service_unavailable", code)
}

func TestSecurityHandler_ReloadGeoIP_ApplyFailureCarriesCodeWithoutDetail(t *testing.T) {
	path := filepath.Join(t.TempDir(), "GeoLite2-Country.mmdb")
	require.NoError(t, os.WriteFile(path, minimalCountryMMDB(), 0o600))
	svc, err := services.NewGeoIPService(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })

	mgr := &countingCaddyManager{applyErr: errors.New("caddy refused the config at /secret/location")}
	h := NewSecurityHandler(config.SecurityConfig{}, openGeoIPReasonTestDB(t), mgr)
	h.SetGeoIPService(svc)

	w := httptest.NewRecorder()
	reloadGeoIPRouter(h).ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/security/geoip/reload", http.NoBody))

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	message, code := decodeReloadErrorBody(t, w.Body.String())
	assert.Equal(t, msgModuleApplyFailed, message)
	assert.Equal(t, "proxy_apply_failed", code)
	assert.NotContains(t, w.Body.String(), "/secret/location")
}
