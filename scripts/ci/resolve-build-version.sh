#!/usr/bin/env bash
# Resolves the build-time VERSION for a Charon image (docs/plans/current_spec.md
# §4.10). The VERSION build-arg is what the binary reports on /api/v1/health, so
# every build channel is decided here rather than in each workflow.
#
# Usage:
#   resolve-build-version.sh --channel <release|main|dev|nightly|pr|branch|local>
#                            [--sha S] [--pr N] [--manifest P]
#
# stdout: the resolved version and nothing else. Diagnostics go to stderr. When
# $GITHUB_STEP_SUMMARY is set, a one-line summary is appended to it.
#
# Channels (the base X.Y.Z is the release-please manifest "." version):
#   release   X.Y.Z                      release commit (is_release, decided by the caller)
#   main      X.Y.Z                      non-release main build; no label, main is the stable line
#   dev       X.Y.Z+dev.<sha7>           development builds
#   pr        X.Y.Z+pr.<N>               pull request builds
#   branch    X.Y.Z+branch.<sha7>        every other ref; the branch name is never embedded
#   nightly   X.Y.Z+nightly.<sha7>       nightly builds (sha = the checked-out commit)
#   local     X.Y.Z+local.<sha7>[.dirty] Makefile builds from a working tree
#
# Failure policy:
#   - A missing or invalid base version gives "dev" and exit 0, except the release
#     channel, which exits 1 (fails closed).
#   - Bad input (unknown channel, malformed --sha or --pr, a required value missing)
#     exits 2.
#
# Exit codes: 0 resolved, 1 release channel without a valid base, 2 usage or bad input.
set -uo pipefail

readonly SEMVER_RE='^[0-9]+\.[0-9]+\.[0-9]+$'
readonly SHA_RE='^[0-9a-f]{7,40}$'
readonly PR_RE='^[1-9][0-9]*$'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MANIFEST="${SCRIPT_DIR}/../../.release-please-manifest.json"
CHANNEL=""
SHA=""
PR=""

usage() {
  echo "usage: resolve-build-version.sh --channel <release|main|dev|nightly|pr|branch|local> [--sha S] [--pr N] [--manifest P]" >&2
}

die_usage() {
  echo "ERROR: $*" >&2
  usage
  exit 2
}

# Requires a value after a flag, so a missing value is a usage error rather than
# an unbound-variable crash under set -u.
need_value() {
  [ "$#" -ge 2 ] && [ -n "$2" ] || die_usage "$1 needs a value"
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --channel) need_value "$@"; CHANNEL="$2"; shift 2 ;;
    --sha) need_value "$@"; SHA="$2"; shift 2 ;;
    --pr) need_value "$@"; PR="$2"; shift 2 ;;
    --manifest) need_value "$@"; MANIFEST="$2"; shift 2 ;;
    -h|--help) usage; exit 0 ;;
    *) die_usage "unknown argument: $1" ;;
  esac
done

case "$CHANNEL" in
  release|main|dev|nightly|pr|branch|local) ;;
  "") die_usage "--channel is required" ;;
  *) die_usage "unknown channel: $CHANNEL" ;;
esac

if [ -n "$SHA" ] && [[ ! "$SHA" =~ $SHA_RE ]]; then
  die_usage "--sha must be 7 to 40 lowercase hex characters"
fi
if [ -n "$PR" ] && [[ ! "$PR" =~ $PR_RE ]]; then
  die_usage "--pr must be a positive integer without leading zeros"
fi

# Channels that embed a commit need one; local may read it from git instead.
case "$CHANNEL" in
  dev|branch|nightly)
    [ -n "$SHA" ] || die_usage "--sha is required for the $CHANNEL channel"
    ;;
  pr)
    [ -n "$PR" ] || die_usage "--pr is required for the pr channel"
    ;;
esac

# read_base_version MANIFEST: prints the "." version, or nothing. Uses jq when it
# is installed and a strict sed match otherwise, so the script has no hard jq
# dependency. Returns non-zero when the file is unreadable or jq rejects it.
read_base_version() {
  local manifest="$1" value=""
  [ -r "$manifest" ] || return 1
  if command -v jq >/dev/null 2>&1; then
    value="$(jq -r 'if type == "object" then (.["."] // "") else "" end' "$manifest" 2>/dev/null)" || return 1
  else
    value="$(sed -n 's/.*"\.":[[:space:]]*"\([^"]*\)".*/\1/p' "$manifest" | head -n 1)"
  fi
  printf '%s' "$value"
}

BASE=""
if base_raw="$(read_base_version "$MANIFEST")" && [[ "$base_raw" =~ $SEMVER_RE ]]; then
  BASE="$base_raw"
fi

emit() {
  printf '%s\n' "$1"
  if [ -n "${GITHUB_STEP_SUMMARY:-}" ]; then
    printf '%s\n' "- Build version (${CHANNEL} channel): ${1}" >> "$GITHUB_STEP_SUMMARY"
  fi
  exit 0
}

# degrade REASON: the non-release fallback. Never fails the build.
degrade() {
  echo "WARNING: $1; resolving to the unversioned value 'dev'." >&2
  emit "dev"
}

if [ "$CHANNEL" = "release" ]; then
  if [ -z "$BASE" ]; then
    echo "ERROR: release channel needs a valid X.Y.Z manifest version in ${MANIFEST}" >&2
    exit 1
  fi
  emit "$BASE"
fi

if [ -z "$BASE" ]; then
  degrade "no valid X.Y.Z base version in ${MANIFEST}"
fi

case "$CHANNEL" in
  main)
    emit "$BASE"
    ;;
  dev)
    emit "${BASE}+dev.${SHA:0:7}"
    ;;
  branch)
    emit "${BASE}+branch.${SHA:0:7}"
    ;;
  nightly)
    emit "${BASE}+nightly.${SHA:0:7}"
    ;;
  pr)
    emit "${BASE}+pr.${PR}"
    ;;
  local)
    if [ -z "$SHA" ]; then
      SHA="$(git rev-parse -q --verify HEAD 2>/dev/null || true)"
    fi
    if [ -z "$SHA" ] || [[ ! "$SHA" =~ $SHA_RE ]]; then
      degrade "no commit available for a local build"
    fi
    # Tracked changes only, matching `git describe --dirty`: untracked files are
    # not part of the compiled tree and are ignored here.
    dirty=""
    if [ -n "$(git status --porcelain --untracked-files=no 2>/dev/null)" ]; then
      dirty=".dirty"
    fi
    emit "${BASE}+local.${SHA:0:7}${dirty}"
    ;;
esac
