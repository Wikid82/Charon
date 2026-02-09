package services

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/pkg/dnsprovider/custom"
	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Manual challenge error codes.
var (
	ErrChallengeNotFound   = errors.New("challenge not found")
	ErrChallengeExpired    = errors.New("challenge has expired")
	ErrChallengeInProgress = errors.New("another challenge is in progress for this FQDN")
	ErrUnauthorized        = errors.New("unauthorized access to challenge")
	ErrDNSNotPropagated    = errors.New("DNS record not yet propagated")
)

// ManualChallengeService manages the lifecycle of manual DNS challenges.
type ManualChallengeService struct {
	db   *gorm.DB
	cron *cron.Cron

	// Mutex for concurrent verification attempts
	verifyMu sync.Mutex

	// DNS resolver for verification (can be overridden for testing)
	resolver DNSResolver
}

// DNSResolver interface for DNS lookups (allows mocking in tests).
type DNSResolver interface {
	LookupTXT(ctx context.Context, name string) ([]string, error)
}

// DefaultDNSResolver uses net.Resolver for DNS lookups.
type DefaultDNSResolver struct {
	resolver *net.Resolver
}

// NewDefaultDNSResolver creates a DNS resolver that queries authoritative nameservers.
func NewDefaultDNSResolver() *DefaultDNSResolver {
	return &DefaultDNSResolver{
		resolver: &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				d := net.Dialer{
					Timeout: 10 * time.Second,
				}
				return d.DialContext(ctx, network, address)
			},
		},
	}
}

// LookupTXT performs a TXT record lookup.
func (r *DefaultDNSResolver) LookupTXT(ctx context.Context, name string) ([]string, error) {
	return r.resolver.LookupTXT(ctx, name)
}

// NewManualChallengeService creates a new manual challenge service.
func NewManualChallengeService(db *gorm.DB) *ManualChallengeService {
	s := &ManualChallengeService{
		db:       db,
		cron:     cron.New(),
		resolver: NewDefaultDNSResolver(),
	}

	// Schedule cleanup job to run every hour
	_, err := s.cron.AddFunc("0 * * * *", s.cleanupExpiredChallenges)
	if err != nil {
		logger.Log().WithError(err).Error("Failed to schedule manual challenge cleanup job")
	}

	return s
}

// Start starts the cron scheduler for cleanup jobs.
func (s *ManualChallengeService) Start() {
	s.cron.Start()
	logger.Log().Info("Manual challenge service cleanup scheduler started")
}

// Stop gracefully shuts down the cron scheduler.
func (s *ManualChallengeService) Stop() {
	ctx := s.cron.Stop()
	<-ctx.Done()
	logger.Log().Info("Manual challenge service cleanup scheduler stopped")
}

// SetResolver allows setting a custom DNS resolver (for testing).
func (s *ManualChallengeService) SetResolver(resolver DNSResolver) {
	s.resolver = resolver
}

// CreateChallengeRequest represents a request to create a manual challenge.
type CreateChallengeRequest struct {
	ProviderID uint
	UserID     uint
	FQDN       string
	Token      string
	Value      string
}

// CreateChallenge creates a new manual DNS challenge.
// Implements database locking to prevent concurrent challenges for the same FQDN.
func (s *ManualChallengeService) CreateChallenge(ctx context.Context, req CreateChallengeRequest) (*models.ManualChallenge, error) {
	// Generate cryptographically random challenge ID (UUIDv4)
	challengeID := uuid.New().String()

	// Get timeout from provider credentials (defaults to 10 minutes)
	timeout := time.Duration(custom.DefaultTimeoutMinutes) * time.Minute

	tx := s.db.WithContext(ctx).Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Attempt to acquire lock on existing active challenges for this FQDN
	var existing models.ManualChallenge
	err := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "NOWAIT"}).
		Where("fqdn = ? AND status IN ?", req.FQDN, []string{
			string(models.ChallengeStatusCreated),
			string(models.ChallengeStatusPending),
			string(models.ChallengeStatusVerifying),
		}).
		First(&existing).Error

	if err == nil {
		// Active challenge exists
		if existing.UserID == req.UserID {
			// Same user - return existing challenge
			tx.Rollback()
			return &existing, nil
		}
		// Different user - reject
		tx.Rollback()
		return nil, ErrChallengeInProgress
	}

	if !errors.Is(err, gorm.ErrRecordNotFound) {
		// Lock acquisition failed or other error
		tx.Rollback()
		return nil, fmt.Errorf("failed to check existing challenges: %w", err)
	}

	// No active challenge exists - create new one
	now := time.Now()
	challenge := &models.ManualChallenge{
		ID:         challengeID,
		ProviderID: req.ProviderID,
		UserID:     req.UserID,
		FQDN:       req.FQDN,
		Token:      req.Token,
		Value:      req.Value,
		Status:     models.ChallengeStatusPending,
		CreatedAt:  now,
		ExpiresAt:  now.Add(timeout),
	}

	if err := tx.Create(challenge).Error; err != nil {
		tx.Rollback()
		return nil, fmt.Errorf("failed to create challenge: %w", err)
	}

	if err := tx.Commit().Error; err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	logger.Log().WithField("challenge_id", challengeID).
		WithField("fqdn", req.FQDN).
		Info("Created manual DNS challenge")

	return challenge, nil
}

