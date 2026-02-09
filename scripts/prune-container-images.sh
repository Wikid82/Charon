#!/usr/bin/env bash
set -euo pipefail

# prune-container-images.sh
# Deletes old images from GHCR and Docker Hub according to retention and protection rules.
# Defaults: dry-run (no deletes). Accepts env vars for configuration.

# Required env vars (workflow will set these):
# - REGISTRIES (comma-separated: ghcr,dockerhub)
# - OWNER (github repository owner)
# - IMAGE_NAME (charon)
# - KEEP_DAYS (default 30)
# - PROTECTED_REGEX (JSON array of regex strings)
# - DRY_RUN (true/false)
# - KEEP_LAST_N (optional, default 30)
# - DOCKERHUB_USERNAME/DOCKERHUB_TOKEN (for Docker Hub)
# - GITHUB_TOKEN (for GHCR API)

REGISTRIES=${REGISTRIES:-ghcr}
OWNER=${OWNER:-${GITHUB_REPOSITORY_OWNER:-Wikid82}}
IMAGE_NAME=${IMAGE_NAME:-charon}
KEEP_DAYS=${KEEP_DAYS:-30}
KEEP_LAST_N=${KEEP_LAST_N:-30}
DRY_RUN=${DRY_RUN:-true}
PROTECTED_REGEX=${PROTECTED_REGEX:-'["^v","^latest$","^main$","^develop$"]'}

LOG_PREFIX="[prune]"
now_ts=$(date +%s)
cutoff_ts=$(date -d "$KEEP_DAYS days ago" +%s 2>/dev/null || date -d "-$KEEP_DAYS days" +%s)

# Totals
TOTAL_CANDIDATES=0
TOTAL_CANDIDATES_BYTES=0
TOTAL_DELETED=0
TOTAL_DELETED_BYTES=0

echo "$LOG_PREFIX starting with REGISTRIES=$REGISTRIES KEEP_DAYS=$KEEP_DAYS DRY_RUN=$DRY_RUN"

