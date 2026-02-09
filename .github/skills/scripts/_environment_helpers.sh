#!/usr/bin/env bash
# Agent Skills - Environment Helpers
#
# Provides environment validation and setup utilities.

# validate_go_environment: Check Go installation and version
validate_go_environment() {
    local min_version="${1:-1.23}"

    if ! command -v go >/dev/null 2>&1; then
        if declare -f log_error >/dev/null 2>&1; then
            log_error "Go is not installed or not in PATH"
        else
            echo "[ERROR] Go is not installed or not in PATH" >&2
        fi
        return 1
    fi

    local go_version
    go_version=$(go version | grep -oP 'go\K[0-9]+\.[0-9]+' || echo "0.0")

    if declare -f log_debug >/dev/null 2>&1; then
        log_debug "Go version: ${go_version} (required: >=${min_version})"
    fi

    # Simple version comparison (assumes semantic versioning)
    if [[ "$(printf '%s\n' "${min_version}" "${go_version}" | sort -V | head -n1)" != "${min_version}" ]]; then
        if declare -f log_error >/dev/null 2>&1; then
            log_error "Go version ${go_version} is below minimum required version ${min_version}"
        else
            echo "[ERROR] Go version ${go_version} is below minimum required version ${min_version}" >&2
        fi
        return 1
    fi

    return 0
}

# validate_python_environment: Check Python installation and version
validate_python_environment() {
    local min_version="${1:-3.8}"

    if ! command -v python3 >/dev/null 2>&1; then
        if declare -f log_error >/dev/null 2>&1; then
            log_error "Python 3 is not installed or not in PATH"
        else
            echo "[ERROR] Python 3 is not installed or not in PATH" >&2
        fi
        return 1
    fi

    local python_version
    python_version=$(python3 --version 2>&1 | grep -oP 'Python \K[0-9]+\.[0-9]+' || echo "0.0")

    if declare -f log_debug >/dev/null 2>&1; then
        log_debug "Python version: ${python_version} (required: >=${min_version})"
    fi

    # Simple version comparison
    if [[ "$(printf '%s\n' "${min_version}" "${python_version}" | sort -V | head -n1)" != "${min_version}" ]]; then
        if declare -f log_error >/dev/null 2>&1; then
            log_error "Python version ${python_version} is below minimum required version ${min_version}"
        else
            echo "[ERROR] Python version ${python_version} is below minimum required version ${min_version}" >&2
        fi
        return 1
    fi

    return 0
}

# validate_node_environment: Check Node.js installation and version
validate_node_environment() {
    local min_version="${1:-18.0}"

    if ! command -v node >/dev/null 2>&1; then
        if declare -f log_error >/dev/null 2>&1; then
            log_error "Node.js is not installed or not in PATH"
        else
            echo "[ERROR] Node.js is not installed or not in PATH" >&2
        fi
        return 1
    fi

    local node_version
    node_version=$(node --version | grep -oP 'v\K[0-9]+\.[0-9]+' || echo "0.0")

    if declare -f log_debug >/dev/null 2>&1; then
        log_debug "Node.js version: ${node_version} (required: >=${min_version})"
    fi

    # Simple version comparison
    if [[ "$(printf '%s\n' "${min_version}" "${node_version}" | sort -V | head -n1)" != "${min_version}" ]]; then
        if declare -f log_error >/dev/null 2>&1; then
            log_error "Node.js version ${node_version} is below minimum required version ${min_version}"
        else
            echo "[ERROR] Node.js version ${node_version} is below minimum required version ${min_version}" >&2
        fi
        return 1
    fi

    return 0
}

# validate_docker_environment: Check Docker installation and daemon
validate_docker_environment() {
    if ! command -v docker >/dev/null 2>&1; then
        if declare -f log_error >/dev/null 2>&1; then
            log_error "Docker is not installed or not in PATH"
        else
            echo "[ERROR] Docker is not installed or not in PATH" >&2
        fi
        return 1
    fi

    # Check if Docker daemon is running
    if ! docker info >/dev/null 2>&1; then
        if declare -f log_error >/dev/null 2>&1; then
            log_error "Docker daemon is not running"
        else
            echo "[ERROR] Docker daemon is not running" >&2
        fi
        return 1
    fi

    if declare -f log_debug >/dev/null 2>&1; then
        local docker_version
        docker_version=$(docker --version | grep -oP 'Docker version \K[0-9]+\.[0-9]+\.[0-9]+' || echo "unknown")
        log_debug "Docker version: ${docker_version}"
    fi

    return 0
}

# set_default_env: Set environment variable with default value if not set
set_default_env() {
    local var_name="$1"
    local default_value="$2"

    if [[ -z "${!var_name:-}" ]]; then
        export "${var_name}=${default_value}"

        if declare -f log_debug >/dev/null 2>&1; then
            log_debug "Set ${var_name}=${default_value} (default)"
        fi
    else
        if declare -f log_debug >/dev/null 2>&1; then
            log_debug "Using ${var_name}=${!var_name} (from environment)"
        fi
    fi
}

# validate_project_structure: Check we're in the correct project directory
validate_project_structure() {
    local required_files=("$@")

    for file in "${required_files[@]}"; do
        if [[ ! -e "${file}" ]]; then
            if declare -f log_error >/dev/null 2>&1; then
                log_error "Required file/directory not found: ${file}"
                log_error "Are you running this from the project root?"
            else
                echo "[ERROR] Required file/directory not found: ${file}" >&2
                echo "[ERROR] Are you running this from the project root?" >&2
            fi
            return 1
        fi
    done

    return 0
}

# get_project_root: Find project root by looking for marker files
get_project_root() {
    local marker_file="${1:-.git}"
    local current_dir
    current_dir="$(pwd)"

    while [[ "${current_dir}" != "/" ]]; do
        if [[ -e "${current_dir}/${marker_file}" ]]; then
            echo "${current_dir}"
            return 0
        fi
        current_dir="$(dirname "${current_dir}")"
    done

    if declare -f log_error >/dev/null 2>&1; then
        log_error "Could not find project root (looking for ${marker_file})"
    else
        echo "[ERROR] Could not find project root (looking for ${marker_file})" >&2
    fi
    return 1
}

# Export functions
export -f validate_go_environment
export -f validate_python_environment
export -f validate_node_environment
export -f validate_docker_environment
export -f set_default_env
export -f validate_project_structure
export -f get_project_root
