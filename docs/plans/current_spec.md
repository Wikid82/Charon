## Data Consistency Failure Remediation Spec

Date: 2026-02-15
Owner: Planning Agent
Scope: Single failure only

---

## 1) Introduction

This specification addresses only the failing Playwright assertion:

- `[firefox] tests/core/data-consistency.spec.ts:327 Failed transaction prevents partial data updates`

Failure signal:
- Expected: original domain remains unchanged after invalid update attempt.
- Received: empty string (`''`) for domain.

Anchor from provided failure report:
- Stack trace lines 369-372 (poll/assertion block in this scenario).

Current file anchor (same logical block in workspace revision):
- `tests/core/data-consistency.spec.ts` lines around 379-382:
  - `return proxy.domain_names || proxy.domainNames || '';`
  - message: `Expected proxy ... domain to remain unchanged after failed update`

Objective:
- Define a deterministic, minimal-request remediation plan that resolves this failure at root cause.
- Keep changes narrowly scoped to this scenario.

---

## 2) Research Findings

### 2.1 Test logic findings

File inspected:
- `tests/core/data-consistency.spec.ts`

Relevant behavior:
- Creates a proxy via `POST /api/v1/proxy-hosts`.
- Sends invalid update payload `domain_names: ''` via `PUT /api/v1/proxy-hosts/:uuid`.
- Accepts status `[200, 400, 422]`.
- Asserts domain remains unchanged afterward.

Observation:
- The test currently permits `200` for the invalid update step while still expecting unchanged data, which weakens contract clarity.

### 2.2 API behavior findings

Files/functions inspected:
- `backend/internal/api/handlers/proxy_host_handler.go`
  - `func (h *ProxyHostHandler) Update(c *gin.Context)`
- `backend/internal/services/proxyhost_service.go`
  - `func (s *ProxyHostService) Update(host *models.ProxyHost) error`
  - `func (s *ProxyHostService) ValidateUniqueDomain(domainNames string, excludeID uint) error`
  - `func (s *ProxyHostService) validateProxyHost(host *models.ProxyHost) error`
- `backend/internal/models/proxy_host.go`
  - `DomainNames string 'gorm:"not null;index"'`

Key observations:
1. Handler `Update` applies payload field directly:
   - if `domain_names` present and string, assigns `host.DomainNames = v` even when `v == ""`.
2. Service validation does not reject empty `domain_names`:
   - `ValidateUniqueDomain` checks duplicates only.
   - `validateProxyHost` validates `forward_host`, not `domain_names`.
3. Model uses `not null`, which allows empty string in SQLite.

Likely direct cause of observed `''`:
- Empty domain is accepted and persisted.

### 2.3 Transaction semantics findings

Files/functions inspected:
- `backend/internal/database/database.go`
  - `gorm.Config{ SkipDefaultTransaction: true }`
- `backend/internal/api/handlers/proxy_host_handler.go`
  - `Update` calls `service.Update(host)` then `caddyManager.ApplyConfig(...)`

Key observations:
1. No explicit transaction wraps validation + update + config-apply in `Update`.
2. If persistence succeeds but Caddy apply fails, request may fail after DB mutation (no rollback).
3. Scenario label says “Failed transaction”, but code path is not transactional in this route.

### 2.4 Frontend contract context (for API/UI parity)

Files/components inspected:
- `frontend/src/components/ProxyHostForm.tsx`
  - Domain input has `required` on client-side form.
- `frontend/src/api/proxyHosts.ts`
  - `updateProxyHost(uuid, host)` forwards payload directly.
- `frontend/src/pages/ProxyHosts.tsx`

Observation:
- UI form prevents blank input for normal interactions, but API path still allows blank when called directly (as in test).

### 2.5 Confidence

Confidence score: 95%

Rationale:
- Failure output and inspected code paths align directly with persisted empty domain.
- Missing domain validation and non-transactional update semantics are explicit in code.

---

## 3) Root Cause Hypothesis (Primary)

Primary root cause:
- Backend update contract allows `domain_names: ''` and persists it.

Contributing factors:
- Test allows `200` on invalid update attempt, reducing strictness of expected failure contract.
- Route semantics are not transactional despite scenario wording.

Non-cause (for this failure):
- UI rendering logic is not primary here; final assertion reads API detail directly.

---

## 4) EARS Requirements (Focused)

- WHEN a proxy host update request includes `domain_names` as an empty string, THE SYSTEM SHALL reject the request with validation error (`400` or `422`).
- WHEN validation rejects that request, THE SYSTEM SHALL NOT persist any change to `domain_names`.
- WHEN the invalid update is attempted in the failing scenario, THE SYSTEM SHALL preserve the previously stored domain value.
- IF update processing fails after persistence-related steps, THEN THE SYSTEM SHALL provide deterministic behavior that prevents partial state for this field path.
- WHEN this failure is fixed, THE SYSTEM SHALL pass the targeted Playwright scenario deterministically across repeated runs.

