package caddy

import (
	"context"
	"encoding/json"
	"os"
	"testing"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/crypto"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	_ "github.com/Wikid82/charon/backend/pkg/dnsprovider/builtin" // Auto-register DNS providers
)

// encryptCredentials is a helper to encrypt credentials for test fixtures
func encryptCredentials(t *testing.T, credentials map[string]string) string {
	t.Helper()

	// Always use a valid 32-byte base64-encoded key (decodes to exactly 32 bytes)
	// base64.StdEncoding.EncodeToString([]byte("12345678901234567890123456789012"))
	// = "MTIzNDU2Nzg5MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTI="
	encryptionKey := "MTIzNDU2Nzg5MDEyMzQ1Njc4OTAxMjM0NTY3ODkwMTI="
	require.NoError(t, os.Setenv("CHARON_ENCRYPTION_KEY", encryptionKey))

	encryptor, err := crypto.NewEncryptionService(encryptionKey)
	require.NoError(t, err)

	credJSON, err := json.Marshal(credentials)
	require.NoError(t, err)

	encrypted, err := encryptor.Encrypt(credJSON)
	require.NoError(t, err)

	return encrypted
}

// setupTestDB creates an in-memory database for testing
func setupTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)

	// Auto-migrate all models including related ones
	err = db.AutoMigrate(
		&models.ProxyHost{},
		&models.Location{},
		&models.DNSProvider{},
		&models.DNSProviderCredential{},
		&models.SSLCertificate{},
		&models.Setting{},
		&models.SecurityConfig{},
		&models.AccessList{},
		&models.SecurityHeaderProfile{},
	)
	require.NoError(t, err)

	return db
}

// TestApplyConfig_SingleCredential_BackwardCompatibility tests that single-credential
// providers continue to work as before (backward compatibility)
func TestApplyConfig_SingleCredential_BackwardCompatibility(t *testing.T) {
	db := setupTestDB(t)

	// Create a single-credential provider
	provider := models.DNSProvider{
		ProviderType:        "cloudflare",
		UseMultiCredentials: false,
		CredentialsEncrypted: encryptCredentials(t, map[string]string{
			"api_token": "test-single-token",
		}),
		PropagationTimeout: 60,
		Enabled:            true,
	}
	require.NoError(t, db.Create(&provider).Error)

	// Create a proxy host with wildcard domain
	host := models.ProxyHost{
		DomainNames:   "*.example.com",
		DNSProviderID: &provider.ID,
		ForwardHost:   "localhost",
		ForwardPort:   8080,
		Enabled:       true,
	}
	require.NoError(t, db.Create(&host).Error)

	// Create ACME email setting
	setting := models.Setting{
		Key:   "caddy.acme_email",
		Value: "test@example.com",
	}
	require.NoError(t, db.Create(&setting).Error)

	// Create manager with mock client
	mockClient := &MockClient{}
	manager := NewManager(mockClient, db, t.TempDir(), "", false, config.SecurityConfig{})

	// Apply config
	err := manager.ApplyConfig(context.Background())
	require.NoError(t, err)

	// Verify the generated config has DNS challenge with single credential
	assert.True(t, mockClient.LoadCalled, "Load should have been called")
	assert.NotNil(t, mockClient.LastLoadedConfig, "Config should have been loaded")

	// Verify TLS automation policies exist
	require.NotNil(t, mockClient.LastLoadedConfig.Apps.TLS)
	require.NotNil(t, mockClient.LastLoadedConfig.Apps.TLS.Automation)
	require.Greater(t, len(mockClient.LastLoadedConfig.Apps.TLS.Automation.Policies), 0)

	// Find the DNS challenge policy
	var dnsPolicy *AutomationPolicy
	for _, policy := range mockClient.LastLoadedConfig.Apps.TLS.Automation.Policies {
		if len(policy.Subjects) > 0 && policy.Subjects[0] == "*.example.com" {
			dnsPolicy = policy
			break
		}
	}
	require.NotNil(t, dnsPolicy, "DNS challenge policy should exist for *.example.com")

	// Verify it uses the single credential
	require.Greater(t, len(dnsPolicy.IssuersRaw), 0)
	issuer := dnsPolicy.IssuersRaw[0].(map[string]any)
	require.NotNil(t, issuer["challenges"])
	challenges := issuer["challenges"].(map[string]any)
	require.NotNil(t, challenges["dns"])
	dnsChallenge := challenges["dns"].(map[string]any)
	require.NotNil(t, dnsChallenge["provider"])
	providerConfig := dnsChallenge["provider"].(map[string]any)

	assert.Equal(t, "cloudflare", providerConfig["name"])
	assert.Equal(t, "test-single-token", providerConfig["api_token"])
}

