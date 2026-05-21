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

	const expectedWSURL = "wss://charon.example.com/api/v1/ws/orthrus/connect"

	assert.Contains(t, snippets.DockerCompose, "<AUTH_KEY>")
	assert.Contains(t, snippets.Systemd, "<AUTH_KEY>")
	assert.Contains(t, snippets.Tarball, "<AUTH_KEY>")
	assert.Contains(t, snippets.Homebrew, "<AUTH_KEY>")
	assert.Contains(t, snippets.KubernetesDaemonSet, "<AUTH_KEY>")

	assert.Contains(t, snippets.DockerCompose, expectedWSURL, "Docker Compose snippet must use wss:// URL")
	assert.Contains(t, snippets.Systemd, expectedWSURL, "systemd snippet must use wss:// URL")
	assert.Contains(t, snippets.Tarball, expectedWSURL, "tarball snippet must use wss:// URL")
	assert.Contains(t, snippets.Homebrew, expectedWSURL, "homebrew snippet must use wss:// URL")
	assert.Contains(t, snippets.KubernetesDaemonSet, expectedWSURL, "Kubernetes snippet must use wss:// URL")
	assert.Contains(t, snippets.DockerCompose, "snippet-agent")
}

func TestOrthrusService_GetInstallSnippets_URLConversions(t *testing.T) {
	db := setupOrthrusTestDB(t)
	svc := services.NewOrthrusService(db, setupOrthrusServer(t, db))

	agent, _, err := svc.Provision("url-test-agent")
	require.NoError(t, err)

	tests := []struct {
		name        string
		inputURL    string
		expectedURL string
	}{
		{
			name:        "https converts to wss with path",
			inputURL:    "https://charon.example.com",
			expectedURL: "wss://charon.example.com/api/v1/ws/orthrus/connect",
		},
		{
			name:        "https with trailing slash converts to wss with path",
			inputURL:    "https://charon.example.com/",
			expectedURL: "wss://charon.example.com/api/v1/ws/orthrus/connect",
		},
		{
			name:        "http converts to ws with path",
			inputURL:    "http://localhost:8080",
			expectedURL: "ws://localhost:8080/api/v1/ws/orthrus/connect",
		},
		{
			name:        "wss already correct is kept as-is",
			inputURL:    "wss://charon.example.com/api/v1/ws/orthrus/connect",
			expectedURL: "wss://charon.example.com/api/v1/ws/orthrus/connect",
		},
		{
			name:        "wss without path gets path appended",
			inputURL:    "ws://localhost:8080",
			expectedURL: "ws://localhost:8080/api/v1/ws/orthrus/connect",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			snippets, err := svc.GetInstallSnippets(agent.UUID, tc.inputURL)
			require.NoError(t, err)
			assert.Contains(t, snippets.DockerCompose, tc.expectedURL)
			assert.Contains(t, snippets.Systemd, tc.expectedURL)
			assert.Contains(t, snippets.Tarball, tc.expectedURL)
			assert.Contains(t, snippets.Homebrew, tc.expectedURL)
			assert.Contains(t, snippets.KubernetesDaemonSet, tc.expectedURL)
		})
	}
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

func TestOrthrusService_Patch_NameOnly(t *testing.T) {
	db := setupOrthrusTestDB(t)
	svc := services.NewOrthrusService(db, setupOrthrusServer(t, db))

	agent, _, err := svc.Provision("original-name")
	require.NoError(t, err)

	newName := "updated-name"
	got, err := svc.Patch(agent.UUID, &newName, nil, nil, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "updated-name", got.Name)
	assert.Equal(t, agent.UUID, got.UUID)
}

func TestOrthrusService_Patch_ProviderFields(t *testing.T) {
	db := setupOrthrusTestDB(t)
	svc := services.NewOrthrusService(db, setupOrthrusServer(t, db))

	agent, _, err := svc.Provision("provider-agent")
	require.NoError(t, err)

	tunnelUUID := "tunnel-uuid-123"
	deviceID := "device-id-456"
	resolved := "10.0.0.1"

	got, err := svc.Patch(agent.UUID, nil, &tunnelUUID, &deviceID, &resolved, nil)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "provider-agent", got.Name)
	require.NotNil(t, got.HecateTunnelUUID)
	assert.Equal(t, tunnelUUID, *got.HecateTunnelUUID)
	require.NotNil(t, got.DeviceID)
	assert.Equal(t, deviceID, *got.DeviceID)
	require.NotNil(t, got.ResolvedAddress)
	assert.Equal(t, resolved, *got.ResolvedAddress)
}

