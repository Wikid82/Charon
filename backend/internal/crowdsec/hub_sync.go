package crowdsec

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	neturl "net/url"
	"os"
	"slices"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/network"
	"github.com/Wikid82/charon/backend/internal/util"

	"gopkg.in/yaml.v3"
)

// CommandExecutor defines the minimal command execution interface we need for cscli calls.
type CommandExecutor interface {
	Execute(ctx context.Context, name string, args ...string) ([]byte, error)
}

const (
	// cdnHubHost is the CrowdSec CDN that cscli downloads the hub from. Its master branch serves the index as
	// .index.json and the items under the same base, so the default base is the branch root.
	cdnHubHost = "cdn-hub.crowdsec.net"
	// legacyHubDataHost is the former default hub. Its index path now answers 404, so it is no longer a default. It
	// stays on the host allowlist so that an operator-set HUB_BASE_URL on it keeps working.
	legacyHubDataHost       = "hub-data.crowdsec.net"
	defaultHubBaseURL       = "https://" + cdnHubHost + "/crowdsecurity/master"
	defaultHubMirrorBaseURL = "https://raw.githubusercontent.com/crowdsecurity/hub/master"
	// defaultHubIndexPath is the index path of an operator-set JSON hub base that is not the CDN.
	defaultHubIndexPath = "/api/index.json"
	maxArchiveSize      = int64(25 * 1024 * 1024) // 25MiB safety cap for the hub index
	defaultPullTimeout  = 25 * time.Second
	// indexFreshTTL is how long after a successful index fetch an untyped pull of a single cached section
	// skips the index request.
	indexFreshTTL = 10 * time.Minute
	// defaultApplyTimeout covers cscli version, hub update, the typed install (which downloads the item and its
	// dependencies), inspect, an optional upgrade with a second inspect, and one hub list. Timed against a real
	// hub: typical installs take 1-5 s, a collection with about 200 dependencies about 22 s, and the largest
	// collections (hundreds of scenarios) take minutes and fail safely with a rollback. HUB_APPLY_TIMEOUT_SECONDS
	// overrides it.
	defaultApplyTimeout = 120 * time.Second
	// defaultHubUpdateTimeout bounds the best-effort hub update so a slow hub cannot use up the install's time.
	defaultHubUpdateTimeout = 20 * time.Second
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
	// Digest is the hub's lower-case sha256 of the item file for the indexed version. It is empty when the index
	// has no record for that version, and it is never part of an API response.
	Digest string `json:"-"`
}

// HubIndex is a small wrapper for hub listing payloads.
type HubIndex struct {
	Items []HubIndexEntry `json:"items"`
	// offline marks an index listed from the local cscli data because the HTTP index was unreachable.
	offline bool
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
	// Type is the hub section of the installed item; empty for a curated bundle.
	Type string `json:"type,omitempty"`
	// Warnings lists non-fatal findings, such as a dependency that was edited on this server.
	Warnings []string `json:"warnings,omitempty"`
}

// HubService coordinates hub pulls, caching, and apply operations.
type HubService struct {
	Exec          CommandExecutor
	Reload        ReloadFunc // optional; reloads the managed CrowdSec process after hub changes
	Cache         *HubCache
	DataDir       string
	HTTPClient    *http.Client
	HubBaseURL    string
	MirrorBaseURL string
	PullTimeout   time.Duration
	ApplyTimeout  time.Duration
	// HubUpdateTimeout bounds the best-effort hub update inside Apply.
	HubUpdateTimeout time.Duration

	// validateURL overrides validateHubURL when non-nil. Test-only: production
	// code never sets it, so every hub request goes through validateHubURL.
	validateURL func(string) error

	// mu serializes Apply and ApplyCurated so backup/rollback cycles never interleave.
	mu sync.Mutex

	// indexMu guards lastIndex and lastIndexAt.
	indexMu sync.Mutex
	// lastIndex is the index of the last successful fetch, taken at lastIndexAt.
	lastIndex   HubIndex
	lastIndexAt time.Time
	// now is the clock for the index window; nil means time.Now. Test-only override.
	now func() time.Time
	// legacySweepMu guards legacySweepDone, which is set once the old-layout cache sweep has succeeded.
	legacySweepMu   sync.Mutex
	legacySweepDone bool
}

