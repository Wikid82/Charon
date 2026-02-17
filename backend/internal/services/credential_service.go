package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Wikid82/charon/backend/internal/crypto"
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/google/uuid"
	"golang.org/x/net/idna"
	"gorm.io/gorm"
)

var (
	// ErrCredentialNotFound is returned when a credential is not found.
	ErrCredentialNotFound = errors.New("credential not found")
	// ErrNoMatchingCredential is returned when no credential matches the domain.
	ErrNoMatchingCredential = errors.New("no matching credential found for domain")
	// ErrMultiCredentialNotEnabled is returned when trying to use multi-credential features on a provider that doesn't have it enabled.
	ErrMultiCredentialNotEnabled = errors.New("multi-credential mode not enabled for this provider")
)

// CreateCredentialRequest represents the request to create a new credential.
type CreateCredentialRequest struct {
	Label              string            `json:"label" binding:"required"`
	ZoneFilter         string            `json:"zone_filter"` // Comma-separated domains
	Credentials        map[string]string `json:"credentials" binding:"required"`
	PropagationTimeout int               `json:"propagation_timeout"`
	PollingInterval    int               `json:"polling_interval"`
	Enabled            bool              `json:"enabled"`
}

// UpdateCredentialRequest represents the request to update a credential.
type UpdateCredentialRequest struct {
	Label              *string           `json:"label"`
	ZoneFilter         *string           `json:"zone_filter"`
	Credentials        map[string]string `json:"credentials,omitempty"`
	PropagationTimeout *int              `json:"propagation_timeout"`
	PollingInterval    *int              `json:"polling_interval"`
	Enabled            *bool             `json:"enabled"`
}

// CredentialService provides operations for managing DNS provider credentials.
type CredentialService interface {
	List(ctx context.Context, providerID uint) ([]models.DNSProviderCredential, error)
	Get(ctx context.Context, providerID, credentialID uint) (*models.DNSProviderCredential, error)
	Create(ctx context.Context, providerID uint, req CreateCredentialRequest) (*models.DNSProviderCredential, error)
	Update(ctx context.Context, providerID, credentialID uint, req UpdateCredentialRequest) (*models.DNSProviderCredential, error)
	Delete(ctx context.Context, providerID, credentialID uint) error
	Test(ctx context.Context, providerID, credentialID uint) (*TestResult, error)
	GetCredentialForDomain(ctx context.Context, providerID uint, domain string) (*models.DNSProviderCredential, error)
	EnableMultiCredentials(ctx context.Context, providerID uint) error
}

// credentialService implements the CredentialService interface.
type credentialService struct {
	db              *gorm.DB
	encryptor       *crypto.EncryptionService
	rotationService *crypto.RotationService
	securityService *SecurityService
}

// NewCredentialService creates a new credential service.
func NewCredentialService(db *gorm.DB, encryptor *crypto.EncryptionService) CredentialService {
	// Attempt to create rotation service (optional for backward compatibility)
	rotationService, err := crypto.NewRotationService(db)
	if err != nil {
		fmt.Printf("Warning: RotationService initialization failed, using basic encryption: %v\n", err)
	}

	return &credentialService{
		db:              db,
		encryptor:       encryptor,
		rotationService: rotationService,
		securityService: NewSecurityService(db),
	}
}

// List retrieves all credentials for a DNS provider.
func (s *credentialService) List(ctx context.Context, providerID uint) ([]models.DNSProviderCredential, error) {
	// Verify provider exists and has multi-credential enabled
	var provider models.DNSProvider
	if err := s.db.WithContext(ctx).Where("id = ?", providerID).First(&provider).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDNSProviderNotFound
		}
		return nil, err
	}

	if !provider.UseMultiCredentials {
		return nil, ErrMultiCredentialNotEnabled
	}

	var credentials []models.DNSProviderCredential
	err := s.db.WithContext(ctx).
		Where("dns_provider_id = ?", providerID).
		Order("label ASC").
		Find(&credentials).Error

	return credentials, err
}

// Get retrieves a specific credential by ID.
func (s *credentialService) Get(ctx context.Context, providerID, credentialID uint) (*models.DNSProviderCredential, error) {
	var credential models.DNSProviderCredential
	err := s.db.WithContext(ctx).
		Where("id = ? AND dns_provider_id = ?", credentialID, providerID).
		First(&credential).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrCredentialNotFound
		}
		return nil, err
	}

	return &credential, nil
}

