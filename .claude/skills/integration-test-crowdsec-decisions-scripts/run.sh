#!/usr/bin/env bash
set -euo pipefail

# Integration Test CrowdSec Decisions - Wrapper Script
# Tests CrowdSec decision API functionality

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"

# Delegate to the existing CrowdSec decision integration test script
exec "${PROJECT_ROOT}/scripts/crowdsec_decision_integration.sh" "$@"
