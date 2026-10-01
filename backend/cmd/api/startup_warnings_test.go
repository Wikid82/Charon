package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/sirupsen/logrus"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLogStartupWarnings(t *testing.T) {
	base, hook := logtest.NewNullLogger()
	logStartupWarnings(logrus.NewEntry(base), []string{"first problem", "second problem"})

	entries := hook.AllEntries()
	require.Len(t, entries, 2)
	for i, want := range []string{"first problem", "second problem"} {
		assert.Equal(t, logrus.WarnLevel, entries[i].Level)
		assert.Equal(t, want, entries[i].Message)
		assert.Equal(t, "config", entries[i].Data["component"])
	}

	hook.Reset()
	logStartupWarnings(logrus.NewEntry(base), nil)
	assert.Empty(t, hook.AllEntries())
}

func TestLoadConfigForDatabase_PreparesSQLiteTempDir(t *testing.T) {
	dataDir := t.TempDir()
	t.Setenv("CHARON_DB_PATH", filepath.Join(dataDir, "charon.db"))
	t.Setenv("CHARON_CADDY_CONFIG_DIR", filepath.Join(dataDir, "caddy"))
	t.Setenv("CHARON_IMPORT_DIR", filepath.Join(dataDir, "imports"))
	t.Setenv("SQLITE_TMPDIR", "")

	cfg, err := loadConfigForDatabase()

	require.NoError(t, err)
	assert.Equal(t, filepath.Join(dataDir, "charon.db"), cfg.DatabasePath)
	assert.Equal(t, filepath.Join(dataDir, ".tmp"), os.Getenv("SQLITE_TMPDIR"))
}

func TestLoadConfigForDatabase_KeepsOperatorTempDir(t *testing.T) {
	dataDir := t.TempDir()
	operator := t.TempDir()
	t.Setenv("CHARON_DB_PATH", filepath.Join(dataDir, "charon.db"))
	t.Setenv("CHARON_CADDY_CONFIG_DIR", filepath.Join(dataDir, "caddy"))
	t.Setenv("CHARON_IMPORT_DIR", filepath.Join(dataDir, "imports"))
	t.Setenv("SQLITE_TMPDIR", operator)

	_, err := loadConfigForDatabase()

	require.NoError(t, err)
	assert.Equal(t, operator, os.Getenv("SQLITE_TMPDIR"))
}

func TestLoadConfigForDatabase_ConfigErrorIsReturned(t *testing.T) {
	t.Setenv("CHARON_CADDY_ADMIN_API", "http://evil.example.com:2019")

	_, err := loadConfigForDatabase()

	require.Error(t, err)
}
