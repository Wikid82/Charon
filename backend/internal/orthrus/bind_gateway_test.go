package orthrus

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDefaultGatewayAddrs_ReadsOnlyUpGatewayDefaultRoutes(t *testing.T) {
	table := []byte(`Iface	Destination	Gateway 	Flags	RefCnt	Use	Metric	Mask		MTU	Window	IRTT
eth0	00000000	0100A8C0	0003	0	0	100	00000000	0	0	0
eth1	00000000	010011AC	0003	0	0	200	00000000	0	0	0
eth2	0000A8C0	00000000	0001	0	0	0	00FFFFFF	0	0	0
eth3	00000000	0200A8C0	0001	0	0	0	00000000	0	0	0
eth4	00000000	0300A8C0	0002	0	0	0	00000000	0	0	0
eth5	00000000	ZZZZZZZZ	0003	0	0	0	00000000	0	0	0
short	00000000`)
	got := defaultGatewayAddrs(table)
	assert.Equal(t, []netip.Addr{
		netip.MustParseAddr("192.168.0.1"),
		netip.MustParseAddr("172.17.0.1"),
	}, got)
}

func TestDefaultGatewayAddrs_EmptyTable(t *testing.T) {
	assert.Empty(t, defaultGatewayAddrs(nil))
}

func TestHostSideAddrs_ReturnsWithoutPanic(t *testing.T) {
	for _, a := range HostSideAddrs() {
		assert.True(t, a.IsValid())
	}
}
