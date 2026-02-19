# CrowdSec Source Build Implementation Plan - CVE-2025-68156 Remediation

**Date:** January 11, 2026
**Priority:** CRITICAL
**Estimated Time:** 3-4 hours
**Target Completion:** Within 48 hours

---

## Executive Summary

This plan details the implementation to build CrowdSec from source in the Dockerfile to remediate **CVE-2025-68156** affecting `expr-lang/expr v1.17.2`. Currently, the Dockerfile downloads pre-compiled CrowdSec binaries which contain the vulnerable dependency. We will implement a multi-stage build following the existing Caddy pattern to compile CrowdSec with patched `expr-lang/expr v1.17.7`.

**Key Insight:** The current Dockerfile **already has a `crowdsec-builder` stage** (lines 199-244) that builds CrowdSec from source using Go 1.25.5+. However, it does **not patch the expr-lang/expr dependency**. This plan adds the missing patch step.

---

## Current State Analysis

### Existing Dockerfile Structure

**CrowdSec Builder Stage (Lines 199-244):**

```dockerfile
# ---- CrowdSec Builder ----
# Build CrowdSec from source to ensure we use Go 1.25.5+ and avoid stdlib vulnerabilities
FROM --platform=$BUILDPLATFORM golang:1.25.5-alpine AS crowdsec-builder
COPY --from=xx / /

WORKDIR /tmp/crowdsec

ARG TARGETPLATFORM
ARG TARGETOS
ARG TARGETARCH
# CrowdSec version - Renovate can update this
# renovate: datasource=github-releases depName=crowdsecurity/crowdsec
ARG CROWDSEC_VERSION=1.7.6

RUN apk add --no-cache git clang lld
RUN xx-apk add --no-cache gcc musl-dev

# Clone CrowdSec source
RUN git clone --depth 1 --branch "v${CROWDSEC_VERSION}" https://github.com/crowdsecurity/crowdsec.git .

# Build CrowdSec binaries for target architecture
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=1 xx-go build -o /crowdsec-out/crowdsec \
        -ldflags "-s -w -X github.com/crowdsecurity/crowdsec/pkg/cwversion.Version=v${CROWDSEC_VERSION}" \
        ./cmd/crowdsec && \
    xx-verify /crowdsec-out/crowdsec

RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=1 xx-go build -o /crowdsec-out/cscli \
        -ldflags "-s -w -X github.com/crowdsecurity/crowdsec/pkg/cwversion.Version=v${CROWDSEC_VERSION}" \
        ./cmd/crowdsec-cli && \
    xx-verify /crowdsec-out/cscli

# Copy config files
RUN mkdir -p /crowdsec-out/config && \
    cp -r config/* /crowdsec-out/config/ || true
```

**Critical Findings:**

1. ✅ **Already builds from source** with Go 1.25.5+
2. ❌ **Missing expr-lang/expr dependency patch** (similar to Caddy at line 181)
3. ✅ Uses multi-stage build with xx-go for cross-compilation
4. ✅ Has proper cache mounts for Go builds
5. ✅ Verifies binaries with xx-verify

### Caddy Build Pattern (Reference Model)

**Lines 150-195 show the pattern we need to replicate:**

```dockerfile
# Build Caddy for the target architecture with security plugins.
# Two-stage approach: xcaddy generates go.mod, we patch it, then build from scratch.
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    sh -c 'set -e; \
        export XCADDY_SKIP_CLEANUP=1; \
        echo "Stage 1: Generate go.mod with xcaddy..."; \
        # Run xcaddy to generate the build directory and go.mod
        GOOS=$TARGETOS GOARCH=$TARGETARCH xcaddy build v${CADDY_VERSION} \
            --with github.com/greenpau/caddy-security \
            --with github.com/corazawaf/coraza-caddy/v2 \
            --with github.com/hslatman/caddy-crowdsec-bouncer \
            --with github.com/zhangjiayin/caddy-geoip2 \
            --with github.com/mholt/caddy-ratelimit \
            --output /tmp/caddy-initial || true; \
        # Find the build directory created by xcaddy
        BUILDDIR=$(ls -td /tmp/buildenv_* 2>/dev/null | head -1); \
        if [ ! -d "$BUILDDIR" ] || [ ! -f "$BUILDDIR/go.mod" ]; then \
            echo "ERROR: Build directory not found or go.mod missing"; \
            exit 1; \
        fi; \
        echo "Found build directory: $BUILDDIR"; \
        cd "$BUILDDIR"; \
        echo "Stage 2: Apply security patches to go.mod..."; \
        # Patch ALL dependencies BEFORE building the final binary
        # renovate: datasource=go depName=github.com/expr-lang/expr
        go get github.com/expr-lang/expr@v1.17.7; \
        # Clean up go.mod and ensure all dependencies are resolved
        go mod tidy; \
        echo "Dependencies patched successfully"; \
        # ... build final binary with patched dependencies
```

