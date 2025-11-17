package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// HealthHandler responds with basic service metadata for uptime checks.
func HealthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":  "ok",
		"service": "caddy-proxy-manager-plus",
	})
}
