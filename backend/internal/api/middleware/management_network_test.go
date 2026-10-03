package middleware

import (
	"net"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseManagementNets_DefaultsWhenEmpty(t *testing.T) {
	nets := ParseManagementNets(nil)
	assert.True(t, IsManagementIP(nets, net.ParseIP("10.1.2.3")))
	assert.True(t, IsManagementIP(nets, net.ParseIP("192.168.1.1")))
	assert.True(t, IsManagementIP(nets, net.ParseIP("172.16.5.5")))
	assert.True(t, IsManagementIP(nets, net.ParseIP("127.0.0.1")))
	assert.True(t, IsManagementIP(nets, net.ParseIP("::1")))
	assert.False(t, IsManagementIP(nets, net.ParseIP("203.0.113.7")))
}

func TestParseManagementNets_CustomAndInvalid(t *testing.T) {
	nets := ParseManagementNets([]string{"203.0.113.0/24", "not-a-cidr"})
	assert.Len(t, nets, 1)
	assert.True(t, IsManagementIP(nets, net.ParseIP("203.0.113.7")))
	assert.False(t, IsManagementIP(nets, net.ParseIP("10.1.2.3")))
}

func TestParseManagementNets_AllInvalidFallsBackToDefaults(t *testing.T) {
	nets := ParseManagementNets([]string{"garbage"})
	assert.True(t, IsManagementIP(nets, net.ParseIP("10.1.2.3")))
}

func TestIsManagementIP_NilIP(t *testing.T) {
	assert.False(t, IsManagementIP(ParseManagementNets(nil), nil))
}
