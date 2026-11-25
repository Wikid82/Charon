// Package upstreamdns answers A and AAAA questions for the proxy's upstream
// connections and leaves out addresses a filter refuses.
//
// The server listens on loopback UDP only. Each question is answered from a lookup
// of the asked name, with the filter applied to the result. Nothing is forwarded and
// nothing is cached beyond sharing one lookup between queries for the same name.
package upstreamdns

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/netip"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

const (
	// maxPacketSize is the read buffer per packet: the EDNS payload size.
	maxPacketSize = 1232
	// Read error backoff: starts at minReadBackoff, doubles, and is capped.
	minReadBackoff = 10 * time.Millisecond
	maxReadBackoff = time.Second

	defaultMaxLookups = 64
	defaultMaxWaiting = 256
	defaultAnswerTTL  = 5
)

// Filter reports whether an answer address must be left out.
type Filter func(addr netip.Addr) bool

// LookupFunc resolves a rooted name ("ip", "ip4" or "ip6").
type LookupFunc func(ctx context.Context, network, name string) ([]netip.Addr, error)

// Listener is one loopback socket with its own filter.
type Listener struct {
	Name   string // "general" or "reserved"; used in callbacks only
	Filter Filter
}

// Config configures a Server. Zero values take the defaults in brackets.
type Config struct {
	Listeners     []Listener
	Lookup        LookupFunc    // [net.DefaultResolver.LookupNetIP]
	LookupTimeout time.Duration // [LookupTimeoutFromResolvConf("/etc/resolv.conf")]
	MaxLookups    int           // [64] distinct names in flight
	MaxWaiting    int           // [256] queries waiting on a lookup
	AnswerTTL     uint32        // [5]

	// Callbacks run on the serving goroutines and must not block. Nil callbacks are ignored.
	OnRefused     func(listener string, count int)
	OnLookupError func(listener string)
	OnServeError  func(err error)
	OnPanic       func()
}

// boundListener is a listening socket with its name and filter.
type boundListener struct {
	name   string
	filter Filter
	conn   net.PacketConn
}

// Server is a running answerer.
type Server struct {
	cfg       Config
	listeners []boundListener
	coalesce  *coalescer

	base      context.Context
	cancel    context.CancelFunc
	closing   chan struct{}
	closeOnce sync.Once
	closeErr  error
	wg        sync.WaitGroup

	writeErrors atomic.Uint64
}

// Start binds every listener on 127.0.0.1 (or ::1) and serves until Close.
func Start(cfg Config) (*Server, error) {
	cfg = withDefaults(cfg)
	if len(cfg.Listeners) == 0 {
		return nil, errors.New("upstream name answerer needs at least one listener")
	}
	bound := make([]boundListener, 0, len(cfg.Listeners))
	for _, l := range cfg.Listeners {
		conn, err := listenLoopback()
		if err != nil {
			for _, b := range bound {
				_ = b.conn.Close()
			}
			return nil, fmt.Errorf("bind upstream name listener %s: %w", l.Name, err)
		}
		bound = append(bound, boundListener{name: l.Name, filter: l.Filter, conn: conn})
	}
	return start(cfg, bound), nil
}

// listenLoopback binds an ephemeral UDP port on 127.0.0.1, or on [::1] if IPv4 is not
// available.
func listenLoopback() (net.PacketConn, error) {
	conn, err4 := net.ListenPacket("udp", "127.0.0.1:0")
	if err4 == nil {
		return conn, nil
	}
	conn, err6 := net.ListenPacket("udp", "[::1]:0")
	if err6 != nil {
		return nil, errors.Join(err4, err6)
	}
	return conn, nil
}

// start runs the serve loop of each bound listener. Tests call it with fake sockets.
func start(cfg Config, bound []boundListener) *Server {
	base, cancel := context.WithCancel(context.Background())
	s := &Server{
		cfg:       cfg,
		listeners: bound,
		base:      base,
		cancel:    cancel,
		closing:   make(chan struct{}),
	}
	lookupTimeout := cfg.LookupTimeout
	lookup := func(ctx context.Context, name string) lookupResult {
		ctx, stop := context.WithTimeout(ctx, lookupTimeout)
		defer stop()
		addrs, err := cfg.Lookup(ctx, "ip", name)
		return lookupResult{addrs: addrs, err: err}
	}
	s.coalesce = newCoalescer(base, lookup, cfg.OnPanic, cfg.MaxLookups, cfg.MaxWaiting)
	for _, l := range bound {
		s.wg.Add(1)
		go s.serve(l)
	}
	return s
}

