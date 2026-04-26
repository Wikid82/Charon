// Package routes defines the API route registration and wiring.
package routes

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/gin-contrib/gzip"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/api/handlers"
	"github.com/Wikid82/charon/backend/internal/api/middleware"
	"github.com/Wikid82/charon/backend/internal/caddy"
	"github.com/Wikid82/charon/backend/internal/cerberus"
	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/crypto"
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/metrics"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"

	// Import custom DNS providers to register them
	_ "github.com/Wikid82/charon/backend/pkg/dnsprovider/custom"
)

type uptimeBootstrapService interface {
	CleanupStaleFailureCounts() error
	SyncMonitors() error
	CheckAll()
}

func runInitialUptimeBootstrap(enabled bool, uptimeService uptimeBootstrapService, logWarn func(error, string), logError func(error, string)) {
	if !enabled {
		return
	}

	if err := uptimeService.CleanupStaleFailureCounts(); err != nil && logWarn != nil {
		logWarn(err, "Failed to cleanup stale failure counts")
	}

	if err := uptimeService.SyncMonitors(); err != nil && logError != nil {
		logError(err, "Failed to sync monitors")
	}

	// Run initial check immediately after sync to avoid the 90s blind window.
	uptimeService.CheckAll()
}

// migrateViewerToPassthrough renames any legacy "viewer" roles to "passthrough".
func migrateViewerToPassthrough(db *gorm.DB) {
	result := db.Model(&models.User{}).Where("role = ?", "viewer").Update("role", string(models.RolePassthrough))
	if result.RowsAffected > 0 {
		logger.Log().WithField("count", result.RowsAffected).Info("Migrated viewer roles to passthrough")
	}
}

// Register wires up API routes and performs automatic migrations.
func Register(ctx context.Context, router *gin.Engine, db *gorm.DB, cfg config.Config) error {
	// Caddy Manager - created early so it can be used by settings handlers for config reload
	caddyClient := caddy.NewClient(cfg.CaddyAdminAPI)
	caddyManager := caddy.NewManager(caddyClient, db, cfg.CaddyConfigDir, cfg.FrontendDir, cfg.ACMEStaging, cfg.Security)

	// Cerberus middleware applies the optional security suite checks (WAF, ACL, CrowdSec)
	cerb := cerberus.New(cfg.Security, db)

	return RegisterWithDeps(ctx, router, db, cfg, caddyManager, cerb)
}

