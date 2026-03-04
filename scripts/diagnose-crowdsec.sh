#!/usr/bin/env bash
# diagnose-crowdsec.sh - CrowdSec Connectivity and Enrollment Diagnostics
# Usage: ./diagnose-crowdsec.sh [--json] [--data-dir /path/to/crowdsec]
# shellcheck disable=SC2312

set -euo pipefail

# Default configuration
DATA_DIR="${CROWDSEC_DATA_DIR:-/var/lib/crowdsec}"
JSON_OUTPUT=false
LAPI_PORT="${CROWDSEC_LAPI_PORT:-8085}"

# Colors for terminal output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Parse arguments
while [[ $# -gt 0 ]]; do
    case "$1" in
        --json)
            JSON_OUTPUT=true
            shift
            ;;
        --data-dir)
            DATA_DIR="$2"
            shift 2
            ;;
        --lapi-port)
            LAPI_PORT="$2"
            shift 2
            ;;
        -h|--help)
            echo "Usage: $0 [--json] [--data-dir /path/to/crowdsec] [--lapi-port 8085]"
            echo ""
            echo "Options:"
            echo "  --json         Output results as JSON"
            echo "  --data-dir     CrowdSec data directory (default: /var/lib/crowdsec)"
            echo "  --lapi-port    LAPI port (default: 8085)"
            echo "  -h, --help     Show this help message"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

# Results storage
declare -A RESULTS

# Logging functions
log_info() {
    if [[ "$JSON_OUTPUT" == "false" ]]; then
        echo -e "${BLUE}[INFO]${NC} $1"
    fi
}

log_success() {
    if [[ "$JSON_OUTPUT" == "false" ]]; then
        echo -e "${GREEN}[PASS]${NC} $1"
    fi
}

log_warning() {
    if [[ "$JSON_OUTPUT" == "false" ]]; then
        echo -e "${YELLOW}[WARN]${NC} $1"
    fi
}

log_error() {
    if [[ "$JSON_OUTPUT" == "false" ]]; then
        echo -e "${RED}[FAIL]${NC} $1"
    fi
}

# Check if command exists
check_command() {
    command -v "$1" &>/dev/null
}

# 1. Check LAPI process running
check_lapi_running() {
    log_info "Checking if CrowdSec LAPI is running..."

    if pgrep -x "crowdsec" &>/dev/null; then
        local pid
        pid=$(pgrep -x "crowdsec" | head -1)
        RESULTS["lapi_running"]="true"
        RESULTS["lapi_pid"]="$pid"
        log_success "CrowdSec LAPI is running (PID: $pid)"
        return 0
    else
        RESULTS["lapi_running"]="false"
        RESULTS["lapi_pid"]=""
        log_error "CrowdSec LAPI is NOT running"
        return 1
    fi
}

# 2. Check LAPI responding
check_lapi_health() {
    log_info "Checking LAPI health endpoint..."

    local health_url="http://127.0.0.1:${LAPI_PORT}/health"
    local response

    if response=$(curl -s --connect-timeout 5 --max-time 10 "$health_url" 2>/dev/null); then
        RESULTS["lapi_healthy"]="true"
        RESULTS["lapi_health_response"]="$response"
        log_success "LAPI health endpoint responding at $health_url"
        return 0
    else
        RESULTS["lapi_healthy"]="false"
        RESULTS["lapi_health_response"]=""
        log_error "LAPI health endpoint not responding at $health_url"
        return 1
    fi
}

# 3. Check cscli available
check_cscli() {
    log_info "Checking cscli availability..."

    if check_command cscli; then
        local version
        version=$(cscli version 2>/dev/null | head -1 || echo "unknown")
        RESULTS["cscli_available"]="true"
        RESULTS["cscli_version"]="$version"
        log_success "cscli is available: $version"
        return 0
    else
        RESULTS["cscli_available"]="false"
        RESULTS["cscli_version"]=""
        log_error "cscli command not found"
        return 1
    fi
}

# 4. Check CAPI registration
check_capi_registered() {
    log_info "Checking CAPI registration..."

    local creds_path="${DATA_DIR}/config/online_api_credentials.yaml"
    if [[ ! -f "$creds_path" ]]; then
        creds_path="${DATA_DIR}/online_api_credentials.yaml"
    fi

    if [[ -f "$creds_path" ]]; then
        RESULTS["capi_registered"]="true"
        RESULTS["capi_creds_path"]="$creds_path"
        log_success "CAPI credentials found at $creds_path"
        return 0
    else
        RESULTS["capi_registered"]="false"
        RESULTS["capi_creds_path"]=""
        log_error "CAPI credentials not found (checked ${DATA_DIR}/config/online_api_credentials.yaml)"
        return 1
    fi
}

