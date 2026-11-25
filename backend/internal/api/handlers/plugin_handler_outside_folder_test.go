package handlers

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

// TestPluginHandler_EnablePlugin_StoredPathOutsidePluginFolderIsRefused pins that enabling a plugin whose stored path is
// outside the plugin folder reports a load failure and never opens that file. The DB row is still enabled, as for every
// load failure.
func TestPluginHandler_EnablePlugin_StoredPathOutsidePluginFolderIsRefused(t *testing.T) {
	db := OpenTestDBWithMigrations(t)
	outside := filepath.Join(t.TempDir(), "elsewhere.so")
	require.NoError(t, os.WriteFile(outside, []byte("not loaded"), 0o600))
	pluginLoader := services.NewPluginLoaderService(db, t.TempDir(), t.TempDir(), nil)

	plugin := models.Plugin{
		UUID:     "outside-folder-uuid",
		Name:     "Outside Folder Plugin",
		Type:     "outside-folder-type",
		Enabled:  false,
		Status:   models.PluginStatusError,
		FilePath: outside,
	}
	require.NoError(t, db.Create(&plugin).Error)
	// Enabled has a column default of true, so the disabled state is written after the insert.
	require.NoError(t, db.Model(&plugin).Update("enabled", false).Error)

	router := gin.New()
	router.POST("/plugins/:id/enable", NewPluginHandler(db, pluginLoader).EnablePlugin)
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/plugins/%d/enable", plugin.ID), http.NoBody)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	assert.Equal(t, http.StatusOK, w.Code)
	assert.True(t, strings.Contains(w.Body.String(), "enabled but failed to load"), "got: %s", w.Body.String())
	assert.True(t, strings.Contains(w.Body.String(), "not inside the plugin folder"), "got: %s", w.Body.String())
	var updated models.Plugin
	require.NoError(t, db.First(&updated, plugin.ID).Error)
	assert.True(t, updated.Enabled)
}
