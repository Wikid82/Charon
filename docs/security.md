---
title: Security Features
description: Comprehensive security documentation for Charon's Cerberus security suite including CrowdSec, WAF, and access control lists.
---

## Security Features

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
4. **Wait 5-15 seconds** for the Local API (LAPI) to start
5. Verify the status badge shows "Active" with a running PID

**What happens during startup:**

When you toggle CrowdSec ON, Charon:

1. Starts the CrowdSec process as the `charon` user (not root)
2. Loads configuration, parsers, and security scenarios
3. Initializes the Local API (LAPI) on port 8085
4. Polls LAPI health every 500ms for up to 60 seconds
5. Returns one of two states:
   - ✅ **LAPI Ready** — "CrowdSec started and LAPI is ready" — You can immediately proceed to console enrollment
   - ⚠️ **LAPI Initializing** — "CrowdSec started but LAPI is still initializing" — Wait 10 more seconds before enrolling

**Expected timing:**

- **Initial start:** 5-10 seconds
- **First start after container restart:** 10-15 seconds
- **Maximum wait:** 60 seconds (with automatic health checks)

**What you'll see in the UI:**

- **Loading overlay** with message "Starting CrowdSec... This may take up to 30 seconds"
- **Success toast** when LAPI is ready
- **Warning toast** if LAPI needs more time
- **Status badge** changes from "Offline" → "Starting" → "Active"

✅ That's it! CrowdSec starts automatically and begins blocking bad IPs once LAPI is ready.

**Persistence Across Restarts:**

Once enabled, CrowdSec **automatically starts** when the container restarts:

- ✅ Server reboot → CrowdSec auto-starts
- ✅ Docker restart → CrowdSec auto-starts
- ✅ Container update → CrowdSec auto-starts
- ❌ Manual toggle OFF → CrowdSec stays disabled until you re-enable

**How it works:**

- Your preference is stored in the database (Settings and SecurityConfig tables)
- Reconciliation function runs at container startup **before** HTTP server starts
- Protected by mutex to prevent race conditions
- Checks both tables to determine if CrowdSec should auto-start
- Validates binary and config paths before starting
- Verifies process is running after start (2-second health check)
- Logs show: "CrowdSec reconciliation: starting based on SecurityConfig mode='local'"

**Verification after restart:**

```bash
docker restart charon
sleep 15
docker exec charon cscli lapi status
```

Expected output:

```
✓ You can successfully interact with Local API (LAPI)
```

**Troubleshooting auto-start:**

See [CrowdSec Startup Fix Documentation](implementation/crowdsec_startup_fix_COMPLETE.md) for detailed troubleshooting including:

- Permission issues
- Missing SecurityConfig table
- Binary not found errors
- Process crashes on startup

⚠️ **DEPRECATED:** Environment variables like `SECURITY_CROWDSEC_MODE=local` are **no longer used**. CrowdSec is now GUI-controlled, just like WAF, ACL, and Rate Limiting. If you have these environment variables in your docker-compose.yml, remove them and use the GUI toggle instead. See [Migration Guide](migration-guide.md).

**What you'll see:** The Cerberus pages show blocked IPs and why they were blocked.

### Enroll with CrowdSec Console (optional)

**Prerequisites:**

✅ **CrowdSec must be enabled** via the GUI toggle (see above)
✅ **LAPI must be running** — Verify with: `docker exec charon cscli lapi status`
✅ **Feature flag enabled** — `crowdsec_console_enrollment` must be ON
✅ **Valid enrollment token** — Obtain from crowdsec.net

**Understanding LAPI Readiness:**

When you enable CrowdSec, the backend returns a response with a `lapi_ready` field:

```json
{
  "status": "started",
  "pid": 203,
  "lapi_ready": true
}
```

- **`lapi_ready: true`** — LAPI is fully initialized and ready for enrollment
- **`lapi_ready: false`** — CrowdSec is running, but LAPI is still starting up (wait 10 seconds)

**Checking LAPI Status Manually:**

```bash
# Quick status check
docker exec charon cscli lapi status

# Expected output when ready:
# ✓ You can successfully interact with Local API (LAPI)

# Health endpoint check
docker exec charon curl -s http://localhost:8085/health

# Expected response:
# {"status":"up"}
```

**Enrollment Steps:**

1. **Ensure CrowdSec is enabled** and **LAPI is running** (check prerequisites above)
2. **Verify LAPI readiness** — Check the success toast message:
   - ✅ "CrowdSec started and LAPI is ready" → Proceed immediately
   - ⚠️ "LAPI is still initializing" → Wait 10 more seconds
3. Navigate to **Cerberus → CrowdSec**
4. Enable the feature flag `crowdsec_console_enrollment` if not already enabled
5. Click **Enroll with CrowdSec Console**
6. Paste the enrollment key from crowdsec.net
7. Click **Submit**
8. **Automatic retry** — Charon checks LAPI availability (3 attempts, 2 seconds apart)
9. Wait for confirmation (this may take 30-60 seconds)
10. Verify your instance appears on crowdsec.net dashboard

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

2. **Check LAPI health endpoint:**

   ```bash
   docker exec charon curl -s http://localhost:8085/health
   ```

   Expected: `{"status":"up"}`

3. **If LAPI is not running:**
   - Go to Security dashboard
   - Toggle CrowdSec **OFF**, then **ON**
   - **Wait 15 seconds** (LAPI needs time to initialize)
   - Re-check LAPI status
   - Verify you see the success toast: "CrowdSec started and LAPI is ready"

