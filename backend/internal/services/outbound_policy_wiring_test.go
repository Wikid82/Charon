package services

import (
	"context"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/network"
)

// dialPolicyProbe dials addr through the client's transport and reports whether
// the address policy refused it. An allowed address fails later (nothing
// listens there) without the policy sentinel.
func dialPolicyProbe(t *testing.T, client *http.Client, addr string) bool {
	t.Helper()
	transport, ok := client.Transport.(*http.Transport)
	if !ok {
		t.Fatalf("unexpected transport type %T", client.Transport)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()
	conn, err := transport.DialContext(ctx, "tcp", addr)
	if err == nil {
		_ = conn.Close()
		return false
	}
	return errors.Is(err, network.ErrBlockedAddress)
}

func TestOutboundClients_OverlayDestinations(t *testing.T) {
	clients := map[string]*http.Client{
		"uptime": newUptimeChecker(&UptimeService{}).httpClient,
		"notify": notifyClientFactory(true, 0),
	}
	for name, client := range clients {
		if dialPolicyProbe(t, client, "100.64.0.1:9") {
			t.Errorf("%s: overlay address refused", name)
		}
		if !dialPolicyProbe(t, client, "100.100.100.200:9") {
			t.Errorf("%s: metadata alias allowed", name)
		}
		if !dialPolicyProbe(t, client, "169.254.169.254:9") {
			t.Errorf("%s: link-local allowed", name)
		}
		for _, addr := range []string{"198.18.0.1:9", "192.0.0.1:9", "[2002::1]:9", "[64:ff9b::808:808]:9"} {
			if !dialPolicyProbe(t, client, addr) {
				t.Errorf("%s: %s allowed", name, addr)
			}
		}
	}
}

func TestNotifyURLValidator_OverlayDestinations(t *testing.T) {
	t.Parallel()
	if _, err := notifyURLValidator("http://100.64.0.1/hook", true); err != nil {
		t.Errorf("overlay address rejected: %v", err)
	}
	if _, err := notifyURLValidator("http://100.100.100.200/hook", true); err == nil {
		t.Error("metadata alias accepted")
	}
	if _, err := notifyURLValidator("http://198.18.0.1/hook", true); err == nil {
		t.Error("benchmark-range address accepted")
	}
}

func TestSecurityWebhooks_OverlayDestinations(t *testing.T) {
	event := models.SecurityEvent{EventType: "waf_block", Severity: "high", Message: "probe"}
	senders := map[string]func(ctx context.Context, url string) error{
		"legacy": func(ctx context.Context, url string) error {
			return (&SecurityNotificationService{}).sendWebhook(ctx, url, event)
		},
		"enhanced": func(ctx context.Context, url string) error {
			return (&EnhancedSecurityNotificationService{}).sendWebhook(ctx, url, event)
		},
	}
	for name, send := range senders {
		ctx, cancel := context.WithTimeout(context.Background(), 400*time.Millisecond)
		err := send(ctx, "http://100.64.0.1:9/hook")
		cancel()
		if err != nil && errors.Is(err, network.ErrBlockedAddress) {
			t.Errorf("%s: overlay address refused: %v", name, err)
		}
		if err := send(context.Background(), "http://100.100.100.200:9/hook"); err == nil {
			t.Errorf("%s: metadata alias accepted", name)
		}
		if err := send(context.Background(), "http://198.18.0.1:9/hook"); err == nil {
			t.Errorf("%s: benchmark-range address accepted", name)
		}
	}
}
