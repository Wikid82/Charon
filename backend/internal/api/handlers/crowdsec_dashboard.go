package handlers

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/models"
	"github.com/Wikid82/charon/backend/internal/network"
	"github.com/gin-gonic/gin"
)

// Cache TTL constants for dashboard endpoints.
const (
	dashSummaryTTL   = 30 * time.Second
	dashTimelineTTL  = 60 * time.Second
	dashTopIPsTTL    = 60 * time.Second
	dashScenariosTTL = 60 * time.Second
	dashAlertsTTL    = 30 * time.Second
	exportMaxRows    = 100_000
)

// parseTimeRange converts a range string to a start time. Empty string defaults to 24h.
func parseTimeRange(rangeStr string) (time.Time, error) {
	now := time.Now().UTC()
	switch rangeStr {
	case "1h":
		return now.Add(-1 * time.Hour), nil
	case "6h":
		return now.Add(-6 * time.Hour), nil
	case "24h", "":
		return now.Add(-24 * time.Hour), nil
	case "7d":
		return now.Add(-7 * 24 * time.Hour), nil
	case "30d":
		return now.Add(-30 * 24 * time.Hour), nil
	default:
		return time.Time{}, fmt.Errorf("invalid range: %s (valid: 1h, 6h, 24h, 7d, 30d)", rangeStr)
	}
}

// normalizeRange returns the canonical range string (defaults empty to "24h").
func normalizeRange(r string) string {
	if r == "" {
		return "24h"
	}
	return r
}

// intervalForRange selects the default time-bucket interval for a given range.
func intervalForRange(rangeStr string) string {
	switch rangeStr {
	case "1h":
		return "5m"
	case "6h":
		return "15m"
	case "24h", "":
		return "1h"
	case "7d":
		return "6h"
	case "30d":
		return "1d"
	default:
		return "1h"
	}
}

// intervalToStrftime maps an interval string to the SQLite strftime expression
// used for time bucketing.
func intervalToStrftime(interval string) string {
	switch interval {
	case "5m":
		return "strftime('%Y-%m-%dT%H:', created_at) || printf('%02d:00Z', (CAST(strftime('%M', created_at) AS INTEGER) / 5) * 5)"
	case "15m":
		return "strftime('%Y-%m-%dT%H:', created_at) || printf('%02d:00Z', (CAST(strftime('%M', created_at) AS INTEGER) / 15) * 15)"
	case "1h":
		return "strftime('%Y-%m-%dT%H:00:00Z', created_at)"
	case "6h":
		return "strftime('%Y-%m-%dT', created_at) || printf('%02d:00:00Z', (CAST(strftime('%H', created_at) AS INTEGER) / 6) * 6)"
	case "1d":
		return "strftime('%Y-%m-%dT00:00:00Z', created_at)"
	default:
		return "strftime('%Y-%m-%dT%H:00:00Z', created_at)"
	}
}

// validInterval checks whether the provided interval is one of the known values.
func validInterval(interval string) bool {
	switch interval {
	case "5m", "15m", "1h", "6h", "1d":
		return true
	default:
		return false
	}
}

// sanitizeCSVField prefixes fields starting with formula-trigger characters
// to prevent CSV injection (CWE-1236).
func sanitizeCSVField(field string) string {
	if field == "" {
		return field
	}
	switch field[0] {
	case '=', '+', '-', '@', '\t', '\r':
		return "'" + field
	}
	return field
}

