package models

import (
	"time"
)

// ImportSession tracks Caddyfile import operations with pending state
// until user reviews and confirms via UI.
type ImportSession struct {
	ID              uint       `json:"id" gorm:"primaryKey"`
	UUID            string     `json:"uuid" gorm:"uniqueIndex"`
	SourceFile      string     `json:"source_file"`                       // Path to original Caddyfile
	Status          string     `json:"status" gorm:"default:'pending'"`   // "pending", "reviewing", "committed", "rejected", "failed"
	ParsedData      string     `json:"parsed_data" gorm:"type:text"`      // JSON representation of detected hosts
	ConflictReport  string     `json:"conflict_report" gorm:"type:text"`  // JSON array of conflicts
	UserResolutions string     `json:"user_resolutions" gorm:"type:text"` // JSON map of conflict resolutions
	ErrorMsg        string     `json:"error_msg"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	CommittedAt     *time.Time `json:"committed_at,omitempty"`
}
