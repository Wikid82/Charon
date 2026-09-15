package services

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/Wikid82/go_notify_yourself/providers/webpush"
	"github.com/Wikid82/go_notify_yourself/transport"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

func setupWebPushAdapterTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.NotificationProvider{}, &models.WebPushSubscription{}))
	return db
}

// statusByEndpointRoundTripper is a fake http.RoundTripper that returns a
// configurable status code per request URL (defaulting to 200), so a test
// can drive different subscriptions in the same dispatch to different
// outcomes (success / 404 / 500) without hitting any real network
// destination. It also counts requests per URL for fan-out assertions.
type statusByEndpointRoundTripper struct {
	mu            sync.Mutex
	statusByURL   map[string]int
	requestCounts map[string]int
}

func newStatusByEndpointRoundTripper() *statusByEndpointRoundTripper {
	return &statusByEndpointRoundTripper{
		statusByURL:   map[string]int{},
		requestCounts: map[string]int{},
	}
}

func (rt *statusByEndpointRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	url := req.URL.String()
	rt.requestCounts[url]++
	status := rt.statusByURL[url]
	if status == 0 {
		status = http.StatusCreated
	}
	return &http.Response{
		StatusCode: status,
		Body:       http.NoBody,
		Header:     make(http.Header),
	}, nil
}

func (rt *statusByEndpointRoundTripper) count(url string) int {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	return rt.requestCounts[url]
}

func newWebPushTestWrapper(rt http.RoundTripper) *transport.Wrapper {
	return transport.NewWrapper(
		transport.WithURLValidator(func(rawURL string, _ bool) (string, error) { return rawURL, nil }),
		transport.WithClientFactory(func(bool, int) *http.Client {
			return &http.Client{Transport: rt}
		}),
		transport.WithRetryPolicy(transport.RetryPolicy{MaxAttempts: 1}),
	)
}

// validSubscriberKeys generates a fresh, valid P256dh/Auth pair so a
// subscription can reach webpush.Client.Send's encryption step without
// error — mirrors go_notify_yourself's own providers/webpush test helper
// (testSubscriber, webpush_test.go), duplicated here since it's unexported.
func validSubscriberKeys(t *testing.T) (p256dh, auth string) {
	t.Helper()
	receiver, err := ecdh.P256().GenerateKey(rand.Reader)
	require.NoError(t, err)
	authSecret := make([]byte, 16)
	_, err = rand.Read(authSecret)
	require.NoError(t, err)
	return base64.RawURLEncoding.EncodeToString(receiver.PublicKey().Bytes()), base64.RawURLEncoding.EncodeToString(authSecret)
}

func validWebPushProvider(t *testing.T) models.NotificationProvider {
	t.Helper()
	pub, priv, err := webpush.GenerateVAPIDKeyPair()
	require.NoError(t, err)
	return models.NotificationProvider{
		ID:            "provider-1",
		Name:          "Browser Push",
		Type:          "webpush",
		Enabled:       true,
		Token:         priv,
		ServiceConfig: fmt.Sprintf(`{"vapid_public_key":%q,"vapid_subject":"mailto:ops@example.com"}`, pub),
		Template:      "minimal",
	}
}

// --- extractHTTPStatusFromNotifyError ---

func TestExtractHTTPStatusFromNotifyError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantOK     bool
	}{
		{
			name:       "nil error",
			err:        nil,
			wantStatus: 0,
			wantOK:     false,
		},
		{
			name:       "status with hint, unwrapped (transport.Wrapper's own shape)",
			err:        errors.New("provider returned status 410: subscription has unsubscribed or expired"),
			wantStatus: 410,
			wantOK:     true,
		},
		{
			name:       "status without hint",
			err:        errors.New("provider returned status 404"),
			wantStatus: 404,
			wantOK:     true,
		},
		{
			name:       "wrapped by webpush.Client.Send's 'failed to send web push' prefix",
			err:        fmt.Errorf("failed to send web push: %w", errors.New("provider returned status 410: gone")),
			wantStatus: 410,
			wantOK:     true,
		},
		{
			name:       "unrelated error text",
			err:        errors.New("connection refused"),
			wantStatus: 0,
			wantOK:     false,
		},
		{
			name:       "provider request failed after retries (no status embedded)",
			err:        errors.New("provider request failed after retries: connection reset"),
			wantStatus: 0,
			wantOK:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, ok := extractHTTPStatusFromNotifyError(tt.err)
			assert.Equal(t, tt.wantOK, ok)
			assert.Equal(t, tt.wantStatus, status)
		})
	}
}

