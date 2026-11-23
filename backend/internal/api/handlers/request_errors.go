package handlers

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Wikid82/charon/backend/internal/api/middleware"
	"github.com/Wikid82/charon/backend/internal/util"
)

// respondInternalError answers 500 with publicText and logs the error text, sanitized, with the request logger. The
// answer never carries the error text itself, so internal messages (storage, encryption, driver) stay on the server.
func respondInternalError(c *gin.Context, publicText string, err error) {
	middleware.GetRequestLogger(c).
		WithField("error", util.SanitizeForLog(err.Error())).
		Error(publicText)
	c.JSON(http.StatusInternalServerError, gin.H{"error": publicText})
}
