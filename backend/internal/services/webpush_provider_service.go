package services

import (
	"encoding/json"
	"errors"
	"fmt"
	neturl "net/url"
	"strings"
	"time"

	"github.com/Wikid82/go_notify_yourself/providers/webpush"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
)

// Sentinel errors surfaced by the Web Push provisioning/subscription
// service methods below (docs/plans/current_spec.md §3.4/§3.8). Handlers
// map these to specific HTTP status codes via errors.Is rather than
// string-matching, mirroring this codebase's existing sentinel-error
// idiom (e.g. crowdsec_whitelist_service.go's ErrDuplicateEntry).
var (
	// ErrWebPushInvalidRequest wraps a caller-input validation failure
	// (missing/malformed field) — maps to 400.
	ErrWebPushInvalidRequest = errors.New("invalid web push request")

	// ErrWebPushAlreadyProvisioned is returned both when the cheap
	// service-layer COUNT fast-path finds an existing webpush provider row
	// and when CreateProvider's INSERT loses the race at the
	// idx_webpush_singleton partial unique index (§3.1/§3.3.4) — the
	// caller cannot distinguish the two, and does not need to. Maps to 409.
	ErrWebPushAlreadyProvisioned = errors.New("a Web Push provider is already configured")

	// ErrWebPushNotProvisioned means no webpush NotificationProvider row
	// exists yet (or, for GetWebPushVAPIDPublicKey, none is Enabled).
	// Maps to 404. Lowercase per Go error-string convention (ST1005); the
	// exact user-facing 503 text docs/plans/current_spec.md §3.4.3 pins is
	// produced by the handler, not by relying on this error's text.
	ErrWebPushNotProvisioned = errors.New("web push is not configured")

	// ErrWebPushProviderDisabled means a webpush provider row exists but
	// Enabled is false — subscribing while disabled would create
	// dead-on-arrival subscriptions. Maps to 503.
	ErrWebPushProviderDisabled = errors.New("web push provider is disabled")

	// ErrWebPushSubscriptionNotFound covers both "no such row" and "row
	// exists but isn't owned by the caller" — collapsed into one 404 so a
	// foreign subscription ID doesn't confirm its own existence, matching
	// this codebase's existing respondSanitizedProviderError convention.
	ErrWebPushSubscriptionNotFound = errors.New("web push subscription not found")
)

// ProvisionWebPush generates a new app-wide VAPID identity and creates the
// singleton Type="webpush" NotificationProvider row (docs/plans/current_spec.md
// §3.2/§3.4.1). name and vapidSubject come directly from the admin's
// request; vapidSubject must be "mailto:" or "https://" prefixed, matching
// webpush.Client.Send's own runtime check (validating it here too avoids a
// provision-succeeds-but-every-send-fails footgun).
//
// Race-safety: the cheap COUNT-based check below is a fast-path only, not
// the enforcement mechanism — see CreateProvider and §3.1 "Enforcement" for
// why the idx_webpush_singleton partial unique index (§3.3.4) is what
// actually guarantees the singleton invariant under concurrent requests,
// and how a losing INSERT's constraint-violation error is mapped to the
// same ErrWebPushAlreadyProvisioned this fast-path returns.
func (s *NotificationService) ProvisionWebPush(name, vapidSubject string) (*models.NotificationProvider, error) {
	name = strings.TrimSpace(name)
	vapidSubject = strings.TrimSpace(vapidSubject)

	if name == "" {
		return nil, fmt.Errorf("%w: name is required", ErrWebPushInvalidRequest)
	}
	if !strings.HasPrefix(vapidSubject, "mailto:") && !strings.HasPrefix(vapidSubject, "https://") {
		return nil, fmt.Errorf(`%w: vapid_subject must start with "mailto:" or "https://"`, ErrWebPushInvalidRequest)
	}

	var count int64
	if err := withTransientSQLiteLockRetry(func() error {
		return s.DB.Model(&models.NotificationProvider{}).Where("type = ?", "webpush").Count(&count).Error
	}); err != nil {
		return nil, fmt.Errorf("check existing web push provider: %w", err)
	}
	if count > 0 {
		return nil, ErrWebPushAlreadyProvisioned
	}

	pub, priv, err := webpush.GenerateVAPIDKeyPair()
	if err != nil {
		return nil, fmt.Errorf("generate VAPID keypair: %w", err)
	}

	serviceConfigJSON, err := json.Marshal(webpushServiceConfig{VAPIDPublicKey: pub, VAPIDSubject: vapidSubject})
	if err != nil {
		return nil, fmt.Errorf("encode web push service config: %w", err)
	}

	provider := &models.NotificationProvider{
		Name:                name,
		Type:                "webpush",
		Token:               priv,
		ServiceConfig:       string(serviceConfigJSON),
		Template:            "minimal",
		Enabled:             true,
		NotifyProxyHosts:    true,
		NotifyRemoteServers: true,
		NotifyDomains:       true,
		NotifyCerts:         true,
		NotifyUptime:        true,
	}

	if err := withTransientSQLiteLockRetry(func() error { return s.CreateProvider(provider) }); err != nil {
		if errors.Is(err, gorm.ErrDuplicatedKey) || strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return nil, ErrWebPushAlreadyProvisioned
		}
		return nil, fmt.Errorf("create web push provider: %w", err)
	}

	return provider, nil
}

