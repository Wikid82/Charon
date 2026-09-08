# QA & Security Report — Prebuilt Caddy/CrowdSec Toolchain Image

**PR**: #1300 — `feat(ci): prebuilt Caddy/CrowdSec toolchain image to fix Docker-build timeouts`
**Branch**: `feat/prebuilt-toolchain-image` → base `main` (draft)
**Branch tip audited**: `22e9c722` (working tree clean, rebased on `origin/main` `cc65e634`)
**Reviewed by**: qa-security agent (final pipeline pass)
**Date**: 2026-09-08
**Spec**: `docs/plans/current_spec.md` (Rev 2 + §3.4.3 "Rev 2.1")

---

## Verdict: PASS WITH FOLLOW-UPS

Clear to bring PR #1300 out of draft. No blocking security or QA issues. Four
non-blocking follow-ups and two residual supply-chain risks the merger should
accept knowingly (enumerated at the end).

The change is CI/build-infrastructure only — no Go or TypeScript application code,
no `backend/internal/models/**`, no GORM queries, no migrations, no frontend
surface. The new executable code is three shell scripts covered by a 17-test bats
suite. All CI checks on the tip are green.

---

## 1. Build-integrity / CVE-recurrence guarantee — VERIFIED

`--no-cache-filter caddy-inline,crowdsec-inline` (the CVE-2026-84304 recurrence
guard that forced from-source rebuilds on every CVE-gate PR) is removed. Every
compensating link claimed in the brief exists and is wired:

| Link | Where | Enforced? |
|---|---|---|
| (a) Content-hash key | `scripts/toolchain-key.sh` — SHA-256 over both inline stage bodies + 16 consumed version ARGs + the two pinned xcaddy plugins + `tonistiigi/xx` pin + digest-pinned `golang:*-alpine` bases + `sha256(.trivyignore)` + `SCHEMA_VERSION=2` | Yes — 10 bats tests assert determinism + per-input sensitivity + fail-loud on broken extraction |
| (b) Daily forced rebuild | `toolchain-image.yml` `on.schedule: '0 6 * * *'`; plan step sets `no_cache="--no-cache --pull"` for `schedule`/`force_rebuild`; `--target toolchain-runtime` recompiles `caddy-inline` + `crowdsec-inline` from source | Yes |
| (c) `verify-toolchain-pin.sh` per-PR check | `quality-checks.yml` job `verify-toolchain-pin` (installs regctl, maps trust env, runs the script) | Yes — passing on the tip; failure-closed on same-repo (see §2). **Branch-protection required-status enrolment is a merger check — see follow-up F1.** |
| (d) LABEL ↔ recipe-key check | `docker-build.yml` step "Verify pinned toolchain image matches the recipe (N5)" — `docker pull @PIN_DIGEST`, reads `io.charon.toolchain.key` LABEL, compares to freshly recomputed `toolchain-key.sh` | Yes |
| (e) Blocking weekly Trivy CRITICAL/HIGH | `toolchain-image.yml` `trivy-scan` job: `severity: CRITICAL,HIGH`, `exit-code: '1'`, `continue-on-error: ${{ github.event_name == 'pull_request' }}`. `security-weekly-rebuild.yml` reaches it via `workflow_call`, where `github.event_name` resolves to the caller's `schedule`/`workflow_dispatch` → `continue-on-error:false` → blocking | Yes |

**Additional independent link** not in the brief: `docker-build.yml` `build-amd64`
/ `build-arm64` and `nightly-build.yml` pull the toolchain by **immutable
`@sha256:` digest** (`FROM ${CHARON_TOOLCHAIN_IMAGE}@${CHARON_TOOLCHAIN_DIGEST}`),
then the merged app image is Syft-SBOM'd, `actions/attest`-attested, Trivy- and
Grype-scanned, and Cosign-signed — so any poisoned content still has to survive
the app-image scan gates.

**Doc-overclaim check — PASS.** Both `SECURITY.md` ("Build Integrity — Bundled
Caddy / CrowdSec Toolchain") and `ARCHITECTURE.md` ("Supply-chain hardening"
callout) retain the caveat verbatim and accurately:

> "…it does **not** close the pre-existing gap where an upstream security fix to a
> genuinely *unpinned* transitive Go dependency is not picked up because nothing
> raises the MVS lower bound — that is unchanged, and is closed only by a human
> adding an explicit `go get <dep>@<fixed>` pin (the recipe already carries ~40)."

