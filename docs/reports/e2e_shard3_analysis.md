# E2E Shard 3 Failure Analysis (Run 21865692694)

## Scope

- Run: 21865692694
- Job: E2E Chromium (Shard 3/4)
- Report: /tmp/playwright-report-chromium-shard-3/index.html
- Job log: /tmp/job-63106399789-logs.zip (text)
- Docker log: /tmp/docker-logs-chromium-shard-3/docker-logs-chromium-shard-3.txt

## Section 4 Artifact Inventory

- [x] Playwright report: /tmp/playwright-report-chromium-shard-3/ (index.html, trace/, data/)
- [x] trace.zip files present:
   - /tmp/playwright-report-chromium-shard-3/data/00db5cbb0834571f645c3baea749583a43f280bc.zip
   - /tmp/playwright-report-chromium-shard-3/data/32a3301b546490061554f0a910ebc65f1a915d1a.zip
   - /tmp/playwright-report-chromium-shard-3/data/39a15e19119fae12390b05ca38d137cce56165d8.zip
   - /tmp/playwright-report-chromium-shard-3/data/741efac1b76de966220d842a250273abcb25ab69.zip
- [x] video files present (report data):
   - /tmp/playwright-report-chromium-shard-3/data/00db95d7a985df7dd2155dce1ce936cb57c37fa2.webm
   - /tmp/playwright-report-chromium-shard-3/data/1dcb8e5203cfa246ceb41dc66f5481f83ab75442.webm
   - /tmp/playwright-report-chromium-shard-3/data/2553aa35e467244cac1da3e0091c9a8b7afb7ee7.webm
   - /tmp/playwright-report-chromium-shard-3/data/2c7ff134d9dc2f082d7a96c7ecb8e15867fe91f3.webm
   - /tmp/playwright-report-chromium-shard-3/data/3d0e040a750d652f263a9e2aaa7e5aff340547f1.webm
   - /tmp/playwright-report-chromium-shard-3/data/576f3766390bd6b213c36e5f02149319715ceb4e.webm
   - /tmp/playwright-report-chromium-shard-3/data/5914ac780cec1a252e81d8e12371d5226b32fddb.webm
   - /tmp/playwright-report-chromium-shard-3/data/6cd814ccc1ed36df26f9008b025e03e06795bfc5.webm
   - /tmp/playwright-report-chromium-shard-3/data/74d3b988c807b8d24d72aff8bac721eb5f9d5822.webm
   - /tmp/playwright-report-chromium-shard-3/data/b63644dffa4b275bbabae0cdb8d0c13e3b2ef8a6.webm
   - /tmp/playwright-report-chromium-shard-3/data/cfafb7d98513e884b92bd0d64a0671a9beac9246.webm
   - /tmp/playwright-report-chromium-shard-3/data/fb6b798ef2d714244b95ee404f7e88ef3cfa1091.webm
- [x] test-results.json or reporter JSON: generated locally
   - Raw reporter output (includes setup logs): /tmp/playwright-shard-3-results.json
   - Clean JSON for parsing: /tmp/playwright-shard-3-results.json.cleaned
   - Summary: total=176, expected=29, unexpected=125, skipped=22, flaky=0, duration=538171.541ms
- [x] stdout/stderr logs:
   - /tmp/playwright-chromium.log
   - /tmp/job-63106399789-logs.zip (text)
- [x] Run/job logs download outputs: /tmp/job-63106399789-logs.zip

## Playwright Report Findings

- Report metadata: 2026-02-10 07:58:20 AM (local time) | Total time 8.5m | 115 tests
- Failed tests (4): all in tests/settings/notifications.spec.ts under Notification Providers

### Failing Tests (from report + job logs)

1) tests/settings/notifications.spec.ts:330:5
   - Notification Providers > Provider CRUD > should edit existing provider > Verify update success
   - Report duration: 42.3s
   - Error: expect(locator).toBeVisible() timed out at 10s (update indicator not found)

2) tests/settings/notifications.spec.ts:545:5
   - Notification Providers > Provider CRUD > should validate provider URL
   - Report duration: 3.1m
   - Error: test timeout of 60000ms exceeded; page context closed during locator.clear()

3) tests/settings/notifications.spec.ts:908:5
   - Notification Providers > Template Management > should delete external template > Click delete button with confirmation
   - Report duration: 24.4s
   - Error: expect(locator).toBeVisible() timed out at 5s (delete button not found)

4) tests/settings/notifications.spec.ts:1187:5
   - Notification Providers > Event Selection > should persist event selections > Verify event selections persisted
   - Error: expect(locator).not.toBeChecked() timed out at 5s (checkbox remained checked)

## Failure Timestamps and Docker Correlation

- Job log failure time: 2026-02-10T13:06:49Z for all four failures (includes retries).
- Docker logs during 13:06:40-13:06:48 show normal 200 responses (GET /settings/notifications, GET /api/v1/notifications/providers, GET /api/v1/notifications/external-templates, etc.).
- No container restarts, panics, or 5xx responses at the failure timestamp.
- A 403 appears at 13:06:48 for DELETE /api/v1/users/101, but it does not align with any test error messages.

