#!/usr/bin/env bash
# shellcheck disable=SC2154  # dry_run, cutoff_ts, build_cutoff_ts come from lib/prune-common.sh
set -euo pipefail
# prune-ghcr.sh
# Deletes old container images from GitHub Container Registry (GHCR)
# according to retention and protection rules.
#
# Retention classes (see scripts/lib/prune-common.sh):
#   protected - any tag matches PROTECTED_REGEX (semver, latest, main, develop): never deleted
#   build     - every tag matches BUILD_TAG_REGEX (build-*, sha256-*, <branch>-<sha>, bare sha):
#               deleted after BUILD_KEEP_DAYS; exempt from the KEEP_LAST_N shield
#   aged      - any other tagged version (nightly-*, pr-*, ...): deleted after KEEP_DAYS,
#               except the KEEP_LAST_N newest
#   untagged  - deleted after KEEP_DAYS when PRUNE_UNTAGGED=true (not subject to KEEP_LAST_N)
#
# Exits 1 when any delete is refused. The GHCR versions API reports no size,
# so no byte accounting is done.

OWNER=${OWNER:-${GITHUB_REPOSITORY_OWNER:-Wikid82}}
IMAGE_NAME=${IMAGE_NAME:-charon}

KEEP_DAYS=${KEEP_DAYS:-30}
KEEP_LAST_N=${KEEP_LAST_N:-30}
BUILD_KEEP_DAYS=${BUILD_KEEP_DAYS:-7}

DRY_RUN=${DRY_RUN:-false}
PRUNE_UNTAGGED=${PRUNE_UNTAGGED:-true}

LOG_PREFIX="[prune-ghcr]"

# shellcheck source=scripts/lib/prune-common.sh
source "$(dirname "${BASH_SOURCE[0]}")/lib/prune-common.sh"

require curl
require jq
prune_init_state
load_tag_patterns

echo "$LOG_PREFIX starting with OWNER=$OWNER IMAGE_NAME=$IMAGE_NAME KEEP_DAYS=$KEEP_DAYS BUILD_KEEP_DAYS=$BUILD_KEEP_DAYS KEEP_LAST_N=$KEEP_LAST_N DRY_RUN=$dry_run"
echo "$LOG_PREFIX PROTECTED_REGEX=$PROTECTED_REGEX PRUNE_UNTAGGED=$PRUNE_UNTAGGED"
echo "$LOG_PREFIX BUILD_TAG_REGEX=$BUILD_TAG_REGEX"

# All echo/log statements go to stderr so stdout remains pure JSON
ghcr_list_all_versions_json() {
  local namespace_type="$1"
  local page=1
  local per_page=100
  local all='[]'

  while :; do
    local url="https://api.github.com/${namespace_type}/${OWNER}/packages/container/${IMAGE_NAME}/versions?per_page=$per_page&page=$page"

    local resp
    resp=$(curl -sS \
      -H "Authorization: Bearer $GITHUB_TOKEN" \
      -H "Accept: application/vnd.github+json" \
      -H "X-GitHub-Api-Version: 2022-11-28" \
      "$url" || true)

    if ! echo "$resp" | jq -e . >/dev/null 2>&1; then
      echo "$LOG_PREFIX GHCR returned non-JSON for url=$url" >&2
      echo "$LOG_PREFIX GHCR response (first 200 chars): $(echo "$resp" | head -c 200 | tr '\n' ' ')" >&2
      echo "[]"
      return 0
    fi

    if echo "$resp" | jq -e 'type == "object" and has("message")' >/dev/null 2>&1; then
      local msg doc
      msg=$(echo "$resp" | jq -r '.message')

      if [[ "$msg" == "Not Found" ]]; then
        echo "$LOG_PREFIX GHCR ${namespace_type} endpoint returned Not Found" >&2
        echo "[]"
        return 0
      fi

      echo "$LOG_PREFIX GHCR API error: $msg" >&2
      doc=$(echo "$resp" | jq -r '.documentation_url // empty')
      [[ -n "$doc" ]] && echo "$LOG_PREFIX GHCR docs: $doc" >&2
      echo "[]"
      return 0
    fi

    local count
    count=$(echo "$resp" | jq -r 'length')
    if [[ -z "$count" || "$count" == "0" ]]; then
      break
    fi

    all=$(jq -s 'add' <(echo "$all") <(echo "$resp"))
    page=$((page + 1))
  done

  echo "$all"
}

