package leash

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewDialHTTPClient_BypassesProxyAndDisablesHTTP2 verifies the handshake
// transport dials directly and never negotiates HTTP/2, which would break the
// WebSocket Upgrade.
func TestNewDialHTTPClient_BypassesProxyAndDisablesHTTP2(t *testing.T) {
	tr, ok := newDialHTTPClient().Transport.(*http.Transport)
	require.True(t, ok, "dial client must use an *http.Transport")
	assert.Nil(t, tr.Proxy, "handshake must not honor proxy environment variables")
	assert.NotNil(t, tr.TLSNextProto, "HTTP/2 must be disabled via an empty TLSNextProto map")
	assert.Empty(t, tr.TLSNextProto)
}
