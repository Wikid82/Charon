package handlers

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/crowdsec"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

// These tests cover how the handlers behave while a background hub install job runs (#1615, PR 6).

const (
	busyJobID = "00000000-0000-4000-8000-000000000001"
)

// setBackgroundInstallActive makes h behave as if install job busyJobID were running.
func setBackgroundInstallActive(t *testing.T, h *CrowdsecHandler) {
	t.Helper()
	require.True(t, h.ApplyJobs().TryAcquire(busyJobID))
	t.Cleanup(func() { h.ApplyJobs().Release(busyJobID) })
}

// newJobGate returns a gate that blocks a fake apply job until release is called. release is also a cleanup, so a
// failing assertion never leaks the blocked job goroutine.
func newJobGate(t *testing.T) (gate <-chan struct{}, release func()) {
	t.Helper()
	ch := make(chan struct{})
	var once sync.Once
	release = func() { once.Do(func() { close(ch) }) }
	t.Cleanup(release)
	return ch, release
}

type busyFixture struct {
	h    *CrowdsecHandler
	r    *gin.Engine
	dir  string
	exec *countingExec
	// presetBody is an async apply request for a preset that is in the hub cache.
	presetBody []byte
	cacheKey   string
}

func newBusyFixture(t *testing.T) *busyFixture {
	t.Helper()
	t.Setenv("FEATURE_CERBERUS_ENABLED", "true")
	t.Setenv("FEATURE_CROWDSEC_CONSOLE_ENROLLMENT", "true")
	db := OpenTestDB(t)
	sqlDB, dbErr := db.DB()
	require.NoError(t, dbErr)
	sqlDB.SetMaxOpenConns(1) // shared-cache SQLite answers concurrent connections with "table is locked"
	require.NoError(t, db.AutoMigrate(&models.SecurityConfig{}, &models.CrowdSecWhitelist{}, &models.CrowdsecConsoleEnrollment{}, &models.CrowdsecApplyJob{}, &models.Setting{}))
	dir := t.TempDir()
	ce := &countingExec{running: true}
	h := newTestCrowdsecHandler(t, db, ce, "/bin/false", dir)
	h.WhitelistSvc = services.NewCrowdSecWhitelistService(db, dir)
	h.Console = crowdsec.NewConsoleEnrollmentService(db, &mockEnvExecutor{}, dir, "test-secret")
	cache, err := crowdsec.NewHubCache(filepath.Join(dir, "hub_cache"), time.Hour)
	require.NoError(t, err)
	meta, err := cache.Store(context.Background(), "crowdsecurity/sshd", "collections", "1.0", "hub", []byte("name: crowdsecurity/sshd\n"))
	require.NoError(t, err)
	body, err := json.Marshal(map[string]any{"slug": "crowdsecurity/sshd", "type": "collections", "cache_key": meta.CacheKey, "async": true})
	require.NoError(t, err)
	h.Hub = crowdsec.NewHubService(&fastCmdExec{}, cache, dir)
	r := gin.New()
	h.RegisterRoutes(r.Group("/api/v1"))
	return &busyFixture{h: h, r: r, dir: dir, exec: ce, presetBody: body, cacheKey: meta.CacheKey}
}

func (f *busyFixture) do(method, path string, body []byte, contentType string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, "/api/v1"+path, bytes.NewReader(body))
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	w := httptest.NewRecorder()
	f.r.ServeHTTP(w, req)
	return w
}

// busyCase is one busy-checked route: a request that would be accepted without a job and the proof that it
// changed nothing while the job runs.
type busyCase struct {
	request func(t *testing.T, f *busyFixture) (method, path string, body []byte, contentType string)
	// untouched asserts the request had no effect (nil when the 409 itself is the whole proof).
	untouched func(t *testing.T, f *busyFixture)
}

