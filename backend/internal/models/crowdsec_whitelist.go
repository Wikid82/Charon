package models

import "time"

// CrowdSecWhitelist represents a single IP or CIDR block that CrowdSec should never ban.
type CrowdSecWhitelist struct {
	ID        uint      `json:"-"          gorm:"primaryKey"`
	UUID      string    `json:"uuid"       gorm:"uniqueIndex;not null"`
	IPOrCIDR  string    `json:"ip_or_cidr" gorm:"not null;uniqueIndex"`
	Reason    string    `json:"reason"     gorm:"not null;default:''"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}
