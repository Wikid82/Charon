# Plan: Curated CrowdSec presets falsely report success (Aikido finding)

Type: single PR, `fix:` (medium fix, branch off `development`: `fix/crowdsec-curated-preset-apply`). Not `(security)`-scoped as a changelog category: the finding is low severity, admin-only, a false security state and not an auth bypass or exploitable vulnerability. Subject must stay generic, e.g. `fix: apply curated CrowdSec presets for real and report failures`.
Status: rev 2, supervisor APPROVED. Decision D1 DECIDED by the user: option (a), rename `geolocation-aware` to the honest "GeoIP Enrichment" preset (slug `geoip-enrichment`). Ready for user plan approval, then implementation.
Predecessor: the previous plan in this file (DB tail, #1426/#1436/#1438/#42) is archived at `docs/plans/archive/2026-10-02_db-tail-1426-1436-1438-42_spec.md`.

## 1. Introduction

Aikido AI-pentest finding, low severity, true positive (user-confirmed): "Curated CrowdSec presets falsely report success and leave protections inactive".

`POST /api/v1/admin/crowdsec/presets/apply` for the curated presets `honeypot-friendly-defaults` and `geolocation-aware` returns HTTP 200 `{status:"applied", reload_hint:true, used_cscli:false}`, records a `CrowdsecPresetEvent{Status:"applied"}` and changes nothing on disk or in CrowdSec. The UI toasts "Preset applied via backend (reload required)". The operator believes protections are on; they are not.

Goal: curated presets are genuinely applied (or fail loudly with a recorded failure), the response and audit trail reflect what actually happened, and the UI never claims success it cannot back.

## 2. Research Findings (root cause trace, CLAUDE.md "Context First")

### 2.1 Entry point (frontend)

- `frontend/src/pages/CrowdSecConfig.tsx:469` `handleApplyPreset` calls `applyCrowdsecPreset({slug, cache_key})` (`frontend/src/api/presets.ts:82`), and on 2xx unconditionally toasts `Preset applied via backend` (line 479). The only fallback is `err.response?.status === 501` (line 485) -> `applyPresetLocally` (line 427).
- The curated list shown in the page comes from the backend `GET /admin/crowdsec/presets` (merged with hub entries) plus a static frontend catalog `frontend/src/data/crowdsecPresets.ts` (`CROWDSEC_PRESETS`, line 160 maps it to `source: 'charon-curated'`).

### 2.2 Transformation (backend)

- `backend/internal/crowdsec/presets.go`: three curated presets. `honeypot-friendly-defaults` and `geolocation-aware` have `RequiresHub:false`; `crowdsecurity/base-http-scenarios` has `RequiresHub:true`. The `Preset` struct holds only metadata (slug, title, summary, source, tags, requires_hub). There is no content/definition anywhere on the server.
- `backend/internal/api/handlers/crowdsec_handler.go:1179` `ApplyPreset`: `if preset, ok := FindPreset(slug); ok && !preset.RequiresHub {` creates the event with `Status:"applied"` (error from `DB.Create` ignored), then returns 200 with `status:"applied"`, `backup:""`, `reload_hint:true`, `used_cscli:false`. No file written, no `cscli` call, no reload. Verified: finding is accurate.
- `PullPreset` (line 1090) has the mirror short-circuit: `preview` is only the string `# Curated preset: <title>\n# <summary>`, `cache_key:"curated-<slug>"`, nothing cached.
- The real path `HubService.Apply` (`backend/internal/crowdsec/hub_sync.go:596`) is unreachable for curated slugs: it backs up (`backupExisting`, line 904), prefers `cscli hub install <slug>` (`runCSCLI`, line 885), else `extractTarGz` of a cached hub archive, with rollback on failure. It only works for slugs that exist as hub index entries; the curated slugs `honeypot-friendly-defaults` / `geolocation-aware` are Charon-invented and are NOT hub entries, so even removing the short-circuit would just fail with a cache miss.

### 2.3 Persistence / exit

- `models.CrowdsecPresetEvent` (`backend/internal/models/crowdsec_preset_event.go`): slug, action, status, cache_key, backup_path, error, timestamps. Failure rows already exist for the hub path (`Status:"failed"`, `Error`). The curated path never records failure.
- Existing test `TestApplyCuratedPresetSkipsHub` (`backend/internal/api/handlers/crowdsec_presets_handler_test.go:478`) asserts the buggy behavior (200 + `applied`, with `cscli`-less hub, empty temp dir). It encodes the bug and must be rewritten.

### 2.4 Why the frontend 501 fallback (Option B) cannot be trusted

`applyPresetLocally` (lines 427-457) writes `presetPreview || selectedPreset.content` into whichever file is currently selected in the file editor (`selectedPath ?? files[0]`), via the generic `writeCrowdsecFile`. Consequences:

1. For curated presets the pulled `presetPreview` is the two-line comment stub above; writing it "applies" nothing.
2. `selectedPreset.content` in `crowdsecPresets.ts` is a pseudo-YAML `configs:\n  collections: ...` document that is not a CrowdSec configuration schema. CrowdSec does not read such a file; it would at best be inert and at worst overwrite a real file (e.g. `config.yaml`) the operator had selected.
3. The content is client-supplied and untrusted from the server's point of view, which is the exact thing Aikido's remediation says to avoid.
4. Several referenced items (`crowdsecurity/geo-fencing`, `crowdsecurity/geo-bf`, the `geoip-enricher` as a collection) are not verified hub items (see 2.5).

So "501 and rely on the frontend" would only move the false success into the browser.

### 2.5 Content validity risk

Verified against the live hub index (`hub-cdn.crowdsec.net/master/.index.json`, supervisor review):

- Exist: collections `crowdsecurity/sshd`, `crowdsecurity/caddy`; scenarios `crowdsecurity/http-backdoors-attempts`, `crowdsecurity/http-probing`, `crowdsecurity/ssh-bf`; parsers `crowdsecurity/sshd-logs`, `crowdsecurity/caddy-logs`, `crowdsecurity/geoip-enrich`, and `crowdsecurity/whitelists` (a PARSER, not a postoverflow; the existing frontend content mislabels it).
- Do NOT exist: `crowdsecurity/geo-fencing`, `crowdsecurity/geo-bf`, `crowdsecurity/geoip-enricher`. `geoip-enrich` exists only as a parser, is already pulled in by collections such as `crowdsecurity/linux`, and is enrichment only (it adds country/ASN metadata to events; it blocks nothing).

So today's `geolocation-aware` content is entirely fictional, and its description ("tighten access by region") is not achievable with hub items. Task B0 re-checks type AND existence of every item at implementation time (the hub changes), but the facts above are the baseline.

## 3. Options evaluated

| | Option A: server-side authoritative apply | Option B: return 501, rely on frontend fallback |
| --- | --- | --- |
| Source of truth | Server-side curated definition (list of hub items by type+name), compiled into the binary | Client-supplied content |
| Writes/activates | Backup, `cscli <type> install <name>` per item (validated item names), reload, verify via `cscli ... list` | Writes a pseudo-YAML or a comment stub into a user-selected file |
| Verifies | Yes (post-install list check) | No |
| Audit trail | Server records applied/failed truthfully | Server records nothing; local write is unaudited |
| Meets Aikido remediation | Yes, fully | Only if the fallback is "guaranteed to apply the intended content", which 2.4 shows it is not |
| Cost | Medium (new definition type, installer, tests) | Small, but leaves the product broken |

Recommendation: Option A (long-term fix). Option B is rejected for the reasons in 2.4. Following the "RequiresHub" semantics, curated presets become "Charon-defined bundles of hub items": they need `cscli` (and hub connectivity for `cscli hub update`), so the honest model is that they are applied through the same hub mechanism with a server-owned definition. When the prerequisites are missing the endpoint fails with a clear error and a recorded `failed` event rather than pretending.

## 4. Technical Specifications

### 4.1 Server-side curated definitions (`backend/internal/crowdsec/presets.go`)

Extend, do not replace, the existing types (keep `ListCuratedPresets`/`FindPreset` signatures and the JSON of `Preset` unchanged for `ListPresets` consumers).

- New type `PresetItem struct { Type string; Name string }` where `Type` is one of the allowlisted cscli hub types `collections|parsers|scenarios|postoverflows` and `Name` matches `^[a-z0-9][a-z0-9_.-]*/[a-z0-9][a-z0-9_.-]*$` (hub `author/name`).
- Add unexported/json-excluded field `Items []PresetItem` to `Preset` (`json:"-"` so the list payload is unchanged) populated in `curatedPresets` for each `RequiresHub:false` preset. Add `func (p Preset) Validate() error` (non-empty items, allowlisted types, name regex); `init`-time or test-time enforced via a unit test over `curatedPresets`.
- `RequiresHub:false` is retained as the "defined by Charon, not fetched from the hub index" marker. Doc comment must say so.
- Content (re-verified in Task B0, type and existence):
  - `honeypot-friendly-defaults`: collections `crowdsecurity/sshd`, `crowdsecurity/caddy`; scenarios `crowdsecurity/http-backdoors-attempts`, `crowdsecurity/http-probing`; parser `crowdsecurity/whitelists` (type `parsers`, NOT `postoverflows`). Drop items already pulled in by the collections (B0 checks the collection contents with `cscli collections inspect`). `ssh-bf`, `sshd-logs`, `caddy-logs` are covered by the collections and need not be listed explicitly unless B0 shows they are not.
  - The preset `Summary` (backend) and `description`/`warning` (frontend) must describe exactly what ends up installed (SSH and Caddy log parsing plus brute-force/probing detection, whitelists parser). B0 either trims the item list or rewrites the text so they match; the current "low-noise tuned for tarpits" claim is unsupported by plain hub installs and must be removed or justified.
- **Decision D1 (geolocation-aware): DECIDED by the user, option (a).** The preset is renamed to an honest GeoIP Enrichment preset; the removal path and its follow-up issue are dropped.
  - Slug changes: `geolocation-aware` -> `geoip-enrichment` (the old slug promises region-based protection the preset cannot give, and keeping it would keep the misleading name in URLs, audit rows and logs). Title "GeoIP Enrichment", tags `geo`, `enrichment` (`access-control` dropped), `RequiresHub:false`, `Source:"charon-curated"`.
  - Items: exactly one, parser `crowdsecurity/geoip-enrich` (type `parsers`). B0 re-verifies type and existence.
  - Backend `Summary`: "Enriches CrowdSec log events with GeoIP data (country and ASN). It does not block or allow traffic by region."
  - Frontend `description`: "Enriches CrowdSec log events with GeoIP data (country and ASN). Useful for decision context and dashboards." Frontend `warning`: "Enrichment only: this does not block traffic by region. Use access lists for region rules." Both must match the backend summary semantics (no mention of tightening access, geo-fencing, or requiring a GeoIP database).
  - Existing data: the old slug only ever appears in `CrowdsecPresetEvent` rows (audit trail of the false "applied" no-op). Those rows are left untouched (historical, they are not a config state; no migration, no rewrite; rewriting audit history would be wrong). Stored UI selections: the preset selection is component state in `CrowdSecConfig.tsx`, not persisted; B0 greps `localStorage`/`sessionStorage` use in that page and the Playwright specs to confirm. A client still posting the old slug gets the unknown-slug path (not a curated preset, so it goes to the hub path and fails with the existing cache-miss error, never a false success); no alias is added.
  - Everything that references the old slug is updated: `presets.go`, `presets_test.go` (the `"another preset", "geolocation-aware"` case ~line 49), `crowdsec_presets_handler_test.go`, `frontend/src/data/crowdsecPresets.ts`, `frontend/src/data/__tests__/crowdsecPresets.test.ts` (lines ~11, 57, 134), the i18n files in all five locales if the title or description is localized there (B0 greps `frontend/src/locales/*/translation.json` for `geolocation`/`Geolocation`), and docs mentioning it.
  - Caveat to state in docs: `geoip-enrich` is already pulled in by collections such as `crowdsecurity/linux`, so applying this preset may be a no-op install that still verifies as installed; that is a truthful success.

### 4.2 Applier (`backend/internal/crowdsec/hub_sync.go` or new `curated_apply.go` in the same package)

New method on `HubService` (reuses `Exec`, `DataDir`, `ApplyTimeout`, `copyDir`, `emptyDir`, `sanitizeSlug`, logger):

`func (s *HubService) ApplyCurated(ctx context.Context, preset Preset) (ApplyResult, error)`

**Backup/rollback must NOT reuse `backupExisting`/`rollback`.** Those rename the whole `DataDir` away (it holds `config.yaml` and the live CrowdSec config), which would leave CrowdSec without a config while `cscli` runs and makes `cscli` write into a vanished directory. `ApplyCurated` uses a copy-based backup that leaves `DataDir` in place and intact at all times:

- Backup: `copyDir(DataDir, DataDir+".backup."+ts)` (existing helper, 0700 backup dir). `DataDir` is never renamed, removed, or emptied before success is known. Backup failure -> remove partial backup, return error, nothing installed.
- Where `cscli` writes: with the Charon-managed layout `cscli` is pointed at `DataDir` as its config dir (`config.yaml` `config_paths`: `config_dir`, `data_dir`, `hub_dir`, i.e. `DataDir/hub`, `DataDir/collections|parsers|scenarios|postoverflows` symlinks, and `DataDir/hub/.index.json`). Task B0 confirms the actual `config_paths` for the shipped image and the implementer MUST ensure the backup covers every directory `cscli` mutates; if `hub_dir`/`data_dir` resolve outside `DataDir`, those are added to the backup set (copy each) and to rollback. Paths are `filepath.Clean`ed.
- Symlinks: the existing `copyDir` (`hub_sync.go`, uses `os.ReadDir` + `entry.IsDir()` + `copyFile`) does NOT preserve symlinks. A symlink entry reports `IsDir()==false`, so it goes to `copyFile`, which `os.Open`s the target: a symlink to a file is silently turned into a regular file copy, and a symlink to a directory (the `collections/`, `parsers/`, `scenarios/`, `postoverflows/` item entries are symlinks into the hub dir) makes `io.Copy` fail with "is a directory", so the backup would fail or restore the wrong structure. Task B0 confirms this against the shipped layout. The curated path therefore MUST NOT use `copyDir` as is: add a symlink-preserving variant (check `entry.Type()&fs.ModeSymlink`, `os.Readlink`, `os.Symlink` at the destination; never follow links; reject/skip links whose resolved target escapes the backup root only when copying, preserving the literal link target). Prefer fixing `copyDir` itself for all callers (DRY) if the existing `copyDir` tests and the hub-path fallback stay green; otherwise add `copyDirPreserveLinks` and use it only for the curated path.
- Rollback note: rollback (`emptyDir` then `copyDir`) is briefly non-atomic against a running CrowdSec (it may see a missing or partial hub dir for a moment); accepted because the backup copy is retained.
- Rollback on failure: restore from the copy into the existing `DataDir` (`emptyDir` of the affected mutation dirs, then `copyDir` back); `DataDir` itself is never removed, so it exists and is intact during and after the failed install. The backup copy is kept (its path is returned to the user, as in the hub path) so an operator can recover manually if rollback itself fails; a rollback failure is logged at Error and appended to the returned error.
- Timeout: `applyCtx, cancel := context.WithTimeout(ctx, s.ApplyTimeout)` exactly like `Apply`; every `Exec.Execute` receives `applyCtx`; an expired ctx is checked before each step and triggers rollback.

Behavior (ordered, fail-closed):

1. `preset.Validate()` runs inside `ApplyCurated` on the input it receives (not only on the static catalog at test time); failure returns `ErrInvalidPresetDefinition` (500, nothing touched).
2. Require `s.hasCSCLI(applyCtx)`; otherwise return `ErrCSCLIUnavailable` (new sentinel) without touching disk. Result `Status:"failed"`.
3. Acquire the `HubService` mutex, create the copy-based backup; set `result.BackupPath` only after success.
4. `cscli hub update` (non-fatal warning on failure), then for each item `cscli <type> install <name>` using argv (never a shell; type from allowlist, name by regex). Stop at first failure.
5. Verify each item with `cscli <type> inspect <name> -o json`, exact argv `["cscli", "<type>", "inspect", "<name>", "-o", "json"]` (e.g. `cscli collections inspect crowdsecurity/sshd -o json`). Parse the JSON and require `installed == true` (and, if present, `up_to_date`/`tainted` do not make it fail unless `tainted==true`). Non-JSON output, nonzero exit, missing field, or `installed:false` is a failure. Unit-test fixtures (B0 confirms the field names against the shipped cscli version; the implementer replaces the fixture if they differ):
   - installed: `{"name":"crowdsecurity/sshd","type":"collections","installed":true,"up_to_date":true,"tainted":false,"local":false,"version":"0.2"}`
   - not installed: `{"name":"crowdsecurity/sshd","type":"collections","installed":false}`
   - malformed: `not json`
6. On any failure after step 3: rollback as above, return a wrapped error (`fmt.Errorf("install %s %s: %w", ...)`), `Status:"failed"`, `ErrorMessage` set.
7. On success: `Status:"applied"`, `UsedCSCLI:true`, `ReloadHint:true`, `CacheKey:"curated-"+slug`, `AppliedPreset:slug`.

Reload: CrowdSec needs a reload to load new items. Use the existing reload convention in the handler (`cscli hub reload` is already used after whitelist mutations at `crowdsec_handler.go:2744` as non-fatal). Decision D2: keep `reload_hint:true` (operator/UI triggers restart through existing start/stop controls) and ALSO attempt `cscli hub reload` best-effort; do not report "reloaded"; the contract only claims "installed and verified". If implementer finds that a reliable programmatic reload exists in `CrowdsecExecutor`, calling it is allowed but optional and must not change the success criterion.

Concurrency: guard `ApplyCurated` and `Apply` with a single `sync.Mutex` on `HubService` so two applies cannot interleave backup/rollback.

### 4.3 Handler (`crowdsec_handler.go`)

- `PullPreset`: leave the curated short-circuit but make the preview truthful: return a human-readable list of the items that will be installed (generated from `preset.Items`), `source:"charon-curated"`. Keeps `cache_key:"curated-<slug>"` so the frontend flow is unchanged.
- `ApplyPreset`: replace the no-op branch with `res, err := h.Hub.ApplyCurated(ctx, preset)` and share the existing success/failure tail used by the hub path. Extract the duplicated event-recording and response-building code into two small helpers (`recordPresetEvent(slug, status, res, err)` and `respondApplySuccess`/`respondApplyFailure`), per the DRY rule (this is the second occurrence). Event-recording errors from `DB.Create` are logged at Warn instead of silently discarded (`_ =`), for both paths.
- Failure event: `CrowdsecPresetEvent{Slug, Action:"apply", Status:"failed", CacheKey:"curated-"+slug, BackupPath, Error: err.Error()}`.
- Status mapping for the curated path (order matters): first `errors.Is(err, crowdsec.ErrCSCLIUnavailable)` -> 503 `{"error":"CrowdSec CLI is not available; curated presets require cscli"}` (handled BEFORE the hub path's `strings.Contains(errorMsg, "cscli unavailable")` rewriting so that message guidance is not mangled); then `mapCrowdsecStatus(err, http.StatusInternalServerError)` (deadline/cancel -> 504, otherwise 500). `ErrInvalidPresetDefinition` -> 500. All failure responses include `backup` when a backup exists (rollback already restored it; keep the existing field semantic) and `cache_key`.
- Never leak raw command output or absolute paths beyond what the hub path already returns; error strings are wrapped Go errors (no stdout dumps).

### 4.4 API contract changes

| Case | Before | After |
| --- | --- | --- |
| Curated preset, cscli present, all items installed and verified | 200 `applied` (nothing done) | 200 `{status:"applied", backup:"<path>", reload_hint:true, used_cscli:true, cache_key:"curated-<slug>", slug}` |
| Curated preset, cscli missing | 200 `applied` | 503 `{error, cache_key}`; event `failed` |
| Install/verify failure | 200 `applied` | 500 `{error, backup?, cache_key}`; rolled back; event `failed` |
| Timeout | n/a | 504 |
| Unknown slug / hub preset path | unchanged | unchanged |

`ApplyCrowdsecPresetResponse` (frontend) is structurally unchanged. No 501 is returned by the server for curated presets. No DB schema change; `AutoMigrate` untouched.

### 4.5 Frontend

- `CrowdSecConfig.tsx` `handleApplyPreset`: keep 501 fallback only for hub presets, never for slugs with `source === 'charon-curated'` (the server is now authoritative; for curated presets a 501 would be a contract violation and shows an error). Success toast text depends on response: when `res.status === 'applied'` show applied (+ reload note); any other status shows an error toast, never success. Add handling for 503 curated (cscli missing) that is distinct from the "hub unavailable" message: show the server error text.
- Frontend `warning`/`description` text for every remaining curated entry is aligned with what the backend actually installs (B0 output) and with the Decision D1 outcome (4.1); the current `geolocation-aware` entry is renamed to `geoip-enrichment` with the D1 texts, and `honeypot-friendly-defaults` low-noise claims are removed or corrected.
- `applyPresetLocally`: stop using pseudo-YAML `crowdsecPresets.ts` content for curated slugs. Delete the now-dead pseudo-YAML `content` of the curated entries (`honeypot-friendly-defaults`, `geoip-enrichment`; entries only stay if the UI still needs description/warning text from this file, otherwise the server list is the sole source) from `frontend/src/data/crowdsecPresets.ts` if nothing else needs them; the server list is the single source. Check `CROWDSEC_PRESETS` usages (`CrowdSecConfig.tsx:160`, `data/__tests__/crowdsecPresets.test.ts`, `frontend/src/data/securityPresets.ts` is unrelated) and remove dead code per CLAUDE.md CLEAN. If `bot-mitigation-essentials` is the only remaining entry and it is a hub preset served by the backend, the file may be deleted entirely along with its test.
- i18n: only if new strings are added; update `frontend/src/locales/*/translation.json` for all five locales (en, de, es, fr, zh).

### 4.6 Error handling and edge cases

- cscli present but hub unreachable: `cscli hub update` warns; install of already-present items may still succeed; verification decides.
- Item already installed: `cscli install` is idempotent (returns 0 or "already installed"); treat as success iff verification passes.
- Partial install then failure: the copy-based rollback (4.2) restores every directory `cscli` mutates (config dir, hub dir, data dir as resolved in B0) while `DataDir` stays in place. If rollback itself fails, the error says so, the backup path is returned, and the failure event is still recorded.
- Cerberus disabled: unchanged 404. `Hub == nil`: unchanged 503.
- Double-click / concurrent applies: mutex (4.2); frontend already disables the button via `isApplyingPreset`.
- Backup accumulation: unchanged behavior (same as hub path); see F3.

## 5. Priorities & Ordering (CLAUDE.md "Findings Triage & Issue Tracking")

Source: Aikido AI-pentest finding (external scanner, not a user-submitted GitHub issue); user-confirmed true positive, severity low. Rules applied: our own finding -> bug fix before any `feat`; nothing here is critical/high, so no emergency preemption and no separate-PR carve-out.

| Rank | Item | Kind | Severity | Decision |
| --- | --- | --- | --- | --- |
| 1 | Curated presets no-op / false success (this PR) | `fix:` | low | Do now, single PR off `development` |
| 2 | F1: `applyPresetLocally` writes unvalidated client content into an arbitrary selected CrowdSec file | `fix:` | low-medium | Narrowed in this PR for curated slugs (4.5). Residual hub-preset fallback behavior: file as issue, schedule after this PR |
| 3 | F2: the HUB path (`HubService.Apply`) still uses `backupExisting`, which renames the entire `DataDir` (including `config.yaml` and `hub_cache`) while CrowdSec runs, and its rollback only covers `DataDir`. The CURATED path is fixed in this PR (copy-based backup, 4.2) and is no longer deferred; only the hub path remains | `fix:` | low-medium | File as issue for the hub path (to verify), after this PR; reuse the copy-based helper |
| 4 | F3: unbounded `DataDir.backup.*` accumulation from repeated applies | `chore:` | low | File as issue |
| 5 | F4: `crowdsec_handler.go` is ~2800 lines; preset handlers should move to `crowdsec_presets_handler.go` | `refactor:` | low | File as issue; do not widen this PR |
| 6 | F5: Playwright `tests/security/crowdsec-config.spec.ts` preset tests are tolerant "may not be implemented" skips that would not have caught this bug | `test:` | low | Partially addressed here (new spec, section 6.3); the old tolerance cleanup is a follow-up issue |

Per CLAUDE.md these out-of-scope findings (F2 to F5, and the residual of F1) are filed automatically as GitHub issues by the orchestrator, none security-sensitive (no private advisory needed). The GitHub MCP server failed to connect in the planning session, so filing must use `gh issue create`.

## 6. Test Plan

### 6.1 Backend unit tests (TDD, written first)

`backend/internal/crowdsec` (new `curated_apply_test.go`, reuse the fake `CommandExecutor` pattern in `hub_pull_apply_test.go`):

- Success: fake exec records `cscli version`, `hub update`, one `install` per item, one `inspect <type> <name> -o json` verification per item (exact argv per 4.2 step 5); asserts argv exactly (no shell), `Status:"applied"`, `UsedCSCLI`, backup path created, `ReloadHint`.
- cscli missing (`version` errors): returns `ErrCSCLIUnavailable`, `Status:"failed"`, no backup dir created, no install calls.
- Install failure on item N: the fake executor's `install` hook asserts, at call time, that `DataDir` exists and its sentinel file (`config.yaml`) is intact (proves no rename away); after the failure the test asserts `DataDir` exists, the sentinel and any files the fake install added/changed are restored to the original contents, `Status:"failed"`, error wraps cause, backup copy still present.
- Symlink rollback test: seed `DataDir` with `hub/collections/crowdsecurity/sshd.yaml` plus `collections/sshd.yaml -> ../hub/collections/crowdsecurity/sshd.yaml` (relative link) and a dangling link; run a failing install that mutates both; assert after rollback `os.Lstat` shows symlinks (mode `ModeSymlink`) with identical `Readlink` targets, regular files byte-identical, no dereferenced copies. Also a unit test for the symlink-preserving copy itself (file link, dir link, dangling link, nested).
- Same assertions for verification failure and for a timeout in the middle of install (DataDir exists and is intact during and after).
- Verification fixtures (4.2 step 5): installed JSON passes; `installed:false`, malformed JSON, nonzero exit, missing field each fail; assert exact argv `cscli <type> inspect <name> -o json`.
- `Validate` runs on the input of `ApplyCurated`: calling it with an invalid/hand-built `Preset` (bad type `../x`, bad name `a b`, empty items) returns `ErrInvalidPresetDefinition` with no exec calls and no backup, independent of the static catalog.
- Timeout: `ApplyTimeout` set tiny via the existing field; a fake executor that blocks until ctx is done yields a deadline error and rollback.
- Verification failure (install exit 0 but `inspect -o json` reports `installed:false`): treated as failure and rolled back.
- Backup failure: no install attempted.
- Timeout via cancelled ctx: error satisfies `errors.Is(err, context.DeadlineExceeded)` or Canceled.
- Concurrency: two parallel `ApplyCurated` calls serialize (run with `-race`).
- `presets_test.go`: every `RequiresHub:false` preset passes `Validate()`; invalid type / name (`../x`, `a b`, `;rm`) rejected; `Items` not serialized in JSON of `Preset`.

`backend/internal/api/handlers/crowdsec_presets_handler_test.go`:

- Rewrite `TestApplyCuratedPresetSkipsHub` into: success returns 200 only when the fake exec succeeds, `used_cscli:true`, backup non-empty, event row `applied`; assert fake exec saw installs (proves not a no-op).
- New: cscli unavailable -> 503, event row `status=failed`, `error` non-empty, response has no `applied`.
- New: install failure -> 500 with `backup`, event `failed`.
- New: `DB.Create` failure is logged, request outcome unchanged (use closed DB / missing table) for both curated and hub paths.
- Update `TestPullCuratedPresetSkipsHub` to assert the preview lists the items.
- Regression: hub path (`TestApply...` existing tests) unchanged and green after the helper extraction.

Coverage: patch coverage target 100% for new code; overall >= 85% (`scripts/go-test-coverage.sh`). GORM scan not required (no model/query changes beyond existing `Create`), but run `./scripts/scan-gorm-security.sh --check` anyway if any `DB` call shape changes.

### 6.2 Frontend unit tests (Vitest, `frontend/src/pages/__tests__/CrowdSecConfig*.test.tsx`)

- Curated apply success: toast success only when `status === 'applied'`; applyInfo shows backup and `used_cscli`.
- Curated apply non-2xx (parametrized over 500, 503, 504, 501, 400): error toast with server message, no success toast, `applyPresetLocally` NOT called, no `writeCrowdsecFile` call. The contract is: ANY non-2xx on a curated slug never calls `applyPresetLocally`.
- Curated apply 501 (contract violation): error shown, still no local write.
- Hub preset 501: existing local fallback behavior preserved (regression).
- Non-`applied` status in 200 response never shows a success toast.
- Remove/update `data/__tests__/crowdsecPresets.test.ts` consistent with 4.5. Frontend coverage >= 85% (`scripts/frontend-test-coverage.sh`); `npm run type-check`.

### 6.3 Playwright (single spec, firefox only, targeted)

Add `tests/security/crowdsec-preset-apply.spec.ts` (replace nothing; leave the tolerant spec alone). Uses `page.route` to stub `POST **/admin/crowdsec/presets/apply`:

1. Stub 503 `{error:"CrowdSec CLI is not available..."}`: select "Honeypot Friendly Defaults", click Apply; expect an error toast/message and NO "applied" success toast.
2. Stub 200 `{status:"applied", used_cscli:true, backup:"...", reload_hint:true}`: expect success toast with reload note.
3. Assert no request to the generic file-write endpoint is made in cases 1 and 2 (no local fallback for curated presets).

Run: `cd /projects/Charon && npx playwright test tests/security/crowdsec-preset-apply.spec.ts --project=firefox` (foreground). Full-suite/cross-browser is CI-only. Real-cscli E2E against the container is optional and only if the E2E container image ships `cscli` (implementer to check; otherwise covered by unit tests).

## 7. Implementation Plan (phases mapped to commits)

Task B0 (before any code): verify every item name in 4.1 against the live hub index (`cscli hub list -a` in the E2E/dev container, or `https://hub-data.crowdsec.net/.index.json`). Record the verified list in the PR description. Check type AND existence of every item (`cscli <type> inspect <name> -o json` against the shipped cscli, or the hub index `hub-cdn.crowdsec.net/master/.index.json`); confirm the JSON field names used by the 4.2 fixtures; confirm `config_paths` (config/hub/data dirs) of the shipped image for the backup set; align the `honeypot-friendly-defaults` summary/warning with what is installed. Decision D1 is already decided (option a); B0 only re-verifies that `crowdsecurity/geoip-enrich` exists as a parser, confirms the localStorage/locale greps from 4.1, and confirms symlink behavior of `copyDir` (4.2).

## 8. Commit Slicing Strategy

Decision: ONE PR (`fix/crowdsec-curated-preset-apply` off `development`, PR into `development`), ordered logical commits. Each commit builds and passes its gate.

| # | Commit | Scope / files | Depends on | Validation gate |
| --- | --- | --- | --- | --- |
| 1 | `test: add failing specs for curated CrowdSec preset apply` | ONLY `tests/security/crowdsec-preset-apply.spec.ts` as `test.fixme` (no backend or other files; backend tests land with commit 3). | none | spec file lints; `npx playwright test <spec> --project=firefox` reports fixme/skipped |
| 2 | `refactor: extract crowdsec preset event/response helpers and add curated item definitions` | `backend/internal/crowdsec/presets.go` (+`PresetItem`, `Items`, `Validate`), `presets_test.go`, handler helper extraction (`recordPresetEvent`, response helpers), DB.Create error logging; NO behavior change to curated apply yet | 1 | `cd backend && go build ./... && go test ./internal/crowdsec/... ./internal/api/handlers/...`; existing tests green |
| 3 | `fix: apply curated CrowdSec presets via cscli and record failures` | `hub_sync.go`/`curated_apply.go` (`ApplyCurated`, `ErrCSCLIUnavailable`, mutex, symlink-preserving copy, `geoip-enrichment` rename in `presets.go`/tests), `crowdsec_handler.go` (ApplyPreset + PullPreset preview), rewritten/new handler and package tests (section 6.1) | 2 | `go test -race ./internal/crowdsec/... ./internal/api/handlers/...`; `make lint-fast`; backend coverage script >= 85%; local patch report |
| 4 | `fix: stop reporting curated preset success the server did not confirm` | `CrowdSecConfig.tsx`, `data/crowdsecPresets.ts` (+ test) cleanup, `api/presets.ts` types if needed, i18n (5 locales) if new strings, Vitest tests (6.2) | 3 | `npm run type-check`, `npx vitest run` on touched tests, `npm run build`, frontend coverage >= 85% |
| 5 | `test: enable curated preset apply e2e and update docs` | un-fixme the Playwright spec; `docs/features.md` / `docs/features/` CrowdSec preset note (curated presets require `cscli`, are verified, failures are reported); `ARCHITECTURE.md` only if the CrowdSec integration section describes preset apply (check; likely a one-line note) | 4 | `npx playwright test tests/security/crowdsec-preset-apply.spec.ts --project=firefox`; `lefthook run pre-commit`; docs sync rule (no edits under `docs-site/docs/`) |

Definition of Done for the PR (CLAUDE.md): targeted Playwright (firefox) first; GORM scan only if models/queries change (not expected); `bash scripts/local-patch-report.sh`; CodeQL/Trivy are deferred to CI (this is a `fix:` with no new feature surface); `lefthook run pre-commit`; staticcheck clean; backend and frontend coverage >= 85%; type-check; both builds; no debug leftovers.

Rollback / contingency:
- The PR is revertable as a unit; no schema change, so revert has no data impact.
- If Task B0 shows no verifiable hub items for a curated preset (for example `geoip-enrich` is gone from the hub), stop and ask the user rather than shipping a no-op preset.
- If `cscli` is absent from some deployment shapes, curated presets will return 503 by design (honest failure); the UI message must tell the operator why. Consider (follow-up, not this PR) hiding the Apply button for curated presets when `cscli` is known unavailable.
- If reliable post-install reload is not feasible in this PR, ship with `reload_hint:true` semantics (explicitly "installed and verified, restart required").

## 9. Acceptance Criteria

1. Applying a curated preset with `cscli` available installs the server-defined hub items, verifies them, creates a backup and returns 200 `applied` with `used_cscli:true` and a non-empty `backup`.
2. Without `cscli`, or on any install/verify/backup/timeout failure, the endpoint returns a non-2xx error, never `applied`, rolls back, and persists a `CrowdsecPresetEvent` with `Status:"failed"` and `Error`.
3. No code path returns `applied` unless something was installed and verified. The old no-op branch is gone.
4. The frontend never shows a success toast for non-`applied` or error responses and never writes client-supplied curated content to a CrowdSec file; hub-preset 501 fallback unchanged.
5. Curated item definitions are server-side, validated (allowlisted types, name regex, argv execution, no shell) and verified against the real hub (Task B0); Decision D1 is implemented as decided (option a: `geoip-enrichment`, enrichment-only wording) and no preset or `warning` text over-promises. Curated apply never renames or empties `DataDir`; rollback keeps it intact.
6. `TestApplyCuratedPresetSkipsHub` no longer asserts the buggy behavior; new tests in 6.1/6.2/6.3 pass; coverage thresholds met; DoD in section 8 satisfied; CI green.
