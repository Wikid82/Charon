package main

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/pkg/dnsprovider"
	"github.com/Wikid82/charon/backend/pkg/safehttp"
)

func TestProviderIdentity(t *testing.T) {
	p := &PowerDNSProvider{}

	if got := p.Type(); got != "powerdns" {
		t.Fatalf("Type() = %q, want powerdns", got)
	}

	md := p.Metadata()
	if md.Type != "powerdns" || md.Name != "PowerDNS" || md.IsBuiltIn {
		t.Fatalf("unexpected metadata: %+v", md)
	}
	if md.InterfaceVersion != dnsprovider.InterfaceVersion {
		t.Fatalf("InterfaceVersion = %q, want %q", md.InterfaceVersion, dnsprovider.InterfaceVersion)
	}

	if Plugin == nil || Plugin.Type() != "powerdns" {
		t.Fatalf("exported Plugin symbol is not the PowerDNS provider")
	}
}

func TestCredentialFieldSpecs(t *testing.T) {
	p := &PowerDNSProvider{}

	required := map[string]bool{}
	for _, f := range p.RequiredCredentialFields() {
		required[f.Name] = true
	}
	if !required["api_url"] || !required["api_key"] || len(required) != 2 {
		t.Fatalf("unexpected required fields: %v", required)
	}

	optional := p.OptionalCredentialFields()
	if len(optional) != 1 || optional[0].Name != "server_id" {
		t.Fatalf("unexpected optional fields: %+v", optional)
	}
}

func TestValidateCredentialsRequiredFields(t *testing.T) {
	p := &PowerDNSProvider{}

	tests := []struct {
		name    string
		creds   map[string]string
		wantErr bool
	}{
		{"missing api_url", map[string]string{"api_key": "k"}, true},
		{"missing api_key", map[string]string{"api_url": "https://pdns.example.com:8081"}, true},
		{"both present", map[string]string{"api_url": "https://pdns.example.com:8081", "api_key": "k"}, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := p.ValidateCredentials(tt.creds); (err != nil) != tt.wantErr {
				t.Fatalf("ValidateCredentials() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestBuildCaddyConfigDefaultsServerID(t *testing.T) {
	p := &PowerDNSProvider{}
	creds := map[string]string{"api_url": "https://pdns.example.com:8081", "api_key": "k"}

	cfg := p.BuildCaddyConfig(creds)
	if cfg["name"] != "powerdns" || cfg["server_id"] != "localhost" || cfg["api_url"] != creds["api_url"] {
		t.Fatalf("unexpected config: %v", cfg)
	}

	creds["server_id"] = "pdns1"
	if got := p.BuildCaddyConfigForZone("example.com", creds)["server_id"]; got != "pdns1" {
		t.Fatalf("server_id = %v, want pdns1", got)
	}
}

func TestPropagationSettings(t *testing.T) {
	p := &PowerDNSProvider{}
	if p.PropagationTimeout() != 60*time.Second || p.PollingInterval() != 2*time.Second {
		t.Fatalf("unexpected propagation settings")
	}
	if p.SupportsMultiCredential() {
		t.Fatalf("SupportsMultiCredential() = true, want false")
	}
}

func TestValidateCredentials_EndpointAndServerID(t *testing.T) {
	p := &PowerDNSProvider{}
	base := func(mut func(map[string]string)) map[string]string {
		c := map[string]string{"api_url": "https://pdns.example.com:8081", "api_key": "k"}
		mut(c)
		return c
	}

	accepted := []map[string]string{
		base(func(c map[string]string) {}),
		base(func(c map[string]string) { c["api_url"] = "http://pdns.example.com:8081" }),
		base(func(c map[string]string) { c["api_url"] = "http://10.0.0.5:8081" }),
		base(func(c map[string]string) { c["api_url"] = "http://192.168.1.10:8081/pdns/" }),
		base(func(c map[string]string) { c["api_url"] = "http://100.64.0.7:8081" }),
		base(func(c map[string]string) { c["server_id"] = "ns1.example_com-2" }),
		base(func(c map[string]string) { c["server_id"] = strings.Repeat("a", 64) }),
	}
	for _, creds := range accepted {
		if err := p.ValidateCredentials(creds); err != nil {
			t.Errorf("%v rejected: %v", creds, err)
		}
	}

	blockedURLs := []string{
		"http://127.0.0.1:8081", "http://localhost:8081", "http://[::1]:8081",
		"http://169.254.169.254", "http://100.100.100.200", "http://[fd00::1]:8081",
		"http://0.0.0.0:8081", "http://[64:ff9b::1]", "http://192.0.0.1",
		"http://127.1:8081", "http://2130706433:8081", "http://0x7f000001:8081", "http://0177.0.0.1:8081",
	}
	for _, raw := range blockedURLs {
		err := p.ValidateCredentials(base(func(c map[string]string) { c["api_url"] = raw }))
		if err == nil {
			t.Errorf("%s accepted", raw)
			continue
		}
		if !errors.Is(err, safehttp.ErrBlockedAddress) {
			t.Errorf("%s: expected blocked-address error, got %v", raw, err)
		}
		if !strings.Contains(err.Error(), "use a non-loopback address reachable from Charon") {
			t.Errorf("%s: missing remedy text: %v", raw, err)
		}
	}

	badURLs := []string{
		"ftp://pdns.example.com", "file:///etc/passwd", "https://user:pw@pdns.example.com",
		"https://pdns.example.com/?x=1", "https://pdns.example.com/#f", "pdns.example.com", "https://",
	}
	for _, raw := range badURLs {
		err := p.ValidateCredentials(base(func(c map[string]string) { c["api_url"] = raw }))
		if err == nil {
			t.Errorf("%s accepted", raw)
		}
	}

	badServerIDs := []string{"x/../y", "x?a=b", "x#f", "%2e%2e", "a b", "x\r\ny", ".", "..", strings.Repeat("a", 65), "a/b", "é"}
	for _, id := range badServerIDs {
		if err := p.ValidateCredentials(base(func(c map[string]string) { c["server_id"] = id })); err == nil {
			t.Errorf("server_id %q accepted", id)
		}
	}
}

func TestTestCredentials_BlockedTargetsMakeNoConnection(t *testing.T) {
	p := &PowerDNSProvider{}
	for _, raw := range []string{"http://127.0.0.1:8081", "https://169.254.169.254", "http://100.100.100.200", "http://localhost:8081"} {
		start := time.Now()
		err := p.TestCredentials(map[string]string{"api_url": raw, "api_key": "k"})
		if err == nil {
			t.Fatalf("%s accepted", raw)
		}
		if !errors.Is(err, safehttp.ErrBlockedAddress) {
			t.Errorf("%s: expected blocked-address error, got %v", raw, err)
		}
		if time.Since(start) > 3*time.Second {
			t.Errorf("%s: rejection should be immediate", raw)
		}
	}

	if err := p.TestCredentials(map[string]string{"api_url": "https://pdns.example.com"}); err == nil {
		t.Error("missing api_key accepted")
	}
}

func probeTarget(t *testing.T, raw string) *url.URL {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatal(err)
	}
	return u
}

func TestProbe_StatusMappingAndNoBodyEcho(t *testing.T) {
	p := &PowerDNSProvider{}
	const marker = "SECRET-BODY-MARKER"

	var gotPath, gotKey atomic.Value
	status := atomic.Int32{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath.Store(r.URL.EscapedPath())
		gotKey.Store(r.Header.Get("X-API-Key"))
		w.WriteHeader(int(status.Load()))
		_, _ = w.Write([]byte(marker))
	}))
	defer srv.Close()

	hc := srv.Client()
	hc.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }

	status.Store(http.StatusOK)
	if err := p.probe(hc, probeTarget(t, srv.URL+"/pdns/"), "ns1", "my-key"); err != nil {
		t.Fatalf("200 should succeed: %v", err)
	}
	if gotPath.Load() != "/pdns/api/v1/servers/ns1" {
		t.Errorf("path = %v", gotPath.Load())
	}
	if gotKey.Load() != "my-key" {
		t.Errorf("api key header = %v", gotKey.Load())
	}

	for _, code := range []int32{http.StatusUnauthorized, http.StatusForbidden, http.StatusNotFound, http.StatusFound, http.StatusInternalServerError} {
		status.Store(code)
		err := p.probe(hc, probeTarget(t, srv.URL), "localhost", "k")
		if err == nil {
			t.Errorf("status %d should fail", code)
			continue
		}
		if !strings.Contains(err.Error(), "API returned status") {
			t.Errorf("status %d: unexpected error %v", code, err)
		}
		if strings.Contains(err.Error(), marker) {
			t.Errorf("status %d: response body echoed: %v", code, err)
		}
	}
}

