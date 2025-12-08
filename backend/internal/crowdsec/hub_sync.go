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
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Wikid82/charon/backend/internal/logger"
)

// CommandExecutor defines the minimal command execution interface we need for cscli calls.
type CommandExecutor interface {
	Execute(ctx context.Context, name string, args ...string) ([]byte, error)
}

const (
	defaultHubBaseURL     = "https://hub-data.crowdsec.net"
	defaultHubIndexPath   = "/api/index.json"
	defaultHubArchivePath = "/%s.tgz"
	defaultHubPreviewPath = "/%s.yaml"
	maxArchiveSize        = int64(25 * 1024 * 1024) // 25MiB safety cap
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
	Exec         CommandExecutor
	Cache        *HubCache
	DataDir      string
	HTTPClient   *http.Client
	HubBaseURL   string
	PullTimeout  time.Duration
	ApplyTimeout time.Duration
}

// NewHubService constructs a HubService with sane defaults.
func NewHubService(exec CommandExecutor, cache *HubCache, dataDir string) *HubService {
	clientTimeout := 10 * time.Second
	return &HubService{
		Exec:         exec,
		Cache:        cache,
		DataDir:      dataDir,
		HTTPClient:   newHubHTTPClient(clientTimeout),
		HubBaseURL:   normalizeHubBaseURL(os.Getenv("HUB_BASE_URL")),
		PullTimeout:  clientTimeout,
		ApplyTimeout: 15 * time.Second,
	}
}

