package services

import (
	"strings"
	"testing"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

func setupSecurityTestDB(t *testing.T) *gorm.DB {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	err = db.AutoMigrate(&models.SecurityConfig{}, &models.SecurityDecision{}, &models.SecurityAudit{}, &models.SecurityRuleSet{})
	assert.NoError(t, err)

	return db
}

func TestSecurityService_Upsert_ValidateAdminWhitelist(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := NewSecurityService(db)

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
	svc := NewSecurityService(db)

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
	svc := NewSecurityService(db)

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
	svc := NewSecurityService(db)

	rs := &models.SecurityRuleSet{Name: "owasp-crs", SourceURL: "https://example.com/owasp.rules", Mode: "owasp", Content: "rule: 1"}
	err := svc.UpsertRuleSet(rs)
	assert.NoError(t, err)

	list, err := svc.ListRuleSets()
	assert.NoError(t, err)
	assert.GreaterOrEqual(t, len(list), 1)
	assert.Equal(t, "owasp-crs", list[0].Name)
}

func TestSecurityService_UpsertRuleSet_ContentTooLarge(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := NewSecurityService(db)

	// Create a string slightly larger than 2MB
	large := strings.Repeat("x", 2*1024*1024+1)
	rs := &models.SecurityRuleSet{Name: "big-crs", Content: large}
	err := svc.UpsertRuleSet(rs)
	assert.Error(t, err)
}

func TestSecurityService_DeleteRuleSet(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := NewSecurityService(db)

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
	svc := NewSecurityService(db)

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
	svc := NewSecurityService(db)

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
	svc := NewSecurityService(db)

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
	svc := NewSecurityService(db)

	// Verify against non-existent config
	ok, err := svc.VerifyBreakGlassToken("nonexistent", "anytoken")
	assert.Error(t, err)
	assert.Equal(t, ErrSecurityConfigNotFound, err)
	assert.False(t, ok)
}

func TestSecurityService_VerifyBreakGlassToken_NoHash(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := NewSecurityService(db)

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
	svc := NewSecurityService(db)

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
	svc := NewSecurityService(db)

	// Get from empty database
	cfg, err := svc.Get()
	assert.Error(t, err)
	assert.Equal(t, ErrSecurityConfigNotFound, err)
	assert.Nil(t, cfg)
}

func TestSecurityService_Upsert_PreserveBreakGlassHash(t *testing.T) {
	db := setupSecurityTestDB(t)
	svc := NewSecurityService(db)

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
