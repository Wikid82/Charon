#!/usr/bin/env bash
# Smoke-tests a BUILT Charon image before it is published (docker-build.yml
# smoke-whats-new job, nightly-build.yml, and docs/plans/current_spec.md §4.7/§8).
#
# The script starts the image on a named volume, waits for the JSON health
# endpoint, asserts the reported version (and optionally the commit), signs in,
# and checks the What's New changelog API: the entry count, the newest entry,
# and optionally the modal behaviour for a user whose last_seen_version is empty.
#
# Isolation:
#   - Every container and volume the script creates is named smoke-wn-<rand>
#     (volume: smoke-wn-<rand>-data) and removed by an EXIT trap.
#   - The container publishes on 127.0.0.1 only, on a free high port unless
#     --port is given.
#   - --volume reuses an EXISTING named volume (upgrade-path checks). The script
#     never removes a volume it did not create. A missing --volume must use the
#     smoke-wn- prefix, so the script only ever creates smoke-wn-* objects.
#   - Existing containers and volumes are never touched.
#
# Host dependencies: docker, curl, jq. sqlite3 runs inside the image
# (docker exec), so no host sqlite3 is needed. DOCKER_HOST is honored as-is.
#
# Exit codes: 0 pass, 1 assertion or runtime failure, 2 usage error.
set -euo pipefail

readonly SEMVER_RE='^[0-9]+\.[0-9]+\.[0-9]+$'
readonly SHA_RE='^[0-9a-f]{40}$'
readonly VOLUME_NAME_RE='^[A-Za-z0-9][A-Za-z0-9_.-]*$'
readonly CONTAINER_PORT=8080
readonly DATA_DIR=/app/data
readonly DB_PATH=/app/data/charon.db
readonly NAME_PREFIX=smoke-wn-

HEALTH_TIMEOUT="${SMOKE_WN_HEALTH_TIMEOUT:-120}"
POLL_INTERVAL="${SMOKE_WN_POLL_INTERVAL:-2}"

IMAGE=""
EXPECT_EXACT=""
EXPECT_REGEX=""
VERSION_FLAG_COUNT=0
EXPECT_COMMIT=""
EXPECT_MIN_ENTRIES=""
EXPECT_NEWEST=""
EXPECT_UNSEEDED=""
PORT=""
VOLUME=""
ADMIN_EMAIL=""
ADMIN_PASSWORD=""

RAND=""
CTR=""
VOLUME_OWNED=0
CONTAINER_OWNED=0
WORKDIR=""
BODY_FILE=""
BASE_URL=""
RUNNING_BASE=""
API_CODE=""
JQ_OUT=""
TOKEN=""
CREDS_GIVEN=0
CHARON_ENCRYPTION_KEY=""

log() {
  printf 'smoke-wn: %s\n' "$*" >&2
}

fail() {
  log "FAIL: $*"
  exit 1
}

usage_error() {
  log "ERROR: $*"
  log "run with --help for usage"
  exit 2
}

