package handlers

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/dbmaint"
	"github.com/Wikid82/charon/backend/internal/models"
)

const maintTestPageSize = 4096

// maintRig is a handler over a real scratch database file with the settings
// table, whose Inspect and disk seams can be replaced by fixed figures.
type maintRig struct {
	h      *DatabaseMaintenanceHandler
	db     *sql.DB
	path   string
	router *gin.Engine
	stats  dbmaint.Stats
	disk   dbmaint.DiskReport
}

func newMaintRig(t *testing.T, envMode string) *maintRig {
	t.Helper()
	gin.SetMode(gin.TestMode)
	path := filepath.Join(t.TempDir(), "charon.db")
	gdb, err := gorm.Open(sqlite.Open(path), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, gdb.AutoMigrate(&models.Setting{}))
	db, err := gdb.DB()
	require.NoError(t, err)
	db.SetMaxOpenConns(1)
	t.Cleanup(func() { _ = db.Close() })

	r := &maintRig{db: db, path: path, h: NewDatabaseMaintenanceHandler(db, path, envMode)}
	// Fixed figures unless a test asks for the real file: a mode-0 database with
	// 500 MB reclaimable (well above the floor and the free ratio).
	r.stats = maintStats(200000, 122070, dbmaint.AutoVacuumNone)
	r.h.inspect = func(context.Context) (dbmaint.Stats, error) { return r.stats, nil }
	r.h.diskReport = func(dbmaint.Stats) (dbmaint.DiskReport, error) { return r.disk, nil }
	r.disk = dbmaint.DiskReport{DataAvailable: 50 << 30, TmpAvailable: 50 << 30, SameFS: true}

	r.router = gin.New()
	r.router.GET("/system/database", r.h.GetStatus)
	r.router.POST("/system/database/optimize-on-restart", r.h.RequestOptimize)
	r.router.DELETE("/system/database/optimize-on-restart", r.h.CancelOptimize)
	return r
}

func maintStats(pages, free int64, autoVacuum int) dbmaint.Stats {
	return dbmaint.Stats{
		PageSize: maintTestPageSize, PageCount: pages, FreelistCount: free, AutoVacuum: autoVacuum,
		MainBytes: pages * maintTestPageSize, WALBytes: 4 << 20,
	}
}

func (r *maintRig) do(t *testing.T, method, path string) (code int, body map[string]any) {
	t.Helper()
	w := httptest.NewRecorder()
	r.router.ServeHTTP(w, httptest.NewRequest(method, path, http.NoBody))
	body = map[string]any{}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body), w.Body.String())
	return w.Code, body
}

func (r *maintRig) status(t *testing.T) map[string]any {
	t.Helper()
	code, body := r.do(t, http.MethodGet, "/system/database")
	require.Equal(t, http.StatusOK, code, body)
	return body
}

const (
	statusPath   = "/system/database"
	optimizePath = "/system/database/optimize-on-restart"
)

func (r *maintRig) fileID(t *testing.T) string {
	t.Helper()
	id, err := dbmaint.FileID(r.path)
	require.NoError(t, err)
	return id
}

// recordFailure records one failed conversion attempt for the scratch database.
func (r *maintRig) recordFailure(t *testing.T) {
	t.Helper()
	_, err := dbmaint.NewStore(r.db).RecordFailure(context.Background(), r.fileID(t))
	require.NoError(t, err)
}

func (r *maintRig) writeLast(t *testing.T, outcome dbmaint.Result, reason dbmaint.Reason) {
	t.Helper()
	require.NoError(t, dbmaint.NewStore(r.db).WriteLastResult(context.Background(), dbmaint.LastResult{
		At: time.Now(), Outcome: outcome, Reason: reason, FileID: r.fileID(t),
	}))
}

func noticeOf(t *testing.T, body map[string]any) map[string]any {
	t.Helper()
	n, ok := body["notice"].(map[string]any)
	if !ok {
		require.Nil(t, body["notice"], "notice is an object or null")
		return nil
	}
	return n
}

