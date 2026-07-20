# Feature Spec: Orthrus Muzzle Normalization Parity + Agent CI Enforcement (GH #1160 + #1161)

**Type**: Bug fix / CI hardening (single feature, single PR — companion issues sharing one root defect class)
**Branch**: `feature/orthrus` (current working branch; no worktree, no new branch, per `CLAUDE.md`)
**Status**: DRAFT — pending Supervisor review
**Author**: `planning` agent
**Research verified against**: repository HEAD on `feature/orthrus`, 2026-07-20 (5 local, unpushed write-mode commits already applied: `a4be39e2`..`d2fa3154`)

---

## 1. Introduction

### 1.1 Objectives

Fix two GitHub issues that describe the same underlying defect class — two independently hand-maintained Docker API allowlist filters (`backend/internal/orthrus/muzzle.go` and `agent/muzzle/muzzle.go`) that are supposed to enforce identical read-only/write policy but are not provably kept in sync:

1. **GH #1160** — the two filters normalize request paths (version-prefix stripping vs. `path.Clean`) in a different order, so a crafted input can be classified differently by each filter *in isolation*.
2. **GH #1161** — the two allowlists are hand-copied with no structural guard against drift, and `agent/` (a separate Go module) has materially weaker CI enforcement than `backend/` (no staticcheck/lint, no coverage gate, and until the write-mode work landed, no CI-run tests at all).

Both issues were filed against the same discovery: a real production incident where a fix landed in the backend copy and shipped/redeployed twice before anyone noticed the agent copy still rejected the same paths (commits `98a68b67`, `b71cbd62`, `eabf358d`). They are treated as **one feature / one PR** per `CLAUDE.md`'s "One Feature = One PR" rule, because fixing #1160 without also closing the drift-detection gap in #1161 leaves the exact same class of silent divergence free to recur on the next allowlist edit.

### 1.2 Why this plan differs from the issues' literal suggestions

Both issues explicitly invite evaluation rather than blind implementation of their suggested fix ("evaluate, don't blindly follow if a better approach exists"). Research against the **current** state of both files (post write-mode commits, not the pre-write-mode version the issues were originally filed against) changed the recommended scope in two material ways, detailed in Section 2:

- The "shared test fixture" GH #1160 asks for **already exists** (`backend/internal/orthrus/testdata/muzzle_corpus.json`, consumed by both `TestMuzzle_SharedCorpus` and `TestFilter_SharedCorpus`), built during the write-mode work. The remaining gap is corpus **content** (no traversal/edge-case entries), not infrastructure.
- The literal fix ("swap two lines in agent to strip-before-clean") is insufficient: agent's *read-path* matching doesn't use a strip-then-match model at all — it duplicates every pattern into an unversioned and a `"/v*/..."` form, matched via `path.Match`. The `v*` wildcard is **more permissive** than backend's numeric-anchored `^/v\d+\.\d+` regex, which is a second, independently exploitable divergence beyond the one the issue names. Section 2.3 has concrete inputs proving this.

---

## 2. Research Findings

### 2.1 Existing architecture

Orthrus tunnels a remote Docker socket through Charon over an outbound WebSocket + yamux session (`ARCHITECTURE.md` has no dedicated Orthrus section yet — out of scope to add one here). Two independent allowlist filters enforce "read-only unless write mode is opted in, and even then only a fixed six-operation set":

| Filter | File | Module | Runs |
|---|---|---|---|
| `Muzzle` | `backend/internal/orthrus/muzzle.go` | `backend` (imports models, logger, util, GORM-adjacent types) | Charon-side, before a request enters the tunnel |
| `Filter` | `agent/muzzle/muzzle.go` | `agent` (standalone module, 4 direct deps, `FROM scratch` Docker image, `CGO_ENABLED=0`) | On the remote agent binary, immediately before dialing the real Docker socket |

Both are already deliberately duplicated, not shared — `agent/muzzle/muzzle.go`'s own doc comments state this explicitly: *"Duplicated here, not shared via import, because agent/ is a separate Go module built as a minimal standalone binary and does not import backend/ packages."* This plan does not challenge that design decision (see Section 2.5 for why).

### 2.2 What the write-mode commits already built (do not re-build)

The five local commits on `feature/orthrus` already:

1. Added `backend/internal/orthrus/testdata/muzzle_corpus.json` (263 lines, JSON array of `{description, method, path, body?, agent_write_enabled, want_allowed}` cases).
2. Wired `TestMuzzle_SharedCorpus` (`backend/internal/orthrus/muzzle_test.go:505-551`) to load and assert against it via `NewMuzzle(...).ServeHTTP`.
3. Wired `TestFilter_SharedCorpus` (`agent/muzzle/muzzle_test.go:568-617`) to load the **same file** via a relative cross-module path (`../../backend/internal/orthrus/testdata/muzzle_corpus.json`) and assert against `Filter.Allow`.
4. Added a "Run Orthrus agent tests" step (`cd agent && go test ./...`) to `.github/workflows/orthrus-build.yml` and `.github/workflows/nightly-build.yml`, gating the Docker image publish on the agent module's own test suite. Comments in `orthrus-build.yml:104-111` already reference GH #1161 and this spec by name.
5. Named `hostConfigAllowedKeys`, `allowedWriteExactPaths`, `allowedWritePatterns`, `maxContainerCreateBodyBytes`, `mountEntry`, `validateNetworkModeValue`, `validateMountsValue` **identically** in both files, with doc comments cross-referencing each other and warning future editors to keep them in sync.

**Implication for this plan**: the corpus mechanism and one CI test-execution path for `agent/` already exist. This plan's job is to (a) fill the corpus with the edge cases that actually catch the #1160 divergence, (b) fix the divergence agent-side, (c) add a *structural* drift guard the corpus alone can't provide, and (d) extend `agent/`'s CI parity with `backend/` beyond "tests run" to "tests run, lint enforced, coverage gated" — not to re-invent the shared-fixture mechanism.

### 2.3 GH #1160 root cause, confirmed with concrete inputs

**Backend** (`backend/internal/orthrus/muzzle.go:420-426`, `ServeHTTP`):
```
rawPath  := versionPrefixRe.ReplaceAllString(r.URL.Path, "")   // strip /vX.Y first (anchored, numeric only)
stripped := path.Clean("/" + strings.TrimLeft(rawPath, "/"))   // then clean
```
`versionPrefixRe = ^/v\d+\.\d+` is anchored to the **start of the raw, uncleaned path** and requires two numeric groups.

**Agent** (`agent/muzzle/muzzle.go:294-339`, `Allow`): does not use a single normalize-then-match model at all.
- Main read-path loop matches `path.Clean(reqPath)` directly against `allowedPatterns`, a slice that lists **both** an unversioned and a literal `"/v*/..."` form of every entry (`path.Match`, where `v*` matches any single path segment starting with `v` — no digit requirement).
- The `imageDistributionPatterns` (namespaced image inspect) branch and `allowWrite`'s exact-path branch separately compute `unversioned := versionPrefixRe.ReplaceAllString(cleanPath, "")` — i.e., **clean first, then strip** — the literal order reversal GH #1160 describes.
- `allowWrite`'s *pattern* branch (`allowedWritePatterns`) uses neither: it matches `cleanPath` directly against `"/v*/containers/*/start"`-style entries, the same loose-`v*` mechanism as the main read loop.

