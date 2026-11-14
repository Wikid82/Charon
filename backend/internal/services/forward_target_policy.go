package services

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strconv"
	"strings"
	"sync/atomic"
	"time"

	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/orthrus"
)

// ErrOrthrusListenerTarget is returned when a forward target names this machine
// on a port Charon uses for an agent proxy. It applies to every caller.
var ErrOrthrusListenerTarget = errors.New("forward target uses a port reserved by this proxy")

// ErrAgentForwardTarget is returned for a forward target in the agent-name form.
// It is neutral by design and applies to every caller.
var ErrAgentForwardTarget = errors.New("forward target is not permitted")

// orthrusAgentTargetPrefix marks a forward host that names an agent. That form is
// refused for every caller and every stored row.
const orthrusAgentTargetPrefix = "orthrus:"

// forwardResolveTimeout bounds the name lookup used for a forward host that is
// not an IP literal. The lookup runs only when the port is a listener port.
const forwardResolveTimeout = 2 * time.Second

// lookupForwardHost resolves a host name. Tests replace it.
var lookupForwardHost = func(ctx context.Context, name string) ([]net.IPAddr, error) {
	return net.DefaultResolver.LookupIPAddr(ctx, name)
}

// gatewayNames are names that always reach the Docker host. A listener port can be
// reached through them, so they count as this machine whatever they resolve to.
var gatewayNames = []string{"host.docker.internal"}

// hostSideAddrs lists the host-side addresses of this container's networks. Tests replace it.
var hostSideAddrs = orthrus.HostSideAddrs

// OrthrusListenerSource reports the live loopback and external proxy ports of
// connected agents. *orthrus.OrthrusServer satisfies it.
type OrthrusListenerSource interface {
	LiveListenerPorts() []int
}

// orthrusListenerSource holds the live source set at startup. An unset source
// means only the persisted external proxy ports are checked.
var orthrusListenerSource atomic.Pointer[OrthrusListenerSource]

// SetOrthrusListenerSource sets the live listener source. nil clears it.
func SetOrthrusListenerSource(src OrthrusListenerSource) {
	if src == nil {
		orthrusListenerSource.Store(nil)
		return
	}
	orthrusListenerSource.Store(&src)
}

// liveListenerPorts returns the live listener ports, or nil when no source is set.
func liveListenerPorts() []int {
	src := orthrusListenerSource.Load()
	if src == nil || *src == nil {
		return nil
	}
	return (*src).LiveListenerPorts()
}

// isMissingTableError reports whether err means the agent table is not created. Only
// that case is read as "no stored agents"; any other failure is returned.
func isMissingTableError(err error) bool {
	return strings.Contains(err.Error(), "no such table")
}

// listenerPortSet returns the ports Charon listens on for agent proxies: every
// persisted external proxy port, whether or not the agent is connected, plus the
// live loopback and external ports.
func listenerPortSet(tx *gorm.DB) (map[int]struct{}, error) {
	ports := make(map[int]struct{})
	var stored []int
	err := tx.Model(&models.OrthrusAgent{}).
		Where("external_proxy_port > ?", 0).
		Pluck("external_proxy_port", &stored).Error
	if err != nil && !isMissingTableError(err) {
		return nil, fmt.Errorf("load external proxy ports: %w", err)
	}
	for _, p := range stored {
		ports[p] = struct{}{}
	}
	for _, p := range liveListenerPorts() {
		if p > 0 {
			ports[p] = struct{}{}
		}
	}
	return ports, nil
}

// isAgentForwardTarget reports whether a forward host uses the agent-name form.
// Nothing supported uses that form, so it is refused for every caller and the
// generator never resolves it.
func isAgentForwardTarget(forwardHost string) bool {
	return strings.HasPrefix(strings.TrimSpace(forwardHost), orthrusAgentTargetPrefix)
}

// refusesListenerTarget reports whether a forward target must not be used: it is
// an agent-name target, or it names this machine on a listener port. The port is
// checked first, so the name lookup runs only for the few targets that could
// matter.
func refusesListenerTarget(tx *gorm.DB, forwardHost string, port int) (bool, error) {
	if isAgentForwardTarget(forwardHost) {
		return true, nil
	}
	if port <= 0 {
		return false, nil
	}
	ports, err := listenerPortSet(tx)
	if err != nil {
		return false, err
	}
	if _, listening := ports[port]; !listening {
		return false, nil
	}

	host := normalizeForwardName(forwardTargetHost(forwardHost))
	if host == "" {
		return false, nil
	}
	if namesThisMachine(host) {
		return true, nil
	}
	if ip := parseLegacyIPv4(host); ip != nil {
		return isThisMachineAddr(ip), nil
	}
	if ip := net.ParseIP(host); ip != nil {
		return isThisMachineAddr(ip), nil
	}
	return resolvesToThisMachine(host), nil
}

