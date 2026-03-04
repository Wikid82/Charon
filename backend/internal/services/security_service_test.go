package services

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupSecurityTestDB(t *testing.T) *gorm.DB {
	dsn := filepath.Join(t.TempDir(), "security_service_test.db") + "?_busy_timeout=5000&_journal_mode=WAL"
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	assert.NoError(t, err)

	sqlDB, err := db.DB()
	assert.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	sqlDB.SetMaxIdleConns(1)

	err = db.AutoMigrate(&models.SecurityConfig{}, &models.SecurityDecision{}, &models.SecurityAudit{}, &models.SecurityRuleSet{})
	assert.NoError(t, err)

	// Close database connection when test completes
	t.Cleanup(func() {
		if sqlDB != nil {
			_ = sqlDB.Close()
		}
	})

	return db
}

// newTestSecurityService creates a SecurityService for tests with proper cleanup
func newTestSecurityService(t *testing.T, db *gorm.DB) *SecurityService {
	svc := NewSecurityService(db)

	// Stop the background goroutine when test completes
	t.Cleanup(func() {
		svc.Close()
	})

	return svc
}

func TestSecurityService_Upsert_ValidateAdminWhitelist(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := newTestSecurityService(t, db)

	// Invalid CIDR in admin whitelist should fail
	cfg := &models.SecurityConfig{Name: "default", Enabled: true, AdminWhitelist: "invalid-cidr"}
	err := svc.Upsert(cfg)
	assert.Error(t, err)
	assert.Equal(t, ErrInvalidAdminCIDR, err)

	// Valid CIDR should succeed
	cfg.AdminWhitelist = "192.168.1.0/24, 10.0.0.1"
	err = svc.Upsert(cfg)
	assert.NoError(t, err)

	// Verify stored
	got, err := svc.Get()
	assert.NoError(t, err)
	assert.True(t, strings.Contains(got.AdminWhitelist, "192.168.1.0/24"))
}

func TestSecurityService_BreakGlassTokenLifecycle(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := newTestSecurityService(t, db)

	// Create record
	cfg := &models.SecurityConfig{Name: "default", Enabled: false}
	err := svc.Upsert(cfg)
	assert.NoError(t, err)

	token, err := svc.GenerateBreakGlassToken("default")
	assert.NoError(t, err)
	assert.NotEmpty(t, token)

	// Verify valid token returns true
	ok, err := svc.VerifyBreakGlassToken("default", token)
	assert.NoError(t, err)
	assert.True(t, ok)

	// Invalid token fails
	ok, err = svc.VerifyBreakGlassToken("default", "wrongtoken")
	assert.Error(t, err)
	assert.False(t, ok)
}

func TestSecurityService_LogDecisionAndList(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := newTestSecurityService(t, db)

	dec := &models.SecurityDecision{Source: "manual", Action: "block", IP: "1.2.3.4", Host: "example.com", RuleID: "manual-1", Details: "test manual block"}
	err := svc.LogDecision(dec)
	assert.NoError(t, err)

	list, err := svc.ListDecisions(10)
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(list), 1)
	assert.Equal(t, "manual", list[0].Source)
}

func TestSecurityService_UpsertRuleSet(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := newTestSecurityService(t, db)

	// Test creating new ruleset
	rs := &models.SecurityRuleSet{Name: "owasp-crs", SourceURL: "https://example.com/owasp.rules", Mode: "owasp", Content: "rule: 1"}
	err := svc.UpsertRuleSet(rs)
	assert.NoError(t, err)
	assert.NotEmpty(t, rs.UUID)
	assert.False(t, rs.LastUpdated.IsZero())

	// Test updating existing ruleset
	rs.Content = "rule: 2"
	rs.Mode = "updated"
	err = svc.UpsertRuleSet(rs)
	assert.NoError(t, err)

	list, err := svc.ListRuleSets()
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(list), 1)
	assert.Equal(t, "owasp-crs", list[0].Name)
	assert.Equal(t, "rule: 2", list[0].Content)
	assert.Equal(t, "updated", list[0].Mode)

	// Test nil ruleset
	err = svc.UpsertRuleSet(nil)
	assert.NoError(t, err)

	// Test ruleset without name
	invalidRuleset := &models.SecurityRuleSet{Content: "test"}
	err = svc.UpsertRuleSet(invalidRuleset)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "name required")
}

