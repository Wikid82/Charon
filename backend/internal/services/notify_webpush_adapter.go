package services

import (
	"context"
	"encoding/json"
	"regexp"
	"strconv"
	"strings"
	"time"

	notify "github.com/Wikid82/go_notify_yourself"
	"github.com/Wikid82/go_notify_yourself/providers/webpush"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/util"
)

// webpushMaxConsecutiveFailures bounds how many consecutive non-404/410
// send failures a WebPushSubscription row tolerates before being pruned as
// presumed-dead (docs/plans/current_spec.md §3.5 step 4 / §3.8). A
// subscription survives up to webpushMaxConsecutiveFailures-1 consecutive
// failures; on reaching this count it is deleted, bounding the table from
// accumulating rows that fail forever for non-404/410 reasons (e.g. a push
// service outage that never resolves before it starts returning 410).
const webpushMaxConsecutiveFailures = 10

// webpushServiceConfig is the JSON shape stored in
// NotificationProvider.ServiceConfig for a Type="webpush" row (§3.2):
// the VAPID public key and subject, which are safe to expose (unlike
// VAPIDPrivateKey, stored unencrypted in NotificationProvider.Token,
// already json:"-").
type webpushServiceConfig struct {
	VAPIDPublicKey string `json:"vapid_public_key"`
	VAPIDSubject   string `json:"vapid_subject"`
}

// providerStatusPattern extracts the numeric HTTP status code out of
// transport.Wrapper.Send's plain formatted error string
// ("provider returned status %d" or "provider returned status %d: %s") —
// see extractHTTPStatusFromNotifyError's doc comment for why this
// regex-based approach is necessary rather than a typed/sentinel error.
var providerStatusPattern = regexp.MustCompile(`provider returned status (\d+)`)

// extractHTTPStatusFromNotifyError parses the numeric HTTP status code out
// of a notify.Sender.Send error's message, if one is present.
//
// This exists because, per docs/plans/current_spec.md §2.1/§7 risk 1,
// go_notify_yourself's transport.Wrapper.Send returns non-2xx failures as a
// plain formatted string (fmt.Errorf("provider returned status %d[: %s]",
// ...)) with no typed/sentinel error anywhere in transport/ or webpush/ —
// there is no errors.Is-compatible way to detect the standard Web Push
// "subscription is gone" 404/410 signal. Parsing the status out of the
// error text is fragile-by-construction (an upstream wording change
// silently breaks detection), which is why this is isolated to one small,
// independently-tested function: if it ever stops matching, the failure
// mode is "subscriptions never auto-prune on 404/410, FailureCount
// accumulates and the row prunes at webpushMaxConsecutiveFailures instead"
// (safe-ish degradation) rather than a crash or silent data loss.
//
// webpush.Client.Send wraps the transport error further
// ("failed to send web push: %w"), so the match is done as a substring
// search against the full error text, not an anchored prefix.
func extractHTTPStatusFromNotifyError(err error) (status int, ok bool) {
	if err == nil {
		return 0, false
	}
	match := providerStatusPattern.FindStringSubmatch(err.Error())
	if len(match) != 2 {
		return 0, false
	}
	parsed, convErr := strconv.Atoi(match[1])
	if convErr != nil {
		return 0, false
	}
	return parsed, true
}

