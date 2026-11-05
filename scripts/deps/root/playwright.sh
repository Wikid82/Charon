#!/bin/bash
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../../.." && pwd)"

NPM_MODULES=(
        "$REPO_ROOT"
        "$REPO_ROOT/frontend"
    )

# Install both packages at one identical version. CI fails if @playwright/test
# differs from the PLAYWRIGHT_IMAGE version. Use the older of the two "latest"
# tags so we never pick a release whose playwright-core isn't published yet.
PW_VERSION="$(printf '%s\n' \
    "$(npm view playwright dist-tags.latest)" \
    "$(npm view @playwright/test dist-tags.latest)" \
    "$(npm view playwright-core dist-tags.latest)" | sort -V | head -n1)"
echo "Using Playwright ${PW_VERSION}"

for MODULE in "${NPM_MODULES[@]}"; do
    echo "============================================================================"
    echo "Updating: $MODULE"
    echo "============================================================================"

    cd "$MODULE" || exit 1
    rm -rf node_modules package-lock.json
    npm install "playwright@${PW_VERSION}" "@playwright/test@${PW_VERSION}" --save-dev
done

echo "Reminder: PLAYWRIGHT_IMAGE in .github/workflows/e2e-tests-split.yml must be v${PW_VERSION}-<distro>."
