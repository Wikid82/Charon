---
post_title: "Current Spec: Notify HTTP Wrapper Rollout for Gotify and Custom Webhook"
categories:
  - actions
  - backend
  - frontend
  - testing
  - security
tags:
  - notify-migration
  - gotify
  - webhook
  - playwright
  - patch-coverage
summary: "Single authoritative plan for Notify HTTP wrapper rollout for Gotify and Custom Webhook, including token secrecy contract, SSRF hardening, transport safety, expanded test matrix, and safe PR slicing."
post_date: 2026-02-23
---

## Active Plan: Notify Migration — HTTP Wrapper for Gotify and Custom Webhook

Date: 2026-02-23
Status: Ready for Supervisor Review
Scope Type: Backend + Frontend + E2E + Coverage/CI alignment
Authority: This is the only active authoritative plan in this file.

## Introduction

This plan defines the Notify migration increment that enables HTTP-wrapper
routing for `gotify` and `webhook` providers while preserving current Discord
behavior.

Primary goals:

1. Enable a unified wrapper path for outbound provider dispatch.
2. Make Gotify token handling write-only and non-leaking by contract.
3. Add explicit SSRF/redirect/rebinding protections.
4. Add strict error leakage controls for preview/test paths.
5. Add wrapper transport guardrails and expanded validation tests.

## Research Findings

### Current architecture and constraints

- Notification provider CRUD/Test/Preview routes already exist:
  - `GET/POST/PUT/DELETE /api/v1/notifications/providers`
  - `POST /api/v1/notifications/providers/test`
  - `POST /api/v1/notifications/providers/preview`
- Current provider handling is Discord-centric in handler/service/frontend.
- Security-event dispatch path exists and is stable.
- Existing notification E2E coverage is mostly Discord-focused.

### Gaps to close

1. Wrapper enablement for Gotify/Webhook is incomplete end-to-end.
2. Token secrecy contract is not explicit enough across write/read/test flows.
3. SSRF policy needs explicit protocol, redirect, and DNS rebinding rules.
4. Error details need strict sanitization and request correlation.
5. Retry/body/header transport limits need explicit hard requirements.

## Requirements (EARS)

1. WHEN provider type is `gotify` or `webhook`, THE SYSTEM SHALL dispatch
   outbound notifications through a shared HTTP wrapper path.
2. WHEN provider type is `discord`, THE SYSTEM SHALL preserve current behavior
   with no regression in create/update/test/preview flows.
3. WHEN a Gotify token is provided, THE SYSTEM SHALL accept it only on create
   and update write paths.
4. WHEN a Gotify token is accepted, THE SYSTEM SHALL store it securely
   server-side.
5. WHEN provider data is returned on read/test/preview responses, THE SYSTEM
   SHALL NOT return token values or secret derivatives.
6. WHEN validation errors or logs are emitted, THE SYSTEM SHALL NOT echo token,
   auth header, or secret material.
7. WHEN wrapper dispatch is used, THE SYSTEM SHALL enforce HTTPS-only targets by
   default.
8. WHEN development override is required for HTTP targets, THE SYSTEM SHALL
   allow it only via explicit controlled dev flag, disabled by default.
9. WHEN redirects are encountered, THE SYSTEM SHALL deny redirects by default;
   if redirects are enabled, THE SYSTEM SHALL re-validate each hop.
10. WHEN resolving destination addresses, THE SYSTEM SHALL block loopback,
    link-local, private, multicast, and IPv6 ULA ranges.
11. WHEN DNS resolution changes during request lifecycle, THE SYSTEM SHALL
    perform re-resolution checks and reject rebinding to blocked ranges.
12. WHEN wrapper mode dispatches Gotify/Webhook, THE SYSTEM SHALL use `POST`
    only.
13. WHEN preview/test/send errors are returned, THE SYSTEM SHALL return only
    sanitized categories and include `request_id`.
14. WHEN preview/test/send errors are returned, THE SYSTEM SHALL NOT include raw
    payloads, token values, or raw query-string data.
15. WHEN wrapper transport executes, THE SYSTEM SHALL enforce max request and
    response body sizes, strict header allowlist, and bounded retry budget with
    exponential backoff and jitter.
16. WHEN retries are evaluated, THE SYSTEM SHALL retry only on network errors,
    `429`, and `5xx`; it SHALL NOT retry other `4xx` responses.

## Technical Specifications

### Backend contract

