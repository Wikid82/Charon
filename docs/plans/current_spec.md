## Temporary CVE-2026-32286 CI Suppression Alignment

Date: 2026-05-24
Scope: PR CI security scans only. This plan is limited to making the existing
approved suppression for CVE-2026-32286 effective in the PR-blocking Trivy path
without broadening suppression scope.

## Introduction

The repo already contains exact-match suppression records for
CVE-2026-32286, but PR CI still fails because the blocking PR image Trivy scan
does not load the repo ignore file. The minimal safe fix is to align the PR
image scan workflow with the suppression mechanism already used elsewhere in the
repo.

## Research Findings

### Existing Mechanisms

1. `.trivyignore` already contains an exact suppression entry for
   `CVE-2026-32286`, with rationale, expiry, review date, and removal criteria.
   It is scoped by vulnerability ID only and documented as applying to the
   CrowdSec-embedded `pgproto3/v2` finding.
2. `.grype.yaml` already contains a package-scoped suppression for
   `CVE-2026-32286` on `github.com/jackc/pgproto3/v2` version `v2.3.3`, with a
   bounded expiry and explicit removal criteria.
3. `.github/workflows/security-pr.yml` already passes `.trivyignore` into both
   Trivy filesystem scan steps, so that PR binary scan path is already wired to
   the repo suppression mechanism.
4. `.github/workflows/supply-chain-pr.yml` already uses `.grype.yaml`, so the
   Grype-based PR supply-chain scan is already wired to the repo suppression
   mechanism.
5. `.github/workflows/docker-build.yml` PR image scan does not pass
   `.trivyignore` to either of its Trivy steps, and the `scan-pr-image` job
   does not materialize the repository before those steps run. This is the
   only confirmed PR security path not aligned with the repo's existing
   suppression mechanism.
6. `Dockerfile` already documents a best-effort mitigation by bumping
   `github.com/jackc/pgx/v4@v4.18.3`, but that is mitigation evidence only, not
   a CI suppression mechanism.

### Confirmed Failure Point

Hypothesis: PR CI is failing because the blocking Trivy image scan in
`.github/workflows/docker-build.yml` neither materializes the repository nor
loads `.trivyignore`, even though the repo already approved suppressions in
that file.

Cheap disconfirming check: inspect the captured CI log for the PR image scan.

Observed result: `.github/logs/ci_failure.log` shows `INPUT_TRIVYIGNORES:` as
blank for the PR image scan and reports `CVE-2026-32286` in
`/usr/local/bin/crowdsec` and `/usr/local/bin/cscli`. Workflow inspection also
shows that `scan-pr-image` lacks an `actions/checkout` or equivalent
materialization step. That confirms the control point is workflow wiring, not
the absence of a suppression record.

### Rejected Alternatives

1. Do not skip the security job or add a workflow path filter. That would hide
   unrelated vulnerabilities and lowers security coverage for the whole PR.
2. Do not add `ignore-unfixed`, severity downgrades, or directory-wide excludes.
   Those would suppress unrelated CVEs and break the repo's current exact-match
   suppression pattern.
3. Do not add a Docker Hub-specific note or registry exception. The failing PR
   scan is against the GHCR PR image, not Docker Hub.
4. Do not introduce VEX as the first response. Trivy suggested it in logs, but
   this repo already standardizes on `.trivyignore` and `.grype.yaml` for
   temporary, documented suppressions.

## Technical Specification

### Exact Files To Edit

1. `.github/workflows/docker-build.yml`
    - Add an `actions/checkout` step, or equivalent repository materialization,
       in `scan-pr-image` before the Trivy steps so `.trivyignore` is present on
       the runner.
   - Add `trivyignores: '.trivyignore'` to the PR image Trivy table step.
   - Add `trivyignores: '.trivyignore'` to the PR image Trivy SARIF/blocking
     step.
   - Do not change severity thresholds, `exit-code`, SARIF upload, or gate
     enforcement logic.
2. No edit required to `.trivyignore` unless the target branch is missing the
   existing `CVE-2026-32286` entry. If touched, keep the suppression as a
   single exact ID line plus bounded comments only.
3. No edit required to `.grype.yaml` unless the justification or expiry needs a
   review refresh. The current entry is already package- and version-scoped.
4. No edit required to `trivy.yaml`, `.github/workflows/security-pr.yml`, or
   `.github/workflows/supply-chain-pr.yml` for this PR-unblock scope.

### Exact Scope Of The Suppression

1. Trivy scope: the PR image scan shall consume the repo's existing
   `.trivyignore` file as-is, without broadening or narrowing its approved
   suppression set in this change.
2. Grype scope: exact advisory ID `CVE-2026-32286` only, limited to package
   `github.com/jackc/pgproto3/v2`, version `v2.3.3`, type `go-module`.
3. Runtime scope: applies only to the CrowdSec-embedded binaries that still
   carry `pgproto3/v2`; it must not be expanded to blanket PostgreSQL, pgx, or
   generic Go-module suppressions.
4. Workflow scope: limited to the PR image Trivy scan job in
   `.github/workflows/docker-build.yml` so that it behaves consistently with the
   already-approved suppression policy used in other PR security scans.

### How To Avoid Suppressing Unrelated CVEs