# 5. Check CAPI connectivity
check_capi_connectivity() {
    log_info "Checking CAPI connectivity..."

    if ! check_command cscli; then
        RESULTS["capi_reachable"]="unknown"
        log_warning "Cannot check CAPI connectivity - cscli not available"
        return 1
    fi

    local config_path="${DATA_DIR}/config/config.yaml"
    if [[ ! -f "$config_path" ]]; then
        config_path="${DATA_DIR}/config.yaml"
    fi

    local cscli_args=("capi" "status")
    if [[ -f "$config_path" ]]; then
        cscli_args=("-c" "$config_path" "capi" "status")
    fi

    local output
    if output=$(timeout 10s cscli "${cscli_args[@]}" 2>&1); then
        RESULTS["capi_reachable"]="true"
        RESULTS["capi_status"]="$output"
        log_success "CAPI is reachable"
        return 0
    else
        RESULTS["capi_reachable"]="false"
        RESULTS["capi_status"]="$output"
        log_error "CAPI is not reachable: $output"
        return 1
    fi
}

# 6. Check Console API reachability
check_console_api() {
    log_info "Checking CrowdSec Console API reachability..."

    local console_url="https://api.crowdsec.net/health"
    local http_code

    http_code=$(curl -s -o /dev/null -w "%{http_code}" --connect-timeout 5 --max-time 10 "$console_url" 2>/dev/null || echo "000")

    if [[ "$http_code" == "200" ]] || [[ "$http_code" == "204" ]]; then
        RESULTS["console_reachable"]="true"
        RESULTS["console_http_code"]="$http_code"
        log_success "Console API is reachable (HTTP $http_code)"
        return 0
    else
        RESULTS["console_reachable"]="false"
        RESULTS["console_http_code"]="$http_code"
        log_error "Console API is not reachable (HTTP $http_code)"
        return 1
    fi
}

# 7. Check Console enrollment status
check_console_enrolled() {
    log_info "Checking Console enrollment status..."

    if ! check_command cscli; then
        RESULTS["console_enrolled"]="unknown"
        log_warning "Cannot check enrollment - cscli not available"
        return 1
    fi

    local config_path="${DATA_DIR}/config/config.yaml"
    if [[ ! -f "$config_path" ]]; then
        config_path="${DATA_DIR}/config.yaml"
    fi

    local cscli_args=("console" "status")
    if [[ -f "$config_path" ]]; then
        cscli_args=("-c" "$config_path" "console" "status")
    fi

    local output
    if output=$(timeout 10s cscli "${cscli_args[@]}" 2>&1); then
        if echo "$output" | grep -qi "enrolled"; then
            RESULTS["console_enrolled"]="true"
            RESULTS["console_enrollment_output"]="$output"
            log_success "Console is enrolled"
            return 0
        else
            RESULTS["console_enrolled"]="false"
            RESULTS["console_enrollment_output"]="$output"
            log_warning "Console enrollment status unclear: $output"
            return 1
        fi
    else
        RESULTS["console_enrolled"]="false"
        RESULTS["console_enrollment_output"]="$output"
        log_error "Failed to check console status: $output"
        return 1
    fi
}

# 8. Check config.yaml
check_config_yaml() {
    log_info "Checking config.yaml..."

    local config_path="${DATA_DIR}/config/config.yaml"
    if [[ ! -f "$config_path" ]]; then
        config_path="${DATA_DIR}/config.yaml"
    fi

    if [[ -f "$config_path" ]]; then
        RESULTS["config_exists"]="true"
        RESULTS["config_path"]="$config_path"
        log_success "config.yaml found at $config_path"

        # Try to validate
        if check_command cscli; then
            if timeout 10s cscli -c "$config_path" config check &>/dev/null; then
                RESULTS["config_valid"]="true"
                log_success "config.yaml is valid"
            else
                RESULTS["config_valid"]="false"
                log_error "config.yaml validation failed"
            fi
        else
            RESULTS["config_valid"]="unknown"
        fi
        return 0
    else
        RESULTS["config_exists"]="false"
        RESULTS["config_valid"]="false"
        log_error "config.yaml not found"
        return 1
    fi
}

