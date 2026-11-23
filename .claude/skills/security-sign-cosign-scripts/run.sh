#!/usr/bin/env bash
# Security Sign Cosign - Execution Script
#
# This script signs Docker images or files using Cosign (Sigstore).
# Supports both keyless (OIDC) and key-based signing.

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

# Set defaults
set_default_env "COSIGN_EXPERIMENTAL" "1"
set_default_env "COSIGN_YES" "true"

# Parse arguments
TYPE="${1:-docker}"
TARGET="${2:-}"

if [[ -z "${TARGET}" ]]; then
    log_error "Usage: security-sign-cosign <type> <target>"
    log_error "  type:   docker or file"
    log_error "  target: Docker image tag or file path"
    log_error ""
    log_error "Examples:"
    log_error "  security-sign-cosign docker charon:local"
    log_error "  security-sign-cosign file ./dist/charon-linux-amd64"
    exit 2
fi

# Validate type
case "${TYPE}" in
    docker|file)
        ;;
    *)
        log_error "Invalid type: ${TYPE}"
        log_error "Type must be 'docker' or 'file'"
        exit 2
        ;;
esac

# Check required tools
log_step "ENVIRONMENT" "Validating prerequisites"

if ! command -v cosign >/dev/null 2>&1; then
    log_error "cosign is not installed"
    log_error "Install from: https://github.com/sigstore/cosign"
    log_error "Quick install: go install github.com/sigstore/cosign/v2/cmd/cosign@latest"
    log_error "Or download and verify v2.4.1:"
    log_error "    curl -sLO https://github.com/sigstore/cosign/releases/download/v2.4.1/cosign-linux-amd64"
    log_error "    echo 'c7c1c5ba0cf95e0bc0cfde5c5a84cd5c4e8f8e6c1c3d3b8f5e9e8d8c7b6a5f4e  cosign-linux-amd64' | sha256sum -c"
    log_error "    sudo install cosign-linux-amd64 /usr/local/bin/cosign"
    exit 2
fi

if [[ "${TYPE}" == "docker" ]]; then
    if ! command -v docker >/dev/null 2>&1; then
        log_error "Docker not found - required for image signing"
        log_error "Install from: https://docs.docker.com/get-docker/"
        exit 1
    fi

    if ! docker info >/dev/null 2>&1; then
        log_error "Docker daemon is not running"
        log_error "Start Docker daemon before signing images"
        exit 1
    fi
fi

cd "${PROJECT_ROOT}"

# Determine signing mode
if [[ "${COSIGN_EXPERIMENTAL}" == "1" ]]; then
    SIGNING_MODE="keyless (GitHub OIDC)"
else
    SIGNING_MODE="key-based"

    # Validate key and password are provided for key-based signing
    if [[ -z "${COSIGN_PRIVATE_KEY:-}" ]]; then
        log_error "COSIGN_PRIVATE_KEY environment variable is required for key-based signing"
        log_error "Set COSIGN_EXPERIMENTAL=1 for keyless signing, or provide COSIGN_PRIVATE_KEY"
        exit 2
    fi
fi

log_info "Signing mode: ${SIGNING_MODE}"