func TestDatabaseMaintenance_StatusContractOnARealFile(t *testing.T) {
	r := newMaintRig(t, config.DBCompactAuto)
	// Undo the fixed figures: read the real scratch file and disk.
	r.h = NewDatabaseMaintenanceHandler(r.db, r.path, config.DBCompactAuto)
	r.router = gin.New()
	r.router.GET(statusPath, r.h.GetStatus)

	body := r.status(t)

	for _, key := range []string{
		"size_bytes", "wal_bytes", "reclaimable_bytes", "auto_vacuum", "env_mode",
		"compact_requested", "can_request_optimize", "disk_free_bytes", "last_result", "notice",
	} {
		assert.Contains(t, body, key)
	}
	assert.Equal(t, "none", body["auto_vacuum"], "a file created without the pragma is legacy mode")
	assert.Equal(t, "auto", body["env_mode"])
	assert.Equal(t, false, body["compact_requested"])
	assert.Equal(t, false, body["can_request_optimize"], "a tiny file is below the reclaim floor")
	assert.Positive(t, body["size_bytes"])
	assert.Positive(t, body["disk_free_bytes"])
	assert.Nil(t, body["last_result"])
	assert.Nil(t, body["notice"])
}

func TestDatabaseMaintenance_StatusReportsFiguresAndLastResult(t *testing.T) {
	r := newMaintRig(t, config.DBCompactAuto)
	r.stats = maintStats(1000, 100, dbmaint.AutoVacuumIncremental)
	at := time.Date(2026, 10, 2, 4, 11, 0, 0, time.UTC)
	require.NoError(t, dbmaint.NewStore(r.db).WriteLastResult(context.Background(), dbmaint.LastResult{
		At: at, Outcome: dbmaint.ResultConverted, BytesBefore: 4800, BytesAfter: 2900, FileID: r.fileID(t),
	}))

	body := r.status(t)

	assert.EqualValues(t, 1000*maintTestPageSize, body["size_bytes"])
	assert.EqualValues(t, 4<<20, body["wal_bytes"])
	assert.EqualValues(t, 100*maintTestPageSize, body["reclaimable_bytes"])
	assert.Equal(t, "incremental", body["auto_vacuum"])
	assert.EqualValues(t, int64(50<<30), body["disk_free_bytes"])
	last, ok := body["last_result"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, "converted", last["outcome"])
	assert.Equal(t, "", last["reason"])
	assert.EqualValues(t, 4800, last["bytes_before"])
	assert.EqualValues(t, 2900, last["bytes_after"])
	assert.Equal(t, at.Format(time.RFC3339), last["at"])
}

func TestDatabaseMaintenance_AutoVacuumNames(t *testing.T) {
	assert.Equal(t, "none", autoVacuumName(0))
	assert.Equal(t, "full", autoVacuumName(1))
	assert.Equal(t, "incremental", autoVacuumName(2))
}

func TestDatabaseMaintenance_CanRequestOptimize(t *testing.T) {
	cases := []struct {
		name  string
		env   string
		stats dbmaint.Stats
		want  bool
	}{
		{"legacy file with enough to reclaim", config.DBCompactAuto, maintStats(200000, 122070, 0), true},
		{"exactly the floor", config.DBCompactAuto, maintStats(200000, 25600, 0), true},
		{"one page below the floor", config.DBCompactAuto, maintStats(200000, 25599, 0), false},
		{"already incremental", config.DBCompactAuto, maintStats(200000, 122070, 2), false},
		{"environment off", config.DBCompactOff, maintStats(200000, 122070, 0), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newMaintRig(t, tc.env)
			r.stats = tc.stats
			assert.Equal(t, tc.want, r.status(t)["can_request_optimize"])
		})
	}
}

