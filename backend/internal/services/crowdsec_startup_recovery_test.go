package services

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/Wikid82/charon/backend/internal/crowdsec"
	"github.com/Wikid82/charon/backend/internal/models"
)

// orderRecordingExecutor records what the configuration looked like at the moment the engine was started.
type orderRecordingExecutor struct {
	markerPath    string
	markerAtStart string
	started       bool
}

func (e *orderRecordingExecutor) Start(context.Context, string, string) (int, error) {
	b, _ := os.ReadFile(e.markerPath) // #nosec G304 -- test path
	e.markerAtStart, e.started = string(b), true
	return 4242, nil
}
func (e *orderRecordingExecutor) Stop(context.Context, string) error { return nil }
func (e *orderRecordingExecutor) Status(context.Context, string) (running bool, pid int, err error) {
	return e.started, 4242, nil
}

func recoveryDB(t *testing.T, mode string) *gorm.DB {
	t.Helper()
	db := setupCrowdsecTestDB(t)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	sqlDB.SetMaxOpenConns(1)
	require.NoError(t, db.AutoMigrate(&models.CrowdsecApplyJob{}))
	require.NoError(t, db.Create(&models.SecurityConfig{UUID: "default", Name: "d", Enabled: true, CrowdSecMode: mode}).Error)
	return db
}

func seedRunningJob(t *testing.T, db *gorm.DB, backupPath string) string {
	t.Helper()
	job := &models.CrowdsecApplyJob{Slug: "a/b", State: models.CrowdsecApplyJobRunning, BackupPath: backupPath}
	require.NoError(t, crowdsec.NewApplyJobRepository(db).Create(context.Background(), job))
	return job.ID
}

func TestReconcileCrowdSecOnStartup_RestoresTheSnapshotOfAnInterruptedInstallBeforeStartingTheEngine(t *testing.T) {
	db := recoveryDB(t, "local")
	binPath, dataDir, cleanup := setupCrowdsecTestFixtures(t)
	defer cleanup()

	snap := filepath.Join(t.TempDir(), "snapshot")
	require.NoError(t, os.MkdirAll(filepath.Join(snap, "config"), 0o750))
	require.NoError(t, os.WriteFile(filepath.Join(snap, "config", "marker"), []byte("snapshot"), 0o600))
	marker := filepath.Join(dataDir, "config", "marker")
	require.NoError(t, os.WriteFile(marker, []byte("half-installed"), 0o600))
	require.NoError(t, os.WriteFile(filepath.Join(dataDir, "config", "leftover"), []byte("x"), 0o600))
	id := seedRunningJob(t, db, snap)

	exec := &orderRecordingExecutor{markerPath: marker}
	ReconcileCrowdSecOnStartup(db, exec, binPath, dataDir, &mockCommandExecutor{})

	assert.True(t, exec.started, "the engine start proceeds after the recovery")
	assert.Equal(t, "snapshot", exec.markerAtStart, "the snapshot is back before the engine reads the configuration")
	assert.NoFileExists(t, filepath.Join(dataDir, "config", "leftover"))
	job, err := crowdsec.NewApplyJobRepository(db).Get(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, models.CrowdsecApplyJobFailed, job.State)
	assert.Equal(t, models.CrowdsecApplyErrInterrupted, job.ErrorCode)
	assert.NotNil(t, job.FinishedAt)
}

func TestReconcileCrowdSecOnStartup_MissingSnapshotOnlyWarnsAndNeverWipesTheConfiguration(t *testing.T) {
	// Disabled mode: the recovery runs whatever the engine mode is.
	db := recoveryDB(t, "disabled")
	binPath, dataDir, cleanup := setupCrowdsecTestFixtures(t)
	defer cleanup()
	marker := filepath.Join(dataDir, "config", "marker")
	require.NoError(t, os.WriteFile(marker, []byte("keep me"), 0o600))
	id := seedRunningJob(t, db, filepath.Join(t.TempDir(), "gone"))
	noSnapshot := seedRunningJob(t, db, "")

	ReconcileCrowdSecOnStartup(db, &mockCrowdsecExecutor{}, binPath, dataDir, &mockCommandExecutor{})

	b, err := os.ReadFile(marker) // #nosec G304 -- test path
	require.NoError(t, err)
	assert.Equal(t, "keep me", string(b), "the configuration is untouched")
	repo := crowdsec.NewApplyJobRepository(db)
	job, err := repo.Get(context.Background(), id)
	require.NoError(t, err)
	assert.Equal(t, models.CrowdsecApplyErrInterrupted, job.ErrorCode)
	assert.Contains(t, job.Warnings, "no longer exists")
	other, err := repo.Get(context.Background(), noSnapshot)
	require.NoError(t, err)
	assert.Equal(t, models.CrowdsecApplyJobFailed, other.State, "a job that never took a snapshot is still failed")
}

func TestRecoverInterruptedInstalls_SkipsWhenThereIsNoJobTableOrDB(t *testing.T) {
	recoverInterruptedInstalls(nil, t.TempDir())
	recoverInterruptedInstalls(setupCrowdsecTestDB(t), t.TempDir()) // no crowdsec_apply_jobs table yet (first start after an upgrade)
}

func TestRecoverInterruptedInstalls_PrunesOldSnapshotsAfterRecovery(t *testing.T) {
	db := recoveryDB(t, "disabled")
	dataDir := filepath.Join(t.TempDir(), "crowdsec")
	require.NoError(t, os.MkdirAll(dataDir, 0o750))
	var snaps []string
	for _, ts := range []string{"20260101-000001.000000", "20260101-000002.000000", "20260101-000003.000000", "20260101-000004.000000", "20260101-000005.000000", "20260101-000006.000000", "20260101-000007.000000"} {
		p := dataDir + "." + crowdsec.BackupKindSnapshot + "." + ts
		require.NoError(t, os.MkdirAll(p, 0o750))
		snaps = append(snaps, p)
	}
	// A recovery that cannot complete (the snapshot is not valid) keeps its oldest snapshot for manual recovery.
	seedRunningJob(t, db, snaps[0])

	recoverInterruptedInstalls(db, dataDir)

	assert.DirExists(t, snaps[0], "the snapshot of the unrecovered job is protected")
	assert.NoDirExists(t, snaps[1])
	assert.DirExists(t, snaps[2])
	assert.DirExists(t, snaps[6])
}
