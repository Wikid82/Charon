package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

// Contract tests: the parsing code is fed output recorded from the real cscli bundled with Charon
// (see testdata/cscli/<version>/README.md for the version and how to refresh the recording).

const (
	cscliFixtureDir = "testdata/cscli/v1.8.2-rc1"
)

// cscliFixture returns the recorded combined stdout/stderr of a command.
func cscliFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(cscliFixtureDir, name+".txt")) //nolint:gosec // G304: fixed testdata directory, names come from the tests
	require.NoError(t, err)
	return data
}

// cscliFixtureExit returns the recorded exit status of a command.
func cscliFixtureExit(t *testing.T, name string) int {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(cscliFixtureDir, name+".exit")) //nolint:gosec // G304: fixed testdata directory, names come from the tests
	require.NoError(t, err)
	code, err := strconv.Atoi(strings.TrimSpace(string(data)))
	require.NoError(t, err)
	return code
}

// lapiFixture returns a recorded body of GET /v1/decisions on the local API.
func lapiFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(cscliFixtureDir, name+".body.json")) //nolint:gosec // G304: fixed testdata directory, names come from the tests
	require.NoError(t, err)
	return data
}

// exitStatusError returns the error os/fx reports for a command that exits with the given status.
func exitStatusError(t *testing.T, code int) error {
	t.Helper()
	err := exec.Command("sh", "-c", "exit "+strconv.Itoa(code)).Run() //nolint:gosec // G204: fixed test command
	var exitErr *exec.ExitError
	require.True(t, errors.As(err, &exitErr))
	return err
}

// fixtureCmdExec answers cscli commands with recorded output. Commands are keyed by their arguments without the
// leading "-c <config>"; an unknown command fails the test so a changed invocation is noticed.
type fixtureCmdExec struct {
	t     *testing.T
	mu    sync.Mutex
	byCmd map[string]string
	calls [][]string
}

func newFixtureCmdExec(t *testing.T, byCmd map[string]string) *fixtureCmdExec {
	t.Helper()
	return &fixtureCmdExec{t: t, byCmd: byCmd}
}

func (f *fixtureCmdExec) Execute(_ context.Context, _ string, args ...string) ([]byte, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.calls = append(f.calls, append([]string(nil), args...))
	rest := args
	if len(rest) >= 2 && rest[0] == "-c" {
		rest = rest[2:]
	}
	key := strings.Join(rest, " ")
	name, ok := f.byCmd[key]
	if !ok {
		f.t.Errorf("unexpected cscli invocation: %q", key)
		return nil, errors.New("unexpected cscli invocation")
	}
	out := cscliFixture(f.t, name)
	if code := cscliFixtureExit(f.t, name); code != 0 {
		return out, exitStatusError(f.t, code)
	}
	return out, nil
}

// rawOutputExec returns fixed output for every command.
type rawOutputExec struct {
	out []byte
	err error
}

func (r *rawOutputExec) Execute(context.Context, string, ...string) ([]byte, error) {
	return r.out, r.err
}

type decisionsResponse struct {
	Decisions []map[string]any `json:"decisions"`
	Total     int              `json:"total"`
	Error     string           `json:"error"`
	Source    string           `json:"source"`
}

func listDecisionsVia(t *testing.T, fx CommandExecutor, dataDir string) (code int, resp decisionsResponse, raw map[string]json.RawMessage) {
	t.Helper()
	h := newTestCrowdsecHandler(t, OpenTestDB(t), &fakeExec{}, "/bin/false", dataDir)
	h.CmdExec = fx
	r := gin.New()
	r.GET("/decisions", h.ListDecisions)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/decisions", http.NoBody))
	if w.Code == http.StatusOK {
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &raw))
	}
	return w.Code, resp, raw
}

func decisionValues(resp decisionsResponse) []string {
	values := make([]string, 0, len(resp.Decisions))
	for _, d := range resp.Decisions {
		values = append(values, fmt.Sprint(d["value"]))
	}
	return values
}

