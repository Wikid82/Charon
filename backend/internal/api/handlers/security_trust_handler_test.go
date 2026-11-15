package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/api/middleware"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/security"
)

// trustApplier records proxy applies and fails them on demand.
type trustApplier struct {
	calls int
	err   error
}

func (a *trustApplier) ApplyConfig(context.Context) error {
	a.calls++
	return a.err
}

func trustTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.TempDir()+"/trust.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.Setting{}))
	return db
}

// trustRouter mounts the trusted-proxy routes with the given caller role, as the real routes do: read for every
// role, writes for administrators only.
func trustRouter(t *testing.T, h *TrustedProxyHandler, role string) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		middleware.SetCaller(c, 1, role)
		c.Next()
	})
	r.GET("/security/trusted-proxies", h.Get)
	admin := r.Group("/")
	admin.Use(func(c *gin.Context) {
		if role != string(models.RoleAdmin) {
			c.AbortWithStatus(http.StatusForbidden)
			return
		}
		c.Next()
	})
	admin.PUT("/security/trusted-proxies", h.Put)
	admin.DELETE("/security/trusted-proxies", h.Delete)
	return r
}

func doTrust(t *testing.T, r http.Handler, method, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, "/security/trusted-proxies", bytes.NewBufferString(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func decodeTrustBody(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var out map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out), "body: %s", w.Body.String())
	return out
}

func storedTrust(t *testing.T, db *gorm.DB) *string {
	t.Helper()
	stored, err := models.ReadTrustedProxiesSetting(db)
	require.NoError(t, err)
	return stored
}

func TestTrustedProxyGet_DefaultSource(t *testing.T) {
	db := trustTestDB(t)
	h := NewTrustedProxyHandler(db, nil, nil, "")
	w := doTrust(t, trustRouter(t, h, string(models.RoleAdmin)), http.MethodGet, "")

	require.Equal(t, http.StatusOK, w.Code)
	body := decodeTrustBody(t, w)
	assert.Equal(t, "default", body["source"])
	assert.Equal(t, []any{}, body["proxies"])
	assert.Equal(t, []any{"127.0.0.1/32", "::1/128"}, body["always_trusted"])
	assert.Equal(t, []any{}, body["api_layer"])
}

func TestTrustedProxyGet_EnvironmentSourceShowsDroppedEntries(t *testing.T) {
	db := trustTestDB(t)
	h := NewTrustedProxyHandler(db, nil, []string{"172.18.0.5", "0.0.0.0/0"}, "")
	w := doTrust(t, trustRouter(t, h, string(models.RoleAdmin)), http.MethodGet, "")

	require.Equal(t, http.StatusOK, w.Code)
	body := decodeTrustBody(t, w)
	assert.Equal(t, "environment", body["source"])
	assert.Equal(t, []any{"172.18.0.5/32"}, body["proxies"])
	assert.Equal(t, []any{"0.0.0.0/0"}, body["dropped_entries"])
	assert.Equal(t, []any{"172.18.0.5", "0.0.0.0/0"}, body["api_layer"])
}

func TestTrustedProxyGet_NonAdminIsRedacted(t *testing.T) {
	db := trustTestDB(t)
	value := "172.18.0.5/32"
	require.NoError(t, models.WriteTrustedProxiesSetting(db, &value))
	h := NewTrustedProxyHandler(db, nil, []string{"10.9.9.9"}, "")

	w := doTrust(t, trustRouter(t, h, "user"), http.MethodGet, "")
	require.Equal(t, http.StatusOK, w.Code)
	body := decodeTrustBody(t, w)
	assert.Equal(t, true, body["has_proxies"])
	assert.Equal(t, "setting", body["source"])
	for _, hidden := range []string{"proxies", "dropped_entries", "api_layer", "warnings"} {
		assert.NotContains(t, body, hidden)
	}
}

func TestTrustedProxyGet_StoredInvalidReportsWarning(t *testing.T) {
	db := trustTestDB(t)
	value := "0.0.0.0/0"
	require.NoError(t, models.WriteTrustedProxiesSetting(db, &value))
	h := NewTrustedProxyHandler(db, nil, nil, "")

	w := doTrust(t, trustRouter(t, h, string(models.RoleAdmin)), http.MethodGet, "")
	require.Equal(t, http.StatusOK, w.Code)
	body := decodeTrustBody(t, w)
	assert.Equal(t, []any{}, body["proxies"], "an invalid saved list falls back to loopback only")
	require.NotEmpty(t, body["warnings"])
}