// withTransientSQLiteLockRetry retries fn a bounded number of times when it
// fails with a transient SQLite lock error. In shared-cache SQLite (used by
// the in-process test DB and single-file deployments), two connections
// racing to INSERT into the same table can surface "database table is
// locked" (SQLITE_LOCKED) rather than the "database is locked"
// (SQLITE_BUSY) error the driver's busy_timeout already retries — so this
// covers the gap for the two concurrent-provision race in
// TestWebPushHandler_Provision_ConcurrentRequestsNeverReturn500. Mirrors the
// retry pattern already used in security_service.go's persistAuditWithRetry
// and credential_service.go's Delete.
func withTransientSQLiteLockRetry(fn func() error) error {
	const maxAttempts = 10
	var err error
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		err = fn()
		if err == nil {
			return nil
		}
		errMsg := strings.ToLower(err.Error())
		isTransientLock := strings.Contains(errMsg, "database is locked") || strings.Contains(errMsg, "database table is locked") || strings.Contains(errMsg, "busy")
		if !isTransientLock || attempt == maxAttempts {
			return err
		}
		time.Sleep(time.Duration(attempt) * 5 * time.Millisecond)
	}
	return err
}

// getWebPushProvider loads the singleton Type="webpush" provider row
// regardless of its Enabled state — callers that need to distinguish
// "not provisioned" from "provisioned but disabled" (§3.4.3) check Enabled
// themselves; GetWebPushVAPIDPublicKey instead filters Enabled in its own
// query since both cases collapse to the same 404 there (§3.4.2).
func (s *NotificationService) getWebPushProvider() (*models.NotificationProvider, error) {
	var provider models.NotificationProvider
	if err := s.DB.Where("type = ?", "webpush").First(&provider).Error; err != nil {
		return nil, err
	}
	return &provider, nil
}

// GetWebPushVAPIDPublicKey returns the current VAPID public key for
// PushManager.subscribe (§3.4.2). Returns ErrWebPushNotProvisioned if no
// webpush provider row exists yet, or if it exists but Enabled is false
// (subscribing while disabled would create dead-on-arrival subscriptions)
// — the caller cannot distinguish the two cases and does not need to.
func (s *NotificationService) GetWebPushVAPIDPublicKey() (string, error) {
	var provider models.NotificationProvider
	if err := s.DB.Where("type = ? AND enabled = ?", "webpush", true).First(&provider).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", ErrWebPushNotProvisioned
		}
		return "", fmt.Errorf("load web push provider: %w", err)
	}

	var cfg webpushServiceConfig
	if err := json.Unmarshal([]byte(provider.ServiceConfig), &cfg); err != nil {
		return "", fmt.Errorf("parse web push service config: %w", err)
	}
	if strings.TrimSpace(cfg.VAPIDPublicKey) == "" {
		return "", ErrWebPushNotProvisioned
	}
	return cfg.VAPIDPublicKey, nil
}

// WebPushSubscribeInput is the validated shape of a browser's
// PushSubscription.toJSON() payload (§3.4.3), decoupled from the HTTP
// request/JSON binding struct so the service layer has no gin dependency.
type WebPushSubscribeInput struct {
	Endpoint  string
	P256dh    string
	Auth      string
	UserAgent string
}