func TestDatabaseMaintenance_NoticeSeverityAndPayloadPerCode(t *testing.T) {
	short := dbmaint.DiskReport{DataRequired: 10 << 30, DataAvailable: 1 << 30, TmpRequired: 1, TmpAvailable: 1 << 40, SameFS: false}
	cases := []struct {
		name     string
		env      string
		stats    dbmaint.Stats
		disk     *dbmaint.DiskReport
		prepare  func(t *testing.T, r *maintRig)
		wantCode string
		wantSev  string
	}{
		{
			name: "restart_to_optimize is info", env: config.DBCompactAuto,
			stats: maintStats(200000, 122070, 0), wantCode: "restart_to_optimize", wantSev: "info",
		},
		{
			name: "insufficient_disk is a warning", env: config.DBCompactAuto,
			stats: maintStats(200000, 122070, 0), disk: &short, wantCode: "insufficient_disk", wantSev: "warning",
		},
		{
			name: "too_many_failures is a warning", env: config.DBCompactAuto,
			stats: maintStats(200000, 122070, 0), wantCode: "too_many_failures", wantSev: "warning",
			prepare: func(t *testing.T, r *maintRig) {
				for range dbmaint.MaxConvertAttempts {
					r.recordFailure(t)
				}
			},
		},
		{
			name: "too_many_failures with a pending request is a warning", env: config.DBCompactAuto,
			stats: maintStats(200000, 122070, 0), wantCode: "too_many_failures", wantSev: "warning",
			prepare: func(t *testing.T, r *maintRig) {
				require.NoError(t, dbmaint.NewStore(r.db).SetFlag(context.Background()))
				for range dbmaint.MaxConvertAttempts {
					r.recordFailure(t)
				}
			},
		},
		{
			name: "database_busy is info", env: config.DBCompactAuto,
			stats: maintStats(200000, 122070, 0), wantCode: "database_busy", wantSev: "info",
			prepare: func(t *testing.T, r *maintRig) { r.writeLast(t, dbmaint.ResultSkipped, dbmaint.ReasonDatabaseBusy) },
		},
		{
			name: "disabled_by_env is info", env: config.DBCompactOff,
			stats: maintStats(200000, 122070, 0), wantCode: "disabled_by_env", wantSev: "info",
			prepare: func(t *testing.T, r *maintRig) {
				require.NoError(t, dbmaint.NewStore(r.db).SetFlag(context.Background()))
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newMaintRig(t, tc.env)
			r.stats = tc.stats
			if tc.disk != nil {
				r.disk = *tc.disk
			}
			if tc.prepare != nil {
				tc.prepare(t, r)
			}

			n := noticeOf(t, r.status(t))

			require.NotNil(t, n)
			assert.Equal(t, tc.wantCode, n["code"])
			assert.Equal(t, tc.wantSev, n["severity"])
			assert.EqualValues(t, tc.stats.ReclaimableBytes(), n["reclaimable_bytes"])
			if tc.wantCode == "insufficient_disk" {
				assert.EqualValues(t, int64(10<<30), n["required_bytes"])
				assert.EqualValues(t, int64(1<<30), n["available_bytes"])
			} else {
				assert.NotContains(t, n, "required_bytes")
				assert.NotContains(t, n, "available_bytes")
			}
		})
	}
}

func TestDatabaseMaintenance_NoNoticeWhenThereIsNothingToSay(t *testing.T) {
	cases := map[string]dbmaint.Stats{
		"healthy small file":  maintStats(1000, 5, 0),
		"below the floor":     maintStats(200000, 25599, 0),
		"already incremental": maintStats(200000, 122070, 2),
	}
	for name, stats := range cases {
		t.Run(name, func(t *testing.T) {
			r := newMaintRig(t, config.DBCompactAuto)
			r.stats = stats
			assert.Nil(t, noticeOf(t, r.status(t)))
		})
	}
}

