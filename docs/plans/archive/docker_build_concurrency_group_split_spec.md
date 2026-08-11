# Plan: Redesign `docker-build.yml` Concurrency Group to Eliminate Residual Push/Pull_request Race

_GitHub Issue: [#1235](https://github.com/Wikid82/Charon/issues/1235) — "Redesign docker-build.yml concurrency group to eliminate residual push/pull_request race"_
_Companion (merged) fix: [PR #1236](https://github.com/Wikid82/Charon/pull/1236) — job-level `pull_request` self-skip for main/development-head PRs_
_Base: `origin/main` @ `6ed08db333d9214f05e9cb49da8d1c2736ffd64e` (merge commit of PR #1236, 2026-08-10 20:32:19Z)_

## 1. Introduction

### 1.1 Objective

`.github/workflows/docker-build.yml` triggers on both `pull_request` (no
branch filter) and `push` (`branches: [main, development]`), plus
`workflow_dispatch` and `workflow_run` (from `Docker Lint`). Its
`concurrency:` group key currently resolves to the **identical string** for a
`push` to `main`/`development` and a `pull_request` whose head branch is
`main`/`development` — the case for bot-generated "propagate main→development"
and weekly nightly→main promotion sync PRs. When both fire for the same
commit, GitHub's concurrency subsystem cancels whichever run registers second,
independent of which one is "supposed" to win.

PR #1236 added a job-level `if:` skip so the `pull_request`-side run
self-resolves to a clean `skipped` conclusion instead of a misleading
`cancelled` red X — but **only in the case where `pull_request`'s `setup` job
happens to evaluate after `push` has already registered its run**. GitHub
decides which run to cancel at *workflow-run registration time*, before any
job's `if:` is evaluated, so PR #1236's fix cannot influence which run "wins"
the concurrency slot — it only cleans up the reported status of the *losing*
`pull_request` run in the common case.

**This was proven in production within minutes of PR #1236 merging to
`main`.** The merge commit itself hit the *adverse* ordering: the `push` run
was cancelled (not the `pull_request` run), so **neither run performed a real
build**, leaving `main`'s published Docker image tag stale until a manual
`workflow_dispatch` rebuild was triggered as a stopgap. See §2.1 for the
verified evidence.

**Goal**: append `${{ github.event_name }}` to the `concurrency:` group key so
`push` and `pull_request` runs for the same branch land in categorically
different groups and can never cancel each other, regardless of registration
order — eliminating the race structurally rather than cleaning up its output.

### 1.2 Non-goals

- This plan does **not** redesign the job-level skip logic added by PR #1236
  (docker-build.yml lines 63–94, the `setup` job and its `if:` gate) — that
  logic is verified to remain correct and necessary under the new concurrency
  scheme (see §3.3).
- This plan does **not** modify any of the nine `workflow_run`-triggered
  consumer workflows unless research in §3 proves one requires a change. The
  research verdict (§3.6) is that **none do**.
- This plan does not address the separate, much rarer `push` vs.
  `workflow_run` (`Docker Lint`) latent collision discovered during research
  (§3.7) — that trigger is `workflow_dispatch`-only on the `Docker Lint` side
  today, making the collision practically unreachable. Documented as a
  follow-up note only, not in scope.

## 2. Research Findings

### 2.1 Verified evidence of the production race (both runs still queryable)

Retrieved live via `gh run view <id> --json databaseId,event,status,conclusion,headBranch,headSha,createdAt,updatedAt`:

| Run ID | Event | Head Branch | Head SHA | Conclusion | Created | Updated |
|---|---|---|---|---|---|---|
| `31429516203` | `push` | `main` | `6ed08db3...` | **`cancelled`** | 2026-08-10T20:32:22Z | 2026-08-10T20:32:26Z |
| `31429519329` | `pull_request` | `main` | `6ed08db3...` | **`skipped`** | 2026-08-10T20:32:24Z | 2026-08-10T20:32:26Z |
| `31430784282` | `workflow_dispatch` | `main` | `6ed08db3...` | (in progress at plan time — stopgap manual rebuild) | 2026-08-10T20:47:59Z | — |

This confirms, with live data, exactly the scenario described in the issue:
the two runs registered 2 seconds apart and both concluded within 4 seconds —
`push` (the run that was supposed to do the real build) was `cancelled` by
the shared concurrency group, and `pull_request` (correctly, per PR #1236)
self-skipped to `skipped` rather than racing for the slot. Net effect: **zero**
real builds for the commit that landed on `main`.

This also settles a fact load-bearing for the rest of this plan's safety
analysis: **a `pull_request` run that self-skips via PR #1236's job-level
`if:` reports overall workflow **`conclusion: "skipped"`**, not `"success"`
and not `"cancelled"`.** Every `workflow_run`-triggered consumer's safety
verdict in §3 depends on this fact.

### 2.2 Current `docker-build.yml` structure (verified against `origin/main`, this exact file)

- **Triggers** — `.github/workflows/docker-build.yml` lines 24–35:
  ```yaml
  on:
    pull_request:
    push:
      branches: [main, development]
    workflow_dispatch:
    workflow_run:
      workflows: ["Docker Lint"]
      types: [completed]
  ```
- **Concurrency block — lines 37–39 (the block this plan changes)**:
  ```yaml
  concurrency:
    group: ${{ github.workflow }}-${{ github.head_ref || github.event.workflow_run.head_branch || github.ref_name }}
    cancel-in-progress: true
  ```
- **`setup` job's job-level skip gate — line 80** (added by PR #1236, unchanged by this plan):
  ```yaml
  if: ${{ (github.event_name != 'workflow_run' || (github.event.workflow_run.conclusion == 'success' && github.event.workflow_run.name == 'Docker Lint' && github.event.workflow_run.path == '.github/workflows/docker-lint.yml')) && (github.event_name != 'pull_request' || !contains(fromJSON('["main","development"]'), github.head_ref)) }}
  ```
  All four downstream jobs (`build-amd64`, `build-arm64`, `merge-and-publish`,
  `scan-pr-image`) gate on `needs.setup.result == 'success'` (or transitively
  on a job that does), so when `setup`'s `if:` evaluates false, the entire job
  graph for that run is skipped and the run concludes `skipped`.
- **`scan-pr-image` job — line 1064**: `if: needs.merge-and-publish.outputs.skip_build != 'true' && needs.merge-and-publish.result == 'success' && github.event_name == 'pull_request'` — intra-run only (depends on a job in the *same* run), not on the concurrency group. Unaffected by this change (§3.4 confirms).

### 2.3 The nine `workflow_run: workflows: ["Docker Build, Publish & Test"]` consumers — re-verified current on `main`

`grep -rl 'Docker Build, Publish & Test' .github/workflows/` (excluding
`docker-build.yml` itself) returns exactly the same nine files the prior
investigation found; no drift:

1. `.github/workflows/auto-changelog.yml`
2. `.github/workflows/auto-versioning.yml`
3. `.github/workflows/docs-to-issues.yml`
4. `.github/workflows/docs.yml`
5. `.github/workflows/dry-run-history-rewrite.yml`
6. `.github/workflows/history-rewrite-tests.yml`
7. `.github/workflows/propagate-changes.yml`
8. `.github/workflows/security-pr.yml`
9. `.github/workflows/supply-chain-verify.yml`

All nine were read in full for this plan. Findings per file in §3.

## 3. Critical Review-Scope Item: Does Anything Depend on a Shared push/pull_request Concurrency Group?

**Verdict up front: No. It is safe to change only `docker-build.yml`, with
no other file touched.** Full evidence follows.

### 3.1 The structural question

Before this change, `push` and `pull_request` runs of `docker-build.yml` for
the same commit share one concurrency group, so **at most one of them ever
reaches a terminal state other than `cancelled`**. After this change, **both
run to completion independently** — `push` doing the real build,
`pull_request` self-skipping (unchanged PR #1236 logic, confirmed still
correct in §3.3). This means every downstream `workflow_run` consumer will now
receive **two** `workflow_run` `completed` events for the same commit instead
of (at best) one: one from the `push`-triggered run (conclusion depends on
build outcome, typically `success`) and one from the `pull_request`-triggered
run (conclusion **always `skipped`**, per the verified evidence in §2.1).

The question is whether any consumer's `if:` condition would treat the
second, `skipped`-conclusion event as actionable — i.e., whether any consumer
assumes "exactly one workflow_run event per commit" rather than "the
workflow_run event that matters is the one with `conclusion == 'success'`."

### 3.2 Per-file verdict

| # | File | Job-level gate on `workflow_run` conclusion | Verdict |
|---|---|---|---|
| 1 | `auto-changelog.yml` | `if: github.event_name != 'workflow_run' \|\| (github.event.workflow_run.conclusion == 'success' && github.event.workflow_run.head_branch == 'main')` (line 21) | **Safe.** Requires `conclusion == 'success'`; the `skipped` `pull_request`-sourced event never satisfies this. |
| 2 | `auto-versioning.yml` | `if: github.event.workflow_run.conclusion == 'success' && github.event.workflow_run.head_branch == 'main'` (line 26) | **Safe.** Same pattern. |
| 3 | `docs-to-issues.yml` | `if: github.actor != 'github-actions[bot]' && (github.event_name != 'workflow_run' \|\| github.event.workflow_run.conclusion == 'success')` (line 37) | **Safe.** Same pattern. |
| 4 | `docs.yml` (`build` job) | `if: github.event_name == 'workflow_dispatch' \|\| github.event.workflow_run.conclusion == 'success'` (line 28) | **`build` job: Safe.** Same pattern. **`deploy` job (lines 359–369): pre-existing latent gap, NOT introduced or worsened by this fix.** Its `if:` is `(event_name=='workflow_run' && workflow_run.head_branch=='main') \|\| (event_name!='workflow_run' && ref=='refs/heads/main')` and `needs: build` — but per GitHub Actions semantics (which `docker-build.yml` itself documents inline at lines 381–383: an explicit job-level `if:` *replaces*, not appends to, the implicit "all `needs:` succeeded" check), `deploy` never references `success()` or `needs.build.result`, so it will attempt to run whenever `head_branch == 'main'` regardless of whether `build` succeeded, skipped, or was cancelled — a spurious red-X at the `actions/deploy-pages` step when no Pages artifact exists. This gap exists today (a `cancelled` `workflow_run` for `main` already triggers it); this fix only relabels the losing side's conclusion from `cancelled` to `skipped`, which `deploy`'s `if:` doesn't distinguish either way — so occurrence frequency is unaffected before/after. See §10 for the recommended follow-up issue. |
| 5 | `dry-run-history-rewrite.yml` | `if: github.event_name != 'workflow_run' \|\| github.event.workflow_run.conclusion == 'success'` (line 22) | **Safe.** Same pattern. |
| 6 | `history-rewrite-tests.yml` | `if: github.event.workflow_run.conclusion == 'success'` (line 18) | **Safe.** Same pattern. |
| 7 | `propagate-changes.yml` | `if: github.actor != 'github-actions[bot]' && github.event.workflow_run.conclusion == 'success' && (github.event.workflow_run.head_branch == 'main' \|\| ... == 'development')` (lines 25–28) | **Safe.** Same pattern. |
| 8 | `security-pr.yml` | `if: ... \|\| (github.event_name == 'workflow_run' && github.event.workflow_run.event == 'pull_request' && github.event.workflow_run.status == 'completed' && github.event.workflow_run.conclusion == 'success')` (lines 34–41) | **Safe**, and doubly so: requires **both** `workflow_run.event == 'pull_request'` **and** `conclusion == 'success'`. The `pull_request`-sourced upstream event is exactly the one that self-skips (`conclusion: skipped`) for the main/development-head race case, so it's filtered on two independent grounds. (This workflow also has its own direct `pull_request:` and `push: branches:[main]` triggers, entirely orthogonal to `docker-build.yml`'s `workflow_run` completion — unaffected either way.) |
| 9 | `supply-chain-verify.yml` (`verify-sbom` job) | `if: (github.event_name != 'schedule' \|\| github.ref == 'refs/heads/main') && (github.event_name != 'workflow_run' \|\| (github.event.workflow_run.event != 'pull_request' && (github.event.workflow_run.status != 'completed' \|\| github.event.workflow_run.conclusion == 'success')))` (lines 33–37) | **Safe**, and the *most* explicit: the inline comment ("Critical Fix #5: Exclude PR builds to prevent duplicate verification") shows this was already hardened to reject **any** `workflow_run` event where the upstream run's own triggering event was `pull_request`, regardless of conclusion. The `pull_request`-sourced race event is excluded by name, not just by conclusion. |

**Every one of the nine consumers gates on `workflow_run.conclusion ==
'success'` (directly, or transitively via a `needs:` dependency on a job that
does), and #8/#9 additionally gate on `workflow_run.event`.** None gates on
"this is the only/first workflow_run event for this commit." Since the
`pull_request`-sourced event for a main/development-head race is
structurally guaranteed to conclude `skipped` (§2.1, §3.3), none of the nine
will treat it as actionable. No downstream file requires a change.

### 3.3 Does PR #1236's skip still correctly prevent a wasteful *second real build*?

Yes — verified independent of the concurrency group. The job-level `if:` at
`docker-build.yml` line 80 gates purely on `github.event_name` and
`github.head_ref`, neither of which is influenced by which concurrency group
the run lands in. Before and after this change:

- `push` to `main`/`development`: the `event_name != 'pull_request'` clause
  is true, so the skip's second half never applies — `push` always proceeds
  to a real build (subject to the pre-existing chore/renovate-actor skip
  logic, unchanged).
- `pull_request` with `head_ref` in `["main","development"]`: the second
  clause evaluates false, so the whole `if:` is false — `setup` (and
  therefore the entire job graph) is skipped, unconditionally, regardless of
  concurrency grouping.

So decoupling the groups changes **only** whether the `pull_request` run gets
a chance to *reach* its own `if:` evaluation and report `skipped` cleanly
(now: always) vs. being pre-empted by cancellation before evaluation (before:
timing-dependent) — it does not change *what that `if:` evaluates to*. The
composition of PR #1236 (correctness of the losing side's reported status)
and this issue's fix (guaranteeing the winning side isn't the one that loses)
is confirmed complementary, not redundant, and introduces no duplicate real
build.

### 3.4 Does `scan-pr-image` (or anything else inside `docker-build.yml`) rely on the shared group?

No. `scan-pr-image`'s `if:` (line 1064) depends only on
`needs.merge-and-publish.result` and `needs.merge-and-publish.outputs.skip_build`
— both scoped to jobs *within the same workflow run*. GitHub Actions
`needs:` outputs never cross workflow-run boundaries, so this job has no
dependency on the concurrency group at all, before or after this change. For
a self-skipped `pull_request` run, `merge-and-publish` itself never runs
(it depends transitively on `setup`), so `scan-pr-image` also skips —
unrelated to concurrency, purely intra-run `needs:` propagation.

### 3.5 Composition sanity check: does splitting the group cause a wasteful *second real build* anywhere else?

Checked the only three other trigger types on `docker-build.yml`
(`workflow_dispatch`, `workflow_run` from `Docker Lint`) for the same
question: could decoupling ever let two runs for the *same commit* both
reach a real build where today's shared group accidentally prevented that?

- `workflow_dispatch`: always its own operator-invoked run; was never
  sharing a group with anything meaningfully (distinguishable via
  `head_ref`/`ref_name` already). No change in behavior.
- `workflow_run` (`Docker Lint`): see §3.7 — theoretically possible but
  practically unreachable today (`Docker Lint` is `workflow_dispatch`-only,
  §3.7), and even if it did collide, both sides doing a real build is a
  CI-cost concern, not a correctness/silent-failure concern (idempotent
  `imagetools create` calls). Out of scope per §1.2.

### 3.6 Overall verdict

**Yes, it is safe to change only `.github/workflows/docker-build.yml`.** No
other workflow file requires a change. This is a single-file, single-block
fix.

### 3.7 Side finding (documented, not actioned): latent `push` vs. `workflow_run` (Docker Lint) collision

While tracing the concurrency group's third fallback term
(`github.event.workflow_run.head_branch`), the same pre-existing shared-group
hazard was found to *also* apply between a `push` run and a
`workflow_run`-from-`Docker-Lint` run targeting the same branch — today they
too can share a group and cancel each other (there is no job-level skip
analogous to PR #1236's for this pairing). This is **not** newly introduced
by this plan's fix — splitting the group by `github.event_name` actually
*also* decouples this pairing, as a side effect, converting "they might
cancel each other" into "they might both build" (a CI-cost inefficiency, not
a silent-failure risk, since `docker buildx imagetools create` is idempotent
for identical inputs). In practice this is very low-risk: verified via
`.github/workflows/docker-lint.yml` line 3 that `Docker Lint`'s only trigger
is `workflow_dispatch` — it never runs automatically on push, so this
collision requires a human to manually dispatch `Docker Lint` at the same
moment a `push`-triggered `docker-build.yml` run is registering, which is not
a realistic production scenario. No action taken; noted here for the record
in case `Docker Lint`'s triggers are ever broadened in the future.

## 4. Technical Specification — The Change

### 4.1 File changed

`.github/workflows/docker-build.yml` — **only** this file, per §3.6.

### 4.2 Exact diff

Current (lines 37–39):

```yaml
concurrency:
  group: ${{ github.workflow }}-${{ github.head_ref || github.event.workflow_run.head_branch || github.ref_name }}
  cancel-in-progress: true
```

New:

```yaml
concurrency:
  group: ${{ github.workflow }}-${{ github.head_ref || github.event.workflow_run.head_branch || github.ref_name }}-${{ github.event_name }}
  cancel-in-progress: true
```

Single-line change: append `-${{ github.event_name }}` to the `group:`
value on line 38. `cancel-in-progress: true` (line 39) is unchanged — it
still deduplicates rapid-fire repeats of the *same* event type (e.g. two
quick pushes to `main`, or two quick PR `synchronize` events), which remains
the correct and intended use of cancellation here.

### 4.3 Resulting group values (worked examples)

| Scenario | `github.event_name` | Resulting group (suffix shown) |
|---|---|---|
| Push to `main` | `push` | `...-main-push` |
| PR with head branch `main` (promotion/sync PR) | `pull_request` | `...-main-pull_request` |
| Push to `development` | `push` | `...-development-push` |
| PR with head branch `development` | `pull_request` | `...-development-pull_request` |
| `Docker Lint` workflow_run completion on `main` | `workflow_run` | `...-main-workflow_run` |
| Manual `workflow_dispatch` on `main` | `workflow_dispatch` | `...-main-workflow_dispatch` |
| Ordinary feature-branch PR (e.g. head `feature/foo`) | `pull_request` | `...-feature/foo-pull_request` |

`push` and `pull_request` for the same branch now always land in different
rows — the race is eliminated by construction, not by timing luck.

### 4.4 Also update the explanatory comment above `push.branches`

`docker-build.yml` lines 27–30 currently read:

```yaml
  push:
    # Keep this branch list in sync with the `if:` skip condition on the
    # `setup` job below (search "trigger-level dedup") — that condition
    # exists specifically to avoid double-building commits covered by both
    # this `push:` trigger and an open PR headed at the same branch.
    branches: [main, development]
```

This comment is still accurate (the job-level skip is unchanged, per §3.3)
but should gain one clause noting the concurrency group is now split by
event type, so a future reader doesn't assume the two mechanisms are
redundant. Proposed addition (append, don't replace, to preserve the
existing "keep in sync" instruction):

```yaml
  push:
    # Keep this branch list in sync with the `if:` skip condition on the
    # `setup` job below (search "trigger-level dedup") — that condition
    # exists specifically to avoid double-building commits covered by both
    # this `push:` trigger and an open PR headed at the same branch.
    # NOTE: `push` and `pull_request` no longer share a concurrency group
    # (see the `-${{ github.event_name }}` suffix below) — this `if:` skip
    # is what stops the `pull_request` side from doing a redundant real
    # build now that group-based cancellation can no longer do it for us.
    # See GH #1235 / docs/plans/current_spec.md for the full rationale.
    branches: [main, development]
```

This is a comment-only addition (no functional change) and may be folded
into the same commit as §4.2's functional line.

## 5. Data Flow / Component Interaction (unchanged, restated for clarity)

```
Commit lands on main (e.g. weekly promotion PR merge)
        │
        ├── push event ──────────► docker-build.yml run A
        │                          group: "...-main-push"
        │                          setup if: true → real build → success
        │                          └─► workflow_run(conclusion=success, event=push)
        │                                 ├─► auto-changelog.yml        (fires: head_branch=main ✓)
        │                                 ├─► auto-versioning.yml       (fires)
        │                                 ├─► docs-to-issues.yml        (fires)
        │                                 ├─► docs.yml                  (fires)
        │                                 ├─► dry-run-history-rewrite   (fires)
        │                                 ├─► history-rewrite-tests    (fires)
        │                                 ├─► propagate-changes.yml     (fires)
        │                                 ├─► security-pr.yml           (event!=pull_request → skips its wr branch)
        │                                 └─► supply-chain-verify.yml   (event!=pull_request → fires)
        │
        └── pull_request event (head=main) ──► docker-build.yml run B
                                   group: "...-main-pull_request"  (DIFFERENT group — no cancellation of A)
                                   setup if: false (PR #1236 gate) → skipped
                                   └─► workflow_run(conclusion=skipped, event=pull_request)
                                          └─► ALL nine consumers: gate on conclusion=='success'
                                              (or event!='pull_request') → none fire a second time
```

## 6. Error Handling / Edge Cases

| Edge case | Behavior after fix | Notes |
|---|---|---|
| Push and PR(head=main) register in either order | Both runs execute to completion independently | This is the exact bug being fixed — order no longer matters |
| Two rapid pushes to `main` (e.g. force-push race, unlikely on protected branch) | Second `push` cancels the first (`cancel-in-progress: true` still applies within the `push` group) | Unchanged, correct, intended behavior |
| Two rapid `pull_request` `synchronize` events on the same PR | Second cancels the first within the `pull_request` group for that branch | Unchanged, correct, intended behavior |
| PR head branch is a normal feature branch (not `main`/`development`) | No `push` run exists for that branch at all (`push.branches` doesn't include it) — no race was ever possible | Unaffected by this change either way |
| `workflow_dispatch` manual rebuild (the stopgap used for run `31430784282`) | Own group (`...-<branch>-workflow_dispatch`), never collides with `push` or `pull_request` | Improves on today: a manual rebuild can no longer be cancelled by an unrelated push/PR event for the same branch |
| `Docker Lint` `workflow_run` completion (rare, manual-dispatch-only) | Own group (`...-<branch>-workflow_run`) | See §3.7 — decoupled as a side effect; low real-world exposure |

## 7. Commit Slicing Strategy

**Decision: single commit, single PR.** The functional change is one line;
the comment update (§4.4) is directly adjacent and explanatory. Splitting
further would add review overhead without improving reviewability — the
entire diff is legible in one hunk, and the bulk of the engineering work in
this task is the *research* documented in §3 (which belongs in the PR
description / this plan, not as separate commits).

### Commit 1 (only commit): `fix: split docker-build.yml concurrency group by event_name to eliminate push/pull_request race`

- **Scope**: `.github/workflows/docker-build.yml` only.
- **Changes**:
  1. §4.2 — append `-${{ github.event_name }}` to the `concurrency.group` expression (line 38).
  2. §4.4 — extend the explanatory comment above `push.branches` (lines 27–30) to note the group split and cross-reference GH #1235 / this plan.
- **Dependencies**: None. Applies cleanly on top of `origin/main` (PR #1236 already merged).
- **Validation gate** (must pass before this commit is considered complete):
  1. `actionlint .github/workflows/docker-build.yml` — zero errors (also runs automatically via `lefthook run pre-commit`'s `actionlint` hook, `lefthook.yml` line 72).
  2. `lefthook run pre-commit` — full fast-hook suite, zero errors.
  3. Manual YAML diff review confirming *only* the `group:` line and the one comment block changed — no other line in the 1340-line file touched (guards against accidental reformatting/whitespace drift in a large generated-looking file).
  4. Live validation — see §8 below (cannot fully gate the commit on this since it requires a real GitHub Actions registration race, but the plan requires a concrete post-merge validation procedure).

### Rollback / contingency

- **Rollback**: single `git revert` of the one commit restores the exact
  pre-fix `concurrency:` block. No schema/state/migration to unwind — this is
  a pure CI-configuration change with no runtime or data-layer footprint.
- **Contingency if the fix doesn't fully resolve the race** (e.g. an
  unforeseen fourth trigger type also collides): the job-level skip pattern
  from PR #1236 is the proven fallback — extend `setup`'s `if:` gate (line
  80) with an additional clause for the new colliding event type, following
  the same "designate one event type as authoritative, skip the other"
  pattern. Does not require reverting this fix; additive.
- **Blast radius if wrong**: worst case is a reversion to *current*
  production behavior (occasional missed builds on main/development sync
  commits, mitigated by manual `workflow_dispatch` as already demonstrated
  in run `31430784282`) — this change cannot make the race *worse* than
  today, only better or (if some undiscovered dependency exists) neutral.

## 8. Validation Plan

### 8.1 Static validation (pre-merge, required)

1. `actionlint .github/workflows/docker-build.yml` (and, for regression
   safety, `actionlint .github/workflows/*.yml` for the full directory,
   since actionlint validates cross-file `workflow_run` references).
2. `lefthook run pre-commit` — confirms the repo's full fast-hook gate
   (YAML validity, actionlint, shellcheck, etc.) passes with the change
   staged.
3. Diff review against §4.2/§4.4 — confirm no unintended changes.

### 8.2 Live validation — constructing the race safely (pre-merge, best-effort)

The coordinator's requirement is to reproduce a genuine simultaneous
`push` + `pull_request` pair for the same commit/branch and confirm both
complete independently post-fix. Per the prior plan's §7.2 precedent, pushing
directly to a branch literally named `main`/`development` to manufacture this
is unsafe (collides with real production branches) and is **not** proposed.
Unlike the prior fix, this one requires *both* events to fire near-
simultaneously (not just an artifact of PR-review timing), which is harder to
construct safely. Evaluated options:

| Option | Safe? | Verdict |
|---|---|---|
| Push directly to `main`/`development` to force a real race | No — collides with production branches/tags/deploys | Rejected (same reasoning as prior plan's §7.2) |
| Temporarily add a disposable branch to `push.branches` | No — user explicitly ruled this out (changes production trigger behavior for the duration of the test) | Rejected per task instructions |
| Open a same-repo PR whose head branch is a disposable branch, then separately push to that disposable branch, with neither in `push.branches` | Safe, but does **not** reproduce the race — `push.branches` gates which pushes even trigger the workflow, so a push to a non-`main`/`development` branch never enters a race with a PR on that branch in the first place | Does not exercise the fix; only proves the no-op case |
| Fork the exact production scenario in a scratch/throwaway **repository** (not branch) that mirrors `docker-build.yml`'s triggers, and script a same-second push + PR-synchronize against a branch named `main` in that scratch repo | Safe (isolated from production Charon) | **Recommended for a true pre-merge repro**, but heavyweight — requires standing up a second repo with GHCR/DockerHub credentials or a build-stubbed variant. Optional; not required to approve this fix given §3's structural proof. |
| Rely on the **next real** main/development sync event post-merge as the validation, with a documented rollback-ready monitoring step | Safe, zero extra infrastructure | **Recommended primary validation** — see §8.3 |

**Honest conclusion, consistent with the prior plan's approach to its own
§7.2**: safely constructing an exact simultaneous push+pull_request pair
against the *real* `Charon` repo pre-merge is not achievable without either
(a) unsafely touching real `main`/`development`, or (b) standing up
disposable infrastructure disproportionate to a one-line config fix whose
correctness is already established structurally (§3, §4.3) and empirically
motivated by a real, timestamped incident (§2.1). The recommended path is
static validation (§8.1) plus the structural proof in §3, gated by real-world
post-merge monitoring (§8.3) with a trivial, already-rehearsed rollback
(§7, "Rollback / contingency" — a revert commit, or the same
`workflow_dispatch` stopgap already proven to work in run `31430784282`).

### 8.3 Post-merge monitoring (primary validation)

1. After merge, monitor the **next** organic event that lands a commit on
   `main` or `development` while an open bot-generated sync/promotion PR
   targets that same branch (weekly `nightly → main` promotions and
   `propagate-changes.yml`-created PRs are the two known generators of this
   pattern — see `.github/workflows/weekly-nightly-promotion.yml` and
   `propagate-changes.yml`).
2. For that event, run:
   ```bash
   gh run list --workflow=docker-build.yml --limit 10 --json databaseId,event,headBranch,headSha,conclusion,createdAt
   ```
   and confirm: a `push` run and a `pull_request` run exist for the same
   `headSha`, the `push` run's conclusion is `success` (or a genuine build
   failure unrelated to concurrency — not `cancelled`), and the
   `pull_request` run's conclusion is `skipped` (not `cancelled`).
3. Cross-check that exactly one of the nine downstream consumers' expected
   side effects occurred once (e.g. `auto-versioning.yml` created at most one
   release tag for the commit, `propagate-changes.yml` opened at most one
   sync PR) — confirms §3's "no double-fire" verdict empirically, not just
   structurally.
4. If step 2 or 3 ever shows a `cancelled` conclusion for either event type
   again, or a downstream consumer double-firing, treat as a P1 regression:
   revert this commit immediately (§7 rollback) and re-open investigation.

## 9. Acceptance Criteria

- [ ] `docker-build.yml`'s `concurrency.group` expression includes
      `${{ github.event_name }}` as its final component (§4.2), and no other
      functional line in the file changed.
- [ ] `actionlint` passes with zero errors on the modified file.
- [ ] `lefthook run pre-commit` passes with zero errors.
- [ ] The explanatory comment above `push.branches` is updated per §4.4.
- [ ] Plan's §3 research is included in the PR description (or linked) so
      reviewers can verify the "no other file needs to change" verdict
      without re-deriving it.
- [ ] A concrete tracking issue for post-merge monitoring (§8.3) is opened
      **at merge time** (not merely "scheduled/assigned" informally) — this
      is the only real-world confirmation this plan gets that the fix
      actually works in production, so it must not be silently dropped. The
      issue should reference §8.3's exact `gh run list` verification steps.

## 10. Risks & Assumptions

- **RISK-001**: If a tenth (currently unknown) workflow is added in the
  future that consumes `docker-build.yml`'s `workflow_run` completion
  *without* gating on `conclusion == 'success'`, it would be newly exposed to
  receiving two completion events per race commit instead of (at most) one.
  Mitigation: this plan's §3.2 table doubles as a checklist — any new
  `workflow_run: workflows: ["Docker Build, Publish & Test"]` consumer added
  in the future should be required (e.g. via a lint rule or PR template
  checkbox) to gate on `conclusion == 'success'`, matching the existing
  convention all nine current consumers already follow.
- **RISK-002** (§3.7): `Docker Lint`'s trigger surface could be broadened in
  the future (e.g. adding a `push:` trigger) in a way that reintroduces a
  push-vs-workflow_run collision, now surfaced as "both build" rather than
  "cancel". Low severity (cost, not correctness) and not currently reachable.
  Mitigation: flagged here for future readers; no action required now.
- **FOLLOW-UP (from Supervisor review)**: File a separate follow-up issue for
  `docs.yml`'s `deploy` job `if:` gap identified in §3.2 row 4 — add
  `&& needs.build.result == 'success'` to `deploy`'s `if:` (line 361–363) so
  it no longer attempts `actions/deploy-pages` when `build` didn't actually
  produce a Pages artifact (skipped or cancelled upstream run). Pre-existing,
  not introduced or worsened by this fix; out of scope for this PR but should
  not be left as untracked prose. Track alongside RISK-001/RISK-002 above,
  following the same "known-deferred fix as an issue" precedent cited in
  §11 (Orthrus dual-muzzle allowlist, GH #1160/#1161).
- **ASSUMPTION-001**: GitHub Actions' documented behavior — that a job whose
  `if:` evaluates false causes the overall workflow run to conclude
  `skipped` when no other job in the run executes — is confirmed empirically
  by run `31429519329` (§2.1), not just assumed from documentation.
- **ASSUMPTION-002 — CLOSED (verified via live data, not deferred)**: No
  workflow outside the nine identified consumers reads `docker-build.yml`'s
  completion status by any means other than the `workflow_run` trigger (e.g.
  polling the Checks API, badge endpoints, branch protection required-status
  checks). This is no longer an open assumption — verified live against the
  actual repository:
  - `gh api repos/Wikid82/Charon/branches/main/protection` → `404 "Branch not
    protected"`.
  - `gh api repos/Wikid82/Charon/rulesets` → two rulesets: `"charon rules"`
    (id `10078859`, `enforcement: "active"`) and `"merging"` (id `10169103`,
    `enforcement: "disabled"`).
  - `gh api repos/Wikid82/Charon/rulesets/10078859` → the only *active*
    ruleset's `rules` array contains a single `{"type": "deletion"}` rule —
    no `required_status_checks` rule present.
  - **Conclusion: `main`/`development` currently have no required-status-check
    enforcement at all.** There is no branch-protection or ruleset surface
    that could depend on `docker-build.yml` reporting exactly one conclusion
    per commit, so this change carries zero risk on that front today. (Side
    observation, out of scope for this fix: the total absence of
    required-status-check enforcement on `main`/`development` may itself be
    worth a separate future issue — this repo currently cannot technically
    block a merge on CI failure via GitHub's native mechanism.)

## 11. Related Specifications / Further Reading

- GitHub Issue [#1235](https://github.com/Wikid82/Charon/issues/1235) (this plan's source)
- Merged PR [#1236](https://github.com/Wikid82/Charon/pull/1236) (companion fix, job-level skip)
- `.github/workflows/docker-build.yml` (file under change)
- `.github/workflows/weekly-nightly-promotion.yml`, `.github/workflows/propagate-changes.yml` (the two known generators of main/development-head sync PRs that trigger this race)
- Precedent for tracking deferred/related fixes as separate issues: Orthrus dual-muzzle allowlist drift, GH #1160/#1161 (referenced in issue #1235's body)
