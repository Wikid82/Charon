package routes_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/api/routes"
	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

// setupBackupRoutingTestRouter spins up the full production router (the
// same routes.Register entry point cmd/api/main.go uses) against an
// in-memory DB, and returns an admin bearer token — required to prove the
// routing regression test #9 exercises Gin's real radix-tree matching
// behavior, not a hand-rolled subset of routes.
func setupBackupRoutingTestRouter(t *testing.T) (*gin.Engine, *gorm.DB, string) {
	t.Helper()
	gin.SetMode(gin.TestMode)

	dsn := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	cfg := config.Config{JWTSecret: "test-secret"}
	router := gin.New()
	require.NoError(t, routes.Register(context.Background(), router, db, cfg))

	admin := &models.User{Email: "admin@example.com", Role: models.RoleAdmin, Enabled: true}
	require.NoError(t, db.Create(admin).Error)
	authSvc := services.NewAuthService(db, cfg)
	token, err := authSvc.GenerateToken(admin)
	require.NoError(t, err)

	return router, db, token
}

func doAuthedRequest(router *gin.Engine, method, path, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, http.NoBody)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	return w
}

// TestBackupRoutes_StaticRoutesResolveToIntendedHandlers is required test
// #9 (part 1): GET/PUT /api/v1/backups/settings and every
// /api/v1/backups/remote-targets* method must dispatch to their intended
// handlers — never backupHandler.List/.Download/etc — despite being static
// siblings of the pre-existing /api/v1/backups/:filename[...] wildcard
// routes (spec §3.3). Verified via the registered route table's Handler
// field (the actual endpoint function, reported by Gin), which only exists
// because Gin's router successfully built a tree with these static and
// wildcard nodes coexisting — this is asserted, not assumed.
func TestBackupRoutes_StaticRoutesResolveToIntendedHandlers(t *testing.T) {
	router, _, _ := setupBackupRoutingTestRouter(t)

	expectedHandlerSuffix := map[string]string{
		"GET /api/v1/backups/settings":                   "BackupHandler).GetSettings",
		"PUT /api/v1/backups/settings":                   "BackupHandler).UpdateSettings",
		"GET /api/v1/backups/remote-targets":             "BackupRemoteHandler).List",
		"POST /api/v1/backups/remote-targets":            "BackupRemoteHandler).Create",
		"PUT /api/v1/backups/remote-targets/:uuid":       "BackupRemoteHandler).Update",
		"DELETE /api/v1/backups/remote-targets/:uuid":    "BackupRemoteHandler).Delete",
		"POST /api/v1/backups/remote-targets/:uuid/test": "BackupRemoteHandler).Test",
		"POST /api/v1/backups/upload":                    "BackupHandler).Upload",
		"POST /api/v1/backups/:filename/validate":        "BackupHandler).Validate",
		"GET /api/v1/backups":                            "BackupHandler).List",
		"DELETE /api/v1/backups/:filename":               "BackupHandler).Delete",
		"GET /api/v1/backups/:filename/download":         "BackupHandler).Download",
		"POST /api/v1/backups/:filename/restore":         "BackupHandler).Restore",
	}

	registered := map[string]string{}
	for _, r := range router.Routes() {
		registered[r.Method+" "+r.Path] = r.Handler
	}

	forbiddenSuffixes := []string{
		"BackupHandler).List", "BackupHandler).Delete", "BackupHandler).Download",
		"BackupHandler).Restore", "BackupHandler).Create", "BackupHandler).GetSettings",
		"BackupHandler).UpdateSettings", "BackupHandler).Upload", "BackupHandler).Validate",
		"BackupRemoteHandler).List", "BackupRemoteHandler).Create", "BackupRemoteHandler).Update",
		"BackupRemoteHandler).Delete", "BackupRemoteHandler).Test",
	}

	for key, wantSuffix := range expectedHandlerSuffix {
		gotHandler, ok := registered[key]
		require.Truef(t, ok, "route %s must be registered", key)
		assert.Containsf(t, gotHandler, wantSuffix, "route %s must dispatch to %s, got %s", key, wantSuffix, gotHandler)

		for _, forbidden := range forbiddenSuffixes {
			if forbidden == wantSuffix {
				continue
			}
			assert.NotContainsf(t, gotHandler, forbidden, "route %s must NOT dispatch to %s", key, forbidden)
		}
	}
}

