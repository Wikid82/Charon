#!/usr/bin/env bats
#
# Covers scripts/ci/check-nightly-sequencing.sh, the guard that prevents a
# release-please cut from landing on `main` while `nightly` still has
# unpromoted release-worthy commits dated older than the incoming release
# boundary (the date-ordering stranding bug documented in
# docs/troubleshooting/release-please-skips.md).
#
# Each test builds an isolated fake git repo so history/dates are fully
# controlled, following the pattern used by
# scripts/tests/local-patch-report_baseline.bats.

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  SCRIPT_UNDER_TEST="$REPO_ROOT/scripts/ci/check-nightly-sequencing.sh"

  TMPROOT="$(mktemp -d)"
  git -C "$TMPROOT" init -q -b main
  git -C "$TMPROOT" config user.email "test@example.com"
  git -C "$TMPROOT" config user.name "Test Runner"

  unset TARGET_REF SOURCE_REF GITHUB_OUTPUT
}

teardown() {
  rm -rf "$TMPROOT"
}

# Commits with an explicit, deterministic author+committer date so ordering
# never depends on when the test happens to run.
commit_at() {
  local date="$1" message="$2"
  GIT_AUTHOR_DATE="$date" GIT_COMMITTER_DATE="$date" \
    git -C "$TMPROOT" commit -q --allow-empty -m "$message"
}

run_check() {
  (cd "$TMPROOT" && "$SCRIPT_UNDER_TEST")
}

@test "no divergence between target and source: defer=false" {
  commit_at "2026-09-20T09:00:00" "chore: init"
  git -C "$TMPROOT" branch nightly

  export TARGET_REF="main" SOURCE_REF="nightly"
  run run_check
  [ "$status" -eq 0 ]
  [[ "$output" == *"defer=false"* ]]
  [[ "$output" == *"no release-worthy commits ahead"* ]]
}

@test "nightly ahead with only non-release-worthy commits: defer=false" {
  commit_at "2026-09-20T09:00:00" "chore: init"
  git -C "$TMPROOT" branch nightly
  git -C "$TMPROOT" checkout -q nightly
  commit_at "2026-09-21T09:00:00" "docs: update README"
  commit_at "2026-09-22T09:00:00" "chore: tidy imports"

  export TARGET_REF="main" SOURCE_REF="nightly"
  run run_check
  [ "$status" -eq 0 ]
  [[ "$output" == *"defer=false"* ]]
  [[ "$output" == *"no release-worthy commits ahead"* ]]
}

@test "nightly has release-worthy commits ahead, but all newer than the boundary: defer=false" {
  commit_at "2026-09-20T09:00:00" "chore: init"
  git -C "$TMPROOT" branch nightly
  commit_at "2026-09-21T09:00:00" "fix: hotfix on main"
  git -C "$TMPROOT" checkout -q nightly
  commit_at "2026-09-25T10:00:00" "fix: newer nightly fix"

  export TARGET_REF="main" SOURCE_REF="nightly"
  run run_check
  [ "$status" -eq 0 ]
  [[ "$output" == *"defer=false"* ]]
  [[ "$output" == *"none predate this boundary"* ]]
}

@test "nightly has an older unpromoted fix commit: defer=true, names oldest" {
  commit_at "2026-09-20T09:00:00" "chore: init"
  git -C "$TMPROOT" branch nightly
  git -C "$TMPROOT" checkout -q nightly
  commit_at "2026-09-25T10:00:00" "fix(security): harden input validation in the API layer"
  git -C "$TMPROOT" checkout -q main
  commit_at "2026-09-28T11:44:00" "fix: unrelated hotfix"

  export TARGET_REF="main" SOURCE_REF="nightly"
  run run_check
  [ "$status" -eq 0 ]
  [[ "$output" == *"defer=true"* ]]
  [[ "$output" == *"1 unpromoted release-worthy commit"* ]]
  [[ "$output" == *"fix(security): harden input validation in the API layer"* ]]
  [[ "$output" == *"docs/troubleshooting/release-please-skips.md"* ]]
}

