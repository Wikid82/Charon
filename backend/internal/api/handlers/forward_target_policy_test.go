package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

// listenerConfigApplier counts the config re-applies triggered by a handler.
type listenerConfigApplier struct {
	calls int
	err   error
}

func (r *listenerConfigApplier) ApplyConfig(context.Context) error {
	r.calls++
	return r.err
}

// listenerTestPort is the external proxy port the seeded agent holds.
const listenerTestPort = 2375

// seedListenerAgent stores an agent whose external proxy port is persisted.
func seedListenerAgent(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Create(&models.OrthrusAgent{UUID: "listener-agent", Name: "listener", AuthKeyHash: "x", ExternalProxyPort: listenerTestPort}).Error)
}

func TestPermissionProxyHostRefusesOrthrusListenerTargetForEveryRole(t *testing.T) {
	p := setupHostPermsRouter(t)
	require.NoError(t, p.db.AutoMigrate(&models.OrthrusAgent{}))
	seedListenerAgent(t, p.db)

	for _, role := range []string{"user", "admin"} {
		t.Run(role, func(t *testing.T) {
			rec := p.do(t, http.MethodPost, "/api/v1/proxy-hosts", role, map[string]any{
				"name":         "listener-" + role,
				"domain_names": "listener-" + role + ".example.com",
				"forward_host": "127.0.0.1",
				"forward_port": 2375,
			})
			assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
			assert.Contains(t, rec.Body.String(), services.ErrOrthrusListenerTarget.Error())
			assert.Contains(t, rec.Body.String(), `"error_code":"forward_target_refused"`)
		})
	}
}

func TestPermissionProxyHostUpdateRefusesOrthrusListenerTarget(t *testing.T) {
	p := setupHostPermsRouter(t)
	require.NoError(t, p.db.AutoMigrate(&models.OrthrusAgent{}))
	seedListenerAgent(t, p.db)

	stored := models.ProxyHost{UUID: "update-target", DomainNames: "update-target.example.com", ForwardHost: "10.0.0.5", ForwardPort: 8080}
	require.NoError(t, p.db.Create(&stored).Error)
	path := "/api/v1/proxy-hosts/" + stored.UUID

	for _, role := range []string{"user", "admin"} {
		rec := p.do(t, http.MethodPut, path, role, map[string]any{
			"name":         "update-target",
			"domain_names": "update-target.example.com",
			"forward_host": "localhost",
			"forward_port": 2375,
		})
		assert.Equal(t, http.StatusBadRequest, rec.Code, "role=%s body=%s", role, rec.Body.String())
		assert.Contains(t, rec.Body.String(), `"error_code":"forward_target_refused"`)
	}
}

func TestPermissionProxyHostAllowsOrdinaryTargetOnListenerRole(t *testing.T) {
	p := setupHostPermsRouter(t)
	require.NoError(t, p.db.AutoMigrate(&models.OrthrusAgent{}))
	seedListenerAgent(t, p.db)

	rec := p.do(t, http.MethodPost, "/api/v1/proxy-hosts", "user", map[string]any{
		"name":         "plain",
		"domain_names": "plain-listener.example.com",
		"forward_host": "10.0.0.5",
		"forward_port": 8080,
	})
	assert.Equal(t, http.StatusCreated, rec.Code, rec.Body.String())
}

