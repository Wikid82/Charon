---
title: release-please Silently Skipping a Release
description: Diagnosing and recovering when release-please logs "No user facing commits found" after a weekly nightly-to-main promotion despite real feat/fix commits being merged.
---

## release-please Silently Skipping a Release

Symptom: the `release-please` GitHub Actions job runs green after a `Weekly:
Promote nightly to main` merge, but no release PR is opened, and the job log
ends with:

```text
✔ Splitting 1 commits by path
❯ commits: 1
✔ Considering: 1 commits
✔ No user facing commits found since <last-release-sha> - skipping
```

This happens even when the promotion genuinely brought in `feat:`/`fix:`
commits (rate-limit hardening, dependency fixes, etc.) — the commits are on
`main`, they are not yet in any tag, but release-please never saw them.

### Root cause

`release-please`'s commit walk on `main` stops as soon as it encounters the
previous release commit's SHA. The weekly promotion lands as a real two-parent
merge commit (per the "Create a merge commit" requirement in `CLAUDE.md` —
squash merges break the auto-versioning bullet parser). Its **first** parent
is whatever `main` pointed to before the merge, and its **second** parent is
the tip of `nightly`, which is where the actual `feat:`/`fix:` commits live.

When nothing has landed directly on `main`'s first-parent line between one
release and the next weekly promotion, the previous release commit *is* the
promotion merge's immediate first parent. release-please's history walk
reaches that boundary SHA almost immediately and stops — before it has
paged far enough to descend into the merge's second-parent subtree, so it
never sees the real `nightly` commits at all, only the (non-conventional)
promotion merge subject itself.

Contrast with a promotion where a couple of ordinary commits (a hotfix, a
`chore(main): release` commit, etc.) landed directly on `main` first between
releases: those extra first-parent hops give the walk enough room before it
hits the boundary, and it does surface buried `feat:`/`fix:` commits from the
second-parent side in the same run. This is a topology-adjacency quirk, not a
missing Conventional Commits prefix or a config problem in
`release-please-config.json` / `.release-please-manifest.json`.

### How to confirm you're hitting this

1. Pull the failed run's log: `gh run view <run-id> --job <job-id> --log`.
   Look for `Set(1) { '<sha>' }` (the release boundary) landing as the
   **second** item release-please backfills file lists for, right after the
   promotion merge commit itself.
2. Verify real unreleased commits exist:
   `git log --oneline <last-release-sha>..origin/nightly`, and confirm they
   are **not** ancestors of the last release commit:
   `git merge-base --is-ancestor <sha> <last-release-sha>` (exit non-zero
   means genuinely new).

### Recovery

Land one ordinary commit directly on `main` via a hotfix branch + PR (per
the `CLAUDE.md` branching strategy — never push straight to `main`). This
gives the *next* release-please run one more first-parent hop of room before
it hits the (now-older) release boundary, which is normally enough for it to
also walk into the still-unreleased second-parent commits and open a correct
release PR. If a single spacer commit isn't enough (the buried commits are
very deep), force it instead: dispatch `release-please-action` manually with
an explicit `release-as` version to build the release PR directly, bypassing
the walk.
