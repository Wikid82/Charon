# Integration Workflow Fix Plan: go-httpbin Port Mismatch & curl-in-Container Remediation

**Status:** Active
**Created:** 2026-03-14
**Branch:** `feature/beta-release`
**Context:** All four integration workflows (cerberus, crowdsec, rate-limit, waf) are failing with "httpbin not starting." Root cause is a compounding port mismatch introduced when `kennethreitz/httpbin` (port 80) was swapped for `mccutchen/go-httpbin` (port 8080) without updating port configuration. A secondary issue exists where two scripts still use `curl` via `docker exec` inside an Alpine container that only has `wget`.
**Previous Plan:** Backed up to `docs/plans/cve_remediation_spec.md`

---

## 1. Root Cause Analysis

### 1.1 Primary: go-httpbin Port Mismatch

**Commit `042c5ec6`** replaced `kennethreitz/httpbin` with `mccutchen/go-httpbin` across all four integration scripts. However, it **only changed the image name** — no port configuration was updated.

| Property | `kennethreitz/httpbin` | `mccutchen/go-httpbin` |
|----------|----------------------|----------------------|
| Default listen port | **80** | **8080** |
| Port env var | N/A | `PORT` |
| Exposed port (Dockerfile) | `80/tcp` | `8080/tcp` |

