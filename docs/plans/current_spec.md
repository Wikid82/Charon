# Infra Fix: Pin `gosu`'s `golang.org/x/sys` to v0.46.0 (GO-2026-5024 / CVE-2026-39824)

**Type**: Infra / supply-chain hygiene fix (NOT a feature — reduced scope, see `Scope & Definition of Done` below)
**Branch**: `development` (current working branch; no worktree, no new branch, per `CLAUDE.md`)
**Target commit count**: 1 (single commit; see Commit Slicing Strategy)
**Owner (implementation)**: `devops` agent
**Owner (docs polish, optional)**: `docs-writer` agent
**Review**: `supervisor` agent after implementation

---

## 1. Introduction

### 1.1 Objective

Grype / GitHub security scanning flags `GO-2026-5024` (`CVE-2026-39824`) inside the Charon container image, pointing at `golang.org/x/sys` embedded in `/usr/sbin/gosu`. `gosu` is already built from source in the `gosu-builder` Dockerfile stage specifically to dodge CVEs baked into Debian's precompiled binary (see `Dockerfile:62-65`). The scanner is picking up the version of `golang.org/x/sys` that upstream `tianon/gosu@1.17` pins in its own `go.mod`/`go.sum` (`v0.13.0`), not anything Charon's own Go modules pull in.

The objective of this change is narrow: force the `golang.org/x/sys` dependency resolved during the `gosu-builder` stage's build up to **`v0.46.0`**, so the scanner stops flagging the image, while leaving `gosu`'s own release version (`GOSU_VERSION=1.17`) untouched.

**Why v0.46.0 and not the advisory's minimum fix version (v0.44.0)**: the authoritative GO-2026-5024 advisory (confirmed against `vuln.go.dev/ID/GO-2026-5024.json`) fixes the vulnerability at `v0.44.0`. However, this same Dockerfile already has an established, unrelated precedent for handling `golang.org/x/sys` version pins: the Delve (`dlv`) debug-binary stage (`Dockerfile:186-211`) runs `go get golang.org/x/sys@v0.46.0` in its own temp module (`Dockerfile:199`) for debug builds. Pinning the `gosu-builder` stage to the same `v0.46.0` (rather than the bare-minimum `v0.44.0`) keeps a single consistent "patched x/sys version" convention across the file, comfortably clears the `v0.44.0` fix floor, and avoids having two different pinned versions for the same advisory living a few dozen lines apart for no functional reason.

### 1.2 Non-goals

- This is **not** a `GOSU_VERSION` bump. Upstream tag `1.19` was checked and rejected: its `go.mod` pins `golang.org/x/sys` at the older `v0.1.0`, which is a regression, not a fix.
- This does **not** touch `backend/`, `frontend/`, `internal/models`, migrations, or any application code. It is a Dockerfile change (one new pin in `gosu-builder`, plus two comment corrections in the unrelated Delve stage — see 1.4) plus documentation.
- This does **not** invoke the full application Definition of Done (no Playwright E2E, no 85% coverage gate, no frontend type-check — see `Scope & Definition of Done` below for the reduced gate set that applies instead).

### 1.3 Why this is safe / low-risk

The vulnerable code (`NewNTUnicodeString` string-length overflow, fixed in `x/sys v0.44.0`) lives entirely in `golang.org/x/sys/windows`. `gosu` is a Unix-only tool, and the Charon image only ever cross-compiles Linux targets in this stage (`CGO_ENABLED=0 xx-go build`, `Dockerfile:98`). Go's build-constraint system (`GOOS`-gated files) means the Windows-only source file is never compiled into the binary Charon ships. Real-world exploitability is effectively zero — this is a scanner limitation (dependency-graph/go.sum version scanning does not evaluate `GOOS` build tags), not a live vulnerability. It is still worth fixing because it is cheap, keeps automated scanning quiet, and avoids repeatedly re-triaging the same non-issue.

