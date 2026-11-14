package caddy

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
)

// recordingGuard refuses the targets in refused and records every check.
type recordingGuard struct {
	refused map[string]bool
	err     error
	checked []string
}

func (g *recordingGuard) RefusesForwardTarget(host string, port int) (bool, error) {
	key := host
	g.checked = append(g.checked, key)
	if g.err != nil {
		return false, g.err
	}
	return g.refused[key], nil
}

func TestDropRefusedForwardTargets_NilGuardKeepsEveryHost(t *testing.T) {
	hosts := []models.ProxyHost{{UUID: "a", Enabled: true, ForwardHost: "127.0.0.1", ForwardPort: 2375}}
	got, err := dropRefusedForwardTargets(nil, hosts)
	require.NoError(t, err)
	assert.Equal(t, hosts, got)
}

func TestDropRefusedForwardTargets_DropsOnlyRefusedHosts(t *testing.T) {
	guard := &recordingGuard{refused: map[string]bool{"127.0.0.1": true}}
	hosts := []models.ProxyHost{
		{UUID: "bad", Enabled: true, ForwardHost: "127.0.0.1", ForwardPort: 2375},
		{UUID: "fine", Enabled: true, ForwardHost: "10.0.0.5", ForwardPort: 8080},
	}
	got, err := dropRefusedForwardTargets(guard, hosts)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "fine", got[0].UUID)
}

func TestDropRefusedForwardTargets_RefusedLocationDropsWholeHost(t *testing.T) {
	guard := &recordingGuard{refused: map[string]bool{"localhost": true}}
	hosts := []models.ProxyHost{
		{
			UUID:        "with-location",
			Enabled:     true,
			ForwardHost: "10.0.0.5",
			ForwardPort: 8080,
			Locations:   []models.Location{{Path: "/x", ForwardHost: "localhost", ForwardPort: 2375}},
		},
	}
	got, err := dropRefusedForwardTargets(guard, hosts)
	require.NoError(t, err)
	assert.Empty(t, got)
}

func TestDropRefusedForwardTargets_DisabledHostsAreNotChecked(t *testing.T) {
	guard := &recordingGuard{refused: map[string]bool{"127.0.0.1": true}}
	hosts := []models.ProxyHost{{UUID: "off", Enabled: false, ForwardHost: "127.0.0.1", ForwardPort: 2375}}
	got, err := dropRefusedForwardTargets(guard, hosts)
	require.NoError(t, err)
	assert.Len(t, got, 1)
	assert.Empty(t, guard.checked)
}

func TestDropRefusedForwardTargets_GuardErrorFailsTheApply(t *testing.T) {
	guard := &recordingGuard{err: errors.New("store unavailable")}
	hosts := []models.ProxyHost{{UUID: "a", Enabled: true, ForwardHost: "127.0.0.1", ForwardPort: 2375}}
	got, err := dropRefusedForwardTargets(guard, hosts)
	require.Error(t, err)
	assert.ErrorContains(t, err, "store unavailable")
	assert.Nil(t, got)
}

func TestManagerSetForwardTargetGuard_StoresGuard(t *testing.T) {
	m := &Manager{}
	guard := &recordingGuard{}
	m.SetForwardTargetGuard(guard)
	assert.Same(t, guard, m.forwardGuard)
}

// failingLocationGuard refuses nothing and fails only the location target checks.
type failingLocationGuard struct{}

func (failingLocationGuard) RefusesForwardTarget(host string, _ int) (bool, error) {
	if host == "loc-host" {
		return false, errors.New("lookup failed")
	}
	return false, nil
}

func TestDropRefusedForwardTargets_LocationGuardErrorFailsTheApply(t *testing.T) {
	hosts := []models.ProxyHost{{
		UUID:        "loc-err",
		Enabled:     true,
		ForwardHost: "10.0.0.5",
		ForwardPort: 8080,
		Locations:   []models.Location{{Path: "/x", ForwardHost: "loc-host", ForwardPort: 1}},
	}}
	_, err := dropRefusedForwardTargets(failingLocationGuard{}, hosts)
	assert.ErrorContains(t, err, "lookup failed")
}

func TestManagerApplyConfig_GuardErrorStopsApplyBeforeCaddy(t *testing.T) {
	var caddyCalls atomic.Int32
	caddyServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		caddyCalls.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer caddyServer.Close()

	dsn := fmt.Sprintf("file:%s?mode=memory&cache=shared", t.Name())
	db, err := gorm.Open(sqlite.Open(dsn), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&models.ProxyHost{}, &models.Location{}, &models.Setting{}, &models.CaddyConfig{}, &models.SSLCertificate{}))
	require.NoError(t, db.Create(&models.ProxyHost{
		UUID:        "guarded",
		DomainNames: "guarded.example.com",
		ForwardHost: "10.0.0.5",
		ForwardPort: 8080,
		Enabled:     true,
	}).Error)

	manager := NewManager(newTestClient(t, caddyServer.URL), db, t.TempDir(), "", false, config.SecurityConfig{})
	manager.SetForwardTargetGuard(&recordingGuard{err: errors.New("lookup failed")})

	err = manager.ApplyConfig(context.Background())
	require.ErrorContains(t, err, "lookup failed")
	assert.Zero(t, caddyCalls.Load(), "a failed target check must not push any config")
}