func TestOrthrusService_Patch_BlankName(t *testing.T) {
	db := setupOrthrusTestDB(t)
	svc := services.NewOrthrusService(db, setupOrthrusServer(t, db))

	agent, _, err := svc.Provision("valid-name")
	require.NoError(t, err)

	blank := "   "
	_, err = svc.Patch(agent.UUID, &blank, nil, nil, nil, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be blank")
}

func TestOrthrusService_Patch_EmptyUpdate(t *testing.T) {
	db := setupOrthrusTestDB(t)
	svc := services.NewOrthrusService(db, setupOrthrusServer(t, db))

	agent, _, err := svc.Provision("no-change-agent")
	require.NoError(t, err)

	got, err := svc.Patch(agent.UUID, nil, nil, nil, nil, nil)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "no-change-agent", got.Name)
	assert.Equal(t, agent.UUID, got.UUID)
}

func TestOrthrusService_Patch_UnknownUUID(t *testing.T) {
	db := setupOrthrusTestDB(t)
	svc := services.NewOrthrusService(db, setupOrthrusServer(t, db))

	newName := "irrelevant"
	_, err := svc.Patch("00000000-0000-0000-0000-000000000000", &newName, nil, nil, nil, nil)
	require.Error(t, err)
	assert.ErrorIs(t, err, gorm.ErrRecordNotFound)
}

func TestOrthrusService_Rename_DelegatesToPatch(t *testing.T) {
	db := setupOrthrusTestDB(t)
	svc := services.NewOrthrusService(db, setupOrthrusServer(t, db))

	agent, _, err := svc.Provision("rename-before")
	require.NoError(t, err)

	got, err := svc.Rename(agent.UUID, "rename-after")
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "rename-after", got.Name)
	assert.Equal(t, agent.UUID, got.UUID)
}

func TestOrthrusService_Patch_ExternalProxyPort_Zero(t *testing.T) {
	db := setupOrthrusTestDB(t)
	svc := services.NewOrthrusService(db, setupOrthrusServer(t, db))

	agent, _, err := svc.Provision("port-zero")
	require.NoError(t, err)

	port := 0
	got, err := svc.Patch(agent.UUID, nil, nil, nil, nil, &port)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, 0, got.ExternalProxyPort)
}

func TestOrthrusService_Patch_ExternalProxyPort_Valid(t *testing.T) {
	db := setupOrthrusTestDB(t)
	svc := services.NewOrthrusService(db, setupOrthrusServer(t, db))

	agent, _, err := svc.Provision("port-valid")
	require.NoError(t, err)

	port := 2375
	got, err := svc.Patch(agent.UUID, nil, nil, nil, nil, &port)
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, 2375, got.ExternalProxyPort)
}

func TestOrthrusService_Patch_ExternalProxyPort_TooLow(t *testing.T) {
	db := setupOrthrusTestDB(t)
	svc := services.NewOrthrusService(db, setupOrthrusServer(t, db))

	agent, _, err := svc.Provision("port-too-low")
	require.NoError(t, err)

	port := 1023
	_, err = svc.Patch(agent.UUID, nil, nil, nil, nil, &port)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "external_proxy_port")
}

func TestOrthrusService_Patch_ExternalProxyPort_TooHigh(t *testing.T) {
	db := setupOrthrusTestDB(t)
	svc := services.NewOrthrusService(db, setupOrthrusServer(t, db))

	agent, _, err := svc.Provision("port-too-high")
	require.NoError(t, err)

	port := 70000
	_, err = svc.Patch(agent.UUID, nil, nil, nil, nil, &port)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "external_proxy_port")
}

func TestOrthrusService_Patch_ExternalProxyPort_Negative(t *testing.T) {
	db := setupOrthrusTestDB(t)
	svc := services.NewOrthrusService(db, setupOrthrusServer(t, db))

	agent, _, err := svc.Provision("port-negative")
	require.NoError(t, err)

	port := -1
	_, err = svc.Patch(agent.UUID, nil, nil, nil, nil, &port)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "external_proxy_port")
}
