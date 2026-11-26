package handlers

import (
	"bytes"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
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

// applyRefusedFixture is an obviously fake credential carried by the advanced configuration of the host below.
const applyRefusedFixture = "FAKE-apply-0021"

// newRefusingAdmin answers /load with a refusal that quotes the header value when the posted config carries it.
func newRefusingAdmin(t *testing.T) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		if bytes.Contains(body, []byte(applyRefusedFixture)) {
			w.WriteHeader(http.StatusBadRequest)
			_, _ = w.Write([]byte(`{"error":"loading new config: refused header value 'Bearer ` + applyRefusedFixture + `'"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(server.Close)
	return server
}

// TestProxyHostCreate_ApplyRefusalDoesNotShowCredential posts a host whose advanced configuration carries a
// credential. The admin answer quotes it; the administrator response must show the refusal without the value.
func TestProxyHostCreate_ApplyRefusalDoesNotShowCredential(t *testing.T) {
	caddyServer := newRefusingAdmin(t)
	db, err := gorm.Open(sqlite.Open(testutil.MemoryDSN(t)), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.ProxyHost{}, &models.Location{}, &models.Setting{}, &models.CaddyConfig{}, &models.SSLCertificate{}, &models.DNSProvider{}, &models.RedirectionHost{}))

	port := caddyServer.Listener.Addr().(*net.TCPAddr).Port
	manager := caddy.NewManager(caddy.NewClientWithExpectedPort(caddyServer.URL, port), db, t.TempDir(), "", false, config.SecurityConfig{})
	h := NewProxyHostHandler(db, manager, services.NewNotificationService(db, nil), nil)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set(middleware.RoleKey, "admin")
		c.Next()
	})
	h.RegisterRoutes(r.Group("/api/v1"))

	payload := map[string]any{
		"domain_names":    "apply-error.example.com",
		"forward_scheme":  "http",
		"forward_host":    "10.0.0.5",
		"forward_port":    8080,
		"enabled":         true,
		"advanced_config": `{"handler":"headers","request":{"set":{"Authorization":["Bearer ` + applyRefusedFixture + `"]}}}`,
	}
	raw, err := json.Marshal(payload)
	require.NoError(t, err)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/api/v1/proxy-hosts", strings.NewReader(string(raw))))

	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	assert.Contains(t, w.Body.String(), "proxy_apply_failed")
	assert.Contains(t, w.Body.String(), "refused header value") // Caddy's own refusal text stays readable
	assert.NotContains(t, w.Body.String(), applyRefusedFixture)
	assert.Contains(t, w.Body.String(), "[redacted]")
}
