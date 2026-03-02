package routes

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

type testUptimeBootstrapService struct {
	cleanupErr error
	syncErr    error

	cleanupCalls  int
	syncCalls     int
	checkAllCalls int
}

func (s *testUptimeBootstrapService) CleanupStaleFailureCounts() error {
	s.cleanupCalls++
	return s.cleanupErr
}

func (s *testUptimeBootstrapService) SyncMonitors() error {
	s.syncCalls++
	return s.syncErr
}

func (s *testUptimeBootstrapService) CheckAll() {
	s.checkAllCalls++
}

func TestRunInitialUptimeBootstrap_Disabled_DoesNothing(t *testing.T) {
	svc := &testUptimeBootstrapService{}

	warnLogs := 0
	errorLogs := 0
	runInitialUptimeBootstrap(
		false,
		svc,
		func(err error, msg string) { warnLogs++ },
		func(err error, msg string) { errorLogs++ },
	)

	assert.Equal(t, 0, svc.cleanupCalls)
	assert.Equal(t, 0, svc.syncCalls)
	assert.Equal(t, 0, svc.checkAllCalls)
	assert.Equal(t, 0, warnLogs)
	assert.Equal(t, 0, errorLogs)
}

func TestRunInitialUptimeBootstrap_Enabled_HappyPath(t *testing.T) {
	svc := &testUptimeBootstrapService{}

	warnLogs := 0
	errorLogs := 0
	runInitialUptimeBootstrap(
		true,
		svc,
		func(err error, msg string) { warnLogs++ },
		func(err error, msg string) { errorLogs++ },
	)

	assert.Equal(t, 1, svc.cleanupCalls)
	assert.Equal(t, 1, svc.syncCalls)
	assert.Equal(t, 1, svc.checkAllCalls)
	assert.Equal(t, 0, warnLogs)
	assert.Equal(t, 0, errorLogs)
}

func TestRunInitialUptimeBootstrap_Enabled_CleanupError_StillProceeds(t *testing.T) {
	svc := &testUptimeBootstrapService{cleanupErr: errors.New("cleanup failed")}

	warnLogs := 0
	errorLogs := 0
	runInitialUptimeBootstrap(
		true,
		svc,
		func(err error, msg string) { warnLogs++ },
		func(err error, msg string) { errorLogs++ },
	)

	assert.Equal(t, 1, svc.cleanupCalls)
	assert.Equal(t, 1, svc.syncCalls)
	assert.Equal(t, 1, svc.checkAllCalls)
	assert.Equal(t, 1, warnLogs)
	assert.Equal(t, 0, errorLogs)
}

func TestRunInitialUptimeBootstrap_Enabled_SyncError_StillChecksAll(t *testing.T) {
	svc := &testUptimeBootstrapService{syncErr: errors.New("sync failed")}

	warnLogs := 0
	errorLogs := 0
	runInitialUptimeBootstrap(
		true,
		svc,
		func(err error, msg string) { warnLogs++ },
		func(err error, msg string) { errorLogs++ },
	)

	assert.Equal(t, 1, svc.cleanupCalls)
	assert.Equal(t, 1, svc.syncCalls)
	assert.Equal(t, 1, svc.checkAllCalls)
	assert.Equal(t, 0, warnLogs)
	assert.Equal(t, 1, errorLogs)
}
