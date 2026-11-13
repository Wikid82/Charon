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
	"gopkg.in/yaml.v3"
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

func TestOrthrusService_GetInstallSnippets_PermissionSettings(t *testing.T) {
	db := setupOrthrusTestDB(t)
	svc := services.NewOrthrusService(db, setupOrthrusServer(t, db))

	agent, _, err := svc.Provision("perm-agent")
	require.NoError(t, err)

	snippets, err := svc.GetInstallSnippets(agent.UUID, "https://charon.example.com")
	require.NoError(t, err)
	require.NotNil(t, snippets)

	// Compose: reference service shape from docs/features/orthrus.md.
	assert.Contains(t, snippets.DockerCompose, "restart: always")
	assert.NotContains(t, snippets.DockerCompose, "unless-stopped")
	assert.Contains(t, snippets.DockerCompose, "container_name: orthrus")
	assert.Contains(t, snippets.DockerCompose, "- PUID=65532")
	assert.Contains(t, snippets.DockerCompose, "- PGID=<DOCKER_GID>")
	assert.Contains(t, snippets.DockerCompose, "- TZ=UTC")
	assert.Contains(t, snippets.DockerCompose, "/var/run/docker.sock:/var/run/docker.sock:ro")

	// Systemd: the agent only drops privileges when started as root, so the
	// unit must not set User=; PUID/PGID are passed as environment instead.
	assert.Contains(t, snippets.Systemd, `Environment="PUID=65532"`)
	assert.Contains(t, snippets.Systemd, `Environment="PGID=<DOCKER_GID>"`)
	assert.NotContains(t, snippets.Systemd, "User=")

	// Tarball and Homebrew: sudo keeps the process root so privdrop applies.
	assert.Contains(t, snippets.Tarball, "sudo env PUID=65532 PGID=<DOCKER_GID> ")
	assert.Contains(t, snippets.Homebrew, "sudo env PUID=65532 PGID=<DOCKER_GID> ")

	// Kubernetes: container starts non-root and mounts the socket read-only.
	assert.Contains(t, snippets.KubernetesDaemonSet, "runAsNonRoot: true")
	assert.Contains(t, snippets.KubernetesDaemonSet, "runAsUser: 65532")
	assert.Contains(t, snippets.KubernetesDaemonSet, "runAsGroup: 65532")
	assert.Contains(t, snippets.KubernetesDaemonSet, "supplementalGroups:")
	assert.Contains(t, snippets.KubernetesDaemonSet, "- <DOCKER_GID>")
	assert.Contains(t, snippets.KubernetesDaemonSet, "allowPrivilegeEscalation: false")
	assert.Contains(t, snippets.KubernetesDaemonSet, "readOnlyRootFilesystem: true")
	assert.Contains(t, snippets.KubernetesDaemonSet, "drop:")
	assert.Contains(t, snippets.KubernetesDaemonSet, "- ALL")
	assert.Contains(t, snippets.KubernetesDaemonSet, "readOnly: true")
	assert.Contains(t, snippets.KubernetesDaemonSet, "type: Socket")
}

func TestOrthrusService_GetInstallSnippets_RenderedYAMLIsValid(t *testing.T) {
	db := setupOrthrusTestDB(t)
	svc := services.NewOrthrusService(db, setupOrthrusServer(t, db))

	agent, _, err := svc.Provision("yaml-agent")
	require.NoError(t, err)

	snippets, err := svc.GetInstallSnippets(agent.UUID, "https://charon.example.com")
	require.NoError(t, err)

	var compose struct {
		Services map[string]struct {
			Image         string   `yaml:"image"`
			Restart       string   `yaml:"restart"`
			ContainerName string   `yaml:"container_name"`
			Environment   []string `yaml:"environment"`
			Volumes       []string `yaml:"volumes"`
		} `yaml:"services"`
	}
	require.NoError(t, yaml.Unmarshal([]byte(snippets.DockerCompose), &compose))
	svcDef, ok := compose.Services["yaml-agent"]
	require.True(t, ok, "compose service must be named after the agent")
	assert.Equal(t, "wikid82/orthrus:latest", svcDef.Image)
	assert.Equal(t, "always", svcDef.Restart)
	assert.Equal(t, "orthrus", svcDef.ContainerName)
	assert.Contains(t, svcDef.Environment, "PUID=65532")
	assert.Contains(t, svcDef.Environment, "PGID=<DOCKER_GID>")
	assert.Contains(t, svcDef.Volumes, "/var/run/docker.sock:/var/run/docker.sock:ro")

	var ds struct {
		Spec struct {
			Template struct {
				Spec struct {
					SecurityContext struct {
						RunAsNonRoot       bool     `yaml:"runAsNonRoot"`
						RunAsUser          int      `yaml:"runAsUser"`
						RunAsGroup         int      `yaml:"runAsGroup"`
						SupplementalGroups []string `yaml:"supplementalGroups"`
					} `yaml:"securityContext"`
					Containers []struct {
						SecurityContext struct {
							AllowPrivilegeEscalation bool `yaml:"allowPrivilegeEscalation"`
							ReadOnlyRootFilesystem   bool `yaml:"readOnlyRootFilesystem"`
							Capabilities             struct {
								Drop []string `yaml:"drop"`
							} `yaml:"capabilities"`
						} `yaml:"securityContext"`
					} `yaml:"containers"`
				} `yaml:"spec"`
			} `yaml:"template"`
		} `yaml:"spec"`
	}
	require.NoError(t, yaml.Unmarshal([]byte(snippets.KubernetesDaemonSet), &ds))
	pod := ds.Spec.Template.Spec
	assert.True(t, pod.SecurityContext.RunAsNonRoot)
	assert.Equal(t, 65532, pod.SecurityContext.RunAsUser)
	assert.Equal(t, 65532, pod.SecurityContext.RunAsGroup)
	assert.Equal(t, []string{"<DOCKER_GID>"}, pod.SecurityContext.SupplementalGroups)
	require.Len(t, pod.Containers, 1)
	c := pod.Containers[0].SecurityContext
	assert.False(t, c.AllowPrivilegeEscalation)
	assert.True(t, c.ReadOnlyRootFilesystem)
	assert.Equal(t, []string{"ALL"}, c.Capabilities.Drop)
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
	got, err := svc.Patch(agent.UUID, services.AgentPatch{Name: &newName})
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

	got, err := svc.Patch(agent.UUID, services.AgentPatch{HecateTunnelUUID: &tunnelUUID, DeviceID: &deviceID, ResolvedAddress: &resolved})
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
	_, err = svc.Patch(agent.UUID, services.AgentPatch{Name: &blank})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "cannot be blank")
}

