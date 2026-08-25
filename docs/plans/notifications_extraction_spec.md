# Notifications Engine Extraction — Scoping Spec

Status: Scoping/design only. No extraction, no new repo, no code changes performed under this
spec. This document is the literal move-list and design brief for a **future session** that will
create the new repository and perform the file-move/refactor/import work.

Owner for this document: **planning** agent.
Owner for execution (future session): TBD — likely a fresh `management`-orchestrated pipeline once
the new repo exists, since it touches both a new external repo and Charon itself.

**Revision note (rev 2):** this draft originally recommended a Phase-1-only extraction (the
SSRF-safe HTTP wrapper alone) and left the provider-payload/template logic and email dispatch for a
deferred v0.2. The user has since decided on **full provider-layer scope**: the
Discord/Slack/Gotify/Pushover/Ntfy/webhook payload builders and email dispatch are genericized and
moved into the new module in this same extraction. §3.1, §3.3, §3.5, §3.6, §4, §5, §6, and §7 are
revised accordingly. §2 (research findings) and the HTTP wrapper DI seams in §3.2 are unchanged from
the prior draft.

**Two follow-up inputs folded into this same revision:**

1. **The new repo already exists.** The user has created it at `/projects/go_notify_yourself`
   (sibling to `/projects/Charon`, remote `github.com/Wikid82/go_notify_yourself`), currently just
   `LICENSE` + a placeholder `README.md` — no `go.mod` yet. Every placeholder module path in this
   spec (`github.com/Wikid82/notifyhttp` in the original draft, briefly `github.com/Wikid82/notify`
   earlier in this revision pass) is now replaced with the real path,
   **`github.com/Wikid82/go_notify_yourself`**. The extraction session's Phase 1 (§4) scaffolds
   *into* this existing directory/repo, not a newly-`git init`'d one. The root Go package name is
   kept as `notify` (not `go_notify_yourself`) since Go package names conventionally avoid
   underscores — this is a normal, unproblematic mismatch (import path ≠ package identifier; the
   compiler resolves the identifier from each file's `package notify` declaration, so consuming code
   still just writes `notify.Message` after `import "github.com/Wikid82/go_notify_yourself"`).
