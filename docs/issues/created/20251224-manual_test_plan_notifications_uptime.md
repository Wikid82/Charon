# Manual Test Plan: Notification Templates & Uptime Monitoring

**Feature:** Universal JSON Template Support + Uptime Monitoring Improvements
**Version:** Phase 5 Implementation
**Date Created:** 2025-12-24
**Test Environment:** Local Docker / Staging
**Prerequisites:**

- Charon running with latest build
- Access to test webhooks (Discord, Slack, Gotify)
- At least 2 proxy hosts configured
- Admin user credentials

---

## Test Status Overview

| Category | Tests | Passed | Failed | Blocked | Not Run |
|----------|-------|--------|--------|---------|---------|
| Notification Templates | 12 | 0 | 0 | 0 | 12 |
| Uptime Monitoring | 10 | 0 | 0 | 0 | 10 |
| Integration | 6 | 0 | 0 | 0 | 6 |
| **Total** | **28** | **0** | **0** | **0** | **28** |

---

## Section 1: Discord Webhook with JSON Templates

### Test 1.1: Discord Minimal Template

**Objective:** Verify Discord notifications work with minimal template.

**Steps:**

1. Navigate to Settings → Notifications
2. Click "Add Provider"
3. Configure:
   - **Name:** "Discord Test Minimal"
   - **Type:** Discord
   - **URL:** `https://discord.com/api/webhooks/...` (your test webhook)
   - **Template:** Minimal
   - **Events:** SSL Certificate events (check all)
4. Click "Send Test Notification"
5. Check Discord channel

**Expected Result:**

- ✅ Test notification received in Discord
- ✅ Message contains title and message text
- ✅ No errors in UI
- ✅ Provider saves successfully

**Actual Result:**

- [ ] Pass
- [ ] Fail (describe issue): \_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_

**Notes:**

---

### Test 1.2: Discord Detailed Template with Embed

**Objective:** Verify Discord rich embeds work correctly.

**Steps:**

1. Edit the Discord provider created in Test 1.1
2. Change **Template** to "Detailed"
3. Click "Send Test Notification"
4. Check Discord channel

**Expected Result:**

- ✅ Rich embed displayed with color
- ✅ Contains title, description, timestamp
- ✅ Fields displayed correctly (Event Type, Host, etc.)
- ✅ Embed has proper structure

**Actual Result:**

- [ ] Pass
- [ ] Fail (describe issue): \_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_

**Notes:**

---

### Test 1.3: Discord Custom Template

**Objective:** Verify custom JSON templates work for Discord.

**Steps:**

1. Edit Discord provider
2. Change **Template** to "Custom"
3. Enter custom template:

```json
{
  "username": "Charon Bot",
  "embeds": [{
    "title": "🚨 {{.Title}}",
    "description": "{{.Message}}",
    "color": 15158332,
    "fields": [
      {
        "name": "Event Type",
        "value": "{{.EventType}}",
        "inline": true
      },
      {
        "name": "Timestamp",
        "value": "{{.Timestamp}}",
        "inline": true
      }
    ],
    "footer": {
      "text": "Charon Alert System"
    }
  }]
}
```

1. Click "Validate Template"
2. Click "Send Test Notification"
3. Check Discord channel

**Expected Result:**

- ✅ Template validates successfully
- ✅ Custom embed appears with emoji in title
- ✅ Custom username "Charon Bot" displayed
- ✅ Footer text appears
- ✅ All template variables replaced correctly

**Actual Result:**

- [ ] Pass
- [ ] Fail (describe issue): \_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_

**Notes:**

---

### Test 1.4: Discord Template Validation Error

**Objective:** Verify invalid JSON is rejected with clear error.

**Steps:**

1. Edit Discord provider
2. Enter invalid JSON template (missing closing brace):

```json
{
  "embeds": [{
    "title": "{{.Title}}",
    "description": "{{.Message}}"
  ]
```

1. Click "Validate Template"

**Expected Result:**

