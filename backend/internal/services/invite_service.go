package services

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"github.com/sirupsen/logrus"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

// InviteTTL is how long an invite link stays usable after it is issued or resent.
const InviteTTL = 48 * time.Hour

var (
	// ErrInviteNotFound means no user holds the token.
	ErrInviteNotFound = errors.New("invite not found")
	// ErrInviteExpired means the token is known but its expiry is unset or has passed.
	ErrInviteExpired = errors.New("invite expired")
	// ErrInviteClosed means the invite was already accepted or no longer applies.
	ErrInviteClosed = errors.New("invite is no longer open")
	// ErrInvitePasswordHash wraps a hashing failure while accepting an invite.
	ErrInvitePasswordHash = errors.New("hash invite password")
)

// InviteService owns invite token lookup, acceptance and expiry handling.
type InviteService struct {
	db  *gorm.DB
	now func() time.Time
	// beforeLock is a test seam, nil in production. It runs immediately before the write lock is taken:
	// in Accept after the pre-check and the password hash, in Reissue after the token is generated.
	beforeLock func()
	// hashCost is the bcrypt cost for accepted passwords.
	hashCost int
}

// NewInviteService returns an InviteService backed by db.
func NewInviteService(db *gorm.DB) *InviteService {
	return &InviteService{db: db, now: time.Now, hashCost: bcrypt.DefaultCost}
}

// GenerateInviteToken returns a new invite token: 32 random bytes, hex encoded.
func GenerateInviteToken() (string, error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("read random invite token: %w", err)
	}
	return hex.EncodeToString(buf), nil
}

// Lookup finds the invited user for token and classifies its state. It is read-only.
// For ErrInviteExpired and ErrInviteClosed the user is returned as well.
func (s *InviteService) Lookup(token string) (*models.User, error) {
	// An empty token would match every account without an invite, so it is refused up front.
	if token == "" {
		return nil, ErrInviteNotFound
	}

	var user models.User
	if err := s.db.Where("invite_token = ?", token).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInviteNotFound
		}
		return nil, fmt.Errorf("lookup invite: %w", err)
	}
	return &user, inviteStateError(user.InviteStateAt(s.now()))
}

// Accept consumes token once: it sets the name and password hash, enables the account and
// clears the invite. Exactly one concurrent call per token succeeds.
//
// Order: read-only pre-check, bcrypt hash, then the write lock with a re-read and a
// conditional update. Nothing slow runs inside the lock.
func (s *InviteService) Accept(token, name, password, lastSeenVersion string) (*models.User, error) {
	user, err := s.Lookup(token)
	switch {
	case errors.Is(err, ErrInviteExpired):
		s.markExpired(user.ID, token)
		return nil, err
	case err != nil:
		return nil, err
	}

	hash, err := models.HashPasswordCost(password, s.hashCost)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvitePasswordHash, err)
	}

	if s.beforeLock != nil {
		s.beforeLock()
	}

	var accepted models.User
	err = WithWriteLock(s.db, &models.User{}, func(tx *gorm.DB) error {
		var row models.User
		if readErr := tx.Where("invite_token = ?", token).First(&row).Error; readErr != nil {
			if errors.Is(readErr, gorm.ErrRecordNotFound) {
				return ErrInviteNotFound
			}
			return fmt.Errorf("reread invite: %w", readErr)
		}
		if stateErr := inviteStateError(row.InviteStateAt(s.now())); stateErr != nil {
			return stateErr
		}
		if consumeErr := consumeInvite(tx, &row, name, hash, lastSeenVersion); consumeErr != nil {
			return consumeErr
		}
		if reloadErr := tx.First(&accepted, row.ID).Error; reloadErr != nil {
			return fmt.Errorf("reload accepted invite: %w", reloadErr)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &accepted, nil
}

// Reissue replaces the token and expiry of a pending invite whose account has no password yet.
// Any expiry qualifies, including one that has already passed, so a lapsed link can be renewed.
// It returns gorm.ErrRecordNotFound (wrapped) for an unknown user and ErrInviteClosed when the invite
// is no longer pending or the account has a password.
func (s *InviteService) Reissue(userID uint) (*models.User, string, time.Time, error) {
	// The token is generated before the lock: random reads do not belong in the write lock.
	token, err := GenerateInviteToken()
	if err != nil {
		return nil, "", time.Time{}, fmt.Errorf("issue invite token: %w", err)
	}
	expires := s.now().Add(InviteTTL)

	if s.beforeLock != nil {
		s.beforeLock()
	}

	var user models.User
	err = WithWriteLock(s.db, &models.User{}, func(tx *gorm.DB) error {
		if readErr := tx.First(&user, userID).Error; readErr != nil {
			return fmt.Errorf("read invite user: %w", readErr)
		}
		if user.InviteStatus != models.InviteStatusPending || user.PasswordHash != "" {
			return ErrInviteClosed
		}
		res := tx.Model(&models.User{}).
			Where("id = ? AND invite_status = ? AND COALESCE(password_hash, '') = ''", userID, models.InviteStatusPending).
			Updates(map[string]any{
				"invite_token":   token,
				"invite_expires": expires,
			})
		if res.Error != nil {
			return fmt.Errorf("store invite token: %w", res.Error)
		}
		if res.RowsAffected != 1 {
			return ErrInviteClosed
		}
		if reloadErr := tx.First(&user, userID).Error; reloadErr != nil {
			return fmt.Errorf("reload invite user: %w", reloadErr)
		}
		return nil
	})
	if err != nil {
		return nil, "", time.Time{}, err
	}
	return &user, token, expires, nil
}

// consumeInvite writes the acceptance for row with a conditional update. It matches only while
// the stored row still has the token, is pending and has no password, and returns ErrInviteClosed
// when nothing matched. Callers must hold the write lock through tx.
func consumeInvite(tx *gorm.DB, row *models.User, name, passwordHash, lastSeenVersion string) error {
	res := tx.Model(&models.User{}).
		Where("id = ? AND invite_token = ? AND invite_status = ? AND COALESCE(password_hash, '') = ''",
			row.ID, row.InviteToken, models.InviteStatusPending).
		Updates(map[string]any{
			"name":              name,
			"password_hash":     passwordHash,
			"enabled":           true,
			"invite_token":      "",
			"invite_expires":    nil,
			"invite_status":     models.InviteStatusAccepted,
			"last_seen_version": lastSeenVersion,
		})
	if res.Error != nil {
		return fmt.Errorf("accept invite: %w", res.Error)
	}
	if res.RowsAffected != 1 {
		return ErrInviteClosed
	}
	return nil
}

// markExpired records an expired invite. The update is conditional on the token, so a resend that
// replaced the token in the meantime is left alone, and on the account having no password, so an
// account that already has one never takes the expired status. A failure is logged; the caller still answers 410.
func (s *InviteService) markExpired(id uint, token string) {
	res := s.db.Model(&models.User{}).
		Where("id = ? AND invite_token = ? AND invite_status = ? AND COALESCE(password_hash, '') = ''",
			id, token, models.InviteStatusPending).
		Update("invite_status", models.InviteStatusExpired)
	if res.Error != nil {
		logrus.WithError(res.Error).WithField("user_id", id).Warn("failed to mark invite expired")
	}
}

// inviteStateError maps an invite state to the sentinel error for it (nil when open).
func inviteStateError(state models.InviteState) error {
	switch state {
	case models.InviteOpen:
		return nil
	case models.InviteExpired:
		return ErrInviteExpired
	case models.InviteClosed:
		return ErrInviteClosed
	default:
		return ErrInviteNotFound
	}
}
