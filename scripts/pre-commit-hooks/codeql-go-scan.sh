#!/usr/bin/env bash
# Pre-commit CodeQL Go scan - CI-aligned
set -euo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}🔍 Running CodeQL Go scan (CI-aligned)...${NC}"
echo ""

if ! command -v jq >/dev/null 2>&1; then
  echo -e "${RED}❌ jq is required for CodeQL extraction metric validation${NC}"
  exit 1
fi

# Clean previous database
rm -rf codeql-db-go

# Create database
echo "📦 Creating CodeQL database..."
codeql database create codeql-db-go \
  --language=go \
  --source-root=backend \
  --codescanning-config=.github/codeql/codeql-config.yml \
  --threads=0 \
  --overwrite

echo ""
echo "📊 Analyzing with security-and-quality + security-experimental suites..."
ANALYZE_LOG=$(mktemp)
# Analyze with CI-aligned suites (mirrors codeql.yml queries: security-and-quality,security-experimental)
codeql database analyze codeql-db-go \
  codeql/go-queries:codeql-suites/go-security-and-quality.qls \
  codeql/go-queries:codeql-suites/go-security-experimental.qls \
  --format=sarif-latest \
  --output=codeql-results-go.sarif \
  --sarif-add-baseline-file-info \
  --threads=0 2>&1 | tee "$ANALYZE_LOG"

echo ""
echo "🧮 Validating extraction metric against go list baseline..."
BASELINE_COUNT=$(cd backend && go list -json ./... | jq -s 'map((.GoFiles|length)+(.CgoFiles|length))|add')
SCAN_LINE=$(grep -Eo 'CodeQL scanned [0-9]+ out of [0-9]+ Go files' "$ANALYZE_LOG" | tail -1 || true)

if [ -z "$SCAN_LINE" ]; then
  rm -f "$ANALYZE_LOG"
  echo -e "${RED}❌ Could not parse CodeQL extraction metric from analyze output${NC}"
  echo "Expected a line like: CodeQL scanned X out of Y Go files"
  exit 1
fi

EXTRACTED_COUNT=$(echo "$SCAN_LINE" | awk '{print $3}')
RAW_COUNT=$(echo "$SCAN_LINE" | awk '{print $6}')
rm -f "$ANALYZE_LOG"

if [ "$EXTRACTED_COUNT" != "$BASELINE_COUNT" ]; then
  echo -e "${RED}❌ CodeQL extraction drift detected${NC}"
  echo "  - go list compiled-file baseline: $BASELINE_COUNT"
  echo "  - CodeQL extracted compiled files: $EXTRACTED_COUNT"
  echo "  - CodeQL raw-repo denominator:     $RAW_COUNT"
  echo "Resolve suite/trigger/build-tag drift before merging."
  exit 1
fi

echo -e "${GREEN}✅ Extraction parity OK${NC} (compiled baseline=$BASELINE_COUNT, extracted=$EXTRACTED_COUNT, raw=$RAW_COUNT)"

echo -e "${GREEN}✅ CodeQL Go scan complete${NC}"
echo "Results saved to: codeql-results-go.sarif"
echo ""
echo "Run 'lefthook run pre-commit' (or `lefthook run pre-commit` which includes codeql-check-findings) to validate findings"
