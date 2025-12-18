# What Can Charon Do?

Here's everything Charon can do for you, explained simply.

---

## \u2699\ufe0f Optional Features

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

## Missing Something?

**[Request a feature](https://github.com/Wikid82/charon/discussions)** — Tell us what you need!
