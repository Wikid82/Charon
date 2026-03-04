# Workflow Review - Emergency Token & Docker Registry Strategy
**Date**: January 26, 2026
**Status**: ✅ Critical fixes applied
**PR**: #550 (Docker Debian Trixie migration)

## Critical Issue Fixed ❌→✅

### Problem
All E2E test workflows were missing `CHARON_EMERGENCY_TOKEN` environment variable, causing security teardown failures identical to the local issue we just resolved.

**Impact**:
- Security teardown would fail with 501 "not configured" error
- Caused cascading test failures (83 tests blocked by ACL)
- CI/CD pipeline would report false failures

### Solution Applied
Added `CHARON_EMERGENCY_TOKEN: ${{ secrets.CHARON_EMERGENCY_TOKEN }}` to environment variables in:

1. **`.github/workflows/docker-build.yml`** → `test-image` job
2. **`.github/workflows/e2e-tests.yml`** → `e2e-tests` job
3. **`.github/workflows/playwright.yml`** → `playwright` job

**Before**:
```yaml
jobs:
  test-image:
    name: Test Docker Image
    runs-on: ubuntu-latest
    steps: ...
```

**After**:
```yaml
jobs:
  test-image:
    name: Test Docker Image
    runs-on: ubuntu-latest
    env:
      # Required for security teardown in integration tests
      CHARON_EMERGENCY_TOKEN: ${{ secrets.CHARON_EMERGENCY_TOKEN }}
    steps: ...
```

---

## Docker Registry Strategy Review ✅

### Current Setup (Optimal)
**`docker-build.yml`** implements the recommended "build once, push twice" strategy:

```yaml
- name: Build and push Docker image
  uses: docker/build-push-action@v6
  with:
    push: ${{ github.event_name != 'pull_request' }}
    tags: ${{ steps.meta.outputs.tags }}  # Contains both GHCR + Docker Hub tags

- name: Sign GHCR Image
  run: cosign sign --yes ${{ env.GHCR_REGISTRY }}/...@${{ digest }}

- name: Sign Docker Hub Image
  run: cosign sign --yes ${{ env.DOCKERHUB_REGISTRY }}/...@${{ digest }}
```

**Verification**:
✅ Single multi-arch build
✅ Same digest pushed to both registries
✅ Both images signed with Cosign
✅ SBOM generated and attached
✅ No duplicate builds or testing

### Why This Is Correct
- **Immutable artifact**: One build = one digest = one set of binaries
- **Efficient**: No rebuilding or re-testing needed
- **Supply chain security**: Same SBOM and signatures for both registries
- **Cost-effective**: Minimal CI/CD minutes

---

## Testing Strategy Review ✅

### Current Approach
Tests are run **once** against the built image (by digest), not separately per registry:

```yaml
test-image:
  steps:
    - name: Pull Docker image
      run: docker pull ${{ env.GHCR_REGISTRY }}/${{ env.IMAGE_NAME }}:${{ steps.tag.outputs.tag }}
    - name: Run Integration Test
      run: ./scripts/integration-test.sh
```

**Why This Is Correct**:
- If the image digest is identical across registries (which it is), testing once validates both
- Registry-specific concerns (access, visibility) are tested by push/pull operations themselves
- E2E tests focus on **application functionality**, not registry operations

---

## Recommendations for GitHub Secrets

### Required Repository Secrets
Add these to **Settings → Secrets and variables → Actions → Repository secrets**:

| Secret Name | Purpose | How to Generate | Status |
|------------|---------|-----------------|--------|
| `CHARON_EMERGENCY_TOKEN` | Security teardown in E2E tests | `openssl rand -hex 32` | ⚠️ **Missing** |
| `CHARON_CI_ENCRYPTION_KEY` | Database encryption in tests | `openssl rand -base64 32` | ✅ Exists |
| `DOCKERHUB_USERNAME` | Docker Hub authentication | Your Docker Hub username | ✅ Exists |
| `DOCKERHUB_TOKEN` | Docker Hub push access | Create at hub.docker.com/settings/security | ✅ Exists |
| `CODECOV_TOKEN` | Coverage upload | From codecov.io project settings | ✅ Exists |

