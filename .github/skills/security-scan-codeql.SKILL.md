---
# agentskills.io specification v1.0
name: "security-scan-codeql"
version: "1.0.0"
description: "Run CodeQL security analysis for Go and JavaScript/TypeScript code"
author: "Charon Project"
license: "MIT"
tags:
  - "security"
  - "scanning"
  - "codeql"
  - "sast"
  - "vulnerabilities"
compatibility:
  os:
    - "linux"
    - "darwin"
  shells:
    - "bash"
requirements:
  - name: "codeql"
    version: ">=2.17.0"
    optional: false
environment_variables:
  - name: "CODEQL_THREADS"
    description: "Number of threads for analysis (0 = auto)"
    default: "0"
    required: false
  - name: "CODEQL_FAIL_ON_ERROR"
    description: "Exit with error on HIGH/CRITICAL findings"
    default: "true"
    required: false
parameters:
  - name: "language"
    type: "string"
    description: "Language to scan (go, javascript, all)"
    default: "all"
    required: false
  - name: "format"
    type: "string"
    description: "Output format (sarif, text, summary)"
    default: "summary"
    required: false
outputs:
  - name: "sarif_files"
    type: "file"
    description: "SARIF files for each language scanned"
  - name: "summary"
    type: "stdout"
    description: "Human-readable findings summary"
  - name: "exit_code"
    type: "number"
    description: "0 if no HIGH/CRITICAL issues, non-zero otherwise"
metadata:
  category: "security"
  subcategory: "sast"
  execution_time: "long"
  risk_level: "low"
  ci_cd_safe: true
  requires_network: false
  idempotent: true
---

# Security Scan CodeQL

## Overview

Executes GitHub CodeQL static analysis security testing (SAST) for Go and JavaScript/TypeScript code. Uses the **security-and-quality** query suite to match GitHub Actions CI configuration exactly.

This skill ensures local development catches the same security issues that CI would detect, preventing CI failures due to security findings.

## Prerequisites

- CodeQL CLI 2.17.0 or higher installed
- Query packs: `codeql/go-queries`, `codeql/javascript-queries`
- Sufficient disk space for CodeQL databases (~500MB per language)

## Usage

### Basic Usage

Scan all languages with summary output:

```bash
cd /path/to/charon
.github/skills/scripts/skill-runner.sh security-scan-codeql
```

### Scan Specific Language

Scan only Go code:

```bash
.github/skills/scripts/skill-runner.sh security-scan-codeql go
```

Scan only JavaScript/TypeScript code:

```bash
.github/skills/scripts/skill-runner.sh security-scan-codeql javascript
```

### Full SARIF Output

Get detailed SARIF output for integration with tools:

```bash
.github/skills/scripts/skill-runner.sh security-scan-codeql all sarif
```

### Text Output

Get text-formatted detailed findings:

```bash
.github/skills/scripts/skill-runner.sh security-scan-codeql all text
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| language  | string | No     | all     | Language to scan (go, javascript, all) |
| format    | string | No     | summary | Output format (sarif, text, summary) |

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| CODEQL_THREADS | No | 0 | Analysis threads (0 = auto-detect) |
| CODEQL_FAIL_ON_ERROR | No | true | Fail on HIGH/CRITICAL findings |

## Query Suite

This skill uses the **security-and-quality** suite to match CI:

| Language | Suite | Queries | Coverage |
|----------|-------|---------|----------|
| Go | go-security-and-quality.qls | version-dependent | Security + quality issues |
| JavaScript | javascript-security-and-quality.qls | version-dependent | Security + quality issues |

**Note:** This matches GitHub Actions CodeQL default configuration exactly.

## Outputs

- **SARIF Files**:
  - `codeql-results-go.sarif` - Go findings
  - `codeql-results-js.sarif` - JavaScript/TypeScript findings
- **Databases**:
  - `codeql-db-go/` - Go CodeQL database
  - `codeql-db-js/` - JavaScript CodeQL database
- **Exit Codes**:
  - 0: No HIGH/CRITICAL findings
  - 1: HIGH/CRITICAL findings detected
  - 2: Scanner error

## Security Categories

### CWE Coverage

| Category | Description | Languages |
|----------|-------------|-----------|
| CWE-079 | Cross-Site Scripting (XSS) | JS |
| CWE-089 | SQL Injection | Go, JS |
| CWE-117 | Log Injection | Go |
| CWE-200 | Information Exposure | Go, JS |
| CWE-312 | Cleartext Storage | Go, JS |
| CWE-327 | Weak Cryptography | Go, JS |
| CWE-502 | Deserialization | Go, JS |
| CWE-611 | XXE Injection | Go |
| CWE-640 | Email Injection | Go |
| CWE-798 | Hardcoded Credentials | Go, JS |
| CWE-918 | SSRF | Go, JS |

## Examples

### Example 1: Full Scan (Default)

```bash
# Scan all languages, show summary
.github/skills/scripts/skill-runner.sh security-scan-codeql
```

Output:
```
[STEP] CODEQL: Scanning Go code...
[INFO] Creating database for backend/
[INFO] Analyzing with security-and-quality suite (61 queries)
[INFO] Found: 0 errors, 5 warnings, 3 notes
[STEP] CODEQL: Scanning JavaScript code...
[INFO] Creating database for frontend/
[INFO] Analyzing with security-and-quality suite (204 queries)
[INFO] Found: 0 errors, 2 warnings, 8 notes
[SUCCESS] CodeQL scan complete - no HIGH/CRITICAL issues
```

### Example 2: Go Only with Text Output

```bash
# Detailed text output for Go findings
.github/skills/scripts/skill-runner.sh security-scan-codeql go text
```

### Example 3: CI/CD Pipeline Integration

```yaml
# GitHub Actions example (already integrated in codeql.yml)
- name: Run CodeQL Security Scan
  run: .github/skills/scripts/skill-runner.sh security-scan-codeql all summary
  continue-on-error: false
