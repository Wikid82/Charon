# Security Features

Charon includes **Cerberus**, a security system that protects your websites. It's **enabled by default** so your sites are protected from the start.

You can disable it in **System Settings → Optional Features** if you don't need it, or configure it using this guide. The sidebar now shows **Cerberus → Dashboard**; the page header reads **Cerberus Dashboard**.

Want the quick reference? See <https://wikid82.github.io/charon/security>.

---

## What Is Cerberus?

Think of Cerberus as a guard dog for your websites. It has three heads (in Greek mythology), and each head watches for different threats:

1. **CrowdSec** — Blocks bad IP addresses
2. **WAF (Web Application Firewall)** — Blocks bad requests
3. **Access Lists** — You decide who gets in

---

## Turn It On (The Safe Way)

**Step 1: Start in "Monitor" Mode**

This means Cerberus watches but doesn't block anyone yet.

Add this to your `docker-compose.yml`:

```yaml
environment:
  - CERBERUS_SECURITY_WAF_MODE=monitor
  - CERBERUS_SECURITY_CROWDSEC_MODE=local
```

Restart Charon:

```bash
docker-compose restart
```

**Step 2: Watch the Logs**

Check "Security" in the sidebar. You'll see what would have been blocked. If it looks right, move to Step 3.

**Step 3: Turn On Blocking**

Change `monitor` to `block`:

```yaml
environment:
  - CERBERUS_SECURITY_WAF_MODE=block
```

Restart again. Now bad guys actually get blocked.

---

## CrowdSec (Block Bad IPs)

**What it does:** Thousands of people share information about attackers. When someone tries to hack one of them, everyone else blocks that attacker too.

**Why you care:** If someone is attacking servers in France, you block them before they even get to your server in California.

### How to Enable It

**Via Web UI (Recommended):**

1. Navigate to **Security** dashboard in the sidebar
2. Find the **CrowdSec** card
3. Toggle the switch to **ON**
4. Wait 10-15 seconds for the Local API (LAPI) to start
5. Verify the status badge shows "Active" with a running PID

✅ That's it! CrowdSec starts automatically and begins blocking bad IPs.

⚠️ **DEPRECATED:** Environment variables like `CHARON_SECURITY_CROWDSEC_MODE=local` are **no longer used**. CrowdSec is now GUI-controlled, just like WAF, ACL, and Rate Limiting. If you have these environment variables in your docker-compose.yml, remove them and use the GUI toggle instead. See [Migration Guide](migration-guide.md).

**What you'll see:** The Cerberus pages show blocked IPs and why they were blocked.

### Enroll with CrowdSec Console (optional)

**Prerequisites:**

✅ **CrowdSec must be enabled** via the GUI toggle (see above)
✅ **LAPI must be running** — Verify with: `docker exec charon cscli lapi status`
✅ **Feature flag enabled** — `crowdsec_console_enrollment` must be ON
✅ **Valid enrollment token** — Obtain from crowdsec.net

**Enrollment Steps:**

1. Ensure CrowdSec is **enabled** and **LAPI is running** (check prerequisites above)
2. Navigate to **Cerberus → CrowdSec**
3. Enable the feature flag `crowdsec_console_enrollment` if not already enabled
4. Click **Enroll with CrowdSec Console**
5. Paste the enrollment key from crowdsec.net
6. Click **Submit**
7. Wait for confirmation (this may take 30-60 seconds)
8. Verify your instance appears on crowdsec.net dashboard

**Important Notes:**

- 🚨 Enrollment **requires an active LAPI connection**. If LAPI is not running, the enrollment will show "enrolled" locally but won't register on crowdsec.net.
- ✅ Enrollment tokens are **reusable** — you can re-submit the same token if enrollment fails
- 🔒 Charon stores the enrollment secret internally (not logged or echoed)
- ♻️ After enrollment, the Console status shows in the CrowdSec card
- 🗑️ You can revoke enrollment from either Charon or crowdsec.net

**Troubleshooting:**

If enrollment shows "enrolled" locally but doesn't appear on crowdsec.net:

1. **Check LAPI status:**
   ```bash
   docker exec charon cscli lapi status
   ```
   Expected: `✓ You can successfully interact with Local API (LAPI)`

2. **If LAPI is not running:**
   - Go to Security dashboard
   - Toggle CrowdSec OFF, then ON
   - Wait 15 seconds
   - Re-check LAPI status

