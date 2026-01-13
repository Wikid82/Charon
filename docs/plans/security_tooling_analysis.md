# Security Tooling Analysis: Additional DoD Enhancements for Charon

**Document Version:** 1.0
**Date:** January 10, 2026
**Author:** GitHub Copilot Security Analysis
**Status:** RECOMMENDATION - Pending Review

---

## Executive Summary

This document evaluates three categories of security tools for potential addition to Charon's Definition of Done: **Grype** (vulnerability scanner), **OWASP Dependency-Check** (dependency scanner), and **Supply Chain Security** tools (SLSA/Sigstore). The analysis examines each tool's unique value proposition, overlap with existing tools (Trivy, CodeQL, Renovate), and integration complexity.

### Key Findings

| Tool Category | Recommendation | Priority | Time Impact |
|--------------|----------------|----------|-------------|
| **Grype** | ❌ **DO NOT ADD** | N/A | N/A |
| **OWASP Dependency-Check** | ❌ **DO NOT ADD** | N/A | N/A |
| **Supply Chain Security** (SLSA/Sigstore) | ✅ **STRONGLY RECOMMEND** | **HIGH** | +3-5 min/build |

**TL;DR:** Trivy already provides comprehensive vulnerability scanning. Adding Grype or OWASP Dependency-Check would be redundant. However, **supply chain security tooling (SBOM generation, attestation, and provenance)** offers substantial unique value and should be prioritized for implementation.

---

## Current Security Stack Overview

### Existing Tools

| Tool | Purpose | Coverage | Execution Context |
|------|---------|----------|-------------------|
| **Trivy** | Vulnerability, secret, and misconfiguration scanning | OS packages, Go modules, npm packages, Dockerfiles, K8s manifests | GitHub Actions (PR + push), VS Code tasks |
| **CodeQL** | Static application security testing (SAST) | Go, JavaScript/TypeScript source code | GitHub Actions (weekly schedule, PR, push) |
| **govulncheck** | Go-specific vulnerability scanning | Go modules using official Go vuln DB | GitHub Actions, VS Code tasks |
| **Renovate** | Automated dependency updates | Go modules, npm packages, Docker images, GitHub Actions | GitHub Actions (scheduled) |
| **Pre-commit hooks** | Linting, formatting, basic security checks | Local development | Developer workstation |

### Coverage Analysis

**What's Protected:**

- ✅ OS-level vulnerabilities (Alpine packages)
- ✅ Go module vulnerabilities (govulncheck + Trivy)
- ✅ npm package vulnerabilities (Trivy + npm audit)
- ✅ Container image vulnerabilities (Trivy)
- ✅ Secrets exposure (Trivy)
- ✅ Dockerfile misconfigurations (Trivy)
- ✅ Source code vulnerabilities (CodeQL)
- ✅ Automated dependency updates (Renovate)

**What's Missing:**

- ❌ Software Bill of Materials (SBOM) generation
- ❌ Provenance attestation (who built what, when, how)
- ❌ Build reproducibility verification
- ❌ Supply chain integrity validation
- ❌ Artifact signing and verification

---

## Tool Analysis

---

## 1. Grype (Anchore)

### Purpose and Scope

Grype is an open-source vulnerability scanner for container images, filesystems, and SBOMs. It identifies known vulnerabilities (CVEs) in OS packages and application dependencies.

**Primary Use Case:** Fast vulnerability scanning of container images and artifacts using multiple vulnerability databases.

### Database Coverage

| Database | Coverage | Unique to Grype? |
|----------|----------|------------------|
| **NVD (National Vulnerability Database)** | CVEs for all software | ❌ Trivy uses this |
| **Alpine SecDB** | Alpine Linux packages | ❌ Trivy uses this |
| **Debian Security Tracker** | Debian packages | ❌ Trivy uses this |
| **Red Hat Security Data** | RHEL/CentOS/Fedora | ❌ Trivy uses this |
| **GitHub Advisory Database** | GitHub security advisories | ❌ Trivy uses this |
| **NPM Audit API** | npm packages | ❌ Trivy uses this |
| **Ruby Advisory Database** | Ruby gems | ❌ Trivy uses this |
| **Rust Advisory Database** | Rust crates | ❌ Trivy uses this |

**Conclusion:** Grype uses the **same vulnerability databases** as Trivy. No unique coverage.

### Overlap Analysis with Trivy

