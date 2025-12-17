# PR #421: Docker Image Tag Invalid Reference Format Fix

## Issue Summary

**Problem**: CI/CD pipeline failure with error:

```
Using PR image: ghcr.io/wikid82/charon:pr-421/merge
docker: invalid reference format
```

**Root Cause**: Docker image tags cannot contain forward slashes (`/`). The `github.ref_name` context variable returns `421/merge` for PR merge refs, which when prefixed with `pr-` creates the invalid tag `pr-421/merge`.

---

## Files Requiring Modification

### 1. `.github/workflows/docker-build.yml`

#### Location 1: Line 101 - Metadata Tags

**Current Code (Lines 97-105):**

```yaml
        with:
          images: ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}
          tags: |
            type=raw,value=latest,enable={{is_default_branch}}
            type=raw,value=dev,enable=${{ github.ref == 'refs/heads/development' }}
            type=raw,value=beta,enable=${{ github.ref == 'refs/heads/feature/beta-release' }}
            type=raw,value=pr-${{ github.ref_name }},enable=${{ github.event_name == 'pull_request' }}
            type=sha,format=short,enable=${{ github.event_name != 'pull_request' }}
```

**Problem**: `github.ref_name` returns `421/merge` for PRs, creating invalid tag `pr-421/merge`.

**Fix**: Use `github.event.pull_request.number` instead, which returns just `421`.

```yaml
            type=raw,value=pr-${{ github.event.pull_request.number }},enable=${{ github.event_name == 'pull_request' }}
```

---

#### Location 2: Line 130 - Verify Caddy Security Patches Step

**Current Code (Lines 127-133):**

```yaml
          # Determine the image reference based on event type
          if [ "${{ github.event_name }}" = "pull_request" ]; then
            IMAGE_REF="${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}:pr-${{ github.ref_name }}"
            echo "Using PR image: $IMAGE_REF"
          else
            IMAGE_REF="${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}@${{ steps.build-and-push.outputs.digest }}"
            echo "Using digest: $IMAGE_REF"
          fi
```

**Problem**: Same issue - uses `github.ref_name` which contains `/`.

**Fix**:

```yaml
            IMAGE_REF="${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}:pr-${{ github.event.pull_request.number }}"
```

---

### 2. `.github/workflows/docker-publish.yml`

> **Note**: This file appears to be a near-duplicate of `docker-build.yml`. Consider consolidating them into a single workflow file.

#### Location 1: Line 104 - Metadata Tags

**Current Code (Lines 100-106):**

```yaml
        with:
          images: ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}
          tags: |
            type=raw,value=latest,enable={{is_default_branch}}
            type=raw,value=dev,enable=${{ github.ref == 'refs/heads/development' }}
            type=raw,value=beta,enable=${{ github.ref == 'refs/heads/feature/beta-release' }}
            type=raw,value=pr-${{ github.ref_name }},enable=${{ github.event_name == 'pull_request' }}
            type=sha,format=short,enable=${{ github.event_name != 'pull_request' }}
```

**Fix**: Same as docker-build.yml:

```yaml
            type=raw,value=pr-${{ github.event.pull_request.number }},enable=${{ github.event_name == 'pull_request' }}
```

---

## Locations That Are ALREADY CORRECT (No Changes Needed)

The following locations use `github.sha` which is always valid (no slashes):

| File | Line | Code | Status |
|------|------|------|--------|
| docker-build.yml | 327 | `docker build -t charon:pr-${{ github.sha }} .` | ✅ OK |
| docker-build.yml | 331 | `CONTAINER=$(docker create charon:pr-${{ github.sha }})` | ✅ OK |
| docker-publish.yml | 267 | `docker build -t charon:pr-${{ github.sha }} .` | ✅ OK |
| docker-publish.yml | 271 | `CONTAINER=$(docker create charon:pr-${{ github.sha }})` | ✅ OK |

These use `github.sha` (a hex string like `abc1234...`) which never contains slashes.

---

## Proposed Fix Summary

### Changes Required

| File | Line | Change |
|------|------|--------|
| `.github/workflows/docker-build.yml` | 101 | `github.ref_name` → `github.event.pull_request.number` |
| `.github/workflows/docker-build.yml` | 130 | `github.ref_name` → `github.event.pull_request.number` |
| `.github/workflows/docker-publish.yml` | 104 | `github.ref_name` → `github.event.pull_request.number` |

### Result

- **Before**: `ghcr.io/wikid82/charon:pr-421/merge` (INVALID)
- **After**: `ghcr.io/wikid82/charon:pr-421` (VALID)

---

## Alternative Approaches Considered

### Option A: Use PR Number (RECOMMENDED)

```yaml
type=raw,value=pr-${{ github.event.pull_request.number }},enable=${{ github.event_name == 'pull_request' }}
```

- **Pros**: Clean, human-readable, matches common patterns (`pr-421`)
- **Cons**: None

### Option B: Replace Slashes with Dashes

```yaml
type=raw,value=pr-${{ github.ref_name | replace('/', '-') }},enable=${{ github.event_name == 'pull_request' }}
```

- **Pros**: Preserves full ref info
- **Cons**: GitHub Actions expressions don't support `replace()` filter. Would require a separate step.

### Option C: Use Short SHA

```yaml
type=raw,value=pr-${{ github.event.pull_request.head.sha | truncate(7) }},enable=${{ github.event_name == 'pull_request' }}
```

- **Pros**: Unique identifier
- **Cons**: Less human-friendly, harder to correlate with PR

---

## Implementation Checklist

- [ ] Update `.github/workflows/docker-build.yml` line 101
- [ ] Update `.github/workflows/docker-build.yml` line 130
- [ ] Update `.github/workflows/docker-publish.yml` line 104
- [ ] Test by creating a new PR and verifying the image tag is valid
- [ ] Consider consolidating `docker-build.yml` and `docker-publish.yml` (future cleanup)

---

## Testing Plan

1. Create a test PR after implementing the fix
2. Verify the workflow step "Extract metadata (tags, labels)" shows tag like `pr-<number>` (no slashes)
3. Verify the "Verify Caddy Security Patches" step can pull the correct image reference
4. Confirm no `invalid reference format` errors in CI logs

---

*Plan created: December 17, 2025*
*Priority: 🔴 CRITICAL - Blocks PR #421 CI/CD*
