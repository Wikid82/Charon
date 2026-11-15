package handlers

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/api/middleware"
	"github.com/Wikid82/charon/backend/internal/caddy"
	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/Wikid82/charon/backend/internal/testutil"
)

// leftOutGuard refuses the one forward host it names, so an apply leaves that host out.
type leftOutGuard struct{ refused string }

func (g leftOutGuard) RefusesForwardTarget(host string, _ int) (bool, error) {
	return host == g.refused, nil
}

// setupLeftOutRouter applies a configuration in which one host is left out, and
// returns a router that serves the proxy host endpoints from that manager.
func setupLeftOutRouter(t *testing.T) *gin.Engine {
	t.Helper()
	caddyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(caddyServer.Close)

	db, err := gorm.Open(sqlite.Open(testutil.MemoryDSN(t)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.ProxyHost{}, &models.Location{}, &models.Setting{}, &models.CaddyConfig{}, &models.SSLCertificate{}, &models.DNSProvider{}, &models.RedirectionHost{}))
	for _, h := range []models.ProxyHost{
		{UUID: "left-out", DomainNames: "left.example.com", ForwardHost: "169.254.1.5", ForwardPort: 80, Enabled: true},
		{UUID: "served", DomainNames: "served.example.com", ForwardHost: "10.0.0.5", ForwardPort: 8080, Enabled: true},
	} {
		require.NoError(t, db.Create(&h).Error)
	}

	port := caddyServer.Listener.Addr().(*net.TCPAddr).Port
	manager := caddy.NewManager(caddy.NewClientWithExpectedPort(caddyServer.URL, port), db, t.TempDir(), "", false, config.SecurityConfig{})
	manager.SetForwardTargetGuard(leftOutGuard{refused: "169.254.1.5"})
	require.NoError(t, manager.ApplyConfig(t.Context()))

	r := gin.New()
	r.Use(func(c *gin.Context) {
		if role := c.GetHeader("X-Test-Role"); role != "" {
			c.Set(middleware.RoleKey, role)
		}
		c.Next()
	})
	api := r.Group("/api/v1")
	NewProxyHostHandler(db, manager, services.NewNotificationService(db, nil), nil).RegisterRoutes(api)
	return r
}

// roleGET runs a GET as role and decodes the body into a list or an object.
func roleGET(t *testing.T, r *gin.Engine, path, role string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, http.NoBody)
	req.Header.Set("X-Test-Role", role)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	return rec
}

func activeConfigByUUID(t *testing.T, body []byte) map[string]any {
	t.Helper()
	var hosts []map[string]any
	require.NoError(t, json.Unmarshal(body, &hosts))
	out := make(map[string]any, len(hosts))
	for _, h := range hosts {
		uuid, _ := h["uuid"].(string)
		out[uuid] = h["active_config"]
	}
	return out
}

func TestProxyHostList_AdminSeesActiveConfigFlag(t *testing.T) {
	r := setupLeftOutRouter(t)
	rec := roleGET(t, r, "/api/v1/proxy-hosts", "admin")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	flags := activeConfigByUUID(t, rec.Body.Bytes())
	assert.Equal(t, false, flags["left-out"], "admin list marks the left-out host")
	assert.Nil(t, flags["served"], "an active host omits the key")
}

func TestProxyHostList_OtherRolesSeeActiveConfigFlagToo(t *testing.T) {
	r := setupLeftOutRouter(t)
	rec := roleGET(t, r, "/api/v1/proxy-hosts", "user")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	flags := activeConfigByUUID(t, rec.Body.Bytes())
	assert.Equal(t, false, flags["left-out"])
	assert.Nil(t, flags["served"])
}

func TestProxyHostGet_AdminSeesActiveConfigFlag(t *testing.T) {
	r := setupLeftOutRouter(t)

	rec := roleGET(t, r, "/api/v1/proxy-hosts/left-out", "admin")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var left map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &left))
	assert.Equal(t, false, left["active_config"])

	rec = roleGET(t, r, "/api/v1/proxy-hosts/served", "admin")
	require.Equal(t, http.StatusOK, rec.Code, rec.Body.String())
	var served map[string]any
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &served))
	assert.NotContains(t, served, "active_config")
}
