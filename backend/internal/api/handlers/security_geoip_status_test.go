package handlers

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/geoip"
	"github.com/Wikid82/charon/backend/internal/services"
)

// getGeoIPStatusBody calls GET /security/geoip/status on h and decodes the JSON object.
func getGeoIPStatusBody(t *testing.T, h *SecurityHandler) map[string]any {
	t.Helper()
	r := gin.New()
	r.GET("/security/geoip/status", h.GetGeoIPStatus)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/security/geoip/status", http.NoBody))
	require.Equal(t, http.StatusOK, w.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	return body
}

// loadedGeoIPService returns a service that has loaded a country database written at path.
func loadedGeoIPService(t *testing.T, path string) *services.GeoIPService {
	t.Helper()
	require.NoError(t, os.WriteFile(path, minimalCountryMMDB(), 0o600))
	svc, err := services.NewGeoIPService(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })
	return svc
}

func TestSecurityHandler_GetGeoIPStatus_ExtendedFieldsWhenLoaded(t *testing.T) {
	path := filepath.Join(t.TempDir(), "GeoLite2-Country.mmdb")
	h := NewSecurityHandler(config.SecurityConfig{}, openGeoIPReasonTestDB(t), nil)
	h.SetGeoIPService(loadedGeoIPService(t, path))

	body := getGeoIPStatusBody(t, h)

	// Existing fields keep their meaning.
	assert.Equal(t, true, body["available"])
	assert.Equal(t, true, body["loaded"])
	assert.Equal(t, path, body["db_path"])
	assert.Equal(t, "GeoIP service available", body["message"])
	assert.NotContains(t, body, "reason", "the reason is present only when the database is unavailable")

	// Card fields. The temporary path is not a known location, so no named source applies.
	assert.Equal(t, string(geoip.SourceNone), body["source"])
	assert.Equal(t, "GeoLite2-Country", body["database_type"])
	assert.Nil(t, body["build_date"], "the fixture carries no build epoch")
	assert.Equal(t, float64(len(minimalCountryMMDB())), body["size_bytes"])
	assert.IsType(t, "", body["loaded_at"])
	assert.IsType(t, false, body["data_copy_present"])
	// Whether the real data folder is writable depends on the host; the reason must agree with the flag.
	assert.Equal(t, body["can_replace"] == true, body["replace_blocked_reason"] == "")
	assert.Equal(t, float64(0), body["unenforced_lists_total"])
	assert.Contains(t, body, "included_build_date", "the field is always present, null when unknown")
}

func TestSecurityHandler_GetGeoIPStatus_CustomPathReportsSourceAndBlock(t *testing.T) {
	path := filepath.Join(t.TempDir(), "custom.mmdb")
	t.Setenv(geoip.EnvDBPath, path)
	h := NewSecurityHandler(config.SecurityConfig{}, openGeoIPReasonTestDB(t), nil)
	h.SetGeoIPService(loadedGeoIPService(t, path))

	body := getGeoIPStatusBody(t, h)

	assert.Equal(t, string(geoip.SourceCustom), body["source"])
	assert.Equal(t, false, body["can_replace"])
	assert.Equal(t, "custom_path_set", body["replace_blocked_reason"])
}

func TestSecurityHandler_GetGeoIPStatus_UnloadedHasNullFields(t *testing.T) {
	h := NewSecurityHandler(config.SecurityConfig{}, openGeoIPReasonTestDB(t), nil)
	h.SetGeoIPService(services.NewUnloadedGeoIPService(filepath.Join(t.TempDir(), "GeoLite2-Country.mmdb")))

	body := getGeoIPStatusBody(t, h)

	assert.Equal(t, false, body["available"])
	assert.Equal(t, geoip.DatabaseFileName, filepath.Base(body["db_path"].(string)))
	assert.Equal(t, "geoip_unavailable", body["reason"])
	assert.Equal(t, string(geoip.SourceNone), body["source"])
	assert.Nil(t, body["build_date"])
	assert.Nil(t, body["size_bytes"])
	assert.Nil(t, body["loaded_at"])
	assert.Equal(t, "", body["database_type"])
}

