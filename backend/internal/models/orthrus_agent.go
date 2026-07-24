package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// OrthrusStatus represents the connectivity state of a registered Orthrus agent.
type OrthrusStatus string

const (
	OrthrusStatusOnline  OrthrusStatus = "online"
	OrthrusStatusOffline OrthrusStatus = "offline"
	OrthrusStatusPending OrthrusStatus = "pending"
)

// OrthrusAgent represents a registered remote Orthrus agent.
type OrthrusAgent struct {
	ID          uint          `json:"-" gorm:"primaryKey"`
	UUID        string        `json:"uuid" gorm:"uniqueIndex;not null"`
	Name        string        `json:"name" gorm:"not null;index"`
	AuthKeyHash string        `json:"-" gorm:"not null"` // bcrypt hash of AUTH_KEY — never exposed
	Status      OrthrusStatus `json:"status" gorm:"default:'pending';index"`

	// JSON array of declared capabilities, e.g. ["docker", "tcp:5432"]
	Capabilities string `json:"capabilities" gorm:"type:text"`

	// PEM-encoded certificate issued by Charon's internal CA (not sensitive).
	AgentCertPEM string `json:"agent_cert_pem,omitempty" gorm:"type:text"`

	// HecateTunnelUUID is the TunnelConfig assigned to THIS AGENT for its own outbound
	// connectivity. Distinct from RemoteServer.HecateTunnelUUID which governs how a
	// RemoteServer is reached by Charon.
	HecateTunnelUUID *string `json:"hecate_tunnel_uuid,omitempty" gorm:"index"`

	// DeviceID is the provider-specific peer/device/member identifier within the
	// assigned tunnel (e.g. Tailscale node ID, NetBird peer ID). Empty for Cloudflare.
	DeviceID *string `json:"device_id,omitempty"`

	// ResolvedAddress is the cached connectivity address for this agent,
	// set by Charon at assignment time, used as upstream host in Remote Servers.
	ResolvedAddress *string `json:"resolved_address,omitempty"`

	// ExternalProxyPort is the TCP port bound on 0.0.0.0 for inter-container Docker API access.
	// 0 = disabled. Valid values: 1024–65535.
	ExternalProxyPort int `json:"external_proxy_port" gorm:"default:0"`

	// WriteEnabled opts this agent into a narrow, fixed set of Docker write endpoints
	// (image pull, container start/stop/restart/create/remove) in addition to the
	// unconditional read-only allowlist. false = read-only (default). Enforced
	// independently by both backend/internal/orthrus/muzzle.go and agent/muzzle/muzzle.go.
	// Takes effect on the agent's next reconnect (see AgentSession handshake).
	WriteEnabled bool `json:"write_enabled" gorm:"default:false"`

	// AllowedVolumesFromSources is an operator-configured allowlist of container
	// names/IDs this agent may reference via a /containers/create request's
	// HostConfig.VolumesFrom field. VolumesFrom makes the new container inherit
	// ALL of the named container's mounts, including any host bind mounts —
	// the muzzle cannot itself determine what that entails, so it defers to
	// this explicit, per-agent operator allowlist instead of a blanket
	// allow/deny. Empty (the default) means no VolumesFrom value is accepted
	// at all. Enforced independently by both muzzles, like WriteEnabled, and
	// takes effect on the agent's next reconnect.
	AllowedVolumesFromSources []string `json:"allowed_volumes_from_sources" gorm:"type:text;serializer:json"`

	LastHeartbeat *time.Time `json:"last_heartbeat,omitempty"`
	LastSeen      *time.Time `json:"last_seen,omitempty"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
}

func (oa *OrthrusAgent) BeforeCreate(tx *gorm.DB) (err error) {
	if oa.UUID == "" {
		oa.UUID = uuid.New().String()
	}
	return
}
