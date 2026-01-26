<!-- PR: History Rewrite & Large-file Removal -->

## Summary

- Provide a short summary of why the history rewrite is needed.

## Checklist - required for history rewrite PRs

- [ ] I have created a **local** backup branch: `backup/history-YYYYMMDD-HHMMSS` and verified it contains all refs.
- [ ] I have pushed the backup branch to the remote origin and it is visible to reviewers.
- [ ] I have run a dry-run locally: `scripts/history-rewrite/preview_removals.sh --paths 'backend/codeql-db,codeql-db,codeql-db-js,codeql-db-go' --strip-size 50` and attached the output or paste it below.
- [ ] I have verified the `data/backups` tarball is present and tests showing rewrite will not remove unrelated artifacts.
- [ ] I have created a tag backup (see `data/backups/`) and verified tags are pushed to the remote or included in the tarball.
- [ ] I have coordinated with repo maintainers for a rewrite window and notified other active forks/tokens that may be affected.
- [ ] I have run the CI dry-run job and ensured it completes without blocked findings.
- [ ] This PR only contains the history-rewrite helpers; no destructive rewrite is included in this PR.
- [ ] I will not run the destructive `--force` step without explicit approval from maintainers and a scheduled maintenance window.

**Note for maintainers**: `validate_after_rewrite.sh` will check that the `backups` and `backup_branch` are present and will fail if they are not. Provide `--backup-branch "backup/history-YYYYMMDD-HHMMSS"` when running the scripts or set the `BACKUP_BRANCH` environment variable so automated validation can find the backup branch.

## Attachments

Attach the `preview_removals` output and `data/backups/history_cleanup-*.log` content and any `data/backups` tarball created for this PR.

## Approach

Describe the paths to be removed, strip size, and whether additional blob stripping is required.

# Notes for maintainers

- The workflow `.github/workflows/dry-run-history-rewrite.yml` will run automatically on PR updates.
- Please follow the checklist and only approve after offline confirmation.
