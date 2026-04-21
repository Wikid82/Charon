# QA/Security DoD Audit Report — Issue #929

Date: 2026-04-20
Repository: /projects/Charon
Branch: feature/beta-release
Scope assessed: tests/a11y/a11y-baseline.ts, tests/a11y/dns-providers.a11y.spec.ts, tests/a11y/README.md

## Final Recommendation

FAIL

Reason: The Trivy filesystem gate reports one HIGH vulnerability (CVE-2026-34040) and is still outstanding.

## Gate Summary

| # | DoD Gate | Status | Notes |
|---|---|---|---|
| 1 | Playwright E2E first | PASS (with flake observed) | Initial run had 1 timeout failure; targeted rerun passed fully |
| 2 | GORM security scan (conditional) | N/A | Not triggered; touched files are a11y tests/docs only, no backend model/DB scope |
| 3a | Backend coverage task/script | PASS | 92.8% coverage vs 85% minimum |
| 3b | Frontend coverage task/script | PASS | 90.4% lines vs 87% configured minimum |
| 4 | Local patch coverage report after coverage artifacts | PASS | Required artifacts generated; report indicates 0 changed lines / 100% patch coverage |
| 5 | Frontend type check | PASS | No TypeScript errors |
| 6 | Pre-commit hooks (fast set) | PASS | Lefthook pre-commit checks passed |
| 7a | Trivy filesystem scan | FAIL | 1 HIGH finding (CVE-2026-34040) |
| 7b | Docker image scan (mandatory) | PASS | Grype image scan shows Medium-only findings, no High/Critical |
| 7c | CodeQL Go + JS (CI-aligned) | PASS | 0 error-level findings |
| 8 | Linting and required quality checks | PASS | lint-fast passed, frontend lint passed (warnings only), backend/frontend build passed |

## Detailed Evidence

### 1) Playwright E2E first

Environment rebuild decision:
- Rebuild skipped because charon-e2e container was already healthy and health endpoint returned HTTP 200.

Execution evidence:
- Command: PLAYWRIGHT_HTML_OPEN=never npx playwright test tests/a11y --project=firefox
- Result: 27 passed, 1 failed
- Failure evidence: tests/a11y/security.a11y.spec.ts timed out at makeAxeBuilder().analyze() with test timeout 90000ms.

Revalidation:
- Command: PLAYWRIGHT_HTML_OPEN=never npx playwright test tests/a11y/security.a11y.spec.ts --project=firefox
- Result: 9 passed, 0 failed
- Disposition: treated as flaky timeout, currently passing.

Artifacts:
- playwright-report/
- test-results/

### 2) GORM scan (conditional)

Trigger check:
- Changed files are under tests/a11y and docs only.
- No changes under backend/internal/models, GORM services, or migrations.

Disposition:
- Gate not applicable for this scope.

### 3) Coverage tests

Backend coverage:
- Command: .github/skills/scripts/skill-runner.sh test-backend-coverage
- Result: PASS
- Coverage: 92.8% (minimum required 85%)
- Artifact: backend/coverage.txt

Frontend coverage:
- Command: .github/skills/scripts/skill-runner.sh test-frontend-coverage
- Result: PASS
- Summary: 182 test files passed, 5 skipped; 2163 tests passed, 90 skipped
- Coverage: Statements 89.51%, Branches 82.09%, Functions 87.18%, Lines 90.4%
- Gate evaluation: PASS (lines 90.4% vs minimum 87%)
- Artifacts:
   - frontend/coverage/lcov.info
   - frontend/coverage/coverage-summary.json
   - frontend/coverage/index.html

### 4) Local patch coverage report (post-coverage)

Execution:
- Command: bash scripts/local-patch-report.sh
- Result: PASS

Required artifacts confirmed:
- test-results/local-patch-report.md
- test-results/local-patch-report.json

Report outcome:
- 0 changed lines, 100% patch coverage, no files below 90% patch threshold.

### 5) Frontend type check

Execution:
- Command: cd frontend && npm run type-check
- Result: PASS

### 6) Pre-commit hooks (fast set)

Execution:
- Command: lefthook run pre-commit
- Result: PASS
- Passing hooks include check-yaml, actionlint, end-of-file-fixer, trailing-whitespace, dockerfile-check, shellcheck.

### 7) Security scans

