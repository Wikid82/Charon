---
title: "Manual Test Plan: Telegram Notification Provider"
labels:
  - testing
  - frontend
  - backend
  - security
priority: medium
assignees: []
---

# Manual Test Plan: Telegram Notification Provider

Scenarios that automated E2E tests cannot fully verify — real network calls, token redaction in DevTools, and cross-browser visual rendering.

## Prerequisites

- A Telegram bot token (create one via [@BotFather](https://t.me/BotFather))
- A Telegram chat ID (send a message to your bot, then check `https://api.telegram.org/bot<TOKEN>/getUpdates`)
- Charon running locally or in Docker
- Firefox, Chrome, and Safari available for cross-browser checks

---

## 1. Real Telegram Integration

- [ ] Navigate to **Settings → Notifications**
- [ ] Click **Add Provider**, select **Telegram** type
- [ ] Enter your real bot token and chat ID, give it a name, click **Save**
- [ ] Click the **Send Test** button on the newly saved provider row
- [ ] Open Telegram and confirm the test message arrived in your chat

## 2. Bot Token Security (DevTools)

- [ ] Open browser DevTools → **Network** tab
- [ ] Load the Notifications page (refresh if needed)
- [ ] Inspect the GET response that returns the provider list
- [ ] Confirm the bot token value is **not** present in the response body — only `has_token: true` (or equivalent indicator)
- [ ] Inspect the provider row in the UI — confirm the token is masked or hidden, never shown in plain text

## 3. Save-Before-Test UX

- [ ] Click **Add Provider**, select **Telegram** type
- [ ] **Before saving**, locate the **Test** button
- [ ] Confirm it is disabled (greyed out / not clickable)
- [ ] Hover over or focus the disabled Test button and confirm a tooltip explains the provider must be saved first

## 4. Error Hint Display

- [ ] Add a new Telegram provider with an **invalid** bot token (e.g. `000000:FAKE`)
- [ ] Save the provider, then click **Send Test**
- [ ] Confirm a toast/notification appears containing a helpful hint (e.g. "Unauthorized" or "bot token is invalid")

## 5. Provider Type Switching

- [ ] Click **Add Provider**
- [ ] Select **Discord** — note the visible form fields
- [ ] Switch to **Telegram** — confirm a **Token** field and **Chat ID** field appear
- [ ] Switch to **Webhook** — confirm Telegram-specific fields disappear and a URL field appears
- [ ] Switch to **Gotify** — confirm a **Token** field appears (similar to Telegram)
- [ ] Switch back to **Telegram** — confirm fields restore correctly with no leftover values

## 6. Keyboard Navigation

- [ ] Tab through the provider list using only the keyboard
- [ ] For each provider row, confirm the **Send Test**, **Edit**, and **Delete** buttons are all reachable via Tab
- [ ] Press Enter or Space on each button to confirm it activates
- [ ] With a screen reader (or DevTools Accessibility panel), verify each button has a descriptive ARIA label (e.g. "Send test notification to My Telegram")

## 7. Cross-Browser Visual Check

For each browser — **Firefox**, **Chrome**, **Safari**:

- [ ] Load the Notifications page and confirm the provider list renders without layout issues
- [ ] Open the Add/Edit provider form and confirm fields align correctly
- [ ] Send a test notification and confirm the toast/notification displays properly
- [ ] Resize the window to a narrow width and confirm the layout remains usable
