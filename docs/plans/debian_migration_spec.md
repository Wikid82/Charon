# Alpine to Debian Slim Migration Specification

> **Version**: 1.0.0
> **Created**: 2026-01-18
> **Status**: PLANNING
> **Author**: Planning Agent

---

## Executive Summary

### Security Rationale

The user has identified a critical CVE in Alpine Linux that necessitates migrating the Docker base images from Alpine to Debian slim. This migration addresses:

1. **Critical CVE Mitigation**: Immediate resolution of the identified Alpine Linux vulnerability
2. **Broader Security Posture**: Debian's larger security team and faster CVE response times
3. **glibc vs musl Compatibility**: Eliminates potential musl libc edge cases that can cause subtle bugs in Go binaries with CGO
4. **Long-term Maintainability**: Debian slim provides a battle-tested, stable base with predictable security update cycles

### Key Benefits of Debian Slim

| Aspect | Alpine | Debian Slim | Advantage |
|--------|--------|-------------|-----------|
| Security Updates | Community-driven | Dedicated security team (Debian Security Team) | Faster CVE patches |
| C Library | musl libc | glibc | Better compatibility with CGO |
| Package Availability | ~10k packages | ~60k packages | More comprehensive |
| DNS Resolution | musl DNS bugs known | glibc mature DNS | More reliable |
| Image Size | ~5MB base | ~25MB base | Alpine smaller, but acceptable trade-off |

---

## Current State Analysis

### Dockerfile Structure Overview

The current `Dockerfile` is a multi-stage build with the following Alpine-based stages:

#### Builder Stages (Alpine-based)

| Stage | Base Image | Purpose |
|-------|------------|---------|
| `xx` | `tonistiigi/xx:1.9.0` | Cross-compilation helpers (unchanged) |
| `frontend-builder` | `node:24.13.0-alpine` | Build React frontend |
| `backend-builder` | `golang:1.25-alpine` | Build Go backend with CGO |
| `caddy-builder` | `golang:1.25-alpine` | Build Caddy with plugins |
| `crowdsec-builder` | `golang:1.25.6-alpine` | Build CrowdSec from source |
| `crowdsec-fallback` | `alpine:3.23` | Fallback binary download |

#### Runtime Stage (Alpine-based)

| Stage | Base Image | Purpose |
|-------|------------|---------|
| Final runtime | `alpine:3.23` (via `CADDY_IMAGE` ARG) | Production runtime |

### Alpine Packages Currently Installed

#### Builder Stage Packages (apk)

```dockerfile
# backend-builder
apk add --no-cache clang lld
xx-apk add --no-cache gcc musl-dev sqlite-dev

# caddy-builder
apk add --no-cache git

# crowdsec-builder
apk add --no-cache git clang lld
xx-apk add --no-cache gcc musl-dev

# crowdsec-fallback
apk add --no-cache curl tar
```

#### Runtime Stage Packages (apk)

```dockerfile
# Final runtime image
apk --no-cache add bash ca-certificates sqlite-libs sqlite tzdata curl gettext su-exec libcap-utils
apk --no-cache upgrade
apk --no-cache upgrade c-ares
```

### Alpine-Specific Commands in Dockerfile

1. **User/Group Creation**:
   ```dockerfile
   RUN addgroup -g 1000 charon && \
       adduser -D -u 1000 -G charon -h /app -s /sbin/nologin charon
   ```

2. **Package Management**:
   - `apk add --no-cache`
   - `apk --no-cache upgrade`
   - `xx-apk add --no-cache` (cross-compilation)

3. **Privilege Dropping**:
   - Uses `su-exec` (Alpine-specific lightweight sudo replacement)

### Alpine-Specific Commands in docker-entrypoint.sh

1. **User Group Management**:
   ```bash
   addgroup -g "$DOCKER_SOCK_GID" docker 2>/dev/null || true
   addgroup charon docker 2>/dev/null || true
   addgroup charon "$GROUP_NAME" 2>/dev/null || true
   ```