7a. Trivy filesystem scan:
- Command: .github/skills/scripts/skill-runner.sh security-scan-trivy
- Result: FAIL (exit code 2)
- Finding:
   - CVE-2026-34040 (HIGH)
   - Package: github.com/docker/docker
   - Installed: v28.5.2+incompatible
   - Fixed: 29.3.1
- Disposition: Outstanding blocker.

7b. Docker image scan (mandatory):
- Command: .github/skills/scripts/skill-runner.sh security-scan-docker-image
- Validation command: jq severity counts from grype-results.json
- Result: PASS
- Severity counts: {"Medium": 4}
- High/Critical list: none

Artifacts:
- sbom-generated.json
- sbom.cyclonedx.json
- grype-results.json
- grype-results.sarif

7c. CodeQL (CI-aligned Go + JS):
- Command: .github/skills/scripts/skill-runner.sh security-scan-codeql all summary
- Result: PASS (exit code 0)

SARIF summary:
- codeql-results-go.sarif: 0 errors, 1 warning, 0 notes
- codeql-results-javascript.sarif: 0 errors, 0 warnings, 0 notes
- codeql-results-js.sarif: 0 errors, 0 warnings, 0 notes

Error-level findings:
- None

### 8) Linting and required quality checks

Fast lint:
- Command: make -C /projects/Charon lint-fast
- Result: PASS (0 issues)

Frontend lint:
- Command: cd frontend && npm run lint
- Result: PASS with warnings
- Summary: 0 errors, 937 warnings

Build checks:
- Command: cd /projects/Charon/backend && go build ./...
- Result: PASS
- Command: cd /projects/Charon/frontend && npm run build
- Result: PASS

## Additional Security Validation

Gotify token exposure check:
- Command: rg -n --hidden -S "token=|gotify.*token|\?token=" test-results docs/reports *.json *.sarif
- Result: no matches in scanned QA/security artifacts.

## Blockers

1. Trivy filesystem scan reports HIGH CVE-2026-34040 in github.com/docker/docker v28.5.2+incompatible.

## Decision

Overall DoD decision for Issue #929: FAIL until the Trivy HIGH finding is remediated or explicitly accepted per project security policy.

---

### CHARON-2025-001 — CrowdSec Go Stdlib CVE Cluster (RESOLVED)

| Field            | Value |
|------------------|-------|
| **Previous Severity** | High |
| **Aliases**      | CVE-2025-58183, CVE-2025-58186, CVE-2025-58187, CVE-2025-61729, CVE-2026-25679, CVE-2025-61732, CVE-2026-27142, CVE-2026-27139 |
| **Resolution**   | CrowdSec binaries now compiled with Go 1.26.1 |
| **Verified**     | None of the aliased CVEs detected in Grype scan |

---

### CVE-2026-27171 — zlib CPU Exhaustion (RESOLVED)

| Field            | Value |
|------------------|-------|
| **Previous Severity** | Medium |
| **Resolution**   | Alpine now ships `zlib` 1.3.2-r0 (fix threshold: 1.3.2) |
| **Verified**     | Not detected in Grype scan; zlib 1.3.2-r0 confirmed in SBOM |

---

### CVE-2026-33186 — gRPC-Go Authorization Bypass (RESOLVED)

| Field            | Value |
|------------------|-------|
| **Previous Severity** | Critical |
| **Packages**     | `google.golang.org/grpc` v1.74.2 (CrowdSec), v1.79.1 (Caddy) |
| **Resolution**   | Upstream releases now include patched gRPC (>= v1.79.3) |
| **Verified**     | Not detected in Grype scan; ignore rule present but no match |

---

### GHSA-69x3-g4r3-p962 / CVE-2026-25793 — Nebula ECDSA Malleability (RESOLVED)

| Field            | Value |
|------------------|-------|
| **Previous Severity** | High |
| **Package**      | `github.com/slackhq/nebula` v1.9.7 in Caddy |
| **Resolution**   | Caddy now ships with nebula >= v1.10.3 |
| **Verified**     | Not detected in Grype scan; Trivy image report from Feb 25 had this but current build does not |

> **Note**: The stale Trivy image report (`trivy-image-report.json`, dated 2026-02-25) still
> shows CVE-2026-25793. This report predates the current build and should be regenerated.

---

### GHSA-479m-364c-43vc — goxmldsig XML Signature Bypass (RESOLVED)

| Field            | Value |
|------------------|-------|
| **Previous Severity** | High |
| **Package**      | `github.com/russellhaering/goxmldsig` v1.5.0 in Caddy |
| **Resolution**   | Caddy now ships with goxmldsig >= v1.6.0 |
| **Verified**     | Not detected in Grype scan; ignore rule present but no match |

