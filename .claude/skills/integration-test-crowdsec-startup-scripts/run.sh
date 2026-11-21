#!/usr/bin/env bash
set -euo pipefail

# Integration Test CrowdSec Startup - Wrapper Script
# Tests CrowdSec startup sequence and initialization

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"

# Delegate to the existing CrowdSec startup test script
exec "${PROJECT_ROOT}/scripts/crowdsec_startup_test.sh" "$@"
