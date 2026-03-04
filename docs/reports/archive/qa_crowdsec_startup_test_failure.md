# QA Report: CrowdSec Startup Integration Test Failure

**Date:** December 15, 2025
**Agent:** QA_Security
**Status:** ❌ **TEST FAILURE - ROOT CAUSE IDENTIFIED**
**Severity:** Medium (Test configuration issue, not a product defect)

---

## Executive Summary

The CrowdSec startup integration test (`scripts/crowdsec_startup_test.sh`) is **failing by design**, not due to a bug. The test expects CrowdSec LAPI to be available on port 8085, but CrowdSec is intentionally **not auto-started** in the current architecture. The system uses **GUI-controlled lifecycle management** instead of environment variable-based auto-start.

**Test Failure:**

```
✗ FAIL: LAPI health check failed (port 8085 not responding)
```

**Root Cause:** The test script sets `CERBERUS_SECURITY_CROWDSEC_MODE=local`, expecting CrowdSec to auto-start during container initialization. However, this behavior was **intentionally removed** in favor of GUI toggle control.

---

## Root Cause Analysis

### 1. Architecture Change: Environment Variables → GUI Control

**File:** [docker-entrypoint.sh](../../docker-entrypoint.sh#L110-L126)

```bash
# CrowdSec Lifecycle Management:
# CrowdSec configuration is initialized above (symlinks, directories, hub updates)
# However, the CrowdSec agent is NOT auto-started in the entrypoint.
# Instead, CrowdSec lifecycle is managed by the backend handlers via GUI controls.
```

**Design Decision:**

- ✅ **Configuration is initialized** during startup
- ❌ **Process is NOT started** until GUI toggle is used
- 🎯 **Rationale:** Consistent UX with other security features

### 2. Environment Variable Mismatch

Test uses: `CERBERUS_SECURITY_CROWDSEC_MODE`
Entrypoint checks: `SECURITY_CROWDSEC_MODE`

**Impact:** Hub items not installed during test initialization.

### 3. Reconciliation Function Does Not Auto-Start for Fresh Containers

For a **fresh container** (empty database):

- ❌ No `SecurityConfig` record exists
- ❌ No `Settings` record exists
- 🎯 **Result:** Reconciliation creates default config with `CrowdSecMode = "disabled"`

---

## Summary of Actionable Remediation Steps

### Immediate (Fix Test Failure)

**Priority: P0 (Blocks CI/CD)**

1. **Update Test Environment Variable** (`scripts/crowdsec_startup_test.sh:124`)

   ```bash
   # Change from:
   -e CERBERUS_SECURITY_CROWDSEC_MODE=local \
   # To:
   -e SECURITY_CROWDSEC_MODE=local \
   ```

2. **Add Database Seeding to Test** (after container start, before checks)

   ```bash
   # Pre-seed database to trigger reconciliation
   docker exec ${CONTAINER_NAME} sqlite3 /app/data/charon.db \
       "INSERT INTO settings (key, value, category, type) VALUES ('security.crowdsec.enabled', 'true', 'security', 'bool');"

   # Restart container to trigger reconciliation
   docker restart ${CONTAINER_NAME}
   sleep 30  # Wait for CrowdSec to start via reconciliation
   ```

3. **Fix Bash Integer Comparisons** (lines 152, 221, 247)

   ```bash
   FATAL_ERROR_COUNT=${FATAL_ERROR_COUNT:-0}
   if [ "$FATAL_ERROR_COUNT" -ge 1 ] 2>/dev/null; then
   ```

---

**Report Prepared By:** QA_Security Agent
**Date:** December 15, 2025
