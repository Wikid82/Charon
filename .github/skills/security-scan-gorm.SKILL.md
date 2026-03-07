---
# agentskills.io specification v1.0
name: "security-scan-gorm"
version: "1.0.0"
description: "Detect GORM security issues including ID leaks, exposed secrets, and common GORM misconfigurations. Use when asked to validate GORM models, check for ID exposure vulnerabilities, scan for API key leaks, verify database security patterns, or ensure GORM best practices compliance. Detects numeric ID exposure (json:id on uint/int fields), exposed API keys/secrets, DTO embedding issues, missing primary key tags, and foreign key indexing problems."
author: "Charon Project"
license: "MIT"
tags:
  - "security"
  - "gorm"
  - "database"
  - "id-leak"
  - "static-analysis"
compatibility:
  os:
    - "linux"
    - "darwin"
  shells:
    - "bash"
requirements:
  - name: "bash"
    version: ">=4.0"
    optional: false
  - name: "grep"
    version: ">=3.0"
    optional: false
environment_variables:
  - name: "VERBOSE"
    description: "Enable verbose debug output"
    default: "0"
    required: false
parameters:
  - name: "mode"
    type: "string"
    description: "Operating mode (--report, --check, --enforce)"
    default: "--report"
    required: false
outputs:
  - name: "scan_results"
    type: "stdout"
    description: "GORM security issues with severity, file locations, and remediation guidance"
  - name: "exit_code"
    type: "number"
    description: "0 if no issues (or report mode), 1 if issues found (check/enforce modes)"
metadata:
  category: "security"
  subcategory: "static-analysis"
  execution_time: "fast"
  risk_level: "low"
  ci_cd_safe: true
  requires_network: false
  idempotent: true
---

# GORM Security Scanner

## Overview

The GORM Security Scanner is a **static analysis tool** that automatically detects GORM security issues and common mistakes in Go codebases. It focuses on preventing ID leak vulnerabilities (IDOR attacks), detecting exposed secrets, and enforcing GORM best practices.

This skill is essential for maintaining secure database models and preventing information disclosure vulnerabilities before they reach production.

## When to Use This Skill

Use this skill when:
- ✅ Creating or modifying GORM database models
- ✅ Reviewing code for security issues before commit
- ✅ Validating API response DTOs for ID exposure
- ✅ Checking for exposed API keys, tokens, or passwords
- ✅ Auditing codebase for GORM best practices compliance
- ✅ Running pre-commit security checks
- ✅ Performing security audits in CI/CD pipelines

## Prerequisites

- Bash 4.0 or higher
- GNU grep (standard on Linux/macOS)
- Read permissions for backend directory
- Project must have Go code with GORM models

## Security Issues Detected

### 🔴 CRITICAL: Numeric ID Exposure

**What:** GORM models with `uint`/`int` primary keys that have `json:"id"` tags

**Risk:** Information disclosure, IDOR vulnerability, database enumeration

**Example:**
```go
// ❌ BAD: Internal database ID exposed
type User struct {
    ID   uint   `json:"id" gorm:"primaryKey"`  // CRITICAL ISSUE
    UUID string `json:"uuid"`
}

// ✅ GOOD: ID hidden, UUID exposed
type User struct {
    ID   uint   `json:"-" gorm:"primaryKey"`
    UUID string `json:"uuid" gorm:"uniqueIndex"`
}
```

**Note:** String-based IDs are **allowed** (assumed to be UUIDs/opaque identifiers)

### 🔴 CRITICAL: Exposed API Keys/Secrets

**What:** Fields with sensitive names (APIKey, Secret, Token, Password) that have visible JSON tags

**Risk:** Credential exposure, unauthorized access

**Example:**
```go
// ❌ BAD: API key visible in responses
type User struct {
    APIKey string `json:"api_key"`  // CRITICAL ISSUE
}

// ✅ GOOD: API key hidden
type User struct {
    APIKey string `json:"-"`
}
```

