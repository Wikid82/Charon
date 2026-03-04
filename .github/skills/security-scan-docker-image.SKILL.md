---
# agentskills.io specification v1.0
name: "security-scan-docker-image"
version: "1.0.0"
description: "Build Docker image and scan with Grype/Syft matching CI supply chain verification"
author: "Charon Project"
license: "MIT"
tags:
  - "security"
  - "scanning"
  - "docker"
  - "supply-chain"
  - "vulnerabilities"
  - "sbom"
compatibility:
  os:
    - "linux"
    - "darwin"
  shells:
    - "bash"
requirements:
  - name: "docker"
    version: ">=24.0"
    optional: false
  - name: "syft"
    version: ">=1.17.0"
    optional: false
    install_url: "https://github.com/anchore/syft"
  - name: "grype"
    version: ">=0.85.0"
    optional: false
    install_url: "https://github.com/anchore/grype"
  - name: "jq"
    version: ">=1.6"
    optional: false
environment_variables:
  - name: "SYFT_VERSION"
    description: "Syft version to use for SBOM generation"
    default: "v1.17.0"
    required: false
  - name: "GRYPE_VERSION"
    description: "Grype version to use for vulnerability scanning"
    default: "v0.107.0"
    required: false
  - name: "IMAGE_TAG"
    description: "Docker image tag to build and scan"
    default: "charon:local"
    required: false
  - name: "FAIL_ON_SEVERITY"
    description: "Comma-separated list of severities that cause failure"
    default: "Critical,High"
    required: false
parameters:
  - name: "image_tag"
    type: "string"
    description: "Docker image tag to build and scan"
    default: "charon:local"
    required: false
  - name: "no_cache"
    type: "boolean"
    description: "Build Docker image without cache"
    default: false
    required: false
outputs:
  - name: "sbom_file"
    type: "file"
    description: "Generated SBOM in CycloneDX JSON format"
  - name: "scan_results"
    type: "file"
    description: "Grype vulnerability scan results in JSON format"
  - name: "exit_code"
    type: "number"
    description: "0 if no critical/high issues, 1 if issues found, 2 if build/scan failed"
metadata:
  category: "security"
  subcategory: "supply-chain"
  execution_time: "long"
  risk_level: "low"
  ci_cd_safe: true
  requires_network: true
  idempotent: false
exit_codes:
  0: "Scan successful, no critical or high vulnerabilities"
  1: "Critical or high severity vulnerabilities found"
  2: "Build failed or scan error"
---

# Security: Scan Docker Image (Local)

## Overview

**CRITICAL GAP ADDRESSED**: This skill closes a critical security gap discovered in the Charon project's local development workflow. While the existing Trivy filesystem scanner catches some issues, it misses vulnerabilities that only exist in the actual built Docker image, including:

- **Alpine package vulnerabilities** in the base image
- **Compiled binary vulnerabilities** in Go dependencies
- **Embedded dependencies** that only exist post-build
- **Multi-stage build artifacts** not present in source
- **Runtime dependencies** added during Docker build

This skill replicates the **exact CI supply chain verification process** used in the `supply-chain-pr.yml` workflow, ensuring local scans match CI scans precisely. This prevents the "works locally but fails in CI" scenario and catches image-only vulnerabilities before they reach production.

## Key Differences from Trivy Filesystem Scan

| Aspect | Trivy (Filesystem) | This Skill (Image Scan) |
|--------|-------------------|------------------------|
| **Scan Target** | Source code + dependencies | Built Docker image |
| **Alpine Packages** | ❌ Not detected | ✅ Detected |
| **Compiled Binaries** | ❌ Not detected | ✅ Detected |
| **Build Artifacts** | ❌ Not detected | ✅ Detected |
| **CI Alignment** | ⚠️ Different results | ✅ Exact match |
| **Supply Chain** | Partial coverage | Full coverage |

## Features

