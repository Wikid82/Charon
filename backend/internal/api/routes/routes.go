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

// Register wires up API routes and performs automatic migrations.
func Register(router *gin.Engine, db *gorm.DB, cfg config.Config) error {
	// Caddy Manager - created early so it can be used by settings handlers for config reload
	caddyClient := caddy.NewClient(cfg.CaddyAdminAPI)
	caddyManager := caddy.NewManager(caddyClient, db, cfg.CaddyConfigDir, cfg.FrontendDir, cfg.ACMEStaging, cfg.Security)

	// Cerberus middleware applies the optional security suite checks (WAF, ACL, CrowdSec)
	cerb := cerberus.New(cfg.Security, db)

	return RegisterWithDeps(router, db, cfg, caddyManager, cerb)
}

// RegisterWithDeps wires up API routes and performs automatic migrations with prebuilt dependencies.
func RegisterWithDeps(router *gin.Engine, db *gorm.DB, cfg config.Config, caddyManager *caddy.Manager, cerb *cerberus.Cerberus) error {
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
	); err != nil {
		return fmt.Errorf("auto migrate: %w", err)
	}

	// Clean up invalid Let's Encrypt certificate associations
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
	notificationService := services.NewNotificationService(db)

	// Ensure notify-only provider migration reconciliation at boot
	if err := notificationService.EnsureNotifyOnlyProviderMigration(context.Background()); err != nil {
		return fmt.Errorf("notify-only provider migration: %w", err)
	}

	// Remote Server Service (needed for Docker handler)
	remoteServerService := services.NewRemoteServerService(db)

	api.POST("/auth/login", authHandler.Login)
	api.POST("/auth/register", authHandler.Register)

	// Forward auth endpoint for Caddy (public, validates session internally)
	api.GET("/auth/verify", authHandler.Verify)
	api.GET("/auth/status", authHandler.VerifyStatus)

	// User handler (public endpoints)
	userHandler := handlers.NewUserHandler(db)
	api.GET("/setup", userHandler.GetSetupStatus)
	api.POST("/setup", userHandler.Setup)
	api.GET("/invite/validate", userHandler.ValidateInvite)
	api.POST("/invite/accept", userHandler.AcceptInvite)

	// Uptime Service - define early so it can be used during route registration
	uptimeService := services.NewUptimeService(db, notificationService)

	protected := api.Group("/")
	protected.Use(authMiddleware)
	{
		protected.POST("/auth/logout", authHandler.Logout)
		protected.POST("/auth/refresh", authHandler.Refresh)
		protected.GET("/auth/me", authHandler.Me)
		protected.POST("/auth/change-password", authHandler.ChangePassword)

		// Backups
		protected.GET("/backups", backupHandler.List)
		protected.POST("/backups", backupHandler.Create)
		protected.DELETE("/backups/:filename", backupHandler.Delete)
		protected.GET("/backups/:filename/download", backupHandler.Download)
		protected.POST("/backups/:filename/restore", backupHandler.Restore)

		// Logs
		// WebSocket endpoints
		logsWSHandler := handlers.NewLogsWSHandler(wsTracker)
		protected.GET("/logs/live", logsWSHandler.HandleWebSocket)
		protected.GET("/logs", logsHandler.List)
		protected.GET("/logs/:filename", logsHandler.Read)
		protected.GET("/logs/:filename/download", logsHandler.Download)

		// WebSocket status monitoring
		protected.GET("/websocket/connections", wsStatusHandler.GetConnections)
		protected.GET("/websocket/stats", wsStatusHandler.GetStats)

		dataRoot := filepath.Dir(cfg.DatabasePath)

		// Security Notification Settings
		securityNotificationService := services.NewSecurityNotificationService(db)
		securityNotificationHandler := handlers.NewSecurityNotificationHandlerWithDeps(securityNotificationService, securityService, dataRoot)
		protected.GET("/security/notifications/settings", securityNotificationHandler.DeprecatedGetSettings)
		protected.PUT("/security/notifications/settings", securityNotificationHandler.DeprecatedUpdateSettings)
		protected.GET("/notifications/settings/security", securityNotificationHandler.GetSettings)
		protected.PUT("/notifications/settings/security", securityNotificationHandler.UpdateSettings)

		// System permissions diagnostics and repair
		systemPermissionsHandler := handlers.NewSystemPermissionsHandler(cfg, securityService, nil)
		protected.GET("/system/permissions", systemPermissionsHandler.GetPermissions)
		protected.POST("/system/permissions/repair", systemPermissionsHandler.RepairPermissions)

		// Audit Logs
		auditLogHandler := handlers.NewAuditLogHandler(securityService)
		protected.GET("/audit-logs", auditLogHandler.List)
		protected.GET("/audit-logs/:uuid", auditLogHandler.Get)

		// Settings - with CaddyManager and Cerberus for security settings reload
		settingsHandler := handlers.NewSettingsHandlerWithDeps(db, caddyManager, cerb, securityService, dataRoot)

		protected.GET("/settings", settingsHandler.GetSettings)
		protected.POST("/settings", settingsHandler.UpdateSetting)
		protected.PATCH("/settings", settingsHandler.UpdateSetting) // E2E tests use PATCH
		protected.PATCH("/config", settingsHandler.PatchConfig)     // Bulk configuration update

		// SMTP Configuration
		protected.GET("/settings/smtp", settingsHandler.GetSMTPConfig)
		protected.POST("/settings/smtp", settingsHandler.UpdateSMTPConfig)
		protected.POST("/settings/smtp/test", settingsHandler.TestSMTPConfig)
		protected.POST("/settings/smtp/test-email", settingsHandler.SendTestEmail)

		// URL Validation
		protected.POST("/settings/validate-url", settingsHandler.ValidatePublicURL)
		protected.POST("/settings/test-url", settingsHandler.TestPublicURL)

		// Auth related protected routes
		protected.GET("/auth/accessible-hosts", authHandler.GetAccessibleHosts)
		protected.GET("/auth/check-host/:hostId", authHandler.CheckHostAccess)

		// Feature flags (DB-backed with env fallback)
		featureFlagsHandler := handlers.NewFeatureFlagsHandler(db)
		protected.GET("/feature-flags", featureFlagsHandler.GetFlags)
		protected.PUT("/feature-flags", featureFlagsHandler.UpdateFlags)

		// User Profile & API Key
		protected.GET("/user/profile", userHandler.GetProfile)
		protected.POST("/user/profile", userHandler.UpdateProfile)
		protected.POST("/user/api-key", userHandler.RegenerateAPIKey)

		// User Management (admin only routes are in RegisterRoutes)
		protected.GET("/users", userHandler.ListUsers)
		protected.POST("/users", userHandler.CreateUser)
		protected.POST("/users/invite", userHandler.InviteUser)
		protected.POST("/users/preview-invite-url", userHandler.PreviewInviteURL)
		protected.GET("/users/:id", userHandler.GetUser)
		protected.PUT("/users/:id", userHandler.UpdateUser)
		protected.DELETE("/users/:id", userHandler.DeleteUser)
		protected.PUT("/users/:id/permissions", userHandler.UpdateUserPermissions)
		protected.POST("/users/:id/resend-invite", userHandler.ResendInvite)

		// Updates
		updateService := services.NewUpdateService()
		updateHandler := handlers.NewUpdateHandler(updateService)
		protected.GET("/system/updates", updateHandler.Check)

		// System info
		systemHandler := handlers.NewSystemHandler()
		protected.GET("/system/my-ip", systemHandler.GetMyIP)

		// Notifications
		notificationHandler := handlers.NewNotificationHandler(notificationService)
		protected.GET("/notifications", notificationHandler.List)
		protected.POST("/notifications/:id/read", notificationHandler.MarkAsRead)
		protected.POST("/notifications/read-all", notificationHandler.MarkAllAsRead)

		// Domains
		domainHandler := handlers.NewDomainHandler(db, notificationService)
		protected.GET("/domains", domainHandler.List)
		protected.POST("/domains", domainHandler.Create)
		protected.DELETE("/domains/:id", domainHandler.Delete)

		// DNS Providers - only available if encryption key is configured
		if cfg.EncryptionKey != "" {
			encryptionService, err := crypto.NewEncryptionService(cfg.EncryptionKey)
			if err != nil {
				logger.Log().WithError(err).Error("Failed to initialize encryption service - DNS provider features will be unavailable")
			} else {
				dnsProviderService := services.NewDNSProviderService(db, encryptionService)
				dnsProviderHandler := handlers.NewDNSProviderHandler(dnsProviderService)
				protected.GET("/dns-providers", dnsProviderHandler.List)
				protected.POST("/dns-providers", dnsProviderHandler.Create)
				protected.GET("/dns-providers/types", dnsProviderHandler.GetTypes)
				protected.GET("/dns-providers/:id", dnsProviderHandler.Get)
				protected.PUT("/dns-providers/:id", dnsProviderHandler.Update)
				protected.DELETE("/dns-providers/:id", dnsProviderHandler.Delete)
				protected.POST("/dns-providers/:id/test", dnsProviderHandler.Test)
				protected.POST("/dns-providers/test", dnsProviderHandler.TestCredentials)
				// Audit logs for DNS providers
				protected.GET("/dns-providers/:id/audit-logs", auditLogHandler.ListByProvider)

				// DNS Provider Auto-Detection (Phase 4)
				dnsDetectionService := services.NewDNSDetectionService(db)
				dnsDetectionHandler := handlers.NewDNSDetectionHandler(dnsDetectionService)
				protected.POST("/dns-providers/detect", dnsDetectionHandler.Detect)
				protected.GET("/dns-providers/detection-patterns", dnsDetectionHandler.GetPatterns)

				// Multi-Credential Management (Phase 3)
				credentialService := services.NewCredentialService(db, encryptionService)
				credentialHandler := handlers.NewCredentialHandler(credentialService)
				protected.GET("/dns-providers/:id/credentials", credentialHandler.List)
				protected.POST("/dns-providers/:id/credentials", credentialHandler.Create)
				protected.GET("/dns-providers/:id/credentials/:cred_id", credentialHandler.Get)
				protected.PUT("/dns-providers/:id/credentials/:cred_id", credentialHandler.Update)
				protected.DELETE("/dns-providers/:id/credentials/:cred_id", credentialHandler.Delete)
				protected.POST("/dns-providers/:id/credentials/:cred_id/test", credentialHandler.Test)
				protected.POST("/dns-providers/:id/enable-multi-credentials", credentialHandler.EnableMultiCredentials)

				// Encryption Management - Admin only endpoints
				rotationService, rotErr := crypto.NewRotationService(db)
				if rotErr != nil {
					logger.Log().WithError(rotErr).Warn("Failed to initialize rotation service - key rotation features will be unavailable")
				} else {
					encryptionHandler := handlers.NewEncryptionHandler(rotationService, securityService)
					adminEncryption := protected.Group("/admin/encryption")
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
				adminPlugins := protected.Group("/admin/plugins")
				adminPlugins.GET("", pluginHandler.ListPlugins)
				adminPlugins.GET("/:id", pluginHandler.GetPlugin)
				adminPlugins.POST("/:id/enable", pluginHandler.EnablePlugin)
				adminPlugins.POST("/:id/disable", pluginHandler.DisablePlugin)
				adminPlugins.POST("/reload", pluginHandler.ReloadPlugins)

				// Manual DNS Challenges (Phase 1) - For users without automated DNS API access
				manualChallengeService := services.NewManualChallengeService(db)
				manualChallengeHandler := handlers.NewManualChallengeHandler(manualChallengeService, dnsProviderService)
				manualChallengeHandler.RegisterRoutes(protected)
			}
		} else {
			logger.Log().Warn("CHARON_ENCRYPTION_KEY not set - DNS provider and plugin features will be unavailable")
		}

		// Docker - Always register routes even if Docker is unavailable
		// The service will return proper error messages when Docker is not accessible
		dockerService := services.NewDockerService()
		dockerHandler := handlers.NewDockerHandler(dockerService, remoteServerService)
		dockerHandler.RegisterRoutes(protected)

		// Uptime Service
		uptimeSvc := services.NewUptimeService(db, notificationService)
		uptimeHandler := handlers.NewUptimeHandler(uptimeSvc)
		protected.GET("/uptime/monitors", uptimeHandler.List)
		protected.POST("/uptime/monitors", uptimeHandler.Create)
		protected.GET("/uptime/monitors/:id/history", uptimeHandler.GetHistory)
		protected.PUT("/uptime/monitors/:id", uptimeHandler.Update)
		protected.DELETE("/uptime/monitors/:id", uptimeHandler.Delete)
		protected.POST("/uptime/monitors/:id/check", uptimeHandler.CheckMonitor)
		protected.POST("/uptime/sync", uptimeHandler.Sync)

		// Notification Providers
		notificationProviderHandler := handlers.NewNotificationProviderHandlerWithDeps(notificationService, securityService, dataRoot)
		protected.GET("/notifications/providers", notificationProviderHandler.List)
		protected.POST("/notifications/providers", notificationProviderHandler.Create)
		protected.PUT("/notifications/providers/:id", notificationProviderHandler.Update)
		protected.DELETE("/notifications/providers/:id", notificationProviderHandler.Delete)
		protected.POST("/notifications/providers/test", notificationProviderHandler.Test)
		protected.POST("/notifications/providers/preview", notificationProviderHandler.Preview)
		protected.GET("/notifications/templates", notificationProviderHandler.Templates)

		// External notification templates (saved templates for providers)
		notificationTemplateHandler := handlers.NewNotificationTemplateHandlerWithDeps(notificationService, securityService, dataRoot)
		protected.GET("/notifications/external-templates", notificationTemplateHandler.List)
		protected.POST("/notifications/external-templates", notificationTemplateHandler.Create)
		protected.PUT("/notifications/external-templates/:id", notificationTemplateHandler.Update)
		protected.DELETE("/notifications/external-templates/:id", notificationTemplateHandler.Delete)
		protected.POST("/notifications/external-templates/preview", notificationTemplateHandler.Preview)

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

			if enabled {
				if err := uptimeService.SyncMonitors(); err != nil {
					logger.Log().WithError(err).Error("Failed to sync monitors")
				}
			}

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

		protected.POST("/system/uptime/check", func(c *gin.Context) {
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

		protected.GET("/security/status", securityHandler.GetStatus)
		// Security Config management
		protected.GET("/security/config", securityHandler.GetConfig)
		protected.POST("/security/config", securityHandler.UpdateConfig)
		protected.POST("/security/enable", securityHandler.Enable)
		protected.POST("/security/disable", securityHandler.Disable)
		protected.POST("/security/breakglass/generate", securityHandler.GenerateBreakGlass)
		protected.GET("/security/decisions", securityHandler.ListDecisions)
		protected.POST("/security/decisions", securityHandler.CreateDecision)
		protected.GET("/security/rulesets", securityHandler.ListRuleSets)
		protected.POST("/security/rulesets", securityHandler.UpsertRuleSet)
		protected.DELETE("/security/rulesets/:id", securityHandler.DeleteRuleSet)
		protected.GET("/security/rate-limit/presets", securityHandler.GetRateLimitPresets)
		// GeoIP endpoints
		protected.GET("/security/geoip/status", securityHandler.GetGeoIPStatus)
		protected.POST("/security/geoip/reload", securityHandler.ReloadGeoIP)
		protected.POST("/security/geoip/lookup", securityHandler.LookupGeoIP)
		// WAF exclusion endpoints
		protected.GET("/security/waf/exclusions", securityHandler.GetWAFExclusions)
		protected.POST("/security/waf/exclusions", securityHandler.AddWAFExclusion)
		protected.DELETE("/security/waf/exclusions/:rule_id", securityHandler.DeleteWAFExclusion)

		// Security module enable/disable endpoints (granular control)
		protected.POST("/security/acl/enable", securityHandler.EnableACL)
		protected.POST("/security/acl/disable", securityHandler.DisableACL)
		protected.PATCH("/security/acl", securityHandler.PatchACL) // E2E tests use PATCH
		protected.POST("/security/waf/enable", securityHandler.EnableWAF)
		protected.POST("/security/waf/disable", securityHandler.DisableWAF)
		protected.PATCH("/security/waf", securityHandler.PatchWAF) // E2E tests use PATCH
		protected.POST("/security/cerberus/enable", securityHandler.EnableCerberus)
		protected.POST("/security/cerberus/disable", securityHandler.DisableCerberus)
		protected.POST("/security/crowdsec/enable", securityHandler.EnableCrowdSec)
		protected.POST("/security/crowdsec/disable", securityHandler.DisableCrowdSec)
		protected.PATCH("/security/crowdsec", securityHandler.PatchCrowdSec) // E2E tests use PATCH
		protected.POST("/security/rate-limit/enable", securityHandler.EnableRateLimit)
		protected.POST("/security/rate-limit/disable", securityHandler.DisableRateLimit)
		protected.PATCH("/security/rate-limit", securityHandler.PatchRateLimit) // E2E tests use PATCH

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
		crowdsecHandler.RegisterRoutes(protected)

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
		protected.GET("/cerberus/logs/ws", cerberusLogsHandler.LiveLogs)

		// Access Lists
		accessListHandler := handlers.NewAccessListHandler(db)
		if geoipSvc != nil {
			accessListHandler.SetGeoIPService(geoipSvc)
		}
		protected.GET("/access-lists/templates", accessListHandler.GetTemplates)
		protected.GET("/access-lists", accessListHandler.List)
		protected.POST("/access-lists", accessListHandler.Create)
		protected.GET("/access-lists/:id", accessListHandler.Get)
		protected.PUT("/access-lists/:id", accessListHandler.Update)
		protected.DELETE("/access-lists/:id", accessListHandler.Delete)
		protected.POST("/access-lists/:id/test", accessListHandler.TestIP)

		// Security Headers
		securityHeadersHandler := handlers.NewSecurityHeadersHandler(db, caddyManager)
		securityHeadersHandler.RegisterRoutes(protected)

		// Certificate routes
		// Use cfg.CaddyConfigDir + "/data" for cert service so we scan the actual Caddy storage
		// where ACME and certificates are stored (e.g. <CaddyConfigDir>/data).
		caddyDataDir := cfg.CaddyConfigDir + "/data"
		logger.Log().WithField("caddy_data_dir", caddyDataDir).Info("Using Caddy data directory for certificates scan")
		certService := services.NewCertificateService(caddyDataDir, db)
		certHandler := handlers.NewCertificateHandler(certService, backupService, notificationService)
		protected.GET("/certificates", certHandler.List)
		protected.POST("/certificates", certHandler.Upload)
		protected.DELETE("/certificates/:id", certHandler.Delete)
	}

	// Caddy Manager already created above

	proxyHostHandler := handlers.NewProxyHostHandler(db, caddyManager, notificationService, uptimeService)
	proxyHostHandler.RegisterRoutes(protected)

	remoteServerHandler := handlers.NewRemoteServerHandler(remoteServerService, notificationService)
	remoteServerHandler.RegisterRoutes(api)

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
func RegisterImportHandler(router *gin.Engine, db *gorm.DB, caddyBinary, importDir, mountPath string) {
	securityService := services.NewSecurityService(db)
	importHandler := handlers.NewImportHandlerWithDeps(db, caddyBinary, importDir, mountPath, securityService)
	api := router.Group("/api/v1")
	importHandler.RegisterRoutes(api)

	// NPM Import Handler - supports Nginx Proxy Manager export format
	npmImportHandler := handlers.NewNPMImportHandler(db)
	npmImportHandler.RegisterRoutes(api)

	// JSON Import Handler - supports both Charon and NPM export formats
	jsonImportHandler := handlers.NewJSONImportHandler(db)
	jsonImportHandler.RegisterRoutes(api)
}
