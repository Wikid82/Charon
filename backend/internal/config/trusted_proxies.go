package config

import (
	"fmt"
	"net/netip"
	"strconv"
	"strings"

	"github.com/Wikid82/charon/backend/internal/security"
)

// ValidateTrustedProxies validates CHARON_TRUSTED_PROXIES once, with Gin's
// parsing rules, so Gin, the session-cookie logic, the sign-in throttle and the
// admin status all share one effective list. When every entry is valid the
// trimmed original strings are returned unchanged. Any invalid entry yields an
// empty list (trust nothing) plus a warning. A list that trusts every address
// is kept, because it is the operator's explicit choice, but also warns.
func ValidateTrustedProxies(entries []string) (effective, warnings []string) {
	trustAll := false
	for _, raw := range entries {
		entry := strings.TrimSpace(raw)
		prefix, err := security.ParseTrustedProxy(entry)
		if err != nil {
			return nil, []string{fmt.Sprintf(
				"CHARON_TRUSTED_PROXIES entry %s is not a valid IP address or CIDR; no proxy is trusted until it is fixed",
				strconv.Quote(entry))}
		}
		if prefix.Contains(netip.IPv4Unspecified()) || prefix.Contains(netip.IPv6Unspecified()) {
			trustAll = true
		}
		effective = append(effective, entry)
	}
	if trustAll {
		warnings = append(warnings, "CHARON_TRUSTED_PROXIES trusts every address (it contains 0.0.0.0 or ::), "+
			"so any client can choose its own address; list only your reverse proxy's exact address")
	}
	return effective, warnings
}
