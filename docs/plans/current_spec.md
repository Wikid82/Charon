# "What's New" Changelog Popup — Implementation Plan

Status: Ready for review
Date: 2026-07-28
Design source of truth: `docs/superpowers/specs/2026-07-28-whats-new-changelog-design.md` (Approved — all product decisions final; this plan does not re-litigate scope, only makes it buildable)

When Charon's Docker integration connects successfully to a Docker daemon that currently has zero running containers, the "Add Proxy Host" page crashes with `Uncaught TypeError: can't access property "map", P is null` at `frontend/src/components/ProxyHostForm.tsx:781`. Root cause is two-sided:

This plan turns the approved "What's New" changelog design into a concrete,
file-by-file implementation. The feature adds a per-user, dismissible modal
that surfaces new features/fixes since the user's last visit, sourced from
conventional-commit messages generated at release build time and embedded
in the binary — zero runtime network calls, zero new external dependencies.

**Objectives**

- Add two columns to `User` (`last_seen_version`, `changelog_opt_out`) and
  seed new users correctly.
- Add a `backend/internal/changelog` package that reads an embedded JSON
  file and answers "what's new since version X" and "give me everything."
- Add four authenticated API routes under `/api/v1/changelog`.
- Add `scripts/generate-changelog.sh` and wire it into
  `.github/workflows/release-goreleaser.yml`.
- Add `WhatsNewModal.tsx`, mount it post-auth, and wire an
  Appearance Settings toggle + revisit link.
- Cover everything with backend unit tests, frontend Vitest tests, and
  Playwright E2E specs, following the repo's TDD/Definition-of-Done
  conventions.

**Non-goals** (inherited from the design doc, not re-decided here):
role-filtered content, rich text/markdown entries, editorial rewriting of
commit messages.

## 2. Research Findings

### 2.1 `UpdateService` injectable-version pattern (to mirror)

`backend/internal/services/update_service.go`:

```go
type UpdateService struct {
    currentVersion string
    ...
}

func NewUpdateService() *UpdateService {
    return &UpdateService{currentVersion: version.Version, ...}
}