3. **Re-submit enrollment token:**
   - Same token works (enrollment tokens are reusable)
   - Go to Cerberus → CrowdSec
   - Paste token and submit again

4. **Check logs:**
   ```bash
   docker logs charon | grep crowdsec
   ```

See also: [CrowdSec Troubleshooting Guide](troubleshooting/crowdsec.md)

### Hub Presets (Configuration Packages)

Charon lets you install security configurations (Collections, Parsers, Scenarios) directly from the CrowdSec Hub.

- **Search & Sort:** Use the search bar to find specific packages (e.g., "wordpress", "nginx"). Sort by name, status, or popularity.
- **One-Click Install:** Click "Install" on any package. Charon handles the download and configuration.
- **Safe Apply:** Changes are applied safely. If something goes wrong, Charon can restore the previous configuration.
- **Updates:** Charon checks for updates automatically. You'll see an "Update" button when a new version is available.

### Troubleshooting

Having trouble with CrowdSec? Check out the [CrowdSec Troubleshooting Guide](troubleshooting/crowdsec.md).

---

## WAF (Block Bad Behavior)

**What it does:** Looks at every request and checks if it's trying to do something nasty—like inject SQL code or run JavaScript attacks.

**Why you care:** Even if your app has a bug, the WAF might catch the attack first.

### How to Enable It

```yaml
environment:
  - CERBERUS_SECURITY_WAF_MODE=block
```

**Start with `monitor` first!** This lets you see what would be blocked without actually blocking it.

---

## Access Lists (You Decide Who Gets In)

Access lists let you block or allow specific countries, IP addresses, or networks.

### Example 1: Block a Country

**Scenario:** You only need access from the US, so block everyone else.

1. Go to **Access Lists**
2. Click **Add List**
3. Name it "US Only"
4. **Type:** Geo Whitelist
5. **Countries:** United States
6. **Assign to your proxy host**

Now only US visitors can access that website. Everyone else sees "Access Denied."

### Example 2: Private Network Only

**Scenario:** Your admin panel should only work from your home network.

1. Create an access list
2. **Type:** Local Network Only
3. Assign it to your admin panel proxy

Now only devices on `192.168.x.x` or `10.x.x.x` can access it. The public internet can't.

### Example 3: Block One Country

**Scenario:** You're getting attacked from one specific country.

1. Create a list
2. **Type:** Geo Blacklist
3. Pick the country
4. Assign to the targeted website

---

## Certificate Management Security

**What it protects:** Certificate deletion is a destructive operation that requires proper authorization.

**How it works:**

- Certificates cannot be deleted while in use by proxy hosts (conflict error)
- Automatic backup is created before any certificate deletion
- Authentication required (when auth is implemented)

**Backup & Recovery:**

- Every certificate deletion triggers an automatic backup
- Find backups in the "Backups" page
- Restore from backup if you accidentally delete the wrong certificate

**Best Practice:**

- Review which proxy hosts use a certificate before deleting it
- When deleting proxy hosts, use the cleanup prompt to delete orphaned certificates
- Keep custom certificates you might reuse later

---

## Don't Lock Yourself Out

**Problem:** If you turn on security and misconfigure it, you might block yourself.

**Solution:** Add your IP to the "Admin Whitelist" first.

### How to Add Your IP

