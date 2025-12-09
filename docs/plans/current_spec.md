# CrowdSec Hub Presets Sync & Apply Plan (feature/beta-release)

## Current State (what exists today)
- Backend: [backend/internal/api/handlers/crowdsec_handler.go](backend/internal/api/handlers/crowdsec_handler.go) exposes `ListPresets` (returns curated list from [backend/internal/crowdsec/presets.go](backend/internal/crowdsec/presets.go)) and a stubbed `PullAndApplyPreset` that only validates slug and returns preview or HTTP 501 when `apply=true`. No real hub sync or apply.
- Backend uses `CommandExecutor` for `cscli decisions` only; no hub pull/install logic and no cache/backups beyond file write backups in `WriteFile` and import flow.
- Frontend: [frontend/src/pages/CrowdSecConfig.tsx](frontend/src/pages/CrowdSecConfig.tsx) calls `pullAndApplyCrowdsecPreset` then falls back to local `writeCrowdsecFile` apply. Preset catalog merges backend list with [frontend/src/data/crowdsecPresets.ts](frontend/src/data/crowdsecPresets.ts). Errors 501/404 are surfaced as info to keep local apply working. Overview toggle/start/stop already wired to `startCrowdsec`/`stopCrowdsec`.
- Docs: [docs/cerberus.md](docs/cerberus.md) still notes CrowdSec integration is a placeholder; no hub sync described.

## Incident Triage: CrowdSec preset pull/apply 502/500 (feature/beta-release)
- Logs to pull first: backend app/GIN logs under `/app/data/logs/charon.log` (or `data/logs/charon.log` in dev) via [backend/cmd/api/main.go](backend/cmd/api/main.go); look for warnings "crowdsec preset pull failed" / "crowdsec preset apply failed" emitted in [backend/internal/api/handlers/crowdsec_handler.go](backend/internal/api/handlers/crowdsec_handler.go). Access logs will also show 502/500 for POST `/api/v1/admin/crowdsec/presets/pull` and `/apply`.
- Routes and code paths: handlers `PullPreset` and `ApplyPreset` live in [backend/internal/api/handlers/crowdsec_handler.go](backend/internal/api/handlers/crowdsec_handler.go) and delegate to `HubService.Pull/Apply` in [backend/internal/crowdsec/hub_sync.go](backend/internal/crowdsec/hub_sync.go) with cache helpers in [backend/internal/crowdsec/hub_cache.go](backend/internal/crowdsec/hub_cache.go). Data dir used is `data/crowdsec` with cache under `data/crowdsec/hub_cache` from [backend/internal/api/routes/routes.go](backend/internal/api/routes/routes.go).
- Quick checks before repro: (1) Cerberus enabled (`feature.cerberus.enabled` setting or `FEATURE_CERBERUS_ENABLED`/`CERBERUS_ENABLED` env) or handler returns 404 early; (2) `cscli` on PATH and executable (`HubService` uses real executor and calls `cscli version`/`cscli hub install`); (3) outbound HTTPS to https://hub.crowdsec.net reachable (fallback after `cscli hub list`); (4) cache dir writable `data/crowdsec/hub_cache` and contains per-slug `metadata.json`, `bundle.tgz`, `preview.yaml`; (5) backup path writable (apply renames `data/crowdsec` to `data/crowdsec.backup.<ts>`).
- Likely 502 on pull: hub cache unavailable or init failed (cache dir permission), invalid slug, hub index fetch errors (`cscli hub list -o json` or direct GET `/api/index.json`), download blocked/size >25MiB, preview/download HTTP non-200, or cache write errors. Handler logs warning and returns 502 with error string.
- Likely 500 on apply: backup rename fails, `cscli` install fails with no cache fallback (if pull never succeeded or cache expired/missing), cache read errors (`metadata.json`/`bundle.tgz` unreadable), tar extraction rejects symlinks/unsafe paths, or rollback after extract failure. Handler writes `CrowdsecPresetEvent` (if DB reachable) with backup path and returns 500 with `backup` hint.
- Validation steps during triage: verify cache entry freshness (TTL 24h) via `metadata.json` timestamps; confirm `cscli hub install <slug>` succeeds manually; if cscli missing, ensure prior pull populated cache; test hub egress with curl to hub index and archive URLs; check file ownership/permissions on `data/crowdsec` and `data/crowdsec/hub_cache`; confirm log lines around warnings for exact error message; inspect backup directory to restore if partial apply.

### Current incident: preset apply returning "Network Error" (feature/beta-release)
- What we see: frontend reports axios "Network Error" while applying a preset. Backend logs do not yet show the apply warning, suggesting the client drops before an HTTP response arrives. Apply path runs `HubService.Apply` in [backend/internal/crowdsec/hub_sync.go](backend/internal/crowdsec/hub_sync.go) with a 15s context; pull uses a 10s HTTP client timeout and does not follow redirects. Axios flags a network error when the TCP connection is reset/timeout rather than when a 4xx/5xx is returned.
- Probable roots to verify quickly:
	- Hub index/preview/archives now redirect to another host; our HTTP client forbids redirects, so FetchIndex/Pull return an error and the handler responds 502 only after the hub timeout. Long hub connect attempts can hit the 10s client timeout, causing the upstream (Caddy) or browser to drop the socket and surface a network error.
	- Runtime image may be missing `cscli` if the release archive layout changed; Dockerfile only moves the binaries when expected paths exist. Without cscli, Apply falls back to cache, but if Pull already failed, Apply exits with an error and no response body. Validate `cscli version` inside the running container built from feature/beta-release.
	- Outbound egress/proxy: container must reach https://hub-data.crowdsec.net (default) from within the Docker network. Missing `HTTP(S)_PROXY`/`NO_PROXY` or a transparent MITM can cause TLS handshake or connection timeouts that the client reports as network errors.
	- TLS/HTML responses: hub returning HTML (maintenance/Cloudflare) or a 3xx/302 to http is treated as an error (`hub index responded with HTML`), which becomes 502. If the redirect/HTML arrives after ~10s the browser may already have given up.
	- Timeout budget: 10s pull / 15s apply may be too tight for hub downloads + cscli install. When the context cancels mid-stream, gin closes the connection and axios logs network error instead of an HTTP code.
- Remediation plan (no code yet):
	- Confirm cscli exists in the runtime image from [Dockerfile](Dockerfile) by running `cscli version` inside the failing container; if missing, adjust build or add a startup preflight that logs absence and forces HTTP hub path.
	- Override HUB_BASE_URL to a known JSON endpoint (e.g., `https://hub-data.crowdsec.net/api/index.json`) when redirects occur, or point to an internal mirror reachable from the Docker network; document this in env examples.
	- Ensure outbound 443 to hub-data is allowed or set `HTTP(S)_PROXY`/`NO_PROXY` on the container; retry pull/apply after validating `curl -v https://hub-data.crowdsec.net/api/index.json` inside the runtime.
	- Consider raising pull/apply timeouts (and matching frontend request timeout) and log when contexts cancel so we return a 504/timeout JSON instead of a dropped socket.
	- Capture docker logs for `charon-debug` during repro; look for `crowdsec preset pull/apply failed` warnings and any TLS/redirect messages from [backend/internal/crowdsec/hub_sync.go](backend/internal/crowdsec/hub_sync.go).

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
