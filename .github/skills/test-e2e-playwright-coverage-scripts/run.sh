#!/usr/bin/env bash
# Test E2E Playwright Coverage - Execution Script
#
# Runs Playwright end-to-end tests with code coverage collection
# using @bgotink/playwright-coverage.
#
# IMPORTANT: For accurate source-level coverage, this script starts
# the Vite dev server (localhost:5173) which proxies API calls to
# the Docker backend (localhost:8080). V8 coverage requires source
# files to be accessible on the test host.

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
PROJECT="firefox"
VITE_PID=""
VITE_PORT="${VITE_PORT:-5173}"  # Default Vite port (avoids conflicts with common ports)
BACKEND_URL="http://localhost:8080"

# Cleanup function to kill Vite dev server on exit
cleanup() {
    if [[ -n "${VITE_PID}" ]] && kill -0 "${VITE_PID}" 2>/dev/null; then
        log_info "Stopping Vite dev server (PID: ${VITE_PID})..."
        kill "${VITE_PID}" 2>/dev/null || true
        wait "${VITE_PID}" 2>/dev/null || true
    fi
}

# Set up trap for cleanup
trap cleanup EXIT INT TERM

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
            --skip-vite)
                SKIP_VITE="true"
                shift
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

Run Playwright E2E tests with coverage collection.

Coverage requires the Vite dev server to serve source files directly.
This script automatically starts Vite at localhost:5173, which proxies
API calls to the Docker backend at localhost:8080.

Options:
    --project=PROJECT   Browser project to run (chromium, firefox, webkit)
                        Default: firefox
    --skip-vite         Skip starting Vite dev server (use existing server)
    -h, --help          Show this help message

