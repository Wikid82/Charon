# CI/CD Workflow Audit Report

**Date:** January 15, 2026
**Auditor:** Planning Agent + Supervisor Review
**Repository:** Charon
**Standard:** GitHub Actions CI/CD Best Practices (`.github/instructions/github-actions-ci-cd-best-practices.instructions.md`)

---

## 1. Executive Summary

### Overall Health Score: **78/100** ⭐⭐⭐⭐ (Revised after Supervisor Review)

| Category | Score | Status | Change |
|----------|-------|--------|--------|
| Security | 75/100 | ⚠️ Needs Attention | ↓15 (hardcoded secret found) |
| Performance | 82/100 | ✅ Good | — |
| Structure | 85/100 | ✅ Good | ↓7 (artifact mismatch bug) |
| Testing | 80/100 | ✅ Good | ↓8 (E2E tests may fail silently) |
| Deployment | 85/100 | ✅ Good | — |

**Summary:** The Charon repository demonstrates strong CI/CD practices overall but has **critical issues identified during Supervisor review** that require immediate action:

1. **🔴 CRITICAL:** Hardcoded encryption key in `playwright.yml` (security risk)
2. **🔴 CRITICAL:** Artifact filename mismatch causing supply-chain verification to fail silently
3. **🔴 CRITICAL:** GoReleaser uses `version: latest` (supply chain risk)
4. **🟠 HIGH:** CodeQL action major version inconsistency (v3 vs v4)
5. **🟡 MEDIUM:** Shell variable escaping issues in release workflow

**Note:** The `no-cache: true` setting in `docker-build.yml` is **intentional security hardening** to prevent false-positive vulnerabilities from cached layers—this is NOT a gap.

---

## 2. Per-Workflow Analysis

### 2.1 docker-build.yml (Docker Build, Publish & Test)

**Purpose:** Main build workflow for Docker images with multi-platform support, SBOM generation, and security scanning.

#### Strengths ✅

- **Permissions:** Explicitly defined with least privilege (`contents: read`, `packages: write`, `security-events: write`, `id-token: write`, `attestations: write`)
- **Concurrency:** Properly configured with `cancel-in-progress: true`
- **Action Pinning:** All actions pinned to full SHA (excellent!)
- **SBOM Generation:** Uses `anchore/sbom-action` for supply chain security
- **SBOM Attestation:** Implements `actions/attest-sbom` for verifiable attestations
- **Security Scanning:** Trivy integration with SARIF upload
- **CVE Verification:** Custom checks for CVE-2025-68156 in Caddy and CrowdSec
- **Smart Skip Logic:** Skips builds for chore commits and Renovate bot
- **No-Cache Security:** Intentional `no-cache: true` prevents false-positive vulnerabilities from cached layers ✅

#### Issues Found

| Severity | Issue | Line(s) | Recommendation |
|----------|-------|---------|----------------|
| LOW | `actions/checkout` uses SHA but label says `v6` | 47 | Update comment to reflect actual pinned version |
| LOW | `continue-on-error: true` on CVE verification | 202, 245 | Consider making CVE checks blocking for security-critical builds |

#### Score: **94/100**

---

### 2.2 playwright.yml (Playwright E2E Tests)

**Purpose:** End-to-end testing using Playwright against PR Docker images.

#### Strengths ✅

- **Workflow Orchestration:** Properly chains from `docker-build.yml` via `workflow_run`
- **Concurrency:** Well-configured cancellation
- **Action Pinning:** All actions pinned to full SHA with version comments
- **Node.js Caching:** Uses `cache: 'npm'` in setup-node
- **Artifact Cleanup:** Proper retention policy (14 days)
- **Health Checks:** Robust health endpoint polling before tests

#### Issues Found

| Severity | Issue | Line(s) | Recommendation |
|----------|-------|---------|----------------|
| **🔴 CRITICAL** | **Hardcoded encryption key in plaintext** | 31 | **MUST move to GitHub Secrets immediately** |
| MEDIUM | Missing `timeout-minutes` on job | 23 | Add timeout to prevent hung workflows |
| LOW | Permissions not explicitly defined | - | Add explicit `permissions` block for clarity |
| LOW | Test report retention could be shorter | 139 | Consider 7 days for PR artifacts |

