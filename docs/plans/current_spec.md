---
post_title: Caddy Version Drift in GitHub Security Root-Cause Plan
categories:
  - plans
tags:
  - security
  - trivy
  - docker
  - github-actions
  - supply-chain
summary: Investigation and durable remediation plan to identify why GitHub Security still reports Caddy 2.11.2 after Dockerfile was updated to 2.11.3, with concrete verification commands, failure-mode matrix, and commit slicing.
post_date: 2026-05-24
---

## Introduction

### Overview

Dockerfile now pins Caddy to 2.11.3, but GitHub Security still shows Trivy
alerts indicating 2.11.2. This plan defines how to identify the exact reporting
source (workflow, category, artifact, branch, platform, or cache path), prove
root cause with evidence, and implement durable fixes so alert state converges
with actual shipped images.

### Objectives

- Identify the precise workflow/category/artifact still emitting Caddy 2.11.2.
- Determine why that path bypassed or outlived the Dockerfile update.
- Define durable remediation over one-off closures.
- Validate remediation with repeatable commands and expected outcomes.

### Scope

In scope:

- GitHub Actions workflows that build images and upload SARIF.
- Trivy/Grype/SBOM/provenance pipeline wiring.
- PR/push/nightly/weekly branch and category behavior.
- Local task/skill wiring that influences operator assumptions.
- Minimal config updates needed for long-term correctness.

Out of scope:

- Unrelated frontend/backend product behavior.
- Dependency policy changes not related to the stale Caddy signal.

## Requirements

### EARS Requirements

- WHEN Dockerfile pins Caddy to 2.11.3, THE SYSTEM SHALL report Caddy 2.11.3
  in all image-based security scans for the same source revision.
- WHEN SARIF is uploaded from multiple workflows, THE SYSTEM SHALL use
  unambiguous categories that map to active workflows only.
- IF a workflow scans a partial artifact (for example only one binary), THEN THE
  SYSTEM SHALL not be treated as authoritative for full container component
  status.
- WHEN builds are skipped or use stale tags/artifacts, THE SYSTEM SHALL emit an
  explicit signal that scan freshness is unknown.
- WHEN multi-arch images are published, THE SYSTEM SHALL verify Caddy version
  per relevant platform digest, not only by floating tag.

## Research Findings

### Confirmed Version Pin

- Dockerfile sets CADDY_VERSION=2.11.3 and CADDY_CANDIDATE_VERSION=2.11.3.
- Caddy is built from source in caddy-builder and copied into runtime.

### Exact Files and Workflows to Inspect

#### Primary CI and Security Sources

- [.github/workflows/docker-build.yml](.github/workflows/docker-build.yml)
- [.github/workflows/security-pr.yml](.github/workflows/security-pr.yml)
- [.github/workflows/security-weekly-rebuild.yml](.github/workflows/security-weekly-rebuild.yml)
- [.github/workflows/nightly-build.yml](.github/workflows/nightly-build.yml)
- [.github/workflows/supply-chain-pr.yml](.github/workflows/supply-chain-pr.yml)
- [.github/workflows/supply-chain-verify.yml](.github/workflows/supply-chain-verify.yml)

#### Scan Configuration and Ignore Policy

- [trivy.yaml](trivy.yaml)
- [.trivyignore](.trivyignore)
- [.grype.yaml](.grype.yaml)

#### Task Wiring and Local Operator Paths

- [.vscode/tasks.json](.vscode/tasks.json)
- [.github/skills/scripts/skill-runner.sh](.github/skills/scripts/skill-runner.sh)
- [.github/skills/security-scan-trivy-scripts/run.sh](.github/skills/security-scan-trivy-scripts/run.sh)
- [.github/skills/security-scan-docker-image-scripts/run.sh](.github/skills/security-scan-docker-image-scripts/run.sh)

#### Documentation that Influences Scan Interpretation

- [SECURITY.md](SECURITY.md)
- [docs/guides/supply-chain-security-developer-guide.md](docs/guides/supply-chain-security-developer-guide.md)
- [ARCHITECTURE.md](ARCHITECTURE.md)

### High-Signal Observations

- docker-build.yml uploads Trivy SARIF under multiple categories, including a
  legacy compatibility alias:
  .github/workflows/docker-publish.yml:build-and-push.
- security-pr.yml does filesystem scan of extracted /app/charon binary,
  not full image and not /usr/bin/caddy; this path cannot be authoritative for
  Caddy image component status.
