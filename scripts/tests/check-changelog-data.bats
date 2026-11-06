#!/usr/bin/env bats
#
# Covers scripts/ci/check-changelog-data.sh, the data guard that fails the
# build when the generated What's New data is missing, malformed or dirty.

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  SCRIPT_UNDER_TEST="$REPO_ROOT/scripts/ci/check-changelog-data.sh"

  TMPROOT="$(mktemp -d)"
  DATA="$TMPROOT/changelog.json"

  VALID_ENTRY='{"version":"1.1.0","date":"2026-01-07","features":["add export"],"fixes":["fix login"],"other":["deps: bump foo"],"security":[{"summary":"harden cookies","sha":"abc1234"}]}'
  OLDER_ENTRY='{"version":"1.0.0","date":"2026-01-03","features":[],"fixes":[],"other":["chore: init"],"security":[]}'
}

teardown() {
  rm -rf "$TMPROOT"
}

write_data() {
  printf '%s' "$1" > "$DATA"
}

@test "valid data with two entries passes and reports the newest version" {
  write_data "[$VALID_ENTRY,$OLDER_ENTRY]"

  run "$SCRIPT_UNDER_TEST" "$DATA"
  [ "$status" -eq 0 ]
  [[ "$output" == *"OK:"* ]]
  [[ "$output" == *"2 version entries"* ]]
  [[ "$output" == *"newest 1.1.0"* ]]
}

@test "--expect-newest passes when the first entry matches" {
  write_data "[$VALID_ENTRY,$OLDER_ENTRY]"

  run "$SCRIPT_UNDER_TEST" "$DATA" --expect-newest 1.1.0
  [ "$status" -eq 0 ]
}

@test "--expect-newest fails when the first entry differs" {
  write_data "[$VALID_ENTRY,$OLDER_ENTRY]"

  run "$SCRIPT_UNDER_TEST" "$DATA" --expect-newest 1.2.0
  [ "$status" -eq 1 ]
  [[ "$output" == *"newest entry is '1.1.0', expected '1.2.0'"* ]]
}

@test "the empty placeholder array fails" {
  write_data "[]"

  run "$SCRIPT_UNDER_TEST" "$DATA"
  [ "$status" -eq 1 ]
  [[ "$output" == *"has no version entries"* ]]
}

@test "invalid JSON fails" {
  write_data "[{not json"

  run "$SCRIPT_UNDER_TEST" "$DATA"
  [ "$status" -eq 1 ]
  [[ "$output" == *"is not valid JSON"* ]]
}

@test "a top-level object instead of an array fails" {
  write_data '{"version":"1.1.0"}'

  run "$SCRIPT_UNDER_TEST" "$DATA"
  [ "$status" -eq 1 ]
  [[ "$output" == *"top level must be a JSON array"* ]]
}

@test "an entry missing one of the four arrays fails and names it" {
  write_data '[{"version":"1.1.0","date":"2026-01-07","features":[],"fixes":[],"security":[]}]'

  run "$SCRIPT_UNDER_TEST" "$DATA"
  [ "$status" -eq 1 ]
  [[ "$output" == *"missing array other"* ]]
}

@test "a non-array field where an array is required fails" {
  write_data '[{"version":"1.1.0","date":"2026-01-07","features":"oops","fixes":[],"other":[],"security":[]}]'

  run "$SCRIPT_UNDER_TEST" "$DATA"
  [ "$status" -eq 1 ]
  [[ "$output" == *"missing array features"* ]]
}

@test "a non-hex security sha fails" {
  write_data '[{"version":"1.1.0","date":"2026-01-07","features":[],"fixes":[],"other":[],"security":[{"summary":"harden cookies","sha":"zz99xx"}]}]'

  run "$SCRIPT_UNDER_TEST" "$DATA"
  [ "$status" -eq 1 ]
  [[ "$output" == *"non-hex sha: zz99xx"* ]]
}

# The guard must reject exactly what the backend drops (commitSHAPattern is
# ^[0-9a-f]{7,40}$), so these boundaries mirror that pattern.
sha_entry() {
  write_data '[{"version":"1.1.0","date":"2026-01-07","features":[],"fixes":[],"other":[],"security":[{"summary":"harden cookies","sha":"'"$1"'"}]}]'
}

