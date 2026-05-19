# Fix: GitHub Actions JS Injection in Weekly Nightly Promotion Workflow

**Issue**: #1022
**Branch**: `fix/weekly-promotion-js-injection`
**PR**: Targets `development`
**Date**: 2026-05-19
**File**: `.github/workflows/weekly-nightly-promotion.yml`

---

## 1. Introduction

### Overview

Issue #1022 is a **JavaScript injection vulnerability** in the
`weekly-nightly-promotion.yml` GitHub Actions workflow. When a user manually
triggers the workflow with a `reason` input containing an apostrophe (e.g.,
`"didn't run as scheduled"`), the value is interpolated directly into a
single-quoted JavaScript string literal, terminating the string prematurely
and producing a `SyntaxError`. The workflow step fails immediately.

A secondary defensive fix addresses the same anti-pattern in the
`notify-on-failure` job where dynamically-generated output values are also
interpolated directly into single-quoted JS strings.

### Objectives

1. Fix the primary injection that causes `SyntaxError` when `inputs.reason`
   contains an apostrophe or other metacharacter.
2. Defensively fix the `notify-on-failure` job to follow the same secure
   pattern, eliminating a latent class of bugs.
3. Apply zero changes outside the two affected steps — no refactoring, no
   other workflow files, no shell steps.

---

## 2. Research Findings

### 2.1 Confirmed Root Cause

The `Create Promotion PR` step (`create-promotion-pr` job, line 297) uses the
`actions/github-script` action and injects `${{ inputs.reason }}` directly
into the JavaScript source at line 310:

```javascript
// Line 310 — verbatim source as it exists today:
const triggerReason = '${{ inputs.reason }}' || 'Scheduled weekly promotion';
```

When the user enters `didn't run as scheduled`, the Actions runner performs
template expansion **before** the script is executed, producing:

```javascript
const triggerReason = 'didn't run as scheduled' || 'Scheduled weekly promotion';
```

The single quote inside `didn't` terminates the string literal. The remaining
token `t run as scheduled` is an unexpected identifier, and V8 throws:

```
SyntaxError: Unexpected identifier 't'
```

The entire `create-promotion-pr` job fails, the promotion PR is never created,
and the `notify-on-failure` job may or may not fire depending on health check
state.

### 2.2 Secondary Risk (Defensive Fix)

The `Create Failure Issue` step (`notify-on-failure` job, line 486) uses the
same pattern for two job output values:

```javascript
// Lines 490–491 — verbatim source:
const failureReason = '${{ needs.check-nightly-health.outputs.failure_reason }}';
const latestRunUrl  = '${{ needs.check-nightly-health.outputs.latest_run_url }}';
```

`failure_reason` is assembled from workflow file names, conclusion strings, and
GitHub HTML URLs (e.g.,
`quality-checks.yml failure (https://github.com/owner/repo/actions/runs/123)`).
In practice these values never contain apostrophes, so this does not
reproducibly fail today. However, any future change to how `failure_reason` is
composed could silently reintroduce the bug. The fix is applied proactively.

### 2.3 Correct Pattern: Environment Variable Passthrough

The `actions/github-script` action supports an `env:` block at the step level.
Values assigned to `env:` are expanded by the Actions runner (the YAML context
layer) and then passed to the Node.js process as environment variables. The
`script:` body reads them via `process.env.*`. The value is **never
interpolated into JavaScript source code** — it arrives as a runtime string,
making injection structurally impossible.

This is the canonical pattern in GitHub's own security-hardening guide for
Actions and is the standard fix for this class of vulnerability
(`CWE-94: Improper Control of Generation of Code`).

---

## 3. Requirements (EARS Notation)