- **Exact CI Matching**: Uses same Syft and Grype versions as supply-chain-pr.yml
- **Image-Based Scanning**: Scans the actual Docker image, not just filesystem
- **SBOM Generation**: Creates CycloneDX JSON SBOM from the built image
- **Severity-Based Failures**: Fails on Critical/High severity by default
- **Detailed Reporting**: Counts vulnerabilities by severity
- **Build Integration**: Builds the Docker image first, ensuring latest code
- **Idempotent Scans**: Can be run repeatedly with consistent results

## Prerequisites

- Docker 24.0 or higher installed and running
- Syft 1.17.0 or higher (auto-checked, installation instructions provided)
- Grype 0.85.0 or higher (auto-checked, installation instructions provided)
- jq 1.6 or higher (for JSON processing)
- Internet connection (for vulnerability database updates)
- Sufficient disk space for Docker image build (~2GB recommended)

## Installation

### Install Syft

```bash
# Linux/macOS
curl -sSfL https://raw.githubusercontent.com/anchore/syft/main/install.sh | sh -s -- -b /usr/local/bin v1.17.0

# Or via package manager
brew install syft  # macOS
```

### Install Grype

```bash
# Linux/macOS
curl -sSfL https://raw.githubusercontent.com/anchore/grype/main/install.sh | sh -s -- -b /usr/local/bin v0.107.0

# Or via package manager
brew install grype  # macOS
```

### Verify Installation

```bash
syft version
grype version
```

## Usage

### Basic Usage (Default Image Tag)

Build and scan the default `charon:local` image:

```bash
cd /path/to/charon
.github/skills/scripts/skill-runner.sh security-scan-docker-image
```

### Custom Image Tag

Build and scan a custom-tagged image:

```bash
.github/skills/scripts/skill-runner.sh security-scan-docker-image charon:test
```

### No-Cache Build

Force a clean build without Docker cache:

```bash
.github/skills/scripts/skill-runner.sh security-scan-docker-image charon:local no-cache
```

### Environment Variable Overrides

Override default versions or behavior:

```bash
# Use specific tool versions
SYFT_VERSION=v1.17.0 GRYPE_VERSION=v0.107.0 \
  .github/skills/scripts/skill-runner.sh security-scan-docker-image

# Change failure threshold
FAIL_ON_SEVERITY="Critical" \
  .github/skills/scripts/skill-runner.sh security-scan-docker-image
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| image_tag | string | No | charon:local | Docker image tag to build and scan |
| no_cache | boolean | No | false | Build without Docker cache (pass "no-cache" as second arg) |

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| SYFT_VERSION | No | v1.17.0 | Syft version (matches CI) |
| GRYPE_VERSION | No | v0.107.0 | Grype version (matches CI) |
| IMAGE_TAG | No | charon:local | Default image tag if not provided |
| FAIL_ON_SEVERITY | No | Critical,High | Severities that cause exit code 1 |

## Outputs

### Generated Files

- **`sbom.cyclonedx.json`**: SBOM in CycloneDX JSON format (industry standard)
- **`grype-results.json`**: Detailed vulnerability scan results
- **`grype-results.sarif`**: SARIF format for GitHub Security integration

### Exit Codes

- **0**: Scan completed successfully, no critical/high vulnerabilities
- **1**: Critical or high severity vulnerabilities found (blocking)
- **2**: Docker build failed or scan error

### Output Format

```
[INFO] Building Docker image: charon:local...
[BUILD] Using Dockerfile with multi-stage build
[BUILD] Image built successfully: charon:local

[SBOM] Generating SBOM using Syft v1.17.0...
[SBOM] Generated SBOM contains 247 packages

[SCAN] Scanning for vulnerabilities using Grype v0.107.0...
[SCAN] Vulnerability Summary:
  🔴 Critical: 0
  🟠 High: 0
  🟡 Medium: 15
  🟢 Low: 42
  📊 Total: 57

