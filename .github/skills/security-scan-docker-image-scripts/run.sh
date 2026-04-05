#!/usr/bin/env bash
# Security Scan Docker Image - Execution Script
#
# Build Docker image and scan with Grype/Syft matching CI supply chain verification
# This script replicates the exact process from supply-chain-pr.yml workflow

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

# Validate environment
log_step "ENVIRONMENT" "Validating prerequisites"

# Check Docker
validate_docker_environment || error_exit "Docker is required but not available"

# Check Syft
if ! command -v syft >/dev/null 2>&1; then
    log_error "Syft not found - install from: https://github.com/anchore/syft"
    log_error "Installation: curl -sSfL https://raw.githubusercontent.com/anchore/syft/main/install.sh | sh -s -- -b /usr/local/bin v1.17.0"
    error_exit "Syft is required for SBOM generation" 2
fi

# Check Grype
if ! command -v grype >/dev/null 2>&1; then
    log_error "Grype not found - install from: https://github.com/anchore/grype"
    log_error "Installation: curl -sSfL https://raw.githubusercontent.com/anchore/grype/main/install.sh | sh -s -- -b /usr/local/bin v0.110.0"
    error_exit "Grype is required for vulnerability scanning" 2
fi

# Check jq
if ! command -v jq >/dev/null 2>&1; then
    log_error "jq not found - install from package manager (apt-get install jq, brew install jq, etc.)"
    error_exit "jq is required for JSON processing" 2
fi

# Verify tool versions match CI
SYFT_INSTALLED_VERSION=$(syft version | grep -oP 'Version:\s*\Kv?[0-9]+\.[0-9]+\.[0-9]+' | head -1 || echo "unknown")
GRYPE_INSTALLED_VERSION=$(grype version | grep -oP 'Version:\s*\Kv?[0-9]+\.[0-9]+\.[0-9]+' | head -1 || echo "unknown")

# Set defaults matching CI workflow
set_default_env "SYFT_VERSION" "v1.42.3"
set_default_env "GRYPE_VERSION" "v0.110.0"
set_default_env "IMAGE_TAG" "charon:local"
set_default_env "FAIL_ON_SEVERITY" "Critical,High"

# Version check (informational only)
log_info "Installed Syft version: ${SYFT_INSTALLED_VERSION}"
log_info "Expected Syft version: ${SYFT_VERSION}"
if [[ "${SYFT_INSTALLED_VERSION}" != "${SYFT_VERSION#v}" ]] && [[ "${SYFT_INSTALLED_VERSION}" != "${SYFT_VERSION}" ]]; then
    log_warning "Syft version mismatch - CI uses ${SYFT_VERSION}, you have ${SYFT_INSTALLED_VERSION}"
    log_warning "Results may differ from CI. Reinstall with: curl -sSfL https://raw.githubusercontent.com/anchore/syft/main/install.sh | sh -s -- -b /usr/local/bin ${SYFT_VERSION}"
fi

log_info "Installed Grype version: ${GRYPE_INSTALLED_VERSION}"
log_info "Expected Grype version: ${GRYPE_VERSION}"
if [[ "${GRYPE_INSTALLED_VERSION}" != "${GRYPE_VERSION#v}" ]] && [[ "${GRYPE_INSTALLED_VERSION}" != "${GRYPE_VERSION}" ]]; then
    log_warning "Grype version mismatch - CI uses ${GRYPE_VERSION}, you have ${GRYPE_INSTALLED_VERSION}"
    log_warning "Results may differ from CI. Reinstall with: curl -sSfL https://raw.githubusercontent.com/anchore/grype/main/install.sh | sh -s -- -b /usr/local/bin ${GRYPE_VERSION}"
fi

# Parse arguments
IMAGE_TAG="${1:-${IMAGE_TAG}}"
NO_CACHE_FLAG=""
if [[ "${2:-}" == "no-cache" ]]; then
    NO_CACHE_FLAG="--no-cache"
    log_info "Building without cache (clean build)"
fi

log_info "Image tag: ${IMAGE_TAG}"
log_info "Fail on severity: ${FAIL_ON_SEVERITY}"

cd "${PROJECT_ROOT}"

# ==============================================================================
# Phase 1: Build Docker Image
# ==============================================================================
log_step "BUILD" "Building Docker image: ${IMAGE_TAG}"

