package upstreamdns

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/dns/dnsmessage"
)

// fakeLookup answers from a table. Unknown names are not found.
func fakeLookup(table map[string][]netip.Addr) LookupFunc {
	return func(_ context.Context, _, name string) ([]netip.Addr, error) {
		if addrs, ok := table[strings.ToLower(name)]; ok {
			return addrs, nil
		}
		return nil, &net.DNSError{Err: "no such host", Name: name, IsNotFound: true}
	}
}

// startServer starts a server on real loopback sockets and closes it at cleanup.
func startServer(t *testing.T, cfg Config) *Server {
	t.Helper()
	if cfg.Listeners == nil {
		cfg.Listeners = []Listener{
			{Name: "general", Filter: refuseLinkLocal},
			{Name: "reserved", Filter: func(a netip.Addr) bool { return refuseLinkLocal(a) || a.IsLoopback() }},
		}
	}
	s, err := Start(cfg)
	require.NoError(t, err)
	t.Cleanup(func() { _ = s.Close() })
	return s
}

// udpExchange sends one packet to the listener and returns the reply, or nil on timeout.
func udpExchange(t testing.TB, s *Server, listener string, packet []byte, wait time.Duration) []byte {
	t.Helper()
	addr, ok := s.Address(listener)
	require.True(t, ok)
	conn, err := net.Dial("udp", strings.TrimPrefix(addr, "udp/"))
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()
	require.NoError(t, conn.SetDeadline(time.Now().Add(wait)))
	_, err = conn.Write(packet)
	require.NoError(t, err)
	buf := make([]byte, 1500)
	n, err := conn.Read(buf)
	if err != nil {
		return nil
	}
	return buf[:n]
}

func TestServer_AddressAndLoopbackBinding(t *testing.T) {
	t.Parallel()
	s := startServer(t, Config{Lookup: fakeLookup(nil)})
	general, ok := s.Address("general")
	require.True(t, ok)
	reserved, ok := s.Address("reserved")
	require.True(t, ok)
	assert.NotEqual(t, general, reserved, "each listener has its own port")
	assert.True(t, strings.HasPrefix(general, "udp/127.0.0.1:"), general)
	_, ok = s.Address("missing")
	assert.False(t, ok)
	var nilServer *Server
	_, ok = nilServer.Address("general")
	assert.False(t, ok)
}

func TestStart_RequiresAListener(t *testing.T) {
	t.Parallel()
	_, err := Start(Config{})
	assert.Error(t, err)
}

func TestServer_CloseIsSafeOnNilAndTwice(t *testing.T) {
	t.Parallel()
	var nilServer *Server
	assert.NoError(t, nilServer.Close())
	s := startServer(t, Config{Lookup: fakeLookup(nil)})
	assert.NoError(t, s.Close())
	assert.NoError(t, s.Close())
}

func TestServer_AnswersFilteredAndRefusedAddresses(t *testing.T) {
	t.Parallel()
	table := map[string][]netip.Addr{
		"app.test.":   {netip.MustParseAddr("10.0.0.1"), netip.MustParseAddr("169.254.169.254")},
		"local.test.": {netip.MustParseAddr("127.0.0.1")},
	}
	var refused atomic.Int32
	s := startServer(t, Config{
		Lookup: fakeLookup(table),
		OnRefused: func(listener string, count int) {
			assert.Contains(t, []string{"general", "reserved"}, listener)
			refused.Add(int32(count)) // #nosec G115 -- small test counter
		},
	})

	reply := udpExchange(t, s, "general", buildQuery(t, 21, "app.test.", dnsmessage.TypeA, dnsmessage.ClassINET), 2*time.Second)
	require.NotNil(t, reply)
	assert.Equal(t, []netip.Addr{netip.MustParseAddr("10.0.0.1")}, parseReply(t, reply).answers)
	assert.Equal(t, int32(1), refused.Load())

	reply = udpExchange(t, s, "reserved", buildQuery(t, 22, "local.test.", dnsmessage.TypeA, dnsmessage.ClassINET), 2*time.Second)
	require.NotNil(t, reply)
	assert.Empty(t, parseReply(t, reply).answers, "the reserved listener also refuses this machine")
}

