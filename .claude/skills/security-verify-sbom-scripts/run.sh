#!/usr/bin/env bash
# Security Verify SBOM - Execution Script
#
# This script generates an SBOM for a Docker image or local file,
# compares it with a baseline (if provided), and scans for vulnerabilities.

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
set_default_env "SBOM_FORMAT" "spdx-json"
set_default_env "VULN_SCAN_ENABLED" "true"

# Parse arguments
TARGET="${1:-}"
BASELINE="${2:-}"

if [[ -z "${TARGET}" ]]; then
    log_error "Usage: security-verify-sbom <target> [baseline]"
    log_error "  target:   Docker image tag or local image name (required)"
    log_error "  baseline: Path to baseline SBOM for comparison (optional)"
    log_error ""
    log_error "Examples:"
    log_error "  security-verify-sbom charon:local"
    log_error "  security-verify-sbom ghcr.io/user/charon:latest"
    log_error "  security-verify-sbom charon:test sbom-baseline.json"
    exit 2
fi

# Validate target format (basic validation)
if [[ ! "${TARGET}" =~ ^[a-zA-Z0-9:/@._-]+$ ]]; then
    log_error "Invalid target format: ${TARGET}"
    log_error "Target must match pattern: [a-zA-Z0-9:/@._-]+"
    exit 2
fi

# Check required tools
log_step "ENVIRONMENT" "Validating prerequisites"

if ! command -v syft >/dev/null 2>&1; then
    log_error "syft is not installed"
    log_error "Install from: https://github.com/anchore/syft"
    log_error "Quick install: curl -sSfL https://raw.githubusercontent.com/anchore/syft/main/install.sh | sh -s -- -b /usr/local/bin"
    exit 2
fi

if ! command -v jq >/dev/null 2>&1; then
    log_error "jq is not installed"
    log_error "Install from: https://stedolan.github.io/jq/download/"
    exit 2
fi

if [[ "${VULN_SCAN_ENABLED}" == "true" ]] && ! command -v grype >/dev/null 2>&1; then
    log_error "grype is not installed (required for vulnerability scanning)"
    log_error "Install from: https://github.com/anchore/grype"
    log_error "Quick install: curl -sSfL https://raw.githubusercontent.com/anchore/grype/main/install.sh | sh -s -- -b /usr/local/bin"
    log_error ""
    log_error "Alternatively, disable vulnerability scanning with: VULN_SCAN_ENABLED=false"
    exit 2
fi

cd "${PROJECT_ROOT}"

# Generate SBOM
log_step "SBOM" "Generating SBOM for ${TARGET}"
log_info "Format: ${SBOM_FORMAT}"

SBOM_OUTPUT="sbom-generated.json"

if ! syft "${TARGET}" -o "${SBOM_FORMAT}" > "${SBOM_OUTPUT}" 2>&1; then
    log_error "Failed to generate SBOM for ${TARGET}"
    log_error "Ensure the image exists locally or can be pulled from a registry"
    exit 1
fi

# Parse and validate SBOM
if [[ ! -f "${SBOM_OUTPUT}" ]]; then
    log_error "SBOM file not generated: ${SBOM_OUTPUT}"
    exit 1
fi

# Validate SBOM schema (SPDX format)
log_info "Validating SBOM schema..."
if ! jq -e '.spdxVersion' "${SBOM_OUTPUT}" >/dev/null 2>&1; then
    log_error "Invalid SBOM: missing spdxVersion field"
    exit 1
fi

if ! jq -e '.packages' "${SBOM_OUTPUT}" >/dev/null 2>&1; then
    log_error "Invalid SBOM: missing packages array"
    exit 1
fi

if ! jq -e '.name' "${SBOM_OUTPUT}" >/dev/null 2>&1; then
    log_error "Invalid SBOM: missing name field"
    exit 1
fi

if ! jq -e '.documentNamespace' "${SBOM_OUTPUT}" >/dev/null 2>&1; then
    log_error "Invalid SBOM: missing documentNamespace field"
    exit 1
fi

SPDX_VERSION=$(jq -r '.spdxVersion' "${SBOM_OUTPUT}")
log_success "SBOM schema valid (${SPDX_VERSION})"

PACKAGE_COUNT=$(jq '.packages | length' "${SBOM_OUTPUT}" 2>/dev/null || echo "0")

if [[ "${PACKAGE_COUNT}" -eq 0 ]]; then
    log_warning "SBOM contains no packages - this may indicate an error"
    log_warning "Target: ${TARGET}"
else
    log_success "Generated SBOM contains ${PACKAGE_COUNT} packages"
fi