> **⚠️ SUPERVISOR FINDING:** Line 31 contains `CHARON_ENCRYPTION_KEY: dGVzdC1lbmNyeXB0aW9uLWtleS1mb3ItY2ktMzJieXQ=` hardcoded in the workflow file. Even if this is a "test" key, hardcoded secrets in YAML files are a security violation and set a bad precedent.

#### Score: **65/100** (↓20 due to hardcoded secret)

---

### 2.3 security-pr.yml (Security Scan for PRs)

**Purpose:** Trivy security scanning on PR Docker images.

#### Strengths ✅

- **Permissions:** Explicitly defined with appropriate scopes
- **Timeout:** Job timeout of 10 minutes configured
- **SARIF Upload:** Results uploaded to GitHub Security tab
- **Binary Extraction:** Extracts and scans the compiled binary specifically
- **Exit Code:** Fails on CRITICAL/HIGH vulnerabilities

#### Issues Found

| Severity | Issue | Line(s) | Recommendation |
|----------|-------|---------|----------------|
| LOW | Duplicate artifact checking logic | 33-77 | Consider extracting to reusable action |
| LOW | `continue-on-error: true` on SARIF upload | 115 | Should document why this is acceptable |

#### Score: **90/100**

---

### 2.4 supply-chain-pr.yml (Supply Chain Verification for PRs)

**Purpose:** SBOM generation and vulnerability scanning using Syft/Grype for PRs.

#### Strengths ✅

- **Permissions:** Comprehensive and appropriate
- **Concurrency:** Properly configured
- **PR Comments:** Provides detailed security results directly on PR
- **Vulnerability Categorization:** Counts by severity (Critical/High/Medium/Low)
- **Failure Gate:** Fails on critical vulnerabilities
- **Tool Versions:** Pinned Syft and Grype versions in environment variables

#### Issues Found

| Severity | Issue | Line(s) | Recommendation |
|----------|-------|---------|----------------|
| **🔴 CRITICAL** | **Artifact filename mismatch - looks for `pr-image.tar` but docker-build.yml saves as `charon-pr-image.tar`** | 152 | **Fix filename to match docker-build.yml output** |
| **🟠 HIGH** | **CodeQL action uses v3.28.1 while all other workflows use v4.31.10** | 177 | **Major version gap (v3→v4) - standardize immediately** |
| LOW | Sparse checkout may cause issues with some tools | 46-49 | Document why sparse checkout is sufficient |

> **⚠️ SUPERVISOR FINDING - BUG:** Line 152 expects `pr-image.tar` but `docker-build.yml` line 140 saves as `charon-pr-image.tar`. This mismatch causes supply-chain verification to fail silently for all PRs! The workflow shows "pr-image.tar not found" and exits without actual verification.
>
> **⚠️ SUPERVISOR FINDING - VERSION GAP:** Line 177 uses CodeQL action `v3.28.1` (`b56ba49b26e50535fa1e7f7db0f4f7b45bf65d80d`) while all other workflows use `v4.31.10`. This is a **major version gap** with potential SARIF schema compatibility issues.

#### Score: **70/100** (↓18 due to critical bug and version gap)

---

### 2.5 release-goreleaser.yml (Release with GoReleaser)

**Purpose:** Release automation using GoReleaser for cross-platform builds.

#### Strengths ✅

- **Concurrency:** `cancel-in-progress: false` for releases (correct!)
- **Full Checkout:** `fetch-depth: 0` for release tagging
- **Cross-Compilation:** Zig toolchain for CGO support

#### Issues Found

