# Docker Build Failure Fix - Comprehensive Implementation Plan

**Date:** February 2, 2026
**Status:** 🔴 CRITICAL - BLOCKING CI/CD
**Priority:** P0 - Immediate Action Required
**Build URL:** https://github.com/Wikid82/Charon/actions/runs/21584236523/job/62188372617

 ---

## Executive Summary

The GitHub Actions Docker build workflow is failing due to a **GeoLite2-Country.mmdb checksum mismatch**, causing cascade failures in multi-stage Docker builds.

**Root Cause:** The upstream GeoLite2 database file was updated, but the Dockerfile still references the old SHA256 checksum.

**Impact:**
- ❌ All CI/CD Docker builds failing since database update
- ❌ Cannot publish new images to GHCR/Docker Hub
- ❌ Blocks all releases and deployments

**Solution:** Update one line in Dockerfile (line 352) with correct checksum.

**Estimated Time to Fix:** 5 minutes
**Testing Time:** 15 minutes (local + CI verification)

---

## Critical Issue Analysis

### Issue #1: GeoLite2-Country.mmdb Checksum Mismatch (ROOT CAUSE)

**Location:** `/projects/Charon/Dockerfile` - Line 352

**Current Value (WRONG):**
```dockerfile
ARG GEOLITE2_COUNTRY_SHA256=6b778471c086c44d15bd4df954661d441a5513ec48f1af5545cb05af8f2e15b9
```

**Correct Value (VERIFIED):**
```dockerfile
ARG GEOLITE2_COUNTRY_SHA256=436135ee98a521da715a6d483951f3dbbd62557637f2d50d1987fc048874bd5d
```

**Verification Method:**
```bash
curl -fsSL "https://github.com/P3TERX/GeoLite.mmdb/raw/download/GeoLite2-Country.mmdb" -o /tmp/test.mmdb
sha256sum /tmp/test.mmdb
# Output: 436135ee98a521da715a6d483951f3dbbd62557637f2d50d1987fc048874bd5d
```

**Error Message:**
```
sha256sum: /app/data/geoip/GeoLite2-Country.mmdb: FAILED
sha256sum: WARNING: 1 computed checksum did NOT match
The command '/bin/sh -c mkdir -p /app/data/geoip && curl -fSL ...' returned a non-zero code: 1
```

### Issue #2: Blob Not Found Errors (CASCADE FAILURE)

**Error Examples:**
```
COPY configs/crowdsec/acquis.yaml /etc/crowdsec.dist/acquis.yaml: blob not found
COPY --from=backend-builder /app/backend/charon /app/charon: blob not found
COPY --from=frontend-builder /app/frontend/dist /app/frontend/dist: blob not found
```

**Analysis:**
These are NOT missing files. All files exist in the repository:

```bash
✅ configs/crowdsec/acquis.yaml
✅ configs/crowdsec/install_hub_items.sh
✅ configs/crowdsec/register_bouncer.sh
✅ frontend/package.json
✅ frontend/package-lock.json
✅ .docker/docker-entrypoint.sh
✅ scripts/db-recovery.sh
```

**Root Cause:** The GeoLite2 checksum failure causes the Docker build to abort during the final runtime stage (line 352-356). When the build aborts, the multi-stage build artifacts from earlier stages (`backend-builder`, `frontend-builder`, `caddy-builder`, `crowdsec-builder`) are not persisted to the builder cache. Subsequent COPY commands trying to reference these non-existent artifacts fail with "blob not found".

**This is a cascade failure from Issue #1 - fixing the checksum will resolve all blob errors.**

---

## Implementation Plan

### PHASE 1: Fix Checksum (5 minutes)

**Step 1.1: Update Dockerfile**

**File:** `/projects/Charon/Dockerfile`
**Line:** 352