func TestSecurityService_UpsertRuleSet_ContentTooLarge(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := newTestSecurityService(t, db)

	// Create a string slightly larger than 2MB
	large := strings.Repeat("x", 2*1024*1024+1)
	rs := &models.SecurityRuleSet{Name: "big-crs", Content: large}
	err := svc.UpsertRuleSet(rs)
	assert.Error(t, err)
}

func TestSecurityService_DeleteRuleSet(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := newTestSecurityService(t, db)

	rs := &models.SecurityRuleSet{Name: "owasp-crs", Content: "rule: 1"}
	err := svc.UpsertRuleSet(rs)
	assert.NoError(t, err)

	// Get list and pick ID
	list, err := svc.ListRuleSets()
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(list), 1)

	id := list[0].ID
	// Delete
	err = svc.DeleteRuleSet(id)
	assert.NoError(t, err)
	// Ensure no rulesets left
	list, err = svc.ListRuleSets()
	assert.NoError(t, err)
	assert.Equal(t, 0, len(list))
}

func TestSecurityService_Upsert_RejectExternalMode(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := newTestSecurityService(t, db)

	// External mode should be rejected by validation
	cfg := &models.SecurityConfig{Name: "default", Enabled: true, CrowdSecMode: "external"}
	err := svc.Upsert(cfg)
	assert.Error(t, err)

	// Unknown mode should also be rejected
	cfg.CrowdSecMode = "unknown"
	err = svc.Upsert(cfg)
	assert.Error(t, err)

	// Local mode should be accepted
	cfg.CrowdSecMode = "local"
	err = svc.Upsert(cfg)
	assert.NoError(t, err)
}

func TestSecurityService_GenerateBreakGlassToken_NewConfig(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := newTestSecurityService(t, db)

	// Generate token for non-existent config (should create it)
	token, err := svc.GenerateBreakGlassToken("newconfig")
	assert.NoError(t, err)
	assert.NotEmpty(t, token)
	assert.Greater(t, len(token), 20) // Should be hex-encoded 24 bytes = 48 chars

	// Verify the token works
	ok, err := svc.VerifyBreakGlassToken("newconfig", token)
	assert.NoError(t, err)
	assert.True(t, ok)

	// Verify config was created with hash
	var cfg models.SecurityConfig
	err = db.Where("name = ?", "newconfig").First(&cfg).Error
	assert.NoError(t, err)
	assert.NotEmpty(t, cfg.BreakGlassHash)
}

func TestSecurityService_GenerateBreakGlassToken_UpdateExisting(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := newTestSecurityService(t, db)

	// Create initial config
	cfg := &models.SecurityConfig{Name: "default", Enabled: true}
	err := svc.Upsert(cfg)
	assert.NoError(t, err)

	// Generate first token
	token1, err := svc.GenerateBreakGlassToken("default")
	assert.NoError(t, err)

	// Generate second token (should replace first)
	token2, err := svc.GenerateBreakGlassToken("default")
	assert.NoError(t, err)
	assert.NotEqual(t, token1, token2)

	// First token should no longer work
	ok, err := svc.VerifyBreakGlassToken("default", token1)
	assert.Error(t, err)
	assert.False(t, ok)

	// Second token should work
	ok, err = svc.VerifyBreakGlassToken("default", token2)
	assert.NoError(t, err)
	assert.True(t, ok)
}

func TestSecurityService_VerifyBreakGlassToken_NoConfig(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := newTestSecurityService(t, db)

	// Verify against non-existent config
	ok, err := svc.VerifyBreakGlassToken("nonexistent", "anytoken")
	assert.Error(t, err)
	assert.Equal(t, ErrSecurityConfigNotFound, err)
	assert.False(t, ok)
}