// --- dispatchWebPushViaNotify: guard-clause paths ---

func TestDispatchWebPushViaNotify_MalformedServiceConfigLogsAndReturns(t *testing.T) {
	db := setupWebPushAdapterTestDB(t)
	provider := models.NotificationProvider{ID: "p1", Type: "webpush", Token: "priv", ServiceConfig: "not-json"}
	require.NoError(t, db.Create(&provider).Error)
	sub := models.WebPushSubscription{ProviderID: provider.ID, UserID: "u1", Endpoint: "https://push.example.net/a", P256dh: "x", Auth: "y"}
	require.NoError(t, db.Create(&sub).Error)

	rt := newStatusByEndpointRoundTripper()
	svc := NewNotificationService(db, nil, WithNotifyTransportWrapper(newWebPushTestWrapper(rt)))

	// Must not panic; must not touch the subscription row.
	svc.dispatchWebPushViaNotify(context.Background(), provider, "test", "Title", "Message", nil)

	assert.Equal(t, 0, rt.count("https://push.example.net/a"), "no HTTP request should be sent for a malformed service config")
}

func TestDispatchWebPushViaNotify_MissingVAPIDFieldsLogsAndReturns(t *testing.T) {
	db := setupWebPushAdapterTestDB(t)
	provider := models.NotificationProvider{
		ID: "p1", Type: "webpush", Token: "", // missing private key
		ServiceConfig: `{"vapid_public_key":"BN...","vapid_subject":"mailto:ops@example.com"}`,
	}
	require.NoError(t, db.Create(&provider).Error)
	sub := models.WebPushSubscription{ProviderID: provider.ID, UserID: "u1", Endpoint: "https://push.example.net/b", P256dh: "x", Auth: "y"}
	require.NoError(t, db.Create(&sub).Error)

	rt := newStatusByEndpointRoundTripper()
	svc := NewNotificationService(db, nil, WithNotifyTransportWrapper(newWebPushTestWrapper(rt)))

	svc.dispatchWebPushViaNotify(context.Background(), provider, "test", "Title", "Message", nil)

	assert.Equal(t, 0, rt.count("https://push.example.net/b"))
}

func TestDispatchWebPushViaNotify_NoSubscriptionsIsNoop(t *testing.T) {
	db := setupWebPushAdapterTestDB(t)
	provider := validWebPushProvider(t)
	require.NoError(t, db.Create(&provider).Error)

	rt := newStatusByEndpointRoundTripper()
	svc := NewNotificationService(db, nil, WithNotifyTransportWrapper(newWebPushTestWrapper(rt)))

	// Should return cleanly with zero subscription rows and no HTTP calls.
	svc.dispatchWebPushViaNotify(context.Background(), provider, "test", "Title", "Message", nil)
}

// --- dispatchWebPushViaNotify: fan-out ---

func TestDispatchWebPushViaNotify_FansOutToEverySubscription(t *testing.T) {
	db := setupWebPushAdapterTestDB(t)
	provider := validWebPushProvider(t)
	require.NoError(t, db.Create(&provider).Error)

	p256dhA, authA := validSubscriberKeys(t)
	p256dhB, authB := validSubscriberKeys(t)
	subA := models.WebPushSubscription{ProviderID: provider.ID, UserID: "u1", Endpoint: "https://push.example.net/subA", P256dh: p256dhA, Auth: authA, FailureCount: 3}
	subB := models.WebPushSubscription{ProviderID: provider.ID, UserID: "u2", Endpoint: "https://push.example.net/subB", P256dh: p256dhB, Auth: authB}
	require.NoError(t, db.Create(&subA).Error)
	require.NoError(t, db.Create(&subB).Error)

	rt := newStatusByEndpointRoundTripper() // both default to 201 (success)
	svc := NewNotificationService(db, nil, WithNotifyTransportWrapper(newWebPushTestWrapper(rt)))

	svc.dispatchWebPushViaNotify(context.Background(), provider, "test", "Title", "Message", nil)

	assert.Equal(t, 1, rt.count(subA.Endpoint))
	assert.Equal(t, 1, rt.count(subB.Endpoint))

	var reloadedA, reloadedB models.WebPushSubscription
	require.NoError(t, db.First(&reloadedA, "id = ?", subA.ID).Error)
	require.NoError(t, db.First(&reloadedB, "id = ?", subB.ID).Error)
	assert.Equal(t, 0, reloadedA.FailureCount, "successful send resets FailureCount")
	assert.False(t, reloadedA.LastSeenAt.IsZero())
	assert.False(t, reloadedB.LastSeenAt.IsZero())
}

