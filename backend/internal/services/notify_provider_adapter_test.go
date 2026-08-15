package services

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"sync"
	"testing"
	"time"

	notify "github.com/Wikid82/go_notify_yourself"
	"github.com/Wikid82/go_notify_yourself/providers/discord"
	"github.com/Wikid82/go_notify_yourself/providers/gotify"
	"github.com/Wikid82/go_notify_yourself/providers/ntfy"
	"github.com/Wikid82/go_notify_yourself/providers/pushover"
	"github.com/Wikid82/go_notify_yourself/providers/slack"
	"github.com/Wikid82/go_notify_yourself/providers/telegram"
	"github.com/Wikid82/go_notify_yourself/providers/webhook"
	"github.com/Wikid82/go_notify_yourself/transport"

	"github.com/Wikid82/charon/backend/internal/models"
)

// capturingRoundTripper is a fake http.RoundTripper that records every
// outbound request (method, URL, headers, body) and returns a canned
// response (200 OK by default, or statusCode when set) without hitting any
// real network destination — including providers like pushover/telegram
// whose dispatch URL is hardcoded to a production API host.
type capturingRoundTripper struct {
	mu         sync.Mutex
	requests   []*http.Request
	bodies     [][]byte
	statusCode int
}

func (c *capturingRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	var body []byte
	if req.Body != nil {
		body, _ = io.ReadAll(req.Body)
		_ = req.Body.Close()
	}
	c.requests = append(c.requests, req)
	c.bodies = append(c.bodies, body)

	status := c.statusCode
	if status == 0 {
		status = http.StatusOK
	}

	return &http.Response{
		StatusCode: status,
		Body:       io.NopCloser(bytes.NewReader(nil)),
		Header:     make(http.Header),
	}, nil
}

func (c *capturingRoundTripper) last() (*http.Request, []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()
	n := len(c.requests)
	if n == 0 {
		return nil, nil
	}
	return c.requests[n-1], c.bodies[n-1]
}

// newCapturingWrapper builds a *transport.Wrapper whose ClientFactory routes
// every outbound request through a capturingRoundTripper (so tests can
// inspect exactly what was dispatched) and whose URLValidator is a
// pass-through (SSRF policy is exercised by notify_client_adapter's own
// tests, not here).
func newCapturingWrapper() (*transport.Wrapper, *capturingRoundTripper) {
	rt := &capturingRoundTripper{}
	w := transport.NewWrapper(
		transport.WithClientFactory(func(bool, int) *http.Client {
			return &http.Client{Transport: rt}
		}),
		transport.WithURLValidator(func(rawURL string, _ bool) (string, error) {
			return rawURL, nil
		}),
	)
	return w, rt
}

func TestBuildNotifySenderDiscord(t *testing.T) {
	w, rt := newCapturingWrapper()
	provider := models.NotificationProvider{
		Type:     "discord",
		URL:      "https://discord.com/api/webhooks/123456/abcdef",
		Template: "minimal",
	}

	sender, err := buildNotifySender(provider, w)
	if err != nil {
		t.Fatalf("buildNotifySender returned error: %v", err)
	}
	if _, ok := sender.(*discord.Client); !ok {
		t.Fatalf("expected *discord.Client, got %T", sender)
	}

	if err := sender.Send(context.Background(), notify.Message{Title: "t", Body: "hello", EventType: "test"}); err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	req, body := rt.last()
	if req == nil {
		t.Fatal("expected a request to be captured")
	}
	if req.URL.String() != provider.URL {
		t.Fatalf("dispatch URL = %q, want %q", req.URL.String(), provider.URL)
	}

	var payload map[string]any
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("invalid JSON payload: %v", err)
	}
	if payload["content"] != "hello" {
		t.Fatalf("expected content fallback from message, got %v", payload)
	}
}

func TestBuildNotifySenderSlackUsesTokenAsWebhookURL(t *testing.T) {
	w, rt := newCapturingWrapper()
	provider := models.NotificationProvider{
		Type:     "slack",
		URL:      "unused-placeholder",
		Token:    "https://hooks.slack.com/services/T000/B000/xxxxxxxxxxxxxxxxxxxxxxxx",
		Template: "minimal",
	}

	sender, err := buildNotifySender(provider, w)
	if err != nil {
		t.Fatalf("buildNotifySender returned error: %v", err)
	}
	if _, ok := sender.(*slack.Client); !ok {
		t.Fatalf("expected *slack.Client, got %T", sender)
	}

	if err := sender.Send(context.Background(), notify.Message{Title: "t", Body: "hello", EventType: "test"}); err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	req, body := rt.last()
	if req.URL.String() != provider.Token {
		t.Fatalf("dispatch URL = %q, want provider.Token %q", req.URL.String(), provider.Token)
	}
	var payload map[string]any
	_ = json.Unmarshal(body, &payload)
	if payload["text"] != "hello" {
		t.Fatalf("expected text fallback from message, got %v", payload)
	}
}

