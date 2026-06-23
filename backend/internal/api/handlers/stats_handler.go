package handlers

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"

	"github.com/Wikid82/charon/backend/internal/services"
)

const (
	statsDefaultLimit  = 10
	statsMaxLimit      = 50
	statsWithinDaysDef = 30
)

var (
	validStatsPeriods = map[string]struct{}{"24h": {}, "7d": {}, "30d": {}}
	validStatsBuckets = map[string]struct{}{"1h": {}, "6h": {}, "1d": {}}
)

// StatsHandler handles REST endpoints for the enhanced dashboard statistics API.
type StatsHandler struct {
	svc      *services.StatsService
	ingester *services.StatsIngester // may be nil when health endpoint not needed
	hub      *StatsWSHub             // may be nil when WebSocket not needed
}

// NewStatsHandler creates a StatsHandler without an ingester or hub reference.
func NewStatsHandler(svc *services.StatsService) *StatsHandler {
	return &StatsHandler{svc: svc}
}

// NewStatsHandlerWithIngester creates a StatsHandler that can also serve the health endpoint.
func NewStatsHandlerWithIngester(svc *services.StatsService, ingester *services.StatsIngester) *StatsHandler {
	return &StatsHandler{svc: svc, ingester: ingester}
}

// NewStatsHandlerFull creates a fully wired StatsHandler with ingester and hub.
func NewStatsHandlerFull(svc *services.StatsService, ingester *services.StatsIngester, hub *StatsWSHub) *StatsHandler {
	return &StatsHandler{svc: svc, ingester: ingester, hub: hub}
}

// GetStatsSummary returns request counts for the last 24h, 7d, and 30d.
// GET /api/stats/summary
func (h *StatsHandler) GetStatsSummary(c *gin.Context) {
	summary, err := h.svc.GetSummary(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Errorf("GetStatsSummary: %w", err).Error()})
		return
	}
	c.JSON(http.StatusOK, summary)
}

// GetTopHosts returns the top N hosts by request count over the given period.
// GET /api/stats/top-hosts?period=24h&limit=10
func (h *StatsHandler) GetTopHosts(c *gin.Context) {
	period := c.DefaultQuery("period", "24h")
	if _, ok := validStatsPeriods[period]; !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "period must be one of: 24h, 7d, 30d"})
		return
	}

	limit := statsDefaultLimit
	if raw := c.Query("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err == nil && parsed > 0 {
			limit = parsed
		}
	}
	if limit > statsMaxLimit {
		limit = statsMaxLimit
	}

	results, err := h.svc.GetTopHosts(c.Request.Context(), period, limit)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Errorf("GetTopHosts: %w", err).Error()})
		return
	}
	c.JSON(http.StatusOK, results)
}

// GetStatusDistribution returns HTTP status code counts over the given period.
// GET /api/stats/status-distribution?period=24h
func (h *StatsHandler) GetStatusDistribution(c *gin.Context) {
	period := c.DefaultQuery("period", "24h")
	if _, ok := validStatsPeriods[period]; !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "period must be one of: 24h, 7d, 30d"})
		return
	}

	results, err := h.svc.GetStatusDistribution(c.Request.Context(), period)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Errorf("GetStatusDistribution: %w", err).Error()})
		return
	}
	c.JSON(http.StatusOK, results)
}

// GetTrafficVolume returns bytes sent bucketed by time interval.
// GET /api/stats/traffic-volume?bucket=1h
func (h *StatsHandler) GetTrafficVolume(c *gin.Context) {
	bucket := c.DefaultQuery("bucket", "1h")
	if _, ok := validStatsBuckets[bucket]; !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bucket must be one of: 1h, 6h, 1d"})
		return
	}

	results, err := h.svc.GetTrafficVolume(c.Request.Context(), bucket)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Errorf("GetTrafficVolume: %w", err).Error()})
		return
	}
	c.JSON(http.StatusOK, results)
}

// GetCertExpiry returns certificates expiring within the given number of days.
// GET /api/stats/cert-expiry?within_days=30
func (h *StatsHandler) GetCertExpiry(c *gin.Context) {
	withinDays := statsWithinDaysDef
	if raw := c.Query("within_days"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "within_days must be an integer between 1 and 365"})
			return
		}
		withinDays = parsed
	}

	if withinDays < 1 || withinDays > 365 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "within_days must be between 1 and 365"})
		return
	}

	results, err := h.svc.GetCertExpiry(c.Request.Context(), withinDays)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Errorf("GetCertExpiry: %w", err).Error()})
		return
	}
	c.JSON(http.StatusOK, results)
}

// GetRequests is an alias for GetTrafficVolume returning per-bucket request counts.
// GET /api/stats/requests?bucket=1h
func (h *StatsHandler) GetRequests(c *gin.Context) {
	bucket := c.DefaultQuery("bucket", "1h")
	if _, ok := validStatsBuckets[bucket]; !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "bucket must be one of: 1h, 6h, 1d"})
		return
	}

	results, err := h.svc.GetTrafficVolume(c.Request.Context(), bucket)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Errorf("GetRequests: %w", err).Error()})
		return
	}
	c.JSON(http.StatusOK, results)
}

// GetStatsHealth returns the ingester health including the number of dropped log entries.
// GET /api/stats/health
func (h *StatsHandler) GetStatsHealth(c *gin.Context) {
	var dropped int64
	if h.ingester != nil {
		dropped = h.ingester.DroppedCount()
	}
	c.JSON(http.StatusOK, gin.H{"dropped_count": dropped})
}

// StatsWS upgrades the connection to WebSocket and streams stats push messages.
// GET /api/stats/ws
func (h *StatsHandler) StatsWS(c *gin.Context) {
	conn, err := upgrader.Upgrade(c.Writer, c.Request, nil) // nosemgrep: go.gorilla.security.audit.websocket-missing-origin-check.websocket-missing-origin-check
	if err != nil {
		return
	}

	if h.hub == nil {
		// Hub not wired — close immediately.
		_ = conn.Close()
		return
	}

	client := &statsWSClient{
		conn: conn,
		send: make(chan []byte, statsWSWriteBuffer),
	}
	h.hub.ServeClient(client)
}
