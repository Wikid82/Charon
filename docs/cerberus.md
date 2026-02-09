---
title: Cerberus Technical Documentation
description: Technical deep-dive into Charon's Cerberus security suite. Architecture, configuration, and API reference for developers.
---

## Cerberus Technical Documentation

This document is for developers and advanced users who want to understand how Cerberus works under the hood.

**Looking for the user guide?** See [Security Features](security.md) instead.

---

## What Is Cerberus?

Cerberus is the optional security suite built into Charon. It includes:

- **WAF (Web Application Firewall)** — Inspects requests for malicious payloads
- **CrowdSec** — Blocks IPs based on behavior and reputation
- **Access Lists** — Static allow/deny rules (IP, CIDR, geo)
- **Rate Limiting** — Volume-based abuse prevention (placeholder)

All components are disabled by default and can be enabled independently.

---

## Architecture

### Request Flow

When a request hits Charon:

1. **Check if Cerberus is enabled** (global setting + dynamic database flag)
2. **WAF evaluation** (if `waf_mode != disabled`)
   - Increment `charon_waf_requests_total` metric
   - Check payload against loaded rulesets
   - If suspicious:
     - `block` mode: Return 403 + increment `charon_waf_blocked_total`
     - `monitor` mode: Log + increment `charon_waf_monitored_total`
3. **ACL evaluation** (if enabled)
   - Test client IP against active access lists
   - First denial = 403 response
4. **CrowdSec check** (placeholder for future)
5. **Rate limit check** (placeholder for future)
6. **Pass to downstream handler** (if not blocked)

### Middleware Integration

Cerberus runs as Gin middleware on all `/api/v1` routes:

```go
r.Use(cerberusMiddleware.RequestLogger())
```

This means it protects the management API but does not directly inspect traffic to proxied websites (that happens in Caddy).

---

## Threat Model & Protection Coverage

### What Cerberus Protects

| Threat Category | CrowdSec | ACL | WAF | Rate Limit |
|-----------------|----------|-----|-----|------------|
| Known attackers (IP reputation) | ✅ | ❌ | ❌ | ❌ |
| Geo-based attacks | ❌ | ✅ | ❌ | ❌ |
| SQL Injection (SQLi) | ❌ | ❌ | ✅ | ❌ |
| Cross-Site Scripting (XSS) | ❌ | ❌ | ✅ | ❌ |
| Remote Code Execution (RCE) | ❌ | ❌ | ✅ | ❌ |
| **Zero-Day Web Exploits** | ⚠️ | ❌ | ✅ | ❌ |
| DDoS / Volume attacks | ❌ | ❌ | ❌ | ✅ |
| Brute-force login attempts | ✅ | ❌ | ❌ | ✅ |
| Credential stuffing | ✅ | ❌ | ❌ | ✅ |

**Legend:**

- ✅ Full protection
- ⚠️ Partial protection (time-delayed)
- ❌ Not designed for this threat

## Zero-Day Exploit Protection (WAF)

The WAF provides **pattern-based detection** for zero-day exploits:

**How It Works:**

1. Attacker discovers new vulnerability (e.g., SQLi in your login form)
2. Attacker crafts exploit: `' OR 1=1--`
3. WAF inspects request → matches SQL injection pattern → **BLOCKED**
4. Your application never sees the malicious input

**Limitations:**

- Only protects HTTP/HTTPS traffic
- Cannot detect completely novel attack patterns (rare)
- Does not protect against logic bugs in application code

**Effectiveness:**

- **~90% of zero-day web exploits** use known patterns (SQLi, XSS, RCE)
- **~10% are truly novel** and may bypass WAF until rules are updated

## Request Processing Pipeline

```
1. [CrowdSec]      Check IP reputation → Block if known attacker
2. [ACL]           Check IP/Geo rules → Block if not allowed
3. [WAF]           Inspect request payload → Block if malicious pattern
4. [Rate Limit]    Count requests → Block if too many
5. [Proxy]         Forward to upstream service
```

## Configuration Model

### Database Schema

**SecurityConfig** table:

```go
type SecurityConfig struct {
    ID                   uint   `gorm:"primaryKey"`
    Name                 string `json:"name"`
    Enabled              bool   `json:"enabled"`
    AdminWhitelist       string `json:"admin_whitelist"`        // CSV of IPs/CIDRs
    CrowdsecMode         string `json:"crowdsec_mode"`          // disabled, local, external
    CrowdsecAPIURL       string `json:"crowdsec_api_url"`
    CrowdsecAPIKey       string `json:"crowdsec_api_key"`
    WafMode              string `json:"waf_mode"`               // disabled, monitor, block
    WafRulesSource       string `json:"waf_rules_source"`       // Ruleset identifier
    WafLearning          bool   `json:"waf_learning"`
    RateLimitEnable      bool   `json:"rate_limit_enable"`
    RateLimitBurst       int    `json:"rate_limit_burst"`
    RateLimitRequests    int    `json:"rate_limit_requests"`
    RateLimitWindowSec   int    `json:"rate_limit_window_sec"`
}
```

### Environment Variables (Fallbacks)

If no database config exists, Charon reads from environment:

- `CERBERUS_SECURITY_WAF_MODE` — `disabled` | `monitor` | `block`
- 🚨 **DEPRECATED:** `CERBERUS_SECURITY_CROWDSEC_MODE` — Use GUI toggle instead (see below)
- 🚨 **DEPRECATED:** `CERBERUS_SECURITY_CROWDSEC_API_URL` — External mode is no longer supported
- 🚨 **DEPRECATED:** `CERBERUS_SECURITY_CROWDSEC_API_KEY` — External mode is no longer supported
- `CERBERUS_SECURITY_ACL_ENABLED` — `true` | `false`
- `CERBERUS_SECURITY_RATELIMIT_ENABLED` — `true` | `false`

⚠️ **IMPORTANT:** The `CHARON_SECURITY_CROWDSEC_MODE` (and legacy `CERBERUS_SECURITY_CROWDSEC_MODE`, `CPM_SECURITY_CROWDSEC_MODE`) environment variables are **DEPRECATED** as of version 2.0. CrowdSec is now **GUI-controlled** through the Security dashboard, just like WAF, ACL, and Rate Limiting.

**Why the change?**

- CrowdSec now works like all other security features (GUI-based)
- No need to restart containers to enable/disable CrowdSec
- Better integration with Charon's security orchestration
- The import config feature replaced the need for external mode

**Migration:** If you have `CHARON_SECURITY_CROWDSEC_MODE=local` in your docker-compose.yml, remove it and use the GUI toggle instead. See [Migration Guide](migration-guide.md) for step-by-step instructions.

---

## WAF (Web Application Firewall)

### Current Implementation

**Status:** Prototype with placeholder detection

The current WAF checks for `<script>` tags as a proof-of-concept. Full OWASP CRS integration is planned.

```go
func (w *WAF) EvaluateRequest(r *http.Request) (Decision, error) {
    if strings.Contains(r.URL.Query().Get("q"), "<script>") {
        return Decision{Action: "block", Reason: "XSS detected"}, nil
    }
    return Decision{Action: "allow"}, nil
}
```

### Future: Coraza Integration