| ID | Requirement |
|----|-------------|
| R-01 | **WHEN** a user manually triggers `weekly-nightly-promotion.yml` with a `reason` input containing an apostrophe, single quote, double quote, backslash, or any other JavaScript metacharacter, **THE SYSTEM SHALL** complete the `Create Promotion PR` step without a `SyntaxError`. |
| R-02 | **WHEN** the `Create Promotion PR` step completes successfully, **THE SYSTEM SHALL** use the literal value of `inputs.reason` (verbatim, unmodified) as the trigger reason in the PR body. |
| R-03 | **WHEN** `inputs.reason` is empty or absent (scheduled run), **THE SYSTEM SHALL** fall back to the string `'Scheduled weekly promotion'` as the trigger reason. |
| R-04 | **WHEN** the `notify-on-failure` job's `Create Failure Issue` step executes, **THE SYSTEM SHALL** read `failure_reason` and `latest_run_url` from environment variables rather than from interpolated JavaScript string literals. |
| R-05 | **THE SYSTEM SHALL NOT** modify any step, job, or file outside the two affected steps (`Create Promotion PR` and `Create Failure Issue`). |
| R-06 | **THE SYSTEM SHALL NOT** alter the observable behaviour of the workflow for any input that does not contain JavaScript metacharacters (i.e., existing scheduled runs remain functionally identical). |

---

## 4. Full Expression Audit

This section audits every `${{ }}` expression that appears inside a
`script: |` block (JavaScript context) in the file. Shell `run:` steps are
not evaluated as code and are **out of scope for injection analysis**.

### 4.1 `check-nightly-health` job — `Check Nightly Workflow Status` step

| Line | Expression | Context | Risk Assessment |
|------|-----------|---------|----------------|
| 53 | `'${{ inputs.skip_workflow_check }}'` | Single-quoted JS string | **SAFE.** Declared `type: boolean`; Actions renders only `true` or `false`. No special characters possible. |

### 4.2 `create-promotion-pr` job — `Check for Existing PR` step

| Line | Expression | Context | Risk Assessment |
|------|-----------|---------|----------------|
| 284 | `` `${context.repo.owner}:${{ env.SOURCE_BRANCH }}` `` | Backtick template literal | **SAFE.** `SOURCE_BRANCH` is a static workflow-level env var (`nightly`). No user-controlled input. Template literals do not amplify single-quote injection. |
| 285 | `'${{ env.TARGET_BRANCH }}'` | Single-quoted JS string | **SAFE.** `TARGET_BRANCH` is a static workflow-level env var (`main`). No special characters. |

### 4.3 `create-promotion-pr` job — `Create Promotion PR` step

| Line | Expression | Context | Risk Assessment |
|------|-----------|---------|----------------|
| 305 | `'${{ steps.commits.outputs.date }}'` | Single-quoted JS string | **SAFE.** Output of `date -u +%Y-%m-%d`; format is always `YYYY-MM-DD` (digits and hyphens only). |
| 306 | `'${{ steps.commits.outputs.commit_count }}'` | Single-quoted JS string | **SAFE.** Output of `git rev-list --count`; always a decimal integer. |
| 307 | `'${{ steps.commits.outputs.files_changed }}'` | Single-quoted JS string | **SAFE.** Output of `git diff --stat \| tail -1`; standard git summary (`N files changed, M insertions(+), K deletions(-)`). Git never produces apostrophes in this output. |
| **310** | **`'${{ inputs.reason }}'`** | **Single-quoted JS string** | **🔴 VULNERABLE — PRIMARY BUG.** Free-text user input. Any apostrophe terminates the JS string literal. **Fix via `process.env`.** |
| 342 | `` `...${{ github.server_url }}/${{ github.repository }}/actions/runs/${{ github.run_id }}...` `` | Backtick template literal | **SAFE.** `github.server_url` is a URL, `github.repository` is `owner/repo`, `github.run_id` is numeric. All are GitHub context values with no special characters. |
| 350 | `'${{ env.SOURCE_BRANCH }}'` | Single-quoted JS string | **SAFE.** Static env var `nightly`. |
| 351 | `'${{ env.TARGET_BRANCH }}'` | Single-quoted JS string | **SAFE.** Static env var `main`. |

### 4.4 `create-promotion-pr` job — `Update Existing PR` step

