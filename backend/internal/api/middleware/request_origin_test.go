package middleware

import (
	"bytes"
	"net"
	"net/http"
	"net/http/httptest"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/ratelimit"
	"github.com/Wikid82/charon/backend/internal/security"
	"github.com/Wikid82/charon/backend/internal/security/selfhop"
)

type peerFunc func(net.IP) bool

func (f peerFunc) Contains(ip net.IP) bool { return f(ip) }

// localPeers accepts loopback and 172.18.0.0/16 as this host's addresses.
var localPeers = peerFunc(func(ip net.IP) bool {
	_, n, _ := net.ParseCIDR("172.18.0.0/16")
	return ip.IsLoopback() || n.Contains(ip)
})

type probeResult struct {
	clientIP   string
	remoteAddr string
	origin     RequestOrigin
	hasOrigin  bool
	headers    http.Header
}

func newOriginSecret(t *testing.T) *selfhop.Secret {
	t.Helper()
	s, err := selfhop.NewSecret()
	require.NoError(t, err)
	return s
}

func originRouter(t *testing.T, secret *selfhop.Secret, trustedProxies []string, res *probeResult) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	require.NoError(t, r.SetTrustedProxies(trustedProxies))
	r.Use(selfHopHandler(secret, security.TrustedProxyMatcher{}, localPeers, ratelimit.NewWarnBudget(3, time.Minute, nil)))
	r.GET("/probe", func(c *gin.Context) {
		res.clientIP = c.ClientIP()
		res.remoteAddr = c.Request.RemoteAddr
		res.origin, res.hasOrigin = RequestOriginFrom(c)
		res.headers = c.Request.Header.Clone()
		c.Status(http.StatusNoContent)
	})
	return r
}

func doProbe(r *gin.Engine, remoteAddr string, headers map[string]string) {
	req := httptest.NewRequest(http.MethodGet, "/probe", http.NoBody)
	req.RemoteAddr = remoteAddr
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	r.ServeHTTP(httptest.NewRecorder(), req)
}

func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	buf := &bytes.Buffer{}
	logger.Init(false, buf)
	return buf
}

func TestSelfHop_WithoutHeadersChangesNothing(t *testing.T) {
	var res probeResult
	r := originRouter(t, newOriginSecret(t), nil, &res)

	doProbe(r, "203.0.113.9:4321", map[string]string{"X-Forwarded-For": "6.6.6.6", "X-Forwarded-Proto": "https"})

	assert.False(t, res.hasOrigin)
	assert.Equal(t, "203.0.113.9:4321", res.remoteAddr)
	assert.Equal(t, "203.0.113.9", res.clientIP)
	assert.Equal(t, "6.6.6.6", res.headers.Get("X-Forwarded-For"))
}

func TestSelfHop_VerifiedHopResolvesClient(t *testing.T) {
	secret := newOriginSecret(t)
	var res probeResult
	r := originRouter(t, secret, nil, &res)

	doProbe(r, "127.0.0.1:5555", map[string]string{
		selfhop.HeaderSecret: secret.Reveal(),
		selfhop.HeaderClient: "198.51.100.23",
		"X-Forwarded-Proto":  "HTTPS, http",
		"X-Forwarded-Host":   "charon.example.com, other",
		"X-Forwarded-For":    "6.6.6.6, 198.51.100.23",
		"X-Real-IP":          "6.6.6.6",
	})

	require.True(t, res.hasOrigin)
	assert.Equal(t, "198.51.100.23", res.clientIP)
	assert.Equal(t, "198.51.100.23:0", res.remoteAddr)
	assert.Equal(t, RequestOrigin{Addr: "198.51.100.23", Peer: "127.0.0.1", Scheme: "https", Host: "charon.example.com"}, res.origin)
	assert.Empty(t, res.headers.Get(selfhop.HeaderSecret))
	assert.Empty(t, res.headers.Get(selfhop.HeaderClient))
	assert.Empty(t, res.headers.Get("X-Forwarded-For"))
	assert.Empty(t, res.headers.Get("X-Real-IP"))
}

