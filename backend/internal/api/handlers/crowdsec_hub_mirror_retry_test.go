package handlers

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/crowdsec"
)

// hubMirrorDialFailure is the output cscli prints when the hub address refuses the connection.
const hubMirrorDialFailure = "failed http request for https://cdn-hub.crowdsec.net/crowdsecurity/hub/.index.json: dial tcp 1.2.3.4:443: connection refused"

// hubMirrorExec answers each cscli call from respond and records the arguments of every call.
type hubMirrorExec struct {
	calls   [][]string
	respond func(n int, args []string) ([]byte, error)
}

func (e *hubMirrorExec) Execute(_ context.Context, name string, args ...string) ([]byte, error) {
	if name != "cscli" {
		return nil, errors.New("unexpected executable: " + name)
	}
	n := len(e.calls)
	e.calls = append(e.calls, append([]string(nil), args...))
	return e.respond(n, args)
}

// hubMirrorConfigFlag returns the path after -c in a cscli call, or "".
func hubMirrorConfigFlag(args []string) string {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == "-c" {
			return args[i+1]
		}
	}
	return ""
}

// newHubMirrorHandler returns a handler whose live configuration exists in its data directory and whose cscli runs
// through exec, the executor the handler uses for hub requests.
func newHubMirrorHandler(t *testing.T, exec CommandExecutor) *CrowdsecHandler {
	t.Helper()
	dir := t.TempDir()
	require.NoError(t, os.WriteFile(filepath.Join(dir, "config.yaml"), []byte("cscli:\n  hub_branch: master\n"), 0o600))
	h := newTestCrowdsecHandler(t, OpenTestDB(t), &countingExec{running: true}, "/bin/false", dir)
	h.CmdExec = exec
	// The hub service has no executor of its own: a retry that used it instead of CmdExec would fail the test.
	h.Hub = crowdsec.NewHubService(nil, nil, dir)
	return h
}

func TestHubUpdateRetriesOnMirrorThroughHandlerExecutor(t *testing.T) {
	exec := &hubMirrorExec{respond: func(n int, args []string) ([]byte, error) {
		if n == 0 {
			require.Equal(t, "-c", args[0])
			return []byte(hubMirrorDialFailure), errors.New("exit status 1")
		}
		return nil, nil
	}}
	h := newHubMirrorHandler(t, exec)

	require.NoError(t, h.hubUpdate(context.Background(), time.Minute))
	require.Len(t, exec.calls, 2)
	require.False(t, strings.HasPrefix(filepath.Base(hubMirrorConfigFlag(exec.calls[0])), ".charon-hub-mirror-"))
	require.True(t, strings.HasPrefix(filepath.Base(hubMirrorConfigFlag(exec.calls[1])), ".charon-hub-mirror-"),
		"the second hub update runs under the derived mirror configuration")
	require.Equal(t, []string{"hub", "update"}, exec.calls[1][2:])
}

func TestInstallHubItemFailsWhenMirrorRetryFails(t *testing.T) {
	exec := &hubMirrorExec{respond: func(_ int, args []string) ([]byte, error) {
		if strings.Contains(strings.Join(args, " "), " install ") {
			return []byte(hubMirrorDialFailure), errors.New("exit status 1")
		}
		return nil, nil
	}}
	h := newHubMirrorHandler(t, exec)

	reason := h.installHubItem(context.Background(), time.Minute, crowdsec.HubItem{Type: "collections", Name: "crowdsecurity/demo"})
	require.Equal(t, hubReasonInstallFail, reason)
	require.Len(t, exec.calls, 3, "live install, mirror hub update, mirror install")
	require.Equal(t, []string{"collections", "install", "crowdsecurity/demo"}, exec.calls[2][2:])
}

func TestHubUpdateWithoutHubServiceIsNotRetried(t *testing.T) {
	exec := &hubMirrorExec{respond: func(_ int, _ []string) ([]byte, error) {
		return []byte(hubMirrorDialFailure), errors.New("exit status 1")
	}}
	h := newHubMirrorHandler(t, exec)
	h.Hub = nil

	require.Error(t, h.hubUpdate(context.Background(), time.Minute))
	require.Len(t, exec.calls, 1)
}