**Key Pattern Elements:**

1. Work in cloned source directory
2. Patch go.mod with `go get` for vulnerable dependencies
3. Run `go mod tidy` to resolve dependencies
4. Build final binaries with patched dependencies
5. Add Renovate tracking comments for dependency versions

---

## CrowdSec Dependency Analysis

### Research Findings

**From CrowdSec GitHub (crowdsecurity/crowdsec v1.7.5):**

- **Language:** Go 81.3%
- **License:** MIT
- **Build System:** Makefile with Go build commands
- **Dependencies:** Managed via `go.mod` and `go.sum`
- **Key Commands:** `./cmd/crowdsec` (daemon), `./cmd/crowdsec-cli` (cscli tool)

**expr-lang/expr Usage in CrowdSec:**

CrowdSec uses `expr-lang/expr` extensively for:

- **Scenario evaluation** (attack pattern matching)
- **Parser filters** (log parsing conditional logic)
- **Whitelist expressions** (decision exceptions)
- **Conditional rule execution**

**Vulnerability Impact:**

CVE-2025-68156 (GHSA-cfpf-hrx2-8rv6) affects expression evaluation, potentially allowing:

- Arbitrary code execution via crafted expressions
- Denial of service through malicious scenarios
- Security bypass in rule evaluation

**Required Patch:** Upgrade from `expr-lang/expr v1.17.2` → `v1.17.7`

### Verification Strategy

**Check Current CrowdSec Dependency:**

```bash
# Extract cscli binary from built image
docker create --name crowdsec-temp charon:local
docker cp crowdsec-temp:/usr/local/bin/cscli ./cscli_binary
docker rm crowdsec-temp

# Inspect dependencies (requires Go toolchain)
go version -m ./cscli_binary | grep expr-lang
# Expected BEFORE patch: github.com/expr-lang/expr v1.17.2
# Expected AFTER patch:  github.com/expr-lang/expr v1.17.7
```

---

## Implementation Plan

### Phase 1: Add expr-lang Patch to CrowdSec Builder

**Objective:** Modify the `crowdsec-builder` stage to patch `expr-lang/expr` before compiling binaries.

#### 1.1 Dockerfile Modifications

**File:** `Dockerfile`

**Location:** Lines 199-244 (crowdsec-builder stage)

**Change Strategy:** Insert dependency patching step after `git clone` and before `go build` commands.

**New Implementation:**

```dockerfile
# ---- CrowdSec Builder ----
# Build CrowdSec from source to ensure we use Go 1.25.5+ and avoid stdlib vulnerabilities
# (CVE-2025-58183, CVE-2025-58186, CVE-2025-58187, CVE-2025-61729)
# Additionally, patch expr-lang/expr to v1.17.7 to fix CVE-2025-68156
# renovate: datasource=docker depName=golang versioning=docker
FROM --platform=$BUILDPLATFORM golang:1.25.5-alpine AS crowdsec-builder
COPY --from=xx / /

WORKDIR /tmp/crowdsec

ARG TARGETPLATFORM
ARG TARGETOS
ARG TARGETARCH
# CrowdSec version - Renovate can update this
# renovate: datasource=github-releases depName=crowdsecurity/crowdsec
ARG CROWDSEC_VERSION=1.7.6

# hadolint ignore=DL3018
RUN apk add --no-cache git clang lld
# hadolint ignore=DL3018,DL3059
RUN xx-apk add --no-cache gcc musl-dev

# Clone CrowdSec source
RUN git clone --depth 1 --branch "v${CROWDSEC_VERSION}" https://github.com/crowdsecurity/crowdsec.git .

# Patch expr-lang/expr dependency to fix CVE-2025-68156
# This follows the same pattern as Caddy's expr-lang patch (Dockerfile line 181)
# renovate: datasource=go depName=github.com/expr-lang/expr
RUN go get github.com/expr-lang/expr@v1.17.7 && \
    go mod tidy

# Build CrowdSec binaries for target architecture with patched dependencies
# hadolint ignore=DL3059
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=1 xx-go build -o /crowdsec-out/crowdsec \
        -ldflags "-s -w -X github.com/crowdsecurity/crowdsec/pkg/cwversion.Version=v${CROWDSEC_VERSION}" \
        ./cmd/crowdsec && \
    xx-verify /crowdsec-out/crowdsec

# hadolint ignore=DL3059
RUN --mount=type=cache,target=/root/.cache/go-build \
    --mount=type=cache,target=/go/pkg/mod \
    CGO_ENABLED=1 xx-go build -o /crowdsec-out/cscli \
        -ldflags "-s -w -X github.com/crowdsecurity/crowdsec/pkg/cwversion.Version=v${CROWDSEC_VERSION}" \
        ./cmd/crowdsec-cli && \
    xx-verify /crowdsec-out/cscli

# Copy config files
RUN mkdir -p /crowdsec-out/config && \
    cp -r config/* /crowdsec-out/config/ || true
```

