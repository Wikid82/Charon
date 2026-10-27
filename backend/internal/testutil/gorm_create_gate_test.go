package testutil

import (
	"errors"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type gateProbeRow struct {
	ID    uint
	Label string
}

type gateOtherRow struct {
	ID uint
}

// openGateDB returns a file-backed database with the probe tables migrated.
func openGateDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "gate.db")), &gorm.Config{})
	require.NoError(t, err)
	require.NoError(t, db.AutoMigrate(&gateProbeRow{}, &gateOtherRow{}))
	return db
}

func countProbeRows(t *testing.T, db *gorm.DB) int64 {
	t.Helper()
	var count int64
	require.NoError(t, db.Model(&gateProbeRow{}).Count(&count).Error)
	return count
}

func TestGateCreate_HoldsMatchingInsertUntilRelease(t *testing.T) {
	db := openGateDB(t)
	entered, release := GateCreate[gateProbeRow](t, db, nil)

	done := make(chan error, 1)
	go func() { done <- db.Create(&gateProbeRow{Label: "held"}).Error }()

	AwaitGate(t, entered)
	require.Zero(t, countProbeRows(t, db), "the gated insert must not write before release")

	release()
	require.NoError(t, <-done)
	require.Equal(t, int64(1), countProbeRows(t, db))
}

func TestGateCreate_InsertErrorAbortsWrite(t *testing.T) {
	db := openGateDB(t)
	refusal := errors.New("the gate refuses this insert")
	entered, release := GateCreate[gateProbeRow](t, db, refusal)

	done := make(chan error, 1)
	go func() { done <- db.Create(&gateProbeRow{Label: "refused"}).Error }()

	AwaitGate(t, entered)
	release()
	require.ErrorIs(t, <-done, refusal)
	require.Zero(t, countProbeRows(t, db), "a refused insert must write nothing")
}

func TestGateCreate_LeavesOtherModelsAlone(t *testing.T) {
	db := openGateDB(t)
	entered, _ := GateCreate[gateProbeRow](t, db, nil)

	require.NoError(t, db.Create(&gateOtherRow{}).Error, "an insert of another model must not wait at the gate")
	select {
	case <-entered:
		t.Fatal("the gate must not start for another model")
	default:
	}
}

// recordingTB records a Fatal call instead of stopping the test, so a failing AwaitGate can be observed.
type recordingTB struct {
	testing.TB
	failed bool
}

func (r *recordingTB) Fatal(...any) { r.failed = true }

func TestAwaitGate_FailsWhenInsertNeverStarts(t *testing.T) {
	previous := gateWaitTimeout
	gateWaitTimeout = 20 * time.Millisecond
	t.Cleanup(func() { gateWaitTimeout = previous })

	recorder := &recordingTB{TB: t}
	AwaitGate(recorder, make(chan struct{}))
	require.True(t, recorder.failed, "AwaitGate must fail the test when the gated insert never starts")
}

// recordingFatalTB records a Fatalf call instead of stopping the test, so a failing registration can be observed.
type recordingFatalTB struct {
	testing.TB
	fatal string
}

func (r *recordingFatalTB) Fatalf(format string, args ...any) { r.fatal = fmt.Sprintf(format, args...) }

func TestGateCreate_ReportsRegistrationFailure(t *testing.T) {
	db := openGateDB(t)
	// The next gate takes the sequence number after the current one, so a callback already holding that name
	// makes its registration fail.
	taken := fmt.Sprintf("testutil:gate_create_%d", gateSeq.Load()+1)
	require.NoError(t, db.Callback().Create().Register(taken, func(*gorm.DB) {}))

	recorder := &recordingFatalTB{TB: t}
	GateCreate[gateProbeRow](recorder, db, nil)

	require.Contains(t, recorder.fatal, "registering the create gate")
}
