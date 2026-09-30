# Plan: Proxy host security header profile must replace upstream headers (GH #1402)

Branch: `fix/security-headers-duplicated-1402` (single PR into `development`).
Type: small backend fix (`fix(security):`), no schema/API/frontend change.
Status: IMPLEMENTED (commits 1 and 2 landed on the branch; commit 3 is this docs update). Follow-ups filed: #1416 (gorm `default:true` bools cannot be set false), #1417 (optional stripping of disabled headers).

## 1. Problem and root cause

Issue: on a proxy host whose upstream already sends security headers (e.g. Charon's own UI), the response carries duplicates, including `Cross-Origin-Resource-Policy: cross-origin` + `same-origin` (invalid, browsers drop it).

### Correction to the issue's premise

The issue says the profile headers are "added on top" (append). The generated config already uses `set`, not `add`. Verified in `backend/internal/caddy/config.go` `buildSecurityHeadersHandler` (~L1449-1547), which returns:

```
{"handler":"headers","response":{"set": {...}}}
```

The real cause is ordering, not `set` vs `add`:

Flow (entry -> exit):
1. Entry: `ProxyHost.SecurityHeaderProfile` (or `SecurityHeadersEnabled` with `getDefaultSecurityHeaderProfile()`, ~L1609; profile CORP default `same-origin`).
2. Transformation: `GenerateConfig` (config.go ~L535) appends the headers handler to `handlers` *before* the `reverse_proxy` handler (main route and per-location routes both reuse `handlers`).
3. Persistence: none (config is regenerated and loaded into Caddy via the admin API).
4. Exit (Caddy runtime, `modules/caddyhttp/headers`, Caddy 2.11.4 per `Dockerfile` `CADDY_VERSION`): a non-deferred `response.set` is applied to the response header map immediately, when the handler runs, i.e. before the upstream is contacted. `reverse_proxy` then copies the upstream response headers into that same map using `Header.Add` semantics, so any header the upstream also sends ends up with two values (profile value first, upstream value second). `"deferred": true` (or `require`) makes Caddy wrap the ResponseWriter and apply the ops in `WriteHeader`, after reverse_proxy has copied upstream headers, so `set` actually replaces.

Confirmed empirically by the integration script (section 7): on the unfixed build the profile host returned two `Cross-Origin-Resource-Policy` values (`same-origin` and `cross-origin`); on the fixed build it returns one.

### Second layer: Charon's own middleware

`backend/internal/api/middleware/security.go` L46-69 (`SecurityHeaders`, mounted globally at `routes.go:169`) sets X-Frame-Options DENY, nosniff, X-XSS-Protection, Referrer-Policy, Permissions-Policy, COOP (non-dev), CORP `same-origin` on every Charon response. That is why Charon's UI is a header-emitting upstream. This is correct for direct access to Charon and is NOT the bug; it is just the "upstream" that triggers it. Any other upstream (nginx, apps) would trigger the same duplication.

### Other emit sites (grep of `"headers"` handlers, `HeaderHandler(`)

| Site | Upstream involved? | Action |
|---|---|---|
| `buildSecurityHeadersHandler` (config.go) | yes (reverse_proxy) | emit non-deferred + deferred `set` pair (the fix) |
| Legacy host HSTS via `HeaderHandler` (config.go ~L545) | yes (same route) | emit the same pair; skipped entirely when the profile already sets Strict-Transport-Security |
| `redirect_routes.go` L78 `HeaderHandler` HSTS | no (static redirect, no upstream) | single handler, unchanged |
| `types.go` `ReverseProxyHandler` request `set` | request side | unaffected |
| `AdvancedConfig` user handler | user-controlled | do not touch |
| Orthrus/remote-server paths | still go through same `GenerateConfig` route assembly (no separate header emitters found) | none |

Note `normalizeHeaderOps` (config.go ~L749-785) only normalizes `set` value types inside `response`/`request`; it leaves sibling keys such as `deferred` untouched, so no change is required there (add a regression assertion in a test).

## 2. Decision: "keep the middleware's values as default when no profile applies"

No code change. When a host has no profile and `SecurityHeadersEnabled == false`, `buildSecurityHeadersHandler` returns nil, no handler is emitted, and upstream headers (including the Charon middleware's, when Charon is the upstream) pass through untouched. When `SecurityHeadersEnabled == true` with no profile, the built-in default profile is authoritative and replaces upstream values. The middleware's values are already the pass-through default. Only document this.

## 3. Exact fix (as built)

A single deferred handler was tried first and rejected: it fixed the duplicate values but dropped the headers from Caddy-generated error responses (502), because the deferred wrapper never runs when reverse_proxy fails before writing a response. The integration test caught this. The final design emits TWO `set` handlers back to back with identical values:

1. `backend/internal/caddy/types.go`: `HeaderHandlers([]Handler)` returns a non-deferred `set` handler immediately followed by a `set` handler with `"deferred": true`, same values in both.
   - Non-deferred: applies before the upstream is contacted, so Caddy-generated error responses (502) carry the profile headers.
   - Deferred: applies at WriteHeader after reverse_proxy has copied upstream headers, so `set` replaces the upstream value and exactly one value (the profile's) is emitted.
2. `backend/internal/caddy/config.go` `buildSecurityHeaderHandlers` builds the profile pair via `HeaderHandlers`; `GenerateConfig` uses it for the main route and per-location routes.
3. The legacy per-host HSTS pair is skipped when the profile already sets Strict-Transport-Security, so the profile wins on both the deferred and non-deferred paths.
4. No `add`/`delete`/`replace` ops. `normalizeHeaderOps` leaves the `deferred` key untouched (asserted by a test).

### Behavior and edge cases

- Headers the profile sets: one value, the profile's.
- Unset/disabled headers are omitted from `set`, so an upstream value survives. A profile that disables a header does NOT strip an upstream value (Q4); tracked in #1417.
- No profile and `SecurityHeadersEnabled == false`: no handler emitted, upstream headers pass through untouched. With `SecurityHeadersEnabled == true` and no profile, the built-in default profile applies.
- CSP vs CSP-Report-Only: a profile emits one or the other. A report-only profile does not remove an upstream enforcing CSP; both reach the browser (Q2).
- Legacy HSTS plus profile HSTS: profile wins (Q3).
- Caddy-generated error responses (502) carry the profile headers (asserted, passing).
- Streaming/WebSocket: verified by the integration script.
- Related, out of scope: `default:true` bool fields cannot currently be set false (#1416).

## 4. Tests

Unit tests (`backend/internal/caddy/`): existing security-header, `HeaderHandler`, redirect-route and `config_test.go` expectations updated for the handler pair; new tests cover the pair shape (non-deferred then deferred, identical values, no `add`/`delete`/`replace`), main and location routes, omission of unset headers, no handler when no profile and headers disabled, `normalizeHeaderOps` preserving `deferred`, and legacy HSTS being skipped when the profile sets HSTS. `backend/internal/api/middleware/security_test.go` is unchanged.

## 5. Integration test

Real Caddy is required, so the repo's bash-script-plus-build-tagged-Go-wrapper pattern is used:

- `scripts/security_headers_integration.sh`, `backend/integration/security_headers_integration_test.go` (tag `integration`), `.github/skills/integration-test-security-headers.SKILL.md` + `-scripts/run.sh`, registered in `scripts/integration-test-all.sh`, plus a `security-headers:` job in `.github/workflows/integration-tests.yml`.
- Upstream sends `X-Content-Type-Options`, `Cross-Origin-Resource-Policy: cross-origin` and a custom `X-Upstream-Only: keep`.
- Asserts (HEAD and GET): profile headers appear exactly once with the profile's values; the custom header passes through; a host with no profile passes upstream CORP through once; the loaded config contains the deferred handler.
- 502 assertion (real, passing): with the upstream stopped, the profile host returns 502 carrying each profile header exactly once.
- Streaming assertion: 200 through the profile host with profile headers once and incremental chunks.

## 6. Docs impact

- `docs/features/security-headers.md`: new plain-language section on how profiles interact with an app's own headers.
- `CHANGELOG.md` (hand-maintained): one line under Unreleased > Fixed.
- No changes to `docs/features.md` or `ARCHITECTURE.md`; no `docs-site` edits; GORM scan not required.

## 7. Commit Slicing Strategy (one PR)

Decision: single PR, ordered commits. EVERY commit builds and passes its validation gate (no red commits). The `fix(security):` subject must stay vague: no header names, "duplicate", "CORP", or "upstream". Never include a session ID or link.

| # | Subject | Scope / files | Depends | Validation gate |
|---|---|---|---|---|
| 1 | `fix(security): harden response header handling in the proxy layer` | `backend/internal/caddy/types.go` (`HeaderHandlers`), `backend/internal/caddy/config.go` (`buildSecurityHeaderHandlers`, legacy HSTS skip), plus unit tests | none | `cd backend && go test ./internal/caddy/... ./internal/api/...`; `make lint-fast`; coverage >= 85%; `bash scripts/local-patch-report.sh`; `lefthook run pre-commit` |
| 2 | `test: add integration coverage for response header handling` | Integration script, Go wrapper, skill, `integration-test-all.sh`, workflow job (section 5) | 1 | Script passes locally against the fixed build; `go vet -tags integration ./integration/...`; CI integration job green |
| 3 | `docs: document security header profile behavior` | `docs/features/security-headers.md`, `CHANGELOG.md`, this plan | 1 | markdown lint via lefthook |

### Unfixed-vs-fixed proof (captured)

The integration script was run against the unfixed build first: the profile host returned two `Cross-Origin-Resource-Policy` values (`same-origin` and `cross-origin`). After the fix it returns one, and the 502 response carries the profile headers. Both captures go in the PR body.

Rollback: revert commit 1; commits 2 and 3 do not affect runtime behavior.

## 8. Acceptance criteria

- Generated Caddy config: profile emits a non-deferred `set` handler followed by a deferred `set` handler with identical values (main and location routes); legacy HSTS is skipped when the profile sets HSTS.
- Integration: upstream sending nosniff + `CORP: cross-origin` yields exactly one of each header, with the profile's values, through a profile-assigned proxy host; unset headers (custom upstream header) pass through; no-profile host unchanged; 502 responses carry the profile headers.
- All existing unit tests pass; backend coverage >= 85%; patch report generated; lefthook and staticcheck clean.

## 9. Decisions on former open questions (resolved)

1. Empirical confirmation of Caddy 2.11.4 behavior: done via the integration script (unfixed two values, fixed one). It also showed a deferred-only handler drops headers from 502 responses, hence the handler pair.
2. CSP-Report-Only profile plus upstream enforcing CSP: both reach the browser; documented.
3. Legacy HSTS vs profile HSTS: profile wins (legacy pair skipped); unit tested; release note added.
4. A profile that disables a header does NOT strip an upstream value: documented; follow-up #1417.