// SetCurrentVersion sets the current version for testing.
func (s *UpdateService) SetCurrentVersion(v string) { s.currentVersion = v }
```

The new `changelog.Service` will follow this exact shape: a private
`currentVersion` field defaulted from `version.Version` in the constructor,
with a `SetCurrentVersion(v string)` test seam. This is also the seam the
`CHARON_CHANGELOG_VERSION` dev-override plugs into at wiring time in
`routes.go` (not inside the service itself — see §3.3).

### 2.2 `User` model

`backend/internal/models/user.go` — `User` struct fields end at
`SessionVersion uint` (line 55) before the invite-system block. GORM
auto-migrates on every boot via `db.AutoMigrate(...)` in
`internal/api/routes/routes.go` (`RegisterWithDeps`, lines 100–143) — new
struct fields become new columns automatically, no manual migration file
needed, matching repo convention ("Migrations: When adding models, update
`internal/models` AND `internal/api/routes/routes.go` (AutoMigrate)" — here
`models.User` is already in the AutoMigrate list, so no routes.go change is
needed for the migration itself, only for the new route registrations).

### 2.3 `version.Version` sentinel

`backend/internal/version/version.go`:

```go
var Version = "dev" // ldflags-injected at release build time
```

`.goreleaser.yaml` injects it via `-X .../version.Version={{.Version}}` in
the `builds.linux.ldflags` block. Local `go run` and non-release Docker
builds never set it, so it stays `"dev"` — this is the exact signal the
design doc's "Unversioned/dev builds" edge case keys off.

### 2.4 `internal/config` env-var convention

`backend/internal/config/config.go` uses `getEnvAny(fallback, keys...)`,
e.g. `Environment: getEnvAny("development", "CHARON_ENV", "CPM_ENV")`.
`CHARON_CHANGELOG_VERSION` will follow the same helper, read directly in
`routes.go` at changelog-service construction time (not added to the
`Config` struct — it's a narrow dev/test-only override, not a general
runtime setting; keeping it out of `Config` avoids widening the struct for
a one-off QA knob, consistent with how no other single-purpose test-only
env var lives on `Config` today).

### 2.5 Route registration conventions

`internal/api/routes/routes.go` `RegisterWithDeps`: all authenticated,
non-admin-gated, non-passthrough-blocked routes that any logged-in user
(admin or user role) should reach live directly under the `protected`
group (e.g. `protected.GET("/user/profile", ...)`), **not** under
`management` (which is blocked for passthrough users via
`middleware.RequireManagementAccess()`, lines 332–334). The four changelog
routes belong under `protected` for the same reason auth/profile routes
do — passthrough users still get a session and could theoretically log in,
though in practice they're redirected client-side to `/passthrough`; the
backend route being reachable is still the correct default absent an
explicit spec instruction to block passthrough (the design spec doesn't
mention role-gating, consistent with "Non-Goals: per-role/permission
filtered content").

### 2.6 GoReleaser / release workflow

`.github/workflows/release-goreleaser.yml` has **no separate "build" step**
— the design doc says to insert "immediately before `goreleaser build`",
but this repo's actual GoReleaser invocation is a single combined
`release --clean` step (`Run GoReleaser`, lines 77–85) which builds *and*
publishes in one action invocation. There is no earlier point at which
`goreleaser build` runs standalone. **Concrete insertion point**: a new
step named `Generate Changelog Data` inserted immediately before the
`Run GoReleaser` step (after `Build Frontend` / `Install Cross-Compilation
Tools`), so `backend/internal/changelog/data/changelog.json` is refreshed
in the checkout **before** GoReleaser's embedded `go build` compiles it in.
This satisfies the design doc's intent ("immediately before…build runs, so
go:embed picks up the freshly generated file") even though the workflow's
step name differs from the doc's phrasing.

`.goreleaser.yaml` already has a `changelog:` section (lines ~74+) — this
is GoReleaser's **own** built-in changelog generator, used to populate the
**GitHub Release notes body** (a separate, pre-existing feature, unrelated
to the in-app "What's New" modal). No conflict: `scripts/generate-changelog.sh`
writes a different file (`backend/internal/changelog/data/changelog.json`)
consumed by `go:embed`, while GoReleaser's `changelog:` block independently
formats the GitHub Release description from the same git log. Both read
the same conventional-commit history but serve different UIs. This plan
does not touch the `.goreleaser.yaml` `changelog:` section.

`VERSION.md` confirms the reason CI never commits generated data back to
`main`: the tag-derived release process explicitly avoids CI pushing to
`main` (§"CI Tag-based Releases (recommended)": "This avoids automatic
commits to `main`"). This validates the design's "regenerate from git
history on every build, never commit back" approach.

### 2.7 Frontend conventions

- **API client** (`frontend/src/api/notifications.ts`): plain async
  functions wrapping the shared `client` from `./client.ts`
  (`axios.create({ baseURL: '/api/v1', withCredentials: true, ... })`),
  exported interfaces for response shapes, JSDoc on each exported function.
- **Hook** (`frontend/src/hooks/useNotifications.ts` /
  `useSecurityNotificationSettings`): `useQuery`/`useMutation` from
  `@tanstack/react-query`, query keys as string arrays, `toast` from
  `../utils/toast` for error/success feedback, `queryClient.invalidateQueries`
  on mutation success.
- **Settings page** (`frontend/src/pages/AppearanceSettings.tsx`): sections
  wrapped in `<Card>/<CardHeader>/<CardTitle>/<CardDescription>/<CardContent>`
  from `../components/ui/Card`, `useTranslation()` for all copy, mutations
  via `useMutation` + `queryClient.invalidateQueries({ queryKey: ['settings'] })`.
- **Layout/mount point** (`frontend/src/components/Layout.tsx`): renders
  `<FeedbackWidget />` as a sibling at the end of the root `<div>` (line
  416) — a standalone, self-fetching, post-auth-mounted widget with no
  props. `WhatsNewModal` mounts the same way, one line below
  `<FeedbackWidget />`.
- **App shell** (`frontend/src/App.tsx`): `Layout` wraps `<Outlet />` only
  for the authenticated `/` route tree (line 67–75), confirming
  `Layout.tsx` — not `App.tsx` — is the correct mount point for a
  post-auth-only component.

### 2.8 Auth/user-object gap (discovered, not in the design doc's file list)

The design doc's Settings section says the toggle is "bound to
`!user.changelog_opt_out`" but the actual frontend `User` type consumed
app-wide is narrower than the DB model:

- `frontend/src/context/AuthContextValue.ts`:
  `export interface User { user_id: number; role: ...; name?: string; email?: string; }`
- `frontend/src/context/AuthContext.tsx` line 13:
  `const response = await client.get<User>('/auth/me');` — the `/auth/me`
  response is cast directly to this `User` type and stored as
  `AuthContext`'s `user`.
- Backend `AuthHandler.Me` (`backend/internal/api/handlers/auth_handler.go`,
  lines 299–326) currently returns only
  `{ user_id, role, name, email }` — no `changelog_opt_out`.

**Consequence**: to satisfy the design doc's literal binding
(`!user.changelog_opt_out`) via the existing app-wide `useAuth().user`
object, `Me()` must also return `changelog_opt_out`, and `User` in
`AuthContextValue.ts` must gain the field. This is a small, necessary
extension of an existing endpoint (not a new design decision) — flagged
here explicitly per the "Context First" rule so implementation doesn't
silently invent a second, redundant fetch for one boolean. See §3.6 for
the exact diff.

### 2.9 User-creation call sites for `LastSeenVersion` seeding

Three places construct a new `models.User{}` with `INSERT`-worthy defaults
in `backend/internal/api/handlers/user_handler.go`:

| Handler | Line | Design doc mentions it? |
|---|---|---|
| `Setup` (initial admin) | 121–190 (struct at 142) | Not explicitly, but "new user seeding" applies identically — see note below |
| `CreateUser` (admin-created user) | 376–464 (struct at 413) | Yes — "admin-created user" |
| `InviteUser` (creates row with `InviteStatus: "pending"`) | 484+ (struct at 531) | No — row created here, activated later |
| `AcceptInvite` (activates the pending row) | 1144–1198 | Yes — "invite acceptance" |

**Implementation decision** (flagged, not a scope change): seed
`LastSeenVersion` at all three *effective* creation points —
`Setup`, `CreateUser`, and `AcceptInvite` — not `InviteUser`. Rationale:
`InviteUser`'s row is disabled/pending and the user has never logged in;
seeding at `AcceptInvite` time (when the row transitions to `enabled: true`
and the user is about to get a real session) is the correct point, exactly
as the design doc says. `Setup` is included because it is, structurally,
identical to "new user creation" (zero users existed before it ran) — the
design doc's rationale for seeding ("new users never see historical
entries on their first login") applies to the initial admin with equal
force, and leaving it out would be an inconsistency: the first admin would
fall into the "pre-existing user, empty string, treated as behind
everything" bucket purely because their creation path predates the two the
doc named, which reads as unintentional. This is called out explicitly for
supervisor sign-off, not silently decided.

### 2.10 `golang.org/x/mod/semver` availability

`backend/go.sum` already lists `golang.org/x/mod v0.37.0` as a transitive
dependency (via another module), confirmed via `grep`. It is not currently
imported directly by any `.go` file in `backend/`. Adding
`import "golang.org/x/mod/semver"` to the new `internal/changelog` package
requires `go mod tidy` to promote it from indirect to direct in `go.mod`
(a `go.sum`/`go.mod` diff, not a `go get` of a new module) — matches the
design doc's "no new package added" claim.

### 2.11 Playwright/E2E conventions

- Config: `frontend/e2e/playwright.config.ts` — `testDir: './tests'`
  resolves to the **project-root** `tests/` directory (not
  `frontend/e2e/`), `baseURL` from `CHARON_BASE_URL` env var.
- Structure: `tests/settings/*.spec.ts` (e.g. `account-settings.spec.ts`,
  `user-lifecycle.spec.ts`) is the existing home for settings-adjacent
  flows — `tests/settings/whats-new-changelog.spec.ts` is the natural new
  file.
- Auth: tests import `{ test, expect } from './fixtures/test'` and rely on
  a pre-generated Playwright `storageState` (see `STORAGE_STATE` from
  `./constants`, and `tests/fixtures/auth-fixtures.ts`) — no `test.fixme`
  usage currently exists anywhere in the repo, so this feature introduces
  the pattern (per CLAUDE.md's suggested commit sequence) for the first
  time; keep the specs simple and self-contained so the pattern is easy to
  copy for future features.
- No `.spec.ts` files currently live in `frontend/e2e/` itself (only
  `playwright.config.ts`); all specs live in the root `tests/` tree.

### 2.12 `.gitignore` / `.dockerignore` / `codecov.yml` / `Dockerfile` review

See §7 for full analysis. Summary: `.gitignore`, `codecov.yml`, and the
`Dockerfile` need **no changes**. `.dockerignore` **does need a change** —
its existing bare, unanchored `data/` pattern (line 96) matches a
directory of that name at any depth (gitignore/dockerignore semantics),
which silently strips `backend/internal/changelog/data/` — including the
committed placeholder `changelog.json` — from every Docker build context
before `COPY backend/ ./` runs, breaking `go:embed data/changelog.json`
compilation in `docker-build.yml`, `e2e-tests-split.yml`, and any other
workflow that builds this repo's `Dockerfile`. GoReleaser release builds
are unaffected (they don't use this `Dockerfile`), which is why this
wasn't caught by reasoning about the release path alone — it's a
Docker-build-context problem, not a release-generation problem. Fixed via
an explicit negation, added to Commit 2 (§6) alongside the rest of the
embed scaffolding, with a real build-context verification step in that
commit's validation gate (not just visual inspection of the negation
pattern — BuildKit's negate-after-broad-exclude behavior is a known
footgun and must be proven, not assumed).

## 3. Technical Specifications

### 3.1 Data Model

`backend/internal/models/user.go` — insert after `SessionVersion uint`
(line 55), before the `// Invite system fields` comment block:

```go
// Changelog / "What's New" tracking
LastSeenVersion string `json:"last_seen_version" gorm:"default:''"`
ChangelogOptOut bool   `json:"changelog_opt_out" gorm:"default:false"`
```

No new migration file — picked up by the existing `db.AutoMigrate(&models.User{}, ...)`
call already in `routes.go` line 109.

### 3.2 `backend/internal/changelog` package (new)

**File: `backend/internal/changelog/changelog.go`**

```go
// Package changelog provides build-time-generated "What's New" data,
// embedded into the binary at compile time — no runtime file I/O or
// network calls.
package changelog

import (
    _ "embed"
    "encoding/json"
    "sort"

    "golang.org/x/mod/semver"
)

//go:embed data/changelog.json
var rawData []byte

// Entry describes one released version's categorized, novice-friendly
// changelog content.
type Entry struct {
    Version  string   `json:"version"`
    Date     string   `json:"date"`
    Features []string `json:"features"`
    Fixes    []string `json:"fixes"`
    Other    []string `json:"other"`
}

var allEntries []Entry // parsed once at package init

func init() {
    // Malformed/missing embedded data degrades to "no changelog" rather
    // than panicking at boot — the placeholder `[]` always parses cleanly,
    // and a bad CI-generated file should never crash the server.
    _ = json.Unmarshal(rawData, &allEntries)
}

// Service resolves the "current version" used for since-comparisons via
// an injectable seam, mirroring services.UpdateService.SetCurrentVersion.
type Service struct {
    currentVersion string
}

// NewService constructs a Service defaulted to the running build's
// version.Version.
func NewService(currentVersion string) *Service {
    return &Service{currentVersion: currentVersion}
}

// SetCurrentVersion overrides the effective current version — used by
// tests and by the CHARON_CHANGELOG_VERSION dev-only override wired in
// routes.go.
func (s *Service) SetCurrentVersion(v string) { s.currentVersion = v }

// CurrentVersion returns the effective current version.
func (s *Service) CurrentVersion() string { return s.currentVersion }

// IsDevBuild reports whether the effective current version is the
// unversioned "dev" sentinel (version.Version's default).
func (s *Service) IsDevBuild() bool { return s.currentVersion == "dev" }

// GetEntriesSince returns entries newer than lastSeen, newest-first.
// Empty/invalid lastSeen is treated as "behind everything" (all entries
// returned) per the design doc's pre-existing-user edge case.
func (s *Service) GetEntriesSince(lastSeen string) []Entry {
    var result []Entry
    for _, e := range allEntries {
        if lastSeen == "" || semver.Compare("v"+e.Version, "v"+lastSeen) > 0 {
            result = append(result, e)
        }
    }
    sortNewestFirst(result)
    return result
}

// GetAllEntries returns the full changelog history, newest-first.
func (s *Service) GetAllEntries() []Entry {
    result := make([]Entry, len(allEntries))
    copy(result, allEntries)
    sortNewestFirst(result)
    return result
}

func sortNewestFirst(entries []Entry) {
    sort.Slice(entries, func(i, j int) bool {
        return semver.Compare("v"+entries[i].Version, "v"+entries[j].Version) > 0
    })
}
```

**File: `backend/internal/changelog/data/changelog.json`** (new, committed
placeholder):

```json
[]
```

**File: `backend/internal/changelog/changelog_test.go`** (new, TDD red
first) — table-driven cases per design doc §Testing:
- `TestGetEntriesSince_ReturnsNewerOnly` — fixture with v1.0.0, v1.1.0,
  v2.0.0; `lastSeen="1.0.0"` → returns v1.1.0, v2.0.0, newest-first.
- `TestGetEntriesSince_EmptyLastSeen_ReturnsAll` — pre-existing-user case.
- `TestGetEntriesSince_EqualVersion_ReturnsEmpty` — boundary: `lastSeen`
  equal to newest entry → `[]`.
- `TestGetAllEntries_ReturnsEverythingNewestFirst`.
- `TestIsDevBuild_TrueForDevSentinel` / `_FalseForRealVersion`.
- `TestNewService_DefaultsCurrentVersion`.
- `TestSetCurrentVersion_Overrides`.

Since `allEntries` is a package-level var parsed from the **real** embedded
placeholder (`[]`) during `go test`, these tests must inject their own
fixture data rather than relying on `data/changelog.json` — add an
unexported test-only seam:

```go
// setEntriesForTest replaces the package-level parsed data; test-only.
func setEntriesForTest(t *testing.T, entries []Entry) {
    t.Helper()
    original := allEntries
    allEntries = entries
    t.Cleanup(func() { allEntries = original })
}
```

This keeps `go:embed` real (no build-tag test double for the embed itself)
while making tests deterministic and independent of the placeholder's
actual (empty) content — same spirit as `UpdateService.SetCurrentVersion`.

### 3.3 API Routes

All four routes are **read/write on the authenticated user's own row**,
registered under `protected` (not `management`) in
`internal/api/routes/routes.go`, immediately after the existing
`protected.POST("/user/api-key", ...)` line (330):

```go
// Changelog / "What's New" — self-service, all authenticated roles
changelogService := changelog.NewService(version.Version)
if cfg.Environment != "production" {
    if v := os.Getenv("CHARON_CHANGELOG_VERSION"); v != "" {
        changelogService.SetCurrentVersion(v)
    }
}
changelogHandler := handlers.NewChangelogHandler(db, changelogService)
protected.GET("/changelog/status", changelogHandler.Status)
protected.GET("/changelog/all", changelogHandler.All)
protected.POST("/changelog/ack", changelogHandler.Ack)
protected.POST("/changelog/opt-in", changelogHandler.OptIn)
```

(`"os"` and `"github.com/Wikid82/charon/backend/internal/changelog"` added
to `routes.go` imports; `version` package also newly imported there.)

**File: `backend/internal/api/handlers/changelog_handler.go`** (new):

```go
package handlers

import (
    "net/http"

    "github.com/gin-gonic/gin"
    "gorm.io/gorm"

    "github.com/Wikid82/charon/backend/internal/changelog"
    "github.com/Wikid82/charon/backend/internal/models"
)

type ChangelogHandler struct {
    db  *gorm.DB
    svc *changelog.Service
}

func NewChangelogHandler(db *gorm.DB, svc *changelog.Service) *ChangelogHandler {
    return &ChangelogHandler{db: db, svc: svc}
}

// GET /api/v1/changelog/status
func (h *ChangelogHandler) Status(c *gin.Context) {
    if rejectPassthrough(c, "view the changelog") {
        return
    }
    userID, ok := requireUserID(c) // shared helper, see note below
    if !ok { return }

    var user models.User
    if err := h.db.First(&user, userID).Error; err != nil {
        c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
        return
    }

    if h.svc.IsDevBuild() || user.ChangelogOptOut {
        c.JSON(http.StatusOK, gin.H{"show_changelog": false, "versions": []changelog.Entry{}})
        return
    }

    entries := h.svc.GetEntriesSince(user.LastSeenVersion)
    c.JSON(http.StatusOK, gin.H{
        "show_changelog": len(entries) > 0,
        "versions":       entries,
    })
}

// GET /api/v1/changelog/all
func (h *ChangelogHandler) All(c *gin.Context) {
    if rejectPassthrough(c, "view the changelog") {
        return
    }
    c.JSON(http.StatusOK, gin.H{"versions": h.svc.GetAllEntries()})
}

type AckRequest struct {
    Action string `json:"action" binding:"required,oneof=dismiss_temporary dismiss_permanent"`
    OptOut bool   `json:"opt_out"`
}

// POST /api/v1/changelog/ack
func (h *ChangelogHandler) Ack(c *gin.Context) {
    if rejectPassthrough(c, "update changelog preferences") {
        return
    }
    userID, ok := requireUserID(c)
    if !ok { return }

    var req AckRequest
    if err := c.ShouldBindJSON(&req); err != nil {
        c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
        return
    }

    updates := map[string]any{}
    if req.Action == "dismiss_permanent" {
        updates["last_seen_version"] = h.svc.CurrentVersion()
    }
    if req.OptOut {
        updates["changelog_opt_out"] = true
    }
    if len(updates) == 0 {
        c.JSON(http.StatusOK, gin.H{"message": "No change"})
        return
    }
    result := h.db.Model(&models.User{}).Where("id = ?", userID).Updates(updates)
    if result.Error != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update changelog state"})
        return
    }
    if result.RowsAffected == 0 {
        c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
        return
    }
    c.JSON(http.StatusOK, gin.H{"message": "Acknowledged"})
}

// POST /api/v1/changelog/opt-in
func (h *ChangelogHandler) OptIn(c *gin.Context) {
    if rejectPassthrough(c, "update changelog preferences") {
        return
    }
    userID, ok := requireUserID(c)
    if !ok { return }

    result := h.db.Model(&models.User{}).Where("id = ?", userID).
        Update("changelog_opt_out", false)
    if result.Error != nil {
        c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to opt in"})
        return
    }
    if result.RowsAffected == 0 {
        c.JSON(http.StatusNotFound, gin.H{"error": "User not found"})
        return
    }
    c.JSON(http.StatusOK, gin.H{"message": "Opted in"})
}
```

**Passthrough guard (required, not optional)**: `rejectPassthrough(c *gin.Context, action string) bool`
already exists in `backend/internal/api/handlers/user_handler.go` (lines
202–210: `if c.GetString("role") == string(models.RolePassthrough) { ...403...; return true }`)
and is already the established pattern for exactly this kind of
self-service-but-not-for-passthrough route — it gates `GetProfile`,
`UpdateProfile`, and `RegenerateAPIKey` today. Because
`changelog_handler.go` lives in the same `handlers` package, it calls the
unexported `rejectPassthrough` directly with no new export needed. All
four `ChangelogHandler` methods call it first, before `requireUserID`,
matching the order used by the existing call sites. `RolePassthrough`
users have no legitimate reason to see or acknowledge the dashboard
changelog (they're redirected client-side to `/passthrough` and never see
`Layout.tsx` or `AppearanceSettings.tsx`), so this is a correctness fix,
not a defense-in-depth nicety — without it, a passthrough user hitting the
route directly would silently succeed against §2.5's originally-assumed
"any authenticated role" default, which was wrong: the repo's actual
convention for `protected`-group self-service routes is
route-group-placement **plus** an in-handler passthrough check, not
route-group-placement alone.

`requireUserID` — check whether an equivalent shared helper already exists
in `handlers` package (several handlers repeat the
`c.Get("userID"); userID, ok := ...(uint)` pattern verbatim, e.g.
`AuthHandler.Me` lines 300–310, `UserHandler.GetProfile` lines 243–247).
**Backend-dev must grep for an existing helper before adding a new one**
(DRY per CLAUDE.md) — if none exists, add
`requireUserID(c *gin.Context) (uint, bool)` to a shared location (e.g.
`handlers/helpers.go` if that file exists, else a new
`handlers/auth_helpers.go`) and refactor the two existing call sites to use
it as a drive-by cleanup, OR inline the existing pattern in
`changelog_handler.go` without introducing a new helper if consolidating
the other two call sites is judged out-of-scope for this PR — **flag this
choice explicitly in the PR description**, don't silently pick one.

**API Response Shapes** (matches design doc verbatim):

| Route | Method | Request Body | Response |
|---|---|---|---|
| `/api/v1/changelog/status` | GET | — | `{ "show_changelog": bool, "versions": Entry[] }` |
| `/api/v1/changelog/all` | GET | — | `{ "versions": Entry[] }` |
| `/api/v1/changelog/ack` | POST | `{ "action": "dismiss_temporary"\|"dismiss_permanent", "opt_out": bool }` | `{ "message": string }` |
| `/api/v1/changelog/opt-in` | POST | — | `{ "message": string }` |

`Entry` JSON shape: `{ "version", "date", "features": string[], "fixes": string[], "other": string[] }`.

### 3.4 `scripts/generate-changelog.sh` (new)

Bash script, POSIX-ish, follows the style of existing scripts in
`scripts/` (e.g. `scripts/check-version-match-tag.sh`,
`scripts/go-test-coverage.sh` — backend-dev/devops to confirm shared
conventions like `set -euo pipefail` and shellcheck compliance before
writing, since `lefthook` likely runs shellcheck on `scripts/**` per
existing CI hooks).

**Logic** (per design doc §"Changelog Data Generation"):

```bash
#!/usr/bin/env bash
set -euo pipefail

OUTPUT="backend/internal/changelog/data/changelog.json"
TAGS=$(git tag -l 'v*' --sort=v:refname)

# Build a JSON array; jq for correctness (already a CI/dev dependency —
# confirm via `command -v jq` and fail loudly if missing, matching the
# "no external dependencies" ethos only for the *runtime binary*, not
# build tooling, where jq/git are already assumed present in CI).

json_entries="[]"
prev_tag=""
for tag in $TAGS; do
  version="${tag#v}"
  date=$(git log -1 --format=%ad --date=short "$tag")
  range="${prev_tag:+$prev_tag..}$tag"

  features="[]"; fixes="[]"; other="[]"
  while IFS= read -r subject; do
    [ -z "$subject" ] && continue
    case "$subject" in
      feat:*|feat\(*\):*) text="${subject#*:}"; features=$(jq --arg t "${text# }" '. + [$t]' <<<"$features") ;;
      fix:*|fix\(*\):*)   text="${subject#*:}"; fixes=$(jq    --arg t "${text# }" '. + [$t]' <<<"$fixes") ;;
      *)                  other=$(jq --arg t "$subject" '. + [$t]' <<<"$other") ;;
    esac
  done < <(git log "$range" --pretty=%s 2>/dev/null || true)

  entry=$(jq -n --arg v "$version" --arg d "$date" \
    --argjson f "$features" --argjson x "$fixes" --argjson o "$other" \
    '{version:$v, date:$d, features:$f, fixes:$x, other:$o}')
  json_entries=$(jq --argjson e "$entry" '. + [$e]' <<<"$json_entries")
  prev_tag="$tag"
done

echo "$json_entries" | jq '.' > "$OUTPUT"
echo "Generated $OUTPUT with $(echo "$json_entries" | jq 'length') version entries"
```

(Exact prefix-parsing regex/case patterns to be finalized by backend-dev
against real repo history during implementation — must handle scoped
conventional commits like `feat(dns):` per this repo's actual commit log,
confirmed via `git log --oneline | head -50` showing scoped prefixes are
in active use, e.g. this session's own recent commits include
`fix(e2e): tolerate Firefox navigation-commit race...` — the case patterns
above already account for `type(scope):` via the `feat\(*\):*` glob, must
be validated with a unit-style shell test or a `--dry-run` manual check
against `git tag -l 'v*'` in this repo before merge.)

**Known gaps to resolve during Commit 2's real-history smoke test (flag
now, don't let them surprise implementation):**

- **Shellcheck SC2086**: the draft above has multiple unquoted expansions
  — `for tag in $TAGS`, `<<<"$features"`, etc. — that will very likely
  trip `SC2086` (unquoted variable, word-splitting/globbing risk), which
  `lefthook`'s pre-commit shellcheck stage enforces on `scripts/**` per
  CLAUDE.md's static-analysis gate. The final script must quote
  consistently (`for tag in $TAGS` in particular needs either
  `readarray -t tags <<< "$TAGS"` + `for tag in "${tags[@]}"`, or an
  equivalent shellcheck-clean rewrite) — treat this as a known,
  budgeted-for cleanup pass in Commit 2, not a late-discovered lint
  failure.
- **Breaking-change marker (`feat!:`/`fix!:`)**: the `case` patterns above
  match `feat:`, `feat(scope):`, `fix:`, `fix(scope):` but not the
  conventional-commits breaking-change shorthand `feat!:`/`fix!:`/
  `feat(scope)!:` — those currently fall through to `other`, silently
  demoting a breaking feature/fix to the collapsed "maintenance details"
  section, which is arguably the opposite of the desired novice-user-
  facing prominence. Backend-dev must check this repo's actual commit
  history for any real use of the `!` marker during Commit 2's smoke test
  (`git log --oneline --all | grep -E '(feat|fix)(\([^)]*\))?!:'`) and
  either (a) extend the case patterns to treat `!`-marked commits as
  `features`/`fixes` (accepting the categorization as-is, just widening
  the match), or (b) explicitly document the gap as accepted if the
  marker turns out to be unused in this repo's history — don't silently
  ship whichever behavior the first draft happens to produce.

**Workflow wiring** — `.github/workflows/release-goreleaser.yml`, insert
new step between `Install Cross-Compilation Tools (Zig)` (ends line 72)
and `Run GoReleaser` (starts line 77):

```yaml
      - name: Generate Changelog Data
        run: bash scripts/generate-changelog.sh

      - name: Run GoReleaser
        uses: goreleaser/goreleaser-action@f06c13b6b1a9625abc9e6e439d9c05a8f2190e94 # v7
        ...
```

Requires `fetch-depth: 0` on `actions/checkout` (already set, line 34) so
full tag/log history is available — no change needed there, just confirms
the existing checkout step already supports this script.

### 3.5 Config wiring (`CHARON_CHANGELOG_VERSION`)

Not added to `config.Config` struct (see §2.4 rationale). Read directly
where the changelog service is constructed in `routes.go` (§3.3), gated on
`cfg.Environment != "production"` — reuses the existing `cfg.Environment`
field already populated by `getEnvAny("development", "CHARON_ENV", "CPM_ENV")`.
No changes to `backend/internal/config/config.go` itself.

### 3.6 Frontend: API client, hook, and User-type extension

**File: `frontend/src/api/changelog.ts`** (new):

```typescript
import client from './client';

export interface ChangelogEntry {
  version: string;
  date: string;
  features: string[];
  fixes: string[];
  other: string[];
}

export interface ChangelogStatus {
  show_changelog: boolean;
  versions: ChangelogEntry[];
}

export interface ChangelogAll {
  versions: ChangelogEntry[];
}

export type ChangelogAckAction = 'dismiss_temporary' | 'dismiss_permanent';

export interface ChangelogAckRequest {
  action: ChangelogAckAction;
  opt_out: boolean;
}

export const getChangelogStatus = async (): Promise<ChangelogStatus> => {
  const response = await client.get<ChangelogStatus>('/changelog/status');
  return response.data;
};

export const getChangelogAll = async (): Promise<ChangelogAll> => {
  const response = await client.get<ChangelogAll>('/changelog/all');
  return response.data;
};

export const ackChangelog = async (req: ChangelogAckRequest): Promise<void> => {
  await client.post('/changelog/ack', req);
};

export const optInChangelog = async (): Promise<void> => {
  await client.post('/changelog/opt-in');
};
```

**File: `frontend/src/hooks/useChangelog.ts`** (new):

```typescript
import { useMutation, useQuery, useQueryClient } from '@tanstack/react-query';

import {
  ackChangelog, getChangelogAll, getChangelogStatus, optInChangelog,
  type ChangelogAckRequest,
} from '../api/changelog';
import { toast } from '../utils/toast';

export function useChangelogStatus() {
  return useQuery({
    queryKey: ['changelog-status'],
    queryFn: getChangelogStatus,
    // Fetched after layout render per design doc — staleTime keeps it
    // from refetching aggressively on route changes within a session.
    staleTime: 1000 * 60 * 5,
  });
}

export function useChangelogAll(enabled: boolean) {
  return useQuery({
    queryKey: ['changelog-all'],
    queryFn: getChangelogAll,
    enabled, // only fetched when browse-mode modal is opened
  });
}

export function useAckChangelog() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: (req: ChangelogAckRequest) => ackChangelog(req),
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['changelog-status'] });
      queryClient.invalidateQueries({ queryKey: ['auth', 'me'] }); // opt_out lives on user
    },
    onError: () => toast.error('Failed to update changelog preferences'),
  });
}

export function useOptInChangelog() {
  const queryClient = useQueryClient();
  return useMutation({
    mutationFn: optInChangelog,
    onSuccess: () => {
      queryClient.invalidateQueries({ queryKey: ['changelog-status'] });
    },
    onError: () => toast.error('Failed to re-enable update notifications'),
  });
}
```

Note: `AuthContext` does not use React Query for `/auth/me` (it's a
`useState`/`useEffect`-driven context, see `AuthContext.tsx` lines 8–13,
55–67) — there is no `['auth', 'me']` query key to invalidate today.
**Frontend-dev must instead call `AuthContext`'s own refresh mechanism**
(inspect `AuthContext.tsx` for an existing `refetchUser`/re-run-of-the-
`/auth/me`-effect function; if none is exported, add one, since the
Appearance Settings toggle needs the freshly-flipped `changelog_opt_out`
to reflect immediately after `ack`/`opt-in` without a full page reload).
This is a concrete gap the hook design above glosses over with a
query-invalidation call that currently has no effect — flagged for
frontend-dev to resolve during implementation, not silently shipped as
dead code.

**File: `frontend/src/context/AuthContextValue.ts`** — extend `User`:

```typescript
export interface User {
  user_id: number;
  role: 'admin' | 'user' | 'passthrough';
  name?: string;
  email?: string;
  changelog_opt_out?: boolean; // added for "What's New" toggle
}
```

**File: `backend/internal/api/handlers/auth_handler.go`** — `Me()`
(lines 320–325), add `changelog_opt_out` to the response:

```go
c.JSON(http.StatusOK, gin.H{
    "user_id":           userID,
    "role":              role,
    "name":              u.Name,
    "email":             u.Email,
    "changelog_opt_out": u.ChangelogOptOut,
})
```

### 3.7 `WhatsNewModal.tsx`

**File: `frontend/src/components/dialogs/WhatsNewModal.tsx`** (new).
Check whether `frontend/src/components/dialogs/` already exists as a
convention (grep for existing modal components — likely
`frontend/src/components/` has a `Modal`/`Dialog` base component to reuse,
e.g. check for existing confirm-dialog patterns used elsewhere in Settings
pages before inventing new modal chrome).

**Props**:

```typescript
interface WhatsNewModalProps {
  mode: 'status' | 'browse'; // status = post-auth auto-check; browse = Settings revisit link
  open: boolean;
  onClose: () => void;
}
```

**Behavior**:
- `mode="status"`: uses `useChangelogStatus()`; renders nothing
  (`return null`) until `data.show_changelog === true`; footer has
  checkbox + "Remind Me Next Time" (`ack({action: 'dismiss_temporary', opt_out: checked})`)
  + "Got It, Thanks" (`ack({action: 'dismiss_permanent', opt_out: checked})`)
  + X/backdrop-click (same effect as "Remind Me Next Time").
- `mode="browse"`: uses `useChangelogAll(open)`; no footer actions besides
  a single "Close" button; no `ack` calls of any kind (voluntary revisit,
  per design doc).
- Content: one `<section>` per `ChangelogEntry`, newest-first (API already
  returns newest-first per §3.2's `sortNewestFirst`, so no client
  re-sorting needed). "✨ New Features" / "🐛 Fixes" expanded by default,
  omitted entirely if the array is empty. "🔧 Other" behind a
  `<details>`/disclosure collapsed by default, labeled "Show maintenance
  details".
- Non-blocking fetch: mounted unconditionally in `Layout.tsx` in
  `mode="status"`; the component's own `useChangelogStatus()` query runs
  after Layout's initial render (React Query fires on mount, which is
  already after first paint) — no additional delay/gating logic needed
  beyond normal component mount order; on fetch error
  (`useChangelogStatus().isError`), render `null`.

### 3.8 `AppearanceSettings.tsx` changes

Insert a new `<Card>` section (matching the existing five-section
pattern in the file) after the "Banner Customization Section" (ends line
235), before the closing `</div>` (line 236):

```tsx
{/* What's New Notifications Section */}
<Card>
  <CardHeader>
    <div className="flex items-center gap-2">
      <Bell className="h-5 w-5 text-content-secondary" />
      <CardTitle>{t('appearance.whatsNew')}</CardTitle>
    </div>
    <CardDescription>{t('appearance.whatsNewDescription')}</CardDescription>
  </CardHeader>
  <CardContent className="flex items-center justify-between">
    <label className="flex items-center gap-3">
      <input
        type="checkbox"
        checked={!user?.changelog_opt_out}
        onChange={(e) => {
          if (e.target.checked) {
            optInMutation.mutate();
          } else {
            ackMutation.mutate({ action: 'dismiss_temporary', opt_out: true });
          }
        }}
      />
      {t('appearance.showWhatsNewToggle')}
    </label>
    <Button variant="secondary" onClick={() => setBrowseModalOpen(true)}>
      {t('appearance.whatsNewRevisit')}
    </Button>
  </CardContent>