// RegisterWithDeps wires up API routes and performs automatic migrations with prebuilt dependencies.
func RegisterWithDeps(ctx context.Context, router *gin.Engine, db *gorm.DB, cfg config.Config, caddyManager *caddy.Manager, cerb *cerberus.Cerberus) error {
	// Emergency bypass must be registered FIRST.
	// When a valid X-Emergency-Token is present from an authorized source,
	// it sets an emergency context flag and strips the token header so downstream
	// middleware (Cerberus/ACL/WAF/etc.) can honor the bypass without logging it.
	router.Use(middleware.EmergencyBypass(cfg.Security.ManagementCIDRs, db))

	// Enable gzip compression for API responses (reduces payload size ~70%)
	router.Use(gzip.Gzip(gzip.DefaultCompression))

	// Apply security headers middleware globally
	// This sets CSP, HSTS, X-Frame-Options, etc.
	securityHeadersCfg := middleware.SecurityHeadersConfig{
		IsDevelopment: cfg.Environment == "development",
	}
	router.Use(middleware.SecurityHeaders(securityHeadersCfg))

	// AutoMigrate all models for Issue #5 persistence layer
	if err := db.AutoMigrate(
		&models.ProxyHost{},
		&models.Location{},
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
		&models.DNSProvider{},
		&models.DNSProviderCredential{}, // Multi-credential support (Phase 3)
		&models.Plugin{},                // Phase 5: DNS provider plugins
		&models.ManualChallenge{},       // Phase 1: Manual DNS challenges
		&models.CrowdSecWhitelist{},     // Issue #939: CrowdSec IP whitelist management
		&models.TunnelConfig{},          // Issue #368: Hecate tunnel provider configs
		&models.OrthrusAgent{},          // Issue #369: Orthrus reverse-proxy agent registry
	); err != nil {
		return fmt.Errorf("auto migrate: %w", err)
	}

	migrateViewerToPassthrough(db)

	// Seed the default SecurityConfig row on every startup (idempotent).
	// Missing on fresh installs causes GetStatus to return all-disabled zero values.
	if _, err := models.SeedDefaultSecurityConfig(db); err != nil {
		logger.Log().WithError(err).Warn("Failed to seed default SecurityConfig — continuing startup")
	}

	// Let's Encrypt certs are auto-managed by Caddy and should not be assigned via certificate_id
	logger.Log().Info("Cleaning up invalid Let's Encrypt certificate associations...")
	var hostsWithInvalidCerts []models.ProxyHost
	if err := db.Joins("LEFT JOIN ssl_certificates ON proxy_hosts.certificate_id = ssl_certificates.id").
		Where("ssl_certificates.provider = ?", "letsencrypt").
		Find(&hostsWithInvalidCerts).Error; err == nil {
		if len(hostsWithInvalidCerts) > 0 {
			for _, host := range hostsWithInvalidCerts {
				logger.Log().WithField("domain", host.DomainNames).Info("Removing invalid Let's Encrypt cert assignment")
				db.Model(&host).Update("certificate_id", nil)
			}
		}
	}

	if caddyManager == nil {
		caddyClient := caddy.NewClient(cfg.CaddyAdminAPI)
		caddyManager = caddy.NewManager(caddyClient, db, cfg.CaddyConfigDir, cfg.FrontendDir, cfg.ACMEStaging, cfg.Security)
	}

	// Wire encryption service to Caddy manager for decrypting certificate private keys
	if cfg.EncryptionKey != "" {
		if svc, err := crypto.NewEncryptionService(cfg.EncryptionKey); err == nil {
			caddyManager.SetEncryptionService(svc)
		}
	}

	if cerb == nil {
		cerb = cerberus.New(cfg.Security, db)
	}

	router.GET("/api/v1/health", cerb.RateLimitMiddleware(), handlers.HealthHandler)

	// Metrics endpoint (Prometheus)
	reg := prometheus.NewRegistry()
	metrics.Register(reg)
	router.GET("/metrics", func(c *gin.Context) {
		promhttp.HandlerFor(reg, promhttp.HandlerOpts{}).ServeHTTP(c.Writer, c.Request)
	})

	// Emergency endpoint
	emergencyHandler := handlers.NewEmergencyHandlerWithDeps(db, caddyManager, cerb)
	emergency := router.Group("/api/v1/emergency")
	// Emergency endpoints must stay responsive and should not be rate limited.
	emergency.POST("/security-reset", emergencyHandler.SecurityReset)

	// Emergency token management (admin-only, protected by EmergencyBypass middleware)
	emergencyTokenService := services.NewEmergencyTokenService(db)
	emergencyTokenHandler := handlers.NewEmergencyTokenHandler(emergencyTokenService)
	emergency.POST("/token/generate", emergencyTokenHandler.GenerateToken)
	emergency.GET("/token/status", emergencyTokenHandler.GetTokenStatus)
	emergency.DELETE("/token", emergencyTokenHandler.RevokeToken)
	emergency.PATCH("/token/expiration", emergencyTokenHandler.UpdateTokenExpiration)

	// Auth routes
	authService := services.NewAuthService(db, cfg)
	authHandler := handlers.NewAuthHandlerWithDB(authService, db)
	authMiddleware := middleware.AuthMiddleware(authService)

	api := router.Group("/api/v1")
	api.Use(middleware.OptionalAuth(authService))
	// Rate Limiting (Emergency/Go-layer) runs after optional auth so authenticated
	// admin control-plane requests can be exempted safely.
	api.Use(cerb.RateLimitMiddleware())
	// Cerberus middleware (ACL, WAF Stats, CrowdSec Tracking) runs after Auth
	// because ACLs need to know if user is authenticated admin to apply whitelist bypass
	api.Use(cerb.Middleware())

	// Backup routes
	backupService := services.NewBackupService(&cfg)
	backupService.Start() // Start cron scheduler for scheduled backups
	securityService := services.NewSecurityService(db)
	backupHandler := handlers.NewBackupHandlerWithDeps(backupService, securityService, db)

	// DB Health endpoint (uses backup service for last backup time)
	dbHealthHandler := handlers.NewDBHealthHandler(db, backupService)
	router.GET("/api/v1/health/db", dbHealthHandler.Check)

	// Log routes
	logService := services.NewLogService(&cfg)
	logsHandler := handlers.NewLogsHandler(logService)

	// WebSocket tracker for connection monitoring
	wsTracker := services.NewWebSocketTracker()
	wsStatusHandler := handlers.NewWebSocketStatusHandler(wsTracker)

	// Notification Service (needed for multiple handlers)
	notificationService := services.NewNotificationService(db, services.NewMailService(db))

	// Ensure notify-only provider migration reconciliation at boot
	if err := notificationService.EnsureNotifyOnlyProviderMigration(context.Background()); err != nil {
		return fmt.Errorf("notify-only provider migration: %w", err)
	}

	// Remote Server Service (needed for Docker handler)
	remoteServerService := services.NewRemoteServerService(db)

	// Security Notification Handler - created early for runtime security event intake
	dataRoot := filepath.Dir(cfg.DatabasePath)
	enhancedSecurityNotificationService := services.NewEnhancedSecurityNotificationService(db)

	// Blocker 3: Invoke migration marker flow at boot with checksum rerun/no-op logic
	if err := enhancedSecurityNotificationService.MigrateFromLegacyConfig(); err != nil {
		logger.Log().WithError(err).Warn("Security notification migration: non-fatal error during boot-time reconciliation")
		// Non-blocking: migration failures are logged but don't prevent startup
	}

	securityNotificationHandler := handlers.NewSecurityNotificationHandlerWithDeps(
		enhancedSecurityNotificationService,
		securityService,
		dataRoot,
		notificationService,
		cfg.Security.ManagementCIDRs,
	)

	api.POST("/auth/login", authHandler.Login)
	api.POST("/auth/register", authHandler.Register)

	// Forward auth endpoint for Caddy (public, validates session internally)
	api.GET("/auth/verify", authHandler.Verify)
	api.GET("/auth/status", authHandler.VerifyStatus)

	// Runtime security event intake endpoint for Cerberus/Caddy bouncer
	// This endpoint receives security events (WAF blocks, CrowdSec decisions, etc.) from Caddy middleware
	// Accessible without user session auth (uses IP whitelist for Caddy/internal traffic)
	// Auth mechanism: Handler validates request originates from localhost or management CIDRs
	api.POST("/security/events", securityNotificationHandler.HandleSecurityEvent)

	// User handler (public endpoints)
	userHandler := handlers.NewUserHandler(db, authService)
	api.GET("/setup", userHandler.GetSetupStatus)
	api.POST("/setup", userHandler.Setup)
	api.GET("/invite/validate", userHandler.ValidateInvite)
	api.POST("/invite/accept", userHandler.AcceptInvite)

	// Uptime Service - define early so it can be used during route registration
	uptimeService := services.NewUptimeService(db, notificationService)

	protected := api.Group("/")
	protected.Use(authMiddleware)
	{
		// Self-service routes — accessible to all authenticated users including passthrough
		protected.POST("/auth/logout", authHandler.Logout)
		protected.POST("/auth/refresh", authHandler.Refresh)
		protected.GET("/auth/me", authHandler.Me)
		protected.POST("/auth/change-password", authHandler.ChangePassword)
		protected.GET("/auth/accessible-hosts", authHandler.GetAccessibleHosts)
		protected.GET("/auth/check-host/:hostId", authHandler.CheckHostAccess)
		protected.GET("/user/profile", userHandler.GetProfile)
		protected.POST("/user/profile", userHandler.UpdateProfile)
		protected.POST("/user/api-key", userHandler.RegenerateAPIKey)

		// Management routes — blocked for passthrough users
		management := protected.Group("/")
		management.Use(middleware.RequireManagementAccess())

		// Backups
		management.GET("/backups", backupHandler.List)
		management.POST("/backups", backupHandler.Create)
		management.DELETE("/backups/:filename", backupHandler.Delete)
		management.GET("/backups/:filename/download", backupHandler.Download)
		management.POST("/backups/:filename/restore", backupHandler.Restore)

		// Logs
		// WebSocket endpoints
		logsWSHandler := handlers.NewLogsWSHandler(wsTracker)
		management.GET("/logs/live", logsWSHandler.HandleWebSocket)
		management.GET("/logs", logsHandler.List)
		management.GET("/logs/:filename", logsHandler.Read)
		management.GET("/logs/:filename/download", logsHandler.Download)

		// WebSocket status monitoring
		management.GET("/websocket/connections", wsStatusHandler.GetConnections)
		management.GET("/websocket/stats", wsStatusHandler.GetStats)

		// Security Notification Settings - Use handler created earlier for event intake
		management.GET("/security/notifications/settings", securityNotificationHandler.DeprecatedGetSettings)
		management.PUT("/security/notifications/settings", securityNotificationHandler.DeprecatedUpdateSettings)
		management.GET("/notifications/settings/security", securityNotificationHandler.GetSettings)
		management.PUT("/notifications/settings/security", securityNotificationHandler.UpdateSettings)

		// System permissions diagnostics and repair
		systemPermissionsHandler := handlers.NewSystemPermissionsHandler(cfg, securityService, nil)
		management.GET("/system/permissions", systemPermissionsHandler.GetPermissions)
		management.POST("/system/permissions/repair", systemPermissionsHandler.RepairPermissions)

		// Audit Logs
		auditLogHandler := handlers.NewAuditLogHandler(securityService)
		management.GET("/audit-logs", auditLogHandler.List)
		management.GET("/audit-logs/:uuid", auditLogHandler.Get)

		// Settings - with CaddyManager and Cerberus for security settings reload
		settingsHandler := handlers.NewSettingsHandlerWithDeps(db, caddyManager, cerb, securityService, dataRoot)

		management.GET("/settings", settingsHandler.GetSettings)
		management.POST("/settings", settingsHandler.UpdateSetting)
		management.PATCH("/settings", settingsHandler.UpdateSetting) // E2E tests use PATCH
		management.PATCH("/config", settingsHandler.PatchConfig)     // Bulk configuration update

		// SMTP Configuration
		management.GET("/settings/smtp", middleware.RequireRole(models.RoleAdmin), settingsHandler.GetSMTPConfig)
		management.POST("/settings/smtp", settingsHandler.UpdateSMTPConfig)
		management.POST("/settings/smtp/test", settingsHandler.TestSMTPConfig)
		management.POST("/settings/smtp/test-email", settingsHandler.SendTestEmail)

		// URL Validation
		management.POST("/settings/validate-url", settingsHandler.ValidatePublicURL)
		management.POST("/settings/test-url", settingsHandler.TestPublicURL)

		// Feature flags (DB-backed with env fallback)
		featureFlagsHandler := handlers.NewFeatureFlagsHandler(db)
		management.GET("/feature-flags", featureFlagsHandler.GetFlags)
		management.PUT("/feature-flags", featureFlagsHandler.UpdateFlags)

		// User Management (admin only routes are in RegisterRoutes)
		management.GET("/users", userHandler.ListUsers)
		management.POST("/users", userHandler.CreateUser)
		management.POST("/users/invite", userHandler.InviteUser)
		management.POST("/users/preview-invite-url", userHandler.PreviewInviteURL)
		management.GET("/users/:id", userHandler.GetUser)
		management.PUT("/users/:id", userHandler.UpdateUser)
		management.DELETE("/users/:id", userHandler.DeleteUser)
		management.PUT("/users/:id/permissions", userHandler.UpdateUserPermissions)
		management.POST("/users/:id/resend-invite", userHandler.ResendInvite)

		// Updates
		updateService := services.NewUpdateService()
		updateHandler := handlers.NewUpdateHandler(updateService)
		management.GET("/system/updates", updateHandler.Check)

		// System info
		systemHandler := handlers.NewSystemHandler()
		management.GET("/system/my-ip", systemHandler.GetMyIP)

		// Notifications
		notificationHandler := handlers.NewNotificationHandler(notificationService)
		management.GET("/notifications", notificationHandler.List)
		management.POST("/notifications/:id/read", notificationHandler.MarkAsRead)
		management.POST("/notifications/read-all", notificationHandler.MarkAllAsRead)

		// Domains
		domainHandler := handlers.NewDomainHandler(db, notificationService)
		management.GET("/domains", domainHandler.List)
		management.POST("/domains", domainHandler.Create)
		management.DELETE("/domains/:id", domainHandler.Delete)

		// DNS Providers - only available if encryption key is configured
		if cfg.EncryptionKey != "" {
			encryptionService, err := crypto.NewEncryptionService(cfg.EncryptionKey)
			if err != nil {
				logger.Log().WithError(err).Error("Failed to initialize encryption service - DNS provider features will be unavailable")
			} else {
				dnsProviderService := services.NewDNSProviderService(db, encryptionService)
				dnsProviderHandler := handlers.NewDNSProviderHandler(dnsProviderService)
				management.GET("/dns-providers", dnsProviderHandler.List)
				management.POST("/dns-providers", dnsProviderHandler.Create)
				management.GET("/dns-providers/types", dnsProviderHandler.GetTypes)
				management.GET("/dns-providers/:id", dnsProviderHandler.Get)
				management.PUT("/dns-providers/:id", dnsProviderHandler.Update)
				management.DELETE("/dns-providers/:id", dnsProviderHandler.Delete)
				management.POST("/dns-providers/:id/test", dnsProviderHandler.Test)
				management.POST("/dns-providers/test", dnsProviderHandler.TestCredentials)
				// Audit logs for DNS providers
				management.GET("/dns-providers/:id/audit-logs", auditLogHandler.ListByProvider)

				// DNS Provider Auto-Detection (Phase 4)
				dnsDetectionService := services.NewDNSDetectionService(db)
				dnsDetectionHandler := handlers.NewDNSDetectionHandler(dnsDetectionService)
				management.POST("/dns-providers/detect", dnsDetectionHandler.Detect)
				management.GET("/dns-providers/detection-patterns", dnsDetectionHandler.GetPatterns)

				// Multi-Credential Management (Phase 3)
				credentialService := services.NewCredentialService(db, encryptionService)
				credentialHandler := handlers.NewCredentialHandler(credentialService)
				management.GET("/dns-providers/:id/credentials", credentialHandler.List)
				management.POST("/dns-providers/:id/credentials", credentialHandler.Create)
				management.GET("/dns-providers/:id/credentials/:cred_id", credentialHandler.Get)
				management.PUT("/dns-providers/:id/credentials/:cred_id", credentialHandler.Update)
				management.DELETE("/dns-providers/:id/credentials/:cred_id", credentialHandler.Delete)
				management.POST("/dns-providers/:id/credentials/:cred_id/test", credentialHandler.Test)
				management.POST("/dns-providers/:id/enable-multi-credentials", credentialHandler.EnableMultiCredentials)

				// Encryption Management - Admin only endpoints
				rotationService, rotErr := crypto.NewRotationService(db)
				if rotErr != nil {
					logger.Log().WithError(rotErr).Warn("Failed to initialize rotation service - key rotation features will be unavailable")
				} else {
					encryptionHandler := handlers.NewEncryptionHandler(rotationService, securityService)
					adminEncryption := management.Group("/admin/encryption")
					adminEncryption.GET("/status", encryptionHandler.GetStatus)
					adminEncryption.POST("/rotate", encryptionHandler.Rotate)
					adminEncryption.GET("/history", encryptionHandler.GetHistory)
					adminEncryption.POST("/validate", encryptionHandler.Validate)
				}

				// Plugin Management (Phase 5) - Admin only endpoints
				pluginDir := os.Getenv("CHARON_PLUGINS_DIR")
				if pluginDir == "" {
					pluginDir = "/app/plugins"
				}
				pluginLoader := services.NewPluginLoaderService(db, pluginDir, nil)
				pluginHandler := handlers.NewPluginHandler(db, pluginLoader)
				adminPlugins := management.Group("/admin/plugins")
				adminPlugins.GET("", pluginHandler.ListPlugins)
				adminPlugins.GET("/:id", pluginHandler.GetPlugin)
				adminPlugins.POST("/:id/enable", pluginHandler.EnablePlugin)
				adminPlugins.POST("/:id/disable", pluginHandler.DisablePlugin)
				adminPlugins.POST("/reload", pluginHandler.ReloadPlugins)

				// Manual DNS Challenges (Phase 1) - For users without automated DNS API access
				manualChallengeService := services.NewManualChallengeService(db)
				manualChallengeHandler := handlers.NewManualChallengeHandler(manualChallengeService, dnsProviderService)
				manualChallengeHandler.RegisterRoutes(management)
			}
		} else {
			logger.Log().Warn("CHARON_ENCRYPTION_KEY not set - DNS provider and plugin features will be unavailable")
		}

		// Docker - Always register routes even if Docker is unavailable
		// The service will return proper error messages when Docker is not accessible
		dockerService := services.NewDockerService()
		dockerHandler := handlers.NewDockerHandler(dockerService, remoteServerService)
		dockerHandler.RegisterRoutes(management)

		// Uptime Service — reuse the single uptimeService instance (defined above)
		// to share in-memory state (mutexes, notification batching) between
		// background checker, ProxyHostHandler, and API handlers.
		uptimeHandler := handlers.NewUptimeHandler(uptimeService)
		management.GET("/uptime/monitors", uptimeHandler.List)
		management.POST("/uptime/monitors", uptimeHandler.Create)
		management.GET("/uptime/monitors/:id/history", uptimeHandler.GetHistory)
		management.PUT("/uptime/monitors/:id", uptimeHandler.Update)
		management.DELETE("/uptime/monitors/:id", uptimeHandler.Delete)
		management.POST("/uptime/monitors/:id/check", uptimeHandler.CheckMonitor)
		management.POST("/uptime/sync", uptimeHandler.Sync)

		// Notification Providers
		notificationProviderHandler := handlers.NewNotificationProviderHandlerWithDeps(notificationService, securityService, dataRoot)
		management.GET("/notifications/providers", notificationProviderHandler.List)
		management.POST("/notifications/providers", notificationProviderHandler.Create)
		management.PUT("/notifications/providers/:id", notificationProviderHandler.Update)
		management.DELETE("/notifications/providers/:id", notificationProviderHandler.Delete)
		management.POST("/notifications/providers/test", notificationProviderHandler.Test)
		management.POST("/notifications/providers/preview", notificationProviderHandler.Preview)
		management.GET("/notifications/templates", notificationProviderHandler.Templates)

		// External notification templates (saved templates for providers)
		notificationTemplateHandler := handlers.NewNotificationTemplateHandlerWithDeps(notificationService, securityService, dataRoot)
		management.GET("/notifications/external-templates", notificationTemplateHandler.List)
		management.POST("/notifications/external-templates", notificationTemplateHandler.Create)
		management.PUT("/notifications/external-templates/:id", notificationTemplateHandler.Update)
		management.DELETE("/notifications/external-templates/:id", notificationTemplateHandler.Delete)
		management.POST("/notifications/external-templates/preview", notificationTemplateHandler.Preview)

		// Ensure uptime feature flag exists to avoid record-not-found logs
		defaultUptime := models.Setting{Key: "feature.uptime.enabled", Value: "true", Type: "bool", Category: "feature"}
		if err := db.Where(models.Setting{Key: defaultUptime.Key}).Attrs(defaultUptime).FirstOrCreate(&defaultUptime).Error; err != nil {
			logger.Log().WithError(err).Warn("Failed to ensure uptime feature flag default")
		}

		// Ensure security header presets exist
		secHeadersSvc := services.NewSecurityHeadersService(db)
		if err := secHeadersSvc.EnsurePresetsExist(); err != nil {
			logger.Log().WithError(err).Warn("Failed to initialize security header presets")
		}

		// Start background checker (every 1 minute)
		go func() {
			// Wait a bit for server to start
			time.Sleep(30 * time.Second)

			// Initial sync if enabled
			var s models.Setting
			enabled := true
			if err := db.Where("key = ?", "feature.uptime.enabled").First(&s).Error; err == nil {
				enabled = s.Value == "true"
			}

			runInitialUptimeBootstrap(
				enabled,
				uptimeService,
				func(err error, msg string) { logger.Log().WithError(err).Warn(msg) },
				func(err error, msg string) { logger.Log().WithError(err).Error(msg) },
			)

			ticker := time.NewTicker(1 * time.Minute)
			for range ticker.C {
				// Check feature flag each tick
				s = models.Setting{} // Reset to prevent ID leakage from previous query
				enabled := true
				if err := db.Where("key = ?", "feature.uptime.enabled").First(&s).Error; err == nil {
					enabled = s.Value == "true"
				}

				if enabled {
					_ = uptimeService.SyncMonitors()
					uptimeService.CheckAll()
				}
			}
		}()

		management.POST("/system/uptime/check", func(c *gin.Context) {
			go uptimeService.CheckAll()
			c.JSON(200, gin.H{"message": "Uptime check started"})
		})

		// caddyManager is already created early in Register() for use by settingsHandler

		// Initialize GeoIP service if database exists
		geoipPath := os.Getenv("CHARON_GEOIP_DB_PATH")
		if geoipPath == "" {
			geoipPath = "/app/data/geoip/GeoLite2-Country.mmdb"
		}

		var geoipSvc *services.GeoIPService
		if _, err := os.Stat(geoipPath); err == nil {
			var geoErr error
			geoipSvc, geoErr = services.NewGeoIPService(geoipPath)
			if geoErr != nil {
				logger.Log().WithError(geoErr).WithField("path", geoipPath).Warn("Failed to load GeoIP database - geo-blocking features will be unavailable")
			} else {
				logger.Log().WithField("path", geoipPath).Info("GeoIP database loaded successfully")
			}
		} else {
			logger.Log().WithField("path", geoipPath).Info("GeoIP database not found - geo-blocking features will be unavailable")
		}

		// Security Status
		securityHandler := handlers.NewSecurityHandlerWithDeps(cfg.Security, db, caddyManager, cerb)
		if geoipSvc != nil {
			securityHandler.SetGeoIPService(geoipSvc)
		}

		management.GET("/security/status", securityHandler.GetStatus)
		// Security Config management
		management.GET("/security/config", securityHandler.GetConfig)
		management.GET("/security/decisions", securityHandler.ListDecisions)
		management.GET("/security/rulesets", securityHandler.ListRuleSets)
		management.GET("/security/rate-limit/presets", securityHandler.GetRateLimitPresets)
		// GeoIP endpoints
		management.GET("/security/geoip/status", securityHandler.GetGeoIPStatus)
		// WAF exclusion endpoints
		management.GET("/security/waf/exclusions", securityHandler.GetWAFExclusions)

		securityAdmin := management.Group("/security")
		securityAdmin.Use(middleware.RequireRole(models.RoleAdmin))
		securityAdmin.POST("/config", securityHandler.UpdateConfig)
		securityAdmin.POST("/enable", securityHandler.Enable)
		securityAdmin.POST("/disable", securityHandler.Disable)
		securityAdmin.POST("/breakglass/generate", securityHandler.GenerateBreakGlass)
		securityAdmin.POST("/decisions", securityHandler.CreateDecision)
		securityAdmin.POST("/rulesets", securityHandler.UpsertRuleSet)
		securityAdmin.DELETE("/rulesets/:id", securityHandler.DeleteRuleSet)
		securityAdmin.POST("/geoip/reload", securityHandler.ReloadGeoIP)
		securityAdmin.POST("/geoip/lookup", securityHandler.LookupGeoIP)
		securityAdmin.POST("/waf/exclusions", securityHandler.AddWAFExclusion)
		securityAdmin.DELETE("/waf/exclusions/:rule_id", securityHandler.DeleteWAFExclusion)

		// Security module enable/disable endpoints (granular control)
		securityAdmin.POST("/acl/enable", securityHandler.EnableACL)
		securityAdmin.POST("/acl/disable", securityHandler.DisableACL)
		securityAdmin.PATCH("/acl", securityHandler.PatchACL) // E2E tests use PATCH
		securityAdmin.POST("/waf/enable", securityHandler.EnableWAF)
		securityAdmin.POST("/waf/disable", securityHandler.DisableWAF)
		securityAdmin.PATCH("/waf", securityHandler.PatchWAF) // E2E tests use PATCH
		securityAdmin.POST("/cerberus/enable", securityHandler.EnableCerberus)
		securityAdmin.POST("/cerberus/disable", securityHandler.DisableCerberus)
		securityAdmin.POST("/crowdsec/enable", securityHandler.EnableCrowdSec)
		securityAdmin.POST("/crowdsec/disable", securityHandler.DisableCrowdSec)
		securityAdmin.PATCH("/crowdsec", securityHandler.PatchCrowdSec) // E2E tests use PATCH
		securityAdmin.POST("/rate-limit/enable", securityHandler.EnableRateLimit)
		securityAdmin.POST("/rate-limit/disable", securityHandler.DisableRateLimit)
		securityAdmin.PATCH("/rate-limit", securityHandler.PatchRateLimit) // E2E tests use PATCH

		// CrowdSec process management and import
		// Data dir for crowdsec (persisted on host via volumes)
		crowdsecDataDir := cfg.Security.CrowdSecConfigDir

		// Use full path to CrowdSec binary to ensure it's found regardless of PATH
		crowdsecBinPath := os.Getenv("CHARON_CROWDSEC_BIN")
		if crowdsecBinPath == "" {
			crowdsecBinPath = "/usr/local/bin/crowdsec" // Default location in Alpine container
		}

		crowdsecExec := handlers.NewDefaultCrowdsecExecutor()
		crowdsecHandler := handlers.NewCrowdsecHandler(db, crowdsecExec, crowdsecBinPath, crowdsecDataDir)
		crowdsecHandler.RegisterRoutes(management)

		// NOTE: CrowdSec reconciliation now happens in main.go BEFORE HTTP server starts
		// This ensures proper initialization order and prevents race conditions
		// The log path follows CrowdSec convention: /var/log/caddy/access.log in production
		// or falls back to the configured storage directory for development
		accessLogPath := os.Getenv("CHARON_CADDY_ACCESS_LOG")
		if accessLogPath == "" {
			accessLogPath = "/var/log/caddy/access.log"
		}

		// Ensure log directory and file exist for LogWatcher
		// This prevents failures after container restart when log file doesn't exist yet
		if err := os.MkdirAll(filepath.Dir(accessLogPath), 0o750); err != nil {
			logger.Log().WithError(err).WithField("path", accessLogPath).Warn("Failed to create log directory for LogWatcher")
		}
		if _, err := os.Stat(accessLogPath); os.IsNotExist(err) {
			// #nosec G304 -- Creating access log file, path is application-controlled
			if f, err := os.Create(accessLogPath); err == nil {
				if closeErr := f.Close(); closeErr != nil {
					logger.Log().WithError(closeErr).Warn("Failed to close log file")
				}
				logger.Log().WithError(err).WithField("path", accessLogPath).Warn("Failed to create log file for LogWatcher")
			}
		}

		logWatcher := services.NewLogWatcher(accessLogPath)
		if err := logWatcher.Start(context.Background()); err != nil {
			logger.Log().WithError(err).Error("Failed to start security log watcher")
		}
		cerberusLogsHandler := handlers.NewCerberusLogsHandler(logWatcher, wsTracker)
		management.GET("/cerberus/logs/ws", cerberusLogsHandler.LiveLogs)

		// Access Lists
		accessListHandler := handlers.NewAccessListHandler(db)
		if geoipSvc != nil {
			accessListHandler.SetGeoIPService(geoipSvc)
		}
		management.GET("/access-lists/templates", accessListHandler.GetTemplates)
		management.GET("/access-lists", accessListHandler.List)
		management.POST("/access-lists", accessListHandler.Create)
		management.GET("/access-lists/:id", accessListHandler.Get)
		management.PUT("/access-lists/:id", accessListHandler.Update)
		management.DELETE("/access-lists/:id", accessListHandler.Delete)
		management.POST("/access-lists/:id/test", accessListHandler.TestIP)

		// Security Headers
		securityHeadersHandler := handlers.NewSecurityHeadersHandler(db, caddyManager)
		securityHeadersHandler.RegisterRoutes(management)

		// Certificate routes
		// Use cfg.CaddyConfigDir + "/data" for cert service so we scan the actual Caddy storage
		// where ACME and certificates are stored (e.g. <CaddyConfigDir>/data).
		caddyDataDir := cfg.CaddyConfigDir + "/data"
		logger.Log().WithField("caddy_data_dir", caddyDataDir).Info("Using Caddy data directory for certificates scan")
		var certEncSvc *crypto.EncryptionService
		if cfg.EncryptionKey != "" {
			svc, err := crypto.NewEncryptionService(cfg.EncryptionKey)
			if err != nil {
				logger.Log().WithError(err).Warn("Failed to initialize encryption service for certificate key storage")
			} else {
				certEncSvc = svc
			}
		}
		certService := services.NewCertificateService(caddyDataDir, db, certEncSvc)
		certHandler := handlers.NewCertificateHandler(certService, backupService, notificationService)
		certHandler.SetDB(db)

		// Migrate unencrypted private keys
		if err := certService.MigratePrivateKeys(); err != nil {
			logger.Log().WithError(err).Warn("Failed to migrate certificate private keys")
		}

		management.GET("/certificates", certHandler.List)
		management.POST("/certificates", certHandler.Upload)
		management.POST("/certificates/validate", certHandler.Validate)
		management.GET("/certificates/:uuid", certHandler.Get)
		management.PUT("/certificates/:uuid", certHandler.Update)
		management.POST("/certificates/:uuid/export", certHandler.Export)
		management.DELETE("/certificates/:uuid", certHandler.Delete)

		// Start certificate expiry checker
		warningDays := 30
		if cfg.CertExpiryWarningDays > 0 {
			warningDays = cfg.CertExpiryWarningDays
		}
		go certService.StartExpiryChecker(ctx, notificationService, warningDays)

		// Proxy Hosts & Remote Servers
		proxyHostHandler := handlers.NewProxyHostHandler(db, caddyManager, notificationService, uptimeService)
		proxyHostHandler.RegisterRoutes(management)

		remoteServerHandler := handlers.NewRemoteServerHandler(remoteServerService, notificationService)
		remoteServerHandler.RegisterRoutes(management)
	}

	// Caddy Manager already created above

	// Initial Caddy Config Sync
	go func() {
		// Wait for Caddy to be ready (max 30 seconds)
		ctx := context.Background()
		timeout := time.After(30 * time.Second)
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()

		ready := false
		for {
			select {
			case <-timeout:
				logger.Log().Warn("Timeout waiting for Caddy to be ready")
				return
			case <-ticker.C:
				if err := caddyManager.Ping(ctx); err == nil {
					ready = true
					goto Apply
				}
			}
		}

	Apply:
		if ready {
			// Apply config
			if err := caddyManager.ApplyConfig(ctx); err != nil {
				logger.Log().WithError(err).Error("Failed to apply initial Caddy config")
			} else {
				logger.Log().Info("Successfully applied initial Caddy config")
			}
		}
	}()

	return nil
}

// RegisterImportHandler wires up import routes with config dependencies.
func RegisterImportHandler(router *gin.Engine, db *gorm.DB, cfg config.Config, caddyBinary, importDir, mountPath string) {
	securityService := services.NewSecurityService(db)
	importHandler := handlers.NewImportHandlerWithDeps(db, caddyBinary, importDir, mountPath, securityService)
	api := router.Group("/api/v1")
	authService := services.NewAuthService(db, cfg)
	authenticatedAdmin := api.Group("/")
	authenticatedAdmin.Use(middleware.AuthMiddleware(authService), middleware.RequireRole(models.RoleAdmin))
	importHandler.RegisterRoutes(authenticatedAdmin)

	// NPM Import Handler - supports Nginx Proxy Manager export format
	npmImportHandler := handlers.NewNPMImportHandler(db)
	npmImportHandler.RegisterRoutes(authenticatedAdmin)

	// JSON Import Handler - supports both Charon and NPM export formats
	jsonImportHandler := handlers.NewJSONImportHandler(db)
	jsonImportHandler.RegisterRoutes(authenticatedAdmin)
}
