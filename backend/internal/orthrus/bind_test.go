package orthrus

import (
	"errors"
	"net"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fakeAddr is a net.Addr whose String is CIDR notation, the same shape
// net.IPNet produces for interface addresses.
type fakeAddr string

func (a fakeAddr) Network() string { return "ip+net" }
func (a fakeAddr) String() string  { return string(a) }

// fakeIface describes one interface for fakeEnv.
type fakeIface struct {
	name  string
	flags net.Flags
	addrs []string // CIDR notation, host address kept
}

const (
	routeTableHeader = "Iface\tDestination\tGateway \tFlags\tRefCnt\tUse\tMetric\tMask\tMTU\tWindow\tIRTT\n"
	// Flags 0003 = RTF_UP|RTF_GATEWAY; destination 00000000 = default route.
	defaultRouteEth0 = "eth0\t00000000\t0100A8C0\t0003\t0\t0\t100\t00000000\t0\t0\t0\n"
	defaultRouteEth1 = "eth1\t00000000\t0100090A\t0003\t0\t0\t600\t00000000\t0\t0\t0\n"
	subnetRouteEth0  = "eth0\t0000AC12\t00000000\t0001\t0\t0\t0\t0000F0FF\t0\t0\t0\n"
)

// fakeEnv returns a netEnv describing the given routes and interfaces, so no
// test reads the real host.
func fakeEnv(routes string, ifaces ...fakeIface) netEnv {
	nets := make([]net.Interface, 0, len(ifaces))
	addrsByName := make(map[string][]net.Addr, len(ifaces))
	for i, f := range ifaces {
		nets = append(nets, net.Interface{Index: i + 1, Name: f.name, Flags: f.flags})
		addrs := make([]net.Addr, 0, len(f.addrs))
		for _, a := range f.addrs {
			addrs = append(addrs, fakeAddr(a))
		}
		addrsByName[f.name] = addrs
	}
	return netEnv{
		interfaces: func() ([]net.Interface, error) { return nets, nil },
		addrs: func(ifc net.Interface) ([]net.Addr, error) {
			return addrsByName[ifc.Name], nil
		},
		routeTable: func() ([]byte, error) { return []byte(routes), nil },
	}
}

// up is the flag set of a running interface.
const up = net.FlagUp

func TestParseBindSetting(t *testing.T) {
	tests := []struct {
		name    string
		in      string
		mode    BindMode
		addr    string
		prefix  string
		wantErr bool
	}{
		{name: "automatic", in: "", mode: BindModeAuto},
		{name: "all IPv4", in: "0.0.0.0", mode: BindModeAll, addr: "0.0.0.0"},
		{name: "all IPv6", in: "::", mode: BindModeAll, addr: "::"},
		{name: "address", in: "10.0.0.5", mode: BindModeAddress, addr: "10.0.0.5"},
		{name: "address IPv6", in: "fd00::5", mode: BindModeAddress, addr: "fd00::5"},
		{name: "subnet is masked", in: "10.0.0.7/24", mode: BindModeSubnet, prefix: "10.0.0.0/24"},
		{name: "zone rejected", in: "fe80::1%eth0", wantErr: true},
		{name: "garbage address", in: "not-an-ip", wantErr: true},
		{name: "hostname", in: "localhost", wantErr: true},
		{name: "bad prefix length", in: "10.0.0.0/33", wantErr: true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			mode, addr, prefix, err := ParseBindSetting(tc.in)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.mode, mode)
			if tc.addr != "" {
				assert.Equal(t, tc.addr, addr.String())
			}
			if tc.prefix != "" {
				assert.Equal(t, tc.prefix, prefix.String())
			}
		})
	}
}

