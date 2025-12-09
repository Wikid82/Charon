#!/usr/bin/env bash
# Verify repository health after a destructive history-rewrite
set -euo pipefail
IFS=$'\n\t'

usage() {
  cat <<EOF
Usage: $0 [--backup-branch BRANCH]

Performs: sanity checks after a destructive history-rewrite.
Options:
  --backup-branch BRANCH   Name of the backup branch created prior to rewrite.
  -h, --help               Show this help and exit.
EOF
}

backup_branch=""

while [ "${#}" -gt 0 ]; do
  case "$1" in
    --backup-branch)
      shift
      if [ -z "${1:-}" ]; then
        echo "Error: --backup-branch requires an argument" >&2
        usage
        exit 2
      fi
      backup_branch="$1"
      shift
      ;;
    -h|--help)
      usage; exit 0
      ;;
    *)
      echo "Unknown argument: $1" >&2; usage; exit 2
      ;;
  esac
done

# Fallback to env variable
if [ -z "${backup_branch}" ]; then
  if [ -n "${BACKUP_BRANCH:-}" ]; then
    backup_branch="$BACKUP_BRANCH"
  fi
fi

# If still not set, try to infer from data/backups logs
if [ -z "${backup_branch}" ] && [ -d data/backups ]; then
  # Look for common patterns referencing a backup branch name
  candidate=$(grep -E "backup[-_]branch" data/backups/* 2>/dev/null | sed -E 's/.*[:=]//; s/^[[:space:]]+//; s/[[:space:]\047"\"]+$//' | head -n1 || true)
  if [ -n "${candidate}" ]; then
    backup_branch="$candidate"
  fi
fi

if [ -z "${backup_branch}" ]; then
  echo "Error: backup branch not provided. Use --backup-branch or set BACKUP_BRANCH environment variable, or ensure data/backups/ contains a log referencing the branch." >&2
  exit 3
fi

# No positional args required; any unknown options are handled during parsing

echo "Running git maintenance: git count-objects -vH"
git count-objects -vH || true

echo "Running git fsck --full"
git fsck --full || true

pre_commit_executable=""
if [ -x "./.venv/bin/pre-commit" ]; then
  pre_commit_executable="./.venv/bin/pre-commit"
elif command -v pre-commit >/dev/null 2>&1; then
  pre_commit_executable=$(command -v pre-commit)
fi

if [ -z "${pre_commit_executable}" ]; then
  echo "Error: pre-commit not found. Install pre-commit in a virtualenv at ./.venv/bin/pre-commit or ensure it's in PATH." >&2
  exit 4
fi

echo "Running pre-commit checks (${pre_commit_executable})"
${pre_commit_executable} run --all-files || { echo "pre-commit checks reported issues" >&2; exit 5; }

if [ -d backend ]; then
  echo "Running backend go tests"
  (cd backend && go test ./... -v) || echo "backend tests failed"
fi

if [ -d frontend ]; then
  echo "Running frontend build"
  (cd frontend && npm run build) || echo "frontend build failed"
fi

echo "Validation complete. Inspect output for errors. If something is wrong, restore:
  git checkout -b restore/$(date +"%Y%m%d-%H%M%S") ${backup_branch:-}"

exit 0
