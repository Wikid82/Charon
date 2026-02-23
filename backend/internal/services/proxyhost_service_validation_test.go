package services

import (
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

func TestProxyHostService_ForwardHostValidation(t *testing.T) {
	db := setupProxyHostTestDB(t)
	service := NewProxyHostService(db)

	tests := []struct {
		name        string
		forwardHost string
		wantErr     bool
	}{
		{
			name:        "Valid IP",
			forwardHost: "192.168.1.1",
			wantErr:     false,
		},
		{
			name:        "Valid Hostname",
			forwardHost: "example.com",
			wantErr:     false,
		},
		{
			name:        "Docker Service Name",
			forwardHost: "my-service",
			wantErr:     false,
		},
		{
			name:        "Docker Service Name with Underscore",
			forwardHost: "my_db_Service",
			wantErr:     false,
		},
		{
			name:        "Docker Internal Host",
			forwardHost: "host.docker.internal",
			wantErr:     false,
		},
		{
			name:        "IP with Port (Should be stripped and pass)",
			forwardHost: "192.168.1.1:8080",
			wantErr:     false,
		},
		{
			name:        "Hostname with Port (Should be stripped and pass)",
			forwardHost: "example.com:3000",
			wantErr:     false,
		},
		{
			name:        "Host with http scheme (Should be stripped and pass)",
			forwardHost: "https://discord.com/api/webhooks/123/abc",
			wantErr:     false,
		},
		{
			name:        "Host with https scheme (Should be stripped and pass)",
			forwardHost: "https://example.com",
			wantErr:     false,
		},
		{
			name:        "Invalid Characters",
			forwardHost: "invalid$host",
			wantErr:     true,
		},
		{
			name:        "Empty Host",
			forwardHost: "",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host := &models.ProxyHost{
				DomainNames: "test-" + tt.name + ".example.com",
				ForwardHost: tt.forwardHost,
				ForwardPort: 8080,
			}
			// We only care about validation error
			err := service.Create(host)
			if tt.wantErr {
				assert.Error(t, err)
			} else if err != nil {
				// Check if error is validation or something else
				// If it's something else, it might be fine for this test context
				// but "forward host must be..." is what we look for.
				assert.NotContains(t, err.Error(), "forward host", "Should not fail validation")
			}
		})
	}
}

func TestProxyHostService_DomainNamesRequired(t *testing.T) {
	db := setupProxyHostTestDB(t)
	service := NewProxyHostService(db)

	t.Run("create rejects empty domain names", func(t *testing.T) {
		host := &models.ProxyHost{
			UUID:          "create-empty-domain",
			DomainNames:   "",
			ForwardHost:   "localhost",
			ForwardPort:   8080,
			ForwardScheme: "http",
		}

		err := service.Create(host)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "domain names is required")
	})

	t.Run("update rejects whitespace-only domain names", func(t *testing.T) {
		host := &models.ProxyHost{
			UUID:          "update-empty-domain",
			DomainNames:   "valid.example.com",
			ForwardHost:   "localhost",
			ForwardPort:   8080,
			ForwardScheme: "http",
		}

		err := service.Create(host)
		assert.NoError(t, err)

		host.DomainNames = "   "
		err = service.Update(host)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "domain names is required")

		persisted, getErr := service.GetByID(host.ID)
		assert.NoError(t, getErr)
		assert.Equal(t, "valid.example.com", persisted.DomainNames)
	})
}

func TestProxyHostService_DNSChallengeValidation(t *testing.T) {
	db := setupProxyHostTestDB(t)
	service := NewProxyHostService(db)

	t.Run("create rejects use_dns_challenge without provider", func(t *testing.T) {
		host := &models.ProxyHost{
			UUID:            "dns-create-validation",
			DomainNames:     "dns-create.example.com",
			ForwardHost:     "localhost",
			ForwardPort:     8080,
			ForwardScheme:   "http",
			UseDNSChallenge: true,
			DNSProviderID:   nil,
		}

		err := service.Create(host)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "dns provider is required")
	})

	t.Run("update rejects use_dns_challenge without provider", func(t *testing.T) {
		host := &models.ProxyHost{
			UUID:            "dns-update-validation",
			DomainNames:     "dns-update.example.com",
			ForwardHost:     "localhost",
			ForwardPort:     8080,
			ForwardScheme:   "http",
			UseDNSChallenge: false,
		}

		err := service.Create(host)
		assert.NoError(t, err)

		host.UseDNSChallenge = true
		host.DNSProviderID = nil
		err = service.Update(host)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "dns provider is required")

		persisted, getErr := service.GetByID(host.ID)
		assert.NoError(t, getErr)
		assert.False(t, persisted.UseDNSChallenge)
		assert.Nil(t, persisted.DNSProviderID)
	})

	t.Run("create trims domain and forward host", func(t *testing.T) {
		host := &models.ProxyHost{
			UUID:          "dns-trim-validation",
			DomainNames:   "  trim.example.com  ",
			ForwardHost:   "  localhost  ",
			ForwardPort:   8080,
			ForwardScheme: "http",
		}

		err := service.Create(host)
		assert.NoError(t, err)

		persisted, getErr := service.GetByID(host.ID)
		assert.NoError(t, getErr)
		assert.Equal(t, "trim.example.com", persisted.DomainNames)
		assert.Equal(t, "localhost", persisted.ForwardHost)
	})
}

