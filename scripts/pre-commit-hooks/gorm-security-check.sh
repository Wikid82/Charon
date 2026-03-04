#!/usr/bin/env bash
# Pre-commit hook for GORM security scanning
# Wrapper for scripts/scan-gorm-security.sh

set -euo pipefail

# Navigate to repository root
cd "$(git rev-parse --show-toplevel)"

echo "🔒 Running GORM Security Scanner..."
echo ""

# Run scanner in check mode (exits 1 if issues found)
./scripts/scan-gorm-security.sh --check