2. **Long-term direction vs. near-term scope.** The user's eventual goal is for this module to
   become a Go equivalent of [Apprise](https://github.com/caronc/apprise) — the Python library that
   unifies notification dispatch across a large number of services via a common interface/URL-scheme
   convention. **Right now, while both Charon and the user's other small family project are still
   under active development**, they explicitly do not want this extraction to add any provider
   Charon doesn't already have (no Twilio/PagerDuty/Matrix/etc.). The move-list in §3.1 stays exactly
   Charon's existing seven HTTP providers + email — no more (six at the time this note was first
   drafted; Telegram was folded in as the seventh per §7 risk 1e/§3.6 step 6, since Charon already
   supports it). The API design in §3.3.3, however, is shaped so that adding providers later is
   additive, not a breaking change — see the new §8 for the explicit tradeoff and what was (and
   wasn't) built now to support that.

---

## 1. Introduction

### 1.1 Objective

Charon's owner now maintains multiple projects and wants a **standalone, reusable Go module** for
notification delivery (SSRF-safe outbound HTTP dispatch, retries, provider payload templating) so
future projects — and Charon itself — can `go get` it instead of re-implementing notification
delivery from scratch each time.

### 1.2 Goals

- Produce an exact inventory of what moves to the new module vs. what stays in Charon, with a
  one-line reason for each file.
- Identify every point where the current engine reaches into Charon-internal code, and define a
  dependency-injection seam that removes that coupling.
- Define the new module's public Go API, generic enough for an unrelated project to adopt.
- Define the new repo's structure, versioning/release strategy, and CI shape.
- Define the Charon-side migration plan for consuming the new module once it exists.
- Surface open questions/risks that the extraction session must resolve or confirm with the user
  before touching code.

### 1.3 Non-goals

- No new repository is created here.
- No files are moved, no import paths are rewritten, no code is written under this spec.
- No decision is made on the new repo's exact GitHub org/visibility — that's the user's call when
  they set up the workspace.

---

## 2. Research Findings

### 2.1 Existing architecture summary

Charon's notification surface spans four layers, and they are **not** equally coupled:

| Layer | Location | Coupling to Charon |
|---|---|---|
| Delivery primitive | `backend/internal/notifications/` | Imports `internal/network` + `internal/security` (SSRF guards) directly. Otherwise pure Go, no GORM, no DB, no Charon config. |
| Orchestration/business logic | `backend/internal/services/notification_service.go`, `security_notification_service.go`, `enhanced_security_notification_service.go` | Heavily coupled: `*gorm.DB`, `models.NotificationProvider`/`NotificationConfig`, Charon's `Setting` table for feature flags, Charon's `logger`/`util`/`trace` packages, `MailServiceInterface` (Charon SMTP), Charon-branded strings ("[Charon Alert]"), Charon domain concepts (`HostName`, `ServiceCount`, `proxy_host`/`remote_server`/`domain`/`cert`/`uptime`/`security_*` event types). |
| Persistence | `backend/internal/models/notification*.go` | GORM models with `BeforeCreate` hooks, `gorm:` tags — pure Charon persistence. |
| Presentation | `frontend/src/{api,pages,hooks,components}/*notification*` | React/TanStack Query UI wired to Charon's REST API and design system. |

This four-layer split is the central finding of this spec: **the reusable "engine" the user asked
for is materially smaller than the full notification feature.** Section 3.1 lays out the exact
line, now revised for full provider-layer scope (see the revision note above).

### 2.2 `backend/internal/notifications/` package (the current "engine")

| File | LOC | Purpose |
|---|---|---|
| `engine.go` | 23 | `DeliveryEngine` interface + `DispatchRequest` struct. **Dead code** — grep confirms nothing outside this file implements or references `DeliveryEngine`; `EngineNotifyV1` const is unused elsewhere. |
| `feature_flags.go` | 15 | String constants naming Charon `Setting`-table keys (e.g. `feature.notifications.service.discord.enabled`). These are Charon policy labels, not engine behavior — the lookup logic lives in `notification_service.go` (`getFeatureFlagValue`), not in this package. |
| `http_client_executor.go` | 8 | Thin `client.Do` wrapper, exists purely as a test seam. |
| `router.go` | 38 | `Router.ShouldUseNotify()`. Comment in the file itself says `// NOTE: used only in tests`. Grep confirms: zero production call sites outside `router_test.go`. **Dead code.** |
| `http_wrapper.go` | 541 | The real engine: `HTTPWrapper.Send()` — SSRF-hardened outbound POST with retry/backoff (`RetryPolicy`), redirect guarding, response size caps (256 KiB request / 1 MiB response), header allowlisting, provider error-hint extraction, transport error sanitization. This is genuinely reusable and provider-agnostic. |
| `http_wrapper_test.go`, `router_test.go` | — | Unit tests, ~31 KB combined coverage of the above. |

**Coupling point**: `http_wrapper.go` imports `internal/network` (for `network.NewSafeHTTPClient`,
`network.Option`, `network.IsPrivateIP`) and `internal/security` (for `security.ValidateExternalURL`,
`security.ValidationOption`). Confirmed via `grep -rl` that both packages are **shared Charon
infrastructure** used well beyond notifications — Caddy client, CrowdSec integration, uptime
monitoring, auth, config, remote-storage SSRF guards. They must **not** move into the new module;
they need a DI seam instead (see §3.2).

Both `internal/network` and `internal/security` were read in full: neither imports GORM, Charon
models, or Charon config beyond `os.Getenv` for two env var overrides
(`CHARON_NOTIFY_ALLOW_HTTP`, `CHARON_NOTIFY_MAX_REDIRECTS`, both read inside
`internal/notifications/http_wrapper.go` itself, not in `network`/`security`). This means the seam
is narrow: two small interfaces, not a deep dependency tree.

### 2.3 `backend/internal/services/notification_service.go` (35.7 KB — the real feature logic)

Confirmed via read of `SendExternal`, `sendJSONPayload`, `dispatchEmail`,
`emailTemplateForEventType`, `RenderTemplate`, and the CRUD methods:

- `sendJSONPayload` builds provider JSON payloads from Go `text/template`, with two built-in
  templates (`minimal`, `detailed`) referencing Charon-specific fields: `HostName`, `HostIP`,
  `ServiceCount`, `Services`. Operates directly on `models.NotificationProvider` (GORM struct), not
  a generic config type.
- `dispatchEmail` hardcodes the subject prefix `"[Charon Alert] %s"` and delegates to
  `MailServiceInterface` (Charon's SMTP service) for template rendering (`email_security_alert.html`,
  `email_ssl_event.html`, etc.) — those HTML templates live in Charon's mail service, not in
  `internal/notifications`.
- `SendExternal` filters providers by Charon domain event types (`proxy_host`, `remote_server`,
  `domain`, `cert`, `uptime`, `security_waf`, `security_acl`, `security_rate_limit`,
  `security_crowdsec`, `test`) matched against per-provider boolean columns on the GORM model.
- Feature-flag gating (`isDispatchEnabled` → `getFeatureFlagValue`) reads `models.Setting` rows via
  `s.DB` directly — this is Charon's own settings/feature-flag system, not something the new module
  should own.
- `httpWrapper *notifications.HTTPWrapper` is the **only** call from this file into the
  `internal/notifications` package for actual delivery — confirms `HTTPWrapper.Send` is the true
  reusable primitive and everything else in this file is Charon-specific orchestration built on top
  of it.

**Conclusion**: this file is not "the engine with some Charon glue" — it's Charon's *product
feature* built on top of a much smaller generic engine. Fully genericizing it (replacing
`models.NotificationProvider` with a generic config struct, replacing `HostName`/`ServiceCount`
with a generic `Data map[string]any`, extracting the Discord/Slack/Gotify/Pushover/Ntfy/webhook
payload-building into provider packages, defining a `Mailer` interface for email) is real design
and implementation work, not a mechanical move. **This is now in scope for the same extraction —
see the revised §3.1 for the function-level split.**

### 2.4 `security_notification_service.go` / `enhanced_security_notification_service.go`

Both operate on `models.SecurityEvent` / `models.NotificationConfig` (GORM), and encode
Charon-specific security taxonomy (WAF blocks, ACL denies, rate-limit hits, CrowdSec decisions —
i.e. Charon's own proxy/WAF feature surface). `enhanced_security_notification_service.go` additionally
implements a legacy-config migration path (`MigrateFromLegacyConfig`, `computeConfigChecksum`)
that is pure Charon schema-evolution logic. Neither belongs in a generic notifications module —
they are consumers of it, not part of it.

### 2.5 `backend/internal/models/notification*.go`

All four files (`notification.go`, `notification_config.go`, `notification_provider.go`,
`notification_template.go`) are GORM models with `gorm:` struct tags and `BeforeCreate` UUID
hooks. Pure persistence — stay in Charon by definition. A generic module must not depend on GORM
at all (a future adopter may use Postgres, a different ORM, or no DB).

### 2.6 `backend/integration/notification_http_wrapper_integration_test.go`

Build-tagged `integration` test exercising `notifications.NewNotifyHTTPWrapper()` directly against
an `httptest.Server` (retry-on-429, no-retry-on-400, tokenized-query rejection). This test only
exercises the `HTTPWrapper` — it has zero dependency on Charon models/DB. It is a strong candidate
to move with the engine (it's effectively already an engine-level integration test), with a thin
Charon-side replacement or deletion once the import path changes.

### 2.7 `docs/features/notifications.md`

Documents the product feature end-to-end (provider setup, JSON template variables including
Charon-specific ones like `{{.HostName}}`, migration guide, "Charon Test" wording, links to
`github.com/Wikid82/charon`). This is Charon user documentation, not module documentation — stays
in Charon. The new module will need its own README/docs describing its generic API, written fresh
during the extraction session (not migrated from this file).

### 2.8 Frontend

Confirmed by line count and a read of the API client shape: `frontend/src/api/notifications.ts`
(271 LOC), `pages/Notifications.tsx` (758 LOC), `hooks/useNotifications.ts` (53 LOC),
`components/NotificationCenter.tsx` (157 LOC), plus their tests. All talk to Charon's REST API
(`/api/notifications`, `/api/notification-providers`, etc.) and render Charon's design system. This
is UI for Charon's product feature, not the engine. **Confirms the assumption in the task context
explicitly**: none of this moves. A Go module has no frontend; a *different* consuming project
would build its own UI (or none) against its own backend, not reuse Charon's React components.

### 2.9 External dependencies / prior art

- The org already publishes and releases via GoReleaser (`.goreleaser.yaml` at repo root, driven by
  `.github/workflows/release-goreleaser.yml`) and has an `auto-versioning` workflow tied to
  Conventional Commits. The new module should reuse this exact pattern rather than invent a new one
  — same maintainer, same tooling, lower cognitive overhead.
- No `.codecov.yml` exists at the repo root currently (checked); Charon's coverage gate is enforced
  via `scripts/go-test-coverage.sh` instead. The new module should carry its own equivalent
  lightweight script rather than depend on Charon's.
- Go module path convention: Charon's backend module is
  `github.com/Wikid82/charon/backend` (go 1.26.6). The new module should follow the same GitHub org
  (`Wikid82`) unless the user decides otherwise when creating the repo.

---

## 3. Technical Specifications

### 3.1 Exact inventory — move list vs. stay list

**Decision (resolved by the user): full provider-layer scope, in this same extraction.** The
Discord/Slack/Gotify/Pushover/Ntfy/Telegram/webhook payload builders and email dispatch are
genericized and moved into the new module now — not deferred to a v0.2. This supersedes the Phase-1-only
recommendation this section previously carried; risk #1 in §7 (previously "scope question, flag to
user") is now resolved and reframed as a behavior-parity risk instead.

This re-read `notification_service.go` (1029 lines) in full, plus `mail_service.go` and the
`templates/*.html` files, to pin exact function boundaries rather than guessing. The seam (per the
revision brief) is: Charon's service layer maps its GORM `NotificationProvider` row + Charon event
data into the new module's generic `Message` type and a provider-specific `Config`, calls the
module's `Sender`/email `Mailer` to dispatch, and logs/persists the result. The module itself ends
up with **zero** imports of GORM, `models`, or any `github.com/Wikid82/charon/*` package.

#### 3.1.1 Moves to the new module — delivery primitive (unchanged from prior draft)

| File | Reason |
|---|---|
| `http_wrapper.go` | SSRF-hardened dispatch, retries, header sanitization. Zero Charon-domain knowledge — only needs the two DI seams in §3.2. |
| `http_wrapper_test.go` | Moves with its subject. |
| `http_client_executor.go` | Test seam used by `http_wrapper.go`. |
| `backend/integration/notification_http_wrapper_integration_test.go` | Exercises only `HTTPWrapper` (§2.6). |

#### 3.1.2 Moves to the new module — provider payload/dispatch logic, function-level (NEW scope)

All of the following are read directly out of `notification_service.go`'s `sendJSONPayload` (lines
383–677), `RenderTemplate` (788–832), and `dispatchEmail`/`sanitizeForEmail` (286–358), and mapped to
their destination package. None of these are simple moves — each is genericized per §3.3.

| Current location (`notification_service.go`) | Logic | Destination |
|---|---|---|
| `minimalTemplate`/`detailedTemplate` consts (385–386, duplicated 792–793) | Built-in JSON templates | `providers/webhook`, genericized: `.HostName`/`.HostIP`/`.ServiceCount`/`.Services` top-level fields replaced with a single `{{toJSON .Data}}` (see "payload shape change" risk in §7). |
| Template parse/exec core (388–447, 811–825) — `text/template` + `toJSON` funcmap, 10 KB size cap, 5 s exec timeout | Shared rendering engine | `providers/internal/render` (unexported, shared by all seven provider packages — avoids 7x duplication of the same template plumbing). |
| `discordWebhookRegex`, `allowedDiscordWebhookHosts`, `normalizeURL`, `validateDiscordWebhookURL`, `validateDiscordProviderURL` (63–127) | Discord webhook URL shape/host validation | `providers/discord` |
| Discord payload normalization (`content`/`embeds` fallback, 458–475) | Discord-specific JSON shape | `providers/discord` |
| `slackWebhookRegex`, `validateSlackWebhookURL` (70–77) | Slack webhook URL shape validation | `providers/slack` |
| Slack payload normalization (`text`/`blocks` fallback, 476–493) + webhook-token substitution (577–586) | Slack-specific JSON shape + dispatch URL resolution | `providers/slack` |
| Gotify `message`-field validation (494–498) + `X-Gotify-Key` header (544–548) | Gotify-specific JSON shape + auth header | `providers/gotify` |
| Pushover `message`-field/priority validation (516–524) + URL build, token/user injection, hostname pin (594–627) | Pushover-specific JSON shape + dispatch URL/auth | `providers/pushover` |
| Ntfy `message`-field validation (525–528) + `Authorization: Bearer` header (588–592) | Ntfy-specific JSON shape + auth header | `providers/ntfy` |
| Telegram `text`-field validation with `message`-field fallback (499–515) + dispatch URL build from `telegramAPIBaseURL + "/bot" + token + "/sendMessage"` with hostname-pin check, `chat_id` injection from `p.URL` (550–575) | Telegram-specific JSON shape + dispatch URL/auth (bot token embedded in URL path, not a header; `p.URL` repurposed as chat ID) | `providers/telegram` |
| Generic/custom webhook dispatch (the plain `webhook`/`generic` case) | Passthrough JSON dispatch, no provider-specific shape | `providers/webhook` |
| `isValidRedirectURL` (685–697) | Generic URL sanity check used before Discord dispatch | Moves with Discord validation into `providers/discord` (only call site). |
| `webhookDoRequestFunc` test hook (375–377) | Test seam for the raw-dispatch path | **Dropped**, not ported — redundant with `http_client_executor.go`'s seam once Discord/webhook dispatch is consolidated onto the shared `transport.Wrapper` (see flagged inconsistency below). One test seam per module, not two. |
| `RenderTemplate` (788–832) | Template preview/validation for the provider-editor UI | Logic moves to `providers/webhook.RenderPreview(tmplStr string, msg notify.Message) (json string, parsed any, err error)` — a public function, reusable for previewing any of the seven provider types since they all share the same template mechanism today. Charon's `CreateProvider`/`UpdateProvider` keep a thin wrapper extracting `.Config`/`.Template` from the GORM row and calling it. |
| `sanitizeForEmail` (286–299) | Control-char stripping for email hygiene | `providers/email` — generic, zero Charon dependency already. |
| `dispatchEmail`'s message composition (safeTitle/safeMessage, subject formatting, `EmailTemplateData` construction; 322–350) | Email message assembly | `providers/email`, genericized: subject becomes `Config.SubjectPrefix + msg.Title` (prefix `""` by default, no `"[Charon Alert]"` baked in — see §3.3.4). |

**Flagged inconsistency found on this re-read, resolved as part of the extraction:** the plain
`webhook`/`generic` dispatch path (lines 639–676) does **not** go through `httpWrapper.Send` today —
unlike gotify/webhook-JSON/telegram/slack/pushover/ntfy (line 531's list, which *does* include
`"webhook"` for the JSON-template path), this fallback branch calls `security.ValidateExternalURL` +
`network.NewSafeHTTPClient` directly, bypassing the shared engine's retry/backoff entirely. This
fallback branch is in practice the **Discord** path plus the literal `"generic"` provider type,
since every other supported type is caught by the line-531 list first. Recommend consolidating
`providers/discord` and `providers/webhook`'s dispatch onto the shared `transport.Wrapper` (via the
Seam 1/2 DI in §3.2) for consistency. This is a genuine behavior change (retry/backoff semantics,
not just a refactor) — called out as a new risk in §7, not silently folded in.

#### 3.1.3 Stays in Charon (per the revision brief) — GORM CRUD, flag gating, event routing

| Function(s) | Reason |
|---|---|
| `SendExternal` (215–284) | Event-type filtering against Charon domain concepts (`proxy_host`/`remote_server`/`domain`/`cert`/`uptime`/`security_*`) mapped to `models.NotificationProvider` boolean columns, plus the GORM `Find` query. Becomes the seam: after filtering + flag-check, maps provider row + event data into `notify.Message` + provider `Config`, calls the module's `Sender`, logs the result. |
| `isDispatchEnabled`, `getFeatureFlagValue` (148–180) | DB-backed feature-flag gating via `models.Setting` — explicitly named as staying in the revision brief. |
| `emailTemplateForEventType` (360–371) | Charon event-type → HTML template name mapping. Stays; becomes the `TemplateName` selector Charon passes into its `providers/email` adapter (§3.3.4). |
| `Create`/`List`/`MarkAsRead`/`MarkAllAsRead` (184–211) | Pure GORM CRUD for the in-app `Notification` bell/log — never part of the engine. |
| `ListProviders`/`CreateProvider`/`UpdateProvider`/`DeleteProvider` (836–946) | Provider GORM CRUD + Charon's own field-level validation (type immutability, token retention rules). Calls the module's new `providers/webhook.RenderPreview` for custom-template validation (thin wrapper, per §3.1.2). |
| `ListTemplates`/`GetTemplate`/`CreateTemplate`/`UpdateTemplate`/`DeleteTemplate` (756–786) | GORM CRUD for `NotificationTemplate` rows. |
| `isSupportedNotificationProviderType`, `supportsJSONTemplates` (129–146) | Charon's own provider-type allowlist gating its REST API/UI input — mirrors the module's provider set but is Charon's validation boundary, not module logic. **Note**: both include `"telegram"`, which is now in scope as the module's seventh provider (§7 risk 1e, resolved). |
| `EnsureNotifyOnlyProviderMigration` (948–1029) | Pure Charon schema-evolution/migration logic (Discord-only rollout reconciliation). Unrelated to the engine. |
| `TestProvider`/`TestEmailProvider` (699–753) | Backing logic for Charon's REST "send test notification" handlers. Become thin adapters: build a `notify.Message`, call the module's `Sender`/`email` client, same seam as `SendExternal`. |

**Deleted, not ported (dead code confirmed on this re-read):**
- `isPrivateIP(ip net.IP) bool` (679–683) in `notification_service.go` — a wrapper around
  `network.IsPrivateIP` with **zero call sites within the file itself** (confirmed by reading the
  full 1029 lines; the identically-named functions in `access_list_service.go` and the
  `hecate/providers/{netbird,zerotier}` clients are unrelated, independently-defined helpers). Drop
  entirely rather than carry forward.

#### 3.1.4 Stays in Charon — unchanged (persistence, security infra, unrelated features)

| File / area | Reason |
|---|---|
| `backend/internal/models/notification.go`, `notification_config.go`, `notification_provider.go`, `notification_template.go` | GORM persistence models; a generic module must not depend on GORM. |
| `backend/internal/services/security_notification_service.go`, `enhanced_security_notification_service.go` + their tests | Charon-specific security event taxonomy (WAF/ACL/rate-limit/CrowdSec) and legacy-config migration logic. Consumers of the engine, not part of it. |
| `docs/features/notifications.md` | Charon user-facing product documentation; the new module gets its own fresh README. |
| `frontend/src/api/notifications.ts`, `pages/Notifications.tsx`, `hooks/useNotifications.ts`, `components/NotificationCenter.tsx` + tests | Charon product UI, confirmed to have no reusable-module role (§2.8). |
| `backend/internal/network/*`, `backend/internal/security/*` | Shared Charon infrastructure used by Caddy, CrowdSec, uptime monitoring, auth, config — far beyond notifications (confirmed by repo-wide grep in §2.2). Stay in Charon; the new module receives DI seams instead (§3.2). |
| `mail_service.go`'s SMTP transport (`SendEmail`, `GetSMTPConfig`, connection handling) and the five HTML templates (`templates/*.html`) | SMTP credentials/connection lifecycle and Charon's branded, event-differentiated email design (`email_base.html` says "Charon" / "Charon Reverse Proxy Manager"). Charon supplies these behind the module's `Mailer`/`TemplateRenderer` interfaces (§3.3.4) — they do not move. |

### 3.2 Coupling points to decouple

The delivery-primitive coupling point is unchanged from the prior draft: `http_wrapper.go`'s use of
`internal/network` and `internal/security` for SSRF-safe HTTP client construction and destination
URL validation. With full provider-layer scope, this same seam is now consumed by **every** HTTP-based
provider package (`discord`, `slack`, `gotify`, `pushover`, `ntfy`, `telegram`, `webhook`) via the shared
`transport.Wrapper` (§3.3.1/§3.5), not just by a single Charon call site — which is exactly what
resolves the discord-dispatch inconsistency flagged in §3.1.2. The email package (§3.3.4) has no
equivalent coupling: it never touches `internal/network`/`internal/security` at all, since SMTP
transport stays entirely behind the host-supplied `Mailer` interface.

#### Seam 1 — safe HTTP client factory

```go
// new module: package transport (see §3.5 for the module layout)

// ClientFactory builds the *http.Client used for outbound provider requests.
// The host application is responsible for SSRF hardening (private-IP blocking,
// DNS-rebinding protection, redirect limits) inside its implementation.
type ClientFactory func(allowHTTP bool, maxRedirects int) *http.Client
```

Charon supplies, at the call site where it constructs the wrapper:

```go
// Charon side, e.g. in services/notification_service.go or a small adapter file
factory := func(allowHTTP bool, maxRedirects int) *http.Client {
    opts := []network.Option{network.WithTimeout(10 * time.Second), network.WithMaxRedirects(maxRedirects)}
    if allowHTTP {
        opts = append(opts, network.WithAllowLocalhost())
    }
    return network.NewSafeHTTPClient(opts...)
}
wrapper := transport.NewWrapper(transport.WithClientFactory(factory), ...)
```

#### Seam 2 — destination URL validator

```go
// new module

// URLValidator validates and normalizes a destination URL before dispatch,
// returning the (possibly normalized) URL or an error if the destination is
// disallowed. Implementations are expected to enforce the host application's
// SSRF policy (private-IP blocking, scheme allowlisting, etc.).
type URLValidator func(rawURL string, allowHTTP bool) (string, error)
```

Charon supplies an adapter that calls `security.ValidateExternalURL` with the equivalent
`WithAllowHTTP()`/`WithAllowLocalhost()` options translated from the bool flag.

#### Seam 3 — private-IP / destination guard (used by `guardDestination`/`isAllowedDestinationIP`)

The current code calls `network.IsPrivateIP(ip)` directly inside `HTTPWrapper.guardDestination`.
Fold this into the same `URLValidator` contract by making validator responsible for **all**
destination-safety decisions (scheme, host, IP-literal, DNS-resolved IPs) rather than splitting
SSRF logic between the module and the callback. This keeps the new module's dependency-injection
surface to exactly two functional options (`WithClientFactory`, `WithURLValidator`) instead of
three overlapping ones, and avoids the module re-implementing partial SSRF logic that could drift
from Charon's `network.IsPrivateIP`.

**No-op / minimal default**: ship the module with a conservative built-in default validator (reject
non-HTTPS, reject IP literals resolving to RFC 1918/loopback/link-local/reserved ranges — i.e., a
self-contained reimplementation of the *IP classification* portion only, which has zero
Charon-specific dependencies as confirmed in §2.2) so the module is immediately useful standalone
without forcing every consumer to write SSRF logic from scratch. Charon overrides this default with
its own `network`/`security`-backed validator via `WithURLValidator` to keep single-source-of-truth
SSRF policy. This is explicitly called out as an **open question** in §7 — it duplicates ~140 LOC of
IP-classification logic between Charon's `internal/network` and the new module's default validator,
which is an acceptable, bounded duplication (public IP-range constants, not business logic) but
should be a conscious choice, not an accident.

#### Seam 4 — env-var overrides (`CHARON_NOTIFY_ALLOW_HTTP`, `CHARON_NOTIFY_MAX_REDIRECTS`)

Currently read via `os.Getenv` directly inside `http_wrapper.go`. Replace with constructor
parameters (`allowHTTP bool`, `maxRedirects int`) passed by the caller. Charon's
`NewNotificationService`/adapter reads its own env vars (still named `CHARON_NOTIFY_*` for
backward compatibility with existing deployments) and passes the resolved values in. This removes
the module's only direct env/config coupling and makes it framework-agnostic (a consumer using
Viper, flags, or hardcoded config all work identically).