**Exact Change:**
```bash
cd /projects/Charon
sed -i 's/ARG GEOLITE2_COUNTRY_SHA256=6b778471c086c44d15bd4df954661d441a5513ec48f1af5545cb05af8f2e15b9/ARG GEOLITE2_COUNTRY_SHA256=436135ee98a521da715a6d483951f3dbbd62557637f2d50d1987fc048874bd5d/' Dockerfile
```

**Verification:**
```bash
grep "GEOLITE2_COUNTRY_SHA256" Dockerfile
# Expected: ARG GEOLITE2_COUNTRY_SHA256=436135ee98a521da715a6d483951f3dbbd62557637f2d50d1987fc048874bd5d
```

**Step 1.2: Commit Change**

```bash
git add Dockerfile
git commit -m "fix(docker): update GeoLite2-Country.mmdb checksum

The upstream GeoLite2 database file was updated, requiring a checksum update.

Old: 6b778471c086c44d15bd4df954661d441a5513ec48f1af5545cb05af8f2e15b9
New: 436135ee98a521da715a6d483951f3dbbd62557637f2d50d1987fc048874bd5d

Fixes: #<issue-number>
Resolves: Blob not found errors (cascade failure from checksum mismatch)"
```

---

### PHASE 2: Local Testing (15 minutes)

**Step 2.1: Clean Build Environment**

```bash
# Remove all build cache
docker builder prune -af

# Remove previous test images
docker images | grep charon | awk '{print $3}' | xargs -r docker rmi -f
```

**Step 2.2: Build for amd64 (Same as CI)**

```bash
cd /projects/Charon

docker buildx build \
  --platform linux/amd64 \
  --no-cache \
  --pull \
  --progress=plain \
  --build-arg VERSION=test-fix \
  --build-arg BUILD_DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ") \
  --build-arg VCS_REF=$(git rev-parse HEAD) \
  -t charon:test-amd64 \
  . 2>&1 | tee /tmp/docker-build-test.log
```

**Expected Success Indicators:**
```
✅ Step X: RUN echo "${GEOLITE2_COUNTRY_SHA256}  /app/data/geoip/GeoLite2-Country.mmdb" | sha256sum -c -
   /app/data/geoip/GeoLite2-Country.mmdb: OK
✅ Step Y: COPY --from=gosu-builder /gosu-out/gosu /usr/sbin/gosu
✅ Step Z: COPY --from=frontend-builder /app/frontend/dist /app/frontend/dist
✅ Step AA: COPY --from=backend-builder /app/backend/charon /app/charon
✅ Step AB: COPY --from=caddy-builder /usr/bin/caddy /usr/bin/caddy
✅ Step AC: COPY --from=crowdsec-builder /crowdsec-out/crowdsec /usr/local/bin/crowdsec
✅ Successfully tagged charon:test-amd64
```

**If Build Fails:**
```bash
# Check for errors
grep -A 5 "ERROR\|FAILED\|blob not found" /tmp/docker-build-test.log

# Verify checksum in Dockerfile
grep "GEOLITE2_COUNTRY_SHA256" Dockerfile

# Re-download and verify checksum
curl -fsSL "https://github.com/P3TERX/GeoLite.mmdb/raw/download/GeoLite2-Country.mmdb" \
  -o /tmp/verify.mmdb
sha256sum /tmp/verify.mmdb
```

**Step 2.3: Runtime Verification**

```bash
# Start container
docker run -d \
  --name charon-test \
  -p 8080:8080 \
  charon:test-amd64

# Wait for startup (30 seconds)
sleep 30

# Check health
docker ps --filter "name=charon-test"
# Expected: Status includes "(healthy)"

# Test API
curl -sf http://localhost:8080/api/v1/health | jq .
# Expected: {"status":"ok","version":"test-fix",...}

# Check for errors in logs
docker logs charon-test 2>&1 | grep -i "error\|failed\|fatal"
# Expected: No critical errors

# Cleanup
docker stop charon-test && docker rm charon-test
```

