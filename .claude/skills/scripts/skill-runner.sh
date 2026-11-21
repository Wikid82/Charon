#!/usr/bin/env bash
# Agent Skills Universal Skill Runner
#
# This script locates and executes Agent Skills by name, providing a unified
# interface for running skills from tasks.json, CI/CD workflows, and the CLI.
#
# Usage:
#   skill-runner.sh <skill-name> [args...]
#
# Exit Codes:
#   0   - Skill executed successfully
#   1   - Skill not found or invalid
#   2   - Skill execution failed
#   126 - Skill script not executable
#   127 - Skill script not found

set -euo pipefail

# Source helper scripts
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=_logging_helpers.sh
source "${SCRIPT_DIR}/_logging_helpers.sh"
# shellcheck source=_error_handling_helpers.sh
source "${SCRIPT_DIR}/_error_handling_helpers.sh"
# shellcheck source=_environment_helpers.sh
source "${SCRIPT_DIR}/_environment_helpers.sh"

# Configuration
SKILLS_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
PROJECT_ROOT="$(cd "${SKILLS_DIR}/../.." && pwd)"

# Validate arguments
if [[ $# -eq 0 ]]; then
    log_error "Usage: skill-runner.sh <skill-name> [args...]"
    log_error "Example: skill-runner.sh test-backend-coverage"
    exit 1
fi

SKILL_NAME="$1"
shift  # Remove skill name from arguments

# Validate skill name format
if [[ ! "${SKILL_NAME}" =~ ^[a-z][a-z0-9-]*$ ]]; then
    log_error "Invalid skill name: ${SKILL_NAME}"
    log_error "Skill names must be kebab-case (lowercase, hyphens, start with letter)"
    exit 1
fi

# Verify SKILL.md exists
SKILL_FILE="${SKILLS_DIR}/${SKILL_NAME}.SKILL.md"
if [[ ! -f "${SKILL_FILE}" ]]; then
    log_error "Skill not found: ${SKILL_NAME}"
    log_error "Expected file: ${SKILL_FILE}"
    log_info "Available skills:"
    for skill_file in "${SKILLS_DIR}"/*.SKILL.md; do
        if [[ -f "${skill_file}" ]]; then
            basename "${skill_file}" .SKILL.md
        fi
    done | sort | sed 's/^/  - /'
    exit 1
fi

# Locate skill execution script (flat structure: skill-name-scripts/run.sh)
SKILL_SCRIPT="${SKILLS_DIR}/${SKILL_NAME}-scripts/run.sh"

if [[ ! -f "${SKILL_SCRIPT}" ]]; then
    log_error "Skill execution script not found: ${SKILL_SCRIPT}"
    log_error "Expected: ${SKILL_NAME}-scripts/run.sh"
    exit 1
fi

if [[ ! -x "${SKILL_SCRIPT}" ]]; then
    log_error "Skill execution script is not executable: ${SKILL_SCRIPT}"
    log_error "Fix with: chmod +x ${SKILL_SCRIPT}"
    exit 126
fi

# Log skill execution
log_info "Executing skill: ${SKILL_NAME}"
log_debug "Skill file: ${SKILL_FILE}"
log_debug "Skill script: ${SKILL_SCRIPT}"
log_debug "Working directory: ${PROJECT_ROOT}"
log_debug "Arguments: $*"

# Change to project root for execution
cd "${PROJECT_ROOT}"

# Execute skill with all remaining arguments
# shellcheck disable=SC2294
if ! "${SKILL_SCRIPT}" "$@"; then
    log_error "Skill execution failed: ${SKILL_NAME}"
    exit 2
fi

log_success "Skill completed successfully: ${SKILL_NAME}"
exit 0