### Decision - 2026-02-15
**Decision**: For this single flaky-failure fix scope, the Supervisor blocker on post-persistence failure behavior is closed by enforcing pre-persistence validation rejection for empty `domain_names` and treating this as the required failure path for the targeted scenario.
**Context**: The failing scenario (`Failed transaction prevents partial data updates`) currently reproduces because `domain_names: ''` is accepted and persisted. The route does not implement an explicit transaction boundary for persistence + post-persistence Caddy apply, so broad rollback semantics are a separate concern.
**Options**:
- **Option A (selected):** Enforce validation rejection before persistence for empty-domain updates in this scope.
- **Option B (deferred):** Implement broader transactional rollback hardening for post-persistence failures in the update flow.
**Rationale**: The reported flaky failure is fully addressed by preventing invalid writes before persistence, which directly satisfies the scenario’s failure expectation with the smallest safe change surface. Expanding into transaction rollback hardening would increase scope and coupling for this fix and is not required to close the current blocker.
**Impact**: The targeted failure path is deterministic for this issue. Broader post-persistence rollback guarantees remain unchanged and are explicitly deferred as follow-up technical hardening.
**Review**: Reassess deferred rollback hardening in a dedicated follow-up item after this flaky-failure fix lands and stabilizes.

### Supervisor Blocker Closure Note
- **Status**: Closed for this issue scope.
- **Closure basis**: Enforced validation rejection path (empty domain rejected pre-persistence) is the approved failure mechanism for this scenario.
- **Deferred follow-up**: Broader transaction rollback hardening for post-persistence failures is out-of-scope for this fix and tracked as subsequent hardening work.

---

## 5) Remediation Plan (Phased)

### Phase 1: Contract lock and deterministic repro

Goal:
- Reproduce only this failure and lock expected contract.

Actions:
1. Run targeted Playwright case only:
   - `tests/core/data-consistency.spec.ts` test: `Failed transaction prevents partial data updates`.
2. Capture request/response for:
   - `PUT /api/v1/proxy-hosts/:uuid` invalid payload.
   - `GET /api/v1/proxy-hosts/:uuid` verification read.
3. Record whether update returns `200` and whether persisted value is empty.

Validation output:
- Baseline artifact proving failure prior to fix.

### Phase 2: Backend validation fix (primary path)

Goal:
- Enforce non-empty domain update contract at API boundary/service.

Files/functions to update (implementation phase reference):
- `backend/internal/api/handlers/proxy_host_handler.go`
  - `Update`
- `backend/internal/services/proxyhost_service.go`
  - `Update` and/or validation helper path

Design constraints:
1. Reject empty/whitespace-only `domain_names` when provided in update payload.
2. Preserve partial-update behavior for omitted fields.
3. Keep current response schema; return clear validation error.

Validation tests to add/update:
- `backend/internal/api/handlers/proxy_host_handler_update_test.go`
  - Add case: empty `domain_names` returns `400/422` and DB value unchanged.
- `backend/internal/services/proxyhost_service_validation_test.go`
  - Add case: domain validation rejects empty string on update path.

### Phase 3: Transaction semantics hardening decision (narrow)

Goal:
- Resolve mismatch between scenario intent (“failed transaction”) and current route behavior.

Decision gate:
- For this specific failure, validation fix is mandatory and sufficient to close the current Supervisor blocker.
- Broader transactional rollback hardening is explicitly deferred follow-up work and is not part of this issue scope.

Files/functions to inspect for conditional hardening:
- `backend/internal/api/handlers/proxy_host_handler.go`
  - `Update` sequence (`service.Update` then `caddyManager.ApplyConfig`).
- `backend/internal/database/database.go`
  - `SkipDefaultTransaction` implications.

Deferred hardening follow-up (out-of-scope for this fix):
- Evaluate wrapping update path in an explicit transaction for persistence + post-persistence operations, or introduce compensating rollback strategy on downstream failure where feasible.
- Produce a dedicated implementation spec for rollback hardening to avoid coupling with this flaky-failure patch.
- Record explicit risk acceptance for this issue: post-persistence failure rollback behavior remains unchanged until follow-up lands.

### Phase 4: Test contract tightening (only after product fix)

Goal:
- Align E2E assertion with backend contract.

File:
- `tests/core/data-consistency.spec.ts`

Allowed change (conditional):
- Narrow accepted status list for invalid update from `[200, 400, 422]` to failure-only status set, but only after backend fix is in place and confirmed.

Not allowed:
- Masking failure by weakening final unchanged-data assertion.

### Phase 5: Deterministic validation sequence (minimal requests)

Goal:
- Confirm fix with low-noise, repeatable verification.

