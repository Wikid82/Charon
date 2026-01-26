---
title: Live Logs & Notifications User Guide
description: Real-time security monitoring and notification configuration for Charon. Stream logs via WebSocket and configure webhooks.
---

## Live Logs & Notifications User Guide

**Quick links:**

- [Overview](#overview)
- [Accessing Live Logs](#accessing-live-logs)
- [Configuring Notifications](#configuring-notifications)
- [Filtering Logs](#filtering-logs)
- [Webhook Integrations](#webhook-integrations)
- [Troubleshooting](#troubleshooting)

---

## Overview

Charon's Live Logs & Notifications feature gives you real-time visibility into security events. See attacks as they happen, not hours later. Get notified immediately when critical threats are detected.

**What you get:**

- \u2705 Real-time security event streaming
- \u2705 Configurable notifications (webhooks, email)
- \u2705 Client-side and server-side filtering
- \u2705 Pause, resume, and clear controls
- \u2705 Auto-scroll with manual scroll override
- \u2705 WebSocket-based (no polling, instant updates)

---

## Accessing Live Logs

### Step 1: Enable Cerberus

Live logs are part of the Cerberus security suite. If you haven't enabled it yet:

1. Go to **System Settings** \u2192 **Optional Features**
2. Toggle **Cerberus Security Suite** to enabled
3. Wait for the system to initialize

### Step 2: Open the Dashboard

1. Click **Cerberus** in the sidebar
2. Click **Dashboard**
3. Scroll to the **Live Activity** section (bottom of page)

You'll see a terminal-like interface showing real-time security events.

### What You'll See

Each log entry shows:

- **Timestamp** \u2014 When the event occurred (ISO 8601 format)
- **Level** \u2014 Severity: debug, info, warn, error
- **Source** \u2014 Component that generated the event (waf, crowdsec, acl)
- **Message** \u2014 Human-readable description
- **Details** \u2014 Structured data (IP addresses, rule IDs, request URIs)

**Example log entry:**

```
[2025-12-09T10:30:45Z] ERROR [waf] WAF blocked SQL injection attempt
  IP: 203.0.113.42
  Rule: 942100
  URI: /api/users?id=1' OR '1'='1
  User-Agent: curl/7.68.0
```

---

## Configuring Notifications

### Step 1: Open Notification Settings

1. Go to **Cerberus Dashboard**
2. Click **\"Notification Settings\"** button (top-right corner)
3. A modal dialog will open

### Step 2: Basic Configuration

**Enable Notifications:**

- Toggle the master switch to enable alerts

**Set Minimum Log Level:**

- Choose the minimum severity that triggers notifications
- **Recommended:** Start with `error` to avoid alert fatigue
- Options:
  - `error` \u2014 Only critical security events
  - `warn` \u2014 Important warnings and errors
  - `info` \u2014 Normal operations plus warnings/errors
  - `debug` \u2014 Everything (very noisy, not recommended)

### Step 3: Choose Event Types

Select which types of security events trigger notifications:

- **\u2611\ufe0f WAF Blocks** \u2014 Firewall blocks (SQL injection, XSS, etc.)
  - Recommended: **Enabled**
  - Use case: Detect active attacks in real-time

- **\u2611\ufe0f ACL Denials** \u2014 Access control rule violations
  - Recommended: **Enabled** if you use geo-blocking or IP filtering
  - Use case: Detect unexpected access attempts or misconfigurations

- **\u2610 Rate Limit Hits** \u2014 Traffic threshold violations
  - Recommended: **Disabled** for most users (can be noisy)
  - Use case: Detect DDoS attempts or scraping bots

### Step 4: Add Delivery Methods

**Webhook URL (recommended):**

- Paste your Discord/Slack webhook URL
- Must be HTTPS (HTTP not allowed for security)
- Format: `https://hooks.slack.com/services/...` or `https://discord.com/api/webhooks/...`

**Email Recipients (future feature):**

- Comma-separated list: `admin@example.com, security@example.com`
- Requires SMTP configuration (not yet implemented)

### Step 5: Save Settings

Click **\"Save\"** to apply your configuration. Changes take effect immediately.

---

## Filtering Logs

### Client-Side Filtering

The Live Log Viewer includes built-in filtering:

1. **Text Search:**
   - Type in the filter box to search by any text
   - Searches message, IP addresses, URIs, and other fields
   - Case-insensitive
   - Updates in real-time

2. **Level Filter:**
   - Click level badges to filter by severity
   - Show only errors, warnings, etc.

3. **Source Filter:**
   - Filter by component (WAF, CrowdSec, ACL)

**Example:** To see only WAF errors from a specific IP:

- Type `203.0.113.42` in the search box
- Click the \"ERROR\" badge
- Results update instantly

### Server-Side Filtering

For better performance with high-volume logs, use server-side filtering:

**Via URL parameters:**

- `?level=error` \u2014 Only error-level logs
- `?source=waf` \u2014 Only WAF-related events
- `?source=cerberus` \u2014 All Cerberus security events

**Example:** To connect directly with filters:

```javascript
const ws = new WebSocket('ws://localhost:8080/api/v1/logs/live?level=error&source=waf');
```

**When to use server-side filtering:**

- Reduces bandwidth usage
- Better performance under heavy load
- Useful for automated monitoring scripts

---

## Webhook Integrations

### Discord

**Step 1: Create Discord Webhook**

1. Open your Discord server
2. Go to **Server Settings** \u2192 **Integrations**
3. Click **Webhooks** \u2192 **New Webhook**
4. Name it \"Charon Security\" (or similar)
5. Choose the channel (e.g., #security-alerts)
6. Click **Copy Webhook URL**

**Step 2: Add to Charon**

1. Open Charon **Notification Settings**
2. Paste the webhook URL in **Webhook URL** field
3. Save settings

**Step 3: Test**

Trigger a security event (e.g., try to access a blocked URL) and check your Discord channel.

**Discord message format:**

Charon sends formatted Discord embeds:

- \ud83d\udee1\ufe0f Icon and title based on event type
- Color-coded severity (red for errors, yellow for warnings)
- Structured fields (IP, Rule, URI)
- Timestamp

### Slack

**Step 1: Create Slack Incoming Webhook**

1. Go to <https://api.slack.com/apps>
2. Click **Create New App** \u2192 **From scratch**
3. Name it \"Charon Security\" and select your workspace
4. Click **Incoming Webhooks** \u2192 Toggle **Activate Incoming Webhooks**
5. Click **Add New Webhook to Workspace**
6. Choose the channel (e.g., #security)
7. Click **Copy Webhook URL**

**Step 2: Add to Charon**

1. Open Charon **Notification Settings**
2. Paste the webhook URL in **Webhook URL** field
3. Save settings

**Slack message format:**

Charon sends JSON payloads compatible with Slack's message format:

```json
{
  "text": "WAF Block: SQL injection attempt blocked",
  "attachments": [{
    "color": "danger",
    "fields": [
      { "title": "IP", "value": "203.0.113.42", "short": true },
      { "title": "Rule", "value": "942100", "short": true }
    ]
  }]
}
```

### Custom Webhooks

**Requirements:**

- Must accept POST requests
- Must use HTTPS (HTTP not supported)
- Should return 2xx status code on success

**Payload format:**

Charon sends JSON POST requests:

```json
{
  "event_type": "waf_block",
  "severity": "error",
  "timestamp": "2025-12-09T10:30:45Z",
  "message": "WAF blocked SQL injection attempt",
  "details": {
    "ip": "203.0.113.42",
    "rule_id": "942100",
    "request_uri": "/api/users?id=1' OR '1'='1",
    "user_agent": "curl/7.68.0"
  }
}
```

**Headers:**

```
Content-Type: application/json
User-Agent: Charon/1.0
```

**Example custom webhook handler (Express.js):**

```javascript
app.post('/charon-webhook', (req, res) => {
  const event = req.body;

  console.log(`Security Event: ${event.event_type}`);
  console.log(`Severity: ${event.severity}`);
  console.log(`Message: ${event.message}`);
  console.log(`Details:`, event.details);

  // Process the event (store in DB, send to SIEM, etc.)

  res.status(200).json({ received: true });
});
```

---

## Viewer Controls

### Pause/Resume

**Pause:**

- Click the **\"Pause\"** button to stop streaming
- Useful for examining specific events
- New logs are buffered but not displayed

**Resume:**

- Click **\"Resume\"** to continue streaming
- Buffered logs appear instantly

### Clear

- Click **\"Clear\"** to remove all current log entries
- Does NOT affect the log stream (new entries continue to appear)
- Useful for starting fresh after reviewing old events

### Auto-Scroll

**Enabled (default):**

- Viewer automatically scrolls to show latest entries
- New logs always visible

**Disabled:**

- Scroll back to review older entries
- Auto-scroll pauses automatically when you scroll up
- Resumes when you scroll back to the bottom

---

## Troubleshooting

### No Logs Appearing

**Check Cerberus status:**

1. Go to **Cerberus Dashboard**
2. Verify Cerberus is enabled
3. Check that at least one security feature is active (WAF, CrowdSec, or ACL)

**Check browser console:**

1. Open Developer Tools (F12)
2. Look for WebSocket connection errors
3. Common issues:
   - WebSocket connection refused \u2192 Check Charon is running
   - 401 Unauthorized \u2192 Authentication issue (when auth is enabled)
   - CORS error \u2192 Check allowed origins configuration

**Check filters:**

- Clear all filters (search box and level/source badges)
- Server-side filters in URL parameters may be too restrictive

**Generate test events:**

- Try accessing a URL with SQL injection pattern: `https://yoursite.com/api?id=1' OR '1'='1`
- Enable WAF in \"Block\" mode to see blocks
- Check CrowdSec is running to see decision logs

### WebSocket Disconnects

**Symptoms:**

- Logs stop appearing
- \"Disconnected\" message shows

**Causes:**

- Network interruption
- Server restart
- Idle timeout (rare\u2014ping keeps connection alive)

**Solution:**

- Live Log Viewer automatically reconnects
- If it doesn't, refresh the page
- Check network connectivity

### Notifications Not Sending

**Check notification settings:**

1. Open **Notification Settings**
2. Verify **Enable Notifications** is toggled on
3. Check **Minimum Log Level** isn't too restrictive
4. Verify at least one event type is enabled

**Check webhook URL:**

- Must be HTTPS (HTTP not supported)
- Test the URL directly with `curl`:

  ```bash
  curl -X POST https://your-webhook-url \
    -H "Content-Type: application/json" \
    -d '{"test": "message"}'
  ```

- Check webhook provider's documentation for correct format

**Check event severity:**

- If minimum level is \"error\", only errors trigger notifications
- Lower to \"warn\" or \"info\" to see more notifications
- Generate a test error event to verify

**Check logs:**

- Look for webhook delivery errors in Charon logs
- Common errors:
  - Connection timeout \u2192 Webhook URL unreachable
  - 4xx status \u2192 Webhook authentication or format error
  - 5xx status \u2192 Webhook provider error

### Too Many Notifications

**Solution 1: Increase minimum log level**

- Change from \"info\" to \"warn\" or \"error\"
- Reduces notification volume significantly

**Solution 2: Disable noisy event types**

- Disable \"Rate Limit Hits\" if you don't need them
- Keep only \"WAF Blocks\" and \"ACL Denials\"

**Solution 3: Use server-side filtering**

- Filter by source (e.g., only WAF blocks)
- Filter by level (e.g., only errors)

**Solution 4: Rate limiting (future feature)**

- Charon will support rate-limited notifications
- Example: Maximum 10 notifications per minute

### Logs Missing Information

**Incomplete log entries:**

- Check that the source component is logging all necessary fields
- Update to latest Charon version (fields may have been added)

**Timestamps in wrong timezone:**

- All timestamps are UTC (ISO 8601 / RFC3339 format)
- Convert to your local timezone in your webhook handler if needed

**IP addresses showing as localhost:**

- Check reverse proxy configuration
- Ensure `X-Forwarded-For` or `X-Real-IP` headers are set

---

## Best Practices

### For Security Monitoring

1. **Start with \"error\" level only**
   - Avoid alert fatigue
   - Gradually lower to \"warn\" if needed

2. **Enable all critical event types**
   - WAF Blocks: Always enable
   - ACL Denials: Enable if using geo-blocking or IP filtering
   - Rate Limits: Enable only if actively monitoring for DDoS

3. **Use Discord/Slack for team alerts**
   - Create dedicated security channel
   - @mention security team on critical events

4. **Review logs regularly**
   - Check Live Log Viewer daily
   - Look for patterns (same IP, same rule)
   - Adjust ACL or WAF rules based on findings

5. **Test your configuration**
   - Trigger test events monthly
   - Verify notifications arrive
   - Update webhook URLs if they change

### For Privacy & Compliance

1. **Secure webhook endpoints**
   - Always use HTTPS
   - Use webhook secrets/authentication when available
   - Don't log webhook URLs in plaintext

2. **Respect data privacy**
   - Log retention: Live logs are in-memory only (not persisted)
   - IP addresses: Consider personal data under GDPR
   - Request URIs: May contain sensitive data

3. **Access control**
   - Limit who can view live logs
   - Implement authentication (when available)
   - Use role-based access control

4. **Third-party data sharing**
   - Webhook notifications send data to Discord, Slack, etc.
   - Review their privacy policies
   - Consider self-hosted alternatives for sensitive data

---

## Advanced Usage

### API Integration

**Connect to WebSocket programmatically:**

```javascript
const API_BASE = 'ws://localhost:8080/api/v1';
const ws = new WebSocket(`${API_BASE}/logs/live?source=waf&level=error`);

ws.onopen = () => {
  console.log('Connected to live logs');
};

ws.onmessage = (event) => {
  const log = JSON.parse(event.data);

  // Process log entry
  if (log.level === 'error') {
    console.error(`Security alert: ${log.message}`);
    // Send to your monitoring system
  }
};

ws.onerror = (error) => {
  console.error('WebSocket error:', error);
};

ws.onclose = () => {
  console.log('Disconnected from live logs');
  // Implement reconnection logic
};
```

### Custom Alerting Logic

**Example: Alert only on repeated attacks from same IP:**

```javascript
const attackCounts = new Map();

ws.onmessage = (event) => {
  const log = JSON.parse(event.data);
  const ip = log.fields?.ip;

  if (!ip) return;

  const count = (attackCounts.get(ip) || 0) + 1;
  attackCounts.set(ip, count);

  // Alert if same IP attacks 5+ times in a row
  if (count >= 5) {
    sendCustomAlert(`IP ${ip} has attacked ${count} times!`);
    attackCounts.delete(ip); // Reset counter
  }
};
```

### Integration with SIEM

**Forward logs to Splunk, ELK, or other SIEM systems:**

```javascript
ws.onmessage = (event) => {
  const log = JSON.parse(event.data);

  // Forward to SIEM via HTTP
  fetch('https://your-siem.com/api/events', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      source: 'charon',
      timestamp: log.timestamp,
      severity: log.level,
      message: log.message,
      fields: log.fields
    })
  });
};
```

---

## Next Steps

- **[Security Guide](https://wikid82.github.io/charon/security)** \u2014 Learn about Cerberus features
- **[API Documentation](https://wikid82.github.io/charon/api)** \u2014 Full API reference
- **[Features Overview](https://wikid82.github.io/charon/features)** \u2014 See all Charon capabilities
- **[WebSocket Troubleshooting](troubleshooting/websocket.md)** — Fix WebSocket connection issues
- **[Troubleshooting](https://wikid82.github.io/charon/troubleshooting)** \u2014 Common issues and solutions

---

## Need Help?

- **GitHub Issues:** <https://github.com/Wikid82/charon/issues>
- **Discussions:** <https://github.com/Wikid82/charon/discussions>
- **Documentation:** <https://wikid82.github.io/charon/>
