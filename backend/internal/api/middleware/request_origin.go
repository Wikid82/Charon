package middleware

import (
	"net"
	"net/http"
	"net/netip"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/ratelimit"
	"github.com/Wikid82/charon/backend/internal/security"
	"github.com/Wikid82/charon/backend/internal/security/selfhop"
	"github.com/Wikid82/charon/backend/internal/util"
)

// requestOriginKey is the gin context key holding the RequestOrigin record.
const requestOriginKey = "request_origin"

// RequestOrigin describes a request that reached the API through Charon's own
// reverse proxy and presented a verified proof of that hop.
type RequestOrigin struct {
	// Addr is the client address reported by the proxy (the raw peer when the
	// proxy did not supply a usable one).
	Addr string
	// Peer is the address of the TCP peer that delivered the request.
	Peer string
	// Scheme is the scheme the client used ("http" or "https"), or empty.
	Scheme string
	// Host is the host the client addressed, or empty.
	Host string
}

// RequestOriginFrom returns the verified origin record, if the request carried one.
func RequestOriginFrom(c *gin.Context) (RequestOrigin, bool) {
	v, ok := c.Get(requestOriginKey)
	if !ok {
		return RequestOrigin{}, false
	}
	origin, ok := v.(RequestOrigin)
	return origin, ok
}

// peerChecker decides whether a TCP peer is allowed to present hop proof.
type peerChecker interface {
	Contains(ip net.IP) bool
}

// SelfHop recognises requests forwarded by Charon's own reverse proxy.
//
// The proxy sets selfhop.HeaderSecret and selfhop.HeaderClient on requests whose
// upstream is Charon. When the peer is loopback or an address of this host and
// the secret verifies, the client address replaces the connection address for
// the rest of the chain (so c.ClientIP() reports the real client) and a
// RequestOrigin record is stored. In every other case the request is left as is.
// Both headers are always removed before the next handler runs.
//
// It must be the first middleware on the engine.
func SelfHop(secret *selfhop.Secret, trusted security.TrustedProxyMatcher) gin.HandlerFunc {
	return selfHopHandler(secret, trusted, selfhop.NewLocalAddrs(), ratelimit.NewWarnBudget(3, time.Minute, nil))
}

func selfHopHandler(secret *selfhop.Secret, trusted security.TrustedProxyMatcher, local peerChecker, warn *ratelimit.WarnBudget) gin.HandlerFunc {
	return func(c *gin.Context) {
		hdr := c.Request.Header
		secretVals, hasSecret := hdr[http.CanonicalHeaderKey(selfhop.HeaderSecret)]
		clientVals, hasClient := hdr[http.CanonicalHeaderKey(selfhop.HeaderClient)]
		if !hasSecret && !hasClient {
			c.Next()
			return
		}
		hdr.Del(selfhop.HeaderSecret)
		hdr.Del(selfhop.HeaderClient)

		peerHost, _, err := net.SplitHostPort(c.Request.RemoteAddr)
		if err != nil {
			peerHost = c.Request.RemoteAddr
		}
		peerIP := net.ParseIP(peerHost)
		peerAddr, _ := netip.AddrFromSlice(peerIP)
		peerAddr = peerAddr.Unmap()

		if peerIP == nil || !local.Contains(peerIP) || len(secretVals) != 1 || !secret.Verify(secretVals[0]) {
			if ok, suppressed := warn.Take(); ok {
				logger.Log().WithFields(map[string]any{
					"peer":       util.SanitizeForLog(peerHost),
					"suppressed": suppressed,
				}).Warn("Ignored unverified forwarding proof on request")
			}
			c.Next()
			return
		}

		// An unusable client value must never fall back to the connection address:
		// that address is this host's own, which later checks treat as local. It
		// resolves to the unspecified address, which no local or management-network
		// check accepts. Scheme and host still come from the verified hop.
		addr := netip.IPv4Unspecified()
		clientOK := false
		if len(clientVals) == 1 {
			addr, clientOK = parseClientAddr(clientVals[0])
			if !clientOK {
				addr = netip.IPv4Unspecified()
			}
		}
		if !clientOK {
			if ok, suppressed := warn.Take(); ok {
				logger.Log().WithField("suppressed", suppressed).Warn("Verified forwarding proof carried no usable client address")
			}
		}
		origin := RequestOrigin{
			Addr:   addr.String(),
			Peer:   peerIP.String(),
			Scheme: forwardedScheme(hdr.Get("X-Forwarded-Proto")),
			Host:   firstListEntry(hdr.Get("X-Forwarded-Host")),
		}

		// A peer the operator configured as a trusted proxy keeps the configured
		// forwarded-header handling. Otherwise the reported client address replaces
		// the connection address and the client-address headers are dropped.
		if trusted.Contains(peerAddr) {
			origin.Addr = c.ClientIP()
		} else {
			c.Request.RemoteAddr = net.JoinHostPort(origin.Addr, "0")
			hdr.Del("X-Forwarded-For")
			hdr.Del("X-Real-IP")
		}
		c.Set(requestOriginKey, origin)
		c.Next()
	}
}

func firstListEntry(v string) string {
	first, _, _ := strings.Cut(v, ",")
	return strings.TrimSpace(first)
}

func forwardedScheme(v string) string {
	switch s := strings.ToLower(firstListEntry(v)); s {
	case "http", "https":
		return s
	default:
		return ""
	}
}

// BaseChain returns the engine-wide middleware installed before anything else:
// request-origin resolution first, then request id, request logging and panic
// recovery. Both the server entrypoint and tests build the chain through here
// so the order stays in one place.
func BaseChain(secret *selfhop.Secret, trusted security.TrustedProxyMatcher, debug bool) []gin.HandlerFunc {
	return []gin.HandlerFunc{
		SelfHop(secret, trusted),
		RequestID(),
		RequestLogger(),
		Recovery(debug),
	}
}

// parseClientAddr parses a client address value. An IPv6 zone is dropped and
// IPv4-mapped addresses are returned in IPv4 form.
func parseClientAddr(v string) (netip.Addr, bool) {
	addr, err := netip.ParseAddr(strings.TrimSpace(v))
	if err != nil {
		return netip.Addr{}, false
	}
	return addr.WithZone("").Unmap(), true
}
