package handlers

import (
	"context"
	"errors"
	"net/http"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseCSCLIDecisions(t *testing.T) {
	t.Parallel()

	t.Run("empty spellings", func(t *testing.T) {
		t.Parallel()
		for _, in := range []string{"", "  \n", "null", "null\n", "[]\n"} {
			got, skipped, _, err := parseCSCLIDecisions([]byte(in))
			require.NoError(t, err, in)
			assert.NotNil(t, got, in)
			assert.Empty(t, got, in)
			assert.Zero(t, skipped, in)
		}
	})

	t.Run("progress lines before the document", func(t *testing.T) {
		t.Parallel()
		in := "time=\"x\" level=info msg=\"loaded\"\n" + `[{"id":1,"value":"192.0.2.1","scope":"Ip"}]`
		got, _, _, err := parseCSCLIDecisions([]byte(in))
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, "192.0.2.1", got[0].Value)
	})

	t.Run("alert fields are carried to the decisions", func(t *testing.T) {
		t.Parallel()
		in := `[{"id":9,"scenario":"alert-s","created_at":"2026-10-16T23:15:34Z","unknown":{"a":1},
		  "decisions":[{"id":4,"origin":"cscli","type":"ban","scope":"Ip","value":"192.0.2.4","duration":"1h","simulated":false}]}]`
		got, skipped, _, err := parseCSCLIDecisions([]byte(in))
		require.NoError(t, err)
		assert.Zero(t, skipped)
		require.Len(t, got, 1)
		assert.Equal(t, CrowdSecDecision{ID: 4, Origin: "cscli", Type: "ban", Scope: "Ip", Value: "192.0.2.4", Duration: "1h",
			Scenario: "alert-s", CreatedAt: time.Date(2026, 10, 16, 23, 15, 34, 0, time.UTC)}, got[0])
	})

	t.Run("a decision keeps its own scenario when the alert has none", func(t *testing.T) {
		t.Parallel()
		in := `[{"id":1,"decisions":[{"id":1,"value":"192.0.2.4","scenario":"own","created_at":"2025-01-02T03:04:05Z"}]}]`
		got, _, _, err := parseCSCLIDecisions([]byte(in))
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, "own", got[0].Scenario)
		assert.Equal(t, 2025, got[0].CreatedAt.Year())
	})

	t.Run("rows without a value are counted", func(t *testing.T) {
		t.Parallel()
		in := `[{"id":1,"scenario":"s","decisions":[{"value":"192.0.2.1"},{"value":""},{"id":3}]},
		  {"id":2,"value":""},{"id":3,"value":"192.0.2.9"},{"id":4,"decisions":[]}]`
		got, skipped, _, err := parseCSCLIDecisions([]byte(in))
		require.NoError(t, err)
		require.Len(t, got, 2)
		assert.Equal(t, "192.0.2.1", got[0].Value)
		assert.Equal(t, "192.0.2.9", got[1].Value)
		assert.Equal(t, 4, skipped)
	})

	t.Run("row cap", func(t *testing.T) {
		t.Parallel()
		in := "[" + strings.Repeat(`{"value":"192.0.2.1"},`, maxCSCLIDecisions+5) + `{"value":"192.0.2.2"}]`
		got, skipped, truncated, err := parseCSCLIDecisions([]byte(in))
		require.NoError(t, err)
		assert.Len(t, got, maxCSCLIDecisions)
		assert.Zero(t, skipped)
		assert.True(t, truncated)
	})

	t.Run("invalid output", func(t *testing.T) {
		t.Parallel()
		for _, in := range []string{"not json", `{"a":1}`, `[{"id":"x"}]`} {
			_, _, _, err := parseCSCLIDecisions([]byte(in))
			assert.Error(t, err, in)
		}
	})
}

// deadlineExec reports the deadline it was given and fails as a timed-out command does.
type deadlineExec struct {
	deadline time.Time
	has      bool
}

func (d *deadlineExec) Execute(ctx context.Context, _ string, _ ...string) ([]byte, error) {
	d.deadline, d.has = ctx.Deadline()
	return nil, context.DeadlineExceeded
}

func TestListDecisions_RunsUnderADeadline(t *testing.T) {
	t.Parallel()
	fx := &deadlineExec{}
	start := time.Now()
	code, resp, _ := listDecisionsVia(t, fx, t.TempDir())
	require.Equal(t, http.StatusOK, code)
	require.True(t, fx.has)
	assert.WithinDuration(t, start.Add(cscliListTimeout), fx.deadline, 5*time.Second)
	assert.Empty(t, resp.Decisions)
	assert.Contains(t, resp.Error, "timed out")
}

func TestCSCLIDecisionsTimedOut(t *testing.T) {
	t.Parallel()
	expired, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	assert.True(t, isCSCLITimeout(expired, errors.New("signal: killed")))
	assert.True(t, isCSCLITimeout(context.Background(), context.DeadlineExceeded))
	assert.False(t, isCSCLITimeout(context.Background(), errors.New("exit status 1")))
}

