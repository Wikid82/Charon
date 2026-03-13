#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly SCRIPT_DIR
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
readonly REPO_ROOT

if ! command -v semgrep >/dev/null 2>&1; then
  echo "Error: semgrep is not installed or not in PATH" >&2
  echo "Install: https://semgrep.dev/docs/getting-started/" >&2
  exit 127
fi

cd "${REPO_ROOT}"

# Default: full security ruleset covering Go backend, JS/TS/React frontend, secrets.
# Override with: SEMGREP_CONFIG=auto git commit (runs all Semgrep rules, ~3-5 min)
if [ -n "${SEMGREP_CONFIG:-}" ]; then
  SEMGREP_CONFIGS=(--config "${SEMGREP_CONFIG}")
  echo "Running Semgrep with override config: ${SEMGREP_CONFIG}"
else
  SEMGREP_CONFIGS=(
    --config p/golang
    --config p/javascript
    --config p/react
    --config p/secrets
  )
  echo "Running Semgrep with configs: p/golang, p/javascript, p/react, p/secrets"
fi

semgrep scan \
  "${SEMGREP_CONFIGS[@]}" \
  --severity ERROR \
  --severity WARNING \
  --error \
  --exclude "frontend/node_modules" \
  --exclude "frontend/coverage" \
  --exclude "frontend/dist" \
  backend frontend/src scripts .github/workflows
