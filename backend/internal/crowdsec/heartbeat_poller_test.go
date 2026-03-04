package crowdsec

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/Wikid82/charon/backend/internal/models"
)

// mockEnvExecutor implements EnvCommandExecutor for testing.
type mockEnvExecutor struct {
	mu        sync.Mutex
	callCount int
	responses []struct {
		out []byte
		err error
	}
	responseIdx int
}

func (m *mockEnvExecutor) ExecuteWithEnv(ctx context.Context, name string, args []string, env map[string]string) ([]byte, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.callCount++
	if m.responseIdx < len(m.responses) {
		resp := m.responses[m.responseIdx]
		m.responseIdx++
		return resp.out, resp.err
	}
	return nil, nil
}

func (m *mockEnvExecutor) getCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.callCount
}

// ============================================
// TestHeartbeatPoller_StartStop
// ============================================

func TestHeartbeatPoller_StartStop(t *testing.T) {
	t.Run("Start sets running to true", func(t *testing.T) {
		db := openConsoleTestDB(t)
		exec := &mockEnvExecutor{}
		poller := NewHeartbeatPoller(db, exec, t.TempDir())

		require.False(t, poller.IsRunning())
		poller.Start()
		require.True(t, poller.IsRunning())
		poller.Stop()
		require.False(t, poller.IsRunning())
	})

	t.Run("Stop stops the poller cleanly", func(t *testing.T) {
		db := openConsoleTestDB(t)
		exec := &mockEnvExecutor{}
		poller := NewHeartbeatPoller(db, exec, t.TempDir())
		poller.SetInterval(10 * time.Millisecond) // Fast for testing

		poller.Start()
		require.True(t, poller.IsRunning())

		// Wait a bit to ensure the poller runs at least once
		time.Sleep(50 * time.Millisecond)

		poller.Stop()
		require.False(t, poller.IsRunning())
	})

	t.Run("multiple Stop calls are safe", func(t *testing.T) {
		db := openConsoleTestDB(t)
		exec := &mockEnvExecutor{}
		poller := NewHeartbeatPoller(db, exec, t.TempDir())

		poller.Start()
		poller.Stop()
		poller.Stop() // Should not panic
		poller.Stop() // Should not panic
		require.False(t, poller.IsRunning())
	})

	t.Run("multiple Start calls are prevented", func(t *testing.T) {
		db := openConsoleTestDB(t)
		exec := &mockEnvExecutor{}
		poller := NewHeartbeatPoller(db, exec, t.TempDir())

		poller.Start()
		poller.Start() // Should be idempotent
		poller.Start() // Should be idempotent

		// Only one goroutine should be running
		require.True(t, poller.IsRunning())
		poller.Stop()
	})
}

// ============================================
// TestHeartbeatPoller_CheckHeartbeat
// ============================================