// Create creates a new credential for a DNS provider.
func (s *credentialService) Create(ctx context.Context, providerID uint, req CreateCredentialRequest) (*models.DNSProviderCredential, error) {
	// Verify provider exists and has multi-credential enabled
	var provider models.DNSProvider
	if err := s.db.WithContext(ctx).Where("id = ?", providerID).First(&provider).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDNSProviderNotFound
		}
		return nil, err
	}

	if !provider.UseMultiCredentials {
		return nil, ErrMultiCredentialNotEnabled
	}

	// Validate credentials for provider type
	if err := validateCredentials(provider.ProviderType, req.Credentials); err != nil {
		return nil, err
	}

	// Encrypt credentials using RotationService if available
	var encryptedCreds string
	var keyVersion int
	credentialsJSON, err := json.Marshal(req.Credentials)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrEncryptionFailed, err)
	}

	if s.rotationService != nil {
		encryptedCreds, keyVersion, err = s.rotationService.EncryptWithCurrentKey(credentialsJSON)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrEncryptionFailed, err)
		}
	} else {
		encryptedCreds, err = s.encryptor.Encrypt(credentialsJSON)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrEncryptionFailed, err)
		}
		keyVersion = 1
	}

	// Set defaults
	propagationTimeout := req.PropagationTimeout
	if propagationTimeout == 0 {
		propagationTimeout = provider.PropagationTimeout
	}

	pollingInterval := req.PollingInterval
	if pollingInterval == 0 {
		pollingInterval = provider.PollingInterval
	}

	enabled := req.Enabled
	// Default to true if not specified in request
	if !enabled && req.Enabled {
		enabled = true
	} else if !req.Enabled {
		enabled = true // Default to enabled
	}

	// Create credential
	credential := &models.DNSProviderCredential{
		UUID:                 uuid.New().String(),
		DNSProviderID:        providerID,
		Label:                req.Label,
		ZoneFilter:           strings.TrimSpace(req.ZoneFilter),
		CredentialsEncrypted: encryptedCreds,
		KeyVersion:           keyVersion,
		PropagationTimeout:   propagationTimeout,
		PollingInterval:      pollingInterval,
		Enabled:              enabled,
	}

	if err := s.db.WithContext(ctx).Create(credential).Error; err != nil {
		return nil, err
	}

	// Log audit event
	detailsJSON, _ := json.Marshal(map[string]interface{}{
		"label":       req.Label,
		"zone_filter": req.ZoneFilter,
		"provider_id": providerID,
	})
	if err := s.securityService.LogAudit(&models.SecurityAudit{
		Actor:         getActorFromContext(ctx),
		Action:        "credential_create",
		EventCategory: "dns_provider",
		ResourceID:    &provider.ID,
		ResourceUUID:  provider.UUID,
		Details:       string(detailsJSON),
		IPAddress:     getIPFromContext(ctx),
		UserAgent:     getUserAgentFromContext(ctx),
	}); err != nil {
		logger.Log().WithError(err).Warn("Failed to log audit event")
	}

	return credential, nil
}

