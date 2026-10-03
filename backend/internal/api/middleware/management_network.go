package middleware

import (
	"net"

	"github.com/Wikid82/charon/backend/internal/logger"
)

// defaultManagementCIDRs are used when no management networks are configured:
// RFC 1918 private ranges plus loopback.
var defaultManagementCIDRs = []string{
	"10.0.0.0/8",
	"172.16.0.0/12",
	"192.168.0.0/16",
	"127.0.0.0/8",
	"::1/128",
}

// ParseManagementNets parses CIDR strings, skipping invalid entries. When no
// valid entry remains it returns the default private and loopback ranges.
func ParseManagementNets(cidrs []string) []*net.IPNet {
	var nets []*net.IPNet
	for _, cidr := range cidrs {
		_, ipnet, err := net.ParseCIDR(cidr)
		if err != nil {
			logger.Log().WithError(err).WithField("cidr", cidr).Warn("Invalid management CIDR")
			continue
		}
		nets = append(nets, ipnet)
	}
	if len(nets) == 0 {
		for _, cidr := range defaultManagementCIDRs {
			nets = append(nets, mustParseCIDR(cidr))
		}
	}
	return nets
}

// IsManagementIP reports whether ip falls inside any of nets.
func IsManagementIP(nets []*net.IPNet, ip net.IP) bool {
	if ip == nil {
		return false
	}
	for _, ipnet := range nets {
		if ipnet.Contains(ip) {
			return true
		}
	}
	return false
}
