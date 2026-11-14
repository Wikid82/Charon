package services_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/orthrus"
	"github.com/Wikid82/charon/backend/internal/services"
)

func ptrTo[T any](v T) *T { return &v }

func TestOrthrusService_RotateKey_ReturnsNewKeyAndReplacesHash(t *testing.T) {
	db := setupOrthrusTestDB(t)
	svc := services.NewOrthrusService(db, setupOrthrusServer(t, db))

	agent, oldKey, err := svc.Provision("rotate-me")
	require.NoError(t, err)

	newKey, err := svc.RotateKey(agent.UUID)
	require.NoError(t, err)

	assert.True(t, strings.HasPrefix(newKey, "ch_orthrus_"), "rotated key should use the key prefix")
	assert.NotEqual(t, oldKey, newKey)

	stored, err := svc.Get(agent.UUID)
	require.NoError(t, err)
	assert.NotEqual(t, agent.AuthKeyHash, stored.AuthKeyHash, "hash should change on rotation")
	assert.NoError(t, bcrypt.CompareHashAndPassword([]byte(stored.AuthKeyHash), orthrus.BcryptInput(newKey)),
		"new key should verify against the stored hash")
	assert.Error(t, bcrypt.CompareHashAndPassword([]byte(stored.AuthKeyHash), orthrus.BcryptInput(oldKey)),
		"old key should no longer verify")
}

func TestOrthrusService_RotateKey_KeepsIdentityAndLinks(t *testing.T) {
	db := setupOrthrusTestDB(t)
	svc := services.NewOrthrusService(db, setupOrthrusServer(t, db))

	agent, _, err := svc.Provision("linked-agent")
	require.NoError(t, err)
	_, err = svc.Patch(agent.UUID, services.AgentPatch{
		HecateTunnelUUID:  ptrTo("tunnel-1"),
		ExternalProxyPort: ptrTo(2375),
		ExternalProxyBind: ptrTo("0.0.0.0"),
		WriteEnabled:      ptrTo(true),
	})
	require.NoError(t, err)

	_, err = svc.RotateKey(agent.UUID)
	require.NoError(t, err)

	after, err := svc.Get(agent.UUID)
	require.NoError(t, err)
	assert.Equal(t, agent.UUID, after.UUID)
	assert.Equal(t, "linked-agent", after.Name)
	require.NotNil(t, after.HecateTunnelUUID)
	assert.Equal(t, "tunnel-1", *after.HecateTunnelUUID)
	assert.Equal(t, 2375, after.ExternalProxyPort)
	assert.Equal(t, "0.0.0.0", after.ExternalProxyBind)
	assert.True(t, after.WriteEnabled)
}

func TestOrthrusService_RotateKey_UnknownUUID(t *testing.T) {
	db := setupOrthrusTestDB(t)
	svc := services.NewOrthrusService(db, setupOrthrusServer(t, db))

	_, err := svc.RotateKey("no-such-agent")
	require.Error(t, err)
	assert.True(t, errors.Is(err, gorm.ErrRecordNotFound))
}

func TestOrthrusService_RotateKey_DBError(t *testing.T) {
	db := setupOrthrusTestDB(t)
	svc := services.NewOrthrusService(db, setupOrthrusServer(t, db))

	agent, _, err := svc.Provision("rotate-dberror")
	require.NoError(t, err)

	sqlDB, sqlErr := db.DB()
	require.NoError(t, sqlErr)
	_ = sqlDB.Close()

	_, err = svc.RotateKey(agent.UUID)
	assert.Error(t, err)
}
