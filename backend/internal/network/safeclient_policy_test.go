package network

import (
	"context"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// withResolver replaces the package resolver seam for the duration of a test.
// Tests using it must not call t.Parallel (the seam is a package variable).
func withResolver(t *testing.T, answers map[string][]string) {
	t.Helper()
	prev := lookupIPAddr
	lookupIPAddr = func(_ context.Context, host string) ([]net.IPAddr, error) {
		raw, ok := answers[host]
		if !ok {
			return nil, &net.DNSError{Err: "no such host", Name: host, IsNotFound: true}
		}
		out := make([]net.IPAddr, 0, len(raw))
		for _, s := range raw {
			out = append(out, net.IPAddr{IP: net.ParseIP(s)})
		}
		return out, nil
	}
	t.Cleanup(func() { lookupIPAddr = prev })
}

func TestIsCGNAT(t *testing.T) {
	t.Parallel()
	tests := []struct {
		ip   string
		want bool
	}{
		{"100.64.0.1", true},
		{"100.127.255.254", true},
		{"100.100.100.200", true},
		{"::ffff:100.64.0.1", true},
		{"100.63.255.255", false},
		{"100.128.0.0", false},
		{"8.8.8.8", false},
		{"2001:4860:4860::8888", false},
	}
	for _, tt := range tests {
		if got := IsCGNAT(net.ParseIP(tt.ip)); got != tt.want {
			t.Errorf("IsCGNAT(%s) = %v, want %v", tt.ip, got, tt.want)
		}
	}
	if IsCGNAT(nil) {
		t.Error("IsCGNAT(nil) must be false")
	}
}

func TestIsTransitionRange(t *testing.T) {
	t.Parallel()
	tests := []struct {
		ip   string
		want bool
	}{
		{"192.0.0.1", true},
		{"198.18.0.1", true},
		{"198.19.255.255", true},
		{"64:ff9b::1", true},
		{"64:ff9b::808:808", true},
		{"64:ff9b:1::1", true},
		{"2002::1", true},
		{"2002:c000:204::1", true},
		{"::1.2.3.4", true},
		{"2001:0:4136:e378:8000:63bf:3fff:fdd2", true},
		// Excluded: handled by IsPrivateIP, or ordinary addresses.
		{"::", false},
		{"::1", false},
		{"::ffff:1.2.3.4", false},
		{"192.0.1.1", false},
		{"198.20.0.1", false},
		{"8.8.8.8", false},
		{"2001:db8::1", false},
		{"2606:4700:4700::1111", false},
	}
	for _, tt := range tests {
		if got := IsTransitionRange(net.ParseIP(tt.ip)); got != tt.want {
			t.Errorf("IsTransitionRange(%s) = %v, want %v", tt.ip, got, tt.want)
		}
	}
	if IsTransitionRange(nil) {
		t.Error("IsTransitionRange(nil) must be false")
	}
}

func TestClientOptionsPolicy(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		ip   string
		opts ClientOptions
		want bool
	}{
		{"public default", "8.8.8.8", ClientOptions{}, false},
		{"nil ip", "", ClientOptions{}, true},
		{"loopback default", "127.0.0.1", ClientOptions{}, true},
		{"loopback allowed", "127.0.0.1", ClientOptions{AllowLocalhost: true}, false},
		{"mapped loopback allowed", "::ffff:127.0.0.1", ClientOptions{AllowLocalhost: true}, false},
		{"rfc1918 default", "10.0.0.5", ClientOptions{}, true},
		{"rfc1918 allowed", "10.0.0.5", ClientOptions{AllowRFC1918: true}, false},
		{"link-local never allowed by rfc1918", "169.254.169.254", ClientOptions{AllowRFC1918: true, AllowLocalhost: true}, true},
		{"cgnat default allowed through", "100.64.0.1", ClientOptions{}, false},
		{"cgnat blocked by option", "100.64.0.1", ClientOptions{BlockCGNAT: true}, true},
		{"cgnat blocked even with rfc1918 allowed", "100.64.0.1", ClientOptions{BlockCGNAT: true, AllowRFC1918: true}, true},
		{"transition default allowed through", "2002::1", ClientOptions{}, false},
		{"transition blocked by option", "2002::1", ClientOptions{BlockTransitionRanges: true}, true},
		{"transition blocked with every allowance", "64:ff9b::1", ClientOptions{BlockTransitionRanges: true, AllowLocalhost: true, AllowRFC1918: true}, true},
		{"teredo blocked by option", "2001:0:4136:e378:8000:63bf:3fff:fdd2", ClientOptions{BlockTransitionRanges: true}, true},
		{"loopback v6 with transition option and allowance", "::1", ClientOptions{BlockTransitionRanges: true, AllowLocalhost: true}, false},
		{"unspecified blocked", "0.0.0.0", ClientOptions{AllowLocalhost: true, AllowRFC1918: true}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var ip net.IP
			if tt.ip != "" {
				ip = net.ParseIP(tt.ip)
			}
			opts := tt.opts
			if got := opts.policy().Blocked(ip); got != tt.want {
				t.Errorf("Blocked(%q) = %v, want %v", tt.ip, got, tt.want)
			}
		})
	}
}

