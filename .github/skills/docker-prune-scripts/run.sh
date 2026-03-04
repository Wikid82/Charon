#!/usr/bin/env bash
set -euo pipefail

# ==============================================================================
# Docker: Prune Unused Resources - Execution Script
# ==============================================================================
# This script removes unused Docker resources to free up disk space.
#
# Usage: ./run.sh
# Exit codes: 0 = success, non-zero = failure
# ==============================================================================

# Remove unused Docker resources (containers, images, networks, build cache)
exec docker system prune -f
