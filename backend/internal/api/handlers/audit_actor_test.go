package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/api/middleware"
	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
)

func TestAuditActor(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)

	newCtx := func() *gin.Context {
		c, _ := gin.CreateTestContext(httptest.NewRecorder())
		req := httptest.NewRequest(http.MethodGet, "/", http.NoBody)
		req.RemoteAddr = "198.51.100.10:1234"
		c.Request = req
		return c
	}

	t.Run("signed-in user", func(t *testing.T) {
		c := newCtx()
		middleware.SetCaller(c, 42, "admin")
		assert.Equal(t, "user:42", auditActor(c))
	})
	t.Run("emergency path", func(t *testing.T) {
		c := newCtx()
		middleware.SetCaller(c, 0, "admin")
		assert.Equal(t, "emergency", auditActor(c))
	})
	t.Run("no identity uses client address", func(t *testing.T) {
		assert.Equal(t, "198.51.100.10", auditActor(newCtx()))
	})
	t.Run("unexpected value type uses client address", func(t *testing.T) {
		c := newCtx()
		c.Set(middleware.UserIDKey, "42")
		assert.Equal(t, "198.51.100.10", auditActor(c))
	})
}

func TestSecurityHandler_AuditEntriesRecordCaller(t *testing.T) {
	t.Parallel()
	gin.SetMode(gin.TestMode)
	db := setupTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityDecision{}, &models.SecurityAudit{}))

	handler := NewSecurityHandler(config.SecurityConfig{}, db, nil)
	t.Cleanup(handler.Close)
	router := gin.New()
	router.Use(func(c *gin.Context) {
		middleware.SetCaller(c, 42, "admin")
		c.Next()
	})
	router.POST("/security/decisions", handler.CreateDecision)

	body, _ := json.Marshal(map[string]any{"ip": "10.0.0.1", "action": "block", "details": "manual"})
	req := httptest.NewRequest(http.MethodPost, "/security/decisions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	handler.svc.Flush()
	var audit models.SecurityAudit
	require.NoError(t, db.Where("action = ?", "create_decision").First(&audit).Error)
	assert.Equal(t, "user:42", audit.Actor)
}
