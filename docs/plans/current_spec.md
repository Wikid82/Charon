---
goal: Migrate Charon's release/versioning pipeline from custom auto-tagging (paulhatch/semantic-version + GoReleaser) to release-please
version: 1.0
date_created: 2026-08-17
status: 'Planned'
tags: [chore, infrastructure, migration, ci-cd]
---

# Introduction

![Status: Planned](https://img.shields.io/badge/status-Planned-blue)

Charon's release pipeline currently computes the next semantic version with `paulhatch/semantic-version` inside `.github/workflows/auto-versioning.yml`, then hand-rolls a changelog body with shell/grep and publishes a GitHub Release via `softprops/action-gh-release`. A second workflow, `.github/workflows/release-goreleaser.yml`, listens for the resulting `v*` tag push and runs GoReleaser — but this workflow has never once succeeded (every run since `v0.3.0` fails at an "Enforce PR-2 release promotion guard" step because the gating repo variable was never set), so GoReleaser has never built or published any binary/archive/deb/rpm asset for this repo.

This plan replaces both workflows with `googleapis/release-please-action`, following the pattern already validated in the sibling project `/projects/go_notify_yourself`. Release-please computes versions from Conventional Commits by walking real git history, opens/maintains a standing "release PR," and tags + publishes the GitHub Release itself when that PR is merged — eliminating the hand-rolled changelog script and the custom semver-calculation action in one move.

This is a standalone, CI/CD-configuration-only chore. It touches no Go or TypeScript application code, no database schema, no API surface, and is entirely unrelated to the in-progress `feature/notifications-engine-extraction` work. It targets a **new branch cut from `main`**, not `development` — see [Branching Note](#branching-note-deviation-from-normal-development-first-flow) below for why.

**Recommended branch name**: `chore/release-please-migration` (from `main`).

**Merge strategy for this PR itself**: `chore/release-please-migration` → `main` should be merged via **squash merge** (the repo's default for ordinary feature/chore PRs; the "merge commit only" rule in CLAUDE.md applies specifically and only to the weekly `nightly` → `main` promotion PR — see Decision 3). Squashing this PR's six commits into one `chore:`-prefixed commit on `main` is fine either way for release-please's own purposes, since none of this PR's commits are themselves releasable (`feat:`/`fix:`) and release-please's manifest is being explicitly seeded rather than derived from this PR's commit history.

## User Decisions Required Before Implementation

Two behavior changes were load-bearing enough that they should not be discovered after the fact — surfaced here for explicit sign-off before implementation starts, in addition to their mentions later in Edge Cases / Manual Post-Merge Follow-Ups. **Both have since been resolved by explicit user sign-off; recorded below for the record.**

1. **Release cadence changes from fully-automatic to manually-gated.** Today, any non-`chore:` push to `main` is *immediately* followed by `auto-versioning.yml` cutting a tag and publishing a GitHub Release — no human action required. Under release-please, a push to `main` only updates a *standing draft PR*; nothing ships (no tag, no Release, no `orthrus-build.yml` trigger) until a human explicitly merges that PR. (Release-please does support auto-merging its own PR via a label + a second small workflow, but this plan does not implement that — see Manual Post-Merge Follow-Up #7 — so the default behavior after this migration is manual-gate-by-default.) **RESOLVED — user sign-off: APPROVED, proceed as planned (manual-gate-by-default, no auto-merge configured in this PR).**
2. **Chore-only pushes no longer cut a release**, and **`perf:` commits stop being patch-worthy.** Today, a week where only `chore:`/`docs:`/`test:`/`ci:`/`build:`/`style:`/`refactor:` commits land on `main` still gets a patch-bumped release (since `auto-versioning.yml` treats "anything that isn't `feat:`" as patch-worthy, and its changelog-categorization step explicitly buckets `perf:` alongside `fix:`). Under release-please's default releasable-type set (`feat`, `fix`, `deps`), a chore-only period produces **no** release PR at all, and `perf:` commits — not in that default set — likewise stop triggering a release/changelog entry on their own. **RESOLVED — user sign-off: ACCEPT release-please's defaults as-is. Do not add `perf:` (or any other type) to `changelog-sections`/releasable types as a special-case override; `perf:` commits behave like `chore:`/`docs:`/`test:` under release-please's out-of-the-box config** — no config change proposed for this. This is treated as an accepted, disclosed behavior change (arguably a correctness improvement — no more empty "dependency updates and maintenance" releases), not an oversight.

## Branching Note: Deviation from Normal Development-First Flow

The repo's normal convention (see the Semgrep CI plan previously in this file, and the "Merge Soak Before Main" project memory) is feature branches off `development`, PR into `development`, then a weekly `nightly` → `main` promotion carries validated work to `main` roughly a week later. This plan deliberately breaks that pattern: `auto-versioning.yml`, `release-goreleaser.yml`, and the new `release-please.yml` all trigger specifically on `main` (`workflow_run` on the main-branch Docker build, or `push: branches: [main]`), and the live release state (existing `v*` tags, the `CHARON_PR2_GATES_PASSED` variable, the next real release cut) only exists on `main`. A `development`-first path would leave the repo running two parallel, half-migrated release mechanisms for up to a week, which is worse than a direct, carefully-gated `main` PR. **Recommendation for the user**: apply extra manual review scrutiny before merging this PR, since it bypasses the usual nightly soak period by design, not by oversight.

# Research Findings

## Existing Architecture Summary

| File | Role | Status found |
|---|---|---|
| `.github/workflows/auto-versioning.yml` | Computes next semver via `paulhatch/semantic-version@v6.0.3` from `workflow_run` of "Docker Build, Publish & Test" on `main`; hand-greps `feat:`/`fix:`/`perf:` commit-body bullets into a release note; creates tag + GitHub Release via `softprops/action-gh-release@v3`. | Working, but hand-rolled and duplicative of what release-please does natively. |
| `.github/workflows/release-goreleaser.yml` | Triggers on `push: tags: ['v*']`; runs `goreleaser release --clean` (builds linux amd64/arm64 binary, tar.gz archive, deb/rpm via nfpm). | **Never succeeded.** Every run since `v0.3.0` fails at "Enforce PR-2 release promotion guard" (repo variable `CHARON_PR2_GATES_PASSED` has never been set — confirmed via `gh api repos/Wikid82/charon/actions/workflows/release-goreleaser.yml/runs`, `REPO_VARS_JSON: {}` on every run). `gh release view` on `v0.36.3`–`v0.36.5` shows zero attached assets. |
| `.goreleaser.yaml` | GoReleaser config: builds linux/amd64+arm64 binary, tar.gz archive, deb/rpm packages, changelog section. Its own header comment: *"used exclusively for changelog generation... builds/archives/nfpms kept for potential future use but not currently utilized."* | Dead weight — see Decision 5. |
| `.github/workflows/docker-build.yml` | Builds/publishes the actual Charon Docker images. Triggers: `pull_request`, `push: branches: [main, development]`, `workflow_dispatch`, `workflow_run` (Docker Lint). **Does NOT trigger on tag push** (`on:` block has no `tags:` key — confirmed by reading the full trigger block). | This is the sole real distribution channel for Charon (100% Docker). Its `docker/metadata-action` step (line ~334-345) includes `type=semver,pattern={{version}}` tag patterns, but since the workflow never runs on a tag ref, `TRIGGER_REF` is always `refs/heads/*` and those semver patterns never actually resolve to anything — they are inert/vestigial in the current design, not a live tag consumer. |
| `.github/workflows/orthrus-build.yml` | Builds the separate Orthrus agent image. Triggers: `push: branches: [main, development], tags: ['v*']`. Its `docker/metadata-action` step (`type=semver,pattern={{version}}` etc.) **does** fire off tag pushes. | **This is the one real, live downstream consumer of the `v*` tag** that this migration must not break — release-please's created tag must still be a bare `v<semver>` (see Decision 1 / tag-naming risk below) for this workflow's `tags: ['v*']` trigger and semver Docker-tag derivation to keep working identically. |
| `.github/workflows/auto-changelog.yml` ("Auto Changelog (Release Drafter)") | Triggers on `workflow_run` (Docker Build success on `main`) and `release: types: [published]`; runs `release-drafter/release-drafter@v7` against `.github/release-drafter.yml`. | **Not mentioned in the task brief but discovered during research — see "Additional Finding" below.** Redundant with release-please's own standing-PR/changelog mechanism; its own tag-template (`v$NEXT_PATCH_VERSION`) always increments patch regardless of PR label, so it's already partially broken (a PR labeled `feature` would still only bump patch). |
| `.github/release-drafter.yml` | release-drafter config: label-based categorization (`feature`/`feat`, `bug`/`fix`, `chore`, `test`), `tag-template: 'v$NEXT_PATCH_VERSION'`. | Retire alongside `auto-changelog.yml` — see Additional Finding. |
| `VERSION.md` | Documents the "canonical" release process. References `.version` as optional/non-canonical (still accurate). Also references a "release-drafter workflow" for changelog generation — **this turned out to be real** (`auto-changelog.yml`), not stale as originally suspected; the doc's inaccuracy is instead that it describes `release-goreleaser.yml`/`docker-build.yml` as if they jointly "build and publish release artifacts/images through CI" from the tag, which is false per the findings above. | Needs a full rewrite (Phase 5 / Commit 6). |
| `.version` | Currently `v0.27.0` (stale — real latest tag is `v0.36.5`). Already documented in `VERSION.md` as "optional... not the canonical release trigger." | Recommend removal — see Decision 4. |
| `scripts/check-version-match-tag.sh` | Compares `.version` to the latest git tag; **already self-deprecating** — prints a warning telling callers to use `.github/skills/scripts/skill-runner.sh utility-version-check` instead, then runs its own logic anyway. `.github/skills/utility-version-check-scripts/run.sh` just `exec`s this same script — the "migration" to the skill runner is circular and never actually happened. | Recommend removal — see Decision 4. |
| `lefthook.yml` (line ~99) | `check-version-match: { glob: ".version", run: "bash scripts/check-version-match-tag.sh" }` | Remove this hook entry alongside `.version` deletion (Commit 5). |
| `backend/internal/version/version.go` | `Version`, `BuildTime`, `GitCommit` vars, defaulted to `"dev"`/`"unknown"`, set via `-ldflags -X ...` at build time. No in-repo manifest file to bump — confirms release-please needs a manifest-less strategy (Decision 1). | Untouched by this migration. |
| `Dockerfile` (lines ~250-286) | Injects `VERSION`/`GIT_COMMIT`/`BUILD_DATE` into the Go binary via `-ldflags -X github.com/Wikid82/charon/backend/internal/version.*=...`, **identically** to what `.goreleaser.yaml`'s `builds.ldflags` does. Also sets `VITE_APP_VERSION` for the frontend build (line ~133-134). | Confirms version injection already happens independently of GoReleaser in the real (Docker) build path — removing GoReleaser does not touch actual version injection at all. |
| `scripts/generate-changelog.sh` | Regenerates `backend/internal/changelog/data/changelog.json` (the in-app "What's New" popup's data source, `go:embed`-ed at build time) by walking `git tag -l 'v*' --sort=v:refname` and categorizing commit subjects between each pair of tags via conventional-commit regex (feat/fix/security-scoped/other). **Depends only on real `v*` tags existing in git history** — it does not read `auto-versioning.yml`'s or `release-goreleaser.yml`'s output, the GitHub Releases API, or any state those workflows produce. | **Fully decoupled from this migration** as long as release-please still creates real, plain-`v*`-prefixed git tags (Decision 1's tag-naming requirement is what makes this true). Called from `nightly-build.yml` (line 225) and the dead `release-goreleaser.yml` (line 77) — **not** from `docker-build.yml`, meaning stable/`latest`-tagged production images currently always ship the placeholder `[]` changelog while nightly images get real data. This is a **pre-existing gap in `docker-build.yml`, out of scope for this CI/CD-versioning-only migration**, flagged here only so it isn't mistaken for something this migration caused or should fix. |
| `backend/internal/services/update_service.go` (line 40) | Self-update checker hits `https://api.github.com/repos/Wikid82/charon/releases/latest` for `tag_name`/`html_url` only — does not need release assets. | Unaffected: release-please still publishes a `releases/latest`-eligible GitHub Release with those fields. |
| `backend/internal/services/orthrus_service.go` (line ~222) | Orthrus agent "Tarball" install method's `curl` command points at `.../releases/latest/download/charon-agent-linux-amd64.tar.gz`. | **This asset has never existed** (GoReleaser never ran successfully, and even if it had, `.goreleaser.yaml` builds `charon`, not `charon-agent`). This is a pre-existing, already-broken feature, unrelated to and unmade-worse by this migration — flagged for the user's awareness only; fixing it is out of scope (would require actually building/publishing an agent binary, a feature-level change). |
| `CHANGELOG.md` (repo root, 600 lines) | Hand-curated, Keep-a-Changelog-format, rich multi-line entries with issue references. Not written by any current CI workflow. | **Conflicts with release-please's default "go" strategy**, which manages `CHANGELOG.md` by prepending auto-generated entries. See Additional Finding / Decision below — resolved via `skip-changelog: true`. |
| `renovate.json` | No references to `goreleaser`/`release-goreleaser`/`release-drafter` by name — Renovate discovers pinned actions generically by scanning workflow YAML. | No Renovate config changes needed; removing `release-goreleaser.yml` and `auto-changelog.yml` simply removes those pinned actions from Renovate's future PRs. |
| `.gitignore` (line ~163-167) | Dedicated `# GoReleaser` section ignoring `dist/`. | Remove alongside `.goreleaser.yaml` (Commit 5). |
| `.dockerignore` (line 13) | Lists `.goreleaser.yaml`. | Remove alongside `.goreleaser.yaml` (Commit 5). |
| `codecov.yml` | No workflow-file or release-artifact references in `ignore:`. | No changes needed. |

## Reference Implementation (`/projects/go_notify_yourself`)

- `.release-please-manifest.json`: `{".": "0.2.0"}`.
- `.github/workflows/release-please.yml`: `googleapis/release-please-action@45996ed1f6d02564a971a2fa1b5860e934307cf7 # v5`, `on: push: branches: [main]`, `permissions: contents: write, pull-requests: write`, `config-file`/`manifest-file` inputs.
- `release-please-config.json`: **as of this session, corrected to `"release-type": "go"`** (it was `"node"` earlier in the session, before the sibling repo's own config was independently fixed — this happened to land on the exact same conclusion this plan's research below reaches independently via release-please's own docs, which is reassuring cross-validation, not something to blindly trust as "already proven correct"). The repo has no `package.json`, so `"node"` would have been wrong (release-please would look for a manifest file that doesn't exist).

# Six Required Decisions

## Decision 1 — `release-type` for `release-please-config.json`

**Decision: `"release-type": "go"`.**

Verified directly against release-please's documented strategy table (`docs/customizing.md` release-type list, fetched this session):

| release-type | Description (verbatim from docs) |
|---|---|
| `go` | "A repository with a CHANGELOG.md" |
| `simple` | "A repository with a version.txt and a CHANGELOG.md" |
| `node` | "A Node.js repository, with a package.json and CHANGELOG.md" |

`simple` was the task brief's suggested candidate to verify — it is **not** correct: it manages a `version.txt` manifest file, which Charon does not have and does not want (per the pre-verified finding: no in-repo version manifest exists, version comes from git tag → Docker build-arg → Go ldflags only). `go` is the only listed strategy requiring **zero** manifest file — exactly Charon's situation, and it still manages `CHANGELOG.md` (mitigated via `skip-changelog: true`, see Additional Finding below) and still creates git tags + GitHub Releases from Conventional Commits.

`node` (the sibling repo's original, now-corrected choice) would have required release-please to manage a `package.json` that doesn't exist in either repo — confirmed by direct filesystem check of `/projects/go_notify_yourself` (no `package.json` present). Not applicable to Charon either.

## Decision 2 — Downstream consumers of the tag/Release that `auto-versioning.yml` currently produces

Read in full: `docker-build.yml`, `orthrus-build.yml`, `auto-changelog.yml`, `nightly-build.yml`, `update_service.go`, `orthrus_service.go`, `generate-changelog.sh`.

| Consumer | Trigger | Depends on |
|---|---|---|
| `orthrus-build.yml` | `push: tags: ['v*']` | **Real dependency.** Must keep firing on the same bare `v<semver>` tag pattern; its `docker/metadata-action` step derives `type=semver` Docker tags directly from the pushed tag ref. |
| `docker-build.yml` | Branch push only, never tags | **No dependency** — confirmed dead/inert `type=semver` patterns in its own `docker/metadata-action` config (never actually reached because `TRIGGER_REF` is always a branch ref). Nothing to preserve here beyond not breaking branch-push behavior, which this migration doesn't touch. |
| `release-goreleaser.yml` | `push: tags: ['v*']` | **No real dependency** — never succeeded, produces nothing (Decision 5). |
| `auto-changelog.yml` | `workflow_run` + `release: types: [published]` | Redundant, not load-bearing for anything else — see Additional Finding. |
| `update_service.go` | Polls `releases/latest` REST endpoint | Needs *a* GitHub Release to exist with `tag_name`/`html_url` — release-please provides this natively. |
| `generate-changelog.sh` (via `nightly-build.yml`) | Reads `git tag -l 'v*'` directly | Needs real `v*` tags in git history — release-please provides this as long as tag-naming is kept bare (Decision 1's `include-component-in-tag: false`, see risk below). |

**Conclusion**: the only workflow with a genuine, live functional dependency on the tag is `orthrus-build.yml`, and it only needs the tag to (a) exist, (b) match glob `v*`, (c) parse as semver for the `docker/metadata-action` `type=semver` patterns. Release-please satisfies all three by default, provided tag-naming is pinned explicitly (see risk callout in Technical Specifications).

## Decision 3 — Interaction with the nightly→main "merge commit only" rule

**Decision: keep the CLAUDE.md rule unchanged. Do not relax it, and do not propose editing CLAUDE.md.**

Investigated directly against release-please's docs (`docs/design.md`, GitHub README commit-conventions section, fetched this session):

- Release-please's own docs state it **"highly recommends"** squash-merging *feature* PRs into a linear history, and that it discovers releasable commits by **iterating backwards through actual git commits** (not by re-parsing PR bodies or bullet-ized squash-commit text) until it hits a known prior release SHA.
- This is superficially the opposite of Charon's current constraint (which exists specifically to keep bullet-per-commit squash bodies parseable by `paulhatch/semantic-version`'s regex). But the underlying *mechanism* that makes the constraint necessary is unchanged: the weekly `nightly` → `main` promotion PR itself accumulates a full week of already-individually-squashed feature commits. If that promotion PR were **squash-merged** into `main`, all of that week's discrete `feat:`/`fix:` commits would collapse into a single commit on `main` whose own subject line is whatever GitHub picks for the squash (typically the PR title, not necessarily a clean Conventional Commit type) — release-please's per-commit git-log walk would then see **one** commit for the entire week, not one-per-change, and lose the same granularity that currently breaks `paulhatch`. Using **"Create a merge commit"** for the promotion PR preserves each week's individual squashed-per-feature commits as distinct commits in `main`'s history, which is exactly what release-please's commit-by-commit walk needs to correctly attribute each `feat:`/`fix:` to the right release.
- **Conclusion**: the rule's *justification* text in CLAUDE.md ("squash merging collapses all commits into bullet lines that the auto-versioning workflow cannot parse") becomes slightly inaccurate wording once `paulhatch` is gone, but the *rule itself* remains equally necessary under release-please, for an adjacent reason. This plan does **not** propose editing CLAUDE.md's rule or its wording — flagging the wording-vs-mechanism nuance here is for the user's own future reference only, per the task's instruction not to propose CLAUDE.md edits.

## Decision 4 — Fate of `.version` and `scripts/check-version-match-tag.sh`

**Decision: remove both, plus the `check-version-match` lefthook hook entry (`lefthook.yml` line ~99) and the now-pointless `.github/skills/utility-version-check*` skill wrapper.**

Reasoning:
- `.version` is already stale (`v0.27.0` vs. real latest tag `v0.36.5`) and already documented in `VERSION.md` as "optional... not the canonical release trigger" — it carries no functional weight today.
- `scripts/check-version-match-tag.sh` is **already self-deprecated in its own source** (prints a warning telling callers to use the skill-runner instead) — but `.github/skills/utility-version-check-scripts/run.sh` just `exec`s this same script, so the "recommended" migration path is circular dead code, not an actual alternative implementation.
- Release-please replaces the entire concept this check exists for: `.release-please-manifest.json` becomes the single source of truth for "what version are we at," continuously kept in sync with tags by release-please itself. A hand-run parity check between a stale flat-text file and `git tag` adds no safety release-please doesn't already provide, and having *two* "canonical" version records (`.release-please-manifest.json` and `.version`) invites exactly the kind of drift the check script exists to catch.
- The check is non-blocking today (`exit 0` when `.version` is absent, per the script's own logic), so removing the file cannot regress any currently-enforced gate.

**Full blast radius** (repo-wide `grep -rln "utility-version-check\|check-version-match-tag"`, re-verified against Supervisor's independent review):

| Reference | Action |
|---|---|
| `.github/skills/utility-version-check-scripts/run.sh`, `.github/skills/utility-version-check.SKILL.md`, `scripts/check-version-match-tag.sh`, `.version`, `lefthook.yml`'s `check-version-match` hook | Delete/remove — already in original Deleted/Modified Files scope. |
| `.github/skills/README.md:72` (Utility Skills table row) and `:267` (kebab-case naming example, `utility-version-check` bullet) | **Now added to this plan's scope**: remove both — assigned to Commit 5. |
| `.vscode/tasks.json:694-698` ("Utility: Check Version Match Tag" task, shells out to `skill-runner.sh utility-version-check`) | **Now added to this plan's scope**: remove this task block — assigned to Commit 5. Left in place, it would error every time it's run post-deletion. |
| `.github/skills/utility-bump-beta.SKILL.md:186` ("Related Skills" cross-link to `utility-version-check.SKILL.md`) | **Now added to this plan's scope**: remove the dead link (keep the rest of that skill's "Related Skills" list intact) — assigned to Commit 5. |
| `CLAUDE.md:261` (Skills table row: `utility-version-check \| Check tool versions`) | **Amended after explicit user sign-off**: the user has explicitly authorized editing this governance row as part of this PR ("include the edit in claude.md. no need to make it a follow-up when it can be done now"), satisfying this plan's own constraint against silently touching CLAUDE.md. **Decision: remove the `utility-version-check` row from CLAUDE.md's Skills table (line 261) in this PR**, folded into Commit 5 alongside the rest of this skill's deletion (contingent on that deletion actually happening in this same commit, which it does — see Commit 5 scope). No longer deferred as a Manual Post-Merge Follow-Up. |

## Decision 5 — Fate of `.goreleaser.yaml` and `release-goreleaser.yml`

**Decision: remove both entirely.**

Confirmed via full reads of `docker-build.yml` and `orthrus-build.yml` (Decision 2) that neither depends on GoReleaser's build/archive/nfpm output — Charon's Docker images are built directly by `docker-build.yml`'s own multi-stage `Dockerfile`, with version injected via ldflags independently and identically to what `.goreleaser.yaml`'s `builds.ldflags` section does (Dockerfile lines ~250-286 vs. `.goreleaser.yaml`'s `builds[0].ldflags`). `.goreleaser.yaml`'s own header comment already self-documents as unused: *"builds, archives, and nfpms... kept for potential future use but are not currently utilized."*

The only *other* thing `release-goreleaser.yml` does is call `scripts/generate-changelog.sh` (line 77) — but that script is independently invoked by `nightly-build.yml` too, and depends only on git tags, not on GoReleaser itself, so removing the workflow does not remove changelog-generation capability from anywhere it currently actually runs.

**No nfpm/package-distribution consumer exists** — grep across `docs/`, `.github/workflows/`, and `Dockerfile` for `nfpm`/`.deb`/`.rpm` distribution steps outside `.goreleaser.yaml` itself returns nothing; Charon ships exclusively as Docker images per `ARCHITECTURE.md`'s stated deployment model.

## Decision 6 — Fate of the "PR-2 release promotion guard"

**Decision: the gate is a deliberate, purpose-built temporary safety mechanism (not a misconfigured accident), whose guarded purpose has since been satisfied and whose host workflow is being retired anyway — so it is removed as a natural consequence of Decision 5, not silently dropped on its own merits.**

Evidence trail:
- `git log -S"Enforce PR-2 release promotion guard"` traces the gate's introduction to commit `834b27f2` / `45458df1`, `"chore: Add Caddy compatibility gate workflow and related scripts; enhance SMTP settings tests"`, dated 2026-02-23. That same commit also adds `.github/workflows/caddy-pr1-compat.yml` and `docs/reports/caddy-pr1-compatibility-matrix.md`.
- `docs/reports/caddy-security-posture.md`, also dated 2026-02-23, is explicitly titled **"PR-2 Security Patch Posture and Advisory Disposition"** — a Caddy 2.11.x upgrade security review (patch retention/retirement decisions for `expr`, `ipstore`, `nebula`; CVE/GHSA disposition table) that is **unrelated to any numbered pull request in this repo's PR history** — "PR-2" here names a phase of a specific historical security workstream (Caddy version-bump security review), not a generic or accidental label.
- That doc's own closure statement: *"PR-2 posture decisions are review-ready: patch disposition is explicit, admin API assumptions are enforced, and rollback remains deterministic."* — i.e., the gate's guarded condition (finish the Caddy PR-2 security review before letting GoReleaser cut a publishable release) **was satisfied the same day the gate was added.**
- Nobody ever flipped `CHARON_PR2_GATES_PASSED=true` afterward — confirmed via `REPO_VARS_JSON: {}` on every subsequent `release-goreleaser.yml` run. This is most plausibly an oversight (the review closed, but the repo-variable flip was a separate manual step nobody circled back to) rather than a deliberate ongoing hold, since the closure doc gives no indication the gate was meant to stay engaged indefinitely.

**Recommendation for the user**: this is not evidence of a security requirement that needs to be re-implemented elsewhere — it was scoped to one specific, already-closed security review. Removing it alongside the rest of `release-goreleaser.yml` is safe. If the user wants a similar "hold releases pending a security sign-off" mechanism for *future* security reviews, that would be a new, forward-looking control to design separately — explicitly flagged here as a possible follow-up, not something this plan implements.

**A second, independent argument for removal, stronger than the intent-inference above**: even setting aside whether the Caddy-review closure was "meant" to release the gate, Decision 5 establishes that GoReleaser will **never again attempt to publish anything** — the workflow it lives in is being deleted outright, not merely disabled. A gate that guards an action which no longer exists has nothing left to guard, independent of any judgment call about the gate's original intent or whether it was ever properly released. This makes the removal safe on structural grounds alone, not just on the historical-intent grounds argued above.

# Additional Findings Beyond the Six Required Points

These surfaced during the mandated research and materially affect the design, so they're resolved here rather than left implicit.

## A. `auto-changelog.yml` / `.github/release-drafter.yml` are redundant with release-please

Not named in the original task brief, but directly in-scope: it's a `.github/workflows/*` file in the exact pipeline being migrated, and it will actively conflict with release-please if left running (both listen on `release: types: [published]`-adjacent events and both try to own "the changelog for this release"). `release-drafter`'s own tag-template (`v$NEXT_PATCH_VERSION`) already never bumps minor/major regardless of label — a pre-existing bug, further weakening the case for keeping it.

**Decision: remove `.github/workflows/auto-changelog.yml` and `.github/release-drafter.yml` in this PR.** Release-please's standing release PR (with its auto-updated body) replaces the "always-fresh draft changelog" function these two files provide.

## B. `CHANGELOG.md` conflict

Charon's root `CHANGELOG.md` is hand-curated (Keep a Changelog format, multi-line rich entries, issue cross-references, 600 lines of history). Release-please's `go` release-type, by default, prepends its own auto-generated entries to whatever file `changelog-path` points at (default `CHANGELOG.md`).

**Decision**: set `"skip-changelog": true` on the `"."` package in `release-please-config.json`. Per the release-please JSON schema (`schemas/config.json`, fetched this session): *"Skip generating a changelog for this package. Defaults to `false`."* This stops release-please from touching `CHANGELOG.md` at all, preserving the existing hand-curated file untouched. GitHub Release notes generation is understood to be independent of the changelog-file-write path (the Release body is built from the same underlying commit grouping, separately from whether it's also written to a file) — **this exact interaction is not explicitly documented** in the pages fetched this session, so it is flagged in Manual Post-Merge Follow-Ups as something to positively confirm on the first real release-please run (does the created GitHub Release still get a populated body with `skip-changelog: true`?).

## C. Tag-naming default is a real breakage risk — must be pinned explicitly

Per the release-please JSON schema (fetched this session): `include-component-in-tag` **defaults to `true`** ("When tagging a release, include the component name as part of the tag"). For a single non-monorepo package at `"."`, this risks producing a tag like `charon-v0.37.0` instead of the bare `v0.37.0` every existing consumer expects (`orthrus-build.yml`'s `tags: ['v*']` trigger, `generate-changelog.sh`'s `git tag -l 'v*'` scan, `update_service.go`'s expectations, and every pre-existing tag in the repo's own history back to `v0.1.0`-style tags).

**Decision**: explicitly set `"include-component-in-tag": false` in `release-please-config.json`. Do not rely on whatever component-name-derivation-for-an-unnamed-root-package default behavior release-please falls back to — pin it. Flagged as a **must-verify-on-first-live-run** item (see Manual Post-Merge Follow-Ups): confirm the first release-please-created tag is exactly `v<semver>`, no prefix/suffix.

## D. Pre-1.0 major-version-bump behavior must be pinned explicitly

`auto-versioning.yml`'s current design deliberately disables automatic major-version bumps ("Major version bumps are intentionally disabled in automation to prevent accidents" — its own header comment; `major_pattern` is set to a regex that can never match). Release-please instead supports major bumps automatically via `!` suffix or `BREAKING CHANGE:` footer conventions, gated pre-1.0 by two schema options (`bump-minor-pre-major`, `bump-patch-for-minor-pre-major`) whose **schema definitions carry no explicit documented default** (confirmed by direct inspection of `schemas/config.json` this session — both properties have a `description` but no `default` key shown).

**Decision**: set both `"bump-minor-pre-major": true` and `"bump-patch-for-minor-pre-major": true` explicitly in `release-please-config.json`, regardless of what the undocumented actual default turns out to be. This guarantees a `feat!:`/`BREAKING CHANGE:` commit at the current `v0.36.5` bumps to `v0.37.0` (matching the existing "major bumps require a deliberate manual tag, never automatic" philosophy) rather than silently jumping to `v1.0.0`. This is materially safer than trusting an unconfirmed default and is worth the two explicit lines.

## E. `chore:`-scoped commits will not trigger a Docker build on `main`

`docker-build.yml`'s `setup` job (lines ~170-173) already skips the real build when the head commit or PR title matches `^chore:` or `^chore\(deps`. Since every commit in this PR is `chore:`-scoped (pure CI/CD config), merging it to `main` will not trigger a Docker build — expected and desired, not a gap to fix.

# Technical Specifications

## New Files

### `release-please-config.json` (repo root)

```json
{
  "$schema": "https://raw.githubusercontent.com/googleapis/release-please/main/schemas/config.json",
  "release-type": "go",
  "include-component-in-tag": false,
  "bump-minor-pre-major": true,
  "bump-patch-for-minor-pre-major": true,
  "pull-request-header": "Here's what's new in Charon",
  "pull-request-footer": "Merge this PR to cut the release.",
  "packages": {
    ".": {
      "skip-changelog": true
    }
  }
}
```

### `.release-please-manifest.json` (repo root)

```json
{
  ".": "0.36.5"
}
```

Seeded to the real latest tag at plan-authoring time (`git tag --sort=-v:refname | head -1` → `v0.36.5`). **Implementer note**: re-run that command immediately before implementation and use whatever the actual latest tag is at that time — do not blindly copy `0.36.5` if additional tags have landed on `main` since this plan was written.

### `.github/workflows/release-please.yml`

```yaml
name: release-please

on:
  push:
    branches: [main]

permissions:
  contents: write
  pull-requests: write

jobs:
  release-please:
    runs-on: ubuntu-latest
    steps:
      - uses: googleapis/release-please-action@45996ed1f6d02564a971a2fa1b5860e934307cf7 # v5
        with:
          config-file: release-please-config.json
          manifest-file: .release-please-manifest.json
```

Mirrors `/projects/go_notify_yourself/.github/workflows/release-please.yml` exactly, including the same pinned SHA. **Implementer note**: verify this SHA still resolves to `v5` (or a newer tag) immediately before implementation — supply-chain pins should reflect the current best-available release, not be copied stale.

## Modified Files

| File | Change |
|---|---|
| `VERSION.md` | Full rewrite: remove "Canonical Release Process (Tag-Derived CI)" section describing `release-goreleaser.yml`/`docker-build.yml` as jointly building release artifacts from the tag (false, per Decision 2/5). Replace with a description of the release-please PR-based flow: commits land on `main` → release-please maintains a standing `chore(main): release X.Y.Z` PR → merging that PR tags + publishes the GitHub Release → `orthrus-build.yml` picks up the tag independently. Remove the "Legacy/Optional `.version` Path" section (Decision 4). Remove the "release-drafter workflow" changelog-generation mention (Additional Finding A). Keep the Container Image Tags / Nightly Versioning Format sections unchanged (still accurate, untouched by this migration). |
| `lefthook.yml` | Remove the `check-version-match: { glob: ".version", run: "bash scripts/check-version-match-tag.sh" }` hook entry (~line 99). |
| `.gitignore` | Remove the `# GoReleaser` section (`dist/` ignore rule, ~lines 163-167). |
| `.dockerignore` | Remove the `.goreleaser.yaml` line (~line 13). |
| `ARCHITECTURE.md` | **Was missing from the first draft of this plan — added per Supervisor review.** Per CLAUDE.md's own rule ("update `ARCHITECTURE.md` when making changes to... deployment model... integration points"), and because it explicitly names a file this plan deletes: the "Release Workflow" section (~lines 1464-1479) opens with a 10-step "Automated Release (GitHub Actions)" list whose step 1 is `"Trigger: Push tag v1.2.0"` — this describes the tag-push-triggers-everything model this migration retires (steps 2-8 describe `docker-build.yml`'s real, still-accurate build/scan/sign/publish pipeline, which is **not** actually tag-triggered per Decision 2's finding — this section conflates `docker-build.yml`'s branch-push pipeline with a tag-triggered release flow that doesn't exist as described). Line 1479 states `scripts/generate-changelog.sh` "runs during the same `release-goreleaser.yml` workflow" — that workflow is deleted by this PR (the script also independently runs from `nightly-build.yml`, per Research Findings, so this line is doubly inaccurate even before this migration). **Rewrite scope**: replace the "Trigger: Push tag" framing with the release-please push-to-main → standing-PR → merge-to-tag flow (mirror the Data Flow diagram below); correct the `generate-changelog.sh` sentence to name `nightly-build.yml` (its one remaining real caller) instead of the deleted workflow; leave steps 2-8's description of `docker-build.yml`'s actual build/scan/sign/publish pipeline untouched (still accurate, just re-anchor which trigger kicks it off). Assigned to Commit 6. |

## Deleted Files

| File | Reason |
|---|---|
| `.github/workflows/auto-versioning.yml` | Replaced by `release-please.yml` (Decision 1-3). |
| `.github/workflows/release-goreleaser.yml` | Never succeeded; no downstream consumer (Decision 5). |
| `.goreleaser.yaml` | Unused build/archive/nfpm config; version injection already duplicated in `Dockerfile` directly (Decision 5). |
| `.github/workflows/auto-changelog.yml` | Redundant with release-please's own PR/changelog mechanism (Additional Finding A). |
| `.github/release-drafter.yml` | Config for the file above; removed alongside it. |
| `.version` | Stale, non-canonical, superseded by `.release-please-manifest.json` (Decision 4). |
| `scripts/check-version-match-tag.sh` | Self-deprecated, circularly "replaced" by a wrapper that calls it, superseded by release-please's manifest (Decision 4). |
| `.github/skills/utility-version-check-scripts/run.sh` | Wraps the script above; dead once its target is gone. |
| `.github/skills/utility-version-check.SKILL.md` | Documents the now-removed skill. |

**Partial edits (file kept, dead reference removed)** — full detail and reasoning in Decision 4 and the Commit Slicing Strategy's Commit 5 row; listed here only as a pointer so this table isn't mistaken for the complete blast radius:

| File | Edit |
|---|---|
| `.github/skills/README.md` | Remove the `utility-version-check` row (~line 72) and naming-example bullet (~line 267). |
| `.vscode/tasks.json` | Remove the "Utility: Check Version Match Tag" task block (~lines 694-698). |
| `.github/skills/utility-bump-beta.SKILL.md` | Remove the dead `utility-version-check` cross-link (~line 186). |
| `scripts/generate-changelog.sh` | Fix the stale header-comment reference to `release-goreleaser.yml` (line 5) — points at `nightly-build.yml` instead. |
| `CLAUDE.md` | **Edited by this PR, per explicit user authorization** — remove the `utility-version-check` Skills-table row (line 261). Folded into Commit 5. |

## Data Flow: Before vs. After

**Before:**
```
push to main (non-chore commit)
  -> docker-build.yml runs (branch push trigger)
    -> auto-versioning.yml runs (workflow_run: on docker-build.yml completion)
      -> paulhatch/semantic-version computes next tag
      -> shell/grep builds release body from commit messages
      -> softprops/action-gh-release creates tag + GitHub Release
        -> release-goreleaser.yml runs (tag push trigger) -> FAILS at PR-2 gate, publishes nothing
        -> orthrus-build.yml runs (tag push trigger) -> builds + publishes semver-tagged orthrus image
        -> auto-changelog.yml runs (release published trigger) -> release-drafter updates a draft release
```

**After:**
```
push to main (any commit)
  -> docker-build.yml runs (branch push trigger, skips build body if chore:) [unchanged]
  -> release-please.yml runs (push trigger, independent of docker-build.yml)
      -> release-please-action opens/updates a standing "chore(main): release X.Y.Z" PR
         (accumulates all releasable commits since the last tag; no PR yet if nothing releasable)

[separately, whenever a human/bot merges that standing release PR]
  -> release-please-action creates the git tag (v<version>, bare per include-component-in-tag:false)
  -> release-please-action creates the GitHub Release (skip-changelog:true, so CHANGELOG.md untouched)
    -> orthrus-build.yml runs (tag push trigger) -> builds + publishes semver-tagged orthrus image [unchanged]
    -> generate-changelog.sh (next nightly-build.yml run) picks up the new tag via `git tag -l 'v*'` [unchanged]
```

## Error Handling / Edge Cases

| Scenario | Behavior |
|---|---|
| No releasable commits since last tag (only `chore:`/`docs:`/`test:`/`ci:`/`build:`/`style:`/`refactor:` land on `main`) | Release-please does not open/update a release PR at all. **Behavior change from today**: `auto-versioning.yml` currently bumps patch for *any* non-`feat:` commit, so today a `chore:`-only week still cuts a release; under release-please, it won't. This is a deliberate, disclosed change (arguably a correctness improvement — no more "no-op" patch releases), not an oversight. Flagged for user sign-off. |
| `feat!:`/`BREAKING CHANGE:` commit lands pre-1.0 | Bumps minor, not major, per Decision/Additional-Finding D's explicit config. |
| Release PR sits open for a long time while more commits land | Release-please updates the existing PR's body/diff in place (its documented standard behavior) — no duplicate PRs. |
| Someone merges the release PR via squash instead of the "Create a merge commit"/default GitHub merge release-please expects | Not explicitly tested in this plan (CI-config-only, no live GitHub run possible locally) — flagged in Manual Post-Merge Follow-Ups as the first thing to verify by watching the first real release PR merge. |
| `.release-please-manifest.json` drifts from the real latest tag (e.g., someone force-pushes a tag manually) | Release-please reads the manifest as its source of truth for "last released version," not live tag state — a manual tag push outside release-please's flow would desync them. Document this in `VERSION.md`'s rewrite as "don't manually tag `v*` releases going forward; let release-please do it." |

# Implementation Plan

This is a CI/CD-configuration-only change: no Go code, no TypeScript/React code, no database migrations, no API surface. Phases below are adapted accordingly from the standard template.

## Phase 1: Playwright Tests (spec behavior) — N/A

No user-facing behavior changes; nothing to spec as `test.fixme`. Explicitly out of scope — see "CLAUDE.md Definition-of-Done Applicability" below.

## Phase 2: Backend Implementation — N/A

No `backend/` changes.

## Phase 3: Frontend Implementation — N/A

No `frontend/` changes.

## Phase 4: CI/CD Configuration Changes (replaces "Integration and Testing" for this chore)

- GOAL-001: Stand up the release-please config/manifest/workflow and validate every JSON/YAML file for syntactic correctness and internal consistency (manifest version matches real latest tag; config's package key matches manifest's package key).

| Task | Description | Completed | Date |
|------|-------------|-----------|------|
| TASK-001 | Create `release-please-config.json` per Technical Specifications, with `include-component-in-tag: false`, `bump-minor-pre-major: true`, `bump-patch-for-minor-pre-major: true`, `skip-changelog: true` all set explicitly. | | |
| TASK-002 | Create `.release-please-manifest.json`, seeded to the actual latest `v*` tag at implementation time (re-verify, don't copy `0.36.5` blindly). | | |
| TASK-003 | Create `.github/workflows/release-please.yml`, pinned-SHA `googleapis/release-please-action`, `push: branches: [main]` trigger, `contents: write` + `pull-requests: write` permissions. | | |
| TASK-004 | Validate all three new/changed files with `jq empty` (JSON) / a YAML parser (`yamllint` or `python -c "import yaml,sys; yaml.safe_load(open(sys.argv[1]))"`) before commit. | | |

## Phase 5: Retirement + Documentation

- GOAL-002: Remove the superseded workflows/config/scripts and rewrite `VERSION.md` to describe the new flow accurately.

| Task | Description | Completed | Date |
|------|-------------|-----------|------|
| TASK-005 | Delete `.github/workflows/auto-versioning.yml`, `.github/workflows/release-goreleaser.yml`, `.goreleaser.yaml`. | | |
| TASK-006 | Delete `.github/workflows/auto-changelog.yml`, `.github/release-drafter.yml`. | | |
| TASK-007 | Delete `.version`, `scripts/check-version-match-tag.sh`, `.github/skills/utility-version-check-scripts/run.sh`, `.github/skills/utility-version-check.SKILL.md`; remove the `check-version-match` entry from `lefthook.yml`. | | |
| TASK-008 | Remove the `# GoReleaser` section from `.gitignore`; remove the `.goreleaser.yaml` line from `.dockerignore`. | | |
| TASK-009 | Rewrite `VERSION.md` per Technical Specifications. | | |
| TASK-010 | Run `lefthook run pre-commit` to confirm no hook references a now-deleted file/glob and everything still passes. | | |

# CLAUDE.md Definition-of-Done Applicability

This PR is CI-config/YAML/JSON/Markdown-only. Mapped explicitly against the standard DoD:

| DoD Item | Applies? | Notes |
|---|---|---|
| 1. Playwright E2E Tests | **N/A** | No user-facing behavior; no frontend/backend code path changes. |
| 1.5. GORM Security Scan | **N/A** | No `backend/internal/models/**`, no GORM queries/migrations touched. |
| 2. Local Patch Coverage Preflight (`scripts/local-patch-report.sh`) | **Run it anyway** | It's mandatory regardless of change type per CLAUDE.md; expect it to report ~0% "patch" surface since no `.go`/`.ts`/`.tsx` lines are touched — confirm it doesn't error out on a code-less diff rather than skip it. |
| 3. Security Scans (CodeQL/Trivy) | **Defer to CI** | This is a `chore:`-scoped change with no new application code path — per CLAUDE.md's own rule ("Defer to CI for fix/test/chore/refactor-scoped changes with no new feature surface"), do not run these locally; CI runs both unconditionally regardless. **Caveat**: neither CodeQL nor Trivy actually provides coverage for this PR's one genuinely new risk surface — `googleapis/release-please-action` is a new third-party Action granted `contents: write` + `pull-requests: write` on this repo. CodeQL scans Go/JS source; Trivy scans container images/dependencies; neither evaluates GitHub Actions permission scopes or third-party Action supply-chain trust. "Defer to CI" is accurate for what those two tools actually check, but should not be read as "this PR's permissions posture is covered" — the mitigation here is the existing pinned-SHA convention (matching every other Action reference in this repo) plus the fact that the sibling repo already runs the identical Action/permissions combination without incident, not CodeQL/Trivy. |
| 4. Lefthook Triage | **Applies** | Run `lefthook run pre-commit` — should no-op past YAML/JSON formatting-class hooks (no `.go`/`.ts` glob matches), but must still pass cleanly, especially after removing the `check-version-match` hook entry (verify lefthook config itself is still valid YAML). |
| 5. Staticcheck | **N/A** (no-op) | No `.go` files touched; hook glob won't match anything. |
| 6. Coverage Testing (85% backend/frontend) | **N/A** | No code touched; nothing for `go-test-coverage.sh`/`frontend-test-coverage.sh` to measure against this diff. |
| 7. Type Safety (`npm run type-check`) | **N/A** | No `.ts`/`.tsx` files touched. |
| 8. Verify Build (`go build`, `npm run build`) | **Recommended as a sanity check, not a real gate** | Neither build path is touched by this diff; running them just confirms the repo wasn't already broken. Not blocking for this PR specifically. |
| 9. Fixed/New Code Testing | **N/A** | No unit-testable code changed. |
| 10. Clean Up (debug prints, dead code) | **Applies in spirit** | The primary output of this PR *is* dead-code removal (Decisions 4-6, Additional Finding A) — this is effectively the main content of the PR, not a final pass. |

**The real gates for this PR** (called out explicitly per the task brief, since the standard DoD doesn't fit a CI-config change well):
- Every new/modified JSON file parses (`jq empty release-please-config.json .release-please-manifest.json`).
- Every new/modified YAML file parses (`.github/workflows/release-please.yml`, and re-validate `lefthook.yml` after the hook-entry removal).
- `lefthook run pre-commit` passes cleanly end-to-end.
- **GitHub Actions workflow behavior (does `release-please.yml` actually open a correct PR, does merging it actually tag+release correctly, does `orthrus-build.yml` actually still fire on that tag) can only be fully verified by a live run on GitHub after merge — not locally, not in this plan.** This is the single biggest residual-risk category for this PR and is why the Manual Post-Merge Follow-Ups section below exists.

# Manual Post-Merge Follow-Ups (cannot be validated without a live push to GitHub)

1. **Confirm repo-level permissions**: verify `pull-requests: write` is actually honored for the `GITHUB_TOKEN` used by Actions in this repo (Settings → Actions → General → Workflow permissions). If the repo/org default is read-only, the `release-please.yml` workflow's explicit `permissions:` block should override it, but confirm on the first run rather than assume.
2. **Watch the first `release-please.yml` run on `main`** after this PR merges: confirm it either (a) opens a `chore(main): release X.Y.Z` PR if there are releasable commits since `v0.36.5`, or (b) does nothing cleanly if there aren't — don't assume silence means broken.
3. **Verify tag format on the first real release**: confirm the tag release-please creates is exactly `v<semver>` (no `charon-` prefix) — this is the `include-component-in-tag: false` risk called out in Additional Finding C. If it's wrong, `orthrus-build.yml`'s `tags: ['v*']` trigger and `generate-changelog.sh`'s `git tag -l 'v*'` scan both silently stop matching.
4. **Verify `CHANGELOG.md` is untouched** by the first release-please PR (confirms `skip-changelog: true` behaves as expected) **and** verify the resulting GitHub Release still gets a populated body (confirms release-notes generation is independent of the changelog-file write path — this exact interaction wasn't found explicitly documented during this session's research).
5. **Verify `orthrus-build.yml` still fires** on the first release-please-created tag and produces the expected `type=semver` Docker tags.
6. **Branch protection interaction**: check whether `main`'s branch protection rules (required reviews, required status checks) block release-please's own bot-authored release PR from being merged, and if so, decide whether to exempt it or just merge manually each time (release-please doesn't require any special exemption to function — it just opens a normal PR).
7. **Decide who merges the release PR and how** (manual click each time vs. some auto-merge label) — this plan does not configure auto-merge; that's a deliberate choice left to the user, since it directly controls when a real release goes out. This is the operational follow-through on the "User Decisions Required Before Implementation" #1 go/no-go above: if the user decides fully-automatic cadence is a hard requirement, a follow-up PR adding a release-please auto-merge label + workflow would be needed — not something this plan implements.
8. **Clean up historical run records** (optional): the deleted workflows' historical Action run logs remain visible under "Actions" until manually deleted/archived if desired — cosmetic only, not required.
9. **Sibling-repo note**: `/projects/go_notify_yourself/release-please-config.json` was independently corrected to `"release-type": "go"` during this session (see Reference Implementation section) — no action needed here, noted only so the user isn't surprised by the diff if they look at that repo later.

# Acceptance Criteria

- [ ] `release-please-config.json`, `.release-please-manifest.json`, and `.github/workflows/release-please.yml` exist, are valid JSON/YAML, and match the Technical Specifications section (including the explicit `include-component-in-tag`, `bump-minor-pre-major`, `bump-patch-for-minor-pre-major`, `skip-changelog` settings).
- [ ] `.github/workflows/auto-versioning.yml`, `.github/workflows/release-goreleaser.yml`, `.goreleaser.yaml`, `.github/workflows/auto-changelog.yml`, `.github/release-drafter.yml`, `.version`, `scripts/check-version-match-tag.sh`, and the two `utility-version-check` skill files are all deleted.
- [ ] `.github/skills/README.md`, `.vscode/tasks.json`, and `.github/skills/utility-bump-beta.SKILL.md` no longer reference the deleted `utility-version-check` skill (Decision 4 blast-radius items).
- [ ] `CLAUDE.md:261`'s `utility-version-check` Skills-table row is removed in this PR (Commit 5), per the user's explicit authorization to edit CLAUDE.md as part of this migration.
- [ ] `lefthook.yml` no longer references `check-version-match-tag.sh`; `lefthook run pre-commit` passes.
- [ ] `.gitignore` and `.dockerignore` no longer reference GoReleaser artifacts/config.
- [ ] `scripts/generate-changelog.sh`'s header comment no longer references the deleted `release-goreleaser.yml`.
- [ ] `VERSION.md` accurately describes the release-please PR-based flow and no longer references `release-goreleaser.yml`, `.version` as a release trigger, or the release-drafter workflow.
- [ ] `ARCHITECTURE.md`'s "Release Workflow" section accurately describes the release-please push-to-main → standing-PR → merge-to-tag flow and no longer references `release-goreleaser.yml` or a tag-push trigger as step 1.
- [ ] `orthrus-build.yml` and `docker-build.yml` are **not modified** by this PR (both confirmed to need no changes per Decision 2).
- [ ] No Go, TypeScript, or database-schema files are touched.
- [ ] PR is opened from `chore/release-please-migration` against `main` (not `development`), with the branching deviation explicitly called out in the PR description per the Branching Note above, and merged via squash merge (per the Merge Strategy note in the Introduction).
- [ ] All commits use `chore:` (or `chore(ci):`) Conventional Commit prefixes, matching CLAUDE.md's CI-trigger convention (so this PR's own merge does not trigger a Docker build).
- [x] The user has explicitly signed off on both items in "User Decisions Required Before Implementation" (manual release-cadence gating: APPROVED; chore-only-weeks/`perf:`-no-longer-releasable: ACCEPT release-please defaults, no special-case override) — see that section for the recorded decisions.
- [x] The user has explicitly authorized editing `CLAUDE.md` as part of this PR to remove the stale `utility-version-check` Skills-table row (line 261), folded into Commit 5 — see Decision 4's blast-radius table.

# Commit Slicing Strategy

**Decision: single PR, `chore/release-please-migration` → `main`, with ordered logical commits.** Per CLAUDE.md's "One Feature = One PR" rule — this is one cohesive infrastructure change and must not be split across multiple PRs (e.g., "add release-please" in one PR and "remove old workflows" in another would leave the repo running two competing release mechanisms simultaneously in the gap between merges, which is strictly worse than doing it atomically).

| Commit | Scope | Files | Depends on | Validation gate |
|---|---|---|---|---|
| **1** | Add release-please config + manifest (no workflow yet — inert until Commit 2) | `release-please-config.json`, `.release-please-manifest.json` | — | `jq empty release-please-config.json .release-please-manifest.json`; manually diff the manifest's seed version against `git tag --sort=-v:refname \| head -1` to confirm it's current. |
| **2** | Add the release-please workflow | `.github/workflows/release-please.yml` | Commit 1 | YAML parses; `lefthook run pre-commit` passes; manual read-through confirming the pinned action SHA/tag comment matches the sibling repo's convention and is a real, current release. |
| **3** | Retire the superseded auto-versioning/GoReleaser pipeline | `.github/workflows/auto-versioning.yml` (delete), `.github/workflows/release-goreleaser.yml` (delete), `.goreleaser.yaml` (delete), `.gitignore` (remove GoReleaser section), `.dockerignore` (remove `.goreleaser.yaml` line), `scripts/generate-changelog.sh` (fix stale header-comment reference at line 5 from `release-goreleaser.yml` to `nightly-build.yml`, its one remaining real caller) | Commits 1-2 (don't remove the old path until the new one exists) | `lefthook run pre-commit` passes; confirm no remaining reference to `.goreleaser.yaml` anywhere (`grep -rn goreleaser --include=*.yml --include=*.md .` minus this plan file itself); confirm `generate-changelog.sh` still runs correctly after the comment-only edit (no functional change, but re-run it locally against a small tag range as a sanity check). |
| **4** | Retire the redundant release-drafter changelog automation | `.github/workflows/auto-changelog.yml` (delete), `.github/release-drafter.yml` (delete) | Commit 2 (release-please must exist as the replacement before removing this) | `lefthook run pre-commit` passes. |
| **5** | Retire the legacy `.version` parity check and its full reference surface | `.version` (delete), `scripts/check-version-match-tag.sh` (delete), `.github/skills/utility-version-check-scripts/run.sh` (delete), `.github/skills/utility-version-check.SKILL.md` (delete), `lefthook.yml` (remove `check-version-match` hook entry), `.github/skills/README.md` (remove the `utility-version-check` row from the Utility Skills table at ~line 72, and the `utility-version-check` bullet from the naming-convention examples at ~line 267), `.vscode/tasks.json` (remove the "Utility: Check Version Match Tag" task block, ~lines 694-698), `.github/skills/utility-bump-beta.SKILL.md` (remove the dead `utility-version-check` cross-link from its "Related Skills" section, ~line 186, keep the rest of that list), `CLAUDE.md` (remove the `utility-version-check` row from the Skills table at ~line 261 — per explicit user authorization to edit CLAUDE.md as part of this PR) | Commit 1 (manifest is the intended replacement source of truth) | `lefthook run pre-commit` passes with no dangling glob/hook referencing a deleted script; confirm `lefthook.yml` and `.vscode/tasks.json` are still valid YAML/JSON respectively; confirm `CLAUDE.md` remains valid Markdown with only the one table row removed, no other governance text touched; `grep -rn "utility-version-check\|check-version-match-tag" .` (excluding this plan file and `.git/`) returns **zero** hits anywhere, including `CLAUDE.md`. |
| **6** | Documentation rewrite | `VERSION.md` (full rewrite per Technical Specifications), `ARCHITECTURE.md` (rewrite the "Release Workflow" section per Modified Files above: replace tag-push-triggers-everything framing with the release-please flow, fix the `generate-changelog.sh`/`release-goreleaser.yml` reference) | Commits 1-5 (must describe the end state, not the transition) | Manual proofread against the final state of every file above; confirm no reference to any deleted file/workflow remains in either doc. |

**Rollback / contingency for the PR as a whole**: since this is entirely additive-then-subtractive CI configuration with no code or schema changes, rollback is a plain `git revert` of the merge commit (or of the whole PR range) — no data migrations, no forward-only state changes are introduced. The one piece of *external* (not-in-git) state this PR's downstream behavior touches is the standing release-please PR itself and any tag it creates after merge; if the migration needs to be rolled back after a real release-please release has already gone out, `.release-please-manifest.json` should be re-seeded to match whatever the real latest tag is at rollback time (not blindly reverted to the pre-migration value), and the old `auto-versioning.yml`/`release-goreleaser.yml` files restored via revert will resume exactly their prior (partially broken) behavior with no additional cleanup needed, since neither of them depended on anything release-please would have introduced.

# Dependencies

- **DEP-001**: `googleapis/release-please-action` (pinned by SHA, `# v5` comment) — new external GitHub Action dependency, matching the pattern already trusted in `/projects/go_notify_yourself`.
- **DEP-002**: No new npm/Go module dependencies. No `package.json`/`go.mod` changes.

# Risks & Assumptions

- **RISK-001**: `include-component-in-tag` default-vs-explicit-`false` mismatch could produce a wrongly-prefixed tag on the first real release, silently breaking `orthrus-build.yml`'s trigger and `generate-changelog.sh`'s tag scan until noticed. Mitigated by explicit config (Additional Finding C) and flagged as the #1 manual-verification item post-merge.
- **RISK-002**: `skip-changelog: true`'s interaction with GitHub Release notes generation is not fully documented in the sources available this session — small chance the Release body comes out empty rather than independently populated. Flagged in Manual Post-Merge Follow-Ups.
- **RISK-003**: Branch protection on `main` could block release-please's bot-authored release PR from merging cleanly (required reviewers, required status checks that don't apply to a docs/manifest-only PR). Flagged in Manual Post-Merge Follow-Ups; no code change can pre-empt this, it must be checked live.
- **RISK-004**: This PR targets `main` directly, bypassing the normal `development` → `nightly` → `main` soak cycle by design (see Branching Note). Slightly higher blast-radius-per-mistake than the repo's usual flow, mitigated by the fact that the change is inert until the *next* real release-worthy commit lands on `main` (release-please won't retroactively do anything to already-tagged history).
- **ASSUMPTION-001**: The latest tag at plan-authoring time (`v0.36.5`) is still the latest tag at implementation time. Re-verify before seeding `.release-please-manifest.json` (explicitly called out as an implementer task, not assumed).
- **ASSUMPTION-002** (revised — the original wording overclaimed completeness; corrected per Supervisor's independent re-run of the same grep, which surfaced two more references this plan now accounts for rather than leaves implicit): a repo-wide grep for `auto-versioning`, `release-goreleaser`, `CHARON_PR2_GATES_PASSED`, `softprops/action-gh-release`, and `paulhatch` does **not** guarantee full coverage of every reference to the files this plan deletes — it only checked those specific literal strings, and it missed `ARCHITECTURE.md` (now added to Modified Files above) and a stale header comment in `scripts/generate-changelog.sh:5` (`"see .github/workflows/release-goreleaser.yml"` — functionally harmless, since the script's actual behavior doesn't depend on GoReleaser, but a dead pointer once that workflow is deleted; corrected as part of Commit 3, since it's tied directly to the GoReleaser removal). Grep-based "nothing else references this" claims in this plan should be read as "no hits for the specific strings searched," not as an exhaustive guarantee — the actual assumption being made is that the Research Findings section's enumerated consumer list is complete, which was cross-checked by Supervisor's independent review and found to need these two additions plus the Decision 4 blast-radius additions above, and no others.

# Related Specifications / Further Reading

- `/projects/go_notify_yourself/release-please-config.json`, `.release-please-manifest.json`, `.github/workflows/release-please.yml` — reference implementation.
- `docs/reports/caddy-security-posture.md` — origin/closure evidence for the "PR-2" gate (Decision 6).
- release-please documentation: `docs/customizing.md`, `docs/design.md`, `schemas/config.json` in `googleapis/release-please` (all fetched and cited directly in this plan).
