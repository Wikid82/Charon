---
post_title: "Current Spec: Caddy 2.11.1 Compatibility, Security, and UX Impact Plan"
categories:
  - actions
  - security
  - backend
  - frontend
  - infrastructure
tags:
  - caddy
  - xcaddy
  - dependency-management
  - vulnerability-management
  - release-planning
summary: "Comprehensive, phased plan to evaluate and safely adopt Caddy v2.11.1 in Charon, covering plugin compatibility, CVE impact, xcaddy patch retirement decisions, UI/UX exposure opportunities, and PR slicing strategy with strict validation gates."
post_date: 2026-02-23
---

## Active Plan: Caddy 2.11.1 Deep Compatibility and Security Rollout

Date: 2026-02-23
Status: Active and authoritative
Scope Type: Architecture/security/dependency research and implementation planning
Authority: This is the only active authoritative plan section in this file.

## Introduction

Charon’s control plane and data plane rely on Caddy as a core runtime backbone.
Because Caddy is embedded and rebuilt via `xcaddy`, upgrading from
`2.11.0-beta.2` to `2.11.1` is not a routine version bump: it impacts
runtime behavior, plugin compatibility, vulnerability posture, and potential UX
surface area.

This plan defines a low-risk, high-observability rollout strategy that answers:

1. Which Caddy 2.11.x features should be exposed in Charon UI/API?
2. Which existing Charon workarounds became redundant upstream?
3. Which `xcaddy` dependency patches remain necessary vs removable?
4. Which known vulnerabilities are fixed now and which should remain on watch?

## Research Findings

### External release and security findings

1. Official release statement confirms `v2.11.1` has no runtime code delta from
   `v2.11.0` except CI/release process correction. Practical implication:
   compatibility/security validation should target **2.11.x** behavior, not
   2.11.1-specific runtime changes.
2. Caddy release lists six security patches (mapped to GitHub advisories):
   - `CVE-2026-27590` → `GHSA-5r3v-vc8m-m96g` (FastCGI split_path confusion)
   - `CVE-2026-27589` → `GHSA-879p-475x-rqh2` (admin API cross-origin no-cors)
   - `CVE-2026-27588` → `GHSA-x76f-jf84-rqj8` (host matcher case bypass)
   - `CVE-2026-27587` → `GHSA-g7pc-pc7g-h8jh` (path matcher escaped-case bypass)
   - `CVE-2026-27586` → `GHSA-hffm-g8v7-wrv7` (mTLS client-auth fail-open)
   - `CVE-2026-27585` → `GHSA-4xrr-hq4w-6vf4` (glob sanitization bypass)
3. NVD/CVE.org entries are currently reserved/not fully enriched. GitHub
   advisories are the most actionable source right now.

### Charon architecture and integration findings

1. Charon compiles custom Caddy in `Dockerfile` via `xcaddy` and injects:
   - `github.com/greenpau/caddy-security`
   - `github.com/corazawaf/coraza-caddy/v2`
   - `github.com/hslatman/caddy-crowdsec-bouncer@v0.10.0`
   - `github.com/zhangjiayin/caddy-geoip2`
   - `github.com/mholt/caddy-ratelimit`
2. Charon applies explicit post-generation `go get` patching in `Dockerfile` for:
   - `github.com/expr-lang/expr@v1.17.7`
   - `github.com/hslatman/ipstore@v0.4.0`
   - `github.com/slackhq/nebula@v1.9.7` (with comment indicating temporary pin)
3. Charon CI has explicit dependency inspection gate in
   `.github/workflows/docker-build.yml` to verify patched `expr-lang/expr`
   versions in built binaries.

### Plugin compatibility findings (highest risk area)

Current plugin module declarations (upstream `go.mod`) target older Caddy cores:

- `greenpau/caddy-security`: `caddy/v2 v2.10.2`
- `hslatman/caddy-crowdsec-bouncer`: `caddy/v2 v2.10.2`
- `corazawaf/coraza-caddy/v2`: `caddy/v2 v2.9.1`
- `zhangjiayin/caddy-geoip2`: `caddy/v2 v2.10.0`
- `mholt/caddy-ratelimit`: `caddy/v2 v2.8.0`

Implication: compile success against 2.11.1 is plausible but not guaranteed.
The plan must include matrix build/provision tests before merge.

### Charon UX and config-surface findings

Current Caddy-related UI/API exposure is narrow:

- `frontend/src/pages/SystemSettings.tsx`
  - state: `caddyAdminAPI`, `sslProvider`
  - saves keys: `caddy.admin_api`, `caddy.ssl_provider`
