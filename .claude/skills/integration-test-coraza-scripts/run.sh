#!/usr/bin/env bash
set -euo pipefail

# Integration Test Coraza - Wrapper Script
# Tests Coraza WAF integration

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"

# Delegate to the existing Coraza integration test script
exec "${PROJECT_ROOT}/scripts/coraza_integration.sh" "$@"
