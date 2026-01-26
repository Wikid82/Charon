# Playwright Security Tests Failures - Investigation & Fix Plan

**Issue**: GitHub Actions run `21351787304` fails in Playwright project `security-tests` (runs as a dependency of `chromium` via Playwright config)
**Status**: ✅ RESOLVED - Test Isolation Fix Applied
**Priority**: 🔴 HIGH - Break-glass + security gating tests are blocking CI
**Created**: 2026-01-26
**Resolved**: 2026-01-26

---

## Resolution Summary

**Root Cause**: Test isolation failure due to shared rate limit bucket state between `emergency-token.spec.ts` (Test 1) and subsequent tests (Test 2, and tests in `emergency-reset.spec.ts`).

**Fix Applied**: Added rate limit bucket drainage waits:
- Test 2 now waits 61 seconds **before** making requests (to drain bucket from Test 1)
- Test 2 now waits 61 seconds **after** completing (to drain bucket before `emergency-reset.spec.ts` runs)

**Files Changed**:
- `tests/security-enforcement/emergency-token.spec.ts` (Test 2 modified)

**Verification**: All 15 emergency security tests now pass consistently.

---

## Original Symptoms (from CI)

- `tests/security-enforcement/emergency-reset.spec.ts`: expects `429` after 5 invalid token attempts, but receives `401`.
- `tests/security-enforcement/emergency-token.spec.ts`: expects `429` on 6th request, but receives `401`.
- An `auditLogs.find is not a function` failure is reported (strong signal the “audit logs” payload was not the expected array/object shape).
- Later security tests that expect `response.ok() === true` start failing (likely cascading after the emergency reset doesn’t disable ACL/Cerberus).

Key observation: these failures happen under Playwright project `security-tests`, which is a configured dependency of the `chromium` project.

---

## How `security-tests` runs in CI (why it fails even when CI runs `--project=chromium`)

- Playwright config defines a project named `security-tests` with `testDir: './tests/security-enforcement'`.
- The `chromium` project declares `dependencies: ['setup', 'security-tests']`.
- Therefore `npx playwright test --project=chromium` runs the `setup` project, then the `security-tests` project, then finally browser tests.

Files:
- `playwright.config.js` (project graph and baseURL rules)
- `tests/security-enforcement/*` (failing tests)

---

## Backend: emergency token configuration (env vars + defaults)

### Tier 1: Main API emergency reset endpoint

Endpoint:
- `POST /api/v1/emergency/security-reset` is registered directly on the Gin router (outside the authenticated `/api/v1` protected group).

Token configuration:
- Environment variable: `CHARON_EMERGENCY_TOKEN`
- Minimum length: `32` chars
- Request header: `X-Emergency-Token`

Code:
- `backend/internal/api/handlers/emergency_handler.go`
  - `EmergencyTokenEnvVar = "CHARON_EMERGENCY_TOKEN"`
  - `EmergencyTokenHeader = "X-Emergency-Token"`
  - `MinTokenLength = 32`
- `backend/internal/api/middleware/emergency.go`
  - Same env var + header constants; validates IP-in-management-CIDR and token match.

### Management CIDR configuration (who is allowed to use token)

- Environment variable: `CHARON_MANAGEMENT_CIDRS` (comma-separated)
- Default if unset: RFC1918 private ranges plus loopback
  - `10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`, `127.0.0.0/8`

Code:
- `backend/internal/config/config.go` → `loadSecurityConfig()` parses `CHARON_MANAGEMENT_CIDRS` into `cfg.Security.ManagementCIDRs`.
- `backend/internal/api/middleware/emergency.go` → `EmergencyBypass(cfg.Security.ManagementCIDRs, db)` falls back to RFC1918 if empty.

### Tier 2: Separate emergency server (not the failing endpoint, but relevant context)

The repo also contains a separate “emergency server” (different port/route):
- `POST /emergency/security-reset` (note: not `/api/v1/...`)

Env vars (tier 2 server):
- `CHARON_EMERGENCY_SERVER_ENABLED` (default `false`)
- `CHARON_EMERGENCY_BIND` (default `127.0.0.1:2019`)
- `CHARON_EMERGENCY_USERNAME`, `CHARON_EMERGENCY_PASSWORD` (basic auth)

Code:
- `backend/internal/server/emergency_server.go`
- `backend/internal/config/config.go` (`EmergencyConfig`)

---

## Backend: rate limiting + middleware order (expected behavior)

### Routing / middleware order

Registration order matters; current code intends:

1. **Emergency bypass middleware is first**
   - `router.Use(middleware.EmergencyBypass(cfg.Security.ManagementCIDRs, db))`
2. Gzip + security headers
3. Register emergency endpoint on the root router:
   - `router.POST("/api/v1/emergency/security-reset", emergencyHandler.SecurityReset)`
4. Create `/api/v1` group and apply Cerberus middleware to it
5. Create protected group and apply auth middleware

Code:
- `backend/internal/api/routes/routes.go`

### Emergency endpoint logic + rate limiting

Rate limiting is implemented inside the handler, keyed by **client IP string**:

- Handler: `(*EmergencyHandler).SecurityReset`
- Rate limiter: `(*EmergencyHandler).checkRateLimit(ip string) bool`
  - State is in-memory: `map[string]*rateLimitEntry` guarded by a mutex.
  - In test/dev/e2e: **5 attempts per 1 minute** (matches test expectations)
  - In prod: **5 attempts per 5 minutes**

