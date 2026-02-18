package crowdsec

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/network"
	"github.com/Wikid82/charon/backend/internal/util"
)

// CommandExecutor defines the minimal command execution interface we need for cscli calls.
type CommandExecutor interface {
	Execute(ctx context.Context, name string, args ...string) ([]byte, error)
}

const (
	defaultHubBaseURL       = "https://hub-data.crowdsec.net"
	defaultHubMirrorBaseURL = "https://raw.githubusercontent.com/crowdsecurity/hub/master"
	defaultHubIndexPath     = "/api/index.json"
	defaultHubArchivePath   = "/%s.tgz"
	defaultHubPreviewPath   = "/%s.yaml"
	maxArchiveSize          = int64(25 * 1024 * 1024) // 25MiB safety cap
	defaultPullTimeout      = 25 * time.Second
	defaultApplyTimeout     = 45 * time.Second
)

// HubIndexEntry represents a single hub catalog entry.
type HubIndexEntry struct {
	Name        string `json:"name"`
	Title       string `json:"title"`
	Version     string `json:"version"`
	Type        string `json:"type"`
	Description string `json:"description"`
	Etag        string `json:"etag"`
	DownloadURL string `json:"download_url"`
	PreviewURL  string `json:"preview_url"`
}

// HubIndex is a small wrapper for hub listing payloads.
type HubIndex struct {
	Items []HubIndexEntry `json:"items"`
}

// PullResult bundles the pull metadata, preview text, and cache entry.
type PullResult struct {
	Meta    CachedPreset
	Preview string
}

// ApplyResult captures the outcome of an apply attempt.
type ApplyResult struct {
	Status        string `json:"status"`
	BackupPath    string `json:"backup_path"`
	ReloadHint    bool   `json:"reload_hint"`
	UsedCSCLI     bool   `json:"used_cscli"`
	CacheKey      string `json:"cache_key"`
	ErrorMessage  string `json:"error,omitempty"`
	AppliedPreset string `json:"slug"`
}

// HubService coordinates hub pulls, caching, and apply operations.
type HubService struct {
	Exec          CommandExecutor
	Cache         *HubCache
	DataDir       string
	HTTPClient    *http.Client
	HubBaseURL    string
	MirrorBaseURL string
	PullTimeout   time.Duration
	ApplyTimeout  time.Duration
}

// validateHubURL validates a hub URL for security (SSRF protection - HIGH-001).
// This function prevents Server-Side Request Forgery by:
// 1. Enforcing HTTPS for production hub URLs
// 2. Allowlisting known CrowdSec hub domains
// 3. Allowing localhost/test URLs for development and testing
//
// Returns: error if URL is invalid or not allowlisted
func validateHubURL(rawURL string) error {
	parsed, err := neturl.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", err)
	}

	// Only allow http/https schemes
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("unsupported scheme: %s (only http and https are allowed)", parsed.Scheme)
	}

	host := parsed.Hostname()
	if host == "" {
		return fmt.Errorf("missing hostname in URL")
	}

	// Allow localhost and test domains for development/testing
	// This is safe because tests control the mock servers
	if host == "localhost" || host == "127.0.0.1" || host == "::1" ||
		strings.HasSuffix(host, ".example.com") || strings.HasSuffix(host, ".example") ||
		host == "example.com" || strings.HasSuffix(host, ".local") ||
		host == "test.hub" { // Allow test.hub for integration tests
		return nil
	}

	// For production URLs, must be HTTPS
	if parsed.Scheme != "https" {
		return fmt.Errorf("hub URLs must use HTTPS (got: %s)", parsed.Scheme)
	}

	// Allowlist known CrowdSec hub domains
	allowedHosts := []string{
		"hub-data.crowdsec.net",
		"hub.crowdsec.net",
		"raw.githubusercontent.com", // GitHub raw content (CrowdSec mirror)
	}

	hostAllowed := false
	for _, allowed := range allowedHosts {
		if host == allowed {
			hostAllowed = true
			break
		}
	}

	if !hostAllowed {
		return fmt.Errorf("unknown hub domain: %s (allowed: hub-data.crowdsec.net, hub.crowdsec.net, raw.githubusercontent.com)", host)
	}

	return nil
}

// NewHubService constructs a HubService with sane defaults.
func NewHubService(exec CommandExecutor, cache *HubCache, dataDir string) *HubService {
	pullTimeout := defaultPullTimeout
	if raw := strings.TrimSpace(os.Getenv("HUB_PULL_TIMEOUT_SECONDS")); raw != "" {
		if secs, err := strconv.Atoi(raw); err == nil && secs > 0 {
			pullTimeout = time.Duration(secs) * time.Second
		}
	}

	applyTimeout := defaultApplyTimeout
	if raw := strings.TrimSpace(os.Getenv("HUB_APPLY_TIMEOUT_SECONDS")); raw != "" {
		if secs, err := strconv.Atoi(raw); err == nil && secs > 0 {
			applyTimeout = time.Duration(secs) * time.Second
		}
	}

	return &HubService{
		Exec:          exec,
		Cache:         cache,
		DataDir:       dataDir,
		HTTPClient:    newHubHTTPClient(pullTimeout),
		HubBaseURL:    normalizeHubBaseURL(os.Getenv("HUB_BASE_URL")),
		MirrorBaseURL: normalizeHubBaseURL(firstNonEmpty(os.Getenv("HUB_MIRROR_BASE_URL"), defaultHubMirrorBaseURL)),
		PullTimeout:   pullTimeout,
		ApplyTimeout:  applyTimeout,
	}
}

