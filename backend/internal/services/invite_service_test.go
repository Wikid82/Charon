package services

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

// openInviteTestDB opens a file-backed WAL database with a busy timeout, so concurrent
// writers wait for each other the way they do in production.
func openInviteTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s/invite.db?_pragma=busy_timeout(10000)&_pragma=journal_mode(WAL)", t.TempDir())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	t.Cleanup(func() {
		if sqlDB, dbErr := db.DB(); dbErr == nil {
			_ = sqlDB.Close()
		}
	})
	require.NoError(t, db.AutoMigrate(&models.User{}))
	return db
}

// seedPendingInvite stores a pending, password-less invitee with token that expires in one hour.
func seedPendingInvite(t *testing.T, db *gorm.DB, email, token string) *models.User {
	t.Helper()
	expires := time.Now().Add(time.Hour)
	user := models.User{
		UUID:          uuid.New().String(),
		APIKey:        uuid.New().String(),
		Email:         email,
		Role:          models.RoleUser,
		InviteToken:   token,
		InviteExpires: &expires,
		InviteStatus:  models.InviteStatusPending,
	}
	require.NoError(t, CreateKeepingSent(db, &user, []string{"enabled"}))
	return &user
}

// newTestInviteService returns a service with the minimum bcrypt cost, so tests stay fast.
func newTestInviteService(db *gorm.DB) *InviteService {
	svc := NewInviteService(db)
	svc.hashCost = bcrypt.MinCost
	return svc
}

func reloadInviteUser(t *testing.T, db *gorm.DB, id uint) models.User {
	t.Helper()
	var user models.User
	require.NoError(t, db.First(&user, id).Error)
	return user
}

func TestInviteService_LookupStates(t *testing.T) {
	db := openInviteTestDB(t)
	svc := newTestInviteService(db)

	pending := seedPendingInvite(t, db, "pending@example.com", "tok-pending")

	past := time.Now().Add(-time.Minute)
	expired := seedPendingInvite(t, db, "expired@example.com", "tok-expired")
	require.NoError(t, db.Model(expired).Update("invite_expires", past).Error)

	accepted := seedPendingInvite(t, db, "accepted@example.com", "tok-accepted")
	require.NoError(t, db.Model(accepted).Update("invite_status", models.InviteStatusAccepted).Error)

	t.Run("unknown token is not found", func(t *testing.T) {
		user, err := svc.Lookup("no-such-token")
		assert.ErrorIs(t, err, ErrInviteNotFound)
		assert.Nil(t, user)
	})

	t.Run("open invite is returned", func(t *testing.T) {
		user, err := svc.Lookup("tok-pending")
		require.NoError(t, err)
		assert.Equal(t, pending.ID, user.ID)
		assert.Equal(t, "pending@example.com", user.Email)
	})

	t.Run("lapsed invite is expired and still returned", func(t *testing.T) {
		user, err := svc.Lookup("tok-expired")
		assert.ErrorIs(t, err, ErrInviteExpired)
		require.NotNil(t, user)
		assert.Equal(t, expired.ID, user.ID)
	})

	t.Run("accepted invite is closed and still returned", func(t *testing.T) {
		user, err := svc.Lookup("tok-accepted")
		assert.ErrorIs(t, err, ErrInviteClosed)
		require.NotNil(t, user)
		assert.Equal(t, accepted.ID, user.ID)
	})
}

func TestInviteService_AcceptSetsAcceptedFields(t *testing.T) {
	db := openInviteTestDB(t)
	svc := newTestInviteService(db)
	invitee := seedPendingInvite(t, db, "invitee@example.com", "tok-accept")

	user, err := svc.Accept("tok-accept", "Invitee Name", "correct-horse", "1.2.3")
	require.NoError(t, err)
	assert.Equal(t, "Invitee Name", user.Name)
	assert.Equal(t, "invitee@example.com", user.Email)

	stored := reloadInviteUser(t, db, invitee.ID)
	assert.Equal(t, "Invitee Name", stored.Name)
	assert.True(t, stored.Enabled)
	assert.Empty(t, stored.InviteToken)
	assert.Nil(t, stored.InviteExpires)
	assert.Equal(t, models.InviteStatusAccepted, stored.InviteStatus)
	assert.Equal(t, "1.2.3", stored.LastSeenVersion)
	assert.True(t, (&models.User{PasswordHash: stored.PasswordHash}).CheckPassword("correct-horse"))
}

