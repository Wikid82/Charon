package services

import (
	"context"
	"net/http"
	"net/http/httptest"
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
	for _, u := range []string{"http://127.0.0.1:9/x", "http://[::1]:9/x", "http://localhost:9/x"} {
		_, err := security.ValidateExternalURL(u, senderURLOptions()...)
		assert.Error(t, err, u)

		req, reqErr := http.NewRequest(http.MethodPost, u, nil)
		require.NoError(t, reqErr)
		resp, doErr := newSenderHTTPClient().Do(req)
		if resp != nil {
			_ = resp.Body.Close()
		}
		assert.Error(t, doErr, u)
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
	hit := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		hit = true
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	event := models.SecurityEvent{EventType: "waf_block", Severity: "warn", Message: "x"}
	require.NoError(t, (&SecurityNotificationService{}).sendWebhook(context.Background(), srv.URL, event))
	require.True(t, hit)
	hit = false
	require.NoError(t, (&EnhancedSecurityNotificationService{}).sendWebhook(context.Background(), srv.URL, event))
	assert.True(t, hit)
}
