
# SSRF Remediation Plan (Index)

This file is intentionally SSRF-focused only.

The authoritative, Supervisor-updated SSRF plan is:

- [docs/plans/ssrf-remediation.md](docs/plans/ssrf-remediation.md)

## Merge policy (Supervisor requirement)

- The global CodeQL exclusion for `go/request-forgery` in
  [.github/codeql/codeql-config.yml](.github/codeql/codeql-config.yml) must be removed
  in the same PR/merge as the underlying SSRF fixes.
- Phase 0 can include local-only recon (e.g., temporary local edit of CodeQL config to
  surface findings), but must not be a mergeable intermediate state.

## SSRF call sites (current known)

- Uptime monitor HTTP checks: `(*UptimeService).checkMonitor` in
  [backend/internal/services/uptime_service.go](backend/internal/services/uptime_service.go)
- CrowdSec LAPI: `(*CrowdsecHandler).GetLAPIDecisions` and
  `(*CrowdsecHandler).CheckLAPIHealth` in
  [backend/internal/api/handlers/crowdsec_handler.go](backend/internal/api/handlers/crowdsec_handler.go)
- Caddy Admin API: `caddy.NewClient` and `(*Client).Load/GetConfig/Ping` in
  [backend/internal/caddy/client.go](backend/internal/caddy/client.go)
- URL connectivity test (SSRF-sensitive client): `utils.TestURLConnectivity` in
  [backend/internal/utils/url_testing.go](backend/internal/utils/url_testing.go)

## Relocated content (no deletions)

- Patch coverage (Codecov) plan (previous Appendix A):
  [docs/plans/patch-coverage-codecov.md](docs/plans/patch-coverage-codecov.md)
- CodeQL/Trivy local scan hygiene notes (generated artifacts, skip dirs, etc.):
  [docs/plans/codeql-local-hygiene.md](docs/plans/codeql-local-hygiene.md)
- DNS provider feature spec (implementation-level):
  [docs/implementation/dns_providers_IMPLEMENTATION.md](docs/implementation/dns_providers_IMPLEMENTATION.md)
