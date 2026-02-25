#!/usr/bin/env bash
set -euo pipefail

# prune-container-images.sh
# Deletes old images from GHCR and Docker Hub according to retention and protection rules.

REGISTRIES=${REGISTRIES:-ghcr}
OWNER=${OWNER:-${GITHUB_REPOSITORY_OWNER:-Wikid82}}
IMAGE_NAME=${IMAGE_NAME:-charon}

KEEP_DAYS=${KEEP_DAYS:-30}
KEEP_LAST_N=${KEEP_LAST_N:-30}

DRY_RUN=${DRY_RUN:-false}
PROTECTED_REGEX=${PROTECTED_REGEX:-'["^v","^latest$","^main$","^develop$"]'}

# Extra knobs (optional)
PRUNE_UNTAGGED=${PRUNE_UNTAGGED:-true}
PRUNE_SBOM_TAGS=${PRUNE_SBOM_TAGS:-true}

LOG_PREFIX="[prune]"

now_ts=$(date +%s)
cutoff_ts=$(date -d "$KEEP_DAYS days ago" +%s 2>/dev/null || date -d "-$KEEP_DAYS days" +%s)

# Normalize DRY_RUN to true/false reliably
dry_run=false
case "${DRY_RUN,,}" in
  true|1|yes|y|on) dry_run=true ;;
  *) dry_run=false ;;
esac

# Totals
TOTAL_CANDIDATES=0
TOTAL_CANDIDATES_BYTES=0
TOTAL_DELETED=0
TOTAL_DELETED_BYTES=0

echo "$LOG_PREFIX starting with REGISTRIES=$REGISTRIES OWNER=$OWNER IMAGE_NAME=$IMAGE_NAME KEEP_DAYS=$KEEP_DAYS KEEP_LAST_N=$KEEP_LAST_N DRY_RUN=$dry_run"
echo "$LOG_PREFIX PROTECTED_REGEX=$PROTECTED_REGEX PRUNE_UNTAGGED=$PRUNE_UNTAGGED PRUNE_SBOM_TAGS=$PRUNE_SBOM_TAGS"

