package services

import (
	"fmt"
	"net"
	"net/netip"
	"time"

	"github.com/Wikid82/charon/backend/internal/caddy"
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/network/upstreamdns"
	"github.com/Wikid82/charon/backend/internal/ratelimit"
)

const (
	// upstreamBindAttempts is how often the answerer tries to bind before giving up.
	upstreamBindAttempts = 3
	// upstreamLogBurst and upstreamLogInterval cap the warnings of the answerer.
	upstreamLogBurst    = 3
	upstreamLogInterval = time.Minute
)

// Tests replace these to exercise the start and lookup paths without sockets or DNS.
var (
	startUpstreamDNS     = upstreamdns.Start
	upstreamLookup       = net.DefaultResolver.LookupNetIP
	upstreamBindRetryGap = 100 * time.Millisecond
)

// UpstreamAnswerFilters returns the answer filters for the upstream name server.
// general refuses what forward targets may not use; reserved also refuses this machine.
func UpstreamAnswerFilters() (general, reserved func(netip.Addr) bool) {
	general = refusedUpstreamAnswer
	reserved = func(a netip.Addr) bool {
		return refusedUpstreamAnswer(a) || isThisMachineAddr(net.IP(a.AsSlice()))
	}
	return general, reserved
}

// refusedUpstreamAnswer applies the probe address policy to one answer address. The
// zone is dropped and an IPv4-mapped address is unmapped first. The unspecified
// address is refused: a block-list resolver answers 0.0.0.0, and connecting there
// reaches this machine's loopback listeners.
func refusedUpstreamAnswer(a netip.Addr) bool {
	a = a.WithZone("").Unmap()
	if a.IsUnspecified() {
		return true
	}
	return probePolicy().Blocked(net.IP(a.AsSlice()))
}

// UpstreamNameServer is the running upstream name answerer.
type UpstreamNameServer struct {
	srv *upstreamdns.Server
}

// StartUpstreamNameServer starts the answerer with both filters and the logging of the upstream name checks.
// A bind that fails is retried; the error is returned after the last attempt.
func StartUpstreamNameServer() (*UpstreamNameServer, error) {
	general, reserved := UpstreamAnswerFilters()
	readBudget := ratelimit.NewWarnBudget(upstreamLogBurst, upstreamLogInterval, nil)
	refusedBudget := ratelimit.NewWarnBudget(upstreamLogBurst, upstreamLogInterval, nil)
	cfg := upstreamdns.Config{
		Listeners: []upstreamdns.Listener{
			{Name: "general", Filter: general},
			{Name: "reserved", Filter: reserved},
		},
		Lookup: upstreamLookup,
		OnRefused: func(listener string, count int) {
			if ok, _ := refusedBudget.Take(); ok {
				logger.Log().WithFields(map[string]any{"count": count, "listener": listener}).
					Warn("Upstream name answer: addresses forward targets may not use were not returned")
			}
		},
		OnLookupError: func(listener string) {
			logger.Log().WithField("listener", listener).Debug("Upstream name lookup failed; the answer is SERVFAIL")
		},
		OnServeError: func(err error) {
			if ok, _ := readBudget.Take(); ok {
				logger.Log().WithError(err).Warn("Upstream name answer: socket read failed; retrying")
			}
		},
		OnPanic: func() {
			logger.Log().Error("Upstream name answer failed")
		},
	}
	var lastErr error
	for attempt := 1; attempt <= upstreamBindAttempts; attempt++ {
		srv, err := startUpstreamDNS(cfg)
		if err == nil {
			s := &UpstreamNameServer{srv: srv}
			addrGeneral, _ := srv.Address("general")
			addrReserved, _ := srv.Address("reserved")
			logger.Log().WithFields(map[string]any{"general": addrGeneral, "reserved": addrReserved}).
				Info("Upstream name answerer listening on loopback")
			return s, nil
		}
		lastErr = err
		if attempt < upstreamBindAttempts {
			time.Sleep(upstreamBindRetryGap)
		}
	}
	return nil, fmt.Errorf("start upstream name answerer after %d attempts: %w", upstreamBindAttempts, lastErr)
}

// Resolver returns the loopback addresses the proxy manager points every name lookup at.
func (s *UpstreamNameServer) Resolver() caddy.UpstreamResolver {
	if s == nil || s.srv == nil {
		return caddy.UpstreamResolver{}
	}
	general, _ := s.srv.Address("general")
	reserved, _ := s.srv.Address("reserved")
	return caddy.UpstreamResolver{General: general, Reserved: reserved}
}

// Close stops the answerer. It is safe on a nil server.
func (s *UpstreamNameServer) Close() error {
	if s == nil || s.srv == nil {
		return nil
	}
	return s.srv.Close()
}
