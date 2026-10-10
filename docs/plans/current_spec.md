# CrowdSec Follow-ups Batch: #1479, #1481, #1526, #1528

Status: DRAFT rev 2 (supervisor changes applied) for re-review. Uncommitted. Separate from `docs/plans/current_spec.md`.
Base: `origin/development`, which already includes #1525 (113f646c). All line numbers below were re-verified against it; re-verify again immediately before implementation.

## 1. Introduction

Four open issues, all authored by the maintainer (none is user-submitted), all low severity:

| Issue | Type | Area | Summary |
|---|---|---|---|
| #1528 | fix / product decision | backend (+ small frontend) | F1 archive entry check too broad, F3 absolute server paths in CrowdSec responses, F5 credentials YAML files still listed/readable in the editor |
| #1479 | hardening | backend `network` pkg | `WithAllowedDomains` option stores a list nothing reads |
| #1481 | cleanup | backend `crowdsec` pkg | production hub URL validator carries test-only host allowances |
| #1526 | test | Playwright | tolerant CrowdSec specs can pass without asserting the feature works |

### Is #1479 CrowdSec-specific?
No. The option lives in the generic `backend/internal/network/safeclient.go` (`ClientOptions.AllowedDomains`, `WithAllowedDomains`). The `crowdsec` label is there only because its single production caller is `newHubHTTPClient` in `backend/internal/crowdsec/hub_sync.go`. It is included because it was found in the same investigation as #1481 (#1476) and the right fix for #1481 (one validator, no test allowances in production) determines the right fix for #1479 (remove the unused option so `validateHubURL` is the only allowlist).

## 2. Research Findings

### 2.1 #1479 `WithAllowedDomains`
- Stored in `ClientOptions.AllowedDomains`; `safeDialer` and `validateRedirectTarget` never read it. Its doc comment ("only these domains will be allowed") is false: a latent footgun.
- Only production caller: `newHubHTTPClient` (hub_sync.go). That client uses default `MaxRedirects` 0, so redirect enforcement is moot. `validateHubURL` already holds its own copy of the same three hosts, so two lists exist and neither is authoritative.
- **Three test references** in `backend/internal/network/safeclient_test.go` must change, otherwise compilation breaks when the field is removed:
  1. `TestSafeDialer_AllowedDomains` (~L153-175): sets `AllowedDomains` on options and asserts nothing about them. Dead test. **Delete.**
  2. `TestNewSafeHTTPClient_WithAllowedDomains` (~L291): **Delete.**
  3. ~L849 (`WithAllowedDomains("example.com", "api.example.com")` inside a larger test): **Edit** to drop the option only; keep the rest of the test.
