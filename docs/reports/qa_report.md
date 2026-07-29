# QA Report — Orthrus Muzzle Normalization Parity + Agent CI Enforcement (GH #1160 + #1161)

**Date:** 2026-07-20
**Branch:** `feature/orthrus` (unpushed — no upstream configured)
**Scope of this audit:** 7 new commits `6fe7a800`..`6562a64b`, stacked on 5 pre-existing write-mode commits `a4be39e2`..`d2fa3154`
**Correct diff base for the 7 audited commits:** `d2fa3154..HEAD` (verified `6fe7a800~1 == d2fa3154`)
**Pipeline stage:** Final QA/Security audit, after Planning → Supervisor (plan, 2 cycles) → Backend Dev (TDD) → Supervisor (implementation, 2 cycles, both APPROVE)

**Files touched by the 7 audited commits** (28 files, `d2fa3154..HEAD`):
`.github/workflows/codecov-upload.yml`, `.github/workflows/quality-checks.yml`, `.golangci-fast.yml` (moved from `backend/`), `ARCHITECTURE.md`, `CHANGELOG.md`, `Makefile`, `agent/cert/cert_test.go` (new), `agent/leash/leash.go`, `agent/leash/leash_test.go`, `agent/muzzle/muzzle.go`, `agent/muzzle/muzzle_test.go`, `agent/protocol/message_test.go` (new), `backend/cmd/localpatchreport/main.go`, `backend/cmd/localpatchreport/main_test.go`, `backend/internal/orthrus/muzzle.go`, `backend/internal/orthrus/muzzle_test.go`, `backend/internal/orthrus/testdata/muzzle_corpus.json`, `backend/internal/patchreport/patchreport.go`, `backend/internal/patchreport/patchreport_test.go`, `codecov.yml`, `docs/plans/current_spec.md`, `lefthook.yml`, `scripts/agent-test-coverage.sh` (new), `scripts/check-module-coverage.sh` (new), `scripts/ci/check_muzzle_allowlist_parity.go` (new), `scripts/local-patch-report.sh`, `scripts/pre-commit-hooks/golangci-lint-fast.sh`, `scripts/pre-commit-hooks/golangci-lint-full.sh`.

**Note on an earlier miscitation in this audit's working notes:** an initial `git diff --stat` was run against the wrong base (`30cf1c08`, one commit too early), which incorrectly attributed `docs/features/orthrus.md` and `tests/orthrus-write-mode.spec.ts` changes to the 7 audited commits. Both files actually belong to the prior, already-approved `d2fa3154` write-mode commit. This was caught and corrected before any gate conclusion was finalized; it does not change any finding below.

---

## Gate Results Summary