func TestServer_NotFoundAndLookupErrors(t *testing.T) {
	t.Parallel()
	var lookupErrors atomic.Int32
	s := startServer(t, Config{
		Lookup: func(_ context.Context, _, name string) ([]netip.Addr, error) {
			if name == "down.test." {
				return nil, errors.New("server misbehaving")
			}
			return fakeLookup(nil)(context.Background(), "ip", name)
		},
		OnLookupError: func(string) { lookupErrors.Add(1) },
	})

	reply := udpExchange(t, s, "general", buildQuery(t, 31, "missing.test.", dnsmessage.TypeA, dnsmessage.ClassINET), 2*time.Second)
	require.NotNil(t, reply)
	assert.Equal(t, dnsmessage.RCodeNameError, parseReply(t, reply).header.RCode)
	assert.Zero(t, lookupErrors.Load(), "a not found name is not a lookup error")

	reply = udpExchange(t, s, "general", buildQuery(t, 32, "down.test.", dnsmessage.TypeA, dnsmessage.ClassINET), 2*time.Second)
	require.NotNil(t, reply)
	assert.Equal(t, dnsmessage.RCodeServerFailure, parseReply(t, reply).header.RCode)
	assert.Equal(t, int32(1), lookupErrors.Load())
}

func TestServer_LookupTimeoutGivesServFail(t *testing.T) {
	t.Parallel()
	s := startServer(t, Config{
		LookupTimeout: 100 * time.Millisecond,
		Lookup: func(ctx context.Context, _, _ string) ([]netip.Addr, error) {
			<-ctx.Done()
			return nil, ctx.Err()
		},
	})
	start := time.Now()
	reply := udpExchange(t, s, "general", buildQuery(t, 41, "slow.test.", dnsmessage.TypeA, dnsmessage.ClassINET), 3*time.Second)
	require.NotNil(t, reply)
	assert.Equal(t, dnsmessage.RCodeServerFailure, parseReply(t, reply).header.RCode)
	assert.Less(t, time.Since(start), 2*time.Second)
}

func TestServer_DropsMalformedAndResponsePackets(t *testing.T) {
	t.Parallel()
	s := startServer(t, Config{Lookup: fakeLookup(nil)})
	assert.Nil(t, udpExchange(t, s, "general", []byte{1, 2, 3}, 300*time.Millisecond))
	valid := buildQuery(t, 51, "app.test.", dnsmessage.TypeA, dnsmessage.ClassINET)
	valid[2] |= 0x80
	assert.Nil(t, udpExchange(t, s, "general", valid, 300*time.Millisecond))
	mx := buildQuery(t, 52, "app.test.", dnsmessage.TypeMX, dnsmessage.ClassINET)
	assert.NotNil(t, udpExchange(t, s, "general", mx, 2*time.Second), "other types get an empty answer")
}

// clientResolver is the resolver Caddy builds for a transport resolver: every
// exchange goes to the configured address over UDP.
func clientResolver(address string) *net.Resolver {
	target := strings.TrimPrefix(address, "udp/")
	return &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "udp", target)
		},
	}
}

func TestServer_GoClientSeesFilteredAnswers(t *testing.T) {
	t.Parallel()
	table := map[string][]netip.Addr{
		"app.test.":       {netip.MustParseAddr("10.0.0.1"), netip.MustParseAddr("fd12::5")},
		"linklocal.test.": {netip.MustParseAddr("169.254.1.1")},
		"broken.test.":    nil,
	}
	s := startServer(t, Config{
		Lookup: func(ctx context.Context, network, name string) ([]netip.Addr, error) {
			if name == "broken.test." {
				return nil, errors.New("server failure")
			}
			return fakeLookup(table)(ctx, network, name)
		},
	})
	addr, ok := s.Address("general")
	require.True(t, ok)
	resolver := clientResolver(addr)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	got, err := resolver.LookupNetIP(ctx, "ip", "app.test.")
	require.NoError(t, err)
	assert.ElementsMatch(t, []netip.Addr{netip.MustParseAddr("10.0.0.1"), netip.MustParseAddr("fd12::5")}, got)

	_, err = resolver.LookupNetIP(ctx, "ip", "missing.test.")
	var dnsErr *net.DNSError
	require.ErrorAs(t, err, &dnsErr)
	assert.True(t, dnsErr.IsNotFound)

	_, err = resolver.LookupNetIP(ctx, "ip", "linklocal.test.")
	require.Error(t, err, "a name whose every address is refused does not connect")

	_, err = resolver.LookupNetIP(ctx, "ip", "broken.test.")
	require.Error(t, err)
	assert.False(t, errors.As(err, &dnsErr) && dnsErr.IsNotFound, "a server failure is not a missing name")
}

