#!/usr/bin/env bash
set -euo pipefail
staged=$(git diff --cached --name-only | tr '\r' '\n' || true)
if [ -n "${staged}" ]; then
  # Exclude the pre-commit-hooks directory and this script itself
  filtered=$(echo "$staged" | grep -v '^scripts/pre-commit-hooks/' | grep -v '^data/backups/' || true)
  if echo "$filtered" | grep -q "codeql-db"; then
    echo "Error: Attempting to commit CodeQL database artifacts (codeql-db)." >&2
    echo "These should not be committed. Remove them or add to .gitignore and try again." >&2
    echo "Tip: Use 'scripts/repo_health_check.sh' to validate repository health." >&2
    exit 1
  fi
fi
exit 0
