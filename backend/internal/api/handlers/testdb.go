package handlers

import (
	"sync"
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/testutil"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

var (
	templateDBOnce sync.Once
	templateDB     *gorm.DB
	templateErr    error
)

// initTemplateDB creates a pre-migrated database template (called once).
// This eliminates repeated AutoMigrate calls across tests.
func initTemplateDB() {
	templateDB, templateErr = gorm.Open(sqlite.Open(":memory:"), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if templateErr != nil {
		return
	}

	// Migrate ALL models once
	templateErr = templateDB.AutoMigrate(
		&models.User{},
		&models.ProxyHost{},
		&models.Location{},
		&models.RemoteServer{},
		&models.Notification{},
		&models.NotificationProvider{},
		&models.NotificationTemplate{},
		&models.NotificationConfig{},
		&models.Setting{},
		&models.SecurityConfig{},
		&models.SecurityDecision{},
		&models.SecurityAudit{},
		&models.SecurityRuleSet{},
		&models.SecurityHeaderProfile{},
		&models.SSLCertificate{},
		&models.AccessList{},
		&models.UptimeMonitor{},
		&models.UptimeHeartbeat{},
		&models.UptimeHost{},
		&models.UptimeNotificationEvent{},
		&models.ImportSession{},
		&models.CaddyConfig{},
		&models.Domain{},
		&models.CrowdsecConsoleEnrollment{},
		&models.CrowdsecApplyJob{},
		&models.Plugin{},
		&models.DNSProvider{},
		&models.DNSProviderCredential{},
	)
}

// GetTemplateDB returns the pre-migrated template database.
// Tests can use this to copy the schema instead of running AutoMigrate each time.
func GetTemplateDB() (*gorm.DB, error) {
	templateDBOnce.Do(initTemplateDB)
	return templateDB, templateErr
}

// Opens a SQLite in-memory DB unique per test and applies
// a busy timeout and WAL journal mode to reduce SQLITE locking during parallel tests.
func OpenTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := testutil.MemoryDSN(t, "_journal_mode=WAL", "_busy_timeout=5000")
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		t.Fatalf("failed to open test db: %v", err)
	}
	// Register cleanup to close database connection
	t.Cleanup(func() {
		sqlDB, err := db.DB()
		if err == nil {
			_ = sqlDB.Close()
		}
	})
	return db
}

// OpenTestDBWithMigrations creates a SQLite in-memory DB and runs AutoMigrate
// for all commonly used models. This is faster than individual test migrations
// because it uses the template database schema when available.
func OpenTestDBWithMigrations(t *testing.T) *gorm.DB {
	t.Helper()

	db := OpenTestDB(t)

	// Try to get template DB and copy schema
	if tmpl, err := GetTemplateDB(); err == nil && tmpl != nil {
		// Copy all table schemas from template
		// For SQLite, we can use the template's schema info
		rows, err := tmpl.Raw("SELECT sql FROM sqlite_master WHERE type='table' AND sql IS NOT NULL").Rows()
		if err == nil {
			defer func() {
				if closeErr := rows.Close(); closeErr != nil {
					t.Logf("warning: failed to close rows: %v", closeErr)
				}
			}()
			for rows.Next() {
				var sql string
				if rows.Scan(&sql) == nil && sql != "" {
					db.Exec(sql)
				}
			}
			return db
		}
	}

	// Fallback: run AutoMigrate directly if template not available
	if err := db.AutoMigrate(
		&models.User{},
		&models.ProxyHost{},
		&models.Location{},
		&models.RemoteServer{},
		&models.Notification{},
		&models.NotificationProvider{},
		&models.NotificationTemplate{},
		&models.NotificationConfig{},
		&models.Setting{},
		&models.SecurityConfig{},
		&models.SecurityDecision{},
		&models.SecurityAudit{},
		&models.SecurityRuleSet{},
		&models.SecurityHeaderProfile{},
		&models.SSLCertificate{},
		&models.AccessList{},
		&models.UptimeMonitor{},
		&models.UptimeHeartbeat{},
		&models.UptimeHost{},
		&models.UptimeNotificationEvent{},
		&models.ImportSession{},
		&models.CaddyConfig{},
		&models.Domain{},
		&models.CrowdsecConsoleEnrollment{},
		&models.CrowdsecApplyJob{},
		&models.Plugin{},
		&models.DNSProvider{},
		&models.DNSProviderCredential{},
	); err != nil {
		t.Fatalf("failed to migrate test db: %v", err)
	}

	return db
}