- nightly-build.yml scans category trivy-nightly and uses separate branch
  flow; stale findings can persist if nightly diverges from main or is not
  rebuilt after merge.
- docker-build.yml has skip logic for chore/Renovate patterns; if build is
  skipped on a critical version-bump commit, canonical scan categories may not
  refresh.
- supply-chain-verify.yml has pull-request tag logic that still references
  pr-<number> style in some paths while docker-build.yml emits immutable
  pr-<number>-<sha> tags; this can cause wrong-artifact lookups in some modes.
- Local Trivy skill scans repository filesystem (trivy fs /app), which can
  disagree with image scans and should not be used to infer GitHub Security
  image alert closure behavior.

## Probable Failure Modes

### FM-1 Stale SARIF Category Track (Legacy Alias)

- Source: docker-build.yml compatibility upload to
  .github/workflows/docker-publish.yml:build-and-push.
- Effect: old category can keep/open alerts even after active workflow updates.

### FM-2 Scanner Targets Wrong Artifact

- Source: security-pr.yml scans only extracted /app/charon.
- Effect: misses Caddy binary (/usr/bin/caddy) status and gives false closure
  confidence.

### FM-3 Build Skipped, Scan Never Refreshed

- Source: skip logic on chore/bot commits in docker-build.yml.
- Effect: post-merge categories not refreshed, old alerts remain open.

### FM-4 Branch Mismatch

- Source: nightly/dev/main have separate scan categories and schedules.
- Effect: Security tab still shows 2.11.2 from non-main category/ref.

### FM-5 Stale Cached Build Output

- Source: BuildKit cache behavior in certain workflows/stages.
- Effect: rebuilt image may still embed older Caddy artifact in one path.

### FM-6 PR Tag and Artifact Resolution Drift

- Source: mutable/legacy tag expectations (pr-<number>) vs immutable tags
  (pr-<number>-<sha>).
- Effect: scanner pulls unexpected image.

### FM-7 Trivy DB/Cache Timing and Feed Drift

- Source: DB cache freshness differences between runs.
- Effect: inconsistent vulnerability metadata across runs.

### FM-8 Multi-Arch Manifest Mismatch

- Source: one architecture updated while another remains old.
- Effect: alerts persist for platform-specific digest even if amd64 looks fixed.

### FM-9 Old SARIF Still Open on Default Branch

- Source: no subsequent successful upload for same category/ref to close prior
  result set.
- Effect: stale findings remain visible.

## Technical Specifications

### Investigation Data Model

For each open Caddy finding, collect this tuple:

- alert number
- tool name (Trivy)
- state
- most recent instance ref
- SARIF category
- workflow/run URL
- scanned target type (image or fs)
- image ref or digest (if applicable)
- observed Caddy version evidence

### Verification Commands and Expected Outcomes

#### 1. Enumerate Open Trivy Alerts and Categories (Paginated + Ref-Aware)

```bash
# Optional: set REF_PREFIX to constrain results to a branch namespace.
# Examples: refs/heads/main, refs/heads/nightly
REF_PREFIX="refs/heads/main"

gh api --paginate \
  -H "Accept: application/vnd.github+json" \
  "/repos/<owner>/<repo>/code-scanning/alerts?tool_name=Trivy&state=open&per_page=100" \
  | jq -r --arg refPrefix "$REF_PREFIX" '
      .[]
      | select(.most_recent_instance.ref | startswith($refPrefix))
      | [
          .number,
          .most_recent_instance.ref,
          .most_recent_instance.commit_sha,
          .most_recent_instance.category,
          .most_recent_instance.analysis_key,
          .most_recent_instance.location.path,
          .most_recent_instance.message.text
        ] | @tsv'
```

Expected outcome:

- Full open-alert inventory is complete (no missed pages).
- Each open alert is mapped to ref + category + analysis key for correlation.
- Explicit pass condition: every alert containing Caddy 2.11.2 has a known
  category/ref owner row in the investigation table.

#### 2. Map Alert Category/Ref to Workflow Runs

```bash
gh run list --workflow docker-build.yml --branch main --limit 50 \
  --json databaseId,headSha,createdAt,conclusion,url

gh run list --workflow security-weekly-rebuild.yml --branch main --limit 20 \
  --json databaseId,headSha,createdAt,conclusion,url

gh run list --workflow nightly-build.yml --branch nightly --limit 20 \
  --json databaseId,headSha,createdAt,conclusion,url
```

Expected outcome:

- For each alert tuple, at least one candidate run is identified by matching
  workflow + ref + headSha window.