# 9. Check acquis.yaml
check_acquis_yaml() {
    log_info "Checking acquis.yaml..."

    local acquis_path="${DATA_DIR}/config/acquis.yaml"
    if [[ ! -f "$acquis_path" ]]; then
        acquis_path="${DATA_DIR}/acquis.yaml"
    fi

    if [[ -f "$acquis_path" ]]; then
        RESULTS["acquis_exists"]="true"
        RESULTS["acquis_path"]="$acquis_path"
        log_success "acquis.yaml found at $acquis_path"

        # Check for datasources
        if grep -q "source:" "$acquis_path" && grep -qE "(filenames?:|journalctl)" "$acquis_path"; then
            RESULTS["acquis_valid"]="true"
            log_success "acquis.yaml has datasource configuration"
        else
            RESULTS["acquis_valid"]="false"
            log_warning "acquis.yaml may be missing datasource configuration"
        fi
        return 0
    else
        RESULTS["acquis_exists"]="false"
        RESULTS["acquis_valid"]="false"
        log_warning "acquis.yaml not found (optional for some setups)"
        return 1
    fi
}

# 10. Check bouncers registered
check_bouncers() {
    log_info "Checking registered bouncers..."

    if ! check_command cscli; then
        RESULTS["bouncers_count"]="unknown"
        log_warning "Cannot check bouncers - cscli not available"
        return 1
    fi

    local config_path="${DATA_DIR}/config/config.yaml"
    if [[ ! -f "$config_path" ]]; then
        config_path="${DATA_DIR}/config.yaml"
    fi

    local cscli_args=("bouncers" "list" "-o" "json")
    if [[ -f "$config_path" ]]; then
        cscli_args=("-c" "$config_path" "bouncers" "list" "-o" "json")
    fi

    local output
    if output=$(timeout 10s cscli "${cscli_args[@]}" 2>/dev/null); then
        local count
        count=$(echo "$output" | jq 'length' 2>/dev/null || echo "0")
        RESULTS["bouncers_count"]="$count"
        RESULTS["bouncers_list"]="$output"
        if [[ "$count" -gt 0 ]]; then
            log_success "Found $count registered bouncer(s)"
        else
            log_warning "No bouncers registered"
        fi
        return 0
    else
        RESULTS["bouncers_count"]="0"
        log_error "Failed to list bouncers"
        return 1
    fi
}

# Output JSON results
output_json() {
    echo "{"
    local first=true
    for key in "${!RESULTS[@]}"; do
        if [[ "$first" == "true" ]]; then
            first=false
        else
            echo ","
        fi
        local value="${RESULTS[$key]}"
        # Escape special characters for JSON
        value="${value//\\/\\\\}"
        value="${value//\"/\\\"}"
        value="${value//$'\n'/\\n}"
        value="${value//$'\r'/\\r}"
        value="${value//$'\t'/\\t}"
        printf '  "%s": "%s"' "$key" "$value"
    done
    echo ""
    echo "}"
}

# Print summary
print_summary() {
    echo ""
    echo "=========================================="
    echo "         CrowdSec Diagnostic Summary"
    echo "=========================================="
    echo ""

    local passed=0
    local failed=0
    local warnings=0

    for key in lapi_running lapi_healthy capi_registered capi_reachable console_reachable console_enrolled config_exists config_valid; do
        case "${RESULTS[$key]:-unknown}" in
            true) ((passed++)) ;;
            false) ((failed++)) ;;
            *) ((warnings++)) ;;
        esac
    done

    echo -e "Checks passed:  ${GREEN}$passed${NC}"
    echo -e "Checks failed:  ${RED}$failed${NC}"
    echo -e "Checks unknown: ${YELLOW}$warnings${NC}"
    echo ""

    if [[ "$failed" -gt 0 ]]; then
        echo -e "${RED}Some checks failed. See details above.${NC}"
        echo ""
        echo "Common solutions:"
        echo "  - If LAPI not running: systemctl start crowdsec"
        echo "  - If CAPI not registered: cscli capi register"
        echo "  - If Console not enrolled: cscli console enroll <token>"
        echo "  - If config missing: Check ${DATA_DIR}/config/"
        exit 1
    else
        echo -e "${GREEN}All critical checks passed!${NC}"
        exit 0
    fi
}

# Main execution
main() {
    if [[ "$JSON_OUTPUT" == "false" ]]; then
        echo "=========================================="
        echo "     CrowdSec Diagnostic Tool v1.0"
        echo "=========================================="
        echo ""
        echo "Data directory: ${DATA_DIR}"
        echo "LAPI port: ${LAPI_PORT}"
        echo ""
    fi

    # Run all checks (continue on failure)
    check_lapi_running || true
    check_lapi_health || true
    check_cscli || true
    check_capi_registered || true
    check_capi_connectivity || true
    check_console_api || true
    check_console_enrolled || true
    check_config_yaml || true
    check_acquis_yaml || true
    check_bouncers || true

    if [[ "$JSON_OUTPUT" == "true" ]]; then
        output_json
    else
        print_summary
    fi
}

main