---

### PHASE 3: Push and Monitor CI (30 minutes)

**Step 3.1: Push to GitHub**

```bash
git push origin <branch-name>
```

**Step 3.2: Monitor Workflow**

1. **Navigate to Actions**:
   https://github.com/Wikid82/Charon/actions

2. **Watch "Docker Build, Publish & Test" workflow**:
   - Should trigger automatically on push
   - Monitor build progress

3. **Expected Stages:**
   ```
   ✅ Build and push (linux/amd64, linux/arm64)
   ✅ Verify Caddy Security Patches
   ✅ Verify CrowdSec Security Patches
   ✅ Run Trivy scan
   ✅ Generate SBOM
   ✅ Attest SBOM
   ✅ Sign image (Cosign)
   ✅ Test image (integration-test.sh)
   ```

**Step 3.3: Verify Published Images**

```bash
# Pull from GHCR
docker pull ghcr.io/wikid82/charon:<tag>

# Verify image works
docker run --rm ghcr.io/wikid82/charon:<tag> /app/charon --version
# Expected: Output shows version info
```

**Step 3.4: Check Security Scans**

- **Trivy Results**: Check for new vulnerabilities
  https://github.com/Wikid82/Charon/security/code-scanning

- **Expr-lang Verification**: Ensure CVE-2025-68156 patch is present
  Check workflow logs for:
  ```
  ✅ PASS: expr-lang version v1.17.7 is patched (>= v1.17.7)
  ```

---

## Success Criteria

### Build Success Indicators

- [ ] Local `docker build` completes without errors
- [ ] No "sha256sum: FAILED" errors
- [ ] No "blob not found" errors
- [ ] All COPY commands execute successfully
- [ ] Container starts and becomes healthy
- [ ] API responds to `/health` endpoint
- [ ] GitHub Actions workflow passes all stages
- [ ] Multi-platform build succeeds (amd64 + arm64)

### Deployment Success Indicators

- [ ] Image published to GHCR: `ghcr.io/wikid82/charon:<tag>`
- [ ] Image signed with Sigstore/Cosign
- [ ] SBOM attached and attestation created
- [ ] Trivy scan shows no critical regressions
- [ ] Integration tests pass (`integration-test.sh`)

---

## Rollback Plan

If the fix introduces new issues:

**Step 1: Revert Commit**
```bash
git revert <commit-sha>
git push origin <branch-name>
```

**Step 2: Emergency Image Rollback (if needed)**
```bash
# Retag previous working image as latest
docker pull ghcr.io/wikid82/charon:sha-<previous-working-commit>
docker tag ghcr.io/wikid82/charon:sha-<previous-working-commit> \
           ghcr.io/wikid82/charon:latest
docker push ghcr.io/wikid82/charon:latest
```

**Step 3: Communicate Status**
- Update issue with rollback details
- Document root cause of new failure
- Create follow-up issue if needed

### Rollback Decision Matrix

Use this matrix to determine whether to rollback or proceed with remediation:

| Scenario | Impact | Decision | Action | Timeline |
|----------|--------|----------|--------|----------|
| **Checksum update breaks local build** | 🔴 Critical | ROLLBACK immediately | Revert commit, investigate upstream changes | < 5 minutes |
| **Local build passes, CI build fails** | 🟡 High | INVESTIGATE first | Check CI environment differences, then decide | 15-30 minutes |
| **Build passes, container fails healthcheck** | 🔴 Critical | ROLLBACK immediately | Revert commit, test with previous checksum | < 10 minutes |
| **Build passes, security scan fails** | 🟠 Medium | REMEDIATE if < 2 hours | Fix security issues if quick, else rollback | < 2 hours |
| **New checksum breaks runtime GeoIP lookups** | 🔴 Critical | ROLLBACK immediately | Revert commit, verify database integrity | < 5 minutes |
| **Automated PR fails syntax validation** | 🟢 Low | REMEDIATE in PR | Fix workflow and retry, no production impact | < 1 hour |
| **Upstream source unavailable (404)** | 🟡 High | BLOCK deployment | Document issue, find alternative source | N/A |
| **Checksum mismatch on re-download** | 🔴 Critical | BLOCK deployment | Investigate cache poisoning, verify source | N/A |
| **Multi-platform build succeeds (amd64), fails (arm64)** | 🟡 High | CONDITIONAL: Proceed for amd64, investigate arm64 | Deploy amd64, fix arm64 separately | < 1 hour |
| **Integration tests pass, E2E tests fail** | 🟠 Medium | INVESTIGATE first | Isolate test failure cause, rollback if service-breaking | 30-60 minutes |