// TestApplyConfig_MultiCredential_ExactMatch tests that multi-credential providers
// correctly match credentials by exact zone match
func TestApplyConfig_MultiCredential_ExactMatch(t *testing.T) {
	db := setupTestDB(t)

	// Create a multi-credential provider
	provider := models.DNSProvider{
		ProviderType:        "cloudflare",
		UseMultiCredentials: true,
		PropagationTimeout:  60,
		Enabled:             true,
	}
	require.NoError(t, db.Create(&provider).Error)

	// Create zone-specific credentials
	exampleComCred := models.DNSProviderCredential{
		UUID:          uuid.New().String(),
		DNSProviderID: provider.ID,
		Label:         "Example.com Credential",
		ZoneFilter:    "example.com",
		CredentialsEncrypted: encryptCredentials(t, map[string]string{
			"api_token": "token-example-com",
		}),
		Enabled: true,
	}
	require.NoError(t, db.Create(&exampleComCred).Error)

	exampleOrgCred := models.DNSProviderCredential{
		UUID:          uuid.New().String(),
		DNSProviderID: provider.ID,
		Label:         "Example.org Credential",
		ZoneFilter:    "example.org",
		CredentialsEncrypted: encryptCredentials(t, map[string]string{
			"api_token": "token-example-org",
		}),
		Enabled: true,
	}
	require.NoError(t, db.Create(&exampleOrgCred).Error)

	// Create proxy hosts for different domains
	hostCom := models.ProxyHost{
		UUID:          uuid.New().String(),
		DomainNames:   "*.example.com",
		DNSProviderID: &provider.ID,
		ForwardHost:   "localhost",
		ForwardPort:   8080,
		Enabled:       true,
	}
	require.NoError(t, db.Create(&hostCom).Error)

	hostOrg := models.ProxyHost{
		UUID:          uuid.New().String(),
		DomainNames:   "*.example.org",
		DNSProviderID: &provider.ID,
		ForwardHost:   "localhost",
		ForwardPort:   8081,
		Enabled:       true,
	}
	require.NoError(t, db.Create(&hostOrg).Error)

	// Create ACME email setting
	setting := models.Setting{
		Key:   "caddy.acme_email",
		Value: "test@example.com",
	}
	require.NoError(t, db.Create(&setting).Error)

	// Create manager with mock client
	mockClient := &MockClient{}
	manager := NewManager(mockClient, db, t.TempDir(), "", false, config.SecurityConfig{})

	// Apply config
	err := manager.ApplyConfig(context.Background())
	require.NoError(t, err)

	// Verify the generated config has separate DNS challenge policies
	assert.True(t, mockClient.LoadCalled)
	require.NotNil(t, mockClient.LastLoadedConfig.Apps.TLS)
	require.NotNil(t, mockClient.LastLoadedConfig.Apps.TLS.Automation)

	policies := mockClient.LastLoadedConfig.Apps.TLS.Automation.Policies
	require.Greater(t, len(policies), 1, "Should have separate policies for each domain")

	// Find policies for each domain
	var comPolicy, orgPolicy *AutomationPolicy
	for _, policy := range policies {
		if len(policy.Subjects) > 0 {
			switch policy.Subjects[0] {
			case "*.example.com":
				comPolicy = policy
			case "*.example.org":
				orgPolicy = policy
			}
		}
	}

	require.NotNil(t, comPolicy, "Policy for *.example.com should exist")
	require.NotNil(t, orgPolicy, "Policy for *.example.org should exist")

	// Verify each policy uses the correct credential
	assertDNSChallengeCredential(t, comPolicy, "cloudflare", "token-example-com")
	assertDNSChallengeCredential(t, orgPolicy, "cloudflare", "token-example-org")
}

// TestApplyConfig_MultiCredential_WildcardMatch tests wildcard zone matching
func TestApplyConfig_MultiCredential_WildcardMatch(t *testing.T) {
	db := setupTestDB(t)

	// Create a multi-credential provider
	provider := models.DNSProvider{
		ProviderType:        "cloudflare",
		UseMultiCredentials: true,
		PropagationTimeout:  60,
		Enabled:             true,
	}
	require.NoError(t, db.Create(&provider).Error)

	// Create wildcard credential for *.example.com (matches app.example.com, api.example.com, etc.)
	wildcardCred := models.DNSProviderCredential{
		UUID:          uuid.New().String(),
		DNSProviderID: provider.ID,
		Label:         "Wildcard Example.com",
		ZoneFilter:    "*.example.com",
		CredentialsEncrypted: encryptCredentials(t, map[string]string{
			"api_token": "token-wildcard",
		}),
		Enabled: true,
	}
	require.NoError(t, db.Create(&wildcardCred).Error)

	// Create proxy host for subdomain
	host := models.ProxyHost{
		UUID:          uuid.New().String(),
		DomainNames:   "*.app.example.com",
		DNSProviderID: &provider.ID,
		ForwardHost:   "localhost",
		ForwardPort:   8080,
		Enabled:       true,
	}
	require.NoError(t, db.Create(&host).Error)

	// Create ACME email setting
	setting := models.Setting{
		Key:   "caddy.acme_email",
		Value: "test@example.com",
	}
	require.NoError(t, db.Create(&setting).Error)

	// Create manager with mock client
	mockClient := &MockClient{}
	manager := NewManager(mockClient, db, t.TempDir(), "", false, config.SecurityConfig{})

	// Apply config
	err := manager.ApplyConfig(context.Background())
	require.NoError(t, err)

	// Verify config was generated
	assert.True(t, mockClient.LoadCalled)
	require.NotNil(t, mockClient.LastLoadedConfig.Apps.TLS)
	require.NotNil(t, mockClient.LastLoadedConfig.Apps.TLS.Automation)

	// Find the DNS challenge policy
	var dnsPolicy *AutomationPolicy
	for _, policy := range mockClient.LastLoadedConfig.Apps.TLS.Automation.Policies {
		if len(policy.Subjects) > 0 && policy.Subjects[0] == "*.app.example.com" {
			dnsPolicy = policy
			break
		}
	}
	require.NotNil(t, dnsPolicy, "DNS challenge policy should exist")

	// Verify it uses the wildcard credential
	assertDNSChallengeCredential(t, dnsPolicy, "cloudflare", "token-wildcard")
}

