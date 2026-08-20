package remotestorage

import (
	"context"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidateIPSSRF_TableDriven is the core SSRF-rejection regression test
// (required test #3 in the Issue #32 gap-closing plan): loopback,
// link-local/cloud-metadata, and other reserved ranges are always rejected,
// RFC1918 private ranges are explicitly allowed (self-hosted NAS use case,
// spec §3.7), and ordinary public addresses are allowed.
func TestValidateIPSSRF_TableDriven(t *testing.T) {
	tests := []struct {
		name    string
		ip      string
		wantErr bool
	}{
		{name: "loopback IPv4 rejected", ip: "127.0.0.1", wantErr: true},
		{name: "loopback IPv6 rejected", ip: "::1", wantErr: true},
		{name: "link-local rejected", ip: "169.254.1.1", wantErr: true},
		{name: "cloud metadata rejected", ip: "169.254.169.254", wantErr: true},
		{name: "reserved 0.0.0.0/8 rejected", ip: "0.1.2.3", wantErr: true},
		{name: "reserved 240.0.0.0/4 rejected", ip: "240.1.2.3", wantErr: true},
		{name: "RFC1918 10.x allowed", ip: "10.1.2.3", wantErr: false},
		{name: "RFC1918 172.16.x allowed", ip: "172.16.1.1", wantErr: false},
		{name: "RFC1918 192.168.x allowed", ip: "192.168.1.1", wantErr: false},
		{name: "public address allowed", ip: "203.0.113.10", wantErr: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateIPSSRF(net.ParseIP(tt.ip))
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		})
	}
}

// TestValidateHostSSRF_RejectsLoopbackAndMetadata proves the config-save-time
// entry point (spec §3.7) rejects loopback and metadata IP literals; using
// literal IPs keeps the test deterministic without a real DNS query.
func TestValidateHostSSRF_RejectsLoopbackAndMetadata(t *testing.T) {
	for _, host := range []string{"127.0.0.1", "169.254.169.254", "::1"} {
		t.Run(host, func(t *testing.T) {
			err := ValidateHostSSRF(host)
			require.Error(t, err)
		})
	}
}

// TestValidateHostSSRF_AllowsRFC1918AndPublic proves legitimate NAS-style
// (RFC1918) and ordinary public targets are accepted at config-save time.
func TestValidateHostSSRF_AllowsRFC1918AndPublic(t *testing.T) {
	for _, host := range []string{"192.168.1.50", "10.0.0.5", "203.0.113.20"} {
		t.Run(host, func(t *testing.T) {
			err := ValidateHostSSRF(host)
			require.NoError(t, err)
		})
	}
}

// TestSafeDialer_RejectsLoopbackAtDialTime proves the dial-time SSRF layer
// (net.Dialer.Control hook) rejects a loopback destination even though the
// TCP connect itself would otherwise succeed — this is what defeats
// DNS-rebinding TOCTOU between config-save-time validation and the real
// dial (spec §3.7).
func TestSafeDialer_RejectsLoopbackAtDialTime(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer func() { _ = listener.Close() }()

	go func() {
		for {
			conn, acceptErr := listener.Accept()
			if acceptErr != nil {
				return
			}
			_ = conn.Close()
		}
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	conn, dialErr := dialContext(ctx, "tcp", listener.Addr().String(), time.Second)
	require.Error(t, dialErr, "dial-time SSRF check must reject a loopback destination")
	assert.Nil(t, conn)
}

// TestSwapSSRFValidators_ConcurrentAccess_NoRace is a deterministic
// regression guard for the data race fixed by backing ssrfValidateHost /
// ssrfValidateDialAddress with atomic.Pointer rather than plain package-level
// vars: one goroutine repeatedly reads via ssrfValidateHost/
// ssrfValidateDialAddress while another concurrently swaps and restores via
// swapSSRFValidators, mirroring the swap/restore pattern
// withPermissiveSSRFForLocalTest and WithPermissiveSSRFForTesting use in
// production tests. The only pass/fail signal is whether `go test -race`
// reports a DATA RACE; no other assertion is meaningful here since either
// the production default or a swapped-in permissive result is a valid
// outcome on any given iteration.
func TestSwapSSRFValidators_ConcurrentAccess_NoRace(t *testing.T) {
	const iterations = 1000

	permissiveHost := func(string) error { return nil }
	permissiveDial := func(net.IP) error { return nil }

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			_ = ssrfValidateHost("127.0.0.1")
			_ = ssrfValidateDialAddress(net.ParseIP("127.0.0.1"))
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < iterations; i++ {
			restore := swapSSRFValidators(permissiveHost, permissiveDial)
			restore()
		}
	}()

	wg.Wait()
}

// TestWithPermissiveSSRFForTesting_SwapsAndRestores proves the exported
// cross-package test seam (used by backup_remote_service_regression_test.go
// in the sibling `services` package) actually swaps both validators to
// permissive no-ops and restores the production defaults afterward. The
// cross-package caller already exercises this behaviorally, but this
// in-package test gives it direct, same-package coverage as well.
func TestWithPermissiveSSRFForTesting_SwapsAndRestores(t *testing.T) {
	require.Error(t, ssrfValidateHost("127.0.0.1"), "production default must reject loopback before swap")
	require.Error(t, ssrfValidateDialAddress(net.ParseIP("127.0.0.1")), "production default must reject loopback before swap")

	restore := WithPermissiveSSRFForTesting()
	assert.NoError(t, ssrfValidateHost("127.0.0.1"), "swapped-in host validator must be permissive")
	assert.NoError(t, ssrfValidateDialAddress(net.ParseIP("127.0.0.1")), "swapped-in dial validator must be permissive")

	restore()
	assert.Error(t, ssrfValidateHost("127.0.0.1"), "restore must reinstate the production default")
	assert.Error(t, ssrfValidateDialAddress(net.ParseIP("127.0.0.1")), "restore must reinstate the production default")
}
