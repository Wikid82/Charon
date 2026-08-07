# QA Report — SQLite Test-Flake Fix + Uptime Monitor Create Retry Hardening

**Branch**: `development`
**Commits reviewed**: `3c81849d`, `96ee480a`, `f1bcb3a4` (already landed on `development`, supervisor-approved)
**Reviewed by**: qa-security agent
**Date**: 2026-08-07
**Scope**: Full backend Definition of Done per `CLAUDE.md`, independent re-verification of backend-dev/supervisor's reported numbers (not re-trusting prior self-reports)
**Plan reference**: `docs/plans/current_spec.md`

## Final Verdict: **PASS**

All required backend DoD gates pass with independently reproduced evidence. No blocking findings. One process finding (non-blocking, but worth institutionalizing) about `scripts/local-patch-report.sh`'s default baseline resolution on this branch — see §3.

---

## 0. Scope confirmation

```
git diff --stat 3c81849d~1..f1bcb3a4
 backend/internal/api/handlers/proxy_host_handler_test.go | 121 +++++++++++++++------
 backend/internal/services/uptime_service.go               |  34 +++++-
 backend/internal/services/uptime_service_race_test.go     |  88 ++++++++++++++
```

Exactly the 3 files the plan and task description claim. No `go.mod`/`go.sum`/`Dockerfile`/`backend/internal/models/**` changes anywhere in the range (confirmed via scoped `git diff --stat`). No frontend files touched. This independently confirms the plan's §8/§9 N/A claims for GORM scan, Trivy, and frontend type-check before accepting them.

---

## 1. Playwright E2E — N/A, confirmed correct

Plan §9 argues N/A. Verified rather than trusted:
- `grep -rliE "proxy.?host" tests/**/*.spec.ts` → found `tests/core/proxy-hosts.spec.ts` (the one spec that plausibly exercises proxy-host creation).
- `grep -n "uptime" -i tests/core/proxy-hosts.spec.ts` → **zero matches**. This spec never asserts on uptime-monitor creation/behavior, so it cannot observe the internal retry-loop change in `uptime_service.go`, and it doesn't touch the test-only DSN fix at all (Go test files aren't part of the built app).
- No frontend files changed in any of the 3 commits.
- Commit 3's change (`createMonitorWithRetry`) is a same-behavior-from-outside internal hardening — bounded retry on an already-fire-and-forget, already-swallowed-error background path. No new API contract, no new response shape.

**Conclusion**: N/A is correct. Running the full suite would add no signal for this diff. Not run.

---

## 2. GORM Security Scan — N/A, confirmed correct

`git diff --stat 3c81849d~1..f1bcb3a4 -- 'backend/internal/models/**'` → empty. No model files, no GORM queries changed outside `uptime_service.go`'s existing `s.DB.Create` call site (unchanged query shape, only wrapped in a retry loop). Condition for triggering the scan is not met. Skipped per CLAUDE.md's conditional trigger.

---

## 3. Local Patch Coverage Preflight

**Command**: `bash scripts/local-patch-report.sh`

### 3a. Reproducing and diagnosing supervisor's `status: input_missing`

