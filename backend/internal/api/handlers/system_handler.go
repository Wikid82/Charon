package handlers

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

type SystemHandler struct{}

func NewSystemHandler() *SystemHandler {
	return &SystemHandler{}
}

type MyIPResponse struct {
	IP     string `json:"ip"`
	Source string `json:"source"`
}

// GetMyIP returns the client's public IP address
func (h *SystemHandler) GetMyIP(c *gin.Context) {
	// Try to get the real IP from various headers (in order of preference)
	// This handles proxies, load balancers, and CDNs
	ip := getClientIP(c.Request)

	source := "direct"
	if c.GetHeader("X-Forwarded-For") != "" {
		source = "X-Forwarded-For"
	} else if c.GetHeader("X-Real-IP") != "" {
		source = "X-Real-IP"
	} else if c.GetHeader("CF-Connecting-IP") != "" {
		source = "Cloudflare"
	}

	c.JSON(http.StatusOK, MyIPResponse{
		IP:     ip,
		Source: source,
	})
}

// getClientIP extracts the real client IP from the request
// Checks headers in order of trust/reliability
func getClientIP(r *http.Request) string {
	// Cloudflare
	if ip := r.Header.Get("CF-Connecting-IP"); ip != "" {
		return ip
	}

	// Other CDNs/proxies
	if ip := r.Header.Get("X-Real-IP"); ip != "" {
		return ip
	}

	// Standard proxy header (can be a comma-separated list)
	if forwarded := r.Header.Get("X-Forwarded-For"); forwarded != "" {
		// Take the first IP in the list (client IP)
		ips := strings.Split(forwarded, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0])
		}
	}

	// Fallback to RemoteAddr (format: "IP:port")
	if ip := r.RemoteAddr; ip != "" {
		// Remove port if present
		if idx := strings.LastIndex(ip, ":"); idx != -1 {
			return ip[:idx]
		}
		return ip
	}

	return "unknown"
}
