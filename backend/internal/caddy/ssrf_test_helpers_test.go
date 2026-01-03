package caddy

import (
	"net/url"
	"strconv"
	"testing"
)

func expectedPortFromURL(t *testing.T, raw string) int {
	t.Helper()
	u, err := url.Parse(raw)
	if err != nil {
		t.Fatalf("failed to parse url %q: %v", raw, err)
	}
	p := u.Port()
	if p == "" {
		t.Fatalf("expected explicit port in url %q", raw)
	}
	port, err := strconv.Atoi(p)
	if err != nil {
		t.Fatalf("failed to parse port %q from url %q: %v", p, raw, err)
	}
	return port
}

func newTestClient(t *testing.T, raw string) *Client {
	t.Helper()
	return NewClientWithExpectedPort(raw, expectedPortFromURL(t, raw))
}
