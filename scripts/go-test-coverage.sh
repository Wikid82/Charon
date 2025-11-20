#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BACKEND_DIR="$ROOT_DIR/backend"
COVERAGE_FILE="$BACKEND_DIR/coverage.pre-commit.out"
MIN_COVERAGE="${CPM_MIN_COVERAGE:-75}"

cd "$BACKEND_DIR"

go test -coverprofile="$COVERAGE_FILE" ./...

go tool cover -func="$COVERAGE_FILE" | tail -n 1
TOTAL_LINE=$(go tool cover -func="$COVERAGE_FILE" | grep total)
TOTAL_PERCENT=$(echo "$TOTAL_LINE" | awk '{print substr($3, 1, length($3)-1)}')

echo "Computed coverage: ${TOTAL_PERCENT}% (minimum required ${MIN_COVERAGE}%)"

export TOTAL_PERCENT
export MIN_COVERAGE

python3 - <<'PY'
import os, sys
from decimal import Decimal

total = Decimal(os.environ['TOTAL_PERCENT'])
minimum = Decimal(os.environ['MIN_COVERAGE'])
if total < minimum:
    print(f"Coverage {total}% is below required {minimum}% (set CPM_MIN_COVERAGE to override)", file=sys.stderr)
    sys.exit(1)
PY

rm -f "$COVERAGE_FILE"

echo "Coverage requirement met"
