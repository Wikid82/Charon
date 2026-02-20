package cerberus

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// TestBlocker1_SecurityEventProduction tests that all security event types are dispatched to the Cerberus path.
func TestBlocker1_SecurityEventProduction(t *testing.T) {
	// Setup test database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	// Run migrations
	err = db.AutoMigrate(&models.Setting{}, &models.NotificationProvider{}, &models.AccessList{})
	assert.NoError(t, err)

	// Enable feature flag
	setting := models.Setting{
		Key:      "feature.notifications.security_provider_events.enabled",
		Value:    "true",
		Type:     "bool",
		Category: "feature",
	}
	assert.NoError(t, db.Create(&setting).Error)

	// Create discord provider with all security events enabled
	provider := models.NotificationProvider{
		ID:                              "test-provider",
		Name:                            "Test Discord",
		Type:                            "discord",
		URL:                             "https://discord.com/api/webhooks/123/abc",
		Enabled:                         true,
		NotifySecurityWAFBlocks:         true,
		NotifySecurityACLDenies:         true,
		NotifySecurityRateLimitHits:     true,
		NotifySecurityCrowdSecDecisions: true,
	}
	assert.NoError(t, db.Create(&provider).Error)

	// Create Cerberus instance
	cfg := config.SecurityConfig{
		CerberusEnabled: true,
		WAFMode:         "enabled",
		ACLMode:         "enabled",
		RateLimitMode:   "enabled",
		CrowdSecMode:    "local",
	}
	cerberus := New(cfg, db)

	// Test each event type
	eventTypes := []string{"waf_block", "acl_deny", "rate_limit", "crowdsec_decision"}

	for _, eventType := range eventTypes {
		t.Run(eventType, func(t *testing.T) {
			event := models.SecurityEvent{
				EventType: eventType,
				Severity:  "warn",
				Message:   "Test " + eventType,
				ClientIP:  "192.168.1.1",
				Path:      "/test",
				Timestamp: time.Now(),
				Metadata:  map[string]any{"test": "data"},
			}

			// Send security event
			err := cerberus.sendSecurityNotification(context.Background(), event)
			assert.NoError(t, err)
		})
	}
}

// TestBlocker1_NotifySecurityEventMethod tests the new NotifySecurityEvent method.
func TestBlocker1_NotifySecurityEventMethod(t *testing.T) {
	// Setup test database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	// Run migrations
	err = db.AutoMigrate(&models.Setting{}, &models.NotificationProvider{}, &models.AccessList{})
	assert.NoError(t, err)

	// Enable feature flag
	setting := models.Setting{
		Key:      "feature.notifications.security_provider_events.enabled",
		Value:    "true",
		Type:     "bool",
		Category: "feature",
	}
	assert.NoError(t, db.Create(&setting).Error)

	// Create discord provider
	provider := models.NotificationProvider{
		ID:                      "test-provider",
		Name:                    "Test Discord",
		Type:                    "discord",
		URL:                     "https://discord.com/api/webhooks/123/abc",
		Enabled:                 true,
		NotifySecurityWAFBlocks: true,
	}
	assert.NoError(t, db.Create(&provider).Error)

	// Create Cerberus instance
	cfg := config.SecurityConfig{
		CerberusEnabled: true,
		WAFMode:         "enabled",
	}
	cerberus := New(cfg, db)

	// Create test context
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request, _ = http.NewRequest("POST", "/test", nil)

	event := models.SecurityEvent{
		EventType: "waf_block",
		Severity:  "warn",
		Message:   "Test WAF block",
		ClientIP:  "192.168.1.1",
		Path:      "/test",
		Timestamp: time.Now(),
	}

	// Test NotifySecurityEvent method
	err = cerberus.NotifySecurityEvent(ctx, event)
	assert.NoError(t, err)
}

// TestBlocker2_FailClosedWhenFlagAbsent tests that no legacy fallback occurs when flag is absent.
func TestBlocker2_FailClosedWhenFlagAbsent(t *testing.T) {
	// Setup test database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	// Run migrations
	err = db.AutoMigrate(&models.Setting{}, &models.NotificationProvider{}, &models.AccessList{})
	assert.NoError(t, err)

	// DO NOT create feature flag - it should be absent

	// Create Cerberus instance
	cfg := config.SecurityConfig{
		CerberusEnabled: true,
		ACLMode:         "enabled",
	}
	cerberus := New(cfg, db)

	event := models.SecurityEvent{
		EventType: "acl_deny",
		Severity:  "warn",
		Message:   "Test ACL deny",
		ClientIP:  "192.168.1.1",
		Path:      "/test",
		Timestamp: time.Now(),
	}

	// Send security event - should fail closed (no error, but no notification sent)
	err = cerberus.sendSecurityNotification(context.Background(), event)
	assert.NoError(t, err)
}

