#!/usr/bin/env bats
#
# Covers scripts/ci/smoke-whats-new.sh without Docker or network access.
# `docker` and `curl` are PATH shims: the docker shim logs its arguments and
# reports what the tests configure, and the curl shim serves fixture files keyed
# by method and API path (with an optional per-call sequence, e.g.
# GET_api_v1_changelog_status.2.body, which is used on the second call).

SHA40="0123456789abcdef0123456789abcdef01234567"

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  SCRIPT_UNDER_TEST="$REPO_ROOT/scripts/ci/smoke-whats-new.sh"

  STUB_DIR="$(mktemp -d)"
  export STUB_DIR
  mkdir -p "$STUB_DIR/bin" "$STUB_DIR/fx"
  write_docker_stub
  write_curl_stub

  export PATH="$STUB_DIR/bin:$PATH"
  export SMOKE_WN_POLL_INTERVAL=0
  export SMOKE_WN_HEALTH_TIMEOUT=1
  unset DOCKER_HOST STUB_CONTAINER_EXISTS STUB_VOLUME_EXISTS STUB_RUN_RC \
    STUB_RUNNING STUB_SQLITE_CHANGES

  default_fixtures
}

teardown() {
  rm -rf "$STUB_DIR"
}

write_docker_stub() {
  cat > "$STUB_DIR/bin/docker" <<'STUB'
#!/usr/bin/env bash
echo "$*" >> "$STUB_DIR/docker.log"
case "$1" in
  container)
    [ "${STUB_CONTAINER_EXISTS:-0}" = 1 ] && exit 0
    exit 1 ;;
  volume)
    case "$2" in
      inspect) [ "${STUB_VOLUME_EXISTS:-0}" = 1 ] && exit 0; exit 1 ;;
      create) echo "$3" ;;
    esac ;;
  run)
    [ "${STUB_RUN_RC:-0}" = 0 ] || exit "$STUB_RUN_RC"
    echo "fake-container-id" ;;
  inspect) echo "${STUB_RUNNING:-true}" ;;
  exec) echo "${STUB_SQLITE_CHANGES:-1}" ;;
  logs) echo "fake log line" ;;
esac
exit 0
STUB
  chmod +x "$STUB_DIR/bin/docker"
}

write_curl_stub() {
  cat > "$STUB_DIR/bin/curl" <<'STUB'
#!/usr/bin/env bash
method=GET; outfile=""; wfmt=""; url=""
while [ "$#" -gt 0 ]; do
  case "$1" in
    -X) method="$2"; shift 2 ;;
    -o) outfile="$2"; shift 2 ;;
    -w) wfmt="$2"; shift 2 ;;
    -H|--data-binary|--max-time) shift 2 ;;
    -*) shift ;;
    *) url="$1"; shift ;;
  esac
done
echo "$method $url" >> "$STUB_DIR/curl.log"

path="$(printf '%s' "$url" | sed -E 's#^https?://[^/]+##')"
key="$(printf '%s' "$path" | sed -E 's#^/##; s#/#_#g')"
fx="$STUB_DIR/fx"

# Per-call sequence: a numbered fixture wins over the plain one.
count_file="$fx/count_${method}_${key}"
n=$(( $(cat "$count_file" 2>/dev/null || echo 0) + 1 ))
echo "$n" > "$count_file"

resolve() {
  local f
  for f in "$fx/${method}_${key}.${n}.$1" "$fx/${method}_${key}.$1"; do
    if [ -f "$f" ]; then echo "$f"; return 0; fi
  done
  return 1
}

if bf="$(resolve body)"; then cat "$bf" > "$outfile"; else echo '{}' > "$outfile"; fi
code=200
if cf="$(resolve code)"; then code="$(cat "$cf")"; fi
ct=application/json
if tf="$(resolve ct)"; then ct="$(cat "$tf")"; fi

case "$wfmt" in
  *content_type*) printf '%s %s' "$code" "$ct" ;;
  *) printf '%s' "$code" ;;
esac
exit 0
STUB
  chmod +x "$STUB_DIR/bin/curl"
}

# fx NAME CONTENT: writes a curl fixture file.
fx() {
  printf '%s' "$2" > "$STUB_DIR/fx/$1"
}

