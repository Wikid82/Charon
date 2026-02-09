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

## Root Cause Analysis

### Current Implementation (Broken)

**File**: `.docker/compose/docker-compose.playwright-ci.yml`
**Lines**: 29-37

```yaml
charon-app:
  # CI default (digest-pinned via workflow output):
  # CHARON_E2E_IMAGE_DIGEST=ghcr.io/wikid82/charon:nightly@sha256:<digest>
  # Local override (tag-based):
  # CHARON_E2E_IMAGE=charon:e2e-test
  image: ${CHARON_E2E_IMAGE_DIGEST:-${CHARON_E2E_IMAGE:-charon:e2e-test}}
```

### Workflow Environment Variable

**File**: `.github/workflows/e2e-tests.yml`
**Line**: 158

```yaml
env:
  CHARON_E2E_IMAGE_DIGEST: ${{ needs.build.outputs.image_digest }}
```

**Problem**: The `needs.build.outputs.image_digest` from the `build` job in `e2e-tests.yml` returns **only the SHA256 digest** (e.g., `sha256:057a9998fa7a5b224a06ec8989c892d2ac8f9323530470965baaf5fcaab7557c`), not a fully-qualified image reference.

### Why Docker Fails

Docker Compose interprets the `image:` field as:
- `sha256:057a9998...` ← **Bare digest, no registry/repository**

Docker then tries to:
1. Parse this as a repository name
2. Look for a repository literally named "sha256"
3. Fail with "pull access denied" because no such repository exists

### Correct Reference Format

Docker requires one of these formats:
1. **Tag-based**: `charon:e2e-test` (local image)
2. **Digest-pinned**: `ghcr.io/wikid82/charon@sha256:057a9998...` (registry + repo + digest)

---

## Technical Investigation

### How the Image is Built and Loaded

**Workflow Flow** (`e2e-tests.yml`):

1. **Build Job** (lines 90-148):
   - Builds Docker image with tag `charon:e2e-test`
   - Saves image to `charon-e2e-image.tar` artifact
   - Outputs image digest from build step

2. **E2E Test Job** (lines 173-177):
   - Downloads `charon-e2e-image.tar` artifact
   - Loads image with: `docker load -i charon-e2e-image.tar`
   - **Loaded image has tag**: `charon:e2e-test` (from build step)

3. **Start Container** (line 219):
   - Runs: `docker compose -f .docker/compose/docker-compose.playwright-ci.yml up -d`
   - Compose file tries to use `$CHARON_E2E_IMAGE_DIGEST` (bare SHA256)
   - **Docker cannot find image** because the digest doesn't match loaded tag

### Mismatch Between Build and Reference

| Step | Image Reference | Status |
|------|----------------|--------|
| Build | `charon:e2e-test` | ✅ Image tagged |
| Save/Load | `charon:e2e-test` | ✅ Tag preserved in tar |
| Compose | `sha256:057a9998...` | ❌ Wrong reference type |

**The loaded image is available as `charon:e2e-test`, but the compose file is looking for `sha256:...`**

---

## Comparison with Working Workflow

### `playwright.yml` (Working) vs `e2e-tests.yml` (Broken)

**playwright.yml** (lines 207-209):
```yaml
- name: Load Docker image
  run: |
    docker load < charon-pr-image.tar
    docker images | grep charon
```

**Container Start** (lines 213-277):
```yaml
- name: Start Charon container
  run: |
    # Explicitly constructs image reference from variables
    IMAGE_NAME=$(echo "${{ github.repository_owner }}/charon" | tr '[:upper:]' '[:lower:]')
    IMAGE_REF="ghcr.io/${IMAGE_NAME}:pr-${{ steps.pr-info.outputs.pr_number }}"

    docker run -d \
      --name charon-test \
      -e CHARON_ENV="${CHARON_ENV}" \
      # ... (uses constructed IMAGE_REF)
```

**Key Difference**: `playwright.yml` uses `docker run` directly with explicit image reference construction, not Docker Compose with environment variable substitution.

---

## Solution Architecture

### Option 1: Use Local Tag Reference (Recommended)

**Rationale**: The loaded image is already tagged as `charon:e2e-test`. We should use this tag directly instead of trying to use a digest.

**Change**: Set `CHARON_E2E_IMAGE_DIGEST` to the **tag** instead of the digest, or use a different variable name.

### Option 2: Re-tag Image with Digest

**Rationale**: Re-tag the loaded image to match the digest-based reference expected by the compose file.

**Change**: After loading, re-tag the image with the full digest reference.

### Option 3: Simplify Compose File

**Rationale**: Remove the digest-based environment variable and always use the local tag for CI.

