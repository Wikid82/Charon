# Technical Spec — Prebuilt Caddy + CrowdSec Toolchain Image (CI Docker-build timeout fix)

**Status:** Revision 2 — for supervisor re-review
**Branch:** `feat/prebuilt-toolchain-image`
**Delivery model:** One feature = one PR, sliced into ordered logical commits (see [Commit Slicing Strategy](#12-commit-slicing-strategy)).
**Author:** Planning (Principal Architect)
**Date:** 2026-09-07
**Supersedes on merge:** the previous `current_spec.md` (Uptime Monitoring at Scale — already delivered).

### Revision history

| Rev | Date | Change |
|---|---|---|
| 1 | 2026-09-07 | Initial draft. |
| **2** | **2026-09-07** | **Supervisor "APPROVE WITH CHANGES (major)" — resolved 7 blocking items:** B1 added §2.6 Alternatives Considered (decision record); B2 corrected the true current recurrence baseline (daily via nightly, not weekly) and pulled a **daily** `schedule` toolchain rebuild into committed scope (§3.4.1, §3.8.1–3.8.2; workflow lands in Commit 1, security-rebuild reroute in Commit 5); B3 rewrote the overstated "fresh `go mod tidy` MVS" claim in §3.8/R3 to state accurately what the forced rebuild catches (base-image + pin-bump drift only); B4 pin the two unpinned xcaddy plugins + feed them to the key (§2.2, §3.2.1, §3.4.2, Commit 1); B5 reworked the Commit Slicing Strategy so the CVE recurrence guard is never inert — the `--no-cache-filter` is retargeted to `caddy-inline`/`crowdsec-inline` inside Commit 1 and every commit gate proves the guard is live; B6 reconciled §3.9 timeouts with §3.7 fork path (fork-reachable CVE-gate jobs stay at 20 min); B7 made `verify-toolchain-pin.sh` failure-closed on same-repo PRs. Folded non-blocking N1–N11. |

---

## 1. Introduction

### 1.1 Overview

Every CI workflow that builds the Charon container image recompiles a **custom Caddy v2 binary** (via `xcaddy`, with in-place source patching of transitive dependencies) and a **custom CrowdSec agent** (`crowdsec` + `cscli`) **from source, from scratch, on every run**. The two Dockerfile stages that do this — `caddy-builder` (`Dockerfile:302`) and `crowdsec-builder` (`Dockerfile:577`) — are explicitly excluded from all layer caching by `--no-cache-filter` / `no-cache-filters` in six workflows plus the shared composite action.

Measured on the PR #1298 amd64 run:

| Stage | Cold build time |
|---|---|
| `caddy-builder` (xcaddy build + patch + rebuild) | **748 s (12.5 min)** |
| `crowdsec-builder` (clone + patch + 2× `xx-go build`) | **~330 s** combined |
| GHA cache export (`type=gha,mode=max`) | ~90 s |

`build-amd64` has `timeout-minutes: 15` with a nested `nick-fields/retry` `timeout_minutes: 15` (`docker-build.yml:403`, `:441`). The ~14-minute cold compile plus cache export blows the 15-minute budget; the integration jobs (`timeout-minutes: 20`) run out of budget once test work is stacked on top of the same cold compile. **PR #1298's four "failed" checks (`build-amd64`, `CrowdSec Bouncer Integration`, `Trivy Binary Scan`, `Cerberus Security Stack Integration`) were all CI job-timeout cancellations on the image build — not test or assertion failures.**

The `--no-cache-filter` guards are deliberate (commits `5c046238`, `8cbc71f2`): the two builder stages patch pinned transitive dependencies **inside** the stage (`go get pkg@fixed`), and a build-arg bump does not reliably invalidate the GHA layer-cache key of a stage that only *consumes* that arg, so a restored stale layer keeps shipping a superseded, still-vulnerable dependency (this is exactly what produced the CVE-2026-45135 and 2026-09-04 grpc-go v1.83.0 recurrences). Removing the guards without another mechanism would reintroduce that class of silent regression.

### 1.2 Objectives & Goals (ranked)

1. **CI image builds stop timing out.** The `xcaddy` / CrowdSec compile must **not** run on the hot path of an ordinary app image build. Target: warm `build-amd64` completes in **< 8 min**; integration jobs **< 12 min**.
2. **The CVE-2026-84304-class recurrence guarantee is preserved or strengthened.** "Upstream ships a security fix, no repo pin changes" must still be caught on a defined cadence with an explicit alert path.
3. **Multi-arch is preserved.** `linux/amd64` and `linux/arm64` images keep getting a correctly cross-compiled Caddy/CrowdSec binary.
4. **A bumped pin can never silently ship an old toolchain.** A stale digest in the Dockerfile against a newer pin must fail a PR fast.
5. **Fork PRs, first-run bootstrap, and local `docker build` still work** without `packages: write` and without a published toolchain image.
6. **One source of truth for the build logic.** The `xcaddy` / CrowdSec build recipe must not be duplicated between the app Dockerfile and a separate toolchain Dockerfile.

### 1.3 Non-goals

- Changing *which* Caddy plugins or CrowdSec version are shipped, or any dependency pin values.
- Changing the runtime image contents, entrypoint, ports, or `internal/caddy` / `internal/cerberus` behavior.
- Reworking the `arm64` QEMU split in `docker-build.yml` (already done in a prior spec).
- Moving off GHCR or introducing a second registry.

### 1.4 EARS-style requirements

| # | Requirement (EARS) |
|---|---|
| R1 | **When** an app image build runs in CI or locally with the default build-args, the system **shall** obtain the Caddy and CrowdSec binaries by `COPY --from` a digest-pinned prebuilt toolchain image, **without** invoking `xcaddy` or compiling CrowdSec. |
| R2 | **When** any security-relevant toolchain input changes on a PR (the two builder-stage bodies, their consumed version ARGs incl. the two now-pinned xcaddy plugins, `xx` pin, the digest-pinned `golang`/`alpine` builder bases, or `.trivyignore`), the toolchain-image workflow **shall** run and the freshness-guard check **shall** fail until the Dockerfile's pinned toolchain digest matches the newly published image for those inputs. |
| R3 | **While** no repo pin has changed, the scheduled **daily** toolchain rebuild **shall** rebuild the toolchain image with `--no-cache --pull` and scan it with Trivy; it **catches base-image drift** (new `golang`/`alpine`/plugin-base CVEs picked up via `--pull` and the digest re-resolve) **and pin-bump drift**, and — if a new digest or a new CRITICAL/HIGH finding results — **shall** open a bot PR bumping the pinned digest and alert via a GitHub issue on failure. It does **not** independently discover upstream security fixes to *unpinned transitive* Go dependencies (see §3.8 — that gap exists identically today and is closed only by an explicit pin bump). |
| R4 | **Where** the builder lacks `packages: write` or the toolchain image is unavailable (fork PR, first bootstrap, offline local build), the system **shall** fall back to compiling the `caddy-inline` / `crowdsec-inline` stages from source, producing an equivalent binary. |
| R5 | **When** the toolchain image is built, it **shall** be published as a multi-arch manifest list covering `linux/amd64` and `linux/arm64`, each entry carrying the correctly cross-compiled binary. |
| R6 | **When** `--no-cache-filter caddy-builder` / `crowdsec-builder` (and the `no-cache-filters` input) are removed from all six workflows and the composite action, normal `type=gha` layer caching **shall** cover every remaining stage. |

---

## 2. Research Findings

### 2.1 Current build graph (verified)

```
Dockerfile stages (944 lines total):

  xx  (tonistiigi/xx:1.9.0, Dockerfile:73)  ── cross-compile helper
   │
   ├─► gosu-builder          (:80,  COPY --from=xx)
   ├─► frontend-builder      (:134, node:24)
   ├─► backend-builder       (:178, COPY --from=xx)
   │
   ├─► caddy-builder         (:302)  FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine
   │        NO `COPY --from=xx`. Pure Go cross-compile:
   │        go install xcaddy → `xcaddy build` (Stage 1, generates go.mod)
   │        → ~20× `go get pkg@fixed` security patches (Stage 2)
   │        → module-cache source patches (celmatcher.go, bouncer)
   │        → GOOS=$TARGETOS GOARCH=$TARGETARCH go build -o /usr/bin/caddy   ← 748 s
   │        → embeds-version assertions (cel-go v0.29.x, grpc v1.83.1)
   │
   ├─► crowdsec-builder      (:577)  FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine
   │        COPY --from=xx / /  (:578).  CGO cross-compile:
   │        xx-apk add gcc musl-dev musl → git clone crowdsec vX.Y
   │        → ~20× `go get pkg@fixed` → sed patch debugger.go
   │        → CGO_ENABLED=1 xx-go build crowdsec + cscli  ← ~330 s
   │        → xx-verify
   │
   ├─► crowdsec-fallback     (:713)  FROM ${ALPINE_IMAGE}  ── DEAD CODE (see note below)
   │
   └─► final runtime         (:751)  FROM ${ALPINE_IMAGE}
            COPY --from=caddy-builder    /usr/bin/caddy        /usr/bin/caddy         (:807)
            COPY --from=crowdsec-builder /crowdsec-out/crowdsec /usr/local/bin/crowdsec (:814)
            COPY --from=crowdsec-builder /crowdsec-out/cscli    /usr/local/bin/cscli    (:815)
            COPY --from=crowdsec-builder /crowdsec-out/config   /etc/crowdsec.dist      (:817)
```

Key finding: **`caddy-builder` does not use `xx`** — it is a plain `$BUILDPLATFORM` golang image doing `GOOS/GOARCH` cross-compilation with `CGO` disabled. `crowdsec-builder` **does** use `xx` because CrowdSec needs `CGO_ENABLED=1` (sqlite). **Both stages are `FROM --platform=$BUILDPLATFORM`**, so on `docker-build.yml`'s arm64 leg the *builder* stages have always run natively on the amd64 host and cross-compiled — **QEMU has only ever emulated the final arm64 stage's `RUN` lines**, never the Caddy/CrowdSec compile. This is what makes a native-amd64, no-QEMU multi-arch toolchain build possible (see §3.5).

**Correction (was wrong in Rev 1): `crowdsec-fallback` (`:713`) is dead code.** Verified: no `COPY --from=crowdsec-fallback`, no `FROM crowdsec-fallback`, and no `--target crowdsec-fallback` anywhere in the repo. It is never built and never consumed. The final stage copies unconditionally from `crowdsec-builder` (`:814-817`). Rev 1's §3.2.3 claim that "`crowdsec-fallback` is selected by the existing arch logic" was incorrect. **Action:** Commit 1 deletes the `crowdsec-fallback` stage (`:713-748`). `CROWDSEC_RELEASE_SHA256` (`:22`, re-declared at `:586`) is *only* used by the tarball `sha256sum -c` inside that stage — after deletion the global `ARG` and the `:586` re-declaration are dead too and are removed in the same commit. `crowdsec-inline` clones from git (`git clone --branch "v${CROWDSEC_VERSION}"`), so it is unaffected. **Verify at implementation time** whether `CROWDSEC_RELEASE_SHA256` has its own updater workflow (grep `.github/workflows/` for `CROWDSEC_RELEASE_SHA256`); if so, delete it in the same commit. (Per CLAUDE.md "delete dead code immediately". If the reviewer prefers to keep `crowdsec-fallback` as a deliberate escape hatch, the fallback position is: leave it untouched and simply exclude it from the toolchain key — it does not affect the shipped binary. Planning's recommendation is deletion.)

### 2.2 Version ARGs consumed by the builder stages (verified line numbers)

| ARG | Global default (line) | Re-declared in |
|---|---|---|
| `GO_VERSION` | `1.27.1` (`:13`) | base of both stages (`FROM golang:${GO_VERSION}-alpine` — **moving tag, see N4 below**) |
| `ALPINE_IMAGE` | `alpine:3.24.1@sha256:28bd…` (`:16`) | toolchain-runtime base (already digest-pinned) |
| `CROWDSEC_VERSION` | `1.8.1` (`:20`) | caddy `:318`, crowdsec `:585`, fallback `:720` |
| `CROWDSEC_RELEASE_SHA256` | `deae1f43…` (`:22`) | crowdsec `:586`, fallback `:721` |
| `EXPR_LANG_VERSION` | `1.17.8` (`:26`) | caddy `:313`, crowdsec `:587` |
| `XNET_VERSION` | `0.58.0` (`:28`) | caddy `:314`, crowdsec `:588` |
| `XCRYPTO_VERSION` | `0.56.0` (`:33`) | caddy `:315`, crowdsec `:589` |
| `KLAUSPOST_COMPRESS_VERSION` | `1.20.0` (`:38`) | caddy `:316`, crowdsec `:590` |
| `GRPC_VERSION` | `1.83.1` (`:44`) | caddy `:317`, crowdsec `:591` |
| `CADDY_VERSION` | `2.11.4` (`:56`) | caddy `:305` |
| `CADDY_CANDIDATE_VERSION` | `2.11.4` (`:58`) | caddy `:306` |
| `CADDY_USE_CANDIDATE` | `0` (`:59`) | caddy `:307` |
| `CADDY_PATCH_SCENARIO` | `B` (`:60`) | caddy `:308` |
| `CADDY_SECURITY_VERSION` | `1.1.64` (`:62`) | caddy `:309` |
| `CORAZA_CADDY_VERSION` | `2.6.0` (`:64`) | caddy `:310` |
| `XCADDY_VERSION` | `0.4.7` (declared inside stage, `:~311`) | caddy only |
| `xx` image | `tonistiigi/xx:1.9.0@sha256:c64defb9…` (`:73`) | crowdsec `:578` |
| **`CADDY_GEOIP2_VERSION`** | **NEW — see B4** | caddy `:391` — currently `--with github.com/zhangjiayin/caddy-geoip2` with **no `@version`** |
| **`CADDY_RATELIMIT_VERSION`** | **NEW — see B4** | caddy `:392` — currently `--with github.com/mholt/caddy-ratelimit` with **no `@version`** |

**B4 — two xcaddy plugins are unpinned (`Dockerfile:391-392`).** `--with github.com/zhangjiayin/caddy-geoip2` and `--with github.com/mholt/caddy-ratelimit` carry no `@version`, so `xcaddy` resolves "latest" at build time. The literal Dockerfile text never changes when those projects tag a release, so `toolchain-key.sh` would not notice, and a stale toolchain image would be reused when an upstream plugin fix actually warrants a rebuild. **Fix (Commit 1, preferred):** add `ARG CADDY_GEOIP2_VERSION=<current resolved>` and `ARG CADDY_RATELIMIT_VERSION=<current resolved>` near `:64` with `# renovate: datasource=go` annotations, and change lines `:391-392` to `--with github.com/zhangjiayin/caddy-geoip2@v${CADDY_GEOIP2_VERSION}` / `--with github.com/mholt/caddy-ratelimit@v${CADDY_RATELIMIT_VERSION}`. Resolve the current versions at implementation time via `xcaddy`'s build log or `go list -m` in the existing `caddy-inline` module cache. Both ARGs join the §3.4.2 key input set.

**N4 — `golang:${GO_VERSION}-alpine` is a moving tag.** Only `GO_VERSION` (the minor, e.g. `1.27.1`) feeds the key; the underlying `-alpine` digest floats and, post-change, the app hot path no longer `--pull`s it (only the toolchain workflow does). A silent `golang:1.27.1-alpine` rebuild upstream (new Alpine base, patched toolchain) would not change the key. **Fix (Commit 1):** digest-pin both builder-stage bases — `FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine@sha256:<digest> AS caddy-inline` (and `crowdsec-inline`) — with a `# renovate: datasource=docker depName=golang` annotation, and include the pinned digest line in the key input set. The **daily** toolchain rebuild's `--pull` + Renovate digest bumps then keep it fresh; the app build inherits it transitively through the pinned toolchain image — the intended daily-cadence refresh path for builder-base drift (the app hot path deliberately does not re-pull it).

Also inside the stages: many *literal* pinned versions in `go get` lines (e.g. `go-jose/v3@v3.0.5`, `cel-go@v0.29.2`, `quic-go@v0.60.0`, `golang.org/x/mod@v0.40.0`, `ipstore@v0.4.0`). Because these are literals in the stage body, the **content hash of the stage text** (not just the ARG list) must feed the toolchain tag key (§3.4).

### 2.3 `--no-cache-filter` / `no-cache-filters` occurrences (verified — full removal list)

| # | File:line | Form | Job / context |
|---|---|---|---|
| 1 | `.github/workflows/docker-build.yml:463-464` | raw `--no-cache-filter caddy-builder` / `crowdsec-builder` | `build-amd64` (`nick-fields/retry` → raw `docker buildx build`) |
| 2 | `.github/workflows/docker-build.yml:549-550` | raw `--no-cache-filter …` | `build-arm64` |
| 3 | `.github/workflows/security-pr.yml:164` | `no-cache-filters: caddy-builder,crowdsec-builder` (composite input) | `Build Docker image (Local)` step, job `timeout-minutes: 20` (`:32`) |
| 4 | `.github/workflows/supply-chain-pr.yml:261` | `no-cache-filters:` (composite input) | `Build Docker image (Local)` step, job `timeout-minutes: 20` (`:34`) |
| 5 | `.github/workflows/e2e-tests-split.yml:224` | `no-cache-filters:` on `docker/build-push-action` (`:215`) | `build` job |
| 6 | `.github/workflows/nightly-build.yml:243` | `no-cache-filters:` on `docker/build-push-action` (`:229`) | `Build and push Docker image` (multi-arch) |
| 7 | `.github/actions/build-charon-image/action.yml:15` (input decl) + `:52` (passthrough) | `no-cache-filters` composite input, default `''` | consumed by `cerberus-integration.yml:34`, `crowdsec-integration.yml:34`, `waf-integration.yml:34`, `rate-limit-integration.yml:34` (none of those four override it today) |

The composite action's own doc comment (`action.yml:16-33`) instructs CVE-scan callers to set `no-cache-filters: caddy-builder,crowdsec-builder`; that comment must be rewritten (§3.6).

### 2.4 Existing patterns to reuse

- **Weekly security rebuild:** `.github/workflows/security-weekly-rebuild.yml` — `schedule: '0 12 * * 2'` (Tue 12:00 UTC) + `workflow_dispatch{force_rebuild}`, `timeout-minutes: 60`, `no-cache: ${{ schedule || force_rebuild }}`, `pull: true`, publishes `ghcr.io/wikid82/charon:security-scan-YYYYMMDD`, Trivy CRITICAL/HIGH gate + SARIF upload + JSON artifact + failure `::warning::`. **This spec repurposes this workflow to rebuild the *toolchain* image** rather than a throwaway app image (it currently scans an image nobody consumes).
- **Bot-PR-bumps-a-pin:** `.github/workflows/update-geolite2.yml` — weekly cron + `workflow_dispatch`, downloads upstream, `sed -i` the `ARG …_SHA256=` line in the Dockerfile, `docker build --check` syntax gate, `peter-evans/create-pull-request@v8` targeting `base: development`, `branch: bot/update-geolite2-checksum`, labels `dependencies/automated/docker`, failure → `actions/github-script` opens an issue. Commits `15ca90b8` / `94b93fdf` are live examples. **Reuse verbatim structure for the digest-bump bot (§3.4.3).**
- **Toolchain-bump scripts:** `scripts/update-go-toolchain.sh`, `scripts/update-node-toolchain.sh`, `scripts/caddy-compat-matrix.sh` — house style for a `scripts/*.sh` helper invoked by CI.
- **Renovate regex managers** already track every `ARG` above via `# renovate:` annotations — must be preserved (§3.3).

### 2.5 Constraints from `CLAUDE.md` / `ARCHITECTURE.md`

- All frontend in `frontend/`, backend in `backend/` — unaffected (this is CI/build only).
- Conventional commits; `(security)` scope only for genuine security work, subject line vague. The initial-pin and freshness-guard commits *are* security-relevant — use `feat(security):` / `fix(security):` with vague subjects (e.g. `feat(security): pin bundled proxy toolchain to a scanned prebuilt image`). The routine daily digest-refresh bot PR uses **`chore(docker):`** — `feat:` there makes release-please cut a minor release on every refresh.
- Weekly `nightly → main` promotion PRs merge via **merge commit**. This feature's PR targets `development` (normal flow) — **confirmed it does not touch `weekly-nightly-promotion.yml`** and imposes no new constraint on the promotion merge method. (`weekly-nightly-promotion.yml` carries the app image through unchanged; the toolchain digest pin travels with the Dockerfile like any other line.)
- `ARCHITECTURE.md` §"Deployment Architecture / Multi-Stage Dockerfile" (`:1082`), §"Infrastructure" table (`:158`), §"Directory Structure" (`:286`), §"Layer 2: CrowdSec Integration" (`:780`) must be updated (§9).
- **Ignore-file check (CLAUDE.md "Ignore Files"):** the new files are `scripts/toolchain-key.sh`, `scripts/verify-toolchain-pin.sh`, `scripts/lib/dockerfile-stage.sh`, `scripts/tests/toolchain-key.bats` (+ `verify-toolchain-pin.bats`, `helpers/toolchain_fixture.bash`), `.github/workflows/toolchain-image.yml`, `docs/ci/toolchain-image.md`. **Correction (Rev 2.1):** the earlier claim that `scripts/` is not copied into the image was wrong — `Dockerfile` `COPY scripts/ /app/scripts/` copies the whole directory into the runtime image (it already ships ~40 `scripts/*.sh` + a pre-existing `.bats`). These four build-only helpers are used only by `toolchain-image.yml` and the `quality-checks.yml` `verify-toolchain-pin` / bats jobs from a plain checkout — never from inside a built container — so **`.dockerignore` now excludes `scripts/tests/`, `scripts/toolchain-key.sh`, `scripts/verify-toolchain-pin.sh`, `scripts/lib/dockerfile-stage.sh`** (blacklist semantics, no `!scripts/…` re-includes to fight). `.github/` and `docs/` are already excluded, so `toolchain-image.yml` / `docs/ci/toolchain-image.md` never enter the context. `.gitignore` — these are source files that must be committed; none matches an existing ignore glob → **no `.gitignore` change**. `.codecov.yml` — shell/bats and YAML carry no Go/TS coverage → **no `.codecov.yml` change**. Recorded explicitly per CLAUDE.md.

---

## 2.6 Alternatives Considered (Decision Record)

The user has confirmed **approach A (prebuilt toolchain image)**. This section records the lighter alternative that was weighed against it, why that alternative is genuinely viable, and the concrete grounds on which A was still chosen — so the decision is auditable rather than assumed.

### Alternative B — Keep the two stages inline; replace `--no-cache-filter` with a content-hash-keyed buildx GHA cache scope

**Mechanism.** Leave `caddy-builder` / `crowdsec-builder` exactly where they are in the Dockerfile. Delete every `--no-cache-filter caddy-builder,crowdsec-builder`. In its place, give the two expensive stages their own dedicated GHA cache scope whose key is the content hash of the pin set (the same `scripts/toolchain-key.sh` output proposed for approach A):

```
KEY=$(scripts/toolchain-key.sh)          # caddy-crowdsec-<hex>
docker buildx build \
  --cache-from type=gha,scope=charon-app \
  --cache-to   type=gha,mode=max,scope=charon-app \
  --cache-from type=gha,scope=builders-${KEY} \
  --cache-to   type=gha,mode=max,scope=builders-${KEY} \
  ...
```

When a pin moves, `KEY` changes, the `builders-<key>` scope is a guaranteed miss, and the stage recompiles exactly once; every subsequent build on that `KEY` restores the layer. When a pin does **not** move, the layer is restored and no compile happens.

**What Alternative B genuinely delivers — stated fairly:**

- It **does** fix the #1298 timeouts in the common case: after the first build on a given `KEY`, every PR/CI build restores the `caddy-builder` / `crowdsec-builder` layers from `builders-<key>` and skips the ~14 min compile.
- It **preserves the exact pin-bump recurrence guarantee**: the cache key is derived from the pin content, so a bumped `CADDY_VERSION` (or any tracked ARG, or any edit to the stage body) forces a clean recompile — the same property approach A's freshness guard enforces, achieved without a guard because the key *is* the cache identity.
- It is **~half the work**: no new image, no new registry package, no `toolchain-image.yml`, no digest-bump bot, no `verify-toolchain-pin.sh`, no fork-fallback selector stages, no `packages: read` cross-workflow plumbing. Roughly Commits 1, 3 and 6 of approach A's plan, and no new failure surface (GHCR availability, private-package permissions, bot-PR merge latency).
- Local `docker build` is unaffected — no image pull, no login.

**Why approach A is still chosen — concrete grounds:**

1. **GHA cache eviction makes Alternative B's timeout fix unreliable.** GitHub Actions caches (`type=gha`) share a **10 GB per-repository LRU budget**. This repo already runs `gh_cache_cleanup.yml` and its existing timeout comments explicitly cite "cold GHA cache (first run / post-eviction) is a full ~10–14 m image build" (`security-pr.yml:32`, `supply-chain-pr.yml:34`, the integration workflows' `:29`). A `mode=max` multi-stage image cache for Charon is large; the `builders-<key>` scope competes with `docker-build-amd64`, `docker-build-arm64`, `charon-integration-image`, `charon-app`, npm, Go build caches, and the e2e image tarball for that 10 GB. On a busy week the `builders-<key>` entry is evicted between runs and the **next** PR eats a cold ~14 min compile again — i.e. Alternative B reduces the *frequency* of timeout-class builds but does not *eliminate* them, which is the actual acceptance bar (§5 AC #2: no timeout across 3 consecutive runs, and none thereafter). A digest-pinned image in GHCR's package store is **not** subject to the Actions cache LRU — it is pulled, not cache-restored — so approach A removes the cold-build possibility entirely rather than making it rarer.

2. **Trivy scans a small, stable, isolated artifact.** With approach A, the weekly/daily security scan targets `ghcr.io/wikid82/charon-toolchain` — two binaries plus an Alpine base, a stable surface whose findings map directly to the bundled Caddy/CrowdSec supply chain. With Alternative B there is no separate artifact: every scan re-derives bundled-binary findings from the full application image on every run, mixed with app-layer and base-image findings, and there is no way to pin/attest "the bundled toolchain that was scanned green on date X" independently of the app image.

3. **The recompile cost is paid out-of-band.** Under approach A the ~14–30 min compile only ever runs in `toolchain-image.yml` (45 min budget) or `security-weekly-rebuild.yml` (60 min budget) — never on a contributor's PR or on `docker-build.yml`'s tight per-arch budgets. Under Alternative B the first build on every new `KEY` (every pin bump — routine, Renovate opens several a week) pays the full compile *on whatever PR happens to bump the pin*, on that PR's normal timeout budget. B6/§3.9 shows those budgets are already close to the edge.

4. **Eviction-immunity also fixes the arm64 leg.** `docker-build.yml`'s `build-arm64` runs under QEMU; today it emulates the *fast* stages plus the final-stage `RUN` lines around a cold-or-warm builders layer. Under approach A the arm64 app build does a `COPY --from` of a pre-cross-compiled binary out of the pinned image's arm64 child — no dependence on an arm64-scoped GHA cache entry surviving. Alternative B's `builders-<key>` scope for arm64 is a separate, separately-evictable entry.

**Residual point in Alternative B's favour, acknowledged:** approach A adds GHCR as a hard build dependency and a private-package permission surface (N8), needs a fork fallback (§3.7), and is more moving parts to operate. The mitigations are in §3.10 (retry wrap, documented inline fallback, `imagetools` platform assertion) and the operator runbook (`docs/ci/toolchain-image.md`, §9). On balance the eviction-immunity (point 1) is decisive: it is the difference between "timeouts become rarer" and "timeouts cannot happen", and the latter is the stated goal.

### Alternative C — Bake binaries into a committed build artifact / Git LFS

Rejected without deep analysis: storing compiled multi-arch binaries in the repo (or LFS) defeats reproducibility, bloats history, has no scan/attestation story, and still needs a refresh mechanism. Strictly worse than A on every axis that matters here.

---

## 3. Technical Specifications

### 3.1 Target architecture

Extract the two expensive stages' *outputs* into a **separately-versioned, independently-scanned multi-arch prebuilt image** — `ghcr.io/wikid82/charon-toolchain` — so the `xcaddy` / CrowdSec compile happens on a **daily schedule and whenever a tracked pin moves**, not once per app build.

**Single source of truth:** the build recipe stays in the **main `Dockerfile`**. The existing stage bodies are renamed `caddy-builder → caddy-inline` and `crowdsec-builder → crowdsec-inline`. A new thin `toolchain-runtime` stage assembles their outputs into a publishable image. The toolchain workflow builds `--target toolchain-runtime`; the app build selects between the prebuilt image and the inline stages via a build-arg. No recipe duplication.

#### Build graph — BEFORE

```
                          ┌─────────────────────────────┐
 every app build  ───────►│ caddy-builder   (748 s)     │──┐
 (CI: --no-cache-filter)  │ xcaddy build + patch + build│  │
                          └─────────────────────────────┘  │  COPY --from
                          ┌─────────────────────────────┐  ├──►  final runtime image
 every app build  ───────►│ crowdsec-builder (330 s)    │──┘
 (CI: --no-cache-filter)  │ clone + patch + xx-go build │
                          └─────────────────────────────┘
   cold compile on EVERY: docker-build (amd64+arm64), nightly, security-pr,
   supply-chain-pr, e2e-tests-split, 4× integration workflows
```

#### Build graph — AFTER

```
  ┌──────────────────────── toolchain image lifecycle (rare) ─────────────────────────┐
  │ trigger: daily cron | workflow_dispatch | PR touching toolchain inputs             │
  │                                                                                   │
  │   docker buildx build --target toolchain-runtime                                  │
  │     --platform linux/amd64,linux/arm64  --no-cache --pull  (cron/dispatch)        │
  │        caddy-inline  (cross-compile, no QEMU)  ─┐                                  │
  │        crowdsec-inline (xx cross-compile, no QEMU) ─┤                              │
  │        toolchain-runtime: FROM alpine; COPY both ─┘                               │
  │     → push ghcr.io/wikid82/charon-toolchain:caddy-crowdsec-<key>  (+ :latest,     │
  │       + :<date>)   → Trivy CRITICAL/HIGH gate → SARIF                             │
  │     → if new digest: bot PR bumps ARG CHARON_TOOLCHAIN_DIGEST in Dockerfile       │
  └───────────────────────────────────────────────────────────────────────────────────┘
                                        │  digest pin (one ARG line in Dockerfile)
                                        ▼
  ┌──────────────────────── every app build (hot path) ──────────────────────────────┐
  │  FROM ${CHARON_TOOLCHAIN_IMAGE}@${CHARON_TOOLCHAIN_DIGEST} AS toolchain-prebuilt  │
  │  FROM ${CADDY_BUILDER_SRC}   AS caddy-builder      (default → toolchain-prebuilt) │
  │  FROM ${CROWDSEC_BUILDER_SRC} AS crowdsec-builder  (default → toolchain-prebuilt) │
  │       COPY --from=caddy-builder    /usr/bin/caddy         ...   (UNCHANGED)        │
  │       COPY --from=crowdsec-builder /crowdsec-out/crowdsec ...   (UNCHANGED)       │
  │  normal type=gha layer cache covers every stage; NO --no-cache-filter            │
  │                                                                                   │
  │  fallback (fork PR / bootstrap / offline):                                        │
  │     --build-arg CADDY_BUILDER_SRC=caddy-inline                                    │
  │     --build-arg CROWDSEC_BUILDER_SRC=crowdsec-inline   → compiles from source     │
  └───────────────────────────────────────────────────────────────────────────────────┘
```

### 3.2 `Dockerfile` changes

#### 3.2.1 New ARGs (add near the pinned-toolchain block, `:11`)

```dockerfile
# ---- Prebuilt Caddy + CrowdSec toolchain image ----
# Built by .github/workflows/toolchain-image.yml from the caddy-inline /
# crowdsec-inline stages below. Bumped by the open-bump-pr job (bot PR) when a
# security-relevant input moves OR the DAILY --no-cache --pull rebuild produces
# a new digest. The freshness-guard CI check (scripts/verify-toolchain-pin.sh)
# fails any PR where TAG/DIGEST is stale for the current pins.
ARG CHARON_TOOLCHAIN_IMAGE=ghcr.io/wikid82/charon-toolchain
# NOT Renovate-tracked (content-hash tag has no series to follow, N7) — the
# open-bump-pr bot in toolchain-image.yml owns these two lines.
ARG CHARON_TOOLCHAIN_TAG=caddy-crowdsec-0000000000000000
ARG CHARON_TOOLCHAIN_DIGEST=sha256:<filled-by-first-publish>

# Stage selector — default uses the prebuilt image; fork PRs / bootstrap /
# offline builds pass `--build-arg CADDY_BUILDER_SRC=caddy-inline
# --build-arg CROWDSEC_BUILDER_SRC=crowdsec-inline` to compile from source.
ARG CADDY_BUILDER_SRC=toolchain-prebuilt
ARG CROWDSEC_BUILDER_SRC=toolchain-prebuilt
```

#### 3.2.2 Rename existing stages + close the two pin gaps (Commit 1)

- `Dockerfile:302` — `FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine AS caddy-builder` → `FROM --platform=$BUILDPLATFORM golang:${GO_VERSION}-alpine@sha256:<digest> AS caddy-inline` (N4 — digest-pin the base).
- `Dockerfile:577` — same treatment for `crowdsec-builder` → `crowdsec-inline`.
- Add near `:64`, with `# renovate: datasource=go` annotations (B4):
  ```dockerfile
  # renovate: datasource=go depName=github.com/zhangjiayin/caddy-geoip2
  ARG CADDY_GEOIP2_VERSION=<resolve at impl time>
  # renovate: datasource=go depName=github.com/mholt/caddy-ratelimit
  ARG CADDY_RATELIMIT_VERSION=<resolve at impl time>
  ```
  and change `Dockerfile:391-392` to `--with github.com/zhangjiayin/caddy-geoip2@v${CADDY_GEOIP2_VERSION}` / `--with github.com/mholt/caddy-ratelimit@v${CADDY_RATELIMIT_VERSION}` (declare both ARGs inside `caddy-inline` alongside the other `ARG CADDY_*` at `:305-310`).
- **Delete** the dead `crowdsec-fallback` stage (`:713-748`) and the now-dead `CROWDSEC_RELEASE_SHA256` ARG (`:22`, `:586`) — N1.

Apart from the base-image digest and the two plugin `@version` suffixes, **no logic inside the two stages changes**. All `go get` patches, module-cache source patches, and embeds-version assertions are retained verbatim — they are the security recipe and the toolchain image is *the* place they now run.

#### 3.2.3 New `toolchain-prebuilt` and `toolchain-runtime` stages

Insert after `crowdsec-inline` (where `crowdsec-fallback` used to be, now deleted):

```dockerfile
# ---- Prebuilt toolchain (default source for caddy-builder / crowdsec-builder) ----
# Digest-pinned. Contains /usr/bin/caddy and /crowdsec-out/{crowdsec,cscli,config}
# at the SAME paths the inline stages produce, so the COPY --from lines in the
# final stage need no change.
FROM ${CHARON_TOOLCHAIN_IMAGE}@${CHARON_TOOLCHAIN_DIGEST} AS toolchain-prebuilt

# ---- Toolchain image assembly target (built by toolchain-image.yml) ----
# NOT part of the app build graph (nothing FROMs it there). `docker buildx build
# --target toolchain-runtime` produces the publishable multi-arch image.
FROM ${ALPINE_IMAGE} AS toolchain-runtime
COPY --from=caddy-inline    /usr/bin/caddy          /usr/bin/caddy
COPY --from=crowdsec-inline /crowdsec-out/crowdsec  /crowdsec-out/crowdsec
COPY --from=crowdsec-inline /crowdsec-out/cscli     /crowdsec-out/cscli
COPY --from=crowdsec-inline /crowdsec-out/config    /crowdsec-out/config
# Provenance label so `docker inspect` on the toolchain image shows the key.
LABEL io.charon.toolchain.key="${CHARON_TOOLCHAIN_TAG}"

# ---- Effective builder stages: alias to prebuilt image OR inline compile ----
FROM ${CADDY_BUILDER_SRC}    AS caddy-builder
FROM ${CROWDSEC_BUILDER_SRC} AS crowdsec-builder
```

`FROM ${ARG} AS name` where the ARG resolves to a **prior stage name** is valid BuildKit; unreferenced stages (`caddy-inline` etc. when `…_SRC=toolchain-prebuilt`) are pruned from the graph and never built. When `…_SRC=caddy-inline`, `toolchain-prebuilt` is still declared but unreferenced → also pruned, so a fork build never needs to pull the image.

**`crowdsec-fallback` (`:713-748`):** deleted in Commit 1 — it is dead code (verified, §2.1 correction). Nothing referenced it before this change. If the reviewer wants it retained as an escape hatch, it stays out of the toolchain key and out of the graph regardless.

#### 3.2.4 Final-stage `COPY --from` lines — UNCHANGED, plus a cheap embed assertion (N5)

`Dockerfile:807`, `:814`, `:815`, `:817` keep referencing `caddy-builder` / `crowdsec-builder` and the same source paths. This is the whole point of putting the binaries at identical paths in `toolchain-runtime`.

**N5 — add a post-`COPY` assertion in the final stage.** Today the "did the binary embed the fixed cel-go / grpc-go" checks (`Dockerfile:564`, `:569`) run *inside* `caddy-inline` — so on the prebuilt path they only ever executed when the toolchain image was built, and a wrong/rolled-back `CHARON_TOOLCHAIN_DIGEST` (or a hand-edited pin pointing at an old image) would sail through the app build silently. Add a small `RUN` right after `COPY --from=caddy-builder … /usr/bin/caddy` (and the crowdsec copies):

```dockerfile
RUN set -e; \
    caddy list-modules 2>/dev/null | grep -q 'http.handlers.rate_limit' || { echo "toolchain image missing expected caddy plugins"; exit 1; }; \
    go_ver_check() { command -v go >/dev/null && go version -m "$1" || true; }; \
    /usr/local/bin/cscli version >/dev/null || { echo "cscli from toolchain image not runnable"; exit 1; }
```

The final stage has no Go toolchain, so a full `go version -m` embed check is not possible there — instead assert (a) the Caddy binary loads and lists the expected custom plugins (`rate_limit`, `crowdsec`, `geoip2`, `coraza`), (b) `cscli version` runs and prints the expected `v${CROWDSEC_VERSION}`. A wrong-arch or stale-recipe image fails these immediately. The authoritative embeds-version assertions remain in `caddy-inline` and run in `toolchain-image.yml`. Additionally, a CI step in `docker-build.yml` (it already has a "Caddy/CrowdSec CVE verification" step post-build, `merge-and-publish`) runs `docker run --rm <img> go version -m /usr/bin/caddy | grep …` against the *final* image for the full check — extend that existing step to also assert the toolchain `LABEL io.charon.toolchain.key` matches `scripts/toolchain-key.sh`.

### 3.3 Renovate / pin-tracking

- Every `# renovate:` annotation on the version ARGs stays. Renovate keeps bumping `CADDY_VERSION` etc. as today; the two new plugin ARGs (B4) and the digest-pinned `golang` base (N4) get annotations too.
- A Renovate bump to any of those ARGs now *also* needs a toolchain rebuild. The **freshness guard** (§3.4.2) turns that into a hard PR failure with a one-line fix (`workflow_dispatch` the toolchain workflow, or wait for the bot), so a Renovate PR that bumps `CADDY_VERSION` cannot merge with a stale toolchain.
- **N7 (corrected):** the `CHARON_TOOLCHAIN_IMAGE` / `_TAG` / `_DIGEST` three-ARG split with a **content-hash tag** (`caddy-crowdsec-<hex>`) is *not* something Renovate's `datasource=docker` manager tracks out of the box — it has no semver/digest series to follow on that tag. It simply won't fire, which is harmless: the daily rebuild + digest-bump bot (§3.4.3) is the sole authority on that pin. Do **not** add a Renovate entry implying it works; add a comment in `renovate.json` stating the toolchain digest is bot-owned.

### 3.4 New workflow: `.github/workflows/toolchain-image.yml`

Builds & publishes `ghcr.io/wikid82/charon-toolchain`.

#### 3.4.1 Triggers, permissions, concurrency

```yaml
name: Toolchain Image — Build & Publish
on:
  schedule:
    - cron: '0 6 * * *'          # DAILY 06:00 UTC — committed scope (B2). --no-cache --pull.
  workflow_dispatch:
    inputs:
      force_rebuild: { type: boolean, default: true, description: "Build with --no-cache --pull" }
  pull_request:
    paths:
      - 'Dockerfile'                                  # coarse; the key script decides if it truly changed
      - '.github/workflows/toolchain-image.yml'
      - 'scripts/toolchain-key.sh'
      - 'scripts/verify-toolchain-pin.sh'
      - 'scripts/lib/dockerfile-stage.sh'
      - '.trivyignore'
  # The Tuesday `security-weekly-rebuild.yml` also `workflow_call`s this workflow for the
  # heavier "full Trivy report + SARIF + JSON artifact" pass; the daily `schedule` above
  # is the freshness driver. Two entry points, one build definition.
  workflow_call:
    inputs:
      force_rebuild: { type: boolean, default: true }
      publish:       { type: boolean, default: true }   # PR path builds but does not push :latest
concurrency:
  group: toolchain-image-${{ github.ref }}
  cancel-in-progress: false          # never cancel a publish mid-push
permissions:
  contents: read
  packages: write                    # push to GHCR
  security-events: write             # Trivy SARIF
  pull-requests: write               # bot digest-bump PR (schedule/dispatch/workflow_call only)
```

**Daily cadence is committed scope, not optional (B2).** See §3.8 for the baseline analysis that requires it. The `schedule` trigger runs `--no-cache --pull` every day at 06:00 UTC; on a day with no digest change it is a ~30-minute no-op (acceptable — one runner, off-peak). `security-weekly-rebuild.yml` keeps its Tuesday slot for the fuller scan/report but is no longer the *only* forced-rebuild driver.

- **`pull_request` from a fork:** GitHub grants only `contents: read`, no `packages: write`. The job's publish/push steps are guarded `if: github.event.pull_request.head.repo.full_name == github.repository`. On a fork PR the workflow still *builds* `--target toolchain-runtime` (validates the recipe compiles) but does not push and does not open a bot PR. The fork's *app* build meanwhile uses the inline fallback (§3.7), so a fork PR is fully testable without the image.
- **`timeout-minutes: 45`** (cold amd64+arm64 cross-compile of both stages ≈ 25–30 min + Trivy).

#### 3.4.2 Tag key derivation — `scripts/toolchain-key.sh`

Deterministic, content-addressed. Output: `caddy-crowdsec-<16 hex>`.

Inputs to the SHA-256:

1. The **exact text** of the `caddy-inline` stage (`Dockerfile` from `FROM … AS caddy-inline` to the blank line before the next `FROM`), extracted by the **shared** `extract_stage` routine in `scripts/lib/dockerfile-stage.sh` (N9 — one copy, `source`d by both `toolchain-key.sh` and `verify-toolchain-pin.sh`).
2. The **exact text** of the `crowdsec-inline` stage (same routine).
3. The resolved default values of every ARG in the §2.2 table — **including the two new `CADDY_GEOIP2_VERSION` / `CADDY_RATELIMIT_VERSION` plugin pins (B4)** — parsed from the `ARG NAME=default` lines, so a bump to `CADDY_VERSION` (or a plugin) changes the key even though the stage body only interpolates `${…}`.
4. The `tonistiigi/xx` pin line (`:73`) **and the digest-pinned `golang:${GO_VERSION}-alpine@sha256:…` base lines of both inline stages (N4)**.
5. `sha256sum .trivyignore`.
6. A `SCHEMA_VERSION` constant in the script (bump to force a global rebuild if the recipe-extraction logic itself changes).

```bash
#!/usr/bin/env bash
# scripts/lib/dockerfile-stage.sh — SHARED (N9). sourced by both scripts.
extract_stage() {  # $1 = stage name, $2 = Dockerfile path
  awk -v s="$1" '
    $0 ~ ("AS "s"$") {c=1}
    c {print}
    c && /^$/ && NR>1 {exit}
    END { if (!c) { print "extract_stage: no stage \"" s "\"" > "/dev/stderr"; exit 3 } }' "$2"
}
```

```bash
#!/usr/bin/env bash
# scripts/toolchain-key.sh — prints the deterministic toolchain image tag.
set -euo pipefail
SCHEMA_VERSION=2   # rev-2: added plugin pins + digest-pinned golang base to the key
df="${1:-Dockerfile}"
here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
# shellcheck source=scripts/lib/dockerfile-stage.sh
source "$here/lib/dockerfile-stage.sh"

caddy_stage="$(extract_stage caddy-inline "$df")"
crowdsec_stage="$(extract_stage crowdsec-inline "$df")"
# sanity: each stage must be non-trivial and actually build something
for s in "$caddy_stage" "$crowdsec_stage"; do
  [[ "$(wc -l <<<"$s")" -ge 20 ]] && grep -q 'go build\|xx-go build' <<<"$s" \
    || { echo "toolchain-key: stage extraction looks wrong" >&2; exit 3; }
done
{
  echo "schema=$SCHEMA_VERSION"
  printf '%s\n' "$caddy_stage" "$crowdsec_stage"
  grep -E '^ARG (GO_VERSION|ALPINE_IMAGE|CROWDSEC_VERSION|EXPR_LANG_VERSION|XNET_VERSION|XCRYPTO_VERSION|KLAUSPOST_COMPRESS_VERSION|GRPC_VERSION|CADDY_VERSION|CADDY_CANDIDATE_VERSION|CADDY_USE_CANDIDATE|CADDY_PATCH_SCENARIO|CADDY_SECURITY_VERSION|CORAZA_CADDY_VERSION|CADDY_GEOIP2_VERSION|CADDY_RATELIMIT_VERSION)=' "$df"
  grep -E 'tonistiigi/xx:|^FROM .*golang:.*-alpine@sha256:' "$df"
  sha256sum .trivyignore | cut -d' ' -f1
} | sha256sum | cut -c1-16 | sed 's/^/caddy-crowdsec-/'
```

Freshness guard — `scripts/verify-toolchain-pin.sh` (runs in `quality-checks.yml` on every PR, fast, no Docker build). **B7 — failure-closed on same-repo PRs:**

```bash
#!/usr/bin/env bash
set -euo pipefail
KEY="$(scripts/toolchain-key.sh)"
PINNED_TAG="$(grep -E '^ARG CHARON_TOOLCHAIN_TAG=' Dockerfile | cut -d= -f2)"
PINNED_DIGEST="$(grep -E '^ARG CHARON_TOOLCHAIN_DIGEST=' Dockerfile | cut -d= -f2)"

# Is this a trusted, same-repo run (has/should-have a registry-read token)?
#   - push / same-repo pull_request / workflow_dispatch / schedule  -> SAME_REPO=1
#   - pull_request from a fork                                       -> SAME_REPO=0
SAME_REPO=1
if [[ "${GITHUB_EVENT_NAME:-}" == "pull_request" \
   && "${GITHUB_EVENT_PULL_REQUEST_HEAD_REPO_FULL_NAME:-}" != "${GITHUB_REPOSITORY:-}" ]]; then
  SAME_REPO=0
fi

if [[ "$KEY" != "$PINNED_TAG" ]]; then
  echo "::error::Toolchain recipe/pins changed (recomputed $KEY, Dockerfile pins $PINNED_TAG)."
  echo "::error::Run the 'Toolchain Image' workflow (workflow_dispatch) or wait for the bot PR, then bump ARG CHARON_TOOLCHAIN_TAG/DIGEST."
  exit 1
fi

if [[ "$SAME_REPO" == "1" ]]; then
  # HARD requirement: the tool AND the token must be present, and the pinned digest
  # MUST resolve and MUST equal what GHCR serves for :$KEY. No silent skip.
  command -v regctl >/dev/null || { echo "::error::regctl missing on a same-repo run — cannot verify digest"; exit 1; }
  : "${GHCR_READ_TOKEN:?::error::GHCR_READ_TOKEN unset on a same-repo run — cannot verify digest}"
  REMOTE_DIGEST="$(regctl image digest "ghcr.io/wikid82/charon-toolchain:$KEY")" \
    || { echo "::error:::$KEY does not resolve in GHCR — toolchain image was never published for this pin"; exit 1; }
  if [[ "$REMOTE_DIGEST" != "$PINNED_DIGEST" ]]; then
    echo "::error::Dockerfile pins $PINNED_DIGEST but GHCR :$KEY = $REMOTE_DIGEST (hand-edited or stale)."
    exit 1
  fi
  echo "Toolchain pin verified (same-repo): $KEY @ $PINNED_DIGEST"
else
  # Fork PR: no packages:read, cannot reach GHCR. Degrade to tag-only equality
  # (already checked above). The real digest check runs when a maintainer
  # re-dispatches the same-repo event (see security-pr.yml workflow_run gate).
  echo "::warning::Fork PR — digest existence not verified (no registry access). Tag matches recomputed key."
fi
```

`GHCR_READ_TOKEN` is `${{ secrets.GITHUB_TOKEN }}` (has `packages: read` for a repo-internal package once N8's package-linking is done); `regctl` is installed by the job (`ghcr.io/regclient/regctl` container or `iarekylew00t/regctl-installer`).

- On a **same-repo PR that legitimately bumps a pin**: `toolchain-image.yml` (path trigger) builds & pushes `:<newkey>`, and its `sync-pin-on-pr` job commits the `CHARON_TOOLCHAIN_TAG`/`DIGEST` bump onto the PR head branch, so the guard goes green within the same PR.
- On a **fork PR that bumps a pin**: guard fails on the tag mismatch with instructions to have a maintainer dispatch the workflow — acceptable, rare, safe. The fork's app build meanwhile uses the inline fallback (§3.7).

#### 3.4.3 Jobs

> **Amendment (Rev 2.1, post-approval — flagged for supervisor re-review).**
> BuildKit's default provenance / SBOM attestation manifests embed per-run
> timestamps + builder identity, so the OCI-index (manifest-list) digest of an
> otherwise byte-identical build changes on every run. Combined with
> `sync-pin-on-pr` + the path-filtered `pull_request` trigger this produced a
> self-perpetuating bot-commit loop on the feature PR. Fixes, all in this PR:
>
> 1. **Deterministic build.** `--provenance=false --sbom=false`, a **fixed**
>    `SOURCE_DATE_EPOCH` (`1700000000`), and
>    `--output type=image,"name=…:KEY,…:DATE,…:latest",push=true,rewrite-timestamp=true`.
>    The toolchain image is an internal build *input*; the app image's own
>    provenance/SBOM (in `docker-build.yml`) is separate and unaffected. Result:
>    identical toolchain key ⇒ identical manifest-list digest (verified by two
>    independent builds producing the same digest).
> 2. **Skip-if-already-published.** On any non-forced event (`pull_request`,
>    plain path trigger) the job first `imagetools inspect`s `:${KEY}`; if it
>    resolves, it SKIPS the build/push entirely and emits that existing digest.
>    Only `schedule` / `workflow_dispatch force_rebuild=true` / `workflow_call`
>    actually rebuild + repush. This also removes the ~30-min rebuild from
>    unrelated Dockerfile PRs.
> 3. **`sync-pin-on-pr` is idempotent + self-trigger-safe:** guarded
>    `github.actor != 'github-actions[bot]'` and no-ops unless
>    `git diff --quiet Dockerfile` shows a real change after the sed.

```
build-toolchain:
  - checkout
  - KEY=$(scripts/toolchain-key.sh); echo to $GITHUB_OUTPUT
  - Set up QEMU? NO. Set up Buildx.
  - login GHCR (skip on fork)
  - PLAN: forced = (schedule || force_rebuild); if !forced && same-repo &&
    `imagetools inspect :${KEY}` resolves -> should_build=false, reuse that digest
  - if should_build:  SOURCE_DATE_EPOCH=1700000000 docker buildx build
      --target toolchain-runtime
      --platform linux/amd64,linux/arm64
      $( forced && echo --no-cache --pull )
      --provenance=false --sbom=false
      --cache-from type=gha,scope=toolchain
      --cache-to   type=gha,mode=max,scope=toolchain
      --output type=image,"name=…:KEY,…:$(date +%Y%m%d)$( same-repo && echo ,…:latest )",push=$( same-repo && echo true || echo false via type=cacheonly ),rewrite-timestamp=true
      .
  - DIGEST = existing_digest (if skipped) else
    $(docker buildx imagetools inspect …:${KEY} --format '{{json .Manifest}}' | jq -r .digest)
  - outputs: key, digest, same_repo

trivy-scan:
  needs: build-toolchain
  - trivy image --severity CRITICAL,HIGH --exit-code 1 --ignorefile .trivyignore \
        ghcr.io/wikid82/charon-toolchain@${{ needs.build-toolchain.outputs.digest }}
  - trivy image --format sarif ... → upload-sarif (category: toolchain-image:trivy)
  - continue-on-error on the gate step is FALSE on schedule/dispatch (must be clean),
    TRUE on PR (report-only; the app-image Trivy gates still run downstream)

sync-pin-on-pr:              # pull_request && same-repo && actor != github-actions[bot]
  needs: [build-toolchain]
  - sed -i "s|^ARG CHARON_TOOLCHAIN_TAG=.*|ARG CHARON_TOOLCHAIN_TAG=${KEY}|" Dockerfile
  - sed -i "s|^ARG CHARON_TOOLCHAIN_DIGEST=.*|ARG CHARON_TOOLCHAIN_DIGEST=${DIGEST}|" Dockerfile
  - if `git diff --quiet Dockerfile`: exit 0 (no commit — idempotent)
  - git commit -m "chore(docker): sync toolchain image pin to ${KEY}" && git push (to PR head branch)
  - ::notice:: re-run the freshness check (GITHUB_TOKEN pushes don't re-trigger PR checks)

open-bump-pr:                # event == schedule | workflow_dispatch | workflow_call ; NEVER on pull_request
  needs: [build-toolchain, trivy-scan]
  if: digest changed vs Dockerfile pin
  - sed -i the two ARG lines (CHARON_TOOLCHAIN_TAG, CHARON_TOOLCHAIN_DIGEST)
  - docker build --check -f Dockerfile .
  - peter-evans/create-pull-request@5f6978faf089d4d20b00c7766989d076bb2fc7f1 # v8.1.1
      base: development
      branch: bot/bump-toolchain-image           # updated in place if already open
      title: "chore(docker): refresh bundled proxy toolchain image"
      labels: dependencies, automated, docker, security
      body: old→new digest, Trivy CRITICAL/HIGH summary, verification checklist
  - on failure: actions/github-script → open issue "🚨 Toolchain image rebuild failed"
```

The bot-PR title/body must **not** name the specific CVE/dependency (CLAUDE.md `(security)` vagueness rule): "refresh bundled proxy toolchain image so the shipped Caddy/CrowdSec binaries pick up upstream fixes."

**Trivy gate semantics (revised):** the `--exit-code 1` CRITICAL/HIGH step is **blocking** on `schedule` / `workflow_dispatch` / `workflow_call` (a known CRITICAL in the bundled binaries turns the daily run red → failure issue). On `pull_request` it is **report-only** (`continue-on-error: true`) because the app-image Trivy gates in `docker-build.yml` / `security-pr.yml` still run downstream and a contributor PR must not be blocked by a pre-existing bundled-binary finding they did not introduce.

#### 3.4.4 Repurpose `security-weekly-rebuild.yml` (N6)

Replace its `Build Docker image (NO CACHE)` step — which builds a `charon:security-scan-YYYYMMDD` app image **that nothing consumes** — with `uses: ./.github/workflows/toolchain-image.yml` (`workflow_call`, `force_rebuild: true`). Keep its Trivy CRITICAL/HIGH table + SARIF upload + JSON artifact + failure `::warning::` steps, re-pointed at the toolchain digest.

- Its `permissions:` block (`security-weekly-rebuild.yml:21`, currently `contents: read`, and job-level `:36-39` `contents/packages/security-events`) **must add `pull-requests: write`** — a `workflow_call`ed workflow cannot request perms the caller did not grant, so the caller must grant everything `open-bump-pr` needs (`contents: write`, `pull-requests: write`, `packages: write`, `security-events: write`).
- Keep `TRIVY_SARIF_CATEGORY` stable to avoid duplicate code-scanning tracks; rename the value to `…:trivy-toolchain`.
- **Cadence:** the Tuesday slot stays for the fuller report; the **daily** `schedule` in `toolchain-image.yml` (§3.4.1) is the freshness driver. What the forced rebuild actually catches is stated precisely in §3.8 — **not** "upstream `go get` MVS drift" (that claim was wrong, see §3.8 / B3).

### 3.5 Multi-arch handling (hard constraint)

**Decision: publish a genuine multi-arch manifest list, built without QEMU via `$BUILDPLATFORM` cross-compilation.**

Justification:
- `caddy-inline` is already `FROM --platform=$BUILDPLATFORM golang:…` + `GOOS=$TARGETOS GOARCH=$TARGETARCH go build` (CGO off). `docker buildx build --platform linux/amd64,linux/arm64` runs this stage once per target platform, all on the amd64 host; each pass emits the correct-arch `caddy`. No emulation.
- `crowdsec-inline` is `FROM --platform=$BUILDPLATFORM golang:…` + `COPY --from=xx / /` + `xx-apk add … musl` + `CGO_ENABLED=1 xx-go build`. `tonistiigi/xx` provides the cross linker/sysroot; this is exactly how CrowdSec cross-compiles today for the arm64 leg of `docker-build.yml`. No emulation.
- `toolchain-runtime` is `FROM ${ALPINE_IMAGE}` + `COPY` only — no `RUN`, so nothing arch-specific executes; BuildKit assembles one layer per platform from the matching `caddy-inline`/`crowdsec-inline` outputs.
- Result: `ghcr.io/wikid82/charon-toolchain:<key>` is a manifest list with `linux/amd64` and `linux/arm64` children. In the app build, `FROM …@sha256:<listdigest> AS toolchain-prebuilt` **without** `--platform` → BuildKit auto-selects the child matching the app build's `$TARGETPLATFORM`. So `docker-build.yml`'s `build-amd64` pulls the amd64 child, `build-arm64` (QEMU) pulls the arm64 child, and each does a plain `COPY --from` instead of running the builders.
- **N2 — accurate framing:** the Caddy/CrowdSec compile was *never* QEMU-emulated on the arm64 leg — both builder stages are `FROM --platform=$BUILDPLATFORM` and always cross-compiled natively on the amd64 host. QEMU on `build-arm64` only ever executed the *final* arm64 stage's `RUN` lines (apk installs, setcap, GeoIP fetch, verification). The real win here is **no compile at all** on any app build (cold or warm, amd64 or arm64) — not "arm64 stops emulating a compile". The arm64 leg still runs its final-stage `RUN` lines under QEMU exactly as before.
- The pinned `CHARON_TOOLCHAIN_DIGEST` is the **manifest-list digest** (arch-independent), so one pin covers both arches.

Runner cost: the toolchain workflow does ~30 min of cross-compile once a day on one `ubuntu-latest` (mostly cache-hit no-ops between pin bumps), versus today's cold ~14-min compile on effectively every app build across `docker-build` (×2 arch), `nightly-build` (daily), `security-pr`, `supply-chain-pr`, `e2e-tests-split`, and 4 integration workflows.

### 3.6 `--no-cache-filter` retarget (Commit 1) then removal (Commit 4, R6) — exact edits

**Two-step, per B5.** Commit 1 changes the *value* at every site from `caddy-builder,crowdsec-builder` to `caddy-inline,crowdsec-inline` (so the recurrence guard keeps invalidating the actual `RUN` layers through the rename). Commit 4 — only after `verify-toolchain-pin` is a live required check — deletes them entirely. The table below is the Commit 4 removal list; Commit 1 touches the same sites with a value change.

| File | Edit (Commit 4 = delete; Commit 1 = retarget value first) |
|---|---|
| `docker-build.yml:463-464` | delete the two `--no-cache-filter …` array lines in the `build-amd64` `BUILD_CMD` |
| `docker-build.yml:549-550` | delete the two `--no-cache-filter …` lines in `build-arm64` `BUILD_CMD` |
| `docker-build.yml:392` | rewrite comment: drop "no-cache-filter passed as native buildx flags"; note toolchain image is digest-pinned so layer cache is authoritative |
| `security-pr.yml:157-164` | remove the `with: no-cache-filters:` block + its 6-line justification comment from the `build-charon-image` step |
| `supply-chain-pr.yml:252-261` | same removal |
| `e2e-tests-split.yml:224` | delete `no-cache-filters: caddy-builder,crowdsec-builder` from the `docker/build-push-action` `with:` |
| `nightly-build.yml:243` | delete `no-cache-filters: caddy-builder,crowdsec-builder` |
| `.github/actions/build-charon-image/action.yml` | remove the `no-cache-filters` input (`:11-33` decl) and the `no-cache-filters: ${{ inputs.no-cache-filters }}` passthrough (`:52`); rewrite the `description:` to state the toolchain image is prebuilt+digest-pinned and every stage is layer-cached |
| `crowdsec-integration.yml`, `waf-integration.yml`, `rate-limit-integration.yml`, `cerberus-integration.yml` | no change needed (none passes the input) — but verify after the input is deleted that the composite still resolves (it will; input had a default) |

After removal, add to each build step (where not already present) `--build-arg CHARON_TOOLCHAIN_DIGEST` is **not** needed — the Dockerfile default is authoritative. CI passes nothing extra on the happy path.

### 3.7 Fork PR / bootstrap / offline fallback (R4, hard constraint)

Three cases, one mechanism (`CADDY_BUILDER_SRC` / `CROWDSEC_BUILDER_SRC` build-args, §3.2.1):

| Case | Detection | Behavior |
|---|---|---|
| **Fork PR** (no `packages: write`, cannot pull an internal image) | job-level expression `github.event.pull_request.head.repo.full_name != github.repository` sets `TOOLCHAIN_SRC=inline` | Every app-image build step passes `--build-arg CADDY_BUILDER_SRC=${{ env.CADDY_SRC }} --build-arg CROWDSEC_BUILDER_SRC=${{ env.CROWDSEC_SRC }}` where the two env vars are `caddy-inline`/`crowdsec-inline` on a fork and `toolchain-prebuilt`/`toolchain-prebuilt` otherwise. Full from-source compile (~14 min). Layer cache (`type=gha`) still applies to the fork's own repeated runs. **Because this path exists, the job `timeout-minutes` for every fork-reachable build job stays ≥ 20 (see §3.9 / B6) — it is NOT cut to 15.** |
| **Bootstrap** (toolchain image does not yet exist) | first `toolchain-image.yml` run publishes it; until then `CHARON_TOOLCHAIN_DIGEST` is a placeholder | Commit 1 publishes the image manually (`workflow_dispatch`) and links/marks the GHCR package internal (N8) **before** Commit 2 flips the Dockerfile default. Freshness guard lands in Commit 3; the `--no-cache-filter` sites are only removed in Commit 4, after the guard is live. |
| **Local `docker build`** (dev, offline, or not logged into GHCR) | developer choice | `docker build .` uses the pinned image (one ~30 MB pull, then cached). Offline / air-gapped: `make build-offline` → `docker build --build-arg CADDY_BUILDER_SRC=caddy-inline --build-arg CROWDSEC_BUILDER_SRC=crowdsec-inline .`. |

**Security non-regression:** the release/CVE-gate paths — `docker-build.yml` (amd64+arm64), `nightly-build.yml`, `security-pr.yml`, `supply-chain-pr.yml` — always use the default (`toolchain-prebuilt`, digest-pinned, **daily-`--no-cache --pull`-rebuilt-and-scanned**). Fork PRs use `caddy-inline`, which is byte-for-byte the same recipe (same `go get pkg@fixed` lines, same embeds-version assertions) — a fork build is *not weaker*, just slower and unpinned. A fork PR cannot merge without a maintainer re-running the trusted same-repo path (`security-pr.yml` already gates this via its `workflow_run` trust-boundary check at `:146`), at which point the real prebuilt+scanned image is exercised.

### 3.8 Security-guarantee analysis (CVE-2026-84304-class recurrence)

#### 3.8.1 The true current baseline (B2 — corrected)

Rev 1 understated this. Today, `--no-cache-filter caddy-builder,crowdsec-builder` forces a from-scratch rebuild of the two builder stages:

- on **`nightly-build.yml`** — `schedule: '0 9 * * *'`, i.e. **daily**, and it builds the *shipped* `nightly` multi-arch image (`nightly-build.yml:229-243`);
- on **every** `docker-build.yml` release build (push to `main`/`development`/`nightly`, every version tag);
- on **every** `security-pr.yml` / `supply-chain-pr.yml` CVE-gate run (per PR);
- on **every** `e2e-tests-split.yml` image build (per PR / per run).

So the effective current cadence at which the bundled Caddy/CrowdSec binaries are recompiled from source (re-running every `go get pkg@fixed`, re-resolving `go mod tidy`, re-pulling base images via the accompanying `--pull`) is **at least daily, and in practice several times a day on active days**. The weekly `security-scan-YYYYMMDD` image is a *scan* artifact, not the only rebuild.

#### 3.8.2 What actually changes, and why the new cadence is acceptable

After this change the forced-rebuild driver is the **daily** `schedule` on `toolchain-image.yml` (§3.4.1) plus per-PR rebuilds whenever a tracked pin moves. Refresh latency for the *shipped* image becomes: `daily toolchain rebuild` → `bot PR` → `human merge of bot PR` → next app build picks up the new digest.

| Property | Today | After |
|---|---|---|
| Bundled-binary recompile cadence (no pin moved) | daily (nightly) + per active PR | **daily** (toolchain `schedule`) |
| Latency from a new toolchain digest to it being in the shipped image | 0 (next nightly/release builds it directly) | **daily rebuild + bot-PR merge latency** (target: merge within 1 business day; the bot PR is `feat(security)`-labelled and shows in the same queue as a Renovate security bump) |
| Human step in the loop | none | **yes — a maintainer merges `bot/bump-toolchain-image`** |

The added human-merge step is the real trade. It is acceptable because: (a) the daily rebuild + Trivy gate still *detects* a problem on the same ~24 h cadence as today — only *shipping* the fix now waits on a PR merge; (b) the bot PR is small (two ARG lines), CI-verified, and lands in the security review queue the team already watches for Renovate; (c) an urgent case is a one-click `workflow_dispatch` + expedited merge (~30 min end to end, §6); (d) the alternative — auto-committing digest bumps to `development` with no review — is worse for a security-sensitive artifact. **The daily cadence (not weekly) is therefore committed scope**, precisely so the *detection* cadence matches today's; only the merge step is new.

#### 3.8.3 What the forced `--no-cache --pull` rebuild does and does NOT catch (B3 — corrected)

Rev 1 claimed the weekly rebuild's "fresh `go mod tidy` MVS → new binary" catches upstream fixes to **unpinned transitive** deps. **That claim is withdrawn — it is false:**

- `go mod tidy` / MVS is **deterministic**. It selects the *minimum* version satisfying the constraints in `go.mod`/`go.sum`. An upstream project publishing a patched `v1.2.4` does **not** cause MVS to move off `v1.2.3` unless something in the require graph raises the lower bound. "Latest patch" is not an MVS input.
- `docker buildx build --no-cache` invalidates *layer* cache. It does **not** clear the `RUN --mount=type=cache,target=/go/pkg/mod` BuildKit cache mount — the Go module cache persists across `--no-cache` builds. (`--pull` only refreshes `FROM` images.)

**What the daily `--no-cache --pull` toolchain rebuild genuinely catches:**

| Vector | Caught? | Mechanism |
|---|---|---|
| Upstream fix to a **pinned** dep (any §2.2 ARG, incl. the two new plugin pins, or a literal `go get x@vN` in the stage body, or the stage text itself) | ✅ per-PR | Renovate/manual bump → `toolchain-key.sh` changes → `verify-toolchain-pin` **fails the PR** until the toolchain is rebuilt and the digest synced |
| **Base-image** drift — new `golang:1.27.1-alpine` / `alpine@sha256:…` / plugin-source-image CVEs | ✅ daily | `--pull` re-resolves the `FROM` digests; with N4's digest-pinned golang base, a Renovate digest bump also trips the key |
| **Alpine package** drift in `toolchain-runtime` / final stage (`apk upgrade`) | ✅ daily (toolchain) + per-release (app `--pull`, kept) | fresh `apk` index on `--no-cache` |
| Trivy signature DB gaining a new match against an **already-shipped** bundled version | ✅ daily | Trivy runs against the toolchain digest every day; new CRITICAL/HIGH → red run + failure issue |
| Upstream security fix to a genuinely **unpinned transitive** Go dep, where nothing raises the MVS lower bound | ❌ — **same gap as today** | only closed by a human adding an explicit `go get dep@fixed` pin (the existing pattern — the stage already has ~40 such pins). Renovate's Go-module manager + the `caddy-major-monitor.yml` / dependency-review tooling surface these; this spec does not change that surface either way. |

**Net:** the recurrence guarantee for *pinned* deps is **strengthened** (a stale pin now hard-fails a PR instead of relying on a cache-key accident). The *unpinned-transitive* gap is **unchanged** — it exists identically today and is out of scope here; the plan explicitly does not claim to close it.

**Strengthening vs today:** the daily toolchain Trivy gate is **blocking** on `schedule`/`dispatch`/`workflow_call` (`exit-code 1`) — today's `security-weekly-rebuild.yml` has `continue-on-error: true` on its first Trivy step, so a known CRITICAL currently only produces a `::warning::`. After this change it produces a red run + a tracked GitHub issue, daily.

**Optional further hardening (follow-up, not committed):** a twice-daily `schedule` guarded by "rebuild only if no published tag for the current key OR last publish > 12 h" — halves detection latency for modest runner cost.

### 3.9 Timeout right-sizing (R1 side-effect)

| Job | File:line | Now | After | Rationale |
|---|---|---|---|---|
| `build-amd64` | `docker-build.yml:403`, `:441` | 15 / 15 | **20 / 20** | No compile on hot path; 20 gives headroom for a cold GHA cache miss on the *fast* stages + cache export + push. `docker-build.yml` never runs the inline fallback (release path, same-repo only), so 20 is safe. |
| `build-arm64` | `docker-build.yml:487`, `:527` | 25 / 25 | **keep 25** | QEMU still runs the final arm64 stage's `RUN` lines + `COPY` from the arm64 toolchain child; 25 stays comfortable. |
| `merge-and-publish` | `docker-build.yml:582` | 10 | keep 10 | unaffected |
| `security-pr` build | `security-pr.yml:32` | 20 | **keep 20** | **B6:** this job IS fork-reachable and runs the inline compile (~14 min) on a fork PR → 14 + checkout + Trivy + overhead would blow a 15-min cap. Keep 20. Update the `:32` comment to: "20m — warm same-repo build ~6–8 m; fork PRs compile the toolchain inline (~14 m + scan), which sets the floor." |
| `supply-chain-pr` build | `supply-chain-pr.yml:34` | 20 | **keep 20** | same reasoning; update comment `:34` identically. |
| `cerberus/crowdsec/waf/rate-limit-integration` | each `:29` | 20 | **keep 20** | fork-reachable + integration test work on top; 20 still right. Update the "first run … full cold build" comments to: "fork PRs build the toolchain inline; same-repo runs `COPY` it from the pinned image". |
| `e2e-tests-split.yml` build job | `:252` etc. | 60 | keep 60 | already generous; fork inline compile fits easily. |
| `toolchain-image.yml` | new | — | **45** | cold amd64+arm64 cross-compile of both stages + Trivy |
| `security-weekly-rebuild.yml` | `:35` | 60 | keep 60 (now mostly the `workflow_call` to toolchain-image) |

**B6 reconciliation, explicit:** §3.7 establishes that `security-pr.yml`, `supply-chain-pr.yml`, and the four `*-integration.yml` jobs run the ~14-minute inline compile on fork PRs. Therefore **no job reachable by a fork inline build has its timeout cut**. Only `build-amd64` (release path, same-repo-only, never inline) is raised 15→20. If the team later wants tighter same-repo feedback, the reduction can be made conditional: `timeout-minutes: ${{ github.event.pull_request.head.repo.full_name == github.repository && 15 || 20 }}` — noted as an option, not adopted now (keeps the YAML simpler and 20 min idle-capacity cost is negligible).

**Stale-comment fixes:** `docker-build.yml:381` ("amd64's fast native build") — reword: the Caddy/CrowdSec compile now lives in the prebuilt toolchain image, and the arm64 builder stages were always cross-compiled (never QEMU) regardless. Fix the dangling `docs/plans/current_spec.md §1.1` cross-reference in the same comment block (it points at the retired uptime spec) to cite this document. Grep `.github/**` for `no-cache-filter` / `xcaddy` / `10-14m` / `12-14 min` / `cold build` / `full cold build` and reconcile every comment (Commit 6).

### 3.10 Error handling / edge cases

| Scenario | Handling |
|---|---|
| Toolchain image pull fails mid app-build (GHCR outage) | app build fails fast with BuildKit's `failed to resolve source` — no silent fallback to a stale local layer. CI: `nick-fields/retry` already wraps `build-amd64`/`build-arm64` (3× / 10s). Document: maintainers can re-run or pass the inline build-args. |
| `regctl` not on runner | every workflow that runs `verify-toolchain-pin.sh` installs `regctl` first (`iarekylew00t/regctl-installer` or `docker run ghcr.io/regclient/regctl`). **B7:** on a same-repo run the script `exit 1`s if `regctl` or `GHCR_READ_TOKEN` is missing — it does **not** silently skip. Only a fork PR (`SAME_REPO=0`) degrades to tag-only comparison, with a `::warning::`. |
| Two toolchain builds race (dispatch + path-trigger on same commit) | `concurrency: toolchain-image-${{ github.ref }}`, `cancel-in-progress: false` → serialized; second is a cache hit / no-op. |
| Bot PR already open | `peter-evans/create-pull-request` updates the existing `bot/bump-toolchain-image` branch in place (same as GeoLite2 bot). |
| `toolchain-key.sh` awk stage-extraction breaks if a future edit removes the blank line between stages | script asserts each `extract_stage` returned ≥ 20 lines and contains `go build`; exits non-zero with a clear message otherwise. Unit-tested (§7). |
| Digest pinned but tag `:latest` moved (someone pushed manually) | guard compares against `:${KEY}` (content tag), never `:latest`; manual `:latest` pushes are cosmetic. |
| `CADDY_USE_CANDIDATE=1` experiment build | changes `toolchain-key.sh` output (ARG is in the hashed set) → distinct tag → distinct image; experiment is isolated, never collides with the mainline pin. |
| Renovate bumps `ghcr.io/wikid82/charon-toolchain` digest directly | harmless; guard still requires `TAG == key`, so a digest-only Renovate bump without a matching key change fails the guard and is closed in favor of the bot PR. Document in `renovate.json` a `packageRules` comment. |
| arm64 toolchain child missing (build published amd64-only by mistake) | app `build-arm64` fails at `FROM …@<listdigest>` with "no match for platform" — loud. `toolchain-image.yml` asserts `regctl manifest get` lists both platforms before pushing `:latest`. |

---

## 4. Implementation Plan

The phases map 1:1 onto the Commit Slicing Strategy (§12); this is the same plan viewed as work packages.

### Phase 1 — "Spec behavior": key/guard scripts + toolchain workflow + stage split (Commit 1)

- `scripts/lib/dockerfile-stage.sh` (shared `extract_stage`), `scripts/toolchain-key.sh`, `scripts/verify-toolchain-pin.sh`, with **bats unit tests** under `scripts/tests/` (house style: `scripts/*.sh` + a `bats` runner; add `shellcheck` + `bats` to the fast-lint set).
- No E2E/Playwright surface — this is CI/build infra. The executable "spec behavior" is: `toolchain-key.sh` is stable across a no-op Dockerfile reformat and changes when a tracked ARG / plugin pin / golang-base digest / stage line / `.trivyignore` changes; `verify-toolchain-pin.sh` is failure-closed on same-repo runs (B7).
- `.github/workflows/toolchain-image.yml` (daily `schedule` + `workflow_dispatch` + `pull_request` paths + `workflow_call`; build/publish + trivy-scan only).
- `Dockerfile`: rename stages, delete dead `crowdsec-fallback`, pin the two plugins + the golang base digest, add `toolchain-runtime` + temp aliases; **retarget the no-cache filters to `caddy-inline`/`crowdsec-inline`**.
- Manual `workflow_dispatch` first publish → capture `:<key>` + manifest-list digest; set GHCR package **Internal** (N8).

### Phase 2 — Consume the pinned image; wire the fallback selector (Commit 2)

- `Dockerfile`: `CHARON_TOOLCHAIN_*` ARGs + `toolchain-prebuilt` + `FROM ${…_SRC} AS caddy-builder`.
- Fork-detection build-args on every app-image build step + **new `builder-src` input on the `build-charon-image` composite** (§3.7, N10).
- `Makefile` `build-offline`.

### Phase 3 — Guardrails: freshness + app-side assertions (Commit 3)

- `verify-toolchain-pin` → required check in `quality-checks.yml` (with `regctl` + `GHCR_READ_TOKEN`).
- `sync-pin-on-pr` + `open-bump-pr` jobs in `toolchain-image.yml`.
- N5 final-stage `RUN` assertions; extend `docker-build.yml`'s post-build verification to check the toolchain `LABEL` key.

### Phase 4 — Remove the forced rebuilds; reroute the security scan (Commits 4–5)

- Delete every `--no-cache-filter` / `no-cache-filters` + the composite `no-cache-filters` input (Commit 4) — only after Phase 3's guard is live.
- Repurpose `security-weekly-rebuild.yml` → `workflow_call` into `toolchain-image.yml`; blocking Trivy on `schedule`/`dispatch`/`workflow_call`; caller `permissions:` gains `contents: write` + `pull-requests: write` (N6) (Commit 5).

### Phase 5 — Timeouts, comment sweep, docs (Commit 6)

- Timeout edits (§3.9 — only `build-amd64` 15→20; CVE-gate jobs stay 20 per B6), stale-comment reconciliation.
- `ARCHITECTURE.md`, `SECURITY.md`/`docs/security.md`, new `docs/ci/toolchain-image.md`, `CONTRIBUTING.md`, `renovate.json` (§9).

---

## 5. Acceptance Criteria (Definition of Done)

1. **No app-image build compiles Caddy/CrowdSec on the happy path.** A CI `build-amd64` run with a warm cache shows no `xcaddy`/`go build … caddy`/`xx-go build … crowdsec` step; total job < 8 min. Verified from the run log in the PR.
2. **`build-amd64` / integration jobs no longer time out** across 3 consecutive CI runs on the PR (main gate that this feature exists to fix).
3. **`verify-toolchain-pin` is a required check** and: (a) passes on `main` HEAD, (b) **fails** on a deliberate commit that bumps `CADDY_VERSION` (or a plugin pin) without rebuilding, (c) **fails** on a deliberate commit that hand-edits `CHARON_TOOLCHAIN_DIGEST` to a wrong-but-valid digest on a same-repo run (proves B7 failure-closed — not a tag-only check), (d) passes again after `sync-pin-on-pr` runs. Demonstrated with temporary commits that are then reverted.
4. **Multi-arch intact:** `docker buildx imagetools inspect ghcr.io/wikid82/charon-toolchain:<key>` lists `linux/amd64` + `linux/arm64`; the merged app image manifest still lists both; `docker run --rm --platform linux/arm64 <app-image> /usr/bin/caddy version` and `… cscli version` succeed (in `docker-build.yml`'s existing verification step). Confirm the arm64 leg still runs only its final-stage `RUN` under QEMU (N2 — it never compiled the builders).
5. **Fallback works:** a CI leg builds the app image with `--build-arg CADDY_BUILDER_SRC=caddy-inline --build-arg CROWDSEC_BUILDER_SRC=crowdsec-inline` and passes the in-`caddy-inline` embeds-version assertions; a simulated fork run stays under the 20-min job cap (B6).
6. **Security guarantee mechanized:** the daily `schedule` on `toolchain-image.yml` runs `--no-cache --pull` + a **blocking** Trivy CRITICAL/HIGH gate; a wired failure issue; a new digest opens `bot/bump-toolchain-image`. `security-weekly-rebuild.yml` routes through the same `workflow_call`. §3.8.3's "does NOT catch unpinned-transitive MVS drift" statement is reflected verbatim in `SECURITY.md` (no overclaim). Dry-run via `workflow_dispatch` on the PR branch.
7. **No `--no-cache-filter` / `no-cache-filters` string remains** under `.github/workflows` or `.github/actions` (grep clean; `docs/` history excepted). The composite action no longer exposes a `no-cache-filters` input.
8. **All existing CI green:** `docker-build.yml`, `nightly-build.yml` (dispatch), `security-pr.yml`, `supply-chain-pr.yml`, `e2e-tests-split.yml`, 4× integration workflows pass on the PR.
9. **Backend/frontend untouched:** `cd backend && go build ./... && go test ./...` and `cd frontend && npm run build && npm run type-check` unaffected (no diff there). GORM security scan **N/A** (no `backend/internal/models/**` change).
10. **`ARCHITECTURE.md` updated** (§9) and `docs/` build/CI docs updated; `docs-writer` pass done.
11. **Trivy on the final app image** (existing `merge-and-publish` step) shows **no new** CRITICAL/HIGH versus the pre-change baseline — the bundled binaries are the same recipe.
12. Lefthook / staticcheck / `make lint-fast` clean; shell scripts pass `shellcheck` (add to `lefthook` if not already).

---

## 6. Risks & Rollback

| Risk | Likelihood | Impact | Mitigation | Rollback |
|---|---|---|---|---|
| Toolchain image stale vs an urgent 0-day; bot-PR-merge latency too slow | Low | High | daily rebuild + Trivy **detects** on ~24 h cadence (unchanged from today); urgent path = `workflow_dispatch` + expedited merge (~30 min); follow-up twice-daily toggle (§3.8.3) | n/a — detection cadence matches today; only the merge step is new (§3.8.2) |
| `FROM ${ARG} AS name` selector unsupported on a pinned BuildKit | Low | Med | verified against BuildKit ≥ 0.11 (repo uses current `buildx`); `docker build --check` + `--print` in Commit 1/2 gates | drop the selector; make `caddy-inline`/`crowdsec-inline` the direct stage names and gate the prebuilt image behind an explicit per-workflow `--build-arg` |
| GHCR outage blocks all builds (new hard dependency) | Low | High | `nick-fields/retry` wraps the release builds; documented inline fallback; Docker Hub mirror is a follow-up | flip the selector build-args to `caddy-inline` fleet-wide via a one-line workflow edit |
| `toolchain-key.sh` false-negative (misses a security-relevant change) | Med | High | key hashes full stage **text** + all consumed ARGs (incl. the 2 plugin pins) + golang-base digest + `.trivyignore` + `SCHEMA_VERSION`; bats tests; the **daily** `--no-cache --pull` rebuild is the base-image/`apk` backstop even if the key never moves (it is NOT a backstop for unpinned-transitive MVS — §3.8.3) | bump `SCHEMA_VERSION` → global rebuild + re-pin |
| Bot PR churn (digest bump when nothing meaningful changed) | Low | Low | `open-bump-pr` fires only when the **manifest-list digest** actually changes; a no-op day (same base digests, same `apk` index) reproduces the same digest → no PR | close PR; tune to "digest changed AND (Trivy delta OR >7 d since last bump)" |
| Fork PRs slower (full inline compile) | High (every fork PR) | Low | expected; fork CI already runs long; CVE-gate job timeouts stay at 20 (B6); documented in `CONTRIBUTING.md` | none needed |
| Timeout bump to `build-amd64` masks a real slowdown | Low | Low | AC #1 asserts < 8 min actual; a run > 12 min is investigated | revert timeout to 15 |
| Human forgets to merge the bot PR for days | Med | Med | bot PR carries the `security` label → shows in the same queue as Renovate security bumps; `repo-health.yml`/stale-bot surfaces it; runbook says target ≤ 1 business day | expedite; or `workflow_dispatch` + merge |

**Whole-PR rollback:** revert the single merged commit. The `charon-toolchain` package stays in GHCR (harmless, unreferenced; `container-prune.yml` ages it out). `security-weekly-rebuild.yml` returns to building the throwaway scan image; the Dockerfile returns to inline `caddy-builder`/`crowdsec-builder` + `--no-cache-filter` — **security posture identical to today**. No data migration, no runtime change; app image content byte-identical (same recipe).

**Contingency:** if the selector-stage approach hits a BuildKit bug in one workflow only, that workflow can pin `--build-arg CADDY_BUILDER_SRC=caddy-inline` as a temporary per-workflow escape hatch while keeping the prebuilt default everywhere else — no revert of the whole feature.

---

## 7. Testing strategy (validate without a 14-min wait)

| What | How | Where |
|---|---|---|
| `toolchain-key.sh` determinism | shell/bats test: run twice → identical; reformat whitespace outside the stages → identical; change a `go get` line inside `caddy-inline` → differs; bump `CADDY_VERSION` default → differs; touch `.trivyignore` → differs | `scripts/tests/toolchain-key.bats`, runs in `quality-checks.yml` (< 5 s) |
| `verify-toolchain-pin.sh` | bats matrix: matching pin → exit 0; mismatched tag → exit 1 (actionable message); **same-repo run + missing `regctl`/token → exit 1** (B7 failure-closed, mocked); **same-repo run + GHCR digest ≠ pinned → exit 1**; fork run (`SAME_REPO=0`) + no registry access → exit 0 with `::warning::` | `scripts/tests/verify-toolchain-pin.bats` |
| Selector stage resolves both ways | `docker build --check` + `docker buildx build --target caddy-builder --print` (BuildKit dry-run, no compile) for both `CADDY_BUILDER_SRC` values | new `toolchain-image.yml` PR-path job, seconds |
| Cache behavior (the actual fix) | CI observation: run `build-amd64` twice on the PR; second run's log shows `CACHED` for every stage and **no** `xcaddy` / `xx-go build` compile lines; assert job wall-time < 8 min via a step that checks `$SECONDS` | PR CI, no local 14-min wait |
| Guard-live check (B5) | in Commit 1's gate: `docker buildx build --no-cache-filter caddy-inline` re-runs the `xcaddy build` step (not `CACHED`); with the old `--no-cache-filter caddy-builder` value on the renamed graph it would show `CACHED` | Commit 1 CI leg |
| Fallback correctness | one CI leg builds the app image with the inline build-args; the in-`caddy-inline` `go version -m /usr/bin/caddy | grep 'cel-go … v0.29'` / `grpc … v${GRPC_VERSION}` assertions (`Dockerfile:564`, `:569`) are the test — they already fail the build if the binary is wrong; plus the new N5 final-stage assertion | `toolchain-image.yml` PR-path matrix leg |
| Wrong-digest detection (N5) | build the app image against a deliberately old `CHARON_TOOLCHAIN_DIGEST` → N5 final-stage `RUN` fails ("missing expected caddy plugins" / bad `cscli version`) | Commit 3 CI leg |
| Multi-arch child selection | `docker buildx imagetools inspect` two-platform assertion in `toolchain-image.yml` (before pushing `:latest`); `docker run --platform linux/arm64 … caddy version` in `docker-build.yml`'s existing post-build verification | existing + new assertion |
| Daily rebuild + bot | `workflow_dispatch` `toolchain-image.yml` from the PR branch with `force_rebuild: true`; confirm it publishes, scans, and (if digest changes) opens a draft `bot/bump-toolchain-image` PR; blocking Trivy gate on the dispatch path | manual, once, during PR review |
| No regression in app-image Trivy | compare `merge-and-publish` Trivy JSON artifact on the PR vs a recent `main` run — diff must be empty for CRITICAL/HIGH | PR CI artifact |
| E2E | existing `e2e-tests-split.yml` runs unchanged against the built image; targeted local run per CLAUDE.md DoD only if a spec is touched (none is) | CI |

**Local dev validation (fast):** `scripts/toolchain-key.sh` + `bats scripts/tests/` (seconds); `docker buildx build --target caddy-builder --print` (no compile); pulling the published toolchain image and running `docker build .` end-to-end is a ~30 MB pull + fast stages only (~4–6 min), well under the old 14-min floor.

---

## 8. Component complexity estimate

| Component | Complexity | Notes |
|---|---|---|
| Dockerfile stage rename + delete dead `crowdsec-fallback` + pin 2 plugins + digest-pin golang base + selector + `toolchain-runtime` + N5 assertion | **M** | mostly mechanical; selector pattern needs `--check` validation; plugin/base pins need one-time version resolution; COPY paths chosen to keep final stage untouched |
| `toolchain-image.yml` | **L** | multi-arch build, GHCR push, Trivy+SARIF, `sync-pin-on-pr`, `open-bump-pr`, fork guards, `workflow_call`, daily `schedule` |
| `scripts/lib/dockerfile-stage.sh` + `toolchain-key.sh` + `verify-toolchain-pin.sh` (failure-closed) + bats | **M** | shared awk extraction; robust asserts; B7 same-repo/fork branching; token+regctl plumbing in CI |
| Retarget then strip `--no-cache-filter` across 6 workflows + composite action | **S–M** | Commit 1 retarget (value change) + Commit 4 removal + composite input deletion (public interface change) + comment rewrites |
| Repurpose `security-weekly-rebuild.yml` | **M** | swap build step for `workflow_call`; caller `permissions:` must grant `contents: write` + `pull-requests: write` (N6); keep Trivy plumbing; blocking gate |
| Fork-detection build-args in every build step **+ new `builder-src` input on the `build-charon-image` composite (public interface change, 4 integration callers)** | **M** | ~8 build steps across 6 workflows + composite input + per-caller `head.repo.full_name` expression (N10 — was S–M, raised to M) |
| Timeout + comment reconciliation | **S** | grep-driven sweep; only `build-amd64` actually changes value |
| `ARCHITECTURE.md` + docs + `docs/ci/toolchain-image.md` runbook | **S–M** | §9 list + new runbook incl. one-time package-visibility step |

---

## 9. `ARCHITECTURE.md` / documentation update list

| File | Section | Change |
|---|---|---|
| `ARCHITECTURE.md` | §"Deployment Architecture / Multi-Stage Dockerfile" (`:1082`) | replace the illustrative snippet's build-from-source framing; add a "Prebuilt toolchain image" subsection: what `charon-toolchain` contains, that Caddy/CrowdSec are compiled there (not in the app build), digest-pinned in `Dockerfile`, rebuilt **daily** `--no-cache --pull`, freshness-guarded, with the fork/offline inline fallback |
| `ARCHITECTURE.md` | §"Infrastructure" table (`:158`) | add row: **Bundled proxy toolchain** — `ghcr.io/wikid82/charon-toolchain` — multi-arch prebuilt Caddy + CrowdSec, daily-rebuilt + Trivy-gated |
| `ARCHITECTURE.md` | §"Directory Structure" (`:286`) | note `.github/workflows/toolchain-image.yml`, `scripts/toolchain-key.sh`, `scripts/verify-toolchain-pin.sh`, `scripts/lib/dockerfile-stage.sh`; note removal of the `crowdsec-fallback` Dockerfile stage |
| `ARCHITECTURE.md` | §"Security Architecture / Layer 2: CrowdSec Integration" (`:780`) and the defense-in-depth intro (`:750`) | note the CrowdSec agent + bouncer-enabled Caddy are supply-chain-hardened via the scanned, digest-pinned toolchain image; recurrence guarantee = **daily** `--no-cache --pull` toolchain rebuild + blocking Trivy gate + bot PR (+ per-PR `verify-toolchain-pin` for pinned-dep bumps). Be precise per §3.8.3: it does not close the unpinned-transitive-MVS gap (unchanged from today) |
| `ARCHITECTURE.md` | §"Development Workflow / Local Development Setup" (`:1204`) | add the offline build note (`--build-arg …_SRC=…-inline`) and `make build-offline` |
| `CONTRIBUTING.md` | build section | fork PRs compile the toolchain from source (slower CI); maintainers re-dispatch for the prebuilt path |
| `docs/features.md` | — | no user-facing capability change → **no edit** (per CLAUDE.md keep brief) |
| `docs/security.md` / `SECURITY.md` | supply-chain / build integrity paragraph | describe the toolchain image, its **daily** `--no-cache --pull` rebuild + blocking Trivy gate, the digest pin, and the `verify-toolchain-pin` freshness guard as the mechanism that keeps bundled binaries patched; state the §3.8.3 scope precisely (pinned-dep + base-image drift covered; unpinned-transitive MVS gap unchanged from today) — do not overclaim |
| new `docs/ci/toolchain-image.md` | — | operator/maintainer runbook: how the key works, how to force a rebuild, how to respond to the bot PR / failure issue, how to roll back |
| `Makefile` | — | `build-offline` target |
| `renovate.json` | — | comment on the `charon-toolchain` datasource entry: digest bumps are owned by the bot workflow, not Renovate |

---

## 10. API / schema impact

**None.** No REST endpoint, no GORM model, no migration, no `internal/**` code, no frontend, no DB. This is entirely CI/build-graph and repo tooling. `routes.go` AutoMigrate untouched.

---

## 11. Out-of-scope / follow-ups

- Mirror `charon-toolchain` to Docker Hub for GHCR-outage resilience.
- Twice-daily toolchain freshness trigger (§3.8.3 hardening toggle).
- Conditional `timeout-minutes` expression on `security-pr` / `supply-chain-pr` to give same-repo runs a tighter 15-min budget while forks keep 20 (§3.9 B6) — deferred to keep the YAML simple.
- Fold `gosu-builder` / `backend-builder` into the toolchain image too (they are already fast; low value).
- Cosign-sign the toolchain image and verify the signature in the app build `FROM` (needs BuildKit attestation verification; separate spec).
- **N11 (confirmation, no action):** `orthrus-build.yml` builds `./agent/Dockerfile` — a **different** image (the Orthrus agent), with its own `cache-from/to type=gha` and no `caddy-builder`/`crowdsec-builder` stages. Verified out of scope; this spec makes no change to it.

---

## 12. Commit Slicing Strategy

**Decision:** one feature = **one PR** targeting `development`, sliced into 6 ordered logical commits. Each commit builds and passes its own gate; the PR merges only when the full DoD (§5) passes. Not split across multiple PRs.

The `weekly-nightly-promotion.yml` "merge commit only" rule is **not** engaged — this PR follows the normal `development` flow and touches no promotion machinery.

**B5 — the CVE-recurrence guard is never inert.** The old plan had a window (Commit 1→3) where the stages were renamed to `caddy-inline`/`crowdsec-inline` but the workflows still said `--no-cache-filter caddy-builder` — a filter on an *alias* node does not invalidate the `RUN` layers that moved into `caddy-inline`, so the guard was silently dead. Fixed below: **Commit 1 retargets every `--no-cache-filter` / `no-cache-filters` from `caddy-builder,crowdsec-builder` to `caddy-inline,crowdsec-inline` in the same commit as the rename**, and the freshness guard (`verify-toolchain-pin`, Commit 3) is in place **before** those filters are removed (Commit 4). Every commit's gate below explicitly checks that *some* live mechanism forces a from-source recompile when a pin/recipe changes.

### Commit 1 — `feat(security): add toolchain-image workflow, key tooling, split builder stages`

- **Scope:** the toolchain build/publish workflow + key/guard scripts; Dockerfile stage split; **retarget the no-cache filters to the RUN-bearing stage names**; first manual publish; make the new GHCR package internal.
- **Files:**
  - `scripts/lib/dockerfile-stage.sh`, `scripts/toolchain-key.sh`, `scripts/verify-toolchain-pin.sh` (new)
  - `scripts/tests/toolchain-key.bats` (+ wire into `quality-checks.yml` as a **non-blocking** job for now)
  - `.github/workflows/toolchain-image.yml` (new — `schedule` daily + `workflow_dispatch` + `pull_request` paths + `workflow_call`; **build/publish + trivy-scan jobs only**; `sync-pin-on-pr` / `open-bump-pr` land in Commit 3)
  - `Dockerfile` — rename `caddy-builder→caddy-inline`, `crowdsec-builder→crowdsec-inline`; **delete the dead `crowdsec-fallback` stage** (`:713-748`) and its now-dead `CROWDSEC_RELEASE_SHA256` ARG (N1); **pin the two xcaddy plugins** `CADDY_GEOIP2_VERSION` / `CADDY_RATELIMIT_VERSION` (B4); **digest-pin the `golang:${GO_VERSION}-alpine` base** of both inline stages (N4); add `toolchain-runtime` assembly stage; add temporary aliases `FROM caddy-inline AS caddy-builder` / `FROM crowdsec-inline AS crowdsec-builder` so the app build is unchanged this commit.
  - **All six no-cache-filter sites + composite action** (§3.6) — change the value `caddy-builder,crowdsec-builder` → `caddy-inline,crowdsec-inline` (do **not** remove yet).
- **Dependencies:** none.
- **Bootstrap / N8:** after CI publishes the first image via `workflow_dispatch`, in GHCR set the `charon-toolchain` package visibility to **Internal** (or link it to the repo and grant the repo `packages: read`) so cross-workflow `FROM ghcr.io/…/charon-toolchain@digest` works with the default `GITHUB_TOKEN`. Document this one-time manual step in `docs/ci/toolchain-image.md` (Commit 6) and in the PR description.
- **Validation gate:**
  1. `bats scripts/tests/` green; `shellcheck scripts/*.sh scripts/lib/*.sh` clean.
  2. `scripts/toolchain-key.sh` is stable across a whitespace-only reformat outside the two stages, and **changes** when (a) a `go get` line inside `caddy-inline` is edited, (b) `CADDY_VERSION` / `CADDY_GEOIP2_VERSION` default is bumped, (c) the golang base digest changes, (d) `.trivyignore` changes.
  3. `workflow_dispatch` toolchain-image.yml on the branch → publishes `ghcr.io/wikid82/charon-toolchain:caddy-crowdsec-<key>`; `docker buildx imagetools inspect` shows **both** `linux/amd64` and `linux/arm64`. Record `:<key>` + manifest-list digest for Commit 2.
  4. **Guard-live check:** on a scratch build, `docker buildx build --no-cache-filter caddy-inline …` shows the `xcaddy build` step running (not `CACHED`); with the old `--no-cache-filter caddy-builder` value it would show `CACHED` — confirm the retarget is what keeps the guard effective.

### Commit 2 — `feat(security): build app image from the pinned toolchain image`

- **Scope:** default path consumes the prebuilt image by digest; inline stages become the selectable fallback; fork detection wired.
- **Files:**
  - `Dockerfile` — add `CHARON_TOOLCHAIN_IMAGE/TAG/DIGEST` ARGs (values from Commit 1's publish), `CADDY_BUILDER_SRC`/`CROWDSEC_BUILDER_SRC` selector ARGs, `toolchain-prebuilt` stage; replace the temp aliases with `FROM ${CADDY_BUILDER_SRC} AS caddy-builder` / `FROM ${CROWDSEC_BUILDER_SRC} AS crowdsec-builder`.
  - **Every app-image build step** in `docker-build.yml`, `nightly-build.yml`, `security-pr.yml`, `supply-chain-pr.yml`, `e2e-tests-split.yml`, and the **`build-charon-image` composite action** (new `builder-src` input, default `toolchain-prebuilt`, with the `head.repo.full_name` expression in each caller) — pass `--build-arg CADDY_BUILDER_SRC=… --build-arg CROWDSEC_BUILDER_SRC=…` (`toolchain-prebuilt` same-repo, `caddy-inline`/`crowdsec-inline` on forks).
  - `Makefile` — `build-offline` target.
- **Dependencies:** Commit 1.
- **Note on the guard in this window:** default builds no longer run `caddy-inline` at all, so the retargeted `--no-cache-filter caddy-inline` is a no-op there — **intended**: the only path that still compiles is the fork/inline path, and the filter remains live *there*. The pin↔digest binding on the default path is enforced by Commit 3's freshness guard, added before any filter is removed (Commit 4).
- **Validation gate:** `docker build --check`; `docker build .` (default) → pulls the image, **no `xcaddy`/`xx-go build` in the log**, image boots, final-stage N5 assertions pass, `caddy version` + `cscli version` OK; `make build-offline` (inline) → compiles and passes the in-`caddy-inline` embeds-version assertions **and** still honours `--no-cache-filter caddy-inline`; `docker buildx build --target caddy-builder --print` resolves for both selector values; simulated fork run (push from a fork or manual expression override) uses the inline path and stays under the 20-min job cap (B6).

### Commit 3 — `feat(security): enforce toolchain pin freshness + app-side embed assertions`

- **Scope:** `verify-toolchain-pin` becomes a **required** check; `sync-pin-on-pr` + `open-bump-pr` jobs added to `toolchain-image.yml`; N5 final-stage assertions; extend `docker-build.yml`'s existing post-build CVE-verification step to also check the toolchain `LABEL` key.
- **Files:** `.github/workflows/quality-checks.yml` (required `verify-toolchain-pin` job, with `regctl` install + `GHCR_READ_TOKEN`), `.github/workflows/toolchain-image.yml` (add `sync-pin-on-pr`, `open-bump-pr`), `Dockerfile` (N5 `RUN` assertion after the `COPY --from` lines), `docker-build.yml` (extend verification step), `renovate.json` (comment: toolchain digest is bot-owned, N7).
- **Dependencies:** Commits 1–2 (a real pin must exist to guard).
- **Validation gate:**
  1. Temp commit bumping `CADDY_VERSION` (no rebuild) → `verify-toolchain-pin` **fails** with the actionable message; the `toolchain-image.yml` path trigger rebuilds and `sync-pin-on-pr` pushes the `TAG`/`DIGEST` bump onto the branch → check green → revert temp commit.
  2. Temp commit hand-editing `CHARON_TOOLCHAIN_DIGEST` to a valid-but-wrong digest → `verify-toolchain-pin` **fails** on the same-repo digest-mismatch branch (proves B7 failure-closed: it is not a tag-only check).
  3. `open-bump-pr` runs only on `schedule`/`workflow_dispatch`/`workflow_call`, never `pull_request` (assert via a dry `workflow_dispatch`).
  4. Build an app image against a deliberately wrong (old) toolchain digest → the N5 final-stage assertion fails the build (proves a bad pin is caught even if `verify-toolchain-pin` were bypassed).

### Commit 4 — `perf(ci): drop the forced from-source rebuilds; rely on the pinned image + guard`

- **Scope:** remove every `--no-cache-filter` / `no-cache-filters` and the composite `no-cache-filters` input — now safe because (a) the default path never compiles, (b) `verify-toolchain-pin` enforces pin↔digest freshness per PR, (c) the daily toolchain rebuild + Trivy gate covers base-image drift, (d) the N5 assertion catches a wrong digest.
- **Files:** `docker-build.yml` (`:463-464`, `:549-550`), `security-pr.yml` (`:157-164` block), `supply-chain-pr.yml` (`:252-261` block), `e2e-tests-split.yml` (`:224`), `nightly-build.yml` (`:243`), `.github/actions/build-charon-image/action.yml` (delete the `no-cache-filters` input decl `:11-33` + passthrough `:52`; rewrite `description`).
- **Dependencies:** Commit 3 (guard must be live *before* the filters go).
- **Validation gate:** `grep -rn "no-cache-filter" .github/workflows .github/actions` → empty (comments/docs excluded); `docker-build.yml build-amd64` run twice on the branch → second run every stage `CACHED`, wall-time **< 8 min**, zero compile lines; all 8 build-consuming workflows green; a bump-a-pin temp commit still fails `verify-toolchain-pin` (guard still live via the freshness mechanism, not the deleted filter).

### Commit 5 — `feat(security): route the security rebuild through the toolchain image`

- **Scope:** `security-weekly-rebuild.yml` `workflow_call`s `toolchain-image.yml` instead of building a throwaway app image; blocking Trivy on `schedule`/`dispatch`/`workflow_call`; caller grants all perms the bot job needs (N6).
- **Files:** `.github/workflows/security-weekly-rebuild.yml` (swap build step; `permissions:` add `contents: write` + `pull-requests: write` at job level; keep Trivy table/SARIF/JSON/`::warning::`; rename `TRIVY_SARIF_CATEGORY` value to `…:trivy-toolchain`).
- **Dependencies:** Commits 1, 3.
- **Validation gate:** `workflow_dispatch` on the branch → toolchain rebuilds `--no-cache --pull`; Trivy runs; SARIF uploads under the stable category; **no** bot PR when the digest is unchanged; temporarily drop a known-ignored item from `.trivyignore` → `schedule`-path Trivy step is **red** and the failure issue is created → restore `.trivyignore`. Confirm the daily `schedule` on `toolchain-image.yml` (added Commit 1) now also produces a bot PR path via `open-bump-pr` (Commit 3) when the digest moves.

### Commit 6 — `docs(ci): document the toolchain image; right-size timeouts; sweep stale comments`

- **Scope:** timeout edits (§3.9), stale-comment sweep, all `ARCHITECTURE.md` / docs updates (§9).
- **Files:** `docker-build.yml` (`build-amd64` timeout `:403`/`:441` 15→20; comment `:381` + dangling `§1.1` cross-ref), `security-pr.yml` (`:32` comment only — timeout **stays 20**, B6), `supply-chain-pr.yml` (`:34` comment only — stays 20), `*-integration.yml` (`:29` comments), `ARCHITECTURE.md` (§9 rows), `SECURITY.md` / `docs/security.md`, `docs/ci/toolchain-image.md` (new runbook — incl. the N8 one-time package-visibility step), `CONTRIBUTING.md`, `renovate.json` comment, `Makefile` (if not in C2).
- **Dependencies:** Commits 1–5.
- **Validation gate:** `grep -rn "xcaddy\|no-cache-filter\|cold build\|full cold build\|10-14m\|12-14 min" .github/` reconciled; markdown lint; `docs-writer` review; full CI green; DoD §5 all boxes checked.

### PR-level rollback / contingency

- **Rollback:** revert the single merged commit. The `charon-toolchain` package stays in GHCR unreferenced (`container-prune.yml` ages it out). `security-weekly-rebuild.yml` reverts to its prior behavior. The Dockerfile reverts to inline `caddy-builder`/`crowdsec-builder` with `--no-cache-filter` — **identical security posture to today**. Zero runtime/app-image content change (same recipe), so nothing to migrate or re-release.
- **Contingency (partial):**
  - Freshness guard misbehaves post-merge → make `verify-toolchain-pin` non-required (repo setting); the daily `--no-cache --pull` toolchain rebuild + Trivy gate + N5 assertion still protect the guarantee.
  - Selector stage breaks one workflow → set that workflow's `--build-arg CADDY_BUILDER_SRC=caddy-inline` as a temporary escape hatch (its `--no-cache-filter caddy-inline` was removed in Commit 4 but can be re-added to that one workflow) — no full revert.
  - GHCR unavailable for a release → the release build fails fast; run it again, or fleet-flip the selector build-args to `caddy-inline` via a one-line workflow edit.
- **Forward-fix preferred over revert** for anything touching the security guarantee (CLAUDE.md: long-term fix over quick patch).

---

## 13. Handoff

On approval: route to **supervisor** for plan review; iterate here until approved; then present to the user for explicit go-ahead before implementation. Implementation is CI/build-only → delegate commit-by-commit primarily to **devops** (with **docs-writer** for Commit 6), each commit gated as above, then **supervisor** re-review, then **qa-security** last against `SECURITY.md` + DoD.
