#!/bin/bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

# eslint is only a dependency of the frontend; the repo root does not install it.
NPM_MODULES=(
        "$REPO_ROOT/frontend"
    )

for MODULE in "${NPM_MODULES[@]}"; do
    echo "============================================================================"
    echo "Updating: $MODULE"
    echo "============================================================================"

    cd "$MODULE" || exit 1
    npm install eslint --save-dev
done
