package services

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/Wikid82/charon/backend/internal/models"
	"gorm.io/gorm"
)

const (
	// uptimeSummaryCacheTTL is how long a computed batch summary is served from
	// memory before the three windowed queries are re-run. Mirrors
	// stats_service.go's summaryCacheTTL (kept as a distinct name to avoid
	// colliding with that package-level const).
	uptimeSummaryCacheTTL = 30 * time.Second

	// uptimeSummaryMaxBeats caps ?beats= (user decision: default 30, cap 60).
	// The cache always computes at this width; GetSummary slices the tail down
	// to the requested count, so every request is served from one cache entry.
	uptimeSummaryMaxBeats = 60

	// uptimeSummaryWindow bounds every heartbeat read (recent beats + 24h
	// uptime) to the trailing 24 hours. Both queries are per-monitor range
	// seeks on idx_heartbeat_monitor_created (monitor_id, created_at), so their
	// cost follows monitors x window, not the retained history. Until the
	// pruner's deferred index exists they remain correct and the refresh total
	// is no slower than before (spec §3.5.6 / R7, GH #1441).
	uptimeSummaryWindow = 24 * time.Hour

	// uptimeMonitorScanLimit is a defensive ceiling on the monitor metadata
	// query. The product tops out at 500 monitors (spec §1.2).
	uptimeMonitorScanLimit = 500
)

// BeatDTO is one heartbeat as exposed on the batch summary sparkline series.
type BeatDTO struct {
	Status    string    `json:"status"`
	Latency   int64     `json:"latency"`
	CreatedAt time.Time `json:"created_at"`
}

// MonitorSummary is the per-monitor row returned by
// GET /api/v1/uptime/monitors/summary. All nullable columns are pointers so the
// JSON renders explicit null rather than a zero value.
type MonitorSummary struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	Type           string     `json:"type"`
	URL            string     `json:"url"`
	Enabled        bool       `json:"enabled"`
	Status         string     `json:"status"`
	Latency        int64      `json:"latency"`
	LastCheck      *time.Time `json:"last_check"`
	Interval       int        `json:"interval"`
	MaxRetries     int        `json:"max_retries"`
	ProxyHostID    *uint      `json:"proxy_host_id"`
	RemoteServerID *uint      `json:"remote_server_id"`
	Uptime24h      *float64   `json:"uptime_24h"`
	RecentBeats    []BeatDTO  `json:"recent_beats"`
}

// uptimeSummaryCache holds the last computed summary (always at
// uptimeSummaryMaxBeats width) with an expiry. Shape ported from
// stats_service.go's summaryCache.
type uptimeSummaryCache struct {
	mu        sync.Mutex
	value     []MonitorSummary
	expiresAt time.Time
}

func (c *uptimeSummaryCache) get() ([]MonitorSummary, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.value) > 0 && time.Now().Before(c.expiresAt) {
		return c.value, true
	}
	return nil, false
}

func (c *uptimeSummaryCache) set(v []MonitorSummary, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.value = v
	c.expiresAt = time.Now().Add(ttl)
}

// UptimeSummaryService answers the batch summary endpoint with exactly three
// SQL queries regardless of monitor count, behind a 30s TTL cache.
type UptimeSummaryService struct {
	db    *gorm.DB
	cache uptimeSummaryCache

	// now is an injectable clock (test seam, matches uptimeConfig.now). It
	// defines the trailing-24h window edge.
	now func() time.Time
}

// NewUptimeSummaryService builds the service bound to db. It only needs a DB
// handle, so it takes one directly (siblings that need pool state take *pool).
func NewUptimeSummaryService(db *gorm.DB) *UptimeSummaryService {
	return &UptimeSummaryService{db: db, now: time.Now}
}

// clampBeats forces the requested sparkline length into [1, uptimeSummaryMaxBeats].
func clampBeats(beats int) int {
	if beats < 1 {
		return 1
	}
	if beats > uptimeSummaryMaxBeats {
		return uptimeSummaryMaxBeats
	}
	return beats
}