func newHubHTTPClient(timeout time.Duration) *http.Client {
	return &http.Client{
		Timeout: timeout,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func normalizeHubBaseURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return defaultHubBaseURL
	}
	return strings.TrimRight(trimmed, "/")
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
	target := strings.TrimRight(s.HubBaseURL, "/") + defaultHubIndexPath
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return HubIndex{}, err
	}
	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return HubIndex{}, fmt.Errorf("fetch hub index: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode >= 300 && resp.StatusCode < 400 {
			loc := resp.Header.Get("Location")
			return HubIndex{}, fmt.Errorf("hub index redirect (%d) to %s; install cscli or set HUB_BASE_URL to a JSON hub endpoint", resp.StatusCode, firstNonEmpty(loc, target))
		}
		return HubIndex{}, fmt.Errorf("hub index status %d from %s", resp.StatusCode, target)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, maxArchiveSize))
	if err != nil {
		return HubIndex{}, fmt.Errorf("read hub index: %w", err)
	}
	ct := strings.ToLower(resp.Header.Get("Content-Type"))
	if ct != "" && !strings.Contains(ct, "application/json") {
		if isLikelyHTML(data) {
			return HubIndex{}, fmt.Errorf("hub index responded with HTML; install cscli or set HUB_BASE_URL to a JSON hub endpoint")
		}
		return HubIndex{}, fmt.Errorf("unexpected hub content-type %s; install cscli or set HUB_BASE_URL to a JSON hub endpoint", ct)
	}
	var idx HubIndex
	if err := json.Unmarshal(data, &idx); err != nil {
		if isLikelyHTML(data) {
			return HubIndex{}, fmt.Errorf("hub index responded with HTML; install cscli or set HUB_BASE_URL to a JSON hub endpoint")
		}
		return HubIndex{}, fmt.Errorf("decode hub index: %w", err)
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

	archiveURL := entry.DownloadURL
	if archiveURL == "" {
		archiveURL = fmt.Sprintf(strings.TrimRight(s.HubBaseURL, "/")+defaultHubArchivePath, cleanSlug)
	}
	previewURL := entry.PreviewURL
	if previewURL == "" {
		previewURL = fmt.Sprintf(strings.TrimRight(s.HubBaseURL, "/")+defaultHubPreviewPath, cleanSlug)
	}

	archiveBytes, err := s.fetchWithLimit(pullCtx, archiveURL)
	if err != nil {
		return PullResult{}, fmt.Errorf("download archive: %w", err)
	}

	previewText, err := s.fetchPreview(pullCtx, previewURL)
	if err != nil {
		logger.Log().WithError(err).WithField("slug", cleanSlug).Warn("failed to download preview, falling back to archive inspection")
		previewText = s.peekFirstYAML(archiveBytes)
	}

	cachedMeta, err := s.Cache.Store(pullCtx, cleanSlug, entry.Etag, "hub", previewText, archiveBytes)
	if err != nil {
		return PullResult{}, err
	}

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
	if !hasCS && metaErr != nil {
		msg := "cscli unavailable and no cached preset; pull the preset or install cscli"
		result.ErrorMessage = msg
		return result, errors.New(msg)
	}

	backupPath := filepath.Clean(s.DataDir) + ".backup." + time.Now().Format("20060102-150405")
	result.BackupPath = backupPath
	if err := s.backupExisting(backupPath); err != nil {
		return result, fmt.Errorf("backup: %w", err)
	}

	// Try cscli first
	if hasCS {
		cscliErr := s.runCSCLI(applyCtx, cleanSlug)
		if cscliErr == nil {
			result.Status = "applied"
			result.ReloadHint = true
			result.UsedCSCLI = true
			return result, nil
		}
		logger.Log().WithField("slug", cleanSlug).WithError(cscliErr).Warn("cscli install failed; attempting cache fallback")
	}

	if metaErr != nil {
		_ = s.rollback(backupPath)
		msg := fmt.Sprintf("load cache: %v", metaErr)
		result.ErrorMessage = msg
		return result, errors.New(msg)
	}

	archive, err := os.ReadFile(meta.ArchivePath)
	if err != nil {
		_ = s.rollback(backupPath)
		return result, fmt.Errorf("read archive: %w", err)
	}
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
	defer gr.Close()
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

func (s *HubService) fetchPreview(ctx context.Context, url string) (string, error) {
	if url == "" {
		return "", fmt.Errorf("preview url missing")
	}
	data, err := s.fetchWithLimit(ctx, url)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (s *HubService) fetchWithLimit(ctx context.Context, url string) ([]byte, error) {
	if s.HTTPClient == nil {
		return nil, fmt.Errorf("http client missing")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("http %d", resp.StatusCode)
	}
	lr := io.LimitReader(resp.Body, maxArchiveSize+1024)
	data, err := io.ReadAll(lr)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > maxArchiveSize {
		return nil, fmt.Errorf("payload too large")
	}
	return data, nil
}

func (s *HubService) loadCacheMeta(ctx context.Context, slug string) (CachedPreset, error) {
	if s.Cache == nil {
		return CachedPreset{}, fmt.Errorf("cache unavailable for manual apply")
	}
	meta, err := s.Cache.Load(ctx, slug)
	if err != nil {
		return CachedPreset{}, err
	}
	return meta, nil
}

func findIndexEntry(idx HubIndex, slug string) (HubIndexEntry, bool) {
	for _, i := range idx.Items {
		if i.Name == slug || i.Title == slug {
			return i, true
		}
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
	if err := os.Rename(s.DataDir, backupPath); err != nil {
		return err
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

// extractTarGz validates and extracts archive into targetDir.
func (s *HubService) extractTarGz(ctx context.Context, archive []byte, targetDir string) error {
	if err := os.RemoveAll(targetDir); err != nil {
		return fmt.Errorf("clean target: %w", err)
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return fmt.Errorf("mkdir target: %w", err)
	}

	buf := bytes.NewReader(archive)
	gr, err := gzip.NewReader(buf)
	if err != nil {
		return fmt.Errorf("gunzip: %w", err)
	}
	defer gr.Close()

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
			if err := os.MkdirAll(destPath, hdr.FileInfo().Mode()); err != nil {
				return fmt.Errorf("mkdir %s: %w", destPath, err)
			}
			continue
		}

		if err := os.MkdirAll(filepath.Dir(destPath), 0o755); err != nil {
			return fmt.Errorf("mkdir parent: %w", err)
		}
		f, err := os.OpenFile(destPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, hdr.FileInfo().Mode())
		if err != nil {
			return fmt.Errorf("open %s: %w", destPath, err)
		}
		if _, err := io.Copy(f, tr); err != nil {
			_ = f.Close()
			return fmt.Errorf("write %s: %w", destPath, err)
		}
		if err := f.Close(); err != nil {
			return fmt.Errorf("close %s: %w", destPath, err)
		}
	}
	return nil
}

// peekFirstYAML attempts to extract the first YAML snippet for preview purposes.
func (s *HubService) peekFirstYAML(archive []byte) string {
	if preview := s.findPreviewFile(archive); preview != "" {
		return preview
	}
	return ""
}
