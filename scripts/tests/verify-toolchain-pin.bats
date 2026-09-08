#!/usr/bin/env bats
#
# scripts/verify-toolchain-pin.sh — failure-closed freshness guard (spec §7, B7).
#
#   matching pin (fork)                          -> exit 0 (::warning::)
#   mismatched tag                               -> exit 1 (actionable)
#   same-repo + regctl absent                    -> exit 1
#   same-repo + GHCR_READ_TOKEN unset            -> exit 1
#   same-repo + GHCR digest != pinned digest     -> exit 1
#   same-repo + GHCR digest == pinned digest     -> exit 0

load helpers/toolchain_fixture

setup() {
  tf_setup
  # Pin the Dockerfile tag to the real recomputed key so tag-equality passes
  # unless a test deliberately breaks it.
  KEY="$(bash "$TF_ROOT/scripts/toolchain-key.sh" "$TF_DF")"
  GOOD_DIGEST="sha256:abc123abc123abc123abc123abc123abc123abc123abc123abc123abc123abcd0"
  sed -i "s|^ARG CHARON_TOOLCHAIN_TAG=.*|ARG CHARON_TOOLCHAIN_TAG=${KEY}|" "$TF_DF"
  sed -i "s|^ARG CHARON_TOOLCHAIN_DIGEST=.*|ARG CHARON_TOOLCHAIN_DIGEST=${GOOD_DIGEST}|" "$TF_DF"
}
teardown() { tf_teardown; }

verify() { bash "$TF_ROOT/scripts/verify-toolchain-pin.sh" "$TF_DF"; }

@test "fork PR with a matching tag: exit 0 + warning, no digest check" {
  export GITHUB_EVENT_NAME=pull_request
  export GITHUB_REPOSITORY=wikid82/Charon
  export GITHUB_EVENT_PULL_REQUEST_HEAD_REPO_FULL_NAME=contributor/Charon
  run verify
  [ "$status" -eq 0 ]
  [[ "$output" == *"::warning::Fork PR"* ]]
}

@test "mismatched tag: exit 1 with actionable message (any trust level)" {
  sed -i "s|^ARG CHARON_TOOLCHAIN_TAG=.*|ARG CHARON_TOOLCHAIN_TAG=caddy-crowdsec-deadbeefdeadbeef|" "$TF_DF"
  export GITHUB_EVENT_NAME=pull_request
  export GITHUB_REPOSITORY=wikid82/Charon
  export GITHUB_EVENT_PULL_REQUEST_HEAD_REPO_FULL_NAME=contributor/Charon
  run verify
  [ "$status" -eq 1 ]
  [[ "$output" == *"recipe/pins changed"* ]]
  [[ "$output" == *"Toolchain Image"* ]]
}

@test "same-repo push + regctl absent: exit 1 (failure-closed)" {
  export GITHUB_EVENT_NAME=push
  export GITHUB_REPOSITORY=wikid82/Charon
  export GHCR_READ_TOKEN=tok
  run env PATH="$(tf_min_path)" bash "$TF_ROOT/scripts/verify-toolchain-pin.sh" "$TF_DF"
  [ "$status" -eq 1 ]
  [[ "$output" == *"regctl missing"* ]]
}

@test "same-repo push + GHCR_READ_TOKEN unset: exit 1 (failure-closed)" {
  tf_stub_regctl "$GOOD_DIGEST"
  export GITHUB_EVENT_NAME=push
  export GITHUB_REPOSITORY=wikid82/Charon
  unset GHCR_READ_TOKEN
  run verify
  [ "$status" -eq 1 ]
  [[ "$output" == *"GHCR_READ_TOKEN unset"* ]]
}

@test "same-repo PR + GHCR digest disagrees with the pinned digest: exit 1" {
  tf_stub_regctl "sha256:0000000000000000000000000000000000000000000000000000000000000bad"
  export GITHUB_EVENT_NAME=pull_request
  export GITHUB_REPOSITORY=wikid82/Charon
  export GITHUB_EVENT_PULL_REQUEST_HEAD_REPO_FULL_NAME=wikid82/Charon
  export GHCR_READ_TOKEN=tok
  run verify
  [ "$status" -eq 1 ]
  [[ "$output" == *"hand-edited or stale"* ]]
}

@test "same-repo PR + GHCR digest matches the pinned digest: exit 0" {
  tf_stub_regctl "$GOOD_DIGEST"
  export GITHUB_EVENT_NAME=pull_request
  export GITHUB_REPOSITORY=wikid82/Charon
  export GITHUB_EVENT_PULL_REQUEST_HEAD_REPO_FULL_NAME=wikid82/Charon
  export GHCR_READ_TOKEN=tok
  run verify
  [ "$status" -eq 0 ]
  [[ "$output" == *"verified (same-repo)"* ]]
}

@test "workflow_dispatch is treated as trusted same-repo (failure-closed)" {
  export GITHUB_EVENT_NAME=workflow_dispatch
  export GITHUB_REPOSITORY=wikid82/Charon
  export GHCR_READ_TOKEN=tok
  run env PATH="$(tf_min_path)" bash "$TF_ROOT/scripts/verify-toolchain-pin.sh" "$TF_DF"
  [ "$status" -eq 1 ]
  [[ "$output" == *"regctl missing"* ]]
}