func TestCSCLIContract_ListDecisions_RecordedOutput(t *testing.T) {
	t.Parallel()
	const cmd = "decisions list -o json"

	t.Run("alert-shaped entries show every decision field", func(t *testing.T) {
		t.Parallel()
		fx := newFixtureCmdExec(t, map[string]string{cmd: "decisions_list_json"})
		code, resp, _ := listDecisionsVia(t, fx, t.TempDir())
		require.Equal(t, http.StatusOK, code)
		require.Equal(t, 3, resp.Total)
		require.Len(t, resp.Decisions, 3)

		want := []map[string]any{
			{"id": float64(3), "origin": "cscli", "type": "ban", "scope": "Range", "value": "198.51.100.0/24", "duration": "1h59m59s", "scenario": "manual ban: range"},
			{"id": float64(2), "origin": "cscli", "type": "ban", "scope": "Ip", "value": "192.0.2.11", "duration": "59m59s", "scenario": "manual ban"},
			{"id": float64(1), "origin": "cscli", "type": "ban", "scope": "Ip", "value": "192.0.2.10", "duration": "23h59m59s", "scenario": "manual ban: example"},
		}
		for i, w := range want {
			for k, v := range w {
				assert.Equal(t, v, resp.Decisions[i][k], "decision %d field %s", i, k)
			}
			created, err := time.Parse(time.RFC3339, fmt.Sprint(resp.Decisions[i]["created_at"]))
			require.NoError(t, err, "decision %d created_at", i)
			assert.Equal(t, time.Date(2026, 10, 16, 23, 15, 34, 0, time.UTC), created.UTC())
		}
	})

	t.Run("list after a deletion and run without a config path", func(t *testing.T) {
		t.Parallel()
		for _, name := range []string{"decisions_list_json_afterdel", "decisions_list_nocfg_json"} {
			fx := newFixtureCmdExec(t, map[string]string{cmd: name})
			code, resp, _ := listDecisionsVia(t, fx, t.TempDir())
			require.Equal(t, http.StatusOK, code, name)
			assert.NotEmpty(t, resp.Decisions, name)
			for _, d := range resp.Decisions {
				assert.NotEmpty(t, d["value"], name)
				assert.NotEmpty(t, d["type"], name)
				assert.NotEmpty(t, d["scope"], name)
				assert.NotEmpty(t, d["origin"], name)
				assert.NotEmpty(t, d["duration"], name)
			}
		}
		fx := newFixtureCmdExec(t, map[string]string{cmd: "decisions_list_json_afterdel"})
		_, resp, _ := listDecisionsVia(t, fx, t.TempDir())
		assert.Equal(t, []string{"198.51.100.0/24", "192.0.2.10"}, decisionValues(resp))
	})

	t.Run("the config path is passed ahead of the command", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		require.NoError(t, os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(importableConfig), 0o600))
		fx := newFixtureCmdExec(t, map[string]string{cmd: "decisions_list_empty_json"})
		code, _, _ := listDecisionsVia(t, fx, dir)
		require.Equal(t, http.StatusOK, code)
		require.Len(t, fx.calls, 1)
		assert.Equal(t, []string{"-c", filepath.Join(dir, "config.yaml"), "decisions", "list", "-o", "json"}, fx.calls[0])
	})

	t.Run("recorded empty list", func(t *testing.T) {
		t.Parallel()
		fx := newFixtureCmdExec(t, map[string]string{cmd: "decisions_list_empty_json"})
		code, resp, raw := listDecisionsVia(t, fx, t.TempDir())
		require.Equal(t, http.StatusOK, code)
		assert.Empty(t, resp.Decisions)
		assert.Equal(t, 0, resp.Total)
		assert.JSONEq(t, `[]`, string(raw["decisions"]))
	})

	t.Run("older and empty spellings of no decisions", func(t *testing.T) {
		t.Parallel()
		for name, out := range map[string]string{"null": "null", "null newline": "null\n", "empty": "", "blank": "\n", "empty array newline": "[]\n"} {
			code, resp, raw := listDecisionsVia(t, &rawOutputExec{out: []byte(out)}, t.TempDir())
			require.Equal(t, http.StatusOK, code, name)
			assert.Empty(t, resp.Decisions, name)
			assert.JSONEq(t, `[]`, string(raw["decisions"]), name)
		}
	})

	t.Run("flat entries are still accepted", func(t *testing.T) {
		t.Parallel()
		// The local API answers with flat entries; so did older cscli output.
		_, resp, _ := listDecisionsVia(t, &rawOutputExec{out: lapiFixture(t, "lapi_v1_decisions")}, t.TempDir())
		assert.Equal(t, []string{"198.51.100.0/24", "192.0.2.10"}, decisionValues(resp))
		assert.Equal(t, "manual ban: example", resp.Decisions[1]["scenario"])

		flat := `[{"id":7,"origin":"crowdsec","type":"ban","scope":"Ip","value":"192.0.2.77","duration":"4h","scenario":"crowdsecurity/ssh-bf","created_at":"2025-12-05T10:00:00Z","until":"2025-12-05T14:00:00Z"}]`
		_, resp, _ = listDecisionsVia(t, &rawOutputExec{out: []byte(flat)}, t.TempDir())
		require.Len(t, resp.Decisions, 1)
		assert.Equal(t, "crowdsecurity/ssh-bf", resp.Decisions[0]["scenario"])
		assert.Equal(t, "2025-12-05T14:00:00Z", resp.Decisions[0]["until"])
	})

	t.Run("response field names are unchanged", func(t *testing.T) {
		t.Parallel()
		_, _, raw := listDecisionsVia(t, newFixtureCmdExec(t, map[string]string{cmd: "decisions_list_json"}), t.TempDir())
		assert.ElementsMatch(t, []string{"decisions", "total"}, keysOf(raw))

		flat := `[{"id":7,"origin":"cscli","type":"ban","scope":"Ip","value":"192.0.2.77","duration":"4h","scenario":"s","created_at":"2025-12-05T10:00:00Z","until":"2025-12-05T14:00:00Z"}]`
		_, resp, _ := listDecisionsVia(t, &rawOutputExec{out: []byte(flat)}, t.TempDir())
		require.Len(t, resp.Decisions, 1)
		assert.ElementsMatch(t, []string{"id", "origin", "type", "scope", "value", "duration", "scenario", "created_at", "until"}, keysOf(resp.Decisions[0]))
	})

	t.Run("a failed cscli run gives an empty list and a message", func(t *testing.T) {
		t.Parallel()
		fx := newFixtureCmdExec(t, map[string]string{cmd: "decisions_list_lapi_down"})
		code, resp, _ := listDecisionsVia(t, fx, t.TempDir())
		require.Equal(t, http.StatusOK, code)
		assert.Empty(t, resp.Decisions)
		assert.NotEmpty(t, resp.Error)
	})
}

