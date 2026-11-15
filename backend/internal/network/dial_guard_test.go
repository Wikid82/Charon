package network

import (
	"context"
	"errors"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"testing"
	"time"
)

// alwaysBlockedAddrs are the cloud instance service addresses that no option
// may allow. Each is tested with every allow flag turned on.
var alwaysBlockedAddrs = []string{
	"169.254.169.254",
	"::ffff:169.254.169.254",
	"fd00:ec2::254",
	"fd00:ec2::23",
	"fd20:ce::254",
	"100.100.100.200",
	"168.63.129.16",
}

func allowEverything() AddressPolicy {
	return AddressPolicy{
		AllowLocalhost:    true,
		AllowRFC1918:      true,
		AllowCGNAT:        true,
		AllowUniqueLocal:  true,
		AllowBenchmarking: true,
		AllowExact:        []netip.Addr{netip.MustParseAddr("fd00:ec2::254"), netip.MustParseAddr("169.254.169.254"), netip.MustParseAddr("100.100.100.200")},
	}
}

func TestIsAlwaysBlocked(t *testing.T) {
	t.Parallel()
	for _, raw := range alwaysBlockedAddrs {
		if !IsAlwaysBlocked(net.ParseIP(raw)) {
			t.Errorf("IsAlwaysBlocked(%s) = false, want true", raw)
		}
	}
	for _, raw := range []string{"169.254.1.2", "10.0.0.1", "fd00::1", "fd00:ec3::1", "8.8.8.8", "100.100.100.201"} {
		if IsAlwaysBlocked(net.ParseIP(raw)) {
			t.Errorf("IsAlwaysBlocked(%s) = true, want false", raw)
		}
	}
	if IsAlwaysBlocked(nil) {
		t.Error("IsAlwaysBlocked(nil) = true, want false")
	}
}

func TestIsUniqueLocalAndBenchmarking(t *testing.T) {
	t.Parallel()
	if !IsUniqueLocal(net.ParseIP("fd12:3456::1")) || !IsUniqueLocal(net.ParseIP("fc00::1")) {
		t.Error("IsUniqueLocal should match fc00::/7")
	}
	for _, raw := range []string{"fe80::1", "2001:db8::1", "::ffff:10.0.0.1", "10.0.0.1"} {
		if IsUniqueLocal(net.ParseIP(raw)) {
			t.Errorf("IsUniqueLocal(%s) = true, want false", raw)
		}
	}
	if !IsBenchmarking(net.ParseIP("198.18.0.5")) || !IsBenchmarking(net.ParseIP("198.19.255.255")) {
		t.Error("IsBenchmarking should match 198.18.0.0/15")
	}
	for _, raw := range []string{"198.20.0.1", "198.17.255.255", "192.0.2.1"} {
		if IsBenchmarking(net.ParseIP(raw)) {
			t.Errorf("IsBenchmarking(%s) = true, want false", raw)
		}
	}
	if IsBenchmarking(nil) {
		t.Error("IsBenchmarking(nil) = true, want false")
	}
}

func TestAddressPolicy_AlwaysBlockedWinsOverAllowFlags(t *testing.T) {
	t.Parallel()
	for _, raw := range alwaysBlockedAddrs {
		if !allowEverything().Blocked(net.ParseIP(raw)) {
			t.Errorf("Blocked(%s) with every allow flag = false, want true", raw)
		}
	}
}

