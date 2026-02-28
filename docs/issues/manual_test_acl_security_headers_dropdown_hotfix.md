## Manual Test Plan — ACL + Security Headers Dropdown Hotfix

- Date: 2026-02-27
- Scope: Proxy Host create/edit dropdown persistence
- Goal: Confirm ACL and Security Headers selections save correctly, can be changed, and can be cleared without regressions.

## Preconditions

- [ ] Charon is running and reachable in browser
- [ ] At least 2 Access Lists exist
- [ ] At least 2 Security Headers profiles exist
- [ ] Tester has permission to create and edit Proxy Hosts

## Test Cases

### TC-001 — Create Host With Both Dropdowns Set

- Steps:
  1. Open Proxy Hosts and start creating a new host.
  2. Fill required host fields.
  3. Select any Access List.
  4. Select any Security Headers profile.
  5. Save.
  6. Reopen the same host in edit mode.
- Expected:
  - The selected Access List remains selected.
  - The selected Security Headers profile remains selected.

### TC-002 — Edit Host And Change Both Selections

- Steps:
  1. Open an existing host that already has both values set.
  2. Change Access List to a different option.
  3. Change Security Headers to a different option.
  4. Save.
  5. Reopen the host.
- Expected:
  - New Access List is persisted.
  - New Security Headers profile is persisted.
  - Previous values are not shown.

### TC-003 — Clear Access List

- Steps:
  1. Open an existing host with an Access List selected.
  2. Set Access List to no selection.
  3. Save.
  4. Reopen the host.
- Expected:
  - Access List is empty (none).
  - No old Access List value returns.

### TC-004 — Clear Security Headers

- Steps:
  1. Open an existing host with a Security Headers profile selected.
  2. Set Security Headers to no selection.
  3. Save.
  4. Reopen the host.
- Expected:
  - Security Headers is empty (none).
  - No old profile value returns.

### TC-005 — Regression Guard: Repeated Edit Cycles

- Steps:
  1. Repeat edit/save cycle 3 times on one host.
  2. Alternate between selecting values and clearing values for both dropdowns.
  3. After each save, reopen the host.
- Expected:
  - Last saved choice is always what appears after reopen.
  - No mismatch between what was selected and what is shown.

## Execution Notes

- Targeted tests for this hotfix are already passing.
- Full-suite, security, and coverage gates are deferred to CI/end pass.
