---
title: Manual Test Plan - Notifications Single Source of Truth
status: Open
priority: High
assignee: QA
labels: testing, frontend, i18n, accessibility
---

# Test Goal
Manually verify the notifications single-source-of-truth behavior for provider security events and confirm related UI, persistence, localization, and accessibility smoke behavior.

# Scope
- Notifications settings page visibility changes
- Provider Add/Edit security event checkbox behavior
- CRUD persistence for security event selections
- Updated locale string rendering
- Provider form accessibility smoke checks

# Preconditions
- Charon is running and reachable in a browser.
- Tester can access Settings and notification provider Add/Edit flows.
- At least one language switch option is available in the UI.

## 1) Notifications settings page: no standalone security section
- [ ] Open Settings → Notifications.
- [ ] Confirm no standalone security section is shown on this page.
- [ ] Confirm page layout remains clean (no large empty gaps where removed content would be).
- [ ] Confirm no untranslated raw keys are visible.

## 2) Provider Add/Edit security event checkboxes as single source of truth
- [ ] Open Add Provider form and locate security event checkboxes.
- [ ] Confirm security event options are configurable in provider form.
- [ ] Save a provider with a specific mixed selection (some checked, some unchecked).
- [ ] Re-open Edit for that provider and confirm checkbox states match saved values.
- [ ] Confirm checkbox labels are clear and not duplicated.

## 3) CRUD flows preserve security event selections
- [ ] Create provider with custom security event selection and save.
- [ ] Refresh and verify saved selections persist.
- [ ] Edit provider, change only one security event selection, save, refresh, verify exact change persisted.
- [ ] Duplicate CRUD sanity: create second provider with different security event selections and confirm values stay isolated per provider.
- [ ] Delete one provider and confirm remaining provider keeps its own security event selections unchanged.

## 4) i18n rendering for modified locale strings
- [ ] Switch to each supported language in turn.
- [ ] Open Notifications settings and provider Add/Edit screens.
- [ ] Confirm modified strings render as user-facing text (no raw translation keys).
- [ ] Confirm labels are not truncated or overlapping in provider forms.
- [ ] Confirm returning to default language restores expected text.

## 5) Accessibility smoke checks for provider form interactions
- [ ] Navigate provider form controls using keyboard only (`Tab`, `Shift+Tab`, `Space`, `Enter`).
- [ ] Confirm each security event checkbox is reachable and operable by keyboard.
- [ ] Confirm visible focus indicator is present on each interactive control.
- [ ] Confirm checkbox label text clearly describes each option.
- [ ] Run a quick screen reader smoke pass and confirm checkbox name + checked state are announced.
