#!/usr/bin/env bash
set -euo pipefail

# Integration Test Setup Code - Wrapper Script
# Tests the first-run setup code bootstrap path

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"

exec "${PROJECT_ROOT}/scripts/setup_code_integration.sh" "$@"