func TestProbe_DialTimeBlockMapsToRemedyAndSendsNothing(t *testing.T) {
	p := &PowerDNSProvider{}
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { hits.Add(1) }))
	defer srv.Close()

	// The production client has no loopback allowance, so the dialer alone must stop this.
	err := p.probe(safehttp.NewClient(safehttp.PrivateNetworkOK(), 2*time.Second), probeTarget(t, srv.URL), "localhost", "k")
	if err == nil {
		t.Fatal("loopback target accepted")
	}
	if !errors.Is(err, safehttp.ErrBlockedAddress) {
		t.Errorf("expected blocked-address error, got %v", err)
	}
	if !strings.Contains(err.Error(), "use a non-loopback address reachable from Charon") {
		t.Errorf("missing remedy text: %v", err)
	}
	if hits.Load() != 0 {
		t.Errorf("server hit %d times, want 0", hits.Load())
	}
}

func TestProbe_RedirectNotFollowed(t *testing.T) {
	p := &PowerDNSProvider{}
	var secondHits atomic.Int32
	second := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { secondHits.Add(1) }))
	defer second.Close()
	first := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, second.URL, http.StatusTemporaryRedirect)
	}))
	defer first.Close()

	hc := first.Client()
	hc.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	err := p.probe(hc, probeTarget(t, first.URL), "localhost", "k")
	if err == nil || !strings.Contains(err.Error(), "API returned status 307") {
		t.Fatalf("expected status 307 error, got %v", err)
	}
	if secondHits.Load() != 0 {
		t.Errorf("redirect target hit %d times", secondHits.Load())
	}
}

func TestProbe_ConnectionFailureKeepsExistingPrefix(t *testing.T) {
	p := &PowerDNSProvider{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	target := probeTarget(t, srv.URL)
	srv.Close()

	err := p.probe(&http.Client{Timeout: time.Second}, target, "localhost", "k")
	if err == nil || !strings.Contains(err.Error(), "API connection failed") {
		t.Fatalf("expected connection failure, got %v", err)
	}
	if errors.Is(err, safehttp.ErrBlockedAddress) {
		t.Error("plain connection failure must not claim a blocked address")
	}
}

func TestProbe_RejectsUnsafeServerID(t *testing.T) {
	p := &PowerDNSProvider{}
	if err := p.probe(http.DefaultClient, probeTarget(t, "https://pdns.example.com"), "../x", "k"); err == nil {
		t.Fatal("dot-dot server id accepted")
	}
}
