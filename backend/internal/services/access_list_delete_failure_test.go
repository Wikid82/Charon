package services

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

func TestAccessListDelete_StorageFailureIsReported(t *testing.T) {
	db := openReferenceEnforcedDB(t)
	acl := models.AccessList{UUID: uuid.NewString(), Name: "refused", Enabled: true}
	require.NoError(t, db.Create(&acl).Error)
	refuseOn(t, db, "DELETE", "access_lists")

	err := NewAccessListService(db).Delete(acl.ID)

	assert.ErrorContains(t, err, "delete access list")
	var remaining int64
	require.NoError(t, db.Model(&models.AccessList{}).Where("id = ?", acl.ID).Count(&remaining).Error)
	assert.Equal(t, int64(1), remaining)
}
