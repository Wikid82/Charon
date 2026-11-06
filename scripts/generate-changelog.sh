#!/usr/bin/env bash
# Regenerates backend/internal/changelog/data/changelog.json from
# conventional-commit git history, one entry per `v*` tag. Run in CI
# immediately before the release build so go:embed picks up fresh data —
# see .github/workflows/nightly-build.yml. Never commit the generated
# output back to the repo; the committed file must stay the `[]` placeholder.
#
# Usage:
#   generate-changelog.sh [--current-version X.Y.Z] [--require-entries] [--output PATH]
#
#   --current-version X.Y.Z  Release channel only. When no `vX.Y.Z` tag exists
#                            yet, synthesize one entry for the commits between
#                            the newest `v*` tag and HEAD, dated today (UTC).
#                            Closes the race where the release being built is
#                            not tagged yet and would otherwise be missing.
#   --require-entries        Exit 1 when the generated data has no entries.
#   --output PATH            Write here instead of the embedded data file.
#                            Relative paths resolve against the repo root.
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

OUTPUT="backend/internal/changelog/data/changelog.json"
CURRENT_VERSION=""
REQUIRE_ENTRIES=false

usage() {
  echo "usage: generate-changelog.sh [--current-version X.Y.Z] [--require-entries] [--output PATH]" >&2
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    --current-version)
      [ "$#" -ge 2 ] || { echo "ERROR: --current-version needs a value" >&2; usage; exit 2; }
      CURRENT_VERSION="$2"
      shift 2
      ;;
    --require-entries)
      REQUIRE_ENTRIES=true
      shift
      ;;
    --output)
      [ "$#" -ge 2 ] || { echo "ERROR: --output needs a path" >&2; usage; exit 2; }
      OUTPUT="$2"
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

if [ -n "$CURRENT_VERSION" ] && ! [[ "$CURRENT_VERSION" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]]; then
  echo "ERROR: --current-version must be X.Y.Z, got '$CURRENT_VERSION'" >&2
  exit 2
fi

if ! command -v jq >/dev/null 2>&1; then
  echo "ERROR: jq is required to generate changelog data" >&2
  exit 1
fi

# jq program that categorizes one version's raw commits (fed in via stdin as
# newline-delimited "<sha>\x1f<subject>" pairs, never as a command-line
# argument — some tag-to-tag ranges in real history span thousands of
# commits, which blows past the OS argv size limit if passed with
# --argjson/--arg instead). \x1f (ASCII unit separator) delimits sha from
# subject since it's effectively never present in a real commit subject,
# unlike more common punctuation. Breaking-change shorthand (feat!:,
# feat(scope)!:) is treated as a regular feature/fix, not demoted to
# "Other" — it's still user-facing. $v/$d are small scalars, safe as
# --arg.
#
# Selection and categorization rules:
#   - Release/promotion chores and merge commits are dropped (they describe
#     the release process or branch sync, not a user-visible change). Merge
#     subjects are matched with or without a conventional prefix because some
#     branch syncs are committed as single-parent "fix: Merge branch ..."
#     commits, which --no-merges alone would not filter out. The match is
#     deliberately broad (any leading "Merge ") so it agrees exactly with the
#     guard in scripts/ci/check-changelog-data.sh, which rejects such subjects.
#   - Duplicate subjects are dropped, keeping the first occurrence, where
#     the subject ignores a trailing "(#NNN)". The same change is often
#     reachable through a squash commit plus its branch commits, and each
#     carries a different PR number.
#   - `(security)`-scoped feat/fix commits are checked FIRST and routed
#     exclusively into a new `security` array (never duplicated into
#     features/fixes) — see
#     docs/superpowers/specs/2026-07-29-security-changelog-category-design.md.
#   - feat/fix commits whose scope is internal (CI, dependencies, tests,
#     docs, ...) move to `other`. They are not user-facing "Fixes".
read -r -d '' CATEGORIZE_JQ <<'JQ_EOF' || true
def release_chore_re: "^chore\\(main\\): release|^chore: promote nightly to main";
def merge_re: "^([a-z]+(\\([^)]*\\))?!?:\\s*)?Merge ";
def security_re: "^(feat|fix)\\(security\\)!?:";
def internal_scope_re: "^(feat|fix)\\((deps|dep|ci|e2e|test|tests|workflow|lint|docs|build|release|renovate)\\)!?:";
def feat_re: "^feat(\\([^)]*\\))?!?:";
def fix_re: "^fix(\\([^)]*\\))?!?:";
def dedupe_key: sub("\\s*\\(#[0-9]+\\)\\s*$"; "");
def strip_prefix: sub("^(feat|fix)(\\([^)]*\\))?!?:\\s*"; "");
def user_feat: test(feat_re) and (test(internal_scope_re) | not);
def user_fix: test(fix_re) and (test(internal_scope_re) | not);

