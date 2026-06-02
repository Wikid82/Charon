# Fix: Stale Grype Code Scanning Results

**Status:** Draft
**Date:** 2026-06-02
**Target Files:** `.grype.yaml`, `.trivyignore`, `.github/workflows/supply-chain-pr.yml`

---

## 1. Introduction

GitHub Code Scanning reports Grype last scanned the repository on **February 4, 2026** —
approximately four months stale. The staleness prevents the security team from identifying
newly disclosed vulnerabilities and erodes confidence in the supply-chain verification pipeline.

The root cause is not a single bug but a chain of compounding failures:

1. Suppression entries in `.trivyignore` began expiring on April 30, 2026.
2. Expired entries caused Trivy PR scans to surface HIGH/CRITICAL findings.
3. The Trivy security gate blocks PR merges → no PRs merged to `main`.
4. Without PR merges, the `supply-chain-pr.yml` push trigger cannot fire.
5. Even if a push *did* land on `main`, the job-level `if:` condition contains no `push` branch,
   so the job would be skipped silently.
6. There is no `schedule:` trigger to run scans periodically regardless of push activity.
7. Separately, `.grype.yaml` suppressions also expired in May, so if a Grype scan *did* run,
   it would detect HIGH findings and fail — preventing a fresh SARIF upload.
8. A `continue-on-error: true` flag on the SARIF upload step silently swallows any upload errors.

**Scope:** This plan covers only the supply-chain scanning pipeline. It does **not** propose
upgrading Grype (v0.112.0 is the current latest, released 2026-05-01) or updating the
`codeql-action` SHA (`7211b7c8077ea37d8641b6271f6a365a22a5fbfa` = v4.36.0, released 2026-05-22).

---

## 2. Research Findings

### 2.1 Files Audited

| File | Lines | Key Finding |
|------|-------|-------------|
| `.grype.yaml` | 641 | 10 of 12 suppression entries expired in May 2026 |
| `.trivyignore` | ~120 | 12 of 14 suppression entries expired between April 30 – May 25, 2026 |
| `.github/workflows/supply-chain-pr.yml` | 490 | Missing `schedule:` trigger; job `if:` drops push events; `continue-on-error: true` on SARIF upload |
| `.github/workflows/docker-build.yml` | 1073 | Trivy scans operate under a separate SARIF category; Grype is not scanned here; no impact on root cause |

### 2.2 Root Cause #1 — Expired `.trivyignore` Suppressions (HIGHEST IMPACT)

**Severity:** Critical — blocks PR merges, the primary gateway to Grype scans running.

Trivy's `exp: YYYY-MM-DD` DSL syntax causes entries to silently stop matching after the expiry
date. With those entries inactive, the CVEs they covered resurface as HIGH/CRITICAL findings. The
`Enforce PR Trivy security gate` step in `docker-build.yml` exits 1 when any HIGH/CRITICAL blocker
is detected, blocking all PR merges.

**Expired entries** (as of 2026-06-02):

| Entry | Package / Context | Expiry | Status |
|-------|-------------------|--------|--------|
| `CVE-2026-25793` | nebula ECDSA sig. malleability; waiting on `smallstep/certificates` | 2026-05-10 | **EXPIRED** |
| `CVE-2026-27171` | zlib 1.3.1-r2 CPU spin; no Alpine fix (MEDIUM — non-blocking by CI policy) | 2026-05-21 | **EXPIRED** |
| `CVE-2026-2673` | libcrypto3/libssl3 3.5.5-r0; no Alpine 3.23 patch | 2026-05-18 | **EXPIRED** |
| `CVE-2026-33186` | gRPC-Go auth bypass in CrowdSec/Caddy embedded binaries; waiting on upstream | 2026-05-04 | **EXPIRED** |
| `GHSA-479m-364c-43vc` | goxmldsig XML sig bypass in Caddy; waiting on caddy-security plugin | 2026-05-04 | **EXPIRED** |
| `GHSA-6g7g-w4f8-9c9x` | buger/jsonparser CrowdSec embedded; no upstream fix | 2026-05-19 | **EXPIRED** |
| `GHSA-jqcq-xjh3-6g23` | pgproto3/v2 panic in CrowdSec; v2 archived, no fix | 2026-05-19 | **EXPIRED** |
| `GHSA-x6gf-mpr2-68h6` | pgproto3/v2 (NVD alias CVE-2026-4427); same as above | 2026-05-21 | **EXPIRED** |
| `CVE-2026-33997` | docker/docker Moby off-by-one; no `docker/docker` import-path fix | 2026-04-30 | **EXPIRED** |
| `GHSA-pxq6-2prw-chj9` | Moby GHSA alias for CVE-2026-33997 | 2026-04-30 | **EXPIRED** |
| `CVE-2026-41889` | pgx/v4 panic in CrowdSec; requires migration to pgx/v5 | 2026-05-25 | **EXPIRED** |
| `GHSA-j88v-2chj-qfwx` | pgx/v4 GHSA alias for CVE-2026-41889 | 2026-05-25 | **EXPIRED** |
| `CVE-2026-32286` | pgproto3/v2 buffer overflow; archived repo, no fix | 2026-07-09 | ✅ VALID |