**Note on a related, distinct prior finding — requires an erratum, not just a mention**: `docs/security/vulnerability-analysis-2026-06-26.md` documents a *separate* instance of this same CVE ID for `backend/go.mod`'s indirect `golang.org/x/sys` dependency (resolved at `v0.46.0` via transitive upgrade, no action needed there). That document's "Decision" and "Reported vs Actual Version" sections went further, though, and concluded the `v0.13.0` Trivy finding was a **"scanner false positive"** caused by a stale SBOM/cache snapshot, recommending an SBOM regeneration. That conclusion is factually wrong, not merely superseded: `v0.13.0` was a real, correctly-scanned version — of `/usr/sbin/gosu` (vendored via upstream `tianon/gosu@1.17`'s own `go.sum`), a binary that investigation never checked because it stopped after confirming `backend/go.mod`. **Required action** (see Section 3.3 for exact placement): append a dated `## Erratum (2026-07-16)` section to the *end* of the existing `docs/security/vulnerability-analysis-2026-06-26.md` file itself — do not just describe the distinction in the new doc — stating plainly that the "scanner false positive" claim was incorrect, explaining the real two-location split (backend/go.mod vs. gosu-builder's vendored go.sum), and linking to the new `2026-07-16` doc.

### 1.4 Related fix folded into this commit: stale CVE-description comments in the Delve stage

