# Spec: Compile DNS-01 Provider Modules into the Caddy Build (GitHub #1361)

- **Issue:** [#1361 — Couldn't request wildcard DNS-01 certificates from web UI](https://github.com/Wikid82/charon/issues/1361)
- **Branch:** `fix/dns-provider-caddy-modules` (dedicated branch, already checked out, branched off `development`; PR into `development` — medium/large scope per `CLAUDE.md` branching rules; this is not a hotfix and must not land directly on `development`. This branch already exists — do not create a new one.)
- **Change class:** `fix` (restores advertised functionality for the 10 existing built-in providers) with a `feat`-shaped addition (20 new Caddy-module-only providers, no backend/UI). One PR, ordered commits — see §6.
- **Author identity for commit trailer/PR:** jhatfield82@proton.me (no session ID/link per `CLAUDE.md` "NO SESSION ATTRIBUTION").

---

## 1. Introduction

### 1.1 Problem

Charon lets users configure "built-in" DNS-01 providers (Cloudflare, Route 53, DigitalOcean, Google Cloud DNS, Azure, Namecheap, GoDaddy, Hetzner, Vultr, DNSimple) through the UI, and `docs/guides/dns-providers.md` states these are "Compiled into Charon." They are not. The root `Dockerfile`'s `xcaddy` build (the `caddy-inline` stage, `--with` list around lines 427-436) only compiles in security/WAF/rate-limit/geoip/crowdsec plugins — zero `github.com/caddy-dns/*` modules. When a user saves a wildcard proxy host with any DNS provider attached, `backend/pkg/dnsprovider/builtin/*.go`'s `BuildCaddyConfig` emits Caddy JSON referencing e.g. `"name": "cloudflare"` (module ID `dns.providers.cloudflare`). Caddy's admin API rejects this with `unknown module: dns.providers.cloudflare` (400), and because `ApplyConfig` → `client.Load` (`backend/internal/caddy/client.go:69-102`) does an atomic `/load`, `internal/caddy/manager.go` rolls back the **entire** config — poisoning unrelated saves (e.g. attaching a manually uploaded cert to a different host) with the same failure.

GitHub issue #1361 also reports a frontend crash (`DNSProviderSelector.tsx:82`, `Cannot read properties of undefined (reading 'toString')`) and inability to input a `*` wildcard host. Current `frontend/src/components/DNSProviderSelector.tsx` (verified in this research pass) contains no `.toString()` call and no code path matching that stack trace — that symptom has already been resolved by unrelated prior work between the reporter's version and current `development`. **This spec addresses the remaining, still-live root cause: the missing Caddy DNS-01 modules.** If a reviewer can still reproduce the frontend crash independently on current `development`, that is a separate frontend bug and should be filed/tracked separately — it is out of scope here.

### 1.2 Objective

1. Compile a curated set of **30** `github.com/caddy-dns/*` provider modules into the xcaddy build so `dns.providers.*` module IDs actually exist in the shipped `caddy` binary.
2. Pin each module's version with its own Renovate-tracked `ARG`, following the repo's exact existing convention (e.g. `CADDY_SECURITY_VERSION`).
3. Correct `docs/guides/dns-providers.md` so it stops claiming universal "compiled into Charon" status and instead accurately separates the 10 providers with full backend/UI support from the 20 Caddy-module-only providers (module compiled in, but no Charon UI/credential form yet — tracked separately in #1374).
4. Do this without introducing a second, divergent way of tracking Caddy plugin versions (reuse the existing per-ARG regex-manager pattern verbatim).

### 1.3 Non-goals

- Backend/UI support (credential forms, `ProviderPlugin` implementations, `RequiredCredentialFields`, API type registration) for the 20 new-module-only providers. Tracked in **#1374**.
- Adding all ~84 `github.com/caddy-dns/*` modules that exist upstream. The user has explicitly chosen the 30 below as the size/build-time/attack-surface tradeoff.
- Any change to `backend/internal/caddy/manager.go`'s atomic rollback-on-failure behavior (that's arguably a separate resiliency improvement — "one broken host poisons every save" — and is not part of this fix; noted as a **Risk** below, not addressed here).
- The frontend crash / `*` wildcard input issue described in §1.1 (already resolved on `development`, unless a reviewer can reproduce it fresh, in which case it is a separate ticket).

---

## 2. Research Findings

### 2.1 Existing convention (Dockerfile + renovate.json)

The Dockerfile already has a fully worked pattern for exactly this kind of pin, used by `caddy-security`, `coraza-caddy`, `caddy-geoip2`, `caddy-ratelimit`:

1. A **global default ARG**, declared before any `FROM`, with a `# renovate: datasource=go depName=<module>` comment immediately above it:
   ```dockerfile
   # renovate: datasource=go depName=github.com/greenpau/caddy-security
   ARG CADDY_SECURITY_VERSION=1.2.1
   ```
   (Dockerfile, global ARG block, ~lines 84-93, alongside `CADDY_GEOIP2_VERSION` / `CADDY_RATELIMIT_VERSION`.)

2. A **redeclaration** of the same `ARG` (no default value) inside the `caddy-inline` stage, so the build-stage scope can see the build-time value:
   ```dockerfile
   ARG CADDY_SECURITY_VERSION
   ```
   (Dockerfile, `caddy-inline` stage ARG block, ~lines 349-356, alongside the other three plugin ARGs and ahead of `XCADDY_VERSION`.)

3. A `--with` line in the `xcaddy build` invocation (Stage 1, "generate go.mod"), inside the `caddy-inline` stage's big `RUN --mount=...` block (~lines 427-436):
   ```dockerfile
   --with github.com/greenpau/caddy-security@v${CADDY_SECURITY_VERSION} \
   ```

4. A matching **Renovate custom regex manager** in `.github/renovate.json` (`customManagers`, ~lines 55-68):
   ```json
   {
     "customType": "regex",
     "description": "Track caddy-security plugin version in Dockerfile",
     "managerFilePatterns": ["/^Dockerfile$/"],
     "matchStrings": ["ARG CADDY_SECURITY_VERSION=(?<currentValue>[^\\s]+)"],
     "depNameTemplate": "github.com/greenpau/caddy-security",
     "datasourceTemplate": "go",
     "versioningTemplate": "semver"
   }
   ```

This spec reuses this exact 4-part pattern for all 30 new `ARG`s — no new tracking mechanism.

### 2.2 `backend/pkg/dnsprovider/builtin/` — confirms zero coupling to a Go SDK

`init.go` registers 10 `ProviderPlugin`s (`CloudflareProvider`, `Route53Provider`, `DigitalOceanProvider`, `GoogleCloudDNSProvider`, `AzureProvider`, `NamecheapProvider`, `GoDaddyProvider`, `HetznerProvider`, `VultrProvider`, `DNSimpleProvider`). Each `*.go` file (e.g. `cloudflare.go`) only builds a `map[string]any` Caddy-JSON fragment (`BuildCaddyConfig`) — **the backend never imports a provider SDK directly**; `backend/go.mod` has no `cloudflare-go`, AWS SDK, Azure SDK, etc. Confirmed via `grep -niE "cloudflare|route53|digitalocean|googleclouddns|azure|namecheap|godaddy|hetzner|vultr|dnsimple"` against `backend/go.mod` → no matches. All actual DNS-provider API calls happen inside the compiled Caddy binary's `dns.providers.*` module, which is exactly what's missing. This also means: **none of the 10 existing providers currently have any Renovate tracker for their Caddy DNS Go module** (`grep -n "caddy-dns" .github/renovate.json Dockerfile backend/go.mod` → no matches anywhere in the repo). All 30 trackers proposed in §4 are net-new; there is no "already tracked" subset to reconcile.

### 2.3 Module inventory — verified import paths, versions, and two upstream gotchas

Verified directly against each `github.com/caddy-dns/<name>` repo's `go.mod` (module path + `go` directive) and GitHub Releases/tags API (2026-09-21). Full table in §4.1.

**Gotcha 1 — `hetzner` uses a `/v2` import path.** `github.com/caddy-dns/hetzner`'s `go.mod` declares `module github.com/caddy-dns/hetzner/v2`, matching its `v2.0.1` tag (correct Go major-version-suffix convention). The `--with` line **must** be `--with github.com/caddy-dns/hetzner/v2@v${CADDY_DNS_HETZNER_VERSION}` — a bare `github.com/caddy-dns/hetzner@v2.0.1` will fail to resolve.

**Gotcha 2 — `transip` tags v2.x but never bumped its module path.** `github.com/caddy-dns/transip`'s `go.mod` still declares `module github.com/caddy-dns/transip` (no `/v2` suffix) even at its latest tag `v2.0.3`. This violates Go's semantic-import-versioning rule (a v2+ module with a `go.mod` present must have `/v2` in its path), and `go get`/`xcaddy --with github.com/caddy-dns/transip@v2.0.3` will fail with `invalid version: module contains a go.mod file, so major version must be compatible: should be v0 or v1, not v2`. **Mitigation:** pin `transip` to its last correctly-path-compatible tag, `v1.0.0`, not the nominally "latest" `v2.0.3`. Flagged as a real upstream inconsistency, not a typo in this spec — do not "fix" it to v2.0.3 during implementation without re-verifying upstream has corrected the module path.

**Gotcha 3 — five modules have never tagged a release.** `digitalocean`, `vultr`, `dnsimple`, `namesilo`, and `civo` have zero tags (`git tags` API returns `[]`). Go modules without tags are consumed via **pseudo-versions** (`v0.0.0-<UTC-commit-timestamp>-<12-char-abbrev-SHA>`), resolved from each repo's default branch HEAD at the time of this research (2026-09-21):

| Module | Default branch | HEAD SHA (12-char) | Commit timestamp (UTC) | Pseudo-version |
|---|---|---|---|---|
| digitalocean | `master` | `04bde2867106` | 2025-06-06T07:45:28Z | `v0.0.0-20250606074528-04bde2867106` |
| vultr | `master` | `55bf3e9768be` | 2025-07-23T12:15:31Z | `v0.0.0-20250723121531-55bf3e9768be` |
| dnsimple | `main` | `0433343c5610` | 2026-03-03T13:12:43Z | `v0.0.0-20260303131243-0433343c5610` |
| namesilo | `master` | `e646346d8db8` | 2026-02-19T11:14:33Z | `v0.0.0-20260219111433-e646346d8db8` |
| civo | `main` | `e2766c887ff5` | 2024-05-12T16:34:41Z | `v0.0.0-20240512163441-e2766c887ff5` |

These are legitimate, resolvable Go pseudo-versions (`go get`/`xcaddy --with` handle them identically to tagged versions) but Renovate's `go` datasource update-detection on a pseudo-version is noisier than on real tags — expect Renovate PRs for these five to bump the pseudo-version whenever upstream's default branch moves, even with no functional change. This is expected/accepted, not a defect to fix — see §7 Risks.

**Gotcha 4 — `civo`, `exoscale`, `transip` are hard-blocked by a project-wide `libdns` MVS conflict (found by the Commit 2 build gate, not visible from static inspection).** Nine of the 30 modules (route53, linode, netlify, desec, scaleway, dnsmadeeasy, powerdns, inwx, loopia) require `github.com/libdns/libdns` v1.1.x, which replaced the old struct-based `libdns.Record` type with an interface (breaking change). Go's MVS therefore resolves `libdns/libdns` to v1.1.1 project-wide once those 9 are in the build. `caddy-dns/namedotcom` and `caddy-dns/vercel` also depend on old-API `libdns/<provider>` packages, but each has a newer release (`libdns/namedotcom` v0.9.0, `libdns/vercel` v0.1.0) that supports the new interface — forcing those via two extra `--with github.com/libdns/<provider>@v...` lines (own tracked ARGs, see §4.2) resolves it. **`civo` and `exoscale` have no such newer release anywhere upstream** (checked latest tags and default-branch HEAD directly) — both are permanently stuck on the old `libdns.Record` struct API and fail to compile once v1.1.1 is forced. **`transip` is separately and independently blocked**: forcing `libdns/transip` to v1.1.2 breaks `caddy-dns/transip`'s wrapper (its pinned pre-v2 commit predates a struct field rename), and the only upstream commits with the new API are tagged v2.x, which Go's semantic-import-versioning rules make unreachable under the base (non-`/v2`) import path (see Gotcha 2) regardless of pin — so there is no resolvable commit that satisfies both constraints.

**Decision (post-review, confirmed with user): `civo`, `exoscale`, and `transip` are dropped from this fix.** The curated set ships as **27 modules**, not 30. This is a hard upstream blocker, not a Dockerfile/pin mistake — re-adding any of the three requires upstream `caddy-dns`/`libdns` releases that don't exist yet. They are re-tracked in GitHub issue #1374 under a "blocked upstream" category (moved out of the "Caddy-module-only, no backend/UI" category they were originally listed under, since they now have neither). All references to these three in §4.1/§4.2/§4.3/§6 below are retained for the record (they show the exact versions/gotchas that were tried) but are marked **EXCLUDED** — do not implement their ARGs/`--with` lines/Renovate trackers.

### 2.4 No conflicting transitive dependencies identified (superseded — see Gotcha 4)

All 30 modules' `go.mod` files declare `go 1.16`–`go 1.25.1` (see §4.1), comfortably under the Dockerfile's `ARG GO_VERSION=1.27.1`. None of the 30 declare a direct dependency overlapping the existing pinned plugin set (`caddy-security`, `coraza-caddy/v2`, `caddy-crowdsec-bouncer`, `caddy-geoip2`, `caddy-ratelimit`) beyond the shared `caddy/v2` API itself, which xcaddy already resolves via MVS across all `--with` modules. **No static conflict found by inspection** — this is confirmed empirically, not just by inspection, via the mandatory Docker build validation gate in Commit 2 (§6), which is exactly why that gate exists rather than being treated as optional.

### 2.5 No Stage-2 `go get` patch step is needed for the 30 new modules

Stage 2 of `caddy-inline` (Dockerfile ~lines 445-483) exists to force-patch **transitive** dependencies with known CVEs (`expr-lang/expr`, `golang.org/x/crypto`, `grpc`, `go-jose`, OTel, etc.) — this is orthogonal to adding new top-level `--with` modules. None of the 30 DNS provider modules are currently patched there, and none are expected to need it: they don't pull in any of the already-patched CVE-affected packages at versions older than the pins Stage 2 already enforces (xcaddy's MVS resolution will pick the Stage-2-pinned version for any shared transitive dep automatically, since `go get @vX` in Stage 2 runs after all `--with` modules are already in `go.mod`). No new Stage 2 entries are planned. If the Docker build validation gate (§6, Commit 2) surfaces a new CVE-relevant transitive dependency pulled in by one of the 30, add a Stage 2 patch line then, following the existing `_retry go get <module>@v<pinned>` + `# renovate: datasource=go depName=<module>` pattern — not before, and not speculatively.