Both files scope the guarantee to "pinned-dependency drift and base-image drift"
only. No overclaim. `docs/ci/toolchain-image.md` "Roll back the whole feature"
correctly states security posture and app-image content are byte-identical to the
pre-PR inline path.

---

## 2. `verify-toolchain-pin.sh` robustness — FAILURE-CLOSED, no bypass found

**Trust classification** (`SAME_REPO`): defaults to `1` (trusted / failure-closed)
and only degrades to `0` (tag-only + `::warning::`) when
`GITHUB_EVENT_NAME == pull_request` **and**
`GITHUB_EVENT_PULL_REQUEST_HEAD_REPO_FULL_NAME != GITHUB_REPOSITORY`. Unknown /
unset → trusted. This is the safe polarity: the only way to *reach* the degraded
path is to be a genuine fork PR (whose token cannot read the private package
anyway); anything ambiguous fails closed.

**Bypass attempts:**

- **Env-var spoofing of `GITHUB_EVENT_PULL_REQUEST_HEAD_REPO_FULL_NAME`** — the
  two env vars are mapped in each workflow from trusted GitHub contexts
  (`${{ github.event.pull_request.head.repo.full_name }}`, `${{ github.repository }}`),
  not from anything a fork PR author controls. A fork PR cannot alter the base
  workflow that runs. A same-repo branch *could* edit `quality-checks.yml` to
  mis-map them, but that requires write access (already a trusted actor) and is
  visible in the PR diff. Not a new weakness.
- **Fork → convince script it's same-repo** — would only make it *stricter*
  (failure-closed digest check); the fork runner has no `GITHUB_TOKEN` with
  `packages:read` on the base repo, so it fails closed. No trust gained.
