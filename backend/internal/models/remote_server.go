package models

import (
	"time"
)

// ConnectionType determines how Charon reaches a remote server.
type ConnectionType string

const (
	ConnectionTypeDirect     ConnectionType = "direct"
	ConnectionTypeOrthrus    ConnectionType = "orthrus"
	ConnectionTypeCloudflare ConnectionType = "cloudflare"
	ConnectionTypeTailscale  ConnectionType = "tailscale"
	ConnectionTypeNetBird    ConnectionType = "netbird"
	ConnectionTypeZeroTier   ConnectionType = "zerotier"
)

// RemoteServer represents a known backend server that can be selected
// when creating proxy hosts, eliminating manual IP/port entry.
type RemoteServer struct {
	ID          uint       `json:"-" gorm:"primaryKey"`
	UUID        string     `json:"uuid" gorm:"uniqueIndex"`
	Name        string     `json:"name" gorm:"index"`
	Provider    string     `json:"provider" gorm:"index"` // e.g., "docker", "vm", "cloud", "manual"
	Host        string     `json:"host" gorm:"index"`     // IP address or hostname
	Port        int        `json:"port"`
	Scheme      string     `json:"scheme"` // http/https
	Tags        string     `json:"tags"`   // comma-separated tags for filtering
	Description string     `json:"description"`
	Enabled     bool       `json:"enabled" gorm:"default:true;index"`
	LastChecked *time.Time `json:"last_checked,omitempty"`
	Reachable   bool       `json:"reachable" gorm:"default:false"`

	// ConnectionType determines how Charon reaches this server.
	// Values: "direct" (default), "orthrus", "cloudflare"
	ConnectionType ConnectionType `json:"connection_type" gorm:"default:'direct';index"`

	// OrthrusAgentUUID links this server to a registered Orthrus agent.
	// Only populated when ConnectionType == "orthrus".
	OrthrusAgentUUID *string `json:"orthrus_agent_uuid,omitempty" gorm:"index"`

	// HecateTunnelUUID links this server to a Hecate tunnel config.
	// Only populated when ConnectionType is "tailscale", "netbird", or "zerotier".
	HecateTunnelUUID *string `json:"hecate_tunnel_uuid,omitempty" gorm:"index"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
