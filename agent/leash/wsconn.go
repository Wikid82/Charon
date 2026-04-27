package leash

import (
	"io"
	"net"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// wsNetConn adapts a gorilla WebSocket connection to the net.Conn interface
// so that yamux can use it as a byte-stream transport.
// gorilla websocket supports one concurrent reader and one concurrent writer;
// rMu serialises reads and wMu serialises writes accordingly.
type wsNetConn struct {
	conn   *websocket.Conn
	rMu    sync.Mutex
	wMu    sync.Mutex
	reader io.Reader
}

// NewWSNetConn wraps conn as a net.Conn suitable for use with yamux.
func NewWSNetConn(conn *websocket.Conn) net.Conn {
	return &wsNetConn{conn: conn}
}

func (c *wsNetConn) Read(b []byte) (int, error) {
	c.rMu.Lock()
	defer c.rMu.Unlock()

	for {
		if c.reader != nil {
			n, err := c.reader.Read(b)
			if err == io.EOF {
				c.reader = nil
				continue
			}
			return n, err
		}

		_, msg, err := c.conn.NextReader()
		if err != nil {
			return 0, err
		}
		c.reader = msg
	}
}

func (c *wsNetConn) Write(b []byte) (int, error) {
	c.wMu.Lock()
	defer c.wMu.Unlock()

	if err := c.conn.WriteMessage(websocket.BinaryMessage, b); err != nil {
		return 0, err
	}
	return len(b), nil
}

func (c *wsNetConn) Close() error {
	return c.conn.Close()
}

func (c *wsNetConn) LocalAddr() net.Addr  { return c.conn.NetConn().LocalAddr() }
func (c *wsNetConn) RemoteAddr() net.Addr { return c.conn.NetConn().RemoteAddr() }

func (c *wsNetConn) SetDeadline(t time.Time) error {
	if err := c.conn.SetReadDeadline(t); err != nil {
		return err
	}
	return c.conn.SetWriteDeadline(t)
}

func (c *wsNetConn) SetReadDeadline(t time.Time) error {
	return c.conn.SetReadDeadline(t)
}

func (c *wsNetConn) SetWriteDeadline(t time.Time) error {
	return c.conn.SetWriteDeadline(t)
}
