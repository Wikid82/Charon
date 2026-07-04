package services

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// sortFixtureService writes six mixed entries (file order = ts ascending) and
// returns a service whose log dir contains "sort.log". Used by the sort_by tests.
//
//	msg  ts   level    method  uri  status
//	e1   100  info     GET     /b   200
//	e2   200  error    post    /a   500
//	e3   300  debug    DELETE  /C   404
//	e4   400  warn     PUT     /d   200
//	e5   500  UNKNOWN  PATCH   /e   301
//	e6   600  info     get     /f   502
func sortFixtureService(t *testing.T) *LogService {
	t.Helper()
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	logsDir := filepath.Join(dataDir, "logs")
	require.NoError(t, os.MkdirAll(logsDir, 0o750))

	lines := []string{
		`{"level":"info","ts":100,"msg":"e1","request":{"method":"GET","uri":"/b"},"status":200}`,
		`{"level":"error","ts":200,"msg":"e2","request":{"method":"post","uri":"/a"},"status":500}`,
		`{"level":"debug","ts":300,"msg":"e3","request":{"method":"DELETE","uri":"/C"},"status":404}`,
		`{"level":"warn","ts":400,"msg":"e4","request":{"method":"PUT","uri":"/d"},"status":200}`,
		`{"level":"UNKNOWN","ts":500,"msg":"e5","request":{"method":"PATCH","uri":"/e"},"status":301}`,
		`{"level":"info","ts":600,"msg":"e6","request":{"method":"get","uri":"/f"},"status":502}`,
	}
	content := strings.Join(lines, "\n") + "\n"
	require.NoError(t, os.WriteFile(filepath.Join(logsDir, "sort.log"), []byte(content), 0o600)) // #nosec G306 -- test fixture

	return NewLogService(&config.Config{DatabasePath: filepath.Join(dataDir, "charon.db")})
}

func msgs(entries []models.CaddyAccessLog) []string {
	out := make([]string, len(entries))
	for i, e := range entries {
		out[i] = e.Msg
	}
	return out
}

func TestQueryLogs_SortBy(t *testing.T) {
	service := sortFixtureService(t)

	tests := []struct {
		sortBy string
		dir    string
		want   []string
	}{
		{"ts", "asc", []string{"e1", "e2", "e3", "e4", "e5", "e6"}},
		{"ts", "desc", []string{"e6", "e5", "e4", "e3", "e2", "e1"}},
		// level rank: unknown(-1) < debug < info < warn < error; equal info pair tiebreaks ts desc (e6 before e1)
		{"level", "asc", []string{"e5", "e3", "e6", "e1", "e4", "e2"}},
		{"level", "desc", []string{"e2", "e4", "e6", "e1", "e3", "e5"}},
		// method case-insensitive; GET==get tiebreaks ts desc (e6 before e1)
		{"method", "asc", []string{"e3", "e6", "e1", "e5", "e2", "e4"}},
		{"method", "desc", []string{"e4", "e2", "e5", "e6", "e1", "e3"}},
		// uri case-insensitive (/C sorts between /b and /d)
		{"uri", "asc", []string{"e2", "e1", "e3", "e4", "e5", "e6"}},
		{"uri", "desc", []string{"e6", "e5", "e4", "e3", "e1", "e2"}},
		// status numeric; equal 200 pair tiebreaks ts desc (e4 before e1) in BOTH directions (deterministic pages)
		{"status", "asc", []string{"e4", "e1", "e5", "e3", "e2", "e6"}},
		{"status", "desc", []string{"e6", "e2", "e3", "e5", "e4", "e1"}},
	}

	for _, tt := range tests {
		t.Run(tt.sortBy+"_"+tt.dir, func(t *testing.T) {
			results, total, skipped, err := service.QueryLogs("sort.log", models.LogFilter{
				Limit: 10, Sort: tt.dir, SortBy: tt.sortBy,
			})
			require.NoError(t, err)
			assert.Equal(t, int64(6), total)
			assert.Equal(t, int64(0), skipped)
			assert.Equal(t, tt.want, msgs(results))
		})
	}
}

