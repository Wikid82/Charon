package services

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

// TestApplyUpdateFunc_BuildDerivesTheUpdateFromTheStoredRow describes a save whose update depends on the row: build
// receives the row as stored under the lock, and the update it returns is the one applied.
func TestApplyUpdateFunc_BuildDerivesTheUpdateFromTheStoredRow(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := newTestSecurityService(t, db)
	seedUpdateTestRow(t, db, fullUpdateTestRow())

	var seen models.SecurityConfig
	stored, _, err := svc.ApplyUpdateFunc("test-actor", models.DefaultSecurityConfigName, func(current models.SecurityConfig) (SecurityConfigUpdate, error) {
		seen = current
		learning := false
		return SecurityConfigUpdate{WAFLearning: &learning}, nil
	})

	require.NoError(t, err)
	assert.True(t, seen.WAFLearning, "build sees the stored value, not the request")
	assert.False(t, stored.WAFLearning)
	assert.False(t, loadUpdateTestRow(t, db).WAFLearning)
}

// TestApplyUpdateFunc_BuildErrorAbortsBeforeAnyWrite describes a refused build: its error is returned as it is, and no
// column is written.
func TestApplyUpdateFunc_BuildErrorAbortsBeforeAnyWrite(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := newTestSecurityService(t, db)
	before := seedUpdateTestRow(t, db, fullUpdateTestRow())
	refused := errors.New("refused by build")

	_, _, err := svc.ApplyUpdateFunc("test-actor", models.DefaultSecurityConfigName, func(models.SecurityConfig) (SecurityConfigUpdate, error) {
		return SecurityConfigUpdate{WAFParanoiaLevel: intPtr(4)}, refused
	})

	require.ErrorIs(t, err, refused)
	requireUpdateTestRowEqual(t, before, loadUpdateTestRow(t, db))
}

// TestApplyUpdateFunc_BuildOnAbsentRowSeesModelDefaults describes a first save: build receives the model defaults for a
// row that does not exist yet, and the row is created from the update.
func TestApplyUpdateFunc_BuildOnAbsentRowSeesModelDefaults(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := newTestSecurityService(t, db)

	var seen models.SecurityConfig
	stored, _, err := svc.ApplyUpdateFunc("test-actor", models.DefaultSecurityConfigName, func(current models.SecurityConfig) (SecurityConfigUpdate, error) {
		seen = current
		return SecurityConfigUpdate{WAFLearning: boolPtr(true)}, nil
	})

	require.NoError(t, err)
	assert.Equal(t, "disabled", seen.RateLimitMode)
	assert.Equal(t, 1, seen.WAFParanoiaLevel)
	assert.True(t, stored.WAFLearning)
	assert.Equal(t, "disabled", loadUpdateTestRow(t, db).RateLimitMode)
}

// TestApplyUpdateTx_AppliesInTheCallersTransaction describes the transaction form: the update is written on the caller's
// transaction, so it is visible inside it.
func TestApplyUpdateTx_AppliesInTheCallersTransaction(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := newTestSecurityService(t, db)
	seedUpdateTestRow(t, db, fullUpdateTestRow())

	err := db.Transaction(func(tx *gorm.DB) error {
		stored, err := svc.ApplyUpdateTx(tx, "test-actor", models.DefaultSecurityConfigName, SecurityConfigUpdate{WAFLearning: boolPtr(false)})
		if err != nil {
			return err
		}
		assert.False(t, stored.WAFLearning)
		return nil
	})

	require.NoError(t, err)
	assert.False(t, loadUpdateTestRow(t, db).WAFLearning)
}

// TestApplyUpdateTx_InvalidUpdateWritesNothing describes a refused value on the transaction form: the error is returned
// and the row keeps its stored value.
func TestApplyUpdateTx_InvalidUpdateWritesNothing(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := newTestSecurityService(t, db)
	before := seedUpdateTestRow(t, db, fullUpdateTestRow())

	err := db.Transaction(func(tx *gorm.DB) error {
		_, err := svc.ApplyUpdateTx(tx, "test-actor", models.DefaultSecurityConfigName, SecurityConfigUpdate{WAFMode: strPtr("not-a-mode")})
		return err
	})

	require.ErrorIs(t, err, ErrInvalidWAFMode)
	requireUpdateTestRowEqual(t, before, loadUpdateTestRow(t, db))
}

func strPtr(v string) *string { return &v }
