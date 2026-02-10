# Definition of Done Remediation Plan

## 1. Introduction

### Overview
This plan remediates Definition of Done (DoD) blockers identified in QA validation for the Notifications changes. It prioritizes the High severity Docker image vulnerability, restores frontend coverage to the 88% gate (with branch focus), resolves linting failures, and re-runs inconclusive checks to reach a clean DoD pass.

### Objectives
- Eliminate the GHSA-69x3-g4r3-p962 vulnerability in the runtime image.
- Restore frontend coverage to >=88% across lines, statements, functions, and branches.
- Fix markdownlint and hadolint failures.
- Re-run TypeScript and pre-commit checks with clean output capture.

### Scope
- Backend dependency graph inspection for nebula source.
- Frontend test coverage targeting Notifications changes.
- Dockerfile lint compliance fixes.
- Markdown table formatting fixes.
- DoD validation re-runs.

## 2. Research Findings

### QA Report Summary
- Docker image scan failed with GHSA-69x3-g4r3-p962 in `github.com/slackhq/nebula@v1.9.7` (fixed in v1.10.3).
- Frontend coverage below 88% (branches 78.78%).
- Markdownlint failure in tests README table formatting.
- Hadolint failures: DL3059 and SC2012.
- TypeScript and pre-commit checks inconclusive.

