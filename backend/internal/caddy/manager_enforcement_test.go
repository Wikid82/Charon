package caddy

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
)

// switchableCaddyClient records loads and fails them while loadErr is set.
type switchableCaddyClient struct {
	loadErr error
	loads   int
}

func (c *switchableCaddyClient) Load(_ context.Context, _ *Config) error {
	c.loads++
	return c.loadErr
}

func (c *switchableCaddyClient) Ping(context.Context) error { return nil }

func (c *switchableCaddyClient) GetConfig(context.Context) (*Config, error) {
	return &Config{}, nil
}

// openEnforcementApplyDB opens an in-memory database with the tables ApplyConfig reads, and one enabled host, a
// default security row in block mode and one ruleset.
func openEnforcementApplyDB(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(
		&models.ProxyHost{}, &models.Location{}, &models.Setting{}, &models.CaddyConfig{},
		&models.SSLCertificate{}, &models.AccessList{}, &models.SecurityHeaderProfile{},
		&models.DNSProvider{}, &models.SecurityConfig{}, &models.SecurityRuleSet{}, &models.SecurityDecision{},
	))
	require.NoError(t, db.Create(&models.SecurityConfig{Name: models.DefaultSecurityConfigName, Enabled: true, WAFMode: "block"}).Error)
	require.NoError(t, db.Create(&models.SecurityRuleSet{UUID: "rs-1", Name: "owasp-crs", Content: "SecRule ARGS \"@contains x\" \"id:1,deny\""}).Error)
	host := enforcementTestHost("apply-host", "apply.example.com", false)
	require.NoError(t, db.Create(&host).Error)
	return db
}

// TestManager_EnforcementReport_StoredAfterSuccessfulApply pins that a successful apply stores the report it generated,
// stamped with the WAF state it was generated for, and that a manager without an apply has no report.
func TestManager_EnforcementReport_StoredAfterSuccessfulApply(t *testing.T) {
	db := openEnforcementApplyDB(t)
	manager := NewManager(&switchableCaddyClient{}, db, t.TempDir(), "", false, config.SecurityConfig{CerberusEnabled: true})

	_, ok := manager.EnforcementReport()
	require.False(t, ok, "no report before the first apply")

	require.NoError(t, manager.ApplyConfig(context.Background()))
	report, ok := manager.EnforcementReport()
	require.True(t, ok)
	require.True(t, report.WAFOn)
	require.False(t, report.RateLimitOn)
	require.Equal(t, ModuleEnforcement{Eligible: 1, Protected: 1}, report.WAF)
}

// TestManager_EnforcementReport_FailedApplyDropsReport pins that a failed apply leaves no report behind, so a report
// from an earlier successful apply is never trusted for a configuration that did not load.
func TestManager_EnforcementReport_FailedApplyDropsReport(t *testing.T) {
	db := openEnforcementApplyDB(t)
	client := &switchableCaddyClient{}
	manager := NewManager(client, db, t.TempDir(), "", false, config.SecurityConfig{CerberusEnabled: true})

	require.NoError(t, manager.ApplyConfig(context.Background()))
	_, ok := manager.EnforcementReport()
	require.True(t, ok)

	client.loadErr = errors.New("caddy refused the config")
	require.Error(t, manager.ApplyConfig(context.Background()))
	_, ok = manager.EnforcementReport()
	require.False(t, ok, "a failed apply must not leave the earlier report in place")
}

// TestManager_EnforcementReport_ApplyFailureBeforeLoadDropsReport pins the same rule when the failure happens before the
// config is loaded, here an apply that cannot read the hosts.
func TestManager_EnforcementReport_ApplyFailureBeforeLoadDropsReport(t *testing.T) {
	db := openEnforcementApplyDB(t)
	manager := NewManager(&switchableCaddyClient{}, db, t.TempDir(), "", false, config.SecurityConfig{CerberusEnabled: true})
	require.NoError(t, manager.ApplyConfig(context.Background()))

	require.NoError(t, db.Migrator().DropTable(&models.ProxyHost{}))
	require.Error(t, manager.ApplyConfig(context.Background()))
	_, ok := manager.EnforcementReport()
	require.False(t, ok)
}

// TestEnforcementReport_StampedWithoutHosts pins that a generation with no hosts still reports, stamped with its flags
// and with no eligible host.
func TestEnforcementReport_StampedWithoutHosts(t *testing.T) {
	report := generateEnforcementReport(t, nil, true, true, nil, nil, nil)
	require.True(t, report.WAFOn)
	require.True(t, report.RateLimitOn)
	require.Equal(t, ModuleEnforcement{}, report.WAF)
	require.Equal(t, ModuleEnforcement{}, report.RateLimit)
}
