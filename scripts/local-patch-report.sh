#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BASELINE="${CHARON_PATCH_BASELINE:-}"

# Advisory opt-out (F.4): default is strict (non-zero exit on any
# below-threshold scope). Either --advisory on the command line or
# CHARON_PATCH_REPORT_ADVISORY=1 in the environment switches to advisory
# mode (always exit 0), forwarded to the Go tool as --advisory=true/false.
ADVISORY="false"
if [[ "${CHARON_PATCH_REPORT_ADVISORY:-}" == "1" ]]; then
    ADVISORY="true"
fi
for arg in "$@"; do
    case "$arg" in
        --advisory)
            ADVISORY="true"
            ;;
    esac
done

BACKEND_COVERAGE_FILE="$ROOT_DIR/backend/coverage.txt"
FRONTEND_COVERAGE_FILE="$ROOT_DIR/frontend/coverage/lcov.info"
AGENT_COVERAGE_FILE="$ROOT_DIR/agent/coverage.txt"
JSON_OUT="$ROOT_DIR/test-results/local-patch-report.json"
MD_OUT="$ROOT_DIR/test-results/local-patch-report.md"

write_preflight_artifacts() {
        local reason="$1"
        local generated_at
        generated_at="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"

        mkdir -p "$ROOT_DIR/test-results"

        cat >"$JSON_OUT" <<EOF
{
    "baseline": "${BASELINE}",
    "generated_at": "${generated_at}",
    "mode": "warn",
    "status": "input_missing",
    "warnings": [
        "${reason}"
    ],
    "artifacts": {
        "markdown": "test-results/local-patch-report.md",
        "json": "test-results/local-patch-report.json"
    }
}
EOF

        cat >"$MD_OUT" <<EOF
# Local Patch Coverage Report

## Metadata

- Generated: ${generated_at}
- Baseline: \
\`${BASELINE}\`
- Mode: \`warn\`

## Warnings

- ${reason}

## Artifacts

- Markdown: \`test-results/local-patch-report.md\`
- JSON: \`test-results/local-patch-report.json\`
EOF
}

if ! command -v git >/dev/null 2>&1; then
    echo "Error: git is required to generate local patch report." >&2
    exit 1
fi

if ! command -v go >/dev/null 2>&1; then
    echo "Error: go is required to generate local patch report." >&2
    exit 1
fi

# Three-tier baseline resolution (F.3):
#   Tier 1 - explicit $CHARON_PATCH_BASELINE override (handled above, already set).
#   Tier 2 - ask gh what the current branch's actual open PR base is, so the
#            local diff matches exactly what Codecov compares against.
#   Tier 3 - static heuristic fallback, preferring origin/development over
#            origin/main (per F.2: development is the default integration
#            branch; main is only ever a target for the scheduled nightly
#            promotion PR).
if [[ -z "$BASELINE" ]] && command -v gh >/dev/null 2>&1; then
    GH_BASE_REF="$(timeout 5s gh pr view --json baseRefName -q .baseRefName 2>/dev/null || true)"
    if [[ -n "$GH_BASE_REF" ]] && git -C "$ROOT_DIR" rev-parse --verify --quiet "origin/${GH_BASE_REF}^{commit}" >/dev/null; then
        BASELINE="origin/${GH_BASE_REF}...HEAD"
    fi
fi

if [[ -z "$BASELINE" ]]; then
    if git -C "$ROOT_DIR" rev-parse --verify --quiet "origin/development^{commit}" >/dev/null; then
        BASELINE="origin/development...HEAD"
    elif git -C "$ROOT_DIR" rev-parse --verify --quiet "development^{commit}" >/dev/null; then
        BASELINE="development...HEAD"
    elif git -C "$ROOT_DIR" rev-parse --verify --quiet "origin/main^{commit}" >/dev/null; then
        BASELINE="origin/main...HEAD"
    elif git -C "$ROOT_DIR" rev-parse --verify --quiet "main^{commit}" >/dev/null; then
        BASELINE="main...HEAD"
    else
        BASELINE="origin/development...HEAD"
    fi
fi

if [[ ! -f "$BACKEND_COVERAGE_FILE" ]]; then
    write_preflight_artifacts "backend coverage input missing at $BACKEND_COVERAGE_FILE"
    echo "Error: backend coverage input missing at $BACKEND_COVERAGE_FILE" >&2
    exit 1
fi

if [[ ! -f "$FRONTEND_COVERAGE_FILE" ]]; then
    write_preflight_artifacts "frontend coverage input missing at $FRONTEND_COVERAGE_FILE"
    echo "Error: frontend coverage input missing at $FRONTEND_COVERAGE_FILE" >&2
    exit 1
fi

if [[ ! -f "$AGENT_COVERAGE_FILE" ]]; then
    write_preflight_artifacts "agent coverage input missing at $AGENT_COVERAGE_FILE"
    echo "Error: agent coverage input missing at $AGENT_COVERAGE_FILE" >&2
    exit 1
fi

BASE_REF="$BASELINE"
if [[ "$BASELINE" == *"..."* ]]; then
    BASE_REF="${BASELINE%%...*}"
fi

if [[ -n "$BASE_REF" ]] && ! git -C "$ROOT_DIR" rev-parse --verify --quiet "${BASE_REF}^{commit}" >/dev/null; then
    echo "Error: baseline base ref '$BASE_REF' is not available locally. Set CHARON_PATCH_BASELINE to a valid range and retry (default attempts origin/development, then development)." >&2
    exit 1
fi

mkdir -p "$ROOT_DIR/test-results"

# Run the Go tool with `set -e` relaxed so a strict-mode non-zero exit (a
# scope below threshold) doesn't abort this script before the artifact
# checks below run — the report's exit code is propagated verbatim at the
# very end instead, after those checks have had a chance to confirm the
# artifacts are actually on disk.
set +e
(
    cd "$ROOT_DIR/backend"
    go run ./cmd/localpatchreport \
        --repo-root "$ROOT_DIR" \
        --baseline "$BASELINE" \
        --backend-coverage "$BACKEND_COVERAGE_FILE" \
        --frontend-coverage "$FRONTEND_COVERAGE_FILE" \
        --agent-coverage "$AGENT_COVERAGE_FILE" \
        --json-out "$JSON_OUT" \
        --md-out "$MD_OUT" \
        --advisory="$ADVISORY"
)
REPORT_STATUS=$?
set -e

if [[ ! -s "$JSON_OUT" ]]; then
    echo "Error: expected non-empty JSON artifact at $JSON_OUT" >&2
    exit 1
fi

if [[ ! -s "$MD_OUT" ]]; then
    echo "Error: expected non-empty markdown artifact at $MD_OUT" >&2
    exit 1
fi

echo "Artifacts verified: $JSON_OUT, $MD_OUT"

exit "$REPORT_STATUS"