func TestSecurityHandler_GetGeoIPStatus_NotInitializedReportsNoSource(t *testing.T) {
	h := NewSecurityHandler(config.SecurityConfig{}, nil, nil)

	body := getGeoIPStatusBody(t, h)

	assert.Equal(t, string(geoip.SourceNone), body["source"])
	assert.Equal(t, false, body["can_replace"])
	assert.Equal(t, float64(0), body["unenforced_lists_total"])
}

func TestSecurityHandler_SetGeoIPServiceNilClearsStore(t *testing.T) {
	h := NewSecurityHandler(config.SecurityConfig{}, openGeoIPReasonTestDB(t), nil)
	h.SetGeoIPService(services.NewUnloadedGeoIPService(filepath.Join(t.TempDir(), "GeoLite2-Country.mmdb")))
	h.SetGeoIPService(nil)

	body := getGeoIPStatusBody(t, h)

	assert.Equal(t, string(geoip.SourceNone), body["source"])
	assert.Nil(t, h.geoipStore)
}

func TestSecurityHandler_GetGeoIPStatus_CountsUnenforcedCountryLists(t *testing.T) {
	db := openEnforcementStatusDB(t)
	list := createCountryList(t, db, 1, "Countries", "geo_whitelist", true)
	attachHost(t, db, "site", list, true)
	h := aclOnAlertHandler(t, db, &plainManager{}, unloadedAlertService(t))

	body := getGeoIPStatusBody(t, h)

	assert.Equal(t, float64(1), body["unenforced_lists_total"])
}

// blockingCaddyManager holds ApplyConfig until release is closed, so a second request can find a change running.
type blockingCaddyManager struct {
	started chan struct{}
	release chan struct{}
}

func (m *blockingCaddyManager) ApplyConfig(_ context.Context) error {
	close(m.started)
	<-m.release
	return nil
}

func TestSecurityHandler_ReloadGeoIP_SuccessIncludesStatus(t *testing.T) {
	path := filepath.Join(t.TempDir(), "GeoLite2-Country.mmdb")
	require.NoError(t, os.WriteFile(path, minimalCountryMMDB(), 0o600))
	h := NewSecurityHandler(config.SecurityConfig{}, openGeoIPReasonTestDB(t), &countingCaddyManager{})
	h.SetGeoIPService(services.NewUnloadedGeoIPService(path))

	w := httptest.NewRecorder()
	reloadGeoIPRouter(h).ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/security/geoip/reload", http.NoBody))

	require.Equal(t, http.StatusOK, w.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, true, body["loaded"])
	assert.Equal(t, path, body["db_path"])
	status, ok := body["status"].(map[string]any)
	require.True(t, ok, "the reload body carries the database status")
	assert.Equal(t, "GeoLite2-Country", status["database_type"])
	assert.Equal(t, float64(0), status["unenforced_lists_total"])
	assert.Contains(t, status, "replace_blocked_reason")
}

func TestSecurityHandler_ReloadGeoIP_ConcurrentChangeIsBusy(t *testing.T) {
	path := filepath.Join(t.TempDir(), "GeoLite2-Country.mmdb")
	require.NoError(t, os.WriteFile(path, minimalCountryMMDB(), 0o600))
	mgr := &blockingCaddyManager{started: make(chan struct{}), release: make(chan struct{})}
	h := NewSecurityHandler(config.SecurityConfig{}, openGeoIPReasonTestDB(t), mgr)
	h.SetGeoIPService(services.NewUnloadedGeoIPService(path))
	router := reloadGeoIPRouter(h)

	first := make(chan *httptest.ResponseRecorder, 1)
	go func() {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/security/geoip/reload", http.NoBody))
		first <- w
	}()
	<-mgr.started

	w := httptest.NewRecorder()
	router.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/security/geoip/reload", http.NoBody))
	assert.Equal(t, http.StatusConflict, w.Code)
	_, code := decodeReloadErrorBody(t, w.Body.String())
	assert.Equal(t, "geoip_busy", code)

	close(mgr.release)
	assert.Equal(t, http.StatusOK, (<-first).Code)
}
