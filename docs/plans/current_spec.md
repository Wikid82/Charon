# CrowdSec Preset Apply Cache Miss — Bot Mitigation Essentials
**Date:** December 11, 2025
**Incident:** `CrowdSec preset add error: Apply failed: load cache: load cache for bot-mitigation-essentials: cache miss. Backup created at data/crowdsec.backup.20251210-193359`

## Context Snapshot
- **Observed error path:** `HubService.Apply()` → `loadCacheMeta()` → `HubCache.Load()` returns `ErrCacheMiss`, while apply already created a backup at `data/crowdsec.backup.*`, indicating we fell through the cscli path and then the manual cache path without a cached bundle.
- **Key components in play:**
	- Cache layer: [backend/internal/crowdsec/hub_cache.go](backend/internal/crowdsec/hub_cache.go) (`Store`, `Load`, `List`, `Exists`, `Touch`)
	- Hub orchestration: [backend/internal/crowdsec/hub_sync.go](backend/internal/crowdsec/hub_sync.go) (`Pull`, `Apply`, `loadCacheMeta`, `runCSCLI`, `extractTarGz`)
	- HTTP surface: [backend/internal/api/handlers/crowdsec_handler.go](backend/internal/api/handlers/crowdsec_handler.go) (`PullPreset`, `ApplyPreset`, `ListPresets`, `GetCachedPreset`)
	- Coverage and repro baselines: [backend/internal/crowdsec/hub_pull_apply_test.go](backend/internal/crowdsec/hub_pull_apply_test.go), [backend/internal/api/handlers/crowdsec_pull_apply_integration_test.go](backend/internal/api/handlers/crowdsec_pull_apply_integration_test.go)
- **Hypotheses to validate:**
	1. **Cache never created** for slug `bot-mitigation-essentials` (e.g., hub index didn’t contain slug, slug mismatch, or pull failure masked by fallback logging).
	2. **Cache existed but expired/evicted** (24h TTL default in `NewHubCache`, `ErrCacheExpired` treated as miss) before apply.
	3. **cscli path failed** and manual path fell back to cache that was missing; backup already created → rollback not restoring correctly on miss.
	4. **Slug naming drift** between curated presets and hub index (e.g., `crowdsecurity/bot-mitigation-essentials` vs `bot-mitigation-essentials`).

## Plan (phased; minimize requests)
### Phase 1 — Fast Forensics (no new mutations)
- Inspect logs for the failing apply to capture:
	- `crowdsec preset apply failed` entries in [backend/internal/api/handlers/crowdsec_handler.go](backend/internal/api/handlers/crowdsec_handler.go) (ensure we log `cache_key`, `backup_path`, `hub_base_url`).
	- Prior `preset pulled and cached successfully` entries for the same slug to see if pull ever succeeded.
- Check cache filesystem state without new pulls:
	- List `data/hub_cache/` and `backend/data/hub_cache/` for `bot-mitigation-essentials` to confirm presence of `metadata.json`, `bundle.tgz`, `preview.yaml`.
	- Read `metadata.json` to confirm `retrieved_at` vs TTL and `cache_key`.
- Confirm whether curated presets include the slug:
	- Inspect `ListCuratedPresets()` in [backend/internal/crowdsec/presets.go](backend/internal/crowdsec/presets.go) (if present) and compare to hub index slugs.

### Phase 2 — Reproduce with Minimal Requests
- Execute one controlled pull + apply sequence for `bot-mitigation-essentials` only:
	1. `POST /api/v1/admin/crowdsec/presets/pull {slug}` — capture response `cache_key`, `etag`, and verify cache files written.
	2. `POST /api/v1/admin/crowdsec/presets/apply {slug}` — watch for fallback message `load cache for ... cache miss`.
- Capture logs around these calls to see which path ran:
	- `HubService.Apply()` branch (`hasCSCLI`, `runCSCLI` success/fail, then `loadCacheMeta`).
	- `HubCache.Load()` result (hit/expired/miss).
