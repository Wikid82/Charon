# "What's New" Changelog Popup — Design

Status: Approved
Date: 2026-07-28

## Summary

A per-user "What's New" modal that appears on first login after an app
update, summarizing new features, fixes, and other changes since the user
last saw the app. Content is generated automatically from conventional
commit messages at release build time and embedded in the binary, so no
runtime network calls or external dependencies are required. Users can
snooze, permanently dismiss, opt out entirely, and revisit the changelog
later from Settings.

## Goals

- Inform users of new features/fixes/other changes after an update, in
  plain, novice-friendly language.
- Never block or slow down login/app load.
- Give users control: snooze, dismiss-and-suppress-until-next-update, or
  opt out entirely (with an easy way back in).
- Zero new external dependencies; fits the self-hosted, single-binary
  deployment model.

## Non-Goals

- Per-role/permission-filtered changelog content (all users see the same
  entries).
- Rich text/markdown/images in changelog entries — plain text bullets only.
- Editorial rewriting of commit messages into marketing copy (entries are
  the literal commit subject lines, grouped by type).

## Data Model

`backend/internal/models/user.go` — two new columns on `User`:

```go
LastSeenVersion string `json:"last_seen_version" gorm:"default:''"`
ChangelogOptOut bool   `json:"changelog_opt_out" gorm:"default:false"`
```

