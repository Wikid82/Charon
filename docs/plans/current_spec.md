# PR #1026 Cerberus Integration CI Failure Fix — Proxy Groups

**Status**: Active
**Target**: PR #1026 (`development` → `main`)

---

## 1. Introduction

### Overview

The Cerberus Integration CI workflow (`.github/workflows/cerberus-integration.yml`) fails on PR #1026 with TC-3 ("Not all malicious requests were blocked by WAF") and TC-4 ("Too many legitimate requests failed"). TC-1, TC-2, and TC-5 pass.

The root cause is a **silent HTTP 500** on proxy host creation at `13:10:30.980Z` (248 ms after API ready). The test script (`scripts/cerberus_integration.sh`) treats any non-201 response as "Proxy host may already exist" and continues without the proxy host in the database. As a result, the security configuration is applied to Caddy with **zero proxy hosts**, producing no `reverse_proxy` route for the test domain. All test traffic reaches a fallback handler that returns the React SPA (`<!DOCTYPE html>`), which the WAF and rate limiter never see.

The **exact Caddy rejection reason is unknown** due to two compounding observability gaps: the script discards the HTTP response body (containing the full error), and the CI debug step runs after `trap cleanup EXIT` has already removed all containers. This plan fixes the observability gap first, then addresses every confirmed defect.

A secondary defect also exists: even if proxy host creation succeeded, `buildWAFHandler` in `config.go` returns `nil` when `secCfg.WAFMode == "disabled"` (the seeded DB value), so the WAF handler would not be present in the Caddy route during the initial proxy host creation. The security config PUT (step 5) sets `WAFMode = "block"` and triggers a second `ApplyConfig`, at which point the WAF would apply correctly. This ordering issue is a separate concern from the 500 and is documented below.

A tertiary defect is stale named Docker volumes: the cleanup function removes containers but not volumes. Stale `charon.db` data from a prior run persists across CI runs and can leave the database in a partially configured state at startup.

### Objectives

1. Fix the observability gap so the next CI run exposes the exact Caddy rejection reason.
2. Fix the script's silent failure on any non-201 proxy host creation response.
3. Fix the CI workflow debug step ordering (containers are removed before logs are captured).
4. Eliminate stale volume state between CI runs.
5. Fix the `buildWAFHandler` / DB-seed interaction so WAF applies from the first `ApplyConfig`.
6. Add a mutex to `ApplyConfig` to prevent concurrent invocations.
7. Fix the CLI `migrate` subcommand missing `ProxyGroup` in the AutoMigrate list.

---

## 2. Research Findings

### 2.1 CI Failure Chain

Confirmed from `.github/logs/ci_failure.log`:

| Time (UTC) | Event | Status |
|---|---|---|
| 13:10:15.390Z | Charon container started | — |
| 13:10:30.732Z | Charon API health check passed | PASS |
| 13:10:30.960Z | Authentication complete | PASS |
| 13:10:30.980Z | `POST /api/v1/proxy-hosts` | **HTTP 500** |
| 13:10:33.982Z | XSS WAF ruleset created (`POST /api/v1/security/waf/rulesets`) | HTTP 201 |
| 13:10:34.011Z | Security config applied (`PUT /api/v1/security/config`) | HTTP 200 |
| 13:10:39.012Z | TC-1: Cerberus features enabled | PASS |
| 13:10:39.012Z | TC-2: Handler order in Caddy config (1 route) | PASS |
| 13:10:39.013Z | TC-3: WAF blocks malicious requests (0/3 blocked) | **FAIL** |
| 13:10:39.013Z | TC-4: Legitimate traffic flows to httpbin (all return `<!DOCTYPE html>`) | **FAIL** |

### 2.2 Defect 1 — Script: Silent Failure on Non-201 (CRITICAL)

**File**: `scripts/cerberus_integration.sh`, lines 261–272

```bash
CREATE_RESP=$(curl -s -w "\n%{http_code}" -X POST ... -d "${PROXY_HOST_PAYLOAD}" ...)
CREATE_STATUS=$(echo "$CREATE_RESP" | tail -n1)
if [ "$CREATE_STATUS" = "201" ]; then
    log_info "Proxy host created successfully"
else
    log_info "Proxy host may already exist (status: $CREATE_STATUS)"   # silent continue
fi
sleep 3
```

