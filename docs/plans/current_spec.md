# Spec: Redirection Hosts — 301/302/307/308 URL Redirects (Issue #1367, part 2)

## 1. Introduction

### 1.1 Overview

GitHub issue #1367's second half reports: **"no clear option to set 301
redirection host."** Charon currently only supports proxying a domain to a
backend service (`ForwardScheme`/`ForwardHost`/`ForwardPort` on
`ProxyHost` → a Caddy `reverse_proxy` handler, see
`backend/internal/caddy/config.go`). There is no way to configure a domain
to instead issue an HTTP redirect to a different URL. The `AdvancedConfig`
raw-JSON escape hatch on `ProxyHost` is technically capable of producing a
redirect handler, but per this repo's Big Picture ("Everything should
prioritize simplicity, usability... users should feel like they have
enterprise-level security and features with zero effort") it is not a
discoverable or novice-usable path — it requires hand-writing Caddy JSON.

Nginx Proxy Manager (a common migration source for Charon's audience) ships
this as a first-class "Redirection Host" concept: pick domain(s), a target
URL/scheme, an HTTP status code, and whether to preserve the incoming
path/query string. This spec adds the equivalent to Charon.

### 1.2 Objectives

- Give users a dedicated, discoverable UI to configure a domain to redirect
  to another URL with a chosen status code, without touching JSON.
- Support a bounded, meaningful set of redirect status codes: **301, 302,
  307, 308**.
- Correctly terminate TLS for the redirecting domain (it still needs a
  certificate — the browser hits Charon's Caddy instance over HTTPS before
  following the redirect) and emit the redirect via Caddy's native
  `static_response` handler mechanism.
- Do this without destabilizing the existing `ProxyHost` model, its GORM
  schema, its 1000+-line handler, its Caddy config generation path, or its
  1600-line frontend form — all of which assume "this host has a real,
  reachable backend."

### 1.3 Non-Goals (v1)

- **Per-path redirect rules.** A Redirection Host redirects the *entire*
  domain (all paths) to a target; NPM's own per-path "Custom Locations" for
  redirects and Charon's per-path `Location` model for proxying are both
  out of scope. A whole-host redirect is what the issue asks for.
- **Wildcard-domain redirect targets** (e.g. rewriting `*.example.com` to
  a templated target per-subdomain). Domain *matching* still supports the
  existing comma-separated multi-domain list (same as `ProxyHost`), but the
  target URL is a single fixed destination.
- **Redirect-chain / redirect-loop graph analysis.** Only a direct
  self-redirect guard is included (a Redirection Host may not target one of
  its own source domains) — detecting `A→B→A` chains across multiple hosts
  is not attempted in v1.
- **Access Lists, WAF, CrowdSec, rate limiting, forward-auth on Redirection
  Hosts.** These all assume there's a backend worth protecting; a
  `static_response` redirect has no backend and minimal attack surface
  beyond serving a 3xx. They can be considered in a future iteration if a
  real need surfaces.
- **Uptime monitoring** of Redirection Hosts (no backend to health-check
  in the `UptimeMonitor` sense; the existing `TestConnection` semantics for
  `ForwardHost:ForwardPort` reachability do not map onto a redirect target).
- **Migrating/converting** an existing `ProxyHost` into a Redirection Host
  in-place (a user who wants this deletes the proxy host and creates a
  redirection host instead, same as NPM's UX).
- **Fixing `ProxyHostService.ValidateUniqueDomain`'s pre-existing
  same-table reordering gap.** Today it compares `domain_names` as a whole
  string (`"a.com,b.com"` vs `"b.com,a.com"` does not collide), so two
  `ProxyHost` rows with a partial comma-list overlap are silently allowed.
  This predates this feature, affects `ProxyHost`-vs-`ProxyHost` only, and
  is left completely untouched by this PR (see §4.3) — fixing it is
  recommended as a separate, independently-reviewable follow-up ticket,
  not something to bundle into an unrelated new-feature PR.

## 2. Research Findings

### 2.1 How `ProxyHost` → Caddy route generation works today

`GenerateConfig` in `backend/internal/caddy/config.go` (function starting
line 20) is the single transformation point from DB rows to Caddy JSON. For
every enabled `ProxyHost` it, in order:

1. Parses/dedupes `DomainNames` (comma-separated) into `uniqueDomains`,
   feeding a package-level `processedDomains` map used for cross-host
   "Ghost Host" duplicate detection (config.go:497-556).
2. Collects domains needing ACME (HTTP-01) vs. DNS-01 challenges to build
   `tls.automation.policies` (config.go:82-417) — this is where TLS
   certs get issued/renewed per domain.
3. Collects custom (`provider == "custom"`) certificates for
   `tls.certificates.load_pem` (config.go:419-486).
4. Builds a per-host handler chain: security pre-handlers (CrowdSec, WAF,
   rate-limit, ACL) → security headers/HSTS/exploit-block → **for every
   `Location`, a `reverse_proxy` sub-route** → **an "emergency route"** that
   always reverse-proxies reserved paths (`/api/v1/emergency/*`) regardless
   of WAF/ACL state → **the main route**, which reverse-proxies
   `ForwardHost:ForwardPort` (config.go:687-790, via
   `ReverseProxyHandler` in `types.go:129`).

Every one of the last four sub-steps assumes a real, dialable backend
(`fmt.Sprintf("%s:%d", host.ForwardHost, host.ForwardPort)`). There is no
branch point in this function today that skips proxying — it is not a
matter of conditionally omitting one call, it's a structurally different
route (no locations, no emergency route, no reverse-proxy handler at all;
just one `static_response` handler with a `Location` header).

### 2.2 Caddy JSON shape for a redirect

Caddy has no separate `http.handlers.redir` JSON module. The Caddyfile
`redir` directive is sugar that compiles to `http.handlers.static_response`
with a `Location` response header and a 3xx `status_code` — confirmed by
Caddy's JSON schema (https://caddyserver.com/docs/json/apps/http/handlers/static_response/).
The idiomatic JSON handler is:

```json
{
  "handler": "static_response",
  "status_code": 301,
  "headers": { "Location": ["https://example.com/{http.request.uri}"] }
}
```

- `{http.request.uri}` is Caddy's placeholder for the full incoming
  path+query — used only when "preserve path & query string" is enabled.
- When not preserving, `Location` is the literal target URL with no
  placeholder appended.
- `status_code` is a plain int here (unlike `reverse_proxy`, no separate
  "close" or upstream config needed).

This matches the existing `Handler` type (`type Handler map[string]any`,
`types.go:124`) and the existing `HeaderHandler`/`RewriteHandler` handler
builders' style (`types.go:204-237`) — no new Caddy-side types are needed
beyond the `Route`/`Match`/`Handler` structs that already exist.

### 2.3 `ProxyHost` schema constraints that make Option B costly

`backend/internal/models/proxy_host.go`: `ForwardHost` is
`gorm:"not null;index"`, `ForwardPort` is `gorm:"not null"`. Both are
required, and are consumed far beyond `GenerateConfig`:

- `proxyhost_service.go:validateProxyHost` (line 120) rejects an empty
  `ForwardHost` and validates it as an IP/hostname.
- `proxyhost_service.go:TestConnection` (line 283) opens a TCP connection
  to `ForwardHost:ForwardPort` — used by the frontend's "Test Connection"
  button and by `UptimeService.SyncAndCheckForHost`, which creates an
  uptime monitor keyed on the same dial target on every create.
- `proxy_host_handler.go:generateForwardHostWarnings` (line 117) inspects
  `ForwardHost` for Docker-bridge/private-IP advisories — meaningless for a
  redirect target, which is a full URL, not a host:port.

Making these nullable (Option B) means every one of those call sites needs
an `is_redirect`-branch added, `ForwardHost`/`ForwardPort` become
conditionally-required (harder to express/validate cleanly than a genuinely
separate resource), and the DB column semantics become dual-purpose
(`forward_host` sometimes means "backend to proxy to", sometimes would need
to mean "redirect target" — worse, a URL doesn't fit the existing
IP/hostname-only `ForwardHost` validation at all, so new columns would be
needed anyway, e.g. `redirect_target`/`redirect_status_code`, on top of the
existing ones). That's schema growth on the hottest, most-consumed model in
the codebase, for a feature that structurally shares almost nothing with
proxying.

### 2.4 Precedent: sibling models are how this codebase adds host-adjacent concepts

`ProxyGroup` (`backend/internal/models/proxy_group.go`) is the existing
precedent for "a concept that relates to hosts but isn't itself host
proxying": its own model, own `BeforeCreate` UUID hook, own handler
(`proxy_group_handler.go`), its own migration entry ordered **before**
`ProxyHost` in `routes.go`'s `AutoMigrate` call (line 113, "must precede
ProxyHost (FK dependency)"), and `ProxyHost` merely holds a nullable FK
(`ProxyGroupID *uint`) to it. Redirection Hosts don't even need a FK back
into `ProxyHost` — they're peers, not children.

### 2.5 Frontend structure

- `frontend/src/pages/ProxyHosts.tsx` (1560 lines) and
  `frontend/src/components/ProxyHostForm.tsx` (1641 lines) are both large
  and already carry significant state (bulk ACL/group/security-header
  modals, drag-and-drop grouping, DNS challenge section, etc.). Threading a
  redirect-mode branch through this form would touch most of its sections
  (SSL cert section stays, but forward-scheme/host/port, locations,
  websocket, application-preset, forward-auth, WAF, ACL sections would all
  need to become conditionally hidden).
- Navigation is a flat array in `frontend/src/components/Layout.tsx`
  (`navigation: NavItem[]`, starting line ~119); adding a new top-level
  item (`{ name: t('navigation.redirectionHosts'), path: '/redirection-hosts', icon: '↪️' }`)
  immediately below the existing Proxy Hosts entry is a one-line,
  low-risk addition and directly solves the issue's "no clear option" /
  discoverability complaint — a dedicated menu entry beats a hidden toggle.
- Routing is a flat `<Routes>` tree in `frontend/src/App.tsx` (~line 56
  onward); a new top-level `<Route path="redirection-hosts" element={<RedirectionHosts />} />`
  sibling to the existing `<Route path="proxy-hosts" element={<ProxyHosts />} />`
  (line 77) follows the exact same pattern.
- `frontend/src/pages/Domains.tsx` is unrelated prior art (a simple
  free-standing domain-name registry, not proxy/redirect config) — not
  reusable here beyond confirming the "small dedicated page" pattern is
  normal in this codebase.

### 2.6 Domain-uniqueness enforcement is per-model today — this must be extended

`ProxyHostService.ValidateUniqueDomain` (`proxyhost_service.go:57`) checks
uniqueness only within `proxy_hosts.domain_names`. `GenerateConfig`'s
"Ghost Host" `processedDomains` map (config.go:498) is scoped to a single
call, only over the `[]models.ProxyHost` slice it's given. **Neither knows
about the other resource type.** If Redirection Hosts are added as a
separate table, a domain could silently be claimed by both a `ProxyHost`
and a `RedirectionHost` with no error today — Caddy would non-deterministically
route to whichever route list-position wins. This is a real gap this spec
must close (see §4.3 and §6).

### 2.7 SSRF / open-redirect review (`internal/network/safeclient.go`)

`internal/network` implements SSRF protection (blocking private/reserved
IP ranges) for *outbound server-initiated requests* Charon itself makes —
e.g. `TestConnection`'s TCP dial to a proxy backend, or any DNS-provider
API call. A Redirection Host's target URL is never fetched or dialed by
the Charon server; it is only ever placed in an HTTP `Location` header sent
back to the *browser that already owns the request*, matching the
domain the *domain owner already controls and has a valid TLS cert for*.
There is no SSRF vector here — `safeclient.go` is not applicable to this
feature, and this spec deliberately does **not** validate the target URL
against the private-IP blocklist that `network.IsPrivateIP`/`privateBlocks`
enforce, since redirecting your own domain to `http://192.168.1.1/` (a
device on your own LAN) is a legitimate, common self-hosted use case, not
an attack.

The one real concern is **user error, not attacker input**: a user could
paste a malformed or attacker-supplied URL as a redirect target. Since the
person configuring it is the authenticated domain owner (not an untrusted
third party), this is the same trust boundary as `ProxyHost.ForwardHost`
today (also fully user-controlled, also unvalidated against SSRF rules) —
consistent with existing precedent. Basic hygiene (`net/url.Parse`,
scheme allowlist `http`/`https`, reject empty host) is enforced for input
correctness, not security, and is called out explicitly for `qa-security`
review. The self-redirect guard (§4.3) is the one abuse case worth a hard
check, since it's a footgun (infinite redirect loop against yourself) a
novice user could trigger accidentally.

## 3. Scope Decision

**Recommendation: Option A — a new, separate `RedirectionHost` model and
resource**, mirroring NPM's Redirection Host concept, as a peer to
`ProxyHost` rather than a mode flag on it.

Rationale (see §2.1–2.6 for full detail):

| Criterion | Option A (new model) | Option B (extend `ProxyHost`) |
|---|---|---|
| Caddy config generation fit | Clean: one small `static_response` handler, no locations/emergency-route/reverse-proxy machinery to skip | Poor: every proxy-only code path in `GenerateConfig`'s per-host loop needs an `if host.IsRedirect` branch |
| Schema impact | Additive only — new table, zero changes to `ProxyHost` | `ForwardHost`/`ForwardPort` (`not null`) become conditionally required; needs new columns anyway (URL doesn't fit hostname validation) |
| Blast radius on existing consumers | None — `ProxyHostService`, `ProxyHostHandler`, `TestConnection`, `UptimeService`, frontend `ProxyHostForm`/`ProxyHosts.tsx` all untouched | All of the above need redirect-aware branches |
| Discoverability (the issue's actual complaint) | New nav item + page = maximally discoverable | Buried as a toggle inside an already-1641-line form |
| Precedent in this codebase | Matches `ProxyGroup` (sibling model, own migration entry, own handler) | No existing precedent for a dual-purpose `ProxyHost` |
| SSL cert / domain handling | Reuses the exact same `CertificateID`/`SSLCertificate` FK pattern and TLS automation-policy domain collection, extended to a second slice | Same reuse, no advantage either way |

The one real cost of Option A is that TLS automation-policy domain
collection and Ghost-Host duplicate detection in `GenerateConfig` must be
extended to consider both `[]models.ProxyHost` and `[]models.RedirectionHost`
together (§4.3, §6). `GenerateConfig`'s call signature does need to change
to receive that data — verified against the actual codebase, this touches
92 call sites across 9 test files plus a `generateConfigFunc` stub
indirection in 3 more files (§4.2 has the exhaustive, file-and-line
breakdown). The design in §4.2 keeps that change non-breaking for all but
~13 lines across 8 files by converting the function's existing trailing
variadic into a functional-options type rather than inserting a new
positional parameter — still real work, but bounded and enumerated, not a
vague "well-contained" claim. This is still a smaller, more localized
change than threading an `if host.IsRedirect` conditional through
`GenerateConfig`'s entire per-host body under Option B.

## 4. Technical Specifications

### 4.1 Data Model

New file `backend/internal/models/redirection_host.go`:

```go
package models

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// RedirectStatusCode enumerates the HTTP redirect status codes Charon
// supports for Redirection Hosts. Only 301/302/307/308 are exposed —
// see docs/plans/current_spec.md §5 for why 300/303/304 are excluded.
type RedirectStatusCode int

const (
	RedirectPermanent            RedirectStatusCode = 301 // Moved Permanently
	RedirectFound                RedirectStatusCode = 302 // Found (temporary)
	RedirectTemporaryPreserve    RedirectStatusCode = 307 // Temporary Redirect (method-preserving)
	RedirectPermanentPreserve    RedirectStatusCode = 308 // Permanent Redirect (method-preserving)
)

// ValidRedirectStatusCodes is the bounded set accepted by validation.
var ValidRedirectStatusCodes = map[int]bool{
	301: true,
	302: true,
	307: true,
	308: true,
}

// RedirectionHost represents a domain (or set of domains) that Charon
// terminates TLS for and immediately redirects to a target URL, instead
// of reverse-proxying to a backend. Peer resource to ProxyHost — see
// docs/plans/current_spec.md §3 for why this is a separate model.
type RedirectionHost struct {
	ID          uint   `json:"-" gorm:"primaryKey"`
	UUID        string `json:"uuid" gorm:"uniqueIndex;not null"`
	Name        string `json:"name" gorm:"index"`

	// DomainNames: comma-separated, same convention as ProxyHost.DomainNames.
	DomainNames string `json:"domain_names" gorm:"not null;index"`

	// Target: full target URL, e.g. "https://newsite.example.com" or
	// "https://newsite.example.com/new-path". Scheme is part of the URL,
	// unlike ProxyHost which splits ForwardScheme/ForwardHost/ForwardPort —
	// a redirect target is a browser-facing URL, not a dial address.
	TargetURL string `json:"target_url" gorm:"not null"`

	// StatusCode: one of 301, 302, 307, 308. Validated against
	// ValidRedirectStatusCodes at the service layer; no DB-level CHECK
	// constraint (SQLite CHECK support via GORM is inconsistent across
	// migrations run against pre-existing DBs, so this stays app-level,
	// consistent with how ProxyHost.ForwardScheme is validated).
	StatusCode int `json:"status_code" gorm:"not null;default:301"`

	// PreservePath: when true, the incoming request's path+query string is
	// appended to TargetURL via Caddy's {http.request.uri} placeholder.
	// When false, TargetURL is used verbatim regardless of the incoming path.
	PreservePath bool `json:"preserve_path" gorm:"default:true"`

	SSLForced    bool `json:"ssl_forced" gorm:"default:true"`
	HTTP2Support bool `json:"http2_support" gorm:"default:true"`

	// HSTS mirrors ProxyHost's fields for consistency — a redirecting
	// domain still terminates HTTPS and can reasonably advertise HSTS.
	HSTSEnabled    bool `json:"hsts_enabled" gorm:"default:false"`
	HSTSSubdomains bool `json:"hsts_subdomains" gorm:"default:false"`

	Enabled bool `json:"enabled" gorm:"default:true;index"`

	// Certificate: same FK pattern as ProxyHost.CertificateID/Certificate.
	CertificateID *uint           `json:"certificate_id" gorm:"index"`
	Certificate   *SSLCertificate `json:"certificate" gorm:"foreignKey:CertificateID"`

	// DNS Challenge configuration — same pattern as ProxyHost, needed
	// because wildcard redirect-source domains still need DNS-01 issuance.
	DNSProviderID   *uint        `json:"dns_provider_id,omitempty" gorm:"index"`
	DNSProvider     *DNSProvider `json:"dns_provider,omitempty" gorm:"foreignKey:DNSProviderID"`
	UseDNSChallenge bool         `json:"use_dns_challenge" gorm:"default:false"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// BeforeCreate assigns a UUID if one has not been set (mirrors ProxyGroup).
func (r *RedirectionHost) BeforeCreate(tx *gorm.DB) (err error) {
	if r.UUID == "" {
		r.UUID = uuid.New().String()
	}
	return
}
```

Explicitly **excluded** from this model (see §1.3 Non-Goals): `Locations`,
`AccessListID`, `WAFDisabled`, `ForwardAuthEnabled`, `SecurityHeaderProfileID`/
inline security-header fields, `EnableStandardHeaders`, `ProxyGroupID`,
`AdvancedConfig`. These are all proxy-specific concepts (backend header
injection, ACL, WAF) that don't apply to a stateless redirect response, or
(in `AdvancedConfig`'s case) reintroduce the "not discoverable" problem
this feature exists to solve. `AdvancedConfig` can be revisited in a future
iteration if real demand appears (e.g. a user wanting a custom response
header on the redirect).

**Migration**: `backend/internal/api/routes/routes.go`'s `AutoMigrate` call
(line 112) gets one new line: `&models.RedirectionHost{},`. Per the
`ProxyGroup` precedent comment ("must precede ProxyHost (FK dependency)"),
`RedirectionHost` has no FK *to* `ProxyHost`/`ProxyGroup`, only its own FKs
to `SSLCertificate`/`DNSProvider` (both already migrated earlier in the
list) — so it can be added anywhere after those two, e.g. directly after
`&models.ProxyHost{}, &models.Location{},` for readability (grouping it
next to its sibling resource).

### 4.2 Caddy Config Generation

New function in `backend/internal/caddy/config.go` (or a new file
`backend/internal/caddy/redirect_routes.go` if preferred for file-size
hygiene — recommended, since `config.go` is already 1792 lines):

```go
// BuildRedirectRoutes converts enabled RedirectionHosts into Caddy routes.
// Mirrors the domain-parsing/dedup conventions in GenerateConfig but emits
// a single static_response handler per host instead of a reverse_proxy
// chain, since a redirect has no backend to proxy to.
func BuildRedirectRoutes(redirectHosts []models.RedirectionHost, processedDomains map[string]bool) ([]*Route, []string) {
	routes := make([]*Route, 0, len(redirectHosts))
	ipSubjects := make([]string, 0)

	for i := len(redirectHosts) - 1; i >= 0; i-- { // newest-first, same as GenerateConfig
		rh := redirectHosts[i] // #nosec G602
		if !rh.Enabled || rh.DomainNames == "" || rh.TargetURL == "" {
			continue
		}

		uniqueDomains, isIPOnly := dedupeAndTrackDomains(rh.DomainNames, processedDomains, "redirection_host", rh.UUID)
		if len(uniqueDomains) == 0 {
			continue
		}
		if isIPOnly {
			ipSubjects = append(ipSubjects, uniqueDomains...)
		}

		location := rh.TargetURL
		if rh.PreservePath {
			location += "{http.request.uri}"
		}

		handlers := []Handler{}
		if rh.HSTSEnabled {
			hstsValue := "max-age=31536000"
			if rh.HSTSSubdomains {
				hstsValue += "; includeSubDomains"
			}
			handlers = append(handlers, HeaderHandler(map[string][]string{
				"Strict-Transport-Security": {hstsValue},
			}))
		}
		handlers = append(handlers, RedirectHandler(location, rh.StatusCode))

		routes = append(routes, &Route{
			Match:    []Match{{Host: uniqueDomains}},
			Handle:   handlers,
			Terminal: true,
		})
	}

	return routes, ipSubjects
}
```

New handler builder in `types.go`, alongside `HeaderHandler`/`RewriteHandler`:

```go
// RedirectHandler creates a static_response handler that issues an HTTP
// redirect to targetURL with the given status code. targetURL may contain
// the {http.request.uri} placeholder when the caller wants to preserve the
// incoming path/query string.
func RedirectHandler(targetURL string, statusCode int) Handler {
	return Handler{
		"handler":     "static_response",
		"status_code": statusCode,
		"headers": map[string]any{
			"Location": []string{targetURL},
		},
	}
}
```

**Wiring into `GenerateConfig` — verified call-site impact and
non-breaking design**: re-checked directly against current code.
`GenerateConfig`'s signature (`config.go:20`) already ends in a trailing
variadic, `encSvc ...*crypto.EncryptionService`, used internally at
`config.go:434-435` (`if len(encSvc) > 0 && encSvc[0] != nil { certEncSvc
= encSvc[0] }`). A grep of `GenerateConfig(` finds **92 actual call sites
across 9 files** (`config_generate_test.go`, `config_extra_test.go`,
`config_crowdsec_test.go`, `config_customcert_test.go`, `config_test.go`,
`config.go` itself, `validator_test.go`, `config_patch_coverage_test.go`,
`config_generate_additional_test.go`, `client_test.go`,
`manager_additional_test.go`), plus the `generateConfigFunc` package-var
indirection (`manager.go:34`, `generateConfigFunc = GenerateConfig`)
reassigned to hand-written stub closures matching its full signature in
`manager_additional_test.go` (5 closures), `manager_patch_coverage_test.go`
(3 closures), and `manager_ssl_provider_test.go` (1 shared helper,
`mockGenerateConfigFunc`, reused by 7 test functions). Inserting a new
required positional parameter anywhere in the list — including the
"variadic fallback" of a second variadic — is not directly possible
(Go allows only one variadic parameter and it must be last), and a naive
positional insertion would break most of these call sites at compile
time, exactly as supervisor flagged.

Grepping the actual call sites (not assumed) shows the vast majority — all
but two — omit the trailing variadic entirely, passing exactly the 16
non-variadic positional arguments (e.g. `config_generate_test.go:28,75`;
every multi-line call in `config_patch_coverage_test.go`). Only two call
sites supply an explicit value into it:
`config_customcert_test.go:43` and `:160` (`..., nil, encSvc)`), plus the
one production call site, `manager.go:439`
(`..., dnsProviderConfigs, m.encSvc)`).

**Primary design (not a contingency): convert the trailing variadic from
a concrete type into a small functional-options type**, so the first 16
positional parameters are completely unchanged and the ~94 call sites that
omit the variadic keep compiling with zero edits:

```go
// GenerateConfigOption configures optional inputs to GenerateConfig that
// don't belong in its long positional parameter list. Introduced by the
// Redirection Hosts feature specifically so adding RedirectionHost input
// does not require a breaking positional-parameter insertion — see
// docs/plans/current_spec.md §4.2.
type GenerateConfigOption func(*generateConfigOptions)

type generateConfigOptions struct {
	encSvc        *crypto.EncryptionService
	redirectHosts []models.RedirectionHost
}

// WithEncryptionService supplies the optional encryption service GenerateConfig
// previously received via its bare *crypto.EncryptionService variadic parameter.
func WithEncryptionService(svc *crypto.EncryptionService) GenerateConfigOption {
	return func(o *generateConfigOptions) { o.encSvc = svc }
}

// WithRedirectionHosts supplies enabled RedirectionHost rows for
// BuildRedirectRoutes and combined TLS/Ghost-Host domain handling.
func WithRedirectionHosts(hosts []models.RedirectionHost) GenerateConfigOption {
	return func(o *generateConfigOptions) { o.redirectHosts = hosts }
}
```

`GenerateConfig`'s only signature change is its final parameter's type:
`encSvc ...*crypto.EncryptionService` becomes `opts ...GenerateConfigOption`
(every parameter before it — all 16 — is untouched in name, type, and
order). Inside the function body, `config.go:434-435`'s
`len(encSvc) > 0 && encSvc[0] != nil` check is replaced by resolving
`opts` into a local `generateConfigOptions{}` at the top of the function
and reading `resolved.encSvc`/`resolved.redirectHosts` from it.

**Exhaustive list of required edits** (everything not listed compiles
unchanged):

| File | Edit |
|---|---|
| `config.go:20` | Change trailing parameter type only, as above |
| `config.go:434-435` | Read `certEncSvc` from resolved options instead of `encSvc[0]` |
| `manager.go:439` | `..., dnsProviderConfigs, m.encSvc)` → `..., dnsProviderConfigs, WithEncryptionService(m.encSvc), WithRedirectionHosts(redirectHosts))` — this edit is required regardless of approach, since this is the one call site that must actually start passing redirect hosts through (§4.2 wiring). `Manager.ApplyConfig` (`manager.go:105`, which already does `m.db.Preload(...).Find(&hosts)` for `ProxyHost`) gains a sibling `m.db.Preload("Certificate").Preload("DNSProvider").Find(&redirectHosts)` call to populate that variable |
| `config_customcert_test.go:43,160` | `..., nil, encSvc)` → `..., nil, WithEncryptionService(encSvc))` (2 lines) |
| `manager_ssl_provider_test.go:21-22` | `mockGenerateConfigFunc`'s returned func type and func literal: trailing `...*crypto.EncryptionService` → `...GenerateConfigOption` (the 8 call sites that invoke this helper elsewhere in the file need no changes) |
| `manager_additional_test.go` | 5 inline `generateConfigFunc = func(...)` closures (~lines 426, 604, 655, 710, 815): same trailing-parameter type edit, pure signature match, no body-logic change (none of these stubs read `encSvc`'s value) |
| `manager_patch_coverage_test.go` | 3 inline `generateConfigFunc = func(...)` closures (~lines 60, 114, 178): same mechanical edit |

That is the complete, concrete set of edits commit 4 makes to satisfy the
type checker — 8 files, ~13 line-level edits, all mechanical
signature-matching with no behavior change to any existing test's
assertions. The remaining ~85+ call sites across
`config_generate_test.go`, `config_extra_test.go`,
`config_crowdsec_test.go`, `config_test.go`, `validator_test.go`,
`config_generate_additional_test.go`, and `client_test.go` need **no
changes** — they omit the variadic today and continue to do so.

Inside `GenerateConfig`, once `redirectHosts` is resolved from options:

- The existing `processedDomains` map (config.go:498) and TLS
  automation-policy domain collection loop (config.go:96-417) must
  consider `redirectHosts`' domains too, so a Redirection Host's domain (a)
  gets a TLS cert issued, and (b) is registered in `processedDomains`
  *before* `ProxyHost` routes are built, so a `ProxyHost` cannot silently
  steal a domain already claimed by a `RedirectionHost` (or vice versa —
  whichever collection runs first wins, and the loser is dropped with the
  existing "Ghost Host detection" warning log, extended to say which
  resource type won).
- `BuildRedirectRoutes`' resulting routes are appended to `routes` *before*
  the `ProxyHost` loop's routes (config.go:520 loop), since Caddy matches
  routes in list order and dedup already prevents genuine collisions — the
  ordering only matters for the log message clarity, not behavior.
- `ipSubjects` from `BuildRedirectRoutes` are merged into the existing
  `ipSubjects` slice (config.go:559, 807-810) for the internal-issuer
  automation policy.

### 4.3 Cross-Resource Domain Uniqueness

Re-verified against current code
(`backend/internal/services/proxyhost_service.go:57-74`):
`ProxyHostService.ValidateUniqueDomain` today runs exactly one query —
`s.db.Model(&models.ProxyHost{}).Where("domain_names = ?", domainNames)`
(optionally `AND id != ?`) — a whole-string, exact-match comparison against
`ProxyHost` only. It does **not** split on commas and does **not** know
`RedirectionHost` exists. This spec makes **zero changes** to this
function: same query, same whole-string semantics, same pre-existing
reordering gap (`"a.com,b.com"` vs `"b.com,a.com"` silently not colliding).
That gap is real but out of scope for this feature — see the new Non-Goal
below — fixing it would change accepted/rejected outcomes for existing
`ProxyHost`-only rows, which contradicts the Acceptance Criteria's "zero
changes to pre-existing ProxyHost-only validation behavior."

Two **strictly additive** changes are made instead, both flagged in §2.6:

1. **New shared cross-table helper** — `backend/internal/services/domain_uniqueness.go`:
   ```go
   // CheckDomainConflict returns an error if any comma-separated domain in
   // domainNames is already claimed by a row in the *other* resource table
   // (ProxyHost when checking from RedirectionHost, and vice versa). It never
   // queries the caller's own table — each service's existing same-table
   // check (unchanged) already covers that. Comparison is per-individual
   // domain (both sides' comma-separated lists are split and lowercased),
   // since the two tables have no shared string convention to exact-match
   // against.
   //
   // otherTable is the GORM model of the *other* resource, e.g. a
   // RedirectionHost service passes &models.ProxyHost{} and vice versa.
   func CheckDomainConflict(db *gorm.DB, domainNames string, otherTable any) error
   ```
   `ProxyHostService.Create`/`Update` and the new
   `RedirectionHostService.Create`/`Update` each call **both**: their own
   existing/new same-table check (unchanged for `ProxyHost`; a new
   same-table check for `RedirectionHost`, scoped only to other
   `RedirectionHost` rows) **and** this new `CheckDomainConflict` helper
   against the other table. `CheckDomainConflict` is purely additive — it
   only ever *adds* a new rejection reason (a domain claimed by the other
   resource type); it never relaxes, replaces, or bypasses either
   service's existing same-table check. Concretely:
   - `ProxyHostService.Create`/`Update`: unchanged
     `ValidateUniqueDomain(domainNames, excludeID)` call, **plus** a new
     `CheckDomainConflict(db, domainNames, &models.RedirectionHost{})` call.
   - `RedirectionHostService.Create`/`Update`: new
     `ValidateUniqueDomain(domainNames, excludeID)` (same-table, scoped to
     `RedirectionHost`), **plus**
     `CheckDomainConflict(db, domainNames, &models.ProxyHost{})`.
   - Two pre-existing `ProxyHost` rows with a partial comma-list overlap
     that was previously silently allowed remain silently allowed after
     this feature ships — `CheckDomainConflict` never runs `ProxyHost`
     against `ProxyHost`, only `ProxyHost` against `RedirectionHost` (and
     the reverse). No existing `ProxyHost`-vs-`ProxyHost` behavior changes.
2. **Self-redirect guard** (§1.3, §2.7) in `RedirectionHostService`:
   reject a create/update if `TargetURL`'s hostname exactly matches any of
   the host's own `DomainNames` entries (case-insensitive) — prevents a
   trivial infinite-redirect-to-self loop.

See §1.3 Non-Goals for the pre-existing `ProxyHost` same-table reordering
gap this deliberately leaves untouched.

### 4.4 API Contract

New handler `backend/internal/api/handlers/redirection_host_handler.go`,
new service `backend/internal/services/redirectionhost_service.go` —
following the exact `ProxyHostHandler`/`ProxyHostService` conventions
(structured errors as `gin.H{"error": "..."}`, server-generated UUIDs via
`github.com/google/uuid`, partial-update-via-`map[string]any` pattern on
`Update` for consistency with `ProxyHostHandler.Update`).

Routes (registered in `routes.go` alongside the existing
`proxyHostHandler.RegisterRoutes(...)` call near line 1017):

| Method | Path | Purpose |
|---|---|---|
| GET | `/api/v1/redirection-hosts` | List all redirection hosts |
| POST | `/api/v1/redirection-hosts` | Create a redirection host |
| GET | `/api/v1/redirection-hosts/:uuid` | Get one by UUID |
| PUT | `/api/v1/redirection-hosts/:uuid` | Partial update by UUID |
| DELETE | `/api/v1/redirection-hosts/:uuid` | Delete by UUID |

Request body (Create), matching model JSON tags:

```json
{
  "name": "Old blog redirect",
  "domain_names": "old-blog.example.com",
  "target_url": "https://newblog.example.com",
  "status_code": 301,
  "preserve_path": true,
  "ssl_forced": true,
  "http2_support": true,
  "hsts_enabled": false,
  "hsts_subdomains": false,
  "enabled": true,
  "certificate_id": null,
  "dns_provider_id": null,
  "use_dns_challenge": false
}
```

Response (201 Created / 200 OK): the full `RedirectionHost` JSON
representation (server-assigned `uuid`, `created_at`, `updated_at`
included; `id` never exposed, `json:"-"` per repo convention).

Validation errors → `400` with `gin.H{"error": "<message>"}`, e.g.:
- `"domain names is required"`
- `"target_url is required"`
- `"target_url must be a valid http(s) URL"` (scheme not in
  `{http, https}`, or `url.Parse` fails, or empty host)
- `"status_code must be one of 301, 302, 307, 308"`
- `"domain already in use by another host"` (from `CheckDomainConflict`,
  §4.3 — same message shape whether the conflict is with a `ProxyHost` or
  another `RedirectionHost`, since the resource type conflicting is an
  internal detail the user doesn't need to reverse-engineer their own
  config from)
- `"redirect target cannot point back to one of this host's own domains"`
  (self-redirect guard)

Not-found → `404` with `gin.H{"error": "redirection host not found"}`
(mirrors `ProxyHostHandler.Get`/`Update`/`Delete` exactly).

`Manager.ApplyConfig` is invoked after Create/Update/Delete exactly as
`ProxyHostHandler` does, with the same rollback-on-failure pattern for
Create (delete the row if `ApplyConfig` fails, mirroring
`proxy_host_handler.go:537-548`).

### 4.5 Frontend

New files, following the existing `ProxyHosts`/`ProxyHostForm` pattern but
intentionally much smaller given the reduced field set:

- `frontend/src/api/redirectionHosts.ts` — typed API client (mirrors
  `frontend/src/api/proxyHosts.ts`'s `ProxyHost` interface + CRUD
  functions wrapping `client.ts`), exporting a `RedirectionHost` interface
  matching §4.4's JSON shape and a `REDIRECT_STATUS_CODES` const array
  `[{value: 301, label: '301 - Permanent'}, {value: 302, label: '302 - Temporary'}, {value: 307, label: '307 - Temporary (preserve method)'}, {value: 308, label: '308 - Permanent (preserve method)'}]`
  for the status-code `<select>`.
- `frontend/src/hooks/useRedirectionHosts.ts` — React Query hook mirroring
  `useProxyHosts.ts` (list/create/update/delete mutations,
  `invalidateQueries(['redirection-hosts'])` on success).
- `frontend/src/components/RedirectionHostForm.tsx` — new, small form
  component: Name, Domain Names (reuse the same comma-separated
  multi-domain input UX as `ProxyHostForm`'s domain field — consider
  extracting that specific sub-input as a shared component,
  `DomainNamesInput.tsx`, if it's cleanly separable without destabilizing
  `ProxyHostForm`; otherwise duplicate the ~20-line input for now and flag
  the extraction as a follow-up rather than risk a `ProxyHostForm` regression
  mid-feature), Target URL, Status Code select, Preserve Path toggle, SSL
  Forced/HTTP2/HSTS toggles, Certificate select (reuse the same
  certificate-dropdown data-fetching pattern as `ProxyHostForm`'s SSL
  section), DNS Provider select (same reuse).
- `frontend/src/pages/RedirectionHosts.tsx` — new list/table page mirroring
  `ProxyHosts.tsx`'s table structure (Domain(s), Target, Status Code,
  Enabled toggle, Edit/Delete actions) but without any of the
  group/bulk-ACL/drag-and-drop machinery — v1 has no bulk operations or
  grouping for Redirection Hosts (Non-Goals).
- `frontend/src/components/Layout.tsx`: one new nav entry (§2.5) added
  directly under the existing Proxy Hosts entry.
- `frontend/src/App.tsx`: one new top-level `<Route>` (§2.5).
- i18n: new keys under `navigation.redirectionHosts`,
  `redirectionHosts.*` (title, form labels, table headers, empty state,
  delete confirmation) added to whatever locale file(s) back
  `useTranslation()` in this repo (same files `navigation.proxyHosts`
  already lives in).

## 5. Status Code Selection

Supported: **301, 302, 307, 308**.

| Code | Name | Method preserved? | Cacheable by default? | Included? |
|---|---|---|---|---|
| 301 | Moved Permanently | No (browsers commonly rewrite POST→GET) | Yes | Yes — the exact code the issue names, and NPM's default |
| 302 | Found | No | No | Yes — most common "temporary" redirect in the wild, matches user expectation from decades of HTTP usage |
| 307 | Temporary Redirect | Yes | No | Yes — needed when the redirected request must stay the same method (e.g. an API client POSTing to the old domain) |
| 308 | Permanent Redirect | Yes | Yes | Yes — the "308 as the method-preserving 301" for the same API-client scenario, permanent case |
| 300 | Multiple Choices | N/A | N/A | **No** — requires the server to offer multiple representations in the response body; no client software expects a bare redirect to send 300, and it isn't rendered as a redirect by browsers without extra body content Charon has no way to generate meaningfully |
| 303 | See Other | No | No | **No** — semantically "the result of your POST is over there, fetch it with GET"; it's a fine general HTTP status but not a "redirect this domain" primitive users reach for, and letting it stand in for 301/302 would blur the plain permanent/temporary distinction the issue is asking for. Can be reconsidered if a concrete use case surfaces |
| 304 | Not Modified | N/A | N/A | **No** — a conditional-GET/caching response code, not a redirect at all; it carries no `Location` header semantics for this purpose |

This set (301/302/307/308) is exactly the four codes NPM itself exposes for
Redirection Hosts, which both validates the selection against a tool this
audience already knows and keeps the API contract a closed `enum`-shaped
field (`ValidRedirectStatusCodes`, §4.1) rather than an open free-text
status code — per the issue's own ask ("301 and any other status code our
Charon should handle," read as "a short, sensible list," not "arbitrary
input").

## 6. Data Flow

```
Browser request for old-domain.example.com
        │
        ▼
