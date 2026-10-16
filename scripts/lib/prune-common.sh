#!/usr/bin/env bash
# shellcheck disable=SC2034  # globals are consumed by the sourcing scripts
# prune-common.sh
# Shared helpers for scripts/prune-ghcr.sh and scripts/prune-dockerhub.sh.
# Meant to be sourced, never executed. The sourcing script must define
# LOG_PREFIX and call prune_init_state before using the helpers below.

# Per-build tags are CI by-products (per-arch build tags, attestation/SBOM tags,
# branch+sha tags, bare sha tags). They are pruned after BUILD_KEEP_DAYS and are
# exempt from the KEEP_LAST_N shield. Release/protected tags always win.
BUILD_KEEP_DAYS=${BUILD_KEEP_DAYS:-7}
PROTECTED_REGEX=${PROTECTED_REGEX:-'["^v","^latest$","^main$","^develop$"]'}
BUILD_TAG_REGEX=${BUILD_TAG_REGEX:-'["^build-","^sha256-","^(development|main)-[0-9a-f]{7,40}$","^(development|main)-[0-9]+(-[A-Za-z0-9._-]+)?$","^[0-9a-f]{7,40}$"]'}

PROTECTED_PATTERNS=()
BUILD_PATTERNS=()

require() {
  command -v "$1" >/dev/null 2>&1 || { echo "$LOG_PREFIX missing required command: $1" >&2; exit 1; }
}

# Parses DRY_RUN into the dry_run boolean and zeroes the counters.
prune_init_state() {
  dry_run=false
  case "${DRY_RUN:-false}" in
    [Tt][Rr][Uu][Ee]|1|[Yy][Ee][Ss]|[Yy]|[Oo][Nn]) dry_run=true ;;
  esac

  TOTAL_CANDIDATES=0
  TOTAL_CANDIDATES_BYTES=0
  TOTAL_DELETED=0
  TOTAL_DELETED_BYTES=0
  TOTAL_FAILED=0

  cutoff_ts=$(date -d "$KEEP_DAYS days ago" +%s)
  build_cutoff_ts=$(date -d "$BUILD_KEEP_DAYS days ago" +%s)
}

# Loads a JSON array of regexes into the named global array (bash ERE).
load_regex_list() {
  local json="$1" name="$2" label="$3"
  if ! echo "$json" | jq -e 'type == "array" and all(type == "string")' >/dev/null 2>&1; then
    echo "$LOG_PREFIX $label must be a JSON array of strings" >&2
    exit 1
  fi
  mapfile -t "$name" < <(echo "$json" | jq -r '.[]')
}

load_tag_patterns() {
  load_regex_list "$PROTECTED_REGEX" PROTECTED_PATTERNS PROTECTED_REGEX
  load_regex_list "$BUILD_TAG_REGEX" BUILD_PATTERNS BUILD_TAG_REGEX
}

# matches_any <tag> <pattern>... ; success when the tag matches one pattern.
matches_any() {
  local tag="$1" rgx
  shift
  for rgx in "$@"; do
    [[ -z "$rgx" ]] && continue
    if [[ "$tag" =~ $rgx ]]; then
      return 0
    fi
  done
  return 1
}

is_protected_tag() { matches_any "$1" "${PROTECTED_PATTERNS[@]}"; }
is_build_tag() { matches_any "$1" "${BUILD_PATTERNS[@]}"; }

# classify_tags <tag>...
# Prints one class for a manifest/version carrying the given tags:
#   untagged  - no tags
#   protected - at least one tag is a release/protected tag
#   build     - every tag is a per-build tag (BUILD_KEEP_DAYS, no last-N shield)
#   aged      - anything else (KEEP_DAYS, subject to the last-N shield)
classify_tags() {
  if (( $# == 0 )); then
    echo untagged
    return
  fi
  local t all_build=true
  for t in "$@"; do
    if is_protected_tag "$t"; then
      echo protected
      return
    fi
    is_build_tag "$t" || all_build=false
  done
  if $all_build; then echo build; else echo aged; fi
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

# registry_delete <description> <bytes> <url> [curl header args...]
# Issues a DELETE and verifies the HTTP status instead of trusting curl's exit
# code. 2xx counts as deleted. 404 is also treated as success: the object is
# already gone, which is the state we wanted. Everything else (401/403/5xx or
# a transport error) is logged and counted in TOTAL_FAILED. Headers are never
# logged, so tokens cannot leak.
registry_delete() {
  local desc="$1" bytes="${2:-0}" url="$3"
  shift 3
  local body_file code snippet
  body_file=$(mktemp)
  code=$(curl -sS -o "$body_file" -w '%{http_code}' -X DELETE "$@" "$url") || code=000
  if [[ "$code" =~ ^2[0-9][0-9]$ ]] || [[ "$code" == "404" ]]; then
    TOTAL_DELETED=$((TOTAL_DELETED + 1))
    TOTAL_DELETED_BYTES=$((TOTAL_DELETED_BYTES + bytes))
    [[ "$code" == "404" ]] && echo "$LOG_PREFIX $desc already gone (http=404)"
  else
    snippet=$(head -c 200 "$body_file" | tr '\n\r' '  ')
    echo "$LOG_PREFIX delete FAILED http=$code $desc ${snippet}" >&2
    TOTAL_FAILED=$((TOTAL_FAILED + 1))
  fi
  rm -f "$body_file"
}
