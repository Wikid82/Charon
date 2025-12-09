History Rewrite: Address Copilot Suggestions (PR #336)
===================================================

Summary
-------
- PR #336 introduced history-rewrite tooling, documentation, and a CI dry-run workflow to detect unwanted large blobs and CodeQL DB artifacts in repository history.
- Copilot left suggestions on the PR asserting a number of robustness, testing, validation, and safety improvements.
- This spec documents how to resolve those suggestions, lists the impacted files and functions, and provides an implementation & QA plan.

Copilot Suggestions (Short Summary)
----------------------------------
- Improve `validate_after_rewrite.sh` to use a defined `backup_branch` variable and fail gracefully when missing.
- Harden `clean_history.sh` and `preview_removals.sh` to handle shallow clones, tags, and refs, validate `git-filter-repo` args, and double-check backups (include tags & annotated refs).
- Add automated script unit tests (shell) for the scripts (preview/dry-run/validate) to make them testable and CI-friendly.
- Add a CI job to run these script tests (e.g., `bats-core`) and trap shallow clones early.
- Expand pre-commit and `.gitignore` coverage (include `data/backups`), validate `backup_branch` push, and refuse running filter-repo on `main`/`master` or non-existent remotes.
- Add more detailed PR checklist validation (tags, backup branch pushed) and update docs/examples.

Files Changed / Impacted
------------------------
Core scripts and CI currently touched by PR #336 and Copilot suggestions (primary targets):
- scripts/history-rewrite/clean_history.sh
  - Functions: `check_requirements`, `timestamp`, `preview_removals` block, local `backup_branch` creation.
  - Behaviors to harden: shallow clone handling; ensure backup branch pushed to remote and tags backed up; refuse to run on `main`/`master`; confirm `git-filter-repo` args are validated; ensure remote tag backup.
- scripts/history-rewrite/preview_removals.sh
  - Behaviors to add: more structured preview output (json or delimited), detect shallow clone and warn, add checks for tags & refs.
- scripts/history-rewrite/validate_after_rewrite.sh
  - Fix bug: `backup_branch` referenced but not set, add env variable or accept `--backup-branch` argument; verify pre-commit location; exit non-zero on failures.
- scripts/ci/dry_run_history_rewrite.sh
  - Add shallow clone detection and early fail with instructions to fetch full history; ensure `git rev-list` does not grow too large on very large repositories (timeout or cap); fail on conditions.
- .github/workflows/dry-run-history-rewrite.yml
  - Behavior: run the new tests; ensure fetch-depth 0; add `bats` runner step or `shellcheck` runner.
- .github/workflows/pr-checklist.yml
  - Behavior: enhance validation of PR body for additional checklist items: ensure `data/backups` log is attached, `tags` backup statement, and maintainers ack for forced rewrite.
- .github/PULL_REQUEST_TEMPLATE/history-rewrite.md
  - Behavior: update the checklist with new checks for tags and `data/backups/` and note `validate_after_rewrite.sh` will fail if not present.
- .gitignore
  - Add `data/backups/` to `.gitignore` to ensure backup logs are not accidentally committed.
- .pre-commit-config.yaml
  - Add a new `block-data-backups-commit` hook to prevent accidental commits to `data/backups`.

Potential Secondary Impact (best-guess; confirm):
- scripts/pre-commit-hooks/block-codeql-db-commits.sh (might need to be more strict): extend to check codeql-db-* and codeql-*.sarif patterns.
- scripts/ci/dry_run_history_rewrite.sh invocation in `.github/workflows/dry-run-history-rewrite.yml`: adjust to ensure `fetch-depth: 0` is set and that `git` is non-shallow.

Implementation Plan (Phases)
--------------------------
PHASE 1 — Script Hardening (2-4 days)
- Goals: fix functional bugs, add validation checks, handle edge cases (shallow clones, tag preservation), make scripts idempotent and testable.
- Tasks:
  1. Update `scripts/history-rewrite/validate_after_rewrite.sh`:
     - Add a command-line argument or `ENV` for `--backup-branch` and fallback to reading `backup_branch` from the log in `data/backups` if present.
     - Ensure it sets `backup_branch` correctly or exits with a clear message.
     - Ensure it currently fails the build on any reported issues (non-zero exit when pre-commit fails in CI mode).
  2. Update `scripts/history-rewrite/clean_history.sh`:
     - Detect shallow clones (if `git rev-parse --is-shallow-repository` returns true) and fail with instructions to `git fetch --unshallow`.
     - When creating `backup_branch`, also include tag backups: `git tag -l | xargs -n1 -I{} git tag -l -n {}...` and push tags to `origin` into `backup/tags/history-YYYY...` namespace OR save them to `data/backups/tags-*.tar`.
     - Validate `git-filter-repo` args are valid—use `git filter-repo --help` to confirm that provided `--strip-blobs-bigger-than` args are numbers and `--paths` exist in repo for the dry-run case.
     - Ensure `backup_branch` is pushed successfully, otherwise abort.
     - Make `read -r confirmation` explicit with `--` or a short timeout to avoid interactive hang; in scripts launched via terminal, interactive fallback is acceptable, but in CI this should not be used. Add `--non-interactive` to skip confirmation in CI with an explicit flag and require maintainers to pass `FORCE=1` in env to proceed.
  3. Update `scripts/history-rewrite/preview_removals.sh`:
     - Add structured `--format` option with `text` (default) and `json` for CI parsing; include commit oids, paths, and sizes in the output.
     - Detect & warn if the repo is shallow.
  4. Add a `scripts/history-rewrite/check_refs.sh` helper:
     - Print current branches, tags, and any remotes pointing to objects in the paths to be removed.
     - Output a tarball `data/backups/tags-YYYYMMDD.tar` with tag refs.

PHASE 2 — Testing & Automation (2-3 days)
- Goals: Add script unit tests and CI steps to run them; add a validation pipeline for maintainers to use.
- Tasks:
  1. Add `bats-core` test harness inside `scripts/history-rewrite/tests/`.
     - `scripts/history-rewrite/tests/preview_removals.bats` — tests ensuring the preview prints commits and objects for specified paths.
     - `scripts/history-rewrite/tests/clean_history.dryrun.bats` — tests that `--dry-run` exits non-zero when repo contains banned paths and that `--force` requires confirmation.
     - `scripts/history-rewrite/tests/validate_after_rewrite.bats` — tests that `validate_after_rewrite.sh` uses `--backup-branch` and fails with the correct non-zero codes when `backup_branch` is missing.
  2. Add a `ci/scripts/test-history-rewrite.yml` workflow to run bats tests in CI and to fail early on shallow clones or missing tools.
  3. Add a script-level `shellcheck` pass and a `bash` minimal lint step; use `shellcheck` GitHub Action or pre-commit hook.

PHASE 3 — PR Pipeline & Pre-commit (1-2 days)
- Goals: Prevent accidental destructive runs and accidental commits of generated backups.
- Tasks:
  1. Update the PR template `.github/PULL_REQUEST_TEMPLATE/history-rewrite.md` adding checklist items: tag backups, confirm `data/backups` tarball included, confirm remote pushed backup branch and tags, optional `CI verification output` from `preview_removals --format json`.
  2. Update `.github/workflows/pr-checklist.yml` to validate: presence of `preview_removals` output in PR body, a check that `data/backups` is attached, and additional keywords like `tag backup` and `backup branch pushed`.
  3. Add `.pre-commit-config.yaml` hook to block commits to `data/backups` and ensure `data/backups` is added to `.gitignore`.
  4. Add `scripts/pre-commit-hooks/validate-backup-branch.sh` which verifies that `backup_branch` exists and points to the expected ref(s).

PHASE 4 — Docs, QA & Rollout (1-2 days)
- Goals: Update docs, add reproducible tests, and provide QA instructions and rollback strategies.
- Tasks:
  1. Update `docs/plans/history_rewrite.md` to include:
     - `backup_branch` naming and tagging policy
     - `data/backups` layout, e.g., `metadata.json`, `tags.tar.gz`, `log` paths
     - Example `preview_removals --format json` output for PR inclusion
  2. Add `docs/plans/current_spec.md` (this file) containing the execution plan and timeline estimate.
  3. QA steps: run `clean_history.sh --dry-run`, `preview_removals.sh` with `--format json` for PR attachments, then proceed with `--force` only after maintainers confirm window; verify via `validate_after_rewrite.sh` and CI.

PHASE 5 — Post-Deploy & Maintenance (1 day)
- Run `git gc` and prune on mirrors; notify downstream consumers; update CI mirrors and caches. Verify repository size decreased within expected tolerance.

Unit & Integration Tests (Files & Functions)
-------------------------------------------
Add these test files to `scripts/history-rewrite/tests/`.
Unit test harness: `bats-core` recommended; tests should run without network and create ephemeral local repositories.

- `scripts/history-rewrite/tests/preview_removals.bats`:
  - test_preview_detects_banned_commits()
  - test_preview_detects_large_blob_sizes()
  - test_preview_outputs_json_when_requested()

- `scripts/history-rewrite/tests/clean_history.dryrun.bats`:
  - test_dry_run_exits_success_when_no_banned_paths()
  - test_dry_run_reports_banned_commits()
  - test_force_requires_confirmation() — simulate interactive confirmation or set `FORCE=1` with `--non-interactive` flag to test non-interactive usage.
  - test_refuse_on_main_branch() — ensures script refuses to run on `main`/`master`.

- `scripts/history-rewrite/tests/validate_after_rewrite.bats`:
  - test_validate_fails_when_backup_branch_missing()
  - test_validate_passes_when_backup_branch_provided_and_all_checks_clear()
  - test_validate_populates_log_and_error_when_precommit_fails()

Integration test (bash / simulated repository): a test that acts as a small git repo containing a `backend/codeql-db` folder and a large fake blob.
- `scripts/history-rewrite/tests/integration_clean_history.bats`:
  - test_integration_end_to_end_preview_then_dry_run(): create a local repo, add a large file under `backend/codeql-db`, commit it, run `preview_removals` to capture output, ensure `clean_history.sh --dry-run` detects it, then run `clean_history.sh --force` but only after backing up repo; verify `git rev-list` no longer returns commits for that path.

Exact tests & names (for maintainers' convenience):
- `scripts/history-rewrite/tests/preview_removals.bats::test_preview_detects_banned_commits`
- `scripts/history-rewrite/tests/preview_removals.bats::test_preview_outputs_json`
- `scripts/history-rewrite/tests/clean_history.dryrun.bats::test_dry_run_reports_banned_commits`
- `scripts/history-rewrite/tests/clean_history.dryrun.bats::test_force_requires_confirmation`
- `scripts/history-rewrite/tests/validate_after_rewrite.bats::test_validate_fails_when_backup_branch_missing`
- `scripts/history-rewrite/tests/integration_clean_history.bats::test_integration_end_to_end_preview_then_dry_run`

CI & Pre-commit Changes
-----------------------
- Add `data/backups/` to `.gitignore` (to avoid accidental commits of backup logs) and ensure `scripts` produce readable `data/backups` logs that can be attached to PRs.
- Add a new pre-commit hook `scripts/pre-commit-hooks/block-data-backups-commit.sh` to block user commits of `data/backups` and `data/backups/*` (mirror `block-codeql-db-commits.sh`).
- Add `shellcheck` to the pre-commit config or add a `scripts/ci/shellcheck_history_rewrite.yml` workflow that ensures scripts pass style checks.
- Create a new CI workflow: `.github/workflows/history-rewrite-tests.yml`
  - Steps: Checkout with `fetch-depth: 0`, install bats-core via apt or package manager, run the `bats` tests, run `shellcheck` for scripts, and run `scripts/ci/dry_run_history_rewrite.sh`.
- Update existing `.github/workflows/dry-run-history-rewrite.yml` to:
  - Ensure `fetch-depth: 0` in `actions/checkout` is set (already the case), and fail early for shallow clones; add a `shellcheck` step and `bats` tests step.

Potential Regressions & Rollback Strategies
-------------------------------------------
- Regressions:
  - Accidental removal of unrelated history entries due to incorrect `--paths` or `--invert-paths` usage.
  - Loss of tags or refs if not properly backed up and pushed to a safe place before rewrite.
  - CI breakage from new pre-commit hooks or failing `bats` tests.
  - Developer pipelines or forks could break from forced `--all --force` push if they do not follow the rollback steps.

- Mitigations & Rollback:
  - **Always create backups**: `backup_branch` and `backup/tags/history-YYYYMMDD` tarball stored outside the working repo (S3/GitHub release) prior to any `--force` push.
  - Maintain a simple rollback command sequence in the docs:
    - `git checkout -b restore/DATE backup/history-YYYYMMDD-HHMMSS`
    - `git push origin restore/DATE` and create a PR to restore the history (or directly replace refs on the remote as maintainers decide)
  - Keep the `data/backups/` tarball outside the repo in a known remote location (this will also help recovery if the `backup_branch` is not visible).
  - Ensure CI `dry-run` workflow is fully functional and fails on shallow clones so maintainers must re-run with a proper clone.
  - Add a section in `docs/plans/history_rewrite.md` to show commands to restore tags if they were mistakenly deleted.

Backwards Compatibility & Maintainers' Notes
-------------------------------------------
- The scripts must remain POSIX-compliant where pragmatic; use `/bin/sh` for portability.
- Avoid automatic `git push --all --force` from scripts; maintainers must perform final coordinated push.
- Scripts will remain safe by default (`--dry-run` or interactive) with `--force` and explicit `I UNDERSTAND` confirmation for destructive operations.

Timeline Estimate (Rough)
------------------------
- Script hardening: 2-4 days
- Tests & CI: 2-3 days
- PR pipeline updates & pre-commit hooks: 1-2 days
- Docs, QA & rollout ( manual coordination): 1-2 days
- Total: 6-11 business days (one-to-two weeks), may vary with availability of maintainers and CR feedback.

Deployment Checklist for Maintainers
----------------------------------
Before scheduling a destructive rewrite:
1. Verify all `bats` tests in `scripts/history-rewrite/tests` pass on CI.
2. Ensure backup branches and tags are pushed to `origin` (and optionally exported to external storage like an S3 bucket).
3. Confirm the PR uses `.github/PULL_REQUEST_TEMPLATE/history-rewrite.md` and the PR automation passes.
4. Run full `scripts/history-rewrite/clean_history.sh --dry-run` and `scripts/history-rewrite/preview_removals.sh --format json` locally and attach outputs to the PR.
5. Have at least two maintainers approve the destructive rewrite before pushing `git push --all --force`.

Development checklist
---------------------
 - [ ] Implement described script and validation changes.
 - [ ] Add `bats` tests and `history-rewrite` test CI workflow.
 - [ ] Add `data/backups/` to `.gitignore` and add pre-commit hooks to block accidental commits.
 - [ ] Update `pr-checklist.yml` to include tag-backup checks, backup logs, and PR content checks.
 - [ ] Add maintainers' docs and rollback examples.

Follow-ups / Outstanding Questions (ask maintainers)
--------------------------------------------------
- Should `data/backups` remain inside repo (but ignored) or be offloaded to a remote store before the rewrite?
- Should `clean_history.sh` create an optional tarball of `refs` and `tags` and push to `origin/backups/` or an alternate remote repository for longer term storage?
- For CI (bats) tests: do we want to install `bats-core` in the main CI image, or depend on an apt install in the `history-rewrite-tests` workflow?
- Is `git-filter-repo` present on official runner images or should we install it in the CI workflow each time? (script currently exits with `Please install git-filter-repo` advisory.)

Appendix: Example `bats` Test Skeleton (preview_removals)
------------------------------------------------------
You can start implementing the tests with `bats` like the following skeleton:

```
#!/usr/bin/env bats

setup() {
  repo_dir="$(mktemp -d)"
  cd "$repo_dir"
  git init -q
  mkdir -p backend/codeql-db
  echo "largefile" > backend/codeql-db/big.txt
  git add -A
  git commit -m "feat: add dummy codeql-db file" || exit 1
}

teardown() {
  rm -rf "$repo_dir"
}

@test "preview_removals reports commits in path" {
  run sh /workspace/scripts/history-rewrite/preview_removals.sh --paths 'backend/codeql-db' --strip-size 1
  [ "$status" -eq 0 ]
  [[ "$output" == *"Commits touching specified paths"* ]]
}
```

This same pattern can be reused to spawn a test repository and run `clean_history.sh --dry-run`, `validate_after_rewrite.sh` and assert expected outputs and exit codes.

Done.
# Investigation and Remediation Plan: CI Failures on feature/beta-release

## 1. Incident Summary
**Issue**: CI builds failing on `feature/beta-release`.
**Symptoms**:
- Frontend build fails due to missing module `../data/crowdsecPresets`.
- Backend coverage check fails (likely due to missing tests or artifacts).
- Docker build fails.
**Root Cause Identified**:
- The file `frontend/src/data/crowdsecPresets.ts` exists locally but was **ignored by git** due to an overly broad pattern in `.gitignore`.
- The pattern `data/` in `.gitignore` (intended for the root `data/` directory) accidentally matched `frontend/src/data/`.

## 2. Diagnosis Details
- **Local Environment**: The file `frontend/src/data/crowdsecPresets.ts` was present, so local `npm run build` and `npm run test:ci` passed.
- **CI Environment**: The file was missing because it was not committed.
- **Git Ignore Analysis**:
  - `.gitignore` contained `data/` under "Caddy Runtime Data".
  - This pattern matches any directory named `data` anywhere in the tree.
  - It matched `frontend/src/data/`, causing `crowdsecPresets.ts` to be ignored.

## 3. Remediation Steps
1.  **Fix `.gitignore`**:
    - Change `data/` to `/data/` to anchor it to the project root.
    - Change `frontend/frontend/` to `/frontend/frontend/` for safety.
2.  **Add Missing File**:
    - Force add or add `frontend/src/data/crowdsecPresets.ts` after fixing `.gitignore`.
3.  **Verify**:
    - Run `git check-ignore` to ensure the file is no longer ignored.
    - Run local build/test to ensure no regressions.

## 4. Verification Results
- **Local Tests**:
  - Backend Coverage: 85.4% (Pass)
  - Frontend Tests: 70 files passed (Pass)
  - Frontend Coverage: 85.97% (Pass)
  - Build: Passed
- **Git Status**:
  - `frontend/src/data/crowdsecPresets.ts` is now staged for commit.
  - `.gitignore` is modified and staged.

## 5. Next Actions
- Commit the changes with message: `fix: resolve CI failures by unignoring frontend data files`.
- Push to `feature/beta-release`.
- Monitor the next CI run.

## 6. Future Prevention
- Use anchored paths (starting with `/`) in `.gitignore` for root-level directories.
- Check `git status` for unexpected ignored files when adding new directories.
- Add a pre-commit check or CI step to verify that all imported modules exist in the git tree (though `tsc` in CI does this, the issue was the discrepancy between local and CI).