**Key Changes:**

1. **Line 201:** Updated comment to mention CVE-2025-68156 remediation
2. **Lines 220-223:** **NEW** - Added expr-lang/expr patch step with Renovate tracking
3. **Line 225:** Updated comment to clarify "patched dependencies"

**Rationale:**

- Mirrors Caddy's proven patching approach (line 181)
- Maintains multi-stage build architecture
- Preserves cross-compilation with xx-go
- Adds Renovate tracking for automated updates
- Minimal changes (4 lines added, 2 modified)

#### 1.2 Dockerfile Comment Updates

**Add Security Note to Top-Level Comments:**

**Location:** Near line 7-17 (version tracking section)

**Addition:**

```dockerfile
## Security Patches:
## - Caddy: expr-lang/expr@v1.17.7 (CVE-2025-68156)
## - CrowdSec: expr-lang/expr@v1.17.7 (CVE-2025-68156)
```

This documents that both Caddy and CrowdSec have the same critical patch applied.

---

### Phase 2: CI/CD Verification Integration

**Objective:** Add automated verification to GitHub Actions workflow to ensure CrowdSec binaries contain patched expr-lang.

#### 2.1 GitHub Actions Workflow Enhancement

**File:** `.github/workflows/docker-build.yml`

**Location:** After Caddy verification (lines 157-217), add parallel CrowdSec verification

**New Step:**

```yaml
      - name: Verify CrowdSec Security Patches (CVE-2025-68156)
        if: success()
        run: |
          echo "🔍 Verifying CrowdSec binaries contain patched expr-lang/expr@v1.17.7..."
          echo ""

          # Determine the image reference based on event type
          if [ "${{ github.event_name }}" = "pull_request" ]; then
            IMAGE_REF="${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}:pr-${{ github.event.pull_request.number }}"
            echo "Using PR image: $IMAGE_REF"
          else
            IMAGE_REF="${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}@${{ steps.build-and-push.outputs.digest }}"
            echo "Using digest: $IMAGE_REF"
          fi

          echo ""
          echo "==> CrowdSec cscli version:"
          timeout 30s docker run --rm $IMAGE_REF cscli version || echo "⚠️ CrowdSec version check timed out or failed (may not be installed for this architecture)"

          echo ""
          echo "==> Extracting cscli binary for inspection..."
          CONTAINER_ID=$(docker create $IMAGE_REF)
          docker cp ${CONTAINER_ID}:/usr/local/bin/cscli ./cscli_binary 2>/dev/null || {
            echo "⚠️ cscli binary not found - CrowdSec may not be available for this architecture"
            docker rm ${CONTAINER_ID}
            exit 0
          }
          docker rm ${CONTAINER_ID}

          echo ""
          echo "==> Checking if Go toolchain is available locally..."
          if command -v go >/dev/null 2>&1; then
            echo "✅ Go found locally, inspecting binary dependencies..."
            go version -m ./cscli_binary > cscli_deps.txt

            echo ""
            echo "==> Searching for expr-lang/expr dependency:"
            if grep -i "expr-lang/expr" cscli_deps.txt; then
              EXPR_VERSION=$(grep "expr-lang/expr" cscli_deps.txt | awk '{print $3}')
              echo ""
              echo "✅ Found expr-lang/expr: $EXPR_VERSION"

              # Check if version is v1.17.7 or higher (vulnerable version is v1.17.2)
              if echo "$EXPR_VERSION" | grep -E "^v1\.(1[7-9]|[2-9][0-9])\.[7-9][0-9]*$|^v1\.17\.([7-9]|[1-9][0-9]+)$" >/dev/null; then
                echo "✅ PASS: expr-lang version $EXPR_VERSION is patched (>= v1.17.7)"
              else
                echo "❌ FAIL: expr-lang version $EXPR_VERSION is vulnerable (< v1.17.7)"
                echo "⚠️ WARNING: expr-lang version $EXPR_VERSION may be vulnerable (< v1.17.7)"
                echo "Expected: v1.17.7 or higher to mitigate CVE-2025-68156"
                exit 1
              fi
            else
              echo "⚠️ expr-lang/expr not found in binary dependencies"
              echo "This could mean:"
              echo "  1. The dependency was stripped/optimized out"
              echo "  2. CrowdSec was built without the expression evaluator"
              echo "  3. Binary inspection failed"
              echo ""
              echo "Displaying all dependencies for review:"
              cat cscli_deps.txt
            fi
          else
            echo "⚠️ Go toolchain not available in CI environment"
            echo "Cannot inspect binary modules - skipping dependency verification"
            echo "Note: Runtime image does not require Go as CrowdSec is a standalone binary"
          fi

          # Cleanup
          rm -f ./cscli_binary cscli_deps.txt

          echo ""
          echo "==> CrowdSec verification complete"
```

