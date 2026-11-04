# CrowdSec Leftovers (PR 17): one engine address, busy answers, hub fallback, test and doc cleanup

Status: plan for Supervisor review. Branch `fix/crowdsec-leftovers`, one PR into `development` (merge commit), cut from `development` at `d7982771b` (after PR 16, #1832). All anchors below were re-read at that commit on 2026-11-04. Backend paths are relative to `backend/internal/` unless they start with another top-level folder. Nothing here is implemented yet.

## 1. Introduction

PR 15 (#1824) and PR 16 (#1832) left a queue of small CrowdSec items. This PR clears them in ordered, independently revertable commits. There is no `feat` here and no schema change. Charon is v0, so no commit carries a `!` or `BREAKING CHANGE:` marker.

Left out on purpose: #1716/#1820 (lint sweep of `tests/`), the cscli fixture re-record, #1725 and the integration-test hub-section skip (not CrowdSec code, `devops` any time), boot-time registration outside the shutdown drain (needs a design), #1701/#1825 timing thresholds. PR 18 (WAF rule sets need rules) is a separate PR.

### Text rules for everything this PR ships

Commit subjects, code comments, test names, error strings, docs and the PR description describe behavior only ("CrowdSec readers use the local engine address", "an apply is refused while a restore runs"). Subjects are plain `fix:`/`test:`/`docs:`; none uses the `(security)` scope. No session IDs or session links anywhere.

## 2. Priorities & Ordering

No finding is critical or high, and none is user-submitted (all authors are the maintainer), so nothing ships ahead of or apart from this PR. All items are our own findings and there is no `feat`, so bug fixes come first. Order is by value, then by quantity:

| Rank | Item | Class | Severity | Commit |
| --- | --- | --- | --- | --- |
| 1 | #1826: five readers of the engine address now share one constant | consistency | low | 1 |
| 2 | #1796: a preset install during a hub restore waits (UI looks stuck); restore-vs-restore 409 lacks `error_code` | user-visible bug | low | 2 |
| 3 | #1806: no per-item mirror fallback in `Pull` | outage robustness | low | 3 |
| 4 | #1807: hub upgrade time share differs between shell and Go (premise mostly resolved, see 4.4) | consistency | low | 4 |
| 5 | #1797: one warning per skipped symlink on export | log noise | low | 5 |
| 6 | #1827/#1794: registration log arms and occupied-port revert not pinned | test gap | low | 6 |
| 7 | #1742 (+ optional #1765): tests leak state or flake | test hygiene | low | 7 |
| 8 | Registered badge has no Playwright assertion | test gap | low | 8 |
| 9 | #1830, #1793 and engine-address docs | stale docs | low | 9 |

Out-of-scope findings seen while re-reading: none new beyond the queue. #1765 (flaky import test) is an existing CrowdSec issue not previously scheduled; see Q3.

## 3. Research findings (drift from the earlier spec, all corrected below)

- Line numbers moved after PR 16: `GetLAPIDecisions` reader now `api/handlers/crowdsec_handler.go:1572-1577`, `CheckLAPIHealth` `:2210-2215`, `handleBouncerRegistration` `:1973`.
- The dashboard reader is `fetchActiveDecisionCount` (`api/handlers/crowdsec_dashboard.go:203-208`) and still holds a hand-copied literal.
- `ApplyJobManager` lives in `crowdsec/apply_jobs.go` (`Busy` `:207`, `CanStart` `:216`), not in `api/handlers`.
- #1807: the shell helper already retries on a bare timeout only for `hub update`, same as Go, and the shell suite already pins it. Only the first-attempt share for `hub upgrade` differs.
- `handleBouncerRegistration` is already 94.4% covered (filtered run); one arm is unpinned. The "stale `TestRegisterBouncer*` comments" no longer exist.
- The Vitest "registered" badge test already exists; only the Playwright assertion is missing.
- `config.Config.CrowdSecAPIURL` must stay: `cerberus/cerberus.go:152` uses it in the all-zero "enabled by default" check, so removing it would change behavior for an install whose only setting is the stale variable.
- #1742 still reproduces under `-count=3`.
- `zensical` is not installed on this host, so the docs build can only be a CI check.

## 4. Technical specification per commit

### 4.1 Commit 1: read the CrowdSec engine address from one place (#1826)

Entry points (all read the stored `SecurityConfig.CrowdSecAPIURL` today):

| Reader | Anchor | After |
| --- | --- | --- |
| Proxy CrowdSec app | `caddy/config.go:99-102` (literal plus stored override) | `crowdsec.LocalLAPIURL` only |
| Decisions | `api/handlers/crowdsec_handler.go:1572-1577` | constant; `h.Security.Get()` call removed |
| Health | `api/handlers/crowdsec_handler.go:2210-2215` | constant |
| Active decision count | `api/handlers/crowdsec_dashboard.go:203-208` | constant (literal removed) |
| Status | `api/handlers/security_handler.go:101,124-125,217` | `crowdsec.api_url` reports the constant |

- `caddy` does not import `crowdsec` today; `crowdsec` depends only on `models`, so there is no cycle.
- `ValidateInternalServiceBaseURL` stays on the handler readers (still correct for the loopback constant). Responses keep `lapi_url`/`api_url` (now the constant); no API shape change.
- The column, seed (`models/seed.go:17`), the settings API (`services/security_config_update.go:175`) and `config.Config.CrowdSecAPIURL` stay for compatibility; mark the config field "ignored" in a comment. The existing one-line boot Warn for the environment variable stays.
- `GetStatus` replaces the whole chain for the address: the static seed from `h.cfg.CrowdSecAPIURL` (`:101`), the row override (`:124-125`) and the response field (`:217`) all become the constant.
- Comment-only update on `frontend/src/api/security.ts:112` ("ignored").

Tests: 49 lines in 18 test files mention the field. Update the tests that exercise the override; never delete them. They are `caddy/config_crowdsec_test.go` (and the other two caddy files: `config_test.go`, `config_generate_additional_test.go`), `crowdsec_dashboard_test.go` (`TestDashboardSummary_ActiveDecisions_LAPIReachable`, `_LAPIBadStatus`, `_LAPIBadJSON`), `crowdsec_handler_test.go` (9 lines), `crowdsec_state_sync_test.go` (`TestStartSyncsSettingsTable`, `TestStopSyncsSettingsTable`, `TestStartAndStopStateConsistency`, `TestExistingSettingIsUpdated`), `crowdsec_stop_lapi_test.go` (`TestGetLAPIDecisions_WithMockServer`, `_Unauthorized`, `_NullResponse`, `_NonJSONContentType`, `TestCheckLAPIHealth_WithMockServer`, `_FallbackToDecisions`), `crowdsec_wave5_test.go` (`TestCrowdsecWave5_GetLAPIDecisions_Unauthorized`, `_NonJSONContentTypeFallsBack`), `crowdsec_wave7_test.go` (`TestCrowdsecWave7_EnsureBouncerRegistration_InvalidFileKeyReRegisters`) and the status tests. The one-line uses in `crowdsec_key_*_test.go`, `crowdsec_cscli_contract_test.go`, `crowdsec_ban_alerts_contract_test.go`, `models/seed_test.go` and the services tests only seed the column and need no change. New cases: with an address that differs from the local engine stored, the proxy config, decisions, health, dashboard count and status all use the local address (the request reaches the local address through the validator seam); `GetStatus` with the environment value set returns the local address; `security_config_save_pins_test.go` keeps asserting the stored column round-trips.

Agent: backend-dev. Gate: `cd backend && go test ./internal/caddy/` (whole package) and `go test ./internal/api/handlers/ -run '<regex>'`, where the regex is generated from the test names in the files above (for example `grep -h '^func Test' <files> | sed 's/func \(Test[A-Za-z0-9_]*\).*/\1/' | paste -sd'|'`) plus `GetStatus|SecurityStatus`; then `go build ./...`.

### 4.2 Commit 2: refuse a preset install at once while a hub restore is running (#1796)

Verified facts: `RestoreHubItems` takes a hold first (`api/handlers/crowdsec_hub_items.go:129`); `hubUpdate` holds `dataMu` for the whole `hub update` (`:230-239`), which is how an apply waited 77 s with the hub down; an async non-curated apply is already refused lock-free by `CanStart` (`api/handlers/crowdsec_preset_handler.go:336`); curated and synchronous applies skip it and queue on `lockData` (`:344`); `Busy()` ignores holds.

Step 0 (before any code, run by qa-security or the main session, not the dev agent, because it needs a container): re-run the QA recipe (hub unreachable, restore in flight, then curated sync, non-curated sync and async non-curated apply); record which waited and for how long in the PR notes. The dev agent consumes that result. If none waits, only part (a) below ships and the commit subject changes to "add an error code to the restore-already-running answer".

Changes:

- (a) `crowdsec_hub_items.go:141-143`: the restore-vs-restore 409 uses `respondCoded(c, http.StatusConflict, codeCrowdsecBusy, msgRestoreRunning, nil)` (same text, adds `error_code: crowdsec_busy`).
- (b) `crowdsec_preset_handler.go:330`: the lock-free pre-check also answers `respondBusy(c, "")` while `h.restore.running.Load()` is true, for every apply flavour. Not extended to the brief holds of Start/Stop/whitelist (waiting a few ms there is correct today).
- Check that the restore UI (`frontend/src/hooks/useCrowdsecHubItems.ts`, `components/crowdsec/HubItemsRestorePanel.tsx`) still shows a sensible text now that the 409 carries a code (`api/errorBody.ts:33`); `crowdsec_busy` is already mapped there and covered by `frontend/src/api/__tests__/errorBody.test.ts:81` (`crowdsecBusyMessage`), so no mapping change is expected, and the restore panel's Vitest gets one case for a coded restore 409; if the text changes, fix it and update the Vitest in the same commit (frontend-dev).

Tests: restore blocked in a fake executor plus each apply flavour answers 409 `crowdsec_busy` within 100 ms and `dataMu` is free; restore-vs-restore body carries the code; the `routeHeld` classification tests (`crowdsec_busy_test.go`, `crowdsec_held_routes_test.go`) keep passing.

Agent: backend-dev (frontend-dev only if the UI check finds a change). Gate: `go test ./internal/api/handlers/ -run 'Busy|Hold|Restore|Route|Preset'`.

### 4.3 Commit 3: try the hub mirror when a single hub item download fails (#1806)

`crowdsec/hub_sync.go` `Pull` (`:698`) downloads the item at `:751` through `fetchWithLimitFromURL` (`:902`). The index walk already falls back per address (`fetchIndexHTTP`, `hubBaseCandidates()`, the `CanFallback()` classification on `hubHTTPError`/`hubTransportError`, `walkContext`/`attemptContext`); the item download does not.

Design: on a failure the walk classes as fall-back-able (refused, DNS, reset, timeout while the caller's context is alive, or the HTTP statuses `hubHTTPError.fallback` marks), retry once against the mirror URL derived from the item's path under the mirror base, inside the existing `PullTimeout` context (each attempt gets its share through `attemptContext`). The digest is verified as usual. An index that has no download location (`idx.offline`, `:753`) keeps today's error. No new setting.

Tests: CDN serves the index and resets the item, mirror serves it; mirror digest mismatch is refused; both fail gives `ErrHubUnreachable`; caller cancel is not retried.

Agent: backend-dev. Gate: `go test ./internal/crowdsec/ -run 'Pull|Mirror'`.

### 4.4 Commit 4: align the hub upgrade time share (#1807)

The retry rule is already the same in both places: `.docker/cscli-hub-mirror.sh:267-275` (`_hub_retry_wanted`) retries on a bare timeout only for `hub update`; `hub upgrade` and installs retry on network-failure text only, as Go does (`crowdsec/cscli_run.go:121-135`), and `scripts/cscli_hub_mirror_test.sh:453-486` pins it. The remaining difference: for `hub upgrade` the script halves the first attempt (`:337-343`, `_first=$((_budget / 2))`) and then does not retry on a timeout, so a slow healthy upgrade can be cut at half its time; Go gives every non-update call the whole share (`runShare`, `cscli_run.go:108-119`).

Change: `hub upgrade` takes the full first share in the script (a blackholed CDN is already caught earlier by `hub update`, which sets mirror-first for later calls). Update the shell cases that assert the share, and the sentence in `docs/troubleshooting/crowdsec.md:266` that describes the halved attempt. After merge, close #1807 with this finding.

Agent: devops (shell script and shell suite), backend-dev only if a Go table case is added; the docs sentence goes in commit 9. Gate: `bash scripts/cscli_hub_mirror_test.sh` (foreground), `go test ./internal/crowdsec/ -run 'Cscli|Mirror'`.

### 4.5 Commit 5: log skipped export symlinks once (#1797) and add a signalable check in signalOSProcess

`crowdsec/export.go:163-165` warns once per non-regular entry. Replace with one Info line carrying the count at the end of the export and one Debug line per entry. The other "skipping" lines (`:160`, `:207`, `:288`, `:302`) are different cases and stay. Test: an export with N symlinks logs one Info with the count and N Debug lines, and the archive is unchanged.

Second part: add a signalable check in `signalOSProcess` (`api/handlers/crowdsec_reload.go:263`): it returns an error when `!signalable(pid)` (`api/handlers/crowdsec_exec.go:173`) instead of signalling. Callers: `api/handlers/crowdsec_handler.go:656` (the `signalProcess` field) and `crowdsec_reload.go:231`; both already pass tracked pids, so this is defence in depth. Tests: pid 0, 1 and the own pid return an error and no signal is sent.

Agent: backend-dev. Gate: `go test ./internal/crowdsec/ -run Export` and `go test ./internal/api/handlers/ -run 'Reload|Signal'`.

### 4.6 Commit 6: cover the registration log arms and the occupied-port revert (#1827, #1794)

- `handleBouncerRegistration` (`api/handlers/crowdsec_handler.go:1973`): one table over `EnsureResult` outcome by `regErr` asserting the log line per outcome with a hook logger, and that no key text appears. The unpinned arm is `OutcomeSavedKeyValid` with `EnvKeyIgnored` (`:2002`). Target 100% for the function.
- #1794: a Start whose engine exits early (`failStartEngineExited`, `api/handlers/crowdsec_lifecycle.go:553`) while the module is already enabled reverts to the exact enabled snapshot. `newResetLifecycleHandler` (`crowdsec_lifecycle_reset_test.go:110`) seeds the module disabled; add an enabled-seed variant next to `TestStartCrowdSec_EngineExitsDuringLAPIWait` in `crowdsec_lifecycle_exit_test.go`. The container run of the occupied-port scenario stays a QA gate.

Agent: backend-dev. Gate: `go test ./internal/api/handlers/ -run 'Register|Bouncer|Registration|EngineExit|StartCrowdSec' -cover` with `handleBouncerRegistration` at 100% (the 100% target applies to that function only, not the package).

### 4.7 Commit 7: make the backup service tests independent of each other (#1742, optional #1765)

Reproduce first with `go test ./internal/services/ -count=3 -shuffle=on -run 'Backup|Restore|RunScheduledBackup'` and record the seed. Failing today: `TestRunScheduledBackup_EncryptionDisabled_ProducesUnencryptedBackup` (`services/backup_service_test.go:576`), its Enabled twin near `:538`, and `TestStartRestoreJob_CompletesSuccessfully_DoesNotSelfDeadlock` (`services/backup_restore_safe_async_test.go:80`). Shared state: the helpers open the database as `file:<t.Name()>?mode=memory&cache=shared` (`backup_service_options_test.go:43`, `backup_restore_safe_async_test.go:138`, `backup_service_async_create_test.go:23`), a name that is identical on every `-count` iteration and stays alive inside the process, so rows from the previous iteration are still there. Fix: add a per-call unique suffix (or a temp-file database) in those helpers, and a temp dir per test. If Q3 is yes, also make the `extraction_failure` subtest (`api/handlers/crowdsec_export_import_test.go:285`) deterministic by building a fixture whose failure point is fixed. #1701 and the perf thresholds stay out.

Agent: backend-dev. Gate: `go test ./internal/services/ -count=3 -run 'Backup|Restore|RunScheduledBackup'` (and `-count=3 -run 'ImportConfigFailure'` in handlers if Q3 is yes).

### 4.8 Commit 8: assert the registered badge in the security UI spec

The Vitest case exists (`frontend/src/components/__tests__/CrowdSecBouncerKeyDisplay.test.tsx:151` and the legacy-field fallback cases). Add the missing Playwright case beside `tests/security/crowdsec-ui-states.spec.ts:323-348`: route-mocked `registered: true, registration: 'registered'`, expects the "Registered" badge and no "Not Registered". PR 17 adds no frontend source branches, so the 0.1 point branch-coverage margin from PR 16 QA is unaffected.

Agent: playwright-dev (frontend-dev is fine if preferred). Gate: `npx playwright test tests/security/crowdsec-ui-states.spec.ts --project=firefox -g "registration"`. See risk R3 about the test container.

### 4.9 Commit 9: docs

Agent: docs-writer. Edit only files under `docs/` and `.github/skills/` (never `docs-site/docs/`; the sync copies from `docs/`).

- Engine address: `docs/cerberus.md:121` (struct listing) and `:139`; `docs/guides/crowdsec-setup.md:463` ("the stored address is ignored too"); `docs/migration-guide.md:70-78` (add the stored field to the unused list).
- #1830: `.github/skills/integration-test-security-headers.SKILL.md:108` delete the `SH_ADMIN_PORT` row (referenced nowhere else); `docs/reports/caddy-security-posture.md:24,35-37,61` say the admin listener is on the loopback address and the address variable is gone. `docs/testing/FAILURE_DIAGNOSIS_GUIDE.md:314` uses `docker exec` and is still valid: leave.
- #1793: `docs/features/crowdsec.md` (near `:436`) and a third bullet in `docs/troubleshooting/crowdsec.md:295-296`: a Start that overlaps another Start answers 409 `crowdsec_busy`. No `docs/api.md` section documents Start.
- The hub upgrade sentence at `docs/troubleshooting/crowdsec.md:266` (commit 4).

Gate: markdownlint on the touched files. The docs-site build (`bash docs-site/scripts/sync-docs.sh && cd docs-site && zensical build --strict`) cannot run locally because `zensical` is not installed; it runs in CI and QA states that explicitly.

### 4.10 Commit 10: QA report

`docs: add the QA report`, written by qa-security to `docs/reports/qa_report.md`.

## 5. Commit Slicing Strategy

One PR, merge commit, branch `fix/crowdsec-leftovers` off `development` at its tip. The main session commits; dev agents leave changes in the working tree and run scoped tests only, in the foreground.

| # | Subject | Depends on | Agent | Scoped gate |
| --- | --- | --- | --- | --- |
| 1 | `fix: read the CrowdSec engine address from one place` | none | backend-dev | 4.1 |
| 2 | `fix: refuse a preset install at once while a hub restore is running` | none | backend-dev (+ frontend-dev if needed) | 4.2 |
| 3 | `fix: try the hub mirror when a single hub item download fails` | none | backend-dev | 4.3 |
| 4 | `fix: align the hub upgrade time share between the startup script and the API` | none | devops | 4.4 |
| 5 | `fix: log skipped export symlinks once and check the pid before a reload signal` | none | backend-dev | 4.5 |
| 6 | `test: cover the bouncer registration log arms and the occupied-port start revert` | 1, 2 (same handler files) | backend-dev | 4.6 |
| 7 | `test: make the backup service and import tests independent of each other` | none | backend-dev | 4.7 |
| 8 | `test: assert the registered badge in the security UI spec` | none | playwright-dev | 4.8 |
| 9 | `docs: remove stale CrowdSec engine address and admin address statements` | 1, 4 | docs-writer | 4.9 |
| 10 | `docs: add the QA report` | all | qa-security | n/a |

Commits 1-5 touch disjoint files except `api/handlers` (1, 2), so run them sequentially in table order, not in parallel. Commits 7 and 8 are independent and may run in parallel with each other after 6.

Whole-PR Definition of Done (run once by qa-security, foreground, in CLAUDE.md order): targeted Playwright (`crowdsec-ui-states.spec.ts`, firefox only) then GORM scan (models untouched, skip) then `bash scripts/local-patch-report.sh` then `lefthook run pre-commit` then `scripts/go-test-coverage.sh` (85%) then `scripts/frontend-test-coverage.sh` (85%) then `npm run type-check`, `npm run build`, `go build ./...` then the pinned linter `docker run --rm -v $PWD:/app -w /app/backend golangci/golangci-lint:v2.15.0 golangci-lint run --timeout=25m ./...` to 0 issues. Security scans (CodeQL, Trivy) are deferred to CI: no commit is a `feat` and no commit adds a new path that accepts input.

Container gates (QA, throwaway containers and volumes with unique names, never the live `charon`):

- G1 (#1826): an address that differs from the local engine is stored through `POST /security/config`; readers still use the local address; the generated proxy config (`docker exec ... curl` against the admin address inside the container) still has `http://127.0.0.1:8085` and the decisions and health endpoints answer from the local engine.
- G2 (#1796): hub unreachable, start a restore, then each apply flavour: 409 `crowdsec_busy` within a second; second restore 409 with `error_code`.
- G3 (#1806): block only the item path on the CDN with a throwaway proxy; install succeeds through the mirror. If the throwaway network cannot do it, unit level is authoritative and QA records why.
- G4 (#1794): occupied-port Start with the module already enabled returns the coded 500 and the module state equals its pre-Start state.
- G5: healthy path re-check, badge shows "registered".

Rollback: any commit reverts alone. Commits 1 and 2 change what the proxy and the UI get and are the only ones with user-visible behavior; 3 and 4 change outage handling; 5-10 are logs, tests and docs. If the PR grows past review comfort, split by concern only for 1 and 2 (still never by backend/frontend).

## 6. Risks and mitigations

| ID | Risk | Mitigation |
| --- | --- | --- |
| R1 | Ignoring the stored address breaks an install whose engine is not on 8085 | External mode was removed in PR 15; the constant is the single place to change; docs say so (commit 9) |
| R2 | Removing `config.Config.CrowdSecAPIURL` changes the Cerberus default | The field and the all-zero check stay; comment marks it ignored |
| R3 | The E2E image build reuses the `charon:local` tag of the live container (#1828) | QA builds a throwaway image with a unique tag and runs commit 8's spec against it; never recreate the live container |
| R4 | Refusing every apply while a restore runs surprises a user who waited successfully before | Only while `restore.running` is set; the answer is the existing busy text; documented in commit 9 |
| R5 | Per-item mirror retry doubles load or masks a bad item | One retry, digest verified, caller cancel and non-fallback statuses are not retried |
| R6 | Dropping the half-share of `hub upgrade` lengthens boot against a blackholed CDN | `hub update` runs first and switches later calls to mirror-first; shell suite pins the new share |
| R7 | The commit 2 recipe shows no wait at all | Commit shrinks to the error code only (4.2 step 0) |

## 7. Open questions (all answers below are recommended, awaiting user approval)

1. **Q1. Keep PR 17 separate from PR 16/18 and plain-subject?** Recommend yes: all subjects plain, no `(security)` scope, because every change is a consistency or test fix with neutral text.
2. **Q2. #1807: narrow change (hub upgrade takes the full first share, close the issue) or add a timeout retry to installs?** Recommend the narrow change; the existing rule "never retry a bare deadline on an install" protects slow healthy installs and is already pinned in both suites.
3. **Q3. Fold #1765 (flaky `extraction_failure` import test) into commit 7?** Recommend yes: tests only, same theme, and a flake blocks CI.
4. **Q4. #1796 refusal scope: every apply flavour only while a hub restore runs, or also during brief holds?** Recommend the narrow version (restore only).
5. **Q5. #1793: keep 409 for an overlapping Start and document it, or return 200 with the pid?** Recommend keep 409 and document.
6. **Q6. Boot-time registration outside the shutdown drain:** recommend leaving it as documented and in the backlog.
7. **Q7. Stored `crowdsec_api_url` setting: ignore now, reject out-of-range values in the #1761 PR?** Recommend yes. Follow-up: file a neutral issue to track the range check in the #1761 PR (stored address), so it is not lost.

## 8. Acceptance criteria

- Every CrowdSec reader of the engine address uses `crowdsec.LocalLAPIURL`; a stored address that differs from the local engine changes nothing.
- An apply of any flavour during a hub restore answers 409 `crowdsec_busy` at once; the restore-already-running answer carries `error_code`.
- A hub item download that fails on the CDN succeeds through the mirror with a verified digest.
- `hub upgrade` timing matches the Go rule in the shell suite; the troubleshooting sentence is corrected.
- One export logs one symlink summary line.
- `handleBouncerRegistration` is at 100%; the enabled-module occupied-port revert is pinned; backup tests pass under `-count=3`.
- The Playwright spec asserts the registered badge.
- Docs no longer describe the stored address, the admin host port or the register script as live.
- The Definition of Done above passes with zero failures.
