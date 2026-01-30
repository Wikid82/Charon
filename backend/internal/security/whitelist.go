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
	}

	return false
}
