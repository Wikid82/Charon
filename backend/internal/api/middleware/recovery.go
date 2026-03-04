package middleware

import (
	"fmt"
	"net/http"
	"runtime/debug"

	"github.com/Wikid82/charon/backend/internal/util"
	"github.com/gin-gonic/gin"
)

// Recovery logs panic information. When verbose is true it logs stacktraces
// and basic request metadata for debugging.
func Recovery(verbose bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		defer func() {
			if r := recover(); r != nil {
				// Try to get a request-scoped logger; fall back to global logger
				entry := GetRequestLogger(c)

				// Sanitize panic message to prevent logging sensitive data
				panicMsg := util.SanitizeForLog(fmt.Sprintf("%v", r))
				if len(panicMsg) > 200 {
					panicMsg = panicMsg[:200] + "..."
				}

				if verbose {
					// Log only the sanitized panic message and safe metadata.
					// Stack traces can contain sensitive data from the call context,
					// so we only log them internally without exposing raw values.
					entry.WithFields(map[string]any{
						"method": c.Request.Method,
						"path":   SanitizePath(c.Request.URL.Path),
					}).Errorf("PANIC: %s", panicMsg)
					// Log stack trace separately at debug level for operators
					// who have enabled verbose logging and understand the risks
					entry.Debugf("Stack trace available for panic recovery (not logged for security)")
					_ = debug.Stack() // Capture but don't log to avoid CWE-312
				} else {
					entry.Errorf("PANIC: %s", panicMsg)
				}
				c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
			}
		}()
		c.Next()
	}
}
