#!/usr/bin/env bash
set -euo pipefail

# ==============================================================================
# Docker: Start Development Environment - Execution Script
# ==============================================================================
# This script starts the Docker Compose development environment.
#
# Usage: ./run.sh
# Exit codes: 0 = success, non-zero = failure
# ==============================================================================

# Determine the repository root directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"

# Change to repository root
cd "$REPO_ROOT"

# Start development environment with Docker Compose
exec docker compose -f .docker/compose/docker-compose.dev.yml up -d