// newHubHTTPClient creates an SSRF-safe HTTP client for hub operations.
// Hub URLs are validated by validateHubURL() which:
// - Enforces HTTPS for production
// - Allowlists known CrowdSec domains (hub-data.crowdsec.net, hub.crowdsec.net, raw.githubusercontent.com)
// - Allows localhost for testing
// Using network.NewSafeHTTPClient provides defense-in-depth at the connection level.
func newHubHTTPClient(timeout time.Duration) *http.Client {
	return network.NewSafeHTTPClient(
		network.WithTimeout(timeout),
		network.WithAllowLocalhost(), // Allow localhost for testing
		network.WithAllowedDomains(
			"hub-data.crowdsec.net",
			"hub.crowdsec.net",
			"raw.githubusercontent.com",
		),
	)
}

func normalizeHubBaseURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return defaultHubBaseURL
	}
	return strings.TrimRight(trimmed, "/")
}

func (s *HubService) hubBaseCandidates() []string {
	candidates := []string{s.HubBaseURL, s.MirrorBaseURL, defaultHubMirrorBaseURL, defaultHubBaseURL}
	return uniqueStrings(candidates)
}

func buildIndexURL(base string) string {
	normalized := normalizeHubBaseURL(base)
	if strings.HasSuffix(strings.ToLower(normalized), ".json") {
		return normalized
	}
	return strings.TrimRight(normalized, "/") + defaultHubIndexPath
}

func indexURLCandidates(base string) []string {
	normalized := normalizeHubBaseURL(base)
	primary := buildIndexURL(normalized)
	if strings.Contains(normalized, "github.io") || strings.Contains(normalized, "githubusercontent.com") {
		mirrorIndex := strings.TrimRight(normalized, "/") + "/.index.json"
		return uniqueStrings([]string{mirrorIndex, primary})
	}

	return []string{primary}
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{})
	out := make([]string, 0, len(values))
	for _, v := range values {
		if _, ok := seen[v]; ok {
			continue
		}
		seen[v] = struct{}{}
		out = append(out, v)
	}
	return out
}

func buildResourceURLs(explicit, slug, pattern string, bases []string) []string {
	urls := make([]string, 0, len(bases)+1)
	if explicit != "" {
		urls = append(urls, explicit)
	}
	for _, base := range bases {
		if base == "" {
			continue
		}
		urls = append(urls, fmt.Sprintf(strings.TrimRight(base, "/")+pattern, slug))
	}
	return uniqueStrings(urls)
}

// FetchIndex downloads the hub index. If the hub is unreachable, returns ErrCacheMiss.
func (s *HubService) FetchIndex(ctx context.Context) (HubIndex, error) {
	if s.Exec != nil {
		if idx, err := s.fetchIndexCSCLI(ctx); err == nil {
			return idx, nil
		} else {
			logger.Log().WithError(err).Debug("cscli hub index failed, falling back to direct hub fetch")
		}
	}
	return s.fetchIndexHTTP(ctx)
}

func (s *HubService) fetchIndexCSCLI(ctx context.Context) (HubIndex, error) {
	if s.Exec == nil {
		return HubIndex{}, fmt.Errorf("executor missing")
	}
	cmdCtx, cancel := context.WithTimeout(ctx, s.PullTimeout)
	defer cancel()

	output, err := s.Exec.Execute(cmdCtx, "cscli", "hub", "list", "-o", "json")
	if err != nil {
		return HubIndex{}, err
	}
	return parseCSCLIIndex(output)
}

func parseCSCLIIndex(raw []byte) (HubIndex, error) {
	bucket := map[string][]map[string]any{}
	if err := json.Unmarshal(raw, &bucket); err != nil {
		return HubIndex{}, fmt.Errorf("parse cscli index: %w", err)
	}
	items := make([]HubIndexEntry, 0)
	for section, list := range bucket {
		for _, obj := range list {
			name := sanitizeSlug(asString(obj["name"]))
			if name == "" {
				continue
			}
			entry := HubIndexEntry{
				Name:        name,
				Title:       firstNonEmpty(asString(obj["title"]), name),
				Version:     asString(obj["version"]),
				Type:        firstNonEmpty(asString(obj["type"]), section),
				Description: asString(obj["description"]),
				Etag:        firstNonEmpty(asString(obj["etag"]), asString(obj["hash"])),
				DownloadURL: asString(obj["download_url"]),
				PreviewURL:  asString(obj["preview_url"]),
			}
			if entry.Title == "" {
				entry.Title = entry.Name
			}
			if entry.Description == "" {
				entry.Description = entry.Title
			}
			items = append(items, entry)
		}
	}
	if len(items) == 0 {
		return HubIndex{}, fmt.Errorf("empty cscli index")
	}
	return HubIndex{Items: items}, nil
}

