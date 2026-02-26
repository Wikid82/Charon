#!/usr/bin/env bash
set -euo pipefail
# prune-dockerhub.sh
# Deletes old container images from Docker Hub according to retention and protection rules.

OWNER=${OWNER:-${GITHUB_REPOSITORY_OWNER:-Wikid82}}
IMAGE_NAME=${IMAGE_NAME:-charon}

KEEP_DAYS=${KEEP_DAYS:-30}
KEEP_LAST_N=${KEEP_LAST_N:-30}

DRY_RUN=${DRY_RUN:-false}
PROTECTED_REGEX=${PROTECTED_REGEX:-'["^v","^latest$","^main$","^develop$"]'}

DOCKERHUB_USERNAME=${DOCKERHUB_USERNAME:-}
DOCKERHUB_TOKEN=${DOCKERHUB_TOKEN:-}

LOG_PREFIX="[prune-dockerhub]"

cutoff_ts=$(date -d "$KEEP_DAYS days ago" +%s 2>/dev/null || date -d "-$KEEP_DAYS days" +%s)

dry_run=false
case "${DRY_RUN,,}" in
  true|1|yes|y|on) dry_run=true ;;
  *) dry_run=false ;;
esac

TOTAL_CANDIDATES=0
TOTAL_CANDIDATES_BYTES=0
TOTAL_DELETED=0
TOTAL_DELETED_BYTES=0

echo "$LOG_PREFIX starting with OWNER=$OWNER IMAGE_NAME=$IMAGE_NAME KEEP_DAYS=$KEEP_DAYS KEEP_LAST_N=$KEEP_LAST_N DRY_RUN=$dry_run"
echo "$LOG_PREFIX PROTECTED_REGEX=$PROTECTED_REGEX"

require() {
  command -v "$1" >/dev/null 2>&1 || { echo "$LOG_PREFIX missing required command: $1" >&2; exit 1; }
}
require curl
require jq

is_protected_tag() {
  local tag="$1"
  local rgx
  while IFS= read -r rgx; do
    [[ -z "$rgx" ]] && continue
    if [[ "$tag" =~ $rgx ]]; then
      return 0
    fi
  done < <(echo "$PROTECTED_REGEX" | jq -r '.[]')
  return 1
}

human_readable() {
  local bytes=${1:-0}
  if [[ -z "$bytes" ]] || (( bytes <= 0 )); then
    echo "0 B"
    return
  fi
  local unit=(B KiB MiB GiB TiB)
  local i=0
  local value=$bytes
  while (( value > 1024 )) && (( i < 4 )); do
    value=$((value / 1024))
    i=$((i + 1))
  done
  printf "%s %s" "${value}" "${unit[$i]}"
}

