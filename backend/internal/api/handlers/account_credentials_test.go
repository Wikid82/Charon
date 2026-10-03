package handlers

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/Wikid82/charon/backend/internal/api/middleware"
	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

type denyGuard struct{ calls int }

func (g *denyGuard) AllowPasswordAttempt(c *gin.Context) bool {
	g.calls++
	c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": "too many attempts"})
	return false
}

type credentialsEnv struct {
	r    *gin.Engine
	db   *gorm.DB
	auth *services.AuthService
}

// newCredentialsEnv wires the real authentication middleware, service and handlers.
func newCredentialsEnv(t *testing.T) *credentialsEnv {
	t.Helper()
	gin.SetMode(gin.TestMode)
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.User{}, &models.Setting{}, &models.SecurityAudit{}))
	auth := services.NewAuthService(db, config.Config{JWTSecret: "test-secret"})
	authHandler := NewAuthHandler(auth, nil)
	userHandler := NewUserHandler(db, auth)

	r := gin.New()
	r.POST("/auth/login", authHandler.Login)
	protected := r.Group("/", middleware.AuthMiddleware(auth))
	protected.POST("/auth/change-password", authHandler.ChangePassword)
	protected.PUT("/users/:id", userHandler.UpdateUser)
	return &credentialsEnv{r: r, db: db, auth: auth}
}

func (e *credentialsEnv) do(t *testing.T, method, path, token string, body any) *httptest.ResponseRecorder {
	t.Helper()
	raw, err := json.Marshal(body)
	require.NoError(t, err)
	req := httptest.NewRequest(method, path, bytes.NewReader(raw))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	e.r.ServeHTTP(w, req)
	return w
}

func (e *credentialsEnv) user(t *testing.T, email string, admin bool) (*models.User, string) {
	t.Helper()
	u, err := e.auth.Register(email, "password123", "User")
	require.NoError(t, err)
	if admin {
		require.NoError(t, e.db.Model(u).Update("role", models.RoleAdmin).Error)
	} else {
		require.NoError(t, e.db.Model(u).Update("role", models.RoleUser).Error)
	}
	token, err := e.auth.Login(email, "password123")
	require.NoError(t, err)
	return u, token
}

func (e *credentialsEnv) status(t *testing.T, token string) int {
	t.Helper()
	return e.do(t, http.MethodPut, "/users/0", token, map[string]any{}).Code
}

func TestLoginHandler_UniformFailureResponse(t *testing.T) {
	e := newCredentialsEnv(t)
	e.user(t, "known@example.com", true)

	bodies := make([]string, 0, 2)
	for _, email := range []string{"known@example.com", "unknown@example.com"} {
		w := e.do(t, http.MethodPost, "/auth/login", "", map[string]string{"email": email, "password": "wrong-password"})
		assert.Equal(t, http.StatusUnauthorized, w.Code)
		bodies = append(bodies, w.Body.String())
	}
	assert.Equal(t, bodies[0], bodies[1])
	assert.JSONEq(t, `{"error":"invalid credentials"}`, bodies[0])
}

func TestLoginHandler_InternalErrorReportedAsUnavailable(t *testing.T) {
	e := newCredentialsEnv(t)
	sqlDB, err := e.db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	w := e.do(t, http.MethodPost, "/auth/login", "", map[string]string{"email": "a@example.com", "password": "password123"})
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.JSONEq(t, `{"error":"login unavailable"}`, w.Body.String())
}

func TestChangePasswordHandler_KeepsCallerSignedInAndEndsOtherSessions(t *testing.T) {
	e := newCredentialsEnv(t)
	_, oldToken := e.user(t, "self@example.com", true)
	require.NotEqual(t, http.StatusUnauthorized, e.status(t, oldToken))

	w := e.do(t, http.MethodPost, "/auth/change-password", oldToken, map[string]string{
		"old_password": "password123", "new_password": "another-password",
	})
	require.Equal(t, http.StatusOK, w.Code)

	var fresh string
	for _, ck := range w.Result().Cookies() {
		if ck.Name == "auth_token" {
			fresh = ck.Value
		}
	}
	require.NotEmpty(t, fresh, "a fresh session cookie must be issued")
	assert.NotEqual(t, http.StatusUnauthorized, e.status(t, fresh))
	assert.Equal(t, http.StatusUnauthorized, e.status(t, oldToken))
}

