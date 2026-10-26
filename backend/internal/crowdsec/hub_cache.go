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
	// ErrInvalidItemType is returned for an item type that is not a hub section.
	ErrInvalidItemType = errors.New("invalid item type")
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

// itemDigest returns the lower-case hex sha256 of a cached or downloaded hub item. It is the value the hub index
// publishes per version and the value cache metadata records.
func itemDigest(item []byte) string {
	sum := sha256.Sum256(item)
	return hex.EncodeToString(sum[:])
}

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

// CachedPreset captures metadata about a pulled preset. File locations are derived from the type and the slug.
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

// HubCache persists pulled hub items on disk with TTL-based eviction. An entry lives at
// <type>/<owner>/<name>/ because one hub name can exist in several sections. One instance must be shared by
// every caller that uses the same directory: the lock lives in the instance.
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

// checkEntryKey validates the (type, slug) pair that addresses an entry.
func checkEntryKey(itemType, slug string) error {
	if !IsCacheItemType(itemType) {
		return ErrInvalidItemType
	}
	if !ValidCacheSlug(slug) {
		return ErrInvalidSlug
	}
	return nil
}

// Store writes the raw item and its metadata and returns the cache metadata. The cache key carries the
// sha256 of the item, so two stores of different content never share a key. The item is written first
// and the metadata last, each through a temporary file and a rename.
func (c *HubCache) Store(ctx context.Context, slug, itemType, etag, source string, item []byte) (CachedPreset, error) {
	if err := ctx.Err(); err != nil {
		return CachedPreset{}, err
	}
	if err := checkEntryKey(itemType, slug); err != nil {
		return CachedPreset{}, err
	}
	if len(item) == 0 || len(item) > maxCacheItemBytes {
		return CachedPreset{}, fmt.Errorf("item size out of range")
	}

	digest := itemDigest(item)
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

	dir := entryDir(itemType, slug)
	c.mu.Lock()
	defer c.mu.Unlock()
	root, err := os.OpenRoot(c.baseDir)
	if err != nil {
		return CachedPreset{}, fmt.Errorf("open cache: %w", err)
	}
	defer closeQuietly(root)
	if err := root.MkdirAll(dir, 0o700); err != nil {
		return CachedPreset{}, fmt.Errorf("create cache entry dir: %w", withoutPath(err))
	}
	removeTempFiles(root, dir)
	if err := writeFileAtomic(root, path.Join(dir, cacheItemFile), item); err != nil {
		return CachedPreset{}, fmt.Errorf("write item: %w", err)
	}
	if err := writeFileAtomic(root, path.Join(dir, cacheMetaFile), raw); err != nil {
		return CachedPreset{}, fmt.Errorf("write metadata: %w", err)
	}
	logger.Log().WithField("slug", util.SanitizeForLog(slug)).WithField("type", util.SanitizeForLog(itemType)).WithField("cache_key", util.SanitizeForLog(meta.CacheKey)).Info("preset stored in cache")
	return meta, nil
}

// entryDir is the path of an entry below the cache root.
func entryDir(itemType, slug string) string { return path.Join(itemType, slug) }

// Load returns cached preset metadata, enforcing validation and TTL.
func (c *HubCache) Load(ctx context.Context, itemType, slug string) (CachedPreset, error) {
	meta, _, err := c.LoadItem(ctx, itemType, slug)
	return meta, err
}

// LoadPreview returns the item contents of a cached preset.
func (c *HubCache) LoadPreview(ctx context.Context, itemType, slug string) (string, error) {
	_, item, err := c.LoadItem(ctx, itemType, slug)
	if err != nil {
		return "", err
	}
	return string(item), nil
}

