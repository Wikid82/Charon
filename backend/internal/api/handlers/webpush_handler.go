package handlers

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Wikid82/charon/backend/internal/services"
)

// WebPushHandler exposes the Web Push provisioning and subscription
// lifecycle endpoints (docs/plans/current_spec.md §3.4). All routes are
// mounted on the existing authenticated `management` route group; the
// Provision route is additionally gated admin-only at the route
// registration layer (routes.go), mirroring the Test/Preview precedent —
// see §3.4.0 for the full authorization-model writeup.
type WebPushHandler struct {
	service *services.NotificationService
}

func NewWebPushHandler(service *services.NotificationService) *WebPushHandler {
	return &WebPushHandler{service: service}
}

type webPushProvisionRequest struct {
	Name         string `json:"name"`
	VAPIDSubject string `json:"vapid_subject"`
}

type webPushSubscribeRequest struct {
	Endpoint string `json:"endpoint"`
	Keys     struct {
		P256dh string `json:"p256dh"`
		Auth   string `json:"auth"`
	} `json:"keys"`
	UserAgent string `json:"user_agent"`
}

type webPushSubscriptionResponse struct {
	ID         string    `json:"id"`
	Endpoint   string    `json:"endpoint"`
	UserAgent  string    `json:"user_agent,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
	LastSeenAt time.Time `json:"last_seen_at"`
}

// webPushUserID extracts the authenticated caller's user ID (set by
// AuthMiddleware as a uint, per models.User.ID) and renders it as the
// string WebPushSubscription.UserID/models column expects. On failure it
// writes a 401 itself, matching requireUserID's contract.
func webPushUserID(c *gin.Context) (string, bool) {
	userID, ok := requireUserID(c)
	if !ok {
		return "", false
	}
	return strconv.FormatUint(uint64(userID), 10), true
}

// Provision generates a new VAPID identity and creates the singleton
// webpush NotificationProvider row (§3.4.1). Admin-only — enforced by
// middleware.RequireRole(models.RoleAdmin) at the route registration layer
// (routes.go), matching the Test/Preview admin-only precedent on this same
// route group; no additional in-handler check is needed.
func (h *WebPushHandler) Provision(c *gin.Context) {
	var req webPushProvisionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request payload"})
		return
	}

	provider, err := h.service.ProvisionWebPush(req.Name, req.VAPIDSubject)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrWebPushAlreadyProvisioned):
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		case errors.Is(err, services.ErrWebPushInvalidRequest):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to provision web push provider"})
		}
		return
	}

	provider.HasToken = provider.Token != ""
	provider.Token = ""
	c.JSON(http.StatusCreated, provider)
}

// VAPIDPublicKey serves the current VAPID public key for
// PushManager.subscribe (§3.4.2). Available to any authenticated
// management-access user, not just admins — a non-admin user's browser can
// still subscribe to receive alerts.
func (h *WebPushHandler) VAPIDPublicKey(c *gin.Context) {
	key, err := h.service.GetWebPushVAPIDPublicKey()
	if err != nil {
		if errors.Is(err, services.ErrWebPushNotProvisioned) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load web push VAPID public key"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"vapid_public_key": key})
}

// Subscribe registers (or idempotently re-registers) the caller's browser
// PushSubscription (§3.4.3).
func (h *WebPushHandler) Subscribe(c *gin.Context) {
	userID, ok := webPushUserID(c)
	if !ok {
		return
	}

	var req webPushSubscribeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request payload"})
		return
	}

	sub, created, err := h.service.RegisterWebPushSubscription(userID, services.WebPushSubscribeInput{
		Endpoint:  req.Endpoint,
		P256dh:    req.Keys.P256dh,
		Auth:      req.Keys.Auth,
		UserAgent: req.UserAgent,
	})
	if err != nil {
		switch {
		case errors.Is(err, services.ErrWebPushNotProvisioned):
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case errors.Is(err, services.ErrWebPushProviderDisabled):
			// Exact text pinned by docs/plans/current_spec.md §3.4.3, kept
			// independent of the sentinel error's own (lowercase, Go-idiom)
			// message.
			c.JSON(http.StatusServiceUnavailable, gin.H{"error": "Web Push provider is disabled"})
		case errors.Is(err, services.ErrWebPushInvalidRequest):
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to register web push subscription"})
		}
		return
	}

	status := http.StatusOK
	if created {
		status = http.StatusCreated
	}
	c.JSON(status, gin.H{"id": sub.ID, "endpoint": sub.Endpoint})
}

// ListSubscriptions returns the caller's own subscriptions only (§3.4.4).
func (h *WebPushHandler) ListSubscriptions(c *gin.Context) {
	userID, ok := webPushUserID(c)
	if !ok {
		return
	}

	subs, err := h.service.ListWebPushSubscriptionsForUser(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to list web push subscriptions"})
		return
	}

	resp := make([]webPushSubscriptionResponse, 0, len(subs))
	for _, sub := range subs {
		resp = append(resp, webPushSubscriptionResponse{
			ID:         sub.ID,
			Endpoint:   sub.Endpoint,
			UserAgent:  sub.UserAgent,
			CreatedAt:  sub.CreatedAt,
			LastSeenAt: sub.LastSeenAt,
		})
	}
	c.JSON(http.StatusOK, resp)
}

// Unsubscribe removes a subscription owned by the caller (§3.4.5). Returns
// 404 — never 403 — for a subscription that doesn't exist or isn't owned by
// the caller, to avoid confirming existence of another user's row.
func (h *WebPushHandler) Unsubscribe(c *gin.Context) {
	userID, ok := webPushUserID(c)
	if !ok {
		return
	}

	id := c.Param("id")
	if err := h.service.DeleteWebPushSubscription(userID, id); err != nil {
		if errors.Is(err, services.ErrWebPushSubscriptionNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to delete web push subscription"})
		return
	}
	c.Status(http.StatusNoContent)
}