# Baseline comparison (if provided)
if [[ -n "${BASELINE}" ]]; then
    log_step "BASELINE" "Comparing with baseline SBOM"

    if [[ ! -f "${BASELINE}" ]]; then
        log_error "Baseline SBOM file not found: ${BASELINE}"
        exit 2
    fi

    BASELINE_COUNT=$(jq '.packages | length' "${BASELINE}" 2>/dev/null || echo "0")

    if [[ "${BASELINE_COUNT}" -eq 0 ]]; then
        log_warning "Baseline SBOM appears empty or invalid"
    else
        log_info "Baseline: ${BASELINE_COUNT} packages, Current: ${PACKAGE_COUNT} packages"

        # Calculate delta and variance using awk for float arithmetic
        DELTA=$((PACKAGE_COUNT - BASELINE_COUNT))
        if [[ "${BASELINE_COUNT}" -gt 0 ]]; then
            # Use awk to prevent integer overflow and get accurate percentage
            VARIANCE_PCT=$(awk -v delta="${DELTA}" -v baseline="${BASELINE_COUNT}" 'BEGIN {printf "%.2f", (delta / baseline) * 100}')
            VARIANCE_ABS=$(awk -v var="${VARIANCE_PCT}" 'BEGIN {print (var < 0 ? -var : var)}')
        else
            VARIANCE_PCT="0.00"
            VARIANCE_ABS="0.00"
        fi

        if [[ "${DELTA}" -gt 0 ]]; then
            log_info "Delta: +${DELTA} packages (${VARIANCE_PCT}% increase)"
        elif [[ "${DELTA}" -lt 0 ]]; then
            log_info "Delta: ${DELTA} packages (${VARIANCE_PCT}% decrease)"
        else
            log_info "Delta: 0 packages (no change)"
        fi

        # Extract package name@version tuples for semantic comparison
        jq -r '.packages[] | "\(.name)@\(.versionInfo // .version // "unknown")"' "${BASELINE}" 2>/dev/null | sort > baseline-packages.txt || true
        jq -r '.packages[] | "\(.name)@\(.versionInfo // .version // "unknown")"' "${SBOM_OUTPUT}" 2>/dev/null | sort > current-packages.txt || true

        # Extract just names for package add/remove detection
        jq -r '.packages[].name' "${BASELINE}" 2>/dev/null | sort > baseline-names.txt || true
        jq -r '.packages[].name' "${SBOM_OUTPUT}" 2>/dev/null | sort > current-names.txt || true

        # Find added packages
        ADDED=$(comm -13 baseline-names.txt current-names.txt 2>/dev/null || echo "")
        if [[ -n "${ADDED}" ]]; then
            log_info "Added packages:"
            echo "${ADDED}" | head -n 10 | while IFS= read -r pkg; do
                VERSION=$(jq -r ".packages[] | select(.name == \"${pkg}\") | .versionInfo // .version // \"unknown\"" "${SBOM_OUTPUT}" 2>/dev/null || echo "unknown")
                log_info "  + ${pkg}@${VERSION}"
            done
            ADDED_COUNT=$(echo "${ADDED}" | wc -l)
            if [[ "${ADDED_COUNT}" -gt 10 ]]; then
                log_info "  ... and $((ADDED_COUNT - 10)) more"
            fi
        else
            log_info "Added packages: (none)"
        fi

        # Find removed packages
        REMOVED=$(comm -23 baseline-names.txt current-names.txt 2>/dev/null || echo "")
        if [[ -n "${REMOVED}" ]]; then
            log_info "Removed packages:"
            echo "${REMOVED}" | head -n 10 | while IFS= read -r pkg; do
                VERSION=$(jq -r ".packages[] | select(.name == \"${pkg}\") | .versionInfo // .version // \"unknown\"" "${BASELINE}" 2>/dev/null || echo "unknown")
                log_info "  - ${pkg}@${VERSION}"
            done
            REMOVED_COUNT=$(echo "${REMOVED}" | wc -l)
            if [[ "${REMOVED_COUNT}" -gt 10 ]]; then
                log_info "  ... and $((REMOVED_COUNT - 10)) more"
            fi
        else
            log_info "Removed packages: (none)"
        fi

        # Detect version changes in existing packages
        log_info "Version changes:"
        CHANGED_COUNT=0
        comm -12 baseline-names.txt current-names.txt 2>/dev/null | while IFS= read -r pkg; do
            BASELINE_VER=$(jq -r ".packages[] | select(.name == \"${pkg}\") | .versionInfo // .version // \"unknown\"" "${BASELINE}" 2>/dev/null || echo "unknown")
            CURRENT_VER=$(jq -r ".packages[] | select(.name == \"${pkg}\") | .versionInfo // .version // \"unknown\"" "${SBOM_OUTPUT}" 2>/dev/null || echo "unknown")
            if [[ "${BASELINE_VER}" != "${CURRENT_VER}" ]]; then
                log_info "  ~ ${pkg}: ${BASELINE_VER} → ${CURRENT_VER}"
                CHANGED_COUNT=$((CHANGED_COUNT + 1))
                if [[ "${CHANGED_COUNT}" -ge 10 ]]; then
                    log_info "  ... (showing first 10 changes)"
                    break
                fi
            fi
        done
        if [[ "${CHANGED_COUNT}" -eq 0 ]]; then
            log_info "  (none)"
        fi

        # Warn if variance exceeds threshold (using awk for float comparison)
        EXCEEDS_THRESHOLD=$(awk -v abs="${VARIANCE_ABS}" 'BEGIN {print (abs > 5.0 ? 1 : 0)}')
        if [[ "${EXCEEDS_THRESHOLD}" -eq 1 ]]; then
            log_warning "Package variance (${VARIANCE_ABS}%) exceeds 5% threshold"
            log_warning "Consider manual review of package changes"
        fi

        # Cleanup temporary files
        rm -f baseline-packages.txt current-packages.txt baseline-names.txt current-names.txt
    fi