func TestServer_SameNameQueriesShareOneLookup(t *testing.T) {
	t.Parallel()
	var calls atomic.Int32
	release := make(chan struct{})
	s := startServer(t, Config{
		Lookup: func(_ context.Context, _, _ string) ([]netip.Addr, error) {
			calls.Add(1)
			<-release
			return []netip.Addr{netip.MustParseAddr("10.0.0.1")}, nil
		},
	})
	addr, _ := s.Address("general")
	resolver := clientResolver(addr)
	var wg sync.WaitGroup
	for range 3 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = resolver.LookupNetIP(context.Background(), "ip", "shared.test.")
		}()
	}
	waitFor(t, func() bool { return calls.Load() >= 1 })
	time.Sleep(50 * time.Millisecond)
	close(release)
	wg.Wait()
	assert.Equal(t, int32(1), calls.Load(), "A and AAAA queries for one name share a lookup")
}

func TestServer_FloodIsCappedAndRecovers(t *testing.T) {
	t.Parallel()
	var inFlight, peak atomic.Int32
	release := make(chan struct{})
	s := startServer(t, Config{
		MaxLookups: 4,
		MaxWaiting: 8,
		Lookup: func(ctx context.Context, _, _ string) ([]netip.Addr, error) {
			now := inFlight.Add(1)
			defer inFlight.Add(-1)
			for {
				old := peak.Load()
				if now <= old || peak.CompareAndSwap(old, now) {
					break
				}
			}
			select {
			case <-release:
				return []netip.Addr{netip.MustParseAddr("10.0.0.1")}, nil
			case <-ctx.Done():
				return nil, ctx.Err()
			}
		},
	})
	addr, _ := s.Address("general")
	conn, err := net.Dial("udp", strings.TrimPrefix(addr, "udp/"))
	require.NoError(t, err)
	defer func() { _ = conn.Close() }()
	for i := range 40 {
		name := fmt.Sprintf("n%02d.test.", i)
		packet := buildQuery(t, uint16(100+i), name, dnsmessage.TypeA, dnsmessage.ClassINET)
		_, err = conn.Write(packet)
		require.NoError(t, err)
	}
	waitFor(t, func() bool { return peak.Load() == 4 })
	close(release)
	require.NoError(t, conn.SetReadDeadline(time.Now().Add(2*time.Second)))
	buf := make([]byte, 1500)
	_, err = conn.Read(buf)
	require.NoError(t, err, "queries that fit under the caps still get replies")
	assert.LessOrEqual(t, peak.Load(), int32(4))
}

func TestServer_PanickingLookupGetsNoReplyAndNextQueryIsServed(t *testing.T) {
	t.Parallel()
	var panics atomic.Int32
	s := startServer(t, Config{
		OnPanic: func() { panics.Add(1) },
		Lookup: func(ctx context.Context, network, name string) ([]netip.Addr, error) {
			if name == "boom.test." {
				panic("lookup exploded")
			}
			return []netip.Addr{netip.MustParseAddr("10.0.0.1")}, nil
		},
	})
	assert.Nil(t, udpExchange(t, s, "general", buildQuery(t, 61, "boom.test.", dnsmessage.TypeA, dnsmessage.ClassINET), 500*time.Millisecond))
	reply := udpExchange(t, s, "general", buildQuery(t, 62, "app.test.", dnsmessage.TypeA, dnsmessage.ClassINET), 2*time.Second)
	require.NotNil(t, reply)
	assert.Len(t, parseReply(t, reply).answers, 1)
	assert.GreaterOrEqual(t, panics.Load(), int32(1))
}

// flakyConn fails the first reads and writes with a transient error, then delegates.
type flakyConn struct {
	net.PacketConn
	readFailures  atomic.Int32
	writeFailures atomic.Int32
}

func (f *flakyConn) ReadFrom(p []byte) (int, net.Addr, error) {
	if f.readFailures.Add(-1) >= 0 {
		return 0, nil, errors.New("transient read error")
	}
	return f.PacketConn.ReadFrom(p)
}

func (f *flakyConn) WriteTo(p []byte, addr net.Addr) (int, error) {
	if f.writeFailures.Add(-1) >= 0 {
		return 0, errors.New("transient write error")
	}
	return f.PacketConn.WriteTo(p, addr)
}

func listenFlaky(t *testing.T, readFailures int32) *flakyConn {
	t.Helper()
	conn, err := net.ListenPacket("udp", "127.0.0.1:0")
	require.NoError(t, err)
	f := &flakyConn{PacketConn: conn}
	f.readFailures.Store(readFailures)
	return f
}

