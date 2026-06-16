// Package database handles database connections and migrations.
package database

import (
	"database/sql"
	"fmt"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

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

	// Set SQLite performance pragmas via SQL execution
	// This is required for modernc.org/sqlite (pure-Go driver) which doesn't
	// support DSN-based pragma parameters like mattn/go-sqlite3
	pragmas := []string{
		"PRAGMA journal_mode=WAL",   // Better concurrent access, faster writes
		"PRAGMA busy_timeout=5000",  // Wait up to 5s instead of failing immediately on lock
		"PRAGMA synchronous=NORMAL", // Good balance of safety and speed
		"PRAGMA cache_size=-64000",  // 64MB cache for better performance
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
	go runQuickCheck(dbPath)

	return db, nil
}

// runQuickCheck opens a dedicated connection and runs PRAGMA quick_check,
// logging the result. It uses its own connection (rather than the shared
// pool, which is capped at one) so the scan - which can take well over a
// minute on larger databases - never blocks startup or migrations.
func runQuickCheck(dbPath string) {
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
	} else if quickCheckResult == "ok" {
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
