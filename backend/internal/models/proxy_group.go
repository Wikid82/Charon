package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// ProxyGroup represents a named group for organizing proxy hosts.
type ProxyGroup struct {
	ID          uint      `json:"-" gorm:"primaryKey"`
	UUID        string    `json:"uuid" gorm:"uniqueIndex;not null"`
	Name        string    `json:"name" gorm:"not null;index"`
	Description string    `json:"description" gorm:"type:text"`
	Color       string    `json:"color" gorm:"default:#6366f1"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// BeforeCreate assigns a UUID if one has not been set.
func (g *ProxyGroup) BeforeCreate(tx *gorm.DB) (err error) {
	if g.UUID == "" {
		g.UUID = uuid.New().String()
	}
	return
}
