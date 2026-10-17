package crowdsec

import (
	"context"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

// The enrollment decision comes from `cscli console status -o json` and its "enrolled" field, with the text match
// as the fallback. The not-registered output is recorded from the real cscli; the enrolled output cannot be
// recorded offline (it needs a registered central API account), so console_status_json_enrolled_synthetic is the
// recorded not-registered document with the console fields set.

// skipUntilLaterCommit skips a test whose behavior a later commit in this series adds. Set
// CHARON_RUN_PENDING_TESTS=1 to run the skipped tests anyway.
func skipUntilLaterCommit(t *testing.T) {
	t.Helper()
	if os.Getenv("CHARON_RUN_PENDING_TESTS") != "" {
		return
	}
	t.Skip("enabled by a later commit in this series")
}

// consoleStatusExec answers every command with out and records the arguments.
type consoleStatusExec struct {
	mu    sync.Mutex
	out   []byte
	err   error
	calls [][]string
}

func (c *consoleStatusExec) ExecuteWithEnv(_ context.Context, _ string, args []string, _ map[string]string) ([]byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.calls = append(c.calls, append([]string(nil), args...))
	return c.out, c.err
}

func heartbeatAfter(t *testing.T, status string, out []byte) (*models.CrowdsecConsoleEnrollment, *consoleStatusExec) {
	t.Helper()
	db := openConsoleTestDB(t)
	now := time.Now().UTC()
	require.NoError(t, db.Create(&models.CrowdsecConsoleEnrollment{
		UUID: "test-uuid", Status: status, AgentName: "test-agent", CreatedAt: now, UpdatedAt: now,
	}).Error)
	exec := &consoleStatusExec{out: out}
	NewHeartbeatPoller(db, exec, t.TempDir()).checkHeartbeat(t.Context())
	var got models.CrowdsecConsoleEnrollment
	require.NoError(t, db.First(&got).Error)
	return &got, exec
}

func TestConsoleStatus_AskedAsJSON(t *testing.T) {
	skipUntilLaterCommit(t)
	_, exec := heartbeatAfter(t, consoleStatusEnrolled, contractFixture(t, "console_status_json_enrolled_synthetic"))
	require.Len(t, exec.calls, 1)
	args := exec.calls[0]
	require.GreaterOrEqual(t, len(args), 4)
	assert.Equal(t, []string{"console", "status", "-o", "json"}, args[len(args)-4:])
}

func TestConsoleStatus_JSONDecidesEnrollment(t *testing.T) {
	t.Run("recorded output of an installation that is not registered", func(t *testing.T) {
		skipUntilLaterCommit(t)
		// The document has an "enrolled" key and a "console" key, so a text match would call it enrolled.
		got, _ := heartbeatAfter(t, consoleStatusEnrolled, contractFixture(t, "console_status_json"))
		assert.Nil(t, got.LastHeartbeatAt, "enrolled is false")
	})

	t.Run("enrolled (synthetic document)", func(t *testing.T) {
		got, _ := heartbeatAfter(t, consoleStatusPendingAcceptance, contractFixture(t, "console_status_json_enrolled_synthetic"))
		require.NotNil(t, got.LastHeartbeatAt)
		assert.Equal(t, consoleStatusEnrolled, got.Status, "pending acceptance becomes enrolled")
		require.NotNil(t, got.EnrolledAt)
	})

	t.Run("enrolled false wins over words in other fields", func(t *testing.T) {
		skipUntilLaterCommit(t)
		doc := `{"console":{"authenticated":true,"decision_management":true,"enrolled":false,"plan":"enrolled and connected console","registered":true}}`
		got, _ := heartbeatAfter(t, consoleStatusEnrolled, []byte(doc))
		assert.Nil(t, got.LastHeartbeatAt)
	})

	t.Run("progress lines before the document are ignored", func(t *testing.T) {
		doc := "level=info msg=\"Loaded: 12 parsers\"\n" + string(contractFixture(t, "console_status_json_enrolled_synthetic"))
		got, _ := heartbeatAfter(t, consoleStatusPendingAcceptance, []byte(doc))
		require.NotNil(t, got.LastHeartbeatAt)
	})

	t.Run("the exit code of the recorded fixtures", func(t *testing.T) {
		assert.Zero(t, contractExit(t, "console_status_json"))
		assert.Zero(t, contractExit(t, "console_status_json_enrolled_synthetic"))
	})
}

func TestConsoleStatus_TextFallback(t *testing.T) {
	t.Run("text that says enrolled and connected", func(t *testing.T) {
		got, _ := heartbeatAfter(t, consoleStatusEnrolled, []byte("You can successfully interact with the Console API.\nYour engine is enrolled and connected to the console."))
		require.NotNil(t, got.LastHeartbeatAt)
	})

	t.Run("recorded text of an installation that is not registered", func(t *testing.T) {
		got, _ := heartbeatAfter(t, consoleStatusEnrolled, contractFixture(t, "console_status"))
		assert.Nil(t, got.LastHeartbeatAt)
	})

	t.Run("text that is not JSON and mentions neither word", func(t *testing.T) {
		got, _ := heartbeatAfter(t, consoleStatusEnrolled, []byte(strings.Repeat("unrelated output\n", 3)))
		assert.Nil(t, got.LastHeartbeatAt)
	})

	t.Run("a document without the enrolled field falls back to the text", func(t *testing.T) {
		got, _ := heartbeatAfter(t, consoleStatusEnrolled, []byte(`{"sharing_options":{"custom":true}}`))
		assert.Nil(t, got.LastHeartbeatAt)
	})
}

func TestUnmarshalCSCLIOutput_AboveTheLimit(t *testing.T) {
	var v any

	t.Run("output at the limit is read", func(t *testing.T) {
		out := make([]byte, maxCSCLIOutput)
		for i := range out {
			out[i] = ' '
		}
		copy(out, "[]\n")
		require.NoError(t, UnmarshalCSCLIOutput(out, &v))
	})

	t.Run("output above the limit has its own error", func(t *testing.T) {
		skipUntilLaterCommit(t)
		out := make([]byte, maxCSCLIOutput+1)
		copy(out, "[]\n")
		err := UnmarshalCSCLIOutput(out, &v)
		require.Error(t, err)
		assert.Contains(t, strings.ToLower(err.Error()), "too large", "the sentinel ErrCSCLIOutputTooLarge says the output is too large")
		assert.NotContains(t, err.Error(), "unexpected cscli output", "it is not the parse error")
	})

	t.Run("garbage within the limit is the parse error", func(t *testing.T) {
		err := UnmarshalCSCLIOutput([]byte("no json here"), &v)
		require.Error(t, err)
		assert.Contains(t, err.Error(), "unexpected cscli output")
	})
}
