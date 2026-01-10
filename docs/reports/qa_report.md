# QA/Security DoD Validation Report

**Date**: 2026-01-09
**Scope**: DoD validation rerun (backend tests + lint + security scans)
**Overall Status**: ❌ FAIL

## Summary

All requested tasks completed successfully (no task execution failures). However, DoD fails due to **HIGH/CRITICAL security findings** in CodeQL and Trivy outputs.

## Frontend Change Check

**Result**: No frontend files detected as changed (no paths under `frontend/` in current workspace changes).

**Action**: Per request, skipped:
- Test: Frontend with Coverage
- Lint: TypeScript Check

Note: the pre-commit run includes a frontend TypeScript check hook, but it is not a substitute for the explicit “Frontend with Coverage” task if frontend source changes are present.

## Task Results (Required)

### 1) Test: Backend with Coverage

**Pass/Fail Criteria**:
- PASS if task exits successfully and produces a coverage result.

**Result**: ✅ PASS (task completed)

**Coverage**:
- Backend total coverage (from `go tool cover -func backend/coverage.txt`): **86.6%**
- Task output included: `coverage: 63.2% of statements` (package `backend/cmd/seed`)

### 2) Lint: Pre-commit (All Files)

**Pass/Fail Criteria**:
- PASS if all hooks complete successfully.

**Result**: ✅ PASS

### 3) Security: CodeQL All (CI-Aligned)

**Pass/Fail Criteria**:
- PASS if no HIGH/CRITICAL findings are present.

**Result**: ❌ FAIL

**Findings**:
- Go SARIF (`codeql-results-go.sarif`): **3 CRITICAL** (security severity 9.8)
	- Rule: `go/email-injection` (“Email content injection”)
	- Location: `backend/internal/services/mail_service.go` (lines ~222, ~340, ~393)
- JS SARIF (`codeql-results-js.sarif`): **1 HIGH** (security severity 7.8)
	- Rule: `js/incomplete-hostname-regexp` (“Incomplete regular expression for hostnames”)
	- Location: `frontend/src/pages/__tests__/ProxyHosts-extra.test.tsx` (line ~252)

### 4) Security: Trivy Scan

**Pass/Fail Criteria**:
- PASS if no HIGH/CRITICAL findings are present.

**Result**: ❌ FAIL

**Counts (from existing artifacts)**:
- `trivy-scan-output.txt`: **CRITICAL=1**, **HIGH=7**
- `trivy-image-scan.txt`: **CRITICAL=0**, **HIGH=1**

## Root Cause (Why DoD Failed)

### CodeQL

1) **CRITICAL** `go/email-injection` in `backend/internal/services/mail_service.go`

**Likely cause**: user-controlled or otherwise untrusted values are being used to build email content (and potentially headers) without robust validation/normalization, enabling header/body injection (e.g., newline injection).

2) **HIGH** `js/incomplete-hostname-regexp` in a frontend test

**Likely cause**: a regex used for host matching in tests does not escape `.`, so it matches more than intended.

### Trivy

**Likely cause**: one or more dependencies in the repo (Go modules and/or image contents) are pinned to vulnerable versions.

Examples extracted from `trivy-scan-output.txt` / `trivy-image-scan.txt` include (non-exhaustive):
- `golang.org/x/crypto` (CVE-2024-45337 CRITICAL; CVE-2025-22869 HIGH)
- `golang.org/x/net` (CVE-2023-39325 HIGH)
- `golang.org/x/oauth2` (CVE-2025-22868 HIGH)
- `gopkg.in/yaml.v3` (CVE-2022-28948 HIGH)
- `github.com/quic-go/quic-go` (CVE-2025-59530 HIGH)
- `github.com/expr-lang/expr` (CVE-2025-68156 HIGH)

## Proposed Remediation (No changes applied)

Per instruction: **no fixes were made**. Suggested remediation steps:

### For CodeQL `go/email-injection`

- Validate/normalize any untrusted values used in mail headers/body (especially ensuring values do not contain `\r`/`\n`).
- Use strict email address parsing/validation (e.g., Go `net/mail`) and explicit header encoding.
- Ensure subject/from/to/reply-to fields are constructed via safe libraries and reject control characters.

### For CodeQL `js/incomplete-hostname-regexp`

- Update the test regex to escape `.` and/or use a safer matcher; rerun CodeQL JS scan.

### For Trivy findings

- Upgrade impacted Go modules to versions containing fixes (follow Trivy “Fixed Version” guidance) and run `go mod tidy`.
- Re-run Trivy scan after dependency upgrades.
- If image findings remain: rebuild the image after base image upgrades and/or OS package updates.

## Artifacts

- Backend coverage profile: `backend/coverage.txt`
- CodeQL results: `codeql-results-go.sarif`, `codeql-results-js.sarif`, `codeql-results-javascript.sarif`
- Trivy results: `trivy-scan-output.txt`, `trivy-image-scan.txt`

## Trivy triage (2026-01-10)

**Task rerun**: VS Code task “Security: Trivy Scan”

**Primary artifact (current task output)**:
- `.trivy_logs/trivy-report.txt`

**What the task is actually scanning**:
- Image scan only (`trivy image --severity CRITICAL,HIGH charon:local`), not a filesystem/repo scan.

**Current HIGH/CRITICAL summary (from `.trivy_logs/trivy-report.txt`)**:
- **CRITICAL=0, HIGH=8**
- All HIGH findings are in **built image contents**, specifically:
	- `usr/local/bin/crowdsec` (**HIGH=4**) and `usr/local/bin/cscli` (**HIGH=4**)
	- Vulnerabilities are attributed to **Go stdlib** in those binaries (built with Go `v1.25.1`):
		- `CVE-2025-58183`, `CVE-2025-58186`, `CVE-2025-58187`, `CVE-2025-61729`

**Attribution**:
- Repo-tracked source paths: **none** (this task does not scan the repo filesystem)
- Generated artifacts/caches: **none** (this task does not scan the repo filesystem)
- Built image contents: **YES** (CrowdSec binaries embed vulnerable Go stdlib)

**What must be fixed next (no fixes applied here)**:
- **Dockerfile/CrowdSec bump**: update the CrowdSec build stage/version/toolchain so `crowdsec` and `cscli` are built with a Go version that includes the fixes (per Trivy, fixed in Go `1.25.2+`, `1.25.3+`, and `1.25.5+` depending on CVE), then rebuild `charon:local` and rerun Trivy.
- If DoD is intended to gate repo dependencies too, consider **scan-scope alignment** (add a separate Trivy filesystem scan of repo-tracked paths with excludes for workspace caches like `.cache/`, `codeql-db*/`, and scan outputs).