// busyCases has one entry per busy-checked route of mutatingCrowdsecRoutes.
//
// The routes classed routeHeld take a hold on the install manager while they run (Start, Stop, ConsoleEnroll,
// whitelist add/delete and hub items restore), so a job cannot start in the middle of them. Their hold-side
// checks live in crowdsec_held_routes_test.go. Every row here is refused with 409 while a job runs.
var busyCases = map[string]busyCase{
	"POST /admin/crowdsec/start": {
		request: func(*testing.T, *busyFixture) (string, string, []byte, string) {
			return http.MethodPost, "/admin/crowdsec/start", nil, ""
		},
		untouched: func(t *testing.T, f *busyFixture) {
			require.Zero(t, f.exec.startCalls)
			require.Zero(t, f.exec.statusCalls, "the busy check runs before the executor, so a 409 never waits on its mutex")
		},
	},
	"POST /admin/crowdsec/stop": {
		request: func(*testing.T, *busyFixture) (string, string, []byte, string) {
			return http.MethodPost, "/admin/crowdsec/stop", nil, ""
		},
		untouched: func(t *testing.T, f *busyFixture) {
			require.Zero(t, f.exec.statusCalls, "the busy check runs before the executor")
		},
	},
	"POST /admin/crowdsec/import": {
		request: func(t *testing.T, _ *busyFixture) (string, string, []byte, string) {
			req := importRequest(t, map[string]string{"config.yaml": importableConfig})
			body := new(bytes.Buffer)
			_, _ = body.ReadFrom(req.Body)
			return http.MethodPost, "/admin/crowdsec/import", body.Bytes(), req.Header.Get("Content-Type")
		},
		untouched: func(t *testing.T, f *busyFixture) {
			require.NoFileExists(t, filepath.Join(f.dir, "config.yaml"), "the import extracted nothing")
		},
	},
	"POST /admin/crowdsec/hub-items/restore": {
		request: func(*testing.T, *busyFixture) (string, string, []byte, string) {
			return http.MethodPost, "/admin/crowdsec/hub-items/restore",
				restoreBody(crowdsec.HubItem{Type: "scenarios", Name: "a/one"}), "application/json"
		},
		untouched: func(t *testing.T, f *busyFixture) {
			require.False(t, f.h.restore.running.Load(), "the restore flag was never taken")
		},
	},
	"POST /admin/crowdsec/file": {
		request: func(*testing.T, *busyFixture) (string, string, []byte, string) {
			return http.MethodPost, "/admin/crowdsec/file",
				[]byte(`{"path":"busy.txt","content":"x"}`), "application/json"
		},
		untouched: func(t *testing.T, f *busyFixture) {
			require.NoFileExists(t, filepath.Join(f.dir, "busy.txt"))
		},
	},
	"POST /admin/crowdsec/presets/apply": {
		request: func(*testing.T, *busyFixture) (string, string, []byte, string) {
			curated := curatedPresetSlug()
			return http.MethodPost, "/admin/crowdsec/presets/apply",
				[]byte(`{"slug":"` + curated + `","cache_key":"curated-` + curated + `"}`), "application/json"
		},
	},
	"POST /admin/crowdsec/console/enroll": {
		request: func(*testing.T, *busyFixture) (string, string, []byte, string) {
			return http.MethodPost, "/admin/crowdsec/console/enroll",
				[]byte(`{"enrollment_key":"abcdefghijklmnop","agent_name":"agent"}`), "application/json"
		},
		untouched: func(t *testing.T, f *busyFixture) {
			var n int64
			require.NoError(t, f.h.DB.Model(&models.CrowdsecConsoleEnrollment{}).Count(&n).Error)
			require.Zero(t, n, "no enrollment attempt was recorded")
		},
	},
	"POST /admin/crowdsec/whitelist": {
		request: func(*testing.T, *busyFixture) (string, string, []byte, string) {
			return http.MethodPost, "/admin/crowdsec/whitelist", []byte(`{"ip_or_cidr":"192.0.2.10"}`), "application/json"
		},
		untouched: func(t *testing.T, f *busyFixture) {
			var n int64
			require.NoError(t, f.h.DB.Model(&models.CrowdSecWhitelist{}).Count(&n).Error)
			require.Zero(t, n, "the entry was not stored")
			require.NoFileExists(t, filepath.Join(f.dir, "config", "parsers", "s02-enrich", "charon-whitelist.yaml"))
		},
	},
	"DELETE /admin/crowdsec/whitelist/:uuid": {
		request: func(*testing.T, *busyFixture) (string, string, []byte, string) {
			return http.MethodDelete, "/admin/crowdsec/whitelist/00000000-0000-4000-8000-0000000000aa", nil, ""
		},
	},
}

func curatedPresetSlug() string {
	for _, p := range crowdsec.ListCuratedPresets() {
		if !p.RequiresHub {
			return p.Slug
		}
	}
	return ""
}

