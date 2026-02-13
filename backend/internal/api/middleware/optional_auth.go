package middleware

import (
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/gin-gonic/gin"
)

// OptionalAuth applies best-effort authentication for downstream middleware without blocking requests.
func OptionalAuth(authService *services.AuthService) gin.HandlerFunc {
	return func(c *gin.Context) {
		if authService == nil {
			c.Next()
			return
		}

		if bypass, exists := c.Get("emergency_bypass"); exists {
			if bypassActive, ok := bypass.(bool); ok && bypassActive {
				c.Next()
				return
			}
		}

		if _, exists := c.Get("role"); exists {
			c.Next()
			return
		}

		tokenString, ok := extractAuthToken(c)
		if !ok {
			c.Next()
			return
		}

		claims, err := authService.ValidateToken(tokenString)
		if err != nil {
			c.Next()
			return
		}

		user, err := authService.GetUserByID(claims.UserID)
		if err != nil || !user.Enabled {
			c.Next()
			return
		}

		c.Set("userID", user.ID)
		c.Set("role", user.Role)
		c.Next()
	}
}
