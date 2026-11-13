#!/usr/bin/env bash
# Enforces the backend line-coverage gate on a Go coverprofile.
#
# Usage: go-coverage-gate.sh <coverage-file>
#
# Shared by scripts/go-test-coverage.sh (single-run local and hook runs) and the
# sharded CI merge (scripts/ci/merge-go-coverage.sh output), so the threshold
# logic exists in exactly one place. Minimum comes from CHARON_MIN_COVERAGE, then
# CPM_MIN_COVERAGE, then 87.
#
# The authoritative value is line coverage computed from the profile blocks.
# Statement coverage from `go tool cover -func` is printed for information.
set -euo pipefail

COVERAGE_FILE="${1:-}"
MIN_COVERAGE="${CHARON_MIN_COVERAGE:-${CPM_MIN_COVERAGE:-87}}"

if [[ -z "$COVERAGE_FILE" ]]; then
    echo "Usage: $(basename "$0") <coverage-file>" >&2
    exit 2
fi

if [ ! -f "$COVERAGE_FILE" ]; then
    echo "Error: coverage file not found: $COVERAGE_FILE" >&2
    exit 1
fi

# Statement coverage summary (informational, but also proves the profile parses).
COVERAGE_OUTPUT=$(timeout 180 go tool cover -func="$COVERAGE_FILE" 2>&1) || {
    echo "Error: go tool cover failed or timed out after 180 seconds"
    echo "This may indicate corrupted coverage data or memory issues"
    exit 1
}

TOTAL_LINE=$(echo "$COVERAGE_OUTPUT" | awk '/^total:/ {line=$0} END {print line}')

if [ -z "$TOTAL_LINE" ]; then
    echo "Error: Coverage report missing 'total:' line"
    echo "Coverage output:"
    echo "$COVERAGE_OUTPUT"
    exit 1
fi

echo "$TOTAL_LINE"

STATEMENT_PERCENT=$(echo "$TOTAL_LINE" | awk '{
    if (NF < 3) {
        print "ERROR: Invalid coverage line format" > "/dev/stderr"
        exit 1
    }
    last_field = $NF
    if (last_field !~ /^[0-9]+(\.[0-9]+)?%$/) {
        printf "ERROR: Last field is not a valid percentage: %s\n", last_field > "/dev/stderr"
        exit 1
    }
    gsub(/%$/, "", last_field)
    print last_field
}')

if [ -z "$STATEMENT_PERCENT" ] || [ "$STATEMENT_PERCENT" = "ERROR" ]; then
    echo "Error: Could not extract coverage percentage from: $TOTAL_LINE"
    exit 1
fi

if ! echo "$STATEMENT_PERCENT" | grep -qE '^[0-9]+(\.[0-9]+)?$'; then
    echo "Error: Extracted coverage value is not numeric: '$STATEMENT_PERCENT'"
    echo "Source line: $TOTAL_LINE"
    exit 1
fi

# Line coverage from profile blocks. Format per line:
#   file:startLine.startCol,endLine.endCol numStatements count
LINE_PERCENT=$(awk '
BEGIN {
    total_lines = 0
    covered_lines = 0
}
NR == 1 {
    next
}
{
    split($1, pos, ":")
    if (length(pos) < 2) {
        next
    }

    file = pos[1]
    split(pos[2], ranges, ",")
    split(ranges[1], start_parts, ".")
    split(ranges[2], end_parts, ".")

    start_line = start_parts[1] + 0
    end_line = end_parts[1] + 0
    count = $3 + 0

    if (start_line <= 0 || end_line <= 0 || end_line < start_line) {
        next
    }

    for (line = start_line; line <= end_line; line++) {
        key = file ":" line
        if (!(key in seen_total)) {
            seen_total[key] = 1
            total_lines++
        }
        if (count > 0 && !(key in seen_covered)) {
            seen_covered[key] = 1
            covered_lines++
        }
    }
}
END {
    if (total_lines == 0) {
        print "0.0"
        exit 0
    }
    printf "%.1f", (covered_lines * 100.0) / total_lines
}
' "$COVERAGE_FILE")

if [ -z "$LINE_PERCENT" ]; then
    echo "Error: Could not compute line coverage from $COVERAGE_FILE"
    exit 1
fi

if ! echo "$LINE_PERCENT" | grep -qE '^[0-9]+(\.[0-9]+)?$'; then
    echo "Error: Computed line coverage is not numeric: '$LINE_PERCENT'"
    exit 1
fi

echo "Statement coverage: ${STATEMENT_PERCENT}%"
echo "Line coverage: ${LINE_PERCENT}%"
echo "Coverage gate (line coverage): minimum required ${MIN_COVERAGE}%"

if awk -v current="$LINE_PERCENT" -v minimum="$MIN_COVERAGE" 'BEGIN { exit !(current + 0 < minimum + 0) }'; then
    echo "Coverage ${LINE_PERCENT}% is below required ${MIN_COVERAGE}% (set CHARON_MIN_COVERAGE or CPM_MIN_COVERAGE to override)"
    exit 1
fi

echo "Coverage requirement met"
