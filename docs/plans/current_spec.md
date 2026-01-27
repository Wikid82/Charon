# GitHub Actions E2E Trigger Investigation Plan (PR #550)

**Context**
- Repository: Wikid82/Charon
- Default branch: `main`
- Active PR: #550 chore(docker): migrate from Alpine to Debian Trixie base image
- Working branch: `feature/beta-release`
- Symptom: After pushing an update to re-enable some E2E tests, the expected workflow did not trigger.

## Phase 0 – Context Validation (30 min)
- Confirm PR #550 source (fork vs upstream) and actor.
- Identify which E2E workflow should have run (list specific file/job after discovery in Phase 1 Task 1).
- Verify that a push occurred to `feature/beta-release` after re-enabling tests.
- Document expected trigger event vs actual run in Actions history.

Create Decision Record:
- Expected workflow: <file>/<job>
- Expected trigger(s): push/pull_request synchronize
- Observation time window: <timestamps>

**Objectives (EARS Requirements)**
- THE SYSTEM SHALL automatically run E2E workflows on eligible events for `feature/**`, `main`, and relevant branches.
- WHEN a commit is pushed to `feature/beta-release`, THE SYSTEM SHALL evaluate workflow `on:` triggers and filters and start corresponding jobs if conditions match.
- WHEN a pull request is updated (synchronize) for PR #550, THE SYSTEM SHALL trigger CI for all workflows configured for `pull_request` to the target branch.
- IF branch/path/actor conditions prevent a run, THEN THE SYSTEM SHALL allow a manual `workflow_dispatch` as a fallback.

**Hypotheses to Validate**
1. Path filters exclude the recent changes (e.g., only watching `frontend/**`, `backend/**`, `tests/**`, `playwright.config.js`, or `.github/workflows/**`).
2. Branch filters do not include `feature/**` or the YAML pattern is mis-specified.
3. PR is from a fork; secrets and permissions prevent jobs from running.
4. Skip conditions (`if:` gates) block runs for specific commit messages (e.g., `chore:`) or bots.
5. Concurrency cancellation due to rapid successive pushes suppresses earlier runs (`concurrency` with `cancel-in-progress`).
6. Workflows only run on `workflow_dispatch` or specific events, not `push`/`pull_request`.

**Design: Trigger Validation Approach**
- Inspect E2E-related workflows in `.github/workflows/` (e.g., `e2e-tests.yml`, `playwright-e2e.yml`, `docker-build.yml`).
- Enumerate `on:` events: `push`, `pull_request`, `pull_request_target`, `workflow_run`, `workflow_dispatch`.
- Capture `branches`, `branches-ignore`, `paths`, `paths-ignore`, `tags` filters; confirm YAML quoting and glob correctness.
- Review top-level `permissions:` and job-level `if:` conditions; note actor-based skips.
- Confirm matrix/include conditions for E2E jobs (e.g., only run when Playwright-related files change).
- Check Actions history for PR #550 and branch `feature/beta-release` to correlate event delivery vs filter gating.

## Phase 1 – Diagnosis (Targeted Checks)

### Task 1: Audit Workflow Triggers (DevOps)
Commands:
- List candidate workflows:
  - `find .github/workflows -name '*e2e*' -o -name '*playwright*' -o -name '*test*' | sort`
- Extract triggers and filters:
  - `grep -nA10 '^on:' <workflow.yml>`
  - `grep -nE 'branches|paths|concurrency|permissions|if:' <workflow.yml>`
Output:
- Table: [Workflow | Triggers | Branches | Paths | if-conditions | Concurrency]

### Task 2: Retrieve Recent Runs (DevOps)
Commands:
- `gh run list --repo Wikid82/Charon --limit 20 --status all`
- `gh run view <run_id> --repo Wikid82/Charon`
- Correlate cancellations and `concurrency` group IDs.

### Task 3: Verify PR Origin & Permissions (DevOps)
Commands:
- `gh pr view 550 --repo Wikid82/Charon --json isCrossRepository,author,headRefName,baseRefName`
Interpretation:
- If `isCrossRepository=true`, factor `pull_request_target` and secret restrictions.

### Task 4: Inspect Commit Messages & Actor Filters (DevOps)
Commands:
- `git log --oneline -n 5`
- Check workflow `if:` conditions referencing `github.actor`, commit message patterns.

**Success Criteria (Phase 1):**
- Root cause identified (±1 hypothesis), reproducible via targeted test.

## Phase 1.5 – Hypothesis Elimination (1 hour)
Targeted tests per hypothesis:
1. Path filter: Commit `tests/.keep`; confirm if E2E triggers.
2. Branch filter: Push to `feature/test-trigger` (wildcard); observe triggers.
3. Fork PR: Confirm with `gh pr view`; evaluate secret usage.
4. Commit message: Push with non-`chore:` message; observe.
5. Concurrency: Push two commits quickly; confirm cancellations & group.

Deliverable:
- Ranked hypothesis list with evidence and logs.

## Phase 2 – Remediation (Proper Fix)

### Scenario A: Path Filter Mismatch
- Fix: Expand `paths:` to include re-enabled tests and configs.
- Acceptance: Workflow triggers on next push touching those paths.

### Scenario B: Branch Filter Mismatch
- Fix: Add `'feature/**'` (quoted) to `branches:` for relevant events.
- Acceptance: Push to `feature/beta-release` triggers E2E.

### Scenario C: Fork PR Gating
- Fix: Use `pull_request_target` with least privileges OR require upstream branch for E2E.
- Acceptance: PR updates trigger E2E without secret leakage.

### Scenario D: Skip Conditions
- Fix: Adjust `if:` to avoid skipping E2E for `chore:` messages; add `workflow_dispatch` fallback.
- Acceptance: E2E runs for typical commits; manual dispatch available.

### Scenario E: Concurrency Conflicts
- Fix: Separate concurrency groups or set `cancel-in-progress: false` for E2E.
- Acceptance: Earlier runs not cancelled improperly; stable execution.

Implementation Notes:
- Apply YAML edits in the respective workflow files; validate via `workflow_dispatch` and a watched-path commit.

## Phase 3 – Validation & Hardening
- Add/verify `workflow_dispatch` inputs for manual E2E runs.
- Push minimal commit touching guaranteed watched path.
- Document test in `docs/testing/`; update `README.md` CI notes.
- Regression test: Trigger from different branch/actor/event to confirm persistence.

**Related Config Checks**
- `codecov.yml`: Verify statuses and paths do not block CI.
- `.dockerignore` / `.gitignore`: Ensure test assets are included in context.
- `Dockerfile`: No gating on branch/commit via args.
- `playwright.config.js`: E2E matrix does not restrict by branch erroneously.

**Risks & Fallbacks**
- Increased CI load from wider `paths:` → keep essential paths only.
- Security concerns with `pull_request_target` → restrict permissions, avoid untrusted code execution.
- Fallbacks: Manual `workflow_dispatch`, dedicated E2E workflow with wide triggers, `repository_dispatch` testing.

**Task Owners**
- DevOps: Workflow trigger analysis and fixes
- QA_Security: Validate runs, review permissions and secret usage
- Frontend/Backend: Provide file-change guidance to exercise triggers

**Timeline & Escalation**
- Phase 1: 2 hours; Phase 2: 4 hours; Phase 3: 2 hours.
- If root cause not found by Phase 1.5, escalate with action log to GitHub Support.

**Next Steps**
- Request approval to begin Phase 1 execution per this plan.