func TestSecurityService_VerifyBreakGlassToken_NoHash(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := newTestSecurityService(t, db)

	// Create config without break-glass hash
	cfg := &models.SecurityConfig{Name: "default", Enabled: true, BreakGlassHash: ""}
	err := svc.Upsert(cfg)
	assert.NoError(t, err)

	// Verify should fail with no hash
	ok, err := svc.VerifyBreakGlassToken("default", "anytoken")
	assert.Error(t, err)
	assert.Equal(t, ErrBreakGlassInvalid, err)
	assert.False(t, ok)
}

func TestSecurityService_VerifyBreakGlassToken_WrongToken(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := newTestSecurityService(t, db)

	// Generate valid token
	token, err := svc.GenerateBreakGlassToken("default")
	assert.NoError(t, err)

	// Try various wrong tokens
	testCases := []string{
		"",
		"wrongtoken",
		"x" + token,
		token[:len(token)-1],
		strings.ToUpper(token),
	}

	for _, wrongToken := range testCases {
		ok, err := svc.VerifyBreakGlassToken("default", wrongToken)
		assert.Error(t, err, "Token should fail: %s", wrongToken)
		assert.Equal(t, ErrBreakGlassInvalid, err)
		assert.False(t, ok)
	}
}

func TestSecurityService_Get_NotFound(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := newTestSecurityService(t, db)

	// Get from empty database
	cfg, err := svc.Get()
	assert.Error(t, err)
	assert.Equal(t, ErrSecurityConfigNotFound, err)
	assert.Nil(t, cfg)
}

func TestSecurityService_Upsert_PreserveBreakGlassHash(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := newTestSecurityService(t, db)

	// Generate token
	token, err := svc.GenerateBreakGlassToken("default")
	assert.NoError(t, err)

	// Get the hash
	var cfg models.SecurityConfig
	err = db.Where("name = ?", "default").First(&cfg).Error
	assert.NoError(t, err)
	originalHash := cfg.BreakGlassHash

	// Update other fields
	cfg.Enabled = true
	cfg.AdminWhitelist = "10.0.0.0/8"
	err = svc.Upsert(&cfg)
	assert.NoError(t, err)

	// Verify hash is preserved
	var updated models.SecurityConfig
	err = db.Where("name = ?", "default").First(&updated).Error
	assert.NoError(t, err)
	assert.Equal(t, originalHash, updated.BreakGlassHash)

	// Original token should still work
	ok, err := svc.VerifyBreakGlassToken("default", token)
	assert.NoError(t, err)
	assert.True(t, ok)
}

func TestSecurityService_Get_PrefersDefaultConfig(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := newTestSecurityService(t, db)
	defer svc.Close()

	// Create a non-default config first to simulate environments with multiple rows.
	// Must provide unique UUIDs since the model has uniqueIndex on UUID field.
	other := &models.SecurityConfig{UUID: "test-other-uuid", Name: "other", Enabled: true}
	assert.NoError(t, db.Create(other).Error)

	def := &models.SecurityConfig{UUID: "test-default-uuid", Name: "default", Enabled: false}
	assert.NoError(t, db.Create(def).Error)

	cfg, err := svc.Get()
	assert.NoError(t, err)
	assert.NotNil(t, cfg)
	assert.Equal(t, "default", cfg.Name)
	assert.False(t, cfg.Enabled)
}

