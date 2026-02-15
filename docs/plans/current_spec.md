## DNS Providers E2E Failure Recovery Spec

Date: 2026-02-15
Owner: Planning Agent
Target file: docs/plans/current_spec.md

---

## 1) Introduction

This specification addresses two failing Playwright Firefox tests in tests/core/domain-dns-management.spec.ts:

1. DNS Providers - list providers after API seed
2. DNS Providers - delete provider via API and verify removal

Observed failures are deterministic across retries and happen before deletion assertions. The plan below focuses on root-cause correction with minimal API request overhead and stable UI semantics.

Primary objective:
- Make DNS provider cards discoverable and testable again without regressing Manual DNS Challenge behavior.

Secondary objective:
- Reduce unnecessary DNS page requests while preserving UX and accessibility behavior.

---

## 2) Research Findings

### 2.1 Confirmed failing assertions

From targeted run:
- tests/core/domain-dns-management.spec.ts:122
  - waitForResourceInUI fails after 15s for namespaced provider name.
- tests/core/domain-dns-management.spec.ts:166
  - expect(providerCard).toBeVisible() fails, element not found.

Both fail before provider deletion verification; both retries fail identically.

### 2.2 Root-cause path (entry -> transform -> render)

Entry path:
- Test fixture creates providers via TestDataManager.createDNSProvider in tests/utils/TestDataManager.ts.
- Backend accepts and persists via DNSProviderHandler.Create in backend/internal/api/handlers/dns_provider_handler.go.

Transform path:
- DNSProviders page fetches provider list through useDNSProviders -> getDNSProviders.
- File chain:
  - frontend/src/pages/DNSProviders.tsx
  - frontend/src/hooks/useDNSProviders.ts
  - frontend/src/api/dnsProviders.ts

Render path:
- In DNSProviders.tsx, showManualChallenge is derived from manualChallenge state.
- loadManualChallenge calls getChallenge(providerId, active).
- On any error, loadManualChallenge currently sets a fallback challenge object.
- Because fallback always sets manualChallenge, showManualChallenge becomes true.
- Provider cards grid is gated by !showManualChallenge and providers.length > 0.
- Result: provider cards are suppressed even when providers exist.

Persistence path validation:
- Backend list/delete routes are present and correct when encryption key is configured:
  - protected.GET /dns-providers
  - protected.DELETE /dns-providers/:id
  in backend/internal/api/routes/routes.go.
- DNS provider service List/Delete implementations do not explain the UI invisibility.

### 2.3 Additional architectural observations

- Manual challenge endpoint GET /dns-providers/:id/manual-challenge/:challengeId returns 404 when challenge is missing.
- Frontend passes challengeId active as a synthetic ID, but backend has no explicit active alias route.
- Manual challenge UI has comprehensive component tests in frontend/src/components/__tests__/ManualDNSChallenge.test.tsx.
- DNS providers page has no dedicated unit test for coexistence rules between provider cards and manual challenge panel.

### 2.4 Confidence score

Confidence: 93% (high)

Rationale:
- Failure output, snapshots, and rendering condition align.
- Issue reproduces across retries.
- Backend CRUD path appears healthy; failure is in frontend display gating.

---

## 3) Technical Specifications

## 3.1 EARS requirements

- WHEN the DNS Providers page loads and no active manual challenge exists, THE SYSTEM SHALL render provider cards from GET /api/v1/dns-providers.
- WHEN provider cards exist, THE SYSTEM SHALL keep them visible regardless of manual challenge fetch failures.
- WHEN stabilizing the current failure, THE SYSTEM SHALL apply frontend root-cause fixes before any E2E test edits.
- IF the same E2E assertion still fails after the frontend fix is applied, THEN THE SYSTEM SHALL permit minimal, targeted test adjustments.
- IF manual challenge retrieval returns not found, THEN THE SYSTEM SHALL treat it as no active challenge and SHALL NOT inject fallback challenge content.
- WHEN a person explicitly requests Manual DNS Challenge view, THE SYSTEM SHALL fetch/display challenge data and SHALL preserve keyboard/screen-reader semantics.
- WHEN a provider is created via API seed in tests, THE SYSTEM SHALL expose it in UI heading text within the provider card grid.
- WHEN a provider is deleted via API and page is refreshed, THE SYSTEM SHALL remove the matching card from the grid.

## 3.2 Request-minimization strategy (least amount of requests)

Current behavior issues:
- DNS page performs challenge fetch eagerly and converts errors into fallback UI state.