4. **Re-submit enrollment token:**
   - Same token works (enrollment tokens are reusable)
   - Go to Cerberus → CrowdSec
   - Paste token and submit again
   - Charon automatically retries LAPI checks (3 attempts, 2s apart)

5. **Check logs:**

   ```bash
   docker logs charon | grep -i crowdsec
   ```

   Look for:
   - ✅ "CrowdSec Local API listening" — LAPI started
   - ✅ "enrollment successful" — Registration completed
   - ❌ "LAPI not available" — LAPI not ready (retry after waiting)
   - ❌ "enrollment failed" — Check enrollment token validity

6. **If enrollment keeps failing:**
   - Verify your server has internet access to `api.crowdsec.net`
   - Check firewall rules allow outbound HTTPS connections
   - Ensure enrollment token is valid (check crowdsec.net)
   - Try generating a new enrollment token

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

---

## Break Glass Protocol Architecture

Charon provides a **3-Tier Break Glass Protocol** for emergency lockout recovery. This system ensures you always have a way to regain access, even when security modules block legitimate administrative traffic.

### Overview of the 3-Tier System

| Tier | Method | Use When | Security Layer |
|------|--------|----------|----------------|
| **Tier 1** | Emergency Token (Digital Key) | Application accessible but security blocking | Layer 7 bypass middleware |
| **Tier 2** | Emergency Server (Sidecar Door) | Caddy/CrowdSec blocking main endpoint | Separate port with minimal security |
| **Tier 3** | Direct System Access (Physical Key) | Complete system failure | SSH/console access to host |

### When to Use Each Tier

**Tier 1: Emergency Token**

Use when you can reach the Charon application but security middleware (ACL, WAF, Rate Limiting) is blocking your requests. The emergency token bypasses all Cerberus security checks at the middleware layer.

**Example scenario:** You enabled ACL with a restrictive whitelist and your IP isn't included.

**Solution:**

```bash
curl -X POST https://charon.example.com/api/v1/emergency/security-reset \
  -H "X-Emergency-Token: your-64-char-hex-token"
```

**Tier 2: Emergency Server**

Use when the main application endpoint is blocked at the Caddy reverse proxy layer (CrowdSec bans, WAF rules) or you need a completely separate entry point.

**Example scenario:** CrowdSec banned your IP at the Caddy layer, and Tier 1 is unreachable.

**Solution:**

```bash
# Create SSH tunnel
ssh -L 2020:localhost:2020 admin@server

# Use emergency server
curl -X POST http://localhost:2020/emergency/security-reset \
  -H "X-Emergency-Token: your-token" \
  -u admin:password
```

**Tier 3: Direct System Access**

Use when all application-level recovery methods fail, or you need to perform system-level repairs (clear CrowdSec bans directly, edit database, restart services).

**Example scenario:** Complete lockout with no network access to Charon endpoints.

**Solution:** SSH to the host and use direct database access or CrowdSec CLI commands.

### Diagram: 3-Tier Architecture

```
┌─────────────────────────────────────────────────────────┐
│                    TIER 1: DIGITAL KEY                  │
│  Emergency Token → Emergency Bypass Middleware → PASS   │
│  ✓ Fast (no SSH required)                               │
│  ✓ Works when application is reachable                  │
│  ⚠️ Blocked if Caddy/CrowdSec blocks at proxy layer     │
└─────────────────────────────────────────────────────────┘
                           ↓ (If Tier 1 fails)
┌─────────────────────────────────────────────────────────┐
│                  TIER 2: SIDECAR DOOR                   │
│  SSH Tunnel → Emergency Server (Port 2020) → PASS       │
│  ✓ Separate network path (bypasses main proxy)          │
│  ✓ Minimal security (Basic Auth only)                   │
│  ⚠️ Requires SSH access and emergency server enabled    │
└─────────────────────────────────────────────────────────┘
                           ↓ (If Tier 2 fails)
┌─────────────────────────────────────────────────────────┐
│                  TIER 3: PHYSICAL KEY                   │
│  SSH → Direct Database Access / CrowdSec CLI → PASS     │
│  ✓ Always works (direct system access)                  │
│  ✓ Can fix any issue (database, config, processes)      │
│  ⚠️ Requires root/sudo access to host                   │
└─────────────────────────────────────────────────────────┘
```

### Security Considerations

**Tier 1 Security:**

- ✅ **Double authentication**: Emergency token + source IP verification (management CIDR)
- ✅ **Timing-safe comparison**: Prevents timing attacks on token validation
- ✅ **Rate limiting**: 5 attempts per minute per IP
- ✅ **Audit logging**: All emergency token usage is logged
- ⚠️ **Token in headers**: Use HTTPS only to protect token in transit
- ⚠️ **ClientIP spoofing**: Configure trusted proxies correctly

**Tier 2 Security:**

- ✅ **Network isolation**: Separate port, can bind to localhost only
- ✅ **Basic Auth**: Optional username/password authentication
- ✅ **SSH tunneling**: Force access through encrypted SSH connection
- ⚠️ **Public exposure risk**: Port 2020 should NEVER be publicly accessible
- ⚠️ **Basic Auth is weak**: Consider mTLS for production (future enhancement)

**Tier 3 Security:**

- ✅ **Physical access required**: Attackers need SSH credentials
- ✅ **Audit trail**: All SSH sessions and commands are logged
- ⚠️ **No application-level protection**: Direct database access bypasses all security
- ⚠️ **Root required**: Most Tier 3 operations require elevated privileges

---

## Emergency Token Management

### Generating Secure Tokens

