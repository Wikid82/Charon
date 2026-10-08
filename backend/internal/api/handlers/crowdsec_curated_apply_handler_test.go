package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/crowdsec"
	"github.com/Wikid82/charon/backend/internal/models"
)

type scriptedHubExec struct {
	mu    sync.Mutex
	calls []string
	fn    func(ctx context.Context, cmd string) ([]byte, error)
}

func (e *scriptedHubExec) Execute(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := strings.Join(append([]string{name}, args...), " ")
	e.mu.Lock()
	e.calls = append(e.calls, cmd)
	e.mu.Unlock()
	if e.fn != nil {
		if out, err := e.fn(ctx, cmd); out != nil || err != nil {
			return out, err
		}
	}
	if strings.Contains(cmd, " inspect ") {
		return []byte(`{"installed":true,"tainted":false}`), nil
	}
	return []byte("ok"), nil
}

func (e *scriptedHubExec) installs() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	n := 0
	for _, c := range e.calls {
		if strings.Contains(c, " install ") {
			n++
		}
	}
	return n
}

type curatedHarness struct {
	router  *gin.Engine
	events  func() []models.CrowdsecPresetEvent
	dataDir string
	exec    *scriptedHubExec
	handler *CrowdsecHandler
}

func newCuratedHarness(t *testing.T, fn func(ctx context.Context, cmd string) ([]byte, error)) *curatedHarness {
	t.Helper()
	t.Setenv("FEATURE_CERBERUS_ENABLED", "true")

	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.CrowdsecPresetEvent{}))

	dataDir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "config.yaml"), []byte("original: true\n"), 0o600))
	cache, err := crowdsec.NewHubCache(filepath.Join(t.TempDir(), "cache"), time.Hour)
	require.NoError(t, err)

	exec := &scriptedHubExec{fn: fn}
	hub := crowdsec.NewHubService(exec, cache, dataDir)
	hub.ApplyTimeout = 5 * time.Second

	h := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", t.TempDir())
	h.Hub = hub

	r := gin.New()
	h.RegisterRoutes(r.Group("/api/v1"))

	return &curatedHarness{
		router:  r,
		dataDir: dataDir,
		exec:    exec,
		handler: h,
		events: func() []models.CrowdsecPresetEvent {
			var ev []models.CrowdsecPresetEvent
			require.NoError(t, db.Order("id").Find(&ev).Error)
			return ev
		},
	}
}

func (c *curatedHarness) apply(t *testing.T, slug string) (code int, resp map[string]any) {
	t.Helper()
	body, _ := json.Marshal(map[string]string{"slug": slug})
	w := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/presets/apply", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	c.router.ServeHTTP(w, req)
	var out map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	return w.Code, out
}

func TestApplyCuratedPresetInstallsViaCSCLI(t *testing.T) {
	c := newCuratedHarness(t, nil)
	slug := "honeypot-friendly-defaults"

	code, resp := c.apply(t, slug)
	require.Equal(t, http.StatusOK, code)
	require.Equal(t, "applied", resp["status"])
	require.Equal(t, slug, resp["slug"])
	require.Equal(t, true, resp["used_cscli"])
	require.Equal(t, true, resp["reload_hint"])
	require.Equal(t, "curated-"+slug, resp["cache_key"])
	backup, _ := resp["backup"].(string)
	require.NotEmpty(t, backup)
	require.DirExists(t, backup)

	preset, ok := crowdsec.FindPreset(slug)
	require.True(t, ok)
	require.Equal(t, len(preset.Items), c.exec.installs(), "every item installed: not a no-op")

	events := c.events()
	require.Len(t, events, 1)
	require.Equal(t, slug, events[0].Slug)
	require.Equal(t, "applied", events[0].Status)
	require.Equal(t, backup, events[0].BackupPath)
}

func TestApplyCuratedPresetCSCLIUnavailableReturns503(t *testing.T) {
	c := newCuratedHarness(t, func(_ context.Context, cmd string) ([]byte, error) {
		if cmd == "cscli version" {
			return nil, errors.New("not found")
		}
		return nil, nil
	})

	code, resp := c.apply(t, "honeypot-friendly-defaults")
	require.Equal(t, http.StatusServiceUnavailable, code)
	require.Equal(t, "CrowdSec CLI is not available; curated presets require cscli", resp["error"])
	require.NotContains(t, resp, "status")
	require.Equal(t, "curated-honeypot-friendly-defaults", resp["cache_key"])
	require.Equal(t, 0, c.exec.installs())

	events := c.events()
	require.Len(t, events, 1)
	require.Equal(t, "failed", events[0].Status)
	require.NotEmpty(t, events[0].Error)
}

