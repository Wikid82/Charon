#!/usr/bin/env bats
#
# scripts/prune-ghcr.sh — verified deletes and retention classes, driven by a
# stub curl (see helpers/prune_fixture.bash). No network or credentials.

load helpers/prune_fixture

setup() {
  pf_setup
  export GITHUB_TOKEN=secret-token OWNER=o IMAGE_NAME=charon
  export KEEP_DAYS=30 BUILD_KEEP_DAYS=7 KEEP_LAST_N=2 DRY_RUN=false PRUNE_UNTAGGED=true
  export PROTECTED_REGEX='["^v?[0-9]+\\.[0-9]+\\.[0-9]+$","^latest$","^main$","^develop$"]'
  pf_route GET '*/orgs/o/packages/container/charon/versions?per_page=100&page=2' 200 '[]'
}
teardown() { pf_teardown; }

run_prune() { run bash "$PF_ROOT/scripts/prune-ghcr.sh"; }


@test "204 deletes are counted as deleted and exit 0" {
  pf_route GET '*/orgs/o/packages/container/charon/versions?per_page=100&page=1' 200 "$(pf_ghcr_versions_json <<< '11 10 build-abc1234-amd64')"
  pf_route DELETE '*/versions/11' 204
  run_prune
  [ "$status" -eq 0 ]
  grep -q '^TOTAL_DELETED=1$' prune-summary-ghcr.env
  grep -q '^TOTAL_FAILED=0$' prune-summary-ghcr.env
  [[ "$output" != *"≈"* ]]
}

@test "403 is not counted as deleted, is logged, and exits non-zero" {
  pf_route GET '*/orgs/o/packages/container/charon/versions?per_page=100&page=1' 200 "$(pf_ghcr_versions_json <<< '11 10 build-abc1234-amd64')"
  pf_route DELETE '*/versions/11' 403 '{"message":"Forbidden"}'
  run_prune
  [ "$status" -eq 1 ]
  [[ "$output" == *"delete FAILED http=403"* ]]
  [[ "$output" != *"secret-token"* ]]
  grep -q '^TOTAL_DELETED=0$' prune-summary-ghcr.env
  grep -q '^TOTAL_FAILED=1$' prune-summary-ghcr.env
}

@test "404 on delete is treated as already gone" {
  pf_route GET '*/orgs/o/packages/container/charon/versions?per_page=100&page=1' 200 "$(pf_ghcr_versions_json <<< '11 10 build-abc1234-amd64')"
  pf_route DELETE '*/versions/11' 404 '{"message":"Not Found"}'
  run_prune
  [ "$status" -eq 0 ]
  grep -q '^TOTAL_DELETED=1$' prune-summary-ghcr.env
}

@test "build tags expire after 7 days; nightly and pr tags keep the 30 day rule" {
  pf_route GET '*/orgs/o/packages/container/charon/versions?per_page=100&page=1' 200 "$(pf_ghcr_versions_json <<'T'
1 8 build-aaa1111-amd64
2 3 build-bbb2222-amd64
3 9 sha256-abcdef.sbom
4 8 development-1234567
5 6 main-7654321
6 10 abcdef1
7 10 nightly-2026-01-01
8 40 nightly-2026-01-02
9 35 pr-5
T
)"
  pf_route DELETE '*' 204
  KEEP_LAST_N=0 run_prune
  [ "$status" -eq 0 ]
  for id in 1 3 4 6 8 9; do grep -q "^DELETE .*/versions/$id$" "$PF_DIR/calls"; done
  for id in 2 5 7; do ! grep -q "/versions/$id$" "$PF_DIR/calls"; done
}

@test "protected tags are never deleted, even alongside build tags" {
  pf_route GET '*/orgs/o/packages/container/charon/versions?per_page=100&page=1' 200 "$(pf_ghcr_versions_json <<'T'
1 400 v1.2.3
2 400 latest
3 400 main
4 400 develop
5 400 1.2.3,build-abc1234-amd64
T
)"
  pf_route DELETE '*' 204
  KEEP_LAST_N=0 run_prune
  [ "$status" -eq 0 ]
  [ "$(pf_deleted)" -eq 0 ]
}

@test "last-N shield protects only non-build tags" {
  pf_route GET '*/orgs/o/packages/container/charon/versions?per_page=100&page=1' 200 "$(pf_ghcr_versions_json <<'T'
1 40 build-aaa1111-amd64
2 41 build-bbb2222-amd64
3 50 pr-1
4 51 pr-2
5 52 pr-3
T
)"
  pf_route DELETE '*' 204
  KEEP_LAST_N=2 run_prune
  [ "$status" -eq 0 ]
  for id in 1 2 5; do grep -q "/versions/$id$" "$PF_DIR/calls"; done
  for id in 3 4; do ! grep -q "/versions/$id$" "$PF_DIR/calls"; done
}

@test "untagged versions follow KEEP_DAYS and PRUNE_UNTAGGED" {
  pf_route GET '*/orgs/o/packages/container/charon/versions?per_page=100&page=1' 200 "$(pf_ghcr_versions_json <<'T'
1 40
2 10
T
)"
  pf_route DELETE '*' 204
  run_prune
  [ "$status" -eq 0 ]
  grep -q "/versions/1$" "$PF_DIR/calls"
  ! grep -q "/versions/2$" "$PF_DIR/calls"
  : > "$PF_DIR/calls"
  PRUNE_UNTAGGED=false run_prune
  [ "$(pf_deleted)" -eq 0 ]
}

@test "dry run lists candidates but issues no DELETE" {
  pf_route GET '*/orgs/o/packages/container/charon/versions?per_page=100&page=1' 200 "$(pf_ghcr_versions_json <<< '11 10 build-abc1234-amd64')"
  pf_route DELETE '*' 204
  DRY_RUN=true run_prune
  [ "$status" -eq 0 ]
  [[ "$output" == *"DRY RUN: would delete"* ]]
  [ "$(pf_deleted)" -eq 0 ]
  grep -q '^TOTAL_CANDIDATES=1$' prune-summary-ghcr.env
  grep -q '^TOTAL_DELETED=0$' prune-summary-ghcr.env
}
