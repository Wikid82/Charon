package services

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/util"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

const (
	// TokenLength is the length of generated emergency tokens in bytes (64 bytes = 128 hex chars)
	TokenLength = 64

	// BcryptCost is the cost factor for bcrypt hashing (12+ for security)
	BcryptCost = 12

	// EmergencyTokenEnvVar is the environment variable name for backward compatibility
	EmergencyTokenEnvVar = "CHARON_EMERGENCY_TOKEN"

	// MinTokenLength is the minimum required length for emergency tokens
	MinTokenLength = 32
)

// EmergencyTokenService handles emergency token generation, validation, and expiration
type EmergencyTokenService struct {
	db *gorm.DB
}

// NewEmergencyTokenService creates a new EmergencyTokenService
func NewEmergencyTokenService(db *gorm.DB) *EmergencyTokenService {
	return &EmergencyTokenService{db: db}
}

// DB returns the database connection for use by handlers
func (s *EmergencyTokenService) DB() *gorm.DB {
	return s.db
}

// GenerateRequest represents a request to generate a new emergency token
type GenerateRequest struct {
	ExpirationDays int   // 0 = never, 30/60/90 = preset, 1-365 = custom
	UserID         *uint // User who generated the token (optional)
}

// GenerateResponse represents the response from generating a token
type GenerateResponse struct {
	Token            string     `json:"token"` // Plaintext token (shown ONCE)
	CreatedAt        time.Time  `json:"created_at"`
	ExpiresAt        *time.Time `json:"expires_at"`
	ExpirationPolicy string     `json:"expiration_policy"`
}

// StatusResponse represents the status of the emergency token
type StatusResponse struct {
	Configured          bool       `json:"configured"`
	CreatedAt           *time.Time `json:"created_at"`
	ExpiresAt           *time.Time `json:"expires_at"`
	ExpirationPolicy    string     `json:"expiration_policy"`
	DaysUntilExpiration int        `json:"days_until_expiration"` // -1 = never expires
	IsExpired           bool       `json:"is_expired"`
	LastUsedAt          *time.Time `json:"last_used_at"`
	UseCount            int        `json:"use_count"`
	Source              string     `json:"source"` // "database" or "environment"
}

// Generate creates a new emergency token with cryptographic randomness
func (s *EmergencyTokenService) Generate(req GenerateRequest) (*GenerateResponse, error) {
	// Generate cryptographically secure random token
	tokenBytes := make([]byte, TokenLength)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("failed to generate random token: %w", err)
	}
	token := hex.EncodeToString(tokenBytes)

	// Hash the token with bcrypt (bcrypt has 72-byte limit, so hash first with SHA-256)
	// This gives us cryptographic security with bcrypt's password hashing benefits
	tokenHash := sha256.Sum256([]byte(token))
	hash, err := bcrypt.GenerateFromPassword(tokenHash[:], BcryptCost)
	if err != nil {
		return nil, fmt.Errorf("failed to hash token: %w", err)
	}

	// Calculate expiration
	var expiresAt *time.Time
	policy := "never"
	if req.ExpirationDays > 0 {
		expiry := time.Now().Add(time.Duration(req.ExpirationDays) * 24 * time.Hour)
		expiresAt = &expiry
		switch req.ExpirationDays {
		case 30:
			policy = "30_days"
		case 60:
			policy = "60_days"
		case 90:
			policy = "90_days"
		default:
			policy = fmt.Sprintf("custom_%d_days", req.ExpirationDays)
		}
	}

	// Delete existing tokens (only one active token at a time)
	if err := s.db.Where("1=1").Delete(&models.EmergencyToken{}).Error; err != nil {
		logger.Log().WithError(err).Warn("Failed to delete existing emergency tokens")
	}

	// Create new token record
	tokenRecord := models.EmergencyToken{
		TokenHash:        string(hash),
		CreatedAt:        time.Now(),
		ExpiresAt:        expiresAt,
		ExpirationPolicy: policy,
		CreatedByUserID:  req.UserID,
		UseCount:         0,
	}

	if err := s.db.Create(&tokenRecord).Error; err != nil {
		return nil, fmt.Errorf("failed to save token: %w", err)
	}

	logger.Log().WithFields(map[string]interface{}{
		"policy":     util.SanitizeForLog(policy),
		"expires_at": expiresAt,
		"user_id":    req.UserID,
	}).Info("Emergency token generated")

	return &GenerateResponse{
		Token:            token,
		CreatedAt:        tokenRecord.CreatedAt,
		ExpiresAt:        tokenRecord.ExpiresAt,
		ExpirationPolicy: tokenRecord.ExpirationPolicy,
	}, nil
}

