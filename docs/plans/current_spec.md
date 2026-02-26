---
post_title: "Current Spec: Fix Workflow Concurrency Groups to Enable Run Cancellation"
categories:
  - planning
  - ci-cd
  - github-actions
tags:
  - concurrency
  - e2e-tests
  - workflow-optimization
status: draft
created: 2026-02-26
---

# Fix Workflow Concurrency Groups to Enable Run Cancellation

## 1. Introduction

### Overview

GitHub Actions workflow runs are queueing for hours instead of canceling prior runs when new commits are pushed to the same branch. The user observed 9+ pages of stacked E2E workflow runs.

### Objective

Audit all 36 workflow files in `.github/workflows/`, identify misconfigured concurrency groups that prevent run cancellation, and define the fix for each affected workflow.

## 2. Root Cause Analysis

### How GitHub Actions Concurrency Works

GitHub Actions uses the `concurrency` block to control parallel execution:

```yaml
concurrency:
  group: <string>           # Runs sharing the same group string are subject to concurrency control
  cancel-in-progress: true  # If true, a new run cancels any in-progress run in the same group
```

**The critical rule**: Two runs will only cancel each other if they resolve to the **exact same** `group` string at runtime.

### The SHA-in-Group Anti-Pattern

The primary offender (`e2e-tests-split.yml`) uses:

```yaml
concurrency:
  group: e2e-split-${{ github.workflow }}-${{ github.ref }}-${{ github.event.pull_request.head.sha || github.sha }}
  cancel-in-progress: true
```

**Why this prevents cancellation:**

| Push # | Branch | SHA | Resolved Group String |
|--------|--------|-----|----------------------|
| 1 | `refs/heads/feat-x` | `abc1234` | `e2e-split-E2E Tests-refs/heads/feat-x-abc1234` |
| 2 | `refs/heads/feat-x` | `def5678` | `e2e-split-E2E Tests-refs/heads/feat-x-def5678` |
| 3 | `refs/heads/feat-x` | `ghi9012` | `e2e-split-E2E Tests-refs/heads/feat-x-ghi9012` |

Every push produces a different SHA, so every run gets a **unique** concurrency group. Since no two runs share a group, `cancel-in-progress: true` has no effect — all runs execute to completion, creating the observed hour-long queue.

### The `run_id`-in-Group Anti-Pattern

`codecov-upload.yml` uses:

```yaml
concurrency:
  group: ${{ github.workflow }}-${{ github.ref_name }}-${{ github.run_id }}
```

`github.run_id` is unique per workflow run by definition, so this has the same effect as the SHA anti-pattern — runs never cancel each other.

### The Correct Pattern

For workflows where you want a new push on the same branch to cancel the prior run:

```yaml
concurrency:
  group: ${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: true
```

This produces the same group string for all runs of the same workflow on the same branch, enabling proper cancellation.

## 3. Full Audit Table

### Legend

| Symbol | Meaning |
|--------|---------|
| `BUG` | Has SHA/run_id in concurrency group — prevents cancellation |
| `OK` | Concurrency group is branch-scoped and works correctly |
| `NO-CANCEL` | `cancel-in-progress: false` — intentional (review needed) |
| `NONE` | No concurrency block at all |
| `N/A` | Workflow nature doesn't need cancellation (schedule-only, manual-only, etc.) |

### Workflow Audit