// RegisterWebPushSubscription creates (or idempotently re-registers) one
// browser's WebPushSubscription row under the current singleton webpush
// provider (§3.4.3). A second registration of the same Endpoint refreshes
// LastSeenAt/UserAgent/owner and resets FailureCount rather than creating a
// duplicate row — the same browser subscribing twice must not receive the
// same push twice.
func (s *NotificationService) RegisterWebPushSubscription(userID string, input WebPushSubscribeInput) (sub *models.WebPushSubscription, created bool, err error) {
	userID = strings.TrimSpace(userID)
	endpoint := strings.TrimSpace(input.Endpoint)
	p256dh := strings.TrimSpace(input.P256dh)
	authSecret := strings.TrimSpace(input.Auth)

	if userID == "" {
		return nil, false, fmt.Errorf("%w: user is required", ErrWebPushInvalidRequest)
	}
	if endpoint == "" {
		return nil, false, fmt.Errorf("%w: endpoint is required", ErrWebPushInvalidRequest)
	}
	parsedEndpoint, parseErr := neturl.Parse(endpoint)
	if parseErr != nil || !strings.EqualFold(parsedEndpoint.Scheme, "https") || parsedEndpoint.Host == "" {
		return nil, false, fmt.Errorf("%w: endpoint must be a valid https:// URL", ErrWebPushInvalidRequest)
	}
	if p256dh == "" || authSecret == "" {
		return nil, false, fmt.Errorf("%w: keys.p256dh and keys.auth are required", ErrWebPushInvalidRequest)
	}

	provider, err := s.getWebPushProvider()
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, false, ErrWebPushNotProvisioned
		}
		return nil, false, fmt.Errorf("load web push provider: %w", err)
	}
	if !provider.Enabled {
		return nil, false, ErrWebPushProviderDisabled
	}

	now := time.Now()

	var existing models.WebPushSubscription
	lookupErr := s.DB.Where("endpoint = ?", endpoint).First(&existing).Error
	switch {
	case lookupErr == nil:
		existing.UserID = userID
		existing.ProviderID = provider.ID
		existing.P256dh = p256dh
		existing.Auth = authSecret
		existing.UserAgent = input.UserAgent
		existing.LastSeenAt = now
		existing.FailureCount = 0
		existing.LastFailureAt = nil
		if err := s.DB.Save(&existing).Error; err != nil {
			return nil, false, fmt.Errorf("update web push subscription: %w", err)
		}
		return &existing, false, nil
	case errors.Is(lookupErr, gorm.ErrRecordNotFound):
		newSub := &models.WebPushSubscription{
			ProviderID: provider.ID,
			UserID:     userID,
			Endpoint:   endpoint,
			P256dh:     p256dh,
			Auth:       authSecret,
			UserAgent:  input.UserAgent,
			LastSeenAt: now,
		}
		if err := s.DB.Create(newSub).Error; err != nil {
			return nil, false, fmt.Errorf("create web push subscription: %w", err)
		}
		return newSub, true, nil
	default:
		return nil, false, fmt.Errorf("look up web push subscription: %w", lookupErr)
	}
}

// ListWebPushSubscriptionsForUser returns only the caller's own
// subscriptions (§3.4.4) — never another user's rows, reinforcing per-user
// device management without a cross-user admin view in this PR.
func (s *NotificationService) ListWebPushSubscriptionsForUser(userID string) ([]models.WebPushSubscription, error) {
	var subs []models.WebPushSubscription
	if err := s.DB.Where("user_id = ?", strings.TrimSpace(userID)).Order("created_at desc").Find(&subs).Error; err != nil {
		return nil, fmt.Errorf("list web push subscriptions: %w", err)
	}
	return subs, nil
}

// DeleteWebPushSubscription removes a subscription owned by userID.
// Scoping the DELETE to both id and user_id in one query (rather than a
// separate ownership-check read) means a foreign ID naturally produces the
// same "0 rows affected" outcome as a nonexistent ID — collapsing "not
// found" and "not yours" into the same ErrWebPushSubscriptionNotFound /
// 404, consistent with this codebase's existing
// respondSanitizedProviderError convention of not leaking cross-tenant
// existence (§3.4.5).
func (s *NotificationService) DeleteWebPushSubscription(userID, id string) error {
	result := s.DB.Where("id = ? AND user_id = ?", strings.TrimSpace(id), strings.TrimSpace(userID)).
		Delete(&models.WebPushSubscription{})
	if result.Error != nil {
		return fmt.Errorf("delete web push subscription: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return ErrWebPushSubscriptionNotFound
	}
	return nil
}
