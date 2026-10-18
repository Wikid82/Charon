package crowdsec

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/Wikid82/charon/backend/internal/logger"
	"github.com/Wikid82/charon/backend/internal/util"
)

// Apply failures the API maps to stable error codes.
var (
	// ErrPresetCacheKeyRequired is returned when a hub preset is applied without the cache key from its pull.
	ErrPresetCacheKeyRequired = errors.New("cache key required")
	// ErrPresetCacheStale is returned when the cached entry is missing, outdated or does not match what cscli can install.
	ErrPresetCacheStale = errors.New("the cached preset is outdated; pull it again")
	// ErrPresetHubUpdating is returned when the installed item differs from the pulled one because cscli's hub
	// index and the index used for the pull have not converged yet.
	ErrPresetHubUpdating = errors.New("the hub is still updating; try again later")
	// ErrPresetLocalModified is returned when the item was edited on this server, so cscli will not replace it.
	ErrPresetLocalModified = errors.New("the preset was edited on this server and was not replaced")
	// ErrPresetApplyTimeout is returned when the apply deadline passed; the configuration is restored.
	ErrPresetApplyTimeout = errors.New("the preset install took too long and was rolled back")
	// ErrPresetApplyCancelled is returned when the caller cancelled the apply; the configuration is restored.
	// It wraps context.Canceled, so errors.Is(err, context.Canceled) holds too.
	ErrPresetApplyCancelled = fmt.Errorf("the preset install was cancelled and was rolled back: %w", context.Canceled)
	// ErrHubUnreachable is returned when the hub cannot be reached over the network, for a pull or an install.
	ErrHubUnreachable = errors.New("the hub could not be reached")
	// ErrPresetTypeAmbiguous is returned when a hub name exists in several sections and no type was given.
	ErrPresetTypeAmbiguous = errors.New("the preset name exists in several hub sections; choose a type")
)

// AmbiguousTypeError names the hub sections that hold the requested preset name.
type AmbiguousTypeError struct{ Types []string }

func (e *AmbiguousTypeError) Error() string {
	return fmt.Sprintf("%s: %s", ErrPresetTypeAmbiguous, strings.Join(e.Types, ", "))
}

func (e *AmbiguousTypeError) Unwrap() error { return ErrPresetTypeAmbiguous }

// dependencySections are the item YAML keys that name other hub items, keyed to the cscli item type.
var dependencySections = []string{"collections", "parsers", "scenarios", "postoverflows", "contexts"}

const maxDependencies = 1000

// Phase names the stage an Apply is in, in the order Apply runs them. cscli reports no progress of its own,
// so ApplyRequest.OnPhase is the only way a caller learns where an apply is.
type Phase string

// Apply phases. PhaseRollback follows whichever phase failed.
const (
	// PhasePreflight covers the hub index update and the inspect before the snapshot.
	PhasePreflight Phase = "preflight"
	// PhaseSnapshot covers the configuration snapshot.
	PhaseSnapshot Phase = "snapshot"
	// PhaseInstall covers the cscli install.
	PhaseInstall Phase = "install"
	// PhaseVerify covers the inspect, upgrade and inspect checks, the hub list and the dependency check.
	PhaseVerify Phase = "verify"
	// PhaseReload covers the CrowdSec reload after a verified install.
	PhaseReload Phase = "reload"
	// PhaseRollback covers the restore of the snapshot after a failure.
	PhaseRollback Phase = "rollback"
)

// ApplyRequest is one preset apply. The zero Timeout uses HubService.ApplyTimeout; a caller that runs the
// apply in the background passes its own, longer, limit.
type ApplyRequest struct {
	Slug     string
	Type     string
	CacheKey string
	Timeout  time.Duration
	// OnPhase, when set, is called synchronously as each phase starts. It must return quickly; a panic in it
	// is logged and never fails the apply.
	OnPhase func(Phase)
	// OnSnapshot, when set, is called with the snapshot path as soon as the snapshot exists, so a caller can
	// persist it before the long install and restore it after a crash. Like OnPhase, a panic in it is logged.
	OnSnapshot func(path string)
}

