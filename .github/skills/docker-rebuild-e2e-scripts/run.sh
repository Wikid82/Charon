#!/usr/bin/env bash
# Docker: Rebuild E2E Environment - Execution Script
#
# Rebuilds the Docker image and restarts the Playwright E2E testing
# environment with fresh code and optionally clean state.

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

# Docker compose file for Playwright E2E tests
COMPOSE_FILE=".docker/compose/docker-compose.playwright-local.yml"
CONTAINER_NAME="charon-e2e"
IMAGE_NAME="charon:local"
HEALTH_TIMEOUT=60
HEALTH_INTERVAL=5

# Default parameter values
NO_CACHE=false
CLEAN=false
PROFILE=""

# Parse command-line arguments
parse_arguments() {
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --no-cache)
                NO_CACHE=true
                shift
                ;;
            --clean)
                CLEAN=true
                shift
                ;;
            --profile=*)
                PROFILE="${1#*=}"
                shift
                ;;
            --profile)
                PROFILE="${2:-}"
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

Rebuild Docker image and restart E2E Playwright container.

Options:
    --no-cache          Force rebuild without Docker cache
    --clean             Remove test volumes for fresh state
    --profile=PROFILE   Docker Compose profile to enable
                        (security-tests, notification-tests)
    -h, --help          Show this help message

Environment Variables:
    DOCKER_NO_CACHE         Force rebuild without cache (default: false)
    SKIP_VOLUME_CLEANUP     Preserve test data volumes (default: false)

Examples:
    run.sh                              # Standard rebuild
    run.sh --no-cache                   # Force complete rebuild
    run.sh --clean                      # Rebuild with fresh volumes
    run.sh --profile=security-tests     # Enable CrowdSec for testing
    run.sh --no-cache --clean           # Complete fresh rebuild
EOF
}

# Stop existing containers
stop_containers() {
    log_step "STOP" "Stopping existing E2E containers"

    local compose_cmd="docker compose -f ${COMPOSE_FILE}"

    # Add profile if specified
    if [[ -n "${PROFILE}" ]]; then
        compose_cmd="${compose_cmd} --profile ${PROFILE}"
    fi

    # Stop and remove containers
    if ${compose_cmd} ps -q 2>/dev/null | grep -q .; then
        log_info "Stopping containers..."
        ${compose_cmd} down --remove-orphans || true
    else
        log_info "No running containers to stop"
    fi
}

# Clean volumes if requested
clean_volumes() {
    if [[ "${CLEAN}" != "true" ]]; then
        return 0
    fi

    if [[ "${SKIP_VOLUME_CLEANUP:-false}" == "true" ]]; then
        log_warning "Skipping volume cleanup (SKIP_VOLUME_CLEANUP=true)"
        return 0
    fi

    log_step "CLEAN" "Removing test volumes"

    local volumes=(
        "playwright_data"
        "playwright_caddy_data"
        "playwright_caddy_config"
        "playwright_crowdsec_data"
        "playwright_crowdsec_config"
    )

    for vol in "${volumes[@]}"; do
        # Try both prefixed and unprefixed volume names
        for prefix in "compose_" ""; do
            local full_name="${prefix}${vol}"
            if docker volume inspect "${full_name}" &>/dev/null; then
                log_info "Removing volume: ${full_name}"
                docker volume rm "${full_name}" || true
            fi
        done
    done

    log_success "Volumes cleaned"
}

# Build Docker image
build_image() {
    log_step "BUILD" "Building Docker image: ${IMAGE_NAME}"

    local build_args=("-t" "${IMAGE_NAME}" ".")

    if [[ "${NO_CACHE}" == "true" ]] || [[ "${DOCKER_NO_CACHE:-false}" == "true" ]]; then
        log_info "Building with --no-cache"
        build_args=("--no-cache" "${build_args[@]}")
    fi

    log_command "docker build ${build_args[*]}"

    if ! docker build "${build_args[@]}"; then
        error_exit "Docker build failed"
    fi

    log_success "Image built successfully: ${IMAGE_NAME}"
}

