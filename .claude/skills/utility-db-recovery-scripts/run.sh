#!/usr/bin/env bash
set -euo pipefail

# ==============================================================================
# Utility: Database Recovery - Execution Script
# ==============================================================================
# This script performs SQLite database integrity checks and recovery.
# It wraps the original db-recovery.sh script.
#
# Usage: ./run.sh [--force]
# Exit codes: 0 = success, 1 = failure
# ==============================================================================

# Determine the repository root directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"

# Change to repository root
cd "$REPO_ROOT"

# Execute the database recovery script
exec scripts/db-recovery.sh "$@"
