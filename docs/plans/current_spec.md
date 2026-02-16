## Backend Coverage Alignment + Buffer Plan

Date: 2026-02-16
Owner: Planning Agent
Status: Active (unit tests + coverage only)

## 1) Goal and Scope

Goal:
- Determine why local backend coverage and CI/Codecov differ.
- Add enough backend unit tests to create a reliable buffer above the 85% gate.

In scope:
- Backend unit tests only.
- Coverage measurement/parsing validation.
- CI-parity validation steps.

Out of scope:
- E2E/Playwright.
- Integration tests.
- Runtime/security feature changes unrelated to unit coverage.

## 2) Research Findings (Current Repo)

### 2.1 CI vs local execution path

- CI backend coverage path uses `codecov-upload.yml` → `bash scripts/go-test-coverage.sh` → uploads `backend/coverage.txt` with Codecov flag `backend`.
- Local recommended path (`test-backend-coverage` skill) runs the same script.

### 2.2 Metric-basis mismatch (explicit)

- `scripts/go-test-coverage.sh` computes the percentage from `go tool cover -func`, whose summary line is `total: (statements) XX.X%`.
- `codecov.yml` project status for backend is configured with `only: lines`.
- Therefore:
  - Local script output is statement coverage.
  - CI gate shown by Codecov is line coverage.
  - These are related but not identical metrics and can diverge by about ~0.5-1.5 points in practice.

### 2.3 Likely mismatch drivers in this repo

1. Coverage metric basis:
  - Local script: statements.
  - Codecov status: lines.
2. Exclusion filter differences:
  - Script excludes only `internal/trace` and `integration` packages.
  - Codecov ignore list also excludes extra backend paths/files (for example logger/metrics and Docker-only files).
  - Different include/exclude sets can shift aggregate percentages.
3. Package-set differences in developer workflows:
  - Some local checks use `go test ./... -cover` (package percentages) instead of the CI-equivalent script with filtering and upload behavior.
4. Cache/artifact variance:
  - Go test cache and Codecov carryforward behavior can hide small swings between runs if not cleaned/verified.

## 3) EARS Requirements

- WHEN backend coverage is evaluated locally, THE SYSTEM SHALL report both statement coverage (from `go tool cover`) and CI-relevant line coverage context (Codecov config basis).
- WHEN coverage remediation is implemented, THE SYSTEM SHALL prioritize deterministic unit tests in existing test files before adding new complex harnesses.
- WHEN validation is performed, THE SYSTEM SHALL execute the same backend coverage script used by CI and verify output artifacts (`backend/coverage.txt`, `backend/test-output.txt`).
- IF local statement coverage is within 0-1 points of the threshold, THEN THE SYSTEM SHALL require additional backend unit tests to produce a safety buffer targeting >=86% CI backend line coverage.
- WHEN evaluating pass/fail semantics, THE SYSTEM SHALL distinguish the effective Codecov pass condition from the engineering buffer target: with `target: 85%` and `threshold: 1%` in `codecov.yml`, CI may pass below 85% lines depending on Codecov threshold/rounding logic.

## 4) Highest-Yield Backend Test Targets (+1% with minimal risk)

Priority 1 (low risk, deterministic, existing test suites):

1. `backend/internal/util/permissions.go`
  - Boost tests around:
    - `CheckPathPermissions` error branches (permission denied, non-existent path, non-dir path).
    - `MapSaveErrorCode` and SQLite-path helpers edge mapping.
  - Why high-yield: package currently low (~71% statements) and logic is pure/deterministic.

2. `backend/internal/api/handlers/security_notifications.go`
  - Extend `security_notifications_test.go` for `normalizeEmailRecipients` branch matrix:
    - whitespace-only entries,
    - duplicate/mixed-case addresses,
    - invalid plus valid mixed payloads,
    - empty input handling.
  - Why high-yield: currently very low function coverage (~17.6%) and no external runtime dependency.

Priority 2 (moderate risk, still unit-level):

3. `backend/internal/api/handlers/system_permissions_handler.go`
  - Add focused tests around `logAudit` non-happy paths (missing actor, nil audit service, error/no-op branches).
  - Why: currently low (~25%) and can be exercised with request context + mocks.

