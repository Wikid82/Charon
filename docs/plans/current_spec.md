
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

**Status**: ✅ **Implementation Complete, QA-Approved** (2026-01-14)
- Backend implementation complete
- QA security review passed
- Operator documentation published: [docs/features/plugin-security.md](docs/features/plugin-security.md)
- Remaining: Unit test coverage for `plugin_loader_test.go`

### Current Implementation Analysis

**PluginLoaderService Location**: [backend/internal/services/plugin_loader.go](backend/internal/services/plugin_loader.go)

**Constructor Signature**:
```go
func NewPluginLoaderService(db *gorm.DB, pluginDir string, allowedSignatures map[string]string) *PluginLoaderService
```

**Service Struct**:
```go
type PluginLoaderService struct {
    pluginDir     string
    allowedSigs   map[string]string // plugin name (without .so) -> expected signature
    loadedPlugins map[string]string // plugin type -> file path
    db            *gorm.DB
    mu            sync.RWMutex
}
```

**Existing Security Checks**:
1. `verifyDirectoryPermissions(dir)` — rejects world-writable directories (mode `0002`)
2. Signature verification in `LoadPlugin()` when `len(s.allowedSigs) > 0`:
   - Checks if plugin name exists in allowlist → returns `dnsprovider.ErrPluginNotInAllowlist` if not
   - Computes SHA-256 via `computeSignature()` → returns `dnsprovider.ErrSignatureMismatch` if different
3. Interface version check via `meta.InterfaceVersion`

**Current main.go Usage** (line ~163):
```go
pluginLoader := services.NewPluginLoaderService(db, pluginDir, nil) // <-- nil bypasses allowlist
```

**Allowlist Behavior**:
- When `allowedSignatures` is `nil` or empty: all plugins are loaded (permissive mode)
- When `allowedSignatures` has entries: only listed plugins with matching signatures are allowed

**Error Types** (from [backend/pkg/dnsprovider/errors.go](backend/pkg/dnsprovider/errors.go)):
- `dnsprovider.ErrPluginNotInAllowlist` — plugin name not found in allowlist map
- `dnsprovider.ErrSignatureMismatch` — SHA-256 hash doesn't match expected value

### Design Decision: Option A (Env Var JSON Map)

**Environment Variable**: `CHARON_PLUGIN_SIGNATURES`

**Format**: JSON object mapping plugin filename (with `.so`) to SHA-256 signature
```json
{"powerdns.so": "sha256:abc123...", "myplugin.so": "sha256:def456..."}
```

**Behavior**:
| Env Var State | Behavior |
|---------------|----------|
| Unset/empty (`""`) | Permissive mode (backward compatible) — all plugins loaded |
| Set to `{}` | Strict mode with empty allowlist — no external plugins loaded |
| Set with entries | Strict mode — only listed plugins with matching signatures |

**Rationale for Option A**:
- Single env var keeps configuration surface minimal
- JSON is parseable in Go with `encoding/json`
- Follows existing pattern (`CHARON_PLUGINS_DIR`, `CHARON_CROWDSEC_*`)
- Operators can generate signatures with: `sha256sum plugin.so | awk '{print "sha256:" $1}'`

### Deliverables

1. **Parse and wire allowlist in main.go**
2. **Helper function to parse signature env var**
3. **Unit tests for PluginLoaderService** (currently missing!)
4. **Operator documentation**

### Implementation Tasks

#### Task 3.1: Add Signature Parsing Helper

**File**: [backend/cmd/api/main.go](backend/cmd/api/main.go) (or new file `backend/internal/config/plugin_config.go`)

```go
// parsePluginSignatures parses the CHARON_PLUGIN_SIGNATURES env var.
// Returns nil if unset/empty (permissive mode).
// Returns empty map if set to "{}" (strict mode, no plugins).
// Returns populated map if valid JSON with entries.
func parsePluginSignatures() (map[string]string, error) {
    raw := os.Getenv("CHARON_PLUGIN_SIGNATURES")
    if raw == "" {
        return nil, nil // Permissive mode
    }

    var sigs map[string]string
    if err := json.Unmarshal([]byte(raw), &sigs); err != nil {
        return nil, fmt.Errorf("invalid CHARON_PLUGIN_SIGNATURES JSON: %w", err)
    }
    return sigs, nil
}
```

