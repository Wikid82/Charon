package services

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/orthrus"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const (
	authKeyPrefix  = "ch_orthrus_"
	authKeyRandLen = 32 // bytes of random hex

	// maxExternalProxyBindLen bounds the stored listen setting.
	maxExternalProxyBindLen = 64
)

// ErrInvalidExternalProxyBind is returned when a listen setting is neither
// automatic, all interfaces, an address, nor a subnet. The handler maps it to a
// fixed operator-facing message and error code.
var ErrInvalidExternalProxyBind = errors.New("external proxy bind is invalid")

// OrthrusService provides agent lifecycle management on top of OrthrusServer.
type OrthrusService struct {
	db     *gorm.DB
	server *orthrus.OrthrusServer
}

// NewOrthrusService creates an OrthrusService.
func NewOrthrusService(db *gorm.DB, server *orthrus.OrthrusServer) *OrthrusService {
	return &OrthrusService{db: db, server: server}
}

// List returns all registered OrthrusAgent records.
func (s *OrthrusService) List() ([]models.OrthrusAgent, error) {
	var agents []models.OrthrusAgent
	if err := s.db.Find(&agents).Error; err != nil {
		return nil, fmt.Errorf("orthrus: list agents: %w", err)
	}
	return agents, nil
}

// Provision creates a new agent and returns it along with the plaintext auth key.
// The key is only available at this point and is not stored in plaintext.
func (s *OrthrusService) Provision(name string) (models.OrthrusAgent, string, error) {
	rawBytes := make([]byte, authKeyRandLen)
	if _, err := rand.Read(rawBytes); err != nil {
		return models.OrthrusAgent{}, "", fmt.Errorf("orthrus: generate auth key: %w", err)
	}
	plainKey := authKeyPrefix + hex.EncodeToString(rawBytes)

	hash, err := bcrypt.GenerateFromPassword(orthrus.BcryptInput(plainKey), orthrus.BcryptCost)
	if err != nil {
		return models.OrthrusAgent{}, "", fmt.Errorf("orthrus: hash auth key: %w", err)
	}

	agent := models.OrthrusAgent{
		Name:        name,
		AuthKeyHash: string(hash),
		Status:      models.OrthrusStatusPending,
	}

	if err := s.db.Create(&agent).Error; err != nil {
		return models.OrthrusAgent{}, "", fmt.Errorf("orthrus: create agent: %w", err)
	}

	return agent, plainKey, nil
}

// Get retrieves a single OrthrusAgent by UUID.
func (s *OrthrusService) Get(uuid string) (*models.OrthrusAgent, error) {
	var agent models.OrthrusAgent
	if err := s.db.Where("uuid = ?", uuid).First(&agent).Error; err != nil {
		return nil, fmt.Errorf("orthrus: get agent %s: %w", uuid, err)
	}
	return &agent, nil
}

// AgentPatch is a partial update to an agent. Only non-nil fields are written.
type AgentPatch struct {
	Name              *string
	HecateTunnelUUID  *string
	DeviceID          *string
	ResolvedAddress   *string
	ExternalProxyPort *int
	ExternalProxyBind *string
	WriteEnabled      *bool
}

// Patch applies a partial update to an agent.
//
// WriteEnabled has no range validation (unlike ExternalProxyPort) since a
// bool has no invalid values; the audit entry for this specific field is
// emitted by the caller (OrthrusHandler.Patch), not here, matching this
// codebase's existing convention of logging admin-initiated audit events at
// the handler layer (see e.g. security_handler.go, crowdsec_handler.go)
// rather than growing OrthrusService's dependencies to include
// *services.SecurityService and a *gin.Context-derived actor.
func (s *OrthrusService) Patch(uuid string, p AgentPatch) (*models.OrthrusAgent, error) {
	updates := map[string]interface{}{}
	if p.Name != nil {
		trimmed := strings.TrimSpace(*p.Name)
		if trimmed == "" {
			return nil, fmt.Errorf("orthrus: agent name cannot be blank")
		}
		updates["name"] = trimmed
	}
	if p.HecateTunnelUUID != nil {
		updates["hecate_tunnel_uuid"] = *p.HecateTunnelUUID
	}
	if p.DeviceID != nil {
		updates["device_id"] = *p.DeviceID
	}
	if p.ResolvedAddress != nil {
		updates["resolved_address"] = *p.ResolvedAddress
	}
	if p.ExternalProxyPort != nil {
		port := *p.ExternalProxyPort
		if port != 0 && (port < 1024 || port > 65535) {
			return nil, fmt.Errorf("external_proxy_port must be 0 (disabled) or in range 1024–65535, got %d", port)
		}
		updates["external_proxy_port"] = port
	}
	if p.ExternalProxyBind != nil {
		bind, err := normalizeExternalProxyBind(*p.ExternalProxyBind)
		if err != nil {
			return nil, fmt.Errorf("orthrus: patch external proxy bind: %w", err)
		}
		updates["external_proxy_bind"] = bind
	}
	if p.WriteEnabled != nil {
		updates["write_enabled"] = *p.WriteEnabled
	}
	if len(updates) == 0 {
		return s.Get(uuid)
	}
	if err := s.db.Model(&models.OrthrusAgent{}).Where("uuid = ?", uuid).Updates(updates).Error; err != nil {
		return nil, fmt.Errorf("orthrus: patch agent %s: %w", uuid, err)
	}
	return s.Get(uuid)
}

