# QA and Security Report: feat/crowdsec-hub-items-restore

Date: 2026-10-11
Scope: full diff `origin/development...HEAD` (HEAD 96260b17, 7 commits, 47 files) against `SECURITY.md`, `CLAUDE.md` (Definition of Done) and the approved batch-3 plan (with the Supervisor amendments). The branch adds a hub item list to the CrowdSec export, an import-time check of notification references, a new admin endpoint `POST /api/v1/admin/crowdsec/hub-items/restore` that installs missing hub items, the matching frontend panel, translated export/import messages, E2E specs and docs.

## Verdict: PASS (no blocking findings)

No critical, high or medium findings. Four low or informational observations (F1-F4) are listed; none blocks merge. No model, migration or GORM query file is in the diff, and no security-sensitive finding was found. One E2E test failed once on the first gate run because the local E2E container's 256 MB data tmpfs filled up (environment, see F1); the same test passes on a clean tmpfs.

## Definition of Done evidence

| Check | Command | Result |
|---|---|---|
| Build and vet | `cd backend && go build ./... && go vet ./...` | pass, no output |
| golangci-lint, backend | `golangci-lint run --config .golangci.yml ./...` (v2.14.0) | 0 issues |
| golangci-lint, agent | `cd agent && golangci-lint run --config ../backend/.golangci.yml ./...` | 0 issues |
| Frontend | `cd frontend && npm run type-check && npm run build` | pass |
| i18n keys | `vitest run src/pages/__tests__/CrowdSecConfig.i18nKeys.test.ts` | 6 passed; the `crowdsecConfig` key set (185 keys, including the new `hubItems` group) is identical in en, de, es, fr and zh |
| Lefthook | `lefthook run pre-commit --all-files` | all 17 hooks pass (795 s), including actionlint, shellcheck, semgrep (367 rules, 1946 files, 0 findings), frontend type-check and lint; the working tree stayed clean (only the three untracked plan files) |
| govulncheck | skill `security-scan-go-vuln` | 0 affected; 1 module-level advisory in a required module that Charon code does not call |
| Trivy | `trivy fs --scanners vuln,misconfig --severity HIGH,CRITICAL` on go.mod/go.sum (backend, agent) and the frontend package files, copied under `$HOME` (the snap trivy cannot read `/projects`) and removed afterwards | no HIGH/CRITICAL |
| CodeQL | `lefthook run codeql` (CLI 2.27.2, current) | Go: 4 results, all 4 suppressed pre-existing entries in `auth_handler.go`, `remote_server_handler.go`, `uptime_service.go` (none in a file of this diff); JS: 0 results; "0 blocking"; "All CodeQL checks passed"; parity check passed. None of the earlier log-injection findings in the restore code remain. |
| GORM scan | not run | no `backend/internal/models/**`, migration or GORM query file in the diff (0 matches) |
| Coverage (given, not rerun) | prior runs | backend 91.4% stmt / 90.4% line, frontend 91.52% lines, patch coverage 94.8% |
| Patch report artifacts | `test-results/local-patch-report.{md,json}` | produced by the earlier run; the directory is cleared by Playwright's output folder, so the files were gone after the E2E gate and were not regenerated (instruction: do not rerun coverage scripts) |

## Race and repeat runs

| Run | Result |
|---|---|
| `go test -race -count=2 -run 'HubItems\|Restore\|Manifest\|Inspect\|Notification\|Export\|Import\|Keep\|Rollback' ./internal/crowdsec/... ./internal/api/handlers/...` | ok (crowdsec 5.7 s, handlers 64.6 s) |
| Full package, `go test -race -count=1 -timeout=40m ./internal/api/handlers/` | ok, 386.6 s, no failure. The earlier unrecorded one-off failure in this package was not reproduced; no test name is available to chase. |

## E2E gate (single project, once, on a freshly rebuilt `charon-e2e`)

`npx playwright test tests/security/crowdsec-hub-items.spec.ts tests/security/crowdsec-import.spec.ts tests/security/crowdsec-config.spec.ts tests/security/crowdsec-diagnostics.spec.ts tests/security/crowdsec-file-editor.spec.ts tests/security-enforcement/zzz-security-ui/crowdsec-import.spec.ts --project=security-tests`