func TestAddressPolicy_AllowFlagsOpenOnlyTheirRange(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		policy  AddressPolicy
		ip      string
		blocked bool
	}{
		{"ula allowed", AddressPolicy{AllowUniqueLocal: true}, "fd12::5", false},
		{"ula blocked by default", AddressPolicy{}, "fd12::5", true},
		{"ula allowed does not open link-local", AddressPolicy{AllowUniqueLocal: true}, "fe80::1", true},
		{"benchmarking allowed", AddressPolicy{AllowBenchmarking: true}, "198.18.0.5", false},
		{"benchmarking blocked by default", AddressPolicy{}, "198.18.0.5", true},
		{"benchmarking does not open the rest of transition", AddressPolicy{AllowBenchmarking: true}, "192.0.0.5", true},
		{"benchmarking does not open next block", AddressPolicy{AllowBenchmarking: true}, "198.20.0.1", false},
		{"ipv4 mapped rfc1918 allowed", AddressPolicy{AllowRFC1918: true}, "::ffff:10.1.2.3", false},
		{"ipv4 mapped link-local blocked", allowEverything(), "::ffff:169.254.1.2", true},
		{"loopback allowed", AddressPolicy{AllowLocalhost: true}, "127.0.0.1", false},
		{"public blocked", allowEverything(), "8.8.8.8", false},
		{"nil blocked", AddressPolicy{}, "", true},
		{"cgnat blocked by default", AddressPolicy{}, "100.64.1.1", true},
		{"cgnat allowed", AddressPolicy{AllowCGNAT: true}, "100.64.1.1", false},
		{"unspecified blocked", allowEverything(), "0.0.0.0", true},
		{"zero policy blocks rfc1918", AddressPolicy{}, "192.168.1.1", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			var ip net.IP
			if tc.ip != "" {
				ip = net.ParseIP(tc.ip)
			}
			if got := tc.policy.Blocked(ip); got != tc.blocked {
				t.Errorf("Blocked(%q) = %v, want %v", tc.ip, got, tc.blocked)
			}
		})
	}
}

func TestAddressPolicy_ExactAllowIsMatchedAfterUnmapping(t *testing.T) {
	t.Parallel()
	p := AddressPolicy{AllowExact: []netip.Addr{netip.MustParseAddr("169.254.1.2")}}
	if p.Blocked(net.ParseIP("169.254.1.2")) {
		t.Error("exact IPv4 entry should allow 169.254.1.2")
	}
	if p.Blocked(net.ParseIP("::ffff:169.254.1.2")) {
		t.Error("exact entry should match the IPv4-mapped form")
	}
	if !p.Blocked(net.ParseIP("169.254.1.3")) {
		t.Error("other link-local addresses must stay blocked")
	}

	mapped := AddressPolicy{AllowExact: []netip.Addr{netip.MustParseAddr("::ffff:169.254.1.2")}}
	if mapped.Blocked(net.ParseIP("169.254.1.2")) {
		t.Error("mapped exact entry should match the IPv4 form")
	}
	if !(AddressPolicy{AllowExact: []netip.Addr{netip.MustParseAddr("0.250.250.254")}}).Blocked(nil) {
		t.Error("nil must stay blocked even with an exact list")
	}
}

func TestAddressPolicy_ExactAllowCannotReachAlwaysBlocked(t *testing.T) {
	t.Parallel()
	p := AddressPolicy{AllowExact: []netip.Addr{netip.MustParseAddr("fd00:ec2::254")}}
	if !p.Blocked(net.ParseIP("fd00:ec2::254")) {
		t.Error("always-blocked address must stay blocked even when listed as exact")
	}
}

func TestCheckDestination_Table(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		dest    string
		policy  AddressPolicy
		blocked bool
	}{
		{"ipv4 mapped metadata", "[::ffff:169.254.169.254]:80", allowEverything(), true},
		{"ipv6 zone on link-local", "[fe80::1%eth0]:80", allowEverything(), true},
		{"ipv6 zone on ula is unzoned", "[fd12::1%eth0]:80", AddressPolicy{AllowUniqueLocal: true}, false},
		{"ipv6 instance service", "[fd00:ec2::254]:80", allowEverything(), true},
		{"gcp ipv6 metadata", "[fd20:ce::254]:80", allowEverything(), true},
		{"azure wire server", "168.63.129.16:80", allowEverything(), true},
		{"ula allowed", "[fd12::5]:443", AddressPolicy{AllowUniqueLocal: true}, false},
		{"fake ip allowed", "198.18.0.5:443", AddressPolicy{AllowBenchmarking: true}, false},
		{"loopback with port", "127.0.0.1:8080", AddressPolicy{AllowLocalhost: true}, false},
		{"public allowed", "8.8.8.8:443", AddressPolicy{}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dest, err := netip.ParseAddrPort(tc.dest)
			if err != nil {
				t.Fatalf("ParseAddrPort(%q): %v", tc.dest, err)
			}
			err = CheckDestination(dest, tc.policy)
			if tc.blocked {
				if !errors.Is(err, ErrBlockedAddress) {
					t.Fatalf("CheckDestination(%s) = %v, want ErrBlockedAddress", tc.dest, err)
				}
				return
			}
			if err != nil {
				t.Fatalf("CheckDestination(%s) = %v, want nil", tc.dest, err)
			}
		})
	}
}

