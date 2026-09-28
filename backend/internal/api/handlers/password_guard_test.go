package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/ratelimit"
	"github.com/Wikid82/charon/backend/internal/services"
)

// fakePasswordGuard records calls and denies with the standard 429 when allow is false.
type fakePasswordGuard struct {
	allow bool
	calls int
}

func (g *fakePasswordGuard) AllowPasswordAttempt(c *gin.Context) bool {
	g.calls++
	if !g.allow {
		ratelimit.Reject(c, ratelimit.Decision{RetryAfter: time.Minute})
	}
	return g.allow
}

func profileRouter(t *testing.T, guard PasswordAttemptGuard) (*gin.Engine, *gorm.DB, *models.User) {
	t.Helper()
	handler, db := setupUserHandler(t)
	if guard != nil {
		handler.SetPasswordAttemptGuard(guard)
	}
	user := &models.User{UUID: uuid.NewString(), Email: "guard@example.com", Name: "Guard", APIKey: uuid.NewString()}
	require.NoError(t, user.SetPassword("password123"))
	require.NoError(t, db.Create(user).Error)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("userID", user.ID)
		c.Next()
	})
	r.POST("/profile", handler.UpdateProfile)
	return r, db, user
}

func postJSON(r *gin.Engine, path string, body any) *httptest.ResponseRecorder {
	payload, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(payload))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestUserHandler_UpdateProfile_PasswordGuardDeniesEmailChange(t *testing.T) {
	guard := &fakePasswordGuard{allow: false}
	r, db, user := profileRouter(t, guard)

	w := postJSON(r, "/profile", map[string]string{"name": "Guard", "email": "new@example.com", "current_password": "password123"})
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Equal(t, "60", w.Header().Get("Retry-After"))
	assert.Equal(t, 1, guard.calls)

	var stored models.User
	require.NoError(t, db.First(&stored, user.ID).Error)
	assert.Equal(t, "guard@example.com", stored.Email)
}

func TestUserHandler_UpdateProfile_PasswordGuardOnlyOnPasswordBranch(t *testing.T) {
	guard := &fakePasswordGuard{allow: false}
	r, _, _ := profileRouter(t, guard)

	w := postJSON(r, "/profile", map[string]string{"name": "Renamed", "email": "guard@example.com"})
	assert.Equal(t, http.StatusOK, w.Code, "a name-only update never verifies a password")

	w = postJSON(r, "/profile", map[string]string{"name": "Renamed", "email": "new@example.com"})
	assert.Equal(t, http.StatusBadRequest, w.Code, "a missing password is rejected before the guard")
	assert.Zero(t, guard.calls)
}

func TestUserHandler_UpdateProfile_PasswordGuardAllows(t *testing.T) {
	guard := &fakePasswordGuard{allow: true}
	r, _, _ := profileRouter(t, guard)

	w := postJSON(r, "/profile", map[string]string{"name": "Guard", "email": "new@example.com", "current_password": "wrong"})
	assert.Equal(t, http.StatusUnauthorized, w.Code)
	w = postJSON(r, "/profile", map[string]string{"name": "Guard", "email": "new@example.com", "current_password": "password123"})
	assert.Equal(t, http.StatusOK, w.Code)
	assert.Equal(t, 2, guard.calls)
}

func exportRouter(t *testing.T, guard PasswordAttemptGuard) *gin.Engine {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SSLCertificate{}, &models.ProxyHost{}, &models.User{}))
	h := NewCertificateHandler(services.NewCertificateService(t.TempDir(), db, nil), nil, nil)
	h.SetDB(db)
	if guard != nil {
		h.SetPasswordAttemptGuard(guard)
	}
	r := gin.New()
	r.POST("/api/certificates/:uuid/export", h.Export)
	return r
}

func TestCertificateHandler_Export_PasswordGuardDeniesKeyExport(t *testing.T) {
	guard := &fakePasswordGuard{allow: false}
	r := exportRouter(t, guard)

	w := postJSON(r, "/api/certificates/abc/export", map[string]any{"format": "pem", "include_key": true, "password": "x"})
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Equal(t, 1, guard.calls)

	w = postJSON(r, "/api/certificates/abc/export", map[string]any{"format": "pem", "include_key": false})
	assert.NotEqual(t, http.StatusTooManyRequests, w.Code, "certificate-only export is not charged")
	assert.Equal(t, 1, guard.calls)
}

func TestCertificateHandler_Export_PasswordGuardAllowsThenReauthenticates(t *testing.T) {
	guard := &fakePasswordGuard{allow: true}
	r := exportRouter(t, guard)

	w := postJSON(r, "/api/certificates/abc/export", map[string]any{"format": "pem", "include_key": true, "password": "x"})
	assert.Equal(t, http.StatusForbidden, w.Code, "the existing re-authentication checks still run")
	assert.Equal(t, 1, guard.calls)
}

func TestAllowPasswordAttempt_NilGuardAllows(t *testing.T) {
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	assert.True(t, allowPasswordAttempt(nil, c))
}
