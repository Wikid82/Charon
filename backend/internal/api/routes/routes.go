package routes

import (
	"context"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/api/handlers"
	"github.com/Wikid82/charon/backend/internal/api/middleware"
	"github.com/Wikid82/charon/backend/internal/caddy"
	"github.com/Wikid82/charon/backend/internal/cerberus"
	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/Wikid82/charon/backend/internal/logger"
)

// Register wires up API routes and performs automatic migrations.
func Register(router *gin.Engine, db *gorm.DB, cfg config.Config) error {
	// AutoMigrate all models for Issue #5 persistence layer
	if err := db.AutoMigrate(
		&models.ProxyHost{},
		&models.Location{},
		&models.CaddyConfig{},
		&models.RemoteServer{},
		&models.SSLCertificate{},
		&models.AccessList{},
		&models.User{},
		&models.Setting{},
		&models.ImportSession{},
		&models.Notification{},
		&models.NotificationProvider{},
		&models.NotificationTemplate{},
		&models.UptimeMonitor{},
		&models.UptimeHeartbeat{},
		&models.UptimeHost{},
		&models.UptimeNotificationEvent{},
		&models.Domain{},
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

	router.GET("/api/v1/health", handlers.HealthHandler)

	api := router.Group("/api/v1")

	// Cerberus middleware applies the optional security suite checks (WAF, ACL, CrowdSec)
	cerb := cerberus.New(cfg.Security, db)
	api.Use(cerb.Middleware())

	// Auth routes
	authService := services.NewAuthService(db, cfg)
	authHandler := handlers.NewAuthHandler(authService)
	authMiddleware := middleware.AuthMiddleware(authService)

	// Backup routes
	backupService := services.NewBackupService(&cfg)
	backupHandler := handlers.NewBackupHandler(backupService)

	// Log routes
	logService := services.NewLogService(&cfg)
	logsHandler := handlers.NewLogsHandler(logService)

	// Notification Service (needed for multiple handlers)
	notificationService := services.NewNotificationService(db)

	// Remote Server Service (needed for Docker handler)
	remoteServerService := services.NewRemoteServerService(db)

	api.POST("/auth/login", authHandler.Login)
	api.POST("/auth/register", authHandler.Register)

	// Uptime Service - define early so it can be used during route registration
	uptimeService := services.NewUptimeService(db, notificationService)

	protected := api.Group("/")
	protected.Use(authMiddleware)
	{
		protected.POST("/auth/logout", authHandler.Logout)
		protected.GET("/auth/me", authHandler.Me)
		protected.POST("/auth/change-password", authHandler.ChangePassword)

		// Backups
		protected.GET("/backups", backupHandler.List)
		protected.POST("/backups", backupHandler.Create)
		protected.DELETE("/backups/:filename", backupHandler.Delete)
		protected.GET("/backups/:filename/download", backupHandler.Download)
		protected.POST("/backups/:filename/restore", backupHandler.Restore)

		// Logs
		protected.GET("/logs", logsHandler.List)
		protected.GET("/logs/:filename", logsHandler.Read)
		protected.GET("/logs/:filename/download", logsHandler.Download)

		// Settings
		settingsHandler := handlers.NewSettingsHandler(db)
		protected.GET("/settings", settingsHandler.GetSettings)
		protected.POST("/settings", settingsHandler.UpdateSetting)

		// Feature flags (DB-backed with env fallback)
		featureFlagsHandler := handlers.NewFeatureFlagsHandler(db)
		protected.GET("/feature-flags", featureFlagsHandler.GetFlags)
		protected.PUT("/feature-flags", featureFlagsHandler.UpdateFlags)

		// User Profile & API Key
		userHandler := handlers.NewUserHandler(db)
		protected.GET("/user/profile", userHandler.GetProfile)
		protected.POST("/user/profile", userHandler.UpdateProfile)
		protected.POST("/user/api-key", userHandler.RegenerateAPIKey)

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

		// Docker
		dockerService, err := services.NewDockerService()
		if err == nil { // Only register if Docker is available
			dockerHandler := handlers.NewDockerHandler(dockerService, remoteServerService)
			dockerHandler.RegisterRoutes(protected)
		} else {
			logger.Log().WithError(err).Warn("Docker service unavailable")
		}

		// Uptime Service
		uptimeService := services.NewUptimeService(db, notificationService)
		uptimeHandler := handlers.NewUptimeHandler(uptimeService)
		protected.GET("/uptime/monitors", uptimeHandler.List)
		protected.GET("/uptime/monitors/:id/history", uptimeHandler.GetHistory)
		protected.PUT("/uptime/monitors/:id", uptimeHandler.Update)
		protected.DELETE("/uptime/monitors/:id", uptimeHandler.Delete)
		protected.POST("/uptime/sync", uptimeHandler.Sync)

		// Notification Providers
		notificationProviderHandler := handlers.NewNotificationProviderHandler(notificationService)
		protected.GET("/notifications/providers", notificationProviderHandler.List)
		protected.POST("/notifications/providers", notificationProviderHandler.Create)
		protected.PUT("/notifications/providers/:id", notificationProviderHandler.Update)
		protected.DELETE("/notifications/providers/:id", notificationProviderHandler.Delete)
		protected.POST("/notifications/providers/test", notificationProviderHandler.Test)
		protected.POST("/notifications/providers/preview", notificationProviderHandler.Preview)
		protected.GET("/notifications/templates", notificationProviderHandler.Templates)

		// External notification templates (saved templates for providers)
		notificationTemplateHandler := handlers.NewNotificationTemplateHandler(notificationService)
		protected.GET("/notifications/external-templates", notificationTemplateHandler.List)
		protected.POST("/notifications/external-templates", notificationTemplateHandler.Create)
		protected.PUT("/notifications/external-templates/:id", notificationTemplateHandler.Update)
		protected.DELETE("/notifications/external-templates/:id", notificationTemplateHandler.Delete)
		protected.POST("/notifications/external-templates/preview", notificationTemplateHandler.Preview)

		// Start background checker (every 1 minute)
		go func() {
			// Wait a bit for server to start
			time.Sleep(30 * time.Second)
			// Initial sync
			if err := uptimeService.SyncMonitors(); err != nil {
				logger.Log().WithError(err).Error("Failed to sync monitors")
			}

			ticker := time.NewTicker(1 * time.Minute)
			for range ticker.C {
				_ = uptimeService.SyncMonitors()
				uptimeService.CheckAll()
			}
		}()

		protected.POST("/system/uptime/check", func(c *gin.Context) {
			go uptimeService.CheckAll()
			c.JSON(200, gin.H{"message": "Uptime check started"})
		})

		// Security Status
		securityHandler := handlers.NewSecurityHandler(cfg.Security, db)
		protected.GET("/security/status", securityHandler.GetStatus)

		// CrowdSec process management and import
		// Data dir for crowdsec (persisted on host via volumes)
		crowdsecDataDir := "data/crowdsec"
		crowdsecExec := handlers.NewDefaultCrowdsecExecutor()
		crowdsecHandler := handlers.NewCrowdsecHandler(db, crowdsecExec, "crowdsec", crowdsecDataDir)
		crowdsecHandler.RegisterRoutes(protected)
	}

	// Caddy Manager
	caddyClient := caddy.NewClient(cfg.CaddyAdminAPI)
	caddyManager := caddy.NewManager(caddyClient, db, cfg.CaddyConfigDir, cfg.FrontendDir, cfg.ACMEStaging, cfg.Security)

	proxyHostHandler := handlers.NewProxyHostHandler(db, caddyManager, notificationService, uptimeService)
	proxyHostHandler.RegisterRoutes(api)

	remoteServerHandler := handlers.NewRemoteServerHandler(remoteServerService, notificationService)
	remoteServerHandler.RegisterRoutes(api)

	// Access Lists
	accessListHandler := handlers.NewAccessListHandler(db)
	protected.GET("/access-lists/templates", accessListHandler.GetTemplates)
	protected.GET("/access-lists", accessListHandler.List)
	protected.POST("/access-lists", accessListHandler.Create)
	protected.GET("/access-lists/:id", accessListHandler.Get)
	protected.PUT("/access-lists/:id", accessListHandler.Update)
	protected.DELETE("/access-lists/:id", accessListHandler.Delete)
	protected.POST("/access-lists/:id/test", accessListHandler.TestIP)

	userHandler := handlers.NewUserHandler(db)
	userHandler.RegisterRoutes(api)

	// Certificate routes
	// Use cfg.CaddyConfigDir + "/data" for cert service so we scan the actual Caddy storage
	// where ACME and certificates are stored (e.g. <CaddyConfigDir>/data).
	caddyDataDir := cfg.CaddyConfigDir + "/data"
	logger.Log().WithField("caddy_data_dir", caddyDataDir).Info("Using Caddy data directory for certificates scan")
	certService := services.NewCertificateService(caddyDataDir, db)
	certHandler := handlers.NewCertificateHandler(certService, notificationService)
	api.GET("/certificates", certHandler.List)
	api.POST("/certificates", certHandler.Upload)
	api.DELETE("/certificates/:id", certHandler.Delete)

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
	importHandler := handlers.NewImportHandler(db, caddyBinary, importDir, mountPath)
	api := router.Group("/api/v1")
	importHandler.RegisterRoutes(api)
}
