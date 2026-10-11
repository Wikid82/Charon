#!/usr/bin/env bash
# Lint the given markdown files (lefthook passes the staged ones) with markdownlint-cli2.
#
# Rules: .markdownlint.jsonc (a few noisy rules are off until the repo-wide cleanup).
# Skipped paths: docs-site/docs (generated), node_modules, CHANGELOG*, plus everything listed
# in .markdownlintignore. Exits 0 without running anything when no lintable files remain.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
readonly SCRIPT_DIR
REPO_ROOT="$(cd "${SCRIPT_DIR}/../.." && pwd)"
readonly REPO_ROOT

cd "${REPO_ROOT}"

# Lint the lefthook-supplied files; with no args, lint every tracked markdown file (make lint-markdown).
if [ "$#" -gt 0 ]; then
  CANDIDATES=("$@")
else
  mapfile -t CANDIDATES < <(git ls-files '*.md')
fi

# Build ignore patterns: fixed generated paths plus .markdownlintignore (gitignore-style subset:
# "dir/" matches the whole directory, anything else matches by glob at any depth).
IGNORES=('docs-site/docs/**' '**/node_modules/**' '**/CHANGELOG*')
if [ -f .markdownlintignore ]; then
  while IFS= read -r line; do
    case "${line}" in
      '' | '#'*) continue ;;
      */) IGNORES+=("${line}**") ;;
      *) IGNORES+=("**/${line}") ;;
    esac
  done < .markdownlintignore
fi

# Drop deleted files and ignored paths (extglob-free matching via bash [[ == ]] globs).
FILES=()
for f in "${CANDIDATES[@]}"; do
  [ -f "${f}" ] || continue
  skip=0
  for pat in "${IGNORES[@]}"; do
    # shellcheck disable=SC2053 # intentional glob match against the pattern
    if [[ "${f}" == ${pat} || "/${f}" == ${pat} || "${f}" == ${pat#\*\*/} ]]; then
      skip=1
      break
    fi
  done
  [ "${skip}" -eq 0 ] && FILES+=("${f}")
done

if [ "${#FILES[@]}" -eq 0 ]; then
  echo "markdownlint: no lintable markdown files, skipping"
  exit 0
fi

if [ ! -x node_modules/.bin/markdownlint-cli2 ]; then
  echo "Error: markdownlint-cli2 not found. Run 'npm ci' at the repo root." >&2
  exit 127
fi

exec node_modules/.bin/markdownlint-cli2 "${FILES[@]}"
