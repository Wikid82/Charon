---
title: "Manual Testing: Ntfy Notification Provider"
labels:
  - testing
  - feature
  - frontend
  - backend
priority: medium
milestone: "v0.2.0-beta.2"
assignees: []
---

# Manual Testing: Ntfy Notification Provider

## Description

Manual testing plan for the Ntfy notification provider feature. Covers UI/UX
validation, dispatch behavior, token security, and edge cases that E2E tests
cannot fully cover.

## Prerequisites

- Ntfy instance accessible (cloud: ntfy.sh, or self-hosted)
- Test topic created (e.g., `https://ntfy.sh/charon-test-XXXX`)
- Ntfy mobile/desktop app installed for push verification
- Optional: password-protected topic with access token for auth testing

## Test Cases

### UI/UX Validation

- [ ] Select "Ntfy" from provider type dropdown — token field and "Topic URL" label appear
- [ ] URL placeholder shows `https://ntfy.sh/my-topic`
- [ ] Token label shows "Access Token (optional)"
- [ ] Token field is a password field (dots, not cleartext)
- [ ] JSON template section (minimal/detailed/custom) appears for Ntfy
- [ ] Switching from Ntfy to Discord clears token field and hides it
- [ ] Switching from Discord to Ntfy shows token field again
- [ ] URL field is required — form rejects empty URL submission
- [ ] Keyboard navigation: tab through all Ntfy form fields without focus traps

### CRUD Operations

- [ ] Create Ntfy provider with URL only (no token) — succeeds
- [ ] Create Ntfy provider with URL + token — succeeds
- [ ] Edit Ntfy provider: change URL — preserves token (shows "Leave blank to keep")
- [ ] Edit Ntfy provider: clear and re-enter token — updates token
- [ ] Delete Ntfy provider — removed from list
- [ ] Create multiple Ntfy providers with different topics — all coexist

### Dispatch Verification (Requires Real Ntfy Instance)

- [ ] Send test notification to ntfy.sh cloud topic — push received on device
- [ ] Send test notification to self-hosted ntfy instance — push received
- [ ] Send test notification with minimal template — message body is correct
- [ ] Send test notification with detailed template — title and body formatted correctly
- [ ] Send test notification with custom JSON template — all fields arrive as specified
- [ ] Token-protected topic with valid token — notification delivered
- [ ] Token-protected topic with no token — notification rejected by ntfy (expected 401)
- [ ] Token-protected topic with invalid token — notification rejected by ntfy (expected 401)

### Token Security

- [ ] After creating provider with token: GET provider response has `has_token: true` but no raw token
- [ ] Browser DevTools Network tab: confirm token never appears in any API response body
- [ ] Edit provider: token field is empty (not pre-filled with existing token)
- [ ] Application logs: confirm no token values in backend logs during dispatch

### Edge Cases

- [ ] Invalid URL (not http/https) — form validation rejects
- [ ] Self-hosted ntfy URL with non-standard port (e.g., `http://192.168.1.50:8080/alerts`) — accepted and dispatches
- [ ] Very long topic name in URL — accepted
- [ ] Unicode characters in message template — dispatches correctly
- [ ] Feature flag disabled (`feature.notifications.service.ntfy.enabled = false`) — ntfy dispatch silently skipped
- [ ] Network timeout to unreachable ntfy server — error handled gracefully, no crash

### Accessibility

- [ ] Screen reader: form field labels announced correctly for Ntfy fields
- [ ] Screen reader: token help text associated via aria-describedby
- [ ] High contrast mode: Ntfy form fields visible and readable
- [ ] Voice access: "Click Topic URL" activates the correct field
- [ ] Keyboard only: complete full CRUD workflow without mouse

## Acceptance Criteria

- [ ] All UI/UX tests pass
- [ ] All CRUD operations work correctly
- [ ] At least one real dispatch to ntfy.sh confirmed
- [ ] Token never exposed in API responses or logs
- [ ] No accessibility regressions

## Related

- Spec: `docs/plans/current_spec.md`
- QA Report: `docs/reports/qa_report_ntfy_notifications.md`
- E2E Tests: `tests/settings/ntfy-notification-provider.spec.ts`
