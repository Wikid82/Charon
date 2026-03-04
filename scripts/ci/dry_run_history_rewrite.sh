#!/usr/bin/env bash
set -euo pipefail

# CI wrapper that fails if the repo contains historical objects or commits
# touching specified paths, or objects larger than the configured strip size.

PATHS="backend/codeql-db,codeql-db,codeql-db-js,codeql-db-go"
STRIP_SIZE=50

usage() {
  cat <<EOF
Usage: $0 [--paths 'p1,p2'] [--strip-size N]

Runs a quick, non-destructive check against the repository history and fails
with a non-zero exit code if any commits or objects are found that touch the
specified paths or if any historical blobs exceed the --strip-size in MB.
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

echo "Checking repository history for banned paths: $paths_list"
echo "Blobs larger than: ${STRIP_SIZE}M will fail the check"

failed=0

# 1) Check for commits touching paths
for p in $paths_list; do
  count=$(git rev-list --all -- "$p" | wc -l | tr -d ' ')
  if [ "$count" -gt 0 ]; then
    echo "ERROR: Found $count historical commit(s) touching path: $p"
    git rev-list --all -- "$p" | nl -ba | sed -n '1,50p'
    echo "DRY-RUN FAILED: historical commits detected"
    exit 1
  else
    echo "OK: No history touching: $p"
  fi
done

# 2) Check for blob objects in paths only (ignore tag/commit objects)
# Temp files
tmp_objects=$(mktemp)
blob_list=$(mktemp)
# shellcheck disable=SC2086 # $paths_list is intentionally unquoted to expand into multiple args
git rev-list --objects --all -- $paths_list > "$tmp_objects"
blob_count=0
tmp_oids="$(mktemp)"
trap 'rm -f "$tmp_objects" "$blob_list" "$tmp_oids"' EXIT INT TERM
while read -r line; do
  oid=$(printf '%s' "$line" | awk '{print $1}')
  # Determine object type and only consider blobs
  type=$(git cat-file -t "$oid" 2>/dev/null || true)
  if [ "$type" = "blob" ]; then
    echo "$line" >> "$blob_list"
    blob_count=$((blob_count + 1))
  fi
done < "$tmp_objects"

if [ "$blob_count" -gt 0 ]; then
  echo "ERROR: Found $blob_count blob object(s) in specified paths"
  nl -ba "$blob_list" | sed -n '1,100p'
  echo "DRY-RUN FAILED: repository blob objects found in banned paths"
  exit 1
else
  echo "OK: No repository blob objects in specified paths"
fi

# 3) Check for large objects across history
echo "Scanning for objects larger than ${STRIP_SIZE}M..."
large_found=0
# Write all object oids to a temp file to avoid a subshell problem
tmp_oids="$(mktemp)"
git rev-list --objects --all | awk '{print $1}' > "$tmp_oids"
while read -r oid; do
  size=$(git cat-file -s "$oid" 2>/dev/null || echo 0)
  if [ -n "$size" ] && [ "$size" -ge $((STRIP_SIZE * 1024 * 1024)) ]; then
    echo "LARGE OBJECT: $oid size=$size"
    large_found=1
    failed=1
  fi
done < "$tmp_oids"
if [ "$large_found" -eq 0 ]; then
  echo "OK: No large objects detected across history"
else
  echo "DRY-RUN FAILED: large historical blobs detected"
  exit 1
fi

if [ "$failed" -ne 0 ]; then
  echo "DRY-RUN FAILED: Repository history contains blocked entries"
  exit 1
fi

echo "DRY-RUN OK: No problems detected"
exit 0