Always use cryptographically secure random generators:

```bash
# Recommended: OpenSSL
openssl rand -hex 32

# Alternative: Python
python3 -c "import secrets; print(secrets.token_hex(32))"

# Alternative: /dev/urandom
head -c 32 /dev/urandom | xxd -p -c 64
```

**Token Requirements:**

- Minimum 32 bytes (produces 64-character hex string)
- Must be unique per deployment
- Never reuse tokens across environments
- Store in secrets manager, never commit to version control

### Token Storage Recommendations

**Priority 1: Secrets Manager**

- HashiCorp Vault
- AWS Secrets Manager
- Azure Key Vault
- Kubernetes Secrets (with encryption at rest)

**Priority 2: Password Manager**

- 1Password
- LastPass
- Bitwarden (self-hosted)
- KeePassXC

**Priority 3: Environment File**

- `.env` file (add to `.gitignore`)
- Environment variables (systemd, Docker secrets)

**❌ NEVER:**

- Hardcode in `docker-compose.yml` tracked by git
- Store in plain text files
- Share via email or unencrypted chat
- Include in screenshots or documentation

### Token Rotation Procedures

**Rotate every 90 days or immediately if:**

- Token was used during an emergency
- Token may have been exposed (logs, screenshots, source control)
- Team member with token access has left
- Security audit requires rotation

**Rotation Steps:**

1. Generate new token: `openssl rand -hex 32`
2. Update secrets manager with new token
3. Update `CHARON_EMERGENCY_TOKEN` in docker-compose.yml or .env
4. Restart Charon container: `docker-compose restart charon`
5. Verify new token works: Test emergency endpoint
6. Verify old token is revoked: Test should return 401 Unauthorized
7. Document rotation in change log

**See [Emergency Token Rotation Guide](runbooks/emergency-token-rotation.md) for detailed procedures.**

### Token Expiration Policy Recommendations

**For organizations with compliance requirements:**

| Environment | Rotation Frequency | Minimum Length | Additional Requirements |
|-------------|-------------------|----------------|------------------------|
| Development | 180 days | 32 bytes | Document in dev handbook |
| Staging | 90 days | 32 bytes | Separate from production |
| Production | 90 days | 32 bytes | Secrets manager, audit trail |
| High Security | 30 days | 64 bytes | mTLS, HSM storage, 2FA |

---

## Management Network Configuration

### What are Management CIDRs?

Management CIDRs (Classless Inter-Domain Routing) define IP address ranges that are allowed to use the emergency token for Tier 1 access. This provides defense-in-depth: even if an attacker obtains the emergency token, they can't use it unless they're coming from an authorized network.

### Default Values (RFC1918)

Charon defaults to private network ranges if `CHARON_MANAGEMENT_CIDRS` is not configured:

```bash
CHARON_MANAGEMENT_CIDRS=10.0.0.0/8,172.16.0.0/12,192.168.0.0/16,127.0.0.0/8
```

**What this means:**

- `10.0.0.0/8` — Private network (10.0.0.0 to 10.255.255.255)
- `172.16.0.0/12` — Private network (172.16.0.0 to 172.31.255.255)
- `192.168.0.0/16` — Private network (192.168.0.0 to 192.168.255.255)
- `127.0.0.0/8` — Localhost (127.0.0.1)

### How to Configure Management CIDRs

**Example 1: Office Network Only**

```yaml
environment:
  - CHARON_MANAGEMENT_CIDRS=192.168.1.0/24
```

**Example 2: Office + VPN**

```yaml
environment:
  - CHARON_MANAGEMENT_CIDRS=192.168.1.0/24,10.8.0.0/24
```

**Example 3: Multiple Offices**

```yaml
environment:
  - CHARON_MANAGEMENT_CIDRS=192.168.1.0/24,192.168.2.0/24,10.10.0.0/16
```

**Example 4: Single Admin IP (Most Restrictive)**

```yaml
environment:
  - CHARON_MANAGEMENT_CIDRS=203.0.113.42/32
```

### Security Implications

**Restrictive CIDRs (Recommended):**

- ✅ **Defense in depth**: Token + network location required
- ✅ **Limits attack surface**: Only trusted networks can attempt emergency access
- ✅ **Audit precision**: Know exactly where emergency access came from
- ⚠️ **Operational risk**: Admin locked out if not in allowed network

**Permissive CIDRs (Not Recommended):**

```yaml
# ❌ DO NOT USE IN PRODUCTION
- CHARON_MANAGEMENT_CIDRS=0.0.0.0/0,::/0
```

- ❌ **No geographic protection**: Token works from anywhere
- ❌ **Increased attack surface**: Attackers can attempt brute force globally
- ❌ **Compliance issues**: May violate security policies (ISO 27001, SOC 2)
- ✅ **Operational safety**: Admin can always use token (no lockout risk)

### Best Practices

1. **Start restrictive, expand if needed**: Begin with office/VPN networks only
2. **Include VPN subnet**: Ensure emergency access works when remote
3. **Document IP changes**: Update CIDRs when networks change
4. **Test after changes**: Verify emergency token works from expected locations
5. **Monitor audit logs**: Review where emergency access attempts come from

---

## Emergency Server Security

### Why Port 2020 Should NEVER Be Publicly Exposed

The emergency server is designed as a **failsafe access mechanism** with minimal security controls. Exposing it to the public internet creates a high-risk attack surface.

**Note:** Port 2020 is used for the emergency server to avoid conflict with Caddy's admin API on port 2019.

**Risks of public exposure:**

