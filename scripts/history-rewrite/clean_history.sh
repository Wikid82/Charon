#!/usr/bin/env bash
# Bash script to safely preview and optionally run a git history rewrite
set -euo pipefail
IFS=$'\n\t'

# Default values
DRY_RUN=1
FORCE=0
NON_INTERACTIVE=0
PATHS="backend/codeql-db,codeql-db,codeql-db-js,codeql-db-go"
STRIP_SIZE=50

usage() {
  cat <<EOF
Usage: $0 [--dry-run] [--force] [--paths 'p1,p2'] [--strip-size N]

Options:
  --dry-run         (default) Show what would be removed; no changes are made.
  --force           Run rewrite (destructive). Requires manual confirmation.
  --paths           Comma-separated list of paths to remove from history.
  --strip-size      Strip blobs larger than N MB in the history.
  --help            Show this help and exit.

Example:
  $0 --dry-run --paths 'backend/codeql-db,codeql-db' --strip-size 50
  $0 --force --paths 'backend/codeql-db' --strip-size 100
EOF
}

check_requirements() {
  if ! command -v git >/dev/null 2>&1; then
    echo "git is required but not found. Aborting." >&2
    exit 1
  fi
  if ! command -v git-filter-repo >/dev/null 2>&1; then
    echo "git-filter-repo not found. Please install it:"
    echo "  - Debian/Ubuntu: sudo apt install git-filter-repo"
    echo "  - Mac (Homebrew): brew install git-filter-repo"
    echo "  - Python pip: pip install git-filter-repo"
    echo "Or see https://github.com/newren/git-filter-repo for details."
    exit 2
  fi
}

timestamp() {
  # POSIX-friendly timestamp
  date +"%Y%m%d-%H%M%S"
}

logdir="data/backups"
mkdir -p "$logdir"
logfile="$logdir/history_cleanup-$(timestamp).log"

echo "Starting history cleanup tool at $(date)" | tee "$logfile"

while [ "$#" -gt 0 ]; do
  case "$1" in
    --dry-run)
      DRY_RUN=1; shift;;
    --force)
      DRY_RUN=0; FORCE=1; shift;;
    --non-interactive)
      NON_INTERACTIVE=1; shift;;
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

check_requirements

# Reject shallow clones
if git rev-parse --is-shallow-repository >/dev/null 2>&1 && [ "$(git rev-parse --is-shallow-repository 2>/dev/null)" = "true" ]; then
  echo "Shallow clone detected; fetch full history before rewriting history. Run: git fetch --unshallow or actions/checkout: fetch-depth: 0 in CI." | tee -a "$logfile"
  exit 4
fi

current_branch=$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "(detached)")
if [ "$current_branch" = "main" ] || [ "$current_branch" = "master" ]; then
  if [ "$FORCE" -ne 1 ]; then
    echo "Refusing to run on main/master branch. Switch to a feature branch and retry. To force running on main/master set FORCE=1" | tee -a "$logfile"
    exit 3
  fi
  echo "WARNING: Running on main/master as FORCE=1 is set." | tee -a "$logfile"
fi

backup_branch="backup/history-$(timestamp)"
echo "Creating backup branch: $backup_branch" | tee -a "$logfile"
git branch -f "$backup_branch" || true
if ! git push origin "$backup_branch" >/dev/null 2>&1; then
  echo "Error: Failed to push backup branch $backup_branch to origin. Aborting." | tee -a "$logfile"
  exit 5
fi

IFS=','; set -f
paths_list=""
for p in $PATHS; do
  # Expand shell expansion
  paths_list="$paths_list $p"
done
set +f; unset IFS

echo "Paths targeted: $paths_list" | tee -a "$logfile"
echo "Strip blobs bigger than: ${STRIP_SIZE}M" | tee -a "$logfile"

# Ensure STRIP_SIZE is numeric
if ! printf '%s\n' "$STRIP_SIZE" | grep -Eq '^[0-9]+$'; then
  echo "Error: --strip-size must be a numeric value (MB). Got: $STRIP_SIZE" | tee -a "$logfile"
  exit 6
fi

