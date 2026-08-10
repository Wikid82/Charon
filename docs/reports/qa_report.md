# QA Report — Issue #619 Test-Infrastructure Debt Closeout

**Branch**: `test/issue-619-test-infra-debt`
**Commits reviewed this pass**: `dac267f3`..`52bdc675` (19 commits — 6 parallel dev-agent fix rounds closing out the 52 E2E failures documented in the prior pass)
**Reviewed by**: qa-security agent
**Date**: 2026-08-08
**Plan reference**: `docs/plans/current_spec.md`

## FINAL CLOSEOUT (added by Management after this report, same day)

Per the codified policy change (`75c63696`), no further local full-suite/multi-browser Playwright runs were performed. CI's next run on the PR is the authoritative full cross-browser confirmation. All other, non-Playwright Definition of Done gates were completed locally after this report (branch then gained one more commit, `716e26b2`, fixing the 2 new a11y findings in §1 below):

| Gate | Result |
|---|---|
| Backend coverage (`go-test-coverage.sh`) | ✅ 89.2% line coverage (min 87%) |
| Frontend coverage (`frontend-test-coverage.sh`) | ✅ 90.84% line coverage (min 87%); Statements 89.63%, Branches 82.8%, Functions 87.27% |
| Local patch coverage (`local-patch-report.sh`) | ✅ 100% overall (18/18 changed lines), vs. 90% minimum |
| Lefthook `pre-commit` (targeted run against all 51 files changed vs. `development`, since the `actionlint` job hung on an environment issue unrelated to this PR — zero `.github/workflows/*.yml` files are touched by this PR, confirmed via `git diff --stat`, so `actionlint` has nothing to check here regardless) | ✅ go-vet, golangci-lint-fast, dockerfile-check, frontend-type-check, frontend-lint all pass. `shellcheck` re-run directly (bypassing a `{staged_files}` templating artifact) — 0 errors. `semgrep` — 424 rules / 973 files / 0 findings. |
| Lefthook `codeql` (Go + JS) | ✅ Go: 1 result, suppressed (pre-existing, documented in `codeql-suppressions.yml`, unrelated to this PR). JS: 0 findings. Parity check passed. |
| `gitleaks` (not part of the mandatory `pre-commit` stage, run directly for defense-in-depth per the "double-check for secrets" guidance) | ✅ 514 pre-existing findings across the broader repo (test fixtures/mock credentials, none introduced by this PR) — **0 findings in any of this PR's 51 changed files**, confirmed by cross-referencing the gitleaks JSON report against the changed-file list. |
| Trivy container/dependency scan | ➡️ Carried forward, not re-run — confirmed via `git diff --stat` that this PR changes zero `go.mod`/`go.sum`/`package.json`/`package-lock.json`/`Dockerfile*` files across all 30 commits vs. `development`, so the earlier confirmed result (0 new Critical/High, 1 pre-existing tracked HIGH documented in `SECURITY.md`) cannot have drifted. |
| Backend build (`go build ./...`) | ✅ Clean |
| Frontend build (`npm run build`) | ✅ Clean, `✓ built in 2.40s` |
| GORM security scan | N/A, correctly skipped — zero `.go` files changed anywhere in this PR |

**This is now a complete Definition of Done sweep for everything except full cross-browser/full-suite Playwright confirmation, which is CI's job per policy.** Recommend: push, open/update the PR, let CI run the full 3-browser matrix, and treat any CI-reported failure as a new finding to triage rather than assuming the local partial runs already covered it.

## STATUS: Partial re-validation — stopped mid-run per updated process guidance. Not a final merge verdict.

This pass was launched as a full Definition-of-Done re-validation (clean rebuild + all 3 browsers + backend/frontend coverage + lefthook + Trivy). **Mid-run, Management issued a process change, since codified in commit `75c63696`** (`CLAUDE.md`, `.claude/agents/qa-security.md`, `management.md`, `playwright-dev.md`): full-suite and multi-browser (`chromium`+`firefox`+`webkit` together) Playwright runs are **CI-only**, never local — not even as a "final" or "consolidated" validation pass. Locally, E2E scope is limited to targeted specs under a single browser (`--project=firefox`). This report captures exactly what was verified before that instruction fully landed, stops all further local full-suite/multi-browser E2E work in compliance with the now-codified policy, and hands back to Management for commit/push so CI can authoritatively confirm cross-browser health. **Do not treat the gates below as a complete DoD sweep** — several were not (re-)run this pass and are marked accordingly, distinct from gates that were actually executed with evidence in this session. No further local full-suite Playwright runs will follow this report.

---

## Gate-by-Gate Status (this pass)

