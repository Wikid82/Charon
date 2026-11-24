package services

import (
	"errors"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

func TestInviteService_ReissueRenewsLapsedPendingInvite(t *testing.T) {
	db := openInviteTestDB(t)
	svc := newTestInviteService(db)
	invitee := seedPendingInvite(t, db, "lapsed-resend@example.com", "tok-old")
	require.NoError(t, db.Model(invitee).Update("invite_expires", time.Now().Add(-24*time.Hour)).Error)

	user, token, expires, err := svc.Reissue(invitee.ID)
	require.NoError(t, err)
	assert.Equal(t, invitee.ID, user.ID)
	assert.Equal(t, "lapsed-resend@example.com", user.Email)
	assert.Len(t, token, 64)
	assert.NotEqual(t, "tok-old", token)
	assert.WithinDuration(t, time.Now().Add(InviteTTL), expires, time.Minute)

	stored := reloadInviteUser(t, db, invitee.ID)
	assert.Equal(t, token, stored.InviteToken)
	assert.Equal(t, models.InviteStatusPending, stored.InviteStatus)
	require.NotNil(t, stored.InviteExpires)
	assert.True(t, stored.InviteExpires.After(time.Now()))
}

func TestInviteService_ReissueAcceptsUnsetExpiry(t *testing.T) {
	db := openInviteTestDB(t)
	svc := newTestInviteService(db)
	invitee := seedPendingInvite(t, db, "unset-resend@example.com", "tok-unset")
	require.NoError(t, db.Model(invitee).Update("invite_expires", nil).Error)

	_, token, _, err := svc.Reissue(invitee.ID)
	require.NoError(t, err)
	assert.Equal(t, token, reloadInviteUser(t, db, invitee.ID).InviteToken)
}

func TestInviteService_ReissueRefusesClosedInvites(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(t *testing.T, db *gorm.DB, id uint)
	}{
		{
			name: "account already has a password",
			mutate: func(t *testing.T, db *gorm.DB, id uint) {
				require.NoError(t, db.Model(&models.User{}).Where("id = ?", id).Update("password_hash", "hash").Error)
			},
		},
		{
			name: "invite accepted",
			mutate: func(t *testing.T, db *gorm.DB, id uint) {
				require.NoError(t, db.Model(&models.User{}).Where("id = ?", id).Update("invite_status", models.InviteStatusAccepted).Error)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db := openInviteTestDB(t)
			svc := newTestInviteService(db)
			invitee := seedPendingInvite(t, db, "closed-resend@example.com", "tok-closed-resend")
			tt.mutate(t, db, invitee.ID)
			before := reloadInviteUser(t, db, invitee.ID)

			_, _, _, err := svc.Reissue(invitee.ID)
			assert.ErrorIs(t, err, ErrInviteClosed)

			after := reloadInviteUser(t, db, invitee.ID)
			assert.Equal(t, before.InviteToken, after.InviteToken)
			assert.Equal(t, before.InviteStatus, after.InviteStatus)
		})
	}
}

func TestInviteService_ReissueUnknownUserIsNotFound(t *testing.T) {
	db := openInviteTestDB(t)
	svc := newTestInviteService(db)

	_, _, _, err := svc.Reissue(9999)
	assert.True(t, errors.Is(err, gorm.ErrRecordNotFound), "expected record not found, got %v", err)
}

func TestInviteService_ReissueLosesToAcceptThatWinsFirst(t *testing.T) {
	db := openInviteTestDB(t)
	svc := newTestInviteService(db)
	invitee := seedPendingInvite(t, db, "race-resend@example.com", "tok-race-resend")

	// The invitee accepts after the resend has generated its token, just before the resend takes the lock.
	svc.beforeLock = func() {
		svc.beforeLock = nil
		_, err := svc.Accept("tok-race-resend", "Racer", "racer-password", "")
		require.NoError(t, err)
	}

	_, _, _, err := svc.Reissue(invitee.ID)
	assert.ErrorIs(t, err, ErrInviteClosed)

	stored := reloadInviteUser(t, db, invitee.ID)
	assert.Equal(t, models.InviteStatusAccepted, stored.InviteStatus)
	assert.Empty(t, stored.InviteToken, "a resend must never write a token onto an accepted account")
	assert.NotEmpty(t, stored.PasswordHash)
}

func TestInviteService_ReissueReportsWriteFailure(t *testing.T) {
	db := openInviteTestDB(t)
	svc := newTestInviteService(db)
	invitee := seedPendingInvite(t, db, "resend-fail@example.com", "tok-resend-fail")

	svc.beforeLock = func() { installWriteBlocker(t, db) }

	_, _, _, err := svc.Reissue(invitee.ID)
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrInviteClosed)
	assert.Equal(t, "tok-resend-fail", reloadInviteUser(t, db, invitee.ID).InviteToken)
}
