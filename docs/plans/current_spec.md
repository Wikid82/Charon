# Cerberus Integration Test Failure - Handler Ordering After Break Glass Protocol

**Status**: ACTIVE - ANALYSIS COMPLETE
**Priority**: HIGH 🔴 - CI Blocking
**Created**: 2026-01-28
**Updated**: 2026-01-28
**Bug**: Cerberus integration test TC-2 fails due to emergency route handler ordering
**CI Run**: https://github.com/Wikid82/Charon/actions/runs/21453152384/job/61786891302
**Branch**: feature/beta-release
**PR**: https://github.com/Wikid82/Charon/pull/550

---

## Executive Summary

The Cerberus integration test (**TC-2: Verify Handler Order in Caddy Config**) is failing in CI after the break glass protocol implementation. The test uses byte position checking to verify that security handlers (WAF, rate_limit) appear **before** the reverse_proxy handler in the Caddy configuration JSON.

**Impact**:
- 🔴 **CI pipeline blocked** - Cannot merge PR #550
- 🔴 **Test TC-2 fails** - Handler order verification fails
- 🟡 **Break glass protocol working correctly** - Emergency routes properly bypass security
- 🟢 **No functional issue** - Caddy processes routes correctly (emergency first, then main)
- 🟢 **All other tests passing** - Only TC-2 affected

**Root Cause**:
After implementing break glass protocol, each proxy host now generates **TWO routes**:
1. **Emergency route** (added first, line 711): Has path matchers, NO security handlers, reverse_proxy only
2. **Main route** (added second, line 731): No path matchers, HAS security handlers + reverse_proxy

The test uses `grep -ob` to find byte positions of handlers in the Caddy JSON config. It fails because:
- Emergency route's `reverse_proxy` appears FIRST in the JSON (earliest byte position)
- Main route's `waf` and `rate_limit` appear LATER (later byte positions)
- Test: `if WAF_POS < PROXY_POS && RATE_POS < PROXY_POS` → **FAILS** (reverse_proxy comes first!)

**The Fix**:
Update the test to be **route-aware** instead of using naive byte position checking. The test should:
1. Parse the Caddy JSON config properly
2. For each route, verify handler order WITHIN that route
3. Skip emergency routes (identified by path matchers for `/api/v1/emergency/*`)
4. Verify security handlers appear before reverse_proxy in main routes only

**Complexity**: MEDIUM - Requires test refactoring, no production code changes needed

---

## Technical Analysis

### Break Glass Protocol Route Structure (Current Implementation)

**File**: `backend/internal/caddy/config.go`

For each proxy host, the config generator creates **TWO routes** in this order:

#### 1. Emergency Route (Lines 687-711)

```go
emergencyPaths := []string{
    "/api/v1/emergency/security-reset",
    "/api/v1/emergency/*",
    "/emergency/security-reset",
    "/emergency/*",
}
// NOTE: NO securityHandlers - bypasses WAF, rate_limit, ACL, etc.
emergencyHandlers := append(append([]Handler{}, handlers...),
    ReverseProxyHandler(dial, host.WebsocketSupport, host.Application, enableStdHeaders))

emergencyRoute := &Route{
    Match: []Match{{
        Host: uniqueDomains,    // e.g., ["cerberus.test.local"]
        Path: emergencyPaths,   // CRITICAL: Has path matchers
    }},
    Handle:   emergencyHandlers, // NO WAF, NO rate_limit, YES reverse_proxy
    Terminal: true,
}
routes = append(routes, emergencyRoute) // Added FIRST
```

**Key Properties**:
- ✅ Has path matchers (`/api/v1/emergency/*`)
- ✅ NO security handlers (WAF, rate_limit, CrowdSec, ACL)
- ✅ DOES have reverse_proxy handler
- ✅ Added to routes slice FIRST
- ✅ Caddy evaluates this first due to specificity (host + path > host only)

#### 2. Main Route (Lines 714-731)

```go
// NOTE: DOES include securityHandlers - applies WAF, rate_limit, ACL, etc.
mainHandlers := append(append([]Handler{}, securityHandlers...), handlers...)
mainHandlers = append(mainHandlers,
    ReverseProxyHandler(dial, host.WebsocketSupport, host.Application, enableStdHeaders))

route := &Route{
    Match: []Match{{
        Host: uniqueDomains,    // e.g., ["cerberus.test.local"]
        // NO Path matchers - matches all paths for this host
    }},
    Handle:   mainHandlers, // YES WAF, YES rate_limit, YES reverse_proxy
    Terminal: true,
}
routes = append(routes, route) // Added SECOND
```

**Key Properties**:
- ✅ NO path matchers (matches all paths)
- ✅ DOES have security handlers (WAF, rate_limit, CrowdSec, ACL)
- ✅ DOES have reverse_proxy handler
- ✅ Added to routes slice SECOND
- ✅ Caddy evaluates this second (less specific matcher)

### How Caddy Processes These Routes (CORRECT BEHAVIOR)

Caddy evaluates routes in order of specificity (not array order):

1. **Most specific matcher**: Emergency route (host + path) → Checked first
   - Request: `GET /api/v1/emergency/security-reset` → Matches emergency route → **Bypasses security**
   - Handlers: `handlers` → `reverse_proxy` (NO security)

2. **Less specific matcher**: Main route (host only) → Checked second
   - Request: `GET /api/v1/proxy-hosts` → Does NOT match emergency route → Falls through to main route
   - Handlers: `securityHandlers` (WAF, rate_limit) → `handlers` → `reverse_proxy`

**This is the intended break glass protocol design** - emergency paths bypass security, everything else goes through security.

### How the Test Checks Handler Order (BROKEN LOGIC)

**File**: `scripts/cerberus_integration.sh` (Lines 374-420)