**Key Features:**

1. Parallels Caddy verification logic exactly
2. Extracts `cscli` binary (more reliable than `crowdsec` daemon)
3. Uses `go version -m` to inspect embedded dependencies
4. Validates expr-lang version >= v1.17.7
5. Gracefully handles architectures where CrowdSec isn't built
6. Fails CI if vulnerable version detected

#### 2.2 Workflow Success Criteria

**Expected CI Output (Successful Patch):**

```
✅ Go found locally, inspecting binary dependencies...

==> Searching for expr-lang/expr dependency:
github.com/expr-lang/expr v1.17.7

✅ Found expr-lang/expr: v1.17.7
✅ PASS: expr-lang version v1.17.7 is patched (>= v1.17.7)

==> CrowdSec verification complete
```

**Expected CI Output (Failure - Pre-Patch):**

```
❌ FAIL: expr-lang version v1.17.2 is vulnerable (< v1.17.7)
⚠️ WARNING: expr-lang version v1.17.2 may be vulnerable (< v1.17.7)
Expected: v1.17.7 or higher to mitigate CVE-2025-68156
```

---

### Phase 3: Testing & Validation

#### 3.1 Local Build Verification

**Step 1: Clean Build**

```bash
# Force rebuild without cache to ensure fresh dependencies
docker build --no-cache --progress=plain -t charon:crowdsec-patch .
```

**Expected Build Time:** 15-20 minutes (no cache)

**Step 2: Extract and Verify Binary**

```bash
# Extract cscli binary
docker create --name crowdsec-verify charon:crowdsec-patch
docker cp crowdsec-verify:/usr/local/bin/cscli ./cscli_binary
docker rm crowdsec-verify

# Verify expr-lang version (requires Go toolchain)
go version -m ./cscli_binary | grep expr-lang
# Expected output: github.com/expr-lang/expr v1.17.7

# Cleanup
rm ./cscli_binary
```

**Step 3: Functional Test**

```bash
# Start container
docker run -d --name crowdsec-test -p 8080:8080 -p 80:80 charon:crowdsec-patch

# Wait for CrowdSec to start
sleep 30

# Verify CrowdSec functionality
docker exec crowdsec-test cscli version
docker exec crowdsec-test cscli hub list
docker exec crowdsec-test cscli metrics

# Check logs for errors
docker logs crowdsec-test 2>&1 | grep -i "crowdsec" | tail -20

# Cleanup
docker stop crowdsec-test
docker rm crowdsec-test
```

**Expected Results:**

- ✅ `cscli version` shows CrowdSec v1.7.5
- ✅ `cscli hub list` displays installed scenarios/parsers
- ✅ `cscli metrics` shows metrics (or "No data" if no logs processed yet)
- ✅ No critical errors in logs

#### 3.2 Integration Test Execution

**Use Existing Test Suite:**

```bash
# Run CrowdSec-specific integration tests
.vscode/tasks.json -> "Integration: CrowdSec"
# Or:
.github/skills/scripts/skill-runner.sh integration-test-crowdsec

# Run CrowdSec startup test
.vscode/tasks.json -> "Integration: CrowdSec Startup"
# Or:
.github/skills/scripts/skill-runner.sh integration-test-crowdsec-startup

# Run CrowdSec decisions test
.vscode/tasks.json -> "Integration: CrowdSec Decisions"
# Or:
.github/skills/scripts/skill-runner.sh integration-test-crowdsec-decisions

# Run all integration tests
.vscode/tasks.json -> "Integration: Run All"
# Or:
.github/skills/scripts/skill-runner.sh integration-test-all
```

**Success Criteria:**

- [ ] All CrowdSec integration tests pass
- [ ] CrowdSec starts without errors
- [ ] Hub items install correctly
- [ ] Bouncers can register
- [ ] Decisions are created and enforced
- [ ] No expr-lang evaluation errors in logs

