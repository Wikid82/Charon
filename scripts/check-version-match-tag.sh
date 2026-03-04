#!/usr/bin/env bash
set -euo pipefail

# ⚠️  DEPRECATED: This script is deprecated and will be removed in v2.0.0
#    Please use: .github/skills/scripts/skill-runner.sh utility-version-check
#    For more info: docs/AGENT_SKILLS_MIGRATION.md
echo "⚠️  WARNING: This script is deprecated and will be removed in v2.0.0" >&2
echo "    Please use: .github/skills/scripts/skill-runner.sh utility-version-check" >&2
echo "    For more info: docs/AGENT_SKILLS_MIGRATION.md" >&2
echo "" >&2
sleep 1

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

if [ ! -f ".version" ]; then
  echo "No .version file present; skipping version consistency check"
  exit 0
fi

VERSION_FILE=$(cat .version | tr -d '\n' | tr -d '\r')
# Use the globally latest semver tag, not just tags reachable from HEAD.
# git describe --tags --abbrev=0 only finds tags in the current branch's
# ancestry, which breaks on feature branches where release tags were applied
# to main/nightly and haven't been merged back yet.
GIT_TAG="$(git tag --sort=-v:refname 2>/dev/null | grep -E '^v?[0-9]+\.[0-9]+' | head -1 || echo "")"

if [ -z "$GIT_TAG" ]; then
  echo "No tags in repository; cannot validate .version against tag"
  # Do not fail; allow commits when no tags exist
  exit 0
fi

# Normalize: strip leading v if present in either
normalize() {
  echo "$1" | sed 's/^v//'
}

TAG_NORM=$(normalize "$GIT_TAG")
VER_NORM=$(normalize "$VERSION_FILE")

if [ "$TAG_NORM" != "$VER_NORM" ]; then
  echo "ERROR: .version ($VERSION_FILE) does not match latest Git tag ($GIT_TAG)"
  echo "To sync, either update .version or tag with 'v$VERSION_FILE'"
  exit 1
fi

echo "OK: .version matches latest Git tag $GIT_TAG"
