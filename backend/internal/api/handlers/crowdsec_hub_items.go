package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/Wikid82/charon/backend/internal/crowdsec"
	"github.com/Wikid82/charon/backend/internal/logger"
)

// addHubItemsReport adds the hub_items field to an import response when the archive carried a usable
// manifest. It diffs against the items installed right now, so the caller must hold dataMu. If the
// installed items cannot be read the field is left out rather than reporting everything as missing.
func (h *CrowdsecHandler) addHubItemsReport(resp map[string]any, manifest *crowdsec.ArchiveHubItems) {
	if manifest == nil {
		return
	}
	installed, err := crowdsec.InstalledHubItems(h.DataDir)
	if err != nil {
		logger.Log().WithError(err).Warn("crowdsec import: installed hub items could not be read")
		return
	}
	resp["hub_items"] = manifest.Report(installed)
}

// Limits of one restore action; see hubRestoreState.limits for the test override.
const (
	hubRestoreMaxBodyBytes  = 1 << 20
	hubRestoreOverall       = 5 * time.Minute
	hubRestoreItemTimeout   = 120 * time.Second
	hubRestoreUpdateTimeout = 60 * time.Second
	hubRestoreReloadTimeout = 10 * time.Second
	hubRestoreLogLimit      = 300
)

// Fixed per-item reasons of a restore. They are the only failure detail a client ever receives.
const (
	hubReasonNotFound     = "not_found"
	hubReasonInstallFail  = "install_failed"
	hubReasonVerifyFail   = "verify_failed"
	hubReasonNotAttempted = "not_attempted"
)

// Fixed messages of the restore endpoint.
const (
	msgRestoreInvalidItem = "invalid hub item"
	msgRestoreNoItems     = "no hub items given"
	msgRestoreBadRequest  = "invalid request"
	msgRestoreTooLarge    = "request too large"
	msgRestoreRunning     = "a restore is already running"
	msgRestoreHubDown     = "the CrowdSec hub could not be reached; nothing was installed"
	msgRestoreReadFailed  = "the installed hub items could not be read; nothing was installed"
)

// hubRestoreLimits are the time budgets of one restore. The zero value means the production budgets.
type hubRestoreLimits struct {
	overall time.Duration
	item    time.Duration
	update  time.Duration
}

func (l hubRestoreLimits) withDefaults() hubRestoreLimits {
	if l.overall <= 0 {
		l.overall = hubRestoreOverall
	}
	if l.item <= 0 {
		l.item = hubRestoreItemTimeout
	}
	if l.update <= 0 {
		l.update = hubRestoreUpdateTimeout
	}
	return l
}

// hubRestoreState is the handler-wide state of the restore action. running is compare-and-swapped and
// never waited on, so a second restore is refused instead of queued. Lock order is always
// running -> CrowdsecHandler.dataMu -> HubService.mu.
type hubRestoreState struct {
	running atomic.Bool
	limits  hubRestoreLimits
}

type hubRestoreRequest struct {
	Items []crowdsec.HubItem `json:"items"`
}

type hubRestoreFailure struct {
	Type   string `json:"type"`
	Name   string `json:"name"`
	Reason string `json:"reason"`
}

type hubRestoreResponse struct {
	Installed        []crowdsec.HubItem  `json:"installed"`
	AlreadyInstalled []crowdsec.HubItem  `json:"already_installed"`
	Failed           []hubRestoreFailure `json:"failed"`
	Remaining        int                 `json:"remaining"`
	SkippedInvalid   int                 `json:"skipped_invalid"`
	Reloaded         bool                `json:"reloaded"`
}

