# Shared fixture for the prune-*.sh bats tests.
#
# Puts a stub `curl` first on PATH. The stub answers from a routes file
# ($PF_DIR/routes), one rule per line:  METHOD|URL glob|HTTP status|body file
# The first matching rule wins; no match answers 404. A status written as
# "401x1" answers that status for the first N matching calls, then the rule is
# skipped so a later rule can answer (e.g. an expired token that works on retry). Every call is appended to
# $PF_DIR/calls as "METHOD URL" so tests can assert what was (not) deleted.

pf_setup() {
  PF_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  export PF_ROOT
  PF_DIR="$(mktemp -d)"
  mkdir -p "$PF_DIR/bin" "$PF_DIR/work"
  : > "$PF_DIR/routes"
  : > "$PF_DIR/calls"
  cat > "$PF_DIR/bin/curl" <<'STUB'
#!/usr/bin/env bash
method=GET out="" write="" url=""
while (( $# )); do
  case "$1" in
    -X) method="$2"; shift 2 ;;
    -o) out="$2"; shift 2 ;;
    -w) write="$2"; shift 2 ;;
    -H|--data-binary|-d) shift 2 ;;
    -*) shift ;;
    *) url="$1"; shift ;;
  esac
done
echo "$method $url" >> "$PF_DIR/calls"
code=404 body='{"message":"no route"}'
n=0
while IFS='|' read -r m glob c f; do
  n=$((n + 1))
  [[ -z "$m" ]] && continue
  # shellcheck disable=SC2053
  if [[ "$m" == "$method" && "$url" == $glob ]]; then
    if [[ "$c" == *x* ]]; then
      hits=$(cat "$PF_DIR/hits.$n" 2>/dev/null || echo 0)
      if (( hits >= ${c#*x} )); then continue; fi
      echo $((hits + 1)) > "$PF_DIR/hits.$n"
      c="${c%x*}"
    fi
    code="$c"
    if [[ -n "$f" ]]; then body=$(cat "$f"); else body=''; fi
    break
  fi
done < "$PF_DIR/routes"
if [[ -n "$out" ]]; then
  printf '%s' "$body" > "$out"
  [[ "$write" == *http_code* ]] && printf '%s' "$code"
else
  printf '%s' "$body"
fi
exit 0
STUB
  chmod +x "$PF_DIR/bin/curl"
  export PF_DIR
  export PATH="$PF_DIR/bin:$PATH"
  cd "$PF_DIR/work" || return 1
}

pf_teardown() { rm -rf "$PF_DIR"; }

# pf_route METHOD GLOB CODE [BODY]
pf_route() {
  local body_file=""
  if [[ -n "${4:-}" ]]; then
    body_file="$PF_DIR/body.$RANDOM$RANDOM"
    printf '%s' "$4" > "$body_file"
  fi
  printf '%s|%s|%s|%s\n' "$1" "$2" "$3" "$body_file" >> "$PF_DIR/routes"
}

pf_iso_days_ago() { date -u -d "$1 days ago" +%Y-%m-%dT%H:%M:%SZ; }

# Docker Hub tags page from lines "name days_ago".
pf_dockerhub_tags_json() {
  local name days
  while read -r name days; do
    [[ -z "$name" ]] && continue
    jq -n --arg n "$name" --arg d "$(pf_iso_days_ago "$days")" \
      '{name:$n,last_updated:$d,images:[{size:1000}]}'
  done | jq -s '{results: .}'
}

# GHCR versions page from lines "id days_ago tag,tag" (tags may be empty).
pf_ghcr_versions_json() {
  local id days tags
  while read -r id days tags; do
    [[ -z "$id" ]] && continue
    jq -n --argjson id "$id" --arg d "$(pf_iso_days_ago "$days")" --arg t "${tags:-}" \
      '{id:$id,created_at:$d,metadata:{container:{tags:($t|split(",")|map(select(length>0)))}}}'
  done | jq -s '.'
}

pf_deleted() { grep -c '^DELETE ' "$PF_DIR/calls" || true; }