func parseRawIndex(raw []byte, baseURL string) (HubIndex, error) {
	bucket := map[string]map[string]struct {
		Path        string `json:"path"`
		Version     string `json:"version"`
		Description string `json:"description"`
	}{}
	if err := json.Unmarshal(raw, &bucket); err != nil {
		return HubIndex{}, fmt.Errorf("parse raw index: %w", err)
	}

	items := make([]HubIndexEntry, 0)
	for section, list := range bucket {
		for name, obj := range list {
			cleanName := sanitizeSlug(name)
			if cleanName == "" {
				continue
			}

			// Construct URLs
			rootURL := baseURL
			if strings.HasSuffix(rootURL, "/.index.json") {
				rootURL = strings.TrimSuffix(rootURL, "/.index.json")
			} else if strings.HasSuffix(rootURL, "/api/index.json") {
				rootURL = strings.TrimSuffix(rootURL, "/api/index.json")
			}

			dlURL := fmt.Sprintf("%s/%s", strings.TrimRight(rootURL, "/"), obj.Path)

			entry := HubIndexEntry{
				Name:        cleanName,
				Title:       cleanName,
				Version:     obj.Version,
				Type:        section,
				Description: obj.Description,
				Etag:        obj.Version,
				DownloadURL: dlURL,
				PreviewURL:  dlURL,
			}
			items = append(items, entry)
		}
	}
	if len(items) == 0 {
		return HubIndex{}, fmt.Errorf("empty raw index")
	}
	return HubIndex{Items: items}, nil
}

func asString(v any) string {
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case fmt.Stringer:
		return val.String()
	default:
		return fmt.Sprintf("%v", v)
	}
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func (s *HubService) fetchIndexHTTP(ctx context.Context) (HubIndex, error) {
	if s.HTTPClient == nil {
		return HubIndex{}, fmt.Errorf("http client missing")
	}

	var targets []string
	for _, base := range s.hubBaseCandidates() {
		targets = append(targets, indexURLCandidates(base)...)
	}
	targets = uniqueStrings(targets)
	var errs []error

	for attempt, target := range targets {
		idx, err := s.fetchIndexHTTPFromURL(ctx, target)
		if err == nil {
			logger.Log().WithField("hub_index", target).WithField("fallback_used", attempt > 0).Info("hub index fetched")
			return idx, nil
		}
		errs = append(errs, fmt.Errorf("%s: %w", target, err))
		if e, ok := err.(interface{ CanFallback() bool }); ok && e.CanFallback() {
			logger.Log().WithField("hub_index", target).WithField("attempt", attempt+1).WithError(err).Warn("hub index fetch failed, trying mirror")
			continue
		}
		break
	}

	if len(errs) == 1 {
		return HubIndex{}, fmt.Errorf("fetch hub index: %w", errs[0])
	}

	return HubIndex{}, fmt.Errorf("fetch hub index: %w", errors.Join(errs...))
}

type hubHTTPError struct {
	url        string
	statusCode int
	inner      error
	fallback   bool
}

func (h hubHTTPError) Error() string {
	if h.inner != nil {
		return fmt.Sprintf("%s (status %d): %v", h.url, h.statusCode, h.inner)
	}
	return fmt.Sprintf("%s (status %d)", h.url, h.statusCode)
}

func (h hubHTTPError) Unwrap() error { return h.inner }
func (h hubHTTPError) CanFallback() bool {
	return h.fallback
}

