---
post_title: "Current Spec: Resolve Proxy Host Hostname Validation Test Failures"
categories:
  - actions
  - testing
  - backend
tags:
  - go
  - proxyhost
  - unit-tests
  - validation
summary: "Focused plan to resolve failing TestProxyHostService_ValidateHostname malformed URL cases by aligning test expectations with intended validation behavior and validating via targeted service tests and coverage gate."
post_date: 2026-02-22
---

## Active Plan: Resolve Failing Hostname Validation Tests

Date: 2026-02-22
Status: Active and authoritative
Scope Type: Backend test-failure remediation (service validation drift analysis)
Authority: This is the only active authoritative plan section in this file.

## Introduction

This plan resolves backend run failures in `TestProxyHostService_ValidateHostname`
for malformed URL cases while preserving intended hostname validation behavior.

Primary objective:

- Restore green test execution in `backend/internal/services` with a minimal,
  low-risk change path.

## Research Findings

### Evidence Collected

- Failing command output confirms two failing subtests:
  - `TestProxyHostService_ValidateHostname/malformed_https_URL`
  - `TestProxyHostService_ValidateHostname/malformed_http_URL`
- Failure message for both cases: `invalid hostname format`.

### Exact Files Involved

1. `backend/internal/services/proxyhost_service_validation_test.go`
   - Test function: `TestProxyHostService_ValidateHostname`
   - Failing cases currently expect `wantErr: false` for malformed URLs.
2. `backend/internal/services/proxyhost_service.go`
   - Service function: `ValidateHostname(host string) error`
   - Behavior: strips scheme, then validates hostname characters; malformed
     residual values containing `:` are rejected with `invalid hostname format`.

### Root Cause Determination

- Root cause is **test expectation drift**, not runtime service regression.
- `git blame` shows malformed URL test cases were added on 2026-02-22 with
  permissive expectations, while validation behavior rejecting malformed host
  strings predates those additions.
- Existing behavior aligns with stricter hostname validation and should remain
  the default unless product requirements explicitly demand permissive handling
  of malformed host inputs.

### Confidence Assessment

- Confidence score: **95% (High)**
- Rationale: direct reproduction, targeted file inspection, and blame history
  converge on expectation drift.

## Requirements (EARS)

- WHEN malformed `http://` or `https://` host strings are passed to
  `ValidateHostname`, THE SYSTEM SHALL return a validation error.
- WHEN service validation behavior is intentionally strict, THE TESTS SHALL
  assert rejection for malformed URL residual host strings.
- IF product intent is permissive for malformed inputs, THEN THE SYSTEM SHALL
  minimally relax parsing logic without weakening valid invalid-character checks.
- WHEN changes are completed, THE SYSTEM SHALL pass targeted service tests and
  the backend coverage gate script.

## Technical Specification

### Minimal Fix Path (Preferred)

Preferred path: **test-only correction**.

1. Update malformed URL table entries in
   `backend/internal/services/proxyhost_service_validation_test.go`:
   - `malformed https URL` -> `wantErr: true`
   - `malformed http URL` -> `wantErr: true`
2. Keep current service behavior in
   `backend/internal/services/proxyhost_service.go` unchanged.
3. Optional test hardening (still test-only): assert error contains
   `invalid hostname format` for those two cases.

### Alternative Path (Only if Product Intent Differs)

Use only if maintainers explicitly confirm malformed URL inputs should pass:

1. Apply minimal service correction in `ValidateHostname` to normalize malformed
   scheme inputs before character validation.
2. Add or update tests to preserve strict rejection for truly invalid hostnames
   (e.g., `$`, `@`, `%`, `&`) so validation is not broadly weakened.

Decision default for this plan: **Preferred path (test updates only)**.

## Implementation Plan

### Phase 1: Test-first Repro and Baseline

1. Confirm current failure (already reproduced).
2. Record failing subtests and error signatures as baseline evidence.

### Phase 2: Minimal Remediation

1. Apply preferred test expectation update in
   `backend/internal/services/proxyhost_service_validation_test.go`.
2. Keep service code unchanged unless product intent is clarified otherwise.

### Phase 3: Targeted Validation

Run in this order:

1. `go test ./backend/internal/services -run TestProxyHostService_ValidateHostname -v`
2. Related service package tests:
   - `go test ./backend/internal/services -run TestProxyHostService -v`
   - `go test ./backend/internal/services -v`
3. Final gate:
   - `bash scripts/go-test-coverage.sh`

## Risk Assessment

### Key Risks

1. **Semantic risk (low):** updating tests could mask an intended behavior
   change if malformed URL permissiveness was deliberate.
2. **Coverage risk (low):** test expectation changes may alter branch coverage
   marginally but should not threaten gate based on current context.
3. **Regression risk (low):** service runtime behavior remains unchanged in the
   preferred path.

### Mitigations

- Keep change surgical to two table entries.
- Preserve existing invalid-character rejection coverage.
- Require full service package run plus coverage script before merge.

## Rollback Plan

If maintainer/product decision confirms permissive malformed URL handling is
required:

1. Revert the test expectation update commit.
2. Implement minimal service normalization change in
   `backend/internal/services/proxyhost_service.go`.
3. Add explicit tests documenting the accepted malformed-input behavior and
   retain strict negative tests for illegal hostname characters.
4. Re-run targeted validation commands and coverage gate.

## PR Slicing Strategy

Decision: **Single PR**.

Rationale:

- Scope is tightly bounded to one service test suite and one failure cluster.
- Preferred remediation is test-only with low rollback complexity.
- Review surface is small and dependency-free.

Contingency split trigger:

- Only split if product intent forces service logic change, in which case:
  - PR-1: test expectation alignment rollback + service behavior decision record
  - PR-2: minimal service correction + adjusted tests

## Config/Infra File Impact Review

Reviewed for required updates:

- `.gitignore`
- `.dockerignore`
- `codecov.yml`
- `Dockerfile`

Planned changes: **None required** for this focused backend test-remediation
scope.

## Acceptance Criteria

1. `TestProxyHostService_ValidateHostname` passes, including malformed URL
   subtests.
2. `go test ./backend/internal/services -run TestProxyHostService -v` passes.
3. `go test ./backend/internal/services -v` passes.
4. `bash scripts/go-test-coverage.sh` passes final gate.
5. Root cause is documented as expectation drift vs. service behavior drift, and
   chosen path is explicitly recorded.