// Update updates an existing credential.
func (s *credentialService) Update(ctx context.Context, providerID, credentialID uint, req UpdateCredentialRequest) (*models.DNSProviderCredential, error) {
	// Fetch existing credential
	credential, err := s.Get(ctx, providerID, credentialID)
	if err != nil {
		return nil, err
	}

	// Fetch provider for validation and audit logging
	var provider models.DNSProvider
	if findErr := s.db.WithContext(ctx).Where("id = ?", providerID).First(&provider).Error; findErr != nil {
		return nil, findErr
	}

	// Track changed fields for audit log
	changedFields := make(map[string]interface{})
	oldValues := make(map[string]interface{})
	newValues := make(map[string]interface{})

	// Update fields if provided
	if req.Label != nil && *req.Label != credential.Label {
		oldValues["label"] = credential.Label
		newValues["label"] = *req.Label
		changedFields["label"] = true
		credential.Label = *req.Label
	}

	if req.ZoneFilter != nil && *req.ZoneFilter != credential.ZoneFilter {
		oldValues["zone_filter"] = credential.ZoneFilter
		newValues["zone_filter"] = *req.ZoneFilter
		changedFields["zone_filter"] = true
		credential.ZoneFilter = strings.TrimSpace(*req.ZoneFilter)
	}

	if req.PropagationTimeout != nil && *req.PropagationTimeout != credential.PropagationTimeout {
		oldValues["propagation_timeout"] = credential.PropagationTimeout
		newValues["propagation_timeout"] = *req.PropagationTimeout
		changedFields["propagation_timeout"] = true
		credential.PropagationTimeout = *req.PropagationTimeout
	}

	if req.PollingInterval != nil && *req.PollingInterval != credential.PollingInterval {
		oldValues["polling_interval"] = credential.PollingInterval
		newValues["polling_interval"] = *req.PollingInterval
		changedFields["polling_interval"] = true
		credential.PollingInterval = *req.PollingInterval
	}

	if req.Enabled != nil && *req.Enabled != credential.Enabled {
		oldValues["enabled"] = credential.Enabled
		newValues["enabled"] = *req.Enabled
		changedFields["enabled"] = true
		credential.Enabled = *req.Enabled
	}

	// Handle credentials update
	if len(req.Credentials) > 0 {
		// Validate credentials
		if err := validateCredentials(provider.ProviderType, req.Credentials); err != nil {
			return nil, err
		}

		// Encrypt new credentials with version tracking
		credentialsJSON, err := json.Marshal(req.Credentials)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrEncryptionFailed, err)
		}

		var encryptedCreds string
		var keyVersion int
		if s.rotationService != nil {
			encryptedCreds, keyVersion, err = s.rotationService.EncryptWithCurrentKey(credentialsJSON)
			if err != nil {
				return nil, fmt.Errorf("%w: %v", ErrEncryptionFailed, err)
			}
		} else {
			encryptedCreds, err = s.encryptor.Encrypt(credentialsJSON)
			if err != nil {
				return nil, fmt.Errorf("%w: %v", ErrEncryptionFailed, err)
			}
			keyVersion = 1
		}

		changedFields["credentials"] = true
		credential.CredentialsEncrypted = encryptedCreds
		credential.KeyVersion = keyVersion
	}

	// Save updates
	if err := s.db.WithContext(ctx).Save(credential).Error; err != nil {
		return nil, err
	}

	// Log audit event if any changes were made
	if len(changedFields) > 0 {
		detailsJSON, _ := json.Marshal(map[string]interface{}{
			"credential_id":  credentialID,
			"changed_fields": changedFields,
			"old_values":     oldValues,
			"new_values":     newValues,
		})
		if err := s.securityService.LogAudit(&models.SecurityAudit{
			Actor:         getActorFromContext(ctx),
			Action:        "credential_update",
			EventCategory: "dns_provider",
			ResourceID:    &provider.ID,
			ResourceUUID:  provider.UUID,
			Details:       string(detailsJSON),
			IPAddress:     getIPFromContext(ctx),
			UserAgent:     getUserAgentFromContext(ctx),
		}); err != nil {
			logger.Log().WithError(err).Warn("Failed to log audit event")
		}
	}

	return credential, nil
}

// Delete deletes a credential.
func (s *credentialService) Delete(ctx context.Context, providerID, credentialID uint) error {
	// Fetch credential and provider for audit log
	credential, err := s.Get(ctx, providerID, credentialID)
	if err != nil {
		return err
	}

	var provider models.DNSProvider
	if err := s.db.WithContext(ctx).Where("id = ?", providerID).First(&provider).Error; err != nil {
		return err
	}

	const maxDeleteAttempts = 5
	var result *gorm.DB
	for attempt := 1; attempt <= maxDeleteAttempts; attempt++ {
		result = s.db.WithContext(ctx).Delete(&models.DNSProviderCredential{}, credentialID)
		if result.Error == nil {
			break
		}

		errMsg := strings.ToLower(result.Error.Error())
		isTransientLock := strings.Contains(errMsg, "database is locked") || strings.Contains(errMsg, "database table is locked") || strings.Contains(errMsg, "busy")
		if !isTransientLock || attempt == maxDeleteAttempts {
			return result.Error
		}

		time.Sleep(time.Duration(attempt) * 10 * time.Millisecond)
	}

	if result == nil || result.RowsAffected == 0 {
		return ErrCredentialNotFound
	}

	// Log audit event
	detailsJSON, _ := json.Marshal(map[string]interface{}{
		"credential_id": credentialID,
		"label":         credential.Label,
		"zone_filter":   credential.ZoneFilter,
	})
	if err := s.securityService.LogAudit(&models.SecurityAudit{
		Actor:         getActorFromContext(ctx),
		Action:        "credential_delete",
		EventCategory: "dns_provider",
		ResourceID:    &provider.ID,
		ResourceUUID:  provider.UUID,
		Details:       string(detailsJSON),
		IPAddress:     getIPFromContext(ctx),
		UserAgent:     getUserAgentFromContext(ctx),
	}); err != nil {
		logger.Log().WithError(err).Warn("Failed to log audit event")
	}

	return nil
}