- Redirect/option parity (the issue's second ask): `safeDialer` and `validateRedirectTarget` both go through `opts.policy()` / `AddressPolicy.Blocked`, so `AllowLocalhost`/`AllowRFC1918`/`AllowCGNAT` already behave identically. Pin that with one table test running the same host/option matrix through both rather than changing behavior.

### 2.2 #1481 test-only allowances
- `validateHubURL` (hub_sync.go ~L106-160) accepts at any scheme (http included): `localhost`, `127.0.0.1`, `::1`, `*.example.com`, `*.example`, `example.com`, `*.local`, `test.hub`, before the HTTPS + three-host allowlist check (`hub-data.crowdsec.net`, `hub.crowdsec.net`, `raw.githubusercontent.com`).
- Production impact (for a docs/changelog note): an operator-set `HUB_BASE_URL`/`HUB_MIRROR_BASE_URL` using http, localhost or `.local` already fails at the dial layer (`network.NewSafeHTTPClient` blocks loopback/private), so tightening the validator changes the error text and where it is raised, not what works. No supported configuration breaks. Mention in release notes in one sentence.
- Existing seam: package var `hubAllowLoopback` (~L95), toggled by `setHubAllowLoopbackForTest` (hub_sync_test.go ~L2252) and exercised by `TestNewHubHTTPClient_LoopbackSeam` (~L2258).
- **Only 2 `httptest.NewServer` uses** exist in the crowdsec package tests: `hub_sync_test.go` ~L106 and ~L2259 (the latter is inside `TestNewHubHTTPClient_LoopbackSeam`). Everything else uses a stub `RoundTripper` with fake hostnames.
- Test reference sizing: roughly 100-110 hits of `test.hub`/`example.com`/`.local`/`hub.example` across `hub_pull_apply_test.go`, `hub_sync_test.go`, `hub_sync_raw_index_test.go`, and the handler package (`crowdsec_preset_handler_test.go`, `crowdsec_pull_apply_integration_test.go`); re-grep before starting and do not rely on exact figures. Handler tests are in `package handlers`, cannot reach an unexported field, and embed hosts in index JSON fixtures such as `download_url`/`preview_url`.
- `hubBaseCandidates()` (~L215) builds `[HubBaseURL, MirrorBaseURL, defaultHubMirrorBaseURL, defaultHubBaseURL]` and de-duplicates with `uniqueStrings`. The defaults are `https://hub-data.crowdsec.net` and `https://raw.githubusercontent.com/crowdsecurity/hub/master`. If migrated tests reuse those same hosts for the primary/mirror roles, candidates collapse via `uniqueStrings` and fallback-order tests silently change meaning.

### 2.3 #1528
- **F1**: `extractTarGz` (hub_sync.go ~L946) uses `strings.HasPrefix(cleanName, "..")`, which also rejects legitimate names like `..hidden.yaml`. The containment check below it (`filepath.Rel`, ~L955) already uses `".."+separator`, so the first check is redundant-but-stricter.
- **F5**: `isReadable` (crowdsec_files.go ~L79) hides `bouncer_key`, `*.db*` and engine-owned trees, but `local_api_credentials.yaml` and `online_api_credentials.yaml` are only in `protectedWriteNames` (write-protected; still listed and readable). Real error messages today: reads of a hidden path return `invalid path` (`resolveReadable` first check ~L128, and the resolved-path re-check ~L149); writes of a hidden path return `file cannot be edited` (`resolveWritable` ~L167-169, `errFileCannotBeEdited`). Mixed-case names: `isReadable` lowercases the base only for the `bouncer_key` and `*.db*` checks, so once the credentials names are added to its deny rules (using the same lowercase comparison) mixed-case variants become hidden too, a deliberate tightening. Nothing in `frontend/src` or the Playwright specs references the credentials files; `docs/troubleshooting/crowdsec.md` (~L644) stays valid.
- **F5 scope limit: `ExportConfig` is not covered.** `ExportConfig` (crowdsec_handler.go ~L810-880) walks the whole `DataDir` and tars it with no `isReadable` filter (it only skips symlinks), so credentials files and `bouncer_key` remain downloadable by an authenticated admin through the export endpoint. The "consistent with `bouncer_key`" argument therefore applies to the editor list/read surface only, not to export.
- **F3 leak sites** (all admin-authenticated; absolute server paths or archive entry names reach the client):
  1. File write `backup` field: `crowdsec_files.go` ~L415 (from `crowdsec.BackupFile`, kind `filebackup`, a directory).
  2. Import `backup` field: `crowdsec_handler.go` ~L727 (from `Snapshot`, kind `backup`, a directory).
  3. Preset apply: `crowdsec_preset_handler.go` ~L353 and ~L365 (`res.BackupPath`).
  4. Acquisition update: `crowdsec_handler.go` ~L2030 (`backupPath`, a **file** named `acquis.yaml.backup.<ts>`, not a directory).
  5. `rollbackFailure` (crowdsec/backup.go ~L291-293): message `"... (rollback failed: %v; backup retained at %s)"` flows via `err.Error()` into the preset-apply and curated-apply JSON error responses.
  6. `extractTarGz` errors (hub_sync.go ~L947, ~L956 and the mkdir/`destPath` errors in the loop): reach the client through import's `extraction failed: %v` (crowdsec_handler.go ~L715).
  7. `ExportConfig` error path: `gin.H{"error": err.Error()}` (~L877) can carry filesystem paths from the walk.
  8. Frontend print sites in `frontend/src/pages/CrowdSecConfig.tsx`: ~L474 (`Backup stored at ...`), ~L493 (`Backup created at ...`), ~L1116 (`t('crowdsecConfig.presets.backup'): {applyInfo.backup}`). Existing vitest `frontend/src/pages/__tests__/CrowdSecConfig.curatedApply.test.tsx` (~L130 stubs `/tmp/backup.tar.gz`, ~L170 asserts the absolute path) must change.
  9. Playwright stub fixtures carrying absolute paths: `tests/security/crowdsec-hub-preset-apply.spec.ts` ~L110, `tests/security/crowdsec-file-editor.spec.ts` ~L89, and `tests/security/crowdsec-preset-apply.spec.ts` (`/app/data/backups/...`).
  Backup directory kinds are `<DataDir>.backup.<ts>` (Snapshot) and `<DataDir>.filebackup.<ts>` (single-file edits); the acquisition backup is a plain file next to `acquis.yaml`. No client uses the value programmatically.
- Docs: `docs/api` has no mention of the `backup` field (checked); `ARCHITECTURE.md` mentions CrowdSec only as directory layout, so it needs no change (confirm no-op at implementation). Locale files carry `crowdsecConfig.presets.backup` ("Backup") in each language under `frontend/src/locales/*/translation.json`; any new UI string needs all locale files updated (en plus the existing languages).

### 2.4 #1526 tolerant specs
Files under `tests/security/`: `crowdsec-dashboard.spec.ts`, `crowdsec-decisions.spec.ts`, `crowdsec-diagnostics.spec.ts`, `crowdsec-console-enrollment.spec.ts`, `crowdsec-import.spec.ts`, and the non-preset cases of `crowdsec-config.spec.ts` (back navigation, config file list/content, export, console enrollment, status indicators).
Counts depend on the pattern; for exactness use two named patterns on `origin/development`: (a) `catch(() => false)` only: dashboard 10, decisions 15, diagnostics 3, console-enrollment 9, import 0, config 11; (b) the broader "tolerance" pattern (adds "may not be", "not implemented", annotation pushes) gave higher numbers in my first pass and 13/15/18/17/1/12 in the supervisor's pass. Do not rely on either number; the gate in section 6 (Bz) defines the final, exact pattern set.
Two styles:
1. UI specs (`dashboard`, `decisions`, `config`, parts of `console-enrollment`) that `goto('/security/crowdsec')` against the real backend and guard on visibility. Convert to stubbed-API specs following `crowdsec-preset-apply.spec.ts` (`page.route` + `route.fulfill({ json })`, `loginUser`, `waitForLoadingComplete`).
2. API-contract specs (`diagnostics`, most of `import`, parts of `console-enrollment`) that call `request.get/post('/api/v1/admin/crowdsec/...')` against the live backend and accept multiple statuses because cscli/LAPI are absent in the E2E container (`crowdsec-import` hides this behind a multi-status `expect([...]).toContain(status)` assertion). Convert to (a) strict assertions on a status the E2E container deterministically produces, or (b) stubbed UI tests plus Go handler tests for the real status mapping.
Strict assertions may expose real bugs (see section 8).

## 3. Decisions

### 3.1 F5: recommendation (needs user confirmation)
**Hide `local_api_credentials.yaml` and `online_api_credentials.yaml` from editor list and read** by adding them to the `isReadable` deny rules next to `bouncer_key` (same lowercase comparison), then delete `protectedWriteNames` and its branch in `isWritable` (redundant: `isWritable` is a subset of `isReadable`).

Behavior change to state in docs: previously readable but write-protected, now hidden entirely (list omits them; read returns `invalid path`; write returns `file cannot be edited`, because `resolveWritable` checks `isReadable` first; new tests must assert these two real messages, not one generic message). Mixed-case names are now hidden too.

Rationale (reworded; this is information-level hardening, not a closed exposure): the editor is a browser surface where plaintext secrets leak through screen shares, screenshots/support captures and the client-side query cache; novices have no need to read or edit these generated files; advanced operators still have disk access. Scope limit, stated plainly: this protects the editor surface only. Admin config export still includes these files and `bouncer_key` (section 2.3), so F5 does not remove the secrets from anything an admin can download. See Open Question on export.

Alternative rejected: list but mask values (extra UI/parser complexity and a new leak path if masking misses a field).

### 3.2 F3: recommendation
Return a **backup name**, not a path: a single helper `backupID(path string) string` (in `backend/internal/crowdsec/backup.go`) returning `filepath.Base`, used at every site in 2.3 items 1-4. It is a name, not an opaque token: the three kinds are `*.backup.*` (directory), `*.filebackup.*` (directory) and `acquis.yaml.backup.*` (file). The absolute path stays in server logs only (already `SanitizeForLog`ged).
- Errors: `rollbackFailure` keeps the cause but replaces the path with the backup name plus the hint "see server logs for the backup location"; `extractTarGz`/import errors become fixed messages (for example `archive contains an unsafe path`, `archive extraction failed`) with the entry name/dest path logged sanitized; `ExportConfig` returns a fixed `failed to export configuration` and logs the error.
- UI keeps manual recovery possible: text such as "Backup created: <name> (next to the CrowdSec data directory)" in the three print sites, via a new i18n key (all locales) rather than hard-coded English; docs pointer in `docs/troubleshooting/crowdsec.md` explains where backups live.
- Field name `backup` is kept. Acceptance criterion: **no absolute path appears in any CrowdSec API response, including error text** (asserted by handler tests with a regex over response bodies for a leading `/` path or the data-dir prefix).

### 3.3 F1
Reject an archive entry only if any path component equals `..` or the path is absolute; keep the `Rel`-based containment check as backstop. Tests: allowed `..hidden.yaml`, `a/..b/c.yaml`, `foo..bar`; rejected `../x`, `a/../../x`, `/etc/x`, NUL. No exploit detail in public text.

### 3.4 #1479: remove vs enforce
Remove `WithAllowedDomains`, `ClientOptions.AllowedDomains` (and its default), the caller in `newHubHTTPClient`, and the three test references in 2.1. Rationale: the only caller has a better-fitting validator, redirects are disabled for that client, and a second list would drift.

### 3.5 #1481: seam design and migration
- Production `validateHubURL`: https only, host in the three-host allowlist.
- Test seam: unexported `validateURL func(string) error` on `HubService` (nil means `validateHubURL`), both call sites (`fetchIndexHTTPFromURL` ~L453, `fetchWithLimitFromURL` ~L754) go through one `s.checkURL(target)`.
- Only the `hub_sync_test.go` ~L106 `httptest.NewServer` test needs the `validateURL` seam (permissive validator) plus a `network.WithAllowLocalhost()` `HTTPClient`. The ~L2259 server lives inside `TestNewHubHTTPClient_LoopbackSeam`, which is deleted together with the var.
- **`hubAllowLoopback` and `setHubAllowLoopbackForTest` are deleted. `TestNewHubHTTPClient_LoopbackSeam` is deleted** (it only tests the removed var); replace with a small test that `newHubHTTPClient` rejects loopback (the first half of the old test, `require.Error`).
- `validateHubURL` tests flip to assert rejection: `localhostURLs` loop (~L996-1010), the other table tests in the range ~L969-1044, and the table at ~L1645; add acceptance cases for the three https hosts and rejection for http on an allowlisted host.
- Stub-transport migration: the roughly 100-110 hits above (including `download_url`/`preview_url` inside index JSON fixtures in `crowdsec_pull_apply_integration_test.go`, ~L40-50, and `crowdsec_preset_handler_test.go`) become allowlisted https hosts. Use one distinct allowlisted host per role so `uniqueStrings` in `hubBaseCandidates()` does not collapse candidates: primary `https://hub-data.crowdsec.net` (default), mirror `https://raw.githubusercontent.com/crowdsecurity/hub/master` (default), and `https://hub.crowdsec.net` for tests that need an extra distinct base. Tests that previously set `HubBaseURL = "http://test.hub"` use `https://hub.crowdsec.net`; tests whose assertions depend on candidate ordering get an explicit check.
- Estimated effort: mechanical host mapping across roughly 100-110 hits (re-grep first) plus one seam-using test; a single commit. Re-grep all line numbers in this spec right before implementation starts.

## 4. Priorities & Ordering

Classification: all four are the maintainer's own findings (not user-submitted), none critical or high, no `feat` in the batch. Order:
1. **#1528**: decide F5 first (as the issue says), then F5, F3, F1 (user-visible and information-exposure items first).
2. **#1479**, then **#1481** (latent hardening of the hub URL policy; no active bypass).
3. **#1526** last: largest and open-ended; it should assert the final contracts from 1 and 2.

Findings rules: a critical/high finding discovered in #1526 ships first as its own PR. **Medium/low findings from #1526 are filed as GitHub issues by default**; they are fixed in-PR only when needed to make a converted test pass or to unblock CI (CLAUDE.md DoD). Security-sensitive findings go to a private advisory per SECURITY.md.

Advisory screening: no private advisory needed for any of the four. #1479/#1481 are public, low, latent, no demonstrated bypass; F3/F5 are admin-authenticated information-level hardening with no known exploit chain; the export limitation is recorded as a known limitation, not a vulnerability (admin already has full control). If any item proves reachable by a lower-privileged actor, stop and convert to a private advisory before further public wording. All public wording (issue comments, commit subjects, PR bodies, docs) stays vague.

## 5. Branch / PR Slicing Decision

**Default: one PR with ordered commits** (PR A commits first, then PR B commits). This respects "one feature = one PR" and the user's dislike of PR slicing.

**Exception, only with the user's approval before work starts: two PRs.** PR A (`fix/crowdsec-followups-hardening`: #1528, #1479, #1481) and PR B (`test/crowdsec-deterministic-e2e`: #1526, branched from `development` after A merges, no stacking). Justification if approved: (1) B is about 2,000 lines of test rewrite with an open-ended findings tail and would hold the small hardening work hostage; (2) a secrets-visibility change cannot be meaningfully reviewed beside that churn; (3) B must assert A's final contracts. Cost of the default: larger review and A blocked on B. This is flagged as an open question; no slicing beyond commits happens unless approved.

## 6. Commit Slicing Strategy

Conventional commits; no session ID/link anywhere. All subagent dispatch prompts must state explicitly: run every test, build, E2E and lint command in the foreground and block until it finishes (no background/detached runs, per CLAUDE.md). Playwright security specs run only with `--project=security-tests` (single project, targeted files only, never the full suite). Backend lint must use the full config (`cd backend && golangci-lint run --config .golangci.yml ./...` or `make lint-backend`), not only the fast hook. GORM scan: N/A (no model/query changes; state this at the gate).

### Part A (hardening)

| # | Commit | Scope | Files | Depends on | Validation gate |
|---|---|---|---|---|---|
| A0 | `test(e2e): add fixme specs for editor secrets and response paths` | `test.fixme` specs for F5 (credentials not listed; read rejected) and F3 (`backup` and error bodies contain no absolute path); update fixture stubs to name-only values | `tests/security/crowdsec-file-editor.spec.ts` (~L89 stub + new cases), `tests/security/crowdsec-hub-preset-apply.spec.ts` (~L110), `tests/security/crowdsec-preset-apply.spec.ts` | none | `npx playwright test tests/security/crowdsec-file-editor.spec.ts tests/security/crowdsec-preset-apply.spec.ts tests/security/crowdsec-hub-preset-apply.spec.ts --project=security-tests` |
| A1 | `fix(security): harden CrowdSec editor access` | F5 per 3.1; tests assert real messages: list omits both names, read -> `invalid path`, write -> `file cannot be edited`, mixed-case and symlink-to-credentials cases (symlink caught by the resolved-path re-check ~L149) | `backend/internal/api/handlers/crowdsec_files.go`, `crowdsec_files_test.go` | A0 | `cd backend && go test ./internal/api/handlers/... ./internal/crowdsec/...` |
| A2 | `fix: stop returning server paths in CrowdSec responses` (see Q5 on whether this deserves `(security)`; if the user says yes use `fix(security): harden CrowdSec API responses`) | F3 per 3.2 across all nine items in 2.3 (`backupID` helper, `rollbackFailure`, extract/export/import error text, three UI print sites + i18n key in all locales, vitest and Playwright fixtures) | `backend/internal/crowdsec/backup.go` (+test), `hub_sync.go`, `crowdsec_files.go`, `crowdsec_handler.go`, `crowdsec_preset_handler.go`, `frontend/src/pages/CrowdSecConfig.tsx`, `frontend/src/locales/*/translation.json`, `frontend/src/pages/__tests__/CrowdSecConfig.curatedApply.test.tsx` and any other test asserting the old value, `docs/troubleshooting/crowdsec.md` | A1 | backend `go test` for touched packages; `cd frontend && npm run type-check && npx vitest run src/pages/__tests__/CrowdSecConfig*`; response-body regex test shows no absolute path |
| A3 | `fix: accept archive entries that merely start with two dots` | F1 | `hub_sync.go` (~L946), `hub_sync_test.go` | none | `go test ./internal/crowdsec/...` |
| A4 | `refactor: remove the unenforced allowed-domains client option` | #1479 incl. the three test references and option-parity table test | `backend/internal/network/safeclient.go`, `safeclient_test.go`, `hub_sync.go` | none | `go test ./internal/network/... ./internal/crowdsec/...` |
| A5 | `refactor: restrict the hub URL check to production hosts` | #1481 per 3.5 | `hub_sync.go`, `hub_sync_test.go`, `hub_pull_apply_test.go`, `hub_sync_raw_index_test.go`, `backend/internal/api/handlers/crowdsec_pull_apply_integration_test.go`, `crowdsec_preset_handler_test.go` | A4 | `go test ./internal/crowdsec/... ./internal/api/handlers/...`; grep shows no `test.hub`/`example.com`/`hubAllowLoopback`/`AllowedDomains` outside intentional places |
| A6 | `docs/test: enable e2e specs and document the changes` | un-fixme A0; docs: credentials not shown in editor, backup names, one-line production note for #1481; confirm ARCHITECTURE.md and docs/api need no change | `tests/security/*.spec.ts`, `docs/features.md`, `docs/troubleshooting/crowdsec.md` (authored under `docs/`, never `docs-site/docs/`) | A1-A5 | targeted `npx playwright test tests/security/crowdsec-file-editor.spec.ts tests/security/crowdsec-preset-apply.spec.ts tests/security/crowdsec-hub-preset-apply.spec.ts --project=security-tests` |

### Part B (#1526), after A

| # | Commit | Scope | Files | Validation gate |
|---|---|---|---|---|
| B1 | `test(e2e): shared CrowdSec stub helpers` | extract repeated route stubs and fixtures (status, decisions, enrollment, file list/content, diagnostics) | new helper under `tests/utils/` or `tests/fixtures/` | one spec run |
| B2 | `test(e2e): deterministic crowdsec-config non-preset cases` | back nav, file list/content, export, enrollment, status indicators | `crowdsec-config.spec.ts` | `npx playwright test tests/security/crowdsec-config.spec.ts --project=security-tests` |
| B3 | `test(e2e): deterministic crowdsec dashboard and decisions` | | `crowdsec-dashboard.spec.ts`, `crowdsec-decisions.spec.ts` | same per file |
| B4 | `test(e2e): deterministic crowdsec console enrollment` | stubbed states: not enrolled / pending / enrolled / failed / heartbeat | `crowdsec-console-enrollment.spec.ts` | same |
| B5 | `test(e2e): deterministic crowdsec diagnostics and import` | strict deterministic statuses or stubbed UI + Go handler tests for status mapping | `crowdsec-diagnostics.spec.ts`, `crowdsec-import.spec.ts` (+ Go tests if gaps) | same |
| B6 | `fix: ...` only when required | in-PR fix only if needed to make a converted test pass or to unblock CI, with a regression test; every other medium/low finding is filed as an issue instead (section 4); high/critical goes to its own PR | as needed | the relevant spec + unit tests |
| Bz | `test(e2e): enforce no tolerant patterns in CrowdSec specs` | gate commit: no tolerance residue in the six files | none (verification only; optionally a lint script) | the command in the code block below (must return nothing, except reviewed legitimate `test.skip` hits); also review by hand for `console.log`-only branches and annotation pushes |

Bz gate command (plain `|` alternation; ripgrep treats `\|` as a literal pipe). `test\.skip` hits need manual review because conditional skips can be legitimate:

```bash
rg -n "catch\(\(\) => false\)|catch\(\(\) => \{\}\)|may not be|not implemented|test\.skip|expect\(\[[0-9, ]+\]\)\.toContain" \
  tests/security/crowdsec-config.spec.ts tests/security/crowdsec-dashboard.spec.ts \
  tests/security/crowdsec-decisions.spec.ts tests/security/crowdsec-diagnostics.spec.ts \
  tests/security/crowdsec-console-enrollment.spec.ts tests/security/crowdsec-import.spec.ts
```

Full Definition of Done for the PR(s): targeted Playwright as above; full-config backend lint; `scripts/go-test-coverage.sh` >= 85%; `scripts/frontend-test-coverage.sh`; `bash scripts/local-patch-report.sh`; `npm run type-check` and `npm run build` in `frontend/`; `go build ./...`; `lefthook run pre-commit`. CodeQL/Trivy are CI-deferred (fix/refactor, no new surface); a local CodeQL Go run is optional for A1/A2.

## 7. Test Plan

- Backend: `isReadable`/`isWritable`/`resolveReadable`/`resolveWritable` table tests asserting real messages (`invalid path` for reads, `file cannot be edited` for writes) for both credentials names, mixed case, nested, symlink target; `backupID` table; handler tests with a response-body regex for absolute paths on success and error paths (file write, import including extraction failure, preset apply, curated apply with a forced rollback failure, acquisition update, export error); `validateHubURL` rejection/acceptance tables; safeclient dialer-vs-redirect parity table; tar entry-name table.
- Frontend: update `CrowdSecConfig.curatedApply.test.tsx` and add tests for the three print sites and the new i18n key.
- E2E: per commit tables.
- Grep gates: no `WithAllowedDomains`/`AllowedDomains`/`hubAllowLoopback` remnants; no test hosts in non-test code.

## 8. Risks and Mitigations

| Risk | Mitigation |
|---|---|
| ~100-110-hit test migration (A5) breaks many tests | Single commit, package-by-package runs, mechanical mapping with distinct hosts per role (3.5) |
| Migration weakens tests via `uniqueStrings` collapse | Distinct hosts per role; ordering assertions where relevant |
| F5 hides a file someone relies on viewing | Documented as a behavior change; disk access unchanged; masked read-only view is the fallback |
| F5 gives false assurance because export still includes secrets | Stated as known limitation; export question below |
| F3 breaks manual-recovery UX | Name plus location hint in UI, docs pointer, server log keeps full path |
| #1526 exposes real bugs | Stop rule: if more than N=6 real bugs surface, stop and re-plan with the user; medium/low are filed as issues by default (section 4) |
| Live-backend E2E flake | Prefer stubs; where live, assert only deterministic states |
| Specs silently not run without `--project=security-tests` | Every gate names the project; confirm test count in output |

## 9. Acceptance Criteria

Part A: credentials files absent from editor list and unreadable/unwritable with the correct messages; no absolute path in any CrowdSec API response, including error text, and UI still names the backup and where it is; `..`-prefixed legitimate names accepted, parent components rejected; `WithAllowedDomains` and `hubAllowLoopback` gone and all three safeclient test references handled; `validateHubURL` accepts only https allowlisted hosts; DoD passes. Part B: the six files pass the Bz grep gate; every converted test fails when the feature it names is removed (mutation spot-check per file); findings handled per section 4; targeted Playwright green.

## 10. Rollback and Contingency

Commits are independent except A2 after A1 (shared fixtures) and A5 after A4. If A5 proves too large, ship A1-A4 and keep A5 as a follow-up rather than a half-migrated seam. If B surfaces a high/critical bug, split it out per section 4.

## 11. Open Questions (need the user's decision)

1. **F5**: approve hiding the two credentials files from editor list and read (recommended), or masked values, or leave as is?
2. **Admin export**: `ExportConfig` currently includes credentials files and `bouncer_key`. Exclude secrets from the export (behavior change: an export-then-import round trip would no longer carry credentials; machine re-registers) or leave as is and document the limitation? Recommendation: leave for this batch, document it, and file a separate issue for the export decision.
3. **Slicing**: default is one PR with commit slicing; approve the two-PR exception (A then B)?
4. **F3 shape**: base name of the backup (recommended) plus UI location hint, versus a random id with a server-side map (not recommended).
5. **`(security)` scope**: A1 as `fix(security): harden CrowdSec editor access` (recommended). A2 as plain `fix:` (recommended; admin-only low information exposure, and every `(security)` subject goes to the public changelog) or `fix(security): harden CrowdSec API responses`?
6. **#1479**: remove (recommended) versus enforce in the dialer.
7. **Include the acquisition-update `backup` field and the non-listed leak sites** (rollback text, extract/export errors) in F3 (recommended; the acceptance criterion requires it).
8. **PR B stop rule**: N=6 real bugs then re-plan with you (recommended)?
