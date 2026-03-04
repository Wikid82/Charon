#!/usr/bin/env bash
set -euo pipefail

# ==============================================================================
# Docker: Stop Development Environment - Execution Script
# ==============================================================================
# This script stops and removes the Docker Compose development environment.
#
# Usage: ./run.sh
# Exit codes: 0 = success, non-zero = failure
# ==============================================================================

# Determine the repository root directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"

# Change to repository root
cd "$REPO_ROOT"

# Stop development environment with Docker Compose
exec docker compose -f .docker/compose/docker-compose.dev.yml down
