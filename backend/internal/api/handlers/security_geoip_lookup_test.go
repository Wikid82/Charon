package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/geoip"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/Wikid82/charon/backend/internal/testutil"
)

// lookupRouter serves LookupGeoIP for a caller with role over a service loaded from a fixture database with opts.
func lookupRouter(t *testing.T, role string, opts ...testutil.GeoIPCountryOption) *gin.Engine {
	t.Helper()
	path := filepath.Join(t.TempDir(), geoip.DatabaseFileName)
	testutil.WriteGeoIPCountryDB(t, path, 1_750_000_000, opts...)
	svc, err := services.NewGeoIPService(path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = svc.Close() })

	h := NewSecurityHandler(config.SecurityConfig{}, nil, nil)
	h.SetGeoIPService(svc)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", role)
		c.Next()
	})
	r.POST("/security/geoip/lookup", h.LookupGeoIP)
	return r
}

// postLookup asks the router for the country of ip and decodes the JSON answer.
func postLookup(t *testing.T, r *gin.Engine, ip string) (status int, body map[string]any) {
	t.Helper()
	payload, err := json.Marshal(map[string]string{"ip_address": ip})
	require.NoError(t, err)
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/security/geoip/lookup", bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	r.ServeHTTP(w, req)

	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	return w.Code, body
}

func TestSecurityHandler_LookupGeoIP_Answers(t *testing.T) {
	t.Run("a resolved address names its country", func(t *testing.T) {
		code, body := postLookup(t, lookupRouter(t, "admin", testutil.WithGeoIPRecordISOCode("US")), "8.8.8.8")

		assert.Equal(t, http.StatusOK, code)
		assert.Equal(t, true, body["found"])
		assert.Equal(t, "US", body["country_code"])
		assert.Equal(t, "8.8.8.8", body["ip_address"])
	})
	t.Run("an address without a country is reported as not found", func(t *testing.T) {
		r := lookupRouter(t, "admin", testutil.WithGeoIPRecord(map[string]any{"continent": map[string]any{"code": "EU"}}))

		code, body := postLookup(t, r, "8.8.8.8")

		assert.Equal(t, http.StatusOK, code)
		assert.Equal(t, false, body["found"])
		assert.Equal(t, "", body["country_code"])
	})
	t.Run("an invalid address is refused", func(t *testing.T) {
		code, body := postLookup(t, lookupRouter(t, "admin"), "not-an-ip")

		assert.Equal(t, http.StatusBadRequest, code)
		assert.Equal(t, "Invalid IP address", body["error"])
	})
	t.Run("a record that cannot be decoded is a lookup failure", func(t *testing.T) {
		r := lookupRouter(t, "admin", testutil.WithGeoIPRecord(map[string]any{"country": "US"}))

		code, body := postLookup(t, r, "8.8.8.8")

		assert.Equal(t, http.StatusInternalServerError, code)
		assert.Contains(t, body["error"], "GeoIP lookup failed")
	})
}

func TestSecurityHandler_GeoIPEndpoints_RequireAdmin(t *testing.T) {
	t.Run("lookup", func(t *testing.T) {
		code, _ := postLookup(t, lookupRouter(t, "user", testutil.WithGeoIPRecordISOCode("US")), "8.8.8.8")

		assert.Equal(t, http.StatusForbidden, code)
	})
	t.Run("reload", func(t *testing.T) {
		h := NewSecurityHandler(config.SecurityConfig{}, nil, nil)
		r := gin.New()
		r.Use(func(c *gin.Context) {
			c.Set("role", "user")
			c.Next()
		})
		r.POST("/security/geoip/reload", h.ReloadGeoIP)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/security/geoip/reload", http.NoBody))

		assert.Equal(t, http.StatusForbidden, w.Code)
	})
}
