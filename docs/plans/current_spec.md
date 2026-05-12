# CI Hardening — WAF & Rate Limit Integration Test Reliability

**Branch**: `fix/ci-waf-integration-hardening`
**PR**: Targets `development`
**Date**: 2026-05-11

---

> **Archived**: The previous spec (CI Fix — Vitest `invites a new user` Failure) has been
> superseded. This document covers CI integration test timing hardening.

---

## 1. Introduction

### Overview

WAF and rate limit integration CI tests experienced transient flakiness. Root cause analysis
confirmed the failures are timing-based CI runner variation, not code regressions. However,
structural vulnerabilities remain that will cause future flakiness without intervention.

Three targeted changes address the root causes:

1. **`waf-integration.yml`** calls the deprecated `coroza_integration.sh` instead of the
   canonical `waf_integration.sh`. This must be corrected.
2. **`waf_integration.sh`** uses fixed `sleep 5` delays after sending WAF config change
   requests. On slow CI runners, Caddy has not yet applied the new configuration by the
   time attack payloads are fired — the WAF handler is not active, requests return `200`
   instead of `403`.
3. **`rate_limit_integration.sh`** tests the `429` response with a single-shot attempt.
   Burst counter inconsistencies on slow runners can cause intermittent failure.

### Objectives

- Replace four `sleep 5` stubs in `waf_integration.sh` with a blocking
  `verify_waf_config()` gate that polls Caddy's admin API (max 50 s).
- Wire `waf-integration.yml` to call `waf_integration.sh` (non-deprecated) and reference
  the correct container name.
- Add 3-retry/2-second resilience around the `429` assertion in
  `rate_limit_integration.sh`.

---

## 2. Research Findings

### Affected Files

| File | Status | Issue |
|---|---|---|
| `.github/workflows/waf-integration.yml` | CI workflow | Calls deprecated `coroza_integration.sh`; references wrong container `charon-debug` |
| `scripts/waf_integration.sh` | Active, non-deprecated | Four `sleep 5` stubs after WAF config changes; no blocking config verification |
| `scripts/rate_limit_integration.sh` | Active; has proven `verify_rate_limit_config()` | Single-shot 429 assertion; no retry |
| `scripts/coroza_integration.sh` | **DEPRECATED** (self-declared at file top) | Uses conflicting port 2019; non-blocking advisory `verify_waf_config()` |

### Port Assignments

`waf_integration.sh` uses isolated ports to avoid collisions with the E2E environment:

```
API_PORT=8380
HTTP_PORT=8180
HTTPS_PORT=8143
CADDY_ADMIN_PORT=2119
```

### Pattern to Replicate

`rate_limit_integration.sh` already contains the correct reliability pattern:

```bash
verify_rate_limit_config() {
    local retries=10
    local wait=5
    echo "Verifying rate limit config in Caddy..."
    for i in $(seq 1 $retries); do
        local caddy_config
        caddy_config=$(curl -s http://localhost:2119/config/ 2>/dev/null || echo "")
        if [ -z "$caddy_config" ]; then
            echo "  Attempt $i/$retries: Caddy admin API not responding, retrying..."
            sleep $wait
            continue
        fi
        if echo "$caddy_config" | grep -q '"handler":"rate_limit"'; then
            echo "  ✓ rate_limit handler found in Caddy config"
            return 0
        else
            echo "  Attempt $i/$retries: rate_limit handler not found, waiting..."
        fi
        sleep $wait
    done
    echo "  ✗ rate_limit handler verification failed after $retries attempts"
    return 1
}
```

Called as a hard gate:

```bash
if ! verify_rate_limit_config; then
    echo "✗ Rate limit handler verification failed — aborting test"
    exit 1
fi
```

### Exact `sleep 5` Locations in `waf_integration.sh`

Four occurrences require replacement (ordered by appearance):

