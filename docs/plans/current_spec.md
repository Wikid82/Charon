## Local Pre-CI Patch Report (Single Scope)

Date: 2026-02-17
Scope: Add a local pre-CI patch report to Definition of Done (DoD) unit-testing flow for both backend and frontend.

## 1) Objective

Add one executable local workflow that computes patch coverage from current branch changes and publishes a consolidated report before CI runs.

The report must consume backend and frontend coverage inputs, use `origin/main...HEAD` as the patch baseline, and produce human-readable and machine-readable artifacts in `test-results/`.

## 2) In Scope / Out of Scope

### In Scope

- Local patch report generation.
- Backend + frontend DoD unit-testing integration.
- VS Code task wiring for repeatable local execution.
- Non-blocking warning policy for initial rollout.

### Out of Scope

- CI gate changes.
- Encryption-key or unrelated reliability/security remediation.
- Historical Codecov placeholder gates and unrelated patch-closure matrices.

## 3) Required Inputs and Baseline

### Coverage Inputs

- Backend coverage profile: `backend/coverage.txt`
- Frontend coverage profile: `frontend/coverage/lcov.info`

### Diff Baseline

- Git diff range: `origin/main...HEAD`

### Preconditions

- `origin/main` is fetchable locally.
- Backend and frontend coverage artifacts exist before report generation.

## 4) Required Output Artifacts

- Markdown report: `test-results/local-patch-report.md`
- JSON report: `test-results/local-patch-report.json`

Both artifacts are mandatory per run. Missing either artifact is a failed local report run.

## 5) Initial Policy (Rollout)

### Initial Policy (Non-Blocking)

- Local patch report does not fail DoD on low patch coverage during initial rollout.
- Local runner emits warnings (stdout + markdown/json status fields) when thresholds are not met.
- DoD requires the report to run and artifacts to exist, even in warning mode.
- Execution and final merge checks in this plan follow this same warn-mode policy during rollout.

### Threshold Defaults and Source Precedence

- Coverage thresholds are resolved with this precedence:
   1. Environment variables (highest precedence)
   2. Built-in defaults (fallback)
- Threshold environment variables:
   - `CHARON_OVERALL_PATCH_COVERAGE_MIN`
   - `CHARON_BACKEND_PATCH_COVERAGE_MIN`
   - `CHARON_FRONTEND_PATCH_COVERAGE_MIN`
- Built-in defaults for this rollout:
   - Overall patch coverage minimum: `90`
   - Backend patch coverage minimum: `85`
   - Frontend patch coverage minimum: `85`
- Parsing/validation:
   - Values must be numeric percentages in `[0, 100]`.
   - Invalid env values are ignored with a warning, and the corresponding default is used.

### Future Policy (Optional Hard Gate)

- Optional future switch to hard gate (non-zero exit on threshold breach).
- Gate behavior is controlled by a dedicated flag/env (to be added during implementation).
- Hard-gate enablement is explicitly deferred and not part of this rollout.

## 6) Technical Specification

### 6.1 Script

Implement a new local report script:

- Path: `scripts/local-patch-report.sh`
- Responsibilities:
  1. Validate required inputs exist (`backend/coverage.txt`, `frontend/coverage/lcov.info`).
  2. Resolve patch files/lines from `origin/main...HEAD`.
  3. Correlate changed lines with backend/frontend coverage data.
  4. Compute patch summary by component and overall.
   5. Resolve thresholds using env-var-first precedence, then defaults (`90/85/85`).
   6. Evaluate statuses against resolved thresholds:
       - `overall.status=pass` when `overall.patch_coverage_pct >= overall_threshold`, else `warn`.
       - `backend.status=pass` when `backend.patch_coverage_pct >= backend_threshold`, else `warn`.
       - `frontend.status=pass` when `frontend.patch_coverage_pct >= frontend_threshold`, else `warn`.
   7. Emit warning status when any scope is below its resolved threshold.
   8. Write required outputs:
     - `test-results/local-patch-report.md`
     - `test-results/local-patch-report.json`

### 6.2 Report Contract

Minimum JSON fields:

- `baseline`: `origin/main...HEAD`
- `generated_at`
- `mode`: `warn` (initial rollout)
- `thresholds`:
   - `overall_patch_coverage_min`
   - `backend_patch_coverage_min`
   - `frontend_patch_coverage_min`