func (s *HubService) fetchIndexHTTPFromURL(ctx context.Context, target string) (HubIndex, error) {
	// CRITICAL FIX: Validate hub URL before making HTTP request (HIGH-001)
	if err := validateHubURL(target); err != nil {
		return HubIndex{}, fmt.Errorf("invalid hub URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, http.NoBody)
	if err != nil {
		return HubIndex{}, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return HubIndex{}, fmt.Errorf("fetch hub index: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			logger.Log().WithError(closeErr).Warn("Failed to close response body")
		}
	}()
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode >= 300 && resp.StatusCode < 400 {
			loc := resp.Header.Get("Location")
			return HubIndex{}, hubHTTPError{url: target, statusCode: resp.StatusCode, inner: fmt.Errorf("hub index redirect to %s; install cscli or set HUB_BASE_URL to a JSON hub endpoint", firstNonEmpty(loc, target)), fallback: true}
		}
		return HubIndex{}, hubHTTPError{url: target, statusCode: resp.StatusCode, fallback: resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden || resp.StatusCode >= 500}
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxArchiveSize))
	if err != nil {
		return HubIndex{}, fmt.Errorf("read hub index: %w", err)
	}
	ct := strings.ToLower(resp.Header.Get("Content-Type"))
	if ct != "" && !strings.Contains(ct, "application/json") && !strings.Contains(ct, "text/plain") {
		if isLikelyHTML(data) {
			return HubIndex{}, hubHTTPError{url: target, statusCode: resp.StatusCode, inner: fmt.Errorf("hub index responded with HTML; install cscli or set HUB_BASE_URL to a JSON hub endpoint"), fallback: true}
		}
		return HubIndex{}, hubHTTPError{url: target, statusCode: resp.StatusCode, inner: fmt.Errorf("unexpected hub content-type %s; install cscli or set HUB_BASE_URL to a JSON hub endpoint", ct), fallback: true}
	}
	var idx HubIndex
	if err := json.Unmarshal(data, &idx); err != nil || len(idx.Items) == 0 {
		// Try parsing as raw index (map of maps)
		if rawIdx, rawErr := parseRawIndex(data, target); rawErr == nil {
			return rawIdx, nil
		}

		if err != nil {
			if isLikelyHTML(data) {
				return HubIndex{}, hubHTTPError{url: target, statusCode: resp.StatusCode, inner: fmt.Errorf("hub index responded with HTML; install cscli or set HUB_BASE_URL to a JSON hub endpoint"), fallback: true}
			}
			return HubIndex{}, fmt.Errorf("decode hub index from %s: %w", target, err)
		}
	}
	return idx, nil
}

// Pull downloads a preset bundle, validates it, and stores it in cache.
func (s *HubService) Pull(ctx context.Context, slug string) (PullResult, error) {
	if s.Cache == nil {
		return PullResult{}, fmt.Errorf("cache unavailable")
	}
	cleanSlug := sanitizeSlug(slug)
	if cleanSlug == "" {
		return PullResult{}, fmt.Errorf("invalid slug")
	}

	// Attempt to load non-expired cache first.
	cached, err := s.Cache.Load(ctx, cleanSlug)
	if err == nil {
		preview, loadErr := os.ReadFile(cached.PreviewPath)
		if loadErr == nil {
			return PullResult{Meta: cached, Preview: string(preview)}, nil
		}
	} else if errors.Is(err, ErrCacheExpired) {
		_ = s.Cache.Evict(ctx, cleanSlug)
	}

	// Refresh index and download bundle
	pullCtx, cancel := context.WithTimeout(ctx, s.PullTimeout)
	defer cancel()

	idx, err := s.FetchIndex(pullCtx)
	if err != nil {
		return PullResult{}, err
	}

	entry, ok := findIndexEntry(idx, cleanSlug)
	if !ok {
		return PullResult{}, fmt.Errorf("preset not found in hub")
	}

	entrySlug := firstNonEmpty(entry.Name, cleanSlug)

	archiveCandidates := buildResourceURLs(entry.DownloadURL, entrySlug, defaultHubArchivePath, s.hubBaseCandidates())
	previewCandidates := buildResourceURLs(entry.PreviewURL, entrySlug, defaultHubPreviewPath, s.hubBaseCandidates())

	archiveBytes, archiveURL, err := s.fetchWithFallback(pullCtx, archiveCandidates)
	if err != nil {
		return PullResult{}, fmt.Errorf("download archive from %s: %w", archiveURL, err)
	}

	// Check if it's a tar.gz
	if !isGzip(archiveBytes) {
		// Assume it's a raw file (YAML/JSON) and wrap it
		filename := filepath.Base(archiveURL)
		if filename == "." || filename == "/" {
			filename = cleanSlug + ".yaml"
		}

		var buf bytes.Buffer
		gw := gzip.NewWriter(&buf)
		tw := tar.NewWriter(gw)

		hdr := &tar.Header{
			Name: filename,
			Mode: 0o644,
			Size: int64(len(archiveBytes)),
		}
		if writeHeaderErr := tw.WriteHeader(hdr); writeHeaderErr != nil {
			return PullResult{}, fmt.Errorf("create tar header: %w", writeHeaderErr)
		}
		if _, writeErr := tw.Write(archiveBytes); writeErr != nil {
			return PullResult{}, fmt.Errorf("write tar content: %w", writeErr)
		}
		_ = tw.Close()
		_ = gw.Close()

		archiveBytes = buf.Bytes()
	}

	previewText, err := s.fetchPreview(pullCtx, previewCandidates)
	if err != nil {
		logger.Log().WithField("error", util.SanitizeForLog(err.Error())).WithField("slug", util.SanitizeForLog(cleanSlug)).Warn("failed to download preview, falling back to archive inspection")
		previewText = s.peekFirstYAML(archiveBytes)
	}

	logger.Log().WithField("slug", util.SanitizeForLog(cleanSlug)).WithField("etag", util.SanitizeForLog(entry.Etag)).WithField("archive_size", len(archiveBytes)).WithField("preview_size", len(previewText)).WithField("hub_endpoint", util.SanitizeForLog(archiveURL)).Info("storing preset in cache")

	cachedMeta, err := s.Cache.Store(pullCtx, cleanSlug, entry.Etag, "hub", previewText, archiveBytes)
	if err != nil {
		logger.Log().WithField("error", util.SanitizeForLog(err.Error())).WithField("slug", util.SanitizeForLog(cleanSlug)).Error("failed to store preset in cache")
		return PullResult{}, fmt.Errorf("cache store: %w", err)
	}

	logger.Log().WithField("slug", util.SanitizeForLog(cachedMeta.Slug)).WithField("cache_key", util.SanitizeForLog(cachedMeta.CacheKey)).WithField("archive_path", util.SanitizeForLog(cachedMeta.ArchivePath)).WithField("preview_path", util.SanitizeForLog(cachedMeta.PreviewPath)).Info("preset successfully cached")

	return PullResult{Meta: cachedMeta, Preview: previewText}, nil
}