// isThisMachineAddr reports whether an address is this machine: unspecified, a
// loopback or interface address, or a host-side address of an attached network.
func isThisMachineAddr(ip net.IP) bool {
	if ip.IsUnspecified() || localHostChecker.Contains(ip) {
		return true
	}
	addr, ok := netip.AddrFromSlice(ip)
	if !ok {
		return false
	}
	addr = addr.Unmap()
	for _, side := range hostSideAddrs() {
		if side.Unmap() == addr {
			return true
		}
	}
	return false
}

// namesThisMachine reports whether a normalized host is this machine by name or
// address, or is one of the Docker gateway names.
func namesThisMachine(host string) bool {
	if localHostChecker.IsLocalHost(host) {
		return true
	}
	for _, name := range gatewayNames {
		if host == name {
			return true
		}
	}
	return false
}

// resolvesToThisMachine reports whether a name resolves to this machine. A name
// that cannot be resolved is refused: a listener port must not be reachable
// through a name whose address Charon cannot establish. It runs only for
// listener ports, so the refusal is narrow.
func resolvesToThisMachine(name string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), forwardResolveTimeout)
	defer cancel()
	addrs, err := lookupForwardHost(ctx, name)
	if err != nil || len(addrs) == 0 {
		return true
	}
	for _, a := range addrs {
		if isThisMachineAddr(a.IP) {
			return true
		}
	}
	return false
}

// normalizeForwardName lowercases a host, drops IPv6 brackets and a trailing dot,
// so that equivalent spellings compare equal.
func normalizeForwardName(host string) string {
	host = strings.TrimSpace(host)
	host = strings.TrimSuffix(strings.Trim(host, "[]"), ".")
	return strings.ToLower(host)
}

// parseLegacyIPv4 reads an IPv4 address written in the shorthand forms that the
// system resolver accepts: one to four dot-separated parts, each decimal, octal
// (leading zero) or hexadecimal (0x prefix). The last part fills the remaining
// bytes. It returns nil for anything else, including names.
func parseLegacyIPv4(host string) net.IP {
	if host == "" || strings.ContainsAny(host, "_") {
		return nil
	}
	parts := strings.Split(host, ".")
	if len(parts) > 4 {
		return nil
	}
	vals := make([]uint64, len(parts))
	for i, p := range parts {
		if !isLegacyNumber(p) {
			return nil
		}
		v, err := strconv.ParseUint(p, 0, 32)
		if err != nil {
			return nil
		}
		vals[i] = v
	}
	last := len(vals) - 1
	var n uint64
	for i := 0; i < last; i++ {
		if vals[i] > 255 {
			return nil
		}
		n |= vals[i] << uint(8*(3-i))
	}
	if vals[last] >= uint64(1)<<uint(8*(4-last)) {
		return nil
	}
	n |= vals[last]
	// Every byte was range-checked above, so each conversion is lossless.
	return net.IPv4(byte(n>>24), byte(n>>16), byte(n>>8), byte(n)) //nolint:gosec // bounded by the checks above

}

// isLegacyNumber reports whether s is made only of digits, hex letters and an x
// marker, so that strconv can judge its base.
func isLegacyNumber(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9', r >= 'a' && r <= 'f', r >= 'A' && r <= 'F', r == 'x', r == 'X':
		default:
			return false
		}
	}
	return true
}

// refuseOrthrusListenerTarget returns ErrOrthrusListenerTarget when the forward
// target reaches an agent listener. Lookup failures are returned wrapped.
func refuseOrthrusListenerTarget(tx *gorm.DB, forwardHost string, port int) error {
	if isAgentForwardTarget(forwardHost) {
		return ErrAgentForwardTarget
	}
	refused, err := refusesListenerTarget(tx, forwardHost, port)
	if err != nil {
		return fmt.Errorf("check forward target: %w", err)
	}
	if refused {
		return ErrOrthrusListenerTarget
	}
	return nil
}

// ListenerTargetGuard answers, for the config generator, whether a stored
// forward target must be left out. It implements caddy.ForwardTargetGuard.
type ListenerTargetGuard struct {
	db *gorm.DB
}

// NewListenerTargetGuard returns a guard backed by db.
func NewListenerTargetGuard(db *gorm.DB) *ListenerTargetGuard {
	return &ListenerTargetGuard{db: db}
}

// RefusesForwardTarget reports whether a stored forward target reaches an agent
// listener. A nil guard or one without a database refuses nothing.
func (g *ListenerTargetGuard) RefusesForwardTarget(host string, port int) (bool, error) {
	if g == nil || g.db == nil {
		return false, nil
	}
	return refusesListenerTarget(g.db, host, port)
}
