---
title: What Can Charon Do?
description: Complete feature guide for Charon reverse proxy manager. Learn about SSL certificates, security, Docker integration, and more.
---

## What Can Charon Do?

Here's everything Charon can do for you, explained simply.

---

## 🌍 Multi-Language Support

Charon speaks your language! The interface is available in multiple languages.

### What Languages Are Supported?

- 🇬🇧 **English** - Default
- 🇪🇸 **Spanish** (Español)
- 🇫🇷 **French** (Français)
- 🇩🇪 **German** (Deutsch)
- 🇨🇳 **Chinese** (中文)

### How to Change Language

1. Go to **Settings** → **System**
2. Scroll to the **Language** section
3. Select your preferred language from the dropdown
4. Changes take effect immediately — no page reload needed!

### Want to Help Translate?

We welcome translation contributions! See our [Translation Contributing Guide](https://github.com/Wikid82/Charon/blob/main/CONTRIBUTING_TRANSLATIONS.md) to learn how you can help make Charon available in more languages.

---

## 🌐 Application URL Configuration

**What it does:** Configures the public URL used in user invitation emails and system-generated links.

**Why you care:** Without this, invite links will use the server's local address (like `http://localhost:8080`), which won't work for users on external networks. Configuring this ensures invitations work correctly.

**Where to find it:** System Settings → Application URL section

### Configuration

**URL Requirements:**

- Must start with `http://` or `https://`
- Should be the URL users use to access Charon
- Cannot include path components (e.g., `/admin`)
- Port numbers are allowed (e.g., `:8080`)

**Validation:**

1. Enter your URL in the input field
2. Click **"Validate"** to check the format
   - Displays normalized URL if valid
   - Shows error message if invalid
   - Warns if using `http://` instead of `https://` in production
3. Click **"Test"** to open the URL in a new browser tab
4. Click **"Save Changes"** to persist the configuration

**Examples:**

✅ **Valid URLs:**

- `https://charon.example.com`
- `https://proxy.mydomain.net`
- `https://charon.example.com:8443` (custom port)
- `http://192.168.1.100:8080` (for internal testing only)

❌ **Invalid URLs:**

- `charon.example.com` (missing protocol)
- `https://charon.example.com/admin` (path not allowed)
- `ftp://charon.example.com` (wrong protocol)
- `https://charon.example.com/` (trailing slash not allowed)

### User Invitation Preview

**What it does:** Preview how invite URLs will look before sending invitations.

**Where to find it:** Users page → "Preview Invite" button when creating a new user

**How it works:**

1. Enter a user's email address in the invitation form
2. Click **"Preview Invite"**
3. See the exact invite URL that will be sent
4. View warning if Application URL is not configured

**Preview includes:**

- Full invite URL with sample token
- Base URL being used
- Configuration status indicator
- Warning message if not configured

**Example preview:**

```
Invite URL Preview:
https://charon.example.com/accept-invite?token=SAMPLE_TOKEN_PREVIEW

Base URL: https://charon.example.com
Status: ✅ Configured
```

**Warning state:**

```
⚠️ Application URL not configured

Invite URL Preview:
http://localhost:8080/accept-invite?token=SAMPLE_TOKEN_PREVIEW

This link may not be accessible from external networks.
Configure the Application URL in System Settings.
```

### Multi-Language Support

The Application URL configuration is fully localized and available in all supported languages:

- English, Spanish, French, German, Chinese
- All validation messages are translated
- Error messages and warnings respect language settings

### Admin-Only Access

Application URL configuration is restricted to administrators:

- Only users with admin role can modify the setting
- Non-admin users cannot access the validation or test endpoints
- API endpoints return 403 Forbidden for non-admin attempts

### API Integration

See [API Documentation](api.md#application-url-endpoints) for programmatic access to:

- `POST /settings/validate-url` - Validate URL format
- `POST /users/preview-invite-url` - Preview invite URL for a user

---

## ⚙️ Optional Features

Charon includes optional features that can be toggled on or off based on your needs.
All features are enabled by default, giving you the full Charon experience from the start.

### What Are Optional Features?

**What it does:** Lets you enable or disable major features like security monitoring and uptime checks.

**Why you care:** If you don't need certain features, turning them off keeps your sidebar cleaner and saves system resources.

**Where to find it:** Go to **System Settings** → Scroll to **Optional Features**

### Available Optional Features

#### Cerberus Security Suite

- **What it is:** Complete security system including CrowdSec integration, country blocking,
  WAF protection, and access control
- **When enabled:** Cerberus/Dashboard entries appear in the sidebar, all protection features are active
- **When disabled:** Security menu is hidden, all protection stops, but configuration data is preserved
- **Default:** Enabled

#### Uptime Monitoring

- **What it is:** Background checks that monitor if your websites are responding
- **When enabled:** Uptime menu appears in sidebar, automatic checks run every minute
- **When disabled:** Uptime menu is hidden, background checks stop, but uptime history is preserved
- **Default:** Enabled

### What Happens When Disabled?

When you disable a feature:

- ✅ **Sidebar item is hidden** — Keeps your navigation clean
- ✅ **Background jobs stop** — Saves CPU and memory resources
- ✅ **API requests are blocked** — Feature-specific endpoints return appropriate errors
- ✅ **Configuration data is preserved** — Your settings remain intact if you re-enable the feature

**Important:** Disabling a feature does NOT delete your data.
All your security rules, uptime history, and configurations stay safe in the database.
You can re-enable features at any time without losing anything.

### How to Toggle Features

1. Go to **System Settings**
2. Scroll to the **Optional Features** section
3. Toggle the switch for the feature you want to enable/disable
4. Changes take effect immediately

**Note:** Both features default to enabled when you first install Charon. This gives you full functionality out of the box.

---

## \ud83d\udce8 Standard Proxy Headers

**What it does:** Automatically adds industry-standard HTTP headers to requests forwarded to your backend applications, providing them with information about the original client connection.

**Why you care:** Your backend applications need to know the real client IP address and original protocol (HTTP vs HTTPS) for proper logging, security decisions, and functionality. Without these headers, your apps only see Charon's IP address.

**What you do:** Enable the checkbox when creating/editing a proxy host, or use bulk apply to enable on multiple hosts at once.

### What Headers Are Added?

When enabled, Charon adds these four standard headers to every proxied request:

| Header | Purpose | Example Value |
|--------|---------|---------------|
| `X-Real-IP` | The actual client IP address (not Charon's IP) | `203.0.113.42` |
| `X-Forwarded-Proto` | Original protocol used by the client | `https` |
| `X-Forwarded-Host` | Original Host header from the client | `example.com` |
| `X-Forwarded-Port` | Original port the client connected to | `443` |
| `X-Forwarded-For` | Chain of proxy IPs (managed by Caddy) | `203.0.113.42, 10.0.0.1` |

**Note:** `X-Forwarded-For` is handled natively by Caddy's reverse proxy and is not explicitly set by Charon to prevent duplication.

### Why These Headers Matter

**Client IP Detection:**

- Security logs show the real attacker IP, not Charon's internal IP
- Rate limiting works correctly per-client instead of limiting all traffic
- GeoIP-based features work with the client's location
- Analytics tools track real user locations

**HTTPS Enforcement:**

- Backend apps know if the original connection was secure
- Redirect logic works correctly (e.g., "redirect to HTTPS")
- Session cookies can be marked `Secure` appropriately
- Mixed content warnings are prevented

**Virtual Host Routing:**

- Backend apps can route requests based on the original hostname
- Multi-tenant applications can identify the correct tenant
- URL generation produces correct absolute URLs

**Example Use Cases:**

```python
# Python/Flask: Get real client IP
from flask import request
client_ip = request.headers.get('X-Real-IP', request.remote_addr)
logger.info(f"Request from {client_ip}")

# Check if original connection was HTTPS
is_secure = request.headers.get('X-Forwarded-Proto') == 'https'
if not is_secure:
    return redirect(request.url.replace('http://', 'https://'))
```

```javascript
// Node.js/Express: Get real client IP
app.use((req, res, next) => {
  const clientIp = req.headers['x-real-ip'] || req.ip;
  console.log(`Request from ${clientIp}`);
  next();
});

// Trust proxy to correctly handle X-Forwarded-* headers
app.set('trust proxy', true);
```

```go
// Go: Get real client IP
clientIP := r.Header.Get("X-Real-IP")
if clientIP == "" {
    clientIP = r.RemoteAddr
}

// Check original protocol
isHTTPS := r.Header.Get("X-Forwarded-Proto") == "https"
```

### Default Behavior

- **New proxy hosts**: Standard headers are **enabled by default** (best practice)
- **Existing hosts**: Standard headers are **disabled by default** (backward compatible)
- **Migration**: Use the info banner or bulk apply to enable on existing hosts

### When to Enable

✅ **Enable if your backend application:**

- Needs accurate client IP addresses for security/logging
- Enforces HTTPS or redirects based on protocol
- Uses IP-based rate limiting or access control
- Serves multiple virtual hosts/tenants
- Generates absolute URLs or redirects

### When to Disable

❌ **Disable if your backend application:**

- Is a legacy app that doesn't understand proxy headers
- Has custom IP detection logic that conflicts with standard headers
- Explicitly doesn't trust X-Forwarded-* headers (security policy)
- Already receives these headers from another source

### Security Considerations

**Trusted Proxies:**
Charon configures Caddy with `trusted_proxies` to prevent IP spoofing. Headers are only trusted when coming from Charon itself, not from external clients.

**Header Injection:**
Caddy overwrites any existing X-Real-IP, X-Forwarded-Proto, X-Forwarded-Host, and X-Forwarded-Port headers sent by clients, preventing header injection attacks.

**Backend Configuration:**
Your backend application must be configured to trust proxy headers. Most frameworks have a "trust proxy" setting:

- Express.js: `app.set('trust proxy', true)`
- Django: `USE_X_FORWARDED_HOST = True`
- Flask: Use `ProxyFix` middleware
- Laravel: Set `trusted_proxies`

### How to Enable

**For a single host:**

1. Go to **Proxy Hosts** → Click **Edit** on the desired host
2. Scroll to the **Standard Proxy Headers** section
3. Check **"Enable Standard Proxy Headers"**
4. Click **Save**

**For multiple hosts (bulk apply):**

1. Go to **Proxy Hosts**
2. Select checkboxes for the hosts you want to update
3. Click **"Bulk Apply"** at the top
4. Toggle **"Standard Proxy Headers"** to **ON**
5. Check **"Apply to selected hosts"** for this setting
6. Click **"Apply Changes"**

**Bulk Apply also supports:**
- Applying or removing security header profiles across multiple hosts
- Enabling/disabling Forward Auth, WAF, or Access Lists in bulk
- Updating SSL certificate assignments for multiple hosts at once

**Info Banner:**
Existing hosts without standard headers show an info banner explaining the feature and providing a quick-enable button.

### Troubleshooting

**Problem:** Backend still sees Charon's IP address

- **Solution:** Ensure the feature is enabled in the proxy host settings
- **Check:** Verify your backend is configured to trust proxy headers

**Problem:** Application breaks after enabling headers

- **Solution:** Disable the feature and check your backend logs
- **Common cause:** Backend has strict header validation or conflicting logic

**Problem:** HTTPS redirects create loops

- **Solution:** Update your backend to check `X-Forwarded-Proto` instead of the connection protocol
- **Example:** Use `X-Forwarded-Proto == 'https'` for HTTPS detection

## \ud83d\udd10 SSL Certificates (The Green Lock)

**What it does:** Makes browsers show a green lock next to your website address.

**Why you care:** Without it, browsers scream "NOT SECURE!" and people won't trust your site.

**What you do:** Nothing. Charon gets free certificates from Let's Encrypt and renews them automatically.

### Choose Your SSL Provider

**What it does:** Lets you select which Certificate Authority (CA) issues your SSL certificates.

**Why you care:** Different providers have different rate limits and reliability. You also get a staging option for testing.

**Where to find it:** Go to System Settings → SSL Provider dropdown

**Available options:**

- **Auto (Recommended)** — The smart default. Tries Let's Encrypt first,
  automatically falls back to ZeroSSL if there are any issues.
  Best reliability with zero configuration.

- **Let's Encrypt (Prod)** — Uses only Let's Encrypt production servers.
  Choose this if you specifically need Let's Encrypt certificates and have no rate limit concerns.

- **Let's Encrypt (Staging)** — For testing purposes only.
  Issues certificates that browsers won't trust, but lets you test your configuration without hitting rate limits.
  See [Testing SSL Certificates](acme-staging.md) for details.

- **ZeroSSL** — Uses only ZeroSSL as your certificate provider.
  Choose this if you prefer ZeroSSL or are hitting Let's Encrypt rate limits.

**Recommended setting:** Leave it on "Auto (Recommended)" unless you have a specific reason to change it.
The auto mode gives you the best of both worlds—Let's Encrypt's speed with ZeroSSL as a backup.

**When to change it:**

- Testing configurations → Use "Let's Encrypt (Staging)"
- Hitting rate limits → Switch to "ZeroSSL"
- Specific CA requirement → Choose that specific provider
- Otherwise → Keep "Auto"

### Smart Certificate Cleanup

**What it does:** When you delete websites, Charon asks if you want to delete unused certificates too.

**Why you care:** Custom and staging certificates can pile up over time. This helps you keep things tidy.

**How it works:**

- Delete a website → Charon checks if its certificate is used elsewhere
- If the certificate is custom or staging (not Let's Encrypt) and orphaned → you get a prompt
- Choose to keep or delete the certificate
- Default is "keep" (safe choice)

**When it prompts:**

- ✅ Custom certificates you uploaded
- ✅ Staging certificates (for testing)
- ❌ Let's Encrypt certificates (managed automatically)

**What you do:**

- See the prompt after clicking Delete on a proxy host
- Check the box if you want to delete the orphaned certificate
- Leave unchecked to keep the certificate (in case you need it later)

---

## 📊 Dashboard

### Certificate Status Card

**What it does:** Displays a real-time overview of all your SSL certificates directly on the Dashboard.

**Why you care:** Know at a glance if any certificates need attention—expired, expiring soon, or still provisioning.

**What you see:**

- **Certificate Breakdown** — Visual count of certificates by status:
  - ✅ Valid certificates (healthy, not expiring soon)
  - ⚠️ Expiring certificates (within 30 days)
  - 🧪 Staging certificates (for testing)
  - ❌ Expired certificates (need immediate attention)
- **Pending Indicator** — Shows when certificates are being provisioned with a progress bar
- **Auto-Refresh** — Card automatically updates during certificate provisioning

**How it works:**

- The card polls for certificate status changes during active provisioning
- Progress bar shows visual feedback while Let's Encrypt/ZeroSSL issues certificates
- Once all certificates are ready, auto-refresh stops to save resources

**What you do:** Check the Dashboard after adding new hosts to monitor certificate provisioning.
If you see pending certificates, the system is working—just wait a moment for issuance to complete.

---

## \ud83d\udee1\ufe0f Security (Optional)

Charon includes **Cerberus**, a security system that blocks bad guys.
It's off by default—turn it on when you're ready.
The main page is the **Cerberus Dashboard** (sidebar: Cerberus → Dashboard).

### Block Bad IPs Automatically

**What it does:** CrowdSec watches for attackers and blocks them before they can do damage.
CrowdSec is now **GUI-controlled** through the Security dashboard—no environment variables needed.

**Why you care:** Someone tries to guess your password 100 times? Blocked automatically.

**What you do:** Toggle the CrowdSec switch in the Security dashboard. That's it! See [Security Guide](security.md).

⚠️ **Note:** Environment variables like `CHARON_SECURITY_CROWDSEC_MODE` are **deprecated**. Use the GUI toggle instead.

### Block Entire Countries

**What it does:** Stop all traffic from specific countries.

**Why you care:** If you only need access from the US, block everywhere else.

**What you do:** Create an access list, pick countries, assign it to your website.

### Block Bad Behavior

**What it does:** Detects common attacks like SQL injection or XSS.

**Why you care:** Protects your apps even if they have bugs.

**What you do:** Turn on "WAF" mode in security settings.

### Zero-Day Exploit Protection

**What it does:** The WAF (Web Application Firewall) can detect and block many zero-day exploits
before they reach your apps.

**Why you care:** Even if a brand-new vulnerability is discovered in your software, the WAF might
catch it by recognizing the attack pattern.

**How it works:**

- Attackers use predictable patterns (SQL syntax, JavaScript tags, command injection)
- The WAF inspects every request for these patterns
- If detected, the request is blocked or logged (depending on mode)

**What you do:**

1. Enable WAF in "Monitor" mode first (logs only, doesn't block)
2. Review logs for false positives
3. Switch to "Block" mode when ready

**Limitations:**

- Only protects web-based exploits (HTTP/HTTPS traffic)
- Does NOT protect against zero-days in Docker, Linux, or Charon itself
- Does NOT replace regular security updates

**Learn more:** [OWASP Core Rule Set](https://coreruleset.org/)

### CrowdSec Integration

**What it does:** Connects your Charon instance to the global CrowdSec network to share and receive threat intelligence.

**Why you care:** Protects your server from IPs that are attacking other people,
and lets you manage your security configuration easily.

**Test Coverage:** 100% frontend test coverage achieved with 162 comprehensive tests covering all CrowdSec features,
API clients, hooks, and utilities. See [QA Report](reports/qa_crowdsec_frontend_coverage_report.md) for details.

**Features:**

- **Hub Presets:** Browse, search, and install security configurations from the CrowdSec Hub.
  - **Search & Sort:** Easily find what you need with the new search bar and sorting options (by name, status, downloads).
  - **One-Click Install:** Download and apply collections, parsers, and scenarios directly from the UI.
  - **Smart Updates:** Checks for updates automatically using ETags to save bandwidth.
  - **Safe Apply:** Automatically backs up your configuration before applying changes.

- **Console Enrollment:** Connect your instance to the CrowdSec Console web interface.
  - **Visual Dashboard:** See your alerts and decisions in a beautiful cloud dashboard.
  - **Easy Setup:** Just click "Enroll" and paste your enrollment key.
    Charon automatically handles CAPI registration if needed.
  - **Configuration:** Uses the local configuration in `data/crowdsec/config.yaml` instead of system defaults.
  - **Secure:** The enrollment key is passed securely to the CrowdSec agent.

- **Live Decisions:** See exactly who is being blocked and why in real-time.

#### Automatic Startup & Persistence

**What it does:** CrowdSec automatically starts when the container restarts if you previously enabled it.

**Why you care:** Your security protection persists across container restarts and server reboots—no manual re-enabling needed.

**How it works:**

When you toggle CrowdSec ON:

1. **Settings table** stores your preference (`security.crowdsec.enabled = true`)
2. **SecurityConfig table** tracks the operational state (`crowdsec_mode = local`)
3. **Reconciliation function** checks both tables on container startup

When the container restarts:

1. **Reconciliation runs automatically** at startup
2. **Checks SecurityConfig table** for `crowdsec_mode = local`
3. **Falls back to Settings table** if SecurityConfig is missing
4. **Auto-starts CrowdSec** if either table indicates enabled
5. **Creates SecurityConfig** if missing (synced to Settings state)

**What you see in logs:**

```json
{"level":"info","msg":"CrowdSec reconciliation: starting based on SecurityConfig mode='local'","time":"..."}
```

Or if Settings table is used:

```json
{"level":"info","msg":"CrowdSec reconciliation: starting based on Settings table override","time":"..."}
```

Or if both are disabled:

```json
{"level":"info","msg":"CrowdSec reconciliation skipped: both SecurityConfig and Settings indicate disabled","time":"..."}
```

**Settings/SecurityConfig Synchronization:**

- **Enable via toggle:** Both tables update automatically
- **Disable via toggle:** Both tables update automatically
- **Container restart:** Reconciliation syncs SecurityConfig to Settings if missing
- **Database corruption:** Reconciliation recreates SecurityConfig from Settings

**When auto-start happens:**

✅ SecurityConfig has `crowdsec_mode = "local"`
✅ Settings table has `security.crowdsec.enabled = "true"`
✅ Either condition triggers auto-start (logical OR)

**When auto-start is skipped:**

❌ Both tables indicate disabled
❌ Fresh install with no Settings entry (defaults to disabled)

**Verification:**

Check CrowdSec status after container restart:

```bash
docker restart charon
sleep 15
docker exec charon cscli lapi status
```

Expected output when auto-start worked:

```
✓ You can successfully interact with Local API (LAPI)
```

### Rate Limiting

**What it does:** Limits how many requests any single IP can make in a given time window.

**Why you care:** Stops aggressive bots or abusive users from overwhelming your server.

**Where to find it:** Cerberus → Dashboard → Rate Limiting card, or click "Configure"
for full settings.

**Dashboard features:**

- **Status Badge:** The Rate Limiting card shows a clear "Active" or "Disabled" badge
  so you know at a glance if protection is enabled.
- **Quick View:** See the current configuration directly on the Security dashboard.

**Configuration page features:**

- **Active Summary Card:** When rate limiting is enabled, a green summary card at the
  top shows your current settings (requests/sec, burst limit, time window).
- **Real-time Updates:** Changes take effect immediately without server restart.

**Settings:**

- **Requests per Second:** Maximum sustained request rate (e.g., 10/sec)
- **Burst Limit:** Allow short bursts above the limit (e.g., 20 requests)
- **Time Window:** How long to track requests (e.g., 60 seconds)

---

## \ud83d\udc33 Docker Integration

### Auto-Discover Containers

**What it does:** Sees all your Docker containers and shows them in a list.

**Why you care:** Instead of typing IP addresses, just click your container and Charon fills everything in.

**What you do:** Make sure Charon can access `/var/run/docker.sock` (it's in the quick start).

### Remote Docker Servers

**What it does:** Manages containers on other computers.

**Why you care:** Run Charon on one server, manage containers on five others.

**What you do:** Add remote servers in the "Docker" section.

---

## \ud83d\udce5 Import Your Old Setup

**What it does:** Reads your existing Caddyfile and creates proxy hosts for you.

**Why you care:** Don't start from scratch if you already have working configs.

**What you do:** Click "Import," paste your Caddyfile, review the results, click "Import."

**[Detailed Import Guide](import-guide.md)**

### Import Success Modal

**What it does:** After importing a Caddyfile, displays a detailed summary modal showing exactly what happened.

**Why you care:** Know immediately how your import went—no guessing or digging through logs.

**What you see:**

- **Hosts Created** — New proxy hosts that were added to your configuration
- **Hosts Updated** — Existing hosts that were modified with new settings
- **Hosts Skipped** — Entries that weren't imported (duplicates or unsupported)
- **Certificate Guidance** — Instructions for SSL certificate provisioning
- **Quick Navigation** — Buttons to go directly to Dashboard or Proxy Hosts

**What you do:** Review the summary after each import.
If hosts were skipped, check the details to understand why.
Use the navigation buttons to proceed with your workflow.

---

## \u26a1 Zero Downtime Updates

**What it does:** Apply changes without stopping traffic.

**Why you care:** Your websites stay up even while you're making changes.

**What you do:** Nothing special—every change is zero-downtime by default.

---

## \ud83c\udfa8 Beautiful Loading Animations

When you make changes, Charon shows you themed animations so you know what's happening.

### The Gold Coin (Login)

When you log in, you see a spinning gold coin.
In Greek mythology, people paid Charon the ferryman with a coin to cross the river into the afterlife.
So logging in = paying for passage!

### The Blue Boat (Managing Websites)

When you create or update websites, you see Charon's boat sailing across the river.
He's literally "ferrying" your changes to the server.

### The Red Guardian (Security)

When you change security settings, you see Cerberus—the three-headed guard dog.
He protects the gates of the underworld, just like your security settings protect your apps.

**Why these exist:** Changes can take 1-10 seconds to apply.
The animations tell you what's happening so you don't think it's broken.

---

## \ud83d\udd0d Health Checks

**What it does:** Tests if your app is actually reachable before saving.

**Why you care:** Catches typos and mistakes before they break things.

**What you do:** Click the "Test" button when adding a website.

---

## \ud83d\udcca Uptime Monitoring

**What it does:** Automatically checks if your websites are responding every minute.

**Why you care:** Get visibility into uptime history and response times for all your proxy hosts.

**What you do:** View the "Uptime" page in the sidebar. Uptime checks run automatically in the background.

**Optional:** You can disable this feature in System Settings → Optional Features if you don't need it.
Your uptime history will be preserved.

### How Uptime Checks Work

Charon uses a **two-level check system** for efficient monitoring:

#### Level 1: Host-Level Pre-Check (TCP)

**What it does:** Quickly tests if the backend host/container is reachable via TCP connection.

**How it works:**
- Groups monitors by their backend IP address (e.g., `172.20.0.11`)
- Attempts TCP connection to the actual backend port (e.g., port `5690` for Wizarr)
- If successful → Proceeds to Level 2 checks
- If failed → Marks all monitors on that host as "down" (skips Level 2)

**Why it matters:** Avoids redundant HTTP checks when an entire backend container is stopped or unreachable.

**Technical detail:** Uses the `forward_port` from your proxy host configuration, not the public URL port.
This ensures correct connectivity checks for services on non-standard ports.

#### Level 2: Service-Level Check (HTTP/HTTPS)

**What it does:** Verifies the specific service is responding correctly via HTTP request.

**How it works:**
- Only runs if Level 1 passes
- Performs HTTP GET to the public URL (e.g., `https://wizarr.hatfieldhosted.com`)
- Accepts these as "up": 2xx (success), 3xx (redirect), 401 (auth required), 403 (forbidden)
- Measures response latency
- Records heartbeat with status

**Why it matters:** Detects service-specific issues like crashes, misconfigurations, or certificate problems.

**Example:** A service might be running (Level 1 passes) but return 500 errors (Level 2 catches this).

### When Things Go Wrong

**Scenario 1: Backend container stopped**
- Level 1: TCP connection fails ❌
- Level 2: Skipped
- Status: "down" with message "Host unreachable"

**Scenario 2: Service crashed but container running**
- Level 1: TCP connection succeeds ✅
- Level 2: HTTP request fails or returns 500 ❌
- Status: "down" with specific HTTP error

**Scenario 3: Everything working**
- Level 1: TCP connection succeeds ✅
- Level 2: HTTP request succeeds ✅
- Status: "up" with latency measurement

---

## \ud83d\udccb Logs & Monitoring

**What it does:** Shows you what's happening with your proxy.

**Why you care:** When something breaks, you can see exactly what went wrong.

**What you do:** Click "Logs" in the sidebar.

---

## 🗄️ Database Maintenance

**What it does:** Keeps your configuration database healthy and recoverable.

**Why you care:** Your proxy hosts, SSL certificates, and security settings are stored in a database. Keeping it healthy prevents data loss.

### Optimized SQLite Configuration

Charon uses SQLite with performance-optimized settings enabled automatically:

- **WAL Mode** — Allows reading while writing, faster performance
- **Busy Timeout** — Waits 5 seconds instead of failing immediately on lock
- **Smart Caching** — 64MB memory cache for faster queries

**What you do:** Nothing—these settings are applied automatically.

### Database Recovery

**What it does:** Detects and repairs database corruption.

**Why you care:** Power outages or disk failures can (rarely) corrupt your database. The recovery script can often fix it.

**When to use it:** If you see errors like "database disk image is malformed" or Charon won't start.

**How to run it:**

```bash
# Docker (stop Charon first)
docker stop charon
docker run --rm -v charon_data:/app/data charon:latest /app/scripts/db-recovery.sh
docker start charon

# Local development
./scripts/db-recovery.sh
```

The script will:

1. Create a backup of your current database
2. Check database integrity
3. Attempt automatic recovery if corruption is found
4. Keep the last 10 backups automatically

**Learn more:** See the [Database Maintenance Guide](database-maintenance.md) for detailed documentation.

---

## 🔴 Live Security Logs & Notifications

**What it does:** Stream security events in real-time and get notified about critical threats.

**Why you care:** See attacks as they happen, not hours later.
Configure alerts for WAF blocks, ACL denials, and suspicious activity.

### Live Log Viewer

**Real-time streaming:** Watch security events appear instantly in the Cerberus Dashboard.
Uses WebSocket technology to stream logs with zero delay.

**What you see:**

- WAF blocks (SQL injection attempts, XSS attacks, etc.)
- CrowdSec decisions (blocked IPs and why)
- Access control denials (geo-blocking, IP filtering)
- Rate limit hits
- All security-related events with full context

**Controls:**

- **Pause/Resume** — Stop the stream to examine specific entries
- **Clear** — Remove old entries to focus on new activity
- **Auto-scroll** — Automatically follows new entries (disable to scroll back)
- **Filter** — Client-side filtering by level, source, or text search

**Where to find it:** Cerberus → Dashboard → Live Activity section (bottom of page)

**Query parameters:** The WebSocket endpoint supports server-side filtering:

- `?level=error` — Only error-level logs
- `?source=waf` — Only WAF-related events
- `?source=cerberus` — All Cerberus security events

### WebSocket Connection Monitoring

**What it does:** Tracks and displays all active WebSocket connections in real-time, helping you troubleshoot connection issues and monitor system health.

**What you see:**

- Total active WebSocket connections
- Breakdown by connection type (General Logs vs Security Logs)
- Oldest connection age
- Detailed connection information:
  - Connection ID and type
  - Remote address (client IP)
  - Active filters being used
  - Connection duration

**Where to find it:** System Settings → WebSocket Connections card

**API Endpoints:** Programmatically access WebSocket statistics:

- `GET /api/v1/websocket/stats` — Aggregate connection statistics
- `GET /api/v1/websocket/connections` — Detailed list of all active connections

**Use cases:**

- **Troubleshooting:** Verify WebSocket connections are working when live logs aren't updating
- **Monitoring:** Track how many users are viewing live logs in real-time
- **Debugging:** Identify connection issues with proxy/load balancer configurations
- **Capacity Planning:** Understand WebSocket connection patterns and usage

**Auto-refresh:** The status card automatically updates every 5 seconds to show current connection state.

**See also:** [WebSocket Troubleshooting Guide](troubleshooting/websocket.md) for help resolving connection issues.

### Notification System

**What it does:** Sends alerts when security events match your configured criteria.

**Where to configure:** Cerberus Dashboard → "Notification Settings" button (top-right)

**Settings:**

- **Enable/Disable** — Master toggle for all notifications
- **Minimum Log Level** — Only notify for warnings and errors (ignore info/debug)
- **Event Types:**
  - WAF blocks (when the firewall stops an attack)
  - ACL denials (when access control rules block a request)
  - Rate limit hits (when traffic thresholds are exceeded)
- **Webhook URL** — Send alerts to Discord, Slack, or custom integrations
- **Email Recipients** — Comma-separated list of email addresses

**Example use cases:**

- Get a Slack message when your site is under attack
- Email yourself when ACL rules block legitimate traffic (false positive alert)
- Send all WAF blocks to your SIEM system for analysis

**What you do:**

1. Go to Cerberus Dashboard
2. Click "Notification Settings"
3. Enable notifications
4. Set minimum level to "warn" or "error"
5. Choose which event types to monitor
6. Add your webhook URL or email addresses
7. Save

**Technical details:**

- Notifications respect the minimum log level (e.g., only send errors)
- Webhook payloads include full event context (IP, request details, rule matched)
- Email delivery requires SMTP configuration (future feature)
- Webhook retries with exponential backoff on failure

---

## \ud83d\udcbe Backup & Restore

**What it does:** Saves a copy of your configuration before destructive changes.

**Why you care:** If you accidentally delete something, restore it with one click.

**What you do:** Backups happen automatically. Restore from the "Backups" page.

---

## \ud83c\udf10 WebSocket Support

**What it does:** Handles real-time connections for chat apps, live updates, etc.

**Why you care:** Apps like Discord bots, live dashboards, and chat servers need this to work.

**What you do:** Nothing—WebSockets work automatically.

## \ud83d\udcf1 Mobile-Friendly Interface

**What it does:** Works perfectly on phones and tablets.

**Why you care:** Fix problems from anywhere, even if you're not at your desk.

**What you do:** Just open the web interface on your phone.

---

## \ud83c\udf19 Dark Mode

**What it does:** Easy-on-the-eyes dark interface.

**Why you care:** Late-night troubleshooting doesn't burn your retinas.

**What you do:** It's always dark mode. (Light mode coming if people ask for it.)

---

## \ud83d\udd0c API for Automation

**What it does:** Control everything via code instead of the web interface.

**Why you care:** Automate repetitive tasks or integrate with other tools.

**What you do:** See the [API Documentation](api.md).

---

## 🧪 Testing & Quality Assurance

Charon maintains high test coverage across both backend and frontend to ensure reliability and stability.

**Overall Backend Coverage:** 85.4% with 38 new test cases recently added across 6 critical files including log_watcher.go (98.2%), crowdsec_handler.go (80%), and console_enroll.go (88.23%).

### Full Integration Test Suite

**What it does:** Validates that all Cerberus security layers (CrowdSec, WAF, ACL, Rate Limiting) work together without conflicts.

**Why you care:** Ensures your security stack is reliable and that enabling one feature doesn't break another.

**Test coverage includes:**

- All features enabling simultaneously without errors
- Correct security pipeline order verification (Decisions → CrowdSec → WAF → Rate Limit → ACL)
- WAF blocking doesn't consume rate limit quota
- Security decisions are enforced before rate limiting
- Legitimate traffic flows through all security layers
- Latency benchmarks to ensure minimal performance overhead

**How to run tests:**

```bash
# Run full integration script
bash scripts/cerberus_integration.sh

# Or via Go test
cd backend && go test -tags=integration ./integration -run TestCerberusIntegration -v
```

### UI/UX Test Coverage

**What it does:** Ensures the Cerberus Dashboard and security configuration pages work correctly.

**Coverage includes:**

- Security card status display (Active/Disabled indicators)
- Loading states and Cerberus-themed overlays
- Error handling and toast notifications
- Mobile responsive design testing

**Mobile responsive testing:**

- Dashboard cards stack on mobile (375px), 2-column on tablet (768px), 4-column on desktop
- Touch-friendly toggle switches (minimum 44px targets)
- Scrollable modals and overlays on small screens

### CrowdSec Frontend Test Coverage

**What it does:** Comprehensive frontend test suite for all CrowdSec features with 100% code coverage.

**Test files created:**

1. **API Client Tests** (`api/__tests__/`)
   - `presets.test.ts` - 26 tests for preset management API
   - `consoleEnrollment.test.ts` - 25 tests for Console enrollment API

2. **Data & Utilities Tests**
   - `data/__tests__/crowdsecPresets.test.ts` - 38 tests validating all 30 presets
   - `utils/__tests__/crowdsecExport.test.ts` - 48 tests for export functionality

3. **React Query Hooks Tests**
   - `hooks/__tests__/useConsoleEnrollment.test.tsx` - 25 tests for enrollment hooks

**Coverage metrics:**

- 162 total CrowdSec-specific tests
- 100% code coverage for all CrowdSec modules
- All tests passing with no flaky tests
- Pre-commit checks validated

**Learn more:** See the test plans in [docs/plans/](plans/) for detailed test cases and the [QA Coverage Report](reports/qa_crowdsec_frontend_coverage_report.md).

---

## 🎨 Modern UI/UX Design System

Charon features a modern, accessible design system built on Tailwind CSS v4 with:

### Design Tokens

- **Semantic Colors**: Brand, surface, border, and text color scales with light/dark mode support
- **Typography**: Consistent type scale with proper hierarchy
- **Spacing**: Standardized spacing rhythm across all components
- **Effects**: Unified shadows, border radius, and transitions

### Component Library

| Component | Description |
|-----------|-------------|
| **Badge** | Status indicators with success/warning/error/info variants |
| **Alert** | Dismissible callouts for notifications and warnings |
| **Dialog** | Accessible modal dialogs using Radix UI primitives |
| **DataTable** | Sortable, selectable tables with sticky headers |
| **StatsCard** | KPI/metric cards with trend indicators |
| **EmptyState** | Consistent empty state patterns with actions |
| **Select** | Accessible dropdown selects via Radix UI |
| **Tabs** | Navigation tabs with keyboard support |
| **Tooltip** | Contextual hints with proper positioning |
| **Checkbox** | Accessible checkboxes with indeterminate state |
| **Progress** | Progress indicators and loading bars |
| **Skeleton** | Loading placeholder animations |

### Layout Components

- **PageShell**: Consistent page wrapper with title, description, and action slots
- **Card**: Enhanced cards with hover states and variants
- **Button**: Multiple variants (primary, secondary, danger, ghost, outline, link) with loading states

### Accessibility

- WCAG 2.1 compliant components via Radix UI
- Proper focus management and keyboard navigation
- ARIA attributes and screen reader support
- Focus-visible states on all interactive elements

### Dark Mode

- Native dark mode with system preference detection
- Consistent color tokens across light and dark themes
- Smooth theme transitions without flash

---

## 🛡️ HTTP Security Headers

**What it does:** Automatically injects enterprise-level HTTP security headers into your proxy responses with zero manual configuration.

**Why you care:** Prevents common web vulnerabilities (XSS, clickjacking, MIME-sniffing) and improves your security posture without touching code.

**What you do:** Apply a preset (Basic/Strict/Paranoid) or create custom header profiles for specific needs.

### Why Security Headers Matter

Modern browsers support powerful security features through HTTP headers, but they're disabled by default.
Security headers tell browsers to enable protections like:

- **Preventing XSS attacks** — Content-Security-Policy blocks unauthorized scripts
- **Stopping clickjacking** — X-Frame-Options prevents embedding your site in malicious iframes
- **HTTPS enforcement** — HSTS ensures browsers always use secure connections
- **Blocking MIME-sniffing** — X-Content-Type-Options prevents browsers from guessing file types
- **Restricting browser features** — Permissions-Policy disables unused APIs (geolocation, camera, mic)

Without these headers, browsers operate in "permissive mode" that prioritizes compatibility over security.

### Quick Start with Presets

**What it does:** Three pre-configured security profiles that cover common use cases.

**Available presets:**

#### Basic (Production Safe)

**Best for:** Public websites, blogs, marketing pages, most production sites

**What it includes:**

- HSTS with 1-year max-age (forces HTTPS)
- X-Frame-Options: DENY (prevents clickjacking)
- X-Content-Type-Options: nosniff (blocks MIME-sniffing)
- Referrer-Policy: strict-origin-when-cross-origin (safe referrer handling)

**What it excludes:**

- Content-Security-Policy (CSP) — Disabled to avoid breaking sites
- Cross-Origin headers — Not needed for most sites

**Use when:** You want essential security without risk of breaking functionality.

#### Strict (High Security)

**Best for:** Web apps handling sensitive data (dashboards, admin panels, SaaS tools)

**What it includes:**

- All "Basic" headers
- Content-Security-Policy with safe defaults:
  - `default-src 'self'` — Only load resources from your domain
  - `script-src 'self'` — Only execute your own scripts
  - `style-src 'self' 'unsafe-inline'` — Your styles plus inline CSS (common need)
  - `img-src 'self' data: https:` — Your images plus data URIs and HTTPS images
- Permissions-Policy: camera=(), microphone=(), geolocation=() (blocks sensitive features)
- Referrer-Policy: no-referrer (maximum privacy)

**Use when:** You need strong security and can test/adjust CSP for your app.

#### Paranoid (Maximum Security)

**Best for:** High-risk applications, financial services, government sites, APIs

**What it includes:**

- All "Strict" headers
- Stricter CSP:
  - `default-src 'none'` — Block everything by default
  - `script-src 'self'` — Only your scripts
  - `style-src 'self'` — Only your stylesheets (no inline CSS)
  - `img-src 'self'` — Only your images
  - `connect-src 'self'` — Only your API endpoints
- Cross-Origin-Opener-Policy: same-origin (isolates window context)
- Cross-Origin-Resource-Policy: same-origin (blocks cross-origin embedding)
- Cross-Origin-Embedder-Policy: require-corp (enforces cross-origin isolation)
- No 'unsafe-inline' or 'unsafe-eval' — Maximum CSP strictness

**Use when:** Security is paramount and you can invest time in thorough testing.

**How to use presets:**

**Option 1: Assign directly to a host**

1. Go to **Proxy Hosts**, edit or create a host
2. Find the **"Security Headers"** dropdown
3. Select a preset (Basic, Strict, or Paranoid)
4. Save the host — Caddy applies the headers immediately

**Option 2: Bulk apply to multiple hosts**

1. Go to **Proxy Hosts**
2. Select checkboxes for the hosts you want to update
3. Click **"Bulk Apply"** at the top
4. In the **"Security Headers"** section, select a profile
5. Check **"Apply to selected hosts"** for this setting
6. Click **"Apply Changes"** — all selected hosts receive the profile

**Option 3: Clone and customize**

1. Go to **Security → HTTP Headers**
2. Find the preset you want (e.g., "Strict")
3. Click **"Clone"**
4. Customize the copied profile
5. Assign your custom profile to proxy hosts (individually or via bulk apply)

### Reusable Header Profiles

**What it does:** Create named profiles with multiple header configurations that can be assigned to proxy hosts via dropdown selection.

**Why you care:** Define security policies once, apply to any website. Update one profile to affect all hosts using it.

**Profile types:**

- **System Profiles (Read-Only)** — Pre-configured presets (Basic, Strict, Paranoid) you can view or clone but not edit
- **Custom Profiles** — Your own profiles that you can edit, delete, and customize freely

**Profile workflow:**

1. **Create Profile** — Go to Security → HTTP Headers, create a new profile or apply a preset
2. **Assign to Host** — Edit a proxy host, select the profile from the "Security Headers" dropdown
3. **Caddy Applies** — Charon automatically configures Caddy to inject the headers for that host
4. **View/Clone** — Browse presets, clone them to create customized versions

**Profile features:**

- **Name & Description** — Organize profiles by purpose ("Blog Security", "Admin Panel Headers")
- **Multi-select Headers** — Choose which headers to include
- **Header-specific Options** — Configure each header's behavior
- **Security Score** — Real-time score (0-100) shows strength of configuration
- **Validation** — Warns about unsafe combinations or missing critical headers

### Supported Headers

#### HSTS (HTTP Strict Transport Security)

**What it does:** Forces browsers to always use HTTPS for your domain.

**Options:**

- **Max-Age** — How long browsers remember the policy (seconds)
  - Recommended: 31536000 (1 year)
  - Minimum: 300 (5 minutes) for testing
- **Include Subdomains** — Apply HSTS to all subdomains
- **Preload** — Submit to browser HSTS preload list (permanent, irreversible)

**Warning:** Preload is a one-way decision. Once preloaded, removing HSTS requires contacting browsers manually.
Only enable preload if you're certain ALL subdomains will support HTTPS forever.

**Example:**

```
Strict-Transport-Security: max-age=31536000; includeSubDomains; preload
```

#### Content-Security-Policy (CSP)

**What it does:** Controls what resources browsers can load (scripts, styles, images, etc.).
The most powerful security header but also the easiest to misconfigure.

**Interactive CSP Builder:**

Charon includes a visual CSP builder that prevents common mistakes:

- **Directive Categories** — Organized by resource type (scripts, styles, images, fonts, etc.)
- **Source Suggestions** — Common values like `'self'`, `'none'`, `https:`, `data:`
- **Validation** — Warns about unsafe combinations (`'unsafe-inline'`, `'unsafe-eval'`)
- **Preview** — See the final CSP string in real-time

**Common directives:**

- `default-src` — Fallback for all resource types
- `script-src` — JavaScript sources (most important for XSS prevention)
- `style-src` — CSS sources
- `img-src` — Image sources
- `connect-src` — XHR/WebSocket/fetch destinations
- `font-src` — Web font sources
- `frame-src` — iframe sources

**Testing strategy:**

1. Start with `Content-Security-Policy-Report-Only` mode (logs violations, doesn't block)
2. Review violations in browser console
3. Adjust CSP to allow legitimate resources
4. Switch to enforcing mode when ready

**Best practices:**

- Avoid `'unsafe-inline'` and `'unsafe-eval'` — These disable XSS protection
- Use `'nonce-'` or `'hash-'` for inline scripts/styles when needed
- Start with `default-src 'self'` and add specific exceptions

#### X-Frame-Options

**What it does:** Prevents your site from being embedded in iframes (clickjacking protection).

**Options:**

- **DENY** — No one can embed your site (safest)
- **SAMEORIGIN** — Only your domain can embed your site

**When to use SAMEORIGIN:** If you embed your own pages in iframes (dashboards, admin tools).

**Example:**

```
X-Frame-Options: DENY
```

#### X-Content-Type-Options

**What it does:** Prevents browsers from MIME-sniffing responses away from declared content-type.

**Value:** Always `nosniff` (no configuration needed)

**Why it matters:** Without this, browsers might execute uploaded images as JavaScript if they contain script-like content.

**Example:**

```
X-Content-Type-Options: nosniff
```

#### Referrer-Policy

**What it does:** Controls how much referrer information browsers send with requests.

**Options:**

- `no-referrer` — Never send referrer (maximum privacy)
- `no-referrer-when-downgrade` — Only send on HTTPS → HTTPS
- `origin` — Only send origin (<https://example.com>), not full URL
- `origin-when-cross-origin` — Full URL for same-origin, origin for cross-origin
- `same-origin` — Only send referrer for same-origin requests
- `strict-origin` — Send origin unless downgrading HTTPS → HTTP
- `strict-origin-when-cross-origin` — Full URL for same-origin, origin for cross-origin (recommended)
- `unsafe-url` — Always send full URL (not recommended)

**Recommended:** `strict-origin-when-cross-origin` balances privacy and analytics needs.

**Example:**

```
Referrer-Policy: strict-origin-when-cross-origin
```

#### Permissions-Policy

**What it does:** Controls which browser features and APIs your site can use (formerly Feature-Policy).

**Interactive Builder:**

Charon provides a visual interface to configure permissions:

- **Common Features** — Camera, microphone, geolocation, payment, USB, etc.
- **Toggle Access** — Allow for your site, all origins, or block completely
- **Delegation** — Allow specific domains to use features

**Common policies:**

- `camera=()` — Block camera access completely
- `microphone=()` — Block microphone access
- `geolocation=(self)` — Allow geolocation only on your domain
- `payment=(self "https://secure-payment.com")` — Allow payment API for specific domains

**Best practice:** Block all features you don't use. This reduces attack surface and prevents third-party scripts from accessing sensitive APIs.

**Example:**

```
Permissions-Policy: camera=(), microphone=(), geolocation=(), payment=()
```

#### Cross-Origin-Opener-Policy (COOP)

**What it does:** Isolates your document's window context from cross-origin documents.

**Options:**

- `unsafe-none` — No isolation (default browser behavior)
- `same-origin-allow-popups` — Isolate except for popups you open
- `same-origin` — Full isolation (recommended for high-security)

**Use case:** Prevents cross-origin pages from accessing your window object (Spectre mitigation).

**Example:**

```
Cross-Origin-Opener-Policy: same-origin
```

#### Cross-Origin-Resource-Policy (CORP)

**What it does:** Prevents other origins from embedding your resources.

**Options:**

- `same-site` — Only same-site can embed
- `same-origin` — Only exact origin can embed (strictest)
- `cross-origin` — Anyone can embed (default)

**Use case:** Protect images, scripts, styles from being hotlinked or embedded by other sites.

**Example:**

```
Cross-Origin-Resource-Policy: same-origin
```

#### Cross-Origin-Embedder-Policy (COEP)

**What it does:** Requires all cross-origin resources to explicitly opt-in to being loaded.

**Options:**

- `unsafe-none` — No restrictions (default)
- `require-corp` — Cross-origin resources must have CORP header (strict)

**Use case:** Enables SharedArrayBuffer and high-precision timers (needed for WebAssembly, advanced web apps).

**Warning:** Can break third-party resources (CDNs, ads) that don't send CORP headers.

**Example:**

```
Cross-Origin-Embedder-Policy: require-corp
```

#### X-XSS-Protection

**What it does:** Legacy XSS filter for older browsers (mostly obsolete).

**Options:**

- `0` — Disable filter (recommended for CSP-protected sites)
- `1` — Enable filter
- `1; mode=block` — Enable filter and block rendering if XSS detected

**Modern approach:** Use Content-Security-Policy instead. This header is deprecated in modern browsers.

**Example:**

```
X-XSS-Protection: 0
```

#### Cache-Control

**What it does:** Controls caching behavior for security-sensitive pages.

**Security-relevant values:**

- `no-store` — Never cache (for sensitive data)
- `no-cache, no-store, must-revalidate` — Full cache prevention
- `private` — Only browser cache, not CDNs

**Use case:** Prevent sensitive data (user dashboards, financial info) from being cached.

**Example:**

```
Cache-Control: no-cache, no-store, must-revalidate, private
```

### Security Score Calculator

**What it does:** Analyzes your header configuration and assigns a 0-100 security score with actionable improvement suggestions.

**Scoring categories:**

| Header Category | Weight | Max Points |
|----------------|--------|------------|
| HSTS | Critical | 20 |
| Content-Security-Policy | Critical | 25 |
| X-Frame-Options | High | 15 |
| X-Content-Type-Options | Medium | 10 |
| Referrer-Policy | Medium | 10 |
| Permissions-Policy | Medium | 10 |
| Cross-Origin Policies | Low | 10 |

**Score interpretation:**

- **🔴 0-49 (Poor)** — Missing critical headers, vulnerable to common attacks
- **🟡 50-74 (Fair)** — Basic protection, but missing important headers
- **🟢 75-89 (Good)** — Strong security posture, minor improvements possible
- **🟢 90-100 (Excellent)** — Maximum security, best practices followed

**What you see:**

- **Overall Score** — Large, color-coded number (0-100)
- **Category Breakdown** — Points earned per header category
- **Improvement Suggestions** — Specific actions to increase score
- **Real-time Preview** — Score updates as you change configuration

**How to use it:**

1. Create or edit a security header profile
2. Review the score in the right sidebar
3. Click suggestion links to fix issues
4. Watch score improve in real-time

### User Workflows

#### Workflow 1: Quick Protection (Basic Preset)

**Goal:** Add essential security headers to a production site without breaking anything.

**Steps:**

1. Go to **Proxy Hosts**, edit your host
2. Scroll to **"Security Headers"** section
3. Select **"Basic (Production Safe)"** from the dropdown
4. Review the security score preview (Score: 65/100)
5. Save

**Result:** Essential headers applied immediately via Caddy, security score ~60-70, zero breakage risk.

#### Workflow 2: Custom Headers for SaaS Dashboard

**Goal:** Create strict CSP for a web app while allowing third-party analytics and fonts.

**Steps:**

1. Go to **Security → HTTP Headers** → Click **"Create Profile"**
2. Name it "Dashboard Security"
3. Enable these headers:
   - HSTS (1 year, include subdomains)
   - CSP (use interactive builder):
     - `default-src 'self'`
     - `script-src 'self' https://cdn.analytics.com`
     - `style-src 'self' 'unsafe-inline'` (for React inline styles)
     - `font-src 'self' https://fonts.googleapis.com`
     - `img-src 'self' data: https:`
     - `connect-src 'self' https://api.analytics.com`
   - X-Frame-Options: DENY
   - X-Content-Type-Options: nosniff
   - Referrer-Policy: strict-origin-when-cross-origin
   - Permissions-Policy: camera=(), microphone=(), geolocation=()
4. Review security score (target: 80+)
5. Save the profile
6. Go to **Proxy Hosts**, edit your dashboard host
7. Select "Dashboard Security" from the **"Security Headers"** dropdown
8. Save — test in browser console for CSP violations
9. Edit profile as needed based on violations

**Result:** Strong security with functional third-party integrations, score 80-85.

#### Workflow 3: Maximum Security for API

**Goal:** Apply paranoid security for a backend API that serves JSON only.

**SGo to **Proxy Hosts**, edit your API host
2. Select **"Paranoid (Maximum Security)"** from the **"Security Headers"** dropdown
3. Review the configuration preview:

- HSTS with preload
- Strict CSP (`default-src 'none'`)
- All cross-origin headers set to `same-origin`
- No unsafe directives

1. Save
2. Test API endpoints (should work—APIs don't need CSP for HTML)
3. Assign to API proxy host
4. Test API endpoints (should work—APIs don't need CSP for HTML)
5. Verify security score (90+)

**Result:** Maximum security, score 90-100, suitable for high-risk environments.

### API Endpoints

Charon exposes HTTP Security Headers via REST API for automation:

```
GET    /api/v1/security/headers/profiles           # List all profiles
POST   /api/v1/security/headers/profiles           # Create profile
GET    /api/v1/security/headers/profiles/:id       # Get profile details
PUT    /api/v1/security/headers/profiles/:id       # Update profile
DELETE /api/v1/security/headers/profiles/:id       # Delete profile
GET    /api/v1/security/headers/presets            # List available presets
POST   /api/v1/security/headers/presets/apply      # Apply preset to create profile
POST   /api/v1/security/headers/score              # Calculate security score
```

**Example: Create profile via API**

```bash
curl -X POST https://charon.example.com/api/v1/security/headers/profiles \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "API Headers",
    "description": "Security headers for backend API",
    "hsts_enabled": true,
    "hsts_max_age": 31536000,
    "hsts_include_subdomains": true,
    "csp_enabled": true,
    "csp_default_src": "'\''none'\''",
    "x_frame_options": "DENY",
    "x_content_type_options": true,
    "referrer_policy": "no-referrer"
  }'
```

**Example: Calculate security score**

```bash
curl -X POST https://charon.example.com/api/v1/security/headers/score \
  -H "Authorization: Bearer $TOKEN" \
  -H "Content-Type: application/json" \
  -d '{
    "hsts_enabled": true,
    "hsts_max_age": 31536000,
    "csp_enabled": true,
    "csp_default_src": "'\''self'\''"
  }'
```

### Implementation Details

**Backend components:**

- **Model:** [`backend/internal/models/security_header_profile.go`](https://github.com/Wikid82/Charon/blob/main/backend/internal/models/security_header_profile.go)
- **Handlers:** [`backend/internal/api/handlers/security_headers_handler.go`](https://github.com/Wikid82/Charon/blob/main/backend/internal/api/handlers/security_headers_handler.go)
- **Services:**
  - [`backend/internal/services/security_headers_service.go`](https://github.com/Wikid82/Charon/blob/main/backend/internal/services/security_headers_service.go)
  - [`backend/internal/services/security_score.go`](https://github.com/Wikid82/Charon/blob/main/backend/internal/services/security_score.go)
- **Caddy Integration:** [`backend/internal/caddy/config.go`](https://github.com/Wikid82/Charon/blob/main/backend/internal/caddy/config.go) (`buildSecurityHeadersHandler`)

**Frontend components:**

- **Profile List:** [`frontend/src/pages/SecurityHeaders.tsx`](https://github.com/Wikid82/Charon/blob/main/frontend/src/pages/SecurityHeaders.tsx)
- **Profile Form:** [`frontend/src/pages/SecurityHeaderProfileForm.tsx`](https://github.com/Wikid82/Charon/blob/main/frontend/src/pages/SecurityHeaderProfileForm.tsx)
- **API Client:** [`frontend/src/api/securityHeaders.ts`](https://github.com/Wikid82/Charon/blob/main/frontend/src/api/securityHeaders.ts)
- **React Query Hooks:** [`frontend/src/hooks/useSecurityHeaders.ts`](https://github.com/Wikid82/Charon/blob/main/frontend/src/hooks/useSecurityHeaders.ts)

**Caddy integration:**

Charon translates security header profiles into Caddy's `header` directive configuration:

```caddyfile
reverse_proxy {
    header_up Host {upstream_hostport}
    header_up X-Forwarded-Host {host}
    header_up X-Forwarded-Proto {scheme}

    # Security headers injected here
    header_down Strict-Transport-Security "max-age=31536000; includeSubDomains"
    header_down Content-Security-Policy "default-src 'self'; script-src 'self'"
    header_down X-Frame-Options "DENY"
    # ... etc
}
```

### Best Practices

**Start conservatively:**

- Begin with "Basic" preset for production sites
- Test "Strict" in staging environment first
- Only use "Paranoid" if you can invest time in thorough testing

**Content-Security-Policy:**

- Use `Content-Security-Policy-Report-Only` initially
- Monitor browser console for violations
- Avoid `'unsafe-inline'` and `'unsafe-eval'` when possible
- Consider using nonces or hashes for inline scripts/styles
- Test with your specific frontend framework (React, Vue, Angular)

**HSTS:**

- Start with short `max-age` (300 seconds) for testing
- Increase to 1 year (31536000) when confident
- Be extremely cautious with `preload`—it's permanent
- Ensure ALL subdomains support HTTPS before `includeSubDomains`

**Testing workflow:**

1. Apply headers in development/staging first
2. Open browser DevTools → Console → Check for violations
3. Use [Security Headers](https://securityheaders.com/) scanner
4. Test with real user workflows (login, forms, uploads)
5. Monitor for errors after deployment
6. Adjust CSP based on real-world violations

**Common CSP pitfalls:**

- Inline event handlers (`onclick`, `onerror`) blocked by default
- Third-party libraries (analytics, ads) need explicit allowance
- `data:` URIs for images/fonts need `data:` in `img-src`/`font-src`
- Webpack/Vite injected scripts need `'unsafe-inline'` or nonce support

**Rate of change:**

- Security headers can break functionality if misconfigured
- Roll out changes gradually (one host, then multiple, then all)
- Keep "Basic" profiles stable, experiment in custom profiles
- Document any special exceptions (why `'unsafe-inline'` is needed)

### Security Considerations

**CSP can break functionality:**

- Modern SPAs often use inline styles/scripts
- Third-party widgets (chat, analytics) need allowances
- Always test CSP changes thoroughly before production

**HSTS preload is permanent:**

- Once preloaded, you cannot easily undo it
- Affects all subdomains forever
- Only enable if 100% committed to HTTPS forever

**Cross-origin isolation:**

- COOP/COEP/CORP can break embedded content
- May break iframes, popups, and third-party resources
- Test with all integrations (SSO, OAuth, embedded videos)

**Default headers are secure but may need tuning:**

- "Basic" preset is safe for 95% of sites
- "Strict" preset may need CSP adjustments for your stack
- "Paranoid" preset requires significant testing

**Security vs. Compatibility:**

- Stricter headers improve security but increase breakage risk
- Balance depends on your threat model
- Enterprise apps → prefer security
- Public websites → prefer compatibility

**Header priority:**

1. HSTS (most important—enforces HTTPS)
2. X-Frame-Options (prevents clickjacking)
3. X-Content-Type-Options (prevents MIME confusion)
4. Content-Security-Policy (strongest but hardest to configure)
5. Other headers (defense-in-depth)

---

## Missing Something?

**[Request a feature](https://github.com/Wikid82/charon/discussions)** — Tell us what you need!