### 3.3 Public API surface (new module)

The module now has four layers, not one: the delivery primitive (§3.3.1, unchanged design from the
prior draft, just relocated to a `transport` subpackage — see the naming note in §3.5/§7), a
generic `Message` type shared by every provider (§3.3.2), a `Sender` interface implemented per
provider package (§3.3.3), and an email-specific `Mailer`/`TemplateRenderer` design (§3.3.4). Only
§3.3.1 was in the original Phase-1 scope; §§3.3.2–3.3.4 are new to this revision.

#### 3.3.1 Delivery primitive (`transport` package)

```go
package transport

// Wrapper dispatches outbound notification payloads with SSRF-safe validation,
// retry/backoff, and response-size caps.
type Wrapper struct { /* unexported */ }

// Option configures a Wrapper at construction time.
type Option func(*wrapperConfig)

func WithClientFactory(f ClientFactory) Option
func WithURLValidator(v URLValidator) Option
func WithRetryPolicy(p RetryPolicy) Option
func WithAllowHTTP(allow bool) Option
func WithMaxRedirects(n int) Option

func NewWrapper(opts ...Option) *Wrapper

type RetryPolicy struct {
    MaxAttempts int
    BaseDelay   time.Duration
    MaxDelay    time.Duration
}

type Request struct {
    URL     string
    Headers map[string]string
    Body    []byte
}

type Result struct {
    StatusCode   int
    ResponseBody []byte
    Attempts     int
}

func (w *Wrapper) Send(ctx context.Context, req Request) (*Result, error)
```