[SUCCESS] Docker image scan complete - no critical or high vulnerabilities
```

## Examples

### Example 1: Standard Local Scan

```bash
$ .github/skills/scripts/skill-runner.sh security-scan-docker-image
[INFO] Building Docker image: charon:local...
[BUILD] Step 1/25 : FROM node:24.13.0-alpine AS frontend-builder
[BUILD] ...
[BUILD] Successfully built abc123def456
[BUILD] Successfully tagged charon:local

[SBOM] Generating SBOM using Syft v1.17.0...
[SBOM] Scanning image: charon:local
[SBOM] Generated SBOM contains 247 packages

[SCAN] Scanning for vulnerabilities using Grype v0.107.0...
[SCAN] Vulnerability Summary:
  🔴 Critical: 0
  🟠 High: 2
  🟡 Medium: 15
  🟢 Low: 42
  📊 Total: 59

[SCAN] High Severity Vulnerabilities:
  - CVE-2024-12345 in alpine-baselayout (CVSS: 7.5)
    Package: alpine-baselayout@3.23.0
    Fixed: alpine-baselayout@3.23.1
    Description: Arbitrary file read vulnerability

  - CVE-2024-67890 in busybox (CVSS: 8.2)
    Package: busybox@1.36.1
    Fixed: busybox@1.36.2
    Description: Remote code execution via crafted input

[ERROR] Found 2 High severity vulnerabilities - please review and remediate
Exit code: 1
```

### Example 2: Clean Build After Code Changes

```bash
$ .github/skills/scripts/skill-runner.sh security-scan-docker-image charon:test no-cache
[INFO] Building Docker image: charon:test (no cache)...
[BUILD] Building without cache to ensure fresh dependencies...
[BUILD] Successfully built and tagged charon:test

[SBOM] Generating SBOM...
[SBOM] Generated SBOM contains 248 packages (+1 from previous scan)

[SCAN] Scanning for vulnerabilities...
[SCAN] Vulnerability Summary:
  🔴 Critical: 0
  🟠 High: 0
  🟡 Medium: 16
  🟢 Low: 43
  📊 Total: 59

[SUCCESS] Docker image scan complete - no critical or high vulnerabilities
Exit code: 0
```

### Example 3: CI/CD Pipeline Integration

```yaml
# .github/workflows/local-verify.yml (example)
- name: Scan Docker Image Locally
  run: .github/skills/scripts/skill-runner.sh security-scan-docker-image
  continue-on-error: false

- name: Upload SBOM Artifact
  uses: actions/upload-artifact@v4
  with:
    name: local-sbom
    path: sbom.cyclonedx.json
```

### Example 4: Pre-Commit Hook Integration

```bash
# .git/hooks/pre-push
#!/bin/bash
echo "Running local Docker image security scan..."
if ! .github/skills/scripts/skill-runner.sh security-scan-docker-image; then
    echo "❌ Security scan failed - please fix vulnerabilities before pushing"
    exit 1
