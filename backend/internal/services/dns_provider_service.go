package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Wikid82/charon/backend/internal/crypto"
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/pkg/dnsprovider"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// contextKey is a custom type for context keys to avoid collisions (matches test usage)
type contextKey string

// Context key constants for extracting request metadata
const (
	contextKeyUserID    contextKey = "user_id"
	contextKeyClientIP  contextKey = "client_ip"
	contextKeyUserAgent contextKey = "user_agent"
)

var (
	// ErrDNSProviderNotFound is returned when a DNS provider is not found.
	ErrDNSProviderNotFound = errors.New("dns provider not found")
	// ErrInvalidProviderType is returned when an unsupported provider type is specified.
	ErrInvalidProviderType = errors.New("invalid provider type")
	// ErrInvalidCredentials is returned when required credentials are missing.
	ErrInvalidCredentials = errors.New("invalid credentials: missing required fields")
	// ErrEncryptionFailed is returned when credential encryption fails.
	ErrEncryptionFailed = errors.New("failed to encrypt credentials")
	// ErrDecryptionFailed is returned when credential decryption fails.
	ErrDecryptionFailed = errors.New("failed to decrypt credentials")
)

// Registry-based provider management replaces hardcoded provider types.
// Provider types and credential fields are now queried from dnsprovider.Global().

// CreateDNSProviderRequest represents the request to create a new DNS provider.
type CreateDNSProviderRequest struct {
	Name               string            `json:"name" binding:"required"`
	ProviderType       string            `json:"provider_type" binding:"required"`
	Credentials        map[string]string `json:"credentials" binding:"required"`
	PropagationTimeout int               `json:"propagation_timeout"`
	PollingInterval    int               `json:"polling_interval"`
	IsDefault          bool              `json:"is_default"`
}

// UpdateDNSProviderRequest represents the request to update an existing DNS provider.
type UpdateDNSProviderRequest struct {
	Name               *string           `json:"name"`
	Credentials        map[string]string `json:"credentials,omitempty"`
	PropagationTimeout *int              `json:"propagation_timeout"`
	PollingInterval    *int              `json:"polling_interval"`
	IsDefault          *bool             `json:"is_default"`
	Enabled            *bool             `json:"enabled"`
}

// DNSProviderResponse represents the API response for a DNS provider.
// Uses explicit fields to avoid exposing internal database IDs.
type DNSProviderResponse struct {
	UUID                string     `json:"uuid"`
	Name                string     `json:"name"`
	ProviderType        string     `json:"provider_type"`
	Enabled             bool       `json:"enabled"`
	IsDefault           bool       `json:"is_default"`
	UseMultiCredentials bool       `json:"use_multi_credentials"`
	KeyVersion          int        `json:"key_version"`
	PropagationTimeout  int        `json:"propagation_timeout"`
	PollingInterval     int        `json:"polling_interval"`
	LastUsedAt          *time.Time `json:"last_used_at,omitempty"`
	SuccessCount        int        `json:"success_count"`
	FailureCount        int        `json:"failure_count"`
	LastError           string     `json:"last_error,omitempty"`
	CreatedAt           time.Time  `json:"created_at"`
	UpdatedAt           time.Time  `json:"updated_at"`
	HasCredentials      bool       `json:"has_credentials"`
}

// NewDNSProviderResponse creates a DNSProviderResponse from a DNSProvider model.
func NewDNSProviderResponse(provider *models.DNSProvider) DNSProviderResponse {
	return DNSProviderResponse{
		UUID:                provider.UUID,
		Name:                provider.Name,
		ProviderType:        provider.ProviderType,
		Enabled:             provider.Enabled,
		IsDefault:           provider.IsDefault,
		UseMultiCredentials: provider.UseMultiCredentials,
		KeyVersion:          provider.KeyVersion,
		PropagationTimeout:  provider.PropagationTimeout,
		PollingInterval:     provider.PollingInterval,
		LastUsedAt:          provider.LastUsedAt,
		SuccessCount:        provider.SuccessCount,
		FailureCount:        provider.FailureCount,
		LastError:           provider.LastError,
		CreatedAt:           provider.CreatedAt,
		UpdatedAt:           provider.UpdatedAt,
		HasCredentials:      provider.CredentialsEncrypted != "",
	}
}