### 2.6 Toolchain-image pin interaction (important build-system detail)

The Dockerfile is **not** compiled on every app build. `scripts/toolchain-key.sh` hashes into a content-addressed tag `caddy-crowdsec-<16-hex>` (pinned via `ARG CHARON_TOOLCHAIN_TAG` / `ARG CHARON_TOOLCHAIN_DIGEST`):

1. the exact text of the `caddy-inline`/`crowdsec-inline` stage bodies, and
2. the resolved default values of a **hardcoded whitelist** of version `ARG`s — the `arg_re` regex at `scripts/toolchain-key.sh:68` — **not every `ARG` in the Dockerfile**. Verified by reading the script directly: `arg_re` names each tracked ARG individually (`GO_VERSION`, `CADDY_VERSION`, `CADDY_SECURITY_VERSION`, `CADDY_GEOIP2_VERSION`, `CADDY_RATELIMIT_VERSION`, etc.); anything not named in that regex is invisible to the hash even if it's a `# renovate:`-tracked `ARG` sitting right next to ones that are (per the script's own `rev-2` comment: "added the two xcaddy plugin pins ... to the hashed input set" — that whitelist had to be hand-expanded when `caddy-security`/`caddy-geoip2`/`caddy-ratelimit` were added; it does not auto-discover new ARGs).