usage() {
  cat <<'EOF'
usage: smoke-whats-new.sh --image <ref>
         (--expect-version-exact <string> | --expect-version-regex <ERE>)
         [--expect-commit <40-hex sha>] [--expect-min-entries N]
         [--expect-newest X.Y.Z] [--expect-status-for-unseeded <shown|hidden>]
         [--port <1024-65535>] [--volume <name>]
         [--admin-email <email> --admin-password <password>]

Version flags (exactly one is required):
  --expect-version-exact   compared as a plain string (what CI uses)
  --expect-version-regex   ERE matched against the whole version. '+' is a
                           regex quantifier, so escape it: 0\.46\.5\+dev

Other checks:
  --expect-commit          health git_commit must equal this sha
  --expect-min-entries     /changelog/all must hold at least N entries
  --expect-newest          the newest /changelog/all entry must equal X.Y.Z
  --expect-status-for-unseeded
                           create a user, clear its last_seen_version in the
                           image's sqlite, and assert the changelog status is
                           shown (exactly 3 entries, all <= the running base
                           version) or hidden; then dismiss_permanent and assert
                           hidden on the next two logins

Volume and admin options (upgrade-path checks):
  --volume                 existing named volume to reuse (never removed by the
                           script), or a new smoke-wn-* name (created and removed)
  --admin-email / --admin-password
                           admin credentials for setup/login; required when the
                           reused volume is already set up

Environment:
  DOCKER_HOST              honored by the docker CLI (e.g. unix:///run/user/1001/docker.sock)
  SMOKE_WN_HEALTH_TIMEOUT  seconds to wait for health (default 120)
  SMOKE_WN_POLL_INTERVAL   seconds between health probes (default 2)

Exit codes: 0 pass, 1 assertion or runtime failure, 2 usage error.
EOF
}

# ---------------------------------------------------------------------------
# Argument parsing and validation
# ---------------------------------------------------------------------------

# require_value <flag> <remaining-arg-count>: the flag needs a following value.
require_value() {
  if [ "$2" -lt 2 ] || [[ "${3-}" == --* ]]; then
    usage_error "$1 needs a value"
  fi
}

parse_args() {
  while [ "$#" -gt 0 ]; do
    case "$1" in
      -h|--help)
        usage
        exit 0
        ;;
      --image)
        require_value "$1" "$#" "${2-}"
        IMAGE="$2"; shift 2 ;;
      --expect-version-exact)
        require_value "$1" "$#" "${2-}"
        EXPECT_EXACT="$2"; VERSION_FLAG_COUNT=$((VERSION_FLAG_COUNT + 1)); shift 2 ;;
      --expect-version-regex)
        require_value "$1" "$#" "${2-}"
        EXPECT_REGEX="$2"; VERSION_FLAG_COUNT=$((VERSION_FLAG_COUNT + 1)); shift 2 ;;
      --expect-commit)
        require_value "$1" "$#" "${2-}"
        EXPECT_COMMIT="$2"; shift 2 ;;
      --expect-min-entries)
        require_value "$1" "$#" "${2-}"
        EXPECT_MIN_ENTRIES="$2"; shift 2 ;;
      --expect-newest)
        require_value "$1" "$#" "${2-}"
        EXPECT_NEWEST="$2"; shift 2 ;;
      --expect-status-for-unseeded)
        require_value "$1" "$#" "${2-}"
        EXPECT_UNSEEDED="$2"; shift 2 ;;
      --port)
        require_value "$1" "$#" "${2-}"
        PORT="$2"; shift 2 ;;
      --volume)
        require_value "$1" "$#" "${2-}"
        VOLUME="$2"; shift 2 ;;
      --admin-email)
        require_value "$1" "$#" "${2-}"
        ADMIN_EMAIL="$2"; shift 2 ;;
      --admin-password)
        require_value "$1" "$#" "${2-}"
        ADMIN_PASSWORD="$2"; shift 2 ;;
      *)
        usage_error "unknown argument: $1" ;;
    esac
  done
}

