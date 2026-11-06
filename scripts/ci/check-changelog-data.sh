#!/usr/bin/env bash
# Guards the generated What's New data (backend/internal/changelog/data/changelog.json)
# before it is embedded into a release, development or PR image. A missing or
# malformed file must fail the build loudly instead of shipping an empty or
# broken changelog (see scripts/generate-changelog.sh and issue #1865).
#
# Usage:
#   scripts/ci/check-changelog-data.sh <file> [--expect-newest X.Y.Z]
#
# Asserts:
#   - the file exists and is valid JSON
#   - the top level is an array with at least one entry
#   - every entry has the arrays features, fixes, other and security
#   - every security[].sha is 7-40 lowercase hex characters, the same pattern
#     the backend applies (commitSHAPattern in backend/internal/changelog)
#   - no entry subject starts with "Merge " (merge commits must never leak in)
#   - with --expect-newest, the first (newest) entry's version equals X.Y.Z
#
# Exit codes: 0 pass, 1 assertion failed, 2 usage error.
set -euo pipefail

usage() {
  echo "usage: check-changelog-data.sh <file> [--expect-newest X.Y.Z]" >&2
}

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

FILE=""
EXPECT_NEWEST=""

while [ "$#" -gt 0 ]; do
  case "$1" in
    --expect-newest)
      [ "$#" -ge 2 ] || { echo "ERROR: --expect-newest needs a value" >&2; usage; exit 2; }
      EXPECT_NEWEST="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    -*)
      echo "ERROR: unknown argument: $1" >&2
      usage
      exit 2
      ;;
    *)
      [ -z "$FILE" ] || { echo "ERROR: only one changelog file may be given" >&2; usage; exit 2; }
      FILE="$1"
      shift
      ;;
  esac
done

if [ -z "$FILE" ]; then
  echo "ERROR: a changelog file path is required" >&2
  usage
  exit 2
fi

if [ -n "$EXPECT_NEWEST" ] && ! [[ "$EXPECT_NEWEST" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "ERROR: --expect-newest must be X.Y.Z, got '$EXPECT_NEWEST'" >&2
  exit 2
fi

if ! command -v jq >/dev/null 2>&1; then
  echo "ERROR: jq is required to check changelog data" >&2
  exit 2
fi

[ -f "$FILE" ] || fail "changelog data file not found: $FILE"

jq -e . "$FILE" >/dev/null 2>&1 || fail "$FILE is not valid JSON"

jq -e 'type == "array"' "$FILE" >/dev/null 2>&1 || fail "$FILE top level must be a JSON array"

entry_count="$(jq 'length' "$FILE")"
[ "$entry_count" -ge 1 ] || fail "$FILE has no version entries (empty placeholder or failed generation)"

# Each check collects matches into an array and takes `first` inside jq. Piping
# into `head` instead can SIGPIPE jq, and pipefail would then hide the message.
missing_array="$(jq -r '
  [ to_entries[] as $e
    | ["features", "fixes", "other", "security"][]
    | select(($e.value[.] | type) != "array")
    | "entry \($e.key) (version \($e.value.version // "?")) is missing array \(.)"
  ] | first // empty
' "$FILE")"
[ -z "$missing_array" ] || fail "$missing_array"

bad_sha="$(jq -r '
  [ .[] | .security[]? | .sha | select((tostring | test("^[0-9a-f]{7,40}$")) | not) | tostring ]
  | first // empty
' "$FILE")"
[ -z "$bad_sha" ] || fail "security entry has a non-hex sha: $bad_sha"

merge_subject="$(jq -r '
  [ .[] | (.features[], .fixes[], .other[], (.security[].summary)) | select(test("^Merge ")) ]
  | first // empty
' "$FILE")"
[ -z "$merge_subject" ] || fail "merge commit subject leaked into changelog data: $merge_subject"

if [ -n "$EXPECT_NEWEST" ]; then
  newest_version="$(jq -r '.[0].version' "$FILE")"
  [ "$newest_version" = "$EXPECT_NEWEST" ] \
    || fail "newest entry is '$newest_version', expected '$EXPECT_NEWEST'"
fi

echo "OK: $FILE is valid changelog data ($entry_count version entries, newest $(jq -r '.[0].version' "$FILE"))"
