# Security: CodeQL Scan

Run CodeQL static analysis for Go and JavaScript/TypeScript.

## Command

```bash
.github/skills/scripts/skill-runner.sh security-scan-codeql
```

## What It Scans

- **Go**: Backend code in `backend/` — injection, path traversal, auth issues, etc.
- **JavaScript/TypeScript**: Frontend code in `frontend/` — XSS, injection, prototype pollution, etc.

## CI Alignment

Uses the same configuration as the CI `codeql.yml` workflow and `.github/codeql/codeql-config.yml`.

## On Findings

For each finding:
1. Read the finding details — understand what code path is flagged
2. Determine if it's a true positive or false positive
3. Fix true positives immediately (these are real vulnerabilities)
4. Document false positives with rationale in the CodeQL config

## Related

- `/security-scan-trivy` — Container and dependency scanning
- `/security-scan-gorm` — GORM-specific SQL security scan
- `/supply-chain-remediation` — Fix dependency vulnerabilities