**Evidence:** `docker inspect mccutchen/go-httpbin --format='{{json .Config.ExposedPorts}}'` returns `{"8080/tcp":{}}`. The [go-httpbin README](https://github.com/mccutchen/go-httpbin) confirms the default port is `8080`, configurable via `-port` flag or `PORT` environment variable.

**Failure mechanism:** Each integration script has a health check loop like:

```bash
for i in {1..45}; do
    if docker exec ${CONTAINER_NAME} sh -c "wget -qO /dev/null http://${BACKEND_CONTAINER}/get" >/dev/null 2>&1; then
        echo "✓ httpbin backend is ready"
        break
    fi
    if [ $i -eq 45 ]; then
        echo "✗ httpbin backend failed to start"
        exit 1
    fi
    sleep 1
done
```

The `wget` URL `http://${BACKEND_CONTAINER}/get` resolves to port 80 (HTTP default). go-httpbin is listening on port 8080. The connection is refused on port 80 for 45 iterations, then the script prints "httpbin backend failed to start" and exits 1.

Additionally, all four scripts create proxy hosts with `"forward_port": 80`, which would also fail at the Caddy reverse proxy level even if the health check passed.

### 1.2 Secondary: curl Not Available Inside Container

**Commit `58b087bc`** correctly migrated the four httpbin health checks from `curl` to `wget` (busybox). However, two other scripts still use `docker exec ... curl` to probe services inside the Alpine container, which does not have `curl` installed (removed as part of CVE remediation):

1. **`scripts/crowdsec_startup_test.sh` L179** — LAPI health check uses `docker exec ${CONTAINER_NAME} curl -sf http://127.0.0.1:8085/health`. Always returns "FAILED" (soft-pass, not blocking CI, but wrong).
2. **`scripts/diagnose-test-env.sh` L104** — CrowdSec LAPI check uses `docker exec charon-e2e curl -sf http://localhost:8090/health`. Always fails silently.

### 1.3 Timeline of Changes

| Commit | Change | Effect |
|--------|--------|--------|
| `042c5ec6` | Swap `kennethreitz/httpbin` → `mccutchen/go-httpbin` | **Introduced port mismatch** (8080 vs 80). Not caught because the prior curl-based health checks were already broken by the missing curl. |
| `58b087bc` | Replace `curl` with `wget` in `docker exec` httpbin health checks | **Correct tool fix** for 4 scripts. But exposed the hidden port mismatch — now wget correctly tries port 80 and correctly fails (connection refused), producing the visible "httpbin not starting" error. |
| `4b896c2e` | Replace `curl` with `wget` in Docker compose healthchecks | Correct, no issues. |

**Key insight:** The curl→wget migration was a correct fix for a real problem. It was applied on top of an earlier, unnoticed bug (port mismatch from the image swap). The wget migration made the port bug visible because wget actually runs inside the container, whereas curl was silently failing (not found) and the health check was timing out for a different reason.

---

## 2. Impact Assessment

### 2.1 Affected Scripts — Port Mismatch (PRIMARY)

| File | Docker Run Line | Health Check Line | Forward Port Line | Status |
|------|----------------|-------------------|-------------------|--------|
| `scripts/cerberus_integration.sh` | L174 | L215 | L255 | **BROKEN** |
| `scripts/waf_integration.sh` | L167 | L206 | L246 | **BROKEN** |
| `scripts/rate_limit_integration.sh` | L188 | L191 | L231 | **BROKEN** |
| `scripts/coraza_integration.sh` | L159 | L163 | L191 | **BROKEN** |

### 2.2 Affected Scripts — curl Inside Container (SECONDARY)

| File | Line | Command | Status |
|------|------|---------|--------|
| `scripts/crowdsec_startup_test.sh` | L179 | `docker exec ${CONTAINER_NAME} curl -sf http://127.0.0.1:8085/health` | **SILENT FAIL** |
| `scripts/diagnose-test-env.sh` | L104 | `docker exec charon-e2e curl -sf http://localhost:8090/health` | **SILENT FAIL** |

### 2.3 NOT Affected

| Component | Reason |
|-----------|--------|
| Dockerfile HEALTHCHECK | Already uses `wget` — targets Charon API `:8080`, not httpbin |
| `.docker/docker-entrypoint.sh` | Already uses `wget` — Caddy readiness on `:2019` |
| All `docker-compose` healthchecks | Already use `wget` |
| Workflow YAML files | Use host-side `curl` (installed on `ubuntu-latest`), not `docker exec` |
| `scripts/crowdsec_integration.sh` | Uses only host-side `curl`; does not use httpbin at all |
| `scripts/integration-test.sh` | Uses `whoami` image (port 80), not go-httpbin |

**File:** `backend/internal/api/routes/routes.go` (lines 260-267)

## 3. Remediation Plan

### 3.1 Fix Strategy: Add `-e PORT=80` to go-httpbin Container

The **least invasive** fix is to set the `PORT` environment variable on the go-httpbin container so it listens on port 80, matching all existing health check URLs and proxy host `forward_port` values. This avoids cascading changes to URLs and Caddy configurations.

**Alternative considered:** Change all health check URLs and `forward_port` values to 8080. Rejected because:
- Requires more changes (health check URLs, forward_port, potentially Caddy route expectations)
- The proxy host `forward_port` is the user-facing API field; port 80 is the natural default for HTTP backends

### 3.2 Exact Changes — Port Fix (4 files)

#### `scripts/cerberus_integration.sh` — Line 174

```diff
-docker run -d --name ${BACKEND_CONTAINER} --network containers_default mccutchen/go-httpbin
+docker run -d --name ${BACKEND_CONTAINER} --network containers_default -e PORT=80 mccutchen/go-httpbin
```

#### `scripts/waf_integration.sh` — Line 167

```diff
-docker run -d --name ${BACKEND_CONTAINER} --network containers_default mccutchen/go-httpbin
+docker run -d --name ${BACKEND_CONTAINER} --network containers_default -e PORT=80 mccutchen/go-httpbin
```

#### `scripts/rate_limit_integration.sh` — Line 188

```diff
-docker run -d --name ${BACKEND_CONTAINER} --network containers_default mccutchen/go-httpbin
+docker run -d --name ${BACKEND_CONTAINER} --network containers_default -e PORT=80 mccutchen/go-httpbin
```

#### `scripts/coraza_integration.sh` — Line 159

```diff
-docker run -d --name coraza-backend --network containers_default mccutchen/go-httpbin
+docker run -d --name coraza-backend --network containers_default -e PORT=80 mccutchen/go-httpbin
```

### 3.3 Exact Changes — curl→wget Inside Container (2 files)

#### `scripts/crowdsec_startup_test.sh` — Line 179

```diff
-LAPI_HEALTH=$(docker exec ${CONTAINER_NAME} curl -sf http://127.0.0.1:8085/health 2>/dev/null || echo "FAILED")
+LAPI_HEALTH=$(docker exec ${CONTAINER_NAME} wget -qO - http://127.0.0.1:8085/health 2>/dev/null || echo "FAILED")
```

Note: Using `wget -qO -` (output to stdout) instead of `wget -qO /dev/null` because the response body is captured in `$LAPI_HEALTH` and checked.

#### `scripts/diagnose-test-env.sh` — Line 104

```diff
-if docker exec charon-e2e curl -sf http://localhost:8090/health > /dev/null 2>&1; then
+if docker exec charon-e2e wget -qO /dev/null http://localhost:8090/health 2>/dev/null; then
```

---

## 4. wget vs curl Flag Mapping Reference

| curl Flag | wget Equivalent | Meaning |
|-----------|----------------|---------|
| `-s` (silent) | `-q` (quiet) | Suppress progress output |
| `-f` (fail silently on HTTP errors) | *(default behavior)* | wget exits nonzero on HTTP 4xx/5xx |
| `-o FILE` (output to file) | `-O FILE` | Write output to specified file |
| `-o /dev/null` | `-O /dev/null` | Discard response body |
| `> /dev/null 2>&1` | `2>/dev/null` | Suppress stderr (wget is quiet with `-q`, only stderr needs redirect) |
| `-m SECS` / `--max-time` | `-T SECS` | Connection timeout |
| `--retry N` | `-t N+1` | Total attempts (wget counts initial try) |
| `-S` (show error) | *(no equivalent)* | wget shows errors by default without `-q` |
| `-L` (follow redirects) | *(default behavior)* | wget follows redirects by default |

    if ip.IsLoopback() {
        return true
    }

## 5. Implementation Plan

### Phase 1: Playwright Tests (Verification of Expected Behavior)

No new Playwright tests needed. The integration scripts are bash-based CI workflows, not UI features. Existing workflow YAML files already serve as the test harness.

### Phase 2: Backend/Script Implementation

| Task | File | Change | Complexity |
|------|------|--------|------------|
| T1 | `scripts/cerberus_integration.sh` | Add `-e PORT=80` to docker run | Trivial |
| T2 | `scripts/waf_integration.sh` | Add `-e PORT=80` to docker run | Trivial |
| T3 | `scripts/rate_limit_integration.sh` | Add `-e PORT=80` to docker run | Trivial |
| T4 | `scripts/coraza_integration.sh` | Add `-e PORT=80` to docker run | Trivial |
| T5 | `scripts/crowdsec_startup_test.sh` | Replace `curl -sf` with `wget -qO -` | Trivial |
| T6 | `scripts/diagnose-test-env.sh` | Replace `curl -sf` with `wget -qO /dev/null` | Trivial |

### Phase 3: Frontend Implementation

N/A — no frontend changes required.

### Phase 4: Integration and Testing

1. **Local validation:** Run each integration script individually against a locally built `charon:local` image
2. **CI validation:** Push branch and verify all four integration workflows pass:
   - `.github/workflows/cerberus-integration.yml`
   - `.github/workflows/waf-integration.yml`
   - `.github/workflows/rate-limit-integration.yml`
   - `.github/workflows/crowdsec-integration.yml`
3. **Regression check:** Confirm E2E Playwright tests still pass (they don't touch these scripts, but verify no accidental breakage)

### Phase 5: Documentation and Deployment

No documentation changes needed. The fix is internal to CI scripts.

---

## 6. File Change Summary

| File | Change | Lines |
|------|--------|-------|
| `scripts/cerberus_integration.sh` | Add `-e PORT=80` to go-httpbin `docker run` | L174 |
| `scripts/waf_integration.sh` | Add `-e PORT=80` to go-httpbin `docker run` | L167 |
| `scripts/rate_limit_integration.sh` | Add `-e PORT=80` to go-httpbin `docker run` | L188 |
| `scripts/coraza_integration.sh` | Add `-e PORT=80` to go-httpbin `docker run` | L159 |
| `scripts/crowdsec_startup_test.sh` | Replace `curl -sf` with `wget -qO -` in docker exec | L179 |
| `scripts/diagnose-test-env.sh` | Replace `curl -sf` with `wget -qO /dev/null` in docker exec | L104 |

**Total: 6 one-line changes across 6 files.**

| Test Name | Host | Scheme | Expected Secure | Expected SameSite |
|-----------|------|--------|-----------------|--------------------|
| `TestSetSecureCookie_HTTP_PrivateIP_Insecure` | `192.168.1.50` | `http` | `false` | `Lax` |
| `TestSetSecureCookie_HTTP_10Network_Insecure` | `10.0.0.5` | `http` | `false` | `Lax` |
| `TestSetSecureCookie_HTTP_172Network_Insecure` | `172.16.0.1` | `http` | `false` | `Lax` |
| `TestSetSecureCookie_HTTPS_PrivateIP_Secure` | `192.168.1.50` | `https` | `true` | `Strict` |
| `TestSetSecureCookie_HTTP_PublicIP_Secure` | `203.0.113.5` | `http` | `true` | `Lax` |

## 7. Commit Slicing Strategy

### Decision: Single PR

**Reasoning:**
- All 6 changes are trivial one-line fixes
- Total diff is ~12 lines (6 deletions + 6 additions)
- All changes are logically related (integration test infrastructure fix)
- No cross-domain concerns (all bash scripts in `scripts/`)
- Review size is well under the 200-line threshold for splitting
- No risk of partial deployment causing issues

### PR-1: Fix integration workflow httpbin startup and curl-in-container issues

**Scope:** All 6 file changes
**Files:** `scripts/{cerberus,waf,rate_limit,coraza}_integration.sh`, `scripts/crowdsec_startup_test.sh`, `scripts/diagnose-test-env.sh`
**Dependencies:** None
**Validation gate:** All four integration workflows pass in CI

**Suggested commit message:**
```
fix(ci): add PORT=80 to go-httpbin containers and replace curl with wget in docker exec

mccutchen/go-httpbin defaults to port 8080, but all integration scripts
expected port 80 from the previous kennethreitz/httpbin image. Add
-e PORT=80 to the docker run commands in cerberus, waf, rate_limit,
and coraza integration scripts.

Also replace remaining docker exec curl commands with wget in
crowdsec_startup_test.sh and diagnose-test-env.sh, since curl is not
installed in the Alpine runtime container.

Fixes: httpbin health check timeout ("httpbin not starting") in all
four integration workflows.
```

**Rollback:** `git revert <commit>` — safe and instant. No database migrations, no API changes, no user-facing impact.

---

## 8. Supporting Files Review

| File | Review Result | Action Needed |
|------|--------------|---------------|
| `.gitignore` | No changes needed — no new files or artifacts | None |
| `codecov.yml` | No changes needed — integration scripts are not coverage-tracked | None |
| `.dockerignore` | No changes needed — scripts are not copied into Docker image | None |
| `Dockerfile` | Already correct — HEALTHCHECK uses wget, no httpbin references | None |
| `.docker/docker-entrypoint.sh` | Already correct — uses wget for Caddy readiness | None |
| `docker-compose*.yml` | Already correct — all healthchecks use wget | None |
| Workflow YAML files | Already correct — use host-side curl for debug dumps | None |

---

## 9. Acceptance Criteria

- [ ] `scripts/cerberus_integration.sh` starts go-httpbin with `-e PORT=80`
- [ ] `scripts/waf_integration.sh` starts go-httpbin with `-e PORT=80`
- [ ] `scripts/rate_limit_integration.sh` starts go-httpbin with `-e PORT=80`
- [ ] `scripts/coraza_integration.sh` starts go-httpbin with `-e PORT=80`
- [ ] `scripts/crowdsec_startup_test.sh` uses `wget` instead of `curl` for LAPI health check
- [ ] `scripts/diagnose-test-env.sh` uses `wget` instead of `curl` for CrowdSec LAPI check
- [ ] CI workflow `cerberus-integration` passes
- [ ] CI workflow `waf-integration` passes
- [ ] CI workflow `rate-limit-integration` passes
- [ ] CI workflow `crowdsec-integration` passes
- [ ] No regression in E2E Playwright tests
- [ ] `grep -r "docker exec.*curl" scripts/` returns zero matches (excluding comments/echo hints)