| # | Gate | Status | Detail |
|---|------|--------|--------|
| 1 | E2E Docker image clean rebuild | ✅ **VERIFIED** | 3 separate clean rebuilds run this pass (one per browser cycle, matching prior methodology). Changelog-fixture injection confirmed working (`FIXTURE Injecting E2E changelog fixture` → build → `FIXTURE Reverting changelog fixture overwrite`, working tree left clean). Caddy proxy port auto-sync confirmed (`PLAYWRIGHT_CADDY_PROXY_PORT already set to 8180 in .env` on every rebuild). |
| 2 | Playwright — chromium + security-tests | ✅ **COMPLETED** (see note) | **1354 passed, 2 failed, 42 skipped** (fresh clean-rebuilt container). The 2 failures are **new findings**, not part of the previously-documented 52 — see §1 below. Neither is caused by the 19 commits in scope. **Note**: this was a full-suite run under one browser, executed before the CI-only policy (`75c63696`) was fully in effect for this session. Retained here as useful evidence since it already ran to completion, but this is the **last** local full-suite run this pass — not to be repeated. |
| 3 | Playwright — firefox | ⚠️ **STOPPED MID-RUN**, then **halted entirely per codified CI-only policy** | 673 of ~955 tests completed, **0 failures observed** before stop (all ✓ or expected skips). Includes `tests/settings/ntfy-notification-provider.spec.ts:564` (one of the two explicitly flagged uncertain items) — **passed**. Suite was killed cleanly (process group terminated, no orphaned processes left running) partway through `tests/settings/pushover-notification-provider.spec.ts`, once the process-change instruction landed. Not a completed, authoritative run — do not read "0 failures so far" as a clean bill of health for the untested remainder. **This run will not be resumed or repeated locally** — full-suite/multi-browser confirmation is CI's job per `75c63696`. |
| 4 | Playwright — webkit | ⛔ **NOT RUN, and will not be run locally** | Never started, per the now-codified CI-only policy for full-suite runs. `tests/core/caddy-import/caddy-import-webkit.spec.ts:173` (flagged, no root cause found/no fix applied) and the ImportSession per-user-scope race question remain **unverified**; CI's webkit job is now the only path to confirming or refuting them. |
| 5 | Backend coverage (`go-test-coverage.sh`) | ⛔ **NOT RUN this pass** | `backend/coverage.txt` on disk is stale (timestamped before this session). No fresh number to report. |
| 6 | Frontend coverage (`frontend-test-coverage.sh`) | ⛔ **NOT RUN this pass** | `frontend/coverage/lcov.info` absent. No fresh number to report. |
| 7 | Local patch coverage (`local-patch-report.sh`) | ⛔ **NOT RUN this pass** | `test-results/local-patch-report.{md,json}` absent. |
| 8 | Lefthook pre-commit (staticcheck, CodeQL Go+JS, semgrep) | ⛔ **NOT RUN this pass** | |
| 9 | Trivy scan | ⛔ **NOT RUN this pass** | Prior pass reported 0 new Critical/High with 1 pre-existing tracked HIGH (`CVE-2026-32286`, documented in `SECURITY.md`); not re-confirmed this session. No dependency changes in the 19-commit range (all `fix(test)`/`fix` commits touching `tests/`, `frontend/src/App.tsx`, `frontend/src/pages/Login.tsx`, `.env`/rebuild scripts — no `go.mod`/`go.sum`/`package.json` changes), so risk of drift is low but **not independently re-confirmed**. |
| 10 | `git diff dac267f3..HEAD --stat -- backend/ \| grep '\.go$'` (GORM scan applicability) | ✅ **VERIFIED** | Empty. Confirmed no `.go` files changed in the 19-commit range — GORM scan correctly skippable. |
| 11 | Frontend type-check / build / backend build | ⛔ **NOT RUN this pass** | |
| 12 | Full `npx vitest run` | ⛔ **NOT RUN this pass** | |
| 13 | Tautology grep (`grep -rn "\|\| true" tests/ --include=*.spec.ts`) | ✅ **VERIFIED** | 0 matches. Cheap, non-Playwright check — run to completion. |

---

## 1. New finding: 2 accessibility failures in chromium run (not part of the original 52, not caused by the 19 commits)

**`tests/a11y/dns-providers.a11y.spec.ts:8`** and **`tests/a11y/security.a11y.spec.ts:59`** failed in the full chromium+security-tests run with genuine axe-core violations:

- **DNS Providers page** — `[CRITICAL] button-name`: 2 icon-only delete buttons (`Trash2` icon, no text/`aria-label`) have no accessible name. Root cause: `frontend/src/components/DNSProviderCard.tsx:187-191` — `<Button variant="danger" onClick={...}><Trash2 className="w-4 h-4" /></Button>` with no `aria-label`.
- **Security dashboard page** — `[CRITICAL] select-name`: 2 `<select>` filter elements (log level, log source) have no accessible name. Root cause: `frontend/src/components/LiveLogViewer.tsx:393-419` — both `<select>` elements have no associated `<label>`/`aria-label`.