| # | Workflow File | Name | Triggers | Concurrency Group | cancel-in-progress | SHA/run_id Bug? | Verdict | Fix? |
|---|--------------|------|----------|-------------------|-------------------|-----------------|---------|------|
| 1 | `e2e-tests-split.yml` | E2E Tests | `workflow_call`, `workflow_dispatch`, `pull_request` | `e2e-split-${{ github.workflow }}-${{ github.ref }}-${{ github.event.pull_request.head.sha \|\| github.sha }}` | `true` | **YES — SHA** | **BUG** | **YES** |
| 2 | `codecov-upload.yml` | Upload Coverage to Codecov | `pull_request`, `push(main)`, `workflow_dispatch` | `${{ github.workflow }}-${{ github.ref_name }}-${{ github.run_id }}` | `true` | **YES — run_id** | **BUG** | **YES** |
| 3 | `codeql.yml` | CodeQL - Analyze | `pull_request`, `push(main)`, `workflow_dispatch`, `schedule` | `${{ github.workflow }}-${{ github.event_name }}-${{ github.head_ref \|\| github.ref_name }}` | `true` | No | OK | No |
| 4 | `quality-checks.yml` | Quality Checks | `pull_request`, `push(main)` | `${{ github.workflow }}-${{ github.ref }}` | `true` | No | OK | No |
| 5 | `docker-build.yml` | Docker Build, Publish & Test | `pull_request`, `push(main)`, `workflow_dispatch`, `workflow_run` | `${{ github.workflow }}-${{ github.event_name }}-${{ ... head_branch fallback }}` | `true` | No | OK | No |
| 6 | `benchmark.yml` | Go Benchmark | `pull_request`, `push(main)`, `workflow_dispatch` | `${{ github.workflow }}-${{ github.event_name }}-${{ ... head_branch \|\| github.ref }}` | `true` | No | OK | No |
| 7 | `cerberus-integration.yml` | Cerberus Integration | `workflow_dispatch`, `pull_request`, `push(main)` | `${{ github.workflow }}-${{ ... event_name }}-${{ ... head_branch \|\| github.ref }}` | `true` | No | OK | No |
| 8 | `crowdsec-integration.yml` | CrowdSec Integration | `workflow_dispatch`, `pull_request`, `push(main)` | `${{ github.workflow }}-${{ ... event_name }}-${{ ... head_branch \|\| github.ref }}` | `true` | No | OK | No |
| 9 | `waf-integration.yml` | WAF integration | `workflow_dispatch`, `pull_request`, `push(main)` | `${{ github.workflow }}-${{ ... event_name }}-${{ ... head_branch \|\| github.ref }}` | `true` | No | OK | No |
| 10 | `rate-limit-integration.yml` | Rate Limit integration | `workflow_dispatch`, `pull_request`, `push(main)` | `${{ github.workflow }}-${{ ... event_name }}-${{ ... head_branch \|\| github.ref }}` | `true` | No | OK | No |
| 11 | `supply-chain-pr.yml` | Supply Chain Verification (PR) | `workflow_dispatch`, `pull_request`, `push(main)` | `supply-chain-pr-${{ ... event_name }}-${{ ... head_branch \|\| github.ref }}` | `true` | No | OK | No |
| 12 | `security-pr.yml` | Security Scan (PR) | `workflow_run`, `workflow_dispatch`, `pull_request`, `push(main)` | `security-pr-${{ ... event_name }}-${{ ... head_branch \|\| github.ref }}` | `true` | No | OK | No |
| 13 | `docker-lint.yml` | Docker Lint | `workflow_dispatch` | `${{ github.workflow }}-${{ github.event_name }}-${{ github.head_ref \|\| github.ref_name }}` | `true` | No | OK | No |
| 14 | `repo-health.yml` | Repo Health Check | `schedule`, `workflow_dispatch` | `${{ github.workflow }}-${{ github.event_name }}-${{ github.head_ref \|\| github.ref_name }}` | `true` | No | OK | No |
| 15 | `auto-changelog.yml` | Auto Changelog | `workflow_run`, `release` | `${{ github.workflow }}-${{ github.event_name }}-${{ ... head_branch \|\| ... ref_name }}` | `true` | No | OK | No |
| 16 | `history-rewrite-tests.yml` | History Rewrite Tests | `workflow_run` | `${{ github.workflow }}-${{ github.event_name }}-${{ ... head_branch \|\| ... ref_name }}` | `true` | No | OK | No |
| 17 | `dry-run-history-rewrite.yml` | History Rewrite Dry-Run | `workflow_run`, `schedule`, `workflow_dispatch` | `${{ github.workflow }}-${{ github.event_name }}-${{ ... head_branch \|\| ... ref_name }}` | `true` | No | OK | No |
| 18 | `pr-checklist.yml` | PR Checklist Validation | `workflow_dispatch` | `${{ github.workflow }}-${{ inputs.pr_number \|\| ... }}` | `true` | No | OK | No |
| 19 | `auto-label-issues.yml` | Auto-label Issues | `issues` | `${{ github.workflow }}-${{ github.event.issue.number }}` | `true` | No | OK | No |
| 20 | `renovate_prune.yml` | Prune Renovate Branches | `workflow_dispatch`, `schedule` | `prune-renovate-branches` (job-level) | `true` | No | OK | No |
| 21 | `docs.yml` | Deploy Docs to Pages | `workflow_run`, `workflow_dispatch` | `pages-${{ github.event_name }}-${{ ... head_branch \|\| github.ref }}` | `false` | No | NO-CANCEL | No |
| 22 | `propagate-changes.yml` | Propagate Changes | `workflow_run` | `${{ github.workflow }}-${{ ... head_branch \|\| github.ref }}` | `false` | No | NO-CANCEL | No |
| 23 | `docs-to-issues.yml` | Convert Docs to Issues | `workflow_run`, `workflow_dispatch` | `${{ github.workflow }}-${{ ... head_branch \|\| github.ref }}` | `false` | No | NO-CANCEL | No |
| 24 | `auto-versioning.yml` | Auto Versioning and Release | `workflow_run(main)` | `${{ github.workflow }}-${{ ... head_branch \|\| github.ref }}` | `false` | No | NO-CANCEL | No |
| 25 | `release-goreleaser.yml` | Release (GoReleaser) | `push(tags: v*)` | `${{ github.workflow }}-${{ github.ref }}` | `false` | No | NO-CANCEL | No |
| 26 | `weekly-nightly-promotion.yml` | Weekly Nightly Promotion | `schedule`, `workflow_dispatch` | `${{ github.workflow }}` | `false` | No | NO-CANCEL | No |
| 27 | `caddy-major-monitor.yml` | Monitor Caddy Major | `schedule`, `workflow_dispatch` | `${{ github.workflow }}` | `false` | No | N/A | No |
| 28 | `renovate.yml` | Renovate | `schedule`, `workflow_dispatch` | `${{ github.workflow }}` | `false` | No | N/A | No |
| 29 | `create-labels.yml` | Create Project Labels | `workflow_dispatch` | `${{ github.workflow }}` | `false` | No | N/A | No |
| 30 | `auto-add-to-project.yml` | Auto-add to Project | `issues` | `${{ github.workflow }}-${{ ... issue.number }}` | `false` | No | N/A | No |
| 31 | `security-weekly-rebuild.yml` | Weekly Security Rebuild | `schedule`, `workflow_dispatch` | `${{ github.workflow }}-${{ github.ref }}` | `false` | No | NO-CANCEL | No |
| 32 | `nightly-build.yml` | Nightly Build & Package | `schedule`, `workflow_dispatch` | **None** | — | — | NONE | Optional |
| 33 | `supply-chain-verify.yml` | Supply Chain Verification | `workflow_dispatch`, `schedule`, `workflow_run`, `release` | **None** | — | — | NONE | Optional |
| 34 | `update-geolite2.yml` | Update GeoLite2 Checksum | `schedule`, `workflow_dispatch` | **None** | — | — | NONE | No |
| 35 | `gh_cache_cleanup.yml` | Cleanup GH caches | `workflow_dispatch` | **None** | — | — | NONE | No |
| 36 | `container-prune.yml` | Container Registry Prune | `pull_request`, `schedule`, `workflow_dispatch` | **None** | — | — | NONE | Optional |