func TestCSCLIContract_ListDecisions_SkipsRowsWithoutValue(t *testing.T) {
	t.Parallel()
	alerts := `[
	 {"id":1,"scenario":"s1","created_at":"2026-10-16T23:15:34Z","decisions":[
	   {"id":1,"origin":"cscli","type":"ban","scope":"Ip","value":"192.0.2.10","duration":"1h"},
	   {"id":2,"origin":"cscli","type":"ban","scope":"Ip","value":"","duration":"1h"},
	   {"id":3,"origin":"cscli","type":"ban","scope":"Ip","duration":"1h"}]},
	 {"id":2,"scenario":"s2","created_at":"2026-10-16T23:15:34Z","decisions":[]},
	 {"id":3,"scenario":"s3","created_at":"2026-10-16T23:15:34Z","decisions":null},
	 {"id":4,"scenario":"s4","created_at":"2026-10-16T23:15:34Z"},
	 {"id":5,"scenario":"s5","created_at":"2026-10-16T23:15:34Z","decisions":[
	   {"id":5,"origin":"cscli","type":"ban","scope":"Range","value":"198.51.100.0/24","duration":"2h"}]}]`
	code, resp, _ := listDecisionsVia(t, &rawOutputExec{out: []byte(alerts)}, t.TempDir())
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, []string{"192.0.2.10", "198.51.100.0/24"}, decisionValues(resp))
	assert.Equal(t, 2, resp.Total)
	assert.Equal(t, "s1", resp.Decisions[0]["scenario"], "the alert scenario is carried to the decision")
	assert.Equal(t, "s5", resp.Decisions[1]["scenario"])
	for _, d := range resp.Decisions {
		assert.NotEmpty(t, d["value"])
	}

	// Flat entries without a value are skipped too.
	flat := `[{"id":1,"origin":"cscli","type":"ban","scope":"Ip","value":"","duration":"1h","scenario":"x"},
	          {"id":2,"origin":"cscli","type":"ban","scope":"Ip","value":"192.0.2.12","duration":"1h","scenario":"x"}]`
	_, resp, _ = listDecisionsVia(t, &rawOutputExec{out: []byte(flat)}, t.TempDir())
	assert.Equal(t, []string{"192.0.2.12"}, decisionValues(resp))
}