Caddy (":443", TLS terminated using RedirectionHost.CertificateID
        or ACME-issued cert for old-domain.example.com)
        │
        ▼
Route match: Match.Host == ["old-domain.example.com"]
        │
        ▼
Handler chain: [optional HSTS header] → static_response
        │         (handler="static_response", status_code=301,
        │          headers.Location=["https://new.example.com{http.request.uri}"])
        ▼
Browser receives 301 + Location header, issues new request to
https://new.example.com/<original-path>?<original-query>
```

Config write path:

```
RedirectionHostForm (frontend) → POST/PUT /api/v1/redirection-hosts
        → redirection_host_handler.go → redirectionhost_service.go
          (ValidateUniqueDomain [cross-table via CheckDomainConflict],
           validateRedirectionHost [URL scheme/host, status code enum,
           self-redirect guard])
        → GORM save → caddyManager.ApplyConfig(ctx)
        → Manager.ApplyConfig fetches []ProxyHost AND []RedirectionHost
        → caddy.GenerateConfig(..., redirectHosts) builds combined
          processedDomains map + TLS policies + routes
        → POST to Caddy admin API (/config/apply, existing client.go path,
          unchanged)
```

## 7. Error Handling & Edge Cases

| Case | Handling |
|---|---|
| `target_url` missing scheme (`example.com` instead of `https://example.com`) | `400` — reject at service validation, do not silently prepend a scheme (avoids surprising the user about which scheme was chosen) |
| `target_url` scheme is neither `http` nor `https` (`ftp://`, `javascript:`, etc.) | `400` — scheme allowlist rejection |
| `domain_names` overlaps an existing `ProxyHost` or another `RedirectionHost` | `400` via `CheckDomainConflict` (§4.3) |
| `target_url` host equals one of `domain_names`' own entries | `400` — self-redirect guard (§4.3) |
| `status_code` not in `{301,302,307,308}` | `400` |
| Redirection host disabled (`enabled=false`) | Excluded from `BuildRedirectRoutes` entirely (same as `ProxyHost.Enabled` today) — domain falls through to Caddy's default 404 catch-all if nothing else claims it |
| Certificate deleted while still referenced by a `RedirectionHost` | Same FK-nullification behavior already implemented for `ProxyHost` (routes.go:176-188 "Cleaning up invalid Let's Encrypt certificate associations" sweep) — extend that sweep to also cover `redirection_hosts.certificate_id` |
| `ApplyConfig` fails after Create | Roll back (delete the just-created row), same pattern as `proxy_host_handler.go:537-548` |
| Two Redirection Hosts list the same domain (shouldn't happen given uniqueness check, but defense-in-depth) | `BuildRedirectRoutes`' newest-first iteration + shared `processedDomains` map keeps the newer one, logs a warning — same "Ghost Host" behavior as `ProxyHost` today |

## 8. Implementation Plan

### Phase 1 — Playwright E2E specs (behavior-first, `test.fixme`)

New `tests/redirection-hosts.spec.ts`:
- Create a redirection host (domain + target + 301), verify it appears in
  the list.
- Edit an existing redirection host's status code and target.
- Delete a redirection host.
- Validation: reject empty target URL, reject self-redirect, reject
  duplicate domain already used by a proxy host.
- Nav: "Redirection Hosts" link visible in sidebar, navigates to the new
  page.

All specs written as `test.fixme(...)` per CLAUDE.md's suggested commit
sequence, enabled once implementation lands (Phase 5).

### Phase 2 — Backend foundation (model + migration, no behavior change to existing resources)

- `backend/internal/models/redirection_host.go` (§4.1) + unit tests
  (`redirection_host_test.go`: `BeforeCreate` UUID assignment, JSON tag
  round-trip).
- `routes.go` AutoMigrate addition.
- `backend/internal/services/domain_uniqueness.go` (`CheckDomainConflict`,
  §4.3) + unit tests covering both directions of the cross-table check.
- `backend/internal/services/proxyhost_service.go`: **additive-only**
  edit — `Create`/`Update` gain one new call each,
  `CheckDomainConflict(s.db, host.DomainNames, &models.RedirectionHost{})`,
  inserted alongside (not replacing) the existing, byte-for-byte-unchanged
  `s.ValidateUniqueDomain(...)` call. No change to `ValidateUniqueDomain`
  itself. Existing `proxyhost_service_test.go` assertions must remain
  green with zero modifications; new tests only cover the new
  cross-table-rejection branch.

### Phase 3 — Backend API + Caddy generation

- `backend/internal/services/redirectionhost_service.go`: `Create`,
  `Update`, `Delete`, `GetByUUID`, `List`, `validateRedirectionHost`
  (URL scheme/host validation, status-code enum check, self-redirect
  guard), and a new `ValidateUniqueDomain` (same-table check scoped to
  `RedirectionHost`, mirroring `ProxyHostService`'s existing whole-string
  `domain_names = ?` query shape for consistency). `Create`/`Update` call
  both this same-table check **and**
  `CheckDomainConflict(s.db, host.DomainNames, &models.ProxyHost{})`
  (§4.3) — the two checks are independent and additive, neither replaces
  the other. Unit tests for every validation branch, including both
  uniqueness checks independently.
- `backend/internal/api/handlers/redirection_host_handler.go`: CRUD
  handlers per §4.4, `RegisterRoutes`, wired into `routes.go`. Unit tests
  per endpoint (success + each 400/404 case) using the existing
  `httptest`/gin test-harness conventions visible in
  `proxy_host_handler_test.go`.
- `backend/internal/caddy/types.go`: `RedirectHandler` builder + unit test.
- `backend/internal/caddy/redirect_routes.go`: `BuildRedirectRoutes` +
  `dedupeAndTrackDomains` shared helper (refactor: extract the
  domain-parse/lowercase/dedupe/`processedDomains`-check block currently
  inlined in `GenerateConfig` at config.go:532-556 into this reusable
  function so both the `ProxyHost` loop and `BuildRedirectRoutes` call the
  same logic instead of duplicating it — a DRY cleanup enabled by this
  feature, not just required by it). Unit tests: single domain, multi-domain,
  IP-only domain, disabled host skipped, preserve-path on/off, HSTS header
  present/absent.
- `GenerateConfig` signature change (functional-options conversion of its
  trailing variadic, §4.2) + wiring + `Manager.ApplyConfig` fetch addition.
  Per §4.2's exhaustive edit list, only `config.go`, `manager.go`,
  `config_customcert_test.go` (2 lines), `manager_ssl_provider_test.go`
  (1 helper), `manager_additional_test.go` (5 closures), and
  `manager_patch_coverage_test.go` (3 closures) need edits — the other
  test files with `GenerateConfig(` call sites need none. Add new tests
  for combined proxy+redirect domain conflict resolution and TLS policy
  inclusion of redirect-host domains.
- `./scripts/scan-gorm-security.sh --check` (new model + queries touch
  GORM — CLAUDE.md DoD step 1.5 trigger).

### Phase 4 — Frontend

- `frontend/src/api/redirectionHosts.ts`, `useRedirectionHosts.ts` +
  Vitest unit tests (mirror `proxyHosts.test.ts`/`useProxyHosts.test.tsx`
  structure).
- `frontend/src/components/RedirectionHostForm.tsx` + tests (mirror a
  trimmed-down `ProxyHostForm.test.tsx`: field rendering, validation
  messages, submit payload shape).
- `frontend/src/pages/RedirectionHosts.tsx` + tests (list rendering,
  create/edit/delete flows, empty state).
- `Layout.tsx` nav entry + `App.tsx` route + i18n keys.

### Phase 5 — Hardening, enable E2E, docs

- Un-`fixme` the Phase 1 Playwright specs; run
  `npx playwright test tests/redirection-hosts.spec.ts --project=firefox`.
- `scripts/go-test-coverage.sh` / `scripts/frontend-test-coverage.sh` ≥85%.
- `docs/features.md`: brief new entry linking to a new
  `docs/features/redirection-hosts.md`.
- `docs/features/redirection-hosts.md`: this is a **whole new feature**,
  not an added capability on an existing documented page — per user
  direction, it needs a full standalone page, not a stub. Mirror the
  depth and structure of `docs/features/orthrus.md`/`hecate.md` (the
  closest existing precedent in this repo): novice-friendly narrative
  covering, in substance if not literal headers, the 5W1H of the feature —
  **What** it is / **Why** it exists (what problem it solves vs. a normal
  proxy host), **Who** it's for and **When** to reach for it instead
  of a Proxy Host, **Where** it lives in the UI, and **How** to set one up
  (step-by-step, including picking a status code and what each one means
  in plain language). Written by `docs-writer` per the standard pipeline
  handoff, in commit 7 below.
- `ARCHITECTURE.md`: note the new peer resource + its Caddy-generation
  path, per CLAUDE.md's "update when adding... integration points."
- **Doc site**: no separate doc-site-specific work item is needed.
  Confirmed `docs-site/scripts/docs-manifest.json` already lists
  `features` as a whole-directory sync entry (not a per-file allowlist),
  and `docs-site/sidebars.ts` is fully `autogenerated` from the synced
  `docs/` tree (no hand-maintained sidebar array) — so a new, correctly
  front-mattered `docs/features/redirection-hosts.md` is picked up by
  `docs-site`'s next `sync-docs.mjs` run and appears in navigation with
  zero additional doc-site edits. Give the new page the same frontmatter
  shape as `orthrus.md`/`hecate.md` (`title`, `description`, `category:
  features`) so it renders and sorts consistently with its siblings.

## 9. Commit Slicing Strategy

**Decision**: single PR on the existing `feat/redirection-hosts-1367`
branch, ordered logical commits, per CLAUDE.md's "One Feature = One PR."

| # | Commit | Scope / Files | Depends on | Validation gate |
|---|---|---|---|---|
| 1 | `test: add fixme e2e specs for redirection hosts (#1367)` | `tests/redirection-hosts.spec.ts` (all `test.fixme`) | — | Spec file parses/lists under `npx playwright test --list`; no execution required yet |
| 2 | `feat: add RedirectionHost model and migration` | `backend/internal/models/redirection_host.go` (+ test), `routes.go` AutoMigrate line | 1 | `go build ./...`, `go test ./backend/internal/models/...`, `make lint-staticcheck-only` |
| 3 | `feat: add additive cross-table domain conflict check` | New: `backend/internal/services/domain_uniqueness.go` (`CheckDomainConflict`, + test). Modified, additive-only: `proxyhost_service.go` `Create`/`Update` each gain one new `CheckDomainConflict(...)` call alongside their existing, byte-for-byte-unchanged `ValidateUniqueDomain` call (§4.3) | 2 | `go test ./backend/internal/services/...` — **every pre-existing `ProxyHostService` test assertion passes unmodified** (this is the gate, not just "still green"); new tests added only for the new cross-table-rejection branch |
| 4 | `feat: add RedirectionHost service, API, and Caddy redirect generation` | `redirectionhost_service.go` (incl. its own same-table `ValidateUniqueDomain` + `CheckDomainConflict` call, §4.3), `redirection_host_handler.go`, `routes.go` route registration, `caddy/types.go` (`RedirectHandler`), `caddy/redirect_routes.go`, `caddy/config.go` (`GenerateConfig`'s trailing-variadic → functional-options conversion, §4.2), `manager.go` (fetch + pass-through), plus the exhaustive, non-speculative edit set from §4.2: `config_customcert_test.go` (2 lines), `manager_ssl_provider_test.go` (1 helper), `manager_additional_test.go` (5 closures), `manager_patch_coverage_test.go` (3 closures) | 2, 3 | `go build ./...`, `go test ./...` (including confirming the ~85+ untouched `GenerateConfig(` call sites in the other 6 test files still compile and pass with zero edits), `make lint-fast`, `./scripts/scan-gorm-security.sh --check` |
| 4.5a | `fix: extend certificate cleanup sweep to redirection hosts` (INSERTED) | `backend/internal/api/routes/routes.go`, `routes_test.go` | 4 | `go build ./...`, `go test ./...`, `make lint-staticcheck-only`, GORM scan |
| 4.5b | `fix: block certificate deletion when in use by a redirection host` (INSERTED) | `backend/internal/services/certificate_service.go` (+ new test file) | 4.5a | `go build ./...`, `go test ./...`, `make lint-staticcheck-only`, GORM scan |
| 5 | `feat: add Redirection Hosts UI (list, form, nav)` | `frontend/src/api/redirectionHosts.ts`, `useRedirectionHosts.ts`, `RedirectionHostForm.tsx`, `RedirectionHosts.tsx`, `Layout.tsx`, `App.tsx`, i18n keys + all `.test.tsx`/`.test.ts` | 4.5b | `npm run type-check`, `npm run build`, Vitest suite green |
| 5.5 | `fix: persist explicit false values for redirection host boolean flags` (INSERTED) | `backend/internal/models/redirection_host.go`, `redirection_host_handler.go` (+ test) | 5 | `go build ./...`, `go test ./...`, GORM scan |
| 6 | `test: enable redirection host e2e specs` | Un-`fixme` commit 1's spec file | 5.5 | `npx playwright test tests/redirection-hosts.spec.ts --project=firefox` passes |
| 7 | `docs: document redirection hosts feature` | `docs/features.md`, `docs/features/redirection-hosts.md`, `ARCHITECTURE.md` | 6 | Doc-writer review; no code impact |
| 7.5 | `fix: persist explicit false value for redirection host enabled flag` (INSERTED) | `backend/internal/models/redirection_host.go`, `redirection_host_handler.go` (+ tests) | 7 | `go build ./...`, `go test ./...`, `make lint-staticcheck-only`, GORM scan |

**Commits 4.5a/4.5b (discovered during commit 4, not in the original plan)**:
giving `RedirectionHost` its own `CertificateID` FK (mirroring `ProxyHost`,
per §4.1) meant the two existing certificate-lifecycle safeguards for that
FK — the startup "null out dangling Let's Encrypt cert assignments" sweep,
and `IsCertificateInUse`'s delete-time block — only covered `ProxyHost`.
Left unfixed, deleting a certificate still referenced by a
`RedirectionHost` would neither be blocked nor cleaned up until the next
restart, risking a live TLS/Caddy-config break for that redirect. Both
fixes reuse the existing mechanisms exactly (same sweep query pattern,
same `ErrCertInUse` block), guarded by `db.Migrator().HasTable(&models.RedirectionHost{})`
— the same guard pattern `CheckDomainConflict` (commit 3) already
established — so every pre-existing `ProxyHost`-only test DB is
unaffected. Zero pre-existing test files were modified for either fix
(new test files only), confirmed via `git diff --stat`.

**Commit 5.5 (discovered during commit 6's E2E enablement, not in the
original plan)**: `RedirectionHost.PreservePath`/`SSLForced`/`HTTP2Support`
were plain `bool` fields tagged `gorm:"default:true"` — the classic GORM
bool-zero-value collision, where GORM cannot distinguish "explicitly set to
`false`" from "left at its Go zero value" and silently substitutes the
column default on `INSERT`. `POST /redirection-hosts` with any of these
three set to `false` silently persisted `true` instead — confirmed via a
direct API call, not just a test. Fixed by removing the colliding
`gorm:"default:true"` tags and defaulting the three fields to `true` at
the handler level (in `RedirectionHostHandler.Create`, only when the
payload key is missing or JSON `null`), mirroring the existing
resolve-before-struct-round-trip pattern already used for
`certificate_id`/`dns_provider_id` in the same handler. Chosen over a
`*bool` conversion because it required touching exactly one function in
one file, with zero ripple into the service layer, Caddy generation, or
the frontend. **Follow-up flagged, not fixed (out of scope for this PR)**:
`ProxyHost.HTTP2Support` and `ProxyHost.BlockExploits` have the identical
latent bug — worth a separate issue if creating a `ProxyHost` with either
explicitly `false` turns out to matter in practice.

**Commit 7.5 (discovered during the final QA/security audit, not in the
original plan)**: the independent QA pass found the `Enabled` field had
the identical GORM bool-zero-value/`default:true` collision that 5.5 fixed
for `PreservePath`/`SSLForced`/`HTTP2Support` — `Enabled` was missed from
that fix. Not reachable via the current UI (the create form never sends
`enabled`), but part of the documented API contract, so an API consumer
creating a pre-disabled host would have it silently persist as enabled.
Fixed with the exact same precedented pattern: removed the colliding
`gorm:"default:true"` tag, added `enabled` to the handler's
`booleanFieldsDefaultingTrueOnCreate` list. Red→green regression test
confirmed.

**Follow-up flagged, not fixed (out of scope for this PR, tracked in
`docs/reports/qa_report.md`)**: QA also found `CertificateService`'s
`refreshCacheFromDB` and `GetCertificate` (the certificate list's "in use"
badge and the cert detail page's assigned-hosts list) still query
`ProxyHost` only, not `RedirectionHost` — a UI-consistency gap, not a
security issue (the actual delete-blocking check, `IsCertificateInUse`,
already correctly covers both tables per commit 4.5b). Recommended as a
tracked fast-follow rather than expanding this PR further.

Each commit builds and passes its own gate before the next starts, per
CLAUDE.md's "Per-Commit Requirement." The PR as a whole must clear the
full Definition of Done (coverage ≥85% both sides, full lint, security
scans, `go build`/`npm run build`) before merge.

### Rollback / contingency (whole-PR level)

- Because Redirection Hosts are a fully separate table and code path, the
  entire feature is revertible with a single `git revert` of the merge
  commit without touching `ProxyHost` rows or logic — no data migration
  of existing hosts occurs, so there's no destructive-rollback risk.
- `GenerateConfig`'s signature change is de-risked by design, not by
  contingency: §4.2's functional-options conversion of its existing
  trailing variadic is the primary plan for commit 4, verified against
  every actual call site (92 across 9 files) rather than assumed
  low-risk. If `backend-dev` discovers a call site during implementation
  that this research missed (e.g. a generated/vendored caller, or a test
  helper not caught by the `GenerateConfig(`/`generateConfigFunc` greps
  used here), the same functional-options pattern still applies — add the
  missing file to §4.2's edit table and update it the same mechanical way,
  rather than falling back to a different signature strategy mid-commit.
- If cross-table `CheckDomainConflict` (commit 3) surfaces pre-existing
  duplicate domains in a real deployment's data (unlikely but possible if
  a user already has overlapping configs some other way), the check must
  fail closed on new writes but must not retroactively break existing rows
  — no destructive migration is included in this plan.

## 10. Acceptance Criteria

- [ ] A user can create a Redirection Host with a domain, target URL, and
      one of 301/302/307/308, from a dedicated "Redirection Hosts" nav
      item, with zero JSON editing required.
- [ ] Visiting the configured domain over HTTPS returns the chosen status
      code with a `Location` header pointing at the target (verified via
      Playwright + a direct Caddy config JSON assertion in a Go test).
- [ ] "Preserve path & query string" toggle correctly appends/omits
      `{http.request.uri}`.
- [ ] Creating a Redirection Host for a domain already used by a
      `ProxyHost` (or another `RedirectionHost`) is rejected with a clear
      `400` error.
- [ ] Creating a Redirection Host whose target points back to its own
      domain is rejected.
- [ ] Deleting/disabling a Redirection Host removes/excludes its Caddy
      route on the next `ApplyConfig`.
- [ ] No changes to `ProxyHost`'s schema, `ProxyHostService.ValidateUniqueDomain`
      (unchanged, byte-for-byte, same whole-string query and same
      pre-existing reordering gap — see §1.3, §4.3), or handler behavior.
      `go test ./...` is green with **zero modifications** to any
      pre-existing `ProxyHost`-only test assertion. The one intentional,
      additive exception: `ProxyHostService.Create`/`Update` gain a new
      `CheckDomainConflict` call (§4.3) that can newly reject a `ProxyHost`
      write when its domain is already claimed by a `RedirectionHost` —
      this is new cross-table behavior this feature is required to add,
      not a change to any existing `ProxyHost`-vs-`ProxyHost` outcome.
      `GenerateConfig` call sites also need the mechanical signature
      updates enumerated in §4.2 (test files only; behavior unchanged).
- [ ] Backend + frontend coverage ≥85%, all Definition-of-Done steps in
      CLAUDE.md pass, zero high/critical CodeQL/Trivy/GORM-scan findings.
