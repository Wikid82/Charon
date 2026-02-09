#!/usr/bin/env bats

setup() {
  TMPREPO=$(mktemp -d)
  cd "$TMPREPO"
  git init -q
  # Set local git identity so commits succeed in CI
  git config user.email "test@example.com"
  git config user.name "Test Runner"
  # Create a commit in an unrelated path
  mkdir -p other/dir
  echo hello > other/dir/file.txt
  git add other/dir/file.txt && git commit -m 'add unrelated file' -q
  # Create an annotated tag
  git tag -a v0.3.0 -m "annotated tag v0.3.0"
}

teardown() {
  rm -rf "$TMPREPO"
}

REPO_ROOT=$(cd "$BATS_TEST_DIRNAME/../../../" && pwd)
SCRIPT="$REPO_ROOT/scripts/ci/dry_run_history_rewrite.sh"

@test "dry_run script ignores tag-only objects and passes" {
  run bash "$SCRIPT" --paths 'backend/codeql-db' --strip-size 50
  [ "$status" -eq 0 ]
  [[ "$output" == *"DRY-RUN OK"* ]]
}
