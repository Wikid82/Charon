package services

import (
	"crypto/rand"
	"encoding/hex"
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
	bcryptCost     = 12
)

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

	// Truncate to bcrypt's 72-byte input limit so the full key is compared;
	// bcrypt is the sole password hashing primitive — no pre-hash step needed.
	keyBytes := []byte(plainKey)
	if len(keyBytes) >= 72 {
		keyBytes = keyBytes[:71]
	}
	hash, err := bcrypt.GenerateFromPassword(keyBytes, bcryptCost)
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

// Rename updates the display name of an agent.
func (s *OrthrusService) Rename(uuid, newName string) (*models.OrthrusAgent, error) {
	if err := s.db.Model(&models.OrthrusAgent{}).
		Where("uuid = ?", uuid).
		Update("name", newName).Error; err != nil {
		return nil, fmt.Errorf("orthrus: rename agent %s: %w", uuid, err)
	}
	return s.Get(uuid)
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
	invalidHash, err := bcrypt.GenerateFromPassword(invalidBytes[:32], bcryptCost)
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
		DockerCompose: fmt.Sprintf(`services:
  %s:
    image: wikid82/orthrus:latest
    environment:
      - ORTHRUS_SERVER_URL=%s
      - ORTHRUS_AGENT_ID=%s
      - ORTHRUS_AUTH_KEY=<AUTH_KEY>
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock:ro
    restart: unless-stopped
`, name, serverURL, agentUUID),

		Systemd: fmt.Sprintf(`[Unit]
Description=Charon Orthrus Agent (%s)
After=network.target

[Service]
Environment="ORTHRUS_SERVER_URL=%s"
Environment="ORTHRUS_AGENT_ID=%s"
Environment="ORTHRUS_AUTH_KEY=<AUTH_KEY>"
ExecStart=/usr/local/bin/charon-agent
Restart=on-failure

[Install]
WantedBy=multi-user.target
`, name, serverURL, agentUUID),

		Tarball: fmt.Sprintf(`curl -fsSL https://github.com/Wikid82/charon/releases/latest/download/charon-agent-linux-amd64.tar.gz | tar xz
ORTHRUS_SERVER_URL=%s ORTHRUS_AGENT_ID=%s ORTHRUS_AUTH_KEY=<AUTH_KEY> ./charon-agent
`, serverURL, agentUUID),

		Homebrew: fmt.Sprintf(`brew install wikid82/tap/charon-agent
ORTHRUS_SERVER_URL=%s ORTHRUS_AGENT_ID=%s ORTHRUS_AUTH_KEY=<AUTH_KEY> charon-agent
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
          volumeMounts:
            - name: docker-sock
              mountPath: /var/run/docker.sock
              readOnly: true
      volumes:
        - name: docker-sock
          hostPath:
            path: /var/run/docker.sock
`, serverURL, agentUUID),
	}, nil
}
