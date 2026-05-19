# QA Audit Report — Issue #1022: Script Injection Fix Validation

| Field | Value |
|-------|-------|
| Date | 2026-05-19 |
| Branch | `hotfix/ci` |
| HEAD | `c1305dd3` — Merge pull request #1028 from Wikid82/main |
| Scope | `.github/workflows/weekly-nightly-promotion.yml` only |
| Issue | [#1022](https://github.com/Wikid82/Charon/issues/1022) — Script injection in `github-script` action blocks |
| Auditor | QA Security Agent |
| Verdict | ⚠️ **CONDITIONAL FAIL** — Fix is partial; one user-controlled input remains inline |

---

## Scope Clarification

The `hotfix/ci` branch contains no Go source, no TypeScript/React source, and no Dockerfile changes.
The only file changed is `.github/workflows/weekly-nightly-promotion.yml`.

The following scopes are **explicitly out of scope** for this audit:
- Backend unit tests and coverage
- Frontend unit tests and coverage
- GORM security scan (no model/database code changed)
- Playwright E2E tests (no application code changed)
- Local patch coverage report

---

## Changed File

| File | Type |
|------|------|
| `.github/workflows/weekly-nightly-promotion.yml` | GitHub Actions workflow (634 lines) |

---

## Tool Results

### 1. yamllint — PASS (with pre-existing style warnings)

```
yamllint .github/workflows/weekly-nightly-promotion.yml
```

Result: ~55 `line-length` errors (lines exceeding 80 chars) and minor style warnings:

| Line | Type | Detail |
|------|------|--------|
| 1 | warning | Missing `---` document start |
| 6 | warning | Truthy value — use `true` instead of `on` |
| 50, 209, 277, 300, 404, 430, 488 | warning | Missing space before comment |
| Multiple | error | Line length > 80 chars |

**Assessment**: None of these issues were introduced by this fix. The `line-length` rule is a style preference, not a functionality or security concern. The YAML is structurally valid.

---

### 2. actionlint — ✅ PASS

```
actionlint -verbose .github/workflows/weekly-nightly-promotion.yml
Exit: 0
```

**0 errors, 0 warnings.**

Note: `pyflakes` was unavailable on this system, so JavaScript-level syntax checks within `github-script` blocks were skipped. actionlint's security-specific checks (expression injection detection) passed cleanly for the checked patterns.

---

### 3. pre-commit — N/A

No `.pre-commit-config.yaml` exists. The project uses **lefthook** as its git hook manager.

`lefthook.yml` configures `actionlint` to run on `.github/workflows/*.{yaml,yml}` files as a pre-commit hook (see above).

---

### 4. Secret Scanning — ✅ PASS

Only one secret reference found:

```
213:          token: ${{ secrets.GITHUB_TOKEN }}
```

`secrets.GITHUB_TOKEN` is the built-in ephemeral GitHub Actions token, scoped to the workflow run. It is not logged, echoed, or stored. No custom secrets (`PAT`, `API_KEY`, etc.) are referenced. No hardcoded credentials detected.

---

## Security Analysis: Script Injection in `github-script` Blocks

GitHub's own security guidance ([Keeping your GitHub Actions and workflows secure](https://docs.github.com/en/actions/security-guides/security-hardening-for-github-actions#understanding-the-risk-of-script-injections)) classifies inline `${{ }}` expressions in JavaScript scripts as an injection risk. The safe pattern is:

```yaml
# SAFE
env:
  USER_INPUT: ${{ inputs.some_value }}
with:
  script: |
    const value = process.env.USER_INPUT;

# UNSAFE
with:
  script: |
    const value = '${{ inputs.some_value }}';  # ← injection risk
```

---

### Fixed Expressions (Correctly Remediated by This PR) ✅

These three expressions were correctly moved to `env:` blocks:

| Expression | Location | Status |
|-----------|----------|--------|
| `${{ inputs.reason }}` | Create Promotion PR step (line 302) | ✅ **FIXED** — `env: TRIGGER_REASON:` → `process.env.TRIGGER_REASON` |
| `${{ needs.check-nightly-health.outputs.failure_reason }}` | Create Failure Issue step (line 492) | ✅ **FIXED** — `env: FAILURE_REASON:` → `process.env.FAILURE_REASON` |
| `${{ needs.check-nightly-health.outputs.latest_run_url }}` | Create Failure Issue step (line 493) | ✅ **FIXED** — `env: LATEST_RUN_URL:` → `process.env.LATEST_RUN_URL` |

**These fixes follow the correct pattern and are properly implemented.**

---

### Remaining Injection Risks (Not Addressed by This PR)

#### Finding 1 — MEDIUM: `inputs.skip_workflow_check` inline in JS (Line 53)

**This is the most significant gap in the fix.** The user-controlled `skip_workflow_check` input was missed entirely.

```yaml
# Line 53 — CHECK NIGHTLY WORKFLOW STATUS step
const skipCheck = '${{ inputs.skip_workflow_check }}' === 'true';
```

**Context:**
- Input defined at the workflow level as `type: boolean`, default `false`, user-supplied via `workflow_dispatch`
- GitHub Actions enforces boolean schema — values should only be `true` or `false`
- Despite schema enforcement, embedding user-controlled inputs inline in JS strings is explicitly the anti-pattern that issue #1022 was opened to fix
- This expression was present in the same PR being audited yet was not included in the fix

**Risk:** While `type: boolean` input constraints reduce the practical injection window, the pattern is fragile. If the input type were changed to `string`, or if there is a GitHub Actions schema validation bypass, an attacker with `workflow_dispatch` trigger access could inject arbitrary JavaScript.

**Recommended Fix:**
```yaml
- name: Check Nightly Workflow Status
  id: check
  uses: actions/github-script@3a2844b7e9c422d3c10d287c895573f7108da1b3 # v9
  env:
    SKIP_WORKFLOW_CHECK: ${{ inputs.skip_workflow_check }}  # ← add this
  with:
    script: |
      const skipCheck = process.env.SKIP_WORKFLOW_CHECK === 'true';  # ← use env
```

---

#### Finding 2 — LOW: `steps.commits.outputs.*` inline in JS (Lines 307–309)

```yaml
# Lines 307-309 — Create Promotion PR step
const date = '${{ steps.commits.outputs.date }}';
const commitCount = '${{ steps.commits.outputs.commit_count }}';
const filesChanged = '${{ steps.commits.outputs.files_changed }}';
```

**Context:**
- `date` originates from `DATE=$(date -u +%Y-%m-%d)` — system command, format `YYYY-MM-DD`. Safe.
- `commit_count` originates from `git rev-list --count` — integer. Safe.
- `files_changed` originates from `git diff --stat ... | tail -1` — summary line, e.g., `"12 files changed, 345 insertions(+), 67 deletions(-)"`. Constrained format; no user-controlled content.

**Risk:** These values are NOT user-controlled. The practical injection risk is negligible. However, the pattern is non-standard and inconsistent with the fix applied to `inputs.reason` in the same step. The `filesChanged` value contains `(+)` and `(-)` characters which are safe in JS string literals.

**Recommendation (defense-in-depth):** Move to `env:` block for consistency with the corrected expressions in the same step.

---

#### Finding 3 — LOW: `steps.existing-pr.outputs.pr_url` inline in JS (Line 418)

```yaml
# Line 418 — Update Existing PR step
core.setOutput('pr_url', '${{ steps.existing-pr.outputs.pr_url }}');
```

**Context:** URL returned from the GitHub REST API (format: `https://github.com/owner/repo/pull/N`). GitHub controls the format. No user-supplied data involved.

**Risk:** Low. GitHub PR URLs are well-constrained. No practical injection vector under current conditions.

**Recommendation:** Move to `env:` block for consistency.

---

#### Finding 4 — LOW: `steps.existing-pr.outputs.pr_number` unquoted in JS (Line 407)

```yaml
# Line 407 — Update Existing PR step
const prNumber = ${{ steps.existing-pr.outputs.pr_number }};
```

**Context:** An integer PR number from the GitHub API — NOT wrapped in quotes. If the step output is unset (e.g., the preceding step was skipped), this evaluates to a JS syntax error.

**Risk:** Not a script injection risk. This is a **robustness** concern — if `steps.existing-pr.outputs.pr_number` is empty, the generated JavaScript would be `const prNumber = ;`, which is a syntax error that would cause the step to fail unexpectedly.

**Recommendation:** Wrap in `parseInt` from an env var: `const prNumber = parseInt(process.env.PR_NUMBER, 10);`

---

#### Finding 5 — LOW: Job result outputs inline in JS (Lines 494, 497)

```yaml
# Lines 494, 497 — Create Failure Issue step
const isHealthy = '${{ needs.check-nightly-health.outputs.is_healthy }}';
const prResult = '${{ needs.create-promotion-pr.result }}';
```

**Context:**
- `is_healthy`: step output from `check-nightly-health` job; value is `'true'` or `'false'`
- `prResult`: GitHub Actions job result context; constrained to `'success'` | `'failure'` | `'skipped'` | `'cancelled'`

**Risk:** Both are GitHub-controlled values with a fixed, constrained value set. No user input involved. Practical risk is negligible.

**Note:** `FAILURE_REASON` and `LATEST_RUN_URL` from the same step were correctly moved to `env:` blocks by this fix. Moving `isHealthy` and `prResult` would complete the pattern.

**Recommendation (defense-in-depth):** Move to `env:` block for consistency.

---

#### Acceptable Patterns (Safe as-is) ✅

| Expression | Location | Rationale |
|-----------|----------|-----------|
| `${{ env.TARGET_BRANCH }}` | Lines 285, 353 | Workflow-level `env:` var → `'main'`. Not user-controlled. |
| `${{ env.SOURCE_BRANCH }}` | Line 352 | Workflow-level `env:` var → `'nightly'`. Not user-controlled. |
| `${{ secrets.GITHUB_TOKEN }}` | Line 213 | Built-in ephemeral token, not injected into JS string. |
| Shell `${{ }}` expressions | Lines 608–612 | Shell variable assignments in `run:` blocks — different injection model (shell, not JS). Shell context uses `"$VAR"` quoting pattern which is acceptable here. |

---

## Summary of Findings

| # | Severity | Line | Expression | Status |
|---|----------|------|-----------|--------|
| 1 | 🟡 MEDIUM | 53 | `inputs.skip_workflow_check` inline in JS string | ❌ Not fixed — **critical gap** |
| 2 | 🟢 LOW | 307–309 | `steps.commits.outputs.date/commit_count/files_changed` inline | ⚠️ Not fixed — low practical risk |
| 3 | 🟢 LOW | 418 | `steps.existing-pr.outputs.pr_url` inline in JS string | ⚠️ Not fixed — low practical risk |
| 4 | 🟢 LOW | 407 | `steps.existing-pr.outputs.pr_number` unquoted in JS | ⚠️ Robustness concern, not injection |
| 5 | 🟢 LOW | 494, 497 | `is_healthy`, `prResult` inline in JS string | ⚠️ Not fixed — GitHub-controlled values |
| — | ✅ FIXED | 302 | `inputs.reason` → `env: TRIGGER_REASON` | ✅ Correctly fixed |
| — | ✅ FIXED | 492 | `outputs.failure_reason` → `env: FAILURE_REASON` | ✅ Correctly fixed |
| — | ✅ FIXED | 493 | `outputs.latest_run_url` → `env: LATEST_RUN_URL` | ✅ Correctly fixed |

---

## Verdict

**⚠️ CONDITIONAL FAIL**

The fix for issue #1022 correctly addresses three of the most impactful injection patterns using the `env:` block pattern. However, the fix is **incomplete**:

1. **`inputs.skip_workflow_check` (line 53) was missed** — This is the only remaining user-controlled `inputs.*` expression in the entire workflow and should have been included in the same fix. While the `type: boolean` constraint limits practical exploitability, this pattern is inconsistent with the stated goal of the fix and should be corrected before merge.

2. **Five additional inline expressions remain** at LOW severity — these involve GitHub-controlled or system-generated values and represent defense-in-depth improvements rather than blocking security issues.

**Recommended action before merging:** Add `SKIP_WORKFLOW_CHECK: ${{ inputs.skip_workflow_check }}` to the `env:` block of the `Check Nightly Workflow Status` step (line ~52) and replace the inline expression on line 53 with `process.env.SKIP_WORKFLOW_CHECK === 'true'`.

The additional LOW-severity findings (lines 307–309, 407, 418, 494, 497) should be addressed as a follow-up for defense-in-depth and pattern consistency, but do not block merge on their own.