// TestResult represents the result of testing DNS provider credentials.
type TestResult struct {
	Success           bool   `json:"success"`
	Message           string `json:"message,omitempty"`
	Error             string `json:"error,omitempty"`
	Code              string `json:"code,omitempty"`
	PropagationTimeMs int64  `json:"propagation_time_ms,omitempty"`
}

// DNSProviderService provides operations for managing DNS providers.
type DNSProviderService interface {
	List(ctx context.Context) ([]models.DNSProvider, error)
	Get(ctx context.Context, id uint) (*models.DNSProvider, error)
	GetByUUID(ctx context.Context, uuid string) (*models.DNSProvider, error)
	Create(ctx context.Context, req CreateDNSProviderRequest) (*models.DNSProvider, error)
	Update(ctx context.Context, id uint, req UpdateDNSProviderRequest) (*models.DNSProvider, error)
	Delete(ctx context.Context, id uint) error
	Test(ctx context.Context, id uint) (*TestResult, error)
	TestCredentials(ctx context.Context, req CreateDNSProviderRequest) (*TestResult, error)
	GetDecryptedCredentials(ctx context.Context, id uint) (map[string]string, error)
	GetSupportedProviderTypes() []string
	GetProviderCredentialFields(providerType string) ([]dnsprovider.CredentialFieldSpec, error)
}

// dnsProviderService implements the DNSProviderService interface.
type dnsProviderService struct {
	db              *gorm.DB
	encryptor       *crypto.EncryptionService
	rotationService *crypto.RotationService
	securityService *SecurityService
}

// NewDNSProviderService creates a new DNS provider service.
func NewDNSProviderService(db *gorm.DB, encryptor *crypto.EncryptionService) DNSProviderService {
	// Attempt to create rotation service (optional for backward compatibility)
	rotationService, err := crypto.NewRotationService(db)
	if err != nil {
		// Fallback to non-rotation mode
		fmt.Printf("Warning: RotationService initialization failed, using basic encryption: %v\n", err)
	}

	return &dnsProviderService{
		db:              db,
		encryptor:       encryptor,
		rotationService: rotationService,
		securityService: NewSecurityService(db),
	}
}

// List retrieves all DNS providers.
func (s *dnsProviderService) List(ctx context.Context) ([]models.DNSProvider, error) {
	var providers []models.DNSProvider
	err := s.db.WithContext(ctx).Order("is_default DESC, name ASC").Find(&providers).Error
	return providers, err
}

// Get retrieves a DNS provider by ID.
func (s *dnsProviderService) Get(ctx context.Context, id uint) (*models.DNSProvider, error) {
	var provider models.DNSProvider
	err := s.db.WithContext(ctx).Where("id = ?", id).First(&provider).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDNSProviderNotFound
		}
		return nil, err
	}
	return &provider, nil
}

// GetByUUID retrieves a DNS provider by UUID.
func (s *dnsProviderService) GetByUUID(ctx context.Context, providerUUID string) (*models.DNSProvider, error) {
	var provider models.DNSProvider
	err := s.db.WithContext(ctx).Where("uuid = ?", providerUUID).First(&provider).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrDNSProviderNotFound
		}
		return nil, err
	}
	return &provider, nil
}

