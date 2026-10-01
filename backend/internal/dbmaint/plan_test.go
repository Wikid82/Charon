package dbmaint

import (
	"context"
	"testing"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const pageSize4K = 4096

// statsWith builds a mode-0 4 KiB-page Stats with the given counts.
func statsWith(pageCount, freelist int64) Stats {
	return Stats{PageSize: pageSize4K, PageCount: pageCount, FreelistCount: freelist}
}

func TestDecide_Table(t *testing.T) {
	const pagesPerMiB = (1 << 20) / pageSize4K

	tests := []struct {
		name       string
		in         Inputs
		wantRun    bool
		wantReason Reason
		wantClear  bool
	}{
		{
			name:       "env off beats everything including the flag",
			in:         Inputs{EnvMode: config.DBCompactOff, FlagRequested: true, Stats: statsWith(128000, 100000)},
			wantReason: ReasonDisabledByEnv,
		},
		{
			name:       "already incremental",
			in:         Inputs{EnvMode: config.DBCompactAuto, Stats: Stats{PageSize: pageSize4K, PageCount: 1000, AutoVacuum: AutoVacuumIncremental}},
			wantReason: ReasonAlreadyOptimized,
		},
		{
			name:       "already incremental with flag clears the flag",
			in:         Inputs{EnvMode: config.DBCompactAuto, FlagRequested: true, Stats: Stats{PageSize: pageSize4K, PageCount: 1000, AutoVacuum: AutoVacuumIncremental}},
			wantReason: ReasonAlreadyOptimized,
			wantClear:  true,
		},
		{
			name:    "exactly 20% free and exactly 100 MiB runs",
			in:      Inputs{EnvMode: config.DBCompactAuto, Stats: statsWith(128000, 25600)},
			wantRun: true,
		},
		{
			name:       "19.99% free is below threshold",
			in:         Inputs{EnvMode: config.DBCompactAuto, Stats: statsWith(128000, 25587)},
			wantReason: ReasonBelowThreshold,
		},
		{
			name:       "20% free but 99 MiB reclaimable is below the floor",
			in:         Inputs{EnvMode: config.DBCompactAuto, Stats: statsWith(126976, 25344)},
			wantReason: ReasonBelowThreshold,
		},
		{
			name:    "1 GiB reclaimable at 5% free runs via the OR trigger",
			in:      Inputs{EnvMode: config.DBCompactAuto, Stats: statsWith(5242880, 1024*pagesPerMiB)},
			wantRun: true,
		},
		{
			name:       "1 GiB minus one page at 5% free does not run",
			in:         Inputs{EnvMode: config.DBCompactAuto, Stats: statsWith(5242880, 1024*pagesPerMiB-1)},
			wantReason: ReasonBelowThreshold,
		},
		{
			name:       "flag below the floor is nothing to reclaim and is cleared",
			in:         Inputs{EnvMode: config.DBCompactAuto, FlagRequested: true, Stats: statsWith(126976, 25344)},
			wantReason: ReasonNothingToReclaim,
			wantClear:  true,
		},
		{
			name:    "flag runs below the ratio once the floor is met",
			in:      Inputs{EnvMode: config.DBCompactAuto, FlagRequested: true, Stats: statsWith(1280000, 25600)},
			wantRun: true,
		},
		{
			name:       "counter at max backs off without the flag",
			in:         Inputs{EnvMode: config.DBCompactAuto, Attempts: MaxConvertAttempts, Stats: statsWith(128000, 64000)},
			wantReason: ReasonTooManyFailures,
		},
		{
			name:    "counter just below max still runs",
			in:      Inputs{EnvMode: config.DBCompactAuto, Attempts: MaxConvertAttempts - 1, Stats: statsWith(128000, 64000)},
			wantRun: true,
		},
		{
			name:    "flag with counter at max runs",
			in:      Inputs{EnvMode: config.DBCompactAuto, FlagRequested: true, Attempts: MaxConvertAttempts, Stats: statsWith(128000, 64000)},
			wantRun: true,
		},
		{
			name:       "empty database is below threshold",
			in:         Inputs{EnvMode: config.DBCompactAuto, Stats: Stats{PageSize: pageSize4K}},
			wantReason: ReasonBelowThreshold,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Decide(tt.in)
			assert.Equal(t, tt.wantRun, got.Run)
			assert.Equal(t, tt.wantReason, got.Reason)
			assert.Equal(t, tt.wantClear, got.ClearFlag)
		})
	}
}