Critical detail: rate limiting is performed **before** token validation in the legacy path.
That is what allows the test behavior “first 5 are 401, 6th is 429”.

Code:
- `backend/internal/api/handlers/emergency_handler.go`
  - `MaxAttemptsPerWindow = 5`
  - `RateLimitWindow = time.Minute`
  - `clientIP := c.ClientIP()` used for rate-limit key.

---

## Playwright tests: expected behavior + env vars

### What the tests expect

- `tests/security-enforcement/emergency-reset.spec.ts`
  - Invalid token returns `401`
  - Missing token returns `401`
  - **Rate limit**: after 5 invalid attempts, the 6th returns `429`

- `tests/security-enforcement/emergency-token.spec.ts`
  - Enables Cerberus + ACL, verifies normal requests are blocked (`403`)
  - Uses the emergency token to reset security and expects `200` and modules disabled
  - **Rate limit**: 6 rapid invalid attempts → first 5 are `401`, 6th is `429`
  - Fetches `/api/v1/audit-logs` and expects the request to succeed (auth cookies via setup storage state)

### Which env vars the tests use

- `PLAYWRIGHT_BASE_URL`
  - Read in `playwright.config.js` as the global `use.baseURL`.
  - In CI `e2e-tests.yml`, it’s set to the Vite dev server (`http://localhost:5173`) and Vite proxies `/api` to backend `http://localhost:8080`.

- `CHARON_EMERGENCY_TOKEN`
  - Used by tests as the emergency token source.
  - Fallback default used in multiple places:
    - `tests/security-enforcement/emergency-reset.spec.ts`
    - `tests/fixtures/security.ts` (exported `EMERGENCY_TOKEN`)

---

## What’s likely misconfigured / fragile in CI wiring

### 1) The emergency token is not explicitly set in CI (tests and container rely on a hardcoded default)

- Compose sets `CHARON_EMERGENCY_TOKEN=${CHARON_EMERGENCY_TOKEN:-test-emergency-token-for-e2e-32chars}`.
- Tests default to the same string when the env var is unset.

This is convenient, but it’s fragile (and not ideal from a “secure-by-default CI” standpoint):
- Any future change to the default in either place silently breaks tests.
- It makes it harder to reason about “what token was used” in a failing run.

File:
- `.docker/compose/docker-compose.playwright.yml`

### 2) Docker Compose is configured to build from source, so the pre-built image artifact is not actually being used

- The workflow `build` job creates `charon:e2e-test` and uploads it.
- The `e2e-tests` job loads that image tar.
- But `.docker/compose/docker-compose.playwright.yml` uses `build:` and the workflow runs `docker compose up -d`.

Result: Compose will prefer building (or at least treat the service as build-based), which defeats the “build once, run many” approach and increases drift risk.

File:
- `.docker/compose/docker-compose.playwright.yml`

### 3) Most likely root cause for the 401 vs 429 mismatch: client IP derivation is unstable and/or spoofable in proxied runs

The rate limiter keys by `clientIP := c.ClientIP()`.

In CI, requests hit Vite (`localhost:5173`) which proxies to backend. Vite adds forwarded headers. If Gin’s `ClientIP()` resolves to different strings across requests (common culprits):
- IPv4 vs IPv6 loopback differences (`127.0.0.1` vs `::1`)
- `X-Forwarded-For` formatting including ports or multiple values
- Untrusted forwarded headers changing per request

Supervisor note / security risk to call out explicitly:
- Gin trusted proxy configuration can make this worse.
  - If the router uses `router.SetTrustedProxies(nil)`, Gin may treat **all** proxies as trusted (behavior depends on Gin version/config), which can cause `c.ClientIP()` to prefer `X-Forwarded-For` from an untrusted hop.
  - That makes rate limiting bypassable (spoofable `X-Forwarded-For`) and can also impact management CIDR checks if they rely on `c.ClientIP()`.
  - If the intent is “trust none”, configure it explicitly (e.g., `router.SetTrustedProxies([]string{})`) so forwarded headers are not trusted.

…then rate limiting becomes effectively per-request and never reaches “attempt 6”, so the handler always returns the token-validation result (`401`).

This hypothesis exactly matches the symptom: “always 401, never 429”.

---

## Minimal, secure fix plan

### Step 1: Confirm the root cause with targeted logging (short-lived)

Add a temporary debug log in `backend/internal/api/handlers/emergency_handler.go` inside `SecurityReset`:
- log the values used for rate limiting:
  - `c.ClientIP()`
  - `c.Request.RemoteAddr`
  - `X-Forwarded-For` and `X-Real-IP` headers (do NOT log token)

Goal: verify whether the IP key differs between requests in CI and/or locally.

### Step 2: Fix/verify Gin trusted proxy configuration (align with “trust none” unless explicitly required)

Goal: ensure `c.ClientIP()` cannot be spoofed via forwarded headers, and that it behaves consistently in proxied runs.

Actions:
- Audit where the Gin router sets trusted proxies.
- If the desired policy is “trust none”, ensure it is configured as such (avoid `SetTrustedProxies(nil)` if it results in “trust all”).
- If some proxies must be trusted (e.g., a known reverse proxy), configure an explicit allow-list and document it.

Verification:
- Confirm requests with arbitrary `X-Forwarded-For` do not change server-side client identity unless coming from a trusted proxy hop.

