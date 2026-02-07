#!/usr/bin/env bash
set -euo pipefail

# Integration Test Cerberus - Wrapper Script
# Tests Cerberus full-stack integration

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"

exec "${PROJECT_ROOT}/scripts/cerberus_integration.sh" "$@"