func TestHeartbeatPoller_CheckHeartbeat(t *testing.T) {
	t.Run("updates heartbeat when enrolled and console status succeeds", func(t *testing.T) {
		db := openConsoleTestDB(t)
		now := time.Now().UTC()

		// Create enrollment record with pending_acceptance status
		rec := &models.CrowdsecConsoleEnrollment{
			UUID:      "test-uuid",
			Status:    consoleStatusPendingAcceptance,
			AgentName: "test-agent",
			CreatedAt: now,
			UpdatedAt: now,
		}
		require.NoError(t, db.Create(rec).Error)

		// Mock executor returns console status showing enrolled
		exec := &mockEnvExecutor{
			responses: []struct {
				out []byte
				err error
			}{
				{out: []byte("You can successfully interact with the Console API.\nYour engine is enrolled and connected to the console."), err: nil},
			},
		}

		poller := NewHeartbeatPoller(db, exec, t.TempDir())

		ctx := context.Background()
		poller.checkHeartbeat(ctx)

		// Verify heartbeat was updated
		var updated models.CrowdsecConsoleEnrollment
		require.NoError(t, db.First(&updated).Error)
		require.NotNil(t, updated.LastHeartbeatAt, "LastHeartbeatAt should be set")
		require.Equal(t, 1, exec.getCallCount())
	})

	t.Run("handles errors gracefully without crashing", func(t *testing.T) {
		db := openConsoleTestDB(t)
		now := time.Now().UTC()

		// Create enrollment record
		rec := &models.CrowdsecConsoleEnrollment{
			UUID:      "test-uuid",
			Status:    consoleStatusEnrolled,
			AgentName: "test-agent",
			CreatedAt: now,
			UpdatedAt: now,
		}
		require.NoError(t, db.Create(rec).Error)

		// Mock executor returns error
		exec := &mockEnvExecutor{
			responses: []struct {
				out []byte
				err error
			}{
				{out: []byte("connection refused"), err: fmt.Errorf("exit status 1")},
			},
		}

		poller := NewHeartbeatPoller(db, exec, t.TempDir())

		ctx := context.Background()
		// Should not panic
		poller.checkHeartbeat(ctx)

		// Heartbeat should not be updated on error
		var updated models.CrowdsecConsoleEnrollment
		require.NoError(t, db.First(&updated).Error)
		require.Nil(t, updated.LastHeartbeatAt, "LastHeartbeatAt should remain nil on error")
	})

	t.Run("skips check when not enrolled", func(t *testing.T) {
		db := openConsoleTestDB(t)
		now := time.Now().UTC()

		// Create enrollment record with not_enrolled status
		rec := &models.CrowdsecConsoleEnrollment{
			UUID:      "test-uuid",
			Status:    consoleStatusNotEnrolled,
			AgentName: "test-agent",
			CreatedAt: now,
			UpdatedAt: now,
		}
		require.NoError(t, db.Create(rec).Error)

		exec := &mockEnvExecutor{}
		poller := NewHeartbeatPoller(db, exec, t.TempDir())

		ctx := context.Background()
		poller.checkHeartbeat(ctx)

		// Should not have called the executor
		require.Equal(t, 0, exec.getCallCount())
	})

	t.Run("skips check when no enrollment record exists", func(t *testing.T) {
		db := openConsoleTestDB(t)
		exec := &mockEnvExecutor{}
		poller := NewHeartbeatPoller(db, exec, t.TempDir())

		ctx := context.Background()
		poller.checkHeartbeat(ctx)

		// Should not have called the executor
		require.Equal(t, 0, exec.getCallCount())
	})
}

// ============================================
// TestHeartbeatPoller_StatusTransition
// ============================================

