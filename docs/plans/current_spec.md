## 1. Introduction

### Overview

`Nightly Build & Package` currently has two active workflow failures that must
be fixed together in one minimal-scope PR:

1. SBOM generation failure in `Generate SBOM` (Syft fetch/version resolution).
2. Dispatch failure from nightly workflow with `Missing required input
   'pr_number' not provided`.

This plan hard-locks runtime code changes to
`.github/workflows/nightly-build.yml` only.

### Objectives

1. Restore deterministic nightly SBOM generation.
2. Enforce strict default-deny dispatch behavior for non-PR nightly events
   (`schedule`, `workflow_dispatch`).
3. Preserve GitHub Actions best practices: pinned SHAs, least privilege, and
   deterministic behavior.
4. Keep both current failures in a single scope and do not pivot to unrelated fixes.
5. Remove `security-pr.yml` from nightly dispatch list unless a hard
   requirement is proven.

## 2. Research Findings

### 2.1 Primary Workflow Scope

File analyzed: `.github/workflows/nightly-build.yml`

Relevant areas:

1. Job `build-and-push-nightly`, step `Generate SBOM` uses
   `anchore/sbom-action@17ae1740179002c89186b61233e0f892c3118b11`.
2. Job `trigger-nightly-validation` dispatches downstream workflows using
   `actions/github-script` and currently includes `security-pr.yml`.

### 2.2 Root Cause: Missing `pr_number`

Directly related called workflow:

1. `.github/workflows/security-pr.yml`
2. Trigger contract includes:
   - `workflow_dispatch.inputs.pr_number.required: true`

Impact:

1. Nightly dispatcher invokes `createWorkflowDispatch` for `security-pr.yml`
   without `pr_number`.
2. For nightly non-PR contexts (scheduled/manual nightly), there is no natural
   PR number, so dispatch fails by contract.
3. PR lookup by nightly head SHA is not a valid safety mechanism for nightly
   non-PR trigger types and must not be relied on for `schedule` or
   `workflow_dispatch`.

### 2.3 Decision: Remove PR-Only Workflow from Nightly Dispatch List

Assessment result:

1. No hard requirement was found that requires nightly workflow to dispatch
   `security-pr.yml`.
2. `security-pr.yml` is contractually PR/manual-oriented because it requires
   `pr_number`.
3. Keeping it in nightly fan-out adds avoidable failure risk and encourages
   invalid context synthesis.

Decision:

1. Remove `security-pr.yml` from nightly dispatch list.
2. Keep strict default-deny guard logic to prevent accidental future dispatch
   from non-PR events.

Risk reduction from removal:

1. Eliminates `pr_number` contract mismatch in nightly non-PR events.
2. Removes a class of false failures from nightly reliability metrics.
3. Simplifies dispatcher logic and review surface.

### 2.4 Root Cause: SBOM/Syft Fetch Failure

Observed behavior indicates Syft retrieval/version resolution instability during
the SBOM step. In current workflow, no explicit `syft-version` is set in
`nightly-build.yml`, so resolution is not explicitly pinned at the workflow
layer.

### 2.5 Constraints and Policy Alignment

1. Keep action SHAs pinned.
2. Keep permission scopes unchanged unless required.
3. Keep change minimal and limited to nightly workflow path only.

## 3. Technical Specification (EARS)

1. WHEN nightly runs from `schedule` or `workflow_dispatch`, THE SYSTEM SHALL
   enforce strict default-deny for PR-only dispatches.

2. WHEN nightly runs from `schedule` or `workflow_dispatch`, THE SYSTEM SHALL
   NOT perform PR-number lookup from nightly head SHA.

3. WHEN evaluating downstream nightly dispatches, THE SYSTEM SHALL exclude
   `security-pr.yml` from nightly dispatch targets unless a hard requirement
   is explicitly introduced and documented.

4. IF `security-pr.yml` is reintroduced in the future, THEN THE SYSTEM SHALL
   dispatch it ONLY when a real PR context includes a concrete `pr_number`,
   and SHALL deny by default in all other contexts.

5. WHEN `Generate SBOM` runs in nightly, THE SYSTEM SHALL use a deterministic
   two-stage strategy in the same PR scope:
   - Primary path: `syft-version: v1.42.1` via `anchore/sbom-action`
   - In-PR fallback path: explicit Syft CLI installation/generation
     with pinned version/checksum and hard verification

6. IF primary SBOM generation fails or does not produce a valid file, THEN THE
   SYSTEM SHALL execute fallback generation and SHALL fail the job when fallback
   also fails or output validation fails.