func TestSecurityService_Upsert_RateLimitFieldsPersist(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := newTestSecurityService(t, db)

	// 1. Create initial config with rate limit settings
	initialCfg := &models.SecurityConfig{
		Name:               "default",
		Enabled:            true,
		RateLimitEnable:    true,
		RateLimitBurst:     10,
		RateLimitRequests:  100,
		RateLimitWindowSec: 60,
		WAFLearning:        false,
		CrowdSecAPIURL:     "http://localhost:8080",
		WAFRulesSource:     "owasp-crs",
	}
	err := svc.Upsert(initialCfg)
	assert.NoError(t, err)

	// Verify initial values
	got, err := svc.Get()
	assert.NoError(t, err)
	assert.Equal(t, 100, got.RateLimitRequests)
	assert.Equal(t, 60, got.RateLimitWindowSec)
	assert.Equal(t, 10, got.RateLimitBurst)
	assert.False(t, got.WAFLearning)
	assert.Equal(t, "http://localhost:8080", got.CrowdSecAPIURL)
	assert.Equal(t, "owasp-crs", got.WAFRulesSource)

	// 2. Update rate limit settings via Upsert
	updatedCfg := &models.SecurityConfig{
		Name:               "default",
		Enabled:            true,
		RateLimitEnable:    true,
		RateLimitBurst:     50,
		RateLimitRequests:  500,
		RateLimitWindowSec: 120,
		WAFLearning:        true,
		CrowdSecAPIURL:     "http://crowdsec:8080",
		WAFRulesSource:     "custom-rules",
	}
	err = svc.Upsert(updatedCfg)
	assert.NoError(t, err)

	// 3. Verify all fields persisted correctly via Get()
	got, err = svc.Get()
	assert.NoError(t, err)
	assert.Equal(t, 500, got.RateLimitRequests, "RateLimitRequests should be updated")
	assert.Equal(t, 120, got.RateLimitWindowSec, "RateLimitWindowSec should be updated")
	assert.Equal(t, 50, got.RateLimitBurst, "RateLimitBurst should be updated")
	assert.True(t, got.WAFLearning, "WAFLearning should be updated")
	assert.Equal(t, "http://crowdsec:8080", got.CrowdSecAPIURL, "CrowdSecAPIURL should be updated")
	assert.Equal(t, "custom-rules", got.WAFRulesSource, "WAFRulesSource should be updated")
}

func TestSecurityService_LogAudit(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := newTestSecurityService(t, db)

	// Test logging valid audit entry
	audit := &models.SecurityAudit{
		Action:  "login_success",
		Actor:   "admin",
		Details: "User admin logged in from 192.168.1.100",
	}
	err := svc.LogAudit(audit)
	assert.NoError(t, err)
	assert.NotEmpty(t, audit.UUID)
	assert.False(t, audit.CreatedAt.IsZero())

	// Give time for async audit logging to process
	time.Sleep(150 * time.Millisecond)

	// Verify audit was stored
	var stored models.SecurityAudit
	err = db.Where("uuid = ?", audit.UUID).First(&stored).Error
	assert.NoError(t, err)
	assert.Equal(t, "login_success", stored.Action)
	assert.Equal(t, "admin", stored.Actor)

	// Test logging nil audit (should not error)
	err = svc.LogAudit(nil)
	assert.NoError(t, err)

	// Test audit with pre-filled UUID
	audit2 := &models.SecurityAudit{
		UUID:    "custom-uuid-123",
		Action:  "config_change",
		Actor:   "admin",
		Details: "Security settings updated",
	}
	err = svc.LogAudit(audit2)
	assert.NoError(t, err)
	assert.Equal(t, "custom-uuid-123", audit2.UUID)
}

func TestSecurityService_DeleteRuleSet_NotFound(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := newTestSecurityService(t, db)

	// Try to delete non-existent ruleset
	err := svc.DeleteRuleSet(9999)
	assert.Error(t, err)
}

func TestSecurityService_ListDecisions_UnlimitedAndLimited(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := newTestSecurityService(t, db)

	// Create multiple decisions
	for i := 0; i < 5; i++ {
		dec := &models.SecurityDecision{
			Source:  "test",
			Action:  "block",
			IP:      "1.2.3." + string(rune('0'+i)),
			Host:    "example.com",
			RuleID:  "test-rule",
			Details: "test decision",
		}
		err := svc.LogDecision(dec)
		assert.NoError(t, err)
	}

	// Test unlimited (limit = 0)
	all, err := svc.ListDecisions(0)
	assert.NoError(t, err)
	assert.Len(t, all, 5)

	// Test limited
	limited, err := svc.ListDecisions(2)
	assert.NoError(t, err)
	assert.Len(t, limited, 2)
}

func TestSecurityService_LogDecision_Nil(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := newTestSecurityService(t, db)

	// Nil decision should not error
	err := svc.LogDecision(nil)
	assert.NoError(t, err)
}