func withDefaults(cfg Config) Config {
	if cfg.Lookup == nil {
		cfg.Lookup = net.DefaultResolver.LookupNetIP
	}
	if cfg.LookupTimeout <= 0 {
		cfg.LookupTimeout = LookupTimeoutFromResolvConf("/etc/resolv.conf")
	}
	if cfg.MaxLookups <= 0 {
		cfg.MaxLookups = defaultMaxLookups
	}
	if cfg.MaxWaiting <= 0 {
		cfg.MaxWaiting = defaultMaxWaiting
	}
	if cfg.AnswerTTL == 0 {
		cfg.AnswerTTL = defaultAnswerTTL
	}
	return cfg
}

// serve reads packets from one listener until the socket is closed. Other read errors
// are reported and retried after a backoff, so the loop only ends with the socket.
func (s *Server) serve(l boundListener) {
	defer s.wg.Done()
	buf := make([]byte, maxPacketSize)
	backoff := time.Duration(0)
	for {
		n, src, err := l.conn.ReadFrom(buf)
		if err != nil {
			if errors.Is(err, net.ErrClosed) {
				return
			}
			if s.cfg.OnServeError != nil {
				s.cfg.OnServeError(err)
			}
			backoff = nextBackoff(backoff)
			select {
			case <-time.After(backoff):
			case <-s.closing:
				return
			}
			continue
		}
		backoff = 0
		packet := append([]byte(nil), buf[:n]...)
		s.wg.Add(1)
		go s.handle(l, packet, src)
	}
}

// nextBackoff doubles the read error backoff, from minReadBackoff up to maxReadBackoff.
func nextBackoff(current time.Duration) time.Duration {
	if current == 0 {
		return minReadBackoff
	}
	return min(current*2, maxReadBackoff)
}

// handle answers one query. A panic is recovered and the query gets no reply.
func (s *Server) handle(l boundListener, packet []byte, src net.Addr) {
	defer s.wg.Done()
	defer func() {
		if recover() != nil && s.cfg.OnPanic != nil {
			s.cfg.OnPanic()
		}
	}()

	req, early, ok := parseQuery(packet)
	if !ok {
		return
	}
	if early != nil {
		s.write(l, early, src)
		return
	}
	name := strings.ToLower(req.question.Name.String())
	res, accepted := s.coalesce.resolve(name)
	if !accepted {
		return
	}
	if res.err != nil && !isNotFound(res.err) && s.cfg.OnLookupError != nil {
		s.cfg.OnLookupError(l.name)
	}
	reply, refused, ok := answer(packet, res, l.filter, s.cfg.AnswerTTL)
	if !ok {
		return
	}
	if refused > 0 && s.cfg.OnRefused != nil {
		s.cfg.OnRefused(l.name, refused)
	}
	s.write(l, reply, src)
}

// write sends a reply to the source address. A write error is counted and not logged.
func (s *Server) write(l boundListener, reply []byte, src net.Addr) {
	if reply == nil {
		return
	}
	if _, err := l.conn.WriteTo(reply, src); err != nil {
		s.writeErrors.Add(1)
	}
}

// Address returns the Caddy resolver address of a listener, e.g. "udp/127.0.0.1:41234".
func (s *Server) Address(listener string) (string, bool) {
	if s == nil {
		return "", false
	}
	for _, l := range s.listeners {
		if l.name == listener {
			return "udp/" + l.conn.LocalAddr().String(), true
		}
	}
	return "", false
}

// Close stops serving and waits for the in-flight answers. It is safe on a nil Server
// and when called more than once.
func (s *Server) Close() error {
	if s == nil {
		return nil
	}
	s.closeOnce.Do(func() {
		close(s.closing)
		s.cancel()
		var errs []error
		for _, l := range s.listeners {
			if err := l.conn.Close(); err != nil && !errors.Is(err, net.ErrClosed) {
				errs = append(errs, err)
			}
		}
		s.wg.Wait()
		s.closeErr = errors.Join(errs...)
	})
	return s.closeErr
}
