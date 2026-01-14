
# Custom DNS Provider Plugin Support — Remaining Work Plan

**Date**: 2026-01-14

This document is a phased completion plan for the remaining work on “Custom DNS Provider Plugin Support” on branch `feature/beta-release` (see PR #461 context in `CHANGELOG.md`).

## What’s Already Implemented (Verified)

- **Provider plugin registry**: `dnsprovider.Global()` registry and `dnsprovider.ProviderPlugin` interface in [backend/pkg/dnsprovider](backend/pkg/dnsprovider).
- **Built-in providers moved behind the registry**: 10 built-ins live in [backend/pkg/dnsprovider/builtin](backend/pkg/dnsprovider/builtin) and are registered via the blank import in [backend/cmd/api/main.go](backend/cmd/api/main.go).
- **External plugin loader**: `PluginLoaderService` in [backend/internal/services/plugin_loader.go](backend/internal/services/plugin_loader.go) (loads `.so`, validates metadata/interface version, optional SHA-256 allowlist, secure dir perms).
- **Plugin management backend** (Phase 5): admin endpoints in `backend/internal/api/handlers/plugin_handler.go` mounted under `/api/admin/plugins` via [backend/internal/api/routes/routes.go](backend/internal/api/routes/routes.go).
- **Example external plugin**: PowerDNS reference implementation in [plugins/powerdns](plugins/powerdns).
- **Registry-driven provider CRUD and Caddy config**:
   - Provider validation/testing uses registry providers via [backend/internal/services/dns_provider_service.go](backend/internal/services/dns_provider_service.go)
   - Caddy config generation is registry-driven (per Phase 5 docs)
- **Manual provider type**: `manual` provider plugin in [backend/pkg/dnsprovider/custom/manual_provider.go](backend/pkg/dnsprovider/custom/manual_provider.go).
- **Manual DNS challenge flow (UI + API)**:
   - API handler: [backend/internal/api/handlers/manual_challenge_handler.go](backend/internal/api/handlers/manual_challenge_handler.go)
   - Routes wired in [backend/internal/api/routes/routes.go](backend/internal/api/routes/routes.go)
   - Frontend API/types: [frontend/src/api/manualChallenge.ts](frontend/src/api/manualChallenge.ts)
   - Frontend UI: [frontend/src/components/dns-providers/ManualDNSChallenge.tsx](frontend/src/components/dns-providers/ManualDNSChallenge.tsx)
- **Playwright coverage exists** for manual provider flows: [tests/manual-dns-provider.spec.ts](tests/manual-dns-provider.spec.ts)

## What’s Missing (Verified)

- **Types endpoint is not registry-driven yet**: `GET /api/v1/dns-providers/types` is currently hardcoded in [backend/internal/api/handlers/dns_provider_handler.go](backend/internal/api/handlers/dns_provider_handler.go) and will not surface:
   - the `manual` provider’s field specs
   - any externally loaded plugin types (e.g., PowerDNS)
   - any future custom providers registered in `dnsprovider.Global()`
- **Plugin signature allowlist is not wired**: `PluginLoaderService` supports an optional SHA-256 allowlist map, but [backend/cmd/api/main.go](backend/cmd/api/main.go) passes `nil`.
- **Sandboxing limitation is structural**: Go plugins run in-process (no OS sandbox). The only practical controls are deny-by-default plugin loading + allowlisting + secure deployment guidance.
- **No first-party webhook/script/rfc2136 provider types** exist as built-in `dnsprovider.ProviderPlugin` implementations (this is optional and should be treated as a separate feature, because external plugins already cover the extensibility goal).

---

## Scope

- Make DNS provider type discovery and UI configuration **registry-driven** so built-in + manual + externally loaded plugins show up correctly.
- Close the key security gap for external plugins by wiring an **operator-controlled allowlist** for plugin SHA-256 signatures.
- Keep the scope aligned to repo conventions: no Python, minimal new files, and follow the repository structure rules for any new docs.

## Non-Goals

- No Python scripts or example servers.
- No unrelated refactors of existing built-in providers.
- No “script execution provider” inside Charon (in-process shell execution is a separate high-risk feature and is explicitly out of scope here).
- No broad redesign of certificate issuance beyond what’s required for correct provider type discovery and safe plugin loading.

## Dependencies

- Backend provider registry: [backend/pkg/dnsprovider/plugin.go](backend/pkg/dnsprovider/plugin.go)
- Provider loader: [backend/internal/services/plugin_loader.go](backend/internal/services/plugin_loader.go)
- DNS provider UI/API type fetch: [frontend/src/api/dnsProviders.ts](frontend/src/api/dnsProviders.ts)
- Manual challenge API (used as a reference pattern for “non-Caddy” flows): [backend/internal/api/handlers/manual_challenge_handler.go](backend/internal/api/handlers/manual_challenge_handler.go)
- Container build pipeline: [Dockerfile](Dockerfile) (Caddy built via `xcaddy`)

## Risks

- **Type discovery mismatch**: UI uses `/api/v1/dns-providers/types`; if backend remains hardcoded, registry/manual/external plugin types won’t be configurable.
- **Supply-chain risk (plugins)**: `.so` loading is inherently sensitive; SHA-256 allowlist must be operator-controlled and deny-by-default in hardened deployments.
- **No sandbox**: Go plugins execute in-process with full memory access. Treat plugins as trusted code; document this clearly and avoid implying sandboxing.
- **SSRF / outbound calls**: plugins may implement `TestCredentials()` with outbound HTTP. Core cannot reliably enforce SSRF policy inside plugin code; mitigate via operational controls (restricted egress, allowlisted outbound via infra) and guidance for plugin authors to reuse Charon URL validators.
- **Patch coverage gate**: any production changes must maintain 100% patch coverage for modified lines.

---

## Definition of Done (DoD) Verification Gates (Per Phase)

Repository testing protocol requires Playwright E2E **before** unit tests.

- **E2E (first)**: `npx playwright test --project=chromium`
- **Backend tests**: VS Code task `shell: Test: Backend with Coverage`
- **Frontend tests**: VS Code task `shell: Test: Frontend with Coverage`
- **TypeScript**: VS Code task `shell: Lint: TypeScript Check`
- **Pre-commit**: VS Code task `shell: Lint: Pre-commit (All Files)`
- **Security scans**:
   - VS Code tasks `shell: Security: CodeQL Go Scan (CI-Aligned) [~60s]` and `shell: Security: CodeQL JS Scan (CI-Aligned) [~90s]`
   - VS Code task `shell: Security: Trivy Scan`
   - VS Code task `shell: Security: Go Vulnerability Check`

**Patch coverage requirement**: 100% for modified lines.

---

## Phase 1 — Registry-Driven Type Discovery (Unblocks UI + plugins)

### Deliverables

- Backend `GET /api/v1/dns-providers/types` returns **registry-driven** types, names, fields, and docs URLs.
- The types list includes: built-in providers, `manual`, and any external plugins loaded from `CHARON_PLUGINS_DIR`.
- Unit tests cover the new type discovery logic with 100% patch coverage on modified lines.

### Tasks & Owners

- **Backend_Dev**
   - Replace hardcoded type list behavior in [backend/internal/api/handlers/dns_provider_handler.go](backend/internal/api/handlers/dns_provider_handler.go) with registry output.
   - Use the service as the abstraction boundary:
      - `h.service.GetSupportedProviderTypes()` for the type list
      - `h.service.GetProviderCredentialFields(type)` for field specs
      - `dnsprovider.Global().Get(type).Metadata()` for display name + docs URL
   - Ensure the handler returns a stable, sorted list for predictable UI rendering.
   - Add/adjust tests for the types endpoint.
- **Frontend_Dev**
   - Confirm `getDNSProviderTypes()` is used as the single source of truth where appropriate.
   - Keep the fallback schemas in `frontend/src/data/dnsProviderSchemas.ts` as a defensive measure, but prefer server-provided fields.
- **QA_Security**
   - Validate that a newly registered provider type becomes visible in the UI without a frontend deploy.
- **Docs_Writer**
   - Update operator docs explaining how types are surfaced and how plugins affect the UI.

### Acceptance Criteria

- Creating a `manual` provider is possible end-to-end using the types endpoint output.
- `/api/v1/dns-providers/types` includes `manual` and any externally loaded provider types (when present).
- 100% patch coverage for modified lines.

### Verification Gates

- If UI changed: run Playwright E2E first.
- Run backend + frontend coverage tasks, TypeScript check, pre-commit, and security scans.

---

## Phase 2 — Provider Implementations: `rfc2136`, `webhook`, `script`

This phase is **optional** and should only proceed if we explicitly want “first-party” provider types inside Charon (instead of shipping these as external `.so` plugins). External plugins already satisfy the extensibility goal.

### Deliverables

- New provider plugins implemented (as `dnsprovider.ProviderPlugin`):
   - `rfc2136`
   - `webhook`
   - `script`
- Each provider defines:
   - `Metadata()` (name/description/docs)
   - `CredentialFields()` (field definitions for UI)
   - Validation (required fields, value constraints)
   - `BuildCaddyConfig()` (or explicit alternate flow) with deterministic JSON output

### Tasks & Owners

- **Backend_Dev**
   - Add provider plugin files under [backend/pkg/dnsprovider/custom](backend/pkg/dnsprovider/custom) (pattern matches `manual_provider.go`).
   - Define clear field schemas for each type (avoid guessing provider-specific parameters not supported by the underlying runtime; keep minimal + extensible).
   - Implement validation errors that are actionable (which field, what’s wrong).
   - Add unit tests for each provider plugin:
      - metadata
      - fields
      - validation
      - config generation
- **Frontend_Dev**
   - Ensure provider forms render correctly from server-provided field definitions.
   - Ensure any provider-specific help text uses the docs URL from the server type info.
- **Docs_Writer**
   - Add/update docs pages for each provider type describing required fields and operational expectations.

### Docker/Caddy Decision Checkpoint (Only if needed)

Before changing Docker/Caddy:

- Confirm whether the running Caddy build includes the required DNS modules for the new types.
- If a module is required and not present, update [Dockerfile](Dockerfile) `xcaddy build` arguments to include it.

### Acceptance Criteria

- `rfc2136`, `webhook`, and `script` show up in `/dns-providers/types` with complete field definitions.
- Creating and saving a provider of each type succeeds with validation.
- 100% patch coverage for modified lines.

### Verification Gates

- If UI changed: run Playwright E2E first.
- Run backend + frontend coverage tasks, TypeScript check, pre-commit, and security scans.

---

## Phase 3 — Plugin Security Hardening & Operator Controls

### Deliverables

- Documented and configurable plugin loading policy:
   - plugin directory (`CHARON_PLUGINS_DIR` already used by startup in [backend/cmd/api/main.go](backend/cmd/api/main.go))
   - optional SHA-256 allowlist support wired end-to-end (from config/env → `NewPluginLoaderService(..., allowedSignatures)`)
- Minimal operator guidance for secure deployment.

### Tasks & Owners

- **Backend_Dev**
   - Wire a configuration source for plugin signatures into the `PluginLoaderService` creation path (currently passed `nil` in [backend/cmd/api/main.go](backend/cmd/api/main.go)).
   - Prefer a single env var to stay minimal (example format: JSON map of `pluginName` → `sha256:...`).
   - Add tests covering:
      - allowlist reject (plugin not in allowlist)
      - signature mismatch
      - insecure directory permissions rejection
- **DevOps**
   - Ensure the plugin directory is mounted read-only where feasible.
   - Validate container permissions align with `verifyDirectoryPermissions()` expectations.
- **QA_Security**
   - Threat model review focused on `.so` loading risks and expected mitigations.
- **Docs_Writer**
   - Update plugin operator docs to explain allowlisting, signatures, and safe deployment patterns.

### Acceptance Criteria

- Plugins can be loaded successfully when allowed, and rejected when disallowed.
- Misconfigured (world-writable) plugin directory is detected and prevents loading.
- 100% patch coverage for modified lines.

### Verification Gates

- Run backend + frontend coverage tasks, TypeScript check, pre-commit, and security scans.

---

## Phase 4 — E2E Coverage + Regression Safety

### Deliverables

- Playwright coverage for:
   - DNS provider types rendering and required-field validation (including plugin types)
   - Manual DNS challenge flow regression (existing spec: `tests/manual-dns-provider.spec.ts`)
   - Creating a provider for at least one external plugin type (e.g., `powerdns`) when a plugin is present
- Documented smoke test steps for operators.

### Tasks & Owners

- **QA_Security**
   - Add/extend Playwright specs under [tests](tests).
   - Validate keyboard navigation and form errors are accessible (screen reader friendly) where tests touch UI.
- **Frontend_Dev**
   - Fix any UI issues uncovered by E2E (focus order, error announcements, labels).
- **Backend_Dev**
   - Fix any API contract mismatches discovered by E2E.

### Acceptance Criteria

- E2E passes reliably in Chromium.
- No regressions to manual challenge flow.

### Verification Gates

- Run Playwright E2E first.
- Run backend + frontend coverage tasks, TypeScript check, pre-commit, and security scans.

---

## Open Questions (Need Explicit Decisions)

- For plugin signature allowlisting: what is the desired configuration shape?
   - **Option A (minimal)**: env var JSON map `pluginName` → `sha256:...` parsed by [backend/cmd/api/main.go](backend/cmd/api/main.go)
   - **Option B (operator-friendly)**: load from a mounted file path (adds new config surface)
- For “first-party” providers (`webhook`, `script`, `rfc2136`): are these still required given external plugins already exist?

---

## Notes on Accessibility

UI work in this plan is built with accessibility in mind, but likely still requires manual review and testing (e.g., with Accessibility Insights) as changes land.
