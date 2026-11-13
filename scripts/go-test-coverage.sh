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
# Optional overrides used by the sharded CI jobs (see scripts/ci/shard-go-packages.sh
# and scripts/ci/merge-go-coverage.sh):
#   CHARON_TEST_PACKAGES  space-separated package list to test (default: ./...).
#                         Set but empty is an error, so a bad shard never silently
#                         runs the whole suite.
#   CHARON_COVERAGE_GATE  1 (default) enforces the minimum line coverage here;
#                         0 still writes the profile but skips the gate, leaving
#                         it to the merge job that sees every shard.
# Per-package `go test` timeout. Go's 10m default is too tight for the slowest
# packages (internal/services takes ~9.5m under -race on CI runners).
TEST_TIMEOUT="${CHARON_TEST_TIMEOUT:-30m}"
COVERAGE_GATE="${CHARON_COVERAGE_GATE:-1}"

case "$COVERAGE_GATE" in
    0|1) ;;
    *)
        echo "Error: CHARON_COVERAGE_GATE must be 0 or 1 (got '${COVERAGE_GATE}')" >&2
        exit 2
        ;;
esac

TEST_PACKAGES=("./...")
if [[ -n "${CHARON_TEST_PACKAGES+x}" ]]; then
    read -r -a TEST_PACKAGES <<< "$CHARON_TEST_PACKAGES"
    if (( ${#TEST_PACKAGES[@]} == 0 )); then
        echo "Error: CHARON_TEST_PACKAGES is set but empty" >&2
        exit 2
    fi
fi

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
    "github.com/Wikid82/charon/backend/internal/crowdsec/hubcachetest"
    "github.com/Wikid82/charon/backend/integration"
)

# internal/database's TestPendingRestoreBootSwap_AcrossRealProcessBoundary
# (and its corrupt-file sibling) prove pending-restore boot-swap behavior by
# compiling and exec'ing cmd/pending-restore-harness as a real, separately
# executed OS process — not an in-process call. Go's -coverprofile
# instrumentation on the `go test` process below has no visibility into that
# child process, so without this, the harness's source would permanently
# show 0% coverage despite being fully exercised. We point the harness
# binary's GOCOVERDIR at this durable directory (via CHARON_HARNESS_GOCOVERDIR,
# read by backend/internal/database/pending_restore_process_test.go) and
# merge its binary coverage data into $COVERAGE_FILE further down, once the
# `go test` run (and the harness subprocess invocations it drives) has
# finished.
HARNESS_COVDIR="$BACKEND_DIR/.harness-covdata"
rm -rf "$HARNESS_COVDIR"
mkdir -p "$HARNESS_COVDIR"
export CHARON_HARNESS_GOCOVERDIR="$HARNESS_COVDIR"

# Try to run tests to produce coverage file; some toolchains may return a non-zero
# exit if certain coverage tooling is unavailable (e.g. covdata) while still
# producing a usable coverage file. Capture the status so we can report real
# test failures after the coverage check.
# Note: -race for race detection; no -v, so output stays small (failures still print --- FAIL)
GO_TEST_STATUS=0
TEST_OUTPUT_FILE=$(mktemp)
trap 'rm -f "$TEST_OUTPUT_FILE"' EXIT

if command -v gotestsum &> /dev/null; then
    set +e
    gotestsum --format pkgname -- -race -timeout="$TEST_TIMEOUT" -mod=readonly -coverprofile="$COVERAGE_FILE" "${TEST_PACKAGES[@]}" 2>&1 | tee "$TEST_OUTPUT_FILE"
    GO_TEST_STATUS=$?
    set -e
else
    set +e
    go test -race -timeout="$TEST_TIMEOUT" -mod=readonly -coverprofile="$COVERAGE_FILE" "${TEST_PACKAGES[@]}" 2>&1 | tee "$TEST_OUTPUT_FILE"
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

# Merge cmd/pending-restore-harness's cross-process binary coverage (see
# HARNESS_COVDIR setup above) into $COVERAGE_FILE. compgen -G checks for the
# covcounters/covmeta files GOCOVERDIR-instrumented binaries write on exit;
# if the harness's process-boundary tests didn't run (e.g. a filtered `go
# test` invocation targeting other packages), this is a no-op.
if compgen -G "$HARNESS_COVDIR/covcounters.*" > /dev/null 2>&1; then
    echo "Merging pending-restore-harness cross-process coverage into ${COVERAGE_FILE}..."
    HARNESS_COVERAGE_TXT="$BACKEND_DIR/.harness-coverage.txt"
    if go tool covdata textfmt -i="$HARNESS_COVDIR" -o="$HARNESS_COVERAGE_TXT" 2>&1; then
        # Both files are covermode "atomic" (coverage.txt because -race forces
        # it; the harness because it's built with -covermode=atomic
        # explicitly), so appending the harness's data lines — everything
        # after its own "mode:" header, which only needs to appear once per
        # file — is a valid merge: each line addresses a distinct
        # file:startLine.startCol,endLine.endCol block that go tool cover
        # sums independently.
        tail -n +2 "$HARNESS_COVERAGE_TXT" >> "$COVERAGE_FILE"
        echo "Merged $(wc -l < "$HARNESS_COVERAGE_TXT" | tr -d ' ') harness coverage line(s)."
    else
        echo "Warning: go tool covdata textfmt failed; continuing without harness coverage merge"
    fi
    rm -f "$HARNESS_COVERAGE_TXT"
else
    echo "No pending-restore-harness coverage data found in ${HARNESS_COVDIR} (harness process-boundary tests may not have run); skipping merge"
fi
rm -rf "$HARNESS_COVDIR"

# Enforce the line-coverage gate (shared with the sharded CI merge job).
if [ "$COVERAGE_GATE" = "1" ]; then
    bash "$ROOT_DIR/scripts/ci/go-coverage-gate.sh" "$COVERAGE_FILE"
else
    echo "Coverage gate skipped (CHARON_COVERAGE_GATE=0); the merge job enforces it across shards"
fi

# Bubble up real test failures (after printing coverage info) so pre-commit
# reflects the actual test status.
if [ "$GO_TEST_STATUS" -ne 0 ]; then
    exit "$GO_TEST_STATUS"
fi