// GetChallenge retrieves a challenge by ID.
func (s *ManualChallengeService) GetChallenge(ctx context.Context, challengeID string) (*models.ManualChallenge, error) {
	var challenge models.ManualChallenge
	if err := s.db.WithContext(ctx).Where("id = ?", challengeID).First(&challenge).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrChallengeNotFound
		}
		return nil, fmt.Errorf("failed to get challenge: %w", err)
	}
	return &challenge, nil
}

// GetChallengeForUser retrieves a challenge and verifies ownership.
func (s *ManualChallengeService) GetChallengeForUser(ctx context.Context, challengeID string, userID uint) (*models.ManualChallenge, error) {
	challenge, err := s.GetChallenge(ctx, challengeID)
	if err != nil {
		return nil, err
	}

	if challenge.UserID != userID {
		logger.Log().Warn("Unauthorized challenge access attempt",
			"challenge_id", challengeID,
			"owner_id", challenge.UserID,
			"requester_id", userID,
		)
		return nil, ErrUnauthorized
	}

	return challenge, nil
}

// ListChallengesForProvider lists all challenges for a specific provider.
func (s *ManualChallengeService) ListChallengesForProvider(ctx context.Context, providerID, userID uint) ([]models.ManualChallenge, error) {
	var challenges []models.ManualChallenge
	if err := s.db.WithContext(ctx).
		Where("provider_id = ? AND user_id = ?", providerID, userID).
		Order("created_at DESC").
		Find(&challenges).Error; err != nil {
		return nil, fmt.Errorf("failed to list challenges: %w", err)
	}
	return challenges, nil
}

// VerifyResult represents the result of a DNS verification attempt.
type VerifyResult struct {
	Success       bool   `json:"success"`
	DNSFound      bool   `json:"dns_found"`
	Message       string `json:"message,omitempty"`
	Status        string `json:"status"`
	TimeRemaining int    `json:"time_remaining_seconds,omitempty"`
}

// VerifyChallenge triggers DNS verification for a challenge.
func (s *ManualChallengeService) VerifyChallenge(ctx context.Context, challengeID string, userID uint) (*VerifyResult, error) {
	// Use mutex to prevent concurrent verification of the same challenge
	s.verifyMu.Lock()
	defer s.verifyMu.Unlock()

	challenge, err := s.GetChallengeForUser(ctx, challengeID, userID)
	if err != nil {
		return nil, err
	}

	// Check if challenge has expired
	if time.Now().After(challenge.ExpiresAt) {
		if challenge.Status != models.ChallengeStatusExpired {
			s.updateChallengeStatus(ctx, challenge, models.ChallengeStatusExpired, "Challenge timed out")
		}
		return nil, ErrChallengeExpired
	}

	// Check if already in terminal state
	if challenge.IsTerminal() {
		return &VerifyResult{
			Success:  challenge.Status == models.ChallengeStatusVerified,
			DNSFound: challenge.DNSPropagated,
			Message:  fmt.Sprintf("Challenge is in terminal state: %s", challenge.Status),
			Status:   string(challenge.Status),
		}, nil
	}

	// Perform DNS lookup
	now := time.Now()
	challenge.LastCheckAt = &now
	dnsFound := s.checkDNSPropagation(ctx, challenge.FQDN, challenge.Value)
	challenge.DNSPropagated = dnsFound

	if dnsFound {
		// DNS record found - mark as verified
		challenge.Status = models.ChallengeStatusVerified
		challenge.VerifiedAt = &now

		if err := s.db.WithContext(ctx).Save(challenge).Error; err != nil {
			logger.Log().WithError(err).Error("Failed to update challenge status to verified")
		}

		logger.Log().WithField("challenge_id", challengeID).
			WithField("fqdn", challenge.FQDN).
			Info("Manual DNS challenge verified successfully")

		return &VerifyResult{
			Success:  true,
			DNSFound: true,
			Message:  "DNS TXT record verified successfully",
			Status:   string(models.ChallengeStatusVerified),
		}, nil
	}

	// DNS record not found yet
	challenge.Status = models.ChallengeStatusPending
	if err := s.db.WithContext(ctx).Save(challenge).Error; err != nil {
		logger.Log().WithError(err).Error("Failed to update challenge last check time")
	}

	return &VerifyResult{
		Success:       false,
		DNSFound:      false,
		Message:       "DNS TXT record not found. Please ensure the record is created and wait for propagation.",
		Status:        string(models.ChallengeStatusPending),
		TimeRemaining: int(challenge.TimeRemaining().Seconds()),
	}, nil
}

