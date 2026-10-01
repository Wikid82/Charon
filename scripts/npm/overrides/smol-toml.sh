#!/bin/bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"
PACKAGE="smol-toml"

# Only modules that already declare this override. `npm pkg get` prints "{}"
# for a missing key, so it cannot be used as an existence check.
NPM_MODULES=(
        "$REPO_ROOT"
    )

for MODULE in "${NPM_MODULES[@]}"; do
    echo "============================================================================"
    echo "Updating: $MODULE"
    echo "============================================================================"

    cd "$MODULE" || exit 1

    CURRENT="$(node -p "(require('./package.json').overrides || {})['$PACKAGE'] || ''")"
    if [ -z "$CURRENT" ]; then
        echo "No overrides.$PACKAGE in $MODULE; refusing to add one." >&2
        exit 1
    fi

    # Preserve the existing range prefix (^ or ~).
    PREFIX="$(echo "$CURRENT" | grep -o '^[\^~]' || true)"
    LATEST="$(npm view "$PACKAGE" version)"
    npm pkg set "overrides.$PACKAGE=${PREFIX}${LATEST}"
    npm install
done