three_entries() {
  printf '%s' '{"versions":[
    {"version":"0.46.5","date":"2026-11-01","features":["add thing"],"fixes":[],"other":[],"security":[]},
    {"version":"0.46.4","date":"2026-10-20","features":[],"fixes":["fix thing"],"other":[],"security":[]},
    {"version":"0.46.3","date":"2026-10-01","features":[],"fixes":[],"other":["deps: bump"],"security":[]}
  ]}'
}

default_fixtures() {
  fx GET_api_v1_health.body "{\"status\":\"ok\",\"version\":\"0.46.5\",\"git_commit\":\"$SHA40\"}"
  fx POST_api_v1_setup.code 201
  fx POST_api_v1_auth_login.body '{"token":"tok-test"}'
  three_entries > "$STUB_DIR/fx/GET_api_v1_changelog_all.body"
  fx POST_api_v1_users.code 201
  fx POST_api_v1_changelog_ack.code 200
  fx GET_api_v1_changelog_status.body '{"show_changelog":false,"versions":[]}'
  fx GET_api_v1_changelog_status.1.body \
    '{"show_changelog":true,"versions":[{"version":"0.46.5"},{"version":"0.46.4"},{"version":"0.46.3"}]}'
}

docker_log() {
  cat "$STUB_DIR/docker.log" 2>/dev/null || true
}

# assert_logged PATTERN / refute_logged PATTERN: ERE checks against the docker log.
# Written as if-statements so that set -e reliably fails the test.
assert_logged() {
  if ! docker_log | grep -Eq -- "$1"; then
    echo "expected a docker call matching: $1" >&2
    return 1
  fi
}

refute_logged() {
  if docker_log | grep -Eq -- "$1"; then
    echo "unexpected docker call matching: $1" >&2
    return 1
  fi
}

# ---------------------------------------------------------------------------
# Argument parsing and validation (exit 2)
# ---------------------------------------------------------------------------

@test "--help prints usage and exits 0" {
  run "$SCRIPT_UNDER_TEST" --help
  [ "$status" -eq 0 ]
  [[ "$output" == *"usage: smoke-whats-new.sh"* ]]
}

@test "missing --image is a usage error" {
  run "$SCRIPT_UNDER_TEST" --expect-version-exact 0.46.5
  [ "$status" -eq 2 ]
  [[ "$output" == *"--image is required"* ]]
}

@test "no version flag is a usage error" {
  run "$SCRIPT_UNDER_TEST" --image img:test
  [ "$status" -eq 2 ]
  [[ "$output" == *"exactly one of --expect-version-exact or --expect-version-regex"* ]]
}

@test "both version flags together are a usage error" {
  run "$SCRIPT_UNDER_TEST" --image img:test --expect-version-exact 0.46.5 --expect-version-regex '0\.46\.5'
  [ "$status" -eq 2 ]
  [[ "$output" == *"exactly one of"* ]]
}

@test "the same version flag given twice is a usage error" {
  run "$SCRIPT_UNDER_TEST" --image img:test --expect-version-exact 0.46.5 --expect-version-exact 0.46.5
  [ "$status" -eq 2 ]
}

@test "an empty exact version is a usage error" {
  run "$SCRIPT_UNDER_TEST" --image img:test --expect-version-exact ''
  [ "$status" -eq 2 ]
  [[ "$output" == *"must not be empty"* ]]
}

@test "an invalid ERE in --expect-version-regex is a usage error" {
  run "$SCRIPT_UNDER_TEST" --image img:test --expect-version-regex '['
  [ "$status" -eq 2 ]
  [[ "$output" == *"not a valid ERE"* ]]
}

@test "a flag without its value is a usage error" {
  run "$SCRIPT_UNDER_TEST" --image img:test --expect-version-exact 0.46.5 --port
  [ "$status" -eq 2 ]
  [[ "$output" == *"--port needs a value"* ]]
}

@test "an unknown argument is a usage error" {
  run "$SCRIPT_UNDER_TEST" --image img:test --expect-version-exact 0.46.5 --bogus
  [ "$status" -eq 2 ]
  [[ "$output" == *"unknown argument: --bogus"* ]]
}

@test "non-numeric --expect-min-entries is a usage error" {
  run "$SCRIPT_UNDER_TEST" --image img:test --expect-version-exact 0.46.5 --expect-min-entries many
  [ "$status" -eq 2 ]
}

@test "--expect-newest must be X.Y.Z" {
  run "$SCRIPT_UNDER_TEST" --image img:test --expect-version-exact 0.46.5 --expect-newest 0.46
  [ "$status" -eq 2 ]
}

