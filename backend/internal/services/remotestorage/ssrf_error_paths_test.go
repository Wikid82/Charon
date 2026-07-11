package remotestorage

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestValidateHostSSRF_UnresolvableHost_WrapsLookupError proves the
// net.LookupIP failure branch (ssrf.go's "resolve host %q" wrap) surfaces a
// wrapped error rather than a bare/nil one, using a hostname under the IANA
// "invalid" reserved TLD (RFC 2606), which is guaranteed to never resolve.
func TestValidateHostSSRF_UnresolvableHost_WrapsLookupError(t *testing.T) {
	err := ValidateHostSSRF("this-host-does-not-exist.invalid")
	require.Error(t, err)
	assert.Contains(t, err.Error(), `resolve host "this-host-does-not-exist.invalid"`)
}

// TestSafeDialer_Control_InvalidDialAddress_MissingPort proves the dial-time
// Control hook's own SplitHostPort error branch: an address with no port at
// all is not something the real net.Dialer would ever pass in practice
// (it always supplies host:port), but calling Control directly — rather
// than through a real dial — lets this defensive branch be exercised
// deterministically and fast, without any real network I/O.
func TestSafeDialer_Control_InvalidDialAddress_MissingPort(t *testing.T) {
	d := safeDialer(time.Second)
	require.NotNil(t, d.Control)

	err := d.Control("tcp", "not-a-host-port-pair", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "invalid dial address")
}

// TestSafeDialer_Control_NonLiteralHost_RejectsUnresolvedHostname proves the
// "did not resolve to a literal IP" branch: by the time Control runs, Go's
// dialer has always already resolved the address to a literal IP, but this
// directly exercises the defensive check guarding against a hostname
// slipping through, keeping it a documented, tested invariant rather than
// silent dead code.
func TestSafeDialer_Control_NonLiteralHost_RejectsUnresolvedHostname(t *testing.T) {
	d := safeDialer(time.Second)

	err := d.Control("tcp", "example.com:443", nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "did not resolve to a literal IP")
}
