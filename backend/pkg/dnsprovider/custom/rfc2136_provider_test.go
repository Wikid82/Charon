package custom

import (
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/pkg/dnsprovider"
)

func TestNewRFC2136Provider(t *testing.T) {
	provider := NewRFC2136Provider()

	if provider == nil {
		t.Fatal("NewRFC2136Provider() returned nil")
		return
	}

	if provider.propagationTimeout != RFC2136DefaultPropagationTimeout {
		t.Errorf("propagationTimeout = %v, want %v", provider.propagationTimeout, RFC2136DefaultPropagationTimeout)
	}

	if provider.pollingInterval != RFC2136DefaultPollingInterval {
		t.Errorf("pollingInterval = %v, want %v", provider.pollingInterval, RFC2136DefaultPollingInterval)
	}
}

func TestRFC2136Provider_Type(t *testing.T) {
	provider := NewRFC2136Provider()

	if got := provider.Type(); got != "rfc2136" {
		t.Errorf("Type() = %q, want %q", got, "rfc2136")
	}
}

func TestRFC2136Provider_Metadata(t *testing.T) {
	provider := NewRFC2136Provider()
	metadata := provider.Metadata()

	if metadata.Type != "rfc2136" {
		t.Errorf("Metadata().Type = %q, want %q", metadata.Type, "rfc2136")
	}

	if metadata.Name != "RFC 2136 (Dynamic DNS)" {
		t.Errorf("Metadata().Name = %q, want %q", metadata.Name, "RFC 2136 (Dynamic DNS)")
	}

	if metadata.IsBuiltIn {
		t.Error("Metadata().IsBuiltIn = true, want false")
	}

	if metadata.Version != "1.0.0" {
		t.Errorf("Metadata().Version = %q, want %q", metadata.Version, "1.0.0")
	}

	if metadata.InterfaceVersion != dnsprovider.InterfaceVersion {
		t.Errorf("Metadata().InterfaceVersion = %q, want %q", metadata.InterfaceVersion, dnsprovider.InterfaceVersion)
	}

	if metadata.DocumentationURL == "" {
		t.Error("Metadata().DocumentationURL is empty")
	}

	if metadata.Description == "" {
		t.Error("Metadata().Description is empty")
	}
}

func TestRFC2136Provider_InitAndCleanup(t *testing.T) {
	provider := NewRFC2136Provider()

	if err := provider.Init(); err != nil {
		t.Errorf("Init() returned error: %v", err)
	}

	if err := provider.Cleanup(); err != nil {
		t.Errorf("Cleanup() returned error: %v", err)
	}
}

func TestRFC2136Provider_RequiredCredentialFields(t *testing.T) {
	provider := NewRFC2136Provider()
	fields := provider.RequiredCredentialFields()

	expectedFields := map[string]bool{
		"nameserver":      false,
		"tsig_key_name":   false,
		"tsig_key_secret": false,
	}

	if len(fields) != len(expectedFields) {
		t.Errorf("RequiredCredentialFields() returned %d fields, want %d", len(fields), len(expectedFields))
	}

	for _, field := range fields {
		if _, ok := expectedFields[field.Name]; !ok {
			t.Errorf("Unexpected required field: %q", field.Name)
		}
		expectedFields[field.Name] = true

		if field.Label == "" {
			t.Errorf("Field %q has empty label", field.Name)
		}
		if field.Type == "" {
			t.Errorf("Field %q has empty type", field.Name)
		}
	}

	for name, found := range expectedFields {
		if !found {
			t.Errorf("Missing required field: %q", name)
		}
	}
}