// Test tests a credential's connectivity.
func (s *credentialService) Test(ctx context.Context, providerID, credentialID uint) (*TestResult, error) {
	credential, err := s.Get(ctx, providerID, credentialID)
	if err != nil {
		return nil, err
	}

	var provider models.DNSProvider
	if findErr := s.db.WithContext(ctx).Where("id = ?", providerID).First(&provider).Error; findErr != nil {
		return nil, findErr
	}

	// Decrypt credentials
	var decryptedData []byte
	if s.rotationService != nil {
		decryptedData, err = s.rotationService.DecryptWithVersion(credential.CredentialsEncrypted, credential.KeyVersion)
		if err != nil {
			return &TestResult{
				Success: false,
				Error:   "Failed to decrypt credentials",
				Code:    "DECRYPTION_ERROR",
			}, nil
		}
	} else {
		decryptedData, err = s.encryptor.Decrypt(credential.CredentialsEncrypted)
		if err != nil {
			return &TestResult{
				Success: false,
				Error:   "Failed to decrypt credentials",
				Code:    "DECRYPTION_ERROR",
			}, nil
		}
	}

	var credentials map[string]string
	if err := json.Unmarshal(decryptedData, &credentials); err != nil {
		return &TestResult{
			Success: false,
			Error:   "Invalid credential format",
			Code:    "INVALID_FORMAT",
		}, nil
	}

	// Perform test using the shared test function
	result := testDNSProviderCredentials(provider.ProviderType, credentials)

	// Update credential statistics
	if result.Success {
		credential.SuccessCount++
		credential.LastError = ""
	} else {
		credential.FailureCount++
		credential.LastError = result.Error
	}
	_ = s.db.WithContext(ctx).Save(credential)

	// Log audit event
	detailsJSON, _ := json.Marshal(map[string]interface{}{
		"credential_id": credentialID,
		"label":         credential.Label,
		"test_result":   result.Success,
		"error":         result.Error,
	})
	if err := s.securityService.LogAudit(&models.SecurityAudit{
		Actor:         getActorFromContext(ctx),
		Action:        "credential_test",
		EventCategory: "dns_provider",
		ResourceID:    &provider.ID,
		ResourceUUID:  provider.UUID,
		Details:       string(detailsJSON),
		IPAddress:     getIPFromContext(ctx),
		UserAgent:     getUserAgentFromContext(ctx),
	}); err != nil {
		logger.Log().WithError(err).Warn("Failed to log audit event")
	}

	return result, nil
}

