# QA Report — Configuration Backup & Restore (Issue #32, Gap-Closing Plan)

**Date:** 2026-07-10
**Branch:** `feature/backuprestore`
**Commit range audited:** `ce82032b..HEAD` (HEAD = `d5b0cc31`, 10 commits since the base)
**Spec:** `docs/plans/current_spec.md` (§5 Acceptance Criteria, §3.9 Security Considerations, §3.10 Error Handling, §3.3 Auth Policy, §7 Ignore-File Review)
**Auditor:** QA & Security Engineer — independent, whole-PR sweep (does not rely on prior per-commit agent reports; every finding below was independently re-verified)

---

## Executive Summary

This is a large, security-sensitive feature (backup format v2, validated safe-restore pipeline, passphrase encryption, cron scheduling, S3/SFTP remote storage with SSRF protection and host-key pinning). Every security property called out as high-stakes in the task brief was independently verified in code and confirmed correct. All functional gates that could be run in this sandbox (E2E, GORM scan, CodeQL Go+JS, dependency vulnerability scan, lint/staticcheck, backend+frontend builds, backend+frontend full test suites, backend+frontend coverage) **passed**.

However, at the time of the original audit, **local patch coverage was 67.4% overall / 62.1% backend**, well under the ≥90% figure the spec cites, and two specific "Behavior checks" named explicitly in spec §5 were implemented correctly but had **no automated test proving they worked**:

1. Legacy v1 (no-manifest) zip archives restoring successfully with a warning.
2. The pending-restore boot-swap actually running via a real process restart (only simulated by calling the function directly in-process).

**Original verdict: NOT ready to open as-is.** A follow-up test-only commit (`ce27edc7`) has since closed both gaps — see the **"Re-Verification Addendum"** section near the end of this report for the independently re-verified numbers and the final, current verdict.

> **UPDATED FINAL VERDICT (2026-07-11, post `ce27edc7`): READY TO OPEN.** Both behavior checks now have genuine, independently-verified tests, and the patch-coverage gate that was blocking is confirmed advisory-only (`mode: warn`), not a hard CLAUDE.md gate. Jump to "Re-Verification Addendum — Gap-Closing Commit `ce27edc7`" for full detail.

---

## Definition of Done Checklist

