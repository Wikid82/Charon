package routes

import (
	"fmt"

	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

// MigrateSchema creates or updates every table and index the server uses. It is the single model list shared by
// the server boot (before the CrowdSec startup check reads the schema), RegisterWithDeps, and the `charon migrate`
// command, so the three cannot drift apart.
//
// It is idempotent: on an up-to-date schema it only inspects the tables and issues no DDL, which the
// migrate_idempotent tests pin for both empty and previous-release databases.
func MigrateSchema(db *gorm.DB) error {
	if err := db.AutoMigrate(
		&models.ProxyGroup{}, // must precede ProxyHost (FK dependency)
		&models.ProxyHost{},
		&models.Location{},
		&models.RedirectionHost{}, // Issue #1367: 301/302/307/308 redirect hosts (peer to ProxyHost)
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
		&models.WebPushSubscription{}, // Web Push subscriptions — FK to NotificationProvider (Type="webpush")
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
		&models.UserPermittedHost{}, // Join table for user permissions
		&models.CrowdsecPresetEvent{},
		&models.CrowdsecConsoleEnrollment{},
		&models.CrowdsecApplyJob{},
		&models.EmergencyToken{}, // Database-backed emergency tokens (the server's token API depends on this table)
		&models.DNSProvider{},
		&models.DNSProviderCredential{}, // Multi-credential support (Phase 3)
		&models.Plugin{},                // Phase 5: DNS provider plugins
		&models.ManualChallenge{},       // Phase 1: Manual DNS challenges
		&models.CrowdSecWhitelist{},     // Issue #939: CrowdSec IP whitelist management
		&models.TunnelConfig{},          // Issue #368: Hecate tunnel provider configs
		&models.OrthrusAgent{},          // Issue #369: Orthrus reverse-proxy agent registry
		&models.RequestLog{},            // Issue #25: Enhanced dashboard statistics
		&models.CustomTheme{},           // User-created named color-scheme themes
		&models.RemoteStorageTarget{},   // Issue #32: S3/SFTP remote backup targets
		&models.BackupRecord{},          // Issue #32: backup history (must precede BackupRemoteCopy FK)
		&models.BackupRemoteCopy{},      // Issue #32: per-target remote upload status
		&models.BackupJob{},             // Async Backup/Restore Jobs: tracks in-flight create/restore jobs
	); err != nil {
		return fmt.Errorf("auto migrate: %w", err)
	}

	// Enforce the Web Push provider singleton invariant at the database level — a service-layer COUNT-then-INSERT
	// check alone is not atomic under concurrent requests (see docs/plans/current_spec.md §3.1).
	// IF NOT EXISTS makes this idempotent across restarts.
	if err := db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_webpush_singleton
		ON notification_providers(type) WHERE type = 'webpush'`).Error; err != nil {
		return fmt.Errorf("create webpush singleton index: %w", err)
	}

	return nil
}
