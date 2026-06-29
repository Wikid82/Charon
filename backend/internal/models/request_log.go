package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// RequestLog records a single proxied HTTP request for dashboard statistics.
// ClientIPHash stores the first 16 bytes of the SHA-256 hash of the client IP
// as a hex string (GDPR-safe pseudonymisation).
type RequestLog struct {
	ID         string    `json:"id" gorm:"primaryKey;type:text"`
	HostID     string    `json:"host_id" gorm:"type:text;not null;index"`
	Timestamp  time.Time `json:"timestamp" gorm:"not null;index"`
	Method     string    `json:"method" gorm:"type:text;not null"`
	StatusCode int       `json:"status_code" gorm:"not null;index"`
	BytesSent  int64     `json:"bytes_sent" gorm:"not null"`
	DurationMs int64     `json:"duration_ms" gorm:"not null"`
	// gorm-scanner:ignore — ClientIPHash is a GDPR-safe SHA-256 pseudonymisation (first 16 bytes, hex-encoded); not raw PII.
	ClientIPHash string `json:"client_ip_hash" gorm:"type:text"`
}

// BeforeCreate sets a new UUID on ID if the caller did not supply one.
func (r *RequestLog) BeforeCreate(_ *gorm.DB) error {
	if r.ID == "" {
		r.ID = uuid.New().String()
	}
	return nil
}