func TestSecurityService_LogDecision_PrefilledUUID(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := newTestSecurityService(t, db)

	dec := &models.SecurityDecision{
		UUID:    "custom-decision-uuid",
		Source:  "manual",
		Action:  "allow",
		IP:      "10.0.0.1",
		Host:    "internal.example.com",
		RuleID:  "whitelist-1",
		Details: "whitelisted",
	}
	err := svc.LogDecision(dec)
	assert.NoError(t, err)
	assert.Equal(t, "custom-decision-uuid", dec.UUID)

	// Verify it was stored with custom UUID
	var stored models.SecurityDecision
	err = db.Where("uuid = ?", "custom-decision-uuid").First(&stored).Error
	assert.NoError(t, err)
}

func TestSecurityService_ListRuleSets_Empty(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := newTestSecurityService(t, db)

	// Empty database should return empty slice, not error
	list, err := svc.ListRuleSets()
	assert.NoError(t, err)
	assert.NotNil(t, list)
	assert.Len(t, list, 0)
}

func TestSecurityService_Upsert_InvalidCrowdSecMode(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := newTestSecurityService(t, db)

	// Test various invalid modes
	invalidModes := []string{"", "invalid", "External", "LOCAL", "disabled123"}
	for _, mode := range invalidModes {
		cfg := &models.SecurityConfig{Name: "default", CrowdSecMode: mode}
		err := svc.Upsert(cfg)
		if mode == "" {
			// Empty mode is valid (defaults to disabled)
			continue
		}
		// Non-empty invalid modes should error
		if mode != "local" && mode != "disabled" && mode != "" {
			assert.Error(t, err, "Mode %q should be invalid", mode)
		}
	}
}

func TestSecurityService_ListAuditLogs(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := newTestSecurityService(t, db)

	// Create test audit logs
	testAudits := []models.SecurityAudit{
		{
			UUID:          "audit-1",
			Actor:         "user-1",
			Action:        "dns_provider_create",
			EventCategory: "dns_provider",
			ResourceUUID:  "provider-1",
			Details:       `{"name":"Provider 1"}`,
		},
		{
			UUID:          "audit-2",
			Actor:         "user-2",
			Action:        "dns_provider_update",
			EventCategory: "dns_provider",
			ResourceUUID:  "provider-2",
			Details:       `{"changed_fields":{"name":true}}`,
		},
		{
			UUID:          "audit-3",
			Actor:         "user-1",
			Action:        "dns_provider_delete",
			EventCategory: "dns_provider",
			ResourceUUID:  "provider-3",
			Details:       `{"name":"Provider 3"}`,
		},
	}

	for _, audit := range testAudits {
		err := db.Create(&audit).Error
		assert.NoError(t, err)
	}

	// Test listing all audits
	audits, total, err := svc.ListAuditLogs(AuditLogFilter{}, 1, 10)
	assert.NoError(t, err)
	assert.Equal(t, int64(3), total)
	assert.Len(t, audits, 3)

	// Test filter by actor
	audits, total, err = svc.ListAuditLogs(AuditLogFilter{Actor: "user-1"}, 1, 10)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, audits, 2)

	// Test filter by action
	audits, total, err = svc.ListAuditLogs(AuditLogFilter{Action: "dns_provider_create"}, 1, 10)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, audits, 1)

	// Test filter by event category
	audits, total, err = svc.ListAuditLogs(AuditLogFilter{EventCategory: "dns_provider"}, 1, 10)
	assert.NoError(t, err)
	assert.Equal(t, int64(3), total)
	assert.Len(t, audits, 3)

	// Test pagination
	audits, total, err = svc.ListAuditLogs(AuditLogFilter{}, 1, 2)
	assert.NoError(t, err)
	assert.Equal(t, int64(3), total)
	assert.Len(t, audits, 2)

	// Second page
	audits, total, err = svc.ListAuditLogs(AuditLogFilter{}, 2, 2)
	assert.NoError(t, err)
	assert.Equal(t, int64(3), total)
	assert.Len(t, audits, 1)
}