Result: 149 passed, 1 failed, no fixme remaining. The failure was `crowdsec-import.spec.ts:132 Rejected Archives > should reject an archive larger than 50 MB`, failing in its `beforeEach` baseline import with 500. Container log: `no space left on device` on the backup write; `/app/data` (256 MB tmpfs) was at 91%, holding about 155 MB of retained backups (each about 39 MB, because the data dir holds about 73 MB of CrowdSec data) and the data dir itself. After removing the backups and `crowdsec.backup.*` leftovers inside the throwaway container, the whole "Rejected Archives" group (5 tests, including that one) passed. `changelog.json` was restored with `git checkout`. See F1.

## Security review

### 1. Restore endpoint

Verified live against the rebuilt `charon-e2e` container (validation paths only; no install was run, the hub was unreachable there):

| Probe | Result |
|---|---|
| No credentials | 401 |
| Authenticated user with role `user` (created for the probe) | 403 `Forbidden` |
| Admin, each invalid item: unknown type, `a/b..c`, leading `-` on owner and on name, NUL, trailing newline, embedded newline, Cyrillic look-alike character, `=`, space, two slashes, 129-character name, empty list, malformed JSON | 400 with the fixed messages `invalid hub item`, `no hub items given`, `invalid request` |
| 1,001 items | 400 `invalid hub item` |
| Body over 1 MiB | 413 `request too large` |
| Unknown JSON field alongside a valid item | ignored (valid item proceeds) |

- Route sits in `managementAdmin` (`RequireRole(admin)`), the same group as the sibling CrowdSec routes; a route test pins the 401.
- Argv is built as the literal `cscli`, then `<type>`, `install`, `<name>`, through the command executor (no shell). Name pattern `^[A-Za-z0-9][A-Za-z0-9._-]{0,63}/[A-Za-z0-9][A-Za-z0-9._-]{0,127}$` plus a `..` ban and a five-value type allowlist; Go RE2 `$` matches only at the end of text, so a trailing newline cannot slip through (probe above confirms). No validated string starts with `-` or contains `=`, whitespace or a second slash, so cobra cannot read it as a flag or extra argument; names beginning with a digit or containing dots are plain positional arguments. cscli v1.8.1 in the image was confirmed to be the version in use.
- Response bodies carry only fixed reason codes (`not_found`, `install_failed`, `verify_failed`, `not_attempted`) and fixed messages; cscli output and paths go to the log only. All log fields in the new code (`type`, `name`, `error`, `output`) pass through `hubLogDetail` (`sanitizeForLog` plus a 300-byte cap); the remaining log lines use fixed strings or filesystem errors that contain no request data. CodeQL confirms no log-injection result.
- Concurrency: a compare-and-swap single-flight flag (never waited on) returns 409 to a second call; each item runs under `dataMu` for one cscli section with a context of min(remaining overall time, 120 s); the overall budget is 5 minutes; the hub update has 60 s; the reload uses a detached 10 s context so an install that succeeded just before a disconnect is still reloaded.
- Lock order `restore.running -> dataMu -> HubService.mu`: all seven `dataMu` users (import, export, file write, preset apply, and the three restore sections) take it non-nested and never call another `dataMu` taker while holding it; `installedHubSet` is the only restore helper that locks and is called without the lock held; the reload runs after every lock is released. No re-entrancy or inversion was found, and the full race run passed.
- Resource use: an admin can start at most one restore at a time (409 otherwise), at most 1,000 items per call, at most 5 minutes per call, and other `dataMu` users wait for at most one 120 s section. The 1 MiB body cap is applied before the flag is taken; slow body reads are a general server property (see F4).

### 2. Manifest path

- `ArchiveEntryRel` is now the single normalization used by both the extractor and `InspectArchive`, so `./charon-hub-items.json`, `a/../charon-hub-items.json` and `charon-hub-items.json` are the same entry for both; a nested `x/charon-hub-items.json` is an ordinary file. The extractor skips the manifest path regardless of the entry type, so it is never written to disk (E2E "never writes the manifest to disk" and the `./` spelling spec pass).
- Bounds: manifest at most 1 MiB, a duplicate entry ignores the whole manifest, malformed JSON, unknown version and more than 1,000 items ignore it, an individually invalid item is dropped and counted (`skipped_invalid`, names not echoed). `InspectArchive` keeps at most 1 MiB per entry, 8 MiB in total and 256 files, and sets `Incomplete` instead of failing.
- Export side: the manifest is derived from link targets only; the target is cleaned and parsed as `hub/<type>/[stage/]<owner>/<name>.yaml`, and the name is re-validated with the same function the restore uses. The test table covers relative links, links elsewhere, wrong type, missing stage, too deep, odd extension, invalid name, overlong owner and a dangling target. A file with the reserved name inside the data dir is skipped on export.

