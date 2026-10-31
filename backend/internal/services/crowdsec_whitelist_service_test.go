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
	content, err := os.ReadFile(yamlPath) //nolint:gosec // G304: path built from controlled tmpDir
	require.NoError(t, err)

	s := string(content)
	assert.Contains(t, s, "name: charon-whitelist")
	assert.Contains(t, s, `"1.1.1.1"`)
	assert.Contains(t, s, `"10.0.0.0/8"`)
}

func TestCrowdSecWhitelistService_WriteYAML_EmptyListWritesPlaceholder(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	svc := services.NewCrowdSecWhitelistService(openWhitelistTestDB(t), tmpDir)

	err := svc.WriteYAML(context.Background())
	require.NoError(t, err)

	// CrowdSec rejects a parser with empty ip/cidr arrays, so the file carries a never-matching expression instead
	// of the lists. The file itself stays in place.
	yamlPath := filepath.Join(tmpDir, "config", "parsers", "s02-enrich", "charon-whitelist.yaml")
	content, err := os.ReadFile(yamlPath) //nolint:gosec // G304: path built from controlled tmpDir
	require.NoError(t, err)
	assert.Equal(t, wantWhitelistPlaceholder, string(content))
}

func TestCrowdSecWhitelistService_WriteYAML_DeleteLastEntryKeepsPlaceholder(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	db := openWhitelistTestDB(t)
	svc := services.NewCrowdSecWhitelistService(db, tmpDir)

	// Write the file by adding an entry.
	_, err := svc.Add(context.Background(), "1.1.1.1", "will be deleted")
	require.NoError(t, err)
	yamlPath := filepath.Join(tmpDir, "config", "parsers", "s02-enrich", "charon-whitelist.yaml")
	require.FileExists(t, yamlPath)

	// Delete the last entry. The file is replaced by the placeholder, never removed: a running engine can die when
	// its whitelist file disappears between reloads.
	entries, err := svc.List(context.Background())
	require.NoError(t, err)
	require.Len(t, entries, 1)
	err = svc.Delete(context.Background(), entries[0].UUID)
	require.NoError(t, err)

	content, err := os.ReadFile(yamlPath) //nolint:gosec // G304: path built from controlled tmpDir
	require.NoError(t, err)
	assert.Equal(t, wantWhitelistPlaceholder, string(content))
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

func TestCrowdSecWhitelistService_List_DBError(t *testing.T) {
	t.Parallel()
	db := openWhitelistTestDB(t)
	svc := services.NewCrowdSecWhitelistService(db, "")
	sqlDB, err := db.DB()
	require.NoError(t, err)
	_ = sqlDB.Close()

	_, err = svc.List(context.Background())
	assert.Error(t, err)
}

func TestCrowdSecWhitelistService_Add_DBCreateError(t *testing.T) {
	t.Parallel()
	db := openWhitelistTestDB(t)
	svc := services.NewCrowdSecWhitelistService(db, "")
	sqlDB, err := db.DB()
	require.NoError(t, err)
	_ = sqlDB.Close()

	_, err = svc.Add(context.Background(), "1.2.3.4", "test")
	assert.Error(t, err)
	assert.NotErrorIs(t, err, services.ErrInvalidIPOrCIDR)
	assert.NotErrorIs(t, err, services.ErrDuplicateEntry)
}

func TestCrowdSecWhitelistService_Delete_DBError(t *testing.T) {
	t.Parallel()
	db := openWhitelistTestDB(t)
	svc := services.NewCrowdSecWhitelistService(db, "")
	sqlDB, err := db.DB()
	require.NoError(t, err)
	_ = sqlDB.Close()

	err = svc.Delete(context.Background(), "some-uuid")
	assert.Error(t, err)
	assert.NotErrorIs(t, err, services.ErrWhitelistNotFound)
}

func TestCrowdSecWhitelistService_WriteYAML_DBError(t *testing.T) {
	t.Parallel()
	db := openWhitelistTestDB(t)
	tmpDir := t.TempDir()
	svc := services.NewCrowdSecWhitelistService(db, tmpDir)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	_ = sqlDB.Close()

	err = svc.WriteYAML(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "query entries")
}

func TestCrowdSecWhitelistService_WriteYAML_MkdirError(t *testing.T) {
	t.Parallel()
	db := openWhitelistTestDB(t)
	// Populate the DB so WriteYAML proceeds past the empty-list early return.
	svcSetup := services.NewCrowdSecWhitelistService(db, "")
	_, err := svcSetup.Add(context.Background(), "1.2.3.4", "trigger mkdir path")
	require.NoError(t, err)

	// Use a path under /dev/null which cannot have subdirectories.
	svc := services.NewCrowdSecWhitelistService(db, "/dev/null/impossible")

	err = svc.WriteYAML(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "create dir")
}

func TestCrowdSecWhitelistService_WriteYAML_WriteFileError(t *testing.T) {
	t.Parallel()
	db := openWhitelistTestDB(t)
	tmpDir := t.TempDir()

	// Populate the DB so WriteYAML proceeds past the empty-list early return.
	svcSetup := services.NewCrowdSecWhitelistService(db, "")
	_, err := svcSetup.Add(context.Background(), "1.2.3.4", "trigger write path")
	require.NoError(t, err)

	svc := services.NewCrowdSecWhitelistService(db, tmpDir)

	// An empty directory at the target cannot be replaced by the rename, so the write fails.
	dir := filepath.Join(tmpDir, "config", "parsers", "s02-enrich")
	require.NoError(t, os.MkdirAll(dir, 0o750))
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "charon-whitelist.yaml"), 0o750))

	err = svc.WriteYAML(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "write whitelist yaml: write file")
}

