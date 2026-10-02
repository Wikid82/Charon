// Package database handles database connections and migrations.
package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// launchQuickCheck is called by Connect to run the integrity check goroutine.
// Tests override this with a synchronous version to avoid cleanup races.
// journalSizeLimitBytes caps how large a write-ahead log may stay after a burst
// (boot-time index builds, imports, bulk deletes): SQLite trims a larger WAL to
// this size at the next WAL reset after a checkpoint. Steady-state WALs are far
// smaller and are never touched.
const journalSizeLimitBytes = 64 << 20

var launchQuickCheck = func(dbPath string) { go runQuickCheck(dbPath) }

// SyncIntegrityCheckForTesting forces the background integrity check that
// Connect launches (see launchQuickCheck) to run synchronously instead of
// in a goroutine. Callers in other packages' test suites that call Connect
// against paths under t.TempDir() should invoke this once, from a TestMain,
// mirroring this package's own TestMain in database_test.go:
//
//	func TestMain(m *testing.M) {
//	    database.SyncIntegrityCheckForTesting()
//	    os.Exit(m.Run())
//	}
//
// Without this, Connect's background integrity-check connection can still be
// reading/writing the SQLite WAL/SHM files when a caller's t.TempDir()
// cleanup (os.RemoveAll) runs after the test returns, which surfaces as an
// intermittent "TempDir RemoveAll cleanup: ... directory not empty" failure.
//
// There is no restore function: Go test binaries are single-process,
// one-shot invocations (the process exits after m.Run()), so there is
// nothing to revert before exit — the same reasoning internal/database's
// own TestMain already relies on.
//
// Production code paths are unaffected: Connect's default behavior (async
// integrity check) is unchanged unless a test explicitly opts in by calling
// this function.
func SyncIntegrityCheckForTesting() {
	launchQuickCheck = runQuickCheck
}

// Connect opens a SQLite database connection with optimized settings.
// Uses WAL mode for better concurrent read/write performance.
func Connect(dbPath string) (*gorm.DB, error) {
	// Open the database connection
	// Note: PRAGMA settings are applied after connection for modernc.org/sqlite compatibility
	db, err := gorm.Open(sqlite.Open(dbPath), &gorm.Config{
		// Skip default transaction for single operations (faster)
		SkipDefaultTransaction: true,
		// Prepare statements for reuse
		PrepareStmt: true,
		// Many lookups (e.g. optional settings) expect a missing row as a
		// normal outcome and already handle it; don't log those as errors.
		Logger: gormlogger.New(log.New(os.Stdout, "\r\n", log.LstdFlags), gormlogger.Config{
			SlowThreshold:             200 * time.Millisecond,
			LogLevel:                  gormlogger.Warn,
			IgnoreRecordNotFoundError: true,
		}),
	})
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	// Configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("get underlying db: %w", err)
	}
	configurePool(sqlDB)

	// A brand-new database is created in incremental auto-vacuum mode so freed
	// pages can be returned to the OS (GH #1422). The mode can only be chosen
	// while the file is empty and must be set before journal_mode=WAL.
	if err := enableIncrementalVacuumIfEmpty(sqlDB); err != nil {
		return nil, err
	}

	// Set SQLite performance pragmas via SQL execution
	// This is required for modernc.org/sqlite (pure-Go driver) which doesn't
	// support DSN-based pragma parameters like mattn/go-sqlite3
	pragmas := []string{
		"PRAGMA journal_mode=WAL",   // Better concurrent access, faster writes
		"PRAGMA busy_timeout=5000",  // Wait up to 5s instead of failing immediately on lock
		"PRAGMA synchronous=NORMAL", // Good balance of safety and speed
		"PRAGMA cache_size=-64000",  // 64MB cache for better performance
		// Trim a WAL that a burst grew past the limit at the next WAL reset
		fmt.Sprintf("PRAGMA journal_size_limit=%d", journalSizeLimitBytes),
	}
	for _, pragma := range pragmas {
		if _, err := sqlDB.Exec(pragma); err != nil {
			return nil, fmt.Errorf("failed to execute %s: %w", pragma, err)
		}
	}

	// Verify WAL mode is enabled and log confirmation
	var journalMode string
	if err := db.Raw("PRAGMA journal_mode").Scan(&journalMode).Error; err != nil {
		logger.Log().WithError(err).Warn("Failed to verify SQLite journal mode")
	} else {
		logger.Log().WithField("journal_mode", journalMode).Info("SQLite database connected with optimized settings")
	}

	// Run quick integrity check on startup in the background (warn-only), on
	// its own connection. The main pool is capped at one connection, so
	// sharing it here would still serialize migrations behind the check.
	registerQuickCheck(dbPath)
	launchQuickCheck(dbPath)

	return db, nil
}

// enableIncrementalVacuumIfEmpty issues PRAGMA auto_vacuum=2 when the database
// has no pages and no schema. On a populated file the pragma is a silent no-op
// until a VACUUM, so populated databases are left alone (the maintenance
// package converts them at boot).
func enableIncrementalVacuumIfEmpty(sqlDB *sql.DB) error {
	var pageCount, objects int64
	if err := sqlDB.QueryRow("PRAGMA page_count").Scan(&pageCount); err != nil {
		return fmt.Errorf("read page_count: %w", err)
	}
	if err := sqlDB.QueryRow("SELECT count(*) FROM sqlite_master").Scan(&objects); err != nil {
		return fmt.Errorf("read sqlite_master: %w", err)
	}
	if pageCount != 0 || objects != 0 {
		return nil
	}
	if _, err := sqlDB.Exec("PRAGMA auto_vacuum=2"); err != nil {
		return fmt.Errorf("enable incremental auto_vacuum: %w", err)
	}
	return nil
}

// runQuickCheck opens a dedicated connection and runs PRAGMA quick_check,
// logging the result. It uses its own connection (rather than the shared
// pool, which is capped at one) so the scan - which can take well over a
// minute on larger databases - never blocks startup or migrations.
func runQuickCheck(dbPath string) {
	verdict := ""
	defer func() { completeQuickCheck(dbPath, verdict) }()

	checkDB, err := sql.Open(sqlite.DriverName, dbPath)
	if err != nil {
		logger.Log().WithError(err).Warn("Failed to open SQLite connection for integrity check")
		return
	}
	defer func() {
		if cerr := checkDB.Close(); cerr != nil {
			logger.Log().WithError(cerr).Warn("Failed to close SQLite integrity check connection")
		}
	}()

	var quickCheckResult string
	if err := checkDB.QueryRow("PRAGMA quick_check").Scan(&quickCheckResult); err != nil {
		logger.Log().WithError(err).Warn("Failed to run SQLite integrity check on startup")
		return
	}
	verdict = quickCheckResult
	if quickCheckResult == QuickCheckOK {
		logger.Log().Info("SQLite database integrity check passed")
	} else {
		// Database has corruption - log error but don't fail startup
		logger.Log().WithField("quick_check_result", quickCheckResult).
			WithField("error_type", "database_corruption").
			Error("SQLite database integrity check failed - database may be corrupted")
	}
}

// configurePool sets connection pool settings for SQLite.
// SQLite handles concurrency differently than server databases,
// so we use conservative settings.
func configurePool(sqlDB *sql.DB) {
	// SQLite is file-based, so we limit connections
	// but keep some idle for reuse
	sqlDB.SetMaxOpenConns(1)    // SQLite only allows one writer at a time
	sqlDB.SetMaxIdleConns(1)    // Keep one connection ready
	sqlDB.SetConnMaxLifetime(0) // Don't close idle connections
}