func TestSelfHop_VerifiedHopFromOwnInterfaceAddress(t *testing.T) {
	secret := newOriginSecret(t)
	var res probeResult
	r := originRouter(t, secret, nil, &res)

	doProbe(r, "172.18.0.5:5555", map[string]string{selfhop.HeaderSecret: secret.Reveal(), selfhop.HeaderClient: "2001:db8::7"})

	require.True(t, res.hasOrigin)
	assert.Equal(t, "2001:db8::7", res.clientIP)
	assert.Equal(t, "[2001:db8::7]:0", res.remoteAddr)
}

func TestSelfHop_UnusableClientResolvesToNonLocalAddress(t *testing.T) {
	secret := newOriginSecret(t)
	mgmt := ParseManagementNets(nil)
	for _, client := range []string{"", "{http.request.remote.host}", "not-an-ip", "198.51.100.23:80", "1.2.3.4, 5.6.7.8", "[::1]"} {
		var res probeResult
		r := originRouter(t, secret, nil, &res)

		doProbe(r, "172.18.0.5:5555", map[string]string{
			selfhop.HeaderSecret: secret.Reveal(),
			selfhop.HeaderClient: client,
			"X-Forwarded-Proto":  "https",
		})

		require.True(t, res.hasOrigin, client)
		assert.Equal(t, "0.0.0.0", res.clientIP, client)
		assert.Equal(t, "https", res.origin.Scheme, client)
		assert.Equal(t, "172.18.0.5", res.origin.Peer, client)
		ip := net.ParseIP(res.clientIP)
		assert.False(t, ip.IsLoopback() || ip.IsPrivate(), client)
		assert.False(t, IsManagementIP(mgmt, ip), client)
	}
}

func TestSelfHop_MissingClientHeaderResolvesToNonLocalAddress(t *testing.T) {
	secret := newOriginSecret(t)
	var res probeResult
	r := originRouter(t, secret, nil, &res)

	doProbe(r, "127.0.0.1:5555", map[string]string{selfhop.HeaderSecret: secret.Reveal(), "X-Forwarded-Proto": "http"})

	require.True(t, res.hasOrigin)
	assert.Equal(t, "0.0.0.0", res.clientIP)
	assert.Equal(t, "http", res.origin.Scheme)
	assert.False(t, IsManagementIP(ParseManagementNets(nil), net.ParseIP(res.clientIP)))
}

func TestSelfHop_ClientAddressNormalisation(t *testing.T) {
	secret := newOriginSecret(t)
	tests := map[string]string{
		"fe80::1%eth0":       "fe80::1",
		"2001:db8::7%en0":    "2001:db8::7",
		"::ffff:203.0.113.9": "203.0.113.9",
		"2001:DB8::A":        "2001:db8::a",
	}
	for in, want := range tests {
		var res probeResult
		r := originRouter(t, secret, nil, &res)

		doProbe(r, "127.0.0.1:5555", map[string]string{selfhop.HeaderSecret: secret.Reveal(), selfhop.HeaderClient: in})

		require.True(t, res.hasOrigin, in)
		assert.Equal(t, want, res.clientIP, in)
		assert.False(t, IsManagementIP(ParseManagementNets(nil), net.ParseIP(res.clientIP)), in)
	}
}

func TestSelfHop_IgnoresUnverifiedProof(t *testing.T) {
	secret := newOriginSecret(t)
	tests := []struct {
		name   string
		peer   string
		secret string
	}{
		{"wrong secret from loopback", "127.0.0.1:1", "0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"},
		{"missing secret", "127.0.0.1:1", ""},
		{"correct secret from outside peer", "203.0.113.9:1", secret.Reveal()},
		{"correct secret from other private peer", "10.0.0.8:1", secret.Reveal()},
		{"unparsable peer", "garbage", secret.Reveal()},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			logs := captureLogs(t)
			var res probeResult
			r := originRouter(t, secret, nil, &res)

			headers := map[string]string{selfhop.HeaderClient: "198.51.100.23"}
			if tc.secret != "" {
				headers[selfhop.HeaderSecret] = tc.secret
			}
			doProbe(r, tc.peer, headers)

			assert.False(t, res.hasOrigin)
			assert.Equal(t, tc.peer, res.remoteAddr)
			assert.NotEqual(t, "198.51.100.23", res.clientIP)
			assert.Empty(t, res.headers.Get(selfhop.HeaderSecret))
			assert.Empty(t, res.headers.Get(selfhop.HeaderClient))
			assert.Equal(t, 1, strings.Count(logs.String(), "Ignored unverified forwarding proof"))
			assert.NotContains(t, logs.String(), secret.Reveal())
			if tc.secret != "" {
				assert.NotContains(t, logs.String(), tc.secret)
			}
		})
	}
}

