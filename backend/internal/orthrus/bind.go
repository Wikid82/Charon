package orthrus

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"os"
	"strconv"
	"strings"
)

// BindMode is the kind of listen setting an agent's external proxy uses.
type BindMode string

const (
	// BindModeAuto picks Charon's primary network address (see automaticBind).
	BindModeAuto BindMode = "auto"
	// BindModeAll listens on every interface (0.0.0.0 or ::).
	BindModeAll BindMode = "all"
	// BindModeAddress listens on one exact local address.
	BindModeAddress BindMode = "address"
	// BindModeSubnet listens on the first local address inside a prefix.
	BindModeSubnet BindMode = "subnet"
)

// BindSource records how the listen address was chosen.
type BindSource string

const (
	BindSourceDefaultRoute     BindSource = "default_route"
	BindSourceSingleInterface  BindSource = "single_interface"
	BindSourceLoopbackFallback BindSource = "loopback_fallback"
	BindSourceConfigured       BindSource = "configured"
)

// procNetRoutePath is the kernel routing table. It is a fixed kernel interface,
// not operator configuration, so it is not read from internal/config.
const procNetRoutePath = "/proc/net/route"

// Route flags from linux/route.h.
const (
	rtfUp      = 0x1
	rtfGateway = 0x2
)

// LocalNetwork is one address Charon can bind the external proxy to.
type LocalNetwork struct {
	Interface string `json:"interface"`
	Address   string `json:"address"`
	CIDR      string `json:"cidr"`
	Primary   bool   `json:"primary"`
}

// netEnv reads host networking state. Every field is injected so tests never
// read the real host.
type netEnv struct {
	interfaces func() ([]net.Interface, error)
	addrs      func(net.Interface) ([]net.Addr, error)
	routeTable func() ([]byte, error) // contents of /proc/net/route; error off Linux
}

// systemNetEnv reads the real host.
func systemNetEnv() netEnv {
	return netEnv{
		interfaces: net.Interfaces,
		addrs:      func(ifc net.Interface) ([]net.Addr, error) { return ifc.Addrs() },
		routeTable: func() ([]byte, error) { return os.ReadFile(procNetRoutePath) },
	}
}

// localAddr is one address on a running interface.
type localAddr struct {
	iface  string
	addr   netip.Addr
	prefix netip.Prefix // masked
}

// ParseBindSetting validates a stored or submitted listen setting.
// "" is automatic; "0.0.0.0" and "::" are all interfaces; an IP literal is one
// address; a prefix is a subnet and is returned masked.
func ParseBindSetting(s string) (mode BindMode, addr netip.Addr, prefix netip.Prefix, err error) {
	switch s {
	case "":
		return BindModeAuto, netip.Addr{}, netip.Prefix{}, nil
	case "0.0.0.0":
		return BindModeAll, netip.IPv4Unspecified(), netip.Prefix{}, nil
	case "::":
		return BindModeAll, netip.IPv6Unspecified(), netip.Prefix{}, nil
	}
	if strings.Contains(s, "/") {
		p, perr := netip.ParsePrefix(s)
		if perr != nil {
			return "", netip.Addr{}, netip.Prefix{}, fmt.Errorf("orthrus: invalid bind subnet: %w", perr)
		}
		return BindModeSubnet, netip.Addr{}, p.Masked(), nil
	}
	a, aerr := netip.ParseAddr(s)
	if aerr != nil {
		return "", netip.Addr{}, netip.Prefix{}, fmt.Errorf("orthrus: invalid bind address: %w", aerr)
	}
	if a.Zone() != "" {
		return "", netip.Addr{}, netip.Prefix{}, errors.New("orthrus: bind address must not carry a zone")
	}
	return BindModeAddress, a, netip.Prefix{}, nil
}

// usableAddr reports whether a is a unicast address a proxy can sensibly
// advertise. Loopback, link-local, multicast and unspecified are excluded.
func usableAddr(a netip.Addr) bool {
	return a.IsValid() && a.IsGlobalUnicast()
}

