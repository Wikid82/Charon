# Rate Limit CI Fix — Implementation Plan

**Target CI workflow**: `.github/workflows/rate-limit-integration.yml`
**Failing job**: `Rate Limiting Integration` (run 23194429042, job 67398830076, PR #852)
**Files touched**: `scripts/rate_limit_integration.sh`, `Dockerfile`

---

## 1. Root Cause Analysis

### Issue 1: `rate_limit` handler never appears in running Caddy config

**Observed symptom** (from CI log):

```
Attempt 10/10: rate_limit handler not found, waiting...
✗ rate_limit handler verification failed after 10 attempts
WARNING: Rate limit handler verification failed (Caddy may still be loading)
Proceeding with test anyway...
Rate limit enforcement test FAILED
```

#### Code path trace

The `verify_rate_limit_config` function in `scripts/rate_limit_integration.sh` (lines ~35–58) executes:

```bash
caddy_config=$(curl -s http://localhost:2119/config 2>/dev/null || echo "")
if echo "$caddy_config" | grep -q '"handler":"rate_limit"'; then
```

This polls Caddy's admin API at `http://localhost:2119/config` (port 2119 = container port 2019 via `-p 2119:2019`) for a JSON document containing the compact string `"handler":"rate_limit"`. The grep pattern is correct for compact JSON emitted by Caddy's admin API; that is not the bug.

The handler is absent from Caddy's running config because `ApplyConfig` in `backend/internal/caddy/manager.go` was either never called with `rateLimitEnabled = true`, or it was called successfully but was then overwritten by a subsequent call.

**Call chain that should produce the handler:**

1. `POST /api/v1/security/config` → `SecurityHandler.UpdateConfig` (`security_handler.go:263`)
2. `UpdateConfig` sets `payload.RateLimitMode = "enabled"` when `payload.RateLimitEnable == true` (`security_handler.go:279`)
3. `svc.Upsert(&payload)` writes to DB (`security_service.go:152`)
4. `h.caddyManager.ApplyConfig(ctx)` is called (`security_handler.go:290`)
5. `ApplyConfig` calls `computeEffectiveFlags` (`manager.go:288`)
6. `computeEffectiveFlags` reads DB: `sc.RateLimitMode = "enabled"` → `rateLimitEnabled = true` (`manager.go:669`)
7. Guard: `if !cerbEnabled { rateLimitEnabled = false }` — only fires if Cerberus is disabled (`manager.go:739`)
8. `GenerateConfig` is called with `rateLimitEnabled = true` and `&secCfg` (`manager.go:421`)
9. In `config.go:594`: `if rateLimitEnabled { buildRateLimitHandler(...) }`
10. `buildRateLimitHandler` returns a handler only when `secCfg.RateLimitRequests > 0 && secCfg.RateLimitWindowSec > 0` (`config.go:1437`)
11. Config is POSTed to Caddy admin API at `0.0.0.0:2019` (`config.go:32`)

**Root cause A — silent failure of the security config POST step** (contributing):

The security config POST step in the script discards stdout only; curl exits 0 for HTTP 4xx without -f flag, so auth failures are invisible:

```bash
# scripts/rate_limit_integration.sh, ~line 248
curl -s -X POST -H "Content-Type: application/json" \
    -d "${SEC_CFG_PAYLOAD}" \
    -b ${TMP_COOKIE} \
    http://localhost:8280/api/v1/security/config >/dev/null
```

No HTTP status check is performed. If this returns 4xx (e.g., `403 Forbidden` because the requesting user lacks the admin role, or `401 Unauthorized` because the cookie was not accepted), the config is never saved to DB, `ApplyConfig` is never called with the rate_limit values, and the handler is never injected.

The route is protected by `middleware.RequireRole(models.RoleAdmin)` (routes.go:572–573):

```go
securityAdmin := management.Group("/security")
securityAdmin.Use(middleware.RequireRole(models.RoleAdmin))
securityAdmin.POST("/config", securityHandler.UpdateConfig)
```

A non-admin authenticated user, or an unauthenticated request, returns `403` silently.

**Root cause B — warn-and-proceed instead of fail-hard** (amplifier):

`verify_rate_limit_config` returns `1` on failure, but the calling site in the script treats the failure as non-fatal:

```bash
# scripts/rate_limit_integration.sh, ~line 269
if ! verify_rate_limit_config; then
    echo "WARNING: Rate limit handler verification failed (Caddy may still be loading)"
    echo "Proceeding with test anyway..."
fi
```

The enforcement test that follows is guaranteed to fail when the handler is absent (all requests pass through with HTTP 200, never hitting 429), yet the test proceeds unconditionally. The verification failure should be a hard exit.

**Root cause C — no response code check for proxy host creation** (contributing):

The proxy host creation at step 5 checks the status code (`201` vs other), but allows non-201 with a soft log message:

```bash
if [ "$CREATE_STATUS" = "201" ]; then
    echo "✓ Proxy host created successfully"
else
    echo "  Proxy host may already exist (status: $CREATE_STATUS)"
fi
```

If this returns `401` (auth failure), no proxy host is registered. Requests to `http://localhost:8180/get` with `Host: ratelimit.local` then hit Caddy's catch-all route returning HTTP 200 (the Charon frontend), not the backend. No 429 will ever appear regardless of rate limit configuration.

**Root cause D — `ApplyConfig` failure is swallowed; Caddy not yet ready when config is posted** (primary):

In `UpdateConfig` (`security_handler.go:289–292`):

```go
if h.caddyManager != nil {
    if err := h.caddyManager.ApplyConfig(c.Request.Context()); err != nil {
        log.WithError(err).Warn("failed to apply security config changes to Caddy")
    }
}
c.JSON(http.StatusOK, gin.H{"config": payload})
```

If `ApplyConfig` fails (Caddy not yet fully initialized, config validation error), the error is logged as a warning but the HTTP response is still `200 OK`. The test script sees 200, assumes success, and proceeds.

---

### Issue 2: GeoIP database checksum mismatch

**Observed symptom**: During non-CI Docker builds, the GeoIP download step prints `⚠️  Checksum failed` and creates a `.placeholder` file, but the downloaded `.mmdb` is left on disk alongside the placeholder.

**Code location**: `Dockerfile`, lines that contain:

```dockerfile
ARG GEOLITE2_COUNTRY_SHA256=aa154fc6bcd712644de232a4abcdd07dac1f801308c0b6f93dbc2b375443da7b
```

**Non-CI verification block** (Dockerfile, local build path):

```dockerfile
if [ -s /app/data/geoip/GeoLite2-Country.mmdb ] && \
   echo "${GEOLITE2_COUNTRY_SHA256}  /app/data/geoip/GeoLite2-Country.mmdb" | sha256sum -c -; then
    echo "✅ GeoIP checksum verified";
else
    echo "⚠️  Checksum failed";
    touch /app/data/geoip/GeoLite2-Country.mmdb.placeholder;
fi;
```

**Root cause**: `P3TERX/GeoLite.mmdb` is a third-party repository that updates `GeoLite2-Country.mmdb` frequently (often weekly). The pinned SHA256 `aa154fc6...` is a point-in-time hash that diverges from the real file as soon as P3TERX publishes an update. The `update-geolite2.yml` workflow exists to keep it synchronized (runs weekly on Monday 02:00 UTC), but if a PR is opened or a build is triggered between the weekly update and the next file change, the hash is stale.

**Additional symptom**: When checksum fails, the valid-but-mismatched `.mmdb` is NOT removed. The image contains both the downloaded `.mmdb` and the `.placeholder`. The application reads `CHARON_GEOIP_DB_PATH=/app/data/geoip/GeoLite2-Country.mmdb` and may load the file (which is valid, just a newer version). This means the "checksum failure" is actually harmless at runtime — the file is a valid GeoIP database — but it creates confusing build output and will break if `sha256sum` is ever made fatal.

**CI path does NOT check the checksum** (from `if [ "$CI" = "true" ]` branch), so CI builds are unaffected by this specific bug. This is a local build / release build concern.

---

## 2. Fix for Issue 1

### 2.1 File: `scripts/rate_limit_integration.sh`

#### Change 1 — Add response code check to Step 4 (auth)

**Function/location**: Step 4, immediately after the `curl` login call (~line 213).

**Current behavior**: Login response is discarded with `>/dev/null`; `"✓ Authentication complete"` is printed unconditionally.

**Required change**: Capture the HTTP status code from the login response. Fail fast if login returns non-200.

Exact change — replace:

```bash
curl -s -X POST -H "Content-Type: application/json" \
    -d '{"email":"ratelimit@example.local","password":"password123"}' \
    -c ${TMP_COOKIE} \
    http://localhost:8280/api/v1/auth/login >/dev/null

echo "✓ Authentication complete"
```

With:

```bash
LOGIN_STATUS=$(curl -s -w "\n%{http_code}" -X POST -H "Content-Type: application/json" \
    -d '{"email":"ratelimit@example.local","password":"password123"}' \
    -c ${TMP_COOKIE} \
    http://localhost:8280/api/v1/auth/login | tail -n1)

if [ "$LOGIN_STATUS" != "200" ]; then
    echo "✗ Login failed (HTTP $LOGIN_STATUS) — aborting"
    exit 1
fi
echo "✓ Authentication complete (HTTP $LOGIN_STATUS)"
```

#### Change 2 — Fix proxy host creation to preserve idempotency while catching auth failures (Step 5)

**Current behavior**: Non-201 responses are treated as "may already exist" and execution continues — including `401`/`403` auth failures.

Required change — replace:

```bash
if [ "$CREATE_STATUS" = "201" ]; then
    echo "✓ Proxy host created successfully"
else
    echo "  Proxy host may already exist (status: $CREATE_STATUS)"
fi
```

With:

```bash
if [ "$CREATE_STATUS" = "201" ]; then
    echo "✓ Proxy host created successfully"
elif [ "$CREATE_STATUS" = "401" ] || [ "$CREATE_STATUS" = "403" ]; then
    echo "✗ Proxy host creation failed — authentication/authorization error (HTTP $CREATE_STATUS)"
    exit 1
else
    echo "  Proxy host may already exist or was created (status: $CREATE_STATUS) — continuing"
fi
```

#### Change 3 — Add Caddy admin API readiness gate before security config POST (PRIMARY FIX)

**Location**: Insert immediately before Step 6 (the security config POST curl call).

**Rationale**: Root Cause D is the primary driver of handler-not-found failures. If Caddy's admin API is not yet fully initialized when the security config is POSTed, `ApplyConfig` fails silently (logged as a warning only), the rate_limit handler is never injected into Caddy's running config, and the verification loop times out. The readiness gate ensures Caddy is accepting admin API requests before any config change is attempted.

**Required change** — insert before the security config POST:

```bash
echo "Waiting for Caddy admin API to be ready..."
for i in {1..20}; do
    if curl -s -f http://localhost:2119/config/ >/dev/null 2>&1; then
        echo "✓ Caddy admin API is ready"
        break
    fi
    if [ $i -eq 20 ]; then
        echo "✗ Caddy admin API failed to become ready"
        exit 1
    fi
    echo -n '.'
    sleep 1
done
```

#### Change 4 — Capture and validate Step 6 security config POST

**Location**: Step 6, the `curl` that calls `/api/v1/security/config` (~line 244–253).

**Current behavior**: Response is discarded with `>/dev/null`. No status check.

Required change — replace:

```bash
curl -s -X POST -H "Content-Type: application/json" \
    -d "${SEC_CFG_PAYLOAD}" \
    -b ${TMP_COOKIE} \
    http://localhost:8280/api/v1/security/config >/dev/null

echo "✓ Rate limiting configured"
```

With:

```bash
SEC_CONFIG_RESP=$(curl -s -w "\n%{http_code}" -X POST -H "Content-Type: application/json" \
    -d "${SEC_CFG_PAYLOAD}" \
    -b ${TMP_COOKIE} \
    http://localhost:8280/api/v1/security/config)
SEC_CONFIG_STATUS=$(echo "$SEC_CONFIG_RESP" | tail -n1)
SEC_CONFIG_BODY=$(echo "$SEC_CONFIG_RESP" | head -n-1)

if [ "$SEC_CONFIG_STATUS" != "200" ]; then
    echo "✗ Security config update failed (HTTP $SEC_CONFIG_STATUS)"
    echo "  Response body: $SEC_CONFIG_BODY"
    echo "  Verify the auth cookie is valid and the user has the admin role."
    exit 1
fi
echo "✓ Rate limiting configured (HTTP $SEC_CONFIG_STATUS)"
```

#### Change 5 — Increase pre-verification wait and make `verify_rate_limit_config` fatal

**Location**: Lines ~266–273 (the `if ! verify_rate_limit_config; then` block).

**Current behavior**: Failed verification logs a warning and continues.

Required change — replace:

```bash
echo "Waiting for Caddy to apply configuration..."
sleep 5

# Verify rate limit handler is configured
if ! verify_rate_limit_config; then
    echo "WARNING: Rate limit handler verification failed (Caddy may still be loading)"
    echo "Proceeding with test anyway..."
fi
```

With:

```bash
echo "Waiting for Caddy to apply configuration..."
sleep 8

# Verify rate limit handler is configured — this is a hard requirement
if ! verify_rate_limit_config; then
    echo "✗ Rate limit handler verification failed — aborting test"
    echo "  The handler must be present in Caddy config before enforcement can be tested."
    echo ""
    echo "=== Caddy admin API full config ==="
    curl -s http://localhost:2119/config/ 2>/dev/null | head -200 || echo "Admin API not responding"
    echo ""
    echo "=== Security config from API ==="
    curl -s -b ${TMP_COOKIE} http://localhost:8280/api/v1/security/config 2>/dev/null || echo "API not responding"
    exit 1
fi
```

**Rationale for increasing sleep from 5 to 8 seconds**: Caddy propagates config changes to its internal state asynchronously after the admin API `/load` call returns. On CI runners that are CPU-constrained, 5 s may be insufficient. 8 s adds a safety margin without meaningfully extending the test runtime. This sleep is a **secondary** improvement addressing propagation delay *after* a successful `ApplyConfig`; the Caddy admin API readiness gate (Change 3) is the primary fix for handler-not-found failures caused by Caddy not yet accepting requests when the config POST is attempted.

#### Change 6 — Update retry parameters in `verify_rate_limit_config`

**Location**: Function `verify_rate_limit_config`, variables `retries` and `wait` (~line 36).

**Current behavior**: 10 retries × 3 second wait = 30 s total budget. With the `sleep 5` removed-as-a-pre-step wait (now `sleep 8`), the first retry fires after 8 s from config application.

No change needed to retry parameters; the 30-second budget (plus the 8-second pre-sleep) is sufficient. If anything, increase `wait=3` to `wait=5` to reduce polling noise:

```bash
# In verify_rate_limit_config function:
local retries=10
local wait=5   # was: 3
```

#### Change 7 — Use trailing slash on Caddy admin API URL in `verify_rate_limit_config`

**Location**: `verify_rate_limit_config`, line ~42:

```bash
caddy_config=$(curl -s http://localhost:2119/config 2>/dev/null || echo "")
```

Caddy's admin API specification defines `GET /config/` (with trailing slash) as the canonical endpoint for the full running config. Omitting the slash works in practice because Caddy does not redirect, but using the canonical form is more correct and avoids any future behavioral change:

Replace:

```bash
caddy_config=$(curl -s http://localhost:2119/config 2>/dev/null || echo "")
```

With:

```bash
caddy_config=$(curl -s http://localhost:2119/config/ 2>/dev/null || echo "")
```

Also update the same URL in the `on_failure` function (~line 65) and the workflow's `Dump Debug Info on Failure` step in `.github/workflows/rate-limit-integration.yml`.

---

## 3. Fix for Issue 2

### 3.1 File: `Dockerfile`

**Decision: Remove checksum validation from the non-CI local build path.**

**Rationale**: The file at `https://github.com/P3TERX/GeoLite.mmdb/raw/download/GeoLite2-Country.mmdb` is updated continuously. The `GEOLITE2_COUNTRY_SHA256` ARG was designed to be updated weekly by `update-geolite2.yml`, but any lag between a P3TERX push and the Monday cron creates a stale hash. Pinning a hash for a file that changes by design is not a meaningful security or integrity control — the source is a public GitHub repo, not a signed artifact. The file-size check (`-s`) provides minimum viability validation (non-empty).

**What NOT to do**: Do not make the checksum check fatal. Do not try to "catch up" by dynamically fetching the expected checksum alongside the file (that would defeat the purpose of a hash check).

**Exact change**: Find the local build path in the `RUN mkdir -p /app/data/geoip` block (Dockerfile ~line 450–475). The `else` branch (non-CI path) currently does:

```dockerfile
else \
    echo "Local - full download (30s timeout, 3 retries)"; \
    if wget -qO /app/data/geoip/GeoLite2-Country.mmdb \
        -T 30 -t 4 "https://github.com/P3TERX/GeoLite.mmdb/raw/download/GeoLite2-Country.mmdb"; then \
        if [ -s /app/data/geoip/GeoLite2-Country.mmdb ] && \
           echo "${GEOLITE2_COUNTRY_SHA256}  /app/data/geoip/GeoLite2-Country.mmdb" | sha256sum -c -; then \
            echo "✅ GeoIP checksum verified"; \
        else \
            echo "⚠️  Checksum failed"; \
            touch /app/data/geoip/GeoLite2-Country.mmdb.placeholder; \
        fi; \
    else \
        echo "⚠️  Download failed"; \
        touch /app/data/geoip/GeoLite2-Country.mmdb.placeholder; \
    fi; \
fi
```

Replace with:

```dockerfile
else \
    echo "Local - full download (30s timeout, 3 retries)"; \
    if wget -qO /app/data/geoip/GeoLite2-Country.mmdb \
        -T 30 -t 4 "https://github.com/P3TERX/GeoLite.mmdb/raw/download/GeoLite2-Country.mmdb" \
        && [ -s /app/data/geoip/GeoLite2-Country.mmdb ]; then \
        echo "✅ GeoIP downloaded"; \
    else \
        echo "⚠️  GeoIP download failed or empty — skipping"; \
        touch /app/data/geoip/GeoLite2-Country.mmdb.placeholder; \
    fi; \
fi
```

**Important**: Do NOT remove the `ARG GEOLITE2_COUNTRY_SHA256` declaration from the Dockerfile. The `update-geolite2.yml` workflow uses `sed` to update that ARG. If the ARG disappears, the workflow's `sed` command will silently no-op and fail to update the Dockerfile on next run, leaving the stale hash in source while the workflow reports success. Keeping the ARG (even unused) preserves Renovate/workflow compatibility.

Keep:

```dockerfile
ARG GEOLITE2_COUNTRY_SHA256=aa154fc6bcd712644de232a4abcdd07dac1f801308c0b6f93dbc2b375443da7b
```

This ARG is now only referenced by the `update-geolite2.yml` workflow (to know if an update is needed), not by the Dockerfile build logic.

---

## 4. Files to Change

| File | Change |
|------|--------|
| `scripts/rate_limit_integration.sh` | Add Caddy admin API readiness gate before security config POST (primary fix, Change 3); add HTTP status checks to auth login (Step 4), proxy host creation (Step 5, idempotent with auth-failure hard exit), and security config POST (Step 6); change `verify_rate_limit_config` failure from warn-and-proceed to hard exit; increase pre-verification sleep from 5 to 8 s (secondary); increase retry wait from 3 to 5 s; use trailing slash on Caddy admin URL |
| `Dockerfile` | Remove `sha256sum -c` check from non-CI GeoIP download path; retain `ARG GEOLITE2_COUNTRY_SHA256` declaration |
| `.github/workflows/rate-limit-integration.yml` | Update debug dump URL from `/config` to `/config/` in `Dump Debug Info on Failure` step |

**No backend Go code changes are required.** The `generate config → push to Caddy` pipeline (`manager.go` → `config.go`) is correct. The bug is entirely in the test script's error handling.

---

## 5. Test Validation

### Validating Issue 1 fix

**Step 1 — Build and run the integration test locally:**

```bash
# From /projects/Charon
chmod +x scripts/rate_limit_integration.sh
scripts/rate_limit_integration.sh 2>&1 | tee /tmp/ratelimit-test.log
```

**Expected output sequence (key lines)**:

```
✓ Charon API is ready
✓ Authentication complete (HTTP 200)
✓ Proxy host created successfully
✓ Rate limiting configured (HTTP 200)
Verifying rate limit config in Caddy...
  ✓ rate_limit handler found in Caddy config
Sending 3 rapid requests (should all return 200)...
  Request 1: HTTP 200 ✓
  Request 2: HTTP 200 ✓
  Request 3: HTTP 200 ✓
Sending request 3+1 (should return 429 Too Many Requests)...
  ✓ Request blocked with HTTP 429 as expected
  ✓ Retry-After header present: Retry-After: ...
=== ALL RATE LIMIT TESTS PASSED ===
```

**Step 2 — Deliberately break auth to verify the new guard fires:**
Temporarily change `password123` in the login curl to a wrong password. The test should now print:

```
✗ Login failed (HTTP 401) — aborting
```

and exit with code 1, rather than proceeding to a confusing 429-enforcement failure.

**Step 3 — Verify Caddy config contains the handler before enforcement:**

```bash
# After security config step and sleep 8:
curl -s http://localhost:2119/config/ | python3 -m json.tool | grep -A2 '"handler": "rate_limit"'
```

Expected: handler block with `"rate_limits"` sub-key containing `"static"` zone.

**Step 4 — CI validation:** Push to a PR and observe the `Rate Limiting Integration` workflow. The workflow now exits at the first unmissable error rather than proceeding to a deceptive "enforcement test FAILED" message.

### Validating Issue 2 fix

**Step 1 — Local build without CI flag:**

```bash
docker build -t charon:geolip-test --build-arg CI=false . 2>&1 | grep -E "GeoIP|GeoLite|checksum|✅|⚠️"
```

Expected: `✅ GeoIP downloaded` (no mention of checksum failure).

**Step 2 — Verify file is present and readable:**

```bash
docker run --rm charon:geolip-test stat /app/data/geoip/GeoLite2-Country.mmdb
```

Expected: file exists with non-zero size, no `.placeholder` alongside.

**Step 3 — Confirm ARG still exists for workflow compatibility:**

```bash
grep "GEOLITE2_COUNTRY_SHA256" Dockerfile
```

Expected: `ARG GEOLITE2_COUNTRY_SHA256=<hash>` line is present.

---

## 6. Commit Slicing Strategy

**Recommendation: Two commits in one PR.**

| Commit | Scope | Rationale |
|--------|-------|-----------|
| `fix(ci): add error handling to rate-limit integration test script` | `scripts/rate_limit_integration.sh`, `.github/workflows/rate-limit-integration.yml` | Fixes the failing CI job. Independent of the Dockerfile change. Can be reviewed and reverted without touching build infrastructure. |
| `fix(docker): remove stale checksum guard from local GeoIP download` | `Dockerfile` | The GeoIP fix is non-urgent (CI builds already bypass the check) and lower risk. Separating it keeps the Dockerfile diff reviewable on its own. |

**Single PR is acceptable** because neither change touches application logic or tests that could regress. The two fixes are independent — reverting either one does not break the other. A single `fix: rate-limit CI and GeoIP checksum` PR is clean.

**Do not split into multiple PRs.** There is no reason to delay the GeoIP fix; it has no review risk.

---

## 7. Risk Assessment

### Issue 1 fixes

| Change | Regression risk | Notes |
|--------|----------------|-------|
| Add `exit 1` on login failure | Low | Only fires on auth failure, which the test never previously survived correctly anyway |
| Fix proxy host creation to preserve idempotency | Low | 401/403 now exit hard; any other non-201 status (including duplicate `400`) continues safely |
| Exit on security config non-200 | Low | Valid `200` path is unchanged; new error path only fires for bugs already causing test failure |
| Change verify to hard failure | Low | The "proceed anyway" path was always incorrect; removing it makes failures faster and clearer |
| Increase sleep from 5 to 8 s | Low positive | Adds 3 s to total test runtime; reduces flakiness on slow CI runners |
| Increase retry wait from 3 to 5 s | Low positive | Reduces Caddy admin API polling frequency; net retry budget remains ~50 s |
| `/config/` trailing slash | Negligible | Caddy handles both; change aligns with documented API spec |

**Watch for**: Any test that depends on the soft-failure path in `verify_rate_limit_config` — there are none in this repo (the function is only called here). No other workflow references `rate_limit_integration.sh`.

### Issue 2 fixes

| Change | Regression risk | Notes |
|--------|----------------|-------|
| Remove `sha256sum` check | Low | The check was already non-fatal (fell through to a placeholder). Removing it makes the behavior identical to the CI path. |
| Retain `ARG GEOLITE2_COUNTRY_SHA256` | None | Preserving the ARG prevents `update-geolite2.yml` from silently failing. |
| `.placeholder` no longer created on version mismatch | Low positive | The `.placeholder` file confused runtime detection; application now always has the valid mmdb. |

**Watch for**: If the application code checks for the `.placeholder` file's existence to disable GeoIP (rather than simply checking if the mmdb opens successfully), removing the forced-placeholder creation could change behavior. Search term: `GeoLite2-Country.mmdb.placeholder` in `backend/`. At time of writing, no application code references the placeholder file; the application checks for the mmdb via `os.Stat(geoipPath)` in `routes.go` and opens it via `services.NewGeoIPService(geoipPath)`.
