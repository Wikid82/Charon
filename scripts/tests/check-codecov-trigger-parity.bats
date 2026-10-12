#!/usr/bin/env bats
#
# Covers the codecov.yml validation step of
# scripts/ci/check-codecov-trigger-parity.sh. curl is replaced by a stub on
# PATH so no test touches the network.

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  SCRIPT_UNDER_TEST="$REPO_ROOT/scripts/ci/check-codecov-trigger-parity.sh"

  TMPROOT="$(mktemp -d)"
  mkdir "$TMPROOT/bin"
  export CODECOV_CONFIG="$TMPROOT/codecov.yml"
  echo "codecov: {}" >"$CODECOV_CONFIG"
  export PATH="$TMPROOT/bin:$PATH"
}

teardown() {
  rm -rf "$TMPROOT"
}

# stub_curl <http_code> <body> [exit_code]: fake curl writing <body> to the
# --output file and printing <http_code> (the --write-out value).
stub_curl() {
  local code="$1" body="$2" rc="${3:-0}"
  cat >"$TMPROOT/bin/curl" <<STUB
#!/usr/bin/env bash
while [[ \$# -gt 0 ]]; do
  if [[ "\$1" == "--output" ]]; then out="\$2"; shift; fi
  shift
done
printf '%s' '$body' >"\$out"
printf '%s' '$code'
exit $rc
STUB
  chmod +x "$TMPROOT/bin/curl"
}

run_check() {
  (cd "$REPO_ROOT" && bash "$SCRIPT_UNDER_TEST")
}

@test "Valid! response passes" {
  stub_curl 200 $'Valid!\n{"codecov":{}}'
  run run_check
  [ "$status" -eq 0 ]
  [[ "$output" == *"is valid according to Codecov"* ]]
}

@test "validation errors fail with the validator message" {
  stub_curl 400 "Error at ['require_ci_to_pass']: unknown field"
  run run_check
  [ "$status" -eq 1 ]
  [[ "$output" == *"::error"* ]]
  [[ "$output" == *"unknown field"* ]]
}

@test "curl failure only warns" {
  stub_curl 000 "" 7
  run run_check
  [ "$status" -eq 0 ]
  [[ "$output" == *"::warning"* ]]
}

@test "non-200 response only warns" {
  stub_curl 503 "Service Unavailable"
  run run_check
  [ "$status" -eq 0 ]
  [[ "$output" == *"::warning"* ]]
}

@test "empty 200 response only warns" {
  stub_curl 200 ""
  run run_check
  [ "$status" -eq 0 ]
  [[ "$output" == *"::warning"* ]]
}

@test "missing codecov config fails" {
  rm -f "$CODECOV_CONFIG"
  stub_curl 200 "Valid!"
  run run_check
  [ "$status" -eq 1 ]
  [[ "$output" == *"Missing Codecov config"* ]]
}
