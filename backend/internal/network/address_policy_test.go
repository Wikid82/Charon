package network

import (
	"net"
	"testing"
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
		{"staging flag lifts translation block", "2002::1", AddressPolicy{AllowTransition: true}, false},
		{"staging flag keeps loopback blocked", "127.0.0.1", AddressPolicy{AllowTransition: true}, true},
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

// TestClientOptionsPolicy_Precedence covers the combined legacy and new
// options: allow wins over block, and the default is unchanged.
func TestClientOptionsPolicy_Precedence(t *testing.T) {
	t.Parallel()
	cgnat := []struct {
		blockCGNAT, allowCGNAT, wantAllowed bool
	}{
		{false, false, true},
		{true, false, false},
		{false, true, true},
		{true, true, true},
	}
	transition := []struct {
		block, wantAllowed bool
	}{
		{false, true},
		{true, false},
	}
	for _, c := range cgnat {
		for _, tr := range transition {
			opts := ClientOptions{BlockCGNAT: c.blockCGNAT, AllowCGNAT: c.allowCGNAT, BlockTransitionRanges: tr.block}
			p := opts.policy()
			if got := !p.Blocked(net.ParseIP("100.64.0.1")); got != c.wantAllowed {
				t.Errorf("%+v: shared space allowed = %v, want %v", opts, got, c.wantAllowed)
			}
			for _, ip := range []string{"198.18.0.1", "2002::1"} {
				if got := !p.Blocked(net.ParseIP(ip)); got != tr.wantAllowed {
					t.Errorf("%+v: %s allowed = %v, want %v", opts, ip, got, tr.wantAllowed)
				}
			}
			for _, ip := range []string{"100.100.100.200", "::ffff:100.100.100.200"} {
				if !p.Blocked(net.ParseIP(ip)) {
					t.Errorf("%+v: %s must be blocked", opts, ip)
				}
			}
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