@test "multiple at-risk commits: count is correct and oldest (not newest) is reported" {
  commit_at "2026-09-20T09:00:00" "chore: init"
  git -C "$TMPROOT" branch nightly
  git -C "$TMPROOT" checkout -q nightly
  commit_at "2026-09-23T09:00:00" "feat: add newer feature"
  commit_at "2026-09-21T09:00:00" "fix: oldest buried fix"
  git -C "$TMPROOT" checkout -q main
  commit_at "2026-09-28T11:44:00" "fix: unrelated hotfix"

  export TARGET_REF="main" SOURCE_REF="nightly"
  run run_check
  [ "$status" -eq 0 ]
  [[ "$output" == *"defer=true"* ]]
  [[ "$output" == *"2 unpromoted release-worthy commit"* ]]
  [[ "$output" == *"oldest buried fix"* ]]
}

@test "breaking-change and scoped subjects are recognized" {
  commit_at "2026-09-20T09:00:00" "chore: init"
  git -C "$TMPROOT" branch nightly
  git -C "$TMPROOT" checkout -q nightly
  commit_at "2026-09-21T09:00:00" "fix(api)!: breaking change to auth header"
  git -C "$TMPROOT" checkout -q main
  commit_at "2026-09-28T11:44:00" "fix: unrelated hotfix"

  export TARGET_REF="main" SOURCE_REF="nightly"
  run run_check
  [[ "$output" == *"defer=true"* ]]
  [[ "$output" == *"1 unpromoted release-worthy commit"* ]]
}

@test "deps: commits are treated as release-worthy" {
  commit_at "2026-09-20T09:00:00" "chore: init"
  git -C "$TMPROOT" branch nightly
  git -C "$TMPROOT" checkout -q nightly
  commit_at "2026-09-21T09:00:00" "deps: bump some-lib to 2.0.0"
  git -C "$TMPROOT" checkout -q main
  commit_at "2026-09-28T11:44:00" "fix: unrelated hotfix"

  export TARGET_REF="main" SOURCE_REF="nightly"
  run run_check
  [[ "$output" == *"defer=true"* ]]
}

@test "missing SOURCE_REF (nightly branch not fetched): defer=false, does not fail" {
  commit_at "2026-09-20T09:00:00" "chore: init"

  export TARGET_REF="main" SOURCE_REF="origin/nightly-does-not-exist"
  run run_check
  [ "$status" -eq 0 ]
  [[ "$output" == *"defer=false"* ]]
  [[ "$output" == *"does not resolve"* ]]
}

@test "invalid TARGET_REF is a hard usage error, not a silent defer=false" {
  commit_at "2026-09-20T09:00:00" "chore: init"
  git -C "$TMPROOT" branch nightly

  export TARGET_REF="not-a-real-ref" SOURCE_REF="nightly"
  run run_check
  [ "$status" -eq 2 ]
  [[ "$output" != *"defer="* ]]
}

@test "writes defer and reason to GITHUB_OUTPUT when set" {
  commit_at "2026-09-20T09:00:00" "chore: init"
  git -C "$TMPROOT" branch nightly
  git -C "$TMPROOT" checkout -q nightly
  commit_at "2026-09-25T10:00:00" "fix: buried fix"
  git -C "$TMPROOT" checkout -q main
  commit_at "2026-09-28T11:44:00" "fix: unrelated hotfix"

  OUT_FILE="$TMPROOT/gh_output"
  : >"$OUT_FILE"
  export TARGET_REF="main" SOURCE_REF="nightly" GITHUB_OUTPUT="$OUT_FILE"
  run run_check
  [ "$status" -eq 0 ]

  run cat "$OUT_FILE"
  [[ "$output" == *"defer=true"* ]]
  [[ "$output" == *"reason="* ]]
}
