package leash

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"

	"github.com/coder/websocket"
)

// wsNetConn exposes a WebSocket connection as a net.Conn so that yamux can use
// it as a byte-stream transport. Each Write is sent as one binary message and
// reads stream the payload of incoming data messages.
type wsNetConn struct {
	net.Conn
	ws *websocket.Conn
}

// NewWSNetConn wraps conn as a net.Conn suitable for use with yamux.
func NewWSNetConn(conn *websocket.Conn) net.Conn {
	return &wsNetConn{
		Conn: websocket.NetConn(context.Background(), conn, websocket.MessageBinary),
		ws:   conn,
	}
}

// Close tears the connection down immediately without the WebSocket closing
// handshake. This matches the agent's previous raw-socket close and avoids
// blocking session teardown on a peer that has stopped responding.
func (c *wsNetConn) Close() error {
	return c.ws.CloseNow()
}

// newDialHTTPClient returns the HTTP client used for the WebSocket handshake.
// It dials directly, ignoring proxy environment variables, as the previous
// dialer did, and disables HTTP/2 negotiation because the Upgrade handshake
// requires HTTP/1.1.
func newDialHTTPClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			DialContext:  (&net.Dialer{}).DialContext,
			TLSNextProto: map[string]func(string, *tls.Conn) http.RoundTripper{},
		},
	}
}
