# Pre-Existing Follow-Ups: Trivy Suppression Expiry & E2E Firefox Flake Scope

**Discovery Date:** 2026-07-29
**Discovered During:** QA audit of the "What's New" changelog feature (`feat/changelog`, see `docs/reports/qa_report.md`)
**Status:** Open
**Priority:** Medium

Both items below are **pre-existing and unrelated to the changelog feature**
— they surfaced as a side effect of that QA pass, not because of anything
in the changelog diff. Full detail/evidence is in `docs/reports/qa_report.md`
(§6.3 and §1b); this doc is just enough to track and pick each one up
later.

## Issue 1: Stale `.trivyignore` Suppression for CVE-2026-32286 — RESOLVED 2026-07-29

**Status: Resolved.** Renewed, not removed — the CVE is still real and
still unfixed upstream, but the entry was stale documentation, not an
active CI failure.

Correction to the original framing above: Trivy's plain-text `.trivyignore`
format (as used by this repo, invoked via `aquasecurity/trivy-action` in
`docker-build.yml` with `trivyignores: '.trivyignore'`) does **not** parse
or enforce the `# exp: DATE` comment at all — that annotation is a
project-only human-review convention, not a Trivy feature. Verified this
directly: ran `trivy image --ignorefile .trivyignore` (same CLI version,
v0.72.0, pinned by the CI action) against an exported `charon:local` image
with `--show-suppressed`, and the finding was still reported as
`"Status": "ignored", "Source": ".trivyignore"` despite the lapsed date —
CI's Trivy gate was **not** actually flagging this on unrelated PRs. The
risk was stale documentation implying a review had happened that hadn't,
not an active gate failure.

