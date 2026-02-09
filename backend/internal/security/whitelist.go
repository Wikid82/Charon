package security

import (
	"net"
	"strings"

	"github.com/Wikid82/charon/backend/internal/util"
)

// IsIPInCIDRList returns true if clientIP matches any CIDR or IP in the list.
// The list is a comma-separated string of CIDRs and/or IPs.
func IsIPInCIDRList(clientIP, cidrList string) bool {
	if strings.TrimSpace(cidrList) == "" {
		return false
	}

	canonical := util.CanonicalizeIPForSecurity(clientIP)
	ip := net.ParseIP(canonical)
	if ip == nil {
		return false
	}

	parts := strings.Split(cidrList, ",")
	for _, part := range parts {
		entry := strings.TrimSpace(part)
		if entry == "" {
			continue
		}

		if parsed := net.ParseIP(entry); parsed != nil {
			// Fix for Issue 1: Canonicalize entry to support mixed IPv4/IPv6 loopback matching
			// This ensures that "::1" in the list matches "127.0.0.1" (from canonicalized client IP)
			if canonEntry := util.CanonicalizeIPForSecurity(entry); canonEntry != "" {
				if p := net.ParseIP(canonEntry); p != nil {
					parsed = p
				}
			}

			if ip.Equal(parsed) {
				return true
			}
			continue
		}

		_, cidr, err := net.ParseCIDR(entry)
		if err != nil {
			continue
		}
		if cidr.Contains(ip) {
			return true
		}

		// Fix for Issue 1: Handle IPv6 loopback CIDR matching against canonicalized IPv4 localhost
		// If client is 127.0.0.1 (canonical localhost) and CIDR contains ::1, allow it
		if ip.Equal(net.IPv4(127, 0, 0, 1)) && cidr.Contains(net.IPv6loopback) {
			return true
		}
	}

	return false
}
