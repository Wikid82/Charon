## Coverage Scope Refinement Spec (Authoritative)

Date: 2026-02-16
Owner: Planning Agent
Status: Active / Authoritative for this objective only

## 1) Objective

Narrow scope only:
- Remove frontend unit-test quarantine execution excludes in
  `frontend/vitest.config.ts`.
- Include selected unit-test-related code in coverage accounting by updating
  `codecov.yml` and `scripts/go-test-coverage.sh`.
- Keep integration/trace exclusions unchanged.
- Keep Docker-only backend exclusions unchanged unless a deterministic,
  CI-testable strategy is explicitly added (not part of this plan).

Out of scope:
- E2E implementation changes.
- Integration coverage model changes.
- Backend Docker socket-dependent coverage inclusion.
- Legacy additive coverage programs not required for this objective.

## 2) Blocker Resolution Mapping

### Supervisor Blocker 1: Correct Codecov line references

Use actual line numbers from `codecov.yml`:
- Entry-point ignore lines are `90-92`:
  - `90`: `backend/cmd/api/**`
  - `91`: `backend/cmd/seed/**`
  - `92`: `frontend/src/main.tsx`

### Supervisor Blocker 2: Keep Docker exclusions by default

Do **not** remove these ignore entries in this plan:
- `backend/internal/services/docker_service.go` (line `105`)
- `backend/internal/api/handlers/docker_handler.go` (line `106`)

Rationale:
- They require Docker socket/runtime conditions and are not deterministic in
  standard CI unit coverage execution.
- No explicit deterministic CI-testable strategy is introduced in this plan.

### Supervisor Blocker 3: Remove ambiguity and legacy scope

This document is now authoritative for only the current objective and excludes
legacy/additive planning content.

### Supervisor Blocker 4: Add explicit post-change verification checks

Post-change checks now require grep-style assertions proving integration/trace
exclusions remain and coverage gates still pass.

## 3) Exact Planned Changes

### A. Frontend quarantine execution excludes (`frontend/vitest.config.ts`)

Remove only the temporary quarantine entries so tests execute again.

Keep existing generic exclusions unchanged (examples):
- `node_modules/**`
- `dist/**`
- `e2e/**`
- `tests/**`

### B. Backend script coverage exclusions (`scripts/go-test-coverage.sh`)

From `EXCLUDE_PACKAGES`, remove only:
- `github.com/Wikid82/charon/backend/cmd/api`
- `github.com/Wikid82/charon/backend/cmd/seed`

Keep unchanged:
- `github.com/Wikid82/charon/backend/internal/trace`
- `github.com/Wikid82/charon/backend/integration`

### C. Codecov ignores (`codecov.yml`)

Remove from ignore list:
- `backend/cmd/api/**` (line `90`)
- `backend/cmd/seed/**` (line `91`)
- `frontend/src/main.tsx` (line `92`)

Keep unchanged:
- `backend/internal/trace/**` (line `99`)
- `backend/integration/**` (line `100`)
- `backend/internal/services/docker_service.go` (line `105`)
- `backend/internal/api/handlers/docker_handler.go` (line `106`)

## 4) Verification Plan (Mandatory)

### A. Static grep-style assertions for preserved exclusions

Run and expect matches:

```bash
grep -n 'backend/internal/trace/\*\*' codecov.yml
grep -n 'backend/integration/\*\*' codecov.yml
grep -n 'backend/internal/services/docker_service.go' codecov.yml
grep -n 'backend/internal/api/handlers/docker_handler.go' codecov.yml
grep -n 'backend/internal/trace' scripts/go-test-coverage.sh
grep -n 'backend/integration' scripts/go-test-coverage.sh
```

Run and expect **no** matches for removed entries:

```bash
grep -n 'backend/cmd/api/\*\*' codecov.yml
grep -n 'backend/cmd/seed/\*\*' codecov.yml
grep -n 'frontend/src/main.tsx' codecov.yml
grep -n 'github.com/Wikid82/charon/backend/cmd/api' scripts/go-test-coverage.sh
grep -n 'github.com/Wikid82/charon/backend/cmd/seed' scripts/go-test-coverage.sh
```

### B. Coverage status gate checks

Required commands:

```bash
.github/skills/scripts/skill-runner.sh test-frontend-unit
.github/skills/scripts/skill-runner.sh test-frontend-coverage
.github/skills/scripts/skill-runner.sh test-backend-unit
.github/skills/scripts/skill-runner.sh test-backend-coverage
```

Pass criteria:
- Frontend and backend coverage gates remain green (project thresholds).
- No regression caused by accidental integration/trace exclusion changes.

## 5) Acceptance Criteria

- Codecov entry-point line references in this plan are corrected to lines
  `90-92`.
- Docker service/handler excludes remain in `codecov.yml`.
- Integration and trace excludes remain unchanged in `codecov.yml` and
  `scripts/go-test-coverage.sh`.
- Frontend quarantine execution excludes are removed as planned.
- Selected unit-test code is included in coverage accounting as planned.
- Grep-style verification checks are executed and documented.
- Coverage status gates continue to pass.