#### 3.3 Security Scan Verification

**Trivy Scan (Post-Patch):**

```bash
# Scan for CVE-2025-68156 specifically
trivy image --severity HIGH,CRITICAL charon:crowdsec-patch | grep -i "68156"
# Expected: No matches (CVE remediated)

# Full scan
trivy image --severity HIGH,CRITICAL charon:crowdsec-patch
# Expected: Zero HIGH/CRITICAL vulnerabilities
```

**govulncheck (Go Module Scan):**

```bash
# Scan built binaries for Go vulnerabilities
cd /tmp
docker create --name vuln-check charon:crowdsec-patch
docker cp vuln-check:/usr/local/bin/cscli ./cscli_test
docker cp vuln-check:/usr/local/bin/crowdsec ./crowdsec_test
docker rm vuln-check

# Check vulnerabilities (requires Go toolchain)
go run golang.org/x/vuln/cmd/govulncheck@latest -mode=binary ./cscli_test
go run golang.org/x/vuln/cmd/govulncheck@latest -mode=binary ./crowdsec_test

# Expected: No vulnerabilities found
# Cleanup
rm ./cscli_test ./crowdsec_test
```

---

### Phase 4: Documentation Updates

#### 4.1 Update Security Remediation Plan

**File:** `docs/plans/security_vulnerability_remediation.md`

**Changes:**

1. Add new section: "Phase 1.3: CrowdSec expr-lang Patch"
2. Update vulnerability inventory with CrowdSec-specific findings
3. Add task breakdown for CrowdSec patching (separate from Caddy)
4. Update timeline estimates
5. Document verification procedures

**Location:** After "Phase 1.2: expr-lang/expr Upgrade" (line ~120)

#### 4.2 Update Dockerfile Comments

**File:** `Dockerfile`

**Add/Update:**

1. Top-level security patch documentation (line ~7-17)
2. CrowdSec builder stage comments (line ~199-202)
3. Final stage comment documenting patched binaries (line ~278-284)

#### 4.3 Update CI Workflow Comments

**File:** `.github/workflows/docker-build.yml`

**Add:**

1. Comment header for CrowdSec verification step
2. Reference to CVE-2025-68156
3. Link to remediation plan document

---

## Architecture Considerations

### Multi-Architecture Builds

**Current Support:**

- ✅ amd64 (x86_64): Full CrowdSec build from source
- ✅ arm64 (aarch64): Full CrowdSec build from source
- ⚠️ Other architectures: Fallback to pre-built binaries (lines 246-274)

**Impact of Patch:**

- **amd64/arm64:** Will receive patched expr-lang binaries
- **Other archs:** Still vulnerable (uses pre-built binaries from fallback stage)

**Recommendation:** Document limitation and consider dropping support for other architectures if security is critical.

### Build Performance

**Expected Impact:**

| Stage | Before Patch | After Patch | Delta |
|-------|--------------|-------------|-------|
| CrowdSec Clone | 10s | 10s | 0s |
| Dependency Download | 30s | 35s | +5s (go get) |
| Compilation | 60s | 60s | 0s |
| **Total CrowdSec Build** | **100s** | **105s** | **+5s** |

**Total Dockerfile Build:** ~15 minutes (no significant impact)

**Cache Optimization:**

- Go module cache: `target=/go/pkg/mod` (already present)
- Build cache: `target=/root/.cache/go-build` (already present)
- Subsequent builds: ~5-7 minutes (with cache)

### Compatibility Considerations

**CrowdSec Version Pinning:**

- Current: `v1.7.6` (January 2026 release)
- expr-lang in v1.7.6: Uses patched `v1.17.7`
- Post-patch: `v1.17.7` (forced upgrade via `go get`)

**Potential Issues:**

1. **Breaking API Changes:** expr-lang v1.17.2 → v1.17.7 may have breaking changes in expression evaluation
2. **Scenario Compatibility:** Hub scenarios may rely on specific expr-lang behavior
3. **Parser Compatibility:** Custom parsers may break with newer expr-lang

**Mitigation:**

- Test comprehensive scenario evaluation (Phase 3.2)
- Monitor CrowdSec logs for expr-lang errors
- Document rollback procedure if incompatibilities found

---

## Rollback Procedures

### Scenario 1: expr-lang Patch Breaks CrowdSec

**Symptoms:**

- CrowdSec fails to start
- Scenario evaluation errors: `expr: unknown function` or `expr: syntax error`
- Hub items fail to install
- Parsers crash on log processing

**Rollback Steps:**

