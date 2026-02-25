---
post_title: "Current Spec: Local Docker Socket Group Access Remediation"
categories:
  - planning
  - docker
  - security
  - backend
  - frontend
tags:
  - docker.sock
  - least-privilege
  - group-add
  - compose
  - validation
summary: "Comprehensive plan to resolve local docker socket access failures for non-root process uid=1000 gid=1000 when host socket gid is not in supplemental groups, with phased rollout, PR slicing, and least-privilege validation."
post_date: 2026-02-25
---

## 1) Introduction

### Overview

Charon local Docker discovery currently fails in environments where:

- Socket mount exists: `/var/run/docker.sock:/var/run/docker.sock:ro`
- Charon process runs non-root (typically `uid=1000 gid=1000`)
- Host socket group (example: `gid=988`) is not present in process supplemental groups

Observed user-facing failure class (already emitted by backend details builder):

- `Local Docker socket mounted but not accessible by current process (uid=1000 gid=1000)... Process groups do not include socket gid 988; run container with matching supplemental group (e.g., --group-add 988).`

### Goals

1. Preserve non-root default execution (`USER charon`) while enabling local Docker discovery safely.
2. Standardize supplemental-group strategy across compose variants and launcher scripts.
3. Keep behavior deterministic in backend/API/frontend error surfacing when permissions are wrong.
4. Validate least-privilege posture (non-root, minimal group grant, no broad privilege escalation).

### Non-Goals

- No redesign of remote Docker support (`tcp://...`) beyond compatibility checks.
- No changes to unrelated security modules (WAF, ACL, CrowdSec workflows).
- No broad Docker daemon hardening beyond this socket-access path.

### Scope Labels (Authoritative)

- `repo-deliverable`: changes that must be included in repository PR slices under `/projects/Charon`.
- `operator-local follow-up`: optional local environment changes outside repository scope (for example `/root/docker/...`), not required for repo PR acceptance.

---

## 2) Research Findings

### 2.1 Critical Runtime Files (Confirmed)

- `backend/internal/services/docker_service.go`
  - Key functions:
    - `NewDockerService()`
    - `(*DockerService).ListContainers(...)`
    - `resolveLocalDockerHost()`
    - `buildLocalDockerUnavailableDetails(...)`
    - `isDockerConnectivityError(...)`
    - `extractErrno(...)`
    - `localSocketStatSummary(...)`
  - Contains explicit supplemental-group hint text with `--group-add <gid>` when `EACCES/EPERM` occurs.

- `backend/internal/api/handlers/docker_handler.go`
  - Key function: `(*DockerHandler).ListContainers(...)`
  - Maps `DockerUnavailableError` to HTTP `503` with `details` string consumed by UI.

- `frontend/src/hooks/useDocker.ts`
  - Hook: `useDocker(host?, serverId?)`
  - Converts `503` payload details into surfaced `Error(message)`.

- `frontend/src/components/ProxyHostForm.tsx`
  - Uses `useDocker`.
  - Error panel title: `Docker Connection Failed`.
  - Existing troubleshooting text currently mentions socket mount but not explicit supplemental group action.

- `.docker/docker-entrypoint.sh`
  - Root path auto-aligns docker socket GID with user group membership via:
    - `get_group_by_gid()`
    - `create_group_with_gid()`
    - `add_user_to_group()`
  - Non-root path logs generic `--group-add` guidance but does not include resolved host socket GID.

- `Dockerfile`
  - Creates non-root user `charon` (uid/gid 1000) and final `USER charon`.
  - This is correct for least privilege and should remain default.

### 2.2 Compose and Script Surface Area

Primary in-repo compose files with docker socket mount:

- `.docker/compose/docker-compose.yml` (`charon` service)
- `.docker/compose/docker-compose.local.yml` (`charon` service)
- `.docker/compose/docker-compose.dev.yml` (`app` service)
- `.docker/compose/docker-compose.playwright-local.yml` (`charon-e2e` service)
- `.docker/compose/docker-compose.playwright-ci.yml` (`charon-app`, `crowdsec` services)

Primary out-of-repo/local-ops file in active workspace:

- `/root/docker/containers/charon/docker-compose.yml` (`charon` service)
  - Includes socket mount.
  - `user:` is currently commented out.
  - No `group_add` entry exists.

Launcher scripts discovered:

- `.github/skills/docker-start-dev-scripts/run.sh`
  - Runs: `docker compose -f .docker/compose/docker-compose.dev.yml up -d`
- `/root/docker/containers/charon/docker-compose-up-charon.sh`
  - Runs: `docker compose up -d`

### 2.3 Existing Tests Relevant to This Failure

Backend service tests (`backend/internal/services/docker_service_test.go`):

- `TestBuildLocalDockerUnavailableDetails_PermissionDeniedIncludesGroupHint`
- `TestBuildLocalDockerUnavailableDetails_MissingSocket`
- Connectivity classification tests across URL/syscall/network errors.

Backend handler tests (`backend/internal/api/handlers/docker_handler_test.go`):

- `TestDockerHandler_ListContainers_DockerUnavailableMappedTo503`
- Other selector and remote-host mapping tests.

Frontend hook tests (`frontend/src/hooks/__tests__/useDocker.test.tsx`):

- `it('extracts details from 503 service unavailable error', ...)`

### 2.4 Config Review Findings (`.gitignore`, `codecov.yml`, `.dockerignore`, `Dockerfile`)

- `.gitignore`: no blocker for this feature; already excludes local env/artifacts extensively.
- `.dockerignore`: no blocker for this feature; includes docs/tests and build artifacts exclusions.
- `Dockerfile`: non-root default is aligned with least-privilege intent.
- `codecov.yml`: currently excludes the two key Docker logic files:
  - `backend/internal/services/docker_service.go`
  - `backend/internal/api/handlers/docker_handler.go`
  This exclusion undermines regression visibility for this exact problem class and should be revised.

### 2.5 Confidence

Confidence score: **97%**

Reasoning:

- Root cause and symptom path are already explicit in code.
- Required files and control points are concrete and localized.
- Existing tests already cover adjacent behavior and reduce implementation risk.

---

## 3) Requirements (EARS)

- WHEN local Docker source is selected and `/var/run/docker.sock` is mounted, THE SYSTEM SHALL return containers if the process has supplemental membership for socket GID.
- WHEN local Docker source is selected and socket permissions deny access (`EACCES`/`EPERM`), THE SYSTEM SHALL return HTTP `503` with a deterministic, actionable details message including supplemental-group guidance.
- WHEN container runs non-root and socket GID is known, THE SYSTEM SHALL provide explicit startup diagnostics indicating the required `group_add` value.
- WHEN docker-compose-based local/dev startup is used, THE SYSTEM SHALL support local-only `group_add` configuration from host socket GID without requiring root process runtime.
- WHEN remote Docker source is selected (`server_id` path), THE SYSTEM SHALL remain functionally unchanged.
- WHEN least-privilege validation is executed, THE SYSTEM SHALL demonstrate non-root process execution and only necessary supplemental group grant.
- IF resolved socket GID equals `0`, THEN THE SYSTEM SHALL require explicit operator opt-in and risk acknowledgment before any `group_add: ["0"]` path is used.

---

## 4) Technical Specifications

### 4.1 Architecture and Data Flow

User flow:

1. UI `ProxyHostForm` sets source = `Local (Docker Socket)`.
2. `useDocker(...)` calls `dockerApi.listContainers(...)`.
3. Backend `DockerHandler.ListContainers(...)` invokes `DockerService.ListContainers(...)`.
4. If socket access denied, backend emits `DockerUnavailableError` with details.
5. Handler returns `503` JSON `{ error, details }`.
6. Frontend surfaces message in `Docker Connection Failed` block.

No database schema change is required.

### 4.2 API Contract (No endpoint shape change)

Endpoint:

- `GET /api/v1/docker/containers`
  - Query params:
    - `host` (allowed: empty or `local` only)
    - `server_id` (UUID for remote server lookup)

Responses:

- `200 OK`: `DockerContainer[]`
- `503 Service Unavailable`:
  - `error: "Docker daemon unavailable"`
  - `details: <actionable message>`
- `400`, `404`, `500` unchanged.

### 4.3 Deterministic `group_add` Policy (Chosen)

Chosen policy: **conditional local-only profile/override while keeping CI unaffected**.

Authoritative policy statement:

1. `repo-deliverable`: repository compose paths used for local operator runs (`.docker/compose/docker-compose.local.yml`, `.docker/compose/docker-compose.dev.yml`) may include local-only `group_add` wiring using `DOCKER_SOCK_GID`.
2. `repo-deliverable`: CI compose paths (`.docker/compose/docker-compose.playwright-ci.yml`) remain unaffected by this policy and must not require `DOCKER_SOCK_GID`.
3. `repo-deliverable`: base compose (`.docker/compose/docker-compose.yml`) remains safe by default and must not force a local host-specific GID requirement in CI.
4. `operator-local follow-up`: out-of-repo operator files (for example `/root/docker/containers/charon/docker-compose.yml`) may mirror this policy but are explicitly outside mandatory repo PR scope.

CI compatibility statement:

- CI workflows remain deterministic because they do not depend on local host socket GID export for this remediation.
- No CI job should fail due to missing `DOCKER_SOCK_GID` after this plan.

Security guardrail for `gid==0` (mandatory):

- If `stat -c '%g' /var/run/docker.sock` returns `0`, local profile/override usage must fail closed by default.
- Enabling `group_add: ["0"]` requires explicit opt-in (for example `ALLOW_DOCKER_SOCK_GID_0=true`) and documented risk acknowledgment in operator guidance.
- Silent fallback to GID `0` is prohibited.

### 4.4 Entrypoint Diagnostic Improvements

In `.docker/docker-entrypoint.sh` non-root socket branch:

- Extend current message to include resolved socket GID from `stat -c '%g' /var/run/docker.sock`.
- Emit exact recommendation format:
  - `Use docker compose group_add: ["<gid>"] or run with --group-add <gid>`
- If resolved GID is `0`, emit explicit warning requiring opt-in/risk acknowledgment instead of generic recommendation.

No privilege escalation should be introduced.

### 4.5 Frontend UX Message Precision

In `frontend/src/components/ProxyHostForm.tsx` troubleshooting text:

- Retain mount guidance.
- Add supplemental-group guidance for containerized runs.
- Keep language concise and operational.

### 4.6 Coverage and Quality Config Adjustments

`codecov.yml` review outcome:

- Proposed: remove Docker logic file ignores for:
  - `backend/internal/services/docker_service.go`
  - `backend/internal/api/handlers/docker_handler.go`
- Reason: this issue is rooted in these files; exclusion hides regressions.

`.gitignore` review outcome:

- No change required for core remediation.

`.dockerignore` review outcome:

- No required change for runtime fix.
- Optional follow-up: verify no additional local-only compose/env files are copied in future.

`Dockerfile` review outcome:

- No required behavioral change; preserve non-root default.

---

## 5) Risks, Edge Cases, Mitigations

### Risks

1. Host socket GID differs across environments (`docker` group not stable numeric ID).
2. CI runners may not permit or need explicit `group_add` depending on runner Docker setup.
3. Over-granting groups could violate least-privilege intent.
4. Socket GID can be `0` on some hosts and implies root-group blast radius.

### Edge Cases

- Socket path missing (`ENOENT`) remains handled with existing details path.
- Rootless host Docker sockets (`/run/user/<uid>/docker.sock`) remain selectable by `resolveLocalDockerHost()`.
- Remote server discovery path (`tcp://...`) must remain unaffected.

### Mitigations

- Use environment-substituted `DOCKER_SOCK_GID`, not hardcoded `988` in committed compose files.
- Keep `group_add` scoped only to local operator flows that require socket discovery.
- Fail closed on `DOCKER_SOCK_GID=0` unless explicit opt-in and risk acknowledgment are present.
- Verify `id` output inside container to confirm only necessary supplemental group is present.

---

## 6) Implementation Plan (Phased, minimal request count)

Design principle for phases: maximize delivery per request by grouping strongly-related changes into each phase and minimizing handoffs.

### Phase 1 — Baseline + Diagnostics + Compose Foundations

Scope:

1. Compose updates in local/dev paths to support local-only `group_add` via `DOCKER_SOCK_GID`.
2. Entrypoint diagnostic enhancement for non-root socket path.

`repo-deliverable` files:

- `.docker/compose/docker-compose.local.yml`
- `.docker/compose/docker-compose.dev.yml`
- `.docker/docker-entrypoint.sh`

`operator-local follow-up` files (non-blocking, out of repo PR scope):

- `/root/docker/containers/charon/docker-compose.yml`
- `/root/docker/containers/charon/docker-compose-up-charon.sh`

Deliverables:

- Deterministic startup guidance and immediate local remediation path.

### Phase 2 — API/UI Behavior Tightening + Tests

Scope:

1. Preserve and, if needed, refine backend detail text consistency in `buildLocalDockerUnavailableDetails(...)`.
2. UI troubleshooting copy update in `ProxyHostForm.tsx`.
3. Expand/refresh tests for permission-denied + supplemental-group hint rendering path.

Primary files:

- `backend/internal/services/docker_service.go`
- `backend/internal/services/docker_service_test.go`
- `backend/internal/api/handlers/docker_handler.go`
- `backend/internal/api/handlers/docker_handler_test.go`
- `frontend/src/hooks/useDocker.ts`
- `frontend/src/hooks/__tests__/useDocker.test.tsx`
- `frontend/src/components/ProxyHostForm.tsx`
- `frontend/src/components/__tests__/ProxyHostForm*.test.tsx`

Deliverables:

- User sees precise, actionable guidance when failure occurs.
- Regression tests protect failure classification and surfaced guidance.

### Phase 3 — Coverage Policy + Documentation + CI/Validation Hardening

Scope:

1. Remove Docker logic exclusions in `codecov.yml`.
2. Update docs to include `group_add` guidance where socket mount is described.
3. Validate CI/playwright compose behavior remains unaffected and verify local least-privilege checks.

Primary files:

- `codecov.yml`
- `README.md`
- `docs/getting-started.md`
- `SECURITY.md`
- `.vscode/tasks.json` (only if adding dedicated validation task labels)

Deliverables:

- Documentation and coverage policy match runtime behavior.
- Verified validation playbook for operators and CI.

---

## 7) PR Slicing Strategy

### Decision

**Split into multiple PRs (PR-1 / PR-2 / PR-3).**

### Trigger Reasons

- Cross-domain change set (compose + shell entrypoint + backend + frontend + tests + docs + coverage policy).
- Distinct rollback boundaries needed (runtime config vs behavior vs governance/reporting).
- Faster and safer review with independently verifiable increments.

### Ordered PR Slices

#### PR-1: Runtime Access Foundation (Compose + Entrypoint)

Scope:

- Add local-only `group_add` strategy to local/dev compose flows.
- Improve non-root entrypoint diagnostics to print required GID.

Files (expected):

- `.docker/compose/docker-compose.local.yml`
- `.docker/compose/docker-compose.dev.yml`
- `.docker/docker-entrypoint.sh`

Operator-local follow-up (not part of repo PR gate):

- `/root/docker/containers/charon/docker-compose.yml`
- `/root/docker/containers/charon/docker-compose-up-charon.sh`

Dependencies:

- None.

Acceptance criteria:

1. Container remains non-root (`id -u = 1000`).
2. With local-only config enabled and `DOCKER_SOCK_GID` exported, `id -G` inside container includes socket GID.
3. `GET /api/v1/docker/containers?host=local` no longer fails due to `EACCES` in correctly configured environment.
4. If resolved socket GID is `0`, setup fails by default unless explicit opt-in and risk acknowledgment are provided.

Rollback/contingency:

- Revert compose and entrypoint deltas only.

#### PR-2: Behavior + UX + Tests

Scope:

- Backend details consistency (if required).
- Frontend troubleshooting message update.
- Add/adjust tests around permission-denied + supplemental-group guidance.

Files (expected):

- `backend/internal/services/docker_service.go`
- `backend/internal/services/docker_service_test.go`
- `backend/internal/api/handlers/docker_handler.go`
- `backend/internal/api/handlers/docker_handler_test.go`
- `frontend/src/hooks/useDocker.ts`
- `frontend/src/hooks/__tests__/useDocker.test.tsx`
- `frontend/src/components/ProxyHostForm.tsx`
- `frontend/src/components/__tests__/ProxyHostForm*.test.tsx`

Dependencies:

- PR-1 recommended (runtime setup available for realistic local validation).

Acceptance criteria:

1. `503` details include actionable group guidance for permission-denied scenarios.
2. UI error panel provides mount + supplemental-group troubleshooting.
3. All touched unit/e2e tests pass for local Docker source path.

Rollback/contingency:

- Revert only behavior/UI/test deltas; keep PR-1 foundations.

#### PR-3: Coverage + Docs + Validation Playbook

Scope:

- Update `codecov.yml` exclusions for Docker logic files.
- Update user/operator docs where socket mount guidance appears.
- Optional task additions for socket-permission diagnostics.

Files (expected):