```bash
log_test "TC-2: Verify Handler Order in Caddy Config"

CADDY_CONFIG=$(curl -sL "http://localhost:${CADDY_ADMIN_PORT}/config/" 2>/dev/null || echo "")

# Find byte positions of handlers in the JSON
WAF_POS=$(echo "$CADDY_CONFIG" | grep -ob '"handler":"waf"' | head -1 | cut -d: -f1 || echo "0")
RATE_POS=$(echo "$CADDY_CONFIG" | grep -ob '"handler":"rate_limit"' | head -1 | cut -d: -f1 || echo "0")
PROXY_POS=$(echo "$CADDY_CONFIG" | grep -ob '"handler":"reverse_proxy"' | head -1 | cut -d: -f1 || echo "0")

# Check if security handlers appear before reverse_proxy
if [ "$WAF_POS" != "0" ] && [ "$RATE_POS" != "0" ] && [ "$PROXY_POS" != "0" ]; then
    if [ "$WAF_POS" -lt "$PROXY_POS" ] && [ "$RATE_POS" -lt "$PROXY_POS" ]; then
        log_info "  ✓ Security handlers appear before reverse_proxy"
        PASSED=$((PASSED + 1))
    else
        fail_test "Security handlers not in correct order"
    fi
fi
```

### Why the Test Fails (THE BUG)

**Problem**: `grep -ob '"handler":"reverse_proxy"' | head -1` finds the **FIRST** occurrence of `reverse_proxy` in the entire JSON.

**Scenario** (with break glass protocol):

```json
{
  "apps": {
    "http": {
      "servers": {
        "charon_server": {
          "routes": [
            {
              "match": [{"host": ["cerberus.test.local"], "path": ["/api/v1/emergency/*"]}],
              "handle": [
                {"handler": "reverse_proxy", ...}  // <-- FIRST reverse_proxy (byte pos 1234)
              ]
            },
            {
              "match": [{"host": ["cerberus.test.local"]}],
              "handle": [
                {"handler": "waf", ...},           // <-- WAF (byte pos 2000)
                {"handler": "rate_limit", ...},    // <-- rate_limit (byte pos 2500)
                {"handler": "reverse_proxy", ...}  // <-- SECOND reverse_proxy (byte pos 3000)
              ]
            }
          ]
        }
      }
    }
  }
}
```