action_delete_ghcr() {
  echo "$LOG_PREFIX -> GHCR cleanup for $OWNER/$IMAGE_NAME (dry-run=$DRY_RUN)"

  page=1
  per_page=100
  namespace_type="orgs"

  while :; do
    url="https://api.github.com/${namespace_type}/${OWNER}/packages/container/${IMAGE_NAME}/versions?per_page=$per_page&page=$page"
    resp=$(curl -sS -H "Authorization: Bearer $GITHUB_TOKEN" "$url")

    # Handle API errors gracefully and try users/organizations as needed
    if echo "$resp" | jq -e '.message' >/dev/null 2>&1; then
      msg=$(echo "$resp" | jq -r '.message')
      if [[ "$msg" == "Not Found" && "$namespace_type" == "orgs" ]]; then
        echo "$LOG_PREFIX GHCR org lookup returned Not Found; switching to users endpoint"
        namespace_type="users"
        page=1
        continue
      fi

      if echo "$msg" | grep -q "read:packages"; then
        echo "$LOG_PREFIX GHCR API error: $msg. Ensure token has 'read:packages' scope or use Actions GITHUB_TOKEN with package permissions."
        return
      fi
    fi

    ids=$(echo "$resp" | jq -r '.[].id' 2>/dev/null)
    if [[ -z "$ids" ]]; then
      break
    fi

    # For each version, capture id, created_at, tags
    echo "$resp" | jq -c '.[]' | while read -r ver; do
      id=$(echo "$ver" | jq -r '.id')
      created=$(echo "$ver" | jq -r '.created_at')
      tags=$(echo "$ver" | jq -r '.metadata.container.tags // [] | join(",")')
      created_ts=$(date -d "$created" +%s 2>/dev/null || date -j -f "%Y-%m-%dT%H:%M:%SZ" "$created" +%s 2>/dev/null || 0)

      # skip protected tags
      protected=false
      for rgx in $(echo "$PROTECTED_REGEX" | jq -r '.[]'); do
        for tag in $(echo "$tags" | sed 's/,/ /g'); do
          if [[ "$tag" =~ $rgx ]]; then
            protected=true
          fi
        done
      done

      if $protected; then
        echo "$LOG_PREFIX keep (protected): id=$id tags=$tags created=$created"
        continue
      fi

      # skip if not older than cutoff
      if (( created_ts >= cutoff_ts )); then
        echo "$LOG_PREFIX keep (recent): id=$id tags=$tags created=$created"
        continue
      fi

      echo "$LOG_PREFIX candidate: id=$id tags=$tags created=$created"

      # Try to estimate size for GHCR by fetching manifest (best-effort)
      candidate_bytes=0
      for tag in $(echo "$tags" | sed 's/,/ /g'); do
        if [[ -n "$tag" && "$tag" != "null" ]]; then
          manifest_url="https://ghcr.io/v2/${OWNER}/${IMAGE_NAME}/manifests/${tag}"
          manifest=$(curl -sS -H "Accept: application/vnd.docker.distribution.manifest.v2+json" -H "Authorization: Bearer $GITHUB_TOKEN" "$manifest_url" || true)
          if [[ -n "$manifest" ]]; then
            bytes=$(echo "$manifest" | jq -r '.layers // [] | map(.size) | add // 0')
            if [[ "$bytes" != "null" && "$bytes" -gt 0 2>/dev/null ]]; then
              candidate_bytes=$((candidate_bytes + bytes))
            fi
          fi
        fi
      done

      TOTAL_CANDIDATES=$((TOTAL_CANDIDATES+1))
      TOTAL_CANDIDATES_BYTES=$((TOTAL_CANDIDATES_BYTES + candidate_bytes))

      if [[ "$DRY_RUN" == "true" ]]; then
        echo "$LOG_PREFIX DRY RUN: would delete GHCR version id=$id (approx ${candidate_bytes} bytes)"
      else
        echo "$LOG_PREFIX deleting GHCR version id=$id (approx ${candidate_bytes} bytes)"
        curl -sS -X DELETE -H "Authorization: Bearer $GITHUB_TOKEN" \
          "https://api.github.com/${namespace_type}/${OWNER}/packages/container/${IMAGE_NAME}/versions/$id"
        TOTAL_DELETED=$((TOTAL_DELETED+1))
        TOTAL_DELETED_BYTES=$((TOTAL_DELETED_BYTES + candidate_bytes))
      fi

    done

    ((page++))
  done
}

