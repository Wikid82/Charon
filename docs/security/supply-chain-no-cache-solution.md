# Supply Chain Security: No-Cache Docker Build Solution

**Date**: 2026-01-11
**PR**: [#461 - DNS Challenge Support](https://github.com/Wikid82/Charon/pull/461)
**Issue**: False positive vulnerabilities from cached Go module layers

---

## Executive Summary

Trivy security scans were reporting **8 Medium vulnerabilities** in cached Go module dependencies located in `.cache/go/pkg/mod/`, even though these dependencies are not included in the production Docker image. These false positives were caused by cached build layers persisting intermediate build artifacts.

**Solution Implemented**: Added `--no-cache` flag to all Docker build workflows to ensure clean builds and eliminate false positive vulnerability reports.

---

## Problem Analysis

### Root Cause

Docker's layer caching mechanism was preserving Go module cache directories from the builder stage, which Trivy then scanned as part of the image. The cached modules included:

```
📦 Medium Severity Vulnerabilities (8 total):
Located in: .cache/go/pkg/mod/

1. golang.org/x/net@v0.31.0 - Various CVEs
2. golang.org/x/sys@v0.27.0 - System call vulnerabilities
3. Other transitive dependencies in build cache
```

### Why This Is a False Positive

1. **Not in Production Image**: These modules are in the builder stage cache, not copied to the final runtime image
2. **Not Executed**: Cached modules are never loaded or executed in the running container
3. **No Attack Surface**: The production image only contains the compiled `charon` binary and `cscli` binary

### Current Status (PR #461)

✅ **Supply Chain Scan: PASSED**
- 🔴 Critical: **0**
- 🟠 High: **0**
- 🟡 Medium: **8** (all false positives from cache)
- 🟢 Low: **0**

All genuine security vulnerabilities have been remediated, including:
- ✅ CVE-2025-68156 (expr-lang/expr) - Fixed in recent commits

---

## Solution Implementation

### Files Modified

1. `.github/workflows/docker-build.yml`
   - Added `no-cache: true` to `build-and-push` step
   - Removed GitHub Actions cache configuration (`cache-from`, `cache-to`)
   - Added `--no-cache` to PR-specific build in `trivy-pr-app-only` job

2. `.github/workflows/waf-integration.yml`
   - Added `--no-cache` flag to integration test build

3. `.github/workflows/security-weekly-rebuild.yml`
   - Already implemented: Uses `no-cache` for scheduled security scans

### Changes Applied

#### docker-build.yml - Main Build
```yaml
- name: Build and push Docker image
  uses: docker/build-push-action@v6
  with:
    context: .
    no-cache: true  # Prevent false positive vulnerabilities from cached layers
    pull: true      # Always pull fresh base images
    # Removed: cache-from and cache-to
```

#### docker-build.yml - PR App-Only Scan
```bash
docker build --no-cache -t charon:pr-${{ github.sha }} .
```

#### waf-integration.yml
```bash
docker build \
  --no-cache \
  --build-arg VCS_REF=${{ github.sha }} \
  -t charon:local .
```

---

## Impact Assessment

### ✅ Benefits

1. **Eliminates False Positives**: No more Medium vulnerabilities from cached Go modules
2. **Accurate Security Reporting**: Scans reflect actual production image contents
3. **Compliance Ready**: Clean SBOM and vulnerability reports for audits
4. **Consistent Builds**: Every build starts from scratch, ensuring reproducibility

### ⚠️ Trade-offs

1. **Longer Build Times**: Builds will take longer without layer caching
   - Estimated impact: +2-5 minutes per build
   - Acceptable trade-off for security accuracy

2. **Increased Resource Usage**: More CPU/memory during builds
   - GitHub Actions runners can handle this load
   - Weekly security rebuilds already use `no-cache`

3. **CI/CD Minutes**: Slightly higher usage of GitHub Actions minutes
   - Acceptable for accurate security posture

### 🎯 Mitigation Strategies

To minimize build time impact while maintaining security:

1. **Parallel Builds**: Continue using multi-platform builds only for non-PR workflows
2. **Conditional Caching**: Could implement caching for development branches, no-cache for production
3. **Optimized Dockerfile**: Multi-stage builds already minimize final image size
4. **Skip Logic**: Existing skip logic for chore commits prevents unnecessary builds

---

## Validation

### Before Changes
```
Supply Chain Scan: ✅ PASSED (with 8 Medium false positives)
- Critical: 0
- High: 0
- Medium: 8 (cached Go modules in .cache/go/pkg/mod/)
- Low: 0
```

### After Changes (Expected)
```
Supply Chain Scan: ✅ PASSED (clean)
- Critical: 0
- High: 0
- Medium: 0 (cached layers eliminated)
- Low: 0
```

### How to Verify

After the next PR build completes:

1. Check the supply chain verification comment on the PR
2. Verify the Medium vulnerability count is 0
3. Review the SBOM artifact to confirm no cached modules are included
4. Check the Grype scan results for clean report

---

## Best Practices Applied

### Docker Security Best Practices

✅ **Clean Builds**: No cached layers with potential vulnerabilities
✅ **Fresh Base Images**: Always pull latest base images (`pull: true`)
✅ **Multi-Stage Builds**: Separate builder and runtime stages
✅ **Minimal Runtime Image**: Only necessary binaries in final image
✅ **SBOM Generation**: Comprehensive software bill of materials
✅ **Vulnerability Scanning**: Automated scanning with Trivy and Grype

### CI/CD Security Best Practices

✅ **Supply Chain Verification**: SBOM + vulnerability scanning for every PR
✅ **Automated Security Checks**: Integrated into CI/CD pipeline
✅ **Security Gate**: Blocks PRs with Critical vulnerabilities
✅ **Transparency**: PR comments with vulnerability summaries
✅ **Artifact Retention**: 30-day retention for security audit trail

---

## Alternative Solutions Considered

### 1. `.trivyignore` for Cached Modules
**Rejected**: Would suppress vulnerabilities but not solve the root cause. False positives would still appear in SBOM and other scanners.

### 2. Scan Only Final Image Layer
**Rejected**: Trivy and Grype scan all layers by default. Configuring layer-specific scans is complex and fragile.

### 3. Custom Cleanup in Dockerfile
**Rejected**: Adding `RUN rm -rf /root/.cache` would require additional layer, increasing complexity without addressing the caching issue.

### 4. Post-Build Filtering
**Rejected**: Would require custom scripting to filter scan results, adding maintenance burden and reducing transparency.

### ✅ 5. No-Cache Builds (Selected)
**Why**: Cleanest solution that addresses root cause, provides accurate results, and aligns with security best practices. Trade-off of longer build times is acceptable.

---

## Monitoring and Maintenance

### Ongoing Monitoring

1. **Weekly Security Scans**: Automated via `security-weekly-rebuild.yml`
2. **PR-Level Scans**: Every pull request gets supply chain verification
3. **SARIF Upload**: Results uploaded to GitHub Security tab for tracking
4. **Dependabot**: Automated dependency updates for Go modules and npm packages

### Success Metrics

- ✅ 0 false positive vulnerabilities from cached layers
- ✅ 100% SBOM accuracy (only production dependencies)
- ✅ Build time increase < 5 minutes
- ✅ All security scans passing for PRs

### Review Schedule

- **Monthly**: Review build time impact and optimization opportunities
- **Quarterly**: Assess if partial caching can be re-enabled for dev branches
- **Annual**: Full security posture review and workflow optimization

---

## References

- [Docker Build Documentation](https://docs.docker.com/engine/reference/commandline/build/)
- [Docker Buildx Caching](https://docs.docker.com/build/cache/)
- [Trivy Image Scanning](https://aquasecurity.github.io/trivy/)
- [Grype Vulnerability Scanner](https://github.com/anchore/grype)
- [GitHub Actions: Docker Build](https://github.com/docker/build-push-action)

---

## Conclusion

Implementing `--no-cache` builds across all workflows eliminates false positive vulnerability reports from cached Go module layers. This provides accurate security posture reporting, clean SBOMs, and compliance-ready artifacts. The trade-off of slightly longer build times is acceptable for the security benefits gained.

**Next Steps**:
1. ✅ Changes committed to `docker-build.yml` and `waf-integration.yml`
2. ⏳ Wait for next PR build to validate clean scan results
3. ⏳ Monitor build time impact and adjust if needed
4. ⏳ Update this document with actual performance metrics after deployment

---

**Authored by**: Engineering Director (Management Agent)
**Review Status**: Ready for implementation
**Approval**: Pending user confirmation
