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

# Validate codecov.yml against Codecov's own validator. An invalid file is
# ignored wholesale by Codecov (default targets apply, settings never take
# effect), so drift here is otherwise silent. Validation errors fail the check;
# an unreachable validator only warns so a third-party outage cannot break CI.
validate_codecov_config() {
  local config="${CODECOV_CONFIG:-codecov.yml}"
  local url="${CODECOV_VALIDATE_URL:-https://codecov.io/validate}"
  local body_file http_code response

  [[ -f "$config" ]] || fail "Missing Codecov config: $config"

  body_file="$(mktemp)"
  if ! http_code="$(curl --silent --show-error --max-time 30 \
    --data-binary "@$config" --output "$body_file" \
    --write-out '%{http_code}' "$url")" || [[ "$http_code" != "200" && "$http_code" != "400" ]]; then
    rm -f "$body_file"
    echo "::warning title=Codecov validation skipped::Could not reach the Codecov validator (HTTP ${http_code:-none}); $config was not validated"
    return 0
  fi

  # The validator answers 200 "Valid!" or 400 with the validation errors; any
  # other status (5xx, 429, proxy errors) is treated as an outage above.
  response="$(cat "$body_file")"
  rm -f "$body_file"
  if [[ -z "$response" ]]; then
    echo "::warning title=Codecov validation skipped::Empty response from the Codecov validator; $config was not validated"
    return 0
  fi

  if [[ "$(head -n 1 <<<"$response")" == "Valid!" ]]; then
    echo "$config is valid according to Codecov"
    return 0
  fi

  # Collapse newlines so the message survives in a single ::error annotation.
  fail "$config failed Codecov validation: $(tr '\n' ' ' <<<"$response" | cut -c1-500)"
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

validate_codecov_config
