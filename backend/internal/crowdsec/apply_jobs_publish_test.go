package crowdsec

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/testutil"
)

// newPublishFixture returns a configured manager over a fresh database. The job body waits on applyGate, so the
// busy flag stays up until the test releases it; cleanup releases it and waits for the job to end.
func newPublishFixture(t *testing.T) (*ApplyJobManager, *gorm.DB) {
	t.Helper()
	db := testutil.OpenMemoryDB(t)
	require.NoError(t, db.AutoMigrate(&models.CrowdsecApplyJob{}))
	m := NewApplyJobManager()
	applyGate := make(chan struct{})
	var once sync.Once
	releaseApply := func() { once.Do(func() { close(applyGate) }) }
	m.Configure(ApplyJobConfig{
		Repo: NewApplyJobRepository(db),
		Apply: func(context.Context, ApplyRequest) (ApplyResult, error) {
			<-applyGate
			return ApplyResult{}, nil
		},
	})
	t.Cleanup(func() {
		releaseApply()
		require.Eventually(t, func() bool { _, busy := m.Busy(); return !busy }, 10*time.Second, 5*time.Millisecond)
	})
	return m, db
}

type startOutcome struct {
	job *models.CrowdsecApplyJob
	err error
}

// startAsync runs Start on its own goroutine, so the test can look at the manager while the job row is written.
func startAsync(m *ApplyJobManager) <-chan startOutcome {
	out := make(chan startOutcome, 1)
	go func() {
		job, err := m.Start(ApplyJobRequest{Slug: "crowdsecurity/sshd", Type: "collections", CacheKey: "k"})
		out <- startOutcome{job: job, err: err}
	}()
	return out
}

func awaitStart(t *testing.T, done <-chan startOutcome) startOutcome {
	t.Helper()
	select {
	case res := <-done:
		return res
	case <-time.After(10 * time.Second):
		t.Fatal("Start never returned")
		return startOutcome{}
	}
}

// TestApplyJobRefusalBeforeRowRecordedNamesNoJob pins T3: while the job row is still being inserted, every refusal
// (Busy, Hold, TryAcquire) must not name a job id, because GET presets/jobs/:id would answer 404 for it.
func TestApplyJobRefusalBeforeRowRecordedNamesNoJob(t *testing.T) {
	m, db := newPublishFixture(t)
	entered, unblockInsert := testutil.GateCreate[models.CrowdsecApplyJob](t, db, nil)
	done := startAsync(m)
	testutil.AwaitGate(t, entered)

	id, busy := m.Busy()
	require.True(t, busy, "the busy flag is taken while the row is inserted")
	require.Empty(t, id, "no job id may be named before its row exists")

	_, holdID, holdOK := m.Hold()
	require.False(t, holdOK, "a hold is refused while the flag is taken")
	require.Empty(t, holdID, "Hold must not name an unrecorded job")

	require.False(t, m.TryAcquire("job-other"), "a second job is refused in the window")

	unblockInsert()
	res := awaitStart(t, done)
	require.NoError(t, res.err)
	require.NotNil(t, res.job)

	id, busy = m.Busy()
	require.True(t, busy)
	require.Equal(t, res.job.ID, id, "after the row exists Busy names the job")

	_, holdID, holdOK = m.Hold()
	require.False(t, holdOK)
	require.Equal(t, res.job.ID, holdID, "after the row exists Hold names the job")
}

// TestApplyJobFailedRowInsertNeverPublishesAJobID checks that a row that fails to insert never names an id, during
// the window or after Start returns, and that the flag is cleared again.
func TestApplyJobFailedRowInsertNeverPublishesAJobID(t *testing.T) {
	m, db := newPublishFixture(t)
	entered, unblockInsert := testutil.GateCreate[models.CrowdsecApplyJob](t, db, errors.New("insert refused"))
	done := startAsync(m)
	testutil.AwaitGate(t, entered)

	id, busy := m.Busy()
	require.True(t, busy)
	require.Empty(t, id)

	unblockInsert()
	res := awaitStart(t, done)
	require.Error(t, res.err)
	require.Nil(t, res.job)

	id, busy = m.Busy()
	require.False(t, busy, "a failed start releases the flag")
	require.Empty(t, id)

	release, holdID, ok := m.Hold()
	require.True(t, ok, "no hold is left behind by the failed start")
	require.Empty(t, holdID)
	release()
}
