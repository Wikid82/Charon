#!/usr/bin/env bash
# Deterministic shard assignment for the `security-tests` Playwright project.
#
# Each security shard runs in its own container, and some security specs depend on
# state that earlier specs in the same run leave behind (the break-glass chain). Playwright's
# own --shard splits by contiguous file ranges, which can cut such a chain in two. This
# script reads an explicit manifest instead, so dependent specs always share a shard and
# every spec runs exactly once.
#
# Usage:
#   shard-security-specs.sh <shard> <total>      print the spec paths for one shard
#   shard-security-specs.sh --report             print the spec count and weight per shard
#   shard-security-specs.sh --generate <total>   print a fresh manifest (maintainers only)
#
# Manifest (scripts/ci/security-shard-manifest.txt): one line per spec,
#   <shard> <group> <weight> <path>
# group "pinned" marks the dependency chain (all pinned specs must share one shard).
# group "balanced" marks specs placed by the balancer. The weight is the number of
# top-level test() calls in the file, a static estimate of its size.
#
# The run fails when a spec under tests/security-enforcement/ or tests/security/ is missing
# from the manifest, listed twice, or when the manifest lists a file that no longer exists.
# A new spec must be added to the manifest in the shard --report shows as lightest.
#
# Environment (fixtures and self-tests only):
#   SECURITY_SPECS_ROOT      repository root to scan (default: this repository)
#   SECURITY_SHARD_MANIFEST  manifest to read (default: scripts/ci/security-shard-manifest.txt)
set -euo pipefail
export LC_ALL=C

ROOT="${SECURITY_SPECS_ROOT:-$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)}"
MANIFEST="${SECURITY_SHARD_MANIFEST:-$ROOT/scripts/ci/security-shard-manifest.txt}"
SPEC_DIRS=(tests/security-enforcement tests/security)

# Break-glass chain: emergency-reset must run before zzz-admin-whitelist-blocking, the
# zzz-security-ui specs, and zzzz-break-glass-recovery, which restores Cerberus with the
# test-runner whitelist bypass. All of them stay in one shard.
CHAIN_ENFORCEMENT=(
  tests/security-enforcement/emergency-reset.spec.ts
  tests/security-enforcement/zzz-admin-whitelist-blocking.spec.ts
  tests/security-enforcement/zzz-security-ui/access-lists-crud.spec.ts
  tests/security-enforcement/zzz-security-ui/crowdsec-import.spec.ts
  tests/security-enforcement/zzz-security-ui/encryption-management.spec.ts
  tests/security-enforcement/zzz-security-ui/real-time-logs.spec.ts
  tests/security-enforcement/zzz-security-ui/system-security-settings.spec.ts
  tests/security-enforcement/zzzz-break-glass-recovery.spec.ts
)

# tests/security/ specs that make real backend calls without setting their own admin
# whitelist. They run after the bypass in the serial order, so they depend on it. This is a
# static over-approximation: it cannot see UI page loads gated by Cerberus, so the
# unpinned tests/security/ specs still need CI confirmation.
CHAIN_SECURITY=(
  tests/security/acl-integration.spec.ts
  tests/security/audit-logs.spec.ts
  tests/security/crowdsec-background-install.spec.ts
  tests/security/crowdsec-diagnostics.spec.ts
  tests/security/crowdsec-dashboard.spec.ts
  tests/security/crowdsec-export-redaction.spec.ts
  tests/security/crowdsec-file-editor.spec.ts
  tests/security/crowdsec-first-enable.spec.ts
  tests/security/crowdsec-hub-items.spec.ts
  tests/security/crowdsec-hub-offline.spec.ts
  tests/security/crowdsec-hub-preset-apply.spec.ts
  tests/security/crowdsec-import.spec.ts
  tests/security/crowdsec-key-robustness.spec.ts
  tests/security/crowdsec-preset-apply.spec.ts
  tests/security/crowdsec-security-toggle-busy.spec.ts
  tests/security/crowdsec-ui-states.spec.ts
  tests/security/emergency-operations.spec.ts
  tests/security/geoip-database-card.spec.ts
  tests/security/self-proxy-requests.spec.ts
  tests/security/suite-integration.spec.ts
  tests/security/system-settings-feature-toggles.spec.ts
  tests/security/websocket-connection-token.spec.ts
)

fail() {
  echo "::error title=Security shard assignment::$*" >&2
  exit 1
}

# Every spec file under the scanned directories, sorted.
discover_specs() {
  local dir
  for dir in "${SPEC_DIRS[@]}"; do
    [[ -d "$ROOT/$dir" ]] || fail "spec directory missing: $dir"
  done
  (cd "$ROOT" && find "${SPEC_DIRS[@]}" -type f \( -name '*.spec.ts' -o -name '*.spec.js' \) | sort)
}

# Number of top-level test( calls in a spec file (at least 1).
weight_of() {
  local count
  count=$(grep -cE '^[[:space:]]*test(\.(only|skip|fixme))?\(' "$ROOT/$1" || true)
  echo $((count > 0 ? count : 1))
}

# Manifest entries without comments or blank lines.
manifest_rows() {
  [[ -f "$MANIFEST" ]] || fail "manifest missing: $MANIFEST"
  grep -vE '^[[:space:]]*(#|$)' "$MANIFEST" || true
}