- ✅ Validation fails
- ✅ Clear error message displayed
- ✅ Cannot save invalid template
- ✅ Error indicates JSON syntax issue

**Actual Result:**

- [ ] Pass
- [ ] Fail (describe issue): \_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_

**Notes:**

---

## Section 2: Slack Webhook with JSON Templates

### Test 2.1: Slack Minimal Template

**Objective:** Verify Slack notifications work with minimal template.

**Steps:**

1. Add new provider:
   - **Name:** "Slack Test Minimal"
   - **Type:** Slack
   - **URL:** `https://hooks.slack.com/services/...` (your test webhook)
   - **Template:** Minimal
   - **Events:** Uptime monitoring events
2. Click "Send Test Notification"
3. Check Slack channel

**Expected Result:**

- ✅ Message received in Slack
- ✅ Contains title and message text
- ✅ No errors in UI

**Actual Result:**

- [ ] Pass
- [ ] Fail (describe issue): \_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_

**Notes:**

---

### Test 2.2: Slack Block Kit Template

**Objective:** Verify Slack Block Kit formatting works.

**Steps:**

1. Edit Slack provider
2. Change **Template** to "Custom"
3. Enter Block Kit template:

```json
{
  "text": "{{.Title}}",
  "blocks": [
    {
      "type": "header",
      "text": {
        "type": "plain_text",
        "text": "🔔 {{.Title}}",
        "emoji": true
      }
    },
    {
      "type": "section",
      "text": {
        "type": "mrkdwn",
        "text": "*Message:* {{.Message}}"
      }
    },
    {
      "type": "section",
      "fields": [
        {
          "type": "mrkdwn",
          "text": "*Event:*\n{{.EventType}}"
        },
        {
          "type": "mrkdwn",
          "text": "*Time:*\n{{.Timestamp}}"
        }
      ]
    }
  ]
}
```

1. Click "Send Test Notification"
2. Check Slack channel

**Expected Result:**

- ✅ Block Kit message displayed with sections
- ✅ Header with emoji shown
- ✅ Markdown formatting applied (bold text)
- ✅ Fields displayed correctly

**Actual Result:**

- [ ] Pass
- [ ] Fail (describe issue): \_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_

**Notes:**

---

## Section 3: Gotify Webhook with JSON Templates

### Test 3.1: Gotify Basic JSON

**Objective:** Verify Gotify notifications work with JSON payload.

**Steps:**

1. Add new provider:
   - **Name:** "Gotify Test"
   - **Type:** Gotify
   - **URL:** `https://your-gotify-instance.com` (your test instance)
   - **Template:** Minimal
   - **Events:** All events
2. Click "Send Test Notification"
3. Check Gotify app/web interface

**Expected Result:**

- ✅ Notification received in Gotify
- ✅ Title and message displayed
- ✅ Default priority applied

**Actual Result:**

- [ ] Pass
- [ ] Fail (describe issue): \_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_

**Notes:**

---

### Test 3.2: Gotify with Extras and Priority

**Objective:** Verify Gotify extras field works correctly.

**Steps:**

1. Edit Gotify provider
2. Change **Template** to "Custom"
3. Enter custom template:

```json
{
  "title": "{{.Title}}",
  "message": "{{.Message}}",
  "priority": 8,
  "extras": {
    "client::display": {
      "contentType": "text/markdown"
    },
    "charon": {
      "event_type": "{{.EventType}}",
      "host": "{{.HostName}}",
      "timestamp": "{{.Timestamp}}"
    }
  }
}
```

1. Click "Send Test Notification"
2. Check Gotify notification

**Expected Result:**

- ✅ High priority notification (8)
- ✅ Markdown content rendered if supported
- ✅ Extras data included in payload
- ✅ All template variables replaced

**Actual Result:**

- [ ] Pass
- [ ] Fail (describe issue): \_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_

**Notes:**

---

## Section 4: Generic Webhook with Custom JSON

### Test 4.1: Generic Webhook Custom Structure