- Explicit pass condition: no unresolved alert tuple remains without a candidate
  producing run.

#### 3. Correlate SARIF to Alerts Using Structured Fields (Not String-Only)

```bash
# After downloading SARIF from candidate runs, extract structured tuples.
jq -r '
  .runs[] as $run
  | $run.results[]?
  | [
      ($run.automationDetails.id // ""),
      (.ruleId // ""),
      (.level // ""),
      (.locations[0].physicalLocation.artifactLocation.uri // ""),
      (.partialFingerprints.primaryLocationLineHash // ""),
      (.properties.image_ref // .properties.imageRef // ""),
      (.message.text // "")
    ] | @tsv
' trivy-results.sarif

# Optional diagnostic grep is allowed only as a secondary check.
grep -En "2\.11\.(2|3)|caddyserver/caddy" trivy-results.sarif || true
```

Expected outcome:

- Structured SARIF fields map back to alert category/ref/analysis key lineage.
- Explicit pass condition: each open 2.11.2 alert is backed by one matched SARIF
  result tuple from the owning workflow/category.

#### 4. Verify Built Image Caddy Version by Digest

```bash
IMAGE="ghcr.io/<owner>/charon@sha256:<digest>"
docker pull "$IMAGE"
docker run --rm --entrypoint caddy "$IMAGE" version
```

Expected outcome:

- Returns v2.11.3 for corrected artifacts.
- If v2.11.2, confirms build-path/cache/tag issue.
- Explicit pass condition: all active-category digests under investigation report
  v2.11.3.

#### 5. Verify Multi-Arch Digests

```bash
docker buildx imagetools inspect "ghcr.io/<owner>/charon@sha256:<digest>"
```

Expected outcome:

- Lists per-platform manifests; follow with per-platform checks where possible.
- Confirms whether one architecture still carries 2.11.2.
- Explicit pass condition: no platform digest in active categories reports
  Caddy 2.11.2.

#### 6. Validate Workflow Skip Behavior on Merge Commit

```bash
gh run view <run-id> --log | grep -n "skip_build\|Determine skip condition"
```

Expected outcome:

- Confirms whether key post-merge build/scan was skipped due to commit title or
  actor rules.
- Explicit pass condition: security-relevant commits are not skipped, or skip is
  accompanied by an accepted/manual refresh path.

#### 7. Validate Scan Target Type

```bash
# Read workflow logic directly for scan target
grep -n "scan-type\|scan-ref\|image-ref\|/app/charon\|/usr/bin/caddy" .github/workflows/security-pr.yml .github/workflows/docker-build.yml
```

Expected outcome:

- Evidence of fs-only binary scan vs full image scan path.
- Explicit pass condition: authoritative image-scan workflow is clearly
  identified and documented for closure decisions.

## Durable Remediation Options (Prioritized)

### Priority 0 Legacy SARIF Category Closure Backfill Before Retirement

- Build a closure inventory of all open legacy-category alerts keyed by:
  category + ref + workflow + alert number.
- Run an explicit backfill cycle that refreshes those legacy tracks with current
  results before retiring compatibility categories.
- Retire compatibility categories only after the backfill shows no unresolved
  Caddy 2.11.2 lineage in those tracks.

Why:

- Prevents orphaned legacy alert tracks from surviving category retirement.
- Ensures closure state is auditable before compatibility removal.

### Priority 1 Remove Legacy SARIF Alias Categories

- Remove compatibility uploads in docker-build.yml for:
  - .github/workflows/docker-publish.yml:build-and-push
  - any duplicate compatibility category not actively used.
- Keep one canonical category per scan job.

Why:

- Prevents long-tail stale alert tracks and ambiguous ownership.

### Priority 2 Enforce Canonical Image Scan as Source of Truth

- Treat image-based Trivy scan in docker-build.yml and
  security-weekly-rebuild.yml as authoritative for Caddy status.
- Keep security-pr.yml binary scan as supplemental, or extend it to extract
  and scan /usr/bin/caddy explicitly if retained for Caddy signal.
- Add explicit category ownership and normalization for
  security-weekly-rebuild.yml (set stable category naming and owner cadence).

Why:

- Eliminates partial-artifact blind spots.
- Removes implicit category drift in weekly rebuild uploads.

### Priority 3 Tighten Post-Merge Refresh Guarantees

- Ensure Dockerfile/security-significant changes cannot bypass build+scan via
  skip logic.
- Add a condition override: if Dockerfile or workflow security files changed,
  force build/scan.

