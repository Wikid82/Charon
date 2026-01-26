# CrowdSec Preset Pull/Apply - Fix Summary

## Changes Made

### 1. Added Comprehensive Logging

**Files Modified:**

- `backend/internal/crowdsec/hub_cache.go` - Added logging to cache Store/Load operations
- `backend/internal/crowdsec/hub_sync.go` - Added logging to Pull/Apply flows
- `backend/internal/api/handlers/crowdsec_handler.go` - Added detailed logging to HTTP handlers

**Logging Added:**

- Cache directory checks and creation
- File storage operations with paths and sizes
- Cache lookup operations (hits/misses)
- File existence verification
- Cache contents listing on failures
- Error conditions with full context

### 2. Enhanced Error Messages

Improved user-facing error messages to be more actionable:

**Before:**

```
"cscli unavailable and no cached preset; pull the preset or install cscli"
```

**After:**

```
"CrowdSec preset not cached. Pull the preset first by clicking 'Pull Preview', then try applying again."
```

### 3. Added File Verification

After pull operations, the system now:

- Verifies archive file exists on disk
- Verifies preview file exists on disk
- Logs warnings if files are missing
- Provides detailed paths for manual inspection

Before apply operations, the system now:

- Checks if preset is cached
- Verifies cached files still exist
- Lists all cached presets if requested one is missing
- Provides detailed diagnostic information

### 4. Created Comprehensive Tests

**New Test Files:**

1. `backend/internal/crowdsec/hub_pull_apply_test.go`
   - `TestPullThenApplyFlow` - End-to-end pull→apply test
   - `TestApplyWithoutPullFails` - Verify error when cache missing
   - `TestCacheExpiration` - Verify TTL enforcement
   - `TestCacheListAfterPull` - Verify cache listing

2. `backend/internal/api/handlers/crowdsec_pull_apply_integration_test.go`
   - `TestPullThenApplyIntegration` - Full HTTP handler integration test
   - `TestApplyWithoutPullReturnsProperError` - Error message validation

3. `backend/internal/api/handlers/crowdsec_cache_verification_test.go`
   - `TestListPresetsShowsCachedStatus` - Verify presets show cached flag
   - `TestCacheKeyPersistence` - Verify cache keys persist correctly

**All tests pass ✅**

## How It Works

### Pull Operation Flow

```
1. Frontend: POST /admin/crowdsec/presets/pull {slug: "test/preset"}
   ↓
2. PullPreset Handler:
   - Logs cache directory and slug
   - Calls Hub.Pull(slug)
   ↓
3. Hub.Pull():
   - Logs "storing preset in cache" with sizes
   - Downloads archive and preview
   - Calls Cache.Store(slug, etag, source, preview, archive)
   ↓
4. Cache.Store():
   - Creates directory: {cacheDir}/{slug}/
   - Writes: bundle.tgz, preview.yaml, metadata.json
   - Logs "preset successfully stored" with all paths
   - Returns metadata with cache_key
   ↓
5. PullPreset Handler:
   - Logs "preset pulled and cached successfully"
   - Verifies files exist
   - Returns success response with cache_key
```

### Apply Operation Flow

```
1. Frontend: POST /admin/crowdsec/presets/apply {slug: "test/preset"}
   ↓
2. ApplyPreset Handler:
   - Logs "attempting to apply preset"
   - Checks if preset is cached
   - If cached: logs paths and cache_key
   - If not cached: logs warning + lists all cached presets
   - Calls Hub.Apply(slug)
   ↓
3. Hub.Apply():
   - Calls loadCacheMeta() -> Cache.Load(slug)
   - If cache miss: logs error and returns failure
   - If cached: logs "successfully loaded cached preset metadata"
   - Reads bundle.tgz from cached path
   - Extracts to dataDir
   - Creates backup
   ↓
4. ApplyPreset Handler:
   - Logs success or failure with full context
   - Returns response with backup path, cache_key, etc.
```

## Example Log Output

### Successful Pull + Apply

```bash
# Pull
time="2025-12-10T00:00:00Z" level=info msg="attempting to pull preset"
  cache_dir=/data/hub_cache
  slug=crowdsecurity/demo

time="2025-12-10T00:00:01Z" level=info msg="storing preset in cache"
  archive_size=12458
  etag=abc123
  preview_size=245
  slug=crowdsecurity/demo

time="2025-12-10T00:00:01Z" level=info msg="preset successfully stored in cache"
  archive_path=/data/hub_cache/crowdsecurity/demo/bundle.tgz
  cache_key=crowdsecurity/demo-1765324634
  meta_path=/data/hub_cache/crowdsecurity/demo/metadata.json
  preview_path=/data/hub_cache/crowdsecurity/demo/preview.yaml
  slug=crowdsecurity/demo

time="2025-12-10T00:00:01Z" level=info msg="preset pulled and cached successfully"
  archive_path=/data/hub_cache/crowdsecurity/demo/bundle.tgz
  cache_key=crowdsecurity/demo-1765324634
  slug=crowdsecurity/demo

# Apply
time="2025-12-10T00:00:10Z" level=info msg="attempting to apply preset"
  cache_dir=/data/hub_cache
  slug=crowdsecurity/demo

time="2025-12-10T00:00:10Z" level=info msg="preset found in cache"
  archive_path=/data/hub_cache/crowdsecurity/demo/bundle.tgz
  cache_key=crowdsecurity/demo-1765324634
  preview_path=/data/hub_cache/crowdsecurity/demo/preview.yaml
  slug=crowdsecurity/demo

time="2025-12-10T00:00:10Z" level=info msg="successfully loaded cached preset metadata"
  archive_path=/data/hub_cache/crowdsecurity/demo/bundle.tgz
  cache_key=crowdsecurity/demo-1765324634
  slug=crowdsecurity/demo
```

