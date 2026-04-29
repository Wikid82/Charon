// Package leash implements the Orthrus agent's reverse WebSocket tunnel.
//
// The Leash maintains a persistent connection from the agent to the Charon server,
// multiplexing Docker socket and TCP port-forward streams over a single WebSocket
// connection using yamux.
package leash

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
	"github.com/hashicorp/yamux"
	"github.com/sirupsen/logrus"

	"github.com/Wikid82/charon/agent/muzzle"
)

const (
	// streamTypeDocker identifies a yamux data stream carrying Docker socket proxy traffic.
	streamTypeDocker = byte(0x01)

	// streamTypePortForward identifies a yamux data stream for TCP port-forwarding.
	streamTypePortForward = byte(0x02)

	reconnectDelayDefault = 5 * time.Second
	reconnectDelayMax     = 60 * time.Second
	reconnectResetAfter   = 60 * time.Second

	wsDialTimeout = 10 * time.Second
)

// Config holds the configuration for a Leash connection.
type Config struct {
	ServerURL    string
	AuthKey      string
	AgentID      string
	DockerSock   string
	Log          *logrus.Logger
	InitialDelay time.Duration // 0 = use default 5s; exposed for testing
}

// Leash manages the reverse WebSocket tunnel to the Charon server.
type Leash struct {
	serverURL    string
	authKey      string
	agentID      string
	dockerSock   string
	log          *logrus.Logger
	filter       *muzzle.Filter
	initialDelay time.Duration
}

// New creates a Leash from cfg.
func New(cfg Config) *Leash {
	d := cfg.InitialDelay
	if d == 0 {
		d = reconnectDelayDefault
	}
	return &Leash{
		serverURL:    cfg.ServerURL,
		authKey:      cfg.AuthKey,
		agentID:      cfg.AgentID,
		dockerSock:   cfg.DockerSock,
		log:          cfg.Log,
		filter:       muzzle.New(),
		initialDelay: d,
	}
}

// Run dials the server and reconnects with exponential backoff on failure.
// It returns when ctx is cancelled.
func (l *Leash) Run(ctx context.Context) error {
	delay := l.initialDelay
	var connectedAt time.Time

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		connectedAt = time.Now()
		err := l.connect(ctx)
		if err != nil && ctx.Err() == nil {
			l.log.WithError(err).WithField("auth_key", "[REDACTED]").Warn("leash: connection lost, will reconnect")
		}

		// Reset backoff if the connection was stable for at least reconnectResetAfter.
		if time.Since(connectedAt) >= reconnectResetAfter {
			delay = l.initialDelay
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(delay):
		}

		delay *= 2
		if delay > reconnectDelayMax {
			delay = reconnectDelayMax
		}
	}
}

// connect performs a single WebSocket dial and yamux session. It blocks until
// the session is closed or ctx is cancelled.
func (l *Leash) connect(ctx context.Context) error {
	dialer := websocket.Dialer{
		HandshakeTimeout: wsDialTimeout,
	}

	l.log.WithField("server_url", l.serverURL).Info("leash: connecting to server")

	wsConn, _, err := dialer.DialContext(ctx, l.serverURL, http.Header{
		"Authorization":     {"Bearer " + l.authKey},
		"X-Orthrus-Version": {"1.0.0"},
		"X-Orthrus-Name":    {l.agentID},
	})
	if err != nil {
		return fmt.Errorf("leash: websocket dial: %w", err)
	}

	cfg := yamux.DefaultConfig()
	cfg.LogOutput = io.Discard

	session, err := yamux.Client(NewWSNetConn(wsConn), cfg)
	if err != nil {
		_ = wsConn.Close()
		return fmt.Errorf("leash: yamux client: %w", err)
	}
	defer session.Close()

	l.log.Info("leash: connected, accepting proxy streams")

	hbCtx, hbCancel := context.WithCancel(ctx)
	defer hbCancel()
	go newHeartbeat(session, l.log).Run(hbCtx)

	for {
		stream, err := session.AcceptStream()
		if err != nil {
			return fmt.Errorf("leash: accept stream: %w", err)
		}
		go l.handleStream(stream)
	}
}

// handleStream reads the leading type byte from a yamux stream and dispatches accordingly.
func (l *Leash) handleStream(stream *yamux.Stream) {
	defer stream.Close()

	typeBuf := make([]byte, 1)
	if _, err := io.ReadFull(stream, typeBuf); err != nil {
		l.log.WithError(err).Error("leash: failed to read stream type byte")
		return
	}

	switch typeBuf[0] {
	case streamTypeDocker:
		l.handleDockerStream(stream)
	case streamTypePortForward:
		l.handlePortForward(stream)
	default:
		l.log.WithField("type_byte", fmt.Sprintf("0x%02x", typeBuf[0])).Warn("leash: unknown stream type, closing")
	}
}

// handleDockerStream proxies the stream to the local Docker socket through the Muzzle filter.
func (l *Leash) handleDockerStream(stream *yamux.Stream) {
	if err := l.filter.ServeProxy(l.dockerSock, stream, stream); err != nil {
		l.log.WithField("error", sanitizeLogField(err.Error())).Debug("leash: docker proxy stream closed")
	}
}

// handlePortForward proxies the stream to a TCP target specified in the stream header.
// Header format: 2-byte big-endian uint16 address length, then the address bytes.
func (l *Leash) handlePortForward(stream *yamux.Stream) {
	lenBuf := make([]byte, 2)
	if _, err := io.ReadFull(stream, lenBuf); err != nil {
		l.log.WithError(err).Error("leash: port forward: failed to read address length")
		return
	}

	addrLen := int(lenBuf[0])<<8 | int(lenBuf[1])
	if addrLen == 0 || addrLen > 255 {
		l.log.WithField("addr_len", addrLen).Error("leash: port forward: invalid address length")
		return
	}

	addrBuf := make([]byte, addrLen)
	if _, err := io.ReadFull(stream, addrBuf); err != nil {
		l.log.WithError(err).Error("leash: port forward: failed to read target address")
		return
	}

	targetAddr := string(addrBuf)

	conn, err := net.Dial("tcp", targetAddr)
	if err != nil {
		l.log.WithError(err).WithField("target", sanitizeLogField(targetAddr)).Error("leash: port forward: dial failed")
		return
	}
	defer conn.Close()

	proxyBidirectional(stream, conn)
}

// sanitizeLogField strips newline and carriage-return characters from a string before
// it is written to a log entry, preventing CWE-117 log injection.
func sanitizeLogField(s string) string {
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\r", `\r`)
	return s
}

// proxyBidirectional copies data between two io.ReadWriters until either side closes.
func proxyBidirectional(a, b io.ReadWriter) {
	done := make(chan struct{}, 2)
	go func() {
		_, _ = io.Copy(b, a)
		done <- struct{}{}
	}()
	go func() {
		_, _ = io.Copy(a, b)
		done <- struct{}{}
	}()
	<-done
}
