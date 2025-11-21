package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type NotificationType string

const (
	NotificationTypeInfo    NotificationType = "info"
	NotificationTypeSuccess NotificationType = "success"
	NotificationTypeWarning NotificationType = "warning"
	NotificationTypeError   NotificationType = "error"
)

type Notification struct {
	ID        string           `gorm:"primaryKey" json:"id"`
	Type      NotificationType `json:"type"`
	Title     string           `json:"title"`
	Message   string           `json:"message"`
	Read      bool             `json:"read"`
	CreatedAt time.Time        `json:"created_at"`
}

func (n *Notification) BeforeCreate(tx *gorm.DB) (err error) {
	if n.ID == "" {
		n.ID = uuid.New().String()
	}
	return
}
