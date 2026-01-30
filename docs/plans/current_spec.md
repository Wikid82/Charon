# Docker Compose CI Failure Remediation Plan

**Status**: Active  
**Created**: 2026-01-30  
**Priority**: CRITICAL (Blocking CI)

---

## Executive Summary

The E2E test workflow (`e2e-tests.yml`) is failing when attempting to start containers via `docker-compose.playwright-ci.yml`. The root cause is an incorrect Docker image reference format in the compose file that attempts to use a bare SHA256 digest instead of a fully-qualified image reference with registry and repository.

**Error Message**:
```
charon-app Error pull access denied for sha256, repository does not exist or may require 'docker login': denied: requested access to the resource is denied
```

**Root Cause**: The compose file's `image:` directive evaluates to a bare SHA256 digest (e.g., `sha256:057a9998...`) instead of a properly formatted image reference like `ghcr.io/wikid82/charon@sha256:057a9998...`.

---

## Issue 1: Nightly Build - GoReleaser macOS Cross-Compile Failure

### Problem Statement

The nightly build fails during GoReleaser release step when cross-compiling for macOS (darwin) using Zig:

```text
release failed after 4m19s
error=
  build failed: exit status 1: go: downloading github.com/gin-gonic/gin v1.11.0
  info: zig can provide libc for related target x86_64-macos.11-none
target=darwin_amd64_v1
```

### Root Cause Analysis

The `.goreleaser.yaml` darwin build uses incorrect Zig target specification:

**Current (WRONG):**
```yaml
CC=zig cc -target {{ if eq .Arch "amd64" }}x86_64{{ else }}aarch64{{ end }}-macos-gnu
CXX=zig c++ -target {{ if eq .Arch "amd64" }}x86_64{{ else }}aarch64{{ end }}-macos-gnu
```

**Issue:** macOS uses its own libc (libSystem), not GNU libc. The `-gnu` suffix is invalid for macOS targets. Zig expects `-macos-none` or `-macos.11-none` for macOS builds.

### Affected Files

| File | Change Type |
|------|-------------|
| `.goreleaser.yaml` | Fix Zig target for darwin builds |

### Recommended Fix

Update the darwin build configuration to use the correct Zig target triple:

**Option A: Use `-macos-none` (Recommended)**
```yaml
- id: darwin
  dir: backend
  main: ./cmd/api
  binary: charon
  env:
    - CGO_ENABLED=1
    - CC=zig cc -target {{ if eq .Arch "amd64" }}x86_64{{ else }}aarch64{{ end }}-macos-none
    - CXX=zig c++ -target {{ if eq .Arch "amd64" }}x86_64{{ else }}aarch64{{ end }}-macos-none
```

**Option B: Specify macOS version (for specific SDK compatibility)**
```yaml
    - CC=zig cc -target {{ if eq .Arch "amd64" }}x86_64{{ else }}aarch64{{ end }}-macos.11-none
    - CXX=zig c++ -target {{ if eq .Arch "amd64" }}x86_64{{ else }}aarch64{{ end }}-macos.11-none
```

**Option C: Remove darwin builds entirely (if macOS support is not required)**
```yaml
# Remove the entire `- id: darwin` build block from .goreleaser.yaml
# Update archives section to remove darwin from the `nix` archive builds
```

### Implementation Details

```diff
--- a/.goreleaser.yaml
+++ b/.goreleaser.yaml
@@ -47,8 +47,8 @@
     binary: charon
     env:
       - CGO_ENABLED=1
-      - CC=zig cc -target {{ if eq .Arch "amd64" }}x86_64{{ else }}aarch64{{ end }}-macos-gnu
-      - CXX=zig c++ -target {{ if eq .Arch "amd64" }}x86_64{{ else }}aarch64{{ end }}-macos-gnu
+      - CC=zig cc -target {{ if eq .Arch "amd64" }}x86_64{{ else }}aarch64{{ end }}-macos-none
+      - CXX=zig c++ -target {{ if eq .Arch "amd64" }}x86_64{{ else }}aarch64{{ end }}-macos-none
     goos:
       - darwin
     goarch:
```

### Verification

```bash
# Local test (requires Zig installed)
cd backend
CGO_ENABLED=1 CC="zig cc -target x86_64-macos-none" go build -o charon-darwin ./cmd/api

# Nightly workflow test
gh workflow run nightly-build.yml --ref development -f reason="Test darwin build fix"
```

---

## Issue 2: Playwright E2E - Admin API Socket Hang Up

### Problem Statement

Playwright test `zzz-admin-whitelist-blocking.spec.ts:126` fails with:

```text
Error: apiRequestContext.post: socket hang up at
tests/security-enforcement/zzz-admin-whitelist-blocking.spec.ts:126:21
```

The test POSTs to `http://localhost:2020/emergency/security-reset` but cannot reach the emergency server.

### Root Cause Analysis

The `playwright.yml` workflow starts the Charon container but **does not set** the `CHARON_EMERGENCY_BIND` environment variable:

**Current workflow (`.github/workflows/playwright.yml`):**
```yaml
docker run -d \
  --name charon-test \
  -p 8080:8080 \
  -p 127.0.0.1:2019:2019 \
  -p "[::1]:2019:2019" \
  -p 127.0.0.1:2020:2020 \
  -p "[::1]:2020:2020" \
  -e CHARON_ENV="${CHARON_ENV}" \
  -e CHARON_DEBUG="${CHARON_DEBUG}" \
  -e CHARON_ENCRYPTION_KEY="${CHARON_ENCRYPTION_KEY}" \
  -e CHARON_EMERGENCY_TOKEN="${CHARON_EMERGENCY_TOKEN}" \
  -e CHARON_EMERGENCY_SERVER_ENABLED="${CHARON_EMERGENCY_SERVER_ENABLED}" \
  "${IMAGE_REF}"
```

**Missing:** `CHARON_EMERGENCY_BIND=0.0.0.0:2020`

Without this variable, the emergency server may not bind to the correct address, or may bind to a loopback-only address that isn't accessible via Docker port mapping.

**Comparison with working compose file:**
```yaml
# .docker/compose/docker-compose.playwright-ci.yml
- CHARON_EMERGENCY_BIND=0.0.0.0:2020
- CHARON_EMERGENCY_USERNAME=admin
- CHARON_EMERGENCY_PASSWORD=changeme
```

### Affected Files

| File | Change Type |
|------|-------------|
| `.github/workflows/playwright.yml` | Add missing emergency server env vars |

### Recommended Fix

Add the missing emergency server environment variables to the docker run command:

```diff
--- a/.github/workflows/playwright.yml
+++ b/.github/workflows/playwright.yml
@@ -163,6 +163,10 @@ jobs:
             -e CHARON_ENCRYPTION_KEY="${CHARON_ENCRYPTION_KEY}" \
             -e CHARON_EMERGENCY_TOKEN="${CHARON_EMERGENCY_TOKEN}" \
             -e CHARON_EMERGENCY_SERVER_ENABLED="${CHARON_EMERGENCY_SERVER_ENABLED}" \
+            -e CHARON_EMERGENCY_BIND="0.0.0.0:2020" \
+            -e CHARON_EMERGENCY_USERNAME="admin" \
+            -e CHARON_EMERGENCY_PASSWORD="changeme" \
+            -e CHARON_SECURITY_TESTS_ENABLED="true" \
             "${IMAGE_REF}"
```

### Full Updated Step

```yaml
      - name: Start Charon container
        if: steps.check-artifact.outputs.artifact_exists == 'true'
        run: |
          echo "🚀 Starting Charon container..."

          # Normalize image name (GitHub lowercases repository owner names in GHCR)
          IMAGE_NAME=$(echo "${{ github.repository_owner }}/charon" | tr '[:upper:]' '[:lower:]')
          if [[ "${{ steps.pr-info.outputs.is_push }}" == "true" ]]; then
            IMAGE_REF="ghcr.io/${IMAGE_NAME}:${{ steps.sanitize.outputs.branch }}"
          else
            IMAGE_REF="ghcr.io/${IMAGE_NAME}:pr-${{ steps.pr-info.outputs.pr_number }}"
          fi

          echo "📦 Starting container with image: ${IMAGE_REF}"
          docker run -d \
            --name charon-test \
            -p 8080:8080 \
            -p 127.0.0.1:2019:2019 \
            -p "[::1]:2019:2019" \
            -p 127.0.0.1:2020:2020 \
            -p "[::1]:2020:2020" \
            -e CHARON_ENV="${CHARON_ENV}" \
            -e CHARON_DEBUG="${CHARON_DEBUG}" \
            -e CHARON_ENCRYPTION_KEY="${CHARON_ENCRYPTION_KEY}" \
            -e CHARON_EMERGENCY_TOKEN="${CHARON_EMERGENCY_TOKEN}" \
            -e CHARON_EMERGENCY_SERVER_ENABLED="${CHARON_EMERGENCY_SERVER_ENABLED}" \
            -e CHARON_EMERGENCY_BIND="0.0.0.0:2020" \
            -e CHARON_EMERGENCY_USERNAME="admin" \
            -e CHARON_EMERGENCY_PASSWORD="changeme" \
            -e CHARON_SECURITY_TESTS_ENABLED="true" \
            "${IMAGE_REF}"

          echo "✅ Container started"
```