# Get build metadata
VERSION="${VERSION:-dev}"
BUILD_DATE=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
VCS_REF=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

log_info "Build args: VERSION=${VERSION}, BUILD_DATE=${BUILD_DATE}, VCS_REF=${VCS_REF}"

# Build Docker image with same args as CI
if docker build ${NO_CACHE_FLAG} \
    --build-arg VERSION="${VERSION}" \
    --build-arg BUILD_DATE="${BUILD_DATE}" \
    --build-arg VCS_REF="${VCS_REF}" \
    -t "${IMAGE_TAG}" \
    -f Dockerfile \
    .; then
    log_success "Docker image built successfully: ${IMAGE_TAG}"
else
    error_exit "Docker build failed" 2
fi

# ==============================================================================
# Phase 2: Generate SBOM
# ==============================================================================
log_step "SBOM" "Generating SBOM using Syft ${SYFT_VERSION}"

log_info "Scanning image: ${IMAGE_TAG}"
log_info "Format: CycloneDX JSON (matches CI)"

# Generate SBOM from the Docker IMAGE (not filesystem)
if syft "${IMAGE_TAG}" \
    --output cyclonedx-json=sbom.cyclonedx.json \
    --output table; then
    log_success "SBOM generation complete"
else
    error_exit "SBOM generation failed" 2
fi

# Count components in SBOM
COMPONENT_COUNT=$(jq '.components | length' sbom.cyclonedx.json 2>/dev/null || echo "0")
log_info "Generated SBOM contains ${COMPONENT_COUNT} packages"

# ==============================================================================
# Phase 3: Scan for Vulnerabilities
# ==============================================================================
log_step "SCAN" "Scanning for vulnerabilities using Grype ${GRYPE_VERSION}"

log_info "Scanning SBOM against vulnerability database..."
log_info "This may take 30-60 seconds on first run (database download)"

# Run Grype against the SBOM (generated from image, not filesystem)
# This matches exactly what CI does in supply-chain-pr.yml
# --config ensures .grype.yaml ignore rules are applied, separating
# ignored matches from actionable ones in the JSON output
if grype sbom:sbom.cyclonedx.json \
    --config .grype.yaml \
    --output json \
    --file grype-results.json; then
    log_success "Vulnerability scan complete"
else
    log_warning "Grype scan completed with findings"
fi

# Generate SARIF output for GitHub Security (matches CI)
grype sbom:sbom.cyclonedx.json \
    --config .grype.yaml \
    --output sarif \
    --file grype-results.sarif 2>/dev/null || true

# ==============================================================================
# Phase 4: Analyze Results
# ==============================================================================
log_step "ANALYSIS" "Analyzing vulnerability scan results"

# Count vulnerabilities by severity (matches CI logic)
if [[ -f grype-results.json ]]; then
    CRITICAL_COUNT=$(jq '[.matches[] | select(.vulnerability.severity == "Critical")] | length' grype-results.json 2>/dev/null || echo "0")
    HIGH_COUNT=$(jq '[.matches[] | select(.vulnerability.severity == "High")] | length' grype-results.json 2>/dev/null || echo "0")
    MEDIUM_COUNT=$(jq '[.matches[] | select(.vulnerability.severity == "Medium")] | length' grype-results.json 2>/dev/null || echo "0")
    LOW_COUNT=$(jq '[.matches[] | select(.vulnerability.severity == "Low")] | length' grype-results.json 2>/dev/null || echo "0")
    NEGLIGIBLE_COUNT=$(jq '[.matches[] | select(.vulnerability.severity == "Negligible")] | length' grype-results.json 2>/dev/null || echo "0")
    UNKNOWN_COUNT=$(jq '[.matches[] | select(.vulnerability.severity == "Unknown")] | length' grype-results.json 2>/dev/null || echo "0")
    TOTAL_COUNT=$(jq '.matches | length' grype-results.json 2>/dev/null || echo "0")
else
    CRITICAL_COUNT=0
    HIGH_COUNT=0
    MEDIUM_COUNT=0
    LOW_COUNT=0
    NEGLIGIBLE_COUNT=0
    UNKNOWN_COUNT=0
    TOTAL_COUNT=0
