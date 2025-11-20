package handlers

import (
	"net/http"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/version"
	"github.com/gin-gonic/gin"
)

// HealthHandler responds with basic service metadata for uptime checks.
func HealthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":     "ok",
		"service":    version.Name,
		"version":    version.Version,
		"git_commit": version.GitCommit,
		"build_time": version.BuildTime,
	})
}