This is a rename/generalization of `HTTPWrapper`/`HTTPWrapperRequest`/`HTTPWrapperResult`. Behavior
(retry/backoff, header allowlist, size caps, redirect re-validation) is unchanged from
`http_wrapper.go`. **What's new in this revision**: every HTTP-based provider package in §3.3.3
calls into this same `Wrapper` rather than making its own `net/http` calls — this is what resolves
the discord/generic-path inconsistency flagged in §3.1.2 (today, Discord dispatch bypasses the
wrapper entirely).

#### 3.3.2 Generic `Message` type (module root package, proposed `notify`)

Replaces Charon's `HostName`/`ServiceCount`-style fields with something generic enough for an
unrelated project's domain:

```go
package notify

// Message is the generic, provider-agnostic notification payload. Host
// applications map their own domain events into a Message before calling a
// Sender or the email package's client.
type Message struct {
    // Title is a short headline (was Charon's data["Title"]).
    Title string

    // Body is the human-readable message text (was Charon's data["Message"]).
    Body string

    // EventType is a free-form, host-defined category string. Provider
    // packages treat it as an opaque template field only — never for
    // routing, access-control, or filtering decisions. (Charon's own
    // proxy_host/cert/uptime/security_* routing logic stays entirely in
    // Charon's SendExternal — see §3.1.3. A different adopter would define
    // its own event-type vocabulary; the module has no opinion on it.)
    EventType string

    // Timestamp defaults to time.Now() if zero when Send is called.
    Timestamp time.Time

    // Data holds arbitrary structured extras (replaces Charon's
    // HostName/HostIP/ServiceCount/Services fields). Provider templates
    // expose it as {{toJSON .Data}} or {{index .Data "key"}}.
    Data map[string]any
}
```

#### 3.3.3 `Sender` interface and provider packages

```go
package notify

// Sender dispatches a Message through one specific provider's transport and
// payload shape. Every providers/* package (including providers/email)
// returns a type implementing this, so a host application can treat all
// configured destinations uniformly.
type Sender interface {
    Send(ctx context.Context, msg Message) error
}
```

Each HTTP-based provider package owns its own `Config`, URL/token validation, JSON payload shape,
and header/auth construction (per the function-level table in §3.1.2), but shares the same template
engine (`providers/internal/render`, unexported) and the same `transport.Wrapper` for dispatch.
Two representative examples — the rest (`slack`, `gotify`, `pushover`, `ntfy`, `telegram`) follow the
identical shape, differing only in `Config` fields and the provider-specific validation/header logic
already itemized in §3.1.2:

```go
package discord

type Config struct {
    WebhookURL     string // validated against discord.com/canary.discord.com per §3.1.2
    Template       string // "minimal" | "detailed" | "custom"
    CustomTemplate string
}

func New(cfg Config, w *transport.Wrapper) *Client
func (c *Client) Send(ctx context.Context, msg notify.Message) error
```

```go
package webhook

type Config struct {
    URL            string // arbitrary destination; no host allowlist (unlike discord/slack)
    Template       string
    CustomTemplate string
}

func New(cfg Config, w *transport.Wrapper) *Client
func (c *Client) Send(ctx context.Context, msg notify.Message) error

// RenderPreview renders tmplStr against msg without dispatching — used by a
// host app's provider-editor UI to validate a custom template before saving
// it. Replaces Charon's RenderTemplate (§3.1.2); reusable for previewing any
// of the seven provider types since they share the same template mechanism.
func RenderPreview(tmplStr string, msg notify.Message) (rendered string, parsed any, err error)
```

Telegram, unlike in earlier drafts of this spec, **is** one of these packages — the user confirmed
including it as the module's seventh provider (§7 risk 1e, resolved; §3.6 step 6), so it moves and
is cut over on the same per-provider commit pattern as the other six (§6).

**Extensibility design, without building it yet (see §8):** the uniform `Sender` interface and the
"each provider is a fully self-contained package, importable independently, with no central
switch-statement inside the module" convention are chosen deliberately so that a future
Apprise-style registry (e.g. `notify.Register(scheme string, factory func(cfg string) (Sender,
error))`, letting a caller dispatch off a URL like `discord://...`) can be bolted on later as pure
addition — no existing provider package needs to change shape to support it. This extraction does
**not** build that registry: with seven known providers and one consumer (Charon), a generic
registry mechanism today would be premature abstraction. The provider-type-string-to-package
mapping stays where it already naturally lives — Charon's own `notify_provider_adapter.go` (§3.6) —
rather than inside the module, which is exactly the seam a future registry would replace without
touching `providers/discord`, `providers/slack`, etc. individually.

#### 3.3.4 `Mailer`/`TemplateRenderer` and the `providers/email` package

Unlike the other five providers, email dispatch today lives entirely in Charon's `mail_service.go`
— SMTP transport, connection lifecycle, and five branded HTML templates
(`email_base.html`/`email_security_alert.html`/`email_ssl_event.html`/`email_uptime_event.html`/
`email_system_event.html`, all saying "Charon" / "Charon Reverse Proxy Manager"). There is no
existing `internal/notifications` email code to move — this is new abstraction design layered over
existing Charon code, which makes it the highest-design-risk piece of this extraction (see §7).

**Design decision**: the module never dials SMTP and never renders HTML directly by default. Two
interfaces isolate the two concerns the host application owns:

```go
package email

// Mailer transports an already-composed email. The host application owns
// SMTP configuration, authentication, and connection lifecycle — the module
// never sees credentials.
type Mailer interface {
    Send(ctx context.Context, recipients []string, subject, htmlBody string) error
}

// TemplateRenderer renders an HTML email body for a Message using a
// host-selected template name. Optional: if the host doesn't supply one,
// the package falls back to its own single neutral built-in template.
type TemplateRenderer interface {
    Render(templateName string, msg notify.Message) (htmlBody string, err error)
}

type Config struct {
    Recipients    []string
    SubjectPrefix string                        // "" by default — no "[Charon Alert]" baked in
    TemplateName  func(msg notify.Message) string // optional; host's event-type -> template-name mapping. nil = constant "default".
    Renderer      TemplateRenderer                // optional; nil uses the package's built-in neutral template
    Mailer        Mailer                          // required
}

func New(cfg Config) *Client
func (c *Client) Send(ctx context.Context, msg notify.Message) error // implements notify.Sender
```

**Tradeoff, decided**: ship exactly **one** neutral, unbranded, inline-styled default HTML template
in the module (no logo, no product name) so the module is immediately useful standalone with zero
config — a bare `Mailer` implementation is enough to get working email out of a fresh adopter.
Charon **must** override `Renderer` (wrapping its existing `MailServiceInterface.RenderNotificationEmail`
and its five branded templates) and `TemplateName` (wrapping `emailTemplateForEventType`, which
stays in Charon per §3.1.3) — this is not optional, since dropping to the module's neutral default
would be a user-visible regression of Charon's existing branded, event-differentiated email design.
This requirement is called out explicitly in the Charon migration plan (§3.6) and as a risk in §7,
not left implicit.