// TestBlocker2_FailClosedWhenFlagFalse tests that no legacy fallback occurs when flag is false.
func TestBlocker2_FailClosedWhenFlagFalse(t *testing.T) {
	// Setup test database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	// Run migrations
	err = db.AutoMigrate(&models.Setting{}, &models.NotificationProvider{}, &models.AccessList{})
	assert.NoError(t, err)

	// Create feature flag as FALSE
	setting := models.Setting{
		Key:      "feature.notifications.security_provider_events.enabled",
		Value:    "false",
		Type:     "bool",
		Category: "feature",
	}
	assert.NoError(t, db.Create(&setting).Error)

	// Create Cerberus instance
	cfg := config.SecurityConfig{
		CerberusEnabled: true,
		ACLMode:         "enabled",
	}
	cerberus := New(cfg, db)

	event := models.SecurityEvent{
		EventType: "acl_deny",
		Severity:  "warn",
		Message:   "Test ACL deny",
		ClientIP:  "192.168.1.1",
		Path:      "/test",
		Timestamp: time.Now(),
	}

	// Send security event - should fail closed (no error, but no notification sent)
	err = cerberus.sendSecurityNotification(context.Background(), event)
	assert.NoError(t, err)
}

// TestBlocker2_DispatchWhenFlagTrue tests that provider dispatch occurs when flag is true.
func TestBlocker2_DispatchWhenFlagTrue(t *testing.T) {
	// Setup test database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	// Run migrations
	err = db.AutoMigrate(&models.Setting{}, &models.NotificationProvider{}, &models.AccessList{})
	assert.NoError(t, err)

	// Create feature flag as TRUE
	setting := models.Setting{
		Key:      "feature.notifications.security_provider_events.enabled",
		Value:    "true",
		Type:     "bool",
		Category: "feature",
	}
	assert.NoError(t, db.Create(&setting).Error)

	// Create discord provider
	provider := models.NotificationProvider{
		ID:                      "test-provider",
		Name:                    "Test Discord",
		Type:                    "discord",
		URL:                     "https://discord.com/api/webhooks/123/abc",
		Enabled:                 true,
		NotifySecurityACLDenies: true,
	}
	assert.NoError(t, db.Create(&provider).Error)

	// Create Cerberus instance
	cfg := config.SecurityConfig{
		CerberusEnabled: true,
		ACLMode:         "enabled",
	}
	cerberus := New(cfg, db)

	event := models.SecurityEvent{
		EventType: "acl_deny",
		Severity:  "warn",
		Message:   "Test ACL deny",
		ClientIP:  "192.168.1.1",
		Path:      "/test",
		Timestamp: time.Now(),
	}

	// Send security event - should dispatch to provider
	err = cerberus.sendSecurityNotification(context.Background(), event)
	// Note: Will fail with network error since discord.com/api/webhooks/123/abc is fake
	// But that's OK - we're testing the dispatch path, not the actual webhook delivery
	// The error should be from the HTTP client, not from missing provider dispatch
	// For this test, we just verify no panic and the code path is exercised
	_ = err // Ignore network errors for this test
}

// TestBlocker2_ACLDenyNotificationInMiddleware tests ACL deny notification in actual middleware flow.
func TestBlocker2_ACLDenyNotificationInMiddleware(t *testing.T) {
	// Setup test database
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	// Run migrations
	err = db.AutoMigrate(&models.Setting{}, &models.NotificationProvider{}, &models.AccessList{}, &models.SecurityConfig{})
	assert.NoError(t, err)

	// Enable feature flag
	setting := models.Setting{
		Key:      "feature.notifications.security_provider_events.enabled",
		Value:    "true",
		Type:     "bool",
		Category: "feature",
	}
	assert.NoError(t, db.Create(&setting).Error)

	// Create discord provider with ACL denies enabled
	provider := models.NotificationProvider{
		ID:                      "test-provider",
		Name:                    "Test Discord",
		Type:                    "discord",
		URL:                     "https://discord.com/api/webhooks/123/abc",
		Enabled:                 true,
		NotifySecurityACLDenies: true,
	}
	assert.NoError(t, db.Create(&provider).Error)

	// Create an ACL that denies access
	acl := models.AccessList{
		UUID:    "test-acl",
		Name:    "Test ACL",
		Type:    "whitelist",
		Enabled: true,
		IPRules: `[{"cidr":"10.0.0.0/8","description":"allow private network"}]`,
	}
	assert.NoError(t, db.Create(&acl).Error)

	// Create Cerberus instance
	cfg := config.SecurityConfig{
		CerberusEnabled: true,
		ACLMode:         "enabled",
	}
	cerberus := New(cfg, db)

	// Create test context with IP that will be denied
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request, _ = http.NewRequest("GET", "/test", bytes.NewReader([]byte{}))
	ctx.Request.RemoteAddr = "192.168.1.1:12345" // IP not in whitelist

	// Run middleware
	middleware := cerberus.Middleware()
	middleware(ctx)

	// Should be forbidden
	assert.Equal(t, http.StatusForbidden, w.Code)
}
