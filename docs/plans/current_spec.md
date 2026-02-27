# ACL + Security Headers Hotfix Plan (Proxy Host Create/Edit)

## 1. Introduction

### Overview
Hotfix request: Proxy Host form dropdown selections for Access Control List (ACL) and Security Headers are not being applied/persisted for new or edited hosts.

Reported behavior:
1. Existing hosts with previously assigned ACL/Security Header profile retain old values.
2. Users cannot reliably remove or change those values in UI.
3. Newly created hosts cannot reliably apply ACL/Security Header profile.

### Objective
Deliver an urgent but correct root-cause fix across frontend binding and backend persistence flow, with minimum user interruption and full validation gates.

## 2. Research Findings (Current Architecture + Touchpoints)

### Frontend Entry Points
1. `frontend/src/pages/ProxyHosts.tsx`
   - `handleSubmit(data)` calls `updateHost(editingHost.uuid, data)` or `createHost(data)`.
   - Renders `ProxyHostForm` modal for create/edit flows.
2. `frontend/src/components/ProxyHostForm.tsx`
   - Local form state initializes `access_list_id` and `security_header_profile_id`.
   - ACL control uses `AccessListSelector`.
   - Security Headers control uses `Select` with `security_header_profile_id` mapping.
   - Submission path: `handleSubmit` -> `onSubmit(payloadWithoutUptime)`.
3. `frontend/src/components/AccessListSelector.tsx`
   - Converts select values between `string` and `number | null`.

### Frontend API/Hooks
1. `frontend/src/hooks/useProxyHosts.ts`
   - `createHost` -> `createProxyHost`.
   - `updateHost` -> `updateProxyHost`.
2. `frontend/src/api/proxyHosts.ts`
   - `createProxyHost(host: Partial<ProxyHost>)` -> `POST /api/v1/proxy-hosts`.
   - `updateProxyHost(uuid, host)` -> `PUT /api/v1/proxy-hosts/:uuid`.
   - Contract fields: `access_list_id`, `security_header_profile_id`.

### Backend Entry/Transformation/Persistence
1. Route registration
   - `backend/internal/api/routes/routes.go`: `proxyHostHandler.RegisterRoutes(protected)`.
2. Handler
   - `backend/internal/api/handlers/proxy_host_handler.go`
   - `Create(c)` uses `ShouldBindJSON(&models.ProxyHost{})`.
   - `Update(c)` uses `map[string]any` partial update parsing.
   - Target fields:
     - `payload["access_list_id"]` -> `parseNullableUintField` -> `host.AccessListID`
     - `payload["security_header_profile_id"]` -> typed conversion -> `host.SecurityHeaderProfileID`
3. Service
   - `backend/internal/services/proxyhost_service.go`
   - `Create(host)` validates + `db.Create(host)`.
   - `Update(host)` validates + `db.Model(...).Select("*").Updates(host)`.
4. Model
   - `backend/internal/models/proxy_host.go`
   - `AccessListID *uint \`json:"access_list_id"\``
   - `SecurityHeaderProfileID *uint \`json:"security_header_profile_id"\``

### Existing Tests Relevant to Incident
1. Frontend unit regression coverage already exists:
   - `frontend/src/components/__tests__/ProxyHostForm-dropdown-changes.test.tsx`
2. E2E regression spec exists:
   - `tests/proxy-host-dropdown-fix.spec.ts`
3. Backend update and security-header tests exist:
   - `backend/internal/api/handlers/proxy_host_handler_update_test.go`
   - `backend/internal/api/handlers/proxy_host_handler_security_headers_test.go`

## 3. Root-Cause-First Trace

### Trace Model (Mandatory)
1. Entry Point:
   - UI dropdown interactions in `ProxyHostForm` and `AccessListSelector`.
2. Transformation:
   - Form state conversion (`string` <-> `number | null`) and payload construction in `ProxyHostForm`.
   - API serialization via `frontend/src/api/proxyHosts.ts`.
3. Persistence:
   - Backend `Update` parser (`proxy_host_handler.go`) and `ProxyHostService.Update` persistence.
4. Exit Point:
   - Response body consumed by React Query invalidation/refetch in `useProxyHosts`.
   - UI reflects updated values in table/form.

### Most Likely Failure Zones
1. Frontend select binding/conversion drift (top candidate)
   - Shared symptom across ACL and Security Headers points to form/select layer.
   - Candidate files:
     - `frontend/src/components/ProxyHostForm.tsx`
     - `frontend/src/components/AccessListSelector.tsx`
     - `frontend/src/components/ui/Select.tsx`
2. Payload mutation or stale form object behavior
   - Ensure payload carries updated `access_list_id` / `security_header_profile_id` values at submit time.