### 🟡 HIGH: Response DTO Embedding Models

**What:** Response structs that embed GORM models, inheriting exposed ID fields

**Risk:** Unintentional ID exposure through embedding

**Example:**
```go
// ❌ BAD: Inherits exposed ID from models.ProxyHost
type ProxyHostResponse struct {
    models.ProxyHost  // HIGH ISSUE
    Warnings []string `json:"warnings"`
}

// ✅ GOOD: Explicitly define fields
type ProxyHostResponse struct {
    UUID        string   `json:"uuid"`
    Name        string   `json:"name"`
    DomainNames string   `json:"domain_names"`
    Warnings    []string `json:"warnings"`
}
```

### 🔵 MEDIUM: Missing Primary Key Tag

**What:** ID fields with GORM tags but missing `primaryKey` directive

**Risk:** GORM may not recognize field as primary key, causing indexing issues

### 🟢 INFO: Missing Foreign Key Index

**What:** Foreign key fields (ending with ID) without index tags

**Impact:** Query performance degradation

**Suggestion:** Add `gorm:"index"` for better performance

## Usage

### Via VS Code Task (Recommended for Development)

1. Open Command Palette (`Cmd/Ctrl+Shift+P`)
2. Select "**Tasks: Run Task**"
3. Choose "**Lint: GORM Security Scan**"
4. View results in dedicated output panel

### Via Script Runner

```bash
# Report mode - Show all issues, always exits 0
.github/skills/scripts/skill-runner.sh security-scan-gorm

# Report mode with file output
.github/skills/scripts/skill-runner.sh security-scan-gorm --report docs/reports/gorm-scan.txt

# Check mode - Exit 1 if issues found (for CI/pre-commit)
.github/skills/scripts/skill-runner.sh security-scan-gorm --check

# Check mode with file output (for CI artifacts)
.github/skills/scripts/skill-runner.sh security-scan-gorm --check docs/reports/gorm-scan-ci.txt

# Enforce mode - Same as check (future: stricter rules)
.github/skills/scripts/skill-runner.sh security-scan-gorm --enforce
```

### Via Pre-commit Hook (Manual Stage)

```bash
# Run manually on all files
pre-commit run --hook-stage manual gorm-security-scan --all-files

# Run on staged files
pre-commit run --hook-stage manual gorm-security-scan
```

### Direct Script Execution

```bash
# Report mode
./scripts/scan-gorm-security.sh --report

# Check mode (exits 1 if issues found)
./scripts/scan-gorm-security.sh --check

# Verbose mode
VERBOSE=1 ./scripts/scan-gorm-security.sh --report
```

## Parameters

| Parameter | Type | Required | Default | Description |
|-----------|------|----------|---------|-------------|
| mode      | string | No     | --report | Operating mode (--report, --check, --enforce) |
| output_file | string | No | (stdout) | Path to save report file (e.g., docs/reports/gorm-scan.txt) |

## Environment Variables

| Variable | Required | Default | Description |
|----------|----------|---------|-------------|
| VERBOSE  | No       | 0       | Enable verbose debug output (set to 1) |

## Outputs

### Exit Codes

- **0**: Success (report mode) or no issues (check/enforce mode)
- **1**: Issues found (check/enforce mode)
- **2**: Invalid arguments
- **3**: File system error

### Output Format

```
🔍 GORM Security Scanner v1.0.0
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📂 Scanning: backend/

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

🔴 CRITICAL: ID Field Exposed in JSON

  📄 File: backend/internal/models/user.go:23
  🏗️  Struct: User

  💡 Fix: Change json:"id" to json:"-" and use UUID for external references

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

📊 SUMMARY
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

  Scanned: 40 Go files (2,031 lines)
  Duration: 2.1 seconds

  🔴 CRITICAL: 3 issues
  🟡 HIGH:     2 issues
  🔵 MEDIUM:   0 issues
  🟢 INFO:     5 suggestions

  Total Issues: 5 (excluding informational)

━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━

❌ FAILED: 5 security issues detected
```

