#!/usr/bin/env bats
#
# Covers scripts/ci/resolve-build-version.sh: every channel, sha and pr validation,
# a missing or garbled manifest (release fails closed with exit 1, every other
# channel degrades to "dev" with exit 0), the jq and no-jq manifest reads, the
# dirty-tree marker, and the step-summary line.

bats_require_minimum_version 1.5.0

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  SCRIPT_UNDER_TEST="$REPO_ROOT/scripts/ci/resolve-build-version.sh"
  # GitHub Actions exports this; a test must never append to the real job summary.
  unset GITHUB_STEP_SUMMARY

  REPO="$(mktemp -d)"
  cd "$REPO" || return 1
  git init -q -b main
  git config user.name "Test Bot"
  git config user.email "test@example.invalid"
  git config commit.gpgsign false

  MANIFEST="$REPO/.release-please-manifest.json"
  write_manifest "0.46.5"
  echo "notes" > "$REPO/notes.txt"
  git add .release-please-manifest.json notes.txt
  git commit -q -m "chore(main): release 0.46.5"
  SHA="$(git rev-parse HEAD)"
  SHA7="${SHA:0:7}"
}

teardown() {
  cd "$BATS_TEST_DIRNAME" || true
  rm -rf "$REPO"
}

write_manifest() {
  printf '{\n  ".": "%s"\n}\n' "$1" > "$MANIFEST"
}

# resolve ARGS...: runs the resolver against the test manifest. stdout is in
# $output, stderr in $stderr.
resolve() {
  run --separate-stderr "$SCRIPT_UNDER_TEST" --manifest "$MANIFEST" "$@"
}

# no_jq_path: prints a PATH directory that holds only the tools the resolver
# needs (no jq), so the script's sed fallback is exercised on hosts without jq.
# bats' own run helper needs mktemp and cat, so they are included as well.
no_jq_path() {
  local bindir="$BATS_TEST_TMPDIR/nojq-bin" tool src
  mkdir -p "$bindir"
  for tool in env bash git sed head dirname cat mktemp rm mkdir; do
    src="$(command -v "$tool")" || continue
    ln -sf "$src" "$bindir/$tool"
  done
  printf '%s' "$bindir"
}

# --- channels (valid manifest) -----------------------------------------------

@test "release channel emits the bare manifest version with no label" {
  resolve --channel release
  [ "$status" -eq 0 ]
  [ "$output" = "0.46.5" ]
}

@test "release channel output never contains a build-metadata plus sign" {
  resolve --channel release
  [ "$status" -eq 0 ]
  [[ "$output" != *+* ]]
}

@test "main channel emits the manifest version with no label" {
  resolve --channel main
  [ "$status" -eq 0 ]
  [ "$output" = "0.46.5" ]
}

@test "dev channel emits X.Y.Z+dev.<sha7>" {
  resolve --channel dev --sha "$SHA"
  [ "$status" -eq 0 ]
  [ "$output" = "0.46.5+dev.$SHA7" ]
}

@test "pr channel emits X.Y.Z+pr.<N> and ignores any sha" {
  resolve --channel pr --pr 1234 --sha "$SHA"
  [ "$status" -eq 0 ]
  [ "$output" = "0.46.5+pr.1234" ]
}

@test "branch channel emits X.Y.Z+branch.<sha7> and never a branch name" {
  git checkout -q -b "feature/odd+name"
  resolve --channel branch --sha "$SHA"
  [ "$status" -eq 0 ]
  [ "$output" = "0.46.5+branch.$SHA7" ]
}

@test "nightly channel emits X.Y.Z+nightly.<sha7>" {
  resolve --channel nightly --sha "$SHA"
  [ "$status" -eq 0 ]
  [ "$output" = "0.46.5+nightly.$SHA7" ]
}

@test "local channel on a clean tree emits X.Y.Z+local.<sha7>" {
  resolve --channel local
  [ "$status" -eq 0 ]
  [ "$output" = "0.46.5+local.$SHA7" ]
}

