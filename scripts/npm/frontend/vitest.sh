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
    rm -rf node_modules package-lock.json
    npm install vitest @vitest/ui @vitest/coverage-istanbul @vitest/coverage-v8 @vitest/eslint-plugin --save-dev
done