// Validate checks if the provided token is valid (matches hash and not expired)
// Returns the token record if valid, error otherwise
func (s *EmergencyTokenService) Validate(token string) (*models.EmergencyToken, error) {
	// Check for empty/whitespace token
	if token == "" || len(strings.TrimSpace(token)) == 0 {
		return nil, fmt.Errorf("token is empty")
	}

	envToken := os.Getenv(EmergencyTokenEnvVar)
	hasValidEnvToken := envToken != "" && len(strings.TrimSpace(envToken)) >= MinTokenLength

	// Try database token first (highest priority)
	var tokenRecord models.EmergencyToken
	err := s.db.First(&tokenRecord).Error
	if err == nil {
		// Found database token - validate hash
		tokenHash := sha256.Sum256([]byte(token))
		if bcrypt.CompareHashAndPassword([]byte(tokenRecord.TokenHash), tokenHash[:]) == nil {
			// Check expiration
			if tokenRecord.IsExpired() {
				return nil, fmt.Errorf("token expired")
			}

			// Update last used timestamp and use count
			now := time.Now()
			tokenRecord.LastUsedAt = &now
			tokenRecord.UseCount++
			if err := s.db.Save(&tokenRecord).Error; err != nil {
				logger.Log().WithError(err).Warn("Failed to update token usage statistics")
			}

			return &tokenRecord, nil
		}

		// If DB token doesn't match, allow explicit environment token as break-glass fallback.
		if hasValidEnvToken && envToken == token {
			logger.Log().Debug("Emergency token validated from environment variable while database token exists")
			return nil, nil
		}

		return nil, fmt.Errorf("invalid token")
	}

	// Fallback to environment variable for backward compatibility
	if envToken == "" || len(strings.TrimSpace(envToken)) == 0 {
		return nil, fmt.Errorf("no token configured")
	}

	if len(envToken) < MinTokenLength {
		return nil, fmt.Errorf("configured token too short")
	}

	// Simple string comparison for env var token (no bcrypt for legacy)
	if envToken != token {
		return nil, fmt.Errorf("invalid token")
	}

	// Environment token is valid (no expiration for env vars)
	logger.Log().Debug("Emergency token validated from environment variable (legacy mode)")
	return nil, nil // Return nil record to indicate env var source
}

// GetStatus returns the current emergency token status without exposing the token
func (s *EmergencyTokenService) GetStatus() (*StatusResponse, error) {
	// Check database token first
	var tokenRecord models.EmergencyToken
	err := s.db.First(&tokenRecord).Error
	if err == nil {
		// Found database token
		return &StatusResponse{
			Configured:          true,
			CreatedAt:           &tokenRecord.CreatedAt,
			ExpiresAt:           tokenRecord.ExpiresAt,
			ExpirationPolicy:    tokenRecord.ExpirationPolicy,
			DaysUntilExpiration: tokenRecord.DaysUntilExpiration(),
			IsExpired:           tokenRecord.IsExpired(),
			LastUsedAt:          tokenRecord.LastUsedAt,
			UseCount:            tokenRecord.UseCount,
			Source:              "database",
		}, nil
	}

	// Check environment variable for backward compatibility
	envToken := os.Getenv(EmergencyTokenEnvVar)
	if envToken != "" && len(strings.TrimSpace(envToken)) >= MinTokenLength {
		// Environment token is configured
		return &StatusResponse{
			Configured:          true,
			CreatedAt:           nil,
			ExpiresAt:           nil,
			ExpirationPolicy:    "never",
			DaysUntilExpiration: -1,
			IsExpired:           false,
			LastUsedAt:          nil,
			UseCount:            0,
			Source:              "environment",
		}, nil
	}

	// No token configured
	return &StatusResponse{
		Configured:          false,
		CreatedAt:           nil,
		ExpiresAt:           nil,
		ExpirationPolicy:    "",
		DaysUntilExpiration: 0,
		IsExpired:           false,
		LastUsedAt:          nil,
		UseCount:            0,
		Source:              "none",
	}, nil
}

// Revoke deletes the current emergency token
func (s *EmergencyTokenService) Revoke() error {
	result := s.db.Where("1=1").Delete(&models.EmergencyToken{})
	if result.Error != nil {
		return fmt.Errorf("failed to revoke token: %w", result.Error)
	}

	if result.RowsAffected == 0 {
		return fmt.Errorf("no token to revoke")
	}

	logger.Log().Info("Emergency token revoked")
	return nil
}

// UpdateExpiration changes the expiration policy for the current token
func (s *EmergencyTokenService) UpdateExpiration(expirationDays int) (*time.Time, error) {
	var tokenRecord models.EmergencyToken
	if err := s.db.First(&tokenRecord).Error; err != nil {
		return nil, fmt.Errorf("no token found to update")
	}

	// Calculate new expiration
	var expiresAt *time.Time
	policy := "never"
	if expirationDays > 0 {
		expiry := time.Now().Add(time.Duration(expirationDays) * 24 * time.Hour)
		expiresAt = &expiry
		switch expirationDays {
		case 30:
			policy = "30_days"
		case 60:
			policy = "60_days"
		case 90:
			policy = "90_days"
		default:
			policy = fmt.Sprintf("custom_%d_days", expirationDays)
		}
	}

	// Update token
	tokenRecord.ExpiresAt = expiresAt
	tokenRecord.ExpirationPolicy = policy

	if err := s.db.Save(&tokenRecord).Error; err != nil {
		return nil, fmt.Errorf("failed to update expiration: %w", err)
	}

	logger.Log().WithFields(map[string]interface{}{
		"policy":     util.SanitizeForLog(policy),
		"expires_at": expiresAt,
	}).Info("Emergency token expiration updated")

	return expiresAt, nil
}