- `frontend/src/pages/ImportCaddy.tsx` and import components:
  - Caddyfile parsing/import workflow, not runtime feature toggles
- `frontend/src/api/import.ts`, `frontend/src/api/settings.ts`
- Backend routes and handlers:
  - `backend/internal/api/routes/routes.go`
  - `backend/internal/api/handlers/settings_handler.go`
  - `backend/internal/api/handlers/import_handler.go`
  - `backend/internal/caddy/manager.go`
  - `backend/internal/caddy/config.go`
  - `backend/internal/caddy/types.go`

No UI controls currently exist for new Caddy 2.11.x capabilities such as
`keepalive_idle`, `keepalive_count`, `trusted_proxies_unix`,
`renewal_window_ratio`, or `0-RTT` behavior.

## Requirements (EARS)

1. WHEN evaluating Caddy `v2.11.1`, THE SYSTEM SHALL validate compatibility
   against all currently enabled `xcaddy` plugins before changing production
   defaults.
2. WHEN security advisories in Caddy 2.11.x affect modules Charon may use,
   THE SYSTEM SHALL document exploitability for Charon’s deployment model and
   prioritize remediation accordingly.
3. WHEN an `xcaddy` patch/workaround no longer provides value,
   THE SYSTEM SHALL remove it only after reproducible build and runtime
   validation gates pass.
4. IF a Caddy 2.11.x feature maps to an existing Charon concept,
   THEN THE SYSTEM SHALL prefer extending existing UI/components over adding new
   parallel controls.
5. WHEN no direct UX value exists, THE SYSTEM SHALL avoid adding UI for upstream
   options and keep behavior backend-managed.
6. WHEN this rollout completes, THE SYSTEM SHALL provide explicit upstream watch
   criteria for unresolved/reserved CVEs and plugin dependency lag.

## Technical Specifications

### Compatibility scope map (code touch inventory)

#### Build/packaging

- `Dockerfile`
  - `ARG CADDY_VERSION`
  - `ARG XCADDY_VERSION`
  - `caddy-builder` stage (`xcaddy build`, plugin list, `go get` patches)
- `.github/workflows/docker-build.yml`
  - binary dependency checks (`go version -m` extraction/gates)
- `.github/renovate.json`
  - regex managers tracking `Dockerfile` patch dependencies

#### Caddy runtime config generation

- `backend/internal/caddy/manager.go`
  - `NewManager(...)`
  - `ApplyConfig(ctx)`
- `backend/internal/caddy/config.go`
  - `GenerateConfig(...)`
- `backend/internal/caddy/types.go`
  - JSON struct model for Caddy config (`Server`, `TrustedProxies`, etc.)

#### Settings and admin surface

- `backend/internal/api/handlers/settings_handler.go`
  - `UpdateSetting(...)`, `PatchConfig(...)`
- `backend/internal/api/routes/routes.go`
  - Caddy manager wiring + settings routes
- `frontend/src/pages/SystemSettings.tsx`
  - current Caddy-related controls

#### Caddyfile import behavior

- `backend/internal/api/handlers/import_handler.go`
  - `RegisterRoutes(...)`, `Upload(...)`, `GetPreview(...)`
- `backend/internal/caddy/importer.go`
  - `NormalizeCaddyfile(...)`, `ParseCaddyfile(...)`, `ExtractHosts(...)`
- `frontend/src/pages/ImportCaddy.tsx`
  - import UX and warning handling

### Feature impact assessment (2.11.x)

#### Candidate features for potential Charon exposure

1. Keepalive server options (`keepalive_idle`, `keepalive_count`)
   - Candidate mapping: advanced per-host connection tuning
   - Likely files: `backend/internal/caddy/types.go`,
     `backend/internal/caddy/config.go`, host settings API + UI
2. `trusted_proxies_unix`
   - Candidate mapping: trusted local socket proxy chains
   - Current `TrustedProxies` struct lacks explicit unix-socket trust fields
3. Certificate lifecycle tunables (`renewal_window_ratio`, maintenance interval)
   - Candidate mapping: advanced TLS policy controls
   - Potentially belongs under system-level TLS settings, not per-host UI

#### Features likely backend-only / no new UI by default

1. Reverse-proxy automatic `Host` rewrite for TLS upstreams
2. ECH key auto-rotation
3. `SIGUSR1` reload fallback behavior
4. Logging backend internals (`timberjack`, ordering fixes)

Plan decision rule: expose only options that produce clear operator value and
can be represented without adding UX complexity.

### Security patch relevance matrix

#### Advisory exploitability rubric and ownership

