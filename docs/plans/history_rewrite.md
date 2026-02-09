# History Rewrite: Plan, Checklist, and Recovery

## Summary

- This document describes the agreed process, checks, and recovery steps for destructive history rewrites performed with the scripts in `scripts/history-rewrite/`.
- It updates the previous guidance by adding explicit backup requirements, tag backups, and a `--backup-branch` argument or `BACKUP_BRANCH` env variable that must be set and pushed to a remote before running a destructive rewrite.

## Minimum Requirements

- Tools: `git` (>=2.25), `git-filter-repo` (Python-based utility), `pre-commit`.
- Optional tools: `bats-core` for tests, `shellcheck` for linting scripts.

## Overview

Use the `preview_removals.sh` script to preview which commits/objects will be removed. Always run `clean_history.sh` with `--dry-run` and create a remote backup branch and a tag backup tarball in `data/backups/` before any destructive operation. After a rewrite, run `validate_after_rewrite.sh` to confirm the repository matches expectations.

## Naming Conventions & Backup Policy

- Backup branch name format: `backup/history-YYYYMMDD-HHMMSS`.
- Tag backup tarball: `data/backups/tags-YYYYMMDD-HHMMSS.tar.gz`.
- Metadata: `data/backups/history-YYYYMMDD-HHMMSS.json` with keys `backup_branch`, `tag_tar`, `created_at`, `remote`.

## Checklist (Before a Destructive Rewrite)

1. Run the preview step and attach output to the PR:
   - `scripts/history-rewrite/preview_removals.sh --paths 'backend/codeql-db' --strip-size 50 --format json`
   - Attach the output (or paste it into the PR) for reviewer consumption.
2. Create a local and remote backup branch:
   - `git checkout -b backup/history-YYYYMMDD-HHMMSS`
   - `git push origin backup/history-YYYYMMDD-HHMMSS`
   - Record the branch name in `--backup-branch` or set `BACKUP_BRANCH` env var so validators can find it.
3. Capture tags:
   - `git tag -l | xargs -n1 git show-ref --tags` and push tags to the origin, or create a tarball of tags in `data/backups/`.
   - Example tag tarball: `git for-each-ref --format='%(refname)' refs/tags/ | xargs -n1 git rev-parse --verify --quiet | tar -czf data/backups/tags-YYYYMMDD-HHMMSS.tar.gz --files-from -` (create a scripted helper if needed).
4. Ensure `data/backups` exists and is included as a tarball or log attachment in the PR:
   - `mkdir -p data/backups && tar -czf data/backups/history-YYYYMMDD-HHMMSS.tar.gz data/backups/` (if logs are present).
5. Run the CI dry-run job and ensure it completes successfully. If `dry-run` reports findings, address them first.
6. Ensure maintainers approve and that you have a scheduled maintenance window. Do not run a destructive `--force` push without explicit approvals.

## Typical Usage Examples

Preview candidates to remove:

```bash
scripts/history-rewrite/preview_removals.sh --paths 'backend/codeql-db,import' --strip-size 50 --format json
```

Create a backup branch and push:

```bash
git checkout -b backup/history-$(date -u +%Y%m%d-%H%M%S)
git push origin HEAD
export BACKUP_BRANCH=$(git rev-parse --abbrev-ref HEAD)
```

Create a tarball of tags and save logs in `data/backups/`:

```bash
mkdir -p data/backups
git for-each-ref --format='%(refname)' refs/tags/ | xargs -n1 -I{} git show-ref --tags {} >> data/backups/tags-$(date -u +%Y%m%d-%H%M%S).txt
tar -czf data/backups/tags-$(date -u +%Y%m%d-%H%M%S).tar.gz data/backups/*
```

Dry-run the rewrite (do not push):

```bash
scripts/history-rewrite/clean_history.sh --paths 'backend/codeql-db,import' --strip-size 50 --dry-run --backup-branch "$BACKUP_BRANCH"
```

Perform the rewrite (coordinated action, after approvals):

```bash
scripts/history-rewrite/clean_history.sh --paths 'backend/codeql-db,import' --strip-size 50 --backup-branch "$BACKUP_BRANCH" --force
# After local rewrite, force-push coordinated with maintainers: `git push origin --all --force`
```

Validate after rewrite:

```bash
scripts/history-rewrite/validate_after_rewrite.sh --backup-branch "$BACKUP_BRANCH"
```

## Recovery Steps (if things go wrong)

1. Ensure your local clone still has the `backup/history-...` branch. If the branch was pushed to origin, check it using:
   - `git ls-remote origin | grep backup/history-` or `git fetch origin backup/history-YYYY...`.
2. Restore the branch to a new or restored head:
   - `git checkout -b restore-YYYY backup/history-YYYYMMDD-HHMMSS`
   - `git push origin restore-YYYY` and open a PR to restore history.
