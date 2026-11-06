# What's New Pipeline Repair and SemVer Build-Metadata Versions

Status: REVISED after the second supervisor review (APPROVE WITH CHANGES: R1-R10 and corrections a-h
folded in, each re-verified against the repo). Base: `development` @ e023f53d2. Date: 2026-11-06.

Workflow rule for every new or changed step in this PR: trigger context (`TRIGGER_*`, `github.*`) reaches
shell code only through `env:`. Never put `${{ }}` inside `run:`.

The user decided that the `+dev`/`+nightly` versioning is pointless while What's New is broken on shipped
images (#1865). This spec therefore covers **one branch and one PR** that does two things:

- fixes every What's New finding (bug-fix commits first), and
- then adds build-metadata versions.

The user explicitly overrides the usual "a high/critical fix ships as its own PR" rule.

Branch: `fix/whats-new-pipeline-and-build-versions` → PR into `development`. No worktrees.

Closes **#1865** and **#1867**. #1866 is closed as not planned (decision: non-release `main` builds keep reporting the manifest version; see §13).

Scope owner agents:

- `playwright-dev`: E2E pinning
- `backend-dev`: Go
- `frontend-dev`: modal, hook, update card
- `devops`: generator, workflows, smoke script, Makefile, Dockerfile
- `docs-writer`: docs
- `qa-security`: runs the built-image verification matrix (§8)

The previous lint-sweep spec is archived at `docs/plans/archive/lint_sweep_1743_spec.md`.

---

## 1. Goals

1. Every shipped image embeds correct, novice-readable changelog data. This covers release (`latest`),
   `development` (`:dev`), PR images and nightly.
2. The What's New modal behaves correctly end to end on those images:
   - It shows the right entries once.
   - It never shows a history dump or entries newer than the binary.
   - It never nags in a loop.
3. CI fails loudly when a build would ship without changelog data, and a post-build smoke check proves the
   behavior on the actual built image.
4. Non-release builds report `<last release>+<channel metadata>` so the What's New feature, the update check
   and backup validation can compare versions.

### Non-goals

- Translating the What's New strings. Locales are owned by Crowdin and about 96% of all keys are untranslated
  in most locales (`ja`/`nl`/`ar`: 2128 of 2215 strings equal English), so this is not What's New-specific.
  This PR only adds new keys to `en`.
- Orthrus agent *version semantics*. Only its wrong commit sha (#1867) is fixed here.
- Rewriting commit subjects into prose. The generator filters and cleans them, but it does not rewrite them.
- Scope filtering cannot catch **unscoped** test-only or internal fixes. Example: `fix: use a loadable
  configuration in the hub mirror contract test` in v0.46.5 still lands under "Fixes". How readable What's
  New is for novices still depends on commit-subject hygiene (the right scope, or `test:`/`chore:`
  prefixes).

---

## 2. Research: what the published images actually contain and do

### 2.1 Method (real artifacts only; E2E fixtures and live containers were not used as evidence)

- Rootless daemon (`DOCKER_HOST=unix:///run/user/1001/docker.sock`).
- Pulled the three published images used as evidence:

  | Tag | Image ID | Note |
  | --- | --- | --- |
  | `ghcr.io/wikid82/charon:latest` | d3ee3bfdb7b9 | Same image as `main-f2cb630`, the v0.46.5 release commit |
  | `:dev` | dd4fd38b67f2 | |
  | `:nightly` | 6d035bd42abb | |

- **No `vX.Y.Z` image tag exists.** None of the 791 GHCR tags and none of the Docker Hub tags are semver;
  see F12. The release image is therefore `latest` / `main-f2cb630`.
- Extracted `/app/charon` from each image via `docker create` + `docker cp`, and counted the embedded JSON
  entries.
- Ran a throwaway container for each image: `wn-research-*`, `127.0.0.1:18080-18083`, data under the
  scratchpad, `CHARON_ENV=production`.
- Ran the scripted flow `scratchpad/wn/exercise.sh` against each container:
  - `POST /setup` and login
  - `GET /changelog/status` and `/changelog/all`
  - `POST /users`
  - `POST /changelog/ack`: temp, permanent, permanent with opt-out, then an invalid action
  - `POST /changelog/opt-in`
  - unauthenticated `/changelog/status`
  - `/system/updates`
  - `users.last_seen_version` read from a read-only copy of SQLite after each step
- A fourth container ran the real nightly image with `CHARON_ENV=development CHARON_CHANGELOG_VERSION=0.46.4`
  to show how the real nightly data behaves once the build is versioned.
- All `wn-research-*` containers and the pulled images were removed afterwards. No live container was touched.

### 2.2 Results per image

| | `latest` (v0.46.5 release) | `:dev` | `:nightly` |
| --- | --- | --- | --- |
| OCI `version` / `revision` label | `0.46.5` / f2cb630d5 | `dev` / dcce994a5 | `nightly` / **f3271ae35 (= `main` HEAD)**, but the build checked out `nightly` (F7/#1867) |
| `/api/v1/health` `version` | `0.46.5` | `dev` | `nightly-f3271ae354631256169bf06a6194669274cddb30` |
| Embedded entries (binary scan) | **0** | **0** | **171** (0.1.0 … 0.46.5) |
| `GET /changelog/all` | `{"versions":[]}` (15 B) | `[]` | 171 entries, **818 KB** raw, 198 KB gzip |
| `last_seen_version` after setup / `POST /users` | `'0.46.5'` / `'0.46.5'` | `''` / `''` | `''` / `''` |
| `GET /changelog/status` (fresh admin) | `show_changelog:false`, 0 entries | false, 0 | false, 0 (unversioned gate) |
| Ack `dismiss_temporary` | `No change` | `No change` | `No change` |
| Ack `dismiss_permanent` | `Acknowledged` (re-writes `0.46.5`) | `No change` (guard) | `No change` (guard) |
| Ack `+opt_out` / `opt-in` | `changelog_opt_out` 1 / 0 ✔ | ✔ | ✔ |
| Ack invalid action | 400, body leaks the validator struct path `Key: 'AckRequest.Action' Error:…` | same | same |
| Unauthenticated status | 401 ✔ | 401 | 401 |
| `/system/updates` | `available:false` ✔ | **`available:true`** (false alarm) | **`available:true`** (false alarm) |
| Settings → "revisit What's New" | Opens an **empty dialog** (no empty state) | Empty dialog | 171 versions, thousands of DOM nodes |

**Conclusion:** no published release image has ever shown What's New. The only image with data (nightly)
hides it behind the unversioned gate.

### 2.3 Nightly data once the build is versioned (real image, override `0.46.4`)

```text
unseeded user ('')        -> show_changelog:true, 171 entries (0.46.5 … 0.1.0), 818 KB, blocking modal
dismiss_permanent         -> last_seen_version '0.46.4'
status after dismissal    -> show_changelog:true, ["0.46.5"]   <- newer than the running version: nag loop
```

Both hazards from the earlier spec (F3 ceiling, F8 first-view dump) are therefore **reproduced on real
published data**.

### 2.4 Pipeline history

- **8cefab1a4** (2026-07-29, `ci(changelog): generate changelog data for docker-build and nightly images`):
  - added `Generate Changelog Data` to `docker-build.yml`, and
  - set `fetch-depth: 0` in that checkout.

  At that point the build still ran in one job.
- **2026-08-10 merges** (a02f62197 "Propagate changes from main into development (#1233)", 1ef633c91 /
  9012d0801 "Merge branch 'development' into main") dropped the step. The `-m -S` search shows the string
  leaving the file in those merges and never returning.
- **aa7f21354** (2026-08-17) then documented nightly as "its one remaining real caller"
  (ARCHITECTURE.md:1994).
- Today `docker-build.yml` builds in `build-amd64` / `build-arm64` (`:429`, `:537`), which use **shallow
  checkouts**. Only `setup` has `fetch-depth: 0` (`:119-123`).
- **Releases that ever embedded real data: none.**
  - The releases published since 2026-08-10 shipped `[]`.
  - Between 2026-07-29 and 2026-08-10 the feature had not yet merged to `main` (#1196 merged 2026-08-03 into
    `development`).
  - Evidence: the current `latest` image (0.46.5) has 0 entries. No older semver image tags exist to inspect
    (F12).
- **Release timing race.**
  - The v0.46.5 release commit f2cb630d5 was pushed at about 11:07:15Z.
  - `docker-build.yml` (push, `main`) started at 11:07:19Z. `release-please.yml` started at the same time.
  - The GitHub Release (and thus tag `v0.46.5`) was **published at 11:09:48Z**, after the build's checkout.
  - So on a release build, the generator (which lists `git tag -l 'v*'`) would very likely **miss the very
    version being released**.

### 2.5 Generator quality (`scripts/generate-changelog.sh`, real output, 171 versions)

| Measure | Value | Cause |
| --- | --- | --- |
| `other` items | 6528, of which **2267 are `Merge …` subjects** | `git log "$prev..$tag"` (`:77-87`) has no `--no-merges` |
| Duplicate items | 322 in features/fixes, 803 in other | The same change is reachable via several commits (squash plus branch commits, promotion and propagation), and subjects differ only by `(#NNN)` |
| Internal scopes shown as user "Fixes" | `fix(deps)` 276, `fix(ci)` 161, `fix(e2e)` 63, `fix(tests)` 57, `fix(test)` 20, `fix(docs)` 17, `fix(workflow)` 7 … (whole-history counts) | Only `feat`/`fix` prefixes are checked, never the scope |
| Release / promotion chores in `other` | `chore(main): release 0.46.5 (#1802)` appears **twice**, plus `chore: promote nightly to main …` | Not filtered |
| Versions with no feature, fix or security entry | 32 of 171 | Expected. The modal handles this, but they add bulk |
| Example: v0.46.5 | 0 features, 22 fixes (including test-only fixes such as "use a loadable configuration in the hub mirror contract test"), 105 other | |
| Failure handling | `git log … 2>/dev/null`. Zero tags (shallow clone) still writes `[]` and **exits 0** | Silent. This is exactly how #1865 shipped unnoticed |
| `(security)` routing | 158 entries, all shas valid hex. Routed only to `security` ✔ | |
| Dates | Author date of the tagged commit ✔ | |
| Ordering | Oldest-first, then reversed. The backend re-sorts by semver ✔ | |

### 2.6 Consumer side, reviewed fresh

| Area | Finding | Evidence |
| --- | --- | --- |
| `GetEntriesSince` | No ceiling (F3); full-history dump for `''` users (F8) | `changelog.go:145-157`; §2.3 |
| Ack | `dismiss_permanent` writes the running version unconditionally. On a **downgrade** it *lowers* `last_seen_version`, so re-upgrading re-shows already-seen entries (F15) | `changelog_handler.go:102-104` |
| Ack error body | Returns `err.Error()` from the binder, which exposes internal struct and validator names (F16, low). The same pattern exists app-wide | `changelog_handler.go:88-91`; §2.2 |
| Security commit link | `href={GITHUB_COMMIT_URL + item.sha}` with no format check. The data is build-time and trusted, but neither the backend nor the frontend validates it (F17, low hardening) | `WhatsNewModal.tsx:20`, `:97-98`; `changelog.go:189-203` |
| React list keys | `key={feature}` / `key={fix}` / `key={item}`. Real data has 1125 duplicate strings, so React gets duplicate keys (console errors, possible dropped or reused rows) (F18) | `WhatsNewModal.tsx:71`, `:82`, `:118` |
| Browse ("revisit") mode | No loading, error or empty state. It opens a blank dialog on every release and dev image (F19). It renders all 171 versions at once (818 KB) (F20) | `WhatsNewModal.tsx:168-193`; §2.2 |
| Headings | Emoji inside heading text is read aloud by screen readers (F21, low). It is not only English: the translated `de`/`es`/`fr`/`zh` values keep the emoji (`"✨ Neue Funktionen"`, `"✨ Nuevas funciones"`, `"✨ Nouvelles fonctionnalités"`, `"✨ 新功能"`), and the other 23 locales copy the English with emoji. The E2E aria snapshot asserts the emoji text | `locales/*/translation.json` `whatsNew.newFeatures/fixes/security`; `tests/settings/whats-new-changelog.spec.ts-snapshots/…behind-the-current-version-1.aria.yml:5,9,14,17,23` |
| Toasts | Hard-coded English in mutation errors (F22, low) | `hooks/useChangelog.ts` `useAckChangelog`/`useOptInChangelog` `onError` |
| Passthrough role | All four routes reject it with 403 via `rejectPassthrough`. The status-mode modal treats the error as "hide" ✔ | `changelog_handler.go:34-36`; `WhatsNewModal.tsx:198` |
| Multi-user | Per-user columns, independent ✔ | |
| Payload | Gzip enabled ✔ (198 KB). Shrinks to roughly 40 KB once F10 cleanup is applied, measured by re-running `git log --no-merges` over v0.46.4..v0.46.5 (107 vs 127 subjects; feat/fix 20 unique) | §2.5 |
| Update checker | False "update available" on dev and nightly (C6 below); System Settings card contract drift (F2); `changelog_url` not validated (F9) | §2.2; `update_service.go:144-156`; `SystemSettings.tsx:37-42`, `:201-213` |

### 2.7 Test coverage, honestly

- Backend unit tests (`changelog_test.go`, `changelog_handler_test.go`) inject entries through
  `SetEntriesForTesting`. None loads the real embedded data or the generator output.
- E2E (`whats-new-changelog.spec.ts`) **only** passes because `e2e-tests-split.yml:204-208` copies
  `tests/fixtures/changelog-fixture.json` over the embedded file and sets `CHARON_CHANGELOG_VERSION=1.2.0`.
- **No test exercises `generate-changelog.sh`, the workflow step, or a built image's embedded data.** A
  #1865-class regression (data missing from shipped images) passes every existing test.
- The nightly "smoke test" (`nightly-build.yml:589-622`) polls `http://127.0.0.1:8080/health`. That route
  does not exist on the main server (only `emergency_server.go:148` has one). The request falls through to
  the SPA and returns **200 `text/html`**, as verified on the real nightly image. **The smoke test passes
  vacuously** (F13).

### 2.8 Version consumers (carried over from the earlier spec, still valid)

| Site | Change |
| --- | --- |
| `routes.go:484-489` changelog service and `CHARON_CHANGELOG_VERSION` override (non-production only) | Anchor, ceiling, cap; warn on an unversioned override |
| `user_handler.go:37-42` `seedLastSeenVersion` | Seed the canonical form. Keep reading `version.Version`, not the override (E2E relies on this) |
| `changelog_handler.go:102-104` Ack | Canonical anchor that never lowers the stored value (F15) |
| `health_handler.go:33`, `main.go:204` | None |
| `update_service.go:38`, `:144-156` | SemVer compare, `current_version`, URL validation |
| `backup_upload.go:58`, `backup_service.go:1087`, `:1149` | None. The longest new value is about 25 chars, under `size:50` |
| `backup_restore_safe.go:205-209` | **Advisory warning only**, never blocks a restore. Compare canonical forms and add a "newer archive" wording |
| Frontend `Layout.tsx:411`, `SystemSettings.tsx:637` | None |

---

## 3. Findings register

Every finding is **rolled into this PR** unless stated otherwise.

**Security-sensitive findings: none.** F9, F16 and F17 are low-severity hardening with no realistic exploit
path, so no private advisory candidate was identified. F16's app-wide pattern is tracked separately.

| ID | Finding | Sev | Evidence | Disposition |
| --- | --- | --- | --- | --- |
| F1 / #1865 | Release, dev and PR images embed `[]`; the generation step was lost in the 2026-08-10 merges | High (feature dead on every release since launch) | §2.2, §2.4 | Commit 6 |
| F10 | Generator output is unfit for novices: merges, duplicates, internal scopes under "Fixes", release chores | Medium | §2.5 | Commit 5 |
| F11 | Generator is silent on the case that matters. `set -euo pipefail` (`generate-changelog.sh:7`) means a failing `git log` *does* fail the script, though its stderr is hidden by `2>/dev/null` (`:87`). But **zero `v*` tags (a shallow clone) writes `[]` and exits 0**. Separately, the build misses the version being released (tag race) | High (root enabler of #1865) | §2.4, §2.5; `generate-changelog.sh:7`, `:65`, `:87` | Commits 5, 6 |
| F13 | Nightly smoke test hits SPA HTML and always passes; no build validates What's New | Medium | §2.7 | Commit 8 |
| F3 | No ceiling, so a nag loop | Medium | §2.3 | Commit 3 |
| F8 | History dump once data ships, in two forms. (1) Users with `last_seen_version ''` get all 171 entries / 818 KB (§2.3). (2) **Release users seeded long ago**: seeding exists since d17cff29a, first released in **v0.36.1** (`git tag --contains d17cff29a` gives 37 tags, v0.36.1 to v0.46.5). Those users have never acked, because no release shipped data. On the first data-bearing release they would get about 35 versions | High once data ships | §2.3; `user_handler.go:37-42` | Commit 3: cap **every** status response at the newest 3 |
| F15 | Ack lowers `last_seen_version` on downgrade | Low | `changelog_handler.go:102-104` | Commit 3 |
| F16 | Ack 400 body exposes binder internals | Low | §2.2 | Commit 3 (changelog handler only). **Not rolled in app-wide**: the same `err.Error()` pattern is in many handlers and is outside What's New, so file an issue |
| F17 | Security-entry sha not validated before building the link | Low | `WhatsNewModal.tsx:97-98` | Commits 3 (backend drops a non-hex sha) and 4 |
| F18 | Duplicate React keys in modal lists | Medium | `WhatsNewModal.tsx:71`, `:82`, `:118` | Commit 4 |
| F19 | Browse mode has no loading, error or empty state, so it opens a blank dialog on release and dev | Medium | §2.2 | Commit 4 |
| F20 | Browse mode renders the whole history at once | Low | §2.2 | Commit 4 (render the newest 10 with a "Show older releases" button; no API change) |
| F21 | Emoji read aloud in headings, in all 28 locales | Low | `locales/*/translation.json`; aria snapshot | Commit 4: strip the leading emoji from all 28 locale files (a mechanical edit of the three keys, an accepted exception to "Crowdin owns locales" because it removes a shared prefix and changes no wording); render the icon `aria-hidden`; update the aria snapshot |
| F22 | Hard-coded English toasts | Low | `useChangelog.ts` | Commit 4 |
| F12 | No version image tags were ever published, although VERSION.md:51-56 promises `v1.0.0`, `1.0` and `1`. `type=semver` only fires on tag refs, and `docker-build.yml` has no tag trigger | Medium | 791 GHCR tags, 0 semver; Docker Hub likewise | Commit 7 publishes `X.Y.Z` and `X.Y` (no `v`, no major-only tag while on v0). Commit 7 also protects `X.Y` from pruning. Commit 12 corrects VERSION.md |
| F7 / #1867 | Nightly (Charon `:275-276` and Orthrus `:542`) uses `github.sha` (= `main` on `schedule`) instead of the checked-out nightly commit | Medium | Nightly label `revision=f3271ae35` = `main` HEAD | Commit 8 |
| F2 | System Settings Updates card reads fields that do not exist | Medium | `SystemSettings.tsx:37-42`, `:668-690` | Commit 9 |
| F9 | `changelog_url` rendered without validation | Low | `NotificationCenter.tsx:116` | Commit 9 |
| C6 | Update check is a string compare, so dev and nightly falsely report an update | Medium | §2.2 | Commit 10 |
| C8 | Backup version warning is exact-match (noise) | Low | `backup_restore_safe.go:205-209` | Commit 10 |
| #1866 | Non-release `main` builds (promotion merges) report the previous release version | Low | `docker-build.yml:392-394` | **Not planned (user decision):** `main` is the stable line, the window before the release-please merge is short, and the update check offers the real release; no label added |
| F4 | Makefile versioned targets invalid; `docker-build-versioned` never runs (`Makefile:99` has a leading space); dead `.version`; dead `VITE_APP_VERSION` | Low | `make -n` prints "Nothing to be done" | Commit 11 |
| F6 | Stale speculative comments in `update_service_test.go` | Low | `:35-41` etc. | Commit 10 |
| F23 | Could a non-`main` source (a fork PR, or a branch named `main`) resolve to the `release` channel? | — | Verified in the repo: (1) The `setup` job `if:` (`docker-build.yml:96`) only admits `workflow_run` runs whose `workflow_run.head_repository.full_name == github.repository`, so a fork's `workflow_run` never runs `setup`. A same-repo `head_branch` of `main` *is* `main`. (2) The same `if:` skips `pull_request` runs whose `head_ref` is `main` or `development`. (3) `is_release` and the `release` channel additionally require `TRIGGER_EVENT != pull_request` (§4.5, §4.10), and a `workflow_run` started by a PR carries `workflow_run.event = pull_request` into `TRIGGER_EVENT` (`:65`). (4) `detect-release-commit.sh` also requires that the manifest version increased and that any existing `vX.Y.Z` tag points at HEAD | **Mitigated, closed.** No gap found. Bats tests cover the PR-event and non-`main` cases (§4.5) |

---

## 4. Technical Specifications

### 4.1 `internal/version`: the single SemVer authority (Commit 2)

```go
func Canonical(v string) (string, bool) // bare MAJOR.MINOR.PATCH[-PRE], metadata stripped; false if unversioned
func IsUnversioned(v string) bool
func Compare(a, b string) (cmp int, ok bool) // ignores build metadata; ok=false if either is unversioned
```

- Unversioned means any of: `dev`, empty, a leading `v`, shorthand (`1`, `1.2`), or invalid.
- Requiring the strict three-part form is deliberate, and it is a **small behavior change** despite the
  `refactor:` label. Today `IsUnversionedBuild("1.2")` is false, because Go's semver accepts shorthand.
  After this commit, `1.2` counts as unversioned. No build emits shorthand, so there is no user-visible
  effect. The commit body must state this.
- `changelog.IsUnversionedBuild` is deleted, and every reference to it in code, comments and docs is
  updated. Gate: `grep -rn IsUnversionedBuild backend docs ARCHITECTURE.md VERSION.md
  --exclude-dir=archive --exclude-dir=superpowers` must be empty.
- This includes the stale comment at `user_handler_changelog_seeding_test.go:228-236`.
- Tests:
  - shorthand
  - leading `v`
  - prerelease: `Compare("0.47.0-rc.1","0.46.5+nightly.abc")` returns `(1,true)`, and
    `Compare("0.47.0-rc.1","0.47.0")` returns `(-1,true)`

### 4.2 Changelog backend fixes (Commit 3)

`changelog.go`:

| Item | Behavior |
| --- | --- |
| `IsDevBuild()` | `version.IsUnversioned(current)` |
| **new** `AnchorVersion()` | Canonical current version, or `""` |
| `GetEntriesSince(lastSeen)` | (1) An invalid non-empty `lastSeen` returns nothing (unchanged). (2) **Ceiling**: when versioned, keep only `Compare(e.Version, current) <= 0`. (3) Keep entries newer than `lastSeen` (all entries when it is `""`). (4) Sort newest-first. (5) **Cap every response** at the newest `firstViewEntryLimit = 3`, whatever `lastSeen` is (F8 form 2: release users seeded at v0.36.1 and never acked). The browse view (`GetAllEntries`, Settings → revisit) covers older history |
| `GetAllEntries()` | Ceiling when versioned. No cap (opt-in browse). Everything when unversioned (local UI iteration) |
| `normalizeEntry` | Also drops a `SecurityEntry` whose `SHA` fails `^[0-9a-f]{7,40}$` (F17), and dedupes each list while keeping order (defense in depth for F18) |

Handlers:

- `ChangelogHandler.Ack`:
  - On `dismiss_permanent` with a versioned build, write `AnchorVersion()` **only if** the stored value is
    empty, invalid, or `Compare(anchor, stored) > 0`. The stored value never decreases (F15).
  - A bind failure returns `{"error":"invalid request"}` (F16).
- `seedLastSeenVersion`: the canonical form of `version.Version`, or `""`.
- `routes.go`: warn when `CHARON_CHANGELOG_VERSION` is set but unversioned.

Tests (new or changed):

- ceiling, with and without a versioned build
- the cap of 3 when `lastSeen == ""`
- the cap of 3 for a user many releases behind: stored `0.36.1`, running `0.46.6`, 40 fixture entries
  returns exactly `0.46.6`, `0.46.5`, `0.46.4`
- the cap with fewer than 3 newer entries returns only those (stored `0.46.5`, running `0.46.6` returns
  `[0.46.6]`)
- metadata in `lastSeen`
- `AnchorVersion`
- invalid sha dropped
- dedupe
- Ack:
  - writes the canonical form for `0.46.5+nightly.x`
  - an override of `1.2.0+dev` writes `1.2.0`
  - a downgrade (stored `0.46.6`, running `0.46.5`) leaves `0.46.6`
  - a bind error returns the generic message
- seeding writes the canonical form

Keep the legacy `nightly-<sha>` guards. The E2E fixture has exactly 3 entries and the override is `1.2.0`,
so the ceiling and the cap change nothing there. Confirm by running the spec.

### 4.3 Frontend What's New fixes (Commit 4)

All in `WhatsNewModal.tsx`:

- **Keys (F18).** List keys become `${index}-${text}`. Security items use `${item.sha}-${index}`.
- **Commit link (F17).** Render it only when `/^[0-9a-f]{7,40}$/` matches the sha.
- **Browse mode (F19, F20).**
  - Show a loading state (`allQuery.isLoading`).
  - Show an error state (`allQuery.isError`) with the new key `whatsNew.loadError`.
  - Show an empty state with the new key `whatsNew.empty`: "No release notes are available for this
    version yet."
  - Render the newest 10 versions, then a "Show older releases" button (new key `whatsNew.showOlder`) that
    reveals 10 more each time.
- **Headings (F21).** Strip the leading emoji plus space from `whatsNew.newFeatures`, `whatsNew.fixes` and
  `whatsNew.security` in **all 28** `frontend/src/locales/*/translation.json` files. The translated `de`,
  `es`, `fr` and `zh` values keep their wording. Render the decorative icon in the component with
  `aria-hidden="true"`.

Other files:

- `hooks/useChangelog.ts`: toasts use `t()` with the new keys `whatsNew.ackError` and `whatsNew.optInError`
  (F22).
- New keys (`loadError`, `empty`, `showOlder`, `ackError`, `optInError`) go into `en/translation.json` only.
  Crowdin syncs the rest.
- `frontend/src/hooks/__tests__/useChangelog.test.ts:166`, `:199`: the hard-coded toast strings become the
  i18n keys or values.
- `tests/settings/whats-new-changelog.spec.ts-snapshots/What-s-New-Changelog-shows-the-modal-on-login-when-the-user-is-behind-the-current-version-1.aria.yml`
  (`:5`, `:9`, `:14`, `:17`, `:23` assert `"✨ New Features"` and `"🐛 Fixes"`): regenerate it to the
  emoji-free headings.

Tests: `WhatsNewModal.test.tsx` cases for:

- duplicate strings render without key warnings (spy on `console.error`)
- an invalid sha renders no link
- browse loading, error and empty states
- "Show older" pagination
- toast i18n

### 4.4 Generator hardening and data guard (Commit 5)

`scripts/generate-changelog.sh` keeps its CLI and adds flags:

```text
generate-changelog.sh [--current-version X.Y.Z] [--require-entries] [--output PATH]
```

- **Commit selection.**
  - `git log --no-merges "$range"`.
  - Drop subjects matching `^chore\(main\): release` and `^chore: promote nightly to main`.
  - Dedupe per version on the subject with any trailing `\(#[0-9]+\)` removed (keep the first occurrence).
- **Categorization.**
  - `(security)`-scoped `feat`/`fix` still go **only** to `security`. That rule is checked first and is
    unchanged.
  - `feat`/`fix` whose scope is in `INTERNAL_SCOPES = deps|dep|ci|e2e|test|tests|workflow|lint|docs|build|release|renovate`
    move to `other`.
  - `deps:`, `chore`, `docs`, `test`, `refactor`, `ci`, `build` and `perf` stay in `other`.
- **Head entry (F11 race).** With `--current-version X.Y.Z` and no tag `vX.Y.Z`, emit one extra entry for
  `<newest v* tag>..HEAD` with today's UTC date. Only the release channel passes this flag.
- **Fail loudly.**
  - `set -euo pipefail` (`:7`) already fails on a `git log` error, but `2>/dev/null` (`:87`) hides the reason.
    Remove it so the error is visible.
  - The real silent path: zero `v*` tags (a shallow clone, `:65`) still writes `[]` and exits 0. Exit 1 with
    "no v* tags found (shallow clone?)".
  - With `--require-entries`, exit 1 when the output is `[]`.
  - Dedupe entries by version (protects against a synthesized head entry that duplicates a real tag).
- **New `scripts/ci/check-changelog-data.sh <file> [--expect-newest X.Y.Z]`.** It asserts:
  - valid JSON
  - length at least 1
  - every entry has the four arrays
  - every `security[].sha` is hex
  - no `^Merge` subject anywhere
  - with `--expect-newest`, the first entry's version equals X.Y.Z
- **Tests: new `scripts/tests/generate-changelog.bats`.**
  - Builds a temp git repo with tags, merges, duplicates, internal scopes, a `(security)` fix and a missing
    head tag, then asserts the exact JSON.
  - Also covers the zero-tags failure and `--require-entries`.
- New `scripts/tests/check-changelog-data.bats`.
- Both are wired into `quality-checks.yml:241-246`: shellcheck plus bats.

### 4.5 Data generation in `docker-build.yml` (Commit 6, #1865)

#### New script: `scripts/ci/detect-release-commit.sh`

```text
detect-release-commit.sh --event <TRIGGER_EVENT> --ref <TRIGGER_REF> [--manifest .release-please-manifest.json]
stdout: is_release=<true|false> and version=<X.Y.Z|>   (two lines, for $GITHUB_OUTPUT)
```

`is_release=true` only when **all** of these hold:

1. `--event` is not `pull_request`.
2. `--ref` is `refs/heads/main`.
3. `git diff --name-only HEAD^1 HEAD` includes the manifest. `HEAD^1` is the first parent, which covers a
   release-please two-parent merge like f2cb630d5 (parents 66b0a139f and 369a61213) as well as a squash.
4. The manifest `"."` value at `HEAD` matches `^[0-9]+\.[0-9]+\.[0-9]+$` and is SemVer-greater than the
   value at `HEAD^1`. For f2cb630d5 that is `0.46.4` to `0.46.5`.
5. If tag `v<version>` already exists, `git rev-parse v<version>^{commit}` equals `HEAD`. A tag elsewhere
   means the commit is not the release.

Anything else gives `is_release=false` and an empty version. It never fails the build, and the safe
direction is "not a release".

Bats tests (`scripts/tests/detect-release-commit.bats`, temp repos) cover:

- a release-please two-parent merge (true)
- a squash release commit (true)
- a weekly promotion merge with no manifest change (false)
- a manifest edit that does not increase the version (false)
- an invalid version (false)
- `refs/heads/development` (false)
- `--event pull_request` on `main` (false)
- an existing tag on another commit (false)
- a tag at HEAD (true)

#### `setup` job steps (after checkout with `fetch-depth: 0`)

Each new step is gated on `steps.skip.outputs.skip_build != 'true'`, so skipped builds never fail in
`setup`. `TRIGGER_*` values are passed via `env:` only.

1. **"Detect release commit"** runs the script and outputs `is_release` and `release_version`. It also checks
   that the version matches `^[0-9]+\.[0-9]+\.[0-9]+$`, so Commits 6 and 7 are validated before the
   Commit 11 resolver exists.
2. **"Generate changelog data"**
   - `bash scripts/generate-changelog.sh --require-entries`, plus `--current-version "$RELEASE_VERSION"` when
     `is_release`.
   - Then `bash scripts/ci/check-changelog-data.sh backend/internal/changelog/data/changelog.json`, plus
     `--expect-newest "$RELEASE_VERSION"` when `is_release`.
3. **Upload** with `actions/upload-artifact@cf430e030ddbb5b0abf93d22962f4752f3646cd9 # v7.0.2` (the pin
   already used in `codecov-upload.yml:162`):
   - name `changelog-data-${{ github.run_id }}`
   - `overwrite: true`, so a re-run never hits 409 "artifact already exists"
   - `retention-days: 1`

#### Build jobs

- **`build-amd64` / `build-arm64`**: before the build steps (`:493`, `:587`), download with
  `actions/download-artifact@9000827ccba6bdab643e8b6fd33ac0654aef8333 # v8` (the pin already used in
  `container-prune.yml:216`) into `backend/internal/changelog/data/`, then re-run `check-changelog-data.sh`.
  The data is identical across arches, and the shallow checkouts stay shallow.
- PR images get data too. Fork PRs stay on the inline builder path, and the artifact flow does not depend
  on registry access.
- `nightly-build.yml:262-263` becomes `generate-changelog.sh --require-entries` plus `check-changelog-data.sh`.
- `e2e-tests-split.yml:204-208` (fixture injection) is unchanged.

### 4.6 Semver image tags for release commits (Commit 7, F12)

- In `docker-build.yml`'s metadata-action, add raw tags `X.Y.Z` (= `release_version`) and `X.Y`
  (computed in `setup`), enabled only when `is_release == 'true'`.
  - **No `v` prefix**, matching the version the binary reports.
  - **No major-only tag while on v0**: `0` would mean "any 0.x", and under SemVer 0.x minors may break.
- Remove the dead `type=semver` lines. They need a tag-ref event that this workflow never receives.
- `container-prune.yml` `PROTECTED_REGEX` (`:46`, `:124`): add `"^[0-9]+\\.[0-9]+$"` so the floating `X.Y`
  tags are never pruned. `X.Y.Z` is already protected by `^v?[0-9]+\.[0-9]+\.[0-9]+$`.
- VERSION.md:51-56 currently promises `v1.0.0`, `1.0` and `1`. Commit 12 corrects it to the real set and
  explains why there is no major tag.

### 4.7 Real smoke test against the built image (Commit 8, F13), plus the nightly sha fix (F7 / #1867)

New `scripts/ci/smoke-whats-new.sh`:

```text
smoke-whats-new.sh --image <ref> (--expect-version-exact <string> | --expect-version-regex <ERE>)
                   [--expect-commit <sha>] [--expect-min-entries N] [--expect-newest X.Y.Z]
                   [--expect-status-for-unseeded <shown|hidden>] [--port 18080]
```

Version flags:

- `--expect-version-exact` compares the string exactly. CI always uses it.
- `--expect-version-regex` is for ad-hoc local use. `+` is a regex quantifier, so `0.46.5+dev` never matches
  itself as a regex; callers must escape it (`0\.46\.5\+dev`).
- Exactly one of the two is required.

What it does:

- **Volume.** Creates a named volume `smoke-wn-<rand>-data`. There is no host bind mount, so it avoids the
  image's uid 1000 vs the runner's uid 1001, the entrypoint `chown`, and rootless uid mapping.
- **Container.** `docker run -d --name smoke-wn-<rand> -p 127.0.0.1:<port>:8080 -v
  smoke-wn-<rand>-data:/app/data -e CHARON_ENV=production -e CHARON_ENCRYPTION_KEY=<random>`.
- A `trap` always runs `docker rm -f smoke-wn-<rand>` and `docker volume rm smoke-wn-<rand>-data`.
- Waits for `GET /api/v1/health` to return `application/json`, not the SPA's `text/html`. Then it asserts
  the version, and `git_commit` when `--expect-commit` is given.
- Calls `POST /api/v1/setup` and logs in.
- Asserts the `GET /changelog/all` count is at least N and the newest entry matches.
- **Unseeded user** (when `--expect-status-for-unseeded` is given):
  - Creates a user via `POST /users`.
  - Sets its `last_seen_version` to `''` with `docker exec smoke-wn-<rand> sqlite3 /app/data/charon.db
    "UPDATE users SET last_seen_version='' WHERE email='…'"`. The image ships `sqlite` (`Dockerfile:1007`).
    No stop or start is needed: the change is visible to the next request.
  - Logs in as that user and asserts `show_changelog` with **3** entries (or `hidden`).
  - Sends `dismiss_permanent` and asserts that the next status is hidden (no nag loop).
- **Host dependencies:** `docker`, `curl` and `jq` only. No host `sqlite3`.
- Never touches existing containers or volumes. The `smoke-wn-` prefix is unique.

`scripts/tests/smoke-whats-new.bats` uses a stubbed `docker`/`curl` on PATH. It covers:

- argument parsing, including that exactly one version flag is required
- exact matching, including `0.46.5+dev` matching exactly
- regex matching, including that the unescaped `0.46.5+dev` does **not** match while the escaped form does
- the HTML-instead-of-JSON health failure
- trap cleanup of the container and the volume

Wiring:

- **`docker-build.yml`** gets a new job `smoke-whats-new`.
  - `needs: [setup, build-amd64]`, skipped for fork PRs and when `skip_build`.
  - Explicit `permissions: { contents: read, packages: read }`.
  - It pulls the **already-pushed per-arch image** `ghcr.io/<image>:build-${{ github.run_id }}-amd64` by
    the digest from `needs.build-amd64.outputs.digest` (`:447`, written at `:533-534`).
  - **`merge-and-publish.needs` (`:638`) adds `smoke-whats-new`**, and its `if:` adds
    `needs.smoke-whats-new.result == 'success'` (or `'skipped'` for fork PRs).
  - The smoke check therefore runs **before** any user-facing tag is published, and a failure stops a bad
    release.
  - arm64 is not smoke-tested. It is the same source and the same embedded data, and QEMU startup would add
    several minutes.

  | Build | Version flag | Other flags |
  | --- | --- | --- |
  | release (`is_release`) | `--expect-version-exact "$RELEASE_VERSION"` | `--expect-newest "$RELEASE_VERSION" --expect-min-entries 100 --expect-status-for-unseeded shown` |
  | others (`main`, `development`, PR, branch) | `--expect-version-exact "$BUILD_VERSION"` (resolver output, §4.10) | `--expect-min-entries 100 --expect-status-for-unseeded shown` |

  The step asserts that `BUILD_VERSION` is one of `X.Y.Z`, `X.Y.Z+dev.<sha7>`, `X.Y.Z+pr.<N>` or
  `X.Y.Z+branch.<sha7>`. A degraded `dev` value therefore fails the gate before publish.

- **`nightly-build.yml:589-622`**: replace the vacuous `/health` loop with the script against the pushed
  nightly digest, using `--expect-commit "$BUILT_SHA"`.
- **Nightly sha fix.**
  - In `build-and-push-nightly`, a new step after checkout (`:205-210`) writes `sha=$(git rev-parse HEAD)` to
    `$GITHUB_OUTPUT` and exposes it as a job output `built_sha`.
  - `VCS_REF=` (`:276`) and the version (`:275`) use it.
  - The smoke job asserts `git_commit == needs.build-and-push-nightly.outputs.built_sha`, the job's own
    checkout. It does **not** use `origin/nightly`, because `workflow_dispatch` checks out `github.ref`
    (`:209`).
  - The Orthrus job (`:542`) gets the same treatment: a `git rev-parse HEAD` after its own checkout feeds
    `VERSION=nightly-<sha>` and `GIT_COMMIT=<sha>`.

### 4.8 Update card, hook, URL validation (Commit 9, F2/F9)

- `update_service.go`:
  - `UpdateInfo` gains `CurrentVersion string \`json:"current_version"\``.
  - New `releaseURL(raw)` keeps the URL only when the scheme is `https` and `Hostname() == "github.com"`
    with no userinfo; otherwise `""`.
  - Table test: http, a look-alike host, userinfo, `javascript:`, empty.
- New `frontend/src/hooks/useSystemUpdates.ts`: `useSystemUpdates({ enabled = true } = {})` with key
  `['system-updates']`, staleTime 1h, and the `refetch` exposed.
  - `SystemStatus.tsx` and `NotificationCenter.tsx` use the default (automatic, as today).
  - `SystemSettings.tsx` keeps its manual-only behavior (today `enabled: false`, `:212`): it calls
    `useSystemUpdates({ enabled: false })` and runs `refetchUpdates()` from its "Check for updates" button.
  - Sharing the key means the button reuses data that NotificationCenter already fetched. That is intended:
    the card shows the cached result immediately instead of a blank card.
  - The hook test covers both `enabled` modes.
- `SystemSettings.tsx`:
  - Drop the local type (`:37-42`) and the inline query (`:201-213`).
  - Rename the destructured `refetch` to `refetchUpdates` (it was `checkUpdates` at `:204`, colliding with
    the API function).
  - Render `current_version`, `available` and `changelog_url`.
  - Hide the link when the URL is `""`, and do the same in `NotificationCenter.tsx:116`.

### 4.9 Semantic compare: update check and backup (Commit 10)

| Running | Latest | `available` |
| --- | --- | --- |
| `0.46.5` | `v0.46.6` | true |
| `0.46.5` | `v0.46.5` | false |
| `0.46.5+dev` / `+nightly.x` | `v0.46.5` | false |
| `0.46.5+nightly.x` | `v0.46.6` | true |
| `0.46.6` | `v0.46.5` | false |
| any unversioned (`dev`, `nightly-<sha>`) | any | **false** (default, pending user confirmation) |
| any | non-semver or empty tag | false |

Backup `appVersionWarning(archive, running)`:

| Archive vs running | Warning |
| --- | --- |
| Same canonical | none |
| Archive older | existing text |
| Archive **newer** | "archive was created by a newer Charon version (X) than the running version (Y); restoring it may fail or lose data" |
| Either unversioned | Exact compare, as today |

`Valid` stays `true` in every case. Also remove the stale comments in `update_service_test.go` (F6).

### 4.10 Build-version resolver (Commit 11 and F4)

```text
scripts/ci/resolve-build-version.sh --channel <release|main|dev|nightly|pr|branch|local> [--sha S] [--pr N] [--manifest P]
```

| Channel | Output | Selected when (`docker-build.yml`, normalized `TRIGGER_*` only) |
| --- | --- | --- |
| `release` | `X.Y.Z` | `is_release == true` (§4.5) |
| `main` | `X.Y.Z` (manifest version, no tags) | `TRIGGER_REF == refs/heads/main`, not a PR, not a release commit; no build metadata, because `main` is the stable line |
| `dev` | `X.Y.Z+dev.<sha7>`, `--sha "$TRIGGER_HEAD_SHA"` | `TRIGGER_REF == refs/heads/development`, not a PR |
| `pr` | `X.Y.Z+pr.<N>` | `TRIGGER_EVENT == pull_request` and `TRIGGER_PR_NUMBER` non-empty |
| `branch` | `X.Y.Z+branch.<sha7>` | everything else (including `workflow_run`-of-PR with no number), `--sha "$TRIGGER_HEAD_SHA"` |
| `nightly` | `X.Y.Z+nightly.<sha7>` | `nightly-build.yml`, `--sha` = checked-out HEAD (§4.7) |
| `local` | `X.Y.Z+local.<sha7>[.dirty]` | Makefile |

Rules:

- `X.Y.Z` comes from `.release-please-manifest.json` (`jq`, with a strict `grep` fallback) and must match
  `^[0-9]+\.[0-9]+\.[0-9]+$`.
- Missing or invalid base gives `dev` and exit 0, **except `release`, which exits 1**.
- Bad input exits 2.
- The step asserts that `release` output contains no `+` and that a PR event never resolves to `release`.
- Writes to the step summary.
- bats test `scripts/tests/resolve-build-version.bats`; shellcheck in `quality-checks.yml`.

Wiring:

| Where | Change |
| --- | --- |
| `docker-build.yml:385-396` | Replace the metadata-version logic. Image tags are unchanged apart from Commit 7 |
| `nightly-build.yml` | Version from the resolver (nightly channel) |
| `Makefile:68-76` | `build-versioned` uses the `local` channel; drop `VITE_APP_VERSION` |
| `Makefile:98-106` | `docker-build-versioned`: fix the `:99` leading space, use the `local` channel, tag with `+` replaced by `-`, drop `.version` |
| `Dockerfile:287-289` | Drop the dead `ENV VITE_APP_VERSION` |
| `Dockerfile:316-320` | Gate regex `^v?[0-9]+\.[0-9]+\.[0-9]+(\+\|$)`; `npm version "${VERSION%%+*}"` with any leading `v` stripped |
| `Dockerfile:5`, `:406`, `:1197` | `ARG VERSION=dev` unchanged |
| `.dockerignore:163-177` | No change in this commit. `scripts/ci/` is excluded in Commit 5 (see §7, "Ignore files") |
| `codecov.yml:90` | Already ignores `scripts/**`. No change |

The smoke job uses `--expect-version-exact` with the exact string the build received. Non-release
images use the resolver output and add `--expect-status-for-unseeded shown`. The nightly job
(`nightly-build.yml`) uses the same flags with `X.Y.Z+nightly.<sha7>` and `--expect-commit "$BUILT_SHA"`.

**Orthrus does not use the resolver.** It keeps `nightly-<sha>`, with the corrected checked-out sha from
Commit 8. Its version is only logged (non-goal).

Unversioned-build E2E pinning (Commit 1) keeps E2E on `dev`.

---

## 5. How each channel works end to end once everything lands

Every status response is capped at the newest 3 entries, at or below the running version (R1).

| Image | Embedded data | Reported version | New user | Existing user `''` | User seeded long ago (`'0.36.1'`, never acked) | Existing user `0.46.5`, new release `0.46.6` | Update check (latest `v0.46.5`) |
| --- | --- | --- | --- | --- | --- | --- | --- |
| Release commit on `main` (`latest`, `0.46.6`, `0.46`) | All tags plus the synthesized `0.46.6` entry | `0.46.6` | Seeded `0.46.6`, no modal | Modal: newest 3 (≤ 0.46.6); dismissal anchors `0.46.6` | Modal: **3** (`0.46.6`, `0.46.5`, `0.46.4`), not about 35 | Modal: `0.46.6` only, once | false |
| Non-release `main` (promotion merge) | All tags (≤ base) | `0.46.5` | Seeded `0.46.5` | Newest 3 ≤ 0.46.5 | 3 | Nothing (base equals the seen version) | false |
| `development` (`:dev`) | All tags | `0.46.5+dev.<sha7>` | Seeded `0.46.5` | Newest 3 ≤ 0.46.5 | 3 | Nothing | false |
| PR (`pr-N`) | All tags | `0.46.5+pr.N` | Seeded `0.46.5` | Newest 3 | 3 | Nothing | false |
| Nightly | All tags (may include `0.46.6` before the manifest propagates) | `0.46.5+nightly.<sha7>` (checked-out sha) | Seeded `0.46.5` | Newest 3 **≤ 0.46.5** (ceiling) | 3, ≤ 0.46.5 | Nothing, and no nag loop | false (true once `v0.46.6` is published) |
| Local `go run` / E2E | `[]` / fixture | `dev` | Not seeded | Hidden (unversioned) | Hidden | Hidden | false |

---

## 6. Priorities and Ordering

- **User override:** all fixes and the versioning work share one PR. Within it, **bug-fix commits come
  first** (Commits 1-9), then the features (Commits 10-11), then docs (12).
  - **Landed order differs:** Commit 10 (`feat`) landed before the CI Commits 6-8 and 11, because its dependencies (Commits 2 and 9) were already in. The consumer fixes (Commits 1-4) still land first. The real order is in §7.
- Consumer fixes come **before** the pipeline starts shipping data (Commit 6). These are the cap on every
  response, the ceiling, the never-lowering Ack and the frontend states. Shipping data first would let an
  intermediate deploy dump 171 entries on `''` users and about 35 on users seeded at v0.36.1.
- High-severity items (F1/#1865, F8, F11) are all in the bug-fix block.
- The orchestrator still needs to file **one** issue: F16's app-wide pattern (binder error leakage in other
  handlers). #1865 and #1867 close with this PR.
- The fork-PR / branch-named-`main` concern (F23) is **mitigated and closed**, with evidence. Nothing to
  file. No private advisory candidate.

---

## 7. Commit Slicing Strategy (one PR; the plan's 12 commits plus 2 extra, in the order they landed)

Landed order on `fix/whats-new-pipeline-and-build-versions`, from `git log --oneline development..HEAD`. The `Commit N` labels are the plan numbers. They stay, because §§4, 6, 8 and 12 cite them. The **Landed** column is the real order. Two commits were not in the plan: the spec commit (row 1) and the import-order autofix (row 5). Supervisor review-fix commits are added after the last row.

| Landed | Plan | Message | Files | Depends | Gate (foreground, targeted) |
| --- | --- | --- | --- | --- | --- |
| 1 | — | `docs: add what's new pipeline and build versions spec` | `docs/plans/current_spec.md` | — | Docs only; no code gate |
| 2 | Commit 1 | `test: pin what's new e2e runs to unversioned dev images` | `e2e-tests-split.yml` (`VERSION=dev` build-arg + comment, `:264-266`), `.github/skills/docker-rebuild-e2e-scripts/run.sh` (`build_image` `--build-arg VERSION=dev`), `tests/settings/whats-new-changelog.spec.ts` (`beforeAll` reads `/api/v1/health`; when `version !== 'dev'` it calls **`test.skip(true, …)`** with the reason: "What's New E2E needs an unversioned VERSION=dev image; registry image_ref runs (e2e-tests-split.yml:28, :167-178) are versioned and lack the fixture") | — | `npx playwright test tests/settings/whats-new-changelog.spec.ts --project=firefox` (runs, not skipped, on the local `dev` image) |
| 3 | Commit 2 | `refactor: centralize semver version checks in the version package` | `internal/version/*`, `internal/changelog/changelog.go` + test, `handlers/user_handler.go`, `handlers/changelog_handler.go` (comments), `handlers/user_handler_changelog_seeding_test.go` (stale comment `:228-236`). The commit body notes the one deliberate tightening: shorthand `1.2` becomes unversioned | — | `go test ./internal/version/... ./internal/changelog/... ./internal/api/handlers/ -run 'Changelog\|SeedLastSeen'`; `make lint-fast`; the `IsUnversionedBuild` grep (§4.1) is empty |
| 4 | Commit 3 | `fix: cap and anchor the what's new changelog for every user` | `changelog.go` (anchor, ceiling, cap 3 on every response, sha validation, dedupe), `changelog_handler.go` (never-lowering Ack, generic bind error), `user_handler.go` (canonical seed), `routes.go` (override warning), tests including stored `0.36.1` / running `0.46.6` returns 3 | 2 | Scoped `go test` as above; **`./scripts/scan-gorm-security.sh --check`** (Ack's conditional update path changes); E2E spec from Commit 1 |
| 5 | — | `chore: apply import-x/order autofix after the plugin bump` | 38 frontend files, import order only (`frontend/src/components/**`, `frontend/src/hooks/**`, `frontend/src/context/**`) | — | The frontend-lint hook passes with `--max-warnings 0`. Why it exists: `eslint-plugin-import-x` 4.18.0 flagged 60 import-order warnings on `development`, which blocked that hook. Autofix only, no behavior change |
| 6 | Commit 4 | `fix: make the what's new dialog safe to browse and render` | `WhatsNewModal.tsx`, `hooks/useChangelog.ts`, **all 28** `frontend/src/locales/*/translation.json` (emoji stripped from 3 keys; new keys in `en` only), `components/dialogs/WhatsNewModal.test.tsx`, `hooks/__tests__/useChangelog.test.ts` (`:166`, `:199`), `tests/settings/whats-new-changelog.spec.ts-snapshots/What-s-New-Changelog-shows-the-modal-on-login-when-the-user-is-behind-the-current-version-1.aria.yml` | — | `npx vitest run src/components/dialogs/WhatsNewModal.test.tsx src/hooks/__tests__/useChangelog.test.ts`; `npm run type-check`; E2E spec (aria snapshot matches) |
| 7 | Commit 9 | `fix: show the running version and update status in the system settings update card` | `update_service.go` (`CurrentVersion`, `releaseURL`) + test, `api/system.ts`, **new** `hooks/useSystemUpdates.ts` (with an `enabled` option) + test, `SystemSettings.tsx` (manual trigger kept), `SystemStatus.tsx`, `NotificationCenter.tsx`, their tests | — | `go test ./internal/services/ -run UpdateService`; vitest on those files; `npm run type-check`; `npx playwright test tests/security-enforcement/zzz-security-ui/system-security-settings.spec.ts --project=firefox` |
| 8 | Commit 5 | `fix(ci): generate clean changelog data and fail when it is missing` | `scripts/generate-changelog.sh`, **new** `scripts/ci/check-changelog-data.sh`, **new** `scripts/tests/generate-changelog.bats`, **new** `scripts/tests/check-changelog-data.bats`, `quality-checks.yml`, `nightly-build.yml:262-263`, `.dockerignore` (exclude all of `scripts/ci/`) | — | shellcheck; both bats files; on the real repo, `bash scripts/generate-changelog.sh --require-entries && bash scripts/ci/check-changelog-data.sh backend/internal/changelog/data/changelog.json` passes with 0 `Merge` subjects (then `git checkout -- backend/internal/changelog/data/changelog.json`) |
| 9 | Commit 10 | `feat: compare versions semantically in update checks and backup validation` | `update_service.go` + test (stale comments removed), `backup_restore_safe.go` + `backup_restore_safe_coverage_test.go` | 2, 9 | `go test ./internal/services/ -run 'UpdateService\|ValidateBackup'` |
| 10 | Commit 6 | `fix(ci): embed changelog data in release, development and pr images` (#1865) | **new** `scripts/ci/detect-release-commit.sh` + `scripts/tests/detect-release-commit.bats`, `docker-build.yml` (`setup`: detect, generate + guard, upload with `overwrite: true`, every step gated on `skip_build`; build jobs: download + guard), `quality-checks.yml` | 3, 4, 5 | shellcheck + bats; **local release simulation:** `bash scripts/generate-changelog.sh --current-version 0.46.6 --require-entries && bash scripts/ci/check-changelog-data.sh backend/internal/changelog/data/changelog.json --expect-newest 0.46.6` (then restore the placeholder); actionlint via lefthook; the PR's own run shows the guard passing and the artifact downloaded in both build jobs |
| 11 | Commit 7 | `fix(ci): publish version tags for release images` | `docker-build.yml` metadata-action (raw `X.Y.Z` / `X.Y` on `is_release`, version regex-checked in `setup`; dead `type=semver` lines removed), `container-prune.yml` `PROTECTED_REGEX` `:46`, `:124` (+ `^[0-9]+\.[0-9]+$`) | 6 | actionlint; on the PR the metadata step shows the version tags disabled |
| 12 | Commit 8 | `fix(ci): smoke test what's new on built images before publishing and label nightlies with the built commit` (#1867) | **new** `scripts/ci/smoke-whats-new.sh` + `scripts/tests/smoke-whats-new.bats`, `docker-build.yml` (`smoke-whats-new` job on the `build-<run_id>-amd64` digest, `permissions: contents: read, packages: read`; `merge-and-publish.needs` and `if:` updated), `nightly-build.yml` (checked-out `built_sha` for Charon `:275-276` and Orthrus `:542`; smoke replaces `:589-622`), `quality-checks.yml` | 6 | shellcheck, bats; the PR run shows `smoke-whats-new` passing **before** `merge-and-publish`; QA runs the script locally against the PR image (§8) |
| 13 | Commit 11 | `feat(ci): stamp non-release builds with the released base version` | **new** `scripts/ci/resolve-build-version.sh` + bats, `docker-build.yml` (resolver feeds `BUILD_VERSION`, `is_release` selects `release`), `nightly-build.yml` (Charon only; Orthrus stays `nightly-<sha>`), `quality-checks.yml`, `Makefile` (F4 incl. `:99`), `Dockerfile` (`:287-289`, `:316-320`), smoke expectations | 3, 6, 8, 10 | shellcheck + bats; `make -n docker-build-versioned` prints the recipe; `make -n build-versioned` shows the resolver; the PR run logs `Resolved build VERSION: 0.46.5+pr.<N>`, and `smoke-whats-new` asserts it exactly and shows the 3-entry modal for an unseeded user |
| 14 | Commit 12 | `docs: document the what's new pipeline and build-metadata versions` | `VERSION.md`, `ARCHITECTURE.md`, `docs/features.md`, `docs/features/security.md`, `docs/issues/whats-new-changelog-manual-test-plan.md` | 11 | `bash docs-site/scripts/sync-docs.sh && cd docs-site && zensical build --strict` (no manifest or nav change needed; `features.md` is already in `docs-site/scripts/docs-manifest.json:4`) |

Dependency check on the landed order: every row depends only on rows above it. Commit 9 (update card) landed before every CI commit, because it has no dependency on them. Commit 10 (semantic compare) landed after Commit 5 and before Commits 6, 7, 8 and 11. It depends only on Commits 2 and 9, which are already in.

**Review-fix commits:** to be added after the supervisor review of this branch. They are not listed yet.

Every commit is independently deployable:

- Before Commit 6, shipped images still embed `[]`, so the consumer fixes are inert but safe.
- Commit 6 ships data only after the cap and ceiling (Commit 3) and the UI fixes (Commit 4) exist.
- Nightly already ships data, but it stays hidden (unversioned) until Commit 11, by which point the cap and
  ceiling are in place.
- Commit 8 only adds a gate before publishing.

Workflow hygiene (every new or changed step):

- Trigger context goes through `env:`, never `${{ }}` inside `run:`.
- Actions are pinned by full SHA.
- New jobs declare minimal `permissions:`.

Ignore files:

- `.dockerignore` gains the whole `scripts/ci/` directory in Commit 5.
  - Rationale: `scripts/ci/` already holds 9 CI-only files plus `lib/`. `Dockerfile:1166` (`COPY scripts/
    /app/scripts/`) currently ships them into the runtime image at `/app/scripts/ci/`, and this PR adds four
    more.
  - Nothing at runtime uses `scripts/ci/`. The only non-CI mention is a doc comment in
    `backend/internal/orthrus/muzzle.go:223`. Confirm with `grep -rn scripts/ci Dockerfile .docker/`.
- `scripts/tests/` is already ignored.
- `codecov.yml:90` already ignores `scripts/**`.

Changelog visibility: every subject is public. None uses `(security)`.

**Branch / PR.**

- Create `fix/whats-new-pipeline-and-build-versions` from `development`.
- The PR goes into `development` and is merged with "Create a merge commit".
- The PR body says `Closes #1865, #1867`, lists the commits, and contains no session ID or link.
- Let it ride nightly before promotion, per the user's soak preference.

Contingency:

- Revert individual commits inside the PR. Commits 7, 8 and 11 are self-contained CI layers.
- If release detection misfires, it fails toward "not a release": no version tags, the plain manifest version
  string, and the data guard still runs.
- If the smoke job is flaky, revert only Commit 8's `merge-and-publish.needs` line. Do not weaken the
  assertions.
- Post-merge rollback reverts the merge commit. Stored `last_seen_version` values are canonical SemVer, so
  no migration is needed.

---

## 8. Acceptance: verification matrix against BUILT images (QA runs this, not E2E fixtures)

### 8.1 How

- **CI.** `smoke-whats-new` (Commit 8) gates every `docker-build.yml` publish (release, `main`,
  `development`, same-repo PRs) and runs in `nightly-build.yml`.
- **Locally**, QA uses the same script against PR or published images. Run in the foreground, rootless
  daemon only. The script creates only `smoke-wn-*` containers and volumes on `127.0.0.1` and removes them.
  QA never touches existing containers.

  ```bash
  export DOCKER_HOST=unix:///run/user/1001/docker.sock
  bash scripts/ci/smoke-whats-new.sh --image ghcr.io/wikid82/charon:pr-<N> \
    --expect-version-exact '0.46.5+pr.<N>' --expect-min-entries 100 --expect-newest 0.46.5 \
    --expect-status-for-unseeded shown
  ```

- **Upgrade path** (real artifacts; named volume `qa-wn-<rand>-data`, removed afterwards):
  1. Run the **published** `ghcr.io/wikid82/charon:latest` (0.46.5) on the volume, run setup, and create
     user A (seeded `'0.46.5'`).
  2. With `docker exec … sqlite3`, set user B to `'0.36.1'` (simulates "seeded at v0.36.1, never acked") and
     user C to `''`.
  3. Remove that container, keep the volume, and start the PR image on the same volume.
  4. Expect:
     - A sees no modal (base 0.46.5).
     - B sees exactly 3 entries, all ≤ base.
     - C sees exactly 3 entries.
     - After "Got it", B and C see nothing on the next 2 logins.

### 8.2 Matrix

| # | Image | Expected `/health.version` | `/changelog/all` | Fresh user status | `''` user status | After `dismiss_permanent` | `/system/updates.available` | Method |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| M1 | PR image (`pr-<N>`) | `0.46.5+pr.<N>` | ≥ 100 entries, newest `0.46.5`, 0 `Merge` subjects, no duplicates | hidden (seeded `0.46.5`) | **shown, 3 entries ≤ 0.46.5** | hidden; `last_seen_version='0.46.5'` | false | smoke script + curl |
| M2 | `:dev` after merge | `0.46.5+dev.<sha7>` | same as M1 | hidden | shown, 3 | hidden | false | smoke script |
| M3 | `:nightly` after merge | `0.46.5+nightly.<sha7>`; `git_commit` and the OCI `revision` equal the nightly job's own `built_sha` (`git rev-parse HEAD` after its checkout), **not** `origin/nightly` | same, plus any newer tags | hidden | shown, 3, **all ≤ 0.46.5** | hidden, also across 2 more logins (no nag loop) | false | smoke script `--expect-commit "$BUILT_SHA"` |
| M4 | First release after merge (`latest`, `X.Y.Z`, `X.Y` tags exist; no major-only tag) | `X.Y.Z` | newest = `X.Y.Z` (synthesized if the tag lagged) | hidden (seeded `X.Y.Z`) | shown, 3 (newest `X.Y.Z`) | hidden | false | smoke script (release flags) + registry tag listing |
| M5 | M4, user upgraded from 0.46.5 (`'0.46.5'`) | `X.Y.Z` | — | — | only the versions in `(0.46.5, X.Y.Z]`, at most 3 | hidden | false | upgrade path |
| M5b | M4 or PR image, user seeded long ago (`'0.36.1'`, never acked) | — | — | — | **exactly 3** (the newest ≤ running), not about 35 | hidden | — | upgrade path (user B) |
| M6 | Non-release `main` build | `X.Y.Z` (no tags) | ≤ base | hidden | shown, 3 | hidden | false | smoke script |
| M7 | Downgrade: user at `X.Y.Z` runs an older image | older version | — | — | — | `last_seen_version` stays `X.Y.Z` | — | upgrade path in reverse + `docker exec sqlite3` read |
| M8 | Any image, passthrough user | — | 403 | 403 (modal hidden) | — | 403 | — | API check: create a passthrough user, log in, then `curl` all four `/changelog/*` routes and expect 403 |
| M9 | PR image, Settings → Appearance → revisit | — | — | Loading state, then the newest 10 plus "Show older releases"; no React key errors in the console; headings have no emoji in the accessible name. Empty state checked on a `dev`-default local build with `[]` | — | — | — | **Playwright MCP browser** against a throwaway PR-image container (snapshot + console messages) |
| M10 | PR image, System Settings Updates card | — | — | Shows `current_version`; the "Check for updates" button triggers the fetch; correct status; link shown only for an https `github.com` URL | — | — | — | **Playwright MCP browser** against the same container |

---

## 9. Test plan summary

| Layer | New or changed |
| --- | --- |
| Go unit | `version` helpers (incl. prerelease); changelog ceiling, cap on every response (incl. `0.36.1` → 3), anchor, sha, dedupe; Ack (canonical, override with metadata, downgrade, bind error); seeding; update compare + `releaseURL`; backup warning |
| Vitest | `WhatsNewModal` (keys, sha, browse states, pagination, emoji-free headings); `useChangelog` toasts; `useSystemUpdates` (both `enabled` modes); SystemSettings / NotificationCenter / SystemStatus |
| bats | `generate-changelog`, `check-changelog-data`, `detect-release-commit`, `smoke-whats-new` (exact vs regex, stubbed docker, trap cleanup), `resolve-build-version` |
| E2E (targeted, firefox) | `whats-new-changelog.spec.ts` (with the `beforeAll` skip guard and the regenerated aria snapshot); `system-security-settings.spec.ts`. No new E2E spec: built-image behavior is covered by the smoke script and the matrix, which do not depend on fixtures |
| Built-image | §8 matrix, in CI (smoke gate) and by QA |

## 10. Definition of Done gates

- Targeted firefox E2E (above).
- **GORM security scan:** `./scripts/scan-gorm-security.sh --check` with zero CRITICAL/HIGH. It applies
  because Commit 3 changes Ack's conditional `Updates` write path.
- Patch-coverage preflight.
- Backend and frontend coverage at or above 85%.
- CodeQL Go/JS and Trivy (new feature surface).
- `npm run type-check`.
- `go build ./...` and `npm run build`.
- `lefthook run pre-commit` (staticcheck, actionlint, hadolint, shellcheck).
- All five bats suites green in `quality-checks.yml`.
- The docs-site strict build.
- The §8 matrix:
  - M1, M5b, M7, M8, M9 and M10 against the PR image (method column)
  - M2 and M3 after merge on the next dev and nightly builds
  - M4 and M5 on the next release

## 11. Documentation updates (Commit 12)

- **`ARCHITECTURE.md`.**
  - Changelog Subsystem (`:385-387`): ceiling, cap of 3 on every status response, never-lowering anchor,
    sha validation.
  - Release Workflow (`:1994-2001`):
    - generation in `docker-build.yml` `setup` with the artifact handoff
    - the data guard
    - `detect-release-commit.sh` and the synthesized head entry
    - the pre-publish smoke gate
    - the resolver
    - version tags
  - Fix "its one remaining real caller".
- **`VERSION.md`.**
  - The channel/version table (replaces `:186-197`).
  - "Development Builds" (`:236-253`), including `VERSION=dev` for E2E.
  - Correct `:51-56` and `:105-121` to the **real** tag set: `X.Y.Z` and `X.Y` (no `v` prefix), `latest`,
    `dev`, `nightly`, `nightly-<date>`, `nightly-<sha>`.
  - Explain that there is **no major-only tag while on v0**, because SemVer treats 0.x minors as potentially
    breaking.
  - The generator's filtering rules.
- **`docs/features.md` What's New (`:399-405`).**
  - Nightly and development builds show it too.
  - It shows the latest 3 releases; older history is under "Revisit", 10 at a time.
- **`docs/features/security.md:5`.** Keep the "What's New" pointer, and note that the Security section is
  now actually populated on releases.
- **`docs/issues/whats-new-changelog-manual-test-plan.md`.** Replace the fixture-based steps with the §8
  built-image matrix and the upgrade-path procedure.
- **No docs-site manifest or nav changes.**

## 12. Risks

| Risk | Mitigation |
| --- | --- |
| Release-commit detection misfires | `detect-release-commit.sh` requires the manifest to change, the version to increase and be valid, and any existing tag to point at HEAD, and it fails toward "not a release". Bats-covered |
| Synthesized head entry duplicates a real tag created later | It is synthesized only when `vX.Y.Z` is absent. The generator dedupes by version |
| The smoke gate adds about 2 minutes before publishing | Accepted: it is what stops a bad release. It runs once (amd64), not per arch |
| arm64 is not smoke-tested | Same source and same embedded data. The existing per-arch checks still run |
| Workflow re-run hits a 409 artifact conflict | `overwrite: true` |
| Skipped builds fail in `setup` | Every new step is gated on `skip_build != 'true'` |
| Internal-scope filtering hides a genuinely user-facing `fix(docs)` | It still appears under maintenance details. The list is defined in one place |
| Unscoped internal fixes still show as "Fixes" | Non-goal. Depends on commit hygiene |
| E2E run on a versioned registry image | `beforeAll` skips with a reason (Commit 1) |
| Locale edit touches Crowdin-owned files | Only a leading emoji prefix is removed and no wording changes. Crowdin keeps the source strings in sync on its next pull |
| A user many releases behind sees only 3 | Intended (passive). The full history is under "Revisit" |

## 13. Open questions: defaults, pending user confirmation

1. `development` metadata: **`+dev.<sha7>`** (decided by user: an issue reporter can tell which build they run from the version string alone).
2. Update check on unversioned builds: **report no update** (pending user confirmation).
3. PR and branch builds: **versioned** (`+pr.<N>`, `+branch.<sha7>`) (pending user confirmation; the
   supervisor agrees).
4. Status cap: **3 entries on every status response** (pending user confirmation).
5. Non-release `main` builds: **decided (user):** keep reporting the manifest version `X.Y.Z`, no label; #1866 closed as not planned.
6. Internal scopes kept out of user-facing "Fixes": `deps|dep|ci|e2e|test|tests|workflow|lint|docs|build|release|renovate`
   (pending user confirmation).
7. Browse-mode page size: **10** (pending user confirmation).
8. *(New)* Release image tags: **`X.Y.Z` and `X.Y` only**, with no `v` prefix and no major-only tag while on
   v0 (pending user confirmation).