// localAddrs lists every address on running interfaces.
func (e netEnv) localAddrs() ([]localAddr, error) {
	ifaces, err := e.interfaces()
	if err != nil {
		return nil, fmt.Errorf("orthrus: list interfaces: %w", err)
	}
	var out []localAddr
	for _, ifc := range ifaces {
		if ifc.Flags&net.FlagUp == 0 {
			continue
		}
		addrs, err := e.addrs(ifc)
		if err != nil {
			return nil, fmt.Errorf("orthrus: list addresses of %s: %w", ifc.Name, err)
		}
		for _, a := range addrs {
			p, err := netip.ParsePrefix(a.String())
			if err != nil {
				continue
			}
			addr := p.Addr().Unmap()
			out = append(out, localAddr{
				iface:  ifc.Name,
				addr:   addr,
				prefix: netip.PrefixFrom(addr, p.Bits()).Masked(),
			})
		}
	}
	return out, nil
}

// primaryRouteInterface returns the interface of the lowest-metric IPv4 default
// route in a /proc/net/route table.
func primaryRouteInterface(table []byte) (string, bool) {
	best, bestMetric := "", uint64(0)
	scanner := bufio.NewScanner(bytes.NewReader(table))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 8 || fields[1] != "00000000" {
			continue
		}
		flags, err := strconv.ParseUint(fields[3], 16, 32)
		if err != nil || flags&rtfUp == 0 || flags&rtfGateway == 0 {
			continue
		}
		metric, err := strconv.ParseUint(fields[6], 10, 32)
		if err != nil {
			continue
		}
		if best == "" || metric < bestMetric {
			best, bestMetric = fields[0], metric
		}
	}
	return best, best != ""
}

// isDockerHostNetwork reports whether the interface list shows the host's
// Docker bridges. Charon then shares the host namespace, so "its own Docker
// network" does not exist.
func isDockerHostNetwork(ifaces []net.Interface) bool {
	for _, ifc := range ifaces {
		if ifc.Name == "docker0" || strings.HasPrefix(ifc.Name, "br-") {
			return true
		}
	}
	return false
}

// firstAddrOf returns the preferred usable address of one interface: IPv4 first,
// then IPv6.
func firstAddrOf(addrs []localAddr, iface string) (netip.Addr, bool) {
	var v6 netip.Addr
	for _, la := range addrs {
		if la.iface != iface || !usableAddr(la.addr) {
			continue
		}
		if la.addr.Is4() {
			return la.addr, true
		}
		if !v6.IsValid() {
			v6 = la.addr
		}
	}
	return v6, v6.IsValid()
}

// automaticBind chooses the address for the "auto" setting:
//  1. the interface of the default route (lowest metric), if it has a usable address;
//  2. otherwise the single interface with a usable address, when no default route exists;
//  3. otherwise loopback, which is never a wildcard.
//
// Host networking (docker0 or br- bridges present) and any unreadable state skip to 3.
func (e netEnv) automaticBind() (netip.Addr, BindSource) {
	fallback := netip.AddrFrom4([4]byte{127, 0, 0, 1})

	ifaces, err := e.interfaces()
	if err != nil || isDockerHostNetwork(ifaces) {
		return fallback, BindSourceLoopbackFallback
	}
	table, err := e.routeTable()
	if err != nil {
		return fallback, BindSourceLoopbackFallback
	}
	locals, err := e.localAddrs()
	if err != nil {
		return fallback, BindSourceLoopbackFallback
	}

	iface, hasDefault := primaryRouteInterface(table)
	if hasDefault {
		if addr, ok := firstAddrOf(locals, iface); ok {
			return addr, BindSourceDefaultRoute
		}
		return fallback, BindSourceLoopbackFallback
	}
	if addr, ok := singleUsableInterface(locals); ok {
		return addr, BindSourceSingleInterface
	}
	return fallback, BindSourceLoopbackFallback
}

// singleUsableInterface returns the first usable address when exactly one
// interface has one.
func singleUsableInterface(locals []localAddr) (netip.Addr, bool) {
	var found netip.Addr
	var foundIface string
	for _, la := range locals {
		if !usableAddr(la.addr) {
			continue
		}
		if foundIface != "" && la.iface != foundIface {
			return netip.Addr{}, false
		}
		if foundIface == "" {
			foundIface = la.iface
			found = la.addr
		}
	}
	return found, foundIface != ""
}