`sanitizeForEmail`'s control-char stripping (§3.1.2) is applied unconditionally inside `Send` to
`msg.Title`/`msg.Body` before either subject formatting or template rendering — generic hygiene,
zero Charon dependency, no reason to make it optional.

**Branding removal, applied module-wide**: the `User-Agent: Charon-Notify/1.0` header
(`sendJSONPayload`, line 534) becomes a generic module default (e.g. `notify-transport/1.0`),
overridable per-provider-package `Config` if a host wants its own UA string. `"[Charon Alert]"` and
`"[Charon Test]"` become Charon-side `SubjectPrefix` values passed into the adapter (§3.6), not
module defaults.

### 3.4 Database schema changes

None. This extraction touches zero database schema — all persistence stays in Charon's
`internal/models`.

### 3.5 New repo structure

Provider-specific senders are now built out for real (the prior draft sketched a `providers/`
layout as "not built now" — that's flipped). **The repo already exists** at
`/projects/go_notify_yourself` (`github.com/Wikid82/go_notify_yourself`, currently just `LICENSE` +
a placeholder `README.md`) — the extraction session scaffolds into it, it does not create a new
repo. The Go package name at the module root is `notify` (see the rev-2 note at the top of this
document for why that's a deliberate, unproblematic mismatch with the directory/module name).

```
go_notify_yourself/                  # existing repo at /projects/go_notify_yourself — see rev-2 note
├── go.mod                           # module github.com/Wikid82/go_notify_yourself
├── go.sum
├── LICENSE                          # match Charon's license
├── README.md                        # public API docs, usage examples per provider, SSRF-seam explanation
├── CHANGELOG.md                     # Keep a Changelog format, driven by Conventional Commits
├── .goreleaser.yaml                 # mirrors Charon's root .goreleaser.yaml, Go-module release only
├── .github/
│   └── workflows/
│       ├── ci.yml                   # go test ./..., go vet, staticcheck, coverage gate
│       └── release.yml              # tag-triggered GoReleaser run
├── message.go                       # Message struct (§3.3.2)
├── sender.go                        # Sender interface (§3.3.3)
├── message_test.go
├── transport/                       # the SSRF-safe delivery primitive (§3.3.1)
│   ├── wrapper.go                   # Wrapper, Option, NewWrapper, Send — from http_wrapper.go
│   ├── wrapper_test.go
│   ├── client_executor.go           # test seam, from http_client_executor.go
│   ├── retry.go                     # RetryPolicy + backoff/jitter helpers
│   ├── validate_default.go          # built-in conservative URLValidator default (§3.2 Seam 3)
│   ├── validate_default_test.go
│   └── integration/
│       └── wrapper_integration_test.go  # from backend/integration/notification_http_wrapper_integration_test.go
└── providers/
    ├── internal/
    │   └── render/                  # unexported shared text/template + toJSON engine (§3.1.2)
    │       ├── render.go
    │       └── render_test.go
    ├── discord/
    │   ├── discord.go               # Config, New, (*Client).Send — webhook validation, content/embeds normalization
    │   └── discord_test.go
    ├── slack/
    │   ├── slack.go                 # Config, New, (*Client).Send — webhook validation, text/blocks normalization
    │   └── slack_test.go
    ├── gotify/
    │   ├── gotify.go                # Config, New, (*Client).Send — X-Gotify-Key header, message-field validation
    │   └── gotify_test.go
    ├── pushover/
    │   ├── pushover.go              # Config, New, (*Client).Send — token/user injection, hostname pin
    │   └── pushover_test.go
    ├── ntfy/
    │   ├── ntfy.go                  # Config, New, (*Client).Send — Bearer auth header
    │   └── ntfy_test.go
    ├── telegram/
    │   ├── telegram.go              # Config, New, (*Client).Send — bot-token-in-URL dispatch, chat_id injection, hostname pin
    │   └── telegram_test.go
    ├── webhook/
    │   ├── webhook.go               # Config, New, (*Client).Send — generic/custom JSON dispatch
    │   ├── preview.go                # RenderPreview(tmplStr, msg) — public (§3.1.2)
    │   └── webhook_test.go
    └── email/
        ├── email.go                 # Config, Mailer, TemplateRenderer, New, (*Client).Send (§3.3.4)
        ├── default_template.go      # single neutral, unbranded built-in HTML template
        └── email_test.go
```

### 3.6 Migration plan for Charon (after the new module exists)

1. **Add dependency**: `go get github.com/Wikid82/go_notify_yourself@v0.1.0` in `backend/go.mod`.
2. **Delete extracted files/logic**:
   - Whole files: `backend/internal/notifications/{http_wrapper,http_wrapper_test,http_client_executor,engine,router,router_test}.go`.
   - Function-level deletions inside `notification_service.go` (now that the module owns this
     logic — not kept as dead code): `minimalTemplate`/`detailedTemplate` consts, the template
     parse/exec block, all Discord/Slack/Gotify/Pushover/Ntfy/Telegram regex/validation/
     normalization/header/dispatch-URL-build code, `sendJSONPayload` and `RenderTemplate` themselves
     (replaced by adapter calls), `sanitizeForEmail`, `dispatchEmail`'s message-composition
     internals, `webhookDoRequestFunc`, and the dead `isPrivateIP` wrapper (§3.1.3).
   - Keep `feature_flags.go` (per §2.2, it stays — Charon policy, not engine code); fold it into
     `internal/services` rather than keeping a single-file package once the rest of
     `internal/notifications` is gone.
3. **Add Charon-side adapters** (new files):
   - `notify_client_adapter.go`: wires `network.NewSafeHTTPClient`/`security.ValidateExternalURL`
     into `transport.ClientFactory`/`URLValidator` (§3.2), resolves `CHARON_NOTIFY_ALLOW_HTTP`/
     `CHARON_NOTIFY_MAX_REDIRECTS`. One shared `*transport.Wrapper` instance, injected into every
     HTTP-based provider package.
   - `notify_provider_adapter.go` (**new**): per-`provider.Type` factory mapping a GORM
     `models.NotificationProvider` row into the matching `discord.Config`/`slack.Config`/
     `gotify.Config`/`pushover.Config`/`ntfy.Config`/`telegram.Config`/`webhook.Config` and
     constructing the corresponding `notify.Sender`.
   - `notify_email_adapter.go` (**new**): implements `email.Mailer` (wrapping
     `s.mailService.SendEmail`) and `email.TemplateRenderer` (wrapping
     `s.mailService.RenderNotificationEmail`), and supplies `TemplateName: emailTemplateForEventType`
     (kept in Charon, §3.1.3) and `SubjectPrefix: "[Charon Alert] "` to preserve the exact
     user-visible subject format.
4. **Update `notification_service.go`**: `SendExternal`'s per-provider dispatch becomes — build a
   `notify.Message` from `title`/`message`/`eventType`/`data`; use `notify_provider_adapter.go` to
   build the right `Config` + `Sender` for `provider.Type`; call `sender.Send(ctx, msg)`; log the
   result. `TestProvider`/`TestEmailProvider` become the same shape. Everything named in §3.1.3
   (CRUD, `isDispatchEnabled`/`getFeatureFlagValue`, `emailTemplateForEventType`,
   `EnsureNotifyOnlyProviderMigration`, the provider-type allowlists) is unchanged.
5. **Resolve the "detailed" template backward-compatibility question** (flagged in §3.1.2/§7): the
   module's generic `detailed` template nests host-specific fields under `Data` instead of exposing
   `HostName`/`HostIP`/`ServiceCount`/`Services` at the JSON top level. Decide, explicitly, whether
   Charon supplies its own `CustomTemplate` string reproducing the old flat shape for
   already-configured `detailed`-template providers (safer, avoids a silent payload-shape change
   for existing integrations) or accepts the shape change with a changelog note. Do not let this be
   decided implicitly by whichever behavior the ported code happens to produce.
6. **Telegram gap — RESOLVED.** User confirmed: add `providers/telegram` to the module alongside
   the other six, so all seven of Charon's current provider types are consistent (none bypass the
   module as a special case). Update §3.1.3, §3.5's `providers/` layout, and the Appendix move-list
   to include a `providers/telegram` package; it moves through the same commit-per-provider pattern
   as the other six in §6.
7. **Update the integration test import**: delete
   `backend/integration/notification_http_wrapper_integration_test.go` (moved to the new repo); if
   Charon wants an adapter-level integration test proving the DI seams work end-to-end with real
   `network`/`security` code, write a **new**, small integration test — not a port of the old one.
8. **Preserve coverage, and expect substantial test rewrites, not just import changes**: this
   extraction now removes the majority of `notification_service.go`'s production code (roughly 450
   of 1029 lines — everything in §3.1.2), not just the ~570 LOC `http_wrapper.go` alone. The three
   existing test files most affected —
   `notification_service_test.go`/`notification_service_json_test.go`/
   `notification_service_discord_only_test.go` (125 KB combined) — assert directly on internals
   (payload shapes, validation error strings, header construction) that are moving to the module.
   These suites need **real rewriting** against the new adapter seam, not a mechanical
   import-path swap. See the relaxed acceptance criterion in §5 and the new risk in §7 — the prior
   draft's "existing suites pass unmodified" bar does not hold for this scope.
9. **Update `docs/features/notifications.md`**: no required content change (documents the product
   feature, not the internal package); optionally credit the new module.
