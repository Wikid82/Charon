#!/usr/bin/env bash
set -euo pipefail
# ⚠️  DEPRECATED: This script is deprecated and will be removed in v2.0.0
#    Please use: .github/skills/scripts/skill-runner.sh test-backend-coverage
#    For more info: docs/AGENT_SKILLS_MIGRATION.md
echo "⚠️  WARNING: This script is deprecated and will be removed in v2.0.0" >&2
echo "    Please use: .github/skills/scripts/skill-runner.sh test-backend-coverage" >&2
echo "    For more info: docs/AGENT_SKILLS_MIGRATION.md" >&2
echo "" >&2
sleep 1
ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BACKEND_DIR="$ROOT_DIR/backend"
COVERAGE_FILE="$BACKEND_DIR/coverage.txt"
MIN_COVERAGE="${CHARON_MIN_COVERAGE:-${CPM_MIN_COVERAGE:-87}}"

generate_test_encryption_key() {
    if command -v openssl >/dev/null 2>&1; then
        openssl rand -base64 32 | tr -d '\n'
        return
    fi

    if command -v python3 >/dev/null 2>&1; then
        python3 - <<'PY'
import base64
import os

print(base64.b64encode(os.urandom(32)).decode())
PY
        return
    fi

    echo ""
}

ensure_encryption_key() {
    local key_source="existing"
    local decoded_key_hex=""
    local decoded_key_bytes=0

    if [[ -z "${CHARON_ENCRYPTION_KEY:-}" ]]; then
        key_source="generated"
        CHARON_ENCRYPTION_KEY="$(generate_test_encryption_key)"
    fi

    if [[ -z "${CHARON_ENCRYPTION_KEY:-}" ]]; then
        echo "Error: Could not provision CHARON_ENCRYPTION_KEY automatically."
        echo "Install openssl or python3, or set CHARON_ENCRYPTION_KEY manually to a base64-encoded 32-byte key."
        exit 1
    fi

    if ! decoded_key_hex=$(printf '%s' "$CHARON_ENCRYPTION_KEY" | base64 --decode 2>/dev/null | od -An -tx1 -v | tr -d ' \n'); then
        key_source="regenerated"
        CHARON_ENCRYPTION_KEY="$(generate_test_encryption_key)"
        if ! decoded_key_hex=$(printf '%s' "$CHARON_ENCRYPTION_KEY" | base64 --decode 2>/dev/null | od -An -tx1 -v | tr -d ' \n'); then
            echo "Error: CHARON_ENCRYPTION_KEY could not be decoded and regeneration failed."
            echo "Set CHARON_ENCRYPTION_KEY to a valid base64-encoded 32-byte key."
            exit 1
        fi
    fi

    decoded_key_bytes=$(( ${#decoded_key_hex} / 2 ))
    if [[ "$decoded_key_bytes" -ne 32 ]]; then
        key_source="regenerated"
        CHARON_ENCRYPTION_KEY="$(generate_test_encryption_key)"
        if ! decoded_key_hex=$(printf '%s' "$CHARON_ENCRYPTION_KEY" | base64 --decode 2>/dev/null | od -An -tx1 -v | tr -d ' \n'); then
            echo "Error: CHARON_ENCRYPTION_KEY has invalid length and regeneration failed."
            echo "Set CHARON_ENCRYPTION_KEY to a valid base64-encoded 32-byte key."
            exit 1
        fi

        decoded_key_bytes=$(( ${#decoded_key_hex} / 2 ))
        if [[ "$decoded_key_bytes" -ne 32 ]]; then
            echo "Error: Could not provision a valid 32-byte CHARON_ENCRYPTION_KEY."
            exit 1
        fi
    fi

    export CHARON_ENCRYPTION_KEY

    if [[ "$key_source" == "generated" ]]; then
        echo "Info: CHARON_ENCRYPTION_KEY was not set; generated an ephemeral test key."
    elif [[ "$key_source" == "regenerated" ]]; then
        echo "Warning: CHARON_ENCRYPTION_KEY was invalid; generated an ephemeral test key."
    fi
}

ensure_encryption_key

# Perf asserts are sensitive to -race overhead; loosen defaults for hook runs
export PERF_MAX_MS_GETSTATUS_P95="${PERF_MAX_MS_GETSTATUS_P95:-25ms}"
export PERF_MAX_MS_GETSTATUS_P95_PARALLEL="${PERF_MAX_MS_GETSTATUS_P95_PARALLEL:-50ms}"
export PERF_MAX_MS_LISTDECISIONS_P95="${PERF_MAX_MS_LISTDECISIONS_P95:-75ms}"

# trap 'rm -f "$COVERAGE_FILE"' EXIT

cd "$BACKEND_DIR"

# Packages to exclude from coverage (main packages and infrastructure code)
# These are entrypoints and initialization code that don't benefit from unit tests
EXCLUDE_PACKAGES=(
    "github.com/Wikid82/charon/backend/internal/trace"
    "github.com/Wikid82/charon/backend/integration"
)

# Try to run tests to produce coverage file; some toolchains may return a non-zero
# exit if certain coverage tooling is unavailable (e.g. covdata) while still
# producing a usable coverage file. Capture the status so we can report real
# test failures after the coverage check.
# Note: Using -v for verbose output and -race for race detection
GO_TEST_STATUS=0
TEST_OUTPUT_FILE=$(mktemp)
trap 'rm -f "$TEST_OUTPUT_FILE"' EXIT

if command -v gotestsum &> /dev/null; then
    set +e
    gotestsum --format pkgname -- -race -mod=readonly -coverprofile="$COVERAGE_FILE" ./... 2>&1 | tee "$TEST_OUTPUT_FILE"
    GO_TEST_STATUS=$?
    set -e
else
    set +e
    go test -race -v -mod=readonly -coverprofile="$COVERAGE_FILE" ./... 2>&1 | tee "$TEST_OUTPUT_FILE"
    GO_TEST_STATUS=$?
    set -e
fi

if [ "$GO_TEST_STATUS" -ne 0 ]; then
    echo "Warning: go test returned non-zero (status ${GO_TEST_STATUS}); checking coverage file presence"
    echo ""
    echo "============================================"
    echo "FAILED TEST SUMMARY:"
    echo "============================================"
    grep -E "(^--- FAIL:|^FAIL[[:space:]]|^WARNING: DATA RACE|^panic:)" "$TEST_OUTPUT_FILE" || echo "No specific failures captured in output"
    echo "============================================"
fi

# Filter out excluded packages from coverage file
if [ -f "$COVERAGE_FILE" ]; then
    echo "Filtering excluded packages from coverage report..."
    FILTERED_COVERAGE="${COVERAGE_FILE}.filtered"

    # Build sed command with all patterns at once (more efficient than loop)
    SED_PATTERN=""
    for pkg in "${EXCLUDE_PACKAGES[@]}"; do
        if [ -z "$SED_PATTERN" ]; then
            SED_PATTERN="\|^${pkg}|d"
        else
            SED_PATTERN="${SED_PATTERN};\|^${pkg}|d"
        fi
    done

    # Use non-blocking sed with explicit input/output (avoids -i hang issues)
    timeout 30 sed "$SED_PATTERN" "$COVERAGE_FILE" > "$FILTERED_COVERAGE" || {
        echo "Error: Coverage filtering failed or timed out"
        echo "Using unfiltered coverage file"
        cp "$COVERAGE_FILE" "$FILTERED_COVERAGE"
    }

    mv "$FILTERED_COVERAGE" "$COVERAGE_FILE"
    echo "Coverage filtering complete"
fi

if [ ! -f "$COVERAGE_FILE" ]; then
    echo "Error: coverage file not generated by go test"
    exit 1
fi

# Generate coverage report once with timeout protection
# NOTE: Large repos can produce big coverage profiles; allow more time for parsing.
COVERAGE_OUTPUT=$(timeout 180 go tool cover -func="$COVERAGE_FILE" 2>&1) || {
    echo "Error: go tool cover failed or timed out after 180 seconds"
    echo "This may indicate corrupted coverage data or memory issues"
    exit 1
}

# Extract and display the summary line (total coverage)
TOTAL_LINE=$(echo "$COVERAGE_OUTPUT" | awk '/^total:/ {line=$0} END {print line}')

if [ -z "$TOTAL_LINE" ]; then
    echo "Error: Coverage report missing 'total:' line"
    echo "Coverage output:"
    echo "$COVERAGE_OUTPUT"
    exit 1
fi

echo "$TOTAL_LINE"

# Extract statement coverage percentage from go tool cover summary line
STATEMENT_PERCENT=$(echo "$TOTAL_LINE" | awk '{
    if (NF < 3) {
        print "ERROR: Invalid coverage line format" > "/dev/stderr"
        exit 1
    }
    # Extract last field and remove trailing %
    last_field = $NF
    if (last_field !~ /^[0-9]+(\.[0-9]+)?%$/) {
        printf "ERROR: Last field is not a valid percentage: %s\n", last_field > "/dev/stderr"
        exit 1
    }
    # Remove trailing %
    gsub(/%$/, "", last_field)
    print last_field
}')

if [ -z "$STATEMENT_PERCENT" ] || [ "$STATEMENT_PERCENT" = "ERROR" ]; then
    echo "Error: Could not extract coverage percentage from: $TOTAL_LINE"
    exit 1
fi

# Validate that extracted value is numeric (allows decimals and integers)
if ! echo "$STATEMENT_PERCENT" | grep -qE '^[0-9]+(\.[0-9]+)?$'; then
    echo "Error: Extracted coverage value is not numeric: '$STATEMENT_PERCENT'"
    echo "Source line: $TOTAL_LINE"
    exit 1
fi

# Compute line coverage directly from coverprofile blocks (authoritative gate in this script)
# Format per line:
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

# Bubble up real test failures (after printing coverage info) so pre-commit
# reflects the actual test status.
if [ "$GO_TEST_STATUS" -ne 0 ]; then
    exit "$GO_TEST_STATUS"
fi