#### Task 3.2: Wire Parsing into main.go

**File**: [backend/cmd/api/main.go](backend/cmd/api/main.go)

**Change** (around line 163):
```go
// Before:
pluginLoader := services.NewPluginLoaderService(db, pluginDir, nil)

// After:
pluginSignatures, err := parsePluginSignatures()
if err != nil {
    log.Fatalf("parse plugin signatures: %v", err)
}
if pluginSignatures != nil {
    logger.Log().Infof("Plugin signature allowlist enabled with %d entries", len(pluginSignatures))
} else {
    logger.Log().Info("Plugin signature allowlist not configured (permissive mode)")
}
pluginLoader := services.NewPluginLoaderService(db, pluginDir, pluginSignatures)
```

#### Task 3.3: Create PluginLoaderService Unit Tests

**File**: [backend/internal/services/plugin_loader_test.go](backend/internal/services/plugin_loader_test.go) (NEW)

**Test Scenarios**:

| Test Name | Setup | Expected Result |
|-----------|-------|-----------------|
| `TestNewPluginLoaderService_NilAllowlist` | `allowedSignatures: nil` | Service created, `allowedSigs` is nil |
| `TestNewPluginLoaderService_EmptyAllowlist` | `allowedSignatures: map[string]string{}` | Service created, `allowedSigs` is empty map |
| `TestNewPluginLoaderService_PopulatedAllowlist` | `allowedSignatures: {"test.so": "sha256:abc"}` | Service created with entries |
| `TestLoadPlugin_AllowlistEmpty_SkipsVerification` | Empty allowlist, mock plugin | Plugin loads without signature check |
| `TestLoadPlugin_AllowlistSet_PluginNotListed` | Allowlist without plugin | Returns `ErrPluginNotInAllowlist` |
| `TestLoadPlugin_AllowlistSet_SignatureMismatch` | Allowlist with wrong hash | Returns `ErrSignatureMismatch` |
| `TestLoadPlugin_AllowlistSet_SignatureMatch` | Allowlist with correct hash | Plugin loads successfully |
| `TestVerifyDirectoryPermissions_Secure` | Dir mode `0755` | Returns nil |
| `TestVerifyDirectoryPermissions_WorldWritable` | Dir mode `0777` | Returns error |
| `TestComputeSignature_ValidFile` | Real file | Returns `sha256:...` string |
| `TestLoadAllPlugins_DirectoryNotExist` | Non-existent dir | Returns nil (graceful skip) |
| `TestLoadAllPlugins_DirectoryInsecure` | World-writable dir | Returns error |

**Note**: Testing actual `.so` loading requires CGO and platform-specific binaries. Focus unit tests on:
- Constructor behavior
- `verifyDirectoryPermissions()` (create temp dirs)
- `computeSignature()` (create temp files)
- Allowlist logic flow (mock the actual `plugin.Open` call)

#### Task 3.4: Create parsePluginSignatures Unit Tests

**File**: [backend/cmd/api/main_test.go](backend/cmd/api/main_test.go) or integrate into plugin_loader_test.go

| Test Name | Env Value | Expected Result |
|-----------|-----------|-----------------|
| `TestParsePluginSignatures_Unset` | (not set) | `nil, nil` |
| `TestParsePluginSignatures_Empty` | `""` | `nil, nil` |
| `TestParsePluginSignatures_EmptyObject` | `"{}"` | `map[string]string{}, nil` |
| `TestParsePluginSignatures_Valid` | `{"a.so":"sha256:x"}` | `map with entry, nil` |
| `TestParsePluginSignatures_InvalidJSON` | `"not json"` | `nil, error` |
| `TestParsePluginSignatures_MultipleEntries` | `{"a.so":"sha256:x","b.so":"sha256:y"}` | `map with 2 entries, nil` |

### Tasks & Owners

