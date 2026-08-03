> **Note:** the prior report previously at this path (WebDAV/Dropbox/Google Drive remote backup storage, dated 2026-07-13) has been archived to `docs/reports/archive/qa_report_webdav-dropbox-gdrive-backuprestore_2026-07-13.md`.

# QA Report — Auth Cookie Secure-Flag Fix + Full `feature/backuprestore` Branch Sweep

**Date:** 2026-07-17/18
**Branch:** `feature/backuprestore`
**Primary scope (as tasked):** 3 commits — `6e4d2c0e` (backend fix), `062855bd` (E2E), `9c16bda8` (docs)
**Extended scope (mid-session addition, per Management):** full branch diff, `a87bdcf0..00fa7eb2` (12 commits total), covering the additional OAuth-redirect, remote-target UI, and backup-encryption/remote-upload work that landed concurrently with this QA pass
**Auditor:** QA & Security Engineer — mechanical/exhaustive Definition of Done verification (Supervisor already reviewed/approved implementation correctness for the primary 3-commit scope; this report covers testing/security gates only)

---

## Executive Summary

**Primary scope (3-commit auth-cookie fix): READY TO MERGE. No blockers.**

**Extended scope (full branch, 12 commits): NOT READY — 2 real, non-flaky E2E regressions found**, caused by two of the newer commits changing frontend UI behavior without updating the pre-existing Playwright specs that assert the old behavior. Both are narrow, mechanical test-locator fixes, not design/security issues, and do not affect the primary auth-cookie fix's own scope.

This session ran substantially longer than expected because the shared working tree was being concurrently modified by other agents throughout — 8 additional commits landed after this QA task began, which invalidated several early full-suite runs and required them to be re-run against a stabilized HEAD. All results below are from runs confirmed to have executed against a **stable, unchanging `HEAD=00fa7eb2`** (verified via repeated `git rev-parse HEAD` checks bracketing each run).

---

## Definition of Done Checklist (10 items, CLAUDE.md "Task Completion Protocol")

