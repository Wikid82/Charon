---
title: "Manual Test Plan - Semgrep CI Security Scan"
status: Open
priority: Medium
labels: testing, ci, security
---

# Test Objective

Confirm that the new `.github/workflows/semgrep.yml` CI workflow behaves correctly once it
actually runs against a live GitHub Actions PR — image pull, scan execution, SARIF upload, and
the hard-fail gate. This is the one part of the Semgrep CI Security Scan feature that could not
be verified locally: Supervisor code review and the qa-security audit both passed (see
`docs/plans/current_spec.md` and `docs/reports/qa_report.md`), but neither can observe a real
GitHub Actions runner pulling the pinned container image or timing a full-repo scan under actual
CI conditions.

# What Was Built

- `scripts/pre-commit-hooks/semgrep-scan.sh` gained an additive, backward-compatible
  `SEMGREP_SARIF_OUTPUT` env var so CI can reuse the exact same scan invocation developers already
  run locally, for both a SARIF-producing pass and a hard-fail gate pass.
- `.github/workflows/semgrep.yml` runs that script inside a pinned
  `semgrep/semgrep:1.173.0@sha256:...` container on every push/PR to `main`, `nightly`, and
  `development`, on manual dispatch, and weekly (Mondays 04:00 UTC). It uploads SARIF results to
  the GitHub Security tab and hard-fails the job on any ERROR/WARNING-severity finding.
- `scripts/ci/check-semgrep-parity.sh` guards against the workflow and the local script silently
  drifting apart in the future.
- `SECURITY.md` and `ARCHITECTURE.md` were updated to document the new coverage.

Commits: `6bf066f8` (script hook + parity guard), `2fbecf07` (workflow), `7c6fb04f` (docs).

# Prerequisites

- A pull request open against `development` (or `main`/`nightly`) that includes these three
  commits, so `semgrep.yml`'s `pull_request` trigger fires.
- Repo admin/write access to view the Actions run and the Security → Code scanning alerts tab.

# Manual Scenarios

## 1) Workflow triggers and appears as a PR check

- [ ] Open the PR containing commits `6bf066f8`, `2fbecf07`, `7c6fb04f`.
- [ ] **Expected**: A check named **Semgrep SAST Scan** (job `semgrep-scan` in workflow
      `Semgrep - SAST Scan`) appears in the PR's checks list shortly after the PR is opened or
      updated.

## 2) Pinned container image pulls successfully

- [ ] Open the Actions run for the Semgrep workflow, expand the earliest steps.
- [ ] **Expected**: No container-pull error (e.g. `manifest unknown`, rate-limit, or timeout
      pulling `semgrep/semgrep:1.173.0@sha256:...`). The job proceeds past the container-setup
      phase into "Checkout repository."

## 3) Job completes within the timeout; check actual timing

- [ ] Note the total run duration for the `semgrep-scan` job once it finishes.
- [ ] **Expected**: Job completes well within the current `timeout-minutes: 15` cap.
- [ ] **If the run takes noticeably close to 15 minutes** (cold image pull + rule-registry fetch
      was never observed live before this PR — flagged as an open risk by both Supervisor and
      DevOps): file a follow-up to bump `timeout-minutes` to ~20-25 in `semgrep.yml`. This is not
      a blocker for merging this PR, but should not be left unaddressed if observed.

## 4) SARIF results appear in the Security tab

- [ ] Navigate to the repo's **Security → Code scanning alerts** tab.
- [ ] Filter by tool **Semgrep**, category **semgrep**.
- [ ] **Expected**: A scan result is listed for the commit/PR, even if it shows 0 findings (a
      SARIF upload with an empty `results` array is still a valid, visible scan entry — this
      confirms the upload step itself worked, not just that the repo is clean).

## 5) Hard-fail gate passes on a clean repo

- [ ] Check the **Run Semgrep (hard-fail gate)** step's log output.
- [ ] **Expected**: Step exits 0. The repo is expected to be clean — 0 findings was reproduced
      locally multiple times (both in DevOps validation and independently in QA's audit) — so
      this step should pass without needing any fix commits.

## 6) Job summary renders correctly

- [ ] Open the Actions run's **Summary** tab (not the individual job log).
- [ ] **Expected**: A "Semgrep SAST Scan Results" section is present, listing the rulesets
      scanned (`p/golang, p/javascript, p/typescript, p/react, p/secrets, p/dockerfile`), the
      severity gate (`ERROR, WARNING`), and a clear PASSED/FAILED line matching the job's actual
      outcome.

# Expected Results

| Scenario | Expected outcome |
|---|---|
| PR trigger | "Semgrep SAST Scan" check appears on the PR |
| Image pull | Pinned `semgrep/semgrep` image pulls with no error |
| Timing | Job finishes comfortably under 15 minutes |
| SARIF upload | Result visible under Security → Code scanning alerts, tool "Semgrep", category `semgrep` |
| Hard-fail gate | Passes (0 findings expected) |
| Job summary | Renders ruleset, severity gate, and pass/fail line in the run summary tab |

# Pass / Fail Criteria

**PASS** — All six scenarios behave as expected: the check appears, the image pulls, the job
finishes well under the timeout, SARIF results are visible in the Security tab under the correct
category, the gate step passes, and the job summary renders correctly.

**FAIL** — Any of: the check never appears on the PR, the image fails to pull, the job times out
or runs suspiciously close to the 15-minute cap, no SARIF entry appears in the Security tab, the
gate step fails unexpectedly on a repo believed to be clean, or the job summary is missing/blank.

A FAIL on the gate step specifically should be triaged on its merits (a real finding vs. a CI
environment issue) before assuming the feature itself is broken — see
`docs/plans/current_spec.md` §3.7 for documented edge cases.

# Known Follow-Ups (not blockers)

1. ~~**Renovate coverage for the pinned image is not yet configured.**~~ **Resolved** (commit
   `9dc2be4e`). Added an explicit custom regex manager in `.github/renovate.json`, anchored on a
   `# renovate: datasource=docker depName=semgrep/semgrep` comment above the `image:` line in
   `semgrep.yml`, mirroring the existing Alpine-image digest tracker pattern used elsewhere in this
   repo. Confirm on the next Semgrep image bump that Renovate actually opens a PR as expected.
2. **`timeout-minutes` may need adjustment after observing real timing.** Set to 15 based on local
   estimates (~45-48s per full-repo scan pass locally); this was never observed against a cold
   image pull + rule-registry fetch on an actual GitHub Actions runner. See Scenario 3 above —
   bump to ~20-25 if the real run comes in close to the cap.

# Related

- `docs/plans/current_spec.md` — full implementation plan for the Semgrep CI Security Scan feature.
- `docs/reports/qa_report.md` — QA/security audit (PASS, no blocking issues).
- Commits `6bf066f8`, `2fbecf07`, `7c6fb04f` on `development`.
