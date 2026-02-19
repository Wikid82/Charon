# 📋 Plan: Security Features Deep Dive - Issues #17, #18, #19

**Created**: December 5, 2025
**Status**: Analysis Complete - Implementation Assessment

---

## 🧐 Executive Summary

After a comprehensive analysis of the CrowdSec (#17), WAF (#18), and Rate Limiting (#19) features, the findings show that **all three features are substantially implemented** with working frontend UIs, backend APIs, and Caddy integration. However, each has specific gaps that need to be addressed for full production readiness.

---

## 📊 Implementation Status Matrix

| Feature | Backend | Frontend | Caddy Integration | Testing | Status |
|---------|---------|----------|-------------------|---------|--------|
| **CrowdSec (#17)** | ✅ 90% | ✅ 90% | ⚠️ 70% | ⚠️ 50% | Near Complete |
| **WAF (#18)** | ✅ 95% | ✅ 95% | ✅ 85% | ✅ 80% | Near Complete |
| **Rate Limiting (#19)** | ⚠️ 60% | ✅ 90% | ⚠️ 40% | ⚠️ 30% | Needs Work |

---

## 🔍 Issue #17: CrowdSec Integration (Critical)

### What's Implemented ✅

**Backend:**

- CrowdSec handler (`crowdsec_handler.go`) with:
  - Start/Stop process control via `CrowdsecExecutor` interface
  - Status monitoring endpoint
  - Import/Export configuration (tar.gz)
  - File listing/reading/writing for config files
  - Routes registered at `/admin/crowdsec/*`

- Security handler integration:
  - `security.crowdsec.mode` setting (disabled/local)
  - `security.crowdsec.enabled` runtime override
  - CrowdSec enabled flag computed in `computeEffectiveFlags()`

**Frontend:**

- `CrowdSecConfig.tsx` page with:
  - Mode selection (disabled/local)
  - Import configuration (file upload)
  - Export configuration (download)
  - File editor for config files
  - Loading states and error handling

**Docker:**

- CrowdSec binary installed at `/usr/local/bin/crowdsec`
- Config directory at `/app/data/crowdsec`
- `caddy-crowdsec-bouncer` plugin compiled into Caddy

### Gaps Identified ❌

1. **Banned IP Dashboard** - Not implemented
   - Need `/api/v1/crowdsec/decisions` endpoint to list banned IPs
   - Need frontend UI to display and manage banned IPs

2. **Manual IP Ban/Unban** - Partially implemented
   - `SecurityDecision` model exists but manual CrowdSec bans not wired
   - Need `/api/v1/crowdsec/ban` and `/api/v1/crowdsec/unban` endpoints

3. **Scenario/Collection Management** - Not implemented
   - No UI for managing CrowdSec scenarios or collections
   - Backend would need to interact with CrowdSec CLI or API

4. **CrowdSec Log Parsing Setup** - Not implemented
   - Need to configure CrowdSec to parse Caddy logs
   - Acquisition config not auto-generated

5. **Caddy Integration Handler** - Placeholder only
   - `buildCrowdSecHandler()` returns `Handler{"handler": "crowdsec"}` but Caddy's `caddy-crowdsec-bouncer` expects different configuration:

     ```json
     {
       "handler": "crowdsec",
       "api_url": "http://localhost:8080",
       "api_key": "..."
     }
     ```

### Acceptance Criteria Assessment

| Criteria | Status |
|----------|--------|
| CrowdSec blocks malicious IPs automatically | ⚠️ Partial - bouncer configured but handler incomplete |
| Banned IPs visible in dashboard | ❌ Not implemented |
| Can manually ban/unban IPs | ⚠️ Partial - backend exists but not wired |
| CrowdSec status visible | ✅ Implemented |

---

## 🔍 Issue #18: WAF Integration (High Priority)

### What's Implemented ✅

**Backend:**

- `SecurityRuleSet` model for storing WAF rules
- `SecurityConfig.WAFMode` (disabled/monitor/block)
- `SecurityConfig.WAFRulesSource` for ruleset selection
- `buildWAFHandler()` generates Coraza handler config:

  ```go
  h := Handler{"handler": "waf"}
  h["directives"] = fmt.Sprintf("Include %s", rulesetPath)
  ```

- Ruleset files written to `/app/data/caddy/coraza/rulesets/`
- `SecRuleEngine On/DetectionOnly` auto-prepended based on mode
- Security service CRUD for rulesets

**Frontend:**

- `WafConfig.tsx` with:
  - Rule set CRUD (create, edit, delete)
  - Mode selection (blocking/detection)
  - WAF presets (OWASP CRS, SQLi protection, XSS protection, Bad Bots)
  - Source URL or inline content support
  - Rule count display

**Docker:**

- `coraza-caddy/v2` plugin compiled into Caddy

**Testing:**

- Integration test `coraza_integration_test.go`
- Unit tests for WAF handler building

### Gaps Identified ❌

1. **WAF Logging and Alerts** - Partially implemented
   - Coraza logs to Caddy but not parsed/displayed in UI
   - No WAF-specific notifications

2. **WAF Statistics Dashboard** - Not implemented
   - Need metrics collection (requests blocked, attack types)
   - Prometheus metrics defined in docs but not implemented

3. **Paranoia Level Selector** - Not implemented
   - OWASP CRS paranoia levels (1-4) not exposed in UI
   - Would need `SecAction "id:900000,setvar:tx.paranoia_level=2"`

4. **Per-Host WAF Toggle** - Partially implemented
   - `host.AdvancedConfig` can reference `ruleset_name` but no UI
   - Need checkbox in ProxyHostForm for "Enable WAF"

5. **Rule Exclusion System** - Not implemented
   - No UI for excluding specific rules that cause false positives
   - Would need `SecRuleRemoveById` directive management

### Acceptance Criteria Assessment

| Criteria | Status |
|----------|--------|
| WAF blocks common attacks (SQLi, XSS) | ✅ Working with Coraza |
| Can enable/disable per host | ⚠️ Via advanced config only |
| False positives manageable | ❌ No exclusion UI |
| WAF events logged and visible | ⚠️ Logs exist but not in UI |

---

## 🔍 Issue #19: Rate Limiting (High Priority)

### What's Implemented ✅

**Backend:**

- `SecurityConfig` model fields:

  ```go
  RateLimitEnable    bool
  RateLimitBurst     int
  RateLimitRequests  int
  RateLimitWindowSec int
  ```

- `security.rate_limit.enabled` setting
- `buildRateLimitHandler()` generates config:

  ```go
  h := Handler{"handler": "rate_limit"}
  h["requests"] = secCfg.RateLimitRequests
  h["window_sec"] = secCfg.RateLimitWindowSec
  h["burst"] = secCfg.RateLimitBurst
  ```

**Frontend:**

- `RateLimiting.tsx` with:
  - Enable/disable toggle
  - Requests per second input
  - Burst allowance input
  - Window (seconds) input
  - Save configuration

### Gaps Identified ❌

1. **Caddy Rate Limit Directive** - **CRITICAL GAP**
   - Caddy doesn't have a built-in `rate_limit` handler
   - Need to use `caddy-ratelimit` module or Caddy's `respond` with headers
   - Current handler is a no-op placeholder

2. **Rate Limit Presets** - Not implemented
   - Issue specifies presets: login, API, standard
   - Need predefined configurations

3. **Per-IP Rate Limiting** - Not implemented correctly
   - Handler exists but Caddy module not compiled in
   - Need `github.com/mholt/caddy-ratelimit` in Dockerfile

4. **Per-Endpoint Rate Limits** - Not implemented
   - No UI for path-specific rate limits
   - Would need rate limit rules per route

5. **Bypass List (Trusted IPs)** - Not implemented
   - Admin whitelist exists but not connected to rate limiting

6. **Rate Limit Violation Logging** - Not implemented
   - No logging when rate limits are hit

7. **Rate Limit Testing Tool** - Not implemented
   - No way to test rate limits from UI

### Acceptance Criteria Assessment

| Criteria | Status |
|----------|--------|
| Rate limits prevent brute force | ❌ Handler is placeholder |
| Presets work correctly | ❌ Not implemented |
| Legitimate traffic not affected | ⚠️ No bypass list |
| Rate limit hits logged | ❌ Not implemented |

---

## 🤝 Handoff Contracts (API Specifications)

### CrowdSec Banned IPs API

```json
// GET /api/v1/crowdsec/decisions
{
  "response": {
    "decisions": [
      {
        "id": "uuid",
        "ip": "192.168.1.100",
        "reason": "ssh-bf",
        "duration": "4h",
        "created_at": "2025-12-05T10:00:00Z",
        "source": "crowdsec"
      }
    ],
    "total": 15
  }
}

// POST /api/v1/crowdsec/ban
{
  "request": {
    "ip": "192.168.1.100",
    "duration": "24h",
    "reason": "Manual ban - suspicious activity"
  },
  "response": {
    "success": true,
    "decision_id": "uuid"
  }
}

// DELETE /api/v1/crowdsec/ban/:ip
{
  "response": {
    "success": true
  }
}
```

### Rate Limit Caddy Integration Fix

The rate limit handler needs to output proper Caddy JSON:

```json
// Correct Caddy rate_limit handler format (requires caddy-ratelimit module)
{
  "handler": "rate_limit",
  "rate_limits": {
    "static": {
      "match": [{"method": ["GET", "POST"]}],
      "key": "{http.request.remote.host}",
      "window": "1m",
      "max_events": 60
    }
  }
}
```

---

## 🏗️ Implementation Phases

### Phase 1: Rate Limiting Fix (Critical - Blocking Beta)

**Backend Changes:**

1. Add `github.com/mholt/caddy-ratelimit` to Dockerfile xcaddy build
2. Fix `buildRateLimitHandler()` to output correct Caddy JSON format
3. Add rate limit bypass using admin whitelist

**Frontend Changes:**

1. Add presets dropdown (Login: 5/min, API: 100/min, Standard: 30/min)
2. Add bypass IP list input (reuse admin whitelist)

### Phase 2: CrowdSec Completeness (High Priority)

**Backend Changes:**

1. Create `/api/v1/crowdsec/decisions` endpoint (call cscli)
2. Create `/api/v1/crowdsec/ban` and `unban` endpoints
3. Fix `buildCrowdSecHandler()` to include proper bouncer config
4. Auto-generate acquisition.yaml for Caddy log parsing

**Frontend Changes:**

1. Add "Banned IPs" tab to CrowdSecConfig page
2. Add "Ban IP" button with duration selector
3. Add "Unban" action to each banned IP row

### Phase 3: WAF Enhancements (Medium Priority)

**Backend Changes:**

1. Add paranoia level to SecurityConfig model
2. Add rule exclusion list to SecurityRuleSet model
3. Parse Coraza logs for WAF events

**Frontend Changes:**

1. Add paranoia level slider (1-4) to WAF config
2. Add "Enable WAF" checkbox to ProxyHostForm
3. Add rule exclusion UI (list of rule IDs to exclude)
4. Add WAF events log viewer

### Phase 4: Testing & QA

1. Create integration tests for each feature
2. Add E2E tests for security flows
3. Manual penetration testing

---

## 🕵️ QA & Security Considerations

### CrowdSec Security

- Ensure API key not exposed in logs
- Validate IP inputs to prevent injection
- Rate limit the ban/unban endpoints themselves

### WAF Security

- Validate ruleset content (no malicious directives)
- Prevent path traversal in ruleset file paths
- Test for WAF bypass techniques

### Rate Limiting Security

- Prevent bypass via IP spoofing (X-Forwarded-For)
- Ensure rate limits apply to all methods
- Test distributed rate limiting behavior

---

## 📚 Documentation Updates Needed

1. Update `docs/cerberus.md` with actual implementation status
2. Update `docs/security.md` user guide with new features
3. Add rate limiting configuration guide
4. Add CrowdSec setup wizard documentation

---

## 🎯 Priority Order

1. **Rate Limiting Caddy Module** - Blocking issue, handler is no-op
2. **CrowdSec Banned IP Dashboard** - High visibility feature
3. **WAF Per-Host Toggle** - User expectation from issue
4. **CrowdSec Manual Ban/Unban** - Security operations feature
5. **WAF Rule Exclusions** - False positive management
6. **Rate Limit Presets** - UX improvement

---

## Summary: What Works vs What Doesn't

### ✅ Working Now

- WAF rule management and blocking (Coraza integration)
- CrowdSec process control (start/stop/status)
- CrowdSec config import/export
- Rate limiting UI and settings storage
- Security status API reporting

### ⚠️ Partially Working

- CrowdSec bouncer (handler exists but config incomplete)
- Per-host WAF (via advanced config only)
- Rate limiting settings (stored but not enforced)

### ❌ Not Working / Missing

- Rate limiting actual enforcement (Caddy module missing)
- CrowdSec banned IP dashboard
- Manual IP ban/unban
- WAF rule exclusions
- Rate limit presets
- WAF paranoia levels