### Action Required ⚠️
```bash
# Generate emergency token for CI (same format as local .env)
openssl rand -hex 32

# Add as CHARON_EMERGENCY_TOKEN in GitHub repo secrets
# Navigate to: https://github.com/Wikid82/Charon/settings/secrets/actions/new
```

---

## Smoke Test Command (Optional Enhancement)

To add explicit registry verification, consider this optional enhancement to `docker-build.yml`:

```yaml
- name: Verify Both Registries (Optional Smoke Test)
  if: github.event_name != 'pull_request'
  run: |
    # Pull from GHCR
    docker pull ${{ env.GHCR_REGISTRY }}/${{ env.IMAGE_NAME }}:latest
    GHCR_DIGEST=$(docker inspect --format='{{index .RepoDigests 0}}' ...)

    # Pull from Docker Hub
    docker pull ${{ env.DOCKERHUB_REGISTRY }}/${{ env.IMAGE_NAME }}:latest
    DOCKERHUB_DIGEST=$(docker inspect --format='{{index .RepoDigests 0}}' ...)

    # Compare digests
    if [[ "$GHCR_DIGEST" != "$DOCKERHUB_DIGEST" ]]; then
      echo "❌ Digest mismatch between registries!"
      exit 1
    fi

    # Verify signatures exist
    cosign verify $GHCR_DIGEST
    cosign verify $DOCKERHUB_DIGEST
```

**Recommendation**: This is **optional** and adds ~30 seconds to CI. Only add if you've experienced registry sync issues in the past.

---

## Container Prune Workflow Added ✅

A new scheduled workflow and helper script were added to safely prune old container images from both **GHCR** and **Docker Hub**.

- **Files added**:
  - `.github/workflows/container-prune.yml` (weekly schedule, manual dispatch)
  - `scripts/prune-ghcr.sh` (GHCR cleanup)
  - `scripts/prune-dockerhub.sh` (Docker Hub cleanup)

- **Behavior**:
  - Default: **dry-run=true** (no destructive changes).
  - Uses `GITHUB_TOKEN` for GHCR package deletions (workflow permission `packages: write` is set).
  - Uses `DOCKERHUB_USERNAME` and `DOCKERHUB_TOKEN` secrets for Docker Hub deletions.
  - Honours protected patterns by default: `v*`, `latest`, `main`, `develop`.
  - Configurable inputs: registries, keep_days, keep_last_n, dry_run.

- **Secrets required**:
  - `DOCKERHUB_USERNAME` (existing)
  - `DOCKERHUB_TOKEN` (existing)
  - `GITHUB_TOKEN` (provided by Actions)

- **How to run**:
  - Manually: `Actions → Container Registry Prune → Run workflow` (adjust inputs as needed)
  - Scheduled: runs weekly (Sundays 03:00 UTC) by default

- **Safety**: The workflow is conservative and will only delete when `dry_run=false` is explicitly set; it is recommended to run a few dry-runs and review candidates before enabling deletions.

---

## Summary

### ✅ What Was Fixed
1. **Critical**: Added `CHARON_EMERGENCY_TOKEN` to all E2E workflow environments
2. **Verified**: Docker build/push strategy is optimal (no changes needed)
3. **Confirmed**: Test strategy is correct (no duplicate testing needed)

### ⚠️ Action Required
- Add `CHARON_EMERGENCY_TOKEN` secret to GitHub repository (generate with `openssl rand -hex 32`)

### ✅ Already Optimal
- Docker multi-registry push strategy
- Image signing and SBOM generation
- Test execution approach

---

## Files Modified
- `.github/workflows/docker-build.yml`
- `.github/workflows/e2e-tests.yml`
- `.github/workflows/playwright.yml`

## Related
- Issue: Security teardown failures in CI
- Fix: Backend emergency endpoint rate limit removal (PR #550)
- Docs: `.env` setup for local development
