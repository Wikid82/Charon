> **Note:** the prior report previously at this path (auth cookie secure-flag fix + `feature/backuprestore`
> branch sweep, dated 2026-07-17/18) has been archived to
> `docs/reports/archive/qa_report_backuprestore-cookie-fix_2026-07-17.md`.

# QA Report — Docker Null-Container-List Hotfix

**Date:** 2026-07-30
**Branch:** `fix/docker-empty-list-null-crash` (off `origin/main`)
**Commits audited:** `bf5384d6` (backend), `d09d2437` (frontend)
**Scope:** Standalone two-file hotfix. Not a feature — gates scoped proportionately per Management's
instructions, but nothing mandatory skipped.
**Auditor:** QA & Security Engineer

---

## Executive Summary

**READY TO MERGE. No blocking issues found.**

Both commits match `docs/plans/current_spec.md` exactly — no scope creep. `git diff origin/main...fix/docker-empty-list-null-crash --name-only`
confirms only the four files the spec authorizes were touched:
`backend/internal/services/docker_service.go`, `backend/internal/services/docker_service_test.go`,
`frontend/src/hooks/useDocker.ts`, `frontend/src/hooks/__tests__/useDocker.test.tsx`.

Every mandatory gate passed with real, observed evidence (see below). One informational note (E2E
coverage gap for the exact zero-containers scenario) and one environmental observation (coverage-instrumented
frontend test runs are flaky under concurrent system load, unrelated to this diff) are recorded but are not
blockers.

**Note:** at report time, Management indicated a second, related bugfix commit was being added to this same
branch by another agent. This report certifies the branch **as audited at `bf5384d6`/`d09d2437`** — it does not
cover any commit added after this audit. Re-validation is required before push/PR if the branch changes further.

---

## Gate Results

| # | Gate | Result | Evidence |
|---|------|--------|----------|
| 1 | Backend full test suite | ✅ PASS | `go test ./...` — all packages `ok`, zero failures, including new `TestListContainers_EmptyResultIsNotNil` |
| 1b | Backend coverage | ✅ PASS | `scripts/go-test-coverage.sh` — **89.1% line coverage / 89.2% statement coverage** vs 87% floor |
| 2 | Frontend full test suite | ✅ PASS | `npx vitest run` — 259 test files / 3170 tests passed, 5 files / 88 tests skipped, 2 todo, **zero failures** |
| 2b | Frontend coverage | ✅ PASS | `scripts/frontend-test-coverage.sh` — **Lines 90.59%** (7398/8166), Statements 89.42%, Branches 82.44%, Functions 87.05%, vs 87% floor. Script's own gate check: `Coverage gate: PASS (lines 90.59% vs minimum 87%)` |
| 3 | Type safety | ✅ PASS | `npm run type-check` (`tsc --noEmit`) — zero errors |
| 4 | Backend build | ✅ PASS | `go build ./...` — clean |
| 4b | Frontend build | ✅ PASS | `npm run build` — clean, `dist/` produced |
| 5 | Local patch coverage | ✅ PASS | `bash scripts/local-patch-report.sh` — **Overall 100% (1/1), Backend 100% (1/1), Frontend 100% (0/0)**, all `pass` status vs 90%/85%/85% thresholds. Artifacts confirmed: `test-results/local-patch-report.md`, `test-results/local-patch-report.json` |
| 6 | Lefthook pre-commit | ✅ PASS | `lefthook run pre-commit --file <4 changed files>` (files staged; used `--file` override since files are already committed, not staged) — `go-vet` ✔, `golangci-lint-fast` ✔ (0 issues — this is the staticcheck-inclusive fast lint; `make lint-staticcheck-only` was **not** used, per the known pre-existing golangci-lint v1/v2 Makefile mismatch noted in the task brief), `semgrep` ✔ (0 findings, 0 blocking, scanned exactly the 4 changed files), `frontend-type-check` ✔, `frontend-lint` ✔ (0 errors; 1201 pre-existing project-wide warnings, **none** in `useDocker.ts`/`useDocker.test.tsx` — confirmed via grep) |
| 7 | GORM security scan | ⚪ SKIPPED (judgment call) | `docker_service.go` has zero `gorm`/`.DB` references (`grep -n "gorm\|\.DB\b"` → no matches). Not a `internal/models/**` file, no GORM queries, no migrations. Matches CLAUDE.md's own exclusion ("skip for docs-only or frontend-only changes") in spirit — the trigger condition ("models, GORM queries, or migrations") is not met. Judgment: not required, correctly out of scope. |
| 8 | Trivy container scan | ✅ PASS | E2E image rebuilt fresh (`docker-rebuild-e2e`, includes both commits) via `charon:local`. `trivy image --severity CRITICAL,HIGH` (via saved tarball, see note below) → **`app/charon` binary: 0 findings.** Only finding in the whole image: `CVE-2026-32286` (HIGH) in `pgproto3/v2`, bundled inside `usr/local/bin/crowdsec` / `cscli` — this is a **pre-existing, already-documented** entry in `SECURITY.md` ("Awaiting Upstream", affects only non-default PostgreSQL-backed CrowdSec deployments), unrelated to this diff. Zero CRITICAL findings anywhere. |
| 9 | Targeted Playwright E2E | ✅ PASS | E2E image rebuilt first (container predated both commits — production code changed, so rebuild was mandatory per CLAUDE.md workflow step 1). `npx playwright test tests/orthrus-proxy-paths.spec.ts --project=firefox` → **10/10 passed**. `npx playwright test tests/core/proxy-hosts.spec.ts --project=firefox -g "Docker Integration"` → **3/3 passed**. See coverage-gap note below. |