func TestRFC2136Provider_OptionalCredentialFields(t *testing.T) {
	provider := NewRFC2136Provider()
	fields := provider.OptionalCredentialFields()

	expectedFields := map[string]bool{
		"port":           false,
		"tsig_algorithm": false,
		"zone":           false,
	}

	if len(fields) != len(expectedFields) {
		t.Errorf("OptionalCredentialFields() returned %d fields, want %d", len(fields), len(expectedFields))
	}

	for _, field := range fields {
		if _, ok := expectedFields[field.Name]; !ok {
			t.Errorf("Unexpected optional field: %q", field.Name)
		}
		expectedFields[field.Name] = true

		if field.Label == "" {
			t.Errorf("Field %q has empty label", field.Name)
		}

		// Verify tsig_algorithm has select options
		if field.Name == "tsig_algorithm" {
			if field.Type != "select" {
				t.Errorf("tsig_algorithm type = %q, want %q", field.Type, "select")
			}
			if len(field.Options) == 0 {
				t.Error("tsig_algorithm has no select options")
			}

			// Verify all valid algorithms are present
			optionValues := make(map[string]bool)
			for _, opt := range field.Options {
				optionValues[opt.Value] = true
			}
			for alg := range ValidTSIGAlgorithms {
				if !optionValues[alg] {
					t.Errorf("Missing algorithm option: %q", alg)
				}
			}
		}
	}

	for name, found := range expectedFields {
		if !found {
			t.Errorf("Missing optional field: %q", name)
		}
	}
}

func TestRFC2136Provider_ValidateCredentials(t *testing.T) {
	provider := NewRFC2136Provider()

	// Valid base64 secret (example)
	// #nosec G101 -- Test fixture with non-functional credential for validation testing
	validSecret := "c2VjcmV0a2V5MTIzNDU2Nzg5MA==" // "secretkey1234567890" in base64

	tests := []struct {
		name    string
		creds   map[string]string
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid credentials with defaults",
			creds: map[string]string{
				"nameserver":      "ns1.example.com",
				"tsig_key_name":   "acme-key.example.com",
				"tsig_key_secret": validSecret,
			},
			wantErr: false,
		},
		{
			name: "valid credentials with all fields",
			creds: map[string]string{
				"nameserver":      "ns1.example.com",
				"tsig_key_name":   "acme-key.example.com",
				"tsig_key_secret": validSecret,
				"port":            "5353",
				"tsig_algorithm":  "hmac-sha512",
				"zone":            "example.com",
			},
			wantErr: false,
		},
		{
			name: "valid credentials with uppercase algorithm",
			creds: map[string]string{
				"nameserver":      "ns1.example.com",
				"tsig_key_name":   "acme-key.example.com",
				"tsig_key_secret": validSecret,
				"tsig_algorithm":  "HMAC-SHA256",
			},
			wantErr: false,
		},
		{
			name: "valid with IP address nameserver",
			creds: map[string]string{
				"nameserver":      "192.168.1.1",
				"tsig_key_name":   "acme-key",
				"tsig_key_secret": validSecret,
			},
			wantErr: false,
		},
		{
			name: "valid with whitespace trimming",
			creds: map[string]string{
				"nameserver":      "  ns1.example.com  ",
				"tsig_key_name":   "  acme-key  ",
				"tsig_key_secret": "  " + validSecret + "  ",
			},
			wantErr: false,
		},
		{
			name: "missing nameserver",
			creds: map[string]string{
				"tsig_key_name":   "acme-key",
				"tsig_key_secret": validSecret,
			},
			wantErr: true,
			errMsg:  "nameserver is required",
		},
		{
			name: "empty nameserver",
			creds: map[string]string{
				"nameserver":      "",
				"tsig_key_name":   "acme-key",
				"tsig_key_secret": validSecret,
			},
			wantErr: true,
			errMsg:  "nameserver is required",
		},
		{
			name: "whitespace-only nameserver",
			creds: map[string]string{
				"nameserver":      "   ",
				"tsig_key_name":   "acme-key",
				"tsig_key_secret": validSecret,
			},
			wantErr: true,
			errMsg:  "nameserver is required",
		},
		{
			name: "missing tsig_key_name",
			creds: map[string]string{
				"nameserver":      "ns1.example.com",
				"tsig_key_secret": validSecret,
			},
			wantErr: true,
			errMsg:  "tsig_key_name is required",
		},
		{
			name: "empty tsig_key_name",
			creds: map[string]string{
				"nameserver":      "ns1.example.com",
				"tsig_key_name":   "",
				"tsig_key_secret": validSecret,
			},
			wantErr: true,
			errMsg:  "tsig_key_name is required",
		},
		{
			name: "missing tsig_key_secret",
			creds: map[string]string{
				"nameserver":    "ns1.example.com",
				"tsig_key_name": "acme-key",
			},
			wantErr: true,
			errMsg:  "tsig_key_secret is required",
		},
		{
			name: "empty tsig_key_secret",
			creds: map[string]string{
				"nameserver":      "ns1.example.com",
				"tsig_key_name":   "acme-key",
				"tsig_key_secret": "",
			},
			wantErr: true,
			errMsg:  "tsig_key_secret is required",
		},
		{
			name: "invalid base64 secret",
			creds: map[string]string{ //nolint:gosec // test fixture with intentionally invalid base64
				"nameserver":      "ns1.example.com",
				"tsig_key_name":   "acme-key",
				"tsig_key_secret": "not-valid-base64!!!",
			},
			wantErr: true,
			errMsg:  "tsig_key_secret must be valid base64",
		},
		{
			name: "invalid port - not a number",
			creds: map[string]string{
				"nameserver":      "ns1.example.com",
				"tsig_key_name":   "acme-key",
				"tsig_key_secret": validSecret,
				"port":            "abc",
			},
			wantErr: true,
			errMsg:  "port must be a number",
		},
		{
			name: "invalid port - too low",
			creds: map[string]string{
				"nameserver":      "ns1.example.com",
				"tsig_key_name":   "acme-key",
				"tsig_key_secret": validSecret,
				"port":            "0",
			},
			wantErr: true,
			errMsg:  "port must be between",
		},
		{
			name: "invalid port - too high",
			creds: map[string]string{
				"nameserver":      "ns1.example.com",
				"tsig_key_name":   "acme-key",
				"tsig_key_secret": validSecret,
				"port":            "65536",
			},
			wantErr: true,
			errMsg:  "port must be between",
		},
		{
			name: "invalid algorithm",
			creds: map[string]string{
				"nameserver":      "ns1.example.com",
				"tsig_key_name":   "acme-key",
				"tsig_key_secret": validSecret,
				"tsig_algorithm":  "invalid-algorithm",
			},
			wantErr: true,
			errMsg:  "tsig_algorithm must be one of",
		},
		{
			name:    "all empty credentials",
			creds:   map[string]string{},
			wantErr: true,
			errMsg:  "nameserver is required",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := provider.ValidateCredentials(tt.creds)

			if tt.wantErr {
				if err == nil {
					t.Error("ValidateCredentials() expected error but got nil")
					return
				}
				if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateCredentials() error = %q, want to contain %q", err.Error(), tt.errMsg)
				}
			} else if err != nil {
				t.Errorf("ValidateCredentials() unexpected error: %v", err)
			}
		})
	}
}

