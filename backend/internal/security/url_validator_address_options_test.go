package security

import (
	"errors"
	"net/netip"
	"testing"

	"github.com/Wikid82/charon/backend/internal/network"
)

func TestValidateExternalURL_AddressPolicyOptions(t *testing.T) {
	t.Parallel()
	exact := func() []netip.Addr { return []netip.Addr{netip.MustParseAddr("169.254.1.2")} }
	cases := []struct {
		name    string
		url     string
		options []ValidationOption
		wantErr bool
	}{
		{"ula refused by default", "https://[fd12::5]/", nil, true},
		{"ula allowed", "https://[fd12::5]/", []ValidationOption{WithAllowUniqueLocal()}, false},
		{"benchmarking refused by default", "https://198.18.0.5/", nil, true},
		{"benchmarking allowed", "https://198.18.0.5/", []ValidationOption{WithAllowBenchmarking()}, false},
		{"exact carve-out allowed", "https://169.254.1.2/", []ValidationOption{WithAllowExact(exact)}, false},
		{"other link-local refused with carve-out", "https://169.254.1.3/", []ValidationOption{WithAllowExact(exact)}, true},
		{"instance service refused with every flag", "https://[fd00:ec2::254]/", []ValidationOption{
			WithAllowUniqueLocal(), WithAllowBenchmarking(), WithAllowRFC1918(), WithAllowCGNAT(),
			WithAllowExact(func() []netip.Addr { return []netip.Addr{netip.MustParseAddr("fd00:ec2::254")} }),
		}, true},
		{"gcp ipv6 metadata refused with ula", "https://[fd20:ce::254]/", []ValidationOption{WithAllowUniqueLocal()}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := ValidateExternalURL(tc.url, tc.options...)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("ValidateExternalURL(%s) = nil, want error", tc.url)
				}
				if !errors.Is(err, network.ErrBlockedAddress) {
					t.Fatalf("ValidateExternalURL(%s) error %v does not wrap ErrBlockedAddress", tc.url, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("ValidateExternalURL(%s) = %v, want nil", tc.url, err)
			}
		})
	}
}
