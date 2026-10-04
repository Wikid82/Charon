package safehttp

import (
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/network"
)

func policies() map[string]Policy {
	return map[string]Policy{
		"zero":      {},
		"public":    PublicHTTPSOnly(),
		"privateOK": PrivateNetworkOK(),
	}
}

func TestPolicyConstructors(t *testing.T) {
	t.Parallel()
	if (Policy{}) != PublicHTTPSOnly() {
		t.Error("zero Policy must equal PublicHTTPSOnly (strictest)")
	}
	if p := PrivateNetworkOK(); !p.allowHTTP || !p.allowRFC1918 || !p.allowCGNAT {
		t.Errorf("PrivateNetworkOK = %+v", p)
	}
	if p := PublicHTTPSOnly(); p.allowHTTP || p.allowRFC1918 || p.allowCGNAT {
		t.Errorf("PublicHTTPSOnly = %+v", p)
	}
}

func TestValidateURLSyntax_AlwaysBlockedAddresses(t *testing.T) {
	t.Parallel()
	hosts := []string{
		"169.254.169.254", "[::ffff:169.254.169.254]", "169.254.169.254.", "[fd00:ec2::254]",
		"100.100.100.200", "[::ffff:100.100.100.200]", "100.100.100.200.",
		"127.0.0.1", "127.0.0.2", "[::1]", "[::ffff:127.0.0.1]", "localhost", "localhost.", "LOCALHOST", "foo.localhost",
		"[fd00::1]", "[fe80::1]", "[fe80::1%25eth0]",
		"0.0.0.0", "240.0.0.1", "255.255.255.255",
		"192.0.0.1", "198.18.0.1", "[64:ff9b::1]", "[64:ff9b:1::1]", "[2002::1]", "[::1.2.3.4]",
		"[2001:0:4136:e378:8000:63bf:3fff:fdd2]",
	}
	for name, p := range policies() {
		for _, h := range hosts {
			for _, scheme := range []string{"https", "http"} {
				raw := scheme + "://" + h + "/"
				_, err := ValidateURLSyntax(raw, p)
				if err == nil {
					t.Errorf("%s: %s accepted", name, raw)
					continue
				}
				// http under the strict policy fails on scheme first; every https form
				// (and every http form under a policy that allows it) must carry the sentinel.
				if (scheme == "https" || p.allowHTTP) && !errors.Is(err, ErrBlockedAddress) {
					t.Errorf("%s: %s: expected ErrBlockedAddress, got %v", name, raw, err)
				}
			}
		}
	}
}

func TestValidateURLSyntax_PolicyDependentAddresses(t *testing.T) {
	t.Parallel()
	tests := []struct {
		host      string
		publicOK  bool
		privateOK bool
	}{
		{"10.0.0.1", false, true},
		{"172.16.0.1", false, true},
		{"192.168.1.1", false, true},
		{"[::ffff:192.168.1.1]", false, true},
		{"100.64.0.1", false, true},
		{"100.127.255.254", false, true},
		{"8.8.8.8", true, true},
		{"[2606:4700:4700::1111]", true, true},
		{"example.com", true, true},
	}
	for _, tt := range tests {
		raw := "https://" + tt.host + ":8081"
		_, errPub := ValidateURLSyntax(raw, PublicHTTPSOnly())
		if (errPub == nil) != tt.publicOK {
			t.Errorf("PublicHTTPSOnly %s: err=%v, wantOK=%v", raw, errPub, tt.publicOK)
		}
		if errPub != nil && !errors.Is(errPub, ErrBlockedAddress) {
			t.Errorf("PublicHTTPSOnly %s: missing sentinel: %v", raw, errPub)
		}
		_, errPriv := ValidateURLSyntax(raw, PrivateNetworkOK())
		if (errPriv == nil) != tt.privateOK {
			t.Errorf("PrivateNetworkOK %s: err=%v, wantOK=%v", raw, errPriv, tt.privateOK)
		}
	}
}