`scripts/verify-toolchain-pin.sh` is a **required, failure-closed CI check** that fails if the pinned tag doesn't match the recomputed key.

**Why this matters for this PR specifically (not just "the hash changes when the Dockerfile changes"):** Commit 2 changes the `caddy-inline` stage *body* (30 new `--with` lines are text inside the hashed stage), so the key *will* change the moment Commit 2 lands, and Commit 2's own `verify-toolchain-pin.sh` gate will correctly go red and get resolved by `sync-pin-on-pr` (see below) — that one-time transition works fine and is not the bug. **The bug is steady-state, post-merge:** the 30 new `CADDY_DNS_*_VERSION` values live in the **global default `ARG` block** (§4.2A), not inside either hashed stage body, and none of the 30 new ARG names are added to `arg_re`. So when Renovate later opens a PR bumping, say, `CADDY_DNS_CLOUDFLARE_VERSION=0.2.4` → `0.2.5`, that value change is invisible to both hash inputs (1) and (2) above — `toolchain-key.sh` recomputes the *same* key as before the bump, `verify-toolchain-pin.sh` passes, and the toolchain image is **never rebuilt** for that version bump. The stale prebuilt image keeps shipping the old plugin version indefinitely, silently, with a green CI check the whole time. This would defeat the entire purpose of pinning these 30 modules via Renovate.

**Required fix (Commit 2b, §6):** add all 30 `CADDY_DNS_<PROVIDER>_VERSION` names to the `arg_re` whitelist in `scripts/toolchain-key.sh:68`, following the exact `rev-2` precedent already documented in the script (same file, same style of expansion previously done for the security/geoip2/ratelimit plugins), and bump `SCHEMA_VERSION` (currently `2` → `3`) per the script's own stated convention ("bump to force a global rebuild if this extraction logic itself changes") — adding new names to `arg_re` is exactly that kind of extraction-logic change, and bumping it forces the one-time rebuild needed to pick up the corrected hash inputs. This is a required part of this PR, not a follow-up.

