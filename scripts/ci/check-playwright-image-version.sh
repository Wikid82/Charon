#!/usr/bin/env bash
# Fails fast when the Playwright container image and the installed
# @playwright/test package disagree on version. Browser builds inside the image
# must match the Playwright client, or runs fail with confusing protocol errors.
#
# Usage: check-playwright-image-version.sh <image-ref> [package.json]
#   image-ref     digest-pinned image, e.g. mcr.microsoft.com/playwright:v1.64.1-noble@sha256:...
#   package.json  defaults to $PLAYWRIGHT_PACKAGE_JSON, then node_modules/@playwright/test/package.json
set -euo pipefail

image_ref="${1:?usage: check-playwright-image-version.sh <image-ref> [package.json]}"
pkg_json="${2:-${PLAYWRIGHT_PACKAGE_JSON:-node_modules/@playwright/test/package.json}}"

if [[ "$image_ref" != *@sha256:* ]]; then
  echo "::error::Playwright image must be pinned by digest (got '$image_ref')" >&2
  exit 1
fi

# Strip the digest, then the repository, leaving the tag (e.g. v1.64.1-noble).
tag="${image_ref%%@*}"
tag="${tag##*:}"
if [[ ! "$tag" =~ ^v([0-9]+\.[0-9]+\.[0-9]+)-[a-z]+$ ]]; then
  echo "::error::Playwright image tag '$tag' must look like vX.Y.Z-<distro>" >&2
  exit 1
fi
image_version="${BASH_REMATCH[1]}"

if [[ ! -f "$pkg_json" ]]; then
  echo "::error::$pkg_json not found; run 'npm ci' before the Playwright container" >&2
  exit 1
fi
installed_version="$(node -p 'require(require("path").resolve(process.argv[1])).version' "$pkg_json")"

if [[ "$image_version" != "$installed_version" ]]; then
  echo "::error::Playwright image version ($image_version, from '$tag') does not match installed @playwright/test ($installed_version). Update PLAYWRIGHT_IMAGE in .github/workflows/e2e-tests-split.yml to v${installed_version}-<distro>@<digest>." >&2
  exit 1
fi

echo "Playwright image ${image_version} matches installed @playwright/test ${installed_version}"
