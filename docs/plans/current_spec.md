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
