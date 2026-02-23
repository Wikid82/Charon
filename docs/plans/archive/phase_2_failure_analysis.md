# Phase 2 E2E Failure Analysis (2026-02-09)

## Overview
Phase 2 commands (tests/core, tests/settings, tests/tasks, tests/monitoring) executed security test suites and then hit widespread ACL/auth failures. The failures are consistent across 2A/2B/2C and point to security modules being enabled during Phase 2, despite Phase 1 reporting a successful security reset.

## Evidence From E2E Remediation Checklist
- Phase 2A/2B/2C failures show ACL blocks and auth errors during non-security runs, e.g.:
  - "Failed to create user: {\"error\":\"Blocked by access control list\"}"
  - "Failed to get security status: 403 {\"error\":\"Blocked by access control list\"}"
  - "Failed to get security status: 401 {\"error\":\"Authorization header required\"}"
- Phase 2A/2B/2C failure lists are dominated by security suite test files, for example:
  - tests/security/waf-config.spec.ts
  - tests/security/security-dashboard.spec.ts
  - tests/security-enforcement/rate-limit-enforcement.spec.ts

## Root Cause Hypotheses (Most Likely)
1) Playwright dependency chain forces security tests to run for Phase 2
   - In playwright.config.js, the browser projects (chromium/firefox/webkit) declare dependencies as:
     - browserDependencies = ['setup', 'security-tests'] unless PLAYWRIGHT_SKIP_SECURITY_DEPS=1
   - Running `npx playwright test tests/core --project=firefox` triggers dependencies for the firefox project, so Playwright executes the security-tests project (security/ and security-enforcement/ suites) before the targeted Phase 2 directory. This explains why security tests appear during Phase 2 commands.

2) Security teardown likely did not execute after security tests failed
   - The security-tests project declares teardown: 'security-teardown'. If security-tests fails early (as reported), teardown may not run, leaving Cerberus/ACL/WAF/rate limiting enabled. That would cause Phase 2 API calls (user creation, security status checks, admin login) to be blocked by ACL, matching the 403/401 errors seen in Phase 2.

3) Auth state is valid for UI but blocked for API due to active security enforcement
   - The 401 "Authorization header required" indicates unauthenticated API calls during security enforcement tests. Combined with ACL blocks, this suggests the system was in security-enforced mode, so normal Phase 2 setup actions (creating users, toggling settings) could not proceed. This is consistent with the security suite running unexpectedly and leaving enforcement active.

## Recommendation
Phase 2 is blocked until the Playwright dependency configuration is addressed or bypassed for Phase 2 runs.
- Confirm whether PLAYWRIGHT_SKIP_SECURITY_DEPS=1 is required for Phase 2 commands.
- Verify that security-teardown ran after the dependency tests or explicitly run the security reset before Phase 2.
- Re-run Phase 2 only after validating that security modules are disabled and auth is functional.

## Next Steps for Engineering Director
1) Validate Playwright dependency behavior with the current config:
   - Confirm that running a single project with --project=firefox triggers security-tests dependencies by design.
2) Decide on a formal Phase 2 execution path:
   - Option A: Require PLAYWRIGHT_SKIP_SECURITY_DEPS=1 for Phase 2 runs.
   - Option B: Add a dedicated Phase 2 project without security-test dependencies.
3) Add a pre-Phase 2 system state verification step:
   - Ensure security modules are disabled and admin whitelist is clear before running Phase 2.
4) Document the dependency requirement in Phase 2 run instructions so this does not repeat.