- **Same-repo → convince script it's a fork** — needs `head.repo.full_name !=
  repository`, impossible for a real same-repo PR without editing the workflow
  (trusted-actor, diff-visible).
- **Hand-edited `CHARON_TOOLCHAIN_DIGEST` that still passes** — on the trusted
  path the script resolves `:$KEY` via `regctl image digest` and requires
  `REMOTE_DIGEST == PINNED_DIGEST`. The only digest that passes is the one GHCR
  actually serves for that content-addressed tag. Defence-in-depth: `docker-build.yml`
  LABEL check rejects a digest that points at a *different-recipe* toolchain
  image.
- **TOCTOU between `imagetools inspect` / `regctl image digest` and the app
  build's `FROM …@digest`** — not exploitable for injection. The app build
  consumes an immutable `@sha256:` reference; re-tagging `:$KEY` afterwards cannot
  change what `@digest` resolves to. Worst case is a spurious check failure
  (false positive), never a silent poisoned pull.

**`bats scripts/tests/` result: 17/17 PASS** (local, `Bats 1.13.0`; also green in
CI job "Toolchain key / freshness-guard scripts (bats)").

Failure modes **actually asserted** by `verify-toolchain-pin.bats`:

| Assertion | Covered |
|---|---|
| fork PR + matching tag → `exit 0` + `::warning::Fork PR` | ✅ |
| mismatched tag (any trust level) → `exit 1`, actionable message | ✅ |
| same-repo `push` + `regctl` absent → `exit 1` (failure-closed) | ✅ |
| same-repo `push` + `GHCR_READ_TOKEN` unset → `exit 1` (failure-closed) | ✅ |
| same-repo PR + GHCR digest ≠ pinned digest → `exit 1` ("hand-edited or stale") | ✅ |
| same-repo PR + GHCR digest == pinned digest → `exit 0` ("verified (same-repo)") | ✅ |
| `workflow_dispatch` treated as trusted same-repo (fails closed on missing regctl) | ✅ |

Failure-closed branches present in the script but **not** directly asserted (see
follow-up F2):

- `PINNED_DIGEST` empty on a same-repo run → `exit 1`.
- `:$KEY` present but `regctl image digest` returns non-zero (unresolvable in
  GHCR) → `exit 1` ("does not resolve in GHCR"). The bats `regctl` stub always
  succeeds, so this specific exit path is uncovered.

`toolchain-key.bats` (10 tests) covers determinism, whitespace-stability of edits
*outside* the two inline stages, and sensitivity to: a `go get` line inside
`caddy-inline`, `CADDY_VERSION`, `CADDY_GEOIP2_VERSION` (B4 plugin pin), the
digest-pinned `golang` base (N4), and `.trivyignore`; plus two fail-loud cases
(stage removed, stage truncated to a stub). Not asserted: sensitivity to a
`tonistiigi/xx` pin move, an `ALPINE_IMAGE` move, or a `SCHEMA_VERSION` bump —
all three *are* in the hashed input set; the gap is test-only (F2).

---

## 3. Determinism fix (§3.4.3 Rev 2.1) — does NOT weaken app-image posture

`toolchain-image.yml` builds with `--provenance=false --sbom=false`, fixed
`SOURCE_DATE_EPOCH=1700000000`, `--output type=image,push=true,rewrite-timestamp=true`.

- **App-image supply-chain posture is unaffected.** Verified: `docker-build.yml`
  `merge-and-publish` generates the app image's own SBOM (`anchore/sbom-action`
  syft `v1.51.1`, with a pinned-syft fallback), attests it (`actions/attest`
  `v4.2.2`), and Cosign-signs the merged digest — all against the final `charon`
  app-image digest, not the toolchain image. `nightly-build.yml` retains
  `provenance: true` / `sbom: true`. `grep` across `.github/workflows/` for any
  consumer of the toolchain image's attestations: **none** — nothing runs
  `cosign verify-attestation` / SBOM-diff against `charon-toolchain`. Disabling
  provenance/SBOM on an internal build *input* that nobody verifies is correct;
  it is what makes "same recipe key ⇒ identical manifest-list digest" hold and
  stops `sync-pin-on-pr` from looping.

- **"Skip build if `:$KEY` already published"** (`toolchain-image.yml` "Decide
  build plan"): on a **non-forced same-repo** run, if
  `imagetools inspect ${TOOLCHAIN_IMAGE}:${KEY}` resolves, `should_build=false`
  and the existing digest is reused / pinned. This does trust the current
  content of the mutable `:$KEY` tag. Mitigations: (i) writing that tag requires
  `packages: write` on the package = trusted maintainer; fork PRs never reach
  this path (no login → `type=cacheonly`); (ii) the daily run is `forced=true`,
  which bypasses skip-if-published, rebuilds deterministically, and — via
  `open-bump-pr` — surfaces any digest discrepancy as a `feat(security)` bot PR,
  so a poisoned tag self-heals within ~24 h; (iii) the app build ultimately pins
  an immutable `@digest` and the resulting app image is Trivy/Grype-scanned and
  signed. **Residual R1** (accept-knowingly): a maintainer-level credential
  compromise could, within a one-day window, get a poisoned `:$KEY` digest
  pinned via `sync-pin-on-pr` without that PR performing a from-source rebuild.
  The pre-PR `--no-cache-filter` behaviour rebuilt from source on every CVE-gate
  PR; this PR trades that for the daily deterministic rebuild + freshness guard.

- **Who holds `packages: write` on `ghcr.io/wikid82/charon-toolchain`:**
  - `toolchain-image.yml` → job `build-toolchain` (workflow-level
    `permissions: packages: write`). GHCR login **and** push are gated
    `if: steps.trust.outputs.same_repo == 'true'`; forks produce `type=cacheonly`
    (no push).
  - `security-weekly-rebuild.yml` → job `toolchain-rebuild`, which is
    `uses: ./.github/workflows/toolchain-image.yml` with
    `permissions: packages: write` (+ `contents/pull-requests/issues: write` for
    the bump-PR job). Same underlying workflow; caller event is
    `schedule`/`workflow_dispatch` ⇒ same-repo.
  - `docker-build.yml` (`build-amd64`/`build-arm64`/`merge-and-publish`),
    `nightly-build.yml`, `orthrus-build.yml` hold `packages: write` but target
    the `charon` / `charon-agent` images — they only **read** (`FROM …@digest`)
    the toolchain image, never push to it.
  No fork-reachable job can write the toolchain package.

---

## 4. New third-party action `iarekylew00t/regctl-installer` — verified, low risk

`quality-checks.yml` `verify-toolchain-pin` job:
`uses: iarekylew00t/regctl-installer@c2202c17a65fe59371c71ecc169c9e58c3710a15 # v4.0.16`

- **SHA ↔ release: VERIFIED.** Annotated tag `v4.0.16` → tag object
  `f14118b1…` → **points at commit `c2202c17a65fe59371c71ecc169c9e58c3710a15`**
  (message "chore: Bumping version to v4.0.16", tagger 2026-08-05). The pin is
  exact and matches the tag comment.