**Objective:** Verify generic webhooks support arbitrary JSON structures.

**Steps:**

1. Add new provider:
   - **Name:** "Generic Test"
   - **Type:** Generic
   - **URL:** `https://webhook.site/...` (use webhook.site for testing)
   - **Template:** Custom
2. Enter completely custom JSON:

```json
{
  "notification": {
    "type": "{{.EventType}}",
    "level": "{{.Severity}}",
    "content": {
      "heading": "{{.Title}}",
      "body": "{{.Message}}"
    },
    "metadata": {
      "source": "charon",
      "host": "{{.HostName}}",
      "time": "{{.Timestamp}}"
    }
  }
}
```

1. Click "Send Test Notification"
2. Check webhook.site to see received payload

**Expected Result:**

- ✅ Webhook receives POST request
- ✅ JSON structure matches custom template exactly
- ✅ All variables replaced with actual values
- ✅ No standard wrapper/envelope added

**Actual Result:**

- [ ] Pass
- [ ] Fail (describe issue): \_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_

**Notes:**

---

## Section 5: Uptime Monitoring - Basic Functionality

### Test 5.1: Enable Uptime Monitoring

**Objective:** Verify uptime monitoring can be enabled for a host.

**Steps:**

1. Navigate to Proxy Hosts
2. Edit an existing, working proxy host
3. Scroll to "Uptime Monitoring" section
4. Toggle "Enable Uptime Monitoring" to ON
5. Save changes
6. Wait 60 seconds for first check

**Expected Result:**

- ✅ Toggle saves successfully
- ✅ No errors on save
- ✅ Host status shows "up" after first check
- ✅ Last check timestamp updates

**Actual Result:**

- [ ] Pass
- [ ] Fail (describe issue): \_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_

**Notes:**

---

### Test 5.2: Uptime Status Consistency

**Objective:** Verify uptime status remains stable during page refreshes.

**Steps:**

1. With monitoring enabled from Test 5.1
2. Refresh the proxy hosts page 10 times over 5 minutes
3. Observe status on each refresh
4. Verify host is actually accessible (visit it in browser)

**Expected Result:**

- ✅ Status consistently shows "up" on all refreshes
- ✅ Host is actually accessible
- ✅ No false "down" status appears
- ✅ Timestamps update appropriately

**Actual Result:**

- [ ] Pass
- [ ] Fail (describe issue): \_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_

**Notes:**

---

### Test 5.3: Check Logs for Debouncing

**Objective:** Verify failure debouncing is working as expected.

**Steps:**

1. Run command:

```bash
docker logs charon 2>&1 | grep "failure_count\|waiting for threshold" | tail -50
```

1. Review log output
2. Temporarily disconnect host (e.g., stop container)
3. Wait for 2 check cycles (120 seconds)
4. Check logs again

**Expected Result:**

- ✅ Logs show failure_count incrementing
- ✅ First failure doesn't change status
- ✅ Second consecutive failure marks host as "down"
- ✅ Logs include "waiting for threshold" message on first failure

**Actual Result:**

- [ ] Pass
- [ ] Fail (describe issue): \_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_

**Notes:**

---

## Section 6: Uptime Monitoring - Failure Detection

### Test 6.1: Detect Real Failure

**Objective:** Verify monitoring correctly detects when a host goes down.

**Steps:**

1. Identify a monitored host
2. Stop the service/container behind that host
3. Wait for 2 check cycles (120 seconds)
4. Check Charon UI for status update
5. Check logs:

```bash
docker logs charon 2>&1 | grep "Host status changed" | tail -10
```

**Expected Result:**

- ✅ Status changes to "down" after 2 consecutive failures
- ✅ Notification sent (if configured)
- ✅ Status badge turns red in UI
- ✅ Logs show status transition

**Actual Result:**

- [ ] Pass
- [ ] Fail (describe issue): \_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_

**Notes:**

---

### Test 6.2: Detect Recovery