- `threshold_sources`:
   - `overall` (`env` | `default`)
   - `backend` (`env` | `default`)
   - `frontend` (`env` | `default`)
- `overall`:
  - `changed_lines`
  - `covered_lines`
  - `patch_coverage_pct`
  - `status` (`pass` | `warn`)
- `backend` and `frontend` objects with same coverage counters and status
- `files_needing_coverage` (required array for execution baselines), where each item includes at minimum:
   - `path`
   - `uncovered_changed_lines`
   - `patch_coverage_pct`
- `artifacts` with emitted file paths

Minimum Markdown sections:

- Run metadata (timestamp, baseline)
- Input paths used
- Resolved thresholds and their sources (env/default)
- Coverage summary table (overall/backend/frontend)
- Warning section (if any)
- Artifact paths

### 6.3 Task Wiring

Add VS Code task entries in `.vscode/tasks.json`:

1. `Test: Local Patch Report`
   - Runs report generation script only.
2. `Test: Backend DoD + Local Patch Report`
   - Runs backend unit test coverage flow, then local patch report.
3. `Test: Frontend DoD + Local Patch Report`
   - Runs frontend unit test coverage flow, then local patch report.
4. `Test: Full DoD Unit + Local Patch Report`
   - Runs backend + frontend unit coverage flows, then local patch report.

Task behavior:

- Reuse existing coverage scripts/tasks where available.
- Keep command order deterministic: coverage generation first, patch report second.

## 7) Implementation Tasks

### Phase 1 — Script Foundation

- [ ] Create `scripts/local-patch-report.sh`.
- [ ] Add input validation + clear error messages.
- [ ] Add diff parsing for `origin/main...HEAD`.

### Phase 2 — Coverage Correlation

- [ ] Parse backend `coverage.txt` and map covered lines.
- [ ] Parse frontend `coverage/lcov.info` and map covered lines.
- [ ] Compute per-scope and overall patch coverage counters.

### Phase 3 — Artifact Emission

- [ ] Generate `test-results/local-patch-report.json` with required schema.
- [ ] Generate `test-results/local-patch-report.md` with summary + warnings.
- [ ] Ensure `test-results/` creation if missing.

### Phase 4 — Task Wiring

- [ ] Add `Test: Local Patch Report` to `.vscode/tasks.json`.
- [ ] Add backend/frontend/full DoD task variants with report execution.
- [ ] Verify tasks run successfully from workspace root.

### Phase 5 — Documentation Alignment

- [ ] Update DoD references in applicable docs/instructions only where this local report is now required.
- [ ] Remove stale references to unrelated placeholder gates in active plan context.

## 8) Validation Commands

Run from repository root unless noted.

1. Generate backend coverage input:

```bash
cd backend && go test ./... -coverprofile=coverage.txt
```

2. Generate frontend coverage input:

```bash
cd frontend && npm run test:coverage
```

3. Generate local patch report directly:

```bash
./scripts/local-patch-report.sh
```

4. Generate local patch report via task:

```bash
# VS Code task: Test: Local Patch Report
```

5. Validate artifacts exist:

```bash
test -f test-results/local-patch-report.md
test -f test-results/local-patch-report.json
```

6. Validate baseline recorded in JSON:

```bash
jq -r '.baseline' test-results/local-patch-report.json
# expected: origin/main...HEAD
```

## 9) Acceptance Criteria

- [ ] Plan remains single-scope: local pre-CI patch report for DoD unit testing only.
- [ ] Inputs are explicit and used:
  - [ ] `backend/coverage.txt`
  - [ ] `frontend/coverage/lcov.info`
  - [ ] `origin/main...HEAD`
- [ ] Outputs are generated on every successful run:
  - [ ] `test-results/local-patch-report.md`
  - [ ] `test-results/local-patch-report.json`
- [ ] Initial policy is non-blocking warning mode.
- [ ] Default thresholds are explicit:
   - [ ] Overall patch coverage: `90`
   - [ ] Backend patch coverage: `85`
   - [ ] Frontend patch coverage: `85`