func TestRFC2136Provider_TestCredentials(t *testing.T) {
	provider := NewRFC2136Provider()

	// #nosec G101 -- Test fixture with non-functional credential for validation testing
	validSecret := "c2VjcmV0a2V5MTIzNDU2Nzg5MA=="

	// TestCredentials should behave the same as ValidateCredentials
	validCreds := map[string]string{
		"nameserver":      "ns1.example.com",
		"tsig_key_name":   "acme-key",
		"tsig_key_secret": validSecret,
	}

	if err := provider.TestCredentials(validCreds); err != nil {
		t.Errorf("TestCredentials() with valid creds returned error: %v", err)
	}

	invalidCreds := map[string]string{
		"nameserver": "ns1.example.com",
	}

	if err := provider.TestCredentials(invalidCreds); err == nil {
		t.Error("TestCredentials() with invalid creds expected error but got nil")
	}
}

func TestRFC2136Provider_SupportsMultiCredential(t *testing.T) {
	provider := NewRFC2136Provider()

	if !provider.SupportsMultiCredential() {
		t.Error("SupportsMultiCredential() = false, want true")
	}
}

func TestRFC2136Provider_BuildCaddyConfig(t *testing.T) {
	provider := NewRFC2136Provider()

	// #nosec G101 -- Test fixture with non-functional credential for validation testing
	validSecret := "c2VjcmV0a2V5MTIzNDU2Nzg5MA=="

	tests := []struct {
		name     string
		creds    map[string]string
		expected map[string]any
	}{
		{
			name: "minimal config with defaults",
			creds: map[string]string{
				"nameserver":      "ns1.example.com",
				"tsig_key_name":   "acme-key",
				"tsig_key_secret": validSecret,
			},
			expected: map[string]any{
				"name":            "rfc2136",
				"nameserver":      "ns1.example.com",
				"tsig_key_name":   "acme-key",
				"tsig_key_secret": validSecret,
				"port":            "53",
				"tsig_algorithm":  "hmac-sha256",
			},
		},
		{
			name: "full config with all options",
			creds: map[string]string{
				"nameserver":      "ns1.example.com",
				"tsig_key_name":   "acme-key",
				"tsig_key_secret": validSecret,
				"port":            "5353",
				"tsig_algorithm":  "hmac-sha512",
				"zone":            "example.com",
			},
			expected: map[string]any{
				"name":            "rfc2136",
				"nameserver":      "ns1.example.com",
				"tsig_key_name":   "acme-key",
				"tsig_key_secret": validSecret,
				"port":            "5353",
				"tsig_algorithm":  "hmac-sha512",
				"zone":            "example.com",
			},
		},
		{
			name: "algorithm normalization to lowercase",
			creds: map[string]string{
				"nameserver":      "ns1.example.com",
				"tsig_key_name":   "acme-key",
				"tsig_key_secret": validSecret,
				"tsig_algorithm":  "HMAC-SHA384",
			},
			expected: map[string]any{
				"name":            "rfc2136",
				"nameserver":      "ns1.example.com",
				"tsig_key_name":   "acme-key",
				"tsig_key_secret": validSecret,
				"port":            "53",
				"tsig_algorithm":  "hmac-sha384",
			},
		},
		{
			name: "whitespace trimming",
			creds: map[string]string{
				"nameserver":      "  ns1.example.com  ",
				"tsig_key_name":   "  acme-key  ",
				"tsig_key_secret": "  " + validSecret + "  ",
				"port":            "  5353  ",
				"zone":            "  example.com  ",
			},
			expected: map[string]any{
				"name":            "rfc2136",
				"nameserver":      "ns1.example.com",
				"tsig_key_name":   "acme-key",
				"tsig_key_secret": validSecret,
				"port":            "5353",
				"tsig_algorithm":  "hmac-sha256",
				"zone":            "example.com",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := provider.BuildCaddyConfig(tt.creds)

			for key, expectedValue := range tt.expected {
				actualValue, ok := config[key]
				if !ok {
					t.Errorf("BuildCaddyConfig() missing key %q", key)
					continue
				}
				if actualValue != expectedValue {
					t.Errorf("BuildCaddyConfig()[%q] = %v, want %v", key, actualValue, expectedValue)
				}
			}

			// Check no unexpected keys (except for zone which is optional)
			for key := range config {
				if _, ok := tt.expected[key]; !ok {
					t.Errorf("BuildCaddyConfig() unexpected key %q", key)
				}
			}
		})
	}
}