Planned behavior:
- One baseline request on page load: GET /api/v1/dns-providers.
- Zero manual challenge requests on initial load unless manual challenge panel is opened explicitly.
- Optional follow-up requests only on user action:
  - GET /api/v1/dns-providers/:id/manual-challenge/active (new optional endpoint), or
  - GET /api/v1/dns-providers/:id/manual-challenge/:challengeId only if challengeId is known.

Net effect:
- Fewer initial requests and deterministic provider list rendering.

## 3.3 API and handler design options

Option A (preferred for least backend change):
- Frontend-only behavior correction.
- In frontend/src/pages/DNSProviders.tsx:
  - Remove synthetic fallback challenge injection from catch block in loadManualChallenge.
  - Track manual panel visibility separately from challenge data.
  - Only call loadManualChallenge from manual action button or explicit deep-link flow.

Option B (clean API contract enhancement):
- Add explicit active challenge endpoint:
  - GET /api/v1/dns-providers/:id/manual-challenge/active
- Handler addition in backend/internal/api/handlers/manual_challenge_handler.go:
  - GetActiveChallenge(c *gin.Context)
- Service addition in backend/internal/services/manual_challenge_service.go:
  - GetLatestActiveChallengeForProvider(ctx, providerID, userID)
- Route registration in backend/internal/api/routes/routes.go.

Recommendation:
- Implement Option A first (fastest unblock).
- Option B can follow if active-challenge UX remains core and reused broadly.

## 3.4 Component-level design changes

Primary component:
- frontend/src/pages/DNSProviders.tsx

Current critical symbols:
- loadManualChallenge
- showManualChallenge
- manualChallenge
- manualProviderId

Planned symbol responsibilities:
- isManualPanelOpen (new): controls panel visibility as explicit UI state.
- manualChallenge: nullable real challenge only (no synthetic fallback).
- loadManualChallenge(providerId):
  - on 404/not-found, set manualChallenge = null and keep provider cards visible.
  - on transport/server errors, surface toast warning without replacing page mode.

Render rules:
- Provider grid visibility should not be blocked by challenge lookup failure.
- Manual panel visibility must be driven by `isManualPanelOpen`, not by presence/absence of `manualChallenge`.
- `manualChallenge` data existence must not implicitly switch overall page mode.

Accessibility continuity:
- Preserve existing button names and aria labeling in ManualDNSChallenge.
- Ensure focus flow remains predictable when opening/closing panel.

## 3.5 Test strategy updates

E2E tests to stabilize:
- tests/core/domain-dns-management.spec.ts

E2E test-edit guard:
- Do not edit failing E2E tests during initial fix.
- Only after frontend fix attempt, if the same failure still reproduces deterministically, allow minimal assertion/wait adjustments limited to the failing steps.

Related E2E tests for regression guard:
- tests/dns-provider-crud.spec.ts
- tests/manual-dns-provider.spec.ts
- tests/dns-provider-types.spec.ts

Frontend unit test additions (new):
- frontend/src/pages/__tests__/DNSProviders.test.tsx (new file)
  - Case 1: providers render when manual challenge fetch returns 404.
  - Case 2: manual panel opens only after explicit button action.
  - Case 3: provider grid remains visible after manual challenge fetch error.

Backend unit tests (only if Option B implemented):
- backend/internal/api/handlers/manual_challenge_handler_test.go
- backend/internal/services/manual_challenge_service_test.go

---

## 4) Implementation Plan (Phased, minimal-request first)

## Phase 1 - Reproduction and baseline lock

Goal:
- Lock failing behavior and ensure deterministic reproduction path.

Actions:
1. Run targeted failing tests only in tests/core/domain-dns-management.spec.ts.
2. Capture failure screenshots, traces, and assertions.
3. Capture request timeline for /api/v1/dns-providers and manual challenge endpoints.

Expected output:
- Baseline artifact bundle confirming pre-fix failure.

## Phase 2 - Frontend display mode correction (least requests)

Goal:
- Ensure provider list renders independently from manual challenge fetch.

Files:
- frontend/src/pages/DNSProviders.tsx

Actions:
1. Decouple panel visibility from challenge fetch side effects.
2. Remove synthetic fallback challenge creation on fetch error.
3. Make manual challenge fetch opt-in (button-driven).
4. Keep provider cards visible by default after provider list fetch.

Expected output:
- The two failing tests can locate seeded provider cards.

## Phase 3 - Test hardening and contract coverage

Goal:
- Prevent regressions in DNS page state machine.

Files:
- tests/core/domain-dns-management.spec.ts (assertion timing refinements only if needed)
- frontend/src/pages/__tests__/DNSProviders.test.tsx (new)

Actions:
1. Add unit tests for no-active-challenge behavior.
2. Verify manual challenge visibility toggle logic.
3. Keep Playwright assertions role-based and deterministic.
4. Do not modify existing failing E2E assertions in this phase unless post-fix deterministic failure persists.