- **Action source at the pinned SHA:** `action.yml` is a compiled
  `using: node24` / `main: dist/index.js` action. Declared purpose: download the
  `regctl` release (default `latest` — here left default, so it resolves newest
  at run time) and, with `verify: true` (default, left on), **cosign-verify the
  downloaded binary's signature**. Inputs are `regctl-release`, `verify`,
  `cache`, `token` (`${{ github.token }}` default) — all consistent with a
  GitHub-API release downloader; nothing in `action.yml` indicates behaviour
  beyond install + verify. `dist/index.js` is a minified bundle and was not
  line-audited.
- **Blast radius:** runs only in the `verify-toolchain-pin` job, whose
  `permissions` are `contents: read` + `packages: read` — no write scope, no
  secrets beyond the read-only `GITHUB_TOKEN`.
- **`curl | sha256sum -c` vs this action:** an inline pinned-hash install would
  remove a compiled-JS third-party action from the trust chain, but it also
  drops the cosign signature check the action performs and needs manual hash
  bumps (staleness risk). Given the minimal job permissions, **not materially
  safer** — noting it (F3) as an optional hardening, not a defect. If adopted,
  pin `regctl-release` to an exact version too (currently `latest`).

---

## 5. Private `charon-toolchain` — every build path covered, forks still build

**`uses: ./.github/actions/build-charon-image` — 6 call sites, all pass both
`builder-src` (fork ternary) and `ghcr-token`:**

| Workflow | `builder-src` | `ghcr-token` |
|---|---|---|
| `security-pr.yml:158` | fork ternary → `inline` \| `prebuilt` | `secrets.GITHUB_TOKEN` |
| `supply-chain-pr.yml:253` | fork ternary | `secrets.GITHUB_TOKEN` |
| `cerberus-integration.yml:35` | fork ternary | `secrets.GITHUB_TOKEN` |
| `crowdsec-integration.yml:35` | fork ternary | `secrets.GITHUB_TOKEN` |
| `waf-integration.yml:35` | fork ternary | `secrets.GITHUB_TOKEN` |
| `rate-limit-integration.yml:35` | fork ternary | `secrets.GITHUB_TOKEN` |

All six also gained `permissions: packages: read`. The composite action logs in
to GHCR only `if: inputs.builder-src != 'inline' && inputs.ghcr-token != ''`, and
rejects an invalid `builder-src` with `exit 1`.

**Raw `docker buildx build` / `build-push-action` app-image paths:**

| Workflow / job | Toolchain source | GHCR login |
|---|---|---|
| `docker-build.yml` `build-amd64` / `build-arm64` | `CADDY_BUILDER_SRC` / `CROWDSEC_BUILDER_SRC` env = fork ternary (`*-inline` for foreign head repo, else `toolchain-prebuilt`) | pre-existing "Log in to GitHub Container Registry" step (`secrets.GITHUB_TOKEN`) |
| `e2e-tests-split.yml` `build` | build-args fork ternary | added `Log in to GHCR` step, `if: image_source == 'build' && head.repo.full_name == github.repository` |
| `nightly-build.yml` | hard-coded `toolchain-prebuilt` (schedule/same-repo only — correct) | pre-existing login-action + `packages: write` |
| `toolchain-image.yml` | builds the toolchain itself (`--target toolchain-runtime`, `caddy-inline`/`crowdsec-inline` from source) | login `if: same_repo == 'true'` |
| `orthrus-build.yml` | builds `agent/Dockerfile` only — **does not use the root Dockerfile / toolchain image**; no change needed | n/a |

**Fork PR path:** every ternary resolves `head.repo.full_name != '' &&
head.repo.full_name != github.repository` → `caddy-inline` / `crowdsec-inline`,
i.e. compile the byte-identical recipe from source (~14 min). `make build-offline`
(new) does the same locally. No fork build path depends on pulling the private
image. **Fork PRs still build.** ✅

**Empty-head-repo guard:** the `head.repo.full_name != ''` conjunct means `push`
and non-PR events (empty `head.repo.full_name`) correctly resolve to
`toolchain-prebuilt`, not accidentally to `inline`.

---

## 6. Trivy — clean, no new suppressions

- **`.trivyignore`: UNCHANGED in this PR.** `git log origin/main..tip -- .trivyignore`
  → no commits; 257 non-blank lines, identical to `main`. **No new blanket
  suppressions.** (`.trivyignore`'s `sha256` is itself a `toolchain-key.sh`
  input, so any future edit forces a toolchain rebuild + re-pin.)