func TestHeartbeatPoller_StatusTransition(t *testing.T) {
	t.Run("transitions from pending_acceptance to enrolled when console shows enrolled", func(t *testing.T) {
		db := openConsoleTestDB(t)
		now := time.Now().UTC()

		// Create enrollment record with pending_acceptance status
		rec := &models.CrowdsecConsoleEnrollment{
			UUID:      "test-uuid",
			Status:    consoleStatusPendingAcceptance,
			AgentName: "test-agent",
			CreatedAt: now,
			UpdatedAt: now,
		}
		require.NoError(t, db.Create(rec).Error)

		// Mock executor returns console status showing enrolled
		exec := &mockEnvExecutor{
			responses: []struct {
				out []byte
				err error
			}{
				{out: []byte("You can enable the following options:\n- console_management: Receive orders from the console (default: enabled)\n\nYour engine is enrolled and connected to console."), err: nil},
			},
		}

		poller := NewHeartbeatPoller(db, exec, t.TempDir())

		ctx := context.Background()
		poller.checkHeartbeat(ctx)

		// Verify status transitioned to enrolled
		var updated models.CrowdsecConsoleEnrollment
		require.NoError(t, db.First(&updated).Error)
		assert.Equal(t, consoleStatusEnrolled, updated.Status)
		assert.NotNil(t, updated.EnrolledAt, "EnrolledAt should be set when transitioning to enrolled")
		assert.NotNil(t, updated.LastHeartbeatAt, "LastHeartbeatAt should be set")
	})

	t.Run("does not change status when already enrolled", func(t *testing.T) {
		db := openConsoleTestDB(t)
		enrolledTime := time.Now().UTC().Add(-24 * time.Hour)

		// Create enrollment record with enrolled status
		rec := &models.CrowdsecConsoleEnrollment{
			UUID:       "test-uuid",
			Status:     consoleStatusEnrolled,
			AgentName:  "test-agent",
			EnrolledAt: &enrolledTime,
			CreatedAt:  enrolledTime,
			UpdatedAt:  enrolledTime,
		}
		require.NoError(t, db.Create(rec).Error)

		// Mock executor returns console status showing enrolled
		exec := &mockEnvExecutor{
			responses: []struct {
				out []byte
				err error
			}{
				{out: []byte("Your engine is enrolled and connected to console."), err: nil},
			},
		}

		poller := NewHeartbeatPoller(db, exec, t.TempDir())

		ctx := context.Background()
		poller.checkHeartbeat(ctx)

		// Verify status remains enrolled but heartbeat is updated
		var updated models.CrowdsecConsoleEnrollment
		require.NoError(t, db.First(&updated).Error)
		assert.Equal(t, consoleStatusEnrolled, updated.Status)
		// EnrolledAt should not change (was already set)
		assert.Equal(t, enrolledTime.Unix(), updated.EnrolledAt.Unix())
		// LastHeartbeatAt should be updated
		assert.NotNil(t, updated.LastHeartbeatAt)
	})

	t.Run("does not transition when console output does not indicate enrolled", func(t *testing.T) {
		db := openConsoleTestDB(t)
		now := time.Now().UTC()

		// Create enrollment record with pending_acceptance status
		rec := &models.CrowdsecConsoleEnrollment{
			UUID:      "test-uuid",
			Status:    consoleStatusPendingAcceptance,
			AgentName: "test-agent",
			CreatedAt: now,
			UpdatedAt: now,
		}
		require.NoError(t, db.Create(rec).Error)

		// Mock executor returns console status NOT showing enrolled
		exec := &mockEnvExecutor{
			responses: []struct {
				out []byte
				err error
			}{
				{out: []byte("You are not enrolled to the console"), err: nil},
			},
		}

		poller := NewHeartbeatPoller(db, exec, t.TempDir())

		ctx := context.Background()
		poller.checkHeartbeat(ctx)

		// Verify status remains pending_acceptance
		var updated models.CrowdsecConsoleEnrollment
		require.NoError(t, db.First(&updated).Error)
		assert.Equal(t, consoleStatusPendingAcceptance, updated.Status)
		// LastHeartbeatAt should NOT be set since not enrolled
		assert.Nil(t, updated.LastHeartbeatAt)
	})
}

// ============================================
// TestHeartbeatPoller_Interval
// ============================================

func TestHeartbeatPoller_Interval(t *testing.T) {
	t.Run("default interval is 5 minutes", func(t *testing.T) {
		db := openConsoleTestDB(t)
		exec := &mockEnvExecutor{}
		poller := NewHeartbeatPoller(db, exec, t.TempDir())

		assert.Equal(t, 5*time.Minute, poller.interval)
	})

	t.Run("SetInterval changes the interval", func(t *testing.T) {
		db := openConsoleTestDB(t)
		exec := &mockEnvExecutor{}
		poller := NewHeartbeatPoller(db, exec, t.TempDir())

		poller.SetInterval(1 * time.Minute)
		assert.Equal(t, 1*time.Minute, poller.interval)
	})
}

// ============================================
// TestHeartbeatPoller_ConcurrentSafety
// ============================================

func TestHeartbeatPoller_ConcurrentSafety(t *testing.T) {
	t.Run("concurrent Start and Stop calls are safe", func(t *testing.T) {
		db := openConsoleTestDB(t)
		exec := &mockEnvExecutor{}
		poller := NewHeartbeatPoller(db, exec, t.TempDir())
		poller.SetInterval(10 * time.Millisecond)

		// Run multiple goroutines trying to start/stop concurrently
		done := make(chan struct{})
		var running atomic.Int32

		for i := 0; i < 10; i++ {
			go func() {
				running.Add(1)
				poller.Start()
				time.Sleep(5 * time.Millisecond)
				poller.Stop()
				running.Add(-1)
			}()
		}

		// Wait for all goroutines to finish
		time.Sleep(200 * time.Millisecond)
		close(done)

		// Final state should be stopped
		require.Eventually(t, func() bool {
			return !poller.IsRunning()
		}, time.Second, 10*time.Millisecond)
	})
}
