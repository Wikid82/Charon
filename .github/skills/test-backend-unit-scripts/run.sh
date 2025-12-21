#!/usr/bin/env bash
# Test Backend Unit - Execution Script
#
# This script runs Go backend unit tests without coverage analysis,
# providing fast test execution for development workflows.

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
validate_go_environment "1.23" || error_exit "Go 1.23+ is required"

# Validate project structure
log_step "VALIDATION" "Checking project structure"
cd "${PROJECT_ROOT}"
validate_project_structure "backend" || error_exit "Invalid project structure"

# Change to backend directory
cd "${PROJECT_ROOT}/backend"

# Execute tests
log_step "EXECUTION" "Running backend unit tests"

# Run go test with all passed arguments
if go test "$@" ./...; then
    log_success "Backend unit tests passed"
    exit 0
else
    exit_code=$?
    log_error "Backend unit tests failed (exit code: ${exit_code})"
    exit "${exit_code}"
fi
