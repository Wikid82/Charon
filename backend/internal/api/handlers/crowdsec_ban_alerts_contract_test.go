package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/crowdsec"
	"github.com/Wikid82/charon/backend/internal/models"
)

// Contract tests for the CrowdSec ban, unban, alerts and dashboard behavior. Commands are answered with output
// recorded from the real bundled cscli (testdata/cscli/v1.8.2-rc1). Tests that describe behavior added later in
// this series call skipUntilLaterCommit; the commit that adds the behavior removes the call.

// skipUntilLaterCommit skips a test whose behavior a later commit in this series adds. Set
// CHARON_RUN_PENDING_TESTS=1 to run the skipped tests anyway.
func skipUntilLaterCommit(t *testing.T) {
	t.Helper()
	if os.Getenv("CHARON_RUN_PENDING_TESTS") != "" {
		return
	}
	t.Skip("enabled by a later commit in this series")
}

const (
	// pr4OverCap is above the parser limit (32 MiB) and below the executor limit (33 MiB).
	pr4OverCap = 32<<20 + 4096

	pr4ListDeadline = 31 * time.Second // listings run under 30 s
	pr4EditDeadline = 16 * time.Second // add and delete run under 15 s
)

// pr4KeyEnvInRuntimeOrder lists the API key variables in the order the CrowdSec runtime reads them.
var pr4KeyEnvInRuntimeOrder = []string{
	"CHARON_SECURITY_CROWDSEC_API_KEY",
	"CROWDSEC_API_KEY",
	"CROWDSEC_BOUNCER_API_KEY",
	"CERBERUS_SECURITY_CROWDSEC_API_KEY",
	"CPM_SECURITY_CROWDSEC_API_KEY",
}

// pr4Exec records the cscli invocations and answers them with respond.
type pr4Exec struct {
	mu        sync.Mutex
	calls     [][]string
	remaining []time.Duration // time left until the context deadline at each call; -1 without a deadline
	respond   func(ctx context.Context, args []string) ([]byte, error)
}

func (e *pr4Exec) Execute(ctx context.Context, _ string, args ...string) ([]byte, error) {
	e.mu.Lock()
	e.calls = append(e.calls, append([]string(nil), args...))
	left := time.Duration(-1)
	if dl, ok := ctx.Deadline(); ok {
		left = time.Until(dl)
	}
	e.remaining = append(e.remaining, left)
	e.mu.Unlock()
	if e.respond == nil {
		return []byte{}, nil
	}
	return e.respond(ctx, args)
}

func (e *pr4Exec) lastCall(t *testing.T) []string {
	t.Helper()
	e.mu.Lock()
	defer e.mu.Unlock()
	require.NotEmpty(t, e.calls, "cscli was not called")
	return e.calls[len(e.calls)-1]
}

func (e *pr4Exec) callCount() int {
	e.mu.Lock()
	defer e.mu.Unlock()
	return len(e.calls)
}

func (e *pr4Exec) lastRemaining(t *testing.T) time.Duration {
	t.Helper()
	e.mu.Lock()
	defer e.mu.Unlock()
	require.NotEmpty(t, e.remaining, "cscli was not called")
	return e.remaining[len(e.remaining)-1]
}

// pr4Answer answers every command with the recorded output of one fixture.
func pr4Answer(t *testing.T, fixture string) *pr4Exec {
	t.Helper()
	out := cscliFixture(t, fixture)
	var err error
	if code := cscliFixtureExit(t, fixture); code != 0 {
		err = exitStatusError(t, code)
	}
	return &pr4Exec{respond: func(context.Context, []string) ([]byte, error) { return out, err }}
}

// pr4Blocking blocks until the command context ends. A killed process reports a generic exit error, as the real
// executor does, unless plain is set.
func pr4Blocking(plain bool) *pr4Exec {
	return &pr4Exec{respond: func(ctx context.Context, _ []string) ([]byte, error) {
		<-ctx.Done()
		if plain {
			return nil, ctx.Err()
		}
		return nil, errors.New("signal: killed")
	}}
}

// pr4StripConfig drops the leading "-c <config>" from cscli arguments.
func pr4StripConfig(args []string) []string {
	if len(args) >= 2 && args[0] == "-c" {
		return args[2:]
	}
	return args
}

// pr4Flag returns the value after flag in args.
func pr4Flag(args []string, flag string) (string, bool) {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == flag {
			return args[i+1], true
		}
	}
	return "", false
}

func pr4Handler(t *testing.T, ex CommandExecutor, dataDir string) *CrowdsecHandler {
	t.Helper()
	db := setupCrowdDB(t)
	require.NoError(t, db.AutoMigrate(&models.SecurityDecision{}, &models.Setting{}))
	h := newTestCrowdsecHandler(t, db, &fakeExec{}, "/bin/false", dataDir)
	h.CmdExec = ex
	return h
}

// pr4Router registers the real routes on a default gin engine.
func pr4Router(h *CrowdsecHandler) *gin.Engine {
	r := gin.New()
	h.RegisterRoutes(r.Group("/api/v1"))
	return r
}

func pr4Serve(r *gin.Engine, method, target, body string) *httptest.ResponseRecorder {
	return pr4ServeCtx(context.Background(), r, method, target, body)
}

func pr4ServeCtx(ctx context.Context, r *gin.Engine, method, target, body string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, target, strings.NewReader(body)).WithContext(ctx)
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func pr4Body(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &m), w.Body.String())
	return m
}

func pr4BanBody(t *testing.T, ip, duration, reason string) string {
	t.Helper()
	m := map[string]string{"ip": ip}
	if duration != "" {
		m["duration"] = duration
	}
	if reason != "" {
		m["reason"] = reason
	}
	b, err := json.Marshal(m)
	require.NoError(t, err)
	return string(b)
}

const pr4BanURL = "/api/v1/admin/crowdsec/ban"

// --- recorded facts about the bundled cscli (documents what the validation has to match) ---