## Detection Patterns

### Pattern 1: ID Leak Detection

**Target:** GORM models with numeric IDs exposed in JSON

**Detection Logic:**
1. Find `type XXX struct` declarations
2. Apply GORM model detection heuristics:
   - File in `internal/models/` directory, OR
   - Struct has 2+ fields with `gorm:` tags, OR
   - Struct embeds `gorm.Model`
3. Check for `ID` field with numeric type (`uint`, `int`, `int64`, etc.)
4. Check for `json:"id"` tag (not `json:"-"`)
5. Flag as **CRITICAL**

**String ID Policy:** String-based IDs are **NOT flagged** (assumed to be UUIDs)

### Pattern 2: DTO Embedding

**Target:** Response/DTO structs that embed GORM models

**Detection Logic:**
1. Find structs with "Response" or "DTO" in name
2. Look for embedded model types (from `models` package)
3. Check if embedded model has exposed ID field
4. Flag as **HIGH**

### Pattern 3: Exposed Secrets

**Target:** API keys, tokens, passwords, secrets with visible JSON tags

**Detection Logic:**
1. Find fields matching: `APIKey`, `Secret`, `Token`, `Password`, `Hash`
2. Check if JSON tag is NOT `json:"-"`
3. Flag as **CRITICAL**

### Pattern 4: Missing Primary Key Tag

**Target:** ID fields without `gorm:"primaryKey"`

**Detection Logic:**
1. Find ID fields with GORM tags
2. Check if `primaryKey` directive is missing
3. Flag as **MEDIUM**

### Pattern 5: Missing Foreign Key Index

**Target:** Foreign key fields without index tags

**Detection Logic:**
1. Find fields ending with `ID` or `Id`
2. Check if GORM tag lacks `index` directive
3. Flag as **INFO**

### Pattern 6: Missing UUID Fields

**Target:** Models with exposed IDs but no external identifier

**Detection Logic:**
1. Find models with exposed `json:"id"`
2. Check if `UUID` field exists
3. Flag as **HIGH** if missing

## Suppression Mechanism

Use inline comments to suppress false positives:

### Comment Format

```go
// gorm-scanner:ignore [optional reason]
```

### Examples

**External API Response:**
```go
// gorm-scanner:ignore External API response, not a GORM model
type GitHubUser struct {
    ID int `json:"id"`  // Won't be flagged
}
```

**Legacy Code During Migration:**
```go
// gorm-scanner:ignore Legacy model, scheduled for refactor in #1234
type OldModel struct {
    ID uint `json:"id" gorm:"primaryKey"`
}
```

**Internal Service (Never Serialized):**
```go
// gorm-scanner:ignore Internal service struct, never serialized to HTTP
type InternalProcessorState struct {
    ID uint `json:"id"`
}
```

## GORM Model Detection Heuristics

The scanner uses three heuristics to identify GORM models (prevents false positives):

1. **Location-based:** File is in `internal/models/` directory
2. **Tag-based:** Struct has 2+ fields with `gorm:` tags
3. **Embedding-based:** Struct embeds `gorm.Model`

**Non-GORM structs are ignored:**
- Docker container info structs
- External API response structs
- WebSocket connection tracking
- Manual challenge structs

## Performance Metrics

**Measured Performance:**
- **Execution Time:** 2.1 seconds (average)
- **Target:** <5 seconds per full scan
- **Performance Rating:** ✅ **Excellent** (58% faster than requirement)
- **Files Scanned:** 40 Go files
- **Lines Processed:** 2,031 lines

## Examples

### Example 1: Development Workflow

```bash
# Before committing changes to GORM models
.github/skills/scripts/skill-runner.sh security-scan-gorm

# Save report for later review
.github/skills/scripts/skill-runner.sh security-scan-gorm --report docs/reports/gorm-scan-$(date +%Y%m%d).txt

# If issues found, fix them
# Re-run to verify fixes
```