@test "--expect-commit must be a full 40-character sha" {
  run "$SCRIPT_UNDER_TEST" --image img:test --expect-version-exact 0.46.5 --expect-commit abc1234
  [ "$status" -eq 2 ]
}

@test "--expect-status-for-unseeded accepts only shown or hidden" {
  run "$SCRIPT_UNDER_TEST" --image img:test --expect-version-exact 0.46.5 --expect-status-for-unseeded maybe
  [ "$status" -eq 2 ]
}

@test "--port outside 1024-65535 is a usage error" {
  run "$SCRIPT_UNDER_TEST" --image img:test --expect-version-exact 0.46.5 --port 80
  [ "$status" -eq 2 ]
}

@test "--admin-email without --admin-password is a usage error" {
  run "$SCRIPT_UNDER_TEST" --image img:test --expect-version-exact 0.46.5 --admin-email a@example.com
  [ "$status" -eq 2 ]
  [[ "$output" == *"must be given together"* ]]
}

@test "usage errors never call docker" {
  run "$SCRIPT_UNDER_TEST" --image img:test
  [ "$status" -eq 2 ]
  [ ! -e "$STUB_DIR/docker.log" ]
}

# ---------------------------------------------------------------------------
# Version matching
# ---------------------------------------------------------------------------

@test "exact match passes and reports the version" {
  run "$SCRIPT_UNDER_TEST" --image img:test --expect-version-exact 0.46.5
  [ "$status" -eq 0 ]
  [[ "$output" == *"OK: version '0.46.5'"* ]]
  [[ "$output" == *"PASS: all smoke checks passed"* ]]
}

@test "exact match accepts the build-metadata version 0.46.5+dev as a literal string" {
  fx GET_api_v1_health.body "{\"status\":\"ok\",\"version\":\"0.46.5+dev\",\"git_commit\":\"$SHA40\"}"
  run "$SCRIPT_UNDER_TEST" --image img:test --expect-version-exact '0.46.5+dev'
  [ "$status" -eq 0 ]
}

@test "exact match is string equality, not a regex" {
  run "$SCRIPT_UNDER_TEST" --image img:test --expect-version-exact '0.46.[5]'
  [ "$status" -eq 1 ]
  [[ "$output" == *"expected exactly '0.46.[5]'"* ]]
}

@test "exact mismatch fails with exit 1" {
  run "$SCRIPT_UNDER_TEST" --image img:test --expect-version-exact 0.46.6
  [ "$status" -eq 1 ]
  [[ "$output" == *"version is '0.46.5', expected exactly '0.46.6'"* ]]
}

@test "regex with an unescaped plus does NOT match 0.46.5+dev" {
  fx GET_api_v1_health.body "{\"status\":\"ok\",\"version\":\"0.46.5+dev\",\"git_commit\":\"$SHA40\"}"
  run "$SCRIPT_UNDER_TEST" --image img:test --expect-version-regex '0.46.5+dev'
  [ "$status" -eq 1 ]
  [[ "$output" == *"does not match regex"* ]]
}

@test "regex with the plus escaped matches 0.46.5+dev" {
  fx GET_api_v1_health.body "{\"status\":\"ok\",\"version\":\"0.46.5+dev\",\"git_commit\":\"$SHA40\"}"
  run "$SCRIPT_UNDER_TEST" --image img:test --expect-version-regex '0\.46\.5\+dev'
  [ "$status" -eq 0 ]
}

@test "regex must match the whole version, not a substring" {
  fx GET_api_v1_health.body "{\"status\":\"ok\",\"version\":\"0.46.55\",\"git_commit\":\"$SHA40\"}"
  run "$SCRIPT_UNDER_TEST" --image img:test --expect-version-regex '0\.46\.5'
  [ "$status" -eq 1 ]
}

@test "regex alternatives are anchored as a group" {
  run "$SCRIPT_UNDER_TEST" --image img:test --expect-version-regex '0\.46\.[0-9]|9\.9\.9'
  [ "$status" -eq 0 ]
}

# ---------------------------------------------------------------------------
# Health and lifecycle
# ---------------------------------------------------------------------------