# Fails loudly on any manifest problem. $1 is the number of shards.
validate() {
  local total="$1" rows bad listed discovered missing stale dups pinned_shards s count
  rows="$(manifest_rows)"
  [[ -n "$rows" ]] || fail "manifest has no entries: $MANIFEST"

  bad="$(awk 'NF != 4' <<<"$rows")"
  [[ -z "$bad" ]] || fail "malformed manifest line (want: <shard> <group> <weight> <path>): $bad"

  bad="$(awk '$2 != "pinned" && $2 != "balanced"' <<<"$rows")"
  [[ -z "$bad" ]] || fail "unknown group (want pinned or balanced): $bad"

  bad="$(awk -v t="$total" '$1 !~ /^[0-9]+$/ || $1 < 1 || $1 > t' <<<"$rows")"
  [[ -z "$bad" ]] || fail "shard number outside 1..$total: $bad"

  dups="$(awk '{print $4}' <<<"$rows" | sort | uniq -d)"
  [[ -z "$dups" ]] || fail "spec listed more than once in manifest: $dups"

  pinned_shards="$(awk '$2 == "pinned" {print $1}' <<<"$rows" | sort -u | wc -l)"
  [[ "$pinned_shards" -le 1 ]] || fail "pinned dependency chain is split across shards; keep every pinned entry in one shard"

  listed="$(awk '{print $4}' <<<"$rows" | sort)"
  discovered="$(discover_specs)"
  missing="$(comm -23 <(printf '%s\n' "$discovered") <(printf '%s\n' "$listed"))"
  [[ -z "$missing" ]] || fail "spec(s) not assigned to a shard; add each to $MANIFEST: $missing"
  stale="$(comm -13 <(printf '%s\n' "$discovered") <(printf '%s\n' "$listed"))"
  [[ -z "$stale" ]] || fail "manifest lists spec(s) that do not exist: $stale"

  for ((s = 1; s <= total; s++)); do
    count="$(awk -v s="$s" '$1 == s' <<<"$rows" | wc -l)"
    [[ "$count" -gt 0 ]] || fail "shard $s of $total has no specs"
  done
}

# Largest shard number in the manifest (for --report).
manifest_total() {
  awk '$1 ~ /^[0-9]+$/ && $1 > m {m = $1} END {print m + 0}' <<<"$(manifest_rows)"
}

shard_list() {
  local shard="$1" total="$2" rows
  [[ "$shard" =~ ^[0-9]+$ && "$total" =~ ^[0-9]+$ ]] || fail "usage: <shard> <total> (integers)"
  [[ "$shard" -ge 1 && "$shard" -le "$total" ]] || fail "shard $shard outside 1..$total"
  validate "$total"
  rows="$(manifest_rows)"
  awk -v s="$shard" '$1 == s {print $4}' <<<"$rows" | sort
}

report() {
  local total rows s
  total="$(manifest_total)"
  validate "$total"
  rows="$(manifest_rows)"
  for ((s = 1; s <= total; s++)); do
    awk -v s="$s" '$1 == s {n++; w += $3} END {printf "shard %d: %d specs, weight %d\n", s, n, w}' <<<"$rows"
  done
}

# Balanced manifest: pinned chain on shard 1, then the remaining specs largest-first onto
# the lightest shard (lowest index on ties). Deterministic for a given tree.
generate() {
  local total="$1" specs_text path w best i sorted
  local -a specs free rows load
  local -A pinned=()
  [[ "$total" =~ ^[0-9]+$ && "$total" -ge 1 ]] || fail "usage: --generate <total>"

  for path in "${CHAIN_ENFORCEMENT[@]}" "${CHAIN_SECURITY[@]}"; do
    [[ -f "$ROOT/$path" ]] || fail "pinned chain spec missing: $path"
    pinned["$path"]=1
  done

  specs_text="$(discover_specs)"
  mapfile -t specs <<<"$specs_text"
  for ((i = 1; i <= total; i++)); do load[i]=0; done

  for path in "${specs[@]}"; do
    w="$(weight_of "$path")"
    if [[ -n "${pinned[$path]:-}" ]]; then
      rows+=("1 pinned $w $path")
      load[1]=$((load[1] + w))
    else
      free+=("$w $path")
    fi
  done

  if [[ ${#free[@]} -gt 0 ]]; then
    sorted="$(printf '%s\n' "${free[@]}" | sort -k1,1nr -k2,2)"
    while IFS=' ' read -r w path; do
      best=1
      for ((i = 2; i <= total; i++)); do
        if ((load[i] < load[best])); then best=$i; fi
      done
      load[best]=$((load[best] + w))
      rows+=("$best balanced $w $path")
    done <<<"$sorted"
  fi

  cat <<'HEADER'
# Security E2E shard manifest (read by scripts/ci/shard-security-specs.sh).
#
# One line per spec under tests/security-enforcement/ and tests/security/:
#   <shard> <group> <weight> <path>
# group: pinned = break-glass dependency chain, all pinned lines share one shard.
#        balanced = placed by the balancer. Weight = top-level test() calls (static estimate).
# Every spec must appear exactly once. A new spec fails CI until it is added here.
# To place a new spec, run `bash scripts/ci/shard-security-specs.sh --report` and pick the
# lightest shard. Regenerating with --generate reshuffles every balanced spec, so avoid it
# unless the whole assignment is being rebalanced.
HEADER
  printf '%s\n' "${rows[@]}" | sort -k1,1n -k4,4
}

case "${1:-}" in
  --report)
    report
    ;;
  --generate)
    [[ $# -eq 2 ]] || fail "usage: --generate <total>"
    generate "$2"
    ;;
  *)
    [[ $# -eq 2 ]] || fail "usage: <shard> <total> | --report | --generate <total>"
    shard_list "$1" "$2"
    ;;
esac
