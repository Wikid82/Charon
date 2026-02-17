# CI Encryption-Key Investigation and Remediation Plan

## Context
- Date: 2026-02-17
- Scope: CI failures where backend jobs report encryption key not picked up.
- In-scope files:
  - `.github/workflows/quality-checks.yml`
  - `.github/workflows/codecov-upload.yml`
  - `scripts/go-test-coverage.sh`
  - `backend/internal/crypto/rotation_service.go`
  - `backend/internal/services/dns_provider_service.go`
  - `backend/internal/services/credential_service.go`

## Problem Statement
CI backend tests can fail late and ambiguously when `CHARON_ENCRYPTION_KEY` is missing or malformed. The root causes are context-dependent secret availability, missing preflight validation, and drift between workflow intent and implementation.

## Research Findings

### Workflow Surface and Risks
| Workflow | Job | Key-sensitive step | Current key source | Main risk |
|---|---|---|---|---|
| `.github/workflows/quality-checks.yml` | `backend-quality` | `Run Go tests`, `Run Perf Asserts` | `${{ secrets.CHARON_ENCRYPTION_KEY_TEST }}` | Empty/malformed input not preflighted |
| `.github/workflows/codecov-upload.yml` | `backend-codecov` | `Run Go tests with coverage` | `${{ secrets.CHARON_ENCRYPTION_KEY_TEST }}` | Same key-risk as above |

### Backend Failure Surface
- `backend/internal/crypto/rotation_service.go`
  - `NewRotationService(db *gorm.DB)` hard-fails if `CHARON_ENCRYPTION_KEY` is empty.
- `backend/internal/services/dns_provider_service.go`
  - `NewDNSProviderService(...)` depends on `NewRotationService(...)` and can degrade to warning-based behavior when key input is bad.
- `backend/internal/services/credential_service.go`
  - `NewCredentialService(...)` has the same dependency pattern.

### Script Failure Mode
- `scripts/go-test-coverage.sh` currently uses `set -euo pipefail` but does not pre-validate key shape before `go test`.
- Empty secret expressions become late runtime failures instead of deterministic preflight failures.

## Supervisor-Required Constraints (Preserved)
1. `pull_request_target` SHALL NOT be used for secret-bearing backend test execution on untrusted code (fork PRs and Dependabot PRs).
2. Same-repo `pull_request` and `workflow_dispatch` SHALL require `CHARON_ENCRYPTION_KEY_TEST`; missing secret SHALL fail fast (no fallback).
3. Fork PRs and Dependabot PRs SHALL use workflow-only ephemeral key fallback for backend test execution.
4. Key material SHALL NEVER be logged.
5. Resolved key SHALL be masked before any potential output path.
6. `GITHUB_ENV` propagation SHALL use safe delimiter write pattern.
7. Workflow layer SHALL own key resolution/fallback.
8. Script layer SHALL only validate and fail fast; it SHALL NOT generate fallback keys.
9. Anti-drift guard SHALL be added so trigger comments and trigger blocks remain aligned.
10. Known drift SHALL be corrected: comment in `quality-checks.yml` about `codecov-upload.yml` trigger behavior must match actual triggers.

## EARS Requirements

### Ubiquitous
- THE SYSTEM SHALL fail fast with explicit diagnostics when encryption-key input is required and unavailable or malformed.
- THE SYSTEM SHALL prevent secret-value exposure in logs, summaries, and artifacts.

### Event-driven
- WHEN workflow context is trusted (same-repo `pull_request` or `workflow_dispatch`), THE SYSTEM SHALL require `secrets.CHARON_ENCRYPTION_KEY_TEST`.
- WHEN workflow context is untrusted (fork PR or Dependabot PR), THE SYSTEM SHALL generate ephemeral key material in workflow preflight only.
- WHEN workflow context is untrusted, THE SYSTEM SHALL NOT use `pull_request_target` for secret-bearing backend tests.

### Unwanted behavior
- IF `CHARON_ENCRYPTION_KEY` is empty, non-base64, or decoded length is not 32 bytes, THEN THE SYSTEM SHALL stop before running tests.
- IF trigger comments diverge from workflow triggers, THEN THE SYSTEM SHALL fail anti-drift validation.

