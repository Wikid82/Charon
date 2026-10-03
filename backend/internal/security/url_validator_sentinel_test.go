package security

import (
	"errors"
	"testing"

	"github.com/Wikid82/charon/backend/internal/network"
)

func TestValidateExternalURL_WrapsBlockedAddressSentinel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		url  string
	}{
		{"loopback literal", "http://127.0.0.1"},
		{"rfc1918 literal", "http://10.0.0.5"},
		{"metadata literal", "http://169.254.169.254"},
		{"ipv6 loopback", "http://[::1]"},
		{"mapped metadata", "http://[::ffff:169.254.169.254]"},
		{"mapped rfc1918", "http://[::ffff:192.168.1.1]"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := ValidateExternalURL(tt.url, WithAllowHTTP())
			if err == nil {
				t.Fatalf("expected %s to be rejected", tt.url)
			}
			if !errors.Is(err, network.ErrBlockedAddress) {
				t.Errorf("error does not wrap ErrBlockedAddress: %v", err)
			}
		})
	}
}

func TestValidateExternalURL_NonAddressFailuresDoNotWrapSentinel(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		url  string
	}{
		{"bad scheme", "ftp://example.com"},
		{"userinfo", "https://user:pw@example.com"},
		{"bad port", "https://example.com:70000"},
		{"http without opt-in", "http://example.com"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			_, err := ValidateExternalURL(tt.url)
			if err == nil {
				t.Fatalf("expected %s to be rejected", tt.url)
			}
			if errors.Is(err, network.ErrBlockedAddress) {
				t.Errorf("non-address failure must not wrap the sentinel: %v", err)
			}
		})
	}
}
