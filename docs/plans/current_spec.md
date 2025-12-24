# Local CodeQL Integration Plan

**Status**: Ready for Implementation
**Last Updated**: 2025-12-24
**Related Context**: CI failing on CWE-918 (SSRF) findings, need local triage workflow

---

## Overview

This plan outlines how to use the local CodeQL installation at `/projects/codeql/codeql` for scanning the Charon project, enabling local triage of security findings before CI runs.

---

## 1. Prerequisites

### Install CodeQL CLI

The CodeQL query packs are in the workspace, but you need the CodeQL CLI:

```bash
# Option 1: Download from GitHub releases
curl -L -o codeql-linux64.zip https://github.com/github/codeql-cli-binaries/releases/latest/download/codeql-linux64.zip
unzip codeql-linux64.zip -d ~/.local/
export PATH="$HOME/.local/codeql:$PATH"

# Option 2: Use VS Code extension (recommended)
# Install the "GitHub.vscode-codeql" extension - it bundles the CLI
```

### Verify Installation

```bash
codeql --version
```

---

## 2. Running CodeQL Locally

### Go Backend Scanning

```bash
# Step 1: Create database (from Charon root)
cd /projects/Charon
codeql database create codeql-db-go \
  --language=go \
  --source-root=backend \
  --overwrite

# Step 2: Run security queries using workspace packs
codeql database analyze codeql-db-go \
  /projects/codeql/codeql/go/ql/src/codeql-suites/go-security-extended.qls \
  --format=sarif-latest \
  --output=codeql-results-go.sarif

# Alternative: Run specific CWE query (e.g., SSRF - CWE-918)
codeql database analyze codeql-db-go \
  /projects/codeql/codeql/go/ql/src/Security/CWE-918 \
  --format=sarif-latest \
  --output=codeql-ssrf-go.sarif
```

### JavaScript/TypeScript Frontend Scanning

```bash
# Step 1: Create database (from Charon root)
cd /projects/Charon
codeql database create codeql-db-js \
  --language=javascript \
  --source-root=frontend \
  --overwrite

# Step 2: Run security queries
codeql database analyze codeql-db-js \
  /projects/codeql/codeql/javascript/ql/src/codeql-suites/javascript-security-extended.qls \
  --format=sarif-latest \
  --output=codeql-results-js.sarif
```

---

## 3. Available Query Suites

The workspace contains these query packs:

| Language | Pack Location | Key Suites |
|----------|---------------|------------|
| Go | `/projects/codeql/codeql/go/ql/src/` | `go-code-scanning.qls`, `go-security-extended.qls` |
| JavaScript | `/projects/codeql/codeql/javascript/ql/src/` | `javascript-code-scanning.qls`, `javascript-security-extended.qls` |

### Go Security CWEs Available

From `/projects/codeql/codeql/go/ql/src/Security/`:

- CWE-020 (Improper Input Validation)
- CWE-022 (Path Traversal)
- CWE-078 (Command Injection)
- CWE-079 (XSS)
- CWE-089 (SQL Injection)
- CWE-295 (Certificate Validation)
- CWE-312 (Cleartext Storage)
- CWE-327 (Weak Crypto)
- CWE-338 (Insecure Randomness)
- CWE-601 (Open Redirect)
- CWE-770 (Resource Exhaustion)
- **CWE-918 (SSRF)** ← Current CI failure

---

## 4. Viewing and Triaging SARIF Results

### Option 1: VS Code SARIF Viewer (Recommended)

1. Install the "SARIF Viewer" extension (`MS-SarifVSCode.sarif-viewer`)
2. Open any `.sarif` file in VS Code
3. Click on findings to navigate directly to code

### Option 2: Command Line Summary

```bash
# Quick summary of findings
jq '.runs[0].results | length' codeql-results-go.sarif
jq '.runs[0].results[] | {rule: .ruleId, file: .locations[0].physicalLocation.artifactLocation.uri, line: .locations[0].physicalLocation.region.startLine}' codeql-results-go.sarif
```

### Option 3: GitHub Code Scanning (CI)

SARIF files are automatically uploaded in CI via `.github/workflows/codeql.yml`.

---

## 5. Current SSRF Findings (CWE-918)

Based on existing `codeql-go.sarif`, there is **1 SSRF finding**:

| File | Line | Issue |
|------|------|-------|
| [internal/services/notification_service.go](../backend/internal/services/notification_service.go#L151) | 151 | URL from user input flows to HTTP request |

**Root Cause**: `provider.URL` from user input is used directly in `http.NewRequest`.

**Remediation Pattern**:

```go
// Before making requests with user-provided URLs:
// 1. Validate URL scheme (only allow http/https)
// 2. Resolve hostname and check against allowlist/blocklist
// 3. Block private IP ranges (10.x, 172.16-31.x, 192.168.x)
// See: backend/internal/security/url_validator.go
```

---

## 6. VS Code Tasks to Add

Add these to `.vscode/tasks.json`:

```jsonc
{
    "label": "Security: CodeQL Go Scan",
    "type": "shell",
    "command": "codeql database create codeql-db-go --language=go --source-root=backend --overwrite && codeql database analyze codeql-db-go /projects/codeql/codeql/go/ql/src/codeql-suites/go-security-extended.qls --format=sarif-latest --output=codeql-results-go.sarif",
    "group": "test",
    "problemMatcher": [],
    "presentation": {
        "reveal": "always",
        "panel": "new"
    }
},
{
    "label": "Security: CodeQL JS Scan",
    "type": "shell",
    "command": "codeql database create codeql-db-js --language=javascript --source-root=frontend --overwrite && codeql database analyze codeql-db-js /projects/codeql/codeql/javascript/ql/src/codeql-suites/javascript-security-extended.qls --format=sarif-latest --output=codeql-results-js.sarif",
    "group": "test",
    "problemMatcher": [],
    "presentation": {
        "reveal": "always",
        "panel": "new"
    }
},
{
    "label": "Security: CodeQL SSRF Check",
    "type": "shell",
    "command": "codeql database create codeql-db-go --language=go --source-root=backend --overwrite && codeql database analyze codeql-db-go /projects/codeql/codeql/go/ql/src/Security/CWE-918 --format=sarif-latest --output=codeql-ssrf.sarif && echo 'Results in codeql-ssrf.sarif'",
    "group": "test",
    "problemMatcher": []
}
```

---

## 7. Definition of Done Updates

Update `.github/instructions/copilot-instructions.md` Task Completion Protocol:

```markdown
## ✅ Task Completion Protocol (Definition of Done)

1. **Security Scans**: Run all security scans and ensure zero vulnerabilities.
    - **CodeQL**: Run VS Code task "Security: CodeQL Go Scan" or "Security: CodeQL JS Scan"
      - View results in SARIF Viewer extension
      - Zero high-severity findings allowed
      - Document any accepted risks with justification
    - **Trivy**: Run as VS Code task or use Skill.
    - **Zero issues allowed**.
```

---

## 8. Quick Reference Commands

```bash
# Full Go security scan
codeql database create codeql-db-go --language=go --source-root=backend --overwrite
codeql database analyze codeql-db-go /projects/codeql/codeql/go/ql/src/codeql-suites/go-security-extended.qls --format=sarif-latest --output=codeql-results-go.sarif

# Full JS security scan
codeql database create codeql-db-js --language=javascript --source-root=frontend --overwrite
codeql database analyze codeql-db-js /projects/codeql/codeql/javascript/ql/src/codeql-suites/javascript-security-extended.qls --format=sarif-latest --output=codeql-results-js.sarif

# Check SSRF only
codeql database analyze codeql-db-go /projects/codeql/codeql/go/ql/src/Security/CWE-918 --format=sarif-latest --output=codeql-ssrf.sarif

# View results count
jq '.runs[0].results | length' codeql-results-go.sarif
```

---

## 9. Existing SARIF Files in Charon

| File | Purpose | Last Run |
|------|---------|----------|
| `codeql-go.sarif` | Go backend analysis | 2025-11-29 |
| `codeql-js.sarif` | JS frontend analysis | - |
| `codeql-results-go.sarif` | Go results | - |
| `codeql-results-go-backend.sarif` | Backend-specific | - |
| `codeql-results-go-new.sarif` | Updated results | - |
| `codeql-results-js.sarif` | JS results | - |

---

## 10. CI Workflow Reference

The existing `.github/workflows/codeql.yml` runs CodeQL on:

- Push to `main`, `development`, `feature/**`
- Pull requests to `main`, `development`
- Weekly schedule (Monday 3am)

Languages scanned: `go`, `javascript-typescript`

---

## Next Steps

1. [ ] Install CodeQL CLI or VS Code extension
2. [ ] Add VS Code tasks from Section 6
3. [ ] Run initial scans and triage existing findings
4. [ ] Fix CWE-918 SSRF issue in notification_service.go
5. [ ] Update copilot-instructions.md with CodeQL in DoD