// dispatchWebPushViaNotify fans a single logical notification out to every
// WebPushSubscription row under provider — one webpush.Client per row, all
// sharing provider's VAPID identity (docs/plans/current_spec.md §3.5).
//
// Sends run sequentially within this already-backgrounded goroutine
// (SendExternal already calls this via `go s.dispatchWebPushViaNotify(...)`
// per provider), not one goroutine per subscription — a provider with many
// subscriptions would otherwise spawn unbounded concurrent outbound HTTP
// requests. This is a deliberate, documented trade-off (§3.5/§7 risk 3),
// not an oversight.
//
// This function never returns an error to its caller (matching the
// fire-and-forget style of every existing dispatchXxxViaNotify); it logs
// and records per-subscription outcomes only.
func (s *NotificationService) dispatchWebPushViaNotify(ctx context.Context, provider models.NotificationProvider, eventType, title, message string, data map[string]any) {
	var cfg webpushServiceConfig
	if err := json.Unmarshal([]byte(provider.ServiceConfig), &cfg); err != nil {
		logger.Log().WithError(err).WithField("provider", util.SanitizeForLog(provider.Name)).Error("Failed to parse Web Push provider service config")
		return
	}
	vapidPrivateKey := strings.TrimSpace(provider.Token)
	if strings.TrimSpace(cfg.VAPIDPublicKey) == "" || strings.TrimSpace(cfg.VAPIDSubject) == "" || vapidPrivateKey == "" {
		logger.Log().WithField("provider", util.SanitizeForLog(provider.Name)).Error("Web Push provider is missing VAPID identity fields")
		return
	}

	var subscriptions []models.WebPushSubscription
	if err := s.DB.Where("provider_id = ?", provider.ID).Find(&subscriptions).Error; err != nil {
		logger.Log().WithError(err).WithField("provider", util.SanitizeForLog(provider.Name)).Error("Failed to load Web Push subscriptions")
		return
	}
	if len(subscriptions) == 0 {
		return
	}

	tmpl, customTemplate := resolveTemplateFields(provider)
	msg := notify.Message{
		Title:     title,
		Body:      message,
		EventType: eventType,
		Data:      notifyMessageDataFromLegacyFlatMap(data),
	}

	for _, sub := range subscriptions {
		client := webpush.New(webpush.Config{
			VAPIDPublicKey:  cfg.VAPIDPublicKey,
			VAPIDPrivateKey: vapidPrivateKey,
			VAPIDSubject:    cfg.VAPIDSubject,
			Endpoint:        sub.Endpoint,
			P256dh:          sub.P256dh,
			Auth:            sub.Auth,
			Template:        tmpl,
			CustomTemplate:  customTemplate,
		}, s.notifyWrapper)

		sendErr := client.Send(ctx, msg)
		s.recordWebPushSendResult(sub, sendErr)
	}
}

// recordWebPushSendResult applies the partial-failure handling policy for
// one subscription's send outcome (docs/plans/current_spec.md §3.5 step 4):
//   - success: refresh LastSeenAt, reset FailureCount.
//   - 404/410 (subscription is gone, per the standard Web Push convention):
//     delete the row immediately.
//   - any other failure: increment FailureCount/LastFailureAt; once
//     FailureCount reaches webpushMaxConsecutiveFailures, delete the row as
//     presumed-dead.
//
// Every failure is logged via the existing logger.Log().WithError(...)
// convention — never silent, matching dispatchViaNotify's existing style.
func (s *NotificationService) recordWebPushSendResult(sub models.WebPushSubscription, sendErr error) {
	now := time.Now()

	if sendErr == nil {
		if err := s.DB.Model(&models.WebPushSubscription{}).Where("id = ?", sub.ID).Updates(map[string]any{
			"last_seen_at":  now,
			"failure_count": 0,
		}).Error; err != nil {
			logger.Log().WithError(err).WithField("subscription_id", sub.ID).Error("Failed to update Web Push subscription after successful send")
		}
		return
	}

	if status, ok := extractHTTPStatusFromNotifyError(sendErr); ok && (status == 404 || status == 410) {
		logger.Log().WithError(sendErr).WithField("subscription_id", sub.ID).WithField("status", status).
			Info("Pruning Web Push subscription reported gone by the push service (404/410)")
		if err := s.DB.Delete(&models.WebPushSubscription{}, "id = ?", sub.ID).Error; err != nil {
			logger.Log().WithError(err).WithField("subscription_id", sub.ID).Error("Failed to prune dead Web Push subscription")
		}
		return
	}

	logger.Log().WithError(sendErr).WithField("subscription_id", sub.ID).Error("Failed to send Web Push notification")

	newFailureCount := sub.FailureCount + 1
	if newFailureCount >= webpushMaxConsecutiveFailures {
		logger.Log().WithField("subscription_id", sub.ID).WithField("failure_count", newFailureCount).
			Warn("Pruning Web Push subscription after reaching max consecutive failures")
		if err := s.DB.Delete(&models.WebPushSubscription{}, "id = ?", sub.ID).Error; err != nil {
			logger.Log().WithError(err).WithField("subscription_id", sub.ID).Error("Failed to prune presumed-dead Web Push subscription")
		}
		return
	}

	if err := s.DB.Model(&models.WebPushSubscription{}).Where("id = ?", sub.ID).Updates(map[string]any{
		"failure_count":   newFailureCount,
		"last_failure_at": now,
	}).Error; err != nil {
		logger.Log().WithError(err).WithField("subscription_id", sub.ID).Error("Failed to update Web Push subscription failure count")
	}
}