// TestBusyCasesCoverEveryBusyCheckedRoute runs today: the busy table and the route classification cannot drift.
func TestBusyCasesCoverEveryBusyCheckedRoute(t *testing.T) {
	t.Parallel()
	for route, c := range mutatingCrowdsecRoutes {
		if c.class == routeBusyChecked || c.class == routeHeld {
			require.Contains(t, busyCases, route, "a busy-checked route needs a case in busyCases")
		} else {
			require.NotContains(t, busyCases, route, "only busy-checked routes belong in busyCases")
		}
	}
}

func TestMutatingHandlersAnswerBusyWhileAnInstallRuns(t *testing.T) {
	for route, bc := range busyCases {
		t.Run(route, func(t *testing.T) {
			f := newBusyFixture(t)
			setBackgroundInstallActive(t, f.h)
			method, path, body, ct := bc.request(t, f)

			w := f.do(method, path, body, ct)

			require.Equal(t, http.StatusConflict, w.Code, w.Body.String())
			var resp map[string]any
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
			require.Equal(t, codeCrowdsecBusy, resp["error_code"])
			require.Equal(t, busyJobID, resp["job_id"], "the active job is named so the UI can resume it")
			require.Equal(t, msgCrowdsecBusy, resp["error"])
			if bc.untouched != nil {
				bc.untouched(t, f)
			}
		})
	}
}

