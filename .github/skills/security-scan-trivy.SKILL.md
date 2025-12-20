---
# agentskills.io specification v1.0
name: "security-scan-trivy"
version: "1.0.0"
description: "Run Trivy security scanner for vulnerabilities, secrets, and misconfigurations"
author: "Charon Project"
license: "MIT"
tags:
  - "security"
  - "scanning"
  - "trivy"
  - "vulnerabilities"
  - "secrets"
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
environment_variables:
  - name: "TRIVY_SEVERITY"
    description: "Comma-separated list of severities to scan for"
    default: "CRITICAL,HIGH,MEDIUM"
    required: false
  - name: "TRIVY_TIMEOUT"
    description: "Timeout for Trivy scan"
    default: "10m"
    required: false
parameters:
  - name: "scanners"
    type: "string"
    description: "Comma-separated list of scanners (vuln, secret, misconfig)"
    default: "vuln,secret,misconfig"
    required: false
  - name: "format"
    type: "string"
    description: "Output format (table, json, sarif)"
    default: "table"
    required: false
outputs:
  - name: "scan_results"
    type: "stdout"
    description: "Trivy scan results in specified format"
  - name: "exit_code"
    type: "number"
    description: "0 if no issues found, non-zero otherwise"
metadata:
  category: "security"
  subcategory: "scan"
  execution_time: "medium"
  risk_level: "low"
  ci_cd_safe: true
  requires_network: true
  idempotent: true
---

# Security Scan Trivy

## Overview

Executes Trivy security scanner using Docker to scan the project for vulnerabilities, secrets, and misconfigurations. Trivy scans filesystem, dependencies, and configuration files to identify security issues.

This skill is designed for CI/CD pipelines and local security validation before commits.

## Prerequisites

- Docker 24.0 or higher installed and running
- Internet connection (for vulnerability database updates)
- Read permissions for project directory

## Usage

### Basic Usage

Run with default settings (all scanners, table format):

```bash
cd /path/to/charon
.github/skills/scripts/skill-runner.sh security-scan-trivy
```

### Custom Scanners

Scan only for vulnerabilities:

```bash
.github/skills/scripts/skill-runner.sh security-scan-trivy vuln
```

Scan for secrets and misconfigurations:

```bash
.github/skills/scripts/skill-runner.sh security-scan-trivy secret,misconfig
```

### Custom Severity

Scan only for critical and high severity issues:

```bash
TRIVY_SEVERITY=CRITICAL,HIGH .github/skills/scripts/skill-runner.sh security-scan-trivy
```

### JSON Output

Get results in JSON format for parsing:

```bash
.github/skills/scripts/skill-runner.sh security-scan-trivy vuln,secret,misconfig json
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| scanners  | string | No     | vuln,secret,misconfig | Comma-separated list of scanners to run |
| format    | string | No     | table   | Output format (table, json, sarif) |

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| TRIVY_SEVERITY | No | CRITICAL,HIGH,MEDIUM | Severities to report |
| TRIVY_TIMEOUT | No | 10m | Maximum scan duration |

## Outputs

- **Success Exit Code**: 0 (no issues found)
- **Error Exit Codes**:
  - 1: Issues found
  - 2: Scanner error
- **Output**: Scan results to stdout in specified format

## Scanner Types

### Vulnerability Scanner (vuln)
Scans for known CVEs in:
- Go dependencies (go.mod)
- npm packages (package.json)
- Docker base images (Dockerfile)

### Secret Scanner (secret)
Detects exposed secrets:
- API keys
- Passwords
- Tokens
- Private keys

### Misconfiguration Scanner (misconfig)
Checks configuration files:
- Dockerfile best practices
- Kubernetes manifests
- Terraform files
- Docker Compose files

## Examples

### Example 1: Full Scan with Table Output

```bash
# Scan all vulnerability types, display as table
.github/skills/scripts/skill-runner.sh security-scan-trivy
```

Output:
```
2025-12-20T10:00:00Z	INFO	Trivy version: 0.48.0
2025-12-20T10:00:01Z	INFO	Scanning filesystem...
Total: 0 (CRITICAL: 0, HIGH: 0, MEDIUM: 0)
```

### Example 2: Vulnerability Scan Only (JSON)

```bash
# Scan for vulnerabilities only, output as JSON
.github/skills/scripts/skill-runner.sh security-scan-trivy vuln json > trivy-results.json
```

### Example 3: Critical Issues Only

```bash
# Scan for critical severity issues only
TRIVY_SEVERITY=CRITICAL .github/skills/scripts/skill-runner.sh security-scan-trivy
```

### Example 4: CI/CD Pipeline Integration

```yaml
# GitHub Actions example
- name: Run Trivy Security Scan
  run: .github/skills/scripts/skill-runner.sh security-scan-trivy
  continue-on-error: false
```

## Error Handling

### Common Issues

**Docker not running**:
```bash
Error: Cannot connect to Docker daemon
Solution: Start Docker service
```

**Network timeout**:
```bash
Error: Failed to download vulnerability database
Solution: Increase TRIVY_TIMEOUT or check internet connection
```

**Vulnerabilities found**:
```bash
Exit code: 1
Solution: Review and remediate reported vulnerabilities
```

## Exit Codes

- **0**: No security issues found
- **1**: Security issues detected
- **2**: Scanner error or invalid arguments

## Related Skills

- [security-scan-go-vuln](./security-scan-go-vuln.SKILL.md) - Go-specific vulnerability checking
- [qa-precommit-all](./qa-precommit-all.SKILL.md) - Pre-commit quality checks

## Notes

- Trivy automatically updates its vulnerability database on each run
- Scan results may vary based on database version
- Some vulnerabilities may have no fix available yet
- Consider using `.trivyignore` file to suppress false positives
- Recommended to run before each release
- Network access required for first run and database updates

## Security Thresholds

**Project Standards**:
- **CRITICAL**: Must fix before release (blocking)
- **HIGH**: Should fix before release (warning)
- **MEDIUM**: Fix in next release cycle (informational)
- **LOW**: Optional, fix as time permits

---

**Last Updated**: 2025-12-20
**Maintained by**: Charon Project
**Source**: Docker inline command (Trivy)