// TestApplyConfig_MultiCredential_CatchAll tests catch-all credential (empty zone_filter)
func TestApplyConfig_MultiCredential_CatchAll(t *testing.T) {
	db := setupTestDB(t)

	// Create a multi-credential provider
	provider := models.DNSProvider{
		ProviderType:        "cloudflare",
		UseMultiCredentials: true,
		PropagationTimeout:  60,
		Enabled:             true,
	}
	require.NoError(t, db.Create(&provider).Error)

	// Create catch-all credential (empty zone_filter)
	catchAllCred := models.DNSProviderCredential{
		UUID:          uuid.New().String(),
		DNSProviderID: provider.ID,
		Label:         "Catch-All",
		ZoneFilter:    "",
		CredentialsEncrypted: encryptCredentials(t, map[string]string{
			"api_token": "token-catch-all",
		}),
		Enabled: true,
	}
	require.NoError(t, db.Create(&catchAllCred).Error)

	// Create proxy host for a domain with no specific credential
	host := models.ProxyHost{
		UUID:          uuid.New().String(),
		DomainNames:   "*.random.net",
		DNSProviderID: &provider.ID,
		ForwardHost:   "localhost",
		ForwardPort:   8080,
		Enabled:       true,
	}
	require.NoError(t, db.Create(&host).Error)

	// Create ACME email setting
	setting := models.Setting{
		Key:   "caddy.acme_email",
		Value: "test@example.com",
	}
	require.NoError(t, db.Create(&setting).Error)

	// Create manager with mock client
	mockClient := &MockClient{}
	manager := NewManager(mockClient, db, t.TempDir(), "", false, config.SecurityConfig{})

	// Apply config
	err := manager.ApplyConfig(context.Background())
	require.NoError(t, err)

	// Verify config was generated
	assert.True(t, mockClient.LoadCalled)
	require.NotNil(t, mockClient.LastLoadedConfig.Apps.TLS)
	require.NotNil(t, mockClient.LastLoadedConfig.Apps.TLS.Automation)

	// Find the DNS challenge policy
	var dnsPolicy *AutomationPolicy
	for _, policy := range mockClient.LastLoadedConfig.Apps.TLS.Automation.Policies {
		if len(policy.Subjects) > 0 && policy.Subjects[0] == "*.random.net" {
			dnsPolicy = policy
			break
		}
	}
	require.NotNil(t, dnsPolicy, "DNS challenge policy should exist")

	// Verify it uses the catch-all credential
	assertDNSChallengeCredential(t, dnsPolicy, "cloudflare", "token-catch-all")
}

// assertDNSChallengeCredential is a helper to verify DNS challenge uses correct credentials
func assertDNSChallengeCredential(t *testing.T, policy *AutomationPolicy, providerType, expectedToken string) { //nolint:unparam // test helper; providerType may vary in future tests
	t.Helper()

	require.Greater(t, len(policy.IssuersRaw), 0, "Policy should have issuers")
	issuer := policy.IssuersRaw[0].(map[string]any)
	require.NotNil(t, issuer["challenges"], "Issuer should have challenges")
	challenges := issuer["challenges"].(map[string]any)
	require.NotNil(t, challenges["dns"], "Challenges should have DNS")
	dnsChallenge := challenges["dns"].(map[string]any)
	require.NotNil(t, dnsChallenge["provider"], "DNS challenge should have provider")
	providerConfig := dnsChallenge["provider"].(map[string]any)

	assert.Equal(t, providerType, providerConfig["name"], "Provider type should match")
	assert.Equal(t, expectedToken, providerConfig["api_token"], "API token should match")
}

// MockClient is a mock Caddy client for testing
type MockClient struct {
	LoadCalled       bool
	LastLoadedConfig *Config
	PingError        error
	LoadError        error
	GetConfigResult  *Config
	GetConfigError   error
}

func (m *MockClient) Load(ctx context.Context, config *Config) error {
	m.LoadCalled = true
	m.LastLoadedConfig = config
	return m.LoadError
}

func (m *MockClient) Ping(ctx context.Context) error {
	return m.PingError
}

func (m *MockClient) GetConfig(ctx context.Context) (*Config, error) {
	return m.GetConfigResult, m.GetConfigError
}
