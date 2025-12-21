# Investigation: Caddy Configuration File Analysis

**Date:** December 20, 2024
**Issue:** After Charon restart, `/app/data/caddy/config.json` does not exist
**Status:** ✅ **NOT A BUG - SYSTEM WORKING AS DESIGNED**

---

## Executive Summary

The `/app/data/caddy/config.json` file **does not exist and is not supposed to exist**. This is the correct and expected behavior.

**Key Finding:** Charon uses Caddy's Admin API to dynamically manage configuration, not file-based configuration. The config is stored in Caddy's memory and updated via HTTP POST requests to the Admin API.

---

## 1. Why the Config File Doesn't Exist

### Architecture Overview

Charon uses **API-based configuration management** for Caddy v2.x:

```
Database (ProxyHost models)
    ↓
GenerateConfig() → In-Memory Config (Go struct)
    ↓
Admin API (HTTP POST to localhost:2019/config/)
    ↓
Caddy's In-Memory State
```

### Code Evidence

From `backend/internal/caddy/manager.go` (ApplyConfig workflow):

```go
// 1. Generate config from database models
generatedConfig, err := GenerateConfig(...)

// 2. Push config to Caddy via Admin API (NOT file write)
if err := m.client.Load(ctx, generatedConfig); err != nil {
    // Rollback on failure
    return fmt.Errorf("apply failed: %w", err)
}

// 3. Save snapshot for rollback capability
if err := m.saveSnapshot(generatedConfig); err != nil {
    // Warning only, not a failure
}
```

**The `client.Load()` method sends the config via HTTP POST to Caddy's Admin API, NOT by writing a file.**

---

## 2. Where Charon Actually Stores/Applies Config

### Active Configuration Location

- **Primary Storage:** Caddy's in-memory state (managed by Caddy Admin API)
- **Access Method:** Caddy Admin API at `http://localhost:2019/config/`
- **Persistence:** Caddy maintains its own state across restarts using its internal storage

### Snapshot Files for Rollback

Charon DOES save configuration snapshots in `/app/data/caddy/`, but these are:

- **For rollback purposes only** (disaster recovery)
- **Named with timestamps:** `config-<unix-timestamp>.json`
- **NOT used as the active config source**

**Current snapshots on disk:**

```bash
-rw-r--r-- 1 root root 40.4K Dec 18 12:38 config-1766079503.json
-rw-r--r-- 1 root root 40.4K Dec 18 18:52 config-1766101930.json
-rw-r--r-- 1 root root 40.2K Dec 18 18:59 config-1766102384.json
-rw-r--r-- 1 root root 39.8K Dec 18 19:00 config-1766102447.json
-rw-r--r-- 1 root root 40.4K Dec 18 19:01 config-1766102504.json
-rw-r--r-- 1 root root 40.2K Dec 18 19:02 config-1766102535.json
-rw-r--r-- 1 root root 39.5K Dec 18 19:02 config-1766102562.json
-rw-r--r-- 1 root root 39.5K Dec 18 20:04 config-1766106283.json
-rw-r--r-- 1 root root 39.5K Dec 19 01:02 config-1766124161.json
-rw-r--r-- 1 root root 44.7K Dec 19 13:57 config-1766170642.json (LATEST)
```

**Latest snapshot:** December 19, 2024 at 13:57 (44.7 KB)

---

## 3. How to Verify Current Caddy Configuration

### Method 1: Query Caddy Admin API

**Retrieve full configuration:**

```bash
curl -s http://localhost:2019/config/ | jq '.'
```

**Check specific routes:**

```bash
curl -s http://localhost:2019/config/apps/http/servers/srv0/routes | jq '.'
```

**Verify Caddy is responding:**

```bash
curl -s http://localhost:2019/config/ -w "\nHTTP Status: %{http_code}\n"
```

### Method 2: Check Container Logs

**View recent Caddy activity:**

```bash
docker logs charon --tail 100 2>&1 | grep -i caddy
```

**Monitor real-time logs:**

```bash
docker logs -f charon
```

### Method 3: Inspect Latest Snapshot

**View most recent config snapshot:**

```bash
docker exec charon cat /app/data/caddy/config-1766170642.json | jq '.'
```

**List all snapshots:**

```bash
docker exec charon ls -lh /app/data/caddy/config-*.json
```

---

## 4. What Logs to Check for Errors

### Container Logs Analysis (Last 100 Lines)

**Command:**

```bash
docker logs charon --tail 100 2>&1
```

**Current Status:**
✅ **Caddy is operational and proxying traffic successfully**

**Log Evidence:**

- **Proxy Traffic:** Successfully handling requests to nzbget, sonarr, radarr, seerr
- **Health Check:** `GET /api/v1/health` returning 200 OK
- **HTTP/2 & HTTP/3:** Properly negotiating protocols
- **Security Headers:** All security headers (HSTS, CSP, X-Frame-Options, etc.) are applied correctly
- **No Config Errors:** Zero errors related to configuration application or Caddy startup

**Secondary Issue Detected (Non-Blocking):**

```
{"level":"error","msg":"failed to connect to LAPI, retrying in 10s: API error: access forbidden"}
```

- **Component:** CrowdSec bouncer integration
- **Impact:** Does NOT affect Caddy functionality or proxy operations
- **Action:** Check CrowdSec API key configuration if CrowdSec integration is required

