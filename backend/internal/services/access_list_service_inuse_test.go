package services

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAccessListService_InUse_CountFailureIsReturned(t *testing.T) {
	db := setupTestDB(t)
	service := NewAccessListService(db)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	inUse, err := service.InUse(1)
	require.Error(t, err)
	require.ErrorContains(t, err, "count proxy hosts using access list 1")
	require.False(t, inUse)
}