func TestQueryLogs_LevelRank(t *testing.T) {
	assert.Equal(t, 0, levelRank("debug"))
	assert.Equal(t, 1, levelRank("info"))
	assert.Equal(t, 2, levelRank("warn"))
	assert.Equal(t, 3, levelRank("error"))
	assert.Equal(t, -1, levelRank("mystery"))
	assert.Equal(t, -1, levelRank(""))
	// case-insensitive
	assert.Equal(t, 3, levelRank("ERROR"))
	assert.Equal(t, 1, levelRank("Info"))
}

func TestQueryLogs_PaginationEdges(t *testing.T) {
	service := sortFixtureService(t)

	// Offset beyond total -> empty page, correct total.
	results, total, _, err := service.QueryLogs("sort.log", models.LogFilter{Limit: 10, Offset: 100})
	require.NoError(t, err)
	assert.Equal(t, int64(6), total)
	assert.Empty(t, results)

	// Partial last page.
	results, total, _, err = service.QueryLogs("sort.log", models.LogFilter{Limit: 4, Offset: 4})
	require.NoError(t, err)
	assert.Equal(t, int64(6), total)
	assert.Len(t, results, 2)

	// Limit clamping happens ONLY in Validate (not duplicated in QueryLogs).
	zero := models.LogFilter{Limit: 0}
	require.NoError(t, zero.Validate())
	assert.Equal(t, 50, zero.Limit)
	results, _, _, err = service.QueryLogs("sort.log", zero)
	require.NoError(t, err)
	assert.Len(t, results, 6)

	huge := models.LogFilter{Limit: 9999}
	require.NoError(t, huge.Validate())
	assert.Equal(t, 500, huge.Limit)
}

func TestQueryLogs_CorruptedLines(t *testing.T) {
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	logsDir := filepath.Join(dataDir, "logs")
	require.NoError(t, os.MkdirAll(logsDir, 0o750))

	oversized := strings.Repeat("a", (1<<20)+100) // > 1MB single line -> skipped, loop continues

	var content strings.Builder
	content.WriteString(`{"level":"info","ts":1,"msg":"json before"}` + "\n")
	content.WriteString("plain text fallback line\n")                 // valid UTF-8, no NUL -> fallback, NOT skipped
	content.WriteString("corrupt\x00with nul\n")                      // JSON-fail + NUL byte -> skipped
	content.WriteString("bad utf8 \xff\xfe\xfd here\n")               // JSON-fail + invalid UTF-8 -> skipped
	content.WriteString(oversized + "\n")                             // over per-line cap -> skipped, loop continues
	content.WriteString(`{"level":"info","ts":2,"msg":"json after"}` + "\n")

	require.NoError(t, os.WriteFile(filepath.Join(logsDir, "mixed.log"), []byte(content.String()), 0o600)) // #nosec G306 -- test fixture

	service := NewLogService(&config.Config{DatabasePath: filepath.Join(dataDir, "charon.db")})

	results, total, skipped, err := service.QueryLogs("mixed.log", models.LogFilter{Limit: 50, Sort: "asc"})
	require.NoError(t, err)
	assert.Equal(t, int64(3), total, "two JSON lines + one plain-text fallback line")
	assert.Equal(t, int64(3), skipped, "NUL line + invalid UTF-8 line + oversized line")

	got := msgs(results)
	assert.Contains(t, got, "json before")
	assert.Contains(t, got, "json after")
	assert.Contains(t, got, "plain text fallback line")
}

func TestQueryLogs_ValidJSONWithEscapedNulIsKept(t *testing.T) {
	// A JSON line that PARSES is never treated as corruption, whatever it encodes.
	tmpDir := t.TempDir()
	dataDir := filepath.Join(tmpDir, "data")
	logsDir := filepath.Join(dataDir, "logs")
	require.NoError(t, os.MkdirAll(logsDir, 0o750))
	line := "{\"level\":\"info\",\"ts\":1,\"msg\":\"escaped \\u0000 nul\"}"
	require.NoError(t, os.WriteFile(filepath.Join(logsDir, "esc.log"), []byte(line+"\n"), 0o600)) // #nosec G306 -- test fixture

	service := NewLogService(&config.Config{DatabasePath: filepath.Join(dataDir, "charon.db")})
	results, total, skipped, err := service.QueryLogs("esc.log", models.LogFilter{Limit: 10})
	require.NoError(t, err)
	assert.Equal(t, int64(1), total)
	assert.Equal(t, int64(0), skipped)
	require.Len(t, results, 1)
}