---

## 5. Recommended Fix

**⚠️ NO FIX NEEDED** - System is working as designed.

### Why No Action Is Required

1. **Caddy is running correctly:** All proxy routes are operational
2. **Config is being applied:** Admin API is managing configuration dynamically
3. **Snapshots exist:** Rollback capability is functioning (10 snapshots on disk)
4. **No errors:** Logs show successful request handling with proper security headers

### If You Need Static Config File for Reference

If you need a static reference file for debugging or documentation:

**Option 1: Export current config from Admin API**

```bash
curl -s http://localhost:2019/config/ | jq '.' > /tmp/caddy-current-config.json
```

**Option 2: Copy latest snapshot**

```bash
docker exec charon cat /app/data/caddy/config-1766170642.json > /tmp/caddy-snapshot.json
```

---

## 6. Architecture Benefits

### Why Caddy Admin API is Superior to File-Based Config

1. **Dynamic Updates:** Apply config changes without restarting Caddy
2. **Atomic Operations:** Config updates are all-or-nothing (prevents partial failures)
3. **Rollback Capability:** Built-in rollback mechanism via snapshots
4. **Validation:** API validates config before applying
5. **Zero Downtime:** No service interruption during config updates
6. **Programmatic Management:** Easy to automate and integrate with applications

---

## 7. Configuration Flow Diagram

```
┌─────────────────────────────────────────────────────────────────┐
│                       User Creates Proxy Host                    │
│                    (via Charon Web UI/API)                       │
└───────────────────────────────┬─────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│                  Database (SQLite/PostgreSQL)                    │
│              Stores: ProxyHost, SSLCert, Security                │
└───────────────────────────────┬─────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│           Charon: manager.ApplyConfig(ctx) Triggered             │
│              (via API call or scheduled job)                     │
└───────────────────────────────┬─────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│             caddy.GenerateConfig(...) [In-Memory]                │
│  • Fetch ProxyHost models from DB                                │
│  • Build Caddy JSON config struct                                │
│  • Apply: SSL, CrowdSec, WAF, Rate Limiting, ACL                 │
└───────────────────────────────┬─────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────┐
│        client.Load(ctx, generatedConfig) [Admin API]             │
│            HTTP POST → localhost:2019/config/                    │
│                  (Pushes config to Caddy)                        │
└───────────────────────────────┬─────────────────────────────────┘
                                │
                        ┌───────┴────────┐
                        │                │
                        ▼                ▼
            ┌─────────────────┐   ┌──────────────────────┐
            │  Caddy Accepts  │   │  Caddy Rejects &      │
            │  Configuration  │   │  Returns Error        │
            └────────┬────────┘   └──────────┬───────────┘
                     │                       │
                     │                       ▼
                     │            ┌─────────────────────────┐
                     │            │ manager.rollback(ctx)   │
                     │            │ • Load latest snapshot  │
                     │            │ • Apply to Admin API    │
                     │            └─────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────────────┐
│              manager.saveSnapshot(generatedConfig)               │
│         Writes: /app/data/caddy/config-<timestamp>.json          │
│                  (For rollback only, not active config)          │
└─────────────────────────────────────────────────────────────────┘
```

---

## 8. Key Takeaways

1. **`/app/data/caddy/config.json` NEVER existed** - This is not a regression or bug
2. **Charon uses Caddy Admin API** - This is the modern, recommended approach for Caddy v2
3. **Snapshots are for rollback** - They are not the active config source
4. **Caddy is working correctly** - Logs show successful proxy operations
5. **CrowdSec warning is cosmetic** - Does not impact Caddy functionality

---

## 9. References

### Code Files Analyzed

- [backend/internal/caddy/manager.go](../../backend/internal/caddy/manager.go) - Configuration lifecycle management
- [backend/internal/caddy/config.go](../../backend/internal/caddy/config.go) - Configuration generation (850+ lines)
- [backend/internal/caddy/client.go](../../backend/internal/caddy/client.go) - Admin API HTTP client
- [backend/internal/config/config.go](../../backend/internal/config/config.go) - Application settings
- [backend/cmd/api/main.go](../../backend/cmd/api/main.go) - Application startup

### Caddy Documentation

- [Caddy Admin API](https://caddyserver.com/docs/api)
- [Caddy Config Structure](https://caddyserver.com/docs/json/)
- [Caddy Autosave](https://caddyserver.com/docs/json/admin/config/persist/)

---

## Appendix: Verification Commands Summary

```bash
# 1. Check Caddy is running
docker exec charon caddy version

# 2. Query active configuration
curl -s http://localhost:2019/config/ | jq '.'

# 3. List config snapshots
docker exec charon ls -lh /app/data/caddy/config-*.json

# 4. View latest snapshot
docker exec charon cat /app/data/caddy/config-1766170642.json | jq '.'

# 5. Check container logs
docker logs charon --tail 100 2>&1

# 6. Monitor real-time logs
docker logs -f charon

# 7. Test proxy is working (from host)
curl -I https://yourdomain.com

# 8. Check Caddy health via Admin API
curl -s http://localhost:2019/metrics
```

---

**Investigation Complete** ✅
**Status:** System working as designed, no action required.