// Rename updates the display name of an agent.
func (s *OrthrusService) Rename(uuid, newName string) (*models.OrthrusAgent, error) {
	return s.Patch(uuid, AgentPatch{Name: &newName})
}

// Delete removes an agent from the database (does not revoke/disconnect first).
func (s *OrthrusService) Delete(uuid string) error {
	if err := s.db.Where("uuid = ?", uuid).Delete(&models.OrthrusAgent{}).Error; err != nil {
		return fmt.Errorf("orthrus: delete agent %s: %w", uuid, err)
	}
	return nil
}

// Revoke invalidates an agent's auth key by replacing the hash with an
// unguessable value, then disconnects any active session.
func (s *OrthrusService) Revoke(uuid string) error {
	invalidBytes := make([]byte, 32)
	if _, err := rand.Read(invalidBytes); err != nil {
		return fmt.Errorf("orthrus: generate revoke token: %w", err)
	}
	invalidHash, err := bcrypt.GenerateFromPassword(invalidBytes[:32], orthrus.BcryptCost)
	if err != nil {
		return fmt.Errorf("orthrus: hash revoke token: %w", err)
	}

	if err := s.db.Model(&models.OrthrusAgent{}).
		Where("uuid = ?", uuid).
		Update("auth_key_hash", string(invalidHash)).Error; err != nil {
		return fmt.Errorf("orthrus: revoke agent %s: %w", uuid, err)
	}

	_ = s.server.DisconnectAgent(uuid)
	return nil
}

// wsURL converts an http/https base URL to the canonical Orthrus WebSocket
// endpoint URL (wss://host/api/v1/ws/orthrus/connect). If the input already
// uses a ws/wss scheme it is returned unchanged (path appended if missing).
func wsURL(base string) string {
	switch {
	case strings.HasPrefix(base, "https://"):
		base = "wss://" + strings.TrimPrefix(base, "https://")
	case strings.HasPrefix(base, "http://"):
		base = "ws://" + strings.TrimPrefix(base, "http://")
	}
	base = strings.TrimRight(base, "/")
	if !strings.HasSuffix(base, "/api/v1/ws/orthrus/connect") {
		base += "/api/v1/ws/orthrus/connect"
	}
	return base
}

