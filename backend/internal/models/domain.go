package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Domain struct {
	ID        uint           `json:"id" gorm:"primarykey"`
	UUID      string         `json:"uuid" gorm:"uniqueIndex;not null"`
	Name      string         `json:"name" gorm:"uniqueIndex;not null"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `json:"deleted_at" gorm:"index"`
}

func (d *Domain) BeforeCreate(tx *gorm.DB) (err error) {
	if d.UUID == "" {
		d.UUID = uuid.New().String()
	}
	return
}
