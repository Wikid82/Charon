# QA/Security Patch-Coverage Remediation Report

**Date (UTC):** 2026-01-02
**Agent:** QA_Security
**Scope:** Verification-only for the patch-coverage remediation task (no code changes performed as part of this audit)

**Branch:** `feature/beta-release`
**Commit:** `8f15fdd97f0e3c80afdf25436b00195d46fc92d0`

**Overall Status:** ❌ **FAIL**

Reason for FAIL (minimal):
1. CodeQL Go SARIF contains **CRITICAL/HIGH** findings.
2. CodeQL JS SARIF contains **HIGH** findings, including at least one from a **generated artifact present during the scan** (a coverage report output).

---

## Mandatory Task Results (rerun)

| Task | Result | Evidence (fresh rerun) |
|------|--------|------------------------|
| `shell: Test: Backend with Coverage` | ✅ PASS | Backend total coverage: **87.5%** (`go tool cover -func backend/coverage.txt`). |
| `shell: Lint: Pre-commit (All Files)` | ✅ PASS | `test-results/precommit-full.log` ends with: `[SUCCESS] All pre-commit hooks passed`. |
| `shell: Security: CodeQL Go Scan (CI-Aligned) [~60s]` | ❌ FAIL | Output SARIF: `codeql-results-go.sarif` (contains CRITICAL/HIGH). |
| `shell: Security: CodeQL JS Scan (CI-Aligned) [~90s]` | ❌ FAIL | Output SARIF: `codeql-results-js.sarif` (contains HIGH). |
| `shell: Security: Trivy Scan` | ✅ PASS | Skill summary: `[SUCCESS] Trivy scan completed - no issues found`. |

---

## CodeQL HIGH/CRITICAL Findings — Source Classification

Classification method:
- Use SARIF locations + `git diff -U0 main...HEAD` hunk ranges.
- Categories:
  - **A) Generated artifacts present during scan** (coverage reports / built assets / SARIF outputs; may be tracked or untracked)
  - **B) Pre-existing vs main** (unchanged file, or finding outside patch hunks)
  - **C) Introduced by this PR** (finding location intersects a patch hunk)

### CodeQL JS (HIGH)

 - **A) Generated artifacts present during scan**
  - `js/xss-through-dom`: `frontend/coverage/lcov-report/sorter.js:116`
  - Note: `frontend/coverage/` appears to be a generated output and is currently **untracked** in this workspace, but it was still included in the local CodeQL scan.
- **C) Introduced by this PR (within patch hunks)**
  - `js/regex/missing-regexp-anchor`: `frontend/src/pages/__tests__/UsersPage.test.tsx:390`, `:447`
- **B) Pre-existing vs main (unchanged or outside patch hunks)**
  - `js/regex/missing-regexp-anchor`: `frontend/src/pages/__tests__/ProxyHosts-progress.test.tsx:138`
  - `js/incomplete-hostname-regexp` and `js/regex/missing-regexp-anchor`: `frontend/src/pages/__tests__/ProxyHosts-extra.test.tsx:252` (file changed in PR, but finding is outside patch hunks)

### CodeQL Go (CRITICAL/HIGH)

- **C) Introduced by this PR (within patch hunks)**
  - `go/request-forgery`: `backend/internal/utils/url_testing.go:276`
  - `go/log-injection`: includes locations within patch hunks (example: `backend/internal/api/handlers/docker_handler.go:59`, `:74`)
- **B) Pre-existing vs main (unchanged or outside patch hunks)**
  - `go/request-forgery`: `backend/internal/services/notification_service.go:374` (file changed in PR, but location is outside patch hunks)
  - `go/email-injection`: `backend/internal/services/mail_service.go:222`, `:340`, `:393` (file changed in PR, but locations are outside patch hunks)
  - `go/log-injection`: many additional locations are outside patch hunks (e.g., `backend/internal/api/handlers/backup_handler.go:77`)

---

## Coverage Verification

### Backend Overall Coverage (project total)

- **Total:** **87.5%** (meets threshold **≥85%**)
- **Profile:** `backend/coverage.txt`

### Patch-Coverage Risk (10 flagged backend files)

This repo’s patch-coverage work is scoped to the 10 files listed in `docs/plans/current_spec.md`. The line ranges below are the **remaining uncovered, patch-adjacent spans** previously extracted from local coverage inspection.

> Note: These ranges are kept as-is per request (verification/report only). They should be re-validated against Codecov patch view if the PR is still failing patch coverage.

#### Priority Order (what to fix first)

1. **P0 — External URL / SSRF surface**
   - `backend/internal/security/url_validator.go`
   - `backend/internal/network/safeclient.go`
   - `backend/internal/utils/url_testing.go`
2. **P1 — Security-relevant outbound notifications + request paths**
   - `backend/internal/services/notification_service.go`
   - `backend/internal/api/routes/routes.go`
   - `backend/internal/api/handlers/settings_handler.go`
3. **P2 — Cryptography + proxy/server config generation (broad blast radius)**
  - `backend/internal/crypto/encryption.go`
   - `backend/internal/caddy/config.go`
   - `backend/internal/caddy/manager.go`
4. **P3 — Remaining core services**
   - `backend/internal/services/dns_provider_service.go`

#### Remaining uncovered patch-adjacent spans (by file)

- `backend/internal/caddy/config.go`: 171-194, 249-251, 253-256, 262-264, 1458-1459
- `backend/internal/services/dns_provider_service.go`: 133-133, 152-154, 157-159, 175-177, 192-194, 212-214, 216-218, 233-235, 238-240, 248-250 (and additional spans)
- `backend/internal/caddy/manager.go`: 77-79, 100-102, 110-112, 116-118
- `backend/internal/utils/url_testing.go`: 24-26, 34-36, 80-80, 157-161, 178-180, 193-195, 197-199, 242-244, 256-258
- `backend/internal/network/safeclient.go`: 62-64, 216-218, 224-225, 236-238, 246-248, 282-283, 290-290
- `backend/internal/api/routes/routes.go`: 309-311, 426-426
- `backend/internal/services/notification_service.go`: 219-219, 221-221, 232-232, 235-236, 296-296, 310-310, 313-313, 315-315, 322-322, 344-344 (and additional spans)
- `backend/internal/crypto/encryption.go`: 40-42, 45-47, 51-53, 71-73, 76-78
- `backend/internal/api/handlers/settings_handler.go`: 243-246, 317-323
- `backend/internal/security/url_validator.go`: 42-43, 127-129, 160-162, 189-191, 263-263

---

## Blockers to Fix Next (smallest set)

1. **Exclude or clean generated artifacts before CodeQL runs** (at minimum `frontend/coverage/lcov-report/*` if present).
2. **Resolve CodeQL JS HIGH in tests introduced by this PR** (`UsersPage.test.tsx` anchored-regex findings).
3. **Resolve CodeQL Go request-forgery introduced by this PR** (`backend/internal/utils/url_testing.go:276`).
4. **Decide how to handle pre-existing HIGH/CRITICAL** (findings outside patch hunks): either remediate, or establish a baseline so patch-coverage-only work is not blocked by unrelated legacy findings.
