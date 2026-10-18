package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"github.com/Wikid82/charon/backend/internal/crowdsec"
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/util"
)

// Limits and codes of the background install job endpoints (#1615).
const (
	applyJobListDefault = 20
	applyJobListMax     = 50
	// applyJobCurrentWindow is how long a finished job is still reported by presets/jobs/current, so the UI can
	// show the outcome once after a page reload.
	applyJobCurrentWindow = 30 * time.Minute

	codeCrowdsecUnavailable = "crowdsec_unavailable"
	codeJobNotActive        = "job_not_active"
	codeJobNotFound         = "job_not_found"
	codeInvalidJobID        = "invalid_job_id"
	codeInvalidLimit        = "invalid_limit"
)

// applyJobView is the API shape of a CrowdsecApplyJob. It is a separate type on purpose: the model carries the
// snapshot path (backup_path), which is a server path and never leaves the process, and stores warnings as a
// JSON string.
type applyJobView struct {
	JobID           string     `json:"job_id"`
	Slug            string     `json:"slug"`
	ItemType        string     `json:"item_type"`
	CacheKey        string     `json:"cache_key"`
	State           string     `json:"state"`
	Phase           string     `json:"phase"`
	ErrorCode       string     `json:"error_code,omitempty"`
	Error           string     `json:"error,omitempty"`
	Warnings        []string   `json:"warnings"`
	ItemsInstalled  int        `json:"items_installed"`
	ItemsTotal      *int       `json:"items_total"`
	CancelRequested bool       `json:"cancel_requested"`
	ElapsedSeconds  int64      `json:"elapsed_seconds"`
	CreatedAt       time.Time  `json:"created_at"`
	StartedAt       *time.Time `json:"started_at"`
	FinishedAt      *time.Time `json:"finished_at"`
}

func newApplyJobView(job *models.CrowdsecApplyJob, now time.Time) applyJobView {
	warnings := []string{}
	if job.Warnings != "" {
		if err := json.Unmarshal([]byte(job.Warnings), &warnings); err != nil || warnings == nil {
			warnings = []string{}
		}
	}
	begin := job.CreatedAt
	if job.StartedAt != nil {
		begin = *job.StartedAt
	}
	end := now
	if job.FinishedAt != nil {
		end = *job.FinishedAt
	}
	return applyJobView{
		JobID: job.ID, Slug: job.Slug, ItemType: job.ItemType, CacheKey: job.CacheKey,
		State: job.State, Phase: job.Phase, ErrorCode: job.ErrorCode, Error: job.Error,
		Warnings: warnings, ItemsInstalled: job.ItemsInstalled, ItemsTotal: job.ItemsTotal,
		CancelRequested: job.CancelRequested, ElapsedSeconds: max(0, int64(end.Sub(begin).Seconds())),
		CreatedAt: job.CreatedAt, StartedAt: job.StartedAt, FinishedAt: job.FinishedAt,
	}
}

// jobManager returns the install job manager wired to the handler's database and hub service, or answers 503
// crowdsec_unavailable and returns false when either is missing. The wiring is lazy because handlers are built
// before the hub service is attached; it is redone if the hub service is replaced.
func (h *CrowdsecHandler) jobManager(c *gin.Context) (*crowdsec.ApplyJobManager, bool) {
	if h.DB == nil || h.Hub == nil {
		respondCoded(c, http.StatusServiceUnavailable, codeCrowdsecUnavailable, "background installs are unavailable", nil)
		return nil, false
	}
	m := h.ApplyJobs()
	h.applyJobsCfgMu.Lock()
	if h.applyJobsHub != h.Hub {
		m.Configure(crowdsec.HubApplyConfig(h.Hub, crowdsec.NewApplyJobRepository(h.DB)))
		h.applyJobsHub = h.Hub
	}
	h.applyJobsCfgMu.Unlock()
	return m, true
}

// ShutdownApplyJobs cancels the running install job, which rolls back from its snapshot, and waits for it to
// end. Call it from the graceful shutdown path; startup recovery is the backstop when the grace period ends first.
func (h *CrowdsecHandler) ShutdownApplyJobs() {
	h.ApplyJobs().Shutdown()
}

// startApplyJob starts a background install of a hub preset and answers 202 with the queued job. The caller
// holds dataMu, so no data-changing handler is between its busy check and its work while the flag is taken.
func (h *CrowdsecHandler) startApplyJob(c *gin.Context, slug, itemType, cacheKey string) {
	m, ok := h.jobManager(c)
	if !ok {
		return
	}
	job, err := m.Start(crowdsec.ApplyJobRequest{Slug: slug, Type: itemType, CacheKey: cacheKey})
	switch {
	case errors.Is(err, crowdsec.ErrApplyJobBusy):
		// The id is empty when the hub item restore, not a job, holds the other side of the exclusion.
		extra := gin.H{}
		if id, busy := m.Busy(); busy && id != "" {
			extra["job_id"] = id
		}
		respondCoded(c, http.StatusConflict, codeCrowdsecBusy, msgCrowdsecBusy, extra)
		return
	case err != nil:
		logger.Log().WithField("error", util.SanitizeForLog(err.Error())).Error("starting crowdsec preset install job failed")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to start the preset install"})
		return
	}
	c.JSON(http.StatusAccepted, newApplyJobView(job, time.Now().UTC()))
}

