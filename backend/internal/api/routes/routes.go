package routes

import (
	"fmt"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/api/handlers"
	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/models"
)

// Register wires up API routes and performs automatic migrations.
func Register(router *gin.Engine, db *gorm.DB) error {
	// AutoMigrate all models for Issue #5 persistence layer
	if err := db.AutoMigrate(
		&models.ProxyHost{},
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

	proxyHostHandler := handlers.NewProxyHostHandler(db)
	proxyHostHandler.RegisterRoutes(api)

	remoteServerHandler := handlers.NewRemoteServerHandler(db)
	remoteServerHandler.RegisterRoutes(api)

	return nil
}

// RegisterImportHandler wires up import routes with config dependencies.
func RegisterImportHandler(router *gin.Engine, db *gorm.DB, caddyBinary, importDir string) {
	importHandler := handlers.NewImportHandler(db, caddyBinary, importDir)
	api := router.Group("/api/v1")
	importHandler.RegisterRoutes(api)
}
