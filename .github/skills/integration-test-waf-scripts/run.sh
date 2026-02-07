#!/usr/bin/env bash
set -euo pipefail

# Integration Test WAF - Wrapper Script
# Tests generic WAF integration

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"

exec "${PROJECT_ROOT}/scripts/waf_integration.sh" "$@"
