package crowdsec

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// cscli downloads every item of a collection before it links the first one, so the downloaded hub files are
// the only count that moves during a long install.
func TestApplyJobProgressFollowsDownloadedHubFiles(t *testing.T) {
	t.Parallel()
	var downloaded atomic.Int64
	downloaded.Store(7) // files of earlier installs: the baseline
	release := make(chan struct{})
	h := newHarness(t, pendingJobConfig{
		DataDir: t.TempDir(), ProgressInterval: 5 * time.Millisecond,
		CountItems:      func() int { return 0 }, // nothing is linked until the end
		CountDownloaded: func() int { return int(downloaded.Load()) },
	})
	h.mgr.cfg.Apply = func(_ context.Context, req ApplyRequest) (ApplyResult, error) {
		req.OnPhase(PhaseInstall)
		downloaded.Store(9)
		<-release
		return ApplyResult{Status: "applied"}, nil
	}
	id, err := h.Start(context.Background(), applyTestSlug, "collections", "key")
	require.NoError(t, err)
	require.Eventually(t, func() bool {
		job, getErr := h.Get(id)
		return getErr == nil && job.ItemsInstalled == 2
	}, 5*time.Second, 5*time.Millisecond)
	close(release)
	done, err := h.Wait(id, 5*time.Second)
	require.NoError(t, err)
	require.Equal(t, jobSucceeded, done.State)
	require.Zero(t, done.ItemsInstalled, "the final value is the linked count, not the download count")
}

func TestCountDownloadedHubFiles(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	require.Zero(t, CountDownloadedHubFiles(dir))
	for _, rel := range []string{
		"hub/collections/crowdsecurity/a.yaml",
		"hub/parsers/s01-parse/crowdsecurity/b.yaml",
		"config/hub/scenarios/crowdsecurity/c.yaml",
		"hub/collections/crowdsecurity/notes.txt",
		"hub/unknown/crowdsecurity/d.yaml",
		"hub/scenarios/sigmahq/e.yml",
		"config/hub/parsers/s01-parse/crowdsecurity/f.yml",
		"hub/collections/crowdsecurity/g.YML",
		"hub/collections/crowdsecurity/h.yml.bak",
	} {
		p := filepath.Join(dir, rel)
		require.NoError(t, os.MkdirAll(filepath.Dir(p), 0o750))
		require.NoError(t, os.WriteFile(p, []byte("x"), 0o600))
	}
	require.Equal(t, 5, CountDownloadedHubFiles(dir), "yaml and yml items count, other suffixes do not")
}

func TestApplyJobRecoveryPrunesSnapshotsAndProtectsUnrecoveredOnes(t *testing.T) {
	t.Parallel()
	h := newHarness(t, pendingJobConfig{DataDir: t.TempDir()})
	snap := lightSnapshotFixture(t)
	require.NoError(t, h.Seed(pendingJob{ID: "ok", State: jobRunning, StartedAt: time.Now(), BackupPath: snap}))
	failed := filepath.Join(t.TempDir(), "gone")
	require.NoError(t, h.Seed(pendingJob{ID: "bad", State: jobRunning, StartedAt: time.Now(), BackupPath: failed}))

	var protect []string
	calls := 0
	h.mgr.cfg.PruneSnapshots = func(p []string) { calls++; protect = p }
	require.NoError(t, h.mgr.RecoverInterrupted(context.Background(), func(string) error { return nil }))
	require.Equal(t, 1, calls)
	require.Equal(t, []string{failed}, protect, "only the snapshot whose recovery did not complete is protected")
}
