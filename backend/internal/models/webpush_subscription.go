package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// WebPushSubscription is one browser/device's Web Push destination,
// created when an authenticated Charon user's browser completes
// PushManager.subscribe() and POSTs the resulting PushSubscription to the
// backend. Each row is fanned out to individually by
// NotificationService.dispatchWebPushViaNotify (one webpush.Client per
// row), all sharing the parent NotificationProvider's VAPID identity.
type WebPushSubscription struct {
	ID         string `gorm:"primaryKey" json:"id"`
	ProviderID string `gorm:"index;not null" json:"provider_id"` // FK -> NotificationProvider.ID (Type="webpush")
	UserID     string `gorm:"index;not null" json:"user_id"`     // FK -> User.ID; owner, for scoped unsubscribe

	// PushSubscription destination (from the browser's PushSubscription
	// object; see webpush.Config's matching field doc comments).
	Endpoint string `gorm:"uniqueIndex;type:text;not null" json:"endpoint"`
	P256dh   string `gorm:"type:text;not null" json:"-"` // subscriber DH public key; not attacker-sensitive but never needed client-side after registration
	Auth     string `gorm:"type:text;not null" json:"-"` // subscriber auth secret; same rationale

	// Display/diagnostic metadata, not used for dispatch.
	UserAgent string `json:"user_agent,omitempty" gorm:"type:text"`

	// Pruning bookkeeping (dispatch fan-out — notify_webpush_adapter.go).
	LastSeenAt    time.Time  `json:"last_seen_at"`
	LastFailureAt *time.Time `json:"last_failure_at,omitempty"`
	FailureCount  int        `json:"failure_count" gorm:"default:0"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (s *WebPushSubscription) BeforeCreate(tx *gorm.DB) (err error) {
	if s.ID == "" {
		s.ID = uuid.New().String()
	}
	return
}
