package services

import (
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type AuthService struct {
	db     *gorm.DB
	config config.Config
}

func NewAuthService(db *gorm.DB, cfg config.Config) *AuthService {
	return &AuthService{db: db, config: cfg}
}

type Claims struct {
	UserID         uint   `json:"user_id"`
	Role           string `json:"role"`
	SessionVersion uint   `json:"session_version"`
	jwt.RegisteredClaims
}

// Sign-in policy constants.
const (
	// MaxFailedLoginAttempts is the number of consecutive failed sign-ins that locks an account.
	MaxFailedLoginAttempts = 5
	// LockDuration is how long an account stays locked once the threshold is reached.
	LockDuration = 15 * time.Minute
)

var (
	// ErrInvalidLogin is returned for every sign-in failure cause so callers
	// present one uniform message.
	ErrInvalidLogin = errors.New("invalid credentials")
	// ErrLoginUnavailable is returned when sign-in cannot be evaluated because of an internal error.
	ErrLoginUnavailable = errors.New("login unavailable")
)

var (
	dummyHashOnce sync.Once
	dummyHash     []byte
)

// placeholderHash returns a bcrypt hash used to keep password-check cost uniform
// when no stored hash applies. It is generated at bcrypt.DefaultCost, the same
// cost models.User.SetPassword uses; if stored hashes ever move to another cost,
// this must follow.
func placeholderHash() []byte {
	dummyHashOnce.Do(func() {
		h, err := bcrypt.GenerateFromPassword([]byte(uuid.New().String()), bcrypt.DefaultCost)
		if err != nil {
			panic(fmt.Errorf("generate placeholder hash: %w", err))
		}
		dummyHash = h
	})
	return dummyHash
}

// failedLoginState is the row state returned by the atomic failure update.
type failedLoginState struct {
	FailedLoginAttempts int
	LockedUntil         *time.Time
}

// recordFailedLogin atomically records one failed attempt for an enabled,
// currently-unlocked account. The counter restarts at 1 once a previous lock has
// expired, and the lock is set in the same statement when the threshold is reached.
// Times are compared with julianday() so stored values in any offset or precision
// compare correctly. It returns the number of rows affected (0 when the account
// was locked or disabled in the meantime).
func (s *AuthService) recordFailedLogin(userID uint, now time.Time) (int64, error) {
	now = now.UTC()
	lockUntil := now.Add(LockDuration)
	const q = `UPDATE users SET
		failed_login_attempts = CASE WHEN locked_until IS NOT NULL AND julianday(locked_until) <= julianday(?) THEN 1 ELSE failed_login_attempts + 1 END,
		locked_until = CASE WHEN (CASE WHEN locked_until IS NOT NULL AND julianday(locked_until) <= julianday(?) THEN 1 ELSE failed_login_attempts + 1 END) >= ? THEN ? ELSE NULL END
		WHERE id = ? AND enabled = ? AND (locked_until IS NULL OR julianday(locked_until) <= julianday(?))
		RETURNING failed_login_attempts, locked_until`
	var state failedLoginState
	res := s.db.Raw(q, now, now, MaxFailedLoginAttempts, lockUntil, userID, true, now).Scan(&state)
	if res.Error != nil {
		return 0, fmt.Errorf("record failed login: %w", res.Error)
	}
	return res.RowsAffected, nil
}

// checkPasswordUniformCost verifies the password against the stored hash. Accounts
// without a stored hash (for example, invitations not yet accepted) are compared
// against the placeholder hash instead and never match, so each call performs one
// full-cost comparison.
func checkPasswordUniformCost(user *models.User, password string) bool {
	if user.PasswordHash == "" {
		_ = bcrypt.CompareHashAndPassword(placeholderHash(), []byte(password))
		return false
	}
	return user.CheckPassword(password)
}

// Login verifies credentials and returns a session token. All sign-in failures
// returns ErrInvalidLogin and performs exactly one bcrypt comparison.
func (s *AuthService) Login(email, password string) (string, error) {
	email = strings.ToLower(email)
	var user models.User
	if err := s.db.Where("email = ?", email).First(&user).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			_ = bcrypt.CompareHashAndPassword(placeholderHash(), []byte(password))
			return "", ErrInvalidLogin
		}
		logger.Log().WithError(err).Error("login: user lookup failed")
		return "", ErrLoginUnavailable
	}

	passwordOK := checkPasswordUniformCost(&user, password)
	now := time.Now().UTC()

	if !user.Enabled || (user.LockedUntil != nil && user.LockedUntil.After(now)) {
		return "", ErrInvalidLogin
	}

	if !passwordOK {
		if _, err := s.recordFailedLogin(user.ID, now); err != nil {
			logger.Log().WithError(err).Error("login: failed to record attempt")
			return "", ErrLoginUnavailable
		}
		// Zero rows means the account was locked or disabled concurrently.
		return "", ErrInvalidLogin
	}

	res := s.db.Model(&models.User{}).
		Where("id = ? AND enabled = ?", user.ID, true).
		Updates(map[string]any{
			"failed_login_attempts": 0,
			"locked_until":          nil,
			"last_login":            now,
		})
	if res.Error != nil {
		logger.Log().WithError(res.Error).Error("login: failed to record success")
		return "", ErrLoginUnavailable
	}
	if res.RowsAffected == 0 {
		return "", ErrInvalidLogin
	}

	return s.GenerateToken(&user)
}

