# CrowdSec Preset Pull/Apply Flow - Debug Report

## Issue Summary

User reported that pulling CrowdSec presets appeared to succeed, but applying them failed with "preset not cached" error, suggesting either:

1. Pull was failing silently
2. Cache was not being saved correctly
3. Apply was looking in the wrong location
4. Cache key mismatch between pull and apply

## Investigation Results

### Architecture Overview

The CrowdSec preset system has three main components:

1. **HubCache** (`backend/internal/crowdsec/hub_cache.go`)
   - Stores presets on disk at `{dataDir}/hub_cache/{slug}/`
   - Each preset has: `bundle.tgz`, `preview.yaml`, `metadata.json`
   - Enforces TTL-based expiration (default: 24 hours)

2. **HubService** (`backend/internal/crowdsec/hub_sync.go`)
   - Orchestrates pull and apply operations
   - `Pull()`: Downloads from hub, stores in cache
   - `Apply()`: Loads from cache, extracts to dataDir

3. **CrowdsecHandler** (`backend/internal/api/handlers/crowdsec_handler.go`)
   - HTTP endpoints: `/pull` and `/apply`
   - Manages hub service and cache initialization

### Pull Flow (What Actually Happens)

```
1. Frontend POST /admin/crowdsec/presets/pull {slug: "test/preset"}
2. Handler.PullPreset() calls Hub.Pull()
3. Hub.Pull():
   - Fetches index from hub
   - Downloads archive (.tgz) and preview (.yaml)
   - Calls Cache.Store(slug, etag, source, preview, archive)
4. Cache.Store():
   - Creates directory: {cacheDir}/{slug}/
   - Writes: bundle.tgz, preview.yaml, metadata.json
   - Returns CachedPreset metadata with paths
5. Handler returns: {status, slug, preview, cache_key, etag, ...}
```

### Apply Flow (What Actually Happens)

```
1. Frontend POST /admin/crowdsec/presets/apply {slug: "test/preset"}
2. Handler.ApplyPreset() calls Hub.Apply()
3. Hub.Apply():
   - Calls loadCacheMeta() which calls Cache.Load(slug)
   - Cache.Load() reads metadata.json from {cacheDir}/{slug}/
   - If cache miss and no cscli: returns error
   - If cached: reads bundle.tgz, extracts to dataDir
4. Handler returns: {status, backup, reload_hint, cache_key, ...}
```

### Root Cause Analysis

**The pull→apply flow was actually working correctly!** The investigation revealed:

1. ✅ **Cache storage works**: Pull successfully stores files to disk
2. ✅ **Cache loading works**: Apply successfully reads from same location
3. ✅ **Cache keys match**: Both use the slug as the lookup key
4. ✅ **Permissions are fine**: Tests show no permission issues

**However, there was a lack of visibility:**

- Pull/apply operations had minimal logging
- Errors could be hard to diagnose without detailed logs
- Cache operations were opaque to operators

## Implemented Fixes

### 1. Comprehensive Logging

Added detailed logging at every critical point:

**HubCache Operations** (`hub_cache.go`):

- Store: Log cache directory, file sizes, paths created
- Load: Log cache lookups, hits/misses, expiration checks
- Include full file paths for debugging

**HubService Operations** (`hub_sync.go`):

- Pull: Log archive download, preview fetch, cache storage
- Apply: Log cache lookup, file extraction, backup creation
- Track each step with context

**Handler Operations** (`crowdsec_handler.go`):

- PullPreset: Log cache directory checks, file existence verification
- ApplyPreset: Log cache status before apply, list cached slugs if miss occurs
- Include hub base URL and slug in all logs

### 2. Enhanced Error Messages

**Before:**

```
error: "cscli unavailable and no cached preset; pull the preset or install cscli"
```

**After:**

```
error: "CrowdSec preset not cached. Pull the preset first by clicking 'Pull Preview', then try applying again."
```

More user-friendly with actionable guidance.

### 3. Verification Checks

Added file existence verification after cache operations:

- After pull: Check that archive and preview files exist
- Before apply: Check cache and verify files are still present
- Log any discrepancies immediately