func TestCrowdSecWhitelistService_Add_WriteYAMLWarning(t *testing.T) {
	t.Parallel()
	db := openWhitelistTestDB(t)
	// dataDir that will cause MkdirAll to fail inside WriteYAML (non-fatal)
	svc := services.NewCrowdSecWhitelistService(db, "/dev/null/impossible")

	entry, err := svc.Add(context.Background(), "2.2.2.2", "yaml warn test")
	require.NoError(t, err)
	assert.Equal(t, "2.2.2.2", entry.IPOrCIDR)
}

func TestCrowdSecWhitelistService_Delete_WriteYAMLWarning(t *testing.T) {
	t.Parallel()
	db := openWhitelistTestDB(t)
	// First add with empty dataDir so it succeeds
	svcAdd := services.NewCrowdSecWhitelistService(db, "")
	entry, err := svcAdd.Add(context.Background(), "3.3.3.3", "to delete")
	require.NoError(t, err)

	// Now create a service with a broken dataDir and delete
	svcDel := services.NewCrowdSecWhitelistService(db, "/dev/null/impossible")
	err = svcDel.Delete(context.Background(), entry.UUID)
	require.NoError(t, err)
}

func TestCrowdSecWhitelistService_WriteYAML_RenameError(t *testing.T) {
	t.Parallel()
	db := openWhitelistTestDB(t)
	tmpDir := t.TempDir()

	// Populate the DB so WriteYAML proceeds past the empty-list early return.
	svcSetup := services.NewCrowdSecWhitelistService(db, "")
	_, err := svcSetup.Add(context.Background(), "1.2.3.4", "trigger rename path")
	require.NoError(t, err)

	svc := services.NewCrowdSecWhitelistService(db, tmpDir)

	// A non-empty directory at the target cannot be replaced by the rename, so the write fails.
	dir := filepath.Join(tmpDir, "config", "parsers", "s02-enrich")
	require.NoError(t, os.MkdirAll(dir, 0o750))
	target := filepath.Join(dir, "charon-whitelist.yaml")
	require.NoError(t, os.MkdirAll(target, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(target, "occupant"), []byte("x"), 0o600))

	err = svc.WriteYAML(context.Background())
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "write whitelist yaml: write file")
}

func TestCrowdSecWhitelistService_Add_InvalidCIDR(t *testing.T) {
	t.Parallel()
	svc := services.NewCrowdSecWhitelistService(openWhitelistTestDB(t), "")
	_, err := svc.Add(context.Background(), "not-an-ip/24", "invalid cidr with slash")
	assert.ErrorIs(t, err, services.ErrInvalidIPOrCIDR)
}
