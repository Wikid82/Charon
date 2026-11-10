package caddy

import (
	"context"
	"errors"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
)

// appliedOnce returns a manager whose first apply succeeded, and the report that apply recorded.
func appliedOnce(t *testing.T, client *switchableCaddyClient) (*Manager, EnforcementReport) {
	t.Helper()
	manager := NewManager(client, openEnforcementApplyDB(t), t.TempDir(), "", false, config.SecurityConfig{CerberusEnabled: true})
	require.NoError(t, manager.ApplyConfig(context.Background()))
	report, ok := manager.LastAppliedReport()
	require.True(t, ok)
	return manager, report
}

// TestManager_LastAppliedReport_SetOnSuccess pins that a successful apply records its report as the last applied one.
func TestManager_LastAppliedReport_SetOnSuccess(t *testing.T) {
	manager := NewManager(&switchableCaddyClient{}, openEnforcementApplyDB(t), t.TempDir(), "", false, config.SecurityConfig{CerberusEnabled: true})

	_, ok := manager.LastAppliedReport()
	require.False(t, ok, "no last applied report before the first apply")

	require.NoError(t, manager.ApplyConfig(context.Background()))
	report, ok := manager.LastAppliedReport()
	require.True(t, ok)
	require.True(t, report.WAFOn)
	require.Equal(t, ModuleEnforcement{Eligible: 1, Protected: 1}, report.WAF)
}

// TestManager_LastAppliedReport_KeptWhenLoadFails pins that a failed Caddy load keeps the report of the last successful
// apply, because Caddy keeps running that configuration.
func TestManager_LastAppliedReport_KeptWhenLoadFails(t *testing.T) {
	client := &switchableCaddyClient{}
	manager, before := appliedOnce(t, client)

	client.loadErr = errors.New("caddy refused the config")
	require.Error(t, manager.ApplyConfig(context.Background()))

	after, ok := manager.LastAppliedReport()
	require.True(t, ok, "the last applied report survives a failed load")
	require.Equal(t, before, after)
}

// TestManager_LastAppliedReport_KeptWhenValidationFails pins that a configuration rejected before it is loaded keeps
// the report of the last successful apply.
func TestManager_LastAppliedReport_KeptWhenValidationFails(t *testing.T) {
	manager, before := appliedOnce(t, &switchableCaddyClient{})

	origValidate := validateConfigFunc
	t.Cleanup(func() { validateConfigFunc = origValidate })
	validateConfigFunc = func(*Config) error { return errors.New("invalid") }

	require.Error(t, manager.ApplyConfig(context.Background()))

	after, ok := manager.LastAppliedReport()
	require.True(t, ok)
	require.Equal(t, before, after)
}

// TestManager_LastAppliedReport_KeptWhenHostsUnreadable pins that a failure before any generation keeps the report.
func TestManager_LastAppliedReport_KeptWhenHostsUnreadable(t *testing.T) {
	manager, before := appliedOnce(t, &switchableCaddyClient{})
	db := manager.db
	require.NoError(t, db.Migrator().DropTable(&models.ProxyHost{}))

	require.Error(t, manager.ApplyConfig(context.Background()))

	after, ok := manager.LastAppliedReport()
	require.True(t, ok)
	require.Equal(t, before, after)
}

// TestManager_LastAppliedReport_UnchangedWhenApplyStarts pins that starting an apply does not clear the last applied
// report, which keeps describing the running config until a new apply succeeds.
func TestManager_LastAppliedReport_UnchangedWhenApplyStarts(t *testing.T) {
	client := &switchableCaddyClient{}
	manager, before := appliedOnce(t, client)

	client.loadErr = errors.New("still failing")
	require.Error(t, manager.ApplyConfig(context.Background()))

	after, ok := manager.LastAppliedReport()
	require.True(t, ok)
	require.Equal(t, before, after)
}
