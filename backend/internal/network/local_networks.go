package network

// localNetworkCIDRs lists the private, loopback and link-local ranges that count as
// "local network" for access control. Keep it as the single source of truth.
var localNetworkCIDRs = []string{
	"10.0.0.0/8",     // RFC1918
	"172.16.0.0/12",  // RFC1918
	"192.168.0.0/16", // RFC1918
	"127.0.0.0/8",    // localhost
	"169.254.0.0/16", // link-local
	"fc00::/7",       // IPv6 ULA
	"fe80::/10",      // IPv6 link-local
	"::1/128",        // IPv6 localhost
}

// LocalNetworkCIDRs returns a fresh copy of the local-network CIDR list so callers
// cannot mutate the shared definition.
func LocalNetworkCIDRs() []string {
	out := make([]string, len(localNetworkCIDRs))
	copy(out, localNetworkCIDRs)
	return out
}
