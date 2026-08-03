#!/usr/bin/env bash
set -euo pipefail
# Runs each Go module's own coverage-gate script in sequence. Invoked by the
# Makefile's `check-module-coverage` target, which previously referenced
# this file without it existing on disk (a pre-existing dangling reference,
# fixed here alongside the new agent coverage script this target now also
# runs).
#
# Scope: backend + agent. Frontend coverage is gated by its own separate
# script/target (scripts/frontend-test-coverage.sh) and was never actually
# invoked by this target, so it is intentionally not run here.

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

echo "=== Backend coverage ==="
bash "$ROOT_DIR/scripts/go-test-coverage.sh"

echo ""
echo "=== Agent coverage ==="
bash "$ROOT_DIR/scripts/agent-test-coverage.sh"
