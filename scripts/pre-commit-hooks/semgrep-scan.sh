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

# Default to p/golang for speed (~30s vs 60-180s for auto).
# Override with: SEMGREP_CONFIG=auto git push
readonly SEMGREP_CONFIG_VALUE="${SEMGREP_CONFIG:-p/golang}"

echo "Running Semgrep with config: ${SEMGREP_CONFIG_VALUE}"
semgrep scan \
  --config "${SEMGREP_CONFIG_VALUE}" \
  --severity ERROR \
  --severity WARNING \
  --error \
  --exclude "frontend/node_modules" \
  --exclude "frontend/coverage" \
  --exclude "frontend/dist" \
  backend frontend/src scripts .github/workflows
