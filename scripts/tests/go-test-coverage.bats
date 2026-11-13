#!/usr/bin/env bats
#
# Covers the CHARON_TEST_PACKAGES and CHARON_COVERAGE_GATE knobs of
# scripts/go-test-coverage.sh. The script is copied into a throwaway repo tree
# so its writes to backend/ never touch the real checkout, and `go` is stubbed
# so no real test suite runs.

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"

  TMPROOT="$(mktemp -d)"
  mkdir -p "$TMPROOT/scripts/ci" "$TMPROOT/backend" "$TMPROOT/bin"
  cp "$REPO_ROOT/scripts/go-test-coverage.sh" "$TMPROOT/scripts/"
  cp "$REPO_ROOT/scripts/ci/go-coverage-gate.sh" "$TMPROOT/scripts/ci/"
  export PATH="$TMPROOT/bin:/usr/bin:/bin"
  export GO_CALLS_LOG="$TMPROOT/go-calls.log"
  export CHARON_ENCRYPTION_KEY="$(printf '%032d' 0 | base64)"

  cat >"$TMPROOT/bin/go" <<'STUB'
#!/usr/bin/env bash
echo "$*" >>"$GO_CALLS_LOG"
if [[ "$1" == "test" ]]; then
  for arg in "$@"; do
    case "$arg" in
      -coverprofile=*)
        printf 'mode: atomic\nexample.com/m/a.go:1.1,2.2 1 1\n' >"${arg#-coverprofile=}"
        ;;
    esac
  done
  exit 0
fi
if [[ "$1" == "tool" && "$2" == "cover" ]]; then
  printf 'total:\t(statements)\t100.0%%\n'
  exit 0
fi
if [[ "$1" == "tool" && "$2" == "covdata" ]]; then
  exit 1
fi
exit 0
STUB
  chmod +x "$TMPROOT/bin/go"
  : >"$GO_CALLS_LOG"
  unset CHARON_TEST_PACKAGES CHARON_COVERAGE_GATE
}

teardown() {
  rm -rf "$TMPROOT"
}

run_script() {
  bash "$TMPROOT/scripts/go-test-coverage.sh"
}

@test "defaults to testing ./... with the coverage gate on" {
  run run_script
  [ "$status" -eq 0 ]
  grep -q ' \./\.\.\.$' "$GO_CALLS_LOG"
  grep -q '^tool cover -func=' "$GO_CALLS_LOG"
  [[ "$output" == *"Coverage requirement met"* ]]
}

@test "CHARON_TEST_PACKAGES restricts the packages passed to go test" {
  export CHARON_TEST_PACKAGES="example.com/m/one example.com/m/two"
  run run_script
  [ "$status" -eq 0 ]
  grep -q ' example\.com/m/one example\.com/m/two$' "$GO_CALLS_LOG"
  ! grep -q '\./\.\.\.' "$GO_CALLS_LOG"
}

@test "CHARON_TEST_PACKAGES set but empty is an error, never ./..." {
  export CHARON_TEST_PACKAGES=""
  run run_script
  [ "$status" -eq 2 ]
  [[ "$output" == *"set but empty"* ]]
  [ ! -s "$GO_CALLS_LOG" ]
}

@test "CHARON_COVERAGE_GATE=0 writes the profile and skips the gate" {
  export CHARON_COVERAGE_GATE=0
  run run_script
  [ "$status" -eq 0 ]
  [ -f "$TMPROOT/backend/coverage.txt" ]
  ! grep -q '^tool cover' "$GO_CALLS_LOG"
  [[ "$output" == *"Coverage gate skipped"* ]]
}

@test "CHARON_COVERAGE_GATE=1 still enforces the minimum" {
  export CHARON_COVERAGE_GATE=1
  export CHARON_MIN_COVERAGE=101
  run run_script
  [ "$status" -eq 1 ]
  [[ "$output" == *"below required 101%"* ]]
}

@test "an unknown CHARON_COVERAGE_GATE value is rejected" {
  export CHARON_COVERAGE_GATE=yes
  run run_script
  [ "$status" -eq 2 ]
  [[ "$output" == *"must be 0 or 1"* ]]
}
