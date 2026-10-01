#!/usr/bin/env bash
set -euo pipefail

# Integration Test Security Headers - Wrapper Script
# Tests security header profile handling

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"

exec "${PROJECT_ROOT}/scripts/security_headers_integration.sh" "$@"