fi

# Display vulnerability summary
echo ""
log_info "Vulnerability Summary:"
echo "  🔴 Critical: ${CRITICAL_COUNT}"
echo "  🟠 High: ${HIGH_COUNT}"
echo "  🟡 Medium: ${MEDIUM_COUNT}"
echo "  🟢 Low: ${LOW_COUNT}"
if [[ ${NEGLIGIBLE_COUNT} -gt 0 ]]; then
    echo "  ⚪ Negligible: ${NEGLIGIBLE_COUNT}"
fi
if [[ ${UNKNOWN_COUNT} -gt 0 ]]; then
    echo "  ❓ Unknown: ${UNKNOWN_COUNT}"
fi
echo "  📊 Total: ${TOTAL_COUNT}"
echo ""

# ==============================================================================
# Phase 5: Detailed Reporting
# ==============================================================================

# Show Critical vulnerabilities if any
if [[ ${CRITICAL_COUNT} -gt 0 ]]; then
    log_error "Critical Severity Vulnerabilities Found:"
    echo ""
    jq -r '.matches[] | select(.vulnerability.severity == "Critical") |
        "  - \(.vulnerability.id) in \(.artifact.name)\n    Package: \(.artifact.name)@\(.artifact.version)\n    Fixed: \(.vulnerability.fix.versions[0] // "No fix available")\n    CVSS: \(.vulnerability.cvss[0].metrics.baseScore // "N/A")\n    Description: \(.vulnerability.description[0:100])...\n"' \
        grype-results.json 2>/dev/null || echo "  (Unable to parse details)"
    echo ""
fi

# Show High vulnerabilities if any
if [[ ${HIGH_COUNT} -gt 0 ]]; then
    log_warning "High Severity Vulnerabilities Found:"
    echo ""
    jq -r '.matches[] | select(.vulnerability.severity == "High") |
        "  - \(.vulnerability.id) in \(.artifact.name)\n    Package: \(.artifact.name)@\(.artifact.version)\n    Fixed: \(.vulnerability.fix.versions[0] // "No fix available")\n    CVSS: \(.vulnerability.cvss[0].metrics.baseScore // "N/A")\n    Description: \(.vulnerability.description[0:100])...\n"' \
        grype-results.json 2>/dev/null || echo "  (Unable to parse details)"
    echo ""
fi

# ==============================================================================
# Phase 6: Exit Code Determination (Matches CI)
# ==============================================================================

# Check if any failing severities were found
SHOULD_FAIL=false

if [[ "${FAIL_ON_SEVERITY}" == *"Critical"* ]] && [[ ${CRITICAL_COUNT} -gt 0 ]]; then
    SHOULD_FAIL=true
fi

if [[ "${FAIL_ON_SEVERITY}" == *"High"* ]] && [[ ${HIGH_COUNT} -gt 0 ]]; then
    SHOULD_FAIL=true
fi

if [[ "${FAIL_ON_SEVERITY}" == *"Medium"* ]] && [[ ${MEDIUM_COUNT} -gt 0 ]]; then
    SHOULD_FAIL=true
fi

if [[ "${FAIL_ON_SEVERITY}" == *"Low"* ]] && [[ ${LOW_COUNT} -gt 0 ]]; then
    SHOULD_FAIL=true
fi

# Final summary and exit
echo ""
log_info "Generated artifacts:"
log_info "  - sbom.cyclonedx.json (SBOM)"
log_info "  - grype-results.json (vulnerability details)"
log_info "  - grype-results.sarif (GitHub Security format)"
echo ""

if [[ "${SHOULD_FAIL}" == "true" ]]; then
    log_error "Found ${CRITICAL_COUNT} Critical and ${HIGH_COUNT} High severity vulnerabilities"
    log_error "These issues must be resolved before deployment"
    log_error "Review grype-results.json for detailed remediation guidance"
    exit 1
else
    if [[ ${TOTAL_COUNT} -gt 0 ]]; then
        log_success "Docker image scan complete - no critical or high vulnerabilities"
        log_info "Found ${MEDIUM_COUNT} Medium and ${LOW_COUNT} Low severity issues (non-blocking)"
    else
        log_success "Docker image scan complete - no vulnerabilities found"
    fi
    exit 0
fi