// Create creates a new DNS provider with encrypted credentials.
func (s *dnsProviderService) Create(ctx context.Context, req CreateDNSProviderRequest) (*models.DNSProvider, error) {
	// Validate provider type
	if !isValidProviderType(req.ProviderType) {
		return nil, ErrInvalidProviderType
	}

	// Validate required credentials
	if err := validateCredentials(req.ProviderType, req.Credentials); err != nil {
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
		// Use rotation service for version tracking
		encryptedCreds, keyVersion, err = s.rotationService.EncryptWithCurrentKey(credentialsJSON)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrEncryptionFailed, err)
		}
	} else {
		// Fallback to basic encryption
		encryptedCreds, err = s.encryptor.Encrypt(credentialsJSON)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrEncryptionFailed, err)
		}
		keyVersion = 1
	}

	// Set defaults
	propagationTimeout := req.PropagationTimeout
	if propagationTimeout == 0 {
		propagationTimeout = 120
	}

	pollingInterval := req.PollingInterval
	if pollingInterval == 0 {
		pollingInterval = 5
	}

	// Handle default provider logic
	if req.IsDefault {
		// Unset any existing default provider
		if err := s.db.WithContext(ctx).Model(&models.DNSProvider{}).Where("is_default = ?", true).Update("is_default", false).Error; err != nil {
			return nil, err
		}
	}

	// Create provider
	provider := &models.DNSProvider{
		UUID:                 uuid.New().String(),
		Name:                 req.Name,
		ProviderType:         req.ProviderType,
		CredentialsEncrypted: encryptedCreds,
		KeyVersion:           keyVersion,
		PropagationTimeout:   propagationTimeout,
		PollingInterval:      pollingInterval,
		IsDefault:            req.IsDefault,
		Enabled:              true,
	}

	if err := s.db.WithContext(ctx).Create(provider).Error; err != nil {
		return nil, err
	}

	// Log audit event asynchronously
	detailsJSON, _ := json.Marshal(map[string]interface{}{
		"name":       req.Name,
		"type":       req.ProviderType,
		"is_default": req.IsDefault,
	})
	if err := s.securityService.LogAudit(&models.SecurityAudit{
		Actor:         getActorFromContext(ctx),
		Action:        "dns_provider_create",
		EventCategory: "dns_provider",
		ResourceID:    &provider.ID,
		ResourceUUID:  provider.UUID,
		Details:       string(detailsJSON),
		IPAddress:     getIPFromContext(ctx),
		UserAgent:     getUserAgentFromContext(ctx),
	}); err != nil {
		logger.Log().WithError(err).Warn("Failed to log audit event")
	}

	return provider, nil
}