preview_removals() {
  echo "=== Preview: commits & blobs touching specified paths ===" | tee -a "$logfile"
  # List commits that touch the paths
  for p in $paths_list; do
    echo "--- Path: $p" | tee -a "$logfile"
    git rev-list --all -- "$p" | head -n 20 | tee -a "$logfile"
  done
  echo "=== End of commit preview ===" | tee -a "$logfile"

  echo "=== Preview: objects in paths ===" | tee -a "$logfile"
  # List objects for the given paths
  for p in $paths_list; do
    echo "Path: $p" | tee -a "$logfile"
    git rev-list --objects --all -- "$p" | while read -r line; do
      oid=$(printf '%s' "$line" | awk '{print $1}')
      label=$(printf '%s' "$line" | awk '{print $2}')
      type=$(git cat-file -t "$oid" 2>/dev/null || true)
      if [ "$type" = "blob" ]; then
        echo "$oid $label"
      else
        echo "[${type^^}] $oid $label"
      fi
    done | head -n 50 | tee -a "$logfile"
  done

  echo "=== Example large objects (candidate for --strip-size) ===" | tee -a "$logfile"
  # List object sizes and show top N
  git rev-list --objects --all | awk '{print $1}' | while read -r oid; do
    size=$(git cat-file -s "$oid" 2>/dev/null || true)
    if [ -n "$size" ] && [ "$size" -ge $((STRIP_SIZE * 1024 * 1024)) ]; then
      echo "$oid size=$size"
    fi
  done | head -n 30 | tee -a "$logfile"
}

if [ "$DRY_RUN" -eq 1 ]; then
  echo "Running dry-run mode. No destructive operations will be performed." | tee -a "$logfile"
  preview_removals
  echo "Dry-run complete. See $logfile for details." | tee -a "$logfile"
  exit 0
fi

if [ "$FORCE" -ne 1 ]; then
  echo "To run a destructive rewrite, pass --force. Aborting." | tee -a "$logfile"
  exit 1
fi

echo "FORCE mode enabled - performing rewrite. This is destructive and will rewrite history." | tee -a "$logfile"

if [ "$NON_INTERACTIVE" -eq 0 ]; then
  echo "Confirm operation: Type 'I UNDERSTAND' to proceed:" | tee -a "$logfile"
  read -r confirmation
  if [ "$confirmation" != "I UNDERSTAND" ]; then
    echo "Confirmation not provided. Aborting." | tee -a "$logfile"
    exit 1
  fi
else
  if [ "$FORCE" -ne 1 ]; then
    echo "Error: Non-interactive mode requires FORCE=1 to proceed. Aborting." | tee -a "$logfile"
    exit 1
  fi
fi

## No additional branch check here; earlier check prevents running on main/master unless FORCE=1

# Build git-filter-repo arguments
paths_args=""
IFS=' '
for p in $paths_list; do
  paths_args="$paths_args --paths $p"
done
set +f

echo "Running git filter-repo with: $paths_args --invert-paths --strip-blobs-bigger-than ${STRIP_SIZE}M" | tee -a "$logfile"

echo "Performing a local dry-run against a local clone before actual rewrite is strongly recommended." | tee -a "$logfile"

 # shellcheck disable=SC2086
set -- $paths_args
git filter-repo --invert-paths "$@" --strip-blobs-bigger-than "${STRIP_SIZE}"M | tee -a "$logfile"

echo "Rewrite complete. Running post-rewrite checks..." | tee -a "$logfile"
git count-objects -vH | tee -a "$logfile"
git fsck --full | tee -a "$logfile"
git gc --aggressive --prune=now | tee -a "$logfile"

# Backup tags list as a tarball and try to push tags to a backup namespace
tags_tar="$logdir/tags-$(timestamp).tar.gz"
tmp_tags_dir=$(mktemp -d)
git for-each-ref --format='%(refname:short) %(objectname)' refs/tags > "$tmp_tags_dir/tags.txt"
tar -C "$tmp_tags_dir" -czf "$tags_tar" . || echo "Warning: failed to create tag tarball" | tee -a "$logfile"
rm -rf "$tmp_tags_dir"
echo "Created tags tarball: $tags_tar" | tee -a "$logfile"

echo "Attempting to push tags to origin under refs/backups/tags/*" | tee -a "$logfile"
for t in $(git tag --list); do
  if ! git push origin "refs/tags/$t:refs/backups/tags/$t" >/dev/null 2>&1; then
    echo "Warning: pushing tag $t to refs/backups/tags/$t failed" | tee -a "$logfile"
  fi
done

echo "REWRITE DONE. Next steps (manual):" | tee -a "$logfile"
cat <<EOF | tee -a "$logfile"
  - Verify repo locally and run CI checks: ./.venv/bin/pre-commit run --all-files
  - Run backend tests: cd backend && go test ./...
  - Run frontend build: cd frontend && npm run build
  - Coordinate with maintainers prior to force-push. To finalize:
      git push --all --force
      git push --tags --force
  - If anything goes wrong, restore from your backup branch: git checkout -b restore/$(date +"%Y%m%d-%H%M%S") $backup_branch
EOF

echo "Log saved to $logfile"

exit 0