## 4. Detailed Fix Plan

### 4.1 FIX: `e2e-tests-split.yml` — PRIMARY OFFENDER

**File:** `.github/workflows/e2e-tests-split.yml`, line 97-99

**Current (broken):**
```yaml
concurrency:
  group: e2e-split-${{ github.workflow }}-${{ github.ref }}-${{ github.event.pull_request.head.sha || github.sha }}
  cancel-in-progress: true
```

**Fixed:**
```yaml
concurrency:
  group: ${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: true
```

**Rationale:**
- Remove `e2e-split-` prefix: redundant since `${{ github.workflow }}` already resolves to `"E2E Tests"`.
- Remove `${{ github.event.pull_request.head.sha || github.sha }}`: this is the root cause — makes every commit get its own group.
- `github.ref` ensures PRs use `refs/pull/N/merge` and branches use `refs/heads/branch-name`.

**Impact:** A new push to the same PR or branch will immediately cancel any in-progress E2E test run for that branch/PR.

### 4.2 FIX: `codecov-upload.yml` — SECONDARY OFFENDER

**File:** `.github/workflows/codecov-upload.yml`, line 21-23

**Current (broken):**
```yaml
concurrency:
  group: ${{ github.workflow }}-${{ github.ref_name }}-${{ github.run_id }}
  cancel-in-progress: true
```

**Fixed:**
```yaml
concurrency:
  group: ${{ github.workflow }}-${{ github.ref }}
  cancel-in-progress: true
```

**Rationale:**
- Remove `${{ github.run_id }}`: unique per run, completely defeats concurrency cancellation.
- Switch `github.ref_name` to `github.ref` for consistency with other workflows and to avoid name collisions between branches and tags with the same name.

**Impact:** A new push to the same branch will cancel any in-progress Codecov upload for that branch.

## 5. Workflows Without Concurrency Blocks (Review)

| Workflow | Risk | Recommendation |
|----------|------|----------------|
| `nightly-build.yml` | Low — schedule/dispatch only | **Optional**: Add `group: ${{ github.workflow }}` with `cancel-in-progress: false` |
| `supply-chain-verify.yml` | Low — schedule/dispatch/workflow_run | **Optional**: Add `group: ${{ github.workflow }}-${{ github.ref }}` with `cancel-in-progress: true` |
| `update-geolite2.yml` | Negligible — weekly schedule | No action needed |
| `gh_cache_cleanup.yml` | Negligible — manual only | No action needed |
| `container-prune.yml` | Low — PR + weekly schedule | **Optional**: Add concurrency for PR trigger runs |

