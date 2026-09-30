package handlers

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"path/filepath"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Wikid82/charon/backend/internal/config"
	"github.com/Wikid82/charon/backend/internal/dbmaint"
	"github.com/Wikid82/charon/backend/internal/logger"
)

// Codes of the database maintenance API. They are part of the contract with
// the UI: the notice codes and the 409 error codes (GH #1422).
const (
	noticeRestartToOptimize = "restart_to_optimize"
	noticeInsufficientDisk  = "insufficient_disk"
	noticeTooManyFailures   = "too_many_failures"
	noticeDatabaseBusy      = "database_busy"
	noticeDisabledByEnv     = "disabled_by_env"

	severityInfo    = "info"
	severityWarning = "warning"

	codeNothingToOptimize = "nothing_to_optimize"
	codeDisabledByEnv     = "disabled_by_env"
)

// DatabaseMaintenanceHandler serves the admin-only database card: file size,
// what could be reclaimed, the notice that applies right now and the
// "reclaim space on next restart" request.
type DatabaseMaintenanceHandler struct {
	store   *dbmaint.Store
	dbPath  string
	envMode string

	// Seams: the defaults read the real database file and disk.
	inspect           func(ctx context.Context) (dbmaint.Stats, error)
	diskReport        func(stats dbmaint.Stats) (dbmaint.DiskReport, error)
	conversionEnabled func() bool
}

// NewDatabaseMaintenanceHandler builds the handler for the database at dbPath.
// envMode is the CHARON_DB_COMPACT_ON_START value (config.DBCompactAuto or
// config.DBCompactOff).
func NewDatabaseMaintenanceHandler(db *sql.DB, dbPath, envMode string) *DatabaseMaintenanceHandler {
	dbPath = filepath.Clean(dbPath)
	return &DatabaseMaintenanceHandler{
		store:   dbmaint.NewStore(db),
		dbPath:  dbPath,
		envMode: envMode,
		inspect: func(ctx context.Context) (dbmaint.Stats, error) {
			return dbmaint.Inspect(ctx, db, dbPath)
		},
		diskReport: func(stats dbmaint.Stats) (dbmaint.DiskReport, error) {
			return dbmaint.BuildDiskReport(stats, filepath.Dir(dbPath))
		},
		conversionEnabled: dbmaint.ConversionEnabled,
	}
}

type databaseLastResult struct {
	At          time.Time `json:"at"`
	Outcome     string    `json:"outcome"`
	Reason      string    `json:"reason"`
	BytesBefore int64     `json:"bytes_before"`
	BytesAfter  int64     `json:"bytes_after"`
}

type databaseNotice struct {
	Code             string `json:"code"`
	Severity         string `json:"severity"`
	ReclaimableBytes int64  `json:"reclaimable_bytes"`
	RequiredBytes    *int64 `json:"required_bytes,omitempty"`
	AvailableBytes   *int64 `json:"available_bytes,omitempty"`
}

type databaseStatusResponse struct {
	SizeBytes          int64               `json:"size_bytes"`
	WALBytes           int64               `json:"wal_bytes"`
	ReclaimableBytes   int64               `json:"reclaimable_bytes"`
	AutoVacuum         string              `json:"auto_vacuum"`
	EnvMode            string              `json:"env_mode"`
	CompactRequested   bool                `json:"compact_requested"`
	CanRequestOptimize bool                `json:"can_request_optimize"`
	DiskFreeBytes      int64               `json:"disk_free_bytes"`
	LastResult         *databaseLastResult `json:"last_result"`
	Notice             *databaseNotice     `json:"notice"`
}

func autoVacuumName(mode int) string {
	switch mode {
	case dbmaint.AutoVacuumNone:
		return "none"
	case dbmaint.AutoVacuumIncremental:
		return "incremental"
	default:
		return "full"
	}
}

func (h *DatabaseMaintenanceHandler) fail(c *gin.Context, what string, err error) {
	logger.Log().WithError(err).Error("database maintenance: " + what)
	c.JSON(http.StatusInternalServerError, gin.H{"error": "database maintenance status is unavailable"})
}

// canRequestOptimize is false when the request could not do anything: the
// environment disables optimization, the file is already incremental, or less
// than the reclaim floor is free.
func (h *DatabaseMaintenanceHandler) canRequestOptimize(stats dbmaint.Stats) bool {
	return h.envMode != config.DBCompactOff &&
		!stats.IsIncremental() &&
		stats.ReclaimableBytes() >= dbmaint.MinReclaimableBytes
}