func TestOrthrusService_Patch_EmptyUpdate(t *testing.T) {
	db := setupOrthrusTestDB(t)
	svc := services.NewOrthrusService(db, setupOrthrusServer(t, db))

	agent, _, err := svc.Provision("no-change-agent")
	require.NoError(t, err)

	got, err := svc.Patch(agent.UUID, services.AgentPatch{})
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "no-change-agent", got.Name)
	assert.Equal(t, agent.UUID, got.UUID)
}

func TestOrthrusService_Patch_UnknownUUID(t *testing.T) {
	db := setupOrthrusTestDB(t)
	svc := services.NewOrthrusService(db, setupOrthrusServer(t, db))

	newName := "irrelevant"
	_, err := svc.Patch("00000000-0000-0000-0000-000000000000", services.AgentPatch{Name: &newName})
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
	got, err := svc.Patch(agent.UUID, services.AgentPatch{ExternalProxyPort: &port})
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
	got, err := svc.Patch(agent.UUID, services.AgentPatch{ExternalProxyPort: &port})
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
	_, err = svc.Patch(agent.UUID, services.AgentPatch{ExternalProxyPort: &port})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "external_proxy_port")
}

func TestOrthrusService_Patch_ExternalProxyPort_TooHigh(t *testing.T) {
	db := setupOrthrusTestDB(t)
	svc := services.NewOrthrusService(db, setupOrthrusServer(t, db))

	agent, _, err := svc.Provision("port-too-high")
	require.NoError(t, err)

	port := 70000
	_, err = svc.Patch(agent.UUID, services.AgentPatch{ExternalProxyPort: &port})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "external_proxy_port")
}

func TestOrthrusService_Patch_ExternalProxyPort_Negative(t *testing.T) {
	db := setupOrthrusTestDB(t)
	svc := services.NewOrthrusService(db, setupOrthrusServer(t, db))

	agent, _, err := svc.Provision("port-negative")
	require.NoError(t, err)

	port := -1
	_, err = svc.Patch(agent.UUID, services.AgentPatch{ExternalProxyPort: &port})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "external_proxy_port")
}

func TestOrthrusService_Patch_WriteEnabled_DefaultsFalse(t *testing.T) {
	db := setupOrthrusTestDB(t)
	svc := services.NewOrthrusService(db, setupOrthrusServer(t, db))

	agent, _, err := svc.Provision("write-mode-default")
	require.NoError(t, err)
	assert.False(t, agent.WriteEnabled)
}

func TestOrthrusService_Patch_WriteEnabled_True(t *testing.T) {
	db := setupOrthrusTestDB(t)
	svc := services.NewOrthrusService(db, setupOrthrusServer(t, db))

	agent, _, err := svc.Provision("write-mode-enable")
	require.NoError(t, err)

	enabled := true
	got, err := svc.Patch(agent.UUID, services.AgentPatch{WriteEnabled: &enabled})
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.True(t, got.WriteEnabled)
}

func TestOrthrusService_Patch_WriteEnabled_BackToFalse(t *testing.T) {
	db := setupOrthrusTestDB(t)
	svc := services.NewOrthrusService(db, setupOrthrusServer(t, db))

	agent, _, err := svc.Provision("write-mode-disable")
	require.NoError(t, err)

	enabled := true
	_, err = svc.Patch(agent.UUID, services.AgentPatch{WriteEnabled: &enabled})
	require.NoError(t, err)

	disabled := false
	got, err := svc.Patch(agent.UUID, services.AgentPatch{WriteEnabled: &disabled})
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.False(t, got.WriteEnabled)
}

func TestOrthrusService_Patch_WriteEnabled_NilLeavesUnchanged(t *testing.T) {
	db := setupOrthrusTestDB(t)
	svc := services.NewOrthrusService(db, setupOrthrusServer(t, db))

	agent, _, err := svc.Provision("write-mode-untouched")
	require.NoError(t, err)

	enabled := true
	_, err = svc.Patch(agent.UUID, services.AgentPatch{WriteEnabled: &enabled})
	require.NoError(t, err)

	newName := "renamed-agent"
	got, err := svc.Patch(agent.UUID, services.AgentPatch{Name: &newName})
	require.NoError(t, err)
	require.NotNil(t, got)
	assert.Equal(t, "renamed-agent", got.Name)
	assert.True(t, got.WriteEnabled, "write_enabled must be untouched by an unrelated Patch call")
}
