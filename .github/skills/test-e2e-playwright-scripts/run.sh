#!/usr/bin/env bash
# Test E2E Playwright - Execution Script
#
# Runs Playwright end-to-end tests with browser selection,
# headed mode, and test filtering support.

set -euo pipefail

# Source helper scripts
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# Helper scripts are in .github/skills/scripts/ (one level up from skill-scripts dir)
SKILLS_SCRIPTS_DIR="$(cd "${SCRIPT_DIR}/../scripts" && pwd)"

# shellcheck source=../scripts/_logging_helpers.sh
source "${SKILLS_SCRIPTS_DIR}/_logging_helpers.sh"
# shellcheck source=../scripts/_error_handling_helpers.sh
source "${SKILLS_SCRIPTS_DIR}/_error_handling_helpers.sh"
# shellcheck source=../scripts/_environment_helpers.sh
source "${SKILLS_SCRIPTS_DIR}/_environment_helpers.sh"

# Project root is 3 levels up from this script (skills/skill-name-scripts/run.sh -> project root)
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/../../.." && pwd)"

# Default parameter values
PROJECT="firefox"
HEADED=false
GREP=""

# Parse command-line arguments
parse_arguments() {
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --project=*)
                PROJECT="${1#*=}"
                shift
                ;;
            --project)
                PROJECT="${2:-firefox}"
                shift 2
                ;;
            --headed)
                HEADED=true
                shift
                ;;
            --grep=*)
                GREP="${1#*=}"
                shift
                ;;
            --grep)
                GREP="${2:-}"
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

Run Playwright E2E tests against the Charon application.

Options:
    --project=PROJECT   Browser project to run (chromium, firefox, webkit, all)
                        Default: firefox
    --headed            Run tests in headed mode (visible browser)
    --grep=PATTERN      Filter tests by title pattern (regex)
    -h, --help          Show this help message

Environment Variables:
    PLAYWRIGHT_BASE_URL     Application URL to test (default: http://localhost:8080)
    PLAYWRIGHT_HTML_OPEN    HTML report behavior (default: never)
    CI                      Set to 'true' for CI environment

Examples:
    run.sh                              # Run all tests in Firefox (headless)
    run.sh --project=chromium           # Run in Chromium
    run.sh --headed                     # Run with visible browser
    run.sh --grep="login"               # Run only login tests
    run.sh --project=all --grep="smoke" # All browsers, smoke tests only
EOF
}

# Validate project parameter
validate_project() {
    local valid_projects=("chromium" "firefox" "webkit" "all")
    local project_lower
    project_lower=$(echo "${PROJECT}" | tr '[:upper:]' '[:lower:]')

    for valid in "${valid_projects[@]}"; do
        if [[ "${project_lower}" == "${valid}" ]]; then
            PROJECT="${project_lower}"
            return 0
        fi
    done

    error_exit "Invalid project '${PROJECT}'. Valid options: chromium, firefox, webkit, all"
}

# Build Playwright command arguments
build_playwright_args() {
    local args=()

    # Add project selection
    if [[ "${PROJECT}" != "all" ]]; then
        args+=("--project=${PROJECT}")
    fi

    # Add headed mode if requested
    if [[ "${HEADED}" == "true" ]]; then
        args+=("--headed")
    fi

    # Add grep filter if specified
    if [[ -n "${GREP}" ]]; then
        args+=("--grep=${GREP}")
    fi

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

    # Validate project parameter
    validate_project

    # Set environment variables for non-interactive execution
    export PLAYWRIGHT_HTML_OPEN="${PLAYWRIGHT_HTML_OPEN:-never}"
    export PLAYWRIGHT_SKIP_SECURITY_DEPS="${PLAYWRIGHT_SKIP_SECURITY_DEPS:-1}"
    # Ensure non-coverage runs do NOT start the Vite dev server (use Docker in CI/local non-coverage)
    export PLAYWRIGHT_COVERAGE="${PLAYWRIGHT_COVERAGE:-0}"
    set_default_env "PLAYWRIGHT_BASE_URL" "http://127.0.0.1:8080"

    # Log configuration
    log_step "CONFIG" "Test configuration"
    log_info "Project: ${PROJECT}"
    log_info "Headed mode: ${HEADED}"
    log_info "Grep filter: ${GREP:-<none>}"
    log_info "Base URL: ${PLAYWRIGHT_BASE_URL}"
    log_info "HTML report auto-open: ${PLAYWRIGHT_HTML_OPEN}"

    # Build command arguments
    local playwright_args
    playwright_args=$(build_playwright_args)

    # Execute Playwright tests
    log_step "EXECUTION" "Running Playwright E2E tests"
    log_command "npx playwright test ${playwright_args}"

    # Run tests with proper error handling
    local exit_code=0
    # shellcheck disable=SC2086
    if npx playwright test ${playwright_args}; then
        log_success "All E2E tests passed"
    else
        exit_code=$?
        log_error "E2E tests failed (exit code: ${exit_code})"
    fi

    # Output report location
    log_step "REPORT" "Test report available"
    log_info "HTML Report: ${PROJECT_ROOT}/playwright-report/index.html"
    log_info "To view in browser: npx playwright show-report --port 9323"
    log_info "VS Code Simple Browser URL: http://127.0.0.1:9323"

    exit "${exit_code}"
}

# Run main with all arguments
main "$@"
