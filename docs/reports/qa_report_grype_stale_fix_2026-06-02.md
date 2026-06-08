# QA Report — Stale Grype Scanning Fix

**Status: QA PASS**
**Date: 2026-06-02**
**Scope: `.trivyignore`, `.grype.yaml`, `.github/workflows/supply-chain-pr.yml`**

---

## Summary

All validation checks passed. No critical or high issues found. One low-severity
code-quality finding documented below.

---

## 1. `.trivyignore` Integrity

| Check | Result |
|---|---|
| All `exp:` dates ≥ 2026-06-02 | ✅ PASS |
| CVE-2026-32286 untouched at `2026-07-09` | ✅ PASS |
| No entry exceeds 90-day limit (2026-09-01) | ✅ PASS |
| Entry count (12 CVE/GHSA entries) | ✅ PASS |

**Expiry distribution:**
- `2026-07-09`: 1 entry (CVE-2026-32286 — unmodified, pre-existing date)
- `2026-08-01`: 8 entries (extended 2026-06-02)
- `2026-09-01`: 3 entries (extended 2026-06-02, EOL packages requiring upstream
  migration)

All dates are within the 90-day maximum window (≤ 2026-09-01).

---

## 2. `.grype.yaml` Integrity

| Check | Result |
|---|---|
| All `expiry:` dates ≥ 2026-06-02 | ✅ PASS |
| CVE-2026-32286 untouched at `2026-07-09` | ✅ PASS |
| No entry exceeds 90-day limit (2026-09-01) | ✅ PASS |
| YAML syntax valid (`yaml.safe_load`) | ✅ PASS |
| Suppression entry count (12 package-level entries) | ✅ PASS |

**Expiry distribution:**
- `2026-07-09`: 1 entry (CVE-2026-32286 — unmodified; comment reads "Reviewed
  2026-04-10", NOT "Extended 2026-06-02", confirming the entry was not touched)
- `2026-08-01`: 9 entries (extended 2026-06-02)
- `2026-09-01`: 2 entries (extended 2026-06-02, EOL pgproto3/v2 packages)

CVE-2026-32286 expiry line: `expiry: "2026-07-09"  # Reviewed 2026-04-10: no fix
path until CrowdSec migrates to pgx/v5. 90-day expiry.` — review comment is
2026-04-10, not 2026-06-02, confirming the entry was not modified.

---

## 3. `supply-chain-pr.yml` Integrity

| Check | Result |
|---|---|
| `schedule:` cron trigger present (`0 2 * * 1`) | ✅ PASS |
| Push trigger includes `development` branch | ✅ PASS |
| Job `if:` has exactly 4 `event_name ==` conditions | ✅ PASS |
| Exactly 1 `continue-on-error: true` remains | ✅ PASS |
| `workflow_run:` removed from `on:` trigger block | ✅ PASS |
| `security-events: write` permission at workflow level | ✅ PASS |
| SARIF upload step has no `continue-on-error` | ✅ PASS |
| YAML syntax valid (`yaml.safe_load`) | ✅ PASS |
| `actionlint` clean (0 issues) | ✅ PASS |

**Verified values:**

```
# Schedule
schedule:
  - cron: '0 2 * * 1'  # Every Monday at 02:00 UTC

# Push branches
push:
  branches:
    - main
    - development

# Job if-condition (4 event_name checks)
if: >
  github.event_name == 'workflow_dispatch' ||
  github.event_name == 'pull_request' ||
  github.event_name == 'push' ||
  github.event_name == 'schedule'

# continue-on-error (1 occurrence — "Comment on PR" step, line 386)
# SARIF upload step (line 366): no continue-on-error present

# Permissions
security-events: write  ✅
```

**Concurrency group** uses
`supply-chain-pr-${{ github.event_name }}-${{ github.ref }}`, which correctly
prevents `schedule` runs from cancelling in-flight PR runs.

---

## 4. Pre-commit Hooks

| Check | Result |
|---|---|
| Project hook framework identified (lefthook) | ✅ INFO |
| `check-yaml` hook (`yaml.safe_load`) on `.grype.yaml` | ✅ PASS |
| `check-yaml` hook (`yaml.safe_load`) on `supply-chain-pr.yml` | ✅ PASS |
| `actionlint` on `supply-chain-pr.yml` | ✅ PASS |

Note: The project uses lefthook (not `.pre-commit-config.yaml`). The `check-yaml`
hook validates YAML syntax via Python's `yaml.safe_load`. Both files pass.
Yamllint line-length warnings visible via direct invocation are pre-existing
across the entire files and are not regressions introduced by these changes.

---

## 5. Security Assessment

| Check | Result |
|---|---|
| No suppression extended beyond 2026-09-01 (90-day max) | ✅ PASS |
| `security-events: write` permission confirmed | ✅ PASS |
| SARIF upload step `continue-on-error` removed | ✅ PASS |
| No secrets or tokens exposed in changed files | ✅ PASS |
| No Gotify tokens in output, logs, or URL query strings | ✅ PASS |

The only token-adjacent string match in the changed files is a comment:
`"Cannot access PR comments (likely token permissions / fork / event context)"`.
This is a diagnostic log message — no credential is exposed.

---

## Findings

### 🟢 LOW — Dead Step Conditions (Code Quality)

**File:** `.github/workflows/supply-chain-pr.yml`

Several step-level `if:` conditions still reference
`github.event_name == 'workflow_run'`:

- Line 119: `if: github.event_name == 'workflow_run' && ...` (Check for PR image
  artifact)
- Line 195: `if: github.event_name == 'workflow_run' && ...` (Skip if no
  artifact)
- Line 202: `if: github.event_name == 'workflow_run' && ...` (Load Docker image)
- Line 221: `if: github.event_name == 'workflow_run' && ...` (Run Grype scan
  from artifact)
- Line 251: `if: github.event_name != 'workflow_run'` (Run Grype scan from local
  build)

Since `workflow_run` is no longer listed in the `on:` trigger block, the
`== 'workflow_run'` steps will always evaluate to false and skip; the
`!= 'workflow_run'` step will always evaluate to true and run. The workflow
remains functionally correct — the dead conditions do not affect security or
correctness.

**Severity:** LOW — no functional or security impact.
**Recommendation:** Optional follow-up cleanup to remove dead `workflow_run`
step conditions. Not required for merge.

---

## Conclusion

**QA PASS.** All 3 changed files pass integrity, syntax, and security checks.
The 12 `.trivyignore` extensions and 10 `.grype.yaml` suppression extensions are
within the 90-day policy window. CVE-2026-32286 is confirmed untouched at its
original `2026-07-09` expiry. The workflow correctly triggers on `schedule`,
`push`, `pull_request`, and `workflow_dispatch` events with a corrected job
if-condition, `security-events: write` permission, and SARIF upload without
`continue-on-error`.
