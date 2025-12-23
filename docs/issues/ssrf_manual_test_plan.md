# SSRF Protection Manual Test Plan

**Purpose**: Manual testing plan for validating SSRF protection in production-like environment.

**Test Date**: _____________
**Tester**: _____________
**Environment**: _____________
**Charon Version**: _____________

---

## Prerequisites

Before beginning tests, ensure:

- [ ] Charon deployed in test environment
- [ ] Admin access to Charon configuration interface
- [ ] Network access to test external webhooks
- [ ] Access to test webhook receiver (e.g., <https://webhook.site>)
- [ ] `curl` or similar HTTP client available
- [ ] Ability to view Charon server logs

---

## Test Environment Setup

### Required Tools

1. **Webhook Testing Service**:
   - Webhook.site: <https://webhook.site> (get unique URL)
   - RequestBin: <https://requestbin.com>
   - Discord webhook: <https://discord.com/developers/docs/resources/webhook>

2. **HTTP Client**:
   ```bash
   # Verify curl is available
   curl --version
   ```

3. **Log Access**:
   ```bash
   # View Charon logs
   docker logs charon --tail=50 --follow
   ```

---

## Test Case Format

Each test case includes:

- **Objective**: What security control is being tested
- **Steps**: Detailed instructions
- **Expected Result**: What should happen (✅)
- **Actual Result**: Record what actually happened
- **Pass/Fail**: Mark after completion
- **Notes**: Any observations or issues

---

## Test Suite 1: Valid External Webhooks

### TC-001: Valid HTTPS Webhook

**Objective**: Verify legitimate HTTPS webhooks work correctly

**Steps**:
1. Navigate to Security Settings → Notifications
2. Configure webhook: `https://webhook.site/<your-unique-id>`
3. Click **Save**
4. Trigger security event (e.g., create test ACL rule)
5. Check webhook.site for received event

**Expected Result**: ✅ Webhook successfully delivered, no errors in logs

**Actual Result**: _____________

**Pass/Fail**: [ ] Pass [ ] Fail

**Notes**:
```

```

---

### TC-002: Valid HTTP Webhook (Non-Production)

**Objective**: Verify HTTP webhooks work when explicitly allowed

**Steps**:
1. Navigate to Security Settings → Notifications
2. Configure webhook: `http://webhook.site/<your-unique-id>`
3. Click **Save**
4. Trigger security event
5. Check webhook receiver

**Expected Result**: ✅ Webhook accepted (if HTTP allowed), or ❌ Rejected with "HTTP is not allowed, use HTTPS"

**Actual Result**: _____________

**Pass/Fail**: [ ] Pass [ ] Fail

**Notes**:
```

```

---

### TC-003: Slack Webhook Format

**Objective**: Verify production webhook services work

**Steps**:
1. Create Slack incoming webhook at <https://api.slack.com/messaging/webhooks>
2. Configure webhook in Charon: `https://hooks.slack.com/services/T00/B00/XXX`
3. Save configuration
4. Trigger security event
5. Check Slack channel for notification

**Expected Result**: ✅ Notification appears in Slack

**Actual Result**: _____________

**Pass/Fail**: [ ] Pass [ ] Fail

**Notes**:
```

```

---

### TC-004: Discord Webhook Format

**Objective**: Verify Discord integration works

**Steps**:
1. Create Discord webhook in server settings
2. Configure webhook in Charon: `https://discord.com/api/webhooks/123456/abcdef`
3. Save configuration
4. Trigger security event
5. Check Discord channel

**Expected Result**: ✅ Notification appears in Discord

**Actual Result**: _____________

**Pass/Fail**: [ ] Pass [ ] Fail

**Notes**:
```

```

---

## Test Suite 2: Private IP Rejection

### TC-005: Class A Private Network (10.0.0.0/8)

**Objective**: Verify RFC 1918 Class A blocking

**Steps**:
1. Attempt to configure webhook: `http://10.0.0.1/webhook`
2. Click **Save**
3. Observe error message

**Expected Result**: ❌ Error: "URL resolves to a private IP address (blocked for security)"

**Actual Result**: _____________

**Pass/Fail**: [ ] Pass [ ] Fail

**Notes**:
```

```

---

### TC-006: Class B Private Network (172.16.0.0/12)

**Objective**: Verify RFC 1918 Class B blocking

**Steps**:
1. Attempt to configure webhook: `http://172.16.0.1/admin`
2. Click **Save**
3. Observe error message

**Expected Result**: ❌ Error: "URL resolves to a private IP address (blocked for security)"

**Actual Result**: _____________

**Pass/Fail**: [ ] Pass [ ] Fail

**Notes**:
```

```

---

### TC-007: Class C Private Network (192.168.0.0/16)

**Objective**: Verify RFC 1918 Class C blocking

**Steps**:
1. Attempt to configure webhook: `http://192.168.1.1/`
2. Click **Save**
3. Observe error message

**Expected Result**: ❌ Error: "URL resolves to a private IP address (blocked for security)"

**Actual Result**: _____________

**Pass/Fail**: [ ] Pass [ ] Fail

**Notes**:
```

```

---

### TC-008: Private IP with Port

**Objective**: Verify port numbers don't bypass protection

**Steps**:
1. Attempt to configure webhook: `http://192.168.1.100:8080/webhook`
2. Click **Save**
3. Observe error message

**Expected Result**: ❌ Error: "URL resolves to a private IP address (blocked for security)"

**Actual Result**: _____________

**Pass/Fail**: [ ] Pass [ ] Fail

**Notes**:
```

```

---

## Test Suite 3: Cloud Metadata Endpoints

### TC-009: AWS Metadata Endpoint

**Objective**: Verify AWS metadata service is blocked

**Steps**:
1. Attempt to configure webhook: `http://169.254.169.254/latest/meta-data/`
2. Click **Save**
3. Observe error message
4. Check logs for HIGH severity SSRF attempt

**Expected Result**:
- ❌ Configuration rejected
- ✅ Log entry: `severity=HIGH event=ssrf_blocked`

**Actual Result**: _____________

**Pass/Fail**: [ ] Pass [ ] Fail

**Notes**:
```

```

---

### TC-010: GCP Metadata Endpoint

**Objective**: Verify GCP metadata service is blocked

**Steps**:
1. Attempt to configure webhook: `http://metadata.google.internal/computeMetadata/v1/`
2. Click **Save**
3. Observe error message

**Expected Result**: ❌ Error: "URL resolves to a private IP address" or "DNS lookup failed"

**Actual Result**: _____________

**Pass/Fail**: [ ] Pass [ ] Fail

**Notes**:
```

```

---

### TC-011: Azure Metadata Endpoint

**Objective**: Verify Azure metadata service is blocked

**Steps**:
1. Attempt to configure webhook: `http://169.254.169.254/metadata/instance?api-version=2021-02-01`
2. Click **Save**
3. Observe error message

**Expected Result**: ❌ Error: "URL resolves to a private IP address (blocked for security)"

**Actual Result**: _____________

**Pass/Fail**: [ ] Pass [ ] Fail

**Notes**:
```

```

---

## Test Suite 4: Loopback Addresses

### TC-012: IPv4 Loopback (127.0.0.1)

**Objective**: Verify localhost blocking (unless explicitly allowed)

**Steps**:
1. Attempt to configure webhook: `http://127.0.0.1:8080/internal`
2. Click **Save**
3. Observe error message

**Expected Result**: ❌ Error: "localhost URLs are not allowed (blocked for security)"

**Actual Result**: _____________

**Pass/Fail**: [ ] Pass [ ] Fail

**Notes**:
```

```

---

### TC-013: Localhost Hostname

**Objective**: Verify `localhost` keyword blocking

**Steps**:
1. Attempt to configure webhook: `http://localhost/admin`
2. Click **Save**
3. Observe error message

**Expected Result**: ❌ Error: "localhost URLs are not allowed (blocked for security)"

**Actual Result**: _____________

**Pass/Fail**: [ ] Pass [ ] Fail

**Notes**:
```

```

---

### TC-014: IPv6 Loopback (::1)

**Objective**: Verify IPv6 loopback blocking

**Steps**:
1. Attempt to configure webhook: `http://[::1]/webhook`
2. Click **Save**
3. Observe error message

**Expected Result**: ❌ Error: "URL resolves to a private IP address (blocked for security)"

**Actual Result**: _____________

**Pass/Fail**: [ ] Pass [ ] Fail

**Notes**:
```

```

---

## Test Suite 5: Protocol Validation

### TC-015: File Protocol

**Objective**: Verify file:// protocol is blocked

**Steps**:
1. Attempt to configure webhook: `file:///etc/passwd`
2. Click **Save**
3. Observe error message

**Expected Result**: ❌ Error: "URL must use HTTP or HTTPS"

**Actual Result**: _____________

**Pass/Fail**: [ ] Pass [ ] Fail

**Notes**:
```

```

---

### TC-016: FTP Protocol

**Objective**: Verify ftp:// protocol is blocked

**Steps**:
1. Attempt to configure webhook: `ftp://internal-server.local/upload/`
2. Click **Save**
3. Observe error message

**Expected Result**: ❌ Error: "URL must use HTTP or HTTPS"

**Actual Result**: _____________

**Pass/Fail**: [ ] Pass [ ] Fail

**Notes**:
```

```

---

### TC-017: Gopher Protocol

**Objective**: Verify gopher:// protocol is blocked

**Steps**:
1. Attempt to configure webhook: `gopher://internal:70/`
2. Click **Save**
3. Observe error message

**Expected Result**: ❌ Error: "URL must use HTTP or HTTPS"

**Actual Result**: _____________

**Pass/Fail**: [ ] Pass [ ] Fail

**Notes**:
```

```

---

### TC-018: Data URL

**Objective**: Verify data: scheme is blocked

**Steps**:
1. Attempt to configure webhook: `data:text/html,<script>alert(1)</script>`
2. Click **Save**
3. Observe error message

**Expected Result**: ❌ Error: "URL must use HTTP or HTTPS"

**Actual Result**: _____________

**Pass/Fail**: [ ] Pass [ ] Fail

**Notes**:
```

```

---

## Test Suite 6: URL Testing Endpoint

### TC-019: Test Valid Public URL

**Objective**: Verify URL test endpoint works for legitimate URLs

**Steps**:
1. Navigate to **System Settings** → **URL Testing** (or use API)
2. Test URL: `https://api.github.com`
3. Submit test
4. Observe result

**Expected Result**: ✅ "URL is reachable" with latency in milliseconds

**Actual Result**: _____________

**Pass/Fail**: [ ] Pass [ ] Fail

**Notes**:
```

```

---

### TC-020: Test Private IP via URL Testing

**Objective**: Verify URL test endpoint also has SSRF protection

**Steps**:
1. Navigate to URL Testing
2. Test URL: `http://192.168.1.1`
3. Submit test
4. Observe error

**Expected Result**: ❌ Error: "URL resolves to a private IP address (blocked for security)"

**Actual Result**: _____________

**Pass/Fail**: [ ] Pass [ ] Fail

**Notes**:
```

```

---

### TC-021: Test Non-Existent Domain

**Objective**: Verify DNS resolution failure handling

**Steps**:
1. Test URL: `https://this-domain-does-not-exist-12345.com`
2. Submit test
3. Observe error

**Expected Result**: ❌ Error: "DNS lookup failed" or "connection timeout"

**Actual Result**: _____________

**Pass/Fail**: [ ] Pass [ ] Fail

**Notes**:
```

```

---

## Test Suite 7: CrowdSec Hub Sync

### TC-022: Official CrowdSec Hub Domain

**Objective**: Verify CrowdSec hub sync works with official domain

**Steps**:
1. Navigate to **Security** → **CrowdSec**
2. Enable CrowdSec (if not already enabled)
3. Trigger hub sync (or wait for automatic sync)
4. Check logs for hub update success

**Expected Result**: ✅ Hub sync completes successfully

**Actual Result**: _____________

**Pass/Fail**: [ ] Pass [ ] Fail

**Notes**:
```

```

---

### TC-023: Invalid CrowdSec Hub Domain

**Objective**: Verify custom hub URLs are validated

**Steps**:
1. Attempt to configure custom hub URL: `http://malicious-hub.evil.com`
2. Trigger hub sync
3. Observe error in logs

**Expected Result**: ❌ Hub sync fails with validation error

**Actual Result**: _____________

**Pass/Fail**: [ ] Pass [ ] Fail

**Notes**:
```
(This test may require configuration file modification)
```

---

## Test Suite 8: Update Service

### TC-024: GitHub Update Check

**Objective**: Verify update service uses validated GitHub URLs

**Steps**:
1. Navigate to **System** → **Updates** (if available in UI)
2. Click **Check for Updates**
3. Observe success or error
4. Check logs for GitHub API request

**Expected Result**: ✅ Update check completes (no SSRF vulnerability)

**Actual Result**: _____________

**Pass/Fail**: [ ] Pass [ ] Fail

**Notes**:
```

```

---

## Test Suite 9: Error Message Validation

### TC-025: Generic Error Messages

**Objective**: Verify error messages don't leak internal information

**Steps**:
1. Attempt various blocked URLs from previous tests
2. Record exact error messages shown to user
3. Verify no internal IPs, hostnames, or network topology revealed

**Expected Result**: ✅ Generic errors like "URL resolves to a private IP address (blocked for security)"

**Actual Result**: _____________

**Pass/Fail**: [ ] Pass [ ] Fail

**Notes**:
```

```

---

### TC-026: Log Detail vs User Error

**Objective**: Verify logs contain more detail than user-facing errors

**Steps**:
1. Attempt blocked URL: `http://192.168.1.100/admin`
2. Check user-facing error message
3. Check server logs for detailed information

**Expected Result**:
- User sees: "URL resolves to a private IP address (blocked for security)"
- Logs show: `severity=HIGH url=http://192.168.1.100/admin resolved_ip=192.168.1.100`

**Actual Result**: _____________

**Pass/Fail**: [ ] Pass [ ] Fail

**Notes**:
```

```

---

## Test Suite 10: Integration Testing

### TC-027: End-to-End Webhook Flow

**Objective**: Verify complete webhook notification flow with SSRF protection

**Steps**:
1. Configure valid webhook: `https://webhook.site/<unique-id>`
2. Trigger CrowdSec block event (simulate attack)
3. Verify notification received at webhook.site
4. Check logs for successful webhook delivery

**Expected Result**:
- ✅ Webhook configured without errors
- ✅ Security event triggered
- ✅ Notification delivered successfully
- ✅ Logs show `Webhook notification sent successfully`

**Actual Result**: _____________

**Pass/Fail**: [ ] Pass [ ] Fail

**Notes**:
```

```

---

### TC-028: Configuration Persistence

**Objective**: Verify webhook validation persists across restarts

**Steps**:
1. Configure valid webhook: `https://webhook.site/<unique-id>`
2. Restart Charon container: `docker restart charon`
3. Trigger security event
4. Verify notification still works

**Expected Result**: ✅ Webhook survives restart and continues to function

**Actual Result**: _____________

**Pass/Fail**: [ ] Pass [ ] Fail

**Notes**:
```

```

---

### TC-029: Multiple Webhook Configurations

**Objective**: Verify SSRF protection applies to all webhook types

**Steps**:
1. Configure security notification webhook (valid)
2. Configure custom webhook notification (valid)
3. Attempt to add webhook with private IP (blocked)
4. Verify both valid webhooks work, blocked one rejected

**Expected Result**:
- ✅ Valid webhooks accepted
- ❌ Private IP webhook rejected
- ✅ Both valid webhooks receive notifications

**Actual Result**: _____________

**Pass/Fail**: [ ] Pass [ ] Fail

**Notes**:
```

```

---

### TC-030: Admin-Only Access Control

**Objective**: Verify URL testing requires admin privileges

**Steps**:
1. Log out of admin account
2. Log in as non-admin user (if available)
3. Attempt to access URL testing endpoint
4. Observe access denied error

**Expected Result**: ❌ 403 Forbidden: "Admin access required"

**Actual Result**: _____________

**Pass/Fail**: [ ] Pass [ ] Fail

**Notes**:
```

```

---

## Test Summary

### Results Overview

| Test Suite | Total Tests | Passed | Failed | Skipped |
|------------|-------------|--------|--------|---------|
| Valid External Webhooks | 4 | ___ | ___ | ___ |
| Private IP Rejection | 4 | ___ | ___ | ___ |
| Cloud Metadata Endpoints | 3 | ___ | ___ | ___ |
| Loopback Addresses | 3 | ___ | ___ | ___ |
| Protocol Validation | 4 | ___ | ___ | ___ |
| URL Testing Endpoint | 3 | ___ | ___ | ___ |
| CrowdSec Hub Sync | 2 | ___ | ___ | ___ |
| Update Service | 1 | ___ | ___ | ___ |
| Error Message Validation | 2 | ___ | ___ | ___ |
| Integration Testing | 4 | ___ | ___ | ___ |
| **TOTAL** | **30** | **___** | **___** | **___** |

### Pass Criteria

**Minimum Requirements**:
- [ ] All 30 test cases passed OR
- [ ] All critical tests passed (TC-005 through TC-018, TC-020) AND
- [ ] All failures have documented justification

**Critical Tests** (Must Pass):
- [ ] TC-005: Class A Private Network blocking
- [ ] TC-006: Class B Private Network blocking
- [ ] TC-007: Class C Private Network blocking
- [ ] TC-009: AWS Metadata blocking
- [ ] TC-012: IPv4 Loopback blocking
- [ ] TC-015: File protocol blocking
- [ ] TC-020: URL testing SSRF protection

---

## Issues Found

### Issue Template

**Issue ID**: _____________
**Test Case**: TC-___
**Severity**: [ ] Critical [ ] High [ ] Medium [ ] Low
**Description**:
```

```

**Steps to Reproduce**:
```

```

**Expected vs Actual**:
```

```

**Workaround** (if applicable):
```

```

---

## Sign-Off

### Tester Certification

I certify that:
- [ ] All test cases were executed as described
- [ ] Results are accurate and complete
- [ ] All issues are documented
- [ ] Test environment matches production configuration
- [ ] SSRF protection is functioning as designed

**Tester Name**: _____________
**Signature**: _____________
**Date**: _____________

---

### QA Manager Approval

- [ ] Test plan executed completely
- [ ] All critical tests passed
- [ ] Issues documented and prioritized
- [ ] SSRF remediation approved for production

**QA Manager Name**: _____________
**Signature**: _____________
**Date**: _____________

---

**Document Version**: 1.0
**Last Updated**: December 23, 2025
**Status**: Ready for Execution