// GetCredentialForDomain selects the best credential match for a domain.
// Priority: exact match > wildcard match > catch-all (empty zone_filter)
func (s *credentialService) GetCredentialForDomain(ctx context.Context, providerID uint, domain string) (*models.DNSProviderCredential, error) {
	// Verify provider exists
	var provider models.DNSProvider
	if err := s.db.WithContext(ctx).Where("id = ?", providerID).First(&provider).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDNSProviderNotFound
		}
		return nil, err
	}

	// If not using multi-credentials, return nil (caller should use provider's main credentials)
	if !provider.UseMultiCredentials {
		return nil, nil
	}

	// Normalize domain (convert IDN to punycode)
	normalizedDomain, err := idna.ToASCII(strings.ToLower(strings.TrimSpace(domain)))
	if err != nil {
		return nil, fmt.Errorf("failed to normalize domain: %w", err)
	}

	// Find all enabled credentials for this provider (without preload)
	var credentials []models.DNSProviderCredential
	if err := s.db.WithContext(ctx).
		Where("dns_provider_id = ? AND enabled = ?", providerID, true).
		Find(&credentials).Error; err != nil {
		return nil, err
	}

	if len(credentials) == 0 {
		return nil, ErrNoMatchingCredential
	}

	// Priority 1: Exact match
	for _, cred := range credentials {
		if matchesDomain(cred.ZoneFilter, normalizedDomain, true) {
			return &cred, nil
		}
	}

	// Priority 2: Wildcard match
	for _, cred := range credentials {
		if matchesDomain(cred.ZoneFilter, normalizedDomain, false) {
			return &cred, nil
		}
	}

	// Priority 3: Catch-all (empty zone_filter)
	for _, cred := range credentials {
		if strings.TrimSpace(cred.ZoneFilter) == "" {
			return &cred, nil
		}
	}

	return nil, ErrNoMatchingCredential
}

// matchesDomain checks if a domain matches a zone filter pattern.
// exactOnly=true means only check for exact matches, false allows wildcards.
func matchesDomain(zoneFilter, domain string, exactOnly bool) bool {
	if strings.TrimSpace(zoneFilter) == "" {
		return false // Empty filter is catch-all, handled separately
	}

	// Parse comma-separated zones
	zones := strings.Split(zoneFilter, ",")
	for _, zone := range zones {
		zone = strings.ToLower(strings.TrimSpace(zone))
		if zone == "" {
			continue
		}

		// Normalize zone (IDN to punycode)
		normalizedZone, err := idna.ToASCII(zone)
		if err != nil {
			continue
		}

		// Exact match
		if normalizedZone == domain {
			return true
		}

		// Wildcard match (only if not exact-only)
		if !exactOnly && strings.HasPrefix(normalizedZone, "*.") {
			suffix := normalizedZone[2:] // Remove "*."
			if strings.HasSuffix(domain, "."+suffix) || domain == suffix {
				return true
			}
		}
	}

	return false
}

// EnableMultiCredentials migrates a provider from single to multi-credential mode.
func (s *credentialService) EnableMultiCredentials(ctx context.Context, providerID uint) error {
	// Fetch provider
	var provider models.DNSProvider
	if err := s.db.WithContext(ctx).Where("id = ?", providerID).First(&provider).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrDNSProviderNotFound
		}
		return err
	}

	// Already enabled
	if provider.UseMultiCredentials {
		return nil
	}

	// Check if provider has existing credentials
	if provider.CredentialsEncrypted == "" {
		return errors.New("provider has no credentials to migrate")
	}

	// Create a default credential with existing credentials
	credential := &models.DNSProviderCredential{
		UUID:                 uuid.New().String(),
		DNSProviderID:        provider.ID,
		Label:                "Default (migrated)",
		ZoneFilter:           "", // Empty = catch-all
		CredentialsEncrypted: provider.CredentialsEncrypted,
		KeyVersion:           provider.KeyVersion,
		PropagationTimeout:   provider.PropagationTimeout,
		PollingInterval:      provider.PollingInterval,
		Enabled:              true,
	}

	// Start transaction
	tx := s.db.WithContext(ctx).Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Create default credential
	if err := tx.Create(credential).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to create default credential: %w", err)
	}

	// Enable multi-credential mode
	if err := tx.Model(&provider).Update("use_multi_credentials", true).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("failed to enable multi-credential mode: %w", err)
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	// Log audit event
	detailsJSON, _ := json.Marshal(map[string]interface{}{
		"provider_id":               providerID,
		"provider_name":             provider.Name,
		"migrated_credential_label": credential.Label,
	})
	if err := s.securityService.LogAudit(&models.SecurityAudit{
		Actor:         getActorFromContext(ctx),
		Action:        "multi_credential_enabled",
		EventCategory: "dns_provider",
		ResourceID:    &provider.ID,
		ResourceUUID:  provider.UUID,
		Details:       string(detailsJSON),
		IPAddress:     getIPFromContext(ctx),
		UserAgent:     getUserAgentFromContext(ctx),
	}); err != nil {
		logger.Log().WithError(err).Warn("Failed to log audit event")
	}

	return nil
}
