package crowdsec

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestApplyJobCanStartReportsIdleManager(t *testing.T) {
	t.Parallel()
	m := NewApplyJobManager()

	jobID, ok := m.CanStart()
	require.True(t, ok)
	require.Empty(t, jobID)
}

func TestApplyJobCanStartRefusesWhileAHoldIsOut(t *testing.T) {
	t.Parallel()
	m := NewApplyJobManager()

	release, _, held := m.Hold()
	require.True(t, held)
	jobID, ok := m.CanStart()
	require.False(t, ok, "a request or restore holding the manager refuses a new job at once")
	require.Empty(t, jobID, "no job is named while only a hold is out")

	release()
	_, ok = m.CanStart()
	require.True(t, ok)
}

func TestApplyJobCanStartNamesThePublishedJob(t *testing.T) {
	t.Parallel()
	m := NewApplyJobManager()
	require.True(t, m.TryAcquire("job-1"))

	jobID, ok := m.CanStart()
	require.False(t, ok)
	require.Equal(t, "job-1", jobID)

	m.Release("job-1")
	_, ok = m.CanStart()
	require.True(t, ok)
}

func TestApplyJobCanStartWithheldJobIDBeforePublication(t *testing.T) {
	t.Parallel()
	m := NewApplyJobManager()
	require.True(t, m.tryAcquire("job-1", false))

	jobID, ok := m.CanStart()
	require.False(t, ok, "an unpublished job still holds the flag")
	require.Empty(t, jobID, "its id is withheld until its row is recorded, as Busy does")
}
