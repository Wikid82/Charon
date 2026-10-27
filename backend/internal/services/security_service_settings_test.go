package services

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

// TestSecurityService_Upsert_SavesGeneralColumns pins that Upsert saves the general (non-module) columns of the
// default row, such as the enabled flag.
func TestSecurityService_Upsert_SavesGeneralColumns(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := newTestSecurityService(t, db)
	require.NoError(t, db.Create(&models.SecurityConfig{Name: "default", Enabled: true, CrowdSecMode: "local"}).Error)

	require.NoError(t, svc.Upsert(&models.SecurityConfig{Name: "default", Enabled: false, CrowdSecMode: "local"}))

	var stored models.SecurityConfig
	require.NoError(t, db.Where("name = ?", "default").First(&stored).Error)
	assert.False(t, stored.Enabled)
	assert.Equal(t, "local", stored.CrowdSecMode)
}

// TestSecurityService_Upsert_DoesNotCopyModuleColumns describes Upsert saving the general fields of a row without
// copying the WAF mode and rate-limit mode and enable columns onto the stored row.
func TestSecurityService_Upsert_DoesNotCopyModuleColumns(t *testing.T) {
	t.Skip("enabled in commit 4 of this PR: Upsert leaves module columns to the module writers")
}
