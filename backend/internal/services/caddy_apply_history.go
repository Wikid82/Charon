package services

import (
	"context"
	"fmt"
	"time"

	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/database"
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
)

// CaddyApplyErrorTextMarker is the settings key that records the one-time clearing of the message text of stored
// proxy apply results. Its value is the time of the run.
const CaddyApplyErrorTextMarker = "migration.caddy_apply_error_text_v1"

// ClearStoredApplyErrorText clears the message text of the stored proxy apply results, once per database. The time,
// the outcome and the config hash of each result stay. The run is recorded by CaddyApplyErrorTextMarker, so later
// calls find it and do nothing. It returns the number of results cleared.
func ClearStoredApplyErrorText(ctx context.Context, db *gorm.DB) (int64, error) {
	var cleared int64
	err := database.WithSecureDelete(db.WithContext(ctx), func(conn *gorm.DB) error {
		return database.WithWriteLock(conn, &models.CaddyConfig{}, func(locked *gorm.DB) error {
			// Each statement starts from a fresh chain, so a Model set by one query does not reach the next.
			tx := locked.Session(&gorm.Session{NewDB: true})
			done, err := applyErrorTextMarked(tx)
			if err != nil || done {
				return err
			}
			res := tx.Model(&models.CaddyConfig{}).Where("error_msg <> ''").Update("error_msg", "")
			if res.Error != nil {
				return fmt.Errorf("clearing stored apply messages: %w", res.Error)
			}
			cleared = res.RowsAffected
			marker := models.Setting{
				Key:      CaddyApplyErrorTextMarker,
				Value:    time.Now().UTC().Format(time.RFC3339),
				Type:     "string",
				Category: "migration",
			}
			if err := tx.Create(&marker).Error; err != nil {
				return fmt.Errorf("recording apply text marker: %w", err)
			}
			return nil
		})
	})
	if err != nil {
		return 0, fmt.Errorf("clearing stored proxy apply messages: %w", err)
	}
	if cleared > 0 {
		logger.Log().WithField("count", cleared).Info("Cleared the message text of earlier proxy apply results")
	}
	return cleared, nil
}

// applyErrorTextMarked reports whether the one-time clearing has already run.
func applyErrorTextMarked(db *gorm.DB) (bool, error) {
	var marked int64
	if err := db.Model(&models.Setting{}).Where("key = ?", CaddyApplyErrorTextMarker).Count(&marked).Error; err != nil {
		return false, fmt.Errorf("reading apply text marker: %w", err)
	}
	return marked > 0, nil
}
