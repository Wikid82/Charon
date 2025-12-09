#!/bin/sh
# Verify repository health after a destructive history-rewrite
set -eu

usage() {
  cat <<EOF
Usage: $0

Performs: git gc, git fsck, pre-commit, backend tests, frontend build.
EOF
}

if [ "$#" -gt 0 ]; then
  usage; exit 1
fi

echo "Running git maintenance: git count-objects -vH"
git count-objects -vH || true

echo "Running git fsck --full"
git fsck --full || true

if [ -x "./.venv/bin/pre-commit" ]; then
  echo "Running pre-commit checks"
  ./.venv/bin/pre-commit run --all-files || echo "pre-commit checks reported issues"
else
  echo "pre-commit not found at ./.venv/bin/pre-commit; please run in your environment to validate."
fi

if [ -d backend ]; then
  echo "Running backend go tests"
  (cd backend && go test ./... -v) || echo "backend tests failed"
fi

if [ -d frontend ]; then
  echo "Running frontend build"
  (cd frontend && npm run build) || echo "frontend build failed"
fi

echo "Validation complete. Inspect output for errors. If something is wrong, restore:
  git checkout -b restore/$(date +"%Y%m%d-%H%M%S") $backup_branch"

exit 0
