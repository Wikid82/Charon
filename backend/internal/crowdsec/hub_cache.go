package crowdsec

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/util"
)

var (
	ErrCacheMiss    = errors.New("cache miss")
	ErrCacheExpired = errors.New("cache expired")
)

// CachedPreset captures metadata about a pulled preset bundle.
type CachedPreset struct {
	Slug        string    `json:"slug"`
	CacheKey    string    `json:"cache_key"`
	Etag        string    `json:"etag"`
	Source      string    `json:"source"`
	RetrievedAt time.Time `json:"retrieved_at"`
	PreviewPath string    `json:"preview_path"`
	ArchivePath string    `json:"archive_path"`
	SizeBytes   int64     `json:"size_bytes"`
}

// HubCache persists pulled bundles on disk with TTL-based eviction.
type HubCache struct {
	baseDir string
	ttl     time.Duration
	nowFn   func() time.Time
}

var slugPattern = regexp.MustCompile(`^[A-Za-z0-9./_-]+$`)

// NewHubCache constructs a cache rooted at baseDir with the provided TTL.
func NewHubCache(baseDir string, ttl time.Duration) (*HubCache, error) {
	if baseDir == "" {
		return nil, fmt.Errorf("baseDir required")
	}
	if err := os.MkdirAll(baseDir, 0o700); err != nil {
		return nil, fmt.Errorf("create cache dir: %w", err)
	}
	return &HubCache{baseDir: baseDir, ttl: ttl, nowFn: time.Now}, nil
}

// TTL returns the configured time-to-live for cached entries.
func (c *HubCache) TTL() time.Duration {
	return c.ttl
}

// Store writes the bundle archive and preview to disk and returns the cache metadata.
func (c *HubCache) Store(ctx context.Context, slug, etag, source, preview string, archive []byte) (CachedPreset, error) {
	if err := ctx.Err(); err != nil {
		return CachedPreset{}, err
	}
	cleanSlug := sanitizeSlug(slug)
	if cleanSlug == "" {
		return CachedPreset{}, fmt.Errorf("invalid slug")
	}
	dir := filepath.Join(c.baseDir, cleanSlug)
	logger.Log().WithField("slug", util.SanitizeForLog(cleanSlug)).WithField("cache_dir", util.SanitizeForLog(dir)).WithField("archive_size", len(archive)).Debug("storing preset in cache")

	if err := os.MkdirAll(dir, 0o700); err != nil {
		logger.Log().WithError(err).WithField("dir", util.SanitizeForLog(dir)).Error("failed to create cache directory")
		return CachedPreset{}, fmt.Errorf("create slug dir: %w", err)
	}

	ts := c.nowFn().UTC()
	cacheKey := fmt.Sprintf("%s-%d", cleanSlug, ts.Unix())

	archivePath := filepath.Join(dir, "bundle.tgz")
	if err := os.WriteFile(archivePath, archive, 0o600); err != nil {
		return CachedPreset{}, fmt.Errorf("write archive: %w", err)
	}
	previewPath := filepath.Join(dir, "preview.yaml")
	if err := os.WriteFile(previewPath, []byte(preview), 0o600); err != nil {
		return CachedPreset{}, fmt.Errorf("write preview: %w", err)
	}

	meta := CachedPreset{
		Slug:        cleanSlug,
		CacheKey:    cacheKey,
		Etag:        etag,
		Source:      source,
		RetrievedAt: ts,
		PreviewPath: previewPath,
		ArchivePath: archivePath,
		SizeBytes:   int64(len(archive)),
	}
	metaPath := filepath.Join(dir, "metadata.json")
	raw, err := json.Marshal(meta)
	if err != nil {
		return CachedPreset{}, fmt.Errorf("marshal metadata: %w", err)
	}
	if err := os.WriteFile(metaPath, raw, 0o600); err != nil {
		logger.Log().WithError(err).WithField("meta_path", util.SanitizeForLog(metaPath)).Error("failed to write metadata file")
		return CachedPreset{}, fmt.Errorf("write metadata: %w", err)
	}

	logger.Log().WithField("slug", util.SanitizeForLog(cleanSlug)).WithField("cache_key", cacheKey).WithField("archive_path", util.SanitizeForLog(archivePath)).WithField("preview_path", util.SanitizeForLog(previewPath)).WithField("meta_path", util.SanitizeForLog(metaPath)).Info("preset successfully stored in cache")

	return meta, nil
}