action_delete_dockerhub() {
  echo "$LOG_PREFIX -> Docker Hub cleanup for ${DOCKERHUB_USERNAME:-<unset>}/$IMAGE_NAME (dry-run=$dry_run)"

  if [[ -z "${DOCKERHUB_USERNAME:-}" || -z "${DOCKERHUB_TOKEN:-}" ]]; then
    echo "$LOG_PREFIX Docker Hub credentials not set; skipping Docker Hub cleanup"
    return
  fi

  local hub_token page page_size all resp results_count total
  local keep_tags tag tag_name last_updated last_ts protected bytes

  hub_token=$(printf '{"username":"%s","password":"%s"}' "$DOCKERHUB_USERNAME" "$DOCKERHUB_TOKEN" | \
    curl -sS -X POST -H "Content-Type: application/json" --data-binary @- \
    https://hub.docker.com/v2/users/login/ | jq -r '.token')

  if [[ -z "$hub_token" || "$hub_token" == "null" ]]; then
    echo "$LOG_PREFIX Failed to obtain Docker Hub token; aborting Docker Hub cleanup"
    return
  fi

  page=1
  page_size=100
  all='[]'
  while :; do
    resp=$(curl -sS -H "Authorization: JWT $hub_token" \
      "https://hub.docker.com/v2/repositories/${DOCKERHUB_USERNAME}/${IMAGE_NAME}/tags?page_size=$page_size&page=$page")

    results_count=$(echo "$resp" | jq -r '.results | length')
    if [[ -z "$results_count" || "$results_count" == "0" ]]; then
      break
    fi

    all=$(jq -s '.[0] + .[1].results' <(echo "$all") <(echo "$resp"))
    ((page++))
  done

  total=$(echo "$all" | jq -r 'length')
  if [[ -z "$total" || "$total" == "0" ]]; then
    echo "$LOG_PREFIX Docker Hub: no tags found"
    return
  fi

  echo "$LOG_PREFIX Docker Hub: fetched $total tags total"

  keep_tags=$(echo "$all" | jq -r --argjson n "${KEEP_LAST_N:-0}" '
    (sort_by(.last_updated) | reverse) as $s
    | ($s[0:$n] | map(.name)) | join(" ")
  ')

  while IFS= read -r tag; do
    tag_name=$(echo "$tag" | jq -r '.name')
    last_updated=$(echo "$tag" | jq -r '.last_updated')
    last_ts=$(date -d "$last_updated" +%s 2>/dev/null || echo 0)

    if [[ -n "$keep_tags" && " $keep_tags " == *" $tag_name "* ]]; then
      echo "$LOG_PREFIX keep (last_n): tag=$tag_name last_updated=$last_updated"
      continue
    fi

    protected=false
    if is_protected_tag "$tag_name"; then
      protected=true
    fi
    if $protected; then
      echo "$LOG_PREFIX keep (protected): tag=$tag_name last_updated=$last_updated"
      continue
    fi

    if (( last_ts >= cutoff_ts )); then
      echo "$LOG_PREFIX keep (recent): tag=$tag_name last_updated=$last_updated"
      continue
    fi

    echo "$LOG_PREFIX candidate: tag=$tag_name last_updated=$last_updated"

    bytes=$(echo "$tag" | jq -r '.images | map(.size) | add // 0' 2>/dev/null || echo 0)
    TOTAL_CANDIDATES=$((TOTAL_CANDIDATES + 1))
    TOTAL_CANDIDATES_BYTES=$((TOTAL_CANDIDATES_BYTES + bytes))

    if $dry_run; then
      echo "$LOG_PREFIX DRY RUN: would delete Docker Hub tag=$tag_name (approx ${bytes} bytes)"
    else
      echo "$LOG_PREFIX deleting Docker Hub tag=$tag_name (approx ${bytes} bytes)"
      curl -sS -X DELETE -H "Authorization: JWT $hub_token" \
        "https://hub.docker.com/v2/repositories/${DOCKERHUB_USERNAME}/${IMAGE_NAME}/tags/${tag_name}/" >/dev/null || true
      TOTAL_DELETED=$((TOTAL_DELETED + 1))
      TOTAL_DELETED_BYTES=$((TOTAL_DELETED_BYTES + bytes))
    fi

  done < <(echo "$all" | jq -c 'sort_by(.last_updated) | .[]')
}

# Main
action_delete_dockerhub

echo "$LOG_PREFIX SUMMARY: total_candidates=${TOTAL_CANDIDATES} total_candidates_bytes=${TOTAL_CANDIDATES_BYTES} total_deleted=${TOTAL_DELETED} total_deleted_bytes=${TOTAL_DELETED_BYTES}"
echo "$LOG_PREFIX SUMMARY_HUMAN: candidates=${TOTAL_CANDIDATES} candidates_size=$(human_readable "${TOTAL_CANDIDATES_BYTES}") deleted=${TOTAL_DELETED} deleted_size=$(human_readable "${TOTAL_DELETED_BYTES}")"

: > prune-summary-dockerhub.env
echo "TOTAL_CANDIDATES=${TOTAL_CANDIDATES}" >> prune-summary-dockerhub.env
echo "TOTAL_CANDIDATES_BYTES=${TOTAL_CANDIDATES_BYTES}" >> prune-summary-dockerhub.env
echo "TOTAL_DELETED=${TOTAL_DELETED}" >> prune-summary-dockerhub.env
echo "TOTAL_DELETED_BYTES=${TOTAL_DELETED_BYTES}" >> prune-summary-dockerhub.env

echo "$LOG_PREFIX done"
