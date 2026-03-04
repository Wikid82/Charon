---
title: Manual Test Tracking Plan - Notify Wrapper (Gotify + Custom Webhook)
status: Open
priority: High
assignee: QA
labels: testing, notifications, backend, frontend, security
---

# Test Goal
Track manual verification for bugs and regressions after the Notify migration that added HTTP wrapper delivery for Gotify and Custom Webhook providers.

# Scope
- Provider creation and editing for Gotify and Custom Webhook
- Send Test and Preview behavior
- Payload rendering and delivery behavior
- Secret handling and error-message safety
- Existing Discord behavior regression checks

# Preconditions
- Charon is running and reachable in a browser.
- Tester can open Settings → Notifications.
- Tester has reachable endpoints for:
  - One Gotify instance
  - One custom webhook receiver

## 1) Smoke Path - Provider CRUD
- [ ] Create a Gotify provider with valid URL and token, save successfully.
- [ ] Create a Custom Webhook provider with valid URL, save successfully.
- [ ] Refresh and confirm both providers persist with expected non-secret fields.
- [ ] Edit each provider, save changes, refresh, and confirm updates persist.

## 2) Smoke Path - Test and Preview
- [ ] Run Send Test for Gotify provider and confirm successful delivery.
- [ ] Run Send Test for Custom Webhook provider and confirm successful delivery.
- [ ] Run Preview for both providers and confirm payload is rendered as expected.
- [ ] Confirm Discord provider test/preview still works.

## 3) Payload Regression Checks
- [ ] Validate minimal payload template sends correctly.
- [ ] Validate detailed payload template sends correctly.
- [ ] Validate custom payload template sends correctly.
- [ ] Verify special characters and multi-line content render correctly.
- [ ] Verify payload output remains stable after provider edit + save.

## 4) Secret and Error Safety Checks
- [ ] Confirm Gotify token is never shown in list/readback UI.
- [ ] Confirm Gotify token is not exposed in test/preview responses shown in UI.
- [ ] Trigger a failed test (invalid endpoint) and confirm error text is clear but does not expose secrets.
- [ ] Confirm failed requests do not leak sensitive values in user-visible error content.

## 5) Failure-Mode and Recovery Checks
- [ ] Test with unreachable endpoint and confirm failure is reported clearly.
- [ ] Test with malformed URL and confirm validation blocks save.
- [ ] Test with slow endpoint and confirm UI remains responsive and recoverable.
- [ ] Fix endpoint values and confirm retry succeeds without recreating provider.

## 6) Cross-Provider Regression Checks
- [ ] Confirm Gotify changes do not alter Custom Webhook settings.
- [ ] Confirm Custom Webhook changes do not alter Discord settings.
- [ ] Confirm deleting one provider does not corrupt remaining providers.

## Pass/Fail Criteria
- [ ] PASS when all smoke checks pass, payload output is correct, secrets stay hidden, and no cross-provider regressions are found.
- [ ] FAIL when delivery breaks, payload rendering regresses, secrets are exposed, or provider changes affect unrelated providers.

## Defect Tracking Notes
- [ ] For each defect, record provider type, action, expected result, actual result, and severity.
- [ ] Attach screenshot/video where useful.
- [ ] Mark whether defect is release-blocking.