# Sign based on type
case "${TYPE}" in
    docker)
        log_step "COSIGN" "Signing Docker image: ${TARGET}"

        # Verify image exists
        if ! docker image inspect "${TARGET}" >/dev/null 2>&1; then
            log_error "Docker image not found: ${TARGET}"
            log_error "Build or pull the image first"
            exit 1
        fi

        # Sign the image
        if [[ "${COSIGN_EXPERIMENTAL}" == "1" ]]; then
            # Keyless signing
            log_info "Using keyless signing (OIDC)"
            if ! cosign sign --yes "${TARGET}" 2>&1 | tee cosign-sign.log; then
                log_error "Failed to sign image with keyless mode"
                log_error "Check that you have valid GitHub OIDC credentials"
                cat cosign-sign.log >&2 || true
                rm -f cosign-sign.log
                exit 1
            fi
            rm -f cosign-sign.log
        else
            # Key-based signing
            log_info "Using key-based signing"

            # Write private key to temporary file
            TEMP_KEY=$(mktemp)
            trap 'rm -f "${TEMP_KEY}"' EXIT
            echo "${COSIGN_PRIVATE_KEY}" > "${TEMP_KEY}"

            # Sign with key
            if [[ -n "${COSIGN_PASSWORD:-}" ]]; then
                export COSIGN_PASSWORD
            fi

            if ! cosign sign --yes --key "${TEMP_KEY}" "${TARGET}" 2>&1 | tee cosign-sign.log; then
                log_error "Failed to sign image with key"
                cat cosign-sign.log >&2 || true
                rm -f cosign-sign.log
                exit 1
            fi
            rm -f cosign-sign.log
        fi

        log_success "Image signed successfully"
        log_info "Signature pushed to registry"

        # Show verification command
        if [[ "${COSIGN_EXPERIMENTAL}" == "1" ]]; then
            log_info "Verification command:"
            log_info "  cosign verify ${TARGET} \\"
            log_info "    --certificate-identity-regexp='https://github.com/USER/REPO' \\"
            log_info "    --certificate-oidc-issuer='https://token.actions.githubusercontent.com'"
        else
            log_info "Verification command:"
            log_info "  cosign verify ${TARGET} --key cosign.pub"
        fi
        ;;

    file)
        log_step "COSIGN" "Signing file: ${TARGET}"

        # Verify file exists
        if [[ ! -f "${TARGET}" ]]; then
            log_error "File not found: ${TARGET}"
            exit 1
        fi

        SIGNATURE_FILE="${TARGET}.sig"
        CERT_FILE="${TARGET}.pem"

        # Sign the file
        if [[ "${COSIGN_EXPERIMENTAL}" == "1" ]]; then
            # Keyless signing
            log_info "Using keyless signing (OIDC)"
            if ! cosign sign-blob --yes \
                --output-signature="${SIGNATURE_FILE}" \
                --output-certificate="${CERT_FILE}" \
                "${TARGET}" 2>&1 | tee cosign-sign.log; then
                log_error "Failed to sign file with keyless mode"
                log_error "Check that you have valid GitHub OIDC credentials"
                cat cosign-sign.log >&2 || true
                rm -f cosign-sign.log
                exit 1
            fi
            rm -f cosign-sign.log

            log_success "File signed successfully"
            log_info "Signature: ${SIGNATURE_FILE}"
            log_info "Certificate: ${CERT_FILE}"

            # Show verification command
            log_info "Verification command:"
            log_info "  cosign verify-blob ${TARGET} \\"
            log_info "    --signature ${SIGNATURE_FILE} \\"
            log_info "    --certificate ${CERT_FILE} \\"
            log_info "    --certificate-identity-regexp='https://github.com/USER/REPO' \\"
            log_info "    --certificate-oidc-issuer='https://token.actions.githubusercontent.com'"
        else
            # Key-based signing
            log_info "Using key-based signing"

            # Write private key to temporary file
            TEMP_KEY=$(mktemp)
            trap 'rm -f "${TEMP_KEY}"' EXIT
            echo "${COSIGN_PRIVATE_KEY}" > "${TEMP_KEY}"

            # Sign with key
            if [[ -n "${COSIGN_PASSWORD:-}" ]]; then
                export COSIGN_PASSWORD
            fi

            if ! cosign sign-blob --yes \
                --key "${TEMP_KEY}" \
                --output-signature="${SIGNATURE_FILE}" \
                "${TARGET}" 2>&1 | tee cosign-sign.log; then
                log_error "Failed to sign file with key"
                cat cosign-sign.log >&2 || true
                rm -f cosign-sign.log
                exit 1
            fi
            rm -f cosign-sign.log

            log_success "File signed successfully"
            log_info "Signature: ${SIGNATURE_FILE}"

            # Show verification command
            log_info "Verification command:"
            log_info "  cosign verify-blob ${TARGET} \\"
            log_info "    --signature ${SIGNATURE_FILE} \\"
            log_info "    --key cosign.pub"
        fi
        ;;
esac

log_success "Signing complete"
exit 0