func TestCSCLIContract_ListDecisions_BoundedInput(t *testing.T) {
	t.Parallel()

	t.Run("number of rows", func(t *testing.T) {
		t.Parallel()
		// No listing may be larger than the cap.
		const ceiling = maxCSCLIDecisions
		const rows = ceiling + 20_000
		var b strings.Builder
		b.WriteString("[")
		for i := 0; i < rows; i++ {
			if i > 0 {
				b.WriteString(",")
			}
			fmt.Fprintf(&b, `{"id":%d,"scenario":"s","created_at":"2026-10-16T23:15:34Z","decisions":[{"id":%d,"origin":"cscli","type":"ban","scope":"Ip","value":"10.%d.%d.%d","duration":"1h"}]}`,
				i, i, (i>>16)&255, (i>>8)&255, i&255)
		}
		b.WriteString("]")
		code, resp, _ := listDecisionsVia(t, &rawOutputExec{out: []byte(b.String())}, t.TempDir())
		require.Equal(t, http.StatusOK, code)
		assert.LessOrEqual(t, len(resp.Decisions), ceiling)
		assert.Equal(t, len(resp.Decisions), resp.Total)
	})

	t.Run("nesting depth", func(t *testing.T) {
		t.Parallel()
		const depth = 50_000
		deep := strings.Repeat(`{"a":`, depth) + `1` + strings.Repeat(`}`, depth)
		alerts := `[{"id":1,"scenario":"s","created_at":"2026-10-16T23:15:34Z","source":` + deep +
			`,"decisions":[{"id":1,"origin":"cscli","type":"ban","scope":"Ip","value":"192.0.2.10","duration":"1h"}]}]`
		start := time.Now()
		code, resp, _ := listDecisionsVia(t, &rawOutputExec{out: []byte(alerts)}, t.TempDir())
		assert.Less(t, time.Since(start), 5*time.Second)
		// Either the unknown field is ignored or the output is refused; it never panics or lists blank rows.
		assert.Contains(t, []int{http.StatusOK, http.StatusInternalServerError}, code)
		for _, d := range resp.Decisions {
			assert.NotEmpty(t, d["value"])
		}
	})
}

func keysOf[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// newLAPIServer serves the given body as the local API decisions answer and records the requested query.
func newLAPIServer(t *testing.T, body []byte) (*httptest.Server, *url.Values) {
	t.Helper()
	var mu sync.Mutex
	got := &url.Values{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		*got = r.URL.Query()
		mu.Unlock()
		if r.URL.Path != "/v1/decisions" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv, got
}

func newLAPIHandler(t *testing.T, lapiURL string) *CrowdsecHandler {
	t.Helper()
	db := OpenTestDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityDecision{}, &models.SecurityConfig{}, &models.Setting{}))
	require.NoError(t, db.Create(&models.SecurityConfig{UUID: "default", Name: "default", CrowdSecAPIURL: lapiURL}).Error)
	h := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", t.TempDir())
	h.validateLAPIURL = url.Parse
	return h
}

func TestCSCLIContract_GetLAPIDecisions_RecordedResponse(t *testing.T) {
	t.Parallel()

	serve := func(t *testing.T, h *CrowdsecHandler, query string) (int, decisionsResponse) {
		t.Helper()
		r := gin.New()
		r.GET("/decisions", h.GetLAPIDecisions)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/decisions"+query, http.NoBody))
		var resp decisionsResponse
		if w.Code == http.StatusOK {
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
		}
		return w.Code, resp
	}

	t.Run("the recorded flat answer is shown completely", func(t *testing.T) {
		t.Parallel()
		srv, _ := newLAPIServer(t, lapiFixture(t, "lapi_v1_decisions"))
		code, resp := serve(t, newLAPIHandler(t, srv.URL), "")
		require.Equal(t, http.StatusOK, code)
		assert.Equal(t, "lapi", resp.Source)
		assert.Equal(t, 2, resp.Total)
		require.Len(t, resp.Decisions, 2)
		assert.Equal(t, map[string]any{"id": float64(3), "origin": "cscli", "type": "ban", "scope": "Range", "value": "198.51.100.0/24", "duration": "1h59m35s", "scenario": "manual ban: range"},
			withoutKey(resp.Decisions[0], "created_at"))
		assert.Equal(t, "192.0.2.10", resp.Decisions[1]["value"])
		assert.Equal(t, "Ip", resp.Decisions[1]["scope"])
		assert.Equal(t, "manual ban: example", resp.Decisions[1]["scenario"])
	})

	t.Run("filters reach the local API", func(t *testing.T) {
		t.Parallel()
		srv, query := newLAPIServer(t, lapiFixture(t, "lapi_v1_decisions_ip_filter"))
		code, resp := serve(t, newLAPIHandler(t, srv.URL), "?ip=192.0.2.10&scope=ip&type=ban")
		require.Equal(t, http.StatusOK, code)
		assert.Equal(t, []string{"192.0.2.10"}, decisionValues(resp))
		assert.Equal(t, "192.0.2.10", query.Get("ip"))
		assert.Equal(t, "ip", query.Get("scope"))
		assert.Equal(t, "ban", query.Get("type"))
	})

	t.Run("the recorded null answer is an empty list", func(t *testing.T) {
		t.Parallel()
		srv, _ := newLAPIServer(t, lapiFixture(t, "lapi_v1_decisions_empty"))
		code, resp := serve(t, newLAPIHandler(t, srv.URL), "?ip=192.0.2.99")
		require.Equal(t, http.StatusOK, code)
		assert.Equal(t, "lapi", resp.Source)
		assert.Empty(t, resp.Decisions)
		assert.Equal(t, 0, resp.Total)
	})

	t.Run("a refused key falls back to the cscli listing", func(t *testing.T) {
		t.Parallel()
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusForbidden) }))
		t.Cleanup(srv.Close)
		fx := newFixtureCmdExec(t, map[string]string{"decisions list -o json": "decisions_list_json"})
		h := newLAPIHandler(t, srv.URL)
		h.CmdExec = fx
		code, resp := serve(t, h, "")
		require.Equal(t, http.StatusOK, code)
		assert.Equal(t, []string{"198.51.100.0/24", "192.0.2.11", "192.0.2.10"}, decisionValues(resp))
	})
}