// Load returns cached preset metadata, enforcing TTL.
func (c *HubCache) Load(ctx context.Context, slug string) (CachedPreset, error) {
	if err := ctx.Err(); err != nil {
		return CachedPreset{}, err
	}
	cleanSlug := sanitizeSlug(slug)
	if cleanSlug == "" {
		return CachedPreset{}, fmt.Errorf("invalid slug")
	}
	metaPath := filepath.Join(c.baseDir, cleanSlug, "metadata.json")
	logger.Log().WithField("slug", util.SanitizeForLog(cleanSlug)).WithField("meta_path", util.SanitizeForLog(metaPath)).Debug("attempting to load cached preset")

	data, err := os.ReadFile(metaPath) // #nosec G304 -- Reading cached preset metadata
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			logger.Log().WithField("slug", util.SanitizeForLog(cleanSlug)).WithField("meta_path", util.SanitizeForLog(metaPath)).Debug("preset not found in cache (cache miss)")
			return CachedPreset{}, ErrCacheMiss
		}
		logger.Log().WithError(err).WithField("slug", util.SanitizeForLog(cleanSlug)).WithField("meta_path", util.SanitizeForLog(metaPath)).Error("failed to read cached preset metadata")
		return CachedPreset{}, err
	}
	var meta CachedPreset
	if err := json.Unmarshal(data, &meta); err != nil {
		logger.Log().WithError(err).WithField("slug", util.SanitizeForLog(cleanSlug)).Error("failed to unmarshal cached preset metadata")
		return CachedPreset{}, fmt.Errorf("unmarshal metadata: %w", err)
	}

	if c.ttl > 0 && c.nowFn().After(meta.RetrievedAt.Add(c.ttl)) {
		logger.Log().WithField("slug", util.SanitizeForLog(cleanSlug)).WithField("retrieved_at", meta.RetrievedAt).WithField("ttl", c.ttl).Debug("cached preset expired")
		return CachedPreset{}, ErrCacheExpired
	}

	logger.Log().WithField("slug", util.SanitizeForLog(meta.Slug)).WithField("cache_key", meta.CacheKey).WithField("archive_path", util.SanitizeForLog(meta.ArchivePath)).Debug("successfully loaded cached preset")
	return meta, nil
}

// LoadPreview returns the preview contents for a cached preset.
func (c *HubCache) LoadPreview(ctx context.Context, slug string) (string, error) {
	meta, err := c.Load(ctx, slug)
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(meta.PreviewPath)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// List returns cached presets that have not expired.
func (c *HubCache) List(ctx context.Context) ([]CachedPreset, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	results := make([]CachedPreset, 0)
	err := filepath.WalkDir(c.baseDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() || d.Name() != "metadata.json" {
			return nil
		}
		rel, err := filepath.Rel(c.baseDir, path)
		if err != nil {
			return nil
		}
		slug := filepath.Dir(rel)
		meta, err := c.Load(ctx, slug)
		if err != nil {
			return nil
		}
		results = append(results, meta)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return results, nil
}

// Evict removes cached data for the given slug.
func (c *HubCache) Evict(ctx context.Context, slug string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	cleanSlug := sanitizeSlug(slug)
	if cleanSlug == "" {
		return fmt.Errorf("invalid slug")
	}
	return os.RemoveAll(filepath.Join(c.baseDir, cleanSlug))
}

// sanitizeSlug guards against traversal and unsupported characters.
func sanitizeSlug(slug string) string {
	trimmed := strings.TrimSpace(slug)
	if trimmed == "" {
		return ""
	}
	cleaned := filepath.Clean(trimmed)
	cleaned = strings.ReplaceAll(cleaned, "\\", "/")
	if strings.HasPrefix(cleaned, "..") || strings.Contains(cleaned, string(os.PathSeparator)+"..") || strings.HasPrefix(cleaned, string(os.PathSeparator)) {
		return ""
	}
	if !slugPattern.MatchString(cleaned) {
		return ""
	}
	return cleaned
}

// Exists returns true when a non-expired cache entry is present.
func (c *HubCache) Exists(ctx context.Context, slug string) bool {
	if _, err := c.Load(ctx, slug); err == nil {
		return true
	}
	return false
}

// Touch updates the timestamp to extend TTL; noop when missing.
func (c *HubCache) Touch(ctx context.Context, slug string) error {
	meta, err := c.Load(ctx, slug)
	if err != nil {
		return err
	}
	meta.RetrievedAt = c.nowFn().UTC()
	raw, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	metaPath := filepath.Join(c.baseDir, meta.Slug, "metadata.json")
	return os.WriteFile(metaPath, raw, 0o600)
}

// Size returns aggregated size of cached archives (best effort).
func (c *HubCache) Size(ctx context.Context) int64 {
	var total int64
	_ = filepath.WalkDir(c.baseDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		total += info.Size()
		return nil
	})
	return total
}