@test "HTML instead of JSON on /health fails and names the content type" {
  fx GET_api_v1_health.ct 'text/html; charset=utf-8'
  fx GET_api_v1_health.body '<!doctype html><html></html>'
  run "$SCRIPT_UNDER_TEST" --image img:test --expect-version-exact 0.46.5
  [ "$status" -eq 1 ]
  [[ "$output" == *"did not return 200 application/json"* ]]
  [[ "$output" == *"text/html"* ]]
}

@test "a container that exits before health is ready fails fast" {
  export STUB_RUNNING=false
  fx GET_api_v1_health.code 000
  run "$SCRIPT_UNDER_TEST" --image img:test --expect-version-exact 0.46.5
  [ "$status" -eq 1 ]
  [[ "$output" == *"container exited before health was ready"* ]]
}

@test "a failed docker run fails with exit 1" {
  export STUB_RUN_RC=125
  run "$SCRIPT_UNDER_TEST" --image img:test --expect-version-exact 0.46.5
  [ "$status" -eq 1 ]
  [[ "$output" == *"docker run failed"* ]]
}

@test "the container publishes on 127.0.0.1 only, on the given port, with the encryption key by name" {
  run "$SCRIPT_UNDER_TEST" --image img:test --expect-version-exact 0.46.5 --port 18080
  [ "$status" -eq 0 ]
  assert_logged '^run .*-p 127\.0\.0\.1:18080:8080 '
  assert_logged '-e CHARON_ENCRYPTION_KEY '
  assert_logged '-e CHARON_ENV=production'
  refute_logged 'CHARON_ENCRYPTION_KEY='
  refute_logged '-p 0\.0\.0\.0'
}

@test "without --port a free high port on 127.0.0.1 is chosen" {
  run "$SCRIPT_UNDER_TEST" --image img:test --expect-version-exact 0.46.5
  [ "$status" -eq 0 ]
  assert_logged '-p 127\.0\.0\.1:[0-9]{5}:8080 '
}

@test "the DOCKER_HOST environment is passed through and reported" {
  run env DOCKER_HOST=unix:///run/user/1001/docker.sock "$SCRIPT_UNDER_TEST" \
    --image img:test --expect-version-exact 0.46.5
  [ "$status" -eq 0 ]
  [[ "$output" == *"docker host: unix:///run/user/1001/docker.sock"* ]]
}

# ---------------------------------------------------------------------------
# Commit, changelog and setup assertions
# ---------------------------------------------------------------------------

@test "--expect-commit passes when git_commit matches" {
  run "$SCRIPT_UNDER_TEST" --image img:test --expect-version-exact 0.46.5 --expect-commit "$SHA40"
  [ "$status" -eq 0 ]
}

@test "--expect-commit fails when git_commit differs" {
  run "$SCRIPT_UNDER_TEST" --image img:test --expect-version-exact 0.46.5 \
    --expect-commit ffffffffffffffffffffffffffffffffffffffff
  [ "$status" -eq 1 ]
  [[ "$output" == *"git_commit is"* ]]
}

@test "--expect-min-entries fails when the changelog is shorter" {
  run "$SCRIPT_UNDER_TEST" --image img:test --expect-version-exact 0.46.5 --expect-min-entries 100
  [ "$status" -eq 1 ]
  [[ "$output" == *"has 3 entries, expected at least 100"* ]]
}

@test "--expect-min-entries 0 passes an empty changelog" {
  fx GET_api_v1_changelog_all.body '{"versions":[]}'
  run "$SCRIPT_UNDER_TEST" --image img:test --expect-version-exact 0.46.5 --expect-min-entries 0
  [ "$status" -eq 0 ]
}

@test "--expect-newest fails when the newest entry differs" {
  run "$SCRIPT_UNDER_TEST" --image img:test --expect-version-exact 0.46.5 --expect-newest 0.46.6
  [ "$status" -eq 1 ]
  [[ "$output" == *"newest changelog entry is '0.46.5', expected '0.46.6'"* ]]
}

@test "duplicate changelog versions fail" {
  fx GET_api_v1_changelog_all.body \
    '{"versions":[{"version":"0.46.5","features":[],"fixes":[],"other":[]},{"version":"0.46.5","features":[],"fixes":[],"other":[]}]}'
  run "$SCRIPT_UNDER_TEST" --image img:test --expect-version-exact 0.46.5
  [ "$status" -eq 1 ]
  [[ "$output" == *"duplicate version entries"* ]]
}

