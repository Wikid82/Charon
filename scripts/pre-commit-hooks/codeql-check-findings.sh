#!/bin/bash
# Check CodeQL SARIF results for blocking findings (CI-aligned)
#
# Thin wrapper around scripts/security/codeql-findings-gate.sh — the single
# shared source of truth for blocking logic, also consumed by
# .github/workflows/codeql.yml in CI. Do not reimplement blocking logic
# here; see docs/plans/current_spec.md §4.1/§5.4 for why local/CI drift is
# the exact problem this wrapper exists to prevent.
set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
GATE_SCRIPT="$ROOT_DIR/scripts/security/codeql-findings-gate.sh"

FAILED=0

check_sarif() {
    local sarif_file=$1
    local lang=$2

    if [ ! -f "$sarif_file" ]; then
        echo -e "${RED}❌ No SARIF file found: $sarif_file${NC}"
        echo "Run CodeQL scan first: lefthook run pre-commit (which includes codeql-$lang-scan) or run \`lefthook run codeql\`"
        FAILED=1
        return 1
    fi

    echo "🔍 Checking $lang findings..."

    if ! bash "$GATE_SCRIPT" "$sarif_file" "$lang"; then
        FAILED=1
    fi
}

echo "🔒 Checking CodeQL findings..."
echo ""

if ! command -v jq &> /dev/null; then
    echo -e "${RED}❌ jq is required for CodeQL finding checks${NC}"
    echo "Install jq and re-run: lefthook run pre-commit"
    exit 1
fi

check_sarif "codeql-results-go.sarif" "go"

# Support both JS artifact names, preferring the CI-aligned canonical file.
if [ -f "codeql-results-js.sarif" ]; then
    check_sarif "codeql-results-js.sarif" "js"
elif [ -f "codeql-results-javascript.sarif" ]; then
    echo -e "⚠️  Using legacy JS SARIF artifact name: codeql-results-javascript.sarif"
    check_sarif "codeql-results-javascript.sarif" "js"
else
    check_sarif "codeql-results-js.sarif" "js"
fi

if [ $FAILED -eq 1 ]; then
    echo ""
    echo -e "${RED}❌ CodeQL scan found blocking findings. Please fix before committing.${NC}"
    echo ""
    echo "To view results:"
    echo "  - VS Code: Install SARIF Viewer extension"
    echo "  - Command line: jq . codeql-results-*.sarif"
    exit 1
fi

echo ""
echo -e "${GREEN}✅ All CodeQL checks passed${NC}"