@test "an uppercase security sha fails" {
  sha_entry "ABC1234"

  run "$SCRIPT_UNDER_TEST" "$DATA"
  [ "$status" -eq 1 ]
  [[ "$output" == *"non-hex sha: ABC1234"* ]]
}

@test "a mixed-case security sha fails" {
  sha_entry "abC1234"

  run "$SCRIPT_UNDER_TEST" "$DATA"
  [ "$status" -eq 1 ]
  [[ "$output" == *"non-hex sha: abC1234"* ]]
}

@test "a 6-character security sha fails" {
  sha_entry "abc123"

  run "$SCRIPT_UNDER_TEST" "$DATA"
  [ "$status" -eq 1 ]
  [[ "$output" == *"non-hex sha: abc123"* ]]
}

@test "a 41-character security sha fails" {
  sha_entry "0123456789abcdef0123456789abcdef012345678"

  run "$SCRIPT_UNDER_TEST" "$DATA"
  [ "$status" -eq 1 ]
  [[ "$output" == *"non-hex sha: 0123456789abcdef0123456789abcdef012345678"* ]]
}

@test "a 7-character lowercase security sha passes" {
  sha_entry "abc1234"

  run "$SCRIPT_UNDER_TEST" "$DATA"
  [ "$status" -eq 0 ]
}

@test "a 40-character lowercase security sha passes" {
  sha_entry "0123456789abcdef0123456789abcdef01234567"

  run "$SCRIPT_UNDER_TEST" "$DATA"
  [ "$status" -eq 0 ]
}

# Only a "Merge " prefix is a merge commit. The generator drops exactly these
# subjects, so the guard must not reject a legitimate subject such as "Mergeable".
@test "a feature subject starting with Mergeable passes" {
  write_data '[{"version":"1.1.0","date":"2026-01-07","features":["Mergeable configs"],"fixes":[],"other":[],"security":[]}]'

  run "$SCRIPT_UNDER_TEST" "$DATA"
  [ "$status" -eq 0 ]
}

@test "a Merge branch subject in features fails" {
  write_data '[{"version":"1.1.0","date":"2026-01-07","features":["Merge branch x"],"fixes":[],"other":[],"security":[]}]'

  run "$SCRIPT_UNDER_TEST" "$DATA"
  [ "$status" -eq 1 ]
  [[ "$output" == *"merge commit subject leaked"* ]]
}

@test "a merge subject in other fails" {
  write_data '[{"version":"1.1.0","date":"2026-01-07","features":[],"fixes":[],"other":["Merge branch '"'"'nightly'"'"' into main"],"security":[]}]'

  run "$SCRIPT_UNDER_TEST" "$DATA"
  [ "$status" -eq 1 ]
  [[ "$output" == *"merge commit subject leaked"* ]]
}

@test "a merge subject in a security summary fails" {
  write_data '[{"version":"1.1.0","date":"2026-01-07","features":[],"fixes":[],"other":[],"security":[{"summary":"Merge pull request","sha":"abc1234"}]}]'

  run "$SCRIPT_UNDER_TEST" "$DATA"
  [ "$status" -eq 1 ]
  [[ "$output" == *"merge commit subject leaked"* ]]
}

@test "a missing file fails" {
  run "$SCRIPT_UNDER_TEST" "$TMPROOT/does-not-exist.json"
  [ "$status" -eq 1 ]
  [[ "$output" == *"changelog data file not found"* ]]
}

@test "no file argument is a usage error" {
  run "$SCRIPT_UNDER_TEST"
  [ "$status" -eq 2 ]
  [[ "$output" == *"a changelog file path is required"* ]]
}

@test "--expect-newest with a non-semver value is a usage error" {
  write_data "[$VALID_ENTRY]"

  run "$SCRIPT_UNDER_TEST" "$DATA" --expect-newest v1.1
  [ "$status" -eq 2 ]
  [[ "$output" == *"--expect-newest must be X.Y.Z"* ]]
}

@test "an unknown flag is a usage error" {
  write_data "[$VALID_ENTRY]"

  run "$SCRIPT_UNDER_TEST" "$DATA" --bogus
  [ "$status" -eq 2 ]
  [[ "$output" == *"unknown argument: --bogus"* ]]
}
