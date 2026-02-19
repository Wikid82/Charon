#!/usr/bin/env bash
set -euo pipefail

# Integration Test Rate Limit - Wrapper Script
# Tests rate limit integration

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"

exec "${PROJECT_ROOT}/scripts/rate_limit_integration.sh" "$@"