func TestResolveBind_Automatic(t *testing.T) {
	tests := []struct {
		name       string
		env        netEnv
		wantAddr   string
		wantSource BindSource
	}{
		{
			name: "bridge container uses default route interface",
			env: fakeEnv(routeTableHeader+defaultRouteEth0,
				fakeIface{name: "lo", flags: up | net.FlagLoopback, addrs: []string{"127.0.0.1/8"}},
				fakeIface{name: "eth0", flags: up, addrs: []string{"172.18.0.4/16"}},
			),
			wantAddr:   "172.18.0.4",
			wantSource: BindSourceDefaultRoute,
		},
		{
			name: "lowest metric default route wins",
			env: fakeEnv(routeTableHeader+defaultRouteEth1+defaultRouteEth0,
				fakeIface{name: "eth0", flags: up, addrs: []string{"172.18.0.4/16"}},
				fakeIface{name: "eth1", flags: up, addrs: []string{"10.9.0.3/24"}},
			),
			wantAddr:   "172.18.0.4",
			wantSource: BindSourceDefaultRoute,
		},
		{
			name: "IPv4 preferred over IPv6 on the primary interface",
			env: fakeEnv(routeTableHeader+defaultRouteEth0,
				fakeIface{name: "eth0", flags: up, addrs: []string{"fd00::5/64", "172.18.0.4/16"}},
			),
			wantAddr:   "172.18.0.4",
			wantSource: BindSourceDefaultRoute,
		},
		{
			name: "IPv6 used when the primary interface has no IPv4",
			env: fakeEnv(routeTableHeader+defaultRouteEth0,
				fakeIface{name: "eth0", flags: up, addrs: []string{"fe80::1/64", "fd00::5/64"}},
			),
			wantAddr:   "fd00::5",
			wantSource: BindSourceDefaultRoute,
		},
		{
			name: "internal network without default route uses the single interface",
			env: fakeEnv(routeTableHeader+subnetRouteEth0,
				fakeIface{name: "lo", flags: up | net.FlagLoopback, addrs: []string{"127.0.0.1/8"}},
				fakeIface{name: "eth0", flags: up, addrs: []string{"172.19.0.2/16"}},
			),
			wantAddr:   "172.19.0.2",
			wantSource: BindSourceSingleInterface,
		},
		{
			name: "host network with docker0 falls back to loopback",
			env: fakeEnv(routeTableHeader+defaultRouteEth0,
				fakeIface{name: "docker0", flags: up, addrs: []string{"172.17.0.1/16"}},
				fakeIface{name: "eth0", flags: up, addrs: []string{"192.168.1.10/24"}},
			),
			wantAddr:   "127.0.0.1",
			wantSource: BindSourceLoopbackFallback,
		},
		{
			name: "host network with a br- bridge falls back to loopback",
			env: fakeEnv(routeTableHeader+defaultRouteEth0,
				fakeIface{name: "br-1a2b3c", flags: up, addrs: []string{"172.20.0.1/16"}},
				fakeIface{name: "eth0", flags: up, addrs: []string{"192.168.1.10/24"}},
			),
			wantAddr:   "127.0.0.1",
			wantSource: BindSourceLoopbackFallback,
		},
		{
			name: "several interfaces and no default route fall back to loopback",
			env: fakeEnv(routeTableHeader+subnetRouteEth0,
				fakeIface{name: "eth0", flags: up, addrs: []string{"172.19.0.2/16"}},
				fakeIface{name: "eth1", flags: up, addrs: []string{"10.9.0.3/24"}},
			),
			wantAddr:   "127.0.0.1",
			wantSource: BindSourceLoopbackFallback,
		},
		{
			name: "unreadable route table falls back to loopback",
			env: netEnv{
				interfaces: func() ([]net.Interface, error) {
					return []net.Interface{{Name: "eth0", Flags: up}}, nil
				},
				addrs: func(net.Interface) ([]net.Addr, error) {
					return []net.Addr{fakeAddr("172.19.0.2/16")}, nil
				},
				routeTable: func() ([]byte, error) { return nil, errors.New("no such file") },
			},
			wantAddr:   "127.0.0.1",
			wantSource: BindSourceLoopbackFallback,
		},
		{
			name: "interface listing error falls back to loopback",
			env: netEnv{
				interfaces: func() ([]net.Interface, error) { return nil, errors.New("denied") },
				addrs:      func(net.Interface) ([]net.Addr, error) { return nil, nil },
				routeTable: func() ([]byte, error) { return []byte(routeTableHeader + defaultRouteEth0), nil },
			},
			wantAddr:   "127.0.0.1",
			wantSource: BindSourceLoopbackFallback,
		},
		{
			name: "address lookup error on every interface falls back to loopback",
			env: netEnv{
				interfaces: func() ([]net.Interface, error) {
					return []net.Interface{{Name: "eth0", Flags: up}}, nil
				},
				addrs:      func(net.Interface) ([]net.Addr, error) { return nil, errors.New("boom") },
				routeTable: func() ([]byte, error) { return []byte(routeTableHeader + defaultRouteEth0), nil },
			},
			wantAddr:   "127.0.0.1",
			wantSource: BindSourceLoopbackFallback,
		},
		{
			name: "no usable address falls back to loopback",
			env: fakeEnv(routeTableHeader+defaultRouteEth0,
				fakeIface{name: "eth0", flags: up, addrs: []string{"fe80::1/64"}},
				fakeIface{name: "eth1", flags: 0, addrs: []string{"10.9.0.3/24"}},
			),
			wantAddr:   "127.0.0.1",
			wantSource: BindSourceLoopbackFallback,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			addr, mode, source, err := tc.env.ResolveBind("")
			require.NoError(t, err)
			assert.Equal(t, tc.wantAddr, addr.String())
			assert.Equal(t, BindModeAuto, mode)
			assert.Equal(t, tc.wantSource, source)
		})
	}
}

