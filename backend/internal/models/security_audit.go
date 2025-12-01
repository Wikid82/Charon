package models

import (
    "time"
)

// SecurityAudit records admin actions or important changes related to security.
type SecurityAudit struct {
    ID        uint      `json:"id" gorm:"primaryKey"`
    UUID      string    `json:"uuid" gorm:"uniqueIndex"`
    Actor     string    `json:"actor"`
    Action    string    `json:"action"`
    Details   string    `json:"details" gorm:"type:text"`
    CreatedAt time.Time `json:"created_at"`
}