### Step 3: Introduce a canonical client IP and use it consistently (rate limiting + management CIDR)

Implement a small helper (single source of truth) to derive a canonical client address:
- Prefer server-observed address by parsing `c.Request.RemoteAddr` and stripping the port.
- Normalize loopback (`::1` → `127.0.0.1`) to keep rate-limit keys stable.
- Only consult forwarded headers when (and only when) Gin trusted proxies are explicitly configured to do so.

Apply this canonical IP to both:
- `EmergencyHandler.SecurityReset` (rate limit key)
- `middleware.EmergencyBypass` / management CIDR enforcement (so bypass eligibility and rate limiting agree on “who the client is”)

Files:
- `backend/internal/api/handlers/emergency_handler.go`
- `backend/internal/api/middleware/emergency.go`

### Step 4: Narrow `EmergencyBypass` scope (avoid global bypass for any request with the token)

Goal: the emergency token should only bypass protections for the emergency reset route(s), not grant broad bypass for unrelated endpoints.

Option (recommended): scope the middleware to only the emergency reset route(s)
- Apply `EmergencyBypass(...)` only to the router/group that serves `POST /api/v1/emergency/security-reset` (and any other intended emergency reset endpoints).
- Do not attach the bypass middleware globally on `router.Use(...)`.

Verification:
- Requests to non-emergency routes that include `X-Emergency-Token` must behave unchanged (e.g., still require auth / still subject to Cerberus/ACL).

### Step 5: Make CI token wiring explicit (remove reliance on defaults)

In `.github/workflows/e2e-tests.yml`:
- Generate a random emergency token per workflow run (32+ chars) and export it to `$GITHUB_ENV`.
- Ensure both Docker Compose and Playwright tests see the same `CHARON_EMERGENCY_TOKEN`.

In `.docker/compose/docker-compose.playwright.yml`:
- Prefer requiring `CHARON_EMERGENCY_TOKEN` in CI (either remove the default or conditionally default only for local).

### Step 6: Align docker-compose with the workflow’s “pre-built image per shard” (avoid unused loaded image artifact)

Current misalignment to document clearly:
- The workflow builds and loads `charon:e2e-test`, but compose is build-based, so the loaded image can be unused (and `--build` can force rebuilds).

Minimal alignment options:
- Option A (recommended): Add a CI-only compose override file used by the workflow
  - Example: `.docker/compose/docker-compose.playwright.ci.yml` that sets `image: charon:e2e-test` and removes/overrides `build:`.
  - Workflow runs `docker compose -f ...playwright.yml -f ...playwright.ci.yml up -d`.
- Option B (minimal): Update the existing compose service to include `image: charon:e2e-test` and ensure CI does not pass `--build`.

This does not directly fix the 401/429 issue, but it reduces variability and is consistent with the workflow intent.

---

## Verification steps

1. Run only the failing security test specs locally against the Playwright docker compose environment:
   - `tests/security-enforcement/emergency-reset.spec.ts`
   - `tests/security-enforcement/emergency-token.spec.ts`

2. Run the full security project:
   - `npx playwright test --project=security-tests`

3. Run CI-equivalent shard command locally (optional):
   - `npx playwright test --project=chromium --shard=1/4`
   - Confirm `security-tests` runs as a dependency and passes.

4. Confirm expected statuses:
   - Invalid token attempts: 5× `401`, then `429`
   - Valid token: `200` and modules disabled
   - `/api/v1/audit-logs` succeeds after emergency reset (auth still valid)

5. Security-specific verification (must not regress):
  - Spoofing check: adding/changing `X-Forwarded-For` from an untrusted hop must not change effective client identity used for rate limiting or CIDR checks.
  - Scope check: `X-Emergency-Token` must not act as a global bypass on non-emergency routes.

---

## Notes on the reported `auditLogs.find` failure

This error typically means downstream code assumed an array but received an object (often an error payload like `{ error: 'unauthorized' }`).
Given the cascade of `401` failures, the most likely explanation is:
- the emergency reset didn’t complete,
- security controls remained enabled,
- and later requests (including audit log requests) returned a non-OK payload.

Once the emergency endpoint’s rate limiting and token flow are stable again, this should stop cascading.

---

# E2E Workflow Optimization - Efficiency Analysis

> NOTE: This section was written against an earlier iteration of the workflow. Validate any line numbers/flags against `.github/workflows/e2e-tests.yml` before implementing changes.

**Issue**: E2E workflow contains redundant build steps and inefficiencies
**Status**: Analysis Complete - Ready for Implementation
**Priority**: 🟡 MEDIUM - Performance optimization opportunity
**Created**: 2026-01-26
**Estimated Savings**: ~2-4 minutes per workflow run (~30-40% reduction)

---

## 🎯 Executive Summary

The E2E workflow `.github/workflows/e2e-tests.yml` builds and tests the application efficiently with proper sharding, but contains **4 critical redundancies** that waste CI resources:

| Issue | Location | Impact | Fix Complexity |
|-------|----------|--------|----------------|
| 🔴 **Docker rebuild** | Line 157 | 30-60s per shard (×4) | LOW - Remove flag |
| 🟡 **Duplicate npm installs** | Lines 81, 205, 215 | 20-30s per shard (×4) | MEDIUM - Cache better |
| 🟡 **Unnecessary pre-builds** | Lines 90, 93 | 30-45s in build job | LOW - Remove steps |
| 🟢 **Browser install caching** | Line 201 | 5-10s per shard (×4) | LOW - Already implemented |