1. Reuse the existing repo `.trivyignore` file unchanged for this workflow; do
   not introduce wildcards, regex-like patterns, `ignore-unfixed`,
   package-wide ignores, or ad hoc PR-only ignores.
2. Preserve Grype's package/version/type match instead of loosening it to a
   vulnerability-only allowlist.
3. Do not change the workflow from `CRITICAL,HIGH` to a softer severity policy.
4. Do not suppress `GHSA-jqcq-xjh3-6g23` or `GHSA-x6gf-mpr2-68h6` unless a scan
   path explicitly starts failing on those advisories too. This plan is for the
   currently failing `CVE-2026-32286` PR path only.
5. Keep the existing PR gate in place so any unrelated CRITICAL/HIGH finding
   continues to fail the workflow.

## Documentation Notes

1. `.trivyignore` remains the Trivy suppression source of truth. Its comment
   block must continue to state:
   - no upstream fix exists in `pgproto3/v2`
   - CrowdSec must migrate to `pgx/v5` / `pgproto3/v3`
   - the suppression is temporary and reviewed on a date-bound schedule
2. `.grype.yaml` remains the Grype suppression source of truth and must stay in
   sync with `.trivyignore` on rationale and removal trigger.
3. `Dockerfile` comments may continue to document best-effort mitigation, but
   must not be treated as the authoritative suppression registry.
4. Add or preserve a short workflow comment in `.github/workflows/docker-build.yml`
   explaining that the PR image Trivy scan intentionally consumes the same
   repo-level ignore file as other PR security scans.

## Implementation Plan

### Phase 1: Workflow Alignment

1. Update `.github/workflows/docker-build.yml` so both PR image Trivy steps load
   `.trivyignore`.
2. Add repository materialization in `scan-pr-image` before those Trivy steps
   so `.trivyignore` is reliably present on the runner.
3. Keep the change surgical: no new job conditions, no filter logic, no scan
   severity changes.

### Phase 2: Suppression Record Verification

1. Verify `.trivyignore` remains the existing repo suppression file and is not
   broadened or rewritten as part of this change.
2. Verify `.grype.yaml` still binds `CVE-2026-32286` to
   `github.com/jackc/pgproto3/v2@v2.3.3` only.
3. Verify the existing review date and removal criteria still reflect the
   upstream state.

### Phase 3: Validation

1. Re-run the PR image Trivy workflow path.
2. Confirm `scan-pr-image` materializes the repository before invoking Trivy.
3. Confirm action logs now show `INPUT_TRIVYIGNORES=.trivyignore` for the PR
   image scan.
4. Confirm `trivy-pr-results.sarif` honors the repo's existing `.trivyignore`
   suppressions.
5. Confirm the PR image scan still fails if another unrelated unsuppressed
   CRITICAL/HIGH finding exists.
6. Confirm `security-pr.yml` and `supply-chain-pr.yml` remain unchanged and keep
   honoring their existing suppression files.

## Acceptance Criteria

### EARS Requirements

1. WHEN the PR image Trivy scan runs in `.github/workflows/docker-build.yml`,
   THE SYSTEM SHALL materialize the repository before invoking Trivy so
   `.trivyignore` is available on the runner.
2. WHEN the PR image Trivy scan evaluates findings, THE SYSTEM SHALL honor the
   repo's existing `.trivyignore` suppression set.
3. WHEN a HIGH or CRITICAL finding in the PR image scan is not suppressed by
   that repo-level `.trivyignore`, THE SYSTEM SHALL continue to fail the PR
   gate.
4. WHILE upstream CrowdSec still depends on `pgx/v4 -> pgproto3/v2`, THE SYSTEM
   SHALL keep the suppression temporary, documented, and review-dated.
5. WHEN CrowdSec migrates to `pgx/v5` and the finding disappears, THE SYSTEM
   SHALL remove the suppression entries from both `.trivyignore` and
   `.grype.yaml`.

## Residual Risk

1. The vulnerable `pgproto3/v2` code remains present inside CrowdSec binaries;
   the plan only suppresses CI reporting for the already-reviewed advisory.
2. The risk is currently accepted because Charon uses SQLite by default and the
   vulnerable PostgreSQL protocol path is not reachable in a standard
   deployment.
3. Non-standard deployments that wire CrowdSec to a PostgreSQL backend retain
   the upstream dependency risk until CrowdSec migrates to `pgx/v5`.
4. This plan intentionally does not broaden suppression to other advisories or
   other workflows beyond the confirmed PR CI failure path.

## Commit Slicing Strategy

Decision: single PR with one logical commit is sufficient because the change is
workflow-only and the suppression records already exist.

### Commit 1

- Scope: align PR image Trivy steps with the repo-level `.trivyignore` policy.
- Files: `.github/workflows/docker-build.yml`
- Dependencies: existing `.trivyignore` entry for `CVE-2026-32286`
- Validation gate: PR image Trivy scan loads `.trivyignore` and stops failing on
  `CVE-2026-32286` while preserving failures for unrelated CRITICAL/HIGH issues.

Rollback and contingency notes:

1. If the workflow change produces unexpected scan regressions, revert only the
   new `trivyignores` wiring in `.github/workflows/docker-build.yml`.
2. Do not remove the existing suppression records during rollback.
3. If a second advisory alias starts failing the same path, handle it as a
   separate documented suppression review rather than widening this one.
