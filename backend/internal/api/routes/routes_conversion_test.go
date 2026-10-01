package routes

import (
	"context"
	"database/sql"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/http/httputil"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	glebarez "github.com/glebarez/sqlite"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/api/handlers"
	"github.com/Wikid82/charon/backend/internal/caddy"
	"github.com/Wikid82/charon/backend/internal/cerberus"
	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/database"
	"github.com/Wikid82/charon/backend/internal/dbmaint"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/server"
)

// caddyAdminStub stands in for Caddy's admin API: it answers the readiness ping
// and records every configuration pushed to /load.
type caddyAdminStub struct {
	*httptest.Server
	mu    sync.Mutex
	loads [][]byte
}

func newCaddyAdminStub(t *testing.T) *caddyAdminStub {
	t.Helper()
	s := &caddyAdminStub{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/load":
			body, _ := io.ReadAll(r.Body)
			s.mu.Lock()
			s.loads = append(s.loads, body)
			s.mu.Unlock()
			w.WriteHeader(http.StatusOK)
		case r.Method == http.MethodGet && r.URL.Path == "/config/":
			_, _ = w.Write([]byte(`{}`))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(s.Close)
	return s
}

func (s *caddyAdminStub) loadCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.loads)
}

func (s *caddyAdminStub) lastLoad() []byte {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.loads) == 0 {
		return nil
	}
	return s.loads[len(s.loads)-1]
}

// standInDataPlane is what Caddy does with the configuration Charon pushed,
// reduced to routing by Host header: it reads ONLY the recorded JSON and never
// calls back into Charon or its database. Real Caddy is a separate process with
// the same property (the plan's 2.1 finding: no forward_auth callback).
func standInDataPlane(t *testing.T, cfgJSON []byte) http.Handler {
	t.Helper()
	var cfg any
	require.NoError(t, json.Unmarshal(cfgJSON, &cfg))

	upstreams := map[string]string{} // host -> dial
	var walk func(node any)
	walk = func(node any) {
		switch v := node.(type) {
		case map[string]any:
			if hosts := matchedHosts(v["match"]); len(hosts) > 0 {
				if dial := findDial(v); dial != "" {
					for _, h := range hosts {
						upstreams[h] = dial
					}
					return
				}
			}
			for _, child := range v {
				walk(child)
			}
		case []any:
			for _, child := range v {
				walk(child)
			}
		}
	}
	walk(cfg)
	require.NotEmpty(t, upstreams, "the pushed configuration routes at least one host: %s", cfgJSON)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		dial, ok := upstreams[r.Host]
		if !ok {
			http.NotFound(w, r)
			return
		}
		(&httputil.ReverseProxy{Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(&url.URL{Scheme: "http", Host: dial})
		}}).ServeHTTP(w, r)
	})
}

func matchedHosts(match any) []string {
	var hosts []string
	list, _ := match.([]any)
	for _, m := range list {
		entry, _ := m.(map[string]any)
		names, _ := entry["host"].([]any)
		for _, n := range names {
			if s, ok := n.(string); ok {
				hosts = append(hosts, s)
			}
		}
	}
	return hosts
}

func findDial(node any) string {
	switch v := node.(type) {
	case map[string]any:
		if dial, ok := v["dial"].(string); ok {
			return dial
		}
		for _, child := range v {
			if dial := findDial(child); dial != "" {
				return dial
			}
		}
	case []any:
		for _, child := range v {
			if dial := findDial(child); dial != "" {
				return dial
			}
		}
	}
	return ""
}

// seedLegacyDatabase writes a mode-0 (auto_vacuum none) WAL database that a
// conversion would take: about 120 MB of which 90% is free. database.Connect
// creates NEW files in incremental mode, so a legacy file has to be built
// without it and opened afterwards.
func seedLegacyDatabase(t *testing.T, path string) {
	t.Helper()
	seed, err := sql.Open(glebarez.DriverName, path)
	require.NoError(t, err)
	seed.SetMaxOpenConns(1)
	for _, stmt := range []string{
		"PRAGMA journal_mode=WAL",
		"CREATE TABLE bulk (id INTEGER PRIMARY KEY, pad BLOB)",
		"WITH RECURSIVE c(x) AS (SELECT 1 UNION ALL SELECT x+1 FROM c WHERE x<1200) " +
			"INSERT INTO bulk(pad) SELECT zeroblob(100000) FROM c",
		"DELETE FROM bulk WHERE id % 10 <> 0",
	} {
		_, err = seed.Exec(stmt)
		require.NoError(t, err, stmt)
	}
	var busy, logFrames, checkpointed int64
	require.NoError(t, seed.QueryRow("PRAGMA wal_checkpoint(TRUNCATE)").Scan(&busy, &logFrames, &checkpointed))
	require.NoError(t, seed.Close())
}

func sqlPragma(t *testing.T, sqlDB *sql.DB, pragma string) int64 {
	t.Helper()
	var v int64
	require.NoError(t, sqlDB.QueryRow("PRAGMA "+pragma).Scan(&v))
	return v
}