This produces **three independently confirmed divergences** (traced past the one example in the issue, per `CLAUDE.md`'s Root Cause Analysis Protocol — "don't stop at the first error"):

| # | Input | Backend (current) | Agent (current, pre-fix) | Divergence |
|---|---|---|---|---|
| 1 | `GET /foo/../v1.44/images/x/json` | **403** — regex doesn't match `/foo/...` (not anchored at start); after clean, `/v1.44/images/x/json` fails the `/images/` prefix check | **200** — clean-then-strip reveals `/images/x/json`, matches `imageDistributionPatterns` | The exact case GH #1160 names |
| 2 | `GET /vFOO/containers/json` | **403** — `^/v\d+\.\d+` requires digits, doesn't match `vFOO` | **200** — `path.Match("/v*/containers/json", ...)` treats `v*` as "any segment starting with v" | New: numeric-regex vs. loose-wildcard mismatch, read path |
| 3 | `POST /vFOO/containers/abc/start` (write mode on) | **403** — same regex mismatch, `allowedWritePatterns` pattern `/containers/*/start` doesn't match the un-stripped `/vFOO/...` path | **200** — `allowedWritePatterns` pattern `/v*/containers/*/start` matches via loose `v*` | New: same bug class, but on a **write** endpoint — higher practical severity than the read-only case the issue text discusses |

In every row, backend is strictly more conservative (fails closed); agent is strictly more permissive. This matches the issue's finding that today's real pipeline (backend always runs first) already masks the bug — but confirms it's not confined to the one read-only inspect endpoint the issue used as its example; row 3 shows the same order/looseness bug reaches a write operation once write mode is enabled, which is the higher-value target if the "backend always runs first" assumption is ever violated (e.g., a future direct-to-agent access path, exactly the risk scenario the issue names).

Two traversal cases already exist in `agent/muzzle/muzzle_test.go:161-163` (`TestFilter_Allow`) but were never promoted into the shared corpus, so backend has no equivalent regression test for them:
```
{"GET", "/v1.47/../containers/json", true},      // resolves to /containers/json — allowed
{"GET", "/containers/../../etc/passwd", false},  // resolves to /etc/passwd — blocked
```
Verified: both filters already agree on these two (traversal that doesn't disguise a version prefix isn't affected by the order bug). They belong in the shared corpus anyway, per `CLAUDE.md`'s DRY guidance — one behavioral assertion instead of two independently-maintained ones.

### 2.4 GH #1161 root cause, confirmed against current tooling

`lefthook.yml`'s blocking `pre-commit` stage has two Go-specific commands, both **hardcoded to `backend/`**:
```yaml
go-vet:
  glob: "*.go"                              # matches agent/**/*.go too...
  run: cd backend && go vet ./...           # ...but only ever lints backend/

golangci-lint-fast:
  glob: "*.go"
  run: scripts/pre-commit-hooks/golangci-lint-fast.sh   # script itself: cd "$(dirname "$0")/../../backend"
```
The glob matches any `.go` file anywhere in the repo (including `agent/`), so editing only `agent/muzzle/muzzle.go` **does** trigger both hooks — but each is blind to `agent/` for a different reason, confirmed by reading each command's actual implementation rather than assuming they behave alike:

- **`go-vet`**: the command is exactly `cd backend && go vet ./...`, with **no revision-scoping at all** — it does not consult `--new-from-rev` or any diff. It unconditionally vets the entirety of `backend/` on every trigger, regardless of what changed. It "passes trivially" for an agent-only change not because it detects zero changed files, but because it is hardcoded to `backend/` and never inspects `agent/` in the first place — full stop.
- **`golangci-lint-fast.sh`** (confirmed at `scripts/pre-commit-hooks/golangci-lint-fast.sh:64` and `:68`): does `cd "$(dirname "$0")/../../backend"` and then runs `golangci-lint run --config .golangci-fast.yml --new-from-rev HEAD ./...`. For *this* script, "finds zero changed backend files under `--new-from-rev HEAD`, passes trivially" is the accurate mechanism, since an agent-only commit produces no backend-scoped diff for that flag to report on.

Either way the net effect is identical and the conclusion is unchanged: **`agent/` is not vetted or linted by pre-commit today.** This is the exact, literal mechanism by which the original incident's agent-side omission passed local pre-commit undetected.

`scripts/go-test-coverage.sh` is hardcoded to `BACKEND_DIR="$ROOT_DIR/backend"` — no coverage gate exists for `agent/` at all. `Makefile`'s `check-module-coverage` target (`scripts/check-module-coverage.sh`) is a **dangling reference** — the script does not exist on disk. Confirmed via `ls`; this predates this feature and is an unrelated pre-existing bug, fixed opportunistically in this PR's Commit 5 since it's directly on-topic (module coverage aggregation) and trivial to fix alongside the new agent script.

`go test -cover ./...` in `agent/` today:
```
github.com/Wikid82/charon/agent          0.0% (main.go, 144 lines — CLI bootstrap/flag parsing)
github.com/Wikid82/charon/agent/cert     0.0%, no test files (cert.go, 68 lines — real logic: generates a self-signed ECDSA cert)
github.com/Wikid82/charon/agent/leash   44.4% (has partial test coverage already)
github.com/Wikid82/charon/agent/muzzle  89.5% (well covered)
github.com/Wikid82/charon/agent/protocol  no test files (37 lines, pure type/const declarations, no branches)
```
`agent` (root/main) is a direct structural analogue of `backend/cmd/api`, which `codecov.yml` already excludes as "entrypoints and infrastructure code... tested via integration tests instead." `cert/` is genuine, currently-untested business logic and does not get that exclusion — it needs a real test, which this plan scopes narrowly (Section 3.5).

**CI test execution** (distinct from lint/coverage) for `agent/` already exists as of the write-mode commits (`orthrus-build.yml`, `nightly-build.yml` — Section 2.2, item 4), but only fires on `agent/**` path changes to pull requests, or on push/tag events — it is not part of the unconditional per-PR `quality-checks.yml` gate that `backend-quality`/`frontend-quality` already are.

### 2.5 Design decision: #1161's unification question

Three options were evaluated, per the task's explicit request to justify the call rather than default to the most elaborate option:

**Option A — Shared importable Go package.** Move the allowlist definitions into a package both `backend/internal/orthrus` and `agent/muzzle` import. Rejected: `agent/go.mod` has 4 direct dependencies and builds to a `FROM scratch`, `CGO_ENABLED=0` image; `backend/internal/orthrus` imports `models`, `logger`, `util`, and (transitively, through `services` → `orthrus`) GORM and Gin-adjacent packages. A shared package would need to live somewhere neither module's existing import graph implies, and `agent` importing anything under `backend/` inverts the module boundary the code's own doc comments say is intentional. This is the elaborate option and was rejected on concrete dependency-graph grounds, not by default.

**Option B — Codegen from a single source-of-truth data file.** A YAML/JSON policy file, code-generated into each module's native types at build time. Rejected for this PR: adds a new build-time toolchain step (`go generate` wiring in two modules, generated-file staleness as its own drift class) for policy data that changes rarely — the allowlists have been touched a handful of times total across the file's history. Disproportionate complexity relative to change frequency. Not ruled out permanently; worth revisiting if the allowlist starts changing often.

**Option C — Structural CI drift-check (chosen), layered on top of the existing behavioral corpus.** The corpus (already built, Section 2.2) proves *outcome* parity for the specific inputs it contains — it cannot catch "a new pattern was added to one file's map literal and nobody thought to add a corresponding corpus case," which is precisely what happened in the original incident. A second, independent guard is needed that compares the *declared allowlist contents* themselves, not just sampled behavior. This is Option C: a small `go/parser`-based Go program (stdlib only, no new dependency) that extracts the string-literal contents of the two files' named allowlist declarations and fails if they differ, run pre-commit and in CI. Chosen because it directly closes the exact gap that caused the incident, adds no new runtime dependency or build step, and its scope (Section 3.3) is deliberately bounded to declarative data (paths, patterns, key sets, the body-size constant) rather than attempting to diff function-body *logic* (`validateNetworkModeValue`/`validateMountsValue`), which stays covered by the existing corpus and code-review cross-references — an explicitly accepted, documented limitation, not a silent gap.

Sequencing note: Option C's checker is **much simpler to write correctly** if agent's allowlist declarations are first collapsed to the same *shape* as backend's (Section 3.1's refactor removes the `/v*/...` duplicate entries and renames variables to match backend 1:1). This is why the Commit Slicing Strategy (Section 5) puts the agent normalization fix (which does this collapse as a side effect) *before* the parity-checker commit, not after.

