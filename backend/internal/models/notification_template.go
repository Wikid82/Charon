package models

import (
    "time"

    "github.com/google/uuid"
    "gorm.io/gorm"
)

// NotificationTemplate represents a reusable external notification template
// that can be applied when sending webhooks or other external notifications.
type NotificationTemplate struct {
    ID          string    `gorm:"primaryKey" json:"id"`
    Name        string    `json:"name"`
    Description string    `json:"description"`
    // Config holds the JSON/template body for external webhook payloads
    Config      string    `json:"config"`
    // Template is a hint: minimal|detailed|custom (optional)
    Template    string    `json:"template" gorm:"default:minimal"`

    CreatedAt time.Time  `json:"created_at"`
    UpdatedAt time.Time  `json:"updated_at"`
}

func (t *NotificationTemplate) BeforeCreate(tx *gorm.DB) (err error) {
    if t.ID == "" {
        t.ID = uuid.New().String()
    }
    return
}
