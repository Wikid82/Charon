#!/usr/bin/env bash
# Runs `playwright test <args>` inside the official Playwright container image.
#
# The image ships every browser engine with its OS libraries, so E2E jobs no
# longer run `playwright install --with-deps` (apt-based, and it routinely hit
# its timeout on flaky mirrors). The Charon stack still runs on the runner via
# docker compose, so the container uses host networking and reaches the app at
# http://127.0.0.1:8080 exactly as the runner does.
#
# Docker access: some specs (tests/security/self-proxy-requests.spec.ts,
# tests/utils/test-certificate.ts, and friends) shell out to the `docker` CLI
# to locate and inspect the Charon container that compose started on the
# runner. The image has no docker CLI and no daemon socket, so when a socket is
# present this script mounts the runner's docker CLI binary (read-only) at
# /usr/local/bin/docker and the daemon socket at /var/run/docker.sock, and adds
# the socket's group so the non-root container user can open it. The CLI is
# dynamically linked only against glibc, which noble provides. If no socket
# exists (or DOCKER_HOST is not a unix socket) the wrapper says so on stderr and
# continues without docker access, so those specs fail on their own terms.
#
# Security trade-off: anything in the container can control the runner's Docker
# daemon, which is root-equivalent on the host. This is acceptable only because
# the runners are ephemeral GitHub-hosted VMs and the code that runs in the
# container is first-party test code. Do not reuse this wrapper on a persistent
# or shared host.
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

# docker_access_args fills docker_args with the mounts, group and environment
# that let the container run the host's docker CLI against the host daemon.
# Leaves docker_args empty (and prints a note) when no usable socket exists.
docker_args=()
docker_access_args() {
  local host="${DOCKER_HOST:-unix:///var/run/docker.sock}"
  if [[ "$host" != unix://* ]]; then
    echo "note: DOCKER_HOST=${host} is not a unix socket; docker access skipped in the container" >&2
    return 0
  fi
  local socket="${host#unix://}"
  if [[ ! -e "$socket" ]]; then
    echo "note: no Docker socket at ${socket}; docker access skipped in the container" >&2
    return 0
  fi

  # Resolve symlinks on the host so bind mounts point at the real files.
  local cli socket_real socket_gid
  cli="$(readlink -f "$(command -v docker)")"
  socket_real="$(readlink -f "$socket")"
  socket_gid="$(stat -c %g "$socket_real")"

  docker_args=(
    --volume "$cli:/usr/local/bin/docker:ro"
    --volume "$socket_real:/var/run/docker.sock"
    --env DOCKER_HOST=unix:///var/run/docker.sock
    --group-add "$socket_gid"
  )
}
docker_access_args

# --init reaps browser child processes; --ipc=host avoids Chromium shared-memory
# crashes (both per Playwright's Docker guidance). The user mapping keeps the
# reports, traces and coverage written into the workspace owned by the runner
# user, so later upload-artifact steps can read them.
exec docker run --rm --init --ipc=host --network host \
  --user "$(id -u):$(id -g)" \
  --env HOME=/tmp \
  "${forward_args[@]}" \
  "${docker_args[@]}" \
  --volume "$REPO_ROOT:/work" \
  --workdir /work \
  "$image" \
  node node_modules/@playwright/test/cli.js test "$@"