func TestCheckDestination_GuardsRefuseAndErrorsOmitAddress(t *testing.T) {
	t.Parallel()
	dest := netip.MustParseAddrPort("10.9.8.7:2019")
	refuse := func(netip.AddrPort) error { return ErrBlockedAddress }
	err := CheckDestination(dest, AddressPolicy{AllowRFC1918: true}, nil, refuse)
	if !errors.Is(err, ErrBlockedAddress) {
		t.Fatalf("guard refusal = %v, want ErrBlockedAddress", err)
	}

	err = CheckDestination(netip.MustParseAddrPort("10.9.8.7:2019"), AddressPolicy{})
	if !errors.Is(err, ErrBlockedAddress) {
		t.Fatalf("policy refusal = %v, want ErrBlockedAddress", err)
	}
	if strings.Contains(err.Error(), "10.9.8.7") || strings.Contains(err.Error(), "2019") {
		t.Errorf("error leaks the destination: %q", err.Error())
	}
}

func TestCheckDestination_GuardReceivesUnmappedLiteral(t *testing.T) {
	t.Parallel()
	var seen netip.AddrPort
	guard := func(dest netip.AddrPort) error {
		seen = dest
		return nil
	}
	dest := netip.MustParseAddrPort("[::ffff:10.0.0.9]:8085")
	if err := CheckDestination(dest, AddressPolicy{AllowRFC1918: true}, guard); err != nil {
		t.Fatalf("CheckDestination = %v, want nil", err)
	}
	if seen.String() != "10.0.0.9:8085" {
		t.Errorf("guard saw %s, want 10.0.0.9:8085", seen)
	}
}

func TestCheckDestination_InvalidAddressRefused(t *testing.T) {
	t.Parallel()
	if err := CheckDestination(netip.AddrPort{}, allowEverything()); !errors.Is(err, ErrBlockedAddress) {
		t.Fatalf("zero AddrPort = %v, want ErrBlockedAddress", err)
	}
}

func TestNewGuardedDialer_ControlRefusesUnparsableAddress(t *testing.T) {
	t.Parallel()
	d := NewGuardedDialer(time.Second, allowEverything)
	if err := d.Control("tcp", "not-an-address", nil); !errors.Is(err, ErrBlockedAddress) {
		t.Fatalf("Control(unparsable) = %v, want ErrBlockedAddress", err)
	}
	if err := d.Control("tcp", "10.0.0.1:80", nil); err != nil {
		t.Fatalf("Control(allowed) = %v, want nil", err)
	}
	if err := d.Control("tcp", "[fd00:ec2::254]:80", nil); !errors.Is(err, ErrBlockedAddress) {
		t.Fatalf("Control(always blocked) = %v, want ErrBlockedAddress", err)
	}
}

func TestNewGuardedDialer_RefusesLiteralDestinationBeforeConnect(t *testing.T) {
	t.Parallel()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close() }()
	port := ln.Addr().(*net.TCPAddr).Port

	refuseLoopback := func(dest netip.AddrPort) error {
		if dest.Addr().IsLoopback() {
			return ErrBlockedAddress
		}
		return nil
	}
	d := NewGuardedDialer(2*time.Second, func() AddressPolicy { return AddressPolicy{AllowLocalhost: true} }, refuseLoopback)
	conn, err := d.DialContext(context.Background(), "tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if conn != nil {
		_ = conn.Close()
	}
	if !errors.Is(err, ErrBlockedAddress) {
		t.Fatalf("DialContext = %v, want ErrBlockedAddress through *net.OpError", err)
	}
}

func TestNewGuardedDialer_FallsThroughToAllowedAddress(t *testing.T) {
	t.Parallel()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close() }()
	port := ln.Addr().(*net.TCPAddr).Port
	go func() {
		for {
			c, acceptErr := ln.Accept()
			if acceptErr != nil {
				return
			}
			_ = c.Close()
		}
	}()

	// "localhost" resolves to loopback addresses. The guard refuses every IPv6
	// one, so the connection must go to the IPv4 address and nothing else.
	refuseIPv6 := func(dest netip.AddrPort) error {
		if dest.Addr().Is6() {
			return ErrBlockedAddress
		}
		return nil
	}
	d := NewGuardedDialer(2*time.Second, func() AddressPolicy { return AddressPolicy{AllowLocalhost: true} }, refuseIPv6)
	conn, err := d.DialContext(context.Background(), "tcp", net.JoinHostPort("localhost", strconv.Itoa(port)))
	if err != nil {
		t.Fatalf("DialContext(localhost) = %v, want success on the IPv4 address", err)
	}
	defer func() { _ = conn.Close() }()
	if remote := conn.RemoteAddr().(*net.TCPAddr); !remote.IP.Equal(net.ParseIP("127.0.0.1")) {
		t.Fatalf("connected to %s, want 127.0.0.1", remote.IP)
	}
}