- ❌ **Weak authentication**: Basic Auth is vulnerable to brute force
- ❌ **No rate limiting at proxy layer**: Emergency server has minimal DoS protection
- ❌ **Credentials in HTTP headers**: Basic Auth sends credentials in every request
- ❌ **Bypass all security**: Emergency server has direct database access
- ❌ **Compliance violations**: Exposure may violate security policies

### How to Use SSH Tunnels

SSH tunneling provides encrypted, authenticated access to the emergency server without exposing it to the internet.

**Create SSH tunnel:**

```bash
# Basic tunnel (port 2020 on localhost → port 2020 on server)
ssh -L 2020:localhost:2020 admin@server.example.com

# Keep terminal open - tunnel stays active
# In new terminal, access emergency server:
curl http://localhost:2020/health
```

**Persistent tunnel with autossh:**

```bash
# Install autossh
sudo apt install autossh

# Create persistent tunnel (auto-reconnect on disconnect)
autossh -M 0 -f -N -L 2020:localhost:2020 admin@server.example.com

# Verify tunnel is active
ps aux | grep autossh

# Stop tunnel
pkill autossh
```

### VPN Configuration Recommendations

**Option 1: WireGuard (Recommended)**

```bash
# Server: Install WireGuard
sudo apt install wireguard

# Generate keys
wg genkey | tee privatekey | wg pubkey > publickey

# Configure tunnel
sudo nano /etc/wireguard/wg0.conf
```

**Option 2: OpenVPN**

```bash
# Server: Install OpenVPN
sudo apt install openvpn

# Use Easy-RSA for certificate generation
make-cadir ~/openvpn-ca
```

**Configure Charon to listen on VPN interface:**

```yaml
environment:
  - CHARON_EMERGENCY_BIND=10.8.0.1:2020  # VPN interface IP
  - CHARON_MANAGEMENT_CIDRS=10.8.0.0/24  # VPN subnet
```

### Basic Auth vs mTLS Trade-offs

**Basic Auth (Current Implementation)**

**Pros:**

- ✅ Simple to configure
- ✅ Works with curl and standard HTTP clients
- ✅ No certificate management required

**Cons:**

- ❌ Credentials sent in every request
- ❌ Vulnerable to brute force
- ❌ No protection against credential theft
- ❌ Requires HTTPS/SSH tunnel for security

**mTLS (Future Enhancement)**

**Pros:**

- ✅ Strong authentication (client certificate)
- ✅ Credentials not sent over wire
- ✅ Protection against brute force
- ✅ Certificate-based access control

**Cons:**

- ❌ Complex certificate management
- ❌ Requires client-side configuration
- ❌ Certificate rotation overhead
- ❌ Not yet implemented in Charon

**Recommendation:** Use Basic Auth with SSH tunneling until mTLS is implemented.

---

## Audit Logging

### What Events Are Logged During Emergency Access

Charon logs all emergency access attempts with detailed context:

**Logged Events:**

| Event | Log Level | Fields Captured |
|-------|-----------|-----------------|
| Emergency token attempt (success) | WARN | Timestamp, IP, user-agent, path, token_valid=true |
| Emergency token attempt (failure) | WARN | Timestamp, IP, user-agent, path, token_valid=false, reason |
| Emergency token rate limit hit | WARN | Timestamp, IP, user-agent, attempts=6+ |
| Security module disabled | INFO | Timestamp, IP, module_name, disabled_by=emergency_token |
| Emergency server access | INFO | Timestamp, IP, endpoint, basic_auth_user |

**Example Log Entries:**

```
[WARN] Emergency bypass active: IP=192.168.1.100, path=/api/v1/emergency/security-reset
[INFO] Emergency token validation: result=success, ip=192.168.1.100, timing=2ms
[INFO] Security module disabled: module=security.acl.enabled, reason=emergency_reset, ip=192.168.1.100
[WARN] Emergency token rate limit exceeded: ip=192.168.1.100, attempts=6, window=60s
```

### How to Review Audit Logs Post-Incident

**View container logs:**

```bash
# Recent emergency events
docker logs charon | grep -i emergency

# With timestamps
docker logs charon --timestamps | grep -i emergency

# Last 24 hours (requires log driver with time filtering)
docker logs charon --since 24h | grep -i emergency

# Export to file for analysis
docker logs charon > /tmp/charon-incident-$(date +%Y%m%d).log
```

**Query audit log API:**

```bash
# Get all audit logs
curl http://localhost:8080/api/v1/audit-logs | jq

# Filter for emergency events
curl http://localhost:8080/api/v1/audit-logs | jq '.[] | select(.action | contains("emergency"))'

# Get logs from specific time range
curl "http://localhost:8080/api/v1/audit-logs?start=2026-01-26T00:00:00Z&end=2026-01-26T23:59:59Z" | jq
```

**Analyze log patterns:**

```bash
# Count emergency token attempts by IP
docker logs charon | grep "emergency token" | awk '{print $5}' | sort | uniq -c

# Find failed attempts
docker logs charon | grep "emergency" | grep "fail"

# Timeline of events
docker logs charon --timestamps | grep "emergency" | sort
```

### Alerting Recommendations

**Critical Alerts (Immediate Response):**

- ✅ Emergency token successfully used
- ✅ Security modules disabled via emergency endpoint
- ✅ Emergency server accessed

**Warning Alerts (Review within 1 hour):**

- ⚠️ Failed emergency token attempts (3+ in 5 minutes)
- ⚠️ Emergency token rate limit exceeded
- ⚠️ Emergency token used from unexpected IP