| Severity | Issue | Line(s) | Recommendation |
|----------|-------|---------|----------------|
| **🔴 CRITICAL** | `goreleaser-action` uses `version: latest` | 46 | **Pin to specific version for reproducible builds** |
| **🟠 HIGH** | Permissions too broad (`contents: write`, `packages: write`) | 19-20 | Consider using environment protection |
| **🟡 MEDIUM** | **Double `$$` shell escaping issue** | 38 | **Fix: `VERSION=${GITHUB_REF#refs/tags/}` (single `$`)** |
| MEDIUM | `actions/setup-node` pinned to older SHA than others | 32 | Standardize action versions |

> **⚠️ SUPERVISOR FINDING:** Line 38 has `VERSION=$${GITHUB_REF#refs/tags/}` with double `$$`. In GitHub Actions YAML, this is incorrect shell escaping. Should be single `$` for shell variable expansion.

#### Score: **70/100** (↓5 due to additional shell escaping issue)

---

### 2.6 codeql.yml (CodeQL Analysis)

**Purpose:** Static Application Security Testing (SAST) for Go and JavaScript.

#### Strengths ✅

- **Permissions:** Least privilege with job-level override
- **Matrix Strategy:** Tests both Go and JavaScript in parallel
- **Config File:** Uses custom CodeQL config for documented exclusions
- **Schedule:** Weekly scheduled scans
- **Fail on High-Severity:** Blocks merge on critical findings

#### Issues Found

| Severity | Issue | Line(s) | Recommendation |
|----------|-------|---------|----------------|
| LOW | `fail-fast: false` may slow PR feedback | 34 | Consider `fail-fast: true` for PRs, `false` for scheduled |
| LOW | Forked PR handling could be more elegant | 31 | Document security implications clearly |

#### Score: **95/100**

---

### 2.7 quality-checks.yml (Quality Checks)

**Purpose:** Backend and frontend quality checks including tests, linting, and coverage.

#### Strengths ✅

- **Path-Based Optimization:** Frontend jobs detect if frontend changed
- **Caching:** Go cache and npm cache properly configured
- **Performance Assertions:** Custom perf tests with configurable thresholds
- **Job Separation:** Clear separation between backend and frontend

#### Issues Found

| Severity | Issue | Line(s) | Recommendation |
|----------|-------|---------|----------------|
| MEDIUM | Permissions not defined | - | Add explicit `permissions: contents: read` |
| LOW | `continue-on-error: true` on golangci-lint | 58 | Consider making lint failures blocking |
| LOW | Duplicate repo health check in both jobs | 28, 73 | Consider extracting to separate job |

#### Score: **85/100**

---

### 2.8 nightly-build.yml (Nightly Build & Package)

**Purpose:** Daily automated builds with comprehensive supply chain verification.

#### Strengths ✅

- **GHA Caching:** Uses `cache-from: type=gha` for Docker builds
- **SBOM Generation:** Both inline SBOM and artifact upload
- **Smoke Tests:** Basic health check against nightly image
- **Artifact Retention:** 30 days for nightly artifacts (appropriate)

#### Issues Found

| Severity | Issue | Line(s) | Recommendation |
|----------|-------|---------|----------------|
| **HIGH** | Actions pinned to different versions than other workflows | Multiple | **Standardize action versions across all workflows** |
| MEDIUM | Hardcoded Go version `1.23` differs from env var pattern | 113 | Use environment variable like other workflows |
| MEDIUM | Hardcoded Node version `20` differs from other workflows | 118 | Use `24.12.0` for consistency |
| LOW | Health check endpoint differs (`/health` vs `/api/v1/health`) | 95 | Verify correct endpoint |

#### Score: **78/100**

---

### 2.9 benchmark.yml (Go Benchmark)

**Purpose:** Performance regression detection using Go benchmarks.

#### Strengths ✅

- **Path Filtering:** Only runs when backend changes
- **Caching:** Go cache properly configured
- **Benchmark Storage:** Uses `github-action-benchmark` for trend tracking
- **Alert Threshold:** 175% threshold accounts for CI variability

#### Issues Found

