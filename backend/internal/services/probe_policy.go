package services

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/crowdsec"
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/network"
	"github.com/Wikid82/charon/backend/internal/ratelimit"
)

// ErrDestinationRefused is returned when a destination is refused by the probe
// policy at save time. Dial-time refusals wrap network.ErrBlockedAddress.
var ErrDestinationRefused = errors.New("destination is not allowed")

// reservedPortsTTL bounds how stale the reserved-port set may be on the dial path.
const reservedPortsTTL = 5 * time.Second

// hostAddrsTTL bounds how stale the host-side address and gateway snapshots may be.
const hostAddrsTTL = 30 * time.Second

// gatewayLookupTimeout bounds each lookup of a gateway name.
const gatewayLookupTimeout = time.Second

// hostsFilePath is the hosts file read for gateway name entries. Tests replace it.
var hostsFilePath = "/etc/hosts"

// emergencyPort is the emergency server port while that server is enabled, or 0.
var emergencyPort atomic.Int32

// SetEmergencyListener records the emergency server port while the server is
// enabled. A disabled server reserves nothing. Call once at startup.
func SetEmergencyListener(cfg config.EmergencyConfig) {
	if !cfg.Enabled {
		emergencyPort.Store(0)
		return
	}
	emergencyPort.Store(int32(listenPortFrom(cfg.BindAddress))) // #nosec G115 -- a port is 0..65535
}

// reservedListenerPorts returns the ports Charon keeps for itself on this machine:
// the agent listener ports (stored and live), the Caddy admin TCP port, the
// emergency port while enabled and the bundled CrowdSec LAPI port. tx may be nil,
// in which case only the live agent ports are included.
func reservedListenerPorts(tx *gorm.DB) (map[int]struct{}, error) {
	var ports map[int]struct{}
	if tx == nil {
		ports = make(map[int]struct{})
		for _, p := range liveListenerPorts() {
			if p > 0 {
				ports[p] = struct{}{}
			}
		}
	} else {
		agent, err := listenerPortSet(tx)
		if err != nil {
			return nil, err
		}
		ports = agent
	}
	for _, p := range []int{
		caddyAdminAPIPort,
		int(emergencyPort.Load()),
		listenPortFrom(crowdsec.LocalLAPIURL),
	} {
		if p > 0 {
			ports[p] = struct{}{}
		}
	}
	return ports, nil
}

// ReservedListenerGuard refuses a literal destination that is this machine on a
// port Charon reserves. Other destinations pass. It caches the reserved set for
// reservedPortsTTL. A database error refuses and is never cached.
type ReservedListenerGuard struct {
	db      *gorm.DB
	now     func() time.Time
	load    func(*gorm.DB) (map[int]struct{}, error)
	warn    *ratelimit.WarnBudget
	mu      sync.Mutex
	ports   map[int]struct{}
	fetched time.Time
}

// NewReservedListenerGuard returns a guard backed by db.
func NewReservedListenerGuard(db *gorm.DB) *ReservedListenerGuard {
	return &ReservedListenerGuard{
		db:   db,
		now:  time.Now,
		load: reservedListenerPorts,
		warn: ratelimit.NewWarnBudget(3, time.Minute, nil),
	}
}

// Check implements network.DialGuard.
func (g *ReservedListenerGuard) Check(dest netip.AddrPort) error {
	addr := dest.Addr().WithZone("").Unmap()
	if !addr.IsValid() || !isThisMachineAddr(net.IP(addr.AsSlice())) {
		return nil
	}
	ports, err := g.reserved()
	if err != nil {
		if ok, _ := g.warn.Take(); ok {
			logWarnReservedPorts(err)
		}
		return fmt.Errorf("reserved listener set unavailable: %w", network.ErrBlockedAddress)
	}
	if _, reserved := ports[int(dest.Port())]; reserved {
		return fmt.Errorf("port reserved on this machine: %w", network.ErrBlockedAddress)
	}
	return nil
}

