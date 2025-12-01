package middleware

import (
    "time"
    "github.com/Wikid82/charon/backend/internal/util"

    "github.com/gin-gonic/gin"
)

// RequestLogger logs basic request information along with the request_id.
func RequestLogger() gin.HandlerFunc {
    return func(c *gin.Context) {
        start := time.Now()
        c.Next()
        latency := time.Since(start)
        entry := GetRequestLogger(c)
        entry.WithFields(map[string]interface{}{
            "status":  c.Writer.Status(),
            "method":  c.Request.Method,
            "path":    SanitizePath(c.Request.URL.Path),
            "latency": latency.String(),
            "client":  util.SanitizeForLog(c.ClientIP()),
        }).Info("handled request")
    }
}