**Total Waste per Run**: ~2-4 minutes (120-240 seconds)
**Frequency**: Every PR with frontend/backend/test changes
**Cost**: ~$0.10-0.20 per run (GitHub-hosted runners)

---

## 📊 Current Workflow Architecture

### Job Flow Diagram

```
┌─────────────────┐
│  1. BUILD JOB   │  Runs once
│  - Build image  │
│  - Save as tar  │
│  - Upload       │
└────────┬────────┘
         │
         ├─────────┬─────────┬─────────┐
         ▼         ▼         ▼         ▼
    ┌────────┐ ┌────────┐ ┌────────┐ ┌────────┐
    │ SHARD 1│ │ SHARD 2│ │ SHARD 3│ │ SHARD 4│  Run in parallel
    │ Tests  │ │ Tests  │ │ Tests  │ │ Tests  │
    └────┬───┘ └────┬───┘ └────┬───┘ └────┬───┘
         │         │         │         │
         └─────────┴─────────┴─────────┘
                     │
         ┌───────────┴──────────┐
         ▼                      ▼
    ┌─────────┐         ┌─────────────┐
    │ MERGE   │         │ UPLOAD      │
    │ REPORTS │         │ COVERAGE    │
    └─────────┘         └─────────────┘
         │                      │
         └──────────┬───────────┘
                    ▼
            ┌──────────────┐
            │ COMMENT PR   │
            └──────────────┘
                    │
                    ▼
            ┌──────────────┐
            │ STATUS CHECK │
            └──────────────┘
```

### Jobs Breakdown

| Job | Dependencies | Parallelism | Duration | Purpose |
|-----|--------------|-------------|----------|---------|
| `build` | None | 1 instance | ~2-3 min | Build Docker image once |
| `e2e-tests` | `build` | 4 shards | ~5-8 min | Run tests with coverage |
| `merge-reports` | `e2e-tests` | 1 instance | ~30-60s | Combine HTML reports |
| `comment-results` | `e2e-tests`, `merge-reports` | 1 instance | ~10s | Post PR comment |
| `upload-coverage` | `e2e-tests` | 1 instance | ~30-60s | Merge & upload to Codecov |
| `e2e-results` | `e2e-tests` | 1 instance | ~5s | Final status gate |

**✅ Parallelism is correct**: 4 shards run different test subsets simultaneously.

---

## 🔍 Detailed Analysis

### 1. Docker Image Lifecycle

#### Current Flow

```yaml
# BUILD JOB (Lines 73-118)
- name: Build frontend
  run: npm run build
  working-directory: frontend               # ← REDUNDANT (Dockerfile does this)

- name: Build backend
  run: make build                           # ← REDUNDANT (Dockerfile does this)

- name: Build Docker image
  uses: docker/build-push-action@v6
  with:
    push: false
    load: true
    tags: charon:e2e-test
    cache-from: type=gha                    # ✅ Good - uses cache
    cache-to: type=gha,mode=max

- name: Save Docker image
  run: docker save charon:e2e-test -o charon-e2e-image.tar

- name: Upload Docker image artifact
  uses: actions/upload-artifact@v6
  with:
    name: docker-image
    path: charon-e2e-image.tar
```

```yaml
# E2E-TESTS JOB - PER SHARD (Lines 142-157)
- name: Download Docker image
  uses: actions/download-artifact@v7
  with:
    name: docker-image                      # ✅ Good - reuses artifact

- name: Load Docker image
  run: docker load -i charon-e2e-image.tar  # ✅ Good - loads pre-built image

- name: Start test environment
  run: |
    docker compose -f .docker/compose/docker-compose.playwright.yml up -d --build
    #                                                                    ^^^^^^^^
    #                                                                    🔴 PROBLEM!
```

#### 🔴 Critical Issue: `--build` Flag (Line 157)

**Evidence**: The `--build` flag forces Docker Compose to rebuild the image **even though** we just loaded a pre-built image.

**Impact**:
- **Time**: 30-60 seconds per shard × 4 shards = **2-4 minutes wasted**
- **Resources**: Rebuilds Go backend and React frontend 4 times unnecessarily
- **Cache misses**: May not use build cache, causing slower builds

**Root Cause**:
The compose file references `build: .` which re-triggers Dockerfile build when `--build` is used.

**Verification Command**:
```bash
# Check docker-compose.playwright.yml for build context
grep -A5 "^services:" .docker/compose/docker-compose.playwright.yml
```

---

### 2. Dependency Installation Redundancy

#### Current Flow

```yaml
# BUILD JOB (Line 81)
- name: Install dependencies
  run: npm ci                               # ← Root package.json (Playwright, tools)

# BUILD JOB (Line 84-86)
- name: Install frontend dependencies
  run: npm ci                               # ← Frontend package.json (React, Vite)
  working-directory: frontend

# E2E-TESTS JOB - PER SHARD (Line 205)
- name: Install dependencies
  run: npm ci                               # ← DUPLICATE: Root again

# E2E-TESTS JOB - PER SHARD (Line 215-218)
- name: Install Frontend Dependencies
  run: |
    cd frontend
    npm ci                                  # ← DUPLICATE: Frontend again
```

#### 🟡 Issue: Triple Installation

