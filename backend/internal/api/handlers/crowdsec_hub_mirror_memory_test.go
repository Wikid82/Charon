package handlers

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/crowdsec"
)

// hubDialFailure is the cscli output of a hub address that refuses the connection.
const hubDialFailure = "failed http request for https://cdn-hub.crowdsec.net/crowdsecurity/hub/.index.json: dial tcp 1.2.3.4:443: connection refused"

// dialFirstExec fails its first failCalls cscli calls with a dial error and answers every later call.
type dialFirstExec struct {
	failCalls int
	calls     int
}

func (e *dialFirstExec) Execute(_ context.Context, _ string, _ ...string) ([]byte, error) {
	e.calls++
	if e.calls <= e.failCalls {
		return []byte(hubDialFailure), errors.New("exit status 1")
	}
	return []byte("ok"), nil
}

// TestHubMirrorMemoryIsSharedByHandlerAndApply pins that the handler's hub downloads and the hub service's own
// downloads (the apply job path) read and write one mirror-first window: a win on either side is seen by the other.
func TestHubMirrorMemoryIsSharedByHandlerAndApply(t *testing.T) {
	dataDir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dataDir, "config"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "config", "config.yaml"), []byte("cscli:\n  hub_branch: master\n"), 0o600))

	hub := crowdsec.NewHubService(nil, nil, dataDir)
	hub.MirrorBaseURL = "https://raw.githubusercontent.com/crowdsecurity/hub/master"
	h := newTestCrowdsecHandler(t, OpenTestDB(t), &fakeExec{}, "/bin/false", dataDir)
	h.Hub = hub
	h.CmdExec = &dialFirstExec{failCalls: 1}

	require.Same(t, hub.MirrorOptions().Memory, h.hubMirrorOptions().Memory, "handler and hub service share one mirror window")
	require.False(t, hub.MirrorOptions().Memory.Active())

	// A handler download wins on the mirror: the apply side sees the window open.
	_, err := h.runHubCSCLI(context.Background(), 10*time.Second, "collections", "install", "demo")
	require.NoError(t, err)
	require.True(t, hub.MirrorOptions().Memory.Active(), "the apply job sees the window the handler opened")

	// An apply-side download (the mirror-first attempt fails, the live address answers) clears the window the handler sees.
	applyExec := &dialFirstExec{failCalls: 1}
	_, err = crowdsec.RunHubCSCLI(context.Background(), applyExec, hub.MirrorOptions(), "collections", "install", "demo")
	require.NoError(t, err)
	require.False(t, h.hubMirrorOptions().Memory.Active(), "the handler sees the window the apply job cleared")
}