- `LastSeenVersion`: highest app version the user has acknowledged. Empty
  string for pre-existing users before this migration runs (treated as
  "behind," so existing users see the current version's entries once).
- `ChangelogOptOut`: per-user opt-out flag, default `false`.
- **New user seeding**: at account creation (invite acceptance, admin-created
  user), `LastSeenVersion` is set to the currently running `version.Version`
  at creation time, so new users never see historical entries on their first
  login. Skip seeding (leave empty) if the running build is unversioned
  (`version.Version == "dev"`).
- Migration via GORM AutoMigrate in `internal/api/routes/routes.go`, per
  existing convention.

## Changelog Data Generation (build-time)

A new script, `scripts/generate-changelog.sh`, runs as a step in
`.github/workflows/release-goreleaser.yml` **immediately before** the
`goreleaser build` step, triggered on tag push (same trigger as the
existing release process).

Because goreleaser compiles from the exact tree at the pushed tag, the
generated data cannot be committed back to `main` either before tagging
(CI would be pushing to main, which the release process explicitly avoids
per `VERSION.md`) or after (the tagged commit's tree would never contain
it). Instead, the script **fully regenerates the cumulative file from
scratch on every release build**, sourced entirely from git history, which
is deterministic and requires no commit-back step:

- Lists all annotated release tags in the repo (`git tag -l 'v*' --sort=v:refname`).
- For each tag, walks `git log <prev-tag>..<tag> --pretty=%s` (subject
  lines only) and categorizes each commit by conventional-commit prefix:
  - `feat:` → `features`
  - `fix:` → `fixes`
  - everything else (`chore:`, `refactor:`, `docs:`, `test:`, `ci:`,
    `perf:`, unprefixed) → `other`
- Strips the prefix and leading whitespace for display text.
- Writes the full array to `backend/internal/changelog/data/changelog.json`
  in the CI build workspace (an array of per-version objects, see below),
  immediately before `go build`/`goreleaser build` runs, so `go:embed`
  picks up the freshly generated file at compile time:

```json
{
  "version": "1.5.0",
  "date": "2026-07-28",
  "features": ["Add TLS certificate management"],
  "fixes": ["Correct proxy timeout handling"],
  "other": ["Bump baseline-browser-mapping deps"]
}
```

- A **placeholder `changelog.json` containing `[]` is committed to the
  repo** (required so `go:embed` has something to embed for local/dev
  builds and non-release CI like PR checks, which never run the generator
  script). Local `go run`/dev Docker builds therefore always embed the
  empty placeholder — harmless, since dev builds already skip the
  changelog check entirely via `version.Version == "dev"` (see Edge
  Cases). Only the tagged release build in CI overwrites it in-workspace
  with real data, and that overwrite is never committed anywhere.
- `internal/changelog/changelog.go` embeds the file via `go:embed
  data/changelog.json` and parses it at package init. No runtime file I/O,
  no network call, works fully offline.

## Backend API

New package `backend/internal/changelog`:

- `GetEntriesSince(lastSeen string) []Entry` — filters the embedded data to
  entries where `semver.Compare("v"+entry.Version, "v"+lastSeen) > 0`, using
  `golang.org/x/mod/semver` (already a transitive dependency — no new
  package added). Returns entries sorted newest-first.
- `GetAllEntries() []Entry` — full history, used by the manual "revisit"
  action.

New routes (`internal/api/routes/routes.go`), authenticated, under
`/api/v1/changelog`:

- `GET /api/v1/changelog/status` →
  ```json
  { "show_changelog": true, "versions": [ /* Entry[] */ ] }
  ```
  `show_changelog` is `false` when `ChangelogOptOut` is true, when the
  running build is unversioned (`dev`), or when there are no unseen
  entries.

- `GET /api/v1/changelog/all` → `{ "versions": [ /* Entry[] */ ] }` — full
  history, used by the "What's New" link in Settings to revisit past
  entries on demand, independent of `LastSeenVersion`.

- `POST /api/v1/changelog/ack` — body:
  ```json
  { "action": "dismiss_temporary" | "dismiss_permanent", "opt_out": false }
  ```
  - `dismiss_permanent`: sets `LastSeenVersion` to the current running
    `version.Version`.
  - `dismiss_temporary`: no change to `LastSeenVersion` (modal reappears
    next login).
  - `opt_out: true` (valid with either action): sets `ChangelogOptOut =
    true`.

- `POST /api/v1/changelog/opt-in` — sets `ChangelogOptOut = false` without
  touching `LastSeenVersion`. Used by the Appearance Settings toggle to
  re-enable notifications.

## Frontend

### Modal — `frontend/src/components/dialogs/WhatsNewModal.tsx`

- Mounted from the main app layout shell (post-auth), fetches
  `/changelog/status` **after** initial layout render (non-blocking — a
  slow or failed request never delays app usability; on fetch error the
  modal simply doesn't show).
- Renders only when `show_changelog` is true.
- Content: one section per version (`v1.5.0`, `v1.4.2`, …), newest first.
  Each section shows:
  - **✨ New Features** and **🐛 Fixes** groups, expanded by default
    (omitted if empty).
  - **🔧 Other** group (chores/refactors/deps/etc.), collapsed by default
    behind a "Show maintenance details" disclosure — keeps focus on
    user-relevant changes per the project's novice-user-first framing.
- Footer:
  - Checkbox: *"Don't show me update notifications"*.
  - **"Remind Me Next Time"** (secondary button) → `ack` with
    `dismiss_temporary`.
  - **"Got It, Thanks"** (primary button) → `ack` with `dismiss_permanent`.
  - X icon / backdrop click → same effect as "Remind Me Next Time".
  - In all three dismiss paths, the checkbox's checked state is sent as
    `opt_out` — it's an independent, explicit user action honored
    regardless of which close method was used.

### Settings — `frontend/src/pages/AppearanceSettings.tsx`

- New toggle: *"Show 'What's New' after updates"*, bound to
  `!user.changelog_opt_out`. Flipping it on calls `POST
  /changelog/opt-in`; flipping it off calls `ack` with
  `{ action: "dismiss_temporary", opt_out: true }`.
- New link/button next to the toggle: **"What's New"** — opens the same
  modal component in "browse" mode, fetching `/changelog/all` instead of
  `/status`, with only a single "Close" action (no ack calls, since this is
  a voluntary revisit, not a real dismissal).

## Local & Pre-Merge Testing

The dev-build skip (`version.Version == "dev"`) means the modal never
appears on a plain `go run ./cmd/api` by default — that's intentional for
real dev usage, but it would also make the feature impossible to exercise
manually before merging. Two additions close that gap, both mirroring
existing conventions already in the codebase:

- **Test seam for automated tests**: `internal/changelog`'s service takes
  an injectable current-version resolver, the same pattern
  `UpdateService.SetCurrentVersion` already uses for its own tests. Go
  unit/integration tests set an arbitrary version directly — no real build
  or git tags required.
- **Dev-only version override for manual QA**: a `CHARON_CHANGELOG_VERSION`
  env var, honored only when `Environment != "production"` (reusing the
  existing `CHARON_ENV` config convention in `internal/config`, which
  already defaults to `"development"`). When set, it overrides the
  effective "current version" used by the `/changelog/status` check,
  independent of the real `version.Version` build var.

**Manual pre-merge workflow**:
1. Populate `backend/internal/changelog/data/changelog.json` locally —
   either run `scripts/generate-changelog.sh` against real local git tags,
   or temporarily edit the file with fixture entries for pure UI
   iteration (never commit the edited fixture over the `[]` placeholder).
2. Run the backend with `CHARON_CHANGELOG_VERSION=1.5.0` (or any version
   newer than your test user's `last_seen_version`) and default
   `CHARON_ENV=development`.
3. Set a test user's `last_seen_version` below that value (directly in the
   dev SQLite DB, or a small seed step) and log in — the modal should
   appear.

Playwright E2E specs use the same `CHARON_CHANGELOG_VERSION` override plus
committed fixture changelog data (not real git tag history), so the tests
stay deterministic and independent of the repo's actual release tags.

## Edge Cases

- **Unversioned/dev builds** (`version.Version == "dev"`): `/status`
  short-circuits to `show_changelog: false`. Prevents meaningless
  comparisons against a non-semver string and avoids showing every commit
  as "new" on every local restart.
- **Pre-existing users at migration time**: empty `LastSeenVersion` is
  treated as "behind everything," so they'll see the current version's
  entries once on their next login post-upgrade — acceptable since it's a
  one-time event and mirrors the "keep users informed" goal.
- **Empty changelog data** (e.g., first release after this feature ships,
  or a version with zero categorizable commits): `/status` returns
  `show_changelog: false` rather than an empty modal.
- **Concurrent multi-user logins**: no shared state beyond the per-user
  columns; no race conditions introduced.

## Testing

- **Backend (Go, TDD)**: unit tests for `changelog.GetEntriesSince` /
  `GetAllEntries` (semver boundary cases, empty data, dev-build
  short-circuit), the `/status`, `/ack`, `/opt-in` handlers, and the User
  model migration/seeding logic.
- **Frontend (Vitest)**: modal rendering per response shape, all three
  dismiss paths and their API payloads, checkbox/opt-out interaction,
  collapsed "Other" disclosure, Appearance Settings toggle and "What's
  New" revisit link.
- **E2E (Playwright)**: version bump → modal appears on login → each
  button's effect verified on a subsequent login → opt-out via checkbox
  suppresses the modal → Settings toggle re-enables it → "What's New" link
  opens browse-mode modal.

## Commit Slicing Strategy (for the implementation plan)

1. E2E specs for the new flow (`test.fixme` until implemented).
2. Foundation: `Entry` type/contract shared assumptions, `changelog.json`
   schema, `generate-changelog.sh` script + release workflow wiring
   (no runtime behavior change).
3. Backend: User model migration + seeding, `internal/changelog` package,
   API routes, unit tests.
4. Frontend: `WhatsNewModal`, layout integration, Appearance Settings
   toggle + revisit link, API client, unit tests.
5. Hardening: enable the E2E specs, docs update (`docs/features.md`).
