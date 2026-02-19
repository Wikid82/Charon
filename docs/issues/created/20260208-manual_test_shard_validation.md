# Manual Test Plan: Shard Isolation Verification

## Objective
Verify that the `e2e-integration` shard (non-security) no longer executes tests requiring Cerberus, WAF, or CrowdSec, and that the `e2e-security` shard picks up the migrated tests.

## Test Cases

### 1. Verify Non-Security Shard
- **Action**: Run the `tests/integration` folder with Cerberus DISABLED.
- **Expected Outcome**:
    - All tests in `multi-feature-workflows.spec.ts` (Groups A, C, D) pass.
    - No tests attempt to navigate to `/security/waf`, `/security/crowdsec`, or toggle WAF features.
    - No 404s or timeouts related to missing security components.

### 2. Verify Security Shard
- **Action**: Run the `tests/security` folder with Cerberus ENABLED.
- **Expected Outcome**:
    - `workflow-security.spec.ts` runs and executes the 4 extracted tests.
    - WAF, CrowdSec, and ACL features are successfully configured.

### 3. CI Pipeline Verification
- **Action**: Trigger a full CI run.
- **Expected Outcome**:
    - `e2e-tests / shard (1, 2)` (Non-security) passes green.
    - `e2e-tests / security-shard` passes green (or fails only on genuine bugs, not configuration mismatches).