- **Backend_Dev**
   - [x] Create `parsePluginSignatures()` helper function ✅ *Completed 2026-01-14*
   - [x] Update [backend/cmd/api/main.go](backend/cmd/api/main.go) to wire parsed signatures ✅ *Completed 2026-01-14*
   - [ ] Create [backend/internal/services/plugin_loader_test.go](backend/internal/services/plugin_loader_test.go) with comprehensive test coverage
   - [x] Add logging for allowlist mode (enabled vs permissive) ✅ *Completed 2026-01-14*
- **DevOps**
   - [x] Ensure the plugin directory is mounted read-only in production (`/app/plugins:ro`) ✅ *Completed 2026-01-14*
   - [x] Validate container permissions align with `verifyDirectoryPermissions()` (mode `0755` or stricter) ✅ *Completed 2026-01-14*
   - [x] Document how to generate plugin signatures: `sha256sum plugin.so | awk '{print "sha256:" $1}'` ✅ *See below*
- **QA_Security**
   - [x] Threat model review focused on `.so` loading risks ✅ *QA-approved 2026-01-14*
   - [x] Verify error messages don't leak sensitive path information ✅ *QA-approved 2026-01-14*
   - [x] Test edge cases: symlinks, race conditions, permission changes ✅ *QA-approved 2026-01-14*
- **Docs_Writer**
   - [x] Create/update plugin operator docs explaining: ✅ *Completed 2026-01-14*
      - `CHARON_PLUGIN_SIGNATURES` format and behavior
      - How to compute signatures
      - Recommended deployment pattern (read-only mounts, strict allowlist)
      - Security implications of permissive mode
   - [x] Created [docs/features/plugin-security.md](docs/features/plugin-security.md) ✅ *Completed 2026-01-14*

### Acceptance Criteria

- [x] Plugins load successfully when signature matches allowlist ✅ *QA-approved*
- [x] Plugins are rejected with `ErrPluginNotInAllowlist` when not in allowlist ✅ *QA-approved*
- [x] Plugins are rejected with `ErrSignatureMismatch` when hash differs ✅ *QA-approved*
- [x] World-writable plugin directory is detected and prevents all plugin loading ✅ *QA-approved*
- [x] Empty/unset `CHARON_PLUGIN_SIGNATURES` maintains backward compatibility (permissive) ✅ *QA-approved*
- [x] Invalid JSON in `CHARON_PLUGIN_SIGNATURES` causes startup failure with clear error ✅ *QA-approved*
- [ ] 100% patch coverage for modified lines in `main.go`
- [ ] New `plugin_loader_test.go` achieves high coverage of testable code paths
- [x] Operator documentation created: [docs/features/plugin-security.md](docs/features/plugin-security.md) ✅ *Completed 2026-01-14*

### Verification Gates

- Run backend coverage task: `shell: Test: Backend with Coverage`
- Run security scans:
   - `shell: Security: CodeQL Go Scan (CI-Aligned) [~60s]`
   - `shell: Security: Go Vulnerability Check`
- Run pre-commit: `shell: Lint: Pre-commit (All Files)`

### Risks & Mitigations

| Risk | Mitigation |
|------|------------|
| Invalid JSON crashes startup | Explicit error handling with descriptive message |
| Plugin name mismatch (with/without `.so`) | Document exact format; code expects filename as key |
| Signature format confusion | Enforce `sha256:` prefix; reject malformed signatures |
| Race condition: plugin modified after signature check | Document atomic deployment pattern (copy then rename) |
| Operators forget to update signatures after plugin update | Log warning when signature verification is enabled |

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

- ~~For plugin signature allowlisting: what is the desired configuration shape?~~
   - **DECIDED: Option A (minimal)**: env var `CHARON_PLUGIN_SIGNATURES` with JSON map `pluginFilename.so` → `sha256:...` parsed by [backend/cmd/api/main.go](backend/cmd/api/main.go). See Phase 3 for full specification.
   - ~~**Option B (operator-friendly)**: load from a mounted file path (adds new config surface)~~ — Not chosen; JSON env var is sufficient and simpler.
- For “first-party” providers (`webhook`, `script`, `rfc2136`): are these still required given external plugins already exist?

---

## Notes on Accessibility

UI work in this plan is built with accessibility in mind, but likely still requires manual review and testing (e.g., with Accessibility Insights) as changes land.