</Card>

<WhatsNewModal mode="browse" open={browseModalOpen} onClose={() => setBrowseModalOpen(false)} />
```

Requires: `import { Bell } from 'lucide-react'` added to existing
lucide import; `const { user } = useAuth()` added (not currently imported
in this file — confirm no naming collision with existing locals);
`useAckChangelog`/`useOptInChangelog` hooks imported; local
`const [browseModalOpen, setBrowseModalOpen] = useState(false)`.

### 3.9 Layout mount point

`frontend/src/components/Layout.tsx`, after `<FeedbackWidget />` (line
416), before the closing `</div>` (line 417):

```tsx
<WhatsNewModal mode="status" open={statusModalOpen} onClose={handleStatusModalClose} />
```

Where `statusModalOpen` is derived from `useChangelogStatus().data?.show_changelog`
inside `WhatsNewModal` itself in `status` mode (the modal owns its own
visibility in status mode per §3.7 — `open`/`onClose` props are primarily
for `browse` mode's externally-controlled visibility; status mode can
either (a) always render `<WhatsNewModal mode="status" />` with no
`open`/`onClose` needed since it self-gates on `show_changelog`, or (b)
take an `open` prop that's ignored in status mode. **Frontend-dev
decision point**: prefer (a) — simplify the props so `open`/`onClose` are
`browse`-mode-only (required together), and `status` mode takes no props
besides `mode` and manages its own dismiss-triggered refetch internally
via the `ack` mutation's `onSuccess` invalidation. Document the final prop
contract in the component's JSDoc.

### 3.10 Data Flow Diagram

```
┌─────────────┐   GET /changelog/status   ┌──────────────────┐   SELECT user   ┌──────────┐
│ Layout.tsx   │ ────────────────────────▶│ ChangelogHandler  │ ───────────────▶│  SQLite  │
│ (post-auth)  │                           │ .Status()         │                 │  users   │
└─────────────┘                           └──────────────────┘                 └──────────┘
      │                                            │
      │ show_changelog=true                        │ changelog.Service
      ▼                                             │ .GetEntriesSince(user.LastSeenVersion)
