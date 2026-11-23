#!/usr/bin/env bash
# Agent Skills - Error Handling Helpers
#
# Provides error handling utilities for robust skill execution.

# error_exit: Print error message and exit with code
error_exit() {
    local message="$1"
    local exit_code="${2:-1}"

    # Source logging helpers if not already loaded
    if ! declare -f log_error >/dev/null 2>&1; then
        echo "[ERROR] ${message}" >&2
    else
        log_error "${message}"
    fi

    exit "${exit_code}"
}

# check_command_exists: Verify a command is available
check_command_exists() {
    local cmd="$1"
    local error_msg="${2:-Command not found: ${cmd}}"

    if ! command -v "${cmd}" >/dev/null 2>&1; then
        error_exit "${error_msg}" 127
    fi
}

# check_file_exists: Verify a file exists
check_file_exists() {
    local file="$1"
    local error_msg="${2:-File not found: ${file}}"

    if [[ ! -f "${file}" ]]; then
        error_exit "${error_msg}" 1
    fi
}

# check_dir_exists: Verify a directory exists
check_dir_exists() {
    local dir="$1"
    local error_msg="${2:-Directory not found: ${dir}}"

    if [[ ! -d "${dir}" ]]; then
        error_exit "${error_msg}" 1
    fi
}

# check_exit_code: Verify previous command succeeded
check_exit_code() {
    local exit_code=$?
    local error_msg="${1:-Command failed with exit code ${exit_code}}"

    if [[ ${exit_code} -ne 0 ]]; then
        error_exit "${error_msg}" "${exit_code}"
    fi
}

# run_with_retry: Run a command with retry logic
run_with_retry() {
    local max_attempts="${1}"
    local delay="${2}"
    shift 2
    local cmd=("$@")

    local attempt=1
    while [[ ${attempt} -le ${max_attempts} ]]; do
        if "${cmd[@]}"; then
            return 0
        fi

        if [[ ${attempt} -lt ${max_attempts} ]]; then
            if declare -f log_warning >/dev/null 2>&1; then
                log_warning "Command failed (attempt ${attempt}/${max_attempts}). Retrying in ${delay}s..."
            else
                echo "[WARNING] Command failed (attempt ${attempt}/${max_attempts}). Retrying in ${delay}s..." >&2
            fi
            sleep "${delay}"
        fi

        ((attempt++))
    done

    if declare -f log_error >/dev/null 2>&1; then
        log_error "Command failed after ${max_attempts} attempts: ${cmd[*]}"
    else
        echo "[ERROR] Command failed after ${max_attempts} attempts: ${cmd[*]}" >&2
    fi
    return 1
}

# trap_error: Set up error trapping for the current script
trap_error() {
    local script_name="${1:-${BASH_SOURCE[1]}}"

    trap 'error_handler ${LINENO} ${BASH_LINENO} "${BASH_COMMAND}" "${script_name}"' ERR
}

# error_handler: Internal error handler for trap
error_handler() {
    local line_no="$1"
    local bash_line_no="$2"
    local command="$3"
    local script="$4"

    if declare -f log_error >/dev/null 2>&1; then
        log_error "Script failed at line ${line_no} in ${script}"
        log_error "Command: ${command}"
    else
        echo "[ERROR] Script failed at line ${line_no} in ${script}" >&2
        echo "[ERROR] Command: ${command}" >&2
    fi
}

# cleanup_on_exit: Register a cleanup function to run on exit
cleanup_on_exit() {
    local cleanup_func="$1"

    # Register cleanup function
    trap "${cleanup_func}" EXIT
}

# Export functions
export -f error_exit
export -f check_command_exists
export -f check_file_exists
export -f check_dir_exists
export -f check_exit_code
export -f run_with_retry
export -f trap_error
export -f error_handler
export -f cleanup_on_exit
