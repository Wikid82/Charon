package middleware

import (
	"crypto/subtle"
	"net"
	"os"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/util"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

const (
	// EmergencyTokenHeader is the HTTP header name for emergency token
	EmergencyTokenHeader = "X-Emergency-Token"
	// EmergencyTokenEnvVar is the environment variable name for emergency token
	EmergencyTokenEnvVar = "CHARON_EMERGENCY_TOKEN"
	// MinTokenLength is the minimum required length for emergency tokens
	MinTokenLength = 32
)

// EmergencyBypass creates middleware that bypasses all security checks
// when a valid emergency token is present from an authorized source.
//
// Security conditions (ALL must be met):
// 1. Request from management CIDR (RFC1918 private networks by default)
// 2. X-Emergency-Token header matches configured token (timing-safe)
// 3. Token meets minimum length requirement (32+ chars)
//
// This middleware must be registered FIRST in the middleware chain.
func EmergencyBypass(managementCIDRs []string, db *gorm.DB) gin.HandlerFunc {
	// Load emergency token from environment
	emergencyToken := os.Getenv(EmergencyTokenEnvVar)
	if emergencyToken == "" {
		logger.Log().Warn("CHARON_EMERGENCY_TOKEN not set - emergency bypass disabled")
		return func(c *gin.Context) { c.Next() } // noop
	}

	if len(emergencyToken) < MinTokenLength {
		logger.Log().Warn("CHARON_EMERGENCY_TOKEN too short - emergency bypass disabled")
		return func(c *gin.Context) { c.Next() } // noop
	}

	// Parse management CIDRs
	var managementNets []*net.IPNet
	for _, cidr := range managementCIDRs {
		_, ipnet, err := net.ParseCIDR(cidr)
		if err != nil {
			logger.Log().WithError(err).WithField("cidr", cidr).Warn("Invalid management CIDR")
			continue
		}
		managementNets = append(managementNets, ipnet)
	}

	// Default to RFC1918 private networks if none specified
	if len(managementNets) == 0 {
		managementNets = []*net.IPNet{
			mustParseCIDR("10.0.0.0/8"),
			mustParseCIDR("172.16.0.0/12"),
			mustParseCIDR("192.168.0.0/16"),
			mustParseCIDR("127.0.0.0/8"), // localhost for local development
			mustParseCIDR("::1/128"),     // IPv6 localhost
		}
	}

	return func(c *gin.Context) {
		// Check if emergency token is present
		providedToken := c.GetHeader(EmergencyTokenHeader)
		if providedToken == "" {
			c.Next() // No emergency token - proceed normally
			return
		}

		// Validate source IP is from management network
		clientIPStr := util.CanonicalizeIPForSecurity(c.ClientIP())
		clientIP := net.ParseIP(clientIPStr)
		if clientIP == nil {
			logger.Log().WithField("ip", util.SanitizeForLog(clientIPStr)).Warn("Emergency bypass: invalid client IP")
			c.Next()
			return
		}

		inManagementNet := false
		for _, ipnet := range managementNets {
			if ipnet.Contains(clientIP) {
				inManagementNet = true
				break
			}
		}

		if !inManagementNet {
			logger.Log().WithField("ip", util.SanitizeForLog(clientIP.String())).Warn("Emergency bypass: IP not in management network")
			c.Next()
			return
		}

		// Timing-safe token comparison
		if !constantTimeCompare(emergencyToken, providedToken) {
			logger.Log().WithField("ip", util.SanitizeForLog(clientIP.String())).Warn("Emergency bypass: invalid token")
			c.Next()
			return
		}

		// Valid emergency token from authorized source
		logger.Log().WithFields(map[string]interface{}{
			"ip":   util.SanitizeForLog(clientIP.String()),
			"path": util.SanitizeForLog(c.Request.URL.Path),
		}).Warn("EMERGENCY BYPASS ACTIVE: Request bypassing all security checks")

		// Set flag for downstream handlers to know this is an emergency request
		c.Set("emergency_bypass", true)

		// Strip emergency token header to prevent it from reaching application
		// This is critical for security - prevents token exposure in logs
		c.Request.Header.Del(EmergencyTokenHeader)

		c.Next()
	}
}

func mustParseCIDR(cidr string) *net.IPNet {
	_, ipnet, _ := net.ParseCIDR(cidr)
	return ipnet
}

func constantTimeCompare(a, b string) bool {
	return subtle.ConstantTimeCompare([]byte(a), []byte(b)) == 1
}