### 3. Notification check

- The check runs after the archive validator and before the lock, so a refusal changes nothing (E2E and handler tests assert the live files are untouched). It skips instead of failing when the inspection is incomplete, there is no profiles file, the config redirects the paths, a profile or notification file does not parse, or a notifications value is not a list of names.
- The 422 message lists at most 10 names, each sanitized and cut at 64 characters, plus "and N more".
- YAML alias abuse: a temporary probe fed alias-expansion documents of 5 to 40 nested levels (up to 2.1 KB) through `CheckProfileNotifications` in both profiles.yaml and a notification file. Each finished in 1-2 ms with under 400 KB allocated in total (the parser rejects the excessive aliasing before expansion, and the check then skips). The probe file was removed; the working tree is clean.

### 4. Frontend panel

- No `dangerouslySetInnerHTML` in the panel or the page; item type and name are rendered as React text; error messages are fixed i18n strings chosen from the HTTP status (502, 409, other); the result area is a `role="status"` `aria-live="polite"` region and the section carries `aria-busy`; the 330 s timeout applies only to the restore call; the panel is passive (no banner, no toast; only the existing import success toast). The import failure toast shows the server message (including the 422 notification text) through the existing response interceptor.

### 5. Commits and docs

- Subjects: `docs: describe the CrowdSec hub item list and the import check`, `feat: offer to install missing hub items after a CrowdSec import`, `feat: add an action to install missing CrowdSec hub items`, `feat: record the installed hub items in a CrowdSec export`, `fix: validate notification references in a CrowdSec import`, `fix: translate the CrowdSec export and import messages`, `test(e2e): add fixme specs for the CrowdSec import checks and hub item restore`. All generic; none uses the `(security)` scope; no session ID or link in any message. The docs and `ARCHITECTURE.md` additions describe behavior only, with no file names, process IDs or secret details.

## Findings

No critical, high or medium findings.

### Low

- **F1: low (test environment, not product).** `tests/security/crowdsec-import.spec.ts:132` (and any import spec late in a full gate run): the local E2E container's `/app/data` is a 256 MB tmpfs, and each import and manual backup writes a roughly 39 MB zip (the data dir holds about 73 MB of CrowdSec data). The gate filled the tmpfs once during the CrowdSec config/import specs and one `beforeEach` import returned 500 (`no space left on device`). Evidence: container log lines `checksum writer: write /app/data/backups/...: no space left on device` and `df` showing 91% used with 4 retained backups. Suggested approach: raise the tmpfs size in the E2E compose file, or have the import/config specs delete the backups they create. Already known behavior ("fills up if the specs run twice"), but it now happens on a single run, so CI on a fresh container should be watched (CI uses its own container; a failure there would look the same).

### Informational

- **F2: info.** `backend/internal/api/handlers/crowdsec_hub_items.go` `hubItemNotFound`: classifies `not_found` by matching "can't find" or "not found" (case-insensitive) in cscli output. It is best effort by design; a different cscli wording only turns `not_found` into `install_failed`, with the same safe response. Nothing to change unless cscli is upgraded; the wording was verified against the real hub in the manual run.
- **F3: info.** `backend/internal/api/handlers/crowdsec_hub_items.go` `installHubItem` / `hubUpdate`: `dataMu.Lock()` is not context-aware, so a restore whose client has left still holds the single-flight flag until it gets the lock and notices the cancelled context (at most one other `dataMu` section, itself bounded). No action needed; mention only for operators who see a 409 right after a cancelled restore.
- **F4: info.** The restore body read (1 MiB cap) has no per-request read deadline of its own; slow-client behavior is whatever the HTTP server configuration provides, as for every other admin endpoint. Admin-only, so not a new exposure.

## Known limitations (not re-opened)

Flat archives (#1559), the image ships no notification plugins (#1551 comment), profiles file hazard (#1554), restore edge cases (#1556).

## Manual evidence (given, on throwaway containers at HEAD bcda295d, not repeated here)

Real-hub restore of 2 real items and 1 bogus (200, bogus reported as `not_found`); export manifest with 61 items; cross-host import reporting the missing items; restore on the second host, then a second restore reporting everything as already installed; a parallel restore refused with 409; import and export waiting during a 34 s ten-collection restore (export about 10.9 s, import about 6.5 s); a real 422 on a missing notification and 200 once defined; an offline restart that is non-fatal.
