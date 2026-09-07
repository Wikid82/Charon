#!/usr/bin/env bats
#
# scripts/toolchain-key.sh — determinism & sensitivity (spec §7).
#
#   * stable across a whitespace-only reformat OUTSIDE the two inline stages
#   * changes when a `go get` line INSIDE caddy-inline changes
#   * changes when a tracked ARG default (CADDY_VERSION / CADDY_GEOIP2_VERSION) moves
#   * changes when the digest-pinned golang base moves
#   * changes when .trivyignore changes
#   * fails loudly if stage extraction breaks

load helpers/toolchain_fixture

setup()    { tf_setup; }
teardown() { tf_teardown; }

key_of() { bash "$TF_ROOT/scripts/toolchain-key.sh" "$TF_DF"; }

@test "prints a caddy-crowdsec-<16hex> tag" {
  run key_of
  [ "$status" -eq 0 ]
  [[ "$output" =~ ^caddy-crowdsec-[0-9a-f]{16}$ ]]
}

@test "deterministic: two runs on the same Dockerfile agree" {
  a="$(key_of)"
  b="$(key_of)"
  [ "$a" = "$b" ]
}

@test "stable across a whitespace reformat OUTSIDE the inline stages" {
  before="$(key_of)"
  # Add blank lines / trailing space to the global ARG block and the final stage,
  # none of which is part of caddy-inline / crowdsec-inline.
  printf '\n\n# a new trailing comment\n' >> "$TF_DF"
  sed -i '1a # extra header comment' "$TF_DF"
  after="$(key_of)"
  [ "$before" = "$after" ]
}

@test "changes when a go get line INSIDE caddy-inline changes" {
  before="$(key_of)"
  export TF_CADDY_GET_LINE='    _retry go get golang.org/x/net@v9.9.9; \'
  tf_write_dockerfile
  after="$(key_of)"
  [ "$before" != "$after" ]
}

@test "changes when the CADDY_VERSION default is bumped" {
  before="$(key_of)"
  export TF_CADDY_VERSION=2.11.5
  tf_write_dockerfile
  after="$(key_of)"
  [ "$before" != "$after" ]
}

@test "changes when the CADDY_GEOIP2_VERSION plugin pin is bumped (B4)" {
  before="$(key_of)"
  export TF_GEOIP2_VERSION='v0.0.0-20270101000000-abcdefabcdef'
  tf_write_dockerfile
  after="$(key_of)"
  [ "$before" != "$after" ]
}

@test "changes when the digest-pinned golang base moves (N4)" {
  before="$(key_of)"
  export TF_GOLANG_DIGEST='sha256:1111111111111111111111111111111111111111111111111111111111111111'
  tf_write_dockerfile
  after="$(key_of)"
  [ "$before" != "$after" ]
}

@test "changes when .trivyignore changes" {
  before="$(key_of)"
  printf 'another-cve\n' >> "$TF_ROOT/.trivyignore"
  after="$(key_of)"
  [ "$before" != "$after" ]
}

@test "fails loudly when a required inline stage is missing" {
  sed -i 's/ AS caddy-inline/ AS caddy-scratch/' "$TF_DF"
  run key_of
  [ "$status" -ne 0 ]
  [[ "$output" == *"caddy-inline"* ]]
}

@test "fails loudly when stage extraction is truncated to a stub" {
  # Blank the caddy-inline body down to just the FROM line.
  awk '
    /AS caddy-inline$/ { print; skip=1; next }
    skip && /^FROM / { skip=0 }
    skip { next }
    { print }
  ' "$TF_DF" > "$TF_DF.new" && mv "$TF_DF.new" "$TF_DF"
  run key_of
  [ "$status" -ne 0 ]
  [[ "$output" == *"extraction looks wrong"* || "$output" == *"caddy-inline"* ]]
}