Use the following deterministic rubric for each advisory before any promotion:

| Field | Required Values | Rule |
| --- | --- | --- |
| Exploitability | `Affected` / `Not affected` / `Mitigated` | `Affected` means a reachable vulnerable path exists in Charon runtime; `Not affected` means required feature/path is not present; `Mitigated` means vulnerable path exists upstream but Charon deployment/runtime controls prevent exploitation. |
| Evidence source | advisory + code/config/runtime proof | Must include at least one authoritative upstream source (GitHub advisory/Caddy release) and one Charon-local proof (config path, test, scan, or runtime verification). |
| Owner | named role | Security owner for final disposition (`QA_Security` lead or delegated maintainer). |
| Recheck cadence | `weekly` / `release-candidate` / `on-upstream-change` | Minimum cadence: weekly until CVE enrichment is complete and disposition is stable for two consecutive checks. |

Promotion gate: every advisory must have all four fields populated and signed by
owner in the PR evidence bundle.

#### High-priority for Charon context

1. `GHSA-879p-475x-rqh2` (admin API cross-origin no-cors)
   - Charon binds admin API internally but still uses `0.0.0.0:2019` in
     generated config. Must verify actual network isolation and container
     exposure assumptions.
2. `GHSA-hffm-g8v7-wrv7` (mTLS fail-open)
   - Relevant if client-auth CA pools are configured anywhere in generated or
     imported config paths.
3. matcher bypass advisories (`GHSA-x76f-jf84-rqj8`, `GHSA-g7pc-pc7g-h8jh`)
   - Potentially relevant to host/path-based access control routing in Caddy.

#### Contextual/conditional relevance

- `GHSA-5r3v-vc8m-m96g` (FastCGI split_path)
  - Relevant only if FastCGI transport is in active use.
- `GHSA-4xrr-hq4w-6vf4` (file matcher glob sanitization)
  - Relevant when file matchers are used in route logic.

### xcaddy patch retirement candidates

#### Candidate to re-evaluate for removal

- `go get github.com/slackhq/nebula@v1.9.7`
  - Upstream Caddy has moved forward to `nebula v1.10.3` and references
    security-related maintenance in the 2.11.x line.
  - Existing Charon pin comment may be stale after upstream smallstep updates.

#### Likely retain until proven redundant

- `go get github.com/expr-lang/expr@v1.17.7`
- `go get github.com/hslatman/ipstore@v0.4.0`

Retention/removal decision must be made using reproducible build + binary
inspection evidence, not assumption.

#### Hard retirement gates (mandatory before removing any pin)

Pin removal is blocked unless all gates pass:

1. Binary module diff gate
   - Produce before/after `go version -m` module diff for Caddy binary.
   - No unexpected module major-version jumps outside approved advisory scope.
2. Security regression gate
   - No new HIGH/CRITICAL findings in CodeQL/Trivy/Grype compared to baseline.
3. Reproducible build parity gate
   - Two clean rebuilds produce equivalent module inventory and matching runtime
     smoke results.
4. Rollback proof gate (mandatory, with explicit `nebula` focus)
   - Demonstrate one-command rollback to previous pin set, with successful
     compile + runtime smoke set after rollback.

Retirement decision for `nebula` cannot proceed without explicit rollback proof
artifact attached to PR evidence.

### Feature-to-control mapping (exposure decision matrix)

| Feature | Control surface | Expose vs backend-only rationale | Persistence path |
| --- | --- | --- | --- |
| `keepalive_idle`, `keepalive_count` | Existing advanced system settings (if approved) | Expose only if operators need deterministic upstream connection control; otherwise keep backend defaults to avoid UX bloat. | `frontend/src/pages/SystemSettings.tsx` → `frontend/src/api/settings.ts` → `backend/internal/api/handlers/settings_handler.go` → DB settings → `backend/internal/caddy/config.go` (`GenerateConfig`) |
| `trusted_proxies_unix` | Backend-only default initially | Backend-only until proven demand for unix-socket trust tuning; avoid misconfiguration risk in general UI. | backend config model (`backend/internal/caddy/types.go`) + generated config path (`backend/internal/caddy/config.go`) |
| `renewal_window_ratio`, cert maintenance interval | Backend-only policy | Keep backend-only unless operations requires explicit lifecycle tuning controls. | settings store (if introduced) → `settings_handler.go` → `GenerateConfig` |
| Reverse-proxy Host rewrite / ECH rotation / reload fallback internals | Backend-only | Operational internals with low direct UI value; exposing would increase complexity without clear user benefit. | backend runtime defaults and generated Caddy config only |