| Severity | Issue | Line(s) | Recommendation |
|----------|-------|---------|----------------|
| MEDIUM | `permissions: contents: write` on all runs | 21 | Restrict to push events only |
| LOW | `fail-on-alert: false` may miss regressions | 37 | Consider `true` for critical paths |

#### Score: **88/100**

---

### 2.10 codecov-upload.yml (Coverage Upload)

**Purpose:** Upload code coverage to Codecov for backend and frontend.

#### Strengths ✅

- **Dedicated Workflow:** Separates coverage upload from test runs
- **Push-Only:** Correctly triggers only on pushes, not PRs
- **Fail on Error:** `fail_ci_if_error: true` ensures reliability

#### Issues Found

| Severity | Issue | Line(s) | Recommendation |
|----------|-------|---------|----------------|
| LOW | Missing timeout on jobs | - | Add `timeout-minutes: 15` |
| LOW | Missing concurrency group | - | Add for consistency |

#### Score: **85/100**

---

### 2.11 supply-chain-verify.yml (Supply Chain Verification)

**Purpose:** Comprehensive supply chain verification for releases.

#### Strengths ✅

- **Comprehensive Verification:** SBOM validation, vulnerability scanning, Cosign verification
- **PR Comments:** Detailed security summaries on PRs
- **Artifact Upload:** 30-day retention for audit trails
- **Fallback Logic:** Handles Rekor unavailability gracefully

#### Issues Found

| Severity | Issue | Line(s) | Recommendation |
|----------|-------|---------|----------------|
| MEDIUM | Cosign checksum verification is commented out | 281 | Enable with correct SHA |
| LOW | `continue-on-error: true` on Grype scan | 255 | Document acceptable failure scenarios |

#### Score: **85/100**

---

### 2.12 security-weekly-rebuild.yml (Weekly Security Rebuild)

**Purpose:** Weekly fresh builds to incorporate latest security patches.

#### Strengths ✅

- **No Cache:** Forced fresh builds for security
- **Comprehensive Scanning:** Multiple Trivy output formats
- **Long Retention:** 90 days for weekly scans (audit trail)
- **Package Version Reporting:** Shows installed Alpine packages

#### Issues Found

| Severity | Issue | Line(s) | Recommendation |
|----------|-------|---------|----------------|
| LOW | `continue-on-error: true` on CRITICAL/HIGH scan | 67 | Consider failing workflow on vulnerabilities |

#### Score: **90/100**

---

### 2.13 Minor Workflows Summary

| Workflow | Score | Key Issues |
|----------|-------|------------|
| `docker-lint.yml` | 95/100 | Missing permissions block |
| `renovate.yml` | 90/100 | Consider adding timeout |
| `waf-integration.yml` | 92/100 | Well-structured with good debugging |
| `docs.yml` | 88/100 | Missing timeout on jobs |
| `repo-health.yml` | 90/100 | Good structure and artifact handling |

---

## 3. Categorized Issues (Revised with Supervisor Findings)

### 🔴 CRITICAL (Block Merge - MUST FIX BEFORE ANY MERGE)

| # | Workflow | Issue | Impact | Added By |
|---|----------|-------|--------|----------|
| 1 | `playwright.yml` | **Hardcoded encryption key** (`CHARON_ENCRYPTION_KEY`) in plaintext at line 31 | Secret exposure, security policy violation | 🔍 Supervisor |
| 2 | `supply-chain-pr.yml` | **Artifact filename mismatch** - expects `pr-image.tar` but docker-build saves as `charon-pr-image.tar` | Supply chain verification silently failing for ALL PRs | 🔍 Supervisor |
| 3 | `release-goreleaser.yml` | GoReleaser action uses `version: latest` | Non-reproducible builds, supply chain risk | Planning Agent |

### 🟠 HIGH (Requires Immediate Action)

