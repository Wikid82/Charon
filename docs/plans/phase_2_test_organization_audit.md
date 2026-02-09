# Phase 2 Test Organization Audit

**Date**: 2026-02-09

## Scope

Phase 2 runs with `PLAYWRIGHT_SKIP_SECURITY_DEPS=1`, so security modules are disabled. This audit flags tests in Phase 2 folders that exercise security UI or security-dependent workflows and should be relocated.

## Findings From Phase 2 Failures

No Phase 2 failure messages reference ACL blocks, WAF, rate limiting, or CrowdSec enforcement. The recorded failures are interruption/teardown errors, not security enforcement failures. Security-dependent tests are still present in Phase 2 suites and should be relocated to avoid running with security disabled.

## Misorganized Tests (Relocate)

### Move to tests/security/ (security UI/config)

- [tests/core/access-lists-crud.spec.ts](tests/core/access-lists-crud.spec.ts)
  - **Tests**: `Access Lists - CRUD Operations` (entire file)
  - **Reason**: Access lists are a Cerberus security feature; these tests validate security configuration UI and should not run with security disabled.

- [tests/settings/system-settings.spec.ts](tests/settings/system-settings.spec.ts)
  - **Tests**: `should toggle Cerberus security feature`, `should toggle CrowdSec console enrollment`, `should persist feature toggle changes`, `should handle concurrent toggle operations`, `should retry on 500 Internal Server Error`, `should fail gracefully after max retries exceeded`
  - **Reason**: These tests explicitly change security feature flags and expect propagation that only makes sense when security is enabled and being exercised.
  - **Note**: Remaining non-security system settings tests can stay in Phase 2; recommend splitting into a security toggles spec.

- [tests/settings/encryption-management.spec.ts](tests/settings/encryption-management.spec.ts)
  - **Tests**: `Encryption Management` (entire file)
  - **Reason**: Encryption management is a security area under `/security/encryption` and depends on security configuration state.

- [tests/tasks/import-crowdsec.spec.ts](tests/tasks/import-crowdsec.spec.ts)
  - **Tests**: `Import CrowdSec Configuration` (entire file)
  - **Reason**: CrowdSec import is a security configuration workflow; it should run with security enabled.

- [tests/monitoring/real-time-logs.spec.ts](tests/monitoring/real-time-logs.spec.ts)
  - **Tests**: `Real-Time Logs Viewer` (entire file)
  - **Reason**: The suite explicitly requires Cerberus to render the LiveLogViewer and exercises security-mode log streams and filters.
  - **Note**: If a future split is desired, only the App Logs mode tests should remain in Phase 2.

### Move to tests/security-enforcement/ (blocking/enforcement)

- **None identified in Phase 2 suites.**
  - The Phase 2 failures list does not include enforcement messages like ACL blocks, WAF violations, or rate-limit errors.

## Phase 2 Tests Likely Failing for Environmental Reasons (Keep)

- [tests/settings/account-settings.spec.ts](tests/settings/account-settings.spec.ts)
  - **Failure type**: `page.goto` interrupted / test ended
  - **Reason**: Interruption/teardown, not security-related.

- [tests/tasks/backups-create.spec.ts](tests/tasks/backups-create.spec.ts)
  - **Failure type**: `Browser.removeBrowserContext` / `Test ended`
  - **Reason**: Browser context teardown, not security-related.

- [tests/utils/wait-helpers.spec.ts](tests/utils/wait-helpers.spec.ts)
  - **Failure type**: Suite interrupted before execution
  - **Reason**: Test run interruption, not security-related.

## Relocation Summary

- **Move to tests/security/**: 5 files
  - Access Lists CRUD
  - System Settings security toggles (subset of tests)
  - Encryption Management
  - Import CrowdSec
  - Real-Time Logs Viewer

- **Move to tests/security-enforcement/**: 0 files

- **Keep in Phase 2** (but investigate interruptions): 3 files

## Recommended Moves

1. Move Access Lists CRUD to tests/security/.
2. Split System Settings tests so security toggles move to tests/security/.
3. Move Encryption Management to tests/security/.
4. Move Import CrowdSec to tests/security/.
5. Move Real-Time Logs Viewer to tests/security/ (or split to keep App Logs only in Phase 2).