┌─────────────┐                                     ▼
│ WhatsNewModal│                          ┌──────────────────────┐
│ (renders)    │                          │ embedded changelog.json│ (go:embed, parsed at init)
└─────────────┘                          └──────────────────────┘
      │
      │ user clicks "Got It, Thanks" (checkbox unchecked)
      ▼
POST /changelog/ack {action: dismiss_permanent, opt_out: false}
      │
      ▼
UPDATE users SET last_seen_version = <current running version> WHERE id = ?
```

### 3.11 Error Handling

| Failure | Behavior |
|---|---|
| `/changelog/status` fetch fails (network/5xx) | React Query `isError`; modal renders `null`; no toast (non-blocking per design doc — a failed background check should not interrupt the user) |
| `/changelog/ack` fails | `toast.error(...)`; modal stays open so the user can retry (do not optimistically close on mutate) |
| `changelog.json` malformed at embed time | `json.Unmarshal` error swallowed in `init()`, `allEntries` stays `nil` → `GetEntriesSince`/`GetAllEntries` return empty slices → `/status` naturally reports `show_changelog: false` (empty-data edge case, §Edge Cases in design doc) — server never fails to boot over bad changelog data |
| User row not found in `Status`/`Ack`/`OptIn` (e.g. deleted mid-session) | **Mandatory**: `Ack`/`OptIn` MUST check `result.RowsAffected == 0` after the `Updates`/`Update` call and return `404 {"error": "User not found"}` — GORM's `Updates`/`Update` on a missing row is a silent no-op (`RowsAffected: 0`, no error), so skipping this check would make the endpoint lie about success. Direct precedent already in the codebase: `backend/internal/api/handlers/custom_theme_handler.go`'s `DeleteTheme` (`result := h.db.Delete(...); if result.RowsAffected == 0 { 404 }`). `Status` uses `db.First(...)` instead, which already returns a real `gorm.ErrRecordNotFound` error on a missing row, handled the same way `GetProfile` does. §3.3's code sketch includes this check on both `Ack` and `OptIn`. |
| `CHARON_CHANGELOG_VERSION` set with `CHARON_ENV=production` | Ignored entirely (gated by `cfg.Environment != "production"` at the call site, §3.5) — no error, just inert, matching how other dev-only overrides in this codebase behave |
| `generate-changelog.sh` fails mid-release (e.g. `jq` missing) | Script uses `set -euo pipefail` → non-zero exit → workflow step fails → release blocked before `goreleaser build` runs, preventing a release with stale/corrupt changelog data from shipping silently |

## 4. Implementation Plan

### Phase 1: Playwright E2E Specs (behavior-first, `test.fixme`)

New file: `tests/settings/whats-new-changelog.spec.ts`. Scenarios (all
`test.fixme(...)` until Phase 4/5 lands):

1. Version bump (`CHARON_CHANGELOG_VERSION` override + fixture data +
   test user with `last_seen_version` below it) → modal appears on login.
2. "Remind Me Next Time" → modal reappears on next login; `LastSeenVersion`
   unchanged (verify via `/user/profile` or DB fixture check).
3. "Got It, Thanks" → modal does not reappear; `LastSeenVersion` updated.
4. X/backdrop click → same effect as "Remind Me Next Time".
5. Opt-out checkbox (checked on any dismiss path) → modal suppressed on
   subsequent logins even after a further version bump.
6. Appearance Settings toggle flips opt-out off → modal reappears on next
   qualifying login.
7. "What's New" revisit link in Appearance Settings → opens browse-mode
   modal showing full history via `/changelog/all`, independent of
   `LastSeenVersion`; closing it does not call `ack`.

Uses committed Playwright fixture changelog data (not real git tags) per
design doc §Local & Pre-Merge Testing — new fixture file, e.g.
`tests/fixtures/changelog-fixture.json`, loaded by temporarily writing it
to `backend/internal/changelog/data/changelog.json` in a `beforeAll`/global
setup step (playwright-dev to confirm exact mechanism against how other
E2E specs inject backend fixture state — likely via `tests/utils/api-helpers.ts`
or direct file write + backend restart, whichever this repo's E2E harness
already supports for backend-state fixtures).

**Validation gate**: `npx playwright test tests/settings/whats-new-changelog.spec.ts --project=firefox`
runs and all scenarios report `fixme` (skipped, not failed) — proves the
file parses and the test names/structure are locked in before any
implementation code exists.

### Phase 2: Foundation (contracts, no behavior change)

- `backend/internal/changelog/changelog.go` + `data/changelog.json`
  placeholder + `changelog_test.go` (TDD red → green for the package in
  isolation, no handler/route wiring yet).
- `scripts/generate-changelog.sh` + `.github/workflows/release-goreleaser.yml`
  step insertion.
- `frontend/src/api/changelog.ts` (types + client functions only — no
  hook/component consumption yet, so `npm run type-check` passes but
  nothing calls these functions).

**Validation gate**: `cd backend && go build ./... && go test ./internal/changelog/...`
green; `cd frontend && npm run type-check` green; `bash scripts/generate-changelog.sh`
runs locally against this repo's real tags without error (manual smoke
test, output diffed against expectations, then reverted — never commit the
locally-generated file over the `[]` placeholder).

### Phase 3: Backend (API/model/service integration)

- `backend/internal/models/user.go` — add `LastSeenVersion`/`ChangelogOptOut`.
- `backend/internal/api/handlers/changelog_handler.go` +
  `changelog_handler_test.go`.
- `backend/internal/api/handlers/auth_handler.go` — extend `Me()`.
- `backend/internal/api/handlers/user_handler.go` — seed
  `LastSeenVersion` at `Setup`, `CreateUser`, `AcceptInvite` (see §2.9).
- `backend/internal/api/routes/routes.go` — route registration +
  `CHARON_CHANGELOG_VERSION` wiring (§3.3, §3.5).
- Backend unit tests per §3.2/§3.3/design-doc §Testing:
  `changelog_test.go`, `changelog_handler_test.go` (including
  passthrough-rejection cases for all four handlers, and the
  `RowsAffected == 0` → 404 case for `Ack`/`OptIn`), plus updates to
  `auth_handler_test.go` for the `Me()` `changelog_opt_out` field, and
  itemized additions to `user_handler_test.go` for seeding — one test per
  call site, each with a real-version case and a `version.Version == "dev"`
  skip-seeding case:
  - `TestSetup_SeedsLastSeenVersion` / `TestSetup_SkipsSeedingOnDevBuild`
  - `TestCreateUser_SeedsLastSeenVersion` / `TestCreateUser_SkipsSeedingOnDevBuild`
  - `TestAcceptInvite_SeedsLastSeenVersion` / `TestAcceptInvite_SkipsSeedingOnDevBuild`
  - (no `InviteUser` seeding test — per §2.9, the pending/disabled row
    created by `InviteUser` is deliberately left unseeded; a test
    asserting `LastSeenVersion == ""` immediately after `InviteUser` is
    optional documentation of that choice, not required)

**Validation gate**: `cd backend && go build ./... && go test ./... && make lint-fast`
green; `./scripts/scan-gorm-security.sh --check` zero CRITICAL/HIGH
(triggered per CLAUDE.md §1.5 — this phase touches `internal/models`).

### Phase 4: Frontend (UI integration + tests)

- `frontend/src/api/changelog.ts` — already created (types/client
  functions) in Phase 2; no changes expected here unless Phase 3's real
  endpoints reveal a shape mismatch.
- `frontend/src/api/changelog.test.ts` (new) — unit tests for each
  exported function's request shape (`getChangelogStatus`/`getChangelogAll`
  hit the right URL and return `response.data`; `ackChangelog`/
  `optInChangelog` POST the right body), mocking `client` the same way
  `frontend/src/api/notifications.test.ts` /
  `frontend/src/api/featureFlags.test.ts` do (existing precedent files in
  `frontend/src/api/` — follow their mocking approach exactly).
- `frontend/src/hooks/useChangelog.ts`.
- `frontend/src/hooks/__tests__/useChangelog.test.ts` (new) — per-hook
  cases: `useChangelogStatus` returns query state correctly;
  `useChangelogAll(enabled)` does NOT fire when `enabled=false` and does
  fire when `enabled=true` (the `enabled` gating is the one behavior in
  this hook file most likely to silently break and go uncaught without a
  dedicated test); `useAckChangelog`/`useOptInChangelog` mutation
  success/error paths, including that `onError` calls `toast.error` and
  `onSuccess` calls `invalidateQueries` with the expected query keys.
- `frontend/src/context/AuthContextValue.ts` — extend `User`; confirm/add
  a refetch mechanism in `AuthContext.tsx` (§3.6 gap).
- `frontend/src/components/dialogs/WhatsNewModal.tsx` + Vitest tests
  (`WhatsNewModal.test.tsx`) per design doc §Testing: rendering per
  response shape, all three dismiss paths' payloads, checkbox/opt-out
  interaction, collapsed "Other" disclosure default state.
- `frontend/src/components/Layout.tsx` — mount point.
- `frontend/src/pages/AppearanceSettings.tsx` — toggle + revisit link +
  Vitest coverage additions.
- i18n strings: `appearance.whatsNew`, `appearance.whatsNewDescription`,
  `appearance.showWhatsNewToggle`, `appearance.whatsNewRevisit`, plus
  modal copy keys — added to whichever locale files this repo maintains
  (check `frontend/src/locales/` for the full language list; every
  locale file must get the new keys, not just `en`, to avoid missing-key
  fallback warnings — grep for how the "Feedback Widget" (nearby feature)
  did its locale rollout as a template).

**Validation gate**: `cd frontend && npm run type-check && npx vitest run`
green; component/hook tests included in the 85% coverage floor.

### Phase 5: Hardening (enable E2E, docs)

- Flip all `test.fixme` → `test` in `tests/settings/whats-new-changelog.spec.ts`.
- `docs/features.md` — brief new entry per CLAUDE.md's "Features: Update
  `docs/features.md` when adding capabilities — keep it brief, link to
  individual docs" (a short new bullet under an appropriate section, no
  new deep-dive doc required per the design doc's scope).
- Full Definition of Done pass (§5 below).

**Validation gate**: `npx playwright test --project=firefox` full suite
green (not just the new spec — regression check per repo convention);
`bash scripts/local-patch-report.sh` produces required artifacts;
`scripts/go-test-coverage.sh` and `scripts/frontend-test-coverage.sh`
both ≥85%.

## 5. Acceptance Criteria

- [ ] `User` gains `last_seen_version`/`changelog_opt_out`; AutoMigrate
      applies cleanly on top of an existing populated DB with no data loss.
- [ ] New users (`Setup`, `CreateUser`, `AcceptInvite`) never see
      historical changelog entries on first login (seeded correctly,
      except when `version.Version == "dev"`).
- [ ] Pre-existing users (empty `last_seen_version` at migration time) see
      **all entries since their (empty) last-seen version — i.e., full
      history, grouped by version** — on their first post-upgrade login,
      per the already-approved "catch-up shows all missed versions
      grouped" behavior. This matches §3.2's implementation and
      `TestGetEntriesSince_EmptyLastSeen_ReturnsAll` exactly: empty
      `lastSeen` is "behind everything," so `GetEntriesSince("")` returns
      every entry, not just the newest one. (The design doc's prose at
      lines ~45/~227 says "see the current version's entries" — read
      loosely, that could mean "only the newest version." That reading
      does not survive the doc's own explicit, separately-approved
      decision that catch-up shows *all* missed versions grouped, which
      is the general rule for any user behind by more than one version —
      an empty `LastSeenVersion` is just the maximal case of "behind,"
      not a special narrower one. Treat §3.2/this bullet as the
      authoritative behavior; the doc's phrasing is imprecise, not a
      conflicting decision.)
- [ ] All four routes (`/status`, `/all`, `/ack`, `/opt-in`) implemented,
      authenticated, reachable by both `admin` and `user` roles.
- [ ] `show_changelog` is `false` for: dev builds, opted-out users, and
      zero-unseen-entries — never renders an empty modal.
- [ ] `scripts/generate-changelog.sh` produces valid, schema-conformant
      JSON from real repo tags; wired into `release-goreleaser.yml` before
      `Run GoReleaser`.
- [ ] Placeholder `changelog.json` (`[]`) committed; local/dev/PR builds
      never see synthetic changelog data.
- [ ] `WhatsNewModal` renders correctly in both `status` and `browse`
      modes; all three dismiss paths send correct `ack` payloads; browse
      mode never calls `ack`.
- [ ] Appearance Settings toggle and revisit link both functional and
      reflect live `changelog_opt_out` state without requiring a page
      reload.
- [ ] Backend coverage ≥85%, frontend coverage ≥85% (CLAUDE.md DoD §6).
- [ ] `go build ./...`, `npm run build`, `npm run type-check`,
      `make lint-fast` all green.
- [ ] Zero CRITICAL/HIGH findings from `./scripts/scan-gorm-security.sh --check`.
- [ ] All Playwright scenarios in §Implementation Plan Phase 1 pass with
      `--project=firefox`.
- [ ] `docs/features.md` updated.

## 6. Commit Slicing Strategy

**Decision**: single PR on `feat/changelog` (current branch), ordered
logical commits within it — one feature, one PR, per CLAUDE.md/repo
convention. No commit is mergeable on its own; the PR merges only once all
commits land and the full DoD (§5) passes.

### Commit 1 — E2E specs for the new flow (`test.fixme`)

**Scope**: test scaffolding only, zero production code.
**Files**:
- `tests/settings/whats-new-changelog.spec.ts` (new)
- `tests/fixtures/changelog-fixture.json` (new)

**Depends on**: nothing (branch tip).
**Validation gate**: `npx playwright test tests/settings/whats-new-changelog.spec.ts --project=firefox`
→ all scenarios report `fixme`, zero failures; `git diff` shows no
non-test files touched.

### Commit 2 — Foundation: contracts, generator script, embed placeholder

**Scope**: `internal/changelog` package (types + service, TDD red→green
against its own fixtures), placeholder embedded data file,
`generate-changelog.sh`, workflow wiring, frontend API client types. No
route registration, no User model change, no UI consumption — nothing
observably changes app behavior yet.
**Files**:
- `backend/internal/changelog/changelog.go` (new)
- `backend/internal/changelog/changelog_test.go` (new)
- `backend/internal/changelog/data/changelog.json` (new, `[]`)
- `scripts/generate-changelog.sh` (new)
- `.github/workflows/release-goreleaser.yml` (modified — new step)
- `.dockerignore` (modified — negate `backend/internal/changelog/data/`
  and `backend/internal/changelog/data/changelog.json` against the
  existing bare `data/` exclude at line 96; see §7/R1)
- `frontend/src/api/changelog.ts` (new)

**Depends on**: Commit 1 (spec file establishes the contract this commit
implements against, though this commit doesn't yet make the spec pass).
**Validation gate**: `go build ./... && go test ./internal/changelog/...`
green; `go mod tidy` diff reviewed (semver promoted indirect→direct);
`npm run type-check` green; manual `bash scripts/generate-changelog.sh`
smoke test against real repo tags, output reviewed then discarded
(`git checkout -- backend/internal/changelog/data/changelog.json`);
**`.dockerignore` fix verified with a real Docker build context check**
(build/inspect the `backend-builder` stage and confirm
`backend/internal/changelog/data/changelog.json` is present post-`COPY` —
see §7's "Required verification" note; do not merge this commit on
negation-syntax inspection alone).

### Commit 3 — Backend: model, routes, handlers, seeding

**Scope**: everything that makes the API real: migration, handler, route
registration, `CHARON_CHANGELOG_VERSION` wiring, `Me()` extension, seeding
at all three user-creation call sites.
**Files**:
- `backend/internal/models/user.go` (modified)
- `backend/internal/api/handlers/changelog_handler.go` (new)
- `backend/internal/api/handlers/changelog_handler_test.go` (new)
- `backend/internal/api/handlers/auth_handler.go` (modified — `Me()`)
- `backend/internal/api/handlers/auth_handler_test.go` (modified)
- `backend/internal/api/handlers/user_handler.go` (modified — 3 seeding
  call sites)
- `backend/internal/api/handlers/user_handler_test.go` (modified)
- `backend/internal/api/routes/routes.go` (modified)

**Depends on**: Commit 2 (`changelog.Service` must exist).
**Validation gate**: `go build ./... && go test ./... && make lint-fast`
green; `./scripts/scan-gorm-security.sh --check` zero CRITICAL/HIGH (model
change trigger, CLAUDE.md §1.5); full backend coverage check
(`scripts/go-test-coverage.sh`) ≥85% for touched packages at minimum.

### Commit 4 — Frontend: modal, hooks, layout, settings integration

**Scope**: everything user-facing.
**Files**:
- `frontend/src/api/changelog.test.ts` (new)
- `frontend/src/hooks/useChangelog.ts` (new)
- `frontend/src/hooks/__tests__/useChangelog.test.ts` (new)
- `frontend/src/context/AuthContextValue.ts` (modified)
- `frontend/src/context/AuthContext.tsx` (modified, if a refetch seam is
  needed per §3.6)
- `frontend/src/components/dialogs/WhatsNewModal.tsx` (new)
- `frontend/src/components/dialogs/WhatsNewModal.test.tsx` (new)
- `frontend/src/components/Layout.tsx` (modified — mount point)
- `frontend/src/pages/AppearanceSettings.tsx` (modified — toggle + link)
- `frontend/src/pages/__tests__/AppearanceSettings.test.tsx` (modified or
  new, confirm existing test file name/location first)
- `frontend/src/locales/**` (modified — new i18n keys, all locales)

**Depends on**: Commit 2, not Commit 3 — Vitest tests mock `client`
directly (per this repo's existing `frontend/src/api/*.test.ts`
convention), so this commit only needs Commit 2's TS types/contracts
(`frontend/src/api/changelog.ts`) to build and pass its own gate; it does
not call live backend endpoints. Commit 3 landing first is convenient
(lets frontend-dev sanity-check the real response shape once) but is not
a hard build/test dependency — noted so the commits can be reordered or
parallelized if useful, without treating Commit 3 as a blocking
prerequisite.
**Validation gate**: `npm run type-check && npx vitest run` green;
frontend coverage ≥85% for touched files; `npm run build` succeeds.

### Commit 5 — Hardening: enable E2E, docs

**Scope**: flip `test.fixme` → `test`, run the full E2E suite, update
`docs/features.md`, final DoD pass.
**Files**:
- `tests/settings/whats-new-changelog.spec.ts` (modified — un-fixme)
- `docs/features.md` (modified)

**Depends on**: Commits 1–4 all merged into the branch.
**Validation gate**: full Definition of Done per CLAUDE.md (§5 of this
plan) — Playwright full suite, patch coverage preflight, security scans,
lefthook, staticcheck, coverage ≥85% both sides, both builds green.

### Rollback & Contingency (PR-level)

- **Pre-merge**: any commit can be amended/squashed within the branch
  freely (not yet merged) — no special rollback concerns.
- **Route grouping** (`protected` vs `management`, plus the
  `rejectPassthrough()` guard): resolved during supervisor review — §2.5's
  `protected`-group placement combined with an in-handler
  `rejectPassthrough()` call (§3.3) is now the settled design, matching
  the repo's actual convention. Not an open inference anymore; no further
  flagging needed unless implementation uncovers a new wrinkle.
- **If `generate-changelog.sh`'s conventional-commit categorization proves
  too lossy against real repo history** (e.g. many unprefixed historical
  commits dumping into "other" and drowning out real entries): this is a
  script tuning problem, not a design change — iterate within Commit 2,
  do not alter the categorization *rules* (which are spec'd) without
  flagging to the user first.
- **Post-merge rollback**: standard `git revert` of the merge commit;
  since `LastSeenVersion`/`ChangelogOptOut` are additive nullable-default
  columns, no destructive down-migration is needed — reverting the PR
  simply stops reading/writing them (GORM AutoMigrate never drops
  columns, so the columns harmlessly remain until a future cleanup if the
  feature is permanently abandoned, which is out of scope to plan for
  here).
- **Feature flag**: not used — the design doc has no feature-flag gating
  requirement, and the `dev`-build short-circuit already provides a safe
  default-off state for anyone building from source without a release tag.

## 7. `.gitignore` / `.dockerignore` / `.codecov.yml` / `Dockerfile` Review

| File | Change needed? | Reasoning |
|---|---|---|
| `.gitignore` | **No** | The placeholder `backend/internal/changelog/data/changelog.json` must be tracked (it's the dev/PR-build fallback and the thing `go:embed` needs to exist at all — an untracked/gitignored embed target would break every build that isn't a tagged release). No existing pattern in `.gitignore` matches this path (checked: no generic `*.json` or `data/**` ignore that would catch it — the closest is `backend/package.json`/`backend/temp_index.json`, both unrelated specific filenames, not a directory-wide rule). Nothing to add. |
| `.dockerignore` | **YES — required** | `.dockerignore:96` is a bare `data/` line with no leading `/` or `**/` qualifier. Docker's `.dockerignore` matcher uses the same glob semantics as `.gitignore`: an unanchored `data/` matches a directory named `data` **at any depth in the build context**, not just a top-level one — confirmed by checking the file directly (line 96 reads exactly `data/`, immediately after the "Databases (created at runtime)" section header, clearly intended for a top-level `data/` runtime dir, but syntactically unscoped). This silently excludes `backend/internal/changelog/data/` (and its committed `changelog.json`) from the build context sent to every `docker build` invocation — including `docker-build.yml`, `e2e-tests-split.yml`, and any Trivy/security-scan workflow that builds this repo's multi-stage `Dockerfile`. Inside the `backend-builder` stage, `COPY backend/ ./` (line 242) then copies a `backend/internal/changelog/` directory with no `data/` subdirectory in it, so `//go:embed data/changelog.json` fails to compile — a hard build break, not a degraded-feature edge case. **Fix**: add explicit negations immediately after line 96 (or in a clearly-commented block near it), e.g. `!backend/internal/changelog/data/` and `!backend/internal/changelog/data/changelog.json` — negating both the directory and the file, since BuildKit's negation-after-broad-exclude has historically required re-including intermediate directories explicitly for file-level negations to take effect (this is the "known footgun" the validation gate below exists to catch, not something to trust on inspection alone). The repo's existing `!.env.example` / `!README.md` / `!LICENSE` lines (near the end of `.dockerignore`) establish that negation-after-broad-exclude is an accepted, already-used pattern in this file — this is a new instance of an existing technique, not a new technique. |
| `codecov.yml` | **No** | The existing `ignore:` list already contains `"*.json"` (under "CI/CD & Config", alongside `"*.yml"`/`"*.yaml"`) — this glob already excludes `changelog.json` from coverage accounting without any new entry. The new Go/TS **source** files (`changelog.go`, `changelog_handler.go`, `changelog.ts`, `useChangelog.ts`, `WhatsNewModal.tsx`) are exactly the kind of application logic this config is designed to *include* in coverage — no exclusion needed or wanted for them. |
| `Dockerfile` | **No** | `COPY backend/ ./` (line 242, `backend-builder` stage) already recursively includes `backend/internal/changelog/data/changelog.json` **once the `.dockerignore` fix above stops stripping it first** — no new `COPY` line needed, the `.dockerignore` change is the actual fix, this file is just the thing that would otherwise silently receive an incomplete context. The release image build path is separately unaffected by this feature (GoReleaser, not this multi-stage Dockerfile, is what runs `generate-changelog.sh` — see §3.4); a plain `docker build` (dev/nightly images) will, once the context is correct, simply embed the committed placeholder `[]`, which is correct/intended per the design doc (dev builds already report `version.Version == "dev"` and short-circuit `show_changelog` to `false` regardless of embedded data). |

