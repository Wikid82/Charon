// Package server provides the HTTP server and router configuration.
package server

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// NewRouter creates a new Gin router with frontend static file serving.
func NewRouter(frontendDir string) *gin.Engine {
	router := gin.Default()
	// Gin trusts all proxies by default. In v1.11.x, SetTrustedProxies(nil) disables
	// trusting forwarded headers entirely, making Context.ClientIP() use the remote
	// socket address. Only enable trusted proxies via an explicit allow-list.
	_ = router.SetTrustedProxies(nil)

	// Serve frontend static files
	if frontendDir != "" {
		router.Static("/assets", frontendDir+"/assets")
		router.StaticFile("/", frontendDir+"/index.html")
		router.StaticFile("/banner.png", frontendDir+"/banner.png")
		router.StaticFile("/banner.webp", frontendDir+"/banner.webp")
		router.StaticFile("/banner.svg", frontendDir+"/banner.svg")
		router.StaticFile("/logo.png", frontendDir+"/logo.png")
		router.StaticFile("/logo.webp", frontendDir+"/logo.webp")
		router.StaticFile("/logo.svg", frontendDir+"/logo.svg")
		router.StaticFile("/favicon.png", frontendDir+"/favicon.png")
		router.NoRoute(func(c *gin.Context) {
			// API routes should never fall back to the SPA HTML.
			path := c.Request.URL.Path
			if path == "/api" || strings.HasPrefix(path, "/api/") {
				c.JSON(http.StatusNotFound, gin.H{"error": "not found"})
				return
			}
			c.File(frontendDir + "/index.html")
		})
	}

	return router
}
