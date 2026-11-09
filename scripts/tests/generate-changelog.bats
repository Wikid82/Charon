#!/usr/bin/env bats
#
# Covers scripts/generate-changelog.sh, which turns conventional-commit history
# into the embedded What's New data. Each test builds an isolated git repo with
# controlled commit dates and tags, copies the script into it (the script
# resolves its repo root from its own location), and asserts the exact output.
#
# Scenario (see the setup of each test that needs it):
#   v1.0.0  feat, fix(ci), chore history
#   v1.1.0  a side branch merged with --no-ff (merge commit), duplicate subjects
#           differing only by (#NNN), a (security) fix, a release chore, a
#           promotion chore, and internal-scope feat/fix commits

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  SOURCE_SCRIPT="$REPO_ROOT/scripts/generate-changelog.sh"

  TMPROOT="$(mktemp -d)"
  mkdir -p "$TMPROOT/scripts"
  cp "$SOURCE_SCRIPT" "$TMPROOT/scripts/generate-changelog.sh"
  GEN="$TMPROOT/scripts/generate-changelog.sh"
  OUT="$TMPROOT/changelog.json"

  git -C "$TMPROOT" init -q -b main
  git -C "$TMPROOT" config user.email "test@example.com"
  git -C "$TMPROOT" config user.name "Test Runner"
  git -C "$TMPROOT" config commit.gpgsign false
}

teardown() {
  rm -rf "$TMPROOT"
}

# Creates an empty commit with a fixed author and committer date and prints its
# short sha (the same abbreviation the generator emits in security entries).
commit_at() {
  local date="$1" message="$2"
  GIT_AUTHOR_DATE="$date" GIT_COMMITTER_DATE="$date" \
    git -C "$TMPROOT" commit -q --allow-empty -m "$message"
  git -C "$TMPROOT" rev-parse --short HEAD
}

# Sorts every array inside each entry so assertions do not depend on git's
# walk order for equal-looking items. Dedupe and categorization are still
# asserted exactly through the sorted contents.
normalize() {
  jq -cS 'map(.features |= sort | .fixes |= sort | .other |= sort | .security |= sort)' "$1"
}

# Builds the v1.0.0 / v1.1.0 history used by the exact-output tests and
# prints the short sha of the security commit via the global SEC_SHA.
build_release_history() {
  commit_at "2026-01-01T12:00:00Z" "chore: init" >/dev/null
  commit_at "2026-01-02T12:00:00Z" "feat: add dashboard" >/dev/null
  commit_at "2026-01-03T12:00:00Z" "fix(ci): repair workflow" >/dev/null
  git -C "$TMPROOT" tag v1.0.0

  git -C "$TMPROOT" checkout -q -b side
  commit_at "2026-01-04T12:00:00Z" "fix: correct login redirect (#101)" >/dev/null
  SEC_SHA="$(commit_at "2026-01-04T13:00:00Z" "fix(security): harden session cookies")"
  commit_at "2026-01-04T14:00:00Z" "feat: add export button" >/dev/null

  git -C "$TMPROOT" checkout -q main
  commit_at "2026-01-05T12:00:00Z" "feat: add export button" >/dev/null
  commit_at "2026-01-05T13:00:00Z" "fix: correct login redirect (#102)" >/dev/null

  GIT_AUTHOR_DATE="2026-01-06T09:00:00Z" GIT_COMMITTER_DATE="2026-01-06T09:00:00Z" \
    git -C "$TMPROOT" merge -q --no-ff side -m "Merge branch 'side'"

  commit_at "2026-01-06T10:00:00Z" "chore(main): release 1.1.0 (#150)" >/dev/null
  commit_at "2026-01-06T11:00:00Z" "chore: promote nightly to main (#151)" >/dev/null
  commit_at "2026-01-07T10:00:00Z" "deps: bump foo" >/dev/null
  commit_at "2026-01-07T11:00:00Z" "feat(deps): bump bar" >/dev/null
  git -C "$TMPROOT" tag v1.1.0
}