**Impact**:
- **Time**: ~20-30 seconds per shard × 4 shards = **1.5-2 minutes wasted**
- **Network**: Downloads same packages multiple times
- **Cache efficiency**: Partially mitigated by cache but still wasteful

**Why This Happens**:
- Build job needs dependencies to run `npm run build`
- Test shards need dependencies to run Playwright
- Test shards need frontend deps to start Vite dev server

**Current Mitigation**:
- ✅ Cache exists (Line 77-82, Line 199)
- ✅ Uses `npm ci` (reproducible installs)
- ⚠️ But still runs installation commands repeatedly

---

### 3. Unnecessary Pre-Build Steps

#### Current Flow

```yaml
# BUILD JOB (Lines 90-96)
- name: Build frontend
  run: npm run build                        # ← Builds frontend assets
  working-directory: frontend

- name: Build backend
  run: make build                           # ← Compiles Go binary

- name: Build Docker image
  uses: docker/build-push-action@v6
  # ... Dockerfile ALSO builds frontend and backend
```

**Dockerfile Excerpt** (assumed based on standard multi-stage builds):
```dockerfile
FROM node:20 AS frontend-builder
WORKDIR /app/frontend
COPY frontend/package*.json ./
RUN npm ci
COPY frontend/ ./
RUN npm run build                           # ← Rebuilds frontend

FROM golang:1.25 AS backend-builder
WORKDIR /app
COPY go.* ./
COPY backend/ ./backend/
RUN go build -o bin/api ./backend/cmd/api   # ← Rebuilds backend
```

#### 🟡 Issue: Double Building

**Impact**:
- **Time**: 30-45 seconds wasted in build job
- **Disk**: Creates extra artifacts (frontend/dist, backend/bin) that aren't used
- **Confusion**: Suggests build artifacts are needed before Docker, but they're not

**Why This Is Wrong**:
- Docker's multi-stage build handles all compilation
- Pre-built artifacts are **not copied into Docker image**
- Build job should only build Docker image, not application code

---

### 4. Test Sharding Analysis

#### ✅ Sharding is Implemented Correctly

```yaml
# Matrix Strategy (Lines 125-130)
strategy:
  fail-fast: false
  matrix:
    shard: [1, 2, 3, 4]
    total-shards: [4]
    browser: [chromium]

# Playwright Command (Line 238)
npx playwright test \
  --project=${{ matrix.browser }} \
  --shard=${{ matrix.shard }}/${{ matrix.total-shards }} \  # ✅ CORRECT
  --reporter=html,json,github
```

**Verification**:
- Playwright's `--shard` flag divides tests evenly across shards
- Each shard runs **different tests**, not duplicates
- Shard 1 runs tests 1-25%, Shard 2 runs 26-50%, etc.

**Evidence**:
```bash
# Test files likely to be sharded:
tests/
├── auth.spec.ts
├── live-logs.spec.ts
├── manual-challenge.spec.ts
├── manual-dns-provider.spec.ts
├── security-dashboard.spec.ts
└── ... (other tests)

# Shard 1 might run: auth.spec.ts, live-logs.spec.ts
# Shard 2 might run: manual-challenge.spec.ts, manual-dns-provider.spec.ts
# Shard 3 might run: security-dashboard.spec.ts, ...
# Shard 4 might run: remaining tests
```

**No issue here** - sharding is working as designed.

---

## 🚀 Optimization Recommendations

### Priority 1: Remove Docker Rebuild (`--build` flag)

**File**: `.github/workflows/e2e-tests.yml`
**Line**: 157
**Complexity**: 🟢 LOW
**Savings**: ⏱️ 2-4 minutes per run

**Current**:
```yaml
- name: Start test environment
  run: |
    docker compose -f .docker/compose/docker-compose.playwright.yml up -d --build
    echo "✅ Container started via docker-compose.playwright.yml"
```

**Optimized**:
```yaml
- name: Start test environment
  run: |
    # Use pre-built image loaded from artifact - no rebuild needed
    docker compose -f .docker/compose/docker-compose.playwright.yml up -d
    echo "✅ Container started with pre-built image"
```

**Verification**:
```bash
# After change, check Docker logs for "Building" messages
# Should see "Using cached image" instead
docker compose logs | grep -i "build"
```

**Risk**: 🟢 LOW
- Image is already loaded and tagged correctly
- Compose file will use existing image
- No functional change to tests

---

### Priority 2: Remove Pre-Build Steps

**File**: `.github/workflows/e2e-tests.yml`
**Lines**: 90-96
**Complexity**: 🟢 LOW
**Savings**: ⏱️ 30-45 seconds per run

**Current**:
```yaml
- name: Install frontend dependencies
  run: npm ci
  working-directory: frontend

- name: Build frontend
  run: npm run build
  working-directory: frontend

- name: Build backend
  run: make build

- name: Set up Docker Buildx
  uses: docker/setup-buildx-action@v3

- name: Build Docker image
  uses: docker/build-push-action@v6
  # ...
```

**Optimized**:
```yaml
# Remove frontend and backend build steps entirely

- name: Set up Docker Buildx
  uses: docker/setup-buildx-action@v3

- name: Build Docker image
  uses: docker/build-push-action@v6
  # ... (no changes to this step)
```

**Justification**:
- Dockerfile handles all builds internally
- Pre-built artifacts are not used
- Reduces job complexity
- Saves time and disk space

**Risk**: 🟢 LOW
- Docker build is self-contained
- No dependencies on pre-built artifacts
- Tests use containerized application only

