package handlers

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Wikid82/charon/backend/internal/crowdsec"
	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/util"

	"github.com/gin-gonic/gin"
)

func ttlRemainingSeconds(now, retrievedAt time.Time, ttl time.Duration) *int64 {
	if retrievedAt.IsZero() || ttl <= 0 {
		return nil
	}
	remaining := retrievedAt.Add(ttl).Sub(now)
	if remaining < 0 {
		var zero int64
		return &zero
	}
	secs := int64(remaining.Seconds())
	return &secs
}

// ListPresets returns the curated preset catalog when Cerberus is enabled.
func (h *CrowdsecHandler) ListPresets(c *gin.Context) {
	if !h.isCerberusEnabled() {
		c.JSON(http.StatusNotFound, gin.H{"error": "cerberus disabled"})
		return
	}

	type presetInfo struct {
		crowdsec.Preset
		Available           bool       `json:"available"`
		Cached              bool       `json:"cached"`
		CacheKey            string     `json:"cache_key,omitempty"`
		Etag                string     `json:"etag,omitempty"`
		RetrievedAt         *time.Time `json:"retrieved_at,omitempty"`
		TTLRemainingSeconds *int64     `json:"ttl_remaining_seconds,omitempty"`
	}

	result := map[string]*presetInfo{}
	for _, p := range crowdsec.ListCuratedPresets() {
		cp := p
		result[p.Slug] = &presetInfo{Preset: cp, Available: true}
	}

	// Merge hub index when available
	if h.Hub != nil {
		ctx := c.Request.Context()
		if idx, err := h.Hub.FetchIndex(ctx); err == nil {
			for _, item := range idx.Items {
				slug := strings.TrimSpace(item.Name)
				if slug == "" {
					continue
				}
				if _, ok := result[slug]; !ok {
					result[slug] = &presetInfo{Preset: crowdsec.Preset{
						Slug:        slug,
						Title:       item.Title,
						Summary:     item.Description,
						Source:      "hub",
						Tags:        []string{item.Type},
						RequiresHub: true,
					}, Available: true}
				} else {
					result[slug].Available = true
				}
			}
		} else {
			logger.Log().WithError(err).Warn("crowdsec hub index unavailable")
		}
	}

	// Merge cache metadata
	if h.Hub != nil && h.Hub.Cache != nil {
		ctx := c.Request.Context()
		if cached, err := h.Hub.Cache.List(ctx); err == nil {
			cacheTTL := h.Hub.Cache.TTL()
			now := time.Now().UTC()
			for _, entry := range cached {
				if _, ok := result[entry.Slug]; !ok {
					result[entry.Slug] = &presetInfo{Preset: crowdsec.Preset{Slug: entry.Slug, Title: entry.Slug, Summary: "cached preset", Source: "hub", RequiresHub: true}}
				}
				result[entry.Slug].Cached = true
				result[entry.Slug].CacheKey = entry.CacheKey
				result[entry.Slug].Etag = entry.Etag
				if !entry.RetrievedAt.IsZero() {
					val := entry.RetrievedAt
					result[entry.Slug].RetrievedAt = &val
				}
				result[entry.Slug].TTLRemainingSeconds = ttlRemainingSeconds(now, entry.RetrievedAt, cacheTTL)
			}
		} else {
			logger.Log().WithError(err).Warn("crowdsec hub cache list failed")
		}
	}

	list := make([]presetInfo, 0, len(result))
	for _, v := range result {
		list = append(list, *v)
	}

	c.JSON(http.StatusOK, gin.H{"presets": list})
}

