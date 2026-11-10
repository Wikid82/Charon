package middleware

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/util"
)

// crossOriginProtection is the standard library's cross-origin check. It has no trusted-origin list, so only same-origin
// and non-browser requests pass for unsafe methods.
var crossOriginProtection = http.NewCrossOriginProtection()

// RequireSameOriginForBrowserAuth refuses cross-origin requests that a browser would send with ambient credentials. A
// request that carries an Authorization header is header-authenticated, so it passes unchanged: a page on another
// origin cannot add that header without a CORS preflight, which Charon does not grant. Any other request is checked
// with the standard library's Sec-Fetch-Site and Origin rules. Safe methods and non-browser clients pass. A refused
// request gets 403 cross_origin_refused before the handler runs.
//
// SameSite cookies alone do not cover same-site sibling origins, so mutating routes that accept cookie sessions use
// this check.
func RequireSameOriginForBrowserAuth() gin.HandlerFunc {
	return func(c *gin.Context) {
		if len(c.Request.Header.Values("Authorization")) > 0 {
			c.Next()
			return
		}
		if err := crossOriginProtection.Check(c.Request); err != nil {
			logger.Log().
				WithField("sec_fetch_site", util.SanitizeForLog(c.Request.Header.Get("Sec-Fetch-Site"))).
				WithField("origin", util.SanitizeForLog(c.Request.Header.Get("Origin"))).
				Debug("Refused cross-origin request to a cookie-authenticated route")
			c.AbortWithStatusJSON(http.StatusForbidden, gin.H{
				"error":      "Cross-origin request refused",
				"error_code": "cross_origin_refused",
			})
			return
		}
		c.Next()
	}
}