**Info Alerts (Review daily):**

- ℹ️ Emergency token configuration changed
- ℹ️ Emergency server enabled/disabled

**Prometheus Alert Example:**

```yaml
- alert: EmergencyTokenUsed
  expr: increase(charon_emergency_token_success_total[5m]) > 0
  labels:
    severity: critical
  annotations:
    summary: "Emergency break glass token was used"
    description: "Someone used the emergency token at {{ $labels.source_ip }}. Review audit logs immediately."
```

**Webhook Notification Example (Discord):**

```json
{
  "embeds": [{
    "title": "🚨 CRITICAL: Emergency Token Used",
    "description": "The emergency break glass token was just used to disable Charon security.",
    "color": 15158332,
    "fields": [
      {"name": "Source IP", "value": "192.168.1.100", "inline": true},
      {"name": "Timestamp", "value": "2026-01-26 10:30:45 UTC", "inline": true},
      {"name": "Disabled Modules", "value": "ACL, WAF, CrowdSec, Rate Limiting", "inline": false}
    ],
    "footer": {"text": "Review audit logs: docker logs charon | grep emergency"}
  }]
}
```

---

## Additional Resources

- **[Complete Emergency Recovery Runbook](runbooks/emergency-lockout-recovery.md)** — Step-by-step procedures for all 3 tiers
- **[Emergency Token Rotation Guide](runbooks/emergency-token-rotation.md)** — Token rotation procedures
- **[Configuration Examples](configuration/emergency-setup.md)** — Docker Compose configurations and firewall rules
- **[Break Glass Protocol Design](plans/break_glass_protocol_redesign.md)** — Detailed architecture and design decisions

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

Charon supports rich notification formatting for multiple services using customizable JSON templates:

**Discord Rich Embed Example:**

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

**Slack Block Kit Example:**

```json
{
  "blocks": [
    {
      "type": "header",
      "text": {"type": "plain_text", "text": "🛡️ Security Alert"}
    },
    {
      "type": "section",
      "text": {
        "type": "mrkdwn",
        "text": "*WAF Block*\nSQL injection attempt detected and blocked"
      }
    },
    {
      "type": "section",
      "fields": [
        {"type": "mrkdwn", "text": "*IP:*\n203.0.113.42"},
        {"type": "mrkdwn", "text": "*Rule:*\n942100"}
      ]
    }
  ]
}
```

**Gotify JSON Payload Example:**

```json
{
  "title": "🛡️ Security Alert",
  "message": "**WAF Block**: SQL injection attempt blocked from 203.0.113.42",
  "priority": 8,
  "extras": {
    "client::display": {"contentType": "text/markdown"},
    "security": {
      "event_type": "waf_block",
      "ip": "203.0.113.42",
      "rule_id": "942100"
    }
  }
}
```

**Gotify Token Hygiene (Required):**

- Treat Gotify application tokens as secrets; never echo, log, or return token values.
- Never expose tokenized endpoint query strings (for example,
  `...?token=...`) in logs, diagnostics, examples, screenshots,
  tickets, or reports.
- Redact query parameters in all diagnostics/examples before display or storage.
- Use write-only token inputs and store tokens in environment variables or a secret manager.
- Validate Gotify connectivity over HTTPS only.
- Rotate tokens immediately on suspected exposure.

**Configuring Notification Templates:**

1. Navigate to **Settings → Notifications**
2. Add or edit a notification provider
3. Select service type: Discord, Slack, Gotify, or Generic
4. Choose template style:
   - **Minimal**: Simple text-based notifications
   - **Detailed**: Rich formatting with comprehensive event data
   - **Custom**: Define your own JSON structure
5. Use template variables for dynamic content:
   - `{{.Title}}` — Event title (e.g., "WAF Block")
   - `{{.Message}}` — Detailed event description
   - `{{.EventType}}` — Event classification (waf_block, uptime_down, ssl_renewal)
   - `{{.Severity}}` — Alert level (info, warning, error)
   - `{{.HostName}}` — Affected proxy host domain
   - `{{.Timestamp}}` — ISO 8601 formatted timestamp
6. Click **"Send Test Notification"** to preview output
7. Save the provider configuration

**For complete examples with all variables and service-specific features, see [Notification Guide](features/notifications.md).**

**Testing your webhook:**

1. Add your webhook URL in Notification Settings
2. Select events to monitor (WAF blocks, uptime changes, SSL renewals)
3. Choose or customize a JSON template
4. Save the settings
5. Click **"Send Test"** to verify the integration
6. Trigger a real event (e.g., attempt to access a blocked URL)
7. Confirm notification appears in your Discord/Slack/Gotify channel

**Troubleshooting webhooks:**

- No notifications? Verify webhook URL is correct and uses HTTPS
- Invalid template? Use **"Send Test"** to validate JSON structure
- Wrong format? Consult your platform's webhook API documentation
- Template variables not replaced? Check variable names match exactly (case-sensitive)
- Too many notifications? Adjust event filters or increase severity threshold to "error" only
- Notifications delayed? Check network connectivity and firewall rules
- Template rendering errors? View logs: `docker logs charon | grep "notification"`

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

## TLS Security

### TLS Version Enforcement

Charon (via Caddy) enforces a minimum TLS version of 1.2 by default. This prevents TLS downgrade attacks that attempt to force connections to use vulnerable TLS 1.0 or 1.1.

**What's Protected:**

- ✅ TLS 1.0/1.1 downgrade attacks
- ✅ BEAST, POODLE, and similar protocol-level attacks
- ✅ Weak cipher suite negotiation

