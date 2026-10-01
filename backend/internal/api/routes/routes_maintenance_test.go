package routes

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/api/handlers"
	"github.com/Wikid82/charon/backend/internal/caddy"
	"github.com/Wikid82/charon/backend/internal/cerberus"
	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/dbmaint"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/server"
)

// maintenanceRig is the production wiring (server.NewRouter + gate middleware +
// RegisterWithDeps) over an in-memory database with the real single-connection
// pool, the way cmd/api/main.go builds it.
type maintenanceRig struct {
	router   *gin.Engine
	db       *gorm.DB
	pin      func() func() // pins the pool's only connection; returns the release
	cerb     *cerberus.Cerberus
	shutdown UptimeShutdownFunc
	cancel   context.CancelFunc
}

func newMaintenanceRig(t *testing.T, gate *dbmaint.Gate) *maintenanceRig {
	t.Helper()
	gin.SetMode(gin.TestMode)

	db, err := gorm.Open(sqlite.Open(isolatedMemoryDSN(t)), &gorm.Config{})
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)

	frontend := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(frontend, "index.html"), []byte("<html>spa-index</html>"), 0o600))

	cfg := config.Config{JWTSecret: "test-secret", FrontendDir: frontend}
	router := server.NewRouter(frontend, "", nil)
	router.Use(gate.Middleware(handlers.HealthHandler))

	caddyManager := caddy.NewManager(caddy.NewClient(cfg.CaddyAdminAPI), db, cfg.CaddyConfigDir, cfg.FrontendDir, cfg.ACMEStaging, cfg.Security)
	cerb := cerberus.New(cfg.Security, db)
	ctx, cancel := context.WithCancel(context.Background())
	shutdown, err := RegisterWithDeps(ctx, router, db, cfg, caddyManager, cerb, gate)
	require.NoError(t, err)
	t.Cleanup(cancel)

	rig := &maintenanceRig{router: router, db: db, cerb: cerb, shutdown: shutdown, cancel: cancel}
	rig.pin = func() func() {
		conn, err := sqlDB.Conn(context.Background())
		require.NoError(t, err)
		return func() { _ = conn.Close() }
	}
	return rig
}

// get serves the request and fails the test if it does not finish within a
// second, which is how a request blocked on the pinned pool shows up.
func (r *maintenanceRig) get(t *testing.T, method, path string, headers ...string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, path, http.NoBody)
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	w := httptest.NewRecorder()
	done := make(chan struct{})
	go func() { r.router.ServeHTTP(w, req); close(done) }()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatalf("%s %s did not answer within 1s: it is blocked on the pinned pool", method, path)
	}
	return w
}

func activeTestGate(t *testing.T) *dbmaint.Gate {
	t.Helper()
	g := dbmaint.NewGate()
	g.MarkPlanned()
	require.NoError(t, g.BeginChecking())
	require.NoError(t, g.BeginConverting(1))
	return g
}

