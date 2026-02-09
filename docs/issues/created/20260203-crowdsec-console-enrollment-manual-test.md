# Manual Test Plan: CrowdSec Console Enrollment

**Issue**: #586
**PR**: #609
**Date**: 2025-01-29

## Overview

This test plan covers manual verification of CrowdSec console enrollment functionality to ensure the engine appears online in the CrowdSec console after enrollment.

## Prerequisites

- Docker container running with CrowdSec enabled
- Valid CrowdSec console account
- Fresh enrollment token from console.crowdsec.net

## Test Cases

### TC1: Fresh Enrollment

| Step | Action | Expected Result |
|------|--------|-----------------|
| 1 | Navigate to Security → CrowdSec | CrowdSec settings page loads |
| 2 | Enable CrowdSec if not enabled | Toggle switches to enabled |
| 3 | Enter valid enrollment token | Token field accepts input |
| 4 | Click Enroll | Loading indicator appears |
| 5 | Wait for completion | Success message shown |
| 6 | Check CrowdSec console | Engine appears online within 5 minutes |

### TC2: Heartbeat Verification

| Step | Action | Expected Result |
|------|--------|-----------------|
| 1 | Complete TC1 enrollment | Engine enrolled |
| 2 | Wait 5 minutes | Heartbeat poller runs |
| 3 | Check logs for `[HEARTBEAT_POLLER]` | Heartbeat success logged |
| 4 | Check console.crowdsec.net | Last seen updates to recent time |

### TC3: Diagnostic Endpoints

| Step | Action | Expected Result |
|------|--------|-----------------|
| 1 | Call GET `/api/v1/cerberus/crowdsec/diagnostics/connectivity` | Returns connectivity status |
| 2 | Verify `lapi_reachable` is true | LAPI is running |
| 3 | Verify `capi_reachable` is true | Can reach CrowdSec cloud |
| 4 | Call GET `/api/v1/cerberus/crowdsec/diagnostics/config` | Returns config validation |

### TC4: Diagnostic Script

| Step | Action | Expected Result |
|------|--------|-----------------|
| 1 | Run `./scripts/diagnose-crowdsec.sh` | All 10 checks execute |
| 2 | Verify LAPI status check passes | Shows "running" |
| 3 | Verify console status check | Shows enrollment status |
| 4 | Run with `--json` flag | Valid JSON output |

### TC5: Recovery from Offline State

| Step | Action | Expected Result |
|------|--------|-----------------|
| 1 | Stop the container | Container stops |
| 2 | Wait 1 hour | Console shows engine offline |
| 3 | Restart container | Container starts |
| 4 | Wait 5-10 minutes | Heartbeat poller reconnects |
| 5 | Check console | Engine shows online again |

### TC6: Token Expiration Handling

| Step | Action | Expected Result |
|------|--------|-----------------|
| 1 | Use an expired enrollment token | |
| 2 | Attempt enrollment | Error message indicates token expired |
| 3 | Check logs | Error is logged with `[CROWDSEC_ENROLLMENT]` |
| 4 | Token is NOT visible in logs | Secret redacted |

### TC7: Already Enrolled Error

| Step | Action | Expected Result |
|------|--------|-----------------|
| 1 | Complete successful enrollment | |
| 2 | Attempt enrollment again with same token | |
| 3 | Error message indicates already enrolled | |
| 4 | Existing enrollment preserved | |

## Known Issues

- **Edge case**: If LAPI takes >30s to start after container restart, first heartbeat may fail (retries automatically)
- **Console lag**: CrowdSec console may take 2-5 minutes to reflect online status

## Bug Tracking

Use this section to track bugs found during manual testing:

| Bug ID | Description | Severity | Status |
|--------|-------------|----------|--------|
| | | | |

## Sign-off

- [ ] All test cases executed
- [ ] Bugs documented
- [ ] Ready for release