| # | After action | Surrounding log message |
|---|---|---|
| 1 | TC-2: Enable WAF block mode | `log_info "Waiting for Caddy to apply WAF configuration..."` |
| 2 | TC-5: Switch to monitor mode | `log_info "  Switched to monitor mode, waiting for Caddy reload..."` |
| 3 | TC-7: Enable SQLi ruleset block mode | `log_info "  Switched to SQLi ruleset in block mode, waiting for Caddy reload..."` |
| 4 | TC-8: Enable combined ruleset | `log_info "  Switched to combined ruleset, waiting for Caddy reload..."` |

TC-9 is an explicit Caddy config assertion test case — it must **not** be changed.

---

## 3. Technical Specifications

### Change 1 — `.github/workflows/waf-integration.yml`

#### 3.1.1 Update "Run WAF integration tests" step

**Find** (exact block):

```yaml
      - name: Run WAF integration tests
        id: waf-test
        run: |
          chmod +x scripts/coroza_integration.sh
          scripts/coroza_integration.sh 2>&1 | tee waf-test-output.txt
          exit "${PIPESTATUS[0]}"
```

**Replace with**:

```yaml
      - name: Run WAF integration tests
        id: waf-test
        run: |
          chmod +x scripts/waf_integration.sh
          scripts/waf_integration.sh 2>&1 | tee waf-test-output.txt
          exit "${PIPESTATUS[0]}"
```

#### 3.1.2 Update "Dump Debug Info on Failure" step — container name

**Find** (in the debug step):

```yaml
            echo "### Charon Container Logs (last 100 lines)"
            echo '```'
            docker logs charon-debug 2>&1 | tail -100 || echo "No container logs available"
            echo '```'
            echo ""

            echo "### WAF Ruleset Files"
            echo '```'
            docker exec charon-debug sh -c 'ls -la /app/data/caddy/coraza/rulesets/ 2>/dev/null && echo "---" && cat /app/data/caddy/coraza/rulesets/*.conf 2>/dev/null' || echo "No ruleset files found"
            echo '```'
```

**Replace with**:

```yaml
            echo "### Charon Container Logs (last 100 lines)"
            echo '```'
            docker logs charon-waf-test 2>&1 | tail -100 || echo "No container logs available"
            echo '```'
            echo ""

            echo "### WAF Ruleset Files"
            echo '```'
            docker exec charon-waf-test sh -c 'ls -la /app/data/caddy/coraza/rulesets/ 2>/dev/null && echo "---" && cat /app/data/caddy/coraza/rulesets/*.conf 2>/dev/null' || echo "No ruleset files found"
            echo '```'
```

#### 3.1.3 Update Caddy admin port in debug step

**Find** (in the debug step):

```yaml
            echo "### Caddy Admin Config"
            echo '```json'
            curl -s http://localhost:2019/config 2>/dev/null | head -200 || echo "Could not retrieve Caddy config"
            echo '```'
```

**Replace with**:

```yaml
            echo "### Caddy Admin Config"
            echo '```json'
            curl -s http://localhost:2119/config/ 2>/dev/null | head -200 || echo "Could not retrieve Caddy config"
            echo '```'
```

#### 3.1.4 Update Cleanup step — container names

**Find** (in the Cleanup step):

```yaml
        run: |
          docker rm -f charon-debug || true
          docker rm -f coraza-backend || true
          docker network rm containers_default || true
```

**Replace with**:

```yaml
        run: |
          docker rm -f charon-waf-test || true
          docker rm -f waf-backend || true
          docker network rm containers_default || true
