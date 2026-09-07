#!/bin/bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"

NPM_MODULES=(
    "$REPO_ROOT"
    "$REPO_ROOT/frontend"
)

for MODULE in "${NPM_MODULES[@]}"; do
    echo "============================================================================"
    echo "Updating: $MODULE"
    echo "============================================================================"

    cd "$MODULE" || exit 1
    npm update lucide-react
done
