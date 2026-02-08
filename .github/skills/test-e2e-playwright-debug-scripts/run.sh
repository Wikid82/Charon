#!/usr/bin/env bash
# Test E2E Playwright Debug - Execution Script
#
# Runs Playwright E2E tests in headed/debug mode with slow motion,
# optional Inspector, and trace collection for troubleshooting.

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

# Project root is 3 levels up from this script
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"

# Default parameter values
FILE=""
GREP=""
SLOWMO=500
INSPECTOR=false
PROJECT="firefox"

# Parse command-line arguments
parse_arguments() {
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --file=*)
                FILE="${1#*=}"
                shift
                ;;
            --file)
                FILE="${2:-}"
                shift 2
                ;;
            --grep=*)
                GREP="${1#*=}"
                shift
                ;;
            --grep)
                GREP="${2:-}"
                shift 2
                ;;
            --slowmo=*)
                SLOWMO="${1#*=}"
                shift
                ;;
            --slowmo)
                SLOWMO="${2:-500}"
                shift 2
                ;;
            --inspector)
                INSPECTOR=true
                shift
                ;;
            --project=*)
                PROJECT="${1#*=}"
                shift
                ;;
            --project)
                PROJECT="${2:-chromium}"
                shift 2
                ;;
            -h|--help)
                show_help
                exit 0
                ;;
            *)
                log_warning "Unknown argument: $1"
                shift
                ;;
        esac
    done
}

