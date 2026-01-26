// Package models defines the data types used throughout the application.
package models

// SecurityLogEntry represents a security-relevant log entry for live streaming.
// This struct is used by the LogWatcher service to broadcast parsed Caddy access logs
// with security event annotations to WebSocket clients.
type SecurityLogEntry struct {
	Timestamp   string         `json:"timestamp"`
	Level       string         `json:"level"`
	Logger      string         `json:"logger"`
	ClientIP    string         `json:"client_ip"`
	Method      string         `json:"method"`
	URI         string         `json:"uri"`
	Status      int            `json:"status"`
	Duration    float64        `json:"duration"`
	Size        int64          `json:"size"`
	UserAgent   string         `json:"user_agent"`
	Host        string         `json:"host"`
	Source      string         `json:"source"`                 // "waf", "crowdsec", "ratelimit", "acl", "normal"
	Blocked     bool           `json:"blocked"`                // True if request was blocked
	BlockReason string         `json:"block_reason,omitempty"` // Reason for blocking
	Details     map[string]any `json:"details,omitempty"`      // Additional metadata
}
