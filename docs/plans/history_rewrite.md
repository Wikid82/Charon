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
- git-filter-repo: install via package manager or pip. See https://github.com/newren/git-filter-repo.
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
