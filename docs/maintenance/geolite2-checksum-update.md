# GeoLite2 Database Checksum Update Guide

## Overview

Charon uses the [MaxMind GeoLite2-Country database](https://dev.maxmind.com/geoip/geolite2-free-geolocation-data) for geographic IP information. When the upstream database is updated, the checksum in the Dockerfile must be updated to match the new file.

**Automated Updates:** Charon includes a GitHub Actions workflow that checks for upstream changes weekly and creates pull requests automatically.

**Manual Updates:** Follow this guide if the automated workflow fails or you need to update immediately.

---

## When to Update

Update the checksum when:

1. **Docker build fails** with the following error:
   ```
   sha256sum: /app/data/geoip/GeoLite2-Country.mmdb: FAILED
   sha256sum: WARNING: 1 computed checksum did NOT match
   ```

2. **Automated workflow creates a PR** (happens weekly on Mondays at 2 AM UTC)

3. **GitHub Actions build fails** with checksum mismatch

---

## Automated Workflow (Recommended)

Charon includes a GitHub Actions workflow that automatically:
- Checks for upstream GeoLite2 database changes weekly
- Calculates the new checksum
- Creates a pull request with the update
- Validates Dockerfile syntax

**Workflow File:** [`.github/workflows/update-geolite2.yml`](../../.github/workflows/update-geolite2.yml)

**Schedule:** Mondays at 2 AM UTC (weekly)

**Manual Trigger:**
```bash
gh workflow run update-geolite2.yml
```

### Reviewing Automated PRs

When the workflow creates a PR:

1. **Review the checksum change** in the PR description
2. **Verify the checksums** match the upstream file (see verification below)
3. **Wait for CI checks** to pass (build, tests, security scans)
4. **Merge the PR** if all checks pass

---

## Manual Update (5 Minutes)

### Step 1: Download and Calculate Checksum

```bash
# Download the database file
curl -fsSL "https://github.com/P3TERX/GeoLite.mmdb/raw/download/GeoLite2-Country.mmdb" \
  -o /tmp/geolite2-test.mmdb

# Calculate SHA256 checksum
sha256sum /tmp/geolite2-test.mmdb

# Example output:
# 436135ee98a521da715a6d483951f3dbbd62557637f2d50d1987fc048874bd5d  /tmp/geolite2-test.mmdb
```

### Step 2: Update Dockerfile

**File:** [`Dockerfile`](../../Dockerfile) (line ~352)

**Find this line:**
```dockerfile
ARG GEOLITE2_COUNTRY_SHA256=<old-checksum>
```

**Replace with the new checksum:**
```dockerfile
ARG GEOLITE2_COUNTRY_SHA256=436135ee98a521da715a6d483951f3dbbd62557637f2d50d1987fc048874bd5d
```

**Using sed (automated):**
```bash
NEW_CHECKSUM=$(curl -fsSL "https://github.com/P3TERX/GeoLite.mmdb/raw/download/GeoLite2-Country.mmdb" | sha256sum | cut -d' ' -f1)

sed -i "s/ARG GEOLITE2_COUNTRY_SHA256=.*/ARG GEOLITE2_COUNTRY_SHA256=${NEW_CHECKSUM}/" Dockerfile
```

### Step 3: Verify the Change

```bash
# Verify the checksum was updated
grep "GEOLITE2_COUNTRY_SHA256" Dockerfile

# Expected output:
# ARG GEOLITE2_COUNTRY_SHA256=436135ee98a521da715a6d483951f3dbbd62557637f2d50d1987fc048874bd5d
```

### Step 4: Test Build

```bash
# Clean build environment (recommended)
docker builder prune -af

# Build without cache to ensure checksum is verified
docker build --no-cache --pull \
  --platform linux/amd64 \
  --build-arg VERSION=test-checksum \
  -t charon:test-checksum \
  .

# Verify build succeeded and container starts
docker run --rm charon:test-checksum /app/charon --version
```

**Expected output:**
```
✅ GeoLite2-Country.mmdb: OK
✅ Successfully tagged charon:test-checksum
```

### Step 5: Commit and Push

```bash
git add Dockerfile
git commit -m "fix(docker): update GeoLite2-Country.mmdb checksum

The upstream GeoLite2 database file was updated, requiring a checksum update.

Old: <old-checksum>
New: <new-checksum>

Fixes: #<issue-number>
Resolves: Docker build checksum mismatch"

git push origin <branch-name>
```

---

## Verification Script

Use this script to check if the Dockerfile checksum matches the upstream file:

```bash
#!/bin/bash
# verify-geolite2-checksum.sh

set -euo pipefail

DOCKERFILE_CHECKSUM=$(grep "ARG GEOLITE2_COUNTRY_SHA256=" Dockerfile | cut -d'=' -f2)
UPSTREAM_CHECKSUM=$(curl -fsSL "https://github.com/P3TERX/GeoLite.mmdb/raw/download/GeoLite2-Country.mmdb" | sha256sum | cut -d' ' -f1)

echo "Dockerfile: $DOCKERFILE_CHECKSUM"
echo "Upstream:   $UPSTREAM_CHECKSUM"

if [ "$DOCKERFILE_CHECKSUM" = "$UPSTREAM_CHECKSUM" ]; then
  echo "✅ Checksum matches"
  exit 0
else
  echo "❌ Checksum mismatch - update required"
  echo ""
  echo "Run: sed -i 's/ARG GEOLITE2_COUNTRY_SHA256=.*/ARG GEOLITE2_COUNTRY_SHA256=$UPSTREAM_CHECKSUM/' Dockerfile"
  exit 1
fi
```

**Make executable:**
```bash
chmod +x scripts/verify-geolite2-checksum.sh
```

**Run verification:**
```bash
./scripts/verify-geolite2-checksum.sh
```

---

## Troubleshooting

### Issue: Build Still Fails After Update

**Symptoms:**
- Checksum verification fails
- "FAILED" error persists

**Solutions:**

1. **Clear Docker build cache:**
   ```bash
   docker builder prune -af
   ```

2. **Verify the checksum was committed:**
   ```bash
   git show HEAD:Dockerfile | grep "GEOLITE2_COUNTRY_SHA256"
   ```

3. **Re-download and verify upstream file:**
   ```bash
   curl -fsSL "https://github.com/P3TERX/GeoLite.mmdb/raw/download/GeoLite2-Country.mmdb" -o /tmp/test.mmdb
   sha256sum /tmp/test.mmdb
   diff <(echo "$EXPECTED_CHECKSUM") <(sha256sum /tmp/test.mmdb | cut -d' ' -f1)
   ```

### Issue: Upstream File Unavailable (404)

**Symptoms:**
- `curl` returns 404 Not Found
- Automated workflow fails with `download_failed` error

**Investigation Steps:**

1. **Check upstream repository:**
   - Visit: https://github.com/P3TERX/GeoLite.mmdb
   - Verify the file still exists at the raw URL
   - Check for repository status or announcements

2. **Check MaxMind status:**
   - Visit: https://status.maxmind.com/
   - Check for service outages or maintenance

**Temporary Solutions:**

1. **Use cached Docker layer** (if available):
   ```bash
   docker build --cache-from ghcr.io/wikid82/charon:latest -t charon:latest .
   ```

2. **Use local copy** (temporary):
   ```bash
   # Download from a working container
   docker run --rm ghcr.io/wikid82/charon:latest cat /app/data/geoip/GeoLite2-Country.mmdb > /tmp/GeoLite2-Country.mmdb

   # Calculate checksum
   sha256sum /tmp/GeoLite2-Country.mmdb
   ```

3. **Alternative source** (if P3TERX mirror is down):
   - Official MaxMind downloads require a license key
   - Consider [MaxMind GeoLite2](https://dev.maxmind.com/geoip/geolite2-free-geolocation-data) signup (free)

### Issue: Checksum Mismatch on Re-download

**Symptoms:**
- Checksum calculated locally differs from what's in the Dockerfile
- Checksum changes between downloads

**Investigation Steps:**

1. **Verify file integrity:**
   ```bash
   # Download multiple times and compare
   for i in {1..3}; do
     curl -fsSL "https://github.com/P3TERX/GeoLite.mmdb/raw/download/GeoLite2-Country.mmdb" | sha256sum
   done
   ```

2. **Check for CDN caching issues:**
   - Wait 5 minutes and retry
   - Try from different network locations

3. **Verify no MITM proxy:**
   ```bash
   # Download via HTTPS and verify certificate
   curl -v -fsSL "https://github.com/P3TERX/GeoLite.mmdb/raw/download/GeoLite2-Country.mmdb" -o /tmp/test.mmdb 2>&1 | grep "CN="
   ```

**If confirmed as supply chain attack:**
- **STOP** and do not proceed
- Report to security team
- See [Security Incident Response](../security-incident-response.md)

### Issue: Multi-Platform Build Fails (arm64)

**Symptoms:**
- `linux/amd64` build succeeds
- `linux/arm64` build fails with checksum error

**Investigation:**

1. **Verify upstream file is architecture-agnostic:**
   - GeoLite2 `.mmdb` files are data files, not binaries
   - Should be identical across all platforms

2. **Check buildx platform emulation:**
   ```bash
   docker buildx ls
   docker buildx inspect
   ```

3. **Test arm64 build explicitly:**
   ```bash
   docker buildx build --platform linux/arm64 --load -t test-arm64 .
   ```

---

## Additional Resources

- **Automated Workflow:** [`.github/workflows/update-geolite2.yml`](../../.github/workflows/update-geolite2.yml)
- **Implementation Plan:** [`docs/plans/current_spec.md`](../plans/current_spec.md)
- **QA Report:** [`docs/reports/qa_report.md`](../reports/qa_report.md)
- **Dockerfile:** [`Dockerfile`](../../Dockerfile) (line ~352)
- **MaxMind GeoLite2:** https://dev.maxmind.com/geoip/geolite2-free-geolocation-data
- **P3TERX Mirror:** https://github.com/P3TERX/GeoLite.mmdb

---

## Historical Context

**Issue:** Docker build failures due to checksum mismatch (February 2, 2026)

**Root Cause:** The upstream GeoLite2 database file was updated by MaxMind, but the Dockerfile still referenced the old SHA256 checksum. When Docker's multi-stage build tried to verify the checksum, it failed and aborted the build, causing cascade failures in subsequent `COPY` commands that referenced earlier build stages.

**Solution:** Updated one line in `Dockerfile` (line 352) with the correct checksum and implemented an automated workflow to prevent future occurrences.

**Build Failure URL:** https://github.com/Wikid82/Charon/actions/runs/21584236523/job/62188372617

**Related PRs:**
- Fix implementation: (link to PR)
- Automated workflow addition: (link to PR)

---

**Last Updated:** February 2, 2026
**Maintainer:** Charon Development Team
**Status:** Active