func TestRFC2136Provider_BuildCaddyConfigForZone(t *testing.T) {
	provider := NewRFC2136Provider()

	// #nosec G101 -- Test fixture for RFC2136 provider testing, not a real credential
	validSecret := "c2VjcmV0a2V5MTIzNDU2Nzg5MA=="

	tests := []struct {
		name         string
		baseDomain   string
		creds        map[string]string
		expectedZone string
	}{
		{
			name:       "zone auto-set from baseDomain",
			baseDomain: "example.org",
			creds: map[string]string{
				"nameserver":      "ns1.example.com",
				"tsig_key_name":   "acme-key",
				"tsig_key_secret": validSecret,
			},
			expectedZone: "example.org",
		},
		{
			name:       "explicit zone takes precedence",
			baseDomain: "example.org",
			creds: map[string]string{
				"nameserver":      "ns1.example.com",
				"tsig_key_name":   "acme-key",
				"tsig_key_secret": validSecret,
				"zone":            "custom.zone.com",
			},
			expectedZone: "custom.zone.com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := provider.BuildCaddyConfigForZone(tt.baseDomain, tt.creds)

			zone, ok := config["zone"]
			if !ok {
				t.Error("BuildCaddyConfigForZone() missing 'zone' key")
				return
			}
			if zone != tt.expectedZone {
				t.Errorf("BuildCaddyConfigForZone() zone = %v, want %v", zone, tt.expectedZone)
			}
		})
	}
}

