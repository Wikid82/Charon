package crowdsec

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestApplyJobHoldRefusesStartUntilReleased(t *testing.T) {
	t.Parallel()
	m := NewApplyJobManager()

	release, jobID, ok := m.Hold()
	require.True(t, ok)
	require.Empty(t, jobID)
	require.False(t, m.TryAcquire("job-1"), "a job must wait out a request that holds the manager")
	_, busy := m.Busy()
	require.False(t, busy, "a refused start leaves no flag behind")

	release()
	require.True(t, m.TryAcquire("job-1"))
}

func TestApplyJobHoldsAreSharedAndNeverRefuseEachOther(t *testing.T) {
	t.Parallel()
	m := NewApplyJobManager()

	releaseA, _, okA := m.Hold()
	releaseB, _, okB := m.Hold()
	require.True(t, okA)
	require.True(t, okB, "two requests must not refuse each other")

	releaseA()
	require.False(t, m.TryAcquire("job-1"), "one hold left, so a job still waits")
	releaseB()
	require.True(t, m.TryAcquire("job-1"))
}

func TestApplyJobHoldRefusedWhileAJobRunsNamesTheJob(t *testing.T) {
	t.Parallel()
	m := NewApplyJobManager()
	require.True(t, m.TryAcquire("job-1"))

	release, jobID, ok := m.Hold()
	require.False(t, ok)
	require.Equal(t, "job-1", jobID)
	require.NotNil(t, release, "a refused hold still returns a callable release")
	release()

	m.Release("job-1")
	_, busy := m.Busy()
	require.False(t, busy, "a refused hold must not take the job's place or leave a hold behind")
	require.True(t, m.TryAcquire("job-2"))
}

func TestApplyJobHoldReleaseIsIdempotent(t *testing.T) {
	t.Parallel()
	m := NewApplyJobManager()

	releaseA, _, _ := m.Hold()
	releaseB, _, _ := m.Hold()
	releaseA()
	releaseA() // a second call must not give back releaseB's hold
	require.False(t, m.TryAcquire("job-1"), "a double release must not free a hold that is still out")

	releaseB()
	require.True(t, m.TryAcquire("job-1"))
}

// TestApplyJobHoldStressNeverOverlapsAJob runs holds and job starts against each other. Each side counts itself in
// while it is inside the manager; the two counters must never both be non-zero at once.
func TestApplyJobHoldStressNeverOverlapsAJob(t *testing.T) {
	t.Parallel()
	m := NewApplyJobManager()
	var holdsOut, jobsOut, overlaps atomic.Int32
	check := func() {
		if holdsOut.Load() > 0 && jobsOut.Load() > 0 {
			overlaps.Add(1)
		}
	}

	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(2)
		go func() {
			defer wg.Done()
			release, _, ok := m.Hold()
			if !ok {
				return
			}
			holdsOut.Add(1)
			check()
			holdsOut.Add(-1)
			release()
		}()
		go func() {
			defer wg.Done()
			if !m.TryAcquire("job") {
				return
			}
			jobsOut.Add(1)
			check()
			jobsOut.Add(-1)
			m.Release("job")
		}()
	}
	wg.Wait()
	require.Zero(t, overlaps.Load(), "a job and a hold were inside the manager at the same time")
	_, busy := m.Busy()
	require.False(t, busy)
}

// TestApplyJobRefusedAcquireNeverShowsAJob pins T1: a refused TryAcquire must never make Busy report a job, so a
// concurrent busy check can never name a job id that does not start.
func TestApplyJobRefusedAcquireNeverShowsAJob(t *testing.T) {
	t.Parallel()
	m := NewApplyJobManager()
	release, _, ok := m.Hold()
	require.True(t, ok)
	defer release()

	stop := make(chan struct{})
	var phantoms atomic.Int32
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			select {
			case <-stop:
				return
			default:
			}
			if id, busy := m.Busy(); busy || id != "" {
				phantoms.Add(1)
			}
		}
	}()
	for i := 0; i < 1000; i++ {
		require.False(t, m.TryAcquire("phantom"))
	}
	close(stop)
	<-done
	require.Zero(t, phantoms.Load(), "a refused start must never be visible as a running job")
}