// TestBackupRoutes_SettingsAndRemoteTargets_RuntimeDispatch supplements the
// registration-table check above with real end-to-end requests, proving
// Gin's tree walk actually resolves these paths to the intended handler at
// request time (not just at registration).
func TestBackupRoutes_SettingsAndRemoteTargets_RuntimeDispatch(t *testing.T) {
	router, _, token := setupBackupRoutingTestRouter(t)

	// GET /backups/settings must return the typed settings object
	// (schedule_cron key), never the backups list (a JSON array).
	settingsResp := doAuthedRequest(router, http.MethodGet, "/api/v1/backups/settings", token)
	require.Equal(t, http.StatusOK, settingsResp.Code)
	var settingsBody map[string]any
	require.NoError(t, json.Unmarshal(settingsResp.Body.Bytes(), &settingsBody))
	assert.Contains(t, settingsBody, "schedule_cron")

	// GET /backups/remote-targets must return the remote-targets list, not
	// be captured by any backup-filename route.
	remoteResp := doAuthedRequest(router, http.MethodGet, "/api/v1/backups/remote-targets", token)
	require.Equal(t, http.StatusOK, remoteResp.Code)
	var remoteBody []any
	require.NoError(t, json.Unmarshal(remoteResp.Body.Bytes(), &remoteBody))
}

// TestBackupRoutes_TestDraftDoesNotCollideWithUUIDTestRoute is the required
// routing regression test for the Issue #32 gap-closing fix: registering
// the new static POST /api/v1/backups/remote-targets/test-draft route
// alongside the pre-existing POST /api/v1/backups/remote-targets/:uuid/test
// param route must not panic at router build time and must resolve each
// path to its own distinct handler — Gin's router successfully building
// this tree (asserted via setupBackupRoutingTestRouter, which panics on any
// route-conflict error from routes.Register) is itself part of the proof.
//
// Handler names are compared with exact equality rather than
// strings.Contains: reflect-derived names are
// ".../BackupRemoteHandler).Test-fm" and
// ".../BackupRemoteHandler).TestDraft-fm" — "Test-fm" is not a substring of
// "TestDraft-fm", but a naive Contains("...).Test") check would spuriously
// match both routes, masking a real collision.
func TestBackupRoutes_TestDraftDoesNotCollideWithUUIDTestRoute(t *testing.T) {
	router, _, token := setupBackupRoutingTestRouter(t)

	registered := map[string]string{}
	for _, r := range router.Routes() {
		registered[r.Method+" "+r.Path] = r.Handler
	}

	testDraftHandler, ok := registered["POST /api/v1/backups/remote-targets/test-draft"]
	require.True(t, ok, "POST /backups/remote-targets/test-draft must be registered")
	assert.True(t, strings.HasSuffix(testDraftHandler, "BackupRemoteHandler).TestDraft-fm"), testDraftHandler)

	uuidTestHandler, ok := registered["POST /api/v1/backups/remote-targets/:uuid/test"]
	require.True(t, ok, "POST /backups/remote-targets/:uuid/test must still be registered")
	assert.True(t, strings.HasSuffix(uuidTestHandler, "BackupRemoteHandler).Test-fm"), uuidTestHandler)

	assert.NotEqual(t, testDraftHandler, uuidTestHandler)

	// Runtime dispatch: a request for the literal segment "test-draft" must
	// never be captured by the :uuid wildcard used by PUT/DELETE
	// /backups/remote-targets/:uuid.
	resp := doAuthedRequest(router, http.MethodPost, "/api/v1/backups/remote-targets/test-draft", token)
	assert.NotEqual(t, http.StatusNotFound, resp.Code, "test-draft must resolve to TestDraft, not 404 via a misrouted :uuid branch")

	var body map[string]any
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
	// No config/type in the request body ⇒ TestDraft's own validation
	// rejects it (400) — proves this request actually reached TestDraft's
	// logic, not some other handler.
	assert.Equal(t, http.StatusBadRequest, resp.Code)
}

// TestBackupRoutes_DeleteSettingsFallsThroughSafely is required test #9
// (part 2): DELETE /api/v1/backups/settings has no static sibling
// registered (spec §3.3), so it falls through to
// DELETE /api/v1/backups/:filename with filename="settings" — this proves
// the existing sanitize/not-found logic in DeleteBackup handles the literal
// segment "settings" safely: a 404, and no interaction whatsoever with the
// Setting GORM model.
func TestBackupRoutes_DeleteSettingsFallsThroughSafely(t *testing.T) {
	router, db, token := setupBackupRoutingTestRouter(t)

	var settingsCountBefore int64
	require.NoError(t, db.Model(&models.Setting{}).Count(&settingsCountBefore).Error)

	resp := doAuthedRequest(router, http.MethodDelete, "/api/v1/backups/settings", token)
	require.Equal(t, http.StatusNotFound, resp.Code)

	var body map[string]any
	require.NoError(t, json.Unmarshal(resp.Body.Bytes(), &body))
	assert.Contains(t, body["error"], "not found")

	var settingsCountAfter int64
	require.NoError(t, db.Model(&models.Setting{}).Count(&settingsCountAfter).Error)
	assert.Equal(t, settingsCountBefore, settingsCountAfter, "DELETE /backups/settings must never create/modify a Setting row")
}
