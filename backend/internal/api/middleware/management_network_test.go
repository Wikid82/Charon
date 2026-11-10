package middleware

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestManagementNets_DefaultsWhenEmpty(t *testing.T) {
	nets, err := ParseManagementNets(nil)
	require.NoError(t, err)
	assert.True(t, IsManagementIP(nets, net.ParseIP("10.1.2.3")))
	assert.True(t, IsManagementIP(nets, net.ParseIP("192.168.1.1")))
	assert.True(t, IsManagementIP(nets, net.ParseIP("172.16.5.5")))
	assert.True(t, IsManagementIP(nets, net.ParseIP("127.0.0.1")))
	assert.True(t, IsManagementIP(nets, net.ParseIP("::1")))
	assert.False(t, IsManagementIP(nets, net.ParseIP("203.0.113.7")))
}

func TestManagementNets_ValidListHasNoError(t *testing.T) {
	nets, err := ParseManagementNets([]string{"203.0.113.0/24", "2001:db8::/32"})
	require.NoError(t, err)
	assert.Len(t, nets, 2)
	assert.True(t, IsManagementIP(nets, net.ParseIP("203.0.113.7")))
	assert.True(t, IsManagementIP(nets, net.ParseIP("2001:db8::1")))
	assert.False(t, IsManagementIP(nets, net.ParseIP("10.1.2.3")))
}

func TestManagementNets_BareIPv4IsSingleAddress(t *testing.T) {
	nets, err := ParseManagementNets([]string{"10.0.0.5"})
	require.NoError(t, err)
	assert.True(t, IsManagementIP(nets, net.ParseIP("10.0.0.5")))
	assert.False(t, IsManagementIP(nets, net.ParseIP("10.0.0.6")))
	assert.False(t, IsManagementIP(nets, net.ParseIP("127.0.0.1")))
}

func TestManagementNets_BareIPv6IsSingleAddress(t *testing.T) {
	nets, err := ParseManagementNets([]string{"2001:db8::5"})
	require.NoError(t, err)
	assert.True(t, IsManagementIP(nets, net.ParseIP("2001:db8::5")))
	assert.False(t, IsManagementIP(nets, net.ParseIP("2001:db8::6")))
}

func TestManagementNets_InvalidEntryFallsBackToLoopbackOnly(t *testing.T) {
	nets, err := ParseManagementNets([]string{"203.0.113.0/24", "not-a-cidr"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "not-a-cidr")
	assert.NotContains(t, err.Error(), "203.0.113.0/24")

	assert.True(t, IsManagementIP(nets, net.ParseIP("127.0.0.1")))
	assert.True(t, IsManagementIP(nets, net.ParseIP("127.9.9.9")))
	assert.True(t, IsManagementIP(nets, net.ParseIP("::1")))
	assert.False(t, IsManagementIP(nets, net.ParseIP("203.0.113.7")))
	assert.False(t, IsManagementIP(nets, net.ParseIP("10.1.2.3")))
	assert.False(t, IsManagementIP(nets, net.ParseIP("192.168.1.1")))
}

func TestManagementNets_AllInvalidFallsBackToLoopbackOnly(t *testing.T) {
	nets, err := ParseManagementNets([]string{"garbage"})
	require.Error(t, err)
	assert.True(t, IsManagementIP(nets, net.ParseIP("127.0.0.1")))
	assert.False(t, IsManagementIP(nets, net.ParseIP("10.1.2.3")))
}

func TestIsManagementIP_NilIP(t *testing.T) {
	nets, err := ParseManagementNets(nil)
	require.NoError(t, err)
	assert.False(t, IsManagementIP(nets, nil))
}
