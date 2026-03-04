# SSRF (go/request-forgery) Remediation — Supervisor-Aligned Plan

**Status:** Planning (active)

This plan remediates CodeQL’s `go/request-forgery` (CWE-918 / SSRF) in the Go backend by hardening remaining outbound network call sites and removing blanket suppressions.

## Constraints (Supervisor requirements)

- **Deny-by-default:** Only allow outbound targets that are explicitly allowed by policy.
- **Docker/service-name deployments:** Internal dependencies (notably CrowdSec LAPI and Caddy Admin) may be reached via Docker service names (e.g., `http://crowdsec:8085`, `http://caddy:2019`), not just `localhost`. The plan must support this safely via explicit allowlists.
- **No mergeable intermediate state:** The global CodeQL exclusion for `go/request-forgery` in [.github/codeql/codeql-config.yml](.github/codeql/codeql-config.yml) must be removed in the same PR as the fixes.
- **Ignore proxy environment variables:** SSRF-sensitive clients must not inherit `HTTP_PROXY` / `HTTPS_PROXY` / `NO_PROXY` behavior.
- **Redirect policy must be explicit:** Each call site must either disable redirects or validate each hop against the same SSRF policy.
- **Keep TCP dialing scope minimal:** Only touch TCP-dial endpoints if they are directly required for `go/request-forgery` remediation or explicitly called out by findings.

## In-scope call sites (confirmed)

These are the known SSRF-relevant sinks discovered during recon:

- Uptime monitor HTTP checks: `(*UptimeService).checkMonitor` in
  [backend/internal/services/uptime_service.go](backend/internal/services/uptime_service.go)
- CrowdSec LAPI: `(*CrowdsecHandler).GetLAPIDecisions` and `(*CrowdsecHandler).CheckLAPIHealth` in
  [backend/internal/api/handlers/crowdsec_handler.go](backend/internal/api/handlers/crowdsec_handler.go)
- Caddy Admin API client: `caddy.NewClient` and `(*Client).Load/GetConfig/Ping` in
  [backend/internal/caddy/client.go](backend/internal/caddy/client.go)
- URL connectivity test path: `utils.TestURLConnectivity` in
  [backend/internal/utils/url_testing.go](backend/internal/utils/url_testing.go)

## Existing primitives (use, don’t duplicate)

- URL validation: `ValidateExternalURL(...)` in
  [backend/internal/security/url_validator.go](backend/internal/security/url_validator.go)
- Dial-time checks + redirect controls: `NewSafeHTTPClient(...)` in
  [backend/internal/network/safeclient.go](backend/internal/network/safeclient.go)

## Target security posture

### Default outbound policy (external)

- Allow `https` only by default (allow `http` only where strictly needed).
- Reject userinfo in URLs.
- Block private/reserved/link-local/loopback/multicast ranges for both IPv4/IPv6.
- Validate via DNS resolution and re-validate at connect time (DNS rebinding defense).

### Internal service policy (explicit allowlist)

Some internal integrations must target *non-public* addresses in Docker or localhost scenarios. These are allowed only if:

- The destination hostname is in an explicit allowlist (exact matches; no wildcards).
- The port matches the expected service port.
- Redirects are disabled (recommended) or strictly hop-validated.
- Proxy env vars are ignored.

Implementation direction (plan-level):

- Introduce a single internal-service allowlist configuration surface (e.g., an env-backed list) used by both CrowdSec and Caddy admin client validation.
- Keep defaults strict: allow `localhost`/`127.0.0.1`/`::1` only, and require explicit config to add Docker service hostnames (`crowdsec`, `caddy`).

## Redirect policy (per call site)

- **Uptime monitor HTTP checks:** redirects should be **disabled**.
- **CrowdSec LAPI calls:** redirects should be **disabled**.
- **Caddy Admin API calls:** redirects should be **disabled**.
- **URL connectivity test (`url_testing.go`):** keep the current explicit redirect validation behavior (hop-validated) or disable redirects; either is acceptable if CodeQL sees a safe path.

## Proxy env var policy

- All SSRF-sensitive `http.Transport` instances used by the above call sites must set `Proxy: nil` (or otherwise guarantee proxy env vars are not consulted).
- [backend/internal/utils/url_testing.go](backend/internal/utils/url_testing.go) already demonstrates this; use it as a precedent to standardize behavior.

## Phases

### Phase 0 — Recon (local-only, not mergeable)

- Temporarily remove the `go/request-forgery` exclusion locally in
  [.github/codeql/codeql-config.yml](.github/codeql/codeql-config.yml)
  to surface the full finding list.
- Run VS Code task: `shell: Security: CodeQL Go Scan (CI-Aligned) [~60s]`.
- Record findings with file/function mapping.

**Exit criteria:** A concrete list of findings drives Phase 1+. This phase alone must not be merged.

### Phase 1 — Remediate uptime monitor outbound HTTP

- Update the uptime HTTP/HTTPS path to:
  - Validate the URL via `ValidateExternalURL` (and use the validated result, not the raw input).
  - Use a safe HTTP client (dial-time checks) with:
    - proxy env vars ignored
    - redirects disabled

**Acceptance criteria:** CodeQL no longer flags the uptime monitor request path.

### Phase 2 — Remediate CrowdSec handler direct LAPI calls

- Replace plain `http.Client` usage in:
  - `(*CrowdsecHandler).GetLAPIDecisions`
  - `(*CrowdsecHandler).CheckLAPIHealth`

With:

- strict validation of the configured LAPI base URL using the internal-service allowlist policy
- safe URL construction (`net/url` for query strings)
- safe HTTP client configuration:
  - redirects disabled
  - proxy env vars ignored

**Acceptance criteria:** CodeQL no longer flags these requests; behavior works for both localhost and Docker service-name deployments when explicitly allowlisted.

### Phase 3 — Remediate Caddy admin client base URL and HTTP client

- Validate `adminAPIURL` in `caddy.NewClient` using the internal-service allowlist policy.
- Use a safe HTTP client:
  - redirects disabled
  - proxy env vars ignored
  - dial-time validation enabled

**Acceptance criteria:** Admin API client cannot be pointed at arbitrary destinations.

### Phase 4 — Remove suppressions (atomic with phases 1–3)

- Remove the global query filter exclusion for `go/request-forgery` from
  [.github/codeql/codeql-config.yml](.github/codeql/codeql-config.yml).
- Remove any inline suppressions related to `go/request-forgery` that are no longer required (notably any `lgtm [go/request-forgery]` style comments).

**Acceptance criteria:** CodeQL Go scan passes without the global exclusion.

## Tests

- Add deterministic unit tests for:
  - URL validation rules in `ValidateExternalURL`
  - redirect behavior (disabled vs hop-validated) where applicable
  - internal-service allowlist behavior (localhost + explicit service-name allow)

Do not rely on real DNS/network. Prefer `httptest.Server` and validation-only failure cases.

## Definition of Done (DoD)

- VS Code task: `shell: Lint: Pre-commit (All Files)` passes.
- VS Code task: `shell: Test: Backend with Coverage` passes and backend coverage ≥ 85%.
- VS Code task: `shell: Security: CodeQL Go Scan (CI-Aligned) [~60s]` passes with `go/request-forgery` enabled (no global exclusion).
- Proxy env var behavior is deterministic for SSRF-sensitive clients.
- Redirect behavior is explicit per call site (disabled or hop-validated).