// Apply installs the preset, preferring cscli when available. Falls back to manual extraction.
func (s *HubService) Apply(ctx context.Context, slug string) (ApplyResult, error) {
	cleanSlug := sanitizeSlug(slug)
	if cleanSlug == "" {
		return ApplyResult{}, fmt.Errorf("invalid slug")
	}
	applyCtx, cancel := context.WithTimeout(ctx, s.ApplyTimeout)
	defer cancel()

	result := ApplyResult{AppliedPreset: cleanSlug, Status: "failed"}
	meta, metaErr := s.loadCacheMeta(applyCtx, cleanSlug)
	if metaErr == nil {
		result.CacheKey = meta.CacheKey
	}
	hasCS := s.hasCSCLI(applyCtx)

	// Read archive into memory BEFORE backup, since cache is inside DataDir.
	// If we backup first, the archive path becomes invalid (file moved).
	var archive []byte
	var archiveReadErr error
	if metaErr == nil {
		archive, archiveReadErr = os.ReadFile(meta.ArchivePath)
		if archiveReadErr != nil {
			logger.Log().WithField("error", util.SanitizeForLog(archiveReadErr.Error())).WithField("archive_path", util.SanitizeForLog(meta.ArchivePath)).
				Warn("failed to read cached archive before backup")
		}
	}

	backupPath := filepath.Clean(s.DataDir) + ".backup." + time.Now().Format("20060102-150405")
	if err := s.backupExisting(backupPath); err != nil {
		// Only set BackupPath if backup was actually created
		return result, fmt.Errorf("backup: %w", err)
	}
	// Set BackupPath only after successful backup
	result.BackupPath = backupPath

	// Try cscli first
	if hasCS {
		cscliErr := s.runCSCLI(applyCtx, cleanSlug)
		if cscliErr == nil {
			result.Status = "applied"
			result.ReloadHint = true
			result.UsedCSCLI = true
			return result, nil
		}
		logger.Log().WithField("slug", util.SanitizeForLog(cleanSlug)).WithField("error", util.SanitizeForLog(cscliErr.Error())).Warn("cscli install failed; attempting cache fallback")
	}

	// Handle cache miss OR failed archive read - need to refresh cache
	if metaErr != nil || archiveReadErr != nil {
		originalErr := metaErr
		if originalErr == nil {
			originalErr = archiveReadErr
		}
		refreshed, refreshErr := s.refreshCache(applyCtx, cleanSlug, originalErr)
		if refreshErr != nil {
			_ = s.rollback(backupPath)
			logger.Log().WithField("error", util.SanitizeForLog(refreshErr.Error())).WithField("slug", util.SanitizeForLog(cleanSlug)).WithField("backup_path", util.SanitizeForLog(backupPath)).Warn("cache refresh failed; rolled back backup")
			msg := fmt.Sprintf("load cache for %s: %v", cleanSlug, refreshErr)
			result.ErrorMessage = msg
			return result, fmt.Errorf("load cache for %s: %w", cleanSlug, refreshErr)
		}
		meta = refreshed
		result.CacheKey = meta.CacheKey

		// Re-read archive from the newly refreshed cache location
		archive, archiveReadErr = os.ReadFile(meta.ArchivePath)
		if archiveReadErr != nil {
			_ = s.rollback(backupPath)
			return result, fmt.Errorf("read archive after refresh: %w", archiveReadErr)
		}
	}

	// Use pre-loaded archive bytes
	if err := s.extractTarGz(applyCtx, archive, s.DataDir); err != nil {
		_ = s.rollback(backupPath)
		return result, fmt.Errorf("extract: %w", err)
	}

	result.Status = "applied"
	result.ReloadHint = true
	result.UsedCSCLI = false
	return result, nil
}