// DashboardSummary returns aggregate counts for the dashboard summary cards.
func (h *CrowdsecHandler) DashboardSummary(c *gin.Context) {
	rangeStr := normalizeRange(c.Query("range"))
	cacheKey := "dashboard:summary:" + rangeStr

	if cached, ok := h.dashCache.Get(cacheKey); ok {
		c.JSON(http.StatusOK, cached)
		return
	}

	since, err := parseTimeRange(rangeStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Historical metrics from SQLite
	var totalDecisions int64
	h.DB.Model(&models.SecurityDecision{}).
		Where("source = ? AND created_at >= ?", "crowdsec", since).
		Count(&totalDecisions)

	var uniqueIPs int64
	h.DB.Model(&models.SecurityDecision{}).
		Where("source = ? AND created_at >= ?", "crowdsec", since).
		Distinct("ip").Count(&uniqueIPs)

	var topScenario struct {
		Scenario string
		Cnt      int64
	}
	h.DB.Model(&models.SecurityDecision{}).
		Select("scenario, COUNT(*) as cnt").
		Where("source = ? AND created_at >= ? AND scenario != ''", "crowdsec", since).
		Group("scenario").
		Order("cnt DESC").
		Limit(1).
		Scan(&topScenario)

	// Trend calculation: compare current period vs previous equal-length period
	duration := time.Since(since)
	previousSince := since.Add(-duration)
	var previousCount int64
	h.DB.Model(&models.SecurityDecision{}).
		Where("source = ? AND created_at >= ? AND created_at < ?", "crowdsec", previousSince, since).
		Count(&previousCount)

	// Trend: percentage change vs. the previous equal-length period.
	// Formula: round((current - previous) / previous * 100, 1)
	// Special cases: no previous data → 0; no current data → -100%.
	var trend float64
	if previousCount == 0 {
		trend = 0.0
	} else if totalDecisions == 0 && previousCount > 0 {
		trend = -100.0
	} else {
		trend = math.Round(float64(totalDecisions-previousCount)/float64(previousCount)*1000) / 10
	}

	// Active decisions from LAPI (real-time)
	activeDecisions := h.fetchActiveDecisionCount(c.Request.Context())

	result := gin.H{
		"total_decisions":  totalDecisions,
		"active_decisions": activeDecisions,
		"unique_ips":       uniqueIPs,
		"top_scenario":     topScenario.Scenario,
		"decisions_trend":  trend,
		"range":            rangeStr,
		"cached":           false,
		"generated_at":     time.Now().UTC().Format(time.RFC3339),
	}

	h.dashCache.Set(cacheKey, result, dashSummaryTTL)
	c.JSON(http.StatusOK, result)
}

// fetchActiveDecisionCount queries LAPI for active decisions count.
// Returns -1 when LAPI is unreachable.
func (h *CrowdsecHandler) fetchActiveDecisionCount(ctx context.Context) int64 {
	lapiURL := "http://127.0.0.1:8085"
	if h.Security != nil {
		cfg, err := h.Security.Get()
		if err == nil && cfg != nil && cfg.CrowdSecAPIURL != "" {
			lapiURL = cfg.CrowdSecAPIURL
		}
	}

	baseURL, err := h.resolveLAPIURLValidator(lapiURL)
	if err != nil {
		return -1
	}

	endpoint := baseURL.ResolveReference(&url.URL{Path: "/v1/decisions"})
	reqURL := endpoint.String()

	apiKey := getLAPIKey()

	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodGet, reqURL, http.NoBody)
	if err != nil {
		return -1
	}
	if apiKey != "" {
		req.Header.Set("X-Api-Key", apiKey)
	}
	req.Header.Set("Accept", "application/json")

	client := network.NewInternalServiceHTTPClient(10 * time.Second)
	resp, err := client.Do(req)
	if err != nil {
		return -1
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return -1
	}

	var decisions []interface{}
	if decErr := json.NewDecoder(resp.Body).Decode(&decisions); decErr != nil {
		return -1
	}
	return int64(len(decisions))
}

// DashboardTimeline returns time-bucketed decision counts for the timeline chart.
func (h *CrowdsecHandler) DashboardTimeline(c *gin.Context) {
	rangeStr := normalizeRange(c.Query("range"))
	interval := c.Query("interval")
	if interval == "" {
		interval = intervalForRange(rangeStr)
	}
	if !validInterval(interval) {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("invalid interval: %s (valid: 5m, 15m, 1h, 6h, 1d)", interval)})
		return
	}

	cacheKey := fmt.Sprintf("dashboard:timeline:%s:%s", rangeStr, interval)
	if cached, ok := h.dashCache.Get(cacheKey); ok {
		c.JSON(http.StatusOK, cached)
		return
	}

	since, err := parseTimeRange(rangeStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	bucketExpr := intervalToStrftime(interval)

	type bucketRow struct {
		Bucket   string
		Bans     int64
		Captchas int64
	}
	var rows []bucketRow

	h.DB.Model(&models.SecurityDecision{}).
		Select(fmt.Sprintf("(%s) as bucket, SUM(CASE WHEN action = 'block' THEN 1 ELSE 0 END) as bans, SUM(CASE WHEN action = 'challenge' THEN 1 ELSE 0 END) as captchas", bucketExpr)).
		Where("source = ? AND created_at >= ?", "crowdsec", since).
		Group("bucket").
		Order("bucket ASC").
		Scan(&rows)

	buckets := make([]gin.H, 0, len(rows))
	for _, r := range rows {
		buckets = append(buckets, gin.H{
			"timestamp": r.Bucket,
			"bans":      r.Bans,
			"captchas":  r.Captchas,
		})
	}

	result := gin.H{
		"buckets":  buckets,
		"range":    rangeStr,
		"interval": interval,
		"cached":   false,
	}

	h.dashCache.Set(cacheKey, result, dashTimelineTTL)
	c.JSON(http.StatusOK, result)
}

