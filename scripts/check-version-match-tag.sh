#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$ROOT_DIR"

if [ ! -f ".version" ]; then
  echo "No .version file present; skipping version consistency check"
  exit 0
fi

VERSION_FILE=$(cat .version | tr -d '\n' | tr -d '\r')
GIT_TAG="$(git describe --tags --abbrev=0 2>/dev/null || echo "")"

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