- `codecov.yml`
- `README.md`
- `docs/getting-started.md`
- `SECURITY.md`
- `.vscode/tasks.json` (optional)

Dependencies:

- PR-2 preferred to ensure policy aligns with test coverage additions.

Acceptance criteria:

1. Codecov includes Docker service/handler in coverage accounting.
2. Docs show both socket mount and supplemental-group requirement.
3. Validation command set is documented and reproducible.

Rollback/contingency:

- Revert reporting/docs/task changes only.

---

## 8) Validation Strategy (Protocol-Ordered)

### 8.1 E2E Prerequisite / Rebuild Check (Mandatory First)

Follow project protocol to decide whether E2E container rebuild is required before tests:

1. If application/runtime or Docker build inputs changed, rebuild E2E environment.
2. If only test files changed and environment is healthy, reuse current container.
3. If environment state is suspect, rebuild.

Primary task:

- VS Code task: `Docker: Rebuild E2E Environment` (or clean variant when needed).

### 8.2 E2E First (Mandatory)

Run E2E before unit tests:

- VS Code task: `Test: E2E Playwright (Targeted Suite)` for scoped regression checks.
- VS Code task: `Test: E2E Playwright (Skill)` for broader safety pass as needed.

### 8.3 Local Patch Report (Mandatory Before Unit/Coverage)

Generate patch artifacts immediately after E2E:

```bash
cd /projects/Charon
bash scripts/local-patch-report.sh
```

Required artifacts:

- `test-results/local-patch-report.md`
- `test-results/local-patch-report.json`

### 8.4 Unit + Coverage Validation

Backend and frontend unit coverage gates after patch report:

```bash
cd /projects/Charon/backend && go test ./internal/services ./internal/api/handlers
cd /projects/Charon/frontend && npm run test -- src/hooks/__tests__/useDocker.test.tsx
```

Then run coverage tasks/scripts per project protocol (minimum threshold enforcement remains unchanged).

### 8.5 Least-Privilege + `gid==0` Guardrail Checks

Pass conditions:

1. Container process remains non-root.
2. Supplemental group grant is limited to socket GID only for local operator flow.
3. No privileged mode or unrelated capability additions.
4. Socket remains read-only.
5. If socket GID resolves to `0`, local run fails closed unless explicit opt-in and risk acknowledgment are present.

---

## 9) Suggested File-Level Updates Summary

### `repo-deliverable` Must Update

- `.docker/compose/docker-compose.local.yml`
- `.docker/compose/docker-compose.dev.yml`
- `.docker/docker-entrypoint.sh`
- `frontend/src/components/ProxyHostForm.tsx`
- `codecov.yml`

### `repo-deliverable` Should Update

- `README.md`
- `docs/getting-started.md`
- `SECURITY.md`

### `repo-deliverable` Optional Update

- `.vscode/tasks.json` (dedicated task to precompute/export `DOCKER_SOCK_GID` and start compose)

### `operator-local follow-up` (Out of Mandatory Repo PR Scope)

- `/root/docker/containers/charon/docker-compose.yml`
- `/root/docker/containers/charon/docker-compose-up-charon.sh`

### Reviewed, No Required Change

- `.gitignore`
- `.dockerignore`
- `Dockerfile` (keep non-root default)

---

## 10) Acceptance Criteria / DoD

1. Local Docker source works in non-root container when supplemental socket group is supplied.
2. Failure path remains explicit and actionable when supplemental group is missing.
3. Scope split is explicit and consistent: `repo-deliverable` vs `operator-local follow-up`.
4. Chosen policy is unambiguous: conditional local-only `group_add`; CI remains unaffected.
5. `gid==0` path is guarded by explicit opt-in/risk acknowledgment and never silently defaulted.
6. Validation order is protocol-aligned: E2E prerequisite/rebuild check -> E2E first -> local patch report -> unit/coverage.
7. Coverage policy no longer suppresses Docker service/handler regression visibility.
8. PR-1, PR-2, PR-3 each pass their slice acceptance criteria with independent rollback safety.
9. This file contains one active plan with one frontmatter block and no archived concatenated plan content.

---

## 11) Handoff

This plan is complete and execution-ready for Supervisor review. It includes:

- Root-cause grounded file/function map
- EARS requirements
- Specific multi-phase implementation path
- PR slicing with dependencies and rollback notes
- Validation sequence explicitly aligned to project protocol order and least-privilege guarantees