### Verification

```bash
# After fix, verify emergency server is listening
docker exec charon-test curl -sf http://localhost:2020/health || echo "Failed"

# Test emergency reset endpoint
curl -X POST http://localhost:2020/emergency/security-reset \
  -H "Authorization: Basic $(echo -n 'admin:changeme' | base64)" \
  -H "X-Emergency-Token: $CHARON_EMERGENCY_TOKEN"
```

---

## Issue 3: Trivy Scan - Invalid Image Reference Format

### Problem Statement

Trivy scan fails with "invalid image reference format" when:
1. PR number is missing (manual dispatch without PR number)
2. Feature branch names contain `/` characters (e.g., `feature/new-thing`)
3. `is_push` and `pr_number` are both empty/false

Resulting in invalid Docker tags like:
- `ghcr.io/owner/charon:pr-` (empty PR number)
- `ghcr.io/owner/charon:` (no tag at all)

### Root Cause Analysis

**Location:** `.github/workflows/playwright.yml` - "Start Charon container" step

```bash
if [[ "${{ steps.pr-info.outputs.is_push }}" == "true" ]]; then
  IMAGE_REF="ghcr.io/${IMAGE_NAME}:${{ steps.sanitize.outputs.branch }}"
else
  IMAGE_REF="ghcr.io/${IMAGE_NAME}:pr-${{ steps.pr-info.outputs.pr_number }}"
fi
```

**Problem:** When `is_push != "true"` AND `pr_number` is empty, this creates:
```
IMAGE_REF="ghcr.io/owner/charon:pr-"
```

This is an invalid Docker reference.

### Affected Files

| File | Change Type |
|------|-------------|
| `.github/workflows/playwright.yml` | Add validation for IMAGE_REF |
| `.github/workflows/docker-build.yml` | Add validation guards (CVE verification step) |

### Recommended Fix

Add defensive validation to fail fast with a clear error message:

```diff
--- a/.github/workflows/playwright.yml
+++ b/.github/workflows/playwright.yml
           # Normalize image name (GitHub lowercases repository owner names in GHCR)
           IMAGE_NAME=$(echo "${{ github.repository_owner }}/charon" | tr '[:upper:]' '[:lower:]')

           if [[ "${{ steps.pr-info.outputs.is_push }}" == "true" ]]; then
             IMAGE_REF="ghcr.io/${IMAGE_NAME}:${{ steps.sanitize.outputs.branch }}"
-          else
+          elif [[ -n "${{ steps.pr-info.outputs.pr_number }}" ]]; then
             IMAGE_REF="ghcr.io/${IMAGE_NAME}:pr-${{ steps.pr-info.outputs.pr_number }}"
+          else
+            echo "❌ ERROR: Cannot determine image reference"
+            echo "  - is_push: ${{ steps.pr-info.outputs.is_push }}"
+            echo "  - pr_number: ${{ steps.pr-info.outputs.pr_number }}"
+            echo "  - branch: ${{ steps.sanitize.outputs.branch }}"
+            echo ""
+            echo "This can happen when:"
+            echo "  1. workflow_dispatch without pr_number input"
+            echo "  2. workflow_run triggered by non-PR, non-push event"
+            exit 1
           fi

+          # Validate the image reference format
+          if [[ ! "${IMAGE_REF}" =~ ^ghcr\.io/[a-z0-9_-]+/[a-z0-9_-]+:[a-zA-Z0-9._-]+$ ]]; then
+            echo "❌ ERROR: Invalid image reference format: ${IMAGE_REF}"
+            exit 1
+          fi
+
           echo "📦 Starting container with image: ${IMAGE_REF}"
```

### Additional Fix for docker-build.yml

The same issue can occur in `docker-build.yml` at the CVE verification step:

```yaml
# Line ~174 in docker-build.yml
if [ "${{ github.event_name }}" = "pull_request" ]; then
  IMAGE_REF="${{ env.GHCR_REGISTRY }}/${{ env.IMAGE_NAME }}:pr-${{ github.event.pull_request.number }}"
```

**Fix:**

