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
	if err := db.AutoMigrate(&models.ProxyHost{}, &models.CaddyConfig{}); err != nil {
		return fmt.Errorf("auto migrate: %w", err)
	}

	router.GET("/api/v1/health", handlers.HealthHandler)

	proxyHostHandler := handlers.NewProxyHostHandler(db)
	api := router.Group("/api/v1")
	proxyHostHandler.RegisterRoutes(api)

	return nil
}