func withoutKey(m map[string]any, key string) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		if k != key {
			out[k] = v
		}
	}
	return out
}

func TestCSCLIContract_DashboardSummary_ActiveDecisionsFromRecordedResponse(t *testing.T) {
	t.Parallel()
	for name, tc := range map[string]struct {
		fixture string
		want    float64
	}{
		"two decisions":  {"lapi_v1_decisions", 2},
		"null answer":    {"lapi_v1_decisions_empty", 0},
		"filtered entry": {"lapi_v1_decisions_ip_filter", 1},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			srv, _ := newLAPIServer(t, lapiFixture(t, tc.fixture))
			h := newLAPIHandler(t, srv.URL)
			r := gin.New()
			h.RegisterRoutes(r.Group("/api/v1"))
			w := httptest.NewRecorder()
			r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/api/v1/admin/crowdsec/dashboard/summary?range=1h", http.NoBody))
			require.Equal(t, http.StatusOK, w.Code)
			var body map[string]any
			require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
			assert.Equal(t, tc.want, body["active_decisions"])
		})
	}
}

func diagnosticsConfigVia(t *testing.T, fx CommandExecutor) (out map[string]any, dir string) {
	t.Helper()
	dir = t.TempDir()
	writeTree(t, dir, map[string]string{"config/config.yaml": importableConfig, "config/acquis.yaml": "source: file\nfilenames:\n  - /var/log/example.log\n"})
	h := newTestCrowdsecHandler(t, OpenTestDB(t), &fakeExec{}, "/bin/false", dir)
	h.CmdExec = fx
	r := gin.New()
	r.GET("/d", h.DiagnosticsConfig)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/d", http.NoBody))
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
	return out, dir
}

func errorStrings(t *testing.T, out map[string]any) []string {
	t.Helper()
	raw, ok := out["errors"].([]any)
	require.True(t, ok, "errors must be a list: %v", out["errors"])
	msgs := make([]string, 0, len(raw))
	for _, e := range raw {
		msgs = append(msgs, fmt.Sprint(e))
	}
	return msgs
}

