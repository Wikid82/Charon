#!/usr/bin/env bash
# Fails if the gocritic settings in the fast pre-commit lint config drift from
# the CI config (backend/.golangci.yml). Drift lets CI flag findings the
# pre-commit hook never reports.
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "$0")/../.." && pwd)"

# Print the gocritic settings block (the key line through the next line at the
# same or lower indentation), stripped of comments and blank lines.
gocritic_block() {
    awk '
        /^ *gocritic:[[:space:]]*$/ { match($0, /^ */); indent = RLENGTH; inblock = 1; print; next }
        inblock {
            if ($0 ~ /^[[:space:]]*$/) next
            match($0, /^ */)
            if (RLENGTH <= indent) exit
            print
        }
    ' "$1" | sed -E 's/[[:space:]]*#.*$//'
}

ci="$(gocritic_block "$ROOT_DIR/backend/.golangci.yml")"
fast="$(gocritic_block "$ROOT_DIR/.golangci-fast.yml")"

if [[ -z "$ci" || "$ci" != "$fast" ]]; then
    echo "ERROR: gocritic settings differ between backend/.golangci.yml and .golangci-fast.yml" >&2
    diff <(echo "$ci") <(echo "$fast") >&2 || true
    exit 1
fi
echo "gocritic settings match between CI and fast lint configs"
