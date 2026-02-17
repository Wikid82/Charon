package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func newTestContextWithRequest() (*gin.Context, *httptest.ResponseRecorder) {
	rec := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(rec)
	ctx.Request = httptest.NewRequest(http.MethodGet, "/", http.NoBody)
	return ctx, rec
}

func TestRequireAdmin(t *testing.T) {
	t.Parallel()

	t.Run("admin allowed", func(t *testing.T) {
		t.Parallel()
		ctx, _ := newTestContextWithRequest()
		ctx.Set("role", "admin")
		assert.True(t, requireAdmin(ctx))
	})

	t.Run("non-admin forbidden", func(t *testing.T) {
		t.Parallel()
		ctx, rec := newTestContextWithRequest()
		ctx.Set("role", "user")
		assert.False(t, requireAdmin(ctx))
		assert.Equal(t, http.StatusForbidden, rec.Code)
		assert.Contains(t, rec.Body.String(), "admin privileges required")
	})
}

func TestIsAdmin(t *testing.T) {
	t.Parallel()

	ctx, _ := newTestContextWithRequest()
	assert.False(t, isAdmin(ctx))

	ctx.Set("role", "admin")
	assert.True(t, isAdmin(ctx))

	ctx.Set("role", "user")
	assert.False(t, isAdmin(ctx))
}

func TestPermissionErrorMessage(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "database is read-only", permissionErrorMessage("permissions_db_readonly"))
	assert.Equal(t, "database is locked", permissionErrorMessage("permissions_db_locked"))
	assert.Equal(t, "filesystem is read-only", permissionErrorMessage("permissions_readonly"))
	assert.Equal(t, "permission denied", permissionErrorMessage("permissions_write_denied"))
	assert.Equal(t, "permission error", permissionErrorMessage("something_else"))
}

func TestBuildPermissionHelp(t *testing.T) {
	t.Parallel()

	emptyPathHelp := buildPermissionHelp("")
	assert.Contains(t, emptyPathHelp, "chown -R")
	assert.Contains(t, emptyPathHelp, "<path-to-volume>")

	help := buildPermissionHelp("/data/path")
	assert.Contains(t, help, "chown -R")
	assert.Contains(t, help, "/data/path")
}

func TestRespondPermissionError_UnmappedReturnsFalse(t *testing.T) {
	t.Parallel()

	ctx, rec := newTestContextWithRequest()
	ok := respondPermissionError(ctx, nil, "action", errors.New("not mapped"), "/tmp")
	assert.False(t, ok)
	assert.Equal(t, http.StatusOK, rec.Code)
}

func TestRespondPermissionError_NonAdminMappedError(t *testing.T) {
	t.Parallel()

	ctx, rec := newTestContextWithRequest()
	ctx.Set("role", "user")

	ok := respondPermissionError(ctx, nil, "save_failed", errors.New("permission denied"), "/data")
	require.True(t, ok)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), "permission denied")
	assert.Contains(t, rec.Body.String(), "permissions_write_denied")
	assert.Contains(t, rec.Body.String(), "contact an administrator")
}

func TestRespondPermissionError_AdminWithAudit(t *testing.T) {
	t.Parallel()

	dbName := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SecurityAudit{}))

	securityService := services.NewSecurityService(db)
	t.Cleanup(func() {
		securityService.Close()
	})

	ctx, rec := newTestContextWithRequest()
	ctx.Set("role", "admin")
	ctx.Set("userID", uint(77))

	ok := respondPermissionError(ctx, securityService, "settings_save_failed", errors.New("database is locked"), "/var/lib/charon")
	require.True(t, ok)
	assert.Equal(t, http.StatusInternalServerError, rec.Code)
	assert.Contains(t, rec.Body.String(), "database is locked")
	assert.Contains(t, rec.Body.String(), "permissions_db_locked")
	assert.Contains(t, rec.Body.String(), "/var/lib/charon")

	securityService.Flush()

	var audits []models.SecurityAudit
	require.NoError(t, db.Find(&audits).Error)
	require.NotEmpty(t, audits)
	assert.Equal(t, "77", audits[0].Actor)
	assert.Equal(t, "settings_save_failed", audits[0].Action)
	assert.Equal(t, "permissions", audits[0].EventCategory)
}

func TestLogPermissionAudit_NoService(t *testing.T) {
	t.Parallel()

	ctx, _ := newTestContextWithRequest()
	assert.NotPanics(t, func() {
		logPermissionAudit(nil, ctx, "action", "permissions_write_denied", "/tmp", true)
	})
}

func TestLogPermissionAudit_ActorFallback(t *testing.T) {
	t.Parallel()

	dbName := "file:" + t.Name() + "?mode=memory&cache=shared"
	db, err := gorm.Open(sqlite.Open(dbName), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.SecurityAudit{}))

	securityService := services.NewSecurityService(db)
	t.Cleanup(func() {
		securityService.Close()
	})

	ctx, _ := newTestContextWithRequest()
	logPermissionAudit(securityService, ctx, "backup_create_failed", "permissions_readonly", "", false)
	securityService.Flush()

	var audit models.SecurityAudit
	require.NoError(t, db.First(&audit).Error)
	assert.Equal(t, "unknown", audit.Actor)
	assert.Equal(t, "backup_create_failed", audit.Action)
	assert.Equal(t, "permissions", audit.EventCategory)
	assert.Contains(t, audit.Details, fmt.Sprintf("\"admin\":%v", false))
}