@test "local channel reads HEAD from git when no sha is given" {
  resolve --channel local
  [ "$status" -eq 0 ]
  [[ "$output" =~ ^0\.46\.5\+local\.[0-9a-f]{7}$ ]]
  [ "$output" = "0.46.5+local.$SHA7" ]
}

@test "local channel adds .dirty when a tracked file is modified" {
  echo "change" >> "$REPO/notes.txt"
  resolve --channel local
  [ "$status" -eq 0 ]
  [ "$output" = "0.46.5+local.$SHA7.dirty" ]
}

@test "local channel ignores untracked files" {
  echo "scratch" > "$REPO/untracked.txt"
  resolve --channel local
  [ "$status" -eq 0 ]
  [ "$output" = "0.46.5+local.$SHA7" ]
}

@test "local channel degrades to dev outside a git repository" {
  mkdir -p "$BATS_TEST_TMPDIR/nogit"
  cd "$BATS_TEST_TMPDIR/nogit" || return 1
  GIT_CEILING_DIRECTORIES="$BATS_TEST_TMPDIR" resolve --channel local
  [ "$status" -eq 0 ]
  [ "$output" = "dev" ]
  [[ "$stderr" == *WARNING* ]]
}

@test "a short 7-character sha is accepted and kept at 7 characters" {
  resolve --channel dev --sha "$SHA7"
  [ "$status" -eq 0 ]
  [ "$output" = "0.46.5+dev.$SHA7" ]
}