- [ ] Threshold source precedence is explicit: env vars first, then defaults.
- [ ] Future hard-gate mode is documented as optional and deferred.
- [ ] Concrete script + task wiring tasks are present and executable.
- [ ] Validation commands are present and reproducible.
- [ ] Stale unrelated placeholder gates are removed from this active spec.

## 10) Concrete Execution Plan — Patch Gap Closure (PR Merge Objective)

Single-scope objective: close current patch gaps for this PR merge by adding targeted tests and iterating local patch reports until changed-line coverage is merge-ready under DoD.

### Authoritative Gap Baseline (2026-02-17)

Use this list as the only planning baseline for this execution cycle:

- `backend/cmd/localpatchreport/main.go`: 0%, 200 uncovered changed lines, ranges `46-59`, `61-73`, `75-79`, `81-85`, `87-96`, `98-123`, `125-156`, `158-165`, `167-172`, `175-179`, `182-187`, `190-198`, `201-207`, `210-219`, `222-254`, `257-264`, `267-269`
- `frontend/src/pages/UsersPage.tsx`: 30.8%, 9 uncovered (`152-160`)
- `frontend/src/pages/CrowdSecConfig.tsx`: 36.8%, 12 uncovered (`975-977`, `1220`, `1248-1249`, `1281-1282`, `1316`, `1324-1325`, `1335`)
- `frontend/src/pages/DNSProviders.tsx`: 70.6%, 10 uncovered
- `frontend/src/pages/AuditLogs.tsx`: 75.0%, 1 uncovered
- `frontend/src/components/ProxyHostForm.tsx`: 75.5%, 12 uncovered
- `backend/internal/api/middleware/auth.go`: 86.4%, 3 uncovered
- `frontend/src/pages/Notifications.tsx`: 88.9%, 3 uncovered
- `backend/internal/cerberus/rate_limit.go`: 91.9%, 12 uncovered

### DoD Entry Gate (Mandatory Before Phase 1)

All execution phases are blocked until this gate is completed in order:

1) E2E first:

```bash
cd /projects/Charon && npx playwright test --project=firefox
```

2) Local patch preflight (baseline refresh trigger):

```bash
cd /projects/Charon && bash scripts/local-patch-report.sh
```

3) Baseline refresh checkpoint (must pass before phase execution):

```bash
cd /projects/Charon && jq -r '.files_needing_coverage[].path' test-results/local-patch-report.json | sort > /tmp/charon-baseline-files.txt
cd /projects/Charon && while read -r f; do git diff --name-only origin/main...HEAD -- "$f" | grep -qx "$f" || echo "baseline file missing from current diff: $f"; done < /tmp/charon-baseline-files.txt
```

4) If checkpoint output is non-empty, refresh this baseline list to match the latest `test-results/local-patch-report.json` before starting Phase 1.

### Ordered Phases (Highest Impact First)

#### Phase 1 — Backend Local Patch Report CLI (Highest Delta)

Targets:
- `backend/cmd/localpatchreport/main.go` (all listed uncovered ranges)

Suggested test file:
- `backend/cmd/localpatchreport/main_test.go`

Test focus:
- argument parsing and mode selection
- coverage input validation paths
- baseline/diff resolution flow
- report generation branches (markdown/json)
- warning/error branches for missing inputs and malformed coverage

Pass criteria:
- maximize reduction of uncovered changed lines in `backend/cmd/localpatchreport/main.go` from the `200` baseline, with priority on highest-impact uncovered ranges and no new uncovered changed lines introduced
- backend targeted test command passes

Targeted test command:

```bash
cd /projects/Charon/backend && go test ./cmd/localpatchreport -coverprofile=coverage.txt
```

#### Phase 2 — Frontend Lowest-Coverage, Highest-Uncovered Pages

Targets:
- `frontend/src/pages/CrowdSecConfig.tsx` (`975-977`, `1220`, `1248-1249`, `1281-1282`, `1316`, `1324-1325`, `1335`)
- `frontend/src/pages/UsersPage.tsx` (`152-160`)
- `frontend/src/pages/DNSProviders.tsx` (10 uncovered changed lines)

Suggested test files:
- `frontend/src/pages/__tests__/CrowdSecConfig.patch-gap.test.tsx`
- `frontend/src/pages/__tests__/UsersPage.patch-gap.test.tsx`
- `frontend/src/pages/__tests__/DNSProviders.patch-gap.test.tsx`

