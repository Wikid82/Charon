package services

import (
	"strings"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	logtest "github.com/sirupsen/logrus/hooks/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

const (
	keyInterval  = "uptime.default_interval_seconds"
	keyPool      = "uptime.worker_pool_size"
	keyRetention = "uptime.heartbeat_retention_days"
)

// seedRow creates one uptime.* row and stamps a fixed UpdatedAt.
func seedRow(t *testing.T, db *gorm.DB, key, value string, at time.Time) {
	t.Helper()
	s := models.Setting{Key: key, Value: value, Type: "int", Category: "uptime"}
	require.NoError(t, db.Create(&s).Error)
	require.NoError(t, db.Model(&models.Setting{}).Where("id = ?", s.ID).
		UpdateColumn("updated_at", at).Error)
}

func settingValue(t *testing.T, db *gorm.DB, key string) string {
	t.Helper()
	var s models.Setting
	require.NoError(t, db.Where("key = ?", key).First(&s).Error)
	return s.Value
}

func markerExists(t *testing.T, db *gorm.DB) bool {
	t.Helper()
	var n int64
	require.NoError(t, db.Model(&models.Setting{}).
		Where("key = ?", UptimeRetentionMigrationMarker).Count(&n).Error)
	return n == 1
}

func TestMigrateUptimeRetentionDefault(t *testing.T) {
	seedAt := time.Date(2026, 8, 27, 10, 0, 0, 0, time.UTC)
	later := seedAt.Add(6 * time.Hour)
	ms := func(n int) time.Time { return seedAt.Add(time.Duration(n) * time.Millisecond) }

	type row struct {
		key, value string
		at         time.Time
	}
	cases := []struct {
		name      string
		rows      []row // created in order (IDs ascend)
		before    func(t *testing.T, db *gorm.DB)
		want      string // expected retention value after; "" = row absent
		wantInfo  string // substring of an Info log expected exactly once ("" = none)
		wantLeft  bool   // expect the "left at 90" line exactly once
		preMarker bool
	}{
		{
			name: "untouched seed is lowered",
			rows: []row{{keyInterval, "60", ms(0)}, {keyPool, "30", ms(5)}, {keyRetention, "90", ms(11)}},
			want: "30", wantInfo: "lowered from 90 to 30",
		},
		{
			name: "deliberate 90 saved hours later stays",
			rows: []row{{keyInterval, "60", ms(0)}, {keyPool, "30", ms(5)}, {keyRetention, "90", later}},
			want: "90", wantLeft: true,
		},
		{
			// 60 -> 90 saved together with a pool change: retention and pool
			// share a timestamp, the interval sibling still carries the seed time.
			name: "same-save 60 to 90 with pool change stays",
			rows: []row{{keyInterval, "60", seedAt}, {keyPool, "50", later}, {keyRetention, "90", later.Add(2 * time.Millisecond)}},
			want: "90", wantLeft: true,
		},
		{
			name: "both siblings edited later stays",
			rows: []row{{keyInterval, "60", later}, {keyPool, "30", later.Add(time.Millisecond)}, {keyRetention, "90", seedAt}},
			want: "90", wantLeft: true,
		},
		{
			// Documented residual risk: a single write of all three rows that
			// leaves the siblings at their seed values is indistinguishable
			// from the untouched seed.
			name: "bulk write with sibling seed values is lowered",
			rows: []row{{keyInterval, "60", later}, {keyPool, "30", later.Add(time.Millisecond)}, {keyRetention, "90", later.Add(2 * time.Millisecond)}},
			want: "30", wantInfo: "lowered from 90 to 30",
		},
		{
			name: "siblings edited to non-default values in same save stays",
			rows: []row{{keyInterval, "120", ms(0)}, {keyPool, "10", ms(1)}, {keyRetention, "90", ms(2)}},
			want: "90", wantLeft: true,
		},
		{
			name: "non-contiguous ids stay",
			rows: []row{{keyInterval, "60", ms(0)}, {keyPool, "30", ms(1)}, {"filler.key", "x", ms(1)}, {keyRetention, "90", ms(2)}},
			want: "90", wantLeft: true,
		},
		{
			name: "missing sibling stays",
			rows: []row{{keyInterval, "60", ms(0)}, {keyRetention, "90", ms(1)}},
			want: "90", wantLeft: true,
		},
		{
			name: "custom value is unchanged",
			rows: []row{{keyInterval, "60", ms(0)}, {keyPool, "30", ms(1)}, {keyRetention, "45", ms(2)}},
			want: "45",
		},
		{
			name: "fresh install already at 30 only writes marker",
			rows: []row{{keyInterval, "60", ms(0)}, {keyPool, "30", ms(1)}, {keyRetention, "30", ms(2)}},
			want: "30",
		},
		{
			name: "missing retention row only writes marker",
			rows: []row{{keyInterval, "60", ms(0)}, {keyPool, "30", ms(1)}},
			want: "",
		},
		{
			name: "existing marker is a no-op for an untouched-looking 90",
			rows: []row{{keyInterval, "60", ms(0)}, {keyPool, "30", ms(1)}, {keyRetention, "90", ms(2)}},
			want: "90", preMarker: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			db := setupUptimeTestDB(t)
			for _, r := range tc.rows {
				seedRow(t, db, r.key, r.value, r.at)
			}
			if tc.preMarker {
				require.NoError(t, db.Create(&models.Setting{
					Key: UptimeRetentionMigrationMarker, Value: "done", Type: "string", Category: "migration",
				}).Error)
			}
			hook := logtest.NewLocal(logger.Log().Logger)

			require.NoError(t, MigrateUptimeRetentionDefault(db))

			if tc.want == "" {
				var n int64
				require.NoError(t, db.Model(&models.Setting{}).Where("key = ?", keyRetention).Count(&n).Error)
				assert.Zero(t, n)
			} else {
				assert.Equal(t, tc.want, settingValue(t, db, keyRetention))
			}
			assert.True(t, markerExists(t, db))
			assert.Equal(t, boolToInt(tc.wantInfo != ""), countLogs(hook, "lowered from 90 to 30"))
			assert.Equal(t, boolToInt(tc.wantLeft), countLogs(hook, "left at 90 days"))

			// Idempotent: a second run changes nothing and logs nothing new.
			hook.Reset()
			require.NoError(t, MigrateUptimeRetentionDefault(db))
			assert.Equal(t, tc.want, settingValueOrEmpty(db, keyRetention))
			assert.Zero(t, countLogs(hook, "lowered from 90 to 30")+countLogs(hook, "left at 90 days"))
		})
	}
}

