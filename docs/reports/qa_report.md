# QA and Security Report: fix/crowdsec-followups-batch4

Date: 2026-10-11
Scope: full diff `origin/development...HEAD` (11 branch commits plus the 1 QA commit listed below, 44 files) against `SECURITY.md`, `CLAUDE.md` (Definition of Done) and the approved batch-4 spec. The branch refuses a CrowdSec import whose configuration is not in the folder CrowdSec reads, keeps the current companion files an archive does not contain (and reports their names), makes the data lock abandonable when a request ends, classifies a missing hub item precisely, matches protected paths regardless of letter case, translates the remaining configuration page messages, and fixes two test-environment issues.

## Verdict: PASS (no blocking findings)

No critical, high or medium findings. One test defect was found and fixed in the branch (F1); two informational observations (F2, F3) are listed. No model, migration or GORM query file is in the diff, and no security-sensitive defect was found in the import path.

## Definition of Done evidence

| Check | Command | Result |
|---|---|---|
| Build | `cd backend && go build ./...` | pass |
| Backend tests and coverage | `scripts/go-test-coverage.sh` | first run: one failing test (F1); after the fix: all packages pass, statements 91.4%, lines 90.4% (gate 87%) |
| golangci-lint | `cd backend && golangci-lint run --config .golangci.yml ./...` (v2.14.0) | 0 issues |
| Frontend type-check and build | `cd frontend && npm run type-check && npm run build` | pass |
| Frontend coverage | `scripts/frontend-test-coverage.sh` | all tests pass; statements 90.38%, branches 83.92%, functions 88.34%, lines 91.53% (gate 87%) |
| Patch coverage | `bash scripts/local-patch-report.sh` (strict) | overall 91.6% (416/454), backend 92.0% (379/412), frontend 88.1% (37/42), agent n/a; all above the 87% gate; artifacts `test-results/local-patch-report.{md,json}` produced |
| CodeQL | `lefthook run codeql` (CLI 2.27.2) | Go: 4 results, all 4 pre-existing suppressed entries in files outside this diff; JS: 0 results; 0 blocking; parity check passed |
| Trivy | `trivy fs --scanners vuln,misconfig --severity HIGH,CRITICAL` on go.mod/go.sum and the frontend package files (copied under `$HOME` because the snap trivy cannot read `/projects`, removed afterwards) | no HIGH/CRITICAL |
| GORM scan | not run | no `backend/internal/models/**`, migration or GORM query file in the diff |
| E2E | rebuilt `charon-e2e` with `docker-rebuild-e2e`; `npx playwright test tests/security/crowdsec-import.spec.ts tests/security/crowdsec-diagnostics.spec.ts --project=security-tests` | 72 passed, 0 failed, 0 fixme or skipped (46.7 s) |

Patch coverage detail (lines still uncovered in the patch): `companions.go` 89.0% (the remaining branches are write and close failures on a file just created, and the lost-race `O_EXCL` case, which need fault injection), `archive_inspect.go` 88.2% (defensive read errors), `crowdsec_handler.go` 92.1% (the 500 branch when keeping the current files fails, lines 824-828), `CrowdSecConfig.tsx` 86.5% (error toasts).

## Security review

### 1. Import path: nothing destructive before validation

Order in `ImportConfig`: upload saved to a private temp folder, archive validator (size, ratio, required file, traversal), archive inspection and notification check, then the data lock, then the layout check, and only then the snapshot (a copy) and the clearing of the old tree. Every refusal before the snapshot changes nothing. After the snapshot, every failure calls `rollback()`: clearing failed, extraction failed, the flat-archive re-check after extraction, failure to keep the current files, and the kept-profile verification. The flat-archive refusal happens both before the snapshot (from the inspection) and after extraction (when the inspection was skipped), and returns a fixed message.

### 2. Archive entry names

`ArchiveEntryRel` is the single normalisation shared by the validator (`requireArchiveFile`), the inspector and the extractor. Absolute names are re-rooted below the destination (as the extractor does), `..` sequences that would leave it are reported as invalid, `./` and doubled slashes collapse, and the required-file check only accepts the top level or `config/`, so a copy such as `parsers/config.yaml` does not count. Case variants are different names on the case-sensitive live filesystem and are never treated as the real configuration; the protected-path predicates (`IsHubPath`, `IsEngineOwnedPath`, `isLiveDBFile`) now lower-case their input, so a differently cased spelling of a protected path is skipped too (conservative direction).