// GetSummary returns one MonitorSummary per monitor (ordered by name ASC), each
// carrying up to beats recent heartbeats (chronological ASC) and the 24h uptime
// percentage. Results are cached for uptimeSummaryCacheTTL; the cache stores the
// full uptimeSummaryMaxBeats-wide series and this call slices the tail.
func (s *UptimeSummaryService) GetSummary(ctx context.Context, beats int) ([]MonitorSummary, error) {
	beats = clampBeats(beats)

	if cached, ok := s.cache.get(); ok {
		return sliceSummaries(cached, beats), nil
	}

	windowStart := s.now().Add(-uptimeSummaryWindow)

	monitors, err := s.loadMonitors(ctx)
	if err != nil {
		return nil, fmt.Errorf("GetSummary: load monitors: %w", err)
	}

	beatsByMonitor, err := s.loadRecentBeats(ctx, windowStart)
	if err != nil {
		return nil, fmt.Errorf("GetSummary: load recent beats: %w", err)
	}

	uptimeByMonitor, err := s.loadUptime24h(ctx, windowStart)
	if err != nil {
		return nil, fmt.Errorf("GetSummary: load 24h uptime: %w", err)
	}

	full := assembleSummaries(monitors, beatsByMonitor, uptimeByMonitor)
	s.cache.set(full, uptimeSummaryCacheTTL)
	return sliceSummaries(full, beats), nil
}

// loadMonitors runs query 1/3: monitor metadata, name-ordered.
func (s *UptimeSummaryService) loadMonitors(ctx context.Context) ([]models.UptimeMonitor, error) {
	var monitors []models.UptimeMonitor
	err := s.db.WithContext(ctx).
		Model(&models.UptimeMonitor{}).
		Order("name ASC").
		Limit(uptimeMonitorScanLimit).
		Find(&monitors).Error
	return monitors, err
}

type recentBeatRow struct {
	MonitorID string
	Status    string
	Latency   int64
	CreatedAt time.Time
}

// recentBeatsSQL is query 2/3: per monitor, the uptimeSummaryMaxBeats most
// recent heartbeats inside the trailing 24h window, emitted oldest-first.
// Each monitor's top-N is one index range read (newest first, stop at the cap)
// on idx_heartbeat_monitor_created, the pruner's deferred index; the window
// function this replaced had to rank every retained heartbeat (GH #1441).
// Without that index the query is still correct, and the refresh total is
// still no slower than before, but the ranking step needs a temp b-tree.
//
// The monitor subset is the one loadMonitors returns. id is the tie-breaker
// for heartbeats sharing a created_at, so the same rows are chosen and ordered
// in every index state. Args: monitor cap, window edge, per-monitor cap; fully
// parameterised, no string interpolation.
const recentBeatsSQL = `
SELECT h.monitor_id, h.status, h.latency, h.created_at
FROM (SELECT id FROM uptime_monitors ORDER BY name LIMIT ?) m
JOIN uptime_heartbeats h ON h.id IN (
  SELECT id FROM uptime_heartbeats
  WHERE monitor_id = m.id AND created_at >= ?
  ORDER BY created_at DESC, id DESC
  LIMIT ?
)
ORDER BY h.monitor_id, h.created_at ASC, h.id ASC`

func (s *UptimeSummaryService) loadRecentBeats(ctx context.Context, windowStart time.Time) (map[string][]BeatDTO, error) {
	var rows []recentBeatRow
	if err := s.db.WithContext(ctx).
		Raw(recentBeatsSQL, uptimeMonitorScanLimit, windowStart, uptimeSummaryMaxBeats).
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	out := make(map[string][]BeatDTO, len(rows))
	for _, r := range rows {
		out[r.MonitorID] = append(out[r.MonitorID], BeatDTO{
			Status:    r.Status,
			Latency:   r.Latency,
			CreatedAt: r.CreatedAt,
		})
	}
	return out, nil
}

type uptime24hRow struct {
	MonitorID string
	Total     int64
	Up        int64
}

