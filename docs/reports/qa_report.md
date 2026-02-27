double check our caddy version# QA Report: Nightly Workflow Fix Audit

- Date: 2026-02-27
- Scope:
  - `.github/workflows/nightly-build.yml`
    1. `pr_number` failure avoidance in nightly dispatch path
    2. Deterministic Syft SBOM generation with fallback
  - `.github/workflows/security-pr.yml` contract check (`pr_number` required)

## Findings (Ordered by Severity)

### ✅ No blocking findings in audited scope

1. `actionlint` validation passed for modified workflow.
   - Command: `actionlint .github/workflows/nightly-build.yml`
   - Result: PASS (no diagnostics)

2. `pr_number` nightly dispatch failure path is avoided by excluding PR-only workflow from nightly fan-out.
   - `security-pr.yml` removed from dispatch list in `.github/workflows/nightly-build.yml:103`
   - Explicit log note added at `.github/workflows/nightly-build.yml:110`

3. SBOM generation is now deterministic with explicit primary pin and verified fallback.
   - Primary action pins Syft version at `.github/workflows/nightly-build.yml:231`
   - Fallback installs pinned `v1.42.1` with checksum verification at `.github/workflows/nightly-build.yml:245`
   - Mandatory artifact verification added at `.github/workflows/nightly-build.yml:268`

4. No permission broadening in modified sections.
   - Dispatch job permissions remain `actions: write`, `contents: read` at `.github/workflows/nightly-build.yml:84`
   - Build job permissions remain `contents: read`, `packages: write`, `id-token: write` at `.github/workflows/nightly-build.yml:145`
   - Diff review confirms no `permissions` changes in the modified hunk.

5. Action pinning remains SHA-based in modified sections.
   - `actions/github-script` pinned SHA at `.github/workflows/nightly-build.yml:89`
   - `anchore/sbom-action` pinned SHA at `.github/workflows/nightly-build.yml:226`
   - `actions/upload-artifact` pinned SHA at `.github/workflows/nightly-build.yml:283`

6. `security-pr.yml` contract still requires `pr_number`.
   - `workflow_dispatch.inputs.pr_number.required: true` at `.github/workflows/security-pr.yml:14`

## Pass/Fail Decision

- QA Status: **PASS with caveats**
- Reason: All requested static validations pass and the scoped workflow logic changes satisfy the audit requirements.

## Residual Risks

1. Fallback integrity uses checksum file from the same release origin as the tarball.
   - Impact: If release origin is compromised, checksum verification alone may not detect tampering.
   - Suggested hardening: verify signed release metadata or verify Syft artifact signature (Cosign/GitHub attestations) in fallback path.

2. Runtime behavior is not fully proven by local static checks.
   - Impact: Dispatch and SBOM behavior still require a real GitHub Actions run to prove end-to-end execution.

## Remote Execution Limitation and Manual Verification

I did not execute remote nightly runs for this exact local diff in this audit. Local `actionlint` and source inspection were performed. To validate end-to-end behavior on GitHub Actions, run:

```bash
cd /projects/Charon

# 1) Syntax/lint (already run locally)
actionlint .github/workflows/nightly-build.yml

# 2) Trigger nightly workflow (manual)
gh workflow run nightly-build.yml --ref nightly -f reason="qa-nightly-audit" -f skip_tests=true

# 3) Inspect latest nightly run
gh run list --workflow "Nightly Build & Package" --branch nightly --limit 1
gh run view <run-id> --log

# 4) Confirm no security-pr dispatch error in nightly logs
# Expectation: no "Missing required input 'pr_number' not provided"

# 5) Confirm security-pr contract still enforced
gh workflow run security-pr.yml --ref nightly
# Expectation: dispatch rejected due to required missing input pr_number

# 6) Positive contract check with explicit pr_number
gh workflow run security-pr.yml --ref nightly -f pr_number=<valid-pr-number>
```

Expected outcomes:
- Nightly run completes dispatch phase without `pr_number` input failure.
- SBOM generation succeeds via primary or fallback path and uploads `sbom-nightly.json`.
- `security-pr.yml` continues enforcing required `pr_number` for manual dispatch.