2. **File Statistics**:
   ```bash
   stat -c '%a' "$PLUGINS_DIR"  # Alpine stat syntax
   stat -c '%g' /var/run/docker.sock
   ```

### CI/CD Workflow References

Files referencing Alpine that need updates:

| File | Line | Reference |
|------|------|-----------|
| `.github/workflows/docker-build.yml` | 103-104 | `caddy:2-alpine` image pull |
| `.github/workflows/security-weekly-rebuild.yml` | 53-54 | `caddy:2-alpine` image pull |
| `.github/workflows/security-weekly-rebuild.yml` | 127 | `apk info` command for package check |

---

## Target State: Debian Slim Configuration

### Recommended Base Images

| Current Alpine Image | Debian Slim Replacement | Notes |
|---------------------|-------------------------|-------|
| `node:24.13.0-alpine` | `node:24.13.0-slim` | Node.js official slim variant |
| `golang:1.25-alpine` | `golang:1.25-bookworm` | Go official Debian variant |
| `golang:1.25.6-alpine` | `golang:1.25.6-bookworm` | CrowdSec builder |
| `alpine:3.23` | `debian:bookworm-slim` | Runtime image |
| `caddy:2-alpine` | Build Caddy ourselves | Already building from source |

### Package Mapping: Alpine → Debian

| Alpine Package | Debian Equivalent | Notes |
|----------------|-------------------|-------|
| `bash` | `bash` | Same |
| `ca-certificates` | `ca-certificates` | Same |
| `sqlite-libs` | `libsqlite3-0` | Runtime library |
| `sqlite` | `sqlite3` | CLI tool |
| `sqlite-dev` | `libsqlite3-dev` | Build dependency |
| `tzdata` | `tzdata` | Same |
| `curl` | `curl` | Same |
| `gettext` | `gettext-base` | Smaller variant with envsubst |
| `su-exec` | `gosu` | Debian equivalent |
| `libcap-utils` | `libcap2-bin` | Contains setcap |
| `clang` | `clang` | Same |
| `lld` | `lld` | Same |
| `gcc` | `gcc` | Same (may need build-essential) |
| `musl-dev` | `libc6-dev` | glibc development files |
| `git` | `git` | Same |
| `tar` | `tar` | Usually pre-installed |
| `c-ares` | `libc-ares2` | Async DNS library |

### User/Group Creation Syntax Changes

| Operation | Alpine | Debian |
|-----------|--------|--------|
| Create group | `addgroup -g 1000 charon` | `groupadd -g 1000 charon` |
| Create user | `adduser -D -u 1000 -G charon -h /app -s /sbin/nologin charon` | `useradd -u 1000 -g charon -d /app -s /usr/sbin/nologin -M charon` |
| Add to group | `addgroup charon docker` | `usermod -aG docker charon` |

> **Note**: Debian uses `/usr/sbin/nologin` instead of Alpine's `/sbin/nologin`

### Entrypoint Script Changes

The `docker-entrypoint.sh` requires these changes:

1. **Replace `addgroup`/`adduser` with `groupadd`/`useradd`**
2. **Replace `su-exec` with `gosu`**
3. **Update stat command syntax** (BSD vs GNU - Debian uses GNU which is same)

---

## Detailed Migration Steps

### Phase 1: Builder Stage Migrations

#### Step 1.1: Frontend Builder

**File**: `Dockerfile`
**Lines**: 27-47

```dockerfile
# BEFORE (Alpine)
FROM --platform=$BUILDPLATFORM node:24.13.0-alpine AS frontend-builder

# AFTER (Debian slim)
FROM --platform=$BUILDPLATFORM node:24.13.0-slim AS frontend-builder
```

**Notes**:
- No package installation changes needed (npm handles dependencies)
- Environment variables remain the same

#### Step 1.2: Backend Builder

