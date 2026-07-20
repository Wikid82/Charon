# QA / Security Definition-of-Done Report — Restore-Reliability Remediation (C1/H1/H2/M4/M5)

**Date:** 2026-07-14
**Branch:** `feature/backuprestore` (PR #1136)
**Scope:** Independent QA/Security re-verification of the 7 commits (`7d45a50d`..`ede4f406`) that remediate findings C1, H1, H2, M4, M5 from `docs/reports/pre_merge_audit_2026-07-14.md`, per the approved plan in `docs/plans/current_spec.md`.
**Relationship to prior reports:** Supplemental to `docs/reports/qa_report.md` (2026-07-13, READY TO MERGE) and independent of the Supervisor's APPROVED WITH MINOR NOTES review. Every gate below was re-run from scratch in this session; nothing is inherited from the prior reports without re-verification.

---

## 1. Summary Verdict

**READY TO MERGE.**

All mandatory gates pass. All 5 findings are correctly fixed/covered per the plan's acceptance criteria. The adversarial re-review found no new blocking defects in the 5 remediated areas. One low-severity, pre-existing-pattern cosmetic gap and one low-severity message-formatting edge case were found during the adversarial pass — both are documented as non-blocking follow-ups below.

---

## 2. Gate Results

| # | Gate | Result | Detail |
|---|---|---|---|
| 1 | GORM Security Scan (`./scripts/scan-gorm-security.sh --check`) | **PASS** | 0 CRITICAL, 0 HIGH, 0 MEDIUM, 2 pre-existing INFO suggestions (unrelated `UserPermittedHost` FK index hints). Triggered correctly by H1's `RehydrateLiveDatabase` GORM query change. |
| 2 | Backend coverage (`scripts/go-test-coverage.sh`) | **PASS** | 89.1% statement / 89.0% line coverage. Gate configured at 87% minimum — met with margin. Zero test failures. |
| 3 | Frontend coverage (`scripts/frontend-test-coverage.sh`) | **PASS** | 89.4% statements / 90.54% lines / 82.4% branches / 87.06% functions. Gate configured at 87% (lines) — met. 3137 tests passed, 88 skipped (intentional), 2 todo, **0 failed**. |
| 4 | Local patch coverage (`bash scripts/local-patch-report.sh`) | **PASS** | Overall 90.8% (3430/3779 changed lines), Backend 89.5%, Frontend 99.8%, against a 90% overall gate. Report generated at `test-results/local-patch-report.md` / `.json`. |
| 5 | Type safety (`cd frontend && npm run type-check`) | **PASS** | `tsc --noEmit` clean, zero errors. |
| 6 | Pre-commit hooks (`lefthook run pre-commit`) | **N/A as invoked / substitute run performed** | Working tree is clean (all 7 commits already committed), so lefthook's staged-file hooks all reported "skip — no matching staged files." Ran the underlying checks directly instead: `golangci-lint run --config .golangci-fast.yml` (staticcheck+govet+errcheck+ineffassign+unused) against `internal/services/...` and `internal/api/handlers/...` → **0 issues**. |
| 7 | Full backend test suite (`go test ./...`) | **PASS** | All packages pass. Confirmed via full coverage run (gate #2) plus targeted `-race` reruns of every H1/C1/H2/M4/M5 test (see §4) — zero failures, zero data races. |
| 8 | Full frontend test suite | **PASS** | Same run as gate #3 — 3137 passed, 0 failed. `RestoreDialog.test.tsx` re-run in isolation: 12/12 pass, including the updated i18n toast assertion. |
| 9 | Builds (`go build ./...`, `npm run build`) | **PASS** | Both succeed cleanly. Frontend production bundle built in 2.58s with no warnings of note. |
| 10 | Playwright E2E | **PASS (scoped subset, firefox only)** | See §3 for scope/rationale. |
| 11 | Trivy / CodeQL / staticcheck | **Partial — see below** | See §5. |

---

## 3. Playwright E2E

**MANDATORY prerequisite performed first:** the `charon-e2e` Docker container was built 2026-07-13 20:26 UTC, *before* all 7 remediation commits (which landed 2026-07-14 18:10–18:53 UTC) and contains real backend (`backup_service.go`, `backup_restore_safe.go`, `backup_handler.go`) and frontend (`RestoreDialog.tsx`) production-code changes — not test-only changes. Per `CLAUDE.md`'s mandatory workflow step 1, the image was rebuilt from scratch (`.github/skills/scripts/skill-runner.sh docker-rebuild-e2e`) before running any E2E tests. Rebuild succeeded; container came up healthy in ~5s.

**Scope actually run:** `--project=firefox` only, targeting every backup/restore-related spec file:
- `tests/integration/backup-restore-e2e.spec.ts`
- `tests/tasks/backups-restore.spec.ts`
- `tests/tasks/backups-upload-restore.spec.ts`
- `tests/tasks/backups-encryption.spec.ts`
- `tests/tasks/backups-create.spec.ts`

**Result: 63/63 passed** in 3.7 minutes (2 workers), including the restore-failure-toast and error-handling groups (`Group E: Error Handling`, `should handle restore failure gracefully`, `should handle restore of corrupted backup with appropriate error message`).

**Why not the full 3-browser matrix:** a full `chromium`+`firefox`+`webkit` run across the entire suite (not just backup/restore) was not attempted — it would cost significantly more wall-clock time than this scoped, risk-targeted subset for a change whose diff is confined to 4 backend files + 1 frontend file, none of which touch other feature areas. Firefox was chosen per `CLAUDE.md`'s "use `--project=firefox` for best local reliability" guidance. This is a deliberate scope reduction, not a silent skip — flagged here as requested. If full-matrix confidence is desired before merge, running chromium/webkit on just the same 5 spec files (not the full suite) would be the next-cheapest increment.

---

## 4. Adversarial Re-Review of the 5 Fixed Areas

This section documents genuine attempts to break each fix, not just confirmation that the shipped tests pass.

### C1 / H1 — `RestoreBackupSafe` double-failure / `RehydrateLiveDatabase` atomicity

**Question: can a later step (Caddy reload, `BackupRecord` status update) still get confused with the hard-failure path?**

Traced the full control flow in `backup_restore_safe.go:271-341`:
- `unrecoverableErr` is set in exactly one place — inside `if !rehydrated { if pendingErr != nil { ... } }`. It is never touched by the Caddy-reload block or by the `BackupRecord.Update("status", "restore_pending")` block, which are two textually-separate, mutually-exclusive branches (the status update only runs in the `else` arm, i.e. only when `pendingErr == nil`, i.e. only when `unrecoverableErr` is guaranteed nil).
- The Caddy reload block runs unconditionally after both branches (by explicit design, per the code comment "runs regardless of A2/F3's outcome, even in the unrecoverable case below") and only ever sets `result.CaddyReloaded` / logs a warning — it can never set or clear `unrecoverableErr`.
- Verdict: **no cross-contamination is possible.** A Caddy-reload failure or a `BackupRecord` status-update failure remains a soft, swallowed-warning failure in both the pre-fix and post-fix code, exactly as the plan's §3.7 table specifies, and cannot be mistaken for or masked by the new hard-failure path. This matches the design intent exactly.

**Question: is there any other path producing a partial/silent failure not covered by the new tests?**

Found one genuine (low-severity) gap: when `s.db == nil` (a configuration that cannot occur in production — `routes.go:228` always passes the real `*gorm.DB` — but is legitimate in several existing test helpers and any future refactor that constructs `BackupService` directly), the `if s.db != nil` retry block is skipped entirely, so `rehydrated` stays `false` and `rehydrateErr` stays its zero value (`nil`). If `writePendingRestoreFile` then also fails in this configuration, `unrecoverableErr`'s message is built as `fmt.Errorf("...: live database rehydrate failed (%v) and...", ..., rehydrateErr, ...)` with `rehydrateErr == nil`, producing a message that reads "*live database rehydrate failed (\<nil\>)*" — which is misleading, since in this configuration rehydrate was never attempted at all (no live DB was wired), not "attempted and failed." This is **not** a silent-success bug (the function still correctly returns a non-nil error wrapping `ErrRestoreUnrecoverable`), and it is **not reachable in production** given the current single production call site — it is a message-clarity gap that only a `s.db == nil` test/future-caller combination could hit. Logged as a low-severity follow-up (§6).

Re-verified independently with `-race`: all `RehydrateLiveDatabase`/`RestoreBackupSafe` tests pass under the race detector with zero data races (50.8s run, see gate #7). This specifically stresses the H1 transaction-wrap's `SetMaxOpenConns(1)` + single-connection ATTACH assumption.

### H1 — mid-loop rollback test quality

Read `TestBackupService_RehydrateLiveDatabase_MidLoopFailure_RollsBackAtomically` in full (`backup_service_rehydrate_test.go:250-304`). It is a genuine atomicity proof, not a shallow "an error was returned" check: `first_table` (processed and fully swapped *before* `second_table`'s `INSERT` fails on a UNIQUE-constraint violation) is asserted, row-for-row, to still hold its **pre-rehydrate** value (`old-first`) after the failure — proving the transaction wrap actually rolls back work that had already "completed" earlier in the same loop iteration sequence, which is exactly the scenario the pre-fix code was vulnerable to. `second_table` is also asserted to retain its original row rather than being left empty. This is the correct, non-trivial assertion the plan's acceptance criteria demanded.

### H2 — corrupted-fixture reliability and CI exercise

- **CI exercise:** `backup_restore_safe_integrity_test.go` is an ordinary `_test.go` file in `internal/services` with no build tags, `t.Skip`, or environment gating — it runs under plain `go test ./...`, identically in CI and locally. Confirmed no `//go:build` constraints on the file.
- **Fixture determinism:** ran `TestSanityCheckSQLiteFile_IntegrityCheckFailure_Rejected` 5 times consecutively with `-count=1` (forcing a fresh temp DB + fresh probe each run) — all 5 passed in ~0.01s each. A run finishing in 0.01s (rather than approaching the fatal timeout of scanning the whole file at 8-byte steps) indicates the probe finds a corrupting offset within the first few candidates near the start of the index page every time, not by exhausting a wide search — a good sign against flakiness. The design (linear probe with `t.Fatal` if none found, never `t.Skip`) is correct: a future SQLite/go-sqlite3 version that changed page-corruption reporting behavior would fail loudly and visibly rather than silently skip coverage.

### M4 — `CleanupOldBackups` edge cases

Read `TestCleanupOldBackups_ExcludesPreRestoreRecordsFromRetention` in full. It correctly proves exclusion from the retention **denominator**, not merely "spared deletion" (asserts `deleted == 5-keep`, i.e., the 2 `pre_restore` records don't count toward `keep`), and does so with `pre_restore` records deliberately given *older* timestamps than the survivors, ruling out "it just happened to be new enough" as an alternative explanation.

One adversarial edge case is **not** covered, but is a pre-existing, documented, non-regressing design trade-off rather than a gap introduced by this fix: `isPreRestoreBackup` (`backup_service.go:441-449`) treats *any* GORM lookup error — including a genuine transient DB error, not just "record not found" — as "not pre_restore," per its own doc comment ("best-effort exclusion, not a correctness-critical check"). A pre_restore record could theoretically be pruned if the lookup itself errors at exactly the wrong moment. This behavior is unchanged by M4's fix (M4 only added test coverage for the existing filter) and is explicitly out of scope per the plan's non-goals.

### M5 — `computeEncryptionKeyRequired` edge cases

Read `TestComputeEncryptionKeyRequired_PositiveAndNegative` in full. It correctly seeds `remote_storage_targets` and proves both the direct branch and the end-to-end `ValidateBackup.EncryptionKeyRequired` round trip. One minor coverage gap: the positive branch is only exercised via `remote_storage_targets`; the other two tables checked by the same loop (`dns_provider_credentials`, `tunnel_configs`) are not separately exercised for the `true` case. Given all three tables run through byte-for-byte identical code (`s.db.Table(table).Count(&count)`, `if count > 0 { return true }`), this is low-risk redundant coverage rather than a real gap, and matches the plan's own scoping (§3.4.5 only specifies seeding one of the three tables). Not blocking.

### OAuth / remote-storage-provider regression check

Ran `go test ./internal/services/remotestorage/... ./internal/services/...` filtered to `OAuth|Dropbox|GoogleDrive|WebDAV|RemoteStorage|RemoteTarget|BackupRemote` — all pass, 0 failures. Confirmed via `git diff a23fb79a ede4f406 --name-only` that none of the OAuth/remote-storage files (`googledrive.go`, `dropbox.go`, `webdav.go`, `ssrf.go`, `oauthtoken.go`, `backup_remote_service.go`, `backup_remote_handler.go`) were touched by this remediation's diff — consistent with the plan's explicit non-goal of not touching M1/M2/M3/H3/L1-L4 files.

---

## 5. Security Scanning Detail

- **GORM scan:** see gate #1 — clean.
- **govulncheck** (`cd backend && govulncheck ./...`): **0 vulnerabilities** called by Charon's own code (1 vulnerability exists in a required-but-uncalled module — informational only, matches `govulncheck`'s standard "not reachable" classification).
- **Full `golangci-lint` (`.golangci.yml`, all linters — the audit's own note that only the fast subset normally runs)** against `internal/services/...` and `internal/api/handlers/...`: **56 issues total**, identical in substance to the pre-merge audit's own count (56) from before this remediation landed. Cross-checked line-by-line: every finding is either (a) pre-existing and in files this PR's diff does not touch (`crowdsec_handler.go`, `proxy_host_handler.go`, `uptime_service.go`, various pre-existing test files, the 3 `G117` findings in `backup_remote_service.go` which is untouched by this diff), or (b) 2 new `gosec G703` ("path traversal via taint analysis") findings in the brand-new `backup_restore_safe_integrity_test.go` (lines 48, 61) — both on `os.WriteFile` calls using `t.TempDir()`-derived paths inside the new H2 fixture generator. These follow the exact same unannotated pattern as numerous other pre-existing test files in this codebase (e.g. `backup_encryption_test.go:27,32,164`, `crowdsec_archive_test.go`) — none of which carry `#nosec` annotations for this class of test-only finding. Not a new risk pattern, not blocking, consistent with existing convention. No new findings in production code.
- **Trivy container scan** against the freshly rebuilt `charon:local` image (via `docker save` + `trivy image --input`, since the Trivy snap's daemon-socket access is sandboxed in this environment): **0 CRITICAL, 1 HIGH** — `CVE-2026-32286` (`jackc/pgproto3/v2`, bundled in the `crowdsec`/`cscli` binaries). This is the pre-existing, already-tracked finding documented in `SECURITY.md` ("Awaiting Upstream," not exploitable under Charon's default SQLite configuration) — not introduced by this remediation, which added no new dependencies.
- **CodeQL:** **not runnable in this local environment** — no local CodeQL CLI/binary is installed, and `.github/workflows/codeql.yml` is a GitHub-Actions-hosted workflow with no local equivalent invoked by this repo's Makefile/scripts. Not silently skipped — flagged explicitly per the task's instruction. Recommend confirming the GitHub-hosted CodeQL run on the PR itself before merge, as it always has for prior commits on this branch.
- **staticcheck:** covered via `golangci-lint run --config .golangci-fast.yml` (which runs staticcheck as one of its 5 enabled linters) — 0 issues, see gate #6.

---

## 6. Follow-Ups (Non-Blocking)

1. **(Low, cosmetic)** `backup_restore_safe.go`'s new `unrecoverableErr` message formats `rehydrateErr` as `(%v)` even when `s.db == nil` (rehydrate never attempted, not attempted-and-failed), producing a misleading "`(<nil>)`" fragment in that configuration. Not reachable via the current production call site (`routes.go:228` always wires a real `*gorm.DB`). Suggested fix: special-case the message when `rehydrateErr == nil` (e.g. "no live database connection was configured" instead of the raw `%v`).
2. **(Low, coverage completeness)** M5's positive-branch test only seeds `remote_storage_targets`; `dns_provider_credentials` and `tunnel_configs` share the identical code path but are not independently exercised for the `true` case. Low risk given the shared implementation, but would close the loop for full per-table proof.
3. **(Informational)** A full 3-browser (chromium/firefox/webkit) Playwright pass across the complete suite was not run in this session — only firefox, scoped to backup/restore specs. Recommend it as a final pre-merge nicety if CI time budget allows, though nothing in this diff's blast radius suggests browser-specific risk.
4. **(Informational)** CodeQL could not be run locally in this environment; rely on the GitHub-hosted CodeQL check on the PR itself as the authoritative gate before merge.

None of the above block merge — all are follow-ups for future hardening or CI-side confirmation, not defects in the shipped remediation.

---

## 7. Conclusion

C1, H1, H2, M4, and M5 are each correctly fixed and tested per the approved plan's acceptance criteria. The coupled C1+H1 fix was independently re-verified end-to-end (including under `-race`) and found to correctly preserve the "soft failure" semantics for every pre-existing partial-failure path while making the true double-failure case loud, as designed — no cross-contamination between the new hard-failure path and existing soft-failure paths was found. All mandatory coverage, lint, build, and test gates pass. The E2E backup/restore flow was verified end-to-end against a freshly rebuilt image reflecting all 7 commits, with zero failures.

**Verdict: READY TO MERGE.**