func TestDatabaseMaintenance_TerminalSkipSilencesThePendingLine(t *testing.T) {
	for _, reason := range []dbmaint.Reason{dbmaint.ReasonIntegrityCheckFailed, dbmaint.ReasonTooManyFailures} {
		t.Run(string(reason), func(t *testing.T) {
			r := newMaintRig(t, config.DBCompactAuto)
			r.writeLast(t, dbmaint.ResultSkipped, reason)
			assert.Nil(t, noticeOf(t, r.status(t)), "do not promise a conversion the next boot refuses")
		})
	}

	t.Run("a transient skip does not", func(t *testing.T) {
		r := newMaintRig(t, config.DBCompactAuto)
		r.writeLast(t, dbmaint.ResultSkipped, dbmaint.ReasonCaddyNotReady)
		assert.Equal(t, "restart_to_optimize", noticeOf(t, r.status(t))["code"])
	})

	t.Run("a result of another file does not", func(t *testing.T) {
		r := newMaintRig(t, config.DBCompactAuto)
		require.NoError(t, dbmaint.NewStore(r.db).WriteLastResult(context.Background(), dbmaint.LastResult{
			At: time.Now(), Outcome: dbmaint.ResultSkipped, Reason: dbmaint.ReasonIntegrityCheckFailed, FileID: "1",
		}))
		assert.Equal(t, "restart_to_optimize", noticeOf(t, r.status(t))["code"])
		assert.Nil(t, r.status(t)["last_result"], "and it is not shown")
	})
}

func TestDatabaseMaintenance_NoticeIsComputedFreshOnEveryCall(t *testing.T) {
	r := newMaintRig(t, config.DBCompactAuto)
	r.stats = maintStats(200000, 1000, 0) // the pruner has not freed anything yet
	assert.Nil(t, noticeOf(t, r.status(t)))

	r.stats = maintStats(200000, 122070, 0) // a prune pass freed pages, no restart in between
	assert.Equal(t, "restart_to_optimize", noticeOf(t, r.status(t))["code"])

	r.stats = maintStats(200000, 1000, 0)
	assert.Nil(t, noticeOf(t, r.status(t)))
}

func TestDatabaseMaintenance_StatusDoesNotConsumeTheBootState(t *testing.T) {
	r := newMaintRig(t, config.DBCompactAuto)
	store := dbmaint.NewStore(r.db)
	require.NoError(t, store.SetInProgress(context.Background(), r.fileID(t), time.Now()))

	r.status(t)

	st, err := store.Load(context.Background(), r.fileID(t))
	require.NoError(t, err)
	assert.Equal(t, 1, st.Attempts, "the boot path still sees the leftover marker")
}

func TestDatabaseMaintenance_StatusFailuresAre500(t *testing.T) {
	t.Run("inspect", func(t *testing.T) {
		r := newMaintRig(t, config.DBCompactAuto)
		r.h.inspect = func(context.Context) (dbmaint.Stats, error) { return dbmaint.Stats{}, errors.New("boom") }
		code, body := r.do(t, http.MethodGet, statusPath)
		assert.Equal(t, http.StatusInternalServerError, code)
		assert.NotContains(t, body["error"], "boom", "internals are not leaked")
	})
	t.Run("file identity", func(t *testing.T) {
		r := newMaintRig(t, config.DBCompactAuto)
		r.h.dbPath = filepath.Join(t.TempDir(), "missing.db")
		code, _ := r.do(t, http.MethodGet, statusPath)
		assert.Equal(t, http.StatusInternalServerError, code)
	})
	t.Run("store", func(t *testing.T) {
		r := newMaintRig(t, config.DBCompactAuto)
		require.NoError(t, r.db.Close())
		code, _ := r.do(t, http.MethodGet, statusPath)
		assert.Equal(t, http.StatusInternalServerError, code)
	})
}

func TestDatabaseMaintenance_DiskFailureStillRendersTheCard(t *testing.T) {
	r := newMaintRig(t, config.DBCompactAuto)
	r.h.diskReport = func(dbmaint.Stats) (dbmaint.DiskReport, error) { return dbmaint.DiskReport{}, errors.New("statfs") }

	body := r.status(t)

	assert.EqualValues(t, 0, body["disk_free_bytes"])
	assert.NotNil(t, body["size_bytes"])
}

func TestDatabaseMaintenance_PostSetsTheFlagAndIsIdempotent(t *testing.T) {
	r := newMaintRig(t, config.DBCompactAuto)

	for i := range 3 {
		code, body := r.do(t, http.MethodPost, optimizePath)
		require.Equal(t, http.StatusOK, code, "request %d", i)
		assert.Equal(t, true, body["requested"])
	}
	assert.Equal(t, true, r.status(t)["compact_requested"])
}

