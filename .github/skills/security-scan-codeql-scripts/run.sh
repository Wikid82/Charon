#!/usr/bin/env bash
# Security Scan CodeQL - Execution Script
#
# This script runs CodeQL security analysis using the security-and-quality
# suite to match GitHub Actions CI configuration exactly.

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

# Some helper scripts may not define ANSI color variables; ensure they exist
# before using them later in this script (set -u is enabled).
RED="${RED:-\033[0;31m}"
GREEN="${GREEN:-\033[0;32m}"
NC="${NC:-\033[0m}"

PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"

# Set defaults
set_default_env "CODEQL_THREADS" "0"
set_default_env "CODEQL_FAIL_ON_ERROR" "true"

# Parse arguments
LANGUAGE="${1:-all}"
FORMAT="${2:-summary}"

# Validate language
case "${LANGUAGE}" in
    go|javascript|js|all)
        ;;
    *)
        log_error "Invalid language: ${LANGUAGE}. Must be one of: go, javascript, all"
        exit 2
        ;;
esac

# Normalize javascript -> js for internal use
if [[ "${LANGUAGE}" == "javascript" ]]; then
    LANGUAGE="js"
fi

# Validate format
case "${FORMAT}" in
    sarif|text|summary)
        ;;
    *)
        log_error "Invalid format: ${FORMAT}. Must be one of: sarif, text, summary"
        exit 2
        ;;
esac

# Validate CodeQL is installed
log_step "ENVIRONMENT" "Validating CodeQL installation"
if ! command -v codeql &> /dev/null; then
    log_error "CodeQL CLI is not installed"
    log_info "Install via: gh extension install github/gh-codeql"
    log_info "Then run: gh codeql set-version latest"
    exit 2
fi

# Check CodeQL version
CODEQL_VERSION=$(codeql version 2>/dev/null | head -1 | grep -oP '\d+\.\d+\.\d+' || echo "unknown")
log_info "CodeQL version: ${CODEQL_VERSION}"

# Minimum version check
MIN_VERSION="2.17.0"
if [[ "${CODEQL_VERSION}" != "unknown" ]]; then
    if [[ "$(printf '%s\n' "${MIN_VERSION}" "${CODEQL_VERSION}" | sort -V | head -n1)" != "${MIN_VERSION}" ]]; then
        log_warning "CodeQL version ${CODEQL_VERSION} may be incompatible"
        log_info "Recommended: gh codeql set-version latest"
    fi
fi

cd "${PROJECT_ROOT}"

# Track findings
GO_ERRORS=0
GO_WARNINGS=0
JS_ERRORS=0
JS_WARNINGS=0
SCAN_FAILED=0

