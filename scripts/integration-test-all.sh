#!/usr/bin/env bash
set -euo pipefail

# Aggregates integration tests using skill entrypoints.

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
SKILL_RUNNER="${PROJECT_ROOT}/.github/skills/scripts/skill-runner.sh"

if [[ ! -x "${SKILL_RUNNER}" ]]; then
  echo "ERROR: skill runner not found or not executable: ${SKILL_RUNNER}" >&2
  exit 1
fi

run_skill() {
  local skill_name="$1"
  shift
  echo "=============================================="
  echo "=== Running skill: ${skill_name} ==="
  echo "=============================================="
  "${SKILL_RUNNER}" "${skill_name}" "$@"
}

run_skill "integration-test-cerberus" "$@"
run_skill "integration-test-coraza" "$@"
run_skill "integration-test-rate-limit" "$@"
run_skill "integration-test-crowdsec" "$@"
run_skill "integration-test-crowdsec-decisions" "$@"
run_skill "integration-test-crowdsec-startup" "$@"

echo "=============================================="
echo "=== ALL INTEGRATION TESTS COMPLETED ==="
echo "=============================================="