func TestSelfHop_StaleSecretFallsBackToPeerWithOneWarning(t *testing.T) {
	logs := captureLogs(t)
	staleSecret := newOriginSecret(t)
	currentSecret := newOriginSecret(t)
	var res probeResult
	r := originRouter(t, currentSecret, nil, &res)

	doProbe(r, "127.0.0.1:1", map[string]string{selfhop.HeaderSecret: staleSecret.Reveal(), selfhop.HeaderClient: "198.51.100.23"})

	assert.False(t, res.hasOrigin)
	assert.Equal(t, "127.0.0.1", res.clientIP)
	assert.Equal(t, 1, strings.Count(logs.String(), "Ignored unverified forwarding proof"))
	assert.NotContains(t, logs.String(), staleSecret.Reveal())
	assert.NotContains(t, logs.String(), currentSecret.Reveal())
}

func TestSelfHop_RepeatedSecretHeadersRejected(t *testing.T) {
	secret := newOriginSecret(t)
	var res probeResult
	r := originRouter(t, secret, nil, &res)

	req := httptest.NewRequest(http.MethodGet, "/probe", http.NoBody)
	req.RemoteAddr = "127.0.0.1:1"
	req.Header.Add(selfhop.HeaderSecret, secret.Reveal())
	req.Header.Add(selfhop.HeaderSecret, secret.Reveal())
	req.Header.Set(selfhop.HeaderClient, "198.51.100.23")
	r.ServeHTTP(httptest.NewRecorder(), req)

	assert.False(t, res.hasOrigin)
	assert.Equal(t, "127.0.0.1", res.clientIP)
}

func TestSelfHop_WarningsAreRateLimited(t *testing.T) {
	logs := captureLogs(t)
	var res probeResult
	r := originRouter(t, newOriginSecret(t), nil, &res)

	for range 20 {
		doProbe(r, "203.0.113.9:1", map[string]string{selfhop.HeaderSecret: "x", selfhop.HeaderClient: "198.51.100.23"})
	}
	assert.LessOrEqual(t, strings.Count(logs.String(), "Ignored unverified forwarding proof"), 3)
}

func TestSelfHop_NilSecretNeverVerifies(t *testing.T) {
	var res probeResult
	r := originRouter(t, nil, nil, &res)

	doProbe(r, "127.0.0.1:1", map[string]string{selfhop.HeaderSecret: "anything", selfhop.HeaderClient: "198.51.100.23"})
	assert.False(t, res.hasOrigin)
	assert.Equal(t, "127.0.0.1", res.clientIP)
}

// A client behind a private network can send any X-Forwarded-For value. With the
// engine trusting private peers, the verified address must still be the one
// resolved, never an entry from the client-supplied header.
func TestSelfHop_IgnoresClientSuppliedForwardingHeadersFromPrivatePeer(t *testing.T) {
	secret := newOriginSecret(t)
	trusted := []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "127.0.0.0/8"}
	var res probeResult
	r := originRouter(t, secret, trusted, &res)

	doProbe(r, "127.0.0.1:1", map[string]string{
		selfhop.HeaderSecret: secret.Reveal(),
		selfhop.HeaderClient: "192.168.1.50",
		"X-Forwarded-For":    "6.6.6.6, 192.168.1.50",
		"X-Real-IP":          "6.6.6.6",
	})

	require.True(t, res.hasOrigin)
	assert.Equal(t, "192.168.1.50", res.clientIP)
}

func TestSelfHop_BaseChainOrder(t *testing.T) {
	secret := newOriginSecret(t)
	chain := BaseChain(secret, security.TrustedProxyMatcher{}, false)
	require.Len(t, chain, 4)

	names := make([]string, len(chain))
	for i, h := range chain {
		names[i] = runtime.FuncForPC(reflect.ValueOf(h).Pointer()).Name()
	}
	idx := func(sub string) int {
		for i, n := range names {
			if strings.Contains(n, sub) {
				return i
			}
		}
		return -1
	}
	assert.Equal(t, 0, idx("elfHop"), names)
	assert.Less(t, idx("elfHop"), idx("RequestID"), names)
	assert.Less(t, idx("elfHop"), idx("RequestLogger"), names)
	assert.Less(t, idx("elfHop"), idx("Recovery"), names)
}

