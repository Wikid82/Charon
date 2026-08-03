package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// requireUserID extracts and type-asserts the authenticated user's ID set
// by the auth middleware (c.Set("userID", uint(...))). On failure it
// writes a 401 response itself and returns false — callers should return
// immediately without further writes to c. Consolidates a pattern
// previously duplicated across GetProfile/UpdateProfile/RegenerateAPIKey
// (and now ChangelogHandler); see auth_handler.go's Me()/ChangePassword
// for other call sites left as-is (out of scope for this DRY pass).
func requireUserID(c *gin.Context) (uint, bool) {
	v, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return 0, false
	}
	userID, ok := v.(uint)
	if !ok {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "Unauthorized"})
		return 0, false
	}
	return userID, true
}
