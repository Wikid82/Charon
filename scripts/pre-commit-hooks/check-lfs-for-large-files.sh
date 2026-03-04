#!/usr/bin/env bash
set -euo pipefail

# pre-commit hook: ensure large files added to git are tracked by Git LFS
MAX_BYTES=$((50 * 1024 * 1024))
FAILED=0

STAGED_FILES=$(git diff --cached --name-only --diff-filter=ACM)
if [ -z "$STAGED_FILES" ]; then
  exit 0
fi

while read -r f; do
  [ -z "$f" ] && continue
  if [ -f "$f" ]; then
    size=$(stat -c%s "$f")
    if [ "$size" -gt "$MAX_BYTES" ]; then
      # check if tracked by LFS via git check-attr
      filter_attr=$(git check-attr --stdin filter <<<"$f" | awk '{print $3}' || true)
      if [ "$filter_attr" != "lfs" ]; then
        echo "ERROR: Large file not tracked by Git LFS: $f ($size bytes)" >&2
        FAILED=1
      fi
    fi
  fi
done <<<"$STAGED_FILES"

if [ $FAILED -ne 0 ]; then
  echo "You must track large files in Git LFS. Aborting commit." >&2
  exit 1
fi

exit 0
