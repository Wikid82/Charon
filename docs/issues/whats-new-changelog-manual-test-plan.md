---
title: "Manual Test Plan: 'What's New' Changelog Modal"
labels:
  - testing
  - feature
  - frontend
priority: medium
milestone: ""
assignees: []
---

# Manual Test Plan: "What's New" Changelog Modal

## Description

Manual, human-driven checklist for the new "What's New" changelog popup
that greets users after an app update. Automated E2E
(`tests/settings/whats-new-changelog.spec.ts`) already covers the happy
paths against fixture data (see `docs/reports/qa_report.md`, verdict
READY TO MERGE) — this plan supplements that with a real deployment
walkthrough (real login sessions, real version bumps, real page reloads)
that automated tests can't fully stand in for. See
`docs/superpowers/specs/2026-07-28-whats-new-changelog-design.md` for the
full feature design and `docs/plans/current_spec.md` for the
implementation plan.

## Prerequisites

- A running Charon instance built with a real (non-`dev`) version string,
  or the dev override `CHARON_CHANGELOG_VERSION` set per the design spec's
  "Manual pre-merge workflow" section (`CHARON_ENV=development` required
  for the override to take effect).
- `backend/internal/changelog/data/changelog.json` populated with at least
  two versions' worth of fixture or real entries (never commit this file
  changed from the `[]` placeholder).
- At least one test user account whose `last_seen_version` you can set/view
  directly in the SQLite database, so each scenario below starts from a
  known state.
- Access to the Appearance Settings page as a logged-in user.

## Test Cases

### First Login After a Version Bump

- [ ] Set a test user's `last_seen_version` to an older version than the
      configured "current" version, then log in — the "What's New" popup
      appears automatically after the page loads (not before — login
      itself is never delayed or blocked waiting on this).
- [ ] Confirm the popup lists the expected new features and fixes for the
      versions between the user's old `last_seen_version` and the current
      one, newest version first.
- [ ] Confirm routine maintenance items (dependency bumps, refactors, etc.)
      are tucked behind a collapsed "Show maintenance details" section, not
      shown by default.
- [ ] Log in again with the same user (without dismissing) — confirm the
      popup still appears every time until one of the dismiss actions below
      is used.

### Dismiss Path 1 — "Remind Me Next Time"

- [ ] With the popup showing, click **"Remind Me Next Time"**.
- [ ] Popup closes immediately.
- [ ] Log out and log back in as the same user — **the popup appears
      again** (this dismissal is temporary; it does not mark the update as
      seen).

### Dismiss Path 2 — "Got It, Thanks"

- [ ] With the popup showing (fresh test user or reset `last_seen_version`
      again), click **"Got It, Thanks"**.
- [ ] Popup closes immediately.
- [ ] Log out and log back in as the same user — **the popup does NOT
      appear again** for this version (this dismissal permanently records
      the current version as seen).
- [ ] Confirm this holds across multiple subsequent logins, not just the
      next one.

### Dismiss Path 3 — Close Button (X) / Backdrop Click

- [ ] With the popup showing (reset `last_seen_version` again), click the
      **X** icon in the corner — popup closes.
- [ ] Log out/in again — popup **reappears** (X behaves the same as
      "Remind Me Next Time": a temporary snooze, not a permanent dismissal).
- [ ] Repeat, this time clicking outside the popup on the dark backdrop
      instead of the X — same result: popup closes, and reappears on next
      login.

### Opt-Out Checkbox

- [ ] With the popup showing, check **"Don't show me update notifications"**
      and then click **"Remind Me Next Time"** (a temporary-dismiss path).
- [ ] Log out and log back in — confirm the popup does **NOT** appear, even
      though this dismiss path alone would normally bring it back. (The
      opt-out checkbox is independent of which button was clicked.)
- [ ] Repeat this check pairing the checkbox with **"Got It, Thanks"** and
      with the **X**/backdrop close — in all three cases, checking the box
      suppresses all future automatic popups regardless of which close
      method was used.

### Re-Enabling via Appearance Settings

- [ ] As a user who is currently opted out (from the previous section),
      navigate to **Appearance Settings**.
- [ ] Confirm the **"Show 'What's New' after updates"** toggle reflects the
      opted-out state (off).
- [ ] Turn the toggle on.
- [ ] Log out and log back in with a `last_seen_version` older than
      current — confirm the popup **now appears again** (opt-out has been
      cleared, and this action does not by itself mark anything as seen).
- [ ] Turn the toggle off again directly from Appearance Settings (without
      going through the popup) — confirm this also suppresses future
      automatic popups, matching the checkbox behavior above.

### "What's New" Revisit Link (Browse Mode)

- [ ] From Appearance Settings, click the **"What's New"** link/button.
- [ ] Confirm the same modal opens, showing the **full** changelog history
      (not just entries since `last_seen_version`).
- [ ] Confirm this browse-mode view only offers a single **Close** action —
      no "Remind Me"/"Got It, Thanks" buttons, no opt-out checkbox.
- [ ] Close it, then check the test user's `last_seen_version` and opt-out
      state directly in the database — confirm **neither changed**.
      Browsing must have zero side effects.
- [ ] Use this link while already opted out — confirm it still opens and
      shows full history (browsing works regardless of opt-out status).

### Pre-Existing User Migration Behavior

- [ ] Find or create a user record with an **empty** `last_seen_version`
      (simulating an account that existed before this feature shipped and
      hasn't logged in since the migration ran).
- [ ] Log in as that user — confirm the popup appears showing the full
      changelog history up to the current version (empty `last_seen_version`
      is treated as "behind everything").
- [ ] Dismiss with **"Got It, Thanks"**.
- [ ] Log out and log back in — confirm the popup does **not** reappear
      (this was a one-time catch-up, not a recurring state).

## Notes

- The popup must never delay or block the login/dashboard flow — if the
  changelog check is slow or fails outright, the app should load normally
  with no popup and no visible error.
- Brand-new users (created after this feature shipped) should never see
  historical changelog entries on their first login — their
  `last_seen_version` is seeded at account creation. Worth a quick sanity
  check if a fresh invite/admin-created account is available during this
  pass.
- Automated E2E already exercises the equivalent flows against fixture
  data with `CHARON_CHANGELOG_VERSION` — this plan exists to catch anything
  specific to a real deployment (real cookies/session lifecycle, real
  database state, real multi-login timing) that fixtures can't surface.
