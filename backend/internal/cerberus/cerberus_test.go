package cerberus_test

import (
	"fmt"
	"testing"
	"time"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/cerberus"
	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/require"
)

func setupTestDB(t *testing.T) *gorm.DB {
	// Use a unique in-memory database per test run to avoid shared state.
	dsn := fmt.Sprintf("file:cerberus_test_%d?mode=memory&cache=shared", time.Now().UnixNano())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	// migrate only the Setting model used by Cerberus
	require.NoError(t, db.AutoMigrate(&models.Setting{}))
	return db
}

func TestCerberus_IsEnabled_ConfigTrue(t *testing.T) {
	db := setupTestDB(t)
	cfg := config.SecurityConfig{CerberusEnabled: true}
	cerb := cerberus.New(cfg, db)
	require.True(t, cerb.IsEnabled())
}

func TestCerberus_IsEnabled_DBSetting(t *testing.T) {
	db := setupTestDB(t)
	// We're storing 'security.cerberus.enabled' key
	db.Create(&models.Setting{Key: "security.cerberus.enabled", Value: "true"})
	cfg := config.SecurityConfig{CerberusEnabled: false}
	cerb := cerberus.New(cfg, db)
	require.True(t, cerb.IsEnabled())
}

func TestCerberus_IsEnabled_Disabled(t *testing.T) {
	db := setupTestDB(t)
	cfg := config.SecurityConfig{CerberusEnabled: false}
	cerb := cerberus.New(cfg, db)
	t.Logf("cfg: %+v", cfg)
	t.Logf("IsEnabled() -> %v", cerb.IsEnabled())
	require.False(t, cerb.IsEnabled())
}
