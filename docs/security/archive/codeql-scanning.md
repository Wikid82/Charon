# CodeQL Security Scanning Guide

## Overview

Charon uses GitHub's CodeQL for static application security testing (SAST). CodeQL analyzes code to find security vulnerabilities and coding errors.

## Quick Start

### Run CodeQL Locally (CI-Aligned)

**Via VS Code Tasks:**

1. Open Command Palette (`Ctrl+Shift+P` / `Cmd+Shift+P`)
2. Type "Tasks: Run Task"
3. Select:
   - `Security: CodeQL Go Scan (CI-Aligned)` - Scan backend
   - `Security: CodeQL JS Scan (CI-Aligned)` - Scan frontend
   - `Security: CodeQL All (CI-Aligned)` - Scan both

**Via Pre-Commit:**

```bash
# Quick security check (govulncheck - 5s)
pre-commit run security-scan --all-files

# Full CodeQL scan (2-3 minutes)
pre-commit run codeql-go-scan --all-files
pre-commit run codeql-js-scan --all-files
pre-commit run codeql-check-findings --all-files
```

**Via Command Line:**

```bash
# Go scan
codeql database create codeql-db-go --language=go --source-root=backend --overwrite
codeql database analyze codeql-db-go \
  codeql/go-queries:codeql-suites/go-security-and-quality.qls \
  --format=sarif-latest --output=codeql-results-go.sarif

# JavaScript/TypeScript scan
codeql database create codeql-db-js --language=javascript --source-root=frontend --overwrite
codeql database analyze codeql-db-js \
  codeql/javascript-queries:codeql-suites/javascript-security-and-quality.qls \
  --format=sarif-latest --output=codeql-results-js.sarif
```

### View Results

**Method 1: VS Code SARIF Viewer (Recommended)**

1. Install extension: `MS-SarifVSCode.sarif-viewer`
2. Open `codeql-results-go.sarif` or `codeql-results-js.sarif`
3. Navigate findings with inline annotations

**Method 2: Command Line (jq)**

```bash
# Summary
jq '.runs[].results | length' codeql-results-go.sarif

# Details
jq -r '.runs[].results[] | "\(.level): \(.message.text) (\(.locations[0].physicalLocation.artifactLocation.uri):\(.locations[0].physicalLocation.region.startLine))"' codeql-results-go.sarif
```

**Method 3: GitHub Security Tab**

- CI automatically uploads results to: `https://github.com/YourOrg/Charon/security/code-scanning`

## Understanding Query Suites

Charon uses the **security-and-quality** suite (GitHub Actions default):

| Suite | Go Queries | JS Queries | Use Case |
|-------|-----------|-----------|----------|
| `security-extended` | 39 | 106 | Security-only, faster |
| `security-and-quality` | 61 | 204 | Security + quality, comprehensive (CI default) |

⚠️ **Important:** Local scans MUST use `security-and-quality` to match CI behavior.

## Severity Levels

- 🔴 **Error (High/Critical):** Must fix before merge - CI will fail
- 🟡 **Warning (Medium):** Should fix - CI continues
- 🔵 **Note (Low/Info):** Consider fixing - CI continues

## Common Issues & Fixes

### Issue: "CWE-918: Server-Side Request Forgery (SSRF)"

**Location:** `backend/internal/api/handlers/url_validator.go`

**Fix:**

```go
// BAD: Unrestricted URL
resp, err := http.Get(userProvidedURL)

// GOOD: Validate against allowlist
if !isAllowedHost(userProvidedURL) {
    return ErrSSRFAttempt
}
resp, err := http.Get(userProvidedURL)
```

**Reference:** [docs/security/ssrf-protection.md](ssrf-protection.md)

### Issue: "CWE-079: Cross-Site Scripting (XSS)"

**Location:** `frontend/src/components/...`

**Fix:**

```typescript
// BAD: Unsafe HTML rendering
element.innerHTML = userInput;

// GOOD: Safe text content
element.textContent = userInput;

// GOOD: Sanitized HTML (if HTML is required)
import DOMPurify from 'dompurify';
element.innerHTML = DOMPurify.sanitize(userInput);
```

### Issue: "CWE-089: SQL Injection"

**Fix:** Use parameterized queries (GORM handles this automatically)

```go
// BAD: String concatenation
db.Raw("SELECT * FROM users WHERE name = '" + userName + "'")

// GOOD: Parameterized query
db.Where("name = ?", userName).Find(&users)
```

## CI/CD Integration

### When CodeQL Runs

- **Push:** Every commit to `main`, `development`, `feature/*`
- **Pull Request:** Every PR to `main`, `development`
- **Schedule:** Weekly scan on Monday at 3 AM UTC

### CI Behavior

✅ **Allowed to merge:**

- No findings
- Only warnings/notes
- Forked PRs (security scanning skipped for permission reasons)

❌ **Blocked from merge:**

- Any error-level (high/critical) findings
- CodeQL analysis failure

### Viewing CI Results

1. **PR Checks:** See "CodeQL analysis (go)" and "CodeQL analysis (javascript-typescript)" checks
2. **Security Tab:** Navigate to repo → Security → Code scanning alerts
3. **Workflow Summary:** Click on failed check → View job summary

## Troubleshooting

### "CodeQL passes locally but fails in CI"

**Cause:** Using wrong query suite locally

**Fix:** Ensure tasks use `security-and-quality`:

```bash
codeql database analyze DB_PATH \
  codeql/LANGUAGE-queries:codeql-suites/LANGUAGE-security-and-quality.qls \
  ...
```

### "SARIF file not found"

**Cause:** Database creation or analysis failed

**Fix:**

1. Check terminal output for errors
2. Ensure CodeQL is installed: `codeql version`
3. Verify source-root exists: `ls backend/` or `ls frontend/`

### "Too many findings to fix"

**Strategy:**

1. Fix all **error** level first (CI blockers)
2. Create issues for **warning** level (non-blocking)
3. Document **note** level for future consideration

**Suppress false positives:**

```go
// codeql[go/sql-injection] - Safe: input is validated by ACL
db.Raw(query).Scan(&results)
```

## Performance Tips

- **Incremental Scans:** CodeQL caches databases, second run is faster
- **Parallel Execution:** Use `--threads=0` for auto-detection
- **CI Only:** Run full scans in CI, quick checks locally

## References

- [CodeQL Documentation](https://codeql.github.com/docs/)
- [OWASP Top 10](https://owasp.org/www-project-top-ten/)
- [CWE Database](https://cwe.mitre.org/)
- [Charon Security Policy](../SECURITY.md)