fi

# Vulnerability scanning (if enabled)
HAS_CRITICAL=false

if [[ "${VULN_SCAN_ENABLED}" == "true" ]]; then
    log_step "VULN" "Scanning for vulnerabilities"

    VULN_OUTPUT="vuln-results.json"

    # Run Grype on the SBOM
    if grype "sbom:${SBOM_OUTPUT}" -o json > "${VULN_OUTPUT}" 2>&1; then
        log_debug "Vulnerability scan completed successfully"
    else
        GRYPE_EXIT=$?
        if [[ ${GRYPE_EXIT} -eq 1 ]]; then
            log_debug "Grype found vulnerabilities (expected)"
        else
            log_warning "Grype scan encountered an error (exit code: ${GRYPE_EXIT})"
        fi
    fi

    # Parse vulnerability counts by severity
    if [[ -f "${VULN_OUTPUT}" ]]; then
        CRITICAL_COUNT=$(jq '[.matches[] | select(.vulnerability.severity == "Critical")] | length' "${VULN_OUTPUT}" 2>/dev/null || echo "0")
        HIGH_COUNT=$(jq '[.matches[] | select(.vulnerability.severity == "High")] | length' "${VULN_OUTPUT}" 2>/dev/null || echo "0")
        MEDIUM_COUNT=$(jq '[.matches[] | select(.vulnerability.severity == "Medium")] | length' "${VULN_OUTPUT}" 2>/dev/null || echo "0")
        LOW_COUNT=$(jq '[.matches[] | select(.vulnerability.severity == "Low")] | length' "${VULN_OUTPUT}" 2>/dev/null || echo "0")

        log_info "Found: ${CRITICAL_COUNT} Critical, ${HIGH_COUNT} High, ${MEDIUM_COUNT} Medium, ${LOW_COUNT} Low"

        # Display critical vulnerabilities
        if [[ "${CRITICAL_COUNT}" -gt 0 ]]; then
            HAS_CRITICAL=true
            log_error "Critical vulnerabilities detected:"
            jq -r '.matches[] | select(.vulnerability.severity == "Critical") | "  - \(.vulnerability.id) in \(.artifact.name)@\(.artifact.version) (CVSS: \(.vulnerability.cvss[0].metrics.baseScore // "N/A"))"' "${VULN_OUTPUT}" 2>/dev/null | head -n 10
            if [[ "${CRITICAL_COUNT}" -gt 10 ]]; then
                log_error "  ... and $((CRITICAL_COUNT - 10)) more critical vulnerabilities"
            fi
        fi

        # Display high vulnerabilities
        if [[ "${HIGH_COUNT}" -gt 0 ]]; then
            log_warning "High severity vulnerabilities:"
            jq -r '.matches[] | select(.vulnerability.severity == "High") | "  - \(.vulnerability.id) in \(.artifact.name)@\(.artifact.version) (CVSS: \(.vulnerability.cvss[0].metrics.baseScore // "N/A"))"' "${VULN_OUTPUT}" 2>/dev/null | head -n 5
            if [[ "${HIGH_COUNT}" -gt 5 ]]; then
                log_warning "  ... and $((HIGH_COUNT - 5)) more high vulnerabilities"
            fi
        fi

        # Display table format for summary
        log_info "Running table format scan for summary..."
        grype "sbom:${SBOM_OUTPUT}" -o table 2>&1 | tail -n 20 || true
    else
        log_warning "Vulnerability scan results not found"
    fi
else
    log_info "Vulnerability scanning disabled (air-gapped mode)"
fi

# Final summary
echo ""
log_step "SUMMARY" "SBOM Verification Complete"
log_info "Target: ${TARGET}"
log_info "Packages: ${PACKAGE_COUNT}"
if [[ -n "${BASELINE}" ]]; then
    log_info "Baseline comparison: ${VARIANCE_PCT}% variance"
fi
if [[ "${VULN_SCAN_ENABLED}" == "true" ]]; then
    log_info "Vulnerabilities: ${CRITICAL_COUNT} Critical, ${HIGH_COUNT} High, ${MEDIUM_COUNT} Medium, ${LOW_COUNT} Low"
fi
log_info "SBOM file: ${SBOM_OUTPUT}"

# Exit with appropriate code
if [[ "${HAS_CRITICAL}" == "true" ]]; then
    log_error "CRITICAL vulnerabilities found - review required"
    exit 1
fi

if [[ "${HIGH_COUNT:-0}" -gt 0 ]]; then
    log_warning "High severity vulnerabilities found - review recommended"
fi

log_success "Verification complete"
exit 0