@test "exact output: merges, duplicates, internal scopes, security and chores are handled" {
  build_release_history

  run bash -c "cd '$TMPROOT' && '$GEN' --output '$OUT'"
  [ "$status" -eq 0 ]
  [[ "$output" == *"Generated $OUT with 2 version entries"* ]]

  expected="[{\"version\":\"1.1.0\",\"date\":\"2026-01-07\",\"features\":[\"add export button\"],\"fixes\":[\"correct login redirect (#102)\"],\"other\":[\"deps: bump foo\",\"feat(deps): bump bar\"],\"security\":[{\"summary\":\"harden session cookies\",\"sha\":\"$SEC_SHA\"}]},{\"version\":\"1.0.0\",\"date\":\"2026-01-03\",\"features\":[\"add dashboard\"],\"fixes\":[],\"other\":[\"chore: init\",\"fix(ci): repair workflow\"],\"security\":[]}]"

  actual="$(normalize "$OUT")"
  [ "$actual" = "$(jq -cS . <<< "$expected")" ]
}

@test "no merge commit subjects leak into any category" {
  build_release_history

  run bash -c "cd '$TMPROOT' && '$GEN' --output '$OUT'"
  [ "$status" -eq 0 ]

  merge_count="$(jq '[ .[] | (.features[], .fixes[], .other[], (.security[].summary)) | select(test("^Merge")) ] | length' "$OUT")"
  [ "$merge_count" -eq 0 ]
}

@test "duplicate subjects differing only by (#NNN) collapse to one item" {
  build_release_history

  run bash -c "cd '$TMPROOT' && '$GEN' --output '$OUT'"
  [ "$status" -eq 0 ]

  # "add export button" appears on both branches; "(#101)" and "(#102)" share a key.
  feature_count="$(jq '.[0].features | length' "$OUT")"
  fix_count="$(jq '.[0].fixes | length' "$OUT")"
  [ "$feature_count" -eq 1 ]
  [ "$fix_count" -eq 1 ]
}

@test "branch-sync merge subjects are dropped even when committed with a fix prefix" {
  commit_at "2026-02-01T12:00:00Z" "fix: real user fix" >/dev/null
  commit_at "2026-02-02T12:00:00Z" "fix: Merge branch 'development'" >/dev/null
  commit_at "2026-02-03T12:00:00Z" "Merge remote-tracking branch 'origin/nightly'" >/dev/null
  commit_at "2026-02-04T12:00:00Z" "fix: Merge conflicts were the cause of the crash" >/dev/null
  git -C "$TMPROOT" tag v4.0.0

  run bash -c "cd '$TMPROOT' && '$GEN' --output '$OUT'"
  [ "$status" -eq 0 ]

  # Any subject leading with "Merge " is dropped, so the guard's no-Merge
  # assertion can never fail on generator output.
  [ "$(jq -c '.[0].fixes' "$OUT")" = '["real user fix"]' ]
  [ "$(jq -c '.[0].other' "$OUT")" = '[]' ]
}

@test "release and promotion chores are dropped from other" {
  build_release_history

  run bash -c "cd '$TMPROOT' && '$GEN' --output '$OUT'"
  [ "$status" -eq 0 ]

  chore_count="$(jq '[ .[].other[] | select(test("release|promote")) ] | length' "$OUT")"
  [ "$chore_count" -eq 0 ]
}

@test "internal scopes move from features and fixes into other" {
  commit_at "2026-02-01T12:00:00Z" "fix(deps): bump lodash" >/dev/null
  commit_at "2026-02-02T12:00:00Z" "fix(e2e): stabilize spec" >/dev/null
  commit_at "2026-02-03T12:00:00Z" "feat(test): add harness" >/dev/null
  commit_at "2026-02-04T12:00:00Z" "fix(workflow): retry job" >/dev/null
  commit_at "2026-02-05T12:00:00Z" "fix(api): close stale session" >/dev/null
  git -C "$TMPROOT" tag v2.0.0

  run bash -c "cd '$TMPROOT' && '$GEN' --output '$OUT'"
  [ "$status" -eq 0 ]

  # Only the non-internal fix reaches user-facing Fixes; the four internal
  # scopes (deps, e2e, test, workflow) land in other with their full subject.
  [ "$(jq -c '.[0].fixes' "$OUT")" = '["close stale session"]' ]
  [ "$(jq '.[0].features | length' "$OUT")" -eq 0 ]
  [ "$(jq '.[0].other | length' "$OUT")" -eq 4 ]
  [ "$(jq -c '.[0].other | sort' "$OUT")" = '["feat(test): add harness","fix(deps): bump lodash","fix(e2e): stabilize spec","fix(workflow): retry job"]' ]
}

