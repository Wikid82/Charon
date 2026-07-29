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

## Issue 1: Stale `.trivyignore` Suppression for CVE-2026-32286

The `.trivyignore` entry suppressing `CVE-2026-32286` (pgproto3 DoS via
negative DataRow field length, bundled in the `crowdsec`/`cscli` binaries)
carries an `# exp: 2026-07-09` review-by annotation that has now lapsed —
expired 20 days ago. Trivy honors that expiry, so with the repo's real
`.trivyignore` applied it still surfaces as 2 HIGH findings, same as running
with no suppression file at all. This is already a documented, accepted
risk in `SECURITY.md` (Charon's default SQLite deployment doesn't reach the
vulnerable PostgreSQL code path), so the fix is almost certainly just a
renewal, not new remediation work — but it needs an owner to re-confirm the
risk assessment still holds and bump both the `.trivyignore` `exp:` date
and `SECURITY.md`'s "Review by" date. Until renewed, CI's Trivy gate
(`docker-build.yml`) will flag this on any PR, not just changelog-related
ones.

## Issue 2: E2E Firefox Navigation-Race Flakiness Broader Than Known 3 Files

Commit `7503c01a` fixed a Firefox/Playwright navigation-commit race by
routing `reload()` calls through a `gotoTolerant()`-style helper, but only
for `reload()` — `page.goto()` calls were left on the direct API and remain
exposed to the same underlying timing issue. The fix's scope was believed
to cover 3 previously-known flaky files
(`user-management.spec.ts`, `theme-banner-userthemes.spec.ts`,
`wait-helpers.spec.ts`). A full-suite local run during this QA pass
(970 tests, `--project=firefox`) reproduced the same `page.goto()`
90-second-timeout signature non-deterministically across many more files
spanning unrelated feature areas (a11y, auth, certificates, CrowdSec,
DNS providers, proxy groups/hosts, uptime monitoring, and others — see
`docs/reports/qa_report.md` §1b for the full file list and the re-run that
confirmed it's load-dependent, not tied to specific code paths). Follow-up:
extend the `goto()` call sites to use the same tolerant-navigation helper
already proven out for `reload()`.

## References

- QA Report: `docs/reports/qa_report.md` (§1b "The other 27 failures", §6.3)
- Prior partial fix: commit `7503c01a`