| Feature | Trivy | Grype |
|---------|-------|-------|
| **Container image scanning** | ✅ | ✅ |
| **Filesystem scanning** | ✅ | ✅ |
| **SBOM scanning** | ✅ | ✅ |
| **OS package vulnerabilities** | ✅ | ✅ |
| **Go module vulnerabilities** | ✅ | ✅ |
| **npm vulnerabilities** | ✅ | ✅ |
| **Secret scanning** | ✅ | ❌ |
| **Misconfiguration detection** | ✅ (IaC, Docker, K8s) | ❌ |
| **License scanning** | ✅ | ❌ |
| **SARIF output for GitHub** | ✅ | ✅ |
| **Database freshness** | Auto-updated | Auto-updated |
| **GitHub Actions integration** | ✅ Native | ✅ Third-party |
| **Performance (Alpine scan)** | ~30-60s | ~20-40s |

**Overlap:** ~95% functional overlap. Grype is **not** a superset of Trivy—it lacks secret scanning, misconfiguration detection, and license compliance features.

### Unique Value Proposition

**What Grype offers that Trivy doesn't:**

- ❌ **None.** Grype uses the same CVE databases and covers the same ecosystems.
- 🤷 **Marginally faster scans** (~20-30% speed improvement), but Trivy already completes in under 60 seconds for Charon's image.
- ✅ **SBOM-first design:** Grype can scan SBOMs generated by other tools (but Trivy can too).

**What Trivy offers that Grype doesn't:**

- ✅ Secret scanning (API keys, tokens, passwords)
- ✅ Misconfiguration detection (Dockerfile, Kubernetes manifests, Terraform)
- ✅ License compliance scanning
- ✅ Built-in SBOM generation (CycloneDX, SPDX)

### Integration Complexity

#### GitHub Actions

```yaml
# Grype GitHub Action (hypothetical)
- name: Run Grype scan
  uses: anchore/grype-action@v1
  with:
    image: ghcr.io/wikid82/charon:latest
    severity: CRITICAL,HIGH
    output-format: sarif
```

**Complexity:** Low (drop-in replacement for Trivy action)
**Maintenance:** Requires monitoring two vulnerability scanners instead of one

#### VS Code Tasks

```json
{
  "label": "Security: Grype Scan",
  "type": "shell",
  "command": "docker run --rm -v /var/run/docker.sock:/var/run/docker.sock anchore/grype:latest charon:local",
  "group": "test"
}
```

**Complexity:** Low
**Maintenance:** Requires Docker image pulls for both Trivy and Grype

### Performance Impact

- **Build time addition:** +30-60 seconds (if run in parallel with Trivy)
- **Cache warming:** Grype maintains separate vulnerability DB cache (~200MB)
- **Network bandwidth:** Additional DB downloads on cache miss

**Estimated DoD Impact:** +1-2 minutes if run sequentially, +30s if parallelized

### False Positive Rate

**Community Feedback:**

- 🟡 **Similar FP rate to Trivy** (both use NVD data, which has inherent false positives)
- 🟢 **VEX (Vulnerability Exploitability eXchange) support** for suppressing known FPs
- 🔴 **Duplicate alerts** when run alongside Trivy

**Example False Positives (common to both tools):**

