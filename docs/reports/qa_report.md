# QA and Security Report: certificate private key export re-authentication (GH #1390)

Branch: `fix/cert-export-private-key-403-1390` (5 commits over `origin/development`)
Date: 2026-09-29

## Verdict: PASS after fixes (the 2 hygiene blockers found in the first audit pass, B1 and B2, are resolved in commits 218df2dc and 9f42400f; no security defects found in the change)

## Gate results

| # | Gate | Result | Detail |
|---|------|--------|--------|
| 1 | `go build ./...` + `go test ./...` | PASS | Build clean. Full `go test ./...` reported no failures (non-race). |
| 1b | Same package under `-race` (as the coverage script runs it) | FAIL, pre-existing flake | `TestCredentialHandler_Update_ByCredentialUUID` failed in the full `-race` handlers run (twice). See B2. |
| 2 | `scan-gorm-security.sh --check` | PASS | 0 critical, 0 high, 0 medium, 2 info, 1 suppressed. |
| 3a | `make lint-fast` | PASS | backend 0 issues, agent 0 issues. |
| 3b | `make lint-backend` (full golangci-lint) | FAIL | 1 issue introduced by this change. See B1. |
| 4 | `local-patch-report.sh` | PASS | Backend patch coverage 39/39 changed lines = 100.0% (overall 100.0%). Artifacts present: `test-results/local-patch-report.md` and `.json`. |
| 5 | `scripts/go-test-coverage.sh` | Coverage PASS, run FAIL | Statement coverage 92.2%, line coverage 88.9%, gate 87%: met. Script's `go test` exited non-zero solely due to the flake in B2. |
| 6 | `lefthook run pre-commit --all-files` | PASS | All 16 hooks passed, including semgrep (367 rules, 0 findings), golangci-lint-fast, go-vet, actionlint, shellcheck, frontend lint and type-check. (Plain `lefthook run pre-commit` skips everything because nothing is staged.) |
| 7 | E2E `tests/certificate-export.spec.ts`, firefox only | PASS | 15/15 passed (45.6s) against an E2E container rebuilt from this branch. |
| 8 | CodeQL Go (local CLI 2.26.4, working) | PASS | 4 results, all in files outside this diff (`auth_handler.go`, `remote_server_handler.go` x2, `uptime_service.go`), all covered by documented entries in `codeql-suppressions.yml`. 0 blocking. JS scan 0 results. |
| 9 | Trivy (`security-scan-trivy` skill; there is no `make trivy` target) | Not attributable to this change | The filesystem scan reports only items under `.claude/worktrees/*` and the git-ignored `docs-site/build/` output (a docs example service-account snippet). No finding in tracked backend, frontend or agent manifests touched by this branch. The skill exits non-zero because of these stale local directories. CI runs the authoritative scan. |

## Blocking issues

### B1. RESOLVED: golangci-lint (`make lint-backend`) failure introduced by this change
`backend/internal/api/handlers/certificate_handler_export_auth_test.go:67`: `gocritic unnamedResult: consider giving a name to these results` on
`func (e *exportHarness) createUser(role models.UserRole) (*models.User, string)`.
It is the only issue in the full run (250 raw, 1 after filtering). `lint-backend` is blocking per CLAUDE.md. Fix: name the results, for example `(user *models.User, token string)`. This is a one-line test-only change.

