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

    # Update prod, dev, optional, peer, and packageManager dependencies.
    npx npm-check-updates -u

    # Also update flat (string-valued) entries in the "overrides" section.
    # npm-check-updates excludes "overrides" from its default --dep list, so
    # packages declared there (e.g. smol-toml, js-yaml, markdown-it) are
    # silently skipped without this extra pass.
    #
    # The frontend package.json contains nested object overrides
    # (e.g. { "eslint-plugin-react-hooks": { "eslint": "^x.y" } }) which
    # cause ncu to crash when --dep overrides is used without a filter.
    # To avoid that, we run a separate targeted pass that only touches the
    # known flat top-level override ("typescript") in the frontend.
    if [ "$MODULE" = "/projects/Charon/frontend" ]; then
        # Update only the flat top-level override; skip nested object entries.
        npx npm-check-updates -u --dep overrides --filter typescript
    else
        # Root package.json has only flat string overrides — safe to update all.
        npx npm-check-updates -u --dep overrides
    fi

    rm -rf node_modules package-lock.json
    npm install --ignore-scripts
    npm dedupe
    npm run --if-present build
    npm audit --audit-level=high
    npm audit fix
    npm outdated

    echo "Done: $MODULE"
done

echo ""
echo "All npm dependencies updated successfully."
