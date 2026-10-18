package crowdsec

import "sync"

// ApplyJobManager holds the busy flag of the background hub preset install (#1615). While a job holds the
// flag, every handler that changes DataDir or the engine process answers 409 crowdsec_busy instead of
// waiting. The job itself runs without any lock; this type only decides who may start and who must wait.
// Job running, persistence and endpoints are added on top of it.
type ApplyJobManager struct {
	// blockedBy reports a competing long operation (the hub item restore) that forbids starting a job.
	blockedBy func() bool

	mu    sync.Mutex
	jobID string
	busy  bool
}

// NewApplyJobManager returns a manager with no active job. blockedBy may be nil.
func NewApplyJobManager(blockedBy func() bool) *ApplyJobManager {
	return &ApplyJobManager{blockedBy: blockedBy}
}

// TryAcquire takes the busy flag for jobID. It fails when a job already holds it or when the competing
// operation reported by blockedBy is running. The flag is taken before blockedBy is consulted and released
// again on refusal, so a restore that starts at the same moment sees the flag or this call sees the restore.
func (m *ApplyJobManager) TryAcquire(jobID string) bool {
	m.mu.Lock()
	if m.busy {
		m.mu.Unlock()
		return false
	}
	m.busy, m.jobID = true, jobID
	m.mu.Unlock()

	if m.blockedBy != nil && m.blockedBy() {
		m.Release()
		return false
	}
	return true
}

// Release clears the busy flag; it is safe to call when no job holds it.
func (m *ApplyJobManager) Release() {
	m.mu.Lock()
	m.busy, m.jobID = false, ""
	m.mu.Unlock()
}

// Busy reports whether a job holds the flag and its id.
func (m *ApplyJobManager) Busy() (jobID string, busy bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.jobID, m.busy
}
