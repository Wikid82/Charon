package handlers_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"sync"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/api/handlers"
	"github.com/Wikid82/charon/backend/internal/api/middleware"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
)

// setupWebPushHandlerTest wires WebPushHandler's routes (and, for the
// admin-gate regression test, the existing generic provider Update route)
// under the same middleware.RequireManagementAccess()/RequireRole(admin)
// gates routes.go registers them with, so these tests exercise the real
// authorization wiring rather than a re-implemented stand-in. Role/user ID
// per request are driven by the X-Test-Role/X-Test-UserID headers so a
// single router serves both admin and non-admin test cases.
func setupWebPushHandlerTest(t *testing.T) (*gin.Engine, *gorm.DB) {
	t.Helper()
	db := handlers.OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.NotificationProvider{}, &models.WebPushSubscription{}, &models.Notification{}))
	// Mirrors routes.go's post-AutoMigrate singleton-index step exactly
	// (docs/plans/current_spec.md §3.3.4).
	require.NoError(t, db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_webpush_singleton
		ON notification_providers(type) WHERE type = 'webpush'`).Error)

	service := services.NewNotificationService(db, nil)
	webPushHandler := handlers.NewWebPushHandler(service)
	providerHandler := handlers.NewNotificationProviderHandler(service)

	r := gin.New()
	r.Use(func(c *gin.Context) {
		role := c.GetHeader("X-Test-Role")
		if role == "" {
			role = "admin"
		}
		uid := uint64(1)
		if raw := c.GetHeader("X-Test-UserID"); raw != "" {
			uid, _ = strconv.ParseUint(raw, 10, 64)
		}
		c.Set("role", role)
		c.Set("userID", uint(uid))
		c.Next()
	})

	api := r.Group("/api/v1")
	management := api.Group("/")
	management.Use(middleware.RequireManagementAccess())
	management.POST("/notifications/providers/webpush/provision", middleware.RequireRole(models.RoleAdmin), webPushHandler.Provision)
	management.GET("/notifications/providers/webpush/vapid-public-key", webPushHandler.VAPIDPublicKey)
	management.POST("/notifications/providers/webpush/subscriptions", webPushHandler.Subscribe)
	management.GET("/notifications/providers/webpush/subscriptions", webPushHandler.ListSubscriptions)
	management.DELETE("/notifications/providers/webpush/subscriptions/:id", webPushHandler.Unsubscribe)
	management.PUT("/notifications/providers/:id", providerHandler.Update)

	return r, db
}

func doJSONRequest(t *testing.T, r *gin.Engine, method, path string, body any, headers map[string]string) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		require.NoError(t, json.NewEncoder(&buf).Encode(body))
	}
	req, err := http.NewRequest(method, path, &buf)
	require.NoError(t, err)
	req.Header.Set("Content-Type", "application/json")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// --- Provision ---

func TestWebPushHandler_Provision_Success(t *testing.T) {
	r, _ := setupWebPushHandlerTest(t)

	w := doJSONRequest(t, r, http.MethodPost, "/api/v1/notifications/providers/webpush/provision",
		map[string]string{"name": "Browser Push", "vapid_subject": "mailto:ops@example.com"}, nil)

	require.Equal(t, http.StatusCreated, w.Code, w.Body.String())

	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.Equal(t, "webpush", resp["type"])
	assert.NotContains(t, w.Body.String(), `"token"`, "VAPID private key must never be serialized")
}

func TestWebPushHandler_Provision_InvalidVAPIDSubjectReturns400(t *testing.T) {
	r, _ := setupWebPushHandlerTest(t)

	w := doJSONRequest(t, r, http.MethodPost, "/api/v1/notifications/providers/webpush/provision",
		map[string]string{"name": "Browser Push", "vapid_subject": "not-a-valid-subject"}, nil)

	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestWebPushHandler_Provision_SecondAttemptReturns409(t *testing.T) {
	r, _ := setupWebPushHandlerTest(t)

	first := doJSONRequest(t, r, http.MethodPost, "/api/v1/notifications/providers/webpush/provision",
		map[string]string{"name": "Browser Push", "vapid_subject": "mailto:ops@example.com"}, nil)
	require.Equal(t, http.StatusCreated, first.Code)

	second := doJSONRequest(t, r, http.MethodPost, "/api/v1/notifications/providers/webpush/provision",
		map[string]string{"name": "Browser Push 2", "vapid_subject": "mailto:ops2@example.com"}, nil)
	assert.Equal(t, http.StatusConflict, second.Code)
}

func TestWebPushHandler_Provision_NonAdminReturns403(t *testing.T) {
	r, _ := setupWebPushHandlerTest(t)

	w := doJSONRequest(t, r, http.MethodPost, "/api/v1/notifications/providers/webpush/provision",
		map[string]string{"name": "Browser Push", "vapid_subject": "mailto:ops@example.com"},
		map[string]string{"X-Test-Role": "user"})

	assert.Equal(t, http.StatusForbidden, w.Code)
}

// TestWebPushHandler_Provision_ConcurrentRequestsNeverReturn500 is the
// required regression test for docs/plans/current_spec.md §3.1/§3.3.4/§9
// commit 6: two Provision requests racing the idx_webpush_singleton
// partial unique index must produce exactly one success and the loser must
// see 409 — never a raw 500 — proving CreateProvider's constraint-violation
// error is caught and mapped, not left to surface as an unhandled error.
func TestWebPushHandler_Provision_ConcurrentRequestsNeverReturn500(t *testing.T) {
	r, db := setupWebPushHandlerTest(t)

	const attempts = 2
	codes := make([]int, attempts)
	var wg sync.WaitGroup
	wg.Add(attempts)
	for i := 0; i < attempts; i++ {
		go func(idx int) {
			defer wg.Done()
			w := doJSONRequest(t, r, http.MethodPost, "/api/v1/notifications/providers/webpush/provision",
				map[string]string{"name": "Browser Push", "vapid_subject": "mailto:ops@example.com"}, nil)
			codes[idx] = w.Code
		}(i)
	}
	wg.Wait()

	successCount, conflictCount, otherCount := 0, 0, 0
	for _, code := range codes {
		switch code {
		case http.StatusCreated:
			successCount++
		case http.StatusConflict:
			conflictCount++
		default:
			otherCount++
		}
	}

	assert.Equal(t, 1, successCount, "exactly one concurrent provision request should succeed")
	assert.Equal(t, 1, conflictCount, "the losing request should get 409, not any other status")
	assert.Equal(t, 0, otherCount, "no request should ever surface a raw 500 or other status")

	var providerCount int64
	require.NoError(t, db.Model(&models.NotificationProvider{}).Where("type = ?", "webpush").Count(&providerCount).Error)
	assert.Equal(t, int64(1), providerCount, "only one webpush provider row should exist after the race")
}

// --- VAPIDPublicKey ---

func TestWebPushHandler_VAPIDPublicKey_NotProvisionedReturns404(t *testing.T) {
	r, _ := setupWebPushHandlerTest(t)

	w := doJSONRequest(t, r, http.MethodGet, "/api/v1/notifications/providers/webpush/vapid-public-key", nil, nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestWebPushHandler_VAPIDPublicKey_ReturnsKeyAfterProvisioning(t *testing.T) {
	r, _ := setupWebPushHandlerTest(t)

	provisionResp := doJSONRequest(t, r, http.MethodPost, "/api/v1/notifications/providers/webpush/provision",
		map[string]string{"name": "Browser Push", "vapid_subject": "mailto:ops@example.com"}, nil)
	require.Equal(t, http.StatusCreated, provisionResp.Code)

	w := doJSONRequest(t, r, http.MethodGet, "/api/v1/notifications/providers/webpush/vapid-public-key", nil, nil)
	require.Equal(t, http.StatusOK, w.Code)

	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	assert.NotEmpty(t, resp["vapid_public_key"])
}

func TestWebPushHandler_VAPIDPublicKey_DisabledProviderReturns404(t *testing.T) {
	r, db := setupWebPushHandlerTest(t)

	provisionResp := doJSONRequest(t, r, http.MethodPost, "/api/v1/notifications/providers/webpush/provision",
		map[string]string{"name": "Browser Push", "vapid_subject": "mailto:ops@example.com"}, nil)
	require.Equal(t, http.StatusCreated, provisionResp.Code)

	require.NoError(t, db.Model(&models.NotificationProvider{}).Where("type = ?", "webpush").Update("enabled", false).Error)

	w := doJSONRequest(t, r, http.MethodGet, "/api/v1/notifications/providers/webpush/vapid-public-key", nil, nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

// --- Subscribe / ListSubscriptions / Unsubscribe ---

func provisionWebPush(t *testing.T, r *gin.Engine) {
	t.Helper()
	w := doJSONRequest(t, r, http.MethodPost, "/api/v1/notifications/providers/webpush/provision",
		map[string]string{"name": "Browser Push", "vapid_subject": "mailto:ops@example.com"}, nil)
	require.Equal(t, http.StatusCreated, w.Code)
}

func subscribeBody(endpoint string) map[string]any {
	return map[string]any{
		"endpoint": endpoint,
		"keys": map[string]string{
			"p256dh": "p256dh-value",
			"auth":   "auth-value",
		},
		"user_agent": "Mozilla/5.0 test-agent",
	}
}

func TestWebPushHandler_Subscribe_NotProvisionedReturns404(t *testing.T) {
	r, _ := setupWebPushHandlerTest(t)

	w := doJSONRequest(t, r, http.MethodPost, "/api/v1/notifications/providers/webpush/subscriptions",
		subscribeBody("https://push.example.net/a"), nil)
	assert.Equal(t, http.StatusNotFound, w.Code)
}

func TestWebPushHandler_Subscribe_DisabledProviderReturns503(t *testing.T) {
	r, db := setupWebPushHandlerTest(t)
	provisionWebPush(t, r)
	require.NoError(t, db.Model(&models.NotificationProvider{}).Where("type = ?", "webpush").Update("enabled", false).Error)

	w := doJSONRequest(t, r, http.MethodPost, "/api/v1/notifications/providers/webpush/subscriptions",
		subscribeBody("https://push.example.net/a"), nil)
	assert.Equal(t, http.StatusServiceUnavailable, w.Code)
}

func TestWebPushHandler_Subscribe_InvalidEndpointReturns400(t *testing.T) {
	r, _ := setupWebPushHandlerTest(t)
	provisionWebPush(t, r)

	w := doJSONRequest(t, r, http.MethodPost, "/api/v1/notifications/providers/webpush/subscriptions",
		subscribeBody("http://not-https.example.net/a"), nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestWebPushHandler_Subscribe_CreatesThenIdempotentlyReRegisters(t *testing.T) {
	r, _ := setupWebPushHandlerTest(t)
	provisionWebPush(t, r)

	first := doJSONRequest(t, r, http.MethodPost, "/api/v1/notifications/providers/webpush/subscriptions",
		subscribeBody("https://push.example.net/dup"), nil)
	require.Equal(t, http.StatusCreated, first.Code)

	second := doJSONRequest(t, r, http.MethodPost, "/api/v1/notifications/providers/webpush/subscriptions",
		subscribeBody("https://push.example.net/dup"), nil)
	assert.Equal(t, http.StatusOK, second.Code, "re-registering the same endpoint is idempotent, not a new row")

	var firstResp, secondResp map[string]string
	require.NoError(t, json.Unmarshal(first.Body.Bytes(), &firstResp))
	require.NoError(t, json.Unmarshal(second.Body.Bytes(), &secondResp))
	assert.Equal(t, firstResp["id"], secondResp["id"])
}

func TestWebPushHandler_ListSubscriptions_ScopedToCaller(t *testing.T) {
	r, _ := setupWebPushHandlerTest(t)
	provisionWebPush(t, r)

	subResp := doJSONRequest(t, r, http.MethodPost, "/api/v1/notifications/providers/webpush/subscriptions",
		subscribeBody("https://push.example.net/mine"), map[string]string{"X-Test-UserID": "1"})
	require.Equal(t, http.StatusCreated, subResp.Code)

	otherResp := doJSONRequest(t, r, http.MethodPost, "/api/v1/notifications/providers/webpush/subscriptions",
		subscribeBody("https://push.example.net/other"), map[string]string{"X-Test-UserID": "2"})
	require.Equal(t, http.StatusCreated, otherResp.Code)

	listResp := doJSONRequest(t, r, http.MethodGet, "/api/v1/notifications/providers/webpush/subscriptions", nil, map[string]string{"X-Test-UserID": "1"})
	require.Equal(t, http.StatusOK, listResp.Code)

	var list []map[string]any
	require.NoError(t, json.Unmarshal(listResp.Body.Bytes(), &list))
	require.Len(t, list, 1)
	assert.Equal(t, "https://push.example.net/mine", list[0]["endpoint"])
}

func TestWebPushHandler_Unsubscribe_OwnSubscriptionSucceeds(t *testing.T) {
	r, _ := setupWebPushHandlerTest(t)
	provisionWebPush(t, r)

	subResp := doJSONRequest(t, r, http.MethodPost, "/api/v1/notifications/providers/webpush/subscriptions",
		subscribeBody("https://push.example.net/mine"), map[string]string{"X-Test-UserID": "1"})
	require.Equal(t, http.StatusCreated, subResp.Code)
	var sub map[string]string
	require.NoError(t, json.Unmarshal(subResp.Body.Bytes(), &sub))

	delResp := doJSONRequest(t, r, http.MethodDelete, "/api/v1/notifications/providers/webpush/subscriptions/"+sub["id"], nil, map[string]string{"X-Test-UserID": "1"})
	assert.Equal(t, http.StatusNoContent, delResp.Code)
}

func TestWebPushHandler_Unsubscribe_ForeignSubscriptionReturns404(t *testing.T) {
	r, _ := setupWebPushHandlerTest(t)
	provisionWebPush(t, r)

	subResp := doJSONRequest(t, r, http.MethodPost, "/api/v1/notifications/providers/webpush/subscriptions",
		subscribeBody("https://push.example.net/mine"), map[string]string{"X-Test-UserID": "1"})
	require.Equal(t, http.StatusCreated, subResp.Code)
	var sub map[string]string
	require.NoError(t, json.Unmarshal(subResp.Body.Bytes(), &sub))

	// A different caller (userID 2) must not be able to delete userID 1's
	// subscription — and must get 404, not 403, to avoid confirming
	// existence of another user's row (§3.4.5).
	delResp := doJSONRequest(t, r, http.MethodDelete, "/api/v1/notifications/providers/webpush/subscriptions/"+sub["id"], nil, map[string]string{"X-Test-UserID": "2"})
	assert.Equal(t, http.StatusNotFound, delResp.Code)
}

func TestWebPushHandler_Unsubscribe_NonexistentReturns404(t *testing.T) {
	r, _ := setupWebPushHandlerTest(t)

	delResp := doJSONRequest(t, r, http.MethodDelete, "/api/v1/notifications/providers/webpush/subscriptions/does-not-exist", nil, nil)
	assert.Equal(t, http.StatusNotFound, delResp.Code)
}

// TestNotificationProviderUpdate_RoleUserForbiddenFromSecurityToggleOnWebPushProvider
// is the required regression test from docs/plans/current_spec.md §3.4.0 /
// §9 commit 6: it pins the CURRENT behavior that the generic
// PUT /notifications/providers/:id endpoint's unconditional requireAdmin(c)
// gate (notification_provider_handler.go's Update, first line) already
// blocks a RoleUser caller from setting any NotifySecurityXxx field on a
// webpush provider row before the request body is even inspected — closing
// the exfiltration scenario Supervisor flagged (a self-service-subscribed
// low-privileged device flipping on security-event forwarding to itself)
// without any new webpush-specific authorization code. This test exists so
// a future refactor of Update's auth check cannot silently reopen that gap.
func TestNotificationProviderUpdate_RoleUserForbiddenFromSecurityToggleOnWebPushProvider(t *testing.T) {
	r, db := setupWebPushHandlerTest(t)
	provisionWebPush(t, r)

	var provider models.NotificationProvider
	require.NoError(t, db.Where("type = ?", "webpush").First(&provider).Error)

	body := map[string]any{
		"name":                       provider.Name,
		"type":                       "webpush",
		"enabled":                    true,
		"notify_security_waf_blocks": true,
		"notify_security_acl_denies": true,
	}
	w := doJSONRequest(t, r, http.MethodPut, "/api/v1/notifications/providers/"+provider.ID, body,
		map[string]string{"X-Test-Role": "user"})

	assert.Equal(t, http.StatusForbidden, w.Code)

	// Belt-and-suspenders: confirm the toggle was in fact NOT persisted.
	var reloaded models.NotificationProvider
	require.NoError(t, db.Where("id = ?", provider.ID).First(&reloaded).Error)
	assert.False(t, reloaded.NotifySecurityWAFBlocks)
	assert.False(t, reloaded.NotifySecurityACLDenies)
}

// --- Error-path coverage: malformed JSON, unauthenticated, and internal errors ---

func closeUnderlyingDB(t *testing.T, db *gorm.DB) {
	t.Helper()
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())
}

func TestWebPushHandler_Provision_MalformedJSONReturns400(t *testing.T) {
	r, _ := setupWebPushHandlerTest(t)

	w := doJSONRequest(t, r, http.MethodPost, "/api/v1/notifications/providers/webpush/provision", "not-an-object", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestWebPushHandler_Provision_UnexpectedServiceErrorReturns500(t *testing.T) {
	r, db := setupWebPushHandlerTest(t)
	closeUnderlyingDB(t, db)

	w := doJSONRequest(t, r, http.MethodPost, "/api/v1/notifications/providers/webpush/provision",
		map[string]string{"name": "Browser Push", "vapid_subject": "mailto:ops@example.com"}, nil)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestWebPushHandler_VAPIDPublicKey_UnexpectedServiceErrorReturns500(t *testing.T) {
	r, db := setupWebPushHandlerTest(t)
	closeUnderlyingDB(t, db)

	w := doJSONRequest(t, r, http.MethodGet, "/api/v1/notifications/providers/webpush/vapid-public-key", nil, nil)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestWebPushHandler_Subscribe_MalformedJSONReturns400(t *testing.T) {
	r, _ := setupWebPushHandlerTest(t)
	provisionWebPush(t, r)

	w := doJSONRequest(t, r, http.MethodPost, "/api/v1/notifications/providers/webpush/subscriptions", "not-an-object", nil)
	assert.Equal(t, http.StatusBadRequest, w.Code)
}

func TestWebPushHandler_Subscribe_UnexpectedServiceErrorReturns500(t *testing.T) {
	r, db := setupWebPushHandlerTest(t)
	provisionWebPush(t, r)
	closeUnderlyingDB(t, db)

	w := doJSONRequest(t, r, http.MethodPost, "/api/v1/notifications/providers/webpush/subscriptions",
		subscribeBody("https://push.example.net/a"), nil)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestWebPushHandler_ListSubscriptions_UnexpectedServiceErrorReturns500(t *testing.T) {
	r, db := setupWebPushHandlerTest(t)
	closeUnderlyingDB(t, db)

	w := doJSONRequest(t, r, http.MethodGet, "/api/v1/notifications/providers/webpush/subscriptions", nil, nil)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

func TestWebPushHandler_Unsubscribe_UnexpectedServiceErrorReturns500(t *testing.T) {
	r, db := setupWebPushHandlerTest(t)
	closeUnderlyingDB(t, db)

	w := doJSONRequest(t, r, http.MethodDelete, "/api/v1/notifications/providers/webpush/subscriptions/some-id", nil, nil)
	assert.Equal(t, http.StatusInternalServerError, w.Code)
}

// unauthenticatedWebPushRouter mounts the caller-scoped Web Push routes with
// no middleware setting "userID" in the gin context, so webPushUserID's
// requireUserID call hits its not-authenticated branch — exercising
// Subscribe/ListSubscriptions/Unsubscribe's own `!ok` early-return path
// (each a distinct statement from webPushUserID's own), matching how a
// request would look if AuthMiddleware were ever bypassed or misconfigured.
func unauthenticatedWebPushRouter(t *testing.T) *gin.Engine {
	t.Helper()
	db := handlers.OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.NotificationProvider{}, &models.WebPushSubscription{}, &models.Notification{}))
	service := services.NewNotificationService(db, nil)
	webPushHandler := handlers.NewWebPushHandler(service)

	r := gin.New()
	api := r.Group("/api/v1")
	api.POST("/notifications/providers/webpush/subscriptions", webPushHandler.Subscribe)
	api.GET("/notifications/providers/webpush/subscriptions", webPushHandler.ListSubscriptions)
	api.DELETE("/notifications/providers/webpush/subscriptions/:id", webPushHandler.Unsubscribe)
	return r
}

func TestWebPushHandler_Subscribe_UnauthenticatedReturns401(t *testing.T) {
	r := unauthenticatedWebPushRouter(t)
	w := doJSONRequest(t, r, http.MethodPost, "/api/v1/notifications/providers/webpush/subscriptions",
		subscribeBody("https://push.example.net/a"), nil)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestWebPushHandler_ListSubscriptions_UnauthenticatedReturns401(t *testing.T) {
	r := unauthenticatedWebPushRouter(t)
	w := doJSONRequest(t, r, http.MethodGet, "/api/v1/notifications/providers/webpush/subscriptions", nil, nil)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestWebPushHandler_Unsubscribe_UnauthenticatedReturns401(t *testing.T) {
	r := unauthenticatedWebPushRouter(t)
	w := doJSONRequest(t, r, http.MethodDelete, "/api/v1/notifications/providers/webpush/subscriptions/some-id", nil, nil)
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}