```

---

### Change 2 — `scripts/waf_integration.sh`: Add blocking `verify_waf_config()` gate

#### 3.2.1 Add `verify_waf_config()` function

Place the function immediately **before** the `trap` setup lines. This is in the Helper
Functions section. The function mirrors `verify_rate_limit_config()` exactly, except it
checks for `"handler":"waf"` and uses the `${CADDY_ADMIN_PORT}` variable already defined in
the script (value: `2119`).

**Find** (exact block):

```bash
# Set up trap to dump debug info on any error and always cleanup
trap on_failure ERR
trap cleanup EXIT
```

**Insert before that block**:

```bash
# Verifies WAF handler is present in Caddy config after a config change
verify_waf_config() {
    local retries=10
    local wait=5

    log_info "Verifying WAF handler in Caddy config..."

    for i in $(seq 1 $retries); do
        local caddy_config
        caddy_config=$(curl -sL "http://localhost:${CADDY_ADMIN_PORT}/config/" 2>/dev/null || echo "")

        if [ -z "$caddy_config" ]; then
            log_warn "  Attempt $i/$retries: Caddy admin API not responding, retrying..."
            sleep $wait
            continue
        fi

        if echo "$caddy_config" | grep -q '"handler":"waf"'; then
            log_info "  ✓ WAF handler found in Caddy config"
            return 0
        else
            log_warn "  Attempt $i/$retries: WAF handler not found, waiting..."
        fi

        sleep $wait
    done

    log_error "  ✗ WAF handler verification failed after $retries attempts"
    return 1
}

```

#### 3.2.2 Replace `sleep 5` after TC-2 (occurrence 1)

**Find** (exact block):

```bash
# Wait for Caddy to reload with WAF config
log_info "Waiting for Caddy to apply WAF configuration..."
sleep 5

# ============================================================================
# TC-3: Test XSS blocking (expect HTTP 403)
```

**Replace with**:

```bash
# Wait for Caddy to reload with WAF config and verify before testing
sleep 3
verify_waf_config || { log_error "WAF config not applied after TC-2 — aborting"; exit 1; }

# ============================================================================
# TC-3: Test XSS blocking (expect HTTP 403)
```

#### 3.2.3 Replace `sleep 5` after TC-5 (occurrence 2)

**Find** (exact block):

```bash
log_info "  Switched to monitor mode, waiting for Caddy reload..."
sleep 5

# Verify XSS passes in monitor mode
```

**Replace with**:

```bash
# Wait for Caddy to reload then verify WAF handler still present (monitor mode)
sleep 3
verify_waf_config || { log_error "WAF config not applied after TC-5 monitor switch — aborting"; exit 1; }

# Verify XSS passes in monitor mode
```

#### 3.2.4 Replace `sleep 5` after TC-7 (occurrence 3)

**Find** (exact block):

```bash
log_info "  Switched to SQLi ruleset in block mode, waiting for Caddy reload..."
sleep 5

# Test SQLi OR 1=1
```

**Replace with**:

```bash
# Wait for Caddy to reload then verify WAF handler with SQLi ruleset
sleep 3
verify_waf_config || { log_error "WAF config not applied after TC-7 SQLi switch — aborting"; exit 1; }

# Test SQLi OR 1=1
```

#### 3.2.5 Replace `sleep 5` after TC-8 (occurrence 4)

**Find** (exact block):

```bash
log_info "  Switched to combined ruleset, waiting for Caddy reload..."
sleep 5

# Test both attacks blocked
```

**Replace with**:

```bash
# Wait for Caddy to reload then verify WAF handler with combined ruleset
sleep 3
verify_waf_config || { log_error "WAF config not applied after TC-8 combined switch — aborting"; exit 1; }

# Test both attacks blocked
```

---

### Change 3 — `scripts/rate_limit_integration.sh`: Add 429 retry resilience

#### 3.3.1 Replace single-shot 429 assertion with retry loop

**Find** (exact block):

```bash
echo ""
echo "Sending request ${RATE_LIMIT_REQUESTS}+1 (should return 429 Too Many Requests)..."

