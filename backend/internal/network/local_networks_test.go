package network

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestLocalNetworkCIDRs_Pinned pins the exact local-network range list shared by
// the proxy ACL generator and the access list service.
func TestLocalNetworkCIDRs_Pinned(t *testing.T) {
	want := []string{
		"10.0.0.0/8",
		"172.16.0.0/12",
		"192.168.0.0/16",
		"127.0.0.0/8",
		"169.254.0.0/16",
		"fc00::/7",
		"fe80::/10",
		"::1/128",
	}
	assert.Equal(t, want, LocalNetworkCIDRs())
}

// TestLocalNetworkCIDRs_ReturnsIndependentCopy guards against callers mutating the shared list.
func TestLocalNetworkCIDRs_ReturnsIndependentCopy(t *testing.T) {
	a := LocalNetworkCIDRs()
	a[0] = "0.0.0.0/0"
	assert.Equal(t, "10.0.0.0/8", LocalNetworkCIDRs()[0])
}