**File**: `Dockerfile`
**Lines**: 49-143

```dockerfile
# BEFORE (Alpine)
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS backend-builder
# ...
RUN apk add --no-cache clang lld
RUN xx-apk add --no-cache gcc musl-dev sqlite-dev

# AFTER (Debian)
FROM --platform=$BUILDPLATFORM golang:1.25-bookworm AS backend-builder
# ...
RUN apt-get update && apt-get install -y --no-install-recommends \
    clang lld \
    && rm -rf /var/lib/apt/lists/*
```

**Critical Change - CGO with glibc**:
- Remove the clang wrapper workaround for ARM64 gold linker (lines 67-91)
- glibc environments handle this natively
- Change `xx-apk` to cross-compilation apt packages or use `TARGETPLATFORM` specific installs

**xx-go Cross Compilation Notes**:
- The `xx` helper supports Debian-based images
- Replace `xx-apk` with appropriate Debian cross-compilation setup

#### Step 1.3: Caddy Builder

**File**: `Dockerfile`
**Lines**: 145-202

```dockerfile
# BEFORE (Alpine)
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS caddy-builder
# ...
RUN apk add --no-cache git

# AFTER (Debian)
FROM --platform=$BUILDPLATFORM golang:1.25-bookworm AS caddy-builder
# ...
RUN apt-get update && apt-get install -y --no-install-recommends git \
    && rm -rf /var/lib/apt/lists/*
```

#### Step 1.4: CrowdSec Builder

**File**: `Dockerfile`
**Lines**: 204-256

```dockerfile
# BEFORE (Alpine)
FROM --platform=$BUILDPLATFORM golang:1.25.6-alpine AS crowdsec-builder
# ...
RUN apk add --no-cache git clang lld
RUN xx-apk add --no-cache gcc musl-dev

# AFTER (Debian)
FROM --platform=$BUILDPLATFORM golang:1.25.6-bookworm AS crowdsec-builder
# ...
RUN apt-get update && apt-get install -y --no-install-recommends \
    git clang lld gcc libc6-dev \
    && rm -rf /var/lib/apt/lists/*
```

#### Step 1.5: CrowdSec Fallback

**File**: `Dockerfile`
**Lines**: 258-293

```dockerfile
# BEFORE (Alpine)
FROM alpine:3.23 AS crowdsec-fallback
# ...
RUN apk add --no-cache curl tar

# AFTER (Debian)
FROM debian:bookworm-slim AS crowdsec-fallback
# ...
RUN apt-get update && apt-get install -y --no-install-recommends \
    curl ca-certificates tar \
    && rm -rf /var/lib/apt/lists/*
```

> **⚠️ IMPORTANT**: Debian slim does NOT include `tar` by default. It must be explicitly installed for CrowdSec binary extraction.

### Phase 2: Runtime Stage Migration

#### Step 2.1: Base Image Change

**File**: `Dockerfile`
**Lines**: 23, 295-303

```dockerfile
# BEFORE (Alpine)
ARG CADDY_IMAGE=alpine:3.23
# ...
FROM ${CADDY_IMAGE}
# ...
RUN apk --no-cache add bash ca-certificates sqlite-libs sqlite tzdata curl gettext su-exec libcap-utils \
    && apk --no-cache upgrade \
    && apk --no-cache upgrade c-ares

# AFTER (Debian)
ARG CADDY_IMAGE=debian:bookworm-slim
# ...
FROM ${CADDY_IMAGE}
# ...
RUN apt-get update && apt-get install -y --no-install-recommends \
    bash ca-certificates libsqlite3-0 sqlite3 tzdata curl gettext-base gosu libcap2-bin libc-ares2 \
    && apt-get upgrade -y \
    && rm -rf /var/lib/apt/lists/*
```

#### Step 2.2: User Creation

**File**: `Dockerfile`
**Lines**: 307-308

