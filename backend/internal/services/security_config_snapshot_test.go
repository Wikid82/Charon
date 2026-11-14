package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

// allUpdateFieldsBody sets every field a config save can carry, so the column parity check sees each writer branch.
const allUpdateFieldsBody = `{
	"admin_whitelist":"192.0.2.0/24",
	"crowdsec_api_url":"http://192.0.2.1:8080",
	"waf_mode":"block",
	"waf_rules_source":"owasp-crs",
	"waf_learning":false,
	"waf_paranoia_level":2,
	"waf_exclusions":"[]",
	"rate_limit_mode":"enabled",
	"rate_limit_enable":true,
	"rate_limit_burst":4,
	"rate_limit_requests":50,
	"rate_limit_window_sec":7,
	"rate_limit_bypass_list":"198.51.100.0/24"
}`

// TestApplyTo_EveryWrittenColumnIsImaged describes the parity between the config save and the snapshot: every column a
// save can write is one the snapshot images, so a refused save can put it back.
func TestApplyTo_EveryWrittenColumnIsImaged(t *testing.T) {
	update := decodeUpdateTestBody(t, allUpdateFieldsBody)
	columns := update.applyTo(&models.SecurityConfig{})

	require.NotEmpty(t, columns)
	imaged := models.ImagedRowColumns()
	for _, column := range columns {
		assert.Contains(t, imaged, column, "column %s is written by a save but not imaged", column)
	}
}

// TestApplyUpdate_RefusedApplyRestoresEveryImagedColumn describes a refused save on the default row: the restore puts
// back the columns the save changed, including the ones that used to be left behind, and the update time.
func TestApplyUpdate_RefusedApplyRestoresEveryImagedColumn(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := newTestSecurityService(t, db)
	seedUpdateTestRow(t, db, fullUpdateTestRow())
	pre := time.Date(2020, time.January, 2, 3, 4, 5, 0, time.UTC)
	require.NoError(t, db.Model(&models.SecurityConfig{}).Where("name = ?", models.DefaultSecurityConfigName).UpdateColumns(map[string]any{"updated_at": pre}).Error)
	before := loadUpdateTestRow(t, db)

	body := `{"waf_paranoia_level":2,"crowdsec_api_url":"http://192.0.2.9:8080","waf_learning":false,"waf_exclusions":"[]"}`
	_, snap, err := svc.ApplyUpdate("test-actor", models.DefaultSecurityConfigName, decodeUpdateTestBody(t, body))
	require.NoError(t, err)
	changed := loadUpdateTestRow(t, db)
	require.Equal(t, 2, changed.WAFParanoiaLevel, "the save is stored before the apply fails")

	require.NoError(t, snap.RestoreSettings(db))
	require.NoError(t, snap.RestoreRow(db))

	restored := loadUpdateTestRow(t, db)
	requireUpdateTestRowEqual(t, before, restored)
	assert.True(t, pre.Equal(restored.UpdatedAt), "updated_at is the pre-image: got %s want %s", restored.UpdatedAt, pre)
}
