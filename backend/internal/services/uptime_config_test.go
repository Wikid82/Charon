package services

import (
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

func seedUptimeSetting(t *testing.T, db *gorm.DB, key, value string) {
	t.Helper()
	require.NoError(t, db.Where(models.Setting{Key: key}).
		Assign(models.Setting{Key: key, Value: value, Type: "int", Category: "uptime"}).
		FirstOrCreate(&models.Setting{}).Error)
}

func TestUptimeConfig_FallsBackToHardcodedDefaultsWhenSettingsMissing(t *testing.T) {
	db := setupUptimeTestDB(t)
	cfg := newUptimeConfig(db)

	assert.Equal(t, 60, cfg.DefaultIntervalSeconds())
	assert.Equal(t, 30, cfg.WorkerPoolSize())
	assert.Equal(t, 90, cfg.RetentionDays())
}

func TestUptimeConfig_ReadsSeededValues(t *testing.T) {
	db := setupUptimeTestDB(t)
	seedUptimeSetting(t, db, "uptime.default_interval_seconds", "45")
	seedUptimeSetting(t, db, "uptime.worker_pool_size", "50")
	seedUptimeSetting(t, db, "uptime.heartbeat_retention_days", "120")

	cfg := newUptimeConfig(db)
	assert.Equal(t, 45, cfg.DefaultIntervalSeconds())
	assert.Equal(t, 50, cfg.WorkerPoolSize())
	assert.Equal(t, 120, cfg.RetentionDays())
}

func TestUptimeConfig_InvalidValueFallsBackToDefault(t *testing.T) {
	db := setupUptimeTestDB(t)
	seedUptimeSetting(t, db, "uptime.default_interval_seconds", "not-a-number")

	cfg := newUptimeConfig(db)
	assert.Equal(t, 60, cfg.DefaultIntervalSeconds())
}

func TestUptimeConfig_TTLCachesUntilExpiry(t *testing.T) {
	db := setupUptimeTestDB(t)
	seedUptimeSetting(t, db, "uptime.default_interval_seconds", "45")

	fake := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	cfg := newUptimeConfig(db)
	cfg.now = func() time.Time { return fake }

	require.Equal(t, 45, cfg.DefaultIntervalSeconds())

	// Change the underlying setting; within the TTL the cached value stands.
	seedUptimeSetting(t, db, "uptime.default_interval_seconds", "90")
	assert.Equal(t, 45, cfg.DefaultIntervalSeconds())

	// Advance the clock past the TTL — the next read refreshes.
	fake = fake.Add(61 * time.Second)
	assert.Equal(t, 90, cfg.DefaultIntervalSeconds())
}

func TestUptimeConfig_ForceRefreshBypassesTTL(t *testing.T) {
	db := setupUptimeTestDB(t)
	seedUptimeSetting(t, db, "uptime.default_interval_seconds", "45")

	cfg := newUptimeConfig(db)
	require.Equal(t, 45, cfg.DefaultIntervalSeconds())

	seedUptimeSetting(t, db, "uptime.default_interval_seconds", "75")
	// Still cached...
	assert.Equal(t, 45, cfg.DefaultIntervalSeconds())
	// ...until forced.
	cfg.forceRefresh()
	assert.Equal(t, 75, cfg.DefaultIntervalSeconds())
}

func TestUptimeConfig_NilDBUsesDefaults(t *testing.T) {
	cfg := newUptimeConfig(nil)
	assert.Equal(t, 60, cfg.DefaultIntervalSeconds())
	assert.Equal(t, 30, cfg.WorkerPoolSize())
	assert.Equal(t, 90, cfg.RetentionDays())
}

func TestClampInterval(t *testing.T) {
	db := setupUptimeTestDB(t)
	cfg := newUptimeConfig(db)

	t.Run("zero resolves to default", func(t *testing.T) {
		assert.Equal(t, 60, clampInterval(0, cfg))
	})
	t.Run("negative resolves to default", func(t *testing.T) {
		assert.Equal(t, 60, clampInterval(-10, cfg))
	})
	t.Run("below floor is raised to 30", func(t *testing.T) {
		assert.Equal(t, 30, clampInterval(5, cfg))
		assert.Equal(t, 30, clampInterval(29, cfg))
	})
	t.Run("at or above floor passes through", func(t *testing.T) {
		assert.Equal(t, 30, clampInterval(30, cfg))
		assert.Equal(t, 45, clampInterval(45, cfg))
		assert.Equal(t, 3600, clampInterval(3600, cfg))
	})
	t.Run("zero respects the configured default", func(t *testing.T) {
		seedUptimeSetting(t, db, "uptime.default_interval_seconds", "45")
		cfg.forceRefresh()
		assert.Equal(t, 45, clampInterval(0, cfg))
	})
	t.Run("configured default below floor is still floored", func(t *testing.T) {
		seedUptimeSetting(t, db, "uptime.default_interval_seconds", "10")
		cfg.forceRefresh()
		assert.Equal(t, 30, clampInterval(0, cfg))
	})
	t.Run("nil cfg uses hardcoded default", func(t *testing.T) {
		assert.Equal(t, 60, clampInterval(0, nil))
		assert.Equal(t, 30, clampInterval(1, nil))
	})
}
