package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/coder/websocket"
	"github.com/gin-gonic/gin"

	"github.com/Wikid82/charon/backend/internal/api/middleware"
	"github.com/Wikid82/charon/backend/internal/security"
)

// errWebSocketOriginDenied is returned by upgradeWebSocket when the request
// Origin is neither the request host nor the configured public origin.
var errWebSocketOriginDenied = errors.New("websocket: request origin not allowed")

// PublicOriginSource yields the configured public origin ("scheme://host[:port]"), or "" when none is set. The browser
// origin guard implements it from its last rebuild, so a WebSocket handshake never reads the database.
type PublicOriginSource interface {
	TrustedOrigin() string
}

// isAllowedWebSocketOrigin reports whether a browser WebSocket request may
// upgrade. Requests without an Origin header are non-browser or same-origin
// clients and are allowed. Otherwise the Origin must equal the configured
// public origin exactly, or its host must equal the host the client addressed.
// That host is the verified self-proxy hop's host or, from a trusted proxy only,
// the first X-Forwarded-Host entry (see middleware.ForwardedRequest); otherwise
// the request Host. The public origin covers a proxy that rewrites Host, so its
// browser handshake is not refused.
func isAllowedWebSocketOrigin(c *gin.Context, publicOrigin string, trusted security.TrustedProxyMatcher) bool {
	origin := c.Request.Header.Get("Origin")
	if origin == "" {
		return true
	}
	if publicOrigin != "" && strings.EqualFold(origin, publicOrigin) {
		return true
	}
	originURL, err := url.Parse(origin)
	if err != nil {
		return false
	}
	requestHost := c.Request.Host
	if _, hosts := middleware.ForwardedRequest(c, trusted); len(hosts) > 0 {
		requestHost = hosts[0]
	}
	return originURL.Host == requestHost
}

// upgradeWebSocket upgrades an HTTP connection to WebSocket after the origin
// check. All WS handlers in this package MUST go through this helper rather
// than calling websocket.Accept directly, so the origin check stays visible to
// single-file static analysis at every call site. publicOrigins may be nil,
// which trusts no public origin. trusted is the API-layer trusted-proxy list
// that decides whether forwarded host headers are honoured.
//
// The origin check is done here rather than by websocket.Accept, which only
// knows about the request Host and cannot honour forwarded hosts. Accept's own
// check is therefore disabled with InsecureSkipVerify, and this function
// rejects disallowed origins with 403 before Accept runs.
func upgradeWebSocket(c *gin.Context, publicOrigins PublicOriginSource, trusted security.TrustedProxyMatcher) (*websocket.Conn, error) {
	publicOrigin := ""
	if publicOrigins != nil {
		publicOrigin = publicOrigins.TrustedOrigin()
	}
	if !isAllowedWebSocketOrigin(c, publicOrigin, trusted) {
		http.Error(c.Writer, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return nil, errWebSocketOriginDenied
	}
	conn, err := websocket.Accept(c.Writer, c.Request, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		return nil, fmt.Errorf("upgrade websocket: %w", err)
	}
	return conn, nil
}