func TestMigrateUptimeRetentionDefault_RestoredPreUpgradeBackupRerunsIdentically(t *testing.T) {
	seedAt := time.Date(2026, 8, 27, 10, 0, 0, 0, time.UTC)
	db := setupUptimeTestDB(t)
	seedRow(t, db, keyInterval, "60", seedAt)
	seedRow(t, db, keyPool, "30", seedAt.Add(time.Millisecond))
	seedRow(t, db, keyRetention, "90", seedAt.Add(2*time.Millisecond))

	require.NoError(t, MigrateUptimeRetentionDefault(db))
	require.Equal(t, "30", settingValue(t, db, keyRetention))

	// Simulate restoring a pre-upgrade backup: row back to 90, marker gone.
	require.NoError(t, db.Model(&models.Setting{}).Where("key = ?", keyRetention).
		UpdateColumns(map[string]any{"value": "90", "updated_at": seedAt.Add(2 * time.Millisecond)}).Error)
	require.NoError(t, db.Where("key = ?", UptimeRetentionMigrationMarker).Delete(&models.Setting{}).Error)

	require.NoError(t, MigrateUptimeRetentionDefault(db))
	assert.Equal(t, "30", settingValue(t, db, keyRetention))
}

func TestMigrateUptimeRetentionDefault_DBError(t *testing.T) {
	db := setupUptimeTestDB(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
	assert.Error(t, MigrateUptimeRetentionDefault(db))
}

func settingValueOrEmpty(db *gorm.DB, key string) string {
	var s models.Setting
	if err := db.Where("key = ?", key).First(&s).Error; err != nil {
		return ""
	}
	return s.Value
}

func countLogs(hook *logtest.Hook, substr string) int {
	n := 0
	for _, e := range hook.AllEntries() {
		if strings.Contains(e.Message, substr) {
			n++
		}
	}
	return n
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