# Start containers
start_containers() {
    log_step "START" "Starting E2E containers"

    local compose_cmd="docker compose -f ${COMPOSE_FILE}"

    # Add profile if specified
    if [[ -n "${PROFILE}" ]]; then
        log_info "Enabling profile: ${PROFILE}"
        compose_cmd="${compose_cmd} --profile ${PROFILE}"
    fi

    log_command "${compose_cmd} up -d"

    if ! ${compose_cmd} up -d; then
        error_exit "Failed to start containers"
    fi

    log_success "Containers started"
}

# Wait for container health
wait_for_health() {
    log_step "HEALTH" "Waiting for container to be healthy"

    local elapsed=0
    local healthy=false

    while [[ ${elapsed} -lt ${HEALTH_TIMEOUT} ]]; do
        local health_status
        health_status=$(docker inspect --format='{{.State.Health.Status}}' "${CONTAINER_NAME}" 2>/dev/null || echo "unknown")

        case "${health_status}" in
            healthy)
                healthy=true
                break
                ;;
            unhealthy)
                log_error "Container is unhealthy"
                docker logs "${CONTAINER_NAME}" --tail 20
                error_exit "Container health check failed"
                ;;
            starting)
                log_info "Health status: starting (${elapsed}s/${HEALTH_TIMEOUT}s)"
                ;;
            *)
                log_info "Health status: ${health_status} (${elapsed}s/${HEALTH_TIMEOUT}s)"
                ;;
        esac

        sleep "${HEALTH_INTERVAL}"
        elapsed=$((elapsed + HEALTH_INTERVAL))
    done

    if [[ "${healthy}" != "true" ]]; then
        log_error "Container did not become healthy in ${HEALTH_TIMEOUT}s"
        docker logs "${CONTAINER_NAME}" --tail 50
        error_exit "Health check timeout"
    fi

    log_success "Container is healthy"
}

# Verify environment
verify_environment() {
    log_step "VERIFY" "Verifying E2E environment"

    # Check container is running
    if ! docker ps --filter "name=${CONTAINER_NAME}" --format "{{.Names}}" | grep -q "${CONTAINER_NAME}"; then
        error_exit "Container ${CONTAINER_NAME} is not running"
    fi

    # Test health endpoint
    log_info "Testing health endpoint..."
    if curl -sf http://localhost:8080/api/v1/health &>/dev/null; then
        log_success "Health endpoint responding"
    else
        log_warning "Health endpoint not responding (may need more time)"
    fi

    # Show container status
    log_info "Container status:"
    docker ps --filter "name=${CONTAINER_NAME}" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}"
}

# Show summary
show_summary() {
    log_step "SUMMARY" "E2E environment ready"

    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo "  E2E Environment Ready"
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
    echo ""
    echo "  Application URL:  http://localhost:8080"
    echo "  Health Check:     http://localhost:8080/api/v1/health"
    echo "  Container:        ${CONTAINER_NAME}"
    echo ""
    echo "  Run E2E tests:"
    echo "    .github/skills/scripts/skill-runner.sh test-e2e-playwright"
    echo ""
    echo "  Run in debug mode:"
    echo "    .github/skills/scripts/skill-runner.sh test-e2e-playwright-debug"
    echo ""
    echo "  View logs:"
    echo "    docker logs ${CONTAINER_NAME} -f"
    echo ""
    echo "━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━"
}

# Main execution
main() {
    parse_arguments "$@"

    # Validate environment
    log_step "ENVIRONMENT" "Validating prerequisites"
    validate_docker_environment || error_exit "Docker is not available"
    check_command_exists "docker" "Docker is required"

    # Validate project structure
    log_step "VALIDATION" "Checking project structure"
    cd "${PROJECT_ROOT}"
    check_file_exists "Dockerfile" "Dockerfile is required"
    check_file_exists "${COMPOSE_FILE}" "Playwright compose file is required"

    # Log configuration
    log_step "CONFIG" "Rebuild configuration"
    log_info "No cache: ${NO_CACHE}"
    log_info "Clean volumes: ${CLEAN}"
    log_info "Profile: ${PROFILE:-<none>}"
    log_info "Compose file: ${COMPOSE_FILE}"

    # Execute rebuild steps
    stop_containers
    clean_volumes
    build_image
    start_containers
    wait_for_health
    verify_environment
    show_summary

    log_success "E2E environment rebuild complete"
}

# Run main with all arguments
main "$@"