Conclusion: failures correlate with UI state/expectation issues, not container instability (H3 is not supported).

## Shard 3 Partition (CI Command)

The job ran:

npx playwright test \
  --project=chromium \
  --shard=3/4 \
  tests/core \
  tests/dns-provider-crud.spec.ts \
  tests/dns-provider-types.spec.ts \
  tests/integration \
  tests/manual-dns-provider.spec.ts \
  tests/monitoring \
  tests/settings \
  tests/tasks

Local shard list (same flags) confirms notifications spec is part of shard 3.

## Shard-to-Test Mapping (Shard 3/4)

Command executed:

```bash
npx playwright test --list --shard=3/4 --project=chromium > /tmp/shard-3-test-list.txt
```

Output:

```
[dotenv@17.2.4] injecting env (2) from .env -- tip: 🔐 prevent committing .env to code: https://dotenvx.com/precommit
Listing tests:
   [setup] › auth.setup.ts:164:1 › authenticate
   [chromium] › phase3/coraza-waf.spec.ts:271:5 › Phase 3: Coraza WAF (Attack Prevention) › Malformed Request Handling › should reject oversized payload
   [chromium] › phase3/coraza-waf.spec.ts:291:5 › Phase 3: Coraza WAF (Attack Prevention) › Malformed Request Handling › should reject null characters in payload
   [chromium] › phase3/coraza-waf.spec.ts:308:5 › Phase 3: Coraza WAF (Attack Prevention) › Malformed Request Handling › should reject double-encoded payloads
   [chromium] › phase3/coraza-waf.spec.ts:325:5 › Phase 3: Coraza WAF (Attack Prevention) › CSRF Token Validation › should validate CSRF token presence in state-changing requests
   [chromium] › phase3/coraza-waf.spec.ts:343:5 › Phase 3: Coraza WAF (Attack Prevention) › CSRF Token Validation › should reject invalid CSRF token
   [chromium] › phase3/coraza-waf.spec.ts:365:5 › Phase 3: Coraza WAF (Attack Prevention) › Benign Request Handling › should allow valid domain names
   [chromium] › phase3/coraza-waf.spec.ts:382:5 › Phase 3: Coraza WAF (Attack Prevention) › Benign Request Handling › should allow valid IP addresses
   [chromium] › phase3/coraza-waf.spec.ts:398:5 › Phase 3: Coraza WAF (Attack Prevention) › Benign Request Handling › should allow GET requests with safe parameters
   [chromium] › phase3/coraza-waf.spec.ts:414:5 › Phase 3: Coraza WAF (Attack Prevention) › WAF Response Indicators › blocked request should not expose WAF details
   [chromium] › phase3/crowdsec-integration.spec.ts:57:5 › Phase 3: CrowdSec Integration › Normal Request Handling › should allow normal requests with legitimate User-Agent
   [chromium] › phase3/crowdsec-integration.spec.ts:69:5 › Phase 3: CrowdSec Integration › Normal Request Handling › should allow requests without additional headers
   [chromium] › phase3/crowdsec-integration.spec.ts:74:5 › Phase 3: CrowdSec Integration › Normal Request Handling › should allow authenticated requests
   [chromium] › phase3/crowdsec-integration.spec.ts:90:5 › Phase 3: CrowdSec Integration › Suspicious Request Detection › requests with suspicious User-Agent should be flagged
   [chromium] › phase3/crowdsec-integration.spec.ts:103:5 › Phase 3: CrowdSec Integration › Suspicious Request Detection › rapid successive requests should be analyzed
   [chromium] › phase3/crowdsec-integration.spec.ts:117:5 › Phase 3: CrowdSec Integration › Suspicious Request Detection › requests with suspicious headers should be tracked
   [chromium] › phase3/crowdsec-integration.spec.ts:135:5 › Phase 3: CrowdSec Integration › Whitelist Functionality › test container IP should be whitelisted
   [chromium] › phase3/crowdsec-integration.spec.ts:143:5 › Phase 3: CrowdSec Integration › Whitelist Functionality › whitelisted IP should bypass CrowdSec even with suspicious patterns
   [chromium] › phase3/crowdsec-integration.spec.ts:155:5 › Phase 3: CrowdSec Integration › Whitelist Functionality › multiple requests from whitelisted IP should not trigger limit
   [chromium] › phase3/crowdsec-integration.spec.ts:175:5 › Phase 3: CrowdSec Integration › CrowdSec Decision Enforcement › CrowdSec decisions should be populated
   [chromium] › phase3/crowdsec-integration.spec.ts:182:5 › Phase 3: CrowdSec Integration › CrowdSec Decision Enforcement › if IP is banned, requests should return 403
   [chromium] › phase3/crowdsec-integration.spec.ts:203:5 › Phase 3: CrowdSec Integration › CrowdSec Decision Enforcement › ban should be lifted after duration expires
   [chromium] › phase3/crowdsec-integration.spec.ts:215:5 › Phase 3: CrowdSec Integration › Bot Detection Patterns › requests with scanning tools User-Agent should be flagged
   [chromium] › phase3/crowdsec-integration.spec.ts:230:5 › Phase 3: CrowdSec Integration › Bot Detection Patterns › requests with spoofed User-Agent should be analyzed
   [chromium] › phase3/crowdsec-integration.spec.ts:242:5 › Phase 3: CrowdSec Integration › Bot Detection Patterns › requests without User-Agent should be allowed
   [chromium] › phase3/crowdsec-integration.spec.ts:253:5 › Phase 3: CrowdSec Integration › Decision Cache Consistency › repeated requests should have consistent blocking
   [chromium] › phase3/crowdsec-integration.spec.ts:269:5 › Phase 3: CrowdSec Integration › Decision Cache Consistency › different endpoints should share ban list
   [chromium] › phase3/crowdsec-integration.spec.ts:291:5 › Phase 3: CrowdSec Integration › Edge Cases & Recovery › should handle high-volume heartbeat requests
   [chromium] › phase3/crowdsec-integration.spec.ts:304:5 › Phase 3: CrowdSec Integration › Edge Cases & Recovery › should handle mixed request patterns
   [chromium] › phase3/crowdsec-integration.spec.ts:328:5 › Phase 3: CrowdSec Integration › Edge Cases & Recovery › decision TTL should expire and remove old decisions
   [chromium] › phase3/crowdsec-integration.spec.ts:340:5 › Phase 3: CrowdSec Integration › CrowdSec Response Indicators › should not expose CrowdSec details in error response
   [chromium] › phase3/crowdsec-integration.spec.ts:351:5 › Phase 3: CrowdSec Integration › CrowdSec Response Indicators › blocked response should indicate rate limit or access denied
   [chromium] › phase3/rate-limiting.spec.ts:52:5 › Phase 3: Rate Limiting › Basic Rate Limit Enforcement › should allow up to 3 requests in 10s window
   [chromium] › phase3/rate-limiting.spec.ts:72:5 › Phase 3: Rate Limiting › Basic Rate Limit Enforcement › should return 429 when exceeding 3 requests in 10s window
   [chromium] › phase3/rate-limiting.spec.ts:90:5 › Phase 3: Rate Limiting › Basic Rate Limit Enforcement › should include rate limit headers in response
   [chromium] › phase3/rate-limiting.spec.ts:116:5 › Phase 3: Rate Limiting › Rate Limit Window Expiration & Reset › should reset rate limit after window expires
   [chromium] › phase3/rate-limiting.spec.ts:155:5 › Phase 3: Rate Limiting › Per-Endpoint Rate Limits › GET /api/v1/proxy-hosts should have rate limit
   [chromium] › phase3/rate-limiting.spec.ts:176:5 › Phase 3: Rate Limiting › Per-Endpoint Rate Limits › GET /api/v1/access-lists should have separate rate limit
   [chromium] › phase3/rate-limiting.spec.ts:202:5 › Phase 3: Rate Limiting › Anonymous Request Rate Limiting › should rate limit anonymous requests separately
   [chromium] › phase3/rate-limiting.spec.ts:230:5 › Phase 3: Rate Limiting › Retry-After Header › 429 response should include Retry-After header
   [chromium] › phase3/rate-limiting.spec.ts:249:5 › Phase 3: Rate Limiting › Retry-After Header › Retry-After should indicate reasonable wait time
   [chromium] › phase3/rate-limiting.spec.ts:282:5 › Phase 3: Rate Limiting › Rate Limit Consistency › same endpoint should share rate limit bucket
   [chromium] › phase3/rate-limiting.spec.ts:300:5 › Phase 3: Rate Limiting › Rate Limit Consistency › different HTTP methods on same endpoint should share limit
   [chromium] › phase3/rate-limiting.spec.ts:343:5 › Phase 3: Rate Limiting › Rate Limit Error Response Format › 429 response should be valid JSON
   [chromium] › phase3/rate-limiting.spec.ts:371:5 › Phase 3: Rate Limiting › Rate Limit Error Response Format › 429 response should not expose rate limit implementation details
   [chromium] › phase3/security-enforcement.spec.ts:54:5 › Phase 3: Security Enforcement › Bearer Token Validation › should reject request with missing bearer token (401)
   [chromium] › phase3/security-enforcement.spec.ts:61:5 › Phase 3: Security Enforcement › Bearer Token Validation › should reject request with invalid bearer token (401)
   [chromium] › phase3/security-enforcement.spec.ts:70:5 › Phase 3: Security Enforcement › Bearer Token Validation › should reject request with malformed authorization header (401)
   [chromium] › phase3/security-enforcement.spec.ts:79:5 › Phase 3: Security Enforcement › Bearer Token Validation › should reject request with empty bearer token (401)
   [chromium] › phase3/security-enforcement.spec.ts:88:5 › Phase 3: Security Enforcement › Bearer Token Validation › should reject request with NULL bearer token (401)
   [chromium] › phase3/security-enforcement.spec.ts:97:5 › Phase 3: Security Enforcement › Bearer Token Validation › should reject request with uppercase "bearer" keyword (case-sensitive)
   [chromium] › phase3/security-enforcement.spec.ts:112:5 › Phase 3: Security Enforcement › JWT Expiration & Auto-Refresh › should handle expired JWT gracefully
   [chromium] › phase3/security-enforcement.spec.ts:125:5 › Phase 3: Security Enforcement › JWT Expiration & Auto-Refresh › should return 401 for JWT with invalid signature
   [chromium] › phase3/security-enforcement.spec.ts:136:5 › Phase 3: Security Enforcement › JWT Expiration & Auto-Refresh › should return 401 for token missing required claims (sub, exp)
   [chromium] › phase3/security-enforcement.spec.ts:153:5 › Phase 3: Security Enforcement › CSRF Token Validation › POST request should include CSRF protection headers
   [chromium] › phase3/security-enforcement.spec.ts:171:5 › Phase 3: Security Enforcement › CSRF Token Validation › PUT request should validate CSRF token
   [chromium] › phase3/security-enforcement.spec.ts:184:5 › Phase 3: Security Enforcement › CSRF Token Validation › DELETE request without auth should return 401
   [chromium] › phase3/security-enforcement.spec.ts:194:5 › Phase 3: Security Enforcement › Request Timeout Handling › should handle slow endpoint with reasonable timeout
   [chromium] › phase3/security-enforcement.spec.ts:212:5 › Phase 3: Security Enforcement › Request Timeout Handling › should return proper error for unreachable endpoint
   [chromium] › phase3/security-enforcement.spec.ts:222:5 › Phase 3: Security Enforcement › Middleware Execution Order › authentication should be checked before authorization
   [chromium] › phase3/security-enforcement.spec.ts:230:5 › Phase 3: Security Enforcement › Middleware Execution Order › malformed request should be validated before processing
   [chromium] › phase3/security-enforcement.spec.ts:242:5 › Phase 3: Security Enforcement › Middleware Execution Order › rate limiting should be applied after authentication
   [chromium] › phase3/security-enforcement.spec.ts:262:5 › Phase 3: Security Enforcement › HTTP Header Validation › should accept valid Content-Type application/json
   [chromium] › phase3/security-enforcement.spec.ts:271:5 › Phase 3: Security Enforcement › HTTP Header Validation › should handle requests with no User-Agent header
   [chromium] › phase3/security-enforcement.spec.ts:276:5 › Phase 3: Security Enforcement › HTTP Header Validation › response should include security headers
   [chromium] › phase3/security-enforcement.spec.ts:293:5 › Phase 3: Security Enforcement › HTTP Method Validation › GET request should be allowed for read operations
   [chromium] › phase3/security-enforcement.spec.ts:303:5 › Phase 3: Security Enforcement › HTTP Method Validation › unsupported methods should return 405 or 401
   [chromium] › phase3/security-enforcement.spec.ts:319:5 › Phase 3: Security Enforcement › Error Response Format › 401 error should include error message
   [chromium] › phase3/security-enforcement.spec.ts:328:5 › Phase 3: Security Enforcement › Error Response Format › error response should not expose internal details
   [chromium] › phase4-integration/01-admin-user-e2e-workflow.spec.ts:25:3 › INT-001: Admin-User E2E Workflow › Complete user lifecycle: creation to resource access
   [chromium] › phase4-integration/01-admin-user-e2e-workflow.spec.ts:137:3 › INT-001: Admin-User E2E Workflow › Role change takes effect immediately on user refresh
   [chromium] › phase4-integration/01-admin-user-e2e-workflow.spec.ts:182:3 › INT-001: Admin-User E2E Workflow › Deleted user cannot login
   [chromium] › phase4-integration/01-admin-user-e2e-workflow.spec.ts:245:3 › INT-001: Admin-User E2E Workflow › Audit log records user lifecycle events
   [chromium] › phase4-integration/01-admin-user-e2e-workflow.spec.ts:287:3 › INT-001: Admin-User E2E Workflow › User cannot promote self to admin
   [chromium] › phase4-integration/01-admin-user-e2e-workflow.spec.ts:336:3 › INT-001: Admin-User E2E Workflow › Users see only their own data
   [chromium] › phase4-integration/01-admin-user-e2e-workflow.spec.ts:396:3 › INT-001: Admin-User E2E Workflow › Session isolation after logout and re-login
   [chromium] › phase4-integration/02-waf-ratelimit-interaction.spec.ts:44:3 › INT-002: WAF & Rate Limit Interaction › WAF blocks malicious SQL injection payload
   [chromium] › phase4-integration/02-waf-ratelimit-interaction.spec.ts:84:3 › INT-002: WAF & Rate Limit Interaction › Rate limiting blocks requests exceeding threshold
   [chromium] › phase4-integration/02-waf-ratelimit-interaction.spec.ts:134:3 › INT-002: WAF & Rate Limit Interaction › WAF enforces regardless of rate limit status
   [chromium] › phase4-integration/02-waf-ratelimit-interaction.spec.ts:192:3 › INT-002: WAF & Rate Limit Interaction › Malicious request gets 403 (WAF) not 429 (rate limit)
   [chromium] › phase4-integration/02-waf-ratelimit-interaction.spec.ts:247:3 › INT-002: WAF & Rate Limit Interaction › Clean request gets 429 when rate limit exceeded
   [chromium] › phase4-integration/03-acl-waf-layering.spec.ts:64:3 › INT-003: ACL & WAF Layering › Regular user cannot bypass WAF on authorized proxy
   [chromium] › phase4-integration/03-acl-waf-layering.spec.ts:131:3 › INT-003: ACL & WAF Layering › WAF blocks malicious requests from all user roles
   [chromium] › phase4-integration/03-acl-waf-layering.spec.ts:211:3 › INT-003: ACL & WAF Layering › Both admin and user roles subject to WAF protection
   [chromium] › phase4-integration/03-acl-waf-layering.spec.ts:289:3 › INT-003: ACL & WAF Layering › ACL restricts access beyond WAF protection
   [chromium] › phase4-integration/04-auth-middleware-cascade.spec.ts:43:3 › INT-004: Auth Middleware Cascade › Request without token gets 401 Unauthorized
   [chromium] › phase4-integration/04-auth-middleware-cascade.spec.ts:75:3 › INT-004: Auth Middleware Cascade › Request with invalid token gets 401 Unauthorized
   [chromium] › phase4-integration/04-auth-middleware-cascade.spec.ts:123:3 › INT-004: Auth Middleware Cascade › Valid token passes ACL validation
   [chromium] › phase4-integration/04-auth-middleware-cascade.spec.ts:158:3 › INT-004: Auth Middleware Cascade › Valid token passes WAF validation
   [chromium] › phase4-integration/04-auth-middleware-cascade.spec.ts:201:3 › INT-004: Auth Middleware Cascade › Valid token passes rate limiting validation
   [chromium] › phase4-integration/04-auth-middleware-cascade.spec.ts:251:3 › INT-004: Auth Middleware Cascade › Valid token passes auth, ACL, WAF, and rate limiting
   [chromium] › phase4-integration/05-data-consistency.spec.ts:64:3 › INT-005: Data Consistency › Data created via UI is properly stored and readable via API
   [chromium] › phase4-integration/05-data-consistency.spec.ts:111:3 › INT-005: Data Consistency › Data modified via API is reflected in UI
   [chromium] › phase4-integration/05-data-consistency.spec.ts:172:3 › INT-005: Data Consistency › Data deleted via UI is removed from API
   [chromium] › phase4-integration/05-data-consistency.spec.ts:224:3 › INT-005: Data Consistency › Concurrent modifications do not cause data corruption
   [chromium] › phase4-integration/05-data-consistency.spec.ts:297:3 › INT-005: Data Consistency › Failed transaction prevents partial data updates
   [chromium] › phase4-integration/05-data-consistency.spec.ts:339:3 › INT-005: Data Consistency › Database constraints prevent invalid data
   [chromium] › phase4-integration/05-data-consistency.spec.ts:377:3 › INT-005: Data Consistency › Client-side and server-side validation consistent
   [chromium] › phase4-integration/05-data-consistency.spec.ts:410:3 › INT-005: Data Consistency › Pagination and sorting produce consistent results
   [chromium] › phase4-integration/06-long-running-operations.spec.ts:62:3 › INT-006: Long-Running Operations › Backup creation does not block other operations
   [chromium] › phase4-integration/06-long-running-operations.spec.ts:110:3 › INT-006: Long-Running Operations › UI remains responsive while backup in progress
   [chromium] › phase4-integration/06-long-running-operations.spec.ts:163:3 › INT-006: Long-Running Operations › Proxy creation independent of backup operation
   [chromium] › phase4-integration/06-long-running-operations.spec.ts:213:3 › INT-006: Long-Running Operations › Authentication completes quickly even during background tasks
   [chromium] › phase4-integration/06-long-running-operations.spec.ts:266:3 › INT-006: Long-Running Operations › Long-running task completion can be verified
   [chromium] › phase4-integration/07-multi-component-workflows.spec.ts:62:3 › INT-007: Multi-Component Workflows › WAF enforcement applies to newly created proxy
   [chromium] › phase4-integration/07-multi-component-workflows.spec.ts:117:3 › INT-007: Multi-Component Workflows › User with proxy creation role can create and manage proxies
   [chromium] › phase4-integration/07-multi-component-workflows.spec.ts:171:3 › INT-007: Multi-Component Workflows › Backup restore recovers deleted user data
   [chromium] › phase4-integration/07-multi-component-workflows.spec.ts:258:3 › INT-007: Multi-Component Workflows › Security modules apply to subsequently created resources
   [chromium] › phase4-integration/07-multi-component-workflows.spec.ts:328:3 › INT-007: Multi-Component Workflows › Security enforced even on previously created resources
   [chromium] › phase4-uat/01-admin-onboarding.spec.ts:21:3 › UAT-001: Admin Onboarding & Setup › Admin logs in with valid credentials
   [chromium] › phase4-uat/01-admin-onboarding.spec.ts:53:3 › UAT-001: Admin Onboarding & Setup › Dashboard displays after login
   [chromium] › phase4-uat/01-admin-onboarding.spec.ts:77:3 › UAT-001: Admin Onboarding & Setup › System settings accessible from menu
   [chromium] › phase4-uat/01-admin-onboarding.spec.ts:107:3 › UAT-001: Admin Onboarding & Setup › Emergency token can be generated
   [chromium] › phase4-uat/01-admin-onboarding.spec.ts:147:3 › UAT-001: Admin Onboarding & Setup › Dashboard loads with encryption key management
   [chromium] › phase4-uat/01-admin-onboarding.spec.ts:171:3 › UAT-001: Admin Onboarding & Setup › Navigation menu items all functional
   [chromium] › phase4-uat/01-admin-onboarding.spec.ts:200:3 › UAT-001: Admin Onboarding & Setup › Logout clears session
   [chromium] › phase4-uat/01-admin-onboarding.spec.ts:242:3 › UAT-001: Admin Onboarding & Setup › Re-login after logout successful
   [chromium] › phase4-uat/02-user-management.spec.ts:51:3 › UAT-002: User Management › Create new user with all fields
   [chromium] › phase4-uat/02-user-management.spec.ts:105:3 › UAT-002: User Management › Assign roles to user
   [chromium] › phase4-uat/02-user-management.spec.ts:162:3 › UAT-002: User Management › Delete user account
   [chromium] › phase4-uat/02-user-management.spec.ts:209:3 › UAT-002: User Management › User login with restricted role
   [chromium] › phase4-uat/02-user-management.spec.ts:270:3 › UAT-002: User Management › User cannot access unauthorized admin resources
   [chromium] › phase4-uat/02-user-management.spec.ts:294:3 › UAT-002: User Management › Guest role has minimal access
   [chromium] › phase4-uat/02-user-management.spec.ts:346:3 › UAT-002: User Management › Modify user email
   [chromium] › phase4-uat/02-user-management.spec.ts:392:3 › UAT-002: User Management › Reset user password
   [chromium] › phase4-uat/02-user-management.spec.ts:457:3 › UAT-002: User Management › Search users by email
   [chromium] › phase4-uat/02-user-management.spec.ts:490:3 › UAT-002: User Management › User list pagination works with many users
   [chromium] › phase4-uat/03-proxy-host-management.spec.ts:48:3 › UAT-003: Proxy Host Management › Create proxy host with domain
   [chromium] › phase4-uat/03-proxy-host-management.spec.ts:86:3 › UAT-003: Proxy Host Management › Edit proxy host settings
   [chromium] › phase4-uat/03-proxy-host-management.spec.ts:136:3 › UAT-003: Proxy Host Management › Delete proxy host
   [chromium] › phase4-uat/03-proxy-host-management.spec.ts:180:3 › UAT-003: Proxy Host Management › Configure SSL/TLS certificate on proxy
   [chromium] › phase4-uat/03-proxy-host-management.spec.ts:218:3 › UAT-003: Proxy Host Management › Proxy routes traffic to backend
   [chromium] › phase4-uat/03-proxy-host-management.spec.ts:249:3 › UAT-003: Proxy Host Management › Access list can be applied to proxy
   [chromium] › phase4-uat/03-proxy-host-management.spec.ts:285:3 › UAT-003: Proxy Host Management › WAF can be applied to proxy
   [chromium] › phase4-uat/03-proxy-host-management.spec.ts:320:3 › UAT-003: Proxy Host Management › Rate limit can be applied to proxy
   [chromium] › phase4-uat/03-proxy-host-management.spec.ts:354:3 › UAT-003: Proxy Host Management › Proxy creation validation for invalid patterns
   [chromium] › phase4-uat/03-proxy-host-management.spec.ts:380:3 › UAT-003: Proxy Host Management › Proxy domain field is required
   [chromium] › phase4-uat/03-proxy-host-management.spec.ts:412:3 › UAT-003: Proxy Host Management › Proxy statistics display
   [chromium] › phase4-uat/03-proxy-host-management.spec.ts:451:3 › UAT-003: Proxy Host Management › Disable proxy temporarily
   [chromium] › phase4-uat/04-security-configuration.spec.ts:18:3 › UAT-004: Security Configuration › Enable Cerberus ACL module
   [chromium] › phase4-uat/04-security-configuration.spec.ts:58:3 › UAT-004: Security Configuration › Configure ACL whitelist rule
   [chromium] › phase4-uat/04-security-configuration.spec.ts:98:3 › UAT-004: Security Configuration › Enable Coraza WAF module
   [chromium] › phase4-uat/04-security-configuration.spec.ts:130:3 › UAT-004: Security Configuration › Configure WAF sensitivity level
   [chromium] › phase4-uat/04-security-configuration.spec.ts:158:3 › UAT-004: Security Configuration › Enable rate limiting module
   [chromium] › phase4-uat/04-security-configuration.spec.ts:190:3 › UAT-004: Security Configuration › Configure rate limit threshold
   [chromium] › phase4-uat/04-security-configuration.spec.ts:221:3 › UAT-004: Security Configuration › Enable CrowdSec integration
   [chromium] › phase4-uat/04-security-configuration.spec.ts:257:3 › UAT-004: Security Configuration › Malicious payload blocked by WAF
   [chromium] › phase4-uat/04-security-configuration.spec.ts:300:3 › UAT-004: Security Configuration › Security dashboard displays module status
   [chromium] › phase4-uat/04-security-configuration.spec.ts:330:3 › UAT-004: Security Configuration › Security audit logs recorded in system
   [chromium] › phase4-uat/05-domain-dns-management.spec.ts:18:3 › UAT-005: Domain & DNS Management › Add domain to system
   [chromium] › phase4-uat/05-domain-dns-management.spec.ts:53:3 › UAT-005: Domain & DNS Management › View DNS records for domain
   [chromium] › phase4-uat/05-domain-dns-management.spec.ts:78:3 › UAT-005: Domain & DNS Management › Add DNS provider configuration
   [chromium] › phase4-uat/05-domain-dns-management.spec.ts:119:3 › UAT-005: Domain & DNS Management › Verify domain ownership
   [chromium] › phase4-uat/05-domain-dns-management.spec.ts:144:3 › UAT-005: Domain & DNS Management › Renew SSL certificate for domain
   [chromium] › phase4-uat/05-domain-dns-management.spec.ts:178:3 › UAT-005: Domain & DNS Management › View domain statistics and status
   [chromium] › phase4-uat/05-domain-dns-management.spec.ts:211:3 › UAT-005: Domain & DNS Management › Disable domain temporarily
   [chromium] › phase4-uat/05-domain-dns-management.spec.ts:239:3 › UAT-005: Domain & DNS Management › Export domains configuration as JSON
   [chromium] › phase4-uat/06-monitoring-audit.spec.ts:18:3 › UAT-006: Monitoring & Audit › Real-time logs display in monitoring
   [chromium] › phase4-uat/06-monitoring-audit.spec.ts:46:3 › UAT-006: Monitoring & Audit › Filter logs by level/type
   [chromium] › phase4-uat/06-monitoring-audit.spec.ts:70:3 › UAT-006: Monitoring & Audit › Search logs by keyword
   [chromium] › phase4-uat/06-monitoring-audit.spec.ts:93:3 › UAT-006: Monitoring & Audit › Export logs to CSV file
   [chromium] › phase4-uat/06-monitoring-audit.spec.ts:121:3 › UAT-006: Monitoring & Audit › Pagination works with large log datasets
   [chromium] › phase4-uat/06-monitoring-audit.spec.ts:147:3 › UAT-006: Monitoring & Audit › Audit trail displays user actions
   [chromium] › phase4-uat/06-monitoring-audit.spec.ts:176:3 › UAT-006: Monitoring & Audit › Security events recorded in audit log
   [chromium] › phase4-uat/06-monitoring-audit.spec.ts:203:3 › UAT-006: Monitoring & Audit › Log retention respects configured policy
   [chromium] › phase4-uat/07-backup-recovery.spec.ts:18:3 › UAT-007: Backup & Recovery › Create manual backup
   [chromium] › phase4-uat/07-backup-recovery.spec.ts:53:3 › UAT-007: Backup & Recovery › Schedule automatic backups
   [chromium] › phase4-uat/07-backup-recovery.spec.ts:99:3 › UAT-007: Backup & Recovery › Download backup file
   [chromium] › phase4-uat/07-backup-recovery.spec.ts:130:3 › UAT-007: Backup & Recovery › Restore from backup
   [chromium] › phase4-uat/07-backup-recovery.spec.ts:155:3 › UAT-007: Backup & Recovery › Data integrity verified after restore
   [chromium] › phase4-uat/07-backup-recovery.spec.ts:182:3 › UAT-007: Backup & Recovery › Delete backup file
   [chromium] › phase4-uat/07-backup-recovery.spec.ts:215:3 › UAT-007: Backup & Recovery › Backup files are encrypted
   [chromium] › phase4-uat/07-backup-recovery.spec.ts:245:3 › UAT-007: Backup & Recovery › Backup restoration with password protection
   [chromium] › phase4-uat/07-backup-recovery.spec.ts:269:3 › UAT-007: Backup & Recovery › Backup retention policy enforced
   [chromium] › phase4-uat/08-emergency-operations.spec.ts:18:3 › UAT-008: Emergency & Break-Glass Operations › Emergency token enables break-glass access
   [chromium] › phase4-uat/08-emergency-operations.spec.ts:42:3 › UAT-008: Emergency & Break-Glass Operations › Break-glass recovery brings system to safe state
Total: 176 tests in 20 files

=============================== Coverage summary ===============================
Statements   : Unknown% ( 0/0 )
Branches     : Unknown% ( 0/0 )
Functions    : Unknown% ( 0/0 )
Lines        : Unknown% ( 0/0 )
================================================================================
```

