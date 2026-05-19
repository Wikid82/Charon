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
