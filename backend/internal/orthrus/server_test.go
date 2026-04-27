package orthrus

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

func setupServerTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := filepath.Join(t.TempDir(), "orthrus_test.db")
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })

	require.NoError(t, db.AutoMigrate(&models.OrthrusAgent{}))
	return db
}

func setupTestCA(t *testing.T) *InternalCA {
	t.Helper()
	ca, err := NewInternalCA(t.TempDir())
	require.NoError(t, err)
	return ca
}

func TestNewOrthrusServer_Initialises(t *testing.T) {
	db := setupServerTestDB(t)
	ca := setupTestCA(t)

	srv, err := NewOrthrusServer(db, ca)
	require.NoError(t, err)
	assert.NotNil(t, srv)
}

func TestOrthrusServer_GetProxyAddr_UnknownUUID(t *testing.T) {
	db := setupServerTestDB(t)
	srv, err := NewOrthrusServer(db, setupTestCA(t))
	require.NoError(t, err)

	addr, ok := srv.GetProxyAddr("nonexistent-uuid")
	assert.Equal(t, "", addr)
	assert.False(t, ok)
}

func TestOrthrusServer_GetSession_UnknownUUID(t *testing.T) {
	db := setupServerTestDB(t)
	srv, err := NewOrthrusServer(db, setupTestCA(t))
	require.NoError(t, err)

	sess, ok := srv.GetSession("nonexistent-uuid")
	assert.Nil(t, sess)
	assert.False(t, ok)
}

func TestOrthrusServer_DisconnectAgent_UnknownUUID_Noop(t *testing.T) {
	db := setupServerTestDB(t)
	srv, err := NewOrthrusServer(db, setupTestCA(t))
	require.NoError(t, err)

	err = srv.DisconnectAgent("ghost-uuid")
	assert.NoError(t, err)
}

func TestExtractBearer(t *testing.T) {
	tests := []struct {
		header   string
		expected string
	}{
		{"Bearer abc123", "abc123"},
		{"bearer abc123", ""},
		{"Token abc123", ""},
		{"", ""},
		{"Bearer ", ""},
	}

	for _, tc := range tests {
		assert.Equal(t, tc.expected, extractBearer(tc.header), "header: %q", tc.header)
	}
}

func TestOrthrusServer_FindAgentByToken_NoAgents(t *testing.T) {
	db := setupServerTestDB(t)
	srv, err := NewOrthrusServer(db, setupTestCA(t))
	require.NoError(t, err)

	_, err = srv.findAgentByToken("ch_orthrus_sometoken")
	assert.Error(t, err)
}
