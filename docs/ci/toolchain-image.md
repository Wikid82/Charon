# Bundled proxy toolchain image — operator runbook

`ghcr.io/wikid82/charon-toolchain` is a multi-arch prebuilt image holding the
custom **Caddy v2** binary (built with `xcaddy` + in-place transitive-dependency
security patches) and the custom **CrowdSec** agent (`crowdsec` + `cscli`). The
app `Dockerfile` `COPY --from`s these instead of recompiling them (~14 min) on
every CI image build.

## How it fits together

| Piece | Role |
|---|---|
| `Dockerfile` `caddy-inline` / `crowdsec-inline` stages | The single source of truth for the build recipe. Compiled only by the toolchain workflow and the fork/offline fallback. |
| `Dockerfile` `toolchain-runtime` stage | `docker buildx build --target toolchain-runtime` → the publishable image. |
| `Dockerfile` `ARG CHARON_TOOLCHAIN_TAG` / `CHARON_TOOLCHAIN_DIGEST` | The pin. `DIGEST` is the arch-independent manifest-list (OCI index) digest. Bot-owned — do **not** hand-edit. |
| `Dockerfile` `ARG CADDY_BUILDER_SRC` / `CROWDSEC_BUILDER_SRC` | Selector: `toolchain-prebuilt` (default) or `caddy-inline` / `crowdsec-inline`. |
| `scripts/toolchain-key.sh` | Prints `caddy-crowdsec-<16hex>` — a content hash of the two inline stage bodies + every consumed version ARG (incl. the two pinned xcaddy plugins) + the digest-pinned `golang`/`xx` bases + `.trivyignore` + a `SCHEMA_VERSION`. |
| `scripts/verify-toolchain-pin.sh` | Required PR check. Failure-closed on trusted same-repo runs: needs `regctl` + `GHCR_READ_TOKEN`, resolves `:$KEY` in GHCR, and asserts the pinned digest matches. A fork PR degrades to a tag-only check with a `::warning::`. |
| `scripts/lib/dockerfile-stage.sh` | Shared `extract_stage` used by both scripts. |
| `.github/workflows/toolchain-image.yml` | Builds / publishes / scans. |
| `.github/workflows/security-weekly-rebuild.yml` | `workflow_call`s the above for the Tuesday full rebuild + MEDIUM/LOW JSON report. |

## Determinism

The build sets `--provenance=false --sbom=false`, a **fixed**
`SOURCE_DATE_EPOCH` (`1700000000`), and `rewrite-timestamp=true`. An unchanged
recipe (same `toolchain-key.sh` output) therefore always reproduces an
**identical** manifest-list digest. This is what stops `sync-pin-on-pr` from
looping. The toolchain image is an internal build *input* — the app image's own
provenance/SBOM attestations (in `docker-build.yml`) are separate and unaffected.

## Triggers

| Event | Behaviour |
|---|---|
| `schedule` (daily 06:00 UTC) | `--no-cache --pull` deterministic rebuild + blocking Trivy CRITICAL/HIGH gate + `:trivy-toolchain` SARIF. If the digest moved → opens `bot/bump-toolchain-image` (base `development`). |
| `workflow_dispatch` (`force_rebuild=true` default) | Same as `schedule`. Use this for an urgent out-of-band refresh. |
| `pull_request` touching a tracked input | **Skip-if-already-published:** if `:$KEY` already exists in GHCR, reuse that digest and do not rebuild. Same-repo PRs that genuinely moved a pin get the two-line `TAG`/`DIGEST` bump pushed onto the PR branch by `sync-pin-on-pr` (then re-run the `verify-toolchain-pin` check — `GITHUB_TOKEN` pushes don't auto-retrigger). Forks build `cacheonly` (recipe-compile validation only). |
| `workflow_call` (from `security-weekly-rebuild.yml`) | Forced deterministic rebuild + full report. |

## Common tasks

### Force an immediate refresh (e.g. an urgent upstream fix in a pinned dep)

1. Bump the relevant `ARG` in the `Dockerfile` (this changes `toolchain-key.sh`).
2. `gh workflow run toolchain-image.yml -f force_rebuild=true` — publishes
   `:<newkey>` and (off the PR path) opens `bot/bump-toolchain-image`.
3. Merge the bot PR (or, on your own PR, let `sync-pin-on-pr` bump the pin and
   re-run the freshness check).

### Respond to the daily-rebuild failure issue (`🚨 Toolchain image rebuild ... failed`)

1. Open the linked run. The blocking Trivy CRITICAL/HIGH gate or a compile
   failure is the usual cause.
2. For a new CRITICAL/HIGH in a bundled binary: add an explicit
   `go get <dep>@<fixed>` pin in the relevant inline stage (the stages already
   carry ~40), or a justified `.trivyignore` entry with an `exp:` review date.
3. `workflow_dispatch` the workflow again; merge the resulting bot PR.

### Respond to a `verify-toolchain-pin` PR failure

The message tells you the recomputed key vs the pinned tag. Either:
- your PR legitimately changed a tracked input → `workflow_dispatch`
  `toolchain-image.yml` (or push and let `sync-pin-on-pr` handle it), then bump
  `CHARON_TOOLCHAIN_TAG` / `CHARON_TOOLCHAIN_DIGEST`; or
- the pin was hand-edited / is stale → restore it to the bot-owned value.

### Roll back the whole feature

Revert the single merged commit. The `charon-toolchain` package stays in GHCR
unreferenced (`container-prune.yml` ages it out). The `Dockerfile` returns to
inline `caddy-builder` / `crowdsec-builder` with `--no-cache-filter`;
`security-weekly-rebuild.yml` returns to its prior behaviour. Security posture is
identical to before, and the app image content is byte-identical (same recipe).

## One-time bootstrap notes

- The GHCR package `charon-toolchain` is created on the first publish and is
  **private**. GHCR auto-links packages pushed by Actions to the source repo, so
  same-repo workflows pull it with `GITHUB_TOKEN` + `permissions: packages: read`
  + an explicit `docker login ghcr.io`. A maintainer may optionally flip it to
  public — not required.
- `Dockerfile:` `COPY scripts/ /app/scripts/` copies the whole `scripts/`
  directory into the runtime image. The build-only helpers this feature adds —
  `scripts/toolchain-key.sh`, `scripts/verify-toolchain-pin.sh`,
  `scripts/lib/dockerfile-stage.sh`, and `scripts/tests/` — are **excluded from
  the image context via `.dockerignore`** (they run from a plain checkout in
  `toolchain-image.yml` / `quality-checks.yml`, never from inside a container).
  No `.gitignore` change (source files, must be committed); no `.codecov.yml`
  change (shell/bats/YAML carry no Go/TS coverage).
