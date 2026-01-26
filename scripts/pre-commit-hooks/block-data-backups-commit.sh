#!/usr/bin/env bash
set -euo pipefail
IFS=$'\n\t'

# Prevent committing any files under data/backups/ accidentally
staged_files=$(git diff --cached --name-only || true)
if [ -z "$staged_files" ]; then
  exit 0
fi

for f in $staged_files; do
  case "$f" in
    data/backups/*)
      echo "Error: Committing files under data/backups/ is blocked. Remove them from the commit and re-run." >&2
      exit 1
      ;;
  esac
done

exit 0
