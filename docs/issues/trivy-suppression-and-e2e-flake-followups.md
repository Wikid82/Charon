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
