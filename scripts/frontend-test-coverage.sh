#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
FRONTEND_DIR="$ROOT_DIR/frontend"
MIN_COVERAGE="${CHARON_MIN_COVERAGE:-${CPM_MIN_COVERAGE:-85}}"

cd "$FRONTEND_DIR"

# Ensure dependencies are installed for CI runs
npm ci --silent

# Ensure coverage output directories exist to avoid intermittent ENOENT errors
mkdir -p coverage/.tmp

# Run tests with coverage and json-summary reporter (force istanbul provider)
# Using istanbul ensures json-summary and coverage-summary artifacts are produced
# so that downstream checks can parse them reliably.
npm run test:coverage -- --run

SUMMARY_FILE="coverage/coverage-summary.json"

if [ ! -f "$SUMMARY_FILE" ]; then
    echo "Error: Coverage summary file not found at $SUMMARY_FILE"
    exit 1
fi

# Extract total statements percentage using python
TOTAL_PERCENT=$(python3 -c "import json; print(json.load(open('$SUMMARY_FILE'))['total']['statements']['pct'])")

echo "Computed frontend coverage: ${TOTAL_PERCENT}% (minimum required ${MIN_COVERAGE}%)"

python3 - <<PY
import os, sys
from decimal import Decimal

total = Decimal('$TOTAL_PERCENT')
minimum = Decimal('$MIN_COVERAGE')
if total < minimum:
    print(f"Frontend coverage {total}% is below required {minimum}% (set CHARON_MIN_COVERAGE or CPM_MIN_COVERAGE to override)", file=sys.stderr)
    sys.exit(1)
PY

echo "Frontend coverage requirement met"
