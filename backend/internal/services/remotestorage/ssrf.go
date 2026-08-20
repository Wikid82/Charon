package remotestorage

import (
	"context"
	"fmt"
	"net"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/Wikid82/charon/backend/internal/network"
)

// ssrfValidateHost / ssrfValidateDialAddress are backed by atomic function
// pointers (rather than plain package-level vars) purely so white-box tests
// in this package can substitute a permissive check when exercising dial
// logic against a local test fixture (e.g. the SFTP host-key discovery
// test, which must dial 127.0.0.1) without weakening the default policy
// every production code path in this file uses, AND without racing
// concurrent readers: safeDialer's net.Dialer.Control hook (below) can run
// on a background net/http.Transport dial goroutine that outlives the
// synchronous test call that spawned it, so a plain unsynchronized var
// swap/restore in t.Cleanup is a genuine data race (caught by
// `go test -race`) against that goroutine's read. Tests restore the
// original via t.Cleanup; production code never reassigns these.
var (
	ssrfValidateHostFn        atomic.Pointer[func(string) error]
	ssrfValidateDialAddressFn atomic.Pointer[func(net.IP) error]
)

func init() {
	defaultHost := ValidateHostSSRF
	ssrfValidateHostFn.Store(&defaultHost)
	defaultDial := validateIPSSRF
	ssrfValidateDialAddressFn.Store(&defaultDial)
}

// ssrfValidateHost calls the currently-active host-validation func (the
// production default, or a test override installed via
// swapSSRFValidators/withPermissiveSSRFForLocalTest/WithPermissiveSSRFForTesting).
func ssrfValidateHost(host string) error {
	return (*ssrfValidateHostFn.Load())(host)
}

// ssrfValidateDialAddress calls the currently-active dial-address-validation
// func. See ssrfValidateHost.
func ssrfValidateDialAddress(ip net.IP) error {
	return (*ssrfValidateDialAddressFn.Load())(ip)
}

// ValidateHostSSRF resolves host and rejects it unless every resolved IP is
// permitted by spec §3.7's rules: RFC1918 private ranges are allowed (the
// primary use case is a self-hosted NAS on the operator's own LAN), but
// loopback, link-local/cloud-metadata (169.254.0.0/16), and other reserved
// ranges remain blocked. Used at remote-target config-save time; the
// dial-time re-check (defeating DNS-rebinding TOCTOU) lives in safeDialer
// below.
func ValidateHostSSRF(host string) error {
	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("resolve host %q: %w", host, err)
	}
	if len(ips) == 0 {
		return fmt.Errorf("no addresses found for host %q", host)
	}
	for _, ip := range ips {
		if err := validateIPSSRF(ip); err != nil {
			return err
		}
	}
	return nil
}

// validateIPSSRF applies the RFC1918-allowed SSRF policy to a single
// resolved IP.
func validateIPSSRF(ip net.IP) error {
	if network.IsRFC1918(ip) {
		return nil
	}
	if network.IsPrivateIP(ip) {
		return fmt.Errorf("connection to disallowed address blocked: %s", ip)
	}
	return nil
}

// safeDialer returns a *net.Dialer whose Control hook re-validates the
// literal address that is actually about to be connected to (spec §3.7
// dial-time check), immediately before connect() — this defeats DNS
// rebinding attacks where a hostname resolves differently between
// config-save-time validation and the real dial.
func safeDialer(timeout time.Duration) *net.Dialer {
	d := &net.Dialer{Timeout: timeout}
	d.Control = func(_ string, address string, _ syscall.RawConn) error {
		host, _, err := net.SplitHostPort(address)
		if err != nil {
			return fmt.Errorf("invalid dial address %q: %w", address, err)
		}
		ip := net.ParseIP(host)
		if ip == nil {
			return fmt.Errorf("dial address %q did not resolve to a literal IP", address)
		}
		return ssrfValidateDialAddress(ip)
	}
	return d
}

// dialContext dials network/address using the SSRF-safe dialer above.
func dialContext(ctx context.Context, dialNetwork, address string, timeout time.Duration) (net.Conn, error) {
	return safeDialer(timeout).DialContext(ctx, dialNetwork, address)
}

// WithPermissiveSSRFForTesting temporarily relaxes both the config-save-time
// and dial-time SSRF checks package-wide, returning a restore func that
// undoes it. It exists solely so tests in OTHER packages that must
// construct a *real* Uploader through the exported New() entry point
// against a local (loopback) test fixture can do so — e.g.
// backup_remote_service's retention-pruning regression test (spec §3.2,
// Issue #32 Phase 2), which proves pruneRemoteRetention still works against
// the actual s3Uploader/sftpUploader List() implementations, not a
// hand-rolled fake Uploader. It wraps the exact same
// ssrfValidateHost/ssrfValidateDialAddress seam the in-package tests already
// use via withPermissiveSSRFForLocalTest — tests within this package should
// keep using that helper instead; this exported wrapper is for cross-package
// test use only. Production code never calls this.
func WithPermissiveSSRFForTesting() (restore func()) {
	return swapSSRFValidators(
		func(string) error { return nil },
		func(net.IP) error { return nil },
	)
}

// swapSSRFValidators atomically replaces both the host- and
// dial-address-validation funcs and returns a restore func that atomically
// puts back whatever was active before the swap (not necessarily the
// production default — swaps may nest across helpers). Both
// withPermissiveSSRFForLocalTest (in-package tests) and
// WithPermissiveSSRFForTesting (cross-package tests) are thin wrappers
// around this so the swap-and-restore logic exists in exactly one place.
func swapSSRFValidators(host func(string) error, dial func(net.IP) error) (restore func()) {
	origHost := ssrfValidateHostFn.Load()
	origDial := ssrfValidateDialAddressFn.Load()
	ssrfValidateHostFn.Store(&host)
	ssrfValidateDialAddressFn.Store(&dial)
	return func() {
		ssrfValidateHostFn.Store(origHost)
		ssrfValidateDialAddressFn.Store(origDial)
	}
}
