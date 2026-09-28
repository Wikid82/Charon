package handlers

import (
	"net/http"
	"net/netip"

	"github.com/gin-gonic/gin"

	"github.com/Wikid82/charon/backend/internal/api/middleware"
	"github.com/Wikid82/charon/backend/internal/ratelimit"
	"github.com/Wikid82/charon/backend/internal/util"
)

// LoginProtectionStatusSource supplies the sign-in throttle's admin status.
type LoginProtectionStatusSource interface {
	Status() middleware.AuthRateLimitStatus
}

// LoginProtectionHandler serves the admin-only login-protection status.
type LoginProtectionHandler struct {
	src LoginProtectionStatusSource
}

// NewLoginProtectionHandler returns a handler reading status from src.
func NewLoginProtectionHandler(src LoginProtectionStatusSource) *LoginProtectionHandler {
	return &LoginProtectionHandler{src: src}
}

type loginProtectionResponse struct {
	middleware.AuthRateLimitStatus
	// CallerClientKey is the throttle key Charon derives for this request.
	CallerClientKey string `json:"caller_client_key"`
	// CallerClientScope is loopback, private or public.
	CallerClientScope string `json:"caller_client_scope"`
}

// Get handles GET /api/v1/security/login-protection. It also reports how
// Charon sees the caller's own address, so admins can spot deployments where
// every visitor shares one address.
func (h *LoginProtectionHandler) Get(c *gin.Context) {
	clientIP := c.ClientIP()
	addr, _ := netip.ParseAddr(util.CanonicalizeIPForSecurity(clientIP))
	c.JSON(http.StatusOK, loginProtectionResponse{
		AuthRateLimitStatus: h.src.Status(),
		CallerClientKey:     ratelimit.ClientKey(clientIP),
		CallerClientScope:   string(ratelimit.ClassifyAddr(addr)),
	})
}
