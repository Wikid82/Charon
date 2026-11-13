#!/usr/bin/env bash
# Self-test for scripts/ci/shard-security-specs.sh.
# Run: bash scripts/ci/shard-security-specs.test.sh
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo="$(cd "$here/../.." && pwd)"
script="$here/shard-security-specs.sh"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT
export LC_ALL=C

passed=0
failed=0

check() {
  local name="$1"
  shift
  if "$@"; then
    passed=$((passed + 1))
    echo "ok   - $name"
  else
    failed=$((failed + 1))
    echo "FAIL - $name"
  fi
}

# Runs the script expecting a non-zero exit whose output contains the given text.
expect_failure() {
  local needle="$1" out
  shift
  if out="$("$@" 2>&1)"; then
    echo "expected failure, got success: $*" >&2
    return 1
  fi
  grep -qF -- "$needle" <<<"$out" || {
    echo "failure output did not contain '$needle': $out" >&2
    return 1
  }
}

expect_success() {
  "$@" >/dev/null 2>&1
}

# Fixture tree with two specs and a manifest that covers them, optionally with a line tweak.
make_fixture() {
  local root="$1"
  mkdir -p "$root/tests/security-enforcement" "$root/tests/security"
  printf "test('one', async () => {});\ntest('two', async () => {});\n" >"$root/tests/security-enforcement/a.spec.ts"
  printf "test('only', async () => {});\n" >"$root/tests/security/b.spec.ts"
  printf '1 balanced 2 tests/security-enforcement/a.spec.ts\n2 balanced 1 tests/security/b.spec.ts\n' >"$root/manifest.txt"
}

fixture_run() {
  local root="$1" shard="$2" total="$3"
  SECURITY_SPECS_ROOT="$root" SECURITY_SHARD_MANIFEST="$root/manifest.txt" bash "$script" "$shard" "$total"
}

# 1. The committed manifest covers every real spec exactly once across three shards.
real_union_is_exact() {
  local discovered union
  discovered="$(cd "$repo" && find tests/security-enforcement tests/security -type f \( -name '*.spec.ts' -o -name '*.spec.js' \) | sort)"
  union="$(for s in 1 2 3; do bash "$script" "$s" 3; done | sort)"
  [[ "$discovered" == "$union" ]] || return 1
  [[ -z "$(uniq -d <<<"$union")" ]]
}
check "committed manifest assigns every spec exactly once (3 shards)" real_union_is_exact

# 2. Each real shard is non-empty and the report runs.
check "report succeeds on the committed manifest" expect_success bash "$script" --report

# 3. Fixture: a valid manifest emits the expected lists.
make_fixture "$work/ok"
ok_lists() {
  [[ "$(fixture_run "$work/ok" 1 2)" == "tests/security-enforcement/a.spec.ts" ]] &&
    [[ "$(fixture_run "$work/ok" 2 2)" == "tests/security/b.spec.ts" ]]
}
check "valid fixture manifest emits per-shard lists" ok_lists

# 4. A spec missing from the manifest fails loudly.
make_fixture "$work/unlisted"
printf "test('new', async () => {});\n" >"$work/unlisted/tests/security/new.spec.ts"
check "unlisted spec fails the run" expect_failure "not assigned to a shard" fixture_run "$work/unlisted" 1 2

# 5. A manifest entry for a deleted spec fails loudly.
make_fixture "$work/stale"
printf '2 balanced 1 tests/security/gone.spec.ts\n' >>"$work/stale/manifest.txt"
check "stale manifest entry fails the run" expect_failure "do not exist" fixture_run "$work/stale" 1 2

# 6. A spec listed twice fails loudly.
make_fixture "$work/dup"
printf '2 balanced 2 tests/security-enforcement/a.spec.ts\n' >>"$work/dup/manifest.txt"
check "duplicate manifest entry fails the run" expect_failure "more than once" fixture_run "$work/dup" 1 2

# 7. Pinned entries split across shards fail loudly.
make_fixture "$work/split"
printf '1 pinned 2 tests/security-enforcement/a.spec.ts\n2 pinned 1 tests/security/b.spec.ts\n' >"$work/split/manifest.txt"
check "split pinned chain fails the run" expect_failure "split across shards" fixture_run "$work/split" 1 2

# 8. A malformed line fails loudly.
make_fixture "$work/malformed"
printf '2 balanced tests/security/b.spec.ts\n' >"$work/malformed/manifest.txt"
check "malformed manifest line fails the run" expect_failure "malformed manifest line" fixture_run "$work/malformed" 1 2

# 9. Requesting a shard that the manifest leaves empty fails loudly.
make_fixture "$work/empty"
check "empty shard fails the run" expect_failure "has no specs" fixture_run "$work/empty" 1 3

# 10. Out-of-range shard index is rejected.
check "shard index above total is rejected" expect_failure "outside 1..2" fixture_run "$work/ok" 3 2

# 11. A generated manifest for the real tree passes validation and keeps the chain together.
generated="$work/generated.txt"
bash "$script" --generate 3 >"$generated"
generated_is_valid() {
  SECURITY_SHARD_MANIFEST="$generated" bash "$script" 1 3 >/dev/null &&
    SECURITY_SHARD_MANIFEST="$generated" bash "$script" 2 3 >/dev/null &&
    SECURITY_SHARD_MANIFEST="$generated" bash "$script" 3 3 >/dev/null
}
check "generated manifest validates against the real tree" generated_is_valid

# 12. The break-glass chain members all land in one shard.
chain_in_one_shard() {
  local shards
  shards="$(awk '$2 == "pinned" {print $1}' "$repo/scripts/ci/security-shard-manifest.txt" | sort -u)"
  [[ "$(wc -l <<<"$shards")" -eq 1 ]] &&
    grep -q "tests/security-enforcement/zzzz-break-glass-recovery.spec.ts" "$repo/scripts/ci/security-shard-manifest.txt"
}
check "break-glass chain is pinned to one shard" chain_in_one_shard

echo
echo "passed: $passed, failed: $failed"
[[ "$failed" -eq 0 ]]
