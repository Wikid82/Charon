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
    # Keep the existing package-lock.json: regenerating it from scratch drops the
    # optional @tailwindcss/oxide-wasm32-wasi entry, which breaks `npm ci` in Docker.
    rm -rf node_modules
    npm install tailwindcss @tailwindcss/postcss @tailwindcss/vite --save-dev
done
