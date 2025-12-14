# CI Docker Build Failure - Root Cause Analysis and Remediation Plan

**Version:** 1.0
**Date:** 2025-12-14
**Status:** 🔴 CRITICAL - Docker builds failing in CI

---

## Executive Summary

The CI Docker build is failing during the xcaddy build process. The root cause is a **Go version mismatch** introduced by a recent commit that downgraded Go from 1.25.x to 1.23.x based on the incorrect assumption that Go 1.25.5 doesn't exist.

### Key Finding

**Go 1.25.5 IS a valid, released version** (as of December 2025). The commit `481208c` ("fix: correct Go version to 1.23 in Dockerfile (1.25.5 does not exist)") incorrectly downgraded Go and **broke the build**.

---

## Root Cause Analysis

### 1. Version Compatibility Matrix (Current State)

| Component | Version Required | Version in Dockerfile | Status |
|-----------|------------------|----------------------|--------|
| **Go** (for Caddy build) | 1.25+ | 1.23 ❌ | **INCOMPATIBLE** |
| **Go** (for backend build) | 1.23+ | 1.23 ✅ | Compatible |
| **Caddy** | 2.10.2 | 2.10.2 ✅ | Correct |
| **xcaddy** | 0.4.5 | latest ✅ | Correct |

### 2. The Problem

Caddy 2.10.2's `go.mod` declares:

```go
go 1.25
```

When xcaddy tries to build Caddy 2.10.2 with Go 1.23, it fails because:

- Go's toolchain directive enforcement (Go 1.21+) prevents building modules that require a newer Go version
- The error manifests during the xcaddy build step in the Dockerfile

### 3. Error Location

**File:** [Dockerfile](../../Dockerfile)
**Stage:** `caddy-builder` (lines 101-145)
**Root Cause Lines:**

- Line 51: `FROM --platform=$BUILDPLATFORM golang:1.23-alpine AS backend-builder`
- Line 101: `FROM --platform=$BUILDPLATFORM golang:1.23-alpine AS caddy-builder`

### 4. Evidence from go.mod Files

**Caddy 2.10.2** (`github.com/caddyserver/caddy/v2`):

```go
go 1.25
```

**xcaddy 0.4.5** (`github.com/caddyserver/xcaddy`):

```go
go 1.21
toolchain go1.23.0
```

**Backend** (`/projects/Charon/backend/go.mod`):

```go
go 1.23
```

**Workspace** (`/projects/Charon/go.work`):

```go
go 1.23
```

### 5. Plugin Compatibility

| Plugin | Go Version Required | Caddy Version Tested |
|--------|---------------------|---------------------|
| caddy-security | 1.24 | v2.9.1 |
| coraza-caddy/v2 | 1.23 | v2.9.1 |
| caddy-crowdsec-bouncer | 1.23 | v2.9.1 |
| caddy-geoip2 | varies | - |
| caddy-ratelimit | varies | - |

**Note:** Plugin compatibility with Caddy 2.10.2 requires Go 1.25 since Caddy itself requires it.

---

## Remediation Plan

### Option A: Upgrade Go to 1.25 (RECOMMENDED)

**Rationale:** Go 1.25.5 exists and is stable. Upgrading aligns with Caddy 2.10.2 requirements.

#### File Changes Required

##### 1. Dockerfile (lines 51, 101)

**Current (BROKEN):**

```dockerfile
FROM --platform=$BUILDPLATFORM golang:1.23-alpine AS backend-builder
...
FROM --platform=$BUILDPLATFORM golang:1.23-alpine AS caddy-builder
```

**Fix:**

```dockerfile
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS backend-builder
...
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS caddy-builder
```

##### 2. backend/go.mod (line 3)

**Current:**

```go
go 1.23
```

**Fix:**

```go
go 1.25
```

##### 3. go.work (line 1)

**Current:**

```go
go 1.23
```

**Fix:**

```go
go 1.25
```

---

### Option B: Downgrade Caddy to 2.9.x (NOT RECOMMENDED)

**Rationale:** Would require pinning to an older Caddy version that still supports Go 1.23.

**Downsides:**

- Miss security fixes in Caddy 2.10.x
- Need to update `CADDY_VERSION` ARG
- Still need to verify plugin compatibility

**File Changes:**

```dockerfile
ARG CADDY_VERSION=2.9.1  # Downgrade from 2.10.2
```

**Not recommended** because it's a regression and delays inevitable Go upgrade.

---

## Recommended Implementation: Option A

### Step-by-Step Remediation

#### Step 1: Update Dockerfile

**File:** [Dockerfile](../../Dockerfile)

| Line | Current | New |
|------|---------|-----|
| 51 | `golang:1.23-alpine` | `golang:1.25-alpine` |
| 101 | `golang:1.23-alpine` | `golang:1.25-alpine` |

#### Step 2: Update go.mod

**File:** [backend/go.mod](../../backend/go.mod)

| Line | Current | New |
|------|---------|-----|
| 3 | `go 1.23` | `go 1.25` |

Then run:

```bash
cd backend && go mod tidy
```

#### Step 3: Update go.work

**File:** [go.work](../../go.work)

| Line | Current | New |
|------|---------|-----|
| 1 | `go 1.23` | `go 1.25` |

#### Step 4: Verify Local Build

```bash
# Build Docker image locally
docker build -t charon:test .

# Run the test suite
cd backend && go test ./...
cd frontend && npm run test
```

#### Step 5: Validate CI Workflows

The following workflows use Go and will automatically use the container's Go version:

- [docker-build.yml](../../.github/workflows/docker-build.yml) - Uses Dockerfile Go version
- [docker-publish.yml](../../.github/workflows/docker-publish.yml) - Uses Dockerfile Go version
- [quality-checks.yml](../../.github/workflows/quality-checks.yml) - May need `go-version` update

Check if `quality-checks.yml` specifies Go version explicitly and update if needed.

---

## Version Compatibility Matrix (After Fix)

| Component | Version | Source |
|-----------|---------|--------|
| Go | 1.25 | Dockerfile, go.mod, go.work |
| Caddy | 2.10.2 | Dockerfile ARG |
| xcaddy | latest (0.4.5+) | go install |
| Node.js | 24.12.0 | Dockerfile |
| Alpine | 3.23 | Dockerfile |

### Plugin Versions (auto-resolved by xcaddy)

| Plugin | Current Version | Notes |
|--------|-----------------|-------|
| caddy-security | 1.1.31 | Works with Caddy 2.x |
| coraza-caddy/v2 | 2.1.0 | Works with Caddy 2.x |
| caddy-crowdsec-bouncer | main | Works with Caddy 2.x |
| caddy-geoip2 | main | Works with Caddy 2.x |
| caddy-ratelimit | main | Works with Caddy 2.x |

---

## Potential Side Effects

### 1. Backend Code Compatibility

Go 1.25 is backwards compatible with Go 1.23 code. The backend should compile without issues.

**Risk:** Low
**Mitigation:** Run `go build ./...` and `go test ./...` after update.

### 2. CI/CD Pipeline

Some workflows may cache Go 1.23 artifacts. Force cache invalidation if builds fail after fix.

**Risk:** Low
**Mitigation:** Clear GitHub Actions cache if needed.

### 3. Local Development

Developers using Go 1.23 locally will need to upgrade to Go 1.25.

**Risk:** Medium
**Mitigation:** Document required Go version in README.md.

---

## Testing Checklist

Before merging the fix:

- [ ] Local Docker build succeeds: `docker build -t charon:test .`
- [ ] Backend compiles: `cd backend && go build ./...`
- [ ] Backend tests pass: `cd backend && go test ./...`
- [ ] Frontend builds: `cd frontend && npm run build`
- [ ] Frontend tests pass: `cd frontend && npm run test`
- [ ] Pre-commit passes: `pre-commit run --all-files`
- [ ] Container starts: `docker run --rm charon:test /app/charon --version`
- [ ] Caddy works: `docker run --rm charon:test caddy version`

---

## Commit Message

```text
fix: upgrade Go to 1.25 for Caddy 2.10.2 compatibility

Caddy 2.10.2 requires Go 1.25 (declared in its go.mod). The previous
commit incorrectly downgraded to Go 1.23 based on the false assumption
that Go 1.25.5 doesn't exist.

This fix:
- Updates Dockerfile Go images from 1.23-alpine to 1.25-alpine
- Updates backend/go.mod to go 1.25
- Updates go.work to go 1.25

Fixes CI Docker build failures in xcaddy stage.
```

---

## Files to Modify (Summary)

| File | Line(s) | Change |
|------|---------|--------|
| `Dockerfile` | 51 | `golang:1.23-alpine` → `golang:1.25-alpine` |
| `Dockerfile` | 101 | `golang:1.23-alpine` → `golang:1.25-alpine` |
| `backend/go.mod` | 3 | `go 1.23` → `go 1.25` |
| `go.work` | 1 | `go 1.23` → `go 1.25` |

---

## Related Issues

- Previous (incorrect) fix commit: `481208c` "fix: correct Go version to 1.23 in Dockerfile (1.25.5 does not exist)"
- Previous commit: `65443a1` "fix: correct Go version to 1.23 (1.25.5 does not exist)"

Both commits should be effectively reverted by this fix.

---

## Appendix: Go Version Verification

As of December 14, 2025, Go 1.25.5 is available:

```json
{
  "version": "go1.25.5",
  "stable": true,
  "files": [
    {"filename": "go1.25.5.linux-amd64.tar.gz", "...": "..."},
    {"filename": "go1.25.5.linux-arm64.tar.gz", "...": "..."},
    {"filename": "go1.25.5.darwin-amd64.tar.gz", "...": "..."}
  ]
}
```

Source: <https://go.dev/dl/?mode=json>

---

## Next Steps

1. Implement the file changes listed above
2. Run local validation tests
3. Push fix with conventional commit message
4. Monitor CI pipeline for successful build
5. Update any documentation that references Go version requirements