```dockerfile
# BEFORE (Alpine)
RUN addgroup -g 1000 charon && \
    adduser -D -u 1000 -G charon -h /app -s /sbin/nologin charon

# AFTER (Debian)
RUN groupadd -g 1000 charon && \
    useradd -u 1000 -g charon -d /app -s /usr/sbin/nologin -M charon
```

> **⚠️ PATH CHANGE**: Debian uses `/usr/sbin/nologin` instead of Alpine's `/sbin/nologin`.

#### Step 2.3: setcap Command

**File**: `Dockerfile`
**Line**: 318

```dockerfile
# BEFORE (Alpine - same)
RUN setcap 'cap_net_bind_service=+ep' /usr/bin/caddy

# AFTER (Debian - same, but requires libcap2-bin)
RUN setcap 'cap_net_bind_service=+ep' /usr/bin/caddy
```

### Phase 3: Entrypoint Script Migration

**File**: `.docker/docker-entrypoint.sh`

> **⚠️ CRITICAL**: Debian slim does NOT include `curl`. The entrypoint uses curl for the Caddy readiness check. All `curl` calls must be replaced with `curl` equivalents.

#### Step 3.0: Replace curl with curl for Caddy Readiness Check

```bash
# BEFORE (Alpine - uses curl)
curl -q --spider http://localhost:2019/config/ || exit 1

# AFTER (Debian - uses curl)
curl -sf http://localhost:2019/config/ > /dev/null || exit 1
```

#### Step 3.1: Replace su-exec with gosu

```bash
# BEFORE (Alpine)
run_as_charon() {
    if is_root; then
        su-exec charon "$@"
    else
        "$@"
    fi
}

# AFTER (Debian)
run_as_charon() {
    if is_root; then
        gosu charon "$@"
    else
        "$@"
    fi
}
```

#### Step 3.2: Replace addgroup/adduser with groupadd/usermod

```bash
# BEFORE (Alpine)
addgroup -g "$DOCKER_SOCK_GID" docker 2>/dev/null || true
addgroup charon docker 2>/dev/null || true

# AFTER (Debian)
groupadd -g "$DOCKER_SOCK_GID" docker 2>/dev/null || true
usermod -aG docker charon 2>/dev/null || true
```

```bash
# BEFORE (Alpine)
addgroup charon "$GROUP_NAME" 2>/dev/null || true

# AFTER (Debian)
usermod -aG "$GROUP_NAME" charon 2>/dev/null || true
```

#### Step 3.3: stat Command (No Change Required)

Both Alpine and Debian use GNU coreutils `stat`, so the syntax remains:
```bash
stat -c '%a' "$PLUGINS_DIR"
stat -c '%g' /var/run/docker.sock
```

### Phase 4: CI/CD Workflow Updates

#### Step 4.1: docker-build.yml

**File**: `.github/workflows/docker-build.yml`
**Lines**: 103-104

```yaml
# BEFORE
run: |
  docker pull caddy:2-alpine
  DIGEST=$(docker inspect --format='{{index .RepoDigests 0}}' caddy:2-alpine)

# AFTER
# Remove this step entirely - we build Caddy from source
# Or update to pull debian:bookworm-slim for digest verification
run: |
  docker pull debian:bookworm-slim
  DIGEST=$(docker inspect --format='{{index .RepoDigests 0}}' debian:bookworm-slim)
```

#### Step 4.2: security-weekly-rebuild.yml

**File**: `.github/workflows/security-weekly-rebuild.yml`

**Lines 53-54** (Caddy digest):
```yaml
# Remove or update similar to docker-build.yml
```

