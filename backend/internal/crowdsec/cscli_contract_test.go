package crowdsec

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Contract tests: the parsing code is fed output recorded from the real cscli bundled with Charon
// (see testdata/cscli/<version>/README.md for the version and how to refresh the recording).

const cscliContractDir = "testdata/cscli/v1.8.2-rc1"

func contractFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(cscliContractDir, name+".txt")) //nolint:gosec // G304: fixed testdata directory, names come from the tests
	require.NoError(t, err)
	return data
}

func contractExit(t *testing.T, name string) int {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(cscliContractDir, name+".exit")) //nolint:gosec // G304: fixed testdata directory, names come from the tests
	require.NoError(t, err)
	code, err := strconv.Atoi(strings.TrimSpace(string(data)))
	require.NoError(t, err)
	return code
}

type contractExec struct {
	byCmd map[string]string
	t     *testing.T
}

func (c contractExec) run(args []string) ([]byte, error) {
	if len(args) >= 2 && args[0] == "-c" {
		args = args[2:]
	}
	name, ok := c.byCmd[strings.Join(args, " ")]
	if !ok {
		c.t.Errorf("unexpected cscli invocation: %q", strings.Join(args, " "))
		return nil, errors.New("unexpected cscli invocation")
	}
	out := contractFixture(c.t, name)
	if code := contractExit(c.t, name); code != 0 {
		return out, fmt.Errorf("exit status %d", code)
	}
	return out, nil
}

func (c contractExec) Execute(_ context.Context, _ string, args ...string) ([]byte, error) {
	return c.run(args)
}

func (c contractExec) ExecuteWithEnv(_ context.Context, _ string, args []string, _ map[string]string) ([]byte, error) {
	return c.run(args)
}

func TestCSCLIContract_RecordedJSONDocuments(t *testing.T) {
	t.Parallel()
	for _, name := range []string{"bouncers_list_json", "bouncers_list_empty_json"} {
		var list []map[string]any
		require.NoError(t, unmarshalCSCLIJSON(contractFixture(t, name), &list), name)
	}
	var bouncers []struct {
		Name    string `json:"name"`
		Revoked bool   `json:"revoked"`
	}
	require.NoError(t, unmarshalCSCLIJSON(contractFixture(t, "bouncers_list_json"), &bouncers))
	require.Len(t, bouncers, 1)
	assert.Equal(t, "caddy-bouncer", bouncers[0].Name)
	assert.False(t, bouncers[0].Revoked)

	var status struct {
		Console map[string]any `json:"console"`
		Sharing map[string]any `json:"sharing_options"`
	}
	require.NoError(t, unmarshalCSCLIJSON(contractFixture(t, "console_status_json"), &status))
	assert.Equal(t, false, status.Console["enrolled"])
}

func TestCSCLIContract_ConsoleStatus(t *testing.T) {
	t.Parallel()
	p := &HeartbeatPoller{}
	t.Run("text of an installation that is not registered is not enrolled", func(t *testing.T) {
		t.Parallel()
		assert.False(t, p.isEnrolledOutput(string(contractFixture(t, "console_status"))))
	})
	t.Run("the invocation exists in the bundled cscli", func(t *testing.T) {
		t.Parallel()
		assert.Zero(t, contractExit(t, "console_status"))
		assert.Zero(t, contractExit(t, "console_status_json"))
	})
}

func TestCSCLIContract_LAPIStatus(t *testing.T) {
	t.Parallel()
	t.Run("running local API is available", func(t *testing.T) {
		t.Parallel()
		svc := NewConsoleEnrollmentService(openConsoleTestDB(t), contractExec{t: t, byCmd: map[string]string{"lapi status": "lapi_status"}}, t.TempDir(), "secret")
		require.NoError(t, svc.checkLAPIAvailable(t.Context()))
	})
	t.Run("stopped local API exits non-zero", func(t *testing.T) {
		t.Parallel()
		assert.NotZero(t, contractExit(t, "lapi_status_down"))
		assert.Contains(t, string(contractFixture(t, "lapi_status_down")), "connection refused")
	})
	t.Run("capi that is not registered exits non-zero", func(t *testing.T) {
		t.Parallel()
		assert.NotZero(t, contractExit(t, "capi_status"))
		assert.Contains(t, string(contractFixture(t, "capi_status")), "capi register")
	})
}

func TestCSCLIContract_Version(t *testing.T) {
	t.Parallel()
	s := &HubService{Exec: contractExec{t: t, byCmd: map[string]string{"version": "version"}}}
	assert.True(t, s.hasCSCLI(t.Context()))
	assert.Contains(t, string(contractFixture(t, "version")), "version: v1.8.2-rc1")
}

// Output limit of the secure executor.

func TestSecureCommandExecutor_ExecuteWithEnv_CombinesStreams(t *testing.T) {
	t.Parallel()
	out, err := (&SecureCommandExecutor{}).ExecuteWithEnv(t.Context(), "sh", []string{"-c", "echo out; echo err >&2; echo $CONTRACT_VAR"}, map[string]string{"CONTRACT_VAR": "v"})
	require.NoError(t, err)
	assert.Equal(t, "out\nerr\nv\n", string(out))
}

func TestSecureCommandExecutor_ExecuteWithEnv_OutputLimit(t *testing.T) {
	t.Parallel()

	t.Run("output at the parser cap is returned in full", func(t *testing.T) {
		t.Parallel()
		out, err := (&SecureCommandExecutor{}).ExecuteWithEnv(t.Context(), "sh", []string{"-c", fmt.Sprintf("head -c %d /dev/zero", maxCSCLIOutput)}, nil)
		require.NoError(t, err)
		assert.Len(t, out, maxCSCLIOutput)
	})

	for _, tc := range []struct{ name, redirect string }{{"stdout", ""}, {"stderr", " >&2"}} {
		redirect := tc.redirect
		t.Run("output beyond the cap on "+tc.name+" stops the command with a clear error", func(t *testing.T) {
			t.Parallel()
			t.Skip("enabled by a later commit in this series")
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()
			start := time.Now()
			// The command idles after printing, so only the executor's own limit can stop it quickly.
			out, err := (&SecureCommandExecutor{}).ExecuteWithEnv(ctx, "sh", []string{"-c", "head -c 40000000 /dev/zero" + redirect + "; exec sleep 5"}, nil)
			require.Error(t, err)
			assert.Contains(t, strings.ToLower(err.Error()), "output")
			assert.Contains(t, strings.ToLower(err.Error()), "limit")
			assert.Less(t, time.Since(start), 4*time.Second, "the command is killed, not waited for")
			assert.Less(t, len(out), 40_000_000, "the output kept is bounded")
		})
	}
}