func TestValidateURLSyntax_MalformedAndTricks(t *testing.T) {
	t.Parallel()
	reject := []string{
		"", "   ",
		"https://user:pw@example.com/", "https://good.com@169.254.169.254/", "https://user@example.com/",
		"https://example.com/#@good.com", "https://example.com/#frag",
		`https://good.com\@10.0.0.1/`,
		"https://example.com/?x=1", "https://example.com/?",
		"file:///etc/passwd", "gopher://example.com", "ftp://example.com", "javascript:alert(1)",
		"//example.com", "example.com", "mailto:a@b.c",
		"https:///path", "https://",
		"https://exa..mple.com",
		"https://" + strings.Repeat("a", 254) + ".com",
		"https://example.com:0", "https://example.com:70000", "https://example.com:abc",
		"https://exa mple.com",
		"https:example.com",
	}
	for _, raw := range reject {
		if _, err := ValidateURLSyntax(raw, PrivateNetworkOK()); err == nil {
			t.Errorf("%q accepted", raw)
		}
	}

	if _, err := ValidateURLSyntax("http://example.com", PublicHTTPSOnly()); err == nil {
		t.Error("http accepted under PublicHTTPSOnly")
	}
	if _, err := ValidateURLSyntax("http://example.com:8081", PrivateNetworkOK()); err != nil {
		t.Errorf("http rejected under PrivateNetworkOK: %v", err)
	}
	u, err := ValidateURLSyntax("HTTPS://Example.com:8443/prefix/", PublicHTTPSOnly())
	if err != nil {
		t.Fatalf("mixed-case scheme rejected: %v", err)
	}
	if u.Scheme != "https" || u.Hostname() != "Example.com" || u.Port() != "8443" || u.Path != "/prefix/" {
		t.Errorf("unexpected parse: %+v", u)
	}
}

func TestValidateURL(t *testing.T) {
	t.Parallel()

	for _, raw := range []string{
		"https://127.0.0.1/", "https://localhost/", "https://169.254.169.254/",
		"https://[::1]/", "http://127.0.0.1:8081/",
	} {
		_, err := ValidateURL(raw, PrivateNetworkOK())
		if !errors.Is(err, ErrBlockedAddress) {
			t.Errorf("%s: expected ErrBlockedAddress, got %v", raw, err)
		}
		if err != nil && strings.Contains(err.Error(), "127.0.0.1") && !strings.Contains(raw, "127.0.0.1") {
			t.Errorf("%s: message leaks resolved address: %v", raw, err)
		}
	}

	// A literal RFC 1918 address passes under the private policy (no DNS needed).
	u, err := ValidateURL("http://10.0.0.5:8081/pdns", PrivateNetworkOK())
	if err != nil {
		t.Fatalf("private literal rejected: %v", err)
	}
	if u.Host != "10.0.0.5:8081" || u.Path != "/pdns" {
		t.Errorf("unexpected url %+v", u)
	}
	if _, err := ValidateURL("https://10.0.0.5/", PublicHTTPSOnly()); !errors.Is(err, ErrBlockedAddress) {
		t.Errorf("private literal under strict policy: %v", err)
	}

	// Privileged non-standard ports are rejected by the shared validator.
	if _, err := ValidateURL("https://8.8.8.8:22/", PublicHTTPSOnly()); err == nil {
		t.Error("privileged port accepted")
	} else if errors.Is(err, ErrBlockedAddress) {
		t.Errorf("port failure must not carry the address sentinel: %v", err)
	}

	// Syntax failures surface before any lookup.
	if _, err := ValidateURL("https://user:pw@8.8.8.8/", PublicHTTPSOnly()); err == nil {
		t.Error("userinfo accepted")
	}
}

func TestNewClient_HardenedTransportForEveryPolicy(t *testing.T) {
	t.Parallel()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { hits.Add(1) }))
	defer srv.Close()

	for name, p := range policies() {
		client := NewClient(p, 2*time.Second)
		tr, ok := client.Transport.(*http.Transport)
		if !ok {
			t.Fatalf("%s: unexpected transport %T", name, client.Transport)
		}
		if tr.Proxy != nil {
			t.Errorf("%s: Proxy must be nil", name)
		}
		if client.Timeout != 2*time.Second {
			t.Errorf("%s: timeout = %v", name, client.Timeout)
		}
		if err := client.CheckRedirect(&http.Request{}, nil); !errors.Is(err, http.ErrUseLastResponse) {
			t.Errorf("%s: CheckRedirect = %v, want ErrUseLastResponse", name, err)
		}

		resp, err := client.Get(srv.URL)
		if err == nil {
			_ = resp.Body.Close()
			t.Errorf("%s: loopback request succeeded", name)
			continue
		}
		if !errors.Is(err, ErrBlockedAddress) {
			t.Errorf("%s: sentinel lost through *url.Error: %v", name, err)
		}
		var ue *url.Error
		if !errors.As(err, &ue) {
			t.Errorf("%s: expected *url.Error, got %T", name, err)
		}
	}
	if hits.Load() != 0 {
		t.Errorf("loopback server hit %d times, want 0", hits.Load())
	}
}

