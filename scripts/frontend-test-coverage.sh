#!/usr/bin/env bash
set -euo pipefail

# ⚠️  DEPRECATED: This script is deprecated and will be removed in v2.0.0
#    Please use: .github/skills/scripts/skill-runner.sh test-frontend-coverage
#    For more info: docs/AGENT_SKILLS_MIGRATION.md
echo "⚠️  WARNING: This script is deprecated and will be removed in v2.0.0" >&2
echo "    Please use: .github/skills/scripts/skill-runner.sh test-frontend-coverage" >&2
echo "    For more info: docs/AGENT_SKILLS_MIGRATION.md" >&2
echo "" >&2
sleep 1

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
FRONTEND_DIR="$ROOT_DIR/frontend"
MIN_COVERAGE="${CHARON_MIN_COVERAGE:-${CPM_MIN_COVERAGE:-87}}"
CANONICAL_COVERAGE_DIR="coverage"
RUN_COVERAGE_DIR="coverage/.run-${PPID}-$$-$(date +%s)"

cd "$FRONTEND_DIR"

# Ensure dependencies are installed for CI runs
npm ci --silent

# Ensure coverage output directory exists
mkdir -p "$CANONICAL_COVERAGE_DIR"

cleanup() {
    rm -rf "$RUN_COVERAGE_DIR"
}

trap cleanup EXIT

# Run tests with coverage in an isolated per-run reports directory to avoid
# collisions when multiple coverage processes execute against the same workspace.
VITEST_COVERAGE_REPORTS_DIR="$RUN_COVERAGE_DIR" npm run test:coverage -- --run

# Publish stable artifacts to the canonical coverage directory used by DoD checks.
cp "$RUN_COVERAGE_DIR/coverage-summary.json" "$CANONICAL_COVERAGE_DIR/coverage-summary.json"
cp "$RUN_COVERAGE_DIR/lcov.info" "$CANONICAL_COVERAGE_DIR/lcov.info"

SUMMARY_FILE="$CANONICAL_COVERAGE_DIR/coverage-summary.json"
LCOV_FILE="$CANONICAL_COVERAGE_DIR/lcov.info"

if [ ! -f "$SUMMARY_FILE" ]; then
    echo "Error: Coverage summary file not found at $SUMMARY_FILE"
    exit 1
fi

if [ ! -f "$LCOV_FILE" ]; then
    echo "Error: LCOV coverage file not found at $LCOV_FILE"
    exit 1
fi

# Extract coverage metrics and validate
LINES_PERCENT=$(python3 - <<'PY'
import json
import sys

try:
    with open('coverage/coverage-summary.json') as f:
        summary = json.load(f)
except (json.JSONDecodeError, KeyError, FileNotFoundError) as e:
    print(f"Error: Failed to read coverage-summary.json: {e}", file=sys.stderr)
    sys.exit(1)

if 'total' not in summary:
    print("Error: 'total' key not found in coverage-summary.json", file=sys.stderr)
    sys.exit(1)

total = summary['total']
metrics = ['statements', 'branches', 'functions', 'lines']
for metric in metrics:
    if metric not in total:
        print(f"Error: '{metric}' metric missing from coverage summary", file=sys.stderr)
        sys.exit(1)
    if not isinstance(total[metric], dict) or 'pct' not in total[metric]:
        print(f"Error: '{metric}' metric missing 'pct' field", file=sys.stderr)
        sys.exit(1)

def fmt(metric):
    return f"{metric['pct']}% ({metric['covered']}/{metric['total']})"

# Print summary to stderr (won't be captured as LINES_PERCENT)
print("Frontend coverage summary:", file=sys.stderr)
print(f"  Statements: {fmt(total['statements'])}", file=sys.stderr)
print(f"  Branches:   {fmt(total['branches'])}", file=sys.stderr)
print(f"  Functions:  {fmt(total['functions'])}", file=sys.stderr)
print(f"  Lines:      {fmt(total['lines'])}", file=sys.stderr)

lines_pct = total['lines']['pct']
if not isinstance(lines_pct, (int, float)):
    print(f"Error: Coverage percentage is not numeric: {lines_pct}", file=sys.stderr)
    sys.exit(1)

# Print only the numeric value to stdout (captured into LINES_PERCENT)
print(lines_pct)
PY
)

python3 - <<PY
import sys
from decimal import Decimal, InvalidOperation

lines_percent = """$LINES_PERCENT""".strip()
min_coverage = """$MIN_COVERAGE""".strip()

if not lines_percent:
    print("Error: Failed to extract coverage percentage from coverage-summary.json", file=sys.stderr)
    sys.exit(1)

try:
    total = Decimal(lines_percent)
except InvalidOperation as e:
    print(f"Error: Coverage value is not numeric: '{lines_percent}' ({e})", file=sys.stderr)
    sys.exit(1)

try:
    minimum = Decimal(min_coverage)
except InvalidOperation as e:
    print(f"Error: Minimum coverage value is not numeric: '{min_coverage}' ({e})", file=sys.stderr)
    print("       Set CHARON_MIN_COVERAGE or CPM_MIN_COVERAGE to a numeric percentage value.", file=sys.stderr)
    sys.exit(1)

status = "PASS" if total >= minimum else "FAIL"
print(f"Coverage gate: {status} (lines {total}% vs minimum {minimum}%)")
if total < minimum:
    print(f"Frontend coverage {total}% is below required {minimum}% (set CHARON_MIN_COVERAGE or CPM_MIN_COVERAGE to override)", file=sys.stderr)
    sys.exit(1)
PY