**Objective:** Verify monitoring detects when a host comes back up.

**Steps:**

1. With host still down from Test 6.1
2. Restart the service/container
3. Wait for 1 check cycle (60 seconds)
4. Check Charon UI for status update
5. Check logs

**Expected Result:**

- ✅ Status changes to "up" after first successful check
- ✅ Recovery notification sent (if configured)
- ✅ Status badge turns green in UI
- ✅ Failure count resets to 0
- ✅ Logs show recovery

**Actual Result:**

- [ ] Pass
- [ ] Fail (describe issue): \_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_

**Notes:**

---

### Test 6.3: Single Failure Ignored

**Objective:** Verify single transient failures don't trigger false alarms.

**Steps:**

1. Monitor logs in real-time:

```bash
docker logs -f charon 2>&1 | grep "Host TCP check"
```

1. Briefly pause host container (5 seconds):

```bash
docker pause <container_name>
sleep 5
docker unpause <container_name>
```

1. Observe logs for next 2 check cycles

**Expected Result:**

- ✅ First check after pause fails
- ✅ failure_count increments to 1
- ✅ Status remains "up"
- ✅ No notification sent
- ✅ Next check succeeds and resets failure_count to 0

**Actual Result:**

- [ ] Pass
- [ ] Fail (describe issue): \_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_

**Notes:**

---

## Section 7: Uptime Monitoring - Race Condition Prevention

### Test 7.1: Rapid API Calls During Check

**Objective:** Verify no race conditions occur during concurrent checks and API reads.

**Steps:**

1. Enable monitoring for 5+ hosts
2. Open browser console
3. Run script to make rapid API calls:

```javascript
for (let i = 0; i < 20; i++) {
  fetch('/api/proxy-hosts')
    .then(r => r.json())
    .then(d => console.log(`Request ${i}:`, d.hosts.map(h => h.status)))
  setTimeout(() => {}, 100 * i)
}
```

1. Observe console output and UI

**Expected Result:**

- ✅ All API calls succeed (no 500 errors)
- ✅ Status values are consistent across calls
- ✅ No database lock errors in logs
- ✅ UI remains responsive

**Actual Result:**

- [ ] Pass
- [ ] Fail (describe issue): \_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_

**Notes:**

---

### Test 7.2: Check All Hosts Synchronization

**Objective:** Verify all host checks complete before next cycle starts.

**Steps:**

1. Monitor logs:

```bash
docker logs -f charon 2>&1 | grep "All host checks completed\|Check cycle started"
```

1. Observe timing over 5 minutes
2. Count check cycles

**Expected Result:**

- ✅ "All host checks completed" appears after each cycle
- ✅ No overlapping check cycles
- ✅ Each cycle completes before next starts
- ✅ Check duration logged

**Actual Result:**

- [ ] Pass
- [ ] Fail (describe issue): \_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_

**Notes:**

---

## Section 8: Integration Tests

### Test 8.1: Uptime Down Notification via Discord

**Objective:** Verify uptime down events trigger Discord notifications correctly.

**Steps:**

1. Configure Discord provider to trigger on "Uptime Down" events
2. Enable uptime monitoring on a test host
3. Stop the host service
4. Wait for status to change to "down" (2 check cycles)
5. Check Discord channel

**Expected Result:**

- ✅ Discord notification received
- ✅ Message indicates host is down
- ✅ Template variables replaced correctly
- ✅ Event type shown as "uptime_down"

**Actual Result:**

- [ ] Pass
- [ ] Fail (describe issue): \_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_

**Notes:**

---

### Test 8.2: Uptime Recovery Notification via Slack

**Objective:** Verify uptime recovery events trigger Slack notifications.

**Steps:**

1. Configure Slack provider to trigger on "Uptime Recovery" events
2. With host still down from Test 8.1
3. Restart the host service
4. Wait for status to change to "up" (1 check cycle)
5. Check Slack channel

**Expected Result:**

