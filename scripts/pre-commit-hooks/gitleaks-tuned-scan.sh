#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly SCRIPT_DIR
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
readonly REPO_ROOT
readonly DEFAULT_REPORT_PATH="${REPO_ROOT}/test-results/security/gitleaks-tuned-precommit.json"
readonly REPORT_PATH="${GITLEAKS_REPORT_PATH:-${DEFAULT_REPORT_PATH}}"

if ! command -v rsync >/dev/null 2>&1; then
  echo "Error: rsync is not installed or not in PATH" >&2
  exit 127
fi

if ! command -v gitleaks >/dev/null 2>&1; then
  echo "Error: gitleaks is not installed or not in PATH" >&2
  echo "Install: https://github.com/gitleaks/gitleaks" >&2
  exit 127
fi

TEMP_ROOT="$(mktemp -d -t gitleaks-tuned-XXXXXX)"
cleanup() {
  rm -rf "${TEMP_ROOT}"
}
trap cleanup EXIT

readonly FILTERED_SOURCE="${TEMP_ROOT}/source-filtered"
mkdir -p "${FILTERED_SOURCE}"
mkdir -p "$(dirname "${REPORT_PATH}")"

cd "${REPO_ROOT}"

echo "Preparing filtered source tree for tuned gitleaks scan"
rsync -a --delete \
  --exclude='.cache/' \
  --exclude='node_modules/' \
  --exclude='frontend/node_modules/' \
  --exclude='backend/.venv/' \
  --exclude='dist/' \
  --exclude='build/' \
  --exclude='coverage/' \
  --exclude='test-results/' \
  ./ "${FILTERED_SOURCE}/"

echo "Running gitleaks tuned scan (no-git mode)"
gitleaks detect \
  --source "${FILTERED_SOURCE}" \
  --no-git \
  --report-format json \
  --report-path "${REPORT_PATH}" \
  --exit-code 1 \
  --no-banner

echo "Gitleaks report: ${REPORT_PATH}"