### 2.3 Root Cause #2 — Expired `.grype.yaml` Suppressions (HIGH IMPACT)

**Severity:** High — when Grype *does* run, the expired entries allow HIGH/CRITICAL findings to
surface. The `Fail on Critical/High vulnerabilities` step exits 1, preventing a successful run.
Because this step runs *after* the SARIF upload, a SARIF may occasionally land in Code Scanning
even when the workflow fails — but the staleness is a signal that no successful scans are occurring.

**Expired entries** (as of 2026-06-02):

| Entry | Package | Expiry | Status |
|-------|---------|--------|--------|
| `CVE-2026-2673` | libcrypto3 3.5.5-r0 | 2026-05-18 | **EXPIRED** |
| `CVE-2026-2673` | libssl3 3.5.5-r0 | 2026-05-18 | **EXPIRED** |
| `CVE-2026-31790` | libcrypto3 3.5.5-r0 | 2026-05-09 | **EXPIRED** |
| `CVE-2026-31790` | libssl3 3.5.5-r0 | 2026-05-09 | **EXPIRED** |
| `CVE-2026-25793` | nebula in Caddy binary | 2026-05-10 | **EXPIRED** |
| `GHSA-6g7g-w4f8-9c9x` | buger/jsonparser in CrowdSec | 2026-05-19 | **EXPIRED** |
| `GHSA-jqcq-xjh3-6g23` | pgproto3/v2 in CrowdSec | 2026-05-19 | **EXPIRED** |
| `GHSA-x6gf-mpr2-68h6` | pgproto3/v2 in CrowdSec (alias) | 2026-05-21 | **EXPIRED** |
| `GHSA-pxq6-2prw-chj9` | docker/docker Moby | 2026-04-30 | **EXPIRED** |
| `GHSA-78h2-9frx-2jm8` | go-jose/v3 in Caddy (libcrypto3 entry) | 2026-05-05 | **EXPIRED** |
| `GHSA-78h2-9frx-2jm8` | go-jose/v3 in Caddy (libssl3 entry) | 2026-05-05 | **EXPIRED** |
| `CVE-2026-32286` | pgproto3/v2 buffer overflow | 2026-07-09 | ✅ VALID |

### 2.4 Root Cause #3 — Job `if:` Condition Missing `push` Event Branch

**Severity:** High — the `push: branches: [main]` trigger in `on:` fires the workflow, but the
job-level `if:` condition evaluates to `false` for push events, causing the job to be silently
skipped.

**Current `if:` condition (supply-chain-pr.yml, lines 35–42):**

```yaml
if: >
  github.event_name == 'workflow_dispatch' ||
  github.event_name == 'pull_request' ||
  (github.event_name == 'workflow_run' &&
   (github.event.workflow_run.event == 'push' || github.event.workflow_run.pull_requests[0].number != null) &&
   (github.event.workflow_run.status != 'completed' || github.event.workflow_run.conclusion == 'success'))
```

When `github.event_name == 'push'`:
- `workflow_dispatch` → `false`
- `pull_request` → `false`
- `workflow_run && ...` → `false` (event_name is not `workflow_run`)
- **Result: job is skipped. SARIF is never uploaded on push events.**

Note: `workflow_run` is **not** in the `on:` trigger for this workflow. The third branch of the
condition is permanently unreachable dead code.

