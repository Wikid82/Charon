# CrowdSec Hub Presets Sync & Apply Plan (feature/beta-release)

## Current State (what exists today)
- Backend: [backend/internal/api/handlers/crowdsec_handler.go](backend/internal/api/handlers/crowdsec_handler.go) exposes `ListPresets` (returns curated list from [backend/internal/crowdsec/presets.go](backend/internal/crowdsec/presets.go)) and a stubbed `PullAndApplyPreset` that only validates slug and returns preview or HTTP 501 when `apply=true`. No real hub sync or apply.
- Backend uses `CommandExecutor` for `cscli decisions` only; no hub pull/install logic and no cache/backups beyond file write backups in `WriteFile` and import flow.
- Frontend: [frontend/src/pages/CrowdSecConfig.tsx](frontend/src/pages/CrowdSecConfig.tsx) calls `pullAndApplyCrowdsecPreset` then falls back to local `writeCrowdsecFile` apply. Preset catalog merges backend list with [frontend/src/data/crowdsecPresets.ts](frontend/src/data/crowdsecPresets.ts). Errors 501/404 are surfaced as info to keep local apply working. Overview toggle/start/stop already wired to `startCrowdsec`/`stopCrowdsec`.
- Docs: [docs/cerberus.md](docs/cerberus.md) still notes CrowdSec integration is a placeholder; no hub sync described.

## Goal
Implement real CrowdSec Hub preset sync + apply on backend (using cscli or direct hub index) with caching, validation, backups, rollback, and wire the UI to new endpoints so operators can preview/apply hub items with clear status/errors.

## Backend Plan (handlers, helpers, storage)
1) Route adjustments (gin group under `/admin/crowdsec` in [backend/internal/api/handlers/crowdsec_handler.go](backend/internal/api/handlers/crowdsec_handler.go)):
	- Replace stub endpoint with `POST /admin/crowdsec/presets/pull` → fetch hub item and cache; returns metadata + preview + cache key/etag.
	- Add `POST /admin/crowdsec/presets/apply` → apply previously pulled item by cache key/slug; performs backup + cscli install + optional restart.
	- Keep `GET /admin/crowdsec/presets` but include hub/etag info and whether cached locally.
	- Optional: `GET /admin/crowdsec/presets/cache/:slug` → raw preview/download for UI.
2) Hub sync helper (new [backend/internal/crowdsec/hub_sync.go](backend/internal/crowdsec/hub_sync.go)):
	- Provide `type HubClient interface { FetchIndex(ctx) (HubIndex, error); FetchPreset(ctx, slug) (PresetBundle, error) }` with real impl using either:
	  a) `cscli hub list -o json` and `cscli hub update` + `cscli hub install <item>` (preferred if cscli present), or
	  b) direct fetch of https://hub.crowdsec.net/ or GitHub raw `.index.json` + tarball download.
	- Validate downloads: size limits, tarball path traversal guard, checksum/etag compare, basic YAML validation.
3) Caching (new [backend/internal/crowdsec/hub_cache.go](backend/internal/crowdsec/hub_cache.go)):
	- Cache pulled bundles under `${DataDir}/hub_cache/<slug>/` with index metadata (etag, fetched_at, source URL) and preview YAML.
	- Expose `LoadCachedPreset(slug)` and `StorePreset(slug, bundle)`; evict stale on TTL (configurable, default 24h) or when etag changes.
4) Apply flow (extend handler):
	- `Pull`: fetch index, resolve slug, download bundle to cache, return preview + warnings (missing cscli, requires restart, etc.).
	- `Apply`: before modify, run `backupDir := DataDir + ".backup." + timestamp` (mirror current write/import backups). Then:
	  a) If cscli available: `cscli hub update`, `cscli hub install <slug>` (or collection path), maybe `cscli decisions list` sanity check. Use `CommandExecutor` with context timeout.
	  b) If cscli absent: extract bundle into DataDir with sanitized paths; preserve permissions.
	  c) Write audit record to DB table `crowdsec_preset_events` (new model in [backend/internal/models](backend/internal/models)).
	- On failure: restore backup (rename back), surface error + backup path.
5) Status and restart:
	- After apply, optionally call `h.Executor.Stop/Start` if running to reload config; or `cscli service reload` when available. Return `reload_performed` flag.
6) Validation & security hardening:
	- Enforce `Cerberus` enablement check (`isCerberusEnabled`) on all new routes.
	- Path sanitization with `filepath.Clean`, limit tar extraction to DataDir, reject symlinks/abs paths.
	- Timeouts on all external calls; default 10s pull, 15s apply.
	- Log with context: slug, etag, source, backup path; redact secrets.
7) Migration of curated list:
	- Keep curated presets in [backend/internal/crowdsec/presets.go](backend/internal/crowdsec/presets.go) but add `Source: "hub"` for hub-backed items and include `RequiresHub` true when not bundled.
	- `ListPresets` should merge curated + live hub index when available, mark availability per slug (cached, remote-only, local-bundled).