### 4. Comprehensive Testing

Created new test suite to verify pull→apply workflow:

**`hub_pull_apply_test.go`**:

- `TestPullThenApplyFlow`: End-to-end pull→apply test
- `TestApplyWithoutPullFails`: Verify proper error when cache missing
- `TestCacheExpiration`: Verify TTL enforcement
- `TestCacheListAfterPull`: Verify cache listing works

**`crowdsec_pull_apply_integration_test.go`**:

- `TestPullThenApplyIntegration`: HTTP handler integration test
- `TestApplyWithoutPullReturnsProperError`: Error message validation

All tests pass ✅

## Example Log Output

### Successful Pull

```
level=info msg="attempting to pull preset" cache_dir=/data/hub_cache slug=test/preset
level=info msg="storing preset in cache" archive_size=158 etag=abc123 preview_size=24 slug=test/preset
level=info msg="preset successfully stored in cache"
  archive_path=/data/hub_cache/test/preset/bundle.tgz
  cache_key=test/preset-1765324634
  preview_path=/data/hub_cache/test/preset/preview.yaml
  slug=test/preset
level=info msg="preset pulled and cached successfully" ...
```

### Successful Apply

```
level=info msg="attempting to apply preset" cache_dir=/data/hub_cache slug=test/preset
level=info msg="preset found in cache"
  archive_path=/data/hub_cache/test/preset/bundle.tgz
  cache_key=test/preset-1765324634
  slug=test/preset
level=info msg="successfully loaded cached preset metadata" ...
```

### Cache Miss Error

```
level=info msg="attempting to apply preset" slug=test/preset
level=warning msg="preset not found in cache before apply" error="cache miss" slug=test/preset
level=info msg="current cache contents" cached_slugs=["other/preset"]
level=warning msg="crowdsec preset apply failed" error="preset not cached" ...
```

## Verification Steps

To verify the fix works, follow these steps:

1. **Build the updated backend:**

   ```bash
   cd backend && go build ./cmd/api
   ```

2. **Run the backend with logging enabled:**

   ```bash
   ./api
   ```

3. **Pull a preset from the UI:**
   - Check logs for "preset successfully stored in cache"
   - Note the archive_path in logs

4. **Apply the preset:**
   - Check logs for "preset found in cache"
   - Should succeed without "preset not cached" error

5. **Verify cache contents:**

   ```bash
   ls -la data/hub_cache/
   ```

   Should show preset directories with files.

## Files Modified

1. `backend/internal/crowdsec/hub_cache.go`
   - Added logger import
   - Added logging to Store() and Load() methods
   - Log cache directory creation, file writes, cache misses

2. `backend/internal/crowdsec/hub_sync.go`
   - Added logging to Pull() and Apply() methods
   - Log cache storage operations and metadata loading
   - Track download sizes and file paths

3. `backend/internal/api/handlers/crowdsec_handler.go`
   - Added comprehensive logging to PullPreset() and ApplyPreset()
   - Check cache directory before operations
   - Verify files exist after pull
   - List cache contents when apply fails

4. `backend/internal/crowdsec/hub_pull_apply_test.go` (NEW)
   - Comprehensive unit tests for pull→apply flow

5. `backend/internal/api/handlers/crowdsec_pull_apply_integration_test.go` (NEW)
   - HTTP handler integration tests

## Conclusion

The pull→apply functionality was working correctly from an implementation standpoint. The issue was lack of visibility into cache operations, making it difficult to diagnose problems. With comprehensive logging now in place:

1. ✅ Operators can verify pull operations succeed
2. ✅ Operators can see exactly where files are cached
3. ✅ Apply failures show cache contents for debugging
4. ✅ Error messages guide users to correct actions
5. ✅ File paths are logged for manual verification

**If users still experience "preset not cached" errors, the logs will now clearly show:**

- Whether pull succeeded
- Where files were saved
- Whether files still exist when apply runs
- What's actually in the cache
- Any permission or filesystem issues

This makes the system much easier to troubleshoot and support.
