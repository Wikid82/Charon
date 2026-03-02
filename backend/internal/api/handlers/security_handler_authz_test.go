package handlers

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
)

func TestSecurityHandler_MutatorsRequireAdmin(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}, &models.SecurityRuleSet{}, &models.SecurityDecision{}, &models.SecurityAudit{}))

	handler := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		c.Set("userID", uint(123))
		c.Set("role", "user")
		c.Next()
	})

	router.POST("/security/config", handler.UpdateConfig)
	router.POST("/security/breakglass/generate", handler.GenerateBreakGlass)
	router.POST("/security/decisions", handler.CreateDecision)
	router.POST("/security/rulesets", handler.UpsertRuleSet)
	router.DELETE("/security/rulesets/:id", handler.DeleteRuleSet)

	testCases := []struct {
		name   string
		method string
		url    string
		body   string
	}{
		{name: "update-config", method: http.MethodPost, url: "/security/config", body: `{"name":"default"}`},
		{name: "generate-breakglass", method: http.MethodPost, url: "/security/breakglass/generate", body: `{}`},
		{name: "create-decision", method: http.MethodPost, url: "/security/decisions", body: `{"ip":"1.2.3.4","action":"block"}`},
		{name: "upsert-ruleset", method: http.MethodPost, url: "/security/rulesets", body: `{"name":"owasp-crs","mode":"block","content":"x"}`},
		{name: "delete-ruleset", method: http.MethodDelete, url: "/security/rulesets/1", body: ""},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.url, bytes.NewBufferString(tc.body))
			req.Header.Set("Content-Type", "application/json")
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
			assert.Equal(t, http.StatusForbidden, w.Code)
		})
	}
}