1. Go to **Settings → Security**
2. Find "Admin Whitelist"
3. Add your IP address (find it at [ifconfig.me](https://ifconfig.me))
4. Save

Now you can never accidentally block yourself.

### Break-Glass Token (Emergency Exit)

If you do lock yourself out:

1. Log into your server directly (SSH)
2. Run this command:

```bash
docker exec charon charon break-glass
```

It generates a one-time token that lets you disable security and get back in.

---

## Recommended Settings by Service Type

### Internal Admin Panels (Router, Pi-hole, etc.)

```
Access List: Local Network Only
```

Blocks all public internet traffic.

### Personal Blog or Portfolio

```
No access list
WAF: Enabled
CrowdSec: Enabled
```

Keep it open for visitors, but protect against attacks.

### Password Manager (Vaultwarden, etc.)

```
Access List: IP Whitelist (your home IP)
Or: Geo Whitelist (your country only)
```

Most restrictive. Only you can access it.

### Media Server (Plex, Jellyfin)

```
Access List: Geo Blacklist (high-risk countries)
CrowdSec: Enabled
```

Allows friends to access, blocks obvious threat countries.

---

## Check If It's Working

1. Go to **Security → Decisions** in the sidebar
2. You'll see a list of recent blocks
3. If you see activity, it's working!

---

## Live Security Monitoring

### Live Log Viewer

**What it does:** Stream security events in real-time directly in the Cerberus Dashboard.

**Where to find it:** Cerberus → Dashboard → Scroll to "Live Activity" section

**What you'll see:**

- Real-time WAF blocks and detections
- CrowdSec decisions as they happen
- ACL denials (geo-blocking, IP filtering)
- Rate limiting events
- All Cerberus security activity

**Controls:**

- **Pause** — Stop the stream to examine specific events
- **Clear** — Remove old entries from the display
- **Auto-scroll** — Automatically follow new events
- **Filter** — Search logs by text, level, or source

**How to use it:**

1. Open Cerberus Dashboard
2. Scroll to the Live Activity section
3. Watch events appear in real-time
4. Click "Pause" to stop streaming and review events
5. Use the filter box to search for specific IPs, rules, or messages
6. Click "Clear" to remove old entries

**Technical details:**

- Uses WebSocket for real-time streaming (no polling)
- Keeps last 500 entries by default (configurable)
- Server-side filtering reduces bandwidth
- Automatic reconnection on disconnect

### Security Notifications

**What it does:** Sends alerts when critical security events occur.

**Why you care:** Get immediate notification of attacks or suspicious activity without watching the dashboard 24/7.

#### Configure Notifications

1. Go to **Cerberus Dashboard**
2. Click **"Notification Settings"** button (top-right)
3. Configure your preferences:

**Basic Settings:**

- **Enable Notifications** — Master toggle
- **Minimum Log Level** — Choose: debug, info, warn, or error
  - `error` — Only critical events (recommended)
  - `warn` — Important warnings and errors
  - `info` — Normal operations plus warnings/errors
  - `debug` — Everything (very noisy, not recommended)

**Event Types:**

- **WAF Blocks** — Notify when firewall blocks an attack
- **ACL Denials** — Notify when access control rules block requests
- **Rate Limit Hits** — Notify when traffic thresholds are exceeded

**Delivery Methods:**

- **Webhook URL** — Send to Discord, Slack, or custom integrations
- **Email Recipients** — Comma-separated email addresses (requires SMTP setup)

#### Webhook Integration

**Security considerations:**

1. **Use HTTPS webhooks only** — Never send security alerts over unencrypted HTTP
2. **Validate webhook endpoints** — Ensure the URL is correct before saving
3. **Protect webhook secrets** — If your webhook requires authentication, use environment variables
4. **Rate limiting** — Charon does NOT rate-limit webhook calls; configure your webhook provider to handle bursts
5. **Sensitive data** — Webhook payloads may contain IP addresses, request URIs, and user agents

**Supported platforms:**

- Discord (use webhook URL from Server Settings → Integrations)
- Slack (create incoming webhook in Slack Apps)
- Microsoft Teams (use incoming webhook connector)
- Custom HTTPS endpoints (any server that accepts POST requests)

**Webhook payload example:**

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

**Discord webhook format:**

Charon automatically formats notifications for Discord:

```json
{
  "embeds": [{
    "title": "🛡️ WAF Block",
    "description": "SQL injection attempt blocked",
    "color": 15158332,
    "fields": [
      { "name": "IP Address", "value": "203.0.113.42", "inline": true },
      { "name": "Rule", "value": "942100", "inline": true },
      { "name": "URI", "value": "/api/users?id=1' OR '1'='1" }
    ],
    "timestamp": "2025-12-09T10:30:45Z"
  }]
}
```

**Testing your webhook:**

1. Add your webhook URL in Notification Settings
2. Save the settings
3. Trigger a test event (try accessing a blocked URL)
4. Check your Discord/Slack channel for the notification

**Troubleshooting webhooks:**

- No notifications? Check webhook URL is correct and HTTPS
- Wrong format? Verify your platform's webhook documentation
- Too many notifications? Increase minimum log level to "error" only
- Notifications delayed? Check your network connection and firewall rules

### Log Privacy Considerations

**What's logged:**

- IP addresses of blocked requests
- Request URIs and query parameters
- User-Agent strings
- Rule IDs that triggered blocks
- Timestamps of security events

**What's NOT logged:**

- Request bodies (POST data)
- Authentication credentials
- Session cookies
- Response bodies

**Privacy best practices:**

1. **Filter logs before sharing** — Remove sensitive IPs or URIs before sharing logs externally
2. **Secure webhook endpoints** — Use HTTPS and authenticate webhook requests
3. **Respect GDPR** — IP addresses are personal data in some jurisdictions
4. **Retention policy** — Live logs are kept for the current session only (not persisted to disk)
5. **Access control** — Only authenticated users can access live logs (when auth is implemented)

**Compliance notes:**

- Live log streaming does NOT persist logs to disk
- Logs are only stored in memory during active WebSocket sessions
- Notification webhooks send log data to third parties (Discord, Slack)
- Email notifications may contain sensitive data

---

## Turn It Off

If security is causing problems:

**Option 1: Via Web UI**

1. Go to **Settings → Security**
2. Toggle "Enable Cerberus" off

**Option 2: Via Environment Variable**

Remove the security lines from `docker-compose.yml` and restart.

---

## Common Questions

### "Will this slow down my websites?"

No. The checks happen in milliseconds. Humans won't notice.

### "Can I whitelist specific paths?"

Not yet, but it's planned. For now, access lists apply to entire websites.

### "What if CrowdSec blocks a legitimate visitor?"

You can manually unblock IPs in the Security → Decisions page.

### "Do I need all three security features?"

No. Use what you need:

- **Just starting?** CrowdSec only
- **Public service?** CrowdSec + WAF
- **Private service?** Access Lists only

---

## Zero-Day Protection

### What We Protect Against

**Web Application Exploits:**

- ✅ SQL Injection (SQLi) — even zero-days using SQL syntax
- ✅ Cross-Site Scripting (XSS) — new XSS vectors caught by pattern matching
- ✅ Remote Code Execution (RCE) — command injection patterns
- ✅ Path Traversal — attempts to read system files
- ⚠️ CrowdSec — protects hours/days after first exploitation (crowd-sourced)

### How It Works

The WAF (Coraza) uses the OWASP Core Rule Set to detect attack patterns. Even if the exploit is brand new, the pattern is usually recognizable.

**Example:** A zero-day SQLi exploit discovered today:

```
https://yourapp.com/search?q=' OR '1'='1
```

- **Pattern:** `' OR '1'='1` matches SQL injection signature
- **Action:** WAF blocks request → attacker never reaches your database

### What We DON'T Protect Against

- ❌ Zero-days in Charon itself (keep Charon updated)
- ❌ Zero-days in Docker, Linux kernel (keep OS updated)
- ❌ Logic bugs in your application code (need code reviews)
- ❌ Insider threats (need access controls + auditing)
- ❌ Social engineering (need user training)

### Recommendation: Defense in Depth

1. **Enable all Cerberus layers:**
   - CrowdSec (IP reputation)
   - ACLs (restrict access by geography/IP)
   - WAF (request inspection)
   - Rate Limiting (slow down attacks)

2. **Keep everything updated:**
   - Charon (watch GitHub releases)
   - Docker images (rebuild regularly)
   - Host OS (enable unattended-upgrades)

3. **Monitor security logs:**
   - Check "Security → Decisions" weekly
   - Set up alerts for high block rates

---

## Testing & Validation

### Integration Testing

Cerberus includes a comprehensive integration test suite to validate all security features work correctly together.

**Run the full test suite:**

```bash
# Integration script
bash scripts/cerberus_integration.sh

# Go test wrapper
cd backend && go test -tags=integration ./integration -run TestCerberusIntegration -v
```

**What's tested:**

- ✅ All features enable without conflicts
- ✅ Correct handler pipeline order
- ✅ WAF doesn't interfere with rate limiting
- ✅ Security decisions enforced at correct layer
- ✅ Legitimate traffic passes through all layers
- ✅ Performance benchmarks (< 50ms overhead)

### UI/UX Testing

The Cerberus Dashboard has extensive UI testing coverage:

- Security card status display verification
- Loading overlay animations
- Error handling and toast notifications
- Mobile responsive layout testing (375px → 1920px)

**Test documentation:**

- [Integration Testing Plan](plans/cerberus_integration_testing_plan.md)
- [UI/UX Testing Plan](plans/cerberus_uiux_testing_plan.md)

### VS Code Tasks

Run tests directly from VS Code using the provided tasks:

- **Cerberus: Run Full Integration Script** — Full shell-based integration test
- **Cerberus: Run Full Integration Go Test** — Go test wrapper

---

## More Technical Details

Want the nitty-gritty? See [Cerberus Technical Docs](cerberus.md).
