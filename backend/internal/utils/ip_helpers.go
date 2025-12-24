package utils

import "net"

// Private IPv4 CIDR ranges (RFC 1918)
var privateIPv4Ranges = []string{
	"10.0.0.0/8",     // Class A private
	"172.16.0.0/12",  // Class B private (includes Docker bridge networks)
	"192.168.0.0/16", // Class C private
}

// Docker bridge network CIDR range
// Docker default bridge: 172.17.0.0/16
// Docker user-defined networks: 172.18.0.0/16 - 172.31.0.0/16
// All fall within 172.16.0.0/12
var dockerBridgeRange = "172.16.0.0/12"

// IsPrivateIP checks if the given host string is a private IPv4 address.
// Returns false for hostnames, invalid IPs, or public IP addresses.
func IsPrivateIP(host string) bool {
	ip := net.ParseIP(host)
	if ip == nil {
		return false
	}

	// Ensure it's IPv4
	ip4 := ip.To4()
	if ip4 == nil {
		return false
	}

	for _, cidr := range privateIPv4Ranges {
		_, network, err := net.ParseCIDR(cidr)
		if err != nil {
			continue
		}
		if network.Contains(ip4) {
			return true
		}
	}
	return false
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

	_, network, err := net.ParseCIDR(dockerBridgeRange)
	if err != nil {
		return false
	}

	return network.Contains(ip4)
}
