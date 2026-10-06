#!/usr/bin/env bash
set -euo pipefail

QUALITY_WORKFLOW=".github/workflows/quality-checks.yml"
CODECOV_WORKFLOW=".github/workflows/codecov-upload.yml"
EXPECTED_COMMENT='Codecov upload moved to `codecov-upload.yml` (pull_request + workflow_dispatch).'

fail() {
  local message="$1"
  echo "::error title=Codecov trigger/comment drift::${message}"
  exit 1
}

[[ -f "$QUALITY_WORKFLOW" ]] || fail "Missing workflow file: $QUALITY_WORKFLOW"
[[ -f "$CODECOV_WORKFLOW" ]] || fail "Missing workflow file: $CODECOV_WORKFLOW"

grep -qE '^on:' "$QUALITY_WORKFLOW" || fail "quality-checks workflow is missing an 'on:' block"
grep -qE '^on:' "$CODECOV_WORKFLOW" || fail "codecov-upload workflow is missing an 'on:' block"

grep -qE '^  pull_request:' "$QUALITY_WORKFLOW" || fail "quality-checks must run on pull_request"
if grep -qE '^  workflow_dispatch:' "$QUALITY_WORKFLOW"; then
  fail "quality-checks unexpectedly includes workflow_dispatch; keep Codecov manual trigger scoped to codecov-upload workflow"
fi

grep -qE '^  pull_request:' "$CODECOV_WORKFLOW" || fail "codecov-upload must run on pull_request"
grep -qE '^  workflow_dispatch:' "$CODECOV_WORKFLOW" || fail "codecov-upload must run on workflow_dispatch"
if grep -qE '^  pull_request_target:' "$CODECOV_WORKFLOW"; then
  fail "codecov-upload must not use pull_request_target"
fi

if ! grep -Fq "$EXPECTED_COMMENT" "$QUALITY_WORKFLOW"; then
  fail "quality-checks Codecov handoff comment is missing or changed; expected: $EXPECTED_COMMENT"
fi

echo "Codecov trigger/comment parity check passed"
