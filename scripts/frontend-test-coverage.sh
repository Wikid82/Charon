#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
FRONTEND_DIR="$ROOT_DIR/frontend"
MIN_COVERAGE="${CPM_MIN_COVERAGE:-81}"

cd "$FRONTEND_DIR"

# Run tests with coverage and json-summary reporter
# We use --passWithNoTests just in case, though we have tests.
npm run test:coverage -- --run --coverage.reporter=text --coverage.reporter=json-summary

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
    print(f"Frontend coverage {total}% is below required {minimum}% (set CPM_MIN_COVERAGE to override)", file=sys.stderr)
    sys.exit(1)
PY

echo "Frontend coverage requirement met"