func (s *HubService) findPreviewFile(data []byte) string {
	buf := bytes.NewReader(data)
	gr, err := gzip.NewReader(buf)
	if err != nil {
		return ""
	}
	defer func() { _ = gr.Close() }()
	tr := tar.NewReader(gr)
	for {
		hdr, err := tr.Next()
		if err != nil {
			return ""
		}
		if hdr.FileInfo().IsDir() {
			continue
		}
		name := strings.ToLower(hdr.Name)
		if strings.HasSuffix(name, ".yaml") || strings.HasSuffix(name, ".yml") {
			limited := io.LimitReader(tr, 2048)
			content, err := io.ReadAll(limited)
			if err == nil {
				return string(content)
			}
		}
	}
}

func (s *HubService) fetchPreview(ctx context.Context, urls []string) (string, error) {
	data, used, err := s.fetchWithFallback(ctx, urls)
	if err != nil {
		return "", fmt.Errorf("preview fetch failed (last endpoint %s): %w", used, err)
	}
	return string(data), nil
}

func (s *HubService) fetchWithFallback(ctx context.Context, urls []string) (data []byte, used string, err error) {
	candidates := uniqueStrings(urls)
	if len(candidates) == 0 {
		return nil, "", fmt.Errorf("no endpoints provided")
	}
	var errs []error
	var last string
	for attempt, u := range candidates {
		last = u
		data, err := s.fetchWithLimitFromURL(ctx, u)
		if err == nil {
			logger.Log().WithField("endpoint", util.SanitizeForLog(u)).WithField("fallback_used", attempt > 0).Info("hub fetch succeeded")
			return data, u, nil
		}
		errs = append(errs, fmt.Errorf("%s: %w", u, err))
		if e, ok := err.(interface{ CanFallback() bool }); ok && e.CanFallback() {
			logger.Log().WithField("error", util.SanitizeForLog(err.Error())).WithField("endpoint", util.SanitizeForLog(u)).WithField("attempt", attempt+1).Warn("hub fetch failed, attempting fallback")
			continue
		}
		break
	}

	if len(errs) == 1 {
		return nil, last, errs[0]
	}

	return nil, last, errors.Join(errs...)
}

func (s *HubService) fetchWithLimitFromURL(ctx context.Context, url string) ([]byte, error) {
	// CRITICAL FIX: Validate hub URL before making HTTP request (HIGH-001)
	if err := validateHubURL(url); err != nil {
		return nil, fmt.Errorf("invalid hub URL: %w", err)
	}

	if s.HTTPClient == nil {
		return nil, fmt.Errorf("http client missing")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, http.NoBody)
	if err != nil {
		return nil, err
	}
	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request %s: %w", url, err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			logger.Log().WithError(closeErr).Warn("Failed to close response body")
		}
	}()
	if resp.StatusCode != http.StatusOK {
		return nil, hubHTTPError{url: url, statusCode: resp.StatusCode, fallback: resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden || resp.StatusCode >= 500}
	}
	lr := io.LimitReader(resp.Body, maxArchiveSize+1024)
	data, err := io.ReadAll(lr)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", url, err)
	}
	if int64(len(data)) > maxArchiveSize {
		return nil, fmt.Errorf("payload too large from %s", url)
	}
	return data, nil
}

func (s *HubService) loadCacheMeta(ctx context.Context, slug string) (CachedPreset, error) {
	if s.Cache == nil {
		logger.Log().WithField("slug", util.SanitizeForLog(slug)).Error("cache unavailable for apply")
		return CachedPreset{}, fmt.Errorf("cache unavailable for manual apply")
	}
	logger.Log().WithField("slug", util.SanitizeForLog(slug)).Debug("attempting to load cached preset metadata")
	meta, err := s.Cache.Load(ctx, slug)
	if err != nil {
		logger.Log().WithField("error", util.SanitizeForLog(err.Error())).WithField("slug", util.SanitizeForLog(slug)).Warn("failed to load cached preset metadata")
		return CachedPreset{}, fmt.Errorf("load cache for %s: %w", slug, err)
	}
	logger.Log().WithField("slug", util.SanitizeForLog(meta.Slug)).WithField("cache_key", util.SanitizeForLog(meta.CacheKey)).WithField("archive_path", util.SanitizeForLog(meta.ArchivePath)).Info("successfully loaded cached preset metadata")
	return meta, nil
}

func (s *HubService) refreshCache(ctx context.Context, slug string, metaErr error) (CachedPreset, error) {
	if !errors.Is(metaErr, ErrCacheMiss) && !errors.Is(metaErr, ErrCacheExpired) {
		return CachedPreset{}, metaErr
	}
	if errors.Is(metaErr, ErrCacheExpired) && s.Cache != nil {
		if err := s.Cache.Evict(ctx, slug); err != nil {
			logger.Log().WithField("error", util.SanitizeForLog(err.Error())).WithField("slug", util.SanitizeForLog(slug)).Warn("failed to evict expired cache before refresh")
		}
	}
	logger.Log().WithField("error", util.SanitizeForLog(metaErr.Error())).WithField("slug", util.SanitizeForLog(slug)).Info("attempting to repull preset after cache load failure")
	refreshed, pullErr := s.Pull(ctx, slug)
	if pullErr != nil {
		return CachedPreset{}, fmt.Errorf("%w: refresh cache: %v", metaErr, pullErr)
	}
	return refreshed.Meta, nil
}