# Show help message
show_help() {
    cat << EOF
Usage: run.sh [OPTIONS]

Run Playwright E2E tests in debug mode for troubleshooting.

Options:
    --file=FILE         Specific test file to run (relative to tests/)
    --grep=PATTERN      Filter tests by title pattern (regex)
    --slowmo=MS         Delay between actions in milliseconds (default: 500)
    --inspector         Open Playwright Inspector for step-by-step debugging
    --project=PROJECT   Browser to use: chromium, firefox, webkit (default: firefox)
    -h, --help          Show this help message

Environment Variables:
    PLAYWRIGHT_BASE_URL     Application URL to test (default: http://localhost:8080)
    PWDEBUG                 Set to '1' for Inspector mode
    DEBUG                   Verbose logging (e.g., 'pw:api')

Examples:
    run.sh                                  # Debug all tests in Firefox
    run.sh --file=login.spec.ts            # Debug specific file
    run.sh --grep="login"                  # Debug tests matching pattern
    run.sh --inspector                     # Open Playwright Inspector
    run.sh --slowmo=1000                   # Slower execution
    run.sh --file=test.spec.ts --inspector # Combine options
EOF
}

# Validate project parameter
validate_project() {
    local valid_projects=("chromium" "firefox" "webkit")
    local project_lower
    project_lower=$(echo "${PROJECT}" | tr '[:upper:]' '[:lower:]')

    for valid in "${valid_projects[@]}"; do
        if [[ "${project_lower}" == "${valid}" ]]; then
            PROJECT="${project_lower}"
            return 0
        fi
    done

    error_exit "Invalid project '${PROJECT}'. Valid options: chromium, firefox, webkit"
}

# Validate test file if specified
validate_test_file() {
    if [[ -z "${FILE}" ]]; then
        return 0
    fi

    local test_path="${PROJECT_ROOT}/tests/${FILE}"

    # Handle if user provided full path
    if [[ "${FILE}" == tests/* ]]; then
        test_path="${PROJECT_ROOT}/${FILE}"
        FILE="${FILE#tests/}"
    fi

    if [[ ! -f "${test_path}" ]]; then
        log_error "Test file not found: ${test_path}"
        log_info "Available test files:"
        ls -1 "${PROJECT_ROOT}/tests/"*.spec.ts 2>/dev/null | xargs -n1 basename || true
        error_exit "Invalid test file"
    fi
}

# Build Playwright command arguments
build_playwright_args() {
    local args=()

    # Always run headed in debug mode
    args+=("--headed")

    # Add project
    args+=("--project=${PROJECT}")

    # Add grep filter if specified
    if [[ -n "${GREP}" ]]; then
        args+=("--grep=${GREP}")
    fi

    # Always collect traces in debug mode
    args+=("--trace=on")

    # Run single worker for clarity
    args+=("--workers=1")

    # No retries in debug mode
    args+=("--retries=0")

    echo "${args[*]}"
}

# Main execution
main() {
    parse_arguments "$@"

    # Validate environment
    log_step "ENVIRONMENT" "Validating prerequisites"
    validate_node_environment "18.0" || error_exit "Node.js 18+ is required"
    check_command_exists "npx" "npx is required (part of Node.js installation)"

    # Validate project structure
    log_step "VALIDATION" "Checking project structure"
    cd "${PROJECT_ROOT}"
    validate_project_structure "tests" "playwright.config.js" "package.json" || error_exit "Invalid project structure"

    # Validate parameters
    validate_project
    validate_test_file

    # Set environment variables
    export PLAYWRIGHT_HTML_OPEN="${PLAYWRIGHT_HTML_OPEN:-never}"
    set_default_env "PLAYWRIGHT_BASE_URL" "http://localhost:8080"

    # Enable Inspector if requested
    if [[ "${INSPECTOR}" == "true" ]]; then
        export PWDEBUG=1
        log_info "Playwright Inspector enabled"
    fi

    # Log configuration
    log_step "CONFIG" "Debug configuration"
    log_info "Project: ${PROJECT}"
    log_info "Test file: ${FILE:-<all tests>}"
    log_info "Grep filter: ${GREP:-<none>}"
    log_info "Slow motion: ${SLOWMO}ms"
    log_info "Inspector: ${INSPECTOR}"
    log_info "Base URL: ${PLAYWRIGHT_BASE_URL}"

    # Build command arguments
    local playwright_args
    playwright_args=$(build_playwright_args)

    # Determine test path
    local test_target=""
    if [[ -n "${FILE}" ]]; then
        test_target="tests/${FILE}"
    fi

    # Build full command
    local full_cmd="npx playwright test ${playwright_args}"
    if [[ -n "${test_target}" ]]; then
        full_cmd="${full_cmd} ${test_target}"
    fi

    # Add slowMo via environment (Playwright config reads this)
    export PLAYWRIGHT_SLOWMO="${SLOWMO}"

    log_step "EXECUTION" "Running Playwright in debug mode"
    log_info "Slow motion: ${SLOWMO}ms delay between actions"
    log_info "Traces will be captured for all tests"
    echo ""
    log_command "${full_cmd}"
    echo ""

    # Create a temporary config that includes slowMo
    local temp_config="${PROJECT_ROOT}/.playwright-debug-config.js"
    cat > "${temp_config}" << EOF
// Temporary debug config - auto-generated
import baseConfig from './playwright.config.js';

export default {
    ...baseConfig,
    use: {
        ...baseConfig.use,
        launchOptions: {
            slowMo: ${SLOWMO},
        },
        trace: 'on',
    },
    workers: 1,
    retries: 0,
};
EOF

    # Run tests with temporary config
    local exit_code=0
    # shellcheck disable=SC2086
    if npx playwright test --config="${temp_config}" --headed --project="${PROJECT}" ${GREP:+--grep="${GREP}"} ${test_target}; then
        log_success "Debug tests completed successfully"
    else
        exit_code=$?
        log_warning "Debug tests completed with failures (exit code: ${exit_code})"
    fi

    # Clean up temporary config
    rm -f "${temp_config}"

    # Output helpful information
    log_step "ARTIFACTS" "Test artifacts"
    log_info "HTML Report: ${PROJECT_ROOT}/playwright-report/index.html"
    log_info "Test Results: ${PROJECT_ROOT}/test-results/"

    # Show trace info if tests ran
    if [[ -d "${PROJECT_ROOT}/test-results" ]] && find "${PROJECT_ROOT}/test-results" -name "trace.zip" -type f 2>/dev/null | head -1 | grep -q .; then
        log_info ""
        log_info "View traces with:"
        log_info "  npx playwright show-trace test-results/<test-name>/trace.zip"
    fi

    exit "${exit_code}"
}

# Run main with all arguments
main "$@"