**HSTS (HTTP Strict Transport Security):**

Charon sets HSTS headers with:

- `max-age=31536000` (1 year)
- `includeSubDomains`
- `preload` (for browser preload lists)

This ensures browsers always use HTTPS after the first visit.

---

## DNS Security

### Protecting Against DNS Hijacking

While Charon cannot directly control your DNS resolver, you can protect against DNS hijacking and cache poisoning by configuring your host to use encrypted DNS.

**Docker Host Configuration (systemd-resolved):**

```bash
# /etc/systemd/resolved.conf
[Resolve]
DNS=1.1.1.1#cloudflare-dns.com 1.0.0.1#cloudflare-dns.com
DNSOverTLS=yes
```

Then restart: `sudo systemctl restart systemd-resolved`

**Alternative DNS Providers with DoH/DoT:**

- Cloudflare: `1.1.1.1` / `1.0.0.1`
- Google: `8.8.8.8` / `8.8.4.4`
- Quad9: `9.9.9.9`

**Additional DNS Protections:**

1. **DNSSEC**: Enable at your domain registrar to prevent DNS spoofing
2. **CAA Records**: Restrict which Certificate Authorities can issue certificates for your domain

---

## Container Hardening

### Running Charon with Maximum Security

Charon supports a fully hardened container configuration with a read-only root filesystem. This section explains the correct configuration based on research of where Charon writes data at runtime.

#### Understanding Charon's Data Storage

Charon uses two types of storage:

1. **Persistent Data** (`/app/data` volume) - Data that must survive container restarts:
   - **Database**: `/app/data/charon.db` - SQLite database with WAL mode
   - **Backups**: `/app/data/backups/` - Daily automated backups (3 AM cron job)
   - **Caddy Certificates**: `/app/data/caddy/` - TLS certificates from Let's Encrypt, ZeroSSL, or custom CAs
   - **Import Directory**: `/app/data/imports/` - Uploaded Caddyfile configurations
   - **CrowdSec Data**: `/app/data/crowdsec/` - CrowdSec configuration, database, and hub cache
   - **GeoIP Database**: `/app/data/geoip/GeoLite2-Country.mmdb` - Pre-populated at build time (read-only at runtime)

2. **Ephemeral Data** (tmpfs mounts) - Temporary data that doesn't need persistence:
   - **Caddy Logs**: `/var/log/caddy/` - Access logs monitored by CrowdSec
   - **CrowdSec Logs**: `/var/log/crowdsec/` - Agent and LAPI logs
   - **Runtime Config**: `/config/` - Dynamically generated Caddy JSON configuration
   - **CrowdSec Runtime**: `/var/lib/crowdsec/` - CrowdSec agent runtime data
   - **Temporary Files**: `/tmp/` - Used by CrowdSec hub operations
   - **Runtime State**: `/run/` - PIDs and runtime state files

#### Complete Hardened Configuration

```yaml
services:
  charon:
    image: ghcr.io/wikid82/charon:latest
    container_name: charon
    restart: unless-stopped

    # Security: Read-only root filesystem
    read_only: true

    # Drop all capabilities except NET_BIND_SERVICE (for ports 80/443)
    cap_drop:
      - ALL
    cap_add:
      - NET_BIND_SERVICE

    # Prevent privilege escalation
    security_opt:
      - no-new-privileges:true

    ports:
      - "80:80"
      - "443:443"
      - "443:443/udp"
      - "8080:8080"

    environment:
      - CHARON_ENV=production
      - TZ=UTC
      - CHARON_HTTP_PORT=8080
      - CHARON_DB_PATH=/app/data/charon.db
      - CHARON_FRONTEND_DIR=/app/frontend/dist
      - CHARON_CADDY_ADMIN_API=http://localhost:2019
      - CHARON_CADDY_CONFIG_DIR=/app/data/caddy
      - CHARON_CADDY_BINARY=caddy
      - CHARON_IMPORT_CADDYFILE=/import/Caddyfile
      - CHARON_IMPORT_DIR=/app/data/imports
      - CHARON_CROWDSEC_CONFIG_DIR=/app/data/crowdsec

    extra_hosts:
      - "host.docker.internal:host-gateway"

    volumes:
      # Persistent data (database, certificates, backups, CrowdSec config)
      - charon_data:/app/data

      # Ephemeral tmpfs mounts for writable directories
      - type: tmpfs
        target: /tmp
        tmpfs:
          size: 100M
          mode: 1777  # Sticky bit for multi-user temp directory

      - type: tmpfs
        target: /var/log/caddy
        tmpfs:
          size: 100M
          mode: 0755

      - type: tmpfs
        target: /var/log/crowdsec
        tmpfs:
          size: 100M
          mode: 0755

      - type: tmpfs
        target: /config
        tmpfs:
          size: 10M
          mode: 0755

      - type: tmpfs
        target: /var/lib/crowdsec
        tmpfs:
          size: 50M
          mode: 0755

      - type: tmpfs
        target: /run
        tmpfs:
          size: 10M
          mode: 0755

      # Docker socket for container discovery (read-only)
      - /var/run/docker.sock:/var/run/docker.sock:ro

      # Optional: Import existing Caddyfile (read-only)
      # - ./my-existing-Caddyfile:/import/Caddyfile:ro

    healthcheck:
      test: ["CMD", "curl", "--no-verbose", "--tries=1", "--spider", "http://localhost:8080/api/v1/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 40s

volumes:
  charon_data:
    driver: local
```

#### Security Features Explained