@test "merge-commit subjects in the changelog fail" {
  fx GET_api_v1_changelog_all.body \
    '{"versions":[{"version":"0.46.5","features":[],"fixes":["Merge branch x"],"other":[]}]}'
  run "$SCRIPT_UNDER_TEST" --image img:test --expect-version-exact 0.46.5
  [ "$status" -eq 1 ]
  [[ "$output" == *"merge-commit subject"* ]]
}

@test "a feature subject starting with Mergeable is not a merge commit" {
  fx GET_api_v1_changelog_all.body \
    '{"versions":[{"version":"0.46.5","features":["Mergeable configs"],"fixes":[],"other":[]}]}'
  run "$SCRIPT_UNDER_TEST" --image img:test --expect-version-exact 0.46.5
  [ "$status" -eq 0 ]
}

@test "a Merge branch subject in features fails" {
  fx GET_api_v1_changelog_all.body \
    '{"versions":[{"version":"0.46.5","features":["Merge branch x"],"fixes":[],"other":[]}]}'
  run "$SCRIPT_UNDER_TEST" --image img:test --expect-version-exact 0.46.5
  [ "$status" -eq 1 ]
  [[ "$output" == *"merge-commit subject"* ]]
}

@test "an existing volume is reused and never removed" {
  export STUB_VOLUME_EXISTS=1
  run "$SCRIPT_UNDER_TEST" --image img:test --expect-version-exact 0.46.5 --volume qa-wn-existing-data
  [ "$status" -eq 0 ]
  [[ "$output" == *"reusing existing volume qa-wn-existing-data"* ]]
  refute_logged '^volume create'
  refute_logged '^volume rm'
  assert_logged '^rm -f smoke-wn-'
}

@test "a missing non-smoke-wn volume is refused and nothing is created" {
  run "$SCRIPT_UNDER_TEST" --image img:test --expect-version-exact 0.46.5 --volume qa-wn-missing-data
  [ "$status" -eq 1 ]
  [[ "$output" == *"only creates smoke-wn-* volumes"* ]]
  refute_logged '^volume create'
  refute_logged '^run '
}

@test "an existing container with the generated name aborts before docker run" {
  export STUB_CONTAINER_EXISTS=1
  run "$SCRIPT_UNDER_TEST" --image img:test --expect-version-exact 0.46.5
  [ "$status" -eq 1 ]
  [[ "$output" == *"already exists"* ]]
  refute_logged '^run '
  refute_logged '^rm -f'
}

@test "setup returning 403 without admin credentials fails with guidance" {
  fx POST_api_v1_setup.code 403
  run "$SCRIPT_UNDER_TEST" --image img:test --expect-version-exact 0.46.5 --volume smoke-wn-x-data
  [ "$status" -eq 1 ]
  [[ "$output" == *"setup is already complete"* ]]
}

@test "setup returning 403 with supplied admin credentials signs in with them" {
  fx POST_api_v1_setup.code 403
  run "$SCRIPT_UNDER_TEST" --image img:test --expect-version-exact 0.46.5 \
    --admin-email admin@example.com --admin-password 'correct-horse'
  [ "$status" -eq 0 ]
  [[ "$output" == *"signing in with the supplied admin"* ]]
  refute_logged 'correct-horse'
}

@test "a failed login fails with exit 1" {
  fx POST_api_v1_auth_login.code 401
  run "$SCRIPT_UNDER_TEST" --image img:test --expect-version-exact 0.46.5
  [ "$status" -eq 1 ]
  [[ "$output" == *"login for"* ]]
}

# ---------------------------------------------------------------------------
# Unseeded user (modal) checks
# ---------------------------------------------------------------------------

@test "unseeded shown: 3 entries, dismissal, then hidden on later sign-ins" {
  fx GET_api_v1_changelog_status.2.body '{"show_changelog":false,"versions":[]}'
  fx GET_api_v1_changelog_status.3.body '{"show_changelog":false,"versions":[]}'
  fx GET_api_v1_changelog_status.4.body '{"show_changelog":false,"versions":[]}'
  run "$SCRIPT_UNDER_TEST" --image img:test --expect-version-exact 0.46.5 \
    --expect-min-entries 3 --expect-status-for-unseeded shown
  [ "$status" -eq 0 ]
  [[ "$output" == *"unseeded user, first login shows 3 entries"* ]]
  [[ "$output" == *"after dismiss_permanent, sign-in 2 hidden"* ]]
  grep -Eq 'POST http://127\.0\.0\.1:[0-9]+/api/v1/changelog/ack' "$STUB_DIR/curl.log"
}