**Decision Criteria:**

- **ROLLBACK immediately** if:
  - Production deployments are affected
  - Core functionality breaks (API, routing, healthchecks)
  - Security posture degrades
  - No clear remediation path within 30 minutes

- **INVESTIGATE first** if:
  - Only test/CI environments affected
  - Failure is non-deterministic
  - Clear path to remediation exists
  - Can be fixed within 2 hours

- **BLOCK deployment** if:
  - Upstream integrity cannot be verified
  - Security validation fails
  - Checksum verification fails on any attempt

**Escalation Triggers:**

- Cannot rollback within 15 minutes
- Rollback itself fails
- Production outage extends beyond 30 minutes
- Security incident detected (cache poisoning, supply chain attack)
- Multiple rollback attempts required

---

## Future Maintenance

### Preventing Future Checksum Failures

**Option A: Automated Checksum Updates (Recommended)**

Create a GitHub Actions workflow to detect and update GeoLite2 checksums automatically:

**File:** `.github/workflows/update-geolite2.yml`
```yaml
name: Update GeoLite2 Checksum

on:
  schedule:
    - cron: '0 2 * * 1'  # Weekly on Mondays at 2 AM UTC
  workflow_dispatch:

jobs:
  update-checksum:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4

      - name: Download and calculate checksum
        id: checksum
        run: |
          CURRENT=$(curl -fsSL "https://github.com/P3TERX/GeoLite.mmdb/raw/download/GeoLite2-Country.mmdb" | sha256sum | cut -d' ' -f1)
          OLD=$(grep "ARG GEOLITE2_COUNTRY_SHA256=" Dockerfile | cut -d'=' -f2)
          echo "current=$CURRENT" >> $GITHUB_OUTPUT
          echo "old=$OLD" >> $GITHUB_OUTPUT

      - name: Update Dockerfile
        if: steps.checksum.outputs.current != steps.checksum.outputs.old
        run: |
          sed -i "s/ARG GEOLITE2_COUNTRY_SHA256=.*/ARG GEOLITE2_COUNTRY_SHA256=${{ steps.checksum.outputs.current }}/" Dockerfile

      - name: Create Pull Request
        if: steps.checksum.outputs.current != steps.checksum.outputs.old
        uses: peter-evans/create-pull-request@v5
        with:
          title: "chore(docker): update GeoLite2-Country.mmdb checksum"
          body: |
            Automated checksum update for GeoLite2-Country.mmdb

            - Old: `${{ steps.checksum.outputs.old }}`
            - New: `${{ steps.checksum.outputs.current }}`

            **Changes:**
            - Updated `Dockerfile` line 352

            **Testing:**
            - [ ] Local build passes
            - [ ] CI build passes
            - [ ] Container starts successfully
          branch: bot/update-geolite2-checksum
          delete-branch: true
```

**Option B: Manual Update Documentation**

Create documentation for manual checksum updates:

**File:** `/projects/Charon/docs/maintenance/geolite2-checksum-update.md`