func TestParseCSCLIDecisions_Truncation(t *testing.T) {
	t.Parallel()
	flatRows := func(n int) string {
		return "[" + strings.TrimSuffix(strings.Repeat(`{"value":"192.0.2.1"},`, n), ",") + "]"
	}

	t.Run("exactly the cap is not truncated", func(t *testing.T) {
		t.Parallel()
		got, _, truncated, err := parseCSCLIDecisions([]byte(flatRows(maxCSCLIDecisions)))
		require.NoError(t, err)
		assert.Len(t, got, maxCSCLIDecisions)
		assert.False(t, truncated)
	})

	t.Run("one past the cap is truncated", func(t *testing.T) {
		t.Parallel()
		got, _, truncated, err := parseCSCLIDecisions([]byte(flatRows(maxCSCLIDecisions + 1)))
		require.NoError(t, err)
		assert.Len(t, got, maxCSCLIDecisions)
		assert.True(t, truncated)
	})

	t.Run("rows without a value past the cap are not truncation", func(t *testing.T) {
		t.Parallel()
		in := flatRows(maxCSCLIDecisions)
		in = strings.TrimSuffix(in, "]") + `,{},{"value":""}]`
		got, skipped, truncated, err := parseCSCLIDecisions([]byte(in))
		require.NoError(t, err)
		assert.Len(t, got, maxCSCLIDecisions)
		assert.Equal(t, 2, skipped)
		assert.False(t, truncated)
	})

	t.Run("nested decisions of one alert", func(t *testing.T) {
		t.Parallel()
		in := `[{"id":1,"scenario":"s","decisions":[` + strings.TrimSuffix(strings.Repeat(`{"value":"192.0.2.1"},`, maxCSCLIDecisions+1), ",") + `]}]`
		got, _, truncated, err := parseCSCLIDecisions([]byte(in))
		require.NoError(t, err)
		assert.Len(t, got, maxCSCLIDecisions)
		assert.True(t, truncated)
		assert.Equal(t, "s", got[0].Scenario)
	})

	t.Run("fail closed on a later bad row before the cap", func(t *testing.T) {
		t.Parallel()
		_, _, _, err := parseCSCLIDecisions([]byte(`[{"value":"192.0.2.1"},{"id":"x"}]`))
		assert.Error(t, err)
		_, _, _, err = parseCSCLIDecisions([]byte(`[{"id":1,"decisions":{"value":"x"}}]`))
		assert.Error(t, err)
		_, _, _, err = parseCSCLIDecisions([]byte(`{"value":"192.0.2.1"}`))
		assert.Error(t, err)
		_, _, _, err = parseCSCLIDecisions([]byte(`[{"id":1,"decisions":[{"id":"x"}]}]`))
		assert.Error(t, err)
	})

	t.Run("a null document is empty", func(t *testing.T) {
		t.Parallel()
		got, skipped, truncated, err := parseCSCLIDecisions([]byte("Loaded: 1 parsers\n[null,{\"decisions\":null,\"value\":\"192.0.2.1\"}]"))
		require.NoError(t, err)
		assert.Len(t, got, 1)
		assert.Equal(t, 1, skipped)
		assert.False(t, truncated)
	})
}

// A flood of tiny rows must be read one row at a time: the heap stays far below what holding every row would
// take (hundreds of MiB for this input if the whole document were decoded first). Not parallel, so no other
// test allocates while it measures.
func TestParseCSCLIDecisions_TinyRowFloodStaysBounded(t *testing.T) {
	const rows = 1_000_000
	in := []byte("[" + strings.Repeat("{},", rows) + "{}]")

	var base runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&base)
	stop := samplePeakHeap(base.HeapAlloc)
	got, skipped, truncated, err := parseCSCLIDecisions(in)
	peak := stop()

	require.NoError(t, err)
	assert.Empty(t, got)
	assert.Equal(t, rows+1, skipped)
	assert.False(t, truncated)
	growth := peak - base.HeapAlloc
	t.Logf("peak heap growth %d bytes for %d rows", growth, rows)
	assert.Less(t, growth, uint64(64<<20), "memory must not grow with the number of rows")
}

// samplePeakHeap polls the heap in the background; the returned function stops it and gives the highest
// HeapAlloc seen.
func samplePeakHeap(start uint64) func() uint64 {
	done, result := make(chan struct{}), make(chan uint64)
	go func() {
		highest := start
		var m runtime.MemStats
		for {
			select {
			case <-done:
				result <- highest
				return
			case <-time.After(2 * time.Millisecond):
				runtime.ReadMemStats(&m)
				highest = max(highest, m.HeapAlloc)
			}
		}
	}()
	return func() uint64 { close(done); return <-result }
}

func TestListDecisions_LogsOnceWhenTheRowCapIsHit(t *testing.T) {
	hook := &messageHook{}
	logger.Log().Logger.AddHook(hook)

	out := "[" + strings.Repeat(`{"value":"192.0.2.1"},`, maxCSCLIDecisions+3) + `{"value":"192.0.2.2"}]`
	code, resp, _ := listDecisionsVia(t, &rawOutputExec{out: []byte(out)}, t.TempDir())
	require.Equal(t, http.StatusOK, code)
	assert.Equal(t, maxCSCLIDecisions, resp.Total)

	hook.mu.Lock()
	defer hook.mu.Unlock()
	var warnings int
	for _, m := range hook.messages {
		if strings.Contains(m, "row limit") {
			warnings++
			assert.NotContains(t, m, "192.0.2")
		}
	}
	assert.Equal(t, 1, warnings)
}
