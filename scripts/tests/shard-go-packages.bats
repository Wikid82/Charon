#!/usr/bin/env bats
#
# Covers scripts/ci/shard-go-packages.sh. A stub `go` on PATH replays a fixed
# `go list ./...` fixture, so the tests never depend on the real module graph.

setup() {
  REPO_ROOT="$(cd "$BATS_TEST_DIRNAME/../.." && pwd)"
  SCRIPT_UNDER_TEST="$REPO_ROOT/scripts/ci/shard-go-packages.sh"

  TMPROOT="$(mktemp -d)"
  mkdir "$TMPROOT/bin"
  export PATH="$TMPROOT/bin:$PATH"

  MODULE="github.com/example/charon/backend"
  FIXTURE="$TMPROOT/go-list.txt"
  cat >"$FIXTURE" <<LIST
$MODULE/cmd/api
$MODULE/internal/api/handlers
$MODULE/internal/api/middleware
$MODULE/internal/config
$MODULE/internal/database
$MODULE/internal/services
$MODULE/internal/services/remotestorage
$MODULE/internal/util
$MODULE/pkg/safehttp
LIST
  export GO_LIST_FIXTURE="$FIXTURE"

  cat >"$TMPROOT/bin/go" <<'STUB'
#!/usr/bin/env bash
if [[ "$1" == "list" ]]; then
  cat "$GO_LIST_FIXTURE"
  exit 0
fi
echo "unexpected go invocation: $*" >&2
exit 99
STUB
  chmod +x "$TMPROOT/bin/go"
}

teardown() {
  rm -rf "$TMPROOT"
}

shard() {
  bash "$SCRIPT_UNDER_TEST" "$@"
}

@test "heavy packages each get a dedicated shard in a 3-way split" {
  run shard 1 3
  [ "$status" -eq 0 ]
  [ "$output" = "$MODULE/internal/services" ]

  run shard 2 3
  [ "$status" -eq 0 ]
  [ "$output" = "$MODULE/internal/api/handlers" ]
}

@test "remaining packages all land in the last shard of a 3-way split" {
  run shard 3 3
  [ "$status" -eq 0 ]
  [[ "$output" == *"$MODULE/cmd/api"* ]]
  [[ "$output" == *"$MODULE/internal/services/remotestorage"* ]]
  [[ "$output" != *"$MODULE/internal/services"$'\n'* ]]
  [[ "$output" != *"internal/api/handlers"* ]]
  [ "$(printf '%s\n' "$output" | wc -l)" -eq 7 ]
}

@test "every package appears in exactly one shard for totals 1 through 7" {
  local expected actual total index
  expected="$(sort "$FIXTURE")"
  for total in 1 2 3 4 5 6 7; do
    actual=""
    for index in $(seq 1 "$total"); do
      # Shards with no packages are an error by design; skip them here.
      out="$(shard "$index" "$total" 2>/dev/null || true)"
      actual+="${out}"$'\n'
    done
    actual="$(printf '%s' "$actual" | sed '/^$/d' | sort)"
    [ "$actual" = "$expected" ] || {
      echo "coverage mismatch for total=$total" >&2
      return 1
    }
  done
}

@test "assignment is deterministic across runs" {
  run shard 2 4
  first="$output"
  run shard 2 4
  [ "$output" = "$first" ]
}

@test "fewer shards than heavy packages still covers everything once" {
  local combined
  combined="$(shard 1 1)"
  [ "$(printf '%s\n' "$combined" | wc -l)" -eq 9 ]

  first="$(shard 1 2)"
  second="$(shard 2 2)"
  union="$(printf '%s\n%s\n' "$first" "$second" | sed '/^$/d' | sort)"
  [ "$union" = "$(sort "$FIXTURE")" ]
}

@test "a heavy entry missing from go list is ignored" {
  sed -i '/internal\/api\/handlers/d' "$FIXTURE"
  # With handlers gone only services is heavy, so shards 2 and 3 share the rest.
  all=""
  for index in 1 2 3; do
    all+="$(shard "$index" 3)"$'\n'
  done
  [[ "$all" != *"internal/api/handlers"* ]]
  [ "$(printf '%s' "$all" | sed '/^$/d' | wc -l)" -eq 8 ]
  run shard 1 3
  [ "$output" = "$MODULE/internal/services" ]
}

@test "a shard with no packages fails loudly instead of printing nothing" {
  # 10 shards over 9 packages (2 dedicated + 7 spread over 8 slots) leaves
  # shard 10 empty; it must fail rather than print nothing.
  run shard 10 10
  [ "$status" -eq 1 ]
  [[ "$output" == *"has no packages"* ]]

  printf '%s\n' "$MODULE/internal/services" >"$FIXTURE"
  run shard 2 2
  [ "$status" -eq 1 ]
  [[ "$output" == *"has no packages"* ]]
}

@test "an empty go list is an error" {
  : >"$FIXTURE"
  run shard 1 3
  [ "$status" -eq 1 ]
  [[ "$output" == *"no packages"* ]]
}

@test "missing or malformed arguments are rejected" {
  run shard
  [ "$status" -eq 2 ]

  run shard 0 3
  [ "$status" -eq 2 ]

  run shard 4 3
  [ "$status" -eq 2 ]

  run shard one three
  [ "$status" -eq 2 ]
}
