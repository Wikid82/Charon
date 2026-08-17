# Notify Provider Registry — Self-Registering Factory Pattern

Status: Scoping/design only. No code changes performed under this spec. This document specifies
the design for a future implementation session, to land as additional commits on the existing,
open branch `feature/notifications-engine-extraction` (PR #1253), across two repositories.

Owner for this document: **planning** agent.
Owner for execution (future session): a `management`-orchestrated pipeline for the Charon-side
commits; direct TDD implementation in `/projects/go_notify_yourself` per that repo's own
`CLAUDE.md` (which explicitly says not to build out a multi-agent roster there) for the
module-side commits.

---

## 1. Introduction

### 1.1 Objective

`go_notify_yourself` (`github.com/Wikid82/go_notify_yourself`, currently tagged `v0.1.0`) already
ships seven HTTP notification providers plus email, each a self-contained package exposing a typed
`Config` struct and a `New(cfg, wrapper) *Client` constructor implementing the shared
`notify.Sender` interface (confirmed by reading `sender.go`, `message.go`, and the `discord`,
`webhook`, and `email` packages in full). There is deliberately **no** registry today — §8 of the
original extraction spec (`docs/plans/notifications_extraction_spec.md`) called this out as a
conscious "not yet" decision, made when there was exactly one consumer (Charon) and seven known
providers.

That deferred need is now live: the user wants to add a Web Push provider to
`go_notify_yourself` for another project, and wants any future provider — theirs or a third
party's — to become available to a host application (Charon or otherwise) automatically after a
version bump, without hand-editing host application code. Confirmed by reading
`backend/internal/services/notify_provider_adapter.go` (already merged on this branch, commit
`11d3489c`) and `notification_service.go`: Charon currently hardcodes a `switch p.Type { case
"discord": ... }` in `buildNotifySender`, plus three more provider-type allowlists
(`isSupportedNotificationProviderType`, `supportsJSONTemplates`, `isDispatchEnabled`). Adding a
provider today requires a code change in **both** repos.

### 1.2 Goals

- Design a self-registering factory pattern for `go_notify_yourself` — the `database/sql` /
  `image` idiom — fitted to what's actually in the repo today (typed per-provider `Config` structs,
  email's non-serializable `Mailer`/`TemplateRenderer` dependencies), not an idealized redesign.
- Specify the exact `Register`/`New` signatures, resolving the config-typing question (generic map
  vs. `json.RawMessage` vs. something else) with a concrete recommendation and rationale.
- Specify a `providers/all` blank-import bundle package as the closest idiomatic equivalent to true
  auto-discovery Go can offer.
- Specify how Charon's adapter layer collapses onto the registry, and flag (not silently resolve)
  the allowlist-vs-full-discovery design tension this creates for Charon's own UI/API surface.
- Specify three documentation deliverables the user explicitly asked for:
  `go_notify_yourself/ARCHITECTURE.md`, a new README section, and
  `go_notify_yourself/docs/INTEGRATION.md`.
- Sequence all of this as additional commits on the existing `feature/notifications-engine-extraction`
  branch/PR — not a new branch, not a new PR.

### 1.3 Non-goals

- No URL-scheme parsing/dispatch convention (Apprise's signature feature, e.g. `discord://...`) —
  still explicitly out of scope, unchanged from the original spec's §8.
- No new provider packages are added or scaffolded under this spec (no Web Push implementation) —
  the registry is the enabling mechanism; the user's Web Push provider is a separate future piece
  of work that becomes trivial once this lands.
- No frontend schema-driven form generator is designed here — flagged as an open question (§3.6),
  not solved.
- No changes to `docs/plans/current_spec.md` (unrelated active work, per the task constraint).
- No `go.mod` edits, no branch creation, no code written — this is planning only.

---

## 2. Research Findings

### 2.1 Current state of `go_notify_yourself` (read in full, not assumed)

| File | Finding |
|---|---|
| `sender.go` | `Sender` interface: `Send(ctx context.Context, msg Message) error`. Every provider package returns a type implementing this. No registry, no factory type exists anywhere in the repo. |
| `message.go` | `Message{Title, Body, EventType, Timestamp, Data map[string]any}` + `Normalized()`. Stable, generic, provider-agnostic — no changes needed for the registry. |
| `providers/discord/discord.go` | `Config{WebhookURL, Template, CustomTemplate}`; `New(cfg Config, w *transport.Wrapper) *Client`. All fields are plain strings — trivially JSON-serializable. |
| `providers/webhook/webhook.go` | Same shape: `Config{URL, Template, CustomTemplate}`, `New(cfg, w)`. Also exposes `RenderPreview` and re-exports `MinimalTemplate`/`DetailedTemplate` consts. |
| `providers/slack`, `gotify`, `pushover`, `ntfy`, `telegram` | Same `New(cfg, w *transport.Wrapper) *Client` shape (confirmed by reading all five `Config` structs — see table in §3.1). All fields are plain strings (`URL`, `Token`, `UserKey`, `APIToken`, `BotToken`, `ChatID`, `BaseURL`, `Template`, `CustomTemplate`). **All eight non-email packages are pure-data, JSON-serializable configs.** |
| `providers/email/email.go` | **The odd one out, confirmed by reading it in full.** `Config{Recipients []string, SubjectPrefix string, TemplateName func(msg notify.Message) string, Renderer TemplateRenderer, Mailer Mailer}`. `Mailer` and `TemplateRenderer` are **behavioral interfaces** the host application implements (e.g. Charon's `notify_email_adapter.go` wraps `MailServiceInterface`/`RenderNotificationEmail`); `TemplateName` is a **Go closure**. None of these three fields can round-trip through JSON. `New(cfg Config) *Client` — no `*transport.Wrapper` parameter at all (email never dials HTTP). This asymmetry is the central design constraint for the registry (see §3.2). |
| `transport/wrapper.go` | `*transport.Wrapper` is constructed once per host application (`transport.NewWrapper(opts...)`) and injected into every HTTP-based provider's `New`. It is itself DI-configured (`ClientFactory`, `URLValidator`, `RetryPolicy`) — the registry must not bypass or duplicate that construction, only thread the already-built `*Wrapper` through. |
| Module root | `package notify` at `github.com/Wikid82/go_notify_yourself` (no subpackage) — `Register`/`New` naturally belong here, alongside `Message`/`Sender`, per Go convention (mirrors `sql.Register`/`sql.Open` living in `database/sql` itself, not a subpackage). |

### 2.2 Prior art: `database/sql` and `image` self-registration idiom

- `database/sql`: `sql.Register(name string, driver driver.Driver)` — panics on duplicate
  registration or nil driver, called from each driver package's `init()`. `sql.Open(driverName,
  dataSourceName string)` looks up the registered driver and returns a `*DB`. The `dataSourceName`
  is an opaque string each driver parses itself (e.g. a DSN) — `database/sql` has zero opinion on
  its shape. This is the closest fit: **the registry core has no opinion on config shape**, it's a
  keyed lookup over a factory function; shape-parsing is entirely the registered package's problem.
- `image`: `image.RegisterFormat(name, magic string, decode DecodeFunc, decodeConfig DecodeConfigFunc)`
  — decoders self-register via blank import (`_ "image/png"`), and `image.Decode` sniffs the magic
  bytes to pick a decoder. No generic "config" concept at all — irrelevant to the config-typing
  question here, but reinforces the blank-import-for-discovery pattern (§3.3).
- Both prior-art examples confirm two things this spec adopts: (1) `Register` panics on
  double-registration (a programmer error caught at `init()` time, not a runtime error path), and
  (2) the registry package itself never imports the packages that register into it — the
  dependency arrow points inward (provider → registry), never outward, which is exactly what keeps
  `providers/all` (§3.3) as the only place that "knows about" every provider.

### 2.3 Charon-side current coupling (re-confirmed on this branch, post-extraction)

`backend/internal/services/notify_provider_adapter.go` (already on this branch, commit `11d3489c`)
has `buildNotifySender`, a `switch strings.ToLower(...) provider.Type { case "discord": ...
discord.New(discord.Config{WebhookURL: provider.URL, ...}, w) ... }` — one case per provider type,
each mapping specific GORM columns to that provider's specific `Config` field names (documented
in-file, non-uniformly: `discord.WebhookURL <- provider.URL`, but `slack.WebhookURL <-
provider.Token`; `pushover.UserKey <- provider.URL`, `pushover.APIToken <- provider.Token`;
`telegram.BotToken <- provider.Token`, `telegram.ChatID <- provider.URL`). **This field-mapping
non-uniformity is itself a research finding that constrains the registry design** — see §3.4.

`backend/internal/services/notification_service.go` (lines 126–166, read directly, not
paraphrased):

```go
func supportsJSONTemplates(providerType string) bool {
	switch strings.ToLower(providerType) {
	case "webhook", "discord", "gotify", "slack", "generic", "telegram", "pushover", "ntfy":
		return true
	default:
		return false
	}
}

func isSupportedNotificationProviderType(providerType string) bool {
	switch strings.ToLower(strings.TrimSpace(providerType)) {
	case "discord", "email", "gotify", "webhook", "telegram", "slack", "pushover", "ntfy":
		return true
	default:
		return false
	}
}

func (s *NotificationService) isDispatchEnabled(providerType string) bool {
	switch strings.ToLower(strings.TrimSpace(providerType)) {
	case "discord":
		return true
	case "email":
		return s.getFeatureFlagValue(FlagEmailServiceEnabled, false)
	case "gotify":
		return s.getFeatureFlagValue(FlagGotifyServiceEnabled, true)
	// ... one flag-gated case per provider type ...
	default:
		return false
	}
}
```

Three independent hardcoded allowlists, one hardcoded factory switch. All four must be reconciled
with any registry that makes providers "discoverable" — see §3.5 for why they should **not** all
collapse onto the registry automatically.

### 2.4 GORM model — `backend/internal/models/notification_provider.go` (read in full)

```go
type NotificationProvider struct {
	ID             string
	Name           string
	Type           string     // provider type discriminator
	URL            string     // reused across types: webhook URL, server URL, user key, chat ID...
	Token          string     `json:"-"` // reused across types: API token, webhook token, bot token...
	Config         string     // JSON payload template for custom webhooks
	ServiceConfig  string     `json:"service_config,omitempty" gorm:"type:text"` // JSON blob for typed service config
	Template       string     `gorm:"default:minimal"`
	// ... Notify* preference bools, migration/audit fields ...
}
```

**Finding, confirmed by repo-wide grep**: `ServiceConfig` is declared but has **zero read or write
call sites anywhere in `backend/internal/`** outside its own struct tag. It is dead/reserved
schema — a column that exists but nothing populates or reads it yet. This is directly relevant:
the model already has a `text` column earmarked (by its doc comment, "JSON blob for typed service
config") for exactly the kind of flexible, not-known-in-advance provider config a new provider type
(Web Push) would need, without a migration. See §3.6 for the recommendation.

The existing `URL`/`Token` pair is a **fixed two-slot** scheme — every provider type today has been
squeezed into "one URL-ish string, one secret-ish string," which is why the field-mapping table in
§2.3 is non-uniform (each type has its own private convention for what `URL` vs `Token` *means*).
A provider needing more than two config values (e.g. Web Push's VAPID public/private keypair +
subscription endpoint — three values, none of which is a natural fit for "URL" or "Token") cannot
be expressed in the current two-slot scheme at all. `ServiceConfig` is the natural place for this,
but is not wired up.

### 2.5 Frontend (confirmed by file search, not deep-read — out of scope per task framing)

`frontend/src/pages/Notifications.tsx` (758 LOC, per the original extraction spec's §2.8) is the
only frontend file referencing `NotificationProvider`/provider type. No separate
`NotificationProviderForm.tsx` component exists — the per-type config fields are rendered inline in
this one page, keyed off `provider.type` (pattern confirmed by grep; the file was not read in full
under this task's scope, per the instruction to flag rather than solve the frontend question).
**Finding**: today, adding a provider type to the UI requires editing this file directly, the same
way adding one to the backend requires editing `notify_provider_adapter.go` — there is no
schema-driven form rendering today. See §3.6 for how this is flagged as an open question.

---

## 3. Technical Specifications

### 3.1 Provider `Config` field inventory (all eight packages, read directly)

| Package | `Config` fields | Serializable? |
|---|---|---|
| `discord` | `WebhookURL, Template, CustomTemplate` | Yes — all strings |
| `slack` | `WebhookURL, Template, CustomTemplate` | Yes — all strings |
| `gotify` | `URL, Token, Template, CustomTemplate` | Yes — all strings |
| `pushover` | `UserKey, APIToken, BaseURL, Template, CustomTemplate` | Yes — all strings |
| `ntfy` | `URL, Token, Template, CustomTemplate` | Yes — all strings |
| `telegram` | `BotToken, ChatID, BaseURL, Template, CustomTemplate` | Yes — all strings |
| `webhook` | `URL, Template, CustomTemplate` | Yes — all strings |
| `email` | `Recipients []string, SubjectPrefix string, TemplateName func(Message) string, Renderer TemplateRenderer, Mailer Mailer` | **No** — `TemplateName`/`Renderer`/`Mailer` are Go closures/interfaces, not data |

This table is the deciding evidence for §3.2's config-typing recommendation: seven of eight
packages have a pure-data `Config`; email's does not, and never can while it keeps the
DI-seam design principle (module never dials SMTP/renders HTML itself) that the original
extraction spec deliberately chose.

### 3.2 Config-typing decision: `map[string]any`, not `json.RawMessage`

**Recommendation: the registry boundary uses `map[string]any`, not `json.RawMessage`/typed
generics.**

Rationale, directly from §3.1's evidence:

- `json.RawMessage` (or any JSON-shaped boundary) works cleanly for the seven HTTP providers — each
  factory would `json.Unmarshal(raw, &Config{})`. It **cannot** work for `email.Config` at all:
  `Mailer`, `TemplateRenderer`, and `TemplateName` are behavioral Go values a host application
  constructs at startup (e.g. Charon's `notify_email_adapter.go` wrapping `MailServiceInterface`),
  not data that arrives over a wire. There is no JSON representation of "call this Go function."
  Forcing email through a JSON boundary would mean either (a) breaking email out of the registry
  entirely as a special case — undermining the "one path for every provider" goal that motivated
  this work — or (b) inventing a side-channel to inject non-JSON deps alongside the JSON blob,
  which is just `map[string]any` with extra steps.
- `map[string]any` handles both cases uniformly: for the seven HTTP providers, callers put plain
  Go strings under well-known keys; for email, the caller puts the actual `Mailer`/`TemplateRenderer`
  values and `TemplateName` closure directly into the map under their own well-known keys. Each
  factory type-asserts what it expects and returns a config error (not a panic) on a
  missing/wrong-typed key.
- **Tradeoff, stated plainly**: this loses compile-time type safety at the registry boundary — a
  caller can put a `string` under a key a factory expects to be `*transport.Wrapper` and won't
  find out until `New()` returns an error at runtime. This is an accepted, explicit cost. The typed
  per-provider `Config` structs (`discord.Config`, `email.Config`, etc.) **remain the primary,
  fully type-safe public API** — a host application that wants compile-time safety and doesn't need
  runtime discovery calls `discord.New(discord.Config{...}, wrapper)` directly, exactly as it does
  today. The registry is an **additive convenience/discovery layer**, not a replacement for the
  typed constructors — this must be explicit in the module's docs (§3.7) so users don't think
  `notify.New` is the only or preferred way to construct a `Sender`.
- A pure `json.RawMessage`-only design was considered and rejected specifically because it would
  make email a permanent second-class citizen of the registry (excluded, or requiring an awkward
  parallel non-JSON registration path) — inconsistent with the goal of one uniform discovery
  mechanism across all provider types, present and future (Web Push, unlike email, is
  HTTP-transport-based like the other seven, but the registry design must not special-case around
  today's provider mix).

### 3.3 `Register`/`New` API (module root, `package notify`)

```go
// factory.go (new file, module root)

package notify

import (
	"fmt"
	"sort"
	"strings"
	"sync"
)

// Factory constructs a Sender from a generic configuration map. Each
// provider package's factory type-asserts the keys/types it expects out of
// config and returns a descriptive error for anything missing or
// wrong-typed — Factory implementations must never panic on bad input from
// a caller (panicking is reserved for Register's own misuse-by-programmer
// checks, per the database/sql convention — see below).
//
// Well-known convention (documented per-package in each provider's doc
// comment and in ARCHITECTURE.md, §3.7): HTTP-based providers expect a
// "transport" key holding the shared *transport.Wrapper; provider-specific
// Config fields are expected under their lowercase snake_case field name
// (e.g. discord's WebhookURL -> config["webhook_url"]). This module makes
// no attempt to enforce these conventions structurally — see the open
// question in §5 risk 2 on whether a stricter typed-key mechanism is worth
// the added complexity.
type Factory func(config map[string]any) (Sender, error)

var (
	registryMu sync.RWMutex
	registry   = map[string]Factory{}
)

// Register makes a provider Factory available under name (case-insensitive;
// stored lowercased). Intended to be called from a provider package's
// init(), mirroring database/sql.Register and image.RegisterFormat.
//
// Register panics if factory is nil or if name is already registered —
// exactly like sql.Register — because a duplicate/nil registration is
// always a programmer error discoverable at package-init time (e.g. two
// packages both claiming "webhook"), never a legitimate runtime condition
// a caller should have to handle.
func Register(name string, factory Factory) {
	if factory == nil {
		panic("notify: Register called with nil Factory for " + name)
	}
	key := strings.ToLower(strings.TrimSpace(name))
	if key == "" {
		panic("notify: Register called with empty name")
	}
	registryMu.Lock()
	defer registryMu.Unlock()
	if _, exists := registry[key]; exists {
		panic("notify: Register called twice for provider " + key)
	}
	registry[key] = factory
}

// New looks up the Factory registered under name (case-insensitive) and
// invokes it with config. Returns an error — never panics — if name is not
// registered or if the factory itself returns an error (e.g. a missing
// required config key).
func New(name string, config map[string]any) (Sender, error) {
	key := strings.ToLower(strings.TrimSpace(name))
	registryMu.RLock()
	factory, ok := registry[key]
	registryMu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("notify: no provider registered for type %q (registered types: %s)",
			name, strings.Join(RegisteredTypes(), ", "))
	}
	return factory(config)
}

// RegisteredTypes returns the sorted list of currently registered provider
// type names. Useful for a host application that wants to validate a
// config value or populate a UI dropdown against exactly what's compiled
// in, without hardcoding its own list (see §3.5's discussion of Charon's
// allowlist-vs-discovery tradeoff).
func RegisteredTypes() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}
```

**Design notes:**

- `sync.RWMutex`-guarded map: registrations happen at `init()` time (effectively single-threaded,
  before `main` runs), but `New`/`RegisteredTypes` may be called concurrently from request-handling
  goroutines in a host application (Charon's HTTP handlers), so read-locking those paths is cheap
  insurance, not overengineering.
- `Register` panicking on misuse mirrors `database/sql` exactly and is the right call here for the
  same reason: a duplicate provider name is a build-time-discoverable defect (two packages both
  registering `"webhook"`), and panicking during `init()` fails the program immediately and loudly
  rather than silently shadowing one provider with another.
- `New` returning an error (never panicking) for an *unregistered* name is the opposite case
  deliberately: "provider type X isn't registered" is a **runtime** condition (a host forgot to
  blank-import the package, or a config file references a typo'd/future type) that calling code
  must be able to handle gracefully — e.g. Charon surfacing "unsupported provider type" back to its
  API caller instead of crashing the process.
- Placed at the module root (`factory.go`, `package notify`) alongside `message.go`/`sender.go`
  rather than a new subpackage — avoids an import-cycle problem symmetric to the one in §3.2: if
  `Register`/`New` lived in a subpackage that provider packages needed to import, and the root
  `notify` package needed to reference that subpackage's types, the two would have to share
  identifiers anyway. Keeping the registry in the root package (which every provider package
  already imports, per `discord.go`'s `notify "github.com/Wikid82/go_notify_yourself"` import) is
  the only placement with zero new import edges.

### 3.4 Provider package migration — each package's `init()`

Every one of the eight existing packages adds a small registration file/block. Two representative
examples (HTTP-based and email), the remaining six follow the same shape as `discord`:

```go
// providers/discord/register.go (new file)
package discord