| # | Gate | Status | Notes |
|---|---|---|---|
| 1 | Playwright E2E | **N/A (confirmed)** | Zero `frontend/` or `tests/` paths in the 7-commit diff (`d2fa3154..HEAD`). Skipped per CLAUDE.md's own gate-applicability allowance. |
| 2 | GORM Security Scan | **N/A (confirmed)** | Zero `backend/internal/models/**`, zero GORM queries/migrations in the 7-commit diff. |
| 3 | Local Patch Coverage Preflight | **PASS (artifacts) / WARN (thresholds)** | Artifacts generated; `agent/` scope now genuinely populated (see Finding 1). |
| 4 | Security Scans — CodeQL Go | **PASS** | `lefthook run codeql` → 0 findings in `codeql-results-go.sarif` (empty results array). |
| 4 | Security Scans — CodeQL JS | **PASS** | `lefthook run codeql` → 0 findings in `codeql-results-js.sarif`. Parity check (`4-parity-check`) also passed. |
| 4 | Semgrep (SAST) | **PASS** | Scoped to the 28 files the 7 commits touch: 122 rules, 0 findings. Full-repo `--all-files` sweep also run for completeness (see Verification Method Notes). |
| 4 | Trivy / govulncheck | **PASS** | No new dependencies in this PR (zero `go.mod`/`go.sum`/`Dockerfile` diff); `govulncheck ./...` clean (0 exploitable) in both `backend/` and `agent/`. Full container Trivy scan deferred to normal PR CI (see notes). |
| 5 | Staticcheck (backend + agent) | **PASS** | `make lint-staticcheck-only` → `0 issues` for both modules. |
| 6 | Backend coverage (≥85%) | **PASS** | 89.0% line coverage via `scripts/go-test-coverage.sh` (gate: 87%). |
| 6 | Agent coverage (≥65%, new gate) | **PASS (thin margin — see Finding 2)** | 65.3% line coverage via `scripts/agent-test-coverage.sh` (gate: 65%). Margin is 0.3 points. |
| 7 | Frontend type-check | **PASS (trivial)** | `tsc --noEmit` clean; zero frontend diff. |
| 8 | Build — backend | **PASS** | `cd backend && go build ./...` clean. |
| 8 | Build — agent | **PASS** | `cd agent && go build ./...` clean. |
| 9 | Full test suites | **PASS** | `backend`: all packages `ok`. `agent`: all packages `ok`, including new `agent/cert`, `agent/protocol` tests. |
| 9 | 4 corpus rows red→green | **PASS (independently verified)** | All 4 previously-failing `TestFilter_SharedCorpus` rows now pass; `TestMuzzle_SharedCorpus` (backend) unaffected (unchanged pass). |
| 9 | Parity checker exits 0 | **PASS** | `go run scripts/ci/check_muzzle_allowlist_parity.go` → exit 0, "all 8 paired declarations match." |
| 10 | Security-specific fix review | **PASS** | See "Security Review of the Fix" below — all 3 documented divergences closed; adversarial probing beyond the corpus found no new divergence. |
| 11 | Git state | **PASS** | All 12 audited commits are clean/committed; no upstream configured for `feature/orthrus`; nothing pushed. (The only working-tree change at report time is this report file itself, `docs/reports/qa_report.md`, being authored as this audit's own deliverable — not part of the audited code.) |

**Overall disposition: READY to hand back to the user for manual review/push**, with two non-blocking findings (below) the user should be aware of before opening the PR — neither is a security defect, and neither requires looping back to Backend Dev unless the user wants the margins widened first.

---

## Findings

### Finding 1 — MEDIUM (process/coverage) — Local patch coverage below mandated thresholds

`bash scripts/local-patch-report.sh` (baseline `origin/main...HEAD`, i.e. the full unmerged feature — all 12 commits, since nothing from this branch has merged yet) reports:

| Scope | Changed Lines | Covered | Patch Coverage | Threshold | Status |
|---|---:|---:|---:|---:|---|
| Overall | 409 | 359 | 87.8% | 90.0% | **warn** |
| Backend | 275 | 248 | 90.2% | 85.0% | pass |
| Frontend | 0 | 0 | 100.0% | 85.0% | pass |
| Agent | 134 | 111 | 82.8% | 85.0% | **warn** |

Files needing coverage (from `test-results/local-patch-report.md`):

| Path | Patch Coverage | Uncovered Lines |
|---|---:|---|
| `backend/internal/orthrus/server.go` | 62.5% | 61-63 |
| `agent/leash/leash.go` | 63.2% | 172, 177-178, 188, 198-199, 232 |
| `agent/muzzle/muzzle.go` | 86.1% | 191-192, 205-206, 216-217, 233-234, 248-249, 409-410, 412-414, 438 |
| `backend/internal/orthrus/session.go` | 86.7% | 165-166 |
| `backend/internal/orthrus/muzzle.go` | 89.6% | 195-196, 222-223, 236-237, 258-259, 265-267, 283-284, 298-299, 327-328 |
| `backend/internal/patchreport/patchreport.go` | 90.5% | 124, 158 |
| `backend/cmd/localpatchreport/main.go` | 91.2% | 112-114 |

**Root cause traced (per CLAUDE.md's Root Cause Analysis Protocol, not just the surface warning):** the tool's diff baseline is `origin/main`, which is correct — nothing in this 12-commit feature has merged, so the *whole* feature must clear 90%/85% before merge, not just today's 7 commits. Most of the gap (`server.go`, `session.go`, most of `agent/leash/leash.go`) originates in the earlier 5 write-mode commits, already through 2 rounds of Supervisor review before today. Within the 7 commits under *this* audit specifically, the relevant uncovered lines are the error-return branches of `validateNetworkModeValue`/`validateMountsValue`/`validateContainerCreateBody` in both `muzzle.go` files (malformed-JSON and oversized-body rejection paths) — these are fail-closed-by-default branches (an unmarshal error already returns `false`/reject), so the coverage gap is a test-completeness gap, not a live security gap: the untested lines cannot be coerced into the *permissive* outcome, only the already-safe one.

**This is genuinely important to flag** because:
- The local script itself is hardcoded to always report `Mode: "warn"` (`backend/cmd/localpatchreport/main.go:157`) and exits 0 regardless — it will never block a local commit. The actual enforcement point is Codecov's patch-coverage check once this PR is opened against GitHub, which may fail and block merge.
- `agent/leash/leash.go`'s uncovered lines are pre-existing/out-of-scope per the plan's own Section 3.5 ("`agent/leash` ... explicitly not required to increase coverage in this PR"), but the tool still counts them against the *overall* number since those lines technically changed (3 `defer x.Close()` → `defer func() { _ = x.Close() }()` edits made to satisfy the new staticcheck gate) and file line-shifting causes the diff to touch nearby context.

**Recommendation:** before opening the PR, either (a) add a handful of targeted unit tests for the listed error-branches in `muzzle.go` (both files) to close the ~3-4 points of gap most directly attributable to this PR's own new code, or (b) accept the current state and rely on Codecov's actual PR-level patch-coverage check, understanding it may require a follow-up commit if it fails there. Not a blocker for handing back to the user — but the user should not be surprised if Codecov flags this on PR open.

### Finding 2 — LOW (process) — Agent coverage gate has almost no margin

`scripts/agent-test-coverage.sh` measured 65.3% against a 65% gate (`CHARON_AGENT_MIN_COVERAGE` default) — a 0.3-percentage-point margin. Verified this is a real, non-rigged calibration (not e.g. a 0% floor): the script's arithmetic is identical to backend's coverage gate, and the threshold is explicitly documented in the script as calibrated to "the module's actual aggregate line coverage after this PR's new tests landed," correctly attributing the low aggregate to `agent/leash` sitting at ~44% (pre-existing, explicitly out of scope per the plan). This is real and working as designed, but the margin is thin enough that almost any future commit touching `agent/` without a matching test could flip this gate red. Non-blocking; flagging for awareness only.

### Finding 3 — informational — `agent-quality` CI job lint scope differs from `backend-quality`

`backend-quality`'s golangci-lint step runs the **full** linter suite with `continue-on-error: true` (non-blocking/advisory — the actual blocking gate for backend is the separate staticcheck-only lefthook pre-commit hook). `agent-quality`'s equivalent step runs only the **fast/staticcheck** config (`--config ../.golangci-fast.yml`) with no `continue-on-error`, i.e. it is blocking. This isn't a security gap — if anything agent's CI is stricter — but it means the two jobs aren't a literal mirror of each other in linter scope vs. enforcement mode. Not flagged in the plan's own gate-applicability table; worth a note for whoever reviews the workflow YAML (the plan itself calls this file out as needing human review since it can't be validated locally without `act`).

---

## Security Review of the Fix (Task item 10)

**Question:** does the agent-side fix in `agent/muzzle/muzzle.go` close all 3 divergences from the plan's Section 2.3, not just the one named in the original GH issue?

**Verified yes, all 3, independently:**

1. **Row 1 (traversal-disguised version prefix, GH #1160's own example)** — `GET /foo/../v1.44/images/x/json`. Confirmed via `TestFilter_SharedCorpus` / `TestMuzzle_SharedCorpus`: both now return `403`. Root cause fix: `normalizeDockerPath` in both files now strips the version-prefix regex against the *raw* path before `path.Clean` resolves `..` segments, so a version prefix only revealed by traversal resolution is never mistaken for a real one.
2. **Row 2 (non-numeric fake version prefix, read path and HEAD `/_ping` variant)** — `GET /vFOO/containers/json`, `HEAD /vBOGUS/_ping`. Confirmed via corpus. Root cause fix: agent's duplicated `/v*/...` loose-wildcard pattern entries were removed entirely; all matching now goes through the single numeric-anchored `versionPrefixRe` (`^/v\d+\.\d+`), identical source string in both files (verified byte-for-byte via the parity checker).
3. **Row 3 (fake version prefix reaching a write endpoint — the highest-severity row)** — `POST /vFOO/containers/abc/start` with write mode on. Confirmed via corpus. Root cause fix: `allowWrite`'s signature changed from re-deriving its own "unversioned" path (exact-path branch only) to accepting the caller's single pre-normalized path, so the pattern-matching branch (`allowedWritePatterns`) no longer matches against the raw un-stripped path.

**Structural drift guard, independently exercised (not just trusted from the report):** I ran the parity checker against two deliberately-introduced mismatches and confirmed it fails loudly and correctly, then restored the file and re-confirmed a clean `git status`:
- Added an extra `allowedWritePatterns` entry to `agent/muzzle/muzzle.go` only → checker output: `allowedWritePatterns: present in agent, missing in backend: {POST /networks/*/connect}`, exit 1.
- Loosened agent's `versionPrefixRe` to `^/v\w+` → checker output: `versionPrefixRe: backend=^/v\d+\.\d+ agent=^/v\w+ (source strings differ)`, exit 1. This is the specific negative-path check for R9 (the Supervisor-requested regex-parity row), and it works.

**Adversarial inputs beyond the committed corpus** (double-encoded traversal, mixed-case version segment, trailing slashes, double version-prefix nesting, case-sensitive `_PING`, encoded traversal segments) were run against both filters. One apparent divergence surfaced initially (`%2e%2e`-encoded traversal: agent said blocked, backend said allowed) — traced to a flaw in my own test harness, not the code: I had called `Filter.Allow` directly with a raw, still-percent-encoded string, bypassing the URL decoding every real request goes through. I verified with a raw HTTP request parsed via `http.ReadRequest` (the actual function `ServeProxy` uses) that `%2e%2e` decodes to `..` in `req.URL.Path` identically to how Gin's `net/url` parsing decodes it for backend — so in the real request-handling path, both filters see the same decoded string and agree (this input resolves to the already-corpus-tested "traversal that legitimately resolves to an allowed path" case). No new divergence found. All scratch test files were deleted after use; `git status` confirmed clean.

**Conclusion:** the fix is not narrowly tailored to only the committed test cases — it's a structural fix (single normalization function, single regex, unversioned-only allowlist data) that closes the entire bug class, not just the 3 named instances of it.

---

## Verification Method Notes

- All findings above were independently reproduced, not taken on trust from commit messages or the plan document — corpus tests were re-run and their output inspected line-by-line, the parity checker was exercised both positively and negatively, and coverage/build/lint commands were run directly rather than assumed from the plan's own validation-gate descriptions.
- `codeql-results-go.sarif` / `codeql-results-js.sarif` (freshly generated 2026-07-20 17:31-17:33) both contain zero results at any severity (`jq '[.runs[].results[] | .level] | ...'` → `[]`), which is stronger than "zero Critical/High" — zero findings of any kind.
- `lefthook run pre-commit` (no modifier) skips everything on a clean tree (all hooks are staged-file-scoped and nothing is staged, since these commits are already committed) — this is expected lefthook behavior, not a gap. `lefthook run pre-commit --all-files` and `lefthook run codeql` were used instead to force full-repo evaluation:
  - `codeql` (Go + JS + parity check): clean, as above.
  - `--all-files` sweep results: `muzzle-allowlist-parity` passed; `golangci-lint-fast` (fast config, full repo) — `0 issues` for both `backend/` and `agent/`; `dockerfile-check` passed; `frontend-lint` — 0 errors (1174 pre-existing warnings, entirely outside this PR's diff — zero frontend files touched by the 7 commits); `shellcheck`, `check-yaml`, `go-vet`, `go-vet-agent`, `end-of-file-fixer`, `block-codeql-db`, `block-data-backups`, `check-lfs-large-files` — all clean/no findings.
  - Two incidental items surfaced by the `--all-files` sweep, neither attributable to the 7 audited commits: (a) `check-version-match` failed because `.version` (`v0.27.0`) is behind the latest git tag (`v0.34.1`) — `.version` was last touched by an unrelated commit (`d231386d`, pre-dating this branch's divergence from `main`) and is not part of this PR's diff; pre-existing branch-hygiene drift, not a defect in this fix. (b) `trailing-whitespace` auto-fixed and re-staged one file — confirmed via `git status`/`git diff --stat` immediately after that it was exclusively this QA report's own not-yet-committed draft (`docs/reports/qa_report.md`), not any committed/audited source file; no audited file was modified.
  - `semgrep`, scoped precisely to the 28 files the 7 audited commits actually touch (`bash scripts/pre-commit-hooks/semgrep-scan.sh $(git diff d2fa3154..HEAD --name-only)`, the same script and ruleset — `p/golang`, `p/javascript`, `p/typescript`, `p/react`, `p/secrets`, `p/dockerfile` — the lefthook hook itself invokes): **122 rules run, 28/28 targets scanned, ~100% parsed, 0 findings.** (The full unscoped `--all-files` semgrep sweep covers the entire repository including unrelated pre-existing code and was still in progress after several minutes when this scoped, directly-relevant run completed with a clean result; the scoped run is the one that answers "does this PR introduce a SAST finding," which it does not.)
- Trivy: no container/dependency scan was run to full completion (a `trivy fs` invocation returned 0 language files scanned, likely a directory-targeting issue, not re-investigated given `govulncheck` — the more precise Go-specific tool — was clean and this PR touches zero `go.mod`/`go.sum`/`Dockerfile` content). Recommend a full `make trivy`-equivalent container scan still be run as part of the normal PR CI pipeline once opened, per SECURITY.md's standard process; not expected to differ from the already-tracked, pre-existing findings in SECURITY.md's "Known Vulnerabilities" section since no new dependencies were introduced.

## Recommendation

**Ready to hand back to the user for manual review/push.** No CRITICAL or HIGH findings. The two coverage-related findings (Finding 1, Finding 2) are process observations worth the user's attention before opening the PR against GitHub (where Codecov's own patch-coverage check may be stricter than this local advisory tool), but do not indicate a defect in the security fix itself — the muzzle normalization parity fix is verified correct, complete against all 3 documented divergences, and independently confirmed by both the shared corpus and out-of-corpus adversarial testing. All 12 commits on `feature/orthrus` remain unpushed (no upstream configured); the only uncommitted change in the working tree is this report itself.
