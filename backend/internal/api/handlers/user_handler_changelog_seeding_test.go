package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/version"
)

// withVersion temporarily overrides version.Version for the duration of a
// test, restoring the original value on cleanup. version.Version defaults
// to "dev" and is only ldflags-injected in release builds, so tests that
// need a "real" release version must set it explicitly.
func withVersion(t *testing.T, v string) {
	t.Helper()
	original := version.Version
	version.Version = v
	t.Cleanup(func() { version.Version = original })
}

// --- Setup ---

func TestUserHandler_Setup_SeedsLastSeenVersion(t *testing.T) {
	withVersion(t, "1.2.3")
	handler, db := setupUserHandler(t)
	r := gin.New()
	r.POST("/setup", handler.Setup)

	body := map[string]string{"name": "Admin", "email": "seed-admin@example.com", "password": "password123"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/setup", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)

	var user models.User
	require.NoError(t, db.Where("email = ?", "seed-admin@example.com").First(&user).Error)
	assert.Equal(t, "1.2.3", user.LastSeenVersion)
}

func TestUserHandler_Setup_SkipsSeedingOnDevBuild(t *testing.T) {
	withVersion(t, "dev")
	handler, db := setupUserHandler(t)
	r := gin.New()
	r.POST("/setup", handler.Setup)

	body := map[string]string{"name": "Admin", "email": "dev-admin@example.com", "password": "password123"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/setup", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)

	var user models.User
	require.NoError(t, db.Where("email = ?", "dev-admin@example.com").First(&user).Error)
	assert.Equal(t, "", user.LastSeenVersion)
}

// --- CreateUser ---

func TestUserHandler_CreateUser_SeedsLastSeenVersion(t *testing.T) {
	withVersion(t, "3.4.5")
	handler, db := setupUserHandlerWithProxyHosts(t)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Set("userID", uint(1))
		c.Next()
	})
	r.POST("/users", handler.CreateUser)

	body := map[string]any{"email": "created-seeded@example.com", "name": "New User", "password": "password123"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)

	var user models.User
	require.NoError(t, db.Where("email = ?", "created-seeded@example.com").First(&user).Error)
	assert.Equal(t, "3.4.5", user.LastSeenVersion)
}

func TestUserHandler_CreateUser_SkipsSeedingOnDevBuild(t *testing.T) {
	withVersion(t, "dev")
	handler, db := setupUserHandlerWithProxyHosts(t)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Set("userID", uint(1))
		c.Next()
	})
	r.POST("/users", handler.CreateUser)

	body := map[string]any{"email": "created-devbuild@example.com", "name": "New User", "password": "password123"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)

	var user models.User
	require.NoError(t, db.Where("email = ?", "created-devbuild@example.com").First(&user).Error)
	assert.Equal(t, "", user.LastSeenVersion)
}

// --- AcceptInvite ---

func TestUserHandler_AcceptInvite_SeedsLastSeenVersion(t *testing.T) {
	withVersion(t, "5.6.7")
	handler, db := setupUserHandlerWithProxyHosts(t)

	expiresAt := time.Now().Add(24 * time.Hour)
	user := &models.User{
		UUID:          uuid.NewString(),
		Email:         "accept-seeded@example.com",
		Name:          "Accept User",
		InviteToken:   "accept-seed-token",
		InviteExpires: &expiresAt,
		InviteStatus:  "pending",
	}
	require.NoError(t, db.Create(user).Error)

	r := gin.New()
	r.POST("/invite/accept", handler.AcceptInvite)

	body := map[string]string{"token": "accept-seed-token", "password": "newpassword123", "name": "Accepted User"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/invite/accept", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var updated models.User
	require.NoError(t, db.First(&updated, user.ID).Error)
	assert.Equal(t, "5.6.7", updated.LastSeenVersion)
}

func TestUserHandler_AcceptInvite_SkipsSeedingOnDevBuild(t *testing.T) {
	withVersion(t, "dev")
	handler, db := setupUserHandlerWithProxyHosts(t)

	expiresAt := time.Now().Add(24 * time.Hour)
	user := &models.User{
		UUID:          uuid.NewString(),
		Email:         "accept-devbuild@example.com",
		Name:          "Accept User",
		InviteToken:   "accept-dev-token",
		InviteExpires: &expiresAt,
		InviteStatus:  "pending",
	}
	require.NoError(t, db.Create(user).Error)

	r := gin.New()
	r.POST("/invite/accept", handler.AcceptInvite)

	body := map[string]string{"token": "accept-dev-token", "password": "newpassword123", "name": "Accepted User"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/invite/accept", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var updated models.User
	require.NoError(t, db.First(&updated, user.ID).Error)
	assert.Equal(t, "", updated.LastSeenVersion)
}

// --- InviteUser: deliberately NOT seeded (pending/disabled row) ---

func TestUserHandler_InviteUser_DoesNotSeedLastSeenVersion(t *testing.T) {
	withVersion(t, "1.0.0")
	handler, db := setupUserHandlerWithProxyHosts(t)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		c.Set("role", "admin")
		c.Set("userID", uint(1))
		c.Next()
	})
	r.POST("/users/invite", handler.InviteUser)

	body := map[string]any{"email": "invited-not-seeded@example.com"}
	jsonBody, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/users/invite", bytes.NewBuffer(jsonBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusCreated, w.Code)

	var user models.User
	require.NoError(t, db.Where("email = ?", "invited-not-seeded@example.com").First(&user).Error)
	assert.Equal(t, "", user.LastSeenVersion, "InviteUser's pending row must not be seeded — only AcceptInvite seeds it")
}

func TestSeedLastSeenVersion(t *testing.T) {
	withVersion(t, "dev")
	assert.Equal(t, "", seedLastSeenVersion())

	withVersion(t, "9.9.9")
	assert.Equal(t, "9.9.9", seedLastSeenVersion())
}

// TestSeedLastSeenVersion_NonSemverVersion covers the CI-produced version
// strings that aren't literally "dev" but also aren't valid semver —
// e.g. nightly-build.yml/docker-build.yml tag distributable images as
// "nightly-<git-sha>". Before seedLastSeenVersion delegated to
// changelog.IsUnversionedBuild, only the literal "dev" sentinel was
// checked here, so a user created while running a nightly image got
// seeded with an invalid, non-empty LastSeenVersion that would
// permanently block them from ever seeing the changelog once the
// deployment upgraded to a real tagged release (see
// changelog.Service.GetEntriesSince's invalid-lastSeen guard).
func TestSeedLastSeenVersion_NonSemverVersion(t *testing.T) {
	for _, v := range []string{"nightly-a1b2c3d", "main", "development", "branch-feat-foo-a1b2c3d"} {
		withVersion(t, v)
		assert.Equalf(t, "", seedLastSeenVersion(), "expected seedLastSeenVersion()==\"\" for non-semver version %q", v)
	}
}
