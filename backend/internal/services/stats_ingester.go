// Package services provides business logic services for the application.
package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"sync/atomic"
	"time"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"gorm.io/gorm"
)

const (
	channelBufferSize = 1000
	batchSize         = 100
	flushInterval     = 500 * time.Millisecond
)

// StatsIngester receives parsed log entries from LogWatcher via a fan-out channel,
// batches them, and bulk-inserts them into the RequestLog table.
// Client IPs are SHA-256 hashed (first 16 bytes) for GDPR-safe pseudonymisation.
type StatsIngester struct {
	db           *gorm.DB
	ingestCh     chan models.SecurityLogEntry
	droppedCount atomic.Int64 //nolint:govet // atomic.Int64 is properly aligned
	hub          BroadcastHub // optional; nil when WebSocket push not required
}

// NewStatsIngester creates a StatsIngester backed by the provided GORM DB.
func NewStatsIngester(db *gorm.DB) *StatsIngester {
	return &StatsIngester{
		db:       db,
		ingestCh: make(chan models.SecurityLogEntry, channelBufferSize),
	}
}

// Send attempts a non-blocking send of an entry to the ingester channel.
// If the channel buffer is full the entry is dropped and droppedCount is incremented.
func (s *StatsIngester) Send(entry models.SecurityLogEntry) {
	select {
	case s.ingestCh <- entry:
	default:
		s.droppedCount.Add(1)
		logger.Log().Warn("StatsIngester channel full — log entry dropped")
	}
}

// DroppedCount returns the total number of entries dropped due to a full channel.
func (s *StatsIngester) DroppedCount() int64 {
	return s.droppedCount.Load()
}

// RegisterHub wires a BroadcastHub into the ingester for real-time WebSocket push.
// Call this before Run so the hub receives updates as batches are flushed.
func (s *StatsIngester) RegisterHub(h BroadcastHub) {
	s.hub = h
}

// Run processes incoming entries, batching writes to SQLite.
// It exits when ctx is cancelled.
func (s *StatsIngester) Run(ctx context.Context) {
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()

	batch := make([]models.RequestLog, 0, batchSize)

	flush := func() {
		if len(batch) == 0 {
			return
		}
		if err := s.db.CreateInBatches(batch, batchSize).Error; err != nil {
			logger.Log().WithError(err).Error("StatsIngester: failed to flush batch to DB")
		}
		batch = batch[:0]
	}

	for {
		select {
		case <-ctx.Done():
			// Drain any remaining queued entries before exiting.
			for {
				select {
				case entry := <-s.ingestCh:
					batch = append(batch, toRequestLog(entry))
					if len(batch) >= batchSize {
						flush()
					}
				default:
					flush()
					return
				}
			}

		case entry := <-s.ingestCh:
			batch = append(batch, toRequestLog(entry))
			if len(batch) >= batchSize {
				flush()
			}

		case <-ticker.C:
			flush()
		}
	}
}

// Stop drains any remaining entries from the channel and flushes them to the DB.
// It is intended to be called after Run has returned (ctx cancelled).
func (s *StatsIngester) Stop() {
	batch := make([]models.RequestLog, 0, batchSize)

	for {
		select {
		case entry := <-s.ingestCh:
			batch = append(batch, toRequestLog(entry))
			if len(batch) >= batchSize {
				if err := s.db.CreateInBatches(batch, batchSize).Error; err != nil {
					logger.Log().WithError(err).Error("StatsIngester.Stop: failed to flush batch")
				}
				batch = batch[:0]
			}
		default:
			if len(batch) > 0 {
				if err := s.db.CreateInBatches(batch, batchSize).Error; err != nil {
					logger.Log().WithError(err).Error("StatsIngester.Stop: failed to flush final batch")
				}
			}
			return
		}
	}
}

// toRequestLog converts a SecurityLogEntry to a RequestLog for persistence.
func toRequestLog(e models.SecurityLogEntry) models.RequestLog {
	ts, err := time.Parse(time.RFC3339, e.Timestamp)
	if err != nil {
		ts = time.Now()
	}
	return models.RequestLog{
		HostID:       e.Host,
		Timestamp:    ts,
		Method:       e.Method,
		StatusCode:   e.Status,
		BytesSent:    e.Size,
		DurationMs:   int64(e.Duration * 1000),
		ClientIPHash: hashClientIP(e.ClientIP),
	}
}

// hashClientIP returns the first 16 bytes of the SHA-256 hash of rawIP as a hex string.
// This provides GDPR-safe pseudonymisation — the raw IP is never stored.
func hashClientIP(rawIP string) string {
	sum := sha256.Sum256([]byte(rawIP))
	return hex.EncodeToString(sum[:16])
}
