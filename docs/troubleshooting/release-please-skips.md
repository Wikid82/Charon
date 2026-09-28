---
title: release-please Silently Skipping or Under-Bumping a Release
description: Diagnosing and recovering when release-please misses real feat/fix commits after a weekly nightly-to-main promotion, either skipping the release entirely or cutting the wrong bump type.
---

## release-please Silently Skipping or Under-Bumping a Release

Symptom: the `release-please` GitHub Actions job runs green after a `Weekly:
Promote nightly to main` merge, but either no release PR is opened, or the
release PR it opens proposes the wrong bump type (e.g. a patch bump when real
`feat:` commits were merged). The job log for the "skip" case ends with:

```text
✔ Splitting 1 commits by path
❯ commits: 1
✔ Considering: 1 commits
✔ No user facing commits found since <last-release-sha> - skipping
```

This happens even when the promotion genuinely brought in `feat:`/`fix:`
commits — the commits are on `main`, they are not yet in any tag, but
release-please never counted them.

### Root cause

release-please's commit-collection walk effectively drops commits once it
reaches the previous release commit's SHA, and the commits it fetches from
GitHub come back ordered by **commit date**, not strict first-parent
topology. This means the walk stops the instant it reaches a commit at or
before the *release boundary's own commit date* — regardless of whether
older, structurally-unreleased commits exist elsewhere in the graph.

The failure condition: a commit's own committer date is **older** than the
date of whatever commit currently sits as the release boundary on `main`,
at the moment it finally lands via a real (non-squash) merge. Concretely,
here is how it happened on 2026-09-28:

1. `feat/auth-rate-limit-1317` merged into `nightly` on **2026-09-25**,
   carrying real `feat:`/`fix(security):` commits dated that day.
2. `nightly` sat un-promoted for several days (weekly cadence).
3. Meanwhile, an unrelated fix landed **directly on `main`** and was
   released as `v0.42.1` on the morning of **2026-09-28** — *before* that
   week's nightly promotion had run.
4. When the weekly promotion merged later that same day, its second-parent
   commits (the real `nightly` work) were now chronologically *older* than
   `v0.42.1`'s own commit timestamp. release-please's date-ordered walk
   reached the `v0.42.1` boundary before it ever paged far enough back to
   see those older, still-unreleased commits, so it silently dropped them.

This is **not** about branch routing — the same stranding can happen to a
long-lived branch merged directly into `main`, if it sits long enough that
an interim release lands first. It is **not** a missing Conventional
Commits prefix either: a structurally identical prior promotion (with an
equally non-conventional merge title) correctly picked up a buried `fix:`
commit, because in that case no interim release had jumped ahead of it.
And it is not fixed by landing spacer commits on `main` afterward — a
spacer only pushes the boundary *later*, widening the date gap rather than
closing it (this was tried on 2026-09-28 via PR #1408 and only produced a
9-commit-late `Set(1)` truncation instead of a full skip — see PR #1410).

The two things that actually matter:

1. **How long does non-squashed work sit before it lands on `main` via a
   real merge?** The longer the gap, the bigger the window for an interim
   release to jump ahead of it.
2. **Sequencing between releases and promotions.** If a same-week
   `release-please` cut is allowed to land on `main` *before* that week's
   pending nightly promotion, anything still waiting in `nightly` is at
   risk of being dated older than the new boundary.

### How to confirm you're hitting this

1. Pull the run's log: `gh run view <run-id> --job <job-id> --log`. Look at
   how many commits it reports considering (`commits: N`) versus how many
   real conventional commits actually exist in the range.
2. Verify real unreleased commits exist and check their dates against the
   boundary's date:
   ```
   git log --format='%H|%cI|%s' <last-release-sha>..origin/main | grep -E '\|(feat|fix|perf|revert)(\(|:)'
   git log -1 --format='%cI' <last-release-sha>
   ```
   Any matching commit with a committer date *older* than the boundary's is
   at risk of being silently dropped, even though
   `git merge-base --is-ancestor <sha> <last-release-sha>` confirms it is
   genuinely new (non-zero exit).

### Recovery

Do not rely on spacer commits or repeated re-runs — the walk is
deterministic on the current git graph and dates, so it will reproduce the
same result. Instead, hand-correct the release-please PR directly:

1. Check out the `release-please--branches--main` branch.
2. Recompute the real commit set and correct bump type:
   ```
   git log --format='%H|%s' <last-release-sha>..origin/main | grep -E '^\w+\|(feat|fix|perf|revert)(\(|:)'
   ```
   `feat:` present → minor bump (this repo has `bump-minor-pre-major: true`);
   only `fix:`/`perf:` → patch.
3. Edit `.release-please-manifest.json` to the corrected version and push a
   `fix:` commit to that branch.
4. Rewrite the PR body/release notes to list the real commits (release-please
   only touches the manifest here since `skip-changelog: true` — there is no
   `CHANGELOG.md` to reconcile), matching the existing bullet format:
   `* <description> ([<short-sha>](.../commit/<sha>))`, with `(#N)` references
   in the subject converted to issue links and `<scope>:` rendered as
   `**scope:**` prefix.
5. Merge the corrected PR.

To prevent recurrence structurally, see the sequencing fix tracked for
`weekly-nightly-promotion.yml` / `release-please.yml` (ensure nightly always
promotes before an interim release is allowed to cut that week).