### 2.5 Root Cause #4 — Missing `schedule:` Trigger

**Severity:** Medium — without a periodic trigger, Code Scanning goes stale any time there is a
lull in PR/push activity to `main`. GitHub Code Scanning recommends scanning at least weekly for
supply-chain workflows.

### 2.6 Root Cause #5 — `continue-on-error: true` on SARIF Upload Step

**Severity:** Medium — any SARIF upload failure (e.g., GitHub API error, permission issue, rate
limit) is silently swallowed. The workflow continues and shows a green check even though no scan
data was persisted in Code Scanning.

### 2.7 Root Cause #6 — Missing `development` Branch in Push Trigger

**Severity:** Low-Medium — `docker-build.yml` targets `[main, development]` for push triggers,
indicating `development` is the primary active branch. `supply-chain-pr.yml` only targets `main`,
creating a blind spot for all supply-chain work on `development`.

### 2.8 Root Cause #7 — Dead `workflow_run` Branch in Job `if:` Condition

**Severity:** Low (cosmetic) — the `workflow_run &&` branch cannot be reached because
`workflow_run` is not in the `on:` block. Removing it eliminates confusion and is addressed as
part of Fix 3 below.

### 2.9 Confirmed Non-Issues

| Item | Finding |
|------|---------|
| Grype version `v0.112.0` | Current latest release, published 2026-05-01 — **no action** |
| `codeql-action` SHA `7211b7c8077ea37d8641b6271f6a365a22a5fbfa` | Equals v4.36.0, published 2026-05-22 — **no action** |
| `docker-build.yml` Trivy scanning | Uses separate SARIF category; no interaction with Grype SARIF |

---

## 3. Technical Specifications

### Fix 1 — Extend `.trivyignore` Suppressions

**File:** `.trivyignore`

Before extending each entry, verify the upstream status:
- Alpine tracker: `https://security.alpinelinux.org/vuln/<CVE>`
- GHSA advisory: `https://github.com/advisories/<GHSA>`
- NVD: `https://nvd.nist.gov/vuln/detail/<CVE>`

If an upstream fix is confirmed and the Docker image can be rebuilt to include it, **remove the
entry** and verify the CVE no longer appears in scan output. If no fix is available, extend
the `exp:` date and update the `# Review by:` comment.

**Proposed new expiry dates** (assuming no upstream fix at time of implementation):

| Entry | Proposed New Expiry | Rationale |
|-------|---------------------|-----------|
| `CVE-2026-25793` | `2026-08-01` | 60-day ext; awaiting `smallstep/certificates` update |
| `CVE-2026-27171` | `2026-08-01` | 60-day ext; Alpine zlib still 1.3.1-r2 |
| `CVE-2026-2673` | `2026-08-01` | 60-day ext; Alpine 3.23 still ships 3.5.5-r0 |
| `CVE-2026-33186` | `2026-08-01` | 60-day ext; awaiting CrowdSec/Caddy update |
| `GHSA-479m-364c-43vc` | `2026-08-01` | 60-day ext; awaiting caddy-security plugin update |
| `GHSA-6g7g-w4f8-9c9x` | `2026-08-01` | 60-day ext; no upstream fix for buger/jsonparser |
| `GHSA-jqcq-xjh3-6g23` | `2026-09-01` | 90-day ext; pgproto3/v2 archived; fix requires CrowdSec migration to pgx/v5 |
| `GHSA-x6gf-mpr2-68h6` | `2026-09-01` | 90-day ext; same tracking as GHSA-jqcq-xjh3-6g23 |
| `CVE-2026-33997` | `2026-08-01` | 60-day ext; no fix for docker/docker import path |
| `GHSA-pxq6-2prw-chj9` | `2026-08-01` | 60-day ext; same tracking as CVE-2026-33997 |
| `CVE-2026-41889` | `2026-09-01` | 90-day ext; CrowdSec pgx/v4 → v5 migration required |
| `GHSA-j88v-2chj-qfwx` | `2026-09-01` | 90-day ext; same tracking as CVE-2026-41889 |
| `CVE-2026-32286` | _no change_ | VALID until 2026-07-09 |

Each updated entry must include an updated comment:
```
exp: YYYY-MM-DD
# Extended 2026-06-02: <reason>. Tracker: <URL>. Next review: YYYY-MM-DD.
```

