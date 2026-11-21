````markdown
---
# agentskills.io specification v1.0
name: "security-verify-sbom"
version: "1.0.0"
description: "Verify SBOM completeness, scan for vulnerabilities, and perform semantic diff analysis"
author: "Charon Project"
license: "MIT"
tags:
  - "security"
  - "sbom"
  - "verification"
  - "supply-chain"
  - "vulnerability-scanning"
compatibility:
  os:
    - "linux"
    - "darwin"
  shells:
    - "bash"
requirements:
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
  - name: "SBOM_FORMAT"
    description: "SBOM format (spdx-json, cyclonedx-json)"
    default: "spdx-json"
    required: false
  - name: "VULN_SCAN_ENABLED"
    description: "Enable vulnerability scanning"
    default: "true"
    required: false
parameters:
  - name: "target"
    type: "string"
    description: "Docker image or file path"
    required: true
    validation: "^[a-zA-Z0-9:/@._-]+$"
  - name: "baseline"
    type: "string"
    description: "Baseline SBOM file path for comparison"
    required: false
    default: ""
  - name: "vuln_scan"
    type: "boolean"
    description: "Run vulnerability scan"
    required: false
    default: true
outputs:
  - name: "sbom_file"
    type: "file"
    description: "Generated SBOM in SPDX JSON format"
  - name: "scan_results"
    type: "stdout"
    description: "Verification results and vulnerability counts"
  - name: "exit_code"
    type: "number"
    description: "0 if no critical issues, 1 if critical vulnerabilities found, 2 if validation failed"
metadata:
  category: "security"
  subcategory: "supply-chain"
  execution_time: "medium"
  risk_level: "low"
  ci_cd_safe: true
  requires_network: true
  idempotent: true
exit_codes:
  0: "Verification successful"
  1: "Verification failed or critical vulnerabilities found"
  2: "Missing dependencies or invalid parameters"
---

# Security: Verify SBOM

Verify Software Bill of Materials (SBOM) completeness, scan for vulnerabilities, and perform semantic diff analysis.

## Overview

This skill generates an SBOM for Docker images or local files, compares it with a baseline (if provided), scans for known vulnerabilities using Grype, and reports any critical security issues. It supports both online vulnerability scanning and air-gapped operation modes.

## Features

- Generate SBOM in SPDX format (standardized)
- Compare with baseline SBOM (semantic diff)
- Scan for vulnerabilities (Critical/High/Medium/Low)
- Validate SBOM structure and completeness
- Support Docker images and local files
- Air-gapped operation support (skip vulnerability scanning)
- Detect added/removed packages between builds

## Prerequisites

- Syft 1.17.0 or higher (for SBOM generation)
- Grype 0.85.0 or higher (for vulnerability scanning)
- jq 1.6 or higher (for JSON processing)
- Internet connection (for vulnerability database updates, unless air-gapped mode)
- Docker (if scanning container images)

## Usage

### Basic Verification

Run with default settings (generate SBOM + scan vulnerabilities):

```bash
cd /path/to/charon
.github/skills/scripts/skill-runner.sh security-verify-sbom ghcr.io/user/charon:latest
```

### Verify Docker Image with Baseline Comparison

Compare current SBOM against a known baseline:

```bash
.github/skills/scripts/skill-runner.sh security-verify-sbom \
  charon:local sbom-baseline.json
```

### Air-Gapped Mode (No Vulnerability Scan)

Verify SBOM structure only, without network access:

```bash
VULN_SCAN_ENABLED=false .github/skills/scripts/skill-runner.sh \
  security-verify-sbom charon:local
```

### Custom SBOM Format

Generate SBOM in CycloneDX format:

```bash
SBOM_FORMAT=cyclonedx-json .github/skills/scripts/skill-runner.sh \
  security-verify-sbom charon:local
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| target    | string | Yes    | -       | Docker image tag or local image name |
| baseline  | string | No     | ""      | Path to baseline SBOM for comparison |
| vuln_scan | boolean | No    | true    | Run vulnerability scan (set VULN_SCAN_ENABLED=false to disable) |

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| SBOM_FORMAT | No | spdx-json | SBOM format (spdx-json or cyclonedx-json) |
| VULN_SCAN_ENABLED | No | true | Enable vulnerability scanning (set to false for air-gapped) |

## Outputs

- **Success Exit Code**: 0 (no critical issues found)
- **Error Exit Codes**:
  - 1: Critical vulnerabilities found or verification failed
  - 2: Missing dependencies or invalid parameters
- **Generated Files**:
  - `sbom-generated.json`: Generated SBOM file
  - `vuln-results.json`: Vulnerability scan results (if enabled)
- **Output**: Verification summary to stdout

## Examples

### Example 1: Verify Local Docker Image

```bash
$ .github/skills/scripts/skill-runner.sh security-verify-sbom charon:test
[INFO] Generating SBOM for charon:test...
[SBOM] Generated SBOM contains 247 packages
[INFO] Scanning for vulnerabilities...
[VULN] Found: 0 Critical, 2 High, 15 Medium, 42 Low
[INFO] High vulnerabilities:
  - CVE-2023-12345 in golang.org/x/crypto (CVSS: 7.5)
  - CVE-2024-67890 in github.com/example/lib (CVSS: 8.2)
