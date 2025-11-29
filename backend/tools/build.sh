#!/usr/bin/env bash
set -euo pipefail
# Run the top-level build script from the repository root.
cd "$(dirname "$0")/.."
exec ./tools/build.sh "$@"