fi
```

## How It Works

### Build Phase

1. **Docker Build**: Builds the Docker image using the project's Dockerfile
   - Uses multi-stage build for frontend and backend
   - Applies build args: VERSION, BUILD_DATE, VCS_REF
   - Tags with specified image tag (default: charon:local)

### SBOM Generation Phase

2. **Image Analysis**: Syft analyzes the built Docker image (not filesystem)
   - Scans all layers in the final image
   - Detects Alpine packages, Go modules, npm packages
   - Identifies compiled binaries and their dependencies
   - Catalogs runtime dependencies added during build

3. **SBOM Creation**: Generates CycloneDX JSON SBOM
   - Industry-standard format for supply chain visibility
   - Contains full package inventory with versions
   - Includes checksums and license information

### Vulnerability Scanning Phase

4. **Database Update**: Grype updates its vulnerability database
   - Fetches latest CVE information
   - Ensures scan uses current vulnerability data

5. **Image Scan**: Grype scans the SBOM against vulnerability database
   - Matches packages against known CVEs
   - Calculates CVSS scores for each vulnerability
   - Generates SARIF output for GitHub Security

6. **Severity Analysis**: Counts vulnerabilities by severity
   - Critical: CVSS 9.0-10.0
   - High: CVSS 7.0-8.9
   - Medium: CVSS 4.0-6.9
   - Low: CVSS 0.1-3.9

### Reporting Phase

7. **Results Summary**: Displays vulnerability counts and details
8. **Exit Code**: Returns appropriate exit code based on severity findings

## Vulnerability Severity Thresholds

**Project Standards (Matches CI)**:

| Severity | CVSS Range | Action | Exit Code |
|----------|-----------|--------|-----------|
| 🔴 **CRITICAL** | 9.0-10.0 | **MUST FIX** - Blocks commit/push | 1 |
| 🟠 **HIGH** | 7.0-8.9 | **SHOULD FIX** - Blocks commit/push | 1 |
| 🟡 **MEDIUM** | 4.0-6.9 | Fix in next release (logged) | 0 |
| 🟢 **LOW** | 0.1-3.9 | Optional, fix as time permits | 0 |

## Error Handling

### Common Issues

**Docker not running**:
```bash
[ERROR] Docker daemon is not running
Solution: Start Docker Desktop or Docker service
```

**Syft not installed**:
```bash
[ERROR] Syft not found - install from: https://github.com/anchore/syft
Solution: Install Syft v1.17.0 using installation instructions above
```

**Grype not installed**:
```bash
[ERROR] Grype not found - install from: https://github.com/anchore/grype
Solution: Install Grype v0.107.0 using installation instructions above
```

**Build failure**:
```bash
[ERROR] Docker build failed with exit code 1
Solution: Check Dockerfile syntax and dependency availability
```

**Network timeout (vulnerability scan)**:
```bash
[WARNING] Failed to update Grype vulnerability database
Solution: Check internet connection or retry later
```

**Disk space insufficient**:
```bash
[ERROR] No space left on device
Solution: Clean up Docker images and containers: docker system prune -a
```

## Integration with Definition of Done

This skill is **MANDATORY** in the Management agent's Definition of Done checklist:

### When to Run

- ✅ **Before every commit** that changes application code
- ✅ **After dependency updates** (Go modules, npm packages)
- ✅ **Before creating a Pull Request**
- ✅ **After Dockerfile modifications**
- ✅ **Before release/tag creation**

### QA_Security Requirements

The QA_Security agent **MUST**:

1. Run this skill after running Trivy filesystem scan
2. Verify that both scans pass with zero Critical/High issues
3. Document any differences between filesystem and image scans
4. Block approval if image scan reveals additional vulnerabilities
5. Report findings in the QA report at `docs/reports/qa_report.md`

### Why This is Critical

**Image-only vulnerabilities** can exist even when filesystem scans pass:

- Alpine base image CVEs (e.g., musl, busybox, apk-tools)
- Compiled Go binary vulnerabilities (e.g., stdlib CVEs)
- Caddy plugin vulnerabilities added during build
- Multi-stage build artifacts with known issues

**Without this scan**, these vulnerabilities reach production undetected.

## Comparison with CI Supply Chain Workflow

This skill **exactly replicates** the supply-chain-pr.yml workflow:

| Step | CI Workflow | This Skill | Match |
|------|------------|------------|-------|
| Build Image | ✅ Docker build | ✅ Docker build | ✅ |
| Load Image | ✅ Load from artifact | ✅ Use built image | ✅ |
| Syft Version | v1.17.0 | v1.17.0 | ✅ |
| Grype Version | v0.107.0 | v0.107.0 | ✅ |
| SBOM Format | CycloneDX JSON | CycloneDX JSON | ✅ |
| Scan Target | Docker image | Docker image | ✅ |
| Severity Counts | Critical/High/Medium/Low | Critical/High/Medium/Low | ✅ |
| Exit on Critical/High | Yes | Yes | ✅ |
| SARIF Output | Yes | Yes | ✅ |

**Guarantee**: If this skill passes locally, the CI supply chain workflow will pass (assuming same code/dependencies).

## Related Skills

- [security-scan-trivy](./security-scan-trivy.SKILL.md) - Filesystem vulnerability scan (complementary)
- [security-verify-sbom](./security-verify-sbom.SKILL.md) - SBOM verification and comparison
- [security-sign-cosign](./security-sign-cosign.SKILL.md) - Sign artifacts with Cosign
- [security-slsa-provenance](./security-slsa-provenance.SKILL.md) - Generate SLSA provenance

## Workflow Integration

### Recommended Execution Order

1. **Trivy Filesystem Scan** - Fast, catches obvious issues
2. **Docker Image Scan (this skill)** - Comprehensive, catches image-only issues
3. **CodeQL Scans** - Static analysis for code quality
4. **SBOM Verification** - Supply chain drift detection

### Combined DoD Checklist

```bash
# 1. Filesystem scan (fast)
.github/skills/scripts/skill-runner.sh security-scan-trivy

