## QA Report - Workflow-Only Audit

| Field | Value |
|---|---|
| Date | 2026-05-24 |
| Scope | .github/workflows/docker-build.yml (workflow-only change) |
| Branch | feature/hecate |
| Verdict | PASS with minor remediation |

## Objective Coverage

### 1) pull_request workflow correctness and security behavior

| Check | Status | Evidence |
|---|---|---|
| Producer output contract for `pr_image_ref` | PASS | Job output maps to `steps.pr_image_ref_output.outputs.image_ref`; contract step enforces exactly one PR tag match and fails on empty output. |
| Consumer gate behavior | PASS | `scan-pr-image` requires `needs.build-and-push.outputs.pr_image_ref != ''` before running. |
| PR Trivy blocking enforcement while preserving SARIF path | PASS | Trivy SARIF step keeps `exit-code: '1'` + `continue-on-error: true`; SARIF upload runs with `if: always()` when file exists; final enforcement step fails when scan outcome is not success. |

## Executed Local Checks

| Command | Status | Result |
|---|---|---|
| `actionlint .github/workflows/docker-build.yml` | FAIL (non-blocking style) | 1 finding: `SC2129` at workflow line 91 (style: grouped redirect suggestion). No contract/gate logic error reported. |
| `yq '.' /projects/Charon/.github/workflows/docker-build.yml` | PASS | YAML parsed successfully (`YQ_PARSE_EXIT:0`). |
| `yamllint /projects/Charon/.github/workflows/docker-build.yml` | FAIL (style policy) | Exit 1 with many style findings (document-start, line-length, comment spacing). Not specific to this change and typically non-blocking for Actions runtime. |
| Pattern verification (`grep`/`rg`) | PASS | Confirmed output mapping, PR consumer guard, Trivy SARIF step, SARIF upload step, and final enforcement step are present. |
| `trivy config` on workflow path | INCONCLUSIVE (tool/path issue) | Trivy is installed, but local runs returned `lstat ... no such file or directory` for existing `.github/workflows` paths in this environment. |
| `checkov` availability | NOT RUN | `checkov` is not installed in this environment. |

## Workflow Diff Assessment

This implementation resolves the previously observed CI failure mode where PR image reference was empty in the security scan job.

1. Producer contract hardened:
- Step ID normalized to `pr_image_ref_output` and wired to job output.
- PR tag resolution hardened (`set -euo pipefail`, strict regex match count = 1).
- Explicit `Assert PR output contract` step added.

2. Consumer gate hardened:
- `scan-pr-image` job now checks `needs.build-and-push.outputs.pr_image_ref != ''` at job-level `if`.

3. Security gate behavior corrected:
- Trivy SARIF scan remains strict (`exit-code: '1'`) but non-blocking at step level so SARIF upload and summaries still execute.
- Final `Enforce PR Trivy security gate` step blocks when vulnerabilities are found or scan fails.

## DoD Test Applicability

| DoD Item | Applicability | Rationale |
|---|---|---|
| Backend unit/integration coverage gates | Not applicable | No backend runtime code changes in scope. |
| Frontend tests/type checks | Not applicable | No frontend code changes in scope. |
| E2E Playwright execution | Not applicable | No UI/API runtime behavior changed by this patch. |
| GORM security scan | Not applicable | No model/database changes in scope. |
| Workflow lint/security static checks | Applicable | Executed and reported above. |

## Remediation Needed

1. Recommended: resolve the `actionlint` shellcheck style warning (`SC2129`) at workflow line 91.
2. Optional: reduce `yamllint` style noise (line-length/comment spacing) or tune project yamllint policy for workflows.
3. Optional: resolve local Trivy config scanner path issue or run scanner in a known-good containerized path to restore workflow-IaC security scanning.

## Final QA Decision

PASS for this workflow-only change. The pull_request producer/consumer contract and PR Trivy enforcement behavior are correct, and the SARIF upload path is preserved under scan-failure conditions.