// Emergency bypass runs after the chain, so it must evaluate the resolved client
// address: only a verified hop can move the evaluated address.
func TestSelfHop_BaseChainFeedsLaterMiddleware(t *testing.T) {
	logger.Init(false, &bytes.Buffer{})
	t.Setenv(EmergencyTokenEnvVar, strings.Repeat("e", MinTokenLength))
	secret := newOriginSecret(t)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	require.NoError(t, r.SetTrustedProxies(nil))
	r.Use(BaseChain(secret, security.TrustedProxyMatcher{}, false)...)
	r.Use(EmergencyBypass([]string{"203.0.113.0/24"}, nil))
	r.GET("/probe", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"bypass": IsEmergencyBypass(c), "client": c.ClientIP()})
	})

	serve := func(peer string, headers map[string]string) string {
		req := httptest.NewRequest(http.MethodGet, "/probe", http.NoBody)
		req.RemoteAddr = peer
		req.Header.Set(EmergencyTokenHeader, strings.Repeat("e", MinTokenLength))
		for k, v := range headers {
			req.Header.Set(k, v)
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w.Body.String()
	}

	// Loopback peer without proof: address stays loopback, outside the network.
	assert.Contains(t, serve("127.0.0.1:1", nil), `"bypass":false`)
	// Same peer claiming an address without a valid secret: still loopback.
	body := serve("127.0.0.1:1", map[string]string{selfhop.HeaderSecret: "nope", selfhop.HeaderClient: "203.0.113.9"})
	assert.Contains(t, body, `"bypass":false`)
	assert.Contains(t, body, `"client":"127.0.0.1"`)
	// Verified proof: the reported client address is what later middleware sees.
	body = serve("127.0.0.1:1", map[string]string{selfhop.HeaderSecret: secret.Reveal(), selfhop.HeaderClient: "203.0.113.9"})
	assert.Contains(t, body, `"bypass":true`)
	assert.Contains(t, body, `"client":"203.0.113.9"`)
}

func TestSelfHop_RealPeerCheckerIsUsedByConstructor(t *testing.T) {
	secret := newOriginSecret(t)
	gin.SetMode(gin.TestMode)
	r := gin.New()
	require.NoError(t, r.SetTrustedProxies(nil))
	r.Use(SelfHop(secret, security.TrustedProxyMatcher{}))
	var seen RequestOrigin
	var ok bool
	r.GET("/probe", func(c *gin.Context) {
		seen, ok = RequestOriginFrom(c)
		c.Status(http.StatusNoContent)
	})

	req := httptest.NewRequest(http.MethodGet, "/probe", http.NoBody)
	req.RemoteAddr = "127.0.0.1:1"
	req.Header.Set(selfhop.HeaderSecret, secret.Reveal())
	req.Header.Set(selfhop.HeaderClient, "198.51.100.23")
	r.ServeHTTP(httptest.NewRecorder(), req)

	require.True(t, ok)
	assert.Equal(t, "198.51.100.23", seen.Addr)

	// A public peer is never accepted, whatever it sends.
	ok = false
	req = httptest.NewRequest(http.MethodGet, "/probe", http.NoBody)
	req.RemoteAddr = "203.0.113.9:1"
	req.Header.Set(selfhop.HeaderSecret, secret.Reveal())
	r.ServeHTTP(httptest.NewRecorder(), req)
	assert.False(t, ok)
}

func TestRequestOriginFrom_WrongTypeIsIgnored(t *testing.T) {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Set(requestOriginKey, "not-an-origin")
	_, ok := RequestOriginFrom(c)
	assert.False(t, ok)
}

func trustedOriginRouter(t *testing.T, secret *selfhop.Secret, configured []string, res *probeResult) *gin.Engine {
	t.Helper()
	gin.SetMode(gin.TestMode)
	r := gin.New()
	require.NoError(t, r.SetTrustedProxies(configured))
	r.Use(selfHopHandler(secret, security.NewTrustedProxyMatcher(configured), localPeers, ratelimit.NewWarnBudget(3, time.Minute, nil)))
	r.GET("/probe", func(c *gin.Context) {
		res.clientIP = c.ClientIP()
		res.remoteAddr = c.Request.RemoteAddr
		res.origin, res.hasOrigin = RequestOriginFrom(c)
		res.headers = c.Request.Header.Clone()
		c.Status(http.StatusNoContent)
	})
	return r
}