**Lines 127-133** (Package version check):
```yaml
# BEFORE
- name: Check Alpine package versions
  run: |
    echo "Checking key security packages:" >> $GITHUB_STEP_SUMMARY
    docker run --rm --entrypoint "" ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}@${{ steps.build.outputs.digest }} \
      sh -c "apk update >/dev/null 2>&1 && apk info c-ares curl libcurl openssl" >> $GITHUB_STEP_SUMMARY

# AFTER
- name: Check Debian package versions
  run: |
    echo "Checking key security packages:" >> $GITHUB_STEP_SUMMARY
    docker run --rm --entrypoint "" ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}@${{ steps.build.outputs.digest }} \
      sh -c "dpkg -l | grep -E 'libc-ares|curl|libcurl|openssl|libssl'" >> $GITHUB_STEP_SUMMARY
```

### Phase 5: Cross-Compilation Considerations

#### xx Helper Compatibility

The `tonistiigi/xx` project supports both Alpine and Debian. Key changes:

1. **Remove xx-apk usage**: Replace with native apt-get for the target architecture
2. **CGO Cross-Compilation**: Debian has better cross-compilation toolchain support
3. **Remove Gold Linker Workaround**: The clang wrapper hack (lines 67-91) for Go 1.25 ARM64 can be removed

```dockerfile
# Debian cross-compilation setup (replaces xx-apk)
ARG TARGETARCH
RUN dpkg --add-architecture ${TARGETARCH} && \
    apt-get update && \
    apt-get install -y --no-install-recommends \
        gcc-$(dpkg-architecture -A ${TARGETARCH} -qDEB_TARGET_GNU_TYPE) \
        libc6-dev:${TARGETARCH} \
        libsqlite3-dev:${TARGETARCH} \
    && rm -rf /var/lib/apt/lists/*
```

**Alternative**: Continue using `xx` helper which has Debian support:
```dockerfile
COPY --from=xx / /
RUN xx-apt install -y libc6-dev libsqlite3-dev
```

---

## Testing Strategy

### Phase 1: Local Build Verification

1. **Build All Architectures**:
   ```bash
   docker buildx build --platform linux/amd64,linux/arm64 -t charon:debian-test .
   ```

2. **Verify Binary Execution**:
   ```bash
   docker run --rm charon:debian-test /app/charon --version
   docker run --rm charon:debian-test caddy version
   docker run --rm charon:debian-test cscli version
   ```

3. **Verify Package Installation**:
   ```bash
   docker run --rm --entrypoint bash charon:debian-test -c "which gosu setcap curl sqlite3"
   ```

### Phase 2: Functional Testing

1. **Run E2E Playwright Tests**:
   ```bash
   npx playwright test --project=chromium
   ```

2. **Run Backend Unit Tests**:
   ```bash
   make test-backend
   ```

3. **Run Docker Compose Stack**:
   ```bash
   docker compose -f .docker/compose/docker-compose.yml up -d
   # Verify all services start correctly
   curl http://localhost:8080/api/v1/health
   ```

### Phase 3: Security Verification

1. **Trivy Vulnerability Scan**:
   ```bash
   trivy image charon:debian-test --severity CRITICAL,HIGH
   ```

2. **Verify No Alpine CVE Present**:
   ```bash
   trivy image charon:debian-test | grep -i alpine
   # Should return nothing
   ```

3. **Verify User Permissions**:
   ```bash
   docker run --rm charon:debian-test id
   # Should show: uid=1000(charon) gid=1000(charon)
   ```

### Phase 4: Performance Validation

1. **Compare Image Sizes**:
   ```bash
   docker images | grep charon
   # Alpine: ~150MB, Debian: ~200MB (acceptable)
   ```

2. **Startup Time Comparison**:
   ```bash
   time docker run --rm charon:debian-test /app/charon --version
   ```

3. **Memory Usage Comparison**:
   ```bash
   docker stats --no-stream charon-container
   ```

---

## Rollback Plan

### Immediate Rollback

1. **Revert Dockerfile Changes**:
   ```bash
   git checkout main -- Dockerfile
   git checkout main -- .docker/docker-entrypoint.sh
   ```

2. **Rebuild with Alpine**:
   ```bash
   docker buildx build --no-cache -t charon:alpine-rollback .
   ```

