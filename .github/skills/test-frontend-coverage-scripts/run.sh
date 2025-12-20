#!/usr/bin/env bash
# Test Frontend Coverage - Execution Script
#
# This script wraps the legacy frontend-test-coverage.sh script while providing
# the Agent Skills interface and logging.

set -euo pipefail

# Source helper scripts
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# Helper scripts are in .github/skills/scripts/
SKILLS_SCRIPTS_DIR="$(cd "${SCRIPT_DIR}/../scripts" && pwd)"

# shellcheck source=../scripts/_logging_helpers.sh
source "${SKILLS_SCRIPTS_DIR}/_logging_helpers.sh"
# shellcheck source=../scripts/_error_handling_helpers.sh
source "${SKILLS_SCRIPTS_DIR}/_error_handling_helpers.sh"
# shellcheck source=../scripts/_environment_helpers.sh
source "${SKILLS_SCRIPTS_DIR}/_environment_helpers.sh"

# Project root is 3 levels up from this script (skills/skill-name-scripts/run.sh -> project root)
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"

# Validate environment
log_step "ENVIRONMENT" "Validating prerequisites"
validate_node_environment "18.0" || error_exit "Node.js 18.0+ is required"
validate_python_environment "3.8" || error_exit "Python 3.8+ is required"

# Validate project structure
log_step "VALIDATION" "Checking project structure"
cd "${PROJECT_ROOT}"
validate_project_structure "frontend" "scripts/frontend-test-coverage.sh" || error_exit "Invalid project structure"

# Set default environment variables
set_default_env "CHARON_MIN_COVERAGE" "85"

# Execute the legacy script
log_step "EXECUTION" "Running frontend tests with coverage"
log_info "Minimum coverage: ${CHARON_MIN_COVERAGE}%"

LEGACY_SCRIPT="${PROJECT_ROOT}/scripts/frontend-test-coverage.sh"
check_file_exists "${LEGACY_SCRIPT}"

# Execute with proper error handling
if "${LEGACY_SCRIPT}" "$@"; then
    log_success "Frontend coverage tests passed"
    exit 0
else
    exit_code=$?
    log_error "Frontend coverage tests failed (exit code: ${exit_code})"
    exit "${exit_code}"
fi
