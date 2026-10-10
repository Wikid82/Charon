# QA and Security Report: chore/repeatable-backend-tests

Date: 2026-10-10
Scope: full diff `origin/development...HEAD` (HEAD 5d7b2f73, 5 commits, 35 files, tests/CI/config only) against `SECURITY.md`, `CLAUDE.md` (Definition of Done) and PR 1 of the local batch-3 plan (issue #1548). The branch adds `testutil.MemoryDSN`/`OpenMemoryDB`, moves the CrowdSec and handler tests onto them so `-count=N` and `-shuffle` work in one process, adds a guard test that forbids hand-written shared-cache DSNs in those two packages, and adds a weekly workflow that runs the repeat check.

## Verdict: PASS (no blocking findings)

No critical, high or medium findings. Four low or informational observations (F1-F4) are listed; none blocks merge. No application code changed (only `_test.go` files, `testutil/dsn.go`, `testdb.go` (a test helper), one workflow and `codecov.yml`), no model or migration file is in the diff, and no security-sensitive finding was found.

## Definition of Done evidence

| Check | Command | Result |
|---|---|---|
| Build and vet | `cd backend && go build ./... && go vet ./...` | pass, no output |
| golangci-lint, backend | `golangci-lint run --config .golangci.yml ./...` | 0 issues |
| golangci-lint, agent | `cd agent && golangci-lint run --config ../backend/.golangci.yml ./...` | 0 issues |
| Lefthook | `lefthook run pre-commit --all-files` | all 17 hooks pass, including actionlint, semgrep (367 rules, 1924 files, 0 findings), frontend type-check and lint. Working tree stayed clean (only the three untracked plan files). |
| govulncheck | skill `security-scan-go-vuln` | 0 affected; 1 module-level advisory in a required module that Charon code does not call |
| Trivy | `trivy fs --scanners vuln,misconfig --severity HIGH,CRITICAL` on go.mod/go.sum (backend, agent) and the frontend package files, copied under `$HOME` (the snap trivy cannot read `/projects`) and removed afterwards | no HIGH/CRITICAL |
| CodeQL | `lefthook run codeql` (CLI 2.27.2, already current) | Go and JS scans completed; JS 0 results; findings gate "0 blocking"; "All CodeQL checks passed"; parity check passed |
| GORM scan | not run | no `backend/internal/models/**`, migration or GORM query files in the diff (0 matches) |
| E2E | not run | no application code changed; the E2E image does not need a rebuild |
| Coverage (given, not rerun) | prior runs | backend 91.3% stmt / 90.3% line, frontend 91.49% lines (no frontend change), patch coverage 90.0% (the 2 uncovered lines are in the codecov-ignored `dsn.go`) |
| codecov.yml | `curl -s -m 30 --data-binary @codecov.yml https://codecov.io/validate` | `Valid!`; only `backend/internal/testutil/dsn.go` was added to `ignore` (1 line); `scripts/ci/check-codecov-trigger-parity.sh` passes |

## Repeatability proof (run during this audit)

| Run | Result |
|---|---|
| `go test -race -count=2 -shuffle=on -timeout=30m ./internal/crowdsec/...` | ok, 213 s |
| `go test -race -count=2 -timeout=40m -run 'TestAuthHandler\|TestSettingsHandler\|TestCertificateHandler_Export_PasswordGuard\|TestDeleteCertificate' ./internal/api/handlers/` | ok, 23 s |
| `go test -race -count=2 -run 'TestConsoleEnroll\|TestHeartbeatPoller_\|TestCertificateHandler_Export_PasswordGuard\|TestAuthHandler_GetAccessibleHosts_PermittedHosts' ./internal/crowdsec/ ./internal/api/handlers/` (the tests named on #1548) | ok, ok |
| `go test -count=1 ./internal/testutil/` | ok |
| Guard red/green: temporary `internal/crowdsec/zz_guard_probe_test.go` containing a shared-cache literal | guard failed and named `internal/crowdsec/zz_guard_probe_test.go:3`; probe file removed |
| Full handlers repetition | not rerun (14+ min); recorded in commit 1.3 as green: `-count=2` 829 s, `-count=3` 1268 s, `-shuffle=on -count=2` 840 s |

## Check-by-check results

1. **The DSN sweep did not change test semantics.** The 24 handler test files in the diff change only DSN construction: every non-DSN changed line was filtered out (the only other changes are `fmt` import removals and the `testutil` import additions). No `t.Skip`/`Skipf`/`Skip(` was added, no `func Test*` or `func Benchmark*` was removed, and no assertion changed. `git diff --numstat` shows every handler file is a 1:1 or 2:1 line swap. Cases checked by hand: the previously process-wide `file::memory:?cache=shared` sites (db_health tests, `pr_coverage_test.go`, `permission_helpers_test.go`, `system_permissions_handler_test.go`, `security_handler_settings_test.go` which had a hard-coded test name) now get one unique name per test, which is stored in a variable and reused where the old code shared it (for example `db_health_handler_test.go` passes the same DSN to `database.Connect` and to `NewDBHealthHandler`, so `CheckIntegrityDedicated` still opens the same named shared-cache database). No function calls `MemoryDSN` twice where the old code used one string twice (scripted check over all changed files: none). The unrelated-connection design of the dedicated-corruption test is kept (separate `MemoryDSN` for the unrelated handler connection, an on-disk file for the corrupt one). Tests that once relied on a database leaking from an earlier test would now fail, and none does. The benchmark file now uses `MemoryDSN(b, ...)`, whose per-call counter keeps every benchmark iteration set unique.
2. **`testutil.MemoryDSN` / `OpenMemoryDB`.** Uniqueness comes from a process-wide `atomic.Uint64` counter combined with the sanitized test name, so it holds across `-count`, `-shuffle`, parallel tests and subtests (tests: same-name subtests, two calls in one test). Sanitization replaces every run of non-`[A-Za-z0-9_]` characters (slashes, spaces, unicode, punctuation) with one underscore, so the name is always a valid SQLite file-URI path; a very long name only makes a long in-memory name (no filesystem is touched). Params are appended with `&` after the fixed `?mode=memory&cache=shared`, so the `?`/`&` ordering is right. It takes `testing.TB`, with a benchmark test. `OpenMemoryDB` closes the pool in `t.Cleanup` (tested: ping fails after the subtest). `handlers.OpenTestDB` keeps `_journal_mode=WAL` and `_busy_timeout=5000`, the silent logger and the close-on-cleanup; only the name suffix changed (counter instead of time plus random), so its 231 callers are unaffected. See F2 for a nit.
3. **Guard test.** It finds the module root by walking up from its own source file via `runtime.Caller` to the first `go.mod`, and fails loudly if a guarded directory is missing. It walks guarded packages recursively over `*_test.go` only and reports `path:line`; its own reporting is covered by a unit test and the red/green probe above. The `"cache=" + "shared"` split keeps the marker out of the guard's own file (the guard lives in `testutil`, which is not a guarded package anyway). The `#nosec G304 G122` comment is adequate: the path comes from a walk of a fixed in-repo directory in a test, with no external input. See F3 for the false-negative scope.
4. **Workflow `.github/workflows/backend-repeatability.yml`.** `actionlint` clean. `actions/checkout@3d3c42e5...` (v7) and `actions/setup-go@b7ad1dad...` (v7.0.0) match the SHAs and versions used in `quality-checks.yml` and the rest of the repo. Permissions: `contents: read` only; `persist-credentials: false`; no secrets; triggers are `schedule` and `workflow_dispatch` only (no `pull_request`, so nothing runs from forks). `concurrency` group per workflow and ref with `cancel-in-progress: false`. Cron `0 5 * * 4` (Thursday 05:00 UTC) collides with no other cron in the repo (checked all `cron:` lines). The run step has no interpolated untrusted input: the seed is `GITHUB_RUN_ID % 2147483646 + 1` (always 1..2147483646, so a valid `-shuffle` value), echoed to the log and the step summary so a failure is replayable, and `TMPDIR` is the runner temp directory. Timeouts: `go test -timeout=60m` inside a 75-minute job; the measured work is about 4 min (CrowdSec) plus about 14 min (handlers) with `-count=2` under `-race`, so there is wide headroom. It is not a required check. The plan suggested `-timeout=30m` and `timeout-minutes: 40`; the larger values are sensible given measured times (see F1 for a note on default-branch-only scheduling).
5. **Plan alignment.** Commit order and subjects match plan commits 1.1-1.5. The plan's "fixes for non-DSN shared state" in 1.3 turned out to be unneeded: the DSN sweep alone made every named test repeatable, which the runs above confirm.
6. **Commits.** `git log origin/development..HEAD --format=%s`: `test: add a unique in-memory database helper for backend tests`, `test: make the CrowdSec tests repeatable in one process`, `test: make the handler tests repeatable in one process`, `test: guard against shared database names in the repeatable packages`, `ci: run the backend repeatability check weekly`. All generic, none uses the `(security)` scope, no session ID or link.
7. **Security-sensitive items.** None. Nothing in the diff touches production code paths, secrets, tokens or CI permissions beyond read-only.

## Findings

No critical, high or medium findings.

### Low

- **F1: low (CI behavior).** `.github/workflows/backend-repeatability.yml`: GitHub runs `schedule` triggers only from the default branch, so after merge into `development` the weekly job first exists on `main` after the next promotion, and until then only `workflow_dispatch` on the branch works. Not a defect; mention in the PR body, and run one manual dispatch once the file is on a branch GitHub knows about.

### Informational

- **F2: info (nit).** `backend/internal/testutil/dsn.go` `MemoryDSN`: a test name that sanitizes to an empty string cannot happen (names always start with `Test`/`Benchmark`/`Example`/`Fuzz`), so no change is needed; if the helper is ever used from `TestMain` or an unnamed context, `t.Name()` would be empty and the DSN `file:_N` would still be unique and valid.
- **F3: info (guard scope).** `backend/internal/testutil/dsn_guard_test.go`: the guard only matches the literal `cache=shared` on a single source line, so a DSN assembled across lines or with a different spelling (for example `file::memory:` without `cache=shared`, or the marker built by concatenation like the guard's own constant) is not detected, and only `internal/crowdsec` and `internal/api/handlers` are covered (192 other `cache=shared` test lines exist in other packages that are not yet clean). Acceptable as a cheap tripwire. Optional: add other packages to `dsnGuardPackages` as they are cleaned up.
- **F4: info (follow-up, already planned).** Packages outside the two guarded ones still use name-derived or fixed shared-cache DSNs; extending the repeatability check to them is future work, not part of this PR.

Out-of-scope findings to file: none that need an issue beyond F3/F4 (low priority, optional).
