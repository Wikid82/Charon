package middleware

import (
	"net/http"
	"strings"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/gin-gonic/gin"
)

func AuthMiddleware(authService *services.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if bypass, exists := c.Get("emergency_bypass"); exists {
			if bypassActive, ok := bypass.(bool); ok && bypassActive {
				c.Set("role", "admin")
				c.Set("userID", uint(0))
				c.Next()
				return
			}
		}

		if authService == nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			return
		}

		tokenString, ok := extractAuthToken(c)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Authorization header required"})
			return
		}

		user, _, err := authService.AuthenticateToken(tokenString)
		if err != nil {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			return
		}

		c.Set("userID", user.ID)
		c.Set("role", string(user.Role))
		c.Next()
	}
}

func extractAuthToken(c *gin.Context) (string, bool) {
	authHeader := c.GetHeader("Authorization")

	// Fall back to cookie for browser flows (including WebSocket upgrades)
	if authHeader == "" {
		if cookieToken := extractAuthCookieToken(c); cookieToken != "" {
			authHeader = "Bearer " + cookieToken
		}
	}

	// DEPRECATED: Query parameter authentication for WebSocket connections
	// This fallback exists only for backward compatibility and will be removed in a future version.
	// Query parameters are logged in access logs and should not be used for sensitive data.
	// Use HttpOnly cookies instead, which are automatically sent by browsers and not logged.
	if authHeader == "" {
		if token := c.Query("token"); token != "" {
			authHeader = "Bearer " + token
		}
	}

	if authHeader == "" {
		return "", false
	}

	tokenString := strings.TrimPrefix(authHeader, "Bearer ")
	if tokenString == "" {
		return "", false
	}

	return tokenString, true
}

func extractAuthCookieToken(c *gin.Context) string {
	if c.Request == nil {
		return ""
	}

	token := ""
	for _, cookie := range c.Request.Cookies() {
		if cookie.Name != "auth_token" {
			continue
		}

		if cookie.Value == "" {
			continue
		}

		token = cookie.Value
	}

	return token
}

func RequireRole(role models.UserRole) gin.HandlerFunc {
	return func(c *gin.Context) {
		userRole := c.GetString("role")
		if userRole == "" {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
			return
		}

		if userRole != string(role) && userRole != string(models.RoleAdmin) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Forbidden"})
			return
		}

		c.Next()
	}
}

func RequireManagementAccess() gin.HandlerFunc {
	return func(c *gin.Context) {
		role := c.GetString("role")
		if role == string(models.RolePassthrough) {
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"error": "Forbidden"})
			return
		}
		c.Next()
	}
}
