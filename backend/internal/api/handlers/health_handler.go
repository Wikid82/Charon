package handlers

import (
	"net"
	"net/http"

	"github.com/Wikid82/CaddyProxyManagerPlus/backend/internal/version"
	"github.com/gin-gonic/gin"
)

// getLocalIP returns the non-loopback local IP of the host
func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, address := range addrs {
		// check the address type and if it is not a loopback then return it
		if ipnet, ok := address.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}
	return ""
}

// HealthHandler responds with basic service metadata for uptime checks.
func HealthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":      "ok",
		"service":     version.Name,
		"version":     version.Version,
		"git_commit":  version.GitCommit,
		"build_time":  version.BuildTime,
		"internal_ip": getLocalIP(),
	})
}