action_delete_ghcr() {
  echo "$LOG_PREFIX -> GHCR cleanup for $OWNER/$IMAGE_NAME (dry-run=$dry_run)"

  if [[ -z "${GITHUB_TOKEN:-}" ]]; then
    echo "$LOG_PREFIX GITHUB_TOKEN not set; skipping GHCR cleanup"
    return
  fi

  local all total
  local namespace_type="orgs"
  all=$(ghcr_list_all_versions_json "$namespace_type")
  if [[ "$(echo "$all" | jq -r 'length')" == "0" ]]; then
    namespace_type="users"
    all=$(ghcr_list_all_versions_json "$namespace_type")
  fi

  total=$(echo "$all" | jq -r 'length')
  if [[ -z "$total" || "$total" == "0" ]]; then
    echo "$LOG_PREFIX GHCR: no versions found (or insufficient access)."
    return
  fi

  echo "$LOG_PREFIX GHCR: fetched $total versions total"

  local kept_aged=0 id created created_ts tags_csv class
  local -a tags

  # Newest first, so the first KEEP_LAST_N "aged" versions are the shielded ones.
  while IFS=$'\t' read -r id created created_ts tags_csv; do
    tags=()
    [[ -n "$tags_csv" ]] && IFS=',' read -ra tags <<< "$tags_csv"
    class=$(classify_tags "${tags[@]}")
    [[ -z "$tags_csv" ]] && tags_csv="<none>"

    case "$class" in
      protected)
        echo "$LOG_PREFIX keep (protected): id=$id tags=$tags_csv created=$created"
        continue
        ;;
      build)
        if (( created_ts >= build_cutoff_ts )); then
          echo "$LOG_PREFIX keep (recent build): id=$id tags=$tags_csv created=$created"
          continue
        fi
        ;;
      untagged)
        if [[ "${PRUNE_UNTAGGED,,}" != "true" ]]; then
          echo "$LOG_PREFIX keep (untagged disabled): id=$id created=$created"
          continue
        fi
        if (( created_ts >= cutoff_ts )); then
          echo "$LOG_PREFIX keep (recent): id=$id tags=$tags_csv created=$created"
          continue
        fi
        ;;
      *)
        if (( kept_aged < KEEP_LAST_N )); then
          kept_aged=$((kept_aged + 1))
          echo "$LOG_PREFIX keep (last_n): id=$id tags=$tags_csv created=$created"
          continue
        fi
        if (( created_ts >= cutoff_ts )); then
          echo "$LOG_PREFIX keep (recent): id=$id tags=$tags_csv created=$created"
          continue
        fi
        ;;
    esac

    echo "$LOG_PREFIX candidate ($class): id=$id tags=$tags_csv created=$created"
    TOTAL_CANDIDATES=$((TOTAL_CANDIDATES + 1))

    if $dry_run; then
      echo "$LOG_PREFIX DRY RUN: would delete GHCR version id=$id"
    else
      echo "$LOG_PREFIX deleting GHCR version id=$id"
      registry_delete "GHCR version id=$id" 0 \
        "https://api.github.com/${namespace_type}/${OWNER}/packages/container/${IMAGE_NAME}/versions/$id" \
        -H "Authorization: Bearer $GITHUB_TOKEN" \
        -H "Accept: application/vnd.github+json" \
        -H "X-GitHub-Api-Version: 2022-11-28"
    fi
  done < <(echo "$all" | jq -r 'map({
      id: .id,
      created_at: .created_at,
      created_ts: (.created_at | fromdateiso8601),
      tags_csv: ((.metadata.container.tags // []) | join(","))
    })
    | sort_by(.created_ts) | reverse | .[]
    | [.id, .created_at, .created_ts, .tags_csv] | @tsv')
}

# Main
action_delete_ghcr

echo "$LOG_PREFIX SUMMARY: total_candidates=${TOTAL_CANDIDATES} total_deleted=${TOTAL_DELETED} total_failed=${TOTAL_FAILED}"
echo "$LOG_PREFIX SUMMARY_HUMAN: candidates=${TOTAL_CANDIDATES} deleted=${TOTAL_DELETED} failed=${TOTAL_FAILED}"

{
  echo "TOTAL_CANDIDATES=${TOTAL_CANDIDATES}"
  echo "TOTAL_DELETED=${TOTAL_DELETED}"
  echo "TOTAL_FAILED=${TOTAL_FAILED}"
} > prune-summary-ghcr.env

if (( TOTAL_FAILED > 0 )); then
  echo "$LOG_PREFIX ERROR: ${TOTAL_FAILED} delete(s) failed; check that the token has packages: write on this package" >&2
  exit 1
fi

echo "$LOG_PREFIX done"
