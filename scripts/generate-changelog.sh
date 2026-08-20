#!/usr/bin/env bash
# Regenerates backend/internal/changelog/data/changelog.json from
# conventional-commit git history, one entry per `v*` tag. Run in CI
# immediately before the release build so go:embed picks up fresh data —
# see .github/workflows/nightly-build.yml. Never commit the generated
# output back to the repo; the committed file must stay the `[]` placeholder.
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

OUTPUT="backend/internal/changelog/data/changelog.json"

if ! command -v jq >/dev/null 2>&1; then
  echo "ERROR: jq is required to generate changelog data" >&2
  exit 1
fi

# jq program that categorizes raw commits (fed in via stdin as
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
# `(security)`-scoped feat/fix commits are checked FIRST and routed
# exclusively into a new `security` array (never duplicated into
# features/fixes) — see
# docs/superpowers/specs/2026-07-29-security-changelog-category-design.md.
read -r -d '' CATEGORIZE_JQ <<'JQ_EOF' || true
split("\n") | map(select(length > 0))
| map(split("\u001f")) | map({sha: .[0], subject: (.[1:] | join("\u001f"))})
as $commits
| ($commits
   | map(select(.subject | test("^(feat|fix)\\(security\\)!?:")))
   | map({summary: (.subject | sub("^(feat|fix)\\(security\\)!?:\\s*"; "")), sha: .sha})
  ) as $security
| ($commits
   | map(select((.subject | test("^(feat|fix)\\(security\\)!?:")) | not))
   | map(.subject)
  ) as $subjects
| ($subjects
   | map(select(test("^feat(\\([^)]*\\))?!?:")))
   | map(sub("^feat(\\([^)]*\\))?!?:\\s*"; ""))
  ) as $features
| ($subjects
   | map(select(test("^fix(\\([^)]*\\))?!?:")))
   | map(sub("^fix(\\([^)]*\\))?!?:\\s*"; ""))
  ) as $fixes
| ($subjects
   | map(select(test("^(feat|fix)(\\([^)]*\\))?!?:") | not))
  ) as $other
| {version: $v, date: $d, features: $features, fixes: $fixes, other: $other, security: $security}
JQ_EOF

TMP_ENTRIES="$(mktemp)"
trap 'rm -f "$TMP_ENTRIES"' EXIT

# One tag per line, oldest-first (git's real semver sort), so "prev_tag"
# always refers to the release immediately before "tag".
readarray -t tags < <(git tag -l 'v*' --sort=v:refname)

prev_tag=""
for tag in "${tags[@]}"; do
  [ -z "$tag" ] && continue

  version="${tag#v}"
  date="$(git log -1 --format=%ad --date=short "$tag")"

  if [ -n "$prev_tag" ]; then
    range="$prev_tag..$tag"
  else
    # First tag in history: everything up to and including it.
    range="$tag"
  fi

  # %x1f emits a literal unit-separator byte (git's own hex-escape
  # placeholder syntax) between the short sha and the subject, matching
  # the \x1f delimiter CATEGORIZE_JQ splits each line on. Short sha
  # (%h) matches this repo's existing convention (see
  # security-scan-docker-image-scripts/run.sh's `git rev-parse --short
  # HEAD`) and resolves fine in GitHub commit URLs.
  git log "$range" --pretty=%h%x1f%s 2>/dev/null | jq -R -s \
    --arg v "$version" \
    --arg d "$date" \
    "$CATEGORIZE_JQ" >> "$TMP_ENTRIES"

  prev_tag="$tag"
done

# Entries were appended oldest-first (matching "tags"); reverse for
# newest-first readability. The backend's Service also sorts by semver
# independently, so on-disk order isn't load-bearing here, only
# diff-friendliness is.
jq -s 'reverse' "$TMP_ENTRIES" > "$OUTPUT"

count="$(jq 'length' "$OUTPUT")"
echo "Generated $OUTPUT with $count version entries"