func TestResolveBind_Explicit(t *testing.T) {
	env := fakeEnv(routeTableHeader+defaultRouteEth0,
		fakeIface{name: "lo", flags: up | net.FlagLoopback, addrs: []string{"127.0.0.1/8"}},
		fakeIface{name: "eth0", flags: up, addrs: []string{"172.18.0.4/16", "fd00::5/64"}},
		fakeIface{name: "eth1", flags: 0, addrs: []string{"10.9.0.3/24"}},
	)

	tests := []struct {
		name     string
		setting  string
		wantAddr string
		wantMode BindMode
		wantErr  string
	}{
		{name: "wildcard IPv4", setting: "0.0.0.0", wantAddr: "0.0.0.0", wantMode: BindModeAll},
		{name: "wildcard IPv6", setting: "::", wantAddr: "::", wantMode: BindModeAll},
		{name: "local address", setting: "172.18.0.4", wantAddr: "172.18.0.4", wantMode: BindModeAddress},
		{name: "loopback address is local", setting: "127.0.0.1", wantAddr: "127.0.0.1", wantMode: BindModeAddress},
		{name: "address on a down interface is not local", setting: "10.9.0.3", wantMode: BindModeAddress, wantErr: "not a local address"},
		{name: "foreign address is refused", setting: "10.1.1.1", wantMode: BindModeAddress, wantErr: "10.1.1.1 is not a local address"},
		{name: "subnet picks the local address", setting: "172.18.0.0/16", wantAddr: "172.18.0.4", wantMode: BindModeSubnet},
		{name: "IPv6 subnet picks the local address", setting: "fd00::/64", wantAddr: "fd00::5", wantMode: BindModeSubnet},
		{name: "subnet without a local address is refused", setting: "192.0.2.0/24", wantMode: BindModeSubnet, wantErr: "no local address in 192.0.2.0/24"},
		{name: "unparsable setting", setting: "not-an-ip", wantErr: "invalid"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			addr, mode, source, err := env.ResolveBind(tc.setting)
			if tc.wantErr != "" {
				require.Error(t, err)
				assert.Contains(t, err.Error(), tc.wantErr)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tc.wantAddr, addr.String())
			assert.Equal(t, tc.wantMode, mode)
			assert.Equal(t, BindSourceConfigured, source)
		})
	}
}

func TestResolveBind_ExplicitAddressInterfaceErrors(t *testing.T) {
	env := netEnv{
		interfaces: func() ([]net.Interface, error) { return nil, errors.New("denied") },
		addrs:      func(net.Interface) ([]net.Addr, error) { return nil, nil },
		routeTable: func() ([]byte, error) { return nil, errors.New("unused") },
	}
	_, _, _, err := env.ResolveBind("172.18.0.4")
	require.Error(t, err)

	_, _, _, err = env.ResolveBind("10.0.0.0/8")
	require.Error(t, err)
}

func TestLocalNetworks(t *testing.T) {
	env := fakeEnv(routeTableHeader+defaultRouteEth0,
		fakeIface{name: "lo", flags: up | net.FlagLoopback, addrs: []string{"127.0.0.1/8", "::1/128"}},
		fakeIface{name: "eth0", flags: up, addrs: []string{"172.18.0.4/16", "fe80::1/64"}},
		fakeIface{name: "eth1", flags: up, addrs: []string{"10.9.0.3/24"}},
		fakeIface{name: "eth2", flags: 0, addrs: []string{"192.168.5.5/24"}},
	)

	networks, auto, source := env.LocalNetworks()

	assert.Equal(t, "172.18.0.4", auto.String())
	assert.Equal(t, BindSourceDefaultRoute, source)
	assert.Equal(t, []LocalNetwork{
		{Interface: "eth0", Address: "172.18.0.4", CIDR: "172.18.0.0/16", Primary: true},
		{Interface: "eth1", Address: "10.9.0.3", CIDR: "10.9.0.0/24", Primary: false},
	}, networks)
}

func TestLocalNetworks_FallbackWithoutInterfaces(t *testing.T) {
	env := netEnv{
		interfaces: func() ([]net.Interface, error) { return nil, errors.New("denied") },
		addrs:      func(net.Interface) ([]net.Addr, error) { return nil, nil },
		routeTable: func() ([]byte, error) { return nil, errors.New("unused") },
	}

	networks, auto, source := env.LocalNetworks()

	assert.NotNil(t, networks, "networks must encode as an array, not null")
	assert.Empty(t, networks)
	assert.Equal(t, netip.MustParseAddr("127.0.0.1"), auto)
	assert.Equal(t, BindSourceLoopbackFallback, source)
}

func TestLocalNetworks_HostEnvironment(t *testing.T) {
	// Reads the real host. Only checks the invariants that hold anywhere.
	networks, auto, source := LocalNetworks()

	assert.True(t, auto.IsValid())
	assert.NotEmpty(t, source)
	assert.NotNil(t, networks)
}
