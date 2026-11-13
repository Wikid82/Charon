package main

import (
	"fmt"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
	"gorm.io/gorm"
)

// resetUserPassword sets a new password for the user with the given email through
// the same path as the API password change, so every existing session ends and
// failed-attempt and lock state is cleared.
func resetUserPassword(db *gorm.DB, email, newPassword string) error {
	var user models.User
	if err := db.Where("email = ?", email).First(&user).Error; err != nil {
		return fmt.Errorf("user not found: %w", err)
	}

	if err := user.SetPassword(newPassword); err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		return services.ApplyPasswordChange(tx, user.ID, user.PasswordHash)
	}); err != nil {
		return fmt.Errorf("save password: %w", err)
	}
	return nil
}