- New module: `backend/internal/notifications/http_wrapper.go`
- Core types: `HTTPWrapperRequest`, `RetryPolicy`, `HTTPWrapperResult`,
  `HTTPWrapper`
- Core functions: `NewNotifyHTTPWrapper`, `Send`, `isRetryableStatus`,
  `sanitizeOutboundHeaders`

### Gotify secret contract

- Token accepted only in write path:
  - `POST /api/v1/notifications/providers`
  - `PUT /api/v1/notifications/providers/:id`
- Token stored securely server-side.
- Token never returned in:
  - provider reads/lists
  - test responses
  - preview responses
- Token never shown in:
  - validation details
  - logs
  - debug payload echoes
- Token transport uses header `X-Gotify-Key` only.
- Query token usage is rejected.

### SSRF hardening requirements

- HTTPS-only by default.
- Controlled dev override for HTTP (explicit flag, default-off).
- Redirect policy:
  - deny redirects by default, or
  - if enabled, re-validate each redirect hop before follow.
- Address range blocking includes:
  - loopback
  - link-local
  - private RFC1918
  - multicast
  - IPv6 ULA
  - other internal/non-routable ranges used by current SSRF guard.
- DNS rebinding mitigation:
  - resolve before request
  - re-resolve before connect/use
  - reject when resolved destination shifts into blocked space.
- Wrapper dispatch method for Gotify/Webhook remains `POST` only.

### Error leakage controls

- Preview/Test/Send errors return:
  - `error`
  - `code`
  - `category` (sanitized)
  - `request_id`
- Forbidden in error payloads/logs:
  - raw request payloads
  - tokens/auth headers
  - full query strings containing secrets
  - raw upstream response dumps that can leak sensitive fields.

### Wrapper transport safety

- Request body max: 256 KiB.
- Response body max: 1 MiB.
- Strict outbound header allowlist:
  - `Content-Type`
  - `User-Agent`
  - `X-Request-ID`
  - `X-Gotify-Key`
  - explicitly allowlisted custom headers only.
- Retry budget:
  - max attempts: 3
  - exponential backoff + jitter
  - retry on network error, `429`, `5xx`
  - no retry on other `4xx`.

## API Behavior by Mode

### `gotify`

- Required: `type`, `url`, valid payload with `message`.
- Token accepted only on create/update writes.
- Outbound auth via `X-Gotify-Key` header.
- Query-token requests are rejected.

### `webhook`

- Required: `type`, `url`, valid renderable template.
- Outbound dispatch through wrapper (`POST` JSON) with strict header controls.

### `discord`

- Existing behavior remains unchanged for this migration.

## Frontend Design

- `frontend/src/api/notifications.ts`
  - supports `discord`, `gotify`, `webhook`
  - submits token only on create/update writes
  - never expects token in read/test/preview payloads
- `frontend/src/pages/Notifications.tsx`
  - conditional provider fields
  - masked Gotify token input
  - no token re-display in readback views
- `frontend/src/pages/__tests__/Notifications.test.tsx`
  - update discord-only assumptions
  - add redaction checks

## Test Matrix Expansion

### Playwright E2E

- Update: `tests/settings/notifications.spec.ts`
- Add: `tests/settings/notifications-payload.spec.ts`

Required scenarios:

1. Redirect-to-internal SSRF attempt is blocked.
2. DNS rebinding simulation is blocked (unit/integration + E2E observable path).
3. Retry policy verification:
   - retry on `429` and `5xx`
   - no retry on non-`429` `4xx`.
4. Token redaction checks across API/log/UI surfaces.
5. Query-token rejection.
6. Oversized payload rejection.
7. Discord regression coverage.

### Backend Unit/Integration

- Update/add:
  - `backend/internal/services/notification_service_json_test.go`
  - `backend/internal/services/notification_service_test.go`
  - `backend/internal/services/enhanced_security_notification_service_test.go`
  - `backend/internal/api/handlers/notification_provider_handler_test.go`
  - `backend/internal/api/handlers/notification_provider_handler_validation_test.go`
- Add integration file:
  - `backend/integration/notification_http_wrapper_integration_test.go`

Mandatory assertions:

- redirect-hop SSRF blocking
- DNS rebinding mitigation
- retry/non-retry classification
- token redaction in API/log/UI
- query-token rejection
- oversized payload rejection

## Implementation Plan

### Phase 1 — Backend safety foundation

- implement wrapper contract
- implement secret contract + SSRF/error/transport controls
- keep frontend unchanged

Exit criteria:

- backend tests green
- no Discord regression in backend paths

### Phase 2 — Frontend enablement

- enable Gotify/Webhook UI/client paths
- enforce token write-only UX semantics

Exit criteria:

- frontend tests green
- accessibility and form behavior validated

### Phase 3 — E2E and coverage hardening

- add expanded matrix scenarios
- enforce DoD sequence and patch-report artifacts

Exit criteria:

- E2E matrix passing
- `test-results/local-patch-report.md` generated
- `test-results/local-patch-report.json` generated

## PR Slicing Strategy

Decision: Multiple PRs for security and rollback safety.

### Schema migration decision

- Decision: no schema migration in `PR-1`.
- Contingency: if schema changes become necessary, create separate `PR-0` for
  migration-only changes before `PR-1`.

### PR-1 — Backend wrapper + safety controls

Scope:

- wrapper module + service/handler integration
- secret contract + SSRF + leakage + transport controls
- unit/integration tests

Mandatory rollout safety:

- feature flags for Gotify/Webhook dispatch are default `OFF` in PR-1.

Validation gates:

- backend tests pass
- no token leakage in API/log/error flows
- no Discord regression

### PR-2 — Frontend provider UX

Scope:

- API client and Notifications page updates
- frontend tests for mode handling and redaction

Dependencies: PR-1 merged.

Validation gates:

- frontend tests pass
- accessibility checks pass

### PR-3 — Playwright matrix and coverage hardening

Scope:

- notifications E2E matrix expansion
- fixture updates as required

Dependencies: PR-1 and PR-2 merged.

Validation gates:

- security matrix scenarios pass
- patch-report artifacts generated

## Risks and Mitigations

1. Risk: secret leakage via error/log paths.
   - Mitigation: mandatory redaction and sanitized-category responses.
2. Risk: SSRF bypass via redirects/rebinding.
   - Mitigation: default redirect deny + per-hop re-validation + re-resolution.
3. Risk: retry storms or payload abuse.
   - Mitigation: capped retries, exponential backoff+jitter, size caps.
4. Risk: Discord regression.
   - Mitigation: preserved behavior, regression tests, default-off new flags.

## Acceptance Criteria (Definition of Done)

1. `docs/plans/current_spec.md` contains one active Notify migration plan only.
2. Gotify token contract is explicit: write-path only, secure storage, zero
   read/test/preview return.
3. SSRF hardening includes HTTPS default, redirect controls, blocked ranges,
   rebinding checks, and POST-only wrapper method.
4. Preview/test error details are sanitized with `request_id` and no raw
   payload/token/query leakage.
5. Transport safety includes body size limits, strict header allowlist, and
   bounded retry/backoff+jitter policy.
6. Test matrix includes redirect-to-internal SSRF, rebinding simulation,
   retry split, redaction checks, query-token rejection, oversized-payload
   rejection.
7. PR slicing includes PR-1 default-off flags and explicit schema decision.
8. No conflicting language remains.
9. Status remains: Ready for Supervisor Review.

## Supervisor Handoff

Ready for Supervisor review.

---

## GAS Warning Remediation Plan — Missing Code Scanning Configurations (2026-02-24)

Status: Planned (ready for implementation PR)
Issue: GitHub Advanced Security warning on PRs:

> Code scanning cannot determine alerts introduced by this PR because 3 configurations present on refs/heads/development were not found: `trivy-nightly (nightly-build.yml)`, `.github/workflows/docker-build.yml:build-and-push`, `.github/workflows/docker-publish.yml:build-and-push`.

### 1) Root Cause Summary

Research outcome from current workflow state and history:

- `.github/workflows/docker-publish.yml` was deleted in commit `f640524baaf9770aa49f6bd01c5bde04cd50526c` (2025-12-21), but historical code-scanning configuration identity from that workflow (`.github/workflows/docker-publish.yml:build-and-push`) still exists in baseline comparisons.
- Both legacy `docker-publish.yml` and current `docker-build.yml` used job id `build-and-push` and uploaded Trivy SARIF only for non-PR events (`push`/scheduled paths), so PR branches often do not produce configuration parity.
- `.github/workflows/nightly-build.yml` uploads SARIF with explicit category `trivy-nightly`, but this workflow is schedule/manual only, so PR branches do not emit `trivy-nightly`.
- Current PR scanning in `docker-build.yml` uses `scan-pr-image` with category `docker-pr-image`, which does not satisfy parity for legacy/base configuration identities.
- Result: GitHub cannot compute “introduced by this PR” for those 3 baseline configurations because matching configurations are absent in PR analysis runs.

