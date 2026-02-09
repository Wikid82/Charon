# CrowdSec Integration Test Failure Analysis

**Date:** 2026-01-28
**PR:** #550 - Alpine to Debian Trixie Migration
**CI Run:** https://github.com/Wikid82/Charon/actions/runs/21456678628/job/61799104804
**Branch:** feature/beta-release

---

## Issue Summary

The CrowdSec integration tests are failing after migrating the Dockerfile from Alpine to Debian Trixie base image. The test builds a Docker image and then tests CrowdSec functionality.

---

## Potential Root Causes

### 1. **CrowdSec Builder Stage Compatibility**

**Alpine vs Debian Differences:**
- **Alpine** uses `musl libc`, **Debian** uses `glibc`
- Different package managers: `apk` (Alpine) vs `apt` (Debian)
- Different package names and availability

**Current Dockerfile (lines 218-270):**
```dockerfile
FROM --platform=$BUILDPLATFORM golang:1.25.7-trixie AS crowdsec-builder
```

**Dependencies Installed:**
```dockerfile
RUN apt-get update && apt-get install -y --no-install-recommends \
    git clang lld \
    && rm -rf /var/lib/apt/lists/*
RUN xx-apt install -y gcc libc6-dev
```

**Possible Issues:**
- **Missing build dependencies**: CrowdSec might require additional packages on Debian that were implicitly available on Alpine
- **Git clone failures**: Network issues or GitHub rate limiting
- **Dependency resolution**: `go mod tidy` might behave differently
- **Cross-compilation issues**: `xx-go` might need additional setup for Debian

### 2. **CrowdSec Binary Path Issues**

**Runtime Image (lines 359-365):**
```dockerfile
# Copy CrowdSec binaries from the crowdsec-builder stage (built with Go 1.25.5+)
COPY --from=crowdsec-builder /crowdsec-out/crowdsec /usr/local/bin/crowdsec
COPY --from=crowdsec-builder /crowdsec-out/cscli /usr/local/bin/cscli
COPY --from=crowdsec-builder /crowdsec-out/config /etc/crowdsec.dist
```

**Possible Issues:**
- If the builder stage fails, these COPY commands will fail
- If fallback stage is used (for non-amd64), paths might be wrong

### 3. **CrowdSec Configuration Issues**

**Entrypoint Script CrowdSec Init (docker-entrypoint.sh):**
- Symlink creation from `/etc/crowdsec` to `/app/data/crowdsec/config`
- Configuration file generation and substitution
- Hub index updates

**Possible Issues:**
- Symlink already exists as directory instead of symlink
- Permission issues with non-root user
- Configuration templates missing or incompatible

### 4. **Test Script Environment Issues**

**Integration Test (crowdsec_integration.sh):**
- Builds the image with `docker build -t charon:local .`
- Starts container and waits for API
- Tests CrowdSec Hub connectivity
- Tests preset pull/apply functionality

**Possible Issues:**
- Build step timing out or failing silently
- Container failing to start properly
- CrowdSec processes not starting
- API endpoints not responding

---

## Diagnostic Steps

### Step 1: Check Build Logs

Review the CI build logs for the CrowdSec builder stage:
- Look for `git clone` errors
- Check for `go get` or `go mod tidy` failures
- Verify `xx-go build` completes successfully
- Confirm `xx-verify` passes

### Step 2: Verify CrowdSec Binaries

Check if CrowdSec binaries are actually present:
```bash
docker run --rm charon:local which crowdsec
docker run --rm charon:local which cscli
docker run --rm charon:local cscli version
```

### Step 3: Check CrowdSec Configuration

Verify configuration is properly initialized:
```bash
docker run --rm charon:local ls -la /etc/crowdsec
docker run --rm charon:local ls -la /app/data/crowdsec
docker run --rm charon:local cat /etc/crowdsec/config.yaml
```

### Step 4: Test CrowdSec Locally

Run the integration test locally:
```bash
# Build image
docker build --no-cache -t charon:local .

# Run integration test
.github/skills/scripts/skill-runner.sh integration-test-crowdsec
```

---

## Recommended Fixes

### Fix 1: Add Missing Build Dependencies

If the build is failing due to missing dependencies, add them to the CrowdSec builder:
```dockerfile
RUN apt-get update && apt-get install -y --no-install-recommends \
    git clang lld \
    build-essential pkg-config \
    && rm -rf /var/lib/apt/lists/*
```

### Fix 2: Add Build Stage Debugging

Add debugging output to identify where the build fails:
```dockerfile
# After git clone
RUN echo "CrowdSec source cloned successfully" && ls -la

# After dependency patching
RUN echo "Dependencies patched" && go mod graph | grep expr-lang

# After build
RUN echo "Build complete" && ls -la /crowdsec-out/
```

### Fix 3: Use CrowdSec Fallback

If the build continues to fail, ensure the fallback stage is working:
```dockerfile
# In final stage, use conditional COPY
COPY --from=crowdsec-fallback /crowdsec-out/bin/crowdsec /usr/local/bin/crowdsec || \
COPY --from=crowdsec-builder /crowdsec-out/crowdsec /usr/local/bin/crowdsec
```

### Fix 4: Verify cscli Before Test

Add a verification step in the entrypoint:
```bash
if ! command -v cscli >/dev/null; then
    echo "ERROR: CrowdSec not installed properly"
    exit 1
fi
```

---

## Next Steps

1. **Access full CI logs** to identify the exact failure point
2. **Run local build** to reproduce the issue
3. **Add debugging output** to the Dockerfile if needed
4. **Verify fallback** mechanism is working
5. **Update test** if CrowdSec behavior changed with new base image

---

## Related Files

- `Dockerfile` (lines 218-310): CrowdSec builder and fallback stages
- `.docker/docker-entrypoint.sh` (lines 120-230): CrowdSec initialization
- `.github/workflows/crowdsec-integration.yml`: CI workflow
- `scripts/crowdsec_integration.sh`: Legacy integration test
- `.github/skills/integration-test-crowdsec-scripts/run.sh`: Modern test wrapper

---

## Status

**Current:** Investigation in progress
**Priority:** HIGH (CI blocking)
**Impact:** Cannot merge PR #550 until resolved