// PullPreset downloads and caches a hub preset while returning a preview.
func (h *CrowdsecHandler) PullPreset(c *gin.Context) {
	if !h.isCerberusEnabled() {
		c.JSON(http.StatusNotFound, gin.H{"error": "cerberus disabled"})
		return
	}

	var payload struct {
		Slug string `json:"slug"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}
	slug := strings.TrimSpace(payload.Slug)
	if slug == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "slug required"})
		return
	}
	if h.Hub == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "hub service unavailable"})
		return
	}

	// Check for curated preset that doesn't require hub
	if preset, ok := crowdsec.FindPreset(slug); ok && !preset.RequiresHub {
		c.JSON(http.StatusOK, gin.H{
			"status":       "pulled",
			"slug":         preset.Slug,
			"preview":      curatedPresetPreview(preset),
			"cache_key":    "curated-" + preset.Slug,
			"etag":         "curated",
			"retrieved_at": time.Now(),
			"source":       "charon-curated",
		})
		return
	}

	ctx := c.Request.Context()
	// Log cache directory before pull
	if h.Hub != nil && h.Hub.Cache != nil {
		cacheDir := filepath.Join(h.DataDir, "hub_cache")
		logger.Log().WithField("cache_dir", util.SanitizeForLog(cacheDir)).WithField("slug", util.SanitizeForLog(slug)).Info("attempting to pull preset")
		if stat, err := os.Stat(cacheDir); err == nil {
			logger.Log().WithField("cache_dir_mode", stat.Mode()).WithField("cache_dir_writable", stat.Mode().Perm()&0o200 != 0).Debug("cache directory exists")
		} else {
			logger.Log().WithError(err).Warn("cache directory stat failed")
		}
	}

	res, err := h.Hub.Pull(ctx, slug)
	if err != nil {
		if errors.Is(err, crowdsec.ErrInvalidSlug) {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid slug"})
			return
		}
		status := mapCrowdsecStatus(err, http.StatusBadGateway)
		// Safe: User input sanitized via util.SanitizeForLog() which removes
		// control characters (0x00-0x1F, 0x7F) including CRLF
		// codeql[go/log-injection]
		logger.Log().WithField("error", util.SanitizeForLog(err.Error())).WithField("slug", util.SanitizeForLog(slug)).WithField("hub_base_url", util.SanitizeForLog(h.Hub.HubBaseURL)).Warn("crowdsec preset pull failed")
		c.JSON(status, gin.H{"error": crowdsec.RedactPaths(err.Error()), "hub_endpoints": h.hubEndpoints()})
		return
	}

	logger.Log().Info("preset pulled and cached successfully")

	c.JSON(http.StatusOK, gin.H{
		"status":       "pulled",
		"slug":         res.Meta.Slug,
		"preview":      res.Preview,
		"cache_key":    res.Meta.CacheKey,
		"etag":         res.Meta.Etag,
		"retrieved_at": res.Meta.RetrievedAt,
		"source":       res.Meta.Source,
	})
}

// Error codes returned next to the English error text of a failed preset apply.
const (
	codePresetCacheKeyRequired = "preset_cache_key_required"
	codePresetCacheStale       = "preset_cache_stale"
	codePresetHubUpdating      = "preset_hub_updating"
	codePresetLocalModified    = "preset_local_modified"
	codePresetApplyTimeout     = "preset_apply_timeout"
	codeCSCLIUnavailable       = "cscli_unavailable"
)

// respondCoded writes an error response that carries a stable error_code next to the English text.
// extra fields are merged into the body.
func respondCoded(c *gin.Context, status int, code, msg string, extra gin.H) {
	body := gin.H{"error": msg, "error_code": code}
	for k, v := range extra {
		body[k] = v
	}
	c.JSON(status, body)
}

// ApplyPreset installs a pulled preset: a curated preset through cscli, a hub preset by installing the
// pulled item with cscli and verifying the result.
func (h *CrowdsecHandler) ApplyPreset(c *gin.Context) {
	if !h.isCerberusEnabled() {
		c.JSON(http.StatusNotFound, gin.H{"error": "cerberus disabled"})
		return
	}

	var payload struct {
		Slug     string `json:"slug"`
		CacheKey string `json:"cache_key"`
	}
	if err := c.ShouldBindJSON(&payload); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid payload"})
		return
	}

	slug := strings.TrimSpace(payload.Slug)
	cacheKey := strings.TrimSpace(payload.CacheKey)
	if slug == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "slug required"})
		return
	}
	if h.Hub == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "hub service unavailable"})
		return
	}

	curated, isCurated := crowdsec.FindPreset(slug)
	isCurated = isCurated && !curated.RequiresHub
	switch {
	case isCurated && cacheKey != "" && cacheKey != "curated-"+curated.Slug:
		respondCoded(c, http.StatusConflict, codePresetCacheStale, msgPresetStale, nil)
		return
	case !isCurated && cacheKey == "":
		respondCoded(c, http.StatusBadRequest, codePresetCacheKeyRequired, "Pull the preset first, then apply it.", nil)
		return
	case !isCurated && !crowdsec.ValidCacheSlug(slug):
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid slug"})
		return
	}

	// One mutation of DataDir at a time across preset apply, config import and file writes.
	if !h.lockData(c, "preset apply") {
		return
	}
	defer h.dataMu.Unlock()

	if isCurated {
		h.applyCuratedPreset(c, curated)
		return
	}

	res, err := h.Hub.Apply(c.Request.Context(), slug, cacheKey)
	if err != nil {
		// Safe: slug is validated against the hub item name pattern; backup_path and cache_key are system-generated values
		// codeql[go/log-injection]
		logger.Log().WithField("error", util.SanitizeForLog(err.Error())).WithField("slug", util.SanitizeForLog(slug)).WithField("backup_path", util.SanitizeForLog(res.BackupPath)).WithField("cache_key", util.SanitizeForLog(res.CacheKey)).Warn("crowdsec preset apply failed")
		h.recordPresetEvent(slug, res, err)
		h.respondApplyError(c, err, res)
		return
	}

	h.recordPresetEvent(slug, res, nil)
	respondApplySuccess(c, res)
}

const msgPresetStale = "The cached preset is outdated or missing. Pull the preset again, then apply it."

// respondApplyError maps a failed hub preset apply to its status and error code.
func (h *CrowdsecHandler) respondApplyError(c *gin.Context, err error, res crowdsec.ApplyResult) {
	extra := applyFailureBody("", res)
	delete(extra, "error")
	switch {
	case errors.Is(err, crowdsec.ErrPresetCacheStale):
		respondCoded(c, http.StatusConflict, codePresetCacheStale, msgPresetStale, extra)
	case errors.Is(err, crowdsec.ErrPresetHubUpdating):
		respondCoded(c, http.StatusConflict, codePresetHubUpdating, "The hub is still updating. Try again later.", extra)
	case errors.Is(err, crowdsec.ErrPresetLocalModified):
		var modified *crowdsec.LocalModifiedError
		if errors.As(err, &modified) {
			extra["tainted_items"] = modified.Items
		}
		respondCoded(c, http.StatusConflict, codePresetLocalModified, "This preset was edited on this server, so it was not replaced. Reset it before applying.", extra)
	case errors.Is(err, crowdsec.ErrPresetApplyTimeout):
		respondCoded(c, http.StatusGatewayTimeout, codePresetApplyTimeout, "The preset install took too long and was rolled back. Try again.", extra)
	case errors.Is(err, crowdsec.ErrCSCLIUnavailable):
		respondCoded(c, http.StatusServiceUnavailable, codeCSCLIUnavailable, "cscli unavailable or not responding", extra)
	default:
		body := applyFailureBody(crowdsec.RedactPaths(err.Error()), res)
		body["hub_endpoints"] = h.hubEndpoints()
		c.JSON(mapCrowdsecStatus(err, http.StatusInternalServerError), body)
	}
}

// applyCuratedPreset installs a Charon-defined preset through cscli and reports the true outcome.
// A failure is recorded as a "failed" audit event and never answered with a 2xx status.
func (h *CrowdsecHandler) applyCuratedPreset(c *gin.Context, preset crowdsec.Preset) {
	res, err := h.Hub.ApplyCurated(c.Request.Context(), preset)
	if err != nil {
		logger.Log().WithField("error", util.SanitizeForLog(err.Error())).WithField("slug", util.SanitizeForLog(preset.Slug)).WithField("backup_path", util.SanitizeForLog(res.BackupPath)).Warn("curated crowdsec preset apply failed")
		h.recordPresetEvent(preset.Slug, res, err)

		status := mapCrowdsecStatus(err, http.StatusInternalServerError)
		msg := crowdsec.RedactPaths(err.Error())
		if errors.Is(err, crowdsec.ErrCSCLIUnavailable) {
			status = http.StatusServiceUnavailable
			msg = "CrowdSec CLI is not available; curated presets require cscli"
		}
		c.JSON(status, applyFailureBody(msg, res))
		return
	}
	h.recordPresetEvent(preset.Slug, res, nil)
	respondApplySuccess(c, res)
}

// curatedPresetPreview renders the hub items a curated preset will install.
func curatedPresetPreview(preset crowdsec.Preset) string {
	var b strings.Builder
	b.WriteString("# Curated preset: " + preset.Title + "\n# " + preset.Summary + "\n#\n# Installs these CrowdSec hub items:\n")
	for _, item := range preset.Items {
		b.WriteString("#   " + item.Type + ": " + item.Name + "\n")
	}
	return b.String()
}

// recordPresetEvent persists an apply audit row. A nil err records the result status
// (defaulting to "applied"); a non-nil err records a "failed" row with the error text.
// Persistence errors are logged and never alter the request outcome.
func (h *CrowdsecHandler) recordPresetEvent(slug string, res crowdsec.ApplyResult, applyErr error) {
	if h.DB == nil {
		return
	}
	event := models.CrowdsecPresetEvent{Slug: slug, Action: "apply", CacheKey: res.CacheKey, BackupPath: res.BackupPath}
	if applyErr != nil {
		event.Status = "failed"
		event.Error = applyErr.Error()
	} else {
		event.Status = res.Status
		if event.Status == "" {
			event.Status = "applied"
		}
		if res.AppliedPreset != "" {
			event.Slug = res.AppliedPreset
		}
	}
	if err := h.DB.Create(&event).Error; err != nil {
		logger.Log().WithError(err).WithField("slug", util.SanitizeForLog(event.Slug)).Warn("failed to record crowdsec preset event")
	}
}

// applyFailureBody builds the common error payload for a failed preset apply.
func applyFailureBody(msg string, res crowdsec.ApplyResult) gin.H {
	body := gin.H{"error": msg}
	if res.BackupPath != "" {
		body["backup"] = crowdsec.BackupID(res.BackupPath)
	}
	if res.CacheKey != "" {
		body["cache_key"] = res.CacheKey
	}
	return body
}

// respondApplySuccess writes the 200 payload for a completed preset apply.
func respondApplySuccess(c *gin.Context, res crowdsec.ApplyResult) {
	body := gin.H{
		"status":      res.Status,
		"backup":      crowdsec.BackupID(res.BackupPath),
		"reload_hint": res.ReloadHint,
		"used_cscli":  res.UsedCSCLI,
		"cache_key":   res.CacheKey,
		"slug":        res.AppliedPreset,
	}
	if len(res.Warnings) > 0 {
		body["warnings"] = res.Warnings
	}
	c.JSON(http.StatusOK, body)
}

// GetCachedPreset returns cached preview for a slug when available.
func (h *CrowdsecHandler) GetCachedPreset(c *gin.Context) {
	if !h.isCerberusEnabled() {
		c.JSON(http.StatusNotFound, gin.H{"error": "cerberus disabled"})
		return
	}
	if h.Hub == nil || h.Hub.Cache == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "hub cache unavailable"})
		return
	}
	ctx := c.Request.Context()
	// The route is a catch-all so owner/name slugs match; the param keeps its leading slash.
	slug := strings.TrimSpace(strings.TrimPrefix(c.Param("slug"), "/"))
	if slug == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "slug required"})
		return
	}
	meta, item, err := h.Hub.Cache.LoadItem(ctx, slug)
	if err != nil {
		if errors.Is(err, crowdsec.ErrInvalidSlug) || errors.Is(err, crowdsec.ErrCacheMiss) || errors.Is(err, crowdsec.ErrCacheExpired) || errors.Is(err, crowdsec.ErrCacheInvalid) {
			c.JSON(http.StatusNotFound, gin.H{"error": "cache miss"})
			return
		}
		logger.Log().WithField("error", util.SanitizeForLog(err.Error())).WithField("slug", util.SanitizeForLog(slug)).Warn("failed to load crowdsec preset preview")
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to load preset preview"})
		return
	}
	cacheTTL := h.Hub.Cache.TTL()
	now := time.Now().UTC()
	c.JSON(http.StatusOK, gin.H{
		"preview":               string(item),
		"cache_key":             meta.CacheKey,
		"etag":                  meta.Etag,
		"retrieved_at":          meta.RetrievedAt,
		"ttl_remaining_seconds": ttlRemainingSeconds(now, meta.RetrievedAt, cacheTTL),
	})
}