- Validate backup rollback: ensure `data/crowdsec.backup.*` is restored when cache miss occurs.

### Phase 3 — Code Fix Design (targeted, low-risk)
- **Cache resilience:**
	- In `HubService.Apply()`, when `runCSCLI` fails **and** `loadCacheMeta` returns `ErrCacheMiss`, attempt a single `Pull()` retry (hub available) before failing, but guard with context and size limits.
	- When `ErrCacheExpired`, auto-evict + repull once to refresh.
- **Slug correctness & curated mapping:**
	- Ensure curated preset slug list includes `crowdsecurity/bot-mitigation-essentials` (verify file [backend/internal/crowdsec/presets.go](backend/internal/crowdsec/presets.go)).
	- In `findIndexEntry` (hub_sync.go), consider accepting slug without namespace by matching suffix when unique to avoid hub miss.
- **Better guidance and rollback:**
	- In `ApplyPreset` handler, if cache miss occurs after backup creation, ensure rollback succeeds and return `backup` + actionable guidance (e.g., "Pull preset again; cache missing").
	- Add explicit log when rollback triggers due to cache miss, including backup path and slug.
- **TTL visibility:**
	- Add `retrieved_at` and TTL remaining to `GetCachedPreset` and `ListPresets` outputs to help UI warn about expired cache.
- **CSCLI guardrails:**
	- If `cscli` is not found or returns non-zero, include stderr in logs and surface a friendlier hint in the error payload.

### Phase 4 — Tests & Repro Harness
- Add regression tests:
	- `HubService` unit: `Apply` with `ErrCacheMiss` triggers single repull then succeeds (mock HTTP + cache).
- Integration handler: simulate missing cache after pull (evict between pull/apply) → expect repull or clear error and rollback confirmed.
	- Slug normalization test: `bot-mitigation-essentials` (no namespace) maps to `crowdsecurity/bot-mitigation-essentials` when hub index only has the namespaced entry.
	- Backup rollback test: ensure `data/crowdsec` restored on cache-miss failure.
- Extend logging assertions in existing tests to validate `cache_key` and `backup` presence in error responses.

### Phase 5 — Observability & UX polish
- Add a lightweight cache status endpoint or extend `ListPresets` to include `cache_state: [hit|expired|miss]` per slug.
- Frontend (CrowdSecConfig.tsx) follow-up (future PR): surface cache age, "repull" CTA on cache miss, and show backup path when apply fails. (Keep frontend changes out of this fix unless necessary.)

### Phase 6 — Verification Checklist (one pass)
1. `go test ./backend/internal/crowdsec ./backend/internal/api/handlers -run Pull|Apply -v` (or focused test names added above).
2. `cd backend && go test ./...` to ensure no regressions.
3. Manual: pull + apply `crowdsecurity/bot-mitigation-essentials` twice; second apply should hit cache without backup churn.
4. Confirm logs show cache hit and no `cache miss` warnings; backup directory not recreated on cache hit.
5. Validate data directories remain git-ignored (`/data/`, `/backend/data/`, backups under `/data/backups/`).

## Config File Review
- **.gitignore** — already ignores `/data/` and `/data/backups/`; covers cache/backup artifacts (`backend/data/`). No change needed.
- **.dockerignore** — excludes `data/` and `backend/data/`, keeping hub cache/backup out of build context. No change needed.
- **.codecov.yml** — excludes `backend/data/**`; cache/backup coverage not expected. No change needed.
- **Dockerfile** — installs `cscli`; ensure version is recent enough for hub pulls (currently `CROWDSEC_VERSION=1.7.4`). No adjustments required for this fix, but verify the image still includes cscli after build.

## Deliverables
- Patch for cache-miss resilience and slug normalization in `HubService.Apply()` and helpers.
- Error/logging improvements in `ApplyPreset` handler.
- Regression tests covering cache-miss + repull, slug normalization, and rollback behavior.
- Optional: cache-status enrichment for UI consumption (if small and low-risk).
