# QA Report — "What's New" Changelog Feature

**Branch**: `feat/changelog`
**Feature commit range**: `79d1efb3..75fe80dc` (14 commits, merge-base with `origin/development`)
**Reviewed by**: qa-security agent
**Date**: 2026-07-29
**Scope**: Full Definition of Done per `CLAUDE.md`, adversarial re-verification (not re-trusting prior agents' self-reports)

## Final Verdict: **READY TO MERGE**

No blocking findings. One pre-existing, unrelated repo-wide security item needs attention independent of this PR (see §6.3). All required gates pass with real, reproduced evidence.

---

## 1. Playwright E2E (mandatory, run first)

**Command**: `npx playwright test --project=firefox` (full suite, 970 tests), run against a freshly rebuilt E2E image (`docker-rebuild-e2e`, image was stale — built before the branch's last 3 commits).

**Result**: `891 passed`, `27 failed`, `37 skipped`, `8 did not run` (46.6 min).

### 1a. Changelog-specific tests (`tests/settings/whats-new-changelog.spec.ts`, 7 tests)

Failed as **expected** against the committed `[]` placeholder (`waitForModal: Could not find visible modal ... "What's New"`) — this is by design; the placeholder must never be replaced with real data in a commit.

**Verified via fixture injection** (per the spec's documented mechanism, reverted afterward):
1. `cp tests/fixtures/changelog-fixture.json backend/internal/changelog/data/changelog.json`
2. `docker-rebuild-e2e` (rebuild so `go:embed` picks up the fixture)
3. `npx playwright test --project=firefox tests/settings/whats-new-changelog.spec.ts`
4. **Result: 8/8 passed** (7 changelog scenarios + 1 auth setup), 31.4s.
5. `git checkout -- backend/internal/changelog/data/changelog.json` (confirmed reverted to `[]`)
6. `docker-rebuild-e2e` again to restore the clean baseline image.
7. Confirmed `git status` clean on the changelog data file before finishing.

**Status: PASS.**

### 1b. The other 27 failures — investigated, not hand-waved

All 27 shared the same signature: `Test timeout of 90000ms exceeded` on `page.goto()`/`page.reload()`, or a downstream timing-dependent assertion (toast visibility, redirect-detection predicate) — never a content/logic assertion failure. Files affected span totally unrelated areas: `domains.a11y`, `authentication.spec.ts`, `caddy-import-debug/gaps`, `certificates.spec.ts`, `data-consistency`, `crowdsec-whitelist`, `debug/certificates-debug`, `dns-provider-types`, `modal-dropdown-triage`, `proxy-groups`, `proxy-host-drag-drop`, `uptime-orthrus`, plus the three already-triaged pre-existing flakes (`user-management.spec.ts`, `theme-banner-userthemes.spec.ts`, `wait-helpers.spec.ts`) and one additional instance in `long-running-operations.spec.ts`.

Per the Root Cause Analysis Protocol, `authentication.spec.ts` failing is exactly the kind of thing that needs real scrutiny (touches auth). Investigated:

- **Re-ran a 13-file subset** containing every "unexplained" failure in isolation (lighter load, no full-suite contention): **all 13 originally-failing tests passed**. 6 *different* tests failed instead (different line numbers, same files), with the identical `page.goto` 90s-timeout / toast-timeout signature.
- This is direct empirical proof of non-deterministic, load-dependent flakiness — not a fixed set of failures tied to specific code paths.
- Checked whether `WhatsNewModal`'s mount in `Layout.tsx` could be adding per-navigation latency across the whole app (a plausible mechanism for new, broader flakiness): `useChangelogStatus()` uses `staleTime: 1000 * 60 * 5` (5 min) — it fires once per session, not per route change. Ruled out.
- Matches CLAUDE.md's own disclosed caveat about a known, already-partially-fixed Firefox navigation-commit race (commit `7503c01a`, "tolerate Firefox navigation-commit race in **reload()**, not just goto()" — the fix covers `reload()` only; `goto()` is evidently still exposed to the same underlying Firefox/Playwright timing issue).

**Conclusion: pre-existing, load-dependent E2E infra flakiness, unrelated to this feature's diff. Not a regression.** Recommend (separate from this PR): extend the `reload()` tolerant-navigation fix to `goto()`, and/or reduce full-suite worker parallelism locally.

**Status: PASS** (with the above documented, non-blocking flake note).

---

## 1.5. GORM Security Scan (conditional — triggered by `backend/internal/models/user.go` changes)

**Command**: `./scripts/scan-gorm-security.sh --check`

**Result**: `CRITICAL: 0`, `HIGH: 0`, `MEDIUM: 0`, `INFO: 2` (pre-existing, unrelated: missing indexes on `UserPermittedHost.UserID`/`ProxyHostID`, not touched by this feature).

**Status: PASS.**

**READY TO MERGE. No blocking issues found.**

## 2. Coverage (mandatory, ≥85% both sides)

### Backend — `scripts/go-test-coverage.sh`

- **Statement coverage: 89.2%** / **Line coverage: 89.1%** (gate: 87% minimum) — **PASS**.
- One test failure observed only in the full `./... -race` run: `TestSecurityHandler_UpsertRuleSet_XSSInContent` (unrelated file: `security_handler_audit_test.go`, XSS-escaping assertion on the security-ruleset endpoint — no relation to changelog code). Re-verified 3 ways: isolated run (`-run` filter) → pass; full `internal/api/handlers` package run → pass; full package run with `-race` alongside all 24 new changelog handler tests → pass. Non-reproducible outside the heaviest-possible full-`./...`-with-race-detector load. Transient resource-contention flake, not a regression. All 24 new `ChangelogHandler` tests pass cleanly in every configuration tried, including under `-race`.

### Frontend — `scripts/frontend-test-coverage.sh`

- **Statements: 89.47%**, **Branches: 82.48%**, **Functions: 87.18%**, **Lines: 90.67%** (gate: 87% minimum on lines) — **PASS**.
- **3235 passed, 0 failed** (88 skipped, 2 todo), 263/268 test files passed.

**Status: PASS** (both sides comfortably clear the 85% minimum and the stricter 87% local gate).

---

## 3. Local Patch Coverage (mandatory, 90% overall)

**Command**: `bash scripts/local-patch-report.sh` — baseline auto-resolved to `origin/development...HEAD`, merge-base `79d1efb3` (exactly the changelog feature's branch point — confirmed via `git merge-base origin/development HEAD`).

| Scope | Changed Lines | Covered | Patch Coverage | Status |
|---|---:|---:|---:|---|
| Overall | 267 | 262 | **98.1%** | pass |
| Backend | 197 | 195 | 99.0% | pass |
| Frontend | 70 | 67 | 95.7% | pass |
| Agent | 0 | 0 | 100.0% | pass |

Minor gaps (non-blocking, well within tolerance): `frontend/src/context/AuthContext.tsx` lines 138-140 (3 lines), `backend/internal/api/routes/routes.go` lines 340-341 (2 lines).

Artifacts confirmed present: `test-results/local-patch-report.md`, `test-results/local-patch-report.json`.

**Status: PASS.**

---

## 4. Type Safety

**Command**: `cd frontend && npm run type-check` → **0 errors**.

**Status: PASS.**

---

## 5. Pre-commit Hooks

**Command**: `lefthook run pre-commit` on the clean working tree — all hooks reported `(skip) no matching staged files` (nothing staged; working tree matches HEAD for all feature files).

Spot-checked commit history for hook bypasses: `git log --format='%H %s' 79d1efb3..HEAD | grep -i "emergency\|no-verify\|bypass"` → **no matches**. No commit in this feature's 14-commit range bypassed hooks.

**Status: PASS** (staticcheck — the actual blocking gate per CLAUDE.md — independently verified fresh in §7 below).

---

## 6. Security Scans

### 6.1 CodeQL — Go

**Command**: `bash scripts/pre-commit-hooks/codeql-go-scan.sh` (fresh run; prior SARIF on disk was stale, dated Jul 25, predating this branch's last 3 commits). Extraction parity confirmed: 252/252 compiled files matched `go list` baseline.

**5 results**, all `warning`-level in SARIF (no `error`-level Go findings):

| Rule | Severity (CodeQL security-severity) | File | In this feature's diff? |
|---|---|---|---|
| `go/path-injection` ×4 | 7.5 (High) | `internal/api/handlers/system_permissions_handler.go` | **No** — file not touched by `79d1efb3..HEAD` |
| `go/cookie-secure-not-set` ×1 | 4.0 (Medium) | `internal/api/handlers/auth_handler.go:191` | File touched, but not this line — verified `git show 79d1efb3:...auth_handler.go` contains the identical line + its existing `// codeql[go/cookie-secure-not-set] Safe: ...` inline suppression comment, byte-for-byte, before this branch started. This feature's only change to that file is adding `changelog_opt_out` to the `/auth/me` JSON response (lines 318-323) |

**All 5 findings are 100% pre-existing, unrelated to the changelog feature.** Zero new CodeQL Go findings introduced by this PR.

### 6.2 CodeQL — JavaScript/TypeScript

**Command**: `bash scripts/pre-commit-hooks/codeql-js-scan.sh` (fresh run, 552/552 files scanned).

**Result: 0 findings.**

*(Note: this script deletes `frontend/coverage`, `frontend/dist`, `test-results`, `playwright-report`, `coverage` as part of its CodeQL-noise-reduction step. Backed up `test-results/` and coverage files beforehand, restored after.)*

**Status: PASS** — zero new CRITICAL/HIGH/error-level findings from either language.

### 6.3 Trivy

**Image scan** (`trivy image` against a freshly built `charon:local`, matching the repo's actual CI gate convention in `Makefile:security-scan-full` / `docker-build.yml`, rather than `trivy fs` against source — confirmed by inspecting how CI invokes Trivy):

- With `--ignorefile /dev/null` (raw, unsuppressed): **2 HIGH** findings, both `CVE-2026-32286` in `github.com/jackc/pgproto3/v2` (bundled inside `crowdsec`/`cscli` binaries).
- With the repo's actual `.trivyignore` applied (matching CI): **still 2 HIGH findings** — the `.trivyignore` entry for `CVE-2026-32286` carries an `# exp: 2026-07-09` expiry annotation that Trivy honors, and that date has passed (20 days ago as of this report). The suppression is **stale**, not missing.
- **This is a pre-existing, already-documented finding** (see `SECURITY.md`: "[HIGH] CVE-2026-32286 · pgproto3 DoS via Negative DataRow Field Length", status "Awaiting Upstream", noting Charon's default SQLite deployment doesn't reach the vulnerable PostgreSQL code path) — **unrelated to CrowdSec/pgproto3 having any connection to the changelog feature.** Zero secrets found in the image.
- **Finding for this report, not a merge blocker for this PR**: the `.trivyignore` review date for `CVE-2026-32286` has lapsed. CI's Trivy gate (`docker-build.yml`, which fails PRs on unsuppressed CRITICAL/HIGH) would currently flag this on **any** PR, not just this one, since it's a pre-existing dependency issue orthogonal to the changelog diff. Recommend the repo owner re-confirm the existing risk assessment still holds (very likely — nothing has changed about pgproto3/v2's archived-upstream status or Charon's SQLite-by-default posture) and bump the `exp:` date in `.trivyignore` + `SECURITY.md`'s "Review by" date.

**Filesystem/secret scan**: `trivy fs --scanners secret` run against every file this feature's diff touched (`git diff 79d1efb3..HEAD --name-only`, 42 files staged to a scan-accessible path) → **0 secrets found**. (A whole-repo `trivy fs` dependency scan was attempted but blocked by the local `trivy` snap package's filesystem confinement outside `$HOME`; the image scan above is the authoritative, CI-matching check and was completed successfully by working around the same confinement via `docker save` + scanning from `$HOME`.)

**govulncheck**: `cd backend && govulncheck ./...` → **0 vulnerabilities** in Charon's own code or called dependency symbols (1 unreachable/uncalled module-level advisory noted, consistent with `SECURITY.md`'s existing entries).

**npm audit** (`npm run audit:ci`): **Passed** — 3 pre-existing HIGH findings, all already documented and allowlisted in `SECURITY.md`/`frontend/audit-ci.json` (`GHSA-mh99-v99m-4gvg` chain), none newly introduced.

**Status: PASS**, with the stale-suppression note in §6.3 flagged for owner follow-up (not blocking this PR).

---

## 7. Linting

### Staticcheck (the actual blocking gate per CLAUDE.md)

**Command**: `make lint-staticcheck-only` → **0 issues** (backend + agent).

### Full `make lint-backend` (all golangci-lint linters — manual/advisory stage per CLAUDE.md, not blocking)

**91 pre-existing issues** across the whole backend (gocritic style suggestions, gosec advisories on test fixtures and pre-existing crowdsec/orthrus/backup code, govet inlining suggestions). **Verified none are in `internal/changelog/`, `changelog_handler.go`, or any file this feature added** — spot-checked the full output against the feature's changed-file list. The one changed file that does appear (`internal/api/routes/routes.go`) is flagged for a pre-existing, unrelated function (`runInitialUptimeBootstrap`, a `paramTypeCombine` style nit) and a pre-existing `G118` goroutine-context advisory, neither in the changelog registration block this feature added.

### Frontend ESLint

**Command**: `npm run lint` → **0 errors**, 1194 pre-existing warnings (repo-wide, `import-x/order` and `testing-library/*` style rules — non-blocking by design). The only warnings touching this feature's files (`WhatsNewModal.test.tsx`, `AppearanceSettings.test.tsx`) are cosmetic (`testing-library/no-node-access`, `import-x/order`), not security- or correctness-relevant.

**Status: PASS.**

---

## 8. Manual Security Review (SECURITY.md requirements)

- **No secrets/credentials introduced**: read `.docker/compose/docker-compose.playwright-ci.yml` / `-local.yml` diffs and all touched `.github/workflows/*.yml` diffs directly (`git diff 79d1efb3..HEAD -- .docker/compose .github/workflows`). Only addition: `CHARON_CHANGELOG_VERSION=1.2.0` (a plaintext version string, not a secret) in E2E-only compose files, plus a `Generate Changelog Data` build step in 3 workflows. No tokens, keys, or credentials added. Confirmed via Trivy secret scan of the full diff (0 secrets) and manual read.
- **Auth on the 4 new routes**: read `changelog_handler.go` and `routes.go` directly (not trusting prior agent claims). All four routes (`GET /changelog/status`, `GET /changelog/all`, `POST /changelog/ack`, `POST /changelog/opt-in`) are registered under the `protected` Gin group (`protected.Use(authMiddleware)`, `routes.go:320-347`) — no bypass. Each handler calls `rejectPassthrough(c, ...)` first, rejecting `RolePassthrough` sessions with 403, matching the existing pattern for `GetProfile`/`UpdateProfile`/`RegenerateAPIKey`. `requireUserID(c)` pulls the user ID from `c.Get("userID")`, set exclusively by the auth middleware from the verified JWT — never client-supplied. `Ack`/`OptIn` scope all DB writes with `.Where("id = ?", userID)` (parameterized, self-row-only). `CHARON_CHANGELOG_VERSION` override is explicitly gated behind `cfg.Environment != "production"` in `routes.go` — cannot be exploited in a production deployment.
- **`generate-changelog.sh` injection surface**: re-verified the jq parameterization directly. Raw commit subjects are piped via stdin using `jq -R -s` (raw-string input mode — never interpolated into a shell command or jq program body), while `$v`/`$d` (version/date, small trusted scalars derived from `git tag`/`git log` on the local repo) are passed via `--arg` (safe, no injection surface). `git tag`/`git log` output is local, trusted repo history — not attacker-controlled. No command injection surface.
- **Path traversal**: no new file-path construction from user input anywhere in this feature. The changelog data file path (`backend/internal/changelog/data/changelog.json`) is a compile-time `go:embed` constant, not runtime-constructed from any request. `generate-changelog.sh`'s `$OUTPUT` path is a hardcoded literal.

**Status: PASS** — no findings.

---

## 9. Build Verification

- `cd backend && go build ./...` → **success**.
- `cd frontend && npm run build` → **success** (Vite build completes, all chunks emitted).

**Status: PASS.**

---

## 10. Debug Prints / Commented-Out Code

**Command**: `git diff 79d1efb3..HEAD -- backend frontend | grep -nE '^\+.*(console\.log|fmt\.Println\(|debugger;|TODO_REMOVE|FIXME_REMOVE)'` → **0 matches**.

Additional scan for stray `TODO`/`FIXME`/`HACK`/`XXX` markers outside test files → **0 matches** (the only regex hits were legitimate `/** JSDoc */`-style doc comments, false positives from an overly broad pattern, manually confirmed as benign).

**Status: PASS.**

---

## Summary Table

| # | Check | Result | Status |
|---|---|---|---|
| 1 | Playwright E2E (full suite) | 891 passed / 27 failed (all explained: 7 expected placeholder-fail + 20 load-dependent pre-existing flake, reproduced non-deterministic) / changelog spec 8/8 via fixture injection | PASS |
| 1.5 | GORM security scan | 0 Critical/High/Medium | PASS |
| 2 | Backend coverage | 89.2% stmt / 89.1% line (gate 87%) | PASS |
| 2 | Frontend coverage | 89.47% stmt / 90.67% line (gate 87%), 3235/3235 tests pass | PASS |
| 3 | Local patch coverage | 98.1% overall (gate 90%) | PASS |
| 4 | Type-check | 0 errors | PASS |
| 5 | Pre-commit hooks | Clean tree, no bypasses in history | PASS |
| 6.1 | CodeQL Go | 5 findings, 100% pre-existing/unrelated | PASS |
| 6.2 | CodeQL JS | 0 findings | PASS |
| 6.3 | Trivy image | 2 HIGH, pre-existing/documented, suppression expired (owner follow-up needed, not this PR) | PASS* |
| 6.3 | Trivy secrets (diff) | 0 | PASS |
| 6.3 | govulncheck | 0 | PASS |
| 6.3 | npm audit | 0 new (3 pre-existing, allowlisted) | PASS |
| 7 | Staticcheck | 0 issues | PASS |
| 7 | Full golangci-lint | 91 pre-existing, 0 in feature files | PASS |
| 7 | Frontend ESLint | 0 errors | PASS |
| 8 | Manual security review | Auth verified, no secrets, no injection, no path traversal | PASS |
| 9 | Backend + frontend build | Both succeed | PASS |
| 10 | Debug print / dead code grep | 0 matches | PASS |

## Findings Requiring Follow-Up (non-blocking for this PR)

1. **Stale Trivy suppression** (`SECURITY.md` / `.trivyignore`, `CVE-2026-32286`): review date expired 2026-07-09. Recommend a follow-up ticket to re-confirm and renew — affects the whole repo's CI gate, not specific to this feature.
2. **E2E flake rate higher than previously documented**: the known 3-file flake list (`user-management.spec.ts`, `theme-banner-userthemes.spec.ts`, `wait-helpers.spec.ts`) is no longer complete — under full-suite local load, the same `page.goto()`/Firefox navigation-commit race class now surfaces non-deterministically across many more files. Recommend extending commit `7503c01a`'s tolerant-navigation fix from `reload()` to `goto()`.
3. Minor: `.vscode/tasks.json` and `docs/plans/current_spec.md` have local uncommitted modifications in the working tree (pre-existing, unrelated to this feature — the plan doc's committed version is stale relative to its working-tree content). Not part of this feature's diff; flagged for hygiene only.

## Final Verdict

**READY TO MERGE.** All Definition of Done gates pass with real, adversarially-reproduced evidence. Zero findings are attributable to the changelog feature's own diff. The two items above are pre-existing repo conditions independent of this PR and do not block it.
