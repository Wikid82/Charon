#!/usr/bin/env bash
# Builds the Zensical source tree for the public docs site.
#
# Copies the user-facing subset of the repo-root docs/ directory, as listed in
# docs-site/scripts/docs-manifest.json, into docs-site/docs/ (the site's docs_dir). Also copies the
# landing page (docs-site/index.md) and the static assets (docs-site/assets/).
#
# docs/ is the single source of truth and this script never modifies it. docs-site/docs/ is git-ignored
# and recreated from scratch on every run, so a page removed from the manifest can never linger as a
# stale file. Any manifest entry that does not exist fails the run loudly, and so does a synced page
# that is missing from the nav in docs-site/zensical.toml (it would otherwise be published as an orphan).
#
# Requires: bash, jq. Run from anywhere; paths resolve relative to this script.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DOCS_SITE_DIR="$(cd "${SCRIPT_DIR}/.." && pwd)"
REPO_ROOT="$(cd "${DOCS_SITE_DIR}/.." && pwd)"

readonly SOURCE_DOCS_DIR="${REPO_ROOT}/docs"
readonly DEST_DOCS_DIR="${DOCS_SITE_DIR}/docs"
readonly MANIFEST_PATH="${SCRIPT_DIR}/docs-manifest.json"
readonly NAV_CONFIG="${DOCS_SITE_DIR}/zensical.toml"
readonly LANDING_PAGE="${DOCS_SITE_DIR}/index.md"
readonly ASSETS_DIR="${DOCS_SITE_DIR}/assets"

fail() {
  echo "[sync-docs] ERROR: $*" >&2
  exit 1
}

command -v jq >/dev/null 2>&1 || fail "jq is required but was not found on PATH"
[ -f "${MANIFEST_PATH}" ] || fail "manifest not found at ${MANIFEST_PATH}"
jq -e '.files and .directories' "${MANIFEST_PATH}" >/dev/null 2>&1 ||
  fail "manifest ${MANIFEST_PATH} must contain 'files' and 'directories' arrays"
[ -d "${SOURCE_DOCS_DIR}" ] || fail "source docs directory not found at ${SOURCE_DOCS_DIR}"
[ -f "${LANDING_PAGE}" ] || fail "landing page not found at ${LANDING_PAGE}"
[ -d "${ASSETS_DIR}" ] || fail "assets directory not found at ${ASSETS_DIR}"

# Wipe and recreate the generated tree so removed or renamed pages never linger.
rm -rf "${DEST_DOCS_DIR}"
mkdir -p "${DEST_DOCS_DIR}"

copied_pages=()

copy_file() {
  local rel="$1"
  local src="${SOURCE_DOCS_DIR}/${rel}"
  [ -f "${src}" ] ||
    fail "manifest \"files\" entry \"${rel}\" does not exist as a file under ${SOURCE_DOCS_DIR}; update docs-site/scripts/docs-manifest.json"
  mkdir -p "$(dirname "${DEST_DOCS_DIR}/${rel}")"
  cp "${src}" "${DEST_DOCS_DIR}/${rel}"
  copied_pages+=("${rel}")
}

copy_directory() {
  local rel="$1"
  local src="${SOURCE_DOCS_DIR}/${rel}"
  [ -d "${src}" ] ||
    fail "manifest \"directories\" entry \"${rel}\" does not exist as a directory under ${SOURCE_DOCS_DIR}; update docs-site/scripts/docs-manifest.json"
  mkdir -p "$(dirname "${DEST_DOCS_DIR}/${rel}")"
  cp -R "${src}" "${DEST_DOCS_DIR}/${rel}"
  local page
  while IFS= read -r page; do
    copied_pages+=("${page#"${SOURCE_DOCS_DIR}/"}")
  done < <(find "${src}" -type f -name '*.md' | sort)
}

file_count=0
while IFS= read -r rel; do
  copy_file "${rel}"
  file_count=$((file_count + 1))
done < <(jq -r '.files[]' "${MANIFEST_PATH}")

dir_count=0
while IFS= read -r rel; do
  copy_directory "${rel}"
  dir_count=$((dir_count + 1))
done < <(jq -r '.directories[]' "${MANIFEST_PATH}")

# Landing page and static assets are owned by docs-site/, not by docs/.
cp "${LANDING_PAGE}" "${DEST_DOCS_DIR}/index.md"
cp -R "${ASSETS_DIR}" "${DEST_DOCS_DIR}/assets"

# Every synced page must be reachable from the nav; otherwise it would be published as an orphan.
missing_from_nav=()
for page in "${copied_pages[@]}"; do
  if ! grep -qF "\"${page}\"" "${NAV_CONFIG}"; then
    missing_from_nav+=("${page}")
  fi
done
if [ "${#missing_from_nav[@]}" -gt 0 ]; then
  printf '[sync-docs] ERROR: these pages are not in the nav of %s:\n' "${NAV_CONFIG}" >&2
  printf '  - %s\n' "${missing_from_nav[@]}" >&2
  exit 1
fi

echo "[sync-docs] done: ${file_count} manifest file(s), ${dir_count} directory(ies), ${#copied_pages[@]} page(s) synced into ${DEST_DOCS_DIR}"
