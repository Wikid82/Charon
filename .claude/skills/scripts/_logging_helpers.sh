#!/usr/bin/env bash
# Agent Skills - Logging Helpers
#
# Provides colored logging functions for consistent output across all skills.

# Color codes
readonly COLOR_RESET="\033[0m"
readonly COLOR_RED="\033[0;31m"
readonly COLOR_GREEN="\033[0;32m"
readonly COLOR_YELLOW="\033[0;33m"
readonly COLOR_BLUE="\033[0;34m"
readonly COLOR_MAGENTA="\033[0;35m"
readonly COLOR_CYAN="\033[0;36m"
readonly COLOR_GRAY="\033[0;90m"

# Check if output is a terminal (for color support)
if [[ -t 1 ]]; then
    COLORS_ENABLED=true
else
    COLORS_ENABLED=false
fi

# Disable colors if NO_COLOR environment variable is set
if [[ -n "${NO_COLOR:-}" ]]; then
    COLORS_ENABLED=false
fi

# log_info: Print informational message
log_info() {
    local message="$*"
    if [[ "${COLORS_ENABLED}" == "true" ]]; then
        echo -e "${COLOR_BLUE}[INFO]${COLOR_RESET} ${message}"
    else
        echo "[INFO] ${message}"
    fi
}

# log_success: Print success message
log_success() {
    local message="$*"
    if [[ "${COLORS_ENABLED}" == "true" ]]; then
        echo -e "${COLOR_GREEN}[SUCCESS]${COLOR_RESET} ${message}"
    else
        echo "[SUCCESS] ${message}"
    fi
}

# log_warning: Print warning message
log_warning() {
    local message="$*"
    if [[ "${COLORS_ENABLED}" == "true" ]]; then
        echo -e "${COLOR_YELLOW}[WARNING]${COLOR_RESET} ${message}" >&2
    else
        echo "[WARNING] ${message}" >&2
    fi
}

# log_error: Print error message
log_error() {
    local message="$*"
    if [[ "${COLORS_ENABLED}" == "true" ]]; then
        echo -e "${COLOR_RED}[ERROR]${COLOR_RESET} ${message}" >&2
    else
        echo "[ERROR] ${message}" >&2
    fi
}

# log_debug: Print debug message (only if DEBUG=1)
log_debug() {
    if [[ "${DEBUG:-0}" == "1" ]]; then
        local message="$*"
        if [[ "${COLORS_ENABLED}" == "true" ]]; then
            echo -e "${COLOR_GRAY}[DEBUG]${COLOR_RESET} ${message}"
        else
            echo "[DEBUG] ${message}"
        fi
    fi
}

# log_step: Print step header
log_step() {
    local step_name="$1"
    shift
    local message="$*"
    if [[ "${COLORS_ENABLED}" == "true" ]]; then
        echo -e "${COLOR_CYAN}[${step_name}]${COLOR_RESET} ${message}"
    else
        echo "[${step_name}] ${message}"
    fi
}

# log_command: Log a command before executing (for transparency)
log_command() {
    local command="$*"
    if [[ "${COLORS_ENABLED}" == "true" ]]; then
        echo -e "${COLOR_MAGENTA}[$]${COLOR_RESET} ${command}"
    else
        echo "[\$] ${command}"
    fi
}

# Export functions so they can be used by sourcing scripts
export -f log_info
export -f log_success
export -f log_warning
export -f log_error
export -f log_debug
export -f log_step
export -f log_command