Confirmed real on first run in this session — `frontend/coverage/lcov.info` did not exist (only `backend/coverage.txt` and `agent/coverage.txt` were present from earlier sessions). This is **not** a script defect and not something that can be papered over: `scripts/local-patch-report.sh` unconditionally requires all three coverage inputs (backend, frontend, agent) to exist before it will run the Go patch-report tool, regardless of which areas a given PR actually touches. For a backend-only PR, the *legitimate* way to produce real numbers is to run the frontend and agent coverage generators once (their own patch coverage will be ~0% since the diff doesn't touch those trees, but the tool needs the *files* present to compute per-scope patch coverage correctly across the whole report).

**Fix applied**: ran `bash scripts/frontend-test-coverage.sh` (needed `fnm install v24.19.0` first — the pinned `.nvmrc` version wasn't installed in this sandbox) to generate `frontend/coverage/lcov.info`, and re-ran `scripts/go-test-coverage.sh` to refresh `backend/coverage.txt`. `agent/coverage.txt` was left as-is (stale from Jul 21, but the agent tree isn't touched by this PR so it's an acceptable input for the tool's presence check).

After that, `local-patch-report.sh` ran successfully in `strict` mode and produced both artifacts.

### 3b. Second finding: default baseline scope leak on this branch

The first successful run resolved baseline to `origin/main...HEAD` (via `gh pr view --json baseRefName`, which found a standing PR from `development` targeting `main` — almost certainly the recurring weekly promotion PR referenced in `CLAUDE.md`). That pulled in **363 changed lines across the entire accumulated `development` vs `main` diff**, including 3 files this PR never touches (`frontend/src/context/AuthContext.tsx`, `backend/internal/api/routes/routes.go`, `backend/internal/api/handlers/system_permissions_handler.go` — from unrelated prior work already merged to `development`). That is not useful signal for reviewing *these 3 commits specifically*.

**Corrected**: re-ran with `CHARON_PATCH_BASELINE="3c81849d~1...f1bcb3a4"` to scope the diff to exactly the reviewed commit range.

### 3c. Final, scoped result (authoritative for this PR)

| Scope | Changed Lines | Covered Lines | Patch Coverage | Status |
|---|---:|---:|---:|---|
| Overall | 16 | 15 | 93.8% | pass |
| Backend | 16 | 15 | 93.8% | pass |
| Frontend | 0 | 0 | 100.0% | pass |
| Agent | 0 | 0 | 100.0% | pass |

One uncovered changed line: `backend/internal/services/uptime_service.go:1417` — the final `return lastErr` after `createMonitorWithRetry`'s loop. Inspected: this is structurally unreachable in practice (the loop already returns from inside on `attempt == maxAttempts`), a compiler-mandated trailing return, not a real gap. Not a concern.

**Artifacts**: `test-results/local-patch-report.md`, `test-results/local-patch-report.json` — both present, non-empty, verified.

**Recommendation for future QA passes on this branch**: when `development` has a standing promotion PR open against `main`, set `CHARON_PATCH_BASELINE` explicitly to the reviewed commit range (or the PR's actual base) rather than trusting the script's auto-resolution — otherwise patch-coverage numbers silently include unrelated accumulated diff. Non-blocking for this sign-off since the scoped numbers clearly pass, but worth a short note in the script's own docs/README so this doesn't need re-discovering each time.

---

## 4. Security Scans

### 4a. CodeQL (Go + JS)

Note: `CLAUDE.md` documents this as `lefthook run pre-commit`, but this repo's actual `lefthook.yml` defines CodeQL as a **separate manual pipeline** (`lefthook run codeql`, sequential go→js→findings→parity) — it is not part of the default `pre-commit` hook set. This is a doc/config drift, not something introduced by this PR; ran the actual mechanism (`lefthook run codeql`) since that's what performs the real scan.

**Command**: `lefthook run codeql` (full DB build + `go-security-and-quality`/JS equivalent suites, ~197s total)

**Result**:
- Go scan: 1 total finding, `go/cookie-secure-not-set` at `backend/internal/api/handlers/auth_handler.go:198` — **SUPPRESSED** (pre-existing, dated, documented suppression unrelated to any file touched by this PR; review-by 2026-11-04, tracked in `docs/issues/codeql-cookie-suppression-not-honored.md`). 0 blocking.
- JS scan: 0 findings.
- Extraction-parity check: passed (no drift between `go list` baseline and CodeQL-extracted file count).
- `codeql-check-findings.sh`: **All CodeQL checks passed**.

**Zero unresolved high/critical findings.** Gate passes.

### 4b. Semgrep (targeted at the 3 changed files)

**Command**: `bash scripts/pre-commit-hooks/semgrep-scan.sh backend/internal/api/handlers/proxy_host_handler_test.go backend/internal/services/uptime_service.go backend/internal/services/uptime_service_race_test.go`

**Result**: 120 rules run (p/golang + community), **0 findings**.

### 4c. Trivy — N/A, confirmed correct

`git diff --stat 3c81849d~1..f1bcb3a4 -- go.mod go.sum backend/go.mod backend/go.sum Dockerfile backend/Dockerfile frontend/Dockerfile .dockerignore` → empty. No dependency manifest or Docker build input changed anywhere in the 3-commit range. Re-running Trivy would reproduce the last `development` scan byte-for-byte with no new signal. N/A confirmed, not run.

### 4d. Gotify token review

No notification/token-handling code touched by this PR (scope is SQLite test setup + an internal DB-write retry loop). No log lines, test artifacts, or API examples from this session contain Gotify tokens. N/A.

---

## 5. Lefthook full pre-commit triage

**Command**: `lefthook run pre-commit`

Ran against a clean working tree (the 3 commits are already committed, nothing staged) — every hook reported `(skip) no matching staged files`/`no files for inspection`. This is expected lefthook behavior for already-committed changes and is **not a real pass/fail signal** on its own for a post-hoc QA review; it only gates at commit time. To get real coverage of what those hooks would have caught, ran the equivalent checks manually against the changed files/whole trees:

- `go vet ./...` (backend) — clean.
- `go vet ./...` (agent) — clean.
- `make lint-fast` (staticcheck + govet + errcheck + ineffassign + unused, backend + agent) — see §6.
- Semgrep — see §4b.
- `frontend/tsc --noEmit` — see §7.

No frontend/shell files in scope for `frontend-lint`/`shellcheck`; not applicable.

---

## 6. Staticcheck (BLOCKING)

**Command**: `make lint-fast`

**Result**:
```
Running fast linters (staticcheck, govet, errcheck, ineffassign, unused) — backend + agent...
cd backend && golangci-lint run --config ../.golangci-fast.yml ./...
0 issues.
cd agent && golangci-lint run --config ../.golangci-fast.yml ./...
0 issues.
```

**Zero errors.** Gate passes.

---

## 7. Type Safety (frontend) — N/A, confirmed

No frontend files touched in any of the 3 commits (confirmed in §0). Ran `cd frontend && npm run type-check` anyway as a sanity check — `tsc --noEmit` completed with no errors. N/A confirmed, and the repo's current frontend type state is clean regardless.

---

## 8. Coverage

### 8a. Backend

**Command**: `scripts/go-test-coverage.sh` (full `go test -race -v -coverprofile=...` across `./...`, ~9 min)

**Result**:
```
total: (statements) 89.3%
Statement coverage: 89.3%
Line coverage: 89.2%
Coverage gate (line coverage): minimum required 87%
Coverage requirement met
```

No `FAIL`, no `DATA RACE` anywhere in the full run log. **Independently reproduces backend-dev's reported 89.3% and supervisor's reported 89.2% exactly** — third independent confirmation.

### 8b. Frontend (for completeness / patch-report input, not required by this PR's scope)

`89.48%` statements / `90.68%` lines — well above the 87% floor. Not required for a backend-only PR but generated as a prerequisite for §3 and confirms no regression in frontend coverage baseline.

---

## 9. Build

```
cd backend && go build ./...
```

Clean, no output, exit 0.

---

## 10. New/changed tests — targeted `-race` runs

### 10a. `TestProxyHostCreate_TriggersAsyncUptimeSyncWhenServiceConfigured`, `..._ConcurrentLoad`, `TestUpdate_ExistingHostsBackwardCompatibility`

**Command**: `go test ./internal/api/handlers/... -run '<the 3 tests>' -race -count=10 -v`

**Result**: 30/30 `PASS` (10 iterations × 3 tests), 0 `FAIL`, 0 `DATA RACE`. `ok ... 6.993s` / `7.356s` across two runs.

### 10b. `TestSyncAndCheckForHost_RetriesOnTransientLockError`

**Command**: `go test ./internal/services/... -run 'TestSyncAndCheckForHost_RetriesOnTransientLockError' -race -count=10 -v`

**Result**: 10/10 `PASS`, 0 `FAIL`, 0 `DATA RACE`.

**Independently verified genuine lock contention** (not just a green checkmark trusted from prior reports): grepped the verbose output for the literal string `database table is locked` → **70 occurrences** across the 10 runs, appearing at `uptime_service.go:1403` (inside `createMonitorWithRetry`'s loop, multiple attempts per run before eventual success) and also at `:1312`/`:377` (other queries hitting the same induced lock window). This confirms backend-dev's and supervisor's claim of "real lock contention, not a mocked/trivial pass" — verified firsthand in this session's own log output, not taken on trust.

---

## 11. Full package regression

| Package | Command | Result |
|---|---|---|
| `internal/api/handlers` | `go test ./internal/api/handlers/... -count=1` | `ok ... 80.011s`, 0 FAIL |
| `internal/services` | `go test ./internal/services/... -count=1` | `ok ... 113.948s` (+ `ok .../remotestorage 31.842s`), 0 FAIL |

No regressions in either package.

---

## 12. Clean-up check

```
git show <sha> | grep -nE "^\+.*(fmt\.Println|console\.log|debugger;|TODO|FIXME|XXX)"
```
run against all 3 commits individually → no matches in any of them. `git status --short` shows only `docs/plans/current_spec.md` modified (the plan document itself, already updated in-place to reflect "Implemented and supervisor-approved" status prior to this QA pass — not a stray file, not part of the 3 commits under review, not touched by this QA session). No debug prints, no commented-out blocks, no stray files introduced by any of the 3 commits.

---

## Summary

| Gate | Result |
|---|---|
| Playwright E2E | N/A (confirmed) |
| GORM Security Scan | N/A (confirmed) |
| Local Patch Coverage | **PASS** (93.8% patch coverage, scoped correctly — see §3 for a process note) |
| CodeQL Go + JS | **PASS** (0 blocking; 1 pre-existing suppressed, unrelated) |
| Semgrep | **PASS** (0 findings) |
| Trivy | N/A (confirmed) |
| Lefthook pre-commit (manual equivalent) | **PASS** |
| Staticcheck / `make lint-fast` | **PASS** (0 issues) |
| Frontend type safety | N/A (confirmed); `tsc --noEmit` clean anyway |
| Backend coverage | **PASS** (89.2% line / 89.3% statement, ≥85% required — 3rd independent confirmation) |
| Build | **PASS** |
| New/changed tests (`-race -count=10`) | **PASS** (40/40, 0 races, genuine lock contention verified in logs) |
| Full package regression | **PASS** (both packages, 0 FAIL) |
| Clean-up | **PASS** |

**Overall verdict: PASS.** Nothing blocks sign-off. The one non-blocking process finding (§3b — `local-patch-report.sh`'s default baseline can silently widen scope to `origin/main...HEAD` on branches with a standing promotion PR) does not affect this PR's own numbers, which pass comfortably once scoped correctly, but is worth a documentation fix so future QA passes don't have to rediscover it.
