#!/usr/bin/env bats
#
# Covers scripts/ci/merge-go-coverage.sh and scripts/ci/go-coverage-gate.sh.
# The gate calls `go tool cover -func` for its informational statement line; a
# stub `go` on PATH replays canned output so no real Go sources are needed.

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  MERGE="$REPO_ROOT/scripts/ci/merge-go-coverage.sh"
  GATE="$REPO_ROOT/scripts/ci/go-coverage-gate.sh"

  TMPROOT="$(mktemp -d)"
  mkdir "$TMPROOT/bin"
  export PATH="$TMPROOT/bin:$PATH"

  cat >"$TMPROOT/bin/go" <<'STUB'
#!/usr/bin/env bash
if [[ "$1" == "tool" && "$2" == "cover" ]]; then
  if [[ -n "${GO_COVER_TOTAL:-}" ]]; then
    printf 'a.go:1:\tF\t100.0%%\n'
    printf 'total:\t(statements)\t%s\n' "$GO_COVER_TOTAL"
  fi
  exit 0
fi
echo "unexpected go invocation: $*" >&2
exit 99
STUB
  chmod +x "$TMPROOT/bin/go"
  export GO_COVER_TOTAL="50.0%"

  # Block 1.1-3.2 is covered (lines 1-3); block 4.1-6.2 is not (lines 4-6).
  # Line coverage of this profile is 50.0%.
  HALF_PROFILE="$TMPROOT/half.txt"
  printf 'mode: atomic\n%s\n%s\n' \
    "example.com/m/a.go:1.1,3.2 2 1" \
    "example.com/m/a.go:4.1,6.2 2 0" >"$HALF_PROFILE"

  FULL_PROFILE="$TMPROOT/full.txt"
  printf 'mode: atomic\nexample.com/m/b.go:1.1,4.2 3 5\n' >"$FULL_PROFILE"

  unset CHARON_MIN_COVERAGE CPM_MIN_COVERAGE
}

teardown() {
  rm -rf "$TMPROOT"
}

@test "merge keeps one mode header and every data line" {
  run bash "$MERGE" "$TMPROOT/out/merged.txt" "$HALF_PROFILE" "$FULL_PROFILE"
  [ "$status" -eq 0 ]
  [ "$(grep -c '^mode:' "$TMPROOT/out/merged.txt")" -eq 1 ]
  [ "$(head -n 1 "$TMPROOT/out/merged.txt")" = "mode: atomic" ]
  [ "$(wc -l <"$TMPROOT/out/merged.txt")" -eq 4 ]
  grep -qx 'example.com/m/b.go:1.1,4.2 3 5' "$TMPROOT/out/merged.txt"
  grep -qx 'example.com/m/a.go:4.1,6.2 2 0' "$TMPROOT/out/merged.txt"
}

@test "merge rejects mismatched coverage modes" {
  printf 'mode: set\nexample.com/m/c.go:1.1,2.2 1 1\n' >"$TMPROOT/set.txt"
  run bash "$MERGE" "$TMPROOT/merged.txt" "$HALF_PROFILE" "$TMPROOT/set.txt"
  [ "$status" -eq 1 ]
  [[ "$output" == *"mode mismatch"* ]]
  [ ! -f "$TMPROOT/merged.txt" ]
}

@test "merge rejects an input without a mode header" {
  printf 'example.com/m/c.go:1.1,2.2 1 1\n' >"$TMPROOT/nohdr.txt"
  run bash "$MERGE" "$TMPROOT/merged.txt" "$HALF_PROFILE" "$TMPROOT/nohdr.txt"
  [ "$status" -eq 1 ]
  [[ "$output" == *"does not start with a 'mode:' header"* ]]
}

@test "merge rejects a missing input" {
  run bash "$MERGE" "$TMPROOT/merged.txt" "$TMPROOT/does-not-exist.txt"
  [ "$status" -eq 1 ]
  [[ "$output" == *"not found"* ]]
}

@test "merge requires an output and at least one input" {
  run bash "$MERGE" "$TMPROOT/merged.txt"
  [ "$status" -eq 2 ]
}

@test "merge of a single shard reproduces that shard exactly" {
  run bash "$MERGE" "$TMPROOT/merged.txt" "$FULL_PROFILE"
  [ "$status" -eq 0 ]
  diff "$FULL_PROFILE" "$TMPROOT/merged.txt"
}

@test "gate passes when line coverage meets the minimum" {
  run env CHARON_MIN_COVERAGE=50 bash "$GATE" "$HALF_PROFILE"
  [ "$status" -eq 0 ]
  [[ "$output" == *"Line coverage: 50.0%"* ]]
  [[ "$output" == *"Coverage requirement met"* ]]
}

@test "gate fails when line coverage is below the minimum" {
  run env CHARON_MIN_COVERAGE=51 bash "$GATE" "$HALF_PROFILE"
  [ "$status" -eq 1 ]
  [[ "$output" == *"below required 51%"* ]]
}

@test "gate honours the CPM_MIN_COVERAGE fallback" {
  run env CPM_MIN_COVERAGE=60 bash "$GATE" "$HALF_PROFILE"
  [ "$status" -eq 1 ]
  [[ "$output" == *"minimum required 60%"* ]]
}

@test "gate defaults to 87 percent" {
  run bash "$GATE" "$HALF_PROFILE"
  [ "$status" -eq 1 ]
  [[ "$output" == *"minimum required 87%"* ]]
}

@test "gate reads the merged shard profile, not a single shard" {
  # The half profile alone is 50% and the full profile alone is 100%. Merged, they
  # cover 3 of 6 lines plus 4 of 4 lines = 7/10 = 70%, so the gate must read both.
  bash "$MERGE" "$TMPROOT/merged.txt" "$HALF_PROFILE" "$FULL_PROFILE"
  run env CHARON_MIN_COVERAGE=69 bash "$GATE" "$TMPROOT/merged.txt"
  [ "$status" -eq 0 ]
  [[ "$output" == *"Line coverage: 70.0%"* ]]
}

@test "gate fails when the profile has no total line" {
  export GO_COVER_TOTAL=""
  run bash "$GATE" "$HALF_PROFILE"
  [ "$status" -eq 1 ]
  [[ "$output" == *"missing 'total:' line"* ]]
}

@test "gate fails on a missing coverage file" {
  run bash "$GATE" "$TMPROOT/nope.txt"
  [ "$status" -eq 1 ]
  [[ "$output" == *"coverage file not found"* ]]
}

@test "gate requires a coverage file argument" {
  run bash "$GATE"
  [ "$status" -eq 2 ]
}