**Root-cause tracing performed (per CLAUDE.md protocol) before reporting:**
- Neither `DNSProviderCard.tsx` nor `LiveLogViewer.tsx` was touched anywhere in `dac267f3..HEAD` (confirmed via `git diff --stat`) or at any point since March (`git log -1` on both files → `615bdd7e`, an unrelated Vitest-config chore commit). **Not a regression from this round's 19 fix commits.**
- **Order/state-dependent, not flaky-random**: re-ran both spec files in isolation against a freshly-rebuilt, zero-state container — both **passed** (10/10). The violations only manifest once the full suite's cumulative state exists (≥2 DNS providers seeded by earlier tests; security/CrowdSec mode toggled on by earlier security-tests specs, which is what causes `LiveLogViewer`'s security-mode-only filters to render). This is the same class of order-dependent test fragility already flagged in the prior QA report's §2 (item #8 / "Additional finding").
- **Conclusion**: these are real, pre-existing accessibility defects in shipped component code (missing `aria-label` on icon-only buttons and unlabeled `<select>` filters), independently confirmed by source inspection, not test flakiness and not in scope of the 19 commits under review. They were not caught in the prior 52-failure pass because that pass's chromium run apparently didn't reach the same accumulated state (order-dependent) — or the a11y specs simply weren't among the 17 documented chromium failures at that time. Flagging as new, real, actionable findings for a future fix round.

**Remediation** (not applied — QA does not fix production code per standing instruction): add `aria-label` to the delete `<Button>` in `DNSProviderCard.tsx` (e.g. `aria-label={t('dnsProviders.delete', { name: provider.name })}`) and to both `<select>` elements in `LiveLogViewer.tsx` (e.g. `aria-label="Filter by log level"` / `aria-label="Filter by log source"`).

---

## 2. Status of the two explicitly flagged uncertain items

- **`tests/core/caddy-import/caddy-import-webkit.spec.ts:173`** — **UNVERIFIED this pass.** Webkit was not run (stopped before starting per process change). Still an open question carried forward.
- **`tests/settings/ntfy-notification-provider.spec.ts:564`** — **Passed** in the partial firefox run (test #669, `✓ access token should not appear in the url field or any visible field`, 4.3s). This is a positive signal for the timeout-bump fix, but it's from an interrupted, non-authoritative run (not a full clean 3-repeat confirmation) — treat as encouraging, not conclusive.

## 3. ImportSession per-user race (flagged, out of scope for a quick fix)

**Not observed this pass** — webkit and the caddy-import-heavy portions of the suite were not exercised to completion. No new evidence either way. The architectural gap (`backend/internal/models/import_session.go` has no per-user scope — single global "most recent pending session" row) remains unconfirmed as an active failure cause and still worth a dedicated look if `caddy-import-webkit.spec.ts:173` continues to fail under CI's parallel workers.

---

## 4. Historical context: the prior pass (52 failures as of `dac267f3`)

Summarized from the previous full report (superseded by this document): 17 chromium+security-tests failures, 20 firefox failures, 15 webkit failures (with cross-browser overlap), headlined by `settings/whats-new-changelog.spec.ts` failing identically on all 3 browsers (21 of 52 instances, root-caused to a missing changelog fixture in the local E2E rebuild) plus a wrong Caddy proxy port, an aria-hidden toast bug, a stale CrowdSec diagnostics precondition, an auth-fixture 401 race, a Login.tsx unmount race, and roughly a dozen files with inconsistent E2E wait timeouts. The 19 commits in `dac267f3..HEAD` were the fix rounds for that list. This pass's chromium result (1354 passed / 2 failed, both new/unrelated a11y findings) and the clean partial firefox result (673/673 passing, 0 failures) are strong positive signals that those fixes hold, but **do not constitute a full re-confirmation** given the interrupted scope — that's CI's job going forward per the updated process guidance.

---

## 5. Handback to Management

Per the now-codified CI-only policy for full-suite/multi-browser Playwright runs (`75c63696`, reflected in `CLAUDE.md` and `.claude/agents/qa-security.md`): no further local full-suite or multi-browser E2E runs will be performed by this agent, in this pass or future ones. Recommend:
1. Commit the 19 already-landed fix commits (already on branch) sliced/organized as needed, push to origin.
2. Let CI run the full 3-browser matrix to authoritatively confirm firefox (remaining ~280 untested specs) and webkit (entirely untested this pass, including the still-open `caddy-import-webkit.spec.ts:173` question and the ImportSession race question).
3. Backend/frontend coverage, local patch coverage, lefthook (staticcheck/CodeQL/semgrep), and Trivy were not re-run this pass — either confirm via CI or, per the updated agent guidance, run a narrow *targeted* local check (not a full suite re-run) if CI can't cover one of them.
4. New a11y findings (§1) are real and actionable but independent of this branch's scope — recommend a separate small fix (2 `aria-label` additions) rather than blocking this PR, unless project policy (as applied in the prior pass) treats "any failing test blocks merge" as still in force, in which case these 2 need triage too.
5. Going forward, any local E2E work by this agent will use targeted single-spec runs under `--project=firefox` only, per the updated `qa-security.md`.
