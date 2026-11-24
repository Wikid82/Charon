package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

// seedLifecycleInvitee stores a pending invitee with the given expiry and enabled flag.
func seedLifecycleInvitee(t *testing.T, db *gorm.DB, email, token string, expires *time.Time, enabled bool) *models.User {
	t.Helper()
	user := &models.User{
		UUID:          uuid.NewString(),
		APIKey:        uuid.NewString(),
		Email:         email,
		Name:          "Invitee",
		Role:          models.RoleUser,
		InviteToken:   token,
		InviteExpires: expires,
		InviteStatus:  models.InviteStatusPending,
	}
	require.NoError(t, services.CreateKeepingSent(db, user, []string{"enabled"}))
	if enabled {
		require.NoError(t, db.Model(user).Update("enabled", true).Error)
	}
	return user
}

func lifecycleAdminRouter(handler *UserHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Set("userID", uint(9999)) // a different caller: the invitee is not the acting admin
		c.Next()
	})
	r.POST("/users/:id/resend-invite", handler.ResendInvite)
	r.PUT("/users/:id", handler.UpdateUser)
	return r
}

func TestResendInvite_RenewsLapsedInviteAndAudits(t *testing.T) {
	handler, db := setupUserHandlerWithProxyHosts(t)
	past := time.Now().Add(-24 * time.Hour)
	invitee := seedLifecycleInvitee(t, db, "renew@example.com", "old-token", &past, false)

	r := lifecycleAdminRouter(handler)
	req := httptest.NewRequest(http.MethodPost, "/users/"+strconv.FormatUint(uint64(invitee.ID), 10)+"/resend-invite", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var stored models.User
	require.NoError(t, db.First(&stored, invitee.ID).Error)
	assert.NotEqual(t, "old-token", stored.InviteToken)
	require.NotNil(t, stored.InviteExpires)
	assert.True(t, stored.InviteExpires.After(time.Now()))

	handler.securitySvc.Flush()
	var audits int64
	require.NoError(t, db.Model(&models.SecurityAudit{}).Where("action = ?", "user_invite_resend").Count(&audits).Error)
	assert.Equal(t, int64(1), audits)
}

func TestResendInvite_RefusesInviteWithPassword(t *testing.T) {
	handler, db := setupUserHandlerWithProxyHosts(t)
	future := time.Now().Add(time.Hour)
	invitee := seedLifecycleInvitee(t, db, "claimed-resend@example.com", "claimed-token", &future, false)
	require.NoError(t, db.Model(invitee).Update("password_hash", "admin-set-hash").Error)

	r := lifecycleAdminRouter(handler)
	req := httptest.NewRequest(http.MethodPost, "/users/"+strconv.FormatUint(uint64(invitee.ID), 10)+"/resend-invite", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "does not have a pending invite")

	var stored models.User
	require.NoError(t, db.First(&stored, invitee.ID).Error)
	assert.Equal(t, "claimed-token", stored.InviteToken)
}

func TestResendInvite_UnknownUserIsNotFound(t *testing.T) {
	handler, _ := setupUserHandlerWithProxyHosts(t)

	r := lifecycleAdminRouter(handler)
	req := httptest.NewRequest(http.MethodPost, "/users/4242/resend-invite", http.NoBody)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusNotFound, w.Code)
	assert.Contains(t, w.Body.String(), "User not found")
}

func TestUpdateUser_AdminResetClosesPendingInvite(t *testing.T) {
	handler, db := setupUserHandlerWithProxyHosts(t)
	future := time.Now().Add(time.Hour)
	invitee := seedLifecycleInvitee(t, db, "reset-closes@example.com", "reset-token", &future, false)

	r := lifecycleAdminRouter(handler)
	body, err := json.Marshal(map[string]any{"password": "admin-chosen-pw"})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPut, "/users/"+strconv.FormatUint(uint64(invitee.ID), 10), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var stored models.User
	require.NoError(t, db.First(&stored, invitee.ID).Error)
	assert.Empty(t, stored.InviteToken)
	assert.Nil(t, stored.InviteExpires)
	assert.Empty(t, stored.InviteStatus)
	assert.False(t, stored.Enabled, "a reset must not switch the account on")
	assert.True(t, stored.CheckPassword("admin-chosen-pw"))

	// The old link no longer works, in either step.
	validate := httptest.NewRequest(http.MethodGet, "/invite/validate?token=reset-token", http.NoBody)
	vw := httptest.NewRecorder()
	lifecyclePublicRouter(handler).ServeHTTP(vw, validate)
	assert.Equal(t, http.StatusNotFound, vw.Code)

	accept := postInviteAccept(t, lifecyclePublicRouter(handler), map[string]string{"token": "reset-token", "name": "Old", "password": "old-link-password"}, "192.0.2.30:1")
	assert.Equal(t, http.StatusNotFound, accept.Code)
	assert.True(t, stored.CheckPassword("admin-chosen-pw"), "the old link must not replace the admin-set password")
}

func TestUpdateUser_AdminResetKeepsLegacyEnabledFlag(t *testing.T) {
	handler, db := setupUserHandlerWithProxyHosts(t)
	future := time.Now().Add(time.Hour)
	invitee := seedLifecycleInvitee(t, db, "reset-legacy@example.com", "reset-legacy-token", &future, true)

	r := lifecycleAdminRouter(handler)
	body, err := json.Marshal(map[string]any{"password": "admin-chosen-pw"})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPut, "/users/"+strconv.FormatUint(uint64(invitee.ID), 10), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var stored models.User
	require.NoError(t, db.First(&stored, invitee.ID).Error)
	assert.True(t, stored.Enabled)
	assert.Empty(t, stored.InviteStatus)
}

func TestUpdateUser_AdminResetWithEnabledStillEnables(t *testing.T) {
	handler, db := setupUserHandlerWithProxyHosts(t)
	future := time.Now().Add(time.Hour)
	invitee := seedLifecycleInvitee(t, db, "reset-enable@example.com", "reset-enable-token", &future, false)

	r := lifecycleAdminRouter(handler)
	body, err := json.Marshal(map[string]any{"password": "admin-chosen-pw", "enabled": true})
	require.NoError(t, err)
	req := httptest.NewRequest(http.MethodPut, "/users/"+strconv.FormatUint(uint64(invitee.ID), 10), bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var stored models.User
	require.NoError(t, db.First(&stored, invitee.ID).Error)
	assert.True(t, stored.Enabled, "an explicit enabled in the same request applies as before")
	assert.Empty(t, stored.InviteToken)
}

// ginRouterFor mounts the public invite routes for the handler.
func lifecyclePublicRouter(handler *UserHandler) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.GET("/invite/validate", handler.ValidateInvite)
	r.POST("/invite/accept", handler.AcceptInvite)
	return r
}
