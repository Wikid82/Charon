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

readonly SEMGREP_CONFIG_VALUE="${SEMGREP_CONFIG:-auto}"

echo "Running Semgrep with config: ${SEMGREP_CONFIG_VALUE}"
semgrep scan \
  --config "${SEMGREP_CONFIG_VALUE}" \
  --error \
  backend frontend scripts .github/workflows