10. **Update `ARCHITECTURE.md`**: per CLAUDE.md's mandatory rule, add a line noting outbound
    notification dispatch (all seven provider types plus email) now goes through the external
    `notify` module with Charon-supplied SSRF/SMTP/template adapters, rather than internal packages.

### 3.7 Error handling / edge cases for the extraction session to watch

- **Import cycle risk**: none anticipated — `internal/network` and `internal/security` don't
  import `internal/notifications` or `internal/services`, so removing the notifications→network/
  security edge and replacing it with notifications←(Charon adapter)→network/security is a clean
  DI inversion, not a cycle fix. This still holds with the larger scope: the new
  `notify_provider_adapter.go`/`notify_email_adapter.go` adapters are additional inversion points of
  the same shape, not new dependency directions.
- **Env var behavior drift**: `allowNotifyHTTPOverride()` currently special-cases
  `os.Args[0]` ending in `.test` to auto-allow HTTP during `go test`. If this logic moves to
  Charon's `notify_client_adapter.go` (per Seam 4), the adapter must preserve this test-detection
  behavior or existing tests that rely on it will start failing against real HTTPS-only validation.
- **`os.Args[0]` test-detection is itself a code smell** worth flagging to the extraction session:
  it's exactly the kind of implicit-environment coupling the module boundary should force out.
  Recommend the Charon adapter accept an explicit `allowHTTP bool` (e.g. from `CHARON_ENV`) rather
  than sniffing `os.Args[0]`, and that test setup pass it explicitly. This is a minor
  behavior-preserving refactor to fold into the migration commits, not a new risk.
- **Provider-specific dispatch-URL construction is not uniform** (confirmed on this re-read):
  Telegram/Pushover build their dispatch URL from a base-URL + path (with a hostname-pin check
  against DNS-spoofed base URLs); Slack substitutes a decrypted token as the entire dispatch URL;
  Gotify/Ntfy dispatch to `provider.URL` directly and add an auth header instead. Each provider
  package's `Send` must reproduce its specific construction exactly — this is precisely the
  behavior-parity risk called out in §7, not a detail that can be generalized away.

---

## 4. Implementation Plan (for the future extraction session — not executed here)

This plan is written for reference by the session that actually performs the extraction. It follows
this repo's phase convention but the "Playwright"/"Frontend" phases are replaced since this is a
backend-only, cross-repo change with no UI surface. Expanded from the prior 5-phase Phase-1-only
plan to cover the full provider layer.

### Phase 1: New-repo scaffolding + shared types + transport (new repo)
- Scaffold into the **existing** `/projects/go_notify_yourself` repo (`LICENSE` already present):
  `go.mod` (`module github.com/Wikid82/go_notify_yourself`), CI workflow, GoReleaser config (§3.5).
- Write `message.go` (`Message`, §3.3.2) and `sender.go` (`Sender`, §3.3.3) at the module root.
- Copy `http_wrapper.go` → `transport/wrapper.go`, renaming exported identifiers per §3.3.1,
  replacing the two Charon imports with the `ClientFactory`/`URLValidator` seam (§3.2).
- Copy/adapt `http_wrapper_test.go`, `http_client_executor.go` into `transport/`.
- Write the built-in default `URLValidator` (§3.2 Seam 3) + its own tests.
- Copy the integration test (§2.6) into `transport/integration/`, update package/import path.
- `go test ./...`, `go vet ./...`, `staticcheck ./...` all green.

### Phase 2: Provider packages (new repo)
- `providers/internal/render`: extract the shared `text/template` + `toJSON` engine (§3.1.2) out of
  `sendJSONPayload`/`RenderTemplate`, generic over `notify.Message`.
- `providers/discord`, `providers/slack`, `providers/gotify`, `providers/pushover`, `providers/ntfy`,
  `providers/telegram`, `providers/webhook`: one package at a time, each porting its slice of §3.1.2's table (URL
  validation, JSON normalization, dispatch URL/header construction), consolidating Discord onto
  `transport.Wrapper` per the flagged inconsistency. `providers/webhook` additionally gets
  `RenderPreview`.
- `providers/email`: build `Mailer`/`TemplateRenderer`/`Config`/`Client` (§3.3.4), including the one
  neutral default HTML template.
- Each package ships with its own tests at parity with (or exceeding) the coverage the equivalent
  logic had inside `notification_service_*_test.go`.
- Tag `v0.1.0` once all provider packages + transport are green.

### Phase 3: Charon-side adapters (Charon repo, this repo)
- Add `github.com/Wikid82/go_notify_yourself` to `backend/go.mod`.
- Write `notify_client_adapter.go` (transport seam, §3.6 step 3).
- Write `notify_provider_adapter.go` (per-type `Config`/`Sender` factory, §3.6 step 3).
- Write `notify_email_adapter.go` (`Mailer`/`TemplateRenderer` wiring, §3.6 step 3).
- Resolve the "detailed" template backward-compat decision (§3.6 step 5) and the Telegram gap
  (§3.6 step 6) explicitly, before proceeding to cutover.

### Phase 4: Charon cutover
- Update `notification_service.go`'s `SendExternal`/`TestProvider`/`TestEmailProvider` to call the
  new adapters (§3.6 step 4), **split per provider per §6** for reviewability rather than one giant
  commit.
- Delete extracted files and dead function-level code from `notification_service.go` and
  `backend/internal/notifications/` (§3.6 step 2).
- Delete `backend/integration/notification_http_wrapper_integration_test.go`.
- Rewrite (not just relink) the affected slices of
  `notification_service_test.go`/`notification_service_json_test.go`/
  `notification_service_discord_only_test.go` against the new adapter seam.
- Run full backend test suite + coverage gate (§3.6 step 8).

### Phase 5: Hardening + docs
- `ARCHITECTURE.md` update (§3.6 step 10).
- CodeQL/Trivy re-run on Charon (new external dependency, larger surface than the Phase-1 draft).
- Confirm `go.sum`/supply-chain scan clean for the new module dependency.

### Phase 6: Deployment
- Tag Charon release per normal Conventional Commits flow. Unlike the Phase-1-only draft, this is
  **not** guaranteed behavior-invisible — the "detailed" template shape decision (Phase 3) and the
  Discord-dispatch consolidation (§3.1.2) are both potential user-visible or operationally-visible
  changes and should be called out in release notes if either is accepted as-is rather than shimmed
  for compatibility.

---

## 5. Acceptance Criteria (for the extraction session's Definition of Done)

- [ ] New repo exists, `go test ./...` and `staticcheck` pass with zero findings, tagged `v0.1.0`.
- [ ] New module has zero imports of anything under `github.com/Wikid82/charon/*`.
- [ ] Each of the seven provider packages (`discord`, `slack`, `gotify`, `pushover`, `ntfy`,
      `telegram`, `webhook`) plus `providers/email` has its own test suite at ≥85% coverage (mirrors
      Charon's own bar).
- [ ] `providers/webhook.RenderPreview` covers custom-template validation equivalent to the old
      `RenderTemplate`'s test coverage.
- [ ] `providers/email`'s default built-in template is neutral/unbranded — a grep for `Charon` (or
      any other host-app name) across the new repo returns zero hits outside README/CHANGELOG.
- [ ] Charon's `go.mod` depends on the new module at a pinned semver tag (no `replace` directive
      left in place post-merge).
- [ ] `backend/internal/notifications/` package is deleted entirely (unlike the Phase-1 draft,
      `feature_flags.go` is folded into `internal/services` rather than left as a lone-file package
      — §3.6 step 2).
- [ ] `notification_service.go`'s black-box behavior is unchanged **except** for the two explicitly
      documented, deliberate changes (the "detailed" template payload shape and the Discord-dispatch
      retry/backoff consolidation) — both must be resolved as conscious decisions per §3.6 steps 5
      and the risk in §7, not accidental drift. This replaces the prior draft's stronger claim that
      existing test assertions pass *unmodified*: with this scope, rewriting
      `notification_service_test.go`/`notification_service_json_test.go`/
      `notification_service_discord_only_test.go` is expected and required, but the rewritten
      assertions must still prove equivalent (or knowingly-changed) external behavior.
- [ ] Charon's backend coverage gate (`scripts/go-test-coverage.sh`, min 85%) still passes after the
      ~450-line reduction in `notification_service.go` and the corresponding test rewrites.
- [ ] `ARCHITECTURE.md` updated.
- [ ] No behavior change observable from the frontend or API beyond the two documented exceptions
      above.

---

## 6. Commit Slicing Strategy

This spec spans **two repositories**, so "one feature = one PR" applies **per repository**: the new
module's scaffolding-through-providers work is one PR in the new repo; Charon's consumption of it is
a second, separate PR in *this* repo (two different features in two different codebases, each
individually complete and mergeable on its own — not a violation of one-feature-one-PR). Within each
PR, commits are ordered and logical. The full-scope decision roughly **triples** the new-module PR's
commit count and requires splitting the Charon cutover per-provider for reviewability, per the
revision brief.

### New-module repo — PR "Initial notify engine + provider layer"

1. **Commit 1** — Scaffolding: `go.mod`, LICENSE, README stub, CI workflow (no behavior). Gate: `go build ./...`.
2. **Commit 2** — Shared types: `message.go` (`Message`), `sender.go` (`Sender`). Gate: `go build ./...`.
3. **Commit 3** — Transport core: `transport/wrapper.go`, `client_executor.go`, `retry.go`, ported
   from `http_wrapper.go`/`http_client_executor.go` with seam interfaces substituted for direct
   `network`/`security` calls. Gate: `go vet`, `staticcheck`.
