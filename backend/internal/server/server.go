// Package server provides the HTTP server and router configuration.
package server

import (
	"net/http"
	"strings"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/gin-gonic/gin"
)

// NewRouter creates a new Gin router with frontend static file serving.
// dataDir is the application data directory (e.g. filepath.Dir(cfg.DatabasePath)).
// When non-empty, /uploads is served from dataDir/uploads for custom logo files.
// trustedProxies lists IPs/CIDRs of reverse proxies whose forwarded headers
// Gin's own ClientIP() should honor (see docs/plans/current_spec.md §13).
// Empty/nil trusts nothing, matching Gin's SetTrustedProxies(nil) default.
func NewRouter(frontendDir string, dataDir string, trustedProxies []string) *gin.Engine {
	router := gin.Default()
	// Gin trusts all proxies by default. In v1.11.x, SetTrustedProxies(nil) disables
	// trusting forwarded headers entirely, making Context.ClientIP() use the remote
	// socket address. Only enable trusted proxies via an explicit allow-list.
	if len(trustedProxies) > 0 {
		if err := router.SetTrustedProxies(trustedProxies); err != nil {
			// Fail safe: an invalid entry must not silently fall through to
			// Gin's "trust everyone" behavior. Log and trust nothing.
			logger.Log().WithError(err).Warn("invalid CHARON_TRUSTED_PROXIES entry; trusting no proxies")
			_ = router.SetTrustedProxies(nil)
		}
	} else {
		_ = router.SetTrustedProxies(nil)
	}

	// Serve uploaded logo files from data directory
	if dataDir != "" {
		router.Static("/uploads", dataDir+"/uploads")
	}

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
