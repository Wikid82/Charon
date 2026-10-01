#!/bin/bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"

NPM_MODULES=(
        "$REPO_ROOT/frontend"
    )

for MODULE in "${NPM_MODULES[@]}"; do
    echo "============================================================================"
    echo "Updating: $MODULE"
    echo "============================================================================"

    cd "$MODULE" || exit 1
    npm install vite --save-dev

    # Keep the plugin-react override range in lockstep with the vite devDependency.
    VITE_RANGE="$(node -p "require('./package.json').devDependencies.vite")"
    npm pkg set "overrides.@vitejs/plugin-react.vite=${VITE_RANGE}"
    npm install
done