Planned integration with [Coraza WAF](https://coraza.io/) and OWASP Core Rule Set:

```go
waf, err := coraza.NewWAF(coraza.NewWAFConfig().
    WithDirectives(loadedRuleContent))
```

This will provide production-grade detection of:

- SQL injection
- Cross-site scripting (XSS)
- Remote code execution
- File inclusion attacks
- And more

### Rulesets

**SecurityRuleSet** table stores rule definitions:

```go
type SecurityRuleSet struct {
    ID         uint   `gorm:"primaryKey"`
    Name       string `json:"name"`
    SourceURL  string `json:"source_url"`  // Optional URL for rule updates
    Mode       string `json:"mode"`        // owasp, custom
    Content    string `json:"content"`     // Raw rule text
}
```

Manage via `/api/v1/security/rulesets`.

### Prometheus Metrics

```
charon_waf_requests_total{mode="block|monitor"} — Total requests evaluated
charon_waf_blocked_total{mode="block"} — Requests blocked
charon_waf_monitored_total{mode="monitor"} — Requests logged but not blocked
```

Scrape from `/metrics` endpoint (no auth required).

### Structured Logging

WAF decisions emit JSON-like structured logs:

```json
{
  "source": "waf",
  "decision": "block",
  "mode": "block",
  "path": "/api/v1/proxy-hosts",
  "query": "name=<script>alert(1)</script>",
  "ip": "203.0.113.50"
}
```

Use these for dashboard creation and alerting.

---

## Access Control Lists (ACLs)

### How They Work

Each `AccessList` defines:

- **Type:** `whitelist` | `blacklist` | `geo_whitelist` | `geo_blacklist` | `local_only`
- **IPs:** Comma-separated IPs or CIDR blocks
- **Countries:** Comma-separated ISO country codes (US, GB, FR, etc.)

**Evaluation logic:**

- **Whitelist:** If IP matches list → allow; else → deny
- **Blacklist:** If IP matches list → deny; else → allow
- **Geo Whitelist:** If country matches → allow; else → deny
- **Geo Blacklist:** If country matches → deny; else → allow
- **Local Only:** If RFC1918 private IP → allow; else → deny

Multiple ACLs can be assigned to a proxy host. The first denial wins.

### GeoIP Database

Uses MaxMind GeoLite2-Country database:

- Path configured via `CHARON_GEOIP_DB_PATH`
- Default: `/app/data/GeoLite2-Country.mmdb` (Docker)
- Update monthly from MaxMind for accuracy

---

## CrowdSec Integration

### GUI-Based Control (Current Architecture)

CrowdSec is now **GUI-controlled**, matching the pattern used by WAF, ACL, and Rate Limiting. The environment variable control (`CHARON_SECURITY_CROWDSEC_MODE`) is **deprecated** and will be removed in a future version.

### LAPI Initialization and Health Checks

**Technical Implementation:**

When you toggle CrowdSec ON via the GUI, the backend performs the following:

1. **Start CrowdSec Process** (`/api/v1/admin/crowdsec/start`)

   ```go
   pid, err := h.Executor.Start(ctx, h.BinPath, h.DataDir)
   ```

2. **Poll LAPI Health** (automatic, server-side)
   - **Polling interval:** 500ms
   - **Maximum wait:** 30 seconds
   - **Health check command:** `cscli lapi status`
   - **Expected response:** Exit code 0 (success)

3. **Return Status with `lapi_ready` Flag**

   ```json
   {
     "status": "started",
     "pid": 203,
     "lapi_ready": true
   }
   ```

**Response Fields:**

- **`status`** — "started" (process successfully initiated) or "error"
- **`pid`** — Process ID of running CrowdSec instance
- **`lapi_ready`** — Boolean indicating if LAPI health check passed
  - `true` — LAPI is fully initialized and accepting requests
  - `false` — CrowdSec is running, but LAPI still initializing (may take 5-10 more seconds)

**Backend Implementation** (`internal/handlers/crowdsec_handler.go:185-230`):

```go
func (h *CrowdsecHandler) Start(c *gin.Context) {
    // Start the process
    pid, err := h.Executor.Start(ctx, h.BinPath, h.DataDir)
    if err != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
        return
    }

    // Wait for LAPI to be ready (with timeout)
    lapiReady := false
    maxWait := 30 * time.Second
    pollInterval := 500 * time.Millisecond
    deadline := time.Now().Add(maxWait)

    for time.Now().Before(deadline) {
        checkCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
        defer cancel()

        _, err := h.CmdExec.Execute(checkCtx, "cscli", []string{"lapi", "status"})
        if err == nil {
            lapiReady = true
            break
        }
        time.Sleep(pollInterval)
    }

    // Return status
    c.JSON(http.StatusOK, gin.H{
        "status":     "started",
        "pid":        pid,
        "lapi_ready": lapiReady,
    })
}
```

**Key Technical Details:**

- **Non-blocking:** The Start() handler waits for LAPI but has a timeout
- **Health check:** Uses `cscli lapi status` (exit code 0 = healthy)
- **Retry logic:** Polls every 500ms instead of continuous checks (reduces CPU)
- **Timeout:** 30 seconds maximum wait (prevents infinite loops)
- **Graceful degradation:** Returns `lapi_ready: false` instead of failing if timeout exceeded

**LAPI Health Endpoint:**

LAPI exposes a health endpoint on `http://localhost:8085/health`:

```bash
curl -s http://localhost:8085/health
```

Response when healthy:

```json
{"status":"up"}
```

This endpoint is used internally by `cscli lapi status`.

### How to Enable CrowdSec

**Step 1: Access Security Dashboard**

1. Navigate to **Security** in the sidebar
2. Find the **CrowdSec** card
3. Toggle the switch to **ON**
4. Wait 10-15 seconds for LAPI to start
5. Verify status shows "Active" with a running PID

**Step 2: Verify LAPI is Running**

```bash
docker exec charon cscli lapi status
```

Expected output:

```
✓ You can successfully interact with Local API (LAPI)
```

**Step 3: (Optional) Enroll in CrowdSec Console**

Once LAPI is running, you can enroll your instance:

1. Go to **Cerberus → CrowdSec**
2. Enable the Console enrollment feature flag (if not already enabled)
3. Click **Enroll with CrowdSec Console**
4. Paste your enrollment token from crowdsec.net
5. Submit

**Prerequisites for Console Enrollment:**

- ✅ CrowdSec must be **enabled** via GUI toggle
- ✅ LAPI must be **running** (verify with `cscli lapi status`)
- ✅ Feature flag `feature.crowdsec.console_enrollment` must be enabled
- ✅ Valid enrollment token from crowdsec.net

⚠️ **Important:** Console enrollment requires an active LAPI connection. If LAPI is not running, the enrollment will appear successful locally but won't register on crowdsec.net.

**Enrollment Retry Logic:**

The console enrollment service automatically checks LAPI availability with retries:

**Implementation** (`internal/services/console_enroll.go:218-246`):

```go
func (s *ConsoleEnrollmentService) checkLAPIAvailable(ctx context.Context) error {
    maxRetries := 3
    retryDelay := 2 * time.Second

    for i := 0; i < maxRetries; i++ {
        checkCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
        defer cancel()

        _, err := s.exec.ExecuteWithEnv(checkCtx, "cscli", []string{"lapi", "status"}, nil)
        if err == nil {
            return nil // LAPI is available
        }

        if i < maxRetries-1 {
            logger.Log().WithError(err).WithField("attempt", i+1).Debug("LAPI not ready, retrying")
            time.Sleep(retryDelay)
        }
    }

    return fmt.Errorf("CrowdSec Local API is not running after %d attempts", maxRetries)
}
```

**Retry Parameters:**

- **Max retries:** 3 attempts
- **Retry delay:** 2 seconds between attempts
- **Total retry window:** Up to 6 seconds (3 attempts × 2 seconds)
- **Command timeout:** 5 seconds per attempt

**Retry Flow:**

1. **Attempt 1** — Immediate LAPI check
2. **Wait 2 seconds** (if failed)
3. **Attempt 2** — Retry LAPI check
4. **Wait 2 seconds** (if failed)
5. **Attempt 3** — Final LAPI check
6. **Return error** — If all 3 attempts fail

This handles most race conditions where LAPI is still initializing after CrowdSec start.

### How CrowdSec Works in Charon

**Startup Flow:**

1. Container starts → CrowdSec config initialized (but agent NOT started)
2. User toggles CrowdSec switch in GUI → Frontend calls `/api/v1/admin/crowdsec/start`
3. Backend handler starts LAPI process → PID tracked in backend
4. User can verify status in Security dashboard
5. User toggles OFF → Backend calls `/api/v1/admin/crowdsec/stop`

**This matches the pattern used by other security features:**

| Feature | Control Method | Status Endpoint | Lifecycle Handler |
|---------|---------------|-----------------|-------------------|
| **Cerberus** | GUI Toggle | `/security/status` | N/A (master switch) |
| **WAF** | GUI Toggle | `/security/status` | Config regeneration |
| **ACL** | GUI Toggle | `/security/status` | Config regeneration |
| **Rate Limit** | GUI Toggle | `/security/status` | Config regeneration |
| **CrowdSec** | ✅ GUI Toggle | `/security/status` | Start/Stop handlers |

### Import Config Feature

The import config feature (`importCrowdsecConfig`) allows you to:

1. Upload a complete CrowdSec configuration (tar.gz)
2. Import pre-configured settings, collections, and bouncers
3. Manage CrowdSec entirely through Charon's GUI

**This replaced the need for "external" mode:**

- **Old way (deprecated):** Set `CROWDSEC_MODE=external` and point to external LAPI
- **New way:** Import your existing config and let Charon manage it internally

### Troubleshooting

**Problem:** Console enrollment shows "enrolled" locally but doesn't appear on crowdsec.net

**Technical Analysis:**
LAPI must be fully initialized before enrollment. Even with automatic retries, there's a window where LAPI might not be ready.

**Solution:**

1. **Verify LAPI process is running:**

   ```bash
   docker exec charon ps aux | grep crowdsec
   ```

   Expected output:

   ```
   crowdsec  203  0.5  2.3  /usr/local/bin/crowdsec -c /app/data/crowdsec/config/config.yaml
   ```

2. **Check LAPI status:**

   ```bash
   docker exec charon cscli lapi status
   ```

   Expected output:

   ```
   ✓ You can successfully interact with Local API (LAPI)
   ```

   If not ready:

   ```
   ERROR: cannot contact local API
   ```

3. **Check LAPI health endpoint:**

   ```bash
   docker exec charon curl -s http://localhost:8085/health
   ```

   Expected response:

   ```json
   {"status":"up"}
   ```

4. **Check LAPI can process requests:**

   ```bash
   docker exec charon cscli machines list
   ```

   Expected output:

   ```
   Name                     IP Address      Auth Type    Version
   charon-local-machine     127.0.0.1      password     v1.x.x
   ```

5. **If LAPI is not running:**
   - Go to Security dashboard
   - Toggle CrowdSec **OFF**, then **ON** again
   - **Wait 15 seconds** (critical: LAPI needs time to initialize)
   - Verify LAPI is running (repeat checks above)
   - Re-submit enrollment token

6. **Monitor LAPI startup:**

   ```bash
   # Watch CrowdSec logs in real-time
   docker logs -f charon | grep -i crowdsec
   ```

   Look for:
   - ✅ "Starting CrowdSec Local API"
   - ✅ "CrowdSec Local API listening on 127.0.0.1:8085"
   - ✅ "parsers loaded: 4"
   - ✅ "scenarios loaded: 46"
   - ❌ "error" or "fatal" (indicates startup problem)

**Problem:** CrowdSec won't start after toggling

**Solution:**

1. **Check logs for errors:**

   ```bash
   docker logs charon | grep -i error | tail -20
   ```

2. **Common startup issues:**

   **Issue: Config directory missing**

   ```bash
   # Check directory exists
   docker exec charon ls -la /app/data/crowdsec/config

   # If missing, restart container to regenerate
   docker compose restart
   ```

   **Issue: Port conflict (8085 in use)**

   ```bash
   # Check port usage
   docker exec charon netstat -tulpn | grep 8085

   # If another process is using port 8085, stop it or change CrowdSec LAPI port
   ```

   **Issue: Permission errors**

   ```bash
   # Fix ownership (run on host machine)
   sudo chown -R 1000:1000 ./data/crowdsec
   docker compose restart
   ```

3. **Remove deprecated environment variables:**

   Edit `docker-compose.yml` and remove:

   ```yaml
   # REMOVE THESE DEPRECATED VARIABLES:
   - CHARON_SECURITY_CROWDSEC_MODE=local
   - CERBERUS_SECURITY_CROWDSEC_MODE=local
   - CPM_SECURITY_CROWDSEC_MODE=local
   ```

   Then restart:

   ```bash
   docker compose down
   docker compose up -d
   ```

4. **Verify CrowdSec binary exists:**

   ```bash
   docker exec charon which crowdsec
   # Expected: /usr/local/bin/crowdsec

   docker exec charon which cscli
   # Expected: /usr/local/bin/cscli
   ```

**Expected LAPI Startup Times:**

- **Initial start:** 5-10 seconds
- **First start after container restart:** 10-15 seconds
- **With many scenarios/parsers:** Up to 20 seconds
- **Maximum timeout:** 30 seconds (Start() handler limit)

**Performance Monitoring:**

```bash
# Check CrowdSec resource usage
docker exec charon ps aux | grep crowdsec

# Check LAPI response time
time docker exec charon curl -s http://localhost:8085/health

# Monitor LAPI availability over time
watch -n 5 'docker exec charon cscli lapi status'
```

See also: [CrowdSec Troubleshooting Guide](troubleshooting/crowdsec.md)

---

## Security Decisions

The `SecurityDecision` table logs all security actions:

```go
type SecurityDecision struct {
    ID        uint      `gorm:"primaryKey"`
    Source    string    `json:"source"`    // waf, crowdsec, acl, ratelimit, manual
    IPAddress string    `json:"ip_address"`
    Action    string    `json:"action"`    // allow, block, challenge
    Reason    string    `json:"reason"`
    Timestamp time.Time `json:"timestamp"`
}
```

**Use cases:**

- Audit trail for compliance
- UI visibility into recent blocks
- Manual override tracking

---

## Self-Lockout Prevention

### Admin Whitelist

**Purpose:** Prevent admins from blocking themselves

**Implementation:**

- Stored in `SecurityConfig.admin_whitelist` as CSV
- Checked before applying any block decision
- If requesting IP matches whitelist → always allow

**Recommendation:** Add your VPN IP, Tailscale IP, or home network before enabling Cerberus.

### Break-Glass Token

**Purpose:** Emergency disable when locked out

**How it works:**

1. Generate via `POST /api/v1/security/breakglass/generate`
2. Returns one-time token (plaintext, never stored hashed)
3. Token can be used in `POST /api/v1/security/disable` to turn off Cerberus
4. Token expires after first use

**Storage:** Tokens are hashed in database using bcrypt.

### Localhost Bypass

Requests from `127.0.0.1` or `::1` may bypass security checks (configurable). Allows local management access even when locked out.

---

## API Reference

### Status

```http
GET /api/v1/security/status
```

Returns:

```json
{
  "enabled": true,
  "waf_mode": "monitor",
  "crowdsec_mode": "local",
  "acl_enabled": true,
  "ratelimit_enabled": false
}
```

### Enable Cerberus

```http
POST /api/v1/security/enable
Content-Type: application/json

{
  "admin_whitelist": "198.51.100.10,203.0.113.0/24"
}
```

Requires either:

- `admin_whitelist` with at least one IP/CIDR
- OR valid break-glass token in header

### Disable Cerberus

```http
POST /api/v1/security/disable
```

Requires either:

- Request from localhost
- OR valid break-glass token in header

### Get/Update Config

```http
GET /api/v1/security/config
POST /api/v1/security/config
```

See SecurityConfig schema above.

### Rulesets

```http
GET /api/v1/security/rulesets
POST /api/v1/security/rulesets
DELETE /api/v1/security/rulesets/:id
```

### Decisions (Audit Log)

```http
GET /api/v1/security/decisions?limit=50
POST /api/v1/security/decisions  # Manual override
```

---

## Testing

### Integration Test

Run the Coraza integration test:

```bash
bash scripts/coraza_integration.sh
```

Or via Go:

```bash
cd backend
go test -tags=integration ./integration -run TestCorazaIntegration -v
```

### Manual Testing

1. Enable WAF in `monitor` mode
2. Send request with `<script>` in query string
3. Check `/api/v1/security/decisions` for logged attempt
4. Switch to `block` mode
5. Repeat — should receive 403

---

## Observability

### Recommended Dashboards

**Block Rate:**

```promql
rate(charon_waf_blocked_total[5m]) / rate(charon_waf_requests_total[5m])
```

**Monitor vs Block Comparison:**

```promql
rate(charon_waf_monitored_total[5m])
rate(charon_waf_blocked_total[5m])
```

### Alerting Rules

**High block rate (potential attack):**

```yaml
alert: HighWAFBlockRate
expr: rate(charon_waf_blocked_total[5m]) > 0.3
for: 10m
annotations:
  summary: "WAF blocking >30% of requests"
```

**No WAF evaluation (misconfiguration):**

```yaml
alert: WAFNotEvaluating
expr: rate(charon_waf_requests_total[10m]) == 0
for: 15m
annotations:
  summary: "WAF received zero requests, check middleware config"
```

---

## Development Roadmap

| Phase | Feature | Status |
|-------|---------|--------|
| 1 | WAF placeholder + metrics | ✅ Complete |
| 2 | ACL implementation | ✅ Complete |
| 3 | Break-glass token | ✅ Complete |
| 4 | Coraza CRS integration | 📋 Planned |
| 5 | CrowdSec local agent | 📋 Planned |
| 6 | Rate limiting enforcement | 📋 Planned |
| 7 | Adaptive learning/tuning | 🔮 Future |

---

## FAQ

### Why is the WAF just a placeholder?

We wanted to ship the architecture and observability first. This lets you enable monitoring, see the metrics, and prepare dashboards before the full rule engine is integrated.

### Can I use my own WAF rules?

Yes, via `/api/v1/security/rulesets`. Upload custom Coraza-compatible rules.

### Does Cerberus protect Caddy's proxy traffic?

Not yet. Currently it only protects the management API (`/api/v1`). Future versions will integrate directly with Caddy's request pipeline to protect proxied traffic.

### Why is monitor mode still blocking?

Known issue with the placeholder implementation. This will be fixed when Coraza integration is complete.

---

## See Also

- [Security Features (User Guide)](security.md)
- [API Documentation](api.md)
- [Features Overview](features.md)
