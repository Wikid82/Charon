#!/bin/bash
set -e

# Check if gh is installed
if ! command -v gh &> /dev/null; then
    echo "Error: GitHub CLI (gh) is not installed."
    exit 1
fi

# Check if gh-codeql extension is installed
if ! gh extension list | grep -q "github/gh-codeql"; then
    echo "Installing GitHub CodeQL extension..."
    gh extension install github/gh-codeql
fi

echo "Creating CodeQL database..."
# Remove existing db if any
rm -rf codeql-db

# Clean up build artifacts and coverage reports to prevent false positives
echo "Cleaning up build artifacts..."
rm -rf frontend/dist backend/coverage

# Create the database cluster
echo "Creating CodeQL database cluster..."
# We specify --command to ensure Go builds correctly
# We include javascript to scan the frontend (TypeScript/React)
# We use --db-cluster to support multiple languages
gh codeql database create codeql-db --language=go,javascript --db-cluster --source-root . --command "./tools/build.sh" --overwrite

echo "Analyzing CodeQL database..."
# Analyze Go
echo "Analyzing Go..."
gh codeql database analyze codeql-db/go codeql/go-queries:codeql-suites/go-security-and-quality.qls --format=sarif-latest --output=codeql-results-go.sarif --download

# Analyze JavaScript/TypeScript
echo "Analyzing JavaScript/TypeScript..."
gh codeql database analyze codeql-db/javascript codeql/javascript-queries:codeql-suites/javascript-security-and-quality.qls --format=sarif-latest --output=codeql-results-js.sarif --download

echo "Scan complete."
echo "Go results: codeql-results-go.sarif"
echo "JS/TS results: codeql-results-js.sarif"
