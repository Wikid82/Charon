#!/bin/bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"

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
    rm -rf node_modules package-lock.json
    npm install --legacy-peer-deps typescript@7.1.0-beta --save-dev
done
