#!/usr/bin/env bash
# Decides whether the commit being built is a release commit, so the Docker
# build can embed the released version's What's New data (issue #1865).
#
# A release commit is one where release-please (or a squash of it) bumped the
# manifest on main. The release tag does not exist yet when the image build
# starts, so the version is read from the manifest rather than from a tag.
#
# Usage:
#   detect-release-commit.sh --event <TRIGGER_EVENT> --ref <TRIGGER_REF> [--manifest PATH]
#
# stdout (two lines, appended to $GITHUB_OUTPUT by the workflow):
#   is_release=<true|false>
#   version=<X.Y.Z or empty>
#
# is_release=true only when ALL of these hold:
#   1. the event is not pull_request
#   2. the ref is refs/heads/main
#   3. the manifest changed between the first parent (HEAD^1) and HEAD
#   4. the manifest "." version at HEAD matches X.Y.Z and is SemVer-greater
#      than the version at HEAD^1
#   5. if tag vX.Y.Z already exists, it points at HEAD
#
# Anything else is "not a release". The script never fails the build for that
# reason; the safe direction is "not a release" (no release tags, plain
# manifest version, and the data guard still runs).
#
# On a push to main that changes the manifest version but is not detected as a
# release, a "::warning::" annotation is written to stderr so a missed release
# is visible in the logs. The decision itself is unchanged and the exit is 0.
#
# Exit codes: 0 decision emitted, 2 usage error.
set -uo pipefail

MANIFEST=".release-please-manifest.json"
EVENT=""
REF=""

usage() {
  echo "usage: detect-release-commit.sh --event <event> --ref <ref> [--manifest PATH]" >&2
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --event)
      [ "$#" -ge 2 ] || { echo "ERROR: --event needs a value" >&2; usage; exit 2; }
      EVENT="$2"
      shift 2
      ;;
    --ref)
      [ "$#" -ge 2 ] || { echo "ERROR: --ref needs a value" >&2; usage; exit 2; }
      REF="$2"
      shift 2
      ;;
    --manifest)
      [ "$#" -ge 2 ] || { echo "ERROR: --manifest needs a path" >&2; usage; exit 2; }
      MANIFEST="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "ERROR: unknown argument: $1" >&2
      usage
      exit 2
      ;;
  esac
done

if [ -z "$EVENT" ] || [ -z "$REF" ]; then
  echo "ERROR: --event and --ref are required" >&2
  usage
  exit 2
fi

emit() {
  printf 'is_release=%s\nversion=%s\n' "$1" "$2"
}

SEMVER_RE='^[0-9]+\.[0-9]+\.[0-9]+$'

# manifest_version REV: prints the "." version from the manifest at REV, or
# nothing when the manifest is missing or unreadable.
manifest_version() {
  git show "$1:$MANIFEST" 2>/dev/null | jq -r '.["."] // empty' 2>/dev/null || true
}

# warn_if_release_may_be_missed: a push to main that changes the manifest
# version but was not detected as a release can leave the data without the
# release's notes (users anchored at the bumped version). Only a warning: the
# build must not fail, and the safe default stays "not a release".
warn_if_release_may_be_missed() {
  [ "$EVENT" != pull_request ] && [ "$REF" = refs/heads/main ] || return 0
  git rev-parse -q --verify 'HEAD^1' >/dev/null || return 0

  local head_version parent_version tag tagged_commit
  head_version="$(manifest_version HEAD)"
  parent_version="$(manifest_version HEAD^1)"
  [[ "$head_version" =~ $SEMVER_RE ]] || return 0
  [ "$head_version" != "$parent_version" ] || return 0

  # A tag for this version on another commit means the release was cut there.
  tag="v${head_version}"
  if git rev-parse -q --verify "refs/tags/${tag}" >/dev/null; then
    tagged_commit="$(git rev-parse "refs/tags/${tag}^{commit}")"
    [ "$tagged_commit" = "$(git rev-parse HEAD)" ] || return 0
  fi
  echo "::warning::Manifest version changed to ${head_version} on main (was '${parent_version:-none}') but this commit was not detected as a release. A release may have been missed; check that ${tag} exists and that the changelog data includes ${head_version}." >&2
}

not_release() {
  echo "Not a release commit: $*" >&2
  warn_if_release_may_be_missed
  emit false ""
  exit 0
}

# semver_gt NEW OLD: succeeds when NEW is strictly greater than OLD. Both are
# X.Y.Z with digits only (validated by the caller). 10# forces decimal so that
# components with leading zeros are not parsed as octal.
semver_gt() {
  local new_major new_minor new_patch old_major old_minor old_patch
  IFS=. read -r new_major new_minor new_patch <<<"$1"
  IFS=. read -r old_major old_minor old_patch <<<"$2"

  if (( 10#$new_major != 10#$old_major )); then
    (( 10#$new_major > 10#$old_major ))
  elif (( 10#$new_minor != 10#$old_minor )); then
    (( 10#$new_minor > 10#$old_minor ))
  else
    (( 10#$new_patch > 10#$old_patch ))
  fi
}

if [ "$EVENT" = "pull_request" ]; then
  not_release "pull_request events never publish a release"
fi

if [ "$REF" != "refs/heads/main" ]; then
  not_release "ref '$REF' is not refs/heads/main"
fi

if ! git rev-parse -q --verify 'HEAD^1' >/dev/null; then
  not_release "HEAD has no first parent (root commit)"
fi

changed_files="$(git diff --name-only HEAD^1 HEAD)"
if ! grep -qxF -- "$MANIFEST" <<<"$changed_files"; then
  not_release "$MANIFEST did not change in the first-parent diff"
fi

new_version="$(manifest_version HEAD)"
if ! [[ "$new_version" =~ $SEMVER_RE ]]; then
  not_release "manifest version '${new_version}' at HEAD is not X.Y.Z"
fi

old_version="$(manifest_version HEAD^1)"
if ! [[ "$old_version" =~ $SEMVER_RE ]]; then
  not_release "manifest version '${old_version}' at HEAD^1 is not X.Y.Z"
fi

if ! semver_gt "$new_version" "$old_version"; then
  not_release "version did not increase ($old_version -> $new_version)"
fi

tag="v${new_version}"
if git rev-parse -q --verify "refs/tags/${tag}" >/dev/null; then
  tagged_commit="$(git rev-parse "refs/tags/${tag}^{commit}")"
  head_commit="$(git rev-parse HEAD)"
  if [ "$tagged_commit" != "$head_commit" ]; then
    not_release "tag ${tag} already points at ${tagged_commit}, not HEAD"
  fi
fi

echo "Release commit detected: ${new_version} (${old_version} -> ${new_version})" >&2
emit true "$new_version"
