#!/usr/bin/env bash
set -euo pipefail

# Integration Test All - Wrapper Script
# Executes the comprehensive integration test suite

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"

# Delegate to the existing integration test script
exec "${PROJECT_ROOT}/scripts/integration-test.sh" "$@"