7. THE SYSTEM SHALL keep GitHub Actions pinned to immutable SHAs and SHALL NOT
   broaden token permissions for this fix.

## 4. Exact Implementation Edits

### 4.1 `.github/workflows/nightly-build.yml`

### Edit A: Harden downstream dispatch for non-PR triggers

Location: job `trigger-nightly-validation`, step
`Dispatch Missing Nightly Validation Workflows`.

Exact change intent:

1. Remove `security-pr.yml` from the nightly dispatch list.
2. Keep dispatch for `e2e-tests-split.yml`, `codecov-upload.yml`,
   `supply-chain-verify.yml`, and `codeql.yml` unchanged.
3. Add explicit guard comments and logging stating non-PR nightly events are
   default-deny for PR-only workflows.
4. Explicitly prohibit PR number synthesis and prohibit PR lookup from nightly
   SHA for `schedule` and `workflow_dispatch`.

Implementation shape (script-level):

1. Keep workflow list explicit.
2. Keep a local denylist/set for PR-only workflows and ensure they are never
   dispatched from nightly non-PR events.
3. No PR-number inputs are synthesized from nightly SHA or non-PR context.
4. No PR lookup calls are executed for nightly non-PR events.

### Edit B: Stabilize Syft source in `Generate SBOM`

Location: job `build-and-push-nightly`, step `Generate SBOM`.

Exact change intent:

1. Keep existing pinned `anchore/sbom-action` SHA unless evidence shows that SHA
   itself is the failure source.
2. Add explicit `syft-version: v1.42.1` in `with:` block as the primary pin.
3. Set the primary SBOM step to `continue-on-error: true` to allow deterministic
   in-PR fallback execution.
4. Add fallback step gated on primary step failure OR missing/invalid output:
   - Install Syft CLI `v1.42.1` from official release with checksum validation.
   - Generate `sbom-nightly.json` via CLI.
5. Add mandatory verification step (no `continue-on-error`) with explicit
   pass/fail criteria:
   - `sbom-nightly.json` exists.
   - file size is greater than 0 bytes.
   - JSON parses successfully (`jq empty`).
   - expected top-level fields exist for selected format.
6. If verification fails, job fails. SBOM cannot pass silently without
   generated artifact.

### 4.2 Scope Lock

1. No edits to `.github/workflows/security-pr.yml` in this plan.
2. Contract remains unchanged: `workflow_dispatch.inputs.pr_number.required: true`.

## 5. Reconfirmation: Non-Target Files

No changes required:

1. `.gitignore`
2. `codecov.yml`
3. `.dockerignore`
4. `Dockerfile`

Rationale:

1. Both failures are workflow orchestration issues, not source-ignore, coverage
   policy, Docker context, or image build recipe issues.

## 6. Risks and Mitigations

| Risk | Impact | Mitigation |
|---|---|---|
| `security-pr.yml` accidentally dispatched in non-PR mode | Low | Remove from nightly dispatch list and enforce default-deny comments/guards |
| Primary Syft acquisition fails (`v1.42.1`) | Medium | Execute deterministic in-PR fallback with pinned checksum and hard output verification |
| SBOM step appears green without real artifact | High | Mandatory verification step with explicit file/JSON checks and hard fail |
| Action SHA update introduces side effects | Medium | Limit SHA change to `Generate SBOM` step only and validate end-to-end nightly path |
| Over-dispatch/under-dispatch in validation job | Low | Preserve existing dispatch logic for all non-PR-dependent workflows |

## 7. Rollback Plan

1. Revert runtime behavior changes in
   `.github/workflows/nightly-build.yml`:
   - `trigger-nightly-validation` dispatch logic
   - `Generate SBOM` primary + fallback + verification sequence
2. Re-run nightly dispatch manually to verify previous baseline runtime
   behavior.

Rollback scope: runtime workflow behavior only in
`.github/workflows/nightly-build.yml`. Documentation updates are not part of
runtime rollback.

## 8. Validation Plan

### 8.1 Static Validation

```bash
cd /projects/Charon
pre-commit run actionlint --files .github/workflows/nightly-build.yml
```

### 8.2 Behavioral Validation (Nightly non-PR)

```bash
gh workflow run nightly-build.yml --ref nightly -f reason="nightly dual-fix validation" -f skip_tests=true
gh run list --workflow "Nightly Build & Package" --branch nightly --limit 1
gh run view <run-id> --json databaseId,headSha,event,status,conclusion,createdAt
gh run view <run-id> --log
```

Expected outcomes:

1. `Generate SBOM` succeeds through primary path or deterministic fallback and
    `sbom-nightly.json` is uploaded.
2. Dispatch step does not attempt `security-pr.yml` from nightly run.
3. No `Missing required input 'pr_number' not provided` error.
4. Both targeted nightly failures are resolved in the same run scope:
   `pr_number` dispatch failure and Syft/SBOM failure.

### 8.3 Explicit Negative Dispatch Verification (Run-Scoped/Time-Scoped)

Verify `security-pr.yml` was not dispatched by this specific nightly run using
time scope and actor scope (not SHA-only):

```bash
RUN_JSON=$(gh run view <nightly-run-id> --json databaseId,createdAt,updatedAt,event,headBranch)
START=$(echo "$RUN_JSON" | jq -r '.createdAt')
END=$(echo "$RUN_JSON" | jq -r '.updatedAt')

gh api repos/<owner>/<repo>/actions/workflows/security-pr.yml/runs \
   --paginate \
   -f event=workflow_dispatch | \
jq --arg start "$START" --arg end "$END" '
   [ .workflow_runs[]
      | select(.created_at >= $start and .created_at <= $end)
      | select(.head_branch == "nightly")
      | select(.triggering_actor.login == "github-actions[bot]")
   ] | length'
```

Expected result: `0`

### 8.4 Positive Validation: Manual `security-pr.yml` Dispatch Still Works

Run a manual dispatch with a valid PR number and verify successful start:

```bash
gh workflow run security-pr.yml --ref <pr-branch> -f pr_number=<valid-pr-number>
gh run list --workflow "Security Scan (PR)" --limit 5 \
   --json databaseId,event,status,conclusion,createdAt,headBranch
gh run view <security-pr-run-id> --log
```

Expected results:

1. Workflow is accepted (no missing-input validation errors).
2. Run event is `workflow_dispatch`.
3. Run completes according to existing workflow behavior.

### 8.5 Contract Validation (No Contract Change)

1. `security-pr.yml` contract remains PR/manual specific and unchanged.
2. Nightly non-PR paths do not consume or synthesize `pr_number`.

## 9. Acceptance Criteria

1. `Nightly Build & Package` no longer fails in `Generate SBOM` due to Syft
   fetch/version resolution, with deterministic in-PR fallback.
2. Nightly validation dispatch no longer fails with missing required
   `pr_number`.
3. For non-PR nightly triggers (`schedule`/`workflow_dispatch`), PR-only
   dispatch of `security-pr.yml` is default-deny and not attempted from nightly
   dispatch targets.
4. Workflow remains SHA-pinned and permissions are not broadened.
5. Validation evidence includes explicit run-scoped/time-scoped proof that
   `security-pr.yml` was not dispatched by the tested nightly run.
6. No changes made to `.gitignore`, `codecov.yml`, `.dockerignore`, or
   `Dockerfile`.
7. Manual dispatch of `security-pr.yml` with valid `pr_number` is validated to
   still work.
8. SBOM step fails hard when neither primary nor fallback path produces a valid
   SBOM artifact.

## 10. PR Slicing Strategy

### Decision

Single PR.

### Trigger Reasons

1. Changes are tightly coupled inside one workflow path.
2. Shared validation path (nightly run) verifies both fixes together.
3. Rollback safety is high with one-file revert.

### Ordered Slices

#### PR-1: Nightly Dual-Failure Workflow Fix

Scope:

1. `.github/workflows/nightly-build.yml` only.
2. SBOM Syft stabilization with explicit tag pin + fallback rule.
3. Remove `security-pr.yml` from nightly dispatch list and enforce strict
   default-deny semantics for non-PR nightly events.

Files:

1. `.github/workflows/nightly-build.yml`
2. `docs/plans/current_spec.md`

Dependencies:

1. `security-pr.yml` keeps required `workflow_dispatch` `pr_number` contract.

Validation gates:

1. `actionlint` passes.
2. Nightly manual dispatch run passes both targeted failure points.
3. SBOM artifact upload succeeds through primary path or fallback path.
4. Explicit run-scoped/time-scoped negative check confirms zero
   bot-triggered `security-pr.yml` dispatches during the nightly run window.
5. Positive manual dispatch check with valid `pr_number` succeeds.

Rollback and contingency:

1. Revert PR-1.
2. If both primary and fallback Syft paths fail, treat as blocking regression
   and do not merge until generation criteria pass.

## 11. Complexity Estimate

1. Implementation complexity: Low.
2. Validation complexity: Medium (requires workflow run completion).
3. Blast radius: Low (single workflow file, no runtime code changes).
