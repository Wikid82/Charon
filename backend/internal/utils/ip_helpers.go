package utils

import (
	"net"

	"github.com/Wikid82/charon/backend/internal/network"
)

// IsPrivateIP checks if the given host string is a private IPv4 address.
// Returns false for hostnames, invalid IPs, or public IP addresses.
//
// Deprecated: This function only checks IPv4. For comprehensive SSRF protection,
// use network.IsPrivateIP() directly which handles IPv4, IPv6, and IPv4-mapped IPv6.
func IsPrivateIP(host string) bool {
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}

	// Ensure it's IPv4 (for backward compatibility)
	ip4 := ip.To4()
	if ip4 == nil {
		return false
	}

	// Use centralized network.IsPrivateIP for consistent checking
	return network.IsPrivateIP(ip)
}

// IsDockerBridgeIP checks if the given host string is likely a Docker bridge network IP.
// Docker typically uses 172.17.x.x for the default bridge and 172.18-31.x.x for user-defined networks.
// Returns false for hostnames, invalid IPs, or non-Docker IP addresses.
func IsDockerBridgeIP(host string) bool {
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}

	// Ensure it's IPv4
	ip4 := ip.To4()
	if ip4 == nil {
		return false
	}

	// Docker bridge network CIDR range: 172.16.0.0/12
	_, dockerNetwork, err := net.ParseCIDR("172.16.0.0/12")
	if err != nil {
		return false
	}

	return dockerNetwork.Contains(ip4)
}
