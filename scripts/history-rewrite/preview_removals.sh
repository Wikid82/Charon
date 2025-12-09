#!/bin/sh
# Preview the list of commits and objects that would be removed by clean_history.sh
set -eu

PATHS="backend/codeql-db,codeql-db,codeql-db-js,codeql-db-go"
STRIP_SIZE=50

usage() {
  cat <<EOF
Usage: $0 [--paths 'p1,p2'] [--strip-size N]

Prints commits and objects that would be removed by a history rewrite.
EOF
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --paths)
      PATHS="$2"; shift 2;;
    --strip-size)
      STRIP_SIZE="$2"; shift 2;;
    --help)
      usage; exit 0;;
    *)
      echo "Unknown option: $1" >&2; usage; exit 1;;
  esac
done

IFS=','; set -f
paths_list=""
for p in $PATHS; do
  paths_list="$paths_list $p"
done
set +f; unset IFS

echo "Paths: $paths_list"
echo "Strip blobs larger than: ${STRIP_SIZE}M"

echo "--- Commits touching specified paths ---"
for p in $paths_list; do
  echo "Path: $p"
  git rev-list --all -- "$p" | nl -ba | sed -n '1,50p'
done

echo "--- Objects in paths ---"
git rev-list --objects --all -- $paths_list | nl -ba | sed -n '1,100p'

echo "--- Example large objects larger than ${STRIP_SIZE}M ---"
git rev-list --objects --all | awk '{print $1}' | while read oid; do
  size=$(git cat-file -s "$oid" 2>/dev/null || true)
  if [ -n "$size" ] && [ "$size" -ge $((STRIP_SIZE * 1024 * 1024)) ]; then
    echo "$oid size=$size"
  fi
done | nl -ba | sed -n '1,50p'

echo "Preview complete. Use clean_history.sh --dry-run to get a log file."

exit 0