func TestSecurityService_GetAuditLogByUUID(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := newTestSecurityService(t, db)

	// Create test audit log
	testAudit := models.SecurityAudit{
		UUID:          "audit-test-uuid",
		Actor:         "user-1",
		Action:        "dns_provider_create",
		EventCategory: "dns_provider",
		ResourceUUID:  "provider-1",
		Details:       `{"name":"Test Provider"}`,
	}
	err := db.Create(&testAudit).Error
	assert.NoError(t, err)

	// Test retrieving existing audit log
	audit, err := svc.GetAuditLogByUUID("audit-test-uuid")
	assert.NoError(t, err)
	assert.NotNil(t, audit)
	assert.Equal(t, "audit-test-uuid", audit.UUID)
	assert.Equal(t, "user-1", audit.Actor)

	// Test retrieving non-existent audit log
	audit, err = svc.GetAuditLogByUUID("non-existent-uuid")
	assert.Error(t, err)
	assert.Nil(t, audit)
	assert.Equal(t, "audit log not found", err.Error())
}

func TestSecurityService_ListAuditLogsByProvider(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := newTestSecurityService(t, db)

	providerID := uint(123)
	otherProviderID := uint(456)

	// Create test audit logs
	testAudits := []models.SecurityAudit{
		{
			UUID:          "audit-provider-1",
			Actor:         "user-1",
			Action:        "dns_provider_create",
			EventCategory: "dns_provider",
			ResourceID:    &providerID,
			ResourceUUID:  "provider-uuid-1",
			Details:       `{"name":"Provider 1"}`,
		},
		{
			UUID:          "audit-provider-2",
			Actor:         "user-1",
			Action:        "dns_provider_update",
			EventCategory: "dns_provider",
			ResourceID:    &providerID,
			ResourceUUID:  "provider-uuid-1",
			Details:       `{"changed_fields":{"name":true}}`,
		},
		{
			UUID:          "audit-other-provider",
			Actor:         "user-2",
			Action:        "dns_provider_create",
			EventCategory: "dns_provider",
			ResourceID:    &otherProviderID,
			ResourceUUID:  "provider-uuid-2",
			Details:       `{"name":"Other Provider"}`,
		},
	}

	for _, audit := range testAudits {
		err := db.Create(&audit).Error
		assert.NoError(t, err)
	}

	// Test listing audits for specific provider
	audits, total, err := svc.ListAuditLogsByProvider(providerID, 1, 10)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, audits, 2)

	// Test listing audits for other provider
	audits, total, err = svc.ListAuditLogsByProvider(otherProviderID, 1, 10)
	assert.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Len(t, audits, 1)

	// Test pagination
	audits, total, err = svc.ListAuditLogsByProvider(providerID, 1, 1)
	assert.NoError(t, err)
	assert.Equal(t, int64(2), total)
	assert.Len(t, audits, 1)
}

func TestSecurityService_AsyncAuditLogging(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := newTestSecurityService(t, db)

	// Log audit asynchronously
	audit := &models.SecurityAudit{
		Actor:         "user-1",
		Action:        "test_action",
		EventCategory: "test_category",
		Details:       "test details",
	}
	err := svc.LogAudit(audit)
	assert.NoError(t, err)
	assert.NotEmpty(t, audit.UUID)

	// Give some time for async processing
	time.Sleep(100 * time.Millisecond)

	// Verify audit was stored
	var stored models.SecurityAudit
	err = db.Where("uuid = ?", audit.UUID).First(&stored).Error
	assert.NoError(t, err)
	assert.Equal(t, "test_action", stored.Action)
}

func TestSecurityService_LogAudit_ChannelFullFallsBackToSyncWrite(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := newTestSecurityService(t, db)

	for i := 0; i < cap(svc.auditChan); i++ {
		svc.auditChan <- &models.SecurityAudit{
			UUID:   fmt.Sprintf("prefill-%d", i),
			Actor:  "prefill",
			Action: "prefill_action",
		}
	}

	audit := &models.SecurityAudit{
		Actor:  "sync-fallback",
		Action: "user_create",
	}

	err := svc.LogAudit(audit)
	assert.NoError(t, err)

	assert.Eventually(t, func() bool {
		var stored models.SecurityAudit
		queryErr := db.Where("uuid = ?", audit.UUID).First(&stored).Error
		if queryErr != nil {
			return false
		}
		return stored.Actor == "sync-fallback"
	}, time.Second, 20*time.Millisecond)
}

