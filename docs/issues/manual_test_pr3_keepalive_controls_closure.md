---
title: "Manual Test Tracking Plan - PR-3 Keepalive Controls Closure"
labels:
  - testing
  - frontend
  - backend
  - security
priority: high
---

# Manual Test Tracking Plan - PR-3 Keepalive Controls Closure

## Scope
PR-3 only.

This plan tracks manual verification for:
- Keepalive control behavior in System Settings
- Safe default/fallback behavior for missing or invalid keepalive values
- Non-exposure constraints for deferred advanced settings

Out of scope:
- PR-1 compatibility closure tasks
- PR-2 security posture closure tasks
- Any new page, route, or feature expansion beyond approved PR-3 controls

## Preconditions
- [ ] Branch includes PR-3 closure changes only.
- [ ] Environment starts cleanly.
- [ ] Tester can access System Settings and save settings.
- [ ] Tester can restart and re-open the app to verify persisted behavior.

## Track A - Keepalive Controls

### TC-PR3-001 Keepalive controls are present and editable
- [ ] Open System Settings.
- [ ] Verify keepalive idle and keepalive count controls are visible.
- [ ] Enter valid values and save.
- Expected result: values save successfully and are shown after refresh.
- Status: [ ] Not run [ ] Pass [ ] Fail
- Notes:

### TC-PR3-002 Keepalive values persist across reload
- [ ] Save valid keepalive idle and count values.
- [ ] Refresh the page.
- [ ] Re-open System Settings.
- Expected result: saved values are preserved.
- Status: [ ] Not run [ ] Pass [ ] Fail
- Notes:

## Track B - Safe Defaults and Fallback

### TC-PR3-003 Missing keepalive input keeps safe defaults
- [ ] Clear optional keepalive inputs (leave unset/empty where allowed).
- [ ] Save and reload settings.
- Expected result: app remains stable and uses safe default behavior.
- Status: [ ] Not run [ ] Pass [ ] Fail
- Notes:

### TC-PR3-004 Invalid keepalive input is handled safely
- [ ] Enter invalid keepalive values (out-of-range or malformed).
- [ ] Attempt to save.
- [ ] Correct the values and save again.
- Expected result: invalid values are rejected safely; system remains stable; valid correction saves.
- Status: [ ] Not run [ ] Pass [ ] Fail
- Notes:

### TC-PR3-005 Regression check after fallback path
- [ ] Trigger one invalid save attempt.
- [ ] Save valid values immediately after.
- [ ] Refresh and verify current values.
- Expected result: no stuck state; final valid values are preserved.
- Status: [ ] Not run [ ] Pass [ ] Fail
- Notes:

## Track C - Non-Exposure Constraints

### TC-PR3-006 Deferred advanced settings remain non-exposed
- [ ] Review System Settings controls.
- [ ] Confirm `trusted_proxies_unix` is not exposed.
- [ ] Confirm certificate lifecycle internals are not exposed.
- Expected result: only approved PR-3 keepalive controls are user-visible.
- Status: [ ] Not run [ ] Pass [ ] Fail
- Notes:

### TC-PR3-007 Scope containment remains intact
- [ ] Verify no new page/tab/modal was introduced for PR-3 controls.
- [ ] Verify settings flow still uses existing System Settings experience.
- Expected result: PR-3 remains contained to approved existing surface.
- Status: [ ] Not run [ ] Pass [ ] Fail
- Notes:

## Defect Log

| ID | Test Case | Severity | Summary | Reproducible | Status |
| --- | --- | --- | --- | --- | --- |
| | | | | | |

## Exit Criteria
- [ ] All PR-3 test cases executed.
- [ ] No unresolved critical defects.
- [ ] Keepalive controls, safe fallback/default behavior, and non-exposure constraints are verified.
- [ ] No PR-1 or PR-2 closure tasks introduced in this PR-3 plan.