```

### Example 4: Pre-Commit Integration

```bash
# Already available via pre-commit
pre-commit run codeql-go-scan --all-files
pre-commit run codeql-js-scan --all-files
pre-commit run codeql-check-findings --all-files
```

## Error Handling

### Common Issues

**CodeQL version too old**:
```bash
Error: Extensible predicate API mismatch
Solution: Upgrade CodeQL CLI: gh codeql set-version latest
```

**Query pack not found**:
```bash
Error: Could not resolve pack codeql/go-queries
Solution: codeql pack download codeql/go-queries codeql/javascript-queries
```

**Database creation failed**:
```bash
Error: No source files found
Solution: Verify source-root points to correct directory
```

## Exit Codes

- **0**: No HIGH/CRITICAL (error-level) findings
- **1**: HIGH/CRITICAL findings detected (blocks CI)
- **2**: Scanner error or invalid arguments

## Related Skills

- [security-scan-trivy](./security-scan-trivy.SKILL.md) - Container/dependency vulnerabilities
- [security-scan-go-vuln](./security-scan-go-vuln.SKILL.md) - Go-specific CVE checking
- [qa-precommit-all](./qa-precommit-all.SKILL.md) - Pre-commit quality checks

## CI Alignment

This skill is specifically designed to match GitHub Actions CodeQL workflow:

| Parameter | Local | CI | Aligned |
|-----------|-------|-----|---------|
| Query Suite | security-and-quality | security-and-quality | ✅ |
| Query Expansion | version-dependent | version-dependent | ✅ (when versions match) |
| Threading | auto | auto | ✅ |
| Baseline Info | enabled | enabled | ✅ |

## Viewing Results

### VS Code SARIF Viewer (Recommended)

1. Install extension: `MS-SarifVSCode.sarif-viewer`
2. Open `codeql-results-go.sarif` or `codeql-results-js.sarif`
3. Navigate findings with inline annotations

### Command Line (jq)

```bash
# Count findings
jq '.runs[].results | length' codeql-results-go.sarif

# List findings
jq -r '.runs[].results[] | "\(.level): \(.message.text)"' codeql-results-go.sarif
```

### GitHub Security Tab

SARIF files are automatically uploaded to GitHub Security tab in CI.

## Performance

| Language | Database Creation | Analysis | Total |
|----------|------------------|----------|-------|
| Go | ~30s | ~30s | ~60s |
| JavaScript | ~45s | ~45s | ~90s |
| All | ~75s | ~75s | ~150s |

**Note:** First run downloads query packs; subsequent runs are faster.

## Notes

- Requires CodeQL CLI 2.17.0+ (use `gh codeql set-version latest` to upgrade)
- Databases are regenerated each run (not cached)
- SARIF files are gitignored (see `.gitignore`)
- Query results may vary between CodeQL versions
- Use `.codeql/` directory for custom queries or suppressions

---

**Last Updated**: 2025-12-24
**Maintained by**: Charon Project
**Source**: CodeQL CLI + GitHub Query Packs