func TestNewGuardedDialer_NameWithOnlyRefusedAddressesFails(t *testing.T) {
	t.Parallel()
	d := NewGuardedDialer(2*time.Second, func() AddressPolicy { return AddressPolicy{} })
	conn, err := d.DialContext(context.Background(), "tcp", "localhost:9")
	if conn != nil {
		_ = conn.Close()
	}
	if !errors.Is(err, ErrBlockedAddress) {
		t.Fatalf("DialContext(localhost) with no loopback policy = %v, want ErrBlockedAddress", err)
	}
}

func TestSafeDialer_LocalhostShortcutRunsGuards(t *testing.T) {
	t.Parallel()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer func() { _ = ln.Close() }()
	port := ln.Addr().(*net.TCPAddr).Port

	opts := defaultOptions()
	opts.AllowLocalhost = true
	opts.DialTimeout = 2 * time.Second
	opts.dialGuards = []DialGuard{func(dest netip.AddrPort) error { return ErrBlockedAddress }}
	dial := safeDialer(&opts)
	conn, err := dial(context.Background(), "tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(port)))
	if conn != nil {
		_ = conn.Close()
	}
	if !errors.Is(err, ErrBlockedAddress) {
		t.Fatalf("localhost shortcut with refusing guard = %v, want ErrBlockedAddress", err)
	}
}

func TestSafeDialer_RefusesAlwaysBlockedNameTarget(t *testing.T) {
	opts := defaultOptions()
	opts.AllowUniqueLocal = true
	opts.AllowCGNAT = true
	opts.AllowRFC1918 = true
	opts.allowExact = func() []netip.Addr { return []netip.Addr{netip.MustParseAddr("fd00:ec2::254")} }
	dial := safeDialer(&opts)
	withResolver(t, map[string][]string{"imds.test": {"fd00:ec2::254"}})
	if _, err := dial(context.Background(), "tcp", "imds.test:80"); !errors.Is(err, ErrBlockedAddress) {
		t.Fatalf("always-blocked name = %v, want ErrBlockedAddress", err)
	}
}

func TestClientOptions_AllowExactIsReadOnEveryCall(t *testing.T) {
	t.Parallel()
	calls := 0
	opts := defaultOptions()
	WithAllowExact(func() []netip.Addr {
		calls++
		return []netip.Addr{netip.MustParseAddr("169.254.1.2")}
	})(&opts)
	first := opts.policy()
	second := opts.policy()
	if calls != 2 {
		t.Fatalf("allow function called %d times, want 2", calls)
	}
	if first.Blocked(net.ParseIP("169.254.1.2")) || second.Blocked(net.ParseIP("169.254.1.2")) {
		t.Error("exact carve-out should allow 169.254.1.2")
	}
}

func TestClientOptions_NewOptionsSetFields(t *testing.T) {
	t.Parallel()
	opts := defaultOptions()
	WithAllowUniqueLocal()(&opts)
	WithAllowBenchmarking()(&opts)
	WithDialGuard(nil)(&opts)
	WithDialGuard(func(netip.AddrPort) error { return nil })(&opts)
	p := opts.policy()
	if !p.AllowUniqueLocal || !p.AllowBenchmarking {
		t.Fatalf("policy = %+v, want unique-local and benchmarking allowed", p)
	}
	if len(opts.dialGuards) != 1 {
		t.Fatalf("dialGuards = %d, want 1 (nil guard skipped)", len(opts.dialGuards))
	}
}

func TestClientOptions_BenchmarkingOptionReachesPolicy(t *testing.T) {
	t.Parallel()
	opts := defaultOptions()
	if !opts.policy().Blocked(net.ParseIP("198.18.0.5")) {
		t.Fatal("benchmarking must be blocked without the option")
	}
	WithAllowBenchmarking()(&opts)
	if opts.policy().Blocked(net.ParseIP("198.18.0.5")) {
		t.Fatal("benchmarking must be allowed with the option")
	}
}