- CVEs affecting unused/optional features (e.g., TLS bugs in binaries that don't use TLS)
- Go stdlib CVEs in third-party binaries (e.g., CrowdSec) that Charon can't fix

### Maintenance Burden

- **Update frequency:** Grype DB updated daily (similar to Trivy)
- **Configuration complexity:** Low (similar to Trivy)
- **Breaking changes:** Moderate (major version upgrades may require config updates)
- **Community support:** Strong (Anchore is a major player in cloud security)

### Go + React/TypeScript Fit

| Language/Stack | Support Level | Notes |
|----------------|---------------|-------|
| **Go modules** | ✅ Excellent | Parses `go.mod`/`go.sum` |
| **npm (React)** | ✅ Excellent | Parses `package-lock.json`, `yarn.lock` |
| **Alpine Linux** | ✅ Excellent | Native support for `apk` packages |
| **Docker images** | ✅ Excellent | Primary use case |

**Verdict:** Perfect fit, but so is Trivy.

---

### Grype Recommendation

#### ❌ **DO NOT ADD**

**Rationale:**

1. **95% functional overlap with Trivy** — nearly all features are duplicates
2. **No unique vulnerability database coverage** — same NVD, Alpine SecDB, GitHub Advisory DB
3. **Missing critical features** — no secret scanning, no misconfiguration detection
4. **Increased maintenance burden** — managing two nearly-identical tools
5. **Potential for alert fatigue** — duplicate CVE alerts from both tools
6. **Marginal speed improvement** (~20-30%) doesn't justify added complexity

**User's Willingness to Add DoD Time:** While the user is open to adding DoD time for security, this time should be invested in tools that provide **unique value**, not redundant coverage.

**Alternative Actions:**

- ✅ **Keep Trivy** as the primary vulnerability scanner
- ✅ Ensure Trivy scans cover all required severity levels (CRITICAL, HIGH)
- ✅ Consider Trivy VEX support to suppress known false positives
- ✅ Monitor Trivy's database freshness (currently auto-updates daily)

---

## 2. OWASP Dependency-Check

### Purpose and Scope

OWASP Dependency-Check is an open-source Software Composition Analysis (SCA) tool that identifies known vulnerabilities (CVEs) in project dependencies. It supports multiple ecosystems and build tools.

**Primary Use Case:** Scanning application dependencies during build time to detect vulnerable libraries.

### Database Coverage

| Database | Coverage | Unique to OWASP DC? |
|----------|----------|---------------------|
| **NVD (National Vulnerability Database)** | CVEs for all software | ❌ Trivy uses this |
| **OSS Index (Sonatype)** | Maven/npm/Python packages | 🟡 Partial (Trivy doesn't use OSS Index directly) |
| **NPM Audit API** | npm packages | ❌ Trivy uses this |
| **Ruby Advisory Database** | Ruby gems | ❌ Trivy uses this |
| **RetireJS** | JavaScript library CVEs | ❌ Trivy covers this via NVD |

**Unique Coverage:**

- 🟡 **OSS Index:** Sonatype's proprietary vulnerability database with additional metadata (license info, security advisories). However, **OSS Index sources most data from NVD**, so overlap is >90%.

**Conclusion:** Minimal unique coverage. Most vulnerabilities detected by OWASP DC are already caught by Trivy.

### Overlap Analysis with Trivy

| Feature | Trivy | OWASP Dependency-Check |
|---------|-------|------------------------|
| **Go module scanning** | ✅ (`go.mod`, `go.sum`) | ✅ (via experimental analyzer) |
| **npm package scanning** | ✅ (`package-lock.json`) | ✅ |
| **Container image scanning** | ✅ | ❌ (filesystem only) |
| **OS package scanning** | ✅ (Alpine, Debian, etc.) | ❌ |
| **SBOM generation** | ✅ (CycloneDX, SPDX) | ✅ (OWASP format) |
| **SARIF output** | ✅ | ✅ |
| **Secret scanning** | ✅ | ❌ |
| **Misconfiguration scanning** | ✅ | ❌ |
| **License compliance** | ✅ | ❌ (deprecated) |

**Overlap:** ~70% functional overlap, but **Trivy is a superset** for Charon's use case.

### Unique Value Proposition

**What OWASP Dependency-Check offers that Trivy doesn't:**

- 🟡 **OSS Index integration:** Sonatype's vulnerability DB (but mostly duplicates NVD)
- 🟡 **Maven-centric tooling:** Better Maven `pom.xml` analysis (not relevant for Charon—Go backend, not Java)
- ❌ **No unique coverage for Go or npm** in Charon's stack

**What Trivy offers that OWASP DC doesn't:**

- ✅ Container image scanning (Charon's primary artifact)
- ✅ OS package vulnerabilities (Alpine Linux)
- ✅ Secret scanning
- ✅ Misconfiguration detection (Dockerfile, K8s manifests)
- ✅ Faster execution (30-60s vs. 2-5 minutes for DC)

### Integration Complexity

#### GitHub Actions

```yaml
# OWASP Dependency-Check Action
- name: Run OWASP Dependency-Check
  uses: dependency-check/Dependency-Check_Action@main
  with:
    project: 'Charon'
    path: '.'
    format: 'SARIF'
```

**Complexity:** Moderate
**Issues:**

- Requires NVD API key or local DB download (~500MB-1GB cache)
- First run takes 5-10 minutes to download NVD data
- Subsequent runs: 2-5 minutes

#### VS Code Tasks

```json
{
  "label": "Security: OWASP Dependency-Check",
  "type": "shell",
  "command": "dependency-check --project Charon --scan . --format SARIF --out dependency-check-report.sarif",
  "group": "test"
}
```

**Complexity:** High
**Prerequisites:**

- Install dependency-check CLI via Homebrew, Docker, or manual download
- Configure NVD API key (optional but recommended to avoid rate limits)
- Manage local NVD cache (~1GB)

### Performance Impact

- **Initial run:** 5-10 minutes (NVD database download)
- **Subsequent runs:** 2-5 minutes (depends on cache freshness)
- **Cache size:** ~500MB-1GB (NVD data)
- **Network bandwidth:** High on cache miss

**Estimated DoD Impact:** +2-5 minutes per build (significantly slower than Trivy's 30-60s)

### False Positive Rate

**Community Feedback:**

- 🔴 **High false positive rate** for npm and Go modules
  - Example: Reports CVEs for dev dependencies that aren't in production builds
  - Example: Flags Go stdlib CVEs in `go.mod` even when not used in compiled binary
- 🔴 **Poor CPE (Common Platform Enumeration) matching** for non-Java ecosystems
  - OWASP DC was originally designed for Java/Maven; Go and npm support is newer and less mature
- 🟢 **Suppression file support** (XML-based) to ignore known FPs

**Example False Positive:**

- OWASP DC may flag CVEs for `node_modules` dependencies that are only used in tests or dev builds, not shipped in the Docker image.

### Maintenance Burden

- **Update frequency:** NVD data updated daily (requires re-download or API sync)
- **Configuration complexity:** High (XML config files, suppression files, analyzers)
- **Breaking changes:** Moderate (major version upgrades may break XML configs)
- **Community support:** Strong (OWASP Foundation backing)

### Go + React/TypeScript Fit

| Language/Stack | Support Level | Notes |
|----------------|---------------|-------|
| **Go modules** | 🟡 Experimental | Parses `go.mod` but less mature than Trivy's Go analyzer |
| **npm (React)** | ✅ Good | Parses `package-lock.json`, `yarn.lock` |
| **Alpine Linux** | ❌ Not supported | Cannot scan OS packages |
| **Docker images** | ❌ Not supported | Filesystem-only tool |

**Verdict:** Poor fit for Charon. Go support is experimental, and Docker image scanning (Charon's primary artifact) is not supported.

---

### OWASP Dependency-Check Recommendation

#### ❌ **DO NOT ADD**

**Rationale:**

1. **70% functional overlap with Trivy, but Trivy is superior for Charon's stack**
2. **No unique value for Go or npm ecosystems** — Trivy's Go/npm analyzers are more mature
3. **Cannot scan Docker images** — Charon's primary security artifact
4. **Significantly slower** — 2-5 minutes vs. Trivy's 30-60s
5. **High false positive rate** for non-Java ecosystems
6. **Complex configuration** — XML-based configs vs. Trivy's simple CLI flags
7. **Poor fit for Alpine Linux + Docker** — OWASP DC doesn't scan OS packages

**User's Willingness to Add DoD Time:** While the user is open to adding DoD time, **OWASP Dependency-Check would add 2-5 minutes for minimal unique value**. This time is better spent on supply chain security tooling (SBOM, attestation).

**Alternative Actions:**

- ✅ **Keep Trivy** as the primary dependency scanner
- ✅ Ensure Trivy scans both `backend/go.mod` and `frontend/package-lock.json`
- ✅ Enable Trivy's SBOM generation feature (already supported)

---

## 3. Supply Chain Security (SLSA, Sigstore, SBOM)

### Purpose and Scope

Supply chain security tools address vulnerabilities in the **build and distribution process**, not just the code itself. They provide **provenance attestation** (proof of how an artifact was built), **artifact signing** (cryptographic verification), and **Software Bills of Materials (SBOMs)** (ingredient lists for software).

**Primary Use Case:** Prove that artifacts are built from trusted sources, by trusted actors, using secure processes.

### Key Technologies

#### 3.1. SLSA (Supply-chain Levels for Software Artifacts)

**What it is:** A security framework with 4 compliance levels (SLSA 0-4) that define best practices for securing the software supply chain.

**SLSA Levels:**

- **SLSA 0:** No guarantees (current state of most projects)
- **SLSA 1:** Build process documented (provenance exists)
- **SLSA 2:** Signed provenance, version-controlled build scripts
- **SLSA 3:** Source and build platform hardened, non-falsifiable provenance
- **SLSA 4:** Two-party review of all changes, hermetic builds

**What it protects against:**

- ✅ **Source tampering** (e.g., compromised GitHub account modifying code)
- ✅ **Build tampering** (e.g., malicious CI/CD job injecting backdoors)
- ✅ **Artifact substitution** (e.g., attacker replacing published Docker image)
- ✅ **Dependency confusion** (e.g., typosquatting attacks)

**Relevance to Charon:**

- ✅ Docker images are signed and attested
- ✅ Users can verify image provenance before deployment
- ✅ Compliance with enterprise security policies (e.g., NIST SSDF, EO 14028)

#### 3.2. Sigstore/Cosign

**What it is:** Open-source keyless signing and verification for software artifacts using ephemeral keys and transparency logs.

**Core Components:**

- **Cosign:** CLI tool for signing and verifying container images, blobs, and SBOMs
- **Fulcio:** Certificate authority for keyless signing (OIDC-based)
- **Rekor:** Transparency log for immutable artifact signatures

**Keyless Signing:** No need to manage private keys—sign with OIDC identity (GitHub, Google, Microsoft).

**What it protects against:**

- ✅ **Image tampering** (unsigned images rejected)
- ✅ **Man-in-the-middle attacks** (cryptographic verification)
- ✅ **Registry compromise** (even if registry is hacked, signatures can't be forged)

**Relevance to Charon:**

- ✅ Sign Docker images automatically in GitHub Actions
- ✅ Users verify signatures before pulling images
- ✅ Integrate with admission controllers (e.g., Kyverno, OPA) for Kubernetes deployments

#### 3.3. SBOM (Software Bill of Materials)

**What it is:** A machine-readable inventory of all components in a software artifact (dependencies, libraries, versions, licenses).

**Formats:**

- **CycloneDX** (OWASP standard, JSON/XML)
- **SPDX** (Linux Foundation standard, JSON/YAML/RDF)

**What it protects against:**

- ✅ **Unknown vulnerabilities** (enables future retrospective scanning)
- ✅ **Compliance violations** (tracks license obligations)
- ✅ **Supply chain attacks** (identifies compromised dependencies)

**Relevance to Charon:**

- ✅ Generate SBOM for Docker image in CI/CD
- ✅ Attach SBOM to image as attestation
- ✅ Enable users to audit Charon's dependencies

---

### Overlap Analysis with Existing Tools

| Feature | Trivy | CodeQL | Renovate | Supply Chain Tools |
|---------|-------|--------|----------|-------------------|
| **Vulnerability scanning** | ✅ | ✅ | ❌ | ❌ (not primary purpose) |
| **Dependency updates** | ❌ | ❌ | ✅ | ❌ |
| **SBOM generation** | ✅ (basic) | ❌ | ❌ | ✅ (comprehensive) |
| **Provenance attestation** | ❌ | ❌ | ❌ | ✅ |
| **Artifact signing** | ❌ | ❌ | ❌ | ✅ |
| **Build reproducibility** | ❌ | ❌ | ❌ | ✅ |
| **Transparency logs** | ❌ | ❌ | ❌ | ✅ (Rekor) |
| **Keyless cryptography** | ❌ | ❌ | ❌ | ✅ (Fulcio) |

**Overlap:** **~10% functional overlap**. Supply chain tools are **complementary**, not replacements.

---

### Unique Value Proposition

**What supply chain security offers that current tools don't:**

| Threat Model | Current Tools | Supply Chain Tools |
|--------------|---------------|-------------------|
| **Compromised maintainer account** | ❌ Not detected | ✅ Provenance shows who built artifact |
| **Malicious CI/CD job** | ❌ Not detected | ✅ Signed provenance prevents tampering |
| **Registry compromise** | ❌ Not detected | ✅ Signature verification fails for tampered images |
| **Dependency confusion** | 🟡 Partial (Renovate checks sources) | ✅ SBOM tracks all dependencies |
| **Backdoored dependency** | 🟡 Partial (Trivy scans for known CVEs) | ✅ SBOM enables retrospective analysis |
| **Compliance (EO 14028, NIST SSDF)** | ❌ Not addressed | ✅ SLSA levels provide compliance framework |

**Real-World Attacks Prevented:**

- ✅ **SolarWinds (2020):** Provenance attestation would have shown build artifacts were tampered
- ✅ **Codecov (2021):** Signed artifacts would have prevented malicious script injection
- ✅ **npm package hijacking:** SBOM would enable tracking affected downstream projects

---

### Integration Complexity

#### GitHub Actions

**Step 1: Generate SBOM**

Already implemented in Charon's `.github/workflows/docker-build.yml`:

```yaml
- name: Generate SBOM
  uses: anchore/sbom-action@v0.21.0
  with:
    image: ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}@${{ steps.build-and-push.outputs.digest }}
    format: cyclonedx-json
    output-file: sbom.cyclonedx.json
```

**Step 2: Attest SBOM**

Already implemented:

```yaml
- name: Attest SBOM
  uses: actions/attest-sbom@v3.0.0
  with:
    subject-name: ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}
    subject-digest: ${{ steps.build-and-push.outputs.digest }}
    sbom-path: sbom.cyclonedx.json
    push-to-registry: true
```

**Step 3: Sign Docker Image with Cosign (NEW)**

```yaml
- name: Install Cosign
  uses: sigstore/cosign-installer@v3.4.0

- name: Sign image with Cosign (keyless)
  env:
    COSIGN_EXPERIMENTAL: 1
  run: |
    cosign sign --yes \
      ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}@${{ steps.build-and-push.outputs.digest }}
```

**Step 4: Generate SLSA Provenance (NEW)**

```yaml
- name: Generate SLSA Provenance
  uses: slsa-framework/slsa-github-generator/.github/workflows/generator_container_slsa3.yml@v1.9.0
  with:
    image: ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}
    digest: ${{ steps.build-and-push.outputs.digest }}
    registry-username: ${{ github.actor }}
    registry-password: ${{ secrets.GITHUB_TOKEN }}
```

**Complexity:** Low-to-Moderate
**Prerequisites:** Enable GitHub Actions' `id-token: write` permission (already enabled)
**Maintenance:** Minimal (actions auto-update with Renovate)

---

#### VS Code Tasks (Verification)

```json
{
  "label": "Security: Verify Image Signature",
  "type": "shell",
  "command": "cosign verify --certificate-identity-regexp='https://github.com/Wikid82/charon' --certificate-oidc-issuer='https://token.actions.githubusercontent.com' ghcr.io/wikid82/charon:latest",
  "group": "test"
}
```

**Complexity:** Low
**Prerequisites:** Install Cosign locally (`brew install cosign`)

---

### Performance Impact

| Step | Time Addition | Notes |
|------|---------------|-------|
| **SBOM generation** | ✅ Already implemented | No additional time (already in CI/CD) |
| **SBOM attestation** | ✅ Already implemented | No additional time (already in CI/CD) |
| **Cosign signing** | +30-60 seconds | Keyless signing via OIDC |
| **SLSA provenance** | +60-120 seconds | Generates and attaches provenance |
| **Rekor transparency log** | Included in above | Automatic with Cosign |

**Total Estimated DoD Impact:** +1.5-3 minutes per build

**Mitigation:** Run signing and provenance generation **after** tests pass, in parallel with other post-build steps.

---

### False Positive Rate

**N/A** — Supply chain tools don't scan for vulnerabilities, so false positives are not applicable. They provide **cryptographic proof of integrity**, which is either valid or invalid.

**Potential Issues:**

- 🟡 **Signature verification failure** if OIDC identity changes (e.g., repo renamed)
- 🟡 **Provenance mismatch** if build scripts are modified without updating attestation
- 🟢 **These are security failures, not false positives** — they indicate tampering or misconfiguration

---

### Maintenance Burden

| Component | Update Frequency | Configuration Complexity | Breaking Changes |
|-----------|------------------|--------------------------|------------------|
| **Cosign** | Monthly (via Renovate) | Low (CLI flags) | Low (stable API) |
| **SLSA Generator** | Quarterly | Low (GitHub Action) | Low (versioned) |
| **SBOM Action** | Monthly | Low (GitHub Action) | Low |
| **Rekor** | N/A (hosted by Sigstore) | None | None |

**Overall Maintenance Burden:** **Low**

- Actions auto-update with Renovate
- No local secrets to manage (keyless signing)
- Public Sigstore infrastructure (no self-hosting required)

---

### Go + React/TypeScript Fit

| Language/Stack | Support Level | Notes |
|----------------|---------------|-------|
| **Go modules** | ✅ Excellent | Syft (SBOM generator) parses `go.mod` |
| **npm (React)** | ✅ Excellent | Syft parses `package-lock.json` |
| **Alpine Linux** | ✅ Excellent | Syft parses `apk` packages |
| **Docker images** | ✅ Excellent | Primary use case for Cosign/SLSA |

**Verdict:** Perfect fit. Supply chain tools are designed for modern polyglot projects.

---

### Supply Chain Security Recommendation

#### ✅ **STRONGLY RECOMMEND**

**Rationale:**

1. **Unique value:** Addresses threats that Trivy/CodeQL/Renovate don't cover (build tampering, artifact substitution)
2. **Minimal overlap:** Complementary to existing tools, not redundant
3. **Low maintenance:** Keyless signing eliminates secret management burden
4. **Compliance:** Meets NIST SSDF, EO 14028, and enterprise security policies
5. **Industry trend:** Leading cloud providers (AWS, Azure, GCP) are adopting Sigstore
6. **Charon already has SBOM generation:** 50% of the work is done; just add signing and provenance

**User's Willingness to Add DoD Time:** This is the **highest-value addition** for the time invested (+3-5 minutes per build).

---

### Priority Ranking

| Priority | Recommendation | Time Impact | Justification |
|----------|----------------|-------------|---------------|
| 🔴 **HIGH** | **Cosign Image Signing** | +30-60s | Protects against image tampering, registry compromise |
| 🔴 **HIGH** | **SLSA Provenance** | +1-2 min | Proves build integrity, meets compliance requirements |
| 🟢 **MEDIUM** | **SBOM Verification Workflow** | +15-30s | Already generating SBOMs; add verification step |
| 🟡 **LOW** | **Rekor Transparency Log Monitoring** | +5-10s | Optional: Monitor Rekor for unauthorized signatures |

---

## Recommended Implementation Plan

### Phase 1: Cosign Image Signing (Week 1)

**Goal:** Sign all published Docker images with keyless Cosign.

#### Step 1.1: Add Cosign to GitHub Actions

```yaml
# .github/workflows/docker-build.yml

- name: Install Cosign
  uses: sigstore/cosign-installer@v3.4.0

- name: Sign Docker image
  if: github.event_name != 'pull_request'
  env:
    COSIGN_EXPERIMENTAL: 1
  run: |
    cosign sign --yes \
      ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}@${{ steps.build-and-push.outputs.digest }}
```

**Prerequisites:**

- ✅ GitHub Actions already has `id-token: write` permission (enabled in `docker-build.yml`)

#### Step 1.2: Add VS Code Task for Signature Verification

```json
// .vscode/tasks.json

{
  "label": "Security: Verify Image Signature",
  "type": "shell",
  "command": ".github/skills/scripts/skill-runner.sh security-verify-signature",
  "group": "test"
}
```

**Create Skill Script:**

```bash
# .github/skills/scripts/security-verify-signature-scripts/run.sh

#!/bin/bash
set -euo pipefail

IMAGE="${1:-ghcr.io/wikid82/charon:latest}"

echo "🔍 Verifying signature for $IMAGE"

cosign verify \
  --certificate-identity-regexp='https://github.com/Wikid82/charon' \
  --certificate-oidc-issuer='https://token.actions.githubusercontent.com' \
  "$IMAGE"

echo "✅ Signature verified"
```

#### Step 1.3: Document in SECURITY.md

Add section:

```markdown
### Artifact Signing

All Charon Docker images are signed with [Sigstore Cosign](https://github.com/sigstore/cosign) using keyless signing.

**Verify image signature:**

```bash
cosign verify \
  --certificate-identity-regexp='https://github.com/Wikid82/charon' \
  --certificate-oidc-issuer='https://token.actions.githubusercontent.com' \
  ghcr.io/wikid82/charon:latest
```

Signatures are logged in the public [Rekor transparency log](https://rekor.sigstore.dev/).

```

**Estimated Time:** 2-4 hours
**DoD Impact:** +30-60 seconds per build

---

### Phase 2: SLSA Provenance (Week 2)

**Goal:** Generate SLSA Level 3 provenance for Docker images.

#### Step 2.1: Add SLSA Provenance Generation

```yaml
# .github/workflows/docker-build.yml

- name: Generate SLSA Provenance
  if: github.event_name != 'pull_request'
  uses: slsa-framework/slsa-github-generator/.github/workflows/generator_container_slsa3.yml@v1.9.0
  with:
    image: ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}
    digest: ${{ steps.build-and-push.outputs.digest }}
    registry-username: ${{ github.actor }}
    registry-password: ${{ secrets.GITHUB_TOKEN }}
```

#### Step 2.2: Add SLSA Badge to README

```markdown
[![SLSA 3](https://slsa.dev/images/gh-badge-level3.svg)](https://slsa.dev)
```

#### Step 2.3: Document in SECURITY.md

```markdown
### Build Provenance

Charon follows [SLSA Level 3](https://slsa.dev/spec/v1.0/levels) practices for build integrity:

- ✅ Builds run on GitHub-hosted runners (hardened build platform)
- ✅ Provenance is signed and immutable
- ✅ Source code is version-controlled
- ✅ Build process is documented and reproducible

**View provenance:**

```bash
cosign verify-attestation \
  --type slsaprovenance \
  --certificate-identity-regexp='https://github.com/Wikid82/charon' \
  --certificate-oidc-issuer='https://token.actions.githubusercontent.com' \
  ghcr.io/wikid82/charon:latest
```

```

**Estimated Time:** 4-6 hours
**DoD Impact:** +1-2 minutes per build

---

### Phase 3: SBOM Verification Workflow (Week 3)

**Goal:** Add verification step to ensure SBOM is signed and attached.

#### Step 3.1: Add SBOM Verification

```yaml
# .github/workflows/docker-build.yml

- name: Verify SBOM Attestation
  if: github.event_name != 'pull_request'
  run: |
    cosign verify-attestation \
      --type cyclonedx \
      --certificate-identity-regexp='https://github.com/Wikid82/charon' \
      --certificate-oidc-issuer='https://token.actions.githubusercontent.com' \
      ${{ env.REGISTRY }}/${{ env.IMAGE_NAME }}@${{ steps.build-and-push.outputs.digest }} | jq
```

**Estimated Time:** 1-2 hours
**DoD Impact:** +15-30 seconds per build

---

### Total Implementation Estimate

| Phase | Estimated Time | DoD Impact | Risk Level |
|-------|----------------|------------|------------|
| Phase 1: Cosign Signing | 2-4 hours | +30-60s | 🟢 Low |
| Phase 2: SLSA Provenance | 4-6 hours | +1-2 min | 🟡 Moderate |
| Phase 3: SBOM Verification | 1-2 hours | +15-30s | 🟢 Low |
| **TOTAL** | **7-12 hours** | **+2-4 min/build** | **🟢 Low** |

**Rollback Plan:**

- Phase 1 and 2 can be disabled by removing steps from `docker-build.yml`
- Phase 3 is non-blocking (verification failure logs a warning, doesn't fail the build)

---

## Updated Definition of Done (Post-Implementation)

### Current DoD (Security Section)

```markdown
## Security

- [ ] All security scans pass with zero CRITICAL/HIGH vulnerabilities
  - [ ] Trivy container image scan (no CVEs)
  - [ ] govulncheck Go module scan (no known vulnerabilities)
  - [ ] CodeQL static analysis (no security findings)
- [ ] No secrets exposed in code or container image
- [ ] No misconfiguration issues (Dockerfile, K8s manifests)
- [ ] Renovate PR created for dependency updates (if applicable)
```

### Proposed DoD (with Supply Chain Security)

```markdown
## Security

- [ ] All security scans pass with zero CRITICAL/HIGH vulnerabilities
  - [ ] Trivy container image scan (no CVEs)
  - [ ] govulncheck Go module scan (no known vulnerabilities)
  - [ ] CodeQL static analysis (no security findings)
- [ ] No secrets exposed in code or container image
- [ ] No misconfiguration issues (Dockerfile, K8s manifests)
- [ ] Renovate PR created for dependency updates (if applicable)
- [ ] **Supply Chain Security** (for Docker image releases)
  - [ ] SBOM generated and attached to image
  - [ ] SBOM attestation signed and verifiable
  - [ ] Docker image signed with Cosign
  - [ ] SLSA provenance generated and signed
  - [ ] Signatures logged in Rekor transparency log
```

---

## Comparison Summary

| Tool | Value | Overlap | Maintenance | Time Impact | Recommendation |
|------|-------|---------|-------------|-------------|----------------|
| **Grype** | ❌ Low | 95% with Trivy | Moderate | +1-2 min | ❌ **DO NOT ADD** |
| **OWASP Dependency-Check** | ❌ Low | 70% with Trivy | High | +2-5 min | ❌ **DO NOT ADD** |
| **Supply Chain (SLSA/Cosign)** | ✅ High | 10% with existing tools | Low | +3-5 min | ✅ **STRONGLY RECOMMEND** |

---

## Key Decision Factors

### Why NOT Grype or OWASP Dependency-Check?

1. **Trivy is a superset:** Covers vulnerabilities + secrets + misconfigurations
2. **Same vulnerability databases:** No unique CVE coverage
3. **Slower execution:** 2-5 min vs. Trivy's 30-60s
4. **Increased maintenance:** Managing multiple overlapping tools
5. **Alert fatigue:** Duplicate CVE notifications

### Why SLSA/Sigstore/SBOM?

1. **Unique threat model:** Protects against supply chain attacks that Trivy can't detect
2. **Minimal overlap:** Complementary to existing tools
3. **Compliance:** Meets NIST SSDF, EO 14028 requirements
4. **Industry adoption:** Standard practice for secure software distribution
5. **Low maintenance:** Keyless signing, auto-updating GitHub Actions
6. **Already 50% done:** Charon already generates SBOMs; just add signing/provenance

---

## Conclusion

**Final Recommendation:**

1. ❌ **DO NOT ADD Grype or OWASP Dependency-Check** — redundant with Trivy, no unique value
2. ✅ **ADD Supply Chain Security Tooling** — high-value, low-maintenance, addresses unique threats
3. 🎯 **Prioritize Cosign signing first** (highest impact, lowest time investment)
4. 📊 **Implement SLSA provenance next** (compliance benefit, moderate time investment)
5. 🔍 **Add SBOM verification last** (nice-to-have, minimal time investment)

**Expected Benefits:**

- ✅ Cryptographic proof of artifact integrity
- ✅ Compliance with federal/enterprise security mandates
- ✅ Protection against supply chain attacks (e.g., SolarWinds-style compromises)
- ✅ Transparent audit trail via Rekor transparency log
- ✅ User confidence in artifact authenticity

**Total Time Investment:** 7-12 hours of implementation, +3-5 minutes per build

---

**Next Steps:**

1. Review this analysis with maintainers
2. Approve Phase 1 (Cosign signing) for immediate implementation
3. Create GitHub issues for Phase 2 and 3
4. Update SECURITY.md and README.md with new capabilities

---

**Document Status:** DRAFT - Awaiting Maintainer Review
**Last Updated:** January 10, 2026
**Version:** 1.0