Why:

- Guarantees scan freshness after security-relevant merges.

### Priority 4 Normalize PR Tag and Artifact Resolution

- Align all workflows on immutable pr-<number>-<sha> semantics.
- Remove or gate legacy pr-<number> fallback paths.

Why:

- Avoids scanning wrong images.

### Priority 5 Add Freshness Telemetry and Closure Proof

- Emit build digest, Caddy version, and scan category into step summary and
  retained artifact for each security upload.
- Optional: add a post-scan assertion that SARIF run metadata references same
  digest scanned.

Why:

- Makes stale-path diagnosis immediate.

### Priority 6 Multi-Arch Explicit Verification

- Add per-arch Caddy version check in CI before SARIF upload, at least for
  linux/amd64 and linux/arm64 manifests where available.

Why:

- Prevents hidden platform drift.

## Review of .gitignore, .dockerignore, codecov.yml, Dockerfile

### .gitignore

- No change required for this issue.
- Existing SARIF/artifact ignores are appropriate.

### .dockerignore

- No change required for this issue.
- Current exclusions do not explain stale Caddy finding source.

### codecov.yml

- No change required for this issue.
- Coverage settings are unrelated to code scanning freshness.

### Dockerfile

- Functional version pin is already correct at 2.11.3.
- Optional hardening (recommended if issue persists):
  - add explicit build-time assertion that built Caddy reports expected major/minor/patch;
  - emit OCI label with built Caddy version for traceability.

## Implementation Plan

### Phase 1 Playwright/UI Validation

Objective:

- Confirm this problem is CI/security-pipeline only, not a UI regression.
- Run UI/Playwright checks conditionally when product-surface files changed.

Tasks:

1. Compute change scope:
  - workflow/security-pipeline-only change (for example .github/workflows,
    scripts security tooling, scan configs), or
  - product-surface change (frontend/backend/runtime behavior).
2. If product-surface change is present, run targeted smoke UI task(s) per DoD
  policy.
3. If workflow/security-pipeline-only, record conditional waiver and skip UI
  execution.
4. Record that no UI behavior change is expected from this remediation.

Validation gate:

- For product-surface changes: no UI regressions.
- For workflow/security-only changes: waiver recorded with scope evidence.

### Phase 2 Root-Cause Evidence Collection

Objective:

- Prove exact stale reporting source.

Tasks:

1. Enumerate open Trivy alerts with category/ref.
2. Correlate each with workflow run and SARIF artifact.
3. Confirm scanned image digest and runtime Caddy version.
4. Determine whether issue is category-stale, artifact mismatch, skip-build,
   branch mismatch, or multi-arch divergence.

Validation gate:

- One or more root causes selected, each mapped to category + ref + workflow
  with evidence artifact links.

### Phase 3 Workflow Remediation

Objective:

- Remove ambiguity and enforce canonical scan path.

Tasks:

1. Execute legacy-category closure backfill for affected category/ref/workflow
  tuples.
2. Remove legacy category uploads in docker-build.yml after verified backfill.
3. Align category naming to one active track per workflow.
4. Add explicit weekly category ownership/normalization for
  security-weekly-rebuild.yml.
5. Add force-scan behavior for Dockerfile/security-file changes.
6. Align PR image tag/artifact resolution where drift exists.

Validation gate:

- Backfill completed and documented before compatibility retirement.
- New run uploads SARIF to canonical category only; no legacy category updates.

### Phase 4 Verification and Backfill

Objective:

- Demonstrate closure behavior and prevent recurrence.

Tasks:

1. Re-run relevant workflows on latest main commit.
2. Verify SARIF reflects Caddy 2.11.3.
3. Verify GitHub Security open Trivy alerts for Caddy 2.11.2 close or are
   superseded by resolved state.
4. If needed, run weekly/nightly paths to refresh branch-specific categories.

Validation gate:

- No open Caddy 2.11.2 findings remain in active categories for current refs.
- Legacy categories either show verified closure after backfill or are
  formally tracked with owner + retirement checkpoint.

### Phase 5 Documentation and Governance

Objective:

- Keep future investigations short and deterministic.

Tasks:

1. Update SECURITY.md and supply-chain docs with category/branch ownership.
2. Document canonical workflow for image-based closure verification.
3. Note deprecated categories and retirement date.

Validation gate:

- Docs clearly map alert -> category -> workflow -> digest -> component version.

## Commit Slicing Strategy

### Decision