- Any non-201 status (including 500) is logged as "may already exist" — **no exit, no body extraction**.
- The response body containing `"Failed to apply configuration: apply failed (rolled back): <caddy_error>"` is **never read or logged**.
- Script continues to steps 5 and 6 with an empty proxy host table.

### 2.3 Defect 2 — Script: Stale Named Volumes (CRITICAL)

The `cleanup()` function and the per-run `docker rm -f` commands remove containers but **never remove named volumes**:

```bash
cleanup() {
    docker rm -f ${CONTAINER_NAME} 2>/dev/null || true
    docker rm -f ${BACKEND_CONTAINER} 2>/dev/null || true
    # volumes charon_cerberus_test_data, caddy_cerberus_test_data, caddy_cerberus_test_config
    # are NOT removed
}
```

On repeated CI runs the volumes persist, leaving `charon.db` with proxy hosts, WAF rulesets, and security config from the prior run. On a stale-volume start, `SeedDefaultSecurityConfig` uses `FirstOrCreate` and preserves old values rather than re-seeding; a previously committed `WAFMode = "block"` or `Enabled = true` survives. Each run must start from a known-good empty state.

### 2.4 Defect 3 — CI Workflow: Debug Step After Container Cleanup (CRITICAL)

**File**: `.github/workflows/cerberus-integration.yml`

The "Dump Debug Info on Failure" step executes `docker logs charon-cerberus-test`, but `trap cleanup EXIT` has already run inside the script and removed the container. Confirmed in the CI log at `13:11:49Z`: `Error: No such container: charon-cerberus-test`.

### 2.5 Root Cause of HTTP 500: Observability Gap

The `Create` handler (`proxy_host_handler.go`) returns HTTP 500 when `ApplyConfig` fails:

```go
c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to apply configuration: " + err.Error()})
```

The `err.Error()` string is one of:
- `"apply failed (rolled back): <caddy_error>"` — Caddy's `/load` API rejected the config.
- `"save snapshot: <io_error>"` — disk error writing the config snapshot.
- `"validate config: <validation_error>"` — local validation of generated config failed.

The exact value is unknown because the script discards the response body. Likely candidates given the CI environment:
- **Stale Caddy data** in the named volumes causing `/load` to reject the config.
- A **Caddy JSON validation error** specific to the generated config for `cerberus.test.local` in non-E2E mode (ACME automation policy for a `.local` domain, rate-limit or WAF handler shape mismatch, etc.).
- A **concurrent `ApplyConfig` collision** between the background startup goroutine and the request handler (no mutex).

Fixes 1, 2, and 3 (observability + volume cleanup) will reveal the exact error on the next CI run.

### 2.6 Defect 4 — buildWAFHandler / DB-Seed Interaction (SECONDARY)

`SeedDefaultSecurityConfig` (`backend/internal/models/seed.go`) creates the "default" `SecurityConfig` row with `Enabled: false` and `WAFMode: "disabled"`. At proxy host creation time, `computeEffectiveFlags` reads this DB record and sets `wafEnabled = false` (because `Enabled == false` forces all flags to false). The WAF handler is therefore absent from the Caddy route generated during proxy host creation.

After the security config PUT (step 5 of the script), `SecurityConfig.Enabled = true` and `WAFMode = "block"` are written to the DB. The subsequent `ApplyConfig` call correctly builds the WAF handler. However, this ordering means the first `ApplyConfig` — the one that actually registers the proxy route — has no WAF. Because the proxy host creation currently fails with 500, this secondary ordering issue is masked; it would surface as TC-3 failing even on a run where the 500 is resolved.

The fix is to reorder the test script so the security configuration is applied **before** the proxy host is created, ensuring `SecurityConfig.Enabled = true` and `WAFMode = "block"` are in the DB when the first `ApplyConfig` runs.

### 2.7 TC-2 Passes Despite Failure

After the proxy host creation rolls back (0 proxy hosts in DB), the security config PUT triggers `ApplyConfig`. This generates a Caddy config that includes the management server and a security-handlers-only route (WAF + rate limiter + ACL), but with no `reverse_proxy` to `cerberus-backend`. TC-2 checks handler ORDER in `.apps.http.servers.charon_server.routes`, finds 1 route, verifies the middleware chain order, and passes. There is no functional proxy route for `cerberus.test.local`, so TC-3 and TC-4 fail: all traffic returns the React SPA served by the management server fallback.

