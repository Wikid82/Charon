package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/gin-gonic/gin"
)

// minMonitorIntervalSeconds mirrors the service-side hard floor
// (services.clampInterval); requests below it are rejected at the edge.
const minMonitorIntervalSeconds = 30

// Summary endpoint sparkline bounds (spec §3.5.1, user decision: default 30,
// cap 60). The service clamps too; the handler clamps first for a predictable
// response shape.
const (
	summaryDefaultBeats = 30
	summaryMaxBeats     = 60
)

type UptimeHandler struct {
	service *services.UptimeService
	summary *services.UptimeSummaryService
}

func NewUptimeHandler(service *services.UptimeService) *UptimeHandler {
	var summary *services.UptimeSummaryService
	if service != nil {
		summary = services.NewUptimeSummaryService(service.DB)
	}
	return &UptimeHandler{service: service, summary: summary}
}

func (h *UptimeHandler) List(c *gin.Context) {
	monitors, err := h.service.ListMonitors()
	if err != nil {
		logger.Log().WithError(err).Error("Failed to list uptime monitors")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list monitors"})
		return
	}
	c.JSON(http.StatusOK, monitors)
}

// CreateMonitorRequest represents the JSON payload for creating a new monitor
type CreateMonitorRequest struct {
	Name       string `json:"name" binding:"required"`
	URL        string `json:"url" binding:"required"`
	Type       string `json:"type" binding:"required,oneof=http tcp https"`
	Interval   int    `json:"interval"`
	MaxRetries int    `json:"max_retries"`
}

// Create creates a new uptime monitor
func (h *UptimeHandler) Create(c *gin.Context) {
	var req CreateMonitorRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Log().WithError(err).Warn("Invalid JSON payload for monitor creation")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// A positive but sub-floor interval is a client error. Zero is allowed —
	// CreateMonitor resolves it to the configured default at write time.
	if req.Interval > 0 && req.Interval < minMonitorIntervalSeconds {
		c.JSON(http.StatusBadRequest, gin.H{"error": "interval must be at least 30 seconds"})
		return
	}

	monitor, err := h.service.CreateMonitor(req.Name, req.URL, req.Type, req.Interval, req.MaxRetries)
	if err != nil {
		logger.Log().WithError(err).Error("Failed to create uptime monitor")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, monitor)
}

func (h *UptimeHandler) GetHistory(c *gin.Context) {
	id := c.Param("id")

	// limit: non-positive / unparseable -> service default (60); the service
	// also enforces the 500 hard cap (spec §3.5.4).
	limit, err := strconv.Atoi(c.DefaultQuery("limit", "0"))
	if err != nil {
		limit = 0
	}

	// before: optional RFC3339 "load older" cursor -> created_at < before.
	var before time.Time
	if raw := c.Query("before"); raw != "" {
		parsed, perr := time.Parse(time.RFC3339, raw)
		if perr != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "before must be an RFC3339 timestamp"})
			return
		}
		before = parsed
	}

	history, err := h.service.GetMonitorHistory(id, limit, before)
	if err != nil {
		logger.Log().WithField("error", sanitizeForLog(err.Error())).WithField("monitor_id", sanitizeForLog(id)).Error("Failed to get monitor history")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get history"})
		return
	}
	c.JSON(http.StatusOK, history)
}

// Summary returns the batch per-monitor dashboard payload (status, latency,
// last check, 24h uptime %, and a recent-beat sparkline) for every monitor in
// one response, from three cached windowed queries (spec §3.5).
// GET /api/v1/uptime/monitors/summary?beats=<1..60>
func (h *UptimeHandler) Summary(c *gin.Context) {
	beats, err := strconv.Atoi(c.DefaultQuery("beats", strconv.Itoa(summaryDefaultBeats)))
	if err != nil {
		beats = summaryDefaultBeats
	}
	if beats < 1 {
		beats = 1
	}
	if beats > summaryMaxBeats {
		beats = summaryMaxBeats
	}

	summaries, err := h.summary.GetSummary(c.Request.Context(), beats)
	if err != nil {
		logger.Log().WithError(err).Error("Failed to build uptime summary")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to build summary"})
		return
	}
	c.JSON(http.StatusOK, summaries)
}

