package services

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/security"
)

// setSenderAllowLoopbackForTest enables the test-only loopback seam for the
// duration of the test.
func setSenderAllowLoopbackForTest(t *testing.T) {
	t.Helper()
	prev := senderAllowLoopback
	senderAllowLoopback = true
	t.Cleanup(func() { senderAllowLoopback = prev })
}

func TestSenderAllowLoopbackDefaultsFalse(t *testing.T) {
	assert.False(t, senderAllowLoopback)
}

func TestSenderClientRejectsLoopbackByDefault(t *testing.T) {
	for _, u := range []string{
		"http://127.0.0.1:8080/x", "http://[::1]:8080/x", "http://localhost:8080/x",
		"http://0.0.0.0:8080/x", "http://[::ffff:127.0.0.1]:8080/x", "http://127.0.0.2:8080/x",
	} {
		_, err := security.ValidateExternalURL(u, senderURLOptions()...)
		require.Error(t, err, u)
		assert.Contains(t, err.Error(), "private ip addresses is blocked", u)

		// Dial-time layer must also refuse, independent of URL validation.
		req, reqErr := http.NewRequest(http.MethodPost, u, http.NoBody)
		require.NoError(t, reqErr)
		resp, doErr := newSenderHTTPClient().Do(req)
		if resp != nil {
			_ = resp.Body.Close()
		}
		require.Error(t, doErr, u)
	}
}

func TestSendersRejectLoopbackDestinations(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("loopback server must not be contacted")
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	event := models.SecurityEvent{EventType: "waf_block", Severity: "warn", Message: "x"}
	err := (&SecurityNotificationService{}).sendWebhook(context.Background(), srv.URL, event)
	assert.Error(t, err)
	err = (&EnhancedSecurityNotificationService{}).sendWebhook(context.Background(), srv.URL, event)
	assert.Error(t, err)
}

func TestSenderSeamPermitsLoopbackInTests(t *testing.T) {
	setSenderAllowLoopbackForTest(t)
	var hit atomic.Bool
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hit.Store(true)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	event := models.SecurityEvent{EventType: "waf_block", Severity: "warn", Message: "x"}
	require.NoError(t, (&SecurityNotificationService{}).sendWebhook(context.Background(), srv.URL, event))
	require.True(t, hit.Load())
	hit.Store(false)
	require.NoError(t, (&EnhancedSecurityNotificationService{}).sendWebhook(context.Background(), srv.URL, event))
	assert.True(t, hit.Load())
}

func TestSenderHelpers_AllowCGNATRejectTransitionByDefault(t *testing.T) {
	require.False(t, senderAllowLoopback)

	_, err := security.ValidateExternalURL("http://100.64.0.1/hook", senderURLOptions()...)
	assert.NoError(t, err, "CGNAT must be allowed")
	for _, u := range []string{
		"http://100.100.100.200/hook", "http://127.0.0.1/hook",
		"http://198.18.0.1/hook", "http://[2002::1]/hook", "http://[64:ff9b::808:808]/hook",
	} {
		_, err := security.ValidateExternalURL(u, senderURLOptions()...)
		assert.Error(t, err, u)
	}

	client := newSenderHTTPClient()
	assert.False(t, dialPolicyProbe(t, client, "100.64.0.1:9"), "CGNAT dial refused")
	for _, addr := range []string{"100.100.100.200:9", "127.0.0.1:9", "198.18.0.1:9", "[2002::1]:9", "[64:ff9b::808:808]:9"} {
		assert.True(t, dialPolicyProbe(t, client, addr), addr)
	}
}