- **CI Trivy runs on the tip — all green:**
  - "Trivy scan (toolchain image)" — `toolchain-image.yml` `trivy-scan`,
    `CRITICAL,HIGH`, `exit-code 1` — **pass**.
  - "Trivy Binary Scan" — `security-pr.yml` — **pass**.
  - "Security Scan PR Image" — app image built from the pinned toolchain —
    **pass**.
  - "Verify Supply Chain" — **pass**; "grype" — **pass**; "Semgrep SAST" /
    "Semgrep OSS" / "semgrep-cloud-platform" — **pass**.
  - Top-level "Trivy" shows `NEUTRAL / skipping` — this is the pre-existing
    always-on external check that no-ops on this event path, not a regression.
  - Zero unignored CRITICAL/HIGH on both the toolchain image and the app image
    built from it (the two `exit-code: '1'` gates passed).
- Local Trivy was **not** re-run: the toolchain image is a private GHCR package
  and this environment has no GHCR credentials; CI ran it with proper auth.
  Per CLAUDE.md this is a CI-scoped (`ci:`/`feat(ci)`) change and CI runs Trivy
  unconditionally, so nothing is skipped.

---

## 7. Definition of Done (CI-scoped change)

| DoD item | Status |
|---|---|
| Targeted Playwright E2E (touched specs) | **N/A to run locally** — no FE/BE/spec files changed. CI full E2E on tip `22e9c722` is **legit and complete**: "Prepare Application Image" (which now exercises the `toolchain-prebuilt` `COPY --from` path on a same-repo PR) **pass**; all shards **pass** — Chromium 1–4 + Security Enforcement, Firefox 1–4 + Security Enforcement, WebKit 1–4 + Security Enforcement; "E2E Test Results (Final)" **pass**. A broken pin/image would have failed the image build, not silently passed. |
| GORM security scan (`scan-gorm-security.sh`) | **N/A** — no `backend/internal/models/**`, no GORM queries, no migrations in the diff. |
| `local-patch-report.sh` / Go+TS patch coverage | **N/A** — no Go/TS lines changed. `codecov/patch` check on PR: **pass** (0 changed coverable lines). New executable code is shell, covered by 17 bats tests (see §2). |
| Frontend `npm run type-check` / `npm run build` / FE coverage 85% | **N/A** — no `frontend/` files touched. |
| Backend `go build ./...` / Go coverage 85% | **N/A** — no `backend/` files touched. "Backend (Go)" / "Agent (Go)" CI: **pass** (unchanged). |
| staticcheck / golangci-lint | **N/A** (no Go). Substituted by `shellcheck --severity=error` on the 3 new scripts — **clean locally** (also clean at default severity) and in CI job "Toolchain key / freshness-guard scripts (bats)"; `actionlint` on all 12 changed workflows — **clean locally** (`exit 0`). |
| CodeQL Go / JS | Green on tip ("CodeQL analysis (go)" + "(javascript-typescript)" **pass**). Effectively N/A (no Go/JS/TS source changed) but ran. |
| `bats scripts/tests/` | **17/17 PASS** locally + CI. |
| Build verification, `docker build` both `builder-src` modes | `toolchain-prebuilt` path: exercised & green across `build-amd64`, `build-arm64`, "Prepare Application Image", and all 6 integration image builds on this same-repo PR. `caddy-inline`/`crowdsec-inline` full-app path: **not exercised on a same-repo PR** (fork-only) — see **F4 / R2**. The inline *stage bodies themselves* are compiled from source on every `toolchain-image.yml` run (daily + tracked-path PRs) via `--target toolchain-runtime`, and passed on this PR ("Build & publish toolchain image" **pass**). |
| No debug leftovers in new scripts | **Clean** — `grep -nE 'TODO|FIXME|XXX|DEBUG|set -x|console.log|fmt.Print'` over the 3 scripts + 2 bats files + fixture helper → no matches. All three scripts use `set -euo pipefail`. |

---

## Follow-ups (non-blocking)

- **F1 — Confirm required-status enrolment.** Verify branch protection for `main`
  (and `development`) lists **"Toolchain pin freshness (verify-toolchain-pin)"**
  and **"Toolchain key / freshness-guard scripts (bats)"** as required checks.
  The failure-closed guarantee in §1(c) / §2 only bites if the check is required;
  the code is correct but enrolment is a repo-settings action outside this diff.
- **F2 — Close two bats coverage gaps** in `verify-toolchain-pin.bats`: (i)
  same-repo run with `regctl` present but `image digest` failing (unresolvable
  `:$KEY`) → `exit 1`; (ii) same-repo run with `CHARON_TOOLCHAIN_DIGEST` empty →
  `exit 1`. And in `toolchain-key.bats`: sensitivity to a `tonistiigi/xx` pin
  move and an `ALPINE_IMAGE` move (both are hashed inputs, currently untested).
