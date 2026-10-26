package services_test

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// openWhitelistSerialDB opens an in-memory whitelist database limited to one connection. Each connection to
// ":memory:" is a separate database, so the concurrent tests need the single connection to share one schema.
func openWhitelistSerialDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&models.CrowdSecWhitelist{}))
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

// whitelistEntriesInFile returns the IP and CIDR entries listed in a whitelist YAML file, in file order.
func whitelistEntriesInFile(content string) []string {
	var entries []string
	for _, line := range strings.Split(content, "\n") {
		if value, ok := strings.CutPrefix(line, "    - \""); ok {
			entries = append(entries, strings.TrimSuffix(value, "\""))
		}
	}
	return entries
}

func TestWhitelistConcurrentAddsLeaveFileMatchingDB(t *testing.T) {
	t.Parallel()
	dataDir := t.TempDir()
	svc := services.NewCrowdSecWhitelistService(openWhitelistSerialDB(t), dataDir)
	ctx := context.Background()

	const adds = 20
	var wg sync.WaitGroup
	errs := make(chan error, adds)
	for i := 1; i <= adds; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, err := svc.Add(ctx, fmt.Sprintf("10.20.30.%d", i), "concurrent")
			errs <- err
		}(i)
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err)
	}

	entries, err := svc.List(ctx)
	require.NoError(t, err)
	require.Len(t, entries, adds)
	dbIPs := make([]string, 0, len(entries))
	for _, e := range entries {
		dbIPs = append(dbIPs, e.IPOrCIDR)
	}
	require.ElementsMatch(t, dbIPs, whitelistEntriesInFile(readWhitelistFile(t, dataDir)),
		"the file must list every committed entry, not one behind the last change")
}

func TestWhitelistWriteYAMLSucceedsWhenContextCancelledAfterCommit(t *testing.T) {
	t.Parallel()
	dataDir := t.TempDir()
	db := openWhitelistSerialDB(t)
	_, err := services.NewCrowdSecWhitelistService(db, "").Add(context.Background(), "8.8.8.8", "")
	require.NoError(t, err)

	// A client that disconnects after the commit cancels the request context before the file is written.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	require.NoError(t, services.NewCrowdSecWhitelistService(db, dataDir).WriteYAML(ctx))

	require.Equal(t, wantWhitelistYAML([]string{"8.8.8.8"}, nil), readWhitelistFile(t, dataDir))
}

func TestWhitelistAddCancelledAfterCommitStillWritesFile(t *testing.T) {
	t.Parallel()
	dataDir := t.TempDir()
	db := openWhitelistSerialDB(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Runs once the insert has committed, as a disconnect would: the row is kept and the write must follow.
	require.NoError(t, db.Callback().Create().After("gorm:commit_or_rollback_transaction").
		Register("test:cancel_after_commit", func(*gorm.DB) { cancel() }))
	svc := services.NewCrowdSecWhitelistService(db, dataDir)

	entry, err := svc.Add(ctx, "9.9.9.9", "disconnect")
	require.NoError(t, err)
	require.Equal(t, "9.9.9.9", entry.IPOrCIDR)
	require.Equal(t, wantWhitelistYAML([]string{"9.9.9.9"}, nil), readWhitelistFile(t, dataDir))
}

func TestWhitelistWriteKeepsDirectoryAtLegacyTempName(t *testing.T) {
	t.Parallel()
	dataDir := t.TempDir()
	dir := whitelistDir(dataDir)
	legacy := filepath.Join(dir, "charon-whitelist.yaml.tmp")
	require.NoError(t, os.MkdirAll(legacy, 0o750))
	svc := services.NewCrowdSecWhitelistService(openWhitelistSerialDB(t), dataDir)

	_, err := svc.Add(context.Background(), "7.7.7.7", "")
	require.NoError(t, err)

	require.Equal(t, wantWhitelistYAML([]string{"7.7.7.7"}, nil), readWhitelistFile(t, dataDir))
	require.DirExists(t, legacy, "only a regular file at the legacy temp name is removed")
}

func TestWhitelistEmptyListRemovesRegularLegacyTemp(t *testing.T) {
	t.Parallel()
	dataDir := t.TempDir()
	dir := whitelistDir(dataDir)
	require.NoError(t, os.MkdirAll(dir, 0o750))
	legacy := filepath.Join(dir, "charon-whitelist.yaml.tmp")
	require.NoError(t, os.WriteFile(legacy, []byte("stale\n"), 0o600))
	svc := services.NewCrowdSecWhitelistService(openWhitelistSerialDB(t), dataDir)

	require.NoError(t, svc.WriteYAML(context.Background()))

	require.NoFileExists(t, legacy)
}
