#!/bin/bash
# Check CodeQL SARIF results for HIGH/CRITICAL findings
set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

FAILED=0

check_sarif() {
    local sarif_file=$1
    local lang=$2

    if [ ! -f "$sarif_file" ]; then
        echo -e "${YELLOW}⚠️  No SARIF file found: $sarif_file${NC}"
        echo "Run CodeQL scan first: pre-commit run codeql-$lang-scan --all-files"
        return 0
    fi

    echo "🔍 Checking $lang findings..."

    # Check for findings using jq (if available)
    if command -v jq &> /dev/null; then
        # Count high/critical severity findings
        HIGH_COUNT=$(jq -r '.runs[].results[] | select(.level == "error" or .level == "warning") | .level' "$sarif_file" 2>/dev/null | wc -l || echo 0)

        if [ "$HIGH_COUNT" -gt 0 ]; then
            echo -e "${RED}❌ Found $HIGH_COUNT potential security issues in $lang code${NC}"
            echo ""
            echo "Summary:"
            jq -r '.runs[].results[] | "\(.level): \(.message.text) (\(.locations[0].physicalLocation.artifactLocation.uri):\(.locations[0].physicalLocation.region.startLine))"' "$sarif_file" 2>/dev/null | head -10
            echo ""
            echo "View full results: code $sarif_file"
            FAILED=1
        else
            echo -e "${GREEN}✅ No security issues found in $lang code${NC}"
        fi
    else
        # Fallback: check if file has results
        if grep -q '"results"' "$sarif_file" && ! grep -q '"results": \[\]' "$sarif_file"; then
            echo -e "${YELLOW}⚠️  CodeQL findings detected in $lang (install jq for details)${NC}"
            echo "View results: code $sarif_file"
            FAILED=1
        else
            echo -e "${GREEN}✅ No security issues found in $lang code${NC}"
        fi
    fi
}

echo "🔒 Checking CodeQL findings..."
echo ""

check_sarif "codeql-results-go.sarif" "go"
check_sarif "codeql-results-js.sarif" "js"

if [ $FAILED -eq 1 ]; then
    echo ""
    echo -e "${RED}❌ CodeQL scan found security issues. Please fix before committing.${NC}"
    echo ""
    echo "To view results:"
    echo "  - VS Code: Install SARIF Viewer extension"
    echo "  - Command line: jq . codeql-results-*.sarif"
    exit 1
fi

echo ""
echo -e "${GREEN}✅ All CodeQL checks passed${NC}"
