#!/usr/bin/env bash
set -euo pipefail

# Integration Test All - Wrapper Script
# Executes the canonical integration test suite aligned with CI workflows

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"

exec bash "${PROJECT_ROOT}/scripts/integration-test-all.sh" "$@"
