---
title: "E2E Security Test Isolation"
status: "draft"
scope: "e2e/ci, tests/playwright"
notes: Separate security-toggling Playwright tests from non-security shards to prevent ACL, WAF, and rate-limit contamination.
---

## 1. Introduction

This plan addresses E2E test contamination where security-focused tests are executed in non-security shards. The goal is to isolate tests that toggle Cerberus, ACL, WAF, CrowdSec, or rate limiting so non-security shards remain stable and do not hit global security state changes. The scope includes Playwright test organization and the E2E workflow split.

Objectives:

- Identify which Playwright tests in non-security shards toggle or reset security modules.
- Separate security-toggling tests into security-only execution paths.
- Keep non-security shards stable by preventing global security state changes within those shards.
- Preserve current coverage of security behaviors while avoiding cross-shard interference.

## 2. Research Findings

### 2.1 Non-Security Shard Inputs

The non-security shards in the E2E workflow run a fixed set of directories and files in [ .github/workflows/e2e-tests-split.yml ](../../.github/workflows/e2e-tests-split.yml). The inputs include tests/settings, tests/integration, and tests/emergency-server, which contain security-toggling behavior.

### 2.2 Security-Toggling Tests in Settings

[ tests/settings/system-settings.spec.ts ](../../tests/settings/system-settings.spec.ts) toggles Cerberus and CrowdSec feature flags via the feature flags API and resets those flags after each test. These tests change global security state and can affect unrelated shards running in parallel.

### 2.3 Emergency Server Tests

[ tests/emergency-server/tier2-validation.spec.ts ](../../tests/emergency-server/tier2-validation.spec.ts) calls the emergency security reset endpoint and validates rate limiting behavior on the emergency server. This directly disables security modules during execution and should be treated as security enforcement coverage.

### 2.4 Global Security Reset in Test Setup

[ tests/global-setup.ts ](../../tests/global-setup.ts) performs an emergency security reset and verifies that ACL and rate limiting are disabled before tests run. This is intended for cleanup, but it reinforces that global security state is shared across shards and is sensitive to security toggles.

Observed behavior in [ tests/global-setup.ts ](../../tests/global-setup.ts):

- Always validates `CHARON_EMERGENCY_TOKEN` and fails fast if missing or invalid.
- Executes pre-auth and authenticated `emergencySecurityReset()`.
- Runs `verifySecurityDisabled()` after the authenticated reset.

This means non-security shards still perform a global security reset even when `CHARON_SECURITY_TESTS_ENABLED` is set to `false` in the workflow.

### 2.5 Security Test Suites Already Isolated

The workflow already routes tests/security and tests/security-enforcement into dedicated security jobs. These suites include explicit security module enablement and enforcement checks, such as rate-limit enforcement in [ tests/security-enforcement/rate-limit-enforcement.spec.ts ](../../tests/security-enforcement/rate-limit-enforcement.spec.ts) and dashboard toggles in [ tests/security/security-dashboard.spec.ts ](../../tests/security/security-dashboard.spec.ts).

### 2.6 Integration Tests Touch Security Domains

Some integration tests create access lists and navigate to security pages, for example [ tests/integration/multi-feature-workflows.spec.ts ](../../tests/integration/multi-feature-workflows.spec.ts). These do not explicitly toggle security modules, but they use security-domain resources that may depend on Cerberus state and should be reviewed for compatibility with Cerberus being disabled.

## 3. Technical Specifications

### 3.1 Security Test Classification Rules

Classify a test as security-affecting if it does any of the following:

- Calls the emergency security reset endpoint.
- Sets or toggles feature flags related to Cerberus, ACL, WAF, CrowdSec, or rate limiting.
- Enables or disables security modules via settings or admin controls.
- Depends on rate limiting behavior or ACL/WAF enforcement for assertions.

### 3.2 Isolation Strategy Options

