package database

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestQuickCheckStatus_UnknownPath(t *testing.T) {
	done, result := QuickCheckStatus(filepath.Join(t.TempDir(), "never-connected.db"))
	assert.Nil(t, done)
	assert.Nil(t, result)
}

// Both launchers (the synchronous test one and the default goroutine) must
// complete the registry entry that Connect creates.
func TestConnect_QuickCheckStatusCompletes(t *testing.T) {
	launchers := map[string]func(string){
		"synchronous": runQuickCheck,
		"async":       func(dbPath string) { go runQuickCheck(dbPath) },
	}
	for name, launcher := range launchers {
		t.Run(name, func(t *testing.T) {
			orig := launchQuickCheck
			t.Cleanup(func() { launchQuickCheck = orig })
			launchQuickCheck = launcher

			path := filepath.Join(t.TempDir(), "qc.db")
			db, err := Connect(path)
			require.NoError(t, err)
			sqlDB, err := db.DB()
			require.NoError(t, err)

			done, result := QuickCheckStatus(path)
			require.NotNil(t, done)
			select {
			case <-done:
			case <-time.After(10 * time.Second):
				t.Fatal("quick_check did not complete")
			}
			assert.Equal(t, QuickCheckOK, result())
			require.NoError(t, sqlDB.Close())
		})
	}
}

func TestQuickCheckStatus_PathIsCleaned(t *testing.T) {
	dir := t.TempDir()
	orig := launchQuickCheck
	t.Cleanup(func() { launchQuickCheck = orig })
	launchQuickCheck = func(string) {} // never completes

	db, err := Connect(filepath.Join(dir, "sub", "..", "clean.db"))
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })

	done, result := QuickCheckStatus(filepath.Join(dir, "clean.db"))
	require.NotNil(t, done)
	select {
	case <-done:
		t.Fatal("entry must still be pending")
	default:
	}
	assert.Empty(t, result(), "no result before completion")
}

func TestRunQuickCheck_ReportsCorruption(t *testing.T) {
	path := filepath.Join(t.TempDir(), "bad.db")
	db, err := Connect(path)
	require.NoError(t, err)
	require.NoError(t, db.Exec("CREATE TABLE t (id INTEGER PRIMARY KEY, v TEXT)").Error)
	require.NoError(t, db.Exec("CREATE INDEX idx_t_v ON t(v)").Error)
	for i := 0; i < 200; i++ {
		require.NoError(t, db.Exec("INSERT INTO t(v) VALUES (?)", "value-"+string(rune('a'+i%26))).Error)
	}
	sqlDB, err := db.DB()
	require.NoError(t, err)
	_, err = sqlDB.Exec("PRAGMA wal_checkpoint(TRUNCATE)")
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	// Smash a page in the middle of the file.
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	require.NoError(t, err)
	_, err = f.WriteAt(make([]byte, 4096), 4096*2)
	require.NoError(t, err)
	require.NoError(t, f.Close())

	registerQuickCheck(path)
	runQuickCheck(path)

	done, result := QuickCheckStatus(path)
	<-done
	assert.NotEmpty(t, result())
	assert.NotEqual(t, QuickCheckOK, result())
}

func TestRunQuickCheck_UnopenableDatabaseStillCompletes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "missing-dir", "x.db")
	registerQuickCheck(path)

	runQuickCheck(path)

	done, result := QuickCheckStatus(path)
	select {
	case <-done:
	default:
		t.Fatal("entry must complete on every exit path")
	}
	assert.Empty(t, result(), "a check that could not run has no verdict")
}

func TestRunQuickCheck_WithoutRegistrationIsHarmless(t *testing.T) {
	runQuickCheck(filepath.Join(t.TempDir(), "unregistered.db"))
}
