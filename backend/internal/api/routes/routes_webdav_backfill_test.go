package routes

import (
	"context"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
)

// A WebDAV target saved over http:// before the HTTPS rule is flagged by the next start-up, and unknown config keys survive.
func TestRegister_BackfillsExistingWebDAVHTTPTargetsOnStart(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db, err := gorm.Open(sqlite.Open(isolatedMemoryDSN(t)), &gorm.Config{})
	require.NoError(t, err)
	cfg := config.Config{JWTSecret: "test-secret"}

	require.NoError(t, Register(context.Background(), gin.New(), db, cfg))

	legacy := &models.RemoteStorageTarget{
		UUID:           uuid.NewString(),
		Name:           "legacy",
		Type:           "webdav",
		Enabled:        true,
		ConfigJSON:     `{"webdav":{"url":"http://203.0.113.10/dav/"},"future_key":"kept"}`,
		LastTestStatus: "never",
	}
	require.NoError(t, db.Create(legacy).Error)

	require.NoError(t, Register(context.Background(), gin.New(), db, cfg))

	var stored models.RemoteStorageTarget
	require.NoError(t, db.Where("uuid = ?", legacy.UUID).First(&stored).Error)
	assert.Contains(t, stored.ConfigJSON, `"allow_insecure_http":true`)
	assert.Contains(t, stored.ConfigJSON, `"future_key":"kept"`)
}