Single PR with ordered logical commits (security pipeline coherence issue, tightly
related files, low product-surface risk).

### Trigger Reasons

- Cross-domain but same concern: workflow wiring + security signal integrity.
- High reviewer value from isolated commits (evidence, wiring, docs).
- Fast rollback needed if any category/reporting side effect appears.

### Ordered Commits

#### Commit 1 Evidence and Traceability Baseline

Scope:

- Add/adjust investigation notes and CI summaries (non-behavioral) to capture
  category/digest/Caddy version mapping.

Files (expected):

- [.github/workflows/docker-build.yml](.github/workflows/docker-build.yml)
- [docs/plans/current_spec.md](docs/plans/current_spec.md)

Dependencies:

- None.

Validation gate:

- Workflow summary includes category + digest + caddy version evidence.

#### Commit 2 Legacy Category Closure Backfill

Scope:

- Build and execute backfill for legacy category/ref/workflow tuples.
- Capture closure evidence and retirement readiness criteria.

Files (expected):

- [.github/workflows/docker-build.yml](.github/workflows/docker-build.yml)
- [docs/plans/current_spec.md](docs/plans/current_spec.md)

Dependencies:

- Commit 1 merged or present in branch.

Validation gate:

- Legacy category closure evidence exists for each tracked tuple.

#### Commit 3 Canonical Category and Weekly Ownership Normalization

Scope:

- Remove legacy SARIF alias uploads and keep canonical categories.
- Add explicit weekly category ownership/normalization for
  security-weekly-rebuild.
- Optionally refine security-pr.yml target semantics for clarity.

Files (expected):

- [.github/workflows/docker-build.yml](.github/workflows/docker-build.yml)
- [.github/workflows/security-weekly-rebuild.yml](.github/workflows/security-weekly-rebuild.yml)
- [.github/workflows/security-pr.yml](.github/workflows/security-pr.yml)

Dependencies:

- Commit 2.

Validation gate:

- SARIF uploaded to canonical categories only with explicit weekly ownership.

#### Commit 4 Freshness and Skip-Guard Enforcement

Scope:

- Ensure Dockerfile/security-file changes force build+scan refresh.
- Normalize PR tag/artifact resolution where needed.

Files (expected):

- [.github/workflows/docker-build.yml](.github/workflows/docker-build.yml)
- [.github/workflows/supply-chain-verify.yml](.github/workflows/supply-chain-verify.yml)
- [.github/workflows/supply-chain-pr.yml](.github/workflows/supply-chain-pr.yml)

Dependencies:

- Commit 3.

Validation gate:

- Security-relevant change cannot be skipped; image ref resolution deterministic.

#### Commit 5 Docs and Operational Runbook

Scope:

- Document canonical verification path and stale-alert troubleshooting.

Files (expected):

- [SECURITY.md](SECURITY.md)
- [docs/guides/supply-chain-security-developer-guide.md](docs/guides/supply-chain-security-developer-guide.md)

Dependencies:

- Commit 4.

Validation gate:

- Operators can map alert -> category -> workflow -> digest -> component version.

### Rollback and Contingency

- If category cleanup causes missing visibility, revert Commit 3 only while
  retaining backfill evidence from Commit 2.
- If force-scan logic creates excessive CI load, revert Commit 4 and keep
  category cleanup.
- Maintain a one-cycle overlap where canonical and old category results are
  compared internally (not uploaded as parallel public categories).

## Risks and Mitigations

- Risk: closing legacy category may hide historical trend continuity.
  - Mitigation: keep artifact retention and migration note in docs.
- Risk: force-scan increases CI time.
  - Mitigation: scope force condition to Dockerfile/security workflow paths only.
- Risk: multi-arch checks add flakiness on runners.
  - Mitigation: keep per-arch checks lightweight and deterministic.

## Acceptance Criteria

- Root cause identified with evidence from open alerts, SARIF category, and
  workflow run mapping.
- Root cause set allows one or more concurrent causes, each mapped to category,
  ref, and workflow with evidence.
- Canonical workflows show scans for current digest with Caddy 2.11.3.
- Legacy category closure backfill completed before retirement of compatibility
  categories.
- Legacy/ambiguous category uploads removed or explicitly deprecated.
- security-weekly-rebuild has explicit category ownership and normalization.
- Security tab no longer shows active Caddy 2.11.2 findings for current active
  categories/refs.
- Durable guardrails in place for skip, tag resolution, and scan target clarity.
- DoD checks pass; any residual findings are documented with explicit owner and
  next action.
