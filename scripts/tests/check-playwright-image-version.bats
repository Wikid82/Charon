#!/usr/bin/env bats
#
# Covers scripts/ci/check-playwright-image-version.sh, which keeps the Playwright
# container image in lockstep with the installed @playwright/test package.

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  SCRIPT_UNDER_TEST="$REPO_ROOT/scripts/ci/check-playwright-image-version.sh"

  TMPROOT="$(mktemp -d)"
  PKG_JSON="$TMPROOT/package.json"
  echo '{"name":"@playwright/test","version":"1.64.1"}' >"$PKG_JSON"
  DIGEST="sha256:$(printf 'a%.0s' {1..64})"
}

teardown() {
  rm -rf "$TMPROOT"
}

@test "passes when the image tag matches the installed version" {
  run bash "$SCRIPT_UNDER_TEST" "mcr.microsoft.com/playwright:v1.64.1-noble@$DIGEST" "$PKG_JSON"
  [ "$status" -eq 0 ]
  [[ "$output" == *"matches installed @playwright/test 1.64.1"* ]]
}

@test "fails when the image version drifts from the installed version" {
  run bash "$SCRIPT_UNDER_TEST" "mcr.microsoft.com/playwright:v1.63.0-noble@$DIGEST" "$PKG_JSON"
  [ "$status" -eq 1 ]
  [[ "$output" == *"does not match installed @playwright/test (1.64.1)"* ]]
}

@test "fails when the image is not pinned by digest" {
  run bash "$SCRIPT_UNDER_TEST" "mcr.microsoft.com/playwright:v1.64.1-noble" "$PKG_JSON"
  [ "$status" -eq 1 ]
  [[ "$output" == *"must be pinned by digest"* ]]
}

@test "fails when the tag has no distro suffix" {
  run bash "$SCRIPT_UNDER_TEST" "mcr.microsoft.com/playwright:v1.64.1@$DIGEST" "$PKG_JSON"
  [ "$status" -eq 1 ]
  [[ "$output" == *"must look like vX.Y.Z-<distro>"* ]]
}

@test "fails when the package.json is missing" {
  run bash "$SCRIPT_UNDER_TEST" "mcr.microsoft.com/playwright:v1.64.1-noble@$DIGEST" "$TMPROOT/absent.json"
  [ "$status" -eq 1 ]
  [[ "$output" == *"run 'npm ci' before the Playwright container"* ]]
}