func TestServer_ReadErrorsBackOffAndLoopContinues(t *testing.T) {
	t.Parallel()
	var serveErrors atomic.Int32
	conn := listenFlaky(t, 5)
	s := start(Config{
		Lookup:        fakeLookup(map[string][]netip.Addr{"app.test.": {netip.MustParseAddr("10.0.0.1")}}),
		LookupTimeout: time.Second,
		MaxLookups:    64,
		MaxWaiting:    256,
		AnswerTTL:     5,
		OnServeError:  func(error) { serveErrors.Add(1) },
	}, []boundListener{{name: "general", conn: conn}})

	reply := udpExchange(t, s, "general", buildQuery(t, 71, "app.test.", dnsmessage.TypeA, dnsmessage.ClassINET), 5*time.Second)
	require.NotNil(t, reply, "the loop answers after the read errors")
	assert.Len(t, parseReply(t, reply).answers, 1)
	assert.Equal(t, int32(5), serveErrors.Load())

	closed := make(chan error, 1)
	go func() { closed <- s.Close() }()
	select {
	case err := <-closed:
		assert.NoError(t, err)
	case <-time.After(5 * time.Second):
		t.Fatal("Close did not end the serve loop")
	}
}

func TestServer_WriteErrorsAreCountedNotLogged(t *testing.T) {
	t.Parallel()
	conn := listenFlaky(t, 0)
	conn.writeFailures.Store(1)
	s := start(Config{
		Lookup:        fakeLookup(map[string][]netip.Addr{"app.test.": {netip.MustParseAddr("10.0.0.1")}}),
		LookupTimeout: time.Second,
		MaxLookups:    64,
		MaxWaiting:    256,
		AnswerTTL:     5,
	}, []boundListener{{name: "general", conn: conn}})
	t.Cleanup(func() { _ = s.Close() })

	addr, _ := s.Address("general")
	client, err := net.Dial("udp", strings.TrimPrefix(addr, "udp/"))
	require.NoError(t, err)
	defer func() { _ = client.Close() }()
	_, err = client.Write(buildQuery(t, 81, "app.test.", dnsmessage.TypeA, dnsmessage.ClassINET))
	require.NoError(t, err)
	waitFor(t, func() bool { return s.writeErrors.Load() == 1 })
}

func TestServer_CloseEndsGoroutines(t *testing.T) {
	s, err := Start(Config{Lookup: fakeLookup(map[string][]netip.Addr{"app.test.": {netip.MustParseAddr("10.0.0.1")}}), LookupTimeout: time.Second,
		Listeners: []Listener{{Name: "general"}}})
	require.NoError(t, err)
	require.Eventually(t, func() bool { return serverGoroutines(s) > 0 }, 2*time.Second, time.Millisecond, "the count sees the serve loop while the server runs")
	reply := udpExchange(t, s, "general", buildQuery(t, 91, "app.test.", dnsmessage.TypeA, dnsmessage.ClassINET), 2*time.Second)
	require.NotNil(t, reply)
	require.NoError(t, s.Close())
	require.Eventually(t, func() bool { return serverGoroutines(s) == 0 }, 3*time.Second, 10*time.Millisecond)
}

func TestServer_HandleRecoversPanicsOutsideTheLookup(t *testing.T) {
	t.Parallel()
	var panics atomic.Int32
	// Every address is refused, so OnRefused runs and panics on the query goroutine.
	s := startServer(t, Config{
		Lookup:    fakeLookup(map[string][]netip.Addr{"app.test.": {netip.MustParseAddr("10.0.0.1")}}),
		OnRefused: func(string, int) { panic("callback panic") },
		OnPanic:   func() { panics.Add(1) },
		Listeners: []Listener{{Name: "general", Filter: func(netip.Addr) bool { return true }}},
	})
	assert.Nil(t, udpExchange(t, s, "general", buildQuery(t, 101, "app.test.", dnsmessage.TypeA, dnsmessage.ClassINET), 500*time.Millisecond))
	assert.Equal(t, int32(1), panics.Load())
}

// serverGoroutines counts goroutines still running the serve and query loops of s.
// The receiver pointer appears in the frame arguments of each such goroutine, so
// goroutines of other servers in the test binary are not counted.
func serverGoroutines(s *Server) int {
	buf := make([]byte, 1<<20)
	stacks := string(buf[:runtime.Stack(buf, true)])
	ptr := fmt.Sprintf("%p", s)
	return strings.Count(stacks, "(*Server).serve("+ptr) + strings.Count(stacks, "(*Server).handle("+ptr)
}
