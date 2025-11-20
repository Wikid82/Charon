package routes

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/api/handlers"
	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/api/middleware"
	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/config"
	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/models"
	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/services"
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
	); err != nil {
		return fmt.Errorf("auto migrate: %w", err)
	}

	router.GET("/api/v1/health", handlers.HealthHandler)

	api := router.Group("/api/v1")

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

	api.POST("/auth/login", authHandler.Login)
	api.POST("/auth/register", authHandler.Register)

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

	}

	proxyHostHandler := handlers.NewProxyHostHandler(db)
	proxyHostHandler.RegisterRoutes(api)

	remoteServerHandler := handlers.NewRemoteServerHandler(db)
	remoteServerHandler.RegisterRoutes(api)

	userHandler := handlers.NewUserHandler(db)
	userHandler.RegisterRoutes(api)

	// Certificate routes
	// Use cfg.CaddyConfigDir + "/data" for cert service
	caddyDataDir := cfg.CaddyConfigDir + "/data"
	certService := services.NewCertificateService(caddyDataDir)
	certHandler := handlers.NewCertificateHandler(certService)
	api.GET("/certificates", certHandler.List)

	return nil
}

// RegisterImportHandler wires up import routes with config dependencies.
func RegisterImportHandler(router *gin.Engine, db *gorm.DB, caddyBinary, importDir string) {
	importHandler := handlers.NewImportHandler(db, caddyBinary, importDir)
	api := router.Group("/api/v1")
	importHandler.RegisterRoutes(api)
}
