package handlers

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"

	"github.com/coder/websocket"
	"github.com/gin-gonic/gin"
)

// errWebSocketOriginDenied is returned by upgradeWebSocket when the request
// Origin does not match the request host.
var errWebSocketOriginDenied = errors.New("websocket: request origin not allowed")

// isAllowedWebSocketOrigin reports whether a browser WebSocket request may
// upgrade. Requests without an Origin header are non-browser or same-origin
// clients and are allowed. Otherwise the Origin host must equal the request
// host, or X-Forwarded-Host when a reverse proxy sets it.
func isAllowedWebSocketOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	originURL, err := url.Parse(origin)
	if err != nil {
		return false
	}
	requestHost := r.Host
	if forwardedHost := r.Header.Get("X-Forwarded-Host"); forwardedHost != "" {
		requestHost = forwardedHost
	}
	return originURL.Host == requestHost
}

// upgradeWebSocket upgrades an HTTP connection to WebSocket after the origin
// check. All WS handlers in this package MUST go through this helper rather
// than calling websocket.Accept directly, so the origin check stays visible to
// single-file static analysis at every call site.
//
// The origin check is done here rather than by websocket.Accept, which only
// knows about the request Host and cannot honour X-Forwarded-Host. Accept's own
// check is therefore disabled with InsecureSkipVerify, and this function
// rejects disallowed origins with 403 before Accept runs.
func upgradeWebSocket(c *gin.Context) (*websocket.Conn, error) {
	if !isAllowedWebSocketOrigin(c.Request) {
		http.Error(c.Writer, http.StatusText(http.StatusForbidden), http.StatusForbidden)
		return nil, errWebSocketOriginDenied
	}
	conn, err := websocket.Accept(c.Writer, c.Request, &websocket.AcceptOptions{InsecureSkipVerify: true})
	if err != nil {
		return nil, fmt.Errorf("upgrade websocket: %w", err)
	}
	return conn, nil
}
