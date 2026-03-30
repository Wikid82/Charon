---
title: "Manual Test Plan - PR-5 TCP Monitor UX Fix"
labels:
  - testing
  - frontend
  - bug
priority: high
---

# Manual Test Plan - PR-5 TCP Monitor UX Fix

## Scope

PR-5 only.

This plan covers manual verification of the five UX fixes applied to the TCP monitor creation form:

1. Corrected URL placeholder (removed misleading `tcp://` prefix)
2. Dynamic per-type placeholder (HTTP vs TCP)
3. Per-type helper text below the URL input
4. Client-side TCP scheme validation with inline error
5. Form field reorder: type selector now appears before URL input

Out of scope:
- Backend monitor logic or storage
- Any other monitor type beyond HTTP and TCP
- Notification provider changes

## Preconditions

- [ ] Environment is running (Docker E2E or local dev).
- [ ] Tester can access the Monitors page and open the Create Monitor modal.
- [ ] Browser DevTools Network tab is available for TC-PR5-007.

---

## Track A — Smoke Tests (Existing HTTP Behaviour)

### TC-PR5-001 HTTP monitor creation still works

- [ ] Open the Create Monitor modal.
- [ ] Select type **HTTP**.
- [ ] Enter a valid URL: `https://example.com`.
- [ ] Fill in remaining required fields and click **Create**.
- Expected result: monitor is created successfully; no errors shown.
- Status: [ ] Not run [ ] Pass [ ] Fail
- Notes:

### TC-PR5-009 Existing HTTP monitors display correctly in the list

- [ ] Navigate to the Monitors list.
- [ ] Confirm any pre-existing HTTP monitors are still shown with correct URLs.
- Expected result: no regressions in list display.
- Status: [ ] Not run [ ] Pass [ ] Fail
- Notes:

### TC-PR5-010 Existing TCP monitors display correctly in the list

- [ ] Navigate to the Monitors list.
- [ ] Confirm any pre-existing TCP monitors are still shown with correct host:port values.
- Expected result: no regressions in list display.
- Status: [ ] Not run [ ] Pass [ ] Fail
- Notes:

---

## Track B — Core Fix (TCP Scheme Validation)

### TC-PR5-002 TCP monitor with `tcp://` prefix shows inline error and blocks submission

- [ ] Open the Create Monitor modal.
- [ ] Select type **TCP**.
- [ ] Enter URL: `tcp://192.168.1.1:8080`.
- [ ] Click **Create** (or attempt to submit).
- Expected result: an inline error appears on the URL field; the form is not submitted; no new monitor appears in the list.
- Status: [ ] Not run [ ] Pass [ ] Fail
- Notes:

### TC-PR5-003 TCP monitor with valid `host:port` format succeeds

- [ ] Open the Create Monitor modal.
- [ ] Select type **TCP**.
- [ ] Enter URL: `192.168.1.1:8080`.
- [ ] Fill in remaining required fields and click **Create**.
- Expected result: monitor is created successfully; no errors shown.
- Status: [ ] Not run [ ] Pass [ ] Fail
- Notes:

---

## Track C — Dynamic Placeholder & Helper Text

### TC-PR5-005 Form field order: Type selector appears above URL input

- [ ] Open the Create Monitor modal.
- [ ] Inspect the visual layout of the form.
- Expected result: the monitor **Type** selector is positioned above the **URL** input field.
- Status: [ ] Not run [ ] Pass [ ] Fail
- Notes:

### TC-PR5-006 Helper text updates when switching between HTTP and TCP

- [ ] Open the Create Monitor modal.
- [ ] Select type **HTTP** and note the helper text shown beneath the URL input.
- [ ] Switch type to **TCP** and note the helper text again.
- Expected result: helper text differs between HTTP and TCP types, giving format guidance appropriate to each.
- Status: [ ] Not run [ ] Pass [ ] Fail
- Notes:

### TC-PR5-006b Placeholder updates when switching between HTTP and TCP

- [ ] Open the Create Monitor modal.
- [ ] Select type **HTTP** and note the URL input placeholder.
- [ ] Switch type to **TCP** and note the placeholder again.
- Expected result: HTTP placeholder shows a full URL (e.g. `https://example.com`); TCP placeholder shows `host:port` format (no scheme).
- Status: [ ] Not run [ ] Pass [ ] Fail
- Notes:

---

## Track D — Interaction Edge Cases

### TC-PR5-004 Switching type from TCP (with error) to HTTP clears the inline error

- [ ] Open the Create Monitor modal.
- [ ] Select type **TCP** and enter `tcp://192.168.1.1:8080` to trigger the inline error.
- [ ] Switch type to **HTTP**.
- Expected result: the scheme-prefix inline error disappears immediately after the type change.
- Status: [ ] Not run [ ] Pass [ ] Fail
- Notes:

### TC-PR5-007 Submit guard: no API call fires when scheme prefix error is present

- [ ] Open browser DevTools and go to the **Network** tab.
- [ ] Open the Create Monitor modal.
- [ ] Select type **TCP** and enter `tcp://192.168.1.1:8080`.
- [ ] Click **Create**.
- Expected result: the inline error is shown and no outbound POST/PUT request to the monitors API endpoint appears in the Network tab.
- Status: [ ] Not run [ ] Pass [ ] Fail
- Notes:

---

## Track E — Localisation (Optional)

### TC-PR5-008 New translation keys appear correctly in non-English locales

- [ ] Switch the application language to **German**, **French**, **Spanish**, or **Chinese**.
- [ ] Open the Create Monitor modal and select type **TCP**.
- [ ] Observe the URL placeholder, helper text, and inline error message (trigger it with `tcp://host:port`).
- Expected result: all three UI strings appear in the selected language without showing raw translation key strings (e.g. no `urlPlaceholder.tcp` visible to the user).
- Status: [ ] Not run [ ] Pass [ ] Fail
- Notes:

---

## Sign-off

| Tester | Date | Environment | Result |
|--------|------|-------------|--------|
|        |      |             |        |