## 6. Workflow Call Interaction Analysis

`e2e-tests-split.yml` defines `workflow_call` inputs, meaning it can be invoked by other workflows as a reusable workflow. However:

- **No workflow in the repository currently calls it via `uses:`**.
- References found in `nightly-build.yml` (line 104) and `weekly-nightly-promotion.yml` (lines 83, 443) are JavaScript code within `actions/github-script` steps that *monitor* workflow run status — they do not invoke `e2e-tests-split.yml` as a reusable workflow.
- The `pull_request` trigger on `e2e-tests-split.yml` is the main trigger that causes the queueing problem.

**Important note about `workflow_call` concurrency**: When a workflow is called via `workflow_call`, the concurrency block in the **called** workflow is evaluated in the caller's context. The simplified group (`${{ github.workflow }}-${{ github.ref }}`) works correctly in both direct-trigger and `workflow_call` contexts.

## 7. Risk Assessment

### Workflows Where We Should NOT Change Concurrency

| Workflow | Reason |
|----------|--------|
| `release-goreleaser.yml` | Releases must complete — canceling mid-publish could leave artifacts broken |
| `auto-versioning.yml` | Version bumps must complete atomically |
| `propagate-changes.yml` | Branch synchronization must complete |
| `docs.yml` (Pages deploy) | GitHub Pages deployment should not be interrupted |
| `weekly-nightly-promotion.yml` | Promotion PR creation must finish cleanly |
| `security-weekly-rebuild.yml` | Security rebuild must complete for compliance |
| `docs-to-issues.yml` | Issue creation should not be interrupted |
| `create-labels.yml` | Manual-only, singleton |
| `renovate.yml` | Dependency updates should complete |
| `caddy-major-monitor.yml` | Monitoring check must complete |
| `auto-add-to-project.yml` | Issue/PR project assignment must complete |

**All of these are correctly configured. Do not modify them.**

### Risks of the Proposed Fix

| Risk | Severity | Mitigation |
|------|----------|-----------|
| In-flight E2E results discarded on cancel | Low | Desired behavior — stale results for an old commit are useless |
| Codecov partial upload on cancel | Low | Codecov handles partial uploads gracefully; next full run uploads complete data |
| `workflow_call` context mismatch if caller added later | None | Fix uses standard pattern that works in both direct and called contexts |

## 8. Acceptance Criteria

- [ ] `e2e-tests-split.yml` concurrency group does not contain SHA or run_id
- [ ] `codecov-upload.yml` concurrency group does not contain SHA or run_id
- [ ] Pushing a new commit to a PR cancels any in-progress E2E test run on that PR
- [ ] Pushing a new commit to a PR cancels any in-progress Codecov upload on that PR
- [ ] All other 34 workflows remain unchanged
- [ ] No workflows with `cancel-in-progress: false` are modified

## 9. Implementation Plan

### Phase 1: Fix (Single PR)

| Task | File | Line(s) | Change |
|------|------|---------|--------|
| 1 | `.github/workflows/e2e-tests-split.yml` | 97-99 | Replace concurrency group: remove SHA, simplify to `${{ github.workflow }}-${{ github.ref }}` |
| 2 | `.github/workflows/codecov-upload.yml` | 21-23 | Replace concurrency group: remove `run_id`, simplify to `${{ github.workflow }}-${{ github.ref }}` |

### Phase 2: Validate

1. Push to a test branch, wait for workflows to start
2. Push again to the same branch within 60 seconds
3. Verify the first E2E run is labeled "cancelled" in the Actions tab
4. Verify first Codecov run is labeled "cancelled"
5. Verify all other workflows are unaffected

## 10. PR Slicing Strategy

**Decision: Single PR**

**Rationale:**
- Config-only change: 2 YAML files, ~4 lines changed total
- No code changes, no build changes, no runtime impact
- Changes are atomic and self-contained
- Rollback is a single revert commit
- Risk is minimal — worst case restores the existing (broken) behavior

**PR Scope:**

| ID | Scope | Files | Dependencies | Validation Gate |
|----|-------|-------|--------------|----------------|
| PR-1 | Fix concurrency groups | `e2e-tests-split.yml`, `codecov-upload.yml` | None | Push 2 commits in quick succession; confirm first run is canceled |

**Rollback:** `git revert <commit-sha>` — restores prior concurrency groups immediately.

## 11. Summary

| Metric | Value |
|--------|-------|
| Total workflows audited | 36 |
| Workflows with concurrency blocks | 31 |
| Workflows without concurrency blocks | 5 |
| **Workflows with SHA/run_id bug** | **2** |
| Workflows with intentional no-cancel | 11 |
| Workflows correctly configured | 18 |
| Files to change | 2 |
| Lines to change | ~4 |