func TestCSCLIContract_DecisionsAddFlags_RecordedFacts(t *testing.T) {
	t.Parallel()

	t.Run("addresses go with -i and ranges with -r", func(t *testing.T) {
		t.Parallel()
		for _, name := range []string{"decisions_add_ban", "decisions_add_ipv6", "decisions_add_range_r", "decisions_add_ipv6_range"} {
			assert.Zero(t, cscliFixtureExit(t, name), name)
		}
		for _, name := range []string{"decisions_add_range_via_i", "decisions_add_addr_via_r"} {
			assert.Equal(t, 1, cscliFixtureExit(t, name), name)
		}
		assert.Contains(t, string(cscliFixture(t, "decisions_add_range_via_i")), "is not a valid ip")
		assert.Contains(t, string(cscliFixture(t, "decisions_add_addr_via_r")), "is not a valid range")
	})

	t.Run("a value starting with a dash is taken as the flag value and refused", func(t *testing.T) {
		t.Parallel()
		for _, name := range []string{"decisions_add_leading_dash", "decisions_add_leading_dash_flag", "decisions_add_leading_dash_attached", "decisions_add_range_leading_dash"} {
			assert.Equal(t, 1, cscliFixtureExit(t, name), name)
			assert.Contains(t, string(cscliFixture(t, name)), "is not a valid", name)
		}
		// A reason that starts with a dash is accepted, so it never needs rejecting.
		assert.Zero(t, cscliFixtureExit(t, "decisions_add_reason_leading_dash"))
	})

	t.Run("invalid values", func(t *testing.T) {
		t.Parallel()
		for _, name := range []string{"decisions_add_badip", "decisions_add_badrange", "decisions_add_badrange_prefix", "decisions_add_zone"} {
			assert.Equal(t, 1, cscliFixtureExit(t, name), name)
		}
		for _, name := range []string{"decisions_add_empty_value", "decisions_add_no_value"} {
			assert.Equal(t, 1, cscliFixtureExit(t, name), name)
			assert.Contains(t, string(cscliFixture(t, name)), "a value is required", name)
		}
		// An unmasked range is accepted as given.
		assert.Zero(t, cscliFixtureExit(t, "decisions_add_unmasked_range"))
	})

	t.Run("duration forms", func(t *testing.T) {
		t.Parallel()
		accepted := []string{"4h", "30m", "1d", "90s", "1h30m", "0", "0s", "_1h", "10000d", "2_5h", "100000000d", "missing"}
		refused := []string{"invalid", "empty", "9999999h", "1w", "1h1d", "4_h", "upper_4H", "24h5"}
		for _, f := range accepted {
			assert.Zero(t, cscliFixtureExit(t, "decisions_add_dur_"+f), f)
		}
		for _, f := range refused {
			assert.Equal(t, 1, cscliFixtureExit(t, "decisions_add_dur_"+f), f)
			assert.Contains(t, string(cscliFixture(t, "decisions_add_dur_"+f)), "decision duration", f)
		}
	})

	t.Run("reasons", func(t *testing.T) {
		t.Parallel()
		for _, name := range []string{"decisions_add_reason_empty", "decisions_add_reason_newline", "decisions_add_reason_long", "decisions_add_ban2_noreason"} {
			assert.Zero(t, cscliFixtureExit(t, name), name)
		}
	})
}

func TestCSCLIContract_DecisionsDelete_RecordedFacts(t *testing.T) {
	t.Parallel()
	deleted := regexp.MustCompile(`(\d+) decision\(s\) deleted`)
	cases := map[string]string{
		"decisions_delete":               "1",
		"decisions_delete_ipv6":          "1",
		"decisions_delete_range_r":       "1",
		"decisions_delete_range_v6":      "1",
		"decisions_delete_one":           "1",
		"decisions_delete_two_for_one":   "3",
		"decisions_delete_missing":       "0",
		"decisions_delete_missing_v6":    "0",
		"decisions_delete_range_absent":  "0",
		"decisions_delete_range_missing": "1", // a wider stored range overlaps the one asked for
	}
	for name, want := range cases {
		m := deleted.FindStringSubmatch(string(cscliFixture(t, name)))
		require.Len(t, m, 2, name)
		assert.Equal(t, want, m[1], name)
		assert.Zero(t, cscliFixtureExit(t, name), "a delete that matches nothing still exits 0: "+name)
	}
	for _, name := range []string{"decisions_delete_range_via_i", "decisions_delete_addr_via_r", "decisions_delete_badip", "decisions_delete_badrange", "decisions_delete_leading_dash", "decisions_delete_range_leading_dash"} {
		assert.Equal(t, 1, cscliFixtureExit(t, name), name)
		assert.Contains(t, string(cscliFixture(t, name)), "is not a valid", name)
	}
	for _, name := range []string{"decisions_delete_no_value", "decisions_delete_empty_value"} {
		assert.Equal(t, 1, cscliFixtureExit(t, name), name)
		assert.Contains(t, string(cscliFixture(t, name)), "Usage:", name)
	}
}

// --- ban ---

func TestBanIP_FlagPerKind(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, ip, flag, value string
		pending               bool
	}{
		{name: "ipv4 address", ip: "192.0.2.10", flag: "-i", value: "192.0.2.10"},
		{name: "ipv6 address", ip: "2001:db8::1", flag: "-i", value: "2001:db8::1"},
		{name: "ipv4 range", ip: "198.51.100.0/24", flag: "-r", value: "198.51.100.0/24", pending: true},
		{name: "ipv6 range", ip: "2001:db8:1::/48", flag: "-r", value: "2001:db8:1::/48", pending: true},
		{name: "range is sent masked", ip: "192.0.2.77/24", flag: "-r", value: "192.0.2.0/24", pending: true},
		{name: "surrounding spaces are removed", ip: "  198.51.100.0/24 ", flag: "-r", value: "198.51.100.0/24", pending: true},
		{name: "surrounding spaces are removed from an address", ip: " 192.0.2.10\t", flag: "-i", value: "192.0.2.10"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if tc.pending {
				skipUntilLaterCommit(t)
			}
			ex := pr4Answer(t, "decisions_add_ban")
			w := pr4Serve(pr4Router(pr4Handler(t, ex, t.TempDir())), http.MethodPost, pr4BanURL, pr4BanBody(t, tc.ip, "4h", "example"))
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			args := pr4StripConfig(ex.lastCall(t))
			require.GreaterOrEqual(t, len(args), 2)
			assert.Equal(t, []string{"decisions", "add"}, args[:2])
			got, ok := pr4Flag(args, tc.flag)
			require.True(t, ok, "flag %s missing in %v", tc.flag, args)
			assert.Equal(t, tc.value, got)
			other := map[string]string{"-i": "-r", "-r": "-i"}[tc.flag]
			_, both := pr4Flag(args, other)
			assert.False(t, both, "only one of -i and -r is sent: %v", args)
			assert.Equal(t, "ban", mustFlag(t, args, "-t"))
		})
	}
}

func mustFlag(t *testing.T, args []string, flag string) string {
	t.Helper()
	v, ok := pr4Flag(args, flag)
	require.True(t, ok, "flag %s missing in %v", flag, args)
	return v
}

