package models

import (
	"time"

	"golang.org/x/crypto/bcrypt"
)

// User represents authenticated users with role-based access control.
// Supports local auth, SSO integration planned for later phases.
type User struct {
	ID                  uint       `json:"id" gorm:"primaryKey"`
	UUID                string     `json:"uuid" gorm:"uniqueIndex"`
	Email               string     `json:"email" gorm:"uniqueIndex"`
	APIKey              string     `json:"api_key" gorm:"uniqueIndex"` // For external API access
	PasswordHash        string     `json:"-"`                          // Never serialize password hash
	Name                string     `json:"name"`
	Role                string     `json:"role" gorm:"default:'user'"` // "admin", "user", "viewer"
	Enabled             bool       `json:"enabled" gorm:"default:true"`
	FailedLoginAttempts int        `json:"-" gorm:"default:0"`
	LockedUntil         *time.Time `json:"-"`
	LastLogin           *time.Time `json:"last_login,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
}

// SetPassword hashes and sets the user's password.
func (u *User) SetPassword(password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	u.PasswordHash = string(hash)
	return nil
}

// CheckPassword compares the provided password with the stored hash.
func (u *User) CheckPassword(password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte(password))
	return err == nil
}
