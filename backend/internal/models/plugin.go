package models

import "time"

// Plugin represents an installed DNS provider plugin.
// This tracks both external .so plugins and their load status.
type Plugin struct {
	ID        uint   `json:"-" gorm:"primaryKey"`
	UUID      string `json:"uuid" gorm:"uniqueIndex;size:36"`
	Name      string `json:"name" gorm:"not null;size:255"`
	Type      string `json:"type" gorm:"uniqueIndex;not null;size:100"`
	FilePath  string `json:"file_path" gorm:"not null;size:500"`
	Signature string `json:"signature" gorm:"size:100"`
	Enabled   bool   `json:"enabled" gorm:"default:true"`
	Status    string `json:"status" gorm:"default:'pending';size:50"` // pending, loaded, error
	Error     string `json:"error,omitempty" gorm:"type:text"`
	Version   string `json:"version,omitempty" gorm:"size:50"`
	Author    string `json:"author,omitempty" gorm:"size:255"`

	LoadedAt  *time.Time `json:"loaded_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// TableName specifies the database table name for GORM.
func (Plugin) TableName() string {
	return "plugins"
}

// PluginStatus constants define the possible status values for a plugin.
const (
	PluginStatusPending = "pending" // Plugin registered but not yet loaded
	PluginStatusLoaded  = "loaded"  // Plugin successfully loaded and registered
	PluginStatusError   = "error"   // Plugin failed to load
)