action_delete_dockerhub() {
  echo "$LOG_PREFIX -> Docker Hub cleanup for $DOCKERHUB_USERNAME/$IMAGE_NAME (dry-run=$DRY_RUN)"

  if [[ -z "${DOCKERHUB_USERNAME:-}" || -z "${DOCKERHUB_TOKEN:-}" ]]; then
    echo "$LOG_PREFIX Docker Hub credentials not set; skipping Docker Hub cleanup"
    return
  fi

  # Login to Docker Hub to get token (v2)
  hub_token=$(curl -sS -X POST -H "Content-Type: application/json" \
    -d "{\"username\":\"${DOCKERHUB_USERNAME}\",\"password\":\"${DOCKERHUB_TOKEN}\"}" \
    https://hub.docker.com/v2/users/login/ | jq -r '.token')

  if [[ -z "$hub_token" || "$hub_token" == "null" ]]; then
    echo "$LOG_PREFIX Failed to obtain Docker Hub token; aborting Docker Hub cleanup"
    return
  fi

  page=1
  page_size=100
  while :; do
    resp=$(curl -sS -H "Authorization: JWT $hub_token" \
      "https://hub.docker.com/v2/repositories/${DOCKERHUB_USERNAME}/${IMAGE_NAME}/tags?page_size=$page_size&page=$page")

    results_count=$(echo "$resp" | jq -r '.results | length')
    if [[ "$results_count" == "0" || -z "$results_count" ]]; then
      break
    fi

    echo "$resp" | jq -c '.results[]' | while read -r tag; do
      tag_name=$(echo "$tag" | jq -r '.name')
      last_updated=$(echo "$tag" | jq -r '.last_updated')
      last_ts=$(date -d "$last_updated" +%s 2>/dev/null || date -j -f "%Y-%m-%dT%H:%M:%S%z" "$last_updated" +%s 2>/dev/null || 0)

      # Check protected patterns
      protected=false
      for rgx in $(echo "$PROTECTED_REGEX" | jq -r '.[]'); do
        if [[ "$tag_name" =~ $rgx ]]; then
          protected=true
          break
        fi
      done
      if $protected; then
        echo "$LOG_PREFIX keep (protected): tag=$tag_name last_updated=$last_updated"
        continue
      fi

      if (( last_ts >= cutoff_ts )); then
        echo "$LOG_PREFIX keep (recent): tag=$tag_name last_updated=$last_updated"
        continue
      fi

      echo "$LOG_PREFIX candidate: tag=$tag_name last_updated=$last_updated"

      # Estimate size from Docker Hub tag JSON (images[].size or full_size)
      bytes=0
      bytes=$(echo "$tag" | jq -r '.images | map(.size) | add // empty') || true
      if [[ -z "$bytes" || "$bytes" == "null" ]]; then
        bytes=$(echo "$tag" | jq -r '.full_size // empty' 2>/dev/null || true)
      fi
      bytes=${bytes:-0}

      TOTAL_CANDIDATES=$((TOTAL_CANDIDATES+1))
      TOTAL_CANDIDATES_BYTES=$((TOTAL_CANDIDATES_BYTES + bytes))

      if [[ "$DRY_RUN" == "true" ]]; then
        echo "$LOG_PREFIX DRY RUN: would delete Docker Hub tag=$tag_name (approx ${bytes} bytes)"
      else
        echo "$LOG_PREFIX deleting Docker Hub tag=$tag_name (approx ${bytes} bytes)"
        curl -sS -X DELETE -H "Authorization: JWT $hub_token" \
          "https://hub.docker.com/v2/repositories/${DOCKERHUB_USERNAME}/${IMAGE_NAME}/tags/${tag_name}/"
        TOTAL_DELETED=$((TOTAL_DELETED+1))
        TOTAL_DELETED_BYTES=$((TOTAL_DELETED_BYTES + bytes))
      fi

    done

    ((page++))
  done
}

# Main: iterate requested registries
IFS=',' read -ra regs <<< "$REGISTRIES"
for r in "${regs[@]}"; do
  case "$r" in
    ghcr)
      action_delete_ghcr
      ;;
    dockerhub)
      action_delete_dockerhub
      ;;
    *)
      echo "$LOG_PREFIX unknown registry: $r"
      ;;
  esac
done

# Summary
human_readable() {
  local bytes=$1
  if (( bytes == 0 )); then
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

echo "$LOG_PREFIX SUMMARY: total_candidates=${TOTAL_CANDIDATES} total_candidates_bytes=${TOTAL_CANDIDATES_BYTES} total_deleted=${TOTAL_DELETED} total_deleted_bytes=${TOTAL_DELETED_BYTES}"
echo "$LOG_PREFIX SUMMARY_HUMAN: candidates=${TOTAL_CANDIDATES} candidates_size=$(human_readable ${TOTAL_CANDIDATES_BYTES}) deleted=${TOTAL_DELETED} deleted_size=$(human_readable ${TOTAL_DELETED_BYTES})"

# Export summary for workflow parsing
echo "TOTAL_CANDIDATES=${TOTAL_CANDIDATES}" >> prune-summary.env
echo "TOTAL_CANDIDATES_BYTES=${TOTAL_CANDIDATES_BYTES}" >> prune-summary.env
echo "TOTAL_DELETED=${TOTAL_DELETED}" >> prune-summary.env
echo "TOTAL_DELETED_BYTES=${TOTAL_DELETED_BYTES}" >> prune-summary.env

echo "$LOG_PREFIX done"