// reserved returns the cached reserved set, reloading it when it is older than reservedPortsTTL.
func (g *ReservedListenerGuard) reserved() (map[int]struct{}, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	now := g.now()
	if g.ports != nil && now.Sub(g.fetched) < reservedPortsTTL {
		return g.ports, nil
	}
	ports, err := g.load(g.db)
	if err != nil {
		return nil, err
	}
	g.ports, g.fetched = ports, now
	return ports, nil
}

// logWarnReservedPorts records that the reserved set could not be read. It names
// no address or port, so it is safe in the server log.
func logWarnReservedPorts(err error) {
	logger.Log().WithError(err).Warn("destination check: reserved port set unavailable, refusing probe")
}

// addrCache holds an address snapshot for a fixed time. The first call loads
// synchronously. After the snapshot expires, callers get the previous value at
// once while one background refresh runs, so a query never waits on a reload.
// When a reload returns nothing, the previous non-empty snapshot is kept.
type addrCache struct {
	mu         sync.Mutex
	ttl        time.Duration
	now        func() time.Time
	load       func() []netip.Addr
	value      []netip.Addr
	fetched    time.Time
	loaded     bool
	refreshing bool
}

func (c *addrCache) get() []netip.Addr {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	if !c.loaded {
		c.store(c.load(), now)
		return c.value
	}
	if now.Sub(c.fetched) >= c.ttl && !c.refreshing {
		c.refreshing = true
		go c.refreshInBackground(c.load, now)
	}
	return c.value
}

// refreshInBackground runs one reload and stores it. It takes the load function
// and the start time as arguments so it never reads fields the caller may change.
func (c *addrCache) refreshInBackground(load func() []netip.Addr, started time.Time) {
	fresh := load()
	c.mu.Lock()
	defer c.mu.Unlock()
	c.store(fresh, started)
	c.refreshing = false
}

// store keeps fresh unless it is empty and a previous non-empty value exists.
// The caller holds c.mu.
func (c *addrCache) store(fresh []netip.Addr, fetchedAt time.Time) {
	if len(fresh) > 0 || len(c.value) == 0 {
		c.value = fresh
	}
	c.fetched, c.loaded = fetchedAt, true
}

func (c *addrCache) reset() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.value, c.loaded = nil, false
}

// hostSideCache caches orthrus.HostSideAddrs for hostAddrsTTL. It calls
// hostSideAddrs so tests can replace the source.
var hostSideCache = &addrCache{ttl: hostAddrsTTL, now: time.Now, load: func() []netip.Addr { return hostSideAddrs() }}

// gatewayCache caches the gateway name addresses for hostAddrsTTL.
var gatewayCache = &addrCache{ttl: hostAddrsTTL, now: time.Now, load: resolveGatewayAddrs}

// cachedHostSideAddrs returns the host-side addresses from the cache.
func cachedHostSideAddrs() []netip.Addr {
	return hostSideCache.get()
}

// gatewayAllowAddrs returns the exact addresses the gateway names map to. They
// are added to the allow list and to the this-machine set; the always-blocked
// list still applies to them.
func gatewayAllowAddrs() []netip.Addr {
	return gatewayCache.get()
}

// resetProbeAddrCaches drops the cached snapshots. Tests use it after replacing a source.
func resetProbeAddrCaches() {
	hostSideCache.reset()
	gatewayCache.reset()
}

// resolveGatewayAddrs collects the addresses of the gateway names from the hosts
// file and from a bounded lookup of each name.
func resolveGatewayAddrs() []netip.Addr {
	out := hostsFileAddrs(hostsFilePath, gatewayNames)
	for _, name := range gatewayNames {
		ctx, cancel := context.WithTimeout(context.Background(), gatewayLookupTimeout)
		addrs, err := lookupForwardHost(ctx, name)
		cancel()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			if ip, ok := netip.AddrFromSlice(a.IP); ok {
				out = append(out, ip.Unmap())
			}
		}
	}
	return dedupeAddrs(out)
}