func TestCSCLIContract_DiagnosticsConfig_ConfigShow(t *testing.T) {
	t.Parallel()
	t.Skip("enabled by a later commit in this series")
	// The old command ("config check") does not exist; it is answered with the recorded usage text and exit status 1.
	const show = "config show"
	oldCheck := "config check"

	t.Run("valid configuration", func(t *testing.T) {
		t.Parallel()
		fx := newFixtureCmdExec(t, map[string]string{show: "config_show_ok", oldCheck: "config_check"})
		out, dir := diagnosticsConfigVia(t, fx)
		assert.Equal(t, true, out["config_exists"])
		assert.Equal(t, true, out["config_valid"])
		assert.Empty(t, errorStrings(t, out))
		require.Len(t, fx.calls, 1)
		assert.Equal(t, []string{"-c", filepath.Join(dir, "config", "config.yaml"), "config", "show"}, fx.calls[0])
	})

	for name, tc := range map[string]struct {
		fixture string
		reason  string
		leaked  string
	}{
		"malformed yaml": {"config_show_malformed", "sequence end token ']' not found", "/tmp/bad.yaml"},
		"missing file":   {"config_show_missing", "no such file or directory", "/tmp/does-not-exist.yaml"},
		"invalid value":  {"config_show_invalid_value", `not a valid logrus Level: "12"`, "/tmp/badval.yaml"},
	} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			fx := newFixtureCmdExec(t, map[string]string{show: tc.fixture})
			out, dir := diagnosticsConfigVia(t, fx)
			assert.Equal(t, false, out["config_valid"])
			msgs := errorStrings(t, out)
			require.Len(t, msgs, 1)
			assert.Contains(t, msgs[0], tc.reason)
			assert.NotContains(t, msgs[0], tc.leaked, "paths are redacted")
			assert.NotContains(t, msgs[0], dir)
			assert.NotContains(t, strings.ToLower(msgs[0]), "unavailable")
		})
	}

	t.Run("only the first output line is reported", func(t *testing.T) {
		t.Parallel()
		multi := &rawOutputExec{out: []byte("Error: first problem\nsecond line with details\nthird line\n"), err: exitStatusError(t, 1)}
		out, _ := diagnosticsConfigVia(t, multi)
		assert.Equal(t, false, out["config_valid"])
		msgs := errorStrings(t, out)
		require.Len(t, msgs, 1)
		assert.Contains(t, msgs[0], "first problem")
		assert.NotContains(t, msgs[0], "second line")
		assert.NotContains(t, msgs[0], "third line")
	})

	t.Run("missing cscli binary is reported as unavailable, not as an invalid config", func(t *testing.T) {
		t.Parallel()
		missing := &rawOutputExec{err: &exec.Error{Name: "cscli", Err: exec.ErrNotFound}}
		out, _ := diagnosticsConfigVia(t, missing)
		msgs := errorStrings(t, out)
		require.Len(t, msgs, 1)
		assert.Contains(t, strings.ToLower(msgs[0]), "cscli unavailable")
		assert.NotContains(t, strings.ToLower(msgs[0]), "invalid")
		assert.NotContains(t, strings.ToLower(msgs[0]), "validation failed")
	})
}

// Sweep: every other cscli invocation Charon relies on, driven by the recorded output.

func TestCSCLIContract_BanAndUnban(t *testing.T) {
	t.Parallel()
	newRouter := func(t *testing.T, fx CommandExecutor) *gin.Engine {
		t.Helper()
		h := newTestCrowdsecHandler(t, setupCrowdDB(t), &fakeExec{}, "/bin/false", t.TempDir())
		h.CmdExec = fx
		r := gin.New()
		h.RegisterRoutes(r.Group("/api/v1"))
		return r
	}
	ban := func(r *gin.Engine, body string) *httptest.ResponseRecorder {
		w := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/crowdsec/ban", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		r.ServeHTTP(w, req)
		return w
	}

	t.Run("ban uses flags the bundled cscli accepts", func(t *testing.T) {
		t.Parallel()
		fx := newFixtureCmdExec(t, map[string]string{"decisions add -i 192.0.2.10 -d 24h -R manual ban: example -t ban": "decisions_add_ban"})
		w := ban(newRouter(t, fx), `{"ip":"192.0.2.10","reason":"example"}`)
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.JSONEq(t, `{"status":"banned","ip":"192.0.2.10","duration":"24h"}`, w.Body.String())
		assert.Equal(t, "level=info msg=\"Decision successfully added\"\n", string(cscliFixture(t, "decisions_add_ban")))
	})

	t.Run("cscli refusing the address is an error", func(t *testing.T) {
		t.Parallel()
		fx := newFixtureCmdExec(t, map[string]string{"decisions add -i notanip -d 1h -R manual ban -t ban": "decisions_add_badip"})
		w := ban(newRouter(t, fx), `{"ip":"notanip","duration":"1h"}`)
		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.JSONEq(t, `{"error":"failed to ban IP"}`, w.Body.String())
	})

	t.Run("unban uses flags the bundled cscli accepts", func(t *testing.T) {
		t.Parallel()
		fx := newFixtureCmdExec(t, map[string]string{"decisions delete -i 192.0.2.11": "decisions_delete"})
		w := httptest.NewRecorder()
		newRouter(t, fx).ServeHTTP(w, httptest.NewRequest(http.MethodDelete, "/api/v1/admin/crowdsec/ban/192.0.2.11", http.NoBody))
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.JSONEq(t, `{"status":"unbanned","ip":"192.0.2.11"}`, w.Body.String())
	})
}