// GetStatus returns the card data. The notice is computed fresh on every call
// from a cheap Inspect, never from a boot snapshot: the pruner frees pages
// without a restart.
// GET /api/v1/system/database
func (h *DatabaseMaintenanceHandler) GetStatus(c *gin.Context) {
	ctx := c.Request.Context()

	stats, err := h.inspect(ctx)
	if err != nil {
		h.fail(c, "could not inspect the database", err)
		return
	}
	fileID, err := dbmaint.FileID(h.dbPath)
	if err != nil {
		h.fail(c, "could not identify the database file", err)
		return
	}
	state, err := h.store.Peek(ctx, fileID)
	if err != nil {
		h.fail(c, "could not read the maintenance state", err)
		return
	}
	disk, err := h.diskReport(stats)
	if err != nil {
		// Without a disk figure the disk-dependent notice cannot be judged; the
		// card still renders with what is known.
		logger.Log().WithError(err).Warn("database maintenance: could not measure free disk space")
		disk = dbmaint.DiskReport{}
	}

	resp := databaseStatusResponse{
		SizeBytes:          stats.MainBytes,
		WALBytes:           stats.WALBytes,
		ReclaimableBytes:   stats.ReclaimableBytes(),
		AutoVacuum:         autoVacuumName(stats.AutoVacuum),
		EnvMode:            h.envMode,
		CompactRequested:   state.FlagRequested,
		CanRequestOptimize: h.canRequestOptimize(stats),
		DiskFreeBytes:      disk.DataAvailable,
		Notice:             h.notice(stats, disk, state),
	}
	if last := state.LastResult; last != nil {
		resp.LastResult = &databaseLastResult{
			At:          last.At,
			Outcome:     string(last.Outcome),
			Reason:      string(last.Reason),
			BytesBefore: last.BytesBefore,
			BytesAfter:  last.BytesAfter,
		}
	}
	c.JSON(http.StatusOK, resp)
}

// notice returns the one thing worth telling the admin right now, or nil. It
// judges with the same Decide the boot path uses, so the two cannot disagree.
func (h *DatabaseMaintenanceHandler) notice(stats dbmaint.Stats, disk dbmaint.DiskReport, state dbmaint.State) *databaseNotice {
	in := dbmaint.Inputs{
		EnvMode:       h.envMode,
		FlagRequested: state.FlagRequested,
		Attempts:      state.Attempts,
		Stats:         stats,
		Disk:          disk,
	}
	reclaimable := stats.ReclaimableBytes()
	n := func(code, severity string) *databaseNotice {
		return &databaseNotice{Code: code, Severity: severity, ReclaimableBytes: reclaimable}
	}

	// The request was made earlier and the environment now disables it. The
	// flag stays set and is honoured again when the variable is removed.
	if state.FlagRequested && h.envMode == config.DBCompactOff {
		return n(noticeDisabledByEnv, severityInfo)
	}

	decision := dbmaint.Decide(in)
	switch decision.Reason {
	case dbmaint.ReasonInsufficientDisk:
		notice := n(noticeInsufficientDisk, severityWarning)
		notice.RequiredBytes, notice.AvailableBytes = &decision.RequiredBytes, &decision.AvailableBytes
		return notice
	case dbmaint.ReasonTooManyFailures:
		return n(noticeTooManyFailures, severityWarning)
	}

	if decision.Run && state.LastResult != nil &&
		state.LastResult.Outcome == dbmaint.ResultSkipped && state.LastResult.Reason == dbmaint.ReasonDatabaseBusy {
		return n(noticeDatabaseBusy, severityInfo)
	}

	// The automatic condition, judged without the user's request or the failure
	// counter (the same dry run Advise uses). It promises a conversion, so it
	// waits for the conversion to exist and stays silent when the next boot
	// would refuse (terminal skip of the last result).
	in.FlagRequested, in.Attempts = false, 0
	if h.conversionEnabled() && dbmaint.Decide(in).Run && !dbmaint.SuppressesPending(state.LastResult) {
		return n(noticeRestartToOptimize, severityInfo)
	}
	return nil
}

// RequestOptimize sets the "reclaim space on next restart" flag. A repeated
// request while the flag is set is a 200 before any other check. A request that
// could never do anything is a 409 whose body names the actual cause.
// POST /api/v1/system/database/optimize-on-restart
func (h *DatabaseMaintenanceHandler) RequestOptimize(c *gin.Context) {
	ctx := c.Request.Context()

	already, err := h.store.FlagRequested(ctx)
	if err != nil {
		h.fail(c, "could not read the optimize request", err)
		return
	}
	if already {
		c.JSON(http.StatusOK, gin.H{"requested": true})
		return
	}

	if h.envMode == config.DBCompactOff {
		c.JSON(http.StatusConflict, gin.H{
			"error": fmt.Sprintf("database optimization is disabled by CHARON_DB_COMPACT_ON_START=%s", config.DBCompactOff),
			"code":  codeDisabledByEnv,
		})
		return
	}
	stats, err := h.inspect(ctx)
	if err != nil {
		h.fail(c, "could not inspect the database", err)
		return
	}
	if !h.canRequestOptimize(stats) {
		c.JSON(http.StatusConflict, gin.H{
			"error": "the database is already optimized or has too little reclaimable space",
			"code":  codeNothingToOptimize,
		})
		return
	}

	// A fresh request starts from a clean slate: it is how an admin recovers
	// from "stopped after 3 attempts".
	if err := h.store.ResetAttempts(ctx); err != nil {
		h.fail(c, "could not reset the failure counter", err)
		return
	}
	if err := h.store.SetFlag(ctx); err != nil {
		h.fail(c, "could not save the optimize request", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"requested": true})
}

// CancelOptimize clears the flag. It is idempotent.
// DELETE /api/v1/system/database/optimize-on-restart
func (h *DatabaseMaintenanceHandler) CancelOptimize(c *gin.Context) {
	if err := h.store.ClearFlag(c.Request.Context()); err != nil {
		h.fail(c, "could not clear the optimize request", err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"requested": false})
}