// hostsFileAddrs returns the addresses that the hosts file at path maps to any of names.
// A missing or unreadable file yields nothing.
func hostsFileAddrs(path string, names []string) []netip.Addr {
	f, err := os.Open(path) // #nosec G304 -- fixed system hosts file path, overridden only by tests
	if err != nil {
		return nil
	}
	defer func() { _ = f.Close() }()
	wanted := make(map[string]struct{}, len(names))
	for _, n := range names {
		wanted[n] = struct{}{}
	}
	var out []netip.Addr
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if i := strings.IndexByte(line, '#'); i >= 0 {
			line = line[:i]
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		matched := false
		for _, name := range fields[1:] {
			if _, ok := wanted[strings.ToLower(name)]; ok {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		if ip, err := netip.ParseAddr(fields[0]); err == nil {
			out = append(out, ip.WithZone("").Unmap())
		}
	}
	return out
}

// dedupeAddrs removes repeated addresses and keeps the first occurrence.
func dedupeAddrs(in []netip.Addr) []netip.Addr {
	seen := make(map[netip.Addr]struct{}, len(in))
	out := in[:0]
	for _, a := range in {
		if _, dup := seen[a]; dup {
			continue
		}
		seen[a] = struct{}{}
		out = append(out, a)
	}
	return out
}

// probePolicy is the address policy for probes and forward targets. Loopback,
// RFC 1918, CGNAT, unique-local, fake-IP and the gateway addresses are allowed;
// link-local, instance services and every other reserved range are refused.
func probePolicy() network.AddressPolicy {
	return network.AddressPolicy{
		AllowLocalhost:    true,
		AllowRFC1918:      true,
		AllowCGNAT:        true,
		AllowUniqueLocal:  true,
		AllowBenchmarking: true,
		AllowExact:        gatewayAllowAddrs(),
	}
}

// ProbeHTTPOptions returns the network options for an HTTP probe client: the
// probe policy plus the reserved-listener guard. The address policy is read at each dial.
func ProbeHTTPOptions(db *gorm.DB) []network.Option {
	return []network.Option{
		network.WithAllowLocalhost(),
		network.WithAllowRFC1918(),
		network.WithAllowCGNAT(),
		network.WithAllowUniqueLocal(),
		network.WithAllowBenchmarking(),
		network.WithAllowExact(gatewayAllowAddrs),
		network.WithDialGuard(NewReservedListenerGuard(db).Check),
	}
}

// NewProbeDialer returns a TCP dialer for probes. Each literal address is
// checked when it is connected to.
func NewProbeDialer(db *gorm.DB, timeout time.Duration) *net.Dialer {
	return network.NewGuardedDialer(timeout, probePolicy, NewReservedListenerGuard(db).Check)
}

// listenPortFrom returns the port of a host:port listen address or of a URL
// with a host and port. It returns 0 for anything without a valid port, which
// matches no forward target.
func listenPortFrom(address string) int {
	if u, err := url.Parse(strings.TrimSpace(address)); err == nil && u.Scheme != "" && u.Host != "" {
		address = u.Host
	}
	_, portStr, err := net.SplitHostPort(address)
	if err != nil {
		return 0
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port < 0 || port > 65535 {
		return 0
	}
	return port
}

// ErrConnectionTestFailed is returned when a connection test does not connect to its
// target. It carries no address, port or cause: the caller shows it to every signed-in user.
var ErrConnectionTestFailed = errors.New("connection test failed")

// CheckProbeTarget is the save-time check of a probe destination (a remote server
// test or an uptime monitor). It applies the forward target rules to host:port:
// refused literal addresses, names whose resolved addresses are refused, and
// reserved ports on this machine. A name whose lookup fails is accepted, because
// the dialer checks the address it actually connects to. It returns
// ErrDestinationRefused for any refusal; other errors are returned as they are.
func CheckProbeTarget(ctx context.Context, db *gorm.DB, host string, port int) error {
	verdicts := preflightForwardNames(ctx, []string{host})
	err := refuseForwardTarget(db, host, port, verdicts)
	switch {
	case err == nil:
		return nil
	case isPolicyRefusal(err):
		return ErrDestinationRefused
	default:
		return fmt.Errorf("check probe destination: %w", err)
	}
}