// configWithUpstream returns a config whose only route proxies to dial.
func configWithUpstream(dial string) *Config {
	return &Config{Apps: Apps{HTTP: &HTTPApp{Servers: map[string]*Server{
		"s": {Routes: []*Route{{Handle: []Handler{ReverseProxyHandler(dial, "", true)}}}},
	}}}}
}

// prefixGuard refuses every host with the given prefix.
type prefixGuard struct{ prefix string }

func (g prefixGuard) RefusesForwardTarget(host string, _ int) (bool, error) {
	return strings.HasPrefix(host, g.prefix), nil
}

func TestSnapshotRoutesRefusedTarget(t *testing.T) {
	refused, err := snapshotRoutesRefusedTarget(prefixGuard{prefix: "127."}, configWithUpstream("127.0.0.1:2375"))
	require.NoError(t, err)
	assert.True(t, refused)

	refused, err = snapshotRoutesRefusedTarget(prefixGuard{prefix: "127."}, configWithUpstream("10.0.0.5:8080"))
	require.NoError(t, err)
	assert.False(t, refused)

	refused, err = snapshotRoutesRefusedTarget(nil, configWithUpstream("127.0.0.1:2375"))
	require.NoError(t, err)
	assert.False(t, refused)

	refused, err = snapshotRoutesRefusedTarget(prefixGuard{prefix: "127."}, nil)
	require.NoError(t, err)
	assert.False(t, refused)
}

func TestSnapshotRoutesRefusedTarget_GuardErrorIsReturned(t *testing.T) {
	_, err := snapshotRoutesRefusedTarget(&recordingGuard{err: errors.New("store down")}, configWithUpstream("127.0.0.1:2375"))
	assert.ErrorContains(t, err, "store down")
}

func TestWalkDialsRefused_SkipsUnparseableDials(t *testing.T) {
	doc := map[string]any{
		"list": []any{"not a map", map[string]any{"dial": "no-port"}, map[string]any{"dial": "host:notanumber"}},
		"dial": 42,
	}
	refused, err := walkDialsRefused(prefixGuard{prefix: ""}, doc)
	require.NoError(t, err)
	assert.False(t, refused)
}

func TestRollback_DoesNotReinstateSnapshotWithRefusedTarget(t *testing.T) {
	m, client, _, _ := newHopManager(t)
	m.forwardGuard = prefixGuard{prefix: "127."}
	_, err := m.saveSnapshot(configWithUpstream("127.0.0.1:2375"))
	require.NoError(t, err)

	err = m.rollback(context.Background())
	require.ErrorIs(t, err, errSnapshotRefusedTarget)
	assert.Empty(t, client.loaded)
}

func TestRollback_GuardErrorKeepsSnapshotOut(t *testing.T) {
	m, client, _, _ := newHopManager(t)
	m.forwardGuard = &recordingGuard{err: errors.New("store down")}
	_, err := m.saveSnapshot(configWithUpstream("127.0.0.1:2375"))
	require.NoError(t, err)

	err = m.rollback(context.Background())
	require.ErrorContains(t, err, "check snapshot targets")
	assert.Empty(t, client.loaded)
}

func TestRollback_ReinstatesSnapshotWithAcceptedTargets(t *testing.T) {
	m, client, _, _ := newHopManager(t)
	m.forwardGuard = prefixGuard{prefix: "127."}
	_, err := m.saveSnapshot(configWithUpstream("10.0.0.5:8080"))
	require.NoError(t, err)

	require.NoError(t, m.rollback(context.Background()))
	assert.Len(t, client.loaded, 1)
}

func TestDropRefusedForwardTargets_StoredAgentNameRowIsSkipped(t *testing.T) {
	guard := &recordingGuard{refused: map[string]bool{"orthrus:agent-uuid": true}}
	hosts := []models.ProxyHost{
		{UUID: "stored-agent", Enabled: true, ForwardHost: "orthrus:agent-uuid", ForwardPort: 8080},
		{UUID: "fine", Enabled: true, ForwardHost: "10.0.0.5", ForwardPort: 8080},
	}
	got, err := dropRefusedForwardTargets(guard, hosts)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "fine", got[0].UUID)
}
