#!/bin/bash
# Pre-commit CodeQL JavaScript/TypeScript scan - CI-aligned
set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}🔍 Running CodeQL JavaScript/TypeScript scan (CI-aligned)...${NC}"
echo ""

# Remove generated artifacts that can create noisy/false findings during CodeQL analysis
rm -rf frontend/coverage frontend/dist playwright-report test-results coverage

# Clean previous database
rm -rf codeql-db-js

# Create database
echo "📦 Creating CodeQL database..."
codeql database create codeql-db-js \
  --language=javascript \
  --build-mode=none \
  --source-root=frontend \
  --threads=0 \
  --overwrite

echo ""
echo "📊 Analyzing with security-and-quality suite..."
# Analyze with CI-aligned suite
codeql database analyze codeql-db-js \
  codeql/javascript-queries:codeql-suites/javascript-security-and-quality.qls \
  --format=sarif-latest \
  --output=codeql-results-js.sarif \
  --sarif-add-baseline-file-info \
  --threads=0

echo -e "${GREEN}✅ CodeQL JavaScript/TypeScript scan complete${NC}"
echo "Results saved to: codeql-results-js.sarif"
echo ""
echo "Run 'pre-commit run codeql-check-findings' to validate findings"
