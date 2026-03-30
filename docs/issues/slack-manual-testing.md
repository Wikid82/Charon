---
title: "Manual Testing: Slack Notification Provider"
labels:
  - testing
  - feature
  - frontend
  - backend
priority: medium
milestone: "v0.2.0-beta.2"
assignees: []
---

# Manual Testing: Slack Notification Provider

## Description

Manual test plan for the Slack notification provider feature. Covers scenarios that automated E2E tests cannot fully validate, such as real Slack workspace delivery, message formatting, and edge cases around webhook lifecycle.

## Pre-requisites

- A Slack workspace with at least one channel
- An Incoming Webhook URL created via Slack App configuration (https://api.slack.com/messaging/webhooks)
- Access to the Charon instance

## Test Cases

### Provider CRUD

- [ ] **Create**: Add a Slack provider with a valid webhook URL and optional channel name (`#alerts`)
- [ ] **Edit**: Change the channel display name — verify webhook URL is preserved (not cleared)
- [ ] **Test**: Click "Send Test Notification" — verify message appears in Slack channel
- [ ] **Delete**: Remove the Slack provider — verify it no longer appears in the list
- [ ] **Re-create**: Add a new Slack provider after deletion — verify clean state

### Security

- [ ] Webhook URL is NOT visible in the provider list UI (only `has_token: true` indicator)
- [ ] Webhook URL is NOT returned in GET `/api/v1/notifications/providers` response body
- [ ] Editing an existing provider does NOT expose the webhook URL in any form field
- [ ] Browser DevTools Network tab shows no webhook URL in any API response

### Message Delivery

- [ ] Default template sends a readable notification to Slack
- [ ] Custom JSON template with `text` field renders correctly
- [ ] Custom JSON template with `blocks` renders Block Kit layout
- [ ] Notifications triggered by proxy host changes arrive in Slack
- [ ] Notifications triggered by certificate events arrive in Slack
- [ ] Notifications triggered by uptime events arrive in Slack (if enabled)

### Error Handling

- [ ] Invalid webhook URL (not matching `hooks.slack.com/services/` pattern) shows validation error
- [ ] Expired/revoked webhook URL returns `no_service` classification error
- [ ] Disabled feature flag (`feature.notifications.service.slack.enabled=false`) prevents Slack dispatch

### Edge Cases

- [ ] Creating provider with empty URL field succeeds (URL is optional channel display name)
- [ ] Very long channel name in URL field is handled gracefully
- [ ] Multiple Slack providers with different webhooks can coexist
- [ ] Switching provider type from Slack to Discord clears the token field appropriately
- [ ] Switching provider type from Discord to Slack shows the webhook URL input field

### Cross-Browser

- [ ] Provider CRUD works in Chrome/Chromium
- [ ] Provider CRUD works in Firefox
- [ ] Provider CRUD works in Safari/WebKit

## Acceptance Criteria

- [ ] All security test cases pass — webhook URL never exposed
- [ ] End-to-end message delivery confirmed in a real Slack workspace
- [ ] No console errors during any provider operations
- [ ] Feature flag correctly gates Slack functionality