@test "unseeded shown fails when the modal is hidden" {
  fx GET_api_v1_changelog_status.1.body '{"show_changelog":false,"versions":[]}'
  run "$SCRIPT_UNDER_TEST" --image img:test --expect-version-exact 0.46.5 --expect-status-for-unseeded shown
  [ "$status" -eq 1 ]
  [[ "$output" == *"expected the modal shown"* ]]
}

@test "unseeded shown fails when the modal does not show exactly 3 entries" {
  fx GET_api_v1_changelog_status.1.body \
    '{"show_changelog":true,"versions":[{"version":"0.46.5"},{"version":"0.46.4"}]}'
  run "$SCRIPT_UNDER_TEST" --image img:test --expect-version-exact 0.46.5 --expect-status-for-unseeded shown
  [ "$status" -eq 1 ]
  [[ "$output" == *"expected exactly 3 entries, got 2"* ]]
}

@test "unseeded shown fails when an entry is newer than the running base version" {
  fx GET_api_v1_changelog_status.1.body \
    '{"show_changelog":true,"versions":[{"version":"0.47.0"},{"version":"0.46.4"},{"version":"0.46.3"}]}'
  run "$SCRIPT_UNDER_TEST" --image img:test --expect-version-exact 0.46.5 --expect-status-for-unseeded shown
  [ "$status" -eq 1 ]
  [[ "$output" == *"newer than the running base 0.46.5"* ]]
}

@test "unseeded shown compares against the base of a build-metadata version" {
  fx GET_api_v1_health.body "{\"status\":\"ok\",\"version\":\"0.46.5+dev\",\"git_commit\":\"$SHA40\"}"
  run "$SCRIPT_UNDER_TEST" --image img:test --expect-version-exact '0.46.5+dev' \
    --expect-status-for-unseeded shown
  [ "$status" -eq 0 ]
}

# shown_for_base BASE ENTRY_VERSION: serves BASE as the running version and a
# single 3-entry modal whose newest entry is ENTRY_VERSION, then runs the
# unseeded shown check. The version ordering under test is SemVer precedence.
shown_for_base() {
  fx GET_api_v1_health.body "{\"status\":\"ok\",\"version\":\"$1\",\"git_commit\":\"$SHA40\"}"
  fx GET_api_v1_changelog_status.1.body \
    "{\"show_changelog\":true,\"versions\":[{\"version\":\"$2\"},{\"version\":\"0.46.4\"},{\"version\":\"0.46.3\"}]}"
  run "$SCRIPT_UNDER_TEST" --image img:test --expect-version-exact "$1" --expect-status-for-unseeded shown
}

@test "a prerelease entry is older than the release it precedes (0.47.0-rc.1 <= 0.47.0)" {
  shown_for_base 0.47.0 0.47.0-rc.1
  [ "$status" -eq 0 ]
}

@test "a release entry is newer than a running prerelease (0.47.0 > 0.47.0-rc.1)" {
  shown_for_base 0.47.0-rc.1 0.47.0
  [ "$status" -eq 1 ]
  [[ "$output" == *"newer than the running base 0.47.0-rc.1"* ]]
}

@test "prerelease numbers compare numerically (rc.10 > rc.2)" {
  shown_for_base 0.47.0-rc.2 0.47.0-rc.10
  [ "$status" -eq 1 ]
  [[ "$output" == *"newer than the running base 0.47.0-rc.2"* ]]
}

@test "prerelease numbers compare numerically (rc.2 <= rc.10)" {
  shown_for_base 0.47.0-rc.10 0.47.0-rc.2
  [ "$status" -eq 0 ]
}

@test "a non-SemVer entry version fails the shown check" {
  shown_for_base 0.46.5 v0.46.5
  [ "$status" -eq 1 ]
  [[ "$output" == *"is not a SemVer version"* ]]
}

@test "unseeded hidden passes when the modal stays hidden through dismissal" {
  fx GET_api_v1_changelog_status.1.body '{"show_changelog":false,"versions":[]}'
  fx GET_api_v1_changelog_status.2.body '{"show_changelog":false,"versions":[]}'
  fx GET_api_v1_changelog_status.3.body '{"show_changelog":false,"versions":[]}'
  fx GET_api_v1_changelog_status.4.body '{"show_changelog":false,"versions":[]}'
  run "$SCRIPT_UNDER_TEST" --image img:test --expect-version-exact 0.46.5 --expect-status-for-unseeded hidden
  [ "$status" -eq 0 ]
}