// ResolveBind turns a listen setting into the address to bind. Explicit
// settings never fall back: an address that is not local, or a subnet with no
// local address, is an error.
func (e netEnv) ResolveBind(setting string) (netip.Addr, BindMode, BindSource, error) {
	mode, addr, prefix, err := ParseBindSetting(setting)
	if err != nil {
		return netip.Addr{}, "", "", err
	}

	switch mode {
	case BindModeAuto:
		resolved, source := e.automaticBind()
		return resolved, mode, source, nil

	case BindModeAll:
		return addr, mode, BindSourceConfigured, nil

	case BindModeAddress:
		locals, err := e.localAddrs()
		if err != nil {
			return netip.Addr{}, mode, BindSourceConfigured, err
		}
		for _, la := range locals {
			if la.addr == addr {
				return addr, mode, BindSourceConfigured, nil
			}
		}
		return netip.Addr{}, mode, BindSourceConfigured, fmt.Errorf("orthrus: %s is not a local address", addr)

	default: // BindModeSubnet
		locals, err := e.localAddrs()
		if err != nil {
			return netip.Addr{}, mode, BindSourceConfigured, err
		}
		for _, la := range locals {
			if usableAddr(la.addr) && prefix.Contains(la.addr) {
				return la.addr, mode, BindSourceConfigured, nil
			}
		}
		return netip.Addr{}, mode, BindSourceConfigured, fmt.Errorf("orthrus: no local address in %s", prefix)
	}
}

// LocalNetworks lists the networks Charon can bind on this host, with the
// automatic choice. Loopback and link-local addresses are omitted. The list is
// never nil, so it encodes as a JSON array.
func (e netEnv) LocalNetworks() ([]LocalNetwork, netip.Addr, BindSource) {
	auto, source := e.automaticBind()
	networks := []LocalNetwork{}

	locals, err := e.localAddrs()
	if err != nil {
		return networks, auto, source
	}
	for _, la := range locals {
		if !usableAddr(la.addr) {
			continue
		}
		networks = append(networks, LocalNetwork{
			Interface: la.iface,
			Address:   la.addr.String(),
			CIDR:      la.prefix.String(),
			Primary:   la.addr == auto,
		})
	}
	return networks, auto, source
}

// LocalNetworks is LocalNetworks on the real host.
func LocalNetworks() ([]LocalNetwork, netip.Addr, BindSource) {
	return systemNetEnv().LocalNetworks()
}

// HostSideAddrs returns the addresses that name the host side of the networks
// this process is attached to: each IPv4 default gateway from the route table,
// and the first address of each directly attached IPv4 network, where Docker
// places its bridge gateway. It returns nil when neither source can be read.
func HostSideAddrs() []netip.Addr {
	env := systemNetEnv()
	var out []netip.Addr
	if table, err := env.routeTable(); err == nil {
		out = append(out, defaultGatewayAddrs(table)...)
	}
	if locals, err := env.localAddrs(); err == nil {
		for _, l := range locals {
			if l.addr.Is4() && !l.addr.IsLoopback() {
				out = append(out, l.prefix.Masked().Addr().Next())
			}
		}
	}
	return out
}

// defaultGatewayAddrs returns the gateways of the IPv4 default routes in a
// /proc/net/route table. The table stores each address as little-endian hex.
func defaultGatewayAddrs(table []byte) []netip.Addr {
	var out []netip.Addr
	scanner := bufio.NewScanner(bytes.NewReader(table))
	for scanner.Scan() {
		fields := strings.Fields(scanner.Text())
		if len(fields) < 4 || fields[1] != "00000000" {
			continue
		}
		flags, err := strconv.ParseUint(fields[3], 16, 32)
		if err != nil || flags&rtfUp == 0 || flags&rtfGateway == 0 {
			continue
		}
		raw, err := hex.DecodeString(fields[2])
		if err != nil || len(raw) != 4 {
			continue
		}
		out = append(out, netip.AddrFrom4([4]byte{raw[3], raw[2], raw[1], raw[0]}))
	}
	return out
}