[SUCCESS] Verification complete - review High severity vulnerabilities
```

### Example 2: With Baseline Comparison

```bash
$ .github/skills/scripts/skill-runner.sh security-verify-sbom \
  charon:latest sbom-baseline.json
[INFO] Generating SBOM for charon:latest...
[SBOM] Generated SBOM contains 247 packages
[INFO] Comparing with baseline...
[BASELINE] Baseline: 245 packages, Current: 247 packages
[BASELINE] Delta: +2 packages (0.8% increase)
[BASELINE] Added packages:
  - golang.org/x/crypto@v0.30.0
  - github.com/pkg/errors@v0.9.1
[BASELINE] Removed packages: (none)
[INFO] Scanning for vulnerabilities...
[VULN] Found: 0 Critical, 0 High, 5 Medium, 20 Low
[SUCCESS] Verification complete (0.8% variance from baseline)
```

### Example 3: Air-Gapped Mode

```bash
$ VULN_SCAN_ENABLED=false .github/skills/scripts/skill-runner.sh \
  security-verify-sbom charon:local
[INFO] Generating SBOM for charon:local...
[SBOM] Generated SBOM contains 247 packages
[INFO] Vulnerability scanning disabled (air-gapped mode)
[SUCCESS] SBOM generation complete
```

### Example 4: CI/CD Pipeline Integration

```yaml
# GitHub Actions example
- name: Verify SBOM
  run: |
    .github/skills/scripts/skill-runner.sh \
      security-verify-sbom ghcr.io/${{ github.repository }}:${{ github.sha }}
  continue-on-error: false
```

## Semantic Diff Analysis

When a baseline SBOM is provided, the skill performs semantic comparison:

1. **Package Count Comparison**: Reports total package delta
2. **Added Packages**: Lists new dependencies with versions
3. **Removed Packages**: Lists removed dependencies
4. **Variance Percentage**: Calculates percentage change
5. **Threshold Check**: Warns if variance exceeds 5%

## Vulnerability Severity Thresholds

**Project Standards**:
- **CRITICAL**: Must fix before release (blocking) - **Script exits with code 1**
- **HIGH**: Should fix before release (warning) - **Script continues but logs warning**
- **MEDIUM**: Fix in next release cycle (informational)
- **LOW**: Optional, fix as time permits

## Error Handling

### Common Issues

**Syft not installed**:
```bash
Error: syft command not found
Solution: Install Syft from https://github.com/anchore/syft
```

**Grype not installed**:
```bash
Error: grype command not found
Solution: Install Grype from https://github.com/anchore/grype
```

**Docker image not found**:
```bash
Error: Unable to find image 'charon:test' locally
Solution: Build the image or pull from registry
```

**Invalid baseline SBOM**:
```bash
Error: Baseline SBOM file not found: sbom-baseline.json
Solution: Verify the file path or omit baseline parameter
```

**Network timeout (vulnerability scan)**:
```bash
Warning: Failed to update vulnerability database
Solution: Check internet connection or use air-gapped mode (VULN_SCAN_ENABLED=false)
```

## Exit Codes

- **0**: Verification successful, no critical vulnerabilities
- **1**: Critical vulnerabilities found or verification failed
- **2**: Missing dependencies or invalid parameters

## Related Skills

- [security-sign-cosign](./security-sign-cosign.SKILL.md) - Sign artifacts with Cosign
- [security-slsa-provenance](./security-slsa-provenance.SKILL.md) - Generate SLSA provenance
- [security-scan-trivy](./security-scan-trivy.SKILL.md) - Alternative vulnerability scanner

## Notes

- SBOM generation requires read access to Docker images
- Vulnerability database is updated automatically by Grype
- Baseline comparison is optional but recommended for drift detection
- Critical vulnerabilities will cause the script to exit with code 1
- High vulnerabilities generate warnings but don't block execution
- Use air-gapped mode when network access is unavailable
- SPDX format is standardized and recommended over CycloneDX

## Security Considerations

- Never commit SBOM files containing sensitive information
- Review all High and Critical vulnerabilities before deployment
- Baseline drift >5% should trigger manual review
- Air-gapped mode skips vulnerability scanning - use with caution
- SBOM files can reveal internal architecture - protect accordingly

---

**Last Updated**: 2026-01-10
**Maintained by**: Charon Project
**Source**: Syft (SBOM generation) + Grype (vulnerability scanning)

````