Expected output:
- Stable UI contract for providers + manual challenge coexistence.

## Phase 4 - Backend/API enhancement (deferred by default)

Goal:
- Normalize active challenge retrieval contract, reduce 404 control-flow usage.

Scope rule:
- Out of scope for this spec by default.
- Enter this phase only if the frontend-only fix fails to resolve the two target failures after deterministic re-runs.

Files:
- backend/internal/api/handlers/manual_challenge_handler.go
- backend/internal/services/manual_challenge_service.go
- backend/internal/api/routes/routes.go
- backend/internal/api/handlers/manual_challenge_handler_test.go
- backend/internal/services/manual_challenge_service_test.go

Actions:
1. Add explicit active challenge endpoint.
2. Return 204 or structured null payload for no-active state.
3. Update frontend manual flow to consume explicit endpoint.

Expected output:
- Cleaner API semantics and fewer error-as-control-flow branches.

## Phase 5 - Validation and CI gates

Goal:
- Validate functionality, patch coverage, and security checks.

Actions:
1. Run exactly the two failing Firefox tests in tests/core/domain-dns-management.spec.ts.
2. Repeat the same two tests to confirm stability (deterministic pass/fail behavior).
3. Run one focused manual DNS regression: tests/manual-dns-provider.spec.ts.
4. Broaden beyond this set only if failures remain unresolved after steps 1-3.
5. Run additional coverage/security gates per repo policy only after deterministic validation set is complete.

Expected output:
- Green E2E for failing scenarios, no regressions in DNS/manual challenge suites.

---

## 5) Risk and Edge Case Matrix

1. Risk: Manual challenge panel no longer appears automatically for valid active challenges.
   - Mitigation: explicit open action plus optional active endpoint check.

2. Risk: Existing manual-dns-provider tests assume automatic panel visibility.
   - Mitigation: update tests to trigger panel intentionally via Manual DNS Challenge button.

3. Risk: Provider card heading selector changes break tests.
   - Mitigation: keep DNSProviderCard title semantics stable in frontend/src/components/DNSProviderCard.tsx.

4. Risk: Encryption key not configured in environment suppresses DNS routes.
   - Mitigation: confirm CHARON_ENCRYPTION_KEY presence in test runtime before asserting provider flows.

---

## 6) File-by-File Change Map

Planned edits (high likelihood):
- frontend/src/pages/DNSProviders.tsx
- frontend/src/pages/__tests__/DNSProviders.test.tsx (new)
- tests/core/domain-dns-management.spec.ts (small sync updates only if required)

Conditional edits (if API enhancement chosen):
- backend/internal/api/handlers/manual_challenge_handler.go
- backend/internal/services/manual_challenge_service.go
- backend/internal/api/routes/routes.go
- backend/internal/api/handlers/manual_challenge_handler_test.go
- backend/internal/services/manual_challenge_service_test.go

No expected edits for root fix:
- backend/internal/api/handlers/dns_provider_handler.go
- backend/internal/services/dns_provider_service.go

---

## 7) Deferred Scope Notes

- Unrelated root-level configuration file review is deferred and removed from this spec scope.
- Any .gitignore, codecov.yml, .dockerignore, or Dockerfile review must be handled in a separate dedicated plan.

---

## 8) Acceptance Criteria

Functional:
- The two previously failing tests in tests/core/domain-dns-management.spec.ts pass in Firefox:
  - DNS Providers - list providers after API seed
  - DNS Providers - delete provider via API and verify removal

Behavioral:
- DNS provider cards are visible after API seed on /dns/providers.
- Provider card visibility no longer depends on fallback manual challenge state.
- Manual challenge panel appears only under explicit trigger or real active challenge state.

Quality:
- No regression in tests/manual-dns-provider.spec.ts and tests/dns-provider-crud.spec.ts.
- Added or updated unit tests cover DNSProviders page state transitions.
- Validation sequence follows minimal deterministic order: two failing Firefox tests, repeat run, then one manual DNS regression test.
- No backend/API enhancement work is executed unless frontend fix attempt fails and failure still reproduces.

Coverage and gate alignment:
- Patch coverage remains 100% for modified lines.
- Existing project coverage thresholds remain satisfied.

---

## 9) Handoff to Supervisor

Supervisor review focus:
1. Verify root-cause alignment: UI state gating vs backend CRUD.
2. Confirm minimal-request architecture in DNSProviders page flow.
3. Confirm manual panel state is explicit UI state and not inferred from challenge object presence.
4. Confirm no hidden regressions in manual challenge UX and accessibility semantics.
5. Approve frontend-first execution, with backend/API phase only if frontend fix fails.