- ✅ Slack notification received
- ✅ Message indicates host is back up
- ✅ Template variables replaced correctly
- ✅ Event type shown as "uptime_up"

**Actual Result:**

- [ ] Pass
- [ ] Fail (describe issue): \_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_

**Notes:**

---

### Test 8.3: Bulk Enable Uptime Monitoring

**Objective:** Verify bulk operations work for enabling uptime monitoring.

**Steps:**

1. Navigate to Proxy Hosts
2. Select 3+ hosts (checkboxes)
3. Click "Bulk Apply"
4. Toggle "Uptime Monitoring" to ON
5. Check "Apply to selected hosts"
6. Click "Apply Changes"
7. Verify each selected host individually

**Expected Result:**

- ✅ Bulk operation succeeds
- ✅ All selected hosts have monitoring enabled
- ✅ Status appears for all hosts after 60s
- ✅ No errors in UI or logs

**Actual Result:**

- [ ] Pass
- [ ] Fail (describe issue): \_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_

**Notes:**

---

### Test 8.4: Template Migration

**Objective:** Verify existing webhook providers can be upgraded to use JSON templates.

**Steps:**

1. Create a provider with type "webhook"
2. Save without template configuration
3. Send test notification (should work with basic format)
4. Edit provider and change type to "discord"
5. Select "Detailed" template
6. Save changes
7. Send test notification

**Expected Result:**

- ✅ Migration saves successfully
- ✅ Template field becomes available
- ✅ New notification uses rich format
- ✅ No data loss during migration

**Actual Result:**

- [ ] Pass
- [ ] Fail (describe issue): \_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_

**Notes:**

---

### Test 8.5: Template Variables All Services

**Objective:** Verify all template variables work across all services.

**Steps:**

1. Create providers for Discord, Slack, and Gotify
2. Use this custom template for each (adapted to service format):

```json
{
  "title": "Variables: {{.Title}}",
  "message": "{{.Message}}",
  "fields": {
    "event": "{{.EventType}}",
    "severity": "{{.Severity}}",
    "host": "{{.HostName}}",
    "timestamp": "{{.Timestamp}}",
    "color": "{{.Color}}",
    "priority": "{{.Priority}}"
  }
}
```

1. Send test notification to each
2. Verify all variables are replaced

**Expected Result:**

- ✅ All variables replaced with actual values
- ✅ No "{{.VariableName}}" remains in output
- ✅ Values are appropriate for event type
- ✅ Works consistently across all services

**Actual Result:**

- [ ] Pass
- [ ] Fail (describe issue): \_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_

**Notes:**

---

### Test 8.6: Performance with Many Hosts

**Objective:** Verify monitoring performs well with many hosts.

**Steps:**

1. Enable uptime monitoring for 10+ proxy hosts
2. Monitor CPU usage:

```bash
docker stats charon
```

1. Check log timing:

```bash
docker logs charon 2>&1 | grep "All host checks completed" | tail -10
```

1. Observe check duration over 5 minutes

**Expected Result:**

- ✅ CPU usage remains reasonable (<50% sustained)
- ✅ Check cycles complete in <30 seconds
- ✅ No timeout errors
- ✅ UI remains responsive

**Actual Result:**

- [ ] Pass
- [ ] Fail (describe issue): \_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_

**Notes:**

---

## Section 9: Edge Cases & Error Handling

### Test 9.1: Invalid Template Variable

**Objective:** Verify graceful handling of unknown template variables.

**Steps:**

1. Create custom template with invalid variable:

```json
{
  "title": "{{.Title}}",
  "custom_field": "{{.NonExistentVariable}}"
}
```

1. Click "Validate Template"
2. Attempt to save

**Expected Result:**

- ✅ Validation error displayed
- ✅ Error message indicates which variable is invalid
- ✅ Cannot save invalid template
- ✅ Provider not created/updated

**Actual Result:**

- [ ] Pass
- [ ] Fail (describe issue): \_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_

**Notes:**

---

### Test 9.2: Network Failure During Notification

