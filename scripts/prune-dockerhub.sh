#!/usr/bin/env bash
# shellcheck disable=SC2154  # dry_run, cutoff_ts, build_cutoff_ts come from lib/prune-common.sh
set -euo pipefail
# prune-dockerhub.sh
# Deletes old container images from Docker Hub according to retention and protection rules.
#
# Retention classes (see scripts/lib/prune-common.sh):
#   protected - PROTECTED_REGEX tags (semver, latest, main, develop): never deleted
#   build     - BUILD_TAG_REGEX tags (build-*, sha256-*, <branch>-<sha>, bare sha):
#               deleted after BUILD_KEEP_DAYS; exempt from the KEEP_LAST_N shield
#   aged      - everything else (nightly-*, pr-*, ...): deleted after KEEP_DAYS,
#               except the KEEP_LAST_N newest
#
# Exits 1 when any delete is refused, so a token without delete rights fails the
# job instead of being reported as success.
#
# Not implemented: pruning of untagged manifests. Docker Hub's tags API never
# lists them, and no documented, verifiable API exists to enumerate or delete
# them (see docs/ci/container-prune.md).

OWNER=${OWNER:-${GITHUB_REPOSITORY_OWNER:-Wikid82}}
IMAGE_NAME=${IMAGE_NAME:-charon}

KEEP_DAYS=${KEEP_DAYS:-30}
KEEP_LAST_N=${KEEP_LAST_N:-30}
BUILD_KEEP_DAYS=${BUILD_KEEP_DAYS:-7}

DRY_RUN=${DRY_RUN:-false}

DOCKERHUB_USERNAME=${DOCKERHUB_USERNAME:-}
DOCKERHUB_TOKEN=${DOCKERHUB_TOKEN:-}

LOG_PREFIX="[prune-dockerhub]"

# shellcheck source=scripts/lib/prune-common.sh
source "$(dirname "${BASH_SOURCE[0]}")/lib/prune-common.sh"

require curl
require jq
prune_init_state
load_tag_patterns

echo "$LOG_PREFIX starting with OWNER=$OWNER IMAGE_NAME=$IMAGE_NAME KEEP_DAYS=$KEEP_DAYS BUILD_KEEP_DAYS=$BUILD_KEEP_DAYS KEEP_LAST_N=$KEEP_LAST_N DRY_RUN=$dry_run"
echo "$LOG_PREFIX PROTECTED_REGEX=$PROTECTED_REGEX"
echo "$LOG_PREFIX BUILD_TAG_REGEX=$BUILD_TAG_REGEX"

