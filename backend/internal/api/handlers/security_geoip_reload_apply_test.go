package handlers

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/services"
)

// minimalCountryMMDB returns the smallest MaxMind DB the reader accepts: an empty search tree, the data separator and
// one metadata map that names a country database. It holds no addresses, so every lookup finds nothing.
func minimalCountryMMDB() []byte {
	str := func(s string) []byte { return append([]byte{0x40 | byte(len(s))}, s...) }
	uint16Value := func(v uint16) []byte {
		b := make([]byte, 2)
		binary.BigEndian.PutUint16(b, v)
		return append([]byte{0xA0 | 2}, b...)
	}
	var meta []byte
	meta = append(meta, 0xE0|6) // map with six entries
	meta = append(meta, str("node_count")...)
	meta = append(meta, 0xC0) // uint32 zero
	meta = append(meta, str("record_size")...)
	meta = append(meta, uint16Value(24)...)
	meta = append(meta, str("ip_version")...)
	meta = append(meta, uint16Value(4)...)
	meta = append(meta, str("database_type")...)
	meta = append(meta, str("GeoLite2-Country")...)
	meta = append(meta, str("binary_format_major_version")...)
	meta = append(meta, uint16Value(2)...)
	meta = append(meta, str("binary_format_minor_version")...)
	meta = append(meta, uint16Value(0)...)

	out := make([]byte, 16) // data section separator, no search tree
	out = append(out, []byte("\xAB\xCD\xEFMaxMind.com")...)
	return append(out, meta...)
}

// countingCaddyManager records applies and returns applyErr from each one.
type countingCaddyManager struct {
	calls    int
	applyErr error
}

func (m *countingCaddyManager) ApplyConfig(_ context.Context) error {
	m.calls++
	return m.applyErr
}

func reloadGeoIPRouter(h *SecurityHandler) *gin.Engine {
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Next()
	})
	r.POST("/security/geoip/reload", h.ReloadGeoIP)
	return r
}

func TestSecurityHandler_ReloadGeoIP_AppliesProxyConfigAfterReload(t *testing.T) {
	path := filepath.Join(t.TempDir(), "GeoLite2-Country.mmdb")
	require.NoError(t, os.WriteFile(path, minimalCountryMMDB(), 0o600))
	svc, err := services.NewGeoIPService(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })

	mgr := &countingCaddyManager{}
	h := NewSecurityHandler(config.SecurityConfig{}, openGeoIPReasonTestDB(t), mgr)
	h.SetGeoIPService(svc)

	w := httptest.NewRecorder()
	reloadGeoIPRouter(h).ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/security/geoip/reload", bytes.NewReader(nil)))

	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 1, mgr.calls, "a successful reload triggers one proxy apply")
}

func TestSecurityHandler_ReloadGeoIP_ApplyFailureIsReported(t *testing.T) {
	path := filepath.Join(t.TempDir(), "GeoLite2-Country.mmdb")
	require.NoError(t, os.WriteFile(path, minimalCountryMMDB(), 0o600))
	svc, err := services.NewGeoIPService(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })

	mgr := &countingCaddyManager{applyErr: errors.New("caddy refused the config")}
	h := NewSecurityHandler(config.SecurityConfig{}, openGeoIPReasonTestDB(t), mgr)
	h.SetGeoIPService(svc)

	w := httptest.NewRecorder()
	reloadGeoIPRouter(h).ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/security/geoip/reload", bytes.NewReader(nil)))

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Equal(t, 1, mgr.calls)
}

func TestSecurityHandler_ReloadGeoIP_LoadFailureSkipsApply(t *testing.T) {
	mgr := &countingCaddyManager{}
	h := NewSecurityHandler(config.SecurityConfig{}, openGeoIPReasonTestDB(t), mgr)
	h.SetGeoIPService(&services.GeoIPService{}) // no database path, so the load fails

	w := httptest.NewRecorder()
	reloadGeoIPRouter(h).ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/security/geoip/reload", bytes.NewReader(nil)))

	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Equal(t, 0, mgr.calls, "a failed reload does not apply")
}