## Timeout Analysis

- Test-level timeout hit: yes
  - tests/settings/notifications.spec.ts:545:5 (Test timeout of 60000ms exceeded)
- Expect timeouts hit: yes
  - 10s expect timeout for update indicator
  - 5s expect timeout for delete button
  - 5s expect timeout for checkbox not-to-be-checked

## Hypotheses (H1-H6 from spec)

H1 - Workflow/job timeout smaller than expected
- Not supported: job completed in ~8.5m and reported test failures; no job timeout messages.

H2 - Runner preemption/connection loss
- Not supported: job logs show clean Playwright failure output and summary; no runner lost/cancel messages.

H3 - Container died or unhealthy
- Not supported: docker logs show normal 200 responses around 13:06:40-13:06:48; no crashes or 5xx at 13:06:49.

H4 - Playwright/Node OOM kill
- Not supported: no "Killed" or OOM messages in job logs; test failures are explicit assertions/timeouts.

H5 - Script-level early timeout (explicit timeout wrapper)
- Not supported: no wrapper timeout or kill signals; command completed with reported failures.

H6 - Misconfigured timeout units
- Not supported: test timeouts are 60s as configured; no evidence of unit mismatch.

## Root Cause Hypotheses (Test-Level)

- UI state not updated or stale after edits (update toast/label not appearing in time).
- Provider URL validation step may close the page or navigate unexpectedly, causing locator.clear() on a closed context.
- Template deletion locator relies on a "pre" element with hard-coded text; likely brittle when list changes or async data loads late.
- Event selection state may persist from prior tests; data cleanup or state reset may be incomplete.