4. `backend/internal/api/handlers/backup_handler.go`
  - Add restore-path negative tests in `backup_handler_test.go` using temp files and invalid backup payloads.
  - Why: function `Restore` is partially covered (~27.6%); additional branch coverage is feasible without integration env.

Do not prioritize for this +1% buffer:
- CrowdSec deep workflows requiring process/network/file orchestration for marginal gain-per-effort.
- Docker-dependent paths already treated as CI-excluded in Codecov config.

## 5) Implementation Tasks

### Phase A — Baseline and mismatch confirmation

1. Capture baseline using CI-equivalent script:
  - `bash scripts/go-test-coverage.sh`
2. Record:
  - Script summary line (`total: (statements) ...`).
  - Computed local statement percentage.
  - Existing backend Codecov line percentage from PR check.
3. Document delta (statement vs line) for this branch.

### Phase B — Add targeted backend unit tests

1. Expand tests in:
  - `backend/internal/util/permissions_test.go`
  - `backend/internal/api/handlers/security_notifications_test.go`
2. If additional buffer needed, expand:
  - `backend/internal/api/handlers/system_permissions_handler_test.go`
  - `backend/internal/api/handlers/backup_handler_test.go`
3. Keep changes test-only; no production logic changes unless a test reveals a real bug.

### Phase C — Validate against CI-equivalent flow

1. `bash scripts/go-test-coverage.sh | tee backend/test-output.txt`
2. `go tool cover -func=backend/coverage.txt | tail -n 1` (record statement total).
3. Confirm no backend test failures in output (`FAIL` lines).
4. Push branch and verify Codecov backend status (lines) in PR checks.

## 6) Exact Validation Sequence (mirror CI as closely as possible)

Run from repository root:

1. Environment parity:
  - `export GOTOOLCHAIN=auto`
  - `export CGO_ENABLED=1`
  - CI Go-version parity check (must match `.github/workflows/codecov-upload.yml`):
    ```sh
    CI_GO_VERSION=$(grep -E "^  GO_VERSION:" .github/workflows/codecov-upload.yml | sed -E "s/.*'([^']+)'.*/\1/")
    LOCAL_GO_VERSION=$(go version | awk '{print $3}' | sed 's/^go//')
    if [ "$LOCAL_GO_VERSION" != "$CI_GO_VERSION" ]; then
      echo "Go version mismatch: local=$LOCAL_GO_VERSION ci=$CI_GO_VERSION"
      exit 1
    fi
    ```
2. Clean stale local artifacts:
  - `rm -f backend/coverage.txt backend/test-output.txt`
3. Execute CI-equivalent backend coverage command:
  - `bash scripts/go-test-coverage.sh 2>&1 | tee backend/test-output.txt`
4. Verify script metric type and value:
  - `grep -n 'total:' backend/test-output.txt`
  - `go tool cover -func=backend/coverage.txt | tail -n 1`
5. Verify test pass state explicitly:
  - ```sh
    if grep -qE '^FAIL[[:space:]]' backend/test-output.txt; then
      echo 'unexpected fails'
      exit 1
    fi
    ```
6. CI confirmation:
  - Open PR and check Codecov backend status (lines) for final gate outcome.

## 7) Risks and Mitigations

Risk 1: Statement/line mismatch remains misunderstood.
- Mitigation: Always record both local statement summary and Codecov line status in remediation PR notes.

Risk 2: Added tests increase flakiness.
- Mitigation: prioritize deterministic pure/helper paths first (`internal/util`, parsing/normalization helpers).

Risk 3: Buffer too small for CI variance.
- Mitigation: target >=86.0 backend in CI line metric rather than barely crossing 85.0.

## 8) Acceptance Criteria

1. Investigation completeness:
  - Plan documents likely mismatch causes: metric basis, exclusions, package-set differences, cache/artifact variance.
2. Metric clarity:
  - Plan explicitly states: local script reads Go statement coverage; CI gate evaluates Codecov lines.
3. Test scope:
  - Only backend unit tests are changed.
  - No E2E/integration additions.
4. Coverage result:
  - Backend Codecov project status passes with practical buffer target >=86.0% lines (engineering target).
  - Effective Codecov pass condition is governed by `target: 85%` with `threshold: 1%`, so CI may pass below 85.0% lines depending on Codecov threshold/rounding behavior.
5. Validation parity:
  - CI-equivalent script sequence executed and results captured (`backend/coverage.txt`, `backend/test-output.txt`).