| Line | Expression | Context | Risk Assessment |
|------|-----------|---------|----------------|
| 405 | `${{ steps.existing-pr.outputs.pr_number }}` | **Unquoted** bare JS value | **SAFE.** PR number is always an integer rendered directly as a numeric literal. No injection possible. |
| 412 | `` `...${{ github.server_url }}/${{ github.repository }}/actions/runs/${{ github.run_id }}...` `` | Backtick template literal | **SAFE.** Same as line 342. |
| 416 | `'${{ steps.existing-pr.outputs.pr_url }}'` | Single-quoted JS string | **SAFE in practice.** GitHub PR URLs have the fixed format `https://github.com/owner/repo/pull/N`. No apostrophes possible in a well-formed URL. |

### 4.5 `notify-on-failure` job — `Create Failure Issue` step

| Line | Expression | Context | Risk Assessment |
|------|-----------|---------|----------------|
| 489 | `'${{ needs.check-nightly-health.outputs.is_healthy }}'` | Single-quoted JS string | **SAFE.** Value is always `'true'` or `'false'` — set by script with literal strings. |
| **490** | **`'${{ needs.check-nightly-health.outputs.failure_reason }}'`** | **Single-quoted JS string** | **🟡 LOW RISK — DEFENSIVE FIX.** Currently safe (workflow file names, conclusion words, and URLs contain no apostrophes), but the value is dynamically constructed. A future change to `failure_reason` composition could silently reintroduce injection. **Fix via `process.env`.** |
| **491** | **`'${{ needs.check-nightly-health.outputs.latest_run_url }}'`** | **Single-quoted JS string** | **🟡 LOW RISK — DEFENSIVE FIX.** GitHub run URLs never contain apostrophes, but same defensive rationale applies. **Fix via `process.env`.** |
| 492 | `'${{ needs.create-promotion-pr.result }}'` | Single-quoted JS string | **SAFE.** GitHub job result is a fixed enum: `success`, `failure`, `cancelled`, `skipped`. |
| 506, 528 | `` `...${{ github.server_url }}/${{ github.repository }}/actions/runs/${{ github.run_id }}...` `` | JS template string in YAML multiline | **SAFE.** Same analysis as line 342. |

### 4.6 `summary` job — `Generate Summary` step (shell `run:`)

| Lines | Expression | Context | Risk Assessment |
|-------|-----------|---------|----------------|
| 603–607, 628 | Various `${{ needs.*.outputs.* }}` and `${{ github.* }}` | Bash `run:` step, **not JavaScript** | **OUT OF SCOPE.** Shell variable assignment (`VAR="${{ expr }}"`) is not code injection. Double-quoting prevents word-splitting; values are only used in `echo`. No fix required. |

### 4.7 Audit Conclusion

**Two locations require changes.** All other `${{ }}` expressions in JS
`script:` blocks are safe by analysis. The pattern
`${{ env.SOURCE_BRANCH }}` / `${{ env.TARGET_BRANCH }}` (static workflow env
vars `nightly` and `main`) are safe enough to leave as-is; converting them
would add noise without meaningful security benefit.

---

## 5. Design

### 5.1 Fix 1 — `Create Promotion PR` Step (Primary Bug)

**File**: `.github/workflows/weekly-nightly-promotion.yml`
**Scope**: Lines 297–311 (step header + first four JS variable declarations)

Add an `env:` block between `uses:` and `with:`, then replace the interpolated
JS string at line 310 with a `process.env` read.

#### Before (lines 297–311):

```yaml
      - name: Create Promotion PR
        id: create-pr
        if: steps.check-diff.outputs.skipped != 'true' && steps.existing-pr.outputs.exists != 'true'
        uses: actions/github-script@3a2844b7e9c422d3c10d287c895573f7108da1b3 # v9
        with:
          script: |
            const fs = require('fs');

            const date = '${{ steps.commits.outputs.date }}';
            const commitCount = '${{ steps.commits.outputs.commit_count }}';
            const filesChanged = '${{ steps.commits.outputs.files_changed }}';
            const commitLog = fs.readFileSync('/tmp/commit_log.md', 'utf8');

            const triggerReason = '${{ inputs.reason }}' || 'Scheduled weekly promotion';
```

#### After:

```yaml
      - name: Create Promotion PR
        id: create-pr
        if: steps.check-diff.outputs.skipped != 'true' && steps.existing-pr.outputs.exists != 'true'
        uses: actions/github-script@3a2844b7e9c422d3c10d287c895573f7108da1b3 # v9
        env:
          TRIGGER_REASON: ${{ inputs.reason }}
        with:
          script: |
            const fs = require('fs');

            const date = '${{ steps.commits.outputs.date }}';
            const commitCount = '${{ steps.commits.outputs.commit_count }}';
            const filesChanged = '${{ steps.commits.outputs.files_changed }}';
            const commitLog = fs.readFileSync('/tmp/commit_log.md', 'utf8');

            const triggerReason = process.env.TRIGGER_REASON || 'Scheduled weekly promotion';
```

**Diff summary**:

| Action | Detail |
|--------|--------|
| Insert 2 lines | `        env:` and `          TRIGGER_REASON: ${{ inputs.reason }}` after the `uses:` line |
| Modify 1 line | `'${{ inputs.reason }}'` → `process.env.TRIGGER_REASON` on the `triggerReason` declaration |

> **Why `env:` before `with:`?** YAML step keys have no required ordering, but
> convention in this file places `env:` between the action reference and its
> `with:` block. This minimises diff noise.

> **Why not escape the apostrophe in the expression?** Escaping inside an
> Actions expression (e.g., `replace(inputs.reason, '''', '\''')`) is fragile
> and does not cover double quotes, backslashes, or newlines. The `process.env`
> approach is the only architecturally correct solution.

### 5.2 Fix 2 — `Create Failure Issue` Step (Defensive)

**File**: `.github/workflows/weekly-nightly-promotion.yml`
**Scope**: Lines 486–492 (step header + first four JS variable declarations)

Add an `env:` block and replace the two `failure_reason`/`latest_run_url`
interpolations with `process.env` reads.

#### Before (lines 486–492):

```yaml
      - name: Create Failure Issue
        uses: actions/github-script@3a2844b7e9c422d3c10d287c895573f7108da1b3 # v9
        with:
          script: |
            const isHealthy = '${{ needs.check-nightly-health.outputs.is_healthy }}';
            const failureReason = '${{ needs.check-nightly-health.outputs.failure_reason }}';
            const latestRunUrl = '${{ needs.check-nightly-health.outputs.latest_run_url }}';
            const prResult = '${{ needs.create-promotion-pr.result }}';
```

#### After:

```yaml
      - name: Create Failure Issue
        uses: actions/github-script@3a2844b7e9c422d3c10d287c895573f7108da1b3 # v9
        env:
          FAILURE_REASON: ${{ needs.check-nightly-health.outputs.failure_reason }}
          LATEST_RUN_URL: ${{ needs.check-nightly-health.outputs.latest_run_url }}
        with:
          script: |
            const isHealthy = '${{ needs.check-nightly-health.outputs.is_healthy }}';
            const failureReason = process.env.FAILURE_REASON || '';
            const latestRunUrl = process.env.LATEST_RUN_URL || 'N/A';
            const prResult = '${{ needs.create-promotion-pr.result }}';
```

**Diff summary**:

| Action | Detail |
|--------|--------|
| Insert 3 lines | `        env:`, `          FAILURE_REASON: ${{ needs.check-nightly-health.outputs.failure_reason }}`, `          LATEST_RUN_URL: ${{ needs.check-nightly-health.outputs.latest_run_url }}` after the `uses:` line |
| Modify 1 line | `'${{ ...failure_reason }}'` → `process.env.FAILURE_REASON \|\| ''` |
| Modify 1 line | `'${{ ...latest_run_url }}'` → `process.env.LATEST_RUN_URL \|\| 'N/A'` |

> **Fallback values**: `|| ''` and `|| 'N/A'` match what `check-nightly-health`
> sets when the health check is skipped (`failure_reason: ''`,
> `latest_run_url: 'N/A - check skipped'`). Behaviour for the non-apostrophe
> path is preserved exactly.

### 5.3 Unchanged Expressions (Explicitly Safe)

The following single-quoted JS string interpolations are **not changed**
because they are safe by analysis (see §4):

