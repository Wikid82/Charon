# Docker Compose CI Fix - Quick Reference

**Document**: [Full Remediation Plan](docker_compose_ci_fix.md)
**Status**: Ready for Implementation
**Priority**: CRITICAL

---

## Problem

E2E tests failing with:
```
charon-app Error pull access denied for sha256, repository does not exist
```

## Root Cause

The workflow passes **bare SHA256 digest** to Docker Compose:
```yaml
CHARON_E2E_IMAGE_DIGEST: sha256:057a9998...
```

Docker tries to pull from a repository named "sha256" (doesn't exist).

## Solution

Use the **local tag** that already exists after `docker load`:

### Change 1: Workflow

**File**: `.github/workflows/e2e-tests.yml` (line 158)

```diff
- CHARON_E2E_IMAGE_DIGEST: ${{ needs.build.outputs.image_digest }}
+ # Use local tag for pre-built image (loaded from artifact)
+ CHARON_E2E_IMAGE: charon:e2e-test
```

### Change 2: Compose File

**File**: `.docker/compose/docker-compose.playwright-ci.yml` (lines 31-37)

```diff
- # CI default (digest-pinned via workflow output):
- # CHARON_E2E_IMAGE_DIGEST=ghcr.io/wikid82/charon:nightly@sha256:<digest>
- # Local override (tag-based):
+ # CI default: Uses pre-built image loaded from artifact
+ # Set via workflow: CHARON_E2E_IMAGE=charon:e2e-test
+ # Local development: Uses locally built image
+ # Override with: CHARON_E2E_IMAGE=charon:local-dev
- image: ${CHARON_E2E_IMAGE_DIGEST:-${CHARON_E2E_IMAGE:-charon:e2e-test}}
+ image: ${CHARON_E2E_IMAGE:-charon:e2e-test}
```

## Why This Works

| Step | Current (Broken) | Fixed |
|------|-----------------|-------|
| Build | Tags as `charon:e2e-test` | Same |
| Load | Image available as `charon:e2e-test` | Same |
| Compose | Tries to use `sha256:...` ❌ | Uses `charon:e2e-test` ✅ |

## Verification

```bash
# After changes, run locally:
export CHARON_E2E_IMAGE=charon:e2e-test
docker compose -f .docker/compose/docker-compose.playwright-ci.yml config | grep "image:"

# Should output:
# image: charon:e2e-test
```

## Testing

1. Create PR with both changes
2. Monitor `e2e-tests.yml` workflow
3. Verify "Start test environment" step succeeds
4. Confirm health check passes

---

**See [docker_compose_ci_fix.md](docker_compose_ci_fix.md) for full analysis and implementation details.**
