package models

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func trustedProxySettingTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open("file:"+t.TempDir()+"/trusted.db"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&Setting{}))
	return db
}

func TestReadTrustedProxiesSetting_AbsentIsNil(t *testing.T) {
	db := trustedProxySettingTestDB(t)
	stored, err := ReadTrustedProxiesSetting(db)
	require.NoError(t, err)
	assert.Nil(t, stored)
}

func TestWriteTrustedProxiesSetting_WriteReadReplaceRemove(t *testing.T) {
	db := trustedProxySettingTestDB(t)
	value := "172.18.0.5/32"
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		return WriteTrustedProxiesSetting(tx, &value)
	}))
	stored, err := ReadTrustedProxiesSetting(db)
	require.NoError(t, err)
	require.NotNil(t, stored)
	assert.Equal(t, value, *stored)

	empty := ""
	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		return WriteTrustedProxiesSetting(tx, &empty)
	}))
	stored, err = ReadTrustedProxiesSetting(db)
	require.NoError(t, err)
	require.NotNil(t, stored, "an empty saved list is stored, not removed")
	assert.Empty(t, *stored)

	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		return WriteTrustedProxiesSetting(tx, nil)
	}))
	stored, err = ReadTrustedProxiesSetting(db)
	require.NoError(t, err)
	assert.Nil(t, stored)
}

func TestReadTrustedProxiesSetting_ReadErrorIsWrapped(t *testing.T) {
	db := trustedProxySettingTestDB(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	_, err = ReadTrustedProxiesSetting(db)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "read security.proxy.trusted_proxies setting")
}

func TestWriteTrustedProxiesSetting_WriteErrorIsWrapped(t *testing.T) {
	db := trustedProxySettingTestDB(t)
	value := "10.0.0.1/32"
	require.NoError(t, WriteTrustedProxiesSetting(db, &value))
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	require.Error(t, WriteTrustedProxiesSetting(db, &value))
	require.Error(t, WriteTrustedProxiesSetting(db, nil))
}