```bash
# Revert Dockerfile changes
git diff HEAD Dockerfile > /tmp/crowdsec_patch.diff
git checkout Dockerfile

# Verify original Dockerfile restored
git diff Dockerfile
# Should show no changes

# Rebuild with original (vulnerable) binaries
docker build --no-cache -t charon:rollback .

# Test rollback
docker run -d --name charon-rollback charon:rollback
docker exec charon-rollback cscli version
docker logs charon-rollback

# If successful, commit rollback
git add Dockerfile
git commit -m "revert: rollback CrowdSec expr-lang patch due to incompatibility"
```

**Post-Rollback Actions:**

1. Document specific expr-lang incompatibility
2. Check CrowdSec v1.7.5+ for native expr-lang v1.17.7 support
3. Consider waiting for upstream CrowdSec release with patched dependency
4. Re-evaluate CVE severity vs. functionality trade-off

### Scenario 2: CI Verification Fails

**Symptoms:**

- GitHub Actions fails on "Verify CrowdSec Security Patches" step
- CI shows `❌ FAIL: expr-lang version v1.17.2 is vulnerable`
- Build succeeds but verification fails

**Debugging Steps:**

```bash
# Check if patch was applied during build
docker build --no-cache --progress=plain -t charon:debug . 2>&1 | grep -A5 "go get.*expr-lang"

# Expected output:
# renovate: datasource=go depName=github.com/expr-lang/expr
# go get github.com/expr-lang/expr@v1.17.7
# go mod tidy

# If patch step is missing, Dockerfile wasn't updated correctly
```

**Resolution:**

1. Verify Dockerfile changes were committed
2. Check git diff against this plan
3. Ensure Renovate comment is present (for tracking)
4. Re-run build with `--no-cache`

### Scenario 3: Integration Tests Fail

**Symptoms:**

- CrowdSec starts but doesn't process logs
- Scenarios don't trigger
- Decisions aren't created
- expr-lang errors in logs: `failed to compile expression`

**Debugging:**

```bash
# Check CrowdSec logs for expr errors
docker exec <container-id> cat /var/log/crowdsec/crowdsec.log | grep -i expr

# Test scenario evaluation manually
docker exec <container-id> cscli scenarios inspect crowdsecurity/http-probing

# Check parser compilation
docker exec <container-id> cscli parsers list
```

**Resolution:**

1. Identify specific scenario/parser with expr-lang issue
2. Check expr-lang v1.17.7 changelog for breaking changes
3. Update scenario expression syntax if needed
4. Report issue to CrowdSec community if upstream bug

---

## Success Criteria

### Critical Success Metrics

1. **CVE Remediation:**
   - ✅ Trivy scan shows zero instances of CVE-2025-68156
   - ✅ `go version -m cscli` shows `expr-lang/expr v1.17.7`
   - ✅ CI verification passes on all builds

2. **Functional Verification:**
   - ✅ CrowdSec starts without errors
   - ✅ Hub items install successfully
   - ✅ Scenarios evaluate correctly (no expr-lang errors)
   - ✅ Decisions are created and enforced
   - ✅ Bouncers function correctly

3. **Integration Tests:**
   - ✅ All CrowdSec integration tests pass
   - ✅ CrowdSec Startup test passes
   - ✅ CrowdSec Decisions test passes
   - ✅ Coraza WAF integration unaffected

### Secondary Success Metrics

1. **Build Performance:**
   - ✅ Build time increase < 10 seconds
   - ✅ Image size increase < 5MB
   - ✅ Cache efficiency maintained

2. **Documentation:**
   - ✅ Dockerfile comments updated
   - ✅ CI workflow documented
   - ✅ Security remediation plan updated
   - ✅ Rollback procedures documented

3. **CI/CD:**
   - ✅ GitHub Actions includes CrowdSec verification
   - ✅ Renovate tracks expr-lang version
   - ✅ PR builds trigger verification
   - ✅ Main branch builds pass all checks

---

## Timeline & Resource Allocation

### Estimated Timeline (Total: 3-4 hours)

| Task | Duration | Dependencies | Critical Path |
|------|----------|--------------|---------------|
| **Phase 1:** Dockerfile Modifications | 30 mins | None | Yes |
| **Phase 2:** CI/CD Integration | 45 mins | Phase 1 | Yes |
| **Phase 3:** Testing & Validation | 90 mins | Phase 2 | Yes |
| **Phase 4:** Documentation | 30 mins | Phase 3 | Yes |

**Critical Path:** Phase 1 → Phase 2 → Phase 3 → Phase 4 = **3.25 hours**

**Contingency Buffer:** +30-45 mins for troubleshooting = **Total: 4 hours**

### Resource Requirements

**Personnel:**

