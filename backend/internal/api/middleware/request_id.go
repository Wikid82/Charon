package middleware

import (
    "github.com/Wikid82/charon/backend/internal/logger"
    "github.com/gin-gonic/gin"
    "github.com/google/uuid"
    "github.com/sirupsen/logrus"
)

const RequestIDKey = "requestID"
const RequestIDHeader = "X-Request-ID"

// RequestID generates a uuid per request and places it in context and header.
func RequestID() gin.HandlerFunc {
    return func(c *gin.Context) {
        rid := uuid.New().String()
        c.Set(RequestIDKey, rid)
        c.Writer.Header().Set(RequestIDHeader, rid)
        // Add to logger fields for this request
        entry := logger.WithFields(map[string]interface{}{"request_id": rid})
        c.Set("logger", entry)
        c.Next()
    }
}

// GetRequestLogger retrieves the request-scoped logger from context or the global logger
func GetRequestLogger(c *gin.Context) *logrus.Entry {
    if v, ok := c.Get("logger"); ok {
        if entry, ok := v.(*logrus.Entry); ok {
            return entry
        }
    }
    // fallback
    return logger.Log()
}