validate_args() {
  [ -n "$IMAGE" ] || usage_error "--image is required"
  [ "$VERSION_FLAG_COUNT" -eq 1 ] || usage_error "exactly one of --expect-version-exact or --expect-version-regex is required"

  if [ -n "$EXPECT_REGEX" ]; then
    # grep -E exits 2 for an invalid ERE; 1 only means "no match", which is fine here.
    local probe_rc=0
    printf '' | grep -E -- "$EXPECT_REGEX" >/dev/null 2>&1 || probe_rc=$?
    [ "$probe_rc" -ne 2 ] || usage_error "--expect-version-regex is not a valid ERE: $EXPECT_REGEX"
  fi
  if [ -z "$EXPECT_EXACT" ] && [ -z "$EXPECT_REGEX" ]; then
    usage_error "the version expectation must not be empty"
  fi

  if [ -n "$EXPECT_COMMIT" ] && [[ ! "$EXPECT_COMMIT" =~ $SHA_RE ]]; then
    usage_error "--expect-commit must be a full 40-character lowercase sha"
  fi
  if [ -n "$EXPECT_MIN_ENTRIES" ] && [[ ! "$EXPECT_MIN_ENTRIES" =~ ^[0-9]+$ ]]; then
    usage_error "--expect-min-entries must be a non-negative integer"
  fi
  if [ -n "$EXPECT_NEWEST" ] && [[ ! "$EXPECT_NEWEST" =~ $SEMVER_RE ]]; then
    usage_error "--expect-newest must be X.Y.Z"
  fi
  if [ -n "$EXPECT_UNSEEDED" ] && [ "$EXPECT_UNSEEDED" != shown ] && [ "$EXPECT_UNSEEDED" != hidden ]; then
    usage_error "--expect-status-for-unseeded must be shown or hidden"
  fi
  if [ -n "$PORT" ]; then
    if [[ ! "$PORT" =~ ^[0-9]+$ ]] || [ "$PORT" -lt 1024 ] || [ "$PORT" -gt 65535 ]; then
      usage_error "--port must be between 1024 and 65535"
    fi
  fi
  if [ -n "$VOLUME" ] && [[ ! "$VOLUME" =~ $VOLUME_NAME_RE ]]; then
    usage_error "--volume has an invalid name: $VOLUME"
  fi
  if [ -n "$ADMIN_EMAIL" ] || [ -n "$ADMIN_PASSWORD" ]; then
    [ -n "$ADMIN_EMAIL" ] && [ -n "$ADMIN_PASSWORD" ] || usage_error "--admin-email and --admin-password must be given together"
    [ "${#ADMIN_PASSWORD}" -ge 8 ] || usage_error "--admin-password must be at least 8 characters"
    CREDS_GIVEN=1
  fi
  if [[ ! "$HEALTH_TIMEOUT" =~ ^[0-9]+$ ]]; then
    usage_error "SMOKE_WN_HEALTH_TIMEOUT must be a whole number of seconds"
  fi
}

check_dependencies() {
  local tool
  for tool in docker curl jq; do
    command -v "$tool" >/dev/null 2>&1 || fail "missing required tool: $tool"
  done
}

# ---------------------------------------------------------------------------
# Helpers
# ---------------------------------------------------------------------------

rand_hex() {
  head -c "$1" /dev/urandom | od -An -tx1 | tr -d ' \n'
}

gen_password() {
  head -c 24 /dev/urandom | base64 -w0 | tr -dc 'A-Za-z0-9'
}

# pick_free_port prints a high port on 127.0.0.1 that nothing is listening on.
pick_free_port() {
  local candidate i
  for i in $(seq 1 50); do
    candidate=$(( 20000 + RANDOM % 40000 ))
    if ! (exec 3<>"/dev/tcp/127.0.0.1/$candidate") 2>/dev/null; then
      echo "$candidate"
      return 0
    fi
  done
  return 1
}

# api METHOD PATH [TOKEN] [JSON_BODY]: sets API_CODE and writes the body to BODY_FILE.
api() {
  local method="$1" path="$2" token="${3:-}" body="${4:-}" code
  local -a args=(-sS --max-time 30 -X "$method" -o "$BODY_FILE" -w '%{http_code}' -H 'Accept: application/json')
  if [ -n "$token" ]; then
    args+=(-H "Authorization: Bearer $token")
  fi
  if [ -n "$body" ]; then
    args+=(-H 'Content-Type: application/json' --data-binary "$body")
  fi
  code="$(curl "${args[@]}" "$BASE_URL$path" 2>/dev/null || true)"
  API_CODE="${code:-000}"
}

