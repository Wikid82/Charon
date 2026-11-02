#!/bin/bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"

PIP_MODULES=(
        "$REPO_ROOT/docs-site"
    )

for MODULE in "${PIP_MODULES[@]}"; do
    echo "============================================================================"
    echo "Updating: $MODULE"
    echo "============================================================================"

    cd "$MODULE" || exit 1
    pip install --upgrade --force-reinstall zensical

done