### Staged Rollback

If issues are discovered post-deployment:

1. **Tag Current (Debian) Image**:
   ```bash
   docker tag ghcr.io/wikid82/charon:latest ghcr.io/wikid82/charon:debian-v1
   ```

2. **Push Previous Alpine Image**:
   ```bash
   docker tag ghcr.io/wikid82/charon:v{previous} ghcr.io/wikid82/charon:latest
   docker push ghcr.io/wikid82/charon:latest
   ```

3. **Document Rollback**:
   - Create GitHub issue documenting the reason
   - Update CHANGELOG.md with rollback notice

### Rollback Criteria

Trigger rollback if any of these occur:
- [ ] Critical security vulnerability in Debian base
- [ ] Application crashes on startup
- [ ] E2E tests fail > 10%
- [ ] Memory usage increases > 50%
- [ ] Build times increase > 3x

---

## Security Considerations

### Security Features to Maintain

| Feature | Alpine Implementation | Debian Implementation | Status |
|---------|----------------------|----------------------|--------|
| Non-root user | `adduser -D` | `useradd -M` | ✅ Maintain |
| Privilege dropping | `su-exec` | `gosu` | ✅ Maintain |
| Capability binding | `setcap` via `libcap-utils` | `setcap` via `libcap2-bin` | ✅ Maintain |
| Read-only filesystem | N/A | N/A | N/A |
| Minimal packages | `--no-cache` | `--no-install-recommends` | ✅ Maintain |
| Security upgrades | `apk upgrade` | `apt-get upgrade` | ✅ Maintain |
| HEALTHCHECK | Present | Present | ✅ Maintain |

### Security Enhancements with Debian

1. **glibc Security**: Better Address Space Layout Randomization (ASLR)
2. **Faster CVE Patches**: Debian Security Team is larger and faster
3. **No musl Edge Cases**: Eliminates subtle bugs in Go binaries with CGO
4. **SELinux Compatibility**: Debian has better SELinux support if needed

### Security Scanning Updates

Update Trivy configuration to scan for Debian-specific vulnerabilities:

```yaml
# .github/workflows/security-weekly-rebuild.yml
- name: Run Trivy vulnerability scanner
  uses: aquasecurity/trivy-action@...
  with:
    image-ref: ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}@${{ steps.build.outputs.digest }}
    vuln-type: 'os,library'
    severity: 'CRITICAL,HIGH'
```

---

## Implementation Checklist

### Pre-Migration

- [ ] Create feature branch: `feature/debian-migration`
- [ ] Document current Alpine image hashes for comparison
- [ ] Run full E2E test suite as baseline
- [ ] Create backup of working Dockerfile
- [ ] **Backup current production image for rollback**:
  ```bash
  docker tag ghcr.io/wikid82/charon:latest ghcr.io/wikid82/charon:pre-debian-migration
  docker push ghcr.io/wikid82/charon:pre-debian-migration
  ```

### Dockerfile Changes

- [ ] Update `frontend-builder` stage to Debian slim
- [ ] Update `backend-builder` stage to Debian bookworm
- [ ] Update `caddy-builder` stage to Debian bookworm
- [ ] Update `crowdsec-builder` stage to Debian bookworm
- [ ] Update `crowdsec-fallback` stage to Debian slim
- [ ] Update final runtime stage to Debian slim
- [ ] Update `CADDY_IMAGE` ARG default
- [ ] Replace all `apk` commands with `apt-get`
- [ ] Update user/group creation commands
- [ ] Replace `su-exec` with `gosu`
- [ ] Remove ARM64 clang wrapper workaround
- [ ] Update cross-compilation setup for xx helper

### Entrypoint Changes

- [ ] Replace `su-exec` with `gosu`
- [ ] Replace `addgroup` with `groupadd`
- [ ] Replace `adduser` with `usermod -aG`

### CI/CD Changes

