#!/usr/bin/env bash
# QA Pre-commit All - Execution Script
#
# This script runs all pre-commit hooks for comprehensive code quality validation.

set -euo pipefail

# Source helper scripts
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SKILLS_SCRIPTS_DIR="$(cd "${SCRIPT_DIR}/../scripts" && pwd)"

# shellcheck source=../scripts/_logging_helpers.sh
source "${SKILLS_SCRIPTS_DIR}/_logging_helpers.sh"
# shellcheck source=../scripts/_error_handling_helpers.sh
source "${SKILLS_SCRIPTS_DIR}/_error_handling_helpers.sh"
# shellcheck source=../scripts/_environment_helpers.sh
source "${SKILLS_SCRIPTS_DIR}/_environment_helpers.sh"

PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"

# Validate environment
log_step "ENVIRONMENT" "Validating prerequisites"
validate_python_environment "3.8" || error_exit "Python 3.8+ is required"

# Check for virtual environment
if [[ -z "${VIRTUAL_ENV:-}" ]]; then
    log_warning "Virtual environment not activated, attempting to activate .venv"
    if [[ -f "${PROJECT_ROOT}/.venv/bin/activate" ]]; then
        # shellcheck source=/dev/null
        source "${PROJECT_ROOT}/.venv/bin/activate"
        log_info "Activated virtual environment: ${VIRTUAL_ENV}"
    else
        error_exit "Virtual environment not found at ${PROJECT_ROOT}/.venv"
    fi
fi

# Check for pre-commit
if ! command -v pre-commit &> /dev/null; then
    error_exit "pre-commit not found. Install with: pip install pre-commit"
fi

# Parse arguments
FILES_MODE="${1:---all-files}"

# Validate files mode
case "${FILES_MODE}" in
    --all-files|staged)
        ;;
    *)
        # If not a recognized mode, treat as a specific hook ID
        HOOK_ID="${FILES_MODE}"
        FILES_MODE="--all-files"
        log_info "Running specific hook: ${HOOK_ID}"
        ;;
esac

# Change to project root
cd "${PROJECT_ROOT}"

# Execute pre-commit
log_step "VALIDATION" "Running pre-commit hooks"
log_info "Files mode: ${FILES_MODE}"

if [[ -n "${SKIP:-}" ]]; then
    log_info "Skipping hooks: ${SKIP}"
fi

# Build pre-commit command
PRE_COMMIT_CMD="pre-commit run"

# Handle files mode
if [[ "${FILES_MODE}" == "staged" ]]; then
    # Run on staged files only (no flag needed, this is default for 'pre-commit run')
    log_info "Running on staged files only"
else
    PRE_COMMIT_CMD="${PRE_COMMIT_CMD} --all-files"
fi

# Add specific hook if provided
if [[ -n "${HOOK_ID:-}" ]]; then
    PRE_COMMIT_CMD="${PRE_COMMIT_CMD} ${HOOK_ID}"
fi

# Execute the validation
log_info "Executing: ${PRE_COMMIT_CMD}"

if eval "${PRE_COMMIT_CMD}"; then
    log_success "All pre-commit hooks passed"
    exit 0
else
    exit_code=$?
    log_error "One or more pre-commit hooks failed (exit code: ${exit_code})"
    log_info "Review the output above for details"
    log_info "Some hooks can auto-fix issues - review and commit changes if appropriate"
    exit "${exit_code}"
fi