- **F3 — (optional) `regctl-installer` hardening.** Either accept as-is (minimal
  job perms, cosign verify on) or replace with a pinned-hash `curl | sha256sum -c`
  install; if kept, pin `regctl-release` to an exact version rather than the
  default `latest`.
- **F4 — Add periodic coverage of the offline/inline app build.** A scheduled or
  label-gated job running `make build-offline` (or at minimum
  `docker build --build-arg CADDY_BUILDER_SRC=caddy-inline
  --build-arg CROWDSEC_BUILDER_SRC=crowdsec-inline --check .`) so the
  `FROM ${CADDY_BUILDER_SRC} AS caddy-builder` selector + final-stage assembly on
  the fork path can't silently rot between fork PRs.
- **F5 — Stale note in `docs/ci/toolchain-image.md`.** The "One-time bootstrap
  notes" paragraph still says *"`COPY scripts/ /app/scripts/` copies the new
  shell scripts into the runtime image … No `.dockerignore` … change is needed"*,
  which the `.dockerignore` follow-up commit `22e9c722` (excludes
  `scripts/tests/`, `scripts/toolchain-key.sh`, `scripts/verify-toolchain-pin.sh`,
  `scripts/lib/dockerfile-stage.sh` from the build context) now contradicts.
  One-paragraph doc fix.

## Residual supply-chain risk — accept knowingly

- **R1 — one-day poisoned-tag window.** An actor with `packages: write` on
  `ghcr.io/wikid82/charon-toolchain` (maintainer-level) could push a poisoned
  image to the mutable `:$KEY` tag; a non-forced same-repo PR that recomputes the
  same key would skip the rebuild and `sync-pin-on-pr` could pin that digest
  without a from-source rebuild *in that PR*. Bounded by: fork PRs cannot reach
  the path; the daily `--no-cache --pull` deterministic rebuild + `open-bump-pr`
  self-heal within ~24 h; the resulting app image is still Trivy/Grype-scanned,
  SBOM-attested and Cosign-signed. Net change vs pre-PR: the "every CVE-gate PR
  rebuilds Caddy/CrowdSec from source" property is replaced by "daily
  deterministic rebuild + per-PR freshness guard + immutable digest pin."
- **R2 — fork/offline `caddy-inline`+`crowdsec-inline` *whole-app* build is not
  CI-exercised on same-repo PRs.** The stage bodies are compiled daily by
  `toolchain-image.yml`; only the `FROM ${ARG} AS caddy-builder` indirection and
  the final-stage COPY wiring on the inline path go unverified until a fork PR or
  a manual `make build-offline`. Low severity (small surface, `toolchain-key.sh`
  sanity-checks the stages exist and contain a build step). F4 closes it.
- **R3 — the toolchain image itself is digest-pinned but not Cosign-signed.**
  Acceptable: it is built by the repo's own Actions, pulled by immutable digest,
  and recipe→digest is bound by the freshness guard + LABEL check; the shipped
  app image carries the signature/attestation.

---

## Scans run for this audit

- `bats scripts/tests/toolchain-key.bats scripts/tests/verify-toolchain-pin.bats` → **17/17 pass** (`Bats 1.13.0`)
- `shellcheck --severity=error` + default severity on `scripts/toolchain-key.sh`, `scripts/verify-toolchain-pin.sh`, `scripts/lib/dockerfile-stage.sh` → **clean**
- `actionlint` on the 12 changed workflow files → **clean (exit 0)**
- Debug-leftover grep over the 3 scripts + 2 bats + fixture → **clean**
- `git log origin/main..tip -- .trivyignore` → **no changes**
- `gh api` verification: `regctl-installer` tag `v4.0.16` → commit `c2202c17…` → **exact match**
- `gh pr checks 1300` on tip `22e9c722` → **no failing / cancelled checks** (all pass or intentionally skipped)
- Diff review of `Dockerfile`, `toolchain-image.yml`, `build-charon-image/action.yml`, and the 11 other changed workflows; `SECURITY.md`, `ARCHITECTURE.md`, `docs/ci/toolchain-image.md`
- GORM security scan — **not run (N/A: no models/queries/migrations)**
- Local Trivy / CodeQL — **deferred to CI** (CI-scoped change; both ran green with proper credentials)
