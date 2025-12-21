#!/usr/bin/env bash
# Security Scan Trivy - Execution Script
#
# This script wraps the Trivy Docker command to scan for vulnerabilities,
# secrets, and misconfigurations.

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
validate_docker_environment || error_exit "Docker is required but not available"

# Set defaults
set_default_env "TRIVY_SEVERITY" "CRITICAL,HIGH,MEDIUM"
set_default_env "TRIVY_TIMEOUT" "10m"

# Parse arguments
SCANNERS="${1:-vuln,secret,misconfig}"
FORMAT="${2:-table}"

# Validate format
case "${FORMAT}" in
    table|json|sarif)
        ;;
    *)
        log_error "Invalid format: ${FORMAT}. Must be one of: table, json, sarif"
        exit 2
        ;;
esac

# Validate scanners
IFS=',' read -ra SCANNER_ARRAY <<< "${SCANNERS}"
for scanner in "${SCANNER_ARRAY[@]}"; do
    case "${scanner}" in
        vuln|secret|misconfig)
            ;;
        *)
            log_error "Invalid scanner: ${scanner}. Must be one of: vuln, secret, misconfig"
            exit 2
            ;;
    esac
done

# Execute Trivy scan
log_step "SCANNING" "Running Trivy security scan"
log_info "Scanners: ${SCANNERS}"
log_info "Format: ${FORMAT}"
log_info "Severity: ${TRIVY_SEVERITY}"
log_info "Timeout: ${TRIVY_TIMEOUT}"

cd "${PROJECT_ROOT}"

# Run Trivy via Docker
if docker run --rm \
    -v "$(pwd):/app:ro" \
    -e "TRIVY_SEVERITY=${TRIVY_SEVERITY}" \
    -e "TRIVY_TIMEOUT=${TRIVY_TIMEOUT}" \
    aquasec/trivy:latest \
    fs \
    --scanners "${SCANNERS}" \
    --format "${FORMAT}" \
    /app; then
    log_success "Trivy scan completed - no issues found"
    exit 0
else
    exit_code=$?
    if [[ ${exit_code} -eq 1 ]]; then
        log_error "Trivy scan found security issues"
    else
        log_error "Trivy scan failed with exit code: ${exit_code}"
    fi
    exit "${exit_code}"
fi