## Implementation Plan

### Phase 1: Playwright and behavior baselining (mandatory first)

Objective: capture stable pre-upgrade behavior and ensure UI/UX parity checks.

1. Run targeted E2E suites covering Caddy-critical flows:
   - `tests/tasks/import-caddyfile.spec.ts`
   - `tests/security-enforcement/zzz-caddy-imports/*.spec.ts`
   - system settings-related tests around Caddy admin API and SSL provider
2. Capture baseline artifacts:
   - Caddy import warning behavior
   - security settings save/reload behavior
   - admin API connectivity assumptions from test fixtures
3. Produce a baseline report in `docs/reports/` for diffing in later phases.

### Phase 2: Backend and build compatibility research implementation

Objective: validate compile/runtime compatibility of Caddy 2.11.1 with current
plugin set and patch set.

1. Bump candidate in `Dockerfile`:
   - `ARG CADDY_VERSION=2.11.1`
2. Execute matrix builds with toggles:
   - Scenario A: current patch set unchanged
   - Scenario B: remove `nebula` pin only
   - Scenario C: remove `nebula` + retain `expr/ipstore`
3. Execute explicit compatibility gate matrix (deterministic):

   | Dimension | Values |
   | --- | --- |
   | Plugin set | `caddy-security`, `coraza-caddy`, `caddy-crowdsec-bouncer`, `caddy-geoip2`, `caddy-ratelimit` |
   | Patch scenario | `A` current pins, `B` no `nebula` pin, `C` no `nebula` pin + retained `expr/ipstore` pins |
   | Platform/arch | `linux/amd64`, `linux/arm64` |
   | Runtime smoke set | boot Caddy, apply generated config, admin API health, import preview, one secured proxy request path |

   Deterministic pass/fail rule:
   - **Pass**: all plugin modules compile/load for the matrix cell AND all smoke
     tests pass.
   - **Fail**: any compile/load error, missing module, or smoke failure.

   Promotion criteria:
   - PR-1 promotion requires 100% pass for Scenario A on both architectures.
   - Scenario B/C may progress only as candidate evidence; they cannot promote to
     default unless all hard retirement gates pass.
4. Validate generated binary dependencies from CI/local:
   - verify `expr`, `ipstore`, `nebula`, `smallstep/certificates` versions
5. Validate runtime config application path:
   - `backend/internal/caddy/manager.go` → `ApplyConfig(ctx)`
   - `backend/internal/caddy/config.go` → `GenerateConfig(...)`
6. Run Caddy package tests and relevant integration tests:
   - `backend/internal/caddy/*`
   - security middleware integration paths that rely on Caddy behavior

### Phase 3: Security hardening and vulnerability posture updates

Objective: translate upstream advisories into Charon policy and tests.

1. Add/adjust regression tests for advisory-sensitive behavior in
   `backend/internal/caddy` and integration test suites, especially:
   - host matcher behavior with large host lists
   - escaped path matcher handling
   - admin API cross-origin assumptions
2. Update security documentation and operational guidance:
   - identify which advisories are mitigated by upgrade alone
   - identify deployment assumptions (e.g., local admin API exposure)
3. Introduce watchlist process for RESERVED CVEs pending NVD enrichment:
   - monitor Caddy advisories and module-level disclosures weekly

### Phase 4: Frontend and API exposure decisions (only if justified)

Objective: decide whether 2.11.x features merit UI controls.

1. Evaluate additions to existing `SystemSettings` UX only (no new page):
   - optional advanced toggles for keepalive tuning and trusted proxy unix scope
2. Add backend settings keys and mapping only where persisted behavior is
   needed:
   - settings handler support in
     `backend/internal/api/handlers/settings_handler.go`
   - propagation to config generation in `GenerateConfig(...)`
3. If no high-value operator need is proven, keep features backend-default and
   document rationale.

### Phase 5: Validation, docs, and release readiness

Objective: ensure secure, reversible, and auditable rollout.

1. Re-run full DoD sequence (E2E, patch report, security scans, coverage).
2. Update architectural docs if behavior/config model changes.
3. Publish release decision memo:
   - accepted changes
   - rejected/deferred UX features
   - retained/removed patches with evidence

## PR Slicing Strategy

### Decision

Use **multiple PRs (PR-1/PR-2/PR-3)**.

Reasoning:

1. Work spans infra/build security + backend runtime + potential frontend UX.
2. Caddy is a blast-radius-critical dependency; rollback safety is mandatory.
3. Review quality and CI signal are stronger with isolated, testable slices.

### PR-1: Compatibility and evidence foundation

Scope:

- `Dockerfile` Caddy candidate bump (and temporary feature branch matrix toggles)
- CI/workflow compatibility instrumentation if needed
- compatibility report artifacts and plan-linked documentation

Dependencies:

- None

Acceptance criteria:

1. Caddy 2.11.1 compiles with existing plugin set under at least one stable
   patch scenario.
2. Compatibility gate matrix (plugin × patch scenario × platform/arch × runtime
    smoke set) executed with deterministic pass/fail output and attached evidence.
3. Binary module inventory report generated and attached.
4. No production behavior changes merged beyond compatibility scaffolding.

Release guard (mandatory for PR-1):

- Candidate tag only (`*-rc`/`*-candidate`) is allowed.
- Release pipeline exclusion is required; PR-1 artifacts must not be eligible
   for production release jobs.
- Promotion to releasable tag is blocked until PR-2 security/retirement gates
   pass.

Rollback notes:

- Revert `Dockerfile` arg changes and instrumentation only.

### PR-2: Security patch posture + patch retirement decision

Scope:

- finalize retained/removed `go get` patch lines in `Dockerfile`
- update security tests/docs tied to six Caddy advisories
- tighten/confirm admin API exposure assumptions

Dependencies:

- PR-1 evidence

Acceptance criteria:

1. Decision logged for each patch (`expr`, `ipstore`, `nebula`) with rationale.
2. Advisory coverage matrix completed with Charon applicability labels.
3. Security scans clean at required policy thresholds.

Rollback notes:

- Revert patch retirement lines and keep previous pinned patch model.

### PR-3: Optional UX/API exposure and cleanup

Scope:

- only approved high-value settings exposed in existing settings surface
- backend mapping and frontend wiring using existing settings flows
- docs and translations updates if UI text changes

Dependencies:

- PR-2 must establish stable runtime baseline first

Acceptance criteria:

1. No net-new page; updates land in existing `SystemSettings` domain.
2. E2E and unit tests cover newly exposed controls and defaults.
3. Deferred features explicitly documented with rationale.

Rollback notes:

- Revert UI/API additions while retaining already landed security/runtime upgrades.

## Config File Review and Proposed Updates

### Dockerfile (required updates)

1. Update `ARG CADDY_VERSION` target to `2.11.1` after PR-1 gating.
2. Reassess and potentially remove stale `nebula` pin in caddy-builder stage
   if matrix build proves compatibility and security posture improves.
3. Keep `expr`/`ipstore` patch enforcement until binary inspection proves
   upstream transitive versions are consistently non-vulnerable.

### .gitignore (suggested updates)

No mandatory update for rollout, but recommended if new evidence artifacts are
generated in temporary paths:

- ensure transient compatibility artifacts are ignored (for example,
  `test-results/caddy-compat/**` if used).

### .dockerignore (suggested updates)

No mandatory update; current file already excludes heavy test/docs/security
artifacts and keeps build context lean. Revisit only if new compatibility
fixture directories are introduced.

### codecov.yml (suggested updates)

No mandatory change for version upgrade itself. If new compatibility harness
tests are intentionally non-coverage-bearing, add explicit ignore patterns to
avoid noise in project and patch coverage reports.

## Risk Register and Mitigations

1. Plugin/API incompatibility with Caddy 2.11.1
   - Mitigation: matrix compile + targeted runtime tests before merge.
2. False confidence from scanner-only dependency policies
   - Mitigation: combine advisory-context review with binary-level inspection.
3. Behavioral drift in reverse proxy/matcher semantics
   - Mitigation: baseline E2E + focused security regression tests.
4. UI sprawl from exposing too many Caddy internals
   - Mitigation: only extend existing settings surface when operator value is
     clear and validated.

## Acceptance Criteria

1. Charon builds and runs with Caddy 2.11.1 and current plugin set under
   deterministic CI validation.
2. A patch disposition table exists for `expr`, `ipstore`, and `nebula`
   (retain/remove/replace + evidence).
3. Caddy advisory applicability matrix is documented, including exploitability
   notes for Charon deployment model.
4. Any added settings are mapped end-to-end:
   frontend state → API payload → persisted setting → `GenerateConfig(...)`.
5. E2E, security scans, and coverage gates pass without regression.
6. PR-1/PR-2/PR-3 deliverables are independently reviewable and rollback-safe.

## Handoff

After approval of this plan:

1. Delegate PR-1 execution to implementation workflow.
2. Require evidence artifacts before approving PR-2 scope reductions
   (especially patch removals).
3. Treat PR-3 as optional and value-driven, not mandatory for the security
   update itself.