func TestNewClient_NamedLoopbackBlockedAtDial(t *testing.T) {
	t.Parallel()
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { hits.Add(1) }))
	defer srv.Close()
	_, port, _ := net.SplitHostPort(srv.Listener.Addr().String())

	// The pre-check is bypassed on purpose: the dialer alone must stop a name
	// that resolves to loopback.
	for _, host := range []string{"localhost", "localhost."} {
		resp, err := NewClient(PrivateNetworkOK(), 2*time.Second).Get("http://" + host + ":" + port + "/")
		if err == nil {
			_ = resp.Body.Close()
			t.Fatalf("%s: request succeeded", host)
		}
		if hits.Load() != 0 {
			t.Fatalf("%s: server was reached", host)
		}
	}
}

func TestClient_RedirectsAreNotFollowed(t *testing.T) {
	t.Parallel()
	for _, code := range []int{http.StatusFound, http.StatusMovedPermanently, http.StatusTemporaryRedirect, http.StatusPermanentRedirect} {
		var secondHits atomic.Int32
		second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			secondHits.Add(1)
			_, _ = w.Write([]byte("second"))
		}))

		targets := []string{second.URL, "http://169.254.169.254/latest/meta-data/", "https://example.com/"}
		for _, target := range targets {
			first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, target, code)
			}))
			resp, err := newClient(PrivateNetworkOK(), 2*time.Second, network.WithAllowLocalhost()).Get(first.URL)
			if err != nil {
				t.Fatalf("code %d target %s: %v", code, target, err)
			}
			if resp.StatusCode != code {
				t.Errorf("code %d target %s: got status %d, want the 3xx surfaced", code, target, resp.StatusCode)
			}
			_ = resp.Body.Close()
			first.Close()
		}
		if secondHits.Load() != 0 {
			t.Errorf("code %d: redirect target hit %d times, want 0", code, secondHits.Load())
		}
		second.Close()
	}
}

func TestClient_IgnoresProxyEnvironment(t *testing.T) {
	var proxyConns atomic.Int32
	proxy, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = proxy.Close() }()
	go func() {
		for {
			c, acceptErr := proxy.Accept()
			if acceptErr != nil {
				return
			}
			proxyConns.Add(1)
			_ = c.Close()
		}
	}()

	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { hits.Add(1) }))
	defer srv.Close()

	proxyURL := "http://" + proxy.Addr().String()
	for _, env := range []string{"HTTP_PROXY", "HTTPS_PROXY", "http_proxy", "https_proxy"} {
		t.Setenv(env, proxyURL)
	}
	t.Setenv("NO_PROXY", "")
	t.Setenv("no_proxy", "")

	resp, err := newClient(PrivateNetworkOK(), 2*time.Second, network.WithAllowLocalhost()).Get(srv.URL)
	if err != nil {
		t.Fatalf("request failed: %v", err)
	}
	_ = resp.Body.Close()
	if hits.Load() != 1 {
		t.Errorf("server hits = %d, want 1 (direct connection)", hits.Load())
	}
	if proxyConns.Load() != 0 {
		t.Errorf("proxy saw %d connections, want 0", proxyConns.Load())
	}
}

func TestClient_TimeoutAndBodyNotRequired(t *testing.T) {
	t.Parallel()
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(500 * time.Millisecond)
	}))
	defer slow.Close()

	start := time.Now()
	resp, err := newClient(PrivateNetworkOK(), 100*time.Millisecond, network.WithAllowLocalhost()).Get(slow.URL)
	if err == nil {
		_ = resp.Body.Close()
		t.Fatal("expected timeout")
	}
	if elapsed := time.Since(start); elapsed > 400*time.Millisecond {
		t.Errorf("timeout took %v", elapsed)
	}
}

