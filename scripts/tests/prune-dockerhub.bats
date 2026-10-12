#!/usr/bin/env bats
#
# scripts/prune-dockerhub.sh — verified deletes and retention classes, driven by
# a stub curl (see helpers/prune_fixture.bash). No network or credentials.

load helpers/prune_fixture

setup() {
  pf_setup
  export DOCKERHUB_USERNAME=u DOCKERHUB_TOKEN=secret-token OWNER=u IMAGE_NAME=charon
  export KEEP_DAYS=30 BUILD_KEEP_DAYS=7 KEEP_LAST_N=2 DRY_RUN=false
  export PROTECTED_REGEX='["^v?[0-9]+\\.[0-9]+\\.[0-9]+$","^latest$","^main$","^develop$"]'
  pf_route POST '*/users/login/' 200 '{"token":"jwt"}'
  pf_route GET '*/tags?page_size=100&page=2' 200 '{"results":[]}'
}
teardown() { pf_teardown; }

run_prune() { run bash "$PF_ROOT/scripts/prune-dockerhub.sh"; }


@test "204 deletes are counted as deleted and exit 0" {
  pf_route GET '*/tags?page_size=100&page=1' 200 "$(pf_dockerhub_tags_json <<< 'build-abc1234-amd64 10')"
  pf_route DELETE '*/tags/build-abc1234-amd64/' 204
  run_prune
  [ "$status" -eq 0 ]
  grep -q '^TOTAL_DELETED=1$' prune-summary-dockerhub.env
  grep -q '^TOTAL_FAILED=0$' prune-summary-dockerhub.env
}

@test "403 is not counted as deleted, is logged, and exits non-zero" {
  pf_route GET '*/tags?page_size=100&page=1' 200 "$(pf_dockerhub_tags_json <<< 'build-abc1234-amd64 10')"
  pf_route DELETE '*/tags/*' 403 '{"detail":"access forbidden"}'
  run_prune
  [ "$status" -eq 1 ]
  [[ "$output" == *"delete FAILED http=403"* ]]
  [[ "$output" == *"access forbidden"* ]]
  [[ "$output" != *"secret-token"* ]]
  grep -q '^TOTAL_DELETED=0$' prune-summary-dockerhub.env
  grep -q '^TOTAL_FAILED=1$' prune-summary-dockerhub.env
}

@test "404 on delete is treated as already gone" {
  pf_route GET '*/tags?page_size=100&page=1' 200 "$(pf_dockerhub_tags_json <<< 'build-abc1234-amd64 10')"
  pf_route DELETE '*/tags/*' 404 '{"message":"not found"}'
  run_prune
  [ "$status" -eq 0 ]
  grep -q '^TOTAL_DELETED=1$' prune-summary-dockerhub.env
  grep -q '^TOTAL_FAILED=0$' prune-summary-dockerhub.env
}

@test "build tags expire after 7 days; nightly and pr tags keep the 30 day rule" {
  pf_route GET '*/tags?page_size=100&page=1' 200 "$(pf_dockerhub_tags_json <<'T'
build-aaa1111-amd64 8
build-bbb2222-amd64 3
sha256-abcdef.sbom 9
development-1234567 8
main-7654321 6
abcdef1 10
nightly-2026-01-01 10
nightly-2026-01-02 40
pr-5 35
T
)"
  pf_route DELETE '*' 204
  KEEP_LAST_N=0 run_prune
  [ "$status" -eq 0 ]
  for t in build-aaa1111-amd64 sha256-abcdef.sbom development-1234567 abcdef1 nightly-2026-01-02 pr-5; do
    grep -q "^DELETE .*/tags/$t/$" "$PF_DIR/calls"
  done
  for t in build-bbb2222-amd64 main-7654321 nightly-2026-01-01; do
    ! grep -q "/tags/$t/" "$PF_DIR/calls"
  done
}

@test "protected tags are never deleted, however old" {
  pf_route GET '*/tags?page_size=100&page=1' 200 "$(pf_dockerhub_tags_json <<'T'
v1.2.3 400
1.2.3 400
latest 400
main 400
develop 400
T
)"
  pf_route DELETE '*' 204
  KEEP_LAST_N=0 run_prune
  [ "$status" -eq 0 ]
  [ "$(pf_deleted)" -eq 0 ]
}

@test "last-N shield protects only non-build tags" {
  pf_route GET '*/tags?page_size=100&page=1' 200 "$(pf_dockerhub_tags_json <<'T'
build-aaa1111-amd64 40
build-bbb2222-amd64 41
pr-1 50
pr-2 51
pr-3 52
T
)"
  pf_route DELETE '*' 204
  KEEP_LAST_N=2 run_prune
  [ "$status" -eq 0 ]
  # Both old build tags go despite being the "newest" overall; the 2 newest pr tags are shielded.
  grep -q '/tags/build-aaa1111-amd64/' "$PF_DIR/calls"
  grep -q '/tags/build-bbb2222-amd64/' "$PF_DIR/calls"
  grep -q '/tags/pr-3/' "$PF_DIR/calls"
  ! grep -q '/tags/pr-1/' "$PF_DIR/calls"
  ! grep -q '/tags/pr-2/' "$PF_DIR/calls"
}

@test "dry run lists candidates but issues no DELETE" {
  pf_route GET '*/tags?page_size=100&page=1' 200 "$(pf_dockerhub_tags_json <<< 'build-abc1234-amd64 10')"
  pf_route DELETE '*' 204
  DRY_RUN=true run_prune
  [ "$status" -eq 0 ]
  [[ "$output" == *"DRY RUN: would delete"* ]]
  [ "$(pf_deleted)" -eq 0 ]
  grep -q '^TOTAL_CANDIDATES=1$' prune-summary-dockerhub.env
  grep -q '^TOTAL_DELETED=0$' prune-summary-dockerhub.env
}

@test "login failure exits non-zero" {
  : > "$PF_DIR/routes"
  pf_route POST '*/users/login/' 401 '{"detail":"bad credentials"}'
  run_prune
  [ "$status" -eq 1 ]
  grep -q '^TOTAL_FAILED=1$' prune-summary-dockerhub.env
}

@test "malformed BUILD_TAG_REGEX is rejected" {
  BUILD_TAG_REGEX='not-json' run_prune
  [ "$status" -eq 1 ]
}