```diff
--- a/.github/workflows/docker-build.yml
+++ b/.github/workflows/docker-build.yml
           # Determine the image reference based on event type
           if [ "${{ github.event_name }}" = "pull_request" ]; then
-            IMAGE_REF="${{ env.GHCR_REGISTRY }}/${{ env.IMAGE_NAME }}:pr-${{ github.event.pull_request.number }}"
+            PR_NUM="${{ github.event.pull_request.number }}"
+            if [ -z "${PR_NUM}" ]; then
+              echo "❌ ERROR: Pull request number is empty"
+              exit 1
+            fi
+            IMAGE_REF="${{ env.GHCR_REGISTRY }}/${{ env.IMAGE_NAME }}:pr-${PR_NUM}"
             echo "Using PR image: $IMAGE_REF"
           else
             IMAGE_REF="${{ env.GHCR_REGISTRY }}/${{ env.IMAGE_NAME }}@${{ steps.build-and-push.outputs.digest }}"
+            if [ -z "${{ steps.build-and-push.outputs.digest }}" ]; then
+              echo "❌ ERROR: Build digest is empty"
+              exit 1
+            fi
             echo "Using digest: $IMAGE_REF"
           fi
```

### Verification

```bash
# Test with empty PR number (should fail fast with clear error)
gh workflow run playwright.yml --ref development

# Check IMAGE_REF construction in logs
gh run view --log | grep "IMAGE_REF"
```

---

## Implementation Plan

### Phase 1: Immediate Fixes (Single PR)

**Objective:** Fix all three CI failures in a single PR for immediate resolution.

**Files to Modify:**

| File | Changes |
|------|---------|
| `.goreleaser.yaml` | Change `-macos-gnu` to `-macos-none` for darwin builds |
| `.github/workflows/playwright.yml` | Add missing emergency server env vars; Add IMAGE_REF validation |
| `.github/workflows/docker-build.yml` | Add IMAGE_REF validation guards |

### Phase 2: Verification

1. Push changes to a feature branch
2. Open PR to trigger docker-build.yml
3. Verify Trivy scan passes with valid IMAGE_REF
4. Verify Playwright workflow if triggered
5. Manually trigger nightly-build.yml with `--ref` pointing to feature branch
6. Verify darwin build succeeds

### Phase 3: Cleanup (Optional)

1. Add validation logic to a shared script (`scripts/validate-image-ref.sh`)
2. Add integration tests for emergency server connectivity
3. Document Zig target requirements for future contributors

---

## Requirements (EARS Notation)

1. WHEN GoReleaser builds darwin targets, THE SYSTEM SHALL use `-macos-none` Zig target (not `-macos-gnu`).
2. WHEN the Playwright workflow starts the Charon container, THE SYSTEM SHALL set `CHARON_EMERGENCY_BIND=0.0.0.0:2020` to ensure the emergency server is reachable.
3. WHEN constructing Docker image references, THE SYSTEM SHALL validate that the tag portion is non-empty before attempting to use it.
4. IF the PR number is empty in a PR-triggered workflow, THEN THE SYSTEM SHALL fail fast with a clear error message explaining the issue.
5. WHEN a feature branch contains `/` characters, THE SYSTEM SHALL sanitize the branch name by replacing `/` with `-` before using it as a Docker tag.

---

## Acceptance Criteria

1. [ ] Nightly build completes successfully with darwin binaries
2. [ ] Playwright E2E tests pass with emergency server accessible on port 2020
3. [ ] Trivy scan passes with valid image reference for all trigger types
4. [ ] Workflow failures produce clear, actionable error messages
5. [ ] No regression in existing CI functionality

---

## Risks & Mitigations

| Risk | Likelihood | Impact | Mitigation |
|------|------------|--------|------------|
| Zig target change breaks darwin binaries | Low | High | Test with local Zig build first |
| Emergency server env vars conflict with existing config | Low | Medium | Verify against docker-compose.playwright-ci.yml |
| IMAGE_REF validation too strict | Medium | Low | Use permissive regex, log values before validation |

---

## Handoff Contract

```json
{
  "plan": "CI Workflow Failures - Fix Plan",
  "status": "Ready for Implementation",
  "owner": "DevOps",
  "handoffTargets": ["Backend_Dev", "DevOps"],
  "files": [
    ".goreleaser.yaml",
    ".github/workflows/playwright.yml",
    ".github/workflows/docker-build.yml"
  ],
  "estimatedEffort": "2-3 hours",
  "priority": "HIGH",
  "blockedWorkflows": [
    "nightly-build.yml",
    "playwright.yml",
    "docker-build.yml (Trivy scan step)"
  ]
}
```

---

## References

- [docs/actions/nightly-build-failure.md](../actions/nightly-build-failure.md)
- [docs/actions/playwright-e2e-failures.md](../actions/playwright-e2e-failures.md)
- [Zig Cross-Compilation Targets](https://ziglang.org/documentation/master/#Targets)
- [GoReleaser CGO Cross-Compilation](https://goreleaser.com/customization/build/#cross-compiling)
