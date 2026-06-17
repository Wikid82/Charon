package handlers

import (
	"context"
	"encoding/json"
	"sync"

	"github.com/gorilla/websocket"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/services"
)

const statsWSWriteBuffer = 64

// statsWSClient represents a single connected WebSocket client.
type statsWSClient struct {
	conn      *websocket.Conn
	send      chan []byte
	closeOnce sync.Once
}

func (c *statsWSClient) close() {
	c.closeOnce.Do(func() {
		close(c.send)
		_ = c.conn.Close()
	})
}

// StatsWSHub manages WebSocket clients for real-time stats push messages.
// It implements services.BroadcastHub.
type StatsWSHub struct {
	mu         sync.RWMutex
	clients    map[*statsWSClient]struct{}
	broadcast  chan services.StatsPushMessage
	register   chan *statsWSClient
	unregister chan *statsWSClient
}

// NewStatsWSHub creates a new StatsWSHub ready to Run.
func NewStatsWSHub() *StatsWSHub {
	return &StatsWSHub{
		clients:    make(map[*statsWSClient]struct{}),
		broadcast:  make(chan services.StatsPushMessage, 64),
		register:   make(chan *statsWSClient, 16),
		unregister: make(chan *statsWSClient, 16),
	}
}

// Broadcast implements services.BroadcastHub.
// It is safe to call from any goroutine; the message is queued non-blocking.
func (h *StatsWSHub) Broadcast(msg services.StatsPushMessage) {
	select {
	case h.broadcast <- msg:
	default:
		logger.Log().Warn("StatsWSHub broadcast channel full — message dropped")
	}
}

// Run processes registrations and broadcasts until ctx is cancelled.
// Start as a goroutine: go hub.Run(ctx)
func (h *StatsWSHub) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			h.mu.Lock()
			for client := range h.clients {
				client.close()
				delete(h.clients, client)
			}
			h.mu.Unlock()
			return

		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = struct{}{}
			h.mu.Unlock()

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				client.close()
			}
			h.mu.Unlock()

		case msg := <-h.broadcast:
			payload, err := json.Marshal(msg)
			if err != nil {
				logger.Log().WithError(err).Error("StatsWSHub: failed to marshal broadcast message")
				continue
			}

			h.mu.RLock()
			for client := range h.clients {
				select {
				case client.send <- payload:
				default:
					// Non-blocking: drop to slow clients.
					logger.Log().Warn("StatsWSHub: slow client dropped message")
				}
			}
			h.mu.RUnlock()
		}
	}
}

// ServeClient upgrades c to WebSocket and registers it with the hub.
// It blocks until the client disconnects. Call from a Gin handler.
func (h *StatsWSHub) ServeClient(c *statsWSClient) {
	h.register <- c

	// Writer goroutine: forwards hub messages to the WebSocket connection.
	go func() {
		for payload := range c.send {
			if err := c.conn.WriteMessage(websocket.TextMessage, payload); err != nil {
				break
			}
		}
		_ = c.conn.Close()
	}()

	// Reader loop: detects client disconnect.
	for {
		if _, _, err := c.conn.ReadMessage(); err != nil {
			break
		}
	}

	h.unregister <- c
}
