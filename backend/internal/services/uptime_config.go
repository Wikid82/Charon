package services

import (
	"encoding/json"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"gorm.io/gorm"
)

// Hardcoded fallback defaults for the uptime.* settings. Used when the
// Settings table has no row yet (fresh DB) or when a stored value fails to
// parse as an integer. These match the seeds written in routes.go (§3.6.1).
const (
	defaultUptimeIntervalSeconds = 60
	defaultUptimeWorkerPoolSize  = 30
	defaultUptimeRetentionDays   = 90

	// minUptimeIntervalSeconds is the hard floor for any monitor check
	// interval — no monitor may be checked more often than this.
	minUptimeIntervalSeconds = 30

	uptimeConfigTTL = 60 * time.Second
)

// cachedUptimeCfg is an immutable snapshot of the three uptime.* settings.
type cachedUptimeCfg struct {
	defaultIntervalSeconds int
	workerPoolSize         int
	retentionDays          int
}

// uptimeConfig is a hot-reloading (TTL-cached) view of the uptime.* rows in
// the Settings table. It is shared, read-only, by the scheduler, the pruner,
// and UptimeService (for write-time interval resolution). Writes go through
// the normal POST /api/v1/settings endpoint.
//
// It is deliberately lowercase: the exported identifier UptimeConfig is
// already taken by an unrelated timeout/threshold struct on UptimeService
// (spec §3.6.2), so the injected instance is stored as UptimeService.uptimeCfg.
type uptimeConfig struct {
	db  *gorm.DB
	mu  sync.RWMutex
	val cachedUptimeCfg
	exp time.Time
	now func() time.Time // injectable clock — test seam (N8)
	ttl time.Duration
}

// newUptimeConfig builds a config snapshot bound to db. The first accessor
// call populates the cache; subsequent calls within ttl return the cached
// value. A nil db is tolerated and always yields the hardcoded defaults.
func newUptimeConfig(db *gorm.DB) *uptimeConfig {
	return &uptimeConfig{
		db:  db,
		now: time.Now,
		ttl: uptimeConfigTTL,
	}
}

// snapshot returns the cached settings, refreshing from the DB when the TTL
// has elapsed (or the cache has never been populated).
func (c *uptimeConfig) snapshot() cachedUptimeCfg {
	c.mu.RLock()
	if !c.exp.IsZero() && c.now().Before(c.exp) {
		v := c.val
		c.mu.RUnlock()
		return v
	}
	c.mu.RUnlock()

	c.mu.Lock()
	defer c.mu.Unlock()
	// Re-check under the write lock: another goroutine may have refreshed
	// while we were upgrading the lock.
	if !c.exp.IsZero() && c.now().Before(c.exp) {
		return c.val
	}

	c.val = cachedUptimeCfg{
		defaultIntervalSeconds: c.loadInt("uptime.default_interval_seconds", defaultUptimeIntervalSeconds),
		workerPoolSize:         c.loadInt("uptime.worker_pool_size", defaultUptimeWorkerPoolSize),
		retentionDays:          c.loadInt("uptime.heartbeat_retention_days", defaultUptimeRetentionDays),
	}
	c.exp = c.now().Add(c.ttl)
	return c.val
}

// loadInt reads a single integer setting, falling back to fallback on any
// error (missing row, missing table, unparseable value).
func (c *uptimeConfig) loadInt(key string, fallback int) int {
	if c.db == nil {
		return fallback
	}
	var setting models.Setting
	if err := c.db.Where("key = ?", key).First(&setting).Error; err != nil {
		return fallback
	}
	n, err := strconv.Atoi(strings.TrimSpace(setting.Value))
	if err != nil {
		logger.Log().WithError(err).WithField("key", key).
			Warn("uptimeConfig: non-integer setting value, using default")
		return fallback
	}
	return n
}

// DefaultIntervalSeconds is the admin-configured default check interval for
// new and legacy (zero-interval) monitors.
func (c *uptimeConfig) DefaultIntervalSeconds() int { return c.snapshot().defaultIntervalSeconds }

// RetentionDays is how long heartbeat rows are kept before the pruner deletes them.
func (c *uptimeConfig) RetentionDays() int { return c.snapshot().retentionDays }

// WorkerPoolSize is the configured worker count. Not hot-reloaded in practice
// (the pool is sized at construction) — exposed here for that construction and
// for the health endpoint.
func (c *uptimeConfig) WorkerPoolSize() int { return c.snapshot().workerPoolSize }

// forceRefresh expires the cache so the next accessor reloads from the DB.
// Test-only seam so hot-reload tests need not sleep out the TTL.
func (c *uptimeConfig) forceRefresh() {
	c.mu.Lock()
	c.exp = time.Time{}
	c.mu.Unlock()
}

// coerceIntervalSeconds best-effort converts a value pulled from an untyped
// update map (map[string]any decoded from JSON, or a Go literal in tests)
// into an integer second count. The bool reports whether the conversion
// succeeded; callers treat a failure as "no interval change to validate".
func coerceIntervalSeconds(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case int32:
		return int(n), true
	case int64:
		return int(n), true
	case float32:
		return int(n), true
	case float64:
		return int(n), true
	case json.Number:
		i, err := n.Int64()
		return int(i), err == nil
	case string:
		i, err := strconv.Atoi(strings.TrimSpace(n))
		return i, err == nil
	default:
		return 0, false
	}
}

// clampInterval resolves a requested interval (in seconds) to a concrete,
// floored value: a non-positive request becomes the admin default, then any
// value below the 30-second hard floor is raised to 30. A nil cfg falls back
// to the hardcoded default (defensive; production always injects one).
func clampInterval(seconds int, cfg *uptimeConfig) int {
	if seconds <= 0 {
		if cfg != nil {
			seconds = cfg.DefaultIntervalSeconds()
		} else {
			seconds = defaultUptimeIntervalSeconds
		}
	}
	if seconds < minUptimeIntervalSeconds {
		seconds = minUptimeIntervalSeconds
	}
	return seconds
}