## Frontend Plan (API wiring + UX)
1) API client updates in [frontend/src/api/presets.ts](frontend/src/api/presets.ts):
	- Replace `pullAndApplyCrowdsecPreset` with `pullCrowdsecPreset({ slug })` and `applyCrowdsecPreset({ slug, cache_key })`; include response typing for preview/status/errors.
	- Add `getCrowdsecPresetCache(slug)` if backend exposes cache preview.
2) CrowdSec config page [frontend/src/pages/CrowdSecConfig.tsx](frontend/src/pages/CrowdSecConfig.tsx):
	- Use new mutations: `pull` to show preview + metadata (etag, fetched_at, source); disable local fallback unless backend says `apply_supported=false`.
	- Show status strip (success/error) and backup path from apply response; surface reload flag and errors inline.
	- Gate preset actions when Cerberus disabled; show tooltip if hub unreachable.
	- Keep local backup + manual file apply as last-resort only when backend explicitly returns 501/NotImplemented.
3) Overview page [frontend/src/pages/Security.tsx](frontend/src/pages/Security.tsx):
	- No UI change except error surfacing when start/stop fails due to hub apply requiring reload; show toast from handler message.
4) Import page [frontend/src/pages/ImportCrowdSec.tsx](frontend/src/pages/ImportCrowdSec.tsx):
	- Add note linking to presets apply so users prefer presets over raw package imports.

## Hub Fetch/Validate/Apply Flow (detailed)
1) Pull
	- Handler: `CrowdsecHandler.PullPreset(ctx)` (new) calls `HubClient.FetchPreset` → `HubCache.StorePreset` → returns `{preset, preview_yaml, etag, cache_key, fetched_at}`.
	- If hub unavailable, return 503 with message; UI shows retry/cached copy option.
2) Apply
	- Handler: `CrowdsecHandler.ApplyPreset(ctx)` loads cache by slug/cache_key → `backupCurrentConfig()` → `InstallPreset()` (cscli or manual) → optional restart → returns `{status:"applied", backup, reloaded:true/false}`.
	- On error: restore backup, include `{status:"failed", backup, error}`.
3) Caching & rollback
	- Cache directory per slug with checksum file; TTL enforced on pull; apply uses cached bundle unless `force_refetch` flag.
	- Backups stored with timestamp; keep last N (configurable). Provide restoration note in response for UI.
4) Validation
	- Tarball extraction guard: reject absolute paths, `..`, symlinks; limit total size.
	- YAML sanity: parse key scenario/collection files to ensure readable; log warning not blocker unless parse fails.
	- Require explicit `apply=true` separate from pull; no implicit apply on pull.

## Security Considerations
- Only allow these endpoints when Cerberus enabled and user authenticated to admin scope.
- Use `CommandExecutor` to shell out to cscli; restrict PATH and working dir; do not pass user-controlled args without whitelist.
- Network egress: if hub URL configurable, validate scheme is https and host is allowlisted (crowdsec official or configured mirror).
- Rate limit pull/apply (simple in-memory token bucket) to avoid abuse.
- Logging: include slug and etag, omit file contents; redact download URLs if they contain tokens (unlikely).

## Required Tests
- Backend unit/integration:
  - `backend/internal/api/handlers/crowdsec_handler_test.go`: success and error cases for `PullPreset` (hub reachable/unreachable, invalid slug), `ApplyPreset` (cscli success, cscli missing fallback, apply fails and restores backup), `ListPresets` merging cached hub entries.
  - `backend/internal/crowdsec/hub_sync_test.go`: parse index JSON, validate tar extraction guards, TTL eviction.
  - `backend/internal/crowdsec/hub_cache_test.go`: store/load/evict logic and checksum verification.
  - `backend/internal/api/handlers/crowdsec_exec_test.go`: ensure executor timeouts/commands constructed for cscli hub calls.
- Frontend unit/UI:
  - [frontend/src/pages/__tests__/CrowdSecConfig.test.tsx](frontend/src/pages/__tests__/CrowdSecConfig.test.tsx): pull shows preview, apply success shows backup path/reload flag, hub failure falls back to cached/local message, Cerberus disabled disables actions.
  - [frontend/src/api/__tests__/presets.test.ts](frontend/src/api/__tests__/presets.test.ts): client hits new endpoints and maps response.
  - [frontend/src/pages/__tests__/Security.test.tsx](frontend/src/pages/__tests__/Security.test.tsx): start/stop toasts remain correct when apply errors bubble.

## Docs Updates
- Update [docs/cerberus.md](docs/cerberus.md) CrowdSec section with new hub preset flow, backup/rollback notes, and requirement for cscli availability when using hub.
- Update [docs/features.md](docs/features.md) to list “CrowdSec Hub presets sync/apply (admin)” and mention offline curated fallback.
- Add short troubleshooting entry in [docs/troubleshooting/crowdsec.md](docs/troubleshooting/crowdsec.md) (new) for hub unreachable, checksum mismatch, or cscli missing.

## Migration Notes
- Existing curated presets remain but are marked as bundled; UI should continue to show them even if hub unreachable.
- Stub endpoint `POST /admin/crowdsec/presets/pull/apply` is replaced by separate `pull` and `apply`; frontend must switch to new API paths before backend removal to avoid 404.
- Backward compatibility: keep returning 501 from old endpoint until frontend merged; remove once new routes live and tested.