### Fix 2 — Extend `.grype.yaml` Suppressions

**File:** `.grype.yaml`

Same upstream verification as Fix 1 before extending. Each entry uses the `expiry:` YAML key.
Update both the `expiry:` value and the `# Review:` history block above each stanza.

**Proposed new expiry dates** (assuming no upstream fix at time of implementation):

| Entry | Package | Proposed New Expiry |
|-------|---------|---------------------|
| `CVE-2026-2673` | libcrypto3 3.5.5-r0 | `2026-08-01` |
| `CVE-2026-2673` | libssl3 3.5.5-r0 | `2026-08-01` |
| `CVE-2026-31790` | libcrypto3 3.5.5-r0 | `2026-08-01` |
| `CVE-2026-31790` | libssl3 3.5.5-r0 | `2026-08-01` |
| `CVE-2026-25793` | nebula in Caddy binary | `2026-08-01` |
| `GHSA-6g7g-w4f8-9c9x` | buger/jsonparser in CrowdSec | `2026-08-01` |
| `GHSA-jqcq-xjh3-6g23` | pgproto3/v2 in CrowdSec | `2026-09-01` |
| `GHSA-x6gf-mpr2-68h6` | pgproto3/v2 alias | `2026-09-01` |
| `GHSA-pxq6-2prw-chj9` | docker/docker | `2026-08-01` |
| `GHSA-78h2-9frx-2jm8` | go-jose/v3 Caddy (libcrypto3) | `2026-08-01` |
| `GHSA-78h2-9frx-2jm8` | go-jose/v3 Caddy (libssl3) | `2026-08-01` |
| `CVE-2026-32286` | pgproto3/v2 | _no change_ (valid until 2026-07-09) |

**Patch format** for each updated `expiry:` line:
```yaml
expiry: "2026-08-01"
# Review: Extended 2026-06-02 — <scanner/tracker> confirms no upstream fix. Next review: 2026-08-01.
```

### Fix 3 — Repair Job `if:` Condition in `supply-chain-pr.yml`

**File:** `.github/workflows/supply-chain-pr.yml`, lines 35–42

Remove the permanently unreachable `workflow_run` branch and add the missing `push` event branch.

**Current:**
```yaml
    if: >
      github.event_name == 'workflow_dispatch' ||
      github.event_name == 'pull_request' ||
      (github.event_name == 'workflow_run' &&
       (github.event.workflow_run.event == 'push' || github.event.workflow_run.pull_requests[0].number != null) &&
       (github.event.workflow_run.status != 'completed' || github.event.workflow_run.conclusion == 'success'))
```

**Replacement:**
```yaml
    if: >
      github.event_name == 'workflow_dispatch' ||
      github.event_name == 'pull_request' ||
      github.event_name == 'push' ||
      github.event_name == 'schedule'
```

### Fix 4 — Add `schedule:` Trigger and `development` Branch to `supply-chain-pr.yml`

**File:** `.github/workflows/supply-chain-pr.yml`, `on:` block

**Current `on:` block:**
```yaml
on:
  workflow_dispatch:
    inputs:
      pr_number:
        description: "PR number to verify (optional, will auto-detect from workflow_run)"
        required: false
        type: string
  pull_request:
  push:
    branches:
      - main
```

**Replacement:**
```yaml
on:
  schedule:
    - cron: '0 2 * * 1'  # Every Monday at 02:00 UTC
  workflow_dispatch:
    inputs:
      pr_number:
        description: "PR number to verify (optional, will auto-detect from workflow_run)"
        required: false
        type: string
  pull_request:
  push:
    branches:
      - main
      - development
```

**Rationale for `development`:** `docker-build.yml` already targets `[main, development]`.
Active supply-chain work happens on `development`; scanning should cover the same scope.

### Fix 5 — Remove `continue-on-error: true` from SARIF Upload Step

**File:** `.github/workflows/supply-chain-pr.yml`

Locate the `upload-sarif` step (search for `category: supply-chain-pr`) and remove the
`continue-on-error: true` line.