// 2.1 / 3.4: once Caddy has its configuration, the data plane does not need the
// database. With the pool's only connection pinned by the runner's real
// conversion, proxied traffic keeps being served from the configuration Charon
// pushed, the healthcheck and the status page answer, the management API gets
// the fast 503, and Caddy is never asked for a new configuration. When the
// conversion ends, the same traffic and the management plane work again.
func TestConversion_ProxyStaysUpWhilePoolIsPinnedByTheRunner(t *testing.T) {
	gin.SetMode(gin.TestMode)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("hello from upstream"))
	}))
	t.Cleanup(upstream.Close)
	admin := newCaddyAdminStub(t)

	dbPath := filepath.Join(t.TempDir(), "charon.db")
	seedLegacyDatabase(t, dbPath)
	db, err := database.Connect(dbPath)
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	t.Cleanup(func() { _ = sqlDB.Close() })
	require.EqualValues(t, dbmaint.AutoVacuumNone, sqlPragma(t, sqlDB, "auto_vacuum"))

	upstreamAddr := upstream.Listener.Addr().(*net.TCPAddr)
	require.NoError(t, db.AutoMigrate(&models.ProxyGroup{}, &models.ProxyHost{}, &models.Location{},
		&models.Setting{}, &models.CaddyConfig{}, &models.SSLCertificate{}))
	require.NoError(t, db.Create(&models.ProxyHost{
		UUID: "host-1", DomainNames: "app.example.test", ForwardScheme: "http",
		ForwardHost: upstreamAddr.IP.String(), ForwardPort: upstreamAddr.Port, Enabled: true,
	}).Error)

	frontend := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(frontend, "index.html"), []byte("<html>spa-index</html>"), 0o600))
	cfg := config.Config{
		JWTSecret: "test-secret", FrontendDir: frontend, DatabasePath: dbPath,
		CaddyAdminAPI: admin.URL, CaddyConfigDir: t.TempDir(),
		DBCompactOnStart: config.DBCompactOff, // RegisterWithDeps' own planner stays out; the test plans below
	}
	gate := dbmaint.NewGate()
	gate.MarkPlanned() // the pipeline waits, as in production for an eligible install
	router := server.NewRouter(frontend, "", nil)
	router.Use(gate.Middleware(handlers.HealthHandler))
	caddyManager := caddy.NewManager(caddy.NewClientWithExpectedPort(cfg.CaddyAdminAPI, admin.Listener.Addr().(*net.TCPAddr).Port), db, cfg.CaddyConfigDir, cfg.FrontendDir, cfg.ACMEStaging, cfg.Security)
	cerb := cerberus.New(cfg.Security, db)

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	_, err = RegisterWithDeps(ctx, router, db, cfg, caddyManager, cerb, gate)
	require.NoError(t, err)

	// The production runner with the real Convert, held at the start of the
	// conversion so the pinned state can be examined deterministically.
	entered, release := make(chan struct{}), make(chan struct{})
	require.True(t, dbmaint.Start(ctx, dbmaint.StartParams{
		Gate: gate, DB: sqlDB, DBPath: dbPath, EnvMode: config.DBCompactAuto,
		Convert: func(cctx context.Context, conn *sql.Conn) error {
			close(entered)
			<-release
			return dbmaint.Convert(cctx, conn)
		},
	}))
	gate.MarkListenerBound() // main.go binds the listener right after RegisterWithDeps

	select {
	case <-entered:
	case <-time.After(60 * time.Second):
		t.Fatal("the runner never reached the conversion: Caddy config applied? " + string(gate.Snapshot().Phase))
	}
	require.GreaterOrEqual(t, admin.loadCount(), 1, "the initial configuration reached Caddy before the pool was pinned")
	require.Equal(t, dbmaint.PhaseConverting, gate.Snapshot().Phase)
	pushed := admin.lastLoad()
	loadsBefore := admin.loadCount()

	dataPlane := httptest.NewServer(standInDataPlane(t, pushed))
	t.Cleanup(dataPlane.Close)
	proxied := func() string {
		req, reqErr := http.NewRequestWithContext(context.Background(), http.MethodGet, dataPlane.URL, http.NoBody)
		require.NoError(t, reqErr)
		req.Host = "app.example.test"
		client := &http.Client{Timeout: time.Second}
		resp, doErr := client.Do(req)
		require.NoError(t, doErr, "a proxied request must not wait for the database")
		defer func() { _ = resp.Body.Close() }()
		require.Equal(t, http.StatusOK, resp.StatusCode)
		body, _ := io.ReadAll(resp.Body)
		return string(body)
	}

	assert.Equal(t, "hello from upstream", proxied(), "proxied traffic while the pool is pinned")
	assert.NotContains(t, string(pushed), "forward_auth", "the data plane has no callback into Charon")

	rig := &maintenanceRig{router: router, cerb: cerb}
	cerb.InvalidateCache()
	assert.Equal(t, http.StatusOK, rig.get(t, http.MethodGet, "/api/v1/health").Code)
	phase, active := statusPhase(t, rig.get(t, http.MethodGet, "/api/v1/maintenance/status"))
	assert.Equal(t, "converting", phase)
	assert.True(t, active)
	assert.Equal(t, http.StatusServiceUnavailable, rig.get(t, http.MethodGet, "/api/v1/health/db").Code)
	assert.Equal(t, http.StatusServiceUnavailable, rig.get(t, http.MethodGet, "/api/v1/auth/me").Code)
	assert.Equal(t, loadsBefore, admin.loadCount(), "Caddy is not asked for anything new while the pool is pinned")

	close(release)
	require.True(t, gate.WaitRunner(60*time.Second), "the conversion finished")

	assert.Equal(t, dbmaint.PhaseDone, gate.Snapshot().Phase)
	assert.EqualValues(t, dbmaint.AutoVacuumIncremental, sqlPragma(t, sqlDB, "auto_vacuum"))
	assert.Equal(t, "hello from upstream", proxied(), "and after the conversion")
	assert.Equal(t, http.StatusOK, rig.get(t, http.MethodGet, "/api/v1/health/db").Code, "the management plane is back")
	phase, active = statusPhase(t, rig.get(t, http.MethodGet, "/api/v1/maintenance/status"))
	assert.Equal(t, "done", phase)
	assert.False(t, active)
}
