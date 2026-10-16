package crowdsec

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/util"
)

var (
	ErrCacheMiss    = errors.New("cache miss")
	ErrCacheExpired = errors.New("cache expired")
	// ErrCacheInvalid marks a cache entry that failed validation; the entry is removed when this is returned.
	ErrCacheInvalid = errors.New("invalid cache entry")
	// ErrCacheLegacy marks an entry written before digests were recorded. It counts as a miss.
	ErrCacheLegacy = fmt.Errorf("%w: entry has no digest", ErrCacheMiss)
	// ErrInvalidSlug is returned for a slug that is not in owner/name form.
	ErrInvalidSlug = errors.New("invalid slug")
)

const (
	cacheItemFile     = "item.yaml"
	cacheMetaFile     = "metadata.json"
	cacheTempPrefix   = ".tmp-"
	maxCacheMetaBytes = 64 << 10
	maxCacheItemBytes = 1 << 20
	maxCacheTextBytes = 256
	maxCacheSlugBytes = 256
	// maxCacheClockSkew is how far retrieved_at may lie in the future.
	maxCacheClockSkew = 5 * time.Minute
)

// cacheItemTypes lists the hub item types a cache entry may carry.
var cacheItemTypes = []string{"collections", "parsers", "scenarios", "postoverflows", "contexts", "appsec-configs", "appsec-rules"}