// TestSecurityService_ListAuditLogs_EdgeCases tests edge cases for audit log listing.
func TestSecurityService_ListAuditLogs_EdgeCases(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := newTestSecurityService(t, db)

	t.Run("list audits with no data returns empty", func(t *testing.T) {
		audits, total, err := svc.ListAuditLogs(AuditLogFilter{}, 1, 10)
		assert.NoError(t, err)
		assert.Equal(t, int64(0), total)
		assert.Empty(t, audits)
	})

	t.Run("list audits with time range filter", func(t *testing.T) {
		// Create audits with specific timestamps
		now := time.Now()
		oldAudit := models.SecurityAudit{
			UUID:      "old-audit",
			Actor:     "user-1",
			Action:    "old_action",
			CreatedAt: now.Add(-48 * time.Hour),
		}
		newAudit := models.SecurityAudit{
			UUID:      "new-audit",
			Actor:     "user-1",
			Action:    "new_action",
			CreatedAt: now.Add(-1 * time.Hour),
		}
		assert.NoError(t, db.Create(&oldAudit).Error)
		assert.NoError(t, db.Create(&newAudit).Error)

		// Filter by time range - last 24 hours
		startDate := now.Add(-24 * time.Hour)
		endDate := now
		filter := AuditLogFilter{
			StartDate: &startDate,
			EndDate:   &endDate,
		}

		audits, total, err := svc.ListAuditLogs(filter, 1, 10)
		assert.NoError(t, err)
		assert.Equal(t, int64(1), total)
		assert.Len(t, audits, 1)
		assert.Equal(t, "new-audit", audits[0].UUID)
	})

	t.Run("list audits with combined filters", func(t *testing.T) {
		// Create diverse audits
		audits := []models.SecurityAudit{
			{UUID: "audit-a", Actor: "user-1", Action: "create", EventCategory: "provider"},
			{UUID: "audit-b", Actor: "user-2", Action: "update", EventCategory: "provider"},
			{UUID: "audit-c", Actor: "user-1", Action: "delete", EventCategory: "host"},
		}
		for _, a := range audits {
			assert.NoError(t, db.Create(&a).Error)
		}

		// Filter by actor AND event category
		filter := AuditLogFilter{
			Actor:         "user-1",
			EventCategory: "provider",
		}

		results, total, err := svc.ListAuditLogs(filter, 1, 10)
		assert.NoError(t, err)
		assert.Equal(t, int64(1), total)
		assert.Len(t, results, 1)
		assert.Equal(t, "audit-a", results[0].UUID)
	})

	t.Run("list audits handles zero page", func(t *testing.T) {
		audit := models.SecurityAudit{UUID: "test", Actor: "user", Action: "test"}
		assert.NoError(t, db.Create(&audit).Error)

		// Page 0 should default to page 1
		audits, total, err := svc.ListAuditLogs(AuditLogFilter{}, 0, 10)
		assert.NoError(t, err)
		assert.Greater(t, total, int64(0))
		assert.NotEmpty(t, audits)
	})

	t.Run("list audits with very large limit", func(t *testing.T) {
		// Should handle large limits gracefully
		audits, total, err := svc.ListAuditLogs(AuditLogFilter{}, 1, 10000)
		assert.NoError(t, err)
		assert.GreaterOrEqual(t, total, int64(0))
		_ = audits
	})
}

// TestSecurityService_ListAuditLogsByProvider_EdgeCases tests edge cases for provider audit logs.
func TestSecurityService_ListAuditLogsByProvider_EdgeCases(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := newTestSecurityService(t, db)
	defer svc.Close()

	t.Run("list audits for non-existent provider returns empty", func(t *testing.T) {
		audits, total, err := svc.ListAuditLogsByProvider(99999, 1, 10)
		assert.NoError(t, err)
		assert.Equal(t, int64(0), total)
		assert.Empty(t, audits)
	})
}