func TestCSCLIContract_BouncerCommands(t *testing.T) {
	t.Parallel()
	h := func(t *testing.T, fx CommandExecutor) *CrowdsecHandler {
		t.Helper()
		hh := newTestCrowdsecHandler(t, setupCrowdDB(t), &fakeExec{}, "/bin/false", t.TempDir())
		hh.CmdExec = fx
		return hh
	}
	const list = "bouncers list -o json"

	t.Run("registered bouncer is found in the recorded list", func(t *testing.T) {
		t.Parallel()
		assert.True(t, h(t, newFixtureCmdExec(t, map[string]string{list: "bouncers_list_json"})).validateBouncerKey(t.Context()))
	})
	t.Run("empty lists are not registered", func(t *testing.T) {
		t.Parallel()
		for _, name := range []string{"bouncers_list_empty_json", "bouncers_list_afterdelete_json"} {
			assert.False(t, h(t, newFixtureCmdExec(t, map[string]string{list: name})).validateBouncerKey(t.Context()), name)
		}
	})
	t.Run("add prints only the key and delete output is ignored", func(t *testing.T) {
		t.Parallel()
		add := cscliFixture(t, "bouncers_add_raw")
		assert.NotContains(t, string(add), "\n")
		assert.Regexp(t, `^[A-Za-z0-9]+$`, string(add))

		fx := newFixtureCmdExec(t, map[string]string{
			"bouncers delete caddy-bouncer":         "bouncers_delete",
			"bouncers add caddy-bouncer -o raw":     "bouncers_add_raw",
			"bouncers delete caddy-bouncer-missing": "bouncers_delete_missing",
			"bouncers add caddy-bouncer-dup -o raw": "bouncers_add_dup",
		})
		hh := h(t, fx)
		key, err := hh.registerAndSaveBouncer(t.Context())
		require.NoError(t, err)
		assert.Equal(t, string(add), key)
		saved, err := os.ReadFile(hh.bouncerKeyPath())
		require.NoError(t, err)
		assert.Equal(t, string(add), strings.TrimSpace(string(saved)))
	})
	t.Run("a refused add is an error and the output is not repeated verbatim as a key", func(t *testing.T) {
		t.Parallel()
		fx := newFixtureCmdExec(t, map[string]string{
			"bouncers delete caddy-bouncer":     "bouncers_delete_missing",
			"bouncers add caddy-bouncer -o raw": "bouncers_add_dup",
		})
		_, err := h(t, fx).registerAndSaveBouncer(t.Context())
		require.Error(t, err)
	})
}

func TestCSCLIContract_AlertsList(t *testing.T) {
	t.Parallel()
	h := func(t *testing.T, fx CommandExecutor) *CrowdsecHandler {
		t.Helper()
		hh := newTestCrowdsecHandler(t, setupCrowdDB(t), &fakeExec{}, "/bin/false", t.TempDir())
		hh.CmdExec = fx
		return hh
	}

	t.Run("recorded alerts", func(t *testing.T) {
		t.Parallel()
		fx := newFixtureCmdExec(t, map[string]string{"alerts list -o json -l 50": "alerts_list_json"})
		alerts, total := h(t, fx).fetchAlertsCscli(t.Context(), "", 50)
		assert.Equal(t, 3, total)
		require.Len(t, alerts, 3)
		first, ok := alerts[0].(map[string]any)
		require.True(t, ok)
		assert.Equal(t, "manual ban: range", first["scenario"])
		assert.NotNil(t, first["decisions"])
	})
	t.Run("recorded empty list", func(t *testing.T) {
		t.Parallel()
		fx := newFixtureCmdExec(t, map[string]string{"alerts list -o json -l 50": "alerts_list_empty_json"})
		alerts, total := h(t, fx).fetchAlertsCscli(t.Context(), "", 50)
		assert.Equal(t, 0, total)
		assert.Empty(t, alerts)
	})
	t.Run("scenario and limit flags are accepted", func(t *testing.T) {
		t.Parallel()
		fx := newFixtureCmdExec(t, map[string]string{
			"alerts list -o json -s manual ban -l 2": "alerts_list_flags_json",
			"alerts list -o json -s nomatch -l 5":    "alerts_list_flags_nomatch_json",
		})
		hh := h(t, fx)
		alerts, total := hh.fetchAlertsCscli(t.Context(), "manual ban", 2)
		assert.Equal(t, 1, total, "the recorded run asked for the alerts of one scenario")
		require.Len(t, alerts, 1)
		alerts, total = hh.fetchAlertsCscli(t.Context(), "nomatch", 5)
		assert.Equal(t, 0, total)
		assert.Empty(t, alerts)
	})
}