@test "(security) commits route only to security and never to fixes" {
  commit_at "2026-02-01T12:00:00Z" "fix(security): harden input handling" >/dev/null
  git -C "$TMPROOT" tag v3.0.0

  run bash -c "cd '$TMPROOT' && '$GEN' --output '$OUT'"
  [ "$status" -eq 0 ]

  [ "$(jq '.[0].fixes | length' "$OUT")" -eq 0 ]
  [ "$(jq '.[0].security | length' "$OUT")" -eq 1 ]
  [ "$(jq -r '.[0].security[0].summary' "$OUT")" = "harden input handling" ]
}

@test "--current-version synthesizes the unreleased head entry dated today (UTC)" {
  build_release_history
  commit_at "2026-01-08T12:00:00Z" "fix(e2e): stabilize spec" >/dev/null
  commit_at "2026-01-08T13:00:00Z" "fix: last minute fix" >/dev/null

  run bash -c "cd '$TMPROOT' && '$GEN' --current-version 1.2.0 --output '$OUT'"
  [ "$status" -eq 0 ]

  [ "$(jq 'length' "$OUT")" -eq 3 ]
  [ "$(jq -r '.[0].version' "$OUT")" = "1.2.0" ]
  [ "$(jq -r '.[0].date' "$OUT")" = "$(date -u +%Y-%m-%d)" ]
  [ "$(jq -c '.[0].fixes' "$OUT")" = '["last minute fix"]' ]
  [ "$(jq -c '.[0].other' "$OUT")" = '["fix(e2e): stabilize spec"]' ]
  [ "$(jq -r '.[1].version' "$OUT")" = "1.1.0" ]
}

@test "without --current-version the unreleased commits are not emitted" {
  build_release_history
  commit_at "2026-01-08T12:00:00Z" "fix: last minute fix" >/dev/null

  run bash -c "cd '$TMPROOT' && '$GEN' --output '$OUT'"
  [ "$status" -eq 0 ]

  [ "$(jq 'length' "$OUT")" -eq 2 ]
  [ "$(jq -r '.[0].version' "$OUT")" = "1.1.0" ]
}

@test "--current-version does not duplicate an entry whose tag already exists" {
  build_release_history

  run bash -c "cd '$TMPROOT' && '$GEN' --current-version 1.1.0 --output '$OUT'"
  [ "$status" -eq 0 ]

  [ "$(jq 'length' "$OUT")" -eq 2 ]
  [ "$(jq -r '[.[].version] | unique | length' "$OUT")" -eq 2 ]
}

@test "zero v* tags (shallow clone) fails loudly and leaves the output untouched" {
  commit_at "2026-01-01T12:00:00Z" "feat: only commit" >/dev/null
  printf 'SENTINEL' > "$OUT"

  run bash -c "cd '$TMPROOT' && '$GEN' --output '$OUT'"
  [ "$status" -eq 1 ]
  [[ "$output" == *"no v* tags found (shallow clone?)"* ]]
  [ "$(cat "$OUT")" = "SENTINEL" ]
}

@test "--require-entries fails when no tags exist" {
  commit_at "2026-01-01T12:00:00Z" "feat: only commit" >/dev/null

  run bash -c "cd '$TMPROOT' && '$GEN' --require-entries --output '$OUT'"
  [ "$status" -eq 1 ]
  [[ "$output" == *"no v* tags found"* ]]
}

@test "--require-entries passes when entries are generated" {
  build_release_history

  run bash -c "cd '$TMPROOT' && '$GEN' --require-entries --output '$OUT'"
  [ "$status" -eq 0 ]
  [ "$(jq 'length' "$OUT")" -eq 2 ]
}

@test "git failures are surfaced instead of hidden" {
  # Run outside any git repository: git's own error must reach the caller.
  NOGIT_DIR="$(mktemp -d)"
  mkdir -p "$NOGIT_DIR/scripts"
  cp "$SOURCE_SCRIPT" "$NOGIT_DIR/scripts/generate-changelog.sh"

  run bash -c "cd '$NOGIT_DIR' && '$NOGIT_DIR/scripts/generate-changelog.sh' --output '$OUT'"
  rm -rf "$NOGIT_DIR"
  [ "$status" -ne 0 ]
  [[ "$output" == *"not a git repository"* ]]
}

@test "invalid --current-version is rejected with usage status" {
  run bash -c "cd '$TMPROOT' && '$GEN' --current-version 1.2 --output '$OUT'"
  [ "$status" -eq 2 ]
  [[ "$output" == *"--current-version must be X.Y.Z"* ]]
}

@test "unknown argument is rejected with usage status" {
  run bash -c "cd '$TMPROOT' && '$GEN' --bogus"
  [ "$status" -eq 2 ]
  [[ "$output" == *"unknown argument: --bogus"* ]]
}