Sequence:
1. Run backend targeted tests only (new/updated handler + service tests).
2. Run single Playwright failing scenario.
3. Re-run the same scenario 2 additional times.

Deterministic pass criterion:
- 3/3 pass for the targeted scenario.

Request minimization guidance for scenario validation:
- Keep one create request, one invalid update request, one detail read request per attempt.
- Avoid unrelated navigation or suite-wide runs until this scenario is stable.

### Phase 6: Coverage gate and Codecov patch triage closure (mandatory)

Goal:
- Close CI coverage blocker for this flaky-failure fix with explicit, auditable evidence.

Required actions:
1. Run focused coverage on only modified backend/frontend test targets for this failure fix.
2. Open Codecov Patch view for the PR/commit and copy exact uncovered or partial ranges.
3. Record ranges in this plan using exact file + line spans from Patch view (no paraphrase).
4. Add targeted tests that execute each missing/partial modified line.
5. Re-run coverage and confirm Patch view reports 100% coverage for modified lines.

Patch-line triage record (must be filled during execution):
- Source: Codecov Patch view
- Missing ranges: `<file>#Lx-Ly`, `<file>#Lx`, ...
- Partial ranges: `<file>#Lx-Ly`, `<file>#Lx`, ...
- Targeted tests added: `<test file>::<test name>` mapped to each range above
- Closure evidence: Patch view status = 100% for modified lines

Hard gate:
- Do not mark this spec complete while any missing/partial modified range remains open in Codecov Patch view.

---

## 6) Guardrails: When test edits are allowed

### Test edits are NOT allowed when:
1. Backend currently accepts invalid domain payload (`''`) or persists it.
2. No backend validation test exists for empty-domain rejection.
3. Root-cause behavior is unfixed in handler/service.

Required action in this state:
- Fix backend/frontend product behavior first (backend for this failure).

### Test edits are allowed only when ALL are true:
1. Backend rejects empty `domain_names` deterministically.
2. Backend unit tests verify unchanged persisted value on invalid update.
3. Targeted Playwright scenario passes without weakening final assertion.

Allowed scope of test edits:
- Tighten status-code expectation and reduce nondeterministic waits.
- No assertion downgrades that hide data-integrity regressions.

---

## 7) Exact Files/Functions/Components to Inspect During Execution

Primary:
- `tests/core/data-consistency.spec.ts`
  - test `Failed transaction prevents partial data updates`
- `backend/internal/api/handlers/proxy_host_handler.go`
  - `Update`
- `backend/internal/services/proxyhost_service.go`
  - `Update`, `ValidateUniqueDomain`, `validateProxyHost`
- `backend/internal/models/proxy_host.go`
  - `DomainNames` field constraints

Supporting:
- `backend/internal/api/handlers/proxy_host_handler_update_test.go`
- `backend/internal/services/proxyhost_service_validation_test.go`
- `backend/internal/database/database.go`
- `frontend/src/components/ProxyHostForm.tsx`
- `frontend/src/api/proxyHosts.ts`
- `frontend/src/pages/ProxyHosts.tsx`

---

## 8) Acceptance Criteria (Failure-Scoped)

1. The targeted Playwright scenario no longer returns empty domain after invalid update attempt.
2. Invalid payload `domain_names: ''` is rejected by backend (`400` or `422`).
3. Persisted domain value remains original after rejection.
4. Targeted backend tests cover both rejection and no-partial-update behavior.
5. Deterministic validation passes (3 repeated targeted Playwright runs).
6. Coverage gate compliance is demonstrated for this fix scope (project threshold met and Patch view validated).
7. Codecov Patch view shows 100% patch coverage for all modified lines in this fix.
8. Exact missing/partial patch ranges (if any appeared) are captured and closed via targeted tests before completion.
9. No unrelated feature/test changes are introduced.

---

## 9) Out-of-Scope (Strict)

- DNS provider flows
- Manual DNS challenge flows
- Bulk security header update transaction path (except reference for transaction pattern only)
- Broad transaction rollback hardening for post-persistence failures in proxy-host update flow (deferred follow-up)
- Broad Playwright suite stabilization unrelated to this single failure

---

## 10) Codecov Patch-Line Closure Protocol (Execution Checklist)

This checklist is required for this flaky failure only and must be completed before handoff.

1. Capture exact Patch view gaps:
  - Copy each missing/partial modified range exactly as displayed in Codecov Patch view.
2. Document triage table in execution notes:
  - `Range` | `Why not covered` | `Targeted test` | `Status`.
3. Implement only targeted tests needed to cover listed ranges.
4. Re-run focused coverage and refresh Codecov Patch view.
5. Repeat triage until all listed ranges are closed and Patch coverage is 100%.

Mandatory closure condition:
- Every modified line in this fix is green in Codecov Patch view with no remaining missing/partial ranges.