// Health exposes the uptime write/execution pipeline back-pressure counters
// (spec §3.5.5). Safe to call before the pool/ingester Run loops start, and in
// unit-test construction where either may be nil -> the field reports 0.
// GET /api/v1/uptime/health
func (h *UptimeHandler) Health(c *gin.Context) {
	var heartbeatsDropped, checksEnqueueDropped int64
	var queueDepth, workerPoolSize int

	if h.service != nil {
		if h.service.Ingester != nil {
			heartbeatsDropped = h.service.Ingester.DroppedCount()
		}
		if h.service.Pool != nil {
			checksEnqueueDropped = h.service.Pool.EnqueueDropped()
			queueDepth = h.service.Pool.QueueDepth()
			workerPoolSize = h.service.Pool.WorkerPoolSize()
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"heartbeats_dropped":     heartbeatsDropped,
		"checks_enqueue_dropped": checksEnqueueDropped,
		"queue_depth":            queueDepth,
		"worker_pool_size":       workerPoolSize,
	})
}

func (h *UptimeHandler) Update(c *gin.Context) {
	id := c.Param("id")
	var updates map[string]any
	if err := c.ShouldBindJSON(&updates); err != nil {
		logger.Log().WithField("error", sanitizeForLog(err.Error())).WithField("monitor_id", sanitizeForLog(id)).Warn("Invalid JSON payload for monitor update")
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	monitor, err := h.service.UpdateMonitor(id, updates)
	if err != nil {
		if errors.Is(err, services.ErrIntervalTooLow) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		logger.Log().WithField("error", sanitizeForLog(err.Error())).WithField("monitor_id", sanitizeForLog(id)).Error("Failed to update monitor")
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, monitor)
}

func (h *UptimeHandler) Sync(c *gin.Context) {
	if err := h.service.SyncMonitors(); err != nil {
		logger.Log().WithError(err).Error("Failed to sync uptime monitors")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to sync monitors"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Sync started"})
}

// Delete removes a monitor and its associated data
func (h *UptimeHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	if err := h.service.DeleteMonitor(id); err != nil {
		logger.Log().WithField("error", sanitizeForLog(err.Error())).WithField("monitor_id", sanitizeForLog(id)).Error("Failed to delete monitor")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete monitor"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Monitor deleted"})
}

// CheckMonitor triggers an immediate check for a specific monitor. With a live
// worker pool it enqueues the job with a short block, returning 503 if the queue
// stays full (spec §3.1.2 / N5); without a pool it falls back to a background
// inline check.
func (h *UptimeHandler) CheckMonitor(c *gin.Context) {
	id := c.Param("id")
	monitor, err := h.service.GetMonitorByID(id)
	if err != nil {
		logger.Log().WithField("error", sanitizeForLog(err.Error())).WithField("monitor_id", sanitizeForLog(id)).Warn("Monitor not found for check")
		c.JSON(http.StatusNotFound, gin.H{"error": "Monitor not found"})
		return
	}

	if h.service.Pool != nil {
		if enqErr := h.service.Pool.Enqueue(c.Request.Context(), services.UptimeJob{
			Kind:    services.JobMonitorCheck,
			Monitor: *monitor,
			Manual:  true,
		}); enqErr != nil {
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "check queue is full, try again"})
			return
		}
		c.JSON(http.StatusOK, gin.H{"message": "Check enqueued"})
		return
	}

	// Trigger immediate check in background
	go h.service.CheckMonitor(*monitor)

	c.JSON(http.StatusOK, gin.H{"message": "Check triggered"})
}
