#!/bin/bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

NPM_MODULES=(
        "$REPO_ROOT"
        "$REPO_ROOT/frontend"
        "$REPO_ROOT/docs-site"
    )

for MODULE in "${NPM_MODULES[@]}"; do
    echo "============================================================================"
    echo "Updating: $MODULE"
    echo "============================================================================"

    cd "$MODULE" || exit 1

    if [ -n "$(npm pkg get overrides.smol-toml)" ]; then
        LATEST="$(npm view smol-toml version)"
        npm pkg set "overrides.smol-toml=^${LATEST}"
        npm install
    else
        npm update smol-toml
    fi
done
