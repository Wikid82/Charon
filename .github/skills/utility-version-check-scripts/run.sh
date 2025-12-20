#!/usr/bin/env bash
set -euo pipefail

# ==============================================================================
# Utility: Version Check - Execution Script
# ==============================================================================
# This script validates that the .version file matches the latest git tag.
# It wraps the original check-version-match-tag.sh script.
#
# Usage: ./run.sh
# Exit codes: 0 = success, 1 = version mismatch
# ==============================================================================

# Determine the repository root directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"

# Change to repository root
cd "$REPO_ROOT"

# Execute the version check script
exec scripts/check-version-match-tag.sh "$@"
