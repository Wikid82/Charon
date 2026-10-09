#!/usr/bin/env bash
set -euo pipefail

# Integration Test CrowdSec Data Persistence - Wrapper Script
# Verifies hub data files survive container recreation

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"

exec "${PROJECT_ROOT}/scripts/crowdsec_data_persistence_test.sh" "$@"