---

## CodeQL

Not run. Per the task's own troubleshooting note in `CLAUDE.md`, local/sandbox `codeql` binaries are commonly
stale/mismatched against this repo's pinned query packs and this is a documented pre-existing environment
limitation — CI (`.github/workflows/codeql.yml`) is the authoritative CodeQL gate via `github/codeql-action@v4`.
Given this is a 2-file, 6-line-of-production-code hotfix with `semgrep` (which covers a materially overlapping
rule set for Go/TS security issues) already run clean against the exact 4 changed files as part of lefthook, and
Trivy confirming zero new findings in the built binary, CodeQL was judged non-blocking for local sign-off on a
hotfix of this size. Recommend CI's CodeQL run be checked once the PR is opened.

## Trivy — technical note

`trivy` (snap-confined) could not reach `/var/run/docker.sock` directly (`permission denied`) nor read a tarball
written under the session's `/tmp/claude-*` scratch path (snap filesystem confinement). Worked around by
`docker save charon:local -o ~/charon-local.tar` and scanning that tarball with `trivy image --input`. This is an
environment/tooling quirk, not a code issue — no image-scan gate was skipped, just re-routed around a local
sandboxing restriction. Scratch tarball was deleted after the scan.

### Finding 2 — LOW (process) — Agent coverage gate has almost no margin

## Findings

### Blocking
None.

### Informational

1. **E2E coverage gap (accepted, not required to fix):** No existing Playwright spec exercises the exact
   "Docker connected + zero containers → empty array, no crash" scenario. `tests/orthrus-proxy-paths.spec.ts`
   mocks either a populated `MOCK_CONTAINERS` array or an API error; neither exercises an empty-array response.
   Per the task brief and `docs/plans/current_spec.md` §3.3, this hotfix's verification was explicitly scoped to
   unit-test level (backend `TestListContainers_EmptyResultIsNotNil` + frontend `useDocker.test.tsx`'s two new
   cases), and a missing E2E case for this exact scenario was pre-approved as acceptable to flag rather than
   required to fix for a hotfix of this size. Flagging for the record only.

2. **Frontend coverage-instrumented test runs are flaky under concurrent system load — unrelated to this diff.**
   Multiple attempts to run `scripts/frontend-test-coverage.sh` while other CPU/IO-heavy jobs (a parallel `docker
   save`, a parallel Trivy DB download, a parallel duplicate test run) were also executing produced spurious,
   non-reproducible failures each time in a *different*, unrelated file/test (`ProxyHostForm.test.tsx`'s
   "allows manual advanced config input" via jsdom `alert()`; then an unhandled-rejection race in
   `Login.overlay.audit.test.tsx`'s async teardown). Neither failure was reproducible in isolation, and neither
   touches Docker/`useDocker` code paths. Once re-run with no competing jobs, three consecutive clean runs (plain
   `vitest run`, and two `frontend-test-coverage.sh` runs) all passed with zero failures and identical, stable
   coverage numbers. Recorded as an environment/resource-contention observation, not a code defect — CI runners
   are dedicated and should not exhibit this.

### Out of scope — confirmed not touched
- `isDockerConnectivityError` (GitHub issue #1205) — not touched.
- Rootless Docker / subgid host configuration — not touched.
- `ProxyHostForm.tsx` / `ProxyHostForm.test.tsx` — not touched, confirmed via `git diff origin/main...HEAD --name-only`.
- `feat/changelog` branch work — unrelated, not touched.

---

## Diff Sanity Check

```
$ git diff origin/main...fix/docker-empty-list-null-crash --name-only
backend/internal/services/docker_service.go
backend/internal/services/docker_service_test.go
frontend/src/hooks/__tests__/useDocker.test.tsx
frontend/src/hooks/useDocker.ts
```

Backend production change (`bf5384d6`):
```go
- var result []DockerContainer
+ result := make([]DockerContainer, 0)
```

Frontend production change (`d09d2437`):
```ts
  return {
-   containers,
+   containers: containers ?? [],
    isLoading,
```

Both exactly match spec §2.1/§2.2. No unrelated changes present.

---

## Recommendation

**Ship it** — as audited at `bf5384d6`/`d09d2437`. Do not push/open the PR until Management confirms the branch's
final state (a second bugfix commit was reported as being added mid-audit); this report does not cover any commit
beyond `d09d2437` and a fresh validation pass is required if the branch changes.
