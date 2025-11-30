package middleware

import (
    "log"
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
                if verbose {
                    log.Printf("PANIC: %v\nRequest: %s %s\nHeaders: %v\nStacktrace:\n%s", r, c.Request.Method, c.Request.URL.String(), c.Request.Header, debug.Stack())
                } else {
                    log.Printf("PANIC: %v", r)
                }
                c.AbortWithStatusJSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
            }
        }()
        c.Next()
    }
}