Test focus:
- branch/error-state rendering tied to uncovered lines
- conditional action handlers and callback guards
- edge-case interaction states not hit by existing tests

Pass criteria:
- maximize reduction of changed-line gaps for the three targets, prioritize highest-impact uncovered lines first, and avoid introducing new uncovered changed lines
- frontend targeted test command passes

Targeted test command:

```bash
cd /projects/Charon/frontend && npm run test:coverage -- src/pages/__tests__/CrowdSecConfig.patch-gap.test.tsx src/pages/__tests__/UsersPage.patch-gap.test.tsx src/pages/__tests__/DNSProviders.patch-gap.test.tsx
```

#### Phase 3 — Backend Residual Middleware/Security Gaps

Targets:
- `backend/internal/api/middleware/auth.go` (3 uncovered changed lines)
- `backend/internal/cerberus/rate_limit.go` (12 uncovered changed lines)

Suggested test targets/files:
- extend `backend/internal/api/middleware/auth_test.go`
- extend `backend/internal/cerberus/rate_limit_test.go`

Test focus:
- auth middleware edge branches (token/context failure paths)
- rate-limit boundary and deny/allow branch coverage

Pass criteria:
- maximize reduction of changed-line gaps for both backend files, prioritize highest-impact uncovered lines first, and avoid introducing new uncovered changed lines
- backend targeted test command passes

Targeted test command:

```bash
cd /projects/Charon/backend && go test ./internal/api/middleware ./internal/cerberus -coverprofile=coverage.txt
```

#### Phase 4 — Frontend Component + Residual Page Gaps

Targets:
- `frontend/src/components/ProxyHostForm.tsx` (12 uncovered changed lines)
- `frontend/src/pages/AuditLogs.tsx` (1 uncovered changed line)
- `frontend/src/pages/Notifications.tsx` (3 uncovered changed lines)

Suggested test files:
- `frontend/src/components/__tests__/ProxyHostForm.patch-gap.test.tsx`
- `frontend/src/pages/__tests__/AuditLogs.patch-gap.test.tsx`
- `frontend/src/pages/__tests__/Notifications.patch-gap.test.tsx`

Test focus:
- form branch paths and validation fallbacks
- single-line residual branch in audit logs
- notification branch handling for low-frequency states

Pass criteria:
- maximize reduction of changed-line gaps for all three targets, prioritize highest-impact uncovered lines first, and avoid introducing new uncovered changed lines
- frontend targeted test command passes

Targeted test command:

```bash
cd /projects/Charon/frontend && npm run test:coverage -- src/components/__tests__/ProxyHostForm.patch-gap.test.tsx src/pages/__tests__/AuditLogs.patch-gap.test.tsx src/pages/__tests__/Notifications.patch-gap.test.tsx
```

### Execution Commands

Run from repository root unless stated otherwise.

1) Backend coverage:

```bash
cd backend && go test ./... -coverprofile=coverage.txt
```

2) Frontend coverage:

```bash
cd frontend && npm run test:coverage
```

3) Local patch report iteration:

```bash
bash scripts/local-patch-report.sh
```

4) Iteration loop (repeat until all target gaps are closed):

```bash
cd backend && go test ./... -coverprofile=coverage.txt
cd /projects/Charon/frontend && npm run test:coverage
cd /projects/Charon && bash scripts/local-patch-report.sh
```

### Phase Completion Checks

- After each phase, rerun `bash scripts/local-patch-report.sh` and confirm that only the next planned target set remains uncovered.
- Do not advance phases when a phase target still shows uncovered changed lines.

### Final Merge-Ready Gate (DoD-Aligned, Warn-Mode Rollout)

This PR is merge-ready only when all conditions are true:

- local patch report runs in warn mode and required artifacts are generated
- practical merge objective: drive a significant reduction in authoritative baseline uncovered changed lines in this PR, prioritizing highest-impact files; `0` remains aspirational and is not a warn-mode merge blocker
- required artifacts exist and are current:
   - `test-results/local-patch-report.md`
   - `test-results/local-patch-report.json`
- backend and frontend coverage commands complete successfully
- DoD checks remain satisfied (E2E first, local patch report preflight, required security/coverage/type/build validations)