var (
	// cacheSlugPattern is the hub item name form: exactly owner/name, case preserved because cscli names are
	// case-sensitive.
	cacheSlugPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._-]*/[A-Za-z0-9][A-Za-z0-9._-]*$`)
	sha256HexPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)
)

// IsCacheItemType reports whether t is an item type a cache entry may carry.
func IsCacheItemType(t string) bool { return slices.Contains(cacheItemTypes, t) }

// ValidCacheSlug reports whether slug is a well-formed hub item name usable as a cache entry key.
func ValidCacheSlug(slug string) bool {
	if len(slug) > maxCacheSlugBytes || !cacheSlugPattern.MatchString(slug) {
		return false
	}
	for part := range strings.SplitSeq(slug, "/") {
		if part == cacheMetaFile || part == cacheItemFile {
			return false
		}
	}
	return true
}

// CachedPreset captures metadata about a pulled preset. File locations are derived from the slug.
type CachedPreset struct {
	Slug        string    `json:"slug"`
	CacheKey    string    `json:"cache_key"`
	Etag        string    `json:"etag"`
	Source      string    `json:"source"`
	RetrievedAt time.Time `json:"retrieved_at"`
	Type        string    `json:"type"`
	Name        string    `json:"name"`
	ItemSHA256  string    `json:"item_sha256"`
	SizeBytes   int64     `json:"size_bytes"`
}

// HubCache persists pulled hub items on disk with TTL-based eviction. One instance must be shared by every
// caller that uses the same directory: the lock lives in the instance.
type HubCache struct {
	baseDir string
	ttl     time.Duration
	nowFn   func() time.Time
	mu      sync.RWMutex
}

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

// Store writes the raw item and its metadata and returns the cache metadata. The cache key carries the
// sha256 of the item, so two stores of different content never share a key. The item is written first
// and the metadata last, each through a temporary file and a rename.
func (c *HubCache) Store(ctx context.Context, slug, itemType, etag, source string, item []byte) (CachedPreset, error) {
	if err := ctx.Err(); err != nil {
		return CachedPreset{}, err
	}
	if !ValidCacheSlug(slug) {
		return CachedPreset{}, ErrInvalidSlug
	}
	if !IsCacheItemType(itemType) {
		return CachedPreset{}, fmt.Errorf("unsupported item type")
	}
	if len(item) == 0 || len(item) > maxCacheItemBytes {
		return CachedPreset{}, fmt.Errorf("item size out of range")
	}

	sum := sha256.Sum256(item)
	digest := hex.EncodeToString(sum[:])
	ts := c.nowFn().UTC()
	meta := CachedPreset{
		Slug:        slug,
		CacheKey:    fmt.Sprintf("%s-%d-%s", slug, ts.Unix(), digest),
		Etag:        cleanText(etag),
		Source:      cleanText(source),
		RetrievedAt: ts,
		Type:        itemType,
		Name:        slug,
		ItemSHA256:  digest,
		SizeBytes:   int64(len(item)),
	}
	raw, err := json.Marshal(meta)
	if err != nil {
		return CachedPreset{}, fmt.Errorf("marshal metadata: %w", err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	root, err := os.OpenRoot(c.baseDir)
	if err != nil {
		return CachedPreset{}, fmt.Errorf("open cache: %w", err)
	}
	defer closeQuietly(root)
	if err := root.MkdirAll(slug, 0o700); err != nil {
		return CachedPreset{}, fmt.Errorf("create cache entry dir: %w", withoutPath(err))
	}
	removeTempFiles(root, slug)
	if err := writeFileAtomic(root, path.Join(slug, cacheItemFile), item); err != nil {
		return CachedPreset{}, fmt.Errorf("write item: %w", err)
	}
	if err := writeFileAtomic(root, path.Join(slug, cacheMetaFile), raw); err != nil {
		return CachedPreset{}, fmt.Errorf("write metadata: %w", err)
	}
	logger.Log().WithField("slug", util.SanitizeForLog(slug)).WithField("cache_key", util.SanitizeForLog(meta.CacheKey)).Info("preset stored in cache")
	return meta, nil
}

// Load returns cached preset metadata, enforcing validation and TTL.
func (c *HubCache) Load(ctx context.Context, slug string) (CachedPreset, error) {
	meta, _, err := c.LoadItem(ctx, slug)
	return meta, err
}

// LoadPreview returns the item contents of a cached preset.
func (c *HubCache) LoadPreview(ctx context.Context, slug string) (string, error) {
	_, item, err := c.LoadItem(ctx, slug)
	if err != nil {
		return "", err
	}
	return string(item), nil
}

// LoadItem returns the metadata and the raw item of a valid, unexpired entry. An invalid or legacy entry is
// removed (after it is read again under the write lock) and reported as ErrCacheInvalid or ErrCacheLegacy.
func (c *HubCache) LoadItem(ctx context.Context, slug string) (CachedPreset, []byte, error) {
	if err := ctx.Err(); err != nil {
		return CachedPreset{}, nil, err
	}
	if !ValidCacheSlug(slug) {
		return CachedPreset{}, nil, ErrInvalidSlug
	}
	c.mu.RLock()
	meta, item, hasTemp, err := c.readEntry(slug)
	c.mu.RUnlock()
	if hasTemp {
		c.sweepTemp(slug)
	}
	if errors.Is(err, ErrCacheInvalid) || errors.Is(err, ErrCacheLegacy) {
		meta, item, err = c.evictInvalid(slug)
	}
	if err != nil {
		return CachedPreset{}, nil, err
	}
	if c.ttl > 0 && c.nowFn().After(meta.RetrievedAt.Add(c.ttl)) {
		return CachedPreset{}, nil, ErrCacheExpired
	}
	return meta, item, nil
}

// List returns cached presets that are valid and have not expired. Entries that fail validation are
// removed, as a load of them would do.
func (c *HubCache) List(ctx context.Context) ([]CachedPreset, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	results := make([]CachedPreset, 0)
	for _, slug := range c.entrySlugs() {
		if ctx.Err() != nil {
			break
		}
		meta, err := c.Load(ctx, slug)
		if err != nil {
			continue
		}
		results = append(results, meta)
	}
	return results, ctx.Err()
}

// entrySlugs lists the owner/name directories that could hold an entry.
func (c *HubCache) entrySlugs() []string {
	root, err := os.OpenRoot(c.baseDir)
	if err != nil {
		return nil
	}
	defer closeQuietly(root)
	var slugs []string
	owners, _ := fs.ReadDir(root.FS(), ".")
	for _, owner := range owners {
		if !owner.IsDir() {
			continue
		}
		names, _ := fs.ReadDir(root.FS(), owner.Name())
		for _, name := range names {
			if slug := owner.Name() + "/" + name.Name(); ValidCacheSlug(slug) {
				slugs = append(slugs, slug)
			}
		}
	}
	return slugs
}

// Evict removes cached data for the given slug.
func (c *HubCache) Evict(ctx context.Context, slug string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !ValidCacheSlug(slug) {
		return ErrInvalidSlug
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.removeEntry(slug)
}

// EvictIfKey removes the entry only when it still carries cacheKey (or no longer validates). An entry that
// a concurrent pull replaced in the meantime is left alone.
func (c *HubCache) EvictIfKey(ctx context.Context, slug, cacheKey string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if !ValidCacheSlug(slug) {
		return ErrInvalidSlug
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	meta, _, _, err := c.readEntry(slug)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil
	case err == nil && meta.CacheKey != cacheKey:
		return nil
	}
	return c.removeEntry(slug)
}

// evictInvalid removes an entry that failed validation, but only after reading it again under the write
// lock: a good entry written in the meantime must survive. It returns the result of that second read.
func (c *HubCache) evictInvalid(slug string) (CachedPreset, []byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	meta, item, _, err := c.readEntry(slug)
	if errors.Is(err, ErrCacheInvalid) || errors.Is(err, ErrCacheLegacy) {
		logger.Log().WithField("slug", util.SanitizeForLog(slug)).Info("removing unusable cache entry")
		if rmErr := c.removeEntry(slug); rmErr != nil {
			logger.Log().WithError(rmErr).WithField("slug", util.SanitizeForLog(slug)).Warn("failed to remove unusable cache entry")
		}
	}
	return meta, item, err
}

// sweepTemp removes temporary files a crashed store left in the entry directory.
func (c *HubCache) sweepTemp(slug string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	root, err := os.OpenRoot(c.baseDir)
	if err != nil {
		return
	}
	defer closeQuietly(root)
	removeTempFiles(root, slug)
}

// removeEntry deletes the entry directory. A link in place of the owner or entry directory is removed as
// a link; its target is never touched. The caller holds the write lock.
func (c *HubCache) removeEntry(slug string) error {
	root, err := os.OpenRoot(c.baseDir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open cache: %w", withoutPath(err))
	}
	defer closeQuietly(root)
	owner := path.Dir(slug)
	for _, p := range []string{owner, slug} {
		info, lerr := root.Lstat(p)
		if errors.Is(lerr, fs.ErrNotExist) {
			return nil
		}
		if lerr != nil {
			return fmt.Errorf("inspect cache entry: %w", withoutPath(lerr))
		}
		if info.Mode()&fs.ModeSymlink != 0 {
			if rmErr := root.Remove(p); rmErr != nil && !errors.Is(rmErr, fs.ErrNotExist) {
				return fmt.Errorf("remove cache entry: %w", withoutPath(rmErr))
			}
			return nil
		}
	}
	if err := root.RemoveAll(slug); err != nil {
		return fmt.Errorf("remove cache entry: %w", withoutPath(err))
	}
	_ = root.Remove(owner) // only succeeds when no other entry lives below it
	return nil
}

// readEntry reads and validates the entry. The caller holds the lock. A missing entry yields an error that
// satisfies errors.Is(fs.ErrNotExist) and ErrCacheMiss; hasTemp reports leftover temporary files.
func (c *HubCache) readEntry(slug string) (meta CachedPreset, item []byte, hasTemp bool, err error) {
	root, err := os.OpenRoot(c.baseDir)
	if err != nil {
		return CachedPreset{}, nil, false, fmt.Errorf("%w: %w", ErrCacheMiss, fs.ErrNotExist)
	}
	defer closeQuietly(root)

	entries, readErr := fs.ReadDir(root.FS(), slug)
	if readErr != nil && !errors.Is(readErr, fs.ErrNotExist) {
		return CachedPreset{}, nil, false, invalidEntry("entry directory")
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), cacheTempPrefix) {
			hasTemp = true
		}
	}

	raw, err := readCapped(root, path.Join(slug, cacheMetaFile), maxCacheMetaBytes)
	if errors.Is(err, fs.ErrNotExist) {
		return CachedPreset{}, nil, hasTemp, fmt.Errorf("%w: %w", ErrCacheMiss, fs.ErrNotExist)
	}
	if err != nil {
		return CachedPreset{}, nil, hasTemp, err
	}
	if jsonErr := json.Unmarshal(raw, &meta); jsonErr != nil {
		return CachedPreset{}, nil, hasTemp, invalidEntry("metadata")
	}
	if meta.Slug != slug {
		return CachedPreset{}, nil, hasTemp, invalidEntry("slug")
	}
	if meta.ItemSHA256 == "" {
		return CachedPreset{}, nil, hasTemp, ErrCacheLegacy
	}
	item, err = readCapped(root, path.Join(slug, cacheItemFile), maxCacheItemBytes)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return CachedPreset{}, nil, hasTemp, invalidEntry("item")
		}
		return CachedPreset{}, nil, hasTemp, err
	}
	if err := c.validateMeta(meta, item); err != nil {
		return CachedPreset{}, nil, hasTemp, err
	}
	return meta, item, hasTemp, nil
}

// validateMeta checks the stored metadata against the entry slug and the item that was read.
func (c *HubCache) validateMeta(meta CachedPreset, item []byte) error {
	sum := sha256.Sum256(item)
	digest := hex.EncodeToString(sum[:])
	switch {
	case meta.Name != meta.Slug:
		return invalidEntry("name")
	case !validCacheKey(meta.CacheKey, meta.Slug, meta.ItemSHA256):
		return invalidEntry("cache key")
	case !sha256HexPattern.MatchString(meta.ItemSHA256) || meta.ItemSHA256 != digest:
		return invalidEntry("digest")
	case meta.SizeBytes != int64(len(item)):
		return invalidEntry("size")
	case !IsCacheItemType(meta.Type):
		return invalidEntry("item type")
	case !printableText(meta.Etag) || !printableText(meta.Source):
		return invalidEntry("text field")
	case meta.RetrievedAt.IsZero() || meta.RetrievedAt.After(c.nowFn().Add(maxCacheClockSkew)):
		return invalidEntry("retrieval time")
	}
	return nil
}

func invalidEntry(what string) error {
	return fmt.Errorf("%w: %s", ErrCacheInvalid, what)
}

func printableText(s string) bool {
	return len(s) <= maxCacheTextBytes && !strings.ContainsFunc(s, func(r rune) bool { return !unicode.IsPrint(r) })
}

// readCapped reads a regular file below root (a path relative to it) with a size cap. The file type is
// checked on the opened descriptor.
func readCapped(root *os.Root, rel string, limit int64) ([]byte, error) {
	f, err := root.OpenFile(rel, cacheOpenFlags, 0)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return nil, err
		}
		return nil, invalidEntry("unreadable file")
	}
	defer closeQuietly(f)
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() {
		return nil, invalidEntry("unexpected file type")
	}
	data, err := io.ReadAll(io.LimitReader(f, limit+1))
	if err != nil {
		return nil, invalidEntry("unreadable file")
	}
	if int64(len(data)) > limit {
		return nil, invalidEntry("file too large")
	}
	return data, nil
}

// writeFileAtomic writes data to rel below root through a temporary file in the same directory.
func writeFileAtomic(root *os.Root, rel string, data []byte) (err error) {
	var suffix [8]byte
	if _, err = rand.Read(suffix[:]); err != nil {
		return fmt.Errorf("temp name: %w", err)
	}
	tmp := path.Join(path.Dir(rel), cacheTempPrefix+hex.EncodeToString(suffix[:]))
	f, err := root.OpenFile(tmp, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0o600)
	if err != nil {
		return withoutPath(err)
	}
	defer func() {
		if err != nil {
			_ = root.Remove(tmp)
		}
	}()
	if _, err = f.Write(data); err != nil {
		_ = f.Close()
		return withoutPath(err)
	}
	if err = f.Sync(); err != nil {
		_ = f.Close()
		return withoutPath(err)
	}
	if err = f.Close(); err != nil {
		return withoutPath(err)
	}
	return withoutPath(root.Rename(tmp, rel))
}

// removeTempFiles deletes leftover temporary files in the entry directory.
func removeTempFiles(root *os.Root, slug string) {
	entries, err := fs.ReadDir(root.FS(), slug)
	if err != nil {
		return
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), cacheTempPrefix) {
			_ = root.Remove(path.Join(slug, e.Name()))
		}
	}
}

func closeQuietly(c io.Closer) {
	if err := c.Close(); err != nil {
		logger.Log().WithError(err).Debug("close failed")
	}
}

// cleanText drops unprintable characters and cuts s to the stored length limit.
func cleanText(s string) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsPrint(r) {
			return r
		}
		return -1
	}, s)
	if len(s) > maxCacheTextBytes {
		s = strings.ToValidUTF8(s[:maxCacheTextBytes], "")
	}
	return s
}

// validCacheKey reports whether key has the form "<slug>-<unix time>-<item sha256>" for the given slug and digest.
func validCacheKey(key, slug, digest string) bool {
	rest, ok := strings.CutPrefix(key, slug+"-")
	if !ok {
		return false
	}
	stamp, hash, ok := strings.Cut(rest, "-")
	if !ok || stamp == "" || hash != digest || !sha256HexPattern.MatchString(hash) {
		return false
	}
	for _, r := range stamp {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