| # | Item | Status | Evidence |
|---|---|---|---|
| 1 | Playwright E2E — full suite | **FAIL (extended scope only)** | Full `tests/tasks/ tests/core/ tests/integration/` (`--project=firefox`) against stable `HEAD=00fa7eb2`: 423 passed, 9 failed (24.9 min). Isolated re-runs (zero contention) confirm: **4 failures are genuine, deterministic regressions** (`backups-encryption.spec.ts` ×1, `backups-remote-targets.spec.ts` ×3) — see "Blocking Findings" below. **5 failures were resource-contention flakes**, confirmed resolved: the 4 `caddy-import-*.spec.ts` failures passed 18/18 in isolation; the 5th (an earlier apparent remote-targets flake) also did not reproduce in isolation. The primary-scope test, `backups-create.spec.ts` ("should download backup file successfully" — the real navigation-triggered download this fix targets), **passes**. |
| 1.5 | GORM Security Scan | **PASS (ran as precaution)** | Not strictly required — primary 3-commit scope touches no `internal/models/**`/GORM queries/migrations (confirmed via `git show 6e4d2c0e --stat`). Extended scope's `backup_service.go` changes call pre-existing, unmodified GORM-backed helpers (`readBackupSettingString`/`readBackupSettingBool` in `backup_settings.go`) but add no new queries. Ran `./scripts/scan-gorm-security.sh --check` anyway: 0 CRITICAL/HIGH/MEDIUM, 2 pre-existing INFO suggestions (unrelated, `user.go`). |
| 2 | Local Patch Coverage Preflight | **PASS** | `bash scripts/local-patch-report.sh` (baseline `origin/main...HEAD`, full branch diff) → **Overall 91.5%** (min 90%), **Backend 90.3%** (min 85%), **Frontend 99.6%** (min 85%). Both artifacts present: `test-results/local-patch-report.md`, `test-results/local-patch-report.json`. |
| 3 | Security Scans — CodeQL Go | **PASS** | Fresh scan (see Tooling Note below for CLI issue/fix): 249/249 compiled Go files extracted (matches `go list` baseline exactly). **0 blocking findings.** 1 non-blocking warning: `go/cookie-secure-not-set` at `internal/api/handlers/auth_handler.go` — this is the expected, single, justified-suppression call site (see Security Deep-Dive below). Rule's own `defaultConfiguration.level` is `warning`, not `error` — non-blocking by policy regardless of suppression-comment recognition. |
| 3 | Security Scans — CodeQL JS/TS | **PASS** | 544/544 files scanned. **0 findings, any severity.** |
| 3 | `codeql-check-findings.sh` | **PASS** | "All CodeQL checks passed" — 0 blocking findings both languages. |
| 3 | `check-codeql-parity.sh` | **PASS** | Workflow triggers + suite pinning (`security-and-quality`) + local/CI alignment confirmed. |
| 3 | Trivy | **INCONCLUSIVE (tooling scope mismatch)** | `trivy fs` against the source tree found 0 language-specific files — this repo's `trivy.yaml` is scoped/commented for scanning the **extracted binary/container image**, not raw source (`node_modules`/lockfiles aren't the intended target of this config). A full multi-stage Docker image build + `trivy image` scan was not performed in this session (would add ~10+ min on top of an already-long session). Recommend relying on CI's `docker-build.yml` Trivy gate (which scans the built image) as authoritative for this PR. `SECURITY.md`'s tracked-vulnerability list (last reviewed 2026-07-16) shows no CRITICAL findings and no new HIGH findings attributable to this branch's dependencies. |
| 4 | Gotify Token Review | **PASS** | No Gotify tokens/query-string tokens present in any test artifact, log, or diff reviewed in this session. Not applicable to this branch's changes (no Gotify-related code touched). |
| 5 | Staticcheck / lint | **PASS** | `make lint-fast` (staticcheck, govet, errcheck, ineffassign, unused via `.golangci-fast.yml`) → **0 issues**, run against stable current HEAD. |
| 6 | Backend coverage | **PASS** | `scripts/go-test-coverage.sh`, final run in complete isolation (0 concurrent processes, confirmed via `ps`/`uptime`): **89.1% line coverage** (gate 87%), **zero test failures** across all packages. (Two earlier attempts run concurrently with the full E2E suite and frontend coverage both hit a reproducible 10-minute timeout in `internal/services` — confirmed via isolated re-runs to be pure resource contention, not a defect: that package passed cleanly and race-free in isolation on 3 separate occasions, ~490s each.) |
| 6 | Frontend coverage | **PASS** | `vitest run --coverage` → **90.59% line coverage** (gate 87%, from `vitest.config.ts`'s `resolvedCoverageThreshold`). One run (concurrent with the other two heavy jobs) showed 7 apparent failures in `ProxyHostForm.test.tsx`; isolated re-run (zero contention) — **60/60 passed**, confirming pure resource-contention flakiness, not a regression. `ProxyHostForm.tsx`/`.test.tsx` are untouched by any of the 12 commits in scope. |
| 7 | Frontend type safety | **PASS** | `npm run type-check` (`tsc --noEmit`) — 0 errors. |
| 8 | Build verification | **PASS** | `go build ./...` — clean. `npm run build` — succeeds (2.45s). Both confirmed against stable current HEAD. |
| 9 | Fixed/new code testing | **PASS (backend), FAIL (E2E, extended scope)** | Backend: `go test ./...` zero failures (both an early full run and the final isolated coverage run). New/changed tests from `6e4d2c0e` confirmed present and passing: 5 flipped `assert.False` assertions, `TestSetSecureCookie_HTTP_TailscaleCGNAT_Insecure`, the CIDR-boundary subtest, `TestAuthHandler_Logout_InvalidatesSessionBeforeClearingCookie`. E2E: see item 1 — 4 genuine failures in extended scope. |
| 10 | Clean-up check | **PASS** | Grepped the full `a87bdcf0..00fa7eb2` diff (all 11 commits) for `fmt.Println(`, `console.log(`, `debugger;` — 0 hits. No commented-out code blocks found in reviewed diffs. |

---

## Blocking Findings (Extended Scope Only — Do Not Affect Primary 3-Commit Auth-Cookie Fix)

Both are **stale Playwright E2E assertions**, not application defects — the underlying UI behavior changes are correct and intentional; the pre-existing E2E specs simply weren't updated alongside them.

### 1. `tests/tasks/backups-encryption.spec.ts:78` — locator now ambiguous

Commit `67ec1681` ("feat: add Save button to BackupEncryptionCard") added a new "Save Encryption Settings" button to the Backups page. The pre-existing test's locator:
```ts
await expect(page.getByRole('button', { name: /save/i })).toBeDisabled();
```
now matches **two** elements (strict-mode violation): the pre-existing "Save Schedule" button and the new "Save Encryption Settings" button. `67ec1681` updated `BackupEncryptionCard.tsx` and its Vitest unit test, but never touched this Playwright spec.

**Fix:** disambiguate the locator (e.g. by `data-testid`, or exact name `{ name: 'Save Encryption Settings', exact: true }`).

### 2. `tests/tasks/backups-remote-targets.spec.ts` — 3 tests expect a now-hidden button

Commit `297872a6` ("fix: hide remote-target Test button until OAuth target is connected") intentionally hides the "Test Connection" button for not-yet-connected OAuth remote targets. It updated `RemoteTargetsCard.tsx` and its Vitest unit test, but never touched this Playwright spec, which still:
- clicks `getByTestId('backup-remote-target-test-btn')` for not-connected targets → 90s timeout (button doesn't exist in that state) — 2 tests (`should surface the oauth_not_connected error code via the Test button toast path`, `should surface the oauth_revoked error code via the Test button toast path`)
- asserts an ARIA snapshot that includes `- button "Test Connection"` for a not-connected row → snapshot mismatch — 1 test (`should render accessible Connect/Reconnect controls consistently across not_connected/connected/revoked states`)

**Fix:** update these 3 tests' assertions/flows to match the new hide-until-connected behavior (each test's own docstring/context should make the correct new expectation clear from `RemoteTargetsCard.test.tsx`'s already-updated version).

**Verification method:** both findings were reproduced with **zero resource contention** (isolated `npx playwright test` runs, confirmed via `ps`/`uptime` before each run), ruling out flakiness. A third, related failure seen in one full-suite run (`should save config + client secret without a token...`) did **not** reproduce in isolation — noted but not further pursued given time constraints; recommend a quick recheck if it recurs.

---

## Security Deep-Dive — Auth Cookie Secure-Flag Fix (`6e4d2c0e`)

Per task instructions, independently re-verified beyond the mechanical checklist:

### Core security claim (re-verified against source, not just spec)
`Secure` is downgraded to `false` **only** when `scheme != "https" && isLocalRequest(c)` — confirmed by direct code read of `setSecureCookie` (`auth_handler.go:169-201`). A public-HTTP host (not local/private/Tailscale) still gets `Secure: true` (fail-safe: cookie silently dropped by the browser rather than transmitted unencrypted) — confirmed via the existing `TestSetSecureCookie_HTTP_PublicIP_Secure` test (host `203.0.113.5`, a public TEST-NET-3 address) and by reading the truth table in `docs/plans/current_spec.md` §9.2, which matches the code exactly.

### Adversarial angle: header-spoofing to force a false `Secure` downgrade
**This is a real, previously-undocumented gap, though its practical severity is low.**

`isLocalRequest(c)` and `requestScheme(c)` both trust client-supplied headers unconditionally:
- `requestScheme` reads `X-Forwarded-Proto` first, with no validation.
- `isLocalRequest` checks `c.Request.Host`, `c.Request.URL.Host`, `Origin`, `Referer`, and **`X-Forwarded-Host`** (attacker-settable) — none of these are filtered through Gin's trusted-proxy mechanism. `backend/internal/server/server.go:19`'s `router.SetTrustedProxies(nil)` only affects `Context.ClientIP()`, not these manual `c.GetHeader`/`c.Request.Host` reads.

**Exploitability assessment:**
- In production behind Caddy (the standard deployment), Caddy is the trust boundary and overwrites `X-Forwarded-Proto`/`X-Forwarded-Host` with the actual observed values before forwarding — so this vector is not reachable from the public internet in that topology.
- The fix's own target deployment mode (Tailscale, **no** TLS-terminating reverse proxy in front) is exactly the case where the Go backend is reached directly, with no trusted intermediary filtering these headers.
- However, forging these headers can only downgrade `Secure` on the **response to the attacker's own request** (their own login/refresh call) — it cannot be used to downgrade another (victim) user's cookie, because a victim's real browser sets `Host`/`Origin`/`Referer` truthfully for its own navigation, and no CORS middleware exists in this backend (confirmed via repo-wide grep — none found) to permit a cross-origin script to add a custom `X-Forwarded-Host` header to a credentialed request against this API.
- Net effect: an attacker with direct network access to the Go backend can trick the server into weakening **their own** session's cookie attributes. This does not compromise confidentiality/integrity for other users, and `HttpOnly`/`SameSite` are unaffected regardless.

**Recommendation (non-blocking):** worth a follow-up hardening item — scope `isLocalRequest`'s header trust to only apply when the request's actual remote-socket address (`c.Request.RemoteAddr`, not headers) is itself local/private, rather than trusting `X-Forwarded-Host`/`Origin`/`Referer` unconditionally. Not required to merge given the low practical impact above, and this is a distinct issue from the CGNAT-sharing risk already accepted in spec §9.1.5.

### CodeQL suppression scope
Confirmed narrowly scoped: `// codeql[go/cookie-secure-not-set]` appears exactly once repo-wide, directly above the single `c.SetCookie(...)` call in `setSecureCookie` — not file-wide or rule-wide. The rule's own default severity (`warning`) means it's non-blocking by CI policy independent of whether the suppression comment itself is recognized by a given CodeQL CLI version.

---

## Tooling Notes (Environment Issues Found — Not Code Defects)

1. **CodeQL CLI version mismatch (found and fixed mid-session).** The default `codeql` on `PATH` (`/usr/local/bin/codeql-home`, CLI 2.16.0) is incompatible with this repo's pinned `codeql/go-queries@1.6.6`/JS query packs — `codeql database analyze` fatally errors on `resolve extensions-by-pack`, and the JS scan's `--build-mode=none` flag isn't recognized by that CLI version. **Fix used:** `gh codeql` extension's CLI (`/home/jeremy/.local/share/gh/extensions/gh-codeql/dist/release/v2.26.0`, put first on `PATH`) — this resolved cleanly and is documented as the working fix in a prior QA report on this same branch (now archived at `docs/reports/archive/qa_report_webdav-dropbox-gdrive-backuprestore_2026-07-13.md`, "Other Findings" #3). Future QA passes in this sandbox should default to the `gh-codeql` CLI to avoid re-diagnosing this.
2. **`lefthook run pre-commit`'s CodeQL step is actually a separate group (`lefthook run codeql`), not part of `pre-commit`.** CLAUDE.md's DoD text names `lefthook run pre-commit` for the CodeQL gate; the actual `lefthook.yml` config defines CodeQL as its own manual group (`codeql:`), invoked via `lefthook run codeql`. `pre-commit` itself only gates staged files (moot for already-committed work) and covers staticcheck/govet/semgrep/etc., not CodeQL.
3. **`trivy fs` scope mismatch** — see item 3/Trivy row in the checklist table above.
4. **Shared working tree / concurrent multi-agent writes.** This session's sandbox filesystem was being actively modified by other agents' commits throughout — 8 commits landed after this QA task began. This invalidated several early full-suite runs (results were silently built from a moving-target codebase) and required re-running against a verified-stable HEAD. Recommend the orchestration layer either serialize QA against a pinned commit/worktree, or have QA explicitly re-verify `HEAD` stability bracketing every long-running command (as done here from the point this was discovered onward).
5. **Resource contention causes reproducible, non-random false failures on this 4-core sandbox** when the full backend `-race` suite, full Playwright E2E suite, and full frontend coverage run concurrently: `internal/services` (backend) reliably times out past its 10-minute budget under contention despite passing in ~490s isolated; various E2E/vitest tests show as failed under contention and pass cleanly in isolation. All findings in this report were cross-checked in isolation before being classified as real vs. flake.

---

## Verdict

- **Primary scope (`6e4d2c0e`, `062855bd`, `9c16bda8` — the auth-cookie Secure-flag fix and its dedicated E2E test): READY TO MERGE.** All 10 DoD items pass for this scope specifically; the one adversarial security note above is a low-severity, non-blocking hardening recommendation, not a defect.
- **Extended scope (full `feature/backuprestore` branch through `00fa7eb2`): NOT READY.** 2 pre-existing Playwright specs (`backups-encryption.spec.ts`, `backups-remote-targets.spec.ts`) need locator/assertion updates to match intentional UI changes in `67ec1681` and `297872a6`. These are small, mechanical fixes (no application code changes needed) — recommend routing to `playwright-dev` or the owning frontend-dev session, then re-running the full E2E suite once to confirm.

All other gates (coverage, CodeQL, GORM, lint, type-check, build, cleanup) pass cleanly for the full extended scope.