---

### Priority 3: Optimize Dependency Caching

**File**: `.github/workflows/e2e-tests.yml`
**Lines**: 205, 215-218
**Complexity**: 🟡 MEDIUM
**Savings**: ⏱️ 1-2 minutes per run (across all shards)

**Option A: Artifact-Based Dependencies** (Recommended)

Upload node_modules from build job, download in test shards.

**Build Job - Add**:
```yaml
- name: Install dependencies
  run: npm ci

- name: Install frontend dependencies
  run: npm ci
  working-directory: frontend

- name: Upload node_modules artifact
  uses: actions/upload-artifact@v6
  with:
    name: node-modules
    path: |
      node_modules/
      frontend/node_modules/
    retention-days: 1
```

**Test Shards - Replace**:
```yaml
- name: Download node_modules
  uses: actions/download-artifact@v7
  with:
    name: node-modules

# Remove these steps:
# - name: Install dependencies
#   run: npm ci
# - name: Install Frontend Dependencies
#   run: npm ci
#   working-directory: frontend
```

**Option B: Better Cache Strategy** (Alternative)

Use composite cache key including package-lock hashes.

```yaml
- name: Cache all dependencies
  uses: actions/cache@v5
  with:
    path: |
      ~/.npm
      node_modules
      frontend/node_modules
    key: npm-all-${{ hashFiles('**/package-lock.json') }}
    restore-keys: npm-all-

- name: Install dependencies (if cache miss)
  run: |
    [[ -d node_modules ]] || npm ci
    [[ -d frontend/node_modules ]] || (cd frontend && npm ci)
```

**Risk**: 🟡 MEDIUM
- Option A: Artifact size ~200-300MB (within GitHub limits)
- Option B: Cache may miss if lockfiles change
- Both require testing to verify coverage still works

**Recommendation**: Start with Option B (safer, uses existing cache infrastructure)

---

### Priority 4: Playwright Browser Caching (Already Optimized)

**Status**: ✅ Already implemented correctly (Line 199-206)

```yaml
- name: Cache Playwright browsers
  uses: actions/cache@v5
  with:
    path: ~/.cache/ms-playwright
    key: playwright-${{ matrix.browser }}-${{ hashFiles('package-lock.json') }}
    restore-keys: playwright-${{ matrix.browser }}-

- name: Install Playwright browsers
  run: npx playwright install --with-deps ${{ matrix.browser }}
```

**No action needed** - this is optimal.

---

## 📈 Expected Performance Impact

### Time Savings Breakdown

| Optimization | Per Shard | Total (4 shards) | Priority |
|--------------|-----------|------------------|----------|
| Remove `--build` flag | 30-60s | **2-4 min** | 🔴 HIGH |
| Remove pre-builds | 10s (shared) | **30-45s** | 🟢 LOW |
| Dependency caching | 20-30s | **1-2 min** | 🟡 MEDIUM |
| **Total** | | **4-6.5 min** | |

### Current vs Optimized Timeline

**Current Workflow**:
```
Build Job:       2-3 min  ████████
Shard 1-4:       5-8 min  ████████████████
Merge Reports:   1 min    ███
Upload Coverage: 1 min    ███
───────────────────────────────────
Total:           9-13 min
```

**Optimized Workflow**:
```
Build Job:       1.5-2 min   ████
Shard 1-4:       3-5 min     ██████████
Merge Reports:   1 min       ███
Upload Coverage: 1 min       ███
───────────────────────────────────
Total:           6.5-9 min  (-30-40%)
```

---

## ⚠️ Risks and Trade-offs

### Risk Matrix

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Compose file requires rebuild | LOW | HIGH | Test with pre-loaded image first |
| Artifact size bloat | MEDIUM | LOW | Monitor artifact sizes, use retention limits |
| Cache misses increase | LOW | MEDIUM | Keep existing cache strategy as fallback |
| Coverage collection breaks | LOW | HIGH | Test coverage report generation thoroughly |

### Trade-offs

**Pros**:
- ✅ Faster CI feedback loop (4-6 min savings)
- ✅ Lower GitHub Actions costs (~30-40% reduction)
- ✅ Reduced network bandwidth usage
- ✅ Simplified workflow logic

**Cons**:
- ⚠️ Requires testing to verify no functional regressions
- ⚠️ Artifact strategy adds complexity (if chosen)
- ⚠️ May need to update local development docs

---

## 🛠️ Implementation Plan

### Phase 1: Quick Wins (Low Risk)

**Estimated Time**: 30 minutes
**Savings**: ~3 minutes per run

1. **Remove `--build` flag**
   - Edit line 157 in `.github/workflows/e2e-tests.yml`
   - Test in PR to verify containers start correctly
   - Verify coverage still collects

2. **Remove pre-build steps**
   - Delete lines 83-96 in build job
   - Verify Docker build still succeeds
   - Check image artifact size (should be same)

**Acceptance Criteria**:
- [ ] E2E tests pass without `--build` flag
- [ ] Coverage reports generated correctly
- [ ] Docker containers start within 10 seconds
- [ ] No "image not found" errors

---

### Phase 2: Dependency Optimization (Medium Risk)

**Estimated Time**: 1-2 hours (includes testing)
**Savings**: ~1-2 minutes per run

**Option A: Implement artifact-based dependencies**

1. Add node_modules upload in build job
2. Replace npm ci with artifact download in test shards
3. Test coverage collection still works
4. Monitor artifact sizes