split("\n") | map(select(length > 0))
| map(split("\u001f") | {sha: .[0], subject: (.[1:] | join("\u001f"))})
| map(select(.subject | (test(release_chore_re) or test(merge_re)) | not))
| reduce .[] as $c ({seen: {}, kept: []};
    ($c.subject | dedupe_key) as $key
    | if .seen[$key] then .
      else .seen[$key] = true | .kept += [$c]
      end)
| .kept as $commits
| ($commits
   | map(select(.subject | test(security_re)))
   | map({summary: (.subject | sub("^(feat|fix)\\(security\\)!?:\\s*"; "")), sha: .sha})
  ) as $security
| ($commits
   | map(select(.subject | test(security_re) | not) | .subject)
  ) as $subjects
| ($subjects | map(select(user_feat) | strip_prefix)) as $features
| ($subjects | map(select(user_fix) | strip_prefix)) as $fixes
| ($subjects | map(select((user_feat or user_fix) | not))) as $other
| {version: $v, date: $d, features: $features, fixes: $fixes, other: $other, security: $security}
JQ_EOF

TMP_ENTRIES="$(mktemp)"
TMP_OUT="$(mktemp)"
trap 'rm -f "$TMP_ENTRIES" "$TMP_OUT"' EXIT

# Appends one categorized entry for commits in $range to TMP_ENTRIES.
# Using `git log` without a 2>/dev/null redirect: a failing git command
# (bad range, broken repo) fails the pipeline under pipefail and stops the
# script with git's own error message visible.
#
# Short sha (%h) matches this repo's existing convention (see
# security-scan-docker-image-scripts/run.sh's `git rev-parse --short
# HEAD`) and resolves fine in GitHub commit URLs. %x1f is git's own
# placeholder for the unit-separator byte that CATEGORIZE_JQ splits on.
emit_entry() {
  local version="$1" entry_date="$2" range="$3"
  git log --no-merges "$range" --pretty=%h%x1f%s | jq -R -s \
    --arg v "$version" \
    --arg d "$entry_date" \
    "$CATEGORIZE_JQ" >> "$TMP_ENTRIES"
}

# One tag per line, oldest-first (git's real semver sort), so "prev_tag"
# always refers to the release immediately before "tag".
tag_list="$(git tag -l 'v*' --sort=v:refname)"
if [ -z "$tag_list" ]; then
  echo "ERROR: no v* tags found (shallow clone?) — fetch full history with tags before generating changelog data" >&2
  exit 1
fi
readarray -t tags <<< "$tag_list"

prev_tag=""
for tag in "${tags[@]}"; do
  version="${tag#v}"
  entry_date="$(git log -1 --format=%ad --date=short "$tag")"

  if [ -n "$prev_tag" ]; then
    range="$prev_tag..$tag"
  else
    # First tag in history: everything up to and including it.
    range="$tag"
  fi

  emit_entry "$version" "$entry_date" "$range"
  prev_tag="$tag"
done

# Release channel: the release being built is not tagged yet. Its commits
# are everything after the newest tag, so synthesize its entry rather than
# leaving it out of the data.
if [ -n "$CURRENT_VERSION" ] && ! git rev-parse -q --verify "refs/tags/v$CURRENT_VERSION" >/dev/null; then
  newest_tag="${tags[-1]}"
  emit_entry "$CURRENT_VERSION" "$(date -u +%Y-%m-%d)" "$newest_tag..HEAD"
fi

# Entries were appended oldest-first (matching "tags"), with any synthesized
# head entry last; drop duplicate versions (first occurrence wins), then
# reverse for newest-first readability. The backend's Service also sorts by
# semver independently, so on-disk order isn't load-bearing here, only
# diff-friendliness is.
jq -s '
  reduce .[] as $e ([]; if any(.[]; .version == $e.version) then . else . + [$e] end)
  | reverse
' "$TMP_ENTRIES" > "$TMP_OUT"

count="$(jq 'length' "$TMP_OUT")"
if [ "$REQUIRE_ENTRIES" = true ] && [ "$count" -eq 0 ]; then
  echo "ERROR: --require-entries set and no changelog entries were generated; $OUTPUT left unchanged" >&2
  exit 1
fi

cat "$TMP_OUT" > "$OUTPUT"
echo "Generated $OUTPUT with $count version entries"
