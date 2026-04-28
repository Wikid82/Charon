package services_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/orthrus"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupOrthrusTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	dsn := filepath.Join(t.TempDir(), "orthrus_svc_test.db")
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = sqlDB.Close() })

	require.NoError(t, db.AutoMigrate(&models.OrthrusAgent{}))
	return db
}

func setupOrthrusServer(t *testing.T, db *gorm.DB) *orthrus.OrthrusServer {
	t.Helper()
	ca, err := orthrus.NewInternalCA(t.TempDir())
	require.NoError(t, err)
	srv, err := orthrus.NewOrthrusServer(db, ca)
	require.NoError(t, err)
	return srv
}

func TestOrthrusService_Provision_KeyPrefix(t *testing.T) {
	db := setupOrthrusTestDB(t)
	svc := services.NewOrthrusService(db, setupOrthrusServer(t, db))

	_, key, err := svc.Provision("agent-1")
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(key, "ch_orthrus_"), "key should have prefix ch_orthrus_")
}

func TestOrthrusService_Provision_DifferentKeysEachTime(t *testing.T) {
	db := setupOrthrusTestDB(t)
	svc := services.NewOrthrusService(db, setupOrthrusServer(t, db))

	_, key1, err := svc.Provision("agent-1")
	require.NoError(t, err)

	_, key2, err := svc.Provision("agent-2")
	require.NoError(t, err)

	assert.NotEqual(t, key1, key2)
}

func TestOrthrusService_Provision_HashNotEqualPlaintext(t *testing.T) {
	db := setupOrthrusTestDB(t)
	svc := services.NewOrthrusService(db, setupOrthrusServer(t, db))

	agent, key, err := svc.Provision("agent-1")
	require.NoError(t, err)

	assert.NotEmpty(t, agent.AuthKeyHash)
	assert.NotEqual(t, key, agent.AuthKeyHash)
}

func TestOrthrusService_Provision_StatusIsPending(t *testing.T) {
	db := setupOrthrusTestDB(t)
	svc := services.NewOrthrusService(db, setupOrthrusServer(t, db))

	agent, _, err := svc.Provision("pending-agent")
	require.NoError(t, err)
	assert.Equal(t, models.OrthrusStatusPending, agent.Status)
}

func TestOrthrusService_List(t *testing.T) {
	db := setupOrthrusTestDB(t)
	svc := services.NewOrthrusService(db, setupOrthrusServer(t, db))

	for i := 0; i < 3; i++ {
		_, _, err := svc.Provision("agent")
		require.NoError(t, err)
	}

	agents, err := svc.List()
	require.NoError(t, err)
	assert.Len(t, agents, 3)
}

func TestOrthrusService_Get(t *testing.T) {
	db := setupOrthrusTestDB(t)
	svc := services.NewOrthrusService(db, setupOrthrusServer(t, db))

	agent, _, err := svc.Provision("test-agent")
	require.NoError(t, err)

	got, err := svc.Get(agent.UUID)
	require.NoError(t, err)
	assert.Equal(t, agent.UUID, got.UUID)
	assert.Equal(t, "test-agent", got.Name)
}

func TestOrthrusService_Get_NotFound(t *testing.T) {
	db := setupOrthrusTestDB(t)
	svc := services.NewOrthrusService(db, setupOrthrusServer(t, db))

	_, err := svc.Get("nonexistent")
	assert.Error(t, err)
}

func TestOrthrusService_Delete(t *testing.T) {
	db := setupOrthrusTestDB(t)
	svc := services.NewOrthrusService(db, setupOrthrusServer(t, db))

	agent, _, err := svc.Provision("delete-me")
	require.NoError(t, err)

	require.NoError(t, svc.Delete(agent.UUID))

	_, err = svc.Get(agent.UUID)
	assert.Error(t, err)
}

func TestOrthrusService_GetInstallSnippets(t *testing.T) {
	db := setupOrthrusTestDB(t)
	svc := services.NewOrthrusService(db, setupOrthrusServer(t, db))

	agent, _, err := svc.Provision("snippet-agent")
	require.NoError(t, err)

	snippets, err := svc.GetInstallSnippets(agent.UUID, "https://charon.example.com")
	require.NoError(t, err)
	require.NotNil(t, snippets)

	assert.Contains(t, snippets.DockerCompose, "<AUTH_KEY>")
	assert.Contains(t, snippets.Systemd, "<AUTH_KEY>")
	assert.Contains(t, snippets.Tarball, "<AUTH_KEY>")
	assert.Contains(t, snippets.Homebrew, "<AUTH_KEY>")
	assert.Contains(t, snippets.KubernetesDaemonSet, "<AUTH_KEY>")
	assert.Contains(t, snippets.DockerCompose, "https://charon.example.com")
	assert.Contains(t, snippets.DockerCompose, "snippet-agent")
}

func TestOrthrusService_List_DBError(t *testing.T) {
	db := setupOrthrusTestDB(t)
	svc := services.NewOrthrusService(db, setupOrthrusServer(t, db))

	sqlDB, err := db.DB()
	require.NoError(t, err)
	_ = sqlDB.Close()

	_, err = svc.List()
	assert.Error(t, err)
}

func TestOrthrusService_Provision_DBError(t *testing.T) {
	db := setupOrthrusTestDB(t)
	svc := services.NewOrthrusService(db, setupOrthrusServer(t, db))

	sqlDB, err := db.DB()
	require.NoError(t, err)
	_ = sqlDB.Close()

	_, _, err = svc.Provision("error-agent")
	assert.Error(t, err)
}

func TestOrthrusService_Delete_DBError(t *testing.T) {
	db := setupOrthrusTestDB(t)
	svc := services.NewOrthrusService(db, setupOrthrusServer(t, db))

	agent, _, err := svc.Provision("to-delete")
	require.NoError(t, err)

	sqlDB, sqlErr := db.DB()
	require.NoError(t, sqlErr)
	_ = sqlDB.Close()

	err = svc.Delete(agent.UUID)
	assert.Error(t, err)
}

func TestOrthrusService_Revoke_Success(t *testing.T) {
	db := setupOrthrusTestDB(t)
	svc := services.NewOrthrusService(db, setupOrthrusServer(t, db))

	agent, _, err := svc.Provision("revoke-me")
	require.NoError(t, err)

	err = svc.Revoke(agent.UUID)
	assert.NoError(t, err)
}

func TestOrthrusService_Revoke_DBError(t *testing.T) {
	db := setupOrthrusTestDB(t)
	svc := services.NewOrthrusService(db, setupOrthrusServer(t, db))

	agent, _, err := svc.Provision("revoke-dberror")
	require.NoError(t, err)

	sqlDB, sqlErr := db.DB()
	require.NoError(t, sqlErr)
	_ = sqlDB.Close()

	err = svc.Revoke(agent.UUID)
	assert.Error(t, err)
}

func TestOrthrusService_GetInstallSnippets_NotFound(t *testing.T) {
	db := setupOrthrusTestDB(t)
	svc := services.NewOrthrusService(db, setupOrthrusServer(t, db))

	_, err := svc.GetInstallSnippets("nonexistent-uuid", "https://charon.example.com")
	assert.Error(t, err)
}
