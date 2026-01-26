---
# agentskills.io specification v1.0
name: "security-scan-go-vuln"
version: "1.0.0"
description: "Run Go vulnerability checker (govulncheck) to detect known vulnerabilities in Go code"
author: "Charon Project"
license: "MIT"
tags:
  - "security"
  - "vulnerabilities"
  - "go"
  - "govulncheck"
  - "scanning"
compatibility:
  os:
    - "linux"
    - "darwin"
  shells:
    - "bash"
requirements:
  - name: "go"
    version: ">=1.23"
    optional: false
environment_variables:
  - name: "GOVULNCHECK_FORMAT"
    description: "Output format (text, json, sarif)"
    default: "text"
    required: false
parameters:
  - name: "format"
    type: "string"
    description: "Output format (text, json, sarif)"
    default: "text"
    required: false
  - name: "mode"
    type: "string"
    description: "Scan mode (source or binary)"
    default: "source"
    required: false
outputs:
  - name: "vulnerability_report"
    type: "stdout"
    description: "List of detected vulnerabilities with remediation advice"
  - name: "exit_code"
    type: "number"
    description: "0 if no vulnerabilities found, 3 if vulnerabilities detected"
metadata:
  category: "security"
  subcategory: "vulnerability"
  execution_time: "short"
  risk_level: "low"
  ci_cd_safe: true
  requires_network: true
  idempotent: true
---

# Security Scan Go Vulnerability

## Overview

Executes `govulncheck` from the official Go vulnerability database to scan Go code and dependencies for known security vulnerabilities. This tool analyzes both direct and transitive dependencies, providing actionable remediation advice.

This skill is designed for CI/CD pipelines and pre-release security validation.

## Prerequisites

- Go 1.23 or higher installed and in PATH
- Internet connection (for vulnerability database access)
- Go module dependencies downloaded (`go mod download`)
- Valid Go project with `go.mod` file

## Usage

### Basic Usage

Run with default settings (text output, source mode):

```bash
cd /path/to/charon
.github/skills/scripts/skill-runner.sh security-scan-go-vuln
```

### JSON Output

Get results in JSON format for parsing:

```bash
.github/skills/scripts/skill-runner.sh security-scan-go-vuln json
```

### SARIF Output

Get results in SARIF format for GitHub Code Scanning:

```bash
.github/skills/scripts/skill-runner.sh security-scan-go-vuln sarif
```

### Custom Format via Environment

```bash
GOVULNCHECK_FORMAT=json .github/skills/scripts/skill-runner.sh security-scan-go-vuln
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| format    | string | No     | text    | Output format (text, json, sarif) |
| mode      | string | No     | source  | Scan mode (source or binary) |

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| GOVULNCHECK_FORMAT | No | text | Output format override |

## Outputs

- **Success Exit Code**: 0 (no vulnerabilities found)
- **Error Exit Codes**:
  - 1: Scan error or invalid arguments
  - 3: Vulnerabilities detected
- **Output**: Vulnerability report to stdout

## Vulnerability Report Format

### Text Output (Default)

```
Scanning for dependencies with known vulnerabilities...
No vulnerabilities found.
```

Or if vulnerabilities are found:

```
Found 2 vulnerabilities in dependencies

Vulnerability #1: GO-2023-1234
  Package: github.com/example/vulnerable
  Version: v1.2.3
  Description: Buffer overflow in Parse function
  Fixed in: v1.2.4
  More info: https://vuln.go.dev/GO-2023-1234

Vulnerability #2: GO-2023-5678
  Package: golang.org/x/crypto/ssh
  Version: v0.1.0
  Description: Insecure default configuration
  Fixed in: v0.3.0
  More info: https://vuln.go.dev/GO-2023-5678
```

## Examples

### Example 1: Basic Scan

```bash
# Scan backend Go code for vulnerabilities
cd backend
.github/skills/scripts/skill-runner.sh security-scan-go-vuln
```

Output:
```
Scanning your code and 125 packages across 23 dependent modules for known vulnerabilities...
No vulnerabilities found.
```

### Example 2: JSON Output for CI/CD

```bash
# Get JSON output for automated processing
.github/skills/scripts/skill-runner.sh security-scan-go-vuln json > vuln-report.json
```

### Example 3: CI/CD Pipeline Integration

```yaml
# GitHub Actions example
- name: Check Go Vulnerabilities
  run: .github/skills/scripts/skill-runner.sh security-scan-go-vuln
  working-directory: backend

- name: Upload SARIF Report
  if: always()
  run: |
    .github/skills/scripts/skill-runner.sh security-scan-go-vuln sarif > results.sarif
    # Upload to GitHub Code Scanning
```

### Example 4: Binary Mode Scan

```bash
# Scan a compiled binary
.github/skills/scripts/skill-runner.sh security-scan-go-vuln text binary
```

## Error Handling

### Common Issues

**Go not installed**:
```bash
Error: Go 1.23+ is required
Solution: Install Go 1.23 or higher
```

**Network unavailable**:
```bash
Error: Failed to fetch vulnerability database
Solution: Check internet connection or proxy settings
```

**Vulnerabilities found**:
```bash
Exit code: 3
Solution: Review vulnerabilities and update affected packages
```

**Module not found**:
```bash
Error: go.mod file not found
Solution: Run from a valid Go module directory
```

## Exit Codes

- **0**: No vulnerabilities found
- **1**: Scan error or invalid arguments
- **3**: Vulnerabilities detected (standard govulncheck exit code)

## Related Skills

- [security-scan-trivy](./security-scan-trivy.SKILL.md) - Multi-language vulnerability scanning
- [test-backend-coverage](./test-backend-coverage.SKILL.md) - Backend test coverage

## Notes

- `govulncheck` uses the official Go vulnerability database at https://vuln.go.dev
- Database is automatically updated during each scan
- Only checks vulnerabilities that are reachable from your code
- Does not require building the code (analyzes source)
- Can also scan compiled binaries with `--mode=binary`
- Results may change as new vulnerabilities are published
- Recommended to run before each release and in CI/CD
- Zero false positives (only reports known CVEs)

## Remediation Workflow

When vulnerabilities are found:

1. **Review the Report**: Understand which packages are affected
2. **Check Fix Availability**: Look for fixed versions in the report
3. **Update Dependencies**: Run `go get -u` to update affected packages
4. **Re-run Scan**: Verify vulnerabilities are resolved
5. **Test**: Run full test suite after updates
6. **Document**: Note any unresolvable vulnerabilities in security log

## Integration with GitHub Security

For SARIF output integration with GitHub Code Scanning:

```bash
# Generate SARIF report
.github/skills/scripts/skill-runner.sh security-scan-go-vuln sarif > govulncheck.sarif

# Upload to GitHub (requires GitHub CLI)
gh api /repos/:owner/:repo/code-scanning/sarifs \
  -F sarif=@govulncheck.sarif \
  -F commit_sha=$GITHUB_SHA \
  -F ref=$GITHUB_REF
```

---

**Last Updated**: 2025-12-20
**Maintained by**: Charon Project
**Source**: `go run golang.org/x/vuln/cmd/govulncheck@latest`
