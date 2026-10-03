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

		if IsEmergencyBypass(c) {
			c.Next()
			return
		}

		if HasCallerRole(c) {
			c.Next()
			return
		}

		tokenString, ok := extractAuthToken(c)
		if !ok {
			c.Next()
			return
		}

		user, _, err := authService.AuthenticateToken(tokenString)
		if err != nil {
			c.Next()
			return
		}

		SetCaller(c, user.ID, string(user.Role))
		c.Next()
	}
}