// Update updates an existing DNS provider.
func (s *dnsProviderService) Update(ctx context.Context, id uint, req UpdateDNSProviderRequest) (*models.DNSProvider, error) {
	// Fetch existing provider
	provider, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	// Track changed fields for audit log
	changedFields := make(map[string]interface{})
	oldValues := make(map[string]interface{})
	newValues := make(map[string]interface{})

	// Update fields if provided
	if req.Name != nil && *req.Name != provider.Name {
		oldValues["name"] = provider.Name
		newValues["name"] = *req.Name
		changedFields["name"] = true
		provider.Name = *req.Name
	}

	if req.PropagationTimeout != nil && *req.PropagationTimeout != provider.PropagationTimeout {
		oldValues["propagation_timeout"] = provider.PropagationTimeout
		newValues["propagation_timeout"] = *req.PropagationTimeout
		changedFields["propagation_timeout"] = true
		provider.PropagationTimeout = *req.PropagationTimeout
	}

	if req.PollingInterval != nil && *req.PollingInterval != provider.PollingInterval {
		oldValues["polling_interval"] = provider.PollingInterval
		newValues["polling_interval"] = *req.PollingInterval
		changedFields["polling_interval"] = true
		provider.PollingInterval = *req.PollingInterval
	}

	if req.Enabled != nil && *req.Enabled != provider.Enabled {
		oldValues["enabled"] = provider.Enabled
		newValues["enabled"] = *req.Enabled
		changedFields["enabled"] = true
		provider.Enabled = *req.Enabled
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
		provider.CredentialsEncrypted = encryptedCreds
		provider.KeyVersion = keyVersion
	}

	// Handle default provider logic
	if req.IsDefault != nil && *req.IsDefault {
		// Unset any existing default provider
		if err := s.db.WithContext(ctx).Model(&models.DNSProvider{}).Where("is_default = ? AND id != ?", true, id).Update("is_default", false).Error; err != nil {
			return nil, err
		}
		oldValues["is_default"] = provider.IsDefault
		newValues["is_default"] = true
		changedFields["is_default"] = true
		provider.IsDefault = true
	} else if req.IsDefault != nil && !*req.IsDefault && provider.IsDefault {
		oldValues["is_default"] = provider.IsDefault
		newValues["is_default"] = false
		changedFields["is_default"] = true
		provider.IsDefault = false
	}

	// Save updates
	if err := s.db.WithContext(ctx).Save(provider).Error; err != nil {
		return nil, err
	}

	// Log audit event if any changes were made
	if len(changedFields) > 0 {
		detailsJSON, _ := json.Marshal(map[string]interface{}{
			"changed_fields": changedFields,
			"old_values":     oldValues,
			"new_values":     newValues,
		})
		if err := s.securityService.LogAudit(&models.SecurityAudit{
			Actor:         getActorFromContext(ctx),
			Action:        "dns_provider_update",
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

	return provider, nil
}

// Delete deletes a DNS provider.
func (s *dnsProviderService) Delete(ctx context.Context, id uint) error {
	// Fetch provider details for audit log before deletion
	provider, err := s.Get(ctx, id)
	if err != nil {
		return err
	}

	hadCredentials := provider.CredentialsEncrypted != ""

	result := s.db.WithContext(ctx).Delete(&models.DNSProvider{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrDNSProviderNotFound
	}

	// Log audit event
	detailsJSON, _ := json.Marshal(map[string]interface{}{
		"name":            provider.Name,
		"type":            provider.ProviderType,
		"had_credentials": hadCredentials,
	})
	if err := s.securityService.LogAudit(&models.SecurityAudit{
		Actor:         getActorFromContext(ctx),
		Action:        "dns_provider_delete",
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

// Test tests a saved DNS provider's credentials.
func (s *dnsProviderService) Test(ctx context.Context, id uint) (*TestResult, error) {
	provider, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	// Decrypt credentials
	credentials, err := s.GetDecryptedCredentials(ctx, id)
	if err != nil {
		// Update provider statistics even on decryption failure
		now := time.Now()
		provider.LastUsedAt = &now
		provider.FailureCount++
		provider.LastError = "Failed to decrypt credentials"
		_ = s.db.WithContext(ctx).Save(provider)

		return &TestResult{
			Success: false,
			Error:   "Failed to decrypt credentials",
			Code:    "DECRYPTION_ERROR",
		}, nil
	}

	// Perform test
	result := testDNSProviderCredentials(provider.ProviderType, credentials)

	// Update provider statistics
	now := time.Now()
	provider.LastUsedAt = &now

	if result.Success {
		provider.SuccessCount++
		provider.LastError = ""
	} else {
		provider.FailureCount++
		provider.LastError = result.Error
	}

	// Save statistics (ignore errors to avoid failing the test operation)
	_ = s.db.WithContext(ctx).Save(provider)

	// Log audit event
	detailsJSON, _ := json.Marshal(map[string]interface{}{
		"provider_name": provider.Name,
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

// TestCredentials tests DNS provider credentials without saving them.
func (s *dnsProviderService) TestCredentials(ctx context.Context, req CreateDNSProviderRequest) (*TestResult, error) {
	// Validate provider type
	if !isValidProviderType(req.ProviderType) {
		return &TestResult{
			Success: false,
			Error:   "Unsupported provider type",
			Code:    "INVALID_PROVIDER_TYPE",
		}, nil
	}

	// Validate credentials
	if err := validateCredentials(req.ProviderType, req.Credentials); err != nil {
		return &TestResult{
			Success: false,
			Error:   err.Error(),
			Code:    "INVALID_CREDENTIALS",
		}, nil
	}

	// Perform test
	return testDNSProviderCredentials(req.ProviderType, req.Credentials), nil
}

// GetDecryptedCredentials retrieves and decrypts a DNS provider's credentials.
func (s *dnsProviderService) GetDecryptedCredentials(ctx context.Context, id uint) (map[string]string, error) {
	provider, err := s.Get(ctx, id)
	if err != nil {
		return nil, err
	}

	// Decrypt credentials using rotation service if available (with version fallback)
	var decryptedData []byte
	if s.rotationService != nil {
		decryptedData, err = s.rotationService.DecryptWithVersion(provider.CredentialsEncrypted, provider.KeyVersion)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrDecryptionFailed, err)
		}
	} else {
		// Fallback to basic decryption
		decryptedData, err = s.encryptor.Decrypt(provider.CredentialsEncrypted)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrDecryptionFailed, err)
		}
	}

	// Parse JSON
	var credentials map[string]string
	if err := json.Unmarshal(decryptedData, &credentials); err != nil {
		return nil, fmt.Errorf("%w: invalid credential format", ErrDecryptionFailed)
	}

	// Update last used timestamp
	now := time.Now()
	provider.LastUsedAt = &now
	_ = s.db.WithContext(ctx).Save(provider)

	// Log audit event
	detailsJSON, _ := json.Marshal(map[string]interface{}{
		"purpose":     "credentials_access",
		"success":     true,
		"key_version": provider.KeyVersion,
	})
	if err := s.securityService.LogAudit(&models.SecurityAudit{
		Actor:         getActorFromContext(ctx),
		Action:        "credential_decrypt",
		EventCategory: "dns_provider",
		ResourceID:    &provider.ID,
		ResourceUUID:  provider.UUID,
		Details:       string(detailsJSON),
		IPAddress:     getIPFromContext(ctx),
		UserAgent:     getUserAgentFromContext(ctx),
	}); err != nil {
		logger.Log().WithError(err).Warn("Failed to log audit event")
	}

	return credentials, nil
}

// isValidProviderType checks if a provider type is supported.
func isValidProviderType(providerType string) bool {
	return dnsprovider.Global().IsSupported(providerType)
}

// validateCredentials validates that all required credential fields are present.
func validateCredentials(providerType string, credentials map[string]string) error {
	// Get provider from registry
	provider, ok := dnsprovider.Global().Get(providerType)
	if !ok {
		return ErrInvalidProviderType
	}

	// Use provider's validation method
	if err := provider.ValidateCredentials(credentials); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidCredentials, err)
	}

	return nil
}

// testDNSProviderCredentials performs validation and testing of DNS provider credentials.
func testDNSProviderCredentials(providerType string, credentials map[string]string) *TestResult {
	startTime := time.Now()

	// Get provider from registry
	provider, ok := dnsprovider.Global().Get(providerType)
	if !ok {
		return &TestResult{
			Success: false,
			Error:   "Provider type not supported",
			Code:    "INVALID_PROVIDER_TYPE",
		}
	}

	// Basic validation
	if err := provider.ValidateCredentials(credentials); err != nil {
		return &TestResult{
			Success: false,
			Error:   err.Error(),
			Code:    "VALIDATION_ERROR",
		}
	}

	// Test credentials with provider API
	if err := provider.TestCredentials(credentials); err != nil {
		return &TestResult{
			Success: false,
			Error:   err.Error(),
			Code:    "CREDENTIALS_TEST_FAILED",
		}
	}

	elapsed := time.Since(startTime).Milliseconds()

	return &TestResult{
		Success:           true,
		Message:           "DNS provider credentials validated and tested successfully",
		PropagationTimeMs: elapsed,
	}
}

// GetSupportedProviderTypes returns all registered provider types from the registry.
func (s *dnsProviderService) GetSupportedProviderTypes() []string {
	return dnsprovider.Global().Types()
}

// GetProviderCredentialFields returns the credential field specifications for a provider type.
func (s *dnsProviderService) GetProviderCredentialFields(providerType string) ([]dnsprovider.CredentialFieldSpec, error) {
	provider, ok := dnsprovider.Global().Get(providerType)
	if !ok {
		return nil, fmt.Errorf("unsupported provider type: %s", providerType)
	}

	// Combine required and optional fields
	fields := provider.RequiredCredentialFields()
	fields = append(fields, provider.OptionalCredentialFields()...)
	return fields, nil
}

// Helper functions to extract context information for audit logging

// getActorFromContext extracts the user ID from the context
func getActorFromContext(ctx context.Context) string {
	// Check for typed contextKey first (from tests and new code)
	if userID, ok := ctx.Value(contextKeyUserID).(string); ok && userID != "" {
		return userID
	}
	if userID, ok := ctx.Value(contextKeyUserID).(uint); ok && userID > 0 {
		return fmt.Sprintf("%d", userID)
	}
	// Fall back to bare string key (from middleware)
	if userID, ok := ctx.Value("user_id").(string); ok && userID != "" {
		return userID
	}
	if userID, ok := ctx.Value("user_id").(uint); ok && userID > 0 {
		return fmt.Sprintf("%d", userID)
	}
	return "system"
}

// getIPFromContext extracts the IP address from the context
func getIPFromContext(ctx context.Context) string {
	// Check for typed contextKey first
	if ip, ok := ctx.Value(contextKeyClientIP).(string); ok {
		return ip
	}
	// Fall back to bare string key
	if ip, ok := ctx.Value("client_ip").(string); ok {
		return ip
	}
	return ""
}

// getUserAgentFromContext extracts the User-Agent from the context
func getUserAgentFromContext(ctx context.Context) string {
	// Check for typed contextKey first
	if ua, ok := ctx.Value(contextKeyUserAgent).(string); ok {
		return ua
	}
	// Fall back to bare string key
	if ua, ok := ctx.Value("user_agent").(string); ok {
		return ua
	}
	return ""
}