func findIndexEntry(idx HubIndex, slug string) (HubIndexEntry, bool) {
	for _, i := range idx.Items {
		if i.Name == slug || i.Title == slug {
			return i, true
		}
	}

	normalized := strings.TrimSpace(slug)
	if normalized == "" {
		return HubIndexEntry{}, false
	}

	if !strings.Contains(normalized, "/") {
		namespaced := "crowdsecurity/" + normalized
		var candidate HubIndexEntry
		found := false
		for _, i := range idx.Items {
			if i.Name == namespaced || i.Title == namespaced || strings.HasSuffix(i.Name, "/"+normalized) || strings.HasSuffix(i.Title, "/"+normalized) {
				if found {
					return HubIndexEntry{}, false
				}
				candidate = i
				found = true
			}
		}
		if found {
			return candidate, true
		}
	}

	var suffixCandidate HubIndexEntry
	foundSuffix := false
	for _, i := range idx.Items {
		if strings.HasSuffix(i.Name, "/"+normalized) || strings.HasSuffix(i.Title, "/"+normalized) {
			if foundSuffix {
				return HubIndexEntry{}, false
			}
			suffixCandidate = i
			foundSuffix = true
		}
	}

	if foundSuffix {
		return suffixCandidate, true
	}

	return HubIndexEntry{}, false
}

func isLikelyHTML(data []byte) bool {
	trimmed := bytes.TrimSpace(data)
	if len(trimmed) == 0 {
		return false
	}
	lower := bytes.ToLower(trimmed)
	if bytes.HasPrefix(lower, []byte("<!doctype")) || bytes.HasPrefix(lower, []byte("<html")) {
		return true
	}
	return bytes.Contains(lower, []byte("<html"))
}

func (s *HubService) hasCSCLI(ctx context.Context) bool {
	if s.Exec == nil {
		return false
	}
	_, err := s.Exec.Execute(ctx, "cscli", "version")
	if err != nil {
		logger.Log().WithError(err).Debug("cscli not available")
		return false
	}
	return true
}

func (s *HubService) runCSCLI(ctx context.Context, slug string) error {
	if s.Exec == nil {
		return fmt.Errorf("executor missing")
	}
	safeSlug := cleanShellArg(slug)
	if safeSlug == "" {
		return fmt.Errorf("invalid slug")
	}
	if _, err := s.Exec.Execute(ctx, "cscli", "hub", "update"); err != nil {
		logger.Log().WithError(err).Warn("cscli hub update failed")
	}
	_, err := s.Exec.Execute(ctx, "cscli", "hub", "install", safeSlug)
	return err
}

func cleanShellArg(val string) string {
	return sanitizeSlug(val)
}

func (s *HubService) backupExisting(backupPath string) error {
	if _, err := os.Stat(s.DataDir); errors.Is(err, os.ErrNotExist) {
		return nil
	}

	// First try rename for performance (atomic operation)
	if err := os.Rename(s.DataDir, backupPath); err == nil {
		return nil
	}

	// If rename fails (e.g., device busy, cross-device), use copy approach
	logger.Log().WithField("data_dir", s.DataDir).WithField("backup_path", backupPath).Info("rename failed; using copy-based backup")

	// Create backup directory
	if err := os.MkdirAll(backupPath, 0o700); err != nil {
		return fmt.Errorf("mkdir backup: %w", err)
	}

	// Copy directory contents recursively
	if err := copyDir(s.DataDir, backupPath); err != nil {
		_ = os.RemoveAll(backupPath)
		return fmt.Errorf("copy backup: %w", err)
	}

	return nil
}

func (s *HubService) rollback(backupPath string) error {
	_ = os.RemoveAll(s.DataDir)
	if backupPath == "" {
		return nil
	}
	if _, err := os.Stat(backupPath); err == nil {
		return os.Rename(backupPath, s.DataDir)
	}
	return nil
}

// emptyDir removes all contents of a directory but leaves the directory itself.
func emptyDir(dir string) error {
	d, err := os.Open(dir) // #nosec G304 -- Directory path from validated backup root // #nosec G304 -- Directory path from validated backup root
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer func() {
		if closeErr := d.Close(); closeErr != nil {
			logger.Log().WithError(closeErr).Warn("Failed to close directory")
		}
	}()
	names, err := d.Readdirnames(-1)
	if err != nil {
		return err
	}
	for _, name := range names {
		if err := os.RemoveAll(filepath.Join(dir, name)); err != nil {
			return err
		}
	}
	return nil
}

