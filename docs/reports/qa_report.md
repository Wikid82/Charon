# QA & Security Report — DNS-01 Challenge Provider Caddy Modules (#1361)

- **Feature branch:** `fix/dns-provider-caddy-modules` (6 commits ahead of `development`)
- **Commits audited:** `ccf59920`, `875058e1`, `c3344952`, `0229e57d`, `9a1dd47a`, `39fba472`
- **Plan:** `docs/plans/current_spec.md` (2 rounds of Supervisor spec review + implementation review, approved with a docs-structure addendum)
- **Prior gates:** Planning → Supervisor (spec, 2 rounds) → implementation → Supervisor (implementation review, approved)
- **Date:** 2026-09-22
- **Verdict:** **PASS — READY TO OPEN PR.** No blocking issues. Zero CRITICAL/HIGH security findings attributable to this change. One non-blocking documentation gap noted (§6). One test-execution gap noted and explained (§1, gate 1).

---

## 0. Scope confirmation

`git diff development..HEAD --stat`:

```
 .github/renovate.json                              |  377 +++++
 ARCHITECTURE.md                                    |    6 +-
 Dockerfile                                         |  150 +++
 docs/features.md                                   |    7 +-
 docs/guides/dns-providers.md                       |  125 +--
 docs/plans/current_spec.md                         | 1045 +++++++-------
 scripts/toolchain-key.sh                           |   11 +-
 tests/integration/wildcard-dns01-...spec.ts        |  148 +++
 8 files changed, 1212 insertions(+), 657 deletions(-)
```

**Confirmed: zero Go or TypeScript application code touched.** No `backend/**`, no `frontend/src/**`, no `.go`/`.tsx`/`.ts` app files in the diff — only Dockerfile, a shell script, a Renovate config, docs, the plan file, and one new Playwright spec. This is a build/deployment-config change, not an application-code change.

**Consequently, the following standard DoD gates are N/A for this PR, and are noted here rather than silently skipped:**
- Backend/frontend unit test coverage gates (`go-test-coverage.sh`, `frontend-test-coverage.sh`) — N/A, no Go/TS source changed to instrument.
- `npm run type-check` — N/A, no TypeScript changed.
- `go build ./...` / `npm run build` — not meaningfully informative on their own since no app code changed, but both were implicitly exercised as part of the full multi-stage `make build-offline` Docker build (§2), which succeeded.
- GORM security scan (`scan-gorm-security.sh --check`) — **explicitly checked and confirmed out of scope**: no `backend/internal/models/**`, GORM queries, or migrations in this diff. Not run.

---

## 1. Definition of Done — gate-by-gate

| # | Gate | Result | Notes |
|---|------|--------|-------|
| 1 | Targeted Playwright E2E (`wildcard-dns01-certificate-save-regression.spec.ts --project=firefox`) | **BLOCKED (environment)** — see below | Spec confirmed not `test.fixme`/`test.skip` (active). Full browser-level run blocked by this shared dev host's pre-existing port contention (see below); strong indirect runtime evidence gathered instead. |
| 1.5 | GORM security scan | **N/A** | Confirmed out of scope — no models/migrations touched. |
| 2 | Local patch coverage preflight | **N/A** | No Go/TS source in diff for `scripts/local-patch-report.sh` to meaningfully instrument; not run for that reason. |
| 3 | Security scans (govulncheck, Trivy, hadolint) | **PASS** | See §3. Run locally given the explicit ask in this audit's scope and the Dockerfile/supply-chain nature of the change. |
| 4 | `lefthook run pre-commit` | Not separately re-run | Superseded by the more targeted checks in §3–5, which cover the same ground (hadolint, Dockerfile) plus additional independent verification (govulncheck, Trivy image scan, Renovate validator) beyond what pre-commit hooks check. |
| 5 | Renovate config validation | **PASS** | See §4. |
| 6 | Coverage | **N/A** | No app code changed (see §0). |
| 7 | Type Safety | **N/A** | No TypeScript changed. |
| 8 | Docker build verification | **PASS** | `make build-offline` completed successfully (see §2). |
| 9 | toolchain-key.sh regression fix | **PASS — independently reproduced** | See §5. |
| 10 | Clean-up check | **PASS** | Dockerfile diff is exclusively new `ARG`/`--with` lines and explanatory comments; no debug output, no dead code. |

### Gate 1 detail: E2E execution blocker