| Line | Expression | Reason not changed |
|------|-----------|-------------------|
| 53 | `'${{ inputs.skip_workflow_check }}'` | Boolean type; renders only `true`/`false` |
| 285 | `'${{ env.TARGET_BRANCH }}'` | Static env var `main` |
| 305 | `'${{ steps.commits.outputs.date }}'` | `YYYY-MM-DD` format only |
| 306 | `'${{ steps.commits.outputs.commit_count }}'` | Integer only |
| 307 | `'${{ steps.commits.outputs.files_changed }}'` | Git stat summary; no apostrophes |
| 350, 351 | `'${{ env.SOURCE_BRANCH }}'`, `'${{ env.TARGET_BRANCH }}'` | Static env vars |
| 416 | `'${{ steps.existing-pr.outputs.pr_url }}'` | GitHub PR URL; no apostrophes in URLs |
| 489 | `'${{ needs.check-nightly-health.outputs.is_healthy }}'` | Fixed `true`/`false` |
| 492 | `'${{ needs.create-promotion-pr.result }}'` | Fixed GitHub result enum |

---

## 6. Implementation Plan

This is a 5-line change to one file. No scaffolding, no test setup, no
multi-phase implementation. The entire work is one atomic commit.

### Phase 1 — Playwright Tests

No UI/UX behaviour changes are introduced by this fix. Playwright E2E tests
are not required.

### Phase 2 — Backend Implementation

Not applicable. This fix is in a CI workflow file only.

### Phase 3 — Frontend Implementation

Not applicable.

### Phase 4 — Integration and Testing

| # | Task | File | Scope |
|---|------|------|-------|
| 1 | Add `env: TRIGGER_REASON` block to `Create Promotion PR` step | `weekly-nightly-promotion.yml` | 2 lines inserted after line 300 |
| 2 | Replace `'${{ inputs.reason }}'` with `process.env.TRIGGER_REASON` | `weekly-nightly-promotion.yml` | Line 310 modified |
| 3 | Add `env: FAILURE_REASON / LATEST_RUN_URL` block to `Create Failure Issue` step | `weekly-nightly-promotion.yml` | 3 lines inserted after line 487 |
| 4 | Replace `'${{ ...failure_reason }}'` with `process.env.FAILURE_REASON \|\| ''` | `weekly-nightly-promotion.yml` | Line 490 modified |
| 5 | Replace `'${{ ...latest_run_url }}'` with `process.env.LATEST_RUN_URL \|\| 'N/A'` | `weekly-nightly-promotion.yml` | Line 491 modified |

Run validation gates after each task (see §8).

### Phase 5 — Documentation and Deployment

No documentation changes required. Merge to `development`; the workflow runs
on `nightly` and `main` branches and will pick up the fix on next trigger.

---

## 7. Commit Slicing Strategy

**Decision**: Single PR, single commit. This is a surgical security fix with
no logic changes, no cross-domain impact, and no migration risk.

**Rationale**: The entire change touches one file, two steps, five lines. A
single atomic commit makes the fix easy to cherry-pick to `nightly` or `main`
if needed, and keeps the diff reviewable in under 30 seconds.

### Commit 1 (and only commit)

```
fix(ci): pass workflow inputs via env vars to prevent JS injection

In weekly-nightly-promotion.yml, two github-script steps interpolated
${{ }} expressions directly into single-quoted JS string literals.
When inputs.reason contained an apostrophe (e.g. "didn't run"),
the string terminated prematurely, producing SyntaxError.

Pass user-controlled and dynamic values through step-level env: blocks
and read them via process.env.* in the script body. This eliminates
the injection vector entirely.

Primary fix: Create Promotion PR step (inputs.reason -> TRIGGER_REASON)
Defensive fix: Create Failure Issue step (failure_reason, latest_run_url)

Closes #1022
```

**Scope**: `.github/workflows/weekly-nightly-promotion.yml` only
**Files changed**: 1
**Lines added**: 5 (`env:` keys and values)
**Lines modified**: 3 (JS variable declarations)

**PR description**:

> ## Summary
>
> Fixes #1022 — `SyntaxError` when manually triggering the weekly promotion
> workflow with a reason containing an apostrophe.
>
> ## Root Cause
>
> ```javascript
> // Before — VULNERABLE: apostrophe in input terminates the JS string
> const triggerReason = '${{ inputs.reason }}' || 'Scheduled weekly promotion';
>
> // After — SAFE: value arrives as an environment variable at runtime
> const triggerReason = process.env.TRIGGER_REASON || 'Scheduled weekly promotion';
> ```
>
> ## Testing
>
> Manually trigger with reason `didn't run as scheduled` and verify
> `Create Promotion PR` step completes without error.

**Rollback**: `git revert <sha>` — zero risk, one commit.

---

## 8. Validation Gates

### Gate 1 — YAML Lint

Run `yamllint` against the modified file before merging. The `env:` block must
parse correctly and indentation must be consistent with the rest of the file
(2-space indent, step keys at 8 spaces).

```bash
yamllint .github/workflows/weekly-nightly-promotion.yml
```

### Gate 2 — actionlint

`actionlint` statically analyses GitHub Actions workflows and will catch:
- Incorrect `env:` key names
- Invalid expression syntax
- Undefined step outputs used in `env:` values

```bash
actionlint .github/workflows/weekly-nightly-promotion.yml
```

### Gate 3 — Manual Trigger Test (post-merge)

1. Navigate to **Actions → Weekly Nightly to Main Promotion → Run workflow**
2. In the `reason` field, enter: `didn't run as scheduled`
3. Set `skip_workflow_check` to `true` (bypasses health check during test)
4. Click **Run workflow**
5. **Expected**: The `Create Promotion PR` step completes without error.
   Either a PR is created (if nightly has changes) or the step is skipped
   (if nightly is already in sync with main). No `SyntaxError` appears.
6. **Failure indicator**: Any log line containing `SyntaxError`, `Unexpected
   identifier`, or `Unexpected token` in the `Create Promotion PR` step.

### Gate 4 — Regression Check

Trigger the workflow with a plain reason (no apostrophes): `Ad-hoc test run`.
Verify the `triggerReason` variable correctly appears in the PR body with the
literal string provided (same behaviour as before the fix).

---

## 9. Acceptance Criteria

| # | Criterion | Pass Condition |
|---|-----------|---------------|
| AC-1 | Workflow completes with reason containing `'` (apostrophe) | `Create Promotion PR` step exits 0; no `SyntaxError` in logs |
| AC-2 | Workflow completes with reason containing `"` (double quote) | Same as AC-1 |
| AC-3 | Workflow completes with reason containing `\` (backslash) | Same as AC-1 |
| AC-4 | Scheduled run (no manual reason) falls back correctly | PR body shows `Scheduled weekly promotion` as trigger |
| AC-5 | Plain reason (no special chars) passes through verbatim | PR body shows the exact string entered |
| AC-6 | `Create Failure Issue` uses `process.env.FAILURE_REASON` | No raw `${{ }}` expansion in JS string context |
| AC-7 | No other steps or files modified | `git diff` touches only `weekly-nightly-promotion.yml` |
| AC-8 | YAML lint passes | `yamllint` exits 0 |
| AC-9 | actionlint passes | `actionlint` exits 0 |

---

## 10. Out-of-Scope Items

The following are **explicitly not part of this fix**:

1. **No changes to shell `run:` steps** — `${{ }}` in bash variable
   assignments is not code injection.
2. **No changes to `check-nightly-health` job** — `inputs.skip_workflow_check`
   is a `type: boolean` input; no injection risk.
3. **No changes to other single-quoted interpolations in JS** — Lines 285,
   305, 306, 307, 350, 351, 416, 489, 492 are safe by analysis (see §4).
4. **No changes to any other workflow files**.
5. **No changes to the `Update Existing PR` step** — Line 405 (`pr_number` as
   a bare numeric literal) and line 416 (`pr_url` as a GitHub URL) are safe.
6. **No refactoring of the JS body** — Only the minimum variable declarations
   are changed; all script logic is untouched.
7. **No changes to the `summary` job** — Its `run:` step uses `${{ }}` in
   bash, not JavaScript.
8. **No version bump to `actions/github-script`** — The pinned commit SHA
   (`3a2844b7e9c422d3c10d287c895573f7108da1b3`) must not be changed as part of
   this fix.
