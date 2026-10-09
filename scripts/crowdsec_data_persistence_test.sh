#!/usr/bin/env bash
set -euo pipefail

# Brief: Integration test for CrowdSec hub data file persistence (container recreation).
#
# CrowdSec hub items live under /etc/crowdsec (symlinked into the /app/data volume) but their
# data files (blocklists, GeoLite2 databases, ...) live in CrowdSec's data_dir. The entrypoint
# redirects data_dir onto the volume and, only when installed items are missing data files,
# runs a bounded `cscli hub upgrade`.
#
# Scenarios (all use a throwaway named volume and no published ports):
#   1. Fresh volume: items really install, data_dir is on the volume, data files exist,
#      GeoLite2-City.mmdb exists, `crowdsec -t` reports no data-init errors.
#   2. Recreate container with the same volume: files present, `cscli hub upgrade` NOT run.
#   3. Recreate with data files deleted from the volume: `cscli hub upgrade` runs once and the
#      files are restored.
#   4. Legacy volume (config.yaml still pointing at /var/lib/crowdsec/data, no data files):
#      data_dir is migrated and the files are restored.
#
# Requires network access (CrowdSec hub) and a charon image (default: charon:local).

PROJECT_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$PROJECT_ROOT"

IMAGE="${CHARON_TEST_IMAGE:-charon:local}"
CONTAINER_NAME="charon-crowdsec-persist-test"
VOLUME_NAME="charon_crowdsec_persist_data"
INIT_TIMEOUT_SECONDS="${INIT_TIMEOUT_SECONDS:-300}"
DATA_DIR="/app/data/crowdsec/data"
CONFIG_YAML="/app/data/crowdsec/config/config.yaml"
INIT_DONE_MARKER="CrowdSec configuration initialized"
UPGRADE_MARKER="Running 'cscli hub upgrade'"

RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() { echo -e "${GREEN}[INFO]${NC} $1"; }
log_test() { echo -e "${BLUE}[TEST]${NC} $1"; }

PASSED=0
FAILED=0

pass_test() {
    PASSED=$((PASSED + 1))
    echo -e "  ${GREEN}✓ PASS${NC}"
}

fail_test() {
    FAILED=$((FAILED + 1))
    echo -e "  ${RED}✗ FAIL${NC}: $1"
}

cleanup() {
    log_info "Cleaning up test resources..."
    docker rm -f "${CONTAINER_NAME}" >/dev/null 2>&1 || true
    docker volume rm "${VOLUME_NAME}" >/dev/null 2>&1 || true
}
trap cleanup EXIT

if ! command -v docker >/dev/null 2>&1; then
    echo "docker is not available; aborting" >&2
    exit 1
fi

if ! docker image inspect "${IMAGE}" >/dev/null 2>&1; then
    log_info "Building ${IMAGE}..."
    docker build -t "${IMAGE}" .
fi

# container_logs: print the container logs.
container_logs() {
    docker logs "${CONTAINER_NAME}" 2>&1
}

# logs_contain: succeed if the container logs contain the fixed string. The logs are buffered
# first so `grep -q` closing a pipe early cannot trip pipefail.
logs_contain() {
    local out
    out="$(container_logs)"
    grep -qF -- "$1" <<<"${out}"
}

# start_container: (re)create the test container on the shared volume and wait for hub init.
start_container() {
    docker rm -f "${CONTAINER_NAME}" >/dev/null 2>&1 || true
    docker run -d --name "${CONTAINER_NAME}" \
        -e CHARON_ENV=development \
        -v "${VOLUME_NAME}:/app/data" \
        "${IMAGE}" >/dev/null

    local waited=0
    until logs_contain "${INIT_DONE_MARKER}"; do
        if [ "${waited}" -ge "${INIT_TIMEOUT_SECONDS}" ]; then
            echo "Timed out after ${INIT_TIMEOUT_SECONDS}s waiting for CrowdSec init" >&2
            docker logs "${CONTAINER_NAME}" 2>&1 | tail -40 >&2
            exit 1
        fi
        if [ "$(docker inspect -f '{{.State.Running}}' "${CONTAINER_NAME}")" != "true" ]; then
            echo "Container exited during startup" >&2
            docker logs "${CONTAINER_NAME}" 2>&1 | tail -40 >&2
            exit 1
        fi
        sleep 3
        waited=$((waited + 3))
    done
}

# volume_sh: run a shell snippet against the volume without starting Charon.
volume_sh() {
    docker run --rm --user 0 --entrypoint sh -v "${VOLUME_NAME}:/app/data" "${IMAGE}" -c "$1"
}

# declared_data_files: data files declared by installed hub items (one per line).
declared_data_files() {
    docker exec "${CONTAINER_NAME}" sh -c \
        "find -L /etc/crowdsec/parsers /etc/crowdsec/scenarios /etc/crowdsec/postoverflows -type f -name '*.yaml' -exec grep -h 'dest_file:' {} + 2>/dev/null | sed -e 's/^.*dest_file:[[:space:]]*//' -e 's/[[:space:]]*\$//' | sort -u"
}

# missing_data_files: declared data files that are absent or empty in data_dir.
missing_data_files() {
    local f
    declared_data_files | while IFS= read -r f; do
        [ -n "${f}" ] || continue
        docker exec "${CONTAINER_NAME}" test -s "${DATA_DIR}/${f}" || echo "${f}"
    done
}