3. Backend partial-update parser edge behavior
   - Ensure `nil`, numeric string, and number conversions are consistent between ACL and security header profile paths.

### Investigation Decision
Root-cause verification will be instrumented through failing-first Playwright scenario and targeted handler tests before applying code changes.

## 4. EARS Requirements

1. WHEN a user selects an ACL in the Proxy Host create/edit form, THE SYSTEM SHALL persist `access_list_id` and return it in API response.
2. WHEN a user changes ACL from one value to another, THE SYSTEM SHALL replace prior `access_list_id` with the new value.
3. WHEN a user selects "No Access Control", THE SYSTEM SHALL persist `access_list_id = null`.
4. WHEN a user selects a Security Headers profile in the Proxy Host create/edit form, THE SYSTEM SHALL persist `security_header_profile_id` and return it in API response.
5. WHEN a user changes Security Headers profile from one value to another, THE SYSTEM SHALL replace prior `security_header_profile_id` with the new value.
6. WHEN a user selects "None" for Security Headers, THE SYSTEM SHALL persist `security_header_profile_id = null`.
7. IF dropdown interaction fails to update internal form state, THEN THE SYSTEM SHALL prevent stale values from being persisted.
8. WHILE updating Proxy Host settings, THE SYSTEM SHALL maintain existing behavior for unrelated fields and not regress certificate, DNS challenge, or uptime-linked updates.

Note: User-visible blocking error behavior is deferred unless required by confirmed root cause.

## 5. Technical Specification (Hotfix Scope)

### API Contract (No Breaking Change)
1. `POST /api/v1/proxy-hosts`
   - Request fields include `access_list_id`, `security_header_profile_id` as nullable numeric fields.
2. `PUT /api/v1/proxy-hosts/:uuid`
   - Partial payload accepts nullable updates for both fields.
3. Response must echo persisted values in snake_case:
   - `access_list_id`
   - `security_header_profile_id`

### Data Model/DB
No schema migration expected. Existing nullable FK fields in `backend/internal/models/proxy_host.go` are sufficient.

### Targeted Code Areas for Fix
1. Frontend
   - `frontend/src/components/ProxyHostForm.tsx`
   - `frontend/src/components/AccessListSelector.tsx`
   - `frontend/src/components/ui/Select.tsx` (only if click/select propagation issue confirmed)
   - `frontend/src/api/proxyHosts.ts` (only if serialization issue confirmed)
2. Backend
   - `backend/internal/api/handlers/proxy_host_handler.go` (only if parsing/persistence mismatch confirmed)
   - `backend/internal/services/proxyhost_service.go` (only if update write path proves incorrect)

## 6. Edge Cases

1. Edit host with existing ACL/profile and switch to another value.
2. Edit host with existing ACL/profile and clear to null.
3. Create new host with ACL/profile set before first save.
4. Submit with stringified numeric values (defensive compatibility).
5. Submit with null values for both fields simultaneously.
6. Missing/deleted profile or ACL IDs in backend (validation errors).
7. Multiple rapid dropdown changes before save (last selection wins).

## 7. Risk Analysis

### High Risk
1. Silent stale-state submission from form controls.
2. Regressing other Proxy Host settings due to broad payload mutation.

### Medium Risk
1. Partial-update parser divergence between ACL and security profile behavior.
2. UI select portal/z-index interaction causing non-deterministic click handling.

### Mitigations
1. Reproduce with Playwright first and capture exact failing action path.
2. Add/strengthen focused frontend tests around create/edit/clear flows.
3. Add/strengthen backend tests for nullable + conversion paths.
4. Keep hotfix minimal and avoid unrelated refactors.

## 8. Implementation Plan (Urgent, Minimal Interruption)

### Phase 1: Reproduction + Guardrails (Playwright First)
1. Execute targeted E2E spec for dropdown flow and create/edit persistence behavior.
2. Capture exact failure step and confirm whether failure is click binding, payload value, or backend persistence.
3. Add/adjust failing-first test if current suite does not capture observed production regression.

### Phase 2: Frontend Fix
1. Patch select binding/state mapping for ACL and Security Headers in `ProxyHostForm`/`AccessListSelector`.
2. If needed, patch `ui/Select` interaction layering.
3. Ensure payload contains correct final `access_list_id` and `security_header_profile_id` values at submit.
4. Extend `ProxyHostForm` tests for create/edit/change/remove flows.

### Phase 3: Backend Hardening (Conditional)
1. Only if frontend payload is correct but persistence is wrong:
   - Backend fix MUST use field-scoped partial-update semantics for `access_list_id` and `security_header_profile_id` only (unless separately justified).
   - Ensure write path persists null transitions reliably.