// LoadItem returns the metadata and the raw item of a valid, unexpired entry. An invalid or legacy entry is
// removed (after it is read again under the write lock) and reported as ErrCacheInvalid or ErrCacheLegacy.
func (c *HubCache) LoadItem(ctx context.Context, itemType, slug string) (CachedPreset, []byte, error) {
	if err := ctx.Err(); err != nil {
		return CachedPreset{}, nil, err
	}
	if err := checkEntryKey(itemType, slug); err != nil {
		return CachedPreset{}, nil, err
	}
	c.mu.RLock()
	meta, item, hasTemp, err := c.readEntry(itemType, slug)
	c.mu.RUnlock()
	if hasTemp {
		c.sweepTemp(itemType, slug)
	}
	if errors.Is(err, ErrCacheInvalid) || errors.Is(err, ErrCacheLegacy) {
		meta, item, err = c.evictInvalid(itemType, slug)
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
	for _, key := range c.entryKeys() {
		if ctx.Err() != nil {
			break
		}
		meta, err := c.Load(ctx, key.itemType, key.slug)
		if err != nil {
			continue
		}
		results = append(results, meta)
	}
	return results, ctx.Err()
}

// ListSlug returns the valid, unexpired entries of one slug: one per hub section that holds it.
func (c *HubCache) ListSlug(ctx context.Context, slug string) ([]CachedPreset, error) {
	if !ValidCacheSlug(slug) {
		return nil, ErrInvalidSlug
	}
	var found []CachedPreset
	for _, itemType := range cacheItemTypes {
		if meta, err := c.Load(ctx, itemType, slug); err == nil {
			found = append(found, meta)
		} else if ctx.Err() != nil {
			return nil, ctx.Err()
		}
	}
	return found, nil
}

// entryKey addresses one cache entry.
type entryKey struct{ itemType, slug string }

// entryKeys lists the <type>/<owner>/<name> directories that could hold an entry.
func (c *HubCache) entryKeys() []entryKey {
	root, err := os.OpenRoot(c.baseDir)
	if err != nil {
		return nil
	}
	defer closeQuietly(root)
	var keys []entryKey
	for _, itemType := range cacheItemTypes {
		owners, _ := fs.ReadDir(root.FS(), itemType)
		for _, owner := range owners {
			if !owner.IsDir() {
				continue
			}
			names, _ := fs.ReadDir(root.FS(), path.Join(itemType, owner.Name()))
			for _, name := range names {
				if slug := owner.Name() + "/" + name.Name(); ValidCacheSlug(slug) {
					keys = append(keys, entryKey{itemType, slug})
				}
			}
		}
	}
	return keys
}

// RemoveLegacyEntries deletes everything below the cache root that is not a type directory. Entries written
// before the cache was keyed by type live at <owner>/<name>/ and are never served; the cache is regenerable.
func (c *HubCache) RemoveLegacyEntries(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	root, err := os.OpenRoot(c.baseDir)
	if err != nil {
		return fmt.Errorf("open cache: %w", withoutPath(err))
	}
	defer closeQuietly(root)
	top, err := fs.ReadDir(root.FS(), ".")
	if err != nil {
		return fmt.Errorf("list cache: %w", withoutPath(err))
	}
	var errs []error
	for _, e := range top {
		if IsCacheItemType(e.Name()) && e.IsDir() {
			continue
		}
		if rmErr := root.RemoveAll(e.Name()); rmErr != nil {
			errs = append(errs, fmt.Errorf("remove legacy cache entry: %w", withoutPath(rmErr)))
			continue
		}
		logger.Log().WithField("entry", util.SanitizeForLog(e.Name())).Info("removed cache entry of the old layout")
	}
	return errors.Join(errs...)
}

// Evict removes cached data for the given entry.
func (c *HubCache) Evict(ctx context.Context, itemType, slug string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := checkEntryKey(itemType, slug); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.removeEntry(itemType, slug)
}

// EvictIfKey removes the entry only when it still carries cacheKey (or no longer validates). An entry that
// a concurrent pull replaced in the meantime is left alone.
func (c *HubCache) EvictIfKey(ctx context.Context, itemType, slug, cacheKey string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := checkEntryKey(itemType, slug); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	meta, _, _, err := c.readEntry(itemType, slug)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return nil
	case err == nil && meta.CacheKey != cacheKey:
		return nil
	}
	return c.removeEntry(itemType, slug)
}