**Change**: Hard-code `charon:e2e-test` or use a simpler env var pattern.

---

## Recommended Solution: Option 1 (Modified Approach)

### Strategy

**Use the pre-built tag for CI, not the digest.** The digest output from the build is metadata but not needed for referencing a locally loaded image.

### Implementation

#### Change 1: Remove Digest from Workflow Environment

**File**: `.github/workflows/e2e-tests.yml`
**Lines**: 155-158

**Current**:
```yaml
env:
  # Required for security teardown (emergency reset fallback when ACL blocks API)
  CHARON_EMERGENCY_TOKEN: ${{ secrets.CHARON_EMERGENCY_TOKEN }}
  # Enable security-focused endpoints and test gating
  CHARON_EMERGENCY_SERVER_ENABLED: "true"
  CHARON_SECURITY_TESTS_ENABLED: "true"
  CHARON_E2E_IMAGE_DIGEST: ${{ needs.build.outputs.image_digest }}
```

**Corrected**:
```yaml
env:
  # Required for security teardown (emergency reset fallback when ACL blocks API)
  CHARON_EMERGENCY_TOKEN: ${{ secrets.CHARON_EMERGENCY_TOKEN }}
  # Enable security-focused endpoints and test gating
  CHARON_EMERGENCY_SERVER_ENABLED: "true"
  CHARON_SECURITY_TESTS_ENABLED: "true"
  # Use local tag for pre-built image (loaded from artifact)
  CHARON_E2E_IMAGE: charon:e2e-test
```

**Rationale**:
- The `docker load` command restores the image with its original tag `charon:e2e-test`
- We should use this tag, not the digest
- The digest is only useful for verifying image integrity, not for referencing locally loaded images

#### Change 2: Update Compose File Comment Documentation

**File**: `.docker/compose/docker-compose.playwright-ci.yml`
**Lines**: 31-37

**Current**:
```yaml
  charon-app:
    # CI default (digest-pinned via workflow output):
    # CHARON_E2E_IMAGE_DIGEST=ghcr.io/wikid82/charon:nightly@sha256:<digest>
    # Local override (tag-based):
    # CHARON_E2E_IMAGE=charon:e2e-test
    image: ${CHARON_E2E_IMAGE_DIGEST:-${CHARON_E2E_IMAGE:-charon:e2e-test}}
```

**Corrected**:
```yaml
  charon-app:
    # CI default: Uses pre-built image loaded from artifact
    # Set via workflow: CHARON_E2E_IMAGE=charon:e2e-test
    # Local development: Uses locally built image
    # Override with: CHARON_E2E_IMAGE=charon:local-dev
    image: ${CHARON_E2E_IMAGE:-charon:e2e-test}
```

**Rationale**:
- Simplify the environment variable fallback chain
- Remove confusing `CHARON_E2E_IMAGE_DIGEST` variable that was set incorrectly
- Document the actual behavior: CI loads pre-built image with known tag
- Make local development override clearer

---

## Alternative Solution: Option 2 (If Digest-Pinning Required)

If there's a requirement to use digest-based references for security/reproducibility, we must re-tag the loaded image.

### Implementation

#### Change 1: Re-tag After Load

**File**: `.github/workflows/e2e-tests.yml`
**After Line**: 177 (in "Load Docker image" step)

**Add**:
```yaml
      - name: Load and re-tag Docker image
        run: |
          # Load the pre-built image
          docker load -i charon-e2e-image.tar
          docker images | grep charon

          # Re-tag for digest-based reference if needed
          IMAGE_DIGEST="${{ needs.build.outputs.image_digest }}"
          if [[ -n "$IMAGE_DIGEST" ]]; then
            # Extract just the digest hash (sha256:...)
            DIGEST_HASH=$(echo "$IMAGE_DIGEST" | grep -oP 'sha256:[a-f0-9]{64}')

            # Construct full reference
            FULL_REF="ghcr.io/wikid82/charon@${DIGEST_HASH}"

            echo "Re-tagging charon:e2e-test as $FULL_REF"
            docker tag charon:e2e-test "$FULL_REF"

            # Export for compose file
            echo "CHARON_E2E_IMAGE_DIGEST=$FULL_REF" >> $GITHUB_ENV
          else
            # Fallback to tag-based reference
            echo "CHARON_E2E_IMAGE=charon:e2e-test" >> $GITHUB_ENV
          fi
```

#### Change 2: Update Compose File

**File**: `.docker/compose/docker-compose.playwright-ci.yml`
**Lines**: 31-37

Keep the current implementation but fix the comment:

```yaml
  charon-app:
    # CI: Digest-pinned reference (re-tagged from loaded artifact)
    #     CHARON_E2E_IMAGE_DIGEST=ghcr.io/wikid82/charon@sha256:<digest>
    # Local: Tag-based reference for development
    #     CHARON_E2E_IMAGE=charon:e2e-test
    image: ${CHARON_E2E_IMAGE_DIGEST:-${CHARON_E2E_IMAGE:-charon:e2e-test}}
```

**Rationale**:
- Preserves digest-based pinning for supply chain security
- Re-tagging creates a local image reference that Docker can resolve
- Falls back gracefully to tag-based reference for local development

---

## Recommended Approach: Option 1 (Simplicity)

**Why Option 1**:
1. **Simpler**: No re-tagging logic needed
2. **Faster**: Fewer Docker operations
3. **Sufficient**: The image is already built and loaded; tag reference is adequate
4. **Consistent**: Matches how `playwright.yml` handles loaded images
5. **Local-first**: The image is local after `docker load`, not in a registry

**When to use Option 2**:
- If there's a compliance requirement to use digest references
- If SBOM/attestation workflows need digest traceability
- If multi-registry scenarios require content-addressable references

---

## Implementation Steps

### Phase 1: Apply Recommended Fix (Option 1)

1. **Update workflow environment variables**
   - File: `.github/workflows/e2e-tests.yml`
   - Line: 158
   - Change: Replace `CHARON_E2E_IMAGE_DIGEST` with `CHARON_E2E_IMAGE: charon:e2e-test`

2. **Update compose file documentation**
   - File: `.docker/compose/docker-compose.playwright-ci.yml`
   - Lines: 31-37
   - Change: Simplify variable fallback and update comments

3. **Verify changes**
   - Run: `docker compose -f .docker/compose/docker-compose.playwright-ci.yml config`
   - Ensure: `image: charon:e2e-test` in output
   - Validate: No environment variable warnings

### Phase 2: Test in CI

1. **Create test PR**
   - Branch: `fix/docker-compose-image-reference`
   - Include: Both file changes from Phase 1

2. **Monitor workflow execution**
   - Watch: `e2e-tests.yml` workflow
   - Check: "Start test environment" step succeeds
   - Verify: Container starts and health check passes

3. **Validate container**
   - Check: `docker ps` shows `charon-playwright` running
   - Test: Health endpoint responds at `http://localhost:8080/api/v1/health`
   - Confirm: Playwright tests execute successfully

### Phase 3: Documentation Update

1. **Update workflow documentation**
   - File: `.github/workflows/e2e-tests.yml`
   - Section: Top-level comments (lines 1-29)
   - Add: Note about using local tag vs. digest

2. **Update compose file documentation**
   - File: `.docker/compose/docker-compose.playwright-ci.yml`
   - Section: Usage section (lines 11-16)
   - Clarify: Environment variable expectations

---

## Verification Checklist

### Pre-Deployment Validation

- [ ] **Syntax Check**: Run `docker compose config` with test environment variables
- [ ] **Variable Resolution**: Confirm `image:` field resolves to `charon:e2e-test`
- [ ] **Local Test**: Load image locally and run compose up
- [ ] **Workflow Dry-run**: Test changes in a draft PR before merging

### CI Validation Points

- [ ] **Build Job**: Completes successfully, uploads image artifact
- [ ] **Download**: Image artifact downloads correctly
- [ ] **Load**: `docker load` succeeds, image appears in `docker images`
- [ ] **Compose Up**: Container starts without pull errors
- [ ] **Health Check**: Container becomes healthy within timeout
- [ ] **Test Execution**: Playwright tests run and report results

### Post-Deployment Monitoring

- [ ] **Success Rate**: Monitor e2e-tests.yml success rate for 10 runs
- [ ] **Startup Time**: Verify container startup time remains under 30s
- [ ] **Resource Usage**: Check for memory/CPU regressions
- [ ] **Flake Rate**: Ensure no new test flakiness introduced

---

## Risk Assessment

### Low Risk Changes
✅ Workflow environment variable change (isolated to CI)
✅ Compose file comment updates (documentation only)

### Medium Risk Changes
⚠️ Compose file `image:` field modification
- **Mitigation**: Test locally before pushing
- **Rollback**: Revert single line in compose file

### No Risk
✅ Read-only investigation and analysis
✅ Documentation improvements

---

## Rollback Plan

### If Option 1 Fails

**Symptoms**:
- Container still fails to start
- Error: "No such image: charon:e2e-test"

**Rollback**:
```bash
git revert <commit-hash>  # Revert the workflow change
```