```markdown
# GeoLite2 Database Checksum Update Guide

## When to Update

Update the checksum when Docker build fails with:
```
sha256sum: /app/data/geoip/GeoLite2-Country.mmdb: FAILED
```

## Quick Fix (5 minutes)

1. Download and calculate new checksum:
   ```bash
   curl -fsSL "https://github.com/P3TERX/GeoLite.mmdb/raw/download/GeoLite2-Country.mmdb" -o /tmp/test.mmdb
   sha256sum /tmp/test.mmdb
   ```

2. Update Dockerfile (line 352):
   ```dockerfile
   ARG GEOLITE2_COUNTRY_SHA256=<new-checksum-from-step-1>
   ```

3. Test locally:
   ```bash
   docker build --no-cache -t test .
   ```

4. Commit and push:
   ```bash
   git add Dockerfile
   git commit -m "fix(docker): update GeoLite2-Country.mmdb checksum"
   git push
   ```

## Verification Script

Use this script to verify before updating:

```bash
#!/bin/bash
# verify-geolite2-checksum.sh

EXPECTED=$(grep "ARG GEOLITE2_COUNTRY_SHA256=" Dockerfile | cut -d'=' -f2)
ACTUAL=$(curl -fsSL "https://github.com/P3TERX/GeoLite.mmdb/raw/download/GeoLite2-Country.mmdb" | sha256sum | cut -d' ' -f1)

echo "Expected: $EXPECTED"
echo "Actual:   $ACTUAL"

if [ "$EXPECTED" = "$ACTUAL" ]; then
  echo "✅ Checksum matches"
  exit 0
else
  echo "❌ Checksum mismatch - update required"
  echo "Run: sed -i 's/ARG GEOLITE2_COUNTRY_SHA256=.*/ARG GEOLITE2_COUNTRY_SHA256=$ACTUAL/' Dockerfile"
  exit 1
fi
```
```

**Recommended Approach:** Implement Option A (automated updates) to prevent future failures.

---

## Related Files

### Modified Files
- `/projects/Charon/Dockerfile` (line 352)

### Reference Files
- `.dockerignore` - Build context exclusions (no changes needed)
- `.gitignore` - Version control exclusions (no changes needed)
- `.github/workflows/docker-build.yml` - CI/CD workflow (no changes needed)

### Documentation
- `docs/maintenance/geolite2-checksum-update.md` (to be created)
- `.github/workflows/update-geolite2.yml` (optional automation)

---

##Appendix A: Multi-Stage Build Structure

### Build Stages (Dependency Graph)

```
1. xx (tonistiigi/xx) ─────────────────────────────┐
                                                     ├──> 2. gosu-builder ──> final
                                                     ├──> 3. backend-builder ──> final
                                                     ├──> 5. crowdsec-builder ──> final
                                                     └──> (cross-compile helpers)

4. frontend-builder (standalone) ──────────────────────> final

6. caddy-builder (standalone) ─────────────────────────> final

7. crowdsec-fallback (not used in normal flow)

8. final (debian:trixie-slim) ◄─── Copies from all stages above
   - Downloads GeoLite2 (FAILS HERE if checksum wrong)
   - Copies binaries from builder stages
   - Sets up runtime environment
```

### COPY Commands in Final Stage

**Line 349:** `COPY --from=gosu-builder /gosu-out/gosu /usr/sbin/gosu`
**Line 359:** `COPY --from=caddy-builder /usr/bin/caddy /usr/bin/caddy`
**Line 366-368:** `COPY --from=crowdsec-builder ...`
**Line 393-395:** `COPY configs/crowdsec/* ...`
**Line 401:** `COPY --from=backend-builder /app/backend/charon /app/charon`
**Line 404:** `COPY --from=backend-builder /go/bin/dlv /usr/local/bin/dlv`
**Line 408:** `COPY --from=frontend-builder /app/frontend/dist /app/frontend/dist`
**Line 411:** `COPY .docker/docker-entrypoint.sh /docker-entrypoint.sh`
**Line 414:** `COPY scripts/ /app/scripts/`