func TestBanIP_InvalidValueIsRefusedBeforeCscli(t *testing.T) {
	t.Parallel()
	cases := []struct{ name, ip string }{
		{"empty", ""},
		{"blank", "   "},
		{"not an address", "notanip"},
		{"too long", strings.Repeat("1", 65)},
		{"too long address-like", strings.Repeat("0123456789", 7)},
		{"leading dash", "-192.0.2.1"},
		{"flag lookalike", "-x"},
		{"long flag lookalike", "--help"},
		{"newline inside", "192.0.2.1\n198.51.100.1"},
		{"control character", "192.0.2.1\x07"},
		{"null byte", "192.0.2.1\x00"},
		{"zone", "fe80::1%eth0"},
		{"prefix too large", "192.0.2.0/33"},
		{"ipv6 prefix too large", "2001:db8::/129"},
		{"trailing text", "192.0.2.1 extra"},
		{"shell text", "192.0.2.1;ls"},
		{"two slashes", "192.0.2.0/24/24"},
		{"range with a dash", "-198.51.100.0/24"},
		{"host name", "example.invalid"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			skipUntilLaterCommit(t)
			ex := pr4Answer(t, "decisions_add_ban")
			w := pr4Serve(pr4Router(pr4Handler(t, ex, t.TempDir())), http.MethodPost, pr4BanURL, pr4BanBody(t, tc.ip, "4h", ""))
			require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
			body := pr4Body(t, w)
			assert.Equal(t, "invalid_ip", body["error_code"])
			msg, _ := body["error"].(string)
			assert.NotEmpty(t, msg, "the English text stays as the fallback")
			assert.Zero(t, ex.callCount(), "nothing reaches cscli")
		})
	}

	t.Run("the English text of an empty value is kept", func(t *testing.T) {
		t.Parallel()
		skipUntilLaterCommit(t)
		w := pr4Serve(pr4Router(pr4Handler(t, &pr4Exec{}, t.TempDir())), http.MethodPost, pr4BanURL, pr4BanBody(t, "   ", "", ""))
		require.Equal(t, http.StatusBadRequest, w.Code)
		body := pr4Body(t, w)
		assert.Equal(t, "invalid_ip", body["error_code"])
		assert.Equal(t, "ip cannot be empty", body["error"])
	})
}

func TestBanIP_DurationForms(t *testing.T) {
	t.Parallel()
	// Accepted: Go durations and the day form cscli takes, from one minute to 365 days.
	accepted := []string{"4h", "30m", "1d", "90s", "1h30m", "2.5h", "1m", "60s", "365d", "8760h", "24h"}
	for _, d := range accepted {
		t.Run("accepts "+d, func(t *testing.T) {
			t.Parallel()
			ex := pr4Answer(t, "decisions_add_ban")
			w := pr4Serve(pr4Router(pr4Handler(t, ex, t.TempDir())), http.MethodPost, pr4BanURL, pr4BanBody(t, "192.0.2.10", d, ""))
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			args := pr4StripConfig(ex.lastCall(t))
			assert.Equal(t, d, mustFlag(t, args, "-d"))
		})
	}

	refused := []string{
		"0", "0s", "-1h", "-4h", "59s", "30s", "1ms", "366d", "8761h", "9999999h", "10000d", "100000000d",
		"1w", "1h1d", "4 h", "4H", "24h5", "notaduration", "-x", "--help", "4h\n", "4h;ls", "1e3h", "NaN",
	}
	for _, d := range refused {
		t.Run("refuses "+d, func(t *testing.T) {
			t.Parallel()
			skipUntilLaterCommit(t)
			ex := pr4Answer(t, "decisions_add_ban")
			w := pr4Serve(pr4Router(pr4Handler(t, ex, t.TempDir())), http.MethodPost, pr4BanURL, pr4BanBody(t, "192.0.2.10", d, ""))
			require.Equal(t, http.StatusBadRequest, w.Code, w.Body.String())
			body := pr4Body(t, w)
			assert.Equal(t, "invalid_duration", body["error_code"])
			assert.NotEmpty(t, body["error"])
			assert.Zero(t, ex.callCount(), "nothing reaches cscli")
		})
	}

	t.Run("no duration means 24h", func(t *testing.T) {
		t.Parallel()
		ex := pr4Answer(t, "decisions_add_ban")
		w := pr4Serve(pr4Router(pr4Handler(t, ex, t.TempDir())), http.MethodPost, pr4BanURL, pr4BanBody(t, "192.0.2.10", "", ""))
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Equal(t, "24h", mustFlag(t, pr4StripConfig(ex.lastCall(t)), "-d"))
		assert.Equal(t, "24h", pr4Body(t, w)["duration"])
	})
}

func TestBanIP_Reason(t *testing.T) {
	t.Parallel()

	t.Run("up to 200 characters", func(t *testing.T) {
		t.Parallel()
		for name, reason := range map[string]string{"ascii": strings.Repeat("a", 200), "multibyte": strings.Repeat("é", 200)} {
			ex := pr4Answer(t, "decisions_add_ban")
			w := pr4Serve(pr4Router(pr4Handler(t, ex, t.TempDir())), http.MethodPost, pr4BanURL, pr4BanBody(t, "192.0.2.10", "1h", reason))
			require.Equal(t, http.StatusOK, w.Code, name+": "+w.Body.String())
			assert.Equal(t, "manual ban: "+reason, mustFlag(t, pr4StripConfig(ex.lastCall(t)), "-R"), name)
		}
	})

	t.Run("longer than 200 characters is refused", func(t *testing.T) {
		t.Parallel()
		skipUntilLaterCommit(t)
		for name, reason := range map[string]string{"ascii": strings.Repeat("a", 201), "multibyte": strings.Repeat("é", 201)} {
			ex := pr4Answer(t, "decisions_add_ban")
			w := pr4Serve(pr4Router(pr4Handler(t, ex, t.TempDir())), http.MethodPost, pr4BanURL, pr4BanBody(t, "192.0.2.10", "1h", reason))
			require.Equal(t, http.StatusBadRequest, w.Code, name+": "+w.Body.String())
			body := pr4Body(t, w)
			assert.Equal(t, "invalid_reason", body["error_code"], name)
			assert.NotEmpty(t, body["error"], name)
			assert.Zero(t, ex.callCount(), name)
		}
	})

	t.Run("control characters are removed", func(t *testing.T) {
		t.Parallel()
		skipUntilLaterCommit(t)
		ex := pr4Answer(t, "decisions_add_ban")
		w := pr4Serve(pr4Router(pr4Handler(t, ex, t.TempDir())), http.MethodPost, pr4BanURL, pr4BanBody(t, "192.0.2.10", "1h", "line1\nline2\x00\x1b[31m\ttab"))
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		got := mustFlag(t, pr4StripConfig(ex.lastCall(t)), "-R")
		assert.True(t, strings.HasPrefix(got, "manual ban: line1"), got)
		for _, r := range got {
			assert.False(t, r < 0x20 || r == 0x7f, "control character %q left in %q", r, got)
		}
	})

	t.Run("a reason that starts with a dash is harmless", func(t *testing.T) {
		t.Parallel()
		ex := pr4Answer(t, "decisions_add_ban")
		w := pr4Serve(pr4Router(pr4Handler(t, ex, t.TempDir())), http.MethodPost, pr4BanURL, pr4BanBody(t, "192.0.2.10", "1h", "-x"))
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Equal(t, "manual ban: -x", mustFlag(t, pr4StripConfig(ex.lastCall(t)), "-R"))
	})

	t.Run("no reason", func(t *testing.T) {
		t.Parallel()
		ex := pr4Answer(t, "decisions_add_ban")
		w := pr4Serve(pr4Router(pr4Handler(t, ex, t.TempDir())), http.MethodPost, pr4BanURL, pr4BanBody(t, "192.0.2.10", "1h", ""))
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Equal(t, "manual ban", mustFlag(t, pr4StripConfig(ex.lastCall(t)), "-R"))
	})
}

