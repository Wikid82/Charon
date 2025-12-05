package database

import (
	"database/sql"
	"fmt"
	"strings"

	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// Connect opens a SQLite database connection with optimized settings.
// Uses WAL mode for better concurrent read/write performance.
func Connect(dbPath string) (*gorm.DB, error) {
	// Add SQLite performance pragmas if not already present
	dsn := dbPath
	if !strings.Contains(dsn, "?") {
		dsn += "?"
	} else {
		dsn += "&"
	}
	// WAL mode: better concurrent access, faster writes
	// busy_timeout: wait up to 5s instead of failing immediately on lock
	// cache: shared cache for better memory usage
	// synchronous=NORMAL: good balance of safety and speed
	dsn += "_journal_mode=WAL&_busy_timeout=5000&_synchronous=NORMAL&_cache_size=-64000"

	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
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

	return db, nil
}

// configurePool sets connection pool settings for SQLite.
// SQLite handles concurrency differently than server databases,
// so we use conservative settings.
func configurePool(sqlDB *sql.DB) {
	// SQLite is file-based, so we limit connections
	// but keep some idle for reuse
	sqlDB.SetMaxOpenConns(1)   // SQLite only allows one writer at a time
	sqlDB.SetMaxIdleConns(1)   // Keep one connection ready
	sqlDB.SetConnMaxLifetime(0) // Don't close idle connections
}
