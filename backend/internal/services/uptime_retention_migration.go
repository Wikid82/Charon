package services

import (
	"errors"
	"fmt"
	"time"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"gorm.io/gorm"
)

const (
	// UptimeRetentionMigrationMarker is the settings row recording that the
	// one-time retention default migration has run. The "migration." prefix
	// keeps it out of the settings API (see isInternalSettingKey).
	UptimeRetentionMigrationMarker = "migration.uptime_retention_default_30"

	retentionSettingKey = "uptime.heartbeat_retention_days"
	intervalSettingKey  = "uptime.default_interval_seconds"
	poolSettingKey      = "uptime.worker_pool_size"

	// legacyRetentionSeed is the previous seeded default that gets migrated.
	legacyRetentionSeed = "90"
	// Seed values of the two sibling rows written next to retention.
	seededIntervalValue = "60"
	seededPoolValue     = "30"

	// seedTimestampWindow is how close the retention row's UpdatedAt must be
	// to each sibling's for the three to count as written by the same seed run.
	seedTimestampWindow = 5 * time.Second
	// seedRowSpan is max(ID) - min(ID) for three consecutively created rows.
	seedRowSpan = 2
)

// MigrateUptimeRetentionDefault lowers the retention setting from the old
// seeded 90 to the new default, but only when the row is provably the untouched
// seed: it is within seedTimestampWindow of BOTH sibling seed rows, the three
// IDs are consecutive, and the siblings still hold their seed values. Any
// ambiguity leaves the value alone. A marker row makes the migration one-shot.
func MigrateUptimeRetentionDefault(db *gorm.DB) error {
	var markers int64
	if err := db.Model(&models.Setting{}).
		Where("key = ?", UptimeRetentionMigrationMarker).Count(&markers).Error; err != nil {
		return fmt.Errorf("check retention migration marker: %w", err)
	}
	if markers > 0 {
		return nil
	}

	var retention models.Setting
	err := db.Where("key = ?", retentionSettingKey).First(&retention).Error
	switch {
	case errors.Is(err, gorm.ErrRecordNotFound):
		return writeRetentionMarker(db)
	case err != nil:
		return fmt.Errorf("load retention setting: %w", err)
	case retention.Value != legacyRetentionSeed:
		return writeRetentionMarker(db)
	}

	untouched, err := isUntouchedRetentionSeed(db, retention)
	if err != nil {
		return err
	}
	if untouched {
		res := db.Model(&models.Setting{}).
			Where("key = ? AND value = ?", retentionSettingKey, legacyRetentionSeed).
			Update("value", fmt.Sprintf("%d", UptimeRetentionDefaultDays))
		if res.Error != nil {
			return fmt.Errorf("lower retention default: %w", res.Error)
		}
		logger.Log().Info("uptime heartbeat retention default lowered from 90 to 30 days; adjust under System Settings -> Uptime Monitoring")
	} else {
		logger.Log().Info("uptime heartbeat retention left at 90 days: value not provably the untouched default (it may have been set deliberately); adjust under System Settings -> Uptime Monitoring")
	}
	return writeRetentionMarker(db)
}

// isUntouchedRetentionSeed reports whether the three rows look like a single
// uninterrupted seed run that nobody has edited since. A missing sibling row is
// indeterminate and counts as not untouched.
func isUntouchedRetentionSeed(db *gorm.DB, retention models.Setting) (bool, error) {
	var interval, pool models.Setting
	for key, dst := range map[string]*models.Setting{intervalSettingKey: &interval, poolSettingKey: &pool} {
		if err := db.Where("key = ?", key).First(dst).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return false, nil
			}
			return false, fmt.Errorf("load sibling uptime setting: %w", err)
		}
	}
	if interval.Value != seededIntervalValue || pool.Value != seededPoolValue {
		return false, nil
	}
	for _, sibling := range []models.Setting{interval, pool} {
		if retention.UpdatedAt.Sub(sibling.UpdatedAt).Abs() > seedTimestampWindow {
			return false, nil
		}
	}
	ids := []uint{retention.ID, interval.ID, pool.ID}
	lo, hi := ids[0], ids[0]
	for _, id := range ids[1:] {
		lo, hi = min(lo, id), max(hi, id)
	}
	return hi-lo == seedRowSpan, nil
}

func writeRetentionMarker(db *gorm.DB) error {
	marker := models.Setting{
		Key:      UptimeRetentionMigrationMarker,
		Value:    "done",
		Type:     "string",
		Category: "migration",
	}
	if err := db.Where(models.Setting{Key: marker.Key}).Attrs(marker).FirstOrCreate(&marker).Error; err != nil {
		return fmt.Errorf("write retention migration marker: %w", err)
	}
	return nil
}