// GetInstallSnippets returns platform-specific install templates for an agent.
// The placeholder "<AUTH_KEY>" is used in place of the actual key, which is
// only provided once at Provision time.
func (s *OrthrusService) GetInstallSnippets(uuid, charonURL string) (*orthrus.InstallSnippets, error) {
	agent, err := s.Get(uuid)
	if err != nil {
		return nil, err
	}

	name := agent.Name
	agentUUID := agent.UUID
	serverURL := wsURL(charonURL)

	return &orthrus.InstallSnippets{
		// The agent drops from root to PUID/PGID at startup (agent/privdrop), so
		// every snippet starts it as root. PGID must equal the host Docker
		// socket's group GID: `stat -c %g /var/run/docker.sock`.
		DockerCompose: fmt.Sprintf(`services:
  %s:
    image: wikid82/orthrus:latest
    restart: always
    container_name: orthrus
    environment:
      # Run as this user/group. Replace <DOCKER_GID> with the output of: stat -c %%g /var/run/docker.sock
      - PUID=65532
      - PGID=<DOCKER_GID>
      - TZ=UTC
      - ORTHRUS_SERVER_URL=%s
      - ORTHRUS_AGENT_ID=%s
      - ORTHRUS_AUTH_KEY=<AUTH_KEY>
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
`, name, serverURL, agentUUID),

		Systemd: fmt.Sprintf(`[Unit]
Description=Charon Orthrus Agent (%s)
After=network.target

[Service]
# Do not set a non-root account: the agent must start as root to drop to PUID/PGID.
# PGID must equal the Docker socket's group GID: stat -c %%g /var/run/docker.sock
Environment="PUID=65532"
Environment="PGID=<DOCKER_GID>"
Environment="ORTHRUS_SERVER_URL=%s"
Environment="ORTHRUS_AGENT_ID=%s"
Environment="ORTHRUS_AUTH_KEY=<AUTH_KEY>"
ExecStart=/usr/local/bin/charon-agent
Restart=on-failure

[Install]
WantedBy=multi-user.target
`, name, serverURL, agentUUID),

		// sudo keeps the process root so the agent can drop to PUID/PGID. A
		// plain (non-root) run ignores PUID/PGID, so that user must be in the
		// docker group instead. Replace <DOCKER_GID> with: stat -c %%g /var/run/docker.sock
		Tarball: fmt.Sprintf(`curl -fsSL https://github.com/Wikid82/charon/releases/latest/download/charon-agent-linux-amd64.tar.gz | tar xz
sudo env PUID=65532 PGID=<DOCKER_GID> ORTHRUS_SERVER_URL=%s ORTHRUS_AGENT_ID=%s ORTHRUS_AUTH_KEY=<AUTH_KEY> ./charon-agent
`, serverURL, agentUUID),

		Homebrew: fmt.Sprintf(`brew install wikid82/tap/charon-agent
sudo env PUID=65532 PGID=<DOCKER_GID> ORTHRUS_SERVER_URL=%s ORTHRUS_AGENT_ID=%s ORTHRUS_AUTH_KEY=<AUTH_KEY> charon-agent
`, serverURL, agentUUID),

		KubernetesDaemonSet: fmt.Sprintf(`apiVersion: apps/v1
kind: DaemonSet
metadata:
  name: orthrus-agent
spec:
  selector:
    matchLabels:
      app: orthrus-agent
  template:
    metadata:
      labels:
        app: orthrus-agent
    spec:
      # The container starts non-root, so the agent skips privilege dropping and
      # PUID/PGID are not needed. The socket group GID (<DOCKER_GID>) is granted
      # as a supplemental group so the process can open the mounted socket.
      securityContext:
        runAsNonRoot: true
        runAsUser: 65532
        runAsGroup: 65532
        supplementalGroups:
          - <DOCKER_GID>
      containers:
        - name: agent
          image: wikid82/orthrus:latest
          env:
            - name: ORTHRUS_SERVER_URL
              value: "%s"
            - name: ORTHRUS_AGENT_ID
              value: "%s"
            - name: ORTHRUS_AUTH_KEY
              value: "<AUTH_KEY>"
          securityContext:
            allowPrivilegeEscalation: false
            readOnlyRootFilesystem: true
            capabilities:
              drop:
                - ALL
          volumeMounts:
            - name: docker-sock
              mountPath: /var/run/docker.sock
              readOnly: true
      volumes:
        - name: docker-sock
          hostPath:
            path: /var/run/docker.sock
            type: Socket
`, serverURL, agentUUID),
	}, nil
}

// normalizeExternalProxyBind validates a listen setting and returns its stored
// form: trimmed, with a subnet masked. Reachability is not checked here,
// because interfaces can change; the bind result is reported in proxy-status.
func normalizeExternalProxyBind(raw string) (string, error) {
	s := strings.TrimSpace(raw)
	if len(s) > maxExternalProxyBindLen {
		return "", ErrInvalidExternalProxyBind
	}
	mode, addr, prefix, err := orthrus.ParseBindSetting(s)
	if err != nil {
		return "", ErrInvalidExternalProxyBind
	}
	switch mode {
	case orthrus.BindModeAuto:
		return "", nil
	case orthrus.BindModeSubnet:
		return prefix.String(), nil
	default: // all interfaces or one address
		return addr.String(), nil
	}
}