func TestBuildNotifySenderGotifySetsAuthHeader(t *testing.T) {
	w, rt := newCapturingWrapper()
	provider := models.NotificationProvider{
		Type:     "gotify",
		URL:      "https://gotify.example.com/message",
		Token:    "app-token-123",
		Template: "minimal",
	}

	sender, err := buildNotifySender(provider, w)
	if err != nil {
		t.Fatalf("buildNotifySender returned error: %v", err)
	}
	if _, ok := sender.(*gotify.Client); !ok {
		t.Fatalf("expected *gotify.Client, got %T", sender)
	}

	if err := sender.Send(context.Background(), notify.Message{Title: "t", Body: "hello", EventType: "test"}); err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	req, _ := rt.last()
	if req.URL.String() != provider.URL {
		t.Fatalf("dispatch URL = %q, want %q", req.URL.String(), provider.URL)
	}
	if got := req.Header.Get("X-Gotify-Key"); got != provider.Token {
		t.Fatalf("X-Gotify-Key header = %q, want %q", got, provider.Token)
	}
}

func TestBuildNotifySenderPushoverBuildsProductionURLAndInjectsCredentials(t *testing.T) {
	w, rt := newCapturingWrapper()
	provider := models.NotificationProvider{
		Type:     "pushover",
		URL:      "user-key-abc",
		Token:    "api-token-xyz",
		Template: "minimal",
	}

	sender, err := buildNotifySender(provider, w)
	if err != nil {
		t.Fatalf("buildNotifySender returned error: %v", err)
	}
	if _, ok := sender.(*pushover.Client); !ok {
		t.Fatalf("expected *pushover.Client, got %T", sender)
	}

	if err := sender.Send(context.Background(), notify.Message{Title: "t", Body: "hello", EventType: "test"}); err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	req, body := rt.last()
	wantURL := "https://api.pushover.net/1/messages.json"
	if req.URL.String() != wantURL {
		t.Fatalf("dispatch URL = %q, want %q", req.URL.String(), wantURL)
	}
	var payload map[string]any
	_ = json.Unmarshal(body, &payload)
	if payload["token"] != provider.Token {
		t.Fatalf("payload token = %v, want %q", payload["token"], provider.Token)
	}
	if payload["user"] != provider.URL {
		t.Fatalf("payload user = %v, want %q", payload["user"], provider.URL)
	}
}

func TestBuildNotifySenderNtfySetsBearerHeader(t *testing.T) {
	w, rt := newCapturingWrapper()
	provider := models.NotificationProvider{
		Type:     "ntfy",
		URL:      "https://ntfy.sh/my-topic",
		Token:    "ntfy-token",
		Template: "minimal",
	}

	sender, err := buildNotifySender(provider, w)
	if err != nil {
		t.Fatalf("buildNotifySender returned error: %v", err)
	}
	if _, ok := sender.(*ntfy.Client); !ok {
		t.Fatalf("expected *ntfy.Client, got %T", sender)
	}

	if err := sender.Send(context.Background(), notify.Message{Title: "t", Body: "hello", EventType: "test"}); err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	req, _ := rt.last()
	if req.URL.String() != provider.URL {
		t.Fatalf("dispatch URL = %q, want %q", req.URL.String(), provider.URL)
	}
	if got := req.Header.Get("Authorization"); got != "Bearer "+provider.Token {
		t.Fatalf("Authorization header = %q, want %q", got, "Bearer "+provider.Token)
	}
}

func TestBuildNotifySenderTelegramBuildsProductionURLAndChatID(t *testing.T) {
	w, rt := newCapturingWrapper()
	provider := models.NotificationProvider{
		Type:     "telegram",
		URL:      "chat-id-456",
		Token:    "bot-token-789",
		Template: "minimal",
	}

	sender, err := buildNotifySender(provider, w)
	if err != nil {
		t.Fatalf("buildNotifySender returned error: %v", err)
	}
	if _, ok := sender.(*telegram.Client); !ok {
		t.Fatalf("expected *telegram.Client, got %T", sender)
	}

	if err := sender.Send(context.Background(), notify.Message{Title: "t", Body: "hello", EventType: "test"}); err != nil {
		t.Fatalf("Send failed: %v", err)
	}

	req, body := rt.last()
	wantURL := "https://api.telegram.org/bot" + provider.Token + "/sendMessage"
	if req.URL.String() != wantURL {
		t.Fatalf("dispatch URL = %q, want %q", req.URL.String(), wantURL)
	}
	var payload map[string]any
	_ = json.Unmarshal(body, &payload)
	if payload["chat_id"] != provider.URL {
		t.Fatalf("payload chat_id = %v, want %q", payload["chat_id"], provider.URL)
	}
	if payload["text"] != "hello" {
		t.Fatalf("expected text fallback from message, got %v", payload)
	}
}

