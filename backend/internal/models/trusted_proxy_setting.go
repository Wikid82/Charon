package models

import (
	"errors"
	"fmt"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/Wikid82/charon/backend/internal/security"
)

// ReadTrustedProxiesSetting returns the saved proxy-layer trust list text, or nil when the operator has not saved one. An
// empty string is a saved list that trusts loopback only, which is different from nil.
func ReadTrustedProxiesSetting(db *gorm.DB) (*string, error) {
	var row Setting
	err := db.Where("key = ?", security.TrustedProxiesSettingKey).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read %s setting: %w", security.TrustedProxiesSettingKey, err)
	}
	return &row.Value, nil
}

// WriteTrustedProxiesSetting writes the saved trust list inside tx. A non-nil value is stored, and nil removes the row so
// the environment or default list applies again. Call it inside a write-locked transaction.
func WriteTrustedProxiesSetting(tx *gorm.DB, value *string) error {
	if value == nil {
		if err := tx.Where("key = ?", security.TrustedProxiesSettingKey).Delete(&Setting{}).Error; err != nil {
			return fmt.Errorf("remove %s setting: %w", security.TrustedProxiesSettingKey, err)
		}
		return nil
	}
	setting := Setting{Key: security.TrustedProxiesSettingKey, Value: *value, Category: "security", Type: "string"}
	err := tx.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "key"}},
		DoUpdates: clause.AssignmentColumns([]string{"value", "type", "category", "updated_at"}),
	}).Create(&setting).Error
	if err != nil {
		return fmt.Errorf("write %s setting: %w", security.TrustedProxiesSettingKey, err)
	}
	return nil
}
