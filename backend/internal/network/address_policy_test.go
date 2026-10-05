package network

import (
	"context"
	"errors"
	"net"
	"testing"
	"time"
)

func TestAddressPolicy_Blocked(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		ip     string
		policy AddressPolicy
		want   bool
	}{
		{"nil", "", AddressPolicy{}, true},
		{"public", "8.8.8.8", AddressPolicy{}, false},
		{"loopback zero value", "127.0.0.1", AddressPolicy{}, true},
		{"loopback allowed", "::1", AddressPolicy{AllowLocalhost: true}, false},
		{"rfc1918 zero value", "10.0.0.1", AddressPolicy{}, true},
		{"rfc1918 allowed", "::ffff:10.0.0.1", AddressPolicy{AllowRFC1918: true}, false},
		{"link-local never opened", "169.254.169.254", AddressPolicy{AllowLocalhost: true, AllowRFC1918: true, AllowCGNAT: true}, true},
		{"shared space zero value", "100.64.0.1", AddressPolicy{}, true},
		{"shared space upper edge", "100.127.255.255", AddressPolicy{}, true},
		{"below shared space", "100.63.255.255", AddressPolicy{}, false},
		{"above shared space", "100.128.0.0", AddressPolicy{}, false},
		{"shared space mapped", "::ffff:100.64.0.1", AddressPolicy{}, true},
		{"shared space allowed", "100.64.0.1", AddressPolicy{AllowCGNAT: true}, false},
		{"metadata alias with allowance", "100.100.100.200", AddressPolicy{AllowCGNAT: true}, true},
		{"metadata alias mapped with allowance", "::ffff:100.100.100.200", AddressPolicy{AllowCGNAT: true, AllowRFC1918: true}, true},
		{"special-purpose v4", "192.0.0.1", AddressPolicy{}, true},
		{"special-purpose v4 neighbour", "192.0.1.1", AddressPolicy{}, false},
		{"benchmark range", "198.18.0.1", AddressPolicy{}, true},
		{"benchmark range upper", "198.19.255.255", AddressPolicy{}, true},
		{"benchmark range neighbour", "198.20.0.1", AddressPolicy{}, false},
		{"translation v6", "64:ff9b::808:808", AddressPolicy{}, true},
		{"translation v6 local", "64:ff9b:1::1", AddressPolicy{}, true},
		{"tunnel v6", "2002:c000:204::1", AddressPolicy{}, true},
		{"embedded v4 v6", "::1.2.3.4", AddressPolicy{}, true},
		{"tunnel v6 prefix", "2001:0:4136:e378:8000:63bf:3fff:fdd2", AddressPolicy{}, true},
		{"translation with every allowance", "64:ff9b::1", AddressPolicy{AllowLocalhost: true, AllowRFC1918: true, AllowCGNAT: true}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			var ip net.IP
			if tt.ip != "" {
				ip = net.ParseIP(tt.ip)
			}
			if got := tt.policy.Blocked(ip); got != tt.want {
				t.Errorf("Blocked(%q) = %v, want %v", tt.ip, got, tt.want)
			}
		})
	}
}

// TestClientOptionsPolicy_EndState covers the policy derived from client options.
func TestClientOptionsPolicy_EndState(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		opts ClientOptions
		ip   string
		want bool
	}{
		{"shared space default", ClientOptions{}, "100.64.0.1", true},
		{"shared space allowed", ClientOptions{AllowCGNAT: true}, "100.64.0.1", false},
		{"shared space with other allowances", ClientOptions{AllowLocalhost: true, AllowRFC1918: true}, "100.64.0.1", true},
		{"metadata alias with allowance", ClientOptions{AllowCGNAT: true}, "100.100.100.200", true},
		{"mapped metadata alias with allowance", ClientOptions{AllowCGNAT: true}, "::ffff:100.100.100.200", true},
		{"benchmark range default", ClientOptions{}, "198.18.0.1", true},
		{"benchmark range with every allowance", ClientOptions{AllowLocalhost: true, AllowRFC1918: true, AllowCGNAT: true}, "198.19.255.255", true},
		{"tunnel range with every allowance", ClientOptions{AllowLocalhost: true, AllowRFC1918: true, AllowCGNAT: true}, "2002::1", true},
		{"public with every allowance", ClientOptions{AllowLocalhost: true, AllowRFC1918: true, AllowCGNAT: true}, "8.8.8.8", false},
	}
	for _, tt := range tests {
		opts := tt.opts
		if got := opts.policy().Blocked(net.ParseIP(tt.ip)); got != tt.want {
			t.Errorf("%s: Blocked(%s) = %v, want %v", tt.name, tt.ip, got, tt.want)
		}
	}
}

func TestAllowOverlayOption_SetsField(t *testing.T) {
	t.Parallel()
	cfg := defaultOptions()
	WithAllowCGNAT()(&cfg)
	if !cfg.AllowCGNAT {
		t.Fatal("WithAllowCGNAT did not set AllowCGNAT")
	}
}

func TestNewSafeHTTPClient_OverlayAllowanceWiredToDialer(t *testing.T) {
	withResolver(t, map[string][]string{"overlay.example": {"100.64.0.9"}})

	blocked := safeDialer(&ClientOptions{DialTimeout: time.Second})
	if _, err := blocked(context.Background(), "tcp", "overlay.example:9"); !errors.Is(err, ErrBlockedAddress) {
		t.Fatalf("default dialer: expected ErrBlockedAddress, got %v", err)
	}

	allowed := safeDialer(&ClientOptions{DialTimeout: 200 * time.Millisecond, AllowCGNAT: true})
	conn, err := allowed(context.Background(), "tcp", "overlay.example:9")
	if conn != nil {
		_ = conn.Close()
	}
	if errors.Is(err, ErrBlockedAddress) {
		t.Fatalf("allowing dialer still refused the address: %v", err)
	}
}