// errHubURLUnparseable is the cause when net/url rejects a hub URL. The parser error quotes the whole input, which
// may carry operator credentials, so the message stays fixed and callers can match it with errors.Is.
var errHubURLUnparseable = errors.New("URL cannot be parsed (check HUB_BASE_URL for unescaped characters)")

// validateHubURL validates a hub URL for security (SSRF protection - HIGH-001).
// This function prevents Server-Side Request Forgery by:
//  1. Enforcing HTTPS for production hub URLs
//  2. Allowlisting known CrowdSec hub domains
//
// Returns: error if URL is invalid or not allowlisted
func validateHubURL(rawURL string) error {
	parsed, err := neturl.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("invalid URL format: %w", errHubURLUnparseable)
	}

	// Only allow http/https schemes at parse level; https is enforced below
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("unsupported scheme: %s (only http and https are allowed)", parsed.Scheme)
	}

	host := parsed.Hostname()
	if host == "" {
		return fmt.Errorf("missing hostname in URL")
	}

	// Hub URLs must be HTTPS
	if parsed.Scheme != "https" {
		return fmt.Errorf("hub URLs must use HTTPS (got: %s)", parsed.Scheme)
	}

	if !slices.Contains(allowedHubHosts, host) {
		return fmt.Errorf("unknown hub domain: %s (allowed: %s)", host, strings.Join(allowedHubHosts, ", "))
	}

	return nil
}

// allowedHubHosts are the only hosts a hub URL may name: the CDN default, the legacy hub host (for operator-set
// URLs), the hub host and the GitHub raw mirror. The match is on the exact hostname.
var allowedHubHosts = []string{
	cdnHubHost,
	legacyHubDataHost,
	"hub.crowdsec.net",
	"raw.githubusercontent.com", // GitHub raw content (CrowdSec mirror)
}

// checkURL validates target with the test override when set, otherwise with
// the production validateHubURL.
func (s *HubService) checkURL(target string) error {
	if s.validateURL != nil {
		return s.validateURL(target)
	}
	return validateHubURL(target)
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

		HubUpdateTimeout: defaultHubUpdateTimeout,
	}
}

// newHubHTTPClient creates an SSRF-safe HTTP client for hub operations.
// Hub URLs are validated by validateHubURL() which:
// - Enforces HTTPS for production
// - Allowlists known CrowdSec domains (see allowedHubHosts)
// - Blocks loopback and private targets at the dial layer
// Using network.NewSafeHTTPClient provides defense-in-depth at the connection level.
func newHubHTTPClient(timeout time.Duration) *http.Client {
	opts := []network.Option{
		network.WithTimeout(timeout),
	}
	return network.NewSafeHTTPClient(opts...)
}

func normalizeHubBaseURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return defaultHubBaseURL
	}
	return strings.TrimRight(trimmed, "/")
}