func TestBuildNotifySenderWebhookGeneric(t *testing.T) {
	for _, providerType := range []string{"webhook", "generic"} {
		t.Run(providerType, func(t *testing.T) {
			w, rt := newCapturingWrapper()
			provider := models.NotificationProvider{
				Type:     providerType,
				URL:      "https://example.com/hook",
				Template: "minimal",
			}

			sender, err := buildNotifySender(provider, w)
			if err != nil {
				t.Fatalf("buildNotifySender returned error: %v", err)
			}
			if _, ok := sender.(*webhook.Client); !ok {
				t.Fatalf("expected *webhook.Client, got %T", sender)
			}

			if err := sender.Send(context.Background(), notify.Message{Title: "t", Body: "hello", EventType: "test"}); err != nil {
				t.Fatalf("Send failed: %v", err)
			}

			req, _ := rt.last()
			if req.URL.String() != provider.URL {
				t.Fatalf("dispatch URL = %q, want %q", req.URL.String(), provider.URL)
			}
		})
	}
}

func TestBuildNotifySenderUnsupportedTypeErrors(t *testing.T) {
	w, _ := newCapturingWrapper()
	provider := models.NotificationProvider{Type: "carrier-pigeon"}

	_, err := buildNotifySender(provider, w)
	if err == nil {
		t.Fatal("expected an error for an unsupported provider type")
	}
}

func TestResolveTemplateFieldsPassesThroughMinimalAndCustom(t *testing.T) {
	minimal := models.NotificationProvider{Template: "minimal", Config: ""}
	if tmpl, custom := resolveTemplateFields(minimal); tmpl != "minimal" || custom != "" {
		t.Fatalf("minimal: got (%q, %q)", tmpl, custom)
	}

	custom := models.NotificationProvider{Template: "custom", Config: `{"foo": {{toJSON .Title}}}`}
	if tmpl, cfg := resolveTemplateFields(custom); tmpl != "custom" || cfg != custom.Config {
		t.Fatalf("custom: got (%q, %q)", tmpl, cfg)
	}
}

func TestResolveTemplateFieldsTranslatesDetailedToLegacyFlatShape(t *testing.T) {
	provider := models.NotificationProvider{Template: "detailed", Config: ""}

	tmpl, custom := resolveTemplateFields(provider)
	if tmpl != "custom" {
		t.Fatalf("expected template to become %q, got %q", "custom", tmpl)
	}
	if custom != legacyDetailedTemplate {
		t.Fatalf("expected custom template to be legacyDetailedTemplate, got %q", custom)
	}
}

// TestDetailedTemplateBackwardCompatMatchesOldFlatJSONShape renders the
// backward-compat legacyDetailedTemplate via providers/webhook.RenderPreview
// (the module's public template-preview function, replacing the old
// RenderTemplate) and asserts the resulting JSON keys/values exactly match
// what the OLD Charon detailedTemplate const in notification_service.go
// would have produced for the same inputs — proving the "detailed" ->
// flat-shape backward-compat translation (extraction spec §3.6 step 5) is
// implemented faithfully.
func TestDetailedTemplateBackwardCompatMatchesOldFlatJSONShape(t *testing.T) {
	fixedTime := time.Date(2026, 8, 15, 12, 0, 0, 0, time.UTC)
	msg := notify.Message{
		Title:     "Certificate Renewed",
		Body:      "Certificate renewed for example.com",
		EventType: "cert",
		Timestamp: fixedTime,
		Data: map[string]any{
			"HostName":     "example.com",
			"HostIP":       "1.2.3.4",
			"ServiceCount": float64(3),
			"Services":     []any{"web", "api", "admin"},
		},
	}

	_, customTemplate := resolveTemplateFields(models.NotificationProvider{Template: "detailed"})

	rendered, parsed, err := webhook.RenderPreview(customTemplate, msg)
	if err != nil {
		t.Fatalf("RenderPreview failed: %v (rendered=%s)", err, rendered)
	}

	parsedMap, ok := parsed.(map[string]any)
	if !ok {
		t.Fatalf("expected parsed output to be a JSON object, got %T", parsed)
	}

	want := map[string]any{
		"title":         "Certificate Renewed",
		"message":       "Certificate renewed for example.com",
		"time":          fixedTime.Format(time.RFC3339),
		"event":         "cert",
		"host":          "example.com",
		"host_ip":       "1.2.3.4",
		"service_count": float64(3),
	}
	for key, wantVal := range want {
		if got := parsedMap[key]; got != wantVal {
			t.Fatalf("key %q = %v (%T), want %v (%T)", key, got, got, wantVal, wantVal)
		}
	}

	services, ok := parsedMap["services"].([]any)
	if !ok || len(services) != 3 {
		t.Fatalf("expected services to be a 3-element array, got %v", parsedMap["services"])
	}

	dataField, ok := parsedMap["data"].(map[string]any)
	if !ok {
		t.Fatalf("expected data field to be a JSON object, got %v", parsedMap["data"])
	}
	if dataField["HostName"] != "example.com" {
		t.Fatalf("expected data.HostName to be preserved, got %v", dataField["HostName"])
	}
}
