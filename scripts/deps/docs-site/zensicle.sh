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

    # System Python is externally managed (PEP 668), so install into a
    # git-ignored virtual environment instead of globally.
    if [ ! -x .venv/bin/pip ]; then
        python3 -m venv .venv
    fi
    .venv/bin/pip install --upgrade zensical

    # Record the newly installed version in the pinned requirements file.
    VERSION="$(.venv/bin/pip show zensical | awk '/^Version:/ {print $2}')"
    sed -i -E "s/^zensical==.*/zensical==${VERSION}/" requirements.txt
    echo "requirements.txt pinned to zensical==${VERSION}"

done