| # | Workflow | Issue | Impact | Added By |
|---|----------|-------|--------|----------|
| 4 | `supply-chain-pr.yml` | **CodeQL action v3 vs v4** - uses `v3.28.1` while others use `v4.31.10` | Major version gap, SARIF compatibility issues | 🔍 Supervisor (upgraded) |
| 5 | `release-goreleaser.yml` | Broad permissions without environment protection | Security risk for release process | Planning Agent |
| 6 | `nightly-build.yml` | Inconsistent action versions across workflows | Maintenance burden, potential compatibility issues | Planning Agent |

### 🟡 MEDIUM (Requires Discussion)

| # | Workflow | Issue | Impact | Added By |
|---|----------|-------|--------|----------|
| 7 | `release-goreleaser.yml` | **Double `$$` shell escaping** at line 38 | Version injection failure in frontend builds | 🔍 Supervisor |
| 8 | `playwright.yml` | Missing job timeout | Potential hung workflows | Planning Agent |
| 9 | `quality-checks.yml` | Missing explicit permissions | Security best practice violation | Planning Agent |
| 10 | `benchmark.yml` | Write permissions on all events | Unnecessary privilege escalation | Planning Agent |
| 11 | `nightly-build.yml` | Hardcoded language versions | Maintenance burden | Planning Agent |

### 🟢 LOW (Suggestions)

| # | Workflow | Issue | Impact |
|---|----------|-------|--------|
| 12 | Multiple | `continue-on-error: true` without documentation | Unclear failure handling |
| 13 | Multiple | Duplicate reusable logic | Code duplication |
| 14 | Multiple | Artifact retention inconsistencies | Storage optimization |
| 15 | `codecov-upload.yml` | Missing concurrency group | Potential duplicate runs |

### ❌ REMOVED Issues (Supervisor Correction)

| # | Original Issue | Reason for Removal |
|---|----------------|-------------------|
| ~~4~~ | `docker-build.yml` - No Docker layer caching | **Intentional security hardening** - `no-cache: true` prevents false-positive vulnerabilities from cached layers |

---

## 4. Specific Remediation Recommendations

### 4.1 🔴 CRITICAL: Move Hardcoded Encryption Key to GitHub Secrets

**File:** `playwright.yml`
**Line:** 31

```yaml
# ❌ Current (SECURITY VIOLATION)
env:
  CHARON_ENV: development
  CHARON_DEBUG: "1"
  CHARON_ENCRYPTION_KEY: dGVzdC1lbmNyeXB0aW9uLWtleS1mb3ItY2ktMzJieXQ=  # HARDCODED!

# ✅ Recommended
env:
  CHARON_ENV: development
  CHARON_DEBUG: "1"
  CHARON_ENCRYPTION_KEY: ${{ secrets.CHARON_CI_ENCRYPTION_KEY }}
```

**Setup Steps:**
1. Go to Repository Settings → Secrets and variables → Actions
2. Create new repository secret: `CHARON_CI_ENCRYPTION_KEY`
3. Set value to a proper test encryption key (32 bytes, base64 encoded)
4. Update workflow to reference the secret

---

### 4.2 🔴 CRITICAL: Fix Artifact Filename Mismatch

**File:** `supply-chain-pr.yml`
**Line:** 152

```yaml
# ❌ Current (BUG - filename mismatch)
- name: Load Docker image
  if: steps.check-artifact.outputs.artifact_found == 'true'
  id: load-image
  run: |
    if [[ ! -f "pr-image.tar" ]]; then    # WRONG: expects pr-image.tar
      echo "❌ pr-image.tar not found in artifact"
      ls -la
      exit 1
    fi

# ✅ Recommended (match docker-build.yml output)
- name: Load Docker image
  if: steps.check-artifact.outputs.artifact_found == 'true'
  id: load-image
  run: |
    if [[ ! -f "charon-pr-image.tar" ]]; then    # CORRECT: matches docker-build.yml
      echo "❌ charon-pr-image.tar not found in artifact"
      ls -la
      exit 1
    fi

    echo "🐳 Loading Docker image..."
    LOAD_OUTPUT=$(docker load -i charon-pr-image.tar)    # CORRECT filename
    echo "${LOAD_OUTPUT}"
```