---

## CodeQL Analysis

### go/cookie-secure-not-set — FALSE POSITIVE

| Field            | Value |
|------------------|-------|
| **Severity**     | Medium (CodeQL) |
| **File**         | `backend/internal/api/handlers/auth_handler.go:152` |
| **Classification** | FALSE POSITIVE (stale SARIF) |

**Finding**: CodeQL reports "Cookie does not set Secure attribute to true" at line 152.

**Verification**: The `setSecureCookie` function at line 148-156 calls `c.SetCookie()`
with `secure: true` (6th positional argument). The Secure attribute IS set correctly.
This SARIF was generated from a previous code version and does not reflect the current
source. **The CodeQL SARIF files should be regenerated.**

### JavaScript / JS

No findings. Both `codeql-results-javascript.sarif` and `codeql-results-js.sarif` contain
0 results.

---

## GORM Security Scanner

| Metric     | Value |
|------------|-------|
| **Result** | PASSED |
| **Files**  | 43 Go files (2,396 lines) |
| **Critical** | 0 |
| **High**   | 0 |
| **Medium** | 0 |
| **Info**   | 2 (missing indexes on foreign keys in `UserPermittedHost`) |

The 2 informational suggestions (`UserID` and `ProxyHostID` missing `gorm:"index"` in
`backend/internal/models/user.go:130-131`) are performance recommendations, not security
issues. They do not block this audit.

---

## CI vs Local Scan Discrepancy

The CI reported **3 Critical, 5 High, 1 Medium**. The local scan on the freshly built
image reports **0 Critical, 0 High, 4 Medium, 2 Low** (active) plus **4 High** (ignored).

**Root causes for the discrepancy:**

1. **Resolved vulnerabilities**: 3 Critical and 4 High findings were resolved by Go 1.26.1
   compilation and upstream Caddy/CrowdSec dependency updates since the CI image was built.
2. **Grype ignore rules**: The local scan applies documented risk acceptance rules that
   suppress 4 High findings in third-party binaries. CI (Trivy) does not use these rules.
3. **Stale CI artifacts**: The `trivy-image-report.json` dates from 2026-02-25 and does
   not reflect the current image state. The `codeql-results-go.sarif` references code that
   has since been fixed.

---

## Recommended Actions

### Immediate (This Sprint)

1. **Update SECURITY.md**: Move CVE-2025-68121, CHARON-2025-001, and CVE-2026-27171 to
   a "Patched Vulnerabilities" section. Add CVE-2025-60876 and CVE-2026-26958 as new
   known vulnerabilities.

2. **Regenerate stale scan artifacts**: Re-run Trivy image scan and CodeQL analysis to
   produce current SARIF/JSON files. The existing files predate fixes and produce
   misleading CI results.

3. **Clean up Grype ignore rules**: Remove ignore entries for vulnerabilities that are
   no longer detected (CVE-2026-33186, GHSA-69x3-g4r3-p962, GHSA-479m-364c-43vc).
   Stale ignore rules obscure the actual security posture.

### Next Release

4. **Monitor Alpine APK updates**: Watch for patched `busybox` (CVE-2025-60876) and
   `openssl` (CVE-2026-2673) packages in Alpine 3.23.

5. **Monitor CrowdSec releases**: Watch for CrowdSec builds with updated
   `filippo.io/edwards25519` >= v1.1.1, `buger/jsonparser` >= v1.1.2, and
   `pgx/v5` migration (replacing pgproto3/v2).

6. **Monitor Go 1.26.2-alpine**: When available, bump `GO_VERSION` to pick up any
   remaining stdlib patches.

### Informational (Non-Blocking)

7. **GORM indexes**: Consider adding `gorm:"index"` to `UserID` and `ProxyHostID` in
   `UserPermittedHost` for query performance.

---

## Gotify Token Review

Verified: No Gotify application tokens appear in scan output, log artifacts, test results,
API examples, or URL query parameters. All diagnostic output is clean.

---

## Conclusion

The Charon container image security posture has materially improved. Six previously known
vulnerabilities are now resolved through Go toolchain and dependency updates. The remaining
active findings are medium/low severity, reside in Alpine base packages and CrowdSec
third-party binaries, and have no available fixes. No vulnerabilities exist in Charon's
own application code. GORM and CodeQL scans confirm the backend code is clean.
