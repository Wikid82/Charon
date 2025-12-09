#!/bin/sh
# POSIX shell script to safely preview and optionally run a git history rewrite
set -eu

# Default values
DRY_RUN=1
FORCE=0
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

current_branch=$(git rev-parse --abbrev-ref HEAD 2>/dev/null || echo "(detached)")
if [ "$current_branch" = "main" ] || [ "$current_branch" = "master" ]; then
  echo "Refusing to run on main/master branch. Switch to a feature branch and retry." | tee -a "$logfile"
  exit 3
fi

backup_branch="backup/history-$(timestamp)"
echo "Creating backup branch: $backup_branch" | tee -a "$logfile"
git branch -f "$backup_branch" || true
git push origin "$backup_branch" || echo "Warning: push failed, ensure remote origin exists and push manually." | tee -a "$logfile"

IFS=','; set -f
paths_list=""
for p in $PATHS; do
  # Expand shell expansion
  paths_list="$paths_list $p"
done
set +f; unset IFS

echo "Paths targeted: $paths_list" | tee -a "$logfile"
echo "Strip blobs bigger than: ${STRIP_SIZE}M" | tee -a "$logfile"

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
  git rev-list --objects --all -- $paths_list | tee -a "$logfile" | awk '{print $1, $2}' | head -n 50 | tee -a "$logfile"

  echo "=== Example large objects (candidate for --strip-size) ===" | tee -a "$logfile"
  # List object sizes and show top N
  git rev-list --objects --all | awk '{print $1}' | while read oid; do
    size=$(git cat-file -s "$oid" 2>/dev/null || true)
    if [ -n "$size" ] && [ "$size" -ge $((STRIP_SIZE * 1024 * 1024)) ]; then
      echo "$oid size=$size" | tee -a "$logfile"
    fi
  done | head -n 30
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

echo "Confirm operation: Type 'I UNDERSTAND' to proceed:" | tee -a "$logfile"
read -r confirmation
if [ "$confirmation" != "I UNDERSTAND" ]; then
  echo "Confirmation not provided. Aborting." | tee -a "$logfile"
  exit 1
fi

if [ "$current_branch" = "main" ] || [ "$current_branch" = "master" ]; then
  echo "Refusing to run filter-repo on main/master. Switch to a safe branch and retry." | tee -a "$logfile"
  exit 1
fi

# Build git-filter-repo arguments
paths_args=""
IFS=' '
for p in $paths_list; do
  paths_args="$paths_args --paths $p"
done
set +f

echo "Running git filter-repo with: $paths_args --invert-paths --strip-blobs-bigger-than ${STRIP_SIZE}M" | tee -a "$logfile"

echo "Performing a local dry-run against a local clone before actual rewrite is strongly recommended." | tee -a "$logfile"

git filter-repo --invert-paths $paths_args --strip-blobs-bigger-than ${STRIP_SIZE}M | tee -a "$logfile"

echo "Rewrite complete. Running post-rewrite checks..." | tee -a "$logfile"
git count-objects -vH | tee -a "$logfile"
git fsck --full | tee -a "$logfile"
git gc --aggressive --prune=now | tee -a "$logfile"

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
