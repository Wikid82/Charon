package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/coder/websocket"

	"github.com/Wikid82/charon/backend/internal/logger"
)

// logSocketReadLimit caps the size of a single client message.
const logSocketReadLimit = 4096

// logSocketTimers holds the log socket timers. Tests shorten them, and a
// handler goroutine from an earlier test may still be reading them, so the
// whole snapshot is swapped atomically.
type logSocketTimers struct {
	// pongWait is how long a ping may go unanswered before the connection is closed.
	// The pong deadline restarts at each answered ping.
	pongWait time.Duration
	// pingPeriod is how often the server pings the client.
	pingPeriod time.Duration
	// writeWait bounds every data write.
	writeWait time.Duration
}

var logSocketTiming atomic.Pointer[logSocketTimers]

func init() {
	logSocketTiming.Store(&logSocketTimers{
		pongWait:   60 * time.Second,
		pingPeriod: 30 * time.Second,
		writeWait:  10 * time.Second,
	})
}

// currentLogSocketTiming returns the timers in effect for a log socket.
func currentLogSocketTiming() logSocketTimers {
	return *logSocketTiming.Load()
}

// configureLogSocket applies the read limit shared by every log WebSocket.
// Call it right after the upgrade, before watchLogSocket.
func configureLogSocket(conn *websocket.Conn) {
	conn.SetReadLimit(logSocketReadLimit)
}

// watchLogSocket keeps a log WebSocket alive and detects client disconnects.
// It drains client messages so pings and close frames are processed, and it
// pings the client every pingPeriod. The returned channel closes when
// the client disconnects or a ping goes unanswered past its pong deadline. In
// both cases the connection is closed. Cancel ctx when the handler returns.
func watchLogSocket(ctx context.Context, conn *websocket.Conn) <-chan struct{} {
	go keepLogSocketAlive(ctx, conn)

	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			if _, _, err := conn.Read(ctx); err != nil {
				return
			}
		}
	}()
	return done
}

// keepLogSocketAlive sends a ping every pingPeriod. Each ping must be
// answered before the pong deadline, which is pongWait after the last
// answered ping, or after the upgrade if none was answered yet. A missed
// deadline closes the connection. Pongs are read by the reader started in
// watchLogSocket, which is why this runs alongside it.
func keepLogSocketAlive(ctx context.Context, conn *websocket.Conn) {
	ticker := time.NewTicker(currentLogSocketTiming().pingPeriod)
	defer ticker.Stop()

	lastPong := time.Now()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}

		pingCtx, cancel := context.WithDeadline(ctx, lastPong.Add(currentLogSocketTiming().pongWait))
		err := conn.Ping(pingCtx)
		cancel()
		if err != nil {
			if ctx.Err() == nil {
				logger.Log().WithError(err).Debug("Log socket keep-alive failed, closing connection")
				_ = conn.CloseNow()
			}
			return
		}
		lastPong = time.Now()
	}
}

// writeLogSocketText sends data as one text message under the write wait.
func writeLogSocketText(conn *websocket.Conn, data []byte) error {
	ctx, cancel := context.WithTimeout(context.Background(), currentLogSocketTiming().writeWait)
	defer cancel()
	if err := conn.Write(ctx, websocket.MessageText, data); err != nil {
		return fmt.Errorf("write log socket message: %w", err)
	}
	return nil
}

// writeLogSocketJSON sends v as a JSON text message under the write wait.
func writeLogSocketJSON(conn *websocket.Conn, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return fmt.Errorf("marshal log socket message: %w", err)
	}
	return writeLogSocketText(conn, data)
}
