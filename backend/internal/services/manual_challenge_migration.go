package services

import (
	"fmt"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"gorm.io/gorm"
)

// OwnerlessChallengeMigrationMarker is the settings row recording that the
// one-time cleanup of challenges without an owner has run. The "migration."
// prefix keeps it out of the settings API (see isInternalSettingKey).
const OwnerlessChallengeMigrationMarker = "migration.manual_challenge_ownerless_expired"

// ExpireOwnerlessChallenges marks still-active challenges that have no owning
// user (user_id = 0) as expired. Such rows cannot be reached by any signed-in
// user, so leaving them active would only block new challenges for the same
// domain until they time out. A marker row makes the cleanup one-shot and
// idempotent; rows are only ever moved from an active state to expired.
func ExpireOwnerlessChallenges(db *gorm.DB) error {
	var markers int64
	if err := db.Model(&models.Setting{}).
		Where("key = ?", OwnerlessChallengeMigrationMarker).Count(&markers).Error; err != nil {
		return fmt.Errorf("check challenge cleanup marker: %w", err)
	}
	if markers > 0 {
		return nil
	}

	res := db.Model(&models.ManualChallenge{}).
		Where("user_id = ? AND status IN ?", 0, []models.ChallengeStatus{
			models.ChallengeStatusCreated,
			models.ChallengeStatusPending,
			models.ChallengeStatusVerifying,
		}).
		Updates(map[string]any{
			"status":        models.ChallengeStatusExpired,
			"error_message": "Challenge closed: no owning user",
		})
	if res.Error != nil {
		return fmt.Errorf("expire ownerless challenges: %w", res.Error)
	}
	if res.RowsAffected > 0 {
		logger.Log().WithField("count", res.RowsAffected).Info("closed manual DNS challenges that had no owning user")
	}

	marker := models.Setting{
		Key:      OwnerlessChallengeMigrationMarker,
		Value:    "done",
		Type:     "string",
		Category: "migration",
	}
	if err := db.Where(models.Setting{Key: marker.Key}).Attrs(marker).FirstOrCreate(&marker).Error; err != nil {
		return fmt.Errorf("write challenge cleanup marker: %w", err)
	}
	return nil
}