**Alternative Fix**: Switch to Option 2 (re-tagging approach)

### If Option 2 Fails

**Symptoms**:
- Re-tag logic fails
- Digest extraction errors

**Rollback**:
1. Remove re-tagging step
2. Fall back to simple tag reference: `CHARON_E2E_IMAGE=charon:e2e-test`

---

## Success Metrics

### Immediate Success Indicators
- ✅ `docker compose up` starts container without errors
- ✅ Container health check passes within 30 seconds
- ✅ Playwright tests execute (pass or fail is separate concern)

### Long-term Success Indicators
- ✅ E2E workflow success rate returns to baseline (>95%)
- ✅ No image reference errors in CI logs for 2 weeks
- ✅ Local development workflow unaffected

---

## Related Issues and Context

### Why Was Digest Being Used?

**Comment from compose file** (line 33):
```yaml
# CHARON_E2E_IMAGE_DIGEST=ghcr.io/wikid82/charon:nightly@sha256:<digest>
```

**Hypothesis**: The original intent was to support digest-pinned references for security/reproducibility, but the implementation was incomplete:
1. The workflow sets only the digest hash, not the full reference
2. The compose file expects the full reference format
3. No re-tagging step bridges the gap

### Why Does playwright.yml Work?

**Key difference** (lines 213-277):
- Uses `docker run` directly with explicit image reference
- Constructs full `ghcr.io/...` reference from variables
- Does not rely on environment variable substitution in compose file

**Lesson**: Direct Docker commands give more control than Compose environment variable interpolation.

---

## Dependencies

### Required Secrets
- ✅ `CHARON_EMERGENCY_TOKEN` (already configured)
- ✅ `CHARON_CI_ENCRYPTION_KEY` (generated in workflow)

### Required Tools
- ✅ Docker Compose (available in GitHub Actions)
- ✅ Docker CLI (available in GitHub Actions)

### No External Dependencies
- ✅ No registry authentication needed (local image)
- ✅ No network calls required (image pre-loaded)

---

## Timeline

| Phase | Duration | Blocking |
|-------|----------|----------|
| **Analysis & Planning** | Complete | ✅ |
| **Implementation** | 30 minutes | ⏳ |
| **Testing (PR)** | 10-15 minutes (CI runtime) | ⏳ |
| **Verification** | 2 hours (10 workflow runs) | ⏳ |
| **Documentation** | 15 minutes | ⏳ |

**Estimated Total**: 3-4 hours from start to complete verification

---

## Next Actions

1. **Immediate**: Implement Option 1 changes (2 file modifications)
2. **Test**: Create PR and monitor e2e-tests.yml workflow
3. **Verify**: Check container startup and health check success
4. **Document**: Update this plan with results
5. **Close**: Mark as complete once verified in main branch

---

## Appendix: Full File Changes

### File 1: `.github/workflows/e2e-tests.yml`

**Line 158**: Change environment variable

```diff
  e2e-tests:
    name: E2E Tests (Shard ${{ matrix.shard }}/${{ matrix.total-shards }})
    runs-on: ubuntu-latest
    needs: build
    timeout-minutes: 30
    env:
      # Required for security teardown (emergency reset fallback when ACL blocks API)
      CHARON_EMERGENCY_TOKEN: ${{ secrets.CHARON_EMERGENCY_TOKEN }}
      # Enable security-focused endpoints and test gating
      CHARON_EMERGENCY_SERVER_ENABLED: "true"
      CHARON_SECURITY_TESTS_ENABLED: "true"
-     CHARON_E2E_IMAGE_DIGEST: ${{ needs.build.outputs.image_digest }}
+     # Use local tag for pre-built image (loaded from artifact)
+     CHARON_E2E_IMAGE: charon:e2e-test
```

### File 2: `.docker/compose/docker-compose.playwright-ci.yml`

**Lines 31-37**: Simplify image reference

```diff
  charon-app:
-   # CI default (digest-pinned via workflow output):
-   # CHARON_E2E_IMAGE_DIGEST=ghcr.io/wikid82/charon:nightly@sha256:<digest>
-   # Local override (tag-based):
+   # CI default: Uses pre-built image loaded from artifact
+   # Set via workflow: CHARON_E2E_IMAGE=charon:e2e-test
+   # Local development: Uses locally built image
+   # Override with: CHARON_E2E_IMAGE=charon:local-dev
-   image: ${CHARON_E2E_IMAGE_DIGEST:-${CHARON_E2E_IMAGE:-charon:e2e-test}}
+   image: ${CHARON_E2E_IMAGE:-charon:e2e-test}
```

---

**End of Remediation Plan**
