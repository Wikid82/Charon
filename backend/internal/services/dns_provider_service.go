package services

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Wikid82/charon/backend/internal/crypto"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/google/uuid"
	"gorm.io/gorm"
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

// SupportedProviderTypes defines the list of supported DNS provider types.
var SupportedProviderTypes = []string{
	"cloudflare",
	"route53",
	"digitalocean",
	"googleclouddns",
	"namecheap",
	"godaddy",
	"azure",
	"hetzner",
	"vultr",
	"dnsimple",
}

// ProviderCredentialFields maps provider types to their required credential fields.
var ProviderCredentialFields = map[string][]string{
	"cloudflare":     {"api_token"},
	"route53":        {"access_key_id", "secret_access_key", "region"},
	"digitalocean":   {"auth_token"},
	"googleclouddns": {"service_account_json", "project"},
	"namecheap":      {"api_user", "api_key", "client_ip"},
	"godaddy":        {"api_key", "api_secret"},
	"azure":          {"tenant_id", "client_id", "client_secret", "subscription_id", "resource_group"},
	"hetzner":        {"api_key"},
	"vultr":          {"api_key"},
	"dnsimple":       {"oauth_token", "account_id"},
}

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
type DNSProviderResponse struct {
	models.DNSProvider
	HasCredentials bool `json:"has_credentials"`
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
	Create(ctx context.Context, req CreateDNSProviderRequest) (*models.DNSProvider, error)
	Update(ctx context.Context, id uint, req UpdateDNSProviderRequest) (*models.DNSProvider, error)
	Delete(ctx context.Context, id uint) error
	Test(ctx context.Context, id uint) (*TestResult, error)
	TestCredentials(ctx context.Context, req CreateDNSProviderRequest) (*TestResult, error)
	GetDecryptedCredentials(ctx context.Context, id uint) (map[string]string, error)
}

// dnsProviderService implements the DNSProviderService interface.
type dnsProviderService struct {
	db        *gorm.DB
	encryptor *crypto.EncryptionService
}

// NewDNSProviderService creates a new DNS provider service.
func NewDNSProviderService(db *gorm.DB, encryptor *crypto.EncryptionService) DNSProviderService {
	return &dnsProviderService{
		db:        db,
		encryptor: encryptor,
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
	err := s.db.WithContext(ctx).First(&provider, id).Error
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

	// Encrypt credentials
	credentialsJSON, err := json.Marshal(req.Credentials)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrEncryptionFailed, err)
	}

	encryptedCreds, err := s.encryptor.Encrypt(credentialsJSON)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrEncryptionFailed, err)
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
		PropagationTimeout:   propagationTimeout,
		PollingInterval:      pollingInterval,
		IsDefault:            req.IsDefault,
		Enabled:              true,
	}

	if err := s.db.WithContext(ctx).Create(provider).Error; err != nil {
		return nil, err
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

	// Update fields if provided
	if req.Name != nil {
		provider.Name = *req.Name
	}

	if req.PropagationTimeout != nil {
		provider.PropagationTimeout = *req.PropagationTimeout
	}

	if req.PollingInterval != nil {
		provider.PollingInterval = *req.PollingInterval
	}

	if req.Enabled != nil {
		provider.Enabled = *req.Enabled
	}

	// Handle credentials update
	if req.Credentials != nil && len(req.Credentials) > 0 {
		// Validate credentials
		if err := validateCredentials(provider.ProviderType, req.Credentials); err != nil {
			return nil, err
		}

		// Encrypt new credentials
		credentialsJSON, err := json.Marshal(req.Credentials)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrEncryptionFailed, err)
		}

		encryptedCreds, err := s.encryptor.Encrypt(credentialsJSON)
		if err != nil {
			return nil, fmt.Errorf("%w: %v", ErrEncryptionFailed, err)
		}

		provider.CredentialsEncrypted = encryptedCreds
	}

	// Handle default provider logic
	if req.IsDefault != nil && *req.IsDefault {
		// Unset any existing default provider
		if err := s.db.WithContext(ctx).Model(&models.DNSProvider{}).Where("is_default = ? AND id != ?", true, id).Update("is_default", false).Error; err != nil {
			return nil, err
		}
		provider.IsDefault = true
	} else if req.IsDefault != nil && !*req.IsDefault {
		provider.IsDefault = false
	}

	// Save updates
	if err := s.db.WithContext(ctx).Save(provider).Error; err != nil {
		return nil, err
	}

	return provider, nil
}

// Delete deletes a DNS provider.
func (s *dnsProviderService) Delete(ctx context.Context, id uint) error {
	result := s.db.WithContext(ctx).Delete(&models.DNSProvider{}, id)
	if result.Error != nil {
		return result.Error
	}
	if result.RowsAffected == 0 {
		return ErrDNSProviderNotFound
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

	// Decrypt credentials
	decryptedData, err := s.encryptor.Decrypt(provider.CredentialsEncrypted)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDecryptionFailed, err)
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

	return credentials, nil
}

// isValidProviderType checks if a provider type is supported.
func isValidProviderType(providerType string) bool {
	for _, supported := range SupportedProviderTypes {
		if providerType == supported {
			return true
		}
	}
	return false
}

// validateCredentials validates that all required credential fields are present.
func validateCredentials(providerType string, credentials map[string]string) error {
	requiredFields, ok := ProviderCredentialFields[providerType]
	if !ok {
		return ErrInvalidProviderType
	}

	// Check for required fields
	for _, field := range requiredFields {
		if value, exists := credentials[field]; !exists || value == "" {
			return fmt.Errorf("%w: missing field '%s'", ErrInvalidCredentials, field)
		}
	}

	return nil
}

// testDNSProviderCredentials performs a basic validation test on DNS provider credentials.
// In a real implementation, this would make actual API calls to the DNS provider.
// For now, we simulate the test with basic validation.
func testDNSProviderCredentials(providerType string, credentials map[string]string) *TestResult {
	// Simulate validation logic
	// In production, this would make actual API calls to verify credentials

	startTime := time.Now()

	// Basic validation - check if credentials have the expected structure
	if err := validateCredentials(providerType, credentials); err != nil {
		return &TestResult{
			Success: false,
			Error:   err.Error(),
			Code:    "VALIDATION_ERROR",
		}
	}

	// Simulate API call delay
	elapsed := time.Since(startTime).Milliseconds()

	// For now, return success if validation passed
	// TODO: Implement actual API calls to DNS providers
	return &TestResult{
		Success:           true,
		Message:           "DNS provider credentials validated successfully (basic validation only)",
		PropagationTimeMs: elapsed,
	}
}
