#!/bin/bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
PACKAGE="eslint"

NPM_MODULES=(
        "$REPO_ROOT"
        "$REPO_ROOT/frontend"
    )

PLUGINS=(
    "eslint-plugin-react-hooks"
    "eslint-plugin-jsx-a11y"
    "eslint-plugin-promise"
)

# Fetch the latest eslint version once before looping to save time
LATEST="$(npm view "$PACKAGE" version)"

for (( i=0; i<${#NPM_MODULES[@]}; i++ )); do
    MODULE="${NPM_MODULES[$i]}"
    echo "============================================================================"
    echo "Updating: $MODULE"
    echo "============================================================================"

    cd "$MODULE" || exit 1

    for PLUGIN in "${PLUGINS[@]}"; do
        # Safely query the nested override object.
        CURRENT="$(node -p "((require('./package.json').overrides || {})['$PLUGIN'] || {})['$PACKAGE'] || ''")"

        if [ -z "$CURRENT" ]; then
            echo "No overrides.$PLUGIN.$PACKAGE in $MODULE; skipping." >&2
            continue
        fi

        # Preserve the existing range prefix (^ or ~).
        PREFIX="$(echo "$CURRENT" | grep -o '^[\^~]' || true)"

        npm pkg set "overrides.$PLUGIN.$PACKAGE=${PREFIX}${LATEST}"
    done

    npm install eslint --save-dev
done
