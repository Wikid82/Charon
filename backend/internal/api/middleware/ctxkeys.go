package middleware

import "github.com/gin-gonic/gin"

// Context keys written by the authentication middleware. All reads and writes
// of caller identity go through the accessors below so that the key names and
// value types are defined in exactly one place.
const (
	// UserIDKey holds the authenticated caller's ID (uint). The value is 0 when
	// the request was admitted through the emergency path.
	UserIDKey = "userID"
	// RoleKey holds the authenticated caller's role (string).
	RoleKey = "role"
)

// SetCaller records the caller identity on the request context.
func SetCaller(c *gin.Context, userID uint, role string) {
	c.Set(UserIDKey, userID)
	c.Set(RoleKey, role)
}

// CallerID returns the authenticated caller's ID. ok is false when no identity
// is present or the stored value has an unexpected type.
func CallerID(c *gin.Context) (uint, bool) {
	v, exists := c.Get(UserIDKey)
	if !exists {
		return 0, false
	}
	id, ok := v.(uint)
	return id, ok
}

// CallerRole returns the authenticated caller's role, or "" when absent.
func CallerRole(c *gin.Context) string {
	v, exists := c.Get(RoleKey)
	if !exists {
		return ""
	}
	role, _ := v.(string)
	return role
}

// HasCallerRole reports whether a role has been recorded on the context.
func HasCallerRole(c *gin.Context) bool {
	_, exists := c.Get(RoleKey)
	return exists
}
