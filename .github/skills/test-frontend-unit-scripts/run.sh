#!/usr/bin/env bash
# Test Frontend Unit - Execution Script
#
# This script runs frontend unit tests without coverage analysis,
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
validate_node_environment "18.0" || error_exit "Node.js 18.0+ is required"

# Validate project structure
log_step "VALIDATION" "Checking project structure"
cd "${PROJECT_ROOT}"
validate_project_structure "frontend" || error_exit "Invalid project structure"

# Change to frontend directory
cd "${PROJECT_ROOT}/frontend"

# Execute tests
log_step "EXECUTION" "Running frontend unit tests"

# Run npm test with all passed arguments
if npm run test -- "$@"; then
    log_success "Frontend unit tests passed"
    exit 0
else
    exit_code=$?
    log_error "Frontend unit tests failed (exit code: ${exit_code})"
    exit "${exit_code}"
fi
