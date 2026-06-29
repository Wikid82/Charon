package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// CustomTheme stores a user-created named color-scheme theme.
// Colors is stored as a JSON text blob matching the frontend CustomThemeColors type.
type CustomTheme struct {
	ID        string    `json:"id"         gorm:"primaryKey;type:text"`
	Name      string    `json:"name"       gorm:"type:text;not null;uniqueIndex"`
	Colors    string    `json:"colors"     gorm:"type:text;not null"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// BeforeCreate generates a UUID if ID is empty.
func (ct *CustomTheme) BeforeCreate(tx *gorm.DB) error {
	if ct.ID == "" {
		ct.ID = uuid.New().String()
	}
	return nil
}