### Example 2: CI/CD Pipeline

```yaml
# GitHub Actions workflow
- name: GORM Security Scanner
  run: .github/skills/scripts/skill-runner.sh security-scan-gorm --check docs/reports/gorm-scan-ci.txt
  continue-on-error: false

- name: Upload GORM Scan Report
  if: always()
  uses: actions/upload-artifact@v4
  with:
    name: gorm-security-report
    path: docs/reports/gorm-scan-ci.txt
    retention-days: 30
```

### Example 3: Pre-commit Hook

```bash
# Manual invocation
pre-commit run --hook-stage manual gorm-security-scan --all-files

# After remediation, move to blocking stage
# Edit .pre-commit-config.yaml:
# stages: [commit]  # Change from [manual]
```

### Example 4: Verbose Mode for Debugging

```bash
# Enable debug output
VERBOSE=1 ./scripts/scan-gorm-security.sh --report

# Shows:
# - File scanning progress
# - GORM model detection decisions
# - Suppression comment handling
# - Pattern matching logic
```

## Error Handling

### Common Issues

**Scanner not found:**
```bash
Error: ./scripts/scan-gorm-security.sh not found
Solution: Ensure script has execute permissions: chmod +x scripts/scan-gorm-security.sh
```

**Permission denied:**
```bash
Error: Permission denied: backend/internal/models/user.go
Solution: Check file permissions and current user access
```

**No Go files found:**
```bash
Warning: No Go files found in backend/
Solution: Verify you're running from project root
```

**False positive on valid code:**
```bash
Solution: Add suppression comment: // gorm-scanner:ignore [reason]
```

## Troubleshooting

### Issue: Scanner reports false positives

**Cause:** Non-GORM struct incorrectly flagged

**Solution:**
1. Add suppression comment with reason
2. Verify struct doesn't match GORM heuristics
3. Report as enhancement if pattern needs refinement

### Issue: Scanner misses known issues

**Cause:** Custom MarshalJSON implementation or XML/YAML tags

**Solution:**
1. Manual code review for custom marshaling
2. Check for `xml:` or `yaml:` tags (not yet supported)
3. See "Known Limitations" section

### Issue: Scanner runs slowly

**Cause:** Large codebase or slow filesystem

**Solution:**
1. Run on specific directory: `cd backend && ../scripts/scan-gorm-security.sh`
2. Use incremental scanning in pre-commit (only changed files)
3. Check filesystem performance

## Known Limitations

1. **Custom MarshalJSON Not Detected**
   - Scanner can't detect ID leaks in custom JSON marshaling logic
   - Mitigation: Manual code review

2. **XML and YAML Tags Not Checked**
   - Only `json:` tags are scanned currently
   - Future: Pattern 7 (XML) and Pattern 8 (YAML)

3. **Multi-line Tag Handling**
   - Tags split across lines may not be detected
   - Enforce single-line tags in style guide

4. **Interface Implementations**
   - Models returned through interfaces may bypass detection
   - Future: Type-based analysis

5. **Map Conversions and Reflection**
   - Runtime conversions not analyzed
   - Mitigation: Code review, runtime monitoring

## Security Thresholds

**Project Standards:**
- **🔴 CRITICAL**: Must fix immediately (blocking)
- **🟡 HIGH**: Should fix before PR merge (warning)
- **🔵 MEDIUM**: Fix in current sprint (informational)
- **🟢 INFO**: Optimize when convenient (suggestion)

## Integration Points

- **Pre-commit:** Manual stage (soft launch), move to commit stage after remediation
- **VS Code:** Command Palette → "Lint: GORM Security Scan"
- **CI/CD:** GitHub Actions quality-checks workflow
- **Definition of Done:** Required check before task completion

## Related Skills

