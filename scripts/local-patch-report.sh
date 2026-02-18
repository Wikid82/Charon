#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
BASELINE="${CHARON_PATCH_BASELINE:-origin/main...HEAD}"
BACKEND_COVERAGE_FILE="$ROOT_DIR/backend/coverage.txt"
FRONTEND_COVERAGE_FILE="$ROOT_DIR/frontend/coverage/lcov.info"
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

BASE_REF="$BASELINE"
if [[ "$BASELINE" == *"..."* ]]; then
    BASE_REF="${BASELINE%%...*}"
fi

if [[ -n "$BASE_REF" ]] && ! git -C "$ROOT_DIR" rev-parse --verify --quiet "${BASE_REF}^{commit}" >/dev/null; then
    echo "Error: baseline base ref '$BASE_REF' is not available locally. Set CHARON_PATCH_BASELINE to a valid range and retry." >&2
    exit 1
fi

mkdir -p "$ROOT_DIR/test-results"

(
    cd "$ROOT_DIR/backend"
    go run ./cmd/localpatchreport \
        --repo-root "$ROOT_DIR" \
        --baseline "$BASELINE" \
        --backend-coverage "$BACKEND_COVERAGE_FILE" \
        --frontend-coverage "$FRONTEND_COVERAGE_FILE" \
        --json-out "$JSON_OUT" \
        --md-out "$MD_OUT"
)

if [[ ! -s "$JSON_OUT" ]]; then
    echo "Error: expected non-empty JSON artifact at $JSON_OUT" >&2
    exit 1
fi

if [[ ! -s "$MD_OUT" ]]; then
    echo "Error: expected non-empty markdown artifact at $MD_OUT" >&2
    exit 1
fi

echo "Artifacts verified: $JSON_OUT, $MD_OUT"