Option A (preferred): Move security-affecting tests into dedicated security folders

- Move or split tests from tests/settings/system-settings.spec.ts into a new security-focused file under tests/security or tests/security-enforcement.
- Move tests/emergency-server to tests/security-enforcement or tests/security, depending on whether they validate enforcement behavior or emergency pathways.
- Keep non-security shards limited to tests that do not mutate security state.

Option B: Use Playwright tags and workflow filters

- Tag security-affecting tests with a consistent tag such as @security-affecting.
- Update security jobs to run tagged tests and non-security jobs to exclude them using grep or grep-invert.

Option C: Update non-security job inputs to explicitly exclude security-affecting files

- Remove tests/settings/system-settings.spec.ts and tests/emergency-server from non-security shard inputs.
- Add those tests to the security job inputs.

Decision: Prefer Option A with a fallback to Option B if the team wants to keep files in their current directories. Option C is acceptable as a short-term mitigation but is less maintainable long-term.

### 3.3 Workflow Separation Rules

Update [ .github/workflows/e2e-tests-split.yml ](../../.github/workflows/e2e-tests-split.yml) so:

- Security jobs explicitly include all security-affecting tests, including those moved from settings and emergency-server.
- Non-security jobs do not include any files or directories that toggle or reset security modules.
- If tags are used, security jobs should run only tagged tests and non-security jobs should invert the tag.

### 3.4 Test Organization Changes

Planned file moves and splits:

- Split tests/settings/system-settings.spec.ts so security-affecting tests move to a dedicated security-focused test file under tests/security.
- Move tests/emergency-server into a security-enforcement folder.
- Review integration tests for dependencies on security module state and move or tag as needed.

Concrete list of tests to move from [ tests/settings/system-settings.spec.ts ](../../tests/settings/system-settings.spec.ts) into a new file [ tests/security/system-settings-feature-toggles.spec.ts ](../../tests/security/system-settings-feature-toggles.spec.ts):

- Feature Toggles:
	- "should toggle Cerberus security feature"
	- "should toggle CrowdSec console enrollment"
	- "should toggle uptime monitoring"
	- "should persist feature toggle changes"
	- "should show overlay during feature update"
- Feature Toggles - Advanced Scenarios (Phase 4):
	- "should handle concurrent toggle operations"
	- "should retry on 500 Internal Server Error"
	- "should fail gracefully after max retries exceeded"
	- "should verify initial feature flag state before tests"

Note: The `test.afterEach` feature flag reset and `test.afterAll` API metrics reporting currently tied to toggles should move with the toggle suite into [ tests/security/system-settings-feature-toggles.spec.ts ](../../tests/security/system-settings-feature-toggles.spec.ts) to keep state cleanup scoped to the security job.

Concrete emergency server file moves:

- Move [ tests/emergency-server/emergency-server.spec.ts ](../../tests/emergency-server/emergency-server.spec.ts) to [ tests/security-enforcement/emergency-server/emergency-server.spec.ts ](../../tests/security-enforcement/emergency-server/emergency-server.spec.ts).
- Move [ tests/emergency-server/tier2-validation.spec.ts ](../../tests/emergency-server/tier2-validation.spec.ts) to [ tests/security-enforcement/emergency-server/tier2-validation.spec.ts ](../../tests/security-enforcement/emergency-server/tier2-validation.spec.ts).

### 3.5 Error Handling and Edge Cases

- Parallel shards must not toggle global security state at the same time.
- Tests that require Cerberus enabled must run only in security jobs where Cerberus is enabled by environment or explicit setup.
- If global setup performs a security reset, security jobs must re-enable required modules before assertions.

### 3.6 Global Setup Conditioning (Critical)

Global setup must not reset security in non-security shards. Add a guard in [ tests/global-setup.ts ](../../tests/global-setup.ts):

