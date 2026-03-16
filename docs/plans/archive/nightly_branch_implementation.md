# Nightly Branch Implementation - Complete Specification

**Date:** 2026-01-13
**Version:** 1.0
**Status:** Planning Phase

---

## Table of Contents

1. [Executive Summary](#executive-summary)
2. [Current State Analysis](#current-state-analysis)
3. [Proposed Architecture](#proposed-architecture)
4. [Implementation Plan (7 Phases)](#implementation-plan)
5. [Detailed Workflow Specifications](#detailed-workflow-specifications)
6. [Configuration Files](#configuration-files)
7. [Testing Strategy](#testing-strategy)
8. [Rollback Procedures](#rollback-procedures)
9. [Monitoring & Alerting](#monitoring--alerting)
10. [Migration Checklist](#migration-checklist)
11. [Troubleshooting Guide](#troubleshooting-guide)
12. [Appendices](#appendices)

---

## Executive Summary

### Objective

Add a `nightly` branch between `development` and `main` to provide a stabilization layer with automated builds and package creation.

### Branch Hierarchy

```
feature/* → development → nightly → main (tagged releases)
```

### Key Benefits

- **Stability Layer**: Code stabilizes in nightly before reaching main
- **Daily Testing**: Automated builds catch integration issues early
- **Package Availability**: Regular nightly releases for testing
- **Reduced Main Breakage**: Main branch stays stable for production releases

### Timeline

**Total Effort:** ~10 hours
**Duration:** 3-4 days with testing

---

## Current State Analysis

### Existing Workflows

#### 1. propagate-changes.yml

**Current Issues:**

- **Line 149**: Incorrect third parameter `'nightly'` in `createPR` call
- **Lines 151-152**: Commented out `development` → `nightly` propagation logic

**Current Structure:**

```yaml
on:
  push:
    branches:
      - main
      - development
      - nightly  # Already present but not used

jobs:
  propagate:
    steps:
      # Issue on line 149:
      - createPR('main', 'development', 'nightly')  # Third param should not exist

      # Lines 151-152 (currently commented):
      # - name: Propagate development to nightly
      #   run: createPR('development', 'nightly')
```

#### 2. docker-build.yml

**Current Triggers:**

```yaml
on:
  push:
    branches:
      - main
      - development
      - 'feature/**'
      - 'beta-release/**'
  # nightly is MISSING
```

**Tag Strategy:**

- `main` → `latest`, `v{version}`
- `development` → `dev`, `dev-{sha}`
- `feature/*` → `pr-{number}`
- Missing: `nightly` → should produce `nightly`, `nightly-{date}`, `nightly-{sha}`

#### 3. auto-versioning.yml

**Current Behavior:**

- Only triggers on `main` branch pushes
- Creates semantic version tags (v1.2.3)
- Creates GitHub releases

**No Changes Needed:** Nightly should NOT auto-version; only main gets version tags.

#### 4. release-goreleaser.yml

**Current Behavior:**

- Triggers on `v*` tag pushes
- Builds cross-platform binaries
- Creates GitHub release with artifacts

**No Changes Needed:** Only tag-triggered; nightly builds handled separately.

#### 5. supply-chain-verify.yml

**Current Issues:**

- Missing `nightly` branch in tag determination logic

**Current Tag Logic:**

```yaml
- name: Determine tag
  run: |
    if [[ "${{ github.ref }}" == "refs/heads/main" ]]; then
      echo "TAG=latest" >> $GITHUB_ENV
    elif [[ "${{ github.ref }}" == "refs/heads/development" ]]; then
      echo "TAG=dev" >> $GITHUB_ENV
    # Missing nightly case
    fi
```

### Configuration Files Status

#### ✅ .gitignore

**Status:** Properly configured, no changes needed

- Ignores `*.cover`, `*.sarif`, `*.txt` test artifacts
- Ignores `test-results/`, `playwright-report/`
- Ignores Docker overrides

#### ✅ .dockerignore

**Status:** Properly configured, no changes needed

- Excludes `.git`, `node_modules`, test files
- Includes `frontend/dist` in build context

#### ✅ Dockerfile

**Status:** Supports VERSION build arg, no changes needed

```dockerfile
ARG VERSION=dev
# ... multi-stage build with Caddy, CrowdSec, Go backend, Node frontend
```

#### ⚠️ propagate-config.yml

**Current:** Defines sensitive paths that block auto-merge
**Recommendation:** Review if nightly needs different sensitivity rules

---

## Proposed Architecture

### Branch Flow Diagram

```
┌──────────────┐
│  feature/*   │
└──────┬───────┘
       │ PR (manual review)
       ▼
┌──────────────┐
│ development  │
└──────┬───────┘
       │ Auto-merge (workflow)
       ▼
┌──────────────┐
│   nightly    │◄────┐ Daily scheduled build (02:00 UTC)
└──────┬───────┘     │
       │ Manual PR   │
       ▼             │
┌──────────────┐     │
│     main     │─────┘ Tagged releases
└──────────────┘
```

### Automation Rules

| Source | Target | Trigger | Automation Level | Review Required |
|--------|--------|---------|------------------|-----------------|
| feature/* | development | PR open | Manual | Yes |
| development | nightly | Push to dev | Automatic | No (unless sensitive files) |
| nightly | main | Manual | Manual | Yes (full review) |
| nightly | - | Schedule/Push | Build only | - |

### Package Creation Strategy

#### Docker Images (via nightly-build.yml)

**Frequency:** Daily at 02:00 UTC + on every nightly push
**Tags:**

- `nightly` (rolling latest nightly)
- `nightly-YYYY-MM-DD` (date-stamped)
- `nightly-{short-sha}` (commit-specific)

**Platforms:** linux/amd64, linux/arm64

**Security:**

- Trivy vulnerability scan
- SBOM generation (CycloneDX format)
- Grype CVE scanning
- CVE-2025-68156 verification

#### Binary Releases (via nightly-build.yml)

**Platforms:**

- Linux: amd64, arm64, arm (with CGO via Zig)
- Windows: amd64, arm64
- macOS: amd64, arm64

**Formats:**

- Archives: tar.gz (Linux/macOS), zip (Windows)
- Packages: deb, rpm

**Artifacts:** Uploaded to GitHub Actions artifacts (not GitHub Releases)

---

## Implementation Plan

### Phase 1: Update Propagate Workflow ⚡ URGENT

**Priority:** P0
**Effort:** 30 minutes
**File:** `.github/workflows/propagate-changes.yml`

#### Changes Required

**Change 1: Fix Line 149**

```yaml
# BEFORE (line 149):
await createPR('main', 'development', 'nightly');

# AFTER:
await createPR('main', 'development');
```

**Change 2: Enable Lines 151-152**

```yaml
# BEFORE (lines 151-152):
# - name: Propagate development to nightly
#   run: |
#     await createPR('development', 'nightly');

# AFTER:
- name: Propagate development to nightly
  run: |
    await createPR('development', 'nightly');
```

#### Testing

1. Create test branch from development
2. Push to development
3. Verify PR created from development to nightly
4. Verify sensitive file blocking still works

---

### Phase 2: Create Nightly Build Workflow

**Priority:** P1
**Effort:** 2 hours
**File:** `.github/workflows/nightly-build.yml` (NEW)

#### Complete Workflow Specification

```yaml
name: Nightly Build & Package

on:
  push:
    branches:
      - nightly
  schedule:
    # Daily at 02:00 UTC
    - cron: '0 2 * * *'
  workflow_dispatch:

env:
  REGISTRY: ghcr.io
  IMAGE_NAME: ${{ github.repository }}

jobs:
  build-and-push-nightly:
    runs-on: ubuntu-latest
    permissions:
      contents: read
      packages: write
      id-token: write
    outputs:
      version: ${{ steps.meta.outputs.version }}
      tags: ${{ steps.meta.outputs.tags }}
      digest: ${{ steps.build.outputs.digest }}

    steps:
      - name: Checkout code
        uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Set up QEMU
        uses: docker/setup-qemu-action@v3

      - name: Set up Docker Buildx
        uses: docker/setup-buildx-action@v3

      - name: Log in to GitHub Container Registry
        uses: docker/login-action@v3
        with:
          registry: ${{ env.REGISTRY }}
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Extract metadata
        id: meta
        uses: docker/metadata-action@v5
        with:
          images: ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}
          tags: |
            type=raw,value=nightly
            type=raw,value=nightly-{{date 'YYYY-MM-DD'}}
            type=sha,prefix=nightly-,format=short
          labels: |
            org.opencontainers.image.title=Charon Nightly
            org.opencontainers.image.description=Nightly build of Charon

      - name: Build and push Docker image
        id: build
        uses: docker/build-push-action@v6
        with:
          context: .
          platforms: linux/amd64,linux/arm64
          push: true
          tags: ${{ steps.meta.outputs.tags }}
          labels: ${{ steps.meta.outputs.labels }}
          build-args: |
            VERSION=nightly-${{ github.sha }}
          cache-from: type=gha
          cache-to: type=gha,mode=max
          provenance: true
          sbom: true

      - name: Generate SBOM
        uses: anchore/sbom-action@v0.21.1
        with:
          image: ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}:nightly
          format: cyclonedx-json
          output-file: sbom-nightly.json

      - name: Upload SBOM artifact
        uses: actions/upload-artifact@v4
        with:
          name: sbom-nightly
          path: sbom-nightly.json
          retention-days: 30

  test-nightly-image:
    needs: build-and-push-nightly
    runs-on: ubuntu-latest
    permissions:
      contents: read
      packages: read

    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Log in to GitHub Container Registry
        uses: docker/login-action@v3
        with:
          registry: ${{ env.REGISTRY }}
          username: ${{ github.actor }}
          password: ${{ secrets.GITHUB_TOKEN }}

      - name: Pull nightly image
        run: docker pull ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}:nightly

      - name: Run container smoke test
        run: |
          docker run --name charon-nightly -d \
            -p 8080:8080 \
            ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}:nightly

          # Wait for container to start
          sleep 10

          # Check container is running
          docker ps | grep charon-nightly

          # Basic health check
          curl -f http://localhost:8080/health || exit 1

          # Cleanup
          docker stop charon-nightly
          docker rm charon-nightly

  build-nightly-release:
    needs: test-nightly-image
    runs-on: ubuntu-latest
    permissions:
      contents: read

    steps:
      - name: Checkout code
        uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - name: Set up Go
        uses: actions/setup-go@v5
        with:
          go-version: "1.26.1"

      - name: Set up Node.js
        uses: actions/setup-node@v4
        with:
          node-version: '20'

      - name: Set up Zig (for cross-compilation)
        uses: goto-bus-stop/setup-zig@v2
        with:
          version: 0.11.0

      - name: Build frontend
        working-directory: ./frontend
        run: |
          npm ci
          npm run build

      - name: Run GoReleaser (snapshot mode)
        uses: goreleaser/goreleaser-action@v6
        with:
          distribution: goreleaser
          version: '~> v2'
          args: release --snapshot --skip=publish --clean
        env:
          GITHUB_TOKEN: ${{ secrets.GITHUB_TOKEN }}

      - name: Upload nightly binaries
        uses: actions/upload-artifact@v4
        with:
          name: nightly-binaries
          path: dist/*
          retention-days: 30

  verify-nightly-supply-chain:
    needs: build-and-push-nightly
    runs-on: ubuntu-latest
    permissions:
      contents: read
      packages: read

    steps:
      - name: Checkout code
        uses: actions/checkout@v4

      - name: Download SBOM
        uses: actions/download-artifact@v4
        with:
          name: sbom-nightly

      - name: Scan with Grype
        uses: anchore/scan-action@v4
        with:
          sbom: sbom-nightly.json
          fail-build: false
          severity-cutoff: high

      - name: Scan with Trivy
        uses: aquasecurity/trivy-action@0.33.1
        with:
          image-ref: ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}:nightly
          format: 'sarif'
          output: 'trivy-nightly.sarif'

      - name: Upload Trivy results
        uses: github/codeql-action/upload-sarif@v3
        with:
          sarif_file: 'trivy-nightly.sarif'
          category: 'trivy-nightly'

      - name: Check for critical CVEs
        run: |
          if grep -q "CRITICAL" trivy-nightly.sarif; then
            echo "❌ Critical vulnerabilities found in nightly build"
            exit 1
          fi
          echo "✅ No critical vulnerabilities found"
```

---

### Phase 3: Update Docker Build Workflow

**Priority:** P1
**Effort:** 30 minutes
**File:** `.github/workflows/docker-build.yml`

#### Change 1: Add nightly to triggers

```yaml
# BEFORE:
on:
  push:
    branches:
      - main
      - development
      - 'feature/**'
      - 'beta-release/**'

# AFTER:
on:
  push:
    branches:
      - main
      - development
      - nightly
      - 'feature/**'
      - 'beta-release/**'
```

#### Change 2: Update metadata action

```yaml
# BEFORE:
- name: Extract metadata
  id: meta
  uses: docker/metadata-action@v5
  with:
    tags: |
      type=ref,event=branch
      type=ref,event=pr
      type=semver,pattern={{version}}

# AFTER:
- name: Extract metadata
  id: meta
  uses: docker/metadata-action@v5
  with:
    tags: |
      type=ref,event=branch
      type=ref,event=pr
      type=semver,pattern={{version}}
      type=raw,value=nightly,enable=${{ github.ref == 'refs/heads/nightly' }}
      type=raw,value=nightly-{{date 'YYYY-MM-DD'}},enable=${{ github.ref == 'refs/heads/nightly' }}
```

#### Change 3: Update test-image tag determination

```yaml
# BEFORE:
- name: Determine test tag
  id: test-tag
  run: |
    if [[ "$GITHUB_REF" == "refs/heads/main" ]]; then
      echo "tag=latest" >> $GITHUB_OUTPUT
    elif [[ "$GITHUB_REF" == "refs/heads/development" ]]; then
      echo "tag=dev" >> $GITHUB_OUTPUT
    else
      echo "tag=${{ github.event.pull_request.number }}" >> $GITHUB_OUTPUT
    fi

# AFTER:
- name: Determine test tag
  id: test-tag
  run: |
    if [[ "$GITHUB_REF" == "refs/heads/main" ]]; then
      echo "tag=latest" >> $GITHUB_OUTPUT
    elif [[ "$GITHUB_REF" == "refs/heads/development" ]]; then
      echo "tag=dev" >> $GITHUB_OUTPUT
    elif [[ "$GITHUB_REF" == "refs/heads/nightly" ]]; then
      echo "tag=nightly" >> $GITHUB_OUTPUT
    else
      echo "tag=${{ github.event.pull_request.number }}" >> $GITHUB_OUTPUT
    fi
```

---

### Phase 4: Update Supply Chain Verification

**Priority:** P2
**Effort:** 30 minutes
**File:** `.github/workflows/supply-chain-verify.yml`

#### Change: Add nightly to tag determination

```yaml
# BEFORE:
- name: Determine tag to verify
  id: tag
  run: |
    if [[ "${{ github.ref }}" == "refs/heads/main" ]]; then
      echo "tag=latest" >> $GITHUB_ENV
    elif [[ "${{ github.ref }}" == "refs/heads/development" ]]; then
      echo "tag=dev" >> $GITHUB_ENV
    elif [[ "${{ github.event_name }}" == "pull_request" ]]; then
      echo "tag=pr-${{ github.event.pull_request.number }}" >> $GITHUB_ENV
    fi

# AFTER:
- name: Determine tag to verify
  id: tag
  run: |
    if [[ "${{ github.ref }}" == "refs/heads/main" ]]; then
      echo "tag=latest" >> $GITHUB_ENV
    elif [[ "${{ github.ref }}" == "refs/heads/development" ]]; then
      echo "tag=dev" >> $GITHUB_ENV
    elif [[ "${{ github.ref }}" == "refs/heads/nightly" ]]; then
      echo "tag=nightly" >> $GITHUB_ENV
    elif [[ "${{ github.event_name }}" == "pull_request" ]]; then
      echo "tag=pr-${{ github.event.pull_request.number }}" >> $GITHUB_ENV
    fi
```

---

### Phase 5: Configuration File Updates

**Priority:** P3
**Effort:** 1 hour

#### Optional: Create codecov.yml

**File:** `codecov.yml` (NEW)
**Purpose:** Configure Codecov behavior for nightly branch

```yaml
coverage:
  status:
    project:
      default:
        target: 85%
        threshold: 2%
        branches:
          - main
          - development
          - nightly
    patch:
      default:
        target: 85%
        branches:
          - main
          - development
          - nightly

ignore:
  - "**/*_test.go"
  - "tools/**"
  - "scripts/**"

comment:
  behavior: default
  require_changes: false
  branches:
    - nightly
```

#### Optional: Update propagate-config.yml

**File:** `.github/propagate-config.yml`
**Current:** Lists sensitive paths that block auto-propagation

**Review Question:** Should nightly have same sensitivity rules as main?

**Recommendation:** Keep same rules; nightly should be as cautious as main.

---

### Phase 6: Branch Protection Configuration

**Priority:** P1
**Effort:** 30 minutes

#### Step 1: Create Nightly Branch

```bash
# From development branch
git checkout development
git pull origin development
git checkout -b nightly
git push -u origin nightly
```

#### Step 2: Configure Branch Protection Rules

**Via GitHub UI or API:**

```json
{
  "protection": {
    "required_status_checks": {
      "strict": true,
      "contexts": [
        "build-and-push",
        "test-image",
        "playwright-e2e-tests",
        "verify-supply-chain"
      ]
    },
    "enforce_admins": false,
    "required_pull_request_reviews": null,
    "restrictions": null,
    "allow_force_pushes": true,
    "allow_deletions": false,
    "required_linear_history": false
  }
}
```

**Key Settings:**

- ✅ Require status checks to pass
- ✅ Allow force pushes (for auto-merge workflow)
- ❌ No required reviewers (auto-merge)
- ❌ No restrictions on who can push

#### Alternative: GitHub CLI Script

```bash
#!/bin/bash
# scripts/setup-nightly-branch-protection.sh

REPO="owner/charon"
BRANCH="nightly"

gh api -X PUT "/repos/$REPO/branches/$BRANCH/protection" \
  --input - <<EOF
{
  "required_status_checks": {
    "strict": true,
    "contexts": [
      "build-and-push",
      "test-image",
      "playwright-e2e-tests",
      "verify-supply-chain"
    ]
  },
  "enforce_admins": false,
  "required_pull_request_reviews": null,
  "restrictions": null,
  "allow_force_pushes": true,
  "allow_deletions": false
}
EOF
```

---

### Phase 7: Documentation Updates

**Priority:** P3
**Effort:** 1 hour

#### File 1: README.md

**Add Section:**

```markdown
## Branch Strategy

Charon uses a multi-tier branching strategy:

- **main**: Stable production releases (tagged)
- **nightly**: Daily automated builds for testing
- **development**: Integration branch for features
- **feature/***: Individual feature branches

### Getting Nightly Builds

Docker images:
```bash
docker pull ghcr.io/owner/charon:nightly
docker pull ghcr.io/owner/charon:nightly-2026-01-13
```

Binary downloads available in [GitHub Actions artifacts](link).

### Contributing

1. Fork the repository
2. Create feature branch from `development`
3. Submit PR to `development`
4. Code auto-merges to `nightly` after review
5. Manual PR from `nightly` to `main` for releases

```

#### File 2: VERSION.md
**Add Section:**

```markdown
## Nightly Builds

Nightly builds are created daily at 02:00 UTC from the `nightly` branch.

**Version Format:** `nightly-{date}` or `nightly-{sha}`

**Examples:**
- `nightly-2026-01-13`
- `nightly-a1b2c3d`

**Stability:** Nightly builds are more stable than `dev` but less stable than tagged releases.

**Use Cases:**
- Early testing of upcoming features
- Integration testing with other systems
- Bug reproduction before release

**Not Recommended For:**
- Production deployments
- Long-term support
- Critical infrastructure
```

#### File 3: CONTRIBUTING.md

**Update Workflow Section:**

```markdown
## Development Workflow

1. **Create Feature Branch**
   ```bash
   git checkout development
   git pull origin development
   git checkout -b feature/your-feature-name
   ```

1. **Develop and Test**
   - Write code with tests
   - Run `make test` locally
   - Commit with conventional commit messages

2. **Submit Pull Request**
   - Target: `development` branch
   - Fill PR template completely
   - Request reviews

3. **Automated Propagation**
   - After merge to `development`, code auto-merges to `nightly`
   - Nightly builds run automatically
   - Monitor for integration issues

4. **Release to Main**
   - Manual PR from `nightly` to `main`
   - Full review and approval required
   - Tagged release created automatically

```

---

## Testing Strategy

### Pre-Deployment Testing

#### 1. Propagate Workflow Testing
```bash
# Test development → nightly propagation
1. Create test branch: `test/nightly-propagation`
2. Push to development
3. Verify PR created automatically
4. Check PR title/body format
5. Verify sensitive file blocking
6. Merge PR manually
7. Verify nightly updated correctly
```

#### 2. Nightly Build Testing

```bash
# Test nightly build workflow
1. Trigger workflow manually via UI
2. Monitor all 4 jobs completion
3. Verify Docker image pushed with all 3 tags
4. Download and test binary artifacts
5. Review SBOM and vulnerability reports
6. Test container smoke test passes
```

#### 3. Integration Testing

```bash
# Test full integration flow
1. Make change in feature branch
2. PR to development
3. Merge to development
4. Verify auto-PR to nightly
5. Verify nightly build triggered
6. Check all packages created
7. Verify supply chain verification
```

### Post-Deployment Monitoring

#### Metrics to Track

- **Propagation Success Rate**: Should be >98%
- **Build Success Rate**: Should be >95%
- **Build Duration**: Should be <25 minutes
- **Image Size**: Monitor for bloat
- **Vulnerability Count**: Should trend downward
- **Auto-merge Failures**: Should be <2% (sensitive files only)

#### Alerting Thresholds

- ❌ Critical: Build failure 3 times in a row
- ⚠️ Warning: Build duration >30 minutes
- ⚠️ Warning: Critical CVE found
- ⚠️ Warning: Auto-merge failure rate >5%

---

## Rollback Procedures

### Scenario 1: Nightly Build Workflow Broken

```bash
# Immediate action
1. Disable nightly-build.yml workflow in GitHub UI
2. Investigate logs and identify issue
3. Fix issue in feature branch
4. Test fix in feature branch
5. Merge to development
6. Re-enable workflow

# If urgent
- Use previous nightly image tag
- Document known issues in README
```

### Scenario 2: Auto-Merge Creating Bad PRs

```bash
# Immediate action
1. Close problematic PR
2. Disable propagate-changes.yml workflow
3. Manually sync nightly with development:
   git checkout nightly
   git reset --hard development
   git push --force origin nightly
4. Fix propagate workflow
5. Re-enable workflow
```

### Scenario 3: Nightly Branch Corrupted

```bash
# Recovery steps
1. Backup current nightly:
   git checkout nightly
   git branch nightly-backup-$(date +%Y%m%d)
   git push origin nightly-backup-$(date +%Y%m%d)

2. Reset from development:
   git reset --hard origin/development
   git push --force origin nightly

3. Investigate corruption cause
4. Update branch protection if needed
```

### Scenario 4: Need to Revert Entire Feature

```bash
# If feature merged to nightly but has critical bug
1. Identify commit range to revert
2. Create revert PR:
   git checkout -b revert/feature-name
   git revert <commit-range>
   git push origin revert/feature-name
3. PR to nightly (bypasses development)
4. After verification, backport revert to development
```

---

## Monitoring & Alerting

### GitHub Actions Monitoring

#### Workflow Run Status

```yaml
# Example monitoring query (GitHub API)
GET /repos/:owner/:repo/actions/workflows/nightly-build.yml/runs
?status=failure
&created=>2026-01-01
```

#### Key Metrics

- Total runs per day: Should be ≥1 (scheduled)
- Success rate: Should be ≥95%
- Average duration: Baseline ~20 minutes
- Artifact upload rate: Should be 100%

### External Monitoring

#### Docker Image Availability

```bash
# Cron job to verify nightly image pullable
#!/bin/bash
docker pull ghcr.io/owner/charon:nightly >/dev/null 2>&1
if [ $? -ne 0 ]; then
  echo "❌ Nightly image pull failed" | notify-alerting-system
fi
```

#### Vulnerability Tracking

```bash
# Daily Grype scan comparison
grype ghcr.io/owner/charon:nightly -o json > scan-$(date +%Y%m%d).json
python scripts/compare-vuln-scans.py scan-yesterday.json scan-today.json
```

### Alerting Channels

1. **GitHub Actions Email**: Built-in workflow failure notifications
2. **Slack Integration**: Webhook for build status
3. **PagerDuty**: Critical failures only (3+ consecutive)

---

## Migration Checklist

### Pre-Migration (Day 0)

- [ ] Review all workflow files locally
- [ ] Verify no syntax errors in new nightly-build.yml
- [ ] Test propagate-changes.yml fixes in feature branch
- [ ] Create implementation branch: `feature/nightly-branch-automation`
- [ ] Document rollback procedures
- [ ] Set up monitoring baseline

### Phase 1: Propagate Workflow (Day 1 Morning)

- [ ] Update `.github/workflows/propagate-changes.yml` (2 lines)
- [ ] Commit and push to feature branch
- [ ] Create PR to development
- [ ] Test propagation in PR checks
- [ ] Merge to development
- [ ] Monitor for auto-PR creation

### Phase 2: Nightly Build Workflow (Day 1 Afternoon)

- [ ] Create `.github/workflows/nightly-build.yml`
- [ ] Commit to same feature branch
- [ ] Test workflow syntax with `act` or GitHub Actions locally
- [ ] Push to feature branch
- [ ] Merge PR to development

### Phase 3-4: Docker & Supply Chain (Day 2 Morning)

- [ ] Update `.github/workflows/docker-build.yml` (3 changes)
- [ ] Update `.github/workflows/supply-chain-verify.yml` (1 change)
- [ ] Test in feature branch
- [ ] Merge to development

### Phase 5: Configuration (Day 2 Afternoon)

- [ ] Create `codecov.yml` (if desired)
- [ ] Review `.github/propagate-config.yml`
- [ ] Test configuration changes

### Phase 6: Branch Creation (Day 3 Morning)

- [ ] Create nightly branch from development
- [ ] Push to GitHub
- [ ] Configure branch protection rules
- [ ] Test force push capability
- [ ] Verify status check requirements

### Phase 7: Documentation (Day 3 Afternoon)

- [ ] Update `README.md`
- [ ] Update `VERSION.md`
- [ ] Update `CONTRIBUTING.md`
- [ ] Create PR with documentation changes
- [ ] Merge to development

### Post-Migration Testing (Day 3-4)

- [ ] Trigger manual nightly build
- [ ] Verify all 4 jobs complete successfully
- [ ] Check Docker images published with all tags
- [ ] Download and test binary artifacts
- [ ] Verify SBOM generated correctly
- [ ] Check vulnerability scan reports
- [ ] Test auto-merge from development
- [ ] Verify scheduled build runs at 02:00 UTC

### Monitoring Period (Day 5-14)

- [ ] Monitor daily build success rate
- [ ] Track auto-merge success rate
- [ ] Review vulnerability trends
- [ ] Check artifact retention working
- [ ] Verify alerting triggers correctly
- [ ] Document any issues encountered

### Sign-Off (Day 15)

- [ ] Review all metrics meet targets
- [ ] Document lessons learned
- [ ] Update troubleshooting guide
- [ ] Archive implementation artifacts
- [ ] Celebrate success! 🎉

---

## Troubleshooting Guide

### Issue 1: Auto-Merge PR Not Created

**Symptoms:**

- Push to development completes
- No PR created from development to nightly

**Diagnosis:**

```bash
# Check workflow run logs
gh run list --workflow=propagate-changes.yml --limit=1
gh run view <run-id> --log

# Check if workflow triggered
gh api /repos/:owner/:repo/actions/workflows/propagate-changes.yml/runs
```

**Common Causes:**

1. Workflow file syntax error
2. GitHub token permissions insufficient
3. Sensitive file detected (expected behavior)
4. Branch protection blocking push

**Solutions:**

1. Validate workflow YAML syntax
2. Check `GITHUB_TOKEN` permissions in workflow
3. Review `.github/propagate-config.yml` for false positives
4. Verify nightly branch allows force pushes

### Issue 2: Nightly Build Failing

**Symptoms:**

- Workflow triggered but fails
- Red X on GitHub Actions

**Diagnosis:**

```bash
# View failure details
gh run list --workflow=nightly-build.yml --status=failure --limit=5
gh run view <run-id> --log-failed

# Check specific job
gh run view <run-id> --job=<job-id>
```

**Common Causes:**

1. Docker build timeout (>25 minutes)
2. Test failure in smoke test
3. Dependency download failure
4. Registry authentication failure

**Solutions:**

1. Check Docker build cache; may need --no-cache
2. Review test logs; may be transient network issue
3. Verify `actions/checkout` fetched correctly
4. Re-run workflow; transient failures common

### Issue 3: Nightly Image Not Pullable

**Symptoms:**

- Build succeeds but `docker pull` fails
- Image not in ghcr.io registry

**Diagnosis:**

```bash
# Check if image pushed
gh api /user/packages/container/charon/versions

# Try pulling with debug
docker pull ghcr.io/owner/charon:nightly --debug

# Check registry permissions
gh api /user/packages/container/charon
```

**Common Causes:**

1. Package visibility set to private
2. GitHub token expired during push
3. Multi-arch manifest not created
4. Tag overwrite issue

**Solutions:**

1. Make package public in GitHub UI
2. Check `docker/login-action` in workflow
3. Verify `docker/build-push-action` platforms
4. Check for conflicting tag push

### Issue 4: Binary Build Fails

**Symptoms:**

- Docker build succeeds
- `build-nightly-release` job fails

**Diagnosis:**

```bash
# Check GoReleaser logs
gh run view <run-id> --job=build-nightly-release --log

# Test locally
goreleaser release --snapshot --skip=publish --clean
```

**Common Causes:**

1. Frontend build missing
2. Zig not installed correctly
3. CGO dependency missing
4. .goreleaser.yaml syntax error

**Solutions:**

1. Verify frontend build step runs before GoReleaser
2. Check `goto-bus-stop/setup-zig` action version
3. Install build-essential in workflow
4. Validate .goreleaser.yaml with `goreleaser check`

---

## Appendices

### Appendix A: Workflow Trigger Matrix

| Workflow | main | development | nightly | feature/* | schedule | manual |
|----------|------|-------------|---------|-----------|----------|--------|
| propagate-changes.yml | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ |
| docker-build.yml | ✅ | ✅ | ✅ | ✅ | ❌ | ❌ |
| nightly-build.yml | ❌ | ❌ | ✅ | ❌ | ✅ (daily) | ✅ |
| auto-versioning.yml | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ |
| release-goreleaser.yml | ❌ | ❌ | ❌ | ❌ | ❌ | tags only |
| supply-chain-verify.yml | ✅ | ✅ | ✅ | ❌ | ❌ | ❌ |

### Appendix B: Tag Naming Convention

| Branch | Docker Tags | Binary Version | SBOM Name |
|--------|-------------|----------------|-----------|
| main | `latest`, `v1.2.3` | `v1.2.3` | `sbom-v1.2.3.json` |
| development | `dev`, `dev-a1b2c3d` | `dev-a1b2c3d` | `sbom-dev.json` |
| nightly | `nightly`, `nightly-2026-01-13`, `nightly-a1b2c3d` | `nightly-a1b2c3d` | `sbom-nightly.json` |
| feature/* | `pr-123` | N/A | `sbom-pr-123.json` |

### Appendix C: Resource Requirements

#### Compute

- **Docker Build**: 4 vCPU, 8 GB RAM, ~20 minutes
- **GoReleaser**: 2 vCPU, 4 GB RAM, ~10 minutes
- **Tests**: 2 vCPU, 4 GB RAM, ~5 minutes
- **Total per nightly**: ~35 minutes of runner time

#### Storage

- **Docker Images**: ~500 MB per arch × 2 = 1 GB per build
- **Binary Artifacts**: ~200 MB per platform × 6 = 1.2 GB per build
- **SBOMs**: ~5 MB per build
- **Total per day**: ~2.2 GB (retained 30 days = ~66 GB)

#### Actions Minutes

- **Free Tier**: 2,000 minutes/month
- **Nightly Usage**: ~35 minutes × 30 days = 1,050 minutes/month
- **Buffer**: ~950 minutes for other workflows
- **Recommendation**: Monitor usage; may need paid tier

### Appendix D: Security Considerations

#### Secrets Management

- **GITHUB_TOKEN**: Scoped to repository, auto-generated
- **Registry Access**: Uses GITHUB_TOKEN, no extra secrets
- **Signing Keys**: Store in repository secrets if using Cosign

#### Vulnerability Response

1. **Critical CVE Found**:
   - Automatic scan failure
   - Block deployment
   - Create security issue
   - Patch within 24 hours

2. **High CVE Found**:
   - Log warning
   - Allow deployment
   - Create tracking issue
   - Patch within 7 days

3. **Medium/Low CVE**:
   - Log for awareness
   - Address in next cycle

#### Supply Chain Integrity

- **SBOM**: Always generated, retained 30 days
- **Provenance**: Docker buildx includes provenance
- **Signatures**: Optional Cosign signing
- **Attestation**: GitHub Actions attestation built-in

### Appendix E: Future Enhancements

1. **Multi-Architecture Binary Testing**
   - QEMU-based testing for ARM64
   - Windows container testing
   - macOS binary testing via hosted runners

2. **Performance Benchmarking**
   - Automated performance regression tests
   - Historical tracking of build times
   - Resource usage profiling

3. **Enhanced Notifications**
   - Slack integration for build status
   - Email digest of nightly builds
   - RSS feed for release notes

4. **Deployment Automation**
   - Automated staging deployment from nightly
   - Blue/green deployment testing
   - Automated rollback on failure

5. **Advanced Analytics**
   - Build success trends
   - Code coverage over time
   - Dependency update frequency
   - Security posture tracking

---

## Conclusion

This specification provides a complete implementation plan for adding a nightly branch with automated builds and package creation. The 7-phase approach ensures incremental rollout with testing at each step.

**Key Success Factors:**

1. Fix propagate workflow issues first (Phase 1)
2. Test thoroughly in feature branch before merging
3. Monitor metrics closely for first 2 weeks
4. Document any deviations or issues
5. Iterate based on real-world usage

**Contact for Questions:**

- Implementation Team: [link]
- DevOps Channel: [link]
- Documentation: This file

**Last Updated:** 2026-01-13
**Next Review:** After Phase 7 completion