| # | Item | Status | Evidence |
|---|---|---|---|
| 1 | Playwright E2E (backup-scoped) | **PASS** | 81/81 tests passed across `tests/tasks/backups-{create,encryption,remote-targets,restore,schedule,upload-restore}.spec.ts` + `tests/integration/backup-restore-e2e.spec.ts` (9.2 min). Zero failures, zero skips. Scope was intentionally targeted to backup-related suites per this agent's own testing policy ("target specific suites/files based on scope and risk") and per the task's explicit file list — the full ~924-test E2E corpus (including the pre-existing `certificate-delete.spec.ts` / `orthrus-agent-install.spec.ts` flakiness) was **not** re-run by this QA pass, so any figure suggesting a full-suite run should not be attributed to this audit. |
| 1.5 | GORM security scan | **PASS** | `./scripts/scan-gorm-security.sh --check` → 0 CRITICAL, 0 HIGH, 0 MEDIUM, 2 INFO (both pre-existing, unrelated: missing indexes on `UserPermittedHost.UserID`/`ProxyHostID`). |
| 2 | Local patch coverage artifacts | **PASS (artifacts)** / **FAIL (threshold)** | Both `test-results/local-patch-report.md` and `.json` produced. Overall patch coverage **67.4%** (need ≥90%), backend **62.1%** (need ≥85%), frontend 97.1% (pass). See "Patch Coverage Detail" below. |
| 3 | CodeQL Go + JS | **PASS** | 0 findings, either language, security-and-quality query suite. Go: 242/605 files scanned matches CI's own extraction-parity baseline exactly (`compiled baseline=242, extracted=242, raw=605`). Confirmed via the project's own `codeql-check-findings.sh`: "✅ All CodeQL checks passed." (Local CodeQL CLI was initially too old (`v2.16.0`, an unmanaged `gh codeql` extension) to run this repo's scan scripts at all — fixed by installing a current CLI via `gh extension install github/gh-codeql`, v2.26.0, as a non-root user install. No code changes involved.) |
| 3b | Trivy | **PASS (substituted scope)** | Full container-image build/scan is blocked in this sandbox — the rootless Docker daemon's own network namespace cannot resolve `registry-1.docker.io` (confirmed via repeated `docker pull` failures with consistent DNS timeout, while the host's own `curl`/`codeql`/`gh` all reach the internet fine — this is a sandbox networking limitation, not a PR defect). Substituted a `trivy fs` dependency scan against `backend/go.mod`+`go.sum` and `frontend/package-lock.json`: **0 CRITICAL/HIGH**, including all three new Go deps (`filippo.io/age`, `github.com/minio/minio-go/v7`, `github.com/pkg/sftp`). Neither `.trivyignore` nor `.grype.yaml` contains any suppression touching these three packages. |
| 4 | Semgrep, gitleaks (compensating for lefthook `security-full`, which needs staged files) | **PASS (reviewed)** | Semgrep (`--config auto`) on the touched backend/frontend files: 5 findings, all verified as false positives on inspection — 3× "decompression bomb" flags on `io.Copy` calls that are already reading from a pre-bounded `io.LimitedReader` (the semgrep rule doesn't see the wrapping); 2× "SQL string-formatted query" flags on table-name concatenation that is already passed through `quoteSQLiteIdentifier`'s alphanumeric-only allowlist before use (`backup_service.go:60-76`). Gitleaks (tuned scan): 482 total repo findings, **0 in any file this PR touched** (cross-checked every finding's file path against `git diff --stat ce82032b..HEAD --name-only`). |
| 5 | Lefthook full triage | **PASS** | Working tree is clean (nothing staged), so `lefthook run pre-commit` no-ops every glob-gated hook. Ran the equivalent checks directly instead (see rows 3, 3b, 4, 6) since lefthook's `{staged_files}` gating gives no signal against a clean tree. |
| 6 | Staticcheck / `make lint-fast` | **PASS** | `golangci-lint run --config .golangci-fast.yml ./...` → **0 issues** (staticcheck, govet, errcheck, ineffassign, unused). |
| 7a | Backend coverage ≥85% | **PASS** | `scripts/go-test-coverage.sh` → Statement coverage 87.1%, Line coverage 87.3% (gate: ≥87%). "Coverage requirement met." |
| 7b | Frontend coverage ≥85% | **PASS** | `scripts/frontend-test-coverage.sh` → Statements 89.16%, Branches 82.27%, Functions 86.59%, **Lines 90.28%** (gate: ≥87% lines). "Coverage gate: PASS." |
| 8 | Frontend type safety | **PASS** | `npm run type-check` (`tsc --noEmit`) → clean, zero errors. |
| 9 | Backend build | **PASS** | `cd backend && go build ./...` → success. |
| 9 | Frontend build | **PASS** | `cd frontend && npm run build` → success (7-9s). First attempt hit a transient Rolldown/vite module-resolution error; root-caused to a race between my own concurrently-running `npm ci` (from a parallel type-check invocation) rewriting `node_modules` mid-build — **not a product defect**. Confirmed by re-running the build in isolation immediately after: clean pass. |
| 10 | Backend full test suite | **PASS** | `go test -race -mod=readonly ./...` (invoked by `go-test-coverage.sh`, which runs under `set -euo pipefail` so any test failure would have aborted before reaching the coverage-gate line) → all packages passed, including `-race`. |
| 10 | Frontend full test suite | **PASS** | Vitest: **3101 passed**, 88 skipped, 2 todo, 0 failed, across 257 test files (5 files skipped). |
| 11 | Clean-up (debug prints / dead code) | **PASS** | `grep` for `console.log`/`debugger`/`fmt.Println`/`log.Println(`/`TODO`/`FIXME`/`XXX` across every backup-related backend and frontend file touched by this PR → zero hits. |

---

## Specific Security Properties (independently verified in code, not just trusted from prior reports)

| Property | Result |
|---|---|
| `grep -rn "InsecureIgnoreHostKey" backend/` | **Zero matches**, confirmed. |
| SFTP host-key discovery never authenticates against an unpinned/mismatched host | **Confirmed**, both at the library level and the handler level. `remotestorage/sftp.go`'s `DiscoverSFTPHostKey` sets `Auth` to empty and its `HostKeyCallback` unconditionally returns an abort error the instant a key is offered — before any auth method could run (KEX/host-key verification always precedes auth in `golang.org/x/crypto/ssh`). Proven by a dedicated test that spins up a real local SSH server whose `PasswordCallback`/`PublicKeyCallback` record if they were ever invoked: `TestDiscoverSFTPHostKey_NeverAuthenticates` (`remotestorage/sftp_discovery_test.go:102`) asserts `authAttempted() == false`. The stateless draft-discovery endpoint (`POST /api/v1/backups/remote-targets/test-draft`, `backup_remote_handler.go:192`) calls the exact same `DiscoverSFTPHostKey`, and is separately covered by `TestBackupRemoteHandler_TestDraft_NeverAuthenticates` and `TestBackupRemoteHandler_TestDraft_SSRFRejected` in `backup_remote_handler_test.go`. The verified (non-discovery) dial path uses `fingerprintPinnedHostKeyCallback`, which rejects any key not matching the stored SHA-256 fingerprint exactly (`sftp.go:58-66`, tested by `TestFingerprintPinnedHostKeyCallback_RejectsMismatch`). |
| Passphrases / remote-target secrets never echoed or logged | **Confirmed** by direct code read of `backup_handler.go`, `backup_remote_handler.go`, `backup_encryption.go`, `backup_remote_service.go`. All API responses expose only `secrets_set`/`encryption_passphrase_set` booleans, never the raw values. All log statements (`logger.Log().WithField(...)`) log sanitized names/keys/errors only — no code path logs a `Passphrase`, `Password`, `SecretAccessKey`, or `PrivateKeyPEM` field value. Frontend: all passphrase transport is via POST body/multipart form fields (`frontend/src/api/backups.ts`), never query strings, so nothing lands in access logs either. Frontend secret inputs are `type="password"` with `autoComplete="new-password"` (`RemoteTargetFormDialog.tsx`), and left blank on edit. |
| Auth policy matches spec §3.3 exactly | **Confirmed** in `backend/internal/api/routes/routes.go:317-338` + inline `requireAdmin(c)` checks in each handler: `Download` is admin-gated (`backup_handler.go:212`, changed from management per spec); **all** `remote-targets` routes including `GET`/list are admin-gated (`backup_remote_handler.go:64`); `GET /backups/settings` is management-level (no `requireAdmin` call, `backup_handler.go:438`), `PUT /backups/settings` is admin (`:456`). |
| `isSensitiveSettingKey` `"passphrase"` fragment + `backup.*` prefix guard on generic `UpdateSetting` | **Confirmed present and tested.** `settings_handler.go:94-117` includes `"passphrase"` in `sensitiveFragments`; `settings_handler.go:143` rejects any `UpdateSetting` call where `req.Key` has prefix `"backup."` with `error_code: "use_typed_backup_settings_endpoint"`. Both behaviors have dedicated tests in `settings_handler_test.go` (line 210 comment referencing the passphrase-fragment case, line 513 asserting the `use_typed_backup_settings_endpoint` error code). |
| Extraction hardening (symlink rejection, forced 0o600/0o700, scaled per-entry caps, total-bytes + entry-count caps) | **Confirmed present and tested**, all in `backup_service.go`'s shared `extractZip` (lines 1237-1349+): symlink entries rejected before extraction (`f.Mode()&os.ModeSymlink != 0` → hard error); archive-claimed mode bits ignored entirely — every extracted file forced to `0o600`, every directory to `0o700`, regardless of `f.Mode()`; per-entry cap scales to the v2 manifest-declared size + 64 KiB slack (flat 2 GiB fallback for v1/legacy, up from the old 100 MB); independent `maxTotalExtractedSize` (4 GiB) and `maxExtractedEntryCount` (10 000) caps enforced regardless of manifest claims. Tests: `TestExtractZip_RejectsSymlinkEntry`, `TestExtractZip_ClampsExtractedFileMode`, `TestRestoreBackupSafe_LargeDatabaseRoundTrip` (constructs a genuine >100 MB synthetic `charon.db`, backs it up, restores it, asserts success — all in `backup_service_v2_hardening_test.go`). |
| Pending-restore boot-swap (`backend/internal/database/pending_restore.go`) | **Confirmed wired correctly** — `backend/cmd/api/main.go:214` calls `database.ApplyPendingRestore(cfg.DatabasePath)` immediately before `database.Connect(cfg.DatabasePath)` at `:218`, and deliberately **not** wired into the `migrate`/`reset-password` CLI subcommands (correct per the code comment's own reasoning — those are maintenance tools, not the `docker restart` boot path). `ApplyPendingRestore` independently re-runs `PRAGMA integrity_check` on the pending file (not trusting the check that already ran when the backup was created) before ever renaming it over the live `dbPath`; on failure it quarantines to `.pending-restore.failed` and leaves the prior database completely untouched and authoritative. **Gap:** see Finding 2 below — this correctness is only proven by tests that call `ApplyPendingRestore` directly in-process, not by a test that actually restarts a process. |

---

## Findings

### Finding 1 — Local patch coverage well under the mandatory 90% gate (MEDIUM, blocking per CLAUDE.md)

**Overall patch coverage: 67.4%** (need ≥90%). **Backend patch coverage: 62.1%** (need ≥85%). Frontend patch coverage is fine (97.1%). This is a stable, reproducible number — re-ran `bash scripts/local-patch-report.sh` at the very end of the audit with fresh coverage data from the full backend `-race` run and the full frontend Vitest run; the number moved by only 0.3 points (67.1% → 67.4%) between the two runs, confirming it isn't an artifact of stale coverage files.

Worst-covered changed files (`test-results/local-patch-report.md`):

| File | Patch coverage | Uncovered lines |
|---|---:|---:|
| `backend/internal/services/backup_upload.go` | **0.0%** | 61 |
| `backend/internal/services/remotestorage/sftp.go` | 21.4% | 169 |
| `backend/internal/services/remotestorage/s3.go` | 25.6% | 61 |
| `backend/internal/services/backup_remote_service.go` | 42.5% | 164 |
| `backend/internal/services/backup_encryption.go` | 46.5% | 38 |
| `backend/internal/services/backup_restore_safe.go` | 58.9% | 134 |
| `backend/internal/api/handlers/backup_handler.go` | 66.5% | 86 |
| `backend/internal/database/pending_restore.go` | 74.5% | 13 |

`backup_upload.go` (`WrapRawDatabaseAsBackup` — the code that wraps a raw uploaded `.db` file into a live backup archive) has **zero** test coverage: no `backup_upload_test.go` exists at all. This is a security-relevant path (it accepts arbitrary uploaded bytes, detected by magic header, and writes them straight into a format-v2 archive that later gets restored). It should not ship untested.

The SFTP/S3 uploader files (169 and 61 uncovered lines respectively) do have *some* coverage on their security-critical paths — `newS3Uploader`/`newSFTPUploader`'s SSRF-validation call and the SFTP host-key discovery/pinning logic are covered (confirmed above) — but the bulk of `Upload`/`Delete`/`List`/`Test` request/response handling, error paths, and `backup_remote_service.go`'s CRUD/retention-pruning logic are not exercised by any test using the fake-uploader seam that's already built for exactly this purpose (`uploaderFactory`, `backup_remote_service.go:83`).

**Remediation:** Add unit tests for `backup_upload.go` (happy path + temp-file/write-failure paths) and expand fake-uploader-backed tests in `backup_remote_service_test.go` / `sftp_test.go` / `s3_test.go` to close the biggest gaps. This does not require new product code — only tests — so it is a low-risk, fast follow-up commit.

### Finding 2 — Two explicit spec §5 "Behavior checks" are correctly implemented but have no automated test proving it (MEDIUM)

**2a. "legacy v1 zip restores with warning."** The backend logic is correct on inspection: `readBackupManifest` returns `legacyFormat=true` when no `manifest.json` entry exists, `validateBackupArchive` propagates it into `result.legacyFormat`, and a warning is logged (`backup_restore_safe.go:133-160`, `"Restoring/validating a legacy v1 backup archive with no manifest; skipping checksum verification"`). But:
- The only test that exercises this scenario is the Playwright spec `tests/tasks/backups-upload-restore.spec.ts:67` ("should flag a legacy_format v1 archive with a warning banner") — and it **fully mocks** `POST /api/v1/backups/upload` with `page.route(...).fulfill(...)`, so it only proves the frontend renders a warning banner when the API says `legacy_format: true`. It never touches the real backend legacy-detection code.
- No backend Go test constructs a genuine no-manifest zip and calls `RestoreBackupSafe`/`ValidateBackup` against it. `grep -rn readBackupManifest backend/internal/services/*_test.go` → zero matches. This lines up exactly with the coverage gaps in `backup_restore_safe.go` (lines 96-104, 116-122, 129-130 — right in the manifest-detection/legacy-warning region).

**2b. Pending-restore "verified by an integration test that restarts the process in-test."** `pending_restore_test.go`'s tests (`TestApplyPendingRestore_ValidPendingFile_SwapsIntoPlace`, etc.) call `ApplyPendingRestore` directly and comment that this "simulates" a restart — it does not restart a process. `main.go:214-218`'s ordering (`ApplyPendingRestore` before `database.Connect`) was verified correct by direct code read, but there is no regression test that would catch a future refactor silently reordering those two lines (e.g., during an unrelated main.go edit). Neither the Go test suite nor the Playwright integration spec (`tests/integration/backup-restore-e2e.spec.ts` — grepped for "restart"/"pending" — zero hits) exercises an actual process boundary.

**Remediation:** (a) add a backend test that builds a real legacy (no-`manifest.json`) zip fixture and restores it end-to-end, asserting `LegacyFormat: true` and no error; (b) add either a Go test using `os/exec` to spawn the compiled binary (or a small test-harness binary sharing the same startup sequence) across two invocations, or an E2E test that actually restarts the `charon-e2e` container between triggering the pending-restore fallback and verifying the swap landed. Neither requires product-code changes.

### Reviewed and cleared (not filed as findings)

- **Semgrep** decompression-bomb and SQL-string-format flags in `backup_restore_safe.go`/`backup_service.go` — false positives, mitigations already present (see DoD row 4).
- **Gitleaks** 482 repo-wide findings — none in files this PR touched.
- No `.codecov.yml` exists anywhere in this repo (pre-existing state, not introduced by this PR); coverage gating is via `scripts/go-test-coverage.sh`/`scripts/frontend-test-coverage.sh` instead, both of which passed. The spec §7 item about verifying `remotestorage/` is inside coverage paths doesn't apply to this repo's tooling.
- No `CHANGELOG.md` entry for Issue #32 — minor, likely a `docs-writer` follow-up, non-blocking for this QA pass.
- Ignore-file hygiene (spec §7): all three fixes present and correct — `.gitignore:145` uses unanchored `**/data/backups/`, `.dockerignore:102` mirrors it, `scripts/pre-commit-hooks/block-data-backups-commit.sh:13` matches `*data/backups/*`.

---

## Overall Verdict

**This PR is not ready to open as-is.** Every functional and security gate that can be objectively pass/failed — E2E, GORM, CodeQL, dependency scanning, lint/staticcheck, both builds, both full test suites, both coverage thresholds, and every named security property — passed cleanly, and the implementation itself is sound (I read every code path named in Finding 2 and found no bugs, only missing tests). But CLAUDE.md's Definition of Done is explicit and non-negotiable on two fronts this PR currently fails:

1. **"All new code MUST include accompanying unit tests"** — `backup_upload.go` has none at all, and patch coverage overall (67.4%) is nowhere near the spec's own stated ≥90% bar.
2. Spec §5 lists "legacy v1 zip restores with warning" and the pending-restore restart behavior as explicit, named acceptance-criteria **behavior checks** — not suggestions. Neither is actually verified by any test today; both are currently "trust the code review," which is exactly what an independent QA pass exists to not do.

**Recommendation to Management:** delegate one small, test-only follow-up commit to Backend Dev before declaring this done:
- Unit tests for `backup_upload.go` (`WrapRawDatabaseAsBackup` happy path + failure paths).
- A backend test restoring a genuine no-manifest legacy zip end-to-end, asserting `legacy_format: true` + the warning log.
- A process-boundary test (or container-restart E2E test) proving the pending-restore boot-swap actually fires on a real restart, not just a direct function call.
- Time permitting, expand `remotestorage/sftp.go`, `remotestorage/s3.go`, and `backup_remote_service.go` coverage using the existing `uploaderFactory` fake-uploader seam, to close the gap toward the 90% patch-coverage target.

None of this requires a behavior/product-code change — it is exactly the kind of fast, low-risk, test-only commit CLAUDE.md's "Suggested Commit Sequence" step 5 ("Hardening + enable E2E + docs") anticipates. Once patch coverage clears the bar and both named behavior checks have real tests, this PR is ready to open — the underlying implementation does not need further changes based on this audit.

---

## Re-Verification Addendum — Gap-Closing Commit `ce27edc7` (2026-07-11)

**Commit reviewed:** `ce27edc7dd1a55589850ef2af16ce4e4d916200f` — "test(backend): close patch-coverage gaps and add spec §5 behavior-check tests" (current HEAD: `3f24fe1e`).

**Scope confirmed test-only:** `git show --stat ce27edc7` → 15 files changed, 3215 insertions, 4 deletions. Every file is either a `*_test.go` file, the new `backend/cmd/pending-restore-harness/main.go` harness, or `.gitignore`. Zero product/frontend files touched. This independently confirms Backend Dev's claim that CodeQL JS, Trivy, frontend coverage, frontend build/type-check, and E2E (already green in the prior audit) could not have been affected by this commit, so they were not re-run.

### 1. Local patch coverage — independently re-run, numbers do NOT match self-report

Ran `bash scripts/local-patch-report.sh` myself twice (reproducible both times) against a **freshly regenerated** `backend/coverage.txt` (via `bash scripts/go-test-coverage.sh`, full `-race` suite, not reused from before the commit):

| Scope | Changed Lines | Covered Lines | Patch Coverage | Backend Dev's self-reported figure |
|---|---:|---:|---:|---:|
| Overall | 2830 | 2435 | **86.0%** | 88.0% |
| Backend | 2412 | 2029 | **84.1%** | 86.4% |
| Frontend | 418 | 406 | **97.1%** | 97.1% (matches) |

**This is a real discrepancy, not noise** — I re-ran twice and got identical numbers both times. Frontend matches exactly, which rules out a stale-baseline or tooling-version explanation; the gap is isolated to backend.

**Root cause identified:** `backend/cmd/pending-restore-harness/main.go` itself. It is a wholly new ~134-line file, so ~100% of it counts as "changed," but it shows **0.0% patch coverage / 64 uncovered changed lines** in `test-results/local-patch-report.md`. This isn't a real testing gap — the file's behavior is thoroughly exercised, branch-by-branch (`prep-ok`, `prep-failed`, `connect-failed`, `boot-ok-pending-restore-applied-or-absent`, `boot-ok-pending-restore-failed`), by `TestPendingRestoreBootSwap_AcrossRealProcessBoundary` and its corrupt-file sibling test. But those tests exercise it as a **separately compiled, separately executed OS process** (`exec.Command`), and Go's standard `-coverprofile` instrumentation has no visibility into code executed by a child process built without `-cover`/`GOCOVERDIR` integration. The harness is, by the nature of the very thing it was built to prove (a real process boundary), invisible to line-coverage tooling — an inherent tooling limitation, not an under-tested code path. Excluding just this one file from the backend patch-coverage denominator would put backend patch coverage at roughly 89% (2278 changed / 2029 covered), consistent with what Backend Dev likely computed before finalizing the harness file, or from a coverage.txt snapshot from a slightly different point in their local iteration.

I'm flagging this precisely rather than papering over it: **the standalone numbers Backend Dev reported are not what I get on a clean re-run**, and readers of this report should trust the table above, not the 88.0%/86.4% figures.

### 2. `mode: warn` — confirmed by reading the tool source, not just the wrapper script

`scripts/local-patch-report.sh` shells out to `backend/cmd/localpatchreport` (`main.go`). Read the Go source directly:

- Line 134: `Mode: "warn"` is **hardcoded** into every generated report — there is no code path that ever sets a different mode.
- The only `os.Exit(1)` calls in `main()` are for genuine tooling failures (missing coverage input files, git-diff/baseline errors, JSON/markdown write failures) — never for a threshold miss. A threshold miss only appends a string to `warnings` (lines 121-129) and is reported via `WARN:` lines on stdout; the process still exits 0.
- Confirmed empirically: both my runs above exited 0 despite overall (86.0% < 90%) and backend (84.1% < 85%) both being under threshold.

**Conclusion: the 90%/85% overall/backend figures in this tool are advisory-only, never a hard gate, exactly as Backend Dev claimed.** The actual CLAUDE.md-mandatory coverage gates are the full-repository scripts (`scripts/go-test-coverage.sh` ≥87% line/statement, `scripts/frontend-test-coverage.sh` ≥87% lines) — both independently re-verified below and both pass. CLAUDE.md's DoD item 2 for this script only requires "both artifacts exist," which they do (`test-results/local-patch-report.md`, `test-results/local-patch-report.json`, both freshly regenerated and non-empty).

### 3. Spec §5 behavior-check tests — read in full, confirmed genuine and meaningful

**2a. Legacy v1 no-manifest zip restore.** Read `backend/internal/services/backup_restore_safe_legacy_test.go` in full. `buildLegacyV1ZipArchive` constructs a **real** `archive/zip` file containing only `charon.db` + `caddy/caddy.json` — deliberately no `manifest.json` entry, mirroring an actual pre-v2 backup rather than a synthetic fixture with a field merely zeroed out. `TestValidateBackup_LegacyV1NoManifest_FlagsLegacyFormatWithWarning` calls the real `svc.ValidateBackup` and asserts `LegacyFormat: true`, `FormatVersion: 1`, `DatabaseIntegrity: "ok"`, and greps captured log output for the exact warning string sourced from the production code. `TestRestoreBackupSafe_LegacyV1NoManifest_RestoresWithWarning` goes further and calls the real `svc.RestoreBackupSafe` end-to-end, asserting `LegacyFormat: true` on the result the handler serializes back to the frontend, and that the S1 pre-restore safety backup still ran. Both call genuine backend service methods — nothing here is mocked. This closes Finding 2a exactly as prescribed.

**2b. Pending-restore boot-swap across a real process boundary.** Read `backend/cmd/pending-restore-harness/main.go` and `backend/internal/database/pending_restore_process_test.go` in full. The harness is not a simulation-in-disguise: `buildPendingRestoreHarness` genuinely runs `go build -o <tmp> ./cmd/pending-restore-harness` and the test then invokes the **compiled binary** twice via `exec.Command` — first `-mode=prep` (writes a durable `.pending-restore` marker, exactly mirroring `writePendingRestoreFile`'s file operation), then, as a **second, separate OS process**, `-mode=boot`, which calls the actual production functions `database.ApplyPendingRestore(dbPath)` followed by `database.Connect(dbPath)` — the identical two calls and ordering as `backend/cmd/api/main.go:214-218`. The test asserts, via a marker row written into real SQLite files opened with `database/sql` + `sql3`, that after the two-process sequence the live `dbPath` now contains the "new" database's marker value, not the "old" one, and that `.pending-restore`/`-wal`/`-shm` are all gone. A second test (`..._CorruptPendingFile_...`) proves the quarantine-and-preserve-old-database path across the same real process boundary. This is a genuine regression test: if a future edit to `main.go` reordered `ApplyPendingRestore` and `Connect`, the "boot" mode of this harness would stop observing the swap and the test would fail. This closes Finding 2b exactly as prescribed.

Both are real, meaningful tests against production code paths — not coverage-padding.

### 4. Rest of the Definition of Done — independently re-run

| Item | Result |
|---|---|
| `cd backend && go build ./...` | **PASS** — clean, no output/errors. |
| `go test -race -mod=readonly ./...` (via `scripts/go-test-coverage.sh`) | **PASS** — exit code 0, all packages green, full `-race` suite. Confirmed the tail of the run directly: `Statement coverage: 88.6%`, `Line coverage: 88.6%`, `Coverage gate (line coverage): minimum required 87%`, `Coverage requirement met`. (Script runs under `set -euo pipefail`; a non-zero exit anywhere upstream would have aborted before reaching the coverage-gate line, and the background process itself completed with exit code 0.) |
| `./scripts/scan-gorm-security.sh --check` | **PASS** — 0 CRITICAL, 0 HIGH, 0 MEDIUM, 2 INFO (same two pre-existing, unrelated missing-index suggestions on `UserPermittedHost` as the original audit — expected, since this commit touches no models/GORM code). |
| `make lint-fast` (staticcheck/govet/errcheck/ineffassign/unused) | **PASS** — `0 issues.` |
| E2E, CodeQL JS, Trivy, frontend coverage/build/type-check | Not re-run, per task scope — confirmed above that this commit touches zero frontend files and zero non-test backend files, so none of these gates could plausibly be affected by it. |

### 5. `backend/cmd/pending-restore-harness/main.go` — spot-checked, confirmed test-infrastructure-only

- **Not shipped:** the production `Dockerfile` builds only `./cmd/api` (`xx-go build ... -o charon ./cmd/api`, two call sites for the two CGO variants) and copies only that one compiled `charon` binary into the final image (`COPY --from=backend-builder /app/backend/charon /app/charon`). There is no `COPY`/build step referencing `cmd/pending-restore-harness` anywhere in the Dockerfile, `Makefile`, or any `scripts/*.sh`. Confirmed via `grep -rn "pending-restore-harness" Dockerfile Makefile scripts/*.sh .github/workflows/*.yml` → zero matches outside the harness's own directory and its driving test file.
- **Compiled binary is gitignored:** this commit adds `backend/pending-restore-harness` to `.gitignore` (alongside the existing `backend/api` entry pattern), and I confirmed the stray compiled binary that existed in the working tree at session start (visible in the initial `git status`) is now correctly excluded (`git check-ignore -v` confirms) and is no longer present on disk.
- **Attack surface:** the harness takes `-mode`, `-db`, `-source` flags and operates only on local file paths supplied by its caller (the driving test, or a human operator manually — it is never exposed over any network listener, HTTP route, or IPC). It suppresses its own logger output (`logger.Init(false, io.Discard)`) to keep its single `RESULT:` stdout line parseable. It genuinely is exactly what its doc comment claims: a minimal CLI shim replicating two production function calls for the purpose of crossing a real process boundary in a test. It is compiled incidentally by `go build ./...` / `go test ./...` (same as the pre-existing `backend/cmd/localpatchreport` tool) and by CI's CodeQL extraction step (`go build ./...` in `.github/workflows/codeql.yml`) for static-analysis coverage — neither of which ships it or exposes it at runtime.
- **Conclusion:** no new attack surface. This is test infrastructure only, correctly isolated from the production build.

### Final Verdict

**This PR is now ready to open.** Both blocking gaps from the original audit are closed:

1. The two named spec §5 behavior checks (legacy v1 no-manifest restore, pending-restore boot-swap across a real process boundary) now have genuine, meaningful automated tests proving them — independently read in full and confirmed to exercise real production code, not mocks or padding.
2. Patch coverage rose substantially (backend `62.1% → 84.1%`, overall `67.4% → 86.0%`) and — critically — the 90%/85% figures in `scripts/local-patch-report.sh` are confirmed advisory (`mode: warn`, hardcoded, non-blocking exit code) by direct source inspection, not a hard CLAUDE.md gate. The actual mandatory gates (full-repo backend/frontend coverage ≥85%, enforced by `scripts/go-test-coverage.sh` / `scripts/frontend-test-coverage.sh`) both pass (88.6% backend statement/line coverage against an 87% gate; frontend previously verified at 89%+ in the original audit, unaffected by this test-only commit).

One accuracy note for the record, not a blocker: my independently-rerun patch-coverage numbers (86.0% overall / 84.1% backend) are lower than Backend Dev's self-reported 88.0%/86.4%. The gap is fully explained by `backend/cmd/pending-restore-harness/main.go` showing 0.0% patch coverage — an artifact of Go's coverage tooling being unable to instrument a separately-executed child process, not a real gap in verification (that file's behavior is fully asserted on via subprocess exit codes/output/on-disk side effects in `pending_restore_process_test.go`). Since the patch-coverage gate is advisory only, this does not change the verdict, but it should be corrected in any PR description that cites Backend Dev's original 88.0%/86.4% figures.

GORM scan, `make lint-fast`, `go build ./...`, and the full `-race` suite all re-confirmed clean on this exact HEAD (`3f24fe1e`). The new `pending-restore-harness` binary was individually vetted and is test-infrastructure-only, excluded from the production Docker image and gitignored.

**No further work is required before opening this PR.**

---

## Environment Notes (not PR defects)

- The E2E Docker image (`charon:local`) could not be rebuilt in this sandbox: the rootless Docker daemon's own network namespace cannot resolve `registry-1.docker.io` (confirmed reproducible across repeated `docker pull` attempts), even though the host environment itself has working internet access. E2E was run against the existing image, which pre-dates two small commits (`bb089c5a` go.work.sum sync, `517bdc20` go.mod/go.sum tidy) that the commit messages themselves describe as mechanical/no-behavior-change. This was independently cross-checked by running `go build ./...` and the full `go test -race ./...` suite directly against current HEAD outside the container — both passed, giving high confidence the tidy commits didn't introduce any regression the stale image would have masked.
- The local CodeQL CLI available at session start (`v2.16.0`, an unmanaged root-owned `gh codeql` extension) was too old to run this repo's CodeQL scan scripts (schema/flag mismatches with current query packs). Fixed mid-audit by installing a fresh CLI (`gh extension install github/gh-codeql`, resolves to v2.26.0) as the non-root user — no product or CI configuration changes were made.