## Technical Design

### Workflow Contract
Both backend jobs (`backend-quality`, `backend-codecov`) implement the same preflight sequence:
1. `Resolve encryption key for backend tests`
2. `Fail fast when required encryption secret is missing`
3. `Validate encryption key format`

### Preflight Resolution Algorithm
1. Detect fork PR context via `github.event.pull_request.head.repo.fork`.
2. Detect Dependabot PR context (actor/repo metadata check).
3. Trusted context: require `secrets.CHARON_ENCRYPTION_KEY_TEST`; fail immediately if empty.
4. Untrusted context: generate ephemeral key (`openssl rand -base64 32`) in workflow only.
5. Mask resolved key via `::add-mask::`.
6. Export via delimiter-based `GITHUB_ENV` write:
   - `CHARON_ENCRYPTION_KEY<<EOF`
   - `<value>`
   - `EOF`

### Script Validation Contract
`scripts/go-test-coverage.sh` adds strict preflight validation:
- Present and non-empty.
- Base64 decodable.
- Decoded length exactly 32 bytes.

Script constraints:
- SHALL NOT generate keys.
- SHALL NOT select key source.
- SHALL only validate and fail fast with deterministic error messages.

### Error Handling Matrix
| Condition | Detection layer | Outcome |
|---|---|---|
| Trusted context + missing secret | Workflow preflight | Immediate failure with explicit message |
| Untrusted context + no secret access | Workflow preflight | Ephemeral key path (masked) |
| Malformed key | Script preflight | Immediate failure before `go test` |
| Trigger/comment drift | Workflow consistency guard | CI failure until synchronized |

## Implementation Plan

### Phase 1: Workflow Hardening
- Update `.github/workflows/quality-checks.yml` and `.github/workflows/codecov-upload.yml` with identical key-resolution and key-validation steps.
- Enforce trusted-context fail-fast and untrusted-context fallback boundaries.
- Add explicit prohibition notes and controls preventing `pull_request_target` migration for secret-bearing tests.

### Phase 2: Script Preflight Hardening
- Update `scripts/go-test-coverage.sh` to validate key presence/format/length before tests.
- Preserve existing coverage behavior; only harden pre-test guard path.

### Phase 3: Anti-Drift Enforcement
- Define one canonical backend-key-bootstrap contract path.
- Add consistency check that enforces trigger/comment parity between `quality-checks.yml` and `codecov-upload.yml`.
- Fix known push-only comment mismatch in `quality-checks.yml`.

## Validation Plan
Run these scenarios:
1. Same-repo PR with valid secret.
2. Same-repo PR with missing secret (must fail fast).
3. Same-repo PR with malformed secret (must fail fast before tests).
4. Fork PR with no secret access (must use ephemeral fallback).
5. Dependabot PR with no secret access (must use ephemeral fallback, no `pull_request_target`).
6. `workflow_dispatch` with valid secret.

Expected results:
- No late ambiguous key-init failures.
- No secret material logged.
- Deterministic and attributable failure messages.
- Trigger docs and trigger config remain synchronized.

## Acceptance Criteria
- Backend jobs in `quality-checks.yml` and `codecov-upload.yml` no longer fail ambiguously on encryption-key pickup.
- Trusted contexts fail fast if `CHARON_ENCRYPTION_KEY_TEST` is missing.
- Untrusted contexts use workflow-only ephemeral fallback.
- `scripts/go-test-coverage.sh` enforces deterministic key preflight checks.
- `pull_request_target` is explicitly prohibited for secret-bearing backend tests on untrusted code.
- Never-log-key-material and safe `GITHUB_ENV` propagation are implemented.
- Workflow/script responsibility boundary is enforced.
- Anti-drift guard is present and known trigger-comment mismatch is resolved.

## Handoff to Supervisor
- This document is intentionally single-scope and restricted to CI encryption-key investigation/remediation.
- Legacy multi-topic coverage planning content has been removed from this file to maintain coherence.