// PollChallengeStatus returns the current status of a challenge for polling.
func (s *ManualChallengeService) PollChallengeStatus(ctx context.Context, challengeID string, userID uint) (*ChallengeStatusResponse, error) {
	challenge, err := s.GetChallengeForUser(ctx, challengeID, userID)
	if err != nil {
		return nil, err
	}

	// Check for expiration
	if time.Now().After(challenge.ExpiresAt) && challenge.Status != models.ChallengeStatusExpired {
		s.updateChallengeStatus(ctx, challenge, models.ChallengeStatusExpired, "Challenge timed out")
		challenge.Status = models.ChallengeStatusExpired
	}

	return &ChallengeStatusResponse{
		ID:                   challenge.ID,
		Status:               string(challenge.Status),
		DNSPropagated:        challenge.DNSPropagated,
		TimeRemainingSeconds: int(challenge.TimeRemaining().Seconds()),
		LastCheckAt:          challenge.LastCheckAt,
	}, nil
}

// ChallengeStatusResponse represents the response for challenge status polling.
type ChallengeStatusResponse struct {
	ID                   string     `json:"id"`
	Status               string     `json:"status"`
	DNSPropagated        bool       `json:"dns_propagated"`
	TimeRemainingSeconds int        `json:"time_remaining_seconds"`
	LastCheckAt          *time.Time `json:"last_check_at,omitempty"`
}

// DeleteChallenge deletes a challenge.
func (s *ManualChallengeService) DeleteChallenge(ctx context.Context, challengeID string, userID uint) error {
	challenge, err := s.GetChallengeForUser(ctx, challengeID, userID)
	if err != nil {
		return err
	}

	if err := s.db.WithContext(ctx).Delete(challenge).Error; err != nil {
		return fmt.Errorf("failed to delete challenge: %w", err)
	}

	logger.Log().WithField("challenge_id", challengeID).Info("Manual DNS challenge deleted")
	return nil
}

// checkDNSPropagation queries DNS for the TXT record.
func (s *ManualChallengeService) checkDNSPropagation(ctx context.Context, fqdn, expectedValue string) bool {
	// Create a context with timeout for DNS lookup
	lookupCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	records, err := s.resolver.LookupTXT(lookupCtx, fqdn)
	if err != nil {
		logger.Log().WithError(err).
			WithField("fqdn", fqdn).
			Debug("DNS TXT lookup failed")
		return false
	}

	// Check if any of the TXT records match the expected value
	for _, record := range records {
		// TXT records may be split into multiple strings, join them
		cleanRecord := strings.TrimSpace(record)
		if cleanRecord == expectedValue {
			return true
		}
	}

	logger.Log().WithField("fqdn", fqdn).
		WithField("found_records", len(records)).
		Debug("DNS TXT record not found or value mismatch")

	return false
}

// updateChallengeStatus updates the status of a challenge.
func (s *ManualChallengeService) updateChallengeStatus(ctx context.Context, challenge *models.ManualChallenge, status models.ChallengeStatus, message string) {
	challenge.Status = status
	challenge.ErrorMessage = message
	if err := s.db.WithContext(ctx).Save(challenge).Error; err != nil {
		logger.Log().WithError(err).
			WithField("challenge_id", challenge.ID).
			Error("Failed to update challenge status")
	}
}

// cleanupExpiredChallenges marks pending challenges as expired and deletes old challenges.
func (s *ManualChallengeService) cleanupExpiredChallenges() {
	// Mark challenges in pending state that have passed their expiration as expired
	expiredCount := s.db.Model(&models.ManualChallenge{}).
		Where("status IN ? AND expires_at < ?",
			[]string{
				string(models.ChallengeStatusCreated),
				string(models.ChallengeStatusPending),
				string(models.ChallengeStatusVerifying),
			},
			time.Now(),
		).
		Updates(map[string]interface{}{
			"status":        models.ChallengeStatusExpired,
			"error_message": "Challenge timed out",
		}).RowsAffected

	if expiredCount > 0 {
		logger.Log().WithField("count", expiredCount).Info("Marked expired manual challenges")
	}

	// Hard delete challenges older than 7 days
	deleteCutoff := time.Now().Add(-7 * 24 * time.Hour)
	deleteResult := s.db.Where("created_at < ?", deleteCutoff).Delete(&models.ManualChallenge{})
	if deleteResult.Error != nil {
		logger.Log().WithError(deleteResult.Error).Error("Failed to delete old manual challenges")
	} else if deleteResult.RowsAffected > 0 {
		logger.Log().WithField("count", deleteResult.RowsAffected).Info("Deleted old manual challenges")
	}
}

// GetActiveChallengeForFQDN returns an active challenge for a given FQDN if one exists.
func (s *ManualChallengeService) GetActiveChallengeForFQDN(ctx context.Context, fqdn string, userID uint) (*models.ManualChallenge, error) {
	var challenge models.ManualChallenge
	err := s.db.WithContext(ctx).
		Where("fqdn = ? AND user_id = ? AND status IN ?", fqdn, userID, []string{
			string(models.ChallengeStatusCreated),
			string(models.ChallengeStatusPending),
			string(models.ChallengeStatusVerifying),
		}).
		First(&challenge).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get active challenge: %w", err)
	}

	return &challenge, nil
}