func TestDatabaseMaintenance_PostResetsTheFailureCounterOnlyWhenNewlySet(t *testing.T) {
	r := newMaintRig(t, config.DBCompactAuto)
	store := dbmaint.NewStore(r.db)
	ctx := context.Background()
	for range dbmaint.MaxConvertAttempts {
		r.recordFailure(t)
	}

	code, _ := r.do(t, http.MethodPost, optimizePath)
	require.Equal(t, http.StatusOK, code)
	st, err := store.Peek(ctx, r.fileID(t))
	require.NoError(t, err)
	assert.Zero(t, st.Attempts, "pressing the button starts over")

	r.recordFailure(t)
	code, _ = r.do(t, http.MethodPost, optimizePath)
	require.Equal(t, http.StatusOK, code)
	st, err = store.Peek(ctx, r.fileID(t))
	require.NoError(t, err)
	assert.Equal(t, 1, st.Attempts, "a repeated request changes nothing")
}

func TestDatabaseMaintenance_Post409BodiesDifferByCause(t *testing.T) {
	t.Run("already optimized", func(t *testing.T) {
		r := newMaintRig(t, config.DBCompactAuto)
		r.stats = maintStats(200000, 122070, dbmaint.AutoVacuumIncremental)
		code, body := r.do(t, http.MethodPost, optimizePath)
		assert.Equal(t, http.StatusConflict, code)
		assert.Equal(t, "nothing_to_optimize", body["code"])
		assert.Equal(t, "the database is already optimized or has too little reclaimable space", body["error"])
	})
	t.Run("too little reclaimable", func(t *testing.T) {
		r := newMaintRig(t, config.DBCompactAuto)
		r.stats = maintStats(200000, 25599, 0)
		code, body := r.do(t, http.MethodPost, optimizePath)
		assert.Equal(t, http.StatusConflict, code)
		assert.Equal(t, "nothing_to_optimize", body["code"])
	})
	t.Run("environment off", func(t *testing.T) {
		r := newMaintRig(t, config.DBCompactOff)
		code, body := r.do(t, http.MethodPost, optimizePath)
		assert.Equal(t, http.StatusConflict, code)
		assert.Equal(t, "disabled_by_env", body["code"])
		assert.Equal(t, "database optimization is disabled by CHARON_DB_COMPACT_ON_START=off", body["error"])
	})
	t.Run("environment off wins over a mode-2 file", func(t *testing.T) {
		r := newMaintRig(t, config.DBCompactOff)
		r.stats = maintStats(200000, 122070, dbmaint.AutoVacuumIncremental)
		_, body := r.do(t, http.MethodPost, optimizePath)
		assert.Equal(t, "disabled_by_env", body["code"], "the same order Decide uses")
	})
	t.Run("a refused request leaves no flag", func(t *testing.T) {
		r := newMaintRig(t, config.DBCompactOff)
		r.do(t, http.MethodPost, optimizePath)
		assert.Equal(t, false, r.status(t)["compact_requested"])
	})
}

func TestDatabaseMaintenance_RepeatedPostIs200EvenIfTheEnvironmentTurnedOffLater(t *testing.T) {
	r := newMaintRig(t, config.DBCompactAuto)
	code, _ := r.do(t, http.MethodPost, optimizePath)
	require.Equal(t, http.StatusOK, code)

	r.h.envMode = config.DBCompactOff
	r.stats = maintStats(200000, 10, 0) // and it no longer qualifies either

	code, body := r.do(t, http.MethodPost, optimizePath)
	assert.Equal(t, http.StatusOK, code)
	assert.Equal(t, true, body["requested"])
}

func TestDatabaseMaintenance_DeleteClearsTheFlagAndIsIdempotent(t *testing.T) {
	r := newMaintRig(t, config.DBCompactAuto)
	r.do(t, http.MethodPost, optimizePath)

	for range 2 {
		code, body := r.do(t, http.MethodDelete, optimizePath)
		require.Equal(t, http.StatusOK, code)
		assert.Equal(t, false, body["requested"])
	}
	assert.Equal(t, false, r.status(t)["compact_requested"])
}

