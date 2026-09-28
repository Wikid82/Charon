package handlers

import "github.com/gin-gonic/gin"

// PasswordAttemptGuard charges one account-password verification against the
// caller's sign-in budget. It returns false after writing the 429 response.
// Implemented by *middleware.AuthRateLimiter.
type PasswordAttemptGuard interface {
	AllowPasswordAttempt(c *gin.Context) bool
}

// allowPasswordAttempt applies guard when one is configured; a nil guard allows.
func allowPasswordAttempt(guard PasswordAttemptGuard, c *gin.Context) bool {
	return guard == nil || guard.AllowPasswordAttempt(c)
}