**Objective:** Verify system handles notification send failures gracefully.

**Steps:**

1. Create Discord provider with invalid webhook URL
2. Trigger an actual event (e.g., stop monitored host)
3. Check logs:

```bash
docker logs charon 2>&1 | grep "Failed to send.*notification"
```

1. Verify system continues operating

**Expected Result:**

- ✅ Error logged for failed notification
- ✅ System continues operating normally
- ✅ Other providers still receive notifications
- ✅ No crash or hang

**Actual Result:**

- [ ] Pass
- [ ] Fail (describe issue): \_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_

**Notes:**

---

### Test 9.3: Extremely Slow Host Response

**Objective:** Verify timeout handling for slow hosts.

**Steps:**

1. Create proxy host pointing to intentionally slow service
2. Enable uptime monitoring
3. Monitor logs for timeout behavior
4. Check if status reflects timeout correctly

**Expected Result:**

- ✅ Check times out after 10 seconds
- ✅ Retry attempts logged
- ✅ After 2 consecutive timeouts, marked as "down"
- ✅ No infinite hangs

**Actual Result:**

- [ ] Pass
- [ ] Fail (describe issue): \_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_

**Notes:**

---

### Test 9.4: Concurrent Configuration Changes

**Objective:** Verify no issues when modifying monitoring config during checks.

**Steps:**

1. Enable monitoring for a host
2. Wait for first check to start
3. Immediately disable monitoring
4. Re-enable monitoring
5. Check for any errors or inconsistent state

**Expected Result:**

- ✅ No errors during rapid config changes
- ✅ Final state matches UI selection
- ✅ No orphaned checks or zombie goroutines
- ✅ Status reflects current config

**Actual Result:**

- [ ] Pass
- [ ] Fail (describe issue): \_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_

**Notes:**

---

## Test Summary & Sign-off

### Overall Results

**Total Tests:** 28
**Passed:** \_\_\_
**Failed:** \_\_\_
**Blocked:** \_\_\_
**Not Run:** \_\_\_

### Critical Issues Found

1. \_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_
2. \_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_
3. \_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_

### Non-Critical Issues Found

1. \_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_
2. \_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_
3. \_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_

### Recommendations

- [ ] Ready for production release
- [ ] Requires minor fixes before release
- [ ] Requires major fixes before release
- [ ] Not ready for release

### Tester Sign-off

**Tested by:** \_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_
**Date:** \_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_
**Environment:** \_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_
**Build Version:** \_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_

### Notes

\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_\_

---

## Appendix: Test Environment Setup

### Prerequisites Checklist

- [ ] Charon container running latest build
- [ ] Docker socket mounted (`/var/run/docker.sock`)
- [ ] Test Discord webhook configured
- [ ] Test Slack webhook configured
- [ ] Test Gotify instance accessible
- [ ] At least 2 proxy hosts configured and working
- [ ] Admin credentials available
- [ ] Network access to all test services

### Test Data Setup

**Discord Webhook:**

```
URL: https://discord.com/api/webhooks/YOUR_WEBHOOK_ID/YOUR_WEBHOOK_TOKEN
Channel: #charon-test
```

**Slack Webhook:**

```
URL: https://hooks.slack.com/services/YOUR/WEBHOOK/PATH
Channel: #charon-test
```

**Gotify Instance:**

```
URL: https://gotify.example.com
Token: YOUR_APP_TOKEN
```

**Test Proxy Hosts:**

```
Host 1: test-app-1.local (port 8081)
Host 2: test-app-2.local (port 8082)
```

### Cleanup After Testing

```bash
# Remove test providers
# (via UI: Settings → Notifications → Delete)

# Disable uptime monitoring
# (via UI: Proxy Hosts → Edit → Toggle OFF)

# Review logs for any errors
docker logs charon 2>&1 | grep "ERROR\|WARN" | tail -50

# Optional: Reset test environment
docker restart charon
```

---

**End of Test Plan**