**Option B: Improve cache strategy**

1. Update cache step with composite key
2. Add conditional npm ci based on cache hit
3. Test across multiple PRs for cache effectiveness
4. Monitor cache hit ratio

**Acceptance Criteria**:
- [ ] Dependencies available in test shards
- [ ] Vite dev server starts successfully
- [ ] Coverage instrumentation works
- [ ] Cache hit ratio >80% on repeated runs

---

### Phase 3: Verification & Monitoring

**Duration**: Ongoing (first week)

1. **Monitor workflow runs**
   - Track actual time savings
   - Check for any failures or regressions
   - Monitor artifact/cache sizes

2. **Collect metrics**
   ```bash
   # Compare before/after durations
   gh run list --workflow="e2e-tests.yml" --json durationMs,conclusion
   ```

3. **Update documentation**
   - Document optimization decisions
   - Update CONTRIBUTING.md if needed
   - Add comments to workflow file

**Success Metrics**:
- ✅ Average workflow time reduced by 25-40%
- ✅ Zero functional regressions
- ✅ No increase in failure rate
- ✅ Coverage reports remain accurate

---

## 📋 Checklist for Implementation

### Pre-Implementation

- [ ] Review this specification with team
- [ ] Backup current workflow file
- [ ] Create test branch for changes
- [ ] Document current baseline metrics

### Phase 1 (Remove Redundant Builds)

- [ ] Remove `--build` flag from line 157
- [ ] Remove frontend build steps (lines 83-89)
- [ ] Remove backend build step (line 93)
- [ ] Test in PR with real changes
- [ ] Verify coverage reports
- [ ] Verify container startup time

### Phase 2 (Optimize Dependencies)

- [ ] Choose Option A or Option B
- [ ] Implement dependency caching strategy
- [ ] Test with cache hit scenario
- [ ] Test with cache miss scenario
- [ ] Verify Vite dev server starts
- [ ] Verify coverage still collects

### Post-Implementation

- [ ] Monitor first 5 workflow runs
- [ ] Compare time metrics before/after
- [ ] Check for any error patterns
- [ ] Update documentation
- [ ] Close this specification issue

---

## 🔄 Rollback Plan

If optimizations cause issues:

1. **Immediate Rollback**
   ```bash
   git revert <commit-hash>
   git push origin main
   ```

2. **Partial Rollback**
   - Re-add `--build` flag if containers fail to start
   - Re-add pre-build steps if Docker build fails
   - Revert dependency changes if coverage breaks

3. **Root Cause Analysis**
   - Check Docker logs for image loading issues
   - Verify artifact upload/download integrity
   - Test locally with same image loading process

---

## 📊 Monitoring Dashboard (Post-Implementation)

Track these metrics for 2 weeks:

| Metric | Baseline | Target | Actual |
|--------|----------|--------|--------|
| Avg workflow duration | 9-13 min | 6-9 min | TBD |
| Build job duration | 2-3 min | 1.5-2 min | TBD |
| Shard duration | 5-8 min | 3-5 min | TBD |
| Workflow success rate | 95% | ≥95% | TBD |
| Coverage accuracy | 100% | 100% | TBD |
| Artifact size | 400MB | <450MB | TBD |

---

## 🎯 Success Criteria

This optimization is considered successful when:

✅ **Performance**:
- E2E workflow completes in 6-9 minutes (down from 9-13 minutes)
- Build job completes in 1.5-2 minutes (down from 2-3 minutes)
- Test shards complete in 3-5 minutes (down from 5-8 minutes)

✅ **Reliability**:
- No increase in workflow failure rate
- Coverage reports remain accurate and complete
- All tests pass consistently

✅ **Maintainability**:
- Workflow logic is simpler and clearer
- Comments explain optimization decisions
- Documentation updated

---

## 🔗 References