---

## 3. Technical Specifications

### 3.1 `agent/muzzle/muzzle.go` — normalization refactor (fixes #1160)

Introduce one normalization function, used everywhere a path is matched, mirroring backend's order exactly:

```
// normalizeDockerPath strips a Docker API version prefix (e.g. "/v1.47"),
// matching backend's versionPrefixRe exactly, THEN runs path.Clean — same
// order as backend/internal/orthrus/muzzle.go's ServeHTTP, so both filters
// reach the same decision on the same input in isolation, not only when
// backend happens to run first in the real request pipeline.
func normalizeDockerPath(reqPath string) string {
    stripped := versionPrefixRe.ReplaceAllString(reqPath, "")
    return path.Clean("/" + strings.TrimLeft(stripped, "/"))
}
```
`versionPrefixRe` (already `^/v\d+\.\d+`, already present in the file at line 79) is reused as-is — no change to the regex itself, only to when it runs relative to `Clean`.

Collapse the duplicated-pattern lists to unversioned-only, matching backend's split into an exact-match set and a dynamic-pattern set, and **rename to match backend 1:1** (enables the Commit 4 parity checker to pair declarations by name):

| Current agent name/shape | New name/shape | Rationale |
|---|---|---|
| `allowedPatterns []string` (mixed exact + dynamic + `/v*/` dupes) | `allowedDockerPaths map[string]struct{}` (exact, unversioned only) + `allowedDockerPatterns []string` (dynamic, unversioned only) | Matches backend's split exactly; halves the list length by dropping the now-redundant `/v*/` dupes |
| `imageDistributionPatterns []struct{prefix,suffix string}` | `allowedDockerPrefixSuffixPatterns []struct{prefix,suffix string}` | Name parity with backend |
| (HEAD `/_ping` handling: loop over `{"/_ping","/v*/_ping"}` via `path.Match`) | `normalizeDockerPath(reqPath) == "/_ping"` | Single check, same numeric-anchored regex as everywhere else — closes divergence row 2/3 above for the HEAD path too |

`allowedWriteExactPaths` and `allowedWritePatterns` keep their current names (already match backend) but change internally:
- Drop the `/v*/...` duplicate entries from `allowedWritePatterns` (4 entries → keep the unversioned form only).
- `allowWrite`'s signature changes from `allowWrite(method, cleanPath string, body []byte) bool` to `allowWrite(method, normalizedPath string, body []byte) bool` — the caller (`Allow`) computes `normalizeDockerPath` once and passes the result in, instead of `allowWrite` separately re-deriving `unversioned` for the exact-path branch only (closing divergence row 3 — the pattern-loop branch now uses the same normalized path as the exact-path branch, not raw `cleanPath`).

`Allow`'s public signature (`Allow(method, reqPath string, body []byte) bool`) is unchanged — this is an internal-only refactor. `ServeProxy`, the only caller, needs no change.

No change to `backend/internal/orthrus/muzzle.go`'s runtime logic — its order is already the reference. Proposed: extract its existing two-line strip+clean sequence (`ServeHTTP` lines 420-426) into a same-named `normalizeDockerPath(rawPath string) string` helper, purely for symmetry with the agent-side helper and to give the Commit 4 parity checker (and future readers) one obviously-paired function name to look for in each file. This is a no-behavior-change refactor, verified by the existing full backend test suite passing unmodified.

### 3.2 Shared corpus additions (fixes the "no shared fixture for edge cases" gap in #1160)

New entries for `backend/internal/orthrus/testdata/muzzle_corpus.json` (consumed by both `TestMuzzle_SharedCorpus` and `TestFilter_SharedCorpus` — no new test-loading code needed, per Section 2.2):