func TestDatabaseMaintenance_WriteFailuresAre500(t *testing.T) {
	t.Run("post reading the flag", func(t *testing.T) {
		r := newMaintRig(t, config.DBCompactAuto)
		require.NoError(t, r.db.Close())
		code, _ := r.do(t, http.MethodPost, optimizePath)
		assert.Equal(t, http.StatusInternalServerError, code)
	})
	t.Run("post inspecting", func(t *testing.T) {
		r := newMaintRig(t, config.DBCompactAuto)
		r.h.inspect = func(context.Context) (dbmaint.Stats, error) { return dbmaint.Stats{}, errors.New("boom") }
		code, _ := r.do(t, http.MethodPost, optimizePath)
		assert.Equal(t, http.StatusInternalServerError, code)
	})
	t.Run("post writing", func(t *testing.T) {
		r := newMaintRig(t, config.DBCompactAuto)
		_, err := r.db.Exec(`CREATE TRIGGER deny_insert BEFORE INSERT ON settings BEGIN SELECT RAISE(ABORT, 'denied'); END`)
		require.NoError(t, err)
		code, _ := r.do(t, http.MethodPost, optimizePath)
		assert.Equal(t, http.StatusInternalServerError, code)
	})
	t.Run("post resetting the counter", func(t *testing.T) {
		r := newMaintRig(t, config.DBCompactAuto)
		r.recordFailure(t)
		_, err := r.db.Exec(`CREATE TRIGGER deny_delete BEFORE DELETE ON settings BEGIN SELECT RAISE(ABORT, 'denied'); END`)
		require.NoError(t, err)
		code, _ := r.do(t, http.MethodPost, optimizePath)
		assert.Equal(t, http.StatusInternalServerError, code)
	})
	t.Run("delete", func(t *testing.T) {
		r := newMaintRig(t, config.DBCompactAuto)
		require.NoError(t, r.db.Close())
		code, _ := r.do(t, http.MethodDelete, optimizePath)
		assert.Equal(t, http.StatusInternalServerError, code)
	})
}

func TestDatabaseMaintenance_FlagIsInvisibleToTheSettingsAPI(t *testing.T) {
	r := newMaintRig(t, config.DBCompactAuto)
	r.do(t, http.MethodPost, optimizePath)
	gdb, err := gorm.Open(sqlite.Dialector{Conn: r.db}, &gorm.Config{})
	require.NoError(t, err)
	settings := NewSettingsHandler(gdb)
	r.router.GET("/settings", settings.GetSettings)

	w := httptest.NewRecorder()
	r.router.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/settings", http.NoBody))

	require.Equal(t, http.StatusOK, w.Code)
	var all map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &all))
	assert.NotContains(t, all, dbmaint.SettingKeyFlag)
}

// failNthRead makes the nth read of the handler's store fail by sending an
// invalid statement in its place.
type failNthRead struct {
	dbmaint.SQLExecer
	n, calls int
}

func (f *failNthRead) QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row {
	f.calls++
	if f.calls == f.n {
		query = "SELECT value FROM no_such_table WHERE 1 = ?"
		args = []any{1}
	}
	return f.SQLExecer.QueryRowContext(ctx, query, args...)
}

// attemptsOf returns the stored failure counter of the scratch database.
func (r *maintRig) attemptsOf(t *testing.T) int {
	t.Helper()
	st, err := dbmaint.NewStore(r.db).Peek(context.Background(), r.fileID(t))
	require.NoError(t, err)
	return st.Attempts
}

func (r *maintRig) setFlag(t *testing.T) {
	t.Helper()
	require.NoError(t, dbmaint.NewStore(r.db).SetFlag(context.Background()))
}