func (s *AuthService) GenerateToken(user *models.User) (string, error) {
	expirationTime := time.Now().Add(24 * time.Hour)
	claims := &Claims{
		UserID:         user.ID,
		Role:           string(user.Role),
		SessionVersion: user.SessionVersion,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			Issuer:    "charon",
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(s.config.JWTSecret))
}

// ChangePassword verifies the current password and stores the new one. In the same
// transaction it advances the session version (ending other sessions) and clears
// failed-attempt and lock state. Callers that keep the user signed in must issue a
// fresh token afterwards.
func (s *AuthService) ChangePassword(userID uint, oldPassword, newPassword string) error {
	var user models.User
	if err := s.db.Where("id = ?", userID).First(&user).Error; err != nil {
		return errors.New("user not found")
	}

	if !user.CheckPassword(oldPassword) {
		return errors.New("invalid current password")
	}

	if err := user.SetPassword(newPassword); err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	if err := s.db.Transaction(func(tx *gorm.DB) error {
		return applyPasswordChange(tx, userID, user.PasswordHash)
	}); err != nil {
		return fmt.Errorf("change password: %w", err)
	}
	return nil
}

// applyPasswordChange stores a new password hash for the user and, in the same
// statement, advances the session version and clears failed-attempt and lock state.
// Every password change (self-service or administrative) goes through it.
func applyPasswordChange(tx *gorm.DB, userID uint, passwordHash string) error {
	res := tx.Model(&models.User{}).Where("id = ?", userID).Updates(map[string]any{
		"password_hash":         passwordHash,
		"session_version":       gorm.Expr("session_version + 1"),
		"failed_login_attempts": 0,
		"locked_until":          nil,
	})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return errors.New("user not found")
	}
	return nil
}

func (s *AuthService) ValidateToken(tokenString string) (*Claims, error) {
	claims := &Claims{}
	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(s.config.JWTSecret), nil
	})

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, errors.New("invalid token")
	}

	return claims, nil
}

func (s *AuthService) AuthenticateToken(tokenString string) (*models.User, *Claims, error) {
	claims, err := s.ValidateToken(tokenString)
	if err != nil {
		return nil, nil, err
	}

	user, err := s.GetUserByID(claims.UserID)
	if err != nil || !user.Enabled {
		return nil, nil, errors.New("invalid token")
	}

	if claims.SessionVersion != user.SessionVersion {
		return nil, nil, errors.New("invalid token")
	}

	return user, claims, nil
}

func (s *AuthService) InvalidateSessions(userID uint) error {
	result := s.db.Model(&models.User{}).
		Where("id = ?", userID).
		Update("session_version", gorm.Expr("session_version + 1"))
	if result.Error != nil {
		return result.Error
	}

	if result.RowsAffected == 0 {
		return errors.New("user not found")
	}

	return nil
}

func (s *AuthService) GetUserByID(id uint) (*models.User, error) {
	var user models.User
	if err := s.db.Where("id = ?", id).First(&user).Error; err != nil {
		return nil, err
	}
	return &user, nil
}

// TokenForUser issues a session token reflecting the user's current session version.
func (s *AuthService) TokenForUser(userID uint) (string, error) {
	user, err := s.GetUserByID(userID)
	if err != nil {
		return "", fmt.Errorf("load user: %w", err)
	}
	return s.GenerateToken(user)
}