// TestSecurityService_GenerateBreakGlassToken_EdgeCases tests token generation edge cases.
func TestSecurityService_GenerateBreakGlassToken_EdgeCases(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := newTestSecurityService(t, db)
	defer svc.Close()

	t.Run("generated tokens are different on regeneration", func(t *testing.T) {
		token1, err := svc.GenerateBreakGlassToken("test-config")
		assert.NoError(t, err)
		assert.NotEmpty(t, token1)

		// Sleep a moment to ensure different token generated
		time.Sleep(10 * time.Millisecond)

		token2, err := svc.GenerateBreakGlassToken("test-config")
		assert.NoError(t, err)
		assert.NotEmpty(t, token2)

		// The tokens themselves should be different (even though they update the same config)
		assert.NotEqual(t, token1, token2, "Regenerated tokens should be different")
	})
}

// TestSecurityService_Flush_EdgeCases tests flush functionality.
func TestSecurityService_Flush_EdgeCases(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := newTestSecurityService(t, db)

	t.Run("flush with empty channel completes quickly", func(t *testing.T) {
		start := time.Now()
		svc.Flush()
		duration := time.Since(start)

		// Should complete in less than 100ms when channel is empty
		assert.Less(t, duration, 100*time.Millisecond)
	})

	t.Run("flush waits for pending audits", func(t *testing.T) {
		// Log multiple audits
		for i := 0; i < 5; i++ {
			audit := &models.SecurityAudit{
				Actor:  "user-1",
				Action: "test_action",
			}
			_ = svc.LogAudit(audit)
		}

		// Flush should wait for them
		svc.Flush()

		// Verify all were processed
		var count int64
		db.Model(&models.SecurityAudit{}).Count(&count)
		assert.Equal(t, int64(5), count)
	})
}

// TestSecurityService_Get_Singleton tests Get behavior with multiple configs.
func TestSecurityService_Get_Singleton(t *testing.T) {
	t.Run("get returns error when no config exists", func(t *testing.T) {
		db := setupSecurityTestDB(t)
		svc := newTestSecurityService(t, db)
		defer svc.Close()

		_, err := svc.Get()
		assert.Error(t, err)
		assert.Equal(t, ErrSecurityConfigNotFound, err)
	})

	t.Run("get returns first config when no default", func(t *testing.T) {
		db := setupSecurityTestDB(t)
		svc := newTestSecurityService(t, db)
		defer svc.Close()

		// Create only non-default config
		cfg := &models.SecurityConfig{Name: "custom", Enabled: false}
		assert.NoError(t, svc.Upsert(cfg))

		// Should fall back to first config
		got, err := svc.Get()
		assert.NoError(t, err)
		assert.Equal(t, "custom", got.Name)
	})

	t.Run("get returns default config when exists", func(t *testing.T) {
		db := setupSecurityTestDB(t)
		svc := newTestSecurityService(t, db)
		defer svc.Close()

		// Create default config
		defaultCfg := &models.SecurityConfig{Name: "default", Enabled: true}
		assert.NoError(t, svc.Upsert(defaultCfg))

		// Get should return "default" config
		got, err := svc.Get()
		assert.NoError(t, err)
		assert.Equal(t, "default", got.Name)
		assert.True(t, got.Enabled)
	})
}

// TestSecurityService_ListRuleSets_EdgeCases tests rule set listing edge cases.
func TestSecurityService_ListRuleSets_EdgeCases(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := newTestSecurityService(t, db)

	t.Run("list rulesets with no data returns empty", func(t *testing.T) {
		rulesets, err := svc.ListRuleSets()
		assert.NoError(t, err)
		assert.Empty(t, rulesets)
	})

	t.Run("list rulesets handles pagination", func(t *testing.T) {
		// Create multiple rulesets
		for i := 0; i < 5; i++ {
			rs := &models.SecurityRuleSet{
				Name:    fmt.Sprintf("ruleset-%d", i),
				Content: "rule",
			}
			assert.NoError(t, svc.UpsertRuleSet(rs))
		}

		// List should return all
		rulesets, err := svc.ListRuleSets()
		assert.NoError(t, err)
		assert.Len(t, rulesets, 5)
	})
}