func TestBanIP_CscliFailureAndAuditRow(t *testing.T) {
	t.Parallel()

	rows := func(t *testing.T, h *CrowdsecHandler) []models.SecurityDecision {
		t.Helper()
		var out []models.SecurityDecision
		require.NoError(t, h.DB.Find(&out).Error)
		return out
	}

	t.Run("a cscli failure keeps the 500 and writes no audit row", func(t *testing.T) {
		t.Parallel()
		ex := pr4Answer(t, "decisions_add_badip")
		h := pr4Handler(t, ex, t.TempDir())
		w := pr4Serve(pr4Router(h), http.MethodPost, pr4BanURL, pr4BanBody(t, "192.0.2.10", "4h", ""))
		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.JSONEq(t, `{"error":"failed to ban IP"}`, w.Body.String())
		assert.Empty(t, rows(t, h))
	})

	t.Run("the audit row is written after cscli succeeds", func(t *testing.T) {
		t.Parallel()
		h := pr4Handler(t, pr4Answer(t, "decisions_add_ban"), t.TempDir())
		w := pr4Serve(pr4Router(h), http.MethodPost, pr4BanURL, pr4BanBody(t, "192.0.2.10", "4h", "example"))
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		got := rows(t, h)
		require.Len(t, got, 1)
		assert.Equal(t, "192.0.2.10", got[0].IP)
		assert.Equal(t, "manual ban: example", got[0].RuleID)
	})

	for _, tc := range []struct {
		duration string
		want     time.Duration
		pending  bool
	}{
		{"4h", 4 * time.Hour, false},
		{"90s", 90 * time.Second, false},
		{"1d", 24 * time.Hour, true},
		{"365d", 365 * 24 * time.Hour, true},
	} {
		t.Run("expiry from "+tc.duration, func(t *testing.T) {
			t.Parallel()
			if tc.pending {
				skipUntilLaterCommit(t)
			}
			h := pr4Handler(t, pr4Answer(t, "decisions_add_ban"), t.TempDir())
			start := time.Now()
			w := pr4Serve(pr4Router(h), http.MethodPost, pr4BanURL, pr4BanBody(t, "192.0.2.10", tc.duration, ""))
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			got := rows(t, h)
			require.Len(t, got, 1)
			require.NotNil(t, got[0].ExpiresAt)
			assert.WithinDuration(t, start.Add(tc.want), *got[0].ExpiresAt, 30*time.Second)
		})
	}

	t.Run("a range is recorded as given", func(t *testing.T) {
		t.Parallel()
		skipUntilLaterCommit(t)
		h := pr4Handler(t, pr4Answer(t, "decisions_add_range_r"), t.TempDir())
		w := pr4Serve(pr4Router(h), http.MethodPost, pr4BanURL, pr4BanBody(t, "198.51.100.0/24", "4h", ""))
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		got := rows(t, h)
		require.Len(t, got, 1)
		assert.Equal(t, "198.51.100.0/24", got[0].IP)
		assert.Equal(t, "198.51.100.0/24", pr4Body(t, w)["ip"])
	})
}

// --- unban ---

func TestUnbanIP_Route(t *testing.T) {
	t.Parallel()
	del := func(r *gin.Engine, value string) *httptest.ResponseRecorder {
		return pr4Serve(r, http.MethodDelete, pr4BanURL+"/"+value, "")
	}

	cases := []struct {
		name, target, flag, value, fixture string
		deleted                            float64
		pending                            bool
	}{
		{name: "single address", target: "192.0.2.11", flag: "-i", value: "192.0.2.11", fixture: "decisions_delete", deleted: 1, pending: true},
		{name: "ipv6 address plain", target: "2001:db8::1", flag: "-i", value: "2001:db8::1", fixture: "decisions_delete_ipv6", deleted: 1, pending: true},
		{name: "ipv6 address encoded", target: url.PathEscape("2001:db8::1"), flag: "-i", value: "2001:db8::1", fixture: "decisions_delete_ipv6", deleted: 1, pending: true},
		{name: "range encoded", target: "198.51.100.0%2F24", flag: "-r", value: "198.51.100.0/24", fixture: "decisions_delete_range_r", deleted: 1, pending: true},
		{name: "range plain", target: "198.51.100.0/24", flag: "-r", value: "198.51.100.0/24", fixture: "decisions_delete_range_r", deleted: 1, pending: true},
		{name: "ipv6 range encoded", target: "2001%3Adb8%3A1%3A%3A%2F48", flag: "-r", value: "2001:db8:1::/48", fixture: "decisions_delete_range_v6", deleted: 1, pending: true},
		{name: "ipv6 range plain", target: "2001:db8:1::/48", flag: "-r", value: "2001:db8:1::/48", fixture: "decisions_delete_range_v6", deleted: 1, pending: true},
		{name: "range is sent masked", target: "192.0.2.77%2F24", flag: "-r", value: "192.0.2.0/24", fixture: "decisions_delete_range_r", deleted: 1, pending: true},
		{name: "several decisions removed", target: "192.0.2.201", flag: "-i", value: "192.0.2.201", fixture: "decisions_delete_two_for_one", deleted: 3, pending: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if tc.pending {
				skipUntilLaterCommit(t)
			}
			ex := pr4Answer(t, tc.fixture)
			w := del(pr4Router(pr4Handler(t, ex, t.TempDir())), tc.target)
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			args := pr4StripConfig(ex.lastCall(t))
			assert.Equal(t, []string{"decisions", "delete", tc.flag, tc.value}, args)
			body := pr4Body(t, w)
			assert.Equal(t, "unbanned", body["status"])
			assert.Equal(t, tc.deleted, body["deleted"])
		})
	}

	t.Run("nothing deleted is a 404 with a code", func(t *testing.T) {
		t.Parallel()
		skipUntilLaterCommit(t)
		for name, tc := range map[string]struct{ target, fixture string }{
			"address":    {"192.0.2.250", "decisions_delete_missing"},
			"ipv6":       {"2001:db8::99", "decisions_delete_missing_v6"},
			"range":      {"198.51.100.0%2F24", "decisions_delete_range_absent"},
			"range path": {"198.51.100.0/24", "decisions_delete_range_absent"},
		} {
			w := del(pr4Router(pr4Handler(t, pr4Answer(t, tc.fixture), t.TempDir())), tc.target)
			require.Equal(t, http.StatusNotFound, w.Code, name+": "+w.Body.String())
			body := pr4Body(t, w)
			assert.Equal(t, "decision_not_found", body["error_code"], name)
			assert.NotEmpty(t, body["error"], name)
		}
	})

	t.Run("invalid values are refused before cscli", func(t *testing.T) {
		t.Parallel()
		skipUntilLaterCommit(t)
		for name, target := range map[string]string{
			"not an address":     "notanip",
			"flag lookalike":     "-x",
			"long flag":          "--help",
			"leading dash":       "-192.0.2.1",
			"range leading dash": "-198.51.100.0%2F24",
			"newline":            "192.0.2.1%0A198.51.100.1",
			"control":            "192.0.2.1%07",
			"zone":               "fe80::1%25eth0",
			"prefix too large":   "192.0.2.0%2F33",
			"too long":           strings.Repeat("1", 65),
			"two slashes":        "192.0.2.0/24/24",
			"nothing":            "",
		} {
			ex := pr4Answer(t, "decisions_delete")
			w := del(pr4Router(pr4Handler(t, ex, t.TempDir())), target)
			require.Equal(t, http.StatusBadRequest, w.Code, name+": "+w.Body.String())
			body := pr4Body(t, w)
			assert.Equal(t, "invalid_ip", body["error_code"], name)
			assert.NotEmpty(t, body["error"], name)
			assert.Zero(t, ex.callCount(), name)
		}
	})

	t.Run("a cscli failure keeps the 500", func(t *testing.T) {
		t.Parallel()
		w := del(pr4Router(pr4Handler(t, pr4Answer(t, "decisions_delete_badip"), t.TempDir())), "192.0.2.11")
		assert.Equal(t, http.StatusInternalServerError, w.Code)
		assert.JSONEq(t, `{"error":"failed to unban IP"}`, w.Body.String())
	})
}

