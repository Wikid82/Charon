package middleware

import (
	"fmt"
	"net"
	"strings"
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

// loopbackManagementCIDRs is the narrowest allowed set. It is used when an
// explicitly configured list contains an invalid entry, so a typo never widens
// access beyond the host itself.
var loopbackManagementCIDRs = []string{
	"127.0.0.0/8",
	"::1/128",
}

// ParseManagementNets converts management network entries into networks. Each
// entry is a CIDR (192.0.2.0/24) or a bare address, which is treated as a
// single host (/32 or /128). An empty list selects the default private and
// loopback ranges.
//
// If any entry is invalid, the returned networks are loopback only and the
// error names every rejected entry. Callers decide how to report it.
func ParseManagementNets(entries []string) ([]*net.IPNet, error) {
	if len(entries) == 0 {
		return parseCIDRList(defaultManagementCIDRs), nil
	}

	nets := make([]*net.IPNet, 0, len(entries))
	var invalid []string
	for _, entry := range entries {
		ipnet, err := parseManagementEntry(entry)
		if err != nil {
			invalid = append(invalid, fmt.Sprintf("%q", entry))
			continue
		}
		nets = append(nets, ipnet)
	}
	if len(invalid) > 0 {
		err := fmt.Errorf("invalid management network entries %s: using loopback only", strings.Join(invalid, ", "))
		return parseCIDRList(loopbackManagementCIDRs), err
	}
	return nets, nil
}

// parseManagementEntry parses one CIDR or bare IP address.
func parseManagementEntry(entry string) (*net.IPNet, error) {
	if strings.Contains(entry, "/") {
		_, ipnet, err := net.ParseCIDR(entry)
		return ipnet, err
	}
	ip := net.ParseIP(entry)
	if ip == nil {
		return nil, fmt.Errorf("not an IP address or CIDR: %q", entry)
	}
	if ip4 := ip.To4(); ip4 != nil {
		return &net.IPNet{IP: ip4, Mask: net.CIDRMask(32, 32)}, nil
	}
	return &net.IPNet{IP: ip, Mask: net.CIDRMask(128, 128)}, nil
}

// parseCIDRList parses CIDRs already known to be valid.
func parseCIDRList(cidrs []string) []*net.IPNet {
	nets := make([]*net.IPNet, 0, len(cidrs))
	for _, cidr := range cidrs {
		nets = append(nets, mustParseCIDR(cidr))
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
