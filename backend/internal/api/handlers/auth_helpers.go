package handlers

import (
	"net/http"

	"github.com/Wikid82/charon/backend/internal/api/middleware"
	"github.com/gin-gonic/gin"
)

// requireUserID returns the authenticated user's ID recorded by the auth
// middleware. On failure it writes a 401 response itself and returns false;
// callers should return immediately without further writes to c.
func requireUserID(c *gin.Context) (uint, bool) {
	userID, ok := middleware.CallerID(c)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return 0, false
	}
	return userID, true
}