- [security-scan-trivy](./security-scan-trivy.SKILL.md) - Container vulnerability scanning
- [security-scan-codeql](./security-scan-codeql.SKILL.md) - Static analysis for Go/JS
- [qa-lefthook-all](./qa-lefthook-all.SKILL.md) - Lefthook pre-commit-phase quality checks

## Best Practices

1. **Run Before Every Commit**: Catch issues early in development
2. **Fix Critical Issues Immediately**: Don't ignore CRITICAL/HIGH findings
3. **Document Suppressions**: Always explain why an issue is suppressed
4. **Review Periodically**: Audit suppression comments quarterly
5. **Integrate in CI**: Prevent regressions from reaching production
6. **Use UUIDs for External IDs**: Never expose internal database IDs
7. **Hide Sensitive Fields**: All API keys, tokens, passwords should have `json:"-"`
8. **Save Reports for Audit**: Export scan results to `docs/reports/` for tracking and compliance
9. **Track Progress**: Compare reports over time to verify issue remediation

## Remediation Guidance

### Fix ID Leak

```go
// Before
type User struct {
    ID   uint   `json:"id" gorm:"primaryKey"`
    UUID string `json:"uuid"`
}

// After
type User struct {
    ID   uint   `json:"-" gorm:"primaryKey"`        // Hidden
    UUID string `json:"uuid" gorm:"uniqueIndex"`    // Exposed
}

// Update API clients to use UUID instead of ID
```

### Fix Exposed Secret

```go
// Before
type User struct {
    APIKey string `json:"api_key"`
}

// After
type User struct {
    APIKey string `json:"-"`  // Never expose credentials
}
```

### Fix DTO Embedding

```go
// Before
type ProxyHostResponse struct {
    models.ProxyHost  // Inherits exposed ID
    Warnings []string `json:"warnings"`
}

// After
type ProxyHostResponse struct {
    UUID        string   `json:"uuid"`        // Explicit fields only
    Name        string   `json:"name"`
    DomainNames string   `json:"domain_names"`
    Warnings    []string `json:"warnings"`
}
```

## Report Files

**Recommended Locations:**
- **Development:** `docs/reports/gorm-scan-YYYYMMDD.txt` (dated reports)
- **CI/CD:** `docs/reports/gorm-scan-ci.txt` (uploaded as artifact)
- **Pre-Release:** `docs/reports/gorm-scan-release.txt` (audit trail)

**Report Format:**
- Plain text with ANSI color codes (terminal-friendly)
- Includes severity breakdown and summary metrics
- Contains file:line references for all issues
- Provides remediation guidance for each finding

**Agent Usage:**
AI agents can read saved reports instead of parsing terminal output:
```bash
# Generate report
.github/skills/scripts/skill-runner.sh security-scan-gorm --report docs/reports/gorm-scan.txt

# Agent reads report
# File contains structured findings with severity, location, and fixes
```

## Documentation

**Specification:** [docs/plans/gorm_security_scanner_spec.md](../../docs/plans/gorm_security_scanner_spec.md)
**Implementation:** [docs/implementation/gorm_security_scanner_complete.md](../../docs/implementation/gorm_security_scanner_complete.md)
**QA Report:** [docs/reports/gorm_scanner_qa_report.md](../../docs/reports/gorm_scanner_qa_report.md)
**Scan Reports:** `docs/reports/gorm-scan-*.txt` (generated by skill)

## Security References

- [OWASP API Security Top 10](https://owasp.org/www-project-api-security/)
- [OWASP Direct Object Reference (IDOR)](https://owasp.org/www-community/attacks/Insecure_Direct_Object_References)
- [CWE-639: Authorization Bypass Through User-Controlled Key](https://cwe.mitre.org/data/definitions/639.html)
- [GORM Documentation](https://gorm.io/docs/)

---

**Last Updated**: 2026-01-28
**Status**: ✅ Production Ready
**Maintained by**: Charon Project
**Source**: [scripts/scan-gorm-security.sh](../../scripts/scan-gorm-security.sh)
