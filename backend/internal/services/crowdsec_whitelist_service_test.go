package services_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

func openWhitelistTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.CrowdSecWhitelist{}))
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

func TestCrowdSecWhitelistService_List_Empty(t *testing.T) {
	t.Parallel()
	svc := services.NewCrowdSecWhitelistService(openWhitelistTestDB(t), "")
	entries, err := svc.List(context.Background())
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestCrowdSecWhitelistService_Add_ValidIP(t *testing.T) {
	t.Parallel()
	svc := services.NewCrowdSecWhitelistService(openWhitelistTestDB(t), "")
	entry, err := svc.Add(context.Background(), "1.2.3.4", "test reason")
	require.NoError(t, err)
	assert.NotEmpty(t, entry.UUID)
	assert.Equal(t, "1.2.3.4", entry.IPOrCIDR)
	assert.Equal(t, "test reason", entry.Reason)
}

func TestCrowdSecWhitelistService_Add_ValidCIDR(t *testing.T) {
	t.Parallel()
	svc := services.NewCrowdSecWhitelistService(openWhitelistTestDB(t), "")
	entry, err := svc.Add(context.Background(), "192.168.1.0/24", "local net")
	require.NoError(t, err)
	assert.Equal(t, "192.168.1.0/24", entry.IPOrCIDR)
}

func TestCrowdSecWhitelistService_Add_NormalizesCIDR(t *testing.T) {
	t.Parallel()
	svc := services.NewCrowdSecWhitelistService(openWhitelistTestDB(t), "")
	entry, err := svc.Add(context.Background(), "10.0.0.1/8", "normalize test")
	require.NoError(t, err)
	assert.Equal(t, "10.0.0.0/8", entry.IPOrCIDR)
}

func TestCrowdSecWhitelistService_Add_InvalidIP(t *testing.T) {
	t.Parallel()
	svc := services.NewCrowdSecWhitelistService(openWhitelistTestDB(t), "")
	_, err := svc.Add(context.Background(), "not-an-ip", "")
	assert.ErrorIs(t, err, services.ErrInvalidIPOrCIDR)
}

func TestCrowdSecWhitelistService_Add_Duplicate(t *testing.T) {
	t.Parallel()
	db := openWhitelistTestDB(t)
	svc := services.NewCrowdSecWhitelistService(db, "")
	_, err := svc.Add(context.Background(), "5.5.5.5", "first")
	require.NoError(t, err)
	_, err = svc.Add(context.Background(), "5.5.5.5", "second")
	assert.ErrorIs(t, err, services.ErrDuplicateEntry)
}

func TestCrowdSecWhitelistService_Delete_Existing(t *testing.T) {
	t.Parallel()
	db := openWhitelistTestDB(t)
	svc := services.NewCrowdSecWhitelistService(db, "")
	entry, err := svc.Add(context.Background(), "6.6.6.6", "to delete")
	require.NoError(t, err)

	err = svc.Delete(context.Background(), entry.UUID)
	require.NoError(t, err)

	entries, err := svc.List(context.Background())
	require.NoError(t, err)
	assert.Empty(t, entries)
}

func TestCrowdSecWhitelistService_Delete_NotFound(t *testing.T) {
	t.Parallel()
	svc := services.NewCrowdSecWhitelistService(openWhitelistTestDB(t), "")
	err := svc.Delete(context.Background(), "00000000-0000-0000-0000-000000000000")
	assert.ErrorIs(t, err, services.ErrWhitelistNotFound)
}

func TestCrowdSecWhitelistService_WriteYAML_EmptyDataDir(t *testing.T) {
	t.Parallel()
	svc := services.NewCrowdSecWhitelistService(openWhitelistTestDB(t), "")
	err := svc.WriteYAML(context.Background())
	assert.NoError(t, err)
}

func TestCrowdSecWhitelistService_WriteYAML_CreatesFile(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	db := openWhitelistTestDB(t)
	svc := services.NewCrowdSecWhitelistService(db, tmpDir)

	_, err := svc.Add(context.Background(), "1.1.1.1", "dns")
	require.NoError(t, err)
	_, err = svc.Add(context.Background(), "10.0.0.0/8", "internal")
	require.NoError(t, err)

	yamlPath := filepath.Join(tmpDir, "config", "parsers", "s02-enrich", "charon-whitelist.yaml")
	content, err := os.ReadFile(yamlPath)
	require.NoError(t, err)

	s := string(content)
	assert.Contains(t, s, "name: charon-whitelist")
	assert.Contains(t, s, `"1.1.1.1"`)
	assert.Contains(t, s, `"10.0.0.0/8"`)
}

func TestCrowdSecWhitelistService_WriteYAML_EmptyLists(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	svc := services.NewCrowdSecWhitelistService(openWhitelistTestDB(t), tmpDir)

	err := svc.WriteYAML(context.Background())
	require.NoError(t, err)

	yamlPath := filepath.Join(tmpDir, "config", "parsers", "s02-enrich", "charon-whitelist.yaml")
	content, err := os.ReadFile(yamlPath)
	require.NoError(t, err)

	s := string(content)
	assert.Contains(t, s, "ip: []")
	assert.Contains(t, s, "cidr: []")
}

func TestCrowdSecWhitelistService_List_AfterAdd(t *testing.T) {
	t.Parallel()
	db := openWhitelistTestDB(t)
	svc := services.NewCrowdSecWhitelistService(db, "")

	for i := 0; i < 3; i++ {
		_, err := svc.Add(context.Background(), fmt.Sprintf("10.0.0.%d", i+1), "")
		require.NoError(t, err)
	}

	entries, err := svc.List(context.Background())
	require.NoError(t, err)
	assert.Len(t, entries, 3)
}

func TestAdd_ValidIPv6_Success(t *testing.T) {
	t.Parallel()
	svc := services.NewCrowdSecWhitelistService(openWhitelistTestDB(t), "")
	entry, err := svc.Add(context.Background(), "2001:db8::1", "ipv6 test")
	require.NoError(t, err)
	assert.Equal(t, "2001:db8::1", entry.IPOrCIDR)

	entries, err := svc.List(context.Background())
	require.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.Equal(t, "2001:db8::1", entries[0].IPOrCIDR)
}