func TestDecide_InsufficientDiskReportsNumbers(t *testing.T) {
	in := Inputs{
		EnvMode: config.DBCompactAuto,
		Stats:   statsWith(128000, 64000),
		Disk:    DiskReport{TmpRequired: 500, TmpAvailable: 400, DataRequired: 10, DataAvailable: 1000},
	}
	got := Decide(in)
	assert.False(t, got.Run)
	assert.Equal(t, ReasonInsufficientDisk, got.Reason)
	assert.EqualValues(t, 500, got.RequiredBytes)
	assert.EqualValues(t, 400, got.AvailableBytes)
}

func TestDiskReport_Shortfall(t *testing.T) {
	tests := []struct {
		name         string
		d            DiskReport
		wantShort    bool
		wantRequired int64
		wantAvail    int64
	}{
		{"zero report is fine", DiskReport{}, false, 0, 0},
		{"separate filesystems, both fit", DiskReport{TmpRequired: 5, TmpAvailable: 5, DataRequired: 7, DataAvailable: 7}, false, 0, 0},
		{"separate filesystems, tmp short", DiskReport{TmpRequired: 6, TmpAvailable: 5, DataRequired: 7, DataAvailable: 70}, true, 6, 5},
		{"separate filesystems, data short", DiskReport{TmpRequired: 5, TmpAvailable: 50, DataRequired: 8, DataAvailable: 7}, true, 8, 7},
		{"same filesystem sums both needs", DiskReport{SameFS: true, TmpRequired: 6, DataRequired: 6, TmpAvailable: 10, DataAvailable: 10}, true, 12, 10},
		{"same filesystem fits exactly", DiskReport{SameFS: true, TmpRequired: 5, DataRequired: 5, TmpAvailable: 10, DataAvailable: 10}, false, 10, 10},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			required, avail, short := tt.d.Shortfall()
			assert.Equal(t, tt.wantShort, short)
			if tt.wantShort || tt.d.SameFS {
				assert.Equal(t, tt.wantRequired, required)
				assert.Equal(t, tt.wantAvail, avail)
			}
		})
	}
}

func TestEstimateDiskNeed(t *testing.T) {
	s := Stats{PageSize: pageSize4K, PageCount: 1000, FreelistCount: 200, WALBytes: 1234}
	live := int64(800 * pageSize4K)

	tmp, data := EstimateDiskNeed(s)

	assert.Equal(t, int64(float64(live)*DiskSafetyMultiplier)+DiskSlackBytes, tmp)
	assert.Equal(t, tmp+1234, data)
}

func TestPlan_RealDatabase(t *testing.T) {
	db, path := newScratchDB(t, scratchOpts{rows: 400, rowBytes: 100000, keepEvery: 10})

	res, err := Plan(context.Background(), db, PlanConfig{DBPath: path, EnvMode: config.DBCompactAuto})
	require.NoError(t, err)

	assert.Greater(t, res.Stats.FreelistCount, int64(0))
	assert.NotZero(t, res.Disk.DataAvailable)
	// A 40 MB scratch database is far below the 100 MiB floor.
	assert.False(t, res.Decision.Run)
	assert.Equal(t, ReasonBelowThreshold, res.Decision.Reason)

	res, err = Plan(context.Background(), db, PlanConfig{DBPath: path, EnvMode: config.DBCompactOff})
	require.NoError(t, err)
	assert.Equal(t, ReasonDisabledByEnv, res.Decision.Reason)
}

func TestPlan_ErrorsAreReturned(t *testing.T) {
	db, path := newScratchDB(t, scratchOpts{rows: 10})
	require.NoError(t, db.Close())

	_, err := Plan(context.Background(), db, PlanConfig{DBPath: path})
	require.Error(t, err)

	db2, _ := newScratchDB(t, scratchOpts{rows: 10})
	_, err = Plan(context.Background(), db2, PlanConfig{DBPath: "/nonexistent-dir/x.db"})
	require.Error(t, err)
}

func TestPlan_BadTempDirIsAnError(t *testing.T) {
	db, path := newScratchDB(t, scratchOpts{rows: 10})
	t.Setenv("SQLITE_TMPDIR", "/definitely/not/a/dir")

	_, err := Plan(context.Background(), db, PlanConfig{DBPath: path})
	require.Error(t, err)
}
