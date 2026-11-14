package models_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

// setRowColumns writes columns on the default row without touching updated_at, so a test controls the update time.
func setRowColumns(t *testing.T, db *gorm.DB, columns map[string]any) {
	t.Helper()
	require.NoError(t, db.Model(&models.SecurityConfig{}).Where("name = ?", models.DefaultSecurityConfigName).UpdateColumns(columns).Error)
}

// TestImagedRowColumns_ListsEveryColumnAConfigSaveCanWrite describes the column list the snapshot images, sorted and
// including the columns a refused save used to leave changed.
func TestImagedRowColumns_ListsEveryColumnAConfigSaveCanWrite(t *testing.T) {
	columns := models.ImagedRowColumns()
	for _, want := range []string{
		"crowd_sec_api_url", "crowd_sec_mode", "waf_exclusions", "waf_learning", "waf_paranoia_level",
		"rate_limit_bypass_list", "admin_whitelist", "enabled",
	} {
		assert.Contains(t, columns, want)
	}
	assert.Len(t, columns, 15)
	for i := 1; i < len(columns); i++ {
		assert.Less(t, columns[i-1], columns[i], "columns are sorted")
	}
}

// TestRestore_PutsBackEveryImagedColumnTheWriterChanged describes a refused write on each column the snapshot images
// now that rowColumns covers the full set: the writer's value is replaced by the pre-image.
func TestRestore_PutsBackEveryImagedColumnTheWriterChanged(t *testing.T) {
	tests := []struct {
		name    string
		initial models.SecurityConfig
		column  string
		change  any
		read    func(models.SecurityConfig) any
		want    any
	}{
		{
			name:    "waf paranoia level",
			initial: models.SecurityConfig{WAFParanoiaLevel: 3},
			column:  "waf_paranoia_level", change: 2,
			read: func(r models.SecurityConfig) any { return r.WAFParanoiaLevel }, want: 3,
		},
		{
			name:    "crowdsec api url",
			initial: models.SecurityConfig{CrowdSecAPIURL: "http://127.0.0.1:8080"},
			column:  "crowd_sec_api_url", change: "http://192.0.2.1:8080",
			read: func(r models.SecurityConfig) any { return r.CrowdSecAPIURL }, want: "http://127.0.0.1:8080",
		},
		{
			name:    "waf learning",
			initial: models.SecurityConfig{WAFLearning: false},
			column:  "waf_learning", change: true,
			read: func(r models.SecurityConfig) any { return r.WAFLearning }, want: false,
		},
		{
			name:    "waf exclusions",
			initial: models.SecurityConfig{WAFExclusions: "[]"},
			column:  "waf_exclusions", change: `[{"rule_id":1}]`,
			read: func(r models.SecurityConfig) any { return r.WAFExclusions }, want: "[]",
		},
		{
			name:    "crowdsec mode",
			initial: models.SecurityConfig{CrowdSecMode: "disabled"},
			column:  "crowd_sec_mode", change: "local",
			read: func(r models.SecurityConfig) any { return r.CrowdSecMode }, want: "disabled",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := newModuleTestDB(t)
			initial := tt.initial
			initial.Name = models.DefaultSecurityConfigName
			require.NoError(t, db.Create(&initial).Error)

			snap, err := models.CaptureModuleState(db, models.DefaultSecurityConfigName, ptrTo(readDefaultRow(t, db)))
			require.NoError(t, err)
			setRowColumns(t, db, map[string]any{tt.column: tt.change})
			require.NoError(t, snap.Seal(db))
			require.NoError(t, snap.RestoreRow(db))

			restored := readDefaultRow(t, db)
			assert.Equal(t, tt.want, tt.read(restored))
		})
	}
}

// TestRestore_FullRestoreReturnsTheRowUpdateTime describes a restore that puts back every touched column: the row's
// update time goes back to the pre-image as well, so the refused write leaves no trace in updated_at.
func TestRestore_FullRestoreReturnsTheRowUpdateTime(t *testing.T) {
	db := newModuleTestDB(t)
	require.NoError(t, db.Create(&models.SecurityConfig{Name: models.DefaultSecurityConfigName, WAFParanoiaLevel: 3}).Error)
	pre := time.Date(2020, time.January, 2, 3, 4, 5, 0, time.UTC)
	setRowColumns(t, db, map[string]any{"updated_at": pre})

	snap, err := models.CaptureModuleState(db, models.DefaultSecurityConfigName, ptrTo(readDefaultRow(t, db)))
	require.NoError(t, err)
	// A normal update stamps the update time with the write time.
	require.NoError(t, db.Model(&models.SecurityConfig{}).Where("name = ?", models.DefaultSecurityConfigName).Update("waf_paranoia_level", 2).Error)
	require.NoError(t, snap.Seal(db))
	require.NoError(t, snap.RestoreRow(db))

	restored := readDefaultRow(t, db)
	assert.Equal(t, 3, restored.WAFParanoiaLevel)
	assert.True(t, pre.Equal(restored.UpdatedAt), "updated_at is the pre-image: got %s want %s", restored.UpdatedAt, pre)
}

// TestRestore_PartialRestoreStampsTheRestoreTime describes a restore where a concurrent write changed a touched column:
// that column is kept and the row was written again, so the update time is the restore time, not the pre-image.
func TestRestore_PartialRestoreStampsTheRestoreTime(t *testing.T) {
	db := newModuleTestDB(t)
	require.NoError(t, db.Create(&models.SecurityConfig{Name: models.DefaultSecurityConfigName, WAFParanoiaLevel: 3}).Error)
	pre := time.Date(2020, time.January, 2, 3, 4, 5, 0, time.UTC)
	setRowColumns(t, db, map[string]any{"updated_at": pre})

	snap, err := models.CaptureModuleState(db, models.DefaultSecurityConfigName, ptrTo(readDefaultRow(t, db)))
	require.NoError(t, err)
	require.NoError(t, db.Model(&models.SecurityConfig{}).Where("name = ?", models.DefaultSecurityConfigName).Update("waf_paranoia_level", 2).Error)
	require.NoError(t, snap.Seal(db))
	require.NoError(t, db.Model(&models.SecurityConfig{}).Where("name = ?", models.DefaultSecurityConfigName).Update("waf_paranoia_level", 4).Error)
	require.NoError(t, snap.RestoreRow(db))

	restored := readDefaultRow(t, db)
	assert.Equal(t, 4, restored.WAFParanoiaLevel, "the concurrent value is kept")
	assert.True(t, restored.UpdatedAt.After(pre), "the restore time is stamped: got %s", restored.UpdatedAt)
}

func ptrTo[T any](v T) *T { return &v }
