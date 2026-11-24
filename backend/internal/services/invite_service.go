package services

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

// InviteTTL is how long an invite link stays usable after it is issued or resent.
const InviteTTL = 48 * time.Hour

var (
	// ErrInviteNotFound means no user holds the token (or the lookup itself failed).
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
	// beforeLock is a test seam, nil in production. It runs after the pre-checks and
	// the password hash, immediately before the write.
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
	var user models.User
	if err := s.db.Where("invite_token = ?", token).First(&user).Error; err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInviteNotFound, err)
	}
	if user.InviteExpires != nil && user.InviteExpires.Before(s.now()) {
		return &user, ErrInviteExpired
	}
	if user.InviteStatus != models.InviteStatusPending {
		return &user, ErrInviteClosed
	}
	return &user, nil
}

// Accept sets the invited user's name and password and clears the invite.
// Commit 1 keeps the non-atomic sequence of the original handler; commit 2 moves the
// check and the write under the write lock.
func (s *InviteService) Accept(token, name, password, lastSeenVersion string) (*models.User, error) {
	user, err := s.Lookup(token)
	if errors.Is(err, ErrInviteExpired) {
		// The mark is best effort, as it was in the handler.
		_ = s.db.Model(user).Update("invite_status", models.InviteStatusExpired).Error
	}
	if err != nil {
		return nil, err
	}

	hash, err := models.HashPasswordCost(password, s.hashCost)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvitePasswordHash, err)
	}

	if s.beforeLock != nil {
		s.beforeLock()
	}

	if err := s.db.Model(user).Updates(map[string]any{
		"name":              name,
		"password_hash":     hash,
		"enabled":           true,
		"invite_token":      "",
		"invite_expires":    nil,
		"invite_status":     models.InviteStatusAccepted,
		"last_seen_version": lastSeenVersion,
	}).Error; err != nil {
		return nil, fmt.Errorf("accept invite: %w", err)
	}

	user.Name = name
	user.PasswordHash = hash
	user.Enabled = true
	user.InviteToken = ""
	user.InviteExpires = nil
	user.InviteStatus = models.InviteStatusAccepted
	user.LastSeenVersion = lastSeenVersion
	return user, nil
}
