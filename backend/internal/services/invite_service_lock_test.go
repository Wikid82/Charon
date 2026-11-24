package services

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

// installWriteBlocker makes every UPDATE on users fail, so writes fail after reads succeed.
func installWriteBlocker(t *testing.T, db *gorm.DB) {
	t.Helper()
	require.NoError(t, db.Exec(
		"CREATE TRIGGER block_user_update BEFORE UPDATE ON users BEGIN SELECT RAISE(ABORT, 'write blocked'); END",
	).Error)
}

func TestInviteService_AcceptRechecksExpiryInsideLock(t *testing.T) {
	db := openInviteTestDB(t)
	svc := newTestInviteService(db)
	invitee := seedPendingInvite(t, db, "clock@example.com", "tok-clock")

	clock := time.Now()
	svc.now = func() time.Time { return clock }
	svc.beforeLock = func() { clock = clock.Add(2 * time.Hour) } // the link lapses between the pre-check and the lock

	_, err := svc.Accept("tok-clock", "Clock", "clock-password", "")
	assert.ErrorIs(t, err, ErrInviteExpired)

	stored := reloadInviteUser(t, db, invitee.ID)
	assert.Empty(t, stored.PasswordHash)
	assert.Equal(t, "tok-clock", stored.InviteToken)
}

func TestInviteService_AcceptRefusesTokenReplacedBeforeLock(t *testing.T) {
	db := openInviteTestDB(t)
	svc := newTestInviteService(db)
	invitee := seedPendingInvite(t, db, "replaced@example.com", "tok-old")

	svc.beforeLock = func() {
		require.NoError(t, db.Model(&models.User{}).Where("id = ?", invitee.ID).Update("invite_token", "tok-new").Error)
	}

	_, err := svc.Accept("tok-old", "Replaced", "replaced-password", "")
	assert.ErrorIs(t, err, ErrInviteNotFound)
	assert.Empty(t, reloadInviteUser(t, db, invitee.ID).PasswordHash)
}

func TestInviteService_AcceptRefusesPasswordSetBeforeLock(t *testing.T) {
	db := openInviteTestDB(t)
	svc := newTestInviteService(db)
	invitee := seedPendingInvite(t, db, "claimed@example.com", "tok-claimed")

	svc.beforeLock = func() {
		require.NoError(t, db.Model(&models.User{}).Where("id = ?", invitee.ID).Update("password_hash", "admin-set-hash").Error)
	}

	_, err := svc.Accept("tok-claimed", "Claimed", "claimed-password", "")
	assert.ErrorIs(t, err, ErrInviteClosed)

	stored := reloadInviteUser(t, db, invitee.ID)
	assert.Equal(t, "admin-set-hash", stored.PasswordHash)
	assert.Equal(t, "tok-claimed", stored.InviteToken)
	assert.Equal(t, models.InviteStatusPending, stored.InviteStatus)
}

func TestInviteService_AcceptReportsWriteFailureInsideLock(t *testing.T) {
	db := openInviteTestDB(t)
	svc := newTestInviteService(db)
	invitee := seedPendingInvite(t, db, "blocked@example.com", "tok-blocked")

	svc.beforeLock = func() { installWriteBlocker(t, db) }

	_, err := svc.Accept("tok-blocked", "Blocked", "blocked-password", "")
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrInviteClosed)
	assert.NotErrorIs(t, err, ErrInviteNotFound)

	stored := reloadInviteUser(t, db, invitee.ID)
	assert.Equal(t, "tok-blocked", stored.InviteToken)
	assert.Empty(t, stored.PasswordHash)
}

func TestInviteService_AcceptExpiredMarkFailureStillReturnsExpired(t *testing.T) {
	db := openInviteTestDB(t)
	svc := newTestInviteService(db)
	invitee := seedPendingInvite(t, db, "markfail@example.com", "tok-markfail")
	require.NoError(t, db.Model(invitee).Update("invite_expires", time.Now().Add(-time.Minute)).Error)
	installWriteBlocker(t, db)

	_, err := svc.Accept("tok-markfail", "Mark", "mark-password", "")
	assert.ErrorIs(t, err, ErrInviteExpired)
	assert.Equal(t, models.InviteStatusPending, reloadInviteUser(t, db, invitee.ID).InviteStatus)
}

func TestInviteService_AcceptNilExpiryFailsClosed(t *testing.T) {
	db := openInviteTestDB(t)
	svc := newTestInviteService(db)
	invitee := seedPendingInvite(t, db, "noexpiry@example.com", "tok-noexpiry")
	require.NoError(t, db.Model(invitee).Update("invite_expires", nil).Error)

	_, err := svc.Accept("tok-noexpiry", "No Expiry", "noexpiry-password", "")
	assert.ErrorIs(t, err, ErrInviteExpired)

	stored := reloadInviteUser(t, db, invitee.ID)
	assert.Empty(t, stored.PasswordHash)
	assert.Equal(t, models.InviteStatusExpired, stored.InviteStatus)
}

func TestInviteService_AcceptLegacyEnabledPendingInviteStillAccepts(t *testing.T) {
	db := openInviteTestDB(t)
	svc := newTestInviteService(db)
	invitee := seedPendingInvite(t, db, "legacy@example.com", "tok-legacy")
	// Invitees were stored enabled before the default was fixed; the predicate ignores enabled.
	require.NoError(t, db.Model(invitee).Update("enabled", true).Error)

	user, err := svc.Accept("tok-legacy", "Legacy", "legacy-password", "")
	require.NoError(t, err)
	assert.Equal(t, invitee.ID, user.ID)
	assert.True(t, reloadInviteUser(t, db, invitee.ID).Enabled)
}