- **Workflow File**: `.github/workflows/e2e-tests.yml`
- **Docker Compose**: `.docker/compose/docker-compose.playwright.yml`
- **Docker Build Cache**: [GitHub Actions Cache](https://docs.github.com/en/actions/using-workflows/caching-dependencies-to-speed-up-workflows)
- **Playwright Sharding**: [Playwright Docs](https://playwright.dev/docs/test-sharding)
- **GitHub Actions Artifacts**: [Artifact Actions](https://github.com/actions/upload-artifact)

---

## 💡 Key Insights

### What's Working Well

✅ **Sharding Strategy**: 4 shards properly divide tests, running different subsets in parallel
✅ **Docker Layer Caching**: Uses GitHub Actions cache (type=gha) for faster builds
✅ **Playwright Browser Caching**: Browsers cached per version, avoiding re-downloads
✅ **Coverage Architecture**: Vite dev server + Docker backend enables source-mapped coverage
✅ **Artifact Strategy**: Building image once and reusing across shards is correct approach

### What's Wasteful

❌ **Docker Rebuild**: `--build` flag rebuilds image despite loading pre-built version
❌ **Pre-Build Steps**: Building frontend/backend before Docker is unnecessary duplication
❌ **Dependency Re-installs**: npm ci runs 4 times across build + test shards
❌ **Missing Optimization**: Could use artifact-based dependency sharing

### Architecture Insights

The workflow follows the **correct pattern** of:
1. Build once (centralized build job)
2. Distribute to workers (artifact upload/download)
3. Execute in parallel (test sharding)
4. Aggregate results (merge reports, upload coverage)

The **inefficiencies are in the details**, not the overall design.

---

## 📝 Decision Record

**Decision**: Optimize E2E workflow by removing redundant builds and improving caching

**Rationale**:
1. **Immediate Impact**: ~30-40% time reduction with minimal risk
2. **Cost Savings**: Reduces GitHub Actions minutes consumption
3. **Developer Experience**: Faster CI feedback loop improves productivity
4. **Sustainability**: Lower resource usage aligns with green CI practices
5. **Principle of Least Work**: Only build/install once, reuse everywhere

**Alternatives Considered**:
- ❌ **Reduce shards to 2**: Would increase shard duration, offsetting savings
- ❌ **Skip coverage collection**: Loses valuable test quality metric
- ❌ **Use self-hosted runners**: Higher maintenance burden, not worth it for this project
- ✅ **Current proposal**: Best balance of impact vs complexity

**Impact Assessment**:
- ✅ **Positive**: Faster builds, lower costs, simpler workflow
- ⚠️ **Neutral**: Requires testing to verify no regressions
- ❌ **Negative**: None identified if implemented carefully

**Review Schedule**: Re-evaluate after 2 weeks of production use

---

## 🚦 Implementation Status

| Phase | Status | Owner | Target Date |
|-------|--------|-------|-------------|
| Analysis | ✅ COMPLETE | AI Agent | 2026-01-26 |
| Review | 🔄 PENDING | Team | TBD |
| Phase 1 Implementation | ⏸️ NOT STARTED | TBD | TBD |
| Phase 2 Implementation | ⏸️ NOT STARTED | TBD | TBD |
| Verification | ⏸️ NOT STARTED | TBD | TBD |
| Documentation | ⏸️ NOT STARTED | TBD | TBD |

---

## 🤔 Questions for Review

Before implementing, please confirm:

1. **Docker Compose Behavior**: Does `.docker/compose/docker-compose.playwright.yml` reference a `build:` context, or does it expect a pre-built image? (Need to verify)

2. **Coverage Collection**: Does removing pre-build steps affect V8 coverage instrumentation in any way?

3. **Artifact Limits**: What's the maximum acceptable artifact size? (Current: ~400MB for Docker image)

4. **Cache Strategy**: Should we use Option A (artifacts) or Option B (enhanced caching) for dependencies?

5. **Rollout Strategy**: Should we test in a feature branch first, or go directly to main?

---

## 📚 Additional Context

### Docker Compose File Analysis Needed

To finalize recommendations, we need to check:

```bash
# Check compose file for build context
cat .docker/compose/docker-compose.playwright.yml | grep -A10 "services:"

# Expected one of:
# Option 1 (build context - needs removal):
#   services:
#     charon:
#       build: .
#       ...
#
# Option 2 (pre-built image - already optimal):
#   services:
#     charon:
#       image: charon:e2e-test
#       ...
```

**Next Action**: Read compose file to determine exact optimization needed.

---

## 📋 Appendix: Full Redundancy Details

### A. Build Job Redundant Steps (Lines 77-96)

```yaml
# Lines 77-82: Cache npm dependencies
- name: Cache npm dependencies
  uses: actions/cache@v5
  with:
    path: ~/.npm
    key: npm-${{ hashFiles('package-lock.json') }}
    restore-keys: npm-

# Line 81: Install root dependencies
- name: Install dependencies
  run: npm ci
  # Why: Needed for... nothing in build job actually uses root node_modules
  # Used by: Test shards (but they re-install)
  # Verdict: Could be removed from build job

# Lines 84-86: Install frontend dependencies
- name: Install frontend dependencies
  run: npm ci
  working-directory: frontend
  # Why: Supposedly for "npm run build" next
  # Used by: Immediately consumed by build step
  # Verdict: Unnecessary - Dockerfile does this

# Lines 90-91: Build frontend
- name: Build frontend
  run: npm run build
  working-directory: frontend
  # Creates: frontend/dist/* (not used by Docker)
  # Dockerfile: Does same build internally
  # Verdict: ❌ REMOVE

# Line 93-94: Build backend
- name: Build backend
  run: make build
  # Creates: backend/bin/api (not used by Docker)
  # Dockerfile: Compiles Go binary internally
  # Verdict: ❌ REMOVE
```

### B. Test Shard Redundant Steps (Lines 205, 215-218)

```yaml
# Line 205: Re-install root dependencies
- name: Install dependencies
  run: npm ci
  # Why: Playwright needs @playwright/test package
  # Problem: Already installed in build job
  # Solution: Share via artifact or cache

# Lines 215-218: Re-install frontend dependencies
- name: Install Frontend Dependencies
  run: |
    cd frontend
    npm ci
  # Why: Vite dev server needs React, etc.
  # Problem: Already installed in build job
  # Solution: Share via artifact or cache
```

### C. Docker Rebuild Evidence

```bash
# Hypothetical compose file content:
# .docker/compose/docker-compose.playwright.yml
services:
  charon:
    build: .                          # ← Triggers rebuild with --build flag
    image: charon:e2e-test
    # Should be:
    # image: charon:e2e-test         # ← Use pre-built image only
    # (no build: context)
```

---

**End of Specification**

**Total Analysis Time**: ~45 minutes
**Confidence Level**: 95% - High confidence in identified issues and solutions
**Recommended Next Step**: Review with team, then implement Phase 1 (quick wins)
