# Security Category for Changelog + Security Docs Page — Design

Status: Approved
Date: 2026-07-29

## Summary

Adds a dedicated "Security" category to the What's New changelog pipeline
(both the in-app modal and the underlying commit-categorization script),
plus a new user-facing `docs/features/security.md` page describing
Charon's overall security posture. Establishes a `(security)` commit
scope convention with a documented "vague by default" writing rule to
avoid the changelog itself becoming a roadmap of exploitable weaknesses
in un-upgraded self-hosted instances.

## Goals

- Give users a clear, dedicated place to see what's being done to keep
  them secure, both per-release (changelog) and in general (docs page).
- Avoid the OPSEC risk of a public, detailed vulnerability changelog for
  self-hosted software where many instances lag behind on updates.
- Reuse the existing changelog generation/rendering pipeline rather than
  building a parallel system.

## Non-Goals

- CVE-level detail or a security advisory database — that's a much larger
  feature or a job for GitHub Security Advisories, not this doc/category.
- Automated vagueness enforcement (e.g. NLP-based scrubbing) — this relies
  on a documented commit-writing convention, not tooling.

## Commit Convention

New scope on the existing `feat`/`fix` types (not a new top-level type):
`feat(security): <subject>` and `fix(security): <subject>`.

**Writing rule** (the "vague by default" mechanism): the subject line
must describe the *category* of issue and the mitigation in general
terms, and must never reveal the specific vulnerability class, attack
vector, or exact vulnerable code path. Good: "Harden input validation in
the API layer." Bad: "Fix SQL injection in host search filter." This is
enforced by convention/review, not tooling — the generator displays the
subject verbatim, same as every other category, so safety depends on how
the commit is written in the first place.

This scope is reserved for genuinely security-relevant changes (real
vulnerability fixes, new protective mechanisms) — not general bug fixes,
to avoid diluting the category's signal.

## `generate-changelog.sh` Changes

Categorization order changes: check for a `(security)` scope on `feat`/
`fix` *before* falling through to the existing bare `feat:`/`fix:`/other
logic. A matched commit routes exclusively into a new `security` array —
not also duplicated into `features`/`fixes`.

Schema change — `security` entries are richer than the other three
categories (which stay plain string arrays, unchanged, to avoid touching
already-shipped/tested code):

```json
{
  "version": "1.6.0",
  "date": "2026-08-01",
  "features": ["..."],
  "fixes": ["..."],
  "other": ["..."],
  "security": [
    { "summary": "Harden input validation in the API layer", "sha": "a1b2c3d..." }
  ]
}
```

## Backend (`internal/changelog`)

`Entry` struct gains a `Security []SecurityEntry` field (`SecurityEntry{
Summary, SHA string }`, JSON tags `summary`/`sha`). No change to semver
comparison or filtering logic — `security` entries follow the same
per-version inclusion rules as the other three categories.

## Frontend (`WhatsNewModal.tsx`)

A 4th group, "🔒 Security", rendered **expanded by default** (unlike the
collapsed "Other" group) — security-relevant information shouldn't be
hidden behind a disclosure. Each item shows the summary text plus a small
"view commit" link (`https://github.com/Wikid82/Charon/commit/<sha>`,
`target="_blank"`, `rel="noopener noreferrer"`) for users who want to dig
deeper. Empty `security` arrays omit the group entirely, same pattern as
the existing empty-group handling.

## `docs/features/security.md`

New Docs Writer page, following the existing `docs/features/
notifications.md` pattern (linked briefly from `docs/features.md`).
Novice-friendly overview of Charon's security posture and philosophy in
general (encryption at rest, automatic HTTPS, CrowdSec integration,
forward-auth gateway, etc.) — **not** per-release specifics, that's the
modal's job. No implementation details, no jargon, per Docs Writer's
existing style guide.

## Docs/Agent File Updates

- `CLAUDE.md`'s Conventional Commits bullet: add the `(security)` scope
  and the vague-subject-line writing rule.
- `backend-dev.md`, `frontend-dev.md`, `devops.md`: told to use the scope
  for genuine security-relevant work, with an explicit caution against
  overusing it for visibility on non-security fixes.
- `docs-writer.md`, `qa-security.md`: no commit-writing changes needed
  (docs-writer doesn't author app commits; qa-security's existing
  `docs/security/` tracking is unaffected) — `docs-writer.md` does need
  a pointer to author the new `docs/features/security.md` page.

## Testing

- Backend: unit tests for the new `Security` field's JSON marshaling and
  semver filtering (reusing existing `changelog_test.go` patterns).
- Frontend: `WhatsNewModal.test.tsx` gets cases for the Security group
  rendering, the commit link, and the empty-array omission case.
- `generate-changelog.sh`: extend its existing smoke-test coverage to
  include `(security)`-scoped commits, verifying exclusive routing (not
  duplicated into `fixes`).
- No new E2E scenarios strictly required (the existing `whats-new-
  changelog.spec.ts` fixture can gain a security entry to exercise the
  new UI in passing), but per user instruction this can be bundled with
  the next E2E-touching commit rather than shipped separately, since CI
  won't run again until the branch is pushed regardless.

## Relationship to the Shard 4 CI Fix

Unrelated in cause, but bundled into the same pre-push batch of work per
user instruction. See the separate root-cause note added to `docs/plans/
current_spec.md` / the PR for the Shard 4 investigation — the changelog
E2E fixture-injection step that Playwright Dev ran manually during local
hardening was never wired into `.github/workflows/e2e-tests-split.yml`'s
shared "Prepare Application Image" job, so CI's E2E image always embeds
the `[]` placeholder and every `whats-new-changelog.spec.ts` scenario
that expects the modal fails identically across all three browsers in
Shard 4. Fix: add the fixture-injection step to that CI job, mirroring
the local `docker-rebuild-e2e` workflow (no revert needed — CI runners
are ephemeral).