func statusPhase(t *testing.T, w *httptest.ResponseRecorder) (phase string, active bool) {
	t.Helper()
	require.Equal(t, http.StatusOK, w.Code)
	var body struct {
		Active bool   `json:"active"`
		Phase  string `json:"phase"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body), w.Body.String())
	return body.Phase, body.Active
}

// 2.2: with the pool's only connection pinned and the rate-limit cache
// expired, the healthcheck and the status page must still answer, and the
// DB-backed health must fail fast.
func TestGate_PinnedPoolHealthAndStatusStillAnswer(t *testing.T) {
	rig := newMaintenanceRig(t, activeTestGate(t))
	release := rig.pin()
	t.Cleanup(release)
	rig.cerb.InvalidateCache()

	health := rig.get(t, http.MethodGet, "/api/v1/health")
	assert.Equal(t, http.StatusOK, health.Code)
	assert.Contains(t, health.Body.String(), `"status":"ok"`)
	assert.Equal(t, http.StatusOK, rig.get(t, http.MethodHead, "/api/v1/health").Code)

	phase, active := statusPhase(t, rig.get(t, http.MethodGet, "/api/v1/maintenance/status"))
	assert.Equal(t, "converting", phase)
	assert.True(t, active)

	dbHealth := rig.get(t, http.MethodGet, "/api/v1/health/db")
	assert.Equal(t, http.StatusServiceUnavailable, dbHealth.Code)
	assert.Equal(t, "15", dbHealth.Header().Get("Retry-After"))
}

// The status page is the gate's own answer in every phase, so it survives a
// pinned pool even when the gate is idle.
func TestGate_PinnedPoolIdleGateStillAnswersStatus(t *testing.T) {
	rig := newMaintenanceRig(t, dbmaint.NewGate())
	release := rig.pin()
	t.Cleanup(release)
	rig.cerb.InvalidateCache()

	phase, active := statusPhase(t, rig.get(t, http.MethodGet, "/api/v1/maintenance/status"))
	assert.Equal(t, "idle", phase)
	assert.False(t, active)
}

// setStrictRateLimit enables the API limiter with a one-request budget.
func (r *maintenanceRig) setStrictRateLimit(t *testing.T) {
	t.Helper()
	for k, v := range map[string]string{
		"security.rate_limit.enabled":  "true",
		"security.rate_limit.requests": "1",
		"security.rate_limit.window":   "60",
		"security.rate_limit.burst":    "1",
	} {
		require.NoError(t, r.db.Where(models.Setting{Key: k}).Assign(models.Setting{Value: v}).FirstOrCreate(&models.Setting{}).Error)
	}
	r.cerb.InvalidateCache()
}

// Outside maintenance the gate must not answer health itself: it stays behind
// the normal chain, so the API rate limiter applies to it.
func TestGate_IdleHealthIsSubjectToTheRateLimiter(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		t.Run(method, func(t *testing.T) {
			rig := newMaintenanceRig(t, dbmaint.NewGate())
			assert.Equal(t, http.StatusOK, rig.get(t, method, "/api/v1/health").Code, "normal traffic is answered")

			rig.setStrictRateLimit(t)
			var limited int
			for range 30 {
				if rig.get(t, method, "/api/v1/health?rapid=1").Code == http.StatusTooManyRequests {
					limited++
				}
			}
			assert.Positive(t, limited, "a burst of health checks must hit the limiter")
		})
	}
}

func TestGate_InactivePhasesLeaveHealthToTheRateLimiter(t *testing.T) {
	for _, phase := range []dbmaint.Phase{dbmaint.PhasePlanned, dbmaint.PhaseDone, dbmaint.PhaseSkipped, dbmaint.PhaseFailed} {
		t.Run(string(phase), func(t *testing.T) {
			gate := dbmaint.NewGate()
			gate.MarkPlanned()
			if phase != dbmaint.PhasePlanned {
				gate.Finish(dbmaint.FinishInfo{Phase: phase})
			}
			rig := newMaintenanceRig(t, gate)
			rig.setStrictRateLimit(t)

			codes := map[int]int{}
			for range 10 {
				codes[rig.get(t, http.MethodGet, "/api/v1/health").Code]++
			}
			assert.Positive(t, codes[http.StatusTooManyRequests])
		})
	}
}

// While the gate is active the DB-free answer replaces the normal chain, so
// the limiter does not apply and the pinned pool cannot block it.
func TestGate_ActiveHealthBypassesTheRateLimiter(t *testing.T) {
	rig := newMaintenanceRig(t, activeTestGate(t))
	rig.setStrictRateLimit(t)
	for range 10 {
		assert.Equal(t, http.StatusOK, rig.get(t, http.MethodGet, "/api/v1/health").Code)
	}
}

func TestGate_StatusIsJSONAfterCompletion(t *testing.T) {
	for _, phase := range []dbmaint.Phase{dbmaint.PhaseDone, dbmaint.PhaseSkipped, dbmaint.PhaseFailed} {
		t.Run(string(phase), func(t *testing.T) {
			gate := dbmaint.NewGate()
			gate.MarkPlanned()
			gate.Finish(dbmaint.FinishInfo{Phase: phase})
			rig := newMaintenanceRig(t, gate)

			w := rig.get(t, http.MethodGet, "/api/v1/maintenance/status")
			got, active := statusPhase(t, w)
			assert.Equal(t, string(phase), got)
			assert.False(t, active)
			assert.Contains(t, w.Header().Get("Content-Type"), "application/json")
			assert.NotContains(t, w.Body.String(), "spa-index", "never the SPA fallback")
		})
	}
}

func TestGate_StatusIsJSONInIdleWithoutAPlan(t *testing.T) {
	rig := newMaintenanceRig(t, dbmaint.NewGate())
	w := rig.get(t, http.MethodGet, "/api/v1/maintenance/status")
	phase, active := statusPhase(t, w)
	assert.Equal(t, "idle", phase)
	assert.False(t, active)
	assert.NotContains(t, w.Body.String(), "spa-index")
}

// 3.6: static routes stay ungated; unmatched paths get the 503 page; the API
// gets 503 JSON.
func TestGate_ActivePhaseSPAFlow(t *testing.T) {
	rig := newMaintenanceRig(t, activeTestGate(t))

	root := rig.get(t, http.MethodGet, "/")
	assert.Equal(t, http.StatusOK, root.Code)
	assert.Contains(t, root.Body.String(), "spa-index")

	deep := rig.get(t, http.MethodGet, "/settings/system", "Accept", "text/html")
	assert.Equal(t, http.StatusServiceUnavailable, deep.Code)
	assert.Contains(t, deep.Header().Get("Content-Type"), "text/html")
	assert.Contains(t, deep.Header().Get("Content-Security-Policy"), "script-src 'sha256-")
	assert.Equal(t, "no-store", deep.Header().Get("Cache-Control"))
	assert.Contains(t, deep.Body.String(), "Optimizing the database")

	api := rig.get(t, http.MethodGet, "/api/v1/auth/me")
	assert.Equal(t, http.StatusServiceUnavailable, api.Code)
	assert.JSONEq(t, `{"error":"Database optimization in progress","maintenance":true,"retry_after_seconds":15}`, api.Body.String())
}

func TestGate_InactivePhasesServeTheAppNormally(t *testing.T) {
	gate := dbmaint.NewGate()
	gate.MarkPlanned() // planned: a conversion is likely but nothing is blocked
	rig := newMaintenanceRig(t, gate)

	deep := rig.get(t, http.MethodGet, "/settings/system")
	assert.Equal(t, http.StatusOK, deep.Code)
	assert.Contains(t, deep.Body.String(), "spa-index")
	assert.Equal(t, http.StatusUnauthorized, rig.get(t, http.MethodGet, "/api/v1/auth/me").Code)
}

// L8: gin does not route HEAD to GET routes, so a dedicated HEAD route keeps
// the two methods consistent even without a gate.
func TestHeadHealthMatchesGetHealthWithoutAGate(t *testing.T) {
	rig := newMaintenanceRig(t, nil)
	assert.Equal(t, http.StatusOK, rig.get(t, http.MethodGet, "/api/v1/health").Code)
	assert.Equal(t, http.StatusOK, rig.get(t, http.MethodHead, "/api/v1/health").Code)
}

func TestHeadHealthMatchesGetHealthInIdleAndActivePhases(t *testing.T) {
	for _, gate := range []*dbmaint.Gate{dbmaint.NewGate(), activeTestGate(t)} {
		rig := newMaintenanceRig(t, gate)
		get := rig.get(t, http.MethodGet, "/api/v1/health")
		head := rig.get(t, http.MethodHead, "/api/v1/health")
		assert.Equal(t, get.Code, head.Code, gate.Snapshot().Phase)
	}
}

// The production planner reads the database file; an install it cannot inspect
// (here an in-memory database) or one that needs no work leaves the gate idle
// so nothing is deferred and no pool is pinned.
func TestRegisterWithDeps_ProductionPlanLeavesTheGateIdleAndDefersNothing(t *testing.T) {
	gate := dbmaint.NewGate()
	rig := newMaintenanceRig(t, gate)

	assert.Equal(t, dbmaint.PhaseIdle, gate.Snapshot().Phase)
	assert.False(t, gate.Deferring())
	assert.True(t, gate.WaitRunner(time.Millisecond), "no runner was started")
	assert.NotNil(t, rig.shutdown)
}

func TestRegisterWithDeps_NilGateIsPermanentlyIdle(t *testing.T) {
	rig := newMaintenanceRig(t, nil)
	assert.Equal(t, http.StatusOK, rig.get(t, http.MethodGet, "/api/v1/health").Code)
}

// The pipeline waits behind a planned gate; cancelling the app context while
// it waits must still let the ordered shutdown complete (uptimeIngesterDone).
func TestRegisterWithDeps_ShutdownCompletesWhileThePipelineIsDeferred(t *testing.T) {
	gate := dbmaint.NewGate()
	gate.MarkPlanned()
	rig := newMaintenanceRig(t, gate)

	rig.cancel()
	graceCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	assert.NoError(t, rig.shutdown(graceCtx), "the ingester-done channel closes even though the ingester never ran")
}

func TestRunWhenIdle(t *testing.T) {
	var runs atomic.Int32
	run := func(context.Context) { runs.Add(1) }

	t.Run("runs at once on an idle gate", func(t *testing.T) {
		runWhenIdle(context.Background(), dbmaint.NewGate(), run)
		assert.Equal(t, int32(1), runs.Swap(0))
	})
	t.Run("runs on a nil gate", func(t *testing.T) {
		runWhenIdle(context.Background(), nil, run)
		assert.Equal(t, int32(1), runs.Swap(0))
	})
	t.Run("waits for the release", func(t *testing.T) {
		gate := dbmaint.NewGate()
		gate.MarkPlanned()
		done := make(chan struct{})
		go func() { runWhenIdle(context.Background(), gate, run); close(done) }()

		time.Sleep(40 * time.Millisecond)
		assert.Zero(t, runs.Load(), "not started while planned")
		gate.Release()
		select {
		case <-done:
		case <-time.After(time.Second):
			t.Fatal("runWhenIdle did not return after the release")
		}
		assert.Equal(t, int32(1), runs.Swap(0))
	})
	t.Run("never runs when the context ends first", func(t *testing.T) {
		gate := dbmaint.NewGate()
		gate.MarkPlanned()
		ctx, cancel := context.WithCancel(context.Background())
		cancel()
		runWhenIdle(ctx, gate, run)
		assert.Zero(t, runs.Load())
	})
}