### Cache Miss Error

```bash
time="2025-12-10T00:00:15Z" level=info msg="attempting to apply preset"
  cache_dir=/data/hub_cache
  slug=crowdsecurity/missing

time="2025-12-10T00:00:15Z" level=warning msg="preset not found in cache before apply"
  error="cache miss"
  slug=crowdsecurity/missing

time="2025-12-10T00:00:15Z" level=info msg="current cache contents"
  cached_slugs=["crowdsecurity/demo", "crowdsecurity/other"]

time="2025-12-10T00:00:15Z" level=warning msg="crowdsec preset apply failed"
  error="CrowdSec preset not cached. Pull the preset first..."
```

## Troubleshooting Guide

### If Pull Succeeds But Apply Fails

1. **Check the logs** for pull operation:

   ```
   grep "preset successfully stored" logs.txt
   ```

   Should show the archive_path and cache_key.

2. **Verify files exist**:

   ```bash
   ls -la data/hub_cache/
   ls -la data/hub_cache/{slug}/
   ```

   Should see: `bundle.tgz`, `preview.yaml`, `metadata.json`

3. **Check file permissions**:

   ```bash
   stat data/hub_cache/{slug}/bundle.tgz
   ```

   Should be readable by the application user.

4. **Check logs during apply**:

   ```
   grep "preset found in cache" logs.txt
   ```

   If you see "preset not found in cache" instead, check:
   - Is the slug exactly the same?
   - Did the cache files get deleted?
   - Check the "cached_slugs" log entry

5. **Check cache TTL**:
   Default TTL is 24 hours. If you pulled >24 hours ago, cache is expired.
   Pull again to refresh.

### If Files Are Missing After Pull

If logs show "preset successfully stored" but files don't exist:

1. Check disk space:

   ```bash
   df -h /data
   ```

2. Check directory permissions:

   ```bash
   ls -ld data/hub_cache/
   ```

3. Check for filesystem errors in system logs

4. Check if something is cleaning up the cache directory

## Test Coverage

All tests pass with comprehensive coverage:

```bash
# Unit tests
go test ./internal/crowdsec -v -run "TestPullThenApplyFlow"
go test ./internal/crowdsec -v -run "TestApplyWithoutPullFails"
go test ./internal/crowdsec -v -run "TestCacheExpiration"
go test ./internal/crowdsec -v -run "TestCacheListAfterPull"

# Integration tests
go test ./internal/api/handlers -v -run "TestPullThenApplyIntegration"
go test ./internal/api/handlers -v -run "TestApplyWithoutPullReturnsProperError"
go test ./internal/api/handlers -v -run "TestListPresetsShowsCachedStatus"
go test ./internal/api/handlers -v -run "TestCacheKeyPersistence"

# All existing tests still pass
go test ./...
```

## Verification Checklist

- [x] Build succeeds without errors
- [x] All new tests pass
- [x] All existing tests still pass
- [x] Logging produces useful diagnostic information
- [x] Error messages are user-friendly
- [x] File paths are logged for manual verification
- [x] Cache operations are transparent
- [x] Pull→Apply flow works correctly
- [x] Error handling is comprehensive
- [x] Documentation is complete

## Next Steps

1. **Deploy and Monitor**: Deploy the updated backend and monitor logs for any pull/apply operations
2. **User Feedback**: If users still report issues, logs will now provide enough information to diagnose
3. **Performance**: If cache gets large, may need to add cache size limits or cleanup policies
4. **Enhancement**: Could add a cache status API endpoint to list all cached presets

## Files Changed

```
backend/internal/crowdsec/hub_cache.go                                 (+15 log statements)
backend/internal/crowdsec/hub_sync.go                                  (+10 log statements)
backend/internal/api/handlers/crowdsec_handler.go                      (+30 log statements + verification)
backend/internal/crowdsec/hub_pull_apply_test.go                       (NEW - 233 lines)
backend/internal/api/handlers/crowdsec_pull_apply_integration_test.go  (NEW - 152 lines)
backend/internal/api/handlers/crowdsec_cache_verification_test.go      (NEW - 105 lines)
docs/reports/crowdsec-preset-pull-apply-debug.md                       (NEW - documentation)
```

## Conclusion

The pull→apply functionality was working correctly. The issue was lack of visibility. With comprehensive logging now in place, operators can:

1. ✅ Verify pull operations succeed
2. ✅ See exactly where files are cached
3. ✅ Diagnose cache misses with full context
4. ✅ Manually verify file existence
5. ✅ Understand cache expiration
6. ✅ Get actionable error messages

This makes the system much easier to troubleshoot and support. If the issue persists for any user, the logs will now clearly show the root cause.