**Root Cause:** `docker-build.yml` line 140 saves: `docker save "${IMAGE_TAG}" -o /tmp/charon-pr-image.tar`

---

### 4.3 🔴 CRITICAL: Pin GoReleaser to Specific Version

**File:** `release-goreleaser.yml`
**Line:** 46

```yaml
# ❌ Current (Insecure)
- name: Run GoReleaser
  uses: goreleaser/goreleaser-action@e435ccd777264be153ace6237001ef4d979d3a7a # v6
  with:
    distribution: goreleaser
    version: latest  # PROBLEM: Non-reproducible

# ✅ Recommended
- name: Run GoReleaser
  uses: goreleaser/goreleaser-action@e435ccd777264be153ace6237001ef4d979d3a7a # v6
  with:
    distribution: goreleaser
    version: '~> v2.5'  # Pin to specific major.minor
    args: release --clean
```

---

### 4.4 🟠 HIGH: Upgrade CodeQL Action to v4

**File:** `supply-chain-pr.yml`
**Line:** 177

```yaml
# ❌ Current (Version mismatch - v3)
- name: Upload SARIF to GitHub Security
  # github/codeql-action v3.28.1
  uses: github/codeql-action/upload-sarif@b56ba49b26e50535fa1e7f7db0f4f7b4bf65d80d
  continue-on-error: true
  with:
    sarif_file: grype-results.sarif
    category: supply-chain-pr

# ✅ Recommended (Match other workflows - v4)
- name: Upload SARIF to GitHub Security
  # github/codeql-action v4.31.10
  uses: github/codeql-action/upload-sarif@cdefb33c0f6224e58673d9004f47f7cb3e328b89
  continue-on-error: true
  with:
    sarif_file: grype-results.sarif
    category: supply-chain-pr
```

---

### 4.5 🟠 HIGH: Add Environment Protection for Releases

**File:** `release-goreleaser.yml`

```yaml
# ✅ Add environment protection
jobs:
  goreleaser:
    runs-on: ubuntu-latest
    environment:
      name: release
      url: https://github.com/${{ github.repository }}/releases
    permissions:
      contents: write
      packages: write
```

Then configure environment protection rules in GitHub repository settings:

1. Go to Settings → Environments → Create "release"
2. Add required reviewers
3. Restrict to protected branches (tags matching `v*`)

---

### 4.6 🟡 MEDIUM: Fix Shell Variable Escaping

**File:** `release-goreleaser.yml`
**Line:** 38

```yaml
# ❌ Current (Double $$ escaping issue)
- name: Build Frontend
  working-directory: frontend
  run: |
    # Inject version into frontend build from tag (if present)
    VERSION=$${GITHUB_REF#refs/tags/}   # WRONG: Double $$
    echo "VITE_APP_VERSION=$$VERSION" >> $GITHUB_ENV   # WRONG: Double $$
    npm ci
    npm run build

# ✅ Recommended (Single $ for shell variables)
- name: Build Frontend
  working-directory: frontend
  run: |
    # Inject version into frontend build from tag (if present)
    VERSION=${GITHUB_REF#refs/tags/}   # CORRECT: Single $
    echo "VITE_APP_VERSION=${VERSION}" >> $GITHUB_ENV   # CORRECT: Single $
    npm ci
    npm run build
```

**Note:** In GitHub Actions `run:` blocks, shell variables use single `$`. Double `$$` is only needed when you want a literal `$` character in the output.

---

### 4.7 MEDIUM: Add Explicit Permissions to quality-checks.yml

**File:** `quality-checks.yml`

```yaml
name: Quality Checks

on:
  push:
    branches: [ main, development, 'feature/**' ]
  pull_request:
    branches: [ main, development ]

concurrency:
  group: ${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: true

# ADD: Explicit permissions block
permissions:
  contents: read
  checks: write  # If you want test annotations

env:
  GO_VERSION: '1.25.5'
  NODE_VERSION: '24.12.0'
```

---

### 4.8 MEDIUM: Standardize Action Versions Across Workflows