- 1x DevOps Engineer (Dockerfile modifications, CI/CD integration)
- 1x QA Engineer (Integration testing)
- 1x Security Specialist (Verification, documentation)

**Infrastructure:**

- Docker build environment (20GB disk space)
- Go 1.25+ toolchain (for local verification)
- GitHub Actions runners (for CI validation)

**Tools:**

- Docker Desktop / Docker CLI
- Go toolchain (optional, for local binary inspection)
- trivy (security scanner)
- govulncheck (Go vulnerability scanner)

---

## Post-Implementation Actions

### Immediate (Within 24 hours)

1. **Monitor CI/CD Pipelines:**
   - Verify all builds pass CrowdSec verification
   - Check for false positives/negatives
   - Adjust regex patterns if needed

2. **Update Documentation:**
   - Link this plan to `security_vulnerability_remediation.md`
   - Update main README.md if security posture mentioned
   - Add to CHANGELOG.md

3. **Notify Stakeholders:**
   - Security team: CVE remediated
   - Development team: New CI check added
   - Users: Security update in next release

### Short-term (Within 1 week)

1. **Monitor CrowdSec Functionality:**
   - Review CrowdSec logs for expr-lang errors
   - Check scenario execution metrics
   - Validate decision creation rates

2. **Renovate Configuration:**
   - Verify Renovate detects expr-lang tracking comment
   - Test automated PR creation for expr-lang updates
   - Document Renovate configuration for future maintainers

3. **Performance Baseline:**
   - Measure build time with/without cache
   - Document image size changes
   - Optimize if performance degradation observed

### Long-term (Within 1 month)

1. **Upstream Monitoring:**
   - Watch for CrowdSec v1.7.5+ release with native expr-lang v1.17.7
   - Consider removing manual patch if upstream includes fix
   - Track expr-lang security advisories

2. **Architecture Review:**
   - Evaluate multi-arch support (drop unsupported architectures?)
   - Consider distroless base images for security
   - Review CrowdSec fallback stage necessity

3. **Security Posture Audit:**
   - Schedule quarterly Trivy scans
   - Enable Dependabot for Go modules
   - Implement automated CVE monitoring

---

## Appendix A: CrowdSec Build Commands Reference

### Manual CrowdSec Build (Outside Docker)

```bash
# Clone CrowdSec
git clone --depth 1 --branch v1.7.6 https://github.com/crowdsecurity/crowdsec.git
cd crowdsec

# Patch expr-lang
go get github.com/expr-lang/expr@v1.17.7
go mod tidy

# Build binaries
CGO_ENABLED=1 go build -o crowdsec \
  -ldflags "-s -w -X github.com/crowdsecurity/crowdsec/pkg/cwversion.Version=v1.7.6" \
  ./cmd/crowdsec

CGO_ENABLED=1 go build -o cscli \
  -ldflags "-s -w -X github.com/crowdsecurity/crowdsec/pkg/cwversion.Version=v1.7.6" \
  ./cmd/crowdsec-cli

# Verify expr-lang version
go version -m ./cscli | grep expr-lang
# Expected: github.com/expr-lang/expr v1.17.7
```

### Verify Patched Binaries

```bash
# Check embedded dependencies
go version -m /usr/local/bin/cscli | grep expr-lang

# Alternative: Use strings (less reliable)
strings /usr/local/bin/cscli | grep -i "expr-lang"

# Check version
cscli version
# Output:
# version: v1.7.5
# ...
```

---

## Appendix B: Dockerfile Diff Preview

**Expected git diff after implementation:**

