#!/usr/bin/env bash
# Runs `playwright test <args>` inside the official Playwright container image.
#
# The image ships every browser engine with its OS libraries, so E2E jobs no
# longer run `playwright install --with-deps` (apt-based, and it routinely hit
# its timeout on flaky mirrors). The Charon stack still runs on the runner via
# docker compose, so the container uses host networking and reaches the app at
# http://127.0.0.1:8080 exactly as the runner does.
#
# Requires PLAYWRIGHT_IMAGE (digest-pinned, see .github/workflows/e2e-tests-split.yml)
# and node_modules installed on the runner (the container reuses the workspace).
#
# Environment forwarding: variables with the prefixes below are passed into the
# container by name. PLAYWRIGHT_BROWSERS_PATH is never forwarded, because the
# image's own browser directory must be used.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
image="${PLAYWRIGHT_IMAGE:?PLAYWRIGHT_IMAGE must be set to the digest-pinned Playwright image}"
pkg_json="${PLAYWRIGHT_PACKAGE_JSON:-$REPO_ROOT/node_modules/@playwright/test/package.json}"
readonly PULL_ATTEMPTS=3
pull_retry_delay="${PULL_RETRY_DELAY_SECONDS:-15}"

bash "$REPO_ROOT/scripts/ci/check-playwright-image-version.sh" "$image" "$pkg_json"

# Registry pulls from the runner are the remaining flaky network step; retry them.
pulled=false
for attempt in $(seq 1 "$PULL_ATTEMPTS"); do
  if docker pull --quiet "$image" > /dev/null; then
    pulled=true
    break
  fi
  echo "docker pull attempt ${attempt}/${PULL_ATTEMPTS} failed" >&2
  [[ "$attempt" -lt "$PULL_ATTEMPTS" ]] && sleep "$pull_retry_delay"
done
if [[ "$pulled" != true ]]; then
  echo "::error::Could not pull ${image} after ${PULL_ATTEMPTS} attempts" >&2
  exit 1
fi

# Collect the names of variables to forward. `docker run --env NAME` copies the
# value from this process, so secrets never appear on the docker command line.
forward_args=()
while IFS= read -r -d '' entry; do
  name="${entry%%=*}"
  case "$name" in
    PLAYWRIGHT_BROWSERS_PATH) ;;
    PLAYWRIGHT_* | CHARON_* | E2E_* | CADDY_ADMIN_HOST | CI | CI_LOG_LEVEL | DEBUG | TEST_WORKER_INDEX)
      forward_args+=(--env "$name")
      ;;
  esac
done < <(env -0)

# --init reaps browser child processes; --ipc=host avoids Chromium shared-memory
# crashes (both per Playwright's Docker guidance). The user mapping keeps the
# reports, traces and coverage written into the workspace owned by the runner
# user, so later upload-artifact steps can read them.
exec docker run --rm --init --ipc=host --network host \
  --user "$(id -u):$(id -g)" \
  --env HOME=/tmp \
  "${forward_args[@]}" \
  --volume "$REPO_ROOT:/work" \
  --workdir /work \
  "$image" \
  node node_modules/@playwright/test/cli.js test "$@"