// A peer the operator listed as a trusted proxy keeps the configured
// forwarded-header handling; the origin record still carries scheme and host.
func TestSelfHop_ConfiguredTrustedPeerKeepsForwardedHeaderHandling(t *testing.T) {
	secret := newOriginSecret(t)
	var res probeResult
	r := trustedOriginRouter(t, secret, []string{"127.0.0.1/32", "::1/128", "192.168.0.0/16"}, &res)

	// The chain as the proxy forwards it: client, then the front proxy.
	doProbe(r, "127.0.0.1:5555", map[string]string{
		selfhop.HeaderSecret: secret.Reveal(),
		selfhop.HeaderClient: "192.168.1.9",
		"X-Forwarded-For":    "203.0.113.50, 192.168.1.9",
		"X-Forwarded-Proto":  "https",
		"X-Forwarded-Host":   "charon.example.com",
	})

	require.True(t, res.hasOrigin)
	assert.Equal(t, "127.0.0.1:5555", res.remoteAddr)
	assert.Equal(t, "203.0.113.50, 192.168.1.9", res.headers.Get("X-Forwarded-For"))
	assert.Equal(t, "203.0.113.50", res.clientIP)
	assert.Equal(t, "203.0.113.50", res.origin.Addr)
	assert.Equal(t, "127.0.0.1", res.origin.Peer)
	assert.Equal(t, "https", res.origin.Scheme)
	assert.Equal(t, "charon.example.com", res.origin.Host)
	assert.Empty(t, res.headers.Get(selfhop.HeaderSecret))
	assert.Empty(t, res.headers.Get(selfhop.HeaderClient))
}

// With a trusted-proxy list that does not include the peer, the reported
// client address is used and client-supplied forwarding headers are ignored.
func TestSelfHop_PeerOutsideConfiguredListUsesReportedClient(t *testing.T) {
	secret := newOriginSecret(t)
	var res probeResult
	r := trustedOriginRouter(t, secret, []string{"10.0.0.0/8"}, &res)

	doProbe(r, "127.0.0.1:5555", map[string]string{
		selfhop.HeaderSecret: secret.Reveal(),
		selfhop.HeaderClient: "10.1.1.1",
		"X-Forwarded-For":    "6.6.6.6, 10.1.1.1",
		"X-Real-IP":          "6.6.6.6",
	})

	require.True(t, res.hasOrigin)
	assert.Equal(t, "10.1.1.1", res.clientIP)
	assert.Equal(t, "10.1.1.1:0", res.remoteAddr)
	assert.Empty(t, res.headers.Get("X-Forwarded-For"))
	assert.Empty(t, res.headers.Get("X-Real-IP"))
}

// A verified hop whose client value is unusable must not let later
// management-network checks pass on the strength of the local peer address.
func TestSelfHop_UnusableClientDoesNotSatisfyManagementCheck(t *testing.T) {
	logger.Init(false, &bytes.Buffer{})
	t.Setenv(EmergencyTokenEnvVar, strings.Repeat("e", MinTokenLength))
	secret := newOriginSecret(t)

	gin.SetMode(gin.TestMode)
	r := gin.New()
	require.NoError(t, r.SetTrustedProxies(nil))
	r.Use(BaseChain(secret, security.TrustedProxyMatcher{}, false)...)
	r.Use(EmergencyBypass(nil, nil)) // default private and loopback networks
	r.GET("/probe", func(c *gin.Context) { c.JSON(http.StatusOK, gin.H{"bypass": IsEmergencyBypass(c)}) })

	for _, client := range []string{"fe80::1%eth0", "garbage", ""} {
		req := httptest.NewRequest(http.MethodGet, "/probe", http.NoBody)
		req.RemoteAddr = "127.0.0.1:1"
		req.Header.Set(EmergencyTokenHeader, strings.Repeat("e", MinTokenLength))
		req.Header.Set(selfhop.HeaderSecret, secret.Reveal())
		req.Header.Set(selfhop.HeaderClient, client)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Contains(t, w.Body.String(), `"bypass":false`, client)
	}
}