import (
	"fmt"

	notify "github.com/Wikid82/go_notify_yourself"
	"github.com/Wikid82/go_notify_yourself/transport"
)

func init() {
	notify.Register("discord", func(config map[string]any) (notify.Sender, error) {
		w, ok := config["transport"].(*transport.Wrapper)
		if !ok || w == nil {
			return nil, fmt.Errorf(`discord: config["transport"] must be a non-nil *transport.Wrapper`)
		}
		cfg := Config{
			Template:       stringField(config, "template"),
			CustomTemplate: stringField(config, "custom_template"),
			WebhookURL:     stringField(config, "webhook_url"),
		}
		return New(cfg, w), nil
	})
}
```

```go
// providers/email/register.go (new file)
package email

import (
	"fmt"

	notify "github.com/Wikid82/go_notify_yourself"
)

func init() {
	notify.Register("email", func(config map[string]any) (notify.Sender, error) {
		mailer, ok := config["mailer"].(Mailer)
		if !ok || mailer == nil {
			return nil, fmt.Errorf(`email: config["mailer"] must be a non-nil Mailer`)
		}
		cfg := Config{
			Mailer:        mailer,
			SubjectPrefix: stringField(config, "subject_prefix"),
			Recipients:    stringSliceField(config, "recipients"),
		}
		if r, ok := config["renderer"].(TemplateRenderer); ok {
			cfg.Renderer = r
		}
		if tn, ok := config["template_name"].(func(notify.Message) string); ok {
			cfg.TemplateName = tn
		}
		return New(cfg), nil
	})
}
```

(`stringField`/`stringSliceField` are small unexported helpers in a shared internal location,
e.g. `providers/internal/regconfig`, to avoid duplicating the same type-assert-with-default
boilerplate across eight `register.go` files — DRY per this repo's own conventions.)

**What breaks / changes in the public API as a result — this is a breaking change:**

- No existing exported identifier changes signature (`Config`, `New(cfg, w)`, `Send` are untouched)
  — this is purely additive at the Go-API level.
- **However**, adding `init()`-time registration means: (a) importing any provider package now has
  a side effect (registering into the global `notify` registry) it didn't have before — a host
  application that imports `providers/discord` for its types but never calls `notify.New` will
  still pay the (tiny) registration cost and take a global-map write at startup. This is normal for
  the self-registration pattern (identical to every `database/sql` driver package) but is a
  behavior change worth calling out in the changelog, not silently shipping. (b) Two packages
  registering the same name in the same binary now **panics at init time** — not a concern for the
  eight packages in this repo (names are fixed and distinct), but is a new failure mode a future
  third-party provider package could trigger if it collided with a built-in name; document this in
  `ARCHITECTURE.md` (§3.7) as a naming-collision hazard to design around.
  - Since this changes both the module's behavior (global registration side effect) and shape
    (new root-package `Register`/`New`/`RegisteredTypes` exported API, new `map[string]any`
    convention every provider must honor), this ships as **`v0.2.0`**, not a patch release —
    consistent with semver: no existing exported signature breaks, but new, previously-absent
    runtime behavior (init-time global mutation, panic-on-collision) is a real enough shift to
    warrant a minor bump at minimum. (The module is pre-1.0 per its `v0.1.0` tag, so strictly
    semver would even permit this as a `v0.1.x`-breaking change without a major bump — recommend
    `v0.2.0` as the clearer, more conventional signal to the one current consumer.)

### 3.5 `providers/all` bundle package

```go
// providers/all/all.go (new file)

