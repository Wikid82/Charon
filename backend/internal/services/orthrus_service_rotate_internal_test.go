package services

import (
	"errors"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

// recordingDisconnector stands in for the live OrthrusServer session table.
type recordingDisconnector struct {
	calls []string
	err   error
}

func (r *recordingDisconnector) DisconnectAgent(agentUUID string) error {
	r.calls = append(r.calls, agentUUID)
	return r.err
}

func TestOrthrusService_RotateKey_DisconnectsLiveSession(t *testing.T) {
	db := setupOrthrusInternalDB(t)
	stub := &recordingDisconnector{}
	svc := &OrthrusService{db: db, server: stub}

	agent, _, err := svc.Provision("live-agent")
	require.NoError(t, err)

	_, err = svc.RotateKey(agent.UUID)
	require.NoError(t, err)
	assert.Equal(t, []string{agent.UUID}, stub.calls)
}

func TestOrthrusService_RotateKey_DisconnectErrorDoesNotFailRotation(t *testing.T) {
	db := setupOrthrusInternalDB(t)
	svc := &OrthrusService{db: db, server: &recordingDisconnector{err: errors.New("close failed")}}

	agent, _, err := svc.Provision("flaky-agent")
	require.NoError(t, err)

	key, err := svc.RotateKey(agent.UUID)
	require.NoError(t, err)
	assert.NotEmpty(t, key)
}

func TestOrthrusService_RotateKey_UnknownUUIDDoesNotDisconnect(t *testing.T) {
	db := setupOrthrusInternalDB(t)
	stub := &recordingDisconnector{}
	svc := &OrthrusService{db: db, server: stub}

	_, err := svc.RotateKey("missing")
	require.Error(t, err)
	assert.Empty(t, stub.calls)
}

func TestOrthrusService_NilServerSkipsDisconnect(t *testing.T) {
	db := setupOrthrusInternalDB(t)
	svc := NewOrthrusService(db, nil)

	agent, _, err := svc.Provision("no-server")
	require.NoError(t, err)

	_, err = svc.RotateKey(agent.UUID)
	require.NoError(t, err)
	require.NoError(t, svc.Revoke(agent.UUID))
}

func setupOrthrusInternalDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "orthrus_internal.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.OrthrusAgent{}))
	return db
}
