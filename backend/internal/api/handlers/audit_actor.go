package handlers

import (
	"fmt"

	"github.com/Wikid82/charon/backend/internal/api/middleware"
	"github.com/gin-gonic/gin"
)

// auditActor returns the identity recorded as the actor of an audit entry:
// "user:<id>" for a signed-in user, "emergency" for the emergency path (which
// has no user record), and the client address when no identity is present.
func auditActor(c *gin.Context) string {
	id, ok := middleware.CallerID(c)
	switch {
	case !ok:
		return c.ClientIP()
	case id == 0:
		return "emergency"
	default:
		return fmt.Sprintf("user:%d", id)
	}
}
