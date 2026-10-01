package dbmaint

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/Wikid82/charon/backend/internal/config"
)

// Reason names why a conversion is skipped. The values are part of the status
// and notice contract (snake_case).
type Reason string

// Skip reasons produced by Decide.
const (
	ReasonDisabledByEnv    Reason = "disabled_by_env"
	ReasonAlreadyOptimized Reason = "already_optimized"
	ReasonNothingToReclaim Reason = "nothing_to_reclaim"
	ReasonBelowThreshold   Reason = "below_threshold"
	ReasonTooManyFailures  Reason = "too_many_failures"
	ReasonInsufficientDisk Reason = "insufficient_disk"
)

// DiskReport holds the free-space figures of the conversion estimate. The
// temporary file of a VACUUM and the copy-back through the WAL both need room;
// on a shared filesystem the two requirements add up.
type DiskReport struct {
	TmpRequired   int64
	TmpAvailable  int64
	DataRequired  int64
	DataAvailable int64
	SameFS        bool
}

// Shortfall reports whether the disk is too small and, if so, the required and
// available bytes of the filesystem that falls short.
func (d DiskReport) Shortfall() (required, available int64, short bool) {
	if d.SameFS {
		required = d.TmpRequired + d.DataRequired
		return required, d.DataAvailable, required > d.DataAvailable
	}
	if d.TmpRequired > d.TmpAvailable {
		return d.TmpRequired, d.TmpAvailable, true
	}
	if d.DataRequired > d.DataAvailable {
		return d.DataRequired, d.DataAvailable, true
	}
	return 0, 0, false
}

// Inputs is everything Decide needs; it performs no I/O.
type Inputs struct {
	// EnvMode is config.DBCompactAuto or config.DBCompactOff.
	EnvMode string
	// FlagRequested is the persisted "reclaim on next restart" request.
	FlagRequested bool
	// Attempts counts consecutive failed conversions recorded for this file.
	Attempts int
	Stats    Stats
	Disk     DiskReport
}

// Decision is the outcome of steps 1-4 and 6 of the boot evaluation. Steps 5
// (boot quick_check) and 7 (writer-lock probe) belong to the runner.
type Decision struct {
	Run    bool
	Reason Reason
	// ClearFlag asks the caller to delete a set request: there is nothing left
	// to optimize (already optimized, nothing to reclaim).
	ClearFlag      bool
	RequiredBytes  int64
	AvailableBytes int64
}

func skip(reason Reason) Decision { return Decision{Reason: reason} }

// Decide evaluates the boot decision table. Order: env off, already
// incremental, the user's flag (the floor still applies), the automatic
// thresholds and the failure back-off, then free disk.
func Decide(in Inputs) Decision {
	if in.EnvMode == config.DBCompactOff {
		return skip(ReasonDisabledByEnv)
	}
	if in.Stats.IsIncremental() {
		d := skip(ReasonAlreadyOptimized)
		d.ClearFlag = in.FlagRequested
		return d
	}

	reclaimable := in.Stats.ReclaimableBytes()
	switch {
	case in.FlagRequested:
		if reclaimable < MinReclaimableBytes {
			d := skip(ReasonNothingToReclaim)
			d.ClearFlag = true
			return d
		}
	default:
		worthwhile := reclaimable >= MinReclaimableBytes &&
			(in.Stats.FreeRatio() >= MinFreeRatio || reclaimable >= ReclaimableTriggerBytes)
		if !worthwhile {
			return skip(ReasonBelowThreshold)
		}
		if in.Attempts >= MaxConvertAttempts {
			return skip(ReasonTooManyFailures)
		}
	}

	if required, available, short := in.Disk.Shortfall(); short {
		d := skip(ReasonInsufficientDisk)
		d.RequiredBytes, d.AvailableBytes = required, available
		return d
	}
	return Decision{Run: true}
}

// PlanConfig carries the inputs of Plan that come from outside the database
// file: the path, the environment mode and the persisted request state.
type PlanConfig struct {
	DBPath        string
	EnvMode       string
	FlagRequested bool
	Attempts      int
}

// PlanResult is what Plan learned and decided.
type PlanResult struct {
	Stats    Stats
	Disk     DiskReport
	Decision Decision
}

// Plan gathers the file statistics and the disk report, then runs Decide. It
// reads, never writes.
func Plan(ctx context.Context, q Querier, cfg PlanConfig) (PlanResult, error) {
	stats, err := Inspect(ctx, q, cfg.DBPath)
	if err != nil {
		return PlanResult{}, fmt.Errorf("inspect database: %w", err)
	}
	disk, err := BuildDiskReport(stats, filepath.Dir(filepath.Clean(cfg.DBPath)))
	if err != nil {
		return PlanResult{}, fmt.Errorf("disk report: %w", err)
	}
	return PlanResult{
		Stats: stats,
		Disk:  disk,
		Decision: Decide(Inputs{
			EnvMode:       cfg.EnvMode,
			FlagRequested: cfg.FlagRequested,
			Attempts:      cfg.Attempts,
			Stats:         stats,
			Disk:          disk,
		}),
	}, nil
}

// EstimateDiskNeed returns the bytes the conversion needs: the rebuilt copy
// (tmpBytes, on the temp directory's filesystem) and the copy-back through the
// WAL (dataBytes, on the database directory's filesystem).
func EstimateDiskNeed(s Stats) (tmpBytes, dataBytes int64) {
	tmpBytes = int64(float64(s.LiveBytes())*DiskSafetyMultiplier) + DiskSlackBytes
	return tmpBytes, tmpBytes + s.WALBytes
}

// BuildDiskReport measures free space on the directory SQLite actually uses
// for temporary files and on the database directory.
func BuildDiskReport(s Stats, dbDir string) (DiskReport, error) {
	tmpDir, err := EffectiveTempDir()
	if err != nil {
		return DiskReport{}, err
	}
	tmpAvail, err := AvailableBytes(tmpDir)
	if err != nil {
		return DiskReport{}, err
	}
	dataAvail, err := AvailableBytes(dbDir)
	if err != nil {
		return DiskReport{}, err
	}
	same, err := SameFilesystem(tmpDir, dbDir)
	if err != nil {
		return DiskReport{}, err
	}
	tmpNeed, dataNeed := EstimateDiskNeed(s)
	return DiskReport{
		TmpRequired:   tmpNeed,
		TmpAvailable:  tmpAvail,
		DataRequired:  dataNeed,
		DataAvailable: dataAvail,
		SameFS:        same,
	}, nil
}