**Read-Only Root Filesystem:**

- `read_only: true` prevents unauthorized file modifications
- Blocks malware from persisting on the container filesystem
- Requires explicit tmpfs mounts for directories that need write access

**Capability Dropping:**

- `cap_drop: ALL` removes all Linux capabilities
- `cap_add: NET_BIND_SERVICE` only allows binding to privileged ports 80/443
- Follows the principle of least privilege

**No Privilege Escalation:**

- `no-new-privileges:true` prevents processes from gaining additional privileges
- Protects against setuid binary exploits and capability escalation

**Tmpfs Mounts:**

- Ephemeral storage that exists only in memory
- Automatically cleared on container restart
- Prevents logs and temporary files from filling disk space
- Size limits prevent memory exhaustion attacks

#### What About the `caddy_data` Volume?

If you're migrating from older documentation, you may notice the `caddy_data:/data` volume has been removed. This volume was never used by Charon. Here's why:

- **Caddy in standalone mode** uses `/data` for certificates
- **Charon configures Caddy** to use `/app/data/caddy/` instead
- The `caddy_data` volume was redundant and has been removed

#### Validation Checklist

Before deploying this configuration, validate that all features work correctly:

- [ ] Charon starts successfully with `read_only: true`
- [ ] Database operations work (create/read/update/delete proxy hosts)
- [ ] Caddy can obtain and renew TLS certificates
- [ ] Backups are created successfully (check `/app/data/backups/`)
- [ ] CrowdSec can start and update hub items (if enabled)
- [ ] Log files are written to `/var/log/caddy/access.log`
- [ ] Container discovery works with Docker socket
- [ ] Import directory accepts uploaded Caddyfiles
- [ ] No "read-only filesystem" errors in logs

**Quick Validation Commands:**

```bash
# Check startup logs
docker logs charon

# Verify database is writable
docker exec charon ls -la /app/data/charon.db

# Verify tmpfs mounts are correct
docker inspect charon | grep -A 10 Tmpfs

# Verify read-only root filesystem
docker inspect charon | grep '"ReadonlyRootfs": true'

# Test certificate directory is writable
docker exec charon touch /app/data/caddy/test.txt && docker exec charon rm /app/data/caddy/test.txt

# Verify logs are being written
docker exec charon ls -la /var/log/caddy/

# Check filesystem permissions
docker exec charon ls -la /app/data
```

#### Troubleshooting

**"read-only filesystem" errors:**

- Verify all tmpfs mounts are configured correctly
- Check that `/app/data` is mounted as a volume (not tmpfs)
- Ensure tmpfs sizes are adequate for your log volume

**CrowdSec fails to start:**

- Verify `/var/lib/crowdsec` tmpfs mount exists
- Check `/app/data/crowdsec` volume is writable
- Ensure symlink `/etc/crowdsec -> /app/data/crowdsec/config` is preserved

**Certificates not persisting:**

- Verify `charon_data` volume is mounted at `/app/data`
- Check that `CHARON_CADDY_CONFIG_DIR=/app/data/caddy` is set
- Ensure `/app/data/caddy` directory exists in the volume

**Security vs Functionality Trade-off:**

If you encounter issues with the hardened configuration, you can gradually relax security settings:

1. **Start with** `read_only: true` + all tmpfs mounts (recommended)
2. **If issues occur**, temporarily remove `read_only: true` to isolate the problem
3. **Identify the directory** that needs write access
4. **Add a tmpfs mount** for that directory (if ephemeral) or bind mount (if persistent)
5. **Re-enable** `read_only: true` once all write locations are properly mounted