// --- the cscli helper: config argument and deadline ---

func TestCscliCalls_ConfigArgument(t *testing.T) {
	t.Parallel()

	layouts := map[string]string{
		"config in the data directory":   "config.yaml",
		"config in the config directory": filepath.Join("config", "config.yaml"),
	}
	calls := map[string]struct {
		run     func(r *gin.Engine) *httptest.ResponseRecorder
		fixture string
	}{
		"ban": {
			run: func(r *gin.Engine) *httptest.ResponseRecorder {
				return pr4Serve(r, http.MethodPost, pr4BanURL, `{"ip":"192.0.2.10","duration":"4h"}`)
			},
			fixture: "decisions_add_ban",
		},
		"unban": {
			run: func(r *gin.Engine) *httptest.ResponseRecorder {
				return pr4Serve(r, http.MethodDelete, pr4BanURL+"/192.0.2.11", "")
			},
			fixture: "decisions_delete",
		},
		"decisions list": {
			run: func(r *gin.Engine) *httptest.ResponseRecorder {
				return pr4Serve(r, http.MethodGet, "/api/v1/admin/crowdsec/decisions", "")
			},
			fixture: "decisions_list_empty_json",
		},
		"alerts list": {
			run: func(r *gin.Engine) *httptest.ResponseRecorder {
				return pr4Serve(r, http.MethodGet, "/api/v1/admin/crowdsec/alerts?limit=50", "")
			},
			fixture: "alerts_list_json",
		},
	}

	for callName, call := range calls {
		for layoutName, rel := range layouts {
			t.Run(callName+"/"+layoutName, func(t *testing.T) {
				t.Parallel()
				dir := t.TempDir()
				cfg := filepath.Join(dir, rel)
				require.NoError(t, os.MkdirAll(filepath.Dir(cfg), 0o750))
				require.NoError(t, os.WriteFile(cfg, []byte("common:\n  log_level: info\n"), 0o600))
				ex := pr4Answer(t, call.fixture)
				h := pr4Handler(t, ex, dir)
				h.validateLAPIURL = func(string) (*url.URL, error) { return nil, errors.New("blocked in test") }
				w := call.run(pr4Router(h))
				require.Equal(t, http.StatusOK, w.Code, w.Body.String())
				args := ex.lastCall(t)
				require.GreaterOrEqual(t, len(args), 3)
				assert.Equal(t, []string{"-c", cfg}, args[:2])
			})
		}

		t.Run(callName+"/no config", func(t *testing.T) {
			t.Parallel()
			ex := pr4Answer(t, call.fixture)
			h := pr4Handler(t, ex, t.TempDir())
			h.validateLAPIURL = func(string) (*url.URL, error) { return nil, errors.New("blocked in test") }
			w := call.run(pr4Router(h))
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			assert.NotEqual(t, "-c", ex.lastCall(t)[0])
		})
	}

	t.Run("the config directory wins when both exist", func(t *testing.T) {
		t.Parallel()
		dir := t.TempDir()
		require.NoError(t, os.MkdirAll(filepath.Join(dir, "config"), 0o750))
		for _, p := range []string{filepath.Join(dir, "config.yaml"), filepath.Join(dir, "config", "config.yaml")} {
			require.NoError(t, os.WriteFile(p, []byte("common:\n"), 0o600))
		}
		ex := pr4Answer(t, "decisions_delete")
		w := pr4Serve(pr4Router(pr4Handler(t, ex, dir)), http.MethodDelete, pr4BanURL+"/192.0.2.11", "")
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Equal(t, []string{"-c", filepath.Join(dir, "config", "config.yaml")}, ex.lastCall(t)[:2])
	})
}