func TestSafeDialer_BlockedAddressSentinelAndNoResolvedIP(t *testing.T) {
	withResolver(t, map[string][]string{
		"internal.example": {"10.20.30.40"},
		"meta.example":     {"169.254.169.254"},
	})
	opts := defaultOptions()
	dial := safeDialer(&opts)

	for _, host := range []string{"internal.example", "meta.example"} {
		_, err := dial(context.Background(), "tcp", net.JoinHostPort(host, "80"))
		if err == nil {
			t.Fatalf("%s: expected error", host)
		}
		if !errors.Is(err, ErrBlockedAddress) {
			t.Errorf("%s: error does not wrap ErrBlockedAddress: %v", host, err)
		}
		if !strings.Contains(err.Error(), "connection to private IP blocked") {
			t.Errorf("%s: unexpected message: %v", host, err)
		}
		for _, leaked := range []string{"10.20.30.40", "169.254.169.254", "resolved to"} {
			if strings.Contains(err.Error(), leaked) {
				t.Errorf("%s: message leaks %q: %v", host, leaked, err)
			}
		}
	}
}

func TestSafeDialer_NonPolicyErrorsDoNotWrapSentinel(t *testing.T) {
	withResolver(t, map[string][]string{"empty.example": {}})
	opts := defaultOptions()
	dial := safeDialer(&opts)

	for _, addr := range []string{"nxdomain.example:80", "empty.example:80", "no-port"} {
		_, err := dial(context.Background(), "tcp", addr)
		if err == nil {
			t.Fatalf("%s: expected error", addr)
		}
		if errors.Is(err, ErrBlockedAddress) {
			t.Errorf("%s: must not wrap ErrBlockedAddress: %v", addr, err)
		}
	}
}