func TestPermissionProxyHostTestConnectionRefusesOrthrusListenerTarget(t *testing.T) {
	p := setupHostPermsRouter(t)
	require.NoError(t, p.db.AutoMigrate(&models.OrthrusAgent{}))
	seedListenerAgent(t, p.db)

	rec := p.do(t, http.MethodPost, "/api/v1/proxy-hosts/test", "user", map[string]any{
		"forward_host": "127.0.0.1",
		"forward_port": 2375,
	})
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"error_code":"forward_target_refused"`)
}

func TestRespondHostWriteError_MapsListenerTargetToBadRequest(t *testing.T) {
	gin.SetMode(gin.TestMode)
	rec := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(rec)

	respondHostWriteError(c, services.ErrOrthrusListenerTarget)
	assert.Equal(t, http.StatusBadRequest, rec.Code)
	assert.Contains(t, rec.Body.String(), `"error_code":"forward_target_refused"`)
}

func TestOrthrusPatch_ExternalProxyChangeReappliesConfig(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openOrthrusTestDB(t)
	require.NoError(t, db.Create(&models.OrthrusAgent{UUID: "patch-agent", Name: "patch", AuthKeyHash: "x"}).Error)

	applier := &listenerConfigApplier{}
	h := NewOrthrusHandler(services.NewOrthrusService(db, nil), nil)
	h.SetConfigApplier(applier)

	r := gin.New()
	h.RegisterRoutes(r.Group("/api/v1"), r.Group("/api/v1"))

	patch := func(body string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(http.MethodPatch, "/api/v1/orthrus/agents/patch-agent", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		r.ServeHTTP(rec, req)
		return rec
	}

	require.Equal(t, http.StatusOK, patch(`{"external_proxy_port":2400}`).Code)
	assert.Equal(t, 1, applier.calls)
	require.Equal(t, http.StatusOK, patch(`{"external_proxy_bind":"10.0.0.5"}`).Code)
	assert.Equal(t, 2, applier.calls)
	require.Equal(t, http.StatusOK, patch(`{"name":"renamed"}`).Code)
	assert.Equal(t, 2, applier.calls, "unrelated fields do not re-apply")
}

func TestOrthrusPatch_ReapplyFailureDoesNotFailPatch(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := openOrthrusTestDB(t)
	require.NoError(t, db.Create(&models.OrthrusAgent{UUID: "fail-agent", Name: "fail", AuthKeyHash: "x"}).Error)

	h := NewOrthrusHandler(services.NewOrthrusService(db, nil), nil)
	h.SetConfigApplier(&listenerConfigApplier{err: errors.New("caddy down")})
	r := gin.New()
	h.RegisterRoutes(r.Group("/api/v1"), r.Group("/api/v1"))

	req := httptest.NewRequest(http.MethodPatch, "/api/v1/orthrus/agents/fail-agent", strings.NewReader(`{"external_proxy_port":2401}`))
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestPermissionProxyHostRefusesAgentNameFormForEveryRole(t *testing.T) {
	p := setupHostPermsRouter(t)
	require.NoError(t, p.db.AutoMigrate(&models.OrthrusAgent{}))

	for _, role := range []string{"user", "admin"} {
		t.Run(role, func(t *testing.T) {
			rec := p.do(t, http.MethodPost, "/api/v1/proxy-hosts", role, map[string]any{
				"name":         "agent-form-" + role,
				"domain_names": "agent-form-" + role + ".example.com",
				"forward_host": "orthrus:some-agent",
				"forward_port": 8080,
			})
			assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
			assert.Contains(t, rec.Body.String(), services.ErrAgentForwardTarget.Error())
			assert.Contains(t, rec.Body.String(), `"error_code":"forward_target_refused"`)
		})
	}
}

func TestPermissionProxyHostRefusesAgentNameFormInLocation(t *testing.T) {
	p := setupHostPermsRouter(t)
	require.NoError(t, p.db.AutoMigrate(&models.OrthrusAgent{}))

	rec := p.do(t, http.MethodPost, "/api/v1/proxy-hosts", "user", map[string]any{
		"name":         "agent-loc",
		"domain_names": "agent-loc.example.com",
		"forward_host": "10.0.0.5",
		"forward_port": 8080,
		"locations": []map[string]any{
			{"path": "/x", "forward_host": "orthrus:some-agent", "forward_port": 8080},
		},
	})
	assert.Equal(t, http.StatusBadRequest, rec.Code, rec.Body.String())
	assert.Contains(t, rec.Body.String(), `"error_code":"forward_target_refused"`)
}