func TestUpdateUser_OwnPasswordRequiresCurrentPassword(t *testing.T) {
	cases := map[string]struct {
		current string
		code    int
		changed bool
	}{
		"missing":   {"", http.StatusBadRequest, false},
		"incorrect": {"not-the-password", http.StatusUnauthorized, false},
		"correct":   {"password123", http.StatusOK, true},
	}
	for name, tc := range cases {
		for _, admin := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s_admin_%t", name, admin), func(t *testing.T) {
				e := newCredentialsEnv(t)
				u, token := e.user(t, "self@example.com", admin)

				body := map[string]any{"password": "brand-new-password"}
				if tc.current != "" {
					body["current_password"] = tc.current
				}
				w := e.do(t, http.MethodPut, "/users/"+strconv.FormatUint(uint64(u.ID), 10), token, body)
				assert.Equal(t, tc.code, w.Code, w.Body.String())

				var after models.User
				require.NoError(t, e.db.First(&after, u.ID).Error)
				assert.Equal(t, tc.changed, after.CheckPassword("brand-new-password"))
				if tc.changed {
					assert.Equal(t, u.SessionVersion+1, after.SessionVersion)
					assert.Equal(t, http.StatusUnauthorized, e.status(t, token), "prior sessions end")
				} else {
					assert.Equal(t, u.SessionVersion, after.SessionVersion)
				}
			})
		}
	}
}

func TestUpdateUser_OwnPasswordChangeIsThrottled(t *testing.T) {
	e := newCredentialsEnv(t)
	guard := &denyGuard{}
	// Rebuild the route with a guard attached.
	userHandler := NewUserHandler(e.db, e.auth)
	userHandler.SetPasswordAttemptGuard(guard)
	r := gin.New()
	r.PUT("/users/:id", middleware.AuthMiddleware(e.auth), userHandler.UpdateUser)
	e.r = r

	u, token := e.user(t, "self@example.com", false)
	w := e.do(t, http.MethodPut, "/users/"+strconv.FormatUint(uint64(u.ID), 10), token, map[string]any{
		"password": "brand-new-password", "current_password": "password123",
	})
	assert.Equal(t, http.StatusTooManyRequests, w.Code)
	assert.Equal(t, 1, guard.calls)
}

func TestUpdateUser_AdminResetOfAnotherUserEndsTheirSessions(t *testing.T) {
	e := newCredentialsEnv(t)
	_, adminToken := e.user(t, "admin@example.com", true)
	target, targetToken := e.user(t, "target@example.com", false)
	require.NoError(t, e.db.Model(target).Updates(map[string]any{"failed_login_attempts": 3}).Error)

	w := e.do(t, http.MethodPut, "/users/"+strconv.FormatUint(uint64(target.ID), 10), adminToken, map[string]any{"password": "reset-password-1"})
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	assert.Equal(t, http.StatusUnauthorized, e.status(t, targetToken))
	assert.NotEqual(t, http.StatusUnauthorized, e.status(t, adminToken), "admin session is unaffected")

	var after models.User
	require.NoError(t, e.db.First(&after, target.ID).Error)
	assert.True(t, after.CheckPassword("reset-password-1"))
	assert.Equal(t, 0, after.FailedLoginAttempts)
	assert.Equal(t, target.SessionVersion+1, after.SessionVersion)
}

func TestUpdateUser_NonPasswordChangesDoNotRequireCurrentPassword(t *testing.T) {
	e := newCredentialsEnv(t)
	u, token := e.user(t, "self@example.com", false)
	w := e.do(t, http.MethodPut, "/users/"+strconv.FormatUint(uint64(u.ID), 10), token, map[string]any{"name": "New Name"})
	assert.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var after models.User
	require.NoError(t, e.db.First(&after, u.ID).Error)
	assert.Equal(t, "New Name", after.Name)
	assert.Equal(t, u.SessionVersion, after.SessionVersion)
}

func TestChangePasswordHandler_SessionRefreshFailureIsReported(t *testing.T) {
	e := newCredentialsEnv(t)
	_, token := e.user(t, "self@example.com", true)

	// After the password update succeeds, make every further lookup fail.
	var failLookups atomic.Bool
	require.NoError(t, e.db.Callback().Update().After("gorm:update").Register("test:arm", func(*gorm.DB) {
		failLookups.Store(true)
	}))
	require.NoError(t, e.db.Callback().Query().Before("gorm:query").Register("test:fail", func(tx *gorm.DB) {
		if failLookups.Load() {
			_ = tx.AddError(errors.New("lookup unavailable"))
		}
	}))

	w := e.do(t, http.MethodPost, "/auth/change-password", token, map[string]string{
		"old_password": "password123", "new_password": "another-password",
	})
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}