func TestCSCLIContract_DiagnosticsConnectivity_StatusCommands(t *testing.T) {
	t.Parallel()
	run := func(t *testing.T, lapi, capi string) map[string]any {
		t.Helper()
		dir := t.TempDir()
		writeTree(t, dir, map[string]string{"config/config.yaml": importableConfig, "config/online_api_credentials.yaml": "x"})
		h := newTestCrowdsecHandler(t, OpenTestDB(t), &fakeExec{started: true}, "/bin/false", dir)
		h.CmdExec = newFixtureCmdExec(t, map[string]string{"lapi status": lapi, "capi status": capi})
		r := gin.New()
		r.GET("/d", h.DiagnosticsConnectivity)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, httptest.NewRequest(http.MethodGet, "/d", http.NoBody))
		require.Equal(t, http.StatusOK, w.Code)
		var out map[string]any
		require.NoError(t, json.Unmarshal(w.Body.Bytes(), &out))
		return out
	}

	t.Run("lapi status exit codes", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, 0, cscliFixtureExit(t, "lapi_status"))
		assert.NotZero(t, cscliFixtureExit(t, "lapi_status_down"))
		assert.Equal(t, true, run(t, "lapi_status", "capi_status")["lapi_ready"])
		assert.Equal(t, false, run(t, "lapi_status_down", "capi_status")["lapi_ready"])
	})
	t.Run("capi status that is not registered", func(t *testing.T) {
		t.Parallel()
		assert.NotZero(t, cscliFixtureExit(t, "capi_status"))
		out := run(t, "lapi_status", "capi_status")
		assert.Equal(t, false, out["capi_reachable"])
		assert.NotContains(t, out, "capi_status_output")
	})
}

// Output limit of the command executor.

// largeOutputScript prints more than the cscli output cap on the given stream, then idles so that only the
// executor's own limit (not the end of the command) can stop it quickly.
func largeOutputScript(stream string) string {
	redirect := ""
	if stream == "stderr" {
		redirect = " >&2"
	}
	return "head -c 40000000 /dev/zero" + redirect + "; fx sleep 5"
}

func TestRealCommandExecutor_Execute_CombinesStreams(t *testing.T) {
	t.Parallel()
	out, err := (&RealCommandExecutor{}).Execute(t.Context(), "sh", "-c", "echo out; echo err >&2")
	require.NoError(t, err)
	assert.Equal(t, "out\nerr\n", string(out))

	out, err = (&RealCommandExecutor{}).Execute(t.Context(), "sh", "-c", "echo bad >&2; exit 3")
	require.Error(t, err)
	var exitErr *exec.ExitError
	require.True(t, errors.As(err, &exitErr))
	assert.Equal(t, "bad\n", string(out))
}

func TestRealCommandExecutor_Execute_OutputLimit(t *testing.T) {
	t.Parallel()

	t.Run("output at the parser cap is returned in full", func(t *testing.T) {
		t.Parallel()
		out, err := (&RealCommandExecutor{}).Execute(t.Context(), "sh", "-c", "head -c 33554432 /dev/zero")
		require.NoError(t, err)
		assert.Len(t, out, 32<<20)
	})

	for _, stream := range []string{"stdout", "stderr"} {
		t.Run("output beyond the cap on "+stream+" stops the command with a clear error", func(t *testing.T) {
			t.Parallel()
			t.Skip("enabled by a later commit in this series")
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()
			start := time.Now()
			out, err := (&RealCommandExecutor{}).Execute(ctx, "sh", "-c", largeOutputScript(stream))
			require.Error(t, err)
			assert.Contains(t, strings.ToLower(err.Error()), "output")
			assert.Contains(t, strings.ToLower(err.Error()), "limit")
			assert.Less(t, time.Since(start), 4*time.Second, "the command is killed, not waited for")
			assert.Less(t, len(out), 40_000_000, "the output kept is bounded")
		})
	}
}