// GH #1438: a request that is still set although the back-off is exhausted
// (state left by an older version, or a stale tab) is a fresh request.
func TestDatabaseMaintenance_PostWithTheFlagSetResetsAnExhaustedCounter(t *testing.T) {
	t.Run("counter at the limit is reset and the flag stays set", func(t *testing.T) {
		r := newMaintRig(t, config.DBCompactAuto)
		r.setFlag(t)
		for range dbmaint.MaxConvertAttempts {
			r.recordFailure(t)
		}

		for i := range 2 { // the second press finds the counter at 0 and changes nothing
			code, body := r.do(t, http.MethodPost, optimizePath)
			require.Equal(t, http.StatusOK, code, "press %d", i)
			assert.Equal(t, true, body["requested"])
			assert.Zero(t, r.attemptsOf(t))
			assert.Equal(t, true, r.status(t)["compact_requested"])
		}
	})
	t.Run("counter below the limit is left alone", func(t *testing.T) {
		r := newMaintRig(t, config.DBCompactAuto)
		r.setFlag(t)
		for range dbmaint.MaxConvertAttempts - 1 {
			r.recordFailure(t)
		}

		code, _ := r.do(t, http.MethodPost, optimizePath)

		require.Equal(t, http.StatusOK, code)
		assert.Equal(t, dbmaint.MaxConvertAttempts-1, r.attemptsOf(t))
	})
	t.Run("unreadable file identity is a 500", func(t *testing.T) {
		r := newMaintRig(t, config.DBCompactAuto)
		r.setFlag(t)
		r.h.dbPath = filepath.Join(t.TempDir(), "missing.db")

		code, _ := r.do(t, http.MethodPost, optimizePath)

		assert.Equal(t, http.StatusInternalServerError, code)
	})
	t.Run("unreadable state is a 500", func(t *testing.T) {
		r := newMaintRig(t, config.DBCompactAuto)
		r.setFlag(t)
		r.h.store = dbmaint.NewStore(&failNthRead{SQLExecer: r.db, n: 2}) // 1: the request check, 2: Peek

		code, _ := r.do(t, http.MethodPost, optimizePath)

		assert.Equal(t, http.StatusInternalServerError, code)
	})
	t.Run("a counter that cannot be reset is a 500", func(t *testing.T) {
		r := newMaintRig(t, config.DBCompactAuto)
		r.setFlag(t)
		for range dbmaint.MaxConvertAttempts {
			r.recordFailure(t)
		}
		_, err := r.db.Exec(`CREATE TRIGGER deny_delete BEFORE DELETE ON settings BEGIN SELECT RAISE(ABORT, 'denied'); END`)
		require.NoError(t, err)

		code, _ := r.do(t, http.MethodPost, optimizePath)

		assert.Equal(t, http.StatusInternalServerError, code)
	})
}

// SF4: the runner drops the request when it exhausts the budget, so the stopped
// notice must follow the back-off state, not the flag. The case that matters is
// a file below the automatic thresholds (100-200 MiB reclaimable, low free
// ratio) that was only ever converted because the user asked.
func TestDatabaseMaintenance_StoppedNoticeSurvivesTheRunnerClearingTheRequest(t *testing.T) {
	belowAutomatic := maintStats(1000000, 38400, 0) // 150 MiB free of a 3.8 GB file: 3.84 %
	exhaust := func(t *testing.T, r *maintRig) {
		for range dbmaint.MaxConvertAttempts {
			r.recordFailure(t)
		}
	}
	cases := []struct {
		name       string
		env        string
		stats      dbmaint.Stats
		prepare    func(t *testing.T, r *maintRig)
		wantCode   string // "" means no notice
		wantCanReq bool
	}{
		{"below the automatic thresholds, flag unset, attempts exhausted", config.DBCompactAuto, belowAutomatic, exhaust, "too_many_failures", true},
		{"counters below the limit", config.DBCompactAuto, belowAutomatic,
			func(t *testing.T, r *maintRig) {
				for range dbmaint.MaxConvertAttempts - 1 {
					r.recordFailure(t)
				}
			}, "", true},
		{"fresh install", config.DBCompactAuto, belowAutomatic, func(*testing.T, *maintRig) {}, "", true},
		{"already incremental", config.DBCompactAuto, maintStats(1000000, 38400, dbmaint.AutoVacuumIncremental), exhaust, "", false},
		{"environment off without a flag", config.DBCompactOff, belowAutomatic, exhaust, "", false},
		{"under the 100 MiB floor", config.DBCompactAuto, maintStats(1000000, 20000, 0), exhaust, "", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := newMaintRig(t, tc.env)
			r.stats = tc.stats
			tc.prepare(t, r)

			body := r.status(t)

			n := noticeOf(t, body)
			if tc.wantCode == "" {
				assert.Nil(t, n)
			} else {
				require.NotNil(t, n)
				assert.Equal(t, tc.wantCode, n["code"])
				assert.Equal(t, "warning", n["severity"])
			}
			assert.Equal(t, tc.wantCanReq, body["can_request_optimize"])
			assert.Equal(t, false, body["compact_requested"])
		})
	}
}

