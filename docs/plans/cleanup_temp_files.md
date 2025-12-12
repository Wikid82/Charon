# Cleanup Temporary Files Plan

## Problem

The pre-commit hook `check-added-large-files` failed because `backend/temp_index.json` and `hub_index.json` are staged. These are temporary files generated during CrowdSec Hub integration and should not be committed to the repository.

## Plan

### 1. Remove Files from Staging and Filesystem

- Unstage `backend/temp_index.json` and `hub_index.json` using `git restore --staged`.
- Remove these files from the filesystem using `rm`.

### 2. Update .gitignore

- Add `hub_index.json` to `.gitignore`.
- Add `temp_index.json` to `.gitignore` (or `backend/temp_index.json`).
- Add `backend/temp_index.json` specifically if `temp_index.json` is too broad, but `temp_index.json` seems safe as a general temp file name.

### 3. Verification

- Run `git status` to ensure files are ignored and not staged.
- Run pre-commit hooks again to verify they pass.

## Execution

I will proceed with these steps immediately.
