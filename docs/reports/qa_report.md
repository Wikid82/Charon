# QA Report - Scoped Workflow Audit

| Field | Value |
|---|---|
| Date | 2026-05-25 |
| Scope | `.github/workflows/docker-build.yml` only (workflow YAML change scope) |
| Canonical Policy Sources | `.github/instructions/copilot-instructions.md`, `.github/instructions/testing.instructions.md`, `.github/instructions/github-actions-ci-cd-best-practices.instructions.md` |
| Final Verdict (Scoped) | **RELEASABLE (WORKFLOW SCOPE)** |
| Release Decision Basis | **`actionlint` pass + scoped applicability classification complete** |

## 0) Final Closure Snapshot

### Final scoped verdict

- **RELEASABLE (WORKFLOW SCOPE)**

### Key evidence

1. Mandatory workflow-scope gate passed:
	- `actionlint .github/workflows/docker-build.yml` -> **PASS**
2. Scope verification confirms workflow-only releasability decision:
	- Changed files include `.github/workflows/docker-build.yml` plus documentation artifacts.

### Applicability classification (final)

1. **Applicable and satisfied**
	- Workflow lint/validation for edited workflow (`actionlint`) -> passed.
2. **Conditionally applicable, not triggered by this scope**
	- GORM scanner, E2E rebuild requirement.
3. **Non-applicable for this workflow-only release decision**
	- App-level coverage/type-check/build/E2E blockers.

## 1) DoD Gate Applicability for Workflow-Only Scope

### Strictly applicable to this change

1. Workflow syntax/semantic validation for edited workflow file.
	- Basis: workflow-specific instruction scope in `.github/instructions/github-actions-ci-cd-best-practices.instructions.md` (`applyTo: .github/workflows/*.yml,.github/workflows/*.yaml`).
	- Enforceable locally with `actionlint` (and optional YAML lint/parse checks).

### Conditionally applicable (not triggered by this scope)

1. GORM security scanner.
	- Basis: `.github/instructions/testing.instructions.md` limits mandatory trigger to backend model/database paths (`backend/internal/models/**`, GORM services, migrations). Not triggered by workflow-only scope.
2. E2E container rebuild requirement.
	- Basis: `.github/instructions/testing.instructions.md` explicitly states rebuild is optional for CI/workflow-only changes (`.github/workflows/**`) unless environment is unhealthy.

### Non-applicable for this scope (application-code gates)

1. Backend/frontend coverage gates and patch coverage gating for changed feature code.
2. Frontend type-check gate.
3. Build verification for backend/frontend binaries.
4. App-level E2E validation as release blocker for this workflow-only diff.

Policy note:

- `.github/instructions/copilot-instructions.md` defines broad DoD gates for implementation tasks; this audit applies strict scope filtering based on conditional triggers in `.github/instructions/testing.instructions.md` and workflow-specific applicability in `.github/instructions/github-actions-ci-cd-best-practices.instructions.md`.

## 2) Minimum Required Verification Set (Workflow Scope) and Evidence

Executed from `/projects/Charon`.

### A. Confirm scope

Command:

- `git diff --name-only`
- `git status --short`

Evidence:

- `.github/workflows/docker-build.yml`
- Additional modified files are documentation/reporting artifacts only:
	- `docs/issues/manual_test_workflow_pr_trivy_phase7_closure.md`
	- `docs/plans/current_spec.md`
	- `docs/reports/qa_report.md`

Scoped target remains `.github/workflows/docker-build.yml`.

### B. actionlint on edited workflow (mandatory in this audit)

Commands:

- `command -v actionlint`
- `actionlint .github/workflows/docker-build.yml`

Result: **PASS**

Evidence:

- Binary found: `/usr/local/bin/actionlint`
- `actionlint` returned no findings for `.github/workflows/docker-build.yml`.

### C. Workflow-centric local checks (additional, advisory in this scope)

Commands:

- `command -v yamllint`
- `yamllint .github/workflows/docker-build.yml`

Result: **Findings present (style/lint policy), not a scoped blocker**

Evidence:

- Binary found: `/bin/yamllint`
- `line 1`: warning `document-start` (missing `---`).
- `line 2`: warning `truthy` (boolean style check).
- `line 10`: error `line-length` (116 > 80).

These checks are useful for formatting consistency but are not defined as mandatory
release gates for this workflow-only verification in the cited canonical policies.

## 3) Classification of Previously Observed Failures

### Applicable blocker for this change

1. None remaining after rerun.
	- `actionlint` passed for `.github/workflows/docker-build.yml`.

### Non-applicable for this change scope

1. E2E failures (`Container failed to start`, unreachable 8080) from app test path.
2. Backend/frontend coverage failures.
3. Frontend type-check outcomes.
4. GORM security scan outcomes (no model/database path changes).
5. Trivy repo findings unrelated to workflow YAML content.
6. `yamllint` style findings (`document-start`, `truthy`, `line-length`) are
   not required release blockers for this scoped workflow audit.

### Environment/tooling blocker requiring separate remediation

1. `pre-commit run --all-files` failure due missing `.pre-commit-config.yaml`.
2. CodeQL local run unavailable due missing local toolchain/extension setup.
3. Docker-image scan failures caused by local runtime/registry access issues.
4. Missing `ruby` interpreter for optional YAML parse check (non-blocking here).

## 4) Scoped Releasability Verdict

**RELEASABLE (workflow scope)**

Reason:

1. The strictly required local gate for this scope (`actionlint` on edited
	workflow) now passes with zero findings.
2. Remaining findings are classified as non-applicable/environment or advisory
	style checks for this scoped release decision.

Optional hardening before merge:

1. Add `---` YAML document start.
2. Normalize boolean style for `truthy` lint rule.
3. Wrap long line(s) to satisfy 80-char lint convention.
