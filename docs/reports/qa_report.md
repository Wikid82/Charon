# QA/Security Audit Report: `security-pr.yml` Workflow Fix

- Date: 2026-02-27
- Auditor: QA Security mode
- Scope: `.github/workflows/security-pr.yml` behavior fix only
- Overall verdict: **PASS (scope-specific)** with one **out-of-scope repository security debt** noted

## Findings (Ordered by Severity)

### 🟡 IMPORTANT: Repository secret-scan debt exists (not introduced by scoped workflow change)
- Check: `pre-commit run --hook-stage manual gitleaks-tuned-scan --all-files`
- Result: **FAIL** (`135` findings)
- Scope impact: `touches_security_pr = 0` (no findings in `.github/workflows/security-pr.yml`)
- Evidence source: `test-results/security/gitleaks-tuned-precommit.json`
- Why this matters: Existing credential-like content raises background security risk even if unrelated to this workflow fix.
- Recommended remediation:
  1. Triage findings by rule/file and classify true positives vs allowed test fixtures.
  2. Add justified allowlist entries for confirmed false positives.
  3. Remove or rotate any real secrets immediately.
  4. Re-run `gitleaks-tuned-scan` until clean/accepted baseline is documented.

### ✅ No blocking defects found in the implemented workflow fix
- Deterministic event handling: validated in workflow logic.
- Artifact/image resolution hardening: validated in workflow logic.
- Security hardening: validated in workflow logic and lint gates.

## Requested Validations

### 1) `actionlint` on security workflow
- Command:
  - `pre-commit run actionlint --files .github/workflows/security-pr.yml`
- Result: **PASS**
- Key output:
  - `actionlint (GitHub Actions)..............................................Passed`

### 2) `pre-commit run --all-files`
- Command:
  - `pre-commit run --all-files`
- Result: **PASS**
- Key output:
  - YAML/shell/actionlint/dockerfile/go vet/golangci-lint/version/LFS/type-check/frontend lint hooks passed.

### 3) Security scans/tasks relevant to workflow change (feasible locally)
- Executed:
  1. `pre-commit run --hook-stage manual codeql-parity-check --all-files` -> **PASS**
  2. `pre-commit run --hook-stage manual codeql-check-findings --all-files` -> **PASS** (no blocking HIGH/CRITICAL)
  3. `pre-commit run --hook-stage manual gitleaks-tuned-scan --all-files` -> **FAIL** (repo baseline debt; not in scoped file)
- Additional QA evidence:
  - `bash scripts/local-patch-report.sh` -> artifacts generated:
    - `test-results/local-patch-report.md`
    - `test-results/local-patch-report.json`

## Workflow Behavior Verification

## A) Deterministic event handling
Validated in `.github/workflows/security-pr.yml`:
- Manual dispatch input is required and validated as digits-only:
  - `.github/workflows/security-pr.yml:10`
  - `.github/workflows/security-pr.yml:14`
  - `.github/workflows/security-pr.yml:71`
  - `.github/workflows/security-pr.yml:78`
- `workflow_run` path constrained to successful upstream PR runs:
  - `.github/workflows/security-pr.yml:31`
  - `.github/workflows/security-pr.yml:36`
  - `.github/workflows/security-pr.yml:38`
- Explicit trust-boundary contract checks for upstream workflow name/event/repository:
  - `.github/workflows/security-pr.yml:127`
  - `.github/workflows/security-pr.yml:130`
  - `.github/workflows/security-pr.yml:136`
  - `.github/workflows/security-pr.yml:143`

Assessment: **PASS** for deterministic triggering and contract enforcement.

## B) Artifact and image resolution hardening
Validated in `.github/workflows/security-pr.yml`:
- Artifact is mandatory in `workflow_run`/`workflow_dispatch` artifact path; failures are explicit (`api_error`/`not_found`):
  - `.github/workflows/security-pr.yml:159`
  - `.github/workflows/security-pr.yml:185`
  - `.github/workflows/security-pr.yml:196`
  - `.github/workflows/security-pr.yml:214`
  - `.github/workflows/security-pr.yml:225`
- Docker image load hardened with:
  - tar readability check
  - `manifest.json` multi-tag parsing (`RepoTags[]`)
  - fallback to `Loaded image ID`
  - deterministic alias `charon:artifact`
  - `.github/workflows/security-pr.yml:255`
  - `.github/workflows/security-pr.yml:261`
  - `.github/workflows/security-pr.yml:267`
  - `.github/workflows/security-pr.yml:273`
  - `.github/workflows/security-pr.yml:282`
  - `.github/workflows/security-pr.yml:295`
  - `.github/workflows/security-pr.yml:300`
- Extraction consumes resolved alias output rather than reconstructed tag:
  - `.github/workflows/security-pr.yml:333`
  - `.github/workflows/security-pr.yml:342`

Assessment: **PASS** for deterministic artifact/image selection and prior mismatch risk mitigation.

## C) Security hardening
Validated in `.github/workflows/security-pr.yml`:
- Least-privilege job permissions:
  - `.github/workflows/security-pr.yml:40`
  - `.github/workflows/security-pr.yml:41`
  - `.github/workflows/security-pr.yml:42`
  - `.github/workflows/security-pr.yml:43`
- Pinned action SHAs maintained for checkout/download/upload/CodeQL SARIF upload/Trivy action usage:
  - `.github/workflows/security-pr.yml:48`
  - `.github/workflows/security-pr.yml:243`
  - `.github/workflows/security-pr.yml:365`
  - `.github/workflows/security-pr.yml:388`
  - `.github/workflows/security-pr.yml:397`
  - `.github/workflows/security-pr.yml:408`

Assessment: **PASS** for workflow-level security hardening within scope.

## DoD Mapping for Workflow-Only Change

Executed:
- `actionlint` scoped check: **Yes (PASS)**
- Full pre-commit: **Yes (PASS)**
- Workflow-relevant security manual checks (CodeQL parity/findings, gitleaks): **Yes (2 PASS, 1 FAIL out-of-scope debt)**
- Local patch report artifacts: **Yes (generated)**

N/A for this scope:
- Playwright E2E feature validation for app behavior: **N/A** (no app/runtime code changes)
- Backend/frontend unit coverage gates: **N/A** (no backend/frontend source modifications in audited fix)
- GORM check-mode gate: **N/A** (no model/database/GORM changes)
- Trivy app binary/image scan execution for changed runtime artifact: **N/A locally for this audit** (workflow logic audited; no image/runtime code delta in this fix)

## Conclusion
The implemented fix in `.github/workflows/security-pr.yml` meets the requested goals for deterministic event handling, robust artifact/image resolution, and workflow security hardening. Required validation commands were executed and passed (`actionlint`, `pre-commit --all-files`), and additional feasible security checks were run. One repository-wide gitleaks debt remains and should be remediated separately from this workflow fix.