// extractTarGz validates and extracts archive into targetDir.
func (s *HubService) extractTarGz(ctx context.Context, archive []byte, targetDir string) error {
	// Clear target directory contents instead of removing the directory itself
	// to avoid "device or resource busy" errors if targetDir is a mount point.
	if err := emptyDir(targetDir); err != nil {
		return fmt.Errorf("clean target: %w", err)
	}
	if err := os.MkdirAll(targetDir, 0o700); err != nil {
		return fmt.Errorf("mkdir target: %w", err)
	}

	buf := bytes.NewReader(archive)
	gr, err := gzip.NewReader(buf)
	if err != nil {
		return fmt.Errorf("gunzip: %w", err)
	}
	defer func() { _ = gr.Close() }()

	tr := tar.NewReader(gr)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		hdr, err := tr.Next()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("read tar: %w", err)
		}
		if hdr.FileInfo().Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("symlinks not allowed in archive")
		}
		if hdr.FileInfo().Mode()&os.ModeType != 0 && !hdr.FileInfo().Mode().IsRegular() && !hdr.FileInfo().IsDir() {
			continue
		}
		cleanName := filepath.Clean(hdr.Name)
		if strings.HasPrefix(cleanName, "..") || strings.Contains(cleanName, ".."+string(os.PathSeparator)) || filepath.IsAbs(cleanName) {
			return fmt.Errorf("unsafe path %s", hdr.Name)
		}
		destPath := filepath.Join(targetDir, cleanName)
		if !strings.HasPrefix(destPath, filepath.Clean(targetDir)) {
			return fmt.Errorf("path escapes target: %s", hdr.Name)
		}

		if hdr.FileInfo().IsDir() {
			if mkdirErr := os.MkdirAll(destPath, hdr.FileInfo().Mode()); mkdirErr != nil {
				return fmt.Errorf("mkdir %s: %w", destPath, mkdirErr)
			}
			continue
		}

		if mkdirErr := os.MkdirAll(filepath.Dir(destPath), 0o700); mkdirErr != nil {
			return fmt.Errorf("mkdir parent: %w", mkdirErr)
		}
		f, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, hdr.FileInfo().Mode()) // #nosec G304 -- Dest path from tar archive extraction // #nosec G304 -- Dest path from tar archive extraction
		if err != nil {
			return fmt.Errorf("open %s: %w", destPath, err)
		}
		// Limit decompressed size to prevent decompression bombs (100MB limit)
		const maxDecompressedSize = 100 * 1024 * 1024 // 100MB
		limitedReader := io.LimitReader(tr, maxDecompressedSize)
		written, err := io.Copy(f, limitedReader)
		if err != nil {
			_ = f.Close()
			return fmt.Errorf("write %s: %w", destPath, err)
		}
		// Verify we didn't hit the limit (potential attack)
		if written >= maxDecompressedSize {
			_ = f.Close()
			return fmt.Errorf("file %s exceeded decompression limit (%d bytes), potential decompression bomb", destPath, maxDecompressedSize)
		}
		if err := f.Close(); err != nil {
			return fmt.Errorf("close %s: %w", destPath, err)
		}
	}
	return nil
}

// copyDir recursively copies a directory tree.
func copyDir(src, dst string) error {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return fmt.Errorf("stat src: %w", err)
	}
	if !srcInfo.IsDir() {
		return fmt.Errorf("src is not a directory")
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return fmt.Errorf("read dir: %w", err)
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := os.MkdirAll(dstPath, 0o700); err != nil {
				return fmt.Errorf("mkdir %s: %w", dstPath, err)
			}
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}
	return nil
}

// copyFile copies a single file.
func copyFile(src, dst string) error {
	srcFile, err := os.Open(src) // #nosec G304 -- Source path from copyDir recursive call // #nosec G304 -- Source path from copyDir recursive call
	if err != nil {
		return fmt.Errorf("open src: %w", err)
	}
	defer func() {
		if closeErr := srcFile.Close(); closeErr != nil {
			logger.Log().WithError(closeErr).Warn("Failed to close source file")
		}
	}()

	srcInfo, err := srcFile.Stat()
	if err != nil {
		return fmt.Errorf("stat src: %w", err)
	}

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, srcInfo.Mode()) // #nosec G304 -- Dst path from copyFile internal call
	if err != nil {
		return fmt.Errorf("create dst: %w", err)
	}
	defer func() {
		if err := dstFile.Close(); err != nil {
			logger.Log().WithError(err).Warn("Failed to close destination file")
		}
	}()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return fmt.Errorf("copy: %w", err)
	}

	return dstFile.Sync()
}

// peekFirstYAML attempts to extract the first YAML snippet for preview purposes.
func (s *HubService) peekFirstYAML(archive []byte) string {
	if preview := s.findPreviewFile(archive); preview != "" {
		return preview
	}
	return ""
}

func isGzip(data []byte) bool {
	if len(data) < 2 {
		return false
	}
	return data[0] == 0x1f && data[1] == 0x8b
}