2. Add/adjust handler/service regression tests proving no unintended mutation of unrelated proxy host fields during these targeted updates.

### Phase 4: Integration + Regression
1. Run complete targeted Proxy Host UI flow tests.
2. Validate list refresh and modal reopen reflect persisted values.
3. Validate no regressions in bulk ACL / bulk security-header operations.

### Phase 5: Documentation + Handoff
1. Update changelog/release notes only for hotfix behavior.
2. Keep architecture docs unchanged unless root cause requires architectural note.
3. Handoff to Supervisor agent for review after plan approval and implementation.

## 9. Acceptance Criteria

1. ACL dropdown selection persists on create and edit.
2. Security Headers dropdown selection persists on create and edit.
3. Clearing ACL persists `null` and is reflected after reload.
4. Clearing Security Headers persists `null` and is reflected after reload.
5. Existing hosts can change from one ACL/profile to another without stale value retention.
6. New hosts can apply ACL/profile at creation time.
7. No regressions in unrelated proxy host fields.
8. All validation gates in Section 11 pass.
9. API create response returns persisted `access_list_id` and `security_header_profile_id` matching submitted values (including `null`).
10. API update response returns persisted `access_list_id` and `security_header_profile_id` after `value->value`, `value->null`, and `null->value` transitions.
11. Backend persistence verification confirms unrelated proxy host fields remain unchanged for targeted updates.

## 10. PR Slicing Strategy

### Decision
Single PR (hotfix-first), with contingency split only if backend root cause is confirmed late.

### Rationale
1. Incident impact is immediate user-facing and concentrated in one feature path.
2. Frontend + targeted backend/test changes are tightly coupled for verification.
3. Single PR minimizes release coordination and user interruption.

### Contingency (Only if split becomes necessary)
1. PR-1: Frontend binding + tests
   - Scope: `ProxyHostForm`, `AccessListSelector`, `ui/Select` (if required), related tests.
   - Dependency: none.
   - Acceptance: UI submit payload verified correct in unit + Playwright.
2. PR-2: Backend parser/persistence + tests (conditional)
   - Scope: `proxy_host_handler.go`, `proxyhost_service.go`, handler/service tests.
   - Dependency: PR-1 merged or rebased for aligned contract.
   - Acceptance: API update/create persist both nullable IDs correctly.
3. PR-3: Regression hardening + docs
   - Scope: extra regression coverage, release-note hotfix entry.
   - Dependency: PR-1/PR-2.
   - Acceptance: full DoD validation sequence passes.

## 11. Validation Plan (Mandatory Sequence)

0. E2E environment prerequisite
   - Determine rebuild necessity per testing policy: if application/runtime or Docker input changes are present, rebuild is required.
   - If rebuild is required or the container is unhealthy, run `.github/skills/scripts/skill-runner.sh docker-rebuild-e2e`.
   - Record container health outcome before executing tests.
1. Playwright first
   - Run targeted Proxy Host dropdown and create/edit persistence scenarios.
2. Local patch coverage preflight
   - Generate `test-results/local-patch-report.md` and `test-results/local-patch-report.json`.
3. Unit and coverage
   - Backend coverage run (threshold >= 85%).
   - Frontend coverage run (threshold >= 85%).
4. Type checks
   - Frontend TypeScript check.
5. Pre-commit
   - `pre-commit run --all-files` with zero blocking failures.
6. Security scans
   - CodeQL Go + JS (security-and-quality).
   - Findings check gate.
   - Trivy scan.
   - Conditional GORM security scan if model/DB-layer changes are made.
7. Build verification
   - Backend build + frontend build pass.

## 12. File Review: `.gitignore`, `codecov.yml`, `.dockerignore`, `Dockerfile`

Assessment for this hotfix:
1. `.gitignore`: no required change for ACL/Security Headers hotfix.
2. `codecov.yml`: no required change; current exclusions/thresholds are compatible.
3. `.dockerignore`: no required change unless new hotfix-only artifact paths are introduced.
4. `Dockerfile`: no required change; incident is application logic/UI binding, not image build pipeline.

If implementation introduces new persistent test artifacts, update ignore files in the same PR.

## 13. Rollback and Contingency

1. If hotfix causes regression in proxy host save flow, revert hotfix commit and redeploy prior stable build.
2. If frontend-only fix is insufficient, activate conditional backend phase immediately.
3. If validation gates fail on security/coverage, hold merge until fixed; no partial exception for this incident.
4. Post-rollback smoke checks:
   - Create host with ACL/profile.
   - Edit to different ACL/profile values.
   - Clear both values to `null`.
   - Verify persisted values in API response and after UI reload.