## Recommended Test-Level Remediations

1) P0 - Update-success waits
   - Replace brittle toast/text OR chain with explicit wait for backend response or a deterministic UI state (e.g., wait for provider row text to update, or wait for a success toast with a stable data-testid).
   - Increase expect timeout only if UX requires it; prefer waiting on network response.

2) P1 - Provider URL validation flow
   - Remove page.waitForTimeout(300); replace with a wait for validation result or server response.
   - Guard against page/context closure by waiting for the input to be attached and visible before clear/fill.

3) P1 - External template delete
   - Use a stable data-testid on the template row or delete button to avoid selector fragility.
   - Add a wait for list to render (or for the template row to be visible) before clicking.

4) P1 - Event selections persistence
   - Reset notification event settings in test setup or use a data cleanup helper after each test.
   - Verify saved state by reloading the page and waiting for settings fetch to complete before asserting checkboxes.

5) P2 - Retry strategy
   - Retries already executed (2 retries). Prefer fixing wait logic over increasing retries.
   - If temporary mitigation is needed, consider raising per-test timeout for URL validation step only.

## Evidence Correlation (Job/Shard Timestamps)

- Job start: 2026-02-10T12:57:37Z (runner initialization begins)
- Shard start: 2026-02-10T12:58:19Z ("Chromium Non-Security Tests - Shard 3/4" start banner)
- Test run begins: 2026-02-10T12:58:24Z ("Running 115 tests")
- Failures logged: 2026-02-10T13:06:49Z
- Shard complete: 2026-02-10T13:06:49Z ("Chromium Shard 3 Complete | Duration: 510s")
- Job end: 2026-02-10T13:06:54Z (post-job cleanup)