### 3. Links and file writes

- Presence of a companion in the live tree is decided with `Lstat`, so a link counts as present and is never replaced or written through. Snapshot entries that are links or have the wrong type are skipped and logged with a fixed message.
- Notification files are restored with `O_WRONLY|O_CREATE|O_EXCL` and mode 0600, after an `Lstat` of the destination; an existing path of any kind (regular file, link, dangling link) is skipped, never written through. The notifications folder is only created when absent, and refused when it is a link or a file. The new unit tests pin each of these (regular, linked and dangling destinations, linked folder, file in the way, read-only folder).
- Only direct children of `notifications/` with a lowercase `.yaml` or `.yml` name are considered, and only files that define a name the kept profile needs are copied back; a restore that would leave a name defined twice is refused (422) and rolled back.

### 4. Secrets and responses

`kept_files` carries file names only (fixed companion names plus `notifications/<file name>`), never contents. Error messages are fixed strings or sanitised, length-bounded name lists; I/O errors are wrapped through `withoutPath` so paths are not returned or logged; log fields pass through `SanitizeForLog`. No token or key is logged in the new code.

### 5. Locking (`ctxMutex`)

`LockContext` returns an error without taking the lock when the context ends first (an already ended context never takes a free lock); `Unlock` panics when not held. All call sites register `defer Unlock()` only after a successful lock (`lockData` returns false with the lock not held and the callers return before the defer; `writeExportLocked`, `hubUpdate`, `installedHubSet` and `installHubItem` return before the defer on error). No double unlock and no leaked lock was found. The restore single-flight flag is a never-waited compare-and-swap taken before any `dataMu` section and the import never takes it, so the lock order `restore flag -> dataMu` cannot invert. A cancelled waiter releasing nothing is covered by the new cancel tests, and the earlier observation about a non-context-aware lock no longer applies.

### 6. Hub item classification

`hubItemNotFound` now matches only cscli's sentence for the requested type and name; any other wording is `install_failed` (safe direction). A failed second read of the installed list now returns 500 with what already happened instead of claiming items were not attempted.

### 7. Frontend

Archive-derived names are rendered as React text (no `dangerouslySetInnerHTML` or `innerHTML` in the page or the helper); `sanitizeKeptFileName` strips control, format (including direction override) and line/paragraph separator characters and caps the length at 80. The translation key sets for the page are identical in en, de, es, fr and zh (checked by the i18n key test).

### 8. Commits and docs

Subjects are generic; none uses the `(security)` scope; no session id or link in any message. Docs additions describe behaviour only.

## Findings

No critical, high or medium findings.

### Low

- **F1: low (test defect, fixed in this review).** `backend/internal/api/handlers/crowdsec_export_import_test.go` `TestEditorHidesPidBouncersAndHub` still asserted that `config/Hub/x.yaml` is writable, which contradicts the case-insensitive hub matching added by the protected-path commit. It failed deterministically in the handlers package (`go test` exit 1, although the coverage gate message said "met"). The assertion now expects the path to be protected. Evidence: the first `scripts/go-test-coverage.sh` run reported this single failure; after the fix the whole run passes.

### Informational

- **F2: info.** `InspectTree` reads the live tree and the pre-import snapshot following links (as CrowdSec does) to parse configuration and notification names. The snapshot keeps links as links, so a linked notifications folder in the live tree is read through, but the contents are only parsed for names and a file is copied back only when it defines a needed name and the file itself is a regular file. This needs an administrator-created link on the server; no action needed.
- **F3: info.** The 500 branch "failed to keep the current CrowdSec files" in `ImportConfig` (rollback after a companion restore error) has no handler-level test; the underlying function's error paths are covered in the package tests. Reaching it from the handler needs a failure injected into a copy, so it was not added.

## Known limitations (not re-opened)

Redacting sensitive values in notification files on export is tracked separately (#1551); the image ships no notification plugins.

## Commits added by this review

- `test: cover the defensive branches of the CrowdSec notification file restore and keep hub paths protected in the editor test` (new tests for `prepareNotificationsDir` 77.8% -> 100% and `writeNotificationFile` 62.5% -> 75%, plus the F1 assertion fix).
- `docs: update QA report for the CrowdSec follow-up batch` (this report).