func TestDuplicateStartAnswersBusyNotAlreadyRunningWhileAnInstallRuns(t *testing.T) {
	f := newBusyFixture(t)

	// Without a job a second Start is the idempotent 200 (runs today).
	w := f.do(http.MethodPost, "/admin/crowdsec/start", nil, "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.Contains(t, w.Body.String(), "already_running")

	setBackgroundInstallActive(t, f.h)
	f.exec.statusCalls = 0
	w = f.do(http.MethodPost, "/admin/crowdsec/start", nil, "")
	require.Equal(t, http.StatusConflict, w.Code, "a reload during an install could interrupt it")
	require.Zero(t, f.exec.statusCalls)
}

func TestReadsStayOpenWhileAnInstallRuns(t *testing.T) {
	f := newBusyFixture(t)
	require.NoError(t, os.WriteFile(filepath.Join(f.dir, "readme.txt"), []byte("x"), 0o600))
	setBackgroundInstallActive(t, f.h)

	for _, path := range []string{
		"/admin/crowdsec/status",
		"/admin/crowdsec/files",
		"/admin/crowdsec/file?path=readme.txt",
		"/admin/crowdsec/export",
		"/admin/crowdsec/presets",
		"/admin/crowdsec/whitelist",
		"/admin/crowdsec/console/status",
		"/admin/crowdsec/acquisition",
	} {
		w := f.do(http.MethodGet, path, nil, "")
		require.NotEqual(t, http.StatusConflict, w.Code, "%s must stay available: %s", path, w.Body.String())
		require.False(t, strings.Contains(w.Body.String(), codeCrowdsecBusy), path)
	}
}

func TestNonBusyRoutesStayOpenWhileAnInstallRuns(t *testing.T) {
	f := newBusyFixture(t)
	setBackgroundInstallActive(t, f.h)

	for route, c := range mutatingCrowdsecRoutes {
		if c.class == routeBusyChecked || c.class == routeHeld {
			continue
		}
		method, path, _ := strings.Cut(route, " ")
		path = strings.NewReplacer("*value", "192.0.2.1", ":uuid", "x", ":id", "00000000-0000-4000-8000-000000000002").Replace(path)
		w := f.do(method, path, []byte(`{}`), "application/json")
		require.NotEqual(t, http.StatusConflict, w.Code, "%s (%s) must not be blocked: %s", route, c.class, w.Body.String())
	}
}

func TestHubItemsRestoreInProgressRefusesAnInstallJob(t *testing.T) {
	f := newBusyFixture(t)
	release, _, ok := f.h.ApplyJobs().Hold() // a restore holds the manager for its whole run
	require.True(t, ok)
	defer release()

	w := f.do(http.MethodPost, "/admin/crowdsec/presets/apply",
		f.presetBody, "application/json")

	require.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	var resp map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.Equal(t, codeCrowdsecBusy, resp["error_code"])
	require.NotContains(t, resp, "job_id", "no job exists, so none is named")
}

// endBackgroundInstall makes h behave as if the running job finished.
func endBackgroundInstall(t *testing.T, h *CrowdsecHandler) {
	t.Helper()
	h.ApplyJobs().Release(busyJobID)
}

func TestBusyFlagIsReleasedWhenAJobEnds(t *testing.T) {
	f := newBusyFixture(t)
	setBackgroundInstallActive(t, f.h)
	write := []byte(`{"path":"a.txt","content":"x"}`)
	require.Equal(t, http.StatusConflict, f.do(http.MethodPost, "/admin/crowdsec/file", write, "application/json").Code)

	endBackgroundInstall(t, f.h)

	require.Equal(t, http.StatusOK, f.do(http.MethodPost, "/admin/crowdsec/file", write, "application/json").Code, "writes work again after the job ended")
}

func TestHubItemsRestoreRunningBlocksTheBusyFlag(t *testing.T) {
	f := newBusyFixture(t)
	release, _, ok := f.h.ApplyJobs().Hold()
	require.True(t, ok)
	require.False(t, f.h.ApplyJobs().TryAcquire(busyJobID), "no job may start while a restore holds the manager")
	release()
	require.True(t, f.h.ApplyJobs().TryAcquire(busyJobID))
	f.h.ApplyJobs().Release(busyJobID)
}

// TestConsoleEnrollRefusedWhileARealInstallRuns drives the console guard with a real async apply job (job row and
// manager), not the stubbed busy flag used by the table above.
func TestConsoleEnrollRefusedWhileARealInstallRuns(t *testing.T) {
	f := newBusyFixture(t)
	gate, release := newJobGate(t)
	useJobApply(t, f, func(context.Context, crowdsec.ApplyRequest) (crowdsec.ApplyResult, error) {
		<-gate
		return crowdsec.ApplyResult{Status: "applied"}, nil
	})
	enrollBody := []byte(`{"enrollment_key":"abcdefghijklmnop","agent_name":"agent"}`)
	id := decodeBody(t, f.do(http.MethodPost, "/admin/crowdsec/presets/apply", f.presetBody, "application/json"))["job_id"].(string)
	waitJobState(t, f, id, models.CrowdsecApplyJobRunning)

	w := f.do(http.MethodPost, "/admin/crowdsec/console/enroll", enrollBody, "application/json")
	require.Equal(t, http.StatusConflict, w.Code, w.Body.String())
	resp := decodeBody(t, w)
	require.Equal(t, codeCrowdsecBusy, resp["error_code"])
	require.Equal(t, id, resp["job_id"], "the running job is named so the UI can resume it")
	var n int64
	require.NoError(t, f.h.DB.Model(&models.CrowdsecConsoleEnrollment{}).Count(&n).Error)
	require.Zero(t, n, "no enrollment attempt was recorded")

	release()
	waitJobState(t, f, id, models.CrowdsecApplyJobSucceeded)
	require.Eventually(t, func() bool { _, busy := f.h.ApplyJobs().Busy(); return !busy }, 5*time.Second, 10*time.Millisecond)

	w = f.do(http.MethodPost, "/admin/crowdsec/console/enroll", enrollBody, "application/json")
	require.NotEqual(t, http.StatusConflict, w.Code, "enrolment is allowed again once the job ended: %s", w.Body.String())
	require.NotContains(t, w.Body.String(), codeCrowdsecBusy)
}

// TestConsoleEnrollCheapGatesWinOverBusy pins that a disabled feature or a malformed payload is answered before the
// busy check, so it is never reported as busy while an install runs.
func TestConsoleEnrollCheapGatesWinOverBusy(t *testing.T) {
	for name, tc := range map[string]struct {
		flag string
		body string
		want int
	}{
		"feature disabled": {flag: "false", body: `{"enrollment_key":"abcdefghijklmnop","agent_name":"agent"}`, want: http.StatusNotFound},
		"invalid payload":  {flag: "true", body: `{`, want: http.StatusBadRequest},
	} {
		t.Run(name, func(t *testing.T) {
			f := newBusyFixture(t)
			t.Setenv("FEATURE_CROWDSEC_CONSOLE_ENROLLMENT", tc.flag)
			setBackgroundInstallActive(t, f.h)

			w := f.do(http.MethodPost, "/admin/crowdsec/console/enroll", []byte(tc.body), "application/json")

			require.Equal(t, tc.want, w.Code, w.Body.String())
			require.NotContains(t, w.Body.String(), codeCrowdsecBusy)
		})
	}
}