// evictInvalid removes an entry that failed validation, but only after reading it again under the write
// lock: a good entry written in the meantime must survive. It returns the result of that second read.
func (c *HubCache) evictInvalid(itemType, slug string) (CachedPreset, []byte, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	meta, item, _, err := c.readEntry(itemType, slug)
	if errors.Is(err, ErrCacheInvalid) || errors.Is(err, ErrCacheLegacy) {
		logger.Log().WithField("slug", util.SanitizeForLog(slug)).WithField("type", util.SanitizeForLog(itemType)).Info("removing unusable cache entry")
		if rmErr := c.removeEntry(itemType, slug); rmErr != nil {
			logger.Log().WithError(rmErr).WithField("slug", util.SanitizeForLog(slug)).Warn("failed to remove unusable cache entry")
		}
	}
	return meta, item, err
}

// sweepTemp removes temporary files a crashed store left in the entry directory.
func (c *HubCache) sweepTemp(itemType, slug string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	root, err := os.OpenRoot(c.baseDir)
	if err != nil {
		return
	}
	defer closeQuietly(root)
	removeTempFiles(root, entryDir(itemType, slug))
}

// removeEntry deletes the entry directory. A link in place of the type, owner or entry directory is removed
// as a link; its target is never touched. The caller holds the write lock.
func (c *HubCache) removeEntry(itemType, slug string) error {
	root, err := os.OpenRoot(c.baseDir)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open cache: %w", withoutPath(err))
	}
	defer closeQuietly(root)
	dir := entryDir(itemType, slug)
	owner := path.Dir(dir)
	for _, p := range []string{itemType, owner, dir} {
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
	if err := root.RemoveAll(dir); err != nil {
		return fmt.Errorf("remove cache entry: %w", withoutPath(err))
	}
	// Each removal only succeeds when nothing else lives below the directory.
	_ = root.Remove(owner)
	_ = root.Remove(itemType)
	return nil
}

// readEntry reads and validates the entry. The caller holds the lock. A missing entry yields an error that
// satisfies errors.Is(fs.ErrNotExist) and ErrCacheMiss; hasTemp reports leftover temporary files.
func (c *HubCache) readEntry(itemType, slug string) (meta CachedPreset, item []byte, hasTemp bool, err error) {
	root, err := os.OpenRoot(c.baseDir)
	if err != nil {
		return CachedPreset{}, nil, false, fmt.Errorf("%w: %w", ErrCacheMiss, fs.ErrNotExist)
	}
	defer closeQuietly(root)

	dir := entryDir(itemType, slug)
	entries, readErr := fs.ReadDir(root.FS(), dir)
	if readErr != nil && !errors.Is(readErr, fs.ErrNotExist) {
		return CachedPreset{}, nil, false, invalidEntry("entry directory")
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), cacheTempPrefix) {
			hasTemp = true
		}
	}

	raw, err := readCapped(root, path.Join(dir, cacheMetaFile), maxCacheMetaBytes)
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
	item, err = readCapped(root, path.Join(dir, cacheItemFile), maxCacheItemBytes)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return CachedPreset{}, nil, hasTemp, invalidEntry("item")
		}
		return CachedPreset{}, nil, hasTemp, err
	}
	if err := c.validateMeta(meta, itemType, item); err != nil {
		return CachedPreset{}, nil, hasTemp, err
	}
	return meta, item, hasTemp, nil
}

// validateMeta checks the stored metadata against the entry address and the item that was read.
func (c *HubCache) validateMeta(meta CachedPreset, itemType string, item []byte) error {
	digest := itemDigest(item)
	switch {
	case meta.Name != meta.Slug:
		return invalidEntry("name")
	case !validCacheKey(meta.CacheKey, meta.Slug, meta.ItemSHA256):
		return invalidEntry("cache key")
	case !sha256HexPattern.MatchString(meta.ItemSHA256) || meta.ItemSHA256 != digest:
		return invalidEntry("digest")
	case meta.SizeBytes != int64(len(item)):
		return invalidEntry("size")
	case meta.Type != itemType:
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
