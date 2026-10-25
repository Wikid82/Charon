package handlers

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/caddy"
	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
)

// stopRequest runs Stop with the given request context and returns the recorded response.
func stopRequest(t *testing.T, h *CrowdsecHandler, reqCtx context.Context) *httptest.ResponseRecorder {
	t.Helper()
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(http.MethodPost, "/admin/crowdsec/stop", http.NoBody).WithContext(reqCtx)
	h.Stop(c)
	return w
}

// seedRunningCrowdSecState stores the state a started CrowdSec leaves behind: the default row in local mode
// and the enabled setting set to true.
func seedRunningCrowdSecState(t *testing.T, h *CrowdsecHandler) {
	t.Helper()
	require.NoError(t, h.DB.Model(&models.SecurityConfig{}).
		Where("name = ?", models.DefaultSecurityConfigName).
		Update("crowd_sec_mode", "local").Error)
	require.NoError(t, upsertSetting(h.DB, models.Setting{Key: "security.crowdsec.enabled", Value: "true", Category: "security", Type: "bool"}))
}

// readStoppedCrowdSecState returns the default row's mode and the enabled setting's value.
func readStoppedCrowdSecState(t *testing.T, h *CrowdsecHandler) (mode, setting string) {
	t.Helper()
	var cfg models.SecurityConfig
	require.NoError(t, h.DB.Where("name = ?", models.DefaultSecurityConfigName).First(&cfg).Error)
	var s models.Setting
	require.NoError(t, h.DB.Where("key = ?", "security.crowdsec.enabled").First(&s).Error)
	return cfg.CrowdSecMode, s.Value
}

// Stop applies the proxy configuration exactly once after a successful stop, and never after a failed one.

func TestStopApply_SuccessAppliesOnceAfterStateIsWritten(t *testing.T) {
	lapiURL, _ := newKeyLAPI(t)
	clearKeyEnv(t, "")
	var modeAtApply, settingAtApply string
	var h *CrowdsecHandler
	applier := &stateCapturingApplier{onApply: func() {
		modeAtApply, settingAtApply = readStoppedCrowdSecState(t, h)
	}}
	h = newStartApplyHandler(t, lapiURL, t.TempDir(), new(MockCommandExecutor), &fakeExec{started: true}, applier)
	seedRunningCrowdSecState(t, h)

	w := stopRequest(t, h, context.Background())

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), `"status":"stopped"`)
	require.Equal(t, 1, applier.count(), "one apply after a successful stop")
	require.Equal(t, "disabled", modeAtApply, "the apply must see the mode Stop wrote")
	require.Equal(t, "false", settingAtApply, "the apply must see the setting Stop wrote")
}

func TestStopApply_AlreadyStoppedAppliesOnce(t *testing.T) {
	lapiURL, _ := newKeyLAPI(t)
	clearKeyEnv(t, "")
	applier := &recordingApplier{}
	// The executor reports success when nothing is running, so Stop is a successful stop.
	h := newStartApplyHandler(t, lapiURL, t.TempDir(), new(MockCommandExecutor), &fakeExec{}, applier)

	w := stopRequest(t, h, context.Background())

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Equal(t, 1, applier.count(), "a stop that finds nothing running still confirms the disabled state")
}

func TestStopApply_ExecutorStopFailureDoesNotApply(t *testing.T) {
	lapiURL, _ := newKeyLAPI(t)
	clearKeyEnv(t, "")
	applier := &recordingApplier{}
	exec := &fakeExecWithError{stopError: errors.New("signal failed")}
	h := newStartApplyHandler(t, lapiURL, t.TempDir(), new(MockCommandExecutor), exec, applier)
	seedRunningCrowdSecState(t, h)

	w := stopRequest(t, h, context.Background())

	require.Equal(t, http.StatusInternalServerError, w.Code, w.Body.String())
	require.Zero(t, applier.count(), "a failed stop writes nothing, so nothing is applied")
	mode, setting := readStoppedCrowdSecState(t, h)
	require.Equal(t, "local", mode, "a failed stop leaves the mode as it was")
	require.Equal(t, "true", setting, "a failed stop leaves the setting as it was")
}

func TestStopApply_BusyRefusalDoesNotApply(t *testing.T) {
	lapiURL, _ := newKeyLAPI(t)
	clearKeyEnv(t, "")
	applier := &recordingApplier{}
	h := newStartApplyHandler(t, lapiURL, t.TempDir(), new(MockCommandExecutor), &fakeExec{started: true}, applier)
	setBackgroundInstallActive(t, h)

	w := stopRequest(t, h, context.Background())

	require.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	require.Zero(t, applier.count())
}

func TestStopApply_ApplyFailureKeepsResponseAndState(t *testing.T) {
	lapiURL, _ := newKeyLAPI(t)
	clearKeyEnv(t, "")
	logs := captureLogs(t)
	applier := &recordingApplier{err: errors.New("caddy admin unreachable")}
	h := newStartApplyHandler(t, lapiURL, t.TempDir(), new(MockCommandExecutor), &fakeExec{started: true}, applier)
	seedRunningCrowdSecState(t, h)

	w := stopRequest(t, h, context.Background())

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), `"status":"stopped"`)
	require.Equal(t, 1, applier.count())
	require.Contains(t, logs.String(), "Failed to apply the proxy configuration after a CrowdSec state change")
	mode, setting := readStoppedCrowdSecState(t, h)
	require.Equal(t, "disabled", mode, "a failed apply does not revert the stop")
	require.Equal(t, "false", setting)
}

