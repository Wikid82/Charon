package services

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

// TestAccessListDelete_AssignedWhileDeleting covers a host that takes the list after the in-use checks ran. The
// reference rule refuses the delete, and the service reports the list as in use.
func TestAccessListDelete_AssignedWhileDeleting(t *testing.T) {
	db := openReferenceEnforcedDB(t)
	acl := models.AccessList{UUID: uuid.NewString(), Name: "office", Enabled: true}
	require.NoError(t, db.Create(&acl).Error)
	aclID := acl.ID

	// Simulates a host created between the pre-checks and the delete.
	const callback = "test:assign_list_before_delete"
	assigned := false
	require.NoError(t, db.Callback().Delete().Before("gorm:delete").Register(callback, func(tx *gorm.DB) {
		if tx.Statement.Schema == nil || tx.Statement.Schema.Name != "AccessList" || assigned {
			return
		}
		assigned = true
		host := models.ProxyHost{UUID: uuid.NewString(), DomainNames: "late.example.com", ForwardHost: "10.0.0.3", ForwardPort: 80, AccessListID: &aclID}
		if err := tx.Session(&gorm.Session{NewDB: true}).Create(&host).Error; err != nil {
			t.Errorf("assign host during delete: %v", err)
		}
	}))
	t.Cleanup(func() { _ = db.Callback().Delete().Remove(callback) })

	err := NewAccessListService(db).Delete(aclID)

	require.True(t, assigned)
	assert.ErrorIs(t, err, ErrAccessListInUse)
	var remaining int64
	require.NoError(t, db.Model(&models.AccessList{}).Where("id = ?", aclID).Count(&remaining).Error)
	assert.Equal(t, int64(1), remaining, "the list must survive a refused delete")
}