// jobFromParam validates the :id parameter as a UUID and loads the job; it answers 400 or 404 itself.
func (h *CrowdsecHandler) jobFromParam(c *gin.Context, m *crowdsec.ApplyJobManager) (*models.CrowdsecApplyJob, bool) {
	id := c.Param("id")
	if _, err := uuid.Parse(id); err != nil {
		respondCoded(c, http.StatusBadRequest, codeInvalidJobID, "invalid job id", nil)
		return nil, false
	}
	job, err := m.Get(c.Request.Context(), id)
	if err != nil {
		h.respondJobLookupError(c, err)
		return nil, false
	}
	return job, true
}

func (h *CrowdsecHandler) respondJobLookupError(c *gin.Context, err error) {
	if errors.Is(err, crowdsec.ErrApplyJobNotFound) {
		respondCoded(c, http.StatusNotFound, codeJobNotFound, "install job not found", nil)
		return
	}
	logger.Log().WithField("error", util.SanitizeForLog(err.Error())).Error("reading crowdsec preset install job failed")
	c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to read the install job"})
}

// jobReadManager is the common start of the read endpoints: cerberus gate plus the manager.
func (h *CrowdsecHandler) jobReadManager(c *gin.Context) (*crowdsec.ApplyJobManager, bool) {
	if !h.isCerberusEnabled() {
		c.JSON(http.StatusNotFound, gin.H{"error": "cerberus disabled"})
		return nil, false
	}
	return h.jobManager(c)
}

// GetCurrentApplyJob answers the active install job, or the newest finished one from the last 30 minutes, or
// {"job": null}. The UI calls it on page load to resume the progress row.
func (h *CrowdsecHandler) GetCurrentApplyJob(c *gin.Context) {
	m, ok := h.jobReadManager(c)
	if !ok {
		return
	}
	jobs, err := m.Recent(c.Request.Context(), 1)
	if err != nil {
		h.respondJobLookupError(c, err)
		return
	}
	now := time.Now().UTC()
	if len(jobs) == 1 {
		job := &jobs[0]
		if job.IsActive() || (job.FinishedAt != nil && now.Sub(*job.FinishedAt) <= applyJobCurrentWindow) {
			c.JSON(http.StatusOK, gin.H{"job": newApplyJobView(job, now)})
			return
		}
	}
	c.JSON(http.StatusOK, gin.H{"job": nil})
}

// GetApplyJob answers one install job by id.
func (h *CrowdsecHandler) GetApplyJob(c *gin.Context) {
	m, ok := h.jobReadManager(c)
	if !ok {
		return
	}
	job, ok := h.jobFromParam(c, m)
	if !ok {
		return
	}
	c.JSON(http.StatusOK, newApplyJobView(job, time.Now().UTC()))
}

// ListApplyJobs answers the most recent install jobs, newest first (default 20, at most 50).
func (h *CrowdsecHandler) ListApplyJobs(c *gin.Context) {
	m, ok := h.jobReadManager(c)
	if !ok {
		return
	}
	limit := applyJobListDefault
	if raw := c.Query("limit"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 {
			respondCoded(c, http.StatusBadRequest, codeInvalidLimit, "limit must be a positive number", nil)
			return
		}
		limit = min(n, applyJobListMax)
	}
	jobs, err := m.Recent(c.Request.Context(), limit)
	if err != nil {
		h.respondJobLookupError(c, err)
		return
	}
	now := time.Now().UTC()
	views := make([]applyJobView, len(jobs))
	for i := range jobs {
		views[i] = newApplyJobView(&jobs[i], now)
	}
	c.JSON(http.StatusOK, gin.H{"jobs": views})
}

// CancelApplyJob asks an active install job to stop; the job rolls back and ends cancelled. A finished job
// answers 409 job_not_active.
func (h *CrowdsecHandler) CancelApplyJob(c *gin.Context) {
	m, ok := h.jobReadManager(c)
	if !ok {
		return
	}
	job, ok := h.jobFromParam(c, m)
	if !ok {
		return
	}
	if !job.IsActive() {
		respondCoded(c, http.StatusConflict, codeJobNotActive, "the install job has already finished", gin.H{"state": job.State})
		return
	}
	if err := m.Cancel(job.ID); err != nil {
		h.respondJobLookupError(c, err)
		return
	}
	if fresh, err := m.Get(c.Request.Context(), job.ID); err == nil {
		job = fresh
	}
	c.JSON(http.StatusAccepted, newApplyJobView(job, time.Now().UTC()))
}
