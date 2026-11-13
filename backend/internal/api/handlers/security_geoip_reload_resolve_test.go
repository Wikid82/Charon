package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/services"
)

func TestSecurityHandler_ReloadGeoIP_LoadsDatabaseAtResolvedPathAddedLater(t *testing.T) {
	dir := t.TempDir()
	dropped := filepath.Join(dir, "GeoLite2-Country.mmdb")
	current := filepath.Join(dir, "absent.mmdb")
	svc := services.NewResolvingGeoIPService(func() string { return current })
	mgr := &countingCaddyManager{}
	h := NewSecurityHandler(config.SecurityConfig{}, openGeoIPReasonTestDB(t), mgr)
	h.SetGeoIPService(svc)

	// The operator drops a database at a location the resolver now prefers; the reload re-resolves and loads it.
	require.NoError(t, os.WriteFile(dropped, minimalCountryMMDB(), 0o600))
	current = dropped
	t.Cleanup(func() { _ = svc.Close() })

	w := httptest.NewRecorder()
	reloadGeoIPRouter(h).ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/security/geoip/reload", bytes.NewReader(nil)))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.True(t, svc.IsLoaded())
	require.Equal(t, dropped, svc.GetDatabasePath())
	require.Equal(t, 1, mgr.calls)
}