3. For tags: restore from tarball or tag list by re-creating tags and pushing them to the remote:
   - `tar -xzf data/backups/tags-YYYYMMDD-HHMMSS.tar.gz -C /tmp/tags
   - Recreate tags as needed and `git push origin --tags`.
4. If a destructive push changed history on remote: coordinate with maintainers to either push restore branches or restore from the backup branch using `git push origin refs/heads/restore-YYYY:refs/heads/main` (requires a maintainers-only action).

## Checklist for PR Reviewers

- Confirm `data/backups` is present or attached in the PR.
- Confirm the backup branch (`backup/history-YYYYMMDD-HHMMSS`) is pushed to origin.
- Confirm tag backups exist and are included in the backup tarball.
- Ensure `preview_removals` output is attached to the PR as evidence.
- Ensure maintainers have scheduled the maintenance window and have approved the change.

## Notes & Safety

- Avoid running destructive pushes from forks without a coordinated maintainers plan.
- The default behavior of the scripts is non-destructive (`--dry-run`)—use `--force` only after approvals.
- The `validate_after_rewrite.sh` script accepts `--backup-branch` or reads `BACKUP_BRANCH` env var; make sure it's present (or the script will exit non-zero).

---
For implementation details, see `scripts/history-rewrite/` and current CI workflows that run the script tests.

History rewrite plan
====================

Rationale
---------

Some committed CodeQL DB directories or large binary blobs can bloat clones, CI cache sizes, and repository size overall. This plan provides a non-destructive, auditable history-rewrite solution to remove these directories and optionally strip out huge blobs.

Scope
-----

This plan targets CodeQL DB directories (e.g., backend/codeql-db, codeql-db, codeql-db-js, codeql-db-go) and other large blobs. Scripts are non-destructive by default and require `--force` to make destructive changes.

Risk & Mitigation
-----------------

- Rewriting history changes commit hashes. We never force-push in the scripts automatically; the maintainer must coordinate before running `git push --force`.
- Always create a backup branch before rewriting; the script creates `backup/history-YYYYMMDD-HHMMSS` and pushes it to `origin`.
- Require the manual confirmation string `I UNDERSTAND` before running any destructive change.

Overview of steps
-----------------

1. Prepare: create and checkout a non-main feature branch (do not run on `main` or `master`).
2. Dry-run and preview: run a dry-run to preview commits and blobs to remove.
   - `scripts/history-rewrite/clean_history.sh --dry-run --paths 'backend/codeql-db,codeql-db' --strip-size 50`
3. Optional detailed preview:
   - `scripts/history-rewrite/preview_removals.sh --paths 'backend/codeql-db,codeql-db' --strip-size 50`
4. With approval, run the destructive rewrite in a local clone or dedicated environment.
   - `scripts/history-rewrite/clean_history.sh --force --paths 'backend/codeql-db,codeql-db' --strip-size 50`
   - When prompted, type `I UNDERSTAND` to proceed.
5. Validation: run the validator script and ensure CI passes locally:
   - `scripts/history-rewrite/validate_after_rewrite.sh`
6. Coordinate with maintainers and force-push only after consensus.

Installation & prerequisites
----------------------------

- git >= 2.25
- git-filter-repo: install via package manager or pip. See <https://github.com/newren/git-filter-repo>.
- pre-commit (optional): installed in the repository virtual environment (`.venv`).

Sample commands and dry-run outputs
----------------------------------

Dry-run:

```
scripts/history-rewrite/clean_history.sh --dry-run --paths 'backend/codeql-db,codeql-db' --strip-size 50
```

Sample dry-run output (excerpt):

--- Path: backend/codeql-db
2b7c6f8d1a... (commits touching this path)
--- Objects in paths
f6a9abcd... backend/codeql-db/project.sarif
--- Example large objects (candidate for --strip-size)
f3ae1234... size=104857600

Force-run (coordination required):

```
scripts/history-rewrite/clean_history.sh --force --paths 'backend/codeql-db,codeql-db' --strip-size 50
```

Followed by verification and manual force-push:

- Check `data/backups/history_cleanup-YYYYMMDD-HHMMSS.log`
- `scripts/history-rewrite/validate_after_rewrite.sh`
- `git push --all --force` (only after maintainers approve)

Rollback plan
-------------

If problems occur, restore from the backup branch:

  git checkout -b restore/YYYYMMDD-HHMMSS backup/history-YYYYMMDD-HHMMSS
  git push origin restore/YYYYMMDD-HHMMSS

Post rewrite maintenance
------------------------

- Run `git gc --aggressive --prune=now` on clones and local copies.
- Run `git count-objects -vH` to confirm size improvements.
- Refresh CI caches and mirrors after the change.

Communication & Approval
------------------------

Open a PR with dry-run logs and `preview_removals` output, tag maintainers for approval before `--force` is used.

CI automation
-------------

- A CI dry-run workflow `.github/workflows/dry-run-history-rewrite.yml` runs a non-destructive check that fails CI when banned history entries or large objects are found. It is triggered on PRs and a daily schedule.
- A PR checklist template `.github/PULL_REQUEST_TEMPLATE/history-rewrite.md` and a checklist validator `.github/workflows/pr-checklist.yml` ensure contributors attach the preview output and backups before seeking approval.
- The PR checklist validator is conditional: it only enforces the checklist when the PR modifies `scripts/history-rewrite/*`, `docs/plans/history_rewrite.md`, or similar history-rewrite related files. This avoids blocking unrelated PRs.
