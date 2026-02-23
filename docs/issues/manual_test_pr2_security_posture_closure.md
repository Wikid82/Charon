---
title: "Manual Test Tracking Plan - Security Posture Closure"
labels:
  - testing
  - security
  - caddy
priority: high
---

# Manual Test Tracking Plan - PR-2 Security Posture Closure

## Scope
PR-2 only.

This plan tracks manual verification for:
- Patch disposition decisions
- Admin API assumptions and guardrails
- Rollback checks

Out of scope:
- PR-1 compatibility closure tasks
- PR-3 feature or UX expansion

## Preconditions
- [ ] Branch contains PR-2 documentation and configuration changes only.
- [ ] Environment starts cleanly with default PR-2 settings.
- [ ] Tester can run container start/restart and review startup logs.

## Track A - Patch Disposition Validation

### TC-PR2-001 Retained patches remain retained
- [ ] Verify `expr` and `ipstore` patch decisions are documented as retained in the PR-2 security posture report.
- [ ] Confirm no conflicting PR-2 docs state these patches are retired.
- Expected result: retained/retained remains consistent across PR-2 closure docs.
- Status: [ ] Not run [ ] Pass [ ] Fail
- Notes:

### TC-PR2-002 Nebula default retirement is clearly bounded
- [ ] Verify PR-2 report states `nebula` retirement is by default scenario switch.
- [ ] Verify rollback instruction is present and explicit.
- Expected result: reviewer can identify default posture and rollback without ambiguity.
- Status: [ ] Not run [ ] Pass [ ] Fail
- Notes:

## Track B - Admin API Assumption Checks

### TC-PR2-003 Internal-only admin API assumption
- [ ] Confirm PR-2 report states admin API is expected to be internal-only.
- [ ] Confirm PR-2 QA report includes admin API validation/normalization posture.
- Expected result: both reports communicate the same assumption.
- Status: [ ] Not run [ ] Pass [ ] Fail
- Notes:

### TC-PR2-004 Invalid admin endpoint fails fast
- [ ] Start with an intentionally invalid/non-allowlisted admin API URL.
- [ ] Verify startup fails fast with clear configuration rejection behavior.
- [ ] Restore valid URL and confirm startup succeeds.
- Expected result: unsafe endpoint rejected; safe endpoint accepted.
- Status: [ ] Not run [ ] Pass [ ] Fail
- Notes:

### TC-PR2-005 Port exposure assumption holds
- [ ] Verify deployment defaults do not publish admin API port `2019`.
- [ ] Confirm no PR-2 doc contradicts this default posture.
- Expected result: admin API remains non-published by default.
- Status: [ ] Not run [ ] Pass [ ] Fail
- Notes:

## Track C - Rollback Safety Checks

### TC-PR2-006 Scenario rollback switch
- [ ] Set `CADDY_PATCH_SCENARIO=A`.
- [ ] Restart and verify the rollback path is accepted by the runtime.
- [ ] Return to PR-2 default scenario and verify normal startup.
- Expected result: rollback is deterministic and reversible.
- Status: [ ] Not run [ ] Pass [ ] Fail
- Notes:

### TC-PR2-007 QA report rollback statement alignment
- [ ] Confirm QA report and security posture report use the same rollback instruction.
- [ ] Confirm both reports remain strictly PR-2 scoped.
- Expected result: no conflicting rollback guidance; no PR-3 references.
- Status: [ ] Not run [ ] Pass [ ] Fail
- Notes:

## Defect Log

| ID | Test Case | Severity | Summary | Reproducible | Status |
| --- | --- | --- | --- | --- | --- |
| | | | | | |

## Exit Criteria
- [ ] All PR-2 test cases executed.
- [ ] No unresolved critical defects.
- [ ] Patch disposition, admin API assumptions, and rollback checks are all verified.
- [ ] No PR-3 material introduced in this tracking plan.
