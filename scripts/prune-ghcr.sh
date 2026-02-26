#!/usr/bin/env bash
set -euo pipefail
# prune-ghcr.sh
# Deletes old container images from GitHub Container Registry (GHCR)
# according to retention and protection rules.

OWNER=${OWNER:-${GITHUB_REPOSITORY_OWNER:-Wikid82}}
IMAGE_NAME=${IMAGE_NAME:-charon}

KEEP_DAYS=${KEEP_DAYS:-30}
KEEP_LAST_N=${KEEP_LAST_N:-30}

DRY_RUN=${DRY_RUN:-false}
PROTECTED_REGEX=${PROTECTED_REGEX:-'["^v","^latest$","^main$","^develop$"]'}

PRUNE_UNTAGGED=${PRUNE_UNTAGGED:-true}
PRUNE_SBOM_TAGS=${PRUNE_SBOM_TAGS:-true}

LOG_PREFIX="[prune-ghcr]"

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
echo "$LOG_PREFIX PROTECTED_REGEX=$PROTECTED_REGEX PRUNE_UNTAGGED=$PRUNE_UNTAGGED PRUNE_SBOM_TAGS=$PRUNE_SBOM_TAGS"

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

tag_is_sbom() {
  local tag="$1"
  [[ "$tag" == *.sbom ]]
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

    if echo "$resp" | jq -e 'has("message")' >/dev/null 2>&1; then
      local msg
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
    ((page++))
  done

  echo "$all"
}

action_delete_ghcr() {
  echo "$LOG_PREFIX -> GHCR cleanup for $OWNER/$IMAGE_NAME (dry-run=$dry_run)"

  if [[ -z "${GITHUB_TOKEN:-}" ]]; then
    echo "$LOG_PREFIX GITHUB_TOKEN not set; skipping GHCR cleanup"
    return
  fi

  local all
  local namespace_type="orgs"
  all=$(ghcr_list_all_versions_json "$namespace_type")
  if [[ "$(echo "$all" | jq -r 'length')" == "0" ]]; then
    namespace_type="users"
    all=$(ghcr_list_all_versions_json "$namespace_type")
  fi

  local total
  total=$(echo "$all" | jq -r 'length')
  if [[ -z "$total" || "$total" == "0" ]]; then
    echo "$LOG_PREFIX GHCR: no versions found (or insufficient access)."
    return
  fi

  echo "$LOG_PREFIX GHCR: fetched $total versions total"

  local normalized
  normalized=$(echo "$all" | jq -c '
    map({
      id: .id,
      created_at: .created_at,
      tags: (.metadata.container.tags // []),
      tags_csv: ((.metadata.container.tags // []) | join(",")),
      created_ts: (.created_at | fromdateiso8601)
    })
  ')

  local keep_ids
  keep_ids=$(echo "$normalized" | jq -r --argjson n "${KEEP_LAST_N:-0}" '
    (sort_by(.created_ts) | reverse) as $s
    | ($s[0:$n] | map(.id)) | join(" ")
  ')

  if [[ -n "$keep_ids" ]]; then
    echo "$LOG_PREFIX GHCR: keeping newest KEEP_LAST_N ids: $KEEP_LAST_N"
  fi

  local ver protected all_sbom candidate_bytes
  while IFS= read -r ver; do
    local id created created_ts tags_csv
    all_sbom=false
    id=$(echo "$ver" | jq -r '.id')
    created=$(echo "$ver" | jq -r '.created_at')
    created_ts=$(echo "$ver" | jq -r '.created_ts')
    tags_csv=$(echo "$ver" | jq -r '.tags_csv')

    if [[ -n "$keep_ids" && " $keep_ids " == *" $id "* ]]; then
      echo "$LOG_PREFIX keep (last_n): id=$id tags=$tags_csv created=$created"
      continue
    fi

    protected=false
    if [[ -n "$tags_csv" ]]; then
      while IFS= read -r t; do
        [[ -z "$t" ]] && continue
        if is_protected_tag "$t"; then
          protected=true
          break
        fi
      done < <(echo "$tags_csv" | tr ',' '\n')
    fi
    if $protected; then
      echo "$LOG_PREFIX keep (protected): id=$id tags=$tags_csv created=$created"
      continue
    fi

    if [[ "${PRUNE_SBOM_TAGS,,}" == "true" && -n "$tags_csv" ]]; then
      all_sbom=true
      while IFS= read -r t; do
        [[ -z "$t" ]] && continue
        if ! tag_is_sbom "$t"; then
          all_sbom=false
          break
        fi
      done < <(echo "$tags_csv" | tr ',' '\n')
    fi

    # If all tags are SBOM tags and PRUNE_SBOM_TAGS is enabled, skip the age check
    if [[ "${all_sbom:-false}" == "true" ]]; then
      echo "$LOG_PREFIX candidate (sbom-only): id=$id tags=$tags_csv created=$created"
    else
      if (( created_ts >= cutoff_ts )); then
        echo "$LOG_PREFIX keep (recent): id=$id tags=$tags_csv created=$created"
        continue
      fi

      if [[ "${PRUNE_UNTAGGED,,}" == "true" ]]; then
        if [[ -z "$tags_csv" ]]; then
          echo "$LOG_PREFIX candidate (untagged): id=$id tags=<none> created=$created"
        else
          echo "$LOG_PREFIX candidate: id=$id tags=$tags_csv created=$created"
        fi
      else
        if [[ -z "$tags_csv" ]]; then
          echo "$LOG_PREFIX keep (untagged disabled): id=$id created=$created"
          continue
        fi
        echo "$LOG_PREFIX candidate: id=$id tags=$tags_csv created=$created"
      fi
    fi

    TOTAL_CANDIDATES=$((TOTAL_CANDIDATES + 1))

    candidate_bytes=0

    if $dry_run; then
      echo "$LOG_PREFIX DRY RUN: would delete GHCR version id=$id (approx ${candidate_bytes} bytes)"
    else
      echo "$LOG_PREFIX deleting GHCR version id=$id"
      curl -sS -X DELETE -H "Authorization: Bearer $GITHUB_TOKEN" \
        "https://api.github.com/${namespace_type}/${OWNER}/packages/container/${IMAGE_NAME}/versions/$id" >/dev/null || true
      TOTAL_DELETED=$((TOTAL_DELETED + 1))
    fi

  done < <(echo "$normalized" | jq -c 'sort_by(.created_ts) | .[]')
}

# Main
action_delete_ghcr

echo "$LOG_PREFIX SUMMARY: total_candidates=${TOTAL_CANDIDATES} total_candidates_bytes=${TOTAL_CANDIDATES_BYTES} total_deleted=${TOTAL_DELETED} total_deleted_bytes=${TOTAL_DELETED_BYTES}"
echo "$LOG_PREFIX SUMMARY_HUMAN: candidates=${TOTAL_CANDIDATES} candidates_size=$(human_readable "${TOTAL_CANDIDATES_BYTES}") deleted=${TOTAL_DELETED} deleted_size=$(human_readable "${TOTAL_DELETED_BYTES}")"

: > prune-summary-ghcr.env
echo "TOTAL_CANDIDATES=${TOTAL_CANDIDATES}" >> prune-summary-ghcr.env
echo "TOTAL_CANDIDATES_BYTES=${TOTAL_CANDIDATES_BYTES}" >> prune-summary-ghcr.env
echo "TOTAL_DELETED=${TOTAL_DELETED}" >> prune-summary-ghcr.env
echo "TOTAL_DELETED_BYTES=${TOTAL_DELETED_BYTES}" >> prune-summary-ghcr.env

echo "$LOG_PREFIX done"