// DashboardTopIPs returns top attacking IPs ranked by decision count.
func (h *CrowdsecHandler) DashboardTopIPs(c *gin.Context) {
	rangeStr := normalizeRange(c.Query("range"))
	limitStr := c.DefaultQuery("limit", "10")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 {
		limit = 10
	}
	if limit > 50 {
		limit = 50
	}

	cacheKey := fmt.Sprintf("dashboard:top-ips:%s:%d", rangeStr, limit)
	if cached, ok := h.dashCache.Get(cacheKey); ok {
		c.JSON(http.StatusOK, cached)
		return
	}

	since, err := parseTimeRange(rangeStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	type ipRow struct {
		IP       string
		Count    int64
		LastSeen time.Time
		Country  string
	}
	var rows []ipRow

	h.DB.Model(&models.SecurityDecision{}).
		Select("ip, COUNT(*) as count, MAX(created_at) as last_seen, MAX(country) as country").
		Where("source = ? AND created_at >= ?", "crowdsec", since).
		Group("ip").
		Order("count DESC").
		Limit(limit).
		Scan(&rows)

	ips := make([]gin.H, 0, len(rows))
	for _, r := range rows {
		ips = append(ips, gin.H{
			"ip":        r.IP,
			"count":     r.Count,
			"last_seen": r.LastSeen.UTC().Format(time.RFC3339),
			"country":   r.Country,
		})
	}

	result := gin.H{
		"ips":    ips,
		"range":  rangeStr,
		"cached": false,
	}

	h.dashCache.Set(cacheKey, result, dashTopIPsTTL)
	c.JSON(http.StatusOK, result)
}

// DashboardScenarios returns scenario breakdown with counts and percentages.
func (h *CrowdsecHandler) DashboardScenarios(c *gin.Context) {
	rangeStr := normalizeRange(c.Query("range"))

	cacheKey := "dashboard:scenarios:" + rangeStr
	if cached, ok := h.dashCache.Get(cacheKey); ok {
		c.JSON(http.StatusOK, cached)
		return
	}

	since, err := parseTimeRange(rangeStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	type scenarioRow struct {
		Name  string
		Count int64
	}
	var rows []scenarioRow

	h.DB.Model(&models.SecurityDecision{}).
		Select("scenario as name, COUNT(*) as count").
		Where("source = ? AND created_at >= ? AND scenario != ''", "crowdsec", since).
		Group("scenario").
		Order("count DESC").
		Limit(50).
		Scan(&rows)

	var total int64
	for _, r := range rows {
		total += r.Count
	}

	scenarios := make([]gin.H, 0, len(rows))
	for _, r := range rows {
		pct := 0.0
		if total > 0 {
			pct = math.Round(float64(r.Count)/float64(total)*1000) / 10
		}
		scenarios = append(scenarios, gin.H{
			"name":       r.Name,
			"count":      r.Count,
			"percentage": pct,
		})
	}

	result := gin.H{
		"scenarios": scenarios,
		"total":     total,
		"range":     rangeStr,
		"cached":    false,
	}

	h.dashCache.Set(cacheKey, result, dashScenariosTTL)
	c.JSON(http.StatusOK, result)
}

// ListAlerts wraps the CrowdSec LAPI /v1/alerts endpoint.
func (h *CrowdsecHandler) ListAlerts(c *gin.Context) {
	rangeStr := normalizeRange(c.Query("range"))
	scenario := strings.TrimSpace(c.Query("scenario"))
	limitStr := c.DefaultQuery("limit", "50")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit < 1 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	cacheKey := fmt.Sprintf("dashboard:alerts:%s:%s:%d:%d", rangeStr, scenario, limit, offset)
	if cached, ok := h.dashCache.Get(cacheKey); ok {
		c.JSON(http.StatusOK, cached)
		return
	}

	since, tErr := parseTimeRange(rangeStr)
	if tErr != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": tErr.Error()})
		return
	}

	alerts, total, source := h.fetchLAPIAlerts(c.Request.Context(), since, scenario, limit, offset)

	result := gin.H{
		"alerts": alerts,
		"total":  total,
		"source": source,
		"cached": false,
	}

	h.dashCache.Set(cacheKey, result, dashAlertsTTL)
	c.JSON(http.StatusOK, result)
}

// fetchLAPIAlerts attempts to get alerts from LAPI, falling back to cscli.
func (h *CrowdsecHandler) fetchLAPIAlerts(ctx context.Context, since time.Time, scenario string, limit, offset int) (alerts []interface{}, total int, source string) {
	lapiURL := "http://127.0.0.1:8085"
	if h.Security != nil {
		cfg, err := h.Security.Get()
		if err == nil && cfg != nil && cfg.CrowdSecAPIURL != "" {
			lapiURL = cfg.CrowdSecAPIURL
		}
	}

	baseURL, err := h.resolveLAPIURLValidator(lapiURL)
	if err != nil {
		return h.fetchAlertsCscli(ctx, scenario, limit)
	}

	q := url.Values{}
	q.Set("since", since.Format(time.RFC3339))
	if scenario != "" {
		q.Set("scenario", scenario)
	}
	q.Set("limit", strconv.Itoa(limit))

	endpoint := baseURL.ResolveReference(&url.URL{Path: "/v1/alerts"})
	endpoint.RawQuery = q.Encode()
	reqURL := endpoint.String()

	apiKey := getLAPIKey()

	reqCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	req, reqErr := http.NewRequestWithContext(reqCtx, http.MethodGet, reqURL, http.NoBody)
	if reqErr != nil {
		return h.fetchAlertsCscli(ctx, scenario, limit)
	}
	if apiKey != "" {
		req.Header.Set("X-Api-Key", apiKey)
	}
	req.Header.Set("Accept", "application/json")

	client := network.NewInternalServiceHTTPClient(10 * time.Second)
	resp, doErr := client.Do(req)
	if doErr != nil {
		return h.fetchAlertsCscli(ctx, scenario, limit)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return h.fetchAlertsCscli(ctx, scenario, limit)
	}

	var rawAlerts []interface{}
	if decErr := json.NewDecoder(resp.Body).Decode(&rawAlerts); decErr != nil {
		return h.fetchAlertsCscli(ctx, scenario, limit)
	}

	// Capture full count before slicing for correct pagination semantics
	fullTotal := len(rawAlerts)

	// Apply offset for pagination
	if offset > 0 && offset < len(rawAlerts) {
		rawAlerts = rawAlerts[offset:]
	} else if offset >= len(rawAlerts) {
		rawAlerts = nil
	}

	if limit < len(rawAlerts) {
		rawAlerts = rawAlerts[:limit]
	}

	return rawAlerts, fullTotal, "lapi"
}

// fetchAlertsCscli falls back to using cscli to list alerts.
func (h *CrowdsecHandler) fetchAlertsCscli(ctx context.Context, scenario string, limit int) (alerts []interface{}, total int, source string) {
	args := []string{"alerts", "list", "-o", "json"}
	if scenario != "" {
		args = append(args, "-s", scenario)
	}
	args = append(args, "-l", strconv.Itoa(limit))

	output, err := h.CmdExec.Execute(ctx, "cscli", args...)
	if err != nil {
		logger.Log().WithError(err).Warn("Failed to list alerts via cscli")
		return []interface{}{}, 0, "cscli"
	}

	if jErr := json.Unmarshal(output, &alerts); jErr != nil {
		return []interface{}{}, 0, "cscli"
	}
	return alerts, len(alerts), "cscli"
}

// ExportDecisions exports decisions as downloadable CSV or JSON.
func (h *CrowdsecHandler) ExportDecisions(c *gin.Context) {
	format := strings.ToLower(c.DefaultQuery("format", "csv"))
	rangeStr := normalizeRange(c.Query("range"))
	source := strings.ToLower(c.DefaultQuery("source", "all"))

	if format != "csv" && format != "json" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid format: must be csv or json"})
		return
	}

	validSources := map[string]bool{"crowdsec": true, "waf": true, "ratelimit": true, "manual": true, "all": true}
	if !validSources[source] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid source: must be crowdsec, waf, ratelimit, manual, or all"})
		return
	}

	since, err := parseTimeRange(rangeStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var decisions []models.SecurityDecision
	q := h.DB.Where("created_at >= ?", since)
	if source != "all" {
		q = q.Where("source = ?", source)
	}
	q.Order("created_at DESC").Limit(exportMaxRows).Find(&decisions)

	ts := time.Now().UTC().Format("20060102-150405")

	switch format {
	case "csv":
		filename := fmt.Sprintf("crowdsec-decisions-%s.csv", ts)
		c.Header("Content-Type", "text/csv; charset=utf-8")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))

		w := csv.NewWriter(c.Writer)
		_ = w.Write([]string{"uuid", "ip", "action", "source", "scenario", "rule_id", "host", "country", "created_at", "expires_at"})
		for _, d := range decisions {
			_ = w.Write([]string{
				d.UUID,
				sanitizeCSVField(d.IP),
				d.Action,
				d.Source,
				sanitizeCSVField(d.Scenario),
				sanitizeCSVField(d.RuleID),
				sanitizeCSVField(d.Host),
				sanitizeCSVField(d.Country),
				d.CreatedAt.UTC().Format(time.RFC3339),
				func() string {
					if d.ExpiresAt != nil {
						return d.ExpiresAt.UTC().Format(time.RFC3339)
					}
					return ""
				}(),
			})
		}
		w.Flush()
		if err := w.Error(); err != nil {
			logger.Log().WithError(err).Warn("CSV export write error")
		}

	case "json":
		filename := fmt.Sprintf("crowdsec-decisions-%s.json", ts)
		c.Header("Content-Type", "application/json")
		c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
		c.JSON(http.StatusOK, decisions)
	}
}