## Complete Reproduction Steps (CI-Equivalent)

1) Rebuild E2E image (CI alignment):

```bash
.github/skills/scripts/skill-runner.sh docker-rebuild-e2e
```

2) Start E2E environment:

```bash
docker compose -f .docker/compose/docker-compose.playwright-ci.yml up -d
```

3) Environment variables (match CI):

```bash
export PLAYWRIGHT_BASE_URL=http://127.0.0.1:8080
export CHARON_EMERGENCY_TOKEN=changeme
export DEBUG=charon:*,charon-test:*
export PLAYWRIGHT_DEBUG=1
export CI_LOG_LEVEL=verbose
```

4) Exact shard reproduction command (CI flags):

```bash
npx playwright test \
   --project=chromium \
   --shard=3/4 \
   tests/core \
   tests/dns-provider-crud.spec.ts \
   tests/dns-provider-types.spec.ts \
   tests/integration \
   tests/manual-dns-provider.spec.ts \
   tests/monitoring \
   tests/settings \
   tests/tasks
```

5) Log collection after failure:

```bash
docker compose -f .docker/compose/docker-compose.playwright-ci.yml logs > /tmp/docker-logs-chromium-shard-3.txt 2>&1
cp /tmp/playwright-chromium.log /tmp/playwright-chromium-shard-3.log
```

## Exact Reproduction Command (from CI)

npx playwright test \
  --project=chromium \
  --shard=3/4 \
  tests/core \
  tests/dns-provider-crud.spec.ts \
  tests/dns-provider-types.spec.ts \
  tests/integration \
  tests/manual-dns-provider.spec.ts \
  tests/monitoring \
  tests/settings \
  tests/tasks

Focused repro example:

npx playwright test tests/settings/notifications.spec.ts -g "should validate provider URL" --project=chromium
