package services

import (
	"fmt"
	"strings"
)

// ValidateCrowdSecMode accepts the empty value and the two supported modes, 'local' and 'disabled'.
// Any other value, including an external one, is refused.
func ValidateCrowdSecMode(mode string) error {
	if mode == "" || mode == "local" || mode == "disabled" {
		return nil
	}
	return fmt.Errorf("invalid crowdsec mode: %s", mode)
}

// ValidateAdminWhitelist checks every comma-separated entry of the admin allow-list as an IP address or CIDR.
// An empty list is valid and clears the allow-list. Blank entries are skipped. Any malformed entry refuses the
// whole list with ErrInvalidAdminCIDR.
func ValidateAdminWhitelist(whitelist string) error {
	if whitelist == "" {
		return nil
	}
	for _, part := range strings.Split(whitelist, ",") {
		entry := strings.TrimSpace(part)
		if entry == "" {
			continue
		}
		if !isValidCIDR(entry) {
			return ErrInvalidAdminCIDR
		}
	}
	return nil
}

// ResolveRateLimitMode derives the stored rate-limit mode from the enable flag and the mode the request carries.
// Enabled always wins. Otherwise a non-empty mode is kept, and an empty mode means disabled.
func ResolveRateLimitMode(enable bool, mode string) string {
	if enable {
		return "enabled"
	}
	if mode == "" {
		return "disabled"
	}
	return mode
}