func TestDispatchWebPushViaNotify_Prunes404And410Immediately(t *testing.T) {
	db := setupWebPushAdapterTestDB(t)
	provider := validWebPushProvider(t)
	require.NoError(t, db.Create(&provider).Error)

	p256dh404, auth404 := validSubscriberKeys(t)
	p256dh410, auth410 := validSubscriberKeys(t)
	sub404 := models.WebPushSubscription{ProviderID: provider.ID, UserID: "u1", Endpoint: "https://push.example.net/gone404", P256dh: p256dh404, Auth: auth404}
	sub410 := models.WebPushSubscription{ProviderID: provider.ID, UserID: "u2", Endpoint: "https://push.example.net/gone410", P256dh: p256dh410, Auth: auth410}
	require.NoError(t, db.Create(&sub404).Error)
	require.NoError(t, db.Create(&sub410).Error)

	rt := newStatusByEndpointRoundTripper()
	rt.statusByURL[sub404.Endpoint] = http.StatusNotFound
	rt.statusByURL[sub410.Endpoint] = http.StatusGone
	svc := NewNotificationService(db, nil, WithNotifyTransportWrapper(newWebPushTestWrapper(rt)))

	svc.dispatchWebPushViaNotify(context.Background(), provider, "test", "Title", "Message", nil)

	var count int64
	require.NoError(t, db.Model(&models.WebPushSubscription{}).Where("provider_id = ?", provider.ID).Count(&count).Error)
	assert.Equal(t, int64(0), count, "both 404 and 410 subscriptions should be pruned")
}

func TestDispatchWebPushViaNotify_NonGoneFailureIncrementsFailureCountBelowThreshold(t *testing.T) {
	db := setupWebPushAdapterTestDB(t)
	provider := validWebPushProvider(t)
	require.NoError(t, db.Create(&provider).Error)

	p256dh, auth := validSubscriberKeys(t)
	sub := models.WebPushSubscription{ProviderID: provider.ID, UserID: "u1", Endpoint: "https://push.example.net/flaky", P256dh: p256dh, Auth: auth, FailureCount: 3}
	require.NoError(t, db.Create(&sub).Error)

	rt := newStatusByEndpointRoundTripper()
	rt.statusByURL[sub.Endpoint] = http.StatusInternalServerError
	svc := NewNotificationService(db, nil, WithNotifyTransportWrapper(newWebPushTestWrapper(rt)))

	svc.dispatchWebPushViaNotify(context.Background(), provider, "test", "Title", "Message", nil)

	var reloaded models.WebPushSubscription
	require.NoError(t, db.First(&reloaded, "id = ?", sub.ID).Error)
	assert.Equal(t, 4, reloaded.FailureCount)
	require.NotNil(t, reloaded.LastFailureAt)
}

func TestDispatchWebPushViaNotify_PrunesAtMaxConsecutiveFailures(t *testing.T) {
	db := setupWebPushAdapterTestDB(t)
	provider := validWebPushProvider(t)
	require.NoError(t, db.Create(&provider).Error)

	p256dh, auth := validSubscriberKeys(t)
	sub := models.WebPushSubscription{ProviderID: provider.ID, UserID: "u1", Endpoint: "https://push.example.net/dying", P256dh: p256dh, Auth: auth, FailureCount: webpushMaxConsecutiveFailures - 1}
	require.NoError(t, db.Create(&sub).Error)

	rt := newStatusByEndpointRoundTripper()
	rt.statusByURL[sub.Endpoint] = http.StatusInternalServerError
	svc := NewNotificationService(db, nil, WithNotifyTransportWrapper(newWebPushTestWrapper(rt)))

	svc.dispatchWebPushViaNotify(context.Background(), provider, "test", "Title", "Message", nil)

	var count int64
	require.NoError(t, db.Model(&models.WebPushSubscription{}).Where("id = ?", sub.ID).Count(&count).Error)
	assert.Equal(t, int64(0), count, "subscription should be pruned once FailureCount reaches the max threshold")
}