While researching this fix, comments were found at `Dockerfile:189` and `Dockerfile:751-754` — both in the **unrelated** Delve (`dlv`) debug-binary stage, not the `gosu-builder` stage — that reference "GO-2026-5024" but describe the vulnerable range inaccurately as "< v0.27.0" / "v0.26.0". The real advisory (confirmed against the official Go vulnerability database) is `golang.org/x/sys/windows`'s `NewNTUnicodeString` string-length overflow, fixed in `v0.44.0+`, with no `v0.27.0`/`v0.26.0` boundary anywhere in the authoritative record. This is stale/inaccurate comment text, not a functional defect: the Delve stage's actual code (`go get golang.org/x/sys@v0.46.0`, originally at `Dockerfile:199` before this commit's insertion shifts line numbers) already resolves well above the real fix version, and production builds ship a stub instead of the real `dlv` binary regardless, so nothing vulnerable ever ships either way.

Since this commit is already touching `golang.org/x/sys`/GO-2026-5024 documentation in this file, correct these two comments in the same commit (small, same-file doc fix — not a separate PR) to accurately state: `golang.org/x/sys/windows`, `CVE-2026-39824` (`GO-2026-5024`), `NewNTUnicodeString` string-length overflow, fixed in `v0.44.0+`. Note: because this fix's own `RUN` insertion (Section 3.1) lands *before* the Delve stage in the file, all line numbers below it shift by the size of the inserted block — verify current line numbers before editing rather than trusting the numbers cited in this plan literally.

---

## 2. Research Findings (confirmed — not re-derived in this plan)

| Item | Finding |
|---|---|
| Flagged CVE | `GO-2026-5024` / `CVE-2026-39824`, low severity |
| Location | `golang.org/x/sys` embedded in `/usr/sbin/gosu` inside the built image |
| Root cause | Upstream `tianon/gosu` tag `1.17` pins `golang.org/x/sys v0.13.0` directly in its own `go.mod`/`go.sum` |
| Fixed upstream at (advisory floor) | `golang.org/x/sys v0.44.0` — confirmed against `vuln.go.dev/ID/GO-2026-5024.json` |
| Version this plan pins to | `golang.org/x/sys v0.46.0` — matches the existing convention already used by the Delve stage (`Dockerfile:199`) for the same advisory; clears the `v0.44.0` floor with margin, see Section 1.1 |
| Unrelated stale comments found (same file) | `Dockerfile:189` and `Dockerfile:751-754` (Delve stage) cite "GO-2026-5024" but describe an inaccurate "< v0.27.0"/"v0.26.0" vulnerable range; corrected as part of this commit (see Section 1.4) — the Delve stage's actual pin (`v0.46.0`) was already fine, only the comment text was wrong |
| Vulnerable function | `NewNTUnicodeString`, in `golang.org/x/sys/windows` only |
| Why not exploitable here | `gosu` is Unix-only; image only builds Linux targets (`CGO_ENABLED=0`, `Dockerfile:98`); Windows-only file never compiled |
| `GOSU_VERSION` bump viable? | **No** — upstream tag `1.19`'s `go.mod` requires an *older* `golang.org/x/sys v0.1.0`; do not bump `GOSU_VERSION` |
| Toolchain compatibility | `ARG GO_VERSION=1.26.5` (`Dockerfile:13`), used as `golang:${GO_VERSION}-alpine` for `gosu-builder` (`Dockerfile:66`), comfortably exceeds the `go 1.25.0` directive in `x/sys v0.44.0`'s own `go.mod` |
| Exact `gosu-builder` stage bounds | `Dockerfile:66` (`FROM ... AS gosu-builder`) through `Dockerfile:99` (`xx-verify /gosu-out/gosu`); `WORKDIR /tmp/gosu` set at `Dockerfile:69` |
| Insertion point confirmed | Between the git-clone retry loop (`Dockerfile:86-92`, closes with `done` at line 92) and the build comment/`RUN --mount` block (`Dockerfile:94-99`) |
| `.hadolint.yaml` | Exists at repo root; ignores `DL3008` (apt version pinning) and `DL3059` (consecutive RUNs) — the new RUN does not trip either rule, no new ignores needed |
| `tools/dockerfile_check.sh` | Exists; checks for Alpine/Debian package-manager mismatches (`apk` vs `apt`) per stage. The new RUN uses `go get`/`go mod tidy`, not `apk`/`apt`, so it will not be flagged |
| Relevant local skills | `.github/skills/security-scan-docker-image.SKILL.md` (builds image + runs Grype/Syft — matches the exact scanner that raised this finding); `.github/skills/security-scan-go-vuln.SKILL.md` (runs `govulncheck` against `backend/go.mod` only — does **not** cover the vendored `gosu` module, noted explicitly so it isn't mistaken for equivalent coverage) |
| Docs precedent | `docs/security/` uses dated filenames (`vulnerability-analysis-YYYY-MM-DD.md`) with YAML front matter (`post_title`, `categories`, `tags`, `summary`, `post_date`) — see `docs/security/vulnerability-analysis-2026-06-26.md` as the template |
| `SECURITY.md` precedent | `### [RESOLVED] <id> · <title>` heading pattern with `What/Who/Where/When/How/Resolution` subsections (see the CrowdSec entry, `SECURITY.md:32-55`) |
| `CHANGELOG.md` precedent | `[Unreleased]` section already has a `### Security` subsection (`CHANGELOG.md:70`) with an existing one-line entry for the *other* CVE-2026-39824 finding — the new entry is added alongside it, clearly distinguished |
| `.gitignore` / `.dockerignore` / `codecov.yml` | Reviewed — **no changes needed**. This change adds no new file types, build artifacts, or source directories that need exclusion; `codecov.yml` gates test coverage, which is not applicable to a Dockerfile-only change |

---

## 3. Technical Specification

### 3.1 Dockerfile change

**File**: `Dockerfile`
**Stage**: `gosu-builder`
**Insertion point**: immediately after line 92 (the `done` closing the git-clone retry loop) and before line 94 (`# Build gosu for target architecture with patched Go stdlib`)

New content to insert (exact commands; version pinned, not `@latest`, for reproducibility):

```dockerfile
# Pin golang.org/x/sys to a patched version for scanner hygiene (GO-2026-5024 / CVE-2026-39824).
# Upstream tianon/gosu@1.17's own go.sum resolves golang.org/x/sys to v0.13.0, which Grype/GitHub
# code scanning flags. The vulnerable code (NewNTUnicodeString overflow) lives only in
# golang.org/x/sys/windows; gosu is Unix-only and this stage only cross-compiles Linux targets
# (CGO_ENABLED=0 xx-go build below), so the flagged code path is never compiled into this binary.
# Fixed regardless, since it's cheap and keeps the scanner quiet. Pinned to v0.46.0 (above the
# advisory's v0.44.0 fix floor) to match the same x/sys version already used by the Delve debug
# stage below for this identical advisory. Do NOT bump GOSU_VERSION instead:
# upstream tag 1.19's go.mod actually requires an OLDER golang.org/x/sys v0.1.0.
RUN go get golang.org/x/sys@v0.46.0 && \
    go mod tidy && \
    go mod verify
```

**Required: wrap the above in this Dockerfile's existing retry pattern.** Every other network-touching `RUN`/loop in this Dockerfile's neighborhood already retries on transient failure — the git-clone retry loop immediately above this insertion point (`Dockerfile:86-92`), the `go mod download` retry (`Dockerfile:216-222` pre-edit numbering), and the `xcaddy` install retry (`Dockerfile:291-297` pre-edit numbering). `go get`/`go mod tidy` hit the network (module proxy) exactly like those do, so this step should not be the one exception. Match whichever retry idiom (attempt counter + backoff loop, or shell `until`/`for` construct) this Dockerfile already uses at those call sites — implementers should read the actual surrounding retry blocks and mirror the same style/variable-naming convention rather than inventing a new one, so the Dockerfile stays internally consistent.

Notes:
- Runs inside `WORKDIR /tmp/gosu` (already set at `Dockerfile:69`, still in effect — no `WORKDIR` change needed).
- Must run **after** the `git clone` (needs `go.mod`/`go.sum` from the cloned `gosu` source to exist) and **before** `xx-go build` (so the build picks up the tidied `go.sum`).
- `go mod verify` is included as a lightweight integrity check on the resulting module cache; it exits non-zero if checksums don't match `go.sum`, which will fail the Docker build loudly if something is wrong, rather than silently shipping a bad module.
- Pin is exact (`@v0.46.0`), not `@latest` — reproducible builds, consistent with how every other dependency in this Dockerfile (`GOSU_VERSION`, `CROWDSEC_VERSION`, `XNET_VERSION`, `XCRYPTO_VERSION`, etc.) is pinned to an explicit version rather than a floating tag, and consistent with the existing Delve-stage pin at the same version.
- No `# renovate:` comment is added for this line — it is a one-time remediation pin tied to a specific CVE fix version, not an upstream release Renovate should track independently (unlike `GOSU_VERSION`, `GO_VERSION`, etc.). If `gosu`'s own `go.mod` eventually adopts `x/sys >= v0.46.0` upstream, this override becomes a no-op and can be removed in a future cleanup.
- Line numbers throughout this plan (e.g. "`Dockerfile:199`" for the Delve stage's pin, "`Dockerfile:751-754`" for its comment block) are pre-edit references. Once this stage's new RUN block (and its retry wrapper) is inserted, every line number below the insertion point shifts down by the size of the inserted block — implementers must re-locate targets by content/context, not by trusting these literal numbers.

### 3.1a Comment corrections in the (unrelated) Delve stage

**File**: `Dockerfile`
**Lines**: `189` and `751-754`

Both currently describe the GO-2026-5024 vulnerable range inaccurately (e.g. "< v0.27.0", "v0.26.0"). Replace with accurate language reflecting the real advisory:

- `Dockerfile:189` — change to state the vulnerability is in `golang.org/x/sys/windows`'s `NewNTUnicodeString` (string-length overflow), fixed in `v0.44.0+`, and that the Delve stage already pins `v0.46.0` (`Dockerfile:199`), above that floor.
- `Dockerfile:751-754` — same correction applied to the production-stub comment block; state the binary this stage would otherwise ship is compiled against `golang.org/x/sys v0.46.0` (patched, above the `v0.44.0` advisory floor), rather than referencing the incorrect "v0.26.0 vulnerable" framing.

This is a same-file documentation correction discovered incidentally while implementing the `gosu-builder` pin above — it is not a functional change (the Delve stage's actual pinned version, `v0.46.0`, was already correct), and is folded into this same single commit rather than filed separately.

### 3.2 No database, API, or model changes

Not applicable — this is a build-time Dockerfile change only. No `internal/models`, no `routes.go`, no frontend API clients are touched.

### 3.3 Documentation changes

**File 1 (new)**: `docs/security/vulnerability-analysis-2026-07-16.md`

Follow the existing template (`docs/security/vulnerability-analysis-2026-06-26.md`) with YAML front matter:

```yaml
---
post_title: "GO-2026-5024 / CVE-2026-39824 Remediation: golang.org/x/sys in gosu-builder Stage"
categories: ["security", "dependency", "docker"]
tags: ["go-2026-5024", "cve-2026-39824", "golang.org/x/sys", "gosu", "docker-build", "scanner-hygiene"]
summary: "Pinned golang.org/x/sys to v0.46.0 during the gosu-builder Dockerfile stage to resolve a Grype-flagged low-severity CVE in gosu's vendored dependency, matching the same version already used by the Delve stage for this advisory. Windows-only vulnerable code path is never compiled for this Linux-only binary; fix applied for scanner hygiene."
post_date: "2026-07-16"
---
```

Body must cover (per the `supply-chain-remediation` skill's documentation phase pattern):
- CVE id (`GO-2026-5024` / `CVE-2026-39824`), affected package (`golang.org/x/sys`), affected version (`v0.13.0`, resolved via upstream `tianon/gosu@1.17`'s `go.sum`), advisory fix floor (`v0.44.0`), version pinned to (`v0.46.0`, chosen to match the existing Delve-stage convention — see below), severity (low).
- Why real-world exploitability is near-zero: vulnerable function `NewNTUnicodeString` lives in `golang.org/x/sys/windows`; `gosu` is Unix-only; this stage builds only Linux targets (`CGO_ENABLED=0`); Go build constraints mean the file is never compiled.
- Explicit note distinguishing this from the `2026-06-26` doc's finding (same CVE ID, different location — `backend/go.mod` vs. vendored `gosu` build stage — and different resolution mechanism — transitive upgrade vs. explicit pin in the Dockerfile).
- A short note that this same advisory ID also appears in this Dockerfile's Delve (`dlv`) debug stage (`Dockerfile:186-211`), which already pins `golang.org/x/sys@v0.46.0` (`Dockerfile:199`) correctly, but had stale/inaccurate comment text (fixed as part of this same commit — see Dockerfile comment corrections below) describing the vulnerable range as "< v0.27.0"/"v0.26.0" instead of the real `NewNTUnicodeString`/`v0.44.0` detail. Mention this so a future reader who greps the Dockerfile for "GO-2026-5024" understands why two stages reference it and why the versions differ slightly (v0.46.0 pin chosen deliberately to match, not coincidentally).
- The fix applied: `go get golang.org/x/sys@v0.46.0 && go mod tidy && go mod verify` added to the `gosu-builder` stage, with the exact Dockerfile location, plus the two comment corrections at `Dockerfile:189` and `751-754`.
- Why `GOSU_VERSION` was not bumped (upstream `1.19` regresses to `x/sys v0.1.0`).
- Validation performed (see Section 4 below) and its results.

**File 1a (edit to an existing file — not merely a mention in the new doc)**: `docs/security/vulnerability-analysis-2026-06-26.md`

Append a new `## Erratum (2026-07-16)` section to the **end** of this existing file (do not rewrite or remove its original content — this is a dated correction appended after the fact, preserving the historical record of what was originally concluded). The erratum must state plainly that the original "scanner false positive"/"stale SBOM cache" conclusion (in that doc's "Reported vs Actual Version" and "Decision" sections) was incorrect — `v0.13.0` was a real, correctly-scanned version of a *different* binary (`/usr/sbin/gosu`, vendored via upstream `tianon/gosu@1.17`'s own `go.sum`) that the original investigation never checked because it stopped after confirming `backend/go.mod` resolved to `v0.46.0`. Explain the corrected two-location finding (backend/go.mod, already fine; gosu-builder's vendored go.sum, fixed by this commit) and link to the new `vulnerability-analysis-2026-07-16.md` doc.

**File 2**: `SECURITY.md`

Add a new entry under **`## Patched Vulnerabilities`** (not `## Known Vulnerabilities`) — this file has two conventions for resolved findings, and this fix matches the majority pattern (5 entries) and the specific precedent of `### ✅ [LOW] CVE-2026-26958 · edwards25519 MultiScalarMult Invalid Results` (a LOW-severity, version-pin-only fix with no residual risk, the closest analog to this one) rather than the minority `### [RESOLVED]`/`## Known Vulnerabilities` style (only 2 entries: CrowdSec, CVE-2026-45135). Use heading `### ✅ [LOW] GO-2026-5024 / CVE-2026-39824 · golang.org/x/sys in gosu Build Stage`, with the `| Field | Value |` table using a **`**Patched**: <date>`** row (not `**Status**`), matching the edwards25519 entry's exact structure (`What`/`Who`/`Where`/`When`/`How`/`Resolution`, `When` using `Discovered`/`Patched`/`Time to patch` rows). Body should mirror the analysis doc at a summary level, link to `docs/security/vulnerability-analysis-2026-07-16.md` for full detail, and explicitly note the erratum added to the 2026-06-26 doc (see File 1 above) so a reader following that link isn't confused by the now-corrected "false positive" claim there.

Also bump the `Last reviewed: <date>` line near the top of `SECURITY.md` (under `## Known Vulnerabilities`) to `2026-07-16` as part of this same commit, since the file is already being edited.

**File 3**: `CHANGELOG.md`

Add one line under the existing `### Security` subsection of `## [Unreleased]` (`CHANGELOG.md:70`), alongside — not replacing — the existing `CVE-2026-39824` line for the unrelated `backend/go.mod` finding. Suggested entry, matching the terse one-line style already used there:

```
- chore(security): pin gosu-builder's golang.org/x/sys to v0.46.0 (GO-2026-5024 / CVE-2026-39824) — upstream tianon/gosu@1.17 vendors v0.13.0; vulnerable code is Windows-only and never compiled for this Linux-only binary; v0.46.0 matches the existing Delve-stage pin for the same advisory
```

**Files NOT changed (confirmed, stated explicitly per plan requirements)**:
- `.gitignore` — no new file patterns introduced.
- `.dockerignore` — no new build context files introduced.
- `codecov.yml` — no test/coverage surface affected; this is a Dockerfile-only change.

---

## 4. Scope & Definition of Done (reduced — infra fix, not a feature)

This is explicitly **not** subject to the full application Definition of Done in `CLAUDE.md` (no Playwright E2E suite, no 85% backend/frontend coverage gate, no frontend `type-check`, no GORM security scan — none of those apply to a Dockerfile-only change with zero app-code touch). Instead, the validation gates for this change are:

| # | Gate | Command | Pass condition |
|---|---|---|---|
| 1 | Isolated stage build | `docker build --target gosu-builder -t charon-gosu-builder-check .` (run from repo root) | Build completes with exit code 0; the new `RUN go get ... && go mod tidy && go mod verify` step succeeds (non-zero exit on checksum mismatch would fail the build here) |
| 2 | Embedded module version check | `docker run --rm --entrypoint sh charon-gosu-builder-check -c "go version -m /gosu-out/gosu | grep golang.org/x/sys"` | Output shows `golang.org/x/sys` at `v0.46.0` (or, at minimum, `>= v0.44.0`) |
| 3 | Dockerfile lint | `hadolint Dockerfile` (if `hadolint` binary available locally; `.hadolint.yaml` config already present at repo root) **and** `bash tools/dockerfile_check.sh Dockerfile` | Both exit 0; no new hadolint findings introduced by the added `RUN` line (it is plain `go` tooling, not `apk`/`apt`, so `tools/dockerfile_check.sh`'s package-manager-mismatch check is unaffected) |
| 4 | Local re-scan | `.github/skills/scripts/skill-runner.sh security-scan-docker-image` (builds full image + runs Grype/Syft — the scanner that originally raised `GO-2026-5024`) | No `golang.org/x/sys` finding for the `gosu` binary in the Grype output. If Docker/Grype/Syft are unavailable in the local environment, note this in the PR/commit and rely on CI's next scheduled scan to confirm; do not block the commit on unavailable local tooling |

`security-scan-go-vuln` (`govulncheck` against `backend/go.mod`) is **not** a valid substitute for gate 4 — it does not scan the vendored `gosu` module built in a separate Docker stage — and should not be cited as evidence this fix worked.

---

## 5. Commit Slicing Strategy

**Decision**: Single commit, on the current `development` branch, inside one PR (per `CLAUDE.md`: "One Feature = One PR" and no worktrees). This is a small infra fix — splitting it into multiple commits would add review overhead without improving reviewability.

### Commit 1 (only commit): `fix(deps): pin gosu's golang.org/x/sys to v0.46.0 (GO-2026-5024)`

**Scope**: Dockerfile dependency pin + two same-file comment corrections + accompanying security documentation.

**Files touched**:
- `Dockerfile` — insert the retry-wrapped `go get golang.org/x/sys@v0.46.0 && go mod tidy && go mod verify` block (with comment) into the `gosu-builder` stage (see Section 3.1); correct the stale CVE-range comments in the unrelated Delve stage (see Section 3.1a; locate by content, not literal line numbers — they shift once the RUN block above is inserted).
- `docs/security/vulnerability-analysis-2026-07-16.md` — new file (see Section 3.3, File 1).
- `docs/security/vulnerability-analysis-2026-06-26.md` — append `## Erratum (2026-07-16)` section (see Section 3.3, File 1a). Do not remove or rewrite existing content.
- `SECURITY.md` — new `### ✅ [LOW]` entry under `## Patched Vulnerabilities`, plus bump `Last reviewed:` to `2026-07-16` (see Section 3.3, File 2).
- `CHANGELOG.md` — one new line under `## [Unreleased]` → `### Security` (see Section 3.3, File 3).

**Dependencies**: None — this is a self-contained, atomic change. No prior commit needed.

**Validation gate for this commit** (must all pass before considering the commit done):
1. Gate 1 and Gate 2 from Section 4 (stage build + embedded version check) — functional proof the fix works.
2. Gate 3 from Section 4 (hadolint + `tools/dockerfile_check.sh`) — style/consistency check on the Dockerfile edit.
3. Gate 4 from Section 4 (local Grype re-scan via `security-scan-docker-image` skill, or explicit note deferring to CI if local tooling unavailable).
4. `lefthook run pre-commit` — standard repo-wide pre-commit hooks still apply (markdown lint on the new/edited `.md` files, any Dockerfile-aware hooks, secret scanning, etc.) even though this is an infra-only change; this is not part of the reduced application DoD, it's baseline repo hygiene for every commit.
5. Manual read-through confirming `SECURITY.md` and `CHANGELOG.md` entries clearly distinguish this fix from the pre-existing, unrelated `CVE-2026-39824` / `backend/go.mod` entry already present in both files, so a future reader doesn't conflate the two.

**Commit message** (conventional commits, per `CLAUDE.md`):
```
fix(deps): pin gosu's golang.org/x/sys to v0.46.0 (GO-2026-5024)
```

### Rollback / contingency notes (for the PR as a whole)

- **Rollback**: This is a single, additive `RUN` step in one Dockerfile stage. Reverting is a one-line-block removal (`git revert <sha>`); no migration, no data, no running-system state is affected.
- **Contingency — Gate 1/2 failure (build breaks or the resolved version doesn't clear `v0.44.0`)**: The primary target for this fix is **`v0.46.0`** — that is the version to pin to, matching the existing Delve-stage convention (Section 1.1). `v0.44.0` is cited here only as the advisory's *fix floor*, i.e. the minimum acceptable fallback if `v0.46.0` itself turns out to be unreachable. If `go get golang.org/x/sys@v0.46.0` surfaces a hard transitive dependency conflict with `gosu`'s other direct dependencies at tag `1.17` (unlikely — `gosu` has a minimal dependency graph, but `go mod tidy` will reveal it if so), fall back to the next available patched version that still satisfies `>= v0.44.0` (the floor), and update the pin plus the security doc accordingly — still do not bump `GOSU_VERSION`.
- **Contingency — Gate 4 unavailable locally (no Docker/Grype/Syft)**: Do not block the commit. Note this explicitly in the analysis doc and PR description; CI's regularly scheduled Grype/Trivy scan will confirm on the next run. This is called out in Section 4, gate 4, as an accepted condition, not a blocker.
- **Contingency — hadolint not installed locally**: `tools/dockerfile_check.sh` still runs (it's a plain bash script with no external binary dependency) and covers the specific class of error (Alpine/Debian package-manager mismatch) most relevant to Dockerfile stage edits. Note in the commit if `hadolint` itself could not be run locally; CI's Dockerfile lint step will still catch anything `tools/dockerfile_check.sh` doesn't.

---

## 6. Acceptance Criteria

1. `Dockerfile`'s `gosu-builder` stage contains the new pinned `go get golang.org/x/sys@v0.46.0 && go mod tidy && go mod verify` step, correctly placed between the git-clone retry block and the `xx-go build` step, with an inline comment explaining the CVE, the Windows-only/never-compiled rationale, why `v0.46.0` was chosen (matches the Delve stage's existing pin for this advisory), and why `GOSU_VERSION` was not bumped instead.
2. `docker build --target gosu-builder .` succeeds, and the resulting `gosu` binary's embedded module info (`go version -m`) reports `golang.org/x/sys` at `v0.46.0` (or, at minimum, `>= v0.44.0`).
3. `Dockerfile:189` and `751-754` (Delve stage) are corrected to accurately describe GO-2026-5024/CVE-2026-39824 (`golang.org/x/sys/windows`, `NewNTUnicodeString`, fixed `v0.44.0+`) instead of the stale "< v0.27.0"/"v0.26.0" language.
4. `hadolint Dockerfile` (if available) and `tools/dockerfile_check.sh Dockerfile` both pass with no new findings attributable to this change.
5. A local Grype/Syft re-scan (via `security-scan-docker-image` skill) no longer reports the `golang.org/x/sys` finding against `gosu`, or — if local scanning tooling is unavailable — this is explicitly noted and deferred to CI's next scan.
6. `docs/security/vulnerability-analysis-2026-07-16.md` exists, follows the repo's existing front-matter/template convention, clearly distinguishes this finding from the unrelated pre-existing `CVE-2026-39824` entry for `backend/go.mod`, and notes the Delve-stage comment correction.
7. `docs/security/vulnerability-analysis-2026-06-26.md` has a new `## Erratum (2026-07-16)` section appended, correcting its original "scanner false positive" conclusion and linking to the new doc.
8. `SECURITY.md` has a new `### ✅ [LOW]` entry under `## Patched Vulnerabilities` for this finding, and its `Last reviewed:` line is bumped to `2026-07-16`.
9. `CHANGELOG.md`'s `[Unreleased]` → `### Security` section has a new one-line entry for this fix, clearly worded to avoid conflation with the existing adjacent `CVE-2026-39824` line.
10. `.gitignore`, `.dockerignore`, and `codecov.yml` are confirmed unchanged (explicitly verified, not silently skipped).
11. All changes land in a single commit on `development`, with the exact conventional-commit message `fix(deps): pin gosu's golang.org/x/sys to v0.46.0 (GO-2026-5024)`.
12. `lefthook run pre-commit` passes on the commit.

---

## 7. Handoff

Implementation is assigned to the **`devops`** agent — this is Dockerfile/CI/CD/build-infrastructure work, not backend Go application code or frontend TypeScript code, and falls squarely within that agent's remit (Docker builds, dependency pinning, build-stage debugging). The **`docs-writer`** agent may be pulled in afterward only if the security-doc/SECURITY.md/CHANGELOG prose needs a polish pass for tone/clarity — the technical content and required sections are fully specified above so this should be optional.

Once implemented, hand off to the **`supervisor`** agent for review against this spec before merge, per the standard workflow.