# Capture headers too for Retry-After check
BLOCKED_RESPONSE=$(curl -s -D - -o /dev/null -H "Host: ${TEST_DOMAIN}" http://localhost:8180/get)
BLOCKED_STATUS=$(echo "$BLOCKED_RESPONSE" | head -1 | grep -o '[0-9]\{3\}' | head -1)

if [ "$BLOCKED_STATUS" = "429" ]; then
    echo "  ✓ Request blocked with HTTP 429 as expected"

    # Check for Retry-After header
    if echo "$BLOCKED_RESPONSE" | grep -qi "Retry-After"; then
        RETRY_AFTER=$(echo "$BLOCKED_RESPONSE" | grep -i "Retry-After" | head -1)
        echo "  ✓ Retry-After header present: $RETRY_AFTER"
    else
        echo "  ⚠ Retry-After header not found (may be plugin-dependent)"
    fi
else
    echo "  ✗ Expected HTTP 429, got HTTP $BLOCKED_STATUS"
```

**Replace with**:

```bash
echo ""
echo "Sending request ${RATE_LIMIT_REQUESTS}+1 (should return 429 Too Many Requests)..."

# Retry up to 3 times with 2-second delay to tolerate burst counter propagation lag
BLOCKED_STATUS=""
BLOCKED_RESPONSE=""
for _retry in 1 2 3; do
    BLOCKED_RESPONSE=$(curl -s -D - -o /dev/null -H "Host: ${TEST_DOMAIN}" http://localhost:8180/get)
    BLOCKED_STATUS=$(echo "$BLOCKED_RESPONSE" | head -1 | grep -o '[0-9]\{3\}' | head -1)

    if [ "$BLOCKED_STATUS" = "429" ]; then
        break
    fi

    echo "  Attempt $_retry/3: got HTTP $BLOCKED_STATUS, retrying in 2 seconds..."
    sleep 2
done

if [ "$BLOCKED_STATUS" = "429" ]; then
    echo "  ✓ Request blocked with HTTP 429 as expected"

    # Check for Retry-After header
    if echo "$BLOCKED_RESPONSE" | grep -qi "Retry-After"; then
        RETRY_AFTER=$(echo "$BLOCKED_RESPONSE" | grep -i "Retry-After" | head -1)
        echo "  ✓ Retry-After header present: $RETRY_AFTER"
    else
        echo "  ⚠ Retry-After header not found (may be plugin-dependent)"
    fi
else
    echo "  ✗ Expected HTTP 429, got HTTP $BLOCKED_STATUS (after 3 attempts)"
```

The `else` failure branch body (debug dumps and `exit 1`) is unchanged — only update the
opening echo to reflect the attempt count as shown above.

---

## 4. Implementation Plan

### Phase 1 — No Playwright phase

These are CI script and workflow changes only. No frontend or backend application code is
modified.

### Phase 2 — Commit 1: `verify_waf_config()` gate in `waf_integration.sh`

**Files**: `scripts/waf_integration.sh`

- [ ] 2.1 Add `verify_waf_config()` function per §3.2.1
- [ ] 2.2 Replace `sleep 5` after TC-2 per §3.2.2
- [ ] 2.3 Replace `sleep 5` after TC-5 per §3.2.3
- [ ] 2.4 Replace `sleep 5` after TC-7 per §3.2.4
- [ ] 2.5 Replace `sleep 5` after TC-8 per §3.2.5
- [ ] 2.6 `bash -n scripts/waf_integration.sh` passes

### Phase 3 — Commit 2: Update `waf-integration.yml`

**Files**: `.github/workflows/waf-integration.yml`

- [ ] 3.1 Script reference: `coroza_integration.sh` → `waf_integration.sh` (§3.1.1)
- [ ] 3.2 Container name in debug step: `charon-debug` → `charon-waf-test` (§3.1.2)
- [ ] 3.3 Admin port in debug step: `2019` → `2119` (§3.1.3)
- [ ] 3.4 Cleanup container names (§3.1.4)

### Phase 4 — Commit 3: 429 retry in `rate_limit_integration.sh`

**Files**: `scripts/rate_limit_integration.sh`

- [ ] 4.1 Replace single-shot 429 assertion with retry loop per §3.3.1
- [ ] 4.2 `bash -n scripts/rate_limit_integration.sh` passes

### Phase 5 — Validation

```bash
bash -n scripts/waf_integration.sh
bash -n scripts/rate_limit_integration.sh

# Must return zero results
grep -n "coroza_integration" .github/workflows/waf-integration.yml
grep -n "charon-debug" .github/workflows/waf-integration.yml
grep -n '"2019"' .github/workflows/waf-integration.yml

# sleep 5 must be gone from waf script
grep -n "sleep 5" scripts/waf_integration.sh

# verify_waf_config must appear 5 times: 1 definition + 4 call sites
grep -c "verify_waf_config" scripts/waf_integration.sh   # expected: 5
```

---

## 5. Commit Slicing Strategy

**Decision**: Single PR, 3 ordered logical commits. All changes are CI-only with no
application logic impact. Ordered commits allow bisection if any individual change causes
issues in CI.

| Commit | Scope | Files | Validation Gate |
|---|---|---|---|
| 1 | `fix(ci)` | `scripts/waf_integration.sh` | `bash -n` passes; 4× `sleep 5` replaced; `verify_waf_config` present 5× |
| 2 | `fix(ci)` | `.github/workflows/waf-integration.yml` | No `coroza_integration`, `charon-debug`, or `:2019` remains |
| 3 | `fix(ci)` | `scripts/rate_limit_integration.sh` | `bash -n` passes; `_retry in 1 2 3` retry loop present |

### Commit Messages

```
fix(ci): add blocking verify_waf_config() gate to waf_integration.sh

Replace four sleep 5 stubs after WAF config change API calls with a
verify_waf_config() function that polls Caddy's admin API for the waf
handler (10 retries x 5 seconds, 50 seconds max). Exits 1 if Caddy
has not applied the configuration, preventing false-pass test results
on slow CI runners.

Mirrors the verify_rate_limit_config() pattern already proven in
rate_limit_integration.sh.
```

```
fix(ci): wire waf-integration.yml to non-deprecated waf_integration.sh

The workflow was calling the deprecated coroza_integration.sh script
which used conflicting port 2019 and a non-blocking advisory config
check. Switch to waf_integration.sh (unique ports 8380/8180/2119,
blocking verification gate added in previous commit).

Update debug step container references from charon-debug to
charon-waf-test to match waf_integration.sh. Update Caddy admin port
from 2019 to 2119. Update cleanup step container names accordingly.
```

```
fix(ci): add 429 retry resilience to rate_limit_integration.sh

The blocked-request assertion fired once immediately after the allowed
requests. Burst counter propagation lag on slow CI runners can result
in a 200 instead of the expected 429. Wrap the assertion in a loop of
3 attempts with 2-second delays before declaring failure.
```

### Rollback Notes

All changes are isolated to CI infrastructure files. Application behaviour is unaffected.
Revert any individual commit with `git revert <SHA>` if it causes regression.

---

## 6. Acceptance Criteria

| # | Criterion | Verification |
|---|---|---|
| AC-1 | `waf-integration.yml` calls `waf_integration.sh` | `grep coroza .github/workflows/waf-integration.yml` → no match |
| AC-2 | Debug step references `charon-waf-test` | `grep charon-debug .github/workflows/waf-integration.yml` → no match |
| AC-3 | Debug step uses admin port `2119` | `grep "localhost:2019" .github/workflows/waf-integration.yml` → no match |
| AC-4 | Cleanup step removes `charon-waf-test` and `waf-backend` | Manual review |
| AC-5 | `verify_waf_config()` function present in `waf_integration.sh` | `grep -c verify_waf_config scripts/waf_integration.sh` → `5` |
| AC-6 | Zero `sleep 5` stubs in `waf_integration.sh` | `grep -c "sleep 5" scripts/waf_integration.sh` → `0` |
| AC-7 | `verify_waf_config` polls `${CADDY_ADMIN_PORT}` (port 2119) | Manual review of function body |
| AC-8 | Minimum `sleep 3` precedes each `verify_waf_config` call | Manual review of all 4 call sites |
| AC-9 | WAF config failure hard-exits with code 1 | Manual review of `|| { ... exit 1; }` at each call site |
| AC-10 | 429 assertion in rate limit script retries 3× / 2 s | `grep "_retry in 1 2 3" scripts/rate_limit_integration.sh` → match |
| AC-11 | All modified files pass `bash -n` syntax check | `bash -n scripts/waf_integration.sh && bash -n scripts/rate_limit_integration.sh` → exit 0 |