// --- recordWebPushSendResult: direct unit coverage of the branch logic ---

func TestRecordWebPushSendResult_SuccessResetsFailureCount(t *testing.T) {
	db := setupWebPushAdapterTestDB(t)
	provider := models.NotificationProvider{ID: "p1", Type: "webpush"}
	require.NoError(t, db.Create(&provider).Error)
	sub := models.WebPushSubscription{ProviderID: provider.ID, UserID: "u1", Endpoint: "https://push.example.net/ok", P256dh: "x", Auth: "y", FailureCount: 5}
	require.NoError(t, db.Create(&sub).Error)

	svc := NewNotificationService(db, nil)
	svc.recordWebPushSendResult(sub, nil)

	var reloaded models.WebPushSubscription
	require.NoError(t, db.First(&reloaded, "id = ?", sub.ID).Error)
	assert.Equal(t, 0, reloaded.FailureCount)
	assert.WithinDuration(t, time.Now(), reloaded.LastSeenAt, 5*time.Second)
}

func TestExtractHTTPStatusFromNotifyError_UnparsableNumberIsNotOK(t *testing.T) {
	// A status string wide enough to overflow int (still matches \d+, but
	// strconv.Atoi fails) exercises the defensive fallback branch.
	status, ok := extractHTTPStatusFromNotifyError(errors.New("provider returned status 99999999999999999999"))
	assert.False(t, ok)
	assert.Equal(t, 0, status)
}

func TestDispatchWebPushViaNotify_SubscriptionLoadErrorLogsAndReturns(t *testing.T) {
	db := setupWebPushAdapterTestDB(t)
	provider := validWebPushProvider(t)
	require.NoError(t, db.Create(&provider).Error)

	// Drop the table out from under the query to force a DB error on the
	// subscriptions Find call, without panicking the dispatch path.
	require.NoError(t, db.Migrator().DropTable(&models.WebPushSubscription{}))

	rt := newStatusByEndpointRoundTripper()
	svc := NewNotificationService(db, nil, WithNotifyTransportWrapper(newWebPushTestWrapper(rt)))

	svc.dispatchWebPushViaNotify(context.Background(), provider, "test", "Title", "Message", nil)
}

func TestRecordWebPushSendResult_DBErrorsAreLoggedNotPanicked(t *testing.T) {
	db := setupWebPushAdapterTestDB(t)
	provider := models.NotificationProvider{ID: "p1", Type: "webpush"}
	require.NoError(t, db.Create(&provider).Error)
	sub := models.WebPushSubscription{ProviderID: provider.ID, UserID: "u1", Endpoint: "https://push.example.net/closed", P256dh: "x", Auth: "y", FailureCount: webpushMaxConsecutiveFailures - 1}
	require.NoError(t, db.Create(&sub).Error)

	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	svc := NewNotificationService(db, nil)

	// Each branch's DB write now fails (connection closed) — all must be
	// logged, never panicked.
	assert.NotPanics(t, func() { svc.recordWebPushSendResult(sub, nil) })
	assert.NotPanics(t, func() {
		svc.recordWebPushSendResult(sub, errors.New("provider returned status 410: gone"))
	})
	assert.NotPanics(t, func() {
		svc.recordWebPushSendResult(sub, errors.New("provider returned status 500"))
	})
	lowFailureSub := sub
	lowFailureSub.FailureCount = 0
	assert.NotPanics(t, func() {
		svc.recordWebPushSendResult(lowFailureSub, errors.New("provider returned status 500"))
	})
}

func TestRecordWebPushSendResult_AmbiguousFailureNoStatusStillCountsAsFailure(t *testing.T) {
	db := setupWebPushAdapterTestDB(t)
	provider := models.NotificationProvider{ID: "p1", Type: "webpush"}
	require.NoError(t, db.Create(&provider).Error)
	sub := models.WebPushSubscription{ProviderID: provider.ID, UserID: "u1", Endpoint: "https://push.example.net/ambiguous", P256dh: "x", Auth: "y"}
	require.NoError(t, db.Create(&sub).Error)

	svc := NewNotificationService(db, nil)
	svc.recordWebPushSendResult(sub, errors.New("outbound request failed: connection refused"))

	var reloaded models.WebPushSubscription
	require.NoError(t, db.First(&reloaded, "id = ?", sub.ID).Error)
	assert.Equal(t, 1, reloaded.FailureCount)
}