# Function to run CodeQL scan for a language
run_codeql_scan() {
    local lang=$1
    local source_root=$2
    local db_name="codeql-db-${lang}"
    local sarif_file="codeql-results-${lang}.sarif"
    local build_mode_args=()
    local codescanning_config="${PROJECT_ROOT}/.github/codeql/codeql-config.yml"

    # Remove generated artifacts that can create noisy/false findings during CodeQL analysis
    rm -rf "${PROJECT_ROOT}/frontend/coverage" \
          "${PROJECT_ROOT}/frontend/dist" \
          "${PROJECT_ROOT}/playwright-report" \
          "${PROJECT_ROOT}/test-results" \
          "${PROJECT_ROOT}/coverage"

    if [[ "${lang}" == "javascript" ]]; then
        build_mode_args=(--build-mode=none)
    fi

    log_step "CODEQL" "Scanning ${lang} code in ${source_root}/"

    # Clean previous database
    rm -rf "${db_name}"

    # Create database
    log_info "Creating CodeQL database..."
    if ! codeql database create "${db_name}" \
        --language="${lang}" \
        "${build_mode_args[@]}" \
        --source-root="${source_root}" \
        --codescanning-config="${codescanning_config}" \
        --threads="${CODEQL_THREADS}" \
        --overwrite 2>&1 | while read -r line; do
            # Filter verbose output, show important messages
            if [[ "${line}" == *"error"* ]] || [[ "${line}" == *"Error"* ]]; then
                log_error "${line}"
            elif [[ "${line}" == *"warning"* ]]; then
                log_warning "${line}"
            fi
        done; then
        log_error "Failed to create CodeQL database for ${lang}"
        return 1
    fi

    # Run analysis
    log_info "Analyzing with Code Scanning config (CI-aligned query filters)..."
    if ! codeql database analyze "${db_name}" \
        --format=sarif-latest \
        --output="${sarif_file}" \
        --sarif-add-baseline-file-info \
        --threads="${CODEQL_THREADS}" 2>&1; then
        log_error "CodeQL analysis failed for ${lang}"
        return 1
    fi

    log_success "SARIF output: ${sarif_file}"

    # Parse results
    if command -v jq &> /dev/null && [[ -f "${sarif_file}" ]]; then
        local total_findings
        local error_count
        local warning_count
        local note_count

        total_findings=$(jq '.runs[].results | length' "${sarif_file}" 2>/dev/null || echo 0)
        error_count=$(jq '[.runs[].results[] | select(.level == "error")] | length' "${sarif_file}" 2>/dev/null || echo 0)
        warning_count=$(jq '[.runs[].results[] | select(.level == "warning")] | length' "${sarif_file}" 2>/dev/null || echo 0)
        note_count=$(jq '[.runs[].results[] | select(.level == "note")] | length' "${sarif_file}" 2>/dev/null || echo 0)

        log_info "Found: ${error_count} errors, ${warning_count} warnings, ${note_count} notes (${total_findings} total)"

        # Store counts for global tracking
        if [[ "${lang}" == "go" ]]; then
            GO_ERRORS=${error_count}
            GO_WARNINGS=${warning_count}
        else
            JS_ERRORS=${error_count}
            JS_WARNINGS=${warning_count}
        fi

        # Show findings based on format
        if [[ "${FORMAT}" == "text" ]] || [[ "${FORMAT}" == "summary" ]]; then
            if [[ ${total_findings} -gt 0 ]]; then
                echo ""
                log_info "Top findings:"
                jq -r '.runs[].results[] | "\(.level): \(.message.text | split("\n")[0]) (\(.locations[0].physicalLocation.artifactLocation.uri):\(.locations[0].physicalLocation.region.startLine))"' "${sarif_file}" 2>/dev/null | head -15
                echo ""
            fi
        fi

        # Check for blocking errors
        if [[ ${error_count} -gt 0 ]]; then
            log_error "${lang}: ${error_count} HIGH/CRITICAL findings detected"
            return 1
        fi
    else
        log_warning "jq not available - install for detailed analysis"
    fi

    return 0
}

# Run scans based on language selection
if [[ "${LANGUAGE}" == "all" ]] || [[ "${LANGUAGE}" == "go" ]]; then
    if ! run_codeql_scan "go" "backend"; then
        SCAN_FAILED=1
    fi
fi

if [[ "${LANGUAGE}" == "all" ]] || [[ "${LANGUAGE}" == "js" ]]; then
    if ! run_codeql_scan "javascript" "frontend"; then
        SCAN_FAILED=1
    fi
fi

# Final summary
echo ""
log_step "SUMMARY" "CodeQL Security Scan Results"
echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

if [[ "${LANGUAGE}" == "all" ]] || [[ "${LANGUAGE}" == "go" ]]; then
    if [[ ${GO_ERRORS} -gt 0 ]]; then
        echo -e "  Go:         ${RED}${GO_ERRORS} errors${NC}, ${GO_WARNINGS} warnings"
    else
        echo -e "  Go:         ${GREEN}0 errors${NC}, ${GO_WARNINGS} warnings"
    fi
fi

if [[ "${LANGUAGE}" == "all" ]] || [[ "${LANGUAGE}" == "js" ]]; then
    if [[ ${JS_ERRORS} -gt 0 ]]; then
        echo -e "  JavaScript: ${RED}${JS_ERRORS} errors${NC}, ${JS_WARNINGS} warnings"
    else
        echo -e "  JavaScript: ${GREEN}0 errors${NC}, ${JS_WARNINGS} warnings"
    fi
fi

echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"

# Exit based on findings
if [[ "${CODEQL_FAIL_ON_ERROR}" == "true" ]] && [[ ${SCAN_FAILED} -eq 1 ]]; then
    log_error "CodeQL scan found HIGH/CRITICAL issues - fix before proceeding"
    echo ""
    log_info "View results:"
    log_info "  VS Code:  Install SARIF Viewer extension, open codeql-results-*.sarif"
    log_info "  CLI:      jq '.runs[].results[]' codeql-results-*.sarif"
    exit 1
else
    log_success "CodeQL scan complete - no blocking issues"
    exit 0
fi