Create a shared workflow or use renovate to ensure consistency:

**Recommended Standard Versions (as of audit date):**

| Action | Recommended SHA | Version |
|--------|-----------------|---------|
| `actions/checkout` | `8e8c483db84b4bee98b60c0593521ed34d9990e8` | v6 |
| `actions/setup-go` | `7a3fe6cf4cb3a834922a1244abfce67bcef6a0c5` | v6 |
| `actions/setup-node` | `395ad3262231945c25e8478fd5baf05154b1d79f` | v6 |
| `actions/upload-artifact` | `b7c566a772e6b6bfb58ed0dc250532a479d7789f` | v6.0.0 |
| `actions/download-artifact` | `fa0a91b85d4f404e444e00e005971372dc801d16` | v4.1.8 |
| `docker/build-push-action` | `263435318d21b8e681c14492fe198d362a7d2c83` | v6 |
| `github/codeql-action/*` | `cdefb33c0f6224e58673d9004f47f7cb3e328b89` | **v4.31.10** |
| `aquasecurity/trivy-action` | `b6643a29fecd7f34b3597bc6acb0a98b03d33ff8` | 0.33.1 |

> ⚠️ **Note:** `supply-chain-pr.yml` uses CodeQL v3.28.1 - must upgrade to v4.31.10!

---

### 4.9 LOW: Add Missing Timeouts

Add to all job definitions without explicit timeouts:

```yaml
jobs:
  job-name:
    runs-on: ubuntu-latest
    timeout-minutes: 30  # Adjust based on expected duration
```

Recommended timeouts:

- Build jobs: 30 minutes
- Test jobs: 15 minutes
- Lint jobs: 10 minutes
- Security scan jobs: 15 minutes
- Deploy jobs: 20 minutes

---

## 5. Priority-Ordered Action Items (Revised with Supervisor Findings)

### 🚨 IMMEDIATE (Block All PRs Until Fixed)

| # | Priority | Task | Workflow | Effort |
|---|----------|------|----------|--------|
| 1 | 🔴 CRITICAL | Move hardcoded encryption key to GitHub Secrets | `playwright.yml` | 15 min |
| 2 | 🔴 CRITICAL | Fix artifact filename mismatch (`pr-image.tar` → `charon-pr-image.tar`) | `supply-chain-pr.yml` | 10 min |
| 3 | 🔴 CRITICAL | Pin GoReleaser to specific version (`~> v2.5`) | `release-goreleaser.yml` | 5 min |

### 🔶 This Sprint (Within 1 Week)

| # | Priority | Task | Workflow | Effort |
|---|----------|------|----------|--------|
| 4 | 🟠 HIGH | Upgrade CodeQL action from v3 to v4 | `supply-chain-pr.yml` | 10 min |
| 5 | 🟠 HIGH | Add environment protection rules for releases | `release-goreleaser.yml` | 30 min |
| 6 | 🟠 HIGH | Standardize action versions in nightly builds | `nightly-build.yml` | 20 min |

### 🟡 Short-Term (Next 2 Sprints)

| # | Priority | Task | Workflow | Effort |
|---|----------|------|----------|--------|
| 7 | 🟡 MEDIUM | Fix shell variable escaping (`$$` → `$`) | `release-goreleaser.yml` | 5 min |
| 8 | 🟡 MEDIUM | Add explicit permissions block | `quality-checks.yml` | 10 min |
| 9 | 🟡 MEDIUM | Add job timeouts | `playwright.yml`, `codecov-upload.yml`, `docs.yml` | 15 min |
| 10 | 🟡 MEDIUM | Reduce benchmark write permissions to push only | `benchmark.yml` | 5 min |

### 🟢 Long-Term (Backlog)

| # | Priority | Task | Workflow | Effort |
|---|----------|------|----------|--------|
| 11 | 🟢 LOW | Create reusable workflow for artifact downloading | Multiple | 2 hrs |
| 12 | 🟢 LOW | Document all `continue-on-error: true` decisions | Multiple | 1 hr |
| 13 | 🟢 LOW | Standardize artifact retention periods | Multiple | 30 min |
| 14 | 🟢 LOW | Add concurrency group | `codecov-upload.yml` | 5 min |