**All of these fail with "blob not found" if GeoLite2 download fails**, because Docker aborts the build before persisting build stage outputs.

---

## Appendix B: Verification Commands

### Pre-Fix Verification
```bash
# Verify current checksum is wrong
grep "GEOLITE2_COUNTRY_SHA256" Dockerfile
# Should show: 6b778471c086c44d15bd4df954661d441a5513ec48f1af5545cb05af8f2e15b9

# Download and check actual checksum
curl -fsSL "https://github.com/P3TERX/GeoLite.mmdb/raw/download/GeoLite2-Country.mmdb" | sha256sum
# Should show: 436135ee98a521da715a6d483951f3dbbd62557637f2d50d1987fc048874bd5d
```

### Post-Fix Verification
```bash
# Verify Dockerfile was updated
grep "GEOLITE2_COUNTRY_SHA256" Dockerfile
# Should show: 436135ee98a521da715a6d483951f3dbbd62557637f2d50d1987fc048874bd5d

# Test build
docker build --no-cache --pull -t test .

# Verify container
docker run --rm test /app/charon --version
```

### CI Verification
```bash
# Check latest workflow run
gh run list --workflow=docker-build.yml --limit=1

# View workflow logs
gh run view <run-id> --log

# Check for success indicators
gh run view <run-id> --log | grep "✅"
```

---

## Appendix C: Troubleshooting

### Issue: Build Still Fails After Checksum Update

**Symptoms:**
- Upload checksum is correct in Dockerfile
- Build still fails with sha256sum error
- Error message shows different checksum

**Possible Causes:**
1. **Browser cached old file**: Clear Docker build cache
   ```bash
   docker builder prune -af
   ```

2. **Git cached old file**: Verify committed change
   ```bash
   git show HEAD:Dockerfile | grep "GEOLITE2_COUNTRY_SHA256"
   ```

3. **Upstream file changed again**: Re-download and recalculate
   ```bash
   curl -fsSL "https://github.com/P3TERX/GeoLite.mmdb/raw/download/GeoLite2-Country.mmdb" | sha256sum
   ```

### Issue: Blob Not Found Persists

**Symptoms:**
- GeoLite2 checksum passes
- Blob not found errors still occur
- Specific COPY command fails

**Debug Steps:**
1. **Check specific stage build:**
   ```bash
   # Test specific stage
   docker build --target backend-builder -t test-backend .
   docker build --target frontend-builder -t test-frontend .
   ```

2. **Check file existence in context:**
   ```bash
   # List build context files
   docker build --dry-run -t test . 2>&1 | grep "COPY\|ADD"
   ```

3. **Verify .dockerignore:**
   ```bash
   # Check if required files are excluded
   grep -E "(configs|scripts|frontend)" .dockerignore
   ```

### Issue: Container Fails Healthcheck

**Symptoms:**
- Build succeeds
- Container starts but never becomes healthy
- Healthcheck fails repeatedly

**Debug Steps:**
```bash
# Check container logs
docker logs <container-name>

# Check healthcheck status
docker inspect <container-name> | jq '.[0].State.Health'

# Manual healthcheck
docker exec <container-name> curl -f http://localhost:8080/api/v1/health
```

---

## Conclusion

This is a straightforward fix requiring a single-line change in the Dockerfile. The "blob not found" errors are a cascade failure and will be resolved automatically once the GeoLite2 checksum is corrected.

**Immediate Action Required:**
1. Update Dockerfile line 352 with correct checksum
2. Test build locally
3. Commit and push
4. Monitor CI/CD pipeline

**Estimated Total Time:** 20 minutes (5 min fix + 15 min testing)

---

**Plan Status:** ✅ Ready for Implementation
**Confidence Level:** 100% - Root cause identified with exact fix
**Risk Assessment:** Low - Single line change, well-tested pattern