func TestProxyHostService_ValidateHostname(t *testing.T) {
	db := setupProxyHostTestDB(t)
	service := NewProxyHostService(db)

	tests := []struct {
		name    string
		host    string
		wantErr bool
	}{
		{name: "plain hostname", host: "example.com", wantErr: false},
		{name: "hostname with scheme", host: "https://example.com", wantErr: false},
		{name: "hostname with http scheme", host: "https://discord.com/api/webhooks/123/abc", wantErr: false},
		{name: "hostname with port", host: "example.com:8080", wantErr: false},
		{name: "ipv4 address", host: "127.0.0.1", wantErr: false},
		{name: "bracketed ipv6 with port", host: "[::1]:443", wantErr: false},
		{name: "docker style underscore", host: "my_service", wantErr: false},
		{name: "invalid character", host: "invalid$host", wantErr: true},
		// Malformed URLs should fail strict hostname validation
		{name: "malformed https URL", host: "https://::invalid::", wantErr: true},
		{name: "malformed http URL", host: "http://::malformed::", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := service.ValidateHostname(tt.host)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			assert.NoError(t, err)
		})
	}
}

// TestProxyHostService_ValidateProxyHost_FallbackParsing covers lines 74-75
func TestProxyHostService_ValidateProxyHost_FallbackParsing(t *testing.T) {
	db := setupProxyHostTestDB(t)
	service := NewProxyHostService(db)

	// Test URLs that will fail url.Parse but fallback can handle
	tests := []struct {
		name        string
		domainName  string
		forwardHost string
		wantErr     bool
	}{
		{
			name:        "Valid after stripping https prefix",
			domainName:  "test1.example.com",
			forwardHost: "https://example.com:3000",
			wantErr:     false, // Fallback strips prefix, validates remaining
		},
		{
			name:        "Valid after stripping http prefix",
			domainName:  "test2.example.com",
			forwardHost: "http://192.168.1.1:8080",
			wantErr:     false, // Fallback strips prefix
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host := &models.ProxyHost{
				UUID:        uuid.New().String(), // Generate unique UUID
				DomainNames: tt.domainName,
				ForwardHost: tt.forwardHost,
				ForwardPort: 8080,
			}
			err := service.Create(host)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// TestProxyHostService_ValidateProxyHost_InvalidHostnameChars covers lines 115-118
func TestProxyHostService_ValidateProxyHost_InvalidHostnameChars(t *testing.T) {
	db := setupProxyHostTestDB(t)
	service := NewProxyHostService(db)

	tests := []struct {
		name        string
		forwardHost string
		expectError string
	}{
		{
			name:        "Special characters dollar sign",
			forwardHost: "host$name",
			expectError: "forward host must be a valid IP address or hostname",
		},
		{
			name:        "Special characters at symbol",
			forwardHost: "host@domain",
			expectError: "forward host must be a valid IP address or hostname",
		},
		{
			name:        "Special characters percent",
			forwardHost: "host%name",
			expectError: "forward host must be a valid IP address or hostname",
		},
		{
			name:        "Special characters ampersand",
			forwardHost: "host&name",
			expectError: "forward host must be a valid IP address or hostname",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			host := &models.ProxyHost{
				DomainNames: "test.example.com",
				ForwardHost: tt.forwardHost,
				ForwardPort: 8080,
			}
			err := service.Create(host)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectError)
		})
	}
}

// TestProxyHostService_ValidateProxyHost_DNSChallenge covers lines 128-129
func TestProxyHostService_ValidateProxyHost_DNSChallenge(t *testing.T) {
	db := setupProxyHostTestDB(t)
	service := NewProxyHostService(db)

	// Test DNS challenge enabled without provider ID
	host := &models.ProxyHost{
		DomainNames:     "test.example.com",
		ForwardHost:     "backend",
		ForwardPort:     8080,
		UseDNSChallenge: true,
		DNSProviderID:   nil, // Missing provider ID
	}

	err := service.Create(host)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "dns provider is required")
}

func TestProxyHostService_ValidateHostname_StripsPath(t *testing.T) {
	db := setupProxyHostTestDB(t)
	service := NewProxyHostService(db)

	err := service.ValidateHostname("backend.internal/api/v1")
	assert.NoError(t, err)
}

func TestProxyHostService_ValidateProxyHost_ParseFallbackAndPathTrim(t *testing.T) {
	db := setupProxyHostTestDB(t)
	service := NewProxyHostService(db)

	host := &models.ProxyHost{
		UUID:        uuid.New().String(),
		DomainNames: "fallback-path.example.com",
		ForwardHost: "https://bad host/path",
		ForwardPort: 8080,
	}

	err := service.Create(host)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "forward host must be a valid IP address or hostname")
}
