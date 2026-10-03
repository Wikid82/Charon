package services

import (
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupChallengeMigrationDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.ManualChallenge{}, &models.Setting{}))
	return db
}

func seedChallenge(t *testing.T, db *gorm.DB, id string, userID uint, status models.ChallengeStatus) {
	t.Helper()
	require.NoError(t, db.Create(&models.ManualChallenge{
		ID: id, ProviderID: 1, UserID: userID, FQDN: id + ".example.com",
		Value: "v", Status: status, ExpiresAt: time.Now().Add(time.Hour),
	}).Error)
}

func challengeStatus(t *testing.T, db *gorm.DB, id string) models.ChallengeStatus {
	t.Helper()
	var ch models.ManualChallenge
	require.NoError(t, db.First(&ch, "id = ?", id).Error)
	return ch.Status
}

func TestExpireOwnerlessChallenges(t *testing.T) {
	db := setupChallengeMigrationDB(t)
	seedChallenge(t, db, "orphan-pending", 0, models.ChallengeStatusPending)
	seedChallenge(t, db, "orphan-created", 0, models.ChallengeStatusCreated)
	seedChallenge(t, db, "orphan-verifying", 0, models.ChallengeStatusVerifying)
	seedChallenge(t, db, "orphan-verified", 0, models.ChallengeStatusVerified)
	seedChallenge(t, db, "owned-pending", 7, models.ChallengeStatusPending)

	require.NoError(t, ExpireOwnerlessChallenges(db))

	assert.Equal(t, models.ChallengeStatusExpired, challengeStatus(t, db, "orphan-pending"))
	assert.Equal(t, models.ChallengeStatusExpired, challengeStatus(t, db, "orphan-created"))
	assert.Equal(t, models.ChallengeStatusExpired, challengeStatus(t, db, "orphan-verifying"))
	assert.Equal(t, models.ChallengeStatusVerified, challengeStatus(t, db, "orphan-verified"), "terminal rows are untouched")
	assert.Equal(t, models.ChallengeStatusPending, challengeStatus(t, db, "owned-pending"), "owned rows are untouched")

	var markers int64
	require.NoError(t, db.Model(&models.Setting{}).Where("key = ?", OwnerlessChallengeMigrationMarker).Count(&markers).Error)
	assert.Equal(t, int64(1), markers)
}

func TestExpireOwnerlessChallenges_RunsOnce(t *testing.T) {
	db := setupChallengeMigrationDB(t)
	require.NoError(t, ExpireOwnerlessChallenges(db))

	// A row appearing after the first run is left alone: the cleanup is one-shot.
	seedChallenge(t, db, "late-orphan", 0, models.ChallengeStatusPending)
	require.NoError(t, ExpireOwnerlessChallenges(db))

	assert.Equal(t, models.ChallengeStatusPending, challengeStatus(t, db, "late-orphan"))

	var markers int64
	require.NoError(t, db.Model(&models.Setting{}).Where("key = ?", OwnerlessChallengeMigrationMarker).Count(&markers).Error)
	assert.Equal(t, int64(1), markers)
}

func TestExpireOwnerlessChallenges_DatabaseError(t *testing.T) {
	db := setupChallengeMigrationDB(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	assert.Error(t, ExpireOwnerlessChallenges(db))
}

func TestExpireOwnerlessChallenges_UpdateError(t *testing.T) {
	db := setupChallengeMigrationDB(t)
	require.NoError(t, db.Migrator().DropTable(&models.ManualChallenge{}))

	assert.Error(t, ExpireOwnerlessChallenges(db))
}

func TestExpireOwnerlessChallenges_MarkerWriteError(t *testing.T) {
	db := setupChallengeMigrationDB(t)
	require.NoError(t, db.Exec(`CREATE TRIGGER reject_marker BEFORE INSERT ON settings
		BEGIN SELECT RAISE(ABORT, 'rejected'); END`).Error)

	assert.Error(t, ExpireOwnerlessChallenges(db))
}
