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
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/changelog"
	"github.com/Wikid82/charon/backend/internal/models"
)

// setChangelogEntriesForTest seeds the changelog package's data for the
// duration of a test via the cross-package test seam
// changelog.SetEntriesForTesting (a plain _test.go helper inside the
// changelog package itself would not be visible here).
func setChangelogEntriesForTest(t *testing.T, _ *changelog.Service, entries []changelog.Entry) {
	t.Helper()
	t.Cleanup(changelog.SetEntriesForTesting(entries))
}

func setupChangelogHandler(t *testing.T) (*ChangelogHandler, *gorm.DB, *changelog.Service) {
	t.Helper()
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.User{}))
	svc := changelog.NewService("2.0.0")
	return NewChangelogHandler(db, svc), db, svc
}

// buildChangelogRouter wires the four changelog routes behind a fake auth
// middleware, mirroring buildThemeRouter's pattern in
// custom_theme_handler_test.go. When userID is 0, no "userID" context key
// is set at all (simulating a request the auth middleware never touched).
func buildChangelogRouter(h *ChangelogHandler, role string, userID uint) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(func(c *gin.Context) {
		if userID != 0 {
			c.Set("userID", userID)
		}
		c.Set("role", role)
		c.Next()
	})
	r.GET("/changelog/status", h.Status)
	r.GET("/changelog/all", h.All)
	r.POST("/changelog/ack", h.Ack)
	r.POST("/changelog/opt-in", h.OptIn)
	return r
}

func createChangelogTestUser(t *testing.T, db *gorm.DB, lastSeen string, optOut bool) uint {
	t.Helper()
	user := models.User{
		UUID:            "test-uuid-" + t.Name(),
		Email:           t.Name() + "@example.com",
		Name:            "Test User",
		Role:            models.RoleUser,
		Enabled:         true,
		LastSeenVersion: lastSeen,
		ChangelogOptOut: optOut,
	}
	require.NoError(t, db.Create(&user).Error)
	return user.ID
}

