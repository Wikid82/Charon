package services

import (
    "testing"
    "strings"

    "github.com/Wikid82/charon/backend/internal/models"
    "github.com/stretchr/testify/assert"
    "gorm.io/driver/sqlite"
    "gorm.io/gorm"
)

func setupSecurityTestDB(t *testing.T) *gorm.DB {
    db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
    assert.NoError(t, err)

    err = db.AutoMigrate(&models.SecurityConfig{})
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
