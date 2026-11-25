package models

import "time"

// EncryptionKeyVersion is one row of the encryption key registry. It gives a
// configured encryption key a stable version number that follows the key
// itself, not the environment variable it is configured in.
//
// KeyID is the first 32 hex characters of HMAC-SHA256 over the decoded key
// bytes, so the same key has the same id however it is base64-encoded, and the
// id reveals nothing that helps recover the key.
type EncryptionKeyVersion struct {
	ID          uint      `json:"-" gorm:"primaryKey"`
	Version     int       `json:"version" gorm:"uniqueIndex;not null"`
	KeyID       string    `json:"key_id" gorm:"uniqueIndex;size:32;not null"`
	FirstSeenAt time.Time `json:"first_seen_at"`
	LastSeenAt  time.Time `json:"last_seen_at"`
}
