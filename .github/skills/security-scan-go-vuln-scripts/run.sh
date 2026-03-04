#!/usr/bin/env bash
# Security Scan Go Vulnerability - Execution Script
#
# This script wraps the Go vulnerability checker (govulncheck) to detect
# known vulnerabilities in Go code and dependencies.

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
validate_go_environment "1.23" || error_exit "Go 1.23+ is required"

# Set defaults
set_default_env "GOVULNCHECK_FORMAT" "text"

# Parse arguments
FORMAT="${1:-${GOVULNCHECK_FORMAT}}"
MODE="${2:-source}"

# Validate format
case "${FORMAT}" in
    text|json|sarif)
        ;;
    *)
        log_error "Invalid format: ${FORMAT}. Must be one of: text, json, sarif"
        exit 1
        ;;
esac

# Validate mode
case "${MODE}" in
    source|binary)
        ;;
    *)
        log_error "Invalid mode: ${MODE}. Must be one of: source, binary"
        exit 1
        ;;
esac

# Change to backend directory
cd "${PROJECT_ROOT}/backend"

# Check for go.mod
if [[ ! -f "go.mod" ]]; then
    log_error "go.mod not found in backend directory"
    exit 1
fi

# Execute govulncheck
log_step "SCANNING" "Running Go vulnerability check"
log_info "Format: ${FORMAT}"
log_info "Mode: ${MODE}"
log_info "Working directory: $(pwd)"

# Build govulncheck command
GOVULNCHECK_CMD="go run golang.org/x/vuln/cmd/govulncheck@latest"

# Add format flag if not text (text is default)
if [[ "${FORMAT}" != "text" ]]; then
    GOVULNCHECK_CMD="${GOVULNCHECK_CMD} -format=${FORMAT}"
fi

# Add mode flag if not source (source is default)
if [[ "${MODE}" != "source" ]]; then
    GOVULNCHECK_CMD="${GOVULNCHECK_CMD} -mode=${MODE}"
fi

# Add target (all packages)
GOVULNCHECK_CMD="${GOVULNCHECK_CMD} ./..."

# Execute the scan
if eval "${GOVULNCHECK_CMD}"; then
    log_success "No vulnerabilities found"
    exit 0
else
    exit_code=$?
    if [[ ${exit_code} -eq 3 ]]; then
        log_error "Vulnerabilities detected (exit code 3)"
        log_info "Review the output above for details and remediation advice"
    else
        log_error "Vulnerability scan failed with exit code: ${exit_code}"
    fi
    exit "${exit_code}"
fi