**Before:**
```yaml
      - name: Upload SARIF to GitHub Security tab
        uses: github/codeql-action/upload-sarif@7211b7c8077ea37d8641b6271f6a365a22a5fbfa # v4.36.0
        with:
          sarif_file: grype-results.sarif
          category: supply-chain-pr
          token: ${{ secrets.GITHUB_TOKEN }}
        continue-on-error: true
```

**After:**
```yaml
      - name: Upload SARIF to GitHub Security tab
        uses: github/codeql-action/upload-sarif@7211b7c8077ea37d8641b6271f6a365a22a5fbfa # v4.36.0
        with:
          sarif_file: grype-results.sarif
          category: supply-chain-pr
          token: ${{ secrets.GITHUB_TOKEN }}
```

---

## 4. Implementation Plan

### Phase 0 — Upstream Fix Verification (Pre-implementation, ~30 min)

Before editing any suppression file, verify whether upstream fixes have shipped for any of the
expired entries. For each CVE/GHSA:

1. Check Alpine security tracker for libcrypto3/libssl3 and zlib.
2. Check GHSA advisories for pgproto3/v2 (GHSA-jqcq-xjh3-6g23, GHSA-x6gf-mpr2-68h6, CVE-2026-32286) and pgx/v4 (CVE-2026-41889, GHSA-j88v-2chj-qfwx).
3. Check if Caddy has released a version with patched goxmldsig and go-jose/v3.
4. Check if `smallstep/certificates` has released a version compatible with nebula v1.10+.
5. For any entry where a fix is confirmed: **remove** the entry; note the CVE in the commit message.
6. For entries where no fix is confirmed: **extend** the expiry using the proposed dates in Fix 1/Fix 2.

### Phase 1 — Extend Suppression Files

**Goal:** Allow PRs to merge again and unblock the entire pipeline.

**Tasks:**
- [ ] Complete Phase 0 verification.
- [ ] Update `.trivyignore`: extend all 12 expired entries (or remove if upstream fix available). Update `# exp:`, `# Extended`, and `# Review by:` comments on each entry.
- [ ] Update `.grype.yaml`: extend all 10 expired entries (or remove if upstream fix available). Update `expiry:` YAML value and `# Review:` history block on each entry.
- [ ] Open a draft PR with these changes only.
- [ ] Confirm `docker-build.yml` Trivy PR scan passes (`Enforce PR Trivy security gate` exits 0).

**Validation gate:** `Enforce PR Trivy security gate` step shows green.

### Phase 2 — Fix `supply-chain-pr.yml`

**Goal:** Ensure Grype scans run on push events, on `development`, and weekly via schedule.

**Tasks:**
- [ ] Add `schedule:` with weekly cron to the `on:` block (Fix 4).
- [ ] Add `development` to `push: branches:` (Fix 4).
- [ ] Replace the job `if:` condition with the simplified three-branch form (Fix 3).
- [ ] Remove `continue-on-error: true` from the `upload-sarif` step (Fix 5).

**Validation gate:**
1. Trigger `workflow_dispatch` manually.
2. Confirm `verify-supply-chain` job is not skipped.
3. Confirm `Upload SARIF to GitHub Security tab` step exits 0.
4. Confirm GitHub Code Scanning shows updated Grype scan date.

### Phase 3 — End-to-End Validation

**Tasks:**
- [ ] Merge the PR from Phase 1 + Phase 2.
- [ ] Push a commit to `main` and verify `verify-supply-chain` job runs (not skipped).
- [ ] Open GitHub Security tab → Code Scanning and confirm Grype `Last scanned` date is today.
- [ ] Push a commit to `development` and verify the job also runs.
- [ ] Confirm no Grype HIGH/CRITICAL findings surface (all suppressions current).
- [ ] Optionally: temporarily change cron to `*/5 * * * *`, confirm schedule fires, then revert.

---

## 5. Acceptance Criteria