func TestInviteService_AcceptReplayIsNotFound(t *testing.T) {
	db := openInviteTestDB(t)
	svc := newTestInviteService(db)
	seedPendingInvite(t, db, "replay@example.com", "tok-replay")

	_, err := svc.Accept("tok-replay", "First", "first-password", "")
	require.NoError(t, err)

	_, err = svc.Accept("tok-replay", "Second", "second-password", "")
	assert.ErrorIs(t, err, ErrInviteNotFound)
}

func TestInviteService_AcceptExpiredMarksInvite(t *testing.T) {
	db := openInviteTestDB(t)
	svc := newTestInviteService(db)
	invitee := seedPendingInvite(t, db, "late@example.com", "tok-late")
	require.NoError(t, db.Model(invitee).Update("invite_expires", time.Now().Add(-time.Minute)).Error)

	_, err := svc.Accept("tok-late", "Late", "late-password", "")
	assert.ErrorIs(t, err, ErrInviteExpired)

	stored := reloadInviteUser(t, db, invitee.ID)
	assert.Equal(t, models.InviteStatusExpired, stored.InviteStatus)
	assert.Equal(t, "tok-late", stored.InviteToken)
	assert.Empty(t, stored.PasswordHash)
}

func TestInviteService_AcceptClosedInviteLeavesRowUnchanged(t *testing.T) {
	db := openInviteTestDB(t)
	svc := newTestInviteService(db)
	invitee := seedPendingInvite(t, db, "closed@example.com", "tok-closed")
	require.NoError(t, db.Model(invitee).Update("invite_status", models.InviteStatusAccepted).Error)

	_, err := svc.Accept("tok-closed", "Closed", "closed-password", "")
	assert.ErrorIs(t, err, ErrInviteClosed)
	assert.Empty(t, reloadInviteUser(t, db, invitee.ID).PasswordHash)
}

func TestInviteService_AcceptHashFailureKeepsInvite(t *testing.T) {
	db := openInviteTestDB(t)
	svc := newTestInviteService(db)
	invitee := seedPendingInvite(t, db, "long@example.com", "tok-long")

	// bcrypt refuses passwords longer than 72 bytes.
	_, err := svc.Accept("tok-long", "Long", strings.Repeat("x", 73), "")
	assert.ErrorIs(t, err, ErrInvitePasswordHash)

	stored := reloadInviteUser(t, db, invitee.ID)
	assert.Equal(t, "tok-long", stored.InviteToken)
	assert.Equal(t, models.InviteStatusPending, stored.InviteStatus)
}

func TestInviteService_AcceptReportsWriteFailure(t *testing.T) {
	db := openInviteTestDB(t)
	svc := newTestInviteService(db)
	seedPendingInvite(t, db, "writefail@example.com", "tok-writefail")

	// Dropping the table in the seam makes the write fail after the checks passed.
	svc.beforeLock = func() {
		require.NoError(t, db.Migrator().DropTable(&models.User{}))
	}

	_, err := svc.Accept("tok-writefail", "Write", "write-password", "")
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrInvitePasswordHash)
	assert.NotErrorIs(t, err, ErrInviteNotFound)
}

func TestInviteService_LookupReportsStorageFailure(t *testing.T) {
	db := openInviteTestDB(t)
	svc := newTestInviteService(db)
	require.NoError(t, db.Migrator().DropTable(&models.User{}))

	user, err := svc.Lookup("any-token")
	require.Error(t, err)
	assert.NotErrorIs(t, err, ErrInviteNotFound)
	assert.Nil(t, user)
}

func TestInviteService_LookupEmptyTokenIsNotFound(t *testing.T) {
	db := openInviteTestDB(t)
	svc := newTestInviteService(db)
	// An account with no invite has an empty token; it must never match an empty lookup.
	require.NoError(t, CreateKeepingSent(db, &models.User{UUID: uuid.New().String(), APIKey: uuid.New().String(), Email: "plain@example.com"}, []string{"enabled"}))

	user, err := svc.Lookup("")
	assert.ErrorIs(t, err, ErrInviteNotFound)
	assert.Nil(t, user)
}

func TestGenerateInviteToken(t *testing.T) {
	first, err := GenerateInviteToken()
	require.NoError(t, err)
	second, err := GenerateInviteToken()
	require.NoError(t, err)

	assert.Len(t, first, 64)
	assert.Regexp(t, "^[0-9a-f]{64}$", first)
	assert.NotEqual(t, first, second)
}