⚠️ **Warning:** Do not skip tmpfs mounts and just remove `read_only: true`. This defeats the purpose of container hardening

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
- ✅ Server-Side Request Forgery (SSRF) — defense-in-depth architecture (CWE-918 resolved, PR #450)
- ⚠️ CrowdSec — protects hours/days after first exploitation (crowd-sourced)

**SSRF Protection Details** (PR #450):

Charon implements four-layer SSRF protection to prevent attacks against internal services, cloud metadata endpoints, and private networks:

1. **Format Validation**: URL scheme and path validation
2. **Pre-Connection Validation**: DNS resolution and IP address validation against 13+ blocked CIDR ranges
3. **Connectivity Testing**: Controlled HEAD requests with strict timeouts
4. **Runtime Re-Validation**: Connection-time IP checks to prevent DNS rebinding (TOCTOU protection)

**Protected Against**:

- Private IP ranges (RFC 1918: 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16)
- Loopback addresses (127.0.0.0/8, ::1/128)
- Link-local addresses (169.254.0.0/16, fe80::/10)
- Cloud metadata endpoints (169.254.169.254/32)
- IPv6 private ranges (fc00::/7)

**Where Applied**:

- Security notification webhooks
- URL connectivity testing endpoint
- CrowdSec hub URL validation
- GitHub update URL validation

See [SSRF Complete Implementation](implementation/SSRF_COMPLETE.md) for technical details.

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

## COOP (Cross-Origin-Opener-Policy) Behavior

### Development Mode

When accessing Charon over HTTP on non-localhost IP addresses (e.g., `http://192.168.1.100:8080`), you may see this browser console warning:

```
Cross-Origin-Opener-Policy policy would block the window.closed call.
```

**This is expected behavior and safe to ignore in local development.**

### Why Does This Happen?

The COOP header is conditionally applied based on the environment:

- **Development (HTTP):** COOP header is **disabled** to allow convenient local testing
- **Production (HTTPS):** COOP header is **enabled** with `same-origin-allow-popups` to protect against Spectre-class attacks

The browser warning appears because:

1. Your development server is accessed via HTTP (not HTTPS)
2. The IP address is not `localhost` (e.g., accessing from another device on your network)
3. Browsers enforce stricter security checks for non-localhost HTTP connections

### Production HTTPS Requirements

**⚠️ All production deployments MUST use HTTPS.** Running Charon in production over HTTP disables critical security protections:

**Security Headers Disabled on HTTP:**

- ✅ HSTS (HTTP Strict Transport Security)
- ✅ COOP (Cross-Origin-Opener-Policy)
- ✅ Secure cookie attributes

**Why HTTPS is Required:**

1. **Spectre Attack Protection:** COOP isolates browsing contexts to prevent cross-origin memory leaks
2. **Secure Cookies:** Session cookies with `Secure` flag only work over HTTPS
3. **Mixed Content:** Modern browsers block HTTP content loaded from HTTPS pages
4. **Compliance:** PCI-DSS, HIPAA, and other regulations mandate encryption in transit

### Load Balancer Configuration

If Charon runs behind a load balancer or reverse proxy (Cloudflare, nginx, Traefik):

**Required Header Forwarding:**

```nginx
# nginx example
proxy_set_header X-Forwarded-Proto $scheme;
proxy_set_header X-Forwarded-Host $host;
proxy_set_header X-Real-IP $remote_addr;
```

**Why this matters:** Charon detects HTTPS mode via the `X-Forwarded-Proto` header. If your load balancer terminates TLS but doesn't forward this header, Charon thinks it's running in HTTP mode and disables security features.

**Verification:**

Check your browser's developer tools → Network → Response Headers:

```
Cross-Origin-Opener-Policy: same-origin-allow-popups
Strict-Transport-Security: max-age=31536000; includeSubDomains; preload
```

If these headers are missing on HTTPS, verify your load balancer configuration.

### Caddy TLS Termination

When using Charon's built-in Caddy reverse proxy:

- ✅ TLS termination happens at Caddy (port 443)
- ✅ Charon backend receives `X-Forwarded-Proto: https` automatically
- ✅ Security headers applied correctly
- ✅ No additional configuration needed

**Docker Network:** Caddy and Charon communicate internally over HTTP, but the `X-Forwarded-Proto` header ensures Charon knows the client connection was HTTPS.

---

## Autocomplete Security

### Why Autocomplete is Enabled

Charon enables the `autocomplete` attribute on password and authentication fields. This is a **security best practice** recommended by OWASP and NIST.

**Benefits:**

1. **Stronger Passwords:** Password managers generate cryptographically secure passwords (20+ characters, high entropy)
2. **Unique Passwords:** Users are more likely to use unique passwords per-site when managers handle storage
3. **Reduced Phishing:** Password managers verify domain names before autofilling, protecting against phishing sites
4. **Better UX:** Improves accessibility and reduces password reuse

### OWASP Recommendations

From [OWASP Authentication Cheat Sheet](https://cheatsheetseries.owasp.org/cheatsheets/Authentication_Cheat_Sheet.html):

> "Do not disable the browser autocomplete on credential inputs. Modern password managers and browsers have secure implementations that rely on autocomplete attributes."

### NIST Guidelines

From [NIST SP 800-63B](https://pages.nist.gov/800-63-3/sp800-63b.html):

> "Verifiers SHOULD permit the use of paste functionality and password managers."

### Implementation in Charon

```html
<!-- Login form -->
<input
  type="text"
  name="username"
  autocomplete="username"
  required
/>
<input
  type="password"
  name="password"
  autocomplete="current-password"
  required
/>
```

**Autocomplete values used:**

- `username` — For login username/email fields
- `current-password` — For password login fields
- `new-password` — For password creation/change fields (future implementation)

### Compliance Considerations

**For most organizations:** Autocomplete is secure and recommended.

**For highly regulated industries (PCI-DSS Level 1, HIPAA, government):** Some compliance frameworks may require disabling autocomplete. If your organization has specific policies against password managers, you can:

1. Enforce company-wide password managers (preferred)
2. Disable browser autocomplete via group policy (not recommended)
3. Use hardware security keys (WebAuthn, FIDO2) as primary authentication

**Charon's position:** We follow modern security best practices. Disabling autocomplete reduces security for 99% of users to accommodate legacy compliance interpretations.

---

## Testing & Validation

### Test Coverage Metrics (PR #450)

Charon maintains comprehensive test coverage to ensure security features work correctly:

**Backend Coverage**: **86.2%** (exceeds 85% threshold)

- Security handlers: 85.6%
- Security middleware: 99.1%
- URL validation utilities: 91.8%
- SSRF protection: 90.2%
- IP helpers: 100%

**Frontend Coverage**: **87.27%** (exceeds 85% threshold)

- Security API: 92.19%
- Security hooks: 96.56%
- Security pages: 85.61%
- UI components: 97.35%

**Security-Specific Test Patterns**:

- ✅ SSRF protection for webhook URLs (HTTPS enforcement, private IP blocking)
- ✅ DNS resolution validation with timeout handling
- ✅ IPv4/IPv6 private address detection (13+ CIDR ranges)
- ✅ Cloud metadata endpoint blocking (169.254.169.254)
- ✅ DNS rebinding/TOCTOU attack prevention
- ✅ URL parser differential attack protection

See [PR #450 Implementation Summary](implementation/PR450_TEST_COVERAGE_COMPLETE.md) for detailed test metrics.

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
