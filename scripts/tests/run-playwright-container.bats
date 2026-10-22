#!/usr/bin/env bats
#
# Covers scripts/ci/run-playwright-container.sh. A stub `docker` on PATH records
# every invocation, so no test touches a real daemon or the network.

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  WRAPPER="$REPO_ROOT/scripts/ci/run-playwright-container.sh"

  TMPROOT="$(mktemp -d)"
  mkdir "$TMPROOT/bin"
  DOCKER_LOG="$TMPROOT/docker.log"
  : >"$DOCKER_LOG"

  # Stub docker: logs "<subcommand> <args...>" per call. `pull` fails the first
  # FAIL_PULLS attempts (default 0) to exercise the retry loop.
  cat >"$TMPROOT/bin/docker" <<'STUB'
#!/usr/bin/env bash
echo "$*" >>"$DOCKER_LOG"
if [[ "$1" == "pull" ]]; then
  count_file="$DOCKER_LOG.pulls"
  n=$(( $(cat "$count_file" 2>/dev/null || echo 0) + 1 ))
  echo "$n" >"$count_file"
  (( n <= ${FAIL_PULLS:-0} )) && exit 1
fi
exit 0
STUB
  chmod +x "$TMPROOT/bin/docker"

  PKG_JSON="$TMPROOT/package.json"
  echo '{"name":"@playwright/test","version":"1.64.1"}' >"$PKG_JSON"
  IMAGE="mcr.microsoft.com/playwright:v1.64.1-noble@sha256:$(printf 'b%.0s' {1..64})"
}

teardown() {
  rm -rf "$TMPROOT"
}

# run_wrapper [extra VAR=value ...] -- [playwright args...]
# Runs the wrapper with a clean environment so only the variables under test are visible.
run_wrapper() {
  local -a extra=()
  while [[ $# -gt 0 && "$1" != "--" ]]; do extra+=("$1"); shift; done
  [[ "${1:-}" == "--" ]] && shift
  run env -i PATH="$TMPROOT/bin:$PATH" HOME="$TMPROOT" DOCKER_LOG="$DOCKER_LOG" \
    PLAYWRIGHT_IMAGE="$IMAGE" PLAYWRIGHT_PACKAGE_JSON="$PKG_JSON" PULL_RETRY_DELAY_SECONDS=0 \
    "${extra[@]}" bash "$WRAPPER" "$@"
}

docker_run_line() {
  grep '^run ' "$DOCKER_LOG"
}

@test "runs playwright test inside the image with host networking and the workspace mounted" {
  run_wrapper -- --project=firefox tests/core
  [ "$status" -eq 0 ]
  line="$(docker_run_line)"
  [[ "$line" == *"--network host"* ]]
  [[ "$line" == *"--ipc=host"* ]]
  [[ "$line" == *"--init"* ]]
  [[ "$line" == *"--volume $REPO_ROOT:/work"* ]]
  [[ "$line" == *"$IMAGE node node_modules/@playwright/test/cli.js test --project=firefox tests/core"* ]]
}

@test "forwards only allowlisted variables, by name, never their values" {
  run_wrapper PLAYWRIGHT_BASE_URL=http://127.0.0.1:8080 PLAYWRIGHT_SETUP_BROWSER=firefox \
    CHARON_EMERGENCY_TOKEN=super-secret-value UNRELATED_VAR=nope -- --project=firefox
  [ "$status" -eq 0 ]
  line="$(docker_run_line)"
  [[ "$line" == *"--env PLAYWRIGHT_BASE_URL"* ]]
  [[ "$line" == *"--env PLAYWRIGHT_SETUP_BROWSER"* ]]
  [[ "$line" == *"--env CHARON_EMERGENCY_TOKEN"* ]]
  [[ "$line" != *"UNRELATED_VAR"* ]]
  [[ "$line" != *"super-secret-value"* ]]
}

@test "never forwards PLAYWRIGHT_BROWSERS_PATH so the image browser directory is used" {
  run_wrapper PLAYWRIGHT_BROWSERS_PATH=/runner/browsers -- --project=chromium
  [ "$status" -eq 0 ]
  [[ "$(docker_run_line)" != *"PLAYWRIGHT_BROWSERS_PATH"* ]]
}

@test "does not run the container when the image version drifts" {
  run_wrapper -- --project=chromium
  [ "$status" -eq 0 ]
  run_wrapper PLAYWRIGHT_IMAGE="mcr.microsoft.com/playwright:v1.63.0-noble@sha256:$(printf 'b%.0s' {1..64})" -- --project=chromium
  [ "$status" -eq 1 ]
  [[ "$output" == *"does not match installed @playwright/test"* ]]
  [[ "$(grep -c '^run ' "$DOCKER_LOG")" -eq 1 ]]
}

@test "retries a failed image pull and still runs when a later attempt succeeds" {
  run_wrapper FAIL_PULLS=2 -- --project=webkit
  [ "$status" -eq 0 ]
  [[ "$(grep -c '^pull ' "$DOCKER_LOG")" -eq 3 ]]
  [[ -n "$(docker_run_line)" ]]
}

@test "fails after three failed pull attempts without starting the container" {
  run_wrapper FAIL_PULLS=9 -- --project=webkit
  [ "$status" -eq 1 ]
  [[ "$output" == *"Could not pull"* ]]
  [[ -z "$(docker_run_line)" ]]
}

@test "fails fast when PLAYWRIGHT_IMAGE is unset" {
  run env -i PATH="$TMPROOT/bin:$PATH" HOME="$TMPROOT" DOCKER_LOG="$DOCKER_LOG" bash "$WRAPPER" --project=chromium
  [ "$status" -ne 0 ]
  [[ "$output" == *"PLAYWRIGHT_IMAGE must be set"* ]]
}
