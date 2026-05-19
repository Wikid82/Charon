#!/bin/bash
# Local security scanning script for pre-commit
# Scans Go dependencies for vulnerabilities using govulncheck (fast, no Docker needed)
# For full Trivy scans, run: make security-scan-full

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Get script directory and repo root
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(dirname "$SCRIPT_DIR")"

echo "🔒 Running local security scan..."

# Check if govulncheck is installed
if ! command -v govulncheck &> /dev/null; then
    echo -e "${YELLOW}Installing govulncheck...${NC}"
    # renovate: datasource=go depName=golang.org/x/vuln
    go install golang.org/x/vuln/cmd/govulncheck@v1.3.0
fi

# Run govulncheck on backend Go code
echo "📦 Scanning Go dependencies for vulnerabilities..."
cd "$REPO_ROOT/backend"

# Run govulncheck and capture output
VULN_OUTPUT=$(govulncheck ./... 2>&1) || true

# Check for actual vulnerabilities (not just "No vulnerabilities found")
if echo "$VULN_OUTPUT" | grep -q "Vulnerability"; then
    echo -e "${RED}❌ Vulnerabilities found in Go dependencies:${NC}"
    echo "$VULN_OUTPUT"

    # Count HIGH/CRITICAL vulnerabilities
    HIGH_COUNT=$(echo "$VULN_OUTPUT" | grep -c "Severity: HIGH\|CRITICAL" || true)

    if [ "$HIGH_COUNT" -gt 0 ]; then
        echo -e "${RED}Found $HIGH_COUNT HIGH/CRITICAL vulnerabilities. Please fix before committing.${NC}"
        exit 1
    else
        echo -e "${YELLOW}⚠️  Found vulnerabilities, but none are HIGH/CRITICAL. Consider fixing.${NC}"
        # Don't fail for lower severity - just warn
    fi
else
    echo -e "${GREEN}✅ No known vulnerabilities in Go dependencies${NC}"
fi

cd "$REPO_ROOT"

# Check for outdated dependencies with known CVEs (quick check)
echo ""
echo "📋 Checking for outdated security-sensitive packages..."

# Check key packages - only show those with updates available (indicated by [...])
cd "$REPO_ROOT/backend"
OUTDATED=$(go list -m -u all 2>/dev/null | grep -E "(crypto|net|quic)" | grep '\[' | head -10 || true)
if [ -n "$OUTDATED" ]; then
    echo -e "${YELLOW}⚠️  Outdated packages found:${NC}"
    echo "$OUTDATED"
else
    echo -e "${GREEN}All security-sensitive packages are up to date${NC}"
fi
cd "$REPO_ROOT"

echo ""
echo -e "${GREEN}✅ Security scan complete${NC}"
echo ""
echo "💡 For a full container scan, run: make security-scan-full"
