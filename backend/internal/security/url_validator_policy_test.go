package security

import (
	"errors"
	"strings"
	"testing"

	"github.com/Wikid82/charon/backend/internal/network"
)

func TestValidateExternalURL_MetadataMessageChosenAfterPolicy(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{"http://169.254.169.254", "http://[::ffff:169.254.169.254]"} {
		_, err := ValidateExternalURL(raw, WithAllowHTTP(), WithAllowRFC1918())
		if err == nil {
			t.Fatalf("%s accepted", raw)
		}
		if !errors.Is(err, network.ErrBlockedAddress) {
			t.Errorf("%s: sentinel missing: %v", raw, err)
		}
		if !strings.Contains(err.Error(), "cloud metadata endpoints") {
			t.Errorf("%s: unexpected message %q", raw, err.Error())
		}
		if !strings.Contains(err.Error(), "169.x.x.x") {
			t.Errorf("%s: address not sanitized: %q", raw, err.Error())
		}
	}
	_, err := ValidateExternalURL("http://10.1.2.3", WithAllowHTTP())
	if err == nil || strings.Contains(err.Error(), "cloud metadata") || !strings.Contains(err.Error(), "private ip addresses") {
		t.Errorf("expected generic private-address message, got %v", err)
	}
}

func TestValidateExternalURL_AllowLocalhostPinned(t *testing.T) {
	t.Parallel()
	for _, raw := range []string{"http://localhost:8080", "http://127.0.0.1:8080", "http://[::1]:8080"} {
		if _, err := ValidateExternalURL(raw, WithAllowHTTP(), WithAllowLocalhost()); err != nil {
			t.Errorf("%s rejected: %v", raw, err)
		}
	}
	// Only the exact literal hosts are exempt: other loopback forms and names
	// that resolve to loopback stay rejected.
	for _, raw := range []string{"http://127.0.0.2:8080", "http://localhost.:8080"} {
		_, err := ValidateExternalURL(raw, WithAllowHTTP(), WithAllowLocalhost())
		if err == nil {
			t.Errorf("%s accepted", raw)
		} else if !errors.Is(err, network.ErrBlockedAddress) {
			t.Errorf("%s: sentinel missing: %v", raw, err)
		}
	}
}

func TestValidateExternalURL_OverlayAllowanceOption(t *testing.T) {
	t.Parallel()
	cfg := &ValidationConfig{}
	WithAllowCGNAT()(cfg)
	if !cfg.AllowCGNAT {
		t.Fatal("WithAllowCGNAT did not set AllowCGNAT")
	}

	if _, err := ValidateExternalURL("http://100.64.0.1", WithAllowHTTP()); !errors.Is(err, network.ErrBlockedAddress) {
		t.Errorf("shared-space literal accepted by default: %v", err)
	}
	if _, err := ValidateExternalURL("http://100.64.0.1", WithAllowHTTP(), WithAllowCGNAT()); err != nil {
		t.Errorf("shared-space literal rejected with the allowance: %v", err)
	}
	// The metadata alias is rejected in every combination.
	for _, opts := range [][]ValidationOption{{WithAllowHTTP()}, {WithAllowHTTP(), WithAllowCGNAT()}} {
		if _, err := ValidateExternalURL("http://100.100.100.200", opts...); err == nil {
			t.Error("metadata alias accepted")
		}
	}
}

func TestValidateExternalURL_ReservedRangesRejectedByDefault(t *testing.T) {
	t.Parallel()
	hosts := []string{
		"192.0.0.1", "198.18.0.1", "198.19.255.255",
		"[64:ff9b::808:808]", "[64:ff9b:1::1]", "[2002:c000:204::1]", "[::1.2.3.4]",
		"[2001:0:4136:e378:8000:63bf:3fff:fdd2]", "[::ffff:100.64.0.1]", "100.127.255.255",
	}
	allowances := [][]ValidationOption{
		{WithAllowHTTP()},
		{WithAllowHTTP(), WithAllowRFC1918(), WithAllowCGNAT(), WithAllowLocalhost()},
	}
	for _, h := range hosts {
		for i, opts := range allowances {
			if h == "[::ffff:100.64.0.1]" || h == "100.127.255.255" {
				if i == 1 {
					continue // shared space is intentionally allowed with the opt-in
				}
			}
			_, err := ValidateExternalURL("http://"+h+":8080", opts...)
			if !errors.Is(err, network.ErrBlockedAddress) {
				t.Errorf("%s (allowance set %d): expected policy rejection, got %v", h, i, err)
			}
		}
	}
	for _, h := range []string{"100.63.255.255", "100.128.0.0", "192.0.1.1", "198.20.0.1"} {
		if _, err := ValidateExternalURL("http://"+h+":8080", WithAllowHTTP()); err != nil {
			t.Errorf("%s rejected: %v", h, err)
		}
	}
}