# 2. Image scan (comprehensive) - THIS SKILL
.github/skills/scripts/skill-runner.sh security-scan-docker-image

# 3. Code analysis
.github/skills/scripts/skill-runner.sh security-scan-codeql

# 4. Go vulnerabilities
.github/skills/scripts/skill-runner.sh security-scan-go-vuln
```

## Performance Considerations

### Execution Time

- **Docker Build**: 2-5 minutes (cached), 5-10 minutes (no-cache)
- **SBOM Generation**: 30-60 seconds
- **Vulnerability Scan**: 30-60 seconds
- **Total**: ~3-7 minutes (typical), ~6-12 minutes (no-cache)

### Optimization Tips

1. **Use Docker layer caching** (default) for faster builds
2. **Run after code changes only** (not needed for doc-only changes)
3. **Parallelize with other scans** (Trivy, CodeQL) for efficiency
4. **Cache vulnerability database** (Grype auto-caches)

## Security Considerations

- SBOM files contain full package inventory (treat as sensitive)
- Vulnerability results may contain CVE details (secure storage)
- Never commit scan results with credentials/tokens
- Review all Critical/High findings before production deployment
- Keep Syft and Grype updated to latest versions

## Troubleshooting

### Build Always Fails

Check Dockerfile syntax and build context:

```bash
# Test build manually
docker build -t charon:test .

# Check build args
docker build --build-arg VERSION=test -t charon:test .
```

### Scan Detects False Positives

Create `.grype.yaml` in project root to suppress known false positives:

```yaml
ignore:
  - vulnerability: CVE-2024-12345
    fix-state: wont-fix
```

### Different Results Than CI

Verify versions match:

```bash
syft version  # Should be v1.17.0
grype version # Should be v0.107.0
```

Update if needed:

```bash
# Reinstall specific versions
curl -sSfL https://raw.githubusercontent.com/anchore/syft/main/install.sh | sh -s -- -b /usr/local/bin v1.17.0
curl -sSfL https://raw.githubusercontent.com/anchore/grype/main/install.sh | sh -s -- -b /usr/local/bin v0.107.0
```

## Notes

- This skill is **not idempotent** due to Docker build step
- Scan results may vary as vulnerability database updates
- Some vulnerabilities may have no fix available yet
- Alpine base image updates may resolve multiple CVEs
- Go stdlib updates may resolve compiled binary CVEs
- Network access required for database updates
- Recommended to run before each commit/push
- Complements but does not replace Trivy filesystem scan

---

**Last Updated**: 2026-01-16
**Maintained by**: Charon Project
**Source**: Syft (SBOM) + Grype (Vulnerability Scanning)
**CI Workflow**: `.github/workflows/supply-chain-pr.yml`
