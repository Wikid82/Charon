package crowdsec

import (
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestApplyJobManagerFlag(t *testing.T) {
	t.Parallel()
	m := NewApplyJobManager(nil)

	_, busy := m.Busy()
	require.False(t, busy)
	require.True(t, m.TryAcquire("job-1"))
	id, busy := m.Busy()
	require.True(t, busy)
	require.Equal(t, "job-1", id)
	require.False(t, m.TryAcquire("job-2"), "only one job at a time")

	m.Release("job-0") // not the holder: ignored
	_, busy = m.Busy()
	require.True(t, busy, "a release for another job id must not clear the flag")
	m.Release("job-1")
	m.Release("job-1") // idempotent
	_, busy = m.Busy()
	require.False(t, busy)
	require.True(t, m.TryAcquire("job-2"))
	m.Release("job-1") // a late release of the first job
	id, busy = m.Busy()
	require.True(t, busy, "a late release must not clear the newer job")
	require.Equal(t, "job-2", id)
}

func TestApplyJobManagerRefusesWhileBlocked(t *testing.T) {
	t.Parallel()
	var blocked atomic.Bool
	m := NewApplyJobManager(blocked.Load)

	blocked.Store(true)
	require.False(t, m.TryAcquire("job-1"))
	_, busy := m.Busy()
	require.False(t, busy, "a refused start leaves no flag behind")

	blocked.Store(false)
	require.True(t, m.TryAcquire("job-1"))
}

func TestApplyJobManagerSingleWinner(t *testing.T) {
	t.Parallel()
	m := NewApplyJobManager(nil)
	var wins atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if m.TryAcquire("j") {
				wins.Add(1)
			}
		}()
	}
	wg.Wait()
	require.Equal(t, int32(1), wins.Load())
}