| Description | Method | Path | Write enabled | Want allowed | Why it belongs in the corpus |
|---|---|---|---|---|---|
| traversal-disguised version prefix on namespaced image inspect (GH #1160's own example) | GET | `/foo/../v1.44/images/x/json` | false | **false** | Divergence row 1 — red on agent pre-fix |
| non-numeric fake version prefix on a plain read path | GET | `/vFOO/containers/json` | false | **false** | Divergence row 2 — red on agent pre-fix |
| non-numeric fake version prefix on `/_ping` via HEAD | HEAD | `/vBOGUS/_ping` | false | **false** | Same bug class, HEAD special-case |
| non-numeric fake version prefix reaching a write endpoint | POST | `/vFOO/containers/abc/start` | true | **false** | Divergence row 3 — highest-severity case found |
| version-prefixed traversal that legitimately resolves to an allowed path | GET | `/v1.47/../containers/json` | false | true | Migrated from agent-only test; proves traversal handling isn't broken by the fix, not just that it's stricter |
| traversal escaping to a disallowed path | GET | `/containers/../../etc/passwd` | false | false | Migrated from agent-only test; both filters already agree — regression guard |
| double-slash with legitimate version prefix | GET | `/v1.44//containers/json` | false | true | Confirms `path.Clean`'s double-slash collapsing is unaffected by the reordering — both filters already agree, kept as an explicit regression case |

All seven are single `want_allowed` values shared by both filters — by design, since the corpus format has no per-filter expected-value column (`muzzleCorpusCase` in both `_test.go` files), and the whole point of Section 3.1's fix is that both filters must agree.

**Expected state after Commit 1 alone (corpus additions, before Commit 3's agent fix)**: `TestMuzzle_SharedCorpus` (backend) passes unchanged — backend's current behavior already matches every new expected value. `TestFilter_SharedCorpus` (agent) fails on the 4 divergence rows — this is the intended red state proving the bug, consistent with `CLAUDE.md`'s suggested commit sequence ("E2E specs for new behavior... as test.fixme" — the Go-test equivalent here is "corpus case added and failing, not yet fixed" — see Section 5 Commit 1 validation gate for how this red state is handled without claiming the PR is done mid-sequence).

### 3.3 Structural allowlist parity checker (fixes #1161 workstream 1)

New file: `scripts/ci/check_muzzle_allowlist_parity.go` (package `main`, stdlib-only: `go/ast`, `go/parser`, `go/token`).

Design:
1. Parse both `backend/internal/orthrus/muzzle.go` and `agent/muzzle/muzzle.go` with `go/parser.ParseFile`.
2. Walk each file's top-level `var`/`const` declarations, extracting the literal contents of a fixed, named set of declarations (paired by identical name across both files, per the Section 3.1 renaming). Two distinct AST shapes are handled: composite literals (map/slice literals, and the single int constant) for seven of the eight declarations below, and — for `versionPrefixRe` specifically — the string-literal first argument of the `regexp.MustCompile(...)` call assigned to it. `versionPrefixRe` is declared as `var versionPrefixRe = regexp.MustCompile(`^/v\d+\.\d+`)` in both files (`backend/internal/orthrus/muzzle.go:38`, `agent/muzzle/muzzle.go:79`) — an `*ast.CallExpr`, not a composite literal — so the checker's extraction function special-cases this one declaration by name, unwraps the call expression's single argument, and asserts it is an `*ast.BasicLit` of kind `STRING` before comparing its literal value:

| Declaration name | Extracted as | Normalization before compare |
|---|---|---|
| `versionPrefixRe` | regex source string (the raw literal passed to `regexp.MustCompile`) | none — must be byte-identical; this is the literal regex named in requirement R2 and is the actual root-cause artifact of GH #1160 (Section 2.3) — the class of drift a future one-sided "fix" to `^/v\w+` or similar would silently introduce if this row didn't exist |
| `allowedDockerPaths` | set of string keys | none — already unversioned-only in both files post-3.1 |
| `allowedDockerPatterns` | set of strings | none |
| `allowedDockerPrefixSuffixPatterns` | set of `(prefix, suffix)` pairs | none |
| `allowedWriteExactPaths` | set of string keys | none |
| `allowedWritePatterns` | set of `(method, pattern)` pairs | none |
| `hostConfigAllowedKeys` | set of string keys | none |
| `maxContainerCreateBodyBytes` | single int literal | none — must be byte-identical, both files' doc comments already assert this |

3. Diff each pair; on any mismatch, print the specific missing/extra entries (e.g. `"allowedWritePatterns: present in backend, missing in agent: {DELETE /containers/*}"`, or for the regex row, `"versionPrefixRe: backend=^/v\d+\.\d+ agent=^/v\w+ (source strings differ)"`) and exit non-zero.
4. Explicitly out of scope, documented in the tool's own header comment: `validateNetworkModeValue`, `validateMountsValue`, and `validateContainerCreateBody`'s logic bodies are **not** structurally diffed — AST-diffing function bodies for semantic (not just textual) equivalence is a much harder problem than diffing data-literal declarations, and the existing shared corpus (Section 3.2) plus the code's own cross-referencing doc comments remain the guard for those. This limitation is called out in the plan and in the tool's usage banner, not silently accepted.

Wired into:
- `lefthook.yml` pre-commit stage, new command `muzzle-allowlist-parity`, `glob: "{backend/internal/orthrus/muzzle.go,agent/muzzle/muzzle.go}"`, `run: go run scripts/ci/check_muzzle_allowlist_parity.go`.
- `.github/workflows/quality-checks.yml`, new small job `muzzle-allowlist-parity` (mirrors the existing `auth-route-protection-contract` job's pattern of a focused, fast, always-run contract-test job), so the guard applies on every PR regardless of whether the PR's diff happens to touch the glob (belt-and-suspenders: lefthook only catches it if a *local* commit touches the file; CI catches it even if lefthook was bypassed with `--no-verify` or the change came from a non-standard path).

### 3.4 Agent CI tooling parity (fixes #1161 workstream 2)

| Backend has | Agent gets |
|---|---|
| `backend/.golangci-fast.yml` | Moved to repo-root `.golangci-fast.yml` (shared config, one source of truth — DRY per `CLAUDE.md`); both modules' lint scripts reference it via relative path |
| `lefthook.yml` `go-vet` (`cd backend && go vet ./...`) | New sibling command `go-vet-agent` (`cd agent && go vet ./...`), `glob: "agent/**/*.go"` — existing `go-vet` command's glob narrowed to `backend/**/*.go` so each command's failure clearly attributes to its module |
| `scripts/pre-commit-hooks/golangci-lint-fast.sh` / `-full.sh` | Both scripts extended to loop over `("backend" "agent")`, running the (now-root) shared config against each module dir in turn |
| `scripts/go-test-coverage.sh` (backend, 87% default gate) | New `scripts/agent-test-coverage.sh` — same coverage-gate arithmetic (line-coverage-from-coverprofile), no encryption-key bootstrapping (agent has none), `EXCLUDE_PACKAGES=("github.com/Wikid82/charon/agent")` (root/main only — mirrors `backend/cmd/api`'s exclusion precedent; `cert/` and `protocol/` are **not** excluded, see Section 3.5 for the tests that bring them into scope). Writes `agent/coverage.txt` in the standard `go test -coverprofile` format (same shape as `backend/coverage.txt`), the input both Section 3.4.1's patch-coverage extension and Section 3.4.2's Codecov upload consume. |
| `lefthook.yml` `testing` manual pipeline (`go-test-coverage`) | New sibling command `agent-test-coverage`, `glob: "agent/**/*.go"` |
| `quality-checks.yml` job `backend-quality` | New job `agent-quality` — `go vet`, golangci-lint (fast config, root-shared) against `agent/`, `scripts/agent-test-coverage.sh`; deliberately **not** a copy of every `backend-quality` step (no encryption-key resolution, no GORM scan — agent has no models, no Perf-assert step — agent has no equivalent benchmark suite) |
| `Makefile` `check-module-coverage` (dangling script reference; stale `"backend + frontend"` echo text) | Implemented as a thin wrapper invoking `scripts/go-test-coverage.sh` then `scripts/agent-test-coverage.sh` in sequence, fixing the pre-existing dangling reference found during this PR's research. The target's echo string is corrected from `"backend + frontend"` to `"backend + agent"` — not `"backend + frontend + agent"` — because the wrapper genuinely only ever invokes the backend and agent scripts; frontend coverage is gated by its own separate script/target (`scripts/frontend-test-coverage.sh`) and was never actually run by this target, so the old text was wrong about what the target does, not merely under-scoped for a future third module. |

`.gitignore`/`.dockerignore` need no changes: `coverage.txt`/`coverage.out` patterns in `.gitignore` (line 163, unanchored — no leading `/`) already match `agent/coverage.txt`. Verified by reading both files during research — explicitly confirmed rather than assumed.

#### 3.4.1 Local patch-coverage tooling parity (closes the `local-patch-report.sh` visibility gap)

`scripts/local-patch-report.sh` is the script CLAUDE.md's Definition of Done Step 2 makes MANDATORY. It is a thin wrapper: it resolves `BACKEND_COVERAGE_FILE`/`FRONTEND_COVERAGE_FILE`, checks they exist, then shells out to `go run ./cmd/localpatchreport` (from `backend/`), which does the actual diff-vs-coverage computation via `backend/internal/patchreport`. Extending it for `agent/` requires changes at both layers — the wrapper script and the Go tool it calls — not just the shell script; a shell-only fix would add an `agent/coverage.txt` existence check but the Go tool underneath would still have no way to attribute changed `agent/**` lines to a coverage scope.

**`scripts/local-patch-report.sh`**:
- Add `AGENT_COVERAGE_FILE="$ROOT_DIR/agent/coverage.txt"` alongside the existing `BACKEND_COVERAGE_FILE`/`FRONTEND_COVERAGE_FILE` (line 6-7).
- Add the same missing-input guard pattern already used for backend/frontend (lines 81-91): if absent, call `write_preflight_artifacts` with an agent-specific reason string and exit 1.
- Pass `--agent-coverage "$AGENT_COVERAGE_FILE"` through to the `go run ./cmd/localpatchreport` invocation (alongside the existing `--backend-coverage`/`--frontend-coverage` args, line 108-113).

**`backend/cmd/localpatchreport/main.go`** (the Go tool itself):
- New flag `agentCoverageFlag := flag.String("agent-coverage", "agent/coverage.txt", "Agent Go coverage profile")`, resolved and existence-checked the same way as `backendCoverageFlag`/`frontendCoverageFlag` (lines 50-76).
- Parse it with the existing `patchreport.ParseGoCoverageProfile` (line 90-94) — reused as-is, since agent's coverage profile is the same `go test -coverprofile` format as backend's; no new parser needed. **Caveat requiring a real code change, not just a new flag** (see `patchreport.go` bullet below): `ParseGoCoverageProfile` internally normalizes every file path through `normalizeGoCoveragePath`, which is currently hardcoded to backend's prefix-stripping rules (`strings.Index(cleaned, "/backend/")`, and a `cmd/`/`internal/`/`pkg/`/`api/`/`integration/`/`tools/` fallback list). Calling it unmodified against an agent coverage profile would leave lines like `github.com/Wikid82/charon/agent/muzzle/muzzle.go` un-normalized (no `/backend/` substring, no matching fallback prefix) — they'd never intersect with the diff-parsed `agent/muzzle/muzzle.go` changed-line set, silently producing 0% "patch coverage" for every agent file regardless of actual test coverage. This is exactly the kind of masked-by-a-different-mechanism bug the Root Cause Analysis Protocol warns about, so it's called out explicitly rather than left for a future bug report.
- Add `CHARON_AGENT_PATCH_COVERAGE_MIN` threshold resolution, `patchreport.ResolveThreshold("CHARON_AGENT_PATCH_COVERAGE_MIN", 85, nil)` — default 85, matching backend's and frontend's existing patch-coverage defaults (distinct from, and not to be confused with, `agent-test-coverage.sh`'s separate *aggregate*-coverage gate threshold from Section 3.4/3.5, which is calibrated to the module's actual total coverage rather than defaulted to 85).
- Extend `thresholdJSON`, `thresholdSourcesJSON`, and `reportJSON` structs (lines 16-45) with an `Agent`/`agent` field each, alongside the existing `Backend`/`Frontend` fields.
- Compute `agentScope := patchreport.ComputeScopeCoverage(agentChanged, agentCoverage)` and `agentFilesNeedingCoverage := patchreport.ComputeFilesNeedingCoverage(agentChanged, agentCoverage, agentThreshold.Value)` (mirrors lines 105-109 exactly). `overallScope` becomes `patchreport.MergeScopeCoverage(backendScope, frontendScope, agentScope)` — `MergeScopeCoverage` and `MergeFileCoverageDetails` are already variadic (`...ScopeCoverage` / `...[]FileCoverageDetail`), so this is an additive call-site change, not a signature break.
- Add an "Agent patch coverage below threshold" warning branch mirroring the existing backend/frontend ones (lines 124-129).
- `writeMarkdown` (line 229 on): add an "Agent coverage" line under `## Inputs`, an "Agent" row under `## Resolved Thresholds`, and an "Agent" row under `## Coverage Summary` (`scopeRow("Agent", report.Agent)`), mirroring the existing Backend/Frontend rows exactly.

**`backend/internal/patchreport/patchreport.go`** (the actual diff-attribution and path-normalization logic — the part of the tool that needs a genuine signature change, not just a new caller-side flag):
- `ParseUnifiedDiffChangedLines(diffContent string) (FileLineSet, FileLineSet, error)` (line 82) returns exactly two `FileLineSet`s today, hardcoded to `backend/` and `frontend/` path prefixes (lines 106-112). Extend its return arity to three: `(backendChanged, frontendChanged, agentChanged FileLineSet, err error)`, adding an `else if strings.HasPrefix(newFile, "agent/") { currentFile = newFile; currentScope = "agent" }` branch alongside the existing two. This is a breaking signature change to an exported function — every call site and every existing test that destructures its two return values must be updated (see test-file impact below); it is the one unavoidable non-additive change in this workstream.
- `normalizeGoCoveragePath` (line 469) is backend-specific by name and by hardcoded prefix list. Generalize it to `normalizeGoCoveragePath(input, moduleDir string) string`, parameterizing the `"/" + moduleDir + "/"` substring search and the `moduleDir + "/"` prefix-check/prepend (replacing the literal `"backend/"` occurrences at lines 475, 478, 485), and drop the backend-only `cmd/`/`internal/`/`pkg/`/`api/`/`integration/`/`tools/` fallback special-case in favor of a per-module fallback prefix list passed alongside `moduleDir`, since `agent/`'s own top-level packages (`cert/`, `leash/`, `muzzle/`, `protocol/`) are a different set. `ParseGoCoverageProfile`'s single call site (line 208) becomes `normalizeGoCoveragePath(filePart, moduleDir)`, so `ParseGoCoverageProfile` itself gains a `moduleDir string` parameter, called as `patchreport.ParseGoCoverageProfile(backendCoveragePath, "backend")` and `patchreport.ParseGoCoverageProfile(agentCoveragePath, "agent")` from `main.go`. This generalization (parameterize, don't duplicate) follows CLAUDE.md's DRY guidance directly, and is what actually closes the silent-0%-coverage caveat above — a new flag alone would not.
- `backend/internal/patchreport/patchreport_test.go`: update every test that calls `ParseUnifiedDiffChangedLines` or `ParseGoCoverageProfile`/`normalizeGoCoveragePath` for the new arity/signature, and add new test cases asserting `agent/`-prefixed diff hunks are attributed to the third return value and that an agent-module coverage profile line normalizes to a repo-relative `agent/...` path.
- `backend/cmd/localpatchreport/main_test.go`: add `-agent-coverage` CLI-flag coverage (the many existing `-backend-coverage`/`-frontend-coverage` call sites at lines 49-50, 556-557, 766-767, 1074, 1086, 1579-1580 are flag-string literals, not Go call sites, and are unaffected by the internal signature changes above — but new tests are needed for the added `Agent` report field, the missing-agent-coverage-file error path, and the merged-overall-includes-agent behavior).

#### 3.4.2 `agent-codecov` job (fixes #1161's remaining CI-dashboard-visibility gap)

`.github/workflows/codecov-upload.yml` has `backend-codecov` (`flags: backend`, uploads `backend/coverage.txt`) and `frontend-codecov` (`flags: frontend`, uploads `frontend/coverage` directory via `codecov/codecov-action`). Add a third job, `agent-codecov`:

- Triggered by the same `if: ${{ github.event_name != 'workflow_dispatch' || inputs.run_agent }}` pattern as the other two, requiring a new `run_agent` `workflow_dispatch` boolean input (default `true`) alongside the existing `run_backend`/`run_frontend` inputs (lines 9-19).
- `actions/setup-go` pointed at `go-version-file: agent/go.mod`, `cache-dependency-path: agent/go.sum` (mirrors `backend-codecov`'s Go setup, lines 47-52).
- **No** "Resolve encryption key" step — agent has no encryption-key bootstrapping (consistent with Section 3.4's `agent-quality` job already omitting this for the same reason; agent's tests don't touch `CHARON_ENCRYPTION_KEY_TEST`).
- Runs `bash scripts/agent-test-coverage.sh` (new in this PR, Section 3.4) in place of `bash scripts/go-test-coverage.sh`, teed to `agent/test-output.txt`, uploaded as an artifact — mirrors `backend-codecov`'s test-and-tee-output pattern (lines 132-146) minus `CGO_ENABLED: 1` (no cgo dependency in `agent/`, confirmed by its `CGO_ENABLED=0` `FROM scratch` Docker build, Section 2.1).
- `codecov/codecov-action` step: `files: ./agent/coverage.txt`, `flags: agent`, `fail_ci_if_error: true` — same pinned action version (`@fb8b3582c8e4def4969c97caa2f19720cb33a72f # v7.0.0`) as the other two jobs, for consistency.

**`codecov.yml` — one necessary addition, correcting the prior draft's blanket "no codecov.yml changes needed" claim**: `backend/cmd/api/**` (backend's bootstrap entrypoint) is explicitly excluded from Codecov's own coverage accounting via the `ignore:` list (line 115), separate from and in addition to `go-test-coverage.sh`'s local `EXCLUDE_PACKAGES` gate-arithmetic exclusion — the raw `coverage.txt` uploaded to Codecov still contains `backend/cmd/api/**` lines (`EXCLUDE_PACKAGES` only affects the local threshold calculation, not what's written to the coverprofile, confirmed by reading `scripts/go-test-coverage.sh`), so `codecov.yml`'s own ignore list is what actually keeps it out of Codecov's reported numbers. `agent/main.go` is agent's direct structural analogue (CLI bootstrap/flag parsing, already excluded from `agent-test-coverage.sh`'s local gate via `EXCLUDE_PACKAGES` per Section 3.4) and needs the same treatment for the new `agent` Codecov flag to report a meaningful, non-diluted number: add `- "agent/main.go"` to `codecov.yml`'s `ignore:` list, in the same "Main entry points (bootstrap code only)" grouping as the existing `backend/cmd/api/**` entry (line 114-115). The earlier "no `codecov.yml` changes needed" statement was correct only for the `scripts/**` ignore pattern (still true, unchanged, still needs no new entries for the new `scripts/ci/check_muzzle_allowlist_parity.go` / `scripts/agent-test-coverage.sh` files) — it did not anticipate this PR adding a new Codecov flag with its own entrypoint-dilution concern, which is a materially different question.

### 3.5 New unit tests required to make the agent coverage gate meaningful (not just passing)

- `agent/cert/cert_test.go` — `LoadOrGenerate` currently has zero coverage and is genuine logic (ECDSA key generation, self-signed cert templating, PEM encoding). New test asserts: no error on a normal call; returned `*tls.Config.Certificates` has exactly one entry; `MinVersion == tls.VersionTLS13`; the leaf certificate's `Subject.CommonName` contains the passed `agentID`; the certificate parses successfully via `x509.ParseCertificate` on the DER bytes recovered from the returned `tls.Certificate`.
- `agent/protocol/message_test.go` — `protocol` has no logic (pure `MessageType` constants + a plain struct), so no behavior to assert; a minimal test asserting the four `MessageType` constants have their documented distinct byte values (`TypePing=0x00` .. `TypeError=0x04`) is added mainly so `go test ./...` doesn't report `[no test files]` for a package now in-scope for the coverage gate, and to guard against a future accidental constant-value change (wire-protocol compatibility).

`agent/leash` (44.4%, pre-existing partial coverage) is explicitly **not** required to increase coverage in this PR — raising it is unrelated to #1160/#1161 and would be scope creep; the coverage gate's initial threshold (Section 5, Commit 5) is calibrated to pass at today's actual aggregate, not an arbitrary target.

---

## 4. EARS Requirements

| ID | Requirement |
|---|---|
| R1 | WHEN a request path contains a version-prefix segment revealed only after path-traversal normalization, THE agent Filter SHALL reach the same allow/deny decision as the backend Muzzle would reach on the same raw input, evaluated independently of request order. |
| R2 | WHILE evaluating whether a path segment is a Docker API version prefix, THE agent Filter SHALL require the segment to match `^/v\d+\.\d+` (numeric major/minor), and SHALL NOT accept an arbitrary `v`-prefixed segment as a version prefix. |
| R3 | WHERE a POST/DELETE request targets a write-mode endpoint pattern, THE agent Filter SHALL apply the identical normalized-path computation used for exact-path write matching, so pattern-matched and exact-matched write paths are not evaluated under different normalization rules within the same file. |
| R4 | THE shared corpus (`muzzle_corpus.json`) SHALL contain at least one entry per confirmed divergence class found during this investigation (traversal-disguised version prefix, non-numeric fake version prefix on a read path, on a HEAD path, and on a write path), asserted identically against both `Muzzle.ServeHTTP` and `Filter.Allow`. |
| R5 | WHEN a developer edits the read-only or write allowlist declarations in either `backend/internal/orthrus/muzzle.go` or `agent/muzzle/muzzle.go`, THE pre-commit hook SHALL run the structural parity checker AND SHALL block the commit if the paired declarations' literal contents differ. |
| R6 | THE structural parity checker SHALL explicitly report, not silently skip, any declaration pair it cannot compare (e.g. a renamed or missing variable), so a refactor that breaks the checker's assumptions fails loudly rather than passing vacuously. |
| R7 | WHEN a pull request is opened or updated, THE CI pipeline SHALL run `go vet`, golangci-lint (fast config), and a minimum test-coverage check against the `agent/` module on every PR, not only PRs whose diff touches `agent/**`. |
| R8 | THE agent coverage gate's minimum threshold SHALL be set no higher than the module's actual coverage after this PR's new tests land, so the gate is enforced from a passing baseline rather than requiring an unrelated test-writing effort to first go green. |
| R9 | THE structural parity checker SHALL compare the `versionPrefixRe` regex source string declared in each file byte-for-byte, in addition to the six allowlist-data declarations, so a future one-sided loosening of the version-prefix regex — the literal defect class of GH #1160 — is caught structurally rather than only by corpus luck. |
| R10 | THE local patch-coverage tooling (`scripts/local-patch-report.sh` and `backend/cmd/localpatchreport`) SHALL compute and report patch coverage for the `agent/` module using the same changed-line-diff-and-coverage-profile-intersection mechanism already used for `backend/` and `frontend/`, so CLAUDE.md's mandatory Definition of Done Step 2 gate has visibility into `agent/muzzle/muzzle.go`, the file this PR's security fix actually lives in. |
| R11 | WHEN backend and frontend coverage are uploaded to Codecov on a pull request, THE CI pipeline SHALL also upload `agent/` coverage under a distinct `agent` flag, so agent coverage appears on Codecov's dashboard and in PR comments alongside backend/frontend, matching R7's CI-parity requirement. |

---

## 5. Commit Slicing Strategy

**Decision**: Single PR on `feature/orthrus`, six ordered commits within it. `CLAUDE.md`'s "One Feature = One PR" rule applies — this is one feature (muzzle normalization parity + the CI enforcement gap that let it happen), not two.

Adaptation from `CLAUDE.md`'s suggested 5-step shape: this change has no frontend and no user-facing E2E surface (Section 6 confirms Playwright is N/A), so "Phase 1: E2E specs as test.fixme" is replaced with "Phase 1: failing corpus cases," the Go-test equivalent of a red-first commit for backend/CI-only work.

---

### Commit 1 — Foundation: shared corpus edge cases (no production code change)

**Scope**: Add the 7 new corpus entries from Section 3.2 to `backend/internal/orthrus/testdata/muzzle_corpus.json`. No changes to `muzzle.go` in either module.

**Files**: `backend/internal/orthrus/testdata/muzzle_corpus.json`

**Dependencies**: None — first commit.

**Expected state**: `TestMuzzle_SharedCorpus` (backend) — **passes** (backend's current behavior already correct). `TestFilter_SharedCorpus` (agent) — **fails on 4 of the 7 new cases** (the divergence rows). This is an intentionally red commit for the agent side, analogous to `CLAUDE.md`'s "E2E specs... as test.fixme" step — the failing corpus cases are the executable proof of the bug, committed before the fix. Because this commit lands inside a single feature PR (not on `main` directly — `CLAUDE.md`'s Definition of Done applies to the PR as a whole, not each intermediate commit), the red state here is expected and reviewed as such, not a violation of the "all tests pass" gate, which is evaluated at the PR's final state (post Commit 3).

**Validation gate**: `cd backend && go test ./internal/orthrus/... -run TestMuzzle_SharedCorpus -v` (expect PASS). `cd agent && go test ./muzzle/... -run TestFilter_SharedCorpus -v` (expect the 4 documented failures, no others — reviewer confirms the failure list matches exactly Section 3.2's 4 divergence rows, not something unrelated).

---

### Commit 2 — Backend: extract `normalizeDockerPath` helper (no behavior change)

**Scope**: Per Section 3.1's last paragraph — extract `ServeHTTP`'s existing two-line strip+clean sequence into a named `normalizeDockerPath(rawPath string) string` helper for naming symmetry with the forthcoming agent-side helper and to give the Commit 4 parity checker (and human reviewers) a stable, identically-named anchor in both files. Backend's runtime behavior is unchanged — this commit is a pure refactor.

**Files**: `backend/internal/orthrus/muzzle.go`, `backend/internal/orthrus/muzzle_test.go` (add direct unit tests for divergence rows 1-2 using `normalizeDockerPath` directly, not only via the corpus, for fast local iteration)

**Dependencies**: None functionally, but ordered after Commit 1 so its new direct unit tests can be written against the same edge cases the corpus already encodes, avoiding duplicate case invention.

**Validation gate**: `cd backend && go test ./internal/orthrus/... -v` (full package, expect 100% pass — no behavior change means every pre-existing test, including the new Commit 1 corpus cases for backend, stays green). `make lint-staticcheck-only`.

---

### Commit 3 — Agent: normalization-order fix + allowlist renaming (fixes GH #1160)

**Scope**: The refactor in Section 3.1 — add `normalizeDockerPath`, split `allowedPatterns` into `allowedDockerPaths`/`allowedDockerPatterns`, rename `imageDistributionPatterns` → `allowedDockerPrefixSuffixPatterns`, drop all `/v*/...` duplicate pattern entries (now redundant — the version prefix is stripped once, up front), change the HEAD `/_ping` check to use `normalizeDockerPath`, and change `allowWrite`'s signature to accept the pre-normalized path so its pattern branch stops using raw `cleanPath`.

**Files**: `agent/muzzle/muzzle.go`, `agent/muzzle/muzzle_test.go` (update any assertions that specifically exercised the now-removed `/v*/...` duplicate-pattern mechanism — expected outcomes are unchanged for legitimate versioned paths since `normalizeDockerPath` still strips real version prefixes; add explicit regression cases for divergence rows 2-3 directly, not only via the corpus)

**Dependencies**: Commit 1 (corpus cases must exist to prove this commit turns them green); conceptually follows Commit 2 for the naming-symmetry rationale, though not a hard technical dependency.

**Validation gate**: `cd agent && go test ./muzzle/... -v` — **all cases now pass**, including the 4 that were red after Commit 1. `cd agent && go test ./...` (full module — confirms `ServeProxy` and `TestFilter_Allow`'s full existing table are unaffected). Full-repo regression: `cd backend && go test ./internal/orthrus/... -v` and `cd agent && go test ./... -v` both green — this is the commit where the repo returns to a fully green test state after Commit 1's intentional red.

---

### Commit 4 — Structural allowlist parity checker (fixes GH #1161 workstream 1)

**Scope**: New `scripts/ci/check_muzzle_allowlist_parity.go` per Section 3.3, diffing all **eight** paired declarations — the seven allowlist/body-size data declarations plus `versionPrefixRe`'s regex source string (extracted via its `regexp.MustCompile(...)` call expression, not a composite literal — see Section 3.3's AST-shape note). Wire into `lefthook.yml` pre-commit stage and a new `quality-checks.yml` job.

**Files**: `scripts/ci/check_muzzle_allowlist_parity.go` (new), `lefthook.yml`, `.github/workflows/quality-checks.yml`

**Dependencies**: Hard dependency on Commit 3 — the checker's pairing-by-name approach relies on agent's declarations having been renamed to match backend's (`allowedDockerPaths`, `allowedDockerPatterns`, `allowedDockerPrefixSuffixPatterns`). Writing the checker against pre-Commit-3 agent code would require it to understand the now-obsolete `/v*/...` duplicate-entry convention, which is exactly the complexity Commit 3 exists to remove. `versionPrefixRe` itself is unrenamed and unchanged by Commit 3 (Section 3.1 keeps its name and regex identical in both files), so its row has no additional dependency beyond Commit 3 landing first for consistency with the rest of the table.

**Validation gate**: `go run scripts/ci/check_muzzle_allowlist_parity.go` exits 0 against the post-Commit-3 state of both files. Negative-path check (manual, documented in PR description, not committed), covering **two** independent cases to prove both AST-extraction shapes actually work: (a) temporarily add an extra entry to one file's `allowedWritePatterns` locally and confirm the checker fails with a specific, correctly-attributed message before reverting; (b) temporarily loosen one file's `versionPrefixRe` (e.g. `^/v\w+`) and confirm the checker independently fails on that row with a message showing both regex source strings, before reverting. (b) is the direct regression check for this Supervisor-requested addition — without it, the checker's coverage of R9 is unverified, not just unimplemented.

---

### Commit 5 — Agent CI tooling parity (fixes GH #1161 workstream 2)

**Scope**: Per Section 3.4/3.5 — move `.golangci-fast.yml` to repo root; extend both `golangci-lint-fast.sh`/`-full.sh` to loop over `backend`/`agent`; split `lefthook.yml`'s `go-vet` into module-scoped `go-vet`/`go-vet-agent`; add `scripts/agent-test-coverage.sh`; add `agent-test-coverage` to `lefthook.yml`'s `testing` pipeline; add `agent-quality` job to `quality-checks.yml`; implement the dangling `scripts/check-module-coverage.sh` (and its corrected `"backend + agent"` echo text); add `agent/cert/cert_test.go` and `agent/protocol/message_test.go`. **Also, per Section 3.4.1/3.4.2 (Supervisor-requested additions)**: extend `scripts/local-patch-report.sh` and `backend/cmd/localpatchreport/main.go` with agent-coverage support, generalize `backend/internal/patchreport`'s path-normalization to be module-parameterized instead of backend-only, and add the `agent-codecov` job (plus its one `codecov.yml` ignore-list entry) so agent coverage reaches both the mandatory local patch-coverage gate and Codecov's dashboard.

**Files**: `.golangci-fast.yml` (moved from `backend/.golangci-fast.yml`), `scripts/pre-commit-hooks/golangci-lint-fast.sh`, `scripts/pre-commit-hooks/golangci-lint-full.sh`, `lefthook.yml`, `scripts/agent-test-coverage.sh` (new), `scripts/check-module-coverage.sh` (new — fixes dangling `Makefile` reference and its stale echo text), `Makefile`, `.github/workflows/quality-checks.yml`, `agent/cert/cert_test.go` (new), `agent/protocol/message_test.go` (new), `scripts/local-patch-report.sh`, `backend/cmd/localpatchreport/main.go`, `backend/cmd/localpatchreport/main_test.go`, `backend/internal/patchreport/patchreport.go`, `backend/internal/patchreport/patchreport_test.go`, `.github/workflows/codecov-upload.yml`, `codecov.yml`

**Dependencies**: Independent of Commits 3/4 in principle, but ordered after them so the coverage/lint gate being stood up here measures the *post-fix* state of `agent/muzzle/muzzle.go`, not a mid-refactor snapshot. Must run after Commit 3 in practice to avoid the new `agent-quality` CI job (and the new `agent-codecov`/patch-coverage-visibility) flagging Commit 3's intermediate diff instead of the finished fix. The `local-patch-report.sh`/`localpatchreport` extension has an internal ordering dependency on `scripts/agent-test-coverage.sh` existing earlier in this same commit (it needs `agent/coverage.txt` to exist as an input) — both land together in Commit 5 rather than being split further, since CLAUDE.md's commit-slicing guidance is about ordering across commits, not fragmenting a single tightly-coupled tooling change into artificially smaller pieces.

**Validation gate**: `make lint-staticcheck-only` (backend, unchanged behavior) and an equivalent manual run of the new agent lint loop; `bash scripts/agent-test-coverage.sh` exits 0 at the calibrated threshold (Section 3.4 — set to the actual post-`cert_test.go`/`message_test.go` aggregate, not a pre-chosen round number); `bash scripts/check-module-coverage.sh` runs both scripts in sequence, exits 0, and its output echoes `"backend + agent"`; `lefthook run pre-commit` clean on a trial commit touching only `agent/**`; `cd backend && go test ./internal/patchreport/... ./cmd/localpatchreport/... -v` green (covers the new three-way `ParseUnifiedDiffChangedLines` arity and the generalized `normalizeGoCoveragePath`/`ParseGoCoverageProfile` signatures); `bash scripts/local-patch-report.sh` run locally against a diff touching `agent/muzzle/muzzle.go` produces a `test-results/local-patch-report.md` with a populated, non-zero "Agent" row (manually confirms the module-prefix-normalization fix actually works end-to-end, not just that the new flag is accepted); `.github/workflows/quality-checks.yml`'s new `agent-quality` job and `.github/workflows/codecov-upload.yml`'s new `agent-codecov` job both reviewed by a human for correct `working-directory`/`go-version-file` wiring (cannot be locally validated without `act` or a real CI run — flagged for Supervisor/DevOps review).

---

### Commit 6 — Documentation

**Scope**: `CHANGELOG.md` entry under `[Unreleased]` (likely `### Fixed` or `### Security` — path-normalization/allowlist-drift is a defense-in-depth hardening fix, not a new user-facing feature); `ARCHITECTURE.md`'s "Continuous Integration (GitHub Actions)" and/or "Pre-Commit Checks" sections updated to note `agent/` now has lint/vet/coverage parity with `backend/` (`CLAUDE.md` requires ARCHITECTURE.md updates for CI/testing architecture changes). No `docs/features.md` change — this PR adds no new user-facing capability, only internal robustness.

**Files**: `CHANGELOG.md`, `ARCHITECTURE.md`

**Dependencies**: Commits 1-5 (documents what actually landed).

**Validation gate**: `markdownlint --fix .` clean (lefthook `lint-full` manual stage); human review that the CHANGELOG entry accurately reflects the merged scope, not the originally-proposed scope.

---

## 6. Definition of Done — gate applicability

| Gate | Applicable? | Why |
|---|---|---|
| Playwright E2E (`npx playwright test --project=firefox`) | **No** | Zero frontend files touched; zero user-facing behavior change. Confirmed by the file list in every commit above — none are under `frontend/`. |
| GORM Security Scan (`./scripts/scan-gorm-security.sh --check`) | **No** | Zero changes to `backend/internal/models/**`, zero GORM queries, zero migrations touched. |
| Local Patch Coverage Preflight (`scripts/local-patch-report.sh`) | **Yes** | Standard gate, applies to all Go changes. As of Commit 5 (Section 3.4.1), it also has visibility into `agent/**` — previously it would have run "successfully" while silently reporting nothing about `agent/muzzle/muzzle.go`, the file this PR's actual security fix lives in. |
| CodeQL Go/JS scan | **Go: Yes. JS: No** (no JS/TS files touched). |
| Trivy container scan | **Marginal** — `agent/Dockerfile` is unchanged by this PR (no new dependencies added to `agent/go.mod`), so no new image-layer surface; still run per standard process. |
| Staticcheck / `make lint-staticcheck-only` | **Yes**, extended in Commit 5 to also cover `agent/`. |
| Backend coverage (85%+) | **Yes**, unchanged script/threshold — Commits 1-4 add tests, don't remove any. |
| Agent coverage | **New in this PR** (Commit 5) — see Section 3.4/3.5 for how the initial threshold is calibrated. |
| Frontend coverage / type-check | **No frontend files touched** — these gates trivially pass (no diff for them to evaluate) but are still run per standard process, not skipped. |
| `go build ./...` (backend and agent) | **Yes.** |

---

## 7. Acceptance Criteria

1. `backend/internal/orthrus/testdata/muzzle_corpus.json` contains all 7 new cases from Section 3.2; `TestMuzzle_SharedCorpus` and `TestFilter_SharedCorpus` both pass against the full corpus.
2. `agent/muzzle/muzzle.go`'s `allowedDockerPaths`, `allowedDockerPatterns`, `allowedDockerPrefixSuffixPatterns` contain no `/v*/...`-prefixed entries (version handling is unified through `normalizeDockerPath`).
3. `scripts/ci/check_muzzle_allowlist_parity.go` diffs all eight paired declarations (the seven data declarations plus `versionPrefixRe`), runs clean against the merged state, and is proven (via the Commit 4 manual negative-path checks, documented in the PR description — one for a data-declaration mismatch, one for a `versionPrefixRe` mismatch) to actually detect an introduced drift in either category.
4. `lefthook run pre-commit` on a change touching only `agent/**` produces the same class of enforcement (vet, lint, and — via the manual `testing` pipeline — coverage) that a `backend/**`-only change already gets.
5. `.github/workflows/quality-checks.yml` runs `agent-quality` unconditionally on every PR, not gated on `agent/**` paths.
6. `scripts/check-module-coverage.sh` exists, exits 0, and its Makefile-invoked output describes its actual scope (`"backend + agent"`), fixing both the pre-existing dangling reference and the stale `"backend + frontend"` text.
7. `go test ./...` (from repo root, exercising both `backend` and `agent` via `go.work`) is fully green.
8. `bash scripts/local-patch-report.sh` succeeds against a diff that includes `agent/**` changes and produces a report with a populated "Agent" coverage row — not just a passing exit code with the agent column silently empty or absent.
9. `.github/workflows/codecov-upload.yml` runs an `agent-codecov` job on every PR (gated the same way as `backend-codecov`/`frontend-codecov`), uploading `agent/coverage.txt` under `flags: agent`; `codecov.yml` excludes `agent/main.go` from that flag's reported coverage, mirroring `backend/cmd/api/**`'s existing treatment.
10. `CHANGELOG.md` and `ARCHITECTURE.md` updated per Commit 6.
11. No behavior change to backend's Docker API allowlist decisions for any input not in the new corpus cases — verified by 100% pre-existing backend test pass rate with zero test modifications required (only additions).