**Required verification during Commit 2** (not optional — see §6 Commit 2's
validation gate): after adding the `.dockerignore` negations, actually
build the `Dockerfile`'s `backend-builder` stage (or at minimum run
`docker build --no-cache -f Dockerfile --target backend-builder .` far
enough to confirm `go build ./...`/`go vet` succeeds inside the container,
or use `docker build -f - . <<< 'FROM scratch AS ctx' ...`-style context
inspection / `docker buildx build --target backend-builder --output type=local,dest=/tmp/ctxcheck ...`)
to prove `backend/internal/changelog/data/changelog.json` is actually
present in the container filesystem post-`COPY`. Do not trust the
negation syntax by inspection alone — verify with a real build, since this
exact class of bug (broad exclude silently eating a nested path, negation
not actually restoring it) is easy to get subtly wrong and would only
surface as a CI failure days later.

## 8. Handoff

This plan is ready for `supervisor` review. Key judgment calls made during
research that supervisor/user should explicitly confirm before Commit 3
begins (flagged throughout, collected here for visibility):

1. **§2.5 / §3.3** — Changelog routes registered under `protected`, not
   `management` — **and required to call `rejectPassthrough()`** (the
   existing pattern from `user_handler.go`'s `GetProfile`/`UpdateProfile`/
   `RegenerateAPIKey`) in all four `ChangelogHandler` methods. This is no
   longer an open question: route-group placement alone was an incomplete
   read of the repo's actual convention for self-service `protected`
   routes, which pairs group placement with an in-handler passthrough
   check. §3.3's code sketch now includes this.
