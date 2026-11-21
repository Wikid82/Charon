#!/usr/bin/env bash
set -euo pipefail

# ==============================================================================
# Utility: Bump Beta Version - Execution Script
# ==============================================================================
# This script increments the beta version number across all project files.
# It wraps the original bump_beta.sh script.
#
# Usage: ./run.sh
# Exit codes: 0 = success, non-zero = failure
# ==============================================================================

# Determine the repository root directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"

# Change to repository root
cd "$REPO_ROOT"

# Execute the bump beta script
exec scripts/bump_beta.sh "$@"