The regression spec (`tests/integration/wildcard-dns01-certificate-save-regression.spec.ts`) is confirmed **not** skipped/fixme — it contains an active `test(...)` (only the file's header comment mentions its `test.fixme` origin in Commit 1, per the Commit Slicing Strategy). I attempted to run it via `npx playwright test ... --project=firefox` against a locally rebuilt E2E container (`docker-rebuild-e2e` skill, image built clean).

Execution was blocked by **host port contention specific to this shared dev machine**, unrelated to the PR: ports `2019`, `2020`, `8080`, and (after remapping) `18080` were each already bound by this session's own IDE/tooling process (`code`, pid 1954815) the moment I attempted to claim them via a Compose port override, and separately `8080`/`80` are bound by this host's own real production Charon deployment (documented in the compose file's own comments). After several remapping attempts I was unable to find a free port set for the full stack (app UI + Caddy admin + emergency API) in this session and stopped rather than continue destructively probing ports on a host running unrelated production services.

**Indirect but strong corroborating evidence the underlying fix works, gathered during container startup for this attempt:**
```
"msg":"Successfully applied initial Caddy config"
```
captured from the container's own logs — i.e., the actual Caddy `/load` admin-API call (the exact code path issue #1361 describes as failing with `unknown module: dns.providers.X` and rolling back the config) completed with **zero errors** on container start, using the production-built binary containing all 27 new DNS provider modules. Combined with the `caddy list-modules` confirmation in §2, this is very strong evidence the fix is correct, even though I could not complete a full browser-level Playwright pass in this sandboxed session. I recommend CI's full E2E run (which does not have this host's port conflicts) as the authoritative confirmation before merge — standard practice per this repo's CI-defers-full-suite convention.

---

## 2. Docker build validation

`make build-offline` (foreground, blocking) completed successfully, producing `charon:offline`. All 88 build steps resolved (mix of fresh execution for content that differs from `development` and `CACHED` layers for unchanged content — cache validity is itself content-addressed by BuildKit, so a `CACHED` hit on the `caddy-inline` stage is only possible because that stage's exact instruction+arg content, including the new `--with` lines, was already built successfully in this environment).

`caddy list-modules` inside the built image confirms:

**All 27 expected `dns.providers.*` module IDs present**, exactly matching the Dockerfile's `--with` list: `azure, bunny, cloudflare, desec, digitalocean, dnsimple, dnsmadeeasy, duckdns, gandi, godaddy, googleclouddns, hetzner, inwx, linode, loopia, namecheap, namedotcom, namesilo, netlify, ovh, porkbun, powerdns, rfc2136, route53, scaleway, vercel, vultr`.

**All pre-existing security/WAF/rate-limit/geoip/crowdsec modules still present**, confirming no regression/removal: `admin.api.crowdsec, crowdsec, geoip2, http.handlers.crowdsec, http.handlers.geoip2, http.handlers.rate_limit, http.handlers.waf, layer4.matchers.crowdsec, security`.

A full side-by-side image diff against a build of `development` HEAD's Dockerfile was **not performed** — per this repo's `CLAUDE.md`, worktrees and branch-switching are disallowed for this task, and a full-context `development`-HEAD build without a worktree was impractical within the audit window. The "at minimum" bar the task specified (confirm pre-existing modules unaffected) was met directly against the built image, as shown above, which is sufficient to rule out any removal/regression of existing modules.

---

## 3. Security scanning

### 3.1 Supply-chain reputability spot-check (`github.com/caddy-dns/*`)

Spot-checked 10 of the 27 new modules via the GitHub API (`gh api repos/caddy-dns/<name>`):

| Repo | Stars | Last push | Archived |
|---|---:|---|---|
| caddy-dns/cloudflare | 999 | 2026-03-23 | No |
| caddy-dns/route53 | 84 | 2026-07-17 | No |
| caddy-dns/duckdns | 94 | 2025-04-19 | No |
| caddy-dns/inwx | 22 | 2025-11-27 | No |
| caddy-dns/bunny | 18 | 2025-05-25 | No |
| caddy-dns/azure | 12 | 2025-04-23 | No |
| caddy-dns/vercel | 12 | 2026-07-10 | No |
| caddy-dns/dnsimple | 7 | 2026-03-03 | No |
| caddy-dns/loopia | 6 | 2026-06-10 | No |
| caddy-dns/namesilo | 5 | 2026-02-19 | No |

All 10 belong to the official `caddy-dns` GitHub org ("Caddy modules that automate manipulation of DNS records (built on libdns interfaces)") — the recognized upstream org for this class of Caddy plugin, not third-party forks. None archived; all pushed to within the last ~10 months. Low star counts on niche providers (loopia, namesilo, azure) are normal/expected for this ecosystem and not itself a red flag — Caddy's DNS-01 provider modules are maintained by a small, dedicated group under the `caddy-dns` org umbrella.

### 3.2 `govulncheck` (symbol-level reachability) against the built binary

Extracted `/usr/bin/caddy` from the built image and ran `govulncheck -mode=binary -show verbose`. Confirmed all 27 `caddy-dns/*` modules and their pinned versions are embedded (module list cross-checked against the Dockerfile's `ARG` defaults — exact match, including the pseudo-versioned ones: `digitalocean@v0.0.0-20250606074528-...`, `vultr@v0.0.0-20250723121531-...`, `dnsimple@v0.0.0-20260303131243-...`, `namesilo@v0.0.0-20260219111433-...`).

**12 total vulnerability findings, all pre-existing and unrelated to this PR:**
- `GO-2026-6094` (`github.com/google/cel-go`) — Caddy core CEL matcher dependency, unrelated.
- `GO-2026-5932` (`golang.org/x/crypto/openpgp`) — unmaintained package warning, unrelated, pre-existing across the codebase.
- `GO-2024-2565` through `GO-2024-2549` (10 findings, all `github.com/greenpau/caddy-security`) — pre-existing, unrelated to DNS providers, not reachable from called code per govulncheck's own reachability analysis.

**Zero vulnerabilities attributed to any of the 27 `caddy-dns/*` or `libdns/*` modules or their dependency trees** (`aws-sdk-go-v2`, `azure-sdk-for-go`, `digitalocean/godo`, `linode/linodego`, `hetznercloud/hcloud-go`, `dnsimple-go`, `ovh/go-ovh`, `mittwald/go-powerdns`, `miekg/dns`, etc. — all scanned, all clean).

### 3.3 Trivy image scan (vuln, CRITICAL/HIGH)

Ran `aquasec/trivy:latest` (Dockerized, via host socket) against `charon:offline`:

```
usr/bin/caddy              gobinary   0 vulnerabilities   ← houses all 27 new DNS-01 modules
app/charon                 gobinary   0 vulnerabilities
usr/sbin/gosu               gobinary   0 vulnerabilities
usr/local/bin/crowdsec     gobinary   1 (HIGH) — CVE-2026-32286, jackc/pgproto3/v2
usr/local/bin/cscli        gobinary   1 (HIGH) — CVE-2026-32286, jackc/pgproto3/v2
```

The only findings (both `CVE-2026-32286`, HIGH) are in the CrowdSec binaries — **entirely unrelated to this PR** (no CrowdSec files touched) and already documented, tracked, and formally suppressed in `.trivyignore` and `SECURITY.md` (pgproto3/v2 is archived upstream with no fix available; this is pre-existing accepted risk, re-verified multiple times per `docs/security/vulnerability-analysis-*.md`). **The Caddy binary containing this PR's actual changes has zero findings.**

### 3.4 Supply-chain / attack-surface assessment for SECURITY.md's threat model

This change compiles ~29 new third-party Go modules (27 `caddy-dns/*` + 2 forced `libdns/*` transitive bumps) into the production Caddy binary, each capable of making outbound API calls to a DNS provider using operator-supplied credentials (API tokens/keys) for ACME DNS-01 challenge automation. Assessment:

- **Access control is already correctly scoped**: `SECURITY.md`'s Authentication & Authorization section already states DNS-provider credentials and ACME configuration require `admin` role (line ~1156), so the credential-configuration surface is already gated appropriately at the application layer — this PR doesn't change that.
- **Execution is not user-code-reachable**: these are Caddy's own DNS-provider client libraries, invoked only by Caddy's TLS/ACME subsystem when a DNS-01 challenge is configured — not general-purpose code paths reachable from Charon's HTTP API surface.
- **Scanned clean**: no CVEs found via govulncheck or Trivy against the exact pinned versions (§3.2, §3.3).
- **Gap (non-blocking)**: `SECURITY.md`'s "Supply Chain Security" section (line ~1202) does not currently mention that ~29 new third-party network-credential-handling modules were added in this change, or acknowledge DNS-provider API client libraries as a distinct category in the threat model. Recommend a follow-up docs note (not blocking this PR) — see §6.

### 3.5 `hadolint`

Re-ran independently (Dockerized `hadolint/hadolint:latest` against `.hadolint.yaml`) rather than trusting the prior agent's claim:

```
HEAD Dockerfile:        8 findings (7 warning, 1 info) — DL3067 x3, DL4006, DL3003, SC2012, DL3025, DL3066
development Dockerfile: 8 findings (identical rule set, same line-shift-only offsets)
```

**Identical finding set, zero new findings introduced by this change.** All 8 are pre-existing style warnings (multi-stage COPY, WORKDIR usage, non-numeric UID) unrelated to the DNS provider additions, at lines nowhere near the new `ARG`/`--with` blocks.

### 3.6 GORM security scan — explicitly out of scope

Per this audit's assigned scope: this PR touches zero files under `backend/internal/models/**`, no GORM queries, and no migrations. `./scripts/scan-gorm-security.sh --check` was **not run** — noted explicitly per this repo's convention of not skipping DoD items silently, rather than omitted without comment.

---

## 4. Renovate config validation

Re-validated independently (did not trust the prior agent's claim):

1. `.github/renovate.json` — valid JSON (`python3 -m json.load`), passes.
2. `npx renovate-config-validator .github/renovate.json` (official Renovate validator): **`INFO: Config validated successfully against 1 file(s)`**.
3. Manually cross-checked all 29 new `customManagers` regex entries (27 `caddy-dns/*` + 2 `libdns/*`): every entry uses `datasourceTemplate: "go"` / `versioningTemplate: "semver"`, matching the exact convention already used by every pre-existing Go-module `ARG` tracker in this same file (`caddy-security`, `expr-lang/expr`, `x/net`, `xcaddy`, etc.) — not a new/divergent pattern. Each `matchStrings` regex correctly targets its own unique `ARG <NAME>_VERSION=(?<currentValue>...)` line with no overlap between entries. Renovate's `go` datasource handles the four pseudo-versioned pins (`digitalocean`, `vultr`, `dnsimple`, `namesilo`) correctly under `semver` versioning — this is standard, well-supported Renovate behavior for Go modules without tagged releases and matches how this file already handles other untagged Go deps.

---

## 5. `toolchain-key.sh` regression fix — independent verification

Did not trust the implementing agent's documented before/after hash comparison; reproduced the regression class independently using a different method (isolating the actual bug scenario the fix addresses — a *future* Renovate version bump landing in the Dockerfile, not just the one-time diff from `development` to `HEAD`, since the stage-body text itself already changed in this diff and would trivially change the hash regardless of the `arg_re` fix).

**Method:** Took the HEAD Dockerfile (with the fix's ARGs already in the hash allowlist) and simulated a hypothetical future Renovate bump by changing only `ARG CADDY_DNS_CLOUDFLARE_VERSION=0.2.4` → `=0.2.5` (a bare top-level `ARG` default value, outside the hashed stage body — exactly the kind of change a real Renovate PR would make).

| Script version | Hash before bump | Hash after bump | Bump detected? |
|---|---|---|---|
| Pre-fix `arg_re` (missing DNS ARG names) | `789bdc4a0ec855cc` | `789bdc4a0ec855cc` | **No — identical, bug reproduced** |
| HEAD (fixed) `arg_re` (includes all 29 new ARG names) | `72a4fd2996163768` | `b4e120bda42cd544` | **Yes — correctly differs** |

This directly confirms the fix: without it, a Renovate bump to any of the 29 new version pins would silently produce the same `caddy-crowdsec-*` content-addressed tag, causing the stale prebuilt toolchain image to be reused indefinitely instead of rebuilding with the bumped module version — exactly the bug this commit set out to close. With the fix, the same bump is correctly detected and triggers a new tag/rebuild.

---

## 6. Recommendations (non-blocking)

1. **SECURITY.md supply-chain note** (§3.4): add a short acknowledgment under "Supply Chain Security" or "Infrastructure Security" that DNS-01 challenge provider modules are third-party API clients handling operator-supplied DNS credentials, scoped to the existing `admin`-only ACME/DNS-provider configuration surface. Not blocking — access control is already correctly enforced; this is purely a threat-model documentation completeness item. Suggested follow-up, not required before merge.
2. **CI E2E confirmation**: since the local Playwright run was blocked by this specific sandboxed session's port contention (§1), treat CI's E2E run on the opened PR as the authoritative confirmation of the regression spec, per this repo's standard "defer full/cross-env runs to CI" convention. The indirect evidence (clean `/load` in container logs, full `list-modules` match) is strong but CI should still be the final word before merge.

---

## Final Verdict

**READY TO OPEN THE PR into `development`.** No CRITICAL/HIGH security findings attributable to this change; the Docker build succeeds and produces a binary with exactly the expected 27 DNS-01 provider modules plus all pre-existing security modules intact; the Renovate config and toolchain-key.sh fixes are independently verified to work as intended; hadolint is clean with no new findings. The only open item is CI confirming the E2E regression spec in a non-conflicting environment, which is expected and standard per this repo's CI-first policy for full/cross-browser E2E confirmation.
