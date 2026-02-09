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
MIN_COVERAGE="${CHARON_MIN_COVERAGE:-${CPM_MIN_COVERAGE:-88}}"

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

# Extract and print total coverage summary using python
LINES_PERCENT=$(python3 - <<'PY'
import json
summary = json.load(open('coverage/coverage-summary.json'))['total']
def fmt(metric):
    return f"{metric['pct']}% ({metric['covered']}/{metric['total']})"

print("Frontend coverage summary:")
print(f"  Statements: {fmt(summary['statements'])}")
print(f"  Branches:   {fmt(summary['branches'])}")
print(f"  Functions:  {fmt(summary['functions'])}")
print(f"  Lines:      {fmt(summary['lines'])}")

print(summary['lines']['pct'])
PY
)

python3 - <<PY
import sys
from decimal import Decimal

total = Decimal('$LINES_PERCENT')
minimum = Decimal('$MIN_COVERAGE')
status = "PASS" if total >= minimum else "FAIL"
print(f"Coverage gate: {status} (lines {total}% vs minimum {minimum}%)")
if total < minimum:
    print(f"Frontend coverage {total}% is below required {minimum}% (set CHARON_MIN_COVERAGE or CPM_MIN_COVERAGE to override)", file=sys.stderr)
    sys.exit(1)
PY
