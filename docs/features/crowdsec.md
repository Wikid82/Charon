---
title: CrowdSec Integration
description: Behavior-based threat detection powered by a global community
---

# CrowdSec Integration

Protect your applications using behavior-based threat detection powered by a global community of security data. Bad actors get blocked automatically before they can cause harm.

## Overview

CrowdSec analyzes your traffic patterns and blocks malicious behavior in real-time. Unlike traditional firewalls that rely on static rules, CrowdSec uses behavioral analysis and crowdsourced threat intelligence to identify and stop attacks.

Key capabilities:

- **Behavior Detection** — Identifies attack patterns like brute-force, scanning, and exploitation
- **Community Blocklists** — Benefit from threats detected by the global CrowdSec community
- **Real-time Blocking** — Malicious IPs are blocked immediately via Caddy integration
- **Automatic Updates** — Threat intelligence updates continuously

## Why Use This

- **Proactive Defense** — Block attackers before they succeed
- **Zero False Positives** — Behavioral analysis reduces incorrect blocks
- **Community Intelligence** — Leverage data from thousands of CrowdSec users
- **GUI-Controlled** — Enable/disable directly from the UI, no environment variables needed

## Configuration

### Enabling CrowdSec

1. Navigate to **Settings → Security**
2. Toggle **CrowdSec Protection** to enabled
3. CrowdSec starts automatically and persists across container restarts

No environment variables or manual configuration required.

### Hub Presets

Access pre-built security configurations from the CrowdSec Hub:

1. Go to **Settings → Security → Hub Presets**
2. Browse available collections (e.g., `crowdsecurity/nginx`, `crowdsecurity/http-cve`)
3. Search for specific parsers, scenarios, or collections
4. Click **Install** to add to your configuration

Popular presets include:

- **HTTP Probing** — Detect reconnaissance and scanning
- **Bad User-Agents** — Block known malicious bots
- **CVE Exploits** — Protection against known vulnerabilities

### Console Enrollment

Connect to the CrowdSec Console for centralized management:

1. Go to **Settings → Security → Console Enrollment**
2. Enter your enrollment key from [console.crowdsec.net](https://console.crowdsec.net)
3. Click **Enroll**

The Console provides:

- Multi-instance management
- Historical attack data
- Alert notifications
- Blocklist subscriptions

### Live Decisions

View active blocks in real-time:

1. Navigate to **Security → Live Decisions**
2. See all currently blocked IPs with:
   - IP address and origin country
   - Reason for block (scenario triggered)
   - Duration remaining
   - Option to manually unban

## Automatic Startup & Persistence

CrowdSec settings are stored in Charon's database and synchronized with the Security Config:

- **On Container Start** — CrowdSec launches automatically if previously enabled
- **Configuration Sync** — Changes in the UI immediately apply to CrowdSec
- **State Persistence** — Decisions and configurations survive restarts

## Troubleshooting Console Enrollment

### Engine Shows "Offline" in Console

Your CrowdSec Console dashboard shows your engine as "Offline" even though it's running locally.

**Why this happens:**

CrowdSec sends periodic "heartbeats" to the Console to confirm it's alive. If heartbeats stop reaching the Console servers, your engine appears offline.

**Quick check:**

Run the diagnostic script to test connectivity:

```bash
./scripts/diagnose-crowdsec.sh
```

Or use the API endpoint:

```bash
curl http://localhost:8080/api/v1/cerberus/crowdsec/diagnostics/connectivity
```

**Common causes and fixes:**

| Cause | Fix |
|-------|-----|
| Firewall blocking outbound HTTPS | Allow connections to `api.crowdsec.net` on port 443 |
| DNS resolution failure | Verify `nslookup api.crowdsec.net` works |
| Proxy not configured | Set `HTTP_PROXY`/`HTTPS_PROXY` environment variables |
| Heartbeat service not running | Force a manual heartbeat (see below) |

**Force a manual heartbeat:**

```bash
curl -X POST http://localhost:8080/api/v1/cerberus/crowdsec/console/heartbeat
```

### Enrollment Token Expired or Invalid

**Error messages:**

- "token expired"
- "unauthorized"
- "invalid enrollment key"

**Solution:**

1. Log in to [console.crowdsec.net](https://console.crowdsec.net)
2. Navigate to **Instances → Add Instance**
3. Generate a new enrollment token
4. Paste the new token in Charon's enrollment form

Tokens expire after a set period. Always use a freshly generated token.

### LAPI Not Started / Connection Refused

**Error messages:**

- "connection refused"
- "LAPI not available"

**Why this happens:**

CrowdSec's Local API (LAPI) needs 30-60 seconds to fully start after the container launches.

**Check LAPI status:**

```bash
docker exec charon cscli lapi status
```

**If you see "connection refused":**

1. Wait 60 seconds after container start
2. Check CrowdSec is enabled in the Security dashboard
3. Try toggling CrowdSec OFF then ON again

### Already Enrolled Error

**Error message:** "instance already enrolled"

**Why this happens:**

A previous enrollment attempt succeeded but Charon's local state wasn't updated.

**Verify enrollment:**

1. Log in to [console.crowdsec.net](https://console.crowdsec.net)
2. Check **Instances** — your engine may already appear
3. If it's listed, Charon just needs to sync

**Force a re-sync:**

```bash
curl -X POST http://localhost:8080/api/v1/cerberus/crowdsec/console/heartbeat
```

### Network/Firewall Issues

**Symptom:** Enrollment hangs or times out

**Test connectivity manually:**

```bash
# Check DNS resolution
nslookup api.crowdsec.net

# Test HTTPS connectivity
curl -I https://api.crowdsec.net
```

**Required outbound connections:**

| Host | Port | Purpose |
|------|------|---------|
| `api.crowdsec.net` | 443 | Console API and heartbeats |
| `hub.crowdsec.net` | 443 | Hub presets download |

## Using the Diagnostic Script

The diagnostic script checks CrowdSec connectivity and configuration in one command.

**Run all diagnostics:**

```bash
./scripts/diagnose-crowdsec.sh
```

**Output as JSON (for automation):**

```bash
./scripts/diagnose-crowdsec.sh --json
```

**Use a custom data directory:**

```bash
./scripts/diagnose-crowdsec.sh --data-dir /custom/path
```

**What it checks:**

- LAPI availability and health
- CAPI (Central API) connectivity
- Console enrollment status
- Heartbeat service status
- Configuration file validity

## Diagnostic API Endpoints

Access diagnostics programmatically through these API endpoints:

| Endpoint | Method | What It Does |
|----------|--------|--------------|
| `/api/v1/cerberus/crowdsec/diagnostics/connectivity` | GET | Tests LAPI and CAPI connectivity |
| `/api/v1/cerberus/crowdsec/diagnostics/config` | GET | Validates enrollment configuration |
| `/api/v1/cerberus/crowdsec/console/heartbeat` | POST | Forces an immediate heartbeat check |

**Example: Check connectivity**

```bash
curl http://localhost:8080/api/v1/cerberus/crowdsec/diagnostics/connectivity
```

**Example response:**

```json
{
  "lapi": {
    "status": "healthy",
    "latency_ms": 12
  },
  "capi": {
    "status": "reachable",
    "latency_ms": 145
  }
}
```

## Reading the Logs

Look for these log prefixes when debugging:

| Prefix | What It Means |
|--------|---------------|
| `[CROWDSEC_ENROLLMENT]` | Enrollment operations (token validation, CAPI registration) |
| `[HEARTBEAT_POLLER]` | Background heartbeat service activity |
| `[CROWDSEC_STARTUP]` | LAPI initialization and startup |

**View enrollment logs:**

```bash
docker logs charon 2>&1 | grep CROWDSEC_ENROLLMENT
```

**View heartbeat activity:**

```bash
docker logs charon 2>&1 | grep HEARTBEAT_POLLER
```

**Common log patterns:**

| Log Message | Meaning |
|-------------|---------|
| `heartbeat sent successfully` | Console communication working |
| `CAPI registration failed: timeout` | Network issue reaching CrowdSec servers |
| `enrollment completed` | Console enrollment succeeded |
| `retrying enrollment (attempt 2/3)` | Temporary failure, automatic retry in progress |

## Related

- [CrowdSec Setup Guide](../guides/crowdsec-setup.md) — Beginner-friendly setup walkthrough
- [Web Application Firewall](./waf.md) — Complement CrowdSec with WAF protection
- [Access Control](./access-control.md) — Manual IP blocking and geo-restrictions
- [CrowdSec Troubleshooting](../troubleshooting/crowdsec.md) — Extended troubleshooting guide
- [Back to Features](../features.md)
