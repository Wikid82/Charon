#!/bin/bash
# Pre-commit CodeQL Go scan - CI-aligned
set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}🔍 Running CodeQL Go scan (CI-aligned)...${NC}"
echo ""

# Clean previous database
rm -rf codeql-db-go

# Create database
echo "📦 Creating CodeQL database..."
codeql database create codeql-db-go \
  --language=go \
  --source-root=backend \
  --threads=0 \
  --overwrite

echo ""
echo "📊 Analyzing with security-and-quality suite..."
# Analyze with CI-aligned suite
codeql database analyze codeql-db-go \
  codeql/go-queries:codeql-suites/go-security-and-quality.qls \
  --format=sarif-latest \
  --output=codeql-results-go.sarif \
  --sarif-add-baseline-file-info \
  --threads=0

echo -e "${GREEN}✅ CodeQL Go scan complete${NC}"
echo "Results saved to: codeql-results-go.sarif"
echo ""
echo "Run 'pre-commit run codeql-check-findings' to validate findings"