func TestApplyCuratedPresetInstallFailureReturns500WithBackup(t *testing.T) {
	c := newCuratedHarness(t, func(_ context.Context, cmd string) ([]byte, error) {
		if strings.Contains(cmd, " install ") {
			return nil, errors.New("install exploded")
		}
		return nil, nil
	})

	code, resp := c.apply(t, "geoip-enrichment")
	require.Equal(t, http.StatusInternalServerError, code)
	require.NotEqual(t, "applied", resp["status"])
	require.Contains(t, resp["error"], "install exploded")
	backup, _ := resp["backup"].(string)
	require.NotEmpty(t, backup)

	// DataDir is intact after rollback.
	cfg, err := os.ReadFile(filepath.Join(c.dataDir, "config.yaml"))
	require.NoError(t, err)
	require.Equal(t, "original: true\n", string(cfg))

	events := c.events()
	require.Len(t, events, 1)
	require.Equal(t, "failed", events[0].Status)
	require.Equal(t, backup, events[0].BackupPath)
	require.Contains(t, events[0].Error, "install exploded")
}

func TestApplyCuratedPresetTimeoutReturns504(t *testing.T) {
	c := newCuratedHarness(t, func(ctx context.Context, cmd string) ([]byte, error) {
		if strings.Contains(cmd, " install ") {
			<-ctx.Done()
			return nil, ctx.Err()
		}
		return nil, nil
	})
	c.handler.Hub.ApplyTimeout = 50 * time.Millisecond

	code, resp := c.apply(t, "geoip-enrichment")
	require.Equal(t, http.StatusGatewayTimeout, code)
	require.NotContains(t, resp, "status")
	require.Equal(t, "failed", c.events()[0].Status)
}

func TestApplyCuratedPresetInvalidDefinitionReturns500(t *testing.T) {
	c := newCuratedHarness(t, nil)
	h := c.handler

	res, err := h.Hub.ApplyCurated(context.Background(), crowdsec.Preset{Slug: "bad"})
	require.ErrorIs(t, err, crowdsec.ErrInvalidPresetDefinition)
	require.Equal(t, "failed", res.Status)

	w := httptest.NewRecorder()
	ctx, _ := gin.CreateTestContext(w)
	ctx.Request = httptest.NewRequest(http.MethodPost, "/", http.NoBody)
	h.applyCuratedPreset(ctx, crowdsec.Preset{Slug: "bad"})
	require.Equal(t, http.StatusInternalServerError, w.Code)
	require.Equal(t, 0, c.exec.installs())
}

func TestApplyPresetEventPersistenceFailureDoesNotChangeOutcome(t *testing.T) {
	t.Setenv("FEATURE_CERBERUS_ENABLED", "true")
	for _, tc := range []struct {
		name string
		slug string
		code int
	}{
		{"curated success", "geoip-enrichment", http.StatusOK},
		{"hub path failure", "unknown/hub-preset", http.StatusInternalServerError},
	} {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			// The preset-event table is intentionally not migrated: DB.Create fails.
			db := OpenTestDB(t)
			dataDir := t.TempDir()
			cache, err := crowdsec.NewHubCache(filepath.Join(t.TempDir(), "cache"), time.Hour)
			require.NoError(t, err)
			hub := crowdsec.NewHubService(&scriptedHubExec{fn: failHubInstall}, cache, dataDir)
			hub.ApplyTimeout = 5 * time.Second
			h := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", t.TempDir())
			h.Hub = hub
			r := gin.New()
			h.RegisterRoutes(r.Group("/api/v1"))

			body, _ := json.Marshal(map[string]string{"slug": tc.slug})
			w := httptest.NewRecorder()
			req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/presets/apply", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")
			r.ServeHTTP(w, req)
			require.Equal(t, tc.code, w.Code, w.Body.String())
		})
	}
}

func TestApplyCuratedPresetOldGeolocationSlugNeverSucceeds(t *testing.T) {
	// A removed slug takes the hub path, where real cscli rejects unknown items.
	c := newCuratedHarness(t, failHubInstall)
	code, resp := c.apply(t, "geolocation-aware")
	require.GreaterOrEqual(t, code, 400)
	require.NotEqual(t, "applied", resp["status"])
}

// failHubInstall mimics cscli rejecting an item that is not in the hub.
func failHubInstall(_ context.Context, cmd string) ([]byte, error) {
	if strings.HasPrefix(cmd, "cscli hub install") {
		return nil, errors.New("unknown item")
	}
	return nil, nil
}