// Package all blank-imports every provider package shipped in this module,
// registering all of them into the root notify package's registry as a
// side effect. Import this package for its side effects only —
//
//	import _ "github.com/Wikid82/go_notify_yourself/providers/all"
//
// — when you want every built-in provider type available to notify.New
// without importing each provider package individually. This is the
// closest equivalent Go offers to true runtime auto-discovery: Go has no
// mechanism to discover and load packages that were not compiled into the
// binary, so *some* single import is unavoidable — providers/all exists so
// that import is exactly one line, added once, rather than one line per
// provider that must be kept in sync by hand as the provider list grows.
package all

import (
	_ "github.com/Wikid82/go_notify_yourself/providers/discord"
	_ "github.com/Wikid82/go_notify_yourself/providers/email"
	_ "github.com/Wikid82/go_notify_yourself/providers/gotify"
	_ "github.com/Wikid82/go_notify_yourself/providers/ntfy"
	_ "github.com/Wikid82/go_notify_yourself/providers/pushover"
	_ "github.com/Wikid82/go_notify_yourself/providers/slack"
	_ "github.com/Wikid82/go_notify_yourself/providers/telegram"
	_ "github.com/Wikid82/go_notify_yourself/providers/webhook"
)
```

**Tradeoff, stated explicitly (per the task's ask):** importing `providers/all` means a host
binary always links every provider package the module ships, even ones the host never configures
or wants to expose (e.g. Charon linking a future `providers/webpush` it has no UI for yet) — larger
binary, and every provider's transitive dependencies come along. The alternative — hand-picking
individual `import _ "…/providers/discord"` lines — keeps binary size and blast radius under the
host's control but means a host must edit its own import list every time it wants a newly-added
provider, which is exactly the manual step the user is trying to eliminate for *their* project (not
necessarily for Charon — see §3.6's recommendation, which treats these as two different answers for
two different consumers). `providers/all` is offered as **one available choice**, not a mandate —
`go_notify_yourself` ships it for consumers who want zero-touch discovery; a consumer that wants
tighter control simply doesn't import it and hand-picks instead. Both remain equally supported by
the registry design; neither requires a different `Register`/`New` API.

### 3.6 Charon-side changes

**3.6.1 `notify_provider_adapter.go`'s switch collapses to a `notify.New` call.**

`buildNotifySender` (§2.3) becomes, in shape:

```go
func buildNotifySender(provider models.NotificationProvider, w *transport.Wrapper, mailer email.Mailer) (notify.Sender, error) {
	tmpl, customTemplate := resolveTemplateFields(provider)
	config := providerConfigMap(provider, w, mailer, tmpl, customTemplate) // per-type field mapping — see below
	return notify.New(provider.Type, config)
}
```

**Important finding, not glossed over**: collapsing the *constructor dispatch* onto `notify.New`
does **not** eliminate the per-type field-mapping logic documented in §2.3 — Charon's GORM schema
reuses `URL`/`Token` with different *meanings* per provider type (discord's `URL` is a webhook URL;
pushover's `URL` is a user key; telegram's `URL` is a chat ID). Something in Charon must still
decide "for type X, config key `webhook_url` <- `provider.URL`; for type Y, config key `user_key`
<- `provider.URL`." The registry removes the "which Go constructor do I call" branch; it does
**not** remove the "which GORM column means what for this type" branch, because that mapping is a
Charon persistence-schema fact, not something `go_notify_yourself` can know. `providerConfigMap`
would likely still contain a `switch provider.Type` internally — smaller in scope (pure data
mapping, no `discord.New(...)`/`slack.New(...)` calls, no per-package imports needed in this file
at all once every field name is passed as a map key) but not eliminated outright. This should be
stated plainly to the user rather than oversold as "the switch statement disappears entirely" — it
shrinks and stops needing to import every provider package, but a mapping table remains until/unless
§2.4's `ServiceConfig` idea (below) is adopted for new types.

**3.6.2 The three provider-type allowlists — open design question, not silently resolved.**

`isSupportedNotificationProviderType`, `supportsJSONTemplates`, and `isDispatchEnabled` currently
hardcode Charon's own opinion about which provider types it exposes in its UI/API — independent of
what the module happens to support. Two options:

- **Option A — keep Charon's allowlist hardcoded (recommended).** Charon's REST API/UI continues to
  explicitly enumerate the provider types it supports, same as today. If Charon later imports
  `providers/all` (or a future `providers/webpush`), that provider becomes *constructible* via
  `notify.New` but Charon's API still rejects it at `isSupportedNotificationProviderType` until a
  human deliberately adds it to the allowlist (and, typically, builds UI for it). **Rationale**: a
  host application importing a module for its providers is not the same decision as committing to
  support that provider in its own product surface — Charon may want `providers/all` linked for
  convenience (§3.5) while still curating what it exposes to users, exactly the same reasoning the
  original extraction spec used for feature-flag gating (`isDispatchEnabled`, which is Charon
  policy, not engine behavior, and stays Charon's regardless of this change).
- **Option B — query the registry directly (`notify.RegisteredTypes()`)** for full auto-discovery:
  Charon's allowlist becomes computed, not hardcoded — any provider the compiled binary happens to
  have linked (via whatever `providers/*` imports exist) is automatically exposed through Charon's
  API. **Rationale for considering it**: this is the literal "add a provider, it just works,
  automatically, after a version bump" behavior the user described wanting. **Rationale against**:
  it removes Charon's ability to link a provider package (for binary-size/testing/future-readiness
  reasons) without also immediately exposing it to end users — e.g. Web Push landing in
  `go_notify_yourself` before Charon has built any UI for it would suddenly appear as a "supported"
  type in Charon's API with no way to configure it meaningfully from the UI, a broken half-feature
  exposed by version bump alone.

**Recommendation: Option A**, with `notify.RegisteredTypes()` used only as an internal
*consistency check* (e.g. a startup assertion or unit test asserting Charon's hardcoded allowlist
is a subset of `notify.RegisteredTypes()`, catching the case where Charon claims to support a type
the linked module build doesn't actually have registered) — not as the live source of truth for
what the API accepts. **This is exactly the kind of decision the task called out as needing the
user's confirmation before implementation** — flagged here, not silently picked.

**3.6.3 `providers/all` vs. hand-picked imports in Charon — recommendation.**

Recommend Charon **hand-picks** individual provider imports
(`import _ "github.com/Wikid82/go_notify_yourself/providers/discord"`, one per supported type, in
`notify_provider_adapter.go` or a dedicated `notify_providers_import.go`) rather than importing
`providers/all`. This is the direct consequence of the Option A recommendation in §3.6.2: if
Charon's allowlist is the actual gate on what's exposed (not the registry), then importing
`providers/all` only adds binary size and a larger transitive dependency surface for providers
Charon's allowlist will reject anyway — there's no discovery benefit to importing more than
Charon's own allowlist currently names. `providers/all` remains the right choice for a *different*
kind of consumer — one that wants Option-B-style full auto-discovery — which is exactly the
scenario the user described for their *other* project, not necessarily for Charon.

**3.6.4 GORM model — `ServiceConfig` is the identified extension point, not yet wired up.**

Confirmed by re-reading `notification_provider.go` (§2.4): the two-slot `URL`/`Token` scheme is
sufficient for all eight of today's provider types (each needs at most two secrets/identifiers,
already squeezed in with per-type reinterpretation), but is **not** sufficient for a
not-yet-known-in-advance provider needing three or more distinct config values with no natural
"URL" or "Token" framing (Web Push's VAPID keypair + endpoint being the concrete example driving
this whole request). No schema change is strictly required to land the registry itself (today's
eight providers keep working unchanged), but **a future provider needing >2 config values will
need `ServiceConfig` wired up** — recommend, as a follow-up (not blocking this registry work): treat
`ServiceConfig` as a JSON-encoded `map[string]string` (or `map[string]any`) column, decoded and
merged into the `config` map passed to `notify.New` alongside the existing `URL`/`Token`-derived
keys, giving future provider types an escape hatch without another migration. Flagged as a
recommendation for a **later** spec/commit, not built now — out of scope for "make existing
providers discoverable," in scope for "the next provider that doesn't fit two slots."

**3.6.5 Frontend — flagged, not solved.**

`Notifications.tsx` renders per-type config fields inline, keyed off `provider.type` (§2.5) — the
same "hardcoded per type" pattern this spec removes from the Go backend. Making the *frontend* form
genuinely schema-driven (e.g. deriving which fields to render from something the backend exposes,
rather than a hardcoded TSX conditional) is a materially different, larger piece of design work
(a form-schema wire format, versioning of that schema, backward compat for already-saved configs)
that this spec does **not** attempt to solve. Flagging per the task's explicit instruction: the
user asked specifically about the engine being "drop-in ready" at the Go level; whether that
implies auto-generated UI is a separate question for the user to weigh in on before any frontend
work is planned.

### 3.7 Documentation deliverables

**3.7.1 `/projects/go_notify_yourself/ARCHITECTURE.md` (new file) — required sections**

Must be precise enough for a human contributor *or a coding agent* to add a provider without
guessing. Required sections, in order:

1. **Module overview** — one paragraph: what this module is, the four-layer shape (`Message`/
   `Sender` at root, `transport` for SSRF-safe HTTP, `providers/*` per-service implementations,
   the registry tying them together), link to README for the quick-start.
2. **The `Sender` contract** — the interface, its one method, the behavioral expectations already
   documented in `sender.go`'s doc comment (respect `ctx`, wrap errors, never panic) restated here
   with a "why" (uniform treatment by host applications fanning a message out to N destinations).
3. **Adding a new provider — step-by-step**, the core of the document:
   - File/package layout convention: `providers/<name>/<name>.go` (main `Config`/`New`/`Send`),
     `providers/<name>/<name>_test.go`, optionally `providers/<name>/register.go` for the `init()`
     registration (kept separate from the main file — mirrors this spec's §3.4 examples — so the
     core type/constructor logic isn't cluttered by registry plumbing).
   - The `Config` struct convention: exported struct, `Template`/`CustomTemplate` fields present
     *only* if the provider is HTTP/JSON-payload-based (email is the documented exception —
     explain why, pointing at §3.1/§3.2's reasoning).
   - The `New(cfg Config, w *transport.Wrapper) *Client` signature convention for HTTP-based
     providers (email's `New(cfg Config) *Client` documented as the one exception, with the reason
     — no HTTP transport).
   - `var _ notify.Sender = (*Client)(nil)` compile-time interface assertion — required in every
     provider package, per the existing pattern (confirmed present in `discord.go`, `email.go`).
   - **The `Register`/factory pattern**: exact template for `register.go`'s `init()`, the
     `map[string]any` key-naming convention (lowercase snake_case of the `Config` field name; the
     `"transport"` key reserved for `*transport.Wrapper` on HTTP-based providers), and the
     requirement that factories return errors (never panic) for bad/missing keys.
   - **Adding to `providers/all`**: the one-line blank-import addition required in
     `providers/all/all.go`, and why this step is easy to forget (it's not enforced by the
     compiler — a new provider that registers itself but isn't added to `providers/all` still
     works for direct `import _ "…/providers/newone"` consumers but silently isn't part of the
     "one import gets everything" bundle). Recommend a CI check (e.g. a small test in
     `providers/all` asserting `len(notify.RegisteredTypes()) == <N>` with a comment requiring the
     constant be bumped alongside any new provider, catching an accidental omission) — specify this
     as a required addition, not just a suggestion, since it's the one step with no compiler
     safety net.
   - **Naming conventions**: provider package names are lowercase, no underscores, matching the
     `Register` key exactly (e.g. package `webpush`, `Register("webpush", ...)`) — collisions
     panic at `init()` per §3.3, so this section should state that explicitly as the reason naming
     matters.
   - **Test expectations**, mirroring the existing eight providers' patterns (to be confirmed
     against actual test files in `providers/discord/discord_test.go` etc. by whoever implements
     this — this spec specifies *that* the doc must describe them, not their exact content, since
     verifying each existing test file's structure is implementation-time work): table-driven
     `Send` tests against a fake `transport.Wrapper` (via injected `ClientFactory`), config
     validation error-path tests, ≥85% coverage per package (per this repo's own `CLAUDE.md`
     coverage bar), and — new for this spec — a registration test asserting `notify.New("<name>",
     validConfig)` succeeds and returns a `Sender`, plus at least one test asserting a
     missing-required-key config produces an error, not a panic.
   - **Config validation/error handling conventions**: factories validate structurally (right type
     present under each expected key) and return `fmt.Errorf`-wrapped errors describing exactly
     which key/type was expected; the underlying `Config`-consuming `New`/`Send` still perform their
     own semantic validation (e.g. Discord's webhook host allowlist) exactly as they do today —
     the registry layer adds a validation step in front of, not instead of, existing validation.
4. **The registry internals** — brief: where `Register`/`New`/`RegisteredTypes` live (module root,
   `factory.go`), the panic-on-duplicate/nil-factory contract, the `RWMutex` concurrency note, and
   an explicit statement that this is an *additive convenience layer* — the typed `New(cfg, w)`
   constructors remain fully supported and are not deprecated by the registry's existence.
5. **Versioning note** — this document itself should state that adding a provider (a new package)
   is a `feat:`/minor-version change under this module's semver policy (unchanged provider API
   surface for existing providers = non-breaking), while changing `Register`/`New`/`Factory`'s
   signature is a breaking/major-version change — giving a contributor or agent the semver
   judgment call up front instead of leaving them to guess at PR time.

**3.7.2 New README section (`/projects/go_notify_yourself/README.md`)**

Brief — a new `## Provider registry` section (placed after the existing "Provider packages" table,
before "Transport"), roughly 150–250 words: what `notify.Register`/`notify.New` are, the one-line
`providers/all` blank-import quick-start, a pointer ("see `ARCHITECTURE.md` for how to add a new
provider, and `docs/INTEGRATION.md` for a full integration walkthrough"), and one sentence stating
the typed constructors remain the recommended path when a caller doesn't need runtime discovery
(consistent with §3.2's "additive, not a replacement" framing, so the README doesn't contradict
`ARCHITECTURE.md`).

**3.7.3 `/projects/go_notify_yourself/docs/INTEGRATION.md` (new file) — required structure (Five Ws
and One H)**

Written for someone integrating the module into their **own, unrelated** project — not Charon.
Required sections, in this order, each specified so a future writer doesn't have to invent
structure:

1. **Who this is for** — the target reader: a Go developer building a self-hosted or small-team
   application (the README's own framing: "most projects... end up re-implementing the same things
   badly") who needs to fire outbound alerts to chat/push/email destinations and doesn't want to
   hand-roll SSRF-safe HTTP dispatch, retries, or per-service payload quirks. Explicitly *not* for:
   someone needing an Apprise-style URL-scheme dispatcher today (§1.3/non-goals), or someone who
   needs inbound/two-way messaging (this module is send-only).
2. **What it does / doesn't do** — a two-column or two-list breakdown. Does: SSRF-safe outbound
   HTTP with retry/backoff (`transport.Wrapper`), a uniform `Sender` interface across eight
   built-in provider types, JSON payload templating with a shared `text/template` + `toJSON`
   engine, a self-registering factory/discovery layer (this spec's addition). Doesn't: own any
   database, config file format, or HTTP framework; provide inbound webhook receiving; provide
   scheduling/queueing/retry-after-process-restart (retries are in-process, in-request only);
   provide a URL-scheme dispatch convention (yet — link to §8 of the extraction spec / this
   module's own long-term-direction note for the Apprise aspiration).
3. **When to reach for it vs. rolling your own** — a short decision checklist: reach for it if you
   need ≥2 of {Discord, Slack, Gotify, Pushover, Ntfy, Telegram, generic webhook, email} dispatch,
   want retry/backoff and SSRF hardening without writing it yourself, and are fine supplying your
   own HTTP client factory / SSRF policy / SMTP mailer via the DI seams. Roll your own if you need
   exactly one destination type with a very custom payload shape *and* don't want any of the shared
   machinery, or need transport types this module doesn't have (SMS, inbound webhooks, message
   queues).
4. **Where it fits in a typical app's architecture** — a short architecture sketch (text or a
   simple diagram) showing: your app's business logic layer maps a domain event into a
   `notify.Message`; a startup-time wiring step builds one shared `*transport.Wrapper` (with your
   `ClientFactory`/`URLValidator` seams) and, if using email, your `Mailer`/`TemplateRenderer`
   implementations; either call typed constructors directly or use `notify.New(type, config)` via
   `providers/all`; the resulting `Sender`(s) are invoked from wherever your app currently fires
   alerts (a notification service, an error handler, a monitoring loop). Explicitly locate this as
   "a dispatch layer your service layer calls into," not a framework that owns your request
   lifecycle.
5. **Why it's built this way** — the DI-seam philosophy: the module has zero DB/framework/HTTP-server
   knowledge by design (restate the non-negotiable rule from this module's own `CLAUDE.md`); every
   environment-specific concern is a constructor-injected interface; this is what makes the module
   equally usable from Charon (GORM/Gin), a CLI tool, or a serverless function, and is why the
   registry (this spec) uses `map[string]any` rather than forcing a specific config-file format or
   framework binding (§3.2's rationale, restated briefly for this different audience).
6. **How to integrate — end-to-end walkthrough**: a real, copy-pasteable sequence —
   `go get github.com/Wikid82/go_notify_yourself@v0.2.0`; blank-import `providers/all` (or
   hand-pick, with the same tradeoff explanation as §3.5, written for this general audience rather
   than Charon-specifically); construct a `transport.Wrapper`; construct one or more senders via
   `notify.New` with a worked `map[string]any` example for at least one HTTP provider and email;
   dispatch a message; handle/log a `Send` error; a short "testing your integration" pointer to the
   README's existing "Testing your own integration" section (don't duplicate it, link to it).

### 3.8 Public API surface summary (new/changed in this spec)

| Symbol | Package | Status |
|---|---|---|
| `type Factory func(config map[string]any) (Sender, error)` | `notify` (root) | New |
| `func Register(name string, factory Factory)` | `notify` (root) | New |
| `func New(name string, config map[string]any) (Sender, error)` | `notify` (root) | New |
| `func RegisteredTypes() []string` | `notify` (root) | New |
| `package all` (blank-import bundle) | `providers/all` | New package |
| `func init()` in each of 8 provider packages | `providers/*` | New (registration side effect) |
| `Config`, `New(cfg, w)`, `Send` in each of 8 provider packages | `providers/*` | **Unchanged** — no breaking signature change |

### 3.9 Database schema changes

None required to land the registry itself. §3.6.4 identifies `ServiceConfig` (already present,
currently unused) as the extension point a *future* provider needing >2 config values would need
wired up — explicitly deferred, not part of this spec's implementation.

### 3.10 Error handling / edge cases

- **Unregistered type at `notify.New`**: returns a wrapped error listing currently-registered
  types (§3.3) — Charon surfaces this as its existing "unsupported provider type" API error, no new
  Charon-side error path needed for this case since `buildNotifySender`'s `default:` branch already
  returns an error today.
- **Missing/wrong-typed required config key inside a factory** (e.g. `config["transport"]` absent
  or not a `*transport.Wrapper`): each factory returns a descriptive `fmt.Errorf` — never panics
  — per §3.4's examples. This is a *caller* bug (Charon's adapter built the map wrong), distinct
  from the *programmer* bugs `Register` panics on.
- **Duplicate registration** (two packages registering the same name in one binary): panics at
  `init()`, crashing the process immediately at startup — by design (§3.3), surfaces the
  misconfiguration as loudly and early as possible rather than silently picking one.
- **`providers/all` omission**: a new provider that registers correctly but isn't added to
  `providers/all` doesn't break anything for a consumer hand-picking imports; it silently isn't
  part of the "everything" bundle for `providers/all` consumers. Mitigated by the CI count-check
  recommended in §3.7.1, not by any runtime mechanism (Go cannot detect "a package that exists in
  this repo but wasn't imported").
- **Charon allowlist drift from the registry** (§3.6.2 Option A risk): Charon's hardcoded allowlist
  could, over time, diverge from what's actually registered in the linked module build (claiming
  support for a type whose provider package isn't imported, or vice versa). Mitigated by the
  recommended consistency check (`isSupportedNotificationProviderType`'s set ⊆
  `notify.RegisteredTypes()`), run as a Charon unit test, not a runtime assertion (avoids a
  startup-time panic risk in production for what's fundamentally a build-configuration mismatch
  best caught in CI).

---

## 4. Implementation Plan (for the future execution session — not executed here)

### Phase 1: `go_notify_yourself` — registry core + provider migrations (module repo)

- Write `factory.go` at module root (`Factory`, `Register`, `New`, `RegisteredTypes`, §3.3) +
  `factory_test.go` (duplicate-registration panic, nil-factory panic, unregistered-name error,
  concurrent-access test).
- Add `register.go` to each of the eight provider packages (§3.4), plus a shared
  `providers/internal/regconfig` helper package for the `stringField`/`stringSliceField`
  boilerplate (DRY — avoids 8x duplication).
- Add `providers/all/all.go` (§3.5) + a test asserting `len(notify.RegisteredTypes()) == 8` (the
  CI safety net from §3.7.1).
- `go build ./...`, `go vet ./...`, `staticcheck ./...`, `go test ./...` (≥85% coverage on new
  code) all green.

### Phase 2: `go_notify_yourself` — documentation (module repo)

- Write `ARCHITECTURE.md` per §3.7.1's required sections.
- Add the README section per §3.7.2.
- Write `docs/INTEGRATION.md` per §3.7.3's required sections.
- Update `CHANGELOG.md`'s `[Unreleased]` section with the registry addition, explicitly noting the
  `v0.2.0` semver bump and its rationale (§3.4).
- Tag `v0.2.0` once Phase 1 + Phase 2 are both merged in this repo.

### Phase 3: Charon — adapter simplification (this repo, same branch)

- Bump `backend/go.mod` to `github.com/Wikid82/go_notify_yourself v0.2.0`.
- Add hand-picked provider imports (§3.6.3) — one `import _ "…/providers/<type>"` line per
  Charon-supported type, in a new small file (e.g. `notify_providers_import.go`) rather than
  scattered across existing files, so the "what's linked" list stays in one obvious place.
- Rewrite `buildNotifySender` to call `notify.New(provider.Type, config)` (§3.6.1), with
  `providerConfigMap` doing the remaining per-type field-name mapping (still present, smaller
  scope — no direct `providers/*` package function calls left in this file, only the blank imports
  above plus `transport`/`email` types needed to build the config map's values).
- Add the recommended consistency-check unit test (§3.6.2/§3.10):
  `isSupportedNotificationProviderType`'s set is a subset of `notify.RegisteredTypes()` given
  Charon's actual linked imports.
- Update `ARCHITECTURE.md` per this repo's own mandatory-update rule.

### Phase 4: Integration and testing (Charon repo)

- Rewrite/extend `notify_provider_adapter_test.go` to cover the new `notify.New`-based dispatch
  path — assert the same per-type behaviors the existing switch-based tests already assert (§2.3),
  now via the registry, plus new tests for the "unregistered/unsupported type" and
  "missing-transport-in-config" error paths.
- Full backend suite + `scripts/go-test-coverage.sh` (≥85%) green.
- No frontend changes in this phase (§3.6.5 is explicitly deferred) — no Playwright changes needed
  since no user-observable behavior changes (same provider types, same UI, same API responses).

### Phase 5: Documentation and deployment (both repos)

- Confirm `go.sum` supply-chain scan clean for the bumped dependency.
- No `docs/features/notifications.md` change required (product-facing behavior is unchanged;
  optional credit-the-module line only, per the original extraction spec's precedent).
- Release Charon per normal Conventional Commits flow on this existing PR/branch.

---

## 5. Acceptance Criteria

- [ ] `go_notify_yourself`: `Register`/`New`/`RegisteredTypes` exist at module root, `go test
      ./...` and `staticcheck` pass with zero findings, tagged `v0.2.0`.
- [ ] All eight provider packages self-register via `init()`; `providers/all` blank-imports all
      eight and its count-check test passes.
- [ ] `notify.New("discord", map[string]any{"transport": w, "webhook_url": "...", "template":
      "minimal"})` returns a working `*discord.Client` equivalent to `discord.New(discord.Config{...},
      w)` — behavioral parity between the typed constructor and the registry path is asserted by
      test, not assumed.
- [ ] `ARCHITECTURE.md`, README's new section, and `docs/INTEGRATION.md` exist and cover every
      required subsection listed in §3.7.
- [ ] Charon's `notify_provider_adapter.go`'s `buildNotifySender` calls `notify.New`; the file's
      import list no longer imports the eight `providers/*` packages directly for constructor calls
      (blank-imports for registration live in a separate, clearly-named file).
- [ ] Charon's three provider-type allowlists (`isSupportedNotificationProviderType`,
      `supportsJSONTemplates`, `isDispatchEnabled`) remain hardcoded per §3.6.2's Option A
      recommendation, with a new test asserting they're a subset of `notify.RegisteredTypes()`.
- [ ] No change to `docs/plans/current_spec.md`.
- [ ] No behavior change observable from Charon's frontend or REST API — same provider types
      supported, same request/response shapes, same dispatch semantics (retry/backoff, SSRF policy
      unchanged from the already-merged extraction).
- [ ] Charon's backend coverage gate (`scripts/go-test-coverage.sh`, min 85%) still passes.
- [ ] `ARCHITECTURE.md` (Charon's) updated per the mandatory rule.

---

## 6. Commit Slicing Strategy

**Decision**: this work is **not** a new feature/new PR — it folds into the existing, still-open
PR #1253 on `feature/notifications-engine-extraction`, exactly as the task specified. Per this
repo's own commit-slicing convention, it spans two repositories (module repo, then Charon repo),
each with its own ordered commit sequence, landing on the *same* existing branch/PR shape the
original extraction already established (two-repo-two-PR-shaped-as-one-feature, not a new branch).

### `go_notify_yourself` repo — additional commits (same repo, no PR needed there per its own
### CLAUDE.md's "direct TDD, no multi-agent roster" note — but still ordered/bisectable commits)

1. **Commit 1** — Registry core: `factory.go` + `factory_test.go` (§3.3, Phase 1). Gate: `go build
   ./...`, `go vet ./...`, `go test ./...` green, new code ≥85% coverage.
2. **Commit 2** — Shared registration helper: `providers/internal/regconfig` (`stringField`/
   `stringSliceField` + tests). Gate: same pattern. Dependency: Commit 1.
3. **Commit 3** — Discord + Slack registration (`register.go` in each, §3.4). Gate: `go test
   ./providers/discord/... ./providers/slack/...` green, registry round-trip test passes.
   Dependency: Commits 1–2.
4. **Commit 4** — Gotify + Pushover + Ntfy registration. Gate: same pattern. Dependency: 1–2.
5. **Commit 5** — Telegram + Webhook registration. Gate: same pattern. Dependency: 1–2.
6. **Commit 6** — Email registration (highest design-risk piece, per the original extraction
   spec's precedent for treating email specially — the non-serializable `Mailer`/`TemplateRenderer`
   type-assertion path deserves dedicated review attention). Gate: `go test ./providers/email/...`
   green, explicit test for missing-`Mailer` error path. Dependency: 1–2.
7. **Commit 7** — `providers/all` bundle + count-check test (§3.5). Gate: `go test
   ./providers/all/...` asserts `len(notify.RegisteredTypes()) == 8`. Dependency: Commits 3–6 (all
   eight must be registered first).
8. **Commit 8** — Documentation: `ARCHITECTURE.md`, README section, `docs/INTEGRATION.md` (§3.7).
   Gate: manual review against §3.7's required-sections checklist (no automated gate for doc
   content). Dependency: Commits 1–7 (docs describe the finished API).
9. **Commit 9** — `CHANGELOG.md` update + tag `v0.2.0`. Gate: `goreleaser release --snapshot`
   dry-run succeeds. Dependency: Commit 8.

Rollback: any commit 3–7 can be reverted independently (each provider's registration is additive
and isolated); Commit 1 is the sole hard dependency for everything after it — reverting it reverts
the whole registry addition cleanly since nothing else in the module depended on a registry
existing before this work.

### Charon repo — additional commits on the existing `feature/notifications-engine-extraction`
### branch (same PR #1253)

1. **Commit 1** — Dependency bump: `go.mod`/`go.sum` to `go_notify_yourself v0.2.0`. Gate: `go
   build ./...`. Dependency: module repo's `v0.2.0` tag must exist first.
2. **Commit 2** — Registration imports: new `notify_providers_import.go` with hand-picked
   blank-imports (§3.6.3) for Charon's eight currently-supported types. No behavior change yet
   (nothing calls `notify.New` still). Gate: `go build ./...`.
3. **Commit 3** — Adapter rewrite: `buildNotifySender` calls `notify.New` (§3.6.1); `resolveTemplateFields`
   and the per-type `providerConfigMap` helper carry forward from the existing switch, adapted to
   produce `map[string]any` instead of typed `Config` structs. Gate: `notify_provider_adapter_test.go`
   rewritten and green, asserting identical behavior to the pre-registry switch for all eight types
   (parity, not just "compiles"). Dependency: Commits 1–2.
4. **Commit 4** — Allowlist consistency test: new test asserting
   `isSupportedNotificationProviderType`'s set ⊆ `notify.RegisteredTypes()` (§3.6.2/§3.10). Gate:
   test passes given Commit 2's imports. Dependency: Commit 3.
5. **Commit 5** — Cleanup: remove now-unused direct `providers/*` package imports from
   `notify_provider_adapter.go` if any remain post-rewrite; confirm no unused imports via
   staticcheck. Gate: `go build ./...`, staticcheck clean. Dependency: Commit 3.
6. **Commit 6** — Docs: `ARCHITECTURE.md` update noting the registry-based dispatch (supersedes
   the switch-statement description added by the original extraction's Commit 12). Gate: manual
   review. Dependency: Commit 3.
7. **Commit 7** — Coverage/hardening: re-run `scripts/go-test-coverage.sh`, fix any regression.
   Gate: full Definition of Done per CLAUDE.md. Dependency: Commits 1–6.

Rollback for the Charon side: since PR #1253 is still open (not yet merged to `development`), any
of Commits 1–7 can be reverted or the whole set squashed out of the branch before merge with zero
production impact — there is no live consumer of the registry-based path yet. Post-merge, the
dependency bump (Commit 1) is the natural revert boundary: rolling back to the pre-`v0.2.0` pin and
restoring the switch-based `buildNotifySender` (Commits 2–5) is a clean, self-contained revert since
no schema/API/GORM change accompanies this work.

Contingency: if Phase 3/4 discovers the `map[string]any` boundary is meaningfully harder to keep in
sync with GORM's `URL`/`Token` reinterpretation-per-type scheme than expected (§3.6.1's caveat),
stop before Charon's Commit 3 and re-scope `providerConfigMap` as its own small design pass — this
does not block the module-repo commits (1–9 above), which are self-contained and useful to the
user's other project regardless of exactly how Charon's adapter ends up shaped.

---

## 7. Risks / Open Questions (for the user to confirm before implementation)

1. **Allowlist vs. full discovery (§3.6.2) — needs explicit user sign-off.** Recommended: Option A
   (Charon keeps its own hardcoded allowlist; `notify.RegisteredTypes()` used only as a CI/test
   consistency check, not a live API gate). Alternative: Option B (Charon's API directly reflects
   `notify.RegisteredTypes()`, true zero-touch exposure). This is a genuine product decision about
   how eagerly Charon should surface engine capabilities it hasn't built UI/support for yet — not a
   technical question this spec can resolve unilaterally.
2. **Config-typing at the registry boundary (§3.2) — `map[string]any`, recommended, with a stated
   type-safety cost.** Confirm the user is comfortable losing compile-time safety at this one
   boundary (the typed constructors remain fully available and are the recommended path when
   runtime discovery isn't needed) in exchange for a single uniform mechanism that also
   accommodates email's non-serializable `Mailer`/`TemplateRenderer` dependencies (§3.1's evidence
   for why `json.RawMessage` alone doesn't work).
3. **`providers/all` vs. hand-picked imports, per consumer (§3.5/§3.6.3).** Recommended: Charon
   hand-picks (tighter control, consistent with the Option A allowlist decision); the user's other
   project (the original motivating use case) more plausibly wants `providers/all` for genuine
   zero-touch discovery. Confirm this isn't meant to be a uniform policy across both consumers —
   the spec currently treats it as a per-consumer choice, which the user should explicitly agree
   is the right framing rather than assuming one answer fits both projects.
4. **`ServiceConfig` wiring (§3.6.4) is flagged, not built.** A future provider needing >2 config
   values (Web Push being the concrete driver) will need this GORM field actually wired up — this
   spec identifies it as the extension point and recommends a shape (JSON-encoded
   `map[string]string`, merged into the registry's `config` map) but treats implementing it as
   follow-up work, not part of this commit sequence. Confirm this sequencing is acceptable — i.e.,
   that the user wants the registry to land first, generically, with the schema question addressed
   only once Web Push (or another >2-field provider) is actually being built against
   `go_notify_yourself`.
5. **Frontend schema-driven forms (§3.6.5) — explicitly out of scope for this pass.** Confirm the
   user agrees "drop-in ready" for this round means the Go module/Charon backend only, and that
   frontend auto-generation is a separate, later decision (potentially never, if Charon's product
   philosophy prefers hand-built, polished per-provider forms over generic ones — a design opinion
   worth surfacing explicitly rather than assuming).
6. **Naming-collision hazard for third-party providers (§3.4).** `Register` panics on a duplicate
   name. This is fine for the eight built-in providers (fixed, non-colliding names) but is a new
   failure mode once truly third-party provider packages exist (per the user's stated long-term
   goal of others contributing providers) — e.g. two unrelated third-party packages both choosing
   `Register("webpush", ...)`. No global namespace-reservation mechanism is proposed here (would be
   premature — there's no third-party provider ecosystem yet); flagging so the user is aware this
   is a real, if distant, consequence of the self-registration pattern, same as it is for
   `database/sql` drivers today.
