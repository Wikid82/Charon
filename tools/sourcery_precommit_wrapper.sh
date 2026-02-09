#!/usr/bin/env bash
set -euo pipefail

# Wrapper for Sourcery pre-commit hook.
# Run Sourcery if the CLI is available or a token is provided.
# This supports both interactive `sourcery login` and token-based CI usage.

if command -v sourcery >/dev/null 2>&1; then
  exec sourcery "$@"
fi

# Try python -m sourcery as a fallback
if python -m sourcery --version >/dev/null 2>&1; then
  exec python -m sourcery "$@"
fi

# If CLI not found but token env var present, try to run via 'sourcery' anyway
if [ -n "${SOURCERY_TOKEN:-}" ] || [ -n "${SOURCERY_API_TOKEN:-}" ] || [ -n "${SOURCERY_API_KEY:-}" ]; then
  if command -v sourcery >/dev/null 2>&1; then
    exec sourcery "$@"
  fi
  if python -m sourcery --version >/dev/null 2>&1; then
    exec python -m sourcery "$@"
  fi
fi

echo "Sourcery CLI not available and no token detected; skipping sourcery pre-commit check."
exit 0
