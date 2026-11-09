#!/usr/bin/env bats
#
# Covers scripts/ci/detect-release-commit.sh. Each test builds a throwaway git
# repo so the release rules (first-parent manifest diff, SemVer increase, tag
# at HEAD) are exercised against real commit graphs.

bats_require_minimum_version 1.5.0

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  SCRIPT_UNDER_TEST="$REPO_ROOT/scripts/ci/detect-release-commit.sh"

  REPO="$(mktemp -d)"
  cd "$REPO" || return 1
  git init -q -b main
  git config user.name "Test Bot"
  git config user.email "test@example.invalid"
  git config commit.gpgsign false
  git config tag.gpgsign false

  write_manifest "0.46.4"
  git add .release-please-manifest.json
  git commit -q -m "chore(main): init"
  git tag -a v0.46.4 -m "0.46.4"
  echo "feature" > app.txt
  git add app.txt
  git commit -q -m "feat: add a feature"
}

teardown() {
  cd "$BATS_TEST_DIRNAME" || true
  rm -rf "$REPO"
}

write_manifest() {
  printf '{\n  ".": "%s"\n}\n' "$1" > .release-please-manifest.json
}

# Writes the given manifest version and commits it on the current branch.
commit_manifest() {
  write_manifest "$1"
  git add .release-please-manifest.json
  git commit -q -m "$2"
}

# stdout only: the workflow appends stdout to $GITHUB_OUTPUT, so diagnostics
# on stderr must never leak into the two output lines asserted below.
run_detect() {
  run --separate-stderr "$SCRIPT_UNDER_TEST" --event "${1:-push}" --ref "${2:-refs/heads/main}"
}

@test "release-please two-parent merge on main is a release" {
  git checkout -q -b release-please--branches--main
  commit_manifest "0.46.5" "chore: release 0.46.5"
  git checkout -q main
  git merge -q --no-ff -m "chore(main): release 0.46.5" release-please--branches--main

  # The merge has two parents, so the release bump is only visible vs HEAD^1.
  [ "$(git rev-list --parents -n1 HEAD | wc -w)" -eq 3 ]

  run_detect
  [ "$status" -eq 0 ]
  [ "${lines[0]}" = "is_release=true" ]
  [ "${lines[1]}" = "version=0.46.5" ]
}

@test "squash-merged release commit on main is a release" {
  commit_manifest "0.46.5" "chore(main): release 0.46.5 (#42)"

  run_detect
  [ "$status" -eq 0 ]
  [ "${lines[0]}" = "is_release=true" ]
  [ "${lines[1]}" = "version=0.46.5" ]
}

@test "weekly promotion merge without a manifest change is not a release" {
  git checkout -q -b nightly
  echo "nightly work" > nightly.txt
  git add nightly.txt
  git commit -q -m "fix: nightly fix"
  git checkout -q main
  git merge -q --no-ff -m "chore: promote nightly to main" nightly

  run_detect
  [ "$status" -eq 0 ]
  [ "${lines[0]}" = "is_release=false" ]
  [ "${lines[1]}" = "version=" ]
}

@test "manifest edit that does not increase the version is not a release" {
  commit_manifest "0.46.3" "chore: roll back manifest"

  run_detect
  [ "$status" -eq 0 ]
  [ "${lines[0]}" = "is_release=false" ]
  [ "${lines[1]}" = "version=" ]
}

@test "manifest with a non-SemVer version is not a release" {
  commit_manifest "0.46.5-rc.1" "chore(main): release 0.46.5-rc.1"

  run_detect
  [ "$status" -eq 0 ]
  [ "${lines[0]}" = "is_release=false" ]
  [ "${lines[1]}" = "version=" ]
}

@test "refs/heads/development is never a release even with a manifest bump" {
  commit_manifest "0.46.5" "chore(main): release 0.46.5"

  run_detect push refs/heads/development
  [ "$status" -eq 0 ]
  [ "${lines[0]}" = "is_release=false" ]
  [ "${lines[1]}" = "version=" ]
}

@test "pull_request event on main is never a release" {
  commit_manifest "0.46.5" "chore(main): release 0.46.5"

  run_detect pull_request refs/heads/main
  [ "$status" -eq 0 ]
  [ "${lines[0]}" = "is_release=false" ]
  [ "${lines[1]}" = "version=" ]
}

@test "existing tag on another commit is not a release" {
  git tag v0.46.5 HEAD
  commit_manifest "0.46.5" "chore(main): release 0.46.5"

  run_detect
  [ "$status" -eq 0 ]
  [ "${lines[0]}" = "is_release=false" ]
  [ "${lines[1]}" = "version=" ]
}

@test "tag vX.Y.Z pointing at HEAD is a release" {
  commit_manifest "0.46.5" "chore(main): release 0.46.5"
  git tag v0.46.5

  run_detect
  [ "$status" -eq 0 ]
  [ "${lines[0]}" = "is_release=true" ]
  [ "${lines[1]}" = "version=0.46.5" ]
}

@test "root commit with no first parent is not a release and does not fail" {
  ROOT_REPO="$(mktemp -d)"
  cd "$ROOT_REPO" || return 1
  git init -q -b main
  git config user.name "Test Bot"
  git config user.email "test@example.invalid"
  write_manifest "0.46.5"
  git add .release-please-manifest.json
  git commit -q -m "chore(main): release 0.46.5"

  run_detect
  local rc=$status
  local out=("${lines[@]}")
  cd "$BATS_TEST_DIRNAME" || true
  rm -rf "$ROOT_REPO"
  [ "$rc" -eq 0 ]
  [ "${out[0]}" = "is_release=false" ]
  [ "${out[1]}" = "version=" ]
}

# Missed-release diagnostic: a main push that changes the manifest version but is
# not a release must warn on stderr and still decide "not a release" with exit 0.

@test "a manifest version change on main that is not a release warns and does not fail" {
  commit_manifest "0.46.3" "chore: roll back manifest"

  run_detect
  [ "$status" -eq 0 ]
  [ "${lines[0]}" = "is_release=false" ]
  [ "${lines[1]}" = "version=" ]
  [[ "$stderr" == *"::warning::Manifest version changed to 0.46.3"* ]]
}

@test "an unchanged manifest version on main does not warn" {
  git checkout -q -b nightly
  echo "nightly work" > nightly.txt
  git add nightly.txt
  git commit -q -m "fix: nightly fix"
  git checkout -q main
  git merge -q --no-ff -m "chore: promote nightly to main" nightly

  run_detect
  [ "$status" -eq 0 ]
  [[ "$stderr" != *"::warning::"* ]]
}

@test "a manifest version change on a pull_request does not warn" {
  commit_manifest "0.46.3" "chore: roll back manifest"

  run_detect pull_request refs/heads/main
  [ "$status" -eq 0 ]
  [[ "$stderr" != *"::warning::"* ]]
}

@test "a manifest version change whose tag was cut on another commit does not warn" {
  git tag v0.46.5 HEAD
  commit_manifest "0.46.5" "chore(main): release 0.46.5"

  run_detect
  [ "$status" -eq 0 ]
  [ "${lines[0]}" = "is_release=false" ]
  [[ "$stderr" != *"::warning::"* ]]
}

@test "missing --event exits with a usage error" {
  run "$SCRIPT_UNDER_TEST" --ref refs/heads/main
  [ "$status" -eq 2 ]
}