2. **§2.9** — `LastSeenVersion` seeded at `Setup` (initial admin) in
   addition to the two call sites the design doc names explicitly.
3. **§3.3** — `requireUserID` helper consolidation (DRY cleanup) is
   optional/backend-dev's call, not mandatory for this feature.
4. **§3.6 / §3.9** — `WhatsNewModal`'s exact prop contract for `status`
   mode (self-gating vs. externally-controlled `open`) and the
   `AuthContext` refetch-after-`ack` mechanism are left as concrete but
   open implementation decisions for frontend-dev, with a recommended
   default (self-gating; add a refetch export if none exists).
5. **§3.11** — `Ack`/`OptIn` **must** check `result.RowsAffected == 0` and
   return 404 on a deleted-user race — mandatory, per direct precedent in
   `custom_theme_handler.go`'s `DeleteTheme`, not a recommendation. §3.3's
   code sketch now includes this.
6. **§7 / R1** — `.dockerignore`'s unanchored `data/` exclude (line 96)
   silently strips `backend/internal/changelog/data/` from every Docker
   build context, breaking `go:embed` compilation in CI Docker builds
   (though not GoReleaser release builds). Fixed via negation in Commit 2,
   with a mandatory real-build verification step — flagged here because
   it's the one item in this plan that would have caused a CI break not
   caught by any of the backend/frontend validation gates on their own.