- Only validate `CHARON_EMERGENCY_TOKEN`, call `emergencySecurityReset()`, and run `verifySecurityDisabled()` when `CHARON_SECURITY_TESTS_ENABLED === 'true'`.
- For non-security shards (`CHARON_SECURITY_TESTS_ENABLED !== 'true'`), skip all security reset logic and continue with health checks and test data cleanup only.
- Preserve existing behavior for security shards so enforcement tests still run against a deterministic baseline.

## 4. Implementation Plan

### Phase 1: Playwright Tests (Behavior Baseline)

- Confirm the current security toggle behavior in system settings and emergency server tests.
- Define expected outcomes for toggling Cerberus and CrowdSec so that moved tests retain coverage.

### Phase 2: Security-Affecting Test Identification

- Inventory tests in tests/settings, tests/emergency-server, and tests/integration against the security-affecting rules.
- Create a list of files to move, split, or tag.

### Phase 3: Test Restructuring

- Split tests/settings/system-settings.spec.ts to isolate security toggles into [ tests/security/system-settings-feature-toggles.spec.ts ](../../tests/security/system-settings-feature-toggles.spec.ts) using the concrete list above.
- Move emergency server tests into [ tests/security-enforcement/emergency-server/ ](../../tests/security-enforcement/emergency-server/) using the concrete list above.
- If integration tests require security modules enabled, relocate or tag them.

### Phase 4: Workflow Updates

- Update non-security shard inputs in [ .github/workflows/e2e-tests-split.yml ](../../.github/workflows/e2e-tests-split.yml):
	- Remove `tests/emergency-server` from non-security job inputs.
	- Keep `tests/settings` but ensure the moved security toggle suite lives under `tests/security` so it is not picked up.
- Update security job inputs to include the relocated emergency server folder:
	- Ensure `tests/security-enforcement/emergency-server` is included (already covered by `tests/security-enforcement/` once moved).
	- Security jobs already include `tests/security/`, which will pick up `tests/security/system-settings-feature-toggles.spec.ts`.
- If tags are adopted, add grep filters to the security and non-security job commands.

### Phase 5: Validation and Guardrails

- Run the security jobs and non-security jobs separately and confirm no security-related tests execute in non-security shards.
- Confirm rate limit and ACL enforcement tests only run under security jobs with Cerberus enabled.
- Capture and review Playwright reports for cross-shard contamination indicators.

## 5. Acceptance Criteria (EARS)

- WHEN a non-security E2E shard runs, THE SYSTEM SHALL exclude all tests that toggle or reset Cerberus, ACL, WAF, CrowdSec, or rate limiting.
- WHEN a non-security E2E shard runs, THE SYSTEM SHALL skip the global security reset in [ tests/global-setup.ts ](../../tests/global-setup.ts) unless `CHARON_SECURITY_TESTS_ENABLED` is `true`.
- WHEN a security E2E shard runs, THE SYSTEM SHALL include all tests that toggle or reset security modules and all enforcement tests.
- WHEN security-affecting tests run, THE SYSTEM SHALL execute them only in workflows where Cerberus is enabled.
- WHEN tests are reorganized, THE SYSTEM SHALL preserve existing security coverage without introducing new cross-shard dependencies.
- WHEN integration tests require security modules enabled, THE SYSTEM SHALL route them to security shards or explicitly enable security in their setup.

## 6. Risks and Mitigations

- Risk: Moving tests breaks historical references or documentation links. Mitigation: update any references in test comments and plan docs after moves.
- Risk: Tag-based filtering is inconsistent across local and CI runs. Mitigation: document the tag usage in Playwright config and ensure local scripts align with CI filters.
- Risk: Integration tests implicitly rely on Cerberus being enabled. Mitigation: audit integration tests and either enable Cerberus in test setup or move them to security shards.

## 7. Confidence Score

Confidence: 78 percent

Rationale: The security-toggling tests are identifiable and the workflow split is clear, but integration test dependencies on security state require additional verification before final routing.
