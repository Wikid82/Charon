# CrowdSec Bouncer Field Name Investigation

**Date:** December 15, 2025
**Agent:** Backend_Dev
**Status:** 🔴 BLOCKED - Plugin Configuration Schema Unknown

---

## Executive Summary

CrowdSec LAPI is running correctly on port 8085 and responding to queries. However, **the Caddy CrowdSec bouncer cannot connect to LAPI** because the plugin rejects ALL field name variants tested in the JSON configuration.

### Field Names Tested (All Rejected)

- ❌ `api_url` - "json: unknown field"
- ❌ `crowdsec_lapi_url` - "json: unknown field"
- ❌ `lapi_url` - "json: unknown field"
- ❌ `enable_streaming` - "json: unknown field"
- ❌ `ticker_interval` - "json: unknown field"

**Hypothesis:** Configuration may need to be at **app-level** (`apps.crowdsec`) instead of **handler-level** (inline in route).

---

## Current Implementation (Handler-Level)

```go
// backend/internal/caddy/config.go, line 750
func buildCrowdSecHandler(...) (Handler, error) {
    h := Handler{"handler": "crowdsec"}
    h["lapi_url"] = "http://127.0.0.1:8085"
    h["api_key"] = apiKey
    return h, nil
}
```

This generates:

```json
{
  "handle": [
    {
      "handler": "crowdsec",
      "lapi_url": "http://127.0.0.1:8085",
      "api_key": "..."
    }
  ]
}
```

**Result:** `json: unknown field "lapi_url"`

---

## Caddyfile Format (from plugin README)

```caddyfile
{
  crowdsec {
    api_url http://localhost:8080
    api_key <api_key>
    ticker_interval 15s
  }
}
```

**Note:** This is **app-level config**, not handler-level!

---

## Proposed Solution: App-Level Configuration

### Structure A: Dedicated CrowdSec App

```json
{
  "apps": {
    "http": {...},
    "crowdsec": {
      "api_url": "http://127.0.0.1:8085",
      "api_key": "..."
    }
  }
}
```

Handler becomes:

```json
{
  "handler": "crowdsec"  // No inline config
}
```

### Structure B: HTTP App Config

```json
{
  "apps": {
    "http": {
      "crowdsec": {
        "api_url": "http://127.0.0.1:8085",
        "api_key": "..."
      },
      "servers": {...}
    }
  }
}
```

---

## Next Steps

1. **Research Plugin Source:**

   ```bash
   git clone https://github.com/hslatman/caddy-crowdsec-bouncer
   cd caddy-crowdsec-bouncer
   grep -r "json:" --include="*.go"
   ```

2. **Test App-Level Config:**
   - Modify `GenerateConfig()` to add `apps.crowdsec`
   - Remove inline config from handler
   - Rebuild and test

3. **Fallback:**
   - File issue with plugin maintainer
   - Request JSON configuration documentation

---

**Blocker:** Unknown JSON configuration schema for caddy-crowdsec-bouncer
**Recommendation:** Pause CrowdSec bouncer work until plugin configuration is clarified
**Impact:** Critical - Zero blocking functionality in production