Environment Variables:
    PLAYWRIGHT_BASE_URL     Override test URL (default: http://localhost:5173)
    VITE_PORT               Vite dev server port (default: 5173)
    CI                      Set to 'true' for CI environment

Prerequisites:
    - Docker backend running at localhost:8080
    - Node.js dependencies installed (npm ci)

Examples:
    run.sh                          # Start Vite, run tests with coverage
    run.sh --project=firefox        # Run in Firefox with coverage
    run.sh --skip-vite              # Use existing Vite server
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

# Check if backend is running
check_backend() {
    log_info "Checking backend at ${BACKEND_URL}..."
    local max_attempts=5
    local attempt=1

    while [[ ${attempt} -le ${max_attempts} ]]; do
        if curl -sf "${BACKEND_URL}/api/v1/health" >/dev/null 2>&1; then
            log_success "Backend is healthy"
            return 0
        fi
        log_info "Waiting for backend... (attempt ${attempt}/${max_attempts})"
        sleep 2
        ((attempt++))
    done

    log_warning "Backend not responding at ${BACKEND_URL}"
    log_warning "Coverage tests require Docker backend. Start with:"
    log_warning "  docker compose -f .docker/compose/docker-compose.local.yml up -d"
    return 1
}

# Start Vite dev server
start_vite() {
    local vite_url="http://localhost:${VITE_PORT}"

    # Check if Vite is already running on our preferred port
    if curl -sf "${vite_url}" >/dev/null 2>&1; then
        log_info "Vite dev server already running at ${vite_url}"
        return 0
    fi

    log_step "VITE" "Starting Vite dev server"
    cd "${PROJECT_ROOT}/frontend"

    # Ensure dependencies are installed
    if [[ ! -d "node_modules" ]]; then
        log_info "Installing frontend dependencies..."
        npm ci --silent
    fi

    # Start Vite in background with explicit port
    log_command "npx vite --port ${VITE_PORT} (background)"
    npx vite --port "${VITE_PORT}" > /tmp/vite.log 2>&1 &
    VITE_PID=$!

    # Wait for Vite to be ready (check log for actual port in case of conflict)
    log_info "Waiting for Vite to start..."
    local max_wait=60
    local waited=0
    local actual_port="${VITE_PORT}"

    while [[ ${waited} -lt ${max_wait} ]]; do
        # Check if Vite logged its ready message with actual port
        if grep -q "Local:" /tmp/vite.log 2>/dev/null; then
            # Extract actual port from Vite log (handles port conflict auto-switch)
            actual_port=$(grep -oP 'localhost:\K[0-9]+' /tmp/vite.log 2>/dev/null | head -1 || echo "${VITE_PORT}")
            vite_url="http://localhost:${actual_port}"
        fi

        if curl -sf "${vite_url}" >/dev/null 2>&1; then
            # Update VITE_PORT if Vite chose a different port
            if [[ "${actual_port}" != "${VITE_PORT}" ]]; then
                log_warning "Port ${VITE_PORT} was busy, Vite using port ${actual_port}"
                VITE_PORT="${actual_port}"
            fi
            log_success "Vite dev server ready at ${vite_url}"
            cd "${PROJECT_ROOT}"
            return 0
        fi
        sleep 1
        ((waited++))
    done

    log_error "Vite failed to start within ${max_wait} seconds"
    log_error "Vite log:"
    cat /tmp/vite.log 2>/dev/null || true
    cd "${PROJECT_ROOT}"
    return 1
}

# Main execution
main() {
    SKIP_VITE="${SKIP_VITE:-false}"
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

    # Check backend is running (required for API proxy)
    log_step "BACKEND" "Checking Docker backend"
    if ! check_backend; then
        error_exit "Backend not available. Coverage tests require Docker backend at ${BACKEND_URL}"
    fi

    # Start Vite dev server for coverage (unless skipped)
    if [[ "${SKIP_VITE}" != "true" ]]; then
        start_vite || error_exit "Failed to start Vite dev server"
    fi

    # Ensure coverage directory exists
    log_step "SETUP" "Creating coverage directory"
    mkdir -p coverage/e2e

    # Set environment variables
    # IMPORTANT: Use Vite URL (3000) for coverage, not Docker (8080)
    export PLAYWRIGHT_HTML_OPEN="${PLAYWRIGHT_HTML_OPEN:-never}"
    export PLAYWRIGHT_BASE_URL="${PLAYWRIGHT_BASE_URL:-http://localhost:${VITE_PORT}}"

    # Log configuration
    log_step "CONFIG" "Test configuration"
    log_info "Project: ${PROJECT}"
    log_info "Test URL: ${PLAYWRIGHT_BASE_URL}"
    log_info "Backend URL: ${BACKEND_URL}"
    log_info "Coverage output: ${PROJECT_ROOT}/coverage/e2e/"
    log_info ""
    log_info "Coverage architecture:"
    log_info "  Tests → Vite (localhost:${VITE_PORT}) → serves source files"
    log_info "  Vite → Docker (localhost:8080) → API proxy"

    # Execute Playwright tests with coverage
    log_step "EXECUTION" "Running Playwright E2E tests with coverage"
    log_command "npx playwright test --project=${PROJECT}"

    local exit_code=0
    if npx playwright test --project="${PROJECT}"; then
        log_success "All E2E tests passed"
    else
        exit_code=$?
        log_error "E2E tests failed (exit code: ${exit_code})"
    fi

    # Check if coverage was generated
    log_step "COVERAGE" "Checking coverage output"
    if [[ -f "coverage/e2e/lcov.info" ]]; then
        log_success "E2E coverage generated: coverage/e2e/lcov.info"

        # Print summary if coverage.json exists
        if [[ -f "coverage/e2e/coverage.json" ]] && command -v jq &> /dev/null; then
            log_info "📊 Coverage Summary:"
            jq '.total' coverage/e2e/coverage.json 2>/dev/null || true
        fi

        # Show file sizes
        log_info "Coverage files:"
        ls -lh coverage/e2e/ 2>/dev/null || true
    else
        log_warning "No coverage data generated"
        log_warning "Ensure test files import from '@bgotink/playwright-coverage'"
    fi

    # Output report locations
    log_step "REPORTS" "Report locations"
    log_info "Coverage HTML: ${PROJECT_ROOT}/coverage/e2e/index.html"
    log_info "Coverage LCOV: ${PROJECT_ROOT}/coverage/e2e/lcov.info"
    log_info "Playwright Report: ${PROJECT_ROOT}/playwright-report/index.html"

    exit "${exit_code}"
}

# Run main with all arguments
main "$@"