@test "default manifest is the repository manifest next to the script" {
  cd "$BATS_TEST_TMPDIR" || return 1
  run --separate-stderr "$SCRIPT_UNDER_TEST" --channel release
  [ "$status" -eq 0 ]
  [[ "$output" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]
}

# --- input validation (exit 2) ------------------------------------------------

@test "uppercase sha is rejected with exit 2" {
  resolve --channel dev --sha "${SHA^^}"
  [ "$status" -eq 2 ]
  [ -z "$output" ]
}

@test "sha shorter than 7 characters is rejected with exit 2" {
  resolve --channel dev --sha "abcdef"
  [ "$status" -eq 2 ]
}

@test "non-hex sha is rejected with exit 2" {
  resolve --channel branch --sha "zzzzzzz"
  [ "$status" -eq 2 ]
}

@test "sha carrying metadata or shell characters is rejected with exit 2" {
  resolve --channel nightly --sha "abc1234+x"
  [ "$status" -eq 2 ]
  resolve --channel nightly --sha "abc1234;rm"
  [ "$status" -eq 2 ]
}

@test "dev, branch and nightly require a sha" {
  local channel
  for channel in dev branch nightly; do
    resolve --channel "$channel"
    [ "$status" -eq 2 ]
  done
}

@test "pr channel requires a pull request number" {
  resolve --channel pr
  [ "$status" -eq 2 ]
}

@test "malformed pull request numbers are rejected with exit 2" {
  local bad
  for bad in abc 0 012 -1 1.5 "1 2" "1;2"; do
    resolve --channel pr --pr "$bad"
    [ "$status" -eq 2 ]
  done
}

@test "unknown channel is rejected with exit 2" {
  resolve --channel production
  [ "$status" -eq 2 ]
}

@test "missing channel is rejected with exit 2" {
  resolve
  [ "$status" -eq 2 ]
}

@test "unknown argument is rejected with exit 2" {
  resolve --channel main --bogus
  [ "$status" -eq 2 ]
}

@test "flag without a value is rejected with exit 2" {
  resolve --channel
  [ "$status" -eq 2 ]
}

# --- manifest failures ---------------------------------------------------------

@test "release fails closed with exit 1 when the manifest is missing" {
  rm -f "$MANIFEST"
  resolve --channel release
  [ "$status" -eq 1 ]
  [ -z "$output" ]
}

@test "release fails closed with exit 1 on a garbled manifest" {
  printf '{ "." : ' > "$MANIFEST"
  resolve --channel release
  [ "$status" -eq 1 ]
  [ -z "$output" ]
}

@test "release fails closed when the version is not strict X.Y.Z" {
  local bad
  for bad in "0.46" "v0.46.5" "0.46.5-rc.1" "0.46.5+dev" "not-a-version"; do
    write_manifest "$bad"
    resolve --channel release
    [ "$status" -eq 1 ]
    [ -z "$output" ]
  done
}

@test "release fails closed when the manifest has no root key" {
  printf '{"other": "0.46.5"}\n' > "$MANIFEST"
  resolve --channel release
  [ "$status" -eq 1 ]
}

@test "every non-release channel degrades to dev with exit 0 when the manifest is missing" {
  rm -f "$MANIFEST"
  resolve --channel main
  [ "$status" -eq 0 ]
  [ "$output" = "dev" ]
  [[ "$stderr" == *WARNING* ]]

  resolve --channel dev --sha "$SHA"
  [ "$status" -eq 0 ]
  [ "$output" = "dev" ]

  resolve --channel pr --pr 7
  [ "$status" -eq 0 ]
  [ "$output" = "dev" ]

  resolve --channel branch --sha "$SHA"
  [ "$status" -eq 0 ]
  [ "$output" = "dev" ]

  resolve --channel nightly --sha "$SHA"
  [ "$status" -eq 0 ]
  [ "$output" = "dev" ]

  resolve --channel local
  [ "$status" -eq 0 ]
  [ "$output" = "dev" ]
}

@test "a garbled manifest degrades a non-release channel to dev" {
  printf '{ "." : ' > "$MANIFEST"
  resolve --channel main
  [ "$status" -eq 0 ]
  [ "$output" = "dev" ]
}

@test "a non-SemVer manifest version degrades a non-release channel to dev" {
  write_manifest "0.46"
  resolve --channel pr --pr 9
  [ "$status" -eq 0 ]
  [ "$output" = "dev" ]
}

# --- jq and no-jq manifest reads -----------------------------------------------

@test "the no-jq PATH really hides jq" {
  local nojq
  nojq="$(no_jq_path)"
  run env PATH="$nojq" bash -c 'command -v jq'
  [ "$status" -ne 0 ]
}

@test "without jq, release reads the manifest version with the sed fallback" {
  local nojq
  nojq="$(no_jq_path)"
  PATH="$nojq" resolve --channel release
  [ "$status" -eq 0 ]
  [ "$output" = "0.46.5" ]
}

@test "without jq, a single-line manifest is still read" {
  printf '{".": "0.46.5"}' > "$MANIFEST"
  local nojq
  nojq="$(no_jq_path)"
  PATH="$nojq" resolve --channel main
  [ "$status" -eq 0 ]
  [ "$output" = "0.46.5" ]
}

@test "without jq, a missing manifest still fails release closed" {
  rm -f "$MANIFEST"
  local nojq
  nojq="$(no_jq_path)"
  PATH="$nojq" resolve --channel release
  [ "$status" -eq 1 ]
}

@test "without jq, a dev build matches the jq result" {
  resolve --channel dev --sha "$SHA"
  local with_jq="$output"
  local nojq
  nojq="$(no_jq_path)"
  PATH="$nojq" resolve --channel dev --sha "$SHA"
  [ "$status" -eq 0 ]
  [ "$output" = "$with_jq" ]
}

# --- step summary ----------------------------------------------------------------

@test "a valid result is appended to GITHUB_STEP_SUMMARY when it is set" {
  GITHUB_STEP_SUMMARY="$BATS_TEST_TMPDIR/summary.md"
  export GITHUB_STEP_SUMMARY
  resolve --channel pr --pr 42
  [ "$status" -eq 0 ]
  grep -qF "0.46.5+pr.42" "$GITHUB_STEP_SUMMARY"
}

@test "no summary file is created when GITHUB_STEP_SUMMARY is unset" {
  unset GITHUB_STEP_SUMMARY
  resolve --channel main
  [ "$status" -eq 0 ]
  [ ! -e "$BATS_TEST_TMPDIR/summary.md" ]
}