// enter reports the start of phase p to OnPhase, if set.
func (r ApplyRequest) enter(p Phase) {
	if r.OnPhase == nil {
		return
	}
	defer func() {
		if rec := recover(); rec != nil {
			logger.Log().WithField("phase", string(p)).WithField("panic", util.SanitizeForLog(fmt.Sprint(rec))).Error("preset apply phase callback panicked")
		}
	}()
	r.OnPhase(p)
}

// snapshotted reports the snapshot path to OnSnapshot, if set.
func (r ApplyRequest) snapshotted(path string) {
	if r.OnSnapshot == nil {
		return
	}
	defer func() {
		if rec := recover(); rec != nil {
			logger.Log().WithField("panic", util.SanitizeForLog(fmt.Sprint(rec))).Error("preset apply snapshot callback panicked")
		}
	}()
	r.OnSnapshot(path)
}

// applyTimeoutError returns the apply timeout error once ctx's deadline has passed, else nil.
func applyTimeoutError(ctx context.Context) error {
	if errors.Is(ctx.Err(), context.DeadlineExceeded) {
		return fmt.Errorf("%w: %w", ErrPresetApplyTimeout, context.DeadlineExceeded)
	}
	return nil
}

// applyContextError maps a finished apply context to its error: a passed deadline to ErrPresetApplyTimeout
// and a cancel to ErrPresetApplyCancelled. It returns nil while ctx is live.
func applyContextError(ctx context.Context) error {
	if err := applyTimeoutError(ctx); err != nil {
		return err
	}
	if errors.Is(ctx.Err(), context.Canceled) {
		return ErrPresetApplyCancelled
	}
	return nil
}

// Apply installs the previewed hub item with cscli and verifies that what cscli installed is the item that
// was pulled. DataDir is never renamed: a snapshot of the configuration (not of the hub tree, see
// snapshotWithoutHub) is taken first and restored in place on failure. Apply never contacts the hub to refresh the cache; an entry that is missing, expired or has a
// different cacheKey is reported as stale. itemType names the hub section of the pulled item; when it is empty,
// the section is taken from the cache entry that carries cacheKey.
func (s *HubService) Apply(ctx context.Context, slug, itemType, cacheKey string) (ApplyResult, error) {
	return s.ApplyWith(ctx, ApplyRequest{Slug: slug, Type: itemType, CacheKey: cacheKey})
}