---

## 6. Compliance Checklist (Updated)

### Security Checklist

- [x] GITHUB_TOKEN permissions explicitly defined with least privilege (most workflows)
- [ ] **⚠️ FAIL: Hardcoded secret in `playwright.yml` line 31** - MUST FIX
- [x] Secrets accessed via `secrets.<NAME>` only (except above violation)
- [x] OIDC used for attestations (`id-token: write`)
- [x] Actions pinned to full SHA (excellent coverage)
- [x] Dependency review / SCA integrated (Grype, Syft)
- [x] SAST (CodeQL) integrated
- [ ] Secret scanning enabled (verify in repo settings)
- [ ] All actions pinned consistently (needs standardization - v3/v4 gap)

### Performance Checklist

- [x] Caching implemented for Go and Node dependencies
- [x] Docker layer caching intentionally disabled for security (`no-cache: true`) ✅
- [x] Matrix strategies used (CodeQL)
- [x] Shallow clones used where appropriate
- [x] Artifacts have retention periods

### Structure Checklist

- [x] Workflows have descriptive names
- [x] Jobs have clear dependencies via `needs`
- [x] Concurrency controls prevent duplicate runs
- [x] `if` conditions used for conditional execution
- [ ] **⚠️ FAIL: Artifact filename mismatch between workflows** - MUST FIX

### Testing Checklist

- [x] Unit tests run on every push/PR
- [x] Integration tests configured (WAF integration)
- [x] E2E tests configured (Playwright)
- [x] Test results uploaded as artifacts
- [ ] **⚠️ WARN: Supply chain verification failing silently due to filename bug**

### Deployment Checklist

- [ ] Environment protection rules configured (needs improvement)
- [ ] Manual approvals for production (needs setup)
- [ ] Rollback strategy documented (partial)

---

## 7. Summary (Revised After Supervisor Review)

The Charon repository demonstrates **solid CI/CD practices** but has **critical issues** discovered during Supervisor review that require immediate attention:

### 🔴 Critical Issues Requiring Immediate Action

| # | Issue | Impact | Workflow |
|---|-------|--------|----------|
| 1 | **Hardcoded encryption key** | Security policy violation, secret exposure risk | `playwright.yml:31` |
| 2 | **Artifact filename mismatch** | Supply chain verification silently failing for ALL PRs | `supply-chain-pr.yml:152` |
| 3 | **GoReleaser `version: latest`** | Non-reproducible builds, supply chain risk | `release-goreleaser.yml:46` |

### 🟠 High Priority Issues

| # | Issue | Impact | Workflow |
|---|-------|--------|----------|
| 4 | **CodeQL v3 vs v4 gap** | Major version mismatch, SARIF compatibility issues | `supply-chain-pr.yml:177` |
| 5 | **Missing environment protection** | No safeguards for production releases | `release-goreleaser.yml` |

### ✅ Strengths Confirmed

- Comprehensive SBOM generation and attestation
- Strong action pinning to SHA (most workflows)
- Proper concurrency controls
- Good test coverage with E2E tests
- Intentional security hardening with `no-cache: true` in Docker builds

### 📊 Revised Health Score

| Category | Original | Revised | Delta |
|----------|----------|---------|-------|
| **Overall** | 87/100 | **78/100** | ↓9 |
| Security | 90/100 | 75/100 | ↓15 |
| Structure | 92/100 | 85/100 | ↓7 |
| Testing | 88/100 | 80/100 | ↓8 |

### Next Steps

1. **IMMEDIATE:** Fix critical issues #1-3 before any new PRs merge
2. **THIS WEEK:** Address high priority issues #4-5
3. **ONGOING:** Work through medium/low priority backlog

---

*Report generated by Planning Agent + Supervisor Review | Last updated: January 15, 2026*
*Supervisor findings marked with 🔍*