func TestCscliCalls_RunUnderADeadline(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name    string
		run     func(r *gin.Engine) *httptest.ResponseRecorder
		fixture string
		max     time.Duration
	}{
		{"ban", func(r *gin.Engine) *httptest.ResponseRecorder {
			return pr4Serve(r, http.MethodPost, pr4BanURL, `{"ip":"192.0.2.10","duration":"1h"}`)
		}, "decisions_add_ban", pr4EditDeadline},
		{"unban", func(r *gin.Engine) *httptest.ResponseRecorder {
			return pr4Serve(r, http.MethodDelete, pr4BanURL+"/192.0.2.11", "")
		}, "decisions_delete", pr4EditDeadline},
		{"decisions list", func(r *gin.Engine) *httptest.ResponseRecorder {
			return pr4Serve(r, http.MethodGet, "/api/v1/admin/crowdsec/decisions", "")
		}, "decisions_list_empty_json", pr4ListDeadline},
		{"alerts list", func(r *gin.Engine) *httptest.ResponseRecorder {
			return pr4Serve(r, http.MethodGet, "/api/v1/admin/crowdsec/alerts", "")
		}, "alerts_list_json", pr4ListDeadline},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			ex := pr4Answer(t, tc.fixture)
			h := pr4Handler(t, ex, t.TempDir())
			h.validateLAPIURL = func(string) (*url.URL, error) { return nil, errors.New("blocked in test") }
			w := tc.run(pr4Router(h))
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			left := ex.lastRemaining(t)
			assert.Positive(t, left, "the cscli context has no deadline")
			assert.LessOrEqual(t, left, tc.max)
		})
	}
}

func TestBanAndUnban_DeadlineGivesCodedTimeout(t *testing.T) {
	t.Parallel()
	requests := map[string]func(r *gin.Engine, ctx context.Context) *httptest.ResponseRecorder{
		"ban": func(r *gin.Engine, ctx context.Context) *httptest.ResponseRecorder {
			return pr4ServeCtx(ctx, r, http.MethodPost, pr4BanURL, `{"ip":"192.0.2.10","duration":"1h"}`)
		},
		"unban": func(r *gin.Engine, ctx context.Context) *httptest.ResponseRecorder {
			return pr4ServeCtx(ctx, r, http.MethodDelete, pr4BanURL+"/192.0.2.11", "")
		},
	}
	for name, run := range requests {
		for _, plain := range []bool{true, false} {
			label := name + " with a killed process"
			if plain {
				label = name + " with the context error"
			}
			t.Run(label, func(t *testing.T) {
				t.Parallel()
				ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
				defer cancel()
				start := time.Now()
				w := run(pr4Router(pr4Handler(t, pr4Blocking(plain), t.TempDir())), ctx)
				assert.Less(t, time.Since(start), 5*time.Second)
				require.Equal(t, http.StatusGatewayTimeout, w.Code, w.Body.String())
				body := pr4Body(t, w)
				assert.Equal(t, "cscli_timeout", body["error_code"])
				assert.Equal(t, "CrowdSec did not answer in time", body["error"])
			})
		}
	}
}

// --- the saved bouncer key ---

// pr4KeyServer answers like the local API for a bouncer key: the right key gets body, any other key a 403; "*" accepts every key.
func pr4KeyServer(t *testing.T, wantKey string, body []byte) (srv *httptest.Server, seenKeys func() []string) {
	t.Helper()
	var mu sync.Mutex
	var seen []string
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, r.Header.Get("X-Api-Key"))
		mu.Unlock()
		if wantKey != "*" && r.Header.Get("X-Api-Key") != wantKey {
			w.WriteHeader(http.StatusForbidden)
			return
		}
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv, func() []string {
		mu.Lock()
		defer mu.Unlock()
		return append([]string(nil), seen...)
	}
}

func pr4ClearKeyEnv(t *testing.T) {
	t.Helper()
	for _, name := range pr4KeyEnvInRuntimeOrder {
		t.Setenv(name, "")
	}
}

func pr4LAPIHandler(t *testing.T, lapiURL string, ex CommandExecutor) *CrowdsecHandler {
	t.Helper()
	h := pr4Handler(t, ex, t.TempDir())
	require.NoError(t, h.DB.Create(&models.SecurityConfig{UUID: "default", Name: "default", CrowdSecAPIURL: lapiURL}).Error)
	h.validateLAPIURL = url.Parse
	return h
}

func pr4WriteKeyFile(t *testing.T, h *CrowdsecHandler, content string) {
	t.Helper()
	require.NoError(t, os.WriteFile(filepath.Join(h.DataDir, "bouncer_key"), []byte(content), 0o600))
}

// pr4KeyUses are the three places the key is used: the decisions call, the dashboard count and the alerts call.
var pr4KeyUses = map[string]struct {
	target string
	body   func(t *testing.T) []byte
	check  func(t *testing.T, w *httptest.ResponseRecorder)
}{
	"GetLAPIDecisions": {
		target: "/api/v1/admin/crowdsec/decisions/lapi",
		body:   func(t *testing.T) []byte { return lapiFixture(t, "lapi_v1_decisions") },
		check: func(t *testing.T, w *httptest.ResponseRecorder) {
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			assert.Equal(t, "lapi", pr4Body(t, w)["source"])
		},
	},
	"dashboard active decisions": {
		target: "/api/v1/admin/crowdsec/dashboard/summary?range=1h",
		body:   func(t *testing.T) []byte { return lapiFixture(t, "lapi_v1_decisions") },
		check: func(t *testing.T, w *httptest.ResponseRecorder) {
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			assert.Equal(t, float64(2), pr4Body(t, w)["active_decisions"])
		},
	},
	"alerts": {
		target: "/api/v1/admin/crowdsec/alerts?limit=50",
		body:   func(t *testing.T) []byte { return lapiFixture(t, "lapi_v1_alerts") },
		check: func(t *testing.T, w *httptest.ResponseRecorder) {
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			assert.Equal(t, "lapi", pr4Body(t, w)["source"])
		},
	},
}

func TestLAPIKey_SavedKeyFileFirst(t *testing.T) {
	for name, use := range pr4KeyUses {
		t.Run(name, func(t *testing.T) {
			skipUntilLaterCommit(t)
			pr4ClearKeyEnv(t)
			t.Setenv("CROWDSEC_API_KEY", "env-key-not-used")
			t.Setenv("CHARON_SECURITY_CROWDSEC_API_KEY", "charon-env-key-not-used")
			srv, seen := pr4KeyServer(t, "saved-key", use.body(t))
			h := pr4LAPIHandler(t, srv.URL, &pr4Exec{})
			pr4WriteKeyFile(t, h, "saved-key\n")
			use.check(t, pr4Serve(pr4Router(h), http.MethodGet, use.target, ""))
			require.NotEmpty(t, seen())
			assert.Equal(t, "saved-key", seen()[0])
		})
	}

	t.Run("an empty key file falls back to the environment", func(t *testing.T) {
		pr4ClearKeyEnv(t)
		t.Setenv("CROWDSEC_API_KEY", "env-key")
		srv, seen := pr4KeyServer(t, "env-key", lapiFixture(t, "lapi_v1_decisions"))
		h := pr4LAPIHandler(t, srv.URL, &pr4Exec{})
		pr4WriteKeyFile(t, h, " \n")
		w := pr4Serve(pr4Router(h), http.MethodGet, "/api/v1/admin/crowdsec/decisions/lapi", "")
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Equal(t, []string{"env-key"}, seen())
	})

	t.Run("no key at all sends no header", func(t *testing.T) {
		pr4ClearKeyEnv(t)
		srv, seen := pr4KeyServer(t, "", lapiFixture(t, "lapi_v1_decisions"))
		h := pr4LAPIHandler(t, srv.URL, &pr4Exec{})
		w := pr4Serve(pr4Router(h), http.MethodGet, "/api/v1/admin/crowdsec/decisions/lapi", "")
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Equal(t, []string{""}, seen())
	})
}

