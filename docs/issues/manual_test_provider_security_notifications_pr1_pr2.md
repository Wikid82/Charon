---
title: Manual Test Plan - Provider Security Notifications (PR-1 + PR-2)
status: Open
priority: High
assignee: QA
labels: testing, frontend, backend, security
---

# Test Goal
Manually verify that security notifications now follow provider-based event settings and that the Notifications/Security settings screens behave correctly for normal use and bug-hunt scenarios.

# Scope
- Provider-based security notification events
- Notifications settings UX
- Security settings UX that triggers notification events
- Focused verification for completed PR-1 + PR-2 slices

# Preconditions
- Charon is running and reachable in a browser.
- Tester can access Settings pages.
- At least one notification provider can be created/edited in UI.
- Optional but recommended: browser with a screen reader available for quick accessibility sanity checks.

# Smoke Checklist (Fast Pass)

## 1) Provider Event Toggles - Basic Save/Reload
- [ ] Open notification provider settings.
- [ ] Enable one security event category, save, and confirm success message.
- [ ] Refresh the page.
- [ ] Confirm the saved value remains enabled after reload.
- [ ] Disable the same category, save, refresh, confirm it remains disabled.

## 2) Multi-Category Toggle Sanity
- [ ] Enable all security event categories for one provider.
- [ ] Save and refresh.
- [ ] Confirm all categories still show enabled.
- [ ] Disable all categories, save, refresh, confirm all are disabled.

## 3) Notifications/Security Settings Visibility
- [ ] Open Settings → Notifications and Security-related settings area.
- [ ] Confirm labels and controls are readable and actionable.
- [ ] Confirm no missing text keys (for example raw key names).
- [ ] Confirm no layout break at common desktop width.

# Regression Checklist (Behavior Consistency)

## 4) Provider Isolation Regression
- [ ] Create or use two providers: Provider A and Provider B.
- [ ] Enable a security event on Provider A only.
- [ ] Save and reload.
- [ ] Confirm Provider B did not inherit Provider A settings.
- [ ] Flip the setup (A off, B on), save and reload.
- [ ] Confirm each provider keeps its own settings.

## 5) Existing Non-Security Notification Settings Regression
- [ ] Record a non-security setting value for a provider.
- [ ] Change only security event toggles.
- [ ] Save and reload.
- [ ] Confirm the recorded non-security setting did not unexpectedly change.

## 6) General Settings Navigation Regression
- [ ] Navigate away from notifications settings and come back.
- [ ] Confirm state shown matches the last saved values.
- [ ] Confirm no stale banner/toast remains after navigation.

# Failure-Mode / Bug-Hunt Checklist

## 7) Save Failure Handling
- [ ] While editing toggles, trigger a failed save condition (for example, stop backend or disconnect network temporarily).
- [ ] Attempt to save.
- [ ] Confirm a clear error message is shown.
- [ ] Confirm UI does not falsely show success.
- [ ] Restore connectivity/service and retry save.
- [ ] Confirm successful retry works and values persist.

## 8) Rapid Toggle Stress
- [ ] Rapidly toggle categories on/off multiple times before saving.
- [ ] Save once.
- [ ] Refresh page.
- [ ] Confirm final persisted state matches what was shown at save time.
- [ ] Confirm no duplicated toasts or frozen controls.

## 9) Multi-Tab Race Check
- [ ] Open same settings screen in two browser tabs.
- [ ] In tab 1, change and save settings.
- [ ] In tab 2, without reload, change conflicting settings and save.
- [ ] Reload both tabs.
- [ ] Confirm final state is consistent and no corrupted UI state appears.

## 10) Empty/Minimal Provider Coverage
- [ ] Use a setup with only one provider enabled.
- [ ] Verify security event toggles still behave as expected.
- [ ] If provider is disabled, verify UI behavior is clear (not misleading about active notifications).

# Accessibility Sanity Checklist

## 11) Keyboard Navigation
- [ ] Reach all controls using `Tab` and `Shift+Tab` only.
- [ ] Confirm visible focus indicator on each interactive control.
- [ ] Toggle checkboxes/switches using keyboard (`Space`/`Enter` where applicable).
- [ ] Confirm Save action is keyboard-operable.

## 12) Labels and Announcements
- [ ] Confirm each toggle has a clear visible label.
- [ ] Confirm status text and error/success messages are understandable.
- [ ] With a screen reader, verify key controls announce meaningful names and state (on/off, checked/unchecked).

# Pass/Fail Criteria
- [ ] PASS when all smoke checks pass, no regression break is found, failure-mode behavior is clear and recoverable, and accessibility sanity checks pass.
- [ ] FAIL when saved values do not persist, providers unexpectedly affect each other, failures report misleading status, or keyboard/screen reader basics break.

# Defect Logging Guidance
- [ ] For each defect, capture: page, provider name, action taken, expected vs actual result.
- [ ] Attach screenshot/video and browser info.
- [ ] Mark severity and whether issue is blocker for PR handoff.

# Handoff Note
Use this checklist as the manual verification baseline for completed focused PR-1 + PR-2 work before final merge confirmation.
