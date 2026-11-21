#!/usr/bin/env bash
set -euo pipefail

# Integration Test CrowdSec - Wrapper Script
# Tests CrowdSec bouncer integration

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"

# Delegate to the existing CrowdSec integration test script
exec "${PROJECT_ROOT}/scripts/crowdsec_integration.sh" "$@"
