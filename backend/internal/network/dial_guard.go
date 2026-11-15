package network

import (
	"fmt"
	"net"
	"net/netip"
	"syscall"
	"time"
)

// DialGuard refuses one literal destination on top of the AddressPolicy. It
// must not resolve names and must wrap ErrBlockedAddress when it refuses, so
// callers can detect the refusal with errors.Is. Guards can only refuse; an
// allow decision always comes from the AddressPolicy.
type DialGuard func(dest netip.AddrPort) error

// NewGuardedDialer returns a dialer whose Control hook checks every literal
// address it is about to connect to against the policy and then each guard.
//
// The policy is a function so that values which change at runtime (for
// example the gateway carve-out addresses) take effect without rebuilding
// the dialer. The hook runs once per connection attempt: when a refused
// address is one of several a name resolves to, the dialer moves on to the
// next address, so a refused address is never connected to.
func NewGuardedDialer(timeout time.Duration, policy func() AddressPolicy, guards ...DialGuard) *net.Dialer {
	return &net.Dialer{
		Timeout: timeout,
		Control: func(_ string, address string, _ syscall.RawConn) error {
			dest, err := netip.ParseAddrPort(address)
			if err != nil {
				return fmt.Errorf("unparsable destination: %w", ErrBlockedAddress)
			}
			return CheckDestination(dest, policy(), guards...)
		},
	}
}

// CheckDestination applies the policy and then each guard to one literal
// destination. IPv6 zones are dropped and IPv4-mapped IPv6 addresses are
// unmapped first, so the policy sees the same address the kernel connects to.
// Errors never include the address.
func CheckDestination(dest netip.AddrPort, policy AddressPolicy, guards ...DialGuard) error {
	addr := dest.Addr().WithZone("").Unmap()
	if !addr.IsValid() {
		return fmt.Errorf("invalid destination: %w", ErrBlockedAddress)
	}
	if policy.Blocked(net.IP(addr.AsSlice())) {
		return fmt.Errorf("destination refused by address policy: %w", ErrBlockedAddress)
	}
	literal := netip.AddrPortFrom(addr, dest.Port())
	for _, guard := range guards {
		if guard == nil {
			continue
		}
		if err := guard(literal); err != nil {
			return err
		}
	}
	return nil
}
