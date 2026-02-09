#!/usr/bin/env bash
set -euo pipefail

# ==============================================================================
# Utility: Clear Go Cache - Execution Script
# ==============================================================================
# This script clears Go build, test, and module caches, plus gopls cache.
# It wraps the original clear-go-cache.sh script.
#
# Usage: ./run.sh
# Exit codes: 0 = success, 1 = failure
# ==============================================================================

# Determine the repository root directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"

# Change to repository root
cd "$REPO_ROOT"

# Execute the cache clear script
exec scripts/clear-go-cache.sh "$@"
