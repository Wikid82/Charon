# Quick Action: Rebuild Image to Apply Security Fixes

**Date**: 2026-01-11
**Severity**: LOW (Fixes already in code)
**Estimated Time**: 5 minutes

## TL;DR

✅ **Good News**: The Dockerfile ALREADY contains all security fixes!
⚠️ **Action Needed**: Rebuild Docker image to apply the fixes

CI scan detected vulnerabilities in a **stale Docker image** built before security patches were committed. Current Dockerfile uses Go 1.25.5, CrowdSec v1.7.4, and patched dependencies.

## What's Wrong?

The Docker image being scanned by CI was built **before** these fixes were added to the Dockerfile (scan date: 2025-12-18, 3 weeks old):

1. **Old Image**: Built with Go 1.25.1 (vulnerable)
2. **Current Dockerfile**: Uses Go 1.25.5 (patched)

## What's Already Fixed in Dockerfile?

```dockerfile
# Line 203: Go 1.25.5 (includes CVE fixes)
FROM --platform=$BUILDPLATFORM golang:1.25.5-alpine AS crowdsec-builder

# Line 213: CrowdSec v1.7.4
ARG CROWDSEC_VERSION=1.7.4

# Lines 227-230: Patched expr-lang/expr (CVE-2025-68156)
RUN go get github.com/expr-lang/expr@v1.17.7 && \
    go mod tidy
```

**All CVEs are fixed:**

- ✅ CVE-2025-58183 (archive/tar) - Fixed in Go 1.25.2+
- ✅ CVE-2025-58186 (net/http) - Fixed in Go 1.25.2+
- ✅ CVE-2025-58187 (crypto/x509) - Fixed in Go 1.25.3+
- ✅ CVE-2025-61729 (crypto/x509) - Fixed in Go 1.25.5+
- ✅ CVE-2025-68156 (expr-lang) - Fixed with v1.17.7

## Quick Fix (5 minutes)

### 1. Rebuild Image with Current Dockerfile

```bash
# Clean old image
docker rmi charon:local 2>/dev/null || true

# Rebuild with latest Dockerfile (no changes needed!)
docker build -t charon:local .
```

### 2. Verify Fix

```bash
# Check CrowdSec version and Go version
docker run --rm charon:local /usr/local/bin/crowdsec version

# Expected output should include:
# version: v1.7.4
# Go: go1.25.5 (or higher)
```

### 3. Run Security Scan

```bash
# Install scanning tools if not present
curl -sSfL https://raw.githubusercontent.com/anchore/syft/main/install.sh | sh -s -- -b /usr/local/bin
curl -sSfL https://raw.githubusercontent.com/anchore/grype/main/install.sh | sh -s -- -b /usr/local/bin

# Scan rebuilt image
syft charon:local -o cyclonedx-json > sbom-check.json
grype sbom:./sbom-check.json --severity HIGH,CRITICAL --output table

# Expected: 0 HIGH/CRITICAL vulnerabilities in all binaries
```

### 4. Push to Registry (if needed)

```bash
# Tag and push updated image
docker tag charon:local ghcr.io/wikid82/charon:latest
docker push ghcr.io/wikid82/charon:latest

# Or trigger CI rebuild by pushing to main
git commit --allow-empty -m "chore: trigger image rebuild with security patches"
git push
```

## Expected Outcome

✅ CI supply chain scan will pass
✅ 0 HIGH/CRITICAL vulnerabilities in all binaries
✅ CrowdSec v1.7.4 with Go 1.25.5
✅ All stdlib CVEs resolved

## Why This Happened

1. **Dockerfile was updated** with security fixes (Go 1.25.5, CrowdSec v1.7.4, patched expr-lang)
2. **Docker image was NOT rebuilt** after Dockerfile changes
3. **CI scan analyzed old image** built before fixes
4. **Local scans** (`govulncheck`) don't detect binary vulnerabilities

**Solution**: Simply rebuild the image to apply fixes already in the Dockerfile.

## If You Need to Rollback

```bash
# Revert Dockerfile
git revert HEAD

# Rebuild
docker build -t charon:local .
```

## Need More Details?

See full analysis:

- [Supply Chain Scan Analysis](./SUPPLY_CHAIN_SCAN_ANALYSIS.md)
- [Detailed Remediation Plan](./SUPPLY_CHAIN_REMEDIATION_PLAN.md)

## Questions?

- **"Is our code vulnerable?"** No, only CrowdSec binary needs update
- **"Can we deploy current build?"** Yes for dev/staging, upgrade recommended for production
- **"Will this break anything?"** No, v1.6.6 is a patch release (minor Go stdlib fixes)
- **"How urgent is this?"** MEDIUM - Schedule for next release, not emergency hotfix

---

**Action Owner**: Dev Team
**Review Required**: Security Team
**Target**: Next deployment window