func TestStopApply_NilManagerLogsNotWired(t *testing.T) {
	lapiURL, _ := newKeyLAPI(t)
	clearKeyEnv(t, "")
	logs := captureLogs(t)
	h := newStartApplyHandler(t, lapiURL, t.TempDir(), new(MockCommandExecutor), &fakeExec{started: true}, nil)

	w := stopRequest(t, h, context.Background())

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Contains(t, logs.String(), errCaddyNotWired.Error())
}

// TestStopApply_ApplyRunsAfterClientDisconnect: a cancelled request must not cancel the apply after a stop.
func TestStopApply_ApplyRunsAfterClientDisconnect(t *testing.T) {
	lapiURL, _ := newKeyLAPI(t)
	clearKeyEnv(t, "")
	applier := &recordingApplier{}
	h := newStartApplyHandler(t, lapiURL, t.TempDir(), new(MockCommandExecutor), &fakeExec{started: true}, applier)

	reqCtx, cancel := context.WithCancel(context.Background())
	cancel()
	w := stopRequest(t, h, reqCtx)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Len(t, applier.seen, 1, "the apply must run even though the request was cancelled")
	seen := applier.seen[0]
	require.NoError(t, seen.ctxErr, "the apply must not inherit the client's cancellation")
	require.True(t, seen.hasDeadline, "the apply must be bounded")
	require.LessOrEqual(t, time.Until(seen.deadline), crowdsecApplyTimeout)
}

// TestStopApply_AppliedConfigHasNoCrowdSec drives the real Caddy manager: before Stop the applied config
// carries apps.crowdsec, after Stop (mode disabled, setting false) it does not.
func TestStopApply_AppliedConfigHasNoCrowdSec(t *testing.T) {
	lapiURL, _ := newKeyLAPI(t)
	clearKeyEnv(t, "test-env-key")
	client := &loadCapturingCaddyClient{}
	h := newStartApplyHandler(t, lapiURL, t.TempDir(), new(MockCommandExecutor), &fakeExec{started: true}, nil)
	migrateCaddyApplyTables(t, h.DB)
	// The manager reads the same DB the handler writes, so Stop's state change is what the next apply sees.
	manager := caddy.NewManager(client, h.DB, t.TempDir(), "", false, config.SecurityConfig{CerberusEnabled: true})
	h.caddyManager = manager
	seedRunningCrowdSecState(t, h)
	require.NoError(t, h.DB.Model(&models.SecurityConfig{}).
		Where("name = ?", models.DefaultSecurityConfigName).
		Update("enabled", true).Error)

	require.NoError(t, manager.ApplyConfig(context.Background()))
	before := client.last()
	require.NotNil(t, before)
	require.NotNil(t, before.Apps.CrowdSec, "precondition: CrowdSec is in the config while it runs")

	w := stopRequest(t, h, context.Background())

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	after := client.last()
	require.NotNil(t, after)
	require.Nil(t, after.Apps.CrowdSec, "after Stop the applied config must have no apps.crowdsec")
}

// stateCapturingApplier runs onApply on each apply, so a test can read the state the apply saw.
type stateCapturingApplier struct {
	onApply func()
	mu      sync.Mutex
	calls   int
}

func (s *stateCapturingApplier) ApplyConfig(context.Context) error {
	s.mu.Lock()
	s.calls++
	s.mu.Unlock()
	s.onApply()
	return nil
}

func (s *stateCapturingApplier) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.calls
}

// loadCapturingCaddyClient records every config the Caddy manager loads.
type loadCapturingCaddyClient struct {
	mu      sync.Mutex
	configs []*caddy.Config
}

func (c *loadCapturingCaddyClient) Load(_ context.Context, cfg *caddy.Config) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.configs = append(c.configs, cfg)
	return nil
}

func (c *loadCapturingCaddyClient) Ping(context.Context) error { return nil }

func (c *loadCapturingCaddyClient) GetConfig(context.Context) (*caddy.Config, error) {
	return c.last(), nil
}

func (c *loadCapturingCaddyClient) last() *caddy.Config {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.configs) == 0 {
		return nil
	}
	return c.configs[len(c.configs)-1]
}

// migrateCaddyApplyTables creates the tables the Caddy manager's ApplyConfig reads.
func migrateCaddyApplyTables(t *testing.T, db interface {
	AutoMigrate(dst ...any) error
}) {
	t.Helper()
	require.NoError(t, db.AutoMigrate(
		&models.ProxyHost{}, &models.Location{}, &models.SSLCertificate{}, &models.AccessList{},
		&models.SecurityHeaderProfile{}, &models.DNSProvider{}, &models.CaddyConfig{},
		&models.SecurityRuleSet{}, &models.SecurityDecision{},
	))
}