```diff
diff --git a/Dockerfile b/Dockerfile
index abc1234..def5678 100644
--- a/Dockerfile
+++ b/Dockerfile
@@ -5,6 +5,9 @@
 ARG VERSION=dev
 ARG BUILD_DATE
 ARG VCS_REF
+## Security Patches:
+## - Caddy: expr-lang/expr@v1.17.7 (CVE-2025-68156)
+## - CrowdSec: expr-lang/expr@v1.17.7 (CVE-2025-68156)

 # Allow pinning Caddy version - Renovate will update this
 # Build the most recent Caddy 2.x release (keeps major pinned under v3).
@@ -197,7 +200,8 @@ RUN --mount=type=cache,target=/root/.cache/go-build \

 # ---- CrowdSec Builder ----
 # Build CrowdSec from source to ensure we use Go 1.25.5+ and avoid stdlib vulnerabilities
-# (CVE-2025-58183, CVE-2025-58186, CVE-2025-58187, CVE-2025-61729)
+# (CVE-2025-58183, CVE-2025-58186, CVE-2025-58187, CVE-2025-61729)
+# Additionally, patch expr-lang/expr to v1.17.7 to fix CVE-2025-68156
 # renovate: datasource=docker depName=golang versioning=docker
 FROM --platform=$BUILDPLATFORM golang:1.25.5-alpine AS crowdsec-builder
 COPY --from=xx / /
@@ -218,7 +222,12 @@ RUN xx-apk add --no-cache gcc musl-dev
 # Clone CrowdSec source
 RUN git clone --depth 1 --branch "v${CROWDSEC_VERSION}" https://github.com/crowdsecurity/crowdsec.git .

-# Build CrowdSec binaries for target architecture
+# Patch expr-lang/expr dependency to fix CVE-2025-68156
+# This follows the same pattern as Caddy's expr-lang patch (Dockerfile line 181)
+# renovate: datasource=go depName=github.com/expr-lang/expr
+RUN go get github.com/expr-lang/expr@v1.17.7 && \
+    go mod tidy
+
+# Build CrowdSec binaries for target architecture with patched dependencies
 # hadolint ignore=DL3059
 RUN --mount=type=cache,target=/root/.cache/go-build \
     --mount=type=cache,target=/go/pkg/mod \
```

**Total Changes:**

- **Added:** 10 lines
- **Modified:** 3 lines
- **Deleted:** 0 lines

---

## Appendix C: Troubleshooting Guide

### Issue: go get fails with "no required module provides package"

**Error:**

```
go: github.com/expr-lang/expr@v1.17.7: reading github.com/expr-lang/expr/go.mod at revision v1.17.7: unknown revision v1.17.7
```

**Cause:** expr-lang/expr v1.17.7 doesn't exist or version tag is incorrect

**Solution:**

```bash
# Check available versions
go list -m -versions github.com/expr-lang/expr

# Use latest available version >= v1.17.7
go get github.com/expr-lang/expr@latest
```

### Issue: go mod tidy fails after patch

**Error:**

```
go: inconsistent vendoring in /tmp/crowdsec:
 github.com/expr-lang/expr@v1.17.7: is explicitly required in go.mod, but not marked as explicit in vendor/modules.txt
```

**Cause:** Vendored dependencies out of sync

**Solution:**

```bash
# Remove vendor directory before patching
RUN go get github.com/expr-lang/expr@v1.17.7 && \
    rm -rf vendor && \
    go mod tidy && \
    go mod vendor
```

### Issue: CrowdSec binary size increases significantly

**Symptoms:**

- cscli grows from 50MB to 80MB
- crowdsec grows from 60MB to 100MB
- Image size increases by 50MB+

**Cause:** Debug symbols not stripped, or expr-lang includes extra dependencies

**Solution:**

```bash
# Ensure -s -w flags are present in ldflags
-ldflags "-s -w -X github.com/crowdsecurity/crowdsec/pkg/cwversion.Version=v${CROWDSEC_VERSION}"

# -s: Strip symbol table
# -w: Strip DWARF debugging info
```

### Issue: xx-verify fails after patching

**Error:**

```
ERROR: /crowdsec-out/cscli: ELF 64-bit LSB executable, x86-64, ... (NEEDED): wrong architecture
```

**Cause:** Binary built for wrong architecture (BUILDPLATFORM instead of TARGETPLATFORM)

**Solution:**

```bash
# Verify xx-go is being used (not regular go)
RUN ... \
    CGO_ENABLED=1 xx-go build -o /crowdsec-out/cscli \
    ...

# NOT:
RUN ... \
    CGO_ENABLED=1 go build -o /crowdsec-out/cscli \
    ...
```

---

## References

1. **CVE-2025-68156:** GitHub Security Advisory GHSA-cfpf-hrx2-8rv6
   - <https://github.com/advisories/GHSA-cfpf-hrx2-8rv6>

2. **expr-lang/expr Repository:**
   - <https://github.com/expr-lang/expr>

3. **CrowdSec GitHub Repository:**
   - <https://github.com/crowdsecurity/crowdsec>

4. **CrowdSec Build Documentation:**
   - <https://doc.crowdsec.net/docs/next/contributing/build_crowdsec>

5. **Dockerfile Best Practices:**
   - <https://docs.docker.com/develop/develop-images/dockerfile_best-practices/>

6. **Go Module Documentation:**
   - <https://go.dev/ref/mod>

7. **Renovate Documentation:**
   - <https://docs.renovatebot.com/>

---

**Document Version:** 1.0
**Last Updated:** January 11, 2026
**Status:** DRAFT - Awaiting Supervisor Review
**Next Review:** After implementation completion

---

**END OF DOCUMENT**