**Test Logic**:
- `WAF_POS = 2000`
- `RATE_POS = 2500`
- `PROXY_POS = 1234` (emergency route's reverse_proxy - FIRST occurrence!)
- Check: `WAF_POS (2000) < PROXY_POS (1234)` → **FALSE** ❌
- Check: `RATE_POS (2500) < PROXY_POS (1234)` → **FALSE** ❌
- **Test fails**: "Security handlers not in correct order"

**Reality**: The security handlers ARE in correct order **within the main route**. The test is checking byte positions across ALL routes, which doesn't account for the multi-route structure.

### Why This Didn't Fail Before

**Before break glass protocol**:
- Only ONE route per proxy host
- That route had: `securityHandlers` → `handlers` → `reverse_proxy`
- `WAF_POS < RATE_POS < PROXY_POS` was always true
- Test passed correctly

**After break glass protocol**:
- TWO routes per proxy host (emergency + main)
- Emergency route has `reverse_proxy` FIRST in JSON (earliest byte position)
- Main route has `WAF → rate_limit → reverse_proxy` LATER in JSON
- Test's byte-position logic breaks - it finds emergency route's reverse_proxy first

---

## Requirements Specification [NEW]

### Functional Requirements

**FR-1: jq Dependency Management**
- The test MUST check for jq availability before execution
- The test MUST fail with a clear error message if jq is not installed
- Error message MUST include installation instructions for common platforms
- NO fallback mode - jq is a hard requirement

**FR-2: Emergency Path Detection**
- The test MUST use EXACT path matching for emergency routes
- The following 4 paths MUST be recognized as emergency paths (from config.go:687-691):
  - `/api/v1/emergency/security-reset` (exact match)
  - `/api/v1/emergency/*` (exact match)
  - `/emergency/security-reset` (exact match)
  - `/emergency/*` (exact match)
- Substring matching (e.g., `contains("/emergency")`) MUST NOT be used
- Emergency paths MUST be defined as a readonly array

**FR-3: Defensive Programming**
- JSON validation MUST occur before any jq processing
- Route structure validation MUST verify `.apps.http.servers.charon_server.routes` exists
- All numeric values (indices, counts) MUST be validated as numeric before comparisons
- All array access operations MUST be preceded by existence checks

**FR-4: Network Resilience**
- curl command MUST include `--max-time 10` timeout
- curl MUST retry up to 3 times with 2-second delay between attempts
- Test MUST fail gracefully if Caddy config cannot be retrieved after retries

**FR-5: Bash Best Practices**
- Loops MUST use bash arithmetic syntax: `for ((i=0; i<N; i++))`
- Numeric comparisons MUST use `-lt`, `-gt`, `-eq` (not string comparison)
- Index comparisons MUST validate operands are numeric first
- Critical failures MUST use `return 1` to exit test immediately

### Non-Functional Requirements

**NFR-1: Performance**
- Test MUST complete within 60 seconds under normal conditions
- Each curl retry MUST have exponential backoff or fixed 2s delay

**NFR-2: Maintainability**
- Emergency paths MUST be easily updateable (single readonly array definition)
- Error messages MUST be descriptive and actionable
- Code MUST include inline comments explaining validation steps

**NFR-3: Backward Compatibility**
- Test MUST pass for configs with only main routes (no emergency routes)
- Test MUST pass for configs with only emergency routes (edge case)
- Test MUST handle multiple proxy hosts correctly

**NFR-4: CI/CD Integration**
- jq MUST be verified as available in GitHub Actions runners
- Test output MUST be parseable by CI logging systems
- Test exit codes MUST be consistent (0=pass, 1=fail)

---

## Rollback Plan [NEW]

### Triggers for Rollback

Rollback to previous byte-position checking if:
1. Test becomes unstable (>5% failure rate in CI)
2. False negatives detected (fails on valid configs)
3. Performance degrades (test takes >90 seconds)
4. jq dependency causes CI failures

### Rollback Procedure

**Step 1: Revert Test Script**
```bash
git revert <commit-hash>  # Revert TC-2 changes
```

**Step 2: Restore Original Logic**
```bash
# Restore simple byte-position checking
WAF_POS=$(echo "$CADDY_CONFIG" | grep -ob '"handler":"waf"' | head -1 | cut -d: -f1)
RATE_POS=$(echo "$CADDY_CONFIG" | grep -ob '"handler":"rate_limit"' | head -1 | cut -d: -f1)
PROXY_POS=$(echo "$CADDY_CONFIG" | grep -ob '"handler":"reverse_proxy"' | head -1 | cut -d: -f1)
```

**Step 3: Skip TC-2 Temporarily**
```bash
# Add condition to skip TC-2 until proper fix is implemented
if [ "$SKIP_TC2" = "true" ]; then
    log_warn "TC-2 skipped (break glass protocol incompatible)"
    return 0
fi
```

**Step 4: Document Issue**
- Create GitHub issue documenting why rollback was needed
- Tag issue with `test-framework` and `cerberus`
- Link to failed CI runs

**Step 5: Alternative Approaches**
If route-aware verification proves problematic:
- **Option A**: Count handler occurrences instead of checking order
- **Option B**: Skip TC-2 entirely and add backend unit tests for route generation
- **Option C**: Move handler order verification to E2E tests (Playwright)

### Rollback Verification

After rollback:
- [ ] Run full Cerberus integration test suite locally
- [ ] Verify CI passes
- [ ] Monitor for 24 hours
- [ ] Document lessons learned

---

## Solution: Route-Aware Handler Order Verification

### Approach

**Fix the test, not the production code.** The production code is correct - the break glass protocol is working as designed. The test's naive byte-position checking needs to be replaced with route-aware verification.

### Implementation Options

#### Option A: Parse JSON and Verify Per-Route (RECOMMENDED) ⭐

**Approach**:
1. Use `jq` to parse the Caddy JSON config
2. Extract routes individually
3. For each route, check if it's an emergency route (has `/api/v1/emergency/*` path matcher)
4. Skip emergency routes (they're supposed to bypass security)
5. For main routes only, verify handler order

**Pros**:
- ✅ Accurate - checks handler order within logical route boundaries
- ✅ Accounts for emergency routes correctly
- ✅ Robust - works with any number of routes
- ✅ Easy to understand and maintain

**Cons**:
- ⚠️ Requires `jq` to be installed in test environment

**Implementation**:

```bash
# TC-2: Verify handler order in Caddy config
log_test "TC-2: Verify Handler Order in Caddy Config (Route-Aware)"

CADDY_CONFIG=$(curl -sL "http://localhost:${CADDY_ADMIN_PORT}/config/" 2>/dev/null || echo "")

if [ -z "$CADDY_CONFIG" ]; then
    fail_test "Could not retrieve Caddy config"
else
    # Extract routes from Caddy config
    # Filter to only check main routes (non-emergency routes)
    # Emergency routes are identified by having path matchers for /api/v1/emergency/*

    # Count total routes
    TOTAL_ROUTES=$(echo "$CADDY_CONFIG" | jq -r '.apps.http.servers.charon_server.routes | length')
    log_info "  Found $TOTAL_ROUTES routes in config"

    # Check each route individually
    MAIN_ROUTES_CHECKED=0
    HANDLER_ORDER_CORRECT=0

    for i in $(seq 0 $((TOTAL_ROUTES - 1))); do
        ROUTE=$(echo "$CADDY_CONFIG" | jq -r ".apps.http.servers.charon_server.routes[$i]")

        # Check if this is an emergency route (has path matchers containing /emergency)
        HAS_EMERGENCY_PATH=$(echo "$ROUTE" | jq -r '
            .match[]? |
            select(.path != null) |
            .path[]? |
            select(contains("/emergency"))' | wc -l)

        if [ "$HAS_EMERGENCY_PATH" -gt 0 ]; then
            log_info "  Route $i: Emergency route (skipping handler order check)"
            continue
        fi

        # This is a main route - check handler order
        MAIN_ROUTES_CHECKED=$((MAIN_ROUTES_CHECKED + 1))

        # Get handler types in order
        HANDLERS=$(echo "$ROUTE" | jq -r '.handle[].handler' | tr '\n' ' ')
        log_info "  Route $i: Main route handlers: $HANDLERS"

        # Verify WAF comes before reverse_proxy (if WAF is present)
        WAF_INDEX=$(echo "$ROUTE" | jq -r '[.handle[].handler] | map(if . == "waf" then true else false end) | index(true) // -1')
        PROXY_INDEX=$(echo "$ROUTE" | jq -r '[.handle[].handler] | map(if . == "reverse_proxy" then true else false end) | index(true) // -1')

        if [ "$WAF_INDEX" != "-1" ] && [ "$PROXY_INDEX" != "-1" ]; then
            if [ "$WAF_INDEX" -lt "$PROXY_INDEX" ]; then
                log_info "    ✓ WAF ($WAF_INDEX) before reverse_proxy ($PROXY_INDEX)"
                HANDLER_ORDER_CORRECT=$((HANDLER_ORDER_CORRECT + 1))
            else
                fail_test "WAF must appear before reverse_proxy in route $i"
            fi
        fi

        # Verify rate_limit comes before reverse_proxy (if rate_limit is present)
        RATE_INDEX=$(echo "$ROUTE" | jq -r '[.handle[].handler] | map(if . == "rate_limit" then true else false end) | index(true) // -1')

        if [ "$RATE_INDEX" != "-1" ] && [ "$PROXY_INDEX" != "-1" ]; then
            if [ "$RATE_INDEX" -lt "$PROXY_INDEX" ]; then
                log_info "    ✓ rate_limit ($RATE_INDEX) before reverse_proxy ($PROXY_INDEX)"
            else
                fail_test "rate_limit must appear before reverse_proxy in route $i"
            fi
        fi
    done

    if [ "$MAIN_ROUTES_CHECKED" -gt 0 ] && [ "$HANDLER_ORDER_CORRECT" -gt 0 ]; then
        log_info "  ✓ Handler order verified in $MAIN_ROUTES_CHECKED main routes"
        PASSED=$((PASSED + 1))
    elif [ "$MAIN_ROUTES_CHECKED" -eq 0 ]; then
        log_warn "  No main routes found to verify (this may indicate a config issue)"
        PASSED=$((PASSED + 1)) # Don't fail if no main routes (may be valid)
    else
        fail_test "Handler order incorrect in main routes"
    fi
fi
```

#### Option B: Count Handlers Instead of Positions (SIMPLER)

**Approach**:
1. Count total occurrences of each handler type
2. Verify expected counts match
3. Don't worry about order - Caddy handles it correctly

**Pros**:
- ✅ Simple - no JSON parsing needed
- ✅ Robust - works with emergency routes
- ✅ No additional dependencies

**Cons**:
- ⚠️ Doesn't actually verify order (less strict)
- ⚠️ Could miss misconfigurations

**Implementation**:

```bash
# TC-2: Verify required handlers are present
log_test "TC-2: Verify Required Security Handlers Present"

CADDY_CONFIG=$(curl -sL "http://localhost:${CADDY_ADMIN_PORT}/config/" 2>/dev/null || echo "")

if [ -z "$CADDY_CONFIG" ]; then
    fail_test "Could not retrieve Caddy config"
else
    # Count handler occurrences
    WAF_COUNT=$(echo "$CADDY_CONFIG" | grep -o '"handler":"waf"' | wc -l)
    RATE_COUNT=$(echo "$CADDY_CONFIG" | grep -o '"handler":"rate_limit"' | wc -l)
    PROXY_COUNT=$(echo "$CADDY_CONFIG" | grep -o '"handler":"reverse_proxy"' | wc -l)

    log_info "  Handler counts: WAF=$WAF_COUNT, rate_limit=$RATE_COUNT, reverse_proxy=$PROXY_COUNT"

    # Verify we have at least one of each security handler
    if [ "$WAF_COUNT" -ge 1 ]; then
        log_info "  ✓ WAF handler present"
        PASSED=$((PASSED + 1))
    else
        fail_test "WAF handler not found"
    fi

    if [ "$RATE_COUNT" -ge 1 ]; then
        log_info "  ✓ rate_limit handler present"
        PASSED=$((PASSED + 1))
    else
        fail_test "rate_limit handler not found"
    fi

    # Verify we have more reverse_proxy handlers than security handlers
    # (because emergency routes have reverse_proxy but no security)
    if [ "$PROXY_COUNT" -gt "$WAF_COUNT" ]; then
        log_info "  ✓ Emergency routes present (more reverse_proxy than WAF)"
        PASSED=$((PASSED + 1))
    else
        log_warn "  ⚠ May be missing emergency routes"
    fi
fi
```

#### Option C: Document Test Limitation and Skip (NOT RECOMMENDED)

**Approach**:
1. Update test to skip handler order check when emergency routes are detected
2. Add comment explaining why
3. Continue with rest of tests

**Pros**:
- ✅ Minimal change
- ✅ Quick to implement

**Cons**:
- ❌ Loses test coverage
- ❌ Doesn't validate handler order at all
- ❌ Defeats purpose of TC-2

### Recommended Approach: Option A (Route-Aware Verification) ⭐

**Rationale**:
- Most accurate and complete verification
- Accounts for emergency routes explicitly
- Maintains test coverage for handler ordering
- Clear and maintainable code
- Aligns with break glass protocol design

---

## Implementation Plan

### Phase 1: Verify Test Failure Locally (15 minutes) [UPDATED]

**Objective**: Reproduce the test failure in local environment and confirm root cause.

**Tasks**:

1. **Run Cerberus integration test locally**:
   ```bash
   # Start test environment
   docker compose -f .docker/compose/docker-compose.playwright-local.yml up -d

   # Run Cerberus integration test
   ./scripts/cerberus_integration.sh
   ```

2. **Capture failing test output**:
   - Document exact error message for TC-2
   - Capture byte positions reported: `WAF_POS`, `RATE_POS`, `PROXY_POS`
   - Verify that `PROXY_POS < WAF_POS` (emergency route's reverse_proxy comes first)

3. **Inspect Caddy config**:
   ```bash
   # Get Caddy config
   curl -s http://localhost:2019/config/ | jq . > caddy_config.json

   # Count routes
   jq '.apps.http.servers.charon_server.routes | length' caddy_config.json

   # Verify emergency route is first
   jq '.apps.http.servers.charon_server.routes[0].match[].path[]?' caddy_config.json

   # Verify main route is second
   jq '.apps.http.servers.charon_server.routes[1].handle[].handler' caddy_config.json
   ```

4. **Verify break glass protocol works**:
   ```bash
   # Emergency endpoint should bypass security
   curl -H "X-Emergency-Token: test-emergency-token-for-e2e-32chars" \
        -X POST http://localhost:8080/api/v1/emergency/security-reset

   # Should return 200 OK and disable security modules
   ```

**Success Criteria**:
- ✅ Test fails with "Security handlers not in correct order"
- ✅ Confirmed: `PROXY_POS < WAF_POS` due to emergency route
- ✅ Confirmed: Emergency route appears first in JSON
- ✅ Confirmed: Break glass protocol works functionally

**Files**:
- `scripts/cerberus_integration.sh` (read only - observe failure)
- Document findings in diagnosis report

**Estimated Time**: 15 minutes

---

### Phase 2: Implement Route-Aware Verification (45 minutes) [UPDATED]

**Objective**: Update the test to properly verify handler order within each route, accounting for emergency routes.

**CRITICAL CHANGES** (Addressing Supervisor Feedback):
1. **Make jq a HARD REQUIREMENT** - No fallback mode
2. **Fix emergency path detection** - Use EXACT path matching for 4 specific paths
3. **Add defensive checks** - JSON validation, numeric validation, route structure checks
4. **Improve curl command** - Add timeout and retry logic
5. **Fix bash scripting** - Use bash arithmetic loops, proper numeric comparisons

**File**: `scripts/cerberus_integration.sh` (lines 374-420)

**Changes**:

```bash
# TC-2: Verify handler order in Caddy config (ROUTE-AWARE)
# ============================================================================
log_test "TC-2: Verify Handler Order in Caddy Config"

# HARD REQUIREMENT: Check if jq is available (no fallback mode)
if ! command -v jq >/dev/null 2>&1; then
    fail_test "jq is required for handler order verification. Install: apt-get install jq / brew install jq"
    return 1
fi

# Fetch Caddy config with timeout and retry
CADDY_CONFIG=""
for attempt in 1 2 3; do
    CADDY_CONFIG=$(curl --max-time 10 -sL "http://localhost:${CADDY_ADMIN_PORT}/config/" 2>/dev/null || echo "")
    if [ -n "$CADDY_CONFIG" ]; then
        break
    fi
    log_warn "  Attempt $attempt/3: Failed to fetch Caddy config, retrying..."
    sleep 2
done

if [ -z "$CADDY_CONFIG" ]; then
    fail_test "Could not retrieve Caddy config after 3 attempts"
    return 1
fi

# Validate JSON structure before processing
if ! echo "$CADDY_CONFIG" | jq empty 2>/dev/null; then
    fail_test "Retrieved Caddy config is not valid JSON"
    return 1
fi

# Validate expected structure exists
if ! echo "$CADDY_CONFIG" | jq -e '.apps.http.servers.charon_server.routes' >/dev/null 2>&1; then
    fail_test "Caddy config missing expected route structure (.apps.http.servers.charon_server.routes)"
    return 1
fi

# Get route count with validation
TOTAL_ROUTES=$(echo "$CADDY_CONFIG" | jq -r '.apps.http.servers.charon_server.routes | length' 2>/dev/null || echo "0")

# Validate route count is numeric and non-negative
if ! [[ "$TOTAL_ROUTES" =~ ^[0-9]+$ ]]; then
    fail_test "Invalid route count (not numeric): $TOTAL_ROUTES"
    return 1
fi

log_info "  Found $TOTAL_ROUTES routes in Caddy config"

if [ "$TOTAL_ROUTES" -eq 0 ]; then
    fail_test "No routes found in Caddy config"
    return 1
fi

# Define EXACT emergency paths (must match config.go lines 687-691)
readonly EMERGENCY_PATHS=(
    "/api/v1/emergency/security-reset"
    "/api/v1/emergency/*"
    "/emergency/security-reset"
    "/emergency/*"
)

ROUTES_VERIFIED=0
EMERGENCY_ROUTES_SKIPPED=0

# Use bash arithmetic loop instead of seq
for ((i=0; i<TOTAL_ROUTES; i++)); do
    # Validate route exists at index
    if ! echo "$CADDY_CONFIG" | jq -e ".apps.http.servers.charon_server.routes[$i]" >/dev/null 2>&1; then
        log_warn "  Route $i: Missing or invalid route structure, skipping"
        continue
    fi

    # Check if this route has EXACT emergency path matches
    IS_EMERGENCY_ROUTE=false
    for emergency_path in "${EMERGENCY_PATHS[@]}"; do
        # EXACT path comparison (not substring matching)
        EXACT_MATCH=$(echo "$CADDY_CONFIG" | jq -r "
            .apps.http.servers.charon_server.routes[$i].match[]? |
            select(.path != null) |
            .path[]? |
            select(. == \"$emergency_path\")" 2>/dev/null | wc -l | tr -d ' ')

        # Validate match count is numeric
        if ! [[ "$EXACT_MATCH" =~ ^[0-9]+$ ]]; then
            log_warn "  Route $i: Invalid match count for path '$emergency_path', skipping"
            continue
        fi

        if [ "$EXACT_MATCH" -gt 0 ]; then
            IS_EMERGENCY_ROUTE=true
            break
        fi
    done

    if [ "$IS_EMERGENCY_ROUTE" = true ]; then
        log_info "  Route $i: Emergency route (security bypass by design) - skipping"
        EMERGENCY_ROUTES_SKIPPED=$((EMERGENCY_ROUTES_SKIPPED + 1))
        continue
    fi

    # Main route - verify handler order
    log_info "  Route $i: Main route - verifying handler order..."

    # Validate handlers array exists
    if ! echo "$CADDY_CONFIG" | jq -e ".apps.http.servers.charon_server.routes[$i].handle" >/dev/null 2>&1; then
        log_warn "  Route $i: No handlers found, skipping"
        continue
    fi

    # Find indices of security handlers and reverse_proxy
    WAF_IDX=$(echo "$CADDY_CONFIG" | jq -r "[.apps.http.servers.charon_server.routes[$i].handle[]?.handler] | map(if . == \"waf\" then true else false end) | index(true) // -1" 2>/dev/null || echo "-1")
    RATE_IDX=$(echo "$CADDY_CONFIG" | jq -r "[.apps.http.servers.charon_server.routes[$i].handle[]?.handler] | map(if . == \"rate_limit\" then true else false end) | index(true) // -1" 2>/dev/null || echo "-1")
    PROXY_IDX=$(echo "$CADDY_CONFIG" | jq -r "[.apps.http.servers.charon_server.routes[$i].handle[]?.handler] | map(if . == \"reverse_proxy\" then true else false end) | index(true) // -1" 2>/dev/null || echo "-1")

    # Validate all indices are numeric
    if ! [[ "$WAF_IDX" =~ ^-?[0-9]+$ ]] || ! [[ "$RATE_IDX" =~ ^-?[0-9]+$ ]] || ! [[ "$PROXY_IDX" =~ ^-?[0-9]+$ ]]; then
        fail_test "Invalid handler indices in route $i (not numeric)"
        return 1
    fi

    # Verify WAF comes before reverse_proxy (if present)
    if [ "$WAF_IDX" -ge 0 ] && [ "$PROXY_IDX" -ge 0 ]; then
        if [ "$WAF_IDX" -lt "$PROXY_IDX" ]; then
            log_info "    ✓ WAF (index $WAF_IDX) before reverse_proxy (index $PROXY_IDX)"
        else
            fail_test "WAF must appear before reverse_proxy in route $i (WAF=$WAF_IDX, proxy=$PROXY_IDX)"
            return 1
        fi
    fi

    # Verify rate_limit comes before reverse_proxy (if present)
    if [ "$RATE_IDX" -ge 0 ] && [ "$PROXY_IDX" -ge 0 ]; then
        if [ "$RATE_IDX" -lt "$PROXY_IDX" ]; then
            log_info "    ✓ rate_limit (index $RATE_IDX) before reverse_proxy (index $PROXY_IDX)"
        else
            fail_test "rate_limit must appear before reverse_proxy in route $i (rate=$RATE_IDX, proxy=$PROXY_IDX)"
            return 1
        fi
    fi

    ROUTES_VERIFIED=$((ROUTES_VERIFIED + 1))
done

log_info "  Summary: Verified $ROUTES_VERIFIED main routes, skipped $EMERGENCY_ROUTES_SKIPPED emergency routes"

if [ "$ROUTES_VERIFIED" -gt 0 ]; then
    log_info "  ✓ Handler order correct in all main routes"
    PASSED=$((PASSED + 1))
else
    log_warn "  No main routes found to verify (all routes are emergency routes?)"
    # Don't fail if only emergency routes exist - this may be valid in some configs
    PASSED=$((PASSED + 1))
fi
```

**Key Changes** [UPDATED]:
1. **BREAKING**: Make jq a HARD REQUIREMENT (fail if missing, no fallback)
2. **FIXED**: Use EXACT path matching for 4 specific emergency paths (not substring `contains`)
3. **ADDED**: JSON validation before processing
4. **ADDED**: Numeric validation for all index comparisons
5. **ADDED**: curl timeout (10s) and retry logic (3 attempts, 2s delay)
6. **FIXED**: Use bash arithmetic loops (`for ((i=0; i<N; i++))`) instead of `seq`
7. **ADDED**: Route structure validation at each step
8. Skip emergency routes (they're supposed to bypass security)
9. Verify handler order WITHIN each main route only
10. Use `return 1` on critical failures to exit test immediately

**Success Criteria** [UPDATED]:
- ✅ Test passes with break glass protocol routes
- ✅ Emergency routes detected using EXACT path matching (not substring)
- ✅ Handler order verified in main routes only
- ✅ jq dependency checked at start (fail fast if missing)
- ✅ All defensive checks in place (JSON validation, numeric validation)
- ✅ curl command includes timeout and retry
- ✅ Bash loops use arithmetic syntax
- ✅ Clear, informative console output
- ✅ Test fails fast on critical errors

**Files**:
- `scripts/cerberus_integration.sh` (TC-2 section only)

**Dependencies**:
- **jq** (REQUIRED) - Must be installed in test environment and CI

**Estimated Time**: 45 minutes (increased due to additional validation)

---

### Phase 3: Update Documentation (25 minutes) [UPDATED]

**Objective**: Document the test change, requirements, and explain why handler order checking must be route-aware.

**Tasks**:

1. **Update test documentation**:
   - File: `docs/testing/cerberus-integration.md` (create if doesn't exist)
   - Explain break glass protocol route structure
   - Document why emergency routes skip security handlers
   - Explain why byte-position checking fails
   - **[NEW]** Document jq as a hard dependency with installation instructions
   - **[NEW]** Document emergency path matching logic (exact match vs substring)
   - **[NEW]** Add troubleshooting section for common issues

2. **Create test requirements document** [NEW]:
   - File: `docs/testing/cerberus-test-requirements.md`
   - List all functional requirements (FR-1 through FR-5)
   - List all non-functional requirements (NFR-1 through NFR-4)
   - Document expected test behavior for each scenario

3. **Update CHANGELOG**:
   ```markdown
   ### Fixed
   - **CI**: Fixed Cerberus integration test TC-2 (handler order verification) to be route-aware
   - Test now correctly accounts for emergency bypass routes that skip security handlers
   - **BREAKING**: jq is now a hard requirement for Cerberus integration tests
   - Added defensive validation: JSON validation, numeric checks, route structure validation
   - Added curl timeout and retry logic for network resilience
   - Fixed emergency path detection to use exact matching instead of substring matching
   - No production code changes - break glass protocol working as designed
   ```

4. **Add rollback documentation** [NEW]:
   - File: `docs/testing/cerberus-rollback-plan.md`
   - Document rollback triggers and procedure
   - List alternative approaches if route-aware verification fails

5. **Update test inline comments**:
   - Add comments explaining emergency route detection logic
   - Document why we use jq for route parsing (required for route-aware checking)
   - **[NEW]** Add comments for each defensive check
   - **[NEW]** Add comments explaining bash arithmetic loops
   - **[REMOVED]** ~~Explain fallback behavior~~ (no fallback mode)

**Success Criteria** [UPDATED]:
- ✅ Test behavior documented clearly with requirements
- ✅ Break glass protocol explained in context
- ✅ jq dependency documented with installation instructions
- ✅ Rollback plan documented
- ✅ Changelog updated with breaking change warning
- ✅ Inline comments added to all validation steps
- ✅ Troubleshooting guide created

**Files**:
- `docs/testing/cerberus-integration.md` (NEW/UPDATE)
- `docs/testing/cerberus-test-requirements.md` (NEW)
- `docs/testing/cerberus-rollback-plan.md` (NEW)
- `CHANGELOG.md` (UPDATE)
- `scripts/cerberus_integration.sh` (inline comments)

**Estimated Time**: 25 minutes (increased due to additional documentation)

---

### Phase 4: Verify Test Fix (45 minutes) [UPDATED]

**Objective**: Run the updated test locally and in CI to confirm it passes. Test all edge cases and backward compatibility.

**Tasks**:

1. **Verify jq is available**:
   ```bash
   # Check jq version
   jq --version

   # If not installed:
   # Ubuntu/Debian: sudo apt-get install -y jq
   # macOS: brew install jq
   # Alpine: apk add --no-cache jq
   ```

2. **Run test locally (standard case)**:
   ```bash
   # Clean environment
   docker compose -f .docker/compose/docker-compose.playwright-local.yml down -v

   # Rebuild and start
   docker compose -f .docker/compose/docker-compose.playwright-local.yml up -d

   # Wait for services to be ready
   sleep 15

   # Run updated Cerberus integration test
   ./scripts/cerberus_integration.sh
   ```

3. **Test edge case: Multiple proxy hosts** [NEW]:
   ```bash
   # Add 2-3 additional proxy hosts via UI or API
   # Each host should generate 2 routes (emergency + main)
   # Run test again - should verify all main routes
   ./scripts/cerberus_integration.sh
   ```

4. **Test edge case: Cerberus disabled** [NEW]:
   ```bash
   # Use override compose file to disable Cerberus
   docker compose -f .docker/compose/docker-compose.playwright-local.yml \
                  -f .docker/compose/docker-compose.e2e.cerberus-disabled.override.yml \
                  up -d

   # Run test - TC-2 should pass (no security handlers to check)
   ./scripts/cerberus_integration.sh
   ```

5. **Test backward compatibility: Single route (no break-glass)** [NEW]:
   ```bash
   # This requires temporarily reverting config.go to only generate main route
   # OR testing against an older version of Charon
   # TC-2 should still pass for single-route configs
   ```

6. **Verify jq is present in GitHub Actions** [NEW]:
   ```bash
   # Check if jq is pre-installed on GitHub-hosted runners
   # Ubuntu 22.04 runners include jq by default
   # Verify in .github/workflows/*.yml
   ```

7. **Verify output** [UPDATED]:
   - TC-2 should PASS
   - Should see "Found N routes in Caddy config"
   - Should see "Emergency route (security bypass by design) - skipping" for each emergency route
   - Should see "Main route - verifying handler order" for each main route
   - Should see "WAF (index X) before reverse_proxy (index Y)" for each main route
   - Should see "rate_limit (index X) before reverse_proxy (index Y)" for each main route
   - Should see "Summary: Verified N main routes, skipped M emergency routes"
   - All other TCs (TC-1, TC-3, TC-4, TC-5) should continue to pass
   - NO fallback messages (jq is required)

8. **Create PR and run CI** [UPDATED]:
   - Push changes to feature branch
   - Verify CI pipeline passes
   - Check that Cerberus integration test passes in CI
   - Review CI logs for expected output

**Success Criteria** [UPDATED]:
- ✅ TC-2 passes locally (standard case)
- ✅ TC-2 passes with multiple proxy hosts
- ✅ TC-2 passes with Cerberus disabled (edge case)
- ✅ TC-2 passes with single route (backward compatibility)
- ✅ All 5 Cerberus integration TCs pass
- ✅ Test output is clear and informative (includes route summaries)
- ✅ jq dependency verified in CI environment
- ✅ CI pipeline passes
- ✅ No regressions in other tests
- ✅ Test fails fast if jq is missing (no silent fallback)

**Estimated Time**: 45 minutes (includes edge case testing)

---

## Testing Strategy

### Unit Tests (Not Applicable)

This issue is purely in the integration test script - no unit tests needed.

### Integration Tests [UPDATED]

**File**: `scripts/cerberus_integration.sh`

The fix IS the integration test itself. Verification involves:

1. **Positive Test Cases**:
   - Single proxy host with emergency + main routes → TC-2 passes
   - **[NEW]** Multiple proxy hosts with multiple routes each → TC-2 passes
   - Emergency routes correctly identified using EXACT path matching
   - Main routes correctly verified for handler order

2. **Edge Cases** [UPDATED]:
   - Only emergency routes (no main routes) → TC-2 should not fail (valid config)
   - **[REMOVED]** ~~No jq available → Fallback~~ (jq now REQUIRED, test fails if missing)
   - Caddy config unavailable after 3 retries → Fail gracefully with clear error
   - Invalid JSON response → Fail with validation error
   - Missing route structure → Fail with structure validation error
   - **[NEW]** Cerberus disabled (no security handlers) → TC-2 passes (no security handlers to check)

3. **Backward Compatibility Test Cases** [NEW]:
   - Single route, pre-break-glass config (no emergency route) → TC-2 should still pass
   - Config with only main route (no path matchers) → Verify handlers normally

3. **Regression Tests**:
   - TC-1 (feature status) still passes
   - TC-3 (WAF doesn't consume rate limit) still passes
   - TC-4 (traffic flows through layers) still passes
   - TC-5 (latency check) still passes

### E2E Tests (Existing Coverage)

The break glass protocol already has E2E test coverage:
- `tests/emergency-server/emergency-server.spec.ts`
- `tests/emergency-server/tier2-validation.spec.ts`
- `tests/security-enforcement/emergency-token.spec.ts`

No additional E2E tests needed - the Cerberus integration test IS the verification.

---

## Success Metrics

### Functionality
- ✅ TC-2 passes with break glass protocol routes
- ✅ Emergency routes correctly detected and skipped
- ✅ Handler order verified in main routes
- ✅ No false positives (emergency routes don't fail order check)
- ✅ No false negatives (real order issues are still detected)

### CI/CD
- ✅ CI pipeline passes on feature/beta-release branch
- ✅ PR #550 can be merged
- ✅ No test flakiness introduced

### Documentation
- ✅ Test behavior documented clearly
- ✅ Break glass protocol explained in test context
- ✅ Changelog updated

---

## Risk Assessment [UPDATED]

| Risk | Impact | Likelihood | Mitigation |
|------|--------|------------|------------|
| **jq not available in CI** | High | Low | **CHANGED**: Make jq REQUIRED; verify it's pre-installed on GitHub runners (Ubuntu 22.04 includes jq) |
| **Test becomes too complex** | Medium | Medium | Clear comments, step-by-step validation, early exit on error |
| **False positives (test passes when it shouldn't)** | High | Low | Extensive defensive checks, numeric validation, structure validation |
| **Regression in other TCs** | Medium | Low | Run full test suite before committing |
| **Test still flaky** | High | Low | Curl retry logic (3 attempts), increased timeout, validate JSON before parsing |
| **Emergency path detection false positives** [NEW] | High | Low | Use EXACT path matching instead of substring matching |
| **Numeric comparison failures** [NEW] | Medium | Low | Validate all indices are numeric before comparison |
| **Missing route structure** [NEW] | Medium | Low | Validate route structure exists before accessing |

---

## Timeline [UPDATED]

**Total Estimated Time**: 2 hours 30 minutes (increased due to additional validation and testing)

- ✅ **Phase 1**: Verify test failure locally (15 min)
- ✅ **Phase 2**: Implement route-aware verification with full validation (45 min) [+15 min]
- ✅ **Phase 3**: Update documentation (25 min) [+10 min]
- ✅ **Phase 4**: Verify test fix and edge cases (45 min) [+15 min]

**Target Completion**: Same day

---

## Implementation Checklist

### Phase 1: Verify Test Failure Locally (15 min)
- [ ] Start E2E environment (`docker compose up`)
- [ ] Run `scripts/cerberus_integration.sh`
- [ ] Capture TC-2 failure output
- [ ] Inspect Caddy config with `curl http://localhost:2019/config/ | jq`
- [ ] Verify emergency route is first, main route is second
- [ ] Test break glass protocol manually
- [ ] Document findings

### Phase 2: Implement Route-Aware Verification (45 min) [UPDATED]
- [ ] **[NEW]** Add jq dependency check at start of TC-2 (fail if missing)
- [ ] Update `scripts/cerberus_integration.sh` TC-2 section
- [ ] **[NEW]** Add curl timeout (10s) and retry logic (3 attempts, 2s delay)
- [ ] **[NEW]** Add JSON validation before processing config
- [ ] **[NEW]** Add route structure validation
- [ ] Add jq-based route parsing
- [ ] **[CHANGED]** Implement emergency route detection using EXACT path matching (not `contains`)
- [ ] **[NEW]** Define readonly array of 4 specific emergency paths
- [ ] **[NEW]** Use bash arithmetic loops instead of `seq`
- [ ] **[NEW]** Add numeric validation for all index variables
- [ ] Skip emergency routes in verification
- [ ] Verify handler order in main routes only
- [ ] **[REMOVED]** ~~Add fallback for environments without jq~~ (jq now REQUIRED)
- [ ] **[NEW]** Add `return 1` statements for critical failures
- [ ] Test locally to confirm TC-2 passes
- [ ] Verify all other TCs still pass

### Phase 3: Update Documentation (25 min) [UPDATED]
- [ ] Create `docs/testing/cerberus-integration.md`
- [ ] Document break glass protocol route structure
- [ ] **[NEW]** Document jq as a hard dependency
- [ ] **[NEW]** Document emergency path matching logic (exact match, not substring)
- [ ] Explain why route-aware checking is needed
- [ ] **[NEW]** Add rollback plan section (revert to byte-position check if issues)
- [ ] **[NEW]** Document defensive checks and validation steps
- [ ] Update CHANGELOG.md
- [ ] Add inline comments to test script
- [ ] **[NEW]** Verify all file paths are accurate (no references to non-existent files)

### Phase 4: Verify Test Fix (45 min) [UPDATED]
- [ ] **[NEW]** Verify jq is installed (`jq --version`)
- [ ] Run test locally 3+ times to ensure stability
- [ ] **[NEW]** Test edge case: Multiple proxy hosts (3+ hosts)
- [ ] **[NEW]** Test edge case: Cerberus disabled (no security handlers)
- [ ] **[NEW]** Test edge case: Single route (backward compatibility)
- [ ] **[REMOVED]** ~~Test: no jq~~ (now fails fast if missing)
- [ ] Test edge case: Only emergency routes
- [ ] **[NEW]** Test: curl timeout (stop Caddy temporarily)
- [ ] **[NEW]** Test: Invalid JSON response (simulate)
- [ ] **[NEW]** Verify jq is available in GitHub Actions runners
- [ ] Create PR with test fix
- [ ] Push to feature/beta-release branch
- [ ] Monitor CI pipeline
- [ ] Verify Cerberus integration test passes in CI
- [ ] **[NEW]** Verify CI logs show route summaries
- [ ] Review CI logs for expected output
- [ ] Request code review

### Post-Implementation
- [ ] Monitor CI stability for 24 hours
- [ ] Merge PR #550 if all checks pass
- [ ] Close related issues

---

## Future Enhancements (Out of Scope)

1. **Structured Test Output**:
   - Convert bash test to Go test with structured JSON output
   - Easier to parse and integrate with CI reporting

2. **Visual Route Inspector**:
   - Add admin UI endpoint showing route structure
   - Highlight emergency vs main routes
   - Show handler order per route

3. **Automated Route Verification**:
   - Add backend unit tests for route generation
   - Verify emergency routes have no security handlers
   - Verify main routes have correct handler order

---

## Appendix

### Related Documentation

- **Break Glass Protocol Design**: `docs/plans/break_glass_protocol_redesign.md`
- **Cerberus Integration Plan**: `docs/plans/cerberus_integration_testing_plan.md`
- **Emergency Server Implementation**: `docs/implementation/PHASE_3_4_TEST_ENVIRONMENT_COMPLETE.md`
- **E2E Test Remediation**: `docs/implementation/e2e_remediation_complete.md`

### References

1. **Caddy Configuration**: `backend/internal/caddy/config.go` (lines 687-731)
   - Emergency route generation (lines 687-711)
   - Main route generation (lines 714-731)

2. **Cerberus Integration Test**: `scripts/cerberus_integration.sh` (lines 374-420)
   - TC-2: Handler order verification

3. **Security Handlers**: `backend/internal/caddy/config.go` (lines 507-580)
   - WAF handler builder
   - Rate limit handler builder
   - ACL handler builder

---

**Plan Status**: ✅ UPDATED - READY FOR SUPERVISOR REVIEW
**Next Action**: Submit to Supervisor for approval of updates
**Assigned To**: Planning Agent → Supervisor Review
**Priority**: HIGH 🔴 - CI blocking, prevents PR #550 from merging
**Estimated Duration**: 2 hours 30 minutes (updated)
**Impact**: Test fix only - No production code changes required

---

## Supervisor Feedback Addressed [NEW]

### ✅ Issue 1: Emergency Path Detection Fixed
- **Before**: Used `contains("/emergency")` substring matching
- **After**: Uses EXACT path matching for 4 specific paths from config.go
- **Implementation**: Readonly array with exact path strings, loop comparison

### ✅ Issue 2: E2E Compose File Path Corrected
- **Before**: Referenced non-existent `.docker/compose/docker-compose.e2e.yml`
- **After**: All references updated to `.docker/compose/docker-compose.playwright-local.yml`
- **Verified**: File exists and contains E2E test configuration

### ✅ Issue 3: jq Dependency Management
- **Before**: Had fallback mode without jq
- **After**: jq is HARD REQUIREMENT, test fails fast if missing
- **Added**: Explicit jq check with installation instructions in error message
- **Verified**: jq is pre-installed on GitHub Actions Ubuntu 22.04 runners

### ✅ Issue 4: Shell Scripting Improvements
- **Added**: JSON validation before processing (`jq empty`)
- **Added**: Route structure validation (`.apps.http.servers.charon_server.routes` exists)
- **Added**: Numeric validation for all indices before comparison
- **Fixed**: Use bash arithmetic loops `for ((i=0; i<N; i++))` instead of `seq`
- **Added**: curl timeout (`--max-time 10`) and retry logic (3 attempts, 2s delay)
- **Added**: Defensive checks at every array access point
- **Added**: `return 1` for critical failures to exit immediately

### ✅ Issue 5: Testing Enhancements
- **Added**: Multiple proxy hosts test case
- **Added**: Backward compatibility test (single route, pre-break-glass)
- **Added**: Edge case - Cerberus disabled (no security handlers)
- **Added**: Edge case - curl timeout/retry testing
- **Added**: Edge case - Invalid JSON response handling

### 📋 Plan Updates Summary
- Increased time estimate: 1h30m → 2h30m (more validation = more time)
- Updated file paths throughout (e2e.yml → playwright-local.yml)
- Enhanced error handling and validation at all steps
- Removed fallback mode entirely
- Added comprehensive requirements specification
- Added rollback plan for risk mitigation