### Repository Evidence
- `github.com/slackhq/nebula@v1.10.3` is present in the workspace sum file, implying a pinned module exists in the workspace graph but not necessarily in the runtime image: [go.work.sum](go.work.sum#L57).
- No direct `nebula` dependency appears in the backend module file; the source is likely a transitive dependency from build-time components (Caddy or CrowdSec build stages) or a separate module in the workspace.
- SC2012 triggers from `ls -la` usage inside Dockerfile runtime validation steps: [Dockerfile](Dockerfile#L429) and [Dockerfile](Dockerfile#L441).
- Markdownlint failure appears in the Test Execution Metrics table: [tests/README.md](tests/README.md#L429-L435).
- New URL validation and update indicator logic in Notifications UI that likely needs test coverage:
  - `validateUrl` logic: [frontend/src/pages/Notifications.tsx](frontend/src/pages/Notifications.tsx#L110)
  - URL validation wiring: [frontend/src/pages/Notifications.tsx](frontend/src/pages/Notifications.tsx#L159)
  - Update indicator state and timer: [frontend/src/pages/Notifications.tsx](frontend/src/pages/Notifications.tsx#L364-L396)
  - Update indicator rendering: [frontend/src/pages/Notifications.tsx](frontend/src/pages/Notifications.tsx#L549)

### Known Contextual Signals
- Prior security report indicates nebula was patched to 1.10.3 for a different CVE, but the current image scan still detects 1.9.7. This suggests image build steps might be pulling a separate older version during Caddy or CrowdSec build stages.

## 3. Technical Specifications

### 3.1 EARS Requirements (DoD Remediation)
- WHEN the runtime image is scanned, THE SYSTEM SHALL report zero HIGH or CRITICAL vulnerabilities.
- WHEN frontend coverage is executed, THE SYSTEM SHALL report at least 88% for lines, statements, functions, and branches.
- WHEN markdownlint runs, THE SYSTEM SHALL report zero lint errors.
- WHEN hadolint runs, THE SYSTEM SHALL report zero DL3059 or SC2012 findings.
- WHEN TypeScript checks and pre-commit hooks are executed, THE SYSTEM SHALL report PASS with complete output.

### 3.2 Dependency Remediation Strategy (Nebula)
- Identify the actual module path pulling `github.com/slackhq/nebula@v1.9.7` by inspecting all build-stage module graphs, with priority on Caddy and CrowdSec build stages.
- Upgrade the dependency at the source module to `v1.10.3` or later and regenerate module sums.
- Rebuild the Docker image and confirm the fix via a container scan (Grype/Trivy).

### 3.3 Frontend Coverage Strategy
- Use the coverage report to pinpoint missing lines/branches in the Notifications flow.
- Add Vitest unit tests for `Notifications.tsx` that cover URL validation branches (invalid protocol, malformed URL, empty allowed), update indicator timer behavior, and form reset state.
- Target frontend unit test files (e.g., `frontend/src/pages/__tests__/Notifications.test.tsx`) and related helpers; do not rely on Playwright E2E for coverage gates.
- Ensure coverage is verified through the standard coverage task for frontend.
- Note: E2E tests verify behavior but do not contribute to Vitest coverage gates.

### 3.4 Lint Fix Strategy
- Markdownlint: correct table spacing (align column pipes consistently).
- Hadolint:
  - DL3059: consolidate consecutive `RUN` steps in affected stages where possible.
   - SC2012: replace `ls -la` usages with `stat` or `test -e` for deterministic existence checks.

### 3.5 Validation Strategy
- Re-run TypeScript check and pre-commit hooks with clean capture.
- Re-run full DoD sequence (E2E already passing for notifications).

## 4. Implementation Plan

### Phase 1: High-Priority Nebula Upgrade (P0)

Status: ACCEPTED RISK (was BLOCKED)
Note: Proceeding to Phase 2-4 with documented security exception.

**Commands**
1. Locate dependency source (module graph):
   - `cd backend && go mod why -m github.com/slackhq/nebula`
   - `rg "slackhq/nebula" -n backend .docker docs configs`
   - If dependency is in build-stage modules, inspect Caddy and CrowdSec build steps by capturing build logs or inspecting generated go.mod within the builder stage.
2. Upgrade to v1.10.3+ at the source module:
   - `go get github.com/slackhq/nebula@v1.10.3` (in the module where it is pulled)
   - `go mod tidy`
3. Rebuild image and rescan:
   - `.github/skills/scripts/skill-runner.sh docker-rebuild-e2e`
   - `.github/skills/scripts/skill-runner.sh security-scan-docker-image`

**Rollback Plan**
- If the upgrade fails, run `git restore backend/go.mod backend/go.sum` (or `Dockerfile` if the patch was applied in a build stage) and rebuild the image.

**Checkpoint**
- STOP: If GHSA-69x3-g4r3-p962 persists after the image scan, reassess the dependency source before continuing to Phase 2. Likely sources are the Caddy builder stage or CrowdSec builder stage module graphs.

**Files to Modify (Expected)**
- If dependency is in backend module: [backend/go.mod](backend/go.mod) and [backend/go.sum](backend/go.sum).
- If dependency is in a build-stage module (Caddy/CrowdSec builder), update the patching logic in [Dockerfile](Dockerfile) in the relevant build stage.

**Expected Outcomes**
- Grype/Trivy reports zero HIGH/CRITICAL vulnerabilities.
- GHSA-69x3-g4r3-p962 removed from image scan output.

**Risks**
- Dependency upgrade could impact Caddy/CrowdSec build reproducibility or plugin compatibility.
- If the dependency is tied to a third-party module (xcaddy build), upgrades may require explicit `go get` overrides.

### Phase 2: Frontend Coverage Improvement (P1)

**Commands**
1. Run verbose coverage:
   - `cd frontend && npm run test:coverage -- --reporter=verbose`
2. Inspect the HTML report:
   - `open coverage/lcov-report/index.html`
3. Identify missing lines/branches in Notifications components and related utilities.

**Files to Modify (Expected)**
- Frontend unit tests (Vitest): add or update `frontend/src/pages/__tests__/Notifications.test.tsx` (or existing test files in `frontend/src/pages/__tests__/`).
- Component coverage targets:
   - URL validation: [frontend/src/pages/Notifications.tsx](frontend/src/pages/Notifications.tsx#L110-L166)
   - Update indicator timer and render: [frontend/src/pages/Notifications.tsx](frontend/src/pages/Notifications.tsx#L364-L549)

**Expected Outcomes**
- Coverage meets or exceeds 88% for lines, statements, functions, branches.
- Patch coverage reaches 100% for all modified lines (Codecov patch view).

**Risks**
- Additional tests may require stable mock setup for API calls and timers.
- Over-mocking can hide real behavior; ensure branch coverage reflects actual runtime behavior.

**Checkpoint**
- Verify coverage >=88% before starting lint fixes.

### Phase 3: Lint Fixes (P2)

**Commands**
1. Markdownlint:
   - `npm run lint:markdown`
2. Hadolint:
   - `docker run --rm -i hadolint/hadolint < Dockerfile`

**Files to Modify**
- Markdown table formatting: [tests/README.md](tests/README.md#L429-L435)
- Dockerfile lint issues:
  - SC2012 replacements: [Dockerfile](Dockerfile#L429) and [Dockerfile](Dockerfile#L441)
   - DL3059 consolidation of adjacent RUN instructions in the affected stages (specify the exact stage during implementation to limit cache impact to that stage only).

**Expected Outcomes**
- Markdownlint passes with zero errors.
- Hadolint passes with zero DL3059 or SC2012 findings.

**Risks**
- Consolidating RUN steps may impact layer caching; ensure build outputs are unchanged.

### Phase 4: Validation Re-runs (P3)

**Commands**
1. E2E (mandatory first):
   - `npx playwright test --project=firefox`
2. Pre-commit (all files):
   - `pre-commit run --all-files`
3. TypeScript check:
   - `cd frontend && npm run type-check`
4. Other DoD validations (as required):
   - Frontend coverage: `scripts/frontend-test-coverage.sh`
   - Backend coverage (if impacted): `scripts/go-test-coverage.sh`
   - Security scans: CodeQL and Trivy/Grype tasks

**Order Note**
- Per .github/instructions/testing.instructions.md, E2E is mandatory first validation. Sequence must be E2E -> pre-commit -> TypeScript -> other validations.

**Expected Outcomes**
- TypeScript and pre-commit checks show PASS with complete logs.
- DoD gates pass with zero blocking findings.

**Risks**
- Pre-commit hooks may surface additional lint failures requiring quick fixes.

## 5. Decision Record

### Decision - 2026-02-10
**Decision**: How to remediate `nebula@v1.9.7` in the runtime image.

**Context**: The image scan finds a High vulnerability in `github.com/slackhq/nebula@v1.9.7`, but the workspace already contains `v1.10.3` in the sum file. The actual source module is unknown and likely part of the Caddy or CrowdSec build stages.

**Options**:
1. Add a direct dependency override in the source module that pulls `nebula` (e.g., `go get` or `replace` in the build-stage module).
2. Add a forced `go get github.com/slackhq/nebula@v1.10.3` patch in the Caddy/CrowdSec builder stage after xcaddy generates its `go.mod`.
3. Upgrade the dependent plugin or dependency chain to a release that already pins `nebula@v1.10.3+`.

**Rationale**: Option 2 offers the most deterministic fix when the dependency is introduced in generated build-stage modules. Option 3 is preferred if a plugin release provides a clean upstream fix without manual overrides.

**Impact**: Ensures the runtime image is free of the known vulnerability and aligns build-stage dependencies with security requirements.

**Review**: Reassess if upstream plugins release versions that pin the dependency and allow removal of manual overrides.

## 6. Acceptance Criteria

- Docker image scan reports zero HIGH/CRITICAL vulnerabilities and GHSA-69x3-g4r3-p962 is absent.
- Frontend coverage meets or exceeds 88% for lines, statements, functions, and branches.
- Markdownlint passes with no table formatting errors.
- Hadolint passes with no DL3059 or SC2012 findings.
- TypeScript check and pre-commit hooks complete with PASS output.
- DoD validation is unblocked and ready for Supervisor review.

## 7. Verification Matrix

| Phase | Check | Expected Artifact | Status |
| --- | --- | --- | --- |
| P0 | Docker scan | grype-results.json shows 0 HIGH/CRITICAL | ⏸️ |
| P0 | Dependency source confirmed | Builder-stage or module graph notes captured | ⏸️ |
| P1 | Frontend coverage | coverage/lcov-report/index.html shows >=88% | ⏸️ |
| P2 | Markdownlint | npm run lint:markdown passes | ⏸️ |
| P2 | Hadolint | hadolint passes with no DL3059/SC2012 | ⏸️ |
| P3 | E2E | Playwright run passes | ⏸️ |
| P3 | Pre-commit | pre-commit run --all-files passes | ⏸️ |
| P3 | TypeScript | npm run type-check passes | ⏸️ |
| P3 | Coverage (if impacted) | scripts/*-test-coverage.sh passes | ⏸️ |
| P3 | Security scans | CodeQL/Trivy/Grype pass | ⏸️ |