func (r *maintRig) recordInterruptions(t *testing.T, n int) {
	t.Helper()
	for range n {
		_, err := dbmaint.NewStore(r.db).RecordInterruption(context.Background(), r.fileID(t))
		require.NoError(t, err)
	}
}

// GH #1436: orderly stops back off like failures, with the same notice, and the
// reclaim button resets them.
func TestDatabaseMaintenance_InterruptionsBackOffLikeFailures(t *testing.T) {
	belowAutomatic := maintStats(1000000, 38400, 0)

	t.Run("only interruptions exhausted on an automatic file", func(t *testing.T) {
		r := newMaintRig(t, config.DBCompactAuto)
		r.recordInterruptions(t, dbmaint.MaxInterruptedRuns)

		body := r.status(t)

		n := noticeOf(t, body)
		require.NotNil(t, n)
		assert.Equal(t, "too_many_failures", n["code"])
		assert.Equal(t, "warning", n["severity"])
	})
	t.Run("only interruptions exhausted below the automatic thresholds, flag unset", func(t *testing.T) {
		r := newMaintRig(t, config.DBCompactAuto)
		r.stats = belowAutomatic
		r.recordInterruptions(t, dbmaint.MaxInterruptedRuns)

		body := r.status(t)

		n := noticeOf(t, body)
		require.NotNil(t, n)
		assert.Equal(t, "too_many_failures", n["code"])
		assert.Equal(t, true, body["can_request_optimize"])
	})
	t.Run("interruptions below the limit say nothing extra", func(t *testing.T) {
		r := newMaintRig(t, config.DBCompactAuto)
		r.stats = belowAutomatic
		r.recordInterruptions(t, dbmaint.MaxInterruptedRuns-1)

		assert.Nil(t, noticeOf(t, r.status(t)))
	})
	t.Run("the dry run for restart_to_optimize ignores interruptions below the limit", func(t *testing.T) {
		r := newMaintRig(t, config.DBCompactAuto)
		r.recordInterruptions(t, dbmaint.MaxInterruptedRuns-1)

		n := noticeOf(t, r.status(t))

		require.NotNil(t, n)
		assert.Equal(t, "restart_to_optimize", n["code"])
	})
	t.Run("a request with the flag set resets exhausted interruptions", func(t *testing.T) {
		r := newMaintRig(t, config.DBCompactAuto)
		r.setFlag(t)
		r.recordInterruptions(t, dbmaint.MaxInterruptedRuns)

		code, _ := r.do(t, http.MethodPost, optimizePath)

		require.Equal(t, http.StatusOK, code)
		st, err := dbmaint.NewStore(r.db).Peek(context.Background(), r.fileID(t))
		require.NoError(t, err)
		assert.Zero(t, st.Interruptions)
		assert.Equal(t, true, r.status(t)["compact_requested"])
	})
	t.Run("a fresh request resets exhausted interruptions", func(t *testing.T) {
		r := newMaintRig(t, config.DBCompactAuto)
		r.recordInterruptions(t, dbmaint.MaxInterruptedRuns)

		code, _ := r.do(t, http.MethodPost, optimizePath)

		require.Equal(t, http.StatusOK, code)
		st, err := dbmaint.NewStore(r.db).Peek(context.Background(), r.fileID(t))
		require.NoError(t, err)
		assert.Zero(t, st.Interruptions)
		n := noticeOf(t, r.status(t))
		require.NotNil(t, n)
		assert.Equal(t, "restart_to_optimize", n["code"], "no stopped notice once the counters are reset")
	})
}