| # | Criterion | How to Verify |
|---|-----------|---------------|
| AC-1 | GitHub Code Scanning shows Grype scan date updated (not Feb 4, 2026) | Security tab → Code Scanning → Grype tool |
| AC-2 | No expired entries remain in `.trivyignore` | `grep "exp: 202[56]-" .trivyignore` returns no past dates |
| AC-3 | No expired entries remain in `.grype.yaml` | Inspect all `expiry:` values; none before 2026-06-02 |
| AC-4 | `supply-chain-pr.yml` `on:` block includes `schedule:` and `development` | `grep -A5 "schedule:" .github/workflows/supply-chain-pr.yml` |
| AC-5 | Job `if:` condition handles `push` events; no `workflow_run` dead code | Inspect lines 35–42; exactly three `event_name` branches |
| AC-6 | SARIF upload step has no `continue-on-error: true` | `grep -n "continue-on-error" .github/workflows/supply-chain-pr.yml` returns no SARIF-related hits |
| AC-7 | `Enforce PR Trivy security gate` passes on a new PR | PR check status green |
| AC-8 | `verify-supply-chain` job is not skipped on push to `main` | Push a commit and observe job status (must show "success", not "skipped") |
| AC-9 | No suppression entries removed for CVEs that still have no upstream fix | Cross-reference removal decisions against tracker URLs in commit message |

---

## 6. Commit Slicing Strategy

**Decision:** Single PR with three ordered logical commits.

One PR keeps the change set reviewable as a cohesive supply-chain fix. Three commits separate by
file type so each commit is independently reviewable and individually revertable if needed.

### Commit 1 — `fix(security): extend expired suppression entries in .grype.yaml and .trivyignore`

**Scope:** Suppression file changes only.

**Files:**
- `.grype.yaml` — update `expiry:` for 10 entries; remove any with confirmed upstream fixes
- `.trivyignore` — update `exp:` for 12 entries; remove any with confirmed upstream fixes

**Dependencies:** Phase 0 upstream verification must be complete before authoring this commit.

**Validation gate:** Open draft PR with this commit only. Confirm `Enforce PR Trivy security gate`
exits 0 before adding Commit 2.

**Rollback note:** Revert this commit only if an entry is found to have incorrectly suppressed a
now-patched CVE. Re-verify upstream trackers and remove the offending entry instead of extending.

---

### Commit 2 — `fix(ci): add schedule trigger and development branch to supply-chain-pr.yml`

**Scope:** `supply-chain-pr.yml` trigger changes only.

**Files:**
- `.github/workflows/supply-chain-pr.yml` — add `schedule:` cron, add `development` to push branches

**Dependencies:** Commit 1 in the same PR and passing CI.

**Validation gate:** `workflow_dispatch` manual run completes; `verify-supply-chain` job runs.

---

### Commit 3 — `fix(ci): repair push-event job condition and remove silent error suppression`

**Scope:** `supply-chain-pr.yml` job condition, `continue-on-error` removal, and concurrency group cleanup.

**Files:**
- `.github/workflows/supply-chain-pr.yml` — replace job `if:` condition (add `push` + `schedule`); remove `continue-on-error: true` from SARIF upload step; simplify `concurrency.group` to remove dead `workflow_run` expressions

**Current concurrency group (dead `workflow_run` references):**
```yaml
group: supply-chain-pr-${{ github.event.workflow_run.event || github.event_name }}-${{ github.event.workflow_run.head_branch || github.ref }}
```

**Replacement:**
```yaml
group: supply-chain-pr-${{ github.event_name }}-${{ github.ref }}
```

**Dependencies:** Commit 2 merged (schedule active before this becomes load-bearing).

**Validation gate:**
1. Push a commit to `main` and confirm `verify-supply-chain` job is **not** skipped.
2. Check GitHub Code Scanning shows an updated Grype scan date after the push.

---

### Contingency Notes

- If extending suppressions causes unexpected Grype output, run Grype locally to verify:
  `grype -c .grype.yaml --add-cpes-if-none -o sarif <image> > grype-results.sarif`
- If the SARIF upload step fails after removing `continue-on-error`, confirm the workflow has
  `security-events: write` in its `permissions:` block (confirmed present during research).
- If the Monday schedule fires but the job is still skipped, verify Commit 3 was applied
  (the job `if:` fix is required for `schedule` events, which also use `github.event_name == 'schedule'`,
  not `push` or `pull_request`).

  > **Important:** The simplified job condition `github.event_name == 'push'` does NOT cover
  > `schedule` events. The final condition must include all four event names:
  >
  > ```yaml
  > if: >
  >   github.event_name == 'workflow_dispatch' ||
  >   github.event_name == 'pull_request' ||
  >   github.event_name == 'push' ||
  >   github.event_name == 'schedule'
  > ```