// ApplyWith is Apply with a per-request time limit and phase reporting; see ApplyRequest.
func (s *HubService) ApplyWith(ctx context.Context, req ApplyRequest) (ApplyResult, error) {
	slug, itemType, cacheKey := req.Slug, req.Type, req.CacheKey
	result := ApplyResult{AppliedPreset: slug, Status: "failed"}
	if !ValidCacheSlug(slug) {
		return result, ErrInvalidSlug
	}
	if itemType != "" && !IsCacheItemType(itemType) {
		return result, ErrInvalidItemType
	}
	if cacheKey == "" {
		return result, ErrPresetCacheKeyRequired
	}
	if s.Cache == nil {
		return result, fmt.Errorf("cache unavailable for manual apply")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	timeout := req.Timeout
	if timeout <= 0 {
		timeout = s.ApplyTimeout
	}
	applyCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if itemType == "" {
		if itemType = s.cachedTypeOfKey(applyCtx, slug, cacheKey); itemType == "" {
			if err := applyTimeoutError(applyCtx); err != nil {
				return result, err
			}
			// No valid entry carries this key, so there is nothing to apply.
			return result, ErrPresetCacheStale
		}
	}
	meta, item, err := s.Cache.LoadItem(applyCtx, itemType, slug)
	if err != nil {
		if timeoutErr := applyTimeoutError(applyCtx); timeoutErr != nil {
			return result, timeoutErr
		}
		if errors.Is(err, ErrCacheMiss) || errors.Is(err, ErrCacheExpired) || errors.Is(err, ErrCacheInvalid) {
			logger.Log().WithField("slug", util.SanitizeForLog(slug)).WithField("reason", util.SanitizeForLog(err.Error())).Info("preset cache entry unusable for apply")
			return result, ErrPresetCacheStale
		}
		return result, fmt.Errorf("load cache for %s: %w", slug, err)
	}
	if meta.CacheKey != cacheKey {
		return result, ErrPresetCacheStale
	}
	result.CacheKey = meta.CacheKey
	result.Type = meta.Type

	if !s.hasCSCLI(applyCtx) {
		return result, fmt.Errorf("%w or not responding", ErrCSCLIUnavailable)
	}

	// The hub index is refreshed and the item inspected before anything is snapshotted, so a refusal that
	// depends only on the state before the install costs no snapshot.
	req.enter(PhasePreflight)
	s.updateHubIndex(applyCtx)
	before, err := s.preflight(applyCtx, meta)
	if err != nil {
		return result, err
	}
	if timeoutErr := applyTimeoutError(applyCtx); timeoutErr != nil {
		return result, timeoutErr
	}
	req.enter(PhaseSnapshot)
	backupPath, err := s.snapshotWithoutHub(before)
	if err != nil {
		return result, fmt.Errorf("backup: %w", err)
	}
	result.BackupPath = backupPath
	req.snapshotted(backupPath)
	defer s.pruneAfterApply()

	// rollBack restores the snapshot and reports cause; a passed deadline or a cancel wins over any other
	// cause and keeps the cache entry. The restore does not use the apply context, so it runs after either.
	rollBack := func(cause error) (ApplyResult, error) {
		if ctxErr := applyContextError(applyCtx); ctxErr != nil {
			cause = ctxErr
		}
		req.enter(PhaseRollback)
		return result, s.restoreAfter(cause, backupPath, slug)
	}

	req.enter(PhaseInstall)
	installOut, err := s.runCSCLI(applyCtx, meta.Type, "install", meta.Name)
	if err != nil {
		return rollBack(installFailure(meta, installOut, err))
	}
	req.enter(PhaseVerify)
	installed, err := s.inspectItem(applyCtx, meta.Type, meta.Name)
	if err != nil {
		return rollBack(err)
	}
	if !*installed.Installed {
		return rollBack(ErrPresetCacheStale)
	}
	if installed.Tainted {
		return rollBack(s.localModified(applyCtx, meta))
	}
	if !strings.EqualFold(installed.LocalHash, meta.ItemSHA256) {
		if installed.UpToDate == nil || *installed.UpToDate {
			req.enter(PhaseRollback)
			return s.rollBackMismatch(applyCtx, result, backupPath, meta)
		}
		// An older, unmodified version was already installed and install left it alone.
		if _, upErr := s.runCSCLI(applyCtx, meta.Type, "upgrade", meta.Name); upErr != nil {
			return rollBack(fmt.Errorf("upgrade %s %s: %w", meta.Type, meta.Name, upErr))
		}
		if installed, err = s.inspectItem(applyCtx, meta.Type, meta.Name); err != nil {
			return rollBack(err)
		}
		if installed.Tainted {
			return rollBack(s.localModified(applyCtx, meta))
		}
		if !strings.EqualFold(installed.LocalHash, meta.ItemSHA256) {
			req.enter(PhaseRollback)
			return s.rollBackMismatch(applyCtx, result, backupPath, meta)
		}
	}

	warnings, err := s.checkDependencies(applyCtx, meta, item)
	if err != nil {
		return rollBack(err)
	}
	result.Warnings = warnings

	req.enter(PhaseReload)
	result.Status = "applied"
	result.ReloadHint = !s.reloadCrowdSec(applyCtx, "hub preset apply")
	result.UsedCSCLI = true
	return result, nil
}

// rollBackMismatch restores the snapshot after the installed hash differed from the pulled item and drops the
// cache entry, but only while it still carries the key Apply loaded: a concurrent pull may have replaced it.
// A passed deadline or a cancel skips the eviction and is reported instead of the mismatch.
func (s *HubService) rollBackMismatch(ctx context.Context, result ApplyResult, backupPath string, meta CachedPreset) (ApplyResult, error) {
	if ctxErr := applyContextError(ctx); ctxErr != nil {
		return result, s.restoreAfter(ctxErr, backupPath, meta.Slug)
	}
	if err := s.Cache.EvictIfKey(ctx, meta.Type, meta.Slug, meta.CacheKey); err != nil {
		logger.Log().WithError(err).WithField("slug", util.SanitizeForLog(meta.Slug)).Warn("failed to evict outdated cache entry")
	}
	return result, s.restoreAfter(ErrPresetHubUpdating, backupPath, meta.Slug)
}

// restoreAfter restores the snapshot after a failed apply step and returns the original error,
// annotated when the restore itself failed.
func (s *HubService) restoreAfter(cause error, backupPath, slug string) error {
	if rbErr := s.restore(backupPath); rbErr != nil {
		return rollbackFailure(cause, rbErr, backupPath)
	}
	logger.Log().WithField("error", util.SanitizeForLog(cause.Error())).WithField("slug", util.SanitizeForLog(slug)).WithField("backup_path", util.SanitizeForLog(backupPath)).Warn("preset apply failed; restored previous configuration")
	return cause
}

// updateHubIndex refreshes cscli's local hub index. It is best effort and runs under its own sub-budget: a
// slow or failing hub update leaves the local index in place and the install goes on.
func (s *HubService) updateHubIndex(ctx context.Context) {
	budget := s.HubUpdateTimeout
	if budget <= 0 {
		budget = defaultHubUpdateTimeout
	}
	updateCtx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()
	if _, err := s.runCSCLI(updateCtx, "hub", "update"); err != nil {
		logger.Log().WithError(err).Warn("cscli hub update failed; continuing with the local hub index")
	}
}

// networkFailureMarkers are the output fragments of a failed network request (cscli's own text and the Go resolver
// and dialer errors it passes on). Each one is specific enough not to match an item or file name.
var networkFailureMarkers = []string{
	"dial tcp",
	"no such host",
	"network is unreachable",
	"connection refused",
	"i/o timeout",
	"unable to send request",
	"unable to retrieve latest crowdsec version",
}

// dnsLookupFailure matches the Go resolver's "lookup <host> on <server>" text, which a bare "lookup" would not
// tell apart from an item name.
var dnsLookupFailure = regexp.MustCompile(`lookup \S+ on \S`)

// hasNetworkFailure reports whether cscli output describes a failed network request.
func hasNetworkFailure(text string) bool {
	text = strings.ToLower(text)
	for _, marker := range networkFailureMarkers {
		if strings.Contains(text, marker) {
			return true
		}
	}
	return dnsLookupFailure.MatchString(text)
}

// installFailure classifies a failed install: an item the local index does not know is a stale preset, and a failed
// network request means the hub could not be reached.
func installFailure(meta CachedPreset, out []byte, err error) error {
	text := string(out) + " " + err.Error()
	if strings.Contains(text, "can't find") || strings.Contains(text, "invalid hub index") {
		return ErrPresetCacheStale
	}
	if hasNetworkFailure(text) {
		return fmt.Errorf("install %s %s: %w: %w", meta.Type, meta.Name, ErrHubUnreachable, err)
	}
	return fmt.Errorf("install %s %s: %w", meta.Type, meta.Name, err)
}

// itemState is the part of `cscli <type> inspect <name> -o json` that Apply reads.
type itemState struct {
	Installed *bool  `json:"installed"`
	Tainted   bool   `json:"tainted"`
	UpToDate  *bool  `json:"up_to_date"`
	LocalHash string `json:"local_hash"`
	// Path is the item's own hub file, relative to the hub directory (for example collections/acme/nginx.yaml).
	Path string `json:"path"`
	// LocalPath is where the item is installed (a link into the hub tree); it only serves as a fallback to find
	// the hub file when Path is missing.
	LocalPath string `json:"local_path"`
}

func (s *HubService) inspectItem(ctx context.Context, itemType, name string) (itemState, error) {
	out, err := s.runCSCLI(ctx, itemType, "inspect", name, "-o", "json")
	if err != nil {
		return itemState{}, fmt.Errorf("inspect %s %s: %w", itemType, name, err)
	}
	return parseItemState(out)
}

// parseItemState reads the `cscli <type> inspect -o json` answer.
func parseItemState(out []byte) (itemState, error) {
	var state itemState
	if err := unmarshalCSCLIJSON(out, &state); err != nil {
		return itemState{}, fmt.Errorf("parse inspect output: %w", err)
	}
	if state.Installed == nil {
		return itemState{}, errors.New("inspect output has no installed field")
	}
	return state, nil
}

// preflight inspects the top-level item before the install and refuses what the install cannot change: an item
// the local index does not know (stale preset) and an item edited on this server. Any other inspect problem is
// left to the checks after the install. The returned state locates the item's own hub file for the snapshot.
func (s *HubService) preflight(ctx context.Context, meta CachedPreset) (itemState, error) {
	out, err := s.runCSCLI(ctx, meta.Type, "inspect", meta.Name, "-o", "json")
	if err != nil {
		if strings.Contains(string(out)+" "+err.Error(), "can't find") {
			return itemState{}, ErrPresetCacheStale
		}
		logger.Log().WithError(err).Warn("preset pre-flight inspect failed; continuing")
		return itemState{}, nil
	}
	state, err := parseItemState(out)
	if err != nil {
		logger.Log().WithError(err).Warn("preset pre-flight inspect unreadable; continuing")
		return itemState{}, nil
	}
	if state.Tainted {
		return itemState{}, s.localModified(ctx, meta)
	}
	return state, nil
}

// hubListStatus runs one `cscli hub list -o json` and returns the status of every installed item, keyed
// "section/name".
func (s *HubService) hubListStatus(ctx context.Context) (map[string]string, error) {
	out, err := s.runCSCLI(ctx, "hub", "list", "-o", "json")
	if err != nil {
		return nil, fmt.Errorf("list installed hub items: %w", err)
	}
	var installed map[string][]struct {
		Name   string `json:"name"`
		Status string `json:"status"`
	}
	if jsonErr := unmarshalCSCLIJSON(out, &installed); jsonErr != nil {
		return nil, fmt.Errorf("parse hub list: %w", jsonErr)
	}
	status := map[string]string{}
	for section, items := range installed {
		for _, it := range items {
			status[section+"/"+it.Name] = it.Status
		}
	}
	return status, nil
}

// LocalModifiedError is ErrPresetLocalModified together with the installed items cscli reports as edited,
// as "type/name" strings. The top-level item can be edited only through one of its dependencies.
type LocalModifiedError struct {
	Items []string
}

func (e *LocalModifiedError) Error() string { return ErrPresetLocalModified.Error() }

func (e *LocalModifiedError) Unwrap() error { return ErrPresetLocalModified }

// maxTaintedItems bounds the edited items reported back.
const maxTaintedItems = 50

// maxScopeInspections bounds the `cscli <type> inspect` calls made to find a preset's member items.
const maxScopeInspections = 20

// localModified builds the error for an edited top-level item. It names the edited items that belong to the
// preset: the top-level item and the members cscli lists for it, collections followed recursively. When the
// members cannot be read, only the top-level item is considered; the rest of the hub is never listed.
func (s *HubService) localModified(ctx context.Context, meta CachedPreset) error {
	top := meta.Type + "/" + meta.Name
	items := []string{top}
	if status, err := s.hubListStatus(ctx); err == nil {
		scope := s.presetScope(ctx, meta)
		if tainted := taintedItems(status, scope); len(tainted) > 0 {
			items = tainted
		}
	}
	return &LocalModifiedError{Items: items}
}

// presetScope returns the "type/name" keys of the top-level item and of the members `cscli inspect` lists
// for it, transitively through member collections. A failed inspection leaves that item's members out.
func (s *HubService) presetScope(ctx context.Context, meta CachedPreset) map[string]bool {
	top := meta.Type + "/" + meta.Name
	scope := map[string]bool{top: true}
	queue := []HubItem{{Type: meta.Type, Name: meta.Name}}
	for inspections := 0; len(queue) > 0 && inspections < maxScopeInspections; inspections++ {
		cur := queue[0]
		queue = queue[1:]
		for _, member := range s.itemMembers(ctx, cur) {
			key := member.Type + "/" + member.Name
			if scope[key] {
				continue
			}
			scope[key] = true
			if member.Type == "collections" {
				queue = append(queue, member)
			}
		}
	}
	return scope
}

// itemMembers returns the well-formed items an inspected item lists as its members, or nil when the
// inspection fails.
func (s *HubService) itemMembers(ctx context.Context, item HubItem) []HubItem {
	out, err := s.runCSCLI(ctx, item.Type, "inspect", item.Name, "-o", "json")
	if err != nil {
		return nil
	}
	var doc map[string]json.RawMessage
	if unmarshalCSCLIJSON(out, &doc) != nil {
		return nil
	}
	var members []HubItem
	for _, section := range HubItemTypes {
		var names []string
		if json.Unmarshal(doc[section], &names) != nil {
			continue
		}
		for _, name := range names {
			if m := (HubItem{Type: section, Name: name}); m.Valid() {
				members = append(members, m)
			}
		}
	}
	return members
}

// taintedItems returns the sorted "type/name" of the items in scope whose status says they were edited.
// Names that are not well-formed hub item names are dropped, so the result is safe to show.
func taintedItems(status map[string]string, scope map[string]bool) []string {
	var items []string
	for key, st := range status {
		section, name, ok := strings.Cut(key, "/")
		if !ok || !scope[key] || !strings.Contains(st, "tainted") || !IsHubItemType(section) || !ValidHubItemName(name) {
			continue
		}
		items = append(items, key)
	}
	slices.Sort(items)
	if len(items) > maxTaintedItems {
		items = items[:maxTaintedItems]
	}
	return items
}

// checkDependencies runs one `cscli hub list -o json` and requires the item and every dependency its YAML
// names to be installed. A missing one is an error; an edited one only produces a warning. Dependencies of
// dependencies are not checked: cscli resolves them.
func (s *HubService) checkDependencies(ctx context.Context, meta CachedPreset, item []byte) ([]string, error) {
	status, err := s.hubListStatus(ctx)
	if err != nil {
		return nil, err
	}
	if _, ok := status[meta.Type+"/"+meta.Name]; !ok {
		return nil, ErrPresetCacheStale
	}

	deps, err := itemDependencies(item)
	if err != nil {
		return nil, err
	}
	var warnings []string
	for _, dep := range deps {
		st, ok := status[dep.section+"/"+dep.name]
		if !ok {
			logger.Log().WithField("dependency", util.SanitizeForLog(dep.name)).Warn("preset dependency is not installed")
			return nil, ErrPresetCacheStale
		}
		if strings.Contains(st, "tainted") {
			if !ValidHubItemName(dep.name) {
				continue
			}
			logger.Log().WithField("dependency", util.SanitizeForLog(dep.name)).Warn("preset dependency was edited on this server")
			warnings = append(warnings, fmt.Sprintf("dependency %s was edited locally", dep.name))
		}
	}
	return warnings, nil
}

type itemDependency struct{ section, name string }

// itemDependencies reads the dependency lists of a hub item's YAML.
func itemDependencies(item []byte) ([]itemDependency, error) {
	docs, err := ParseYAMLDocuments(bytes.NewReader(item), true)
	if err != nil || len(docs) != 1 {
		return nil, fmt.Errorf("parse item: %w", errors.Join(ErrPresetCacheStale, err))
	}
	var doc map[string]any
	if err := docs[0].Decode(&doc); err != nil {
		return nil, fmt.Errorf("parse item: %w", errors.Join(ErrPresetCacheStale, err))
	}
	var deps []itemDependency
	for _, section := range dependencySections {
		list, _ := doc[section].([]any)
		for _, v := range list {
			if name, ok := v.(string); ok && name != "" {
				deps = append(deps, itemDependency{section: section, name: name})
			}
		}
	}
	if len(deps) > maxDependencies {
		return nil, fmt.Errorf("item names too many dependencies: %w", ErrPresetCacheStale)
	}
	return deps, nil
}

// cachedTypeOfKey returns the section of the cache entry of slug that carries cacheKey, or "" when none does.
func (s *HubService) cachedTypeOfKey(ctx context.Context, slug, cacheKey string) string {
	cached, err := s.Cache.ListSlug(ctx, slug)
	if err != nil {
		return ""
	}
	for _, entry := range cached {
		if entry.CacheKey == cacheKey {
			return entry.Type
		}
	}
	return ""
}