require() {
  command -v "$1" >/dev/null 2>&1 || { echo "$LOG_PREFIX missing required command: $1"; exit 1; }
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

# Some repos generate tons of tags like sha-xxxx, pr-123-xxxx, *.sbom.
# We treat SBOM-only tags as deletable (optional).
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

# --- GHCR ---
ghcr_list_all_versions_json() {
  local namespace_type="$1"  # orgs or users
  local page=1
  local per_page=100
  local all='[]'

  while :; do
    local url="https://api.github.com/${namespace_type}/${OWNER}/packages/container/${IMAGE_NAME}/versions?per_page=$per_page&page=$page"

    # Use GitHub’s recommended headers
    local resp
    resp=$(curl -sS \
      -H "Authorization: Bearer $GITHUB_TOKEN" \
      -H "Accept: application/vnd.github+json" \
      -H "X-GitHub-Api-Version: 2022-11-28" \
      "$url" || true)

    # ✅ NEW: ensure we got JSON
    if ! echo "$resp" | jq -e . >/dev/null 2>&1; then
      echo "$LOG_PREFIX GHCR returned non-JSON for url=$url"
      echo "$LOG_PREFIX GHCR response (first 200 chars): $(echo "$resp" | head -c 200 | tr '\n' ' ')"
      echo "[]"
      return 0
    fi

    # Handle JSON error messages
    if echo "$resp" | jq -e 'has("message")' >/dev/null 2>&1; then
      local msg
      msg=$(echo "$resp" | jq -r '.message')

      if [[ "$msg" == "Not Found" ]]; then
        echo "$LOG_PREFIX GHCR ${namespace_type} endpoint returned Not Found"
        echo "[]"
        return 0
      fi

      echo "$LOG_PREFIX GHCR API error: $msg"
      # also print documentation_url if present (helpful)
      doc=$(echo "$resp" | jq -r '.documentation_url // empty')
      [[ -n "$doc" ]] && echo "$LOG_PREFIX GHCR docs: $doc"
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

  # Try orgs first, then users
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

  # Normalize a working list:
  # - id
  # - created_at
  # - created_ts
  # - tags array
  # - tags_csv
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

  # Compute the globally newest KEEP_LAST_N ids to always keep
  # (If KEEP_LAST_N is 0 or empty, keep none by this rule)
  local keep_ids
  keep_ids=$(echo "$normalized" | jq -r --argjson n "${KEEP_LAST_N:-0}" '
    (sort_by(.created_ts) | reverse) as $s
    | ($s[0:$n] | map(.id)) | join(" ")
  ')

  if [[ -n "$keep_ids" ]]; then
    echo "$LOG_PREFIX GHCR: keeping newest KEEP_LAST_N ids: $KEEP_LAST_N"
  fi

  # Iterate versions sorted oldest->newest so deletions are predictable
  while IFS= read -r ver; do
    local id created created_ts tags_csv
    id=$(echo "$ver" | jq -r '.id')
    created=$(echo "$ver" | jq -r '.created_at')
    created_ts=$(echo "$ver" | jq -r '.created_ts')
    tags_csv=$(echo "$ver" | jq -r '.tags_csv')

    # KEEP_LAST_N rule (global)
    if [[ -n "$keep_ids" && " $keep_ids " == *" $id "* ]]; then
      echo "$LOG_PREFIX keep (last_n): id=$id tags=$tags_csv created=$created"
      continue
    fi

    # Protected tags rule
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

    # Optional: treat SBOM-only versions/tags as deletable
    # If every tag is *.sbom and PRUNE_SBOM_TAGS=true, we allow pruning regardless of “tag protected” rules.
    if [[ "${PRUNE_SBOM_TAGS,,}" == "true" && -n "$tags_csv" ]]; then
      all_sbom=true
      while IFS= read -r t; do
        [[ -z "$t" ]] && continue
        if ! tag_is_sbom "$t"; then
          all_sbom=false
          break
        fi
      done < <(echo "$tags_csv" | tr ',' '\n')
      if $all_sbom; then
        # allow fallthrough; do not "keep" just because tags are recent
        :
      fi
    fi

    # Age rule
    if (( created_ts >= cutoff_ts )); then
      echo "$LOG_PREFIX keep (recent): id=$id tags=$tags_csv created=$created"
      continue
    fi

    # Optional: prune untagged versions (common GHCR bloat)
    if [[ "${PRUNE_UNTAGGED,,}" == "true" ]]; then
      # tags_csv can be empty for untagged
      if [[ -z "$tags_csv" ]]; then
        echo "$LOG_PREFIX candidate (untagged): id=$id tags=<none> created=$created"
      else
        echo "$LOG_PREFIX candidate: id=$id tags=$tags_csv created=$created"
      fi
    else
      # If not pruning untagged, skip them
      if [[ -z "$tags_csv" ]]; then
        echo "$LOG_PREFIX keep (untagged disabled): id=$id created=$created"
        continue
      fi
      echo "$LOG_PREFIX candidate: id=$id tags=$tags_csv created=$created"
    fi

    # Candidate bookkeeping
    TOTAL_CANDIDATES=$((TOTAL_CANDIDATES + 1))

    # Best-effort size estimation: GHCR registry auth is messy; don’t block prune on it.
    candidate_bytes=0

    if $dry_run; then
      echo "$LOG_PREFIX DRY RUN: would delete GHCR version id=$id (approx ${candidate_bytes} bytes)"
    else
      echo "$LOG_PREFIX deleting GHCR version id=$id"
      # Use GitHub API delete
      curl -sS -X DELETE -H "Authorization: Bearer $GITHUB_TOKEN" \
        "https://api.github.com/${namespace_type}/${OWNER}/packages/container/${IMAGE_NAME}/versions/$id" >/dev/null || true
      TOTAL_DELETED=$((TOTAL_DELETED + 1))
    fi

  done < <(echo "$normalized" | jq -c 'sort_by(.created_ts) | .[]')
}

# --- Docker Hub ---
action_delete_dockerhub() {
  echo "$LOG_PREFIX -> Docker Hub cleanup for ${DOCKERHUB_USERNAME:-<unset>}/$IMAGE_NAME (dry-run=$dry_run)"

  if [[ -z "${DOCKERHUB_USERNAME:-}" || -z "${DOCKERHUB_TOKEN:-}" ]]; then
    echo "$LOG_PREFIX Docker Hub credentials not set; skipping Docker Hub cleanup"
    return
  fi

  hub_token=$(curl -sS -X POST -H "Content-Type: application/json" \
    -d "{\"username\":\"${DOCKERHUB_USERNAME}\",\"password\":\"${DOCKERHUB_TOKEN}\"}" \
    https://hub.docker.com/v2/users/login/ | jq -r '.token')

  if [[ -z "$hub_token" || "$hub_token" == "null" ]]; then
    echo "$LOG_PREFIX Failed to obtain Docker Hub token; aborting Docker Hub cleanup"
    return
  fi

  # Fetch all pages first so KEEP_LAST_N can be global
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
    last_ts=$(date -d "$last_updated" +%s 2>/dev/null || 0)

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

# Main: iterate requested registries
IFS=',' read -ra regs <<< "$REGISTRIES"
for r in "${regs[@]}"; do
  case "$r" in
    ghcr) action_delete_ghcr ;;
    dockerhub) action_delete_dockerhub ;;
    *) echo "$LOG_PREFIX unknown registry: $r" ;;
  esac
done

# Summary
echo "$LOG_PREFIX SUMMARY: total_candidates=${TOTAL_CANDIDATES} total_candidates_bytes=${TOTAL_CANDIDATES_BYTES} total_deleted=${TOTAL_DELETED} total_deleted_bytes=${TOTAL_DELETED_BYTES}"
echo "$LOG_PREFIX SUMMARY_HUMAN: candidates=${TOTAL_CANDIDATES} candidates_size=$(human_readable "${TOTAL_CANDIDATES_BYTES}") deleted=${TOTAL_DELETED} deleted_size=$(human_readable "${TOTAL_DELETED_BYTES}")"

# Export summary for workflow parsing
: > prune-summary.env
echo "TOTAL_CANDIDATES=${TOTAL_CANDIDATES}" >> prune-summary.env
echo "TOTAL_CANDIDATES_BYTES=${TOTAL_CANDIDATES_BYTES}" >> prune-summary.env
echo "TOTAL_DELETED=${TOTAL_DELETED}" >> prune-summary.env
echo "TOTAL_DELETED_BYTES=${TOTAL_DELETED_BYTES}" >> prune-summary.env

echo "$LOG_PREFIX done"