// RestoreHubItems installs the requested hub items that are not installed yet: POST
// /admin/crowdsec/hub-items/restore. Every hub write takes dataMu for exactly one cscli section, so an
// import, export, file save or preset apply waits for at most one bounded call and never overlaps an
// install.
func (h *CrowdsecHandler) RestoreHubItems(c *gin.Context) {
	items, status, msg := parseHubRestoreRequest(c)
	if status != 0 {
		c.JSON(status, gin.H{"error": msg})
		return
	}

	if !h.restore.running.CompareAndSwap(false, true) {
		c.JSON(http.StatusConflict, gin.H{"error": msgRestoreRunning})
		return
	}
	defer h.restore.running.Store(false)

	limits := h.restore.limits.withDefaults()
	ctx, cancel := context.WithTimeout(c.Request.Context(), limits.overall)
	defer cancel()

	if err := h.hubUpdate(ctx, limits.update); err != nil {
		logger.Log().WithField("error", hubLogDetail(err.Error())).Warn("crowdsec hub items restore: hub update failed")
		c.JSON(http.StatusBadGateway, gin.H{"error": msgRestoreHubDown})
		return
	}

	res := &hubRestoreResponse{
		Installed:        []crowdsec.HubItem{},
		AlreadyInstalled: []crowdsec.HubItem{},
		Failed:           []hubRestoreFailure{},
	}
	var collections, others []crowdsec.HubItem
	for _, it := range items {
		if it.Type == "collections" {
			collections = append(collections, it)
		} else {
			others = append(others, it)
		}
	}

	// Collections first: they bring most scenarios and parsers with them, so the rest is re-diffed
	// against a fresh locked read.
	if !h.restorePass(ctx, limits.item, collections, res) {
		c.JSON(http.StatusInternalServerError, gin.H{"error": msgRestoreReadFailed})
		return
	}
	if !h.restorePass(ctx, limits.item, others, res) {
		for _, it := range others {
			res.Failed = append(res.Failed, hubRestoreFailure{it.Type, it.Name, hubReasonNotAttempted})
		}
	}
	for _, f := range res.Failed {
		if f.Reason == hubReasonNotAttempted {
			res.Remaining++
		}
	}

	if len(res.Installed) > 0 {
		res.Reloaded = h.reloadAfterRestore(ctx)
	}
	c.JSON(http.StatusOK, res)
}

// parseHubRestoreRequest reads and validates the body. A non-zero status carries the error to send.
func parseHubRestoreRequest(c *gin.Context) (items []crowdsec.HubItem, status int, msg string) {
	body := http.MaxBytesReader(c.Writer, c.Request.Body, hubRestoreMaxBodyBytes)
	var req hubRestoreRequest
	if err := json.NewDecoder(body).Decode(&req); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return nil, http.StatusRequestEntityTooLarge, msgRestoreTooLarge
		}
		return nil, http.StatusBadRequest, msgRestoreBadRequest
	}
	if len(req.Items) == 0 {
		return nil, http.StatusBadRequest, msgRestoreNoItems
	}
	if len(req.Items) > crowdsec.MaxHubItems {
		return nil, http.StatusBadRequest, msgRestoreInvalidItem
	}
	seen := make(map[crowdsec.HubItem]struct{}, len(req.Items))
	for _, it := range req.Items {
		if !it.Valid() {
			return nil, http.StatusBadRequest, msgRestoreInvalidItem
		}
		if _, dup := seen[it]; dup {
			continue
		}
		seen[it] = struct{}{}
		items = append(items, it)
	}
	return items, 0, ""
}

// hubUpdate refreshes the hub index in its own locked section.
func (h *CrowdsecHandler) hubUpdate(ctx context.Context, limit time.Duration) error {
	h.dataMu.Lock()
	defer h.dataMu.Unlock()
	uctx, cancel := context.WithTimeout(ctx, limit)
	defer cancel()
	if out, err := h.CmdExec.Execute(uctx, "cscli", "hub", "update"); err != nil {
		return fmt.Errorf("hub update: %w (%s)", err, hubLogDetail(string(out)))
	}
	return nil
}

// installedHubSet reads the installed items under dataMu, so it never observes a tree an import is
// clearing.
func (h *CrowdsecHandler) installedHubSet() (map[crowdsec.HubItem]struct{}, error) {
	h.dataMu.Lock()
	defer h.dataMu.Unlock()
	installed, err := crowdsec.InstalledHubItems(h.DataDir)
	if err != nil {
		return nil, fmt.Errorf("read installed hub items: %w", err)
	}
	set := make(map[crowdsec.HubItem]struct{}, len(installed))
	for _, it := range installed {
		set[it] = struct{}{}
	}
	return set, nil
}