func TestLAPIKey_EnvironmentInRuntimeOrder(t *testing.T) {
	// The runtime (caddy/config.go) reads CHARON_SECURITY_CROWDSEC_API_KEY first.
	for i, first := range pr4KeyEnvInRuntimeOrder {
		t.Run("first set is "+first, func(t *testing.T) {
			if i == 0 {
				skipUntilLaterCommit(t)
			}
			pr4ClearKeyEnv(t)
			for _, name := range pr4KeyEnvInRuntimeOrder[i:] {
				t.Setenv(name, "key-of-"+name)
			}
			srv, seen := pr4KeyServer(t, "key-of-"+first, lapiFixture(t, "lapi_v1_decisions"))
			h := pr4LAPIHandler(t, srv.URL, &pr4Exec{})
			w := pr4Serve(pr4Router(h), http.MethodGet, "/api/v1/admin/crowdsec/decisions/lapi", "")
			require.Equal(t, http.StatusOK, w.Code, w.Body.String())
			assert.Equal(t, []string{"key-of-" + first}, seen())
		})
	}

	t.Run("a rejected key still gives the N/A value", func(t *testing.T) {
		pr4ClearKeyEnv(t)
		t.Setenv("CROWDSEC_API_KEY", "wrong-key")
		srv, _ := pr4KeyServer(t, "right-key", lapiFixture(t, "lapi_v1_decisions"))
		h := pr4LAPIHandler(t, srv.URL, &pr4Exec{})
		w := pr4Serve(pr4Router(h), http.MethodGet, "/api/v1/admin/crowdsec/dashboard/summary?range=1h", "")
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Equal(t, float64(-1), pr4Body(t, w)["active_decisions"])
	})
}

// --- bounded cscli output ---

func TestOversizedCscliOutput_SameSoftError(t *testing.T) {
	t.Parallel()
	over := append([]byte("[]\n"), make([]byte, pr4OverCap)...)
	for i := 3; i < len(over); i++ {
		over[i] = ' '
	}

	decisions := func(t *testing.T, ex CommandExecutor) *httptest.ResponseRecorder {
		t.Helper()
		return pr4Serve(pr4Router(pr4Handler(t, ex, t.TempDir())), http.MethodGet, "/api/v1/admin/crowdsec/decisions", "")
	}

	t.Run("decisions list: output above the parser limit gives the executor-cap answer", func(t *testing.T) {
		t.Parallel()
		skipUntilLaterCommit(t)
		capped := decisions(t, &pr4Exec{respond: func(context.Context, []string) ([]byte, error) { return []byte("[]\n"), crowdsec.ErrOutputLimit }})
		require.Equal(t, http.StatusOK, capped.Code, capped.Body.String())
		require.NotEmpty(t, pr4Body(t, capped)["error"])

		w := decisions(t, &pr4Exec{respond: func(context.Context, []string) ([]byte, error) { return over, nil }})
		require.Equal(t, http.StatusOK, w.Code, "the parser limit gives the same soft error as the executor limit")
		assert.JSONEq(t, capped.Body.String(), w.Body.String())
	})

	t.Run("decisions list: output within the limit still parses", func(t *testing.T) {
		t.Parallel()
		within := append([]byte("[]\n"), make([]byte, 1<<20)...)
		for i := 3; i < len(within); i++ {
			within[i] = ' '
		}
		w := decisions(t, &pr4Exec{respond: func(context.Context, []string) ([]byte, error) { return within, nil }})
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Empty(t, pr4Body(t, w)["error"])
	})

	t.Run("alerts list: oversized output gives an empty list", func(t *testing.T) {
		t.Parallel()
		h := pr4Handler(t, &pr4Exec{respond: func(context.Context, []string) ([]byte, error) { return over, nil }}, t.TempDir())
		h.validateLAPIURL = func(string) (*url.URL, error) { return nil, errors.New("blocked in test") }
		w := pr4Serve(pr4Router(h), http.MethodGet, "/api/v1/admin/crowdsec/alerts", "")
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		body := pr4Body(t, w)
		assert.Equal(t, float64(0), body["total"])
		assert.Empty(t, body["alerts"])
	})

	t.Run("bouncer check: oversized output is not a registered bouncer", func(t *testing.T) {
		t.Parallel()
		h := pr4Handler(t, &pr4Exec{respond: func(context.Context, []string) ([]byte, error) { return over, nil }}, t.TempDir())
		assert.False(t, h.validateBouncerKey(t.Context()))
	})
}

// --- the alerts endpoint returns a typed view ---

// pr4AlertFields are the fields the UI reads (frontend/src/api/crowdsecDashboard.ts, CrowdSecAlert).
var pr4AlertFields = []string{"id", "scenario", "ip", "message", "events_count", "start_at", "stop_at", "created_at", "duration", "type", "origin"}

func pr4AlertList(t *testing.T, w *httptest.ResponseRecorder) []map[string]any {
	t.Helper()
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	var body struct {
		Alerts []map[string]any `json:"alerts"`
		Total  int              `json:"total"`
		Source string           `json:"source"`
	}
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &body))
	assert.Equal(t, len(body.Alerts), body.Total)
	return body.Alerts
}

func pr4RequireTypedAlert(t *testing.T, got map[string]any) {
	t.Helper()
	allowed := map[string]bool{}
	for _, f := range pr4AlertFields {
		allowed[f] = true
	}
	for k := range got {
		assert.True(t, allowed[k], "field %q is not part of the alert view", k)
	}
	for _, banned := range []string{"machine_id", "uuid", "decisions", "source", "labels", "events", "leakspeed", "scenario_hash", "scenario_version", "capacity", "kind", "remediation", "simulated"} {
		assert.NotContains(t, got, banned)
	}
}