func doJSON(t *testing.T, r *gin.Engine, method, path string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var reqBody *bytes.Buffer
	if body != nil {
		b, err := json.Marshal(body)
		require.NoError(t, err)
		reqBody = bytes.NewBuffer(b)
	} else {
		reqBody = bytes.NewBuffer(nil)
	}
	req := httptest.NewRequest(method, path, reqBody)
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// --- Status ---

func TestChangelogHandler_Status_ShowsEntriesSinceLastSeen(t *testing.T) {
	h, db, svc := setupChangelogHandler(t)
	setChangelogEntriesForTest(t, svc, []changelog.Entry{
		{Version: "1.0.0", Features: []string{"old"}},
		{Version: "2.0.0", Features: []string{"new"}},
	})
	userID := createChangelogTestUser(t, db, "1.0.0", false)
	r := buildChangelogRouter(h, "user", userID)

	w := doJSON(t, r, http.MethodGet, "/changelog/status", nil)

	require.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		ShowChangelog bool              `json:"show_changelog"`
		Versions      []changelog.Entry `json:"versions"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.True(t, resp.ShowChangelog)
	require.Len(t, resp.Versions, 1)
	assert.Equal(t, "2.0.0", resp.Versions[0].Version)
}

func TestChangelogHandler_Status_DevBuild_NeverShows(t *testing.T) {
	h, db, svc := setupChangelogHandler(t)
	svc.SetCurrentVersion("dev")
	setChangelogEntriesForTest(t, svc, []changelog.Entry{{Version: "1.0.0"}})
	userID := createChangelogTestUser(t, db, "", false)
	r := buildChangelogRouter(h, "user", userID)

	w := doJSON(t, r, http.MethodGet, "/changelog/status", nil)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, false, resp["show_changelog"])
}

func TestChangelogHandler_Status_OptedOut_NeverShows(t *testing.T) {
	h, db, svc := setupChangelogHandler(t)
	setChangelogEntriesForTest(t, svc, []changelog.Entry{{Version: "2.0.0"}})
	userID := createChangelogTestUser(t, db, "", true)
	r := buildChangelogRouter(h, "user", userID)

	w := doJSON(t, r, http.MethodGet, "/changelog/status", nil)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, false, resp["show_changelog"])
}

func TestChangelogHandler_Status_ZeroUnseenEntries_NeverShows(t *testing.T) {
	h, db, svc := setupChangelogHandler(t)
	setChangelogEntriesForTest(t, svc, []changelog.Entry{{Version: "1.0.0"}})
	userID := createChangelogTestUser(t, db, "2.0.0", false)
	r := buildChangelogRouter(h, "user", userID)

	w := doJSON(t, r, http.MethodGet, "/changelog/status", nil)

	require.Equal(t, http.StatusOK, w.Code)
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, false, resp["show_changelog"])
}

func TestChangelogHandler_Status_UserNotFound_Returns404(t *testing.T) {
	h, _, svc := setupChangelogHandler(t)
	setChangelogEntriesForTest(t, svc, []changelog.Entry{{Version: "2.0.0"}})
	r := buildChangelogRouter(h, "user", 9999)

	w := doJSON(t, r, http.MethodGet, "/changelog/status", nil)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestChangelogHandler_Status_PassthroughRejected(t *testing.T) {
	h, db, _ := setupChangelogHandler(t)
	userID := createChangelogTestUser(t, db, "", false)
	r := buildChangelogRouter(h, string(models.RolePassthrough), userID)

	w := doJSON(t, r, http.MethodGet, "/changelog/status", nil)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

func TestChangelogHandler_Status_MissingUserID_Unauthorized(t *testing.T) {
	h, _, _ := setupChangelogHandler(t)
	r := buildChangelogRouter(h, "user", 0)

	w := doJSON(t, r, http.MethodGet, "/changelog/status", nil)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

// --- All ---

func TestChangelogHandler_All_ReturnsFullHistory(t *testing.T) {
	h, db, svc := setupChangelogHandler(t)
	setChangelogEntriesForTest(t, svc, []changelog.Entry{
		{Version: "1.0.0"},
		{Version: "2.0.0"},
	})
	userID := createChangelogTestUser(t, db, "2.0.0", false)
	r := buildChangelogRouter(h, "user", userID)

	w := doJSON(t, r, http.MethodGet, "/changelog/all", nil)

	require.Equal(t, http.StatusOK, w.Code)
	var resp struct {
		Versions []changelog.Entry `json:"versions"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Len(t, resp.Versions, 2)
	assert.Equal(t, "2.0.0", resp.Versions[0].Version)
}

func TestChangelogHandler_All_PassthroughRejected(t *testing.T) {
	h, db, _ := setupChangelogHandler(t)
	userID := createChangelogTestUser(t, db, "", false)
	r := buildChangelogRouter(h, string(models.RolePassthrough), userID)

	w := doJSON(t, r, http.MethodGet, "/changelog/all", nil)

	assert.Equal(t, http.StatusForbidden, w.Code)
}

// --- Ack ---

func TestChangelogHandler_Ack_DismissPermanent_UpdatesLastSeenVersion(t *testing.T) {
	h, db, svc := setupChangelogHandler(t)
	userID := createChangelogTestUser(t, db, "1.0.0", false)
	r := buildChangelogRouter(h, "user", userID)

	w := doJSON(t, r, http.MethodPost, "/changelog/ack", map[string]any{
		"action": "dismiss_permanent", "opt_out": false,
	})

	require.Equal(t, http.StatusOK, w.Code)
	var user models.User
	require.NoError(t, db.First(&user, userID).Error)
	assert.Equal(t, svc.CurrentVersion(), user.LastSeenVersion)
	assert.False(t, user.ChangelogOptOut)
}

func TestChangelogHandler_Ack_DismissTemporary_DoesNotUpdateLastSeenVersion(t *testing.T) {
	h, db, _ := setupChangelogHandler(t)
	userID := createChangelogTestUser(t, db, "1.0.0", false)
	r := buildChangelogRouter(h, "user", userID)

	w := doJSON(t, r, http.MethodPost, "/changelog/ack", map[string]any{
		"action": "dismiss_temporary", "opt_out": false,
	})

	require.Equal(t, http.StatusOK, w.Code)
	var user models.User
	require.NoError(t, db.First(&user, userID).Error)
	assert.Equal(t, "1.0.0", user.LastSeenVersion, "temporary dismiss must not advance last_seen_version")
}

func TestChangelogHandler_Ack_OptOutTrue_SetsChangelogOptOut(t *testing.T) {
	h, db, _ := setupChangelogHandler(t)
	userID := createChangelogTestUser(t, db, "1.0.0", false)
	r := buildChangelogRouter(h, "user", userID)

	w := doJSON(t, r, http.MethodPost, "/changelog/ack", map[string]any{
		"action": "dismiss_temporary", "opt_out": true,
	})

	require.Equal(t, http.StatusOK, w.Code)
	var user models.User
	require.NoError(t, db.First(&user, userID).Error)
	assert.True(t, user.ChangelogOptOut)
}

func TestChangelogHandler_Ack_InvalidAction_BadRequest(t *testing.T) {
	h, db, _ := setupChangelogHandler(t)
	userID := createChangelogTestUser(t, db, "1.0.0", false)
	r := buildChangelogRouter(h, "user", userID)

	w := doJSON(t, r, http.MethodPost, "/changelog/ack", map[string]any{
		"action": "not_a_real_action", "opt_out": false,
	})

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestChangelogHandler_Ack_UserNotFound_Returns404(t *testing.T) {
	h, _, _ := setupChangelogHandler(t)
	r := buildChangelogRouter(h, "user", 9999)

	w := doJSON(t, r, http.MethodPost, "/changelog/ack", map[string]any{
		"action": "dismiss_permanent", "opt_out": false,
	})

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestChangelogHandler_Ack_MissingUserID_Unauthorized(t *testing.T) {
	h, _, _ := setupChangelogHandler(t)
	r := buildChangelogRouter(h, "user", 0)

	w := doJSON(t, r, http.MethodPost, "/changelog/ack", map[string]any{
		"action": "dismiss_permanent", "opt_out": false,
	})

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestChangelogHandler_Ack_DBError(t *testing.T) {
	h, db, _ := setupChangelogHandler(t)
	userID := createChangelogTestUser(t, db, "1.0.0", false)
	r := buildChangelogRouter(h, "user", userID)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	w := doJSON(t, r, http.MethodPost, "/changelog/ack", map[string]any{
		"action": "dismiss_permanent", "opt_out": false,
	})

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestChangelogHandler_Ack_PassthroughRejected(t *testing.T) {
	h, db, _ := setupChangelogHandler(t)
	userID := createChangelogTestUser(t, db, "", false)
	r := buildChangelogRouter(h, string(models.RolePassthrough), userID)

	w := doJSON(t, r, http.MethodPost, "/changelog/ack", map[string]any{
		"action": "dismiss_permanent", "opt_out": false,
	})

	assert.Equal(t, http.StatusForbidden, w.Code)
}

// --- OptIn ---

func TestChangelogHandler_OptIn_ClearsChangelogOptOut(t *testing.T) {
	h, db, _ := setupChangelogHandler(t)
	userID := createChangelogTestUser(t, db, "1.0.0", true)
	r := buildChangelogRouter(h, "user", userID)

	w := doJSON(t, r, http.MethodPost, "/changelog/opt-in", nil)

	require.Equal(t, http.StatusOK, w.Code)
	var user models.User
	require.NoError(t, db.First(&user, userID).Error)
	assert.False(t, user.ChangelogOptOut)
}

func TestChangelogHandler_OptIn_MissingUserID_Unauthorized(t *testing.T) {
	h, _, _ := setupChangelogHandler(t)
	r := buildChangelogRouter(h, "user", 0)

	w := doJSON(t, r, http.MethodPost, "/changelog/opt-in", nil)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestChangelogHandler_OptIn_DBError(t *testing.T) {
	h, db, _ := setupChangelogHandler(t)
	userID := createChangelogTestUser(t, db, "1.0.0", true)
	r := buildChangelogRouter(h, "user", userID)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	w := doJSON(t, r, http.MethodPost, "/changelog/opt-in", nil)

	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestChangelogHandler_OptIn_UserNotFound_Returns404(t *testing.T) {
	h, _, _ := setupChangelogHandler(t)
	r := buildChangelogRouter(h, "user", 9999)

	w := doJSON(t, r, http.MethodPost, "/changelog/opt-in", nil)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestChangelogHandler_OptIn_PassthroughRejected(t *testing.T) {
	h, db, _ := setupChangelogHandler(t)
	userID := createChangelogTestUser(t, db, "", false)
	r := buildChangelogRouter(h, string(models.RolePassthrough), userID)

	w := doJSON(t, r, http.MethodPost, "/changelog/opt-in", nil)

	assert.Equal(t, http.StatusForbidden, w.Code)
}