func TestSafeDialer_RejectsBlockedAnswerInEitherPosition(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()
	_, port, _ := net.SplitHostPort(srv.Listener.Addr().String())

	withResolver(t, map[string][]string{
		"blocked-first.example": {"10.0.0.5", "127.0.0.1"},
		"blocked-last.example":  {"127.0.0.1", "10.0.0.5"},
		"cgnat-mixed.example":   {"127.0.0.1", "100.64.0.1"},
		"trans-mixed.example":   {"2002::1", "127.0.0.1"},
		"meta-alias.example":    {"::ffff:169.254.169.254", "127.0.0.1"},
		"clean.example":         {"127.0.0.1"},
	})

	tests := []struct {
		name string
		host string
		opts ClientOptions
		ok   bool
	}{
		{"rfc1918 blocked first", "blocked-first.example", ClientOptions{AllowLocalhost: true}, false},
		{"rfc1918 blocked last", "blocked-last.example", ClientOptions{AllowLocalhost: true}, false},
		{"rfc1918 allowed last", "blocked-last.example", ClientOptions{AllowLocalhost: true, AllowRFC1918: true}, true},
		{"cgnat blocked by option", "cgnat-mixed.example", ClientOptions{AllowLocalhost: true, BlockCGNAT: true}, false},
		{"cgnat not blocked without option", "cgnat-mixed.example", ClientOptions{AllowLocalhost: true}, true},
		{"transition blocked by option", "trans-mixed.example", ClientOptions{AllowLocalhost: true, BlockTransitionRanges: true}, false},
		{"mapped link-local blocked despite allowances", "meta-alias.example", ClientOptions{AllowLocalhost: true, AllowRFC1918: true}, false},
		{"clean answer", "clean.example", ClientOptions{AllowLocalhost: true}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := tt.opts
			opts.DialTimeout = 2 * time.Second
			conn, err := safeDialer(&opts)(context.Background(), "tcp", net.JoinHostPort(tt.host, port))
			if tt.ok {
				if err != nil {
					t.Fatalf("expected dial to succeed: %v", err)
				}
				_ = conn.Close()
				return
			}
			if conn != nil {
				_ = conn.Close()
			}
			if !errors.Is(err, ErrBlockedAddress) {
				t.Fatalf("expected ErrBlockedAddress, got %v", err)
			}
		})
	}
	if hits.Load() != 0 {
		t.Errorf("server received %d requests; no HTTP request should have been sent", hits.Load())
	}
}

func TestValidateRedirectTarget_SharedPolicy(t *testing.T) {
	withResolver(t, map[string][]string{
		"lan.example":     {"192.168.1.10"},
		"overlay.example": {"100.64.0.9"},
		"nat64.example":   {"64:ff9b::1"},
		"public.example":  {"8.8.8.8"},
		"meta.example":    {"169.254.169.254"},
	})

	tests := []struct {
		name string
		host string
		opts ClientOptions
		want bool // true = blocked
	}{
		{"rfc1918 default", "lan.example", ClientOptions{}, true},
		{"rfc1918 allowed", "lan.example", ClientOptions{AllowRFC1918: true}, false},
		{"cgnat blocked", "overlay.example", ClientOptions{AllowRFC1918: true, BlockCGNAT: true}, true},
		{"cgnat default", "overlay.example", ClientOptions{}, false},
		{"transition blocked", "nat64.example", ClientOptions{BlockTransitionRanges: true}, true},
		{"public", "public.example", ClientOptions{}, false},
		{"link-local with allowances", "meta.example", ClientOptions{AllowRFC1918: true, AllowLocalhost: true}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			opts := tt.opts
			opts.DialTimeout = time.Second
			req, _ := http.NewRequest(http.MethodGet, "https://"+tt.host+"/", http.NoBody)
			err := validateRedirectTarget(req, &opts)
			if tt.want {
				if !errors.Is(err, ErrBlockedAddress) {
					t.Fatalf("expected ErrBlockedAddress, got %v", err)
				}
				if strings.Contains(err.Error(), "192.168") || strings.Contains(err.Error(), "169.254") {
					t.Errorf("message leaks resolved address: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("expected redirect allowed, got %v", err)
			}
		})
	}
}

func TestValidateRedirectTarget_LocalhostWrapsSentinel(t *testing.T) {
	t.Parallel()
	opts := &ClientOptions{DialTimeout: time.Second}
	req, _ := http.NewRequest(http.MethodGet, "http://localhost/", http.NoBody)
	if err := validateRedirectTarget(req, opts); !errors.Is(err, ErrBlockedAddress) {
		t.Fatalf("expected ErrBlockedAddress, got %v", err)
	}
}

func TestValidateRedirectTarget_ResolutionFailure(t *testing.T) {
	withResolver(t, map[string][]string{})
	opts := &ClientOptions{DialTimeout: time.Second}
	req, _ := http.NewRequest(http.MethodGet, "https://gone.example/", http.NoBody)
	err := validateRedirectTarget(req, opts)
	if err == nil || errors.Is(err, ErrBlockedAddress) {
		t.Fatalf("expected non-policy resolution error, got %v", err)
	}
}

func TestNewSafeHTTPClient_BlockOptionsWired(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hits.Add(1)
	}))
	defer srv.Close()
	_, port, _ := net.SplitHostPort(srv.Listener.Addr().String())

	withResolver(t, map[string][]string{
		"overlay.example": {"100.64.0.9"},
		"nat64.example":   {"64:ff9b::1"},
	})

	for _, host := range []string{"overlay.example", "nat64.example"} {
		client := NewSafeHTTPClient(
			WithAllowRFC1918(), WithBlockCGNAT(), WithBlockTransitionRanges(),
			WithTimeout(2*time.Second),
		)
		resp, err := client.Get("http://" + net.JoinHostPort(host, port) + "/")
		if err == nil {
			_ = resp.Body.Close()
			t.Fatalf("%s: expected request to be blocked", host)
		}
		if !errors.Is(err, ErrBlockedAddress) {
			t.Errorf("%s: sentinel must survive *url.Error wrapping: %v", host, err)
		}
	}
	if hits.Load() != 0 {
		t.Errorf("server hit %d times, want 0", hits.Load())
	}
}