count_upgrades() {
    container_logs | grep -c "${UPGRADE_MARKER}" || true
}

# crowdsec_data_errors: run the CrowdSec agent briefly (it is not auto-started by the entrypoint)
# and print any data-file / GeoIP initialization errors from its log.
crowdsec_data_errors() {
    docker exec "${CONTAINER_NAME}" sh -c \
        'rm -f /var/log/crowdsec/crowdsec.log; timeout 20 crowdsec -c /etc/crowdsec/config.yaml >/dev/null 2>&1; grep -Ei "unable to init data for file|unable to initialize GeoIP" /var/log/crowdsec/crowdsec.log || true'
}

echo "=============================================="
echo "=== CrowdSec Data Persistence Test ==="
echo "=============================================="

docker rm -f "${CONTAINER_NAME}" >/dev/null 2>&1 || true
docker volume rm "${VOLUME_NAME}" >/dev/null 2>&1 || true

# ----------------------------------------------------------------------------
log_info "Scenario 1: fresh volume"
start_container

log_test "Check 1: install guard really installs items on a fresh volume"
PROBING_JSON="$(docker exec "${CONTAINER_NAME}" cscli scenarios inspect crowdsecurity/http-probing -o json)"
if logs_contain "Installing http-probing" &&
    grep -q '"installed": *true' <<<"${PROBING_JSON}"; then
    pass_test
else
    fail_test "http-probing was not installed on a fresh volume"
fi

log_test "Check 2: data_dir points at the persistent volume"
if docker exec "${CONTAINER_NAME}" grep -q "data_dir: ${DATA_DIR}/" "${CONFIG_YAML}"; then
    pass_test
else
    fail_test "data_dir is not ${DATA_DIR}/ in config.yaml"
fi

log_test "Check 3: hub data files and GeoLite2-City.mmdb exist in data_dir"
DECLARED="$(declared_data_files)"
if [ -z "${DECLARED}" ]; then
    fail_test "installed hub items declare no data files (expected some)"
elif [ -n "$(missing_data_files)" ]; then
    fail_test "missing data files: $(missing_data_files | tr '\n' ' ')"
elif ! docker exec "${CONTAINER_NAME}" test -s "${DATA_DIR}/GeoLite2-City.mmdb"; then
    fail_test "GeoLite2-City.mmdb is missing"
else
    pass_test
fi

log_test "Check 4: hub upgrade not run on a fresh volume with intact data files"
if [ "$(count_upgrades)" -eq 0 ]; then
    pass_test
else
    fail_test "hub upgrade ran on first start"
fi

log_test "Check 5: crowdsec.log has no data/GeoIP initialization errors"
CS_ERRORS="$(crowdsec_data_errors)"
if [ -n "${CS_ERRORS}" ]; then
    fail_test "crowdsec reported data/GeoIP init errors"
    echo "${CS_ERRORS}" | head -5
else
    pass_test
fi

# ----------------------------------------------------------------------------
log_info "Scenario 2: recreate with intact data (healthy restart)"
start_container

log_test "Check 6: data files still present and hub upgrade NOT run"
if [ -n "$(missing_data_files)" ]; then
    fail_test "data files missing after recreation: $(missing_data_files | tr '\n' ' ')"
elif [ "$(count_upgrades)" -ne 0 ]; then
    fail_test "hub upgrade ran although all data files were present"
else
    pass_test
fi

# ----------------------------------------------------------------------------
log_info "Scenario 3: recreate after data files were lost"
docker rm -f "${CONTAINER_NAME}" >/dev/null
volume_sh "find ${DATA_DIR} -mindepth 1 -maxdepth 1 ! -name 'crowdsec.db*' -exec rm -rf {} +"
start_container

log_test "Check 7: hub upgrade runs once, restores the data files and CrowdSec loads cleanly"
if [ "$(count_upgrades)" -ne 1 ]; then
    fail_test "expected exactly one hub upgrade, got $(count_upgrades)"
elif [ -n "$(missing_data_files)" ]; then
    fail_test "data files still missing: $(missing_data_files | tr '\n' ' ')"
elif [ -n "$(crowdsec_data_errors)" ]; then
    fail_test "crowdsec reported data/GeoIP init errors after restore"
else
    pass_test
fi

# ----------------------------------------------------------------------------
log_info "Scenario 4: legacy volume (data_dir still at the old location)"
docker rm -f "${CONTAINER_NAME}" >/dev/null
volume_sh "sed -i 's|data_dir: ${DATA_DIR}/|data_dir: /var/lib/crowdsec/data/|' ${CONFIG_YAML} && find ${DATA_DIR} -mindepth 1 -maxdepth 1 ! -name 'crowdsec.db*' -exec rm -rf {} +"
start_container

log_test "Check 8: data_dir migrated and data files restored"
if ! docker exec "${CONTAINER_NAME}" grep -q "data_dir: ${DATA_DIR}/" "${CONFIG_YAML}"; then
    fail_test "data_dir was not migrated to ${DATA_DIR}/"
elif [ -n "$(missing_data_files)" ]; then
    fail_test "data files missing after migration: $(missing_data_files | tr '\n' ' ')"
else
    pass_test
fi

echo ""
echo "=============================================="
echo "Passed: ${PASSED}  Failed: ${FAILED}"
echo "=============================================="

if [ "${FAILED}" -ne 0 ]; then
    exit 1
fi
