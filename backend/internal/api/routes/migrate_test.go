package routes

import (
	"bytes"
	"context"
	"io"
	"path/filepath"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

// openFileDatabase opens a fresh SQLite file database with the same driver the server uses.
func openFileDatabase(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "charon.db")), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	return db
}

// TestMigrateSchema_CreatesEveryModelTable pins the full table list, so a model that drops out of the shared list
// fails here instead of silently on a fresh install.
func TestMigrateSchema_CreatesEveryModelTable(t *testing.T) {
	db := openFileDatabase(t)

	require.NoError(t, MigrateSchema(db))

	expected := []any{
		&models.ProxyGroup{},
		&models.ProxyHost{},
		&models.Location{},
		&models.RedirectionHost{},
		&models.CaddyConfig{},
		&models.RemoteServer{},
		&models.SSLCertificate{},
		&models.AccessList{},
		&models.SecurityHeaderProfile{},
		&models.User{},
		&models.Setting{},
		&models.ImportSession{},
		&models.Notification{},
		&models.NotificationProvider{},
		&models.WebPushSubscription{},
		&models.NotificationTemplate{},
		&models.NotificationConfig{},
		&models.UptimeMonitor{},
		&models.UptimeHeartbeat{},
		&models.UptimeHost{},
		&models.UptimeNotificationEvent{},
		&models.Domain{},
		&models.SecurityConfig{},
		&models.SecurityDecision{},
		&models.SecurityAudit{},
		&models.SecurityRuleSet{},
		&models.UserPermittedHost{},
		&models.CrowdsecPresetEvent{},
		&models.CrowdsecConsoleEnrollment{},
		&models.CrowdsecApplyJob{},
		&models.EmergencyToken{},
		&models.DNSProvider{},
		&models.DNSProviderCredential{},
		&models.Plugin{},
		&models.ManualChallenge{},
		&models.CrowdSecWhitelist{},
		&models.TunnelConfig{},
		&models.OrthrusAgent{},
		&models.RequestLog{},
		&models.CustomTheme{},
		&models.RemoteStorageTarget{},
		&models.BackupRecord{},
		&models.BackupRemoteCopy{},
		&models.BackupJob{},
	}
	for _, model := range expected {
		assert.Truef(t, db.Migrator().HasTable(model), "MigrateSchema must create the table for %T", model)
	}
	assert.True(t, db.Migrator().HasIndex(&models.NotificationProvider{}, "idx_webpush_singleton"),
		"MigrateSchema must create the Web Push singleton index")
}

// TestMigrateSchema_EmergencyTokenGenerateWorksOnFreshDatabase is the user-visible result of the emergency token
// table being in the shared list: before this change Generate failed with "no such table" on a server-only database.
func TestMigrateSchema_EmergencyTokenGenerateWorksOnFreshDatabase(t *testing.T) {
	db := openFileDatabase(t)
	require.NoError(t, MigrateSchema(db))

	resp, err := services.NewEmergencyTokenService(db).Generate(services.GenerateRequest{ExpirationDays: 30})

	require.NoError(t, err)
	assert.NotEmpty(t, resp.Token)
}

// TestMigrateSchema_SecondCallOnEmptyDatabaseIsNoOp covers the second pass that RegisterWithDeps runs on the schema
// the first pass already made.
func TestMigrateSchema_SecondCallOnEmptyDatabaseIsNoOp(t *testing.T) {
	db := openFileDatabase(t)
	require.NoError(t, MigrateSchema(db))

	require.NoError(t, MigrateSchema(db))
	assert.True(t, db.Migrator().HasTable(&models.SecurityConfig{}))
}

// TestMigrateSchema_ReconciliationAfterMigrationLogsNoMissingTable pins the boot order in main: the schema exists
// before the CrowdSec startup check reads it, so the check never reports a missing table on a fresh database.
func TestMigrateSchema_ReconciliationAfterMigrationLogsNoMissingTable(t *testing.T) {
	logs := &bytes.Buffer{}
	logger.Init(false, logs)
	t.Cleanup(func() { logger.Init(false, io.Discard) })
	db := openFileDatabase(t)
	require.NoError(t, MigrateSchema(db))

	services.ReconcileCrowdSecOnStartup(db, &noopCrowdsecProcess{}, "", t.TempDir(), nil)

	assert.NotContains(t, logs.String(), "table not found")
	assert.NotContains(t, logs.String(), "SecurityConfig table is missing")
	var rows int64
	require.NoError(t, db.Model(&models.SecurityConfig{}).Count(&rows).Error)
	assert.Equal(t, int64(1), rows, "the reconciliation seeds the default row on a fresh database")
}

// noopCrowdsecProcess is a CrowdsecProcessManager that never starts anything.
type noopCrowdsecProcess struct{}

func (noopCrowdsecProcess) Start(context.Context, string, string) (int, error) { return 0, nil }
func (noopCrowdsecProcess) Stop(context.Context, string) error                 { return nil }
func (noopCrowdsecProcess) Status(context.Context, string) (running bool, pid int, err error) {
	return false, 0, nil
}