### 2.8 Defect 5 — No Mutex on ApplyConfig (CODE SMELL)

`ApplyConfig` in `backend/internal/caddy/manager.go` has no synchronization. The background startup goroutine calls `ApplyConfig` once (after Caddy pings successfully) and request handlers call it on every proxy host create/update/delete. In the CI sequence, the background goroutine fires ~1 second after container start and the API health check passes at t+15 s — no overlap by default. However, concurrent HTTP requests can trigger concurrent `ApplyConfig` calls, causing interleaved config generation and snapshot corruption. A mutex is a cheap, correct prevention.

### 2.9 Defect 6 — CLI Migrate Missing ProxyGroup (MEDIUM)

**File**: `backend/cmd/api/main.go`, CLI `migrate` subcommand AutoMigrate list.

`&models.ProxyGroup{}` is missing before `&models.ProxyHost{}`. `ProxyHost` has a foreign key to `ProxyGroup`. Running `go run . migrate` on a fresh database would attempt to create `proxy_hosts` before `proxy_groups`, producing an FK constraint violation. Not triggered in CI (CI uses `routes.go:RegisterWithDeps`), but breaks the standalone migration path.

---

## 3. Technical Specifications

### Fix 1 — Fail-Fast Script with Response Body Logging

**File**: `scripts/cerberus_integration.sh`

Replace the proxy host creation block (lines 261–272):

```bash
log_info "Creating proxy host '${TEST_DOMAIN}' pointing to backend..."
CREATE_RESP=$(curl -s -w "\n%{http_code}" -X POST \
    -H "Content-Type: application/json" \
    -b "${TMP_COOKIE}" \
    "http://localhost:${API_PORT}/api/v1/proxy-hosts" \
    -d "${PROXY_HOST_PAYLOAD}")
CREATE_STATUS=$(echo "$CREATE_RESP" | tail -n1)
CREATE_BODY=$(echo "$CREATE_RESP" | head -n -1)

if [ "$CREATE_STATUS" = "201" ]; then
    log_info "Proxy host created successfully"
elif [ "$CREATE_STATUS" = "409" ]; then
    log_info "Proxy host already exists (HTTP 409) — continuing"
else
    log_error "Proxy host creation failed (HTTP ${CREATE_STATUS})"
    log_error "Response body: ${CREATE_BODY}"
    exit 1
fi
```

Only 201 and 409 are acceptable. Any other status exits non-zero with full body logged.

### Fix 2 — Volume Cleanup in Script

**File**: `scripts/cerberus_integration.sh`

Update `cleanup()` to save container logs before removing containers, and remove named volumes:

```bash
cleanup() {
    # Save container logs before removal (consumed by CI artifact upload)
    docker logs "${CONTAINER_NAME}" > /tmp/charon-cerberus-test.log 2>&1 || true
    docker logs "${BACKEND_CONTAINER}" > /tmp/cerberus-backend.log 2>&1 || true

    docker rm -f ${CONTAINER_NAME} 2>/dev/null || true
    docker rm -f ${BACKEND_CONTAINER} 2>/dev/null || true

    # Remove volumes to ensure a clean state for the next run
    docker volume rm charon_cerberus_test_data 2>/dev/null || true
    docker volume rm caddy_cerberus_test_data 2>/dev/null || true
    docker volume rm caddy_cerberus_test_config 2>/dev/null || true
}
```

Also add volume removal in the explicit per-run cleanup block before `docker run`:

```bash
docker rm -f ${CONTAINER_NAME} 2>/dev/null || true
docker rm -f ${BACKEND_CONTAINER} 2>/dev/null || true
docker volume rm charon_cerberus_test_data 2>/dev/null || true
docker volume rm caddy_cerberus_test_data 2>/dev/null || true
docker volume rm caddy_cerberus_test_config 2>/dev/null || true
```

### Fix 3 — CI Artifact Upload for Container Logs

**File**: `.github/workflows/cerberus-integration.yml`

Add an artifact upload step immediately after "Run Cerberus Integration Test". Container logs are written to `/tmp` by `cleanup()` before containers are removed, so this step captures them even on failure:

```yaml
- name: Upload Container Logs on Failure
  if: failure()
  uses: actions/upload-artifact@v4
  with:
    name: cerberus-container-logs-${{ github.run_id }}
    path: |
      /tmp/charon-cerberus-test.log
      /tmp/cerberus-backend.log
    if-no-files-found: ignore
    retention-days: 7
```

### Fix 4 — Reorder Script Steps (Security Config Before Proxy Host)

**File**: `scripts/cerberus_integration.sh`

Move the security config application and WAF ruleset creation to run **before** proxy host creation:

```
Current order:
  Step 4: Create proxy host                       ← ApplyConfig with Enabled=false, WAFMode=disabled
  Step 5: Create WAF ruleset
  Step 6: Apply security config (Enabled=true, WAFMode=block)

Correct order:
  Step 4: Apply security config (Enabled=true, WAFMode=block)
  Step 5: Create WAF ruleset
  Step 6: Create proxy host                       ← ApplyConfig with Enabled=true, WAFMode=block
```

This ensures `computeEffectiveFlags` reads the correct DB state when it generates the Caddy config for the proxy host route. After this reordering, `buildWAFHandler` will receive `wafEnabled=true` and the WAF handler will be included in the route.

The security config endpoint should not require an existing proxy host, so this reordering is safe. Add a short `sleep 1` after the security config PUT to allow Caddy to finish loading the management-only config before the proxy host triggers a follow-up load.

### Fix 5 — Mutex on ApplyConfig

**File**: `backend/internal/caddy/manager.go`

Add a `sync.Mutex` field to `Manager` and acquire it at the start of `ApplyConfig`:

```go
type Manager struct {
    mu sync.Mutex
    // ... existing fields unchanged ...
}

func (m *Manager) ApplyConfig(ctx context.Context) error {
    m.mu.Lock()
    defer m.mu.Unlock()
    // rest of function unchanged
}
```

`sync.Mutex` zero-value is an unlocked mutex; no constructor change is needed.

### Fix 6 — CLI Migrate Missing ProxyGroup

**File**: `backend/cmd/api/main.go`

Add `&models.ProxyGroup{}` immediately before `&models.ProxyHost{}` in the CLI `migrate` subcommand's AutoMigrate call. The order must mirror `routes.go:RegisterWithDeps`.

---

## 4. Implementation Plan

### Phase 1: Playwright Tests

Not applicable. No frontend or UI changes are part of this fix.

### Phase 2: Backend Changes

| # | Task | File | Complexity |
|---|---|---|---|
| B1 | Add `mu sync.Mutex` to `Manager` and lock `ApplyConfig` | `backend/internal/caddy/manager.go` | Low |
| B2 | Add `&models.ProxyGroup{}` before `&models.ProxyHost{}` in CLI migrate | `backend/cmd/api/main.go` | Trivial |

**B1 detail**: Add `mu sync.Mutex` field, add `m.mu.Lock(); defer m.mu.Unlock()` at top of `ApplyConfig`. Verify `sync` import. 3-line change.

**B2 detail**: Find the `migrate` subcommand AutoMigrate call, insert `&models.ProxyGroup{}` immediately before `&models.ProxyHost{}`. 1-line addition.

### Phase 3: Script and CI Changes

| # | Task | File | Complexity |
|---|---|---|---|
| S1 | Fail-fast proxy host creation with body logging | `scripts/cerberus_integration.sh` | Low |
| S2 | Volume cleanup in `cleanup()` and pre-run block | `scripts/cerberus_integration.sh` | Low |
| S3 | Reorder steps: security config → WAF ruleset → proxy host | `scripts/cerberus_integration.sh` | Low |
| S4 | Add artifact upload step for container logs | `.github/workflows/cerberus-integration.yml` | Low |

**S1 detail**: Replace the proxy host creation block (lines 261–272) per Fix 1 spec. ~10-line change.

**S2 detail**: Add `docker logs` capture and `docker volume rm` calls to `cleanup()` and the per-run cleanup block. ~8-line addition.

**S3 detail**: Move the security config PUT block (and WAF ruleset POST) to before the proxy host creation. Add `sleep 1` after security config PUT. The actual block positions depend on the current line numbers — confirm with `grep -n "Step [456]"` before editing.