# Docker Hub JWTs are short-lived and a large prune outlasts one, so login is a
# function: registry_delete re-runs it when a delete comes back 401.
hub_token=""
dockerhub_login() {
  hub_token=$(printf '{"username":"%s","password":"%s"}' "$DOCKERHUB_USERNAME" "$DOCKERHUB_TOKEN" | \
    curl -sS -X POST -H "Content-Type: application/json" --data-binary @- \
    https://hub.docker.com/v2/users/login/ | jq -r '.token' 2>/dev/null || true)
  [[ -n "$hub_token" && "$hub_token" != "null" ]]
}

dockerhub_auth_header() { echo "Authorization: JWT $hub_token"; }

# shellcheck disable=SC2034  # read by registry_delete in lib/prune-common.sh
REGISTRY_AUTH_HEADER_FN=dockerhub_auth_header
# shellcheck disable=SC2034  # read by registry_delete in lib/prune-common.sh
REGISTRY_REAUTH_FN=dockerhub_login

action_delete_dockerhub() {
  echo "$LOG_PREFIX -> Docker Hub cleanup for ${DOCKERHUB_USERNAME:-<unset>}/$IMAGE_NAME (dry-run=$dry_run)"

  if [[ -z "${DOCKERHUB_USERNAME:-}" || -z "${DOCKERHUB_TOKEN:-}" ]]; then
    echo "$LOG_PREFIX Docker Hub credentials not set; skipping Docker Hub cleanup"
    return
  fi

  local page page_size all resp results_count total
  local kept_aged tag_name last_updated bytes last_ts class

  if ! dockerhub_login; then
    echo "$LOG_PREFIX Failed to obtain Docker Hub token; aborting Docker Hub cleanup" >&2
    TOTAL_FAILED=$((TOTAL_FAILED + 1))
    return
  fi

  page=1
  page_size=100
  all='[]'
  while :; do
    resp=$(curl -sS -H "Authorization: JWT $hub_token" \
      "https://hub.docker.com/v2/repositories/${DOCKERHUB_USERNAME}/${IMAGE_NAME}/tags?page_size=$page_size&page=$page" || true)

    results_count=$(echo "$resp" | jq -r '(.results // []) | length' 2>/dev/null || echo 0)
    if [[ -z "$results_count" || "$results_count" == "0" ]]; then
      break
    fi

    all=$(jq -s '.[0] + .[1].results' <(echo "$all") <(echo "$resp"))
    page=$((page + 1))
  done

  total=$(echo "$all" | jq -r 'length')
  if [[ -z "$total" || "$total" == "0" ]]; then
    echo "$LOG_PREFIX Docker Hub: no tags found"
    return
  fi

  echo "$LOG_PREFIX Docker Hub: fetched $total tags total"

  # Newest first, so the first KEEP_LAST_N "aged" tags are the shielded ones.
  kept_aged=0
  while IFS=$'\t' read -r tag_name last_updated bytes; do
    last_ts=$(date -d "$last_updated" +%s 2>/dev/null || echo 0)
    class=$(classify_tags "$tag_name")

    case "$class" in
      protected)
        echo "$LOG_PREFIX keep (protected): tag=$tag_name last_updated=$last_updated"
        continue
        ;;
      build)
        if (( last_ts >= build_cutoff_ts )); then
          echo "$LOG_PREFIX keep (recent build): tag=$tag_name last_updated=$last_updated"
          continue
        fi
        ;;
      *)
        if (( kept_aged < KEEP_LAST_N )); then
          kept_aged=$((kept_aged + 1))
          echo "$LOG_PREFIX keep (last_n): tag=$tag_name last_updated=$last_updated"
          continue
        fi
        if (( last_ts >= cutoff_ts )); then
          echo "$LOG_PREFIX keep (recent): tag=$tag_name last_updated=$last_updated"
          continue
        fi
        ;;
    esac

    echo "$LOG_PREFIX candidate ($class): tag=$tag_name last_updated=$last_updated"
    TOTAL_CANDIDATES=$((TOTAL_CANDIDATES + 1))
    TOTAL_CANDIDATES_BYTES=$((TOTAL_CANDIDATES_BYTES + bytes))

    if $dry_run; then
      echo "$LOG_PREFIX DRY RUN: would delete Docker Hub tag=$tag_name (approx ${bytes} bytes)"
    else
      echo "$LOG_PREFIX deleting Docker Hub tag=$tag_name (approx ${bytes} bytes)"
      registry_delete "Docker Hub tag=$tag_name" "$bytes" \
        "https://hub.docker.com/v2/repositories/${DOCKERHUB_USERNAME}/${IMAGE_NAME}/tags/${tag_name}/"
    fi
  done < <(echo "$all" | jq -r 'sort_by(.last_updated) | reverse | .[]
    | [.name, .last_updated, ((.images // []) | map(.size // 0) | add // 0)] | @tsv')
}

# Main
action_delete_dockerhub

echo "$LOG_PREFIX SUMMARY: total_candidates=${TOTAL_CANDIDATES} total_candidates_bytes=${TOTAL_CANDIDATES_BYTES} total_deleted=${TOTAL_DELETED} total_deleted_bytes=${TOTAL_DELETED_BYTES} total_failed=${TOTAL_FAILED}"
echo "$LOG_PREFIX SUMMARY_HUMAN: candidates=${TOTAL_CANDIDATES} candidates_size=$(human_readable "${TOTAL_CANDIDATES_BYTES}") deleted=${TOTAL_DELETED} deleted_size=$(human_readable "${TOTAL_DELETED_BYTES}") failed=${TOTAL_FAILED}"

{
  echo "TOTAL_CANDIDATES=${TOTAL_CANDIDATES}"
  echo "TOTAL_CANDIDATES_BYTES=${TOTAL_CANDIDATES_BYTES}"
  echo "TOTAL_DELETED=${TOTAL_DELETED}"
  echo "TOTAL_DELETED_BYTES=${TOTAL_DELETED_BYTES}"
  echo "TOTAL_FAILED=${TOTAL_FAILED}"
} > prune-summary-dockerhub.env

if (( TOTAL_FAILED > 0 )); then
  echo "$LOG_PREFIX ERROR: ${TOTAL_FAILED} operation(s) failed; check that DOCKERHUB_TOKEN has delete permission (Read, Write, Delete); a 401 that persists after re-login means the token is invalid or revoked" >&2
  exit 1
fi

echo "$LOG_PREFIX done"
