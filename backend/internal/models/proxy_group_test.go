package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupProxyGroupModelDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&ProxyGroup{}))
	return db
}

func TestProxyGroupBeforeCreate_AssignsUUID(t *testing.T) {
	db := setupProxyGroupModelDB(t)
	g := &ProxyGroup{Name: "auto uuid group"}
	require.NoError(t, db.Create(g).Error)
	assert.NotEmpty(t, g.UUID)
}

func TestProxyGroupBeforeCreate_PreservesExistingUUID(t *testing.T) {
	db := setupProxyGroupModelDB(t)
	preset := "preset-uuid-value-12345"
	g := &ProxyGroup{Name: "preset uuid group", UUID: preset}
	require.NoError(t, db.Create(g).Error)
	assert.Equal(t, preset, g.UUID)
}