### B2. RESOLVED: Flaky test: `TestCredentialHandler_Update_ByCredentialUUID` (pre-existing, not caused by this change)
- Symptom: PUT returns 500 instead of 200. Log line: `credential_service.go:354 database table is locked` (SQLite shared-cache table lock).
- Frequency: failed in the full `go test -race -v ./internal/api/handlers` run (twice). In isolation with `-race -run TestCredentialHandler_ -count=100` it failed 3 to 4 times in each 100-run sample.
- Proof it is not from this diff: the same 100-run `-race` reproduction on a clean `git archive` of `origin/development` failed 4 times in 100. The test, `credential_handler_test.go`, and `credential_service.go` are not in this branch's diff. Plain `-count=10` without `-race` passed.
- Cause: `setupCredentialHandlerTest` opens `file:<TestName>?mode=memory&cache=shared&_journal_mode=WAL`. Shared-cache mode raises `SQLITE_LOCKED` (table locked) when two connections touch the same table, and no busy timeout helps. Under the race detector the timing exposes it.
- Per CLAUDE.md a failing test must not be deferred. Recommended fix (backend-dev): give the test DB a single connection (`sqlDB.SetMaxOpenConns(1)`) or drop `cache=shared` for a temp-file or per-connection DB. This needs a separate `test:` or `fix:` commit. `TestPerf_GetStatus_AssertThreshold` did not fail in any run.

## Security review of `git diff origin/development...HEAD`

Verdict: no defects found. The change restores the re-authentication safeguard correctly.

- Ordering in `reauthenticateForKeyExport` (only runs when `include_key` is true): admin role check, then user ID from the session, then emergency-bypass rejection, then DB availability, then empty-password rejection, then sign-in budget charge, then user lookup, then password check. The route also has `RequireRole(models.RoleAdmin)`, so admin is enforced twice.
- Emergency bypass: a session with `userID == 0` and `IsEmergencyBypass` true (strict boolean check) gets 403 for key export. Certificate-only export (`include_key=false`) stays available. Route-level test `EmergencyBypassRejected` covers both.
- Rate limiting: empty password returns 403 before charging the budget, so a missing-password request cannot burn budget (tested with a 1-attempt budget). Every real password check is charged before verification, whether or not it turns out correct. A nil guard allows, matching the existing helper contract. The `password_guard_inventory_test` allowlist now points at `CertificateHandler.reauthenticateForKeyExport` and still requires the guard call.
- Logging and error leaks: the password is never logged. Server-side errors are wrapped with `%w` and logged without request data. Client responses are generic (`incorrect password`, `user not found`, `internal error`, `password required to export private key`). The lookup now uses `errors.Is(gorm.ErrRecordNotFound)`, so a DB failure returns 500 instead of a misleading 403. The `user not found` response for a deleted account is not an enumeration risk because the caller already holds a valid admin session.
- User ID parsing: now via the shared `requireUserID` helper instead of the old untyped `map[string]any` lookup (`h.db.First(&user, "id = ?", userID)`), which removes the earlier type-assertion failure path.
- Other private-key release paths:
  - `CertificateHandler.Get` and `List` return only `HasKey`, never key material.
  - `CertificateService.GetDecryptedPrivateKey` is called only from `ExportCertificate`, and `ExportCertificate` has a single caller (the gated handler).
  - Charon-generated exports are the only route that decrypts stored keys; no other handler references it.

## Non-blocking notes
- Coverage headroom is comfortable: 92.2% statements, 88.9% lines against the 87% gate.
- CodeQL suppression entries in `codeql-suppressions.yml` have review dates of 2026-11-04 and 2026-11-27; none relate to this change.

## Working tree
`git status --short` is empty on branch `fix/cert-export-private-key-403-1390`, apart from this report. Generated artifacts (`test-results/local-patch-report.*`, `backend/coverage.txt`, `codeql-results-*.sarif`, `playwright/.auth/`) are git-ignored, so there are no stray tracked or untracked files. Nothing was committed, no branches were switched, and no application code was modified.

## Re-verification after fixes
- B1: results named in the test helper (218df2dc). `make lint-backend` and `make lint-fast`: 0 issues.
- B2: root cause was the audit-writer goroutine and the handler writing on two pooled connections to a shared-cache in-memory SQLite database (`SQLITE_LOCKED` ignores `busy_timeout`). The test helpers now use a single connection (9f42400f). Before: 3 failures in 100 `-race` runs. After: 0 in 300, and 0 in 50 on re-verification. The full `-race` handlers package run passed.
- `go build ./...` and `go test ./internal/api/handlers/... ./internal/api/middleware/... ./internal/api/routes/...`: pass.