// restorePass installs the items of one pass that are not installed, recording every outcome in res.
// It returns false, having recorded nothing, when the installed items cannot be read.
func (h *CrowdsecHandler) restorePass(ctx context.Context, itemLimit time.Duration, items []crowdsec.HubItem, res *hubRestoreResponse) bool {
	if len(items) == 0 {
		return true
	}
	installed, err := h.installedHubSet()
	if err != nil {
		logger.Log().WithField("error", hubLogDetail(err.Error())).Warn("crowdsec hub items restore: cannot read installed items")
		return false
	}
	for _, it := range items {
		if _, ok := installed[it]; ok {
			res.AlreadyInstalled = append(res.AlreadyInstalled, it)
			continue
		}
		if reason := h.installHubItem(ctx, itemLimit, it); reason != "" {
			res.Failed = append(res.Failed, hubRestoreFailure{it.Type, it.Name, reason})
			continue
		}
		res.Installed = append(res.Installed, it)
	}
	return true
}

// installHubItem installs and verifies one item as a single critical section under dataMu, bounded by
// min(remaining overall deadline, itemLimit). It returns "" on success or a fixed reason.
func (h *CrowdsecHandler) installHubItem(ctx context.Context, itemLimit time.Duration, it crowdsec.HubItem) string {
	if ctx.Err() != nil {
		return hubReasonNotAttempted
	}
	h.dataMu.Lock()
	defer h.dataMu.Unlock()

	// The lock wait may have used up the budget or the client may have left meanwhile.
	limit := itemLimit
	if dl, ok := ctx.Deadline(); ok {
		limit = min(limit, time.Until(dl))
	}
	if ctx.Err() != nil || limit <= 0 {
		return hubReasonNotAttempted
	}
	ictx, cancel := context.WithTimeout(ctx, limit)
	defer cancel()

	// Literal binary, no shell, one item per call; type and name passed ValidHubItem checks and cannot
	// read as a flag.
	out, err := h.CmdExec.Execute(ictx, "cscli", it.Type, "install", it.Name)
	if err != nil {
		logger.Log().
			WithField("type", hubLogDetail(it.Type)).WithField("name", hubLogDetail(it.Name)).
			WithField("error", hubLogDetail(err.Error())).WithField("output", hubLogDetail(string(out))).
			Warn("crowdsec hub items restore: install failed")
		if hubItemNotFound(out) {
			return hubReasonNotFound
		}
		return hubReasonInstallFail
	}
	if err := crowdsec.VerifyHubItem(ictx, h.CmdExec, it.Type, it.Name); err != nil {
		logger.Log().
			WithField("type", hubLogDetail(it.Type)).WithField("name", hubLogDetail(it.Name)).
			WithField("error", hubLogDetail(err.Error())).
			Warn("crowdsec hub items restore: verification failed")
		return hubReasonVerifyFail
	}
	return ""
}

// hubItemNotFound reports whether cscli's output says the hub does not know the item. This is a
// best-effort match on cscli's wording; anything else is install_failed.
func hubItemNotFound(out []byte) bool {
	s := strings.ToLower(string(out))
	return strings.Contains(s, "can't find") || strings.Contains(s, "not found")
}

// reloadAfterRestore asks CrowdSec to reload on a detached, bounded context so an install that succeeded
// just before a client disconnect or the deadline is still picked up. Best-effort: it reports whether a
// reload was performed.
func (h *CrowdsecHandler) reloadAfterRestore(ctx context.Context) bool {
	rctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), hubRestoreReloadTimeout)
	defer cancel()
	switch err := h.reloadCrowdSec(rctx); {
	case err == nil:
		return true
	case errors.Is(err, crowdsec.ErrCrowdSecNotRunning):
	default:
		logger.Log().WithField("error", hubLogDetail(err.Error())).Warn("crowdsec hub items restore: reload failed (non-fatal)")
	}
	return false
}

// hubLogDetail sanitizes and truncates cscli detail for the log; it is never sent to a client.
func hubLogDetail(s string) string {
	s = sanitizeForLog(s)
	if len(s) > hubRestoreLogLimit {
		s = s[:hubRestoreLogLimit]
	}
	return s
}