func TestInviteService_ExpiredMarkIsConditionalOnToken(t *testing.T) {
	db := openInviteTestDB(t)
	svc := newTestInviteService(db)
	invitee := seedPendingInvite(t, db, "resent@example.com", "tok-current")

	// A resend replaced the token after the caller read the old one: the old token must not mark the new invite.
	svc.markExpired(invitee.ID, "tok-stale")
	assert.Equal(t, models.InviteStatusPending, reloadInviteUser(t, db, invitee.ID).InviteStatus)

	svc.markExpired(invitee.ID, "tok-current")
	assert.Equal(t, models.InviteStatusExpired, reloadInviteUser(t, db, invitee.ID).InviteStatus)
}

func TestInviteService_AcceptEmptyTokenIsNotFound(t *testing.T) {
	db := openInviteTestDB(t)
	svc := newTestInviteService(db)
	require.NoError(t, CreateKeepingSent(db, &models.User{UUID: "u-empty", APIKey: "k-empty", Email: "none@example.com"}, []string{"enabled"}))

	_, err := svc.Accept("", "Empty", "empty-password", "")
	assert.ErrorIs(t, err, ErrInviteNotFound)
}

func TestConsumeInviteRefusesStaleRow(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(t *testing.T, db *gorm.DB, id uint)
	}{
		{
			name: "token changed",
			mutate: func(t *testing.T, db *gorm.DB, id uint) {
				require.NoError(t, db.Model(&models.User{}).Where("id = ?", id).Update("invite_token", "tok-moved").Error)
			},
		},
		{
			name: "status changed",
			mutate: func(t *testing.T, db *gorm.DB, id uint) {
				require.NoError(t, db.Model(&models.User{}).Where("id = ?", id).Update("invite_status", models.InviteStatusAccepted).Error)
			},
		},
		{
			name: "password set",
			mutate: func(t *testing.T, db *gorm.DB, id uint) {
				require.NoError(t, db.Model(&models.User{}).Where("id = ?", id).Update("password_hash", "set-elsewhere").Error)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := openInviteTestDB(t)
			invitee := seedPendingInvite(t, db, "stale@example.com", "tok-stale-row")

			var stale models.User
			require.NoError(t, db.First(&stale, invitee.ID).Error)
			tt.mutate(t, db, invitee.ID)
			before := reloadInviteUser(t, db, invitee.ID)

			err := WithWriteLock(db, &models.User{}, func(tx *gorm.DB) error {
				return consumeInvite(tx, &stale, "Stale", "new-hash", "")
			})
			assert.ErrorIs(t, err, ErrInviteClosed)

			after := reloadInviteUser(t, db, invitee.ID)
			assert.Equal(t, before.InviteToken, after.InviteToken)
			assert.Equal(t, before.InviteStatus, after.InviteStatus)
			assert.Equal(t, before.PasswordHash, after.PasswordHash)
			assert.Equal(t, before.Name, after.Name)
		})
	}
}

func TestConsumeInviteReportsWriteFailure(t *testing.T) {
	db := openInviteTestDB(t)
	invitee := seedPendingInvite(t, db, "consume@example.com", "tok-consume")
	installWriteBlocker(t, db)

	err := WithWriteLock(db, &models.User{}, func(tx *gorm.DB) error {
		return consumeInvite(tx, invitee, "Consume", "hash", "")
	})
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrInviteClosed)
}

func TestApplyPasswordChange_ClosesPendingInviteOnly(t *testing.T) {
	db := openInviteTestDB(t)
	invitee := seedPendingInvite(t, db, "apply@example.com", "tok-apply")
	require.NoError(t, db.Model(invitee).Update("enabled", true).Error)

	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		return ApplyPasswordChange(tx, invitee.ID, "new-hash")
	}))

	stored := reloadInviteUser(t, db, invitee.ID)
	assert.Equal(t, "new-hash", stored.PasswordHash)
	assert.Empty(t, stored.InviteToken)
	assert.Nil(t, stored.InviteExpires)
	assert.Empty(t, stored.InviteStatus)
	assert.True(t, stored.Enabled, "the password change must leave enabled as it was")
}

func TestApplyPasswordChange_LeavesOtherInviteStatusAlone(t *testing.T) {
	db := openInviteTestDB(t)
	accepted := seedPendingInvite(t, db, "accepted-apply@example.com", "tok-accepted-apply")
	require.NoError(t, db.Model(accepted).Update("invite_status", models.InviteStatusAccepted).Error)

	require.NoError(t, db.Transaction(func(tx *gorm.DB) error {
		return ApplyPasswordChange(tx, accepted.ID, "new-hash")
	}))

	stored := reloadInviteUser(t, db, accepted.ID)
	assert.Equal(t, models.InviteStatusAccepted, stored.InviteStatus)
	assert.Equal(t, "tok-accepted-apply", stored.InviteToken)
}

func TestInviteService_ExpiredMarkSkipsAccountWithPassword(t *testing.T) {
	db := openInviteTestDB(t)
	svc := newTestInviteService(db)
	invitee := seedPendingInvite(t, db, "haspw-expired@example.com", "tok-haspw")
	require.NoError(t, db.Model(invitee).Updates(map[string]any{
		"invite_expires": time.Now().Add(-time.Minute),
		"password_hash":  "admin-set-hash",
	}).Error)

	_, err := svc.Accept("tok-haspw", "Late", "late-password", "")
	assert.ErrorIs(t, err, ErrInviteExpired)
	assert.Equal(t, models.InviteStatusPending, reloadInviteUser(t, db, invitee.ID).InviteStatus,
		"an account that already has a password must not take the expired status")
}