# jqr FILTER: runs jq against BODY_FILE and stores the raw output in JQ_OUT.
jqr() {
  if ! JQ_OUT="$(jq -r "$1" "$BODY_FILE" 2>/dev/null)"; then
    fail "response body is not the expected JSON (filter $1)"
  fi
}

expect_2xx() {
  case "$API_CODE" in
    2??) return 0 ;;
    *) fail "$1 returned HTTP $API_CODE" ;;
  esac
}

# semver_le A B: succeeds when dotted numeric version A <= B.
semver_le() {
  [ "$(printf '%s\n%s\n' "$1" "$2" | sort -V | head -n 1)" = "$1" ]
}

# login EMAIL PASSWORD: sets TOKEN. The password is never logged.
login() {
  local body
  body="$(jq -cn --arg e "$1" --arg p "$2" '{email:$e, password:$p}')"
  api POST /api/v1/auth/login "" "$body"
  [ "$API_CODE" = 200 ] || fail "login for $1 returned HTTP $API_CODE"
  jqr '.token'
  TOKEN="$JQ_OUT"
  [ -n "$TOKEN" ] && [ "$TOKEN" != null ] || fail "login for $1 returned no token"
}

# ---------------------------------------------------------------------------
# Lifecycle: volume, container, cleanup
# ---------------------------------------------------------------------------

cleanup() {
  local rc=$?
  set +e
  if [ "$CONTAINER_OWNED" = 1 ] && [ -n "$CTR" ]; then
    docker rm -f "$CTR" >/dev/null 2>&1
  fi
  if [ "$VOLUME_OWNED" = 1 ] && [[ "$VOLUME" == "$NAME_PREFIX"* ]]; then
    docker volume rm "$VOLUME" >/dev/null 2>&1
  fi
  if [ -n "$WORKDIR" ]; then
    rm -rf "$WORKDIR"
  fi
  exit "$rc"
}

prepare_volume() {
  if [ -z "$VOLUME" ]; then
    VOLUME="${NAME_PREFIX}${RAND}-data"
  fi
  if docker volume inspect "$VOLUME" >/dev/null 2>&1; then
    log "reusing existing volume $VOLUME (not removed on exit)"
    return 0
  fi
  case "$VOLUME" in
    "$NAME_PREFIX"*) ;;
    *) fail "volume $VOLUME does not exist; this script only creates ${NAME_PREFIX}* volumes" ;;
  esac
  docker volume create "$VOLUME" >/dev/null || fail "could not create volume $VOLUME"
  VOLUME_OWNED=1
}

start_container() {
  if [ -z "$PORT" ]; then
    PORT="$(pick_free_port)" || fail "could not find a free port on 127.0.0.1"
  fi
  BASE_URL="http://127.0.0.1:${PORT}"

  if docker container inspect "$CTR" >/dev/null 2>&1; then
    fail "container $CTR already exists"
  fi
  CONTAINER_OWNED=1
  # The key is passed by name only, so it never appears in argv.
  export CHARON_ENCRYPTION_KEY
  docker run -d --name "$CTR" \
    -p "127.0.0.1:${PORT}:${CONTAINER_PORT}" \
    -v "${VOLUME}:${DATA_DIR}" \
    -e CHARON_ENV=production \
    -e CHARON_ENCRYPTION_KEY \
    "$IMAGE" >/dev/null || fail "docker run failed for $IMAGE"
  log "started $CTR from $IMAGE on $BASE_URL"
}