func TestRFC2136Provider_PropagationTimeout(t *testing.T) {
	provider := NewRFC2136Provider()

	timeout := provider.PropagationTimeout()

	if timeout != 60*time.Second {
		t.Errorf("PropagationTimeout() = %v, want %v", timeout, 60*time.Second)
	}
}

func TestRFC2136Provider_PollingInterval(t *testing.T) {
	provider := NewRFC2136Provider()

	interval := provider.PollingInterval()

	if interval != 2*time.Second {
		t.Errorf("PollingInterval() = %v, want %v", interval, 2*time.Second)
	}
}

func TestRFC2136Provider_GetPort(t *testing.T) {
	provider := NewRFC2136Provider()

	tests := []struct {
		name     string
		creds    map[string]string
		expected string
	}{
		{
			name:     "default port when not set",
			creds:    map[string]string{},
			expected: "53",
		},
		{
			name:     "default port when empty",
			creds:    map[string]string{"port": ""},
			expected: "53",
		},
		{
			name:     "custom port",
			creds:    map[string]string{"port": "5353"},
			expected: "5353",
		},
		{
			name:     "custom port with whitespace",
			creds:    map[string]string{"port": "  5353  "},
			expected: "5353",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			port := provider.GetPort(tt.creds)
			if port != tt.expected {
				t.Errorf("GetPort() = %q, want %q", port, tt.expected)
			}
		})
	}
}

func TestRFC2136Provider_GetAlgorithm(t *testing.T) {
	provider := NewRFC2136Provider()

	tests := []struct {
		name     string
		creds    map[string]string
		expected string
	}{
		{
			name:     "default algorithm when not set",
			creds:    map[string]string{},
			expected: "hmac-sha256",
		},
		{
			name:     "default algorithm when empty",
			creds:    map[string]string{"tsig_algorithm": ""},
			expected: "hmac-sha256",
		},
		{
			name:     "custom algorithm",
			creds:    map[string]string{"tsig_algorithm": "hmac-sha512"},
			expected: "hmac-sha512",
		},
		{
			name:     "uppercase algorithm normalized",
			creds:    map[string]string{"tsig_algorithm": "HMAC-SHA384"},
			expected: "hmac-sha384",
		},
		{
			name:     "algorithm with whitespace",
			creds:    map[string]string{"tsig_algorithm": "  hmac-sha1  "},
			expected: "hmac-sha1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			algorithm := provider.GetAlgorithm(tt.creds)
			if algorithm != tt.expected {
				t.Errorf("GetAlgorithm() = %q, want %q", algorithm, tt.expected)
			}
		})
	}
}

func TestRFC2136Provider_ValidTSIGAlgorithms(t *testing.T) {
	expectedAlgorithms := []string{
		"hmac-sha256",
		"hmac-sha384",
		"hmac-sha512",
		"hmac-sha1",
		"hmac-md5",
	}

	for _, alg := range expectedAlgorithms {
		if !ValidTSIGAlgorithms[alg] {
			t.Errorf("ValidTSIGAlgorithms missing %q", alg)
		}
	}

	if len(ValidTSIGAlgorithms) != len(expectedAlgorithms) {
		t.Errorf("ValidTSIGAlgorithms has %d entries, want %d", len(ValidTSIGAlgorithms), len(expectedAlgorithms))
	}
}

func TestRFC2136Provider_ImplementsInterface(t *testing.T) {
	provider := NewRFC2136Provider()

	// Compile-time interface check
	var _ dnsprovider.ProviderPlugin = provider
}

// Helper function to check if string contains substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