@test "a modal that comes back after dismiss_permanent fails (no nag loop)" {
  fx GET_api_v1_changelog_status.2.body \
    '{"show_changelog":true,"versions":[{"version":"0.46.5"},{"version":"0.46.4"},{"version":"0.46.3"}]}'
  run "$SCRIPT_UNDER_TEST" --image img:test --expect-version-exact 0.46.5 --expect-status-for-unseeded shown
  [ "$status" -eq 1 ]
  [[ "$output" == *"after dismiss_permanent: expected the modal hidden"* ]]
}

@test "the unseeded user update must change exactly one row" {
  export STUB_SQLITE_CHANGES=0
  run "$SCRIPT_UNDER_TEST" --image img:test --expect-version-exact 0.46.5 --expect-status-for-unseeded shown
  [ "$status" -eq 1 ]
  [[ "$output" == *"expected to clear last_seen_version on 1 row, changed 0"* ]]
}

# ---------------------------------------------------------------------------
# Cleanup and isolation
# ---------------------------------------------------------------------------

@test "a passing run removes its own container and volume" {
  run "$SCRIPT_UNDER_TEST" --image img:test --expect-version-exact 0.46.5
  [ "$status" -eq 0 ]
  assert_logged '^rm -f smoke-wn-[0-9a-f]{12}$'
  assert_logged '^volume rm smoke-wn-[0-9a-f]{12}-data$'
}

@test "the cleanup trap removes the container and volume when an assertion fails" {
  run "$SCRIPT_UNDER_TEST" --image img:test --expect-version-exact 0.46.6
  [ "$status" -eq 1 ]
  assert_logged '^rm -f smoke-wn-[0-9a-f]{12}$'
  assert_logged '^volume rm smoke-wn-[0-9a-f]{12}-data$'
}

@test "the cleanup trap removes the container and volume when health never becomes JSON" {
  fx GET_api_v1_health.ct text/plain
  run "$SCRIPT_UNDER_TEST" --image img:test --expect-version-exact 0.46.5
  [ "$status" -eq 1 ]
  assert_logged '^rm -f smoke-wn-'
  assert_logged '^volume rm smoke-wn-'
}

@test "the cleanup trap removes the container when docker run fails" {
  export STUB_RUN_RC=125
  run "$SCRIPT_UNDER_TEST" --image img:test --expect-version-exact 0.46.5
  [ "$status" -eq 1 ]
  assert_logged '^rm -f smoke-wn-'
}

@test "every removal targets only smoke-wn- objects, and the reused volume is never removed" {
  export STUB_VOLUME_EXISTS=1
  run "$SCRIPT_UNDER_TEST" --image img:test --expect-version-exact 0.46.6 --volume qa-wn-keep-data
  [ "$status" -eq 1 ]
  assert_logged '^rm -f smoke-wn-'
  refute_logged '^volume rm '
  local line
  while IFS= read -r line; do
    case "$line" in
      "rm "*)
        if [[ "$line" != *smoke-wn-* ]]; then echo "unsafe removal: $line" >&2; false; fi ;;
    esac
  done < "$STUB_DIR/docker.log"
}

@test "every docker call that names a container or volume uses a smoke-wn- name or the reused volume" {
  fx GET_api_v1_changelog_status.1.body '{"show_changelog":false,"versions":[]}'
  fx GET_api_v1_changelog_status.2.body '{"show_changelog":false,"versions":[]}'
  fx GET_api_v1_changelog_status.3.body '{"show_changelog":false,"versions":[]}'
  fx GET_api_v1_changelog_status.4.body '{"show_changelog":false,"versions":[]}'
  run "$SCRIPT_UNDER_TEST" --image img:test --expect-version-exact 0.46.5 --expect-status-for-unseeded hidden
  [ "$status" -eq 0 ]
  local line
  while IFS= read -r line; do
    case "$line" in
      "rm "*|"volume rm "*|"exec "*|"inspect "*|"container inspect "*|"volume inspect "*|"logs "*)
        if [[ "$line" != *smoke-wn-* ]]; then echo "untouched name used: $line" >&2; false; fi ;;
    esac
  done < "$STUB_DIR/docker.log"
}