# wait_for_health: polls until /api/v1/health returns 200 application/json.
# The SPA also answers 200 with text/html, which must not count as healthy.
wait_for_health() {
  local deadline=$(( SECONDS + HEALTH_TIMEOUT )) probe code ct running last
  while :; do
    probe="$(curl -sS --max-time 5 -o "$BODY_FILE" -w '%{http_code} %{content_type}' \
      "$BASE_URL/api/v1/health" 2>/dev/null || true)"
    code="${probe%% *}"
    ct="${probe#* }"
    last="HTTP ${code:-none}, content-type ${ct:-none}"
    if [ "$code" = 200 ] && [[ "$ct" == application/json* ]]; then
      return 0
    fi
    running="$(docker inspect -f '{{.State.Running}}' "$CTR" 2>/dev/null || echo unknown)"
    if [ "$running" = false ]; then
      docker logs --tail 40 "$CTR" >&2 2>&1 || true
      fail "container exited before health was ready"
    fi
    if [ "$SECONDS" -ge "$deadline" ]; then
      fail "health did not return 200 application/json within ${HEALTH_TIMEOUT}s (last: $last)"
    fi
    sleep "$POLL_INTERVAL"
  done
}

# ---------------------------------------------------------------------------
# Assertions
# ---------------------------------------------------------------------------

check_version() {
  local version
  jqr '.version'
  version="$JQ_OUT"
  if [ -n "$EXPECT_EXACT" ]; then
    [ "$version" = "$EXPECT_EXACT" ] || fail "version is '$version', expected exactly '$EXPECT_EXACT'"
  else
    local re="^(${EXPECT_REGEX})\$"
    [[ "$version" =~ $re ]] || fail "version '$version' does not match regex '$EXPECT_REGEX'"
  fi
  RUNNING_BASE="${version%%+*}"
  log "OK: version '$version' (base $RUNNING_BASE)"
}

check_commit() {
  [ -n "$EXPECT_COMMIT" ] || return 0
  jqr '.git_commit'
  [ "$JQ_OUT" = "$EXPECT_COMMIT" ] || fail "git_commit is '$JQ_OUT', expected '$EXPECT_COMMIT'"
  log "OK: git_commit matches"
}

setup_and_login() {
  local body
  if [ "$CREDS_GIVEN" != 1 ]; then
    ADMIN_EMAIL="${NAME_PREFIX}${RAND}@example.com"
    ADMIN_PASSWORD="$(gen_password)"
  fi
  body="$(jq -cn --arg e "$ADMIN_EMAIL" --arg p "$ADMIN_PASSWORD" \
    '{name:"Smoke Test", email:$e, password:$p}')"
  api POST /api/v1/setup "" "$body"
  case "$API_CODE" in
    2??) log "setup: created the admin user" ;;
    403)
      [ "$CREDS_GIVEN" = 1 ] || fail "setup is already complete on this volume; pass --admin-email and --admin-password"
      log "setup: already complete, signing in with the supplied admin" ;;
    *) fail "setup returned HTTP $API_CODE" ;;
  esac
  login "$ADMIN_EMAIL" "$ADMIN_PASSWORD"
  log "OK: signed in as admin"
}

check_changelog_all() {
  local count newest
  api GET /api/v1/changelog/all "$TOKEN"
  [ "$API_CODE" = 200 ] || fail "changelog/all returned HTTP $API_CODE"

  jqr '.versions | length'
  count="$JQ_OUT"
  [[ "$count" =~ ^[0-9]+$ ]] || fail "changelog/all has no versions array"
  if [ -n "$EXPECT_MIN_ENTRIES" ]; then
    [ "$count" -ge "$EXPECT_MIN_ENTRIES" ] || fail "changelog has $count entries, expected at least $EXPECT_MIN_ENTRIES"
  fi

  jqr '[.versions[].version] | length - (unique | length)'
  [ "$JQ_OUT" = 0 ] || fail "changelog has $JQ_OUT duplicate version entries"

  jqr '[.versions[] | (.features + .fixes + .other)[] | select(startswith("Merge"))] | length'
  [ "$JQ_OUT" = 0 ] || fail "changelog leaks $JQ_OUT merge-commit subject(s)"

  jqr '.versions[0].version // ""'
  newest="$JQ_OUT"
  if [ -n "$EXPECT_NEWEST" ]; then
    [ "$newest" = "$EXPECT_NEWEST" ] || fail "newest changelog entry is '$newest', expected '$EXPECT_NEWEST'"
  fi
  log "OK: changelog has $count entries, newest '${newest:-none}', no duplicates or merge subjects"
}

