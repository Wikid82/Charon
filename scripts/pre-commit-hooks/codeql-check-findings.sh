#!/bin/bash
# Check CodeQL SARIF results for blocking findings (CI-aligned)
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
        echo -e "${RED}❌ No SARIF file found: $sarif_file${NC}"
        echo "Run CodeQL scan first: pre-commit run --hook-stage manual codeql-$lang-scan --all-files"
        FAILED=1
        return 1
    fi

    echo "🔍 Checking $lang findings..."

        # Check for findings using jq (if available)
    if command -v jq &> /dev/null; then
                # Count blocking findings.
                # CI behavior: block only effective level=error (high/critical equivalent);
                # warnings are reported but non-blocking unless escalated by policy.
                BLOCKING_COUNT=$(jq -r '[
                    .runs[] as $run
                    | $run.results[]
                    | . as $result
                    | ($run.tool.driver.rules // []) as $rules
                    | ((
                        $result.level
                        // (if (($result.ruleIndex | type) == "number") then ($rules[$result.ruleIndex].defaultConfiguration.level // empty) else empty end)
                        // ([
                            $rules[]?
                            | select((.id // "") == ($result.ruleId // ""))
                            | (.defaultConfiguration.level // empty)
                        ][0] // empty)
                        // ""
                    ) | ascii_downcase) as $effectiveLevel
                    | select($effectiveLevel == "error")
                ] | length' "$sarif_file" 2>/dev/null || echo 0)

                WARNING_COUNT=$(jq -r '[
                    .runs[] as $run
                    | $run.results[]
                    | . as $result
                    | ($run.tool.driver.rules // []) as $rules
                    | ((
                        $result.level
                        // (if (($result.ruleIndex | type) == "number") then ($rules[$result.ruleIndex].defaultConfiguration.level // empty) else empty end)
                        // ([
                            $rules[]?
                            | select((.id // "") == ($result.ruleId // ""))
                            | (.defaultConfiguration.level // empty)
                        ][0] // empty)
                        // ""
                    ) | ascii_downcase) as $effectiveLevel
                    | select($effectiveLevel == "warning")
                ] | length' "$sarif_file" 2>/dev/null || echo 0)

        if [ "$BLOCKING_COUNT" -gt 0 ]; then
            echo -e "${RED}❌ Found $BLOCKING_COUNT blocking CodeQL issues in $lang code${NC}"
            echo ""
            echo "Blocking summary (error-level):"
                        jq -r '
                            .runs[] as $run
                            | $run.results[]
                            | . as $result
                            | ($run.tool.driver.rules // []) as $rules
                            | ((
                                $result.level
                                // (if (($result.ruleIndex | type) == "number") then ($rules[$result.ruleIndex].defaultConfiguration.level // empty) else empty end)
                                // ([
                                    $rules[]?
                                    | select((.id // "") == ($result.ruleId // ""))
                                    | (.defaultConfiguration.level // empty)
                                ][0] // empty)
                                // ""
                            ) | ascii_downcase) as $effectiveLevel
                            | select($effectiveLevel == "error")
                            | "\($effectiveLevel): \($result.ruleId // "<unknown-rule>"): \($result.message.text) (\($result.locations[0].physicalLocation.artifactLocation.uri):\($result.locations[0].physicalLocation.region.startLine))"
                        ' "$sarif_file" 2>/dev/null | head -10
            echo ""
            echo "View full results: code $sarif_file"
            FAILED=1
        else
            echo -e "${GREEN}✅ No blocking CodeQL issues found in $lang code${NC}"
            if [ "$WARNING_COUNT" -gt 0 ]; then
                echo -e "${YELLOW}⚠️  Non-blocking warnings in $lang: $WARNING_COUNT (policy triage required)${NC}"
            fi
        fi
    else
                        echo -e "${RED}❌ jq is required for semantic CodeQL severity evaluation (${lang})${NC}"
                        echo "Install jq and re-run: pre-commit run --hook-stage manual codeql-check-findings --all-files"
                        FAILED=1
    fi
}

echo "🔒 Checking CodeQL findings..."
echo ""

                if ! command -v jq &> /dev/null; then
                    echo -e "${RED}❌ jq is required for CodeQL finding checks${NC}"
                    echo "Install jq and re-run: pre-commit run --hook-stage manual codeql-check-findings --all-files"
                    exit 1
                fi

check_sarif "codeql-results-go.sarif" "go"

# Support both JS artifact names, preferring the CI-aligned canonical file.
if [ -f "codeql-results-js.sarif" ]; then
    check_sarif "codeql-results-js.sarif" "js"
elif [ -f "codeql-results-javascript.sarif" ]; then
    echo -e "${YELLOW}⚠️  Using legacy JS SARIF artifact name: codeql-results-javascript.sarif${NC}"
    check_sarif "codeql-results-javascript.sarif" "js"
else
    check_sarif "codeql-results-js.sarif" "js"
fi

if [ $FAILED -eq 1 ]; then
    echo ""
    echo -e "${RED}❌ CodeQL scan found blocking findings (error-level). Please fix before committing.${NC}"
    echo ""
    echo "To view results:"
    echo "  - VS Code: Install SARIF Viewer extension"
    echo "  - Command line: jq . codeql-results-*.sarif"
    exit 1
fi

echo ""
echo -e "${GREEN}✅ All CodeQL checks passed${NC}"
