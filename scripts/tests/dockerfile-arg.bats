#!/usr/bin/env bats
#
# Covers scripts/ci/dockerfile-arg.sh, which resolves a Dockerfile ARG default
# for the release and nightly image builds.

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  SCRIPT_UNDER_TEST="$REPO_ROOT/scripts/ci/dockerfile-arg.sh"

  TMPROOT="$(mktemp -d)"
  DOCKERFILE="$TMPROOT/Dockerfile"
  DIGEST="sha256:$(printf 'a%.0s' {1..64})"
  cat >"$DOCKERFILE" <<EOF
# syntax=docker/dockerfile:1
ARG ALPINE_IMAGE_EXTRA=alpine:0.0.1
ARG ALPINE_IMAGE=alpine:3.24.4@$DIGEST
FROM \${ALPINE_IMAGE}
EOF
}

teardown() {
  rm -rf "$TMPROOT"
}

@test "prints the full pinned reference of the named ARG" {
  run bash "$SCRIPT_UNDER_TEST" --require-digest "$DOCKERFILE" ALPINE_IMAGE
  [ "$status" -eq 0 ]
  [ "$output" = "alpine:3.24.4@$DIGEST" ]
}

@test "matches the exact ARG name, not a longer name with the same prefix" {
  run bash "$SCRIPT_UNDER_TEST" "$DOCKERFILE" ALPINE_IMAGE_EXTRA
  [ "$status" -eq 0 ]
  [ "$output" = "alpine:0.0.1" ]
}

@test "without --require-digest a tag-only default is printed" {
  printf 'ARG ALPINE_IMAGE=alpine:3.24.4\n' >"$DOCKERFILE"
  run bash "$SCRIPT_UNDER_TEST" "$DOCKERFILE" ALPINE_IMAGE
  [ "$status" -eq 0 ]
  [ "$output" = "alpine:3.24.4" ]
}

@test "--require-digest fails when the default has no sha256 digest" {
  printf 'ARG ALPINE_IMAGE=alpine:3.24.4\n' >"$DOCKERFILE"
  run bash "$SCRIPT_UNDER_TEST" --require-digest "$DOCKERFILE" ALPINE_IMAGE
  [ "$status" -eq 1 ]
  [[ "$output" == *"must be pinned by sha256 digest"* ]]
}

@test "--require-digest fails when the digest is malformed" {
  printf 'ARG ALPINE_IMAGE=alpine:3.24.4@sha256:abc\n' >"$DOCKERFILE"
  run bash "$SCRIPT_UNDER_TEST" --require-digest "$DOCKERFILE" ALPINE_IMAGE
  [ "$status" -eq 1 ]
}

@test "fails when the ARG is absent" {
  run bash "$SCRIPT_UNDER_TEST" "$DOCKERFILE" MISSING_ARG
  [ "$status" -eq 1 ]
  [[ "$output" == *"no 'ARG MISSING_ARG=' line"* ]]
}

@test "fails when the ARG has an empty default" {
  printf 'ARG ALPINE_IMAGE=\n' >"$DOCKERFILE"
  run bash "$SCRIPT_UNDER_TEST" "$DOCKERFILE" ALPINE_IMAGE
  [ "$status" -eq 1 ]
  [[ "$output" == *"empty default"* ]]
}

@test "fails when the Dockerfile does not exist" {
  run bash "$SCRIPT_UNDER_TEST" "$TMPROOT/nope" ALPINE_IMAGE
  [ "$status" -eq 1 ]
  [[ "$output" == *"Dockerfile not found"* ]]
}

@test "usage error on a malformed ARG name" {
  run bash "$SCRIPT_UNDER_TEST" "$DOCKERFILE" "BAD-NAME"
  [ "$status" -eq 2 ]
}

@test "usage error when arguments are missing" {
  run bash "$SCRIPT_UNDER_TEST" "$DOCKERFILE"
  [ "$status" -eq 2 ]
}