4. **Commit 4** — Transport tests: `transport/wrapper_test.go`, adapted to inject fake
   `ClientFactory`/`URLValidator`. Gate: `go test ./...` green, coverage ≥85%.
5. **Commit 5** — Default validator: `transport/validate_default.go` + tests (§3.2 Seam 3). Gate: tests green.
6. **Commit 6** — Transport integration test: `transport/integration/wrapper_integration_test.go`.
   Gate: `go test -tags=integration ./...`.
7. **Commit 7** — Shared render engine: `providers/internal/render` (§3.1.2). Gate: tests green.
8. **Commit 8** — `providers/discord` (webhook validation, content/embeds normalization, dispatch
   consolidated onto `transport.Wrapper`). Gate: `go test ./providers/discord/...` ≥85%.
9. **Commit 9** — `providers/slack`. Gate: same pattern.
10. **Commit 10** — `providers/gotify`. Gate: same pattern.
11. **Commit 11** — `providers/pushover`. Gate: same pattern.
12. **Commit 12** — `providers/ntfy`. Gate: same pattern.
13. **Commit 13** — `providers/telegram` (bot-token-in-URL dispatch build, hostname pin, `chat_id`
    injection, `text`/`message`-field payload validation per §3.1.2). Gate: `go test
    ./providers/telegram/...` ≥85%.
14. **Commit 14** — `providers/webhook` (generic dispatch + `RenderPreview`). Gate: same pattern.
15. **Commit 15** — `providers/email` (`Mailer`/`TemplateRenderer`/`Config`, one neutral default
    template). Gate: `go test ./providers/email/...` ≥85%; this is the highest-design-risk commit
    (§7) and should get dedicated review attention, not be rubber-stamped alongside the others.
16. **Commit 16** — Release plumbing: `.goreleaser.yaml`, release workflow, `CHANGELOG.md` seed.
    Gate: dry-run `goreleaser release --snapshot`.

Rollback: any commit can be reverted independently since the repo has no existing consumers yet;
worst case the repo simply isn't tagged until it's right.

### Charon repo — PR "Consume extracted notify module"

1. **Commit 1** — Dependency + transport adapter: add `go.mod` requirement, write
   `notify_client_adapter.go` + tests. No behavior change yet. Gate: `go build ./...`, adapter tests pass.
2. **Commit 2** — Provider + email adapters: `notify_provider_adapter.go`,
   `notify_email_adapter.go` (unused by production code paths yet). Gate: `go build ./...`, adapter
   tests pass.
3. **Commit 3** — Cutover: Discord. `SendExternal`/`TestProvider` route Discord dispatch through
   `providers/discord`. Gate: `notification_service_discord_only_test.go` passes (rewritten per
   §3.6 step 8) and explicitly documents the retry/backoff behavior change from consolidating onto
   `transport.Wrapper`. **This commit's description and the release changelog entry must call out
   the retry-behavior change explicitly** — state it plainly as "Discord notifications now retry on
   transient failures" (per §7 risk 1c, resolved) rather than letting it read as "just a refactor";
   it is a user-visible, operator-noticeable improvement, not an implementation detail.
4. **Commit 4** — Cutover: Slack. Gate: relevant slice of `notification_service_test.go` rewritten and green.
5. **Commit 5** — Cutover: Gotify. Gate: same pattern.
6. **Commit 6** — Cutover: Pushover. Gate: same pattern.
7. **Commit 7** — Cutover: Ntfy. Gate: same pattern.
8. **Commit 8** — Cutover: Telegram. `SendExternal`/`TestProvider` route Telegram dispatch through
   `providers/telegram` (bot-token-in-URL dispatch build, `chat_id` injection from `p.URL`,
   hostname-pin check). Gate: relevant slice of `notification_service_test.go` rewritten and green,
   same pattern as the other provider cutovers.
9. **Commit 9** — Cutover: generic Webhook, including replacing `RenderTemplate` call sites in
   `CreateProvider`/`UpdateProvider` with `providers/webhook.RenderPreview`. Gate: same pattern, plus
   the "detailed" template backward-compat decision (§3.6 step 5) is implemented here, not deferred.
10. **Commit 10** — Cutover: Email. `dispatchEmail`/`TestEmailProvider` route through
    `notify_email_adapter.go`; `SubjectPrefix`/`TemplateName` preserve exact current subject/template
    behavior. Gate: email-path tests rewritten and green; grep confirms no accidental exposure of the
    module's neutral default template in production.
11. **Commit 11** — Cleanup: delete now-dead code — old `sendJSONPayload`/`RenderTemplate`/
    `dispatchEmail`/`sanitizeForEmail`/validation functions, `isPrivateIP`, all of
    `backend/internal/notifications/`, the old integration test. Gate: `go build ./...`, no unused
    imports/symbols (staticcheck).
12. **Commit 12** — Coverage/lint/docs hardening: re-run `scripts/go-test-coverage.sh`, fix any gate
    regression; update `ARCHITECTURE.md`. Gate: full Definition of Done per CLAUDE.md.

Rollback for the PR as a whole: since this is a pure dependency swap with no schema/API change, a
full revert of the PR is safe at any point before merge; post-merge, `go.mod` can be pinned back to
the pre-extraction commit and the deleted files restored from git history if an unforeseen
regression surfaces — no data migration to unwind. Per-provider commit slicing (3–10) additionally
means a single provider's cutover can be reverted in isolation post-merge without unwinding the
others, which was not possible under the prior single-commit-cutover plan.

Contingency: if the extraction session discovers a provider's DI seam is insufficient (e.g. SSRF
policy or auth-header handling genuinely can't be expressed through the shared interfaces without
either leaking Charon internals or weakening a provider's safety), stop before that provider's
cutover commit and re-scope that one provider — the per-provider commit slicing means this no longer
blocks the other six providers' cutover from proceeding.

---

## 7. Risks / Open Questions

1. **Scope of "the engine" — RESOLVED.** The user has confirmed full provider-layer scope: the
   Discord/Slack/Gotify/Pushover/Ntfy/webhook payload builders and email dispatch move into the new
   module now. This replaces the prior draft's open question. The scope increase introduces the
   following new risks (1a–1e), which did not exist under the Phase-1-only plan:

   - **1a. Behavior-parity risk across 6+ providers is much higher than for a single HTTP wrapper.**
     Each provider's URL validation, header construction, and auth-injection quirks (Discord's
     regex/host-allowlist, Slack's token-substitution, Pushover's hostname-pinned URL build, Gotify's
     header vs. Ntfy's bearer-auth header) must be reproduced exactly or the 125 KB of existing
     `notification_service_*_test.go` coverage will catch regressions the extraction session must
     then triage one provider at a time. Budget real time for this — it is not a mechanical port.
   - **1b. Template/payload shape change risk.** Genericizing the built-in `detailed` template
     (dropping top-level `HostName`/`HostIP`/`ServiceCount`/`Services` in favor of a nested
     `{{toJSON .Data}}`) is a **user-visible breaking change** for any existing custom integration
     parsing the old flat JSON keys. §3.6 step 5 requires this to be a conscious decision (Charon
     ships a compatibility `CustomTemplate` for existing providers, or accepts the change with a
     changelog note) — flag to the user before the cutover commit ships either way.
   - **1c. Discord/generic-webhook dispatch consolidation — RESOLVED.** §3.1.2 found that Discord
     dispatch today bypasses `HTTPWrapper` entirely (direct `network`/`security` calls, no
     retry/backoff). User confirmed: fold Discord onto the shared `transport.Wrapper` along with
     every other provider — it gains retry/backoff it lacked before, and all providers share one
     dispatch path with no special case. This is a deliberate, user-approved behavior change (not
     merely a refactor); document it in the Charon PR description and changelog as "Discord
     notifications now retry on transient failures," since it's an observable improvement an
     operator could notice.
   - **1d. Email is the trickiest single piece.** It currently lives entirely in Charon's
     `mail_service.go` (SMTP + 5 branded HTML templates), not in `internal/notifications` at all —
     so unlike the other five providers, there's no existing engine code to port, only a new
     `Mailer`/`TemplateRenderer` abstraction to design and retrofit around existing Charon code.
     Higher design risk than any HTTP provider; §6 gives it a dedicated commit in both PRs and flags
     it for extra review attention rather than folding it in alongside the others.
   - **1e. Telegram gap — RESOLVED.** Charon supports a 7th provider type (`telegram`) that was
     outside the original six-provider list. User confirmed: include it — `providers/telegram` is
     added to the module alongside the other six (§3.6 step 6), so the move-list is now seven HTTP
     providers + email, not six.
   - **1f. Scope-creep guardrail for the Apprise-inspired long-term direction (§8).** The provider
     list in §3.1 is deliberately exactly Charon's existing seven HTTP providers + email (six plus
     Telegram, per 1e above) — nothing more. The extraction session must resist the temptation to
     "just add one more" (Matrix,
     PagerDuty, Twilio, etc.) even though the API is designed to make that easy later (§3.3.3, §8).
     Adding providers Charon doesn't use today is explicitly out of scope for this extraction and
     would need its own separate decision from the user once the project is closer to maintenance
     mode.