**S4 detail**: Add the `actions/upload-artifact@v4` step after "Run Cerberus Integration Test" per Fix 3 spec. ~10-line addition.

### Phase 4: Integration and Testing

| # | Task |
|---|---|
| T1 | Push branch changes, trigger `.github/workflows/cerberus-integration.yml` |
| T2 | If HTTP 500 still occurs: download the artifact `cerberus-container-logs-*` and read `charon-cerberus-test.log` for the `"Failed to apply configuration: ..."` line |
| T3 | Implement targeted fix based on the exact Caddy error (defer to Contingency Commit 5 if needed) |
| T4 | Re-run until TC-1 through TC-5 all pass |

### Phase 5: Documentation

No user-facing documentation changes required beyond this spec.

---

## 5. Acceptance Criteria

| # | Criterion | Verification |
|---|---|---|
| AC1 | Script exits non-zero on any non-201/non-409 proxy host creation response | Script output contains `"Proxy host creation failed (HTTP ...)"` and exits 1 |
| AC2 | Full HTTP response body is logged on proxy host creation failure | Script output contains `"Response body: ..."` with the Caddy error string |
| AC3 | Named volumes are removed at cleanup and before each run | `docker volume ls` shows no `charon_cerberus_test_data` after script exit |
| AC4 | Container logs saved to `/tmp` before container removal | `/tmp/charon-cerberus-test.log` exists and is non-empty after a test run |
| AC5 | Container logs uploaded as CI artifact on failure | GitHub Actions run shows `cerberus-container-logs-*` artifact on any failing run |
| AC6 | Security config applied to DB before proxy host creation | `computeEffectiveFlags` returns `wafEnabled=true` when proxy host `ApplyConfig` fires |
| AC7 | `ApplyConfig` calls are serialized by mutex | No concurrent `ApplyConfig` executions possible |
| AC8 | `go run . migrate` succeeds on a fresh database | `proxy_groups` table created before `proxy_hosts`; no FK constraint error |
| AC9 | TC-3 passes: 3/3 malicious requests blocked | WAF returns HTTP 403 for XSS and injection payloads |
| AC10 | TC-4 passes: 10/10 legitimate requests succeed | Responses are from httpbin backend (HTTP 200, JSON body) |
| AC11 | TC-1, TC-2, TC-5 continue to pass | No regression from current PASS state |
| AC12 | DoD: all 5 TCs pass in CI without errors | `.github/workflows/cerberus-integration.yml` run exits green; no `"Proxy host may already exist"` log line present |

---

## 6. Commit Slicing Strategy

**Decision**: Single PR (#1026) with 4 ordered logical commits. All changes are confined to script, workflow, and backend files. Each commit is independently revertible.

| Commit | Scope | Files | Dependencies | Validation Gate |
|---|---|---|---|---|
| Commit 1 | Observability: fail-fast + body logging + volume cleanup + log capture + CI artifact | `scripts/cerberus_integration.sh`, `.github/workflows/cerberus-integration.yml` | None | Next CI run produces either a PASS or a readable artifact with the exact Caddy error |
| Commit 2 | Script: reorder steps so security config precedes proxy host creation | `scripts/cerberus_integration.sh` | Commit 1 (clean volumes needed to verify ordering) | TC-3 and TC-4 pass; `wafEnabled=true` at proxy host `ApplyConfig` time |
| Commit 3 | Backend: mutex on `ApplyConfig` | `backend/internal/caddy/manager.go` | None | Existing unit tests pass; no behavioral change |
| Commit 4 | Backend: CLI `migrate` fix | `backend/cmd/api/main.go` | None | `go run . migrate` on a fresh SQLite file creates `proxy_groups` before `proxy_hosts` |

**Rollback**: Each commit can be reverted independently. Commits 1 and 2 are highest priority; Commits 3 and 4 are precautionary hardening with zero risk.

**Contingency**: If Commit 1 reveals a non-trivial Caddy error (e.g., a JSON shape regression in the generated config, a plugin API mismatch, a TLS policy validation failure for `.local` domains), a Commit 5 addresses that specific code path after diagnosis. The nature of that fix cannot be specified until the error body is captured from the artifact.
