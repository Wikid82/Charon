package middleware

import (
    "net/http"
    "runtime/debug"

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
                if verbose {
                    entry.WithFields(map[string]interface{}{
                        "method":  c.Request.Method,
                        "path":    SanitizePath(c.Request.URL.Path),
                        "headers": SanitizeHeaders(c.Request.Header),
                    }).Errorf("PANIC: %v\nStacktrace:\n%s", r, debug.Stack())
                } else {
                    entry.Errorf("PANIC: %v", r)
                }
                c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
            }
        }()
        c.Next()
    }
}
