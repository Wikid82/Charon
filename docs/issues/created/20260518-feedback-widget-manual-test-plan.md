---
# REQUIRED: Issue title
title: "Manual Testing: Feedback Widget"

# OPTIONAL: Labels to apply (will be created if missing)
labels:
  - testing
  - frontend
  - ui

# OPTIONAL: Priority (creates matching label)
priority: medium

# OPTIONAL: Milestone name
milestone: ""

# OPTIONAL: GitHub usernames to assign
assignees: []
---

# Manual Testing: Feedback Widget

## Description

Validate the newly implemented feedback widget that appears on every authenticated page in Charon. The widget is a fixed floating button in the bottom-right corner that opens a panel with links to GitHub Issues for bug reports and feature requests.

---

## Functional Tests

### Trigger Button

- [ ] Widget trigger button is visible on every authenticated page
- [ ] Button renders in the bottom-right corner and does not obscure page content
- [ ] Clicking the trigger opens the feedback panel
- [ ] Panel is hidden on initial page load

### Panel Open / Close

- [ ] Clicking the trigger a second time closes the panel
- [ ] Clicking outside the panel (backdrop) closes the panel
- [ ] Pressing `Escape` while the panel is open closes the panel
- [ ] After closing via `Escape`, focus returns to the trigger button
- [ ] After closing via backdrop click, panel is dismissed cleanly

### Links

- [ ] "Report a Bug" link is visible in the panel
- [ ] "Report a Bug" link navigates to `https://github.com/Wikid82/Charon/issues/new?template=bug_report.md`
- [ ] "Report a Bug" link opens in a new browser tab
- [ ] "Request a Feature" link is visible in the panel
- [ ] "Request a Feature" link navigates to `https://github.com/Wikid82/Charon/issues/new?template=feature_request.md`
- [ ] "Request a Feature" link opens in a new browser tab
- [ ] Clicking a link closes the panel

---

## Keyboard Accessibility Tests

- [ ] Widget trigger is reachable via `Tab` key navigation
- [ ] Pressing `Enter` or `Space` on the trigger opens the panel
- [ ] When panel opens, focus moves to the first link ("Report a Bug")
- [ ] `Tab` moves focus from first link to second link ("Request a Feature")
- [ ] `Shift+Tab` moves focus backward from second link to first
- [ ] `Escape` closes the panel and returns focus to the trigger button
- [ ] Links are activatable via `Enter` key

---

## Screen Reader / Accessibility Tests

- [ ] Trigger button announces its label to screen readers (e.g., VoiceOver, NVDA)
- [ ] `aria-expanded="false"` when panel is closed
- [ ] `aria-expanded="true"` when panel is open
- [ ] Panel is announced as a navigation landmark
- [ ] "Report a Bug" and "Request a Feature" are announced as links (not buttons)
- [ ] "Opens in new tab" announcement is present for both links
- [ ] No role="menu" or role="menuitem" ARIA attributes (should be native `<nav>` + `<a>`)

---

## Visual / Theme Tests

- [ ] Widget renders correctly in light mode
- [ ] Widget renders correctly in dark mode
- [ ] Panel shadow and border are visible in both modes
- [ ] Widget does not overlap or interfere with notifications or other fixed elements
- [ ] Widget does not cover critical page actions (modals, drawers, forms)

---

## Internationalization Tests

Test each of the 5 supported locales (change locale in Settings):

| Locale | Trigger Label | Panel Title | Bug Link | Feature Link |
|--------|-------------|-------------|----------|--------------|
| `en` (English) | - [ ] Renders | - [ ] Renders | - [ ] Renders | - [ ] Renders |
| `de` (Deutsch) | - [ ] Renders | - [ ] Renders | - [ ] Renders | - [ ] Renders |
| `es` (Español) | - [ ] Renders | - [ ] Renders | - [ ] Renders | - [ ] Renders |
| `fr` (Français) | - [ ] Renders | - [ ] Renders | - [ ] Renders | - [ ] Renders |
| `zh` (中文) | - [ ] Renders | - [ ] Renders | - [ ] Renders | - [ ] Renders |

---

## Regression Tests

- [ ] Existing notification center still opens and closes correctly
- [ ] Existing modals and dialogs are not obscured by the widget
- [ ] Sidebar navigation is still fully accessible and not blocked by the widget
- [ ] Page scrolling is not affected by the widget
- [ ] Widget does not cause layout shift on page load

---

## Acceptance Criteria

- [ ] Widget is visible on all authenticated pages (Dashboard, Proxy Hosts, Certificates, Settings, etc.)
- [ ] All functional open/close behaviors work correctly
- [ ] Keyboard navigation passes all checks above
- [ ] Both links open correct GitHub Issues URLs in new tabs
- [ ] Widget renders correctly in both light and dark themes
- [ ] All 5 locales render translated strings without layout breaks
- [ ] No regressions introduced to existing UI functionality