### 2) Minimal-Risk Remediation Strategy (Future-PR Safe)

Decision: keep existing security scans and add compatibility SARIF uploads in PR context, without changing branch/release behavior.

Why this is minimal risk:

- No changes to image build semantics, release tags, or nightly promotion flow.
- Reuses already-generated SARIF files (no new scanner runtime dependency).
- Limited to additive upload steps and explicit categories.
- Provides immediate parity for PRs while allowing controlled cleanup of legacy configuration.

### 3) Exact Workflow Edits to Apply

#### A. `.github/workflows/docker-build.yml`

In job `scan-pr-image`, after existing `Upload Trivy scan results` step:

1. Add compatibility upload step reusing `trivy-pr-results.sarif` with category:
  - `.github/workflows/docker-build.yml:build-and-push`
2. Add compatibility alias upload step reusing `trivy-pr-results.sarif` with category:
  - `trivy-nightly`
3. Add temporary legacy compatibility upload step reusing `trivy-pr-results.sarif` with category:
  - `.github/workflows/docker-publish.yml:build-and-push`

Implementation notes:

- Keep existing `docker-pr-image` category upload unchanged.
- Add SARIF file existence guards before each compatibility upload (for example, conditional check that `trivy-pr-results.sarif` exists) to avoid spurious step failures.
- Keep compatibility upload steps non-blocking with `continue-on-error: true`; use `if: always()` plus existence guard so upload attempts are resilient but quiet when SARIF is absent.
- Add TODO/date marker in step name/description indicating temporary status for `docker-publish` alias and planned removal checkpoint.

#### B. Mandatory category hardening (same PR)

In `docker-build.yml` non-PR Trivy upload, explicitly set category to `.github/workflows/docker-build.yml:build-and-push`.

- Requirement level: mandatory (not optional).
- Purpose: make identity explicit and stable even if future upload defaults change.
- Safe because it aligns with currently reported baseline identity.

### 4) Migration/Cleanup for Legacy `docker-publish` Configuration

Planned two-stage cleanup:

1. **Stabilization window (concrete trigger):**
  - Keep compatibility upload for `.github/workflows/docker-publish.yml:build-and-push` enabled.
  - Keep temporary alias active through **2026-03-24** and until **at least 8 merged PRs** with successful `scan-pr-image` runs are observed (both conditions required).
  - Verify warning is gone across representative PRs.

2. **Retirement window:**
  - Remove compatibility step for `docker-publish` category from `docker-build.yml`.
  - In GitHub UI/API, close/dismiss remaining alerts tied only to legacy configuration if they persist and are no longer actionable.
  - Confirm new PRs still show introduced-alert computation without warnings.

### 5) Validation Steps (Expected Workflow Observations)

For at least two PRs (one normal feature PR and one workflow-only PR), verify:

1. `docker-build.yml` runs `scan-pr-image` and uploads SARIF under:
  - `docker-pr-image`
  - `.github/workflows/docker-build.yml:build-and-push`
  - `trivy-nightly`
  - `.github/workflows/docker-publish.yml:build-and-push` (temporary)
2. PR Security tab no longer shows:
  - “Code scanning cannot determine alerts introduced by this PR because ... configurations ... were not found”.
3. No regression:
  - Existing Trivy PR blocking behavior remains intact.
  - Main/development/nightly push flows continue unchanged.

### 6) Rollback Notes

If compatibility uploads create noise, duplicate alert confusion, or unstable checks:

1. Revert only the newly added compatibility upload steps (keep original uploads).
2. Re-run workflows on a test PR and confirm baseline behavior restored.
3. If warning reappears, switch to fallback strategy:
  - Keep only `.github/workflows/docker-build.yml:build-and-push` compatibility upload.
  - Remove `trivy-nightly` alias and handle nightly parity via separate dedicated PR-safe workflow.

### 7) PR Slicing Strategy for This Fix

- **PR-1 (recommended single PR, low-risk additive):** add compatibility SARIF uploads in `docker-build.yml` (`scan-pr-image`) with SARIF existence guards, `continue-on-error` on compatibility uploads, and mandatory non-PR category hardening, plus brief inline rationale comments.
- **PR-2 (cleanup PR, delayed):** remove `.github/workflows/docker-publish.yml:build-and-push` compatibility upload after stabilization window and verify no warning recurrence.
