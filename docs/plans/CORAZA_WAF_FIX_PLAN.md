# 📋 Plan: Coraza WAF Integration Fix

> **Status**: Ready for Implementation
> **Created**: 2024-12-04
> **CI Failure Reference**: [Run #19912145599](https://github.com/Wikid82/Charon/actions/runs/19912145599)

---

## 🧐 UX & Context Analysis

### Current State
The Coraza WAF integration is **architecturally correct** - the plugin is properly compiled into Caddy via xcaddy, and the handler generation pipeline exists. However, the CI integration test consistently fails because the generated Caddy configuration has bugs that prevent the WAF from properly evaluating requests.

### Desired User Flow
1. User creates a WAF ruleset via Security → WAF Config page
2. User enables WAF mode (`block` or `monitor`) in Security settings
3. WAF automatically applies to all proxy hosts
4. Malicious requests are blocked (block mode) or logged (monitor mode)
5. User sees WAF activity in logs and metrics

### Integration Test Expectation
```
POST http://integration.local/post
Body: <script>alert(1)</script>
Expected: HTTP 403 (blocked by Coraza)
Actual: HTTP 200 (request passed through)
```

---

## 🤝 Handoff Contract (The Truth)

### Caddy JSON API - WAF Handler Format

The `coraza-caddy` plugin registers as `http.handlers.waf`. The JSON structure must be:

```json
{
  "handler": "waf",
  "directives": "SecRuleEngine On\nSecRequestBodyAccess On\nInclude /app/data/caddy/coraza/rulesets/integration-xss-a1b2c3d4.conf"
}
```

**Key Fields:**
| Field | Type | Required | Description |
|-------|------|----------|-------------|
| `handler` | string | ✅ | Must be `"waf"` (maps to `http.handlers.waf`) |
| `directives` | string | ✅ | ModSecurity directive string including `Include` statements |
| `load_owasp_crs` | bool | ❌ | If true, loads embedded OWASP CRS (not used in our integration) |

### Ruleset File Format

Files in `/app/data/caddy/coraza/rulesets/{name}-{hash}.conf`:

```modsecurity
SecRuleEngine On
SecRequestBodyAccess On

SecRule REQUEST_BODY "<script>" "id:12345,phase:2,deny,status:403,msg:'XSS blocked'"
```

**Critical Directives:**
- `SecRuleEngine On` → Blocking mode (returns 403)
- `SecRuleEngine DetectionOnly` → Monitor mode (logs but passes through)
- `SecRequestBodyAccess On` → Required to inspect POST bodies

---

## 🔍 Root Cause Analysis

### Bug 1: Ruleset Selection Priority (CRITICAL)
**File**: [backend/internal/caddy/config.go#L743-L748](../backend/internal/caddy/config.go#L743-L748)

```go
// CURRENT (buggy):
if r.Name == "owasp-crs" || (host != nil && r.Name == host.Application) ||
   (hostRulesetName != "" && r.Name == hostRulesetName) ||
   (secCfg != nil && r.Name == secCfg.WAFRulesSource) {
```

**Problem**: `owasp-crs` is checked FIRST, so if any ruleset named "owasp-crs" exists in the database, it will always be selected even when the user specifies a different ruleset via `waf_rules_source`.

**Fix**: Reorder conditions to prioritize user-specified names:
```go
// FIXED:
if (secCfg != nil && secCfg.WAFRulesSource != "" && r.Name == secCfg.WAFRulesSource) ||
   (hostRulesetName != "" && r.Name == hostRulesetName) ||
   (host != nil && r.Name == host.Application) ||
   r.Name == "owasp-crs" {
```

### Bug 2: WAF Handler Returned Without Directives
**File**: [backend/internal/caddy/config.go#L754-L770](../backend/internal/caddy/config.go#L754-L770)

```go
// CURRENT (buggy):
h := Handler{"handler": "waf"}
if selected != nil {
    // set directives...
} else if secCfg != nil && secCfg.WAFRulesSource != "" {
    // set directives...
}
// BUG: Returns handler even if no directives were set!
return h, nil
```

**Problem**: If no matching ruleset is found, the handler is returned without any rules, creating a no-op WAF that blocks nothing.

**Fix**: Return `nil` if no directives could be set:
```go
// FIXED:
h := Handler{"handler": "waf"}
directivesSet := false

if selected != nil {
    if rulesetPaths != nil {
        if p, ok := rulesetPaths[selected.Name]; ok && p != "" {
            h["directives"] = fmt.Sprintf("Include %s", p)
            directivesSet = true
        }
    }
} else if secCfg != nil && secCfg.WAFRulesSource != "" {
    if rulesetPaths != nil {
        if p, ok := rulesetPaths[secCfg.WAFRulesSource]; ok && p != "" {
            h["directives"] = fmt.Sprintf("Include %s", p)
            directivesSet = true
        }
    }
}

if !directivesSet {
    logger.Log().Warn("WAF enabled but no ruleset directives could be set")
    return nil, nil  // Don't create a useless handler
}

return h, nil
```

### Bug 3: Missing Debug Logging for Generated Config
**File**: [backend/internal/caddy/manager.go](../backend/internal/caddy/manager.go)

**Problem**: When the integration test fails, there's no easy way to see what Caddy config was actually generated and sent.

**Fix**: Add structured debug logging:
```go
// In ApplyConfig, after generating handlers:
logger.Log().WithFields(map[string]interface{}{
    "waf_enabled":    wafEnabled,
    "ruleset_count":  len(rulesets),
    "ruleset_paths":  rulesetPaths,
}).Debug("WAF configuration state")
```

### Bug 4: Integration Test Timing Issues
**File**: [scripts/coraza_integration.sh](../scripts/coraza_integration.sh)

**Problem**: The test creates a proxy host, then a ruleset, then security config. Each triggers `ApplyConfig`. The final config might not include the WAF handler if timing is off.

**Fix**: Add explicit config reload and verification:
```bash
# After setting up config, force a reload and verify
echo "Forcing Caddy config reload..."
curl -s http://localhost:8080/api/v1/caddy/reload || true
sleep 3

# Verify WAF handler is present in Caddy config
echo "Verifying WAF handler in Caddy config..."
CADDY_CONFIG=$(curl -s http://localhost:2019/config)
if echo "$CADDY_CONFIG" | grep -q '"handler":"waf"'; then
    echo "✓ WAF handler found in Caddy config"
else
    echo "✗ WAF handler NOT found in Caddy config"
    echo "Caddy config dump:"
    echo "$CADDY_CONFIG" | head -100
    exit 1
fi
```

---

## 🏗️ Phase 1: Backend Implementation (Go)

### Task 1.1: Fix Ruleset Selection Priority
**File**: `backend/internal/caddy/config.go`
**Function**: `buildWAFHandler`
**Estimate**: 30 minutes

```go
// Replace lines 743-748 with:
for i, r := range rulesets {
    // Priority order:
    // 1. Exact match to secCfg.WAFRulesSource (user's global choice)
    // 2. Exact match to hostRulesetName (per-host advanced_config)
    // 3. Match to host.Application (app-specific defaults)
    // 4. Fallback to owasp-crs
    if (secCfg != nil && secCfg.WAFRulesSource != "" && r.Name == secCfg.WAFRulesSource) {
        selected = &rulesets[i]
        break
    }
    if hostRulesetName != "" && r.Name == hostRulesetName {
        selected = &rulesets[i]
        break
    }
    if host != nil && r.Name == host.Application {
        selected = &rulesets[i]
        break
    }
    if r.Name == "owasp-crs" && selected == nil {
        selected = &rulesets[i]
        // Don't break - keep looking for better matches
    }
}
```

### Task 1.2: Add Validation for WAF Handler
**File**: `backend/internal/caddy/config.go`
**Function**: `buildWAFHandler`
**Estimate**: 30 minutes

Ensure the handler is only returned if it has valid directives. Log a warning otherwise.

### Task 1.3: Add Debug Logging
**File**: `backend/internal/caddy/manager.go`
**Function**: `ApplyConfig`
**Estimate**: 20 minutes

Add structured logging to capture WAF state during config generation.

### Task 1.4: Update Unit Tests
**Files**:
- `backend/internal/caddy/config_test.go`
- `backend/internal/caddy/manager_additional_test.go`

**Estimate**: 1 hour

- Test ruleset selection priority
- Test handler validation
- Test empty ruleset handling

---

## 🎨 Phase 2: Frontend (No Changes Required)

The frontend WAF configuration UI is working correctly. No changes needed.

---

## 🛠️ Phase 3: DevOps/CI Fixes

### Task 3.1: Improve Integration Test Robustness
**File**: `scripts/coraza_integration.sh`
**Estimate**: 45 minutes

```bash
#!/usr/bin/env bash
set -euo pipefail

# ... existing setup ...

# IMPROVEMENT 1: Add config verification step
verify_waf_config() {
    local retries=5
    local wait=2

    for i in $(seq 1 $retries); do
        CADDY_CONFIG=$(curl -s http://localhost:2019/config)

        if echo "$CADDY_CONFIG" | grep -q '"handler":"waf"'; then
            echo "✓ WAF handler verified in Caddy config"

            # Also verify the directives include our ruleset
            if echo "$CADDY_CONFIG" | grep -q "integration-xss"; then
                echo "✓ Ruleset 'integration-xss' found in directives"
                return 0
            fi
        fi

        echo "Waiting for config to propagate (attempt $i/$retries)..."
        sleep $wait
    done

    echo "✗ WAF handler verification failed after $retries attempts"
    echo "Caddy config dump:"
    curl -s http://localhost:2019/config | head -200
    return 1
}

# IMPROVEMENT 2: Add container log dump on failure
on_failure() {
    echo ""
    echo "=== FAILURE DEBUG INFO ==="
    echo ""
    echo "=== Charon API Logs ==="
    docker logs charon-debug 2>&1 | tail -100
    echo ""
    echo "=== Caddy Config ==="
    curl -s http://localhost:2019/config | head -200
    echo ""
    echo "=== Ruleset Files ==="
    docker exec charon-debug sh -c 'ls -la /app/data/caddy/coraza/rulesets/ 2>/dev/null' || echo "No rulesets found"
    docker exec charon-debug sh -c 'cat /app/data/caddy/coraza/rulesets/*.conf 2>/dev/null' || echo "No ruleset content"
}
trap on_failure ERR

# ... rest of test with verify_waf_config calls ...
```

### Task 3.2: Add CI Debug Output
**File**: `.github/workflows/waf-integration.yml`
**Estimate**: 20 minutes

Add step to dump Caddy config on failure for easier debugging.

---

## 🕵️ Phase 4: QA & Testing

### Manual Test Checklist

1. **Block Mode Test**
   - [ ] Create ruleset with XSS rule
   - [ ] Set WAF mode to `block`
   - [ ] Send `<script>` payload → Expect 403

2. **Monitor Mode Test**
   - [ ] Set WAF mode to `monitor`
   - [ ] Send `<script>` payload → Expect 200 (logged only)
   - [ ] Verify log entry shows WAF detection

3. **Ruleset Priority Test**
   - [ ] Create two rulesets: `test-rules` and `owasp-crs`
   - [ ] Set `waf_rules_source` to `test-rules`
   - [ ] Verify `test-rules` is used (not `owasp-crs`)

4. **Empty Ruleset Test**
   - [ ] Enable WAF with no rulesets created
   - [ ] Verify no WAF handler is added (not a broken one)

### CI Verification

After fixes are merged:
- [ ] WAF Integration workflow passes
- [ ] No regressions in main CI pipeline

---

## 📚 Phase 5: Documentation

### Task 5.1: Update Cerberus Docs
**File**: `docs/cerberus.md`

- Update WAF status from "Prototype" to "Functional"
- Document proper ruleset creation flow
- Add troubleshooting section

### Task 5.2: Add Debug Guide
**File**: `docs/debugging-waf.md` (new)

Document how to:
- Inspect Caddy config via admin API
- Check ruleset file contents
- Read WAF logs

---

## ⏱️ Timeline Estimate

| Phase | Task | Estimate |
|-------|------|----------|
| 1 | Backend fixes | 2.5 hours |
| 2 | Frontend | 0 hours |
| 3 | CI/DevOps | 1 hour |
| 4 | QA Testing | 1 hour |
| 5 | Documentation | 1 hour |
| **Total** | | **~5.5 hours** |

---

## 📎 Appendix: Technical Reference

### Coraza-Caddy Plugin Source
- Repository: https://github.com/corazawaf/coraza-caddy
- Module ID: `http.handlers.waf`
- JSON fields: `handler`, `directives`, `include` (deprecated), `load_owasp_crs`

### ModSecurity Directive Reference
- `SecRuleEngine On|Off|DetectionOnly`
- `SecRequestBodyAccess On|Off`
- `SecRule VARIABLE OPERATOR "ACTIONS"`
- `Include /path/to/file.conf`

### Charon WAF Flow
```
User creates ruleset → DB insert → ApplyConfig triggered
  ↓
manager.go writes ruleset file with hash
  ↓
config.go buildWAFHandler() creates handler with Include directive
  ↓
Handler added to securityHandlers slice
  ↓
JSON config sent to Caddy admin API
  ↓
Caddy loads coraza-caddy plugin, parses directives, creates WAF instance
```