func TestJoinPath(t *testing.T) {
	t.Parallel()
	mustParse := func(raw string) *url.URL {
		u, err := url.Parse(raw)
		if err != nil {
			t.Fatal(err)
		}
		return u
	}

	ok := []struct {
		name string
		base string
		segs []string
		want string
	}{
		{"plain", "https://pdns.example.com:8081", []string{"api", "v1", "servers", "localhost"}, "https://pdns.example.com:8081/api/v1/servers/localhost"},
		{"trailing slash", "https://pdns.example.com/", []string{"api"}, "https://pdns.example.com/api"},
		{"path prefix", "https://example.com/pdns/", []string{"api", "v1", "servers", "ns1.example_com-2"}, "https://example.com/pdns/api/v1/servers/ns1.example_com-2"},
		{"ipv6 host", "http://[fd00::1]:8081", []string{"api"}, "http://[fd00::1]:8081/api"},
		{"space is escaped", "https://example.com", []string{"a b"}, "https://example.com/a%20b"},
		{"percent is escaped", "https://example.com", []string{"a%b"}, "https://example.com/a%25b"},
		{"query chars escaped", "https://example.com", []string{"x?a=b", "y#f"}, "https://example.com/x%3Fa=b/y%23f"},
		{"no segments", "https://example.com/base", nil, "https://example.com/base"},
	}
	for _, tt := range ok {
		got, err := JoinPath(mustParse(tt.base), tt.segs...)
		if err != nil {
			t.Errorf("%s: %v", tt.name, err)
			continue
		}
		if got != tt.want {
			t.Errorf("%s: got %q want %q", tt.name, got, tt.want)
		}
		// Each segment must survive as exactly one path element.
		if len(tt.segs) > 0 {
			count := func(p string) int {
				p = strings.Trim(p, "/")
				if p == "" {
					return 0
				}
				return len(strings.Split(p, "/"))
			}
			if n := count(mustParse(got).EscapedPath()); n != count(mustParse(tt.base).EscapedPath())+len(tt.segs) {
				t.Errorf("%s: %q has %d path elements", tt.name, got, n)
			}
		}
	}

	base := mustParse("https://example.com")
	for _, seg := range []string{"", ".", "..", "%2e", "%2E", "%2e%2e", "%2E%2e", "a/b", `a\b`, "a%2Fb", "a%5Cb", "x\r\ny", "x\ny", "a\x00b"} {
		if got, err := JoinPath(base, "api", seg); err == nil {
			t.Errorf("segment %q accepted: %s", seg, got)
		}
	}

	if _, err := JoinPath(nil, "a"); err == nil {
		t.Error("nil base accepted")
	}
	if _, err := JoinPath(&url.URL{}, "a"); err == nil {
		t.Error("hostless base accepted")
	}
}

func TestErrBlockedAddressIdentity(t *testing.T) {
	t.Parallel()
	if !errors.Is(ErrBlockedAddress, network.ErrBlockedAddress) {
		t.Error("safehttp.ErrBlockedAddress must be the network sentinel")
	}
}

func TestValidateURLSyntax_RejectsNonCanonicalNumericHosts(t *testing.T) {
	t.Parallel()
	hosts := []string{
		"127.1", "2130706433", "0x7f000001", "0177.0.0.1", "0x7f.0.0.1", "0x7f.1",
		"017700000001", "169.254.43518", "10.1", "1.2.3.4.5", "0X7F000001", "127.1.", "0x",
	}
	for name, p := range policies() {
		for _, h := range hosts {
			raw := "https://" + h + ":8081/"
			_, err := ValidateURLSyntax(raw, p)
			if err == nil {
				t.Errorf("%s: %s accepted", name, raw)
				continue
			}
			if !errors.Is(err, ErrBlockedAddress) {
				t.Errorf("%s: %s error %v does not wrap ErrBlockedAddress", name, raw, err)
			}
		}
	}
}

func TestValidateURLSyntax_AllowsDigitContainingNames(t *testing.T) {
	t.Parallel()
	hosts := []string{
		"dns1.example.com", "10-0-0-1.example.com", "123.example.com", "1.2.3.4.example.com",
		"0x7f.example.com", "a1", "pdns-2", "8.8.8.8", "203.0.113.7",
	}
	for _, h := range hosts {
		if _, err := ValidateURLSyntax("https://"+h+"/", PublicHTTPSOnly()); err != nil {
			t.Errorf("%s rejected: %v", h, err)
		}
	}
}

func TestIsNonCanonicalNumericHost_EdgeCases(t *testing.T) {
	t.Parallel()
	for host, want := range map[string]bool{
		"": false, ".": false, ".1": false, "1.": false, "1.2.3.4": true, "0x": true, "0xg": false, "example": false,
	} {
		if got := isNonCanonicalNumericHost(host); got != want {
			t.Errorf("isNonCanonicalNumericHost(%q) = %v, want %v", host, got, want)
		}
	}
}