Practical consequence for this PR:
- The moment Commit 2 (Dockerfile `--with`/`ARG` changes) is pushed, the pinned `CHARON_TOOLCHAIN_TAG`/`CHARON_TOOLCHAIN_DIGEST` become stale by construction — `verify-toolchain-pin.sh` **will fail** on that commit until the toolchain image is rebuilt and re-pinned. This stays true, and is expected, whether or not Commit 2b has landed yet (Commit 2b changes *what future bumps* the hash notices, not whether *this* commit's stage-body edit is noticed — that part already works via the stage-body hash).
- This is expected and self-healing in CI: `.github/workflows/toolchain-image.yml`'s `pull_request` trigger fires on any same-repo PR touching `Dockerfile` (and now `scripts/toolchain-key.sh` once Commit 2b lands), rebuilds `--target toolchain-runtime`, publishes `:<new-key>`, and its `sync-pin-on-pr` job pushes a commit onto the PR branch updating `CHARON_TOOLCHAIN_TAG`/`CHARON_TOOLCHAIN_DIGEST` to match. No manual pin-bump commit is needed in this PR — **do not hand-edit `CHARON_TOOLCHAIN_TAG`/`CHARON_TOOLCHAIN_DIGEST`**; let `sync-pin-on-pr` own that value.
- Locally, this rebuild-and-republish cycle doesn't run, so local validation must use the **from-source fallback path** (`--build-arg CADDY_BUILDER_SRC=caddy-inline --build-arg CROWDSEC_BUILDER_SRC=crowdsec-inline`, i.e. `make build-offline`), which compiles the real `caddy-inline` stage directly instead of pulling the (not-yet-republished) prebuilt toolchain image. This is the Commit 2 validation gate (§6).

### 2.7 No "advanced config" escape hatch exists for DNS provider JSON

Searched `backend/internal/caddy/` and `docs/` for a raw-Caddy-JSON passthrough for TLS/DNS config specifically (`grep -n -i "raw config|advanced config|caddy json|escape hatch|custom config"`). The only "advanced config" surface found (`backend/internal/caddy/config.go` `NormalizeAdvancedConfig`, tested in `config_test.go:1560`) is scoped to per-host **handler/header** injection in the reverse-proxy route, not to `tls.automation.policies[].issuers[].challenges.dns.provider`. There is **no way for a user to manually wire up one of the 20 Caddy-module-only providers via the UI today**, even after this fix compiles the module in. This confirms the docs rewrite plan in §5: those 20 are "compiled in, not yet usable from Charon" — not a hidden advanced-config workaround — until #1374 adds backend/UI support.

### 2.8 `ARCHITECTURE.md` consulted (per CLAUDE.md "Architecture Awareness")

This PR changes the toolchain/deployment build recipe (the xcaddy `--with` set and, per §2.6, `scripts/toolchain-key.sh`), which is core deployment-configuration territory, so `ARCHITECTURE.md` was read before finalizing this plan, specifically the "Prebuilt toolchain image" section (`ARCHITECTURE.md:1208-1244`). Relevant finding: the "Freshness guard" bullet (`ARCHITECTURE.md:1231-1234`) currently reads:

> `scripts/toolchain-key.sh` derives a content-addressed tag (`caddy-crowdsec-<hex>`) from the two inline stage bodies + every consumed version ARG (**incl. the two pinned xcaddy plugins**) + the digest-pinned `golang`/`xx` bases + `.trivyignore`.

The parenthetical "(incl. the two pinned xcaddy plugins)" describes exactly the `arg_re` whitelist mechanism this spec is expanding from 2 tracked plugin ARGs to 32 (2 existing + 30 new `CADDY_DNS_*_VERSION`). Left as-is after this PR ships, that parenthetical would be stale/misleading to the next engineer reading it. §6 Commit 2b updates this sentence to reflect the enlarged pinned-plugin set (see exact wording in the Commit 2b scope below). No other part of `ARCHITECTURE.md` needs a change — no new stages, images, ports, volumes, or deployment topology are introduced by this PR.

---

## 3. Technical Specifications

No API contracts, database schema, or component interactions change in this PR — this is a build-recipe and documentation fix. The relevant "interface" is the Caddy JSON `tls.automation.policies[].issuers[].challenges.dns.provider.name` value each `BuildCaddyConfig()` in `backend/pkg/dnsprovider/builtin/*.go` already emits; this PR makes the corresponding `dns.providers.<name>` module ID resolvable at Caddy admin-API `/load` time. No Go/TS code changes.

### 3.1 Error handling / edge cases carried by this change

| Scenario | Before this fix | After this fix |
|---|---|---|
| User attaches a supported provider (10) to a wildcard host, saves | `ApplyConfig` 400s `unknown module: dns.providers.<name>`, entire config rollback (including unrelated hosts) | Module resolves; Caddy issues the DNS-01 challenge normally (success/failure now depends on real credentials/DNS propagation, not a missing module) |
| User picks one of the 20 module-only providers | Not selectable — no `ProviderPlugin`/UI entry exists (unchanged by this PR) | Still not selectable — unchanged; module is compiled in for #1374 to build on, but is inert without backend/UI support |
| `transip` pinned at `v1.0.0` instead of `v2.0.3` | N/A (not present) | Documented deviation (§2.3 Gotcha 2) — if a future Renovate PR proposes bumping to `v2.x`, it must be **manually rejected/held** until upstream fixes the module path, or the build breaks. Recommend adding `transip` to a follow-up note/issue so this isn't silently forgotten. |
| Docker build where a new `--with` module conflicts with an existing pinned transitive dep | N/A | Build fails during `xcaddy build` Stage 1 (go.mod resolution) or Stage 2 patch step; caught by the Commit 2 Docker build validation gate before merge, never silently shipped |

---

## 4. Implementation Plan

### 4.1 Full module table (30) — exact `--with` value per module

All new `ARG` names follow `CADDY_DNS_<PROVIDER_UPPER>_VERSION`, stored **without** a leading `v` (matching the existing `CADDY_SECURITY_VERSION`/`CADDY_GEOIP2_VERSION` convention — the `--with` line supplies the `v`).

| # | Provider | Backend/UI? | Go import path | ARG name | Pinned value | `go.mod` `go` directive | Note |
|---|---|---|---|---|---|---|---|
| 1 | cloudflare | Yes (existing) | `github.com/caddy-dns/cloudflare` | `CADDY_DNS_CLOUDFLARE_VERSION` | `0.2.4` | 1.23.0 | |
| 2 | route53 | Yes (existing) | `github.com/caddy-dns/route53` | `CADDY_DNS_ROUTE53_VERSION` | `1.6.2` | 1.25.0 | |
| 3 | digitalocean | Yes (existing) | `github.com/caddy-dns/digitalocean` | `CADDY_DNS_DIGITALOCEAN_VERSION` | `0.0.0-20250606074528-04bde2867106` | 1.24 | no tags — pseudo-version, §2.3 Gotcha 3 |
| 4 | googleclouddns | Yes (existing) | `github.com/caddy-dns/googleclouddns` | `CADDY_DNS_GOOGLECLOUDDNS_VERSION` | `1.1.0` | 1.23.0 | |
| 5 | azure | Yes (existing) | `github.com/caddy-dns/azure` | `CADDY_DNS_AZURE_VERSION` | `0.6.0` | 1.24 | |
| 6 | namecheap | Yes (existing) | `github.com/caddy-dns/namecheap` | `CADDY_DNS_NAMECHEAP_VERSION` | `1.0.0` | 1.23 | |
| 7 | godaddy | Yes (existing) | `github.com/caddy-dns/godaddy` | `CADDY_DNS_GODADDY_VERSION` | `1.2.0` | 1.25 | |
| 8 | hetzner | Yes (existing) | `github.com/caddy-dns/hetzner/v2` | `CADDY_DNS_HETZNER_VERSION` | `2.0.1` | 1.25.0 | `/v2` import path, §2.3 Gotcha 1 |
| 9 | vultr | Yes (existing) | `github.com/caddy-dns/vultr` | `CADDY_DNS_VULTR_VERSION` | `0.0.0-20250723121531-55bf3e9768be` | 1.24 | no tags — pseudo-version |
| 10 | dnsimple | Yes (existing) | `github.com/caddy-dns/dnsimple` | `CADDY_DNS_DNSIMPLE_VERSION` | `0.0.0-20260303131243-0433343c5610` | 1.25.0 | no tags — pseudo-version |
| 11 | ovh | No (#1374) | `github.com/caddy-dns/ovh` | `CADDY_DNS_OVH_VERSION` | `1.1.0` | 1.24 | |
| 12 | gandi | No (#1374) | `github.com/caddy-dns/gandi` | `CADDY_DNS_GANDI_VERSION` | `1.1.0` | 1.24 | |
| 13 | linode | No (#1374) | `github.com/caddy-dns/linode` | `CADDY_DNS_LINODE_VERSION` | `0.8.0` | 1.25.1 | |
| 14 | porkbun | No (#1374) | `github.com/caddy-dns/porkbun` | `CADDY_DNS_PORKBUN_VERSION` | `0.3.1` | 1.24 | |
| 15 | netlify | No (#1374) | `github.com/caddy-dns/netlify` | `CADDY_DNS_NETLIFY_VERSION` | `1.2.0` | 1.23 | |
| 16 | desec | No (#1374) | `github.com/caddy-dns/desec` | `CADDY_DNS_DESEC_VERSION` | `1.1.0` | 1.25 | |
| 17 | scaleway | No (#1374) | `github.com/caddy-dns/scaleway` | `CADDY_DNS_SCALEWAY_VERSION` | `0.2.2` | 1.25.1 | |
| 18 | duckdns | No (#1374) | `github.com/caddy-dns/duckdns` | `CADDY_DNS_DUCKDNS_VERSION` | `0.5.0` | 1.24 | |
| 19 | dnsmadeeasy | No (#1374) | `github.com/caddy-dns/dnsmadeeasy` | `CADDY_DNS_DNSMADEEASY_VERSION` | `1.2.0` | 1.18.0 | |
| 20 | namedotcom | No (#1374) | `github.com/caddy-dns/namedotcom` | `CADDY_DNS_NAMEDOTCOM_VERSION` | `0.1.2` | 1.16 | requires forced `libdns/namedotcom@v0.9.0` bump, §2.3 Gotcha 4 |
| 21 | namesilo | No (#1374) | `github.com/caddy-dns/namesilo` | `CADDY_DNS_NAMESILO_VERSION` | `0.0.0-20260219111433-e646346d8db8` | 1.16 | no tags — pseudo-version |
| 22 | rfc2136 | No (#1374) | `github.com/caddy-dns/rfc2136` | `CADDY_DNS_RFC2136_VERSION` | `1.0.0` | 1.24 | |
| 23 | powerdns | No (#1374) | `github.com/caddy-dns/powerdns` | `CADDY_DNS_POWERDNS_VERSION` | `1.0.2` | 1.25 | |
| 24 | inwx | No (#1374) | `github.com/caddy-dns/inwx` | `CADDY_DNS_INWX_VERSION` | `0.4.1` | 1.24 | |
| 25 | loopia | No (#1374) | `github.com/caddy-dns/loopia` | `CADDY_DNS_LOOPIA_VERSION` | `1.0.1` | 1.25.1 | |
| ~~26~~ | ~~transip~~ | **EXCLUDED** | `github.com/caddy-dns/transip` | — | — | — | Gotcha 4: hard-blocked, no resolvable commit satisfies both the `/v2` import-path rule and the `libdns` v1.1.1 floor. Not shipped. |
| ~~27~~ | ~~exoscale~~ | **EXCLUDED** | `github.com/caddy-dns/exoscale` | — | — | — | Gotcha 4: no upstream release supports `libdns` v1.1.1. Not shipped. |
| ~~28~~ | ~~civo~~ | **EXCLUDED** | `github.com/caddy-dns/civo` | — | — | — | Gotcha 4: no upstream release supports `libdns` v1.1.1. Not shipped. |
| 26 | bunny | No (#1374) | `github.com/caddy-dns/bunny` | `CADDY_DNS_BUNNY_VERSION` | `1.2.0` | 1.24 | |
| 27 | vercel | No (#1374) | `github.com/caddy-dns/vercel` | `CADDY_DNS_VERCEL_VERSION` | `0.0.2` | 1.25 | requires forced `libdns/vercel@v0.1.0` bump, §2.3 Gotcha 4 |

### 4.2 Dockerfile diff plan

**A. Global default `ARG` block** — insert immediately after the existing plugin block (after `ARG CADDY_RATELIMIT_VERSION=0.1.0`, before the cross-compilation helpers section), one `ARG` per module from §4.1, each preceded by its Renovate marker comment:

```dockerfile
# ---- DNS-01 Challenge Provider Modules (github.com/caddy-dns/*) ----
# Curated set of 30 DNS provider modules compiled into xcaddy so
# backend/pkg/dnsprovider/builtin/*.go's BuildCaddyConfig() output (module ID
# "dns.providers.<name>") actually resolves at Caddy admin-API /load time.
# Fixes GitHub #1361. 10 have full Charon backend/UI support (existing
# built-in providers); 20 are Caddy-module-only, tracked for backend/UI in
# GitHub #1374. See docs/plans/current_spec.md §4.1 for the full table,
# including two upstream import-path/versioning gotchas (hetzner /v2 path;
# transip pinned below its latest tag because that tag's go.mod never moved
# to /v2) and five modules pinned to pseudo-versions (no upstream tags).
# renovate: datasource=go depName=github.com/caddy-dns/cloudflare
ARG CADDY_DNS_CLOUDFLARE_VERSION=0.2.4
# renovate: datasource=go depName=github.com/caddy-dns/route53
ARG CADDY_DNS_ROUTE53_VERSION=1.6.2
# renovate: datasource=go depName=github.com/caddy-dns/digitalocean
ARG CADDY_DNS_DIGITALOCEAN_VERSION=0.0.0-20250606074528-04bde2867106
# renovate: datasource=go depName=github.com/caddy-dns/googleclouddns
ARG CADDY_DNS_GOOGLECLOUDDNS_VERSION=1.1.0
# renovate: datasource=go depName=github.com/caddy-dns/azure
ARG CADDY_DNS_AZURE_VERSION=0.6.0
# renovate: datasource=go depName=github.com/caddy-dns/namecheap
ARG CADDY_DNS_NAMECHEAP_VERSION=1.0.0
# renovate: datasource=go depName=github.com/caddy-dns/godaddy
ARG CADDY_DNS_GODADDY_VERSION=1.2.0
# renovate: datasource=go depName=github.com/caddy-dns/hetzner/v2
ARG CADDY_DNS_HETZNER_VERSION=2.0.1
# renovate: datasource=go depName=github.com/caddy-dns/vultr
ARG CADDY_DNS_VULTR_VERSION=0.0.0-20250723121531-55bf3e9768be
# renovate: datasource=go depName=github.com/caddy-dns/dnsimple
ARG CADDY_DNS_DNSIMPLE_VERSION=0.0.0-20260303131243-0433343c5610
# renovate: datasource=go depName=github.com/caddy-dns/ovh
ARG CADDY_DNS_OVH_VERSION=1.1.0
# renovate: datasource=go depName=github.com/caddy-dns/gandi
ARG CADDY_DNS_GANDI_VERSION=1.1.0
# renovate: datasource=go depName=github.com/caddy-dns/linode
ARG CADDY_DNS_LINODE_VERSION=0.8.0
# renovate: datasource=go depName=github.com/caddy-dns/porkbun
ARG CADDY_DNS_PORKBUN_VERSION=0.3.1
# renovate: datasource=go depName=github.com/caddy-dns/netlify
ARG CADDY_DNS_NETLIFY_VERSION=1.2.0
# renovate: datasource=go depName=github.com/caddy-dns/desec
ARG CADDY_DNS_DESEC_VERSION=1.1.0
# renovate: datasource=go depName=github.com/caddy-dns/scaleway
ARG CADDY_DNS_SCALEWAY_VERSION=0.2.2
# renovate: datasource=go depName=github.com/caddy-dns/duckdns
ARG CADDY_DNS_DUCKDNS_VERSION=0.5.0
# renovate: datasource=go depName=github.com/caddy-dns/dnsmadeeasy
ARG CADDY_DNS_DNSMADEEASY_VERSION=1.2.0
# renovate: datasource=go depName=github.com/caddy-dns/namedotcom
ARG CADDY_DNS_NAMEDOTCOM_VERSION=0.1.2
# renovate: datasource=go depName=github.com/caddy-dns/namesilo
ARG CADDY_DNS_NAMESILO_VERSION=0.0.0-20260219111433-e646346d8db8
# renovate: datasource=go depName=github.com/caddy-dns/rfc2136
ARG CADDY_DNS_RFC2136_VERSION=1.0.0
# renovate: datasource=go depName=github.com/caddy-dns/powerdns
ARG CADDY_DNS_POWERDNS_VERSION=1.0.2
# renovate: datasource=go depName=github.com/caddy-dns/inwx
ARG CADDY_DNS_INWX_VERSION=0.4.1
# renovate: datasource=go depName=github.com/caddy-dns/loopia
ARG CADDY_DNS_LOOPIA_VERSION=1.0.1
# Pinned to v1.0.0, NOT the nominally-latest v2.0.3: upstream tagged v2.x
# without moving go.mod's module path to /v2 (violates Go's semantic import
# versioning), so `--with .../transip@v2.0.3` fails to resolve. Re-verify
# upstream before ever bumping past v1.x — see spec §2.3 Gotcha 2.
# renovate: datasource=go depName=github.com/caddy-dns/transip
ARG CADDY_DNS_TRANSIP_VERSION=1.0.0
# renovate: datasource=go depName=github.com/caddy-dns/exoscale
ARG CADDY_DNS_EXOSCALE_VERSION=1.0.0
# renovate: datasource=go depName=github.com/caddy-dns/civo
ARG CADDY_DNS_CIVO_VERSION=0.0.0-20240512163441-e2766c887ff5
# renovate: datasource=go depName=github.com/caddy-dns/bunny
ARG CADDY_DNS_BUNNY_VERSION=1.2.0
# renovate: datasource=go depName=github.com/caddy-dns/vercel
ARG CADDY_DNS_VERCEL_VERSION=0.0.2
```

**B. Redeclare inside `caddy-inline` stage** — insert immediately after `ARG CADDY_RATELIMIT_VERSION` (before `# renovate: datasource=go depName=github.com/caddyserver/xcaddy` / `ARG XCADDY_VERSION=0.4.7`):

```dockerfile
ARG CADDY_DNS_CLOUDFLARE_VERSION
ARG CADDY_DNS_ROUTE53_VERSION
ARG CADDY_DNS_DIGITALOCEAN_VERSION
ARG CADDY_DNS_GOOGLECLOUDDNS_VERSION
ARG CADDY_DNS_AZURE_VERSION
ARG CADDY_DNS_NAMECHEAP_VERSION
ARG CADDY_DNS_GODADDY_VERSION
ARG CADDY_DNS_HETZNER_VERSION
ARG CADDY_DNS_VULTR_VERSION
ARG CADDY_DNS_DNSIMPLE_VERSION
ARG CADDY_DNS_OVH_VERSION
ARG CADDY_DNS_GANDI_VERSION
ARG CADDY_DNS_LINODE_VERSION
ARG CADDY_DNS_PORKBUN_VERSION
ARG CADDY_DNS_NETLIFY_VERSION
ARG CADDY_DNS_DESEC_VERSION
ARG CADDY_DNS_SCALEWAY_VERSION
ARG CADDY_DNS_DUCKDNS_VERSION
ARG CADDY_DNS_DNSMADEEASY_VERSION
ARG CADDY_DNS_NAMEDOTCOM_VERSION
ARG CADDY_DNS_NAMESILO_VERSION
ARG CADDY_DNS_RFC2136_VERSION
ARG CADDY_DNS_POWERDNS_VERSION
ARG CADDY_DNS_INWX_VERSION
ARG CADDY_DNS_LOOPIA_VERSION
ARG CADDY_DNS_TRANSIP_VERSION
ARG CADDY_DNS_EXOSCALE_VERSION
ARG CADDY_DNS_CIVO_VERSION
ARG CADDY_DNS_BUNNY_VERSION
ARG CADDY_DNS_VERCEL_VERSION
```

**C. `--with` lines** — append to the existing `xcaddy build` invocation, immediately after the existing `--with github.com/mholt/caddy-ratelimit@v${CADDY_RATELIMIT_VERSION} \` line and before `--output /tmp/caddy-initial;`:

```dockerfile
            --with github.com/caddy-dns/cloudflare@v${CADDY_DNS_CLOUDFLARE_VERSION} \
            --with github.com/caddy-dns/route53@v${CADDY_DNS_ROUTE53_VERSION} \
            --with github.com/caddy-dns/digitalocean@v${CADDY_DNS_DIGITALOCEAN_VERSION} \
            --with github.com/caddy-dns/googleclouddns@v${CADDY_DNS_GOOGLECLOUDDNS_VERSION} \
            --with github.com/caddy-dns/azure@v${CADDY_DNS_AZURE_VERSION} \
            --with github.com/caddy-dns/namecheap@v${CADDY_DNS_NAMECHEAP_VERSION} \
            --with github.com/caddy-dns/godaddy@v${CADDY_DNS_GODADDY_VERSION} \
            --with github.com/caddy-dns/hetzner/v2@v${CADDY_DNS_HETZNER_VERSION} \
            --with github.com/caddy-dns/vultr@v${CADDY_DNS_VULTR_VERSION} \
            --with github.com/caddy-dns/dnsimple@v${CADDY_DNS_DNSIMPLE_VERSION} \
            --with github.com/caddy-dns/ovh@v${CADDY_DNS_OVH_VERSION} \
            --with github.com/caddy-dns/gandi@v${CADDY_DNS_GANDI_VERSION} \
            --with github.com/caddy-dns/linode@v${CADDY_DNS_LINODE_VERSION} \
            --with github.com/caddy-dns/porkbun@v${CADDY_DNS_PORKBUN_VERSION} \
            --with github.com/caddy-dns/netlify@v${CADDY_DNS_NETLIFY_VERSION} \
            --with github.com/caddy-dns/desec@v${CADDY_DNS_DESEC_VERSION} \
            --with github.com/caddy-dns/scaleway@v${CADDY_DNS_SCALEWAY_VERSION} \
            --with github.com/caddy-dns/duckdns@v${CADDY_DNS_DUCKDNS_VERSION} \
            --with github.com/caddy-dns/dnsmadeeasy@v${CADDY_DNS_DNSMADEEASY_VERSION} \
            --with github.com/caddy-dns/namedotcom@v${CADDY_DNS_NAMEDOTCOM_VERSION} \
            --with github.com/caddy-dns/namesilo@v${CADDY_DNS_NAMESILO_VERSION} \
            --with github.com/caddy-dns/rfc2136@v${CADDY_DNS_RFC2136_VERSION} \
            --with github.com/caddy-dns/powerdns@v${CADDY_DNS_POWERDNS_VERSION} \
            --with github.com/caddy-dns/inwx@v${CADDY_DNS_INWX_VERSION} \
            --with github.com/caddy-dns/loopia@v${CADDY_DNS_LOOPIA_VERSION} \
            --with github.com/caddy-dns/transip@v${CADDY_DNS_TRANSIP_VERSION} \
            --with github.com/caddy-dns/exoscale@v${CADDY_DNS_EXOSCALE_VERSION} \
            --with github.com/caddy-dns/civo@v${CADDY_DNS_CIVO_VERSION} \
            --with github.com/caddy-dns/bunny@v${CADDY_DNS_BUNNY_VERSION} \
            --with github.com/caddy-dns/vercel@v${CADDY_DNS_VERCEL_VERSION} \
```

**D. No Stage 2 `go get` patch lines added** — see §2.5. Revisit only if the build validation gate surfaces a real transitive CVE/conflict.

**E. No change to `CHARON_TOOLCHAIN_TAG`/`CHARON_TOOLCHAIN_DIGEST`** — owned by CI's `sync-pin-on-pr` job, see §2.6.

### 4.3 `.github/renovate.json` diff plan

Append 30 entries to `customManagers` (after the existing `caddy-ratelimit`-style entries, before the closing `]`), one per module, following the exact existing shape. Example for `cloudflare` (all 30 follow this template with the module's own depName/ARG substituted per §4.1):

```json
{
  "customType": "regex",
  "description": "Track caddy-dns/cloudflare DNS-01 provider module version in Dockerfile",
  "managerFilePatterns": ["/^Dockerfile$/"],
  "matchStrings": ["ARG CADDY_DNS_CLOUDFLARE_VERSION=(?<currentValue>[^\\s]+)"],
  "depNameTemplate": "github.com/caddy-dns/cloudflare",
  "datasourceTemplate": "go",
  "versioningTemplate": "semver"
}
```

For `hetzner`, `depNameTemplate` is `github.com/caddy-dns/hetzner/v2` (the real module path, matching §4.1 exactly, so Renovate resolves version updates against the correct module). All other 29 use their bare `github.com/caddy-dns/<name>` path from §4.1 col 4.

No changes to `ignoreDeps`, `customManagers` ordering conventions, or any other top-level Renovate key.

### 4.4 `docs/guides/dns-providers.md` rewrite plan

Current text (§"Supported DNS Providers" → "Built-in Providers" table + intro bullet "Built-in providers — Compiled into Charon (Cloudflare, Route 53, etc.)") is inaccurate on two axes: (a) implies compiled ⇒ these are the only ones Charon will ever support, and (b) is currently **false** (nothing is compiled in today). Replace with three explicit tiers:

1. **"Fully Supported (Charon UI)"** — the 10 existing providers, unchanged table content (type strings, setup-guide links), but drop the false "Compiled into Charon" framing from the intro bullet; replace with something like: *"Fully supported — usable end-to-end from the Charon UI (credential form, validation, wildcard host attachment) and its DNS module is compiled into the shipped Caddy binary."*
2. **"Compiled, Not Yet Usable from the UI"** — new table listing the 20 module-only providers (name, Caddy module doc link `https://caddyserver.com/docs/modules/dns.providers.<name>` where it exists upstream, and a one-line note: *"Module is compiled into Charon's Caddy binary but has no credential form or API type yet — tracked in [#1374](https://github.com/Wikid82/charon/issues/1374)."* Confirm before publishing that no advanced-config escape hatch exists (§2.7) so this framing stays accurate — do not imply a workaround exists.
3. **"Not Supported"** — one sentence: *"Any DNS provider not listed above requires its `github.com/caddy-dns/*` module to be added to the Dockerfile's xcaddy build first — see [#1374](https://github.com/Wikid82/charon/issues/1374) for the current tracked backlog, or open a new feature request."*

Also update the `is_built_in` API-response section: clarify that `is_built_in: true` reflects **Charon backend/UI registration** (the 10), not raw Caddy-module compilation — since the 20 module-only providers have no `ProviderPlugin`/API type at all, they will never appear in `GET /api/v1/dns-providers/types` regardless of `is_built_in`, and that's fine/expected — no code change needed there, just make sure the docs don't imply that endpoint reflects Caddy-module compilation status.

---

## 5. Acceptance Criteria

- [ ] `caddy list-modules` (or `caddy list-modules --skip-standard` — either shows the `dns.providers.*` set) on a from-source (`CADDY_BUILDER_SRC=caddy-inline`) build of this branch lists all 30 `dns.providers.<name>` module IDs from §4.1 (using each module's registered Caddy short name, which may differ from the Go package folder name for a couple of these — verify each ID against the module's own `RegisterModule`/`CaddyModule()` call if a mismatch is found during the build gate, and correct the table/docs name, not the module).
- [ ] A wildcard proxy host with a Cloudflare (or any of the other 9 existing) DNS provider attached, saved through the UI on this build, results in a successful Caddy `/load` (no `unknown module` 400, no full-config rollback) — validated by the Commit 1 Playwright spec once un-`fixme`'d in Commit 4.
- [ ] `docs/guides/dns-providers.md` no longer states or implies that DNS providers beyond the 10 are "compiled into Charon" without qualification, and clearly separates the three tiers from §4.4.
- [ ] `.github/renovate.json` passes best-effort local validation (`npx --yes renovate-config-validator@latest .github/renovate.json`, or equivalent JSON-schema/lint check if that package is unavailable in the sandbox) with the 30 new entries present and no duplicate `matchStrings`.
- [ ] `scripts/toolchain-key.sh`'s `arg_re` whitelist includes all 30 `CADDY_DNS_*_VERSION` names and `SCHEMA_VERSION` is bumped to `3` (§2.6, §6 Commit 2b); a value-only edit to any one of the 30 new ARGs' defaults measurably changes the script's printed hash (validated per Commit 2b's gate item 2).
- [ ] `ARCHITECTURE.md`'s "Prebuilt toolchain image" → "Freshness guard" bullet (`ARCHITECTURE.md:1231-1234`) reflects the enlarged pinned-plugin set and no longer says just "the two pinned xcaddy plugins" (§2.8, §6 Commit 2b).
- [ ] `hadolint` (`lefthook run pre-commit` invokes `docker run --rm -i hadolint/hadolint < Dockerfile`) reports no new findings introduced by the Dockerfile diff.
- [ ] Full DoD per `CLAUDE.md` — see §6 per-commit gates; this PR carries no backend/frontend Go/TS code changes, so the Go/TS unit-test coverage gates (`scripts/go-test-coverage.sh`, `scripts/frontend-test-coverage.sh`) are unaffected/no-op for this change, not skipped — confirm they still pass at their pre-existing baseline (no regression), since CI runs them unconditionally regardless of change type.

---

## 6. Commit Slicing Strategy

**Decision:** Single PR, `fix/dns-provider-caddy-modules` → `development`, five ordered commits. This is a `fix`-scale correction to already-advertised functionality (the 10 existing providers) that also happens to compile in 20 module-only providers as groundwork for #1374 — it stays one PR because it's one coherent fix to one Dockerfile recipe with one docs update; splitting the Dockerfile change from its Renovate tracking or its docs correction would leave any individual commit either unbuildable-but-untracked or tracked-but-undocumented.

This PR has **no backend or frontend application-code changes**, so the CLAUDE.md-suggested "backend → frontend" commit slots collapse into the infra changes below. The E2E-first and hardening/docs-last bookends still apply per CLAUDE.md's suggested sequence.

### Commit 1 — E2E spec for the fixed behavior (`test.fixme`)

- **Scope:** New Playwright spec asserting that saving a wildcard proxy host with a built-in DNS provider (e.g. Cloudflare, using a test/mock credential fixture consistent with existing DNS-provider E2E fixtures) succeeds without a Caddy `/load` rollback error toast. Mark the assertion `test.fixme(true, 'blocked on #1361 — dns.providers.* module not compiled in yet')` since it will fail against the current (unfixed) toolchain image.
- **Files:** new spec under `tests/` (playwright-dev should confirm the exact existing DNS-provider E2E directory/naming convention before creating the file — do not invent a new top-level test directory).
- **Dependencies:** none.
- **Validation gate:** `npx playwright test <new-spec-file> --project=firefox` from repo root — expect the run to report the test as `fixme`/skipped, not failed; confirm the spec file type-checks and lints cleanly (`lefthook run pre-commit` scoped to the new file). No backend build required yet.

### Commit 2 — Dockerfile: compile the 30 DNS-01 provider modules

- **Scope:** §4.2 parts A, B, C (30 `ARG` declarations × 2 locations + 30 `--with` lines). No Stage 2 patch lines unless the validation gate below finds a real conflict (§2.5).
- **Files:** `Dockerfile` only.
- **Dependencies:** none (independent of Commit 1; ordered after it only per the suggested E2E-first sequence).
- **Validation gate (the substantive one for this PR):**
  1. `hadolint` clean: `docker run --rm -i hadolint/hadolint < Dockerfile` (or `lefthook run pre-commit`, which includes this per `lefthook.yml:197-199`).
  2. **Real Docker build**, using the from-source fallback path so it doesn't depend on the (not-yet-republished) prebuilt toolchain image (§2.6):
     ```bash
     make build-offline
     # or equivalently:
     docker build --build-arg CADDY_BUILDER_SRC=caddy-inline \
                   --build-arg CROWDSEC_BUILDER_SRC=crowdsec-inline \
                   -t charon:dns-modules-check .
     ```
  3. Extract and run the built `caddy` binary's `list-modules` (e.g. `docker run --rm --entrypoint caddy charon:dns-modules-check list-modules`) and grep for `dns.providers.` — **assert all 30 expected module short names from §4.1 are present**. Any mismatch between an expected short name and the module's actual registered Caddy name must be corrected in this commit's table/docs before proceeding (see Acceptance Criteria caveat).
  4. Confirm `scripts/verify-toolchain-pin.sh` **fails locally** with a staleness message referencing the recomputed `caddy-crowdsec-<hex>` key — this is the *expected*, correct local result (§2.6); do not attempt to "fix" it by hand-editing `CHARON_TOOLCHAIN_TAG`/`CHARON_TOOLCHAIN_DIGEST` locally. Note this expected-failure state in the PR description so reviewers aren't surprised when they see it red before `sync-pin-on-pr` runs in CI.
  5. This commit is a Dockerfile-only change with no Go/TS source touched — the GORM security scan (§ DoD 1.5) and Playwright/coverage gates are not triggered/applicable; CodeQL/Trivy still run in CI unconditionally per the standing DoD, no local run required (Dockerfile-only, `fix`-scoped, no new app code path).

### Commit 2b — Fix `scripts/toolchain-key.sh` whitelist gap + update `ARCHITECTURE.md`

- **Scope (blocking, required — see §2.6 for the full root-cause analysis):**
  1. Add all 30 `CADDY_DNS_<PROVIDER>_VERSION` names (exact names from §4.1 col 4) to the `arg_re` whitelist regex at `scripts/toolchain-key.sh:68`, in the same alternation style already used there, immediately following the existing `CADDY_RATELIMIT_VERSION` entry.
  2. Bump `SCHEMA_VERSION` at `scripts/toolchain-key.sh:29` from `2` to `3`, and add a `rev-3:` comment above it following the exact precedent of the existing `rev-2:` comment (`scripts/toolchain-key.sh:27-28`), e.g. `# rev-3: added the 30 caddy-dns/* provider module version ARGs to the hashed input set.`
  3. Update `ARCHITECTURE.md:1233` — change "(incl. the two pinned xcaddy plugins)" to reflect the enlarged set, e.g. "(incl. the pinned xcaddy plugins — 2 security/observability plugins plus 30 `caddy-dns/*` DNS-01 provider modules, see `docs/plans/current_spec.md` §4.1)". Do not reword anything else in that bullet or section — see §2.8 for why this is the only `ARCHITECTURE.md` change needed.
- **Files:** `scripts/toolchain-key.sh`, `ARCHITECTURE.md`.
- **Dependencies:** Commit 2 (the `ARG` names being added to `arg_re` must already exist in `Dockerfile`, and this commit's expected-key-change assertion below is meaningless without Commit 2's stage-body edit already present).
- **Validation gate:**
  1. Re-run `scripts/toolchain-key.sh` before and after this commit's edit against the Commit-2-modified `Dockerfile` and confirm the printed `caddy-crowdsec-<hex>` key **changes** as a result of this commit alone (proves the whitelist edit + `SCHEMA_VERSION` bump actually altered the hash inputs, not just cosmetic).
  2. Manually simulate the steady-state bug this commit fixes: edit one of the 30 new `ARG CADDY_DNS_*_VERSION=` default values in a scratch copy of the post-Commit-2 `Dockerfile`, re-run `scripts/toolchain-key.sh` against it, and confirm the key **changes** in response to that value edit (this is the regression check for the exact bug described in §2.6 — before this commit, that edit would *not* change the key; after, it must).
  3. `shellcheck scripts/toolchain-key.sh` clean (part of `lefthook run pre-commit`).
  4. `scripts/verify-toolchain-pin.sh` still correctly fails locally at this point (same expected-red state as Commit 2's gate item 4 — this commit changes hash *inputs*, not the fact that the pin is stale post-Commit-2; `sync-pin-on-pr` resolves both in the same CI cycle).
  5. Confirm no other line in `ARCHITECTURE.md` changed besides the one sentence at line 1233 (`git diff ARCHITECTURE.md` should show a single-line change).

### Commit 3 — Renovate tracking for the 30 new pins

- **Scope:** §4.3 — 30 new `customManagers` entries in `.github/renovate.json`.
- **Files:** `.github/renovate.json` only.
- **Dependencies:** Commit 2 (the `ARG` names being regex-matched must already exist in `Dockerfile`, or the manager entries are dead/unverifiable). Ordered after Commit 2b but has no direct dependency on it.
- **Validation gate:** `npx --yes renovate-config-validator@latest .github/renovate.json` (or repo-preferred equivalent if that package isn't available/allowed in the sandbox — note in the PR if so); spot-check at least the `hetzner` entry's `depNameTemplate` resolves to `github.com/caddy-dns/hetzner/v2` (not the bare path) and the `transip` entry's tracked `ARG` matches the `1.0.0` pin with its cautionary comment intact from Commit 2. **Minor, worth a spot-check:** the 5 pseudo-versioned entries (`digitalocean`, `vultr`, `dnsimple`, `namesilo`, `civo`, §2.3 Gotcha 3) use `versioningTemplate: "semver"` against Go pseudo-version strings (`0.0.0-<timestamp>-<sha>`), which is untested in this repo's Renovate config for pseudo-versions specifically — don't assume the existing tagged-release pattern transfers cleanly. As part of this gate, either run the config validator against a fixture `currentValue` matching one of the five pseudo-version strings and confirm it parses without error, or do a `renovate --dry-run=full` (or equivalent local dry-run, sandbox permitting) scoped to `Dockerfile` and visually confirm Renovate resolves at least one of the five pseudo-version ARGs to a sane candidate update rather than erroring or silently ignoring it. Note the result (pass, or "could not verify locally, deferring to first real Renovate run post-merge") in the PR description either way.

### Commit 4 — Docs rewrite + enable the E2E spec + hardening

- **Scope:**
  1. `docs/guides/dns-providers.md` rewrite per §4.4 (three-tier tables, `#1374` links, `is_built_in` clarification).
  2. Remove the `test.fixme(...)` from Commit 1's spec now that Commit 2 makes the underlying behavior work.
  3. Any cleanup surfaced by the Commit 2 build gate (unused Stage 2 patch stub, stray debug output, etc. — expected to be none, but this is the designated slot per the DoD's "Clean Up" step).
- **Files:** `docs/guides/dns-providers.md`, the Commit 1 spec file (fixme removal).
- **Dependencies:** Commits 1, 2, 2b, and 3 all merged/present on the branch.
- **Validation gate:**
  1. `npx playwright test <spec-file> --project=firefox` — must now **pass** (not fixme/skip) against a locally built image from Commit 2's validated recipe (`docker build --build-arg CADDY_BUILDER_SRC=caddy-inline ...` per Commit 2, or the CI-side prebuilt-toolchain path once `sync-pin-on-pr` has landed its digest bump on the branch).
  2. Full DoD pass per `CLAUDE.md` §"Task Completion Protocol" before marking the PR ready: `scripts/local-patch-report.sh`, `lefthook run pre-commit`, `make lint-fast`, backend/frontend `go build`/`npm run build`, `npm run type-check` — all expected to be unaffected no-ops given zero Go/TS source changes, but must be run and confirmed green per the standing DoD (not skipped merely because "nothing should have changed").

### Rollback / contingency (PR-level)

- If the Docker build validation gate (Commit 2) finds a real transitive-dependency conflict for any of the 30 modules, do **not** silently drop that module from the `--with` list without updating §4.1's table and `docs/guides/dns-providers.md`'s tier-3 "Not Supported" note to explain why — file a quick follow-up note/issue so the gap isn't invisible.
- Commit 2b (`scripts/toolchain-key.sh` whitelist + `ARCHITECTURE.md`) is not optional/deferrable follow-up work — without it, future Renovate bumps of the 30 new `CADDY_DNS_*_VERSION` ARGs silently pass `verify-toolchain-pin.sh` without triggering a toolchain rebuild (§2.6). If Commit 2b is somehow dropped from this PR during review, that is a blocking regression to flag, not a nice-to-have to defer.
- If CI's `sync-pin-on-pr` job does not land its digest-bump commit on the branch within a reasonable window after Commit 2 is pushed (e.g. workflow failure, GHCR auth issue), do not hand-roll the pin — escalate via the existing `docs/ci/toolchain-image.md` runbook rather than working around `verify-toolchain-pin.sh`.
- If the whole PR needs to be reverted post-merge, a single `git revert` of the merge commit is sufficient — no data migrations, no backend/frontend deploy coordination, no feature flags involved. The only follow-on cleanup is `sync-pin-on-pr`/`open-bump-pr` naturally re-converging `CHARON_TOOLCHAIN_TAG`/`CHARON_TOOLCHAIN_DIGEST` back down on the next toolchain rebuild after the revert lands (no manual pin-revert needed).

---

## 7. Risks

| Risk | Likelihood | Impact | Mitigation |
|---|---|---|---|
| `transip` upstream eventually fixes its module path and Renovate proposes `v2.x` before this spec's caution is remembered | Medium (upstream-dependent, unknown timeline) | Build break if merged blindly | ARG comment in Dockerfile (§4.2A) explicitly warns; PR reviewer checklist item |
| One of the 5 pseudo-versioned modules (digitalocean/vultr/dnsimple/namesilo/civo) force-pushes or rewrites its default branch, invalidating the pinned commit SHA | Low | `go get`/xcaddy fails to resolve that pseudo-version at next cold build (module proxy cache may still serve it if already fetched once — GOPROXY is typically immutable-cache by design, so this is low-severity even if it happens) | Toolchain image is content-addressed and only rebuilt on recipe change or daily schedule; a resolution failure surfaces immediately and loudly in `toolchain-image.yml`, not silently |
| Adding 30 modules meaningfully increases `xcaddy build` time and/or final binary size | Medium | Slower daily toolchain rebuild, marginally larger image | Accepted tradeoff per explicit user decision (curated 30 vs. all ~84); not a defect to fix in this PR |
| Existing "atomic `/load` rollback poisons unrelated saves" behavior (§1.1) remains after this fix — a **future** bad DNS-provider save (e.g. wrong credentials) still rolls back everything | Ongoing (pre-existing, unchanged by this PR) | Same blast-radius issue as today, just no longer triggered by *this* specific missing-module cause | Explicitly out of scope (§1.3); worth a separate follow-up issue for `internal/caddy/manager.go` partial-apply/validation-before-apply, not bundled into this fix |
| Renovate noise from the 5 pseudo-version trackers | Medium | Reviewer fatigue on low-value PRs | Accepted (§2.3); can be tuned later (e.g. `minimumReleaseAge` per-package) if it proves too noisy in practice — not addressed preemptively here |
| `scripts/toolchain-key.sh`'s `arg_re` whitelist doesn't auto-discover new ARGs, so a future Renovate bump of one of the 30 new pins could silently fail to trigger a toolchain rebuild | Was High/certain without mitigation | Stale toolchain image ships outdated DNS provider module code indefinitely, undetected | Mitigated in this PR by Commit 2b (§2.6, §6) — all 30 names added to `arg_re` before merge, so this risk is closed for these 30 ARGs specifically. Residual: the same class of gap could recur for any *future* plugin ARG added without a matching `arg_re`/`SCHEMA_VERSION` update — worth a reviewer checklist item, not addressed structurally (e.g. via auto-discovery) in this PR |
| Renovate `versioningTemplate: "semver"` against Go pseudo-versions for the 5 no-tag modules is untested in this repo's Renovate config | Low-Medium | Renovate silently fails to propose updates, or errors, for those 5 entries specifically | Spot-checked per Commit 3's validation gate (§6); if the local check can't be run in-sandbox, deferred to observing the first real Renovate run post-merge, noted explicitly in the PR rather than assumed |

---

## 8. Example Commit Messages (no session attribution)

```
fix: compile 30 DNS-01 challenge provider modules into the Caddy build

Charon's xcaddy build never included any github.com/caddy-dns/* module,
so every "built-in" DNS provider (Cloudflare, Route 53, etc.) failed at
Caddy's /load with "unknown module: dns.providers.<name>", rolling back
the entire config including unrelated saves. Adds pinned ARGs + --with
lines for the 10 existing built-in providers plus 20 additional
Caddy-module-only providers (backend/UI tracked separately in #1374).

Fixes #1361
```

```
fix: track new caddy-dns plugin ARGs in the toolchain freshness hash

scripts/toolchain-key.sh derives its content-addressed toolchain tag from
a hardcoded ARG whitelist (arg_re), not every ARG in the Dockerfile.
Without this change, a future Renovate bump of any of the 30 new
CADDY_DNS_*_VERSION ARGs would go undetected by verify-toolchain-pin.sh,
and the prebuilt toolchain image would never rebuild for that bump.
Adds all 30 names to arg_re and bumps SCHEMA_VERSION per the script's own
rev-2 precedent. Also updates ARCHITECTURE.md's freshness-guard
description to reflect the enlarged pinned-plugin set.
```

```
chore: track caddy-dns provider module versions via Renovate

Adds a regex customManager per module, following the existing
caddy-security/caddy-geoip2 pattern.
```

```
docs: correct dns-providers guide to reflect actual compiled/UI support

Splits providers into fully-supported (Charon UI), compiled-but-UI-less
(tracked in #1374), and unsupported tiers instead of a blanket
"compiled into Charon" claim that was inaccurate for every provider
before this fix.
```
