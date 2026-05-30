#!/bin/bash

# This script updates npm dependencies for all modules in the project.

set -euo pipefail

export PATH="/usr/share/nodejs/corepack/shims:$PATH"

MODULES=(
    "/projects/Charon"
    "/projects/Charon/frontend"
)

for MODULE in "${MODULES[@]}"; do
    echo "============================================================================"
    echo "Updating: $MODULE"
    echo "============================================================================"

    cd "$MODULE" || exit 1

    npx npm-check-updates -u
    npm install
    npm dedupe
    npm audit --audit-level=high
    npm audit fix
    npm outdated

    echo "Done: $MODULE"
done

echo ""
echo "All npm dependencies updated successfully."