// HubURLForLog returns a hub URL with its userinfo, query and fragment removed, so that a URL can be logged or
// returned in an error without carrying credentials an operator put in HUB_BASE_URL.
func HubURLForLog(raw string) string {
	parsed, err := neturl.Parse(raw)
	if err != nil {
		return "<invalid hub URL>"
	}
	parsed.User = nil
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

// transportCause drops the request URL that net/http embeds in a *url.Error, keeping the underlying cause. Callers
// name the URL with HubURLForLog instead, so the error never shows the URL with its userinfo.
func transportCause(err error) error {
	for {
		var urlErr *neturl.Error
		if !errors.As(err, &urlErr) {
			return err
		}
		err = urlErr.Err
	}
}

// hubBaseCandidates lists the hub bases to try, in a fixed order: an operator-set HUB_BASE_URL first, then the CDN
// default, then the GitHub mirror (the configured one, then the built-in one). The legacy hub-data host is not a
// default, so it is only tried when an operator names it.
func (s *HubService) hubBaseCandidates() []string {
	var candidates []string
	if s.HubBaseURL != defaultHubBaseURL {
		candidates = append(candidates, s.HubBaseURL)
	}
	candidates = append(candidates, defaultHubBaseURL, s.MirrorBaseURL, defaultHubMirrorBaseURL)
	return uniqueStrings(candidates)
}

// isCDNHubBase reports whether base is on the CrowdSec CDN host, whose index is the .index.json file under the base.
func isCDNHubBase(base string) bool {
	parsed, err := neturl.Parse(base)
	return err == nil && parsed.Hostname() == cdnHubHost
}

func buildIndexURL(base string) string {
	normalized := normalizeHubBaseURL(base)
	if strings.HasSuffix(strings.ToLower(normalized), ".json") {
		return normalized
	}
	if isCDNHubBase(normalized) {
		return normalized + "/.index.json"
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

// FetchIndex downloads the full hub index over HTTP. Only when no hub endpoint answers does it fall back
// to the index cscli holds locally (`cscli hub list -a`), which lists every item but carries no download
// URLs, so it serves listing only.
func (s *HubService) FetchIndex(ctx context.Context) (HubIndex, error) {
	idx, httpErr := s.fetchIndexHTTP(ctx)
	if httpErr == nil {
		return idx, nil
	}
	if s.Exec == nil {
		return HubIndex{}, httpErr
	}
	idx, cscliErr := s.fetchIndexCSCLI(ctx)
	if cscliErr != nil {
		failure := fmt.Errorf("%w; cscli fallback failed: %v", httpErr, cscliErr)
		if ctx.Err() != nil {
			return HubIndex{}, failure
		}
		// Neither the hub nor the local index answered; callers show a fixed message and the detail stays in the log.
		return HubIndex{}, fmt.Errorf("%w: %w", ErrHubUnreachable, failure)
	}
	logger.Log().WithError(httpErr).Debug("hub index unreachable, listing from the local cscli index")
	idx.offline = true
	return idx, nil
}

func (s *HubService) fetchIndexCSCLI(ctx context.Context) (HubIndex, error) {
	cmdCtx, cancel := context.WithTimeout(ctx, s.PullTimeout)
	defer cancel()

	output, err := s.runCSCLI(cmdCtx, "hub", "list", "-a", "-o", "json")
	if err != nil {
		return HubIndex{}, err
	}
	return parseCSCLIIndex(output)
}

func parseCSCLIIndex(raw []byte) (HubIndex, error) {
	bucket := map[string][]map[string]any{}
	if err := unmarshalCSCLIJSON(raw, &bucket); err != nil {
		return HubIndex{}, fmt.Errorf("parse cscli index: %w", err)
	}
	items := make([]HubIndexEntry, 0)
	dropped := 0
	for section, list := range bucket {
		for _, obj := range list {
			name := strings.TrimSpace(asString(obj["name"]))
			if !ValidCacheSlug(name) {
				dropped++
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
	logDroppedIndexNames("cscli", dropped)
	if len(items) == 0 {
		return HubIndex{}, fmt.Errorf("empty cscli index")
	}
	return HubIndex{Items: items}, nil
}

// validIndexPath reports whether an index item path is safe to append to the hub base URL: a relative path
// without a scheme, a leading slash, backslashes, control characters, ".." segments, or any of "%", "?" and "#".
// Rejecting "%" outright also rejects percent-encoded dot segments (%2e%2e) and encoded separators, which a hub
// index has no use for.
func validIndexPath(p string) bool {
	if strings.HasPrefix(p, "/") || strings.ContainsAny(p, "\\%?#") || strings.Contains(p, "://") {
		return false
	}
	for _, r := range p {
		if r < 0x20 || r == 0x7f {
			return false
		}
	}
	if u, err := neturl.Parse(p); err != nil || u.Scheme != "" || u.Host != "" {
		return false
	}
	return !slices.Contains(strings.Split(p, "/"), "..")
}

// rawIndexItem is one item record of the raw hub index. Versions maps each published version to its record,
// which carries the digest of that version's item file.
type rawIndexItem struct {
	Path        string `json:"path"`
	Version     string `json:"version"`
	Description string `json:"description"`
	Versions    map[string]struct {
		Digest string `json:"digest"`
	} `json:"versions"`
}

func parseRawIndex(raw []byte, baseURL string) (HubIndex, error) {
	bucket := map[string]map[string]rawIndexItem{}
	if err := json.Unmarshal(raw, &bucket); err != nil {
		return HubIndex{}, fmt.Errorf("parse raw index: %w", err)
	}

	items := make([]HubIndexEntry, 0)
	dropped := 0
	for section, list := range bucket {
		for name, obj := range list {
			cleanName := strings.TrimSpace(name)
			if !ValidCacheSlug(cleanName) {
				dropped++
				continue
			}

			if !validIndexPath(obj.Path) {
				dropped++
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
				Digest:      strings.ToLower(strings.TrimSpace(obj.Versions[obj.Version].Digest)),
			}
			items = append(items, entry)
		}
	}
	logDroppedIndexNames("raw", dropped)
	if len(items) == 0 {
		return HubIndex{}, fmt.Errorf("empty raw index")
	}
	return HubIndex{Items: items}, nil
}

// logDroppedIndexNames records how many index names were skipped as malformed; the names themselves are not logged.
func logDroppedIndexNames(source string, dropped int) {
	if dropped > 0 {
		logger.Log().WithField("source", source).WithField("dropped", dropped).Debug("hub index entries with unusable names skipped")
	}
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
			logger.Log().WithField("hub_index", HubURLForLog(target)).WithField("fallback_used", attempt > 0).Info("hub index fetched")
			return idx, nil
		}
		errs = append(errs, candidateError(target, err))
		// A caller that cancelled or whose deadline passed has no use for the remaining candidates.
		if ctx.Err() != nil {
			break
		}
		if e, ok := err.(interface{ CanFallback() bool }); ok && e.CanFallback() {
			logger.Log().WithField("hub_index", HubURLForLog(target)).WithField("attempt", attempt+1).WithError(err).Warn("hub index fetch failed, trying mirror")
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
	shown := HubURLForLog(h.url)
	if h.inner != nil {
		return fmt.Sprintf("%s (status %d): %v", shown, h.statusCode, h.inner)
	}
	return fmt.Sprintf("%s (status %d)", shown, h.statusCode)
}

func (h hubHTTPError) Unwrap() error { return h.inner }
func (h hubHTTPError) CanFallback() bool {
	return h.fallback
}

// hubTransportError is a request that failed before any HTTP status arrived: a dial failure, TLS error, reset or
// timeout. The next candidate may still answer, so it falls back like an unavailable hub.
type hubTransportError struct {
	url   string
	inner error
}

func (h hubTransportError) Error() string {
	return fmt.Sprintf("%s (request failed): %v", HubURLForLog(h.url), h.inner)
}

func (h hubTransportError) Unwrap() error { return h.inner }
func (h hubTransportError) CanFallback() bool {
	return true
}

// candidateError labels a failed candidate with its URL exactly once. hubHTTPError and hubTransportError already
// name their URL in Error, so they are kept as they are; any other failure gets the URL as a prefix.
func candidateError(target string, err error) error {
	var httpErr hubHTTPError
	var transportErr hubTransportError
	if errors.As(err, &httpErr) || errors.As(err, &transportErr) {
		return err
	}
	return fmt.Errorf("%s: %w", HubURLForLog(target), err)
}

func (s *HubService) fetchIndexHTTPFromURL(ctx context.Context, target string) (HubIndex, error) {
	// CRITICAL FIX: Validate hub URL before making HTTP request (HIGH-001)
	if err := s.checkURL(target); err != nil {
		return HubIndex{}, fmt.Errorf("invalid hub URL: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, http.NoBody)
	if err != nil {
		return HubIndex{}, err
	}
	req.Header.Set("Accept", "application/json")

	resp, err := s.HTTPClient.Do(req)
	if err != nil {
		return HubIndex{}, hubTransportError{url: target, inner: transportCause(err)}
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			logger.Log().WithError(closeErr).Warn("Failed to close response body")
		}
	}()
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode >= 300 && resp.StatusCode < 400 {
			loc := resp.Header.Get("Location")
			return HubIndex{}, hubHTTPError{url: target, statusCode: resp.StatusCode, inner: fmt.Errorf("hub index redirect to %s; install cscli or set HUB_BASE_URL to a JSON hub endpoint", HubURLForLog(firstNonEmpty(loc, target))), fallback: true}
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
			return HubIndex{}, fmt.Errorf("decode hub index from %s: %w", HubURLForLog(target), err)
		}
	}
	return idx, nil
}

// Pull downloads a hub item, validates it and stores the raw item in the cache. It never changes hub state
// and holds the cache lock only while storing. A name that exists in several hub sections needs itemType;
// without it Pull answers an *AmbiguousTypeError listing the sections.
func (s *HubService) Pull(ctx context.Context, slug, itemType string) (PullResult, error) {
	if s.Cache == nil {
		return PullResult{}, fmt.Errorf("cache unavailable")
	}
	slug = strings.TrimSpace(slug)
	if !ValidCacheSlug(slug) {
		return PullResult{}, ErrInvalidSlug
	}
	if itemType != "" && !IsCacheItemType(itemType) {
		return PullResult{}, ErrInvalidItemType
	}
	s.sweepLegacyCache(ctx)

	pullCtx, cancel := context.WithTimeout(ctx, s.PullTimeout)
	defer cancel()

	var idx HubIndex
	haveIndex := false
	if itemType == "" {
		// Only the index knows whether the name exists in more than one section.
		var err error
		if cachedType, ok := s.freshCachedType(ctx, slug); ok {
			itemType = cachedType
		} else if idx, err = s.fetchIndexRecorded(pullCtx); err == nil {
			haveIndex = true
			if itemType, err = resolveItemType(idx, slug); err != nil {
				return PullResult{}, err
			}
		} else if itemType = s.onlyCachedType(ctx, slug); itemType == "" {
			return PullResult{}, err
		}
	}

	// Serve a valid, unexpired cache entry first.
	cachedMeta, cachedItem, err := s.Cache.LoadItem(ctx, itemType, slug)
	switch {
	case err == nil:
		return PullResult{Meta: cachedMeta, Preview: string(cachedItem)}, nil
	case errors.Is(err, ErrCacheExpired):
		_ = s.Cache.Evict(ctx, itemType, slug)
	case ctx.Err() != nil:
		return PullResult{}, ctx.Err()
	}

	if !haveIndex {
		if idx, err = s.fetchIndexRecorded(pullCtx); err != nil {
			return PullResult{}, err
		}
	}
	entry, ok := findIndexEntry(idx, itemType, slug)
	if !ok {
		return PullResult{}, fmt.Errorf("preset not found in hub")
	}
	if entry.DownloadURL == "" {
		if idx.offline {
			// The index came from cscli because the hub could not be reached, and cscli's index has no download location.
			return PullResult{}, fmt.Errorf("hub index has no download location for this preset: %w", ErrHubUnreachable)
		}
		return PullResult{}, fmt.Errorf("hub index has no download location for this preset")
	}

	item, err := s.fetchWithLimitFromURL(pullCtx, entry.DownloadURL, maxCacheItemBytes)
	if err != nil {
		return PullResult{}, fmt.Errorf("download preset: %w", err)
	}
	if verifyErr := verifyIndexDigest(entry, item); verifyErr != nil {
		return PullResult{}, verifyErr
	}
	if checkErr := checkHubItem(item); checkErr != nil {
		return PullResult{}, checkErr
	}

	stored, err := s.Cache.Store(pullCtx, slug, itemType, entry.Etag, "hub", item)
	if err != nil {
		logger.Log().WithField("error", util.SanitizeForLog(err.Error())).WithField("slug", util.SanitizeForLog(slug)).Error("failed to store preset in cache")
		return PullResult{}, fmt.Errorf("cache store: %w", err)
	}
	return PullResult{Meta: stored, Preview: string(item)}, nil
}

// fetchIndexRecorded fetches the index and, on success, remembers it with the time of the fetch. A failed
// fetch leaves the previous record untouched.
func (s *HubService) fetchIndexRecorded(ctx context.Context) (HubIndex, error) {
	idx, err := s.FetchIndex(ctx)
	if err != nil {
		return idx, err
	}
	s.indexMu.Lock()
	s.lastIndex, s.lastIndexAt = idx, s.clock()
	s.indexMu.Unlock()
	return idx, nil
}

func (s *HubService) clock() time.Time {
	if s.now != nil {
		return s.now()
	}
	return time.Now()
}

// freshCachedType returns the section of slug when the index request can be skipped: the slug has exactly one
// valid cached section, the index was fetched within indexFreshTTL, and that index does not list the name in
// another section. A name the retained index shows in several sections stays ambiguous.
func (s *HubService) freshCachedType(ctx context.Context, slug string) (string, bool) {
	s.indexMu.Lock()
	idx, at := s.lastIndex, s.lastIndexAt
	s.indexMu.Unlock()
	if at.IsZero() || s.clock().Sub(at) >= indexFreshTTL {
		return "", false
	}
	cachedType := s.onlyCachedType(ctx, slug)
	if cachedType == "" {
		return "", false
	}
	// Any error (ambiguous, not found, unsupported) or a different section means the retained index disagrees
	// with the cache: refetch so the caller receives the real answer from a current index.
	if indexType, err := resolveItemType(idx, slug); err != nil || indexType != cachedType {
		return "", false
	}
	return cachedType, true
}

// onlyCachedType returns the section of the single valid cache entry of slug, or "" when there is none or
// several. It lets a pull without a type be served from the cache while the hub index is unreachable.
func (s *HubService) onlyCachedType(ctx context.Context, slug string) string {
	cached, err := s.Cache.ListSlug(ctx, slug)
	if err != nil || len(cached) != 1 {
		return ""
	}
	return cached[0].Type
}

// resolveItemType returns the one section of the index that holds slug. A name in no section is not found; a
// name in several sections is ambiguous.
func resolveItemType(idx HubIndex, slug string) (string, error) {
	var types []string
	found := false
	for _, i := range idx.Items {
		if i.Name != slug {
			continue
		}
		found = true
		if IsCacheItemType(i.Type) && !slices.Contains(types, i.Type) {
			types = append(types, i.Type)
		}
	}
	switch len(types) {
	case 0:
		if found {
			return "", fmt.Errorf("unsupported hub item type")
		}
		return "", fmt.Errorf("preset not found in hub")
	case 1:
		return types[0], nil
	}
	slices.Sort(types)
	return "", &AmbiguousTypeError{Types: types}
}

// ErrHubItemUnverified is returned when a downloaded hub item does not match the digest the hub index publishes
// for its version, or when that digest record is malformed. It satisfies errors.Is(err, ErrPresetHubUpdating):
// the index and the item files are published at different moments, so a retry normally succeeds.
var ErrHubItemUnverified error = hubItemUnverifiedError{}

type hubItemUnverifiedError struct{}

func (hubItemUnverifiedError) Error() string {
	return "hub item could not be verified"
}

// Is makes an unverified item count as the hub still updating, so both reach the same retryable answer.
func (hubItemUnverifiedError) Is(target error) bool { return target == ErrPresetHubUpdating }

// verifyIndexDigest checks a downloaded hub item against the digest the hub index publishes for its version.
// An entry without a digest (an index with no record for the version, a mirror, or a custom endpoint) is accepted
// and logged at info level, so those endpoints keep working and the gap stays visible. Each Pull downloads one
// item, so this logs once per Pull. A malformed digest record or a mismatch is refused.
// Cache hits never reach this check; they were verified when they were stored.
func verifyIndexDigest(entry HubIndexEntry, item []byte) error {
	if entry.Digest == "" {
		logger.Log().WithField("slug", util.SanitizeForLog(entry.Name)).WithField("digest_checked", false).Info("hub item pulled without a digest to compare")
		return nil
	}
	if !sha256HexPattern.MatchString(entry.Digest) || entry.Digest != itemDigest(item) {
		return ErrHubItemUnverified
	}
	logger.Log().WithField("slug", util.SanitizeForLog(entry.Name)).WithField("digest_checked", true).Debug("hub item matches the hub index digest")
	return nil
}

// checkHubItem requires a hub download to be one raw YAML mapping. Archives, HTML pages and other
// payloads are refused with fixed messages and never cached.
func checkHubItem(item []byte) error {
	if len(item) > 1 && item[0] == 0x1f && item[1] == 0x8b {
		return fmt.Errorf("hub item is an archive; only raw item files are supported")
	}
	docs, err := ParseYAMLDocuments(bytes.NewReader(item), false)
	if err != nil || len(docs) != 1 || docs[0].Kind != yaml.DocumentNode || len(docs[0].Content) != 1 || docs[0].Content[0].Kind != yaml.MappingNode {
		return fmt.Errorf("hub item is not a YAML mapping")
	}
	return nil
}

// fetchWithLimitFromURL downloads one hub resource of at most limit bytes.
func (s *HubService) fetchWithLimitFromURL(ctx context.Context, url string, limit int64) ([]byte, error) {
	if err := s.checkURL(url); err != nil {
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
		return nil, fmt.Errorf("request %s: %w", HubURLForLog(url), transportCause(err))
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil {
			logger.Log().WithError(closeErr).Warn("Failed to close response body")
		}
	}()
	if resp.StatusCode != http.StatusOK {
		return nil, hubHTTPError{url: url, statusCode: resp.StatusCode, fallback: resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusForbidden || resp.StatusCode >= 500}
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, limit+1))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", HubURLForLog(url), err)
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("payload too large from %s", HubURLForLog(url))
	}
	return data, nil
}

// findIndexEntry returns the entry of section itemType whose name equals slug exactly. Hub item names are
// case-sensitive, so neither titles, suffixes nor other case variants match, and the same name in another
// section is a different item.
func findIndexEntry(idx HubIndex, itemType, slug string) (HubIndexEntry, bool) {
	if slug == "" {
		return HubIndexEntry{}, false
	}
	for _, i := range idx.Items {
		if i.Name == slug && i.Type == itemType {
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

// copyFile copies a single file, replacing dst when it exists.
func copyFile(src, dst string) error { return copyFileFlag(src, dst, os.O_TRUNC) }

// copyFileExclusive copies a single file to a path that must not exist yet: a file or link already there makes
// it fail instead of being written through.
func copyFileExclusive(src, dst string) error { return copyFileFlag(src, dst, os.O_EXCL) }

// errNotRegularFile is returned when a copy source is a link, directory or device.
var errNotRegularFile = errors.New("source is not a regular file")

// copyFileFlag copies src to dst, opened with O_CREATE|O_WRONLY and the given extra flag. The source is
// opened without following a final symlink and must be a regular file, checked on the open descriptor so a
// swap after the open cannot change what is read.
func copyFileFlag(src, dst string, flag int) error {
	srcFile, err := os.OpenFile(src, os.O_RDONLY|syscall.O_NOFOLLOW|syscall.O_NONBLOCK, 0) // #nosec G304 -- callers pass paths under Charon-owned data or snapshot folders; O_NOFOLLOW and the regular-file check below refuse links and special files
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
	if !srcInfo.Mode().IsRegular() {
		return fmt.Errorf("open src: %w", errNotRegularFile)
	}

	dstFile, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|flag, srcInfo.Mode()) // #nosec G304 -- dst is built by the callers below their own data or snapshot folder; O_EXCL (copyFileExclusive) refuses an existing entry
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

// sweepLegacyCache removes cache entries of the old layout. It runs detached from ctx cancellation and is
// marked done only on success, so a failed or cancelled first attempt is retried on the next pull.
func (s *HubService) sweepLegacyCache(ctx context.Context) {
	s.legacySweepMu.Lock()
	defer s.legacySweepMu.Unlock()
	if s.legacySweepDone {
		return
	}
	if err := s.Cache.RemoveLegacyEntries(context.WithoutCancel(ctx)); err != nil {
		logger.Log().WithError(err).Warn("failed to remove cache entries of the old layout")
		return
	}
	s.legacySweepDone = true
}