Re-investigated the underlying CVE itself on 2026-07-29: `jackc/pgproto3`
remains archived (still v2.3.3, no new tags). Checked upstream `go.mod`
directly (not just release notes) for CrowdSec v1.7.8 (Charon's current
pin, and upstream's latest stable release) and v1.8.0-rc1 (latest
including pre-releases) — both still resolve
`github.com/jackc/pgx/v4 v4.18.3` → `github.com/jackc/pgproto3/v2 v2.3.3`.
No pgx/v5 migration has landed upstream. The original justification
(Charon defaults to SQLite; the vulnerable PostgreSQL wire-protocol path
isn't reached in a standard deployment) still holds.

Action taken: renewed the suppression rather than removing it or applying
a fix, since no upstream fix path exists yet. Updated `.trivyignore` and
the matching `.grype.yaml` entry with a fresh 2026-07-29 review note and a
new `exp:`/`expiry` of `2026-09-01` — aligned with the two sibling entries
covering the exact same underlying pgproto3/v2 bug under different
advisory IDs (`GHSA-jqcq-xjh3-6g23`, `GHSA-x6gf-mpr2-68h6`), which were
already extended to `2026-09-01` on 2026-06-02, so all three now review
together going forward instead of drifting apart. Also updated
`SECURITY.md`'s `CVE-2026-32286` entry with the same re-verification note.
Re-ran the Trivy scan after the change to confirm the finding is still
cleanly suppressed under the renewed, non-expired entry.

## Issue 2: E2E Firefox Navigation-Race Flakiness Broader Than Known 3 Files

**Status: Partially fixed.** See below for what landed and what's still open.

Commit `7503c01a` fixed a Firefox/Playwright navigation-commit race by
adding a `reloadTolerant()` sibling to the pre-existing `gotoTolerant()`
helper in `tests/utils/wait-helpers.ts`, and applied both to
`user-lifecycle.spec.ts`'s `navigateToLogin()`. `gotoTolerant()` itself
already existed and was already in use in several files
(`user-management.spec.ts`, `long-running-operations.spec.ts`,
`wait-helpers.spec.ts`'s own tests). The gap: `theme-banner-userthemes.spec.ts`'s
`goToAppearance()` helper (and its sibling `loginWithStoredState()`) still
called raw, unprotected `page.goto()` — never wrapped. A full-suite local
run during the QA pass (970 tests, `--project=firefox`) reproduced the same
`page.goto()` timeout signature non-deterministically across many more
files spanning unrelated feature areas (a11y, auth, certificates, CrowdSec,
DNS providers, proxy groups/hosts, uptime monitoring, and others — see
`docs/reports/qa_report.md` §1b for the full file list and the re-run that
confirmed it's load-dependent, not tied to specific code paths).

### Fixed (this pass)

- `tests/theme-banner-userthemes.spec.ts`: `loginWithStoredState()` and the
  inline `page.goto('/')` in the banner-persistence test now use
  `gotoTolerant()`. `goToAppearance()` now uses `gotoTolerant()` for the
  first navigation to `/settings/appearance`, but switches to
  `reloadTolerant()` when the page is already on that URL — several tests
  in this file call `goToAppearance()` a second time mid-test (e.g. to
  refresh the theme list after creating a theme via the API), which is the
  same "second goto() to a URL you're already on" race that `7503c01a`
  originally fixed for `navigateToLogin()`'s reload, just manifesting here
  via a helper instead of an inline call. Confirmed reproducible under
  2-worker load before the fix (`deleting a user theme removes it from the
  list` timed out waiting on the appearance page's radiogroup after a
  same-URL `goto()`); 32/32 passed consistently across repeated runs after.
- `tests/theme.spec.ts`: near-identical `loginWithStoredState()` /
  `goToAppearance()` helpers (same file family, same underlying pattern,
  not yet reported as flaky but structurally the same risk) updated the
  same way. Its `goToAppearance()` is only ever called once per test here,
  so no same-URL/reload case applies.
- Validated: `theme-banner-userthemes.spec.ts`, `user-management.spec.ts`,
  and `wait-helpers.spec.ts` together — all pass except the pre-existing,
  separately-tracked `reloadTolerant` gap below. `theme.spec.ts` +
  `theme-banner-userthemes.spec.ts` together — 32/32, repeatable.

### Still open

- `wait-helpers.spec.ts:386` (`reloadTolerant › should swallow a timeout
  instead of throwing`) fails on a race variant `reloadTolerant()` doesn't
  yet cover: Firefox can reject a same-URL `reload()` with
  `NS_BINDING_ABORTED`, a message `isExpectedNavigationRace()` in
  `wait-helpers.ts` doesn't currently match (it only checks for
  `'Timeout'`, `'interrupted by another navigation'`, and
  `'net::ERR_ABORTED'`). Needs its own fix to `isExpectedNavigationRace()`;
  out of scope for this pass.
- `user-management.spec.ts`'s `should show error for regular user access`
  (already using `gotoTolerant()`) failed once in a full 184-test
  `tests/settings/` run under 2-worker contention, but passed cleanly in an
  isolated run — this is an authorization/rendering `expect.poll` race
  unrelated to `page.goto()`/`page.reload()` at all, not something this
  pass's fix touches.
- The broader sweep (a11y, auth, certificates, CrowdSec, DNS providers,
  proxy groups/hosts, uptime monitoring, etc. named in the original QA
  report) is still untouched — this pass covered the two confirmed
  `goToAppearance()`-family files only, per explicit scope. Remaining
  `page.goto()` call sites (mostly one-off, per-test navigations rather
  than shared helpers reused across many tests) are lower-leverage and
  higher-risk to sweep in bulk; still open for a follow-up.
- `whats-new-changelog.spec.ts` had 7 failures in the same
  `tests/settings/` run (`waitForModal` never finding the "What's New"
  dialog) — unrelated to navigation races and to this fix (the file was
  not touched here); tracked separately as a changelog-feature issue, not
  part of this E2E-flake follow-up.

## Issue 3: "Security Enforcement" CI Job Never Actually Runs `security-enforcement/`

**Status:** Open. **Discovered:** 2026-07-30, while validating the
`createUserViaApi`/changelog-suppression fix below — unrelated to that fix,
noted in passing and tracked here per the same "pre-existing, not part of
this pass" convention as Issues 1-2.

`.github/workflows/e2e-tests-split.yml`'s "Security Enforcement" job invokes
`npx playwright test --project=chromium tests/security-enforcement/
tests/security/ tests/integration/multi-feature-workflows.spec.ts`. But
`playwright.config.js`'s `chromium` project has `testIgnore:
['**/security-enforcement/**', '**/tests/security/**']` (mirrored on the
`firefox`/`webkit` projects) — those two directories are only ever collected
by the dedicated `security-tests` project (Chromium-only, sequential,
`workers: 1`, brings up CrowdSec/WAF via `security-shard-setup`), which the
CI job never selects. Net effect: the job's `--project=chromium` invocation
silently matches zero tests in `tests/security-enforcement/` and
`tests/security/` and only ever actually runs
`multi-feature-workflows.spec.ts`, without erroring or reporting the other
two paths as skipped/missing. Confirmed locally: `npx playwright test
tests/security-enforcement/multi-component-security-workflows.spec.ts
--project=chromium --list` → "Total: 0 tests in 0 files", while
`--project=security-tests` collects and runs them normally.

Not fixed here — out of scope for the changelog-suppression fix that
surfaced it. Likely fix shape: either change the CI job to
`--project=security-tests` (and drop `tests/security/`/
`multi-feature-workflows.spec.ts` from that invocation if they need the
plain `chromium` project instead, since `security-tests` and `chromium`
have different `use:` blocks/security-module setup), or split the job into
two invocations — one per project.

## References

- QA Report: `docs/reports/qa_report.md` (§1b "The other 27 failures", §6.3)
- Prior partial fix: commit `7503c01a`
- This pass's fix: commit (see git log for the `fix(e2e): extend gotoTolerant()...` commit on `feat/changelog`)
- Issue 3: `.github/workflows/e2e-tests-split.yml` "Security Enforcement" job vs. `playwright.config.js` project `testIgnore`/`testMatch` config
