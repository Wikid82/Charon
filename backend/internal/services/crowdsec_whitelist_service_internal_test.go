package services

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"

	"github.com/Wikid82/charon/backend/internal/crowdsec"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// The tests in this file swap the writeWhitelistFile seam, so they must not run in parallel.

func openWhitelistSeamDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: gormlogger.Default.LogMode(gormlogger.Silent),
	})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.CrowdSecWhitelist{}))
	return db
}

// stubWhitelistWriter replaces the writeWhitelistFile seam for the duration of the test.
func stubWhitelistWriter(t *testing.T, fn func(string, []byte, fs.FileMode, crowdsec.AtomicWriteOptions) (crowdsec.AtomicWriteResult, error)) {
	t.Helper()
	orig := writeWhitelistFile
	writeWhitelistFile = fn
	t.Cleanup(func() { writeWhitelistFile = orig })
}

func TestWhitelistWriteGoesThroughAtomicHelperWithSafeOptions(t *testing.T) {
	dataDir := t.TempDir()
	var calls int
	var gotPath string
	var gotData []byte
	var gotMode fs.FileMode
	var gotOpts crowdsec.AtomicWriteOptions
	stubWhitelistWriter(t, func(path string, data []byte, mode fs.FileMode, opts crowdsec.AtomicWriteOptions) (crowdsec.AtomicWriteResult, error) {
		calls++
		gotPath, gotData, gotMode, gotOpts = path, data, mode, opts
		return crowdsec.AtomicWriteResult{}, nil
	})
	svc := NewCrowdSecWhitelistService(openWhitelistSeamDB(t), dataDir)

	_, err := svc.Add(context.Background(), "1.2.3.4", "")
	require.NoError(t, err)

	require.Equal(t, 1, calls)
	require.Equal(t, filepath.Join(dataDir, "config", "parsers", "s02-enrich", "charon-whitelist.yaml"), gotPath)
	require.Equal(t, fs.FileMode(0o640), gotMode)
	require.Contains(t, string(gotData), "    - \"1.2.3.4\"")
	require.True(t, gotOpts.KeepOwner, "the live file's owner is kept, as for the acquisition file")
	require.True(t, gotOpts.InPlaceFallback, "a refused rename falls back to an in-place write")
	require.True(t, gotOpts.NoFollow, "a symlink at the live path is never written through")
	require.Equal(t, whitelistStaleTempAge, gotOpts.SweepTempsOlderThan)
	require.Empty(t, gotOpts.TempPattern, "the default <base>.tmp-* pattern is used")
}

func TestWhitelistWriteErrorFromHelperIsWrapped(t *testing.T) {
	dataDir := t.TempDir()
	sentinel := errors.New("disk refused")
	stubWhitelistWriter(t, func(string, []byte, fs.FileMode, crowdsec.AtomicWriteOptions) (crowdsec.AtomicWriteResult, error) {
		return crowdsec.AtomicWriteResult{}, sentinel
	})
	svc := NewCrowdSecWhitelistService(openWhitelistSeamDB(t), dataDir)
	_, err := svc.Add(context.Background(), "5.6.7.8", "")
	require.NoError(t, err, "Add stays successful when the file write fails; the failure is logged")

	err = svc.WriteYAML(context.Background())
	require.ErrorIs(t, err, sentinel)
	require.ErrorContains(t, err, "write whitelist yaml: write file")
}

// stubWhitelistRemoval replaces the removal seam for the duration of the test.
func stubWhitelistRemoval(t *testing.T, fn func(string) error) {
	t.Helper()
	orig := removeWhitelistPath
	removeWhitelistPath = fn
	t.Cleanup(func() { removeWhitelistPath = orig })
}

func TestWhitelistEmptyListRemoveFailureIsReturned(t *testing.T) {
	dataDir := t.TempDir()
	sentinel := errors.New("remove refused")
	stubWhitelistRemoval(t, func(string) error { return sentinel })
	svc := NewCrowdSecWhitelistService(openWhitelistSeamDB(t), dataDir)

	err := svc.WriteYAML(context.Background())
	require.ErrorIs(t, err, sentinel)
	require.ErrorContains(t, err, "write whitelist yaml: remove empty")
}

func TestWhitelistEmptyListMissingFileIsNotAnError(t *testing.T) {
	dataDir := t.TempDir()
	stubWhitelistRemoval(t, func(path string) error { return &fs.PathError{Op: "remove", Path: path, Err: fs.ErrNotExist} })
	svc := NewCrowdSecWhitelistService(openWhitelistSeamDB(t), dataDir)

	require.NoError(t, svc.WriteYAML(context.Background()))
}

func TestWhitelistEmptyListDirectorySyncFailureIsNotFatal(t *testing.T) {
	dataDir := t.TempDir()
	dir := filepath.Join(dataDir, "config", "parsers", "s02-enrich")
	require.NoError(t, os.MkdirAll(dir, 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "charon-whitelist.yaml"), []byte("stale\n"), 0o600))
	orig := syncWhitelistDir
	syncWhitelistDir = func(string) error { return errors.New("sync refused") }
	t.Cleanup(func() { syncWhitelistDir = orig })
	svc := NewCrowdSecWhitelistService(openWhitelistSeamDB(t), dataDir)

	require.NoError(t, svc.WriteYAML(context.Background()), "a failed directory sync is logged, not returned")
	require.NoFileExists(t, filepath.Join(dir, "charon-whitelist.yaml"))
}

func TestWhitelistLegacyTempRemoveFailureIsNotFatal(t *testing.T) {
	dataDir := t.TempDir()
	dir := filepath.Join(dataDir, "config", "parsers", "s02-enrich")
	require.NoError(t, os.MkdirAll(dir, 0o750))
	legacy := filepath.Join(dir, "charon-whitelist.yaml.tmp")
	require.NoError(t, os.WriteFile(legacy, []byte("stale\n"), 0o600))
	stubWhitelistRemoval(t, func(string) error { return errors.New("remove refused") })
	svc := NewCrowdSecWhitelistService(openWhitelistSeamDB(t), dataDir)
	_, err := svc.Add(context.Background(), "4.3.2.1", "")
	require.NoError(t, err)

	require.NoError(t, svc.WriteYAML(context.Background()), "a legacy temp that cannot be removed does not fail the write")
	require.FileExists(t, legacy)
}
