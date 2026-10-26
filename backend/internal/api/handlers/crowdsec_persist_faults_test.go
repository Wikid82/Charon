package handlers

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/testutil/gormfault"
)

// whereEqualsKey matches a statement whose WHERE clause compares the key column with key. The settings upsert
// names its key in a struct condition, which GORM stores as an equality clause rather than a raw expression.
func whereEqualsKey(key string) func(tx *gorm.DB) bool {
	return func(tx *gorm.DB) bool {
		where, _ := tx.Statement.Clauses["WHERE"].Expression.(clause.Where)
		for _, expr := range where.Exprs {
			if eq, ok := expr.(clause.Eq); ok && eq.Value == key {
				return true
			}
		}
		return false
	}
}

// A failed write of the deprecated mode key stops the start before the enabled key is written.
func TestPersistCrowdsecStart_ModeSettingWriteFailureLeavesEnabledKeyUnwritten(t *testing.T) {
	db := setupCrowdDB(t)
	gormfault.Inject(t, db, gormfault.Query, "settings", whereEqualsKey(models.CrowdSecDeprecatedModeSettingKey))
	h := &CrowdsecHandler{DB: db}

	err := h.persistCrowdsecStart()

	require.ErrorContains(t, err, "set crowdsec mode setting")
	_, found := storedSettingValue(t, db, models.CrowdSecEnabledSettingKey)
	assert.False(t, found, "the enabled key must not be written after the mode key fails")
}