2. **Default `URLValidator` duplication** (§3.2 Seam 3): shipping a built-in conservative SSRF
   validator in the new module duplicates IP-classification logic already in Charon's
   `internal/network`. Bounded and low-risk (public CIDR constants, not business logic), but worth
   the user's explicit sign-off since "duplicate SSRF logic" is the kind of thing that should never
   happen by accident.
3. **Module name/org — RESOLVED.** The repo already exists at `/projects/go_notify_yourself`
   (`github.com/Wikid82/go_notify_yourself`, remote confirmed via `git remote -v`), so this is no
   longer an open placeholder question — every reference in this spec now uses that path, with the
   Go package name kept as `notify` at the root (module-path/package-name mismatch is intentional,
   see the rev-2 note at the top of this document). The one remaining sub-decision is whether the
   delivery-primitive subpackage is literally named `transport` as sketched in §3.3.1/§3.5, or
   something else — cosmetic, not blocking.
4. **`os.Args[0]` test-detection removal** (§3.7): behavior-preserving in intent, but any change to
   how `CHARON_NOTIFY_ALLOW_HTTP` is resolved touches existing test setup across
   `notification_service_test.go` and friends (125 KB file) — the extraction session should budget
   time to verify every test relying on the old auto-detect still passes under explicit
   configuration.
5. **`feature_flags.go` fate**: stays in Charon (§3.1.2), folded into `internal/services` once the
   rest of `internal/notifications` is deleted (§3.6 step 2) — a style call, not a functional one.
6. **GoReleaser artifact shape for a pure library**: Charon's existing `.goreleaser.yaml` builds
   binaries/Docker images; a library module needs a much lighter GoReleaser config (just changelog
   + GitHub release, no build/archive stanzas). This is unaffected by the larger provider-layer
   scope — GoReleaser still just tags the whole module regardless of how many packages it contains.
   The extraction session should not copy Charon's `.goreleaser.yaml` wholesale — treat it as a
   reference for style/conventions only.
7. **CI cost for a single-maintainer module**: recommend the new repo's CI stay to lint + unit test
   + coverage on PR/push, with release only on tag push — explicitly no CodeQL/Trivy/multi-browser
   E2E apparatus. The coverage surface is now larger (transport + 6 providers + email vs. just the
   wrapper), but the CI *policy* is unchanged — flagging so the extraction session doesn't
   over-engineer CI to match Charon's much larger surface just because the module itself grew.
8. **Email default-template tradeoff needs explicit sign-off.** §3.3.4 decides to ship one neutral
   built-in template and require Charon to override it. An unrelated future adopter who *doesn't*
   override it gets a plain, unbranded email — acceptable for a zero-config default, but the
   extraction session should confirm this "ship one neutral default, hosts override for anything
   branded" position with the user before locking in the `TemplateRenderer` interface shape, since
   it's a design opinion, not a mechanical extraction fact.
9. **`notification_service_test.go`/`_json_test.go`/`_discord_only_test.go` (125 KB combined) need
   substantial rewriting, not import/construction changes.** The prior Phase-1-only draft's
   acceptance criterion — "existing suites pass without modification to their assertions" — no
   longer holds now that the bulk of the file's production logic (§3.1.2, ~450 of 1029 lines) is
   deleted outright rather than relinked. §5 has been relaxed accordingly: black-box behavior must
   remain equivalent (except the two documented deliberate changes in 1b/1c above), but the
   assertions themselves are expected to be rewritten against the new adapter seam.

---

## 8. Future Direction (context for whoever picks this up later)

**Long-term goal, stated by the user:** `go_notify_yourself` should eventually become a Go
equivalent of [Apprise](https://github.com/caronc/apprise) — the Python library that lets a caller
dispatch a single notification across a large, open-ended catalog of services through one common
interface/URL-scheme convention, rather than hand-rolling per-service integration code.

**Near-term constraint, also stated by the user and binding on this extraction:** Charon and the
user's other small family project are both still under active development, not yet in "maintenance
mode." Scope creep into new provider integrations right now would compete with that active-dev time
for no near-term payoff — there is exactly one consumer (Charon) and it uses exactly seven HTTP
providers + email. §3.1's move-list is intentionally capped at those seven, and §7 risk 1f exists
specifically to stop a future session from "just adding one more" opportunistically during this
extraction.

**What this spec deliberately does do, to keep the Apprise path open without building it now:**
- The `Sender` interface (§3.3.3) is uniform across every provider — `Send(ctx, Message) error` —
  regardless of transport (HTTP POST, SMTP) or payload shape. A future registry only needs one
  interface to key off, not per-provider special cases.
- Every provider package is fully self-contained and independently importable, with **no** central
  switch-statement inside the module mapping type-strings to packages — that mapping lives in
  Charon's own `notify_provider_adapter.go` (§3.6), outside the module. This means the module itself
  has zero knowledge of "which providers exist" beyond the packages present in the repo, which is
  exactly the property an Apprise-style URL-scheme registry (`discord://...`, `mailto://...`) would
  need to slot in as a pure addition later.
- `providers/internal/render`, the shared template engine (§3.1.2), is already factored out as a
  reusable internal dependency rather than duplicated per-package — a future provider package (once
  the scope constraint is lifted) can reuse it immediately instead of re-solving JSON templating.

**What this spec deliberately does NOT do, to avoid scope creep now:**
- No `notify.Register`/registry type is built in this extraction (§3.3.3's extensibility note) —
  with one consumer and seven known providers, a generic registry today is premature abstraction, not
  a real need.
- No URL-scheme parsing/dispatch convention (Apprise's signature feature) is designed or built here
  — that's a substantial API-design exercise in its own right and belongs in a dedicated future spec
  once the user decides it's time to grow past Charon's provider set.
- No providers beyond Charon's existing seven + email are added, discussed as candidates, or
  scaffolded as stubs — see §7 risk 1f.

A future session picking this up for "add provider N" or "build the Apprise-style registry" should
treat this section as the record of *why* the provider list was small at extraction time and
*which* properties of the API (uniform `Sender`, no in-module type registry, shared internal
template engine) were chosen specifically so that later work would be additive rather than a
breaking rework.

---

## Appendix: File-level move list (flat reference)

**Move to new repo (whole files):**
- `backend/internal/notifications/http_wrapper.go`
- `backend/internal/notifications/http_wrapper_test.go`
- `backend/internal/notifications/http_client_executor.go`
- `backend/integration/notification_http_wrapper_integration_test.go`

**Move to new repo (function-level extraction out of `notification_service.go`, genericized — see
§3.1.2 for exact line ranges and destination packages; the file itself does not move, it shrinks):**
- Built-in `minimal`/`detailed` JSON templates → `providers/webhook`
- Template parse/exec engine (`text/template` + `toJSON` funcmap, size/timeout limits) → `providers/internal/render`
- Discord webhook regex/host validation/normalization → `providers/discord`
- Slack webhook regex/validation/token substitution → `providers/slack`
- Gotify message-field validation + auth header → `providers/gotify`
- Pushover message/priority validation + URL build + token/user injection → `providers/pushover`
- Ntfy message-field validation + bearer auth header → `providers/ntfy`
- Telegram text/message-field validation + bot-token-in-URL dispatch build + `chat_id` injection + hostname pin → `providers/telegram`
- Generic/custom webhook dispatch → `providers/webhook`
- `RenderTemplate` → `providers/webhook.RenderPreview`
- `sanitizeForEmail` + `dispatchEmail`'s message composition → `providers/email`

**Delete (dead code, do not port as-is):**
- `backend/internal/notifications/engine.go`
- `backend/internal/notifications/router.go`
- `backend/internal/notifications/router_test.go`
- `notification_service.go`'s unused `isPrivateIP(ip net.IP) bool` wrapper (§3.1.3)
- `notification_service.go`'s `webhookDoRequestFunc` test hook (superseded by the module's own test seam)

**Stays in Charon, unmodified:**
- `backend/internal/models/notification.go`
- `backend/internal/models/notification_config.go`
- `backend/internal/models/notification_provider.go`
- `backend/internal/models/notification_provider_test.go`
- `backend/internal/models/notification_template.go`
- `backend/internal/models/notification_test.go`
- `backend/internal/services/security_notification_service.go` + test
- `backend/internal/services/enhanced_security_notification_service.go` + tests
- `backend/internal/services/uptime_service_notification_test.go`
- `backend/internal/services/mail_service.go`'s SMTP transport + `templates/*.html` (behind the new `Mailer`/`TemplateRenderer` seam, §3.3.4)
- `docs/features/notifications.md`
- `frontend/src/api/notifications.ts` + tests
- `frontend/src/pages/Notifications.tsx` + tests
- `frontend/src/hooks/useNotifications.ts` + tests
- `frontend/src/components/NotificationCenter.tsx` + tests
- `frontend/src/components/SecurityNotificationSettingsModal.tsx` + tests

**Stays in Charon, modified (Charon migration phase, §3.6):**
- `backend/internal/notifications/feature_flags.go` (relocate into `internal/services`, don't extract)
- `backend/internal/services/notification_service.go` (shrinks substantially — §3.1.3 keeps CRUD/flag-gating/event-routing; §3.1.2's logic is deleted, replaced by thin calls into three new adapter files)
- `ARCHITECTURE.md` (documentation update)