func TestIsPrivateIP_CGNATMetadataAliasAddress(t *testing.T) {
	t.Parallel()
	tests := []struct {
		ip   string
		want bool
	}{
		{"100.100.100.200", true},
		{"::ffff:100.100.100.200", true},
		{"100.100.100.199", false},
		{"100.100.100.201", false},
		{"100.64.0.1", false},
		{"100.127.255.254", false},
	}
	for _, tt := range tests {
		if got := IsPrivateIP(net.ParseIP(tt.ip)); got != tt.want {
			t.Errorf("IsPrivateIP(%s) = %v, want %v", tt.ip, got, tt.want)
		}
	}
	if IsRFC1918(net.ParseIP("100.100.100.200")) {
		t.Error("100.100.100.200 must not be classified as RFC 1918")
	}
}

func TestClientOptionsPolicy_MetadataAliasNeverReachableThroughAllowBranches(t *testing.T) {
	t.Parallel()
	combos := []ClientOptions{
		{},
		{AllowLocalhost: true},
		{AllowRFC1918: true},
		{AllowLocalhost: true, AllowRFC1918: true},
		// CGNAT-allowed policy: the range is not blocked by option.
		{AllowRFC1918: true, BlockTransitionRanges: true},
		{AllowLocalhost: true, AllowRFC1918: true, BlockTransitionRanges: true, BlockCGNAT: true},
	}
	for _, ipStr := range []string{"100.100.100.200", "::ffff:100.100.100.200"} {
		for i, opts := range combos {
			o := opts
			if !o.policy().Blocked(net.ParseIP(ipStr)) {
				t.Errorf("combo %d: %s must be blocked", i, ipStr)
			}
		}
	}
	// Neighbouring CGNAT addresses stay reachable when CGNAT is not blocked.
	o := ClientOptions{AllowRFC1918: true}
	if o.policy().Blocked(net.ParseIP("100.100.100.199")) {
		t.Error("100.100.100.199 must not be blocked when CGNAT blocking is off")
	}
}

func TestSafeDialer_CGNATMetadataAliasBlockedInBothLoops(t *testing.T) {
	withResolver(t, map[string][]string{
		"first.example": {"100.100.100.200", "8.8.8.8"},
		"last.example":  {"8.8.8.8", "100.100.100.200"},
		"mixed.example": {"::ffff:100.100.100.200"},
	})
	opts := ClientOptions{AllowLocalhost: true, AllowRFC1918: true, DialTimeout: time.Second}
	for _, host := range []string{"first.example", "last.example", "mixed.example"} {
		_, err := safeDialer(&opts)(context.Background(), "tcp", net.JoinHostPort(host, "80"))
		if !errors.Is(err, ErrBlockedAddress) {
			t.Errorf("%s: expected ErrBlockedAddress, got %v", host, err)
		}
	}
}
