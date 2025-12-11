#!/usr/bin/env bash
# Preview the list of commits and objects that would be removed by clean_history.sh
set -euo pipefail
IFS=$'\n\t'

PATHS="backend/codeql-db,codeql-db,codeql-db-js,codeql-db-go"
STRIP_SIZE=50
FORMAT="text"

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
    --format)
      FORMAT="$2"; shift 2;;
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

# Reject shallow clones
if git rev-parse --is-shallow-repository >/dev/null 2>&1 && [ "$(git rev-parse --is-shallow-repository 2>/dev/null)" = "true" ]; then
  echo "Error: Shallow clone detected. Please run 'git fetch --unshallow' or use actions/checkout fetch-depth: 0 to fetch full history." >&2
  exit 2
fi

# Ensure STRIP_SIZE is numeric
if ! printf '%s\n' "$STRIP_SIZE" | grep -Eq '^[0-9]+$'; then
  echo "Error: --strip-size must be a numeric value (MB). Got: $STRIP_SIZE" >&2
  exit 3
fi

if [ "$FORMAT" = "json" ]; then
  printf '{"paths":['
  first_path=true
  for p in $paths_list; do
    if [ "$first_path" = true ]; then
      printf '"%s"' "$p"
      first_path=false
    else
      printf ',"%s"' "$p"
    fi
  done
  printf '],"strip_size":%s,"commits":{' "$STRIP_SIZE"
fi
echo "--- Commits touching specified paths ---"
for p in $paths_list; do
  if [ "$FORMAT" = "json" ]; then
    printf '"%s":[' "$p"
    git rev-list --all -- "$p" | head -n 50 | awk '{printf "%s\n", $0}' | sed -n '1,50p' | awk '{printf "%s,", $0}' | sed 's/,$//'
    printf '],'
  else
    echo "Path: $p"
    git rev-list --all -- "$p" | nl -ba | sed -n '1,50p'
  fi
done

if [ "$FORMAT" = "json" ]; then
  printf '},"objects":['
  for p in $paths_list; do
    git rev-list --objects --all -- "$p" | head -n 100 | awk '{printf "\"%s\",", $1}' | sed 's/,$//'
  done
  printf '],'
else
  echo "--- Objects in paths (blob objects shown; tags highlighted) ---"
    for p in $paths_list; do
      echo "Path: $p"
      git rev-list --objects --all -- "$p" | while read -r line; do
        oid=$(printf '%s' "$line" | awk '{print $1}')
        label=$(printf '%s' "$line" | awk '{print $2}')
        type=$(git cat-file -t "$oid" 2>/dev/null || true)
        if [ "$type" = "blob" ]; then
          echo "$oid  $label"
        else
          echo "[${type^^}] $oid  $label"
        fi
      done | nl -ba | sed -n '1,100p'
    done
fi

echo "--- Example large objects larger than ${STRIP_SIZE}M ---"
git rev-list --objects --all | awk '{print $1}' | while read -r oid; do
  size=$(git cat-file -s "$oid" 2>/dev/null || true)
      if [ -n "$size" ] && [ "$size" -ge $((STRIP_SIZE * 1024 * 1024)) ]; then
        if [ "$FORMAT" = "json" ]; then
          printf '{"oid":"%s","size":%s},' "$oid" "$size"
        else
          echo "$oid size=$size"
        fi
  fi
done | nl -ba | sed -n '1,50p'

if [ "$FORMAT" = "json" ]; then
  printf '],"large_objects":[]}'
  echo
else
  echo "Preview complete. Use clean_history.sh --dry-run to get a log file."
fi

exit 0