# assert_status EXPECT(shown|hidden) TOKEN LABEL
assert_status() {
  local expect="$1" token="$2" label="$3" show version
  api GET /api/v1/changelog/status "$token"
  [ "$API_CODE" = 200 ] || fail "$label: changelog/status returned HTTP $API_CODE"
  jqr '.show_changelog'
  show="$JQ_OUT"

  if [ "$expect" = hidden ]; then
    [ "$show" = false ] || fail "$label: expected the modal hidden, show_changelog=$show"
    log "OK: $label hidden"
    return 0
  fi

  [ "$show" = true ] || fail "$label: expected the modal shown, show_changelog=$show"
  jqr '.versions | length'
  [ "$JQ_OUT" = 3 ] || fail "$label: expected exactly 3 entries, got $JQ_OUT"
  jqr '.versions[].version'
  while IFS= read -r version; do
    semver_le "$version" "$RUNNING_BASE" || fail "$label: entry $version is newer than the running base $RUNNING_BASE"
  done <<< "$JQ_OUT"
  log "OK: $label shows 3 entries, all <= $RUNNING_BASE"
}

check_unseeded() {
  local expect="$1" user_email user_pass user_body changed user_token i
  user_email="${NAME_PREFIX}${RAND}-user@example.com"
  user_pass="$(gen_password)"
  user_body="$(jq -cn --arg e "$user_email" --arg p "$user_pass" \
    '{name:"Smoke User", email:$e, password:$p}')"

  api POST /api/v1/users "$TOKEN" "$user_body"
  expect_2xx "create test user"

  # The image ships sqlite3 at /usr/bin; the change is visible to the next request.
  if ! changed="$(docker exec "$CTR" sqlite3 "$DB_PATH" \
      "UPDATE users SET last_seen_version='' WHERE email='$user_email'; SELECT changes();" 2>&1)"; then
    fail "sqlite3 update failed: $changed"
  fi
  [ "$changed" = 1 ] || fail "expected to clear last_seen_version on 1 row, changed $changed"

  login "$user_email" "$user_pass"
  user_token="$TOKEN"
  assert_status "$expect" "$user_token" "unseeded user, first login"

  api POST /api/v1/changelog/ack "$user_token" '{"action":"dismiss_permanent"}'
  expect_2xx "dismiss_permanent"
  assert_status hidden "$user_token" "unseeded user, after dismiss_permanent"

  # Two more sign-ins: the modal must not come back (no nag loop).
  for i in 1 2; do
    login "$user_email" "$user_pass"
    assert_status hidden "$TOKEN" "unseeded user, after dismiss_permanent, sign-in $i"
  done
}

main() {
  parse_args "$@"
  validate_args
  check_dependencies

  RAND="$(rand_hex 6)"
  CTR="${NAME_PREFIX}${RAND}"
  # AES-256 key: 32 random bytes, base64-encoded (see backend/internal/crypto).
  CHARON_ENCRYPTION_KEY="$(head -c 32 /dev/urandom | base64 -w0)"
  WORKDIR="$(mktemp -d)"
  BODY_FILE="$WORKDIR/body.json"

  trap cleanup EXIT
  trap 'exit 130' INT
  trap 'exit 143' TERM

  log "docker host: ${DOCKER_HOST:-<default>}"
  log "image: $IMAGE"

  prepare_volume
  start_container

  wait_for_health
  check_version
  check_commit

  setup_and_login
  check_changelog_all

  if [ -n "$EXPECT_UNSEEDED" ]; then
    check_unseeded "$EXPECT_UNSEEDED"
  fi

  log "PASS: all smoke checks passed for $IMAGE"
}

main "$@"
