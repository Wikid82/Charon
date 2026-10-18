package routes

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/config"
)

// Every background install route sits in the admin group: without a session none of them reaches a handler.
func TestRegister_CrowdsecInstallJobRoutesRequireAuthentication(t *testing.T) {
	gin.SetMode(gin.TestMode)
	router := gin.New()

	db, err := gorm.Open(sqlite.Open(isolatedMemoryDSN(t)), &gorm.Config{})
	require.NoError(t, err)
	cfg := config.Config{
		JWTSecret:     "test-secret",
		EncryptionKey: "AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA=",
	}
	require.NoError(t, Register(context.Background(), router, db, cfg))

	const id = "00000000-0000-4000-8000-000000000001"
	for _, rt := range []struct{ method, path, body string }{
		{http.MethodPost, "/api/v1/admin/crowdsec/presets/apply", `{"slug":"a/b","cache_key":"k","async":true}`},
		{http.MethodGet, "/api/v1/admin/crowdsec/presets/jobs/current", ""},
		{http.MethodGet, "/api/v1/admin/crowdsec/presets/jobs", ""},
		{http.MethodGet, "/api/v1/admin/crowdsec/presets/jobs/" + id, ""},
		{http.MethodDelete, "/api/v1/admin/crowdsec/presets/jobs/" + id, ""},
	} {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(rt.method, rt.path, strings.NewReader(rt.body))
		req.Header.Set("Content-Type", "application/json")
		router.ServeHTTP(w, req)
		assert.Equal(t, http.StatusUnauthorized, w.Code, rt.method+" "+rt.path)
	}
}
