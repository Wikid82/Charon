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

---

# QA Report — release-please Migration (CI/CD Config Only)

**Branch**: `chore/release-please-migration`
**Base**: `origin/main` @ `67b4f2da`
**Tip reviewed**: `aa7f2135` (6 commits)
**Reviewed by**: qa-security agent
**Date**: 2026-08-17
**Plan reference**: `docs/plans/current_spec.md` ("CLAUDE.md Definition-of-Done Applicability" section)

## Scope confirmation

Verified independently (not just taking the plan's word for it) that this branch touches zero Go/TypeScript/React/database-schema files: `git diff --stat origin/main..HEAD` shows 22 files changed, all `.yml`/`.yaml`/`.json`/`.md`/`.sh`(shell wrapper only)/dotfiles under `.github/`, root config, `.vscode/`, `ARCHITECTURE.md`, `CLAUDE.md`, `VERSION.md`. Per the plan's own DoD-applicability table, this correctly makes Playwright E2E, GORM scan, backend/frontend 85% coverage gates, frontend type-check, and staticcheck all N/A — none were run, per instruction.

## Gate-by-Gate Status

| # | Gate | Status | Detail |
|---|------|--------|--------|
| 1 | `bash scripts/local-patch-report.sh` | ✅ **PASS** | Ran cleanly on a code-less diff, no errors. Both `test-results/local-patch-report.md` and `test-results/local-patch-report.json` produced. 0 changed `.go`/`.ts`/`.tsx` lines detected in all four scopes (overall/backend/frontend/agent) → reported as 100% (0/0) "pass" per the script's own convention. Confirms the mandatory-regardless-of-change-type gate is satisfied and doesn't error on this diff shape. |
| 2 | `lefthook run pre-commit` | ✅ **PASS** | A bare `lefthook run pre-commit` no-ops (nothing staged in the working tree — the branch's changes are already committed). Re-ran explicitly against the branch's actual changed-file list via `lefthook run pre-commit --force --file <each changed, still-existing file>` to get real signal. Exit code 0, no `❌` in output. `actionlint` validated `.github/workflows/release-please.yml` cleanly; `check-yaml` validated all touched YAML/JSON; `shellcheck`, `dockerfile-check`, `muzzle-allowlist-parity`, `go-vet` (backend+agent), `golangci-lint-fast` (0 issues), `frontend-type-check`, and `frontend-lint` all passed. Note: `--force` caused glob-matched hooks to run repo-wide rather than scoped strictly to the branch's files (e.g. `frontend-lint` reported 1188 pre-existing warnings, 0 errors, across the whole frontend tree) — this is a stronger check than required, not a false pass; all relevant hooks reported clean/zero-error results. |
| 3 | JSON/YAML parse validation | ✅ **PASS** | `jq empty release-please-config.json .release-please-manifest.json` — both valid JSON. `python3 -c "import yaml; yaml.safe_load(...)"` on `.github/workflows/release-please.yml` and `lefthook.yml` — both valid YAML. |
| 4 | CodeQL / Trivy (local) | ➡️ **Correctly deferred to CI** | `chore:`-scoped, no new application code path — per CLAUDE.md's own rule, not run locally. CI runs both unconditionally on every PR regardless, so nothing is skipped, only not duplicated. |
| 5 | `cd backend && go build ./...` | ✅ **PASS (sanity only)** | Clean build, exit 0. Not a real gate for this diff (no backend files touched) — confirms the tree wasn't already broken. |
| 6 | `cd frontend && npm run build` | ✅ **PASS (sanity only)** | Clean build (`✓ built in 10.86s`), exit 0. Not a real gate for this diff (no frontend files touched). |
| 7 | Dangling-reference sweep for deleted release-pipeline assets | ✅ **PASS** | `git grep` (HEAD, tracked files only) for `utility-version-check`, `check-version-match-tag`, `release-goreleaser`, `goreleaser.yaml`, `auto-versioning.yml`, `release-drafter`, `auto-changelog.yml` returns hits only in `docs/implementation/`, `docs/plans/archive/`, `docs/reports/archive/`, and `docs/superpowers/specs/` — all historical/archived implementation records of past work, not live guidance. The four "blast radius" files the plan calls out as needing updates (`.github/skills/README.md`, `.vscode/tasks.json`, `.github/skills/utility-bump-beta.SKILL.md`, `lefthook.yml`) were individually re-checked post-diff and are clean (no matches). |

## Security Assessment: `googleapis/release-please-action` permissions & supply chain

**File**: `.github/workflows/release-please.yml`

```yaml
on:
  push:
    branches: [main]
permissions:
  contents: write
  pull-requests: write
```

- **Permission scope — appropriately minimal.** `contents: write` is required to create tags and GitHub Releases; `pull-requests: write` is required to open/update the standing release PR. This is exactly the documented minimal permission set for `release-please-action` (no `packages:`, `actions:`, `id-token:`, or other elevated scopes granted). No blanket/default `permissions: write-all` used.
- **SHA-pinning convention — consistent with the rest of the repo.** Verified via `git ls-remote --tags` that `45996ed1f6d02564a971a2fa1b5860e934307cf7` resolves directly to the genuine `googleapis/release-please-action` `v5.0.0` tag (same commit object as `refs/tags/v5`, `refs/tags/v5.0`, `refs/tags/v5.0.0`). Spot-checked repo-wide: every `uses:` line across all 35 workflow files (246 total references) is SHA-pinned with a `# vX` trailer comment in the same style (e.g. `actions/checkout@3d3c42e5...# v7`); grepping for any `uses:` line *without* a 40-char SHA turned up zero real matches (one false hit was a comment, not a `uses:` line). The new action reference follows the established convention exactly.
- **Fork/untrusted-contributor exposure — none.** The workflow triggers only on `push: branches: [main]`, not `pull_request` or `pull_request_target`. An external/untrusted contributor cannot invoke this workflow (with its `contents: write` + `pull-requests: write` grant) directly via a fork PR — it only ever runs after code has already been merged to `main` by a maintainer, which is the standard/expected trust boundary for a release-cutting workflow. Repo-wide grep for `pull_request_target` (the genuinely dangerous pattern for granting write perms to fork-triggered runs) found it only in `codecov-upload.yml` and `quality-checks.yml`, neither touched by this branch — out of scope for this review but noted as already-existing, unrelated surface.
- **SECURITY.md alignment**: the repo's documented "Digest Pinning Policy" (SECURITY.md, ~line 959) requires digest-pinned refs for `.github/workflows/*.yml`; the new action satisfies this.
- **No CodeQL/Trivy coverage gap left unaddressed**: as the plan itself notes, neither tool evaluates GitHub Actions permission scopes or third-party Action supply-chain trust — this manual review is the actual mitigation for that gap, not a CI tool. Assessment above stands in for that coverage.

**Verdict: no CRITICAL/HIGH findings on the permissions/supply-chain surface.** Scope is minimal, pinning is consistent and verified genuine, and the trigger surface excludes untrusted fork contributions.

## CLAUDE.md diff verification (independent re-check of governance-sensitive file)

`git diff origin/main..chore/release-please-migration -- CLAUDE.md` shows exactly **one line removed**, at the former Skills-table row:

```diff
-| `utility-version-check` | Check tool versions |
```

No other line in `CLAUDE.md` is touched — confirmed by inspecting the full diff output directly (not by re-reading Supervisor's prior conclusion). No governance/precedence text, security requirement, DoD gate, commit-convention rule, or any other policy statement in `CLAUDE.md` is altered by this branch. This independently confirms Supervisor's earlier finding.

## Minor / non-blocking findings

**[LOW] `.dockerignore` retains a stale "GoReleaser" comment label after this PR removes GoReleaser itself.**

- `git diff origin/main..HEAD -- .gitignore .dockerignore` shows `.gitignore` fully removed its `# GoReleaser` section (comment header + the root-level `dist/` rule that was GoReleaser-specific — confirmed redundant, since `frontend/dist/` already has its own explicit ignore rule at `.gitignore:37`).
- `.dockerignore`, by contrast, only removed the standalone `.goreleaser.yaml` config-file exclusion line, but left an untouched section a few lines down still headed `# GoReleaser & dist artifacts` (`.dockerignore:157-159`) guarding a `dist/` rule.
- This is **not a functional bug** — the underlying `dist/` pattern is not purely dead weight; unanchored `dist/` in `.dockerignore` also incidentally excludes `frontend/dist/` (and any other `dist` directory) from the Docker build context, which remains legitimately useful independent of GoReleaser. The issue is purely the comment label now name-dropping a tool this very PR retires, which is a minor accuracy/consistency gap against the plan's own acceptance criterion ("`.gitignore` and `.dockerignore` no longer reference GoReleaser artifacts/config" — `.gitignore` fully satisfies this, `.dockerignore` only partially does).
- **Remediation**: rename the comment to something like `# Build/dist artifacts` (or fold the `dist/` line into the existing generic exclusions block) in a follow-up commit or this PR's next revision. Not blocking — does not affect security, correctness, or CI behavior.

No other findings. No secrets, tokens, or credentials observed in any file touched by this branch (also consistent with this being a pure CI-config/docs change with no logging/API-example surface).

## Summary

## Final Overall Verdict: **PASS — ready to be marked done.**

---
---

# QA/Security Audit — Notify Provider Registry (Self-Registering Factory) (Independent Verification)

**Branch**: `feature/notifications-engine-extraction`
**Commit range reviewed**: `a28f0db9..HEAD` (`e3e971ec`, `326d28e9`, `14421c06`, `567de4e7`, `7c48f009`)
**Reviewed by**: qa-security agent
**Date**: 2026-08-17
**Scope**: Backend-only. Replaces a hardcoded `switch p.Type { case "discord": ... }` provider-construction dispatch in `backend/internal/services/notify_provider_adapter.go` with a call into a new self-registering factory registry (`notify.New`) shipped by a companion module, `github.com/Wikid82/go_notify_yourself` (pinned `v0.2.0` via a local-path `replace` directive to `/projects/go_notify_yourself`, branch `feature/provider-registry`, not yet pushed to GitHub — expected/tracked, not a defect).
**Prior reviews**: `backend-dev` implementation pass + `supervisor` code review — **APPROVED WITH MINOR NOTES** (no blocking issues; email construction path and the local-path `replace` directive both flagged as known, non-blocking).
**Purpose**: Independent re-verification of all Definition of Done gates plus a dedicated security review of the new `map[string]any` config-boundary and provider registry, per `docs/plans/notify_provider_registry_spec.md` §3 and §5.

## Summary Verdict: **READY TO MERGE** — no blocking issues found. All Definition of Done gates independently re-run and passed with real, non-cached numbers where applicable.

---

## 1. Definition of Done Gates (all independently re-run from repo root / `backend/`)

| Gate | Command | Result |
|---|---|---|
| Backend build | `cd backend && go build ./...` | **PASS** — clean, zero errors |
| Static vet | `cd backend && go vet ./...` | **PASS** — zero findings |
| Staticcheck | `make lint-staticcheck-only` | **PASS** — `0 issues.` (backend), `0 issues.` (agent) |
| Full backend test suite | `cd backend && go test ./...` | **PASS** — all packages `ok`, zero failures. `internal/services` (the touched package) ran uncached, 111.308s, all green. |
| Backend coverage gate | `bash scripts/go-test-coverage.sh` (`CHARON_MIN_COVERAGE` default 87%) | **PASS** — Statement coverage **89.3%**, line coverage **89.2%** (gate: line coverage ≥ 87%). Console: `Coverage gate (line coverage): minimum required 87% / Coverage requirement met`. |
| Local patch coverage preflight | `bash scripts/local-patch-report.sh` | **PASS** — Backend: 235/235 changed lines covered = **100.0%** patch coverage (threshold 85%). Overall/Frontend/Agent all 100.0% (0 changed lines outside backend). Artifacts confirmed at `test-results/local-patch-report.md` and `test-results/local-patch-report.json`. |
| GORM security scan | `./scripts/scan-gorm-security.sh --check` | **PASS** — `CRITICAL: 0`, `HIGH: 0`, `MEDIUM: 0`, `INFO: 2` (both pre-existing, in `backend/internal/models/user.go`'s `UserPermittedHost` struct — unrelated to this change; 61 files / 3675 lines scanned). |
| Lefthook pre-commit | `lefthook run pre-commit` | **N/A / clean** — working tree is clean (no staged changes; this is an audit pass on already-committed code, `git status` confirmed clean at session start), so every hook reported `(skip) no matching staged files`. The equivalent checks (`go vet`, staticcheck) were independently re-run directly above and passed. |

Playwright E2E: **not run**, per task scope. Verified during the security review (§3 below) that provider dispatch behavior is byte-for-byte unchanged — `notify_provider_adapter_test.go`'s existing per-provider tests assert the exact same dispatch URL, headers, and JSON payload shape as the pre-registry switch (e.g. Gotify's `X-Gotify-Key` header, Pushover's hardcoded production URL + `user`/`token` payload fields, Telegram's `bot<token>/sendMessage` URL). No new HTTP payload/header/URL behavior was introduced for any provider type, so the Playwright skip condition in the task brief is satisfied.

---

## 2. Commit-Range Diff Verification

- `git diff --stat a28f0db9..HEAD`: 7 files changed, 223 insertions(+), 84 deletions(-) — `ARCHITECTURE.md`, `backend/go.mod`, `backend/go.sum`, `notification_service_registry_consistency_test.go` (new), `notify_provider_adapter.go`, `notify_provider_adapter_test.go`, `notify_providers_import.go` (new).
- `git diff a28f0db9..HEAD -- backend/internal/models/notification_provider.go` → **empty**. Confirms the `Token` field's `json:"-"` GORM protection is untouched.
- `git diff a28f0db9..HEAD -- backend/internal/services/notification_service.go` → **empty**. Confirms `isSupportedNotificationProviderType`, `supportsJSONTemplates`, `isDispatchEnabled` are byte-for-byte unchanged, per the spec's hard design requirement (§3.6.2 Option A).
- `grep -rn "providers/all" backend/` → only 2 matches, both inside a comment in `notify_providers_import.go` explicitly explaining *why* `providers/all` is deliberately **not** imported. No actual import anywhere in `backend/`.

---

## 3. Security-Specific Findings

### 3.1 No token/secret leakage into logs or errors — **PASS**

Read `backend/internal/services/notify_provider_adapter.go` and `notify_providers_import.go` in full, plus every `providers/*/register.go` in `/projects/go_notify_yourself` (discord, slack, gotify, pushover, ntfy, telegram, webhook, email) and `factory.go`.

- The only error-wrapping call site in `notify_provider_adapter.go` is `fmt.Errorf("notify provider adapter: %w", err)` (line 175) — wraps, never re-formats field values.
- Every registry-level error (`notify.New`'s "no provider registered for type %q", each factory's `config["transport"] must be a non-nil *transport.Wrapper` / `config["mailer"] must be a non-nil Mailer`) references only **field/key names and the provider type discriminator** — never `provider.URL`, `provider.Token`, or any config map value.
- Log call sites that consume `buildNotifySender`'s error (`notification_service.go:312`, `:324`) log only `util.SanitizeForLog(p.Name)` plus the wrapped error — never `p.URL`/`p.Token`.
- No `fmt.Println`, `log.Print`, or debug statements were introduced anywhere in the diff.

**Conclusion**: no Gotify token, webhook URL-as-secret, or any provider credential can reach logs, error messages, or test artifacts through this code path. SECURITY.md's "Gotify Token Hygiene" requirement is satisfied.

### 3.2 `Token` field GORM protection untouched — **PASS**

Confirmed via the empty diff above (§2). `Token string \`json:"-"\`` is unchanged in `backend/internal/models/notification_provider.go`.

### 3.3 `map[string]any` config-boundary type-confusion risk — **PASS, verified independently, not just re-trusted**

Read `/projects/go_notify_yourself/factory.go` and all eight `providers/*/register.go` files directly (not assumed from the spec). Every factory:

- Type-asserts `config["transport"].(*transport.Wrapper)` (or `config["mailer"].(Mailer)` for email) with the two-value `ok` form and an explicit nil check — **never a bare/panicking assertion**.
- Returns a descriptive `fmt.Errorf` (never panics) when the assertion fails or the value is nil.
- Delegates all scalar/slice field extraction to `providers/internal/regconfig.StringField`/`StringSliceField`, both of which are deliberately lenient: a wrong-typed or missing key produces the zero value (`""`/`nil`), never a panic. Verified via `regconfig`'s own test table, which explicitly covers `wrong type int`, `wrong type nil value`, `any slice with non-string element`, and `wrong type entirely` — all resolve to zero values, not panics.
- `Register` itself panics only on programmer misuse (`nil` factory, empty name, duplicate registration) at `init()` time — never on caller-supplied runtime data, matching the `database/sql`/`image` prior-art convention the spec cites.

Charon's own tests (`notify_provider_adapter_test.go`) independently exercise this boundary: `TestBuildNotifySenderMissingTransportErrors` and `TestBuildNotifySenderInvalidTransportInConfigMapErrors` both assert a nil/invalid transport produces an error containing "transport", not a panic. `TestBuildNotifySenderUnsupportedTypeErrors` asserts an unregistered type ("carrier-pigeon") produces a "no provider registered" error, not a panic.

**Conclusion**: no type-confusion panic path exists at the registry boundary. Supervisor's prior claim is independently confirmed, not merely re-trusted.

### 3.4 `isSupportedNotificationProviderType` / `supportsJSONTemplates` / `isDispatchEnabled` unchanged — **PASS** (see §2, empty diff)

### 3.5 `providers/all` not imported anywhere in `backend/` — **PASS** (see §2)

`notify_providers_import.go` hand-picks exactly Charon's eight supported types (`discord`, `email`, `gotify`, `ntfy`, `pushover`, `slack`, `telegram`, `webhook`), matching `isSupportedNotificationProviderType`'s allowlist. A new consistency test, `TestSupportedProviderAllowlistIsSubsetOfRegisteredTypes` (`notification_service_registry_consistency_test.go`), asserts this allowlist is a subset of `notify.RegisteredTypes()` — guarding against future drift between the hand-picked imports and the allowlist.

### 3.6 `go.mod` local-path replace directive — tracked, not a defect

```
replace github.com/Wikid82/go_notify_yourself => /projects/go_notify_yourself
```

Annotated in `go.mod` with a `TODO` explaining it must be removed once `go_notify_yourself v0.2.0` is pushed/tagged upstream. This is a real, temporary condition (confirmed: `/projects/go_notify_yourself` is on local branch `feature/provider-registry`, not yet on GitHub) and matches what the task brief and supervisor's prior review both already flagged as expected. Not a merge blocker for the Charon-side PR per the spec's own commit-slicing sequencing, but **must** be resolved before this local-path pin would work in CI or for any other contributor — flagged below as a required follow-up.

---

## 4. Non-Blocking Follow-Ups (tracked, not blocking this PR)

1. **`go.mod` local-path `replace` directive** (`backend/go.mod:9`) must be removed and re-pinned to the real published `go_notify_yourself v0.2.0` tag once `/projects/go_notify_yourself`'s `feature/provider-registry` branch is pushed and tagged on GitHub. Already tracked via the in-file `TODO` comment; CI/other contributors cannot build this branch until resolved.
2. Email's construction (`notification_service.go:350`, `dispatchEmailViaNotify`) still calls `email.New(...)` directly rather than routing through `notify.New` — consistent with pre-existing architecture (email's `Mailer`/`TemplateRenderer` DI seam predates this work) and not a regression, per supervisor's prior note. No action required by this PR.

## Blocking Issues

**None.**

## Final Overall Verdict: **READY TO MERGE**

All seven independently re-run Definition of Done gates pass with real numbers (89.3%/89.2% overall backend coverage against an 87% gate; 100% patch coverage on the 235 changed lines against an 85% gate; 0 CRITICAL/HIGH/MEDIUM GORM findings; 0 staticcheck/vet findings; full backend suite green). The security-specific review independently verified — by reading the actual factory/register code in `/projects/go_notify_yourself`, not by re-trusting the prior supervisor review — that no token/secret can reach logs or error messages, that the `map[string]any` registry boundary cannot panic on caller-controlled input, that the `Token` field's `json:"-"` protection and Charon's three provider allowlists are byte-for-byte unchanged from before this work, and that `providers/all` is never imported in `backend/`. The one open item (local-path `go.mod` replace directive) is already tracked and does not block merging this Charon-side PR per the spec's own two-repo commit-slicing sequencing.
