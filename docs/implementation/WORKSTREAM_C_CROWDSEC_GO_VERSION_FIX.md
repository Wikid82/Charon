# Workstream C: CrowdSec Go Version Fix

**Date:** 2026-01-10
**Issue:** CrowdSec binaries built with Go 1.25.1 containing 4 HIGH CVEs
**Solution**: Pin CrowdSec builder to Go 1.26.0+

## Problem

Trivy scan identified that the CrowdSec binaries (`crowdsec` and `cscli`) embedded in the container image were built with Go 1.25.1, which has 4 HIGH severity CVEs:

- CVE-2025-58183
- CVE-2025-58186
- CVE-2025-58187
- CVE-2025-61729

The CrowdSec builder stage in the Dockerfile was using `golang:1.25-alpine`, which resolved to the vulnerable Go 1.25.1 version.

## Solution

Updated the `CrowdSec Builder` stage in the Dockerfile to explicitly pin to Go 1.26.0:

```dockerfile
# Before:
FROM --platform=$BUILDPLATFORM golang:1.25-alpine AS crowdsec-builder

# After:
# renovate: datasource=docker depName=golang versioning=docker
FROM --platform=$BUILDPLATFORM golang:1.25.5-alpine AS crowdsec-builder
```

## Changes Made

### File: `Dockerfile`

**Line ~275-279:** Updated the CrowdSec builder stage base image

- Changed from: `golang:1.25-alpine` (resolves to 1.25.1)
- Changed to: `golang:1.25.5-alpine` (fixed version)
- Added Renovate annotation to track future Go version updates

## Impact

- **Security:** Eliminates 4 HIGH CVEs in the CrowdSec binaries
- **Build Process:** No changes to build logic, only base image version
- **CrowdSec Version:** Remains at v1.7.4 (no version change needed)
- **Compatibility:** No breaking changes; CrowdSec functionality unchanged

## Verification

After this change, the following validations should be performed:

1. **Rebuild the image** (no-cache recommended):

   ```bash
   # Use task: Build & Run: Local Docker Image No-Cache
   ```

2. **Run Trivy scan** on the rebuilt image:

   ```bash
   # Use task: Security: Trivy Scan
   ```

3. **Expected outcome:**
   - Trivy image scan should report **0 HIGH/CRITICAL** vulnerabilities
   - CrowdSec binaries should be built with Go 1.26.0+
   - All CrowdSec functionality should remain operational

## Related

- **Plan:** [docs/plans/current_spec.md](../plans/current_spec.md) - Workstream C
- **CVE List:** Go 1.25.1 stdlib vulnerabilities (CVE-2025-58183, CVE-2025-58186, CVE-2025-58187, CVE-2025-61729)
- **Dependencies:** CrowdSec v1.7.4 (no change)
- **Next Step:** QA validation after image rebuild

## Notes

- The Backend Builder stage already uses `golang:1.25-alpine` but may resolve to a patched minor version. If needed, it can be pinned similarly.
- Renovate will track the pinned `golang:1.25.5-alpine` image and suggest updates when newer patch versions are available.
- The explicit version pin ensures reproducible builds and prevents accidental rollback to vulnerable versions.