func TestTrustedProxyGet_StorageErrorIs500(t *testing.T) {
	db := trustTestDB(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	w := doTrust(t, trustRouter(t, NewTrustedProxyHandler(db, nil, nil, ""), string(models.RoleAdmin)), http.MethodGet, "")
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestTrustedProxyPut_SavesAndApplies(t *testing.T) {
	db := trustTestDB(t)
	applier := &trustApplier{}
	h := NewTrustedProxyHandler(db, applier, nil, "")

	w := doTrust(t, trustRouter(t, h, string(models.RoleAdmin)), http.MethodPut, `{"proxies":["172.18.0.5","fd00::5","172.18.0.5"]}`)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	body := decodeTrustBody(t, w)
	assert.Equal(t, []any{"172.18.0.5/32", "fd00::5/128"}, body["proxies"])
	assert.Equal(t, "setting", body["source"])
	assert.Equal(t, 1, applier.calls)
	stored := storedTrust(t, db)
	require.NotNil(t, stored)
	assert.Equal(t, "172.18.0.5/32,fd00::5/128", *stored)
}

func TestTrustedProxyPut_EmptyListTrustsLoopbackOnly(t *testing.T) {
	db := trustTestDB(t)
	h := NewTrustedProxyHandler(db, nil, []string{"10.9.9.9"}, "")

	w := doTrust(t, trustRouter(t, h, string(models.RoleAdmin)), http.MethodPut, `{"proxies":[]}`)

	require.Equal(t, http.StatusOK, w.Code)
	body := decodeTrustBody(t, w)
	assert.Equal(t, "setting", body["source"], "a saved empty list overrides the environment list")
	assert.Equal(t, []any{}, body["proxies"])
	stored := storedTrust(t, db)
	require.NotNil(t, stored)
	assert.Empty(t, *stored)
}

func TestTrustedProxyPut_WarningsAreReturned(t *testing.T) {
	db := trustTestDB(t)
	h := NewTrustedProxyHandler(db, nil, nil, "")

	w := doTrust(t, trustRouter(t, h, string(models.RoleAdmin)), http.MethodPut, `{"proxies":["10.0.0.0/8"]}`)

	require.Equal(t, http.StatusOK, w.Code)
	assert.NotEmpty(t, decodeTrustBody(t, w)["warnings"])
}

func TestTrustedProxyPut_RefusedEntries(t *testing.T) {
	tests := []struct {
		name      string
		body      string
		wantCode  string
		wantEntry string
	}{
		{"unparsable entry", `{"proxies":["not-an-ip"]}`, codeTrustedProxyInvalid, "not-an-ip"},
		{"too broad entry", `{"proxies":["0.0.0.0/0"]}`, codeTrustedProxyTooBroad, "0.0.0.0/0"},
		{"IPv6 range wider than /24", `{"proxies":["2001:db8::/23"]}`, codeTrustedProxyTooBroad, "2001:db8::/23"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := trustTestDB(t)
			applier := &trustApplier{}
			w := doTrust(t, trustRouter(t, NewTrustedProxyHandler(db, applier, nil, ""), string(models.RoleAdmin)), http.MethodPut, tt.body)

			require.Equal(t, http.StatusBadRequest, w.Code)
			body := decodeTrustBody(t, w)
			assert.Equal(t, tt.wantCode, body["error_code"])
			assert.Equal(t, tt.wantEntry, body["entry"])
			assert.Equal(t, 0, applier.calls, "a refused list is never applied")
			assert.Nil(t, storedTrust(t, db))
		})
	}
}

func TestTrustedProxyPut_TooManyEntries(t *testing.T) {
	db := trustTestDB(t)
	var entries []string
	for i := range security.MaxTrustedProxies + 1 {
		entries = append(entries, "172.18."+strconv.Itoa(i/200)+"."+strconv.Itoa(i%200))
	}
	raw, err := json.Marshal(trustedProxiesRequest{Proxies: entries})
	require.NoError(t, err)

	w := doTrust(t, trustRouter(t, NewTrustedProxyHandler(db, nil, nil, ""), string(models.RoleAdmin)), http.MethodPut, string(raw))
	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Equal(t, codeTrustedProxiesTooMany, decodeTrustBody(t, w)["error_code"])
}

func TestTrustedProxyPut_MalformedBodyIs400(t *testing.T) {
	db := trustTestDB(t)
	w := doTrust(t, trustRouter(t, NewTrustedProxyHandler(db, nil, nil, ""), string(models.RoleAdmin)), http.MethodPut, `{"proxies":`)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestTrustedProxyPut_ApplyFailureRestoresPreviousValue(t *testing.T) {
	db := trustTestDB(t)
	previous := "10.1.1.1/32"
	require.NoError(t, models.WriteTrustedProxiesSetting(db, &previous))
	applier := &trustApplier{err: errors.New("caddy refused")}
	h := NewTrustedProxyHandler(db, applier, nil, "")

	w := doTrust(t, trustRouter(t, h, string(models.RoleAdmin)), http.MethodPut, `{"proxies":["172.18.0.5"]}`)

	require.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Equal(t, msgModuleApplyFailed, decodeTrustBody(t, w)["error"])
	stored := storedTrust(t, db)
	require.NotNil(t, stored)
	assert.Equal(t, previous, *stored)
}

func TestTrustedProxyPut_ApplyFailureRemovesNewlyCreatedValue(t *testing.T) {
	db := trustTestDB(t)
	applier := &trustApplier{err: errors.New("caddy refused")}
	h := NewTrustedProxyHandler(db, applier, nil, "")

	w := doTrust(t, trustRouter(t, h, string(models.RoleAdmin)), http.MethodPut, `{"proxies":["172.18.0.5"]}`)

	require.Equal(t, http.StatusInternalServerError, w.Code)
	assert.Nil(t, storedTrust(t, db), "with nothing saved before, the failed value is removed again")
}

func TestTrustedProxyPut_StorageFailureIs500(t *testing.T) {
	db := trustTestDB(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	w := doTrust(t, trustRouter(t, NewTrustedProxyHandler(db, nil, nil, ""), string(models.RoleAdmin)), http.MethodPut, `{"proxies":["172.18.0.5"]}`)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestTrustedProxyDelete_RemovesSavedValue(t *testing.T) {
	db := trustTestDB(t)
	saved := "172.18.0.5/32"
	require.NoError(t, models.WriteTrustedProxiesSetting(db, &saved))
	applier := &trustApplier{}
	h := NewTrustedProxyHandler(db, applier, []string{"10.9.9.9"}, "")

	w := doTrust(t, trustRouter(t, h, string(models.RoleAdmin)), http.MethodDelete, "")

	require.Equal(t, http.StatusOK, w.Code)
	body := decodeTrustBody(t, w)
	assert.Equal(t, "environment", body["source"], "the environment list applies again once the saved list is removed")
	assert.Equal(t, []any{"10.9.9.9/32"}, body["proxies"])
	assert.Equal(t, 1, applier.calls)
	assert.Nil(t, storedTrust(t, db))
}

func TestTrustedProxyDelete_ApplyFailureRestoresValue(t *testing.T) {
	db := trustTestDB(t)
	saved := "172.18.0.5/32"
	require.NoError(t, models.WriteTrustedProxiesSetting(db, &saved))
	h := NewTrustedProxyHandler(db, &trustApplier{err: errors.New("caddy refused")}, nil, "")

	w := doTrust(t, trustRouter(t, h, string(models.RoleAdmin)), http.MethodDelete, "")

	require.Equal(t, http.StatusInternalServerError, w.Code)
	stored := storedTrust(t, db)
	require.NotNil(t, stored)
	assert.Equal(t, saved, *stored)
}

func TestTrustedProxyWrites_ForbiddenForNonAdmin(t *testing.T) {
	db := trustTestDB(t)
	applier := &trustApplier{}
	r := trustRouter(t, NewTrustedProxyHandler(db, applier, nil, ""), "user")

	assert.Equal(t, http.StatusForbidden, doTrust(t, r, http.MethodPut, `{"proxies":["172.18.0.5"]}`).Code)
	assert.Equal(t, http.StatusForbidden, doTrust(t, r, http.MethodDelete, "").Code)
	assert.Equal(t, 0, applier.calls)
	assert.Nil(t, storedTrust(t, db))
}

func TestTrustedProxyPut_NoCaddyManagerStoresWithoutApply(t *testing.T) {
	db := trustTestDB(t)
	h := NewTrustedProxyHandler(db, nil, nil, "")

	w := doTrust(t, trustRouter(t, h, string(models.RoleAdmin)), http.MethodPut, `{"proxies":["172.18.0.5"]}`)
	require.Equal(t, http.StatusOK, w.Code)
	assert.NotNil(t, storedTrust(t, db))
}

func TestRespondTrustedProxyError_UnexpectedIs500(t *testing.T) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	respondTrustedProxyError(c, errors.New("boom"))
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestNonNilStrings(t *testing.T) {
	assert.Equal(t, []string{}, nonNilStrings(nil))
	assert.Equal(t, []string{"a"}, nonNilStrings([]string{"a"}))
}