func TestAlertsEndpoint_TypedView(t *testing.T) {
	t.Parallel()

	wantFirst := map[string]any{
		"id":           float64(3),
		"scenario":     "manual ban: range",
		"ip":           "198.51.100.0/24",
		"message":      "manual ban: range",
		"events_count": float64(1),
		"start_at":     "2026-10-16T23:15:34Z",
		"stop_at":      "2026-10-16T23:15:34Z",
		"created_at":   "2026-10-16T23:15:34Z",
		"duration":     "1h59m59s",
		"type":         "ban",
		"origin":       "cscli",
	}

	t.Run("cscli path", func(t *testing.T) {
		t.Parallel()
		skipUntilLaterCommit(t)
		ex := newFixtureCmdExec(t, map[string]string{"alerts list -o json -l 50": "alerts_list_json"})
		h := pr4Handler(t, ex, t.TempDir())
		h.validateLAPIURL = func(string) (*url.URL, error) { return nil, errors.New("blocked in test") }
		alerts := pr4AlertList(t, pr4Serve(pr4Router(h), http.MethodGet, "/api/v1/admin/crowdsec/alerts?limit=50", ""))
		require.Len(t, alerts, 3)
		assert.Equal(t, wantFirst, alerts[0])
		for _, a := range alerts {
			pr4RequireTypedAlert(t, a)
		}
	})

	t.Run("cscli path: alert with ipv6 source and no decisions", func(t *testing.T) {
		t.Parallel()
		skipUntilLaterCommit(t)
		raw := `[{"id":9,"scenario":"s","message":"m","events_count":4,"machine_id":"testlocal","uuid":"u","decisions":null,"created_at":"2026-10-16T23:15:34Z","start_at":"2026-10-16T23:15:00Z","stop_at":"2026-10-16T23:15:30Z","source":{"ip":"2001:db8::1","scope":"Ip","value":"2001:db8::1"}}]`
		h := pr4Handler(t, &pr4Exec{respond: func(context.Context, []string) ([]byte, error) { return []byte(raw), nil }}, t.TempDir())
		h.validateLAPIURL = func(string) (*url.URL, error) { return nil, errors.New("blocked in test") }
		alerts := pr4AlertList(t, pr4Serve(pr4Router(h), http.MethodGet, "/api/v1/admin/crowdsec/alerts", ""))
		require.Len(t, alerts, 1)
		assert.Equal(t, "2001:db8::1", alerts[0]["ip"])
		assert.Equal(t, float64(4), alerts[0]["events_count"])
		assert.Equal(t, float64(9), alerts[0]["id"])
		pr4RequireTypedAlert(t, alerts[0])
	})

	t.Run("local API path", func(t *testing.T) {
		t.Parallel()
		skipUntilLaterCommit(t)
		srv, _ := pr4KeyServer(t, "*", lapiFixture(t, "lapi_v1_alerts"))
		h := pr4LAPIHandler(t, srv.URL, &pr4Exec{})
		alerts := pr4AlertList(t, pr4Serve(pr4Router(h), http.MethodGet, "/api/v1/admin/crowdsec/alerts?limit=50", ""))
		require.Len(t, alerts, 2)
		for _, a := range alerts {
			pr4RequireTypedAlert(t, a)
			for _, f := range pr4AlertFields {
				assert.Contains(t, a, f)
			}
		}
		assert.Equal(t, "manual-ban", alerts[0]["scenario"])
		assert.Equal(t, "ban", alerts[0]["type"])
		assert.Equal(t, "cscli", alerts[0]["origin"])
	})

	t.Run("local API path: ipv6 source", func(t *testing.T) {
		t.Parallel()
		skipUntilLaterCommit(t)
		srv, _ := pr4KeyServer(t, "*", lapiFixture(t, "lapi_v1_alerts_ipv6"))
		h := pr4LAPIHandler(t, srv.URL, &pr4Exec{})
		alerts := pr4AlertList(t, pr4Serve(pr4Router(h), http.MethodGet, "/api/v1/admin/crowdsec/alerts", ""))
		require.Len(t, alerts, 1)
		assert.Equal(t, "2001:db8::1", alerts[0]["ip"])
		pr4RequireTypedAlert(t, alerts[0])
	})

	t.Run("empty answers", func(t *testing.T) {
		t.Parallel()
		srv, _ := pr4KeyServer(t, "*", lapiFixture(t, "lapi_v1_alerts_empty"))
		h := pr4LAPIHandler(t, srv.URL, &pr4Exec{})
		w := pr4Serve(pr4Router(h), http.MethodGet, "/api/v1/admin/crowdsec/alerts", "")
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		body := pr4Body(t, w)
		assert.Equal(t, float64(0), body["total"])
		assert.Empty(t, body["alerts"])
	})
}

func TestAlertsEndpoint_RealLocalAPIRefusesABouncerKey(t *testing.T) {
	t.Parallel()
	// Recorded: GET /v1/alerts needs a machine login; a bouncer key gets 401. The endpoint then falls back to
	// cscli, so the alerts always come from the cscli path against a real local API.
	recorded := string(cscliFixture(t, "lapi_v1_alerts_bouncer_key"))
	require.Contains(t, recorded, "401 Unauthorized")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"code":401,"message":"cookie token is empty"}`))
	}))
	t.Cleanup(srv.Close)
	ex := newFixtureCmdExec(t, map[string]string{"alerts list -o json -l 50": "alerts_list_json"})
	h := pr4LAPIHandler(t, srv.URL, ex)
	w := pr4Serve(pr4Router(h), http.MethodGet, "/api/v1/admin/crowdsec/alerts?limit=50", "")
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())
	body := pr4Body(t, w)
	assert.Equal(t, "cscli", body["source"])
	assert.Equal(t, float64(3), body["total"])
}

func TestAlertsEndpoint_CscliFlagsAndFailures(t *testing.T) {
	t.Parallel()

	t.Run("scenario and limit reach cscli", func(t *testing.T) {
		t.Parallel()
		ex := newFixtureCmdExec(t, map[string]string{"alerts list -o json -s manual ban -l 2": "alerts_list_flags_json"})
		h := pr4Handler(t, ex, t.TempDir())
		h.validateLAPIURL = func(string) (*url.URL, error) { return nil, errors.New("blocked in test") }
		w := pr4Serve(pr4Router(h), http.MethodGet, "/api/v1/admin/crowdsec/alerts?limit=2&scenario="+url.QueryEscape("manual ban"), "")
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		assert.Equal(t, "cscli", pr4Body(t, w)["source"])
	})

	t.Run("a failed cscli run gives an empty list", func(t *testing.T) {
		t.Parallel()
		h := pr4Handler(t, pr4Answer(t, "alerts_list_badlimit_json"), t.TempDir())
		h.validateLAPIURL = func(string) (*url.URL, error) { return nil, errors.New("blocked in test") }
		w := pr4Serve(pr4Router(h), http.MethodGet, "/api/v1/admin/crowdsec/alerts", "")
		require.Equal(t, http.StatusOK, w.Code, w.Body.String())
		body := pr4Body(t, w)
		assert.Equal(t, float64(0), body["total"])
		assert.Empty(t, body["alerts"])
	})
}