- [ ] Update `docker-build.yml` Caddy digest step
- [ ] Update `security-weekly-rebuild.yml` package check
- [ ] Update any other workflows referencing Alpine
- [ ] **Update Renovate configuration** (`renovate.json`) to track Debian base image updates (see Appendix B)

### Testing

- [ ] Build multi-architecture image (amd64, arm64)
- [ ] Run all E2E Playwright tests
- [ ] Run all backend unit tests
- [ ] Run Trivy vulnerability scan
- [ ] Verify non-root user execution
- [ ] Verify CrowdSec initialization
- [ ] Verify Caddy startup
- [ ] **gosu functionality test**: Verify privilege dropping works correctly
  ```bash
  docker run --rm charon:debian-test gosu charon id
  # Expected: uid=1000(charon) gid=1000(charon) groups=1000(charon)
  docker run --rm charon:debian-test gosu charon whoami
  # Expected: charon
  ```
- [ ] **Docker socket integration test**: Verify socket group mapping works
  ```bash
  docker run --rm -v /var/run/docker.sock:/var/run/docker.sock charon:debian-test \
    bash -c "stat -c '%g' /var/run/docker.sock && groups charon"
  # Verify charon user is added to the docker socket's group
  ```

### Documentation

- [ ] Update README.md if any user-facing changes
- [ ] Update CHANGELOG.md with migration details
- [ ] Update `docs/DOCKER.md` with Debian-specific instructions
- [ ] Update `docs/features.md` to reflect base image change
- [ ] Archive this plan to `docs/implementation/`

### Post-Migration

- [ ] Monitor production for 48 hours
- [ ] Verify no regression in vulnerability reports
- [ ] Close related security issues/CVEs
- [ ] Remove any Alpine-specific workarounds from codebase

---

## Appendix A: Complete Debian Package List

```dockerfile
# Runtime packages
RUN apt-get update && apt-get install -y --no-install-recommends \
    bash \
    ca-certificates \
    curl \
    gettext-base \
    gosu \
    libc-ares2 \
    libcap2-bin \
    libsqlite3-0 \
    sqlite3 \
    tzdata \
    && apt-get upgrade -y \
    && rm -rf /var/lib/apt/lists/*
```

## Appendix B: Renovate Configuration Updates

If using Renovate for dependency management, update `renovate.json`:

```json
{
  "regexManagers": [
    {
      "fileMatch": ["^Dockerfile$"],
      "matchStrings": ["ARG CADDY_IMAGE=debian:(?<currentValue>[\\w.-]+)"],
      "depNameTemplate": "debian",
      "datasourceTemplate": "docker"
    }
  ]
}
```

## Appendix C: Image Size Comparison (Expected)

| Component | Alpine Size | Debian Size | Delta |
|-----------|-------------|-------------|-------|
| Base image | 5 MB | 25 MB | +20 MB |
| Runtime packages | 45 MB | 55 MB | +10 MB |
| Go binary (Charon) | 30 MB | 30 MB | 0 |
| Caddy binary | 45 MB | 45 MB | 0 |
| CrowdSec binaries | 25 MB | 25 MB | 0 |
| Frontend assets | 10 MB | 10 MB | 0 |
| **Total** | **~160 MB** | **~190 MB** | **+30 MB** |

*Note: Actual sizes may vary. The ~30MB increase is an acceptable trade-off for improved security.*

---

## References

- [Debian Docker Official Images](https://hub.docker.com/_/debian)
- [Node.js Docker Official Images](https://hub.docker.com/_/node)
- [Go Docker Official Images](https://hub.docker.com/_/golang)
- [gosu GitHub Repository](https://github.com/tianon/gosu)
- [tonistiigi/xx Cross-Compilation](https://github.com/tonistiigi/xx)
- [Alpine vs Debian for Docker](https://docs.docker.com/build/building/best-practices/)
- [musl vs glibc Considerations](https://wiki.musl-libc.org/functional-differences-from-glibc.html)