// uptime24hSQL is query 3/3: per monitor, the number of heartbeats and of "up"
// heartbeats in the trailing 24h window, for the same monitor subset as
// recentBeatsSQL. Two correlated counts are used on purpose: each is a
// covering range seek (idx_heartbeat_monitor_created for the total,
// idx_heartbeat_lookup for the "up" count) and measured several times faster
// than any single-pass aggregate, which needs a row lookup per in-window
// heartbeat to read status. Args: window edge (total), window edge (up),
// monitor cap; fully parameterised, no string interpolation.
const uptime24hSQL = `
SELECT m.id AS monitor_id,
       (SELECT COUNT(*) FROM uptime_heartbeats h
         WHERE h.monitor_id = m.id AND h.created_at >= ?) AS total,
       (SELECT COUNT(*) FROM uptime_heartbeats h
         WHERE h.monitor_id = m.id AND h.status = 'up' AND h.created_at >= ?) AS up
FROM (SELECT id FROM uptime_monitors ORDER BY name LIMIT ?) m`

func (s *UptimeSummaryService) loadUptime24h(ctx context.Context, windowStart time.Time) (map[string]float64, error) {
	var rows []uptime24hRow
	if err := s.db.WithContext(ctx).
		Raw(uptime24hSQL, windowStart, windowStart, uptimeMonitorScanLimit).
		Scan(&rows).Error; err != nil {
		return nil, err
	}

	out := make(map[string]float64, len(rows))
	for _, r := range rows {
		if r.Total == 0 {
			continue // no in-window beats: uptime_24h stays null
		}
		out[r.MonitorID] = float64(r.Up) * 100.0 / float64(r.Total)
	}
	return out, nil
}

// assembleSummaries joins the three result sets in Go on monitor_id. Monitors
// with no in-window heartbeats get recent_beats: [] and uptime_24h: null, with
// status taken from the monitor row.
func assembleSummaries(
	monitors []models.UptimeMonitor,
	beatsByMonitor map[string][]BeatDTO,
	uptimeByMonitor map[string]float64,
) []MonitorSummary {
	out := make([]MonitorSummary, 0, len(monitors))
	for i := range monitors {
		m := monitors[i]

		var lastCheck *time.Time
		if !m.LastCheck.IsZero() {
			lc := m.LastCheck
			lastCheck = &lc
		}

		var uptime24h *float64
		if pct, ok := uptimeByMonitor[m.ID]; ok {
			p := pct
			uptime24h = &p
		}

		beats := beatsByMonitor[m.ID]
		if beats == nil {
			beats = []BeatDTO{}
		}

		// Legacy rows persisted before max_retries existed carry 0; surface the
		// effective default (mirrors uptime_service.go's maxRetries <= 0 -> 3)
		// so the Edit-monitor modal round-trips the value the checker actually
		// uses instead of silently resetting it on save.
		maxRetries := m.MaxRetries
		if maxRetries <= 0 {
			maxRetries = 3
		}

		out = append(out, MonitorSummary{
			ID:             m.ID,
			Name:           m.Name,
			Type:           m.Type,
			URL:            m.URL,
			Enabled:        m.Enabled,
			Status:         m.Status,
			Latency:        m.Latency,
			LastCheck:      lastCheck,
			Interval:       m.Interval,
			MaxRetries:     maxRetries,
			ProxyHostID:    m.ProxyHostID,
			RemoteServerID: m.RemoteServerID,
			Uptime24h:      uptime24h,
			RecentBeats:    beats,
		})
	}
	return out
}

// sliceSummaries returns a copy of src with each monitor's RecentBeats trimmed
// to its trailing beats entries. The copy protects the cached slice from
// mutation by callers.
func sliceSummaries(src []MonitorSummary, beats int) []MonitorSummary {
	out := make([]MonitorSummary, len(src))
	for i := range src {
		out[i] = src[i]
		rb := src[i].RecentBeats
		if len(rb) > beats {
			rb = rb[len(rb)-beats:]
		}
		trimmed := make([]BeatDTO, len(rb))
		copy(trimmed, rb)
		out[i].RecentBeats = trimmed
	}
	return out
}
